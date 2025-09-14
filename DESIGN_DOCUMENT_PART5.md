# UnifyDialog 设计方案 - 第五部分

## 5. 设计模式应用

### 5.1 创建型模式

#### 5.1.1 工厂方法模式 (Factory Method)

```go
// AbstractModelFactory 抽象模型工厂
type AbstractModelFactory interface {
    CreateChatModel(ctx context.Context, config ModelConfig) (model.ChatModel, error)
    CreateEmbeddingModel(ctx context.Context, config ModelConfig) (embedding.EmbeddingModel, error)
    SupportedModels() []string
    ValidateConfig(config ModelConfig) error
}

// OpenAIModelFactory OpenAI模型工厂实现
type OpenAIModelFactory struct {
    client *openai.Client
    config *OpenAIFactoryConfig
}

func (f *OpenAIModelFactory) CreateChatModel(ctx context.Context, config ModelConfig) (model.ChatModel, error) {
    openAIConfig := &openai.ChatModelConfig{
        BaseURL:     config.BaseURL,
        APIKey:      config.APIKey,
        Model:       config.Model,
        Temperature: config.Temperature,
        MaxTokens:   config.MaxTokens,
    }

    // 模型特定优化
    switch config.Model {
    case "gpt-4":
        openAIConfig.Timeout = 60 * time.Second
    case "gpt-3.5-turbo":
        openAIConfig.Timeout = 30 * time.Second
    }

    return openai.NewChatModel(ctx, openAIConfig)
}

// FactoryRegistry 工厂注册表
type FactoryRegistry struct {
    factories map[string]AbstractModelFactory
    mutex     sync.RWMutex
}

func (fr *FactoryRegistry) RegisterFactory(provider string, factory AbstractModelFactory) {
    fr.mutex.Lock()
    defer fr.mutex.Unlock()
    fr.factories[provider] = factory
}

func (fr *FactoryRegistry) GetFactory(provider string) (AbstractModelFactory, error) {
    fr.mutex.RLock()
    defer fr.mutex.RUnlock()

    factory, exists := fr.factories[provider]
    if !exists {
        return nil, fmt.Errorf("unknown model provider: %s", provider)
    }
    return factory, nil
}
```

#### 5.1.2 建造者模式 (Builder Pattern)

```go
// GraphBuilder 图构建器
type GraphBuilder struct {
    graph     *compose.Graph[*AgentInput, *AgentOutput]
    nodes     []*NodeBuilder
    edges     []*EdgeBuilder
    options   []compose.GraphCompileOption
    validator GraphValidator
}

type NodeBuilder struct {
    name        string
    nodeType    NodeType
    config      map[string]interface{}
    dependencies []string
    conditions  []NodeCondition
    middleware  []NodeMiddleware
}

type EdgeBuilder struct {
    from       string
    to         string
    weight     float64
    condition  EdgeCondition
    metadata   map[string]interface{}
}

// GraphBuilder建造者方法链
func NewGraphBuilder() *GraphBuilder {
    return &GraphBuilder{
        graph:     compose.NewGraph[*AgentInput, *AgentOutput](),
        nodes:     make([]*NodeBuilder, 0),
        edges:     make([]*EdgeBuilder, 0),
        options:   make([]compose.GraphCompileOption, 0),
        validator: NewDefaultGraphValidator(),
    }
}

func (gb *GraphBuilder) AddPlannerNode(name string, agent *Agent) *GraphBuilder {
    nodeBuilder := &NodeBuilder{
        name:     name,
        nodeType: NodeTypeChatModel,
        config: map[string]interface{}{
            "agent": agent,
            "role":  "planner",
        },
        dependencies: []string{},
        conditions:   []NodeCondition{},
        middleware:   []NodeMiddleware{},
    }

    gb.nodes = append(gb.nodes, nodeBuilder)
    return gb
}

func (gb *GraphBuilder) AddExecutorNode(name string, agent *Agent, tools []tool.BaseTool) *GraphBuilder {
    nodeBuilder := &NodeBuilder{
        name:     name,
        nodeType: NodeTypeChatModel,
        config: map[string]interface{}{
            "agent": agent,
            "role":  "executor",
            "tools": tools,
        },
        dependencies: []string{},
        conditions:   []NodeCondition{},
        middleware:   []NodeMiddleware{},
    }

    gb.nodes = append(gb.nodes, nodeBuilder)
    return gb
}

func (gb *GraphBuilder) Connect(from, to string) *GraphBuilder {
    edge := &EdgeBuilder{
        from:      from,
        to:        to,
        weight:    1.0,
        condition: nil,
        metadata:  make(map[string]interface{}),
    }

    gb.edges = append(gb.edges, edge)
    return gb
}

func (gb *GraphBuilder) AddConditionalBranch(from string, condition BranchCondition) *GraphBuilder {
    nodeBuilder := &NodeBuilder{
        name:     fmt.Sprintf("branch_%s", from),
        nodeType: NodeTypeBranch,
        config: map[string]interface{}{
            "condition": condition,
        },
    }

    gb.nodes = append(gb.nodes, nodeBuilder)
    return gb
}

func (gb *GraphBuilder) WithStateManagement(stateHandler compose.StateHandler) *GraphBuilder {
    gb.options = append(gb.options, compose.WithStatePreHandler(stateHandler))
    return gb
}

func (gb *GraphBuilder) WithCallbacks(callbacks []callbacks.Handler) *GraphBuilder {
    for _, callback := range callbacks {
        gb.options = append(gb.options, compose.WithGraphCallbacks(callback))
    }
    return gb
}

func (gb *GraphBuilder) WithTimeout(timeout time.Duration) *GraphBuilder {
    gb.options = append(gb.options, compose.WithTimeout(timeout))
    return gb
}

// Build 构建最终的图
func (gb *GraphBuilder) Build(ctx context.Context) (compose.Runnable[*AgentInput, *AgentOutput], error) {
    // 验证图结构
    if err := gb.validator.ValidateStructure(gb.nodes, gb.edges); err != nil {
        return nil, fmt.Errorf("invalid graph structure: %w", err)
    }

    // 添加节点
    for _, nodeBuilder := range gb.nodes {
        if err := gb.addNodeToGraph(nodeBuilder); err != nil {
            return nil, fmt.Errorf("failed to add node %s: %w", nodeBuilder.name, err)
        }
    }

    // 添加边
    for _, edgeBuilder := range gb.edges {
        if err := gb.addEdgeToGraph(edgeBuilder); err != nil {
            return nil, fmt.Errorf("failed to add edge %s->%s: %w", edgeBuilder.from, edgeBuilder.to, err)
        }
    }

    // 编译图
    compiledGraph, err := gb.graph.Compile(ctx, gb.options...)
    if err != nil {
        return nil, fmt.Errorf("failed to compile graph: %w", err)
    }

    return compiledGraph, nil
}

func (gb *GraphBuilder) addNodeToGraph(nodeBuilder *NodeBuilder) error {
    switch nodeBuilder.nodeType {
    case NodeTypeChatModel:
        agent, ok := nodeBuilder.config["agent"].(*Agent)
        if !ok {
            return fmt.Errorf("invalid agent configuration for node %s", nodeBuilder.name)
        }
        return gb.graph.AddChatModelNode(nodeBuilder.name, agent.Model)

    case NodeTypeLambda:
        handler, ok := nodeBuilder.config["handler"]
        if !ok {
            return fmt.Errorf("missing handler for lambda node %s", nodeBuilder.name)
        }
        return gb.graph.AddLambdaNode(nodeBuilder.name, handler)

    case NodeTypeBranch:
        condition, ok := nodeBuilder.config["condition"].(BranchCondition)
        if !ok {
            return fmt.Errorf("invalid condition for branch node %s", nodeBuilder.name)
        }
        return gb.graph.AddBranch(nodeBuilder.name, condition)

    default:
        return fmt.Errorf("unsupported node type: %s", nodeBuilder.nodeType)
    }
}
```

