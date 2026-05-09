package trader

import (
	"nofx/kernel"
	"nofx/store"
)

func buildShadowQualityGate(decision *kernel.Decision) *store.DecisionActionQualityGate {
	if decision == nil {
		return nil
	}
	gate := &store.DecisionActionQualityGate{
		ShadowMode: true,
		Decision:   "would_pass",
		Passed:     true,
		Regime:     decision.Regime,
		SetupType:  decision.SetupType,
		Confidence: decision.Confidence,
	}
	if decision.QualityScore != nil {
		gate.QualityTotal = decision.QualityScore.Total
	}
	if decision.EntryProtection != nil {
		gate.NetRR = decision.EntryProtection.RiskReward.NetEstimatedRR
	}

	if !isOpenAction(decision.Action) {
		return gate
	}
	if decision.Confidence > 0 && decision.Confidence < 75 {
		gate.FailedChecks = append(gate.FailedChecks, "confidence_below_75")
	}
	if decision.SetupType != "" && decision.SetupType != "trend_pullback" && decision.SetupType != "range_edge" && decision.SetupType != "breakout_retest" {
		gate.FailedChecks = append(gate.FailedChecks, "unsupported_setup_type")
	}
	if gate.NetRR > 0 && gate.NetRR < 2.5 {
		gate.FailedChecks = append(gate.FailedChecks, "net_rr_below_2_5")
	}
	if decision.Regime == "chop" || decision.Regime == "news_risk" || decision.Regime == "no_trade" {
		gate.FailedChecks = append(gate.FailedChecks, "blocked_regime")
	}
	if len(gate.FailedChecks) > 0 {
		gate.Passed = false
		gate.Decision = "would_block"
	}
	return gate
}

func isOpenAction(action string) bool {
	return action == "open_long" || action == "open_short"
}
