# Strategy Control Policy: System-Governed Entry/Protection Decisions

_Updated: 2026-04-20_

## Purpose

This policy defines who has authority over an entry/protection decision after Protection Rationalization.

The AI may propose a trade thesis and structured rationale. The system must make the final executable judgment by validating, recomputing, overriding, or refusing unsafe parts with deterministic code and compact audited data.

This is an implementation-grounded policy for the current codebase, not a future design wishlist.

## Current implementation anchors

Authoritative paths today:

- AI decision parsing and validation: `kernel/engine_analysis.go`
- entry rationale schema: `kernel/engine.go`
- execution-aware RR recomputation: `kernel/risk_reward_execution.go`
- compact execution snapshot collection: `trader/execution_constraints_snapshot.go`
- decision-cycle persistence/audit context: `trader/auto_trader_loop.go`
- persisted action audit shape: `store/decision.go`

Current hard gate:

- `kernel.ValidateEntryProtectionRationale`

Current compact audit surface:

- `store.DecisionActionReviewContext`
- `decision_records.decisions[].review_context`

## Authority boundary

### AI may propose

For `open_long` / `open_short`, the AI may propose:

- action, symbol, leverage, position size, stop/take-profit fields;
- `protection_plan` where required by the active strategy route;
- `entry_protection_rationale` with:
  - timeframe context;
  - key support/resistance levels;
  - volatility notes;
  - risk/reward entry, invalidation, first target, gross/net RR;
  - compact execution constraints if already known;
  - compact anchors and alignment notes.

The proposal is advisory until the system accepts it.

### System must verify/recompute

For every open action, the system must verify at least the currently implemented contract:

1. `entry_protection_rationale` exists.
2. `risk_reward.entry`, `invalidation`, `first_target`, and `gross_estimated_rr` are positive.
3. Direction matches action:
   - long: `invalidation < entry < first_target`
   - short: `invalidation > entry > first_target`
4. Gross RR is recomputed from price geometry and must match within current tolerance `0.05`.
5. Effective RR must meet `strategy.risk_control.min_risk_reward_ratio`, falling back to `1.5`.
6. If `net_estimated_rr > 0`, net RR becomes the effective gate.
7. If compact execution constraints are present, RR is recomputed using implemented price rounding plus fee/slippage adjustment.
8. If `min_required_rr` is supplied, it must match the strategy minimum within current tolerance `0.02`.
9. `risk_reward.passed` must agree with the effective RR gate within the current `0.02` band.
10. Implemented protection alignment checks must not contradict rationale invalidation / first target.

### System may override

The system may override only data it can derive deterministically from trusted runtime context. Current examples:

- compact execution constraints can be merged into the rationale when the AI omitted them and a runtime snapshot exists;
- action review context can mark RR `passed=true` when effective RR meets the system minimum even if the AI did not set the boolean;
- compact protection alignment booleans are derived from the active protection snapshot, not trusted from AI text.

The system must not silently invent market facts, fees, precision, or venue limits.

## Reject vs downgrade-to-wait policy candidates

Current runtime behavior is still primarily **reject-on-invalid open contract**: validation errors fail the AI decision cycle and are persisted as an error. This is safer than executing a structurally inconsistent open.

Future downgrade-to-wait behavior should be added only as an explicit, audited strategy-control path. Candidate policy:

### Must reject the whole AI response / cycle

Reject when the response cannot be safely interpreted or could hide ambiguous intent:

- invalid JSON / unparseable decision payload;
- unknown action value;
- malformed open action that fails required schema after parsing;
- open action direction mismatch;
- gross RR materially inconsistent with entry/invalidation/target;
- effective RR below strategy minimum;
- supplied net RR inconsistent with system recomputation;
- protection plan contradicts invalidation or first target under implemented checks;
- leverage, position sizing, or action fields violate existing hard validation limits.

### May downgrade one open action to `wait`

Downgrade is acceptable only when all of the following can be audited:

- the original action is a single open proposal for a known symbol;
- the failure is localized to entry/protection quality, not parse integrity;
- no close/urgent risk-management action is being suppressed;
- the downgraded record preserves the original proposed action and failure reason;
- no order is placed after downgrade.

Candidate downgrade reasons:

