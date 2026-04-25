# NOFXi Agent 架构现状说明

本文档说明当前 `nofxi-dev/agent` 的整体设计、关键执行链路、内存与快照机制、skill 协作方式，以及这套实现是怎么逐步收成现在这个样子的。

适用范围：
- `nofxi-dev/agent`
- 与 Agent 强相关的 tool / store / workflow / frontend chat 行为

## 1. 当前目标

当前 Agent 的目标不是“单轮问答机器人”，而是一个可持续管理配置、跨多轮续接、可在多个对象之间切换上下文的任务型 Agent。

核心要求有 4 个：

1. 用户一句模糊话，也要先判断是在继续当前任务、切回旧任务、开新任务，还是取消。
2. `model / exchange / trader / strategy` 四个 management skill 都要能被统一路由、统一理解、统一执行。
3. 用户说的是名称，系统执行的是 ID，中间必须有稳定映射。
4. 快照、当前引用对象、最近对话、执行状态，不能只是“存着”，而要真的被 LLM 当成决策输入。

---

## 2. 顶层执行链路

当前主入口在：
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:820)

整体顺序是：

1. `tryLLMIntentRoute`
2. `tryStatePriorityPath`
3. `tryInstantDirectReply`
4. `tryReadFastPath`
5. `tryHardSkill`
6. `runPlannedAgent`

也就是说，现在系统优先做“统一语义判断”，然后才看 active flow、direct reply、hard skill 和 planner。

### 2.1 统一语义网关

顶层语义网关在：
- [llm_skill_router.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_skill_router.go:25)

LLM 目前先判断一条消息属于哪种 intent：

- `continue_active`
- `resume_snapshot`
- `start_new`
- `cancel`
- `instant_reply`

如果是 `start_new`，再继续判断 route：

- `skill`
- `workflow`
- `planner`

这层的意义是：先判断“这句话在和哪个上下文说话”，再判断“具体怎么做”。

### 2.2 顾问式系统前缀

现在统一语义网关和 Planner 共享同一份顾问式系统前缀，而不再只是“路由器”或“计划器”口吻。

这份前缀的核心基调是：

- 你是 NOFX 的核心智能中枢 `NOFXi`
- 你的首要目标不是盲目执行命令
- 你需要以资深量化顾问身份，确保每一次配置都正确、安全且符合逻辑
- 当用户遇到问题时，你要结合当前状态和平台边界主动诊断，并给出具体解决方案

统一前缀已经抽成共享 helper：
- [prompt_persona.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/prompt_persona.go)

当前已注入：

- 顶层统一语义网关
- `one-pass` 统一语义网关
- planner
- replanner

这样做的目的，是让“理解用户意图”和“后续制定执行计划”都遵守同一套顾问式人格、安全边界和诊断基调。

### 2.3 兼容式 One-Pass JSON 网关

为了减少 `router -> classifier -> extractor` 的串行 LLM 调用次数，当前已经在统一语义网关前面接入了一个兼容式 `One-Pass JSON` 网关：
- [llm_skill_router.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_skill_router.go)

它会在一次调用里尝试同时输出：

- `intent`
- `target_skill`
- `target_snapshot_id`
- `extracted_fields`
- `need_planner_help`

典型返回形状如下：

```json
{
  "intent": "continue_active",
  "target_skill": "trader_management:create",
  "target_snapshot_id": "draft_7788",
  "extracted_fields": {"leverage": "100"},
  "need_planner_help": false
}
```

当前它采用的是“兼容式接入”，不是硬切：

1. 先尝试 `one-pass` 网关
2. 如果输出不可用，回退到原有统一语义网关
3. 原有 `planner / workflow / hard skill` 分层继续保留

这意味着系统已经开始减少多次串行 LLM 往返，但不会因为一次新网关输出失误就直接把旧链路全部推翻。

---

## 3. 状态优先层

状态优先层在：
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:954)

