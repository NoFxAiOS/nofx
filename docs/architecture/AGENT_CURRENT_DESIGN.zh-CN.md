# NOFXi Agent 当前设计说明

## 目的

本文描述当前 NOFXi Agent 的实际设计，而不是早期版本的理想设计。重点回答这些问题：

- 用户消息从哪里进入
- 什么请求会进入 planner
- 当前有哪些记忆层
- planner 如何生成与执行 plan
- tool 现在是怎么设计的
- 动态快照和当前引用分别解决什么问题
- 为什么某些问题会出现“看起来有历史，但模型还是会追问”

本文对应的主要实现文件：

- `agent/agent.go`
- `agent/web.go`
- `api/agent_routes.go`
- `agent/planner_runtime.go`
- `agent/execution_state.go`
- `agent/memory.go`
- `agent/history.go`
- `agent/tools.go`

## 一句话总览

当前 Agent 的运行模型可以概括为：

1. 前端把消息发到 `/api/agent/chat/stream`
2. 后端把登录用户身份放进 context
3. Agent 除 `/clear` 和 `/status` 外，其他消息全部进入 planner
4. planner 结合多层记忆、动态快照和 tool schema 生成 plan
5. 执行 plan 中的 `tool / reason / ask_user / respond`
6. 在执行过程中持续更新执行态、短期原话、长期摘要和当前对象引用

## 请求入口

### 前端入口

前端 Agent 页面在：

- `web/src/pages/AgentChatPage.tsx`

当前聊天使用：

- `POST /api/agent/chat/stream`

请求体里会传：

- `message`
- `lang`
- `user_key`

### 后端路由入口

路由注册在：

- `api/agent_routes.go`

这里会：

1. 经过 `authMiddleware`
2. 从登录态里取出 `user_id`
3. 通过 `agent.WithStoreUserID(...)` 写入 request context

### Agent Web Handler

真正的 HTTP handler 在：

- `agent/web.go`

主要入口：

- `HandleChat(...)`
- `HandleChatStream(...)`

再往下进入：

- `HandleMessageForStoreUser(...)`
- `HandleMessageStreamForStoreUser(...)`

## 最外层分流

当前外层分流已经被收口。

在 `agent/agent.go` 中，除了这两个命令之外，其他输入全部交给 planner：

- `/clear`
- `/status`

也就是说，现在这些都不再在外层直接处理：

- setup flow
- trade confirmation
- direct trade regex
- 自然语言配置流程
- 自然语言策略创建

这些都统一进入 planner。

这是当前设计里一个很重要的原则：

- 外层分流越少，行为边界越清晰
- 自然语言理解尽量统一交给 planner + tool

## 当前的 5 层记忆

当前不是 3 层，也不是 4 层，而是 5 层：

1. `chatHistory`
2. `TaskState`
3. `ExecutionState`
4. `CurrentReferences`
5. `Persistent Preferences`

### 1. chatHistory

定义位置：

- `agent/history.go`

作用：

- 保存最近几轮用户 / assistant 原始消息
- 给模型保留最近原话上下文
- 为后续摘要成 `TaskState` 提供原始素材

特点：

- 只保留短期原话
- 内存态
- `/clear` 时清空

适合存：

- 最近几轮对话原文
- 用户的最新措辞
- 刚刚的自然语言上下文

不适合存：

- 长期真相
- 当前外部系统状态
- 当前流程精确执行位置

### 2. TaskState

定义位置：

- `agent/memory.go`

作用：

- 保存跨轮次仍然有意义的高层摘要
- 注入 planner / reasoning / final response

持久化 key：

- `agent_task_state_<userID>`

字段：

- `CurrentGoal`
- `ActiveFlow`
- `OpenLoops`
- `ImportantFacts`
- `LastDecision`
- `UpdatedAt`

适合存：

- 当前高层目标
- 跨轮次仍然成立的未闭环事项
- 关键事实
- 最近一次重要决策及其原因

不适合存：

- step 级待办
- “下一步调用哪个 tool”
- 动态余额、持仓、配置存在性
- 任何可以通过 tool 重新读取的实时状态

### 3. ExecutionState

定义位置：

- `agent/execution_state.go`

作用：

- 保存当前 plan 的执行态
- 支持 `ask_user` 之后继续执行
- 保存 plan、当前步骤、执行日志、等待状态等

持久化 key：

- `agent_execution_state_<userID>`

当前关键字段：

