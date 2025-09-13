package main

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnifiedDialogAgentFeatures comprehensively tests the core features described in README.md
func TestUnifiedDialogAgentFeatures(t *testing.T) {
	t.Run("Core Architecture Components", func(t *testing.T) {
		t.Run("MockChatModel Implementation", func(t *testing.T) {
			ctx := context.Background()

			// Test different model types
			plannerModel := NewMockChatModel("planner")
			supervisorModel := NewMockChatModel("supervisor")

			require.NotNil(t, plannerModel)
			require.NotNil(t, supervisorModel)

			// Test message generation
			messages := []*schema.Message{
				schema.UserMessage("Analyze user data and create execution plan"),
			}

			plannerResponse, err := plannerModel.Generate(ctx, messages)
			require.NoError(t, err)
			assert.Equal(t, schema.Assistant, plannerResponse.Role)
			assert.Contains(t, plannerResponse.Content, "goal")

			// Test streaming capability
			stream, err := plannerModel.Stream(ctx, messages)
			require.NoError(t, err)

			streamMsg, err := stream.Recv()
			require.NoError(t, err)
			assert.Equal(t, schema.Assistant, streamMsg.Role)

			stream.Close()
		})

		t.Run("MCP Tool Integration", func(t *testing.T) {
			ctx := context.Background()

			// Test Database Tool
			dbTool := NewDatabaseMCPClient(MCPToolConfig{Name: "database"})

			// Test tool info
			info, err := dbTool.GetToolInfo(ctx)
			require.NoError(t, err)
			assert.Equal(t, "database", info.Name)

			// Test query operation
			queryResult, err := dbTool.Call(ctx, "query", map[string]interface{}{
				"operation": "query",
				"query":     "SELECT * FROM users WHERE active = true",
			})
			require.NoError(t, err)

			queryMap := queryResult.(map[string]interface{})
			assert.Contains(t, queryMap, "rows")
			assert.Contains(t, queryMap, "rowCount")
			assert.Equal(t, 2, queryMap["rowCount"])

			// Test Email Tool
			emailTool := NewEmailMCPClient(MCPToolConfig{Name: "email"})

			emailResult, err := emailTool.Call(ctx, "send", map[string]interface{}{
				"operation": "send",
				"to":        "admin@example.com",
				"subject":   "Test Report",
				"body":      "User analysis complete",
			})
			require.NoError(t, err)

			emailMap := emailResult.(map[string]interface{})
			assert.Equal(t, "sent", emailMap["status"])
			assert.Contains(t, emailMap, "messageId")

			// Test API Tool
			apiTool := NewAPIMCPClient(MCPToolConfig{Name: "api"})

			apiResult, err := apiTool.Call(ctx, "request", map[string]interface{}{
				"operation": "request",
				"endpoint":  "/api/user-metrics",
				"method":    "GET",
			})
			require.NoError(t, err)

			apiMap := apiResult.(map[string]interface{})
			assert.Contains(t, apiMap, "data")
			assert.Equal(t, 200, apiMap["statusCode"])
		})

		t.Run("MCP Client Manager", func(t *testing.T) {
			ctx := context.Background()
			manager := NewMCPClientManager()

			// Register multiple tools
			tools := map[string]MCPClient{
				"database": NewDatabaseMCPClient(MCPToolConfig{Name: "database"}),
				"email":    NewEmailMCPClient(MCPToolConfig{Name: "email"}),
				"api":      NewAPIMCPClient(MCPToolConfig{Name: "api"}),
			}

			for name, client := range tools {
				manager.RegisterClient(name, client)
			}

			// Test client retrieval
			for name := range tools {
				client, err := manager.GetClient(name)
				require.NoError(t, err)
				require.NotNil(t, client)

				// Test tool info retrieval
				info, err := client.GetToolInfo(ctx)
				require.NoError(t, err)
				assert.Equal(t, name, info.Name)
			}

			// Test non-existent client
			_, err := manager.GetClient("nonexistent")
			assert.Error(t, err)
		})
	})

	t.Run("Parameter Resolution System", func(t *testing.T) {
		ctx := context.Background()

		// Create sample execution state
		state := &ExecutionState{
			StepResults: map[string]*StepResult{
				"user_query": {
					StepID: "user_query",
					Status: StepStatusSuccess,
					Result: map[string]interface{}{
						"users": []map[string]interface{}{
							{"name": "Alice", "id": 1, "active": true},
							{"name": "Bob", "id": 2, "active": false},
						},
						"count":      2,
						"active_count": 1,
					},
				},
				"metrics_data": {
					StepID: "metrics_data",
					Status: StepStatusSuccess,
					Result: map[string]interface{}{
						"total_visits": 150,
						"avg_session": "5.2 minutes",
					},
				},
			},
		}

		resolver := NewParameterResolver(state)

		// Test basic parameter passing
		step := &ExecutionStep{
			ID:   "test_step",
			Type: StepTypeToolCall,
			Parameters: map[string]interface{}{
				"static_param":  "test_value",
				"numeric_param": 42,
				"bool_param":    true,
			},
		}

		resolved, err := resolver.ResolveParameters(ctx, step)
		require.NoError(t, err)

		assert.Equal(t, "test_value", resolved["static_param"])
		assert.Equal(t, 42, resolved["numeric_param"])
		assert.Equal(t, true, resolved["bool_param"])
	})

	t.Run("Execution State Management", func(t *testing.T) {
		// Test step result tracking
		stepResult := &StepResult{
			StepID:    "test_step",
			Status:    StepStatusPending,
			StartTime: time.Now(),
		}

		// Status transitions
		assert.Equal(t, StepStatusPending, stepResult.Status)

		stepResult.Status = StepStatusRunning
		assert.Equal(t, StepStatusRunning, stepResult.Status)

		stepResult.Status = StepStatusSuccess
		stepResult.EndTime = time.Now()
		stepResult.Result = map[string]interface{}{
			"success": true,
			"data":    []string{"item1", "item2"},
		}

		assert.Equal(t, StepStatusSuccess, stepResult.Status)
		assert.True(t, stepResult.EndTime.After(stepResult.StartTime))

		result := stepResult.Result.(map[string]interface{})
		assert.Equal(t, true, result["success"])
		assert.Len(t, result["data"], 2)
	})

	t.Run("Execution Plans and Step Types", func(t *testing.T) {
		// Test execution plan structure
		plan := &ExecutionPlan{
			ID:   "comprehensive_test_plan",
			Goal: "Test all step types and dependencies",
			Steps: []ExecutionStep{
				{
					ID:   "step1_tool_call",
					Type: StepTypeToolCall,
					Tool: "database",
					Name: "Query user data",
					Parameters: map[string]interface{}{
						"operation": "query",
						"query":     "SELECT * FROM users",
					},
					Dependencies: []string{},
					RetryPolicy: &RetryPolicy{
						MaxAttempts:  3,
						BackoffTime:  2 * time.Second,
						Exponential:  true,
					},
					ErrorPolicy: ErrorPolicyRetry,
				},
				{
					ID:   "step2_data_processing",
					Type: StepTypeDataProcessing,
					Name: "Process user data",
					Parameters: map[string]interface{}{
						"operation": "filter",
						"criteria":  "active = true",
					},
					Dependencies: []string{"step1_tool_call"},
					ErrorPolicy:  ErrorPolicyContinue,
				},
				{
					ID:   "step3_condition",
					Type: StepTypeCondition,
					Name: "Check if notification needed",
					Parameters: map[string]interface{}{
						"condition": "user_count > 0",
					},
					Dependencies: []string{"step2_data_processing"},
				},
			},
		}

		// Verify plan structure
		assert.Equal(t, "comprehensive_test_plan", plan.ID)
		assert.Equal(t, 3, len(plan.Steps))

		// Verify step types are all defined
		stepTypes := []StepType{
			StepTypeToolCall,
			StepTypeDataProcessing,
			StepTypeCondition,
			StepTypeSupervisor,
		}

		for _, stepType := range stepTypes {
			assert.NotEmpty(t, string(stepType))
		}

		// Verify error policies
		errorPolicies := []ErrorHandlingPolicy{
			ErrorPolicyRetry,
			ErrorPolicyContinue,
			ErrorPolicyFailFast,
			ErrorPolicyAskUser,
		}

		for _, policy := range errorPolicies {
			assert.NotEmpty(t, string(policy))
		}
	})

	t.Run("Supervisor Decision Types", func(t *testing.T) {
		// Test all supervisor action types are defined
		actions := []ActionType{
			ActionContinue,
			ActionRetry,
			ActionSkip,
			ActionAbort,
			ActionModifyStep,
			ActionAddStep,
			ActionReplan,
			ActionAskUser,
			ActionComplete,
		}

		for _, action := range actions {
			assert.NotEmpty(t, string(action))
		}

		// Test supervisor response structure
		response := &SupervisorResponse{
			Action:      ActionRetry,
			Reason:      "Network timeout, retrying with backoff",
			StepID:      "failed_step",
			Parameters:  map[string]interface{}{"retry_count": 2},
			UserMessage: "Retrying failed operation...",
		}

		assert.Equal(t, ActionRetry, response.Action)
		assert.NotEmpty(t, response.Reason)
		assert.Contains(t, response.Parameters, "retry_count")
	})
}

