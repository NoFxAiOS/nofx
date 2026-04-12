# NOFX 核心数据表与实体关系说明（中文，正式版）

> 状态：接管收口版 v1  
> 时间：2026-03-24

---

## 1. 文档目的

这份文档用于把 store 层最关键的实体说明清楚：

- 核心表有哪些
- 主要字段是什么
- 彼此怎么关联
- 哪些是真相源，哪些是衍生/展示辅助数据

---

## 2. 总体关系图（概念级）

```text
User
 ├─ AIModel
 ├─ Exchange
 ├─ Strategy
 └─ Trader
      ├─ DecisionRecord
      ├─ EquitySnapshot
      ├─ TraderOrder
      │    └─ TraderFill
      ├─ TraderPosition
      ├─ GridState / GridOrders（grid 相关）
      └─ AIChargeRecord

TelegramConfig
SystemConfig
```

说明：
- `Trader` 是运行主体
- 订单、持仓、权益、决策、AI 成本几乎都围绕 `Trader` 展开
- `Strategy` / `Exchange` / `AIModel` 更像 `Trader` 的配置依赖

---

## 3. 核心实体说明

## 3.1 `users`

对应：`store/user.go`

职责：
- 平台用户身份
- 登录、权限、用户级资源归属

它是大多数业务对象的拥有者根节点。

---

## 3.2 `ai_models`

对应：`store/ai_model.go`

职责：
- 记录用户可用 AI 模型配置
- 保存 provider、api key、custom api url、model name、enabled 状态等

关系：
- 一个 `Trader` 会引用一个 `AIModelID`

---

## 3.3 `exchanges`

对应：`store/exchange.go`

职责：
- 保存交易所账户配置
- 包括 CEX / DEX 的凭证与账户信息

关系：
- 一个 `Trader` 会引用一个 `ExchangeID`
- 一个 exchange 可以被多个 trader 复用（逻辑上允许，但要注意运行风险）

---

## 3.4 `strategies`

对应：`store/strategy.go`

职责：
- 保存策略配置 JSON
- 包括：
  - coin source
  - indicators
  - risk_control
  - protection
  - prompt_sections
  - grid_config

关键特点：
- `Config` 本体是 JSON 字符串
- `StrategyConfig` 是其结构化内存表示
- 已新增 `ProtectionConfig`，成为当前策略实体的重要一部分

关系：
- 一个 `Trader` 会引用一个 `StrategyID`

---

## 3.5 `traders`

对应：`store/trader.go`

职责：
- 是系统最核心的“运行实体配置表”
- 描述某个具体自动交易实例如何运行

关键字段：
- `id`
- `user_id`
- `name`
- `ai_model_id`
- `exchange_id`
- `strategy_id`
- `initial_balance`
- `scan_interval_minutes`
- `is_running`
- `is_cross_margin`
- `show_in_competition`

关系：
- Trader 连接配置侧与运行侧
- 大部分运行数据都以 `trader_id` 为主外键语义

---

## 3.6 `decision_records`

对应：`store/decision.go`

职责：
- 保存每个 AI cycle 的决策日志
- 是“为什么这么做”的主要审计源

主要内容：
- prompt
- CoT trace
- raw response
- decision JSON
- execution log
- success / error
- AI request duration

注意：
- 它更偏“审计日志 / 决策记录”
- 不是最终成交真相源

---

## 3.7 `trader_equity_snapshots`

对应：`store/equity.go`

职责：
- 保存 trader 在时间轴上的权益快照
- 用于收益曲线、排行榜、历史趋势展示

主要字段：
- `trader_id`
- `timestamp`
- `total_equity`
- `balance`
- `unrealized_pnl`
- `position_count`
- `margin_used_pct`

注意：
- 它是时间序列快照
- 更偏展示/统计，不是订单级真相

---

## 3.8 `trader_orders`

对应：`store/order.go`

职责：
- 保存订单主记录
- 是执行层最重要的结构化真相之一

关键字段：
- `trader_id`
- `exchange_id`
- `exchange_order_id`
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

