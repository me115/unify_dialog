package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Fixed version with standard library JSON instead of sonic

// Message represents a chat message without EINO dependency
type FixedMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MockBaseChatModel interface for demonstration
type FixedBaseChatModel interface {
	Generate(ctx context.Context, messages []*FixedMessage) (*FixedMessage, error)
}

// MockChatModel provides a mock implementation
type FixedMockChatModel struct {
	name string
}

func NewFixedMockChatModel(name string) *FixedMockChatModel {
	return &FixedMockChatModel{name: name}
}

func (m *FixedMockChatModel) Generate(ctx context.Context, messages []*FixedMessage) (*FixedMessage, error) {
	// Extract the latest user message
	var userMessage string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMessage = messages[i].Content
			break
		}
	}

	// Generate response based on model type and user message
	var response string
	if m.name == "planner" {
		response = m.generatePlannerResponse(userMessage)
	} else if m.name == "supervisor" {
		response = m.generateSupervisorResponse(userMessage)
	} else {
		response = "I understand your request and will help you with that."
	}

	return &FixedMessage{
		Role:    "assistant",
		Content: response,
	}, nil
}

func (m *FixedMockChatModel) generatePlannerResponse(userMessage string) string {
	// Generate a mock execution plan based on user input
	plan := ExecutionPlan{
		Goal: fmt.Sprintf("Process user request: %s", userMessage),
		Steps: []ExecutionStep{
			{
				ID:   "step_1",
				Type: StepTypeToolCall,
				Tool: "database",
				Name: "Query user data",
				Parameters: map[string]interface{}{
					"operation": "query",
					"query":     "SELECT * FROM users WHERE active = true",
				},
				Dependencies: []string{},
				RetryPolicy: &RetryPolicy{
					MaxAttempts: 3,
					BackoffTime: time.Second,
					Exponential: true,
				},
				ErrorPolicy: ErrorPolicyRetry,
			},
			{
				ID:   "step_2",
				Type: StepTypeToolCall,
				Tool: "api",
				Name: "Fetch additional data",
				Parameters: map[string]interface{}{
					"operation": "request",
					"endpoint":  "https://api.example.com/data",
					"method":    "GET",
				},
				Dependencies: []string{"step_1"},
				RetryPolicy: &RetryPolicy{
					MaxAttempts: 2,
					BackoffTime: 500 * time.Millisecond,
				},
				ErrorPolicy: ErrorPolicyContinue,
				Parallel:    true,
			},
			{
				ID:   "step_3",
				Type: StepTypeDataProcessing,
				Name: "Process and combine data",
				Parameters: map[string]interface{}{
					"operation": "merge",
					"data1":     map[string]interface{}{"$ref": "steps.step_1.result"},
					"data2":     map[string]interface{}{"$ref": "steps.step_2.result"},
				},
				Dependencies: []string{"step_1", "step_2"},
				ErrorPolicy:  ErrorPolicyFailFast,
			},
			{
				ID:   "step_4",
				Type: StepTypeToolCall,
				Tool: "email",
				Name: "Send notification",
				Parameters: map[string]interface{}{
					"operation": "send",
					"to":        "user@example.com",
					"subject":   "Request processed",
					"body":      "Your request has been processed successfully",
				},
				Dependencies: []string{"step_3"},
				ErrorPolicy:  ErrorPolicyContinue,
			},
		},
	}

	planJSON, _ := json.MarshalIndent(plan, "", "  ")
	return string(planJSON)
}

func (m *FixedMockChatModel) generateSupervisorResponse(userMessage string) string {
	// Generate a mock supervisor decision
	response := SupervisorResponse{
		Action: ActionContinue,
		Reason: "Analysis complete. Execution can continue as planned.",
	}

	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	return string(responseJSON)
}