### 5.2 结构型模式

#### 5.2.1 适配器模式 (Adapter Pattern)

```go
// ModelAdapter 模型适配器接口
type ModelAdapter interface {
    Adapt(ctx context.Context, request *AdaptedRequest) (*AdaptedResponse, error)
    SupportsModel(modelType string) bool
    GetCapabilities() []string
}

// AdaptedRequest 适配后的请求
type AdaptedRequest struct {
    Messages    []*schema.Message      `json:"messages"`
    Parameters  map[string]interface{} `json:"parameters"`
    Tools       []tool.BaseTool        `json:"tools"`
    ModelType   string                 `json:"model_type"`
    RequestID   string                 `json:"request_id"`
}

// AdaptedResponse 适配后的响应
type AdaptedResponse struct {
    Message      *schema.Message        `json:"message"`
    Usage        *TokenUsage            `json:"usage"`
    Metadata     map[string]interface{} `json:"metadata"`
    ResponseTime time.Duration          `json:"response_time"`
}

// OpenAIAdapter OpenAI模型适配器
type OpenAIAdapter struct {
    client       *openai.Client
    rateLimiter  *rate.Limiter
    circuitBreaker *CircuitBreaker
    transformer  *MessageTransformer
}

func (adapter *OpenAIAdapter) Adapt(ctx context.Context, request *AdaptedRequest) (*AdaptedResponse, error) {
    startTime := time.Now()

    // 速率限制
    if err := adapter.rateLimiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit exceeded: %w", err)
    }

    // 断路器保护
    result, err := adapter.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
        return adapter.callOpenAI(ctx, request)
    })

    if err != nil {
        return nil, err
    }

    response := result.(*AdaptedResponse)
    response.ResponseTime = time.Since(startTime)

    return response, nil
}

// DeepSeekAdapter DeepSeek模型适配器
type DeepSeekAdapter struct {
    client      *deepseek.Client
    rateLimiter *rate.Limiter
    transformer *MessageTransformer
}

func (adapter *DeepSeekAdapter) Adapt(ctx context.Context, request *AdaptedRequest) (*AdaptedResponse, error) {
    // DeepSeek特定的处理逻辑
    deepSeekRequest := adapter.transformer.TransformToDeepSeek(request)

    result, err := adapter.client.ChatCompletion(ctx, deepSeekRequest)
    if err != nil {
        return nil, fmt.Errorf("DeepSeek API call failed: %w", err)
    }

    return adapter.transformer.TransformFromDeepSeek(result), nil
}

// AdapterRegistry 适配器注册表
type AdapterRegistry struct {
    adapters map[string]ModelAdapter
    fallback ModelAdapter
    mutex    sync.RWMutex
}

func (registry *AdapterRegistry) GetAdapter(modelType string) (ModelAdapter, error) {
    registry.mutex.RLock()
    defer registry.mutex.RUnlock()

    if adapter, exists := registry.adapters[modelType]; exists {
        return adapter, nil
    }

    if registry.fallback != nil {
        return registry.fallback, nil
    }

    return nil, fmt.Errorf("no adapter found for model type: %s", modelType)
}
```

#### 5.2.2 装饰器模式 (Decorator Pattern)