它负责优先处理这些场景：

- 是否要恢复挂起任务
- 是否已有 active workflow
- 是否已有 active skill session
- 是否已有 active execution state

这层不是重新发明语义，而是消费上层已经决定好的“继续当前 / 切回旧快照 / 新开任务”。

如果当前有 active skill session，它会进一步进入：

- `resolveSkillSessionTurn`
- `classifySkillSessionIntentWithLLM`
- `extractSkillSessionFieldsWithLLM`

如果当前是 execution state，则会尝试：

- `bridgeExecutionStateToSkillSession`

这一层的目标，是把“planner 等待态”或者“执行等待态”桥接成真实 skill session，而不是让后续执行时丢上下文。

---

## 4. 四层上下文

当前 Agent 在规划和路由时，主要使用四层上下文：

1. `Current reference summary`
2. `Execution state JSON`
3. `Recent conversation`
4. `Task state`

它们现在被显式写进 planner prompt，相关逻辑在：
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:2950)

当前优先级是：

1. 当前引用对象
2. 当前执行状态
3. 最近对话
4. 持久化压缩背景

这解决的是“明明刚才就在说某个 trader / strategy，但后面又像不认识了一样”的问题。

---

## 5. CurrentReferences、快照和持久引用记忆

### 5.1 CurrentReferences

`CurrentReferences` 表示当前锁定的对象，例如：

- 当前 trader
- 当前 model
- 当前 exchange
- 当前 strategy

它会进入：

- router prompt
- planner prompt
- active flow classifier
- flow extraction

相关读取点包括：
- [llm_skill_router.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_skill_router.go:38)
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:2950)
- [llm_flow_extractor.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_flow_extractor.go:73)

### 5.2 Suspended snapshots

挂起任务快照用于：

- 打断当前流程
- 以后恢复到具体旧流程
- 让 LLM 在“刚才那个”“前面那个”这种模糊指代下仍能选对上下文

快照信息会进入：

- top-level router
- active flow classifier
- flow extraction

### 5.3 Persistent reference memory

现在 `CurrentReferences` 不只存在于 `ExecutionState` 里。

新增了持久引用记忆：
- [reference_memory.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/reference_memory.go)

核心函数：

- `semanticCurrentReferences`
- `semanticReferenceHistory`
- `rememberReferencesFromToolResult`

这层的作用是：

- 即使 `ExecutionState` 被清掉
- 当前对象记忆仍可延续
- 后续 follow-up 仍能命中“当前 trader / 当前 strategy”

### 5.4 DB 活性校验

持久化记忆不能被 100% 信任，因为真实对象可能已经被前端或其他入口删除。

因此现在在真正执行实体更新前，会先做一次轻量级活性校验：

- 若当前 `TargetRef` 指向的对象已经不存在
- 不再盲目继续执行
- 会清掉失效引用，并要求用户重新指定目标对象

这解决的是：

- Agent 记得“当前策略 A”
- 但真实数据库里的 `Strategy A` 已被网页前端删掉
- 后续再说“就按当前策略来”时，不会直接拿悬空 ID 去执行

---

## 6. Skill 体系

当前正式 management skill 有四个：

- `trader_management`
- `exchange_management`
- `model_management`
- `strategy_management`

Skill 定义的正式来源在：
- [agent/skills/trader_management.json](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skills/trader_management.json)
- [agent/skills/exchange_management.json](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skills/exchange_management.json)
- [agent/skills/model_management.json](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skills/model_management.json)
- [agent/skills/strategy_management.json](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skills/strategy_management.json)

---

## 7. 统一 skill 上下文：单一真源

之前的问题是：

- skill JSON 有一份简介
- router prompt 又手写一份
- workflow prompt 再手写一份
- classifier / extraction 又各有自己的上下文说明

这会导致不同层看到的 skill 描述不一致。

现在已经收成统一 helper：
- [skill_registry.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_registry.go)

关键函数：

