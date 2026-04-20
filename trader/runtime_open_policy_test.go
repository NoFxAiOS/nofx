package trader

import (
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/store"
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

func TestApplyRuntimeOpenPolicyRecommendOnlyDowngradesTargetAlignmentMismatchToWait(t *testing.T) {
	decision := &kernel.Decision{
		Symbol:    "BTCUSDT",
		Action:    "open_long",
		Reasoning: "breakout setup",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     95,
				FirstTarget:      110,
				GrossEstimatedRR: 2.0,
				NetEstimatedRR:   1.8,
				Passed:           true,
			},
		},
	}
	protection := &store.DecisionActionProtectionAlignment{
		StopBeyondInvalidation: true,
		TargetAligned:          false,
		BreakEvenBeforeTarget:  true,
		FallbackWithinEnvelope: true,
		PolicyStatus:           "recomputed",
		PolicyOverride:         true,
		PolicyReasons:          []string{"target_before_first_target"},
	}

	result := applyRuntimeOpenPolicy(decision, nil, 1.5, "recommend_only", protection)

	if result.Blocked {
		t.Fatalf("recommend_only target mismatch should downgrade, not block: %+v", result)
	}
	if result.Decision != "downgraded_to_wait" || result.ReasonCode != "protection_target_before_first_target" {
		t.Fatalf("expected target mismatch downgrade, got %+v", result)
	}
	if result.OriginalAction != "open_long" || result.FinalAction != "wait" {
		t.Fatalf("expected original open/final wait audit fields, got %+v", result)
	}
	if decision.Action != "wait" {
		t.Fatalf("expected executable action to be downgraded to wait, got %q", decision.Action)
	}
	if !strings.Contains(result.Reason, "configured target is before rationale first target") || !strings.Contains(decision.Reasoning, "configured target is before rationale first target") {
		t.Fatalf("expected downgrade reason to be auditable, result=%+v reasoning=%q", result, decision.Reasoning)
	}
}

func TestApplyRuntimeOpenPolicyStrictRejectsTargetAlignmentMismatch(t *testing.T) {
	decision := &kernel.Decision{
		Symbol: "BTCUSDT",
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     95,
				FirstTarget:      110,
				GrossEstimatedRR: 2.0,
				NetEstimatedRR:   1.8,
				Passed:           true,
			},
		},
	}
	protection := &store.DecisionActionProtectionAlignment{
		StopBeyondInvalidation: true,
		TargetAligned:          false,
		BreakEvenBeforeTarget:  true,
		FallbackWithinEnvelope: true,
	}

	result := applyRuntimeOpenPolicy(decision, nil, 1.5, "strict", protection)

	if !result.Blocked || result.Decision != "rejected" || result.ReasonCode != "protection_target_before_first_target" {
		t.Fatalf("expected strict target mismatch rejection, got %+v", result)
	}
	if result.FinalAction != "open_long" || decision.Action != "open_long" {
		t.Fatalf("strict reject should preserve executable action, got result=%+v action=%q", result, decision.Action)
	}
}

func TestApplyRuntimeOpenPolicyAuditOnlyFlagsTargetAlignmentMismatchWithoutBlocking(t *testing.T) {
	decision := &kernel.Decision{
		Symbol: "BTCUSDT",
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     95,
				FirstTarget:      110,
				GrossEstimatedRR: 2.0,
				NetEstimatedRR:   1.8,
				Passed:           true,
			},
		},
	}
	protection := &store.DecisionActionProtectionAlignment{
		StopBeyondInvalidation: true,
		TargetAligned:          false,
		BreakEvenBeforeTarget:  true,
		FallbackWithinEnvelope: true,
	}

	result := applyRuntimeOpenPolicy(decision, nil, 1.5, "audit_only", protection)

	if result.Blocked || result.Decision != "accepted" || result.ReasonCode != "protection_target_before_first_target" {
		t.Fatalf("expected audit_only target mismatch flag without block, got %+v", result)
	}
	if decision.Action != "open_long" {
		t.Fatalf("audit_only should preserve action, got %q", decision.Action)
	}
}

func TestApplyRuntimeOpenPolicyRecommendOnlyDowngradesLowNetRRToWait(t *testing.T) {
	decision := &kernel.Decision{
		Symbol:    "BTCUSDT",
		Action:    "open_long",
		Reasoning: "breakout setup",
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
		t.Fatalf("recommend_only downgrade should not use strict block path: %+v", result)
	}
	if result.Decision != "downgraded_to_wait" || result.ReasonCode != "runtime_rr_below_min" {
		t.Fatalf("expected downgraded low-RR recommendation with failed check, got %+v", result)
	}
	if result.OriginalAction != "open_long" || result.FinalAction != "wait" {
		t.Fatalf("expected original open/final wait audit fields, got %+v", result)
	}
	if decision.Action != "wait" {
		t.Fatalf("expected executable action to be downgraded to wait, got %q", decision.Action)
	}
	if !strings.Contains(result.Reason, "downgraded") || !strings.Contains(decision.Reasoning, "downgraded to wait") {
		t.Fatalf("expected downgrade reason to be auditable, result=%+v reasoning=%q", result, decision.Reasoning)
	}
	if !result.RRRecomputed || decision.EntryProtection.RiskReward.Passed {
		t.Fatalf("expected recommendation mode to preserve failed runtime RR audit, got %+v", result)
	}
}