```go
// ModelDecorator 模型装饰器基础接口
type ModelDecorator interface {
    model.ChatModel
    SetNext(model.ChatModel)
}

// BaseModelDecorator 基础装饰器
type BaseModelDecorator struct {
    next model.ChatModel
}

func (d *BaseModelDecorator) SetNext(next model.ChatModel) {
    d.next = next
}

// CachingDecorator 缓存装饰器
type CachingDecorator struct {
    BaseModelDecorator
    cache     Cache
    keyGen    CacheKeyGenerator
    ttl       time.Duration
    hitCount  int64
    missCount int64
}

func (d *CachingDecorator) Generate(ctx context.Context, messages []*schema.Message, options ...model.GenerateOption) (*schema.Message, error) {
    // 生成缓存键
    cacheKey := d.keyGen.GenerateKey(messages, options)

    // 尝试从缓存获取
    if cached, found := d.cache.Get(cacheKey); found {
        atomic.AddInt64(&d.hitCount, 1)
        return cached.(*schema.Message), nil
    }

    atomic.AddInt64(&d.missCount, 1)

    // 缓存未命中，调用下一个处理器
    response, err := d.next.Generate(ctx, messages, options...)
    if err != nil {
        return nil, err
    }

    // 缓存结果
    d.cache.Set(cacheKey, response, d.ttl)

    return response, nil
}

// LoggingDecorator 日志装饰器
type LoggingDecorator struct {
    BaseModelDecorator
    logger    Logger
    level     LogLevel
    includeInput  bool
    includeOutput bool
}

func (d *LoggingDecorator) Generate(ctx context.Context, messages []*schema.Message, options ...model.GenerateOption) (*schema.Message, error) {
    requestID := extractRequestID(ctx)
    startTime := time.Now()

    if d.includeInput && d.level <= LogLevelDebug {
        d.logger.Debug("Model request", map[string]interface{}{
            "request_id": requestID,
            "messages":   messages,
            "options":    options,
        })
    }

    response, err := d.next.Generate(ctx, messages, options...)
    duration := time.Since(startTime)

    if err != nil {
        d.logger.Error("Model request failed", map[string]interface{}{
            "request_id": requestID,
            "error":      err.Error(),
            "duration":   duration,
        })
        return nil, err
    }

    d.logger.Info("Model request completed", map[string]interface{}{
        "request_id": requestID,
        "duration":   duration,
        "tokens":     extractTokenCount(response),
    })

    if d.includeOutput && d.level <= LogLevelDebug {
        d.logger.Debug("Model response", map[string]interface{}{
            "request_id": requestID,
            "response":   response,
        })
    }

    return response, nil
}

// RateLimitingDecorator 速率限制装饰器
type RateLimitingDecorator struct {
    BaseModelDecorator
    limiter      *rate.Limiter
    waitTimeout  time.Duration
    rejectedCount int64
}

func (d *RateLimitingDecorator) Generate(ctx context.Context, messages []*schema.Message, options ...model.GenerateOption) (*schema.Message, error) {
    // 创建带超时的上下文
    waitCtx, cancel := context.WithTimeout(ctx, d.waitTimeout)
    defer cancel()

    // 等待速率限制
    if err := d.limiter.Wait(waitCtx); err != nil {
        atomic.AddInt64(&d.rejectedCount, 1)
        return nil, fmt.Errorf("rate limit exceeded: %w", err)
    }

    return d.next.Generate(ctx, messages, options...)
}

// RetryDecorator 重试装饰器
type RetryDecorator struct {
    BaseModelDecorator
    strategy    RetryStrategy
    backoff     BackoffStrategy
    retryCount  int64
    successCount int64
}

func (d *RetryDecorator) Generate(ctx context.Context, messages []*schema.Message, options ...model.GenerateOption) (*schema.Message, error) {
    var lastErr error

    for attempt := 0; attempt <= d.strategy.MaxAttempts; attempt++ {
        if attempt > 0 {
            delay := d.backoff.NextDelay(attempt)
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return nil, ctx.Err()
            }
            atomic.AddInt64(&d.retryCount, 1)
        }

        response, err := d.next.Generate(ctx, messages, options...)
        if err == nil {
            if attempt > 0 {
                atomic.AddInt64(&d.successCount, 1)
            }
            return response, nil
        }

        lastErr = err

        // 检查是否应该重试
        if !d.strategy.ShouldRetry(err) {
            break
        }
    }

    return nil, lastErr
}

// ModelDecoratorChain 装饰器链
type ModelDecoratorChain struct {
    decorators []ModelDecorator
    baseModel  model.ChatModel
}

func NewModelDecoratorChain(baseModel model.ChatModel) *ModelDecoratorChain {
    return &ModelDecoratorChain{
        decorators: make([]ModelDecorator, 0),
        baseModel:  baseModel,
    }
}

func (chain *ModelDecoratorChain) AddDecorator(decorator ModelDecorator) *ModelDecoratorChain {
    chain.decorators = append(chain.decorators, decorator)
    return chain
}

func (chain *ModelDecoratorChain) Build() model.ChatModel {
    if len(chain.decorators) == 0 {
        return chain.baseModel
    }

    // 从最后一个装饰器开始，向前链接
    current := chain.baseModel
    for i := len(chain.decorators) - 1; i >= 0; i-- {
        decorator := chain.decorators[i]
        decorator.SetNext(current)
        current = decorator
    }

    return current
}
```

### 5.3 行为型模式

#### 5.3.1 策略模式 (Strategy Pattern)

