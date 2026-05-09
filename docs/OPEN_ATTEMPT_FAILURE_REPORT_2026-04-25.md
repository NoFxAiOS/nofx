# Open-attempt failure report, 2026-04-25 after 00:30 Asia/Shanghai

Scope: `decision_records` failures with `timestamp >= 2026-04-24 16:30:00+00:00` (which is 2026-04-25 00:30 CST), plus corroborating evidence from `data/nofx_2026-04-25.log`.

## Executive summary

After 00:30 local, the system logged **182 failed decision/open cycles**. The failures split into two broad phases:

1. **00:30 to about 01:49 local**: AI transport failures (`context deadline exceeded`) repeatedly blocked any new open decision and triggered safe mode.
2. **From about 02:11 local onward**: most failures came from **AI-produced open decisions failing local validation**, mainly in `kernel/engine_analysis.go` and `kernel/engine_position.go`.

The dominant failure classes were:

| Failure class | Count |
|---|---:|
| Multiple protection routes in one decision | 44 |
| Invalidation misaligned with structural level | 41 |
| Opening amount too small | 29 |
| Too many resistance key levels | 21 |
| AI API timeout | 18 |
| Too many support key levels | 15 |
| Gross RR inconsistent with prices | 11 |
| Other structural validation edge cases | 3 |

## Evidence base

### SQL used

```sql
SELECT cycle_number, timestamp, error_message
FROM decision_records
WHERE timestamp >= '2026-04-24 16:30:00+00:00' AND success = 0
ORDER BY timestamp;
```

Classification was derived from `error_message`, and affected symbols were extracted from `raw_response` decision payloads.

### Log anchors

Representative log lines from `data/nofx_2026-04-25.log`:

- `1599`: `00:30:19 ... Execution failed: ... AI API call failed ... context deadline exceeded`
- `2266-2270`: `00:46:38 ... SAFE MODE ACTIVATED ... No new positions will be opened`
- `5336`: `02:11:35 ... gross_estimated_rr 2.79 inconsistent ... 15.60`
- `6645`: `02:42:12 ... key_levels resistance exceeds max 3`
- `7067`: `02:45:13 ... supports only one AI protection route at a time`
- `8421`: `02:57:08 ... ETHUSDT opening amount too small (40.00 USDT), must be ≥60.00 USDT`
- `9357`: `03:06:07 ... invalidation 358.0200 too far from structural support 356.2778`
- `41688`: `09:54:18 ... JSON numbers cannot contain thousand separator comma, found: 1,362.1395`

## Counts, examples, affected symbols, likely code paths

### 1. Multiple protection routes in one decision, 44
- Example cycle: **5952**, `2026-04-24 18:45:13+00:00`, symbol `STABLEUSDT`
- Example error: `current strategy route supports only one AI protection route at a time (full, ladder, or drawdown)`
- Most affected symbols: `RIVERUSDT(13)`, `ZECUSDT(9)`, `KATUSDT(7)`, `TRUMPUSDT(5)`, `STABLEUSDT(3)`
- Likely code path:
  - `kernel/engine_analysis.go:448`
  - surfaced by `trader/auto_trader.go` log entries such as line `7067`
- Interpretation: the AI returned an open decision containing more than one protection mode at once, and local validation rejected it before order placement.

### 2. Invalidation misaligned with structural level, 41
- Example cycle: **5959**, `2026-04-24 19:06:07+00:00`, symbol `ZECUSDT`
- Example error: `invalidation 358.0200 too far from structural support 356.2778`
- Most affected symbols: `ZECUSDT(18)`, `RIVERUSDT(9)`, `ORDIUSDT(7)`, `KATUSDT(2)`, `TRUMPUSDT(2)`
- Likely code path:
  - `kernel/engine_analysis.go:687`
  - related edge variant at `kernel/engine_analysis.go:712` for `must sit near/above resistance`
  - surfaced in log lines like `9357`, `13283`, `25926`, `51146`
- Interpretation: the AI chose stop/invalidation levels that did not sit close enough to the declared structural support/resistance anchors, so structurally-driven open entries were rejected.

### 3. Opening amount too small, 29
- Example cycle: **5956**, `2026-04-24 18:57:08+00:00`, symbol `ETHUSDT`
- Example error: `ETHUSDT opening amount too small (40.00 USDT), must be ≥60.00 USDT`
- Most affected symbols: `BTCUSDT(17)`, `ETHUSDT(12)`
- Likely code path:
  - `kernel/engine_position.go:65` for BTC/ETH-specific minimum
  - `kernel/engine_position.go:69` for general minimum
  - surfaced in log lines like `8421`, `10275`, `17035`, `24003`, `44252`
- Interpretation: the AI kept proposing BTC/ETH open sizes below the hard minimum notional threshold, so execution never reached exchange order submission.

### 4. Too many resistance key levels, 21
- Example cycle: **5951**, `2026-04-24 18:42:12+00:00`, symbol `RIVERUSDT`
- Example error: `entry_protection_rationale.key_levels resistance exceeds max 3`
- Most affected symbols: `RIVERUSDT(6)`, `ZECUSDT(6)`, `KATUSDT(4)`, `ORDIUSDT(3)`, `CHIPUSDT(2)`
- Likely code path:
  - validation block in `kernel/engine_analysis.go` near the structural rationale checks, surfaced by exact error text logged at `6645`
- Interpretation: the AI often returned 4+ resistance anchors in `entry_protection_rationale.key_levels.resistance`, exceeding the schema/validator limit.

### 5. AI API timeout, 18
- Example cycle: **5908**, `2026-04-24 16:30:19+00:00`
- Example error: `AI API call failed: failed to send request ... context deadline exceeded`
- Affected symbol: none, decision never arrived
- Likely code path:
  - `kernel/engine_analysis.go:90` wraps upstream AI request failure
  - `trader/auto_trader_loop.go:266-293` enters safe mode after repeated failures
  - corroborated by log lines `1599`, `1847`, `2083`, `2175`, `2266-2270`, `2838-2842`
- Interpretation: upstream AI requests timed out before any open candidate could be validated. This blocked opens directly and also caused safe-mode skips.

### 6. Too many support key levels, 15
- Example cycle: **5953**, `2026-04-24 18:48:02+00:00`, symbol `RIVERUSDT`
- Example error: `entry_protection_rationale.key_levels support exceeds max 3`
- Most affected symbols: `RIVERUSDT(6)`, `ZECUSDT(2)`, `KATUSDT(2)`, `STABLEUSDT(2)`
- Likely code path:
  - same structural validator area in `kernel/engine_analysis.go`
  - surfaced in log lines `7158`, `8003`, `16514`, `22064`, `48084`
- Interpretation: same pattern as resistance overflow, but on support anchors.

### 7. Gross RR inconsistent with prices, 11
- Example cycle: **5941**, `2026-04-24 18:11:35+00:00`, symbol `TRUMPUSDT`
- Example error: `gross_estimated_rr 2.79 inconsistent with entry/invalidation/first_target 15.60`
- Most affected symbols: `ZECUSDT(5)`, `ORDIUSDT(2)`, `TRUMPUSDT(1)`, `RIVERUSDT(1)`
- Likely code path:
  - `kernel/engine_analysis.go:606`
  - surfaced in log lines `5336`, `10689`, `21242`
- Interpretation: the AI-reported RR number often did not match the actual RR implied by entry, invalidation, and first target.

### 8. Other structural validation edge cases, 3
- Examples:
  - cycle **6075**, `RIVERUSDT`: `invalidation 6.5570 must sit near/above resistance 6.6010`
  - 2 JSON formatting failures where the AI emitted comma-separated numbers like `1,362.1395`
- Likely code path:
  - `kernel/engine_analysis.go:712` for resistance-side invalidation validation
  - `kernel/engine_analysis.go:381` for JSON numeric format validation
  - surfaced by log lines `36825`, `41688`, `49550`, `50364`

## Symbol impact summary

By failed open/decision attempts after 00:30 local, the most repeatedly affected symbols were:

- `ZECUSDT`, especially structural invalidation and resistance-overflow failures
- `RIVERUSDT`, especially multi-route, support/resistance overflow, and invalidation failures
- `BTCUSDT` and `ETHUSDT`, almost entirely from minimum opening size failures
- `KATUSDT`, `ORDIUSDT`, `TRUMPUSDT`, `STABLEUSDT` as recurring structural-validation candidates

## Most likely root causes

1. **Upstream AI reliability issue early in the window**
   - Concrete evidence: repeated `context deadline exceeded` errors, then safe-mode activation.
   - Primary files: `kernel/engine_analysis.go`, `trader/auto_trader_loop.go`.

2. **Prompt/output contract drift for structural-entry decisions**
   - Concrete evidence: repeated violations of mutually-exclusive protection routes, max-3 key-level limits, RR consistency, and structural invalidation placement.
   - Primary file: `kernel/engine_analysis.go`.

3. **AI sizing below hard notional floor for BTC/ETH**
   - Concrete evidence: 29 rejects, all on BTC/ETH, all below the `≥60 USDT` floor.
   - Primary file: `kernel/engine_position.go`.

## Bottom line

Today’s open failures after 00:30 local were **mostly pre-exchange validation failures**, not exchange rejections. The system was mainly blocked by:

- early **AI transport timeouts**, then
- repeated **AI output/schema violations for structural-entry fields**, and
- **undersized BTC/ETH position sizing** against the local minimum-open rule.
