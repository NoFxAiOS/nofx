
## Drawdown / Break-even Analysis Contracts (2026-04-15)

### What is now enforced
- Drawdown Take Profit and Break-even Stop are no longer only prompt hints.
- They are treated as reasoning contracts when enabled in strategy config.
- AI reasoning must explicitly acknowledge:
  - drawdown / trailing / profit-protection ownership when drawdown_take_profit is enabled
  - break-even / additional stop layer when break_even_stop is enabled

### Where this is enforced
- `kernel/protection_reasoning_contract.go`
- `kernel/engine_analysis.go` via `ParseAndValidateAIDecisionsWithStrategy(...)`
- `/api/strategies/test-run` now exposes reasoning-contract failures through `parse_error`
- `cmd/protectiontestrun` also uses the same contract-aware validation chain

### Covered tests
- `kernel/protection_reasoning_contract_test.go`
- `api/strategy_test_run_reasoning_contract_test.go`

### Scope boundary
- These are analysis-contract constraints, not new decision JSON shape constraints.
- Full / Ladder remain route-aware shape constraints.
- Drawdown / Break-even currently require explicit acknowledgement in reasoning, not a dedicated protection JSON payload.