```go
// ExecutionStrategy 执行策略接口
type ExecutionStrategy interface {
    Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error)
    CanHandle(plan *Plan) bool
    GetPriority() int
    GetName() string
}

// SequentialExecutionStrategy 顺序执行策略
type SequentialExecutionStrategy struct {
    name       string
    toolRegistry *ToolRegistry
    timeout    time.Duration
    maxRetries int
}

func (s *SequentialExecutionStrategy) Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error) {
    results := make([]StepResult, 0, len(plan.Steps))
    executionLog := make([]ExecutionEntry, 0)

    for i, step := range plan.Steps {
        stepCtx, cancel := context.WithTimeout(ctx, s.timeout)

        result, err := s.executeStep(stepCtx, &step, i)
        cancel()

        if err != nil {
            return &ExecutionResult{
                Success: false,
                Results: results,
                Error:   err,
                Logs:    executionLog,
            }, err
        }

        results = append(results, *result)
        executionLog = append(executionLog, ExecutionEntry{
            StepID:    step.ID,
            Status:    "completed",
            Result:    result.Data,
            Timestamp: time.Now(),
        })
    }

    return &ExecutionResult{
        Success: true,
        Results: results,
        Logs:    executionLog,
    }, nil
}

// ParallelExecutionStrategy 并行执行策略
type ParallelExecutionStrategy struct {
    name         string
    toolRegistry *ToolRegistry
    maxWorkers   int
    timeout      time.Duration
}

func (s *ParallelExecutionStrategy) Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error) {
    // 构建依赖图
    depGraph := s.buildDependencyGraph(plan)

    // 分层并行执行
    layers := s.topologicalSort(depGraph)
    results := make(map[string]*StepResult)
    executionLog := make([]ExecutionEntry, 0)

    for _, layer := range layers {
        // 并行执行当前层的所有步骤
        layerResults, err := s.executeLayer(ctx, layer)
        if err != nil {
            return &ExecutionResult{
                Success: false,
                Results: s.mapToSlice(results),
                Error:   err,
                Logs:    executionLog,
            }, err
        }

        // 合并结果
        for stepID, result := range layerResults {
            results[stepID] = result
            executionLog = append(executionLog, ExecutionEntry{
                StepID:    stepID,
                Status:    "completed",
                Result:    result.Data,
                Timestamp: time.Now(),
            })
        }
    }

    return &ExecutionResult{
        Success: true,
        Results: s.mapToSlice(results),
        Logs:    executionLog,
    }, nil
}

// AdaptiveExecutionStrategy 自适应执行策略
type AdaptiveExecutionStrategy struct {
    strategies []ExecutionStrategy
    selector   StrategySelector
    fallback   ExecutionStrategy
}

func (s *AdaptiveExecutionStrategy) Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error) {
    // 选择最合适的策略
    strategy := s.selector.SelectStrategy(plan, s.strategies)
    if strategy == nil {
        strategy = s.fallback
    }

    return strategy.Execute(ctx, plan)
}

// StrategySelector 策略选择器
type StrategySelector interface {
    SelectStrategy(plan *Plan, strategies []ExecutionStrategy) ExecutionStrategy
}

// SmartStrategySelector 智能策略选择器
type SmartStrategySelector struct {
    metrics       *StrategyMetrics
    preferences   *StrategyPreferences
    loadBalancer  LoadBalancer
}

func (s *SmartStrategySelector) SelectStrategy(plan *Plan, strategies []ExecutionStrategy) ExecutionStrategy {
    candidates := make([]ExecutionStrategy, 0)

    // 过滤可用策略
    for _, strategy := range strategies {
        if strategy.CanHandle(plan) {
            candidates = append(candidates, strategy)
        }
    }

    if len(candidates) == 0 {
        return nil
    }

    // 根据计划特征选择策略
    if s.hasComplexDependencies(plan) {
        return s.findStrategyByName(candidates, "parallel")
    }

    if s.isSimplePlan(plan) {
        return s.findStrategyByName(candidates, "sequential")
    }

    // 使用负载均衡选择
    return s.loadBalancer.SelectStrategy(candidates)
}
```

#### 5.3.2 观察者模式 (Observer Pattern)

```go
// EventBus 事件总线
type EventBus struct {
    subscribers map[EventType][]EventHandler
    middleware  []EventMiddleware
    metrics     *EventMetrics
    bufferSize  int
    workers     int
    eventQueue  chan *Event
    stopChan    chan struct{}
    mutex       sync.RWMutex
}

// Event 事件定义
type Event struct {
    ID         string                 `json:"id"`
    Type       EventType              `json:"type"`
    Source     string                 `json:"source"`
    Data       interface{}            `json:"data"`
    Metadata   map[string]interface{} `json:"metadata"`
    Timestamp  time.Time              `json:"timestamp"`
    Priority   EventPriority          `json:"priority"`
}

type EventType string

const (
    EventTypeStateChanged   EventType = "state_changed"
    EventTypeTaskCompleted  EventType = "task_completed"
    EventTypeTaskFailed     EventType = "task_failed"
    EventTypeAgentStarted   EventType = "agent_started"
    EventTypeAgentFinished  EventType = "agent_finished"
    EventTypeToolExecuted   EventType = "tool_executed"
    EventTypeErrorOccurred  EventType = "error_occurred"
)

type EventPriority int

const (
    PriorityLow    EventPriority = 1
    PriorityNormal EventPriority = 5
    PriorityHigh   EventPriority = 10
)

// EventHandler 事件处理器
type EventHandler interface {
    Handle(ctx context.Context, event *Event) error
    GetName() string
    GetPriority() int
}

// EventMiddleware 事件中间件
type EventMiddleware func(context.Context, *Event, EventHandler) error

// Subscribe 订阅事件
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) error {
    eb.mutex.Lock()
    defer eb.mutex.Unlock()

    if eb.subscribers[eventType] == nil {
        eb.subscribers[eventType] = make([]EventHandler, 0)
    }

    eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)

    // 按优先级排序
    sort.Slice(eb.subscribers[eventType], func(i, j int) bool {
        return eb.subscribers[eventType][i].GetPriority() > eb.subscribers[eventType][j].GetPriority()
    })

    return nil
}

// Publish 发布事件
func (eb *EventBus) Publish(ctx context.Context, event *Event) error {
    // 添加事件ID和时间戳
    if event.ID == "" {
        event.ID = generateEventID()
    }
    if event.Timestamp.IsZero() {
        event.Timestamp = time.Now()
    }

    // 发送到事件队列
    select {
    case eb.eventQueue <- event:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    default:
        return fmt.Errorf("event queue is full")
    }
}

// processEvents 处理事件队列
func (eb *EventBus) processEvents(ctx context.Context) {
    for {
        select {
        case event := <-eb.eventQueue:
            eb.handleEvent(ctx, event)
        case <-eb.stopChan:
            return
        case <-ctx.Done():
            return
        }
    }
}

func (eb *EventBus) handleEvent(ctx context.Context, event *Event) {
    eb.mutex.RLock()
    handlers, exists := eb.subscribers[event.Type]
    eb.mutex.RUnlock()

    if !exists {
        return
    }

    // 并发处理所有处理器
    var wg sync.WaitGroup
    for _, handler := range handlers {
        wg.Add(1)
        go func(h EventHandler) {
            defer wg.Done()
            eb.executeHandler(ctx, event, h)
        }(handler)
    }

    wg.Wait()
}

func (eb *EventBus) executeHandler(ctx context.Context, event *Event, handler EventHandler) {
    startTime := time.Now()

    // 应用中间件
    finalHandler := handler.Handle
    for i := len(eb.middleware) - 1; i >= 0; i-- {
        middleware := eb.middleware[i]
        finalHandler = func(ctx context.Context, event *Event) error {
            return middleware(ctx, event, &handlerAdapter{handler: finalHandler})
        }
    }

    // 执行处理器
    err := finalHandler(ctx, event)
    duration := time.Since(startTime)

    // 记录指标
    eb.metrics.RecordHandlerExecution(handler.GetName(), duration, err)

    if err != nil {
        log.Printf("Event handler %s failed: %v", handler.GetName(), err)
    }
}

// StateChangeEventHandler 状态变更事件处理器
type StateChangeEventHandler struct {
    name         string
    stateManager *StateManager
    notifier     Notifier
}

func (h *StateChangeEventHandler) Handle(ctx context.Context, event *Event) error {
    stateData, ok := event.Data.(*StateChangeData)
    if !ok {
        return fmt.Errorf("invalid state change data")
    }

    // 发送通知
    notification := &Notification{
        Type:      NotificationTypeStateChange,
        Title:     fmt.Sprintf("State changed to %s", stateData.NewState),
        Message:   fmt.Sprintf("System state changed from %s to %s", stateData.OldState, stateData.NewState),
        Metadata:  event.Metadata,
        Timestamp: event.Timestamp,
    }

    return h.notifier.Send(ctx, notification)
}

type StateChangeData struct {
    OldState ExecutionStatus `json:"old_state"`
    NewState ExecutionStatus `json:"new_state"`
    Reason   string          `json:"reason"`
    Context  map[string]interface{} `json:"context"`
}
```

