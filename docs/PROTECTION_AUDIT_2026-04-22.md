# Protection Audit — Recent 24h Execution Review (2026-04-22)

## Scope

This note audits the recent ~24h protection behavior in `nofxmax` with the following user intent:

- Do **not** focus on why positions were not manually/AI closed earlier.
- Focus on whether stop-loss / break-even / protection-driven exits matched the intended behavior.
- Focus on whether protection orders were operating correctly in practice.

## Current protection snapshot baseline

Sampled decision records across recent cycles (4404, 4426, 4534, 4586, 4637, 4731, 4796, 4814) show a stable protection snapshot:

- `ladder_tp_sl.enabled = true`
- `mode = ai`
- `take_profit_enabled = false`
- `stop_loss_enabled = true`
- `fallback_max_loss = 5`
- drawdown rules:
  - `min_profit_pct=0.7`, `max_drawdown_pct=55`, `close_ratio_pct=70`
  - `min_profit_pct=1.5`, `max_drawdown_pct=45`, `close_ratio_pct=85`
- break-even:
  - `enabled = true`
  - `trigger_value = 0.6%`
  - `offset_pct = 0.2%`

Interpretation:

- The system is currently configured more like **protective stop stack + break-even + optional drawdown protection**.
- It is **not** configured as an explicit ladder take-profit system right now (`take_profit_enabled=false`).
- Therefore, profitable exits in recent trades should often be interpreted as **break-even/protective stop exits**, not classic TP fills.

## Strong runtime evidence

### DOGE live protection

Current DOGE live position repeatedly shows:

- `GetOpenOrders: found 3 open orders for DOGEUSDT`
- `Protection reconciler: DOGEUSDT long exchange protection verified`

repeated over a long window.

Interpretation:

- Exchange-side protection orders exist.
- Reconciler sees them as matching expected protection state.
- This is strong evidence that the live protection path is healthy.

## Recent trade-by-trade interpretation

### ADAUSDT (entry 0.2473, qty 110)

Close fills:

- 30 @ 0.2490 → +0.6874%
- 50 @ 0.2490 → +0.6874%
- 20 @ 0.2497 → +0.9705%
- 10 @ 0.24912727 → +0.7389%

Relevant logs:

- `break-even trigger met (0.71% >= 0.60%), applying native stop`
- `Break-even stop verified`
- `Break-even stop applied`

Interpretation:

- This is the clearest positive proof that **break-even protection is actively working**.
- Exits are in positive territory above the break-even trigger region.
- This does **not** look like AI discretionary close.

### ADAUSDT (entry 0.2535, qty 190)

Close fills:

- 130 @ 0.2565 → +1.1834%
- 40 @ 0.2559 → +0.9467%
- 10 @ 0.25625789 → +1.0879%
- 10 @ 0.2554 → +0.7495%

Relevant logs:

- repeated break-even trigger/verify/apply lines
- protection plan materialized:
  - `ladderSL=2`
  - `ladderTP=0`
  - `fullSL=true`
  - `fullTP=false`

Interpretation:

- Since ladder/full TP were disabled but stop stack existed, this profitable exit pattern is best interpreted as **protective stop / break-even-managed profitable exit**, not take-profit ladder execution.
- This is acceptable and consistent with the current strategy intent.

### ADAUSDT (entry 0.2501, qty 170)

Close fills:

- 80 @ 0.2478 → -0.9196%
- 30 @ 0.2478 → -0.9196%
- 20 @ 0.2477 → -0.9596%
- 20 @ 0.2478 → -0.9196%
- 10 @ 0.2478 → -0.9196%
- 10 @ 0.24778824 → -0.9243%

Interpretation:

- This is a clean **stop-loss style** exit cluster.
- It is not drawdown (not in profit first), and not break-even.
- Most likely explanation: ladder/full SL/fallback-protection execution around the same loss band.

### XAUUSDT LONG (entry 4816.4, qty 0.005)

Close fills:

- 0.002 @ 4772.6 → -0.9094%
- 0.002 @ 4773.0 → -0.9011%
- 0.001 @ 4767.06 → -1.0244%

Interpretation:

- Also strongly consistent with **stop-loss protection**.
- Not break-even, not drawdown.

### XAUUSDT SHORT (entry 4693.8, qty 0.012)

Close fills:

- 0.008 @ 4680.4 → +0.2855%
- 0.003 @ 4681.866667 → +0.2542%
- 0.001 @ 4684.8 → +0.1917%

Interpretation:

- Small-profit exit, but below break-even trigger (`0.6%`) and below first drawdown gate (`0.7%`).
- Best interpretation: protective stop / exchange-side protection / structural protection cleanup rather than break-even or drawdown.

### TRUMPUSDT SHORT (entry 2.831, qty 14.1)

Close fills:

- 9.9 @ 2.824 → +0.2473%
- 4.2 @ 2.824596 → +0.2262%

Interpretation:

- Same category as XAU short small-profit exit.
- Below break-even and drawdown gates.
- Most likely a protection-side close rather than AI close.

### SOLUSDT LONG (entry 85.96, qty 0.48)

Close fills:

- six fills at 86.12 → +0.1861% each

Interpretation:

- This does **not** match drawdown or break-even thresholds.
- Because all fills occurred at the same price, this looks more like a **single protection execution fragmented into multiple exchange fill records**, rather than multiple independent logic decisions.

### ZECUSDT LONG (entry 322.55, qty 0.16)

Close fills:

- 0.07 @ 320.74 → -0.5612%
- 0.01 @ 320.74 → -0.5612%
- 0.08 @ 319.795 → -0.8541%

Interpretation:

- Most consistent with **stop-loss-side protection**.
- Not enough evidence for drawdown/break-even.

## What is confirmed to work

### Confirmed healthy

- AI close suppression (`allow_ai_close=false`) in recent decision context.
- Break-even application and verification (explicit log evidence on ADA).
- Stop-loss-side protection exits in live behavior.
- Reconciler verification of exchange protection on current live positions.

### Not proven by a clean recent hit sample

- Managed drawdown as a distinct, audit-visible execution source.

The code path exists and is intended to call `closePositionByReason(..., "managed_drawdown")`, but the recent sample set did not contain a clean, undeniable drawdown-specific live hit.

## Attribution gap found

A likely attribution bug was identified in `store/position.go` `deriveCloseReason()`:

- generic `close_*` action matching could consume the reason before more specific protection semantics such as `managed_drawdown`
- matching was also too dependent on narrow tag handling

A fix was applied to make reason recovery more robust by:

- checking both `ClientOrderID` and `OrderAction`
- prioritizing specific protection semantics (`break_even`, `native_trailing`, `managed_drawdown`, `ladder_*`, `full_*`) before generic `close_*`
- mapping `fallback_maxloss*` into stop-loss-side attribution

## Heuristic reconstruction for incomplete historical data

Because older close events were already flattened to generic `close_long` / `close_short`, a reconstruction helper was added:

- `scripts/reconstruct_protection_reasons.py`

This script does **not** mutate stored data. Instead it infers a best-effort protection reason from:

- linked `trader_orders.client_order_id`
- linked `trader_orders.order_action`
- linked `trader_orders.type`
- price-vs-entry relationship
- partial/full close ratio

Current reconstruction output is consistent with the manual audit:

- loss-side partial exits (ADA losing long, XAU losing long, ZEC losing long) reconstruct as `ladder_sl?`
- profitable ADA exits above the break-even trigger reconstruct as `break_even_stop?`
- small-profit exits below BE/drawdown gates (SOL, TRUMP short, XAU short) reconstruct as `protective_profit_exit?`

The trailing `?` markers are intentional: they indicate a reasoned inference rather than first-class preserved runtime attribution.

## Operational note

- The new attribution fix improves **future** close-event labeling.
- It does **not** rewrite old rows automatically.
- For historical audits, use:
  - `scripts/audit_recent_protection_events.sh`
  - `scripts/reconstruct_protection_reasons.py`
- For post-fix live verification, use:
  - `scripts/verify_postfix_protection_reasons.py`
  - optional second arg for lookback hours, e.g. `scripts/verify_postfix_protection_reasons.py data/data.db 6`

