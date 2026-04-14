# AI Protection Workflow Test Coverage

## Current Covered Layers

### 1. Config / Persistence
- strategy config round-trip preserves new protection fields
- update merge semantics preserve nested protection fields

### 2. Planner / Execution
- Full / Ladder / fallback max loss planner paths
- manual / ai / disabled structural paths
- reconciler presence checks for fallback max loss

### 3. Prompt / Parsing / Validation
- prompt contains protection_plan guidance and few-shot examples
- parser supports nested protection_plan and ladder_rules
- validator accepts current action set and rejects invalid protection_plan shapes

### 4. API / UI Observability
- test-run envelope always returns parsed_decisions + parse_error
- StrategyStudio test panel displays parsed_decisions and protection_plan summary

## Positive Acceptance Covered
- open with full protection_plan
- open with ladder protection_plan
- close without protection_plan
- fixture-driven prompt workflow acceptance

## Negative Acceptance Covered
- close action carrying protection_plan
- full mode carrying ladder_rules
- ladder mode without rules
- full mode without any thresholds
- unknown protection_plan mode
- invalid ladder close ratios / ladder rule bounds
- invalid outputs propagate through parse_error envelope

## Remaining Non-Automated Area
- real provider/model output quality under live external inference
- this depends on model availability, credentials, and current market context

## Current Guidance
- for code changes: keep extending validator and prompt together
- for live quality checks: use docs/fixtures/protection-test-run-fixture.json with StrategyStudio test-run
