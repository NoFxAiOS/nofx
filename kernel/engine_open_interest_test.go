package kernel

import (
	"strings"
	"testing"

	"nofx/market"
	"nofx/store"
)

func TestOpenInterestLiquidityFilter(t *testing.T) {
	for _, tt := range []struct {
		name     string
		symbol   string
		oi       *market.OIData
		price    float64
		position bool
		skip     bool
	}{
		{"missing data keeps BTC", "BTCUSDT", nil, 50000, false, false},
		{"missing data keeps ETH", "ETHUSDT", nil, 2000, false, false},
		{"real zero is filtered", "BTCUSDT", &market.OIData{Latest: 0}, 50000, false, true},
		{"low liquidity is filtered", "BTCUSDT", &market.OIData{Latest: 299}, 50000, false, true},
		{"threshold is included", "BTCUSDT", &market.OIData{Latest: 300}, 50000, false, false},
		{"healthy liquidity is included", "BTCUSDT", &market.OIData{Latest: 1000}, 50000, false, false},
		{"position is retained", "BTCUSDT", &market.OIData{Latest: 1}, 50000, true, false},
		{"xyz exemption is retained", "xyz:TSLA", &market.OIData{Latest: 0}, 200, false, false},
		{"missing price cannot determine OI value", "BTCUSDT", &market.OIData{Latest: 1000}, 0, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := &market.Data{Symbol: tt.symbol, CurrentPrice: tt.price, OpenInterest: tt.oi}
			if got := skipLowOpenInterest(tt.symbol, data, tt.position); got != tt.skip {
				t.Fatalf("skip = %v, want %v", got, tt.skip)
			}
		})
	}
}

func TestMarketPromptDistinguishesMissingAndZeroOpenInterest(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.Indicators.EnableOI = true
	engine := NewStrategyEngine(&cfg)
	data := &market.Data{Symbol: "BTCUSDT", CurrentPrice: 50000}
	missing := engine.formatMarketData(data)
	if !strings.Contains(missing, "Open Interest: unavailable") || strings.Contains(missing, "Latest: 0.00") {
		t.Fatalf("unknown OI must not be presented as a real zero: %s", missing)
	}
	data.OpenInterest = &market.OIData{Latest: 0}
	zero := engine.formatMarketData(data)
	if !strings.Contains(zero, "Open Interest: Latest: 0.00") || strings.Contains(zero, "unavailable") {
		t.Fatalf("measured zero must remain distinguishable: %s", zero)
	}
	cfg.Indicators.EnableOI = false
	if prompt := engine.formatMarketData(data); strings.Contains(prompt, "Open Interest:") {
		t.Fatalf("disabled OI should stay out of the prompt: %s", prompt)
	}
}
