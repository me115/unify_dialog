package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// MCPClientManager manages multiple MCP clients
type MCPClientManager struct {
	clients map[string]MCPClient
	mu      sync.RWMutex
}

// MCPClient interface for MCP tool interactions
type MCPClient interface {
	// Name returns the client name
	Name() string
	// Call executes a tool operation
	Call(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error)
	// GetToolInfo returns tool information for this client
	GetToolInfo(ctx context.Context) (*schema.ToolInfo, error)
	// IsHealthy checks if the client is healthy
	IsHealthy(ctx context.Context) bool
}

// NewMCPClientManager creates a new MCP client manager
func NewMCPClientManager() *MCPClientManager {
	return &MCPClientManager{
		clients: make(map[string]MCPClient),
	}
}

// RegisterClient registers an MCP client
func (m *MCPClientManager) RegisterClient(name string, client MCPClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[name] = client
}

// GetClient returns an MCP client by name
func (m *MCPClientManager) GetClient(name string) (MCPClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[name]
	if !exists {
		return nil, fmt.Errorf("MCP client '%s' not found", name)
	}
	return client, nil
}

// GetAllClients returns all registered clients
func (m *MCPClientManager) GetAllClients() map[string]MCPClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make(map[string]MCPClient)
	for name, client := range m.clients {
		clients[name] = client
	}
	return clients
}

// GetToolInfos returns tool information for all registered clients
func (m *MCPClientManager) GetToolInfos(ctx context.Context) ([]*schema.ToolInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var toolInfos []*schema.ToolInfo
	for _, client := range m.clients {
		toolInfo, err := client.GetToolInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get tool info for client '%s': %w", client.Name(), err)
		}
		toolInfos = append(toolInfos, toolInfo)
	}
	return toolInfos, nil
}

// BaseMCPClient provides a base implementation for MCP clients
type BaseMCPClient struct {
	name        string
	description string
	config      MCPToolConfig
	executor    func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error)
}

// NewBaseMCPClient creates a new base MCP client
func NewBaseMCPClient(config MCPToolConfig, executor func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error)) *BaseMCPClient {
	return &BaseMCPClient{
		name:        config.Name,
		description: config.Description,
		config:      config,
		executor:    executor,
	}
}

// Name returns the client name
func (c *BaseMCPClient) Name() string {
	return c.name
}

// Call executes a tool operation with timeout and retry logic
func (c *BaseMCPClient) Call(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
	// Apply timeout
	if c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	var lastErr error
	maxRetries := c.config.Retries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err := c.executor(ctx, operation, parameters)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry on context cancellation or permanent errors
		if ctx.Err() != nil {
			break
		}

		// Add exponential backoff for retries
		if attempt < maxRetries-1 {
			backoff := time.Duration(attempt+1) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	return nil, fmt.Errorf("MCP client '%s' operation '%s' failed after %d attempts: %w", c.name, operation, maxRetries, lastErr)
}

// GetToolInfo returns tool information for this client
func (c *BaseMCPClient) GetToolInfo(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: c.name,
		Desc: c.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"operation": {
				Type:     schema.String,
				Desc:     "The operation to perform",
				Required: false,
			},
		}),
	}, nil
}

// IsHealthy checks if the client is healthy
func (c *BaseMCPClient) IsHealthy(ctx context.Context) bool {
	// Simple health check - try to call a basic operation
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.Call(healthCtx, "health", map[string]interface{}{})
	return err == nil
}

// DatabaseMCPClient implements an MCP client for database operations
type DatabaseMCPClient struct {
	*BaseMCPClient
}

// NewDatabaseMCPClient creates a new database MCP client
func NewDatabaseMCPClient(config MCPToolConfig) *DatabaseMCPClient {
	executor := func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
		switch operation {
		case "query":
			query, ok := parameters["query"].(string)
			if !ok {
				return nil, fmt.Errorf("query parameter is required and must be a string")
			}

			// Simulate database query execution
			// In a real implementation, this would connect to an actual database
			result := map[string]interface{}{
				"query":    query,
				"rows":     simulateQueryResult(query),
				"rowCount": 2,
				"duration": "15ms",
			}
			return result, nil

		case "health":
			return map[string]interface{}{"status": "healthy", "timestamp": time.Now()}, nil

		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}

	baseClient := NewBaseMCPClient(config, executor)
	return &DatabaseMCPClient{BaseMCPClient: baseClient}
}

// APIMCPClient implements an MCP client for API operations
type APIMCPClient struct {
	*BaseMCPClient
}

// NewAPIMCPClient creates a new API MCP client
func NewAPIMCPClient(config MCPToolConfig) *APIMCPClient {
	executor := func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
		switch operation {
		case "request":
			endpoint, ok := parameters["endpoint"].(string)
			if !ok {
				return nil, fmt.Errorf("endpoint parameter is required and must be a string")
			}

			method, ok := parameters["method"].(string)
			if !ok {
				method = "GET"
			}

			// Simulate API request
			// In a real implementation, this would make actual HTTP requests
			result := map[string]interface{}{
				"endpoint":   endpoint,
				"method":     method,
				"statusCode": 200,
				"data":       simulateAPIResponse(endpoint, method),
				"duration":   "250ms",
			}
			return result, nil

		case "health":
			return map[string]interface{}{"status": "healthy", "timestamp": time.Now()}, nil

		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}

	baseClient := NewBaseMCPClient(config, executor)
	return &APIMCPClient{BaseMCPClient: baseClient}
}