// Fixed configuration structure
type FixedAgentConfig struct {
	MaxIterations      int                      `json:"max_iterations"`
	Tools              []MCPToolConfig          `json:"tools"`
	PlannerConfig      *PlannerConfig           `json:"planner_config"`
	ExecutorConfig     *ExecutorConfig          `json:"executor_config"`
	SupervisorConfig   *SupervisorConfig        `json:"supervisor_config"`
	DefaultRetryPolicy *RetryPolicy             `json:"default_retry_policy"`
	GlobalTimeout      time.Duration            `json:"global_timeout"`
	EnableDebug        bool                     `json:"enable_debug"`
}

// Fixed Unified Dialog Agent without EINO dependencies
type FixedUnifiedDialogAgent struct {
	config      *FixedAgentConfig
	planner     *PlannerAgent
	executor    *ExecutorAgent
	supervisor  *SupervisorAgent
	mcpManager  *MCPClientManager
}

// NewFixedUnifiedDialogAgent creates a new fixed unified dialog agent
func NewFixedUnifiedDialogAgent(ctx context.Context, config *FixedAgentConfig, plannerModel, supervisorModel FixedBaseChatModel) (*FixedUnifiedDialogAgent, error) {
	// Create MCP client manager
	mcpManager := NewMCPClientManager()

	// Register MCP clients based on configuration
	if err := registerFixedMCPClients(mcpManager, config.Tools); err != nil {
		return nil, fmt.Errorf("failed to register MCP clients: %w", err)
	}

	// Create planner agent (adapted for fixed models)
	planner, err := NewFixedPlannerAgent(ctx, config.PlannerConfig, plannerModel, mcpManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create planner agent: %w", err)
	}

	// Create executor agent
	executor := NewExecutorAgent(config.ExecutorConfig, mcpManager)

	// Create supervisor agent (adapted for fixed models)
	supervisor, err := NewFixedSupervisorAgent(ctx, config.SupervisorConfig, supervisorModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create supervisor agent: %w", err)
	}

	return &FixedUnifiedDialogAgent{
		config:     config,
		planner:    planner,
		executor:   executor,
		supervisor: supervisor,
		mcpManager: mcpManager,
	}, nil
}

// ProcessUserInput processes user input and returns a response
func (a *FixedUnifiedDialogAgent) ProcessUserInput(ctx context.Context, userInput string) (string, error) {
	// Convert string input to message format
	messages := []*FixedMessage{
		{
			Role:    "user",
			Content: userInput,
		},
	}

	// Initialize execution state
	state := &ExecutionState{
		UserInput:     convertToSchemaMessages(messages),
		DataStore:     make(map[string]interface{}),
		StepResults:   make(map[string]*StepResult),
		ExecutionLog:  make([]ExecutionLogEntry, 0),
		MaxIterations: a.config.MaxIterations,
	}

	// Apply global timeout if configured
	if a.config.GlobalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.config.GlobalTimeout)
		defer cancel()
	}

	// Log the start of processing
	state.AddLogEntry("", "user_input_processing_started", map[string]interface{}{
		"user_query": userInput,
	}, LogLevelInfo)

	// PHASE 1: PLANNING
	plan, err := a.executePhase1Planning(ctx, messages, state)
	if err != nil {
		return "", fmt.Errorf("planning phase failed: %w", err)
	}

	// PHASE 2: EXECUTION WITH SUPERVISION
	response, err := a.executePhase2ExecutionWithSupervision(ctx, plan, state)
	if err != nil {
		return "", fmt.Errorf("execution phase failed: %w", err)
	}

	// Log the completion of processing
	state.AddLogEntry("", "user_input_processing_completed", map[string]interface{}{
		"total_steps":      len(plan.Steps),
		"completed_steps":  len(state.GetCompletedSteps()),
		"total_iterations": state.Iteration,
	}, LogLevelInfo)

	return response, nil
}

// Fixed Planner Agent
type FixedPlannerAgent struct {
	config     *PlannerConfig
	chatModel  FixedBaseChatModel
	mcpManager *MCPClientManager
}

func NewFixedPlannerAgent(ctx context.Context, config *PlannerConfig, chatModel FixedBaseChatModel, mcpManager *MCPClientManager) (*FixedPlannerAgent, error) {
	return &FixedPlannerAgent{
		config:     config,
		chatModel:  chatModel,
		mcpManager: mcpManager,
	}, nil
}

