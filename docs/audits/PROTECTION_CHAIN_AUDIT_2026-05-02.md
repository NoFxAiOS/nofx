# Protection Chain Audit — 2026-05-02

## Why this pass happened

Live ETHUSDT/TRBUSDT protection behavior showed the protection chain was not honoring the intended ownership model:

- Break-even was not clearly selected by its own manual/AI setting.
- AI ladder values were being lost or mixed with configured 0.9% / 1.5% default ladder percentages.
- Configured AI reference rules could materialize as real exchange orders.
- Drawdown tiers could all become same-timeframe 100% exits.
- Bad AI protection output could reject otherwise acceptable entries instead of falling back safely.
- OKX non-executable protection orders could cause retry/reconcile noise.
- Natural-language `~` inside AI reason strings could trigger JSON validation failure.

Representative live case: ETHUSDT position id `163`, decision cycle `9282`. AI provided structural protection, but live exchange orders were fixed-percent ladder SLs (`2266.49`, `2280.29`) plus native trailing, not the intended AI structural plan.

## Ownership model now enforced

Each protection leg chooses its source independently:

- **Break-even**
  - `manual`: use strategy `trigger_value` / `offset_pct`.
  - `ai`: use AI `break_even_trigger_*` fields when present, otherwise fallback to strategy safety.
- **Ladder**
  - `manual`: materialize configured ladder rules.
  - `ai`: configured 0.9/1.5-style reference rules are not materialized; AI decision plan supplies real ladder orders.
- **Drawdown**
  - `manual`: materialize configured rules.
  - `ai`: restore/use per-position AI drawdown rules; do not silently substitute configured default rules for an active position.
- **Fallback max-loss**
  - Preserved as manual safety even when ladder/full modes are AI-owned and skipped.

## Change groups

### Kernel / AI parsing and route handling

Files:

- `kernel/engine.go`
- `kernel/engine_analysis.go`
- `kernel/engine_prompt.go`
- `kernel/schema_alias_helpers.go`
- `kernel/schema_hardening_test.go`
- `kernel/protection_decision_test.go`
- `kernel/protection_plan_structure_validation_test.go`

Changes:

- Added ladder ratio alias handling for model outputs using:
  - `take_profit_ratio_pct`
  - `stop_loss_ratio_pct`
  - `tp_ratio_pct`
  - `sl_ratio_pct`
- Added `firstPositiveFloat()` helper for direct alias fallback.
- Changed `~` validation to reject only range symbols outside JSON strings, allowing natural-language reason text.
- Prompt now distinguishes manual vs AI break-even behavior.
- Kernel requires AI BE fields only when BE mode is AI.
- Bad/missing AI drawdown/combined protection structure now warns instead of hard-rejecting otherwise good open proposals.
- Added drawdown structure warnings/tests for all-100% / repeated same-stage tiers.

### Store / configuration model

File:

- `store/strategy.go`

Changes:

- Added `BreakEvenStopConfig.Mode` with manual default.
- This makes BE ownership explicit and consistent with ladder/drawdown.

### Trader execution / reconciler / runtime

Files:

- `trader/protection_execution.go`
- `trader/protection_plan.go`
- `trader/protection_phase3.go`
- `trader/protection_reconciler.go`
- `trader/protection_owner_policy.go`
- `trader/auto_trader_risk.go`
- `trader/partial_drawdown_native.go`

Changes:

- AI decision protection plan replaces configured AI reference ladder/full percentages instead of merging them into live protection.
- Manual fallback max-loss is extracted and preserved even when AI-owned configured protection legs are skipped.
- BE execution and reconciliation now use AI plan values only when BE mode is AI; otherwise strategy manual BE wins.
- AI ladder absolute structural prices are not double-buffered; volatility buffer is applied only to percent-derived local fallbacks.
- Drawdown runner policy (`min_runner_keep_pct`, `max_first_reduce_pct`, `break_even_runner_policy`) is enforced for managed close paths too.
- AI drawdown in active-position runtime restores per-position AI rules; configured default rules are not silently substituted for AI-owned active positions.
- Protection execution validates live mark/market price before placing SL/TP, dropping non-executable tiers.
- OKX-style non-retryable protection rejects (`trigger price must be less/greater than last price`, code `51280`) stop immediate retry loops.
- Reconciler can verify held TP direction executability and uses plan-specific BE config.

### Web UI

Files:

- `web/src/types/strategy.ts`
- `web/src/components/strategy/ProtectionEditor.tsx`

Changes:

- Added BE `mode` field to frontend types.
- Added BE manual/AI selector in Protection Editor.

### Tests updated / added

Files:

- `trader/protection_combined_ai_test.go`
- `trader/protection_execution_test.go`
- `trader/protection_reconciler_test.go`
- `trader/protection_plan_test.go`
- `trader/auto_trader_risk_test.go`
- `trader/position_protection_runtime_test.go`
- kernel tests listed above

Coverage added for:

- Combined AI ladder + drawdown + BE preservation.
- Configured AI-owned ladder not materializing 0.9/1.5 defaults.
- BE manual mode vs AI mode selection.
- AI absolute ladder price not double-buffered.
- Ladder ratio aliases mapping into close ratios.
- Non-executable XAG-style protection tiers dropped before placement.
- Non-retryable OKX protection rejects stopping retry storms.
- Managed fallback after native trailing callback safety rejection.
- Drawdown runner policy enforcement.

## Verification performed

Latest passing gates:

```bash
go test ./kernel ./trader/okx ./trader
go build ./...
cd web && npm run build
```

## Current live-operation note

Code changes do **not** automatically repair already-live exchange orders. Existing ETHUSDT live protection still had old orders in DB inspection:

- `2266.49` ladder/full SL
- `2280.29` ladder/full SL
- `2323.55` native trailing

Before deployment/observation, decide whether to:

1. Leave existing ETHUSDT protection untouched and let only future opens use the new chain.
2. Manually/operationally cancel and rebuild ETHUSDT protection from the entry decision.
3. Only add/verify fallback or BE without disturbing current SLs.

## Suggested commit message

```text
fix(protection): honor AI/manual ownership across BE ladder drawdown

- add break-even mode and UI selector; use AI BE only when selected
- prevent configured AI ladder/full references from materializing as live percent orders
- preserve manual fallback max-loss when AI-owned protection legs are skipped
- prefer AI decision protection plan without merging stale configured 0.9/1.5 ladder defaults
- keep AI absolute ladder prices unchanged and accept ratio aliases from model output
- warn, sanitize, and fallback for weak AI drawdown/combined protection instead of rejecting good opens
- enforce runner keep / first reduce policy on managed drawdown paths
- drop non-executable protection tiers before placement and stop retrying non-retryable OKX rejects
- relax `~` JSON validation to allow natural-language reason strings
- add regression coverage for BE mode, AI ladder aliasing, drawdown runner policy, and XAG/OKX protection rejects
```

## Remaining follow-up

- Commit these changes after review.
- Deploy/restart runtime.
- Observe next open for:
  - AI structural ladder prices instead of 0.9/1.5 defaults.
  - correct BE source by mode.
  - fallback max-loss preserved.
  - drawdown runner tiers not all 100% same-timeframe exits.
- Decide operational treatment for currently live ETHUSDT protection.
