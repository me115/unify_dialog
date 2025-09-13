# UnifyDialog 设计方案 - 第二部分

## 3.3 工具管理子系统

### 3.3.1 MCPToolManager 类设计

```go
type MCPToolManager struct {
    config      *UnifyDialogConfig
    tools       map[string]tool.BaseTool
    clients     map[string]*MCPClient       // MCP客户端连接池
    lifecycle   *ToolLifecycleManager       // 工具生命周期管理
    healthcheck *HealthCheckManager         // 健康检查管理
    mutex       sync.RWMutex               // 并发安全
}

type MCPClient struct {
    serverName   string
    command      string
    args         []string
    env          map[string]string
    process      *exec.Cmd
    stdin        io.WriteCloser
    stdout       io.ReadCloser
    stderr       io.ReadCloser
    isConnected  bool
    lastActivity time.Time
    mutex        sync.Mutex
}

type ToolLifecycleManager struct {
    startupHooks  []func(toolName string) error
    shutdownHooks []func(toolName string) error
    restartPolicy RestartPolicy
}

type RestartPolicy struct {
    MaxRestarts     int
    RestartDelay    time.Duration
    BackoffMultiplier float64
    MaxRestartDelay time.Duration
}
```

**核心方法实现**:

```go
// Initialize 初始化MCP工具管理器
func (m *MCPToolManager) Initialize(ctx context.Context) error {
    if !m.config.MCP.Enabled {
        log.Println("MCP is disabled, using mock tools")
        return m.initializeMockTools()
    }

    // 初始化MCP服务器连接
    for serverName, serverConfig := range m.config.MCP.Servers {
        if err := m.initializeMCPServer(ctx, serverName, serverConfig); err != nil {
            log.Printf("Failed to initialize MCP server %s: %v", serverName, err)
            // 根据配置决定是否继续
            if m.config.System.Debug {
                continue // 调试模式下容错
            }
            return fmt.Errorf("critical MCP server %s failed to initialize: %w", serverName, err)
        }
    }

    // 启动健康检查
    if err := m.startHealthCheck(ctx); err != nil {
        return fmt.Errorf("failed to start health check: %w", err)
    }

    log.Printf("MCP Tool Manager initialized with %d servers, %d tools",
        len(m.clients), len(m.tools))
    return nil
}

// initializeMCPServer 初始化单个MCP服务器
func (m *MCPToolManager) initializeMCPServer(ctx context.Context, serverName string, config MCPServerConfig) error {
    // 创建MCP客户端
    client := &MCPClient{
        serverName: serverName,
        command:    config.Command,
        args:       config.Args,
        env:        config.Env,
    }

    // 启动MCP服务器进程
    if err := client.Connect(ctx); err != nil {
        return fmt.Errorf("failed to connect to MCP server: %w", err)
    }

    // 发现服务器提供的工具
    tools, err := client.DiscoverTools(ctx)
    if err != nil {
        client.Disconnect()
        return fmt.Errorf("failed to discover tools: %w", err)
    }

    // 注册工具
    m.mutex.Lock()
    m.clients[serverName] = client
    for _, tool := range tools {
        toolName := fmt.Sprintf("%s_%s", serverName, tool.GetName())
        m.tools[toolName] = &MCPProxyTool{
            client:     client,
            serverName: serverName,
            toolName:   tool.GetName(),
            toolInfo:   tool,
            manager:    m,
        }
        log.Printf("Registered MCP tool: %s", toolName)
    }
    m.mutex.Unlock()

    return nil
}

// Connect MCP客户端连接实现
func (c *MCPClient) Connect(ctx context.Context) error {
    c.mutex.Lock()
    defer c.mutex.Unlock()

    if c.isConnected {
        return nil
    }

    // 创建进程
    c.process = exec.CommandContext(ctx, c.command, c.args...)
    c.process.Env = os.Environ()

    // 添加自定义环境变量
    for key, value := range c.env {
        c.process.Env = append(c.process.Env, fmt.Sprintf("%s=%s", key, value))
    }

    // 设置管道
    stdin, err := c.process.StdinPipe()
    if err != nil {
        return fmt.Errorf("failed to create stdin pipe: %w", err)
    }
    c.stdin = stdin

    stdout, err := c.process.StdoutPipe()
    if err != nil {
        return fmt.Errorf("failed to create stdout pipe: %w", err)
    }
    c.stdout = stdout

    stderr, err := c.process.StderrPipe()
    if err != nil {
        return fmt.Errorf("failed to create stderr pipe: %w", err)
    }
    c.stderr = stderr

    // 启动进程
    if err := c.process.Start(); err != nil {
        return fmt.Errorf("failed to start MCP server process: %w", err)
    }

    // 等待服务器就绪
    if err := c.waitForReady(ctx); err != nil {
        c.process.Kill()
        return fmt.Errorf("MCP server failed to become ready: %w", err)
    }

    c.isConnected = true
    c.lastActivity = time.Now()

    log.Printf("MCP client connected to server: %s", c.serverName)
    return nil
}

// MCPProxyTool MCP工具代理实现
type MCPProxyTool struct {
    client     *MCPClient
    serverName string
    toolName   string
    toolInfo   *ToolInfo
    manager    *MCPToolManager
}

func (t *MCPProxyTool) Info(ctx context.Context) (*tool.Info, error) {
    return &tool.Info{
        Name: t.toolName,
        Desc: t.toolInfo.Description,
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "params": {
                Type:        schema.ParameterTypeObject,
                Description: "Tool parameters",
                Required:    false,
            },
        }),
    }, nil
}

func (t *MCPProxyTool) Run(ctx context.Context, params string, options ...tool.Option) (string, error) {
    // 检查客户端连接状态
    if !t.client.isConnected {
        if err := t.client.Connect(ctx); err != nil {
            return "", fmt.Errorf("failed to reconnect to MCP server: %w", err)
        }
    }

    // 构建MCP请求
    request := &MCPRequest{
        Method: "tools/call",
        Params: map[string]interface{}{
            "name":      t.toolName,
            "arguments": params,
        },
    }

    // 发送请求
    response, err := t.client.SendRequest(ctx, request)
    if err != nil {
        return "", fmt.Errorf("MCP tool call failed: %w", err)
    }

    // 处理响应
    if response.Error != nil {
        return "", fmt.Errorf("MCP tool error: %s", response.Error.Message)
    }

    result, ok := response.Result.(string)
    if !ok {
        resultJSON, _ := json.Marshal(response.Result)
        result = string(resultJSON)
    }

    return result, nil
}
```