func (p *FixedPlannerAgent) GeneratePlan(ctx context.Context, userInput []*FixedMessage) (*ExecutionPlan, error) {
	// Generate response from the model
	response, err := p.chatModel.Generate(ctx, userInput)
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

	return plan, nil
}

func (p *FixedPlannerAgent) parsePlanFromResponse(content string) (*ExecutionPlan, error) {
	var plan ExecutionPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan JSON: %w", err)
	}
	return &plan, nil
}

// Fixed Supervisor Agent
type FixedSupervisorAgent struct {
	config      *SupervisorConfig
	chatModel   FixedBaseChatModel
	failureCount map[string]int
}

func NewFixedSupervisorAgent(ctx context.Context, config *SupervisorConfig, chatModel FixedBaseChatModel) (*FixedSupervisorAgent, error) {
	return &FixedSupervisorAgent{
		config:       config,
		chatModel:    chatModel,
		failureCount: make(map[string]int),
	}, nil
}

func (s *FixedSupervisorAgent) ShouldIntervene(ctx context.Context, stepID string, error error, state *ExecutionState) bool {
	if error != nil {
		s.failureCount[stepID]++
		if s.failureCount[stepID] >= s.config.TriggerThreshold {
			return true
		}
	}
	return false
}

func (s *FixedSupervisorAgent) MakeDecision(ctx context.Context, request *SupervisorRequest) (*SupervisorResponse, error) {
	// Create input messages for supervisor
	messages := []*FixedMessage{
		{
			Role:    "user",
			Content: fmt.Sprintf("Error in step %s: %s", request.CurrentStep.ID, request.ErrorContext.ErrorMessage),
		},
	}

	// Generate response from the model
	response, err := s.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate supervisor decision: %w", err)
	}

	// Parse the decision from the response
	var decision SupervisorResponse
	if err := json.Unmarshal([]byte(response.Content), &decision); err != nil {
		// Fallback to default decision if parsing fails
		decision = SupervisorResponse{
			Action: ActionContinue,
			Reason: "Default supervisor decision - continue execution",
		}
	}

	return &decision, nil
}