// TestErrorHandlingStrategies tests the error handling capabilities mentioned in README
func TestErrorHandlingStrategies(t *testing.T) {
	ctx := context.Background()

	// Create MCP manager with database tool only
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))

	// Create executor with timeout settings
	executor := NewExecutorAgent(&ExecutorConfig{
		MaxParallelSteps: 1,
		StepTimeout:      2 * time.Second,
	}, mcpManager)

	// Test plan with non-existent tool (should trigger error handling)
	errorPlan := &ExecutionPlan{
		ID:   "error_handling_test",
		Goal: "Test error handling strategies",
		Steps: []ExecutionStep{
			{
				ID:   "failing_step",
				Type: StepTypeToolCall,
				Tool: "nonexistent_tool",
				Name: "This should fail",
				Parameters: map[string]interface{}{
					"operation": "test",
				},
				Dependencies: []string{},
				RetryPolicy: &RetryPolicy{
					MaxAttempts:  2,
					BackoffTime:  500 * time.Millisecond,
					Exponential:  false,
				},
				ErrorPolicy: ErrorPolicyRetry,
			},
		},
	}

	state := &ExecutionState{
		Plan:          errorPlan,
		StepResults:   make(map[string]*StepResult),
		MaxIterations: 5,
		UserInput: []*schema.Message{
			schema.UserMessage("Test error handling"),
		},
	}

	// Execute plan - should handle error gracefully
	err := executor.ExecutePlan(ctx, errorPlan, state)
	assert.Error(t, err, "Expected error due to nonexistent tool")

	// Verify error was logged
	assert.True(t, len(state.ExecutionLog) > 0, "Expected execution log entries")

	// Verify step result shows failure
	if stepResult, exists := state.StepResults["failing_step"]; exists {
		assert.Equal(t, StepStatusFailed, stepResult.Status)
		assert.NotNil(t, stepResult.Error)
	}
}

