package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// SimpleUnifiedDialogAgent provides a simplified demonstration
type SimpleUnifiedDialogAgent struct {
	config      *AgentConfig
	mcpClients  map[string]MCPClient
}

// NewSimpleAgent creates a simplified agent for demonstration
func NewSimpleAgent() *SimpleUnifiedDialogAgent {
	// Create basic configuration
	config := &AgentConfig{
		MaxIterations: 5,
		Tools: []MCPToolConfig{
			{Name: "database", Description: "Database operations", Timeout: 10 * time.Second},
			{Name: "api", Description: "API requests", Timeout: 10 * time.Second},
			{Name: "email", Description: "Email operations", Timeout: 5 * time.Second},
		},
	}

	// Create MCP clients
	clients := make(map[string]MCPClient)
	for _, toolConfig := range config.Tools {
		switch toolConfig.Name {
		case "database":
			clients[toolConfig.Name] = NewDatabaseMCPClient(toolConfig)
		case "api":
			clients[toolConfig.Name] = NewAPIMCPClient(toolConfig)
		case "email":
			clients[toolConfig.Name] = NewEmailMCPClient(toolConfig)
		}
	}

	return &SimpleUnifiedDialogAgent{
		config:     config,
		mcpClients: clients,
	}
}

// ProcessRequest processes a user request and returns results
func (s *SimpleUnifiedDialogAgent) ProcessRequest(ctx context.Context, userRequest string) (string, error) {
	fmt.Printf("📝 Processing request: %s\n", userRequest)

	// PHASE 1: Generate a simple plan
	plan := s.generateSimplePlan(userRequest)
	fmt.Printf("📋 Generated plan with %d steps\n", len(plan.Steps))

	// PHASE 2: Execute the plan
	results, err := s.executePlan(ctx, plan)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// PHASE 3: Generate response
	response := s.generateResponse(plan, results)
	return response, nil
}

// generateSimplePlan creates a basic execution plan
func (s *SimpleUnifiedDialogAgent) generateSimplePlan(userRequest string) *ExecutionPlan {
	plan := &ExecutionPlan{
		ID:        fmt.Sprintf("plan_%d", time.Now().Unix()),
		CreatedAt: time.Now(),
		Goal:      fmt.Sprintf("Process user request: %s", userRequest),
		Steps:     []ExecutionStep{},
	}

	// Generate steps based on request content
	if contains(userRequest, "database", "data", "query", "users") {
		plan.Steps = append(plan.Steps, ExecutionStep{
			ID:   "step_db",
			Type: StepTypeToolCall,
			Tool: "database",
			Name: "Query database",
			Parameters: map[string]interface{}{
				"operation": "query",
				"query":     "SELECT * FROM users LIMIT 10",
			},
		})
	}

	if contains(userRequest, "api", "fetch", "get", "request") {
		plan.Steps = append(plan.Steps, ExecutionStep{
			ID:   "step_api",
			Type: StepTypeToolCall,
			Tool: "api",
			Name: "Fetch data from API",
			Parameters: map[string]interface{}{
				"operation": "request",
				"endpoint":  "https://api.example.com/data",
				"method":    "GET",
			},
		})
	}

	if contains(userRequest, "email", "send", "notify", "notification") {
		plan.Steps = append(plan.Steps, ExecutionStep{
			ID:   "step_email",
			Type: StepTypeToolCall,
			Tool: "email",
			Name: "Send notification",
			Parameters: map[string]interface{}{
				"operation": "send",
				"to":        "user@example.com",
				"subject":   "Request processed",
				"body":      "Your request has been completed successfully.",
			},
		})
	}

	// If no specific tools identified, add a generic processing step
	if len(plan.Steps) == 0 {
		plan.Steps = append(plan.Steps, ExecutionStep{
			ID:   "step_process",
			Type: StepTypeDataProcessing,
			Name: "Process request",
			Parameters: map[string]interface{}{
				"operation": "process",
				"input":     userRequest,
			},
		})
	}

	return plan
}

// executePlan executes the generated plan
func (s *SimpleUnifiedDialogAgent) executePlan(ctx context.Context, plan *ExecutionPlan) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	for i, step := range plan.Steps {
		fmt.Printf("🔄 Executing step %d/%d: %s\n", i+1, len(plan.Steps), step.Name)

		result, err := s.executeStep(ctx, &step)
		if err != nil {
			fmt.Printf("❌ Step failed: %v\n", err)
			// Simple error handling - continue with next step
			results[step.ID] = map[string]interface{}{
				"error":  err.Error(),
				"status": "failed",
			}
			continue
		}

		fmt.Printf("✅ Step completed successfully\n")
		results[step.ID] = result
	}

	return results, nil
}

