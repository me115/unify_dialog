package main

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test MockChatModel functionality
func TestMockChatModel(t *testing.T) {
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

// Test PlannerAgent functionality
func TestPlannerAgent(t *testing.T) {
	ctx := context.Background()

	// Create mock chat model
	mockModel := NewMockChatModel("planner")

	// Create mock MCP manager
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))
	mcpManager.RegisterClient("email", NewEmailMCPClient(MCPToolConfig{Name: "email"}))

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

// Test ExecutorAgent functionality
func TestExecutorAgent(t *testing.T) {
	ctx := context.Background()

	// Create mock MCP manager
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))
	mcpManager.RegisterClient("email", NewEmailMCPClient(MCPToolConfig{Name: "email"}))

	// Create executor config
	config := &ExecutorConfig{
		MaxParallelSteps: 2,
		StepTimeout:      30 * time.Second,
	}

	// Create executor agent
	executor := NewExecutorAgent(config, mcpManager)
	require.NotNil(t, executor)

	// Create test execution plan
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
		Plan:        plan,
		StepResults: make(map[string]*StepResult),
		UserInput: []*schema.Message{
			schema.UserMessage("Execute test plan"),
		},
	}

	// Test plan execution
	err := executor.ExecutePlan(ctx, plan, state)
	require.NoError(t, err)

	// Verify execution results
	assert.True(t, len(state.StepResults) > 0)
}

// Test SupervisorAgent functionality
func TestSupervisorAgent(t *testing.T) {
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

	// Test intervention decision
	step := &ExecutionStep{
		ID:   "test_step",
		Type: StepTypeToolCall,
		Name: "Test step",
	}

	state := &ExecutionState{
		Plan: &ExecutionPlan{
			Steps: []ExecutionStep{*step},
		},
	}

	// Test without error - should not intervene
	shouldIntervene := supervisor.ShouldIntervene(ctx, "test_step", nil, state)
	assert.False(t, shouldIntervene)

	// Test with error but below threshold
	testErr := assert.AnError
	shouldIntervene = supervisor.ShouldIntervene(ctx, "test_step", testErr, state)
	assert.False(t, shouldIntervene)

	// Test with error at threshold
	shouldIntervene = supervisor.ShouldIntervene(ctx, "test_step", testErr, state)
	assert.True(t, shouldIntervene)

	// Test making decision
	decision, err := supervisor.MakeDecision(ctx, "test_step", testErr, state, map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, decision)

	assert.NotEmpty(t, decision.Action)
	assert.NotEmpty(t, decision.Reason)
}

// Test Parameter Resolver functionality
func TestParameterResolver(t *testing.T) {
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

	t.Run("Simple Reference Resolution", func(t *testing.T) {
		params := map[string]interface{}{
			"data": map[string]interface{}{
				"$ref": "steps.step1.result",
			},
		}

		resolved, err := resolver.ResolveParameters(ctx, params, state)
		require.NoError(t, err)

		data := resolved["data"].(map[string]interface{})
		assert.Equal(t, 2, data["count"])
		assert.NotNil(t, data["users"])
	})

	t.Run("Structured Reference Resolution", func(t *testing.T) {
		params := map[string]interface{}{
			"user_count": map[string]interface{}{
				"type":      "step_result",
				"reference": "step1",
				"path":      "count",
			},
		}

		resolved, err := resolver.ResolveParameters(ctx, params, state)
		require.NoError(t, err)

		assert.Equal(t, 2, resolved["user_count"])
	})

	t.Run("Template Variable Resolution", func(t *testing.T) {
		params := map[string]interface{}{
			"message": "Found ${steps.step1.count} users",
		}

		resolved, err := resolver.ResolveParameters(ctx, params, state)
		require.NoError(t, err)

		assert.Equal(t, "Found 2 users", resolved["message"])
	})
}

// Test MCP Client Manager functionality
func TestMCPClientManager(t *testing.T) {
	manager := NewMCPClientManager()
	require.NotNil(t, manager)

	// Test client registration
	dbClient := NewDatabaseMCPClient(MCPToolConfig{Name: "database"})
	manager.RegisterClient("database", dbClient)

	apiClient := NewAPIMCPClient(MCPToolConfig{Name: "api"})
	manager.RegisterClient("api", apiClient)

	emailClient := NewEmailMCPClient(MCPToolConfig{Name: "email"})
	manager.RegisterClient("email", emailClient)

	// Test client retrieval
	retrievedClient, exists := manager.GetClient("database")
	assert.True(t, exists)
	assert.NotNil(t, retrievedClient)

	// Test non-existent client
	_, exists = manager.GetClient("nonexistent")
	assert.False(t, exists)

	// Test listing clients
	clients := manager.ListClients()
	assert.Equal(t, 3, len(clients))

	// Test client health check
	ctx := context.Background()
	healthy := manager.IsHealthy(ctx, "database")
	assert.True(t, healthy)

	// Test getting all tools
	tools, err := manager.GetAllTools(ctx)
	require.NoError(t, err)
	assert.True(t, len(tools) >= 3)
}

