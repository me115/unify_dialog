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
├── 主程序文件
│   ├── main.go                    # 原始主程序和演示代码
│   ├── improved_main.go           # 改进的统一对话系统主程序
│   ├── simple_main.go            # 简化版主程序
│   └── fixed_main.go             # 修复版主程序
├── 核心架构文件
│   ├── agent.go                   # 统一对话智能体核心实现
│   ├── planner.go                # 规划师组件（任务分解和计划生成）
│   ├── executor.go               # 执行器组件（步骤执行和并行控制）
│   └── supervisor.go             # 监督者组件（错误监督和决策干预）
├── 配置和工厂模式
│   ├── config.go                 # 配置管理系统（YAML配置，环境变量）
│   ├── model_factory.go          # 模型工厂（多提供商模型创建）
│   ├── callbacks.go              # 回调管理系统（日志、性能、调试）
│   └── mcp_tools.go              # MCP工具管理（官方组件集成准备）
├── 多智能体系统
│   ├── multiagent_system.go      # 多智能体编排系统实现
│   └── demo_improved.go          # 改进系统演示和测试
├── 工具和辅助系统
│   ├── parameter_resolver.go     # 动态参数解析器
│   ├── mcp_client.go             # MCP 客户端管理器
│   └── types.go                  # 核心数据类型和结构定义
├── 测试文件
│   ├── agent_test.go             # 智能体单元测试
│   ├── unified_agent_test.go     # 统一智能体测试
│   ├── simple_test.go            # 简单功能测试
│   └── comprehensive_test.go     # 综合集成测试
├── 演示和示例
│   └── demo.go                   # 基础演示程序
├── 设计文档
│   ├── DESIGN_DOCUMENT_PART1.md  # 项目概述和架构设计
│   ├── DESIGN_DOCUMENT_PART2.md  # 核心类设计和关系组织
│   ├── DESIGN_DOCUMENT_PART3.md  # 状态管理和多智能体系统
│   ├── DESIGN_DOCUMENT_PART4.md  # 数据流和系统交互
│   └── DESIGN_DOCUMENT_PART5.md  # 设计模式和可扩展性
├── 配置示例
│   ├── config.example.yaml       # 配置文件示例
│   └── config.dev.yaml          # 开发环境配置示例
├── 项目管理
│   ├── go.mod                    # Go 模块定义和依赖
│   ├── go.sum                    # 依赖版本锁定
│   ├── CLAUDE.md                 # Claude Code 使用指南
│   └── README.md                 # 项目说明文档（本文件）
```

### 文件功能详细说明

#### 🚀 主程序文件
- **main.go**: 原始实现，展示基础的 Planner-Supervisor-Executor 架构
- **improved_main.go**: 改进版本，集成了配置管理、模型工厂、回调系统
- **simple_main.go**: 简化版本，适合快速理解核心概念
- **fixed_main.go**: 修复和优化版本

#### 🏗️ 核心架构文件
- **agent.go**: 统一对话智能体的主要实现，协调三个核心组件
- **planner.go**: 规划师智能体，负责将用户请求分解为结构化执行计划
- **executor.go**: 执行器智能体，按依赖关系执行计划步骤，支持并行处理
- **supervisor.go**: 监督者智能体，监控执行过程并在需要时进行智能干预

#### ⚙️ 配置和工厂模式
- **config.go**: 统一配置管理，支持 YAML 文件和环境变量
- **model_factory.go**: 模型工厂模式，支持 OpenAI、DeepSeek、Claude 等多个提供商
- **callbacks.go**: 回调管理系统，提供调试、日志、性能监控等功能
- **mcp_tools.go**: MCP 工具管理，为官方 MCP 组件集成做准备

#### 🤖 多智能体系统
- **multiagent_system.go**: 实现官方多智能体编排模式和状态管理
- **demo_improved.go**: 改进系统的演示程序，展示各项功能集成

#### 🔧 工具和辅助系统
- **parameter_resolver.go**: 动态参数解析，支持复杂的引用和模板语法
- **mcp_client.go**: MCP 客户端管理器，统一管理多个工具客户端
- **types.go**: 核心数据结构定义，包括执行计划、步骤、状态等

#### 🧪 测试文件
- **agent_test.go**: 智能体核心功能的单元测试
- **unified_agent_test.go**: 统一智能体的集成测试
- **simple_test.go**: 基础功能验证测试
- **comprehensive_test.go**: 全面的系统集成测试

#### 📚 设计文档
完整的设计文档系列，详细说明：
- **PART1**: 项目概述、架构设计和技术选择
- **PART2**: 核心类设计和组件关系
- **PART3**: 状态管理和多智能体系统实现
- **PART4**: 数据流设计和系统交互模式
- **PART5**: 设计模式应用和扩展性考虑

## 运行示例

### 1. 基础运行方式

#### 运行原始版本（main.go）
```bash
cd examples/unify_dialog
go mod tidy
go run main.go
```

#### 运行改进版本（improved_main.go）
```bash
# 使用默认配置运行
go run improved_main.go

