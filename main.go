package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// MockChatModel provides a mock implementation of BaseChatModel for demonstration
type MockChatModel struct {
	name string
}

func NewMockChatModel(name string) *MockChatModel {
	return &MockChatModel{name: name}
}

func (m *MockChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// Extract the latest user message
	var userMessage string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
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

	return &schema.Message{
		Role:    schema.Assistant,
		Content: response,
	}, nil
}

func (m *MockChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// For simplicity, we'll just call Generate and wrap it in a stream
	msg, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}

	// Create a simple stream reader that returns the message once
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *MockChatModel) generatePlannerResponse(userMessage string) string {
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

func (m *MockChatModel) generateSupervisorResponse(userMessage string) string {
	// Generate a mock supervisor decision
	response := SupervisorResponse{
		Action: ActionContinue,
		Reason: "Analysis complete. Execution can continue as planned.",
	}

	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	return string(responseJSON)
}

func main() {
	fmt.Println("🤖 Unified Dialog Agent Example")
	fmt.Println("================================")

	ctx := context.Background()

	// Create agent configuration
	config := &AgentConfig{
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
			{
				Name:        "filesystem",
				Description: "File system operations tool",
				Timeout:     5 * time.Second,
				Retries:     2,
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
	plannerModel := NewMockChatModel("planner")
	supervisorModel := NewMockChatModel("supervisor")

	// Create the unified dialog agent
	agent, err := NewUnifiedDialogAgent(ctx, config, plannerModel, supervisorModel)
	if err != nil {
		log.Fatalf("Failed to create unified dialog agent: %v", err)
	}

	// Example user inputs to test the agent
	testCases := []struct {
		name    string
		input   string
		description string
	}{
		{
			name:    "Data Analysis Request",
			input:   "Please analyze our user engagement data from the last month and send me a summary report.",
			description: "Tests multi-step workflow with database queries, data processing, and notification",
		},
		{
			name:    "Customer Support",
			input:   "A customer complained about their order. Can you look up their order details and send them an update?",
			description: "Tests customer data lookup and communication workflow",
		},
		{
			name:    "System Health Check",
			input:   "Check the health of all our services and create a status report.",
			description: "Tests parallel API calls and report generation",
		},
	}

	fmt.Println("\n🧪 Running Test Cases")
	fmt.Println("---------------------")

	for i, testCase := range testCases {
		fmt.Printf("\n%d. %s\n", i+1, testCase.name)
		fmt.Printf("   Description: %s\n", testCase.description)
		fmt.Printf("   Input: %s\n", testCase.input)

		// Create user input message
		userInput := []*schema.Message{
			{
				Role:    schema.User,
				Content: testCase.input,
			},
		}

		// Process the user input
		fmt.Printf("   Processing... ")
		start := time.Now()

		response, err := agent.ProcessUserInput(ctx, userInput)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ FAILED (%.2fs)\n", duration.Seconds())
			fmt.Printf("   Error: %v\n", err)
			continue
		}

		fmt.Printf("✅ SUCCESS (%.2fs)\n", duration.Seconds())

		// Display response
		if len(response) > 0 {
			fmt.Printf("   Response: %s\n", response[0].Content)
		}

		// Add delay between test cases
		time.Sleep(500 * time.Millisecond)
	}

	// Demonstrate MCP client manager functionality
	fmt.Println("\n🔧 MCP Client Manager Demo")
	fmt.Println("--------------------------")

	mcpManager := agent.GetMCPManager()
	clients := mcpManager.GetAllClients()

	fmt.Printf("Registered MCP Clients: %d\n", len(clients))
	for name, client := range clients {
		fmt.Printf("- %s: %s\n", name, client.Name())

		// Test client health
		healthy := client.IsHealthy(ctx)
		status := "❌ Unhealthy"
		if healthy {
			status = "✅ Healthy"
		}
		fmt.Printf("  Status: %s\n", status)

		// Get tool info
		toolInfo, err := client.GetToolInfo(ctx)
		if err == nil {
			fmt.Printf("  Description: %s\n", toolInfo.Desc)
		}
	}

	// Demonstrate parameter resolution
	fmt.Println("\n🔍 Parameter Resolution Demo")
	fmt.Println("----------------------------")

	// Create a test execution state
	testState := &ExecutionState{
		DataStore: map[string]interface{}{
			"step_1": map[string]interface{}{
				"users": []map[string]interface{}{
					{"id": 1, "name": "Alice", "email": "alice@example.com"},
					{"id": 2, "name": "Bob", "email": "bob@example.com"},
				},
				"count": 2,
			},
		},
		UserInput: []*schema.Message{
			{Role: schema.User, Content: "Hello, world!"},
		},
	}

	resolver := NewParameterResolver(testState)

	// Test parameter resolution
	testStep := &ExecutionStep{
		ID:   "test_step",
		Type: StepTypeToolCall,
		Parameters: map[string]interface{}{
			"user_count": map[string]interface{}{
				"type":      "step_result",
				"reference": "step_1",
				"path":      "count",
			},
			"first_user": map[string]interface{}{
				"type":      "step_result",
				"reference": "step_1",
				"path":      "users[0].name",
			},
			"user_input": map[string]interface{}{
				"type":      "user_input",
				"reference": "content",
			},
			"template": "Found ${steps.step_1.count} users. First user: ${steps.step_1.users[0].name}",
		},
	}

	resolvedParams, err := resolver.ResolveParameters(ctx, testStep)
	if err != nil {
		fmt.Printf("❌ Parameter resolution failed: %v\n", err)
	} else {
		fmt.Printf("✅ Parameter resolution successful:\n")
		paramJSON, _ := json.MarshalIndent(resolvedParams, "  ", "  ")
		fmt.Printf("  %s\n", paramJSON)
	}

	fmt.Println("\n🎉 Demo completed successfully!")
	fmt.Println("\nThis demonstrates a unified dialog agent with:")
	fmt.Println("- Planner-Supervisor-Executor hybrid architecture")
	fmt.Println("- Dynamic parameter resolution with $ref syntax")
	fmt.Println("- Multiple MCP tool integrations")
	fmt.Println("- Error handling and supervisor intervention")
	fmt.Println("- Parallel and sequential step execution")
	fmt.Println("- Comprehensive logging and monitoring")
}