# System Judgment Priority for Strategy Control

_Updated: 2026-04-20_

## Purpose

This document states the current priority order for system judgment over AI strategy-control proposals. It is implementation-grounded: it describes what the code can do now, where the hard gates are, and which fields remain advisory.

Related policy docs:

- `docs/STRATEGY_CONTROL_POLICY_2026-04-20.md`
- `docs/PROTECTION_RATIONALIZATION_COMPACT_CONTRACT_2026-04-20.md`

## Current implementation anchors

- AI response extraction and format validation: `kernel/engine_analysis.go`
- open-action entry/protection schema: `kernel/engine.go`
- deterministic execution-aware RR math: `kernel/risk_reward_execution.go`
- runtime open policy and mode behavior: `trader/runtime_open_policy.go`
- decision-cycle execution/audit assembly: `trader/auto_trader_loop.go`
- persisted compact review schema: `store/decision.go`
- strategy-control mode config: `store/strategy.go`

## Priority order

When an AI proposes an open action, authority flows in this order:

1. **AI may propose** a trade thesis and executable-looking fields.
2. **System must validate** structural legality, direction, risk/reward consistency, route requirements, and protection alignment checks that are implemented today.
3. **System must recompute** RR values when deterministic price geometry or trusted compact execution constraints are available.
4. **System may override** only derived audit/control fields and deterministic execution-constraint merges/recomputed RR.
5. **System must reject/block** unsafe or inconsistent open actions in the current hard gates; runtime blocking is mode-dependent.

The system must not trust an AI-supplied boolean, RR value, or protection claim when the same value can be checked from structured fields.

## Classification matrix

| Area / field / decision | AI may propose | System must validate | System must recompute | System may override | System must reject/block |
|---|---:|---:|---:|---:|---:|
| JSON decision payload and known action values | yes | yes | no | no | yes, if unparseable/unknown/malformed |
| `action`, `symbol`, `leverage`, `position_size_usd`, `stop_loss`, `take_profit`, `confidence`, `risk_usd` | yes | yes, via existing decision validation and execution path | no | no | yes, if existing validators/execution constraints fail |
| Open action requires `entry_protection_rationale` | yes | yes | no | no | yes, if missing |
| `risk_reward.entry`, `invalidation`, `first_target`, `gross_estimated_rr` | yes | yes, positive and directionally sane | gross RR from price geometry | runtime may replace RR after execution-aware recompute | yes, if invalid/inconsistent/below min |
| Direction geometry (`open_long` / `open_short`) | yes | yes | yes, from entry/invalidation/target ordering | no | yes, on mismatch |
| `risk_reward.net_estimated_rr` | yes, optional | yes when supplied, especially with execution constraints | yes when execution constraints are present | runtime may replace with recomputed net RR | yes, if inconsistent or effective RR below min in hard gate |
| `risk_reward.min_required_rr` | yes | yes, must match strategy min within `0.02` if supplied | strategy min comes from config, default `1.5` | no | yes, if inconsistent |
| `risk_reward.passed` | yes | yes, must agree with effective RR within `0.02` | effective pass/fail from min RR | audit builder may mark passed true if effective RR meets min | yes, if AI boolean contradicts kernel validation |
| `execution_constraints.*` in rationale | yes | yes, only if present and compact enough for RR math | price rounding / cost-adjusted RR | runtime may merge trusted snapshot if AI omitted fields | no solely for absence on weak venues; reject only if supplied values make RR inconsistent/below min |
| Runtime execution snapshot (`tick_size`, precision, fees/slippage, last price, etc.) | no, collected by system | yes, via compact trusted collector shape | yes, runtime RR | yes, merge into rationale/review context | strict mode blocks if runtime-effective RR below min |
| `protection_plan` route (`full`/`ladder`/`drawdown`) | yes when strategy route requires | yes, one AI route at a time and required shape checks | partial: full TP/SL percentages and BE/fallback relation to rationale | no silent semantic rewrite | yes, if required route/shape/alignment fails |
| Compact rationale context (`timeframe_context`, key levels, anchors, volatility/derivatives notes) | yes | lightly filtered/compacted for audit | no | compact top levels/anchors for review context | not currently hard-gated except through required RR/protection fields |
| Review/audit context (`review_context`) | no direct authority | yes, populated from accepted parsed decision/runtime data | yes for derived policy fields | yes, system creates compact audit summary | n/a |
| Control outcome (`control.decision`, reasons, failed checks, no-order flag) | no | yes, from runtime policy result | yes, from policy result | yes, system-authored | strict runtime rejection sets `no_order_placed` |