// executeStep executes a single step
func (s *SimpleUnifiedDialogAgent) executeStep(ctx context.Context, step *ExecutionStep) (interface{}, error) {
	switch step.Type {
	case StepTypeToolCall:
		client, exists := s.mcpClients[step.Tool]
		if !exists {
			return nil, fmt.Errorf("tool not found: %s", step.Tool)
		}

		operation, _ := step.Parameters["operation"].(string)
		return client.Call(ctx, operation, step.Parameters)

	case StepTypeDataProcessing:
		// Simple data processing
		return map[string]interface{}{
			"processed": true,
			"input":     step.Parameters["input"],
			"timestamp": time.Now(),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

// generateResponse creates a final response
func (s *SimpleUnifiedDialogAgent) generateResponse(plan *ExecutionPlan, results map[string]interface{}) string {
	var response []string
	response = append(response, fmt.Sprintf("✅ Successfully processed your request: %s", plan.Goal))
	response = append(response, fmt.Sprintf("📊 Executed %d steps:", len(plan.Steps)))

	for _, step := range plan.Steps {
		if result, exists := results[step.ID]; exists {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if _, hasError := resultMap["error"]; hasError {
					response = append(response, fmt.Sprintf("  ❌ %s: Failed", step.Name))
				} else {
					response = append(response, fmt.Sprintf("  ✅ %s: Completed", step.Name))
				}
			} else {
				response = append(response, fmt.Sprintf("  ✅ %s: Completed", step.Name))
			}
		}
	}

	return fmt.Sprintf("%s\n", response)
}

// Helper function
func contains(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if fmt.Sprintf("%s", text) == keyword {
			return true
		}
	}
	return false
}

// Main function for demonstration
func runSimpleDemo() {
	fmt.Println("🤖 Simple Unified Dialog Agent Demo")
	fmt.Println("===================================")

	ctx := context.Background()
	agent := NewSimpleAgent()

	// Test cases
	testCases := []string{
		"Please query the database for user information",
		"Fetch data from the API and send me an email notification",
		"Process my request and analyze the results",
		"Get user data, call the external API, and send a summary email",
	}

	for i, testCase := range testCases {
		fmt.Printf("\n🧪 Test Case %d:\n", i+1)
		fmt.Printf("Input: %s\n", testCase)

		start := time.Now()
		response, err := agent.ProcessRequest(ctx, testCase)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ Failed (%.2fs): %v\n", duration.Seconds(), err)
		} else {
			fmt.Printf("⏱️  Completed in %.2fs\n", duration.Seconds())
			fmt.Printf("Response: %s\n", response)
		}
	}

	// Demonstrate MCP client functionality
	fmt.Println("\n🔧 MCP Client Demo")
	fmt.Println("------------------")

	for name, client := range agent.mcpClients {
		fmt.Printf("Testing %s client:\n", name)

		result, err := client.Call(ctx, "health", map[string]interface{}{})
		if err != nil {
			fmt.Printf("  ❌ Health check failed: %v\n", err)
		} else {
			fmt.Printf("  ✅ Health check passed\n")
			resultJSON, _ := json.MarshalIndent(result, "  ", "  ")
			fmt.Printf("  Result: %s\n", resultJSON)
		}
	}

	// Demonstrate parameter resolution
	fmt.Println("\n🔍 Parameter Resolution Demo")
	fmt.Println("----------------------------")

	state := &ExecutionState{
		DataStore: map[string]interface{}{
			"user_data": map[string]interface{}{
				"users": []map[string]interface{}{
					{"id": 1, "name": "Alice"},
					{"id": 2, "name": "Bob"},
				},
				"count": 2,
			},
		},
	}

	resolver := NewParameterResolver(state)
	testStep := &ExecutionStep{
		Parameters: map[string]interface{}{
			"message": "Found ${steps.user_data.count} users",
			"user_ref": map[string]interface{}{
				"type":      "data_store",
				"reference": "user_data",
				"path":      "users[0].name",
			},
		},
	}

	resolved, err := resolver.ResolveParameters(ctx, testStep)
	if err != nil {
		fmt.Printf("❌ Parameter resolution failed: %v\n", err)
	} else {
		fmt.Printf("✅ Parameter resolution successful:\n")
		resolvedJSON, _ := json.MarshalIndent(resolved, "  ", "  ")
		fmt.Printf("%s\n", resolvedJSON)
	}

	fmt.Println("\n🎉 Demo completed!")
}

func main() {
	runSimpleDemo()
}