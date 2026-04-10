# Exchange-native Protection Capability Audit (working note)

Date: 2026-04-10

## Current verified code-level baseline

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

## Current execution gap being closed
A reconciler now re-checks open positions and re-applies missing exchange protection orders.
This closes the gap where protection was intended but not actually present on exchange.

## Drawdown / trailing-native prioritization
### Best initial targets
1. OKX — already has algo-order polling and richer trigger/algo semantics in code.
2. Binance — already uses Algo Order API for stop-loss / take-profit.
3. Bitget — already uses plan-order API and likely the cleanest next place for exchange-native drawdown/trailing adaptation.

### Principle
- If exchange can hold the rule natively, prefer exchange-native placement.
- Local monitoring only remains for rules the exchange cannot express safely.

## Next implementation focus
- Break-even: per-position one-shot arming, verify, then stop repeated monitoring for that position.
- Drawdown: audit exchange-native trailing/plan/algo support and implement the first native-backed path on one exchange.
