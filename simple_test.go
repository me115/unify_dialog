package main

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test MockChatModel basic functionality
func TestMockChatModel_Basic(t *testing.T) {
	ctx := context.Background()

	// Test planner model
	plannerModel := NewMockChatModel("planner")
	require.NotNil(t, plannerModel)

	// Test generation
	messages := []*schema.Message{
		schema.UserMessage("Test message"),
	}

	result, err := plannerModel.Generate(ctx, messages)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, schema.Assistant, result.Role)
	assert.NotEmpty(t, result.Content)

	// Test streaming
	stream, err := plannerModel.Stream(ctx, messages)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read from stream
	streamResult, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, streamResult)
	assert.Equal(t, schema.Assistant, streamResult.Role)

	stream.Close()
}

// Test MCP Client Manager basic functionality
func TestMCPClientManager_Basic(t *testing.T) {
	manager := NewMCPClientManager()
	require.NotNil(t, manager)

	// Test client registration
	dbClient := NewDatabaseMCPClient(MCPToolConfig{Name: "database"})
	manager.RegisterClient("database", dbClient)

	emailClient := NewEmailMCPClient(MCPToolConfig{Name: "email"})
	manager.RegisterClient("email", emailClient)

	// Test client retrieval
	retrievedClient, err := manager.GetClient("database")
	require.NoError(t, err)
	require.NotNil(t, retrievedClient)

	// Test non-existent client
	_, err = manager.GetClient("nonexistent")
	assert.Error(t, err)
}

// Test Database MCP Client functionality
func TestDatabaseMCPClient(t *testing.T) {
	ctx := context.Background()
	client := NewDatabaseMCPClient(MCPToolConfig{Name: "database"})
	require.NotNil(t, client)

	// Test tool info
	info, err := client.GetToolInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "database", info.Name)

	// Test query operation
	params := map[string]interface{}{
		"operation": "query",
		"query":     "SELECT * FROM users",
	}

	result, err := client.Call(ctx, "query", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	resultMap := result.(map[string]interface{})
	assert.Contains(t, resultMap, "rows")
	assert.Contains(t, resultMap, "rowCount")

	// Test health check
	healthResult, err := client.Call(ctx, "health", map[string]interface{}{})
	require.NoError(t, err)

	healthMap := healthResult.(map[string]interface{})
	assert.Equal(t, "healthy", healthMap["status"])
}

// Test Email MCP Client functionality
func TestEmailMCPClient(t *testing.T) {
	ctx := context.Background()
	client := NewEmailMCPClient(MCPToolConfig{Name: "email"})
	require.NotNil(t, client)

	// Test tool info
	info, err := client.GetToolInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "email", info.Name)

	// Test send operation
	params := map[string]interface{}{
		"operation": "send",
		"to":        "test@example.com",
		"subject":   "Test Email",
		"body":      "This is a test email",
	}

	result, err := client.Call(ctx, "send", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	resultMap := result.(map[string]interface{})
	assert.Contains(t, resultMap, "messageId")
	assert.Contains(t, resultMap, "status")
	assert.Equal(t, "sent", resultMap["status"])
	assert.Equal(t, "This is a test email", resultMap["body"])
}

// Test API MCP Client functionality
func TestAPIMCPClient(t *testing.T) {
	ctx := context.Background()
	client := NewAPIMCPClient(MCPToolConfig{Name: "api"})
	require.NotNil(t, client)

	// Test tool info
	info, err := client.GetToolInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "api", info.Name)

	// Test request operation
	params := map[string]interface{}{
		"operation": "request",
		"endpoint":  "/api/users",
		"method":    "GET",
	}

	result, err := client.Call(ctx, "request", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	resultMap := result.(map[string]interface{})
	assert.Contains(t, resultMap, "data")
	assert.Contains(t, resultMap, "endpoint")
}

// Test Parameter Resolver with actual method signature
func TestParameterResolver_Basic(t *testing.T) {
	// Create execution state with sample data
	state := &ExecutionState{
		StepResults: map[string]*StepResult{
			"step1": {
				StepID: "step1",
				Result: map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{"name": "Alice", "id": 1},
						map[string]interface{}{"name": "Bob", "id": 2},
					},
					"count": 2,
				},
				Status: StepStatusSuccess,
			},
		},
	}

	// Create parameter resolver
	resolver := NewParameterResolver(state)
	require.NotNil(t, resolver)

	ctx := context.Background()

	// Create a test step with simpler parameters that don't reference other steps
	step := &ExecutionStep{
		ID:   "test_step",
		Type: StepTypeToolCall,
		Parameters: map[string]interface{}{
			"simple_param": "test_value",
			"number_param": 42,
		},
	}

	// Test parameter resolution
	resolved, err := resolver.ResolveParameters(ctx, step)
	require.NoError(t, err)
	require.NotNil(t, resolved)

	// Verify resolved parameters
	assert.Contains(t, resolved, "simple_param")
	assert.Contains(t, resolved, "number_param")

	assert.Equal(t, "test_value", resolved["simple_param"])
	assert.Equal(t, 42, resolved["number_param"])
}

