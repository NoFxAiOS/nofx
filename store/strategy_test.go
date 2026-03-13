package store

import (
	"testing"
)

func TestGetDefaultStrategyConfig_HasMacroMicroFields(t *testing.T) {
	config := GetDefaultStrategyConfig("en")

	if config.StrategyType != "ai_trading" && config.StrategyType != "" {
		t.Errorf("Default config should be ai_trading or empty, got %q", config.StrategyType)
	}
	if UsesMultiTurnFlow(&config) {
		t.Error("Default config should not use multi-turn flow")
	}

	if config.MacroDeepDiveLimit != 5 {
		t.Errorf("Expected MacroDeepDiveLimit=5, got %d", config.MacroDeepDiveLimit)
	}

	if config.PositionCheckExtraPrompt == "" {
		t.Error("Expected non-empty PositionCheckExtraPrompt")
	}

	if config.SizingAdjustmentExtraPrompt == "" {
		t.Error("Expected non-empty SizingAdjustmentExtraPrompt")
	}
}

func TestUsesMultiTurnFlow(t *testing.T) {
	tests := []struct {
		name   string
		config *StrategyConfig
		want   bool
	}{
		{"nil config", nil, false},
		{"ai_trading", &StrategyConfig{StrategyType: "ai_trading"}, false},
		{"multi_turn_ai_trading", &StrategyConfig{StrategyType: "multi_turn_ai_trading"}, true},
		{"grid_trading", &StrategyConfig{StrategyType: "grid_trading"}, false},
		{"legacy EnableMacroMicroFlow", &StrategyConfig{StrategyType: "ai_trading", EnableMacroMicroFlow: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UsesMultiTurnFlow(tt.config); got != tt.want {
				t.Errorf("UsesMultiTurnFlow() = %v, want %v", got, tt.want)
			}
		})
	}
}
