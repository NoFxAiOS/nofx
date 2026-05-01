package trader

import (
	"fmt"
	"strings"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

type regimeStructureGateResult struct {
	Allowed       bool
	Reason        string
	Regime        string
	AllowedSetups []string
	Mode          store.StrategyControlPolicyMode
}

func evaluateRegimeStructureGate(decision *kernel.Decision, data *market.Data, mode store.StrategyControlPolicyMode) regimeStructureGateResult {
	result := regimeStructureGateResult{Allowed: true, Mode: mode}
	if decision == nil || !isOpenAction(decision.Action) || data == nil {
		return result
	}
	guidance := market.BuildRegimeEntryGuidance(data, market.BuildMarketStructureBrief(data), market.BuildDerivativesContext(data), data.QuantContext, market.BuildExchangeFlowContext(data))
	if guidance == nil {
		return result
	}
	result.Regime = guidance.Regime
	result.AllowedSetups = guidance.AllowedSetups
	setup := strings.ToLower(strings.TrimSpace(decision.SetupType))
	if setup == "" || setup == "none" {
		setup = "none"
	}
	allowed := false
	for _, item := range guidance.AllowedSetups {
		if strings.EqualFold(item, setup) {
			allowed = true
			break
		}
	}
	if !allowed {
		result.Allowed = false
		result.Reason = fmt.Sprintf("regime-structure gate: setup_type=%s incompatible with regime=%s allowed=%s", setup, guidance.Regime, strings.Join(guidance.AllowedSetups, ","))
		return result
	}
	if guidance.Regime == "squeeze_risk" || guidance.Regime == "crowded" {
		if decision.Confidence > 0 && decision.Confidence < 80 {
			result.Allowed = false
			result.Reason = fmt.Sprintf("regime-structure gate: %s requires confidence >=80 under crowded/squeeze regime", guidance.Regime)
			return result
		}
		if decision.EntryProtection != nil && decision.EntryProtection.RiskReward.NetEstimatedRR > 0 && decision.EntryProtection.RiskReward.NetEstimatedRR < 2.5 {
			result.Allowed = false
			result.Reason = fmt.Sprintf("regime-structure gate: %s requires net RR >=2.5", guidance.Regime)
			return result
		}
	}
	return result
}

func applyRegimeStructureGatePolicy(result regimeStructureGateResult) runtimePolicyResult {
	policy := runtimePolicyResult{Decision: "accepted", OriginalAction: "open"}
	if result.Allowed || result.Reason == "" {
		return policy
	}
	switch effectiveRuntimePolicyMode(result.Mode) {
	case store.StrategyControlPolicyModeRecommendOnly:
		policy.Decision = "downgraded_to_wait"
		policy.Reason = result.Reason
	case store.StrategyControlPolicyModeAuditOnly:
		policy.Decision = "accepted"
		policy.Reason = "audit only: " + result.Reason
	default:
		policy.Decision = "blocked"
		policy.Blocked = true
		policy.Reason = result.Reason
	}
	return policy
}
