package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// UnifyDialogConfig 表示统一对话系统的完整配置
type UnifyDialogConfig struct {
	// 模型提供商配置
	ModelProvider string `yaml:"model_provider" json:"model_provider"` // openai, deepseek, claude, mock

	// OpenAI配置
	OpenAI struct {
		BaseURL string `yaml:"base_url" json:"base_url"`
		APIKey  string `yaml:"api_key" json:"api_key"`
		Model   string `yaml:"model" json:"model"`
	} `yaml:"openai" json:"openai"`

	// DeepSeek配置
	DeepSeek struct {
		BaseURL string `yaml:"base_url" json:"base_url"`
		APIKey  string `yaml:"api_key" json:"api_key"`
		Model   string `yaml:"model" json:"model"`
	} `yaml:"deepseek" json:"deepseek"`

	// Claude配置
	Claude struct {
		BaseURL string `yaml:"base_url" json:"base_url"`
		APIKey  string `yaml:"api_key" json:"api_key"`
		Model   string `yaml:"model" json:"model"`
	} `yaml:"claude" json:"claude"`

	// MCP配置
	MCP struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
		Servers map[string]MCPServerConfig `yaml:"servers" json:"servers"`
	} `yaml:"mcp" json:"mcp"`

	// 模型参数
	Temperature float32 `yaml:"temperature" json:"temperature"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	TopP        float32 `yaml:"top_p" json:"top_p"`

	// 系统配置
	System struct {
		Debug           bool   `yaml:"debug" json:"debug"`
		EnableCallbacks bool   `yaml:"enable_callbacks" json:"enable_callbacks"`
		LogLevel        string `yaml:"log_level" json:"log_level"`
	} `yaml:"system" json:"system"`

	// 回调配置
	Callbacks struct {
		Cozeloop struct {
			Enabled     bool   `yaml:"enabled" json:"enabled"`
			APIToken    string `yaml:"api_token" json:"api_token"`
			WorkspaceID string `yaml:"workspace_id" json:"workspace_id"`
		} `yaml:"cozeloop" json:"cozeloop"`

		LangSmith struct {
			Enabled  bool   `yaml:"enabled" json:"enabled"`
			APIKey   string `yaml:"api_key" json:"api_key"`
			Endpoint string `yaml:"endpoint" json:"endpoint"`
		} `yaml:"langsmith" json:"langsmith"`
	} `yaml:"callbacks" json:"callbacks"`
}

// MCPServerConfig 表示MCP服务器配置
type MCPServerConfig struct {
	Command string            `yaml:"command" json:"command"`
	Args    []string          `yaml:"args" json:"args"`
	Env     map[string]string `yaml:"env" json:"env"`
}

// LoadConfig 从YAML文件加载配置
func LoadConfig(configPath string) (*UnifyDialogConfig, error) {
	config := &UnifyDialogConfig{
		// 默认值
		ModelProvider: "mock",
		Temperature:   0.7,
		MaxTokens:     2000,
		TopP:          0.9,
	}

	// 设置嵌套结构的默认值
	config.System.LogLevel = "info"
	config.MCP.Enabled = false

	// 如果文件存在则从文件加载
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// 环境变量覆盖
	config.loadFromEnv()

	return config, nil
}

// loadFromEnv 从环境变量加载配置
func (c *UnifyDialogConfig) loadFromEnv() {
	// 模型提供商
	if provider := os.Getenv("MODEL_PROVIDER"); provider != "" {
		c.ModelProvider = provider
	}

	// OpenAI
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		c.OpenAI.APIKey = apiKey
	}
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		c.OpenAI.BaseURL = baseURL
	}
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		c.OpenAI.Model = model
	}

	// DeepSeek
	if apiKey := os.Getenv("DEEPSEEK_API_KEY"); apiKey != "" {
		c.DeepSeek.APIKey = apiKey
	}
	if baseURL := os.Getenv("DEEPSEEK_BASE_URL"); baseURL != "" {
		c.DeepSeek.BaseURL = baseURL
	}
	if model := os.Getenv("DEEPSEEK_MODEL"); model != "" {
		c.DeepSeek.Model = model
	}

	// Claude
	if apiKey := os.Getenv("CLAUDE_API_KEY"); apiKey != "" {
		c.Claude.APIKey = apiKey
	}
	if baseURL := os.Getenv("CLAUDE_BASE_URL"); baseURL != "" {
		c.Claude.BaseURL = baseURL
	}
	if model := os.Getenv("CLAUDE_MODEL"); model != "" {
		c.Claude.Model = model
	}

	// Cozeloop
	if token := os.Getenv("COZELOOP_API_TOKEN"); token != "" {
		c.Callbacks.Cozeloop.APIToken = token
		c.Callbacks.Cozeloop.Enabled = true
	}
	if workspaceID := os.Getenv("COZELOOP_WORKSPACE_ID"); workspaceID != "" {
		c.Callbacks.Cozeloop.WorkspaceID = workspaceID
	}

	// LangSmith
	if apiKey := os.Getenv("LANGSMITH_API_KEY"); apiKey != "" {
		c.Callbacks.LangSmith.APIKey = apiKey
		c.Callbacks.LangSmith.Enabled = true
	}
	if endpoint := os.Getenv("LANGSMITH_ENDPOINT"); endpoint != "" {
		c.Callbacks.LangSmith.Endpoint = endpoint
	}
}

// Validate 检查配置是否有效
func (c *UnifyDialogConfig) Validate() error {
	switch c.ModelProvider {
	case "openai":
		if c.OpenAI.APIKey == "" {
			return fmt.Errorf("使用OpenAI提供商时需要API密钥")
		}
		if c.OpenAI.Model == "" {
			c.OpenAI.Model = "gpt-4o-mini"
		}
	case "deepseek":
		if c.DeepSeek.APIKey == "" {
			return fmt.Errorf("使用DeepSeek提供商时需要API密钥")
		}
		if c.DeepSeek.Model == "" {
			c.DeepSeek.Model = "deepseek-chat"
		}
	case "claude":
		if c.Claude.APIKey == "" {
			return fmt.Errorf("使用Claude提供商时需要API密钥")
		}
		if c.Claude.Model == "" {
			c.Claude.Model = "claude-3-sonnet-20240229"
		}
	case "mock":
		// Mock提供商不需要验证
	default:
		return fmt.Errorf("不支持的模型提供商: %s", c.ModelProvider)
	}

	return nil
}