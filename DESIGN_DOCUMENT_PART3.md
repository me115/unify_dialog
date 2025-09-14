# UnifyDialog 设计方案 - 第三部分

## 3.5 状态管理子系统

### 3.5.1 StateManager 类设计

```go
type StateManager struct {
    currentState    *SystemState
    stateHistory    []*StateSnapshot
    maxHistorySize  int
    stateValidator  StateValidator
    stateObservers  []StateObserver
    persistManager  *StatePersistManager
    mutex          sync.RWMutex
}

type SystemState struct {
    // 核心状态字段
    Plan             *Plan                  `json:"plan"`
    ExecutionLog     []ExecutionEntry       `json:"execution_log"`
    CurrentStep      int                    `json:"current_step"`
    Feedback         []string               `json:"feedback"`
    Context          map[string]interface{} `json:"context"`

    // 执行控制字段
    MaxIterations    int                    `json:"max_iterations"`
    CurrentIteration int                    `json:"current_iteration"`
    Status           ExecutionStatus        `json:"status"`

    // 元数据字段
    SessionID        string                 `json:"session_id"`
    UserID          string                 `json:"user_id"`
    StartTime       time.Time              `json:"start_time"`
    LastModified    time.Time              `json:"last_modified"`

    // 性能指标
    Metrics         *ExecutionMetrics      `json:"metrics"`

    // 扩展字段
    Extensions      map[string]interface{} `json:"extensions"`
}

type Plan struct {
    ID          string     `json:"id"`
    Goal        string     `json:"goal"`
    Steps       []PlanStep `json:"steps"`
    Priority    Priority   `json:"priority"`
    Constraints []string   `json:"constraints"`
    Dependencies []string  `json:"dependencies"`
    EstimatedTime time.Duration `json:"estimated_time"`
    CreatedBy   string     `json:"created_by"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
    Version     int        `json:"version"`
}

type PlanStep struct {
    ID           string                 `json:"id"`
    Description  string                 `json:"description"`
    Tool         string                 `json:"tool,omitempty"`
    Parameters   map[string]interface{} `json:"parameters,omitempty"`
    Dependencies []string               `json:"dependencies,omitempty"`
    Status       StepStatus             `json:"status"`
    Result       *StepResult            `json:"result,omitempty"`
    StartTime    time.Time              `json:"start_time,omitempty"`
    EndTime      time.Time              `json:"end_time,omitempty"`
    Retries      int                    `json:"retries"`
    MaxRetries   int                    `json:"max_retries"`
}

type StepResult struct {
    Success     bool                   `json:"success"`
    Data        interface{}            `json:"data"`
    Error       string                 `json:"error,omitempty"`
    Metadata    map[string]interface{} `json:"metadata"`
    Timestamp   time.Time              `json:"timestamp"`
}

type ExecutionEntry struct {
    ID        string                 `json:"id"`
    StepID    string                 `json:"step_id"`
    Action    string                 `json:"action"`
    Status    string                 `json:"status"`
    Result    interface{}            `json:"result"`
    Error     string                 `json:"error,omitempty"`
    Duration  time.Duration          `json:"duration"`
    Timestamp time.Time              `json:"timestamp"`
    Agent     string                 `json:"agent"`
    Metadata  map[string]interface{} `json:"metadata"`
}

type ExecutionStatus string

const (
    StatusPending    ExecutionStatus = "pending"
    StatusPlanning   ExecutionStatus = "planning"
    StatusExecuting  ExecutionStatus = "executing"
    StatusWaiting    ExecutionStatus = "waiting"
    StatusCompleted  ExecutionStatus = "completed"
    StatusFailed     ExecutionStatus = "failed"
    StatusCancelled  ExecutionStatus = "cancelled"
)

type Priority string

const (
    PriorityLow      Priority = "low"
    PriorityMedium   Priority = "medium"
    PriorityHigh     Priority = "high"
    PriorityCritical Priority = "critical"
)

type StepStatus string

const (
    StepStatusPending    StepStatus = "pending"
    StepStatusRunning    StepStatus = "running"
    StepStatusCompleted  StepStatus = "completed"
    StepStatusFailed     StepStatus = "failed"
    StepStatusSkipped    StepStatus = "skipped"
)
```

**核心方法实现**:

```go
// NewStateManager 创建状态管理器
func NewStateManager(config *StateManagerConfig) *StateManager {
    return &StateManager{
        currentState:   NewEmptySystemState(),
        stateHistory:   make([]*StateSnapshot, 0),
        maxHistorySize: config.MaxHistorySize,
        stateValidator: NewDefaultStateValidator(),
        stateObservers: make([]StateObserver, 0),
        persistManager: NewStatePersistManager(config.PersistConfig),
    }
}

