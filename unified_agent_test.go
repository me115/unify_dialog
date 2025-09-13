package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Planner Agent functionality
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

// Test Executor Agent functionality
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
			{
				ID:   "step2",
				Type: StepTypeToolCall,
				Tool: "email",
				Name: "Send notification",
				Parameters: map[string]interface{}{
					"operation": "send",
					"to":        "test@example.com",
					"subject":   "Test Report",
					"body":      "Test completed",
				},
				Dependencies: []string{"step1"},
			},
		},
	}

	// Create execution state
	state := &ExecutionState{
		Plan:        plan,
		StepResults: make(map[string]*StepResult),
	}

	// Test plan execution
	result, err := executor.ExecutePlan(ctx, plan, []*schema.Message{
		schema.UserMessage("Execute test plan"),
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify execution results
	assert.Equal(t, StatusCompleted, result.Status)
	assert.True(t, len(result.StepResults) > 0)
}

// Test Supervisor Agent functionality
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
		Status: StatusRunning,
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
	// Create parameter resolver
	resolver := NewParameterResolver()
	require.NotNil(t, resolver)

	// Create test execution state
	state := &ExecutionState{
		StepResults: map[string]interface{}{
			"step1": map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{"name": "Alice", "id": 1},
					map[string]interface{}{"name": "Bob", "id": 2},
				},
				"count": 2,
			},
		},
	}

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

	t.Run("Nested Parameter Resolution", func(t *testing.T) {
		params := map[string]interface{}{
			"data": map[string]interface{}{
				"count": map[string]interface{}{
					"$ref": "steps.step1.count",
				},
				"first_user": map[string]interface{}{
					"type":      "step_result",
					"reference": "step1",
					"path":      "users[0].name",
				},
			},
		}

		resolved, err := resolver.ResolveParameters(ctx, params, state)
		require.NoError(t, err)

		data := resolved["data"].(map[string]interface{})
		assert.Equal(t, 2, data["count"])
		assert.Equal(t, "Alice", data["first_user"])
	})
}

