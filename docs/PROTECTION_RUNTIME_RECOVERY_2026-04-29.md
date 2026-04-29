# NOFX protection runtime recovery - 2026-04-29

## Current live state

- Backend is healthy and running the latest local binary built after `53abb997`.
- Frontend is healthy on Vite port 3000.
- Trader `gpt` remains stopped in DB: `is_running=0`.
- AI trading loop has **not** been restored.
- Live OKX position observed through API/logs:
  - `BTCUSDT LONG`, OKX `0.09` contracts / local `0.0009 BTC`
- Live BTC open orders are healthy and intentionally preserved:
  - 3 stop-loss orders: fallback max-loss + ladder SLs
  - 1 native trailing order: `3521114176838070272`
- DOGE live open orders are zero.

## Work completed

### Runtime protection execution fixes

- `bd8fc31e` persisted OKX native trailing order ids.
- `bbb21658` skipped duplicate reconciler drawdown arms.
- `c3271f25` guarded native trailing arming races.
- `7b7abc9f` preserved dynamic protection state after reconcile.
- `21fa7c29`, `443c0945`, `2f24b209` repaired native owner replacement/restoration.
- `138222d2` persisted break-even protection state.
- `6c983cfe` reused already-live break-even stops instead of placing duplicates.
- `4cb4b841` made reconcile respect armed break-even state.

### Live/order cleanup and accounting fixes

- `e56e198c` documented DOGE protection order residue.
- `978b4a19` documented current BTC protection order state.
- `03bf155b` added store-level stale local open-order reconciliation.
- `53abb997` wired stale local open-order reconciliation into `/api/open-orders`, so it also works when the trader loop/reconciler is stopped.
- `ce3c08a1` ignored local `nofx.pre-*` binary backups.

### Manual DB reconciliation performed

After confirming live OKX had only BTC and no TAO position, a stale local current-trader position was closed locally:

- `TAOUSDT SHORT 0.02 @ 242.207519`
- Marked `CLOSED`, `quantity=0`, `exit_price=entry_price`, `close_reason=sync_absent_from_exchange`

No exchange action was taken.

## Current DB state after cleanup

For current trader `8f43a158_fc0b7412-8c02-4c43-b3ad-cf29ba3ee28c_openai_1775840538`:

- `trader_positions status=OPEN`:
  - `BTCUSDT LONG 0.0009 @ 77165.5`
- `trader_orders`:
  - `BTCUSDT NEW=4`, matching the live BTC protection orders
  - `DOGEUSDT CANCELED=34`, `FILLED=39`, no DOGE NEW
  - `TAOUSDT FILLED=149`, no TAO OPEN order residue

## Recovery guardrails

- Do not bulk-cancel BTC protection orders; they currently match the live position.
- Do not start the AI trader casually. Trader is stopped by design while hardening continues.
- If restarting backend, keep `traders.is_running=0` unless intentionally restoring trading.
- If restoring trading, prefer a protect-only / no-new-position mode first rather than full AI opening.

## Recommended next steps

1. Add a formal position reconciliation path:
   - Compare live `GetPositions()` with local `trader_positions status=OPEN` for the current trader.
   - Mark local positions absent from exchange as `CLOSED` with `close_reason=sync_absent_from_exchange`.
   - Do not delete rows.
   - Preserve live matching positions.
2. Add tests for the reconciliation behavior.
3. Only after that, decide whether to:
   - stay backend-only with trader stopped;
   - resume in protect-only/no-new-position mode;
   - or resume full AI trading.
