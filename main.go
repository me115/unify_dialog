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

// MockChatModel 提供BaseChatModel的模拟实现用于演示
type MockChatModel struct {
	name string
}

func NewMockChatModel(name string) *MockChatModel {
	return &MockChatModel{name: name}
}

func (m *MockChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// 提取最新的用户消息
	var userMessage string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			userMessage = messages[i].Content
			break
		}
	}

	// 根据模型类型和用户消息生成响应
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
	// 为简化起见，我们只调用Generate并将其包装在流中
	msg, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}

	// 创建一个简单的流读取器，返回消息一次
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *MockChatModel) generatePlannerResponse(userMessage string) string {
	// 根据用户输入生成模拟执行计划
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
	// 生成模拟监督者决策
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

	// 创建智能体配置
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

	// 创建模拟模型
	plannerModel := NewMockChatModel("planner")
	supervisorModel := NewMockChatModel("supervisor")

	// 创建统一对话智能体
	agent, err := NewUnifiedDialogAgent(ctx, config, plannerModel, supervisorModel)
	if err != nil {
		log.Fatalf("Failed to create unified dialog agent: %v", err)
	}

	// 测试智能体的示例用户输入
	testCases := []struct {
		name    string
		input   string
		description string
	}{
		{
			name:    "Data Analysis Request",
			input:   "Please analyze our user engagement data from the last month and send me a summary report.",
			description: "测试包含数据库查询、数据处理和通知的多步骤工作流",
		},
		{
			name:    "Customer Support",
			input:   "A customer complained about their order. Can you look up their order details and send them an update?",
			description: "测试客户数据查找和通信工作流",
		},
		{
			name:    "System Health Check",
			input:   "Check the health of all our services and create a status report.",
			description: "测试并行API调用和报告生成",
		},
	}

	fmt.Println("\n🧪 Running Test Cases")
	fmt.Println("---------------------")

	for i, testCase := range testCases {
		fmt.Printf("\n%d. %s\n", i+1, testCase.name)
		fmt.Printf("   Description: %s\n", testCase.description)
		fmt.Printf("   Input: %s\n", testCase.input)

		// 创建用户输入消息
		userInput := []*schema.Message{
			{
				Role:    schema.User,
				Content: testCase.input,
			},
		}

		// 处理用户输入
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

		// 显示响应
		if len(response) > 0 {
			fmt.Printf("   Response: %s\n", response[0].Content)
		}

		// 在测试用例之间添加延迟
		time.Sleep(500 * time.Millisecond)
	}

	// 演示MCP客户端管理器功能
	fmt.Println("\n🔧 MCP Client Manager Demo")
	fmt.Println("--------------------------")

	mcpManager := agent.GetMCPManager()
	clients := mcpManager.GetAllClients()

	fmt.Printf("Registered MCP Clients: %d\n", len(clients))
	for name, client := range clients {
		fmt.Printf("- %s: %s\n", name, client.Name())

		// 测试客户端健康状态
		healthy := client.IsHealthy(ctx)
		status := "❌ Unhealthy"
		if healthy {
			status = "✅ Healthy"
		}
		fmt.Printf("  Status: %s\n", status)

		// 获取工具信息
		toolInfo, err := client.GetToolInfo(ctx)
		if err == nil {
			fmt.Printf("  Description: %s\n", toolInfo.Desc)
		}
	}

	// 演示参数解析
	fmt.Println("\n🔍 Parameter Resolution Demo")
	fmt.Println("----------------------------")

	// 创建测试执行状态
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

	// 测试参数解析
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