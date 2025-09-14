package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// ImprovedUnifyDialogSystem 表示改进的统一对话系统
type ImprovedUnifyDialogSystem struct {
	config           *UnifyDialogConfig
	modelFactory     *ModelFactory
	mcpToolManager   *MCPToolManager
	toolRegistry     *ToolRegistry
	callbackManager  *CallbackManager
	compiledGraph    compose.Runnable[string, *FinalOutput]
}

// NewImprovedUnifyDialogSystem 创建新的改进统一对话系统
func NewImprovedUnifyDialogSystem(config *UnifyDialogConfig) (*ImprovedUnifyDialogSystem, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &ImprovedUnifyDialogSystem{
		config:          config,
		modelFactory:    NewModelFactory(config),
		mcpToolManager:  NewMCPToolManager(config),
		callbackManager: NewCallbackManager(config),
	}, nil
}

// Initialize 设置系统组件
func (s *ImprovedUnifyDialogSystem) Initialize(ctx context.Context) error {
	// 初始化MCP工具
	if err := s.mcpToolManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize MCP tools: %w", err)
	}

	// 设置工具注册表
	s.toolRegistry = NewToolRegistry(s.mcpToolManager)

	// 初始化回调
	if err := s.callbackManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize callbacks: %w", err)
	}

	// 构建多智能体图
	if err := s.buildGraph(ctx); err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	log.Println("Improved UnifyDialog system initialized successfully")
	return nil
}

// buildGraph 构建多智能体编排图
func (s *ImprovedUnifyDialogSystem) buildGraph(ctx context.Context) error {
	// 使用适当的类型参数创建图
	g := compose.NewGraph[string, *FinalOutput]()

	// 使用真实模型创建智能体
	plannerModel, err := s.modelFactory.CreatePlannerModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to create planner model: %w", err)
	}

	executorModel, err := s.modelFactory.CreateExecutorModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to create executor model: %w", err)
	}

	supervisorModel, err := s.modelFactory.CreateSupervisorModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to create supervisor model: %w", err)
	}

	// 创建提示模板
	plannerPrompt := prompt.FromMessages(
		schema.SystemMessage("You are a planning agent. Create a detailed plan to address the user's query."),
		schema.UserMessage("Query: {{.query}}"),
	)

	executorPrompt := prompt.FromMessages(
		schema.SystemMessage("You are an execution agent. Execute the given plan step by step."),
		schema.UserMessage("Plan: {{.plan}}"),
	)

	supervisorPrompt := prompt.FromMessages(
		schema.SystemMessage("You are a supervisor agent. Evaluate the execution results and provide feedback."),
		schema.UserMessage("Results: {{.results}}"),
	)

	// 向图中添加节点
	if err := g.AddChatTemplateNode("planner_prompt", plannerPrompt); err != nil {
		return err
	}
	if err := g.AddChatModelNode("planner", plannerModel); err != nil {
		return err
	}

	if err := g.AddLambdaNode("plan_parser", compose.InvokableLambda(s.parsePlan)); err != nil {
		return err
	}

	if err := g.AddChatTemplateNode("executor_prompt", executorPrompt); err != nil {
		return err
	}
	if err := g.AddChatModelNode("executor", executorModel); err != nil {
		return err
	}

	// 如果工具可用，添加工具节点
	if tools := s.toolRegistry.GetAllTools(); len(tools) > 0 {
		toolsNode := compose.NewToolNode("tools", tools...)
		if err := g.AddToolsNode("tools", toolsNode); err != nil {
			return err
		}
	}

	if err := g.AddChatTemplateNode("supervisor_prompt", supervisorPrompt); err != nil {
		return err
	}
	if err := g.AddChatModelNode("supervisor", supervisorModel); err != nil {
		return err
	}

	if err := g.AddLambdaNode("final_output", compose.InvokableLambda(s.prepareFinalOutput)); err != nil {
		return err
	}

	// 用边连接节点
	g.AddEdge(compose.START, "planner_prompt")
	g.AddEdge("planner_prompt", "planner")
	g.AddEdge("planner", "plan_parser")
	g.AddEdge("plan_parser", "executor_prompt")
	g.AddEdge("executor_prompt", "executor")

	// 为工具执行添加条件边
	g.AddBranch("executor", s.checkToolExecution)

	if len(s.toolRegistry.GetAllTools()) > 0 {
		g.AddEdge("tools", "supervisor_prompt")
	}

	g.AddEdge("supervisor_prompt", "supervisor")
	g.AddEdge("supervisor", "final_output")
	g.AddEdge("final_output", compose.END)

	// 使用回调编译图
	options := []compose.GraphCompileOption{}
	if handlers := s.callbackManager.GetHandlers(); len(handlers) > 0 {
		for _, handler := range handlers {
			options = append(options, compose.WithGraphCallbacks(handler))
		}
	}

	compiledGraph, err := g.Compile(ctx, options...)
	if err != nil {
		return fmt.Errorf("failed to compile graph: %w", err)
	}

	s.compiledGraph = compiledGraph
	return nil
}

// Process 通过多智能体系统处理用户查询
func (s *ImprovedUnifyDialogSystem) Process(ctx context.Context, query string) (*FinalOutput, error) {
	log.Printf("Processing query: %s", query)

	// 使用回调运行图
	options := []compose.GraphRunOption{}
	if handlers := s.callbackManager.GetHandlers(); len(handlers) > 0 {
		for _, handler := range handlers {
			options = append(options, compose.WithCallbacks(handler))
		}
	}

	output, err := s.compiledGraph.Invoke(ctx, query, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to process query: %w", err)
	}

	return output, nil
}

