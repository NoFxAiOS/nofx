package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/market"
)

func TestEvaluateShadowQualityGateAddsTrendAndRangeChecks(t *testing.T) {
	data := &market.Data{
		CurrentPrice:  100,
		CurrentEMA20:  110,
		PriceChange4h: -2,
		PriceChange1h: -1,
		CurrentMACD:   -1,
	}
	gate := evaluateShadowQualityGate(&kernel.Decision{
		Action:          "open_long",
		Regime:          "range",
		SetupType:       "trend_pullback",
		Confidence:      72,
		QualityScore:    &kernel.AIQualityScore{Total: 72},
		EntryProtection: &kernel.AIEntryProtectionRationale{RiskReward: kernel.AIRiskRewardRationale{NetEstimatedRR: 1.9}},
	}, data, 2.5, 75)
	if gate == nil || gate.Passed || gate.Decision != "would_block" {
		t.Fatalf("expected would_block, got %+v", gate)
	}
	want := map[string]bool{
		"confidence_below_75":             true,
		"runtime_confidence_below_min":    true,
		"net_rr_below_2_5":                true,
		"runtime_net_rr_below_min":        true,
		"trend_alignment_failed":          true,
		"range_middle_without_edge_setup": true,
	}
	for _, check := range gate.FailedChecks {
		delete(want, check)
	}
	if len(want) != 0 {
		t.Fatalf("missing failed checks: %+v (gate=%+v)", want, gate)
	}
}

func TestEvaluateShadowQualityGatePassesAlignedBreakoutRetest(t *testing.T) {
	data := &market.Data{
		CurrentPrice:  100,
		CurrentEMA20:  95,
		PriceChange4h: 3,
		PriceChange1h: 1,
		CurrentMACD:   1,
	}
	gate := evaluateShadowQualityGate(&kernel.Decision{
		Action:          "open_long",
		Regime:          "range",
		SetupType:       "breakout_retest",
		Confidence:      85,
		QualityScore:    &kernel.AIQualityScore{Total: 85},
		EntryProtection: &kernel.AIEntryProtectionRationale{RiskReward: kernel.AIRiskRewardRationale{NetEstimatedRR: 3.1}},
	}, data, 2.5, 75)
	if gate == nil || !gate.Passed || gate.Decision != "would_pass" || len(gate.FailedChecks) != 0 {
		t.Fatalf("expected would_pass, got %+v", gate)
	}
}
