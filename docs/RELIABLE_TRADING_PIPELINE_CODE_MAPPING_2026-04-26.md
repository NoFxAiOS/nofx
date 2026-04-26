# Current Code Mapping for Reliable Trading Pipeline

Date: 2026-04-26
Purpose: map the proposed reliable pipeline onto the existing nofxmax skeleton before behavior changes.

## Pipeline Mapping

### 1. Candidate Universe

Current files:

- `market/hot_coins.go`
- `kernel/engine.go`
- `trader/auto_trader_loop.go`

Current behavior:

- Candidate coins are fetched from configured market sources such as hot / oi_top.
- Static fallback coins may be used when market list sources fail.

Risks:

- Candidate selection can include symbols with incomplete 3m/15m/1h data.
- Candidate selection currently does not fully gate by derivatives crowding or higher timeframe regime.

Planned improvement:

- Add record-only market context v2 per candidate before changing candidate eligibility.

### 2. Market Data / Technical Context

Current files:

- `market/data.go`
- `market/data_klines.go`
- `market/data_indicators.go`
- `market/fibonacci.go`
- `market/structure.go`
- `kernel/engine_analysis.go`

Current behavior:

- Multi-timeframe Kline data and technical/structure context are assembled for AI.
- Primary timeframe is currently centered around strategy config, commonly 15m with 3m/1h.

Risks:

- 4H/1D macro context is not consistently represented.
- Missing market data is sometimes treated as candidate skip, but not always as a strategy-level no-trade reason.

Planned improvement:

- Extend context with 1D/4H summaries.
- Add data quality/staleness flags.

### 3. Derivatives / Crowding Data

Current files/providers:

- `provider/coinank/open_interest.go`
- `provider/coinank/liquidation.go`
- `provider/coinank/net_positions.go`
- `provider/nofxos/oi.go`
- `provider/nofxos/netflow.go`
- `market/hot_coins.go`

Current behavior:

- Some OI-ranked market sources exist.
- OI is used for candidate ranking / filtering in places, but not as a complete AI decision contract.

Risks:

- AI may see chart structure without enough information about leverage crowding.
- Funding / mark premium / OI change are not a hard quality layer yet.

Planned improvement:

- Introduce `DerivativesContext` record-only first.
- Feed compact summary to AI and decision records.

### 4. AI Prompt and Decision Schema

Current files:

- `kernel/engine_prompt.go`
- `kernel/prompt_builder.go`
- `kernel/schema.go`
- `kernel/schema_registry.go`
- `kernel/schema_alias_helpers.go`
- `kernel/engine_analysis.go`
- `kernel/protection_reasoning_contract.go`

Current behavior:

- Existing decision schema supports open/hold/wait/close style actions.
- Protection rationale and AI protection contract are already partly present.
- Empty array no-trade is supported.

Risks:

- AI can still over-focus on entry without a formal regime/setup/quality score.
- AI protection outputs may be structurally valid but semantically weak without derivatives context and net RR.

Planned improvement:

- Add optional schema v2 fields:
  - `regime`
  - `setup_type`
  - `quality_score`
  - `net_estimated_rr`
  - `derivatives_notes`
- Keep old outputs compatible during shadow mode.

### 5. Deterministic Open Policy / Runtime Gates

Current files:

- `trader/runtime_open_policy.go`
- `kernel/risk_reward_execution.go`
- `trader/auto_trader_loop.go`
- `trader/auto_trader_orders.go`

Current behavior:

- There are already execution constraints, RR validation, and runtime open policy tests.

Risks:

- Some gates are schema/entry-structure focused, not yet complete market-regime gates.
- Low-quality but parse-valid decisions can still reach execution if they satisfy old fields.

Planned improvement:

- Add quality gates after schema v2 shadow data is stable:
  - confidence threshold;
  - net RR threshold;
  - setup_type whitelist;
  - stale data WAIT;
  - middle-of-range WAIT;
  - cooldown / drawdown safety.

### 6. Open Execution

Current files:

- `trader/auto_trader_orders.go`
- `trader/okx/trader_orders.go`
- `trader/executable_min_position_test.go`
- `trader/execution_constraints_snapshot.go`

Current behavior:

- Orders are sent through exchange adapters.
- OKX formatting and min quantities are handled in adapter/execution constraints.