// InitializeState 初始化系统状态
func (sm *StateManager) InitializeState(sessionID, userID string) error {
    sm.mutex.Lock()
    defer sm.mutex.Unlock()

    // 创建新状态
    state := &SystemState{
        SessionID:        sessionID,
        UserID:          userID,
        Status:          StatusPending,
        Context:         make(map[string]interface{}),
        ExecutionLog:    make([]ExecutionEntry, 0),
        Feedback:        make([]string, 0),
        MaxIterations:   5,
        CurrentIteration: 0,
        CurrentStep:     0,
        StartTime:       time.Now(),
        LastModified:    time.Now(),
        Extensions:      make(map[string]interface{}),
        Metrics:         NewExecutionMetrics(),
    }

    // 验证状态
    if err := sm.stateValidator.Validate(state); err != nil {
        return fmt.Errorf("invalid initial state: %w", err)
    }

    // 保存快照
    sm.createSnapshot("initialize")

    // 设置当前状态
    sm.currentState = state

    // 通知观察者
    sm.notifyObservers(StateEventInitialized, state, nil)

    // 持久化
    if err := sm.persistManager.SaveState(state); err != nil {
        log.Printf("Failed to persist initial state: %v", err)
    }

    log.Printf("Initialized state for session: %s", sessionID)
    return nil
}

// UpdateState 更新系统状态
func (sm *StateManager) UpdateState(updater StateUpdater) error {
    sm.mutex.Lock()
    defer sm.mutex.Unlock()

    // 创建状态副本
    oldState := sm.deepCopyState(sm.currentState)

    // 应用更新
    newState, err := updater.Update(sm.currentState)
    if err != nil {
        return fmt.Errorf("state update failed: %w", err)
    }

    // 验证新状态
    if err := sm.stateValidator.Validate(newState); err != nil {
        return fmt.Errorf("invalid updated state: %w", err)
    }

    // 更新时间戳
    newState.LastModified = time.Now()

    // 保存快照
    sm.createSnapshot(fmt.Sprintf("update_%d", len(sm.stateHistory)))

    // 应用新状态
    sm.currentState = newState

    // 通知观察者
    sm.notifyObservers(StateEventUpdated, newState, oldState)

    // 持久化
    if err := sm.persistManager.SaveState(newState); err != nil {
        log.Printf("Failed to persist updated state: %v", err)
    }

    return nil
}

// GetState 获取当前状态
func (sm *StateManager) GetState() *SystemState {
    sm.mutex.RLock()
    defer sm.mutex.RUnlock()
    return sm.deepCopyState(sm.currentState)
}

// AddExecutionEntry 添加执行日志条目
func (sm *StateManager) AddExecutionEntry(entry ExecutionEntry) error {
    return sm.UpdateState(&ExecutionLogUpdater{Entry: entry})
}

// UpdatePlan 更新执行计划
func (sm *StateManager) UpdatePlan(plan *Plan) error {
    return sm.UpdateState(&PlanUpdater{Plan: plan})
}

// UpdateStepStatus 更新步骤状态
func (sm *StateManager) UpdateStepStatus(stepID string, status StepStatus, result *StepResult) error {
    return sm.UpdateState(&StepStatusUpdater{
        StepID: stepID,
        Status: status,
        Result: result,
    })
}

// StateUpdater 接口定义
type StateUpdater interface {
    Update(*SystemState) (*SystemState, error)
}

// ExecutionLogUpdater 执行日志更新器
type ExecutionLogUpdater struct {
    Entry ExecutionEntry
}

func (u *ExecutionLogUpdater) Update(state *SystemState) (*SystemState, error) {
    newState := *state
    u.Entry.Timestamp = time.Now()
    u.Entry.ID = generateEntryID()

    newState.ExecutionLog = append(newState.ExecutionLog, u.Entry)

    // 限制日志大小
    if len(newState.ExecutionLog) > 1000 {
        newState.ExecutionLog = newState.ExecutionLog[1:]
    }

    return &newState, nil
}

// PlanUpdater 计划更新器
type PlanUpdater struct {
    Plan *Plan
}