### 3.3.2 ToolRegistry 统一工具注册

```go
type ToolRegistry struct {
    mcpManager   *MCPToolManager
    customTools  map[string]tool.BaseTool
    toolMetadata map[string]*ToolMetadata
    eventBus     *ToolEventBus
    mutex        sync.RWMutex
}

type ToolMetadata struct {
    Name         string            `json:"name"`
    Type         string            `json:"type"`          // "mcp", "custom", "builtin"
    Category     string            `json:"category"`      // "database", "file", "api", etc.
    Tags         []string          `json:"tags"`
    Version      string            `json:"version"`
    Author       string            `json:"author"`
    Description  string            `json:"description"`
    Usage        string            `json:"usage"`
    Examples     []ToolExample     `json:"examples"`
    Dependencies []string          `json:"dependencies"`
    Metrics      *ToolMetrics      `json:"metrics"`
    CreatedAt    time.Time         `json:"created_at"`
    UpdatedAt    time.Time         `json:"updated_at"`
}

type ToolExample struct {
    Description string                 `json:"description"`
    Input       map[string]interface{} `json:"input"`
    Output      string                 `json:"output"`
}

type ToolMetrics struct {
    CallCount     int64         `json:"call_count"`
    SuccessCount  int64         `json:"success_count"`
    ErrorCount    int64         `json:"error_count"`
    AverageTime   time.Duration `json:"average_time"`
    LastUsed      time.Time     `json:"last_used"`
}

type ToolEventBus struct {
    subscribers map[ToolEventType][]ToolEventHandler
    mutex       sync.RWMutex
}

type ToolEventType string

const (
    ToolRegistered   ToolEventType = "tool_registered"
    ToolUnregistered ToolEventType = "tool_unregistered"
    ToolExecuted     ToolEventType = "tool_executed"
    ToolFailed       ToolEventType = "tool_failed"
)

type ToolEventHandler func(event *ToolEvent)

type ToolEvent struct {
    Type      ToolEventType          `json:"type"`
    ToolName  string                 `json:"tool_name"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
}
```

**核心方法实现**:

```go
// RegisterCustomTool 注册自定义工具
func (r *ToolRegistry) RegisterCustomTool(name string, tool tool.BaseTool, metadata *ToolMetadata) error {
    r.mutex.Lock()
    defer r.mutex.Unlock()

    // 检查是否已存在
    if _, exists := r.customTools[name]; exists {
        return fmt.Errorf("tool %s already registered", name)
    }

    // 设置默认元数据
    if metadata == nil {
        metadata = &ToolMetadata{
            Name:        name,
            Type:        "custom",
            Category:    "general",
            Version:     "1.0.0",
            Description: "Custom tool",
            Metrics:     &ToolMetrics{},
            CreatedAt:   time.Now(),
            UpdatedAt:   time.Now(),
        }
    }

    // 注册工具
    r.customTools[name] = tool
    r.toolMetadata[name] = metadata

    // 发布事件
    r.eventBus.Publish(&ToolEvent{
        Type:      ToolRegistered,
        ToolName:  name,
        Timestamp: time.Now(),
        Data: map[string]interface{}{
            "type": metadata.Type,
            "category": metadata.Category,
        },
    })

    log.Printf("Registered custom tool: %s", name)
    return nil
}