// Test ExecutorAgent basic functionality
func TestExecutorAgent_Basic(t *testing.T) {
	ctx := context.Background()

	// Create mock MCP manager
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))

	// Create executor config
	config := &ExecutorConfig{
		MaxParallelSteps: 2,
		StepTimeout:      30 * time.Second,
	}

	// Create executor agent
	executor := NewExecutorAgent(config, mcpManager)
	require.NotNil(t, executor)

	// Create simple execution plan
	plan := &ExecutionPlan{
		ID:   "test_plan",
		Goal: "Test execution",
		Steps: []ExecutionStep{
			{
				ID:   "step1",
				Type: StepTypeToolCall,
				Tool: "database",
				Name: "Query data",
				Parameters: map[string]interface{}{
					"operation": "query",
					"query":     "SELECT * FROM users",
				},
				Dependencies: []string{},
			},
		},
	}

	// Create execution state
	state := &ExecutionState{
		Plan:          plan,
		StepResults:   make(map[string]*StepResult),
		MaxIterations: 10, // Set maximum iterations
		UserInput: []*schema.Message{
			schema.UserMessage("Execute test plan"),
		},
	}

	// Test plan execution
	err := executor.ExecutePlan(ctx, plan, state)
	require.NoError(t, err)

	// Verify execution results
	assert.True(t, len(state.StepResults) > 0)
	assert.Contains(t, state.StepResults, "step1")

	stepResult := state.StepResults["step1"]
	assert.Equal(t, "step1", stepResult.StepID)
	assert.Equal(t, StepStatusSuccess, stepResult.Status)
	assert.NotNil(t, stepResult.Result)
}

// Test PlannerAgent basic functionality
func TestPlannerAgent_Basic(t *testing.T) {
	ctx := context.Background()

	// Create mock chat model
	mockModel := NewMockChatModel("planner")

	// Create mock MCP manager
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))

	// Create planner config
	config := &PlannerConfig{
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	// Create planner agent
	planner, err := NewPlannerAgent(ctx, config, mockModel, mcpManager)
	require.NoError(t, err)
	require.NotNil(t, planner)

	// Test plan generation
	userInput := []*schema.Message{
		schema.UserMessage("Analyze user data and send report"),
	}

	plan, err := planner.GeneratePlan(ctx, userInput)
	require.NoError(t, err)
	require.NotNil(t, plan)

	// Verify plan structure
	assert.NotEmpty(t, plan.Goal)
	assert.NotEmpty(t, plan.Steps)
	assert.True(t, len(plan.Steps) > 0)

	// Verify step structure
	step := plan.Steps[0]
	assert.NotEmpty(t, step.ID)
	assert.NotEmpty(t, step.Type)
	assert.NotEmpty(t, step.Name)
	assert.NotNil(t, step.Parameters)
}

// Test SupervisorAgent basic functionality
func TestSupervisorAgent_Basic(t *testing.T) {
	ctx := context.Background()

	// Create mock chat model
	mockModel := NewMockChatModel("supervisor")

	// Create supervisor config
	config := &SupervisorConfig{
		TriggerThreshold: 2,
		Temperature:      0.3,
	}

	// Create supervisor agent
	supervisor, err := NewSupervisorAgent(ctx, config, mockModel)
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	// Create test execution state
	state := &ExecutionState{
		Plan: &ExecutionPlan{
			Steps: []ExecutionStep{
				{
					ID:   "test_step",
					Type: StepTypeToolCall,
					Name: "Test step",
				},
			},
		},
	}

	// Test intervention decision without error
	shouldIntervene := supervisor.ShouldIntervene(ctx, "test_step", nil, state)
	assert.False(t, shouldIntervene)

	// Test intervention decision with error (first time - below threshold)
	testErr := assert.AnError
	shouldIntervene = supervisor.ShouldIntervene(ctx, "test_step", testErr, state)
	assert.False(t, shouldIntervene)

	// Test intervention decision with error (second time - at threshold)
	shouldIntervene = supervisor.ShouldIntervene(ctx, "test_step", testErr, state)
	assert.True(t, shouldIntervene)

	// Test making decision with proper SupervisorRequest
	request := &SupervisorRequest{
		PlanID:      "test_plan",
		CurrentStep: &state.Plan.Steps[0],
		ExecutionState: state,
		Context:     map[string]interface{}{},
	}

	decision, err := supervisor.MakeDecision(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, decision)

	assert.NotEmpty(t, decision.Action)
	assert.NotEmpty(t, decision.Reason)
}

