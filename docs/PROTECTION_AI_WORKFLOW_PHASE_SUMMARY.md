# Protection AI Workflow Phase Summary (2026-04-15)

## Phase Goal
This phase aimed to turn protection handling from a loose prompt-side suggestion into a structured, testable, and partially route-aware contract spanning config, execution, parsing, validation, test-run observability, and real-model verification.

## What Is Complete

### 1. Config / UI / Persistence
- Full / Ladder / fallback protection config upgraded to field-level mode/value structure.
- Frontend ProtectionEditor upgraded for Full and Ladder.
- StrategyStudio uses normalized protection config for backward-compatible rendering.
- Store/API round-trip and merge semantics for new protection fields are covered by tests.

### 2. Execution / Planner
- Manual and AI protection routes are separated.
- AI protection enters runtime only through `decision.protection_plan`.
- Full and Ladder planner/execution paths are both implemented.
- Fallback max-loss is integrated as an independent guardrail.

### 3. Parser / Validator / Test-Run Observability
- Nested `protection_plan` / `ladder_rules` parsing works.
- `test-run` returns `parsed_decisions` and `parse_error`.
- StrategyStudio displays parsed decisions and protection summaries.
- Invalid protection outputs are surfaced through `parse_error`.

### 4. Route-aware Contracts
- Full / Ladder are no longer treated as pure AI preference in validation.
- If strategy route is Full-only AI, open actions must carry `mode=full`.
- If strategy route is Ladder-only AI, open actions must carry `mode=ladder` with 2~3 tiers.

### 5. Runtime Protection Analysis Contracts
- Drawdown / Break-even are introduced as reasoning contracts.
- When enabled in strategy config, AI reasoning must explicitly acknowledge them.
- These are exposed through test-run validation, not via a new JSON output shape.

## Real Validation Results

### Full Route
- Real model output successfully produced:
  - `open_long`
  - `protection_plan.mode=full`
  - `take_profit_pct` / `stop_loss_pct`
  - `parse_error = ""`
- Conclusion: **Full AI protection route is real-output validated.**

### Ladder Route
- Engineering path, parser, validator, route-aware checks, and fixtures are in place.
- Real model under tested market contexts still prefers `wait` instead of emitting ladder open output.
- Conclusion: **Ladder route is engineering-validated, but not yet real-output validated in tested live contexts.**

## Test Assets Added In This Phase
- `docs/fixtures/protection-test-run-fixture.json`
- `docs/fixtures/protection-test-run-open-bias-fixture.json`
- `docs/fixtures/protection-test-run-single-open-bias-fixture.json`
- `docs/fixtures/protection-test-run-single-ladder-bias-fixture.json`
- `docs/fixtures/protection-test-run-single-ladder-only-fixture.json`
- `docs/fixtures/PROTECTION_TEST_RUN_GUIDE.md`
- `docs/AI_PROTECTION_WORKFLOW_TEST_COVERAGE.md`
- `cmd/protectiontestrun`

## Key Checkpoints
- `3b59a978` feat: wire structured ai protection workflow
- `a5ed21dd` test: add protection workflow acceptance coverage
- `81ad031d` test: cover invalid ai protection outputs
- `d44d7f14` feat: enforce route-aware ai protection validation
- `66130520` feat: enforce runtime protection reasoning contracts

## Remaining Boundaries
- Real provider/model quality for ladder output is not fully proven.
- Drawdown / Break-even are currently enforced as reasoning contracts, not dedicated output-shape contracts.
- Some unrelated modified files were intentionally excluded from this protection-focused sequence to keep checkpoints narrow.

## Recommended Next Step
- Treat this phase as functionally closed.
- Future work should only continue when there is a concrete business need to push ladder real-output validation further or to formalize Drawdown / Break-even into stronger runtime contracts.
