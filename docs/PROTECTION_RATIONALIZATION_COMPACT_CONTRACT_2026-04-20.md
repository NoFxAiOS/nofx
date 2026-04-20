# Protection Rationalization / Entry-Protection Alignment Compact Contract

_Updated: 2026-04-20_

## Purpose

This document records the compact, implementation-grounded contract that now exists for entry/protection rationalization in `nofxmax`.

It is intentionally narrow:

- prioritize valuable, auditable fields over more fields;
- keep Binance / OKX as the first-class execution-constraint sources;
- degrade safely when trusted execution data is unavailable;
- avoid prompt bloat and avoid unverifiable market-data dumps.

## Current scope

Implemented scope today covers:

1. structured open-action rationale;
2. RR / direction / consistency validation;
3. compact execution-constraint collection and RR-aware validation;
4. compact audit persistence and history display.

This is not yet a full protection-semantic theorem prover. It is a compact hardening layer.

## Required open-action rationale

For `open_long` / `open_short`, AI decisions must include:

- `entry_protection_rationale`

Current rationale structure includes:

- `timeframe_context`
- `key_levels`
- `volatility_adjustment`
- `risk_reward`
- `execution_constraints`
- `derivatives_context`
- `anchors`
- `alignment_notes`

### Minimum practical fields required today

Validation currently relies on these `risk_reward` fields:

- `entry`
- `invalidation`
- `first_target`
- `gross_estimated_rr`

`net_estimated_rr` is optional but, when present, is validated more strictly if execution constraints are available.

## Direction / RR hard checks

For open actions:

### Long

- `invalidation < entry < first_target`

### Short

- `invalidation > entry > first_target`

### Minimum RR

Effective RR uses:

- `net_estimated_rr` when present and validated;
- otherwise `gross_estimated_rr`.

Minimum RR source:

- `strategy.risk_control.min_risk_reward_ratio`
- fallback default: `1.5`

### Consistency checks

The validator now enforces:

- `min_required_rr` must match strategy min RR within tolerance;
- `passed` must agree with effective RR vs min RR;
- `gross_estimated_rr` must remain roughly consistent with rationale price geometry;
- when execution constraints are present, rounded/cost-adjusted RR is used for stricter consistency checks.

## Compact execution constraints philosophy

Execution constraints are optional and compact.

They exist to answer one question:

> does this rationale still make sense under trusted venue execution limits?

### First-class near-term exchanges

1. Binance
2. OKX

### Compact fields only

Preferred high-value fields:

- `tick_size`
- `price_precision`
- `qty_step_size`
- `qty_precision`
- `min_qty`
- `min_notional` (only when venue-derived and trusted)
- `contract_value` (OKX)
- `last_price`
- `mark_price` when compact and trusted
- `taker_fee_rate` / `maker_fee_rate` when explicitly known
- `estimated_slippage_bps` when explicitly configured or provided

### Intentionally excluded from first-wave prompt/audit expansion

- broad orderbook ladders
- liquidation dumps
- excessive OI/funding windows
- unverifiable external fields with unclear provenance

## Binance / OKX-first collector behavior

### Binance

Current compact collection path may populate:

- tick size
- price precision
- qty step size
- qty precision
- min qty
- min notional
- last price

### OKX

Current compact collection path may populate:

- tick size
- qty step size
- min qty
- contract value
- last price

Both degrade safely:

- missing fields remain empty/zero;
- no fake values should be fabricated;
- gross RR fallback remains legal when compact constraints are absent.

## Rounding-aware RR validation

When execution constraints are present inside the rationale:

- prices are rounded using `tick_size` first, else `price_precision`;
- optional cost effect is applied only when fee/slippage fields already exist in the rationale;
- no live exchange calls are made inside kernel validation.

This produces a compact recomputed RR used for:

- gross RR consistency checking;
- net RR consistency checking when `net_estimated_rr` is supplied.

## Protection alignment checks currently in scope

Current safe, narrow checks include:

- `full` protection plan stop-loss percentage vs rationale invalidation distance;
- `full` protection plan take-profit percentage vs rationale first-target distance;
- break-even trigger must not exceed rationale first-target threshold;
- configured fallback max-loss must not sit inside the rationale invalidation envelope.

These checks are intentionally tolerant and compact.

## Audit surface contract

Compact rationale/audit data is persisted per action under:

- `decision_records.decisions[].review_context`

Current compact audit fields include:

- `primary_timeframe`
- `min_risk_reward`
- `risk_reward.{entry,invalidation,first_target,gross_estimated_rr,net_estimated_rr,passed}`
- `key_levels.{support,resistance}`
- `anchors[]`
- `protection.{stop_beyond_invalidation,target_aligned,break_even_before_target,fallback_within_envelope,notes}`
- `execution_constraints.{tick_size,qty_step_size,min_qty,min_notional,contract_value,last_price,taker_fee_rate,maker_fee_rate,estimated_slippage_bps,...}`

History/UI should display this compactly and never dump raw JSON by default.

## Out of scope for now

Not yet intentionally covered as hard validation:

- full ladder semantic equivalence checks;
- drawdown multi-stage semantic consistency checks;
- broad multi-source derivatives-context hard gating;
- deep orderbook / liquidity / liquidation reasoning in prompt contract;
- fully generalized exchange capability normalization beyond Binance / OKX-first compact paths.

## Design rule

If a field is not trusted, not compact, or not actionable, it should not become a hard dependency.

This rationalization layer exists to make entry/protection logic:

- more deterministic,
- more auditable,
- more execution-aware,
- without turning the system into a noisy data sink.
