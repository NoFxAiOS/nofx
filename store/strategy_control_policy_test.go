package store

import (
	"encoding/json"
	"testing"
)

func TestStrategyControlPolicyEffectiveModeDefaultsToStrict(t *testing.T) {
	var cfg StrategyControlPolicyConfig
	if got := cfg.EffectiveMode(); got != StrategyControlPolicyModeStrict {
		t.Fatalf("zero-value policy mode should default to strict, got %q", got)
	}

	cfg.Mode = StrategyControlPolicyMode("surprise")
	if got := cfg.EffectiveMode(); got != StrategyControlPolicyModeStrict {
		t.Fatalf("unknown policy mode should default to strict, got %q", got)
	}
}

func TestStrategyConfigStrategyControlPolicyRoundTripAndLegacyOmission(t *testing.T) {
	cfg := GetDefaultStrategyConfig("en")
	if got := cfg.StrategyControlPolicy.EffectiveMode(); got != StrategyControlPolicyModeStrict {
		t.Fatalf("default strategy control policy should be strict, got %q", got)
	}

	cfg.StrategyControlPolicy.Mode = StrategyControlPolicyModeAuditOnly
	blob, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got StrategyConfig
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.StrategyControlPolicy.Mode != StrategyControlPolicyModeAuditOnly {
		t.Fatalf("expected audit_only after round-trip, got %q", got.StrategyControlPolicy.Mode)
	}

	legacyJSON := `{"language":"en","coin_source":{"source_type":"static","use_ai500":false,"use_oi_top":false,"use_oi_low":false},"indicators":{"klines":{"primary_timeframe":"5m","primary_count":30,"enable_multi_timeframe":false},"enable_raw_klines":true},"risk_control":{"max_positions":1,"btc_eth_max_leverage":1,"altcoin_max_leverage":1,"max_margin_usage":0.9,"min_position_size":10,"min_risk_reward_ratio":2,"min_confidence":70},"protection":{"full_tp_sl":{"enabled":false},"ladder_tp_sl":{"enabled":false,"take_profit_enabled":false,"stop_loss_enabled":false},"drawdown_take_profit":{"enabled":false,"rules":[]},"break_even_stop":{"enabled":false},"regime_filter":{"enabled":false}}}`
	var legacy StrategyConfig
	if err := json.Unmarshal([]byte(legacyJSON), &legacy); err != nil {
		t.Fatalf("legacy unmarshal failed: %v", err)
	}
	if legacy.StrategyControlPolicy.Mode != "" {
		t.Fatalf("legacy payload should omit explicit mode, got %q", legacy.StrategyControlPolicy.Mode)
	}
	if legacy.StrategyControlPolicy.EffectiveMode() != StrategyControlPolicyModeStrict {
		t.Fatalf("legacy payload should behave as strict by default, got %q", legacy.StrategyControlPolicy.EffectiveMode())
	}
}