## Current hard validation status

Implemented today in `kernel.ValidateEntryProtectionRationale` and route validation:

- open actions require `entry_protection_rationale`;
- required RR fields must be positive;
- long requires `invalidation < entry < first_target`;
- short requires `invalidation > entry > first_target`;
- gross RR is recomputed from price geometry and must match AI `gross_estimated_rr` within `0.05`;
- effective RR uses `net_estimated_rr` when positive, otherwise gross RR;
- effective RR must meet `strategy.risk_control.min_risk_reward_ratio`, defaulting to `1.5`;
- supplied `min_required_rr` must match strategy min within `0.02`;
- supplied `passed` must agree with effective RR vs min RR within `0.02`;
- if execution constraints are present, validation rounds prices by `tick_size` or `price_precision`, recomputes gross/net RR, and rejects supplied net RR mismatches over `0.05`;
- full protection-plan stop/take-profit percentages are checked against rationale invalidation/first target when present;
- break-even trigger must not exceed first target threshold;
- fallback max-loss must not sit inside the rationale invalidation envelope under the implemented helper.

These kernel validation failures are hard failures before runtime policy mode can soften them.

## Current runtime policy status

Implemented today in `trader.applyRuntimeOpenPolicy`:

- applies only to `open_long` / `open_short`;
- merges runtime execution constraints into the AI rationale when available and missing;
- recomputes runtime gross/net RR from merged constraints;
- updates the in-memory rationale RR and `passed` flag after runtime recomputation;
- records compact control metadata in `review_context.control` when there is a policy effect or useful runtime/audit data;
- blocks order placement only when runtime-effective RR is below min **and** mode is `strict`.

Runtime control currently has one explicit failed check code: `runtime_rr_below_min`.

## Mode-specific behavior

Strategy-control mode is configured at `strategy_control_policy.mode` and resolved by `StrategyControlPolicyConfig.EffectiveMode()`:

- omitted mode: `strict`;
- unknown mode: `strict`;
- valid modes: `strict`, `audit_only`, `recommend_only`.

### `strict`

- Default and safest behavior.
- Kernel parse/schema/rationale failures reject the decision cycle.
- Runtime RR below min sets control decision to `rejected`, records `runtime_rr_below_min`, sets `no_order_placed`, and does not place the open order.

### `audit_only`

- Kernel parse/schema/rationale failures still reject the decision cycle.
- Runtime RR below min is flagged in control reasons/failed checks but does **not** block execution.
- Useful for measuring how often runtime policy would block without changing order flow.

### `recommend_only`

- Kernel parse/schema/rationale failures still reject the decision cycle.
- Current code behaves the same as `audit_only` for runtime blocking: low runtime RR is flagged but not blocked.
- The name is reserved for softer advisory behavior, but there is no distinct recommendation channel beyond current compact control audit fields yet.

## Current audit surface

Implemented compact audit fields include:

- `primary_timeframe`
- `min_risk_reward`
- `risk_reward.{entry,invalidation,first_target,gross_estimated_rr,net_estimated_rr,passed}`
- compact `key_levels.support/resistance`
- compact `anchors[]`
- `protection.{stop_beyond_invalidation,target_aligned,break_even_before_target,fallback_within_envelope,policy_status,policy_override,policy_rejected,policy_reasons,notes}`
- `execution_constraints.{tick_size,price_precision,qty_step_size,qty_precision,min_qty,min_notional,contract_value,last_price,fees,slippage,...}`
- `control.{decision,reasons,failed_checks,constraints_merged,runtime_rr_recomputed,ai_gross_rr,ai_net_rr,runtime_gross_rr,runtime_net_rr,effective_rr,effective_rr_source,execution_constraint_sources,no_order_placed}`

Not currently implemented as first-class audit fields:

- `original_action` / `final_action` differences for downgrade-to-wait;
- general downgrade-to-wait control branch;
- broad semantic equivalence proofs for ladder/drawdown protection;
- deep orderbook/liquidation/funding hard gates.

## Non-goals for current implementation

- Do not let AI decide whether its own RR check passed.
- Do not make missing weak-exchange execution metadata a hard failure by itself.
- Do not fetch live exchange data inside kernel validation.
- Do not promote noisy raw exchange payloads into prompts or audit records.
- Do not silently downgrade open actions to `wait` until explicit original/final action audit fields exist.
