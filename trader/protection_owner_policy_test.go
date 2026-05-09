package trader

import (
	"testing"

	"nofx/store"
)

func TestEvaluateProtectionOwnerPolicyPrefersDrawdownProfitAndLadderStop(t *testing.T) {
	policy := evaluateProtectionOwnerPolicy(store.ProtectionConfig{
		FullTPSL:           store.FullTPSLConfig{Enabled: true, StopLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 2}, TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 4}},
		LadderTPSL:         store.LadderTPSLConfig{Enabled: true, StopLossEnabled: true, TakeProfitEnabled: true},
		DrawdownTakeProfit: store.DrawdownTakeProfitConfig{Enabled: true},
		BreakEvenStop:      store.BreakEvenStopConfig{Enabled: true},
	})
	if policy.StopOwner != "ladder" || !policy.UseLadderStops {
		t.Fatalf("expected ladder stop owner, got %+v", policy)
	}
	if policy.ProfitOwner != "drawdown" || !policy.UseDrawdownTP || !policy.SuppressStaticTP {
		t.Fatalf("expected drawdown profit owner, got %+v", policy)
	}
}

func TestEvaluateProtectionOwnerPolicyFallsBackToFullOwners(t *testing.T) {
	policy := evaluateProtectionOwnerPolicy(store.ProtectionConfig{FullTPSL: store.FullTPSLConfig{Enabled: true, StopLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 2}, TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 4}}})
	if policy.StopOwner != "full" || policy.ProfitOwner != "full" || policy.SuppressStaticTP {
		t.Fatalf("expected full owners, got %+v", policy)
	}
}
