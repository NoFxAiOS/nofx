package trader

import "testing"

func TestCollectUnexpectedProtectionOrderIDs(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 100.9}
	orders := []OpenOrder{
		{OrderID: "keep_sl", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 100.9},
		{OrderID: "4c363c81edc5bcde_old_sl", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 105},
	}
	ids := collectUnexpectedProtectionOrderIDs(orders, "SHORT", plan, false, false)
	if len(ids) != 1 || ids[0] != "4c363c81edc5bcde_old_sl" {
		t.Fatalf("expected only bot-created old_sl unexpected, got %+v", ids)
	}
}
