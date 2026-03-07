package store

import (
	"testing"
)

func TestGetDefaultStrategyConfig_HasMacroMicroFields(t *testing.T) {
	config := GetDefaultStrategyConfig("en")

	if config.EnableMacroMicroFlow != false {
		t.Errorf("Expected EnableMacroMicroFlow=false, got %v", config.EnableMacroMicroFlow)
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