// Test MCP Client functionality
func TestMCPClients(t *testing.T) {
	ctx := context.Background()

	t.Run("Database MCP Client", func(t *testing.T) {
		client := NewDatabaseMCPClient(MCPToolConfig{Name: "database"})
		require.NotNil(t, client)

		// Test tool info
		info, err := client.GetToolInfo(ctx)
		require.NoError(t, err)
		assert.Equal(t, "database", info.Name)
		assert.NotEmpty(t, info.Description)

		// Test query operation
		params := map[string]interface{}{
			"operation": "query",
			"query":     "SELECT * FROM users",
		}

		result, err := client.Call(ctx, "query", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		resultMap := result.(map[string]interface{})
		assert.Contains(t, resultMap, "data")
		assert.Contains(t, resultMap, "row_count")

		// Test health check
		healthResult, err := client.Call(ctx, "health", map[string]interface{}{})
		require.NoError(t, err)

		healthMap := healthResult.(map[string]interface{})
		assert.Equal(t, "healthy", healthMap["status"])
	})

	t.Run("Email MCP Client", func(t *testing.T) {
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
	})

	t.Run("API MCP Client", func(t *testing.T) {
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
		assert.Contains(t, resultMap, "message")
		assert.Contains(t, resultMap, "endpoint")
	})
}

// Test complex execution workflow
func TestComplexExecutionWorkflow(t *testing.T) {
	ctx := context.Background()

	// Create MCP manager
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))
	mcpManager.RegisterClient("api", NewAPIMCPClient(MCPToolConfig{Name: "api"}))
	mcpManager.RegisterClient("email", NewEmailMCPClient(MCPToolConfig{Name: "email"}))

	// Create executor
	config := &ExecutorConfig{
		MaxParallelSteps: 2,
		StepTimeout:      30 * time.Second,
	}

	executor := NewExecutorAgent(config, mcpManager)

	// Create complex plan with dependencies and parameter references
	plan := &ExecutionPlan{
		ID:   "complex_plan",
		Goal: "Complex multi-step workflow with dependencies",
		Steps: []ExecutionStep{
			{
				ID:   "step1",
				Type: StepTypeToolCall,
				Tool: "database",
				Name: "Query user data",
				Parameters: map[string]interface{}{
					"operation": "query",
					"query":     "SELECT * FROM users WHERE active = true",
				},
				Dependencies: []string{},
			},
			{
				ID:   "step2",
				Type: StepTypeToolCall,
				Tool: "api",
				Name: "Fetch additional data",
				Parameters: map[string]interface{}{
					"operation": "request",
					"endpoint":  "/api/user-metrics",
					"method":    "GET",
				},
				Dependencies: []string{},
			},
			{
				ID:   "step3",
				Type: StepTypeDataProcessing,
				Name: "Process and combine data",
				Parameters: map[string]interface{}{
					"operation":  "combine",
					"user_data":  map[string]interface{}{"$ref": "steps.step1.result"},
					"metrics":    map[string]interface{}{"$ref": "steps.step2.result"},
					"template":   "Processing ${steps.step1.result.row_count} users",
				},
				Dependencies: []string{"step1", "step2"},
			},
		},
	}

	// Create execution state
	state := &ExecutionState{
		Plan:        plan,
		StepResults: make(map[string]*StepResult),
		UserInput: []*schema.Message{
			schema.UserMessage("Execute complex analysis"),
		},
	}

	// Execute complex plan
	err := executor.ExecutePlan(ctx, plan, state)
	require.NoError(t, err)

	// Verify steps were executed
	assert.True(t, len(state.StepResults) >= 2) // At least step1 and step2
}

// Test error handling in execution
func TestExecutionErrorHandling(t *testing.T) {
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

	// Verify error was logged in execution log
	assert.True(t, len(state.ExecutionLog) > 0)
}

// Test step status tracking
func TestStepStatusTracking(t *testing.T) {
	// Create step result
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

	params := map[string]interface{}{
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
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveParameters(ctx, params, state)
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