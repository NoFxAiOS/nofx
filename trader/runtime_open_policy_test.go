package trader

import (
	"strings"
	"testing"

	"nofx/kernel"
)

func TestApplyRuntimeOpenPolicyMergesRuntimeConstraintsAndBlocksLowNetRR(t *testing.T) {
	decision := &kernel.Decision{
		Symbol: "BTCUSDT",
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     90,
				FirstTarget:      120,
				GrossEstimatedRR: 2.0,
				Passed:           true,
			},
		},
	}
	snapshot := &ExecutionConstraintsSnapshot{
		TickSize:             1,
		TakerFeeRate:         0.02,
		EstimatedSlippageBps: 200,
		Source:               map[string]string{"tick_size": "test"},
	}

	result := applyRuntimeOpenPolicy(decision, snapshot, 1.5, "")

	if !result.Blocked || !strings.Contains(result.Reason, "execution-aware rr") {
		t.Fatalf("expected runtime policy block, got %+v", result)
	}
	if result.ReasonCode != "runtime_rr_below_min" {
		t.Fatalf("expected stable reason code, got %+v", result)
	}
	if !result.ConstraintsMerged || !result.RRRecomputed || result.EffectiveRRSource != "runtime_net" {
		t.Fatalf("expected compact runtime audit flags, got %+v", result)
	}
	if result.OriginalAction != "open_long" || result.FinalAction != "open_long" {
		t.Fatalf("expected original/final action audit to preserve strict reject action, got %+v", result)
	}
	if result.AIGrossRR != 2.0 || result.RuntimeNetRR <= 0 {
		t.Fatalf("expected ai/runtime rr summary, got %+v", result)
	}
	if decision.EntryProtection.ExecutionConstraints.TickSize != 1 {
		t.Fatalf("expected runtime constraints to be merged: %+v", decision.EntryProtection.ExecutionConstraints)
	}
	if decision.EntryProtection.RiskReward.NetEstimatedRR >= 1.5 {
		t.Fatalf("expected recomputed net RR below min, got %+v", decision.EntryProtection.RiskReward)
	}
	if decision.EntryProtection.RiskReward.Passed {
		t.Fatalf("expected system-controlled passed=false after runtime recompute")
	}
}

func TestApplyRuntimeOpenPolicyPreservesWaitAndEmptyLegality(t *testing.T) {
	decision := &kernel.Decision{Symbol: "BTCUSDT", Action: "wait", Reasoning: "no trade"}
	result := applyRuntimeOpenPolicy(decision, &ExecutionConstraintsSnapshot{TickSize: 1, Source: map[string]string{"tick_size": "test"}}, 1.5, "")
	if result.Blocked || result.Reason != "" {
		t.Fatalf("wait should not be blocked by runtime open policy: %+v", result)
	}
	if decision.EntryProtection != nil {
		t.Fatalf("wait decision should not gain entry protection: %+v", decision.EntryProtection)
	}
}

func TestApplyRuntimeOpenPolicyKeepsPassingOpenExecutable(t *testing.T) {
	decision := &kernel.Decision{
		Symbol: "ETHUSDT",
		Action: "open_short",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     110,
				FirstTarget:      70,
				GrossEstimatedRR: 3.0,
				Passed:           true,
			},
		},
	}

	result := applyRuntimeOpenPolicy(decision, &ExecutionConstraintsSnapshot{TickSize: 1, Source: map[string]string{"tick_size": "test"}}, 1.5, "")
	if result.Blocked {
		t.Fatalf("expected passing open to remain executable: %+v", result)
	}
	if !result.ConstraintsMerged || !result.RRRecomputed || result.EffectiveRRSource != "runtime_net" {
		t.Fatalf("expected runtime audit summary, got %+v", result)
	}
	if result.OriginalAction != "open_short" || result.FinalAction != "open_short" || result.Decision != "accepted" {
		t.Fatalf("expected accepted action audit to preserve original/final action, got %+v", result)
	}
	if decision.EntryProtection.RiskReward.NetEstimatedRR < 1.5 {
		t.Fatalf("unexpected low runtime net RR: %+v", decision.EntryProtection.RiskReward)
	}
	if !decision.EntryProtection.RiskReward.Passed {
		t.Fatalf("expected passed to remain true after runtime recompute")
	}
}

func TestApplyRuntimeOpenPolicyAuditOnlyFlagsLowNetRRWithoutBlocking(t *testing.T) {
	decision := &kernel.Decision{
		Symbol: "BTCUSDT",
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     90,
				FirstTarget:      120,
				GrossEstimatedRR: 2.0,
				Passed:           true,
			},
		},
	}
	snapshot := &ExecutionConstraintsSnapshot{
		TickSize:             1,
		TakerFeeRate:         0.02,
		EstimatedSlippageBps: 200,
		Source:               map[string]string{"tick_size": "test"},
	}

	result := applyRuntimeOpenPolicy(decision, snapshot, 1.5, "audit_only")

	if result.Blocked {
		t.Fatalf("audit_only should not block low runtime RR: %+v", result)
	}
	if result.Decision != "accepted" || result.ReasonCode != "runtime_rr_below_min" || !strings.Contains(result.Reason, "flagged") {
		t.Fatalf("expected audited failed check without rejection, got %+v", result)
	}
	if !result.RRRecomputed || result.EffectiveRRSource != "runtime_net" {
		t.Fatalf("expected runtime RR audit to be preserved, got %+v", result)
	}
	if decision.EntryProtection.RiskReward.Passed {
		t.Fatalf("expected audit fields to preserve failed runtime RR outcome")
	}
}

func TestApplyRuntimeOpenPolicyRecommendOnlyFlagsLowNetRRWithoutBlocking(t *testing.T) {
	decision := &kernel.Decision{
		Symbol: "BTCUSDT",
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     90,
				FirstTarget:      120,
				GrossEstimatedRR: 2.0,
				Passed:           true,
			},
		},
	}
	snapshot := &ExecutionConstraintsSnapshot{
		TickSize:             1,
		TakerFeeRate:         0.02,
		EstimatedSlippageBps: 200,
		Source:               map[string]string{"tick_size": "test"},
	}

	result := applyRuntimeOpenPolicy(decision, snapshot, 1.5, "recommend_only")

	if result.Blocked {
		t.Fatalf("recommend_only should not block low runtime RR: %+v", result)
	}
	if result.Decision != "accepted" || result.ReasonCode != "runtime_rr_below_min" {
		t.Fatalf("expected recommendation audit failed check without rejection, got %+v", result)
	}
	if !result.RRRecomputed || decision.EntryProtection.RiskReward.Passed {
		t.Fatalf("expected recommendation mode to preserve failed runtime RR audit, got %+v", result)
	}
}
