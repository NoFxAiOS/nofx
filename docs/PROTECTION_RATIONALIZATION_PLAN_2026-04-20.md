# Protection Rationalization / Entry-Protection Alignment Plan

_Updated: 2026-04-20_

## Why this exists

The protection execution stack is now much stronger than the earlier baseline:

- Full / Ladder / fallback / drawdown / break-even roles are clearer.
- Drawdown and break-even have AI contract routes.
- Protection source and status are visible in runtime and partially in historical views.
- Reconciler and cleanup are more role-aware.

However, a higher-level quality question remains:

> Are the entry and protection points actually reasonable relative to real K-line structure, support/resistance, fibonacci levels, volatility, and RR?

The answer today is: execution is much better, but the reasoning evidence and RR gate are not yet structured enough.

This document defines the next implementation direction.

---

## Current data sufficiency

The current strategy/input pipeline is sufficient for AI to reason about real market structure, provided the configured data switches are enabled:

- Raw OHLCV K-lines via `enable_raw_klines`
- Primary timeframe via `indicators.klines.primary_timeframe`
- Multi-timeframe context via `selected_timeframes` / `longer_timeframe`
- EMA / RSI / ATR / BOLL
- OI / funding / quant data when enabled
- Existing risk settings, including `risk_control.min_risk_reward_ratio`

Recent runtime logs show active strategies using, for example:

- primary timeframe: `15m`
- adjacent/larger frames: `3m`, `1h`

So the data is not the blocker. The blocker is that key-level and RR reasoning are not yet structured, validated, persisted, and shown consistently.

---

## Target outcome

For every `open_long` / `open_short` decision, AI must output a structured entry-protection rationale that includes:

1. Timeframe context
2. Key market levels
3. Volatility adjustment
4. Invalidation level
5. First target / protection target
6. Estimated RR
7. Protection plan alignment

The backend must validate this before execution.

If the decision does not satisfy minimum RR or has inconsistent protection levels, the open action should be rejected or downgraded to wait.

---

## Proposed structure

Add a structured object alongside or inside `AIProtectionPlan`, tentatively named:

- `AIProtectionRationale`
- or `AIRiskRewardPlan`

Suggested JSON:

```json
{
  "timeframe_context": {
    "primary": "15m",
    "lower": ["3m", "5m"],
    "higher": ["1h"]
  },
  "key_levels": {
    "support": [123.4, 121.8],
    "resistance": [128.0, 130.5],
    "fibonacci": {
      "swing_high": 129.2,
      "swing_low": 118.4,
      "levels": [0.382, 0.5, 0.618]
    }
  },
  "volatility_adjustment": {
    "atr14_pct": 1.8,
    "widening_pct": 0.25
  },
  "risk_reward": {
    "entry": 124.0,
    "invalidation": 121.6,
    "first_target": 128.8,
    "estimated_rr": 2.0
  },
  "anchors": [
    "15m primary structure breakout retest",
    "5m continuation support",
    "1h resistance overhead"
  ]
}
```

Important: these values must be derived from the actual K-line / indicator context provided to the model, not invented as generic numbers.

---

## Validation rules

### Required for open actions

For `open_long` / `open_short`, require:

- `protection_plan`
- structured key-level/RR rationale
- `risk_reward.entry > 0`
- `risk_reward.invalidation > 0`
- `risk_reward.first_target > 0`
- `risk_reward.estimated_rr > 0`

### Direction sanity

For long:

- invalidation < entry
- first_target > entry

For short:

- invalidation > entry
- first_target < entry

### Minimum RR gate

Use:

- `strategy.risk_control.min_risk_reward_ratio`

If absent or zero, choose a conservative default such as `1.5`.

Reject or downgrade open action if:

```text
estimated_rr < min_risk_reward_ratio
```

### Protection alignment sanity

Check that protection plan and rationale are not contradictory:

- Full/Ladder stop should be near or beyond invalidation side.
- Drawdown first profit gate should not be below trivial noise relative to ATR.
- Break-even trigger should not be beyond the first target.
- Fallback max-loss should be no looser than an extreme invalidation envelope unless explicitly justified.

---

## Prompt / contract changes

Update:

- `kernel/prompt_builder.go`
- `kernel/engine_prompt.go`
- `kernel/engine_analysis.go`
- `kernel/protection_reasoning_contract.go`

Prompt must state:

- Use actual provided primary and adjacent timeframe K-lines.
- Identify support / resistance / swing high / swing low from supplied data.
- If using Fibonacci, specify swing anchors.
- Volatility widening/tightening must be small and justified by ATR/BOLL/market regime.
- Do not output open action if RR is below configured minimum.
- Do not output empty key-level/risk plan in AI protection modes.

---

## Execution layer changes

Update:

- `trader/protection_phase3.go`
- `trader/protection_plan.go`
- `trader/protection_execution.go`

Execution should:

1. Parse AI key-level/RR rationale.
2. Validate it before placing orders.
3. Attach rationale metadata to the merged protection plan where useful.
4. Refuse or downgrade unsafe open decisions.

---

## Persistence / audit

Update:

- `store/decision.go`
- `trader/auto_trader_loop.go`
- `api/handler_order.go`

Persist key-level/RR rationale in:

- `decision_records.review_context`, or
- a new field under `ProtectionSnapshot`, or both.

Recommended additions to `ProtectionSnapshot`:

```json
{
  "risk_reward": {
    "entry": 124.0,
    "invalidation": 121.6,
    "first_target": 128.8,
    "estimated_rr": 2.0,
    "min_required_rr": 1.5,
    "passed": true
  },
  "key_levels": {
    "primary_timeframe": "15m",
    "support": [123.4, 121.8],
    "resistance": [128.0, 130.5],
    "fibonacci": {...},
    "volatility_adjustment": {...}
  }
}
```

---

## Frontend / UX

Update:

- `web/src/types/trading.ts`
- `web/src/components/trader/PositionProtectionPanel.tsx`
- `web/src/components/trader/PositionHistory.tsx`

Display concise audit data:

- RR badge: `RR 2.1 / min 1.5`
- Primary timeframe: `15m`
- Key support/resistance used
- Protection source: `AI` / `Strategy`
- Whether RR gate passed

Keep UI concise. Do not show long JSON. Use small chips and expandable details.

---

## Implementation phases

### Phase 1 — Contract skeleton

- Add data structures for key-level/RR rationale.
- Update prompt/schema.
- Add parser validation and negative tests.

### Phase 2 — RR gate

- Compute/validate RR directionally.
- Enforce `min_risk_reward_ratio`.
- Reject/downgrade invalid open decisions.

### Phase 3 — Persistence

- Store rationale and RR gate result in decision records / protection snapshot.
- Add store round-trip tests.

### Phase 4 — UI audit view

- Show concise RR/key-level badges in current and historical views.
- Avoid expensive frontend calculations; backend should provide normalized fields.

### Phase 5 — Protection alignment checks

- Validate ladder/full/drawdown/break-even plans against key-level rationale.
- Add trader-level tests.

---

## Important engineering guidance

- Do not rely only on natural-language reasoning.
- Do not let AI output empty rationale in AI protection modes.
- Keep frontend light: backend should normalize fields.
- Avoid frequent polling or expensive chart recomputation.
- Keep backward compatibility with existing protection configs.
- Full side toggles are currently UI/store-level; backend execution semantics remain backward-compatible until a proper three-state migration exists.

---

## Current best next task

Start next session with:

> Implement Protection Rationalization / Entry-Protection Alignment Phase 1: add structured key-level and RR rationale to AI open decisions, update prompt/schema/validation, and add negative tests.