func (u *PlanUpdater) Update(state *SystemState) (*SystemState, error) {
    newState := *state

    if u.Plan != nil {
        u.Plan.UpdatedAt = time.Now()
        if u.Plan.Version == 0 {
            u.Plan.Version = 1
        } else {
            u.Plan.Version++
        }
    }

    newState.Plan = u.Plan
    newState.Status = StatusPlanning

    return &newState, nil
}

// StepStatusUpdater 步骤状态更新器
type StepStatusUpdater struct {
    StepID string
    Status StepStatus
    Result *StepResult
}

func (u *StepStatusUpdater) Update(state *SystemState) (*SystemState, error) {
    newState := *state

    if newState.Plan == nil {
        return &newState, fmt.Errorf("no plan available to update step")
    }

    // 查找并更新步骤
    for i, step := range newState.Plan.Steps {
        if step.ID == u.StepID {
            newState.Plan.Steps[i].Status = u.Status

            if u.Status == StepStatusRunning {
                newState.Plan.Steps[i].StartTime = time.Now()
            } else if u.Status == StepStatusCompleted || u.Status == StepStatusFailed {
                newState.Plan.Steps[i].EndTime = time.Now()
            }

            if u.Result != nil {
                u.Result.Timestamp = time.Now()
                newState.Plan.Steps[i].Result = u.Result
            }

            break
        }
    }

    // 更新整体状态
    if u.Status == StepStatusRunning {
        newState.Status = StatusExecuting
    }

    return &newState, nil
}
```

### 3.5.2 多智能体编排器 (MultiAgentOrchestrator)

```go
type MultiAgentOrchestrator struct {
    // 配置和依赖
    config          *UnifyDialogConfig
    modelFactory    *ModelFactory
    toolRegistry    *ToolRegistry
    callbackManager *CallbackManager
    stateManager    *StateManager

    // 图执行引擎
    compiledGraph   compose.Runnable[*AgentInput, *AgentOutput]
    graphBuilder    *GraphBuilder

    // 智能体管理
    agents          map[string]*Agent
    agentCoordinator *AgentCoordinator

    // 执行控制
    executor        *GraphExecutor
    interruptManager *InterruptManager
    checkpointManager *CheckpointManager

    // 监控和调试
    debugger        *GraphDebugger
    profiler        *GraphProfiler

    // 并发控制
    semaphore       chan struct{}
    activeRequests  sync.Map

    mutex           sync.RWMutex
}

type Agent struct {
    ID           string
    Name         string
    Role         string
    Description  string
    Model        model.ChatModel
    Tools        []tool.BaseTool
    Prompt       *prompt.ChatTemplate
    Config       *AgentConfig
    Metrics      *AgentMetrics
    State        *AgentState
}

type AgentConfig struct {
    MaxRetries      int                    `json:"max_retries"`
    Timeout         time.Duration          `json:"timeout"`
    Temperature     float32                `json:"temperature"`
    MaxTokens       int                    `json:"max_tokens"`
    SystemPrompt    string                 `json:"system_prompt"`
    Tools           []string               `json:"tools"`
    Capabilities    []string               `json:"capabilities"`
    Constraints     []string               `json:"constraints"`
    CustomSettings  map[string]interface{} `json:"custom_settings"`
}

type AgentMetrics struct {
    CallCount       int64         `json:"call_count"`
    SuccessCount    int64         `json:"success_count"`
    ErrorCount      int64         `json:"error_count"`
    TotalTime       time.Duration `json:"total_time"`
    AverageTime     time.Duration `json:"average_time"`
    LastUsed        time.Time     `json:"last_used"`
    TokensUsed      int64         `json:"tokens_used"`
}

type AgentState struct {
    Status          AgentStatus            `json:"status"`
    CurrentTask     string                 `json:"current_task"`
    Context         map[string]interface{} `json:"context"`
    Memory          []string               `json:"memory"`
    LastOutput      string                 `json:"last_output"`
    ErrorHistory    []string               `json:"error_history"`
}

type AgentStatus string

const (
    AgentStatusIdle      AgentStatus = "idle"
    AgentStatusWorking   AgentStatus = "working"
    AgentStatusWaiting   AgentStatus = "waiting"
    AgentStatusError     AgentStatus = "error"
    AgentStatusDisabled  AgentStatus = "disabled"
)

type GraphBuilder struct {
    graph           *compose.Graph[*AgentInput, *AgentOutput]
    nodeDefinitions map[string]*NodeDefinition
    edgeDefinitions []EdgeDefinition
    stateHandlers   map[string]compose.StateHandler
    branchLogic     map[string]BranchFunction
    middleware      []GraphMiddleware
}

