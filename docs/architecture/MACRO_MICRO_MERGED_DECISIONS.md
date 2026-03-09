# Macro-Micro: Merged Decisions and Entry Handling

## How merged decisions are built

1. **Macro** returns `symbols_for_deep_dive`. The pipeline ensures this list **always includes every open position symbol** (via `ValidateAndMergeMacroOutput` in `kernel/macro.go`), plus up to N opportunity symbols.
2. **Deep-dive**: One AI call per symbol in `symbols_for_deep_dive`. Each returns a single `Decision` (e.g. `open_long`, `open_short`, `wait`) for that symbol, including `reasoning`.
3. **Position-check** (if there are open positions): One AI call with the macro brief + full position list. Returns one decision per **open position** (e.g. `close_long`, `close_short`, `hold`), each with `reasoning`.
4. **Merge** (`kernel/engine.go`):
   - Start with the list of deep-dive decisions (one per symbol in `symbols_for_deep_dive`).
   - For each symbol that has an open position, **replace** that symbol’s deep-dive decision with the **position-check** decision for that symbol (position-check overrides deep-dive for that symbol).
   - Any position-check decisions for symbols that were not in the deep-dive list are appended.
   - Result: **one decision per symbol**, including **every open position**. For open positions the merged decision is the position-check outcome (close/hold) so the trader can close or replace positions. Each decision keeps the full struct: `symbol`, `action`, `reasoning`, and for opens also `leverage`, `position_size_usd`, `stop_loss`, `take_profit`, `confidence`, `risk_usd`.

So the merged list can contain multiple **entry** decisions (`open_long` / `open_short`) and multiple `wait` (or `hold`). There is no step that caps “at most K new entries” in the merged list; the list is just “one decision per symbol.”

## Open positions are in the merged array

Yes. Every open position has exactly one decision in the merged array:

- Open-position symbols are always included in `symbols_for_deep_dive` (enforced in `ValidateAndMergeMacroOutput`).
- For each of those symbols, the merge uses the **position-check** decision (close_long, close_short, or hold) instead of the deep-dive. So the merged list contains the TP/SL/hold decision for each position, allowing the trader to close or hold and, in the same cycle, open new positions (e.g. replace one symbol with another).

## Reasoning in merged decisions

Each merged decision is a full `kernel.Decision`. The `reasoning` field is preserved:

- **Deep-dive** decisions: parsed from the model’s JSON via `extractDecisions`, which fills `Reasoning` from the AI output.
- **Position-check** decisions: same parsing; each close/hold decision includes the model’s reasoning.

The strategy test-run API returns the full decision struct (including `reasoning`) so the UI can show it for every merged decision.

## Validation of merged decisions

- **`validateDecisions`** (`kernel/engine.go`) runs on the **entire merged slice** but validates **each decision individually**:
  - Action must be one of: `open_long`, `open_short`, `close_long`, `close_short`, `hold`, `wait`.
  - For `open_long` / `open_short`: leverage, `position_size_usd`, `stop_loss`, `take_profit`, min position size, max position value vs equity, and risk/reward ≥ 3.0 are checked per decision.
  - It does **not** enforce “at most N open_long/open_short” or “total new entries ≤ MaxPositions” at this stage. So you can have e.g. 2 `open_long` and 3 `wait` in the merged list; both `open_long` are validated on their own (and must have valid leverage, size, SL/TP, etc.).
- If validation fails for a decision, an error is returned and **logged** with `logger.Warnf`; the merged list is **not** modified or filtered. So invalid decisions (e.g. `open_long` without `position_size_usd` / `stop_loss` / `take_profit`) remain in the list and will cause execution to fail when the trader tries to open that position.

## How entry points are handled at execution

- **Trader** (`trader/auto_trader.go`):
  - Decisions are **sorted** by priority: `close_long`/`close_short` first, then `open_long`/`open_short`, then `hold`/`wait`.
  - The trader then **iterates** over this sorted list and executes each decision (close, then open, then wait).
  - **Max positions** are enforced **when opening**: before each `open_long` / `open_short`, `enforceMaxPositions(currentPositionCount)` is called. If `currentPositionCount >= MaxPositions`, the open is **rejected** with “Already at max positions” and that decision is not executed. So:
    - Entry decisions in the merged list are **not** pre-filtered to “at most N opens.”
    - Execution order is: closes first (freeing slots), then opens in sorted order until the next open would exceed `MaxPositions`; that open and any further opens in the list fail at execution time.

