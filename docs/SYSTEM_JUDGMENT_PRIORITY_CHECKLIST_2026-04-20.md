# System Judgment Priority Enforcement Checklist

_Updated: 2026-04-20_

Scope checked against:
- `docs/STRATEGY_CONTROL_POLICY_2026-04-20.md`
- `kernel/engine_analysis.go`
- `trader/runtime_open_policy.go`
- `trader/auto_trader_loop.go`
- `store/decision.go`
- `web/src/components/trader/PositionHistory.tsx`

This note maps the policy priority table to current implementation status.

Status legend:
- **implemented** = deterministic code path exists and is wired into current flow
- **partial** = some fields/UI/audit exist, but enforcement or completeness is missing
- **missing** = documented policy row has no concrete implementation yet

## Priority rows / checklist

| Field / rule | Current owner | Status | Current implementation / gap |
|---|---|---:|---|
| `entry_protection_rationale` required for `open_long/open_short` | kernel | **implemented** | `ValidateEntryProtectionRationale` rejects open actions when rationale is absent. |
| Positive `risk_reward.entry/invalidation/first_target/gross_estimated_rr` | kernel | **implemented** | Hard-validated in `kernel/engine_analysis.go`. |
| Direction match: long=`invalidation < entry < first_target`, short inverse | kernel | **implemented** | Hard-validated in `ValidateEntryProtectionRationale`. |
| Recompute gross RR from price geometry and compare with AI value (tol 0.05) | kernel | **implemented** | `computedRR` derived from entry/invalidation/first_target and compared against AI gross RR. |
| Effective RR must meet `strategy.risk_control.min_risk_reward_ratio` (fallback 1.5) | kernel | **implemented** | Enforced in `ValidateEntryProtectionRationale`; strategy route passes configured min RR. |
| If `net_estimated_rr > 0`, use net RR as effective gate | kernel | **implemented** | Kernel validation switches effective RR to net when present. |
| If execution constraints exist, recompute RR with rounding/fee/slippage | kernel + runtime | **implemented** | Kernel recomputes during validation; runtime recomputes again after live constraint merge in `applyRuntimeOpenPolicy`. |
| If `min_required_rr` supplied, it must match strategy min (tol 0.02) | kernel | **implemented** | Hard-validated in `ValidateEntryProtectionRationale`. |
| `risk_reward.passed` must agree with effective RR gate | kernel | **implemented** | Hard-validated with ±0.02 band. Review context also normalizes display to true when RR meets min. |
| Protection alignment must not contradict invalidation / first target | kernel | **implemented** | `validateProtectionPlanAlignmentSkeleton`, break-even alignment, and fallback alignment reject contradictions. |
| Only system-derived fields may be silently merged/overridden | runtime | **partial** | Runtime only merges execution constraints and recomputes RR; this is narrow and deterministic. But there is no explicit generic override audit state besides accepted/rejected metadata. |
| Merge runtime execution constraints into rationale when AI omitted them | runtime | **implemented** | `applyRuntimeOpenPolicy` calls `mergeExecutionConstraints(...)`; audit exposes `constraints_merged`. |
| System may set pass outcome based on recomputed effective RR | runtime + audit | **implemented** | Runtime sets `decision.EntryProtection.RiskReward.Passed` after recomputation; review context/control expose recomputation and effective RR. |
| Protection alignment booleans derived from protection snapshot, not trusted from AI text | runtime/audit | **implemented** | `deriveProtectionAlignment` computes stop/target/break-even/fallback booleans from `ProtectionSnapshot`. |
| Invalid JSON / unparseable decision should reject cycle | AI/kernel | **partial** | Truly invalid JSON errors still fail parsing, but a missing structured decision falls back to synthetic `wait` in `extractDecisions` instead of always rejecting. This differs from the stricter reject row. |
| Unknown action value should reject cycle | kernel | **implemented** | Covered by decision format validation path before strategy-route validation. |
| Malformed open action should reject cycle | kernel | **implemented** | Open actions fail on rationale/protection schema validation. |
| Direction mismatch should reject cycle | kernel | **implemented** | Hard rejection in rationale validation. |
| Gross RR materially inconsistent with geometry should reject cycle | kernel | **implemented** | Hard rejection with 0.05 tolerance. |
| Effective RR below strategy minimum should reject cycle | kernel + runtime | **implemented** | Kernel rejects pre-execution; runtime also blocks strict-mode opens when live execution-aware RR falls below min. |
| Supplied net RR inconsistent with recomputation should reject cycle | kernel | **implemented** | Hard rejection when execution-aware recompute differs from AI net RR by >0.05. |
| Protection plan contradicts invalidation/first target should reject cycle | kernel | **implemented** | Hard rejection via alignment checks. |
| Leverage / size / action hard validation limits | kernel | **implemented** | Enforced by broader decision format validation before route validation. |
| Downgrade one open action to `wait` instead of reject | runtime | **missing** | No downgrade branch exists. Current runtime only accepts or rejects. |
| Persist `original_action` when downgraded | store/UI | **missing** | No field in `DecisionActionControlOutcome` or action record. |
| Persist `final_action` when different | store/UI | **missing** | No field today. |
| Persist `control_decision` as `accepted/rejected/downgraded_to_wait/overridden` | store/UI | **partial** | Stored as `control.decision`; accepted/rejected are used, UI can render all four labels, but downgrade/override are not produced by runtime. |
| Persist `control_reasons[]` | store/UI | **implemented** | Stored as `control.reasons`; runtime currently writes one reason string when present. |
| Persist `failed_checks[]` | store/UI | **implemented** | Stored as `control.failed_checks`; runtime currently writes reason code(s) such as `runtime_rr_below_min`. |
| Persist `no_order_placed` for rejected/downgraded opens | runtime/store/UI | **partial** | Set for runtime-blocked rejected opens. No downgrade path exists yet. Kernel-parse rejections outside runtime do not map per-action `no_order_placed` entries. |
| Persist `effective_rr` and `effective_rr_source` | runtime/store/UI | **implemented** | Stored in `DecisionActionControlOutcome`; shown by Position History badges. |
| Persist `recomputed_gross_rr` and `recomputed_net_rr` when execution recompute ran | runtime/store/UI | **implemented** | Stored as `runtime_gross_rr` / `runtime_net_rr`; naming differs from policy doc but semantics are present. |
| Persist execution constraint source map | runtime/store/UI | **implemented** | Stored as `execution_constraint_sources`; runtime compacts snapshot sources. UI does not currently display the sources text. |
| Persist compact execution constraints only | runtime/store/UI | **implemented** | `DecisionActionExecutionConstraints` is compact and `PositionHistory` shows a compact subset. |
| Binance-first compact fields (`tick_size`, `price_precision`, `qty_step_size`, `qty_precision`, `min_qty`, `min_notional`, `last_price`, optional TOB/spread) | runtime/store | **implemented** | Compact execution constraint structure includes all listed Binance fields. |
| OKX-first compact fields (`tick_size`, `qty_step_size`, `min_qty`, `contract_value`, `last_price`, optional TOB/spread) | runtime/store | **implemented** | Compact execution constraint structure includes OKX-needed fields. |
| Other exchanges degrade legally without fabricated metadata | kernel + runtime | **implemented** | Validation only uses execution constraints if present; runtime helper treats missing constraints as legal/no-op. |
| Never fetch live exchange data inside kernel validation | kernel | **implemented** | Kernel validation only consumes provided rationale/constraints; runtime snapshot collection happens outside kernel. |
| UI shows compact audit surface for RR / key levels / anchors / protection / control | UI | **implemented** | `PositionHistory.tsx` renders these audit chips/details. |
| UI shows control decision labels for accepted/rejected/downgraded/overridden | UI | **partial** | Renderer supports all labels, but runtime currently emits only accepted/rejected. |
| UI shows failed checks | UI | **implemented** | Renders `control.failed_checks`. |
| UI shows `no_order_placed` | UI | **implemented** | Renders badge when field exists. |
| UI shows execution constraint source map | UI | **missing** | Sources are stored but not rendered in `PositionHistory.tsx`. |
| UI shows action delta (`original_action` -> `final_action`) | UI | **missing** | No stored fields and no renderer. |

