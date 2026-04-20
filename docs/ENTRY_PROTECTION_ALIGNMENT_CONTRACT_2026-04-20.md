# Entry-Protection Alignment Contract

_Updated: 2026-04-20_

This document describes the **current implemented contract** for Protection Rationalization / Entry-Protection Alignment.

It is not a wishlist. It records what open-action decisions are already expected to provide, what the backend currently validates, what compact execution data is intentionally carried, and what audit fields are surfaced to persistence/API/UI.

## 1. Scope

Applies to AI decisions with action:

- `open_long`
- `open_short`

It does **not** apply to:

- `hold`
- `wait`
- close actions
- empty decision lists (`[]`)

For non-open actions, `entry_protection_rationale` is not required and should not be treated as part of the action contract.

## 2. Required open-action rationale fields

Open actions must include `entry_protection_rationale`.

Current kernel type: `kernel.AIEntryProtectionRationale`

Implemented structure:

- `timeframe_context`
  - `primary`
  - `lower[]`
  - `higher[]`
- `key_levels`
  - `support[]`
  - `resistance[]`
  - `swing_highs[]`
  - `swing_lows[]`
  - `fibonacci`
- `volatility_adjustment`
  - `atr14_pct`
  - `boll_width_pct`
  - `market_regime`
  - `widening_pct`
- `risk_reward`
  - `entry`
  - `invalidation`
  - `first_target`
  - `gross_estimated_rr`
  - `net_estimated_rr`
  - `min_required_rr`
  - `passed`
- `execution_constraints`
  - compact venue/execution fields only
- `derivatives_context`
  - optional compact derivatives context
- `anchors[]`
  - compact rationale anchors
- `alignment_notes[]`
  - short alignment notes

### Actually required today for validation

The backend currently requires the following for open actions:

- `entry_protection_rationale` present
- `risk_reward.entry > 0`
- `risk_reward.invalidation > 0`
- `risk_reward.first_target > 0`
- `risk_reward.gross_estimated_rr > 0`

Everything else in the rationale is currently optional at validator level.

### Strongly expected even when optional

Although not hard-required by the validator, these fields are part of the implemented compact audit shape and should be treated as first-class when available:

- `timeframe_context.primary`
- top `key_levels.support` / `key_levels.resistance`
- `anchors[]`
- `alignment_notes[]`
- `execution_constraints` when sourced reliably from exchange/runtime

## 3. RR, direction, and consistency checks

Current validator: `kernel.ValidateEntryProtectionRationale`

### 3.1 Direction sanity

For `open_long`:

- `invalidation < entry`
- `first_target > entry`

For `open_short`:

- `invalidation > entry`
- `first_target < entry`

If these conditions do not hold, the open action is rejected as directionally inconsistent.

### 3.2 Gross RR consistency

The backend recomputes gross RR from:

- `entry`
- `invalidation`
- `first_target`

Formula:

- `gross_rr = abs(first_target - entry) / abs(entry - invalidation)`

The provided `gross_estimated_rr` must stay close to the recomputed value.

Current tolerance:

- reject if deviation is greater than `0.05`

### 3.3 Strategy minimum RR gate

The effective minimum RR comes from:

- `strategy.risk_control.min_risk_reward_ratio`

Current fallback if unset/non-positive:

- `1.5`

The action is rejected if effective RR is below the minimum.

### 3.4 Gross vs net RR behavior

Current effective RR selection:

- use `net_estimated_rr` if it is present and positive
- otherwise fall back to `gross_estimated_rr`

That means the gate is already net-first when net RR is supplied.

### 3.5 `min_required_rr` consistency

If `risk_reward.min_required_rr` is provided, it must match the active strategy minimum RR closely.

Current tolerance:

- reject if absolute deviation is greater than `0.02`

### 3.6 `passed` consistency

`risk_reward.passed` must agree with the effective RR outcome.

Current behavior:

- reject if `passed=true` but effective RR is materially below min RR
- reject if `passed=false` but effective RR materially meets/exceeds min RR

Current tolerance band:

- `0.02`

## 4. Execution-constraint philosophy (compact, Binance/OKX-first)

Current design goal is **not** to feed the model every available exchange datum.

The implemented philosophy is:

- keep execution context compact
- carry only high-confidence, execution-relevant fields
- prefer deterministic values already available from the adapter/runtime
- do not fabricate venue rules, fees, or quote fields
- degrade cleanly on weaker exchanges

### 4.1 Current capability layer

Runtime capability helper:

- `trader.MarketDataCapabilities`

Current posture:

- **Binance**: strongest first-wave support
- **OKX**: strongest first-wave support, including contract-value awareness
- other exchanges: degraded profile, compact subset only

### 4.2 Current compact execution fields

Implemented execution constraint shape:

- `tick_size`
- `price_precision`
- `qty_step_size`
- `qty_precision`
- `min_qty`
- `min_notional`
- `contract_value` (audit/review surface; especially relevant for OKX)
- `mark_price`
- `last_price`
- `best_bid`
- `best_ask`
- `spread_bps`
- `taker_fee_rate`
- `maker_fee_rate`
- `estimated_slippage_bps`

Current collection intentionally excludes broader noisy datasets such as:

- orderbook ladders
- liquidation streams
- raw OI/funding windows in the execution snapshot
- fabricated exchange limits

### 4.3 Current degraded-mode rule

If compact execution fields are not available with enough confidence, the system leaves them empty.

Current practical effect:

