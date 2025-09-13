package main

import (
	"context"
	"fmt"
	"sync"
	"time"

)

// ExecutorAgent implements the executor component of the unified dialog agent
type ExecutorAgent struct {
	config         *ExecutorConfig
	mcpManager     *MCPClientManager
	resolver       *ParameterResolver
	parallelLimiter chan struct{} // Semaphore for controlling parallel execution
}

// NewExecutorAgent creates a new executor agent
func NewExecutorAgent(config *ExecutorConfig, mcpManager *MCPClientManager) *ExecutorAgent {
	// Create a semaphore to limit parallel execution
	parallelLimiter := make(chan struct{}, config.MaxParallelSteps)

	return &ExecutorAgent{
		config:          config,
		mcpManager:      mcpManager,
		parallelLimiter: parallelLimiter,
	}
}

// ExecutePlan executes an execution plan
func (e *ExecutorAgent) ExecutePlan(ctx context.Context, plan *ExecutionPlan, state *ExecutionState) error {
	// Initialize resolver with current state
	e.resolver = NewParameterResolver(state)

	// Track plan in state
	state.Plan = plan
	state.AddLogEntry("", "plan_execution_started", map[string]interface{}{
		"plan_id":    plan.ID,
		"step_count": len(plan.Steps),
	}, LogLevelInfo)

	// Apply global timeout if configured
	if e.config.StepTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.StepTimeout)
		defer cancel()
	}

	// Execute steps in dependency order
	for {
		// Check if we've exceeded iteration limits
		if state.Iteration >= state.MaxIterations {
			return fmt.Errorf("exceeded maximum iterations (%d)", state.MaxIterations)
		}
		state.Iteration++

		// Get steps that are ready to execute
		completedSteps := state.GetCompletedSteps()
		readySteps := plan.GetReadySteps(completedSteps)

		if len(readySteps) == 0 {
			// Check if all steps are completed
			if len(completedSteps) == len(plan.Steps) {
				state.AddLogEntry("", "plan_execution_completed", map[string]interface{}{
					"completed_steps": len(completedSteps),
					"total_steps":     len(plan.Steps),
				}, LogLevelInfo)
				return nil // All steps completed successfully
			}

			// Find remaining steps that haven't completed
			var remainingSteps []string
			for _, step := range plan.Steps {
				if !completedSteps[step.ID] {
					// Check if step failed
					if result, exists := state.StepResults[step.ID]; exists && result.Status == StepStatusFailed {
						remainingSteps = append(remainingSteps, fmt.Sprintf("%s (failed)", step.ID))
					} else {
						remainingSteps = append(remainingSteps, step.ID)
					}
				}
			}

			return fmt.Errorf("no ready steps available, but %d steps remain incomplete: %v", len(remainingSteps), remainingSteps)
		}

		// Separate parallel and sequential steps
		parallelSteps := make([]ExecutionStep, 0)
		sequentialSteps := make([]ExecutionStep, 0)

		for _, step := range readySteps {
			if step.Parallel {
				parallelSteps = append(parallelSteps, step)
			} else {
				sequentialSteps = append(sequentialSteps, step)
			}
		}

		// Execute parallel steps first
		if len(parallelSteps) > 0 {
			if err := e.executeStepsInParallel(ctx, parallelSteps, state); err != nil {
				return fmt.Errorf("parallel execution failed: %w", err)
			}
		}

		// Execute sequential steps
		for _, step := range sequentialSteps {
			if err := e.executeStep(ctx, &step, state); err != nil {
				if step.ErrorPolicy == ErrorPolicyFailFast {
					return fmt.Errorf("step %s failed with fail_fast policy: %w", step.ID, err)
				}
				// Log error but continue with other steps if policy allows
				state.AddLogEntry(step.ID, "step_error_continue", map[string]interface{}{
					"error":        err.Error(),
					"error_policy": step.ErrorPolicy,
				}, LogLevelError)
			}
		}
	}
}

