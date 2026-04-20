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
