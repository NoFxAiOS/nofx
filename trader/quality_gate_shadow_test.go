package trader

import (
	"testing"

	"nofx/kernel"
)

func TestBuildShadowQualityGateBlocksWeakOpenInShadowOnly(t *testing.T) {
	gate := buildShadowQualityGate(&kernel.Decision{
		Action:          "open_long",
		Regime:          "chop",
		SetupType:       "none",
		Confidence:      60,
		QualityScore:    &kernel.AIQualityScore{Total: 60},
		EntryProtection: &kernel.AIEntryProtectionRationale{RiskReward: kernel.AIRiskRewardRationale{NetEstimatedRR: 1.4}},
	})
	if gate == nil || !gate.ShadowMode || gate.Passed || gate.Decision != "would_block" {
		t.Fatalf("expected shadow would_block, got %+v", gate)
	}
	if len(gate.FailedChecks) != 4 {
		t.Fatalf("expected four failed checks, got %+v", gate.FailedChecks)
	}
}

func TestBuildShadowQualityGatePassesStrongOpen(t *testing.T) {
	gate := buildShadowQualityGate(&kernel.Decision{
		Action:          "open_short",
		Regime:          "trend_down",
		SetupType:       "trend_pullback",
		Confidence:      82,
		QualityScore:    &kernel.AIQualityScore{Total: 82},
		EntryProtection: &kernel.AIEntryProtectionRationale{RiskReward: kernel.AIRiskRewardRationale{NetEstimatedRR: 2.8}},
	})
	if gate == nil || !gate.ShadowMode || !gate.Passed || gate.Decision != "would_pass" || len(gate.FailedChecks) != 0 {
		t.Fatalf("expected shadow pass, got %+v", gate)
	}
}

func TestBuildShadowQualityGateDoesNotBlockHold(t *testing.T) {
	gate := buildShadowQualityGate(&kernel.Decision{Action: "hold", Regime: "chop", SetupType: "none", Confidence: 20})
	if gate == nil || !gate.Passed || gate.Decision != "would_pass" {
		t.Fatalf("expected hold to remain pass in shadow gate, got %+v", gate)
	}
}
