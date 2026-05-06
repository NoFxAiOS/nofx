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
		if guidance := market.BuildRegimeEntryGuidance(data, market.BuildMarketStructureBrief(data), market.BuildDerivativesContext(data), data.QuantContext, market.BuildExchangeFlowContext(data)); guidance != nil && guidance.Regime != "" {
			regime = guidance.Regime
			allowed := false
			setup := strings.ToLower(strings.TrimSpace(decision.SetupType))
			for _, item := range guidance.AllowedSetups {
				if strings.EqualFold(item, setup) {
					allowed = true
					break
				}
			}
			if !allowed {
				failed = appendMissing(failed, "regime_structure_mismatch")
			}
		}
		if gate.Regime == "" {
			gate.Regime = regime
		}
		if !isTrendAligned(decision.Action, data) {
			failed = appendMissing(failed, "trend_alignment_failed")
		}
		if isRangeMiddleRegime(gate.Regime) && !isRangeEdgeSetup(decision.SetupType) {
			failed = appendMissing(failed, "range_middle_without_edge_setup")
		}
		// Phase 3: Cross-validate AI-reported regime against system detection
		if decision.Regime != "" {
			systemRegime := market.InferExecutionRegimePublic(data)
			if isRegimeCrossValidationFailed(decision.Regime, systemRegime) {
				// Breakout retests occur at regime transitions — AI reports the prior
				// structure (range) while the system sees the new trend. Allow when
				// the trade direction is aligned with the detected trend.
				setup := strings.ToLower(strings.TrimSpace(decision.SetupType))
				exempt := setup == "breakout_retest" && isTrendAlignedWithRegime(decision.Action, systemRegime)
				if !exempt {
					failed = appendMissing(failed, "regime_cross_validation_failed")
				}
			}
		}
		// Phase 6.2: Shorts in non-trending regimes require higher confidence
		if isShortAction(decision.Action) && !isTrendDownRegime(gate.Regime) {
			shortMinConf := 85
			if decision.Confidence > 0 && decision.Confidence < shortMinConf {
				failed = appendMissing(failed, "short_confidence_below_regime_min")
			}
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

// isRegimeCrossValidationFailed detects when the AI claims a regime that
// contradicts the system's own detection from market data.
func isRegimeCrossValidationFailed(aiRegime, systemRegime string) bool {
	ai := strings.ToLower(strings.TrimSpace(aiRegime))
	sys := strings.ToLower(strings.TrimSpace(systemRegime))
	if sys == "balanced" || sys == "" {
		return false
	}
	switch sys {
	case "trend_up":
		return ai == "trend_down" || ai == "range" || ai == "balanced"
	case "trend_down":
		return ai == "trend_up" || ai == "range" || ai == "balanced"
	case "squeeze_risk", "crowded":
		return ai == "balanced"
	}
	return false
}

func isTrendAlignedWithRegime(action, systemRegime string) bool {
	a := strings.ToLower(strings.TrimSpace(action))
	switch systemRegime {
	case "trend_up":
		return a == "open_long"
	case "trend_down":
		return a == "open_short"
	}
	return false
}

func isShortAction(action string) bool {
	return strings.ToLower(action) == "open_short"
}

func isTrendDownRegime(regime string) bool {
	r := strings.ToLower(strings.TrimSpace(regime))
	return r == "trend_down" || r == "trending_down"
}

// enforcedQualityGateChecks are the checks that block execution when failed.
var enforcedQualityGateChecks = map[string]bool{
	"trend_alignment_failed":          true,
	"regime_cross_validation_failed":  true,
	"runtime_confidence_below_min":    true,
	"short_confidence_below_regime_min": true,
}

// HasEnforcedFailure returns true if any of the failed checks are in the enforced set.
func HasEnforcedFailure(failedChecks []string) bool {
	for _, check := range failedChecks {
		if enforcedQualityGateChecks[check] {
			return true
		}
	}
	return false
}