## 6. 扩展性设计

### 6.1 插件系统

#### 6.1.1 插件接口定义

```go
// Plugin 插件接口
type Plugin interface {
    // 基础信息
    GetName() string
    GetVersion() string
    GetDescription() string
    GetAuthor() string

    // 生命周期
    Initialize(ctx context.Context, config PluginConfig) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Cleanup(ctx context.Context) error

    // 功能检查
    GetCapabilities() []string
    IsCompatible(version string) bool

    // 健康检查
    HealthCheck(ctx context.Context) error
}

// ModelProviderPlugin 模型提供商插件
type ModelProviderPlugin interface {
    Plugin
    CreateChatModel(ctx context.Context, config ModelConfig) (model.ChatModel, error)
    CreateEmbeddingModel(ctx context.Context, config ModelConfig) (embedding.EmbeddingModel, error)
    SupportedModels() []ModelInfo
    ValidateConfig(config ModelConfig) error
}

// ToolProviderPlugin 工具提供商插件
type ToolProviderPlugin interface {
    Plugin
    GetTools(ctx context.Context) ([]tool.BaseTool, error)
    GetToolByName(ctx context.Context, name string) (tool.BaseTool, error)
    RegisterTool(ctx context.Context, tool tool.BaseTool) error
    UnregisterTool(ctx context.Context, name string) error
}

// CallbackPlugin 回调插件
type CallbackPlugin interface {
    Plugin
    CreateHandler(ctx context.Context, config CallbackConfig) (callbacks.Handler, error)
    GetSupportedEvents() []EventType
    GetConfigSchema() *schema.JSONSchema
}

// PluginManager 插件管理器
type PluginManager struct {
    plugins      map[string]Plugin
    registry     *PluginRegistry
    loader       *PluginLoader
    configManager *PluginConfigManager
    lifecycle    *PluginLifecycleManager
    dependencies *DependencyManager
    security     *PluginSecurity
    metrics      *PluginMetrics
    mutex        sync.RWMutex
}

// PluginRegistry 插件注册表
type PluginRegistry struct {
    plugins     map[string]*PluginInfo
    categories  map[string][]string
    index       *PluginIndex
    mutex       sync.RWMutex
}

type PluginInfo struct {
    Name         string            `json:"name"`
    Version      string            `json:"version"`
    Description  string            `json:"description"`
    Author       string            `json:"author"`
    Category     string            `json:"category"`
    Tags         []string          `json:"tags"`
    Dependencies []PluginDependency `json:"dependencies"`
    Capabilities []string          `json:"capabilities"`
    ConfigSchema *schema.JSONSchema `json:"config_schema"`
    License      string            `json:"license"`
    Homepage     string            `json:"homepage"`
    Repository   string            `json:"repository"`
    CreatedAt    time.Time         `json:"created_at"`
    UpdatedAt    time.Time         `json:"updated_at"`
}

type PluginDependency struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Optional bool  `json:"optional"`
}
```

#### 6.1.2 插件加载和管理

