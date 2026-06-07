package api

import "testing"

func TestIsSupportedTraderSymbol(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
		want   bool
	}{
		{name: "legacy USDT perp", symbol: "BTCUSDT", want: true},
		{name: "legacy USDT perp lowercase", symbol: "ethusdt", want: true},
		{name: "Hyperliquid xyz stock USDC pair", symbol: "SMSN-USDC", want: true},
		{name: "Hyperliquid xyz commodity USDC pair", symbol: "GOLD-USDC", want: true},
		{name: "legacy internal xyz prefix still accepted", symbol: "xyz:SMSN", want: true},
		{name: "empty slot ignored", symbol: "  ", want: true},
		{name: "bare stock without xyz prefix rejected", symbol: "SMSN", want: false},
		{name: "unknown non-USDT pair rejected", symbol: "BTCUSD", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSupportedTraderSymbol(tt.symbol); got != tt.want {
				t.Fatalf("isSupportedTraderSymbol(%q) = %v, want %v", tt.symbol, got, tt.want)
			}
		})
	}
}
