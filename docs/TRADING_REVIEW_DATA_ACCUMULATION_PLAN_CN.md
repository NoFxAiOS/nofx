# 交易复盘与数据积累方案 V1（中文）

> 状态：方案交付版（少动系统优先）
> 时间：2026-04-13
> 适用范围：`MAX-LIUS/nofxmax`
> 目标：在尽量少改运行系统的前提下，建立后续 AI 分析审计、执行审计、逐笔复盘、策略改进所需的数据积累基础设施。

---

## 1. 方案定位

本方案不是立即大改交易系统，而是先完成三件事：

1. 盘清当前已经存在的可用数据渠道
2. 识别未来做“逐笔反思 + 系统改进”所缺的关键数据
3. 设计最小增量补强方案，避免为了分析而大范围重构运行系统

核心原则：

- **少动系统**
- **先把获取渠道摸清**
- **优先补连接能力，不优先堆字段**
- **新数据未来正确，旧数据逐步回填**

---

## 2. 现有数据渠道总表

## 2.1 决策层：`decision_records`

来源：`store/decision.go`

已具备字段能力：
- `cycle_number`
- `timestamp`
- `system_prompt`
- `input_prompt`
- `cot_trace`
- `decision_json`
- `raw_response`
- `candidate_coins`
- `execution_log`
- `decisions`
- `success`
- `error_message`
- `ai_request_duration_ms`
- `allow_ai_close`
- `ai_decision_mode`
- `protection_snapshot`

当前价值：
- 可回看 AI 当时看到了什么、说了什么、输出了什么动作
- 可审计当前决策模式、AI 平仓开关、保护配置快照

当前缺口：
- 缺少结构化环境快照（如 regime / funding / gate result）
- 缺少与 position / close event 的显式连接键
- 缺少“决策后结果评价”容器

---

## 2.2 执行层：`trader_orders` + `trader_fills`

来源：`store/order.go`

已具备字段能力：
- order 级：
  - `exchange_order_id`
  - `client_order_id`
  - `symbol`
  - `side`
  - `position_side`
  - `type`
  - `quantity`
  - `price`
  - `stop_price`
  - `status`
  - `filled_quantity`
  - `avg_fill_price`
  - `commission`
  - `reduce_only`
  - `close_position`
  - `order_action`
  - `filled_at`
- fill 级：
  - `exchange_trade_id`
  - `price`
  - `quantity`
  - `quote_quantity`
  - `commission`
  - `realized_pnl`
  - `created_at`

当前价值：
- 是执行层和交易所回执的核心事实源之一
- 是 close attribution / close event / 对账分析的关键中间层

当前缺口：
- 还没有统一显式的 `reason_tag / source_tag` 正式字段
- 不同交易所 sync 数据完整度不一致
- order 与 decision / review 之间连接仍不够强

---

## 2.3 仓位主记录层：`trader_positions`

来源：`store/position.go`

已具备字段能力：
- entry / exit
- `entry_quantity`
- `quantity`
- `entry_price`
- `exit_price`
- `realized_pnl`
- `fee`
- `leverage`
- `close_reason`
- `exit_order_id`
- `status`

当前价值：
- 适合作为“仓位生命周期主记录”
- 适合总体统计、最终 outcome、按 symbol/side 聚合

当前缺口：
- 不适合表达多段平仓事件细节
- 不适合表达每一次 partial close 的具体原因与 PnL 贡献

---

## 2.4 子事件流层：`position_close_events`

来源：`store/position_close_event.go`

已具备字段能力：
- `position_id`
- `trader_id`
- `exchange_id`
- `symbol`
- `side`
- `close_reason`
- `execution_source`
- `execution_type`
- `exchange_order_id`
- `close_quantity`
- `close_ratio_pct`
- `execution_price`
- `close_value_usdt`
- `realized_pnl_delta`
- `fee_delta`
- `event_time`

当前价值：
- 是未来逐笔复盘最关键的新层
- 支持“主记录 + 子事件流”结构
- 支持 partial / final close 细分审计

当前已支持的业务原因：
- `ai_close_long`
- `ai_close_short`
- `manual_close_long`
- `manual_close_short`
- `managed_drawdown`
- `emergency_protection_close`
- `native_trailing`
- `break_even_stop`
- `full_tp`
- `full_sl`
- `ladder_tp`
- `ladder_sl`

当前缺口：
- 旧历史不会自动拥有事件流
- 某些交易所 sync 路径仍可能丢失保护来源细节
- 事件仍缺少与 decision 的显式连接键

---

## 2.5 权益与统计层：`trader_equity_snapshots` + `position_query`

来源：`store/equity.go`、`store/position_query.go`、`store/position_history.go`

已具备字段能力：
- 权益快照：
  - `total_equity`
  - `balance`
  - `unrealized_pnl`
  - `position_count`
  - `margin_used_pct`
- 统计能力：
  - `GetFullStats`
  - `GetSymbolStats`
  - `GetDirectionStats`
  - `GetRecentTrades`
  - `GetHistorySummary`

当前价值：
- 可支撑账户表现、symbol 维度、long/short 维度、近期表现、回撤等分析

当前缺口：
- 结果统计强，但缺少与 decision / close event 的因果连接
- 适合“看结果”，不够回答“为什么变成这个结果”

---

## 2.6 配置与策略层：`strategies` / `traders` / runtime config

来源：`store/strategy.go`、`store/trader.go`

已具备字段能力：
- strategy config
- prompt sections
- custom prompt
- risk control
- regime filter
- protection config
- AI close gate
- AI decision mode
- protection snapshot（在 decision_records 中已有）