```go
// PluginLoader 插件加载器
type PluginLoader struct {
    searchPaths []string
    cache       *PluginCache
    validator   *PluginValidator
    sandboxer   *PluginSandbox
}

func (pl *PluginLoader) LoadPlugin(ctx context.Context, pluginPath string) (Plugin, error) {
    // 验证插件文件
    if err := pl.validator.ValidateFile(pluginPath); err != nil {
        return nil, fmt.Errorf("plugin validation failed: %w", err)
    }

    // 检查缓存
    if cached := pl.cache.Get(pluginPath); cached != nil {
        return cached, nil
    }

    // 创建沙箱环境
    sandbox, err := pl.sandboxer.CreateSandbox(ctx, pluginPath)
    if err != nil {
        return nil, fmt.Errorf("failed to create sandbox: %w", err)
    }

    // 加载插件
    plugin, err := pl.loadPluginInSandbox(ctx, sandbox, pluginPath)
    if err != nil {
        sandbox.Cleanup()
        return nil, err
    }

    // 缓存插件
    pl.cache.Set(pluginPath, plugin)

    return plugin, nil
}

// PluginLifecycleManager 插件生命周期管理器
type PluginLifecycleManager struct {
    states   map[string]PluginState
    hooks    map[PluginState][]LifecycleHook
    metrics  *PluginMetrics
    mutex    sync.RWMutex
}

type PluginState string

const (
    StateUnloaded    PluginState = "unloaded"
    StateLoaded      PluginState = "loaded"
    StateInitialized PluginState = "initialized"
    StateStarted     PluginState = "started"
    StateError       PluginState = "error"
    StateStopped     PluginState = "stopped"
)

type LifecycleHook func(ctx context.Context, plugin Plugin, oldState, newState PluginState) error

func (plm *PluginLifecycleManager) TransitionTo(ctx context.Context, pluginName string, targetState PluginState) error {
    plm.mutex.Lock()
    defer plm.mutex.Unlock()

    currentState := plm.states[pluginName]

    // 执行前置钩子
    if err := plm.executeHooks(ctx, pluginName, currentState, targetState); err != nil {
        return fmt.Errorf("pre-transition hook failed: %w", err)
    }

    // 更新状态
    plm.states[pluginName] = targetState

    // 记录指标
    plm.metrics.RecordStateTransition(pluginName, currentState, targetState)

    return nil
}

// DependencyManager 依赖管理器
type DependencyManager struct {
    graph      *DependencyGraph
    resolver   *DependencyResolver
    downloader *DependencyDownloader
}

func (dm *DependencyManager) ResolveDependencies(plugin *PluginInfo) ([]*PluginInfo, error) {
    // 构建依赖图
    if err := dm.graph.AddPlugin(plugin); err != nil {
        return nil, err
    }

    // 解析依赖
    dependencies, err := dm.resolver.Resolve(plugin.Dependencies)
    if err != nil {
        return nil, err
    }

    // 检查循环依赖
    if dm.graph.HasCycle() {
        return nil, fmt.Errorf("circular dependency detected")
    }

    return dependencies, nil
}

// PluginSecurity 插件安全管理
type PluginSecurity struct {
    policies    []SecurityPolicy
    scanner     *SecurityScanner
    permissions *PermissionManager
}

type SecurityPolicy interface {
    Evaluate(ctx context.Context, plugin Plugin) (*SecurityResult, error)
    GetName() string
    GetSeverity() SecuritySeverity
}

type SecurityResult struct {
    Allowed   bool                   `json:"allowed"`
    Reason    string                 `json:"reason"`
    Warnings  []string               `json:"warnings"`
    Metadata  map[string]interface{} `json:"metadata"`
}
```

### 6.2 API 扩展性

#### 6.2.1 可扩展的 API 设计

```go
// ExtensibleAPI 可扩展API接口
type ExtensibleAPI interface {
    // 核心方法
    Process(ctx context.Context, request *ProcessRequest) (*ProcessResponse, error)

    // 扩展方法
    RegisterExtension(name string, extension APIExtension) error
    UnregisterExtension(name string) error
    GetExtensions() []string

    // 中间件支持
    Use(middleware APIMiddleware) error
    UseFor(path string, middleware APIMiddleware) error

    // 路由扩展
    AddRoute(route *Route) error
    RemoveRoute(path string) error

    // 验证扩展
    AddValidator(validator RequestValidator) error
    AddTransformer(transformer RequestTransformer) error
}

// APIExtension API扩展接口
type APIExtension interface {
    GetName() string
    GetVersion() string
    GetPaths() []string
    Handle(ctx context.Context, request *ExtensionRequest) (*ExtensionResponse, error)
    Initialize(ctx context.Context, config ExtensionConfig) error
    Cleanup(ctx context.Context) error
}

// CustomAgentExtension 自定义智能体扩展
type CustomAgentExtension struct {
    name           string
    version        string
    agentFactory   AgentFactory
    configSchema   *schema.JSONSchema
    capabilities   []string
}

func (e *CustomAgentExtension) Handle(ctx context.Context, request *ExtensionRequest) (*ExtensionResponse, error) {
    switch request.Action {
    case "create_agent":
        return e.handleCreateAgent(ctx, request)
    case "list_agents":
        return e.handleListAgents(ctx, request)
    case "get_agent":
        return e.handleGetAgent(ctx, request)
    case "delete_agent":
        return e.handleDeleteAgent(ctx, request)
    default:
        return nil, fmt.Errorf("unsupported action: %s", request.Action)
    }
}

func (e *CustomAgentExtension) GetPaths() []string {
    return []string{
        "/api/v1/agents/custom",
        "/api/v1/agents/custom/{id}",
        "/api/v1/agents/custom/{id}/execute",
    }
}

// WorkflowExtension 工作流扩展
type WorkflowExtension struct {
    name           string
    workflowEngine *WorkflowEngine
    templates      map[string]*WorkflowTemplate
}

func (e *WorkflowExtension) Handle(ctx context.Context, request *ExtensionRequest) (*ExtensionResponse, error) {
    switch request.Action {
    case "create_workflow":
        return e.handleCreateWorkflow(ctx, request)
    case "execute_workflow":
        return e.handleExecuteWorkflow(ctx, request)
    case "get_workflow_status":
        return e.handleGetWorkflowStatus(ctx, request)
    default:
        return nil, fmt.Errorf("unsupported workflow action: %s", request.Action)
    }
}

// APIMiddleware API中间件
type APIMiddleware func(context.Context, *APIRequest, APIHandler) (*APIResponse, error)

// AuthenticationMiddleware 认证中间件
func AuthenticationMiddleware(authService AuthService) APIMiddleware {
    return func(ctx context.Context, request *APIRequest, next APIHandler) (*APIResponse, error) {
        // 提取认证信息
        authHeader := request.Headers["Authorization"]
        if authHeader == "" {
            return &APIResponse{
                StatusCode: 401,
                Body:       map[string]interface{}{"error": "missing authorization header"},
            }, nil
        }

        // 验证token
        user, err := authService.ValidateToken(ctx, authHeader)
        if err != nil {
            return &APIResponse{
                StatusCode: 401,
                Body:       map[string]interface{}{"error": "invalid token"},
            }, nil
        }

        // 将用户信息添加到上下文
        ctx = context.WithValue(ctx, "user", user)

        return next(ctx, request)
    }
}

// RateLimitingMiddleware 速率限制中间件
func RateLimitingMiddleware(limiter RateLimiter) APIMiddleware {
    return func(ctx context.Context, request *APIRequest, next APIHandler) (*APIResponse, error) {
        // 获取客户端标识
        clientID := extractClientID(request)

        // 检查速率限制
        if !limiter.Allow(clientID) {
            return &APIResponse{
                StatusCode: 429,
                Body:       map[string]interface{}{"error": "rate limit exceeded"},
                Headers:    map[string]string{"Retry-After": "60"},
            }, nil
        }

        return next(ctx, request)
    }
}
```