- `buildSkillDefinitionSummary`
- `buildSkillDependencySummary`
- `buildSkillForbiddenSummary`
- `buildManagementSkillContext`

### 7.1 buildManagementSkillContext

这是现在 management skill 上下文的统一入口。

它输出两类信息：

1. 四个 management skill 的简要说明
2. 四个 management skill 的负向约束
3. 当前 active skill 的依赖说明

例如对于 `trader_management:create`，它现在会明确告诉模型：

- 创建 trader 依赖已启用交易所
- 依赖已启用模型
- 依赖可用策略
- 修复这些依赖时，仍属于 trader create 的主流程

同时它也会告诉模型一些“不能做什么”的边界，例如：

- `model_management` 不负责测试连接和诊断上游错误
- `exchange_management` 不负责行情和交易执行
- `strategy_management` 只负责模板管理，不负责直接运行

### 7.2 已接入的层

这个统一 helper 现在已经接入：

- [llm_skill_router.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_skill_router.go:39)
- [workflow.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/workflow.go:569)
- [llm_flow_extractor.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_flow_extractor.go:73)
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:2010)
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:2951)

也就是说，现在 router、workflow、classifier、extraction、planner 使用的是同一套 management skill 说明。

---

## 7.3 语义就绪检查

仅仅把消息路由到某个 skill 还不够，还要判断这条消息在“语义上是否已经准备好进入执行层”。

这层现在在：
- [skill_semantic_gate.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_semantic_gate.go)

关键点：

- `evaluateHardSkillCandidate`
- `semanticReadinessMissingSlots`
- `skillSemanticReadinessSummary`

设计目标是：

- 如果 LLM/规则已经判到某个 skill/action
- 但当前消息还明显缺少核心必填字段
- 就不要直接往 hard skill 执行层掉

例如：

- 用户说“帮我新建一个模型配置”
- 但还没有 `provider / api_key / custom_model_name`

这时系统会优先把控制权交给 planner / ask_user，而不是直接返回程序式“缺字段”提示。

这样做的价值是：

- 减少生硬的 hard skill 报错
- 让交互更像“LLM 正在推进表单”
- 避免路由下坠到执行层后再回滚

---

## 8. Active flow 内部是怎么继续的

如果顶层判断是 `continue_active`，当前消息不会直接执行 tool，而是进入当前 flow 的续接过程。

进一步地，当前只要已经进入某个 active `skill:action`，系统会优先沿着当前 action 继续推进。
旧的 `detectManagementAction / has*Patch / detect*Patch` 这类文本 heuristics 仍然保留，但已经更明确地退到：

1. 没有 active skill session 时，用于粗路由和兜底识别
2. active skill session 内，只有在 session/patch 都无法给出结果时，才作为 fallback 参与判断

这保证了：

- 先尊重已经由 LLM 和状态机确定下来的当前 flow
- 再在必要时使用旧 heuristics 补洞
- 避免“已经在当前 skill 里了，却又被文本规则抢去改 action”的抖动

现在 active flow 的打断条件也更收紧了：

- 单纯在当前 flow 里提到 `model / exchange / strategy`
- 或者在补依赖时顺带提到其他 domain 名词

不再默认视为“跳到新任务”。

只有当消息整体更像一个新的顶层请求时，跨 domain 提及才会触发中断和重新路由。

### 8.1 Skill session 续接

相关逻辑：
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:1583)
- [llm_flow_extractor.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_flow_extractor.go:61)

主要分两步：

1. `classifySkillSessionIntentWithLLM`
   - 判断是继续、取消、打断还是闲聊
2. `extractSkillSessionFieldsWithLLM`
   - 把这条消息抽成结构化字段

然后把结构化字段 merge 回当前 skill session。

### 8.1.1 对话驱动式 skill 收集

对于高价值的多轮 management flow，当前已经开始把“补槽”从代码猜测迁到 LLM 对话驱动器：
- [llm_skill_conversation.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_skill_conversation.go)

