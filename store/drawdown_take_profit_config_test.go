package store

import (
	"encoding/json"
	"testing"
)

func TestDrawdownTakeProfitLegacyModeDefaults(t *testing.T) {
	var disabled DrawdownTakeProfitConfig
	if err := json.Unmarshal([]byte(`{"enabled":false,"rules":[]}`), &disabled); err != nil {
		t.Fatalf("unmarshal disabled legacy drawdown config failed: %v", err)
	}
	if disabled.Mode != ProtectionModeDisabled {
		t.Fatalf("expected disabled legacy drawdown config to default mode=disabled, got %q", disabled.Mode)
	}

	var enabled DrawdownTakeProfitConfig
	if err := json.Unmarshal([]byte(`{"enabled":true,"rules":[{"min_profit_pct":5,"max_drawdown_pct":40,"close_ratio_pct":100}]}`), &enabled); err != nil {
		t.Fatalf("unmarshal enabled legacy drawdown config failed: %v", err)
	}
	if enabled.Mode != ProtectionModeManual {
		t.Fatalf("expected enabled legacy drawdown config to default mode=manual, got %q", enabled.Mode)
	}
}

func TestDrawdownTakeProfitRoundTripPreservesAIMode(t *testing.T) {
	cfg := DrawdownTakeProfitConfig{
		Enabled: true,
		Mode:    ProtectionModeAI,
		Rules: []DrawdownTakeProfitRule{{
			MinProfitPct:        6,
			MaxDrawdownPct:      30,
			CloseRatioPct:       75,
			PollIntervalSeconds: 15,
		}},
	}

	blob, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal drawdown config failed: %v", err)
	}

	var got DrawdownTakeProfitConfig
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal drawdown config failed: %v", err)
	}

	if got.Mode != ProtectionModeAI {
		t.Fatalf("expected AI drawdown mode to round-trip, got %+v", got)
	}
	if len(got.Rules) != 1 || got.Rules[0].CloseRatioPct != 75 {
		t.Fatalf("expected drawdown rules to round-trip, got %+v", got.Rules)
	}
}