- missing optional compact execution data on a weak exchange, while pure price geometry is otherwise safe but not strong enough for the selected strategy mode;
- stale or unavailable runtime quote data where the configured policy prefers waiting rather than failing the entire cycle;
- confidence/protection rationale too weak for open, but existing position handling can continue independently.

### Audit requirement for downgrade

A downgrade-to-wait implementation must persist at least:

- `original_action`
- `final_action: wait`
- `control_decision: downgraded_to_wait`
- `control_reasons[]`
- `failed_checks[]`
- `no_order_placed: true`

Until these fields exist, hard rejection remains the clearer current behavior for invalid opens.

## Binance/OKX-first execution-aware rules

Execution-aware control must stay narrow and venue-trusted.

### Binance

Preferred compact fields when available:

- `tick_size`
- `price_precision`
- `qty_step_size`
- `qty_precision`
- `min_qty`
- `min_notional`
- `last_price`
- top-of-book `best_bid`, `best_ask`, `spread_bps` only when cheaply available and compact

### OKX

Preferred compact fields when available:

- `tick_size`
- `qty_step_size`
- `min_qty`
- `contract_value`
- `last_price`
- top-of-book `best_bid`, `best_ask`, `spread_bps` only when cheaply available and compact

### Other exchanges

Other venues degrade cleanly:

- do not fabricate Binance/OKX-grade metadata;
- leave unavailable fields empty/zero;
- allow gross geometry validation to remain legal when compact constraints are absent;
- do not block Phase 1/2 solely because the exchange has a weak execution profile.

### Current recomputation rule

When execution constraints are present:

- round prices by `tick_size`, else by `price_precision`;
- recompute gross RR from rounded prices;
- subtract entry+exit slippage and fee cost from reward distance when fee/slippage fields exist;
- never fetch live exchange data inside kernel validation.

## Required audit fields for system decisions

Existing compact action audit fields that should remain first-class:

- `primary_timeframe`
- `min_risk_reward`
- `risk_reward.entry`
- `risk_reward.invalidation`
- `risk_reward.first_target`
- `risk_reward.gross_estimated_rr`
- `risk_reward.net_estimated_rr`
- `risk_reward.passed`
- `key_levels.support[]` / `key_levels.resistance[]` compacted to top levels
- `anchors[]` compacted to the most useful items
- `protection.stop_beyond_invalidation`
- `protection.target_aligned`
- `protection.break_even_before_target`
- `protection.fallback_within_envelope`
- `protection.notes[]`
- `execution_constraints.*` only for compact trusted fields

Additional fields required before system-controlled final judgment becomes fully explainable:

- `control_decision`: `accepted`, `rejected`, `downgraded_to_wait`, `overridden`
- `control_reasons[]`: short human-readable reasons
- `failed_checks[]`: machine-stable check names
- `original_action` and `final_action` when they differ
- `effective_rr` and `effective_rr_source`: `net`, `gross`, or `execution_recomputed_net`
- `recomputed_gross_rr` and `recomputed_net_rr` when execution recomputation ran
- `execution_constraints_source`: compact source map such as `binance:instrument`, `okx:ticker`, `okx:top_of_book`
- `no_order_placed` for rejected/downgraded opens

These fields should be added as compact structured fields, not as raw JSON dumps.

## Strict no-noise data principle

Do not add data because it exists. Add it only if it is:

1. trusted or explicitly sourced;
2. compact enough for prompt/audit use;
3. directly actionable for entry/protection/execution control;
4. recomputable or checkable by system code;
5. useful for explaining the final system decision.

Excluded by default:

- broad orderbook ladders;
- liquidation dumps;
- large OI/funding windows;
- raw exchange payloads;
- unverifiable external fields;
- UI raw JSON dumps as the primary explanation.

If a field is not trusted, not compact, or not actionable, it must not become a hard dependency.

## Practical near-term implementation checklist

To move from current rationale validation to full system-controlled final judgment without changing the compact design:

1. Keep current hard rejection for structurally invalid opens.
2. Add explicit control-decision audit fields before implementing downgrade-to-wait.
3. Make downgrade-to-wait an audited policy branch, not an implicit validation side effect.
4. Preserve Binance/OKX as first-class execution constraint sources.
5. Keep weaker exchanges degraded but legal when pure price geometry validates.
6. Never promote noisy, unverifiable data into prompts, validators, or hard gates.
