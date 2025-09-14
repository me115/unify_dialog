package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// UnifiedDialogAgent 使用混合架构实现主要的统一对话智能体
type UnifiedDialogAgent struct {
	config      *AgentConfig
	planner     *PlannerAgent
	executor    *ExecutorAgent
	supervisor  *SupervisorAgent
	mcpManager  *MCPClientManager
}

// NewUnifiedDialogAgent 创建新的统一对话智能体
func NewUnifiedDialogAgent(ctx context.Context, config *AgentConfig, plannerModel, supervisorModel model.BaseChatModel) (*UnifiedDialogAgent, error) {
	// 创建MCP客户端管理器
	mcpManager := NewMCPClientManager()

	// 根据配置注册MCP客户端
	if err := registerMCPClients(mcpManager, config.Tools); err != nil {
		return nil, fmt.Errorf("failed to register MCP clients: %w", err)
	}

	// 创建规划智能体
	planner, err := NewPlannerAgent(ctx, config.PlannerConfig, plannerModel, mcpManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create planner agent: %w", err)
	}

	// 创建执行智能体
	executor := NewExecutorAgent(config.ExecutorConfig, mcpManager)

	// 创建监督智能体
	supervisor, err := NewSupervisorAgent(ctx, config.SupervisorConfig, supervisorModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create supervisor agent: %w", err)
	}

	return &UnifiedDialogAgent{
		config:     config,
		planner:    planner,
		executor:   executor,
		supervisor: supervisor,
		mcpManager: mcpManager,
	}, nil
}

// ProcessUserInput 处理用户输入并返回响应
func (a *UnifiedDialogAgent) ProcessUserInput(ctx context.Context, userInput []*schema.Message) ([]*schema.Message, error) {
	// 初始化执行状态
	state := &ExecutionState{
		UserInput:     userInput,
		DataStore:     make(map[string]interface{}),
		StepResults:   make(map[string]*StepResult),
		ExecutionLog:  make([]ExecutionLogEntry, 0),
		MaxIterations: a.config.MaxIterations,
	}

	// 如果配置了全局超时，应用全局超时
	if a.config.GlobalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.config.GlobalTimeout)
		defer cancel()
	}

	// 记录处理开始
	state.AddLogEntry("", "user_input_processing_started", map[string]interface{}{
		"input_count": len(userInput),
		"user_query":  a.extractUserQuery(userInput),
	}, LogLevelInfo)

	// 阶段1：规划
	plan, err := a.executePhase1Planning(ctx, userInput, state)
	if err != nil {
		return nil, fmt.Errorf("planning phase failed: %w", err)
	}

	// 阶段2：在监督下执行
	response, err := a.executePhase2ExecutionWithSupervision(ctx, plan, state)
	if err != nil {
		return nil, fmt.Errorf("execution phase failed: %w", err)
	}

	// 记录处理完成
	state.AddLogEntry("", "user_input_processing_completed", map[string]interface{}{
		"total_steps":       len(plan.Steps),
		"completed_steps":   len(state.GetCompletedSteps()),
		"total_iterations":  state.Iteration,
		"response_length":   len(response),
	}, LogLevelInfo)

	return response, nil
}

// executePhase1Planning 执行规划阶段
func (a *UnifiedDialogAgent) executePhase1Planning(ctx context.Context, userInput []*schema.Message, state *ExecutionState) (*ExecutionPlan, error) {
	state.AddLogEntry("", "planning_phase_started", nil, LogLevelInfo)

	plan, err := a.planner.GeneratePlan(ctx, userInput)
	if err != nil {
		state.AddLogEntry("", "planning_phase_failed", map[string]interface{}{
			"error": err.Error(),
		}, LogLevelError)
		return nil, err
	}

	state.AddLogEntry("", "planning_phase_completed", map[string]interface{}{
		"plan_id":    plan.ID,
		"step_count": len(plan.Steps),
		"goal":       plan.Goal,
	}, LogLevelInfo)

	return plan, nil
}

// executePhase2ExecutionWithSupervision 在监督者监督下执行执行阶段
func (a *UnifiedDialogAgent) executePhase2ExecutionWithSupervision(ctx context.Context, plan *ExecutionPlan, state *ExecutionState) ([]*schema.Message, error) {
	state.AddLogEntry("", "execution_phase_started", map[string]interface{}{
		"plan_id": plan.ID,
	}, LogLevelInfo)

	// 带有监督者干预的主执行循环
	for {
		// 执行计划
		err := a.executor.ExecutePlan(ctx, plan, state)

		if err == nil {
			// 执行成功完成
			state.AddLogEntry("", "execution_phase_completed", map[string]interface{}{
				"plan_id": plan.ID,
				"status":  "success",
			}, LogLevelInfo)
			break
		}

		// 检查监督者是否应该干预
		shouldIntervene := a.supervisor.ShouldIntervene(ctx, state.CurrentStepID, err, state)
		if !shouldIntervene {
			// 执行失败，监督者不会干预
			state.AddLogEntry("", "execution_phase_failed", map[string]interface{}{
				"plan_id": plan.ID,
				"error":   err.Error(),
			}, LogLevelError)
			return nil, err
		}

		// PHASE 3: SUPERVISOR INTERVENTION
		decision, err := a.executePhase3SupervisorIntervention(ctx, plan, state, err)
		if err != nil {
			return nil, fmt.Errorf("supervisor intervention failed: %w", err)
		}

		// Apply supervisor decision
		shouldContinue, err := a.applySupervisorDecision(ctx, decision, plan, state)
		if err != nil {
			return nil, fmt.Errorf("failed to apply supervisor decision: %w", err)
		}

		if !shouldContinue {
			break
		}
	}

	// Generate final response
	return a.generateFinalResponse(ctx, plan, state)
}

