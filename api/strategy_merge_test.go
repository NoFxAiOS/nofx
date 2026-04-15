package api

import (
	"testing"

	"nofx/store"
)

func TestStrategyConfigMergePreservesNewProtectionFieldsOnPartialTopLevelUpdate(t *testing.T) {
	existing := store.GetDefaultStrategyConfig("zh")
	existing.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled:         true,
		Mode:            store.ProtectionModeAI,
		TakeProfit:      store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: 8},
		StopLoss:        store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 4},
		FallbackMaxLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 12},
	}
	existing.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeManual,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		FallbackMaxLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 15},
		Rules:             []store.LadderTPSLRule{{TakeProfitPct: 2.5, TakeProfitCloseRatioPct: 40, StopLossPct: 1.5, StopLossCloseRatioPct: 60}},
	}

	partialUpdate := []byte(`{"grid_enabled":true}`)
	merged, err := mergeStrategyConfig(existing, partialUpdate)
	if err != nil {
		t.Fatalf("merge partial update failed: %v", err)
	}

	if merged.Protection.FullTPSL.FallbackMaxLoss.Value != 12 {
		t.Fatalf("expected full fallback max loss to survive top-level partial update, got %+v", merged.Protection.FullTPSL.FallbackMaxLoss)
	}
	if merged.Protection.LadderTPSL.FallbackMaxLoss.Value != 15 {
		t.Fatalf("expected ladder fallback max loss to survive top-level partial update, got %+v", merged.Protection.LadderTPSL.FallbackMaxLoss)
	}
}

func TestStrategyConfigMergeNestedObjectPreservesUnmentionedProtectionFields(t *testing.T) {
	existing := store.GetDefaultStrategyConfig("zh")
	existing.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled:         true,
		Mode:            store.ProtectionModeAI,
		TakeProfit:      store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: 8},
		StopLoss:        store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 4},
		FallbackMaxLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 12},
	}

	nestedPartial := []byte(`{"protection":{"full_tp_sl":{"enabled":true}}}`)
	merged, err := mergeStrategyConfig(existing, nestedPartial)
	if err != nil {
		t.Fatalf("merge nested partial failed: %v", err)
	}

	if merged.Protection.FullTPSL.TakeProfit.Mode != store.ProtectionValueModeAI || merged.Protection.FullTPSL.FallbackMaxLoss.Mode != store.ProtectionValueModeManual {
		t.Fatalf("expected nested partial update to preserve unmentioned full_tp_sl fields, got %+v", merged.Protection.FullTPSL)
	}
}

func TestStrategyConfigMergePreservesLadderAIModeAndFallbackWhenOnlyOneFieldChanges(t *testing.T) {
	existing := store.GetDefaultStrategyConfig("zh")
	existing.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeAI,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 2},
		StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 50},
		FallbackMaxLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 9},
		Rules:             []store.LadderTPSLRule{{TakeProfitPct: 3, TakeProfitCloseRatioPct: 50, StopLossPct: 2, StopLossCloseRatioPct: 50}},
	}

	partial := []byte(`{"protection":{"ladder_tp_sl":{"take_profit_enabled":false}}}`)
	merged, err := mergeStrategyConfig(existing, partial)
	if err != nil {
		t.Fatalf("merge ladder partial failed: %v", err)
	}

	if merged.Protection.LadderTPSL.Mode != store.ProtectionModeAI {
		t.Fatalf("expected ladder mode ai to survive partial update, got %+v", merged.Protection.LadderTPSL)
	}
	if merged.Protection.LadderTPSL.TakeProfitPrice.Mode != store.ProtectionValueModeAI || merged.Protection.LadderTPSL.TakeProfitSize.Mode != store.ProtectionValueModeAI {
		t.Fatalf("expected ladder ai value modes to survive partial update, got %+v", merged.Protection.LadderTPSL)
	}
	if merged.Protection.LadderTPSL.FallbackMaxLoss.Value != 9 {
		t.Fatalf("expected ladder fallback max loss to survive partial update, got %+v", merged.Protection.LadderTPSL)
	}
}