// executeStepsInParallel executes multiple steps in parallel
func (e *ExecutorAgent) executeStepsInParallel(ctx context.Context, steps []ExecutionStep, state *ExecutionState) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(steps))

	for _, step := range steps {
		wg.Add(1)
		go func(s ExecutionStep) {
			defer wg.Done()

			// Acquire semaphore slot
			e.parallelLimiter <- struct{}{}
			defer func() { <-e.parallelLimiter }()

			if err := e.executeStep(ctx, &s, state); err != nil {
				if s.ErrorPolicy == ErrorPolicyFailFast {
					errChan <- fmt.Errorf("parallel step %s failed: %w", s.ID, err)
					return
				}
				// Log error but don't fail the entire parallel execution
				state.AddLogEntry(s.ID, "parallel_step_error", map[string]interface{}{
					"error":        err.Error(),
					"error_policy": s.ErrorPolicy,
				}, LogLevelError)
			}
		}(step)
	}

	// Wait for all parallel steps to complete
	wg.Wait()
	close(errChan)

	// Check if any critical errors occurred
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// executeStep executes a single step
func (e *ExecutorAgent) executeStep(ctx context.Context, step *ExecutionStep, state *ExecutionState) error {
	stepResult := &StepResult{
		StepID:    step.ID,
		Status:    StepStatusRunning,
		StartTime: time.Now(),
		Attempts:  1,
	}

	// Track step execution in state
	state.CurrentStepID = step.ID
	state.SetStepResult(step.ID, stepResult)
	state.AddLogEntry(step.ID, "step_execution_started", map[string]interface{}{
		"step_type": step.Type,
		"step_name": step.Name,
	}, LogLevelInfo)

	// Execute step with retry logic
	var lastErr error
	maxAttempts := 1
	if step.RetryPolicy != nil {
		maxAttempts = step.RetryPolicy.MaxAttempts
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		stepResult.Attempts = attempt + 1

		// Apply step timeout
		stepCtx := ctx
		if e.config.StepTimeout > 0 {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(ctx, e.config.StepTimeout)
			defer cancel()
		}

		// Execute the step
		result, err := e.performStepExecution(stepCtx, step, state)
		if err == nil {
			// Step succeeded
			stepResult.Status = StepStatusSuccess
			stepResult.Result = result
			stepResult.EndTime = time.Now()
			stepResult.Duration = stepResult.EndTime.Sub(stepResult.StartTime)

			state.SetStepResult(step.ID, stepResult)
			state.AddLogEntry(step.ID, "step_execution_completed", map[string]interface{}{
				"duration": stepResult.Duration.String(),
				"attempts": stepResult.Attempts,
			}, LogLevelInfo)

			return nil
		}

		lastErr = err

		// Check if we should retry
		if attempt < maxAttempts-1 && step.RetryPolicy != nil {
			// Calculate backoff time
			backoffTime := step.RetryPolicy.BackoffTime
			if step.RetryPolicy.Exponential {
				backoffTime = time.Duration(int64(backoffTime) * int64(attempt+1))
			}

			state.AddLogEntry(step.ID, "step_retry", map[string]interface{}{
				"attempt":     attempt + 1,
				"max_attempts": maxAttempts,
				"backoff":     backoffTime.String(),
				"error":       err.Error(),
			}, LogLevelWarn)

			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffTime):
				// Continue to next attempt
			}
		}
	}

	// Step failed after all attempts
	stepResult.Status = StepStatusFailed
	stepResult.Error = lastErr.Error()
	stepResult.EndTime = time.Now()
	stepResult.Duration = stepResult.EndTime.Sub(stepResult.StartTime)

	state.SetStepResult(step.ID, stepResult)
	state.AddLogEntry(step.ID, "step_execution_failed", map[string]interface{}{
		"error":    lastErr.Error(),
		"attempts": stepResult.Attempts,
		"duration": stepResult.Duration.String(),
	}, LogLevelError)

	return lastErr
}

