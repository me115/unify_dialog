# 统一对话智能体 (Unified Dialog Agent)

基于 EINO 框架实现的统一对话智能体，采用 Planner-Supervisor-Executor 混合架构，支持复杂的多步骤任务执行、工具调用和智能监督。

## 架构概述

本项目实现了 README.md 中描述的混合模式架构，包含三个核心组件：

### 1. Planner (规划师)
- **功能**: 将用户请求分解为结构化的执行计划
- **特点**:
  - 支持复杂依赖关系的步骤规划
  - 动态参数引用 (`$ref` 语法)
  - 并行和串行步骤组合
  - 错误处理策略配置

### 2. Executor (执行器)
- **功能**: 按依赖顺序执行计划中的步骤
- **特点**:
  - 自动依赖解析和拓扑排序
  - 并行执行支持
  - 动态参数解析
  - 多种步骤类型支持（工具调用、数据处理、条件判断等）
  - 重试机制和错误处理

### 3. Supervisor (监督者)
- **功能**: 监控执行过程，在关键节点进行决策
- **特点**:
  - 智能错误分析和恢复
  - 多种干预策略（重试、跳过、重新规划等）
  - 基于阈值的触发机制
  - 上下文感知的决策制定

## 核心特性

### 🎯 动态参数解析
支持复杂的参数引用系统：

```json
{
  "parameters": {
    "user_data": {"$ref": "steps.step1.result"},
    "user_count": {
      "type": "step_result",
      "reference": "step1",
      "path": "users.length"
    },
    "template": "Found ${steps.step1.users.length} users"
  }
}
```

### 🔧 MCP 工具集成
支持多种 MCP (Model Context Protocol) 工具：
- **数据库工具**: 查询和操作数据库
- **API 工具**: HTTP API 请求
- **邮件工具**: 发送邮件通知
- **文件系统工具**: 文件读写操作
- **自定义工具**: 可扩展的工具接口

### ⚡ 并行执行
支持步骤的并行执行：
- 自动识别可并行的步骤
- 信号量控制并发数量
- 错误隔离和处理

### 🔄 智能监督
多层次的错误处理和恢复：
- **错误分类**: 瞬时、语义、永久、用户操作
- **恢复策略**: 重试、跳过、修改参数、重新规划
- **决策记录**: 完整的执行日志和决策轨迹

## 项目结构

```
unify_dialog/
├── main.go              # 主程序和演示代码
├── types.go             # 核心数据类型定义
├── agent.go             # 统一对话智能体实现
├── planner.go           # 规划师组件
├── executor.go          # 执行器组件
├── supervisor.go        # 监督者组件
├── parameter_resolver.go # 参数解析器
├── mcp_client.go        # MCP 客户端管理
├── go.mod              # Go 模块定义
└── README.md           # 项目说明文档
```

## 运行示例

### 1. 编译和运行

```bash
cd examples/unify_dialog
go mod tidy
go run .
```

### 2. 示例输出

```
🤖 Unified Dialog Agent Example
================================

🧪 Running Test Cases
---------------------

1. Data Analysis Request
   Description: Tests multi-step workflow with database queries, data processing, and notification
   Input: Please analyze our user engagement data from the last month and send me a summary report.
   Processing... ✅ SUCCESS (0.15s)
   Response: I have processed your request with the following results:

   Goal: Process user request: Please analyze our user engagement data...
   Completed 4 out of 4 steps successfully.

   Key Results:
   - Query user data: Completed successfully
   - Fetch additional data: Completed successfully
   - Process and combine data: Completed successfully
   - Send notification: Completed successfully

🔧 MCP Client Manager Demo
--------------------------
Registered MCP Clients: 4
- database: database
  Status: ✅ Healthy
  Description: Database query and manipulation tool
- api: api
  Status: ✅ Healthy
  Description: HTTP API request tool
...
```

## 执行计划示例

### 简单任务规划
```json
{
  "goal": "Query user data and send notification",
  "steps": [
    {
      "id": "step_1",
      "type": "tool_call",
      "tool": "database",
      "name": "Query user data",
      "parameters": {
        "operation": "query",
        "query": "SELECT * FROM users WHERE active = true"
      },
      "dependencies": [],
      "retry_policy": {
        "max_attempts": 3,
        "backoff_time": "1s",
        "exponential": true
      },
      "error_policy": "retry"
    },
    {
      "id": "step_2",
      "type": "tool_call",
      "tool": "email",
      "name": "Send notification",
      "parameters": {
        "operation": "send",
        "to": "admin@example.com",
        "subject": "User Data Report",
        "body": "Found ${steps.step_1.result.count} active users"
      },
      "dependencies": ["step_1"],
      "error_policy": "continue"
    }
  ]
}
```

### 复杂并行任务
```json
{
  "goal": "Parallel data collection and processing",
  "steps": [
    {
      "id": "fetch_users",
      "type": "tool_call",
      "tool": "database",
      "name": "Fetch user data",
      "parameters": {
        "operation": "query",
        "query": "SELECT * FROM users"
      },
      "dependencies": [],
      "parallel": true
    },
    {
      "id": "fetch_orders",
      "type": "tool_call",
      "tool": "api",
      "name": "Fetch order data",
      "parameters": {
        "operation": "request",
        "endpoint": "/api/orders",
        "method": "GET"
      },
      "dependencies": [],
      "parallel": true
    },
    {
      "id": "merge_data",
      "type": "data_processing",
      "name": "Merge user and order data",
      "parameters": {
        "operation": "merge",
        "users": {"$ref": "steps.fetch_users.result"},
        "orders": {"$ref": "steps.fetch_orders.result"}
      },
      "dependencies": ["fetch_users", "fetch_orders"]
    }
  ]
}
```

