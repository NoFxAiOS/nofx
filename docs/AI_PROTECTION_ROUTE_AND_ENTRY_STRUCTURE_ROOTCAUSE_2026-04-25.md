# AI protection route and entry-structure validation root cause notes (2026-04-25)

## Scope
Investigated the failures:
- `current strategy route supports only one AI protection route at a time (full, ladder, or drawdown)`
- `entry_protection_rationale.key_levels support exceeds max 3`
- `entry_protection_rationale.key_levels resistance exceeds max 3`

Focus areas: prompt-building, schema validation, parser/repair, and config flow.

---

## 1) Route validation failure: multiple AI protection routes can persist in config

### What fails
Runtime rejects any open decision when more than one protection route is configured in AI mode.

### Evidence
- `kernel/engine_analysis.go:436-449`
  - `fullAI`, `ladderAI`, `drawdownAI` are computed from strategy config.
  - Validation hard-fails when more than one is true.
- `api/strategy.go:200-204`
  - conflicting AI route combinations only produce **warnings**, not errors.
- `api/strategy.go:18-41` (`mergeStrategyConfig`, `deepMergeMap`)
  - incoming partial config is deep-merged into existing config, so old `mode: ai` values can survive unless the caller explicitly clears them.
- `web/src/components/strategy/ProtectionEditor.tsx`
  - editor exposes full / ladder / drawdown independently.
  - I found mismatch warnings for disabled+AI (`fullModeMismatch`, `ladderModeMismatch`, `drawdownModeMismatch`), but no exclusivity enforcement before save.

### Root cause
This is primarily a **config acceptance / merge** issue, not a parser issue.

A likely failure path is:
1. Strategy already has one route in `mode=ai`.
2. User switches another route to `mode=ai` via partial update.
3. `mergeStrategyConfig()` preserves the older AI mode unless it is explicitly reset.
4. API returns warnings only, so invalid config is still saved.
5. Runtime prompt is built and AI responds.
6. `validateAIDecisionRoutesWithStrategy()` rejects all open decisions immediately.

### Prompt contribution
Prompting is not the direct cause of this specific error, but it does not help:
- `kernel/engine_prompt.go:223-236` still describes all protection plan modes as generally available.
- `kernel/prompt_builder.go:260-334` includes examples for `full`, `drawdown`, and `ladder` in one generic block.

So even when the active route should be singular, the prompt surface still looks multi-route capable.

### Concrete code changes to make
1. **Turn conflicting AI route warnings into hard validation errors at save/update time**
   - File: `api/strategy.go`
   - In `validateStrategyConfig`, return structured errors or a second `fatalErrors` list for:
     - `full.mode=ai && ladder.mode=ai`
     - `full.mode=ai && drawdown.mode=ai`
     - `ladder.mode=ai && drawdown.mode=ai`
2. **Normalize conflicting modes during merge**
   - File: `api/strategy.go`
   - After `mergeStrategyConfig()`, run a post-merge sanitizer that ensures only one of full/ladder/drawdown can remain `mode=ai`.
   - Recommendation: reject rather than auto-pick, unless product explicitly wants priority rules.
3. **UI guardrail**
   - File: `web/src/components/strategy/ProtectionEditor.tsx`
   - When one route is switched to `ai`, either:
     - auto-switch other routes to `manual`/`disabled`, or
     - block save with a clear inline error.
4. **Prompt tightening**
   - File: `kernel/engine_prompt.go`
   - Emit only the active route instructions/examples for the current strategy.
   - If route config is invalid, fail before prompt generation.

---

## 2) `key_levels support/resistance exceeds max 3`: repair/backfill inflates arrays past validator limits

### What fails
Validation enforces entry-structure maxima on `key_levels.support` and `key_levels.resistance`.

### Evidence
- `kernel/engine_analysis.go:503-511`
  - runtime rejects when `len(support) > MaxSupportLevels` or `len(resistance) > MaxResistanceLevels`.
- `web/src/components/strategy/EntryStructureEditor.tsx:9-19`
  - default UI limits are `max_support_levels: 3`, `max_resistance_levels: 3`.
- `kernel/schema_registry.go:10-25`
  - `key_levels.support` and `key_levels.resistance` are marked `AutoFill: true`.
- `kernel/engine_analysis.go:1012-1068` (`backfillEntryProtectionKeyLevels`)
  - fills `key_levels.support/resistance` from **all** `structural_key_levels` and then from anchors.
  - there is no cap, no nearest-level selection, and no awareness of `config.EntryStructure.Max*`.
