# Minimum opening amount / order sizing root cause (2026-04-25)

## Executive summary

There is a real mismatch between three layers:

1. **Kernel validation** hard-rejects BTC/ETH opens below **60 USDT** and alts below **12 USDT**.
2. **Prompt sizing guidance** tells the AI only **`Min Position Size: ≥12 USDT`**, with no BTC/ETH exception.
3. **Runtime execution** also only enforces the configurable `risk_control.min_position_size` (default/current strategy: **12**), while the exchange layer only checks a generic Binance min-notional of **10 USDT**.

Because of that mismatch, the AI is being *guided* to emit 34-50 USDT BTC/ETH openings, then the kernel rejects them before execution.

For small-balance accounts, the current behavior is especially bad: if equity is around **68 USDT**, the prompt literally teaches the model that BTC/ETH max size is 68 and min size is 12, so confidence-based sizing naturally lands in the **34-54 USDT** range, which is invalid for BTC/ETH under kernel rules.

## Where the `>= 60 USDT` rule is enforced

### 1) Kernel decision validation, hard rejection

File: `kernel/engine_position.go`

```go
const minPositionSizeGeneral = 12.0
const minPositionSizeBTCETH = 60.0

if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
    if d.PositionSizeUSD < minPositionSizeBTCETH {
        return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
    }
} else {
    if d.PositionSizeUSD < minPositionSizeGeneral {
        return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.PositionSizeUSD, minPositionSizeGeneral)
    }
}
```

This is the actual blocker for BTC/ETH 34-50 USDT opens.

### 2) Runtime trader enforcement, but only generic min size

File: `trader/auto_trader_risk.go`

```go
minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
if minSize <= 0 {
    minSize = 12
}
if positionSizeUSD < minSize {
    return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum (%.2f USDT)", positionSizeUSD, minSize)
}
```

This layer does **not** know about BTC/ETH `60`. It only knows the generic config min.

### 3) Exchange/API layer only checks generic min-notional

Files:
- `trader/binance/futures_orders.go`
- `trader/binance/futures_positions.go`

Relevant behavior:
- quantity is formatted to symbol precision
- then Binance min-notional is checked via `CheckMinNotional`
- current code returns a fixed conservative default of **10 USDT**

```go
func (t *FuturesTrader) GetMinNotional(symbol string) float64 {
    return 10.0
}
```

So execution-layer reality is closer to **10-12 USDT**, not 60 USDT.

## Where the AI is being nudged into 34-50 USDT openings

### Prompt hard-constraints section is misleading for BTC/ETH

File: `kernel/engine_prompt.go`

Current prompt generation includes:

```go
sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))
```

This uses only `riskControl.MinPositionSize`.

For the live failing trader, the strategy config has:
- `risk_control.min_position_size = 12`
- `risk_control.btc_eth_max_position_value_ratio = 1`
- account equity around **68 USDT**

So the system prompt stored in `decision_records.system_prompt` says:

- `Position Value Limit (BTC/ETH): max 68 USDT (= equity 68 × 1.0x)`
- `Min Position Size: ≥12 USDT`
- sizing guidance:
  - high confidence: 80-100% of max
  - medium confidence: 50-80% of max
  - low confidence: 30-50% of max

With confidence in the mid-70s, the model is rationally driven toward roughly:
- `68 × 0.5 = 34`
- `68 × 0.7 ≈ 48`
- `68 × 0.74 ≈ 50`

That exactly matches the bad outputs seen in logs and DB.

## Evidence from live records

Trader:
- `traders.id = 8f43a158_fc0b7412-8c02-4c43-b3ad-cf29ba3ee28c_openai_1775840538`
- `strategy_id = 49f1cf5c-e165-4b00-b9f6-257dd8f21b5b`
- `initial_balance ≈ 72.03`

Strategy config (`strategies.id = 49f1cf5c-e165-4b00-b9f6-257dd8f21b5b`) includes:
- `btc_eth_max_position_value_ratio = 1`
- `altcoin_max_position_value_ratio = 1`
- `min_position_size = 12`
- `btc_eth_max_leverage = 1`

Observed failing outputs in `decision_records` / `data/nofx_2026-04-25.log`:

- BTCUSDT `position_size_usd = 34`
- BTCUSDT `position_size_usd = 42`
- BTCUSDT `position_size_usd = 45`
- BTCUSDT `position_size_usd = 50`
- ETHUSDT `position_size_usd = 34, 38, 40, 41, 42, 48`

Representative failure:

- `decision_records.id = 8800`
- prompt says BTC/ETH max = 68, min = 12
- AI output: `BTCUSDT`, `position_size_usd = 34`
- failure: `BTCUSDT opening amount too small (34.00 USDT), must be ≥60.00 USDT`

So the 34-50 outputs are not random. They are the direct result of current prompt math.

## Did the `>= 60 USDT` rule change recently?

Yes, but not very recently.

The key change was commit:
- `7027d7a2 refactor(decision): relax minimum position size constraints for flexibility`