// EmailMCPClient implements an MCP client for email operations
type EmailMCPClient struct {
	*BaseMCPClient
}

// NewEmailMCPClient creates a new email MCP client
func NewEmailMCPClient(config MCPToolConfig) *EmailMCPClient {
	executor := func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
		switch operation {
		case "send":
			to, ok := parameters["to"].(string)
			if !ok {
				return nil, fmt.Errorf("to parameter is required and must be a string")
			}

			subject, ok := parameters["subject"].(string)
			if !ok {
				return nil, fmt.Errorf("subject parameter is required and must be a string")
			}

			body, ok := parameters["body"].(string)
			if !ok {
				return nil, fmt.Errorf("body parameter is required and must be a string")
			}

			// Simulate email sending
			result := map[string]interface{}{
				"messageId": fmt.Sprintf("msg_%d", time.Now().Unix()),
				"to":        to,
				"subject":   subject,
				"body":      body,
				"status":    "sent",
				"timestamp": time.Now(),
			}
			return result, nil

		case "health":
			return map[string]interface{}{"status": "healthy", "timestamp": time.Now()}, nil

		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}

	baseClient := NewBaseMCPClient(config, executor)
	return &EmailMCPClient{BaseMCPClient: baseClient}
}

// FileSystemMCPClient implements an MCP client for file system operations
type FileSystemMCPClient struct {
	*BaseMCPClient
}

// NewFileSystemMCPClient creates a new file system MCP client
func NewFileSystemMCPClient(config MCPToolConfig) *FileSystemMCPClient {
	executor := func(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
		switch operation {
		case "read":
			path, ok := parameters["path"].(string)
			if !ok {
				return nil, fmt.Errorf("path parameter is required and must be a string")
			}

			// Simulate file reading
			result := map[string]interface{}{
				"path":    path,
				"content": fmt.Sprintf("Content of file: %s\nThis is simulated content.", path),
				"size":    1024,
				"mtime":   time.Now().Add(-24 * time.Hour),
			}
			return result, nil

		case "write":
			path, ok := parameters["path"].(string)
			if !ok {
				return nil, fmt.Errorf("path parameter is required and must be a string")
			}

			content, ok := parameters["content"].(string)
			if !ok {
				return nil, fmt.Errorf("content parameter is required and must be a string")
			}

			// Simulate file writing
			result := map[string]interface{}{
				"path":      path,
				"bytesWritten": len(content),
				"timestamp": time.Now(),
			}
			return result, nil

		case "health":
			return map[string]interface{}{"status": "healthy", "timestamp": time.Now()}, nil

		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}

	baseClient := NewBaseMCPClient(config, executor)
	return &FileSystemMCPClient{BaseMCPClient: baseClient}
}

// Helper functions for simulation

func simulateQueryResult(query string) []map[string]interface{} {
	// Simulate different query results based on query content
	if strings.Contains(strings.ToLower(query), "users") {
		return []map[string]interface{}{
			{"id": 1, "name": "Alice", "email": "alice@example.com", "created_at": "2023-01-15"},
			{"id": 2, "name": "Bob", "email": "bob@example.com", "created_at": "2023-02-20"},
		}
	}

	if strings.Contains(strings.ToLower(query), "products") {
		return []map[string]interface{}{
			{"id": 101, "name": "Laptop", "price": 999.99, "category": "Electronics"},
			{"id": 102, "name": "Mouse", "price": 29.99, "category": "Electronics"},
		}
	}

	// Default result
	return []map[string]interface{}{
		{"result": "Query executed successfully", "timestamp": time.Now()},
	}
}

func simulateAPIResponse(endpoint, method string) interface{} {
	if strings.Contains(endpoint, "weather") {
		return map[string]interface{}{
			"location":    "Beijing",
			"temperature": 22,
			"humidity":    65,
			"condition":   "Partly Cloudy",
			"forecast": []map[string]interface{}{
				{"date": "2024-01-01", "high": 25, "low": 18, "condition": "Sunny"},
				{"date": "2024-01-02", "high": 23, "low": 16, "condition": "Cloudy"},
			},
		}
	}

	if strings.Contains(endpoint, "users") && method == "GET" {
		return map[string]interface{}{
			"users": []map[string]interface{}{
				{"id": 1, "name": "Alice", "active": true},
				{"id": 2, "name": "Bob", "active": false},
			},
			"total": 2,
		}
	}

	// Default response
	return map[string]interface{}{
		"message": "API request completed",
		"endpoint": endpoint,
		"method":   method,
		"timestamp": time.Now(),
	}
}

// ConvertToEinoTool converts an MCP client to an Eino tool
func ConvertToEinoTool(client MCPClient) tool.InvokableTool {
	return &MCPToolAdapter{
		client: client,
	}
}

// MCPToolAdapter adapts MCPClient to tool.InvokableTool interface
type MCPToolAdapter struct {
	client MCPClient
}

func (t *MCPToolAdapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.client.GetToolInfo(ctx)
}

func (t *MCPToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	operation, ok := params["operation"].(string)
	if !ok {
		operation = "default"
	}

	// Remove operation from params to avoid passing it to the client
	clientParams := make(map[string]interface{})
	for k, v := range params {
		if k != "operation" {
			clientParams[k] = v
		}
	}

	result, err := t.client.Call(ctx, operation, clientParams)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	// Convert result to JSON string
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultBytes), nil
}