目前最先落地的是：
- `trader_management:create`
- `trader_management:update_bindings`
- `trader_management:configure_strategy`
- `trader_management:configure_exchange`
- `trader_management:configure_model`
- `model_management:update / update_status / update_endpoint / update_name`
- `exchange_management:update / update_status / update_name`
- `strategy_management:update / update_name / update_prompt / update_config / activate / duplicate`

这层的设计目标不是“让代码先把用户的话拆碎”，而是：

1. 先由 LLM 理解当前这句话在当前 flow 里到底是什么意思
2. 再按需披露当前 `skill:action` 的规则书
3. 再按当前缺失槽位，动态注入最相关的资源列表
4. 最后由代码校验结果并落地执行

当前 `llmSkillConversationDriver` 会显式拿到：

- 当前 active `skill:action` 的 contract
- 当前已收集字段
- 当前缺失槽位
- 最近一轮对话
- 只和当前缺失槽位相关的资源列表（例如只缺 `model` 时，只注入模型列表）

它返回的核心结果是：

- `ready`
- `question`
- `extracted`
- `needs_clarification`
- `cancel`

也就是说，现在 active skill 内部已经开始从：

- `代码先猜字段`
- `模型后补救`

迁移到：

- `模型先理解当前回答的语义`
- `代码只做 guardrail 与执行`

进一步地，执行层现在也开始优先消费 `skillSession` 里已经由 LLM 提取好的字段和目标对象。
只有当 session 中还没有对应值时，才会退回到旧的文本解析 fallback。

更具体地说，当前高频 management update 动作的执行顺序已经开始统一成：

1. 先消费 `session` 中已经由 LLM 提取好的字段/patch
2. 若 `session` 仍为空，再看当前整句是否能直接形成结构化 patch
3. 只有前两步都失败时，才退回到 `update_field + 单字段值` 这类旧文本猜测

这意味着旧的 `detect... / parse... / pick...` 路径仍然存在，但已经逐步退到真正的兜底层。

### 8.1.2 按需资源披露

对话驱动器不会每轮都把用户所有模型、交易所、策略全量塞进 Prompt。

现在这层已经改成：

- 缺 `exchange` 才查并注入交易所列表
- 缺 `model` 才查并注入模型列表
- 缺 `strategy` 才查并注入策略列表
- 某个槽位填完后，下一轮立即把对应资源列表从 Prompt 中移除

这就是“按需喂饭（Just-In-Time Context Injection）”：

- 节省 token
- 降低延迟
- 避免注意力稀释
- 减少无关资源对当前推理的干扰

### 8.2 Execution state 到 skill session 的桥接

如果当前是 planner / execution waiting 状态，会尝试桥接成 skill session：
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:1211)

这解决的是：

- planner 已经问到一半
- 用户回复了字段
- 但后续 hard skill 执行时又像“没收到”

现在系统已经能把 execution waiting 中收集到的信息投影回 skill session。

### 8.3 子任务成功后的父任务回流

现在快照不只是“可恢复存档”，还带有父任务信息。

相关结构在：
- [execution_state.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/execution_state.go)

新增字段包括：

- `intent_id`
- `parent_intent_id`
- `resume_on_success`
- `resume_triggers`

构建点在：
- [planner_runtime.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/planner_runtime.go:2214)

执行成功后的回流点在：
- [skill_dispatcher.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_dispatcher.go)

这解决的是：

- 用户本来在 `trader_management:create`
- 中途去启用一个被禁用的交易所或模型
- 子任务成功后，系统不再“断片”
- 会自动恢复父任务上下文，并继续提示主流程剩余缺失项

因此现在的 suspended snapshot 已更接近“带返回指针的任务栈”。

### 8.4 取消时的任务栈回溯清理

如果用户在子任务中途说：

- `算了`
- `不改了`
- `换话题`