type NodeDefinition struct {
    Name        string
    Type        NodeType
    Agent       *Agent
    Handler     interface{}
    Config      map[string]interface{}
    Timeout     time.Duration
    Retries     int
}

type EdgeDefinition struct {
    From        string
    To          string
    Condition   string
    Weight      float64
    Metadata    map[string]interface{}
}

type NodeType string

const (
    NodeTypeChatModel    NodeType = "chat_model"
    NodeTypeChatTemplate NodeType = "chat_template"
    NodeTypeLambda       NodeType = "lambda"
    NodeTypeTools        NodeType = "tools"
    NodeTypeBranch       NodeType = "branch"
    NodeTypeState        NodeType = "state"
)

type BranchFunction func(context.Context, interface{}) (string, error)
type GraphMiddleware func(context.Context, *AgentInput) (*AgentInput, error)
```

**核心实现方法**:

```go
// NewMultiAgentOrchestrator 创建多智能体编排器
func NewMultiAgentOrchestrator(config *UnifyDialogConfig) (*MultiAgentOrchestrator, error) {
    // 验证配置
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    // 创建并发控制信号量
    semaphore := make(chan struct{}, config.System.MaxConcurrency)
    for i := 0; i < config.System.MaxConcurrency; i++ {
        semaphore <- struct{}{}
    }

    orchestrator := &MultiAgentOrchestrator{
        config:            config,
        modelFactory:      NewModelFactory(config),
        callbackManager:   NewCallbackManager(config),
        stateManager:      NewStateManager(&StateManagerConfig{}),
        agents:           make(map[string]*Agent),
        graphBuilder:     NewGraphBuilder(),
        semaphore:        semaphore,
        debugger:         NewGraphDebugger(config.System.Debug),
        profiler:         NewGraphProfiler(),
        interruptManager: NewInterruptManager(),
    }

    return orchestrator, nil
}

// Initialize 初始化多智能体编排器
func (o *MultiAgentOrchestrator) Initialize(ctx context.Context) error {
    log.Println("Initializing Multi-Agent Orchestrator...")

    // 1. 初始化工具注册表
    mcpManager := NewMCPToolManager(o.config)
    if err := mcpManager.Initialize(ctx); err != nil {
        return fmt.Errorf("failed to initialize MCP tools: %w", err)
    }
    o.toolRegistry = NewToolRegistry(mcpManager)

    // 2. 初始化回调管理器
    if err := o.callbackManager.Initialize(ctx); err != nil {
        return fmt.Errorf("failed to initialize callbacks: %w", err)
    }

    // 3. 创建智能体
    if err := o.createAgents(ctx); err != nil {
        return fmt.Errorf("failed to create agents: %w", err)
    }

    // 4. 构建编排图
    if err := o.buildOfficialPatternGraph(ctx); err != nil {
        return fmt.Errorf("failed to build orchestration graph: %w", err)
    }

    // 5. 初始化检查点管理器
    o.checkpointManager = NewCheckpointManager(o.stateManager)

    log.Printf("Multi-Agent Orchestrator initialized successfully with %d agents", len(o.agents))
    return nil
}

// createAgents 创建智能体实例
func (o *MultiAgentOrchestrator) createAgents(ctx context.Context) error {
    // 创建规划智能体
    plannerAgent, err := o.createPlannerAgent(ctx)
    if err != nil {
        return fmt.Errorf("failed to create planner agent: %w", err)
    }
    o.agents["planner"] = plannerAgent

    // 创建执行智能体
    executorAgent, err := o.createExecutorAgent(ctx)
    if err != nil {
        return fmt.Errorf("failed to create executor agent: %w", err)
    }
    o.agents["executor"] = executorAgent

    // 创建监督智能体
    supervisorAgent, err := o.createSupervisorAgent(ctx)
    if err != nil {
        return fmt.Errorf("failed to create supervisor agent: %w", err)
    }
    o.agents["supervisor"] = supervisorAgent

    return nil
}