- `SessionID`
- `Goal`
- `Status`
- `PlanID`
- `Steps`
- `CurrentStepID`
- `DynamicSnapshots`
- `ExecutionLog`
- `SummaryNotes`
- `Waiting`
- `CurrentReferences`
- `FinalAnswer`
- `LastError`

### 4. CurrentReferences

定义位置：

- `agent/execution_state.go`

作用：

- 记录当前对话里“这个 / 那个 / 刚才那个”到底指的是谁

当前支持的引用对象：

- `strategy`
- `trader`
- `model`
- `exchange`

这是为了解决一种常见问题：

- 用户明明前一轮刚说过“激进策略”
- 下一轮说“改一下这个策略”
- 如果没有结构化引用，模型虽然有聊天历史，也容易重新追问

`CurrentReferences` 不是系统状态快照，而是：

- 当前对话焦点对象
- 当前代词绑定对象

### 5. Persistent Preferences

对应工具：

- `get_preferences`
- `manage_preferences`

作用：

- 保存用户长期偏好

适合存：

- 默认中文回复
- 偏好激进风格
- 更关注 BTC / ETH
- 不喜欢高频
- 每天固定时间简报

它和 `TaskState` 的区别是：

- `TaskState` 偏向当前任务摘要
- `Persistent Preferences` 偏向长期用户画像

## DynamicSnapshots 是什么

`DynamicSnapshots` 是当前真实系统状态的快照。

它不是历史，也不是长期记忆，而是 planner 在规划前或执行中插入的“当前事实”。

当前会进入快照的典型信息包括：

- 当前模型配置列表
- 当前交易所配置列表
- 当前策略列表
- 当前 trader 列表
- 当前余额
- 当前持仓
- 最近交易历史

作用：

- 防止 planner 盲信旧结论
- 避免“之前没配置，现在其实已经配好了却还说没有”
- 避免“之前余额是 A，现在拿旧 observation 继续回答”

一句话：

- `DynamicSnapshots` = 当前世界里真实有什么

## CurrentReferences 和 DynamicSnapshots 的区别

这两个容易混淆，但职责完全不同。

`DynamicSnapshots`：

- 当前系统状态快照
- 是候选集合 / 当前事实
- 例如当前有两个策略：`激进`、`新策略`

`CurrentReferences`：

- 当前对话焦点对象
- 是“这个”到底指谁
- 例如用户现在说的“这个策略”就是 `激进`

可以这样理解：

- `DynamicSnapshots` 是地图
- `CurrentReferences` 是你手指现在指着地图上的哪个点

## Planner 的输入

planner 主逻辑在：

- `agent/planner_runtime.go`

生成计划时，当前会把这些东西一起送给模型：

- 当前用户请求
- tool schema
- `Persistent Preferences`
- `TaskState`
- `ExecutionState`
- `Resume context`
- `Structured waiting state`
- `Observation context`

其中 observation context 不是旧版单数组，而是分层后的：

- `dynamic_snapshots`
- `execution_log`
- `summary_notes`

## Plan 的结构

当前 planner 只允许这 4 类 step：

- `tool`
- `reason`
- `ask_user`
- `respond`

这意味着现在的 Agent 不是一个“自由发挥的回复器”，而是：

- 先规划
- 再执行步骤
- 必要时重规划

## 步骤执行流程

`executePlan(...)` 的核心逻辑是：

1. 找下一个 pending step
2. 标记 step 为 running
3. 执行对应类型
4. 写回 `ExecutionState`
5. 必要时触发 replanning

不同 step 类型行为如下：

### tool

- 调内部 tool
- 把结果写入 `ExecutionLog`
- 根据结果更新 `CurrentReferences`
- 必要时触发 replanner

### reason

- 发起一次短 reasoning 调用
- 生成一段简短中间推理
- 写入 `ExecutionLog`

### ask_user

- 进入 `waiting_user`
- 保存 `WaitingState`
- 把问题直接回给用户

### respond

- 生成最终回答
- 标记当前执行完成

## WaitingState 是什么

`WaitingState` 用来解决：

- 用户回复 `是`
- 用户回复 `继续`
- 用户回复 `那个就行`

这类短回复如果没有结构化等待状态，很容易丢上下文。

当前字段包括：

- `Question`
- `Intent`
- `PendingFields`
- `ConfirmationTarget`
- `CreatedAt`

它的作用是：

- 告诉 planner 上一轮到底在等什么
- 让这轮短回复更容易被理解成“对上一问的回答”

## CurrentReferences 如何更新