// Test step status handling
func TestStepStatus(t *testing.T) {
	stepResult := &StepResult{
		StepID:    "test_step",
		Status:    StepStatusPending,
		StartTime: time.Now(),
	}

	// Test status transitions
	assert.Equal(t, StepStatusPending, stepResult.Status)

	stepResult.Status = StepStatusRunning
	assert.Equal(t, StepStatusRunning, stepResult.Status)

	stepResult.Status = StepStatusSuccess
	stepResult.EndTime = time.Now()
	assert.Equal(t, StepStatusSuccess, stepResult.Status)
	assert.True(t, stepResult.EndTime.After(stepResult.StartTime))

	// Test with result data
	stepResult.Result = map[string]interface{}{
		"data": "test result",
		"count": 42,
	}

	result := stepResult.Result.(map[string]interface{})
	assert.Equal(t, "test result", result["data"])
	assert.Equal(t, 42, result["count"])
}

// Test error handling for non-existent tools
func TestExecutorAgent_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Create MCP manager with limited clients
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))

	// Create executor
	executor := NewExecutorAgent(&ExecutorConfig{
		MaxParallelSteps: 1,
		StepTimeout:      5 * time.Second,
	}, mcpManager)

	// Create plan with non-existent tool
	plan := &ExecutionPlan{
		ID:   "error_plan",
		Goal: "Test error handling",
		Steps: []ExecutionStep{
			{
				ID:   "failing_step",
				Type: StepTypeToolCall,
				Tool: "nonexistent_tool", // This should cause an error
				Name: "Failing step",
				Parameters: map[string]interface{}{
					"operation": "test",
				},
				Dependencies: []string{},
				RetryPolicy: &RetryPolicy{
					MaxAttempts:  2,
					BackoffTime:  1 * time.Second,
					Exponential:  false,
				},
				ErrorPolicy: ErrorPolicyRetry,
			},
		},
	}

	state := &ExecutionState{
		Plan:        plan,
		StepResults: make(map[string]*StepResult),
		UserInput: []*schema.Message{
			schema.UserMessage("Test error handling"),
		},
	}

	// Execute plan - should handle error gracefully
	err := executor.ExecutePlan(ctx, plan, state)
	// We expect an error since the tool doesn't exist
	assert.Error(t, err)

	// Verify error was handled
	assert.True(t, len(state.ExecutionLog) > 0)
}

// Benchmark parameter resolution performance
func BenchmarkParameterResolution(b *testing.B) {
	state := &ExecutionState{
		StepResults: map[string]*StepResult{
			"step1": {
				StepID: "step1",
				Result: map[string]interface{}{
					"data": map[string]interface{}{
						"users": []interface{}{
							map[string]interface{}{"name": "Alice", "id": 1},
							map[string]interface{}{"name": "Bob", "id": 2},
						},
						"count": 2,
					},
				},
				Status: StepStatusSuccess,
			},
		},
	}

	resolver := NewParameterResolver(state)
	ctx := context.Background()

	step := &ExecutionStep{
		ID:   "test_step",
		Type: StepTypeToolCall,
		Parameters: map[string]interface{}{
			"user_count": map[string]interface{}{
				"$ref": "steps.step1.data.count",
			},
			"message": "Found ${steps.step1.data.count} users",
			"nested": map[string]interface{}{
				"first_user": map[string]interface{}{
					"type":      "step_result",
					"reference": "step1",
					"path":      "data.users[0].name",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveParameters(ctx, step)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark MCP tool execution performance
func BenchmarkMCPToolExecution(b *testing.B) {
	client := NewDatabaseMCPClient(MCPToolConfig{Name: "database"})
	ctx := context.Background()

	params := map[string]interface{}{
		"operation": "query",
		"query":     "SELECT * FROM users",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Call(ctx, "query", params)
		if err != nil {
			b.Fatal(err)
		}
	}
}