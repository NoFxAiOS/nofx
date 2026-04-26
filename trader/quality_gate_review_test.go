package trader

import (
	"testing"

	"nofx/store"
)

func TestAttachQualityGateReviewSummary(t *testing.T) {
	record := &store.DecisionRecord{
		Decisions: []store.DecisionAction{
			{
				Symbol: "BTCUSDT",
				Action: "open_long",
				ReviewContext: &store.DecisionActionReviewContext{
					QualityGate: &store.DecisionActionQualityGate{
						Passed:       false,
						Decision:     "would_block",
						FailedChecks: []string{"runtime_net_rr_below_min"},
						Regime:       "range",
						SetupType:    "trend_pullback",
						Confidence:   70,
						QualityTotal: 68,
						NetRR:        1.8,
					},
				},
			},
			{
				Symbol: "ETHUSDT",
				Action: "wait",
				ReviewContext: &store.DecisionActionReviewContext{
					QualityGate: &store.DecisionActionQualityGate{Passed: true, Decision: "would_pass"},
				},
			},
		},
	}
	attachQualityGateReviewSummary(record)
	if record.ReviewContext == nil {
		t.Fatalf("expected review context")
	}
	shadow, ok := record.ReviewContext["quality_gate_shadow"].(map[string]interface{})
	if !ok || len(shadow) != 1 {
		t.Fatalf("expected one blocked summary, got %#v", record.ReviewContext["quality_gate_shadow"])
	}
	if _, ok := shadow["BTCUSDT"]; !ok {
		t.Fatalf("expected BTCUSDT in blocked summary, got %#v", shadow)
	}
}