- `kernel/engine_prompt.go:144-151`, `223-236`
  - prompt tells model to include `structural_key_levels` and also says support/resistance are required.
- Log snippets in `data/nofx_2026-04-25.log`
  - repeated validation failures for `support exceeds max 3` / `resistance exceeds max 3`.

### Root cause
This is mainly a **schema-repair / validation ordering** problem, amplified by prompt wording.

The system currently treats `structural_key_levels` as rich evidence, then auto-expands that evidence into `key_levels.support/resistance`. That repair step can create arrays larger than the user-configured maxima before validation runs.

So the model can be doing something locally reasonable, for example:
- 1 explicit support/resistance pair in `key_levels`
- several structural levels in `structural_key_levels`
- several anchors

but `backfillEntryProtectionKeyLevels()` converts those extras into `key_levels.*`, and the validator then rejects the repaired object.

### Why this is a design mismatch
The prompt says:
- keep structural entry compact,
- include structural key levels that influenced protection,
- ensure key_levels support/resistance are present.

But the repair path effectively treats every structural level as something that must also count against the small `max_support_levels/max_resistance_levels` budget.

That means the repair layer is stricter than the intended contract.

### Concrete code changes to make
1. **Only autofill missing buckets, not append indefinitely**
   - File: `kernel/engine_analysis.go`
   - In `backfillEntryProtectionKeyLevels()`:
     - if `key_levels.support` already exists, do not append additional support levels from `structural_key_levels`/anchors unless needed to satisfy minimum presence.
     - same for resistance.
2. **Make autofill config-aware and capped**
   - File: `kernel/engine_analysis.go`
   - Pass `config.EntryStructure.MaxSupportLevels` / `MaxResistanceLevels` into repair, or do capped repair in `ValidateEntryProtectionRationale()` before limit checks.
   - Keep only the most relevant levels, for example nearest/primary levels after sorting.
3. **Differentiate evidence arrays from compact validation arrays**
   - Files: `kernel/engine.go`, `kernel/engine_analysis.go`
   - Keep `structural_key_levels` as the rich audit trail.
   - Keep `key_levels.support/resistance` as the compact contract fields.
   - Do not require them to mirror every structural level 1:1.
4. **Prompt the max explicitly**
   - File: `kernel/engine_prompt.go`
   - Add wording like: `key_levels.support/resistance should contain only the top 1..N levels (N from strategy, commonly 3), not every detected level.`
5. **Add regression tests**
   - New tests around `backfillEntryProtectionKeyLevels()` + strategy max limits.
   - Cases:
     - structural levels > 3 but explicit compact key_levels already present -> should pass.
     - empty key_levels + 5 structural levels -> backfill should cap to max.

---

## 3) Prompt examples are internally inconsistent with the structural-entry contract

### Evidence
- `kernel/engine_prompt.go:211-240`
  - says `entry_protection_rationale` is required and support/resistance are required when entry_structure is enabled.
- The inline example around `kernel/engine_prompt.go:198-221` shows `entry_protection_rationale` with `anchors` and `risk_reward`, but **no `key_levels.support/resistance`**.
- `kernel/prompt_builder.go:260-334` generic examples also omit the now-required `key_levels.support/resistance` fields.

### Why it matters
The model is being shown examples that violate the stricter runtime validator. That increases the odds of missing fields, over-reliance on backfill, and then max-limit failures after repair.

### Concrete code changes to make
1. Update all open-action examples to include:
   - `key_levels.support`
   - `key_levels.resistance`
   - and only a compact number of levels.
2. Remove generic examples for inactive protection modes from the active prompt.
3. If `entry_structure.enabled=true`, example JSON should be validator-clean for the active strategy config.

---

## 4) Recommended implementation order

1. **Block invalid multi-AI-route configs at save/update time** (`api/strategy.go`)
2. **Cap / narrow backfill behavior** (`kernel/engine_analysis.go`)
3. **Fix prompt examples to match validator reality** (`kernel/engine_prompt.go`, `kernel/prompt_builder.go`)
4. **Add regression tests** for both classes of failures

---

## Bottom line
- The **route conflict** failure is caused by invalid strategy config being allowed to persist, especially through deep-merge updates and warning-only validation.
- The **support/resistance max** failure is caused by a mismatch between the compact entry-structure contract and the repair path, which auto-expands rich structural evidence into capped key-level arrays.
- Prompt examples currently reinforce the mismatch by omitting required compact fields and by advertising multiple protection modes generically.
