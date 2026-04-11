package trader

import (
	"testing"

	"nofx/store"
)

func TestBuildPartialDrawdownNativePlanCandidate_Long(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  50,
	}

	plan := buildPartialDrawdownNativePlanCandidate(100, "open_long", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if !plan.RequiresPartialClose {
		t.Fatal("expected partial-close requirement")
	}
	if len(plan.TakeProfitOrders) != 1 {
		t.Fatalf("expected one native partial order, got %d", len(plan.TakeProfitOrders))
	}
	if plan.TakeProfitOrders[0].CloseRatioPct != 50 {
		t.Fatalf("expected close ratio 50, got %.2f", plan.TakeProfitOrders[0].CloseRatioPct)
	}
	if plan.TakeProfitOrders[0].Price <= 0 {
		t.Fatalf("expected positive derived price, got %.4f", plan.TakeProfitOrders[0].Price)
	}
}

func TestBuildPartialDrawdownNativePlanCandidate_Short(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  40,
	}

	plan := buildPartialDrawdownNativePlanCandidate(100, "open_short", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if plan.TakeProfitOrders[0].Price <= 0 {
		t.Fatalf("expected positive derived price, got %.4f", plan.TakeProfitOrders[0].Price)
	}
}

func TestBuildPartialDrawdownNativePlanCandidate_IgnoresFullClose(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  100,
	}
	if plan := buildPartialDrawdownNativePlanCandidate(100, "open_long", rule); plan != nil {
		t.Fatal("expected nil for full-close rule")
	}
}
