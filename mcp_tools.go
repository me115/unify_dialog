package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// MCPToolManager manages MCP tools with official component support
type MCPToolManager struct {
	config *UnifyDialogConfig
	tools  map[string]tool.BaseTool
}

// NewMCPToolManager creates a new MCP tool manager
func NewMCPToolManager(config *UnifyDialogConfig) *MCPToolManager {
	return &MCPToolManager{
		config: config,
		tools:  make(map[string]tool.BaseTool),
	}
}

// Initialize sets up MCP tools
func (m *MCPToolManager) Initialize(ctx context.Context) error {
	if !m.config.MCP.Enabled {
		log.Println("MCP is disabled, using mock tools")
		return m.initializeMockTools()
	}

	// Future: Use official MCP component
	// This would be the real implementation with eino-ext/components/tool/mcp
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

	// For now, fall back to mock tools
	log.Println("Official MCP support not yet integrated, using mock tools")
	return m.initializeMockTools()
}

// initializeMockTools creates mock tools for testing
func (m *MCPToolManager) initializeMockTools() error {
	// Create mock tools that simulate MCP tools
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

	log.Printf("Initialized %d mock MCP tools", len(m.tools))
	return nil
}

// GetTools returns all available tools
func (m *MCPToolManager) GetTools() []tool.BaseTool {
	tools := make([]tool.BaseTool, 0, len(m.tools))
	for _, t := range m.tools {
		tools = append(tools, t)
	}
	return tools
}

// GetTool returns a specific tool by name
func (m *MCPToolManager) GetTool(name string) (tool.BaseTool, bool) {
	t, ok := m.tools[name]
	return t, ok
}

// Mock tool handlers
func (m *MCPToolManager) handleDatabaseQuery(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
	}

	// Simulate database query
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

	// Simulate file read
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

	// Simulate API request
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

	// Simulate web search
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

// MCPMockTool implements a mock MCP tool
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

// ToolRegistry provides a registry of all available tools
type ToolRegistry struct {
	mcpManager *MCPToolManager
	customTools map[string]tool.BaseTool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(mcpManager *MCPToolManager) *ToolRegistry {
	return &ToolRegistry{
		mcpManager: mcpManager,
		customTools: make(map[string]tool.BaseTool),
	}
}

// RegisterCustomTool registers a custom tool
func (r *ToolRegistry) RegisterCustomTool(name string, tool tool.BaseTool) {
	r.customTools[name] = tool
}

// GetAllTools returns all available tools (MCP + custom)
func (r *ToolRegistry) GetAllTools() []tool.BaseTool {
	tools := r.mcpManager.GetTools()
	for _, t := range r.customTools {
		tools = append(tools, t)
	}
	return tools
}

// GetToolByName returns a tool by name
func (r *ToolRegistry) GetToolByName(name string) (tool.BaseTool, bool) {
	// Check MCP tools first
	if t, ok := r.mcpManager.GetTool(name); ok {
		return t, true
	}

	// Check custom tools
	if t, ok := r.customTools[name]; ok {
		return t, true
	}

	return nil, false
}