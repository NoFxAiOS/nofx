# BTCUSDT 当前保护单只读核对（2026-04-29 16:29）

## 当前运行状态

- 后端/API 正常；`./nofx` 仍是旧二进制，尚未包含 `03bf155b fix(store): reconcile stale local open orders`。
- trader DB `is_running=0`，日志最后一次 trader stop 在 13:20 左右；但 API/订单查询和 OKX sync 仍随服务运行。
- 当前 live 交易所持仓：`BTCUSDT LONG 0.09` contract，对应本地 position quantity `0.0009` BTC。
- 本地 open position：`trader_positions.id=146`，entry `77165.5`，entry order `2557646627`。

## Live OKX open orders（通过 `/api/open-orders?symbol=BTCUSDT`）

共 4 条：

1. Fallback max loss SL
   - `3521114023091920896_sl`
   - qty `0.0009`
   - stop `75173.5`
   - role `stop_loss`

2. Ladder SL
   - `3521113946889805824_sl`
   - qty `0.0005`
   - stop `76022.5`
   - role `stop_loss`

3. Ladder SL
   - `3521113932327182336_sl`
   - qty `0.0005`
   - stop `76485.6`
   - role `stop_loss`

4. Native trailing
   - `3521114176838070272`
   - qty `0.0009`
   - activation `78052.3`
   - callback `0.0061`
   - role `trailing`

## 本地 DB 对照

`trader_orders` 中 BTC `status=NEW` 正好也是上述 4 条，数量和 stop/activation 一致。

`dynamic_protection_state_v1` 中 BTC：

- `native_trailing` armed：`3521114176838070272`
- 旧 `native_trailing` replaced：`3521114099897749504`

## 结论

- BTC 当前 4 条保护单与 live 持仓匹配，未发现残留/重复。
- 当前不建议取消 BTC 单。
- 若要应用 `03bf155b` 的本地订单状态 reconcile 修复，需要安全窗口重启后再观察。
- 因 trader 当前 DB 状态为 stopped，重启后需确认是否只重启 backend 还是同时恢复 trader running；不要误开新 AI 交易循环。
