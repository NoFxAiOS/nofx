# Structural Entry Contract (2026-04-21)

This document defines the next-stage contract for discretionary-style AI opens that rely on:

- primary timeframe
- adjacent lower/higher timeframes
- support / resistance
- fibonacci (only when required by strategy)
- structural invalidation

The goal is **strong purpose, not maximum data**.

## 1. Principle

Do not collect or prompt every available market field.
The system should request and validate only the smallest set of exchange/runtime-derived structure needed to justify:

- why the entry exists;
- where invalidation lives;
- why the first target is structurally plausible;
- whether adjacent timeframes confirm or contradict the setup.

## 2. Strategy surface

Strategy config now exposes `entry_structure`:

- `enabled`
- `require_primary_timeframe`
- `require_adjacent_timeframes`
- `require_support_resistance`
- `require_structural_anchors`
- `require_fibonacci`
- `max_support_levels`
- `max_resistance_levels`
- `max_anchor_count`

This lets Strategy Studio decide how strict the open-action structure contract should be.

## 3. Current runtime behavior

When `entry_structure.enabled=true`, backend validation now rejects open actions missing required structure fields.

Current enforced fields (depending on strategy toggles):

- `timeframe_context.primary`
- at least one adjacent timeframe in `lower[]` or `higher[]`
- `key_levels.support[]` and `key_levels.resistance[]`
- `anchors[]`
- `key_levels.fibonacci` with `swing_high`, `swing_low`, and `levels[]` when fibonacci is required

## 4. Data discipline

The system should prefer exchange/runtime data that supports these structural judgments without noise.

Useful:

- selected multi-timeframe klines
- compact execution constraints
- mark / last / bid / ask when relevant to execution-aware RR
- compact volatility context

Not useful as default hard dependencies:

- full orderbook ladders
- raw liquidation streams
- oversized OI/funding windows
- indiscriminate indicator dumps

## 5. Expected AI behavior

For open actions, the AI should:

1. identify the primary timeframe;
2. reference at least one adjacent confirming timeframe;
3. name the specific support/resistance or breakout/retest zone;
4. explain invalidation as a structural failure, not just a numeric stop;
5. only include fibonacci when it materially affects the setup;
6. output `wait` / `[]` if required structure evidence is missing.

## 6. UI / backend closure status

Implemented on 2026-04-21:

- backend `entry_structure` config in strategy model
- kernel validation for required structural fields
- Strategy Studio UI editor for `entry_structure`
- Strategy save payload coverage
- prompt updates instructing compact, purpose-driven structural evidence

Still desirable follow-up:

- tighter semantic validation of anchor quality / level count caps
- strategy-aware trimming of oversized support/resistance/fibonacci payloads
- explicit runtime collection notes showing which exchange-derived data fed the structure judgment
