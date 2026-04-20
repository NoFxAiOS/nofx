# Protection Rationalization Integration Directive

_Updated: 2026-04-20 21:44 Asia/Shanghai_

## User principle

Data is not better simply because there is more of it. Extra noisy fields can degrade AI judgment.

Prioritize valuable, decision-relevant, auditable data.

Near-term exchange focus:

1. Binance
2. OKX

Other exchanges should degrade cleanly and should not block Phase 1/2.

Longer-term goal:

- This work must support a future precisely controlled complete strategy design.
- Therefore data contracts, validators, and audit surfaces should be deterministic, compact, explainable, and not overstuffed.

## Integration priorities

### Priority 1 — Connect kernel rationale to audit persistence/UI

- Map `kernel.Decision.EntryProtection` / `entry_protection_rationale` into `store.DecisionAction.ReviewContext`.
- Keep action-level review compact:
  - primary timeframe
  - min RR
  - gross/net RR
  - entry / invalidation / first target
  - pass/fail
  - top support/resistance
  - compact anchors
  - protection alignment summary when available
- Avoid dumping full raw JSON into UI.

### Priority 2 — Fix tests after new open-action contract

- Existing tests with open actions must either include valid `entry_protection_rationale` or assert the new error envelope.
- Preserve legal `[]` no-trade and `wait` behavior.

### Priority 3 — Binance/OKX-first market data capability layer

Implement narrowly and conservatively:

- `MarketDataCapabilities` parallel to `ProtectionCapabilities`.
- Conservative profiles for Binance and OKX first.
- Other exchanges get weak/degraded profiles.
- Do not add noisy data to prompts by default.

Valuable first fields:

- tick size / price precision
- qty step / qty precision
- min qty / min notional or min size
- contract value for OKX
- last/mark price
- best bid / best ask / spread if readily available
- fee/slippage fallback from config or safe defaults

Avoid first-wave prompt bloat:

- no broad orderbook ladders
- no liquidation dumps
- no excessive OI/funding windows
- no raw data that cannot be traced or validated

### Priority 4 — Execution constraints as optional compact context

- Add execution constraints only when source quality is known.
- Include provenance/source map where possible.
- If data missing, leave empty and use gross RR fallback.
- Do not fabricate precision or fee data.

### Priority 5 — Later alignment checks

After A/C integration and tests pass:

- protection plan vs invalidation consistency
- first target vs take-profit consistency
- break-even trigger before first target
- rounding-aware net RR

## Coordination note

Use subagents for parallel inspection/fixes, but parent must integrate and commit coherent changes.
