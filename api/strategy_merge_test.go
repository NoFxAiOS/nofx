package api

import (
	"encoding/json"
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
		Rules: []store.LadderTPSLRule{{TakeProfitPct: 2.5, TakeProfitCloseRatioPct: 40, StopLossPct: 1.5, StopLossCloseRatioPct: 60}},
	}

	blob, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing config failed: %v", err)
	}

	var merged store.StrategyConfig
	if err := json.Unmarshal(blob, &merged); err != nil {
		t.Fatalf("unmarshal existing config failed: %v", err)
	}

	partialUpdate := []byte(`{"grid_enabled":true}`)
	if err := json.Unmarshal(partialUpdate, &merged); err != nil {
		t.Fatalf("unmarshal partial update failed: %v", err)
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

	blob, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing config failed: %v", err)
	}

	var merged store.StrategyConfig
	if err := json.Unmarshal(blob, &merged); err != nil {
		t.Fatalf("unmarshal existing config failed: %v", err)
	}

	// Current implementation relies on json.Unmarshal into an existing struct.
	// Nested objects update provided fields while preserving omitted sibling fields.
	nestedPartial := []byte(`{"protection":{"full_tp_sl":{"enabled":true}}}`)
	if err := json.Unmarshal(nestedPartial, &merged); err != nil {
		t.Fatalf("unmarshal nested partial failed: %v", err)
	}

	if merged.Protection.FullTPSL.TakeProfit.Mode != store.ProtectionValueModeAI || merged.Protection.FullTPSL.FallbackMaxLoss.Mode != store.ProtectionValueModeManual {
		t.Fatalf("expected nested partial update to preserve unmentioned full_tp_sl fields, got %+v", merged.Protection.FullTPSL)
	}
}