So **entry points in merged decisions** are handled as follows:

- **Merge**: Every symbol in `symbols_for_deep_dive` gets exactly one decision (from deep-dive or overridden by position-check). Entry decisions (`open_long`/`open_short`) appear in the merged list like any other decision.
- **Validation**: Each entry is validated per decision (leverage, size, SL/TP, risk/reward). No cap on how many entries are in the list.
- **Execution**: Closes first, then opens in order; each open is allowed only if `currentPositionCount < MaxPositions`. So multiple entry decisions in the list are fine; they are executed in order until the position count limit is reached.

## Example (your payload)

```json
[
  { "action": "open_long", "reasoning": "", "symbol": "BANANAS31USDT" },
  { "action": "open_long", "reasoning": "", "symbol": "ASTERUSDT" },
  { "action": "wait", "reasoning": "", "symbol": "SOLUSDT" },
  { "action": "wait", "reasoning": "", "symbol": "TAOUSDT" },
  { "action": "wait", "reasoning": "", "symbol": "RIVERUSDT" }
]
```

- This is **one decision per symbol** (5 symbols → 5 decisions). The two `open_long` and three `wait` are the merged outcome of the deep-dives (and position-check overrides if you had open positions in any of these).
- For **execution** to succeed, each `open_long` must have valid `leverage`, `position_size_usd`, `stop_loss`, `take_profit` (and pass risk/reward) in the actual `kernel.Decision` used by the engine/trader. The API test response you see may show a **subset** of fields (e.g. only `action`, `reasoning`, `symbol`) for display; the real decisions in the kernel should carry the full opening params if the model returned them.
- If the model returned `open_long` without size/SL/TP, validation would warn and execution would fail when trying to open. So in practice, entry handling depends on the AI returning complete open decisions and on execution order + `MaxPositions` as above.

## Merged result has full deep-dive fields

The merged list is a slice of full `kernel.Decision` structs. Each deep-dive returns one `Decision` (parsed from the model’s JSON); the merge only chooses *which* decision per symbol (deep-dive vs position-check). It does not strip fields. So each merged decision includes, when present from the AI:

- `symbol`, `action`, `reasoning`
- For opens: `leverage`, `position_size_usd`, `stop_loss`, `take_profit`, `confidence`, `risk_usd`

The **strategy test-run API** (macro-micro) returns this merged list. The response now serializes each decision in full (not only `symbol`/`action`/`reasoning`) so the UI can show confidence, SL/TP, size, etc.

## Trades are processed from the merged result

Yes. The trader (and any consumer of the strategy output) uses the **merged** `FullDecision.Decisions`:

1. **Live/auto trader**: `buildTradingContext` → kernel `GetFullDecisionMacroMicro` (or single-turn path) → `FullDecision.Decisions` → sorted by priority → each decision executed (opens use `Leverage`, `PositionSizeUSD`, `StopLoss`, `TakeProfit` from the merged decision).
2. **Strategy test-run**: Same kernel flow returns `FullDecision` with the same merged `Decisions`; the API now returns those decisions in full so the “Decisions (merged)” section shows all deep-dive fields.

So the merged result is the single source of truth for what to do per symbol, and execution uses it as-is (with validation and execution-time checks like max positions).

## Relevant code

- Merge: `kernel/engine.go` — `getFullDecisionMacroMicro` / `GetFullDecisionMacroMicroWithTrace` (steps 4–6: deep-dives, position-check, merge loop).
- Validation: `kernel/engine.go` — `validateDecisions` / `validateDecision`.
- Test-run response: `api/strategy.go` — macro-micro branch builds `decisionsPayload` from full `fullDecision.Decisions` (full struct serialized).
- Execution order: `trader/auto_trader.go` — `sortDecisionsByPriority`, then loop over `sortedDecisions` with `executeDecisionWithRecord`.
- Max positions: `trader/auto_trader.go` — `enforceMaxPositions` called inside `executeOpenLongWithRecord` / `executeOpenShortWithRecord`.
