package trader

import "nofx/store"

func attachQualityGateReviewSummary(record *store.DecisionRecord) {
	if record == nil || len(record.Decisions) == 0 {
		return
	}
	blocked := make(map[string]interface{})
	for _, result := range record.Decisions {
		if result.ReviewContext == nil || result.ReviewContext.QualityGate == nil {
			continue
		}
		gate := result.ReviewContext.QualityGate
		if gate.Passed {
			continue
		}
		blocked[result.Symbol] = map[string]interface{}{
			"action":        result.Action,
			"decision":      gate.Decision,
			"failed_checks": append([]string{}, gate.FailedChecks...),
			"regime":        gate.Regime,
			"setup_type":    gate.SetupType,
			"confidence":    gate.Confidence,
			"quality_total": gate.QualityTotal,
			"net_rr":        gate.NetRR,
		}
	}
	if len(blocked) == 0 {
		return
	}
	if record.ReviewContext == nil {
		record.ReviewContext = map[string]interface{}{}
	}
	record.ReviewContext["quality_gate_shadow"] = blocked
	record.ReviewContext["quality_gate_shadow_record_only"] = true
}
