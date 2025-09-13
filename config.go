package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// UnifyDialogConfig represents the complete configuration for the unify dialog system
type UnifyDialogConfig struct {
	// Model provider configuration
	ModelProvider string `yaml:"model_provider" json:"model_provider"` // openai, deepseek, claude, mock

	// OpenAI configuration
	OpenAI struct {
		BaseURL string `yaml:"base_url" json:"base_url"`
		APIKey  string `yaml:"api_key" json:"api_key"`
		Model   string `yaml:"model" json:"model"`
	} `yaml:"openai" json:"openai"`

	// DeepSeek configuration
	DeepSeek struct {
		BaseURL string `yaml:"base_url" json:"base_url"`
		APIKey  string `yaml:"api_key" json:"api_key"`
		Model   string `yaml:"model" json:"model"`
	} `yaml:"deepseek" json:"deepseek"`

	// Claude configuration
	Claude struct {
		BaseURL string `yaml:"base_url" json:"base_url"`
		APIKey  string `yaml:"api_key" json:"api_key"`
		Model   string `yaml:"model" json:"model"`
	} `yaml:"claude" json:"claude"`

	// MCP configuration
	MCP struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
		Servers map[string]MCPServerConfig `yaml:"servers" json:"servers"`
	} `yaml:"mcp" json:"mcp"`

	// Model parameters
	Temperature float32 `yaml:"temperature" json:"temperature"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	TopP        float32 `yaml:"top_p" json:"top_p"`

	// System configuration
	System struct {
		Debug           bool   `yaml:"debug" json:"debug"`
		EnableCallbacks bool   `yaml:"enable_callbacks" json:"enable_callbacks"`
		LogLevel        string `yaml:"log_level" json:"log_level"`
	} `yaml:"system" json:"system"`

	// Callback configuration
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

// MCPServerConfig represents MCP server configuration
type MCPServerConfig struct {
	Command string            `yaml:"command" json:"command"`
	Args    []string          `yaml:"args" json:"args"`
	Env     map[string]string `yaml:"env" json:"env"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*UnifyDialogConfig, error) {
	config := &UnifyDialogConfig{
		// Default values
		ModelProvider: "mock",
		Temperature:   0.7,
		MaxTokens:     2000,
		TopP:          0.9,
	}

	// Set defaults for nested structs
	config.System.LogLevel = "info"
	config.MCP.Enabled = false

	// Load from file if exists
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	config.loadFromEnv()

	return config, nil
}

// loadFromEnv loads configuration from environment variables
func (c *UnifyDialogConfig) loadFromEnv() {
	// Model provider
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

// Validate checks if the configuration is valid
func (c *UnifyDialogConfig) Validate() error {
	switch c.ModelProvider {
	case "openai":
		if c.OpenAI.APIKey == "" {
			return fmt.Errorf("OpenAI API key is required when using OpenAI provider")
		}
		if c.OpenAI.Model == "" {
			c.OpenAI.Model = "gpt-4o-mini"
		}
	case "deepseek":
		if c.DeepSeek.APIKey == "" {
			return fmt.Errorf("DeepSeek API key is required when using DeepSeek provider")
		}
		if c.DeepSeek.Model == "" {
			c.DeepSeek.Model = "deepseek-chat"
		}
	case "claude":
		if c.Claude.APIKey == "" {
			return fmt.Errorf("Claude API key is required when using Claude provider")
		}
		if c.Claude.Model == "" {
			c.Claude.Model = "claude-3-sonnet-20240229"
		}
	case "mock":
		// Mock provider doesn't need validation
	default:
		return fmt.Errorf("unsupported model provider: %s", c.ModelProvider)
	}

	return nil
}