系统现在不会只取消当前子任务就结束，而是会检查栈里是否还有父任务挂起。

如果存在父任务，会明确追问：

- 当前子任务已经取消
- 之前的父任务是否还要继续
- 或者是否“一并取消”

这样做是为了防止：

- 父任务长期挂在栈底
- 子任务被取消后无人接管
- 最终形成僵尸任务和状态堆积

---

## 9. Trader create 为什么特殊重要

`trader_management:create` 是当前最复杂的 management flow 之一，因为它天然依赖另外三个 skill 的资源状态：

- exchange
- model
- strategy

因此它不是一个封闭 skill，而是一个“父 skill”。

用户在创建交易员时说：

- 启用某个交易所
- 换一个模型
- 使用现有策略

这些都不应该默认被理解成新的平级 top-level 任务，而应优先理解成：

- 为 `trader create` 补齐依赖
- 然后继续主流程

目前这一层的 prompt 级理解已经通过统一 skill dependency summary 接入，但执行层的“修复依赖后自动回流主流程”还需要继续补强。

---

## 10. 名称和 ID 的连接

用户说的是自然语言名称，比如：

- `test`
- `DeepSeek AI`
- `高频做空策略`
- `白开水`

执行层需要的是稳定 ID。

当前这层连接主要做在：
- [skill_dispatcher.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_dispatcher.go)

核心函数：

- `hydrateCreateTraderSlotReferences`
- `findOptionByIDOrName`

设计原则是：

1. 用户层允许说名字
2. 系统尽快解析出唯一对象
3. 一旦唯一，就落成真实 ID
4. 展示给用户时仍然优先显示友好名字

这解决的是“确认文案看起来正确，但真正执行又说缺字段”的问题。

### 10.1 歧义引用澄清

除了“名字映射到 ID”，系统现在也开始处理“多个候选对象同名或近似”的情况。

相关逻辑在：

- [skill_dispatcher.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_dispatcher.go)
- [skill_management_handlers.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_management_handlers.go)

核心做法是：

1. 若唯一命中，直接解析成 ID
2. 若多个候选同时命中，不再静默选择
3. 统一返回澄清问题，让用户明确要操作哪一个对象

这比“猜一个”更安全，也避免了对象误绑。

---

## 10.2 用户级串行化

同一个用户可能在网络卡顿或前端重发的情况下，几乎同时发出两条修改消息。

为了避免：

- 两条消息并发进入同一个 active flow
- extraction 结果交叉 merge
- `skillSession` / `ExecutionState` 变成缝合状态

现在 `thinkAndAct` 和 `thinkAndActStream` 已经在用户维度上做了串行化处理。

也就是说：

- 同一个 `userID`
- 任意时刻只允许一条主消息进入 flow merge/execute 链路

这比只在单个 `save*` 调用上加锁更有效，因为它保护的是整条“读状态 -> 理解 -> merge -> 执行 -> 写状态”的事务链。

---

## 11. Tool 层约束

之前一个根问题是：上层像是“创建成功”了，但底层实际上没拿到完整必填字段。

现在部分 create 约束已经下沉到 tool 层：
- [tools.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/tools.go)

当前明确加了必填约束的包括：

- `model_management:create`
- `exchange_management:create`

这意味着：

- 不再只靠上层 prompt 判断“够不够建”
- tool 自己也会拒绝缺关键字段的 create

这能防止“草稿像成功了，但对象其实是半残”的情况。

### 11.1 Tool 层安全硬隔离

安全不能只靠 Prompt。

因此现在 Tool 层已经补了两类后端硬边界：

1. 敏感凭证永不明文返回  
说明：
- `toolGetModelConfigs`
- `toolGetExchangeConfigs`
- 以及对应 create / update 响应

都会先走安全视图，再做一层递归敏感字段剥离。  
也就是说，像 `api_key`、`secret_key`、`passphrase`、私钥这类字段，不会因为 LLM 被注入而通过 Tool 明文吐回去。

