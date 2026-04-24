# NOFXi 诊断与配置 Skills（第一批）

这份文档用于沉淀交易智能助手的第一批高频诊断与配置 skill。

目标不是让模型“更会想”，而是让它面对常见问题时，优先走稳定、可复用的排查路径。

## 设计原则

- 优先按 skill 回答，不要对高频问题重复自由规划
- 先归类问题，再给出原因、检查项和修复建议
- 能通过工具验证当前状态时，先查再下结论
- 敏感信息只指导填写，不完整回显
- 对结论不确定时，要明确标注为“更可能”或“优先怀疑”

## skill_model_api_setup

### 适用场景

- 用户问某个大模型的 API key 去哪里申请
- 用户问 base URL 怎么填
- 用户问 model name 怎么填
- 用户问 OpenAI / Claude / Gemini / DeepSeek / Qwen / Kimi / Grok / MiniMax 怎么接入

### 处理策略

1. 先确认用户要配置哪个 provider
2. 告诉用户需要准备的最少字段：
   - provider
   - API key
   - custom_api_url
   - custom_model_name
3. 如果系统已有默认地址和默认模型名，优先给推荐值
4. 回答按步骤组织，不要泛泛解释概念

### 已知实现事实

- 系统内置 provider 默认运行配置，见 `agent.resolveModelRuntimeConfig(...)`
- 常见 provider 已有默认 URL 和默认 model name

## skill_model_config_diagnosis

### 适用场景

- 模型保存成功但 agent 仍然不可用
- 提示 AI unavailable
- 提示模型没启用
- 提示 custom_api_url 不合法
- 配置后 trader 不生效

### 优先排查

1. 是否存在已启用模型
2. API key 是否为空
3. custom_api_url 是否为合法 HTTPS 地址
4. custom_model_name 是否为空或不匹配
5. 当前 trader 是否绑定了这个模型
6. 更新模型后是否已触发 trader reload

### 已知实现事实

- 非 HTTPS 的 `custom_api_url` 会被后端拒绝，见 `api/handler_ai_model.go`
- 已启用模型如果缺少 API Key 或 URL，会导致 agent 无法就绪，见 `agent.ensureAIClientForStoreUser(...)`
- 更新模型配置后，系统会尝试移除并重载相关 trader，使新配置立即生效

### 输出格式

- 现象
- 更可能原因
- 先检查什么
- 下一步怎么修复

## skill_exchange_api_setup

### 适用场景

- 用户要新建交易所 API
- 用户不知道交易所需要哪些权限
- 用户问 API key / secret / passphrase 分别填什么

### 通用处理策略

1. 先确认交易所类型
2. 告知必须权限与禁止权限
3. 告知是否需要额外字段
4. 强调 IP 白名单与权限配置
5. 引导用户回到系统内完成绑定

### 特殊规则

- OKX 除 API Key 和 Secret 外，还需要 passphrase
- Bybit 永续/合约交易需要合约权限
- 不建议开启提现权限

### 参考文档

- `docs/getting-started/okx-api.md`
- `docs/getting-started/bybit-api.md`

## skill_exchange_api_diagnosis

### 适用场景

- `invalid signature`
- `timestamp` 错误
- `IP not allowed`
- `permission denied`
- 交易所连接不上

### 优先排查

1. 系统时间是否同步
2. API Key / Secret 是否正确
3. 是否遗漏额外字段，如 OKX passphrase
4. IP 白名单是否包含当前服务器
5. 是否启用了交易或合约权限
6. 密钥是否过期或已重建

### 已知实现事实

- 时间不同步是 `invalid signature` / `timestamp` 的高频根因，见 `docs/guides/TROUBLESHOOTING.zh-CN.md`
- OKX 的 passphrase 缺失会导致签名相关问题，见 `docs/getting-started/okx-api.md`

### 输出格式

- 报错现象
- 最常见根因
- 优先检查顺序
- 修复步骤

## skill_trader_start_diagnosis

### 适用场景

- trader 启动不了
- trader 启动了但没开始交易
- 页面显示已启动但一直没有动作
- 用户怀疑 strategy / model / exchange 绑定有问题

### 优先排查

1. 是否有已启用的模型配置
2. 是否有已启用的交易所配置
3. trader 是否绑定了 exchange_id / strategy_id / ai_model_id
4. 交易所余额和权限是否满足下单条件
5. AI 最近的决策到底是 wait、hold 还是下单失败

### 回答原则

- 要区分“没启动”“启动了但 AI 选择不交易”“尝试下单但失败”这三类
- 不要把“没开仓”直接等同于“系统故障”

## skill_order_execution_diagnosis

### 适用场景

- 下单失败
- 只开空不开户 / 只开单边
- 杠杆报错
- position side mismatch

### 优先排查

1. 账户模式是否匹配，例如 Binance 是否为 Hedge Mode
2. 是否为子账户杠杆限制
3. 合约权限是否开启
4. 余额、保证金、可交易 symbol 是否满足条件

### 已知实现事实

- Binance 在 One-way Mode 下，可能出现 `position side mismatch` 或单边行为
- 某些子账户杠杆上限较低，超过限制会直接失败
- 这些问题在 `docs/guides/TROUBLESHOOTING.md` 已有明确说明

## skill_strategy_diagnosis

### 适用场景

- 用户说策略没生效
- 用户说 prompt 预览和实际不一致
- 用户说修改策略后 trader 行为没有变化

### 优先排查

1. 当前编辑的是策略模板，还是 trader 的 custom prompt
2. 策略是否真的保存成功
3. 是否需要重新读取当前配置做对比
4. 用户说的“没生效”是指未保存、未绑定，还是运行结果与预期不一致

### 回答原则

- 先明确“对象”再排查：strategy template / trader / prompt override
- 如果能读取当前保存值，就不要凭印象判断

## 后续扩展方向

下一批可以继续补：

- `skill_balance_and_position_diagnosis`
- `skill_market_data_diagnosis`
- `skill_prompt_generation_diagnosis`
- `skill_strategy_test_run_diagnosis`
- `skill_exchange_specific_setup_<exchange>`
- `skill_model_provider_setup_<provider>`
