package trader

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestAdjustNativeDrawdownCallbackRaisesNoiseCallbackWhenProfitSpaceAllows(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 0.9, MaxDrawdownPct: 1.5, CloseRatioPct: 100}
	callback := calculateProfitBasedTrailingCallbackRatio(0.1529, "short", rule.MinProfitPct, rule.MaxDrawdownPct)
	if callback >= minNativeDrawdownCallbackRatio {
		t.Fatalf("test setup expected noisy callback below safety floor, got %.8f", callback)
	}
	adjusted, err := adjustNativeDrawdownCallbackRatio(0.1529, "short", rule, callback)
	if err != nil {
		t.Fatalf("expected callback adjustment, got %v", err)
	}
	if !adjusted.Adjusted || adjusted.CallbackRatio != minNativeDrawdownCallbackRatio {
		t.Fatalf("expected callback raised to floor, got %+v", adjusted)
	}
}

func TestAdjustNativeDrawdownCallbackRejectsWhenProfitSpaceCannotSupportFloor(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 0.2, MaxDrawdownPct: 50, CloseRatioPct: 100}
	callback := calculateProfitBasedTrailingCallbackRatio(100, "long", rule.MinProfitPct, rule.MaxDrawdownPct)
	_, err := adjustNativeDrawdownCallbackRatio(100, "long", rule, callback)
	if err == nil {
		t.Fatal("expected insufficient profit space rejection")
	}
	if !strings.Contains(err.Error(), "cannot support safe callback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdjustNativeDrawdownCallbackKeepsStrategyDrawdown(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 0.8, MaxDrawdownPct: 70, CloseRatioPct: 50}
	callback := calculateProfitBasedTrailingCallbackRatio(0.1529, "short", rule.MinProfitPct, rule.MaxDrawdownPct)
	adjusted, err := adjustNativeDrawdownCallbackRatio(0.1529, "short", rule, callback)
	if err != nil {
		t.Fatalf("expected strategy drawdown rule to pass, got %v", err)
	}
	if adjusted.Adjusted || adjusted.CallbackRatio != callback {
		t.Fatalf("expected callback unchanged, got %+v", adjusted)
	}
}