// Test MCP Client Manager functionality
func TestMCPClientManager(t *testing.T) {
	manager := NewMCPClientManager()
	require.NotNil(t, manager)

	// Test client registration
	dbClient := NewDatabaseMCPClient()
	manager.RegisterClient("database", dbClient)

	apiClient := NewAPIMCPClient()
	manager.RegisterClient("api", apiClient)

	emailClient := NewEmailMCPClient()
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

// Test Unified Dialog Agent integration
func TestUnifiedDialogAgent(t *testing.T) {
	ctx := context.Background()

	// Create mock models
	plannerModel := NewMockChatModel("planner")
	supervisorModel := NewMockChatModel("supervisor")

	// Create agent config
	config := &AgentConfig{
		MaxIterations: 5,
		GlobalTimeout: 2 * time.Minute,

		PlannerConfig: &PlannerConfig{
			MaxTokens:   1000,
			Temperature: 0.7,
		},

		ExecutorConfig: &ExecutorConfig{
			MaxParallelSteps: 2,
			StepTimeout:      30 * time.Second,
		},

		SupervisorConfig: &SupervisorConfig{
			TriggerThreshold: 2,
			Temperature:      0.3,
		},
	}

	// Create unified dialog agent
	agent, err := NewUnifiedDialogAgent(ctx, config, plannerModel, supervisorModel)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Test simple dialog
	userMessages := []*schema.Message{
		schema.UserMessage("Please analyze user data and send me a report"),
	}

	response, err := agent.ProcessDialog(ctx, userMessages)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response structure
	assert.NotEmpty(t, response.Content)
	assert.Equal(t, schema.Assistant, response.Role)
}

// Test parallel execution
func TestParallelExecution(t *testing.T) {
	ctx := context.Background()

	// Create MCP manager with multiple clients
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient())
	mcpManager.RegisterClient("api", NewAPIMCPClient(MCPToolConfig{Name: "api"}))
	mcpManager.RegisterClient("email", NewEmailMCPClient())

	// Create executor config for parallel execution
	config := &ExecutorConfig{
		MaxParallelSteps: 3,
		StepTimeout:      30 * time.Second,
	}

	executor, err := NewExecutorAgent(ctx, config, mcpManager)
	require.NoError(t, err)

	// Create plan with parallel steps
	plan := &ExecutionPlan{
		ID:   "parallel_test",
		Goal: "Test parallel execution",
		Steps: []ExecutionStep{
			{
				ID:           "fetch_users",
				Type:         StepTypeToolCall,
				Tool:         "database",
				Name:         "Fetch user data",
				Parameters:   map[string]interface{}{"operation": "query", "query": "SELECT * FROM users"},
				Dependencies: []string{},
			},
			{
				ID:           "fetch_orders",
				Type:         StepTypeToolCall,
				Tool:         "api",
				Name:         "Fetch order data",
				Parameters:   map[string]interface{}{"operation": "request", "endpoint": "/api/orders", "method": "GET"},
				Dependencies: []string{},
			},
			{
				ID:   "merge_data",
				Type: StepTypeDataProcessing,
				Name: "Merge data",
				Parameters: map[string]interface{}{
					"operation": "merge",
					"users":     map[string]interface{}{"$ref": "steps.fetch_users.result"},
					"orders":    map[string]interface{}{"$ref": "steps.fetch_orders.result"},
				},
				Dependencies: []string{"fetch_users", "fetch_orders"},
			},
		},
	}

	// Execute plan
	startTime := time.Now()
	result, err := executor.ExecutePlan(ctx, plan, []*schema.Message{
		schema.UserMessage("Execute parallel plan"),
	})
	duration := time.Since(startTime)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify parallel execution completed successfully
	assert.Equal(t, StatusCompleted, result.Status)
	assert.True(t, len(result.StepResults) >= 3)

	// Verify reasonable execution time (parallel should be faster than sequential)
	assert.True(t, duration < 10*time.Second, "Parallel execution took too long: %v", duration)
}

// Test error handling and supervisor intervention
func TestErrorHandlingAndSupervision(t *testing.T) {
	ctx := context.Background()

	// Create mock models
	supervisorModel := NewMockChatModel("supervisor")

	// Create supervisor config with low threshold for testing
	supervisorConfig := &SupervisorConfig{
		TriggerThreshold: 1,
		Temperature:      0.3,
	}

	supervisor, err := NewSupervisorAgent(ctx, supervisorConfig, supervisorModel)
	require.NoError(t, err)

	// Create execution state with error-prone step
	step := &ExecutionStep{
		ID:   "failing_step",
		Type: StepTypeToolCall,
		Tool: "nonexistent_tool",
		Name: "Failing step",
		RetryPolicy: &RetryPolicy{
			MaxAttempts:  2,
			BackoffTime:  1 * time.Second,
			Exponential:  true,
		},
		ErrorPolicy: ErrorPolicyRetry,
	}

	state := &ExecutionState{
		Plan: &ExecutionPlan{
			Steps: []ExecutionStep{*step},
		},
		Status: StatusRunning,
	}

	// Test supervisor intervention on error
	testErr := assert.AnError
	shouldIntervene := supervisor.ShouldIntervene(ctx, "failing_step", testErr, state)
	assert.True(t, shouldIntervene)

	// Test supervisor decision making
	decision, err := supervisor.MakeDecision(ctx, "failing_step", testErr, state, map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, decision)

	// Verify decision structure
	assert.NotEmpty(t, decision.Action)
	assert.NotEmpty(t, decision.Reason)
	assert.Contains(t, []string{
		string(ActionContinue),
		string(ActionRetry),
		string(ActionSkip),
		string(ActionAbort),
	}, string(decision.Action))
}