注意：
- 一个 order 可能对应多个 fill

---

## 3.9 `trader_fills`

对应：`store/order.go`

职责：
- 保存成交明细
- 是订单执行细粒度真相源

关键字段：
- `order_id`
- `exchange_trade_id`
- `symbol`
- `side`
- `price`
- `quantity`
- `commission`
- `realized_pnl`

关系：
- 多个 fills → 一个 order

在统计口径专项里，这张表很关键。

---

## 3.10 `trader_positions`

对应：`store/position.go`

职责：
- 保存本地重建/维护的持仓视图
- 记录开仓、平仓、持仓状态、实现盈亏等

关键字段：
- `trader_id`
- `exchange_id`
- `exchange_position_id`
- `symbol`
- `side`
- `entry_quantity`
- `quantity`
- `entry_price`
- `entry_time`
- `exit_price`
- `exit_time`
- `realized_pnl`
- `fee`
- `leverage`
- `status`
- `close_reason`
- `source`

注意：
- 这是“本地仓位真相层”
- 它和订单/成交一起构成 PnL 口径核验的核心基础

---

## 3.11 `ai_charges`

对应：`store/ai_charge.go`

职责：
- 记录 AI 调用成本 / 次数
- 支撑 AI cost 页面与运营统计

---

## 3.12 `telegram_config`

对应：`store/telegram_config.go`

职责：
- 保存 Telegram bot token、model 绑定、chat 绑定等配置

---

## 3.13 `system_config`

对应：`store/store.go` 中初始化

职责：
- 保存系统级键值配置
- 如 installation id 等全局系统信息

---

## 4. 哪些是“配置真相”，哪些是“运行真相”

## 4.1 配置真相
- `users`
- `ai_models`
- `exchanges`
- `strategies`
- `traders`
- `telegram_config`
- `system_config`

## 4.2 运行真相
- `trader_orders`
- `trader_fills`
- `trader_positions`

## 4.3 运行审计 / 展示辅助
- `decision_records`
- `trader_equity_snapshots`
- `ai_charges`

这个区分很重要：
- 调配置问题，看配置真相
- 查执行问题，看运行真相
- 看 AI 为什么这么做，看审计记录
- 看收益曲线与展示效果，看展示辅助表

---

## 5. 当前最值得重点审计的实体关系

1. `traders` → `strategies / exchanges / ai_models`
2. `trader_orders` → `trader_fills`
3. `trader_positions` 与 `trader_orders / fills` 的一致性
4. `decision_records` 与真实执行结果之间的偏差
5. `trader_equity_snapshots` 与 position/order 口径是否完全一致

---

## 6. 2026-04-13 新增：主记录 + 子事件流数据层

为支撑后续逐笔反思与保护归因，当前数据模型已新增并开始使用：

### `position_close_events`
对应：`store/position_close_event.go`

职责：
- 保存每一段 partial / final close 的子事件流
- 作为 `trader_positions` 主记录的展开事件层
- 用于区分 AI / 手动 / 保护接管导致的各段退出

关键字段：
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
- `trader_positions` 继续作为仓位生命周期主记录
- `position_close_events` 承担多段平仓事件流
- 后续逐笔复盘、保护接管分析、AI/手动/系统行为对账，优先围绕该层展开

当前建议的真相源分层：
1. 决策真相源：`decision_records`
2. 执行真相源：`trader_orders` + `trader_fills`
3. 仓位主记录真相源：`trader_positions`
4. 逐段退出真相源：`position_close_events`
5. 权益时间序列真相源：`trader_equity_snapshots`

---

## 7. 当前结论

NOFX 的数据模型已经具备平台化系统的基本形态：

- 配置实体独立
- 运行实体围绕 Trader 聚合
- 决策、权益、AI 成本作为时间轴辅助信息存在

后续如果要做收益统计 / PnL 口径专项，这份文档里的：

- `trader_orders`
- `trader_fills`
- `trader_positions`
- `trader_equity_snapshots`

会是优先核验对象。
