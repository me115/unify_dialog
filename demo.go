package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Basic types without EINO dependencies

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ExecutionPlan represents a complex execution plan
type DemoExecutionPlan struct {
	ID        string              `json:"id"`
	CreatedAt time.Time           `json:"created_at"`
	Goal      string              `json:"goal"`
	Steps     []DemoExecutionStep `json:"steps"`
}

type DemoExecutionStep struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Tool         string                 `json:"tool,omitempty"`
	Operation    string                 `json:"operation,omitempty"`
	Name         string                 `json:"name"`
	Parameters   map[string]interface{} `json:"parameters"`
	Dependencies []string               `json:"dependencies"`
	Parallel     bool                   `json:"parallel,omitempty"`
}

type DemoMCPClient interface {
	Name() string
	Call(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error)
	IsHealthy(ctx context.Context) bool
}

type DemoBaseMCPClient struct {
	name     string
	executor func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error)
}

func (c *DemoBaseMCPClient) Name() string {
	return c.name
}

func (c *DemoBaseMCPClient) Call(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
	return c.executor(ctx, operation, parameters)
}

func (c *DemoBaseMCPClient) IsHealthy(ctx context.Context) bool {
	_, err := c.Call(ctx, "health", map[string]interface{}{})
	return err == nil
}

// Database MCP Client
func NewDemoDatabaseClient() *DemoBaseMCPClient {
	executor := func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
		switch operation {
		case "query":
			return map[string]interface{}{
				"rows": []map[string]interface{}{
					{"id": 1, "name": "Alice", "email": "alice@example.com"},
					{"id": 2, "name": "Bob", "email": "bob@example.com"},
				},
				"count": 2,
			}, nil
		case "health":
			return map[string]interface{}{"status": "healthy"}, nil
		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}
	return &DemoBaseMCPClient{name: "database", executor: executor}
}

// API MCP Client
func NewDemoAPIClient() *DemoBaseMCPClient {
	executor := func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
		switch operation {
		case "request":
			return map[string]interface{}{
				"data": map[string]interface{}{
					"weather": "sunny",
					"temp":    "22°C",
				},
				"status": 200,
			}, nil
		case "health":
			return map[string]interface{}{"status": "healthy"}, nil
		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}
	return &DemoBaseMCPClient{name: "api", executor: executor}
}

// Email MCP Client
func NewDemoEmailClient() *DemoBaseMCPClient {
	executor := func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
		switch operation {
		case "send":
			return map[string]interface{}{
				"messageId": fmt.Sprintf("msg_%d", time.Now().Unix()),
				"status":    "sent",
			}, nil
		case "health":
			return map[string]interface{}{"status": "healthy"}, nil
		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}
	return &DemoBaseMCPClient{name: "email", executor: executor}
}

// Parameter Resolver (simplified)
type DemoParameterResolver struct {
	dataStore map[string]interface{}
}

func (r *DemoParameterResolver) ResolveParameters(ctx context.Context, parameters map[string]interface{}) (map[string]interface{}, error) {
	resolved := make(map[string]interface{})

	for key, value := range parameters {
		resolvedValue, err := r.resolveValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter '%s': %w", key, err)
		}
		resolved[key] = resolvedValue
	}

	return resolved, nil
}

func (r *DemoParameterResolver) resolveValue(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Handle template variables like ${data_store.key}
		re := regexp.MustCompile(`\$\{([^}]+)\}`)
		result := v
		matches := re.FindAllStringSubmatch(v, -1)

		for _, match := range matches {
			if len(match) != 2 {
				continue
			}

			placeholder := match[0]
			reference := match[1]

			// Simple resolution from data store
			if strings.HasPrefix(reference, "data_store.") {
				key := strings.TrimPrefix(reference, "data_store.")
				if val, exists := r.dataStore[key]; exists {
					result = strings.Replace(result, placeholder, fmt.Sprintf("%v", val), -1)
				}
			}
		}
		return result, nil

	case map[string]interface{}:
		// Handle reference objects
		if refType, exists := v["type"]; exists && refType == "data_store" {
			if ref, exists := v["reference"].(string); exists {
				if val, exists := r.dataStore[ref]; exists {
					// Handle path if specified
					if path, hasPath := v["path"].(string); hasPath {
						return r.extractPath(val, path)
					}
					return val, nil
				}
			}
		}

		// Recursively resolve nested objects
		resolved := make(map[string]interface{})
		for k, val := range v {
			resolvedVal, err := r.resolveValue(val)
			if err != nil {
				return nil, err
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil

	default:
		return value, nil
	}
}

func (r *DemoParameterResolver) extractPath(data interface{}, path string) (interface{}, error) {
	if path == "" {
		return data, nil
	}

	// Simple path extraction for arrays: field[0]
	if strings.Contains(path, "[") {
		re := regexp.MustCompile(`^([^[]+)\[(\d+)\]$`)
		matches := re.FindStringSubmatch(path)
		if len(matches) == 3 {
			fieldName := matches[1]
			indexStr := matches[2]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, err
			}

			// Get field value
			if dataMap, ok := data.(map[string]interface{}); ok {
				if fieldVal, exists := dataMap[fieldName]; exists {
					if arr, ok := fieldVal.([]interface{}); ok && index < len(arr) {
						return arr[index], nil
					}
				}
			}
		}
	}

	// Simple field access
	if dataMap, ok := data.(map[string]interface{}); ok {
		return dataMap[path], nil
	}

	return nil, fmt.Errorf("cannot extract path %s from data", path)
}

