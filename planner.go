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

// PlannerAgent 实现统一对话智能体的规划组件
type PlannerAgent struct {
	config      *PlannerConfig
	chatModel   model.BaseChatModel
	mcpManager  *MCPClientManager
	genInputFn  func(ctx context.Context, userInput []*schema.Message) ([]*schema.Message, error)
	promptTemplate prompt.ChatTemplate
}

// NewPlannerAgent 创建新的规划智能体
func NewPlannerAgent(ctx context.Context, config *PlannerConfig, chatModel model.BaseChatModel, mcpManager *MCPClientManager) (*PlannerAgent, error) {
	// 创建规划器提示模板
	promptTemplate := createPlannerPrompt(config, mcpManager)

	planner := &PlannerAgent{
		config:         config,
		chatModel:      chatModel,
		mcpManager:     mcpManager,
		promptTemplate: promptTemplate,
	}

	// Set default input function
	planner.genInputFn = planner.defaultGenInputFn

	return planner, nil
}

// GeneratePlan generates an execution plan based on user input
func (p *PlannerAgent) GeneratePlan(ctx context.Context, userInput []*schema.Message) (*ExecutionPlan, error) {
	// Generate input messages for the model
	messages, err := p.genInputFn(ctx, userInput)
	if err != nil {
		return nil, fmt.Errorf("failed to generate planner input: %w", err)
	}

	// Apply model options
	modelOptions := []model.Option{}
	if p.config.Temperature > 0 {
		modelOptions = append(modelOptions, model.WithTemperature(float32(p.config.Temperature)))
	}
	if p.config.MaxTokens > 0 {
		modelOptions = append(modelOptions, model.WithMaxTokens(p.config.MaxTokens))
	}

	// Apply timeout if configured
	if p.config.PlanningTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.config.PlanningTimeout)
		defer cancel()
	}

	// Generate response from the model
	response, err := p.chatModel.Generate(ctx, messages, modelOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan: %w", err)
	}

	// Parse the plan from the response
	plan, err := p.parsePlanFromResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	// Set plan metadata
	plan.ID = fmt.Sprintf("plan_%d", time.Now().Unix())
	plan.CreatedAt = time.Now()

	// Validate the plan
	if err := p.validatePlan(plan); err != nil {
		return nil, fmt.Errorf("plan validation failed: %w", err)
	}

	return plan, nil
}

// defaultGenInputFn generates the input messages for the planner
func (p *PlannerAgent) defaultGenInputFn(ctx context.Context, userInput []*schema.Message) ([]*schema.Message, error) {
	// Get available tools information
	availableTools, err := p.getAvailableToolsDescription(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools description: %w", err)
	}

	// Format user input for the prompt
	userInputText := p.formatUserInput(userInput)

	// Generate messages using the prompt template
	messages, err := p.promptTemplate.Format(ctx, map[string]interface{}{
		"user_input":       userInputText,
		"available_tools":  availableTools,
		"example_plans":    p.formatExamplePlans(),
		"current_time":     time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to format prompt: %w", err)
	}

	return messages, nil
}

// parsePlanFromResponse parses the execution plan from the model response
func (p *PlannerAgent) parsePlanFromResponse(content string) (*ExecutionPlan, error) {
	// Extract JSON from the response (handles cases where model adds extra text)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}") + 1

	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in response: %s", content)
	}

	jsonContent := content[jsonStart:jsonEnd]

	// Parse the JSON into an ExecutionPlan
	var plan ExecutionPlan
	if err := json.Unmarshal([]byte(jsonContent), &plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan JSON: %w", err)
	}

	return &plan, nil
}