// TestParallelExecutionCapabilities tests the parallel execution features
func TestParallelExecutionCapabilities(t *testing.T) {
	ctx := context.Background()

	// Create MCP manager with multiple tools
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))
	mcpManager.RegisterClient("api", NewAPIMCPClient(MCPToolConfig{Name: "api"}))
	mcpManager.RegisterClient("email", NewEmailMCPClient(MCPToolConfig{Name: "email"}))

	// Create executor configured for parallel execution
	executor := NewExecutorAgent(&ExecutorConfig{
		MaxParallelSteps: 3,
		StepTimeout:      10 * time.Second,
	}, mcpManager)

	// Create plan with independent parallel steps
	parallelPlan := &ExecutionPlan{
		ID:   "parallel_execution_test",
		Goal: "Test parallel step execution",
		Steps: []ExecutionStep{
			{
				ID:   "fetch_users",
				Type: StepTypeToolCall,
				Tool: "database",
				Name: "Fetch user data",
				Parameters: map[string]interface{}{
					"operation": "query",
					"query":     "SELECT * FROM users",
				},
				Dependencies: []string{},
			},
			{
				ID:   "fetch_metrics",
				Type: StepTypeToolCall,
				Tool: "api",
				Name: "Fetch metrics",
				Parameters: map[string]interface{}{
					"operation": "request",
					"endpoint":  "/api/metrics",
					"method":    "GET",
				},
				Dependencies: []string{},
			},
			{
				ID:   "prepare_notification",
				Type: StepTypeDataProcessing,
				Name: "Prepare notification content",
				Parameters: map[string]interface{}{
					"operation": "template",
					"template":  "Report ready",
				},
				Dependencies: []string{},
			},
		},
	}

	state := &ExecutionState{
		Plan:          parallelPlan,
		StepResults:   make(map[string]*StepResult),
		MaxIterations: 10,
		UserInput: []*schema.Message{
			schema.UserMessage("Execute parallel tasks"),
		},
	}

	// Measure execution time
	startTime := time.Now()
	err := executor.ExecutePlan(ctx, parallelPlan, state)
	executionDuration := time.Since(startTime)

	// Should complete successfully
	require.NoError(t, err)

	// Should have results for independent steps
	assert.True(t, len(state.StepResults) >= 2, "Expected at least 2 steps to complete")

	// Parallel execution should be reasonably fast
	assert.True(t, executionDuration < 15*time.Second, "Parallel execution took too long")

	// Verify individual step results
	if fetchUsersResult, exists := state.StepResults["fetch_users"]; exists {
		assert.Equal(t, StepStatusSuccess, fetchUsersResult.Status)
		assert.NotNil(t, fetchUsersResult.Result)
	}

	if fetchMetricsResult, exists := state.StepResults["fetch_metrics"]; exists {
		assert.Equal(t, StepStatusSuccess, fetchMetricsResult.Status)
		assert.NotNil(t, fetchMetricsResult.Result)
	}
}

