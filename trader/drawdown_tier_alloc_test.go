package trader

import (
	"nofx/store"
	"testing"
)

func TestComputeDrawdownTierAllocations(t *testing.T) {
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 3, MaxDrawdownPct: 40, CloseRatioPct: 50, StageName: "T1"},
		{MinProfitPct: 6, MaxDrawdownPct: 30, CloseRatioPct: 25, StageName: "T2"},
		{MinProfitPct: 10, MaxDrawdownPct: 25, CloseRatioPct: 25, StageName: "T3"},
	}

	allocs := computeDrawdownTierAllocations(100.0, rules)
	if len(allocs) != 3 {
		t.Fatalf("expected 3 tiers, got %d", len(allocs))
	}

	// T1: 50% of 100 = 50
	if allocs[0].Quantity != 50 {
		t.Fatalf("T1 qty: expected 50, got %.4f", allocs[0].Quantity)
	}
	if allocs[0].StageName != "T1" {
		t.Fatalf("T1 stage: expected T1, got %s", allocs[0].StageName)
	}
	if allocs[0].Status != "pending" {
		t.Fatalf("T1 status: expected pending, got %s", allocs[0].Status)
	}

	// T2: 25% of 100 = 25
	if allocs[1].Quantity != 25 {
		t.Fatalf("T2 qty: expected 25, got %.4f", allocs[1].Quantity)
	}

	// T3: 25% of 100 = 25
	if allocs[2].Quantity != 25 {
		t.Fatalf("T3 qty: expected 25, got %.4f", allocs[2].Quantity)
	}
}

func TestComputeDrawdownTierAllocationsExceedsTotal(t *testing.T) {
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 3, MaxDrawdownPct: 40, CloseRatioPct: 60},
		{MinProfitPct: 6, MaxDrawdownPct: 30, CloseRatioPct: 60},
	}

	allocs := computeDrawdownTierAllocations(100.0, rules)
	if len(allocs) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(allocs))
	}

	// First: 60% = 60
	if allocs[0].Quantity != 60 {
		t.Fatalf("T1 qty: expected 60, got %.4f", allocs[0].Quantity)
	}
	// Second: capped to remaining 40% = 40
	if allocs[1].Quantity != 40 {
		t.Fatalf("T2 qty: expected 40, got %.4f", allocs[1].Quantity)
	}
}

func TestComputeDrawdownTierAllocationsSortsByMinProfit(t *testing.T) {
	// Rules in reverse order should still be sorted by MinProfitPct
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 10, MaxDrawdownPct: 25, CloseRatioPct: 20},
		{MinProfitPct: 3, MaxDrawdownPct: 40, CloseRatioPct: 50},
	}

	allocs := computeDrawdownTierAllocations(100.0, rules)
	if len(allocs) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(allocs))
	}
	if allocs[0].MinProfitPct != 3 {
		t.Fatalf("first tier should be min_profit=3, got %.2f", allocs[0].MinProfitPct)
	}
}

func TestResolveDrawdownRulesWithModes(t *testing.T) {
	strategy := []store.DrawdownTakeProfitRule{
		{
			MinProfitPct:    3,
			MaxDrawdownPct:  40,
			CloseRatioPct:   50,
			CloseRatioMode:  store.ProtectionValueModeManual,
			MinProfitMode:   store.ProtectionValueModeAI,
			MaxDrawdownMode: store.ProtectionValueModeManual,
		},
	}

	ai := []store.DrawdownTakeProfitRule{
		{
			MinProfitPct:   5,
			MaxDrawdownPct: 30,
			CloseRatioPct:  60,
		},
	}

	resolved := resolveDrawdownRulesWithModes(strategy, ai)
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved rule, got %d", len(resolved))
	}

	// close_ratio_mode=manual → uses strategy value 50
	if resolved[0].CloseRatioPct != 50 {
		t.Fatalf("expected close_ratio=50 (manual), got %.2f", resolved[0].CloseRatioPct)
	}
	// min_profit_mode=ai → uses AI value 5
	if resolved[0].MinProfitPct != 5 {
		t.Fatalf("expected min_profit=5 (AI), got %.2f", resolved[0].MinProfitPct)
	}
	// max_drawdown_mode=manual → uses strategy value 40
	if resolved[0].MaxDrawdownPct != 40 {
		t.Fatalf("expected max_drawdown=40 (manual), got %.2f", resolved[0].MaxDrawdownPct)
	}
}

func TestHasAllTiersCompleted(t *testing.T) {
	allocs := []store.DrawdownTierAllocation{
		{Status: "executed"},
		{Status: "be_covered"},
	}
	if !hasAllTiersCompleted(allocs) {
		t.Fatal("expected all completed")
	}

	allocs[1].Status = "tracking"
	if hasAllTiersCompleted(allocs) {
		t.Fatal("expected not all completed with tracking tier")
	}
}

func TestGetPendingTierCount(t *testing.T) {
	allocs := []store.DrawdownTierAllocation{
		{Status: "executed"},
		{Status: "tracking"},
		{Status: "pending"},
	}
	if got := getPendingTierCount(allocs); got != 2 {
		t.Fatalf("expected 2 pending, got %d", got)
	}
}