当前是双路径更新：

### 1. 用户消息命中对象名时更新

如果用户说：

- `修改激进策略`
- `停止 lky`
- `用 DeepSeek`

系统会去当前用户的策略 / trader / model / exchange 列表里尝试匹配名称或 ID。

匹配成功后，更新 `CurrentReferences`。

### 2. tool 成功返回对象时更新

比如：

- `manage_strategy(create/update/activate)`
- `manage_trader(create/update)`
- `manage_model_config(update)`
- `manage_exchange_config(update)`

只要 tool 返回了具体对象，系统就会把对应 ID / name 写回当前引用。

## Tool 设计

当前 tool 是“资源型 tool”设计，不是“页面动作型 tool”。

### 当前主要工具

配置资源：

- `get_exchange_configs`
- `manage_exchange_config`
- `get_model_configs`
- `manage_model_config`

策略资源：

- `get_strategies`
- `manage_strategy`

trader 资源：

- `manage_trader`

交易 / 查询资源：

- `search_stock`
- `execute_trade`
- `get_positions`
- `get_balance`
- `get_market_price`
- `get_trade_history`

### 为什么这么设计

优点：

- tool schema 稳定
- 行为边界清晰
- planner 更容易学会
- 资源增删改查统一

当前 `manage_strategy` 支持：

- `list`
- `get_default_config`
- `create`
- `update`
- `delete`
- `activate`
- `duplicate`

当前 `manage_trader` 支持：

- `list`
- `create`
- `update`
- `delete`
- `start`
- `stop`

## 为什么“创建策略”不该默认依赖交易所和模型

当前设计里，策略模板应该是独立资源：

- `strategy`

而运行态对象是：

- `trader`

更合理的边界是：

- 创建策略模板：用 `manage_strategy`
- 把策略跑起来：用 `manage_trader`

也就是说：

- 策略不默认依赖交易所和模型
- 只有当用户要求“运行 / 部署 / 创建 trader”时，才需要进一步关联 exchange / model / trader

## 当前一个完整例子

用户输入：

`帮我创建一个新的激进策略模板，名字就叫激进。创建完后，再把这个策略绑定到 trader lky。`

当前大致流程：

1. 前端请求 `/api/agent/chat/stream`
2. 后端注入 `store_user_id`
3. Agent 进入 planner
4. planner 刷新动态快照：
   - 当前策略
   - 当前 trader
5. 生成 plan，例如：
   - `get_strategies`
   - `manage_strategy(create)`
   - `manage_trader(update)`
   - `respond`
6. 执行 `manage_strategy(create)` 后：
   - 写入 `ExecutionLog`
   - 更新 `CurrentReferences.strategy`
7. 执行 `manage_trader(update)` 时：
   - 直接使用刚创建策略的 ID
8. 输出最终回复

如果此后用户继续说：

`把这个策略的 prompt 改激进一点`

系统会优先从 `CurrentReferences.strategy` 理解“这个策略”。

## 为什么看起来“有历史”，模型还是会追问

因为“有聊天历史”不等于“有结构化对象绑定”。

如果没有 `CurrentReferences`：

- 模型只能依赖原话文本推断“这个策略”是谁
- 一旦中间插入多条消息，或者有多个候选策略
- 就容易重新追问

所以当前设计里，`CurrentReferences` 是补齐这一块的关键。

## 当前已知限制

### 1. 外层虽然已经大幅收口，但仍然不是纯 graph runtime

现在比之前更统一，但整体仍然是：

- Agent 主入口
- Planner
- Tool 执行

而不是完整 node-graph 引擎。

### 2. ExecutionState 仍然是按 userID 单槽位

这意味着：

- 同一用户的多个并行任务仍然可能相互影响

更彻底的方向应该是：

- 按 thread / session 多实例存储

### 3. CurrentReferences 目前还是轻量实现

当前只覆盖：

- strategy
- trader
- model
- exchange

后面如果要更强，需要考虑：

- 多候选冲突消解
- 昵称映射
- 跨更长会话的稳定实体绑定

## 当前设计的核心思想

一句话总结：

- `chatHistory` 记原话
- `Persistent Preferences` 记长期偏好
- `TaskState` 记高层摘要
- `ExecutionState` 记当前流程
- `DynamicSnapshots` 记当前事实
- `CurrentReferences` 记当前指代对象
- `planner` 决定步骤
- `tools` 执行落地动作

这就是当前 NOFXi Agent 的实际运行设计。