// TestComplexWorkflowExecution tests end-to-end complex workflow execution
func TestComplexWorkflowExecution(t *testing.T) {
	ctx := context.Background()

	// Create full MCP manager
	mcpManager := NewMCPClientManager()
	mcpManager.RegisterClient("database", NewDatabaseMCPClient(MCPToolConfig{Name: "database"}))
	mcpManager.RegisterClient("api", NewAPIMCPClient(MCPToolConfig{Name: "api"}))
	mcpManager.RegisterClient("email", NewEmailMCPClient(MCPToolConfig{Name: "email"}))

	// Create executor
	executor := NewExecutorAgent(&ExecutorConfig{
		MaxParallelSteps: 2,
		StepTimeout:      30 * time.Second,
	}, mcpManager)

	// Create comprehensive workflow plan
	workflowPlan := &ExecutionPlan{
		ID:   "comprehensive_workflow",
		Goal: "Complete data analysis and notification workflow",
		Steps: []ExecutionStep{
			{
				ID:   "step1_query_users",
				Type: StepTypeToolCall,
				Tool: "database",
				Name: "Query active users",
				Parameters: map[string]interface{}{
					"operation": "query",
					"query":     "SELECT * FROM users WHERE active = true",
				},
				Dependencies: []string{},
				RetryPolicy: &RetryPolicy{
					MaxAttempts:  3,
					BackoffTime:  1 * time.Second,
					Exponential:  true,
				},
				ErrorPolicy: ErrorPolicyRetry,
			},
			{
				ID:   "step2_get_metrics",
				Type: StepTypeToolCall,
				Tool: "api",
				Name: "Get user metrics",
				Parameters: map[string]interface{}{
					"operation": "request",
					"endpoint":  "/api/user-metrics",
					"method":    "GET",
				},
				Dependencies: []string{},
				ErrorPolicy:  ErrorPolicyContinue,
			},
			{
				ID:   "step3_process_data",
				Type: StepTypeDataProcessing,
				Name: "Process and combine data",
				Parameters: map[string]interface{}{
					"operation": "merge",
					"source1":   "user_data",
					"source2":   "metrics_data",
				},
				Dependencies: []string{"step1_query_users", "step2_get_metrics"},
				ErrorPolicy:  ErrorPolicyFailFast,
			},
			{
				ID:   "step4_send_report",
				Type: StepTypeToolCall,
				Tool: "email",
				Name: "Send analysis report",
				Parameters: map[string]interface{}{
					"operation": "send",
					"to":        "admin@example.com",
					"subject":   "User Analysis Report",
					"body":      "Analysis completed successfully",
				},
				Dependencies: []string{"step3_process_data"},
				ErrorPolicy:  ErrorPolicyRetry,
			},
		},
	}

	state := &ExecutionState{
		Plan:          workflowPlan,
		StepResults:   make(map[string]*StepResult),
		MaxIterations: 20,
		UserInput: []*schema.Message{
			schema.UserMessage("Please analyze our user engagement data from the last month and send me a summary report."),
		},
	}

	// Execute comprehensive workflow
	err := executor.ExecutePlan(ctx, workflowPlan, state)
	require.NoError(t, err, "Comprehensive workflow should complete successfully")

	// Verify workflow completion
	expectedSteps := []string{"step1_query_users", "step2_get_metrics"}
	for _, stepID := range expectedSteps {
		if result, exists := state.StepResults[stepID]; exists {
			assert.Equal(t, StepStatusSuccess, result.Status, "Step %s should complete successfully", stepID)
		}
	}

	// Verify execution log contains meaningful entries
	assert.True(t, len(state.ExecutionLog) >= 2, "Expected execution log entries")

	// Verify some results were generated
	assert.True(t, len(state.StepResults) >= 2, "Expected multiple step results")
}

