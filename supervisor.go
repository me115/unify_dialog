package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// SupervisorAgent 实现统一对话智能体的监督组件
type SupervisorAgent struct {
	config         *SupervisorConfig
	chatModel      model.BaseChatModel
	promptTemplate prompt.ChatTemplate
	failureCount   map[string]int // 跟踪每个步骤的失败次数以进行基于阈值的触发
}

// NewSupervisorAgent 创建新的监督智能体
func NewSupervisorAgent(ctx context.Context, config *SupervisorConfig, chatModel model.BaseChatModel) (*SupervisorAgent, error) {
	// 创建监督者提示模板
	promptTemplate := createSupervisorPrompt(config)

	return &SupervisorAgent{
		config:         config,
		chatModel:      chatModel,
		promptTemplate: promptTemplate,
		failureCount:   make(map[string]int),
	}, nil
}

// ShouldIntervene determines if the supervisor should intervene based on the current situation
func (s *SupervisorAgent) ShouldIntervene(ctx context.Context, stepID string, error error, state *ExecutionState) bool {
	// Always intervene for supervisor_call steps
	if state.CurrentStepID != "" {
		for _, step := range state.Plan.Steps {
			if step.ID == stepID && step.Type == StepTypeSupervisor {
				return true
			}
		}
	}

	// Intervene if error occurred and threshold is reached
	if error != nil {
		s.failureCount[stepID]++
		if s.failureCount[stepID] >= s.config.TriggerThreshold {
			return true
		}
	}

	// Check if state indicates supervisor intervention is needed
	if state.ErrorContext != nil && state.ErrorContext.ErrorType == ErrorTypeUserAction {
		return true
	}

	return false
}

// MakeDecision asks the supervisor to make a decision about how to proceed
func (s *SupervisorAgent) MakeDecision(ctx context.Context, request *SupervisorRequest) (*SupervisorResponse, error) {
	// Apply decision timeout if configured
	if s.config.DecisionTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.DecisionTimeout)
		defer cancel()
	}

	// Generate input messages for the supervisor
	messages, err := s.generateSupervisorInput(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to generate supervisor input: %w", err)
	}

	// Apply model options
	modelOptions := []model.Option{}
	if s.config.Temperature > 0 {
		modelOptions = append(modelOptions, model.WithTemperature(float32(s.config.Temperature)))
	}
	if s.config.MaxTokens > 0 {
		modelOptions = append(modelOptions, model.WithMaxTokens(s.config.MaxTokens))
	}

	// Generate response from the model
	response, err := s.chatModel.Generate(ctx, messages, modelOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate supervisor decision: %w", err)
	}

	// Parse the decision from the response
	decision, err := s.parseDecisionFromResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse supervisor decision: %w", err)
	}

	// Validate the decision
	if err := s.validateDecision(decision, request); err != nil {
		return nil, fmt.Errorf("invalid supervisor decision: %w", err)
	}

	return decision, nil
}

// generateSupervisorInput generates input messages for the supervisor model
func (s *SupervisorAgent) generateSupervisorInput(ctx context.Context, request *SupervisorRequest) ([]*schema.Message, error) {
	// Format the execution context
	executionContext := s.formatExecutionContext(request.ExecutionState)

	// Format available actions
	availableActions := s.formatAvailableActions(request.AvailableActions)

	// Format current error context if any
	errorContext := ""
	if request.ErrorContext != nil {
		errorContext = s.formatErrorContext(request.ErrorContext)
	}

	// Format current step information
	currentStepInfo := ""
	if request.CurrentStep != nil {
		stepJSON, err := json.MarshalIndent(request.CurrentStep, "", "  ")
		if err == nil {
			currentStepInfo = string(stepJSON)
		}
	}

	// Generate messages using the prompt template
	messages, err := s.promptTemplate.Format(ctx, map[string]interface{}{
		"plan_id":            request.PlanID,
		"current_step":       currentStepInfo,
		"execution_context":  executionContext,
		"error_context":      errorContext,
		"available_actions":  availableActions,
		"current_time":       time.Now().Format(time.RFC3339),
		"failure_threshold":  s.config.TriggerThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to format supervisor prompt: %w", err)
	}

	return messages, nil
}

// parseDecisionFromResponse parses the supervisor decision from the model response
func (s *SupervisorAgent) parseDecisionFromResponse(content string) (*SupervisorResponse, error) {
	// Extract JSON from the response (handles cases where model adds extra text)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}") + 1

	if jsonStart == -1 || jsonEnd <= jsonStart {
		// If no JSON found, try to parse the response as a simple action
		return s.parseSimpleDecision(content)
	}

	jsonContent := content[jsonStart:jsonEnd]

	// Parse the JSON into a SupervisorResponse
	var decision SupervisorResponse
	if err := json.Unmarshal([]byte(jsonContent), &decision); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decision JSON: %w", err)
	}

	return &decision, nil
}