// executePhase3SupervisorIntervention executes supervisor intervention
func (a *UnifiedDialogAgent) executePhase3SupervisorIntervention(ctx context.Context, plan *ExecutionPlan, state *ExecutionState, executionError error) (*SupervisorResponse, error) {
	state.AddLogEntry("", "supervisor_intervention_started", map[string]interface{}{
		"reason": "execution_error",
		"error":  executionError.Error(),
	}, LogLevelInfo)

	// Create supervisor request
	request := &SupervisorRequest{
		PlanID:         plan.ID,
		ExecutionState: state,
		ErrorContext: &ErrorContext{
			FailedStepID: state.CurrentStepID,
			ErrorMessage: executionError.Error(),
			ErrorType:    categorizeError(executionError),
			RetryCount:   a.getRetryCount(state.CurrentStepID, state),
			PossibleActions: []string{
				string(ActionContinue),
				string(ActionRetry),
				string(ActionSkip),
				string(ActionReplan),
				string(ActionAbort),
			},
		},
		AvailableActions: a.generateAvailableActions(state),
	}

	// Get current step if available
	if state.CurrentStepID != "" {
		for _, step := range plan.Steps {
			if step.ID == state.CurrentStepID {
				request.CurrentStep = &step
				break
			}
		}
	}

	// Make supervisor decision
	decision, err := a.supervisor.MakeDecision(ctx, request)
	if err != nil {
		state.AddLogEntry("", "supervisor_intervention_failed", map[string]interface{}{
			"error": err.Error(),
		}, LogLevelError)
		return nil, err
	}

	state.AddLogEntry("", "supervisor_intervention_completed", map[string]interface{}{
		"action": decision.Action,
		"reason": decision.Reason,
	}, LogLevelInfo)

	return decision, nil
}

// applySupervisorDecision applies the supervisor's decision
func (a *UnifiedDialogAgent) applySupervisorDecision(ctx context.Context, decision *SupervisorResponse, plan *ExecutionPlan, state *ExecutionState) (bool, error) {
	state.AddLogEntry("", "applying_supervisor_decision", map[string]interface{}{
		"action": decision.Action,
	}, LogLevelInfo)

	switch decision.Action {
	case ActionContinue:
		// Continue with normal execution
		return true, nil

	case ActionRetry:
		// Reset the failed step and retry
		if decision.StepID != "" {
			if result, exists := state.StepResults[decision.StepID]; exists {
				result.Status = StepStatusPending
				result.Error = ""
			}
		}
		return true, nil

	case ActionSkip:
		// Mark the step as skipped and continue
		if decision.StepID != "" {
			state.SetStepResult(decision.StepID, &StepResult{
				StepID:    decision.StepID,
				Status:    StepStatusSkipped,
				StartTime: time.Now(),
				EndTime:   time.Now(),
			})
		}
		return true, nil

	case ActionModifyStep:
		// Modify step parameters and retry
		if decision.StepID != "" && decision.Parameters != nil {
			// Find and modify the step
			for i, step := range plan.Steps {
				if step.ID == decision.StepID {
					// Update parameters
					for key, value := range decision.Parameters {
						plan.Steps[i].Parameters[key] = value
					}
					break
				}
			}
			// Reset step status for retry
			if result, exists := state.StepResults[decision.StepID]; exists {
				result.Status = StepStatusPending
				result.Error = ""
			}
		}
		return true, nil

	case ActionAddStep:
		// Add new step to the plan
		if decision.NewStep != nil {
			plan.Steps = append(plan.Steps, *decision.NewStep)
		}
		return true, nil

	case ActionReplan:
		// Generate a new plan (simplified - in practice, you'd call the planner again)
		if decision.NewPlan != nil {
			*plan = *decision.NewPlan
			// Reset execution state for the new plan
			state.StepResults = make(map[string]*StepResult)
			state.CurrentStepID = ""
		}
		return true, nil

	case ActionAskUser:
		// For now, we'll just log this and continue
		// In a real implementation, you'd prompt the user for input
		state.AddLogEntry("", "user_intervention_requested", map[string]interface{}{
			"message": decision.UserMessage,
		}, LogLevelWarn)
		return true, nil

	case ActionAbort:
		// Stop execution
		state.AddLogEntry("", "execution_aborted_by_supervisor", map[string]interface{}{
			"reason": decision.Reason,
		}, LogLevelError)
		return false, nil

	case ActionComplete:
		// Mark execution as complete
		state.AddLogEntry("", "execution_completed_by_supervisor", map[string]interface{}{
			"reason": decision.Reason,
		}, LogLevelInfo)
		return false, nil

	default:
		return false, fmt.Errorf("unknown supervisor action: %s", decision.Action)
	}
}

