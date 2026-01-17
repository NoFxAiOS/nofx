package store

import (
	"encoding/json"
	"testing"
)

func TestTrailingStopWithDefaultsLegacyOmitted(t *testing.T) {
	cfg := TrailingStopConfig{}

	res := cfg.WithDefaults()

	if !res.Enabled {
		t.Fatalf("expected legacy omitted config to be enabled by default")
	}
	if res.Mode != "pnl_pct" {
		t.Fatalf("expected default mode, got %s", res.Mode)
	}
	if res.CheckIntervalSec != 30 {
		t.Fatalf("expected default interval 30s, got %d", res.CheckIntervalSec)
	}
	if res.ClosePct != 1.0 {
		t.Fatalf("expected default close pct 1.0, got %f", res.ClosePct)
	}
}

func TestTrailingStopWithDefaultsExplicitDisable(t *testing.T) {
	cfg := TrailingStopConfig{
		Enabled:    false,
		Provided:   true,
		EnabledSet: true,
	}

	res := cfg.WithDefaults()

	if res.Enabled {
		t.Fatalf("expected explicit disable to stay disabled")
	}
	if res.Mode == "" {
		t.Fatalf("expected mode to be backfilled even when disabled")
	}
	if res.CheckIntervalMs == 0 {
		t.Fatalf("expected interval to be backfilled when disabled")
	}
}

func TestTrailingStopWithDefaultsEnabledNormalization(t *testing.T) {
	cfg := TrailingStopConfig{
		Enabled:          true,
		EnabledSet:       true,
		Provided:         true,
		Mode:             "price_pct",
		TrailPct:         1.5,
		CheckIntervalMs:  1200,
		ActivationPct:    0.2,
		ClosePct:         0, // should backfill to default
		TightenBands:     nil,
		CheckIntervalSec: 0,
	}

	res := cfg.WithDefaults()

	if !res.Enabled {
		t.Fatalf("expected enabled config to stay enabled")
	}
	if res.Mode != "price_pct" || res.TrailPct != 1.5 {
		t.Fatalf("expected custom mode and trail pct to be preserved")
	}
	if res.CheckIntervalSec == 0 || res.CheckIntervalMs != 1200 {
		t.Fatalf("expected interval backfilled in both units, got sec=%d ms=%d", res.CheckIntervalSec, res.CheckIntervalMs)
	}
	if res.ClosePct != 1.0 {
		t.Fatalf("expected close pct to backfill to default 1.0, got %f", res.ClosePct)
	}
	if res.TightenBands == nil {
		t.Fatalf("expected tighten bands slice initialized")
	}
}

func TestTrailingStopUnmarshalPresence(t *testing.T) {
	jsonData := []byte(`{"risk_control":{}}`)
	var cfg StrategyConfig
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		t.Fatalf("failed to unmarshal strategy: %v", err)
	}
	if cfg.RiskControl.TrailingStop.Provided {
		t.Fatalf("expected trailing stop to be marked not provided when field is absent")
	}
}

func TestTrailingStopUnmarshalEnabledMarker(t *testing.T) {
	jsonData := []byte(`{"risk_control":{"trailing_stop":{"enabled":false}}}`)
	var cfg StrategyConfig
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		t.Fatalf("failed to unmarshal strategy: %v", err)
	}
	if !cfg.RiskControl.TrailingStop.Provided {
		t.Fatalf("expected trailing stop to be marked provided when field exists")
	}
	if !cfg.RiskControl.TrailingStop.EnabledSet {
		t.Fatalf("expected enabled flag presence to be tracked")
	}
	if cfg.RiskControl.TrailingStop.Enabled {
		t.Fatalf("expected enabled to be false from payload")
	}
}