// GetAllTools 获取所有可用工具
func (r *ToolRegistry) GetAllTools() []tool.BaseTool {
    r.mutex.RLock()
    defer r.mutex.RUnlock()

    var tools []tool.BaseTool

    // 添加MCP工具
    for _, tool := range r.mcpManager.GetTools() {
        tools = append(tools, tool)
    }

    // 添加自定义工具
    for _, tool := range r.customTools {
        tools = append(tools, tool)
    }

    return tools
}

// GetToolByName 根据名称获取工具
func (r *ToolRegistry) GetToolByName(name string) (tool.BaseTool, bool) {
    r.mutex.RLock()
    defer r.mutex.RUnlock()

    // 先查找MCP工具
    if tool, ok := r.mcpManager.GetTool(name); ok {
        return tool, true
    }

    // 再查找自定义工具
    if tool, ok := r.customTools[name]; ok {
        return tool, true
    }

    return nil, false
}

// ExecuteTool 执行工具并记录指标
func (r *ToolRegistry) ExecuteTool(ctx context.Context, name string, params string) (string, error) {
    startTime := time.Now()

    // 获取工具
    tool, ok := r.GetToolByName(name)
    if !ok {
        return "", fmt.Errorf("tool %s not found", name)
    }

    // 更新调用计数
    r.updateToolMetrics(name, func(metrics *ToolMetrics) {
        metrics.CallCount++
        metrics.LastUsed = startTime
    })

    // 执行工具
    result, err := tool.Run(ctx, params)

    // 计算执行时间
    executionTime := time.Since(startTime)

    // 更新指标
    r.updateToolMetrics(name, func(metrics *ToolMetrics) {
        if err != nil {
            metrics.ErrorCount++
        } else {
            metrics.SuccessCount++
        }

        // 更新平均时间
        if metrics.CallCount > 0 {
            metrics.AverageTime = (metrics.AverageTime*time.Duration(metrics.CallCount-1) + executionTime) / time.Duration(metrics.CallCount)
        } else {
            metrics.AverageTime = executionTime
        }
    })

    // 发布事件
    eventType := ToolExecuted
    if err != nil {
        eventType = ToolFailed
    }

    r.eventBus.Publish(&ToolEvent{
        Type:      eventType,
        ToolName:  name,
        Timestamp: time.Now(),
        Data: map[string]interface{}{
            "execution_time": executionTime,
            "error":         err,
            "params":        params,
        },
    })

    return result, err
}

// GetToolMetadata 获取工具元数据
func (r *ToolRegistry) GetToolMetadata(name string) (*ToolMetadata, bool) {
    r.mutex.RLock()
    defer r.mutex.RUnlock()

    metadata, ok := r.toolMetadata[name]
    return metadata, ok
}

// ListToolsByCategory 按分类列出工具
func (r *ToolRegistry) ListToolsByCategory(category string) []string {
    r.mutex.RLock()
    defer r.mutex.RUnlock()

    var tools []string
    for name, metadata := range r.toolMetadata {
        if metadata.Category == category {
            tools = append(tools, name)
        }
    }

    return tools
}