// performStepExecution performs the actual step execution based on step type
func (e *ExecutorAgent) performStepExecution(ctx context.Context, step *ExecutionStep, state *ExecutionState) (interface{}, error) {
	// Resolve step parameters
	resolvedParams, err := e.resolver.ResolveParameters(ctx, step)
	if err != nil {
		return nil, fmt.Errorf("parameter resolution failed: %w", err)
	}

	switch step.Type {
	case StepTypeToolCall:
		return e.executeToolCall(ctx, step, resolvedParams)

	case StepTypeDataProcessing:
		return e.executeDataProcessing(ctx, step, resolvedParams, state)

	case StepTypeCondition:
		return e.executeCondition(ctx, step, resolvedParams, state)

	case StepTypeSupervisor:
		return e.executeSupervisorCall(ctx, step, resolvedParams, state)

	case StepTypeDataStore:
		return e.executeDataStore(ctx, step, resolvedParams, state)

	default:
		return nil, fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// executeToolCall executes an MCP tool call
func (e *ExecutorAgent) executeToolCall(ctx context.Context, step *ExecutionStep, params map[string]interface{}) (interface{}, error) {
	if step.Tool == "" {
		return nil, fmt.Errorf("tool name is required for tool_call step")
	}

	client, err := e.mcpManager.GetClient(step.Tool)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP client '%s': %w", step.Tool, err)
	}

	operation := step.Operation
	if operation == "" {
		operation = "default"
	}

	result, err := client.Call(ctx, operation, params)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	return result, nil
}

// executeDataProcessing executes data processing operations
func (e *ExecutorAgent) executeDataProcessing(ctx context.Context, step *ExecutionStep, params map[string]interface{}, state *ExecutionState) (interface{}, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation parameter is required for data_processing step")
	}

	switch operation {
	case "merge":
		return e.mergeData(params)
	case "filter":
		return e.filterData(params)
	case "transform":
		return e.transformData(params)
	case "aggregate":
		return e.aggregateData(params)
	default:
		return nil, fmt.Errorf("unknown data processing operation: %s", operation)
	}
}

// executeCondition executes conditional logic
func (e *ExecutorAgent) executeCondition(ctx context.Context, step *ExecutionStep, params map[string]interface{}, state *ExecutionState) (interface{}, error) {
	condition, ok := params["condition"].(string)
	if !ok {
		return nil, fmt.Errorf("condition parameter is required for condition step")
	}

	// Simple condition evaluation (in a real implementation, you might use a more sophisticated expression evaluator)
	result := e.evaluateSimpleCondition(condition, params)

	return map[string]interface{}{
		"condition":     condition,
		"result":        result,
		"evaluated_at":  time.Now(),
	}, nil
}

// executeSupervisorCall executes a supervisor call
func (e *ExecutorAgent) executeSupervisorCall(ctx context.Context, step *ExecutionStep, params map[string]interface{}, state *ExecutionState) (interface{}, error) {
	reason, ok := params["reason"].(string)
	if !ok {
		reason = "Manual supervisor intervention requested"
	}

	// Create supervisor request
	_ = SupervisorRequest{
		PlanID:      state.Plan.ID,
		CurrentStep: step,
		ExecutionState: state,
		Context:     params,
	}

	// For now, we'll simulate supervisor response
	// In a real implementation, this would call an actual supervisor agent
	response := SupervisorResponse{
		Action: ActionContinue,
		Reason: fmt.Sprintf("Supervisor reviewed step %s: %s", step.ID, reason),
	}

	return response, nil
}

// executeDataStore executes data store operations
func (e *ExecutorAgent) executeDataStore(ctx context.Context, step *ExecutionStep, params map[string]interface{}, state *ExecutionState) (interface{}, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation parameter is required for data_store step")
	}

	key, ok := params["key"].(string)
	if !ok {
		return nil, fmt.Errorf("key parameter is required for data_store step")
	}

	switch operation {
	case "store":
		value := params["value"]
		state.DataStore[key] = value
		return map[string]interface{}{
			"operation": "store",
			"key":       key,
			"stored_at": time.Now(),
		}, nil

	case "retrieve":
		value, exists := state.DataStore[key]
		if !exists {
			return nil, fmt.Errorf("key '%s' not found in data store", key)
		}
		return value, nil

	default:
		return nil, fmt.Errorf("unknown data store operation: %s", operation)
	}
}

// Helper methods for data processing operations

func (e *ExecutorAgent) mergeData(params map[string]interface{}) (interface{}, error) {
	// Simple merge implementation
	result := make(map[string]interface{})
	for key, value := range params {
		if key != "operation" {
			result[key] = value
		}
	}
	return result, nil
}

func (e *ExecutorAgent) filterData(params map[string]interface{}) (interface{}, error) {
	// Simple filter implementation (placeholder)
	return params, nil
}

func (e *ExecutorAgent) transformData(params map[string]interface{}) (interface{}, error) {
	// Simple transform implementation (placeholder)
	return params, nil
}

func (e *ExecutorAgent) aggregateData(params map[string]interface{}) (interface{}, error) {
	// Simple aggregate implementation (placeholder)
	return params, nil
}

func (e *ExecutorAgent) evaluateSimpleCondition(condition string, params map[string]interface{}) bool {
	// Very simple condition evaluation (placeholder)
	// In a real implementation, you'd use a proper expression evaluator
	return true
}