package trader

import "testing"

func TestReconcilerOwnershipZeroOrdersCannotBeVerified(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 98, NeedsTakeProfit: true, TakeProfitPrice: 110}
	state := evaluateProtectionOwnership(nil, "SHORT", plan, false, false)
	if state.Verified {
		t.Fatalf("zero open orders must not verify protection ownership: %+v", state)
	}
	if state.State != "unprotected" || !state.MissingStop || !state.MissingProfit {
		t.Fatalf("expected explicit unprotected missing stop/profit state, got %+v", state)
	}
}

func TestReconcilerOwnershipDrawdownDoesNotSatisfyStopOwner(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 98, NeedsTakeProfit: true, TakeProfitPrice: 110}
	state := evaluateProtectionOwnership(nil, "SHORT", plan, false, true)
	if state.Verified {
		t.Fatalf("drawdown profit owner must not verify missing stop owner: %+v", state)
	}
	if state.StopOwner != "" || state.ProfitOwner != "drawdown" || !state.MissingStop || state.MissingProfit {
		t.Fatalf("expected drawdown-only degraded ownership, got %+v", state)
	}
}