// SearchTools 搜索工具
func (r *ToolRegistry) SearchTools(query string) []string {
    r.mutex.RLock()
    defer r.mutex.RUnlock()

    query = strings.ToLower(query)
    var matches []string

    for name, metadata := range r.toolMetadata {
        if strings.Contains(strings.ToLower(name), query) ||
           strings.Contains(strings.ToLower(metadata.Description), query) ||
           r.containsTag(metadata.Tags, query) {
            matches = append(matches, name)
        }
    }

    return matches
}

func (r *ToolRegistry) containsTag(tags []string, query string) bool {
    for _, tag := range tags {
        if strings.Contains(strings.ToLower(tag), query) {
            return true
        }
    }
    return false
}

// GetToolStats 获取工具统计信息
func (r *ToolRegistry) GetToolStats() map[string]interface{} {
    r.mutex.RLock()
    defer r.mutex.RUnlock()

    stats := map[string]interface{}{
        "total_tools":    len(r.toolMetadata),
        "mcp_tools":      len(r.mcpManager.GetTools()),
        "custom_tools":   len(r.customTools),
        "categories":     make(map[string]int),
        "total_calls":    int64(0),
        "total_errors":   int64(0),
        "average_time":   time.Duration(0),
    }

    categoryCount := make(map[string]int)
    var totalCalls, totalErrors int64
    var totalTime time.Duration

    for _, metadata := range r.toolMetadata {
        categoryCount[metadata.Category]++
        if metadata.Metrics != nil {
            totalCalls += metadata.Metrics.CallCount
            totalErrors += metadata.Metrics.ErrorCount
            totalTime += metadata.Metrics.AverageTime
        }
    }

    stats["categories"] = categoryCount
    stats["total_calls"] = totalCalls
    stats["total_errors"] = totalErrors
    if len(r.toolMetadata) > 0 {
        stats["average_time"] = totalTime / time.Duration(len(r.toolMetadata))
    }

    return stats
}
```

## 3.4 回调管理子系统

### 3.4.1 CallbackManager 类设计

```go
type CallbackManager struct {
    config         *UnifyDialogConfig
    handlers       []callbacks.Handler
    eventCollector *EventCollector
    metricsStore   *MetricsStore
    logWriter      *LogWriter
    mutex          sync.RWMutex
}

type EventCollector struct {
    events    []CallbackEvent
    maxEvents int
    mutex     sync.RWMutex
}

type CallbackEvent struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    RunInfo   *callbacks.RunInfo     `json:"run_info"`
    Input     interface{}            `json:"input,omitempty"`
    Output    interface{}            `json:"output,omitempty"`
    Error     string                 `json:"error,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
    Duration  time.Duration          `json:"duration,omitempty"`
    Metadata  map[string]interface{} `json:"metadata"`
}

type MetricsStore struct {
    counters   map[string]int64
    timers     map[string][]time.Duration
    errors     map[string]int64
    lastUpdate time.Time
    mutex      sync.RWMutex
}

type LogWriter struct {
    writers []io.Writer
    level   LogLevel
    mutex   sync.Mutex
}

type LogLevel int

const (
    LogLevelDebug LogLevel = iota
    LogLevelInfo
    LogLevelWarn
    LogLevelError
)
```

**核心方法实现**:

```go
// Initialize 初始化回调管理器
func (cm *CallbackManager) Initialize(ctx context.Context) error {
    if !cm.config.System.EnableCallbacks {
        log.Println("Callbacks are disabled")
        return nil
    }

    // 初始化事件收集器
    cm.eventCollector = &EventCollector{
        events:    make([]CallbackEvent, 0),
        maxEvents: 1000, // 最多保存1000个事件
    }

    // 初始化指标存储
    cm.metricsStore = &MetricsStore{
        counters:   make(map[string]int64),
        timers:     make(map[string][]time.Duration),
        errors:     make(map[string]int64),
        lastUpdate: time.Now(),
    }

    // 初始化日志写入器
    cm.logWriter = &LogWriter{
        level: cm.parseLogLevel(cm.config.System.LogLevel),
    }

    // 添加标准输出写入器
    cm.logWriter.AddWriter(os.Stdout)

    // 创建内置处理器
    handlers := []callbacks.Handler{}

    // 调试处理器
    if cm.config.System.Debug {
        handlers = append(handlers, cm.createDebugHandler())
    }

    // 日志处理器
    handlers = append(handlers, cm.createLoggingHandler())

    // 性能监控处理器
    handlers = append(handlers, cm.createPerformanceHandler())

    // 指标收集处理器
    handlers = append(handlers, cm.createMetricsHandler())

    // 外部集成处理器
    if cm.config.Callbacks.Cozeloop.Enabled {
        if handler, err := cm.createCozeloopHandler(); err == nil {
            handlers = append(handlers, handler)
        } else {
            log.Printf("Failed to create Cozeloop handler: %v", err)
        }
    }

    cm.handlers = handlers

    log.Printf("Callback manager initialized with %d handlers", len(cm.handlers))
    return nil
}

// createDebugHandler 创建调试处理器
func (cm *CallbackManager) createDebugHandler() callbacks.Handler {
    return &DebugCallbackHandler{
        eventCollector: cm.eventCollector,
        logWriter:      cm.logWriter,
    }
}

// createPerformanceHandler 创建性能监控处理器
func (cm *CallbackManager) createPerformanceHandler() callbacks.Handler {
    return &PerformanceCallbackHandler{
        metricsStore:   cm.metricsStore,
        startTimes:     make(map[string]time.Time),
        eventCollector: cm.eventCollector,
    }
}

// DebugCallbackHandler 增强的调试处理器
type DebugCallbackHandler struct {
    callbacks.SimpleCallbackHandler
    eventCollector *EventCollector
    logWriter      *LogWriter
    callDepth      int32
    mutex          sync.Mutex
}

func (h *DebugCallbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
    h.mutex.Lock()
    depth := atomic.AddInt32(&h.callDepth, 1)
    h.mutex.Unlock()

    indent := strings.Repeat("  ", int(depth-1))

    h.logWriter.WriteDebug(fmt.Sprintf("%s[START] %s (%s) - RunID: %s",
        indent, info.Type, info.Name, info.RunID))

    // 记录事件
    event := CallbackEvent{
        ID:        generateEventID(),
        Type:      "start",
        RunInfo:   info,
        Input:     input,
        Timestamp: time.Now(),
        Metadata: map[string]interface{}{
            "depth": depth,
            "context_values": extractContextValues(ctx),
        },
    }

    h.eventCollector.AddEvent(event)

    // 在context中保存开始时间
    return context.WithValue(ctx, fmt.Sprintf("start_time_%s", info.RunID), time.Now())
}

func (h *DebugCallbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
    depth := atomic.AddInt32(&h.callDepth, -1)
    indent := strings.Repeat("  ", int(depth))

    // 计算执行时间
    var duration time.Duration
    if startTime, ok := ctx.Value(fmt.Sprintf("start_time_%s", info.RunID)).(time.Time); ok {
        duration = time.Since(startTime)
    }

    h.logWriter.WriteDebug(fmt.Sprintf("%s[END] %s (%s) - Duration: %v",
        indent, info.Type, info.Name, duration))

    // 记录事件
    event := CallbackEvent{
        ID:        generateEventID(),
        Type:      "end",
        RunInfo:   info,
        Output:    output,
        Timestamp: time.Now(),
        Duration:  duration,
        Metadata: map[string]interface{}{
            "depth": depth + 1,
        },
    }

    h.eventCollector.AddEvent(event)

    return ctx
}

func (h *DebugCallbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
    depth := atomic.LoadInt32(&h.callDepth)
    indent := strings.Repeat("  ", int(depth))

    h.logWriter.WriteError(fmt.Sprintf("%s[ERROR] %s (%s) - Error: %v",
        indent, info.Type, info.Name, err))

    // 记录事件
    event := CallbackEvent{
        ID:        generateEventID(),
        Type:      "error",
        RunInfo:   info,
        Error:     err.Error(),
        Timestamp: time.Now(),
        Metadata: map[string]interface{}{
            "depth": depth,
            "error_type": fmt.Sprintf("%T", err),
        },
    }

    h.eventCollector.AddEvent(event)

    return ctx
}

// PerformanceCallbackHandler 增强的性能监控处理器
type PerformanceCallbackHandler struct {
    callbacks.SimpleCallbackHandler
    metricsStore   *MetricsStore
    eventCollector *EventCollector
    startTimes     map[string]time.Time
    mutex          sync.RWMutex
}

func (h *PerformanceCallbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
    h.mutex.Lock()
    h.startTimes[info.RunID] = time.Now()
    h.mutex.Unlock()

    // 增加调用计数
    h.metricsStore.IncrementCounter(fmt.Sprintf("%s.calls", info.Type))

    return ctx
}

func (h *PerformanceCallbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
    h.mutex.Lock()
    startTime, exists := h.startTimes[info.RunID]
    if exists {
        delete(h.startTimes, info.RunID)
    }
    h.mutex.Unlock()

    if exists {
        duration := time.Since(startTime)

        // 记录执行时间
        h.metricsStore.RecordTiming(fmt.Sprintf("%s.duration", info.Type), duration)

        // 增加成功计数
        h.metricsStore.IncrementCounter(fmt.Sprintf("%s.success", info.Type))

        // 如果执行时间过长，记录慢查询
        if duration > 5*time.Second {
            log.Printf("[SLOW] %s (%s) took %v", info.Type, info.Name, duration)
            h.metricsStore.IncrementCounter("slow_operations")
        }
    }

    return ctx
}

func (h *PerformanceCallbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
    h.mutex.Lock()
    if _, exists := h.startTimes[info.RunID]; exists {
        delete(h.startTimes, info.RunID)
    }
    h.mutex.Unlock()

    // 增加错误计数
    h.metricsStore.IncrementCounter(fmt.Sprintf("%s.errors", info.Type))
    h.metricsStore.IncrementError(err.Error())

    return ctx
}

// MetricsStore 指标存储实现
func (ms *MetricsStore) IncrementCounter(name string) {
    ms.mutex.Lock()
    defer ms.mutex.Unlock()
    ms.counters[name]++
    ms.lastUpdate = time.Now()
}

func (ms *MetricsStore) RecordTiming(name string, duration time.Duration) {
    ms.mutex.Lock()
    defer ms.mutex.Unlock()
    ms.timers[name] = append(ms.timers[name], duration)

    // 保持最近100个时间记录
    if len(ms.timers[name]) > 100 {
        ms.timers[name] = ms.timers[name][1:]
    }
    ms.lastUpdate = time.Now()
}

func (ms *MetricsStore) IncrementError(errorType string) {
    ms.mutex.Lock()
    defer ms.mutex.Unlock()
    ms.errors[errorType]++
    ms.lastUpdate = time.Now()
}

func (ms *MetricsStore) GetMetrics() map[string]interface{} {
    ms.mutex.RLock()
    defer ms.mutex.RUnlock()

    result := make(map[string]interface{})
    result["counters"] = make(map[string]int64)
    result["timers"] = make(map[string]interface{})
    result["errors"] = make(map[string]int64)
    result["last_update"] = ms.lastUpdate

    // 复制计数器
    for k, v := range ms.counters {
        result["counters"].(map[string]int64)[k] = v
    }

    // 复制错误计数
    for k, v := range ms.errors {
        result["errors"].(map[string]int64)[k] = v
    }

    // 计算定时器统计
    timers := result["timers"].(map[string]interface{})
    for name, durations := range ms.timers {
        if len(durations) > 0 {
            var total time.Duration
            min := durations[0]
            max := durations[0]

            for _, d := range durations {
                total += d
                if d < min {
                    min = d
                }
                if d > max {
                    max = d
                }
            }

            timers[name] = map[string]interface{}{
                "count":   len(durations),
                "average": total / time.Duration(len(durations)),
                "min":     min,
                "max":     max,
                "total":   total,
            }
        }
    }

    return result
}

// GetHandlers 获取所有处理器
func (cm *CallbackManager) GetHandlers() []callbacks.Handler {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()

    handlers := make([]callbacks.Handler, len(cm.handlers))
    copy(handlers, cm.handlers)
    return handlers
}

// GetMetrics 获取所有指标
func (cm *CallbackManager) GetMetrics() map[string]interface{} {
    metrics := cm.metricsStore.GetMetrics()
    metrics["events_count"] = cm.eventCollector.GetEventCount()
    return metrics
}

// GetEvents 获取最近的事件
func (cm *CallbackManager) GetEvents(limit int) []CallbackEvent {
    return cm.eventCollector.GetRecentEvents(limit)
}
```