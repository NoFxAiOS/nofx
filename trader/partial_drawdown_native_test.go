package trader

import (
	"testing"

	"nofx/store"
)

func TestBuildManagedPartialDrawdownPlanCandidate_Long(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  50,
	}

	plan := buildManagedPartialDrawdownPlanCandidate(100, "open_long", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if plan.Mode != "drawdown_partial_managed" {
		t.Fatalf("expected managed mode, got %q", plan.Mode)
	}
	if !plan.RequiresPartialClose {
		t.Fatal("expected partial-close requirement")
	}
	if len(plan.TakeProfitOrders) != 1 {
		t.Fatalf("expected one managed partial order, got %d", len(plan.TakeProfitOrders))
	}
	if plan.TakeProfitOrders[0].CloseRatioPct != 50 {
		t.Fatalf("expected close ratio 50, got %.2f", plan.TakeProfitOrders[0].CloseRatioPct)
	}
	if plan.TakeProfitOrders[0].Price <= 0 {
		t.Fatalf("expected positive derived price, got %.4f", plan.TakeProfitOrders[0].Price)
	}
}

func TestBuildManagedPartialDrawdownPlanCandidate_Short(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  40,
	}

	plan := buildManagedPartialDrawdownPlanCandidate(100, "open_short", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if plan.Mode != "drawdown_partial_managed" {
		t.Fatalf("expected managed mode, got %q", plan.Mode)
	}
	if plan.TakeProfitOrders[0].Price <= 0 {
		t.Fatalf("expected positive derived price, got %.4f", plan.TakeProfitOrders[0].Price)
	}
}

func TestBuildManagedPartialDrawdownPlanCandidate_IgnoresFullClose(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  100,
	}
	if plan := buildManagedPartialDrawdownPlanCandidate(100, "open_long", rule); plan != nil {
		t.Fatal("expected nil for full-close rule")
	}
}

func TestBuildManagedPartialDrawdownPlanCandidate_CarriesRunnerStateAndSuppressesBE(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:       10,
		MaxDrawdownPct:     20,
		CloseRatioPct:      70,
		StageName:          "lock_first_profit",
		RunnerKeepPct:      30,
		RunnerStopMode:     "structure",
		RunnerStopSource:   "adjacent_support_flip",
		RunnerTargetMode:   "structure",
		RunnerTargetSource: "primary_resistance",
	}

	plan := buildManagedPartialDrawdownPlanCandidate(100, "open_long", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if plan.DrawdownRunnerState == nil {
		t.Fatal("expected runner state")
	}
	if !plan.BreakEvenSuppressedByRunner {
		t.Fatal("expected break-even suppression by runner")
	}
	if plan.DrawdownRunnerState.StageName != "lock_first_profit" {
		t.Fatalf("expected stage name carried, got %q", plan.DrawdownRunnerState.StageName)
	}
	if plan.DrawdownRunnerState.RunnerKeepPct != 30 {
		t.Fatalf("expected runner keep 30, got %.2f", plan.DrawdownRunnerState.RunnerKeepPct)
	}
}
