# UnifyDialog 项目详细设计方案

## 目录

1. [项目概述](#1-项目概述)
2. [整体架构设计](#2-整体架构设计)
3. [核心类设计与关系](#3-核心类设计与关系)
4. [数据流设计](#4-数据流设计)
5. [设计模式应用](#5-设计模式应用)
6. [扩展性设计](#6-扩展性设计)
7. [性能优化设计](#7-性能优化设计)
8. [部署和运维设计](#8-部署和运维设计)

---

## 1. 项目概述

### 1.1 项目背景

UnifyDialog 是基于 CloudWeGo Eino 框架构建的企业级统一对话系统，旨在提供高可用、高扩展、高性能的多智能体协作对话能力。系统采用现代化的微服务架构理念，通过图编排技术实现复杂的对话流程管理。

### 1.2 核心特性

- **多智能体协作**: 基于规划-执行-监督模式的智能体编排
- **模型提供商无关**: 支持 OpenAI、DeepSeek、Claude 等多种模型提供商
- **MCP工具生态**: 深度集成 Model Context Protocol，支持丰富的外部工具
- **配置驱动**: 灵活的配置管理，支持多环境部署
- **可观测性**: 完整的回调机制，支持监控、日志、调试
- **流式处理**: 原生支持流式响应，提升用户体验
- **状态管理**: 集中式状态管理，支持复杂对话流程

### 1.3 技术栈

- **核心框架**: CloudWeGo Eino v0.x
- **编程语言**: Go 1.21+
- **配置管理**: YAML + 环境变量
- **并发模型**: Goroutine + Context
- **图编排**: Eino Compose Graph
- **工具协议**: MCP (Model Context Protocol)
- **可观测性**: 自定义回调 + 外部集成

### 1.4 设计目标

1. **可维护性**: 清晰的代码组织和依赖关系
2. **可扩展性**: 易于添加新模型、工具和功能
3. **可测试性**: 完善的单元测试和集成测试支持
4. **高性能**: 支持高并发和低延迟场景
5. **生产就绪**: 完善的错误处理和监控机制

---

## 2. 整体架构设计

### 2.1 系统分层架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户接口层 (Interface Layer)               │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────────┐ │
│  │   CLI Interface │ │   HTTP API      │ │   gRPC Service      │ │
│  │                 │ │   (Future)      │ │   (Future)          │ │
│  └─────────────────┘ └─────────────────┘ └─────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                 ↓
┌─────────────────────────────────────────────────────────────────┐
│                      业务逻辑层 (Business Layer)                  │
│  ┌─────────────────────────────┐ ┌─────────────────────────────┐ │
│  │  MultiAgentOrchestrator     │ │ ImprovedUnifyDialogSystem   │ │
│  │  ┌─────────────────────────┐│ │ ┌─────────────────────────┐ │ │
│  │  │    StateManager         ││ │ │    Graph Builder        │ │ │
│  │  │    ┌───────────────────┐││ │ │    ┌─────────────────┐ │ │ │
│  │  │    │ SystemState       │││ │ │    │ Node Manager    │ │ │ │
│  │  │    │ - Plan            │││ │ │    │ Edge Manager    │ │ │ │
│  │  │    │ - ExecutionLog    │││ │ │    │ Compiler        │ │ │ │
│  │  │    │ - Context         │││ │ │    └─────────────────┘ │ │ │
│  │  │    └───────────────────┘││ │ └─────────────────────────┘ │ │
│  │  └─────────────────────────┘│ └─────────────────────────────┘ │
│  └─────────────────────────────┘                                 │
└─────────────────────────────────────────────────────────────────┘
                                 ↓
┌─────────────────────────────────────────────────────────────────┐
│                     组件服务层 (Component Layer)                  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ │
│  │ModelFactory │ │ToolRegistry │ │CallbackMgr  │ │ConfigMgr    │ │
│  │             │ │             │ │             │ │             │ │
│  │┌───────────┐│ │┌───────────┐│ │┌───────────┐│ │┌───────────┐│ │
│  ││OpenAI     ││ ││MCPTools   ││ ││Debug      ││ ││YAML       ││ │
│  ││DeepSeek   ││ ││Custom     ││ ││Logging    ││ ││Env Vars   ││ │
│  ││Claude     ││ ││Registry   ││ ││Performance││ ││Validation ││ │
│  ││Mock       ││ ││Lifecycle  ││ ││Streaming  ││ ││Defaults   ││ │
│  │└───────────┘│ │└───────────┘│ │└───────────┘│ │└───────────┘│ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                 ↓
┌─────────────────────────────────────────────────────────────────┐
│                    基础设施层 (Infrastructure Layer)              │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────┐ │
│  │ Eino Framework│ │MCP Protocol  │ │External APIs │ │Monitoring│ │
│  │              │ │              │ │              │ │          │ │
│  │┌────────────┐│ │┌────────────┐│ │┌────────────┐│ │┌────────┐│ │
│  ││Graph       ││ ││Stdio       ││ ││OpenAI      ││ ││Metrics ││ │
│  ││Compose     ││ ││Transport   ││ ││DeepSeek    ││ ││Logging ││ │
│  ││Streaming   ││ ││Tool Schema ││ ││Claude      ││ ││Tracing ││ │
│  ││Callbacks   ││ ││Discovery   ││ ││Anthropic   ││ ││Health  ││ │
│  │└────────────┘│ │└────────────┘│ │└────────────┘│ │└────────┘│ │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 核心架构原则

#### 2.2.1 依赖倒置原则 (Dependency Inversion Principle)

系统采用依赖注入模式，高层模块不依赖低层模块的具体实现：

```go
// 高层模块依赖抽象接口
type MultiAgentOrchestrator struct {
    modelFactory    ModelFactoryInterface    // 依赖抽象
    toolRegistry    ToolRegistryInterface    // 依赖抽象
    callbackManager CallbackManagerInterface // 依赖抽象
}

// 通过工厂注入具体实现
func NewMultiAgentOrchestrator(config *UnifyDialogConfig) *MultiAgentOrchestrator {
    return &MultiAgentOrchestrator{
        modelFactory:    NewModelFactory(config),         // 注入具体实现
        toolRegistry:    NewToolRegistry(mcpManager),     // 注入具体实现
        callbackManager: NewCallbackManager(config),      // 注入具体实现
    }
}
```

#### 2.2.2 单一职责原则 (Single Responsibility Principle)

每个组件负责单一职责领域：

- **ModelFactory**: 只负责模型实例创建和管理
- **ToolRegistry**: 只负责工具注册和发现
- **CallbackManager**: 只负责回调处理器管理
- **StateManager**: 只负责系统状态管理

#### 2.2.3 开闭原则 (Open/Closed Principle)

系统对扩展开放，对修改封闭：

```go
// 新增模型提供商无需修改现有代码
func (f *ModelFactory) CreateChatModel(ctx context.Context, role string) (model.ChatModel, error) {
    switch f.config.ModelProvider {
    case "openai":
        return f.createOpenAIModel(ctx, role)
    case "deepseek":
        return f.createDeepSeekModel(ctx, role)
    case "new_provider": // 新增提供商，无需修改现有case
        return f.createNewProviderModel(ctx, role)
    }
}
```

### 2.3 模块间通信设计

#### 2.3.1 同步通信

主要用于配置获取、模型创建等确定性操作：

```go
// 同步模型创建
model, err := factory.CreatePlannerModel(ctx)
if err != nil {
    return fmt.Errorf("failed to create planner model: %w", err)
}
```

#### 2.3.2 异步通信

用于工具执行、回调处理等可并行操作：

```go
// 异步工具执行
go func() {
    result, err := tool.Run(ctx, params)
    resultChan <- ToolResult{Result: result, Error: err}
}()
```

#### 2.3.3 事件驱动通信

通过回调机制实现组件间的松耦合通信：

```go
// 事件发布
for _, handler := range cm.handlers {
    handler.OnStart(ctx, runInfo, input)
}
```

---

## 3. 核心类设计与关系

### 3.1 配置管理子系统

#### 3.1.1 UnifyDialogConfig 类设计

```go
type UnifyDialogConfig struct {
    // 模型提供商配置
    ModelProvider string `yaml:"model_provider" json:"model_provider"`

    // 各模型提供商的详细配置
    OpenAI   OpenAIConfig   `yaml:"openai" json:"openai"`
    DeepSeek DeepSeekConfig `yaml:"deepseek" json:"deepseek"`
    Claude   ClaudeConfig   `yaml:"claude" json:"claude"`

    // MCP工具配置
    MCP MCPConfig `yaml:"mcp" json:"mcp"`

    // 模型参数配置
    Temperature float32 `yaml:"temperature" json:"temperature"`
    MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
    TopP        float32 `yaml:"top_p" json:"top_p"`

    // 系统运行配置
    System SystemConfig `yaml:"system" json:"system"`

    // 回调处理器配置
    Callbacks CallbacksConfig `yaml:"callbacks" json:"callbacks"`
}
```

**核心方法实现**:

```go
// LoadConfig 支持多数据源配置加载
func LoadConfig(configPath string) (*UnifyDialogConfig, error) {
    config := &UnifyDialogConfig{
        // 设置默认值
        ModelProvider: "mock",
        Temperature:   0.7,
        MaxTokens:     2000,
        TopP:          0.9,
    }

    // 1. 从文件加载
    if configPath != "" {
        data, err := os.ReadFile(configPath)
        if err != nil {
            return nil, fmt.Errorf("failed to read config file: %w", err)
        }

        if err := yaml.Unmarshal(data, config); err != nil {
            return nil, fmt.Errorf("failed to parse config file: %w", err)
        }
    }

    // 2. 环境变量覆盖
    config.loadFromEnv()

    // 3. 配置验证
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    return config, nil
}

// Validate 配置验证逻辑
func (c *UnifyDialogConfig) Validate() error {
    // 验证模型提供商配置
    switch c.ModelProvider {
    case "openai":
        if c.OpenAI.APIKey == "" {
            return fmt.Errorf("OpenAI API key is required")
        }
        if c.OpenAI.Model == "" {
            c.OpenAI.Model = "gpt-4o-mini" // 设置默认模型
        }
    case "deepseek":
        if c.DeepSeek.APIKey == "" {
            return fmt.Errorf("DeepSeek API key is required")
        }
        if c.DeepSeek.Model == "" {
            c.DeepSeek.Model = "deepseek-chat"
        }
    case "mock":
        // Mock模式不需要验证
    default:
        return fmt.Errorf("unsupported model provider: %s", c.ModelProvider)
    }

    // 验证参数范围
    if c.Temperature < 0 || c.Temperature > 2 {
        return fmt.Errorf("temperature must be between 0 and 2")
    }

    if c.MaxTokens < 1 || c.MaxTokens > 32000 {
        return fmt.Errorf("max_tokens must be between 1 and 32000")
    }

    return nil
}
```

**配置子类型设计**:

```go
type OpenAIConfig struct {
    BaseURL string `yaml:"base_url" json:"base_url"`
    APIKey  string `yaml:"api_key" json:"api_key"`
    Model   string `yaml:"model" json:"model"`
    OrgID   string `yaml:"org_id" json:"org_id"`           // 组织ID
    Timeout int    `yaml:"timeout" json:"timeout"`         // 超时时间(秒)
}

type MCPConfig struct {
    Enabled bool                            `yaml:"enabled" json:"enabled"`
    Servers map[string]MCPServerConfig      `yaml:"servers" json:"servers"`
    Timeout int                             `yaml:"timeout" json:"timeout"`
}

type MCPServerConfig struct {
    Command string            `yaml:"command" json:"command"`
    Args    []string          `yaml:"args" json:"args"`
    Env     map[string]string `yaml:"env" json:"env"`
    WorkDir string            `yaml:"work_dir" json:"work_dir"`
}

type SystemConfig struct {
    Debug           bool   `yaml:"debug" json:"debug"`
    EnableCallbacks bool   `yaml:"enable_callbacks" json:"enable_callbacks"`
    LogLevel        string `yaml:"log_level" json:"log_level"`
    MaxConcurrency  int    `yaml:"max_concurrency" json:"max_concurrency"`
}
```

### 3.2 模型工厂子系统

#### 3.2.1 ModelFactory 类设计

```go
type ModelFactory struct {
    config      *UnifyDialogConfig
    modelCache  map[string]model.ChatModel  // 模型实例缓存
    mutex       sync.RWMutex               // 并发安全
}

// NewModelFactory 工厂构造函数
func NewModelFactory(config *UnifyDialogConfig) *ModelFactory {
    return &ModelFactory{
        config:     config,
        modelCache: make(map[string]model.ChatModel),
    }
}
```

**核心方法实现**:

```go
// CreateChatModel 创建聊天模型实例
func (f *ModelFactory) CreateChatModel(ctx context.Context, role string) (model.ChatModel, error) {
    // 生成缓存键
    cacheKey := fmt.Sprintf("%s_%s", f.config.ModelProvider, role)

    // 尝试从缓存获取
    f.mutex.RLock()
    if cached, exists := f.modelCache[cacheKey]; exists {
        f.mutex.RUnlock()
        return cached, nil
    }
    f.mutex.RUnlock()

    // 创建新实例
    var chatModel model.ChatModel
    var err error

    switch f.config.ModelProvider {
    case "openai":
        chatModel, err = f.createOpenAIModel(ctx, role)
    case "deepseek":
        chatModel, err = f.createDeepSeekModel(ctx, role)
    case "claude":
        chatModel, err = f.createClaudeModel(ctx, role)
    case "mock":
        chatModel = &MockChatModel{role: role, config: f.config}
    default:
        return nil, fmt.Errorf("unsupported model provider: %s", f.config.ModelProvider)
    }

    if err != nil {
        return nil, fmt.Errorf("failed to create %s model for role %s: %w",
            f.config.ModelProvider, role, err)
    }

    // 缓存实例
    f.mutex.Lock()
    f.modelCache[cacheKey] = chatModel
    f.mutex.Unlock()

    return chatModel, nil
}

// createOpenAIModel 创建OpenAI模型
func (f *ModelFactory) createOpenAIModel(ctx context.Context, role string) (model.ChatModel, error) {
    // 注意: 这里显示的是预期的实现，需要eino-ext依赖
    /*
    import "github.com/cloudwego/eino-ext/components/model/openai"

    config := &openai.ChatModelConfig{
        BaseURL:     f.config.OpenAI.BaseURL,
        APIKey:      f.config.OpenAI.APIKey,
        Model:       f.config.OpenAI.Model,
        Temperature: &f.config.Temperature,
        MaxTokens:   &f.config.MaxTokens,
        TopP:        &f.config.TopP,
    }

    // 根据角色调整模型参数
    switch role {
    case "planner":
        temp := float32(0.3) // 规划需要更确定性的输出
        config.Temperature = &temp
    case "executor":
        temp := float32(0.1) // 执行需要精确性
        config.Temperature = &temp
    case "supervisor":
        temp := float32(0.5) // 监督需要平衡
        config.Temperature = &temp
    }

    return openai.NewChatModel(ctx, config)
    */

    // 当前返回Mock实现
    return &MockChatModel{role: role, config: f.config}, nil
}
```

**角色化模型管理**:

```go
// CreatePlannerModel 创建规划智能体模型
func (f *ModelFactory) CreatePlannerModel(ctx context.Context) (model.ChatModel, error) {
    return f.CreateChatModel(ctx, "planner")
}

// CreateExecutorModel 创建执行智能体模型
func (f *ModelFactory) CreateExecutorModel(ctx context.Context) (model.ChatModel, error) {
    return f.CreateChatModel(ctx, "executor")
}

// CreateSupervisorModel 创建监督智能体模型
func (f *ModelFactory) CreateSupervisorModel(ctx context.Context) (model.ChatModel, error) {
    return f.CreateChatModel(ctx, "supervisor")
}

// InvalidateCache 缓存失效
func (f *ModelFactory) InvalidateCache() {
    f.mutex.Lock()
    defer f.mutex.Unlock()
    f.modelCache = make(map[string]model.ChatModel)
}

// GetCacheStats 获取缓存统计
func (f *ModelFactory) GetCacheStats() map[string]interface{} {
    f.mutex.RLock()
    defer f.mutex.RUnlock()

    return map[string]interface{}{
        "cache_size": len(f.modelCache),
        "provider":   f.config.ModelProvider,
    }
}
```

#### 3.2.2 MockChatModel 增强实现

```go
type MockChatModel struct {
    role     string
    config   *UnifyDialogConfig
    callCount int
    mutex     sync.Mutex
}

func (m *MockChatModel) Generate(ctx context.Context, messages []*schema.Message, options ...model.GenerateOption) (*schema.Message, error) {
    m.mutex.Lock()
    m.callCount++
    callNum := m.callCount
    m.mutex.Unlock()

    // 模拟处理延迟
    if m.config.System.Debug {
        processingTime := time.Duration(100+rand.Intn(200)) * time.Millisecond
        time.Sleep(processingTime)
    }

    switch m.role {
    case "planner":
        return m.generatePlannerResponse(messages, callNum)
    case "executor":
        return m.generateExecutorResponse(messages, callNum)
    case "supervisor":
        return m.generateSupervisorResponse(messages, callNum)
    default:
        return schema.AssistantMessage(fmt.Sprintf("Mock response from %s (call #%d)", m.role, callNum)), nil
    }
}

func (m *MockChatModel) generatePlannerResponse(messages []*schema.Message, callNum int) (*schema.Message, error) {
    // 根据输入生成更真实的规划响应
    userQuery := m.extractUserQuery(messages)

    plan := map[string]interface{}{
        "goal": "Address the user query: " + userQuery,
        "steps": []map[string]interface{}{
            {
                "id":          "step_1",
                "description": "Analyze the user query and identify key requirements",
                "tool":        "text_analysis",
                "parameters":  map[string]interface{}{"text": userQuery},
                "dependencies": []string{},
            },
            {
                "id":          "step_2",
                "description": "Search for relevant information if needed",
                "tool":        "web_search",
                "parameters":  map[string]interface{}{"query": userQuery},
                "dependencies": []string{"step_1"},
            },
            {
                "id":          "step_3",
                "description": "Generate comprehensive response based on analysis",
                "tool":        "response_generator",
                "parameters":  map[string]interface{}{"input": "analysis_and_search_results"},
                "dependencies": []string{"step_1", "step_2"},
            },
        },
        "metadata": map[string]interface{}{
            "call_number": callNum,
            "timestamp":   time.Now().Unix(),
            "model_role":  m.role,
        },
    }

    planJSON, _ := json.MarshalIndent(plan, "", "  ")
    return schema.AssistantMessage(string(planJSON)), nil
}

func (m *MockChatModel) extractUserQuery(messages []*schema.Message) string {
    for _, msg := range messages {
        if msg.Role == schema.User {
            return msg.Content
        }
    }
    return "general query"
}

// Stream 实现流式接口
func (m *MockChatModel) Stream(ctx context.Context, messages []*schema.Message, options ...model.GenerateOption) (*schema.StreamReader[*schema.Message], error) {
    // 模拟流式输出
    response, err := m.Generate(ctx, messages, options...)
    if err != nil {
        return nil, err
    }

    // 将完整响应分块流式返回
    return m.createMockStream(response.Content), nil
}

func (m *MockChatModel) createMockStream(content string) *schema.StreamReader[*schema.Message] {
    chunks := m.splitIntoChunks(content, 50) // 每50字符一个块

    reader := &schema.StreamReader[*schema.Message]{}

    // 在goroutine中逐块发送
    go func() {
        defer reader.Close()

        for i, chunk := range chunks {
            select {
            case <-reader.Done():
                return
            default:
                msg := schema.AssistantMessage(chunk)
                msg.Extra = map[string]interface{}{
                    "chunk_index": i,
                    "is_final":    i == len(chunks)-1,
                }

                reader.Send(msg)

                // 模拟流式延迟
                if m.config.System.Debug {
                    time.Sleep(50 * time.Millisecond)
                }
            }
        }
    }()

    return reader
}
```