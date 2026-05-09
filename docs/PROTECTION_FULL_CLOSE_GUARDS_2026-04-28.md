# Protection full-close guards / stale-position cleanup（2026-04-28）

## 背景

延续 `docs/PROTECTION_REPAIR_PLAN_2026-04-27.md` 的 P0 收口：
实盘 BTC 样本显示 **full close sync 之后，break-even / drawdown / protection reconcile 仍继续对已关闭仓位补写保护**。

## 本轮已落地

### 1. 保护写入前增加 live-position gate
以下入口现在都会先用交易所 live positions 校验 `symbol + side` 仍然存在且数量 > 0：

- `applyBreakEvenStop`
- `applyNativeTrailingDrawdown`
- `reconcileProtectionForPosition`
- cleanup 后的 protection re-apply 分支
- missing protection re-apply / placement 分支

若仓位已关闭：

- 跳过新的保护写入；
- 清理本地 protection / break-even / drawdown / peak pnl / cooldown 状态；
- 对 inactive symbol 触发 orphan protection cleanup。

### 2. OKX order sync 接入 full-close callback
`AutoTrader.Run()` 现改为启动：

- `StartOrderSyncWithFullCloseHandler(..., at.handleSyncedFullClose)`

OKX sync 在检测到 close fill 吞掉剩余仓位时，会回调：

- `handleSyncedFullClose(symbol, side)`

该回调会立刻按当前 live positions 重新计算 active key，并执行 inactive protection state cleanup，缩短“仓位已平但保护仍存活”的窗口。

### 3. 回归测试补齐
新增 / 扩展测试覆盖：

- closed position 下 break-even 不再下单，且本地状态被清空；
- closed position 下 native trailing drawdown 不再 arm；
- OKX `SyncOrdersFromOKXWithFullCloseHandler`：full close 会触发 callback，partial close 不会误触发；
- `handleSyncedFullClose` 联动：full close 后会清理本地 protection / break-even / drawdown / peak pnl 状态、取消 orphan stop/trailing，并阻断后续 BE / native trailing tick 再写入；
- 对冲/双向模式保护：同 symbol 另一侧仍有 live position 时，只清理已关闭 side 的本地状态，不做 symbol-wide orphan cleanup，避免误删另一侧保护；
- 依赖 `GetPositions()` 的保护执行测试桩补齐 live-position 数据路径。

### 4. 顺手修正的 sync 侧隐藏缺陷

在给 OKX full-close callback 补测试时顺手抓到一个真 bug：

- order sync 为了保留 close source，会把 `close_short` / `close_long` 映射成 `native_trailing` / `managed_drawdown` / `break_even_stop` 等 `requestedReason`；
- 但 `PositionBuilder.ProcessTrade()` 只认 `open_*` / `close_*`；
- 这会导致部分 protection close fill 被写入 order/fill，却**没有真正推动 position builder 做 close**，同时 full-close callback 也不会触发。

现已修正为：

- `orderRecord.OrderAction` 继续保留 source-aware `requestedReason`；
- `PositionBuilder.ProcessTrade()` 与 full-close callback 判断统一改用 `canonicalAction`（原始 `open_*` / `close_*`）。

## 验证

已通过：

```bash
go test ./trader/...
go test ./...
```

## 当前结论

这轮已经把 **“已 full close 但后续 monitor/reconciler 继续补写保护”** 的主入口先卡住，并让 OKX full-close sync 能主动触发 cleanup。

另外继续往前收了两段 native trailing 的 stale re-arm 问题：

- 之前 full trailing 只要看到“同 side 存在任意 trailing order”就会直接视为已 armed；
- 这会让 **明显漂移/过期的 full trailing order** 永久压住重新 arm，形成“状态是 armed，但实际挂的是旧参数单”的假稳定；
- 现已改为：full trailing 也按 activation/callback drift 校验，若偏离计划则允许 re-arm；
- 对 OKX tagged trailing，会在成功补上新单后按 ID 清掉旧的 stale trailing order。

同时又补了 partial tier replacement 的 quantity drift / 多 tier 候选场景：

- 之前 partial trailing replacement 只有在“已存在 tier 恰好匹配 planned qty + callback”时，才会拿到旧 tier 的 `orderID`；
- 如果旧 partial tier 的 **quantity 已经漂移**，系统虽然会重挂新 tier，但拿不到旧 tier 引用，导致旧单可能残留；
- 现已增加 replacement candidate 回退：即便不再等价，也会抓取同 side 下最像目标 tier 的 trailing tier，在 OKX replacement 成功后按 ID 清掉旧单；
- candidate 选择从单纯 qty 距离升级为综合评分：`qty drift + callback drift + activation drift`，避免多 tier 并存时误删“数量更近但 callback/activation 明显不属于该 tier”的单。

对应新增回归测试：

- 等价 full trailing 已存在时，不重复补单；
- stale full trailing 存在时，会触发 replacement，并清掉旧单；
- 等价 partial tier 已存在时，不重复补单；
- partial tier quantity drift 时，会触发 replacement，并清掉旧单；
- 多个 partial trailing tier 并存时，会优先清理综合匹配度最高的 stale tier，保留低匹配度的其他 tier。

## 仍建议继续盯的下一步

1. 继续核对 native trailing / drawdown 的 exchange-level duplicate arm key 是否还会在极端 sync 窗口漂移；
2. 实盘复核下一次 full close：确认 open order 数会下降到清洁状态，而不是继续回升；
3. 若还要继续收口，可补更贴近 OKX sync→callback→cleanup 的跨包/集成链路回归。

### 追加：OKX native trailing cleanup 验证

已补 `trader/okx/trailing_cleanup_test.go`：

