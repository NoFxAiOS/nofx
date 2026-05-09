package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestDeriveProtectionAlignmentPolicyTransparencyAligned(t *testing.T) {
	decision := &kernel.Decision{
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:        100,
				Invalidation: 95,
				FirstTarget:  110,
			},
		},
	}
	snapshot := &store.ProtectionSnapshot{
		FullTPSL: &store.ProtectionSnapshotFullTPSL{
			StopLoss:   store.ProtectionSnapshotValueSource{Mode: "price", Value: 94},
			TakeProfit: store.ProtectionSnapshotValueSource{Mode: "price", Value: 111},
		},
		BreakEven: &store.ProtectionSnapshotBreakEven{Enabled: true, TriggerMode: "price", TriggerValue: 108},
		LadderTPSL: &store.ProtectionSnapshotLadder{
			FallbackMaxLoss: store.ProtectionSnapshotValueSource{Mode: "price", Value: 94},
		},
	}

	alignment := deriveProtectionAlignment(decision, snapshot)
	if alignment == nil {
		t.Fatal("expected alignment")
	}
	if alignment.PolicyStatus != "aligned" || alignment.PolicyOverride || alignment.PolicyRejected {
		t.Fatalf("unexpected policy transparency: %+v", alignment)
	}
	if len(alignment.PolicyReasons) != 0 {
		t.Fatalf("expected no policy reasons, got %+v", alignment.PolicyReasons)
	}
}

func TestDeriveProtectionAlignmentPolicyTransparencyRejected(t *testing.T) {
	decision := &kernel.Decision{
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:        100,
				Invalidation: 95,
				FirstTarget:  110,
			},
		},
	}
	snapshot := &store.ProtectionSnapshot{
		FullTPSL: &store.ProtectionSnapshotFullTPSL{
			StopLoss:   store.ProtectionSnapshotValueSource{Mode: "price", Value: 97},
			TakeProfit: store.ProtectionSnapshotValueSource{Mode: "price", Value: 108},
		},
		BreakEven: &store.ProtectionSnapshotBreakEven{Enabled: true, TriggerMode: "price", TriggerValue: 112},
		LadderTPSL: &store.ProtectionSnapshotLadder{
			FallbackMaxLoss: store.ProtectionSnapshotValueSource{Mode: "price", Value: 97},
		},
	}

	alignment := deriveProtectionAlignment(decision, snapshot)
	if alignment == nil {
		t.Fatal("expected alignment")
	}
	if alignment.PolicyStatus != "rejected" || !alignment.PolicyOverride || !alignment.PolicyRejected {
		t.Fatalf("unexpected policy transparency: %+v", alignment)
	}
	if len(alignment.PolicyReasons) < 2 {
		t.Fatalf("expected compact mismatch reasons, got %+v", alignment.PolicyReasons)
	}
}

func TestDeriveProtectionAlignmentPrefersAIPlanOverAIOwnedSnapshotDefaults(t *testing.T) {
	decision := &kernel.Decision{
		Action: "open_long",
		ProtectionPlan: &kernel.AIProtectionPlan{LadderRules: []kernel.AIProtectionLadderRule{
			{TakeProfitPrice: 2301.25, StopLossPrice: 2292.78},
			{TakeProfitPrice: 2307.55, StopLossPrice: 2292.78},
		}},
		EntryProtection: &kernel.AIEntryProtectionRationale{RiskReward: kernel.AIRiskRewardRationale{
			Entry: 2297.85, Invalidation: 2294.04, FirstTarget: 2301.25,
		}},
	}
	snapshot := &store.ProtectionSnapshot{
		LadderTPSL: &store.ProtectionSnapshotLadder{
			Enabled: true,
			Mode:    string(store.ProtectionModeAI),
			FallbackMaxLoss: store.ProtectionSnapshotValueSource{
				Mode:  string(store.ProtectionValueModeManual),
				Value: 2.6,
			},
			Rules: []store.ProtectionSnapshotLadderRule{{TakeProfitPct: 5, StopLossPct: 0.9}, {TakeProfitPct: 9, StopLossPct: 1.5}},
		},
		BreakEven: &store.ProtectionSnapshotBreakEven{Enabled: true, Source: "strategy", TriggerMode: "profit_pct", TriggerValue: 0.7, OffsetPct: 0.3},
	}
	strategy := &store.StrategyConfig{Protection: store.ProtectionConfig{LadderTPSL: store.LadderTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeAI,
		FallbackMaxLoss: store.ProtectionValueSource{
			Mode:  store.ProtectionValueModeManual,
			Value: 2.6,
		},
	}}}

	alignment := deriveProtectionAlignmentWithStrategy(decision, snapshot, strategy)
	if alignment == nil {
		t.Fatal("expected alignment")
	}
	if !alignment.StopBeyondInvalidation || !alignment.TargetAligned || !alignment.FallbackWithinEnvelope {
		t.Fatalf("expected AI plan/fallback alignment to pass, got %+v", alignment)
	}
	if alignment.PolicyRejected {
		t.Fatalf("AI-owned configured ladder defaults must not reject structural AI plan: %+v", alignment)
	}
}

func TestDeriveProtectionAlignmentBreakEvenAfterTargetIsWarningOnly(t *testing.T) {
	decision := &kernel.Decision{
		Action:         "open_long",
		ProtectionPlan: &kernel.AIProtectionPlan{LadderRules: []kernel.AIProtectionLadderRule{{TakeProfitPrice: 2301.25, StopLossPrice: 2292.78}}},
		EntryProtection: &kernel.AIEntryProtectionRationale{RiskReward: kernel.AIRiskRewardRationale{
			Entry: 2297.85, Invalidation: 2294.04, FirstTarget: 2301.25,
		}},
	}
	snapshot := &store.ProtectionSnapshot{
		LadderTPSL: &store.ProtectionSnapshotLadder{
			Enabled:         true,
			Mode:            string(store.ProtectionModeAI),
			FallbackMaxLoss: store.ProtectionSnapshotValueSource{Mode: string(store.ProtectionValueModeManual), Value: 2.6},
		},
		BreakEven: &store.ProtectionSnapshotBreakEven{Enabled: true, Source: "strategy", TriggerMode: "profit_pct", TriggerValue: 0.7, OffsetPct: 0.3},
	}
	strategy := &store.StrategyConfig{Protection: store.ProtectionConfig{LadderTPSL: store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI, FallbackMaxLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 2.6}}}}

	alignment := deriveProtectionAlignmentWithStrategy(decision, snapshot, strategy)
	if alignment == nil {
		t.Fatal("expected alignment")
	}
	if alignment.BreakEvenBeforeTarget {
		t.Fatalf("expected BE after first target warning, got %+v", alignment)
	}
	if alignment.PolicyStatus != "recomputed" || alignment.PolicyRejected {
		t.Fatalf("BE after first ladder target should be warning/recompute, not reject: %+v", alignment)
	}
}
