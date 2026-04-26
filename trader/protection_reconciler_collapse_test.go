package trader

import "testing"

func TestReconcilerCollapsesLadderStopsToSingleExecutableStop(t *testing.T) {
	plan := &ProtectionPlan{StopLossOrders: []ProtectionOrder{{Price: 101.5, CloseRatioPct: 50}, {Price: 100.9, CloseRatioPct: 50}}, RequiresPartialClose: true}
	collapseLadderStopsToTightestFullStop(plan, "open_short")
	missingSL, missingTP := detectMissingProtection([]OpenOrder{{PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 100.9}}, "SHORT", plan)
	if missingSL || missingTP {
		t.Fatalf("expected collapsed single stop to satisfy reconciler plan, missingSL=%v missingTP=%v plan=%+v", missingSL, missingTP, plan)
	}
}
