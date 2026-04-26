package trader

import "testing"

func TestCollectUnexpectedProtectionOrderIDs(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 100.9}
	orders := []OpenOrder{
		{OrderID: "keep_sl", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 100.9},
		{OrderID: "old_sl", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 105},
	}
	ids := collectUnexpectedProtectionOrderIDs(orders, "SHORT", plan, false, false)
	if len(ids) != 1 || ids[0] != "old_sl" {
		t.Fatalf("expected only old_sl unexpected, got %+v", ids)
	}
}