- `CancelTrailingStopOrders(symbol)` 会查询 `move_order_stop` pending algos，并在 symbol 完全 inactive 的 broad cleanup 场景下取消全部 trailing algos；
- `CancelTrailingStopOrdersByIDs(symbol, ids)` 只取消指定 algo id，用于 stale/replacement 场景，避免误删其他 tier；
- 同时更新了 inactive-symbol cleanup 注释，明确 broad symbol cleanup 只在该 symbol 没有任何 live side 时触发，因此对双向/对冲模式是安全边界。

### 追加：unexpected protection 结构化分类

已补 `trader/protection_unexpected_classification.go`：

- 区分 expected static owner / expected dynamic owner / stale bot duplicate / orphan for inactive position / manual-or-foreign；
- cleanup IDs 现在只来自 bot-created stale duplicate 或 inactive-position orphan；
- manual / foreign protective orders 会被分类并记录，但不会被 bot cleanup 误删；
- ownership 日志增加 staleBot / manualForeign / dynamicOwner counts。

验证：

```bash
go test ./trader -run 'TestClassifyUnexpectedProtectionOrders|TestCollectUnexpectedProtectionOrderIDs'
go test ./...
```

### 追加：OKX sync traceability

OKX sync now attaches synced open/close orders back to inferred position rows:

- open fills attach to the current open position after `PositionBuilder.ProcessTrade`;
- close fills attach via the generated `position_close_events` row by exchange trade/order id;
- `OrderAction` still preserves source-aware reason such as `native_trailing` while the position builder receives canonical `open_*` / `close_*` actions.

### 追加：dynamic protection persisted key foundation

已补 `store/dynamic_protection_state.go`：

- 使用 `system_config(dynamic_protection_state_v1)` 保存 dynamic protection records；
- canonical key 字段覆盖 trader / exchange / symbol / side / position fingerprint / protection type / rule fingerprint / close ratio；
- managed drawdown execution fingerprint 与 native full trailing arm 会写入 dynamic protection record；
- inactive-position cleanup 会 prune 不再 active 的 dynamic protection records。

当前边界：这已经是跨重启状态基础，但不是完整 per-algo ownership ledger；部分 exchange-native partial trailing 路径还没有记录 exchange algo id。

### 追加：partial trailing per-tier persistence

继续补上 OKX partial native trailing 的 per-tier state：

- tagged OKX partial trailing verify 成功后，会再次读取 live open orders；
- 通过 qty/callback 匹配排除被替换的旧 tier，推断新 tier algo id；
- dynamic protection record 现在会写入 `native_partial_trailing`、rule fingerprint、close ratio 和推断到的 exchange algo id；
- 这把 Step 3 从“只有 full trailing / managed drawdown foundation”推进到 partial tier 也有持久化记录。

### 追加：dynamic protection restart restore

已补 runtime restore：

- `AutoTrader.Run()` 启动时会读取 `dynamic_protection_state_v1`；
- 按当前 trader id 恢复 `native_trailing_armed` / `native_partial_trailing_armed` runtime state；
- 同步恢复对应 drawdown rule fingerprint，避免进程重启后遗忘已 armed / 已执行的 dynamic protection；
- 已补 `trader/dynamic_protection_state_runtime_test.go` 验证只恢复当前 trader 的记录并忽略其他 trader。

### 追加：sync traceability helper shared beyond OKX

已将 sync order→position attach 逻辑抽到 `store.AttachSyncedOrderToPosition`：

- open order 通过当前 open position 回填 `related_position_id`；
- close order 通过 `position_close_events.exchange_order_id` 回填 `related_position_id`；
- OKX 与 Binance sync 已接入该 shared helper；
- `store/sync_traceability_test.go` 覆盖 open/close attach 基础语义。

### 追加：sync traceability rollout to remaining exchange syncs

`store.AttachSyncedOrderToPosition` 已继续接入：

- Bitget
- Bybit
- KuCoin
- Lighter
- Hyperliquid
- Aster

加上前一轮 OKX / Binance，当前主要 exchange order sync 路径都已在 `PositionBuilder.ProcessTrade` 后尝试回填 `related_position_id`。

### 追加：fill-row traceability

已补 `TraderFill.related_position_id`：

- `store.AttachSyncedOrderToPosition` 现在同时回填 order 与同 order_id 下的 fills；
- `store/sync_traceability_test.go` 已覆盖 open/close order 和 fill 都 attach 到同一 position；
- Step 5 的 traceability 从 order-centric 推进到 order + fill + close-event centric。

### 追加：position-history traceability presentation

已补 API presentation：

- `/api/positions/history` 的 `close_events[]` 现在会带出 `order_id`、`related_position_id`、`fill_count`；
- 前端/排查侧可以从 position history 直接跳到 close event 对应 order/fills，不必只靠 exchange order id 或日志。

### 追加：unexpected protection runtime summary

已补 runtime 外显：`protection_runtime.unexpected_protection` 现在会返回：

- `stale_bot_duplicate_count` / `stale_bot_duplicate_order_ids`
- `orphan_inactive_count` / `orphan_inactive_order_ids`
- `manual_or_foreign_count` / `manual_or_foreign_order_ids`
- `expected_dynamic_owner_count`
- `expected_static_owner_count`

这使 dashboard / API 调试侧可以直接区分“会被 bot cleanup 的 stale/orphan”和“应保留的 manual/foreign protective orders”。

### 追加：dynamic protection restore status gate

重启恢复 dynamic protection state 时现在会检查记录状态：

- native trailing / native partial trailing 只恢复 `status=armed` 的记录；
- canceled / stale / 非 armed native 记录不会重新标记 runtime state，避免重启后把已取消的旧 algo 当成仍 armed；
- managed drawdown 的 `status=executed` 仍会恢复 fingerprint，用于防止已执行规则重启后重复触发。
