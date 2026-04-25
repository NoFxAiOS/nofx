# Agent 4 Skill 验收清单

本文档用于验收 Agent 对 4 个管理类 skill 的字段认知、工具调用和用户可见行为是否与页面编辑能力对齐。

当前范围：
- `model_management`
- `exchange_management`
- `trader_management`
- `strategy_management`

验收目标：
- 页面上能手动改的核心字段，Agent 也能稳定改
- Agent 能回答页面上可见的字段与选项
- 模糊请求不会被硬塞进错误 skill
- 多字段一句话更新时，不会被窄动作截断

## 0. 前置条件

- 已完成登录
- 后端已启动
- 至少准备 1 条可编辑的模型、交易所、交易员、策略数据
- 测试前如果有旧上下文，先在 Agent 会话里执行 `/clear`

建议先跑自动化回归：

```bash
go test ./agent -run 'Test(ManageModelToolSchemaExposesEditableFields|ManageExchangeToolSchemaExposesEditableFields|ManageTraderToolSchemaExposesAdvancedFields|ManageStrategyToolSchemaExposesFieldLevelConfig|ModelManagementManualEditableFieldsAreCoveredByAgent|ExchangeManagementManualEditableFieldsAreCoveredByAgent|TraderManagementManualEditableFieldsAreCoveredByAgent|StrategyManagementManualEditableFieldsAreCoveredByAgent|ExchangeManagementUpdateSupportsManualFields|ModelManagementThinkAndActSupportsCompositeFieldUpdates|TraderManagementUpdateSupportsAdvancedManualFields|StrategyManagementThinkAndActSupportsGridAndRiskFields)'
```

对应测试主要在：
- [skill_dispatcher_test.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/skill_dispatcher_test.go)
- [config_tools_test.go](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/agent/config_tools_test.go)

## 1. 自动化覆盖基线

通过以下检查后，才进入手工验收：

- [ ] 4 个 skill 的 tool schema 已暴露字段级参数
- [ ] 4 个 skill 的 manual editable field 集合都被 agent 字段目录覆盖
- [ ] `model` 支持一句话同时改 `enabled + custom_api_url + custom_model_name`
- [ ] `exchange` 支持一句话同时改 `account_name + hyperliquid_wallet_addr + testnet`
- [ ] `trader` 支持高级字段更新
- [ ] `strategy` 支持 grid/risk 多字段更新

## 2. Model Skill

页面参考：
- [ModelConfigModal.tsx](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/web/src/components/trader/ModelConfigModal.tsx)

核心字段：
- `provider`
- `name`
- `api_key`
- `custom_api_url`
- `custom_model_name`
- `enabled`

手工验收：

- [ ] 说“列出我的模型配置”时，能列出当前模型
- [ ] 说“这个模型的接口地址改成 xxx，模型名称改成 yyy，并且禁用”时，能一次成功更新
- [ ] 说“这个模型有哪些字段能改”时，回答至少覆盖 `API Key / 接口地址 / 模型名称 / 启用状态`
- [ ] 说“把这个模型启用”时，不会误触发重命名流程
- [ ] 说“把这个模型改成最好的”这类抽象诉求时，不应硬造字段值；应该解释或引导

通过标准：
- 回复文本明确说明已更新模型配置
- 页面刷新后字段真实变化
- 不出现“我还需要你明确要操作哪个对象”这种错误兜底

## 3. Exchange Skill

页面参考：
- [ExchangeConfigModal.tsx](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/web/src/components/trader/ExchangeConfigModal.tsx)

核心字段：
- 公共字段：
  - `exchange_type`
  - `account_name`
  - `enabled`
  - `testnet`
- CEX:
  - `api_key`
  - `secret_key`
  - `passphrase`
- Hyperliquid:
  - `api_key`
  - `hyperliquid_wallet_addr`
- Aster:
  - `aster_user`
  - `aster_signer`
  - `aster_private_key`
- Lighter:
  - `lighter_wallet_addr`
  - `lighter_api_key_private_key`
  - `lighter_api_key_index`

手工验收：

- [ ] 说“把 Dex 的账户名改成 Dex Pro，Hyperliquid 钱包改成 0xabc，testnet 打开”时，能一次成功更新
- [ ] 说“这个交易所有哪些字段能改”时，能按当前交易所类型回答差异字段
- [ ] 说“把这个交易所禁用”时，不会误进入改名分支
- [ ] 说“列出我的交易所配置”时，能读出当前配置
- [ ] 对缺少必填凭证的创建请求，会明确指出缺哪一项，而不是模糊失败

通过标准：
- 回复文本明确说明已更新交易所配置
- 页面刷新后对应字段真实变化
- 不因为对象解析失败而掉到“请明确对象”

## 4. Trader Skill

页面参考：
- [TraderConfigModal.tsx](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/web/src/components/trader/TraderConfigModal.tsx)

页面核心字段：
- `name`
- `ai_model_id`
- `exchange_id`
- `strategy_id`
- `is_cross_margin`
- `show_in_competition`
- `scan_interval_minutes`
- `initial_balance`

Agent 扩展字段：
- `btc_eth_leverage`
- `altcoin_leverage`
- `trading_symbols`
- `custom_prompt`
- `override_base_prompt`
- `system_prompt_template`
- `use_ai500`
- `use_oi_top`