系统只允许返回类似：

- `has_api_key`
- `has_secret_key`
- `has_passphrase`

这种布尔存在性信息。

2. 交易执行必须通过会话级授权  
说明：
- `execute_trade` 不再只靠“模型说要下单”就能执行
- Tool 层现在会检查当前请求上下文里的会话权限
- 没有交易执行权限的 session，会被后端直接拒绝

这意味着即使 Prompt 被注入，模型生成了合法的 `execute_trade` 调用，只要当前 token/session 没有对应权限，后端仍然不会执行。

当前实现上，这条边界先采用：

- 已认证会话
- 明确的 session policy
- 服务端 `AllowTradeExecution` 开关

的组合约束。

也就是说，真正的安全边界现在开始下沉到了 Tool / Session / Server Policy，而不是停留在提示词层。

---

## 11.1 Planner 的多槽补齐策略

语义就绪检查把“准备不足”的请求挡回 planner 后，如果 planner 还是一轮只问一个槽位，用户体验会很差。

因此现在 planner prompt 已经明确被要求：

- 优先一次性询问多个核心缺失字段
- 在安全且常见的场景下，可以同时提出合理默认值

目标不是简单的“防止 hard skill 报错”，而是：

- 让补槽更像一次有组织的表单引导
- 减少挤牙膏式的一问一答

---

## 12. 为什么要保留 planner、workflow、hard skill 三层

当前不是所有请求都应该直接落到 hard skill。

### 12.1 Hard skill

适合：

- 结构明确
- skill 明确
- action 明确
- 必填足够

### 12.2 Workflow

适合：

- 多个 management action 串联
- 存在依赖关系

### 12.3 Planner

适合：

- 开放式目标
- 需要先探索当前状态
- 结构还不够稳定

当前的设计方向是：

- LLM 先判断当前在和哪个上下文说话
- 再决定 route
- 再进入 skill / workflow / planner

而不是一开始就靠 hard skill 猜。

### 12.4 Planner 的人格与职责

Planner 现在不只是“拆步骤”的模块，也共享了同一份 `NOFXi` 顾问式系统前缀。

这意味着 Planner 在生成计划时，会优先遵守这些原则：

- 先保证配置正确、安全、逻辑一致
- 先做状态诊断，而不是机械执行
- 缺信息时，优先组织更像顾问的多槽追问和默认值建议

因此 Planner 现在承担的是：

- 任务澄清
- 风险控制
- 配置诊断
- 计划生成

这也是为什么统一语义网关和 Planner 必须共用同一份系统前缀。

---

## 13. 前端聊天页的运行形态

前端聊天页之前的问题是：

- 切页就 abort 流式请求
- 正在生成的消息会消失

现在这部分已经调整成：

- 流式回复由更全局的 runtime/store 托管
- 站内切页不会立刻中断流
- 已生成内容会保留

这让 Agent 更接近“后台持续回复”，而不是“仅页面内临时回复”。

---

## 14. 这套结构是怎么一步步收出来的

当前架构不是一次性设计出来的，而是沿着这些问题逐步收口：

### 阶段 1：先把快照恢复链打通

目标：

- 挂起任务可恢复
- `target_snapshot_id` 真能驱动恢复

结果：

- router、flow extraction、runtime 都开始理解 snapshot

### 阶段 2：把状态续接和全局路由收成统一语义网关

目标：

- 不再一层判断“是不是当前流程”，另一层再重新猜一遍

结果：

- 先做 `continue_active / resume_snapshot / start_new / cancel / instant_reply`
- 再进入具体执行层

### 阶段 3：让 CurrentReferences 真正成为“参考书”

目标：

- 当前对象不能只是埋在 JSON 里
- 要显式进入 prompt 决策

结果：

- router、planner、classifier、extraction 都看当前引用对象

### 阶段 4：把 skill 说明和依赖说明收成单一真源

目标：