## 参数引用语法

### 1. 简单引用
```json
{"$ref": "steps.step_id.result"}
```

### 2. 结构化引用
```json
{
  "type": "step_result",
  "reference": "step_id",
  "path": "data.users[0].name",
  "default": "Unknown",
  "transform": "upper"
}
```

### 3. 模板变量
```json
{
  "message": "Hello ${steps.user_step.result.name}, you have ${steps.count_step.result} messages"
}
```

### 4. 支持的引用类型
- `step_result`: 引用其他步骤的结果
- `user_input`: 引用用户输入
- `data_store`: 引用数据存储
- `environment`: 引用环境变量
- `static`: 静态值

### 5. 路径表达式
- `field.subfield`: 对象字段访问
- `array[0]`: 数组索引访问
- `users[0].name`: 组合访问
- `data.items.length`: 属性访问

### 6. 转换函数
- `string`: 转换为字符串
- `json`: 转换为 JSON
- `upper/lower`: 大小写转换
- `length`: 获取长度
- `first/last`: 获取首/尾元素

## 监督者决策

### 决策类型
- `continue`: 继续执行
- `retry`: 重试步骤
- `skip`: 跳过步骤
- `abort`: 终止执行
- `modify_step`: 修改步骤参数
- `add_step`: 添加新步骤
- `replan`: 重新规划
- `ask_user`: 请求用户干预
- `complete`: 标记完成

### 错误分类
- `transient`: 瞬时错误（网络、超时）→ 通常重试
- `semantic`: 语义错误（参数错误）→ 通常修改步骤
- `permanent`: 永久错误（认证失败）→ 通常跳过或重新规划
- `user_action`: 需要用户操作 → 请求用户干预

## 扩展开发

### 添加新的 MCP 工具

1. 实现 `MCPClient` 接口：
```go
type CustomMCPClient struct {
    *BaseMCPClient
}

func (c *CustomMCPClient) Call(ctx context.Context, operation string, parameters map[string]interface{}) (interface{}, error) {
    // 实现具体的工具逻辑
    return result, nil
}
```

2. 注册到管理器：
```go
client := NewCustomMCPClient(config)
mcpManager.RegisterClient("custom_tool", client)
```

### 添加新的步骤类型

1. 在 `types.go` 中添加步骤类型：
```go
const StepTypeCustom StepType = "custom_operation"
```

2. 在 `executor.go` 中添加处理逻辑：
```go
case StepTypeCustom:
    return e.executeCustomOperation(ctx, step, resolvedParams, state)
```

### 自定义监督者策略

1. 继承 `SupervisorAgent` 或实现自定义决策逻辑
2. 重写 `ShouldIntervene` 和 `MakeDecision` 方法
3. 配置自定义触发条件和决策策略

## 配置选项

### 全局配置
```go
config := &AgentConfig{
    MaxIterations:  10,
    GlobalTimeout:  5 * time.Minute,
    EnableDebug:    true,
}
```

### 规划器配置
```go
PlannerConfig: &PlannerConfig{
    Model:           "gpt-4",
    Temperature:     0.7,
    MaxTokens:       2000,
    PlanningTimeout: 30 * time.Second,
}
```

### 执行器配置
```go
ExecutorConfig: &ExecutorConfig{
    MaxParallelSteps: 3,
    StepTimeout:      60 * time.Second,
    EnableCaching:    true,
}
```

### 监督者配置
```go
SupervisorConfig: &SupervisorConfig{
    Model:            "gpt-4",
    Temperature:      0.3,
    TriggerThreshold: 2,
    DecisionTimeout:  20 * time.Second,
}
```

## 最佳实践

### 1. 规划设计
- 保持步骤的原子性和独立性
- 明确指定依赖关系
- 合理使用并行执行
- 配置适当的错误处理策略

### 2. 参数管理
- 使用类型化的参数引用
- 提供默认值和转换函数
- 避免复杂的嵌套引用

### 3. 错误处理
- 根据错误类型选择合适的策略
- 设置合理的重试次数和退避时间
- 记录详细的执行日志

### 4. 性能优化
- 利用并行执行提高效率
- 启用结果缓存减少重复计算
- 设置适当的超时时间

## 技术特点

- **类型安全**: 基于 Go 的强类型系统和 EINO 的类型对齐机制
- **流式处理**: 支持 EINO 的强大流式处理能力
- **编排灵活性**: 利用 EINO 的 Graph 和 Chain 编排能力
- **可观测性**: 完整的回调机制和执行日志
- **错误恢复**: 智能的错误处理和恢复机制

## 总结

本统一对话智能体展示了如何基于 EINO 框架构建复杂的 AI 应用系统。通过 Planner-Supervisor-Executor 混合架构，系统能够：

1. **智能规划**: 将复杂任务分解为可执行的步骤序列
2. **高效执行**: 支持并行执行和动态参数解析
3. **智能监督**: 在关键节点进行干预和决策
4. **灵活扩展**: 支持多种工具集成和自定义扩展

这种架构平衡了效率和灵活性，既能处理确定性强的任务（通过详细规划），也能应对不确定性高的场景（通过监督者干预），是构建强大智能体系统的理想选择。