# 使用配置文件运行
go run improved_main.go -config config.example.yaml

# 创建示例配置文件
go run improved_main.go -create-config

# 直接处理查询
go run improved_main.go -query "请分析我们的用户数据并生成报告"
```

#### 运行简化版本（simple_main.go）
```bash
go run simple_main.go
```

### 2. 配置文件使用

#### 创建配置文件
```bash
go run improved_main.go -create-config
```

这将创建 `config.example.yaml` 文件，包含所有配置选项的示例。

#### 自定义配置
```yaml
# 复制并编辑配置文件
cp config.example.yaml config.yaml
# 编辑 config.yaml 设置你的 API 密钥和其他选项
go run improved_main.go -config config.yaml
```

### 3. 环境变量配置
```bash
# 设置模型提供商
export MODEL_PROVIDER=openai
export OPENAI_API_KEY=your_api_key_here
export OPENAI_MODEL=gpt-4o-mini

# 运行系统
go run improved_main.go
```

### 4. 演示模式
```bash
# 运行改进系统演示
go run demo_improved.go

# 运行基础演示
go run demo.go
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

## 系统架构演进

### 版本演进历程

#### V1.0 - 基础架构（main.go）
- ✅ 实现基础的 Planner-Supervisor-Executor 架构
- ✅ 支持简单的任务规划和执行
- ✅ 基本的 MCP 工具集成
- ✅ 动态参数解析功能

#### V2.0 - 配置管理（improved_main.go）
- ✅ 统一配置管理系统
- ✅ 多模型提供商支持
- ✅ 完整的回调和监控系统
- ✅ 流式处理支持

#### V3.0 - 多智能体系统（multiagent_system.go）
- ✅ 官方多智能体编排模式
- ✅ 状态管理和持久化
- ✅ 复杂工作流支持
- ✅ 全面的错误处理和恢复

### 未来路线图

#### V4.0 - 官方组件集成（计划中）
- 🔄 集成 eino-ext/components/tool/mcp 官方组件
- 🔄 使用 eino-ext/components/model/* 真实模型提供商
- 🔄 集成 eino-ext/callbacks/* 官方回调组件
- 🔄 使用官方多智能体架构模式

#### V5.0 - 增强功能（规划中）
- 📋 支持更多 MCP 工具类型
- 📋 实现工作流可视化
- 📋 添加批处理模式
- 📋 支持分布式执行

#### V6.0 - 生产就绪（未来）
- 📋 性能优化和缓存策略
- 📋 完整的监控和告警
- 📋 安全加固和权限控制
- 📋 Docker 容器化部署

## 开发指南

### 贡献代码

1. **Fork 项目**并创建功能分支
2. **遵循现有代码风格**和注释规范
3. **添加必要的测试**覆盖新功能
4. **更新文档**说明新增功能
5. **提交 PR**并描述改动详情

### 代码规范

- 所有公开函数都需要中文注释
- 保持类名、变量名等程序术语为英文
- 遵循 Go 语言标准代码格式
- 使用有意义的变量和函数命名

### 测试规范

```bash
# 运行所有测试
go test ./...

# 运行特定测试
go test -run TestUnifiedDialogAgent

# 测试覆盖率
go test -cover ./...

# 性能基准测试
go test -bench=.
```

## 总结

本统一对话智能体展示了如何基于 EINO 框架构建复杂的 AI 应用系统。通过 Planner-Supervisor-Executor 混合架构，系统能够：

1. **智能规划**: 将复杂任务分解为可执行的步骤序列
2. **高效执行**: 支持并行执行和动态参数解析
3. **智能监督**: 在关键节点进行干预和决策
4. **灵活扩展**: 支持多种工具集成和自定义扩展
5. **配置驱动**: 通过配置文件和环境变量灵活配置
6. **生产就绪**: 完整的回调、监控和错误处理机制

这种架构平衡了效率和灵活性，既能处理确定性强的任务（通过详细规划），也能应对不确定性高的场景（通过监督者干预），是构建强大智能体系统的理想选择。

### 核心优势

- 🏗️ **模块化设计**: 清晰的组件分离和职责划分
- 🔄 **智能协调**: 三个智能体协同工作，互相补充
- ⚡ **高效执行**: 并行处理和智能调度优化性能
- 🛡️ **容错能力**: 完善的错误处理和自动恢复机制
- 🔧 **易于扩展**: 插件化的工具系统和清晰的接口设计
- 📊 **可观测性**: 全面的日志、监控和调试支持

该项目不仅是一个完整的智能体实现，更是学习和研究多智能体系统设计的宝贵参考资源。