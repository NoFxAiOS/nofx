package trader

import (
	"testing"

	"nofx/kernel"
)

func TestCalculateImmediateTrailingCallbackLong(t *testing.T) {
	entry := 1.028
	slOrders := []ProtectionOrder{
		{Price: 1.02204, CloseRatioPct: 50},
	}
	callback, ok := calculateImmediateTrailingCallback(entry, "long", slOrders)
	if !ok {
		t.Fatal("expected ok=true")
	}
	expected := (entry - 1.02204) / entry
	if diff := callback - expected; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("callback=%.6f expected=%.6f", callback, expected)
	}
	if callback < minNativeDrawdownCallbackRatio {
		t.Fatalf("callback %.6f below safety floor %.6f", callback, minNativeDrawdownCallbackRatio)
	}
}

func TestCalculateImmediateTrailingCallbackShort(t *testing.T) {
	entry := 1.028
	slOrders := []ProtectionOrder{
		{Price: 1.034, CloseRatioPct: 50},
	}
	callback, ok := calculateImmediateTrailingCallback(entry, "short", slOrders)
	if !ok {
		t.Fatal("expected ok=true")
	}
	expected := (1.034 - entry) / entry
	if diff := callback - expected; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("callback=%.6f expected=%.6f", callback, expected)
	}
}

func TestCalculateImmediateTrailingCallbackBelowFloor(t *testing.T) {
	entry := 100.0
	slOrders := []ProtectionOrder{
		{Price: 99.98, CloseRatioPct: 50}, // 0.02% distance, below 0.3% floor
	}
	_, ok := calculateImmediateTrailingCallback(entry, "long", slOrders)
	if ok {
		t.Fatal("expected ok=false for callback below safety floor")
	}
}

func TestCalculateImmediateTrailingCallbackNoOrders(t *testing.T) {
	_, ok := calculateImmediateTrailingCallback(1.028, "long", nil)
	if ok {
		t.Fatal("expected ok=false for empty SL orders")
	}
}

func TestBuildAIDecisionLadderLastTierFullPosition(t *testing.T) {
	rules := []kernel.AIProtectionLadderRule{
		{StopLossPct: 0.58, StopLossCloseRatioPct: 50},
		{StopLossPct: 1.0, StopLossCloseRatioPct: 50},
		{StopLossPct: 1.5, StopLossCloseRatioPct: 50},
	}
	plan, err := buildAIDecisionLadderProtectionPlan(1.028, "open_long", rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.StopLossOrders) != 3 {
		t.Fatalf("expected 3 SL orders, got %d", len(plan.StopLossOrders))
	}
	if plan.StopLossOrders[0].CloseRatioPct != 50 {
		t.Fatalf("tier 1 expected 50%%, got %.1f%%", plan.StopLossOrders[0].CloseRatioPct)
	}
	if plan.StopLossOrders[1].CloseRatioPct != 50 {
		t.Fatalf("tier 2 expected 50%%, got %.1f%%", plan.StopLossOrders[1].CloseRatioPct)
	}
	// Last tier is always full-position close (100%) — the absolute backstop
	if plan.StopLossOrders[2].CloseRatioPct != 100 {
		t.Fatalf("last tier expected 100%% (full position), got %.1f%%", plan.StopLossOrders[2].CloseRatioPct)
	}
}