// Simple Agent
type DemoUnifiedAgent struct {
	mcpClients map[string]DemoMCPClient
	resolver   *DemoParameterResolver
}

func NewDemoUnifiedAgent() *DemoUnifiedAgent {
	clients := map[string]DemoMCPClient{
		"database": NewDemoDatabaseClient(),
		"api":      NewDemoAPIClient(),
		"email":    NewDemoEmailClient(),
	}

	resolver := &DemoParameterResolver{
		dataStore: make(map[string]interface{}),
	}

	return &DemoUnifiedAgent{
		mcpClients: clients,
		resolver:   resolver,
	}
}

// ProcessRequest handles user requests
func (a *DemoUnifiedAgent) ProcessRequest(ctx context.Context, userInput string) (string, error) {
	fmt.Printf("📝 Processing: %s\n", userInput)

	// Phase 1: Generate Plan
	plan := a.generatePlan(userInput)
	fmt.Printf("📋 Generated plan with %d steps\n", len(plan.Steps))

	// Phase 2: Execute Plan
	results, err := a.executePlan(ctx, plan)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// Phase 3: Generate Response
	return a.generateResponse(plan, results), nil
}

func (a *DemoUnifiedAgent) generatePlan(userInput string) *DemoExecutionPlan {
	plan := &DemoExecutionPlan{
		ID:        fmt.Sprintf("plan_%d", time.Now().Unix()),
		CreatedAt: time.Now(),
		Goal:      fmt.Sprintf("Process: %s", userInput),
	}

	// Simple rule-based planning
	lower := strings.ToLower(userInput)

	if strings.Contains(lower, "database") || strings.Contains(lower, "query") || strings.Contains(lower, "user") {
		plan.Steps = append(plan.Steps, DemoExecutionStep{
			ID:   "step_db",
			Type: "tool_call",
			Tool: "database",
			Name: "Query database",
			Parameters: map[string]interface{}{
				"operation": "query",
			},
		})
	}

	if strings.Contains(lower, "api") || strings.Contains(lower, "fetch") || strings.Contains(lower, "weather") {
		plan.Steps = append(plan.Steps, DemoExecutionStep{
			ID:   "step_api",
			Type: "tool_call",
			Tool: "api",
			Name: "Call API",
			Parameters: map[string]interface{}{
				"operation": "request",
			},
		})
	}

	if strings.Contains(lower, "email") || strings.Contains(lower, "send") || strings.Contains(lower, "notify") {
		dependencies := []string{}
		if len(plan.Steps) > 0 {
			dependencies = []string{plan.Steps[len(plan.Steps)-1].ID}
		}

		plan.Steps = append(plan.Steps, DemoExecutionStep{
			ID:   "step_email",
			Type: "tool_call",
			Tool: "email",
			Name: "Send email",
			Parameters: map[string]interface{}{
				"operation": "send",
				"to":        "user@example.com",
				"subject":   "Request processed",
				"body":      "Results: ${data_store.summary}",
			},
			Dependencies: dependencies,
		})
	}

	if len(plan.Steps) == 0 {
		plan.Steps = append(plan.Steps, DemoExecutionStep{
			ID:   "step_default",
			Type: "data_processing",
			Name: "Process request",
			Parameters: map[string]interface{}{
				"input": userInput,
			},
		})
	}

	return plan
}

