package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLiquiditySweepConfigDefaultsAndRoundTrip(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		if GetDefaultStrategyConfig(lang).Indicators.EnableLiquiditySweep {
			t.Fatalf("%s default must leave experimental sweep disabled", lang)
		}
	}
	for _, raw := range []string{`{}`, `{"indicators":{"enable_ema":true}}`, `{"indicators":{"enable_liquidity_sweep":false}}`} {
		var cfg StrategyConfig
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.Indicators.EnableLiquiditySweep {
			t.Fatalf("legacy/disabled config enabled sweep: %s", raw)
		}
		encoded, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "enable_liquidity_sweep") {
			t.Fatal("disabled flag changed legacy config JSON")
		}
	}
	var cfg StrategyConfig
	if err := json.Unmarshal([]byte(`{"indicators":{"enable_liquidity_sweep":true}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.Indicators.EnableLiquiditySweep {
		t.Fatal("explicit opt-in ignored")
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var restored StrategyConfig
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.Indicators.EnableLiquiditySweep {
		t.Fatal("opt-in lost in config round trip")
	}
}