### 6.3 配置扩展性

#### 6.3.1 动态配置系统

```go
// ConfigManager 配置管理器
type ConfigManager struct {
    providers    []ConfigProvider
    watchers     map[string]ConfigWatcher
    cache        ConfigCache
    validator    ConfigValidator
    transformer  ConfigTransformer
    eventBus     *EventBus
    mutex        sync.RWMutex
}

// ConfigProvider 配置提供者
type ConfigProvider interface {
    GetName() string
    Load(ctx context.Context, path string) (map[string]interface{}, error)
    Watch(ctx context.Context, path string, callback ConfigChangeCallback) error
    Save(ctx context.Context, path string, config map[string]interface{}) error
    Supports(path string) bool
}

// YAMLConfigProvider YAML配置提供者
type YAMLConfigProvider struct {
    name string
    fs   FileSystem
}

func (p *YAMLConfigProvider) Load(ctx context.Context, path string) (map[string]interface{}, error) {
    data, err := p.fs.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }

    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse YAML config: %w", err)
    }

    return config, nil
}

// ConsulConfigProvider Consul配置提供者
type ConsulConfigProvider struct {
    name   string
    client ConsulClient
    prefix string
}

func (p *ConsulConfigProvider) Load(ctx context.Context, path string) (map[string]interface{}, error) {
    fullPath := fmt.Sprintf("%s/%s", p.prefix, path)

    kvPairs, err := p.client.GetKVList(ctx, fullPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load from Consul: %w", err)
    }

    config := make(map[string]interface{})
    for _, kv := range kvPairs {
        key := strings.TrimPrefix(kv.Key, fullPath+"/")
        config[key] = string(kv.Value)
    }

    return config, nil
}

// ConfigWatcher 配置监听器
type ConfigWatcher interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    GetPath() string
    GetProvider() string
}

type ConfigChangeCallback func(ctx context.Context, oldConfig, newConfig map[string]interface{}) error

// DynamicConfig 动态配置
type DynamicConfig struct {
    manager     *ConfigManager
    path        string
    provider    string
    current     atomic.Value
    subscribers []ConfigSubscriber
    mutex       sync.RWMutex
}

type ConfigSubscriber interface {
    OnConfigChanged(ctx context.Context, oldConfig, newConfig interface{}) error
    GetName() string
}

func (dc *DynamicConfig) Get() interface{} {
    return dc.current.Load()
}

func (dc *DynamicConfig) Set(config interface{}) {
    oldConfig := dc.current.Load()
    dc.current.Store(config)

    // 通知订阅者
    dc.notifySubscribers(context.Background(), oldConfig, config)
}

func (dc *DynamicConfig) Subscribe(subscriber ConfigSubscriber) {
    dc.mutex.Lock()
    defer dc.mutex.Unlock()
    dc.subscribers = append(dc.subscribers, subscriber)
}

func (dc *DynamicConfig) notifySubscribers(ctx context.Context, oldConfig, newConfig interface{}) {
    dc.mutex.RLock()
    defer dc.mutex.RUnlock()

    for _, subscriber := range dc.subscribers {
        go func(sub ConfigSubscriber) {
            if err := sub.OnConfigChanged(ctx, oldConfig, newConfig); err != nil {
                log.Printf("Config subscriber %s error: %v", sub.GetName(), err)
            }
        }(subscriber)
    }
}

// ConfigValidator 配置验证器
type ConfigValidator interface {
    Validate(ctx context.Context, config map[string]interface{}) error
    GetSchema() *schema.JSONSchema
}

// SchemaConfigValidator JSON Schema配置验证器
type SchemaConfigValidator struct {
    schema *schema.JSONSchema
}

func (v *SchemaConfigValidator) Validate(ctx context.Context, config map[string]interface{}) error {
    return v.schema.Validate(config)
}

// ConfigTransformer 配置转换器
type ConfigTransformer interface {
    Transform(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error)
    GetName() string
}

// EnvironmentVariableTransformer 环境变量转换器
type EnvironmentVariableTransformer struct {
    prefix string
}

func (t *EnvironmentVariableTransformer) Transform(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
    transformed := make(map[string]interface{})

    for key, value := range config {
        if strValue, ok := value.(string); ok {
            // 替换环境变量
            expanded := os.ExpandEnv(strValue)
            transformed[key] = expanded
        } else {
            transformed[key] = value
        }
    }

    return transformed, nil
}
```

### 6.4 数据存储扩展性

#### 6.4.1 存储抽象层