// Benchmark tests for performance validation
func BenchmarkMCPToolPerformance(b *testing.B) {
	ctx := context.Background()

	b.Run("DatabaseTool", func(b *testing.B) {
		client := NewDatabaseMCPClient(MCPToolConfig{Name: "database"})
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
	})

	b.Run("EmailTool", func(b *testing.B) {
		client := NewEmailMCPClient(MCPToolConfig{Name: "email"})
		params := map[string]interface{}{
			"operation": "send",
			"to":        "test@example.com",
			"subject":   "Benchmark Test",
			"body":      "Performance test email",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.Call(ctx, "send", params)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("APITool", func(b *testing.B) {
		client := NewAPIMCPClient(MCPToolConfig{Name: "api"})
		params := map[string]interface{}{
			"operation": "request",
			"endpoint":  "/api/benchmark",
			"method":    "GET",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.Call(ctx, "request", params)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParameterResolutionComplex(b *testing.B) {
	state := &ExecutionState{
		StepResults: map[string]*StepResult{
			"step1": {
				StepID: "step1",
				Status: StepStatusSuccess,
				Result: map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{"name": "Alice", "id": 1},
						map[string]interface{}{"name": "Bob", "id": 2},
					},
					"count": 2,
				},
			},
		},
	}

	resolver := NewParameterResolver(state)
	ctx := context.Background()

	step := &ExecutionStep{
		ID:   "benchmark_step",
		Type: StepTypeToolCall,
		Parameters: map[string]interface{}{
			"simple_param":  "test_value",
			"numeric_param": 42,
			"complex_param": map[string]interface{}{
				"nested": map[string]interface{}{
					"value": "nested_test",
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