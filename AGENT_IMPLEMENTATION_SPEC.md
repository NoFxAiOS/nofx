# AGENT Implementation Spec

## Goal
重构 Agent 的主循环逻辑：将意图识别、快照管理、字段提取完全交给 LLM 驱动。建立“任务栈”管理机制，确保跨 Skill、中途换话题、回填参数等复杂场景下的状态一致性，并由 DAG 自动化完成执行逻辑。

## 1. Core Principles

### 1.1 必须遵守
- 大脑前置：每一轮用户输入必须先经过意图识别，禁止规则层先于模型层截断复合语义。
- 快照优先：LLM 必须感知当前及历史快照（Snapshot/Reference），优先判断输入是否属于对已有任务的续接。
- DAG 驱动执行：Skill Handler 仅负责原子操作，复杂的逻辑依赖、顺序、确认由 LLM 生成的 DAG 任务包驱动执行。

### 1.2 明确禁止
- 禁止静默失败：参数越界或逻辑冲突时，必须通过 LLM 反馈给用户原因，禁止直接丢弃输入。
- 禁止单向路由：禁止在单次意图识别中只认一个 Action，必须支持多 Action 列表化输出。

## 2. Intent Handling

### 2.1 总入口
所有用户输入都必须先经过统一意图识别：
- [x] 是

统一意图识别需要判断以下结果：
- [x] `continue`
说明：续接当前快照。
- [x] `switch`
说明：开启新快照或切换到历史快照。
- [x] `cancel`
说明：明确取消当前任务。
- [x] `instant_reply`
说明：直接回答，无需进入快照流转。

### 2.2 由谁判断
意图识别主要由以下方式负责：
- [x] 大模型

### 2.3 当前 Flow 中的输入
`continue`：
- 用户提供了缺失字段。
- 用户对配置表示确认，例如“OK”“建吧”。
- 用户对已有配置进行微调。

`switch`：
- 用户输入了与当前快照领域无关的新需求。
- 例如正在调策略时突然要查余额。
- 例如正在配置一个模型时突然要求配置另一个模型。

`cancel`：
- 用户表达了明确的负面意图。
- 例如“算了”“不要了”“重来”。

## 3. Slot Extraction / Field Extraction

### 3.1 抽取方式
字段抽取主要由以下方式负责：
- [x] 大模型

如果由大模型负责，抽取时必须输入以下上下文：
- [x] 当前 `skill/action`
- [x] 当前 `draft/session`
- [x] 当前缺失字段（来自 DAG 定义）
- [x] 历史对话（近 3 轮）
- [x] 快照 / 当前引用

### 3.2 输出格式
期望大模型返回如下结构化 JSON：

```json
{
  "intent": "continue | switch | cancel",
  "target_snapshot_id": "uuid-xxxx",
  "tasks": [
    {
      "skill": "strategy",
      "action": "create",
      "fields": {
        "leverage": 20,
        "name": "my_strat"
      }
    }
  ],
  "reason": "用户提供了杠杆倍数，继续策略创建流程"
}
```

### 3.3 合并策略
- 补全模式：新抽取的字段 merge 到原快照中。
- 覆盖模式：若用户明确修改已存在的值，以最新输入为准，但必须经过 Validator 重新校验。

## 4. Flow / State Machine

### 4.1 统一状态机
所有 flow 必须统一走同一个 orchestrator：
- [x] 是

Flow 状态至少包含：
- [x] `collecting`
说明：字段收集中。
- [x] `waiting_confirmation`
说明：待用户确认。
- [x] `ready`
说明：校验通过。
- [x] `executing`
说明：DAG 执行中。
- [x] `suspended`
说明：被新任务压栈挂起。

### 4.2 Switch / Suspend / Resume
用户切换话题时，当前任务应该：
- [x] 压栈
说明：放入 History Snapshots 栈，支持后续唤回。

## 5. Skill Scope

### 5.1 适用范围
这套方法模式适用于：
- [x] 全部
说明：实现全架构的语义编排。

### 5.2 不允许单独补丁
是否禁止只针对单个 skill / 单句子打补丁：
- [x] 是

补充说明：
- 必须保证所有 Skill 共享同一套 Router 和快照机制。

## 6. Risk Control / Validation

### 6.1 校验层
字段抽取后必须统一进入 validator：
- [x] 是

validator 需要覆盖：
- [x] `strategy` 数值限制（Clamp）
- [x] `model` 配置合法性
- [x] `exchange` 凭证合法性
- [x] `trader` 绑定关系合法性

### 6.2 错误提示
- 提示原则：LLM 解析校验失败结果，用自然语言告知用户安全范围。
- 示例：`杠杆最高 20 倍，已为您设为 20，是否接受？`

## 7. Performance

### 7.1 调用策略
- 流式响应：意图识别确定后，第一时间返回“正在处理[某意图]...”，减少用户感知延迟。
- LLM Cache：对高频重复意图进行缓存。

### 7.2 快路径
允许不用大模型直接返回的场景：
- 简单打招呼，例如 `Hi`、`Hello`
- 完全匹配的单词退出指令，例如 `exit`、`quit`

## 8. Testing / Acceptance

### 8.1 必测场景
- 意图切换：正在创建策略时，询问“比特币价格”，查完后回答“继续创建刚才的策略吗”并成功恢复快照。
- 多动作合并：一句话同时完成“创建策略 A”和“配置交易所 B”。
- 纠错重填：用户输入了错误的杠杆倍数，系统提示纠正后，用户补填正确数值，系统能正确合并到原快照。

### 8.2 验收标准
- 无静默吞咽：任何有效信息必须体现在快照更新或回复中。
- 快照一致性：`CurrentReferences` 必须能精准映射到用户口中的“它”或“那个策略”。

## 9. Notes
- 快照快照还是快照：代码底层必须实现一个 `SnapshotManager`，支持 `Save/Load/List` 动作，供 LLM 通过特定的内部 Tool 进行调用。
- DAG 是约束而非死板流程：DAG 告诉 LLM 缺什么，但 LLM 决定如何通过对话向用户要到这些。