```go
// StorageProvider 存储提供者接口
type StorageProvider interface {
    // 基础操作
    Get(ctx context.Context, key string) ([]byte, error)
    Put(ctx context.Context, key string, value []byte) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)

    // 批量操作
    GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)
    PutMulti(ctx context.Context, items map[string][]byte) error
    DeleteMulti(ctx context.Context, keys []string) error

    // 列表操作
    List(ctx context.Context, prefix string) ([]string, error)
    ListWithDetails(ctx context.Context, prefix string) ([]*StorageItem, error)

    // 事务操作
    BeginTransaction(ctx context.Context) (Transaction, error)

    // 监听变化
    Watch(ctx context.Context, prefix string) (<-chan *StorageEvent, error)

    // 管理操作
    Close() error
    Stats(ctx context.Context) (*StorageStats, error)
}

// Transaction 存储事务
type Transaction interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Put(ctx context.Context, key string, value []byte) error
    Delete(ctx context.Context, key string) error
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}

// StorageItem 存储项
type StorageItem struct {
    Key       string            `json:"key"`
    Value     []byte            `json:"value"`
    Metadata  map[string]string `json:"metadata"`
    Version   int64             `json:"version"`
    CreatedAt time.Time         `json:"created_at"`
    UpdatedAt time.Time         `json:"updated_at"`
    Size      int64             `json:"size"`
}

// StorageEvent 存储事件
type StorageEvent struct {
    Type      StorageEventType `json:"type"`
    Key       string           `json:"key"`
    Value     []byte           `json:"value"`
    OldValue  []byte           `json:"old_value"`
    Timestamp time.Time        `json:"timestamp"`
}

type StorageEventType string

const (
    StorageEventPut    StorageEventType = "put"
    StorageEventDelete StorageEventType = "delete"
    StorageEventUpdate StorageEventType = "update"
)

// RedisStorageProvider Redis存储提供者
type RedisStorageProvider struct {
    client redis.Client
    prefix string
    config *RedisConfig
}

func (p *RedisStorageProvider) Get(ctx context.Context, key string) ([]byte, error) {
    fullKey := p.buildKey(key)
    result, err := p.client.Get(ctx, fullKey).Bytes()
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return nil, ErrKeyNotFound
        }
        return nil, err
    }
    return result, nil
}

// EtcdStorageProvider etcd存储提供者
type EtcdStorageProvider struct {
    client etcd.Client
    prefix string
    config *EtcdConfig
}

func (p *EtcdStorageProvider) Watch(ctx context.Context, prefix string) (<-chan *StorageEvent, error) {
    fullPrefix := p.buildKey(prefix)
    watchChan := p.client.Watch(ctx, fullPrefix, etcd.WithPrefix())

    eventChan := make(chan *StorageEvent, 100)

    go func() {
        defer close(eventChan)

        for watchResponse := range watchChan {
            for _, event := range watchResponse.Events {
                storageEvent := &StorageEvent{
                    Key:       strings.TrimPrefix(string(event.Kv.Key), p.prefix+"/"),
                    Value:     event.Kv.Value,
                    Timestamp: time.Now(),
                }

                switch event.Type {
                case etcd.EventTypePut:
                    storageEvent.Type = StorageEventPut
                case etcd.EventTypeDelete:
                    storageEvent.Type = StorageEventDelete
                }

                select {
                case eventChan <- storageEvent:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()

    return eventChan, nil
}

// StorageManager 存储管理器
type StorageManager struct {
    providers map[string]StorageProvider
    router    StorageRouter
    cache     StorageCache
    metrics   *StorageMetrics
    config    *StorageConfig
}

// StorageRouter 存储路由器
type StorageRouter interface {
    Route(ctx context.Context, operation StorageOperation, key string) (StorageProvider, error)
    AddRule(rule *RoutingRule) error
    RemoveRule(ruleID string) error
}

type RoutingRule struct {
    ID         string            `json:"id"`
    Pattern    string            `json:"pattern"`
    Provider   string            `json:"provider"`
    Priority   int               `json:"priority"`
    Conditions map[string]string `json:"conditions"`
}

// ConsistentHashRouter 一致性哈希路由器
type ConsistentHashRouter struct {
    ring      *ConsistentHashRing
    providers map[string]StorageProvider
    replicas  int
}

func (r *ConsistentHashRouter) Route(ctx context.Context, operation StorageOperation, key string) (StorageProvider, error) {
    providerName := r.ring.Get(key)
    if provider, exists := r.providers[providerName]; exists {
        return provider, nil
    }
    return nil, fmt.Errorf("provider %s not found", providerName)
}
```

这样我们就完成了UnifyDialog项目的详细设计方案文档。整个设计方案包含了：

1. **项目概述和架构设计** - 分层架构和设计原则
2. **核心类设计与关系** - 配置管理、模型工厂、工具管理、回调管理
3. **状态管理和多智能体系统** - 状态流转、智能体编排
4. **数据流和系统交互** - 消息传递、错误处理、性能监控
5. **设计模式应用** - 工厂模式、建造者模式、适配器模式、装饰器模式、策略模式、观察者模式
6. **扩展性设计** - 插件系统、API扩展、配置扩展、存储扩展

<function_calls>
<invoke name="TodoWrite">
<parameter name="todos">[{"content": "\u521b\u5efa\u8be6\u7ec6\u7684\u8bbe\u8ba1\u65b9\u6848\u6587\u6863", "status": "completed", "activeForm": "Creating detailed design documentation"}, {"content": "\u5206\u6790\u6838\u5fc3\u7c7b\u7684\u5173\u7cfb\u548c\u5b9e\u73b0", "status": "completed", "activeForm": "Analyzing core class relationships and implementations"}, {"content": "\u7f16\u5199\u67b6\u6784\u8bbe\u8ba1\u548c\u6570\u636e\u6d41", "status": "completed", "activeForm": "Writing architecture design and data flow"}, {"content": "\u6587\u6863\u5316\u6269\u5c55\u6027\u548c\u6027\u80fd\u8bbe\u8ba1", "status": "completed", "activeForm": "Documenting extensibility and performance design"}]