func (a *DemoUnifiedAgent) executePlan(ctx context.Context, plan *DemoExecutionPlan) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	completed := make(map[string]bool)

	for len(completed) < len(plan.Steps) {
		progress := false

		for _, step := range plan.Steps {
			if completed[step.ID] {
				continue
			}

			// Check dependencies
			canExecute := true
			for _, dep := range step.Dependencies {
				if !completed[dep] {
					canExecute = false
					break
				}
			}

			if !canExecute {
				continue
			}

			// Execute step
			fmt.Printf("🔄 Executing: %s\n", step.Name)
			result, err := a.executeStep(ctx, &step)
			if err != nil {
				fmt.Printf("❌ Step failed: %v\n", err)
				results[step.ID] = map[string]interface{}{"error": err.Error()}
			} else {
				fmt.Printf("✅ Step completed\n")
				results[step.ID] = result

				// Store result in resolver's data store
				a.resolver.dataStore[step.ID] = result
				if step.ID == "step_db" {
					a.resolver.dataStore["summary"] = "Database query completed successfully"
				}
			}

			completed[step.ID] = true
			progress = true
		}

		if !progress {
			return nil, fmt.Errorf("no progress possible - circular dependencies or missing steps")
		}
	}

	return results, nil
}

func (a *DemoUnifiedAgent) executeStep(ctx context.Context, step *DemoExecutionStep) (interface{}, error) {
	// Resolve parameters
	resolvedParams, err := a.resolver.ResolveParameters(ctx, step.Parameters)
	if err != nil {
		return nil, fmt.Errorf("parameter resolution failed: %w", err)
	}

	switch step.Type {
	case "tool_call":
		client, exists := a.mcpClients[step.Tool]
		if !exists {
			return nil, fmt.Errorf("tool not found: %s", step.Tool)
		}

		operation, _ := resolvedParams["operation"].(string)
		return client.Call(ctx, operation, resolvedParams)

	case "data_processing":
		return map[string]interface{}{
			"processed": true,
			"input":     resolvedParams["input"],
			"timestamp": time.Now(),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

func (a *DemoUnifiedAgent) generateResponse(plan *DemoExecutionPlan, results map[string]interface{}) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("✅ Completed: %s\n\n", plan.Goal))
	response.WriteString(fmt.Sprintf("📊 Executed %d steps:\n", len(plan.Steps)))

	for _, step := range plan.Steps {
		if result, exists := results[step.ID]; exists {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if _, hasError := resultMap["error"]; hasError {
					response.WriteString(fmt.Sprintf("  ❌ %s: Failed\n", step.Name))
				} else {
					response.WriteString(fmt.Sprintf("  ✅ %s: Success\n", step.Name))
				}
			}
		}
	}

	return response.String()
}

// Demo function
func runStandaloneDemo() {
	fmt.Println("🤖 Unified Dialog Agent - Standalone Demo")
	fmt.Println("==========================================")

	ctx := context.Background()
	agent := NewDemoUnifiedAgent()

	testCases := []string{
		"Query the database for user information",
		"Get weather data from the API and send me an email",
		"Process my request and analyze the data",
		"Fetch database info, call the API, and notify me via email",
	}

	for i, testCase := range testCases {
		fmt.Printf("\n🧪 Test Case %d: %s\n", i+1, testCase)
		fmt.Println(strings.Repeat("-", 50))

		start := time.Now()
		response, err := agent.ProcessRequest(ctx, testCase)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ Failed (%.2fs): %v\n", duration.Seconds(), err)
		} else {
			fmt.Printf("⏱️  Completed in %.2fs\n", duration.Seconds())
			fmt.Println(response)
		}
	}

	// MCP Client Demo
	fmt.Println("\n🔧 MCP Client Health Check")
	fmt.Println("---------------------------")
	for name, client := range agent.mcpClients {
		healthy := client.IsHealthy(ctx)
		status := "❌ Unhealthy"
		if healthy {
			status = "✅ Healthy"
		}
		fmt.Printf("%s: %s\n", name, status)
	}

	// Parameter Resolution Demo
	fmt.Println("\n🔍 Parameter Resolution Demo")
	fmt.Println("-----------------------------")

	// Set up test data
	agent.resolver.dataStore["test_data"] = map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "Alice", "id": 1},
			map[string]interface{}{"name": "Bob", "id": 2},
		},
		"count": 2,
	}
	agent.resolver.dataStore["count"] = 2

	testParams := map[string]interface{}{
		"message": "Found ${data_store.count} users",
		"user_ref": map[string]interface{}{
			"type":      "data_store",
			"reference": "test_data",
			"path":      "users[0]",
		},
	}

	resolved, err := agent.resolver.ResolveParameters(ctx, testParams)
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Success:")
		for k, v := range resolved {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	fmt.Println("\n🎉 Demo completed successfully!")
	fmt.Println("\nThis demonstrates:")
	fmt.Println("• Planner-Executor hybrid architecture")
	fmt.Println("• Dynamic parameter resolution")
	fmt.Println("• Multiple MCP tool integration")
	fmt.Println("• Dependency-based step execution")
	fmt.Println("• Error handling and recovery")
}

func main() {
	runStandaloneDemo()
}