// createPlannerAgent 创建规划智能体
func (o *MultiAgentOrchestrator) createPlannerAgent(ctx context.Context) (*Agent, error) {
    model, err := o.modelFactory.CreatePlannerModel(ctx)
    if err != nil {
        return nil, err
    }

    systemPrompt := `你是一个专业的任务规划智能体。你的职责是：

1. 分析用户查询，理解核心需求和目标
2. 制定详细的执行计划，包括具体的步骤和工具调用
3. 识别潜在的风险和约束条件
4. 估算执行时间和资源需求
5. 提供清晰的JSON格式输出

输出格式要求：
{
  "goal": "明确的目标描述",
  "steps": [
    {
      "id": "step_1",
      "description": "步骤描述",
      "tool": "工具名称(可选)",
      "parameters": {"key": "value"},
      "dependencies": ["前置步骤ID"],
      "estimated_time": "预估时间"
    }
  ],
  "constraints": ["约束条件"],
  "risks": ["潜在风险"],
  "success_criteria": ["成功标准"]
}`

    prompt := prompt.FromMessages(
        schema.SystemMessage(systemPrompt),
        schema.UserMessage("Query: {{.query}}\nContext: {{.context}}"),
    )

    agent := &Agent{
        ID:          "planner",
        Name:        "Planning Agent",
        Role:        "planner",
        Description: "Responsible for task analysis and execution planning",
        Model:       model,
        Prompt:      prompt,
        Config: &AgentConfig{
            MaxRetries:   3,
            Timeout:      30 * time.Second,
            Temperature:  0.3, // 较低温度保证计划的确定性
            MaxTokens:    2000,
            SystemPrompt: systemPrompt,
        },
        Metrics: &AgentMetrics{},
        State: &AgentState{
            Status:  AgentStatusIdle,
            Context: make(map[string]interface{}),
            Memory:  make([]string, 0, 10),
        },
    }

    return agent, nil
}

// createExecutorAgent 创建执行智能体
func (o *MultiAgentOrchestrator) createExecutorAgent(ctx context.Context) (*Agent, error) {
    model, err := o.modelFactory.CreateExecutorModel(ctx)
    if err != nil {
        return nil, err
    }

    // 获取可用工具
    tools := o.toolRegistry.GetAllTools()

    systemPrompt := `你是一个专业的任务执行智能体。你的职责是：

1. 按照既定计划逐步执行任务
2. 合理使用可用的工具和资源
3. 处理执行过程中的异常情况
4. 记录执行结果和关键信息
5. 与其他智能体协作完成复杂任务

可用工具：
{{range .tools}}
- {{.name}}: {{.description}}
{{end}}

执行原则：
- 严格按照计划顺序执行
- 每个步骤完成后记录结果
- 遇到错误时尝试恢复或报告
- 保持执行状态的及时更新`

    prompt := prompt.FromMessages(
        schema.SystemMessage(systemPrompt),
        schema.UserMessage("Plan: {{.plan}}\nCurrent Step: {{.current_step}}"),
    )

    agent := &Agent{
        ID:          "executor",
        Name:        "Execution Agent",
        Role:        "executor",
        Description: "Responsible for task execution and tool invocation",
        Model:       model,
        Tools:       tools,
        Prompt:      prompt,
        Config: &AgentConfig{
            MaxRetries:   5,
            Timeout:      60 * time.Second,
            Temperature:  0.1, // 极低温度保证执行的精确性
            MaxTokens:    1500,
            SystemPrompt: systemPrompt,
            Tools:        o.getToolNames(tools),
        },
        Metrics: &AgentMetrics{},
        State: &AgentState{
            Status:  AgentStatusIdle,
            Context: make(map[string]interface{}),
            Memory:  make([]string, 0, 20),
        },
    }

    return agent, nil
}

// createSupervisorAgent 创建监督智能体
func (o *MultiAgentOrchestrator) createSupervisorAgent(ctx context.Context) (*Agent, error) {
    model, err := o.modelFactory.CreateSupervisorModel(ctx)
    if err != nil {
        return nil, err
    }

    systemPrompt := `你是一个专业的质量监督智能体。你的职责是：

1. 评估任务执行结果的质量和完整性
2. 识别执行过程中的问题和改进空间
3. 决定是否需要重新执行或调整计划
4. 提供具体的反馈和建议
5. 确保最终输出满足用户需求

评估标准：
- 任务完成度
- 结果准确性
- 执行效率
- 用户满意度
- 安全性和合规性

输出格式：
{
  "evaluation": {
    "score": 0.85,
    "completeness": "完成度评估",
    "accuracy": "准确性评估",
    "efficiency": "效率评估"
  },
  "action": "complete|retry|escalate",
  "feedback": ["具体反馈"],
  "improvements": ["改进建议"]
}`

    prompt := prompt.FromMessages(
        schema.SystemMessage(systemPrompt),
        schema.UserMessage(`Goal: {{.goal}}
Execution Results: {{.results}}
Execution Log: {{.execution_log}}
Performance Metrics: {{.metrics}}`),
    )

    agent := &Agent{
        ID:          "supervisor",
        Name:        "Supervision Agent",
        Role:        "supervisor",
        Description: "Responsible for quality control and result evaluation",
        Model:       model,
        Prompt:      prompt,
        Config: &AgentConfig{
            MaxRetries:   2,
            Timeout:      45 * time.Second,
            Temperature:  0.5, // 中等温度平衡客观性和创造性
            MaxTokens:    1800,
            SystemPrompt: systemPrompt,
        },
        Metrics: &AgentMetrics{},
        State: &AgentState{
            Status:  AgentStatusIdle,
            Context: make(map[string]interface{}),
            Memory:  make([]string, 0, 15),
        },
    }

    return agent, nil
}