// parseSimpleDecision parses a simple text decision (fallback)
func (s *SupervisorAgent) parseSimpleDecision(content string) (*SupervisorResponse, error) {
	content = strings.ToLower(strings.TrimSpace(content))

	// Simple keyword-based parsing
	switch {
	case strings.Contains(content, "continue"):
		return &SupervisorResponse{
			Action: ActionContinue,
			Reason: "Supervisor decided to continue execution",
		}, nil
	case strings.Contains(content, "retry"):
		return &SupervisorResponse{
			Action: ActionRetry,
			Reason: "Supervisor decided to retry the step",
		}, nil
	case strings.Contains(content, "skip"):
		return &SupervisorResponse{
			Action: ActionSkip,
			Reason: "Supervisor decided to skip the step",
		}, nil
	case strings.Contains(content, "abort"):
		return &SupervisorResponse{
			Action: ActionAbort,
			Reason: "Supervisor decided to abort execution",
		}, nil
	case strings.Contains(content, "replan"):
		return &SupervisorResponse{
			Action: ActionReplan,
			Reason: "Supervisor decided to replan",
		}, nil
	default:
		return &SupervisorResponse{
			Action: ActionContinue,
			Reason: "Default action - continue execution",
		}, nil
	}
}

// validateDecision validates the supervisor decision
func (s *SupervisorAgent) validateDecision(decision *SupervisorResponse, request *SupervisorRequest) error {
	// Check if action is valid
	validActions := []ActionType{
		ActionContinue, ActionRetry, ActionSkip, ActionAbort,
		ActionModifyStep, ActionAddStep, ActionReplan, ActionAskUser, ActionComplete,
	}

	isValidAction := false
	for _, validAction := range validActions {
		if decision.Action == validAction {
			isValidAction = true
			break
		}
	}

	if !isValidAction {
		return fmt.Errorf("invalid action: %s", decision.Action)
	}

	// Validate action-specific requirements
	switch decision.Action {
	case ActionModifyStep, ActionRetry:
		if decision.StepID == "" && request.CurrentStep != nil {
			decision.StepID = request.CurrentStep.ID
		}
		if decision.StepID == "" {
			return fmt.Errorf("step_id is required for action: %s", decision.Action)
		}

	case ActionAddStep:
		if decision.NewStep == nil {
			return fmt.Errorf("new_step is required for action: %s", decision.Action)
		}

	case ActionReplan:
		// No specific validation needed for replan
	}

	return nil
}

// formatExecutionContext formats the execution context for the supervisor prompt
func (s *SupervisorAgent) formatExecutionContext(state *ExecutionState) string {
	var parts []string

	// Basic execution info
	parts = append(parts, fmt.Sprintf("Iteration: %d/%d", state.Iteration, state.MaxIterations))
	parts = append(parts, fmt.Sprintf("Current Step: %s", state.CurrentStepID))

	// Step results summary
	if len(state.StepResults) > 0 {
		parts = append(parts, "\nStep Results:")
		for stepID, result := range state.StepResults {
			status := result.Status
			duration := result.Duration.String()
			attempts := result.Attempts

			summary := fmt.Sprintf("  %s: %s (duration: %s, attempts: %d)", stepID, status, duration, attempts)
			if result.Error != "" {
				summary += fmt.Sprintf(" - Error: %s", result.Error)
			}
			parts = append(parts, summary)
		}
	}

	// Recent log entries
	if len(state.ExecutionLog) > 0 {
		parts = append(parts, "\nRecent Log Entries:")
		// Show last 10 log entries
		start := 0
		if len(state.ExecutionLog) > 10 {
			start = len(state.ExecutionLog) - 10
		}

		for i := start; i < len(state.ExecutionLog); i++ {
			entry := state.ExecutionLog[i]
			timestamp := entry.Timestamp.Format("15:04:05")
			logLine := fmt.Sprintf("  [%s] %s: %s (%s)", timestamp, entry.StepID, entry.Event, entry.Level)
			parts = append(parts, logLine)
		}
	}

	return strings.Join(parts, "\n")
}

