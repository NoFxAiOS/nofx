package trader

import (
	"fmt"
	"strings"

	"nofx/kernel"
	"nofx/store"
)

// runtimePolicyResult captures the narrow, deterministic runtime policy effect
// applied after compact execution constraints are collected.
type runtimePolicyResult struct {
	Blocked            bool
	Reason             string
	ReasonCode         string
	Decision           string
	OriginalAction     string
	FinalAction        string
	ConstraintsMerged  bool
	RRRecomputed       bool
	AIGrossRR          float64
	AINetRR            float64
	RuntimeGrossRR     float64
	RuntimeNetRR       float64
	EffectiveRR        float64
	EffectiveRRSource  string
	ConstraintsSources []string
	Protection         *store.DecisionActionProtectionAlignment
}

// applyRuntimeOpenPolicy enforces the smallest system-controlled final judgment
// for open actions after runtime execution constraints are available.
//
// Current scope is intentionally narrow and audit-friendly:
//  1. merge runtime execution constraints when AI omitted them;
//  2. recompute execution-aware RR deterministically from merged constraints;
//  3. block only open actions whose runtime-effective RR falls below minRR when
//     strategy control mode is strict;
//  4. in recommend_only, downgrade low-RR open actions to wait with explicit
//     original/final action audit fields so execution skips order placement;
//  5. in recommend_only, downgrade one stable protection-alignment mismatch
//     (target before first target) to wait using existing compact protection
//     data; strict/audit_only keep their existing reject/flag semantics.
//
// Non-open actions remain untouched so wait/hold legality is preserved.
func applyRuntimeOpenPolicy(decision *kernel.Decision, snapshot *ExecutionConstraintsSnapshot, minRR float64, mode store.StrategyControlPolicyMode, protection ...*store.DecisionActionProtectionAlignment) runtimePolicyResult {
	if decision == nil {
		return runtimePolicyResult{}
	}
	if decision.Action != "open_long" && decision.Action != "open_short" {
		return runtimePolicyResult{}
	}
	if decision.EntryProtection == nil {
		return runtimePolicyResult{}
	}
	if minRR <= 0 {
		minRR = 1.5
	}
	mode = effectiveRuntimePolicyMode(mode)

	merged := mergeExecutionConstraints(decision, snapshot)

	rr := decision.EntryProtection.RiskReward
	result := runtimePolicyResult{
		Decision:           "accepted",
		OriginalAction:     decision.Action,
		FinalAction:        decision.Action,
		ConstraintsMerged:  merged,
		AIGrossRR:          rr.GrossEstimatedRR,
		AINetRR:            rr.NetEstimatedRR,
		ConstraintsSources: compactExecutionConstraintSources(snapshot),
		Protection:         firstRuntimeProtectionAlignment(protection),
	}
	if applyRuntimeProtectionAlignmentPolicy(decision, mode, &result) {
		return result
	}
	if rr.Entry <= 0 || rr.Invalidation <= 0 || rr.FirstTarget <= 0 {
		return result
	}
	if !hasRuntimeRiskRewardExecutionConstraints(decision.EntryProtection.ExecutionConstraints) {
		return result
	}

	effectiveRR := rr.GrossEstimatedRR
	effectiveRRSource := "gross"
	if rr.NetEstimatedRR > 0 {
		effectiveRR = rr.NetEstimatedRR
		effectiveRRSource = "net"
	}
	if recomputedGross, recomputedNet, ok := recomputeRuntimeRiskRewardWithExecutionConstraints(decision.Action, rr, decision.EntryProtection.ExecutionConstraints); ok {
		decision.EntryProtection.RiskReward.GrossEstimatedRR = recomputedGross
		decision.EntryProtection.RiskReward.NetEstimatedRR = recomputedNet
		effectiveRR = recomputedNet
		effectiveRRSource = "runtime_net"
		decision.EntryProtection.RiskReward.Passed = effectiveRR+0.02 >= minRR
		result.RRRecomputed = true
		result.RuntimeGrossRR = recomputedGross
		result.RuntimeNetRR = recomputedNet
	}
	result.EffectiveRR = effectiveRR
	result.EffectiveRRSource = effectiveRRSource
	if effectiveRR > 0 && effectiveRR+0.02 < minRR {
		result.ReasonCode = "runtime_rr_below_min"
		if mode == store.StrategyControlPolicyModeRecommendOnly {
			result.Decision = "downgraded_to_wait"
			result.FinalAction = "wait"
			result.Reason = fmt.Sprintf("runtime RR policy downgraded %s %s to wait: execution-aware rr %.2f below min %.2f", decision.Action, decision.Symbol, effectiveRR, minRR)
			decision.Action = "wait"
			decision.Reasoning = appendDowngradedToWaitReasoning(decision.Reasoning, effectiveRR, minRR)
			return result
		}
		result.Reason = fmt.Sprintf("runtime RR policy %s %s %s: execution-aware rr %.2f below min %.2f", runtimePolicyVerb(mode), decision.Action, decision.Symbol, effectiveRR, minRR)
		if mode == store.StrategyControlPolicyModeStrict {
			result.Blocked = true
			result.Decision = "rejected"
		}
		return result
	}
	return result
}

