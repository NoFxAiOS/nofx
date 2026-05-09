package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestValidateAIDecisionsWithStrategyRequiresHigherTimeframeAnchorWhenHigherContextProvided(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.EntryStructure = store.EntryStructureConfig{
		Enabled:                   true,
		RequirePrimaryTimeframe:   true,
		RequireAdjacentTimeframes: true,
		RequireSupportResistance:  true,
		RequireStructuralAnchors:  true,
	}
	entry := validEntryProtectionForTest("open_long")
	entry.Anchors = []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 95, Reason: "primary invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 110, Reason: "primary target"}}
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        1,
		PositionSizeUSD: 100,
		Reasoning:       "valid except missing higher anchor",
		EntryProtection: entry,
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "higher timeframe anchor") {
		t.Fatalf("expected higher timeframe anchor validation error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyAcceptsHigherTimeframeAnchor(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.EntryStructure = store.EntryStructureConfig{
		Enabled:                   true,
		RequirePrimaryTimeframe:   true,
		RequireAdjacentTimeframes: true,
		RequireSupportResistance:  true,
		RequireStructuralAnchors:  true,
	}
	entry := validEntryProtectionForTest("open_long")
	entry.HigherAnchors = []AIEntryProtectionAnchor{{Type: "resistance", Timeframe: "1h", Price: 116, Reason: "higher timeframe runner objective"}}
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        1,
		PositionSizeUSD: 100,
		Reasoning:       "valid higher anchor",
		EntryProtection: entry,
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected higher timeframe anchor to pass, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRequiresHigherFibonacciWhenFibonacciRequired(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.EntryStructure = store.EntryStructureConfig{
		Enabled:                   true,
		RequirePrimaryTimeframe:   true,
		RequireAdjacentTimeframes: true,
		RequireSupportResistance:  true,
		RequireStructuralAnchors:  true,
		RequireFibonacci:          true,
	}
	entry := validTighterLongEntryProtectionForTest()
	entry.HigherAnchors = []AIEntryProtectionAnchor{{Type: "resistance", Timeframe: "1h", Price: 112, Reason: "higher runner target"}}
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        1,
		PositionSizeUSD: 100,
		Reasoning:       "missing higher fib",
		EntryProtection: entry,
	}}

	err := ValidateAIDecisionsWithStrategy(decisions, cfg)
	if err == nil || !strings.Contains(err.Error(), "higher timeframe fibonacci") {
		t.Fatalf("expected higher timeframe fibonacci validation error, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyAcceptsHigherTimeframeStructureFibonacci(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.EntryStructure = store.EntryStructureConfig{
		Enabled:                   true,
		RequirePrimaryTimeframe:   true,
		RequireAdjacentTimeframes: true,
		RequireSupportResistance:  true,
		RequireStructuralAnchors:  true,
		RequireFibonacci:          true,
	}
	entry := validTighterLongEntryProtectionForTest()
	entry.TimeframeStructures = []AIEntryTimeframeStructure{{
		Timeframe:  "1h",
		Role:       "runner",
		Resistance: []float64{112},
		Fibonacci:  &AIEntryFibonacci{SwingHigh: 112, SwingLow: 96, Levels: []float64{105.888, 102.112}},
		Anchors:    []AIEntryProtectionAnchor{{Type: "resistance", Timeframe: "1h", Price: 112, Reason: "higher runner target"}},
		UsedFor:    "outer_drawdown_runner",
	}}
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        1,
		PositionSizeUSD: 100,
		Reasoning:       "valid higher tf fib",
		EntryProtection: entry,
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected higher timeframe structure fibonacci to pass, got %v", err)
	}
}
