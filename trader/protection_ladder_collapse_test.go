package trader

import "testing"

func TestCollapseLadderStopsToTightestFullStopForShort(t *testing.T) {
	plan := &ProtectionPlan{StopLossOrders: []ProtectionOrder{{Price: 101.5, CloseRatioPct: 50}, {Price: 100.9, CloseRatioPct: 50}}, RequiresPartialClose: true}
	collapseLadderStopsToTightestFullStop(plan, "open_short")
	if len(plan.StopLossOrders) != 0 || !plan.NeedsStopLoss || !almostEqual(plan.StopLossPrice, 100.9) {
		t.Fatalf("expected tightest short stop collapsed to full stop, got %+v", plan)
	}
}

func TestCollapseLadderStopsToTightestFullStopForLong(t *testing.T) {
	plan := &ProtectionPlan{StopLossOrders: []ProtectionOrder{{Price: 98, CloseRatioPct: 50}, {Price: 96, CloseRatioPct: 50}}, RequiresPartialClose: true}
	collapseLadderStopsToTightestFullStop(plan, "open_long")
	if len(plan.StopLossOrders) != 0 || !plan.NeedsStopLoss || !almostEqual(plan.StopLossPrice, 98) {
		t.Fatalf("expected tightest long stop collapsed to full stop, got %+v", plan)
	}
}
