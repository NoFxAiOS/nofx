# NOFXi Agent 实现状态文档

> 更新时间：2026-04-23

---

## 一、架构概述

ReAct 模式：LLM 做意图识别和路由，代码执行具体 skill action。

### 主要流程

```
用户消息
  → tryOnePassSemanticGateway（快速单次 LLM 路由）
      命中 → 直接执行
      未命中 → tryLLMIntentRoute（完整 LLM 路由）
                → skill（单个结构化操作）
                → workflow（多步骤跨 skill 操作）
                → planner（复杂/模糊请求）
```

### Intent 类型

| Intent | 含义 |
|--------|------|
| `continue_active` | 继续当前激活流程 |
| `resume_snapshot` | 恢复某个挂起快照 |
| `start_new` | 开启新的顶层请求 |
| `cancel` | 取消当前流程 |
| `instant_reply` | 纯聊天，不触发任务 |

---

## 二、Skill 体系

### 4 个 Management Skills

#### trader_management
- **触发：** 用户提到"交易员/trader/agent" + 操作动词
- **创建必填：** name、exchange_id、ai_model_id、strategy_id
- **字段约束：**
  - `name`：最多 50 字符
  - `scan_interval_minutes`：3～60 分钟，超出自动收敛
  - `initial_balance`：最低 100，超出自动收敛
  - `is_cross_margin`：bool，默认 true（全仓）
  - `show_in_competition`：bool，默认 false
  - `auto_start`：bool，默认 false
- **支持操作：** create、update、update_name、update_bindings、configure_strategy、configure_exchange、configure_model、start（需确认）、stop（需确认）、delete（需确认）、query_list、query_running、query_detail

#### exchange_management
- **触发：** 用户提到交易所名称 + 操作动词
- **创建必填（按交易所）：**

| 交易所 | 必填字段 |
|--------|---------|
| binance / bybit / gate | api_key、secret_key |
| okx / kucoin | api_key、secret_key、passphrase |
| hyperliquid | hyperliquid_wallet_addr |
| aster | aster_user、aster_signer、aster_private_key |
| lighter | lighter_wallet_addr、lighter_api_key_private_key |

- **其他约束：**
  - `api_key`：至少 8 位字母数字
  - `secret_key`：至少 8 位字母数字或十六进制
  - `lighter_api_key_index`：0～255，超出自动收敛
  - `testnet`：bool，默认 false
- **支持操作：** create、update、update_name、update_status、delete（需确认）、query_list、query_detail

#### model_management
- **触发：** 用户提到模型/provider 名称 + 操作动词
- **支持 provider：** openai、deepseek、claude、gemini、qwen、kimi、grok、minimax、claw402、blockrun-base
- **字段约束：**
  - `api_key`：OpenAI 必须以 `sk-` 开头
  - `custom_api_url`：必须是合法 HTTPS 地址
  - `enabled=true` 前必须填写 api_key 和 custom_model_name
- **支持操作：** create、update、update_status、update_endpoint、update_name、delete（需确认）、query_list、query_detail

#### strategy_management
- **触发：** 用户提到"策略/strategy" + 操作动词
- **字段约束：**
  - `btceth_max_leverage`：1～20，超出自动收敛
  - `altcoin_max_leverage`：1～20，超出自动收敛
  - `min_confidence`：0～100，超出自动收敛
  - `grid_count`：最小 2
  - `lower_price` 必须小于 `upper_price`
  - 策略模板**不能直接启动**，只有绑定了该策略的交易员才能启动
- **支持操作：** create、update、update_name、update_prompt、update_config、activate、duplicate、delete（需确认）、query_list、query_detail

### 4 个 Diagnosis Skills

- `trader_diagnosis`：交易员启动失败、未下单、收益异常等诊断
- `exchange_diagnosis`：invalid signature、timestamp、权限不足等诊断
- `model_diagnosis`：模型调用失败、接口不兼容、鉴权错误等诊断
- `strategy_diagnosis`：策略不生效、参数不一致等诊断

---

## 三、LLM 收到的 Prompt 内容

路由阶段 LLM 收到：
1. Skill 摘要（名称、描述、创建必填）
2. Skill 禁止规则（各 skill 不能做什么）
3. 近期对话上下文
4. 当前任务状态
5. 当前激活流程摘要 + JSON
6. 当前引用摘要
7. 执行状态 JSON
8. 挂起快照 JSON

> 注意：`buildManagementSkillContext(lang, nil)` 传 nil，active skill 的 dynamic_rules 不会注入路由 LLM。

---

## 四、Snapshot / Resume 机制

- `SnapshotManager.Save(task)` 压栈挂起任务
- `task.ResumeOnSuccess = true` + `task.ResumeTriggers` 控制子任务完成后自动回流
- `maybeResumeParentTaskAfterSuccessfulSkill` 在子任务成功后检查栈，自动恢复父任务

---

## 五、本次改动记录（2026-04-23）

### 1. 4 个 skill JSON 补全字段约束

文件：`agent/skills/*.json`

- `trader_management.json`：补全 field_constraints、validation_rules、完整 actions
- `exchange_management.json`：补全各交易所必填字段、per_exchange_required_fields
- `model_management.json`：补全 provider 枚举、API key 格式、HTTPS 校验
- `strategy_management.json`：补全杠杆/置信度/网格约束，修复中文弯引号 JSON 错误

### 2. 路由层加 inline_sub_intent

文件：`agent/llm_skill_router.go`

- `llmSkillRouteDecision` 和 `onePassGatewayDecision` 加 `InlineSubIntent` 字段
- 两个 gateway 的 JSON shape 加 `inline_sub_intent`
- Rules 加：configure_strategy/exchange/model 流程里用户想新建依赖资源时，判断 `continue_active` + `inline_sub_intent=create_sub_resource`
- `continue_active` 分支把 `inline_sub_intent` 写入 session fields

### 3. 执行层消费 inline_sub_intent

文件：`agent/skill_execution_handlers.go`

- `configure_strategy/exchange/model` 分支检测到 `inline_sub_intent=create_sub_resource` 时：
  1. 压栈当前 session（`ResumeOnSuccess=true`）
  2. 切换到对应子任务（strategy/exchange/model create）
  3. 子任务完成后自动回流父任务

---

## 六、已知问题

| 问题 | 状态 |
|------|------|
| 创建交易员时直接用现有配置，未询问用户确认 | 未修复 |
| 路由 LLM 缺 active session context（buildManagementSkillContext 传 nil） | 未修复 |