// buildOfficialPatternGraph 构建官方多智能体编排模式
func (o *MultiAgentOrchestrator) buildOfficialPatternGraph(ctx context.Context) error {
    g := compose.NewGraph[*AgentInput, *AgentOutput]()

    // 状态处理器 - 管理共享状态
    stateHandler := compose.StatePreHandler[*SystemState](
        func(ctx context.Context, state *SystemState, input compose.GraphState) (*SystemState, error) {
            if state == nil {
                // 初始化新状态
                sessionID := generateSessionID()
                o.stateManager.InitializeState(sessionID, "system")
                state = o.stateManager.GetState()
            }
            return state, nil
        },
    )

    // 规划节点 - 智能任务规划
    err := g.AddLambdaNode("planner", compose.InvokableLambda(
        func(ctx context.Context, input *AgentInput) (*PlannerOutput, error) {
            return o.executePlannerAgent(ctx, input)
        },
    ))
    if err != nil {
        return fmt.Errorf("failed to add planner node: %w", err)
    }

    // 执行节点 - 计划执行
    err = g.AddLambdaNode("executor", compose.InvokableLambda(
        func(ctx context.Context, input *PlannerOutput) (*ExecutorOutput, error) {
            return o.executeExecutorAgent(ctx, input)
        },
    ))
    if err != nil {
        return fmt.Errorf("failed to add executor node: %w", err)
    }

    // 监督节点 - 质量控制
    err = g.AddLambdaNode("supervisor", compose.InvokableLambda(
        func(ctx context.Context, input *ExecutorOutput) (*SupervisorOutput, error) {
            return o.executeSupervisorAgent(ctx, input)
        },
    ))
    if err != nil {
        return fmt.Errorf("failed to add supervisor node: %w", err)
    }

    // 最终输出节点
    err = g.AddLambdaNode("final_output", compose.InvokableLambda(
        func(ctx context.Context, input *SupervisorOutput) (*AgentOutput, error) {
            return o.prepareFinalOutput(ctx, input)
        },
    ))
    if err != nil {
        return fmt.Errorf("failed to add final output node: %w", err)
    }

    // 构建边连接
    g.AddEdge(compose.START, "planner")
    g.AddEdge("planner", "executor")
    g.AddEdge("executor", "supervisor")

    // 条件分支 - 基于监督结果决定下一步
    g.AddBranch("supervisor", func(ctx context.Context, output *SupervisorOutput) (string, error) {
        // 获取当前状态
        state := o.stateManager.GetState()

        switch output.Action {
        case "complete":
            return "final_output", nil
        case "retry":
            // 检查重试次数
            if state.CurrentIteration < state.MaxIterations {
                // 增加迭代次数
                o.stateManager.UpdateState(&IterationUpdater{})
                return "planner", nil // 重新规划
            }
            return "final_output", nil // 超过最大重试次数
        case "escalate":
            // 人工干预 - 暂时返回到最终输出
            return "final_output", nil
        default:
            return "final_output", nil
        }
    })

    g.AddEdge("final_output", compose.END)

    // 编译选项
    options := []compose.GraphCompileOption{
        compose.WithStatePreHandler(stateHandler),
    }

    // 添加回调处理器
    if handlers := o.callbackManager.GetHandlers(); len(handlers) > 0 {
        for _, handler := range handlers {
            options = append(options, compose.WithGraphCallbacks(handler))
        }
    }

    // 添加中断支持
    if o.config.System.Debug {
        options = append(options, compose.WithInterruptSupport())
    }

    // 编译图
    compiledGraph, err := g.Compile(ctx, options...)
    if err != nil {
        return fmt.Errorf("failed to compile orchestration graph: %w", err)
    }

    o.compiledGraph = compiledGraph
    log.Println("Multi-agent orchestration graph compiled successfully")

    return nil
}
```