// generateFinalResponse generates the final response to the user
func (a *UnifiedDialogAgent) generateFinalResponse(ctx context.Context, plan *ExecutionPlan, state *ExecutionState) ([]*schema.Message, error) {
	// Collect results from successful steps
	var resultParts []string

	// Add plan goal
	resultParts = append(resultParts, fmt.Sprintf("Goal: %s", plan.Goal))

	// Add execution summary
	completedSteps := state.GetCompletedSteps()
	resultParts = append(resultParts, fmt.Sprintf("Completed %d out of %d steps successfully.", len(completedSteps), len(plan.Steps)))

	// Add key results
	if len(state.StepResults) > 0 {
		resultParts = append(resultParts, "\nKey Results:")
		for stepID, result := range state.StepResults {
			if result.Status == StepStatusSuccess && result.Result != nil {
				// Find step name for better presentation
				stepName := stepID
				for _, step := range plan.Steps {
					if step.ID == stepID {
						stepName = step.Name
						break
					}
				}
				resultParts = append(resultParts, fmt.Sprintf("- %s: Completed successfully", stepName))
			}
		}
	}

	// Create response message
	response := &schema.Message{
		Role:    schema.Assistant,
		Content: fmt.Sprintf("I have processed your request with the following results:\n\n%s", fmt.Sprintf("%s", resultParts)),
	}

	return []*schema.Message{response}, nil
}

// Helper methods

func (a *UnifiedDialogAgent) extractUserQuery(userInput []*schema.Message) string {
	for _, msg := range userInput {
		if msg.Role == schema.User {
			return msg.Content
		}
	}
	return ""
}

func (a *UnifiedDialogAgent) getRetryCount(stepID string, state *ExecutionState) int {
	if result, exists := state.StepResults[stepID]; exists {
		return result.Attempts
	}
	return 0
}

func (a *UnifiedDialogAgent) generateAvailableActions(state *ExecutionState) []SupervisorAction {
	return []SupervisorAction{
		{Action: ActionContinue, Description: "Continue with normal execution"},
		{Action: ActionRetry, Description: "Retry the failed step"},
		{Action: ActionSkip, Description: "Skip the failed step and continue"},
		{Action: ActionReplan, Description: "Generate a new execution plan"},
		{Action: ActionAbort, Description: "Abort execution"},
	}
}

func categorizeError(err error) ErrorType {
	// Simple error categorization based on error message
	errMsg := err.Error()
	switch {
	case contains(errMsg, "timeout", "network", "connection"):
		return ErrorTypeTransient
	case contains(errMsg, "parameter", "validation", "invalid"):
		return ErrorTypeSemantic
	case contains(errMsg, "auth", "permission", "not found"):
		return ErrorTypePermanent
	default:
		return ErrorTypeSemantic
	}
}

func contains(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if fmt.Sprintf("%s", text) == keyword {
			return true
		}
	}
	return false
}

// registerMCPClients registers MCP clients based on configuration
func registerMCPClients(manager *MCPClientManager, toolConfigs []MCPToolConfig) error {
	for _, config := range toolConfigs {
		var client MCPClient

		// Create client based on tool type
		switch config.Name {
		case "database":
			client = NewDatabaseMCPClient(config)
		case "api":
			client = NewAPIMCPClient(config)
		case "email":
			client = NewEmailMCPClient(config)
		case "filesystem":
			client = NewFileSystemMCPClient(config)
		default:
			// Create a generic client
			client = NewBaseMCPClient(config, func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{
					"message":   fmt.Sprintf("Tool %s executed operation %s", config.Name, operation),
					"operation": operation,
					"parameters": parameters,
					"timestamp": time.Now(),
				}, nil
			})
		}

		manager.RegisterClient(config.Name, client)
	}

	return nil
}

// GetMCPManager returns the MCP client manager (for external access)
func (a *UnifiedDialogAgent) GetMCPManager() *MCPClientManager {
	return a.mcpManager
}

// GetExecutionState returns the current execution state (for debugging)
func (a *UnifiedDialogAgent) GetExecutionState() *ExecutionState {
	// Note: In a real implementation, you'd want to store and manage execution state properly
	return &ExecutionState{
		DataStore:   make(map[string]interface{}),
		StepResults: make(map[string]*StepResult),
	}
}