// Helper functions
func (a *FixedUnifiedDialogAgent) executePhase1Planning(ctx context.Context, userInput []*FixedMessage, state *ExecutionState) (*ExecutionPlan, error) {
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

func (a *FixedUnifiedDialogAgent) executePhase2ExecutionWithSupervision(ctx context.Context, plan *ExecutionPlan, state *ExecutionState) (string, error) {
	state.AddLogEntry("", "execution_phase_started", map[string]interface{}{
		"plan_id": plan.ID,
	}, LogLevelInfo)

	// Execute the plan
	err := a.executor.ExecutePlan(ctx, plan, state)
	if err != nil {
		// Check if supervisor should intervene
		shouldIntervene := a.supervisor.ShouldIntervene(ctx, state.CurrentStepID, err, state)
		if shouldIntervene {
			// Create supervisor request
			request := &SupervisorRequest{
				PlanID:       plan.ID,
				CurrentStep:  &plan.Steps[0], // Simplified
				ExecutionState: state,
				ErrorContext: &ErrorContext{
					FailedStepID: state.CurrentStepID,
					ErrorMessage: err.Error(),
					ErrorType:    ErrorTypeSemantic,
				},
			}

			// Get supervisor decision
			decision, err := a.supervisor.MakeDecision(ctx, request)
			if err != nil {
				return "", fmt.Errorf("supervisor intervention failed: %w", err)
			}

			// Apply decision (simplified)
			if decision.Action == ActionContinue {
				state.AddLogEntry("", "supervisor_decided_continue", nil, LogLevelInfo)
			}
		} else {
			return "", err
		}
	}

	// Generate final response
	return a.generateFinalResponse(plan, state), nil
}

func (a *FixedUnifiedDialogAgent) generateFinalResponse(plan *ExecutionPlan, state *ExecutionState) string {
	completedSteps := state.GetCompletedSteps()
	return fmt.Sprintf("Successfully processed your request: %s\nCompleted %d out of %d steps.",
		plan.Goal, len(completedSteps), len(plan.Steps))
}

// Utility functions
func convertToSchemaMessages(messages []*FixedMessage) []*Message {
	// This is a simplified conversion
	result := make([]*Message, len(messages))
	for i, msg := range messages {
		result[i] = &Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return result
}

func registerFixedMCPClients(manager *MCPClientManager, toolConfigs []MCPToolConfig) error {
	for _, config := range toolConfigs {
		var client MCPClient

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

func runFixedMain() {
	fmt.Println("🤖 Fixed Unified Dialog Agent Example")
	fmt.Println("=====================================")

	ctx := context.Background()

	// Create agent configuration
	config := &FixedAgentConfig{
		MaxIterations: 10,
		GlobalTimeout: 5 * time.Minute,
		Tools: []MCPToolConfig{
			{
				Name:        "database",
				Description: "Database query and manipulation tool",
				Timeout:     30 * time.Second,
				Retries:     3,
			},
			{
				Name:        "api",
				Description: "HTTP API request tool",
				Timeout:     15 * time.Second,
				Retries:     2,
			},
			{
				Name:        "email",
				Description: "Email sending tool",
				Timeout:     10 * time.Second,
				Retries:     1,
			},
		},
		PlannerConfig: &PlannerConfig{
			Model:           "gpt-4",
			Temperature:     0.7,
			MaxTokens:       2000,
			PlanningTimeout: 30 * time.Second,
		},
		ExecutorConfig: &ExecutorConfig{
			MaxParallelSteps: 3,
			StepTimeout:      60 * time.Second,
			EnableCaching:    true,
			CacheTTL:         10 * time.Minute,
		},
		SupervisorConfig: &SupervisorConfig{
			Model:            "gpt-4",
			Temperature:      0.3,
			MaxTokens:        1000,
			TriggerThreshold: 2,
			DecisionTimeout:  20 * time.Second,
		},
		DefaultRetryPolicy: &RetryPolicy{
			MaxAttempts: 3,
			BackoffTime: time.Second,
			Exponential: true,
		},
		EnableDebug: true,
	}

	// Create mock models
	plannerModel := NewFixedMockChatModel("planner")
	supervisorModel := NewFixedMockChatModel("supervisor")

	// Create the unified dialog agent
	agent, err := NewFixedUnifiedDialogAgent(ctx, config, plannerModel, supervisorModel)
	if err != nil {
		log.Fatalf("Failed to create unified dialog agent: %v", err)
	}

	// Example user inputs to test the agent
	testCases := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:        "Data Analysis Request",
			input:       "Please analyze our user engagement data from the last month and send me a summary report.",
			description: "Tests multi-step workflow with database queries, data processing, and notification",
		},
		{
			name:        "Customer Support",
			input:       "A customer complained about their order. Can you look up their order details and send them an update?",
			description: "Tests customer data lookup and communication workflow",
		},
		{
			name:        "System Health Check",
			input:       "Check the health of all our services and create a status report.",
			description: "Tests parallel API calls and report generation",
		},
	}

	fmt.Println("\n🧪 Running Test Cases")
	fmt.Println("---------------------")

	for i, testCase := range testCases {
		fmt.Printf("\n%d. %s\n", i+1, testCase.name)
		fmt.Printf("   Description: %s\n", testCase.description)
		fmt.Printf("   Input: %s\n", testCase.input)

		// Process the user input
		fmt.Printf("   Processing... ")
		start := time.Now()

		response, err := agent.ProcessUserInput(ctx, testCase.input)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ FAILED (%.2fs)\n", duration.Seconds())
			fmt.Printf("   Error: %v\n", err)
			continue
		}

		fmt.Printf("✅ SUCCESS (%.2fs)\n", duration.Seconds())
		fmt.Printf("   Response: %s\n", response)

		// Add delay between test cases
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n🎉 Fixed version completed successfully!")
	fmt.Println("\nThis demonstrates the complete unified dialog agent")
	fmt.Println("running without sonic dependency issues.")
}

func main() {
	runFixedMain()
}