- 不再在每层 prompt 写一份不同的 skill 描述

结果：

- `buildManagementSkillContext` 成为统一入口

### 阶段 5：把名字和 ID 连接起来

目标：

- 用户交互说名字
- 系统执行用 ID

结果：

- draft -> resolved object -> ID 的链路更稳

---

## 15. 当前已经验证过的方向

当前已经补过定向测试的方向包括：

- 顶层 router prompt 包含 management skill summary
- 顶层 router / one-pass gateway / planner prompt 共享顾问式系统前缀
- 顶层/flow prompt 包含 management skill negative constraints
- 顶层 router prompt 包含 current reference summary
- active flow extraction prompt 包含 suspended snapshots
- `trader create` 的依赖说明进入统一 skill context
- semantic readiness 会把未准备好的 create 请求挡回 planner
- Tool 层不会明文返回配置秘钥，只返回存在性标记
- `execute_trade` 必须通过会话级授权和服务端开关
- 子任务成功后会自动恢复父任务上下文
- 名称歧义会触发澄清，而不是静默命中
- execution waiting state 能桥接回 skill session
- persistent reference memory 在 execution state 清掉后仍能命中当前对象
- `model/exchange` create 的 tool 必填约束生效

相关测试文件主要包括：

- [llm_intent_router_test.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/llm_intent_router_test.go)
- [skill_dispatcher_test.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_dispatcher_test.go)
- [skill_registry_test.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_registry_test.go)
- [config_tools_test.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/config_tools_test.go)

另外，仓库里现在已经有一套“AI 对练”种子回放骨架：

- [self_play_replay_test.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/self_play_replay_test.go)
- [agent_self_play_seed.zh-CN.json](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/docs/qa/fixtures/agent_self_play_seed.zh-CN.json)
- [AGENT_AI_SELF_PLAY.zh-CN.md](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/docs/qa/AGENT_AI_SELF_PLAY.zh-CN.md)

它的用途是：

- 让代码助手或大模型根据产品说明生成极端对话场景
- 把这些场景写成 JSON fixture
- 用统一回放器批量喂给 `thinkAndAct`
- 再把暴露的问题沉淀为：
  - Skill JSON 说明
  - Validator / Resolver / Readiness Gate
  - 按需上下文注入规则

---

## 16. 当前仍需要继续收的点

虽然主链已经比之前完整很多，但还有几块需要继续收：

1. `configure_strategy / configure_exchange / configure_model` 这类 action 的语义落地
说明：
这些已经在动作语义上更清晰，但要继续让 LLM 和执行层稳定对齐。

2. 更完整的 create 约束下沉
说明：
`strategy / trader` 的部分约束还可以继续更严格地下沉到执行层和 tool 层。

3. 更完整的跨 skill 依赖图
说明：
现在重点收了 `trader create` 的依赖图，未来可以继续扩展到其他多 skill 依赖场景。

4. 歧义消除的 LLM 参与度
说明：
当前歧义澄清、活性校验、父任务回溯已经有了规则级保护；后续可以继续让 LLM 参与“如何问得更自然、如何结合上下文缩小候选范围”。

5. 更细粒度的事务型状态版本控制
说明：
当前已经做了用户级串行化，足以挡住同一用户的大部分并发污染；后续如果要支持更复杂的多端并发或后台异步写入，可以继续升级成显式版本号或乐观锁。

---

## 17. 一句话总结

当前 NOFXi Agent 已经从“多个局部 if-else 叠起来的 chat handler”，逐步收成了一套：

- 统一语义网关
- 快照恢复
- 当前对象引用记忆
- 单一真源 skill context
- skill/workflow/planner 分层执行

的任务型 Agent 架构。

它现在最核心的设计原则是：

- 先判断用户在和哪个上下文说话
- 再判断在当前上下文里要做什么
- 再把自然语言解析成结构化状态
- 最后由对应 skill/workflow/tool 去安全执行