Risks:

- Protection may be applied after fill but must be strictly verified before state is considered safe.
- Open success and protection success must be semantically separate.

Planned improvement:

- Keep open execution intact initially.
- Add explicit post-fill protection state transitions later.

### 7. Protection Plan Materialization

Current files:

- `trader/protection_plan.go`
- `trader/protection_structural.go`
- `kernel/protection_contract_test.go`
- `kernel/protection_fixture_test.go`

Current behavior:

- Strategy config materializes full/ladder/fallback protection.
- AI protection plan can merge with configured plan.
- Structural fallback exists.

Risks:

- Plan materialization can return nil when modes/values are AI/disabled or when strategy values are incomplete.
- Manual and AI ownership semantics are spread across plan builder, execution, risk monitor, and reconciler.

Planned improvement:

- Add explicit plan explain output for tests and diagnostics:
  - why plan exists / nil;
  - which protection legs are expected;
  - which ownership side each leg covers.
- Do this in tests first.

### 8. Protection Execution

Current files:

- `trader/protection_execution.go`
- `trader/protection_execution_test.go`
- `trader/protection_lifecycle_test.go`
- `trader/partial_drawdown_native.go`

Current behavior:

- Applies ladder/full/fallback protection.
- Verifies open orders with retry.
- Drops non-executable tiers and degrades.

Risks:

- Static protection and dynamic protection ownership can conflict when drawdown is enabled.
- Verification lag can look like missing orders and trigger duplicate cleanup/reapply.

Planned improvement:

- Introduce pure ownership evaluator before changing execution.
- Ensure drawdown cannot own TP side until truly armed.

### 9. Drawdown / Break-even Runtime Monitors

Current files:

- `trader/auto_trader_risk.go`
- `trader/auto_trader_risk_test.go`
- `trader/partial_drawdown_native.go`
- `trader/partial_drawdown_native_test.go`

Current behavior:

- Drawdown monitor tracks peak PnL and applies managed/native trailing or partial close.
- Break-even guard exists to avoid duplicate reapply.

Risks:

- Runtime state is in memory and can be lost on restart.
- Exchange order visibility and runtime state can diverge.

Planned improvement:

- Add restart rebuild semantics to ownership evaluator.
- Make dynamic owner states explicit: pending/arming/armed/failed.

### 10. Protection Reconciler

Current files:

- `trader/protection_reconciler.go`
- `trader/protection_reconciler_test.go`

Current behavior:

- Checks positions and open orders.
- Reapplies missing protection.
- Cleans unexpected orders.
- Cleans orphaned state.

Risks:

- Reconciler currently contains too much implicit ownership logic.
- `verified` must not mean merely "no error returned".

Planned improvement:

- Extract a pure expected-vs-observed ownership function with matrix tests.
- Only then use reconciler to execute the evaluator's action list.

### 11. Sync / Fill Source Preservation

Current files:

- `trader/okx/order_sync.go`
- exchange-specific order sync files.
- `store/order.go`
- `store/position_close_event.go`

Current behavior:

- OKX fill sync preserves specific close sources in recent fixes.

Risks:

- Exit reason must remain precise for review and ownership cleanup.

Planned improvement:

- Ensure exit source records include:
  - full_tp;
  - ladder_tp;
  - full_sl;
  - ladder_sl;
  - fallback_sl;
  - break_even_stop;
  - native_trailing;
  - managed_drawdown;
  - manual;
  - ai_close if ever enabled.

### 12. Decision Records / Review

Current files:

- `store/decision.go`
- `trader/auto_trader_decision.go`
- `trader/auto_trader_loop_review_context_test.go`

Current behavior:

- Decision records store prompts, raw response, parsed decisions, protection snapshot, review context.

Risks:

- Review context is not yet sufficient for systematic replay of regime/setup/validator decisions.

Planned improvement:

- Extend review context JSON with:
  - market_context_v2;
  - regime;
  - setup_type;
  - quality_score;
  - validator result;
  - protection ownership expected/observed.

## Immediate Non-Behavior Work Items

1. Commit this mapping/spec after user review.
2. Add test skeletons for protection ownership matrix.
3. Add record-only structs for MarketContextV2 if agreed.
4. Do not alter live protection execution until matrix tests exist.
