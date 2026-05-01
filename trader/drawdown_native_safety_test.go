package trader

import (
	"math"
	"strings"
	"testing"

	"nofx/store"
)

func TestAdjustNativeDrawdownCallbackRejectsNoiseCallbackInsteadOfWidening(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 0.9, MaxDrawdownPct: 1.5, CloseRatioPct: 100}
	callback := calculateProfitBasedTrailingCallbackRatio(0.1529, "short", rule.MinProfitPct, rule.MaxDrawdownPct)
	if callback >= minNativeDrawdownCallbackRatio {
		t.Fatalf("test setup expected noisy callback below safety floor, got %.8f", callback)
	}
	adjusted, err := adjustNativeDrawdownCallbackRatio(0.1529, "short", rule, callback)
	if err == nil {
		t.Fatal("expected native callback rejection so managed fallback can preserve rule math")
	}
	if adjusted.Adjusted || adjusted.CallbackRatio != callback {
		t.Fatalf("expected callback unchanged on rejection, got %+v", adjusted)
	}
	if !strings.Contains(err.Error(), "managed drawdown fallback") {
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

func TestDrawdownRuleCallbackRatioSupportsAbsoluteProfitDrawdown(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 1.15, MaxDrawdownPct: 55, MaxDrawdownAbsPct: 0.55, CloseRatioPct: 50}
	callback := calculateDrawdownRuleCallbackRatio(100, "long", rule)
	// Absolute profit drawdown of 0.55% from entry at activation 101.15 gives 0.55 / 101.15.
	expected := 0.55 / 101.15
	if math.Abs(callback-expected) > 1e-8 {
		t.Fatalf("expected absolute drawdown callback %.8f, got %.8f", expected, callback)
	}
}

func TestDrawdownThresholdSupportsAbsoluteProfitDrawdown(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 1.15, MaxDrawdownPct: 55, MaxDrawdownAbsPct: 0.55, CloseRatioPct: 50}
	if !isDrawdownThresholdMet(0.59, 0, rule) {
		t.Fatal("expected absolute profit drawdown threshold to trigger after profit falls below min-profit minus abs drawdown")
	}
	if isDrawdownThresholdMet(0.80, 0, rule) {
		t.Fatal("did not expect absolute profit drawdown threshold before enough profit giveback")
	}
}
