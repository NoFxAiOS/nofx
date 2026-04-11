# Partial Drawdown Nativeization Notes

Date: 2026-04-11

## Current state
Native drawdown is already implemented for full-close (`close_ratio_pct ~= 100%`) on:
- Binance
- Bitget
- OKX

Partial drawdown (`close_ratio_pct < 100%`) still falls back to local runtime monitoring.

## Key architectural insight
Partial drawdown does NOT need a completely separate execution system.
The repo already has a normalized native protection representation:
- `ProtectionPlan`
- `ProtectionOrder`
- `placeAndVerifyLadderProtection(...)`

This means partial drawdown can potentially be expressed as a synthetic ladder/native plan when:
1. exchange supports native partial close
2. exchange supports enough stop/tp/algo semantics to preserve rule meaning
3. the mapping does not distort drawdown semantics beyond an acceptable threshold

## Proposed direction
### Phase A - normalized translation layer
Introduce a translation step:
- drawdown rule(s) -> partial native protection plan candidate

Conceptually:
- full-close drawdown -> native trailing stop (already landed)
- partial drawdown -> candidate ladder/native partial-close protection plan

### Phase B - safety gate
Only enable native partial drawdown when all are true:
- exchange `NativePartialClose == true`
- mapping is deterministic and reversible enough for verification
- verification can confirm all expected protection legs are present

### Phase C - fallback transparency
If native partial translation is not safe:
- keep local fallback
- expose that explicitly in UI (`native_full_local_partial` / similar mode)

## Important constraint
Drawdown semantics depend on runtime peak profit and subsequent retrace.
A naive translation from drawdown to fixed ladder prices may distort strategy meaning.
So the first native-partial implementation should likely be limited to:
- simple single-stage partial drawdown
- one derived protection leg
- exchanges with strong native partial-close support

## Immediate next coding step
1. add a helper that can build a candidate `ProtectionPlan` from drawdown rules
2. keep it disconnected from main trigger flow initially
3. add unit tests for plan generation semantics
4. only then wire it into runtime with safety checks