func appendDowngradedToWaitReasoning(reasoning string, effectiveRR, minRR float64) string {
	note := fmt.Sprintf("runtime policy downgraded to wait: execution-aware rr %.2f below min %.2f", effectiveRR, minRR)
	return appendRuntimeDowngradeReasoning(reasoning, note)
}

func appendRuntimeDowngradeReasoning(reasoning, note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return strings.TrimSpace(reasoning)
	}
	reasoning = strings.TrimSpace(reasoning)
	if reasoning == "" {
		return note
	}
	if strings.Contains(reasoning, note) {
		return reasoning
	}
	return reasoning + " | " + note
}

func firstRuntimeProtectionAlignment(protection []*store.DecisionActionProtectionAlignment) *store.DecisionActionProtectionAlignment {
	if len(protection) == 0 {
		return nil
	}
	return protection[0]
}

func applyRuntimeProtectionAlignmentPolicy(decision *kernel.Decision, mode store.StrategyControlPolicyMode, result *runtimePolicyResult) bool {
	if decision == nil || result == nil || result.Protection == nil || result.Protection.TargetAligned {
		return false
	}
	result.ReasonCode = "protection_target_before_first_target"
	if mode == store.StrategyControlPolicyModeRecommendOnly {
		result.Decision = "downgraded_to_wait"
		result.FinalAction = "wait"
		result.Reason = fmt.Sprintf("runtime protection policy downgraded %s %s to wait: configured target is before rationale first target", result.OriginalAction, decision.Symbol)
		decision.Action = "wait"
		decision.Reasoning = appendRuntimeDowngradeReasoning(decision.Reasoning, "runtime policy downgraded to wait: configured target is before rationale first target")
		return true
	}
	result.Reason = fmt.Sprintf("runtime protection policy %s %s %s: configured target is before rationale first target", runtimePolicyVerb(mode), result.OriginalAction, decision.Symbol)
	if mode == store.StrategyControlPolicyModeStrict {
		result.Blocked = true
		result.Decision = "rejected"
	}
	return true
}

func effectiveRuntimePolicyMode(mode store.StrategyControlPolicyMode) store.StrategyControlPolicyMode {
	switch mode {
	case store.StrategyControlPolicyModeAuditOnly, store.StrategyControlPolicyModeRecommendOnly:
		return mode
	default:
		return store.StrategyControlPolicyModeStrict
	}
}

func runtimePolicyVerb(mode store.StrategyControlPolicyMode) string {
	if effectiveRuntimePolicyMode(mode) == store.StrategyControlPolicyModeStrict {
		return "blocked"
	}
	return "flagged"
}

func recomputeRuntimeRiskRewardWithExecutionConstraints(action string, rr kernel.AIRiskRewardRationale, c kernel.AIEntryExecutionConstraints) (grossRR, netRR float64, ok bool) {
	return kernelRuntimeRecomputeRR(action, rr, c)
}

func hasRuntimeRiskRewardExecutionConstraints(c kernel.AIEntryExecutionConstraints) bool {
	return kernelRuntimeHasRRConstraints(c)
}

// Thin indirection vars keep the helper easy to unit-test without widening the
// kernel API surface.
var (
	kernelRuntimeRecomputeRR = func(action string, rr kernel.AIRiskRewardRationale, c kernel.AIEntryExecutionConstraints) (float64, float64, bool) {
		return 0, 0, false
	}
	kernelRuntimeHasRRConstraints = func(c kernel.AIEntryExecutionConstraints) bool {
		return c.TickSize > 0 || c.PricePrecision > 0 || c.TakerFeeRate > 0 || c.MakerFeeRate > 0 || c.EstimatedSlippageBps > 0
	}
)

func init() {
	kernelRuntimeRecomputeRR = func(action string, rr kernel.AIRiskRewardRationale, c kernel.AIEntryExecutionConstraints) (float64, float64, bool) {
		return runtimeRecomputeRRViaValidationContract(action, rr, c)
	}
}

func runtimeRecomputeRRViaValidationContract(action string, rr kernel.AIRiskRewardRationale, c kernel.AIEntryExecutionConstraints) (float64, float64, bool) {
	// Keep runtime behavior aligned with kernel validation without exporting more
	// symbols than necessary: validate a synthetic open decision and read back the
	// same deterministic recomputation inputs. The actual math function is bridged
	// from a small kernel wrapper in this change set.
	return kernel.RuntimeRecomputeRiskRewardWithExecutionConstraints(action, rr, c)
}

func appendRuntimePolicyNote(decision *kernel.Decision, note string) {
	if decision == nil || decision.EntryProtection == nil || strings.TrimSpace(note) == "" {
		return
	}
	decision.EntryProtection.AlignmentNotes = append(decision.EntryProtection.AlignmentNotes, note)
}

func compactExecutionConstraintSources(snapshot *ExecutionConstraintsSnapshot) []string {
	if snapshot == nil || len(snapshot.Source) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(snapshot.Source))
	for _, source := range snapshot.Source {
		source = strings.TrimSpace(source)
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		out = append(out, source)
	}
	return out
}