// Test complex execution plan with dependencies
func TestComplexExecutionPlan(t *testing.T) {
	ctx := context.Background()

	// Create MCP manager
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient())
	mcpManager.RegisterClient("api", NewAPIMCPClient(MCPToolConfig{Name: "api"}))
	mcpManager.RegisterClient("email", NewEmailMCPClient())

	// Create executor
	config := &ExecutorConfig{
		MaxParallelSteps: 2,
		StepTimeout:      30 * time.Second,
	}

	executor, err := NewExecutorAgent(ctx, config, mcpManager)
	require.NoError(t, err)

	// Create complex plan with multiple dependencies and parameter references
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
			{
				ID:   "step4",
				Type: StepTypeToolCall,
				Tool: "email",
				Name: "Send notification",
				Parameters: map[string]interface{}{
					"operation": "send",
					"to":        "admin@example.com",
					"subject":   "Data Processing Complete",
					"body":      "Report: ${steps.step3.result}",
				},
				Dependencies: []string{"step3"},
			},
		},
	}

	// Execute complex plan
	result, err := executor.ExecutePlan(ctx, plan, []*schema.Message{
		schema.UserMessage("Execute complex analysis"),
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify all steps completed
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, 4, len(result.StepResults))

	// Verify step order and dependencies were respected
	assert.Contains(t, result.StepResults, "step1")
	assert.Contains(t, result.StepResults, "step2")
	assert.Contains(t, result.StepResults, "step3")
	assert.Contains(t, result.StepResults, "step4")
}

// Test tool conversion to EINO interface
func TestToolConversion(t *testing.T) {
	ctx := context.Background()

	// Create MCP client
	dbClient := NewDatabaseMCPClient()

	// Convert to EINO tool
	einoTool := ConvertToEinoTool(dbClient)
	require.NotNil(t, einoTool)

	// Test tool info
	info, err := einoTool.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "database", info.Name)

	// Test tool execution
	params := map[string]interface{}{
		"operation": "query",
		"query":     "SELECT * FROM users",
	}

	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	result, err := einoTool.InvokableRun(ctx, string(paramsJSON))
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Verify result is valid JSON
	var resultData map[string]interface{}
	err = json.Unmarshal([]byte(result), &resultData)
	require.NoError(t, err)
	assert.Contains(t, resultData, "data")
}

// Test execution state management
func TestExecutionStateManagement(t *testing.T) {
	// Create execution state
	state := &ExecutionState{
		Plan: &ExecutionPlan{
			ID:   "test_plan",
			Goal: "Test state management",
		},
		StepResults:    make(map[string]interface{}),
		CurrentStepID:  "",
		Status:         StatusPending,
		StartTime:      time.Now(),
	}

	// Test status transitions
	state.Status = StatusRunning
	assert.Equal(t, StatusRunning, state.Status)

	// Test step result storage
	state.StepResults["step1"] = map[string]interface{}{
		"result": "success",
		"data":   []string{"item1", "item2"},
	}

	assert.Contains(t, state.StepResults, "step1")

	stepResult := state.StepResults["step1"].(map[string]interface{})
	assert.Equal(t, "success", stepResult["result"])

	// Test current step tracking
	state.CurrentStepID = "step2"
	assert.Equal(t, "step2", state.CurrentStepID)

	// Test completion
	state.Status = StatusCompleted
	state.EndTime = time.Now()
	assert.Equal(t, StatusCompleted, state.Status)
	assert.True(t, state.EndTime.After(state.StartTime))
}

// Benchmark tests for performance validation
func BenchmarkParameterResolution(b *testing.B) {
	resolver := NewParameterResolver()

	state := &ExecutionState{
		StepResults: map[string]interface{}{
			"step1": map[string]interface{}{
				"data": map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{"name": "Alice", "id": 1},
						map[string]interface{}{"name": "Bob", "id": 2},
					},
					"count": 2,
				},
			},
		},
	}

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

func BenchmarkMCPToolExecution(b *testing.B) {
	client := NewDatabaseMCPClient()
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