package trader

import (
	"strings"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

// evaluateShadowQualityGate centralizes deterministic gate checks as a pure,
// record-only evaluator. It intentionally does not mutate decisions or runtime
// policy outcomes.
func evaluateShadowQualityGate(decision *kernel.Decision, data *market.Data, minRR float64, minConfidence int) *store.DecisionActionQualityGate {
	gate := buildShadowQualityGate(decision)
	if gate == nil || decision == nil {
		return gate
	}
	if !isOpenAction(decision.Action) {
		return gate
	}

	failed := dedupeStrings(gate.FailedChecks)
	if minConfidence > 0 && decision.Confidence > 0 && decision.Confidence < minConfidence {
		failed = appendMissing(failed, "runtime_confidence_below_min")
	}
	if minRR > 0 && gate.NetRR > 0 && gate.NetRR+0.02 < minRR {
		failed = appendMissing(failed, "runtime_net_rr_below_min")
	}
	if data != nil {
		regime := classifyProtectionRegime(data)
		if gate.Regime == "" {
			gate.Regime = regime
		}
		if !isTrendAligned(decision.Action, data) {
			failed = appendMissing(failed, "trend_alignment_failed")
		}
		if isRangeMiddleRegime(gate.Regime) && !isRangeEdgeSetup(decision.SetupType) {
			failed = appendMissing(failed, "range_middle_without_edge_setup")
		}
	}

	gate.FailedChecks = failed
	gate.Passed = len(failed) == 0
	if gate.Passed {
		gate.Decision = "would_pass"
	} else {
		gate.Decision = "would_block"
	}
	return gate
}

func isRangeMiddleRegime(regime string) bool {
	r := strings.ToLower(strings.TrimSpace(regime))
	switch r {
	case "range", "standard", "narrow", "wide":
		return true
	default:
		return false
	}
}

func isRangeEdgeSetup(setup string) bool {
	switch strings.ToLower(strings.TrimSpace(setup)) {
	case "range_edge", "breakout_retest":
		return true
	default:
		return false
	}
}

func appendMissing(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func dedupeStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