// formatErrorContext formats error context for the supervisor prompt
func (s *SupervisorAgent) formatErrorContext(errorCtx *ErrorContext) string {
	if errorCtx == nil {
		return "No error context available."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Failed Step: %s", errorCtx.FailedStepID))
	parts = append(parts, fmt.Sprintf("Error Type: %s", errorCtx.ErrorType))
	parts = append(parts, fmt.Sprintf("Error Message: %s", errorCtx.ErrorMessage))
	parts = append(parts, fmt.Sprintf("Retry Count: %d", errorCtx.RetryCount))

	if len(errorCtx.PossibleActions) > 0 {
		parts = append(parts, fmt.Sprintf("Possible Actions: %s", strings.Join(errorCtx.PossibleActions, ", ")))
	}

	return strings.Join(parts, "\n")
}

// formatAvailableActions formats available actions for the supervisor prompt
func (s *SupervisorAgent) formatAvailableActions(actions []SupervisorAction) string {
	if len(actions) == 0 {
		return "No specific actions provided."
	}

	var parts []string
	for _, action := range actions {
		actionDesc := fmt.Sprintf("- %s: %s", action.Action, action.Description)
		if action.StepID != "" {
			actionDesc += fmt.Sprintf(" (Step: %s)", action.StepID)
		}
		parts = append(parts, actionDesc)
	}

	return strings.Join(parts, "\n")
}

// createSupervisorPrompt creates the prompt template for the supervisor
func createSupervisorPrompt(config *SupervisorConfig) prompt.ChatTemplate {
	systemPrompt := config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultSupervisorSystemPrompt
	}

	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(`## EXECUTION CONTEXT
Plan ID: {plan_id}
Current Time: {current_time}
Failure Threshold: {failure_threshold}

## CURRENT STEP
{current_step}

## EXECUTION STATE
{execution_context}

## ERROR CONTEXT
{error_context}

## AVAILABLE ACTIONS
{available_actions}

Based on the above information, please analyze the situation and provide your decision as a JSON object with the following structure:

{
  "action": "continue|retry|skip|abort|modify_step|add_step|replan|ask_user|complete",
  "reason": "Clear explanation of your decision",
  "step_id": "step_id_if_applicable",
  "parameters": {},
  "new_step": {},
  "new_plan": {},
  "user_message": "message_for_user_if_applicable"
}

Consider the execution state, error context, and available actions to make the best decision for proceeding with the plan execution.`),
	)
}

const defaultSupervisorSystemPrompt = `You are an intelligent execution supervisor responsible for monitoring and guiding the execution of complex multi-step plans. Your role is to analyze execution state, handle errors, and make strategic decisions to ensure successful plan completion.

## YOUR RESPONSIBILITIES

1. **Monitor Execution**: Track the progress of plan execution and identify issues
2. **Error Analysis**: Analyze failures and determine appropriate recovery strategies
3. **Decision Making**: Choose the best course of action when intervention is needed
4. **Plan Adaptation**: Modify or replan when necessary to achieve goals
5. **User Communication**: Communicate with users when human intervention is required

## AVAILABLE ACTIONS

- **continue**: Proceed with normal execution
- **retry**: Retry the current failed step (with or without modifications)
- **skip**: Skip the current step and continue with dependent steps
- **abort**: Stop execution entirely (use only for unrecoverable situations)
- **modify_step**: Modify step parameters and retry
- **add_step**: Add a new step to the plan
- **replan**: Generate a new plan from the current state
- **ask_user**: Request user input or intervention
- **complete**: Mark the plan as completed

## DECISION CRITERIA

When making decisions, consider:

1. **Error Severity**: Is this a transient error or a fundamental issue?
2. **Retry Potential**: Would retrying the step likely succeed?
3. **Impact Assessment**: How critical is this step to the overall goal?
4. **Alternative Paths**: Are there alternative ways to achieve the goal?
5. **Resource Efficiency**: What's the most efficient path forward?
6. **User Experience**: How does this affect the user's experience?

## ERROR CLASSIFICATION

- **Transient**: Network issues, timeouts, rate limits → Often retry
- **Semantic**: Invalid parameters, validation errors → Often modify_step
- **Permanent**: Authentication failures, not found → Often skip or replan
- **User Action**: Requires user input → Always ask_user

## QUALITY PRINCIPLES

- **Minimize User Interruption**: Only ask for user input when absolutely necessary
- **Preserve Progress**: Avoid losing completed work when possible
- **Be Decisive**: Make clear, actionable decisions
- **Explain Reasoning**: Always provide clear reasoning for decisions
- **Consider Context**: Factor in the full execution context, not just the current error

Make intelligent, context-aware decisions that maximize the likelihood of successful plan completion while minimizing user disruption.`