func TestStrategyConfigMergePreservesFullAIModeWhenUpdatingFallbackOnly(t *testing.T) {
	existing := store.GetDefaultStrategyConfig("zh")
	existing.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled:         true,
		Mode:            store.ProtectionModeAI,
		TakeProfit:      store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		StopLoss:        store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		FallbackMaxLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeDisabled, Value: 0},
	}

	partial := []byte(`{"protection":{"full_tp_sl":{"fallback_max_loss":{"mode":"manual","value":6}}}}`)
	merged, err := mergeStrategyConfig(existing, partial)
	if err != nil {
		t.Fatalf("merge full partial failed: %v", err)
	}

	if merged.Protection.FullTPSL.Mode != store.ProtectionModeAI {
		t.Fatalf("expected full mode ai to survive fallback-only update, got %+v", merged.Protection.FullTPSL)
	}
	if merged.Protection.FullTPSL.TakeProfit.Mode != store.ProtectionValueModeAI || merged.Protection.FullTPSL.StopLoss.Mode != store.ProtectionValueModeAI {
		t.Fatalf("expected full ai value modes to survive fallback-only update, got %+v", merged.Protection.FullTPSL)
	}
	if merged.Protection.FullTPSL.FallbackMaxLoss.Mode != store.ProtectionValueModeManual || merged.Protection.FullTPSL.FallbackMaxLoss.Value != 6 {
		t.Fatalf("expected fallback update to apply cleanly, got %+v", merged.Protection.FullTPSL)
	}
}

func TestStrategyConfigMergePreservesFullFallbackWhenUpdatingLadderOnly(t *testing.T) {
	existing := store.GetDefaultStrategyConfig("zh")
	existing.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled:         true,
		Mode:            store.ProtectionModeAI,
		TakeProfit:      store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		StopLoss:        store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 2},
		FallbackMaxLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 7},
	}
	existing.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeManual,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 3},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 50},
		StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 2},
		StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 50},
		FallbackMaxLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeDisabled, Value: 0},
		Rules:             []store.LadderTPSLRule{{TakeProfitPct: 3, TakeProfitCloseRatioPct: 50, StopLossPct: 2, StopLossCloseRatioPct: 50}},
	}

	partial := []byte(`{"protection":{"ladder_tp_sl":{"mode":"ai","take_profit_price":{"mode":"ai"},"take_profit_size":{"mode":"ai"}}}}`)
	merged, err := mergeStrategyConfig(existing, partial)
	if err != nil {
		t.Fatalf("merge cross-section partial failed: %v", err)
	}

	if merged.Protection.FullTPSL.Mode != store.ProtectionModeAI || merged.Protection.FullTPSL.FallbackMaxLoss.Value != 7 {
		t.Fatalf("expected full config to survive ladder-only update, got %+v", merged.Protection.FullTPSL)
	}
	if merged.Protection.LadderTPSL.Mode != store.ProtectionModeAI || merged.Protection.LadderTPSL.TakeProfitPrice.Mode != store.ProtectionValueModeAI || merged.Protection.LadderTPSL.TakeProfitSize.Mode != store.ProtectionValueModeAI {
		t.Fatalf("expected ladder ai update to apply cleanly, got %+v", merged.Protection.LadderTPSL)
	}
}

func TestStrategyConfigMergePreservesLadderWhenUpdatingFullFallbackOnly(t *testing.T) {
	existing := store.GetDefaultStrategyConfig("zh")
	existing.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled:         true,
		Mode:            store.ProtectionModeAI,
		TakeProfit:      store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		StopLoss:        store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		FallbackMaxLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeDisabled, Value: 0},
	}
	existing.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeAI,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
		StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 2},
		StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 50},
		FallbackMaxLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 8},
		Rules:             []store.LadderTPSLRule{{TakeProfitPct: 3, TakeProfitCloseRatioPct: 50, StopLossPct: 2, StopLossCloseRatioPct: 50}},
	}

	partial := []byte(`{"protection":{"full_tp_sl":{"fallback_max_loss":{"mode":"manual","value":6}}}}`)
	merged, err := mergeStrategyConfig(existing, partial)
	if err != nil {
		t.Fatalf("merge full-fallback-only partial failed: %v", err)
	}

	if merged.Protection.FullTPSL.FallbackMaxLoss.Mode != store.ProtectionValueModeManual || merged.Protection.FullTPSL.FallbackMaxLoss.Value != 6 {
		t.Fatalf("expected full fallback update to apply, got %+v", merged.Protection.FullTPSL)
	}
	if merged.Protection.LadderTPSL.Mode != store.ProtectionModeAI || merged.Protection.LadderTPSL.FallbackMaxLoss.Value != 8 {
		t.Fatalf("expected ladder config to survive full-only update, got %+v", merged.Protection.LadderTPSL)
	}
}
