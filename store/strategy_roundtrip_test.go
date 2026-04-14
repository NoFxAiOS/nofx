package store

import (
	"encoding/json"
	"testing"
)

func TestStrategyConfigProtectionRoundTripPreservesNewProtectionFields(t *testing.T) {
	cfg := GetDefaultStrategyConfig("zh")
	cfg.Protection.FullTPSL = FullTPSLConfig{
		Enabled:         true,
		Mode:            ProtectionModeAI,
		TakeProfit:      ProtectionValueSource{Mode: ProtectionValueModeAI, Value: 8},
		StopLoss:        ProtectionValueSource{Mode: ProtectionValueModeManual, Value: 4},
		FallbackMaxLoss: ProtectionValueSource{Mode: ProtectionValueModeManual, Value: 12},
	}
	cfg.Protection.LadderTPSL = LadderTPSLConfig{
		Enabled:           true,
		Mode:              ProtectionModeManual,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   ProtectionValueSource{Mode: ProtectionValueModeAI, Value: 0},
		TakeProfitSize:    ProtectionValueSource{Mode: ProtectionValueModeManual, Value: 1},
		StopLossPrice:     ProtectionValueSource{Mode: ProtectionValueModeManual, Value: 1},
		StopLossSize:      ProtectionValueSource{Mode: ProtectionValueModeAI, Value: 0},
		FallbackMaxLoss:   ProtectionValueSource{Mode: ProtectionValueModeManual, Value: 15},
		Rules: []LadderTPSLRule{{
			TakeProfitPct:           2.5,
			TakeProfitCloseRatioPct: 40,
			StopLossPct:             1.5,
			StopLossCloseRatioPct:   60,
		}},
	}

	blob, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got StrategyConfig
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.Protection.FullTPSL.FallbackMaxLoss.Mode != ProtectionValueModeManual || got.Protection.FullTPSL.FallbackMaxLoss.Value != 12 {
		t.Fatalf("full fallback max loss lost after round-trip: %+v", got.Protection.FullTPSL.FallbackMaxLoss)
	}
	if got.Protection.LadderTPSL.TakeProfitPrice.Mode != ProtectionValueModeAI || got.Protection.LadderTPSL.StopLossSize.Mode != ProtectionValueModeAI {
		t.Fatalf("ladder mixed modes lost after round-trip: %+v", got.Protection.LadderTPSL)
	}
	if len(got.Protection.LadderTPSL.Rules) != 1 {
		t.Fatalf("expected ladder rules preserved, got %+v", got.Protection.LadderTPSL.Rules)
	}
}

func TestDefaultStrategyConfigIncludesProtectionValueSources(t *testing.T) {
	cfg := GetDefaultStrategyConfig("en")

	if cfg.Protection.FullTPSL.TakeProfit.Mode == "" || cfg.Protection.FullTPSL.StopLoss.Mode == "" {
		t.Fatalf("default full tp/sl value source modes should be initialized: %+v", cfg.Protection.FullTPSL)
	}
	if cfg.Protection.LadderTPSL.TakeProfitPrice.Mode == "" || cfg.Protection.LadderTPSL.StopLossSize.Mode == "" {
		t.Fatalf("default ladder value source modes should be initialized: %+v", cfg.Protection.LadderTPSL)
	}
}