That commit changed:
- **before**: BTC/ETH `100`, altcoins `15`
- **after**: BTC/ETH `60`, altcoins `12`

It also changed the prompt layer from an explicit BTC/ETH-vs-alt minimum to a simplified generic recommendation:
- old prompt: `BTC/ETH ≥100 USDT | altcoins ≥15 USDT`
- new prompt: generic `≥12 USDT`

This is the root of today’s mismatch: the validation layer kept a BTC/ETH special case, but the prompt layer stopped teaching it.

Later refactors only moved the code:
- `cb31782b refactor: split large files and clean up project structure`

I did **not** find evidence that the `60` rule changed again after that. The current breakage is mainly from **layer drift**, not a fresh change to the threshold itself.

## Why small-balance accounts are broken by design right now

For a small account, these conditions can coexist:

- prompt says BTC/ETH max position value = equity × ratio
- prompt says BTC/ETH min = 12
- kernel says BTC/ETH min = 60

If equity is near 60-70 USDT and BTC/ETH ratio is 1x, then:
- prompt max may be only 60-70
- confidence-based guidance will suggest positions in the 30s, 40s, or low 50s
- kernel rejects them all

That means the model keeps seeing BTC/ETH as eligible, but execution cannot accept the recommended size band.

This is a policy contradiction, not an AI quality problem.

## Robust fix recommendation

### Recommendation: replace hardcoded BTC/ETH `60` logic with executable-min-size logic derived from execution constraints, and align prompt/runtime/kernel to the same source of truth

### Patch plan

#### A. Introduce a single sizing floor function

Create one shared function used by:
- kernel validation
- prompt generation
- runtime order execution

It should compute **required minimum executable notional** for a symbol using:
1. venue `min_notional` when available
2. symbol `min_qty`, `qty_step_size`, `last_price` / `mark_price`
3. configurable safety buffer (for example 10-20%)
4. optional strategy-level absolute override, if the user really wants a stricter floor

Conceptually:

`effective_min_open_usdt = max(strategy_floor, venue_min_notional, rounded_min_qty_notional) + buffer`

For BTC/ETH this will usually still be above tiny alt amounts, but it should be **symbol/exchange-derived**, not a permanent magic `60`.

#### B. Make prompt constraints symbol-aware

In `kernel/engine_prompt.go`, stop emitting only:
- `Min Position Size: ≥12 USDT`

Instead emit something like:
- `Min Position Size (general): ≥12 USDT`
- `Executable floor for BTCUSDT today: ≥X USDT`
- `Executable floor for ETHUSDT today: ≥Y USDT`
- if equity or max-position cap is below executable floor, explicitly say: `Do not open BTC/ETH this cycle`

This is the most direct fix for the 34-50 USDT outputs.

#### C. Gate impossible symbols before model choice, not after

If:

`max_position_value_for_symbol < effective_min_open_usdt`

then remove that symbol from open-candidate consideration or explicitly annotate it as:
- `open disabled this cycle: account too small for executable minimum`

For the observed 68 USDT account, BTC/ETH should probably be auto-disabled when the executable floor plus buffer exceeds feasible size.

#### D. Unify runtime enforcement with kernel enforcement

`trader/auto_trader_risk.go:enforceMinPositionSize()` must use the same symbol-aware floor logic as kernel validation.

Right now runtime enforces 12 while kernel enforces 60 for BTC/ETH. That split is brittle and guarantees future drift.

#### E. Prefer venue facts over asset-class heuristics

The current `BTC/ETH => 60` rule is a blunt proxy for:
- price precision
- lot precision
- min qty rounding risk

But the code already has execution-constraint plumbing (`min_notional`, `min_qty`, `qty_step_size`, `last_price`). The robust design is to base the floor on those facts.

#### F. Optional product decision: explicit “small account mode”

If the product wants predictable behavior for tiny accounts, add a mode such as:
- disable BTC/ETH opens below a configured equity threshold, or
- prefer only symbols whose executable minimum fits within e.g. `<= 70%` of current max-position cap

That gives a cleaner UX than repeated AI proposals followed by hard rejection.

## Minimal safe patch order

1. **Fix prompt text first** so AI stops proposing invalid BTC/ETH 34-50 amounts.
2. **Refactor min-size calculation into one shared function**.
3. **Make kernel + runtime both call it**.
4. **Use execution constraints / venue minima instead of hardcoded 60 where available**.
5. **Skip symbols whose cap is below executable floor**.

## Bottom line

- The `>=60 USDT` BTC/ETH rule is currently enforced in **kernel validation**.
- It was introduced as part of the relaxation commit `7027d7a2` and has not meaningfully changed since.
- The current bug is that the **prompt and runtime do not enforce or even describe the same rule**.
- The AI emits 34-50 USDT BTC/ETH opens because the prompt tells it BTC/ETH max is ~68 and min is 12, so those numbers are internally consistent from the model’s perspective.
- The robust fix is to **remove policy drift** and compute a **single symbol-aware executable minimum** shared by prompt, kernel, and runtime, while pre-disabling impossible symbols for small-balance accounts.