- open-action validation still works from pure price geometry (`entry/invalidation/first_target`)
- gross RR remains sufficient
- net RR recomputation is only attempted when enough execution fields are present

This means missing execution data does **not** block Phase 1/2 open decisions.

## 5. Rounding-aware and execution-aware RR checks

Current helper:

- `kernel.recomputeRiskRewardWithExecutionConstraints`

When execution constraints contain enough data, the backend recomputes RR using:

- rounded entry/invalidation/target
- fee estimate (`taker_fee_rate`, fallback to `maker_fee_rate`)
- `estimated_slippage_bps`

### 5.1 Price rounding

Current rounding behavior:

- prefer `tick_size`
- otherwise use `price_precision`
- otherwise leave prices unchanged

### 5.2 Net RR recomputation

Current net-RR adjustment is intentionally simple and auditable:

- total slippage cost assumes entry + exit
- total fee cost assumes entry + exit
- cost is subtracted from reward distance only

This is a compact approximation, not a full execution simulator.

### 5.3 Current validator use

If execution constraints are sufficient:

- gross RR is recomputed after rounding
- net RR is recomputed when fee/slippage fields exist
- provided `net_estimated_rr` must stay close to recomputed net RR

Current tolerance:

- reject if net RR deviation is greater than `0.05`

## 6. Protection-plan alignment checks currently implemented

Current alignment validation is intentionally narrow.

Validator helper:

- `kernel.validateProtectionPlanAlignmentSkeleton`

### 6.1 Full/default protection mode

If `protection_plan.mode` is empty or `full`, and corresponding values are present:

- `protection_plan.stop_loss_pct` must align with rationale invalidation distance
- `protection_plan.take_profit_pct` must align with rationale first-target distance

Current tolerance:

- reject if deviation is greater than `0.05` percentage points

### 6.2 What this means in practice

The current validator is checking that:

- the rationale’s invalidation is not saying one thing while full-stop placement says another
- the rationale’s first target is not saying one thing while full TP says another

This is a **skeleton alignment check**, not a full protection-plan verifier.

## 7. Audit surface currently persisted and exposed

The implemented persistence/UI contract is deliberately compact.

Primary storage shape:

- `store.DecisionActionReviewContext`

### 7.1 Compact review fields

Current compact action-level audit fields:

- `primary_timeframe`
- `min_risk_reward`
- `risk_reward`
  - `entry`
  - `invalidation`
  - `first_target`
  - `gross_estimated_rr`
  - `net_estimated_rr`
  - `passed`
- `key_levels`
  - compact top `support[]`
  - compact top `resistance[]`
- `anchors[]`
  - compact rationale anchors
- `protection`
  - `stop_beyond_invalidation`
  - `target_aligned`
  - `break_even_before_target`
  - `fallback_within_envelope`
  - `notes[]`
- `execution_constraints`
  - compact venue/execution fields only

### 7.2 Compaction rules already reflected in code

Current review-context building intentionally compresses data:

- support/resistance are compacted to the top two valid levels
- anchors are compacted to at most three items
- alignment notes are compacted to a short list
- execution constraints are omitted entirely when empty

### 7.3 Protection snapshot linkage

The decision record also stores a separate protection snapshot:

- full TP/SL config snapshot
- ladder TP/SL snapshot and rules
- drawdown snapshot
- break-even snapshot

This allows review surfaces to compare:

- rationale-side intent
- configured protection-side mechanics

### 7.4 Derived alignment audit fields

Current trader-side review context derives compact booleans from rationale + protection snapshot:

- whether stop appears beyond invalidation
- whether target appears aligned with first target
- whether break-even trigger appears before first target
- whether fallback max-loss remains within invalidation envelope

These are **audit booleans**, not the full execution contract.

## 8. Intentionally out of scope for now

The following are deliberately **not** part of the current required contract:

### 8.1 Broad prompt/data expansion

Not in scope now:

- large raw orderbook payloads
- liquidation dumps
- heavy multi-window derivatives payloads by default
- noisy exchange-specific raw fields without clear audit value

### 8.2 Full protection-plan semantic verification

Not in scope now:

- exhaustive ladder-rule validation against every target/invalidation scenario
- full drawdown-rule semantic validation against ATR/BOLL regime
- full break-even strategy optimization or sequencing validation
- complete fallback-loss envelope theory beyond compact audit checks

### 8.3 Full execution simulation

Not in scope now:

- venue-specific fill simulation
- side-aware maker/taker path modeling
- deep slippage modeling from orderbook depth
- exact min-notional / min-size rejection modeling across every venue path

### 8.4 Mandatory execution constraints on every exchange

Not in scope now:

- blocking open decisions because an exchange cannot provide strong execution metadata
- forcing weak exchanges to provide Binance/OKX-grade precision/quote data

## 9. Practical contract summary

For an open action to be acceptable under the current implementation:

1. `entry_protection_rationale` must be present.
2. `risk_reward.entry`, `invalidation`, `first_target`, and `gross_estimated_rr` must be positive.
3. Direction must match the action.
4. RR derived from price geometry must be internally consistent.
5. Effective RR must meet strategy minimum RR.
6. If net RR is provided, it becomes the effective gating RR.
7. If execution constraints are provided, net/gross RR may be recomputed and must remain consistent.
8. If a full/default protection plan is provided, its TP/SL percentages must not contradict rationale invalidation / first target.
9. Audit persistence should stay compact and reviewable rather than storing or rendering large raw rationale payloads.

That is the current contract baseline for Protection Rationalization / Entry-Protection Alignment.
