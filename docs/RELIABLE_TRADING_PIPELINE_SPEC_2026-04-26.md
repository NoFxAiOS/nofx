# Reliable Trading Pipeline on Current nofxmax Skeleton

Date: 2026-04-26
Status: working spec v1 — implementation must proceed in small gated phases

## Goal

Build a more reliable trading system on top of the current nofxmax skeleton, not by replacing it.

Reliability here means:

- low-frequency, high-quality entries;
- data completeness and staleness controls;
- AI uses a strict language and defaults to WAIT;
- code validators outrank AI text;
- real orders are protected immediately after fill;
- full / ladder / drawdown / break-even have explicit ownership and do not delete each other accidentally;
- every decision is auditable and replayable.

This document is not a promise of stable profit. It is a design to reduce avoidable losses from bad entries, missing data, naked positions, duplicated protection, stale state, and AI hallucinated protection plans.

## Current Skeleton to Preserve

Existing nofxmax modules already provide most of the skeleton:

- `market/*`: Kline, structure, Fibonacci, indicators, hot coins, OKX market client.
- `provider/*`: external data providers such as CoinAnk, nofxos, Hyperliquid.
- `kernel/*`: strategy engine, prompt building, AI decision parsing, protection contract validation.
- `trader/auto_trader_loop.go`: periodic strategy loop and decision orchestration.
- `trader/auto_trader_orders.go`: open / close order execution.
- `trader/protection_plan.go`: strategy protection config materialization.
- `trader/protection_execution.go`: applying full/ladder/fallback protection after open.
- `trader/auto_trader_risk.go`: drawdown and break-even runtime monitoring.
- `trader/protection_reconciler.go`: reconciliation of exchange protection orders.
- `trader/okx/*`: OKX order, position, sync, and protection adapter.
- `store/*`: strategy config, decision records, positions, fills.
- `api/*`: config, decisions, open orders, test-run endpoints.

We should extend these seams, not create a parallel bot.

## End-to-End Pipeline

```text
candidate universe
  -> market context v2
  -> regime classifier
  -> setup gate
  -> AI decision schema v2
  -> deterministic validators
  -> execution preflight
  -> open order
  -> post-fill protection engine
  -> protection reconciler / risk monitors
  -> exit / partial exit / realized RR
  -> decision record / review context / replay
```

## Data Layer

### Required Timeframes

Use current 15m/3m/1h flow, but add higher-timeframe context:

- 1D: broad market risk and major trend.
- 4H: main swing trend / high-level range.
- 1H: intraday directional bias.
- 15m: setup structure and protection planning anchor.
- 3m: execution trigger only, never standalone direction.

Implementation seams:

- Extend `kernel/engine_analysis.go` and `kernel/prompt_builder.go` to request/include 1D and 4H context.
- Extend `market/timeframe.go` if timeframe constants are incomplete.
- Store a compact summary in `decision_records.review_context` before any behavior change.

### Derivatives / Crowding Context

Add a compact derivatives context per candidate:

```json
{
  "oi": 0,
  "oi_change_15m_pct": 0,
  "oi_change_1h_pct": 0,
  "funding_rate": 0,
  "funding_bias": "neutral|long_crowded|short_crowded|unknown",
  "mark_index_premium_pct": 0,
  "volume_zscore": 0,
  "squeeze_risk": "low|medium|high|unknown",
  "data_quality": "ok|partial|stale|missing"
}
```

Free data sources, in priority order:

1. OKX if available in existing adapters.
2. Existing `provider/coinank/*` OI/liquidation/net-position providers.
3. `provider/nofxos/*` OI/netflow if available.
4. Binance public market endpoints as a fallback for common symbols.

Initial rollout: record-only. Do not let this context open trades until validators are tested.

## Regime Classifier

The first decision is not long/short. It is whether the market is tradable.

Regime values:

- `trend_up`
- `trend_down`
- `range`
- `squeeze`
- `chop`
- `news_risk`
- `no_trade`

Bias values:

- `long`
- `short`
- `neutral`

Allowed setup types:

- `trend_pullback`
- `range_edge`
- `breakout_retest`
- `none`

Hard principles:

- 1D/4H/1H disagreement lowers score.
- 15m middle-of-range normally forces WAIT.
- 3m may confirm trigger but cannot justify a trade alone.
- incomplete/stale data forces WAIT or lowers confidence depending on severity.

Implementation seams:

- `kernel/engine_analysis.go`: build compact regime evidence.
- `kernel/schema.go`: add optional `regime`, `setup_type`, `quality_score` fields.
- `kernel/*_test.go`: parse and validate schema without breaking old responses.

## AI Decision Language v2

AI output should be strict JSON inside the existing decision extraction contract.

Minimum v2 fields for open actions:

```json
{
  "symbol": "SOLUSDT",
  "action": "open_long|open_short|hold|wait",
  "regime": "trend_up|trend_down|range|squeeze|chop|news_risk|no_trade",
  "setup_type": "trend_pullback|range_edge|breakout_retest|none",
  "confidence": 0,
  "quality_score": {
    "total": 0,
    "trend_alignment": 0,
    "structure_location": 0,
    "sr_fib_quality": 0,
    "derivatives_context": 0,
    "trigger_quality": 0,
    "net_rr": 0
  },
  "leverage": 1,
  "position_size_usd": 0,
  "risk_usd": 0,
  "stop_loss": 0,
  "take_profit": 0,
  "entry_protection_rationale": {
    "timeframe_context": {
      "macro": ["1d", "4h"],
      "bias": "1h",
      "primary": "15m",
      "trigger": "3m"
    },
    "risk_reward": {
      "entry": 0,
      "invalidation": 0,
      "first_target": 0,
      "gross_estimated_rr": 0,
      "net_estimated_rr": 0,
      "min_required_rr": 2.5,
      "passed": false
    },
    "anchors": [],
    "derivatives_notes": [],
    "alignment_notes": []
  },
  "protection_plan": {}
}
```

Backwards compatibility:

- Existing schema remains accepted.
- Missing v2 fields initially degrade to record-only warnings.
- Execution gates should only become hard after shadow data is collected.

## Deterministic Validators

AI may propose. Code decides whether execution is allowed.

Hard gates for new opens:

- confidence >= 75.
- setup_type is one of the three allowed setup types.
- net RR >= 2.5 after fees/slippage assumptions.
- no range-middle entry.
- direction sanity:
  - long: `stop < entry < target`.
  - short: `target < entry < stop`.
- stop is beyond structural invalidation plus buffer.
- 4H and 1H are not both strongly against the trade.
- data is not missing/stale for required market fields.
- spread/depth/min-notional constraints pass.
- daily/weekly loss limits and cooldowns pass.

Implementation seams:

- Existing `kernel/risk_reward_execution.go` and tests.
- Existing `trader/runtime_open_policy.go` and tests.
- Add `trader/trade_quality_validator.go` only if existing files become too large.

## Execution Pipeline

Open sequence should remain in existing `auto_trader_orders.go`, but enforce two-stage protection:

1. Preflight builds expected static protection plan.
2. Open order is submitted.
3. Actual fill / position is synced.
4. Protection is recalculated from actual entry and qty.
5. Full/ladder/fallback protection is placed.
6. Exchange visibility is verified.
7. Position state becomes protected only after verification.

Failure rules:

- If static SL/fallback cannot be placed, retry.
- If still failing, degrade to full SL.
- If no protective stop can be placed, enter emergency path: close or safe-mode based on configured risk tolerance.
- Never mark a position protected because reconciliation returned nil.

## Four Protection Systems

### Full Protection

Manual mode:

- deterministic TP/SL/fallback percentages or prices.
- immediate post-fill exchange orders.

AI mode:

- AI provides one stop and one target from structure.
- code validates direction, RR, min distance, and structure alignment.
- if invalid, degrade to manual fallback if configured; otherwise WAIT before opening.

### Ladder Protection

Manual mode:

- deterministic multi-leg TP/SL.
- exchange-min filters; non-executable legs degrade to full/fallback.

AI mode:

- AI provides structural TP/SL tiers:
  - TP1: nearest liquidity / local structure.
  - TP2: primary 15m target.
  - TP3: extension / 1H level.
  - SL: invalidation plus buffer.
- code validates close ratios, direction, min quantity, and structure consistency.

### Drawdown Protection

Manual mode:

- min profit / max drawdown / close ratio rules.
- activation only after profit condition is satisfied.

AI mode:

- AI proposes activation, callback, full vs partial, close ratio, and rationale based on volatility, target proximity, OI/funding, and regime.

Ownership rule:

- drawdown starts as `pending`.
- drawdown may only remove/replace TP-side orders after native trailing or managed drawdown is actually visible/armed.
- drawdown must never remove fallback stop.

### Break-even Protection

Manual mode is sufficient for now.

- trigger profit pct.
- offset pct.
- only affects stop side.
- must avoid duplicate reapply.
- can be suppressed by a stronger runner/drawdown stop.

## Protection Ownership Model

For every active position maintain an explicit runtime view:

```json
{
  "symbol": "TRUMPUSDT",
  "side": "short",
  "fingerprint": "symbol|side|entry|qty",
  "static_owner": "full|ladder|fallback|none",
  "profit_owner": "full_tp|ladder_tp|drawdown|none",
  "stop_owner": "full_sl|ladder_sl|breakeven|fallback|none",
  "state": "unprotected|protecting|protected|degraded|closing|closed",
  "last_verified_at": 0
}
```

Verified means:

- position exists;
- at least one real protective stop/fallback is visible or a stronger native protection is verifiably armed;
- if the strategy requires TP/profit ownership, a TP/trailing/runner owner is visible/armed;
- stale/legacy orders are not masquerading as current owners;
- open orders = 0 cannot be `verified` unless position is closed or an exchange-native protection endpoint confirms protection.

Important: this must be implemented as a tested ownership model, not as scattered `if` patches in reconciler.

## Reconciler Rules

The reconciler should be boring and conservative:

- rebuild expected ownership from strategy config + runtime state + exchange orders;
- compare expected vs observed;
- repair only one direction at a time;
- use cooldowns to avoid API storms;
- never delete TP merely because drawdown is enabled;
- delete/rewrite TP only when drawdown is armed;
- never delete fallback SL while position exists;
- never log `exchange protection verified` if no verified owner exists.

Implementation seam:

- `trader/protection_reconciler.go` should eventually delegate to a small pure function:
  - input: position, strategy config, runtime state, open orders, capabilities.
  - output: expected owners, observed owners, action list.

This pure function must have matrix tests before live behavior changes.

## Test Matrix

Minimum matrix before changing live protection behavior:

| Case | Full | Ladder | Drawdown | BE | Expected |
| --- | --- | --- | --- | --- | --- |
| long full manual | manual | off | off | off | full TP/SL visible |
| short full manual | manual | off | off | off | full TP/SL visible |
| long ladder manual | off | manual | off | off | ladder visible |
| short ladder manual | off | manual | off | off | ladder visible |
| ladder tiny qty | off | manual | off | off | degrade to full/fallback |
| full AI valid | ai | off | off | off | full plan accepted |
| full AI invalid RR | ai | off | off | off | WAIT/reject |
| ladder AI valid | off | ai | off | off | structural tiers accepted |
| ladder AI invalid tiers | off | ai | off | off | degrade/reject |
| drawdown manual pending | full/ladder | any | manual | off | static TP retained |
| drawdown manual armed | full/ladder | any | manual | off | profit side owned by drawdown |
| drawdown AI pending | full/ladder | any | ai | off | static TP retained |
| drawdown AI armed | full/ladder | any | ai | off | drawdown owns profit side |
| breakeven trigger | any | any | any | manual | stop side moves once |
| restart with open position | any | any | any | any | ownership rebuilt, no naked verified |
| openOrders=0 active position | any | any | any | any | unprotected / repair / fail, never verified |
| position closed orders remain | any | any | any | any | orphan cleanup |
| OKX visibility lag | any | any | any | any | retry/cooldown, no duplicate storm |

## Phased Implementation Plan

### Phase 0 — Freeze and Document

- Keep current live behavior stable.
- Do not modify execution semantics.
- Add this spec and a current-code mapping.

Acceptance:

- docs only.
- `go test ./...` optional but recommended.

### Phase 1 — Market Context v2 Record-Only

- Add 1D/4H context.
- Add derivatives context if available.
- Store in `review_context`.
- Prompt may mention it, but validators do not hard-block yet.

Acceptance:

- no change to orders/protection.
- decision_records include v2 context.

### Phase 2 — AI Schema v2 Shadow Mode

- Extend schema/parser for regime/setup/quality/net RR.
- Record validation warnings.
- No live execution behavior change except rejecting malformed opens already rejected today.

Acceptance:

- existing tests pass.
- old AI outputs still accepted.
- v2 fixture parses.

### Phase 3 — Deterministic Entry Gates

- Enforce WAIT for low confidence, low RR, stale data, middle range, and cooldown.
- Start conservative.

Acceptance:

- open attempts reduce, not increase.
- no protection behavior changes.

### Phase 4 — Protection Ownership Pure Model

- Add pure ownership evaluator and matrix tests.
- No live behavior until tests cover the cases above.

Acceptance:

- matrix tests pass.
- reconciler still unchanged or only logs evaluator comparison.

### Phase 5 — Protection Engine v2 Rollout

Order:

1. full manual.
2. ladder manual.
3. break-even manual.
4. drawdown manual.
5. full AI.
6. ladder AI.
7. drawdown AI.

Each subphase requires:

- focused tests;
- `go test ./trader/...`;
- `go test ./...`;
- `go build -o nofx .`;
- explicit restart confirmation;
- new-cycle verification.

### Phase 6 — Replay / Review Loop

- Save signal quality, validator decisions, protection ownership, exits, realized RR.
- Build replay fixtures from live decisions.
- Use data to tighten thresholds.

## Immediate Next Code-Safe Task

Do not touch live protection semantics yet.

Next safe task:

1. Add current-code mapping doc.
2. Add test matrix skeleton file or TODO test list.
3. Add no behavior-changing types for MarketContextV2 if needed.

Only after that should implementation begin.