当前价值：
- 能回看“当时系统允许什么、约束了什么”
- 是 AI 分析审计和执行审计的重要背景层

当前缺口：
- 还没有足够结构化的“实际生效环境快照”
- 后续复盘仍要从 prompt / config 里手动反推一部分上下文

---

## 3. 当前最关键的问题：数据不是不够，而是连接不够

当前系统已经有：
- 决策记录
- 订单/成交
- 仓位主记录
- 子事件流
- 权益快照
- 统计查询

真正缺的是这几层之间的**稳定连接能力**。

理想链条：

```text
DecisionRecord
  -> TraderOrder / TraderFill
  -> TraderPosition
  -> PositionCloseEvents
  -> Equity impact
  -> Post-trade review
```

当前状态：
- 链条基本存在
- 但大多靠时间、symbol、order_id、事件推断拼起来
- 还没有足够多的显式连接键

因此下阶段真正应做的不是“多加几十个字段”，而是：

1. 明确连接键
2. 补结构化环境快照
3. 固化 review 模型

---

## 4. 最小增量补强方案（建议）

## 4.1 P0：连接键补强（优先级最高）

### 建议补强项
1. `decision_records` → `positions` 之间补显式连接
   - 候选字段：`entry_decision_id` / `entry_cycle_number`
2. `position_close_events` → `decision_records` 之间补显式连接（如果未来某次 close 由 AI 主动发起）
   - 候选字段：`decision_id`
3. 保持 `exchange_order_id` 作为 close event / order / fill 的核心桥接键

### 目的
未来做逐笔反思时，能稳定回答：
- 哪个 decision 开了这笔仓
- 哪个 decision（如果有）关了这笔仓
- 中间保护链如何接管

---

## 4.2 P0：环境快照结构化（最小版）

建议不是大改 prompt，而是补一个最小结构化快照：

候选字段：
- `regime`
- `funding_state`
- `volatility_bucket`
- `trend_alignment`
- `position_count`
- `margin_used_pct`
- `candidate_count`

### 目的
避免未来每次复盘时都从 prompt 文本里重新猜当时环境。

---

## 4.3 P1：统一 review 模型定义（先定义，不急着全实现）

建议先固定一版复盘输出结构：
- 决策是否合理
- 执行是否合理
- 保护接管是否合理
- 本可更优的点
- 属于哪类错误模式
- 是否需要参数调整

### 原则
先定义 review 目标模型，再反向决定数据需要补什么。

---

## 4.4 P1：历史回填工具列入方案，但暂不大规模动库

### 原则
- 先列出哪些历史可回填
- 哪些历史因交易所数据不完整无法可靠回填
- 先做 dry-run 工具设计，再决定是否正式回填

### 原因
你明确要求“少动系统”，当前更值得先把**未来数据保证正确**。

---

## 5. 当前可直接用于长期分析的核心数据集

如果未来让我持续做系统分析 / 逐笔反思，我最建议优先使用这四类表/记录：

### 5.1 决策真相源
- `decision_records`

### 5.2 执行真相源
- `trader_orders`
- `trader_fills`

### 5.3 仓位生命周期真相源
- `trader_positions`

### 5.4 逐段退出真相源
- `position_close_events`

这四层已经足以支撑第一阶段的：
- AI 审计
- 执行审计
- 保护链审计
- 逐笔复盘

---

## 6. 当前已知边界与风险

### 6.1 旧历史不会自动变新
- 旧记录缺 `position_close_events`
- 旧记录缺新的保护归因 tag
- 旧记录只能继续依赖有限 enrich 或后续回填工具

### 6.2 部分交易所 sync 数据仍先天贫血
- 有些 fills/history 接口不会直接返回保护来源
- 需要继续依赖：
  - tag
  - type
  - order_action
  - quantity partial/full
  - 价格接近 entry 的 break-even 推断

### 6.3 当前复盘还没形成标准化输出
- 现在已经有材料
- 但还没有正式的 trade review 容器 / 结果表

---

## 7. 推荐实施顺序

### Phase A（当前建议立即批准）
**数据地图与积累方案固化**
- 输出本方案文档
- 更新 `DATA_MODEL_RELATIONS_CN.md`
- 更新 `DEVLOG / PROJECT_MEMORY_ARCHIVE / 本地记忆`

### Phase B（最小代码补强）
**连接键与最小结构化快照**
- decision ↔ position
- close event ↔ decision
- regime/gating minimal snapshot

### Phase C（复盘基础设施）
**逐笔反思模板与 review 数据容器**
- 统一 review 模型
- 自动生成分析输入材料
- 后续再考虑自动化 review 结果持久化

---

## 8. 当前交付建议

当前建议不是继续扩功能，而是把系统正式分成两层理解：

### 运行层
- AI 决策
- 下单执行
- 保护接管
- 交易所同步

### 研究层
- 决策审计
- 执行审计
- 保护归因
- 逐笔反思
- 参数演化

本方案的意义在于：
**先把研究层的数据基础设施设计做成正式资产，后续每次优化都能延续，而不是靠临时聊天解释。**

---

## 9. 当前结论

在“少动系统”的要求下，当前最优策略不是继续大改运行逻辑，而是：

1. 盘清现有数据渠道
2. 明确关键缺口
3. 以最小增量补连接键与结构化快照
4. 为后续逐笔复盘与系统改进建立稳定的数据积累基础

这也是未来让 AI 分析、执行审计、系统功能评估和每笔交易反思真正可持续的前提。