手工验收：

- [ ] 说“把交易员 A 切换到策略 B，扫描间隔改成 8 分钟，全仓关闭，竞技场显示关闭”时，能一次成功更新
- [ ] 说“把高级交易员的 BTC/ETH 杠杆改成 8，山寨币杠杆改成 4，交易对改成 BTC、ETH，自定义 prompt 改成 xxx，启用 AI500”时，能成功更新
- [ ] 说“这个交易员有哪些字段能改”时，至少能回答页面核心字段和 Agent 扩展字段
- [ ] 说“启动这个交易员”时，仍会保留高风险确认链路
- [ ] 说“为什么我的交易员不交易”时，仍能走诊断 skill，不会被错误识别成 update

通过标准：
- 回复文本明确说明更新了交易员配置或绑定
- 页面刷新或查询结果能看到真实变化
- `交易对` 提取不会误吞后半句自然语言

## 5. Strategy Skill

页面参考：
- [StrategyStudioPage.tsx](/Users/zheweifang/Desktop/Nofx2/nofxi-dev/web/src/pages/StrategyStudioPage.tsx)

编辑器模块：
- `grid_config`
- `coin_source`
- `indicators`
- `risk_control`
- `prompt_sections`
- `custom_prompt`
- `publish_settings`

重点字段：
- 元信息：
  - `name`
  - `description`
  - `strategy_type`
  - `is_public`
  - `config_visible`
- Grid:
  - `symbol`
  - `grid_count`
  - `total_investment`
  - `upper_price`
  - `lower_price`
  - `use_atr_bounds`
  - `atr_multiplier`
  - `distribution`
  - `enable_direction_adjust`
  - `direction_bias_ratio`
  - `max_drawdown_pct`
  - `stop_loss_pct`
  - `daily_loss_limit_pct`
  - `use_maker_only`
- Coin source:
  - `source_type`
  - `static_coins`
  - `excluded_coins`
  - `use_ai500`
  - `ai500_limit`
  - `use_oi_top`
  - `oi_top_limit`
  - `use_oi_low`
  - `oi_low_limit`
- Risk:
  - `max_positions`
  - `min_confidence`
  - `min_risk_reward_ratio`
  - `btceth_max_leverage`
  - `altcoin_max_leverage`
  - `btceth_max_position_value_ratio`
  - `altcoin_max_position_value_ratio`
  - `max_margin_usage`
  - `min_position_size`
- Indicators / timeframe:
  - `primary_timeframe`
  - `primary_count`
  - `selected_timeframes`
  - `ema_periods`
  - `rsi_periods`
  - `atr_periods`
  - `boll_periods`
  - `enable_ema`
  - `enable_macd`
  - `enable_rsi`
  - `enable_atr`
  - `enable_boll`
  - `enable_volume`
  - `enable_oi`
  - `enable_funding_rate`
- Prompt:
  - `role_definition`
  - `trading_frequency`
  - `entry_standards`
  - `decision_process`
  - `custom_prompt`

手工验收：

- [ ] 说“把策略 A 改成网格策略，网格数量改成 14，ATR 倍数改成 2.5，最大保证金使用率改成 0.6”时，能一次成功更新
- [ ] 说“把选币来源改成静态，静态币改成 BTC、ETH，排除 DOGE，AI500 关闭”时，能成功更新
- [ ] 说“选币来源有哪些”时，能回答当前面板的来源类型与相关选项，而不是重复草稿摘要
- [ ] 说“这个策略里面的参数和 prompt 分别是什么样的”时，能走 explain/detail，不会误更新
- [ ] 说“帮我创建一个不亏钱的策略”这类抽象请求时，不应直接强绑到字段创建；应该回退 planner 或引导细化

通过标准：
- 回复文本明确说明已更新策略参数或进入合理引导
- Strategy Studio 刷新后真实反映更新
- 不会把开放式目标误当作已可执行的精确配置

## 6. 跨 Skill 语义验收

- [ ] 模糊输入先过统一语义网关，再决定 `continue_active / resume_snapshot / start_new`
- [ ] 一个 skill 进行中时，问页面字段选项，优先走 explain，不要硬落 execute
- [ ] 开放式目标型请求在参数不足时，优先回 planner，不要强行进 hard skill
- [ ] 同一句话改多个字段时，不会只改其中一个窄字段
- [ ] `/clear` 后，旧的 skill session / workflow / execution state / snapshots 都被清空
- [ ] 切回旧话题时，snapshot restore 能恢复到正确对象，而不是凭 heuristics 误接

## 7. 回归记录模板

每次验收建议记录：

- 日期：
- 提交版本：
- 后端 PID：
- 前端地址：
- 本轮执行人：

逐项记录：
- 用例：
- 用户原话：
- 预期：
- 实际：
- 是否通过：
- 备注：

## 8. 当前结论口径

当本文档第 1 节自动化基线和第 2-6 节核心手工项全部通过后，才建议对外宣称：

“Agent 对 4 个 skill 已基本对齐当前页面可编辑能力，并具备稳定的 explain / execute / planner fallback 行为。”