// 图节点的辅助方法

func (s *ImprovedUnifyDialogSystem) parsePlan(ctx context.Context, input *schema.Message) (map[string]interface{}, error) {
	// 将规划器的输出解析为结构化计划
	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(input.Content), &plan); err != nil {
		// 如果JSON解析失败，使用原始内容
		plan = map[string]interface{}{
			"raw_plan": input.Content,
		}
	}
	return plan, nil
}

func (s *ImprovedUnifyDialogSystem) checkToolExecution(ctx context.Context, input *schema.Message) (string, error) {
	// 检查执行器的输出是否包含工具调用
	if input.ToolCalls != nil && len(input.ToolCalls) > 0 {
		return "tools", nil
	}
	return "supervisor_prompt", nil
}

func (s *ImprovedUnifyDialogSystem) prepareFinalOutput(ctx context.Context, input *schema.Message) (*FinalOutput, error) {
	// 准备最终输出结构
	return &FinalOutput{
		Success: true,
		Result:  input.Content,
		Metadata: map[string]interface{}{
			"model_provider": s.config.ModelProvider,
			"tools_used":     len(s.toolRegistry.GetAllTools()),
		},
	}, nil
}

// ProcessWithStream 处理具有流式支持的查询
func (s *ImprovedUnifyDialogSystem) ProcessWithStream(ctx context.Context, query string) (*schema.StreamReader[*FinalOutput], error) {
	log.Printf("Processing query with streaming: %s", query)

	// 使用流式运行图
	options := []compose.GraphRunOption{}
	if handlers := s.callbackManager.GetHandlers(); len(handlers) > 0 {
		for _, handler := range handlers {
			options = append(options, compose.WithCallbacks(handler))
		}
	}

	stream, err := s.compiledGraph.Stream(ctx, query, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to stream query: %w", err)
	}

	return stream, nil
}

// 配置文件示例创建
func createExampleConfigFile() {
	config := `# UnifyDialog Configuration File
# This file configures the improved unified dialog system

# Model provider: openai, deepseek, claude, mock
model_provider: mock

# OpenAI Configuration
openai:
  base_url: https://api.openai.com/v1
  api_key: ${OPENAI_API_KEY}
  model: gpt-4o-mini

# DeepSeek Configuration
deepseek:
  base_url: https://api.deepseek.com/v1
  api_key: ${DEEPSEEK_API_KEY}
  model: deepseek-chat

# Claude Configuration
claude:
  base_url: https://api.anthropic.com/v1
  api_key: ${CLAUDE_API_KEY}
  model: claude-3-sonnet-20240229

# MCP Configuration
mcp:
  enabled: false
  servers:
    filesystem:
      command: python
      args: ["-m", "mcp_server_filesystem"]
      env:
        MCP_SERVER_PORT: "3000"
    database:
      command: python
      args: ["-m", "mcp_server_database"]
      env:
        DATABASE_URL: "postgresql://localhost/mydb"

# Model Parameters
temperature: 0.7
max_tokens: 2000
top_p: 0.9

# System Configuration
system:
  debug: true
  enable_callbacks: true
  log_level: info

# Callback Configuration
callbacks:
  cozeloop:
    enabled: false
    api_token: ${COZELOOP_API_TOKEN}
    workspace_id: ${COZELOOP_WORKSPACE_ID}

  langsmith:
    enabled: false
    api_key: ${LANGSMITH_API_KEY}
    endpoint: https://api.smith.langchain.com
`

	if err := os.WriteFile("config.example.yaml", []byte(config), 0644); err != nil {
		log.Printf("Failed to create example config file: %v", err)
	} else {
		log.Println("Created config.example.yaml")
	}
}

// 改进系统的主函数
func runImprovedSystem() {
	var (
		configFile = flag.String("config", "", "Path to configuration file")
		query      = flag.String("query", "", "Query to process")
		createConfig = flag.Bool("create-config", false, "Create example config file")
	)
	flag.Parse()

	if *createConfig {
		createExampleConfigFile()
		return
	}

	// 加载配置
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 创建并初始化系统
	ctx := context.Background()
	system, err := NewImprovedUnifyDialogSystem(config)
	if err != nil {
		log.Fatalf("Failed to create system: %v", err)
	}

	if err := system.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize system: %v", err)
	}

	// 如果提供了查询则处理查询
	if *query != "" {
		output, err := system.Process(ctx, *query)
		if err != nil {
			log.Fatalf("Failed to process query: %v", err)
		}

		// 打印输出
		outputJSON, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(outputJSON))
	} else {
		// 交互模式
		fmt.Println("Improved UnifyDialog System - Interactive Mode")
		fmt.Println("Type 'exit' to quit")
		fmt.Println("----------------------------------------")

		for {
			fmt.Print("> ")
			var input string
			fmt.Scanln(&input)

			if input == "exit" {
				break
			}

			output, err := system.Process(ctx, input)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			fmt.Printf("Response: %s\n", output.Result)
		}
	}
}

// 演示流式处理的辅助函数
func demonstrateStreaming(system *ImprovedUnifyDialogSystem, ctx context.Context, query string) {
	stream, err := system.ProcessWithStream(ctx, query)
	if err != nil {
		log.Printf("Streaming failed: %v", err)
		return
	}

	fmt.Println("Streaming response:")
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Printf("Stream error: %v", err)
			break
		}

		fmt.Printf("Chunk: %+v\n", chunk)
	}
}