## Concrete gaps to close for full policy parity

1. **Downgrade-to-wait path is not implemented**
   - Missing runtime/state branch for `downgraded_to_wait`.
   - Missing persistence of `original_action` / `final_action`.
   - Missing audited `no_order_placed` for downgrade cases.

2. **Override state is documented more broadly than implemented**
   - Current runtime does deterministic merge + RR recomputation only.
   - No explicit emitted `overridden` outcome is produced today.

3. **Reject-vs-safe-wait behavior is mixed at parse layer**
   - Policy text says invalid/unparseable decision payload should reject.
   - Current parser sometimes synthesizes a safe `wait` action when no JSON decision array is found.

4. **Control audit shape is close but not fully policy-complete**
   - Present: `decision`, `reasons`, `failed_checks`, `effective_rr`, `effective_rr_source`, recomputed runtime RR, constraint sources, `no_order_placed`.
   - Missing: `original_action`, `final_action`.
   - Naming drift: policy says `recomputed_gross_rr/recomputed_net_rr`; code stores `runtime_gross_rr/runtime_net_rr`.

5. **UI is ahead of runtime in a few labels**
   - UI can display `downgraded_to_wait` and `overridden`, but current runtime never emits them.
   - UI does not surface `execution_constraint_sources` yet.

## Short conclusion

Today the codebase already enforces the **core priority rows** for open-action judgment in kernel validation and adds a second **runtime strict RR block** after execution constraints are collected. The biggest remaining policy gaps are not the core RR/protection math; they are the **audited control-state completeness**:
- no real `downgraded_to_wait` path,
- no `original_action/final_action` persistence,
- no produced `overridden` outcome,
- and one parser fallback that is softer than the policy's strict-reject wording.
