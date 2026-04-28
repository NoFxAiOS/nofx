package trader

import "testing"

func TestClassifyUnexpectedProtectionOrdersSeparatesManualForeignFromBotDuplicate(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 100.9}
	orders := []OpenOrder{
		{OrderID: "keep_sl", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 100.9},
		{OrderID: "4c363c81edc5bcde_ladder_sl_old", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 105},
		{OrderID: "manual-protective-stop", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 110},
	}

	summary := classifyUnexpectedProtectionOrders(orders, "SHORT", plan, false, false, true)
	if summary.ExpectedStaticOwner != 1 {
		t.Fatalf("expected one static owner, got %+v", summary)
	}
	if summary.StaleBotDuplicate != 1 || len(summary.StaleBotDuplicateIDs) != 1 || summary.StaleBotDuplicateIDs[0] != "4c363c81edc5bcde_ladder_sl_old" {
		t.Fatalf("expected one stale bot duplicate id, got %+v", summary)
	}
	if summary.ManualOrForeign != 1 || len(summary.ManualOrForeignIDs) != 1 || summary.ManualOrForeignIDs[0] != "manual-protective-stop" {
		t.Fatalf("expected one manual/foreign id, got %+v", summary)
	}

	ids := collectUnexpectedProtectionOrderIDs(orders, "SHORT", plan, false, false)
	if len(ids) != 1 || ids[0] != "4c363c81edc5bcde_ladder_sl_old" {
		t.Fatalf("expected cleanup ids to include only stale bot duplicate, got %+v", ids)
	}
}

func TestClassifyUnexpectedProtectionOrdersOrphanForInactivePosition(t *testing.T) {
	orders := []OpenOrder{{OrderID: "native_trailing_old", PositionSide: "SHORT", Type: "TRAILING_STOP_MARKET", StopPrice: 99, CallbackRate: 0.02}}
	summary := classifyUnexpectedProtectionOrders(orders, "SHORT", nil, false, false, false)
	if summary.OrphanForInactive != 1 || len(summary.OrphanForInactiveIDs) != 1 || summary.OrphanForInactiveIDs[0] != "native_trailing_old" {
		t.Fatalf("expected inactive trailing order classified as orphan, got %+v", summary)
	}
}

func TestClassifyUnexpectedProtectionOrdersExpectedDynamicOwners(t *testing.T) {
	orders := []OpenOrder{
		{OrderID: "be-stop", PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 100},
		{OrderID: "native_trailing_1", PositionSide: "LONG", Type: "TRAILING_STOP_MARKET", StopPrice: 105, CallbackRate: 0.02},
	}
	summary := classifyUnexpectedProtectionOrders(orders, "LONG", nil, true, true, true)
	if summary.ExpectedDynamicOwner != 2 {
		t.Fatalf("expected break-even and trailing as dynamic owners, got %+v", summary)
	}
	if summary.StaleBotDuplicate != 0 || summary.ManualOrForeign != 0 {
		t.Fatalf("expected no unexpected categories for dynamic owners, got %+v", summary)
	}
}
