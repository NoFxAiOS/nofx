package trader

import "testing"

func TestReconcilerPreservesVisibleLadderStops(t *testing.T) {
	plan := &ProtectionPlan{StopLossOrders: []ProtectionOrder{{Price: 101.5, CloseRatioPct: 50}, {Price: 100.9, CloseRatioPct: 50}}, RequiresPartialClose: true}
	openOrders := []OpenOrder{
		{PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 101.5},
		{PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 100.9},
	}
	missingSL, missingTP := detectMissingProtection(openOrders, "SHORT", plan)
	unexpectedSL, unexpectedTP := detectUnexpectedProtectionOrders(openOrders, "SHORT", plan, false, false)
	if missingSL || missingTP || unexpectedSL != 0 || unexpectedTP != 0 {
		t.Fatalf("expected visible ladder stops to be preserved, missingSL=%v missingTP=%v unexpectedSL=%d unexpectedTP=%d", missingSL, missingTP, unexpectedSL, unexpectedTP)
	}
}
