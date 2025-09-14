package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// MCPToolManager 管理具有官方组件支持的MCP工具
type MCPToolManager struct {
	config *UnifyDialogConfig
	tools  map[string]tool.BaseTool
}

// NewMCPToolManager 创建新的MCP工具管理器
func NewMCPToolManager(config *UnifyDialogConfig) *MCPToolManager {
	return &MCPToolManager{
		config: config,
		tools:  make(map[string]tool.BaseTool),
	}
}

// Initialize 设置MCP工具
func (m *MCPToolManager) Initialize(ctx context.Context) error {
	if !m.config.MCP.Enabled {
		log.Println("MCP已禁用，使用模拟工具")
		return m.initializeMockTools()
	}

	// 未来：使用官方MCP组件
	// 这是使用eino-ext/components/tool/mcp的真实实现
	/*
	import "github.com/cloudwego/eino-ext/components/tool/mcp"
	import "github.com/mark3labs/mcp-go/client"

	for serverName, serverConfig := range m.config.MCP.Servers {
		log.Printf("Initializing MCP server: %s", serverName)

		// Create MCP client
		mcpClient, err := client.NewStdioMCPClient(
			serverConfig.Command,
			serverConfig.Env,
			serverConfig.Args...,
		)
		if err != nil {
			return fmt.Errorf("failed to create MCP client for %s: %w", serverName, err)
		}

		// Get tools from MCP server
		tools, err := mcp.GetTools(ctx, &mcp.Config{
			Cli: mcpClient,
			// Optionally specify tool names if we want to filter
			// ToolNameList: []string{"specific_tool1", "specific_tool2"},
		})
		if err != nil {
			return fmt.Errorf("failed to get tools from %s: %w", serverName, err)
		}

		// Register tools
		for _, tool := range tools {
			toolName := fmt.Sprintf("%s_%s", serverName, tool.GetToolName())
			m.tools[toolName] = tool
			log.Printf("Registered tool: %s", toolName)
		}
	}
	*/

	// 现在回退到模拟工具
	log.Println("官方MCP支持尚未集成，使用模拟工具")
	return m.initializeMockTools()
}

// initializeMockTools 为测试创建模拟工具
func (m *MCPToolManager) initializeMockTools() error {
	// 创建模拟MCP工具的模拟工具
	m.tools["database_query"] = &MCPMockTool{
		name:        "database_query",
		description: "Query a database",
		handler:     m.handleDatabaseQuery,
	}

	m.tools["file_read"] = &MCPMockTool{
		name:        "file_read",
		description: "Read a file",
		handler:     m.handleFileRead,
	}

	m.tools["api_request"] = &MCPMockTool{
		name:        "api_request",
		description: "Make an API request",
		handler:     m.handleAPIRequest,
	}

	m.tools["web_search"] = &MCPMockTool{
		name:        "web_search",
		description: "Search the web",
		handler:     m.handleWebSearch,
	}

	log.Printf("初始化了 %d 个模拟MCP工具", len(m.tools))
	return nil
}

// GetTools 返回所有可用工具
func (m *MCPToolManager) GetTools() []tool.BaseTool {
	tools := make([]tool.BaseTool, 0, len(m.tools))
	for _, t := range m.tools {
		tools = append(tools, t)
	}
	return tools
}

// GetTool 根据名称返回特定工具
func (m *MCPToolManager) GetTool(name string) (tool.BaseTool, bool) {
	t, ok := m.tools[name]
	return t, ok
}

// 模拟工具处理器
func (m *MCPToolManager) handleDatabaseQuery(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
	}

	// 模拟数据库查询
	result := map[string]interface{}{
		"query":   query,
		"results": []map[string]interface{}{
			{"id": 1, "name": "Sample Record 1"},
			{"id": 2, "name": "Sample Record 2"},
		},
		"count": 2,
	}

	if m.config.System.Debug {
		log.Printf("[MCP] Database query executed: %s", query)
	}

	return result, nil
}

func (m *MCPToolManager) handleFileRead(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	// 模拟文件读取
	content := fmt.Sprintf("Mock content of file: %s", path)

	if m.config.System.Debug {
		log.Printf("[MCP] File read: %s", path)
	}

	return map[string]interface{}{
		"path":    path,
		"content": content,
		"size":    len(content),
	}, nil
}

func (m *MCPToolManager) handleAPIRequest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	url, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter is required")
	}

	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}

	// 模拟API请求
	response := map[string]interface{}{
		"status": 200,
		"body": map[string]interface{}{
			"message": "Mock API response",
			"data":    "sample data",
		},
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
	}

	if m.config.System.Debug {
		log.Printf("[MCP] API request: %s %s", method, url)
	}

	return response, nil
}

func (m *MCPToolManager) handleWebSearch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
	}

	// 模拟网络搜索
	results := []map[string]interface{}{
		{
			"title":   "Result 1",
			"url":     "https://example.com/1",
			"snippet": fmt.Sprintf("Mock search result for: %s", query),
		},
		{
			"title":   "Result 2",
			"url":     "https://example.com/2",
			"snippet": fmt.Sprintf("Another mock result for: %s", query),
		},
	}

	if m.config.System.Debug {
		log.Printf("[MCP] Web search: %s", query)
	}

	return map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	}, nil
}

// MCPMockTool 实现模拟MCP工具
type MCPMockTool struct {
	name        string
	description string
	handler     func(context.Context, map[string]interface{}) (interface{}, error)
}

func (t *MCPMockTool) Info(ctx context.Context) (*tool.Info, error) {
	return &tool.Info{
		Name: t.name,
		Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"params": {
				Type:        schema.ParameterTypeObject,
				Description: "Tool parameters",
				Required:    false,
			},
		}),
	}, nil
}

func (t *MCPMockTool) Run(ctx context.Context, params string, options ...tool.Option) (string, error) {
	var paramsMap map[string]interface{}
	if err := json.Unmarshal([]byte(params), &paramsMap); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	result, err := t.handler(ctx, paramsMap)
	if err != nil {
		return "", err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// ToolRegistry 提供所有可用工具的注册表
type ToolRegistry struct {
	mcpManager *MCPToolManager
	customTools map[string]tool.BaseTool
}

// NewToolRegistry 创建新的工具注册表
func NewToolRegistry(mcpManager *MCPToolManager) *ToolRegistry {
	return &ToolRegistry{
		mcpManager: mcpManager,
		customTools: make(map[string]tool.BaseTool),
	}
}

// RegisterCustomTool 注册自定义工具
func (r *ToolRegistry) RegisterCustomTool(name string, tool tool.BaseTool) {
	r.customTools[name] = tool
}

// GetAllTools 返回所有可用工具（MCP + 自定义）
func (r *ToolRegistry) GetAllTools() []tool.BaseTool {
	tools := r.mcpManager.GetTools()
	for _, t := range r.customTools {
		tools = append(tools, t)
	}
	return tools
}

// GetToolByName 根据名称返回工具
func (r *ToolRegistry) GetToolByName(name string) (tool.BaseTool, bool) {
	// 首先检查MCP工具
	if t, ok := r.mcpManager.GetTool(name); ok {
		return t, true
	}

	// 检查自定义工具
	if t, ok := r.customTools[name]; ok {
		return t, true
	}

	return nil, false
}