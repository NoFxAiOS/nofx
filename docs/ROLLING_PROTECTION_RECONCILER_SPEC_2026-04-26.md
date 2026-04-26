# Rolling Protection Reconciler Spec - 2026-04-26

## Goal

Drawdown and ladder take-profit protection must be active as exchange-side orders immediately after opening a position, then adapt during held-position analysis without naked transition windows.

This is not a runner-first architecture. Runner-like behavior should emerge from:

- open-time drawdown tiers,
- rolling ladder take-profit tiers,
- dynamic ladder stop-loss movement,
- fallback max-loss protection.

## Non-negotiable invariants

1. **Open-time drawdown arming**
   - Drawdown tiers are placed after open; `min_profit_pct` is an activation threshold, not a condition for submitting the order.

2. **Add before remove**
   - New desired protection tiers must be placed and verified before obsolete tiers are canceled.

3. **Bridge tier preservation**
   - Rolling upgrades must keep an overlapping middle/bridge tier when possible.
   - Example: current `DD1, DD2`, desired `DD2, DD3` means: add `DD3`, keep `DD2`, cancel `DD1` only after `DD3` is verified.

4. **No naked profit-protection window**
   - During drawdown / ladderTP migrations, at least one effective profit-side protection tier must remain active if any existed before.

5. **No silent AI-mode downgrade**
   - If AI-mode protection lacks required fields at open time, reject by default.
   - Exception: if a rare data issue prevents AI protection but deterministic gates still judge the trade valid, fallback may be used, but it must be explicit, recorded, surfaced in review/UI, and counted.
   - Repeated fallback is a bug signal: prompt, schema, validator, or data plumbing must be fixed.

6. **Held-position stale-safe behavior**
   - If held-position AI update fails or lacks a new desired set, keep the last verified protection set rather than deleting or downgrading it.

## Rolling drawdown example

Current active tiers:

- `DD1`: profit `1.0%`, drawdown `40%`
- `DD2`: profit `1.5%`, drawdown `50%`

New structure supports a higher tier:

- `DD2`: profit `1.5%`, drawdown `50%`
- `DD3`: profit `2.5%`, drawdown `40%`

Migration plan:

1. Add `DD3`.
2. Verify `DD3` visible on exchange.
3. Keep `DD2` as bridge.
4. Cancel `DD1` only after `DD3` is verified.

If adding `DD3` fails, keep `DD1` and `DD2` unchanged.

## LadderTP rolling logic

LadderTP follows the same rolling migration semantics as drawdown:

- add new upper TP tier first,
- verify it,
- keep bridge tier,
- then cancel obsolete lower tier.

## Dynamic ladderSL logic

Stop side is more conservative:

- long stop-loss may only move up,
- short stop-loss may only move down,
- fallback max-loss must not be canceled before replacement protection is verified.

## Fallback policy

Fallback is allowed only as an explicit degraded route:

- `source=fallback_exception`, not `source=normal`;
- must carry reason: `missing_ai_fields`, `market_data_unavailable`, `exchange_temporarily_unavailable`, etc.;
- must be review-visible;
- repeated fallback within a rolling window should trigger a system issue marker.

Fallback is not a substitute for making AI protection fields work.