// validatePlan validates the generated execution plan
func (p *PlannerAgent) validatePlan(plan *ExecutionPlan) error {
	if len(plan.Steps) == 0 {
		return fmt.Errorf("plan must contain at least one step")
	}

	stepIDs := make(map[string]bool)
	for _, step := range plan.Steps {
		// Check for duplicate step IDs
		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		stepIDs[step.ID] = true

		// Validate step dependencies
		for _, depID := range step.Dependencies {
			if !stepIDs[depID] && !p.isValidDependency(depID, plan.Steps) {
				return fmt.Errorf("step %s depends on non-existent step: %s", step.ID, depID)
			}
		}

		// Validate step tools
		if step.Type == StepTypeToolCall && step.Tool != "" {
			if _, err := p.mcpManager.GetClient(step.Tool); err != nil {
				return fmt.Errorf("step %s references unknown tool: %s", step.ID, step.Tool)
			}
		}
	}

	// Check for circular dependencies
	if err := p.checkCircularDependencies(plan); err != nil {
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	return nil
}

// isValidDependency checks if a dependency ID is valid (exists in the plan)
func (p *PlannerAgent) isValidDependency(depID string, steps []ExecutionStep) bool {
	for _, step := range steps {
		if step.ID == depID {
			return true
		}
	}
	return false
}

// checkCircularDependencies checks for circular dependencies in the plan
func (p *PlannerAgent) checkCircularDependencies(plan *ExecutionPlan) error {
	// Build dependency graph
	graph := make(map[string][]string)
	for _, step := range plan.Steps {
		graph[step.ID] = step.Dependencies
	}

	// Check each step for circular dependencies using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(stepID string) bool
	hasCycle = func(stepID string) bool {
		visited[stepID] = true
		recStack[stepID] = true

		for _, dep := range graph[stepID] {
			if !visited[dep] && hasCycle(dep) {
				return true
			} else if recStack[dep] {
				return true
			}
		}

		recStack[stepID] = false
		return false
	}

	for stepID := range graph {
		if !visited[stepID] && hasCycle(stepID) {
			return fmt.Errorf("circular dependency detected involving step: %s", stepID)
		}
	}

	return nil
}

// getAvailableToolsDescription returns a description of available tools
func (p *PlannerAgent) getAvailableToolsDescription(ctx context.Context) (string, error) {
	clients := p.mcpManager.GetAllClients()
	if len(clients) == 0 {
		return "No tools available.", nil
	}

	var descriptions []string
	for name, client := range clients {
		toolInfo, err := client.GetToolInfo(ctx)
		if err != nil {
			continue // Skip clients that can't provide tool info
		}

		description := fmt.Sprintf("- %s: %s", name, toolInfo.Desc)
		descriptions = append(descriptions, description)
	}

	return strings.Join(descriptions, "\n"), nil
}

// formatUserInput formats user input messages into a string
func (p *PlannerAgent) formatUserInput(userInput []*schema.Message) string {
	var parts []string
	for _, msg := range userInput {
		role := string(msg.Role)
		content := msg.Content
		parts = append(parts, fmt.Sprintf("[%s]: %s", role, content))
	}
	return strings.Join(parts, "\n")
}

// formatExamplePlans formats example plans for the prompt
func (p *PlannerAgent) formatExamplePlans() string {
	if len(p.config.ExamplePlans) == 0 {
		return "No example plans available."
	}

	var examples []string
	for i, plan := range p.config.ExamplePlans {
		planJSON, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			continue
		}

		example := fmt.Sprintf("Example %d:\n%s", i+1, string(planJSON))
		examples = append(examples, example)
	}

	return strings.Join(examples, "\n\n")
}

// createPlannerPrompt creates the prompt template for the planner
func createPlannerPrompt(config *PlannerConfig, mcpManager *MCPClientManager) prompt.ChatTemplate {
	systemPrompt := config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultPlannerSystemPrompt
	}

	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(`## USER REQUEST
{user_input}

## AVAILABLE TOOLS
{available_tools}

## EXAMPLE PLANS
{example_plans}

## CURRENT TIME
{current_time}

Please analyze the user request and create a comprehensive execution plan. Return your response as a JSON object following the ExecutionPlan schema.

Remember to:
1. Break down complex tasks into smaller, manageable steps
2. Identify dependencies between steps
3. Use appropriate tools for each step
4. Include error handling strategies
5. Consider parallel execution where possible
6. Use parameter references (e.g., {"$ref": "steps.step1.result"}) to pass data between steps

Return only the JSON object, no additional text.`),
	)
}

const defaultPlannerSystemPrompt = `You are an expert AI planning agent specialized in creating comprehensive execution plans for complex tasks. Your role is to analyze user requests and break them down into a series of well-structured, executable steps.

## YOUR CAPABILITIES

You have access to various MCP (Model Context Protocol) tools that can:
- Query databases
- Make API requests
- Send emails
- Read/write files
- Process data
- And more (see available tools list)

## PLANNING PRINCIPLES

1. **Decomposition**: Break complex tasks into smaller, manageable steps
2. **Dependencies**: Identify and specify dependencies between steps
3. **Data Flow**: Use parameter references to pass data between steps
4. **Error Handling**: Include appropriate retry policies and error handling
5. **Efficiency**: Identify opportunities for parallel execution
6. **Completeness**: Ensure the plan addresses all aspects of the user's request

## EXECUTION PLAN SCHEMA

Create plans using this JSON structure:

{
  "goal": "Clear description of what this plan accomplishes",
  "steps": [
    {
      "id": "unique_step_identifier",
      "type": "tool_call|data_processing|condition|supervisor_call|data_store",
      "tool": "tool_name_for_tool_call_steps",
      "operation": "specific_operation_to_perform",
      "name": "human_readable_step_name",
      "parameters": {
        "param1": "static_value",
        "param2": {"$ref": "steps.previous_step.result"},
        "param3": {"type": "step_result", "reference": "step1", "path": "data.items[0].id"}
      },
      "dependencies": ["step_ids_this_step_depends_on"],
      "retry_policy": {
        "max_attempts": 3,
        "backoff_time": "1s",
        "exponential": true
      },
      "error_policy": "fail_fast|continue|retry|ask_user",
      "parallel": false
    }
  ]
}

## PARAMETER REFERENCE TYPES

1. **Simple Reference**: {"$ref": "steps.step_id.result"}
2. **Structured Reference**: {
   "type": "step_result|user_input|data_store|environment",
   "reference": "step_id_or_key",
   "path": "data.field[0].subfield",
   "default": "fallback_value",
   "transform": "string|json|upper|lower|length|first|last"
}

## STEP TYPES

- **tool_call**: Execute an MCP tool operation
- **data_processing**: Process, transform, or combine data
- **condition**: Conditional logic and branching
- **supervisor_call**: Request human or supervisor intervention
- **data_store**: Store intermediate results for later use

## QUALITY REQUIREMENTS

- Each step must be independently executable
- Dependencies must be clearly specified
- Parameters must use appropriate reference syntax
- Error handling must be considered
- The plan must be complete and achieve the stated goal

Generate execution plans that are robust, efficient, and comprehensive.`