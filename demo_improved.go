package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// DemoImprovedSystem demonstrates the improved unified dialog system
func DemoImprovedSystem() {
	fmt.Println("=== Improved UnifyDialog System Demo ===")

	// 1. Load configuration
	config, err := LoadConfig("config.example.yaml")
	if err != nil {
		log.Printf("Using default configuration: %v", err)
		config = &UnifyDialogConfig{
			ModelProvider: "mock",
			Temperature:   0.7,
			MaxTokens:     2000,
			TopP:          0.9,
		}
		config.System.Debug = true
		config.System.EnableCallbacks = true
		config.System.LogLevel = "info"
	}

	// Override with environment if needed
	if provider := os.Getenv("MODEL_PROVIDER"); provider != "" {
		config.ModelProvider = provider
	}

	ctx := context.Background()

	// 2. Demonstrate basic improved system
	fmt.Println("\n--- Basic Improved System ---")
	demoBasicImprovedSystem(ctx, config)

	// 3. Demonstrate multi-agent orchestrator
	fmt.Println("\n--- Multi-Agent Orchestrator ---")
	demoMultiAgentOrchestrator(ctx, config)

	// 4. Demonstrate configuration management
	fmt.Println("\n--- Configuration Management ---")
	demoConfigurationManagement(config)

	// 5. Demonstrate callback system
	fmt.Println("\n--- Callback System ---")
	demoCallbackSystem(ctx, config)

	// 6. Demonstrate tool integration
	fmt.Println("\n--- Tool Integration ---")
	demoToolIntegration(ctx, config)
}

func demoBasicImprovedSystem(ctx context.Context, config *UnifyDialogConfig) {
	system, err := NewImprovedUnifyDialogSystem(config)
	if err != nil {
		log.Printf("Failed to create improved system: %v", err)
		return
	}

	if err := system.Initialize(ctx); err != nil {
		log.Printf("Failed to initialize improved system: %v", err)
		return
	}

	// Test queries
	testQueries := []string{
		"What is the weather like today?",
		"Help me plan a trip to Japan",
		"Solve this math problem: 2x + 5 = 15",
	}

	for i, query := range testQueries {
		fmt.Printf("\nQuery %d: %s\n", i+1, query)
		output, err := system.Process(ctx, query)
		if err != nil {
			log.Printf("Error processing query: %v", err)
			continue
		}

		fmt.Printf("Success: %t\n", output.Success)
		fmt.Printf("Response: %s\n", output.Result)
		fmt.Printf("Metadata: %v\n", output.Metadata)
	}
}

func demoMultiAgentOrchestrator(ctx context.Context, config *UnifyDialogConfig) {
	orchestrator, err := NewMultiAgentOrchestrator(config)
	if err != nil {
		log.Printf("Failed to create multi-agent orchestrator: %v", err)
		return
	}

	if err := orchestrator.Initialize(ctx); err != nil {
		log.Printf("Failed to initialize multi-agent orchestrator: %v", err)
		return
	}

	// Test complex queries that benefit from multi-agent approach
	complexQueries := []struct {
		query   string
		context map[string]interface{}
	}{
		{
			query: "Analyze the current market trends and provide investment recommendations",
			context: map[string]interface{}{
				"risk_tolerance": "moderate",
				"time_horizon":   "5 years",
			},
		},
		{
			query: "Create a comprehensive project plan for building a mobile app",
			context: map[string]interface{}{
				"app_type": "social media",
				"budget":   "$50000",
				"timeline": "6 months",
			},
		},
	}

	for i, test := range complexQueries {
		fmt.Printf("\nComplex Query %d: %s\n", i+1, test.query)
		fmt.Printf("Context: %v\n", test.context)

		output, err := orchestrator.Process(ctx, test.query, test.context)
		if err != nil {
			log.Printf("Error processing complex query: %v", err)
			continue
		}

		fmt.Printf("Success: %t\n", output.Success)
		fmt.Printf("Response: %s\n", output.Response)

		// Show execution details
		if output.State != nil {
			fmt.Printf("Plan Steps: %d\n", len(output.State.Plan.Steps))
			fmt.Printf("Execution Log Entries: %d\n", len(output.State.ExecutionLog))
			fmt.Printf("Iterations: %d\n", output.State.CurrentIteration)
		}
	}
}

func demoConfigurationManagement(config *UnifyDialogConfig) {
	fmt.Printf("Current Configuration:\n")
	fmt.Printf("- Model Provider: %s\n", config.ModelProvider)
	fmt.Printf("- Temperature: %.2f\n", config.Temperature)
	fmt.Printf("- Max Tokens: %d\n", config.MaxTokens)
	fmt.Printf("- Debug Mode: %t\n", config.System.Debug)
	fmt.Printf("- Callbacks Enabled: %t\n", config.System.EnableCallbacks)
	fmt.Printf("- Log Level: %s\n", config.System.LogLevel)
	fmt.Printf("- MCP Enabled: %t\n", config.MCP.Enabled)

	// Show how to override with environment
	fmt.Printf("\nEnvironment Variable Support:\n")
	fmt.Printf("- OPENAI_API_KEY: %s\n", maskAPIKey(os.Getenv("OPENAI_API_KEY")))
	fmt.Printf("- DEEPSEEK_API_KEY: %s\n", maskAPIKey(os.Getenv("DEEPSEEK_API_KEY")))
	fmt.Printf("- MODEL_PROVIDER: %s\n", os.Getenv("MODEL_PROVIDER"))

	// Demonstrate validation
	fmt.Printf("\nConfiguration Validation:\n")
	if err := config.Validate(); err != nil {
		fmt.Printf("❌ Configuration invalid: %v\n", err)
	} else {
		fmt.Printf("✅ Configuration valid\n")
	}
}

func demoCallbackSystem(ctx context.Context, config *UnifyDialogConfig) {
	callbackManager := NewCallbackManager(config)
	if err := callbackManager.Initialize(ctx); err != nil {
		log.Printf("Failed to initialize callback manager: %v", err)
		return
	}

	handlers := callbackManager.GetHandlers()
	fmt.Printf("Initialized %d callback handlers:\n", len(handlers))

	// Demonstrate different handler types
	for i, handler := range handlers {
		fmt.Printf("Handler %d: %T\n", i+1, handler)
	}

	// Show what callbacks would track
	fmt.Printf("\nCallbacks would track:\n")
	fmt.Printf("- Execution start/end times\n")
	fmt.Printf("- Model inference calls\n")
	fmt.Printf("- Tool executions\n")
	fmt.Printf("- Error occurrences\n")
	fmt.Printf("- Performance metrics\n")
}

func demoToolIntegration(ctx context.Context, config *UnifyDialogConfig) {
	mcpManager := NewMCPToolManager(config)
	if err := mcpManager.Initialize(ctx); err != nil {
		log.Printf("Failed to initialize MCP manager: %v", err)
		return
	}

	tools := mcpManager.GetTools()
	fmt.Printf("Available Tools: %d\n", len(tools))

	for _, tool := range tools {
		info, err := tool.Info(ctx)
		if err != nil {
			continue
		}
		fmt.Printf("- %s: %s\n", info.Name, info.Desc)
	}

	// Demonstrate tool execution
	if len(tools) > 0 {
		fmt.Printf("\nTool Execution Demo:\n")
		tool := tools[0]
		info, _ := tool.Info(ctx)

		testParams := `{"query": "test"}`
		result, err := tool.Run(ctx, testParams)
		if err != nil {
			fmt.Printf("Tool execution failed: %v\n", err)
		} else {
			fmt.Printf("Tool %s executed successfully:\n%s\n", info.Name, result)
		}
	}
}

// Helper function to mask API keys for display
func maskAPIKey(key string) string {
	if key == "" {
		return "<not set>"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "***" + key[len(key)-4:]
}

// DemoRealWorldScenarios demonstrates real-world usage scenarios
func DemoRealWorldScenarios() {
	fmt.Println("\n=== Real-World Scenarios Demo ===")

	scenarios := []struct {
		name        string
		description string
		query       string
		context     map[string]interface{}
	}{
		{
			name:        "Customer Support",
			description: "Multi-step customer issue resolution",
			query:       "I'm having trouble with my order #12345. It was supposed to arrive yesterday but hasn't shown up.",
			context: map[string]interface{}{
				"customer_id": "CUST001",
				"order_id":    "12345",
				"priority":    "high",
			},
		},
		{
			name:        "Data Analysis",
			description: "Complex data analysis with multiple steps",
			query:       "Analyze our Q3 sales data and provide insights for Q4 strategy",
			context: map[string]interface{}{
				"data_source": "sales_db",
				"quarter":     "Q3",
				"year":        "2024",
			},
		},
		{
			name:        "Content Generation",
			description: "Multi-format content creation",
			query:       "Create marketing content for our new product launch",
			context: map[string]interface{}{
				"product":     "Smart Widget 2.0",
				"target":      "tech-savvy millennials",
				"formats":     []string{"email", "social", "blog"},
			},
		},
	}

	config := &UnifyDialogConfig{
		ModelProvider: "mock",
		Temperature:   0.7,
		MaxTokens:     2000,
	}
	config.System.Debug = true
	config.System.EnableCallbacks = true

	ctx := context.Background()

	for _, scenario := range scenarios {
		fmt.Printf("\n--- %s ---\n", scenario.name)
		fmt.Printf("Description: %s\n", scenario.description)
		fmt.Printf("Query: %s\n", scenario.query)
		fmt.Printf("Context: %v\n", scenario.context)

		orchestrator, err := NewMultiAgentOrchestrator(config)
		if err != nil {
			log.Printf("Failed to create orchestrator: %v", err)
			continue
		}

		if err := orchestrator.Initialize(ctx); err != nil {
			log.Printf("Failed to initialize orchestrator: %v", err)
			continue
		}

		output, err := orchestrator.Process(ctx, scenario.query, scenario.context)
		if err != nil {
			log.Printf("Scenario failed: %v", err)
			continue
		}

		fmt.Printf("✅ Success: %t\n", output.Success)
		fmt.Printf("Response Summary: %s\n", output.Response)

		if output.State != nil && output.State.Plan != nil {
			fmt.Printf("Execution Plan (%d steps):\n", len(output.State.Plan.Steps))
			for i, step := range output.State.Plan.Steps {
				fmt.Printf("  %d. %s\n", i+1, step.Description)
			}
		}
	}
}

// ShowMigrationGuide shows how to migrate from the old system
func ShowMigrationGuide() {
	fmt.Println("\n=== Migration Guide ===")
	fmt.Println("To migrate from the old unify_dialog system:")
	fmt.Println()
	fmt.Println("1. Update Configuration:")
	fmt.Println("   - Replace hardcoded models with ModelFactory")
	fmt.Println("   - Use config.yaml instead of environment variables")
	fmt.Println("   - Enable callbacks for better observability")
	fmt.Println()
	fmt.Println("2. Replace Components:")
	fmt.Println("   - MockChatModel -> ModelFactory.CreateXModel()")
	fmt.Println("   - MCPClientManager -> MCPToolManager")
	fmt.Println("   - Simple agents -> MultiAgentOrchestrator")
	fmt.Println()
	fmt.Println("3. Add Callbacks:")
	fmt.Println("   - Initialize CallbackManager")
	fmt.Println("   - Enable debug and performance tracking")
	fmt.Println("   - Add external callback integrations")
	fmt.Println()
	fmt.Println("4. Enable Real Models:")
	fmt.Println("   - Set MODEL_PROVIDER environment variable")
	fmt.Println("   - Configure API keys for chosen provider")
	fmt.Println("   - Update model parameters in config")
	fmt.Println()
	fmt.Println("5. Integrate MCP Tools:")
	fmt.Println("   - Configure MCP servers in config.yaml")
	fmt.Println("   - Enable MCP in configuration")
	fmt.Println("   - Test tool integrations")
}

// Performance comparison
func ShowPerformanceImprovements() {
	fmt.Println("\n=== Performance Improvements ===")
	fmt.Println("Improved system provides:")
	fmt.Println("- ✅ Structured configuration management")
	fmt.Println("- ✅ Real model provider integration")
	fmt.Println("- ✅ Official MCP tool support")
	fmt.Println("- ✅ Multi-agent orchestration patterns")
	fmt.Println("- ✅ Comprehensive callback system")
	fmt.Println("- ✅ State management and checkpoints")
	fmt.Println("- ✅ Streaming response support")
	fmt.Println("- ✅ Error handling and recovery")
	fmt.Println("- ✅ Performance monitoring")
	fmt.Println("- ✅ Extensible architecture")
}

// Main demo function
func main() {
	// Check if we should run demos
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		DemoImprovedSystem()
		DemoRealWorldScenarios()
		ShowMigrationGuide()
		ShowPerformanceImprovements()
		return
	}

	// Otherwise run the improved system
	runImprovedSystem()
}