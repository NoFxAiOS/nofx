# Exchange-native Protection Capability Audit

Date: 2026-04-11

## Verified current baseline in code

### Exchanges with native stop/take-profit plumbing already present
- Binance
- OKX
- Bitget
- Bybit
- Gate
- KuCoin
- Hyperliquid
- Aster
- Lighter

These currently expose at least some combination of:
- `SetStopLoss`
- `SetTakeProfit`
- `CancelStopLossOrders`
- `CancelTakeProfitOrders`
- `GetOpenOrders`

### Capability matrix from current `GetProtectionCapabilities()`
- Binance: native SL/TP/partial-close, amend=true, algo=true
- OKX: native SL/TP/partial-close, amend=true, algo=true
- Gate: native SL/TP/partial-close, amend=false
- KuCoin: native SL/TP/partial-close, amend=false
- Bybit: native SL/TP/partial-close, amend=false
- Bitget: native SL/TP/partial-close, amend=false, uses plan orders
- Aster: native SL/TP/partial-close, amend=false
- Lighter: native SL/TP, partial-close currently conservative=false
- Hyperliquid: native SL/TP/partial-close, but stop-vs-tp cancellation semantics are special

## What is already implemented now

### 1. Exchange protection reconciler
A reconciler now re-checks open positions and re-applies missing exchange protection orders.
This closes the gap where protection was intended but not actually present on exchange.

Implemented behavior:
- inspect current positions
- inspect exchange open orders
- detect missing manual protection orders
- re-apply missing exchange-native protection
- verify after re-apply
- avoid re-arming already-armed native protection states
- clear stale state after positions disappear

### 2. Per-position state visibility
Per-position API/UI now surfaces:
- `protection_state`
- `break_even_state`
- `drawdown_execution_mode`
- `break_even_execution_mode`

The dashboard now distinguishes:
- exchange-native protection orders
- local runtime fallback logic
- exchange-specific native trailing state labels

### 3. Native drawdown paths already landed
#### Binance
Implemented native drawdown path using trailing stop market / algo order route.
Current safe mapping:
- only applies for drawdown rules with `close_ratio_pct ≈ 100%`
- sets native trailing stop
- marks state as `native_trailing_armed`

#### Bitget
Implemented native drawdown path using `/api/mix/v1/plan/placeTrailStop`.
Document-confirmed fields used:
- `symbol`
- `marginCoin`
- `size`
- `triggerPrice`
- `side`
- `rangeRate`
- `clientOid`

Current safe mapping:
- only applies for drawdown rules with `close_ratio_pct ≈ 100%`
- uses native trailing stop route
- marks state as `native_trailing_armed`

#### OKX
Implemented native drawdown path using advance algo / `move_order_stop` route.
Field basis gathered from external SDK/docs references:
- `ordType=move_order_stop`
- `activePx`
- `callbackRatio`
- separate cancel-advance-algos path

Current safe mapping:
- only applies for drawdown rules with `close_ratio_pct ≈ 100%`
- uses native trailing stop route
- marks state as `native_trailing_armed`

## What is still NOT finished

### 1. Partial drawdown native execution
Native drawdown currently only handles the full-close style mapping.
Not finished yet:
- partial-close drawdown native mapping
- multi-leg native drawdown plans
- exchange-by-exchange partial semantics validation

### 2. Break-even full lifecycle hardening
Current system already has per-position `armed` state and avoids repeated re-arming.
Still needs more lifecycle work:
- re-arm/reset after position resize
- re-arm/reset after side changes
- stronger explicit handling for newly reopened positions

### 3. UI explanation of partial-native boundaries
UI now shows execution mode, but should still be made more explicit about:
- native trailing currently covering full-close drawdown paths
- partial drawdown still being local fallback for now

## Recommended next implementation order
1. finish partial drawdown native strategy design (do not fake semantics)
2. harden break-even lifecycle reset / rebuild behavior
3. make UI explain exactly when native vs local fallback is in force
