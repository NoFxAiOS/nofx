# Protection Execution Repair Plan - 2026-04-27

## Current findings

### Today's BTC position

- BTCUSDT SHORT opened at `2026-04-27 12:58:17`, entry `78867.8`, qty `0.0002`.
- It was closed at `2026-04-27 17:33:14`, exit `77904.4`, realized pnl `+0.10`.
- Database row: `trader_positions.id=138`, status now `CLOSED`, source `sync`, close_reason `close_short`.

### Critical runtime anomaly after BTC close

Immediately after the close sync, the runtime continued to treat BTCUSDT SHORT as active:

- `17:33:31` full close was synced.
- `17:33:32` protection ownership still ran for BTCUSDT SHORT.
- `17:33:34` break-even stop was applied after the close.
- `17:33:37+` native partial trailing drawdown was repeatedly armed after the close.
- Open order count continued oscillating around 8-9 orders.
- `unexpectedSL` increased from 1 to 2.

This is more severe than the earlier “ownership degraded while protected” issue: after position close, dynamic protection logic still re-applied protection for a stale/closed position window.

### Current database position state caveat

`trader_positions WHERE status='OPEN'` still contains old stale rows from March/April:

- SOLUSDT SHORT id=23
- BTCUSDT SHORT id=24
- SUIUSDT SHORT id=25
- SOLUSDT SHORT id=27
- BNBUSDT SHORT id=28
- TAOUSDT SHORT id=75

These should not be trusted as current live exchange exposure without exchange-position reconciliation. The database has historical ghost-open rows.

## Root problem clusters

### P0 - Position liveness is not authoritative enough

Risk/protection monitors appear to decide liveness from stale local position state or delayed sync state. They can continue acting after exchange position has already closed.

Required behavior:

- Before applying break-even, drawdown, trailing, or protection re-apply, verify that an exchange position still exists with non-zero size.
- If exchange size is zero or position is absent, do not arm anything. Run orphan cleanup instead.

### P0 - Dynamic protection idempotency key is incomplete

After BTC close, the system repeatedly created native partial trailing orders with new activation values and new algo IDs for the same closed/stale residual context.

Required behavior:

- Dynamic protection state should be keyed by live position identity and remaining quantity.
- If position is closed, dynamic protection state must transition to terminal/closed and block all future arm attempts.
- For native trailing, “already armed” must compare against existing exchange algo orders and local state before creating another one.

### P0 - Orphan cleanup after full close is too late or not authoritative

The system should cancel protection orders for inactive symbols promptly after full close. Instead, BTC open protection count remained high and new protection orders were added.

Required behavior:

- Full close sync should trigger immediate symbol-side protection cleanup.
- Cleanup must cancel static stops, break-even stops, trailing stops, and drawdown algo orders for the closed side.
- After cleanup, protection reconciler should not preserve unexpected orders for inactive/zero-size positions.

### P1 - Order/fill/position traceability is insufficient

Today most rows have:

- `trader_positions.source='sync'`
- `entry_decision_cycle=0`
- `trader_orders.related_position_id=0`
- close source flattened to `close_long` / `close_short`

Required behavior:

- Preserve source: `ai_open`, `break_even_stop`, `managed_drawdown`, `native_trailing`, `ladder_sl`, `fallback_stop`, `manual/sync_unknown`.
- Attach fills/orders to inferred position id using symbol, side, time window, and quantity matching when exchange does not provide a direct id.
- Do not flatten protection-triggered closes into generic close actions.

### P1 - Protection ownership state is too tolerant of unexpected stops

Repeated `state=degraded verified=false unexpectedSL=1/2` became a steady state.

Required behavior:

- Unexpected protective orders should be classified:
  - expected dynamic owner: break-even/drawdown/trailing
  - stale same-side orphan
  - foreign/manual order
  - duplicate local order
- Only preserve when explicitly classified as safe/manual/foreign.
- Duplicates created by this bot should be canceled or absorbed into canonical ownership.

## Proposed implementation sequence

### Step 1 - Add live-position gate to risk/protection actions ✅ 2026-04-28

Patch all entry points that can place/ensure protection:

- break-even monitor/apply
- drawdown monitor/native trailing arm
- protection reconciler re-apply
- fallback/ladder protection execution

Before any exchange write:

1. Fetch current exchange position for `symbol + side`.
2. Normalize quantity to exchange contract unit and base quantity.
3. If no position or quantity <= min size:
   - mark local runtime protection state closed/inactive;
   - enqueue/call orphan cleanup for that symbol/side;
   - return without placing orders.

Acceptance test:

- Simulate a full close fill, then invoke BE/drawdown/reconciler ticks.
- Assert no new stop/trailing orders are placed.
- Assert cleanup is called once.

Implementation note 2026-04-28:

- `applyBreakEvenStop`, `applyNativeTrailingDrawdown`, `reconcileProtectionForPosition` and re-apply branches now call a live-position gate before exchange writes.
- Added cleanup/state regression coverage for closed-position BE/native trailing and full-close callback follow-up ticks.

### Step 2 - Make full-close sync trigger immediate cleanup ✅ 2026-04-28

When `position_builder` detects a full close:

1. Persist close event.
2. Mark local position closed.
3. Clear protection runtime state for that position fingerprint.
4. Cancel same-side protection orders on exchange.
5. Record cleanup result.

Acceptance test:

- Given active break-even + drawdown algo orders, when a full close fill arrives, cleanup cancels all protection orders and later monitor ticks do nothing.

Implementation note 2026-04-28:

- OKX order sync now has `StartOrderSyncWithFullCloseHandler` / `SyncOrdersFromOKXWithFullCloseHandler` and `AutoTrader.Run()` wires it to `handleSyncedFullClose`.
- `handleSyncedFullClose` recomputes live active keys from exchange positions, clears inactive protection/break-even/drawdown/peak state, and cancels orphan stop/trailing orders when the whole symbol is inactive.
- Added tests for full close callback, partial close no-callback, cleanup blocking later BE/native trailing writes, and opposite-side preservation in hedge mode.

### Step 3 - Fix dynamic drawdown/trailing duplicate arm key ✅ foundation added 2026-04-28

Create a canonical dynamic protection key:

`trader_id + exchange_id + symbol + side + position_fingerprint + protection_type + rule_fingerprint + close_ratio_pct`

Where `position_fingerprint` should include:

- entry time or exchange position id if available
- entry price rounded to tick
- original/open quantity
- side

Use this key to:

- detect already-armed native trailing;
- prevent re-arm while pending/arming/armed;
- terminalize on full close;
- avoid new algo IDs every monitor tick.

Acceptance test:

- Repeated monitor ticks with changing current price must not create multiple trailing orders for the same rule.
- Re-arm is allowed only if the existing algo is canceled/missing and position is still live.

Implementation note 2026-04-28:

- Existing native trailing orders are now compared against planned activation/callback/quantity before considering the state satisfied.
- Stale full trailing and stale partial trailing tiers can be replaced; OKX targeted cleanup cancels the replaced algo id after the new tier is visible.
- Partial tier replacement candidate selection uses a combined `qty + callback + activation` drift score to avoid mis-canceling unrelated tiers when multiple tiers coexist.
- Added a persisted dynamic protection state foundation in `system_config(dynamic_protection_state_v1)`, with canonical key fields covering trader/exchange/symbol/side/position fingerprint/protection type/rule fingerprint/close ratio.
- Managed drawdown execution fingerprints and native full trailing arms now persist dynamic protection records; inactive-position cleanup prunes inactive records.
- Remaining gap: not every exchange-native partial trailing path records exchange algo IDs yet; this is enough to survive process restart as a state foundation, but not yet a complete per-algo ownership ledger.

### Step 4 - Tighten ownership classification for unexpectedSL ✅ partially completed 2026-04-28

Replace the binary unexpected count with structured categories:

- `owned_static_ladder_sl`
- `owned_fallback_sl`
- `owned_break_even_sl`
- `owned_drawdown_algo`
- `manual_or_foreign`
- `stale_bot_duplicate`
- `orphan_for_inactive_position`

Behavior:

- `stale_bot_duplicate` => cancel or absorb; do not preserve indefinitely.
- `orphan_for_inactive_position` => cancel immediately.
- `manual_or_foreign` => preserve, but mark verified=false with explicit reason.

Acceptance test:

- Duplicate bot-created stop is not left as permanent `unexpectedSL=1`.
- Manual foreign stop is preserved and reported clearly.

Implementation note 2026-04-28:

- Added structured classification for expected static owner, expected dynamic owner, stale bot duplicate, orphan for inactive position, and manual/foreign orders.
- Cleanup IDs are now sourced only from bot-created stale duplicates / inactive-position orphans; manual/foreign protective orders are classified separately and are not bot-cleaned.
- Ownership logs now include staleBot/manualForeign/dynamicOwner counts.
- Remaining gap: UI/API reporting still mostly exposes coarse unexpected counts; richer categories are currently internal/log/test-visible.

### Step 5 - Improve traceability in storage ✅ mostly completed 2026-04-28

Enhance sync/fill builder:

- infer and write `related_position_id` for open/close orders/fills;
- preserve execution source when known;
- add/derive close event `execution_source` from order_action/algo/protection metadata;
- store exchange algo id and client order id mapping to protection owner.

Acceptance test:

- A drawdown close fill creates `position_close_events.execution_source='managed_drawdown'` or `native_trailing` instead of generic `close_short`.
- Orders/fills for today's scenario can be reconstructed position -> protection -> fill without log scraping.

Implementation note 2026-04-28:

- OKX sync keeps source-aware `TraderOrder.OrderAction` while feeding canonical `open_*` / `close_*` to `PositionBuilder`.
- `OrderStore.UpdateOrderRelatedPosition`, `PositionCloseEventStore.GetByTraderAndExchangeOrderID`, and `store.AttachSyncedOrderToPosition` now provide shared order→position attachment.
- OKX, Binance, Bitget, Bybit, KuCoin, Lighter, Hyperliquid, and Aster sync paths now call the shared helper after `PositionBuilder.ProcessTrade`.
- OKX sync tests cover open/close orders attaching to the same position while preserving `native_trailing` close source.
- Remaining gap: order/fill traceability is now covered for synced rows and exposed through position-history close events; unexpected protection classification is also exposed in runtime summaries. Broader UI affordances can still be improved.

## Immediate operational recommendation

Until P0 fixes are applied:

1. Treat database `status='OPEN'` as advisory only; verify with exchange live positions.
2. After any full close, manually or programmatically check/cancel orphaned protection orders for the symbol/side.
3. Alert if `Protection ownership` remains `verified=false` with `unexpectedSL>0` for more than 2 consecutive cycles.
4. Alert immediately if protection logic places/arms anything after a full close sync.

## Verification replay checklist

After fixes:

- Re-run unit tests: `go test ./...`
- Add targeted regression tests for:
  - no protection writes after full close;
  - duplicate native trailing prevention;
  - stale bot stop cleanup;
  - related_position_id inference;
  - close source preservation.
- Replay/inspect the 2026-04-27 BTC close window and confirm:
  - no break-even apply after `17:33:31` full close;
  - no native trailing arm after full close;
  - open order count drops instead of rising;
  - ownership becomes inactive/cleaned instead of degraded.
