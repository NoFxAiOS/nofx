package agent

import "testing"

func TestIsStockSymbol(t *testing.T) {
	tests := []struct {
		sym  string
		want bool
	}{
		// Known crypto base symbols — must NOT be detected as stock
		{"BTC", false},
		{"ETH", false},
		{"SOL", false},
		{"BNB", false},
		{"XRP", false},
		{"DOGE", false},
		{"ADA", false},
		{"AVAX", false},
		{"DOT", false},
		{"LINK", false},
		{"PEPE", false},
		{"SHIB", false},
		{"TRUMP", false},
		{"USDT", false},
		{"USDC", false},
		{"W", false}, // single letter crypto

		// Crypto pairs — must NOT be stock
		{"BTCUSDT", false},
		{"ETHUSDT", false},
		{"SOLUSDT", false},
		{"DOGEUSDT", false},

		// Real stock tickers — must be detected as stock
		{"AAPL", true},
		{"TSLA", true},
		{"NVDA", true},
		{"MSFT", true},
		{"GOOGL", true},
		{"AMZN", true},
		{"META", true},
		{"AMD", true},
		{"PLTR", true},
		{"BA", true},
		{"F", true},   // Ford — 1 letter
		{"GM", true},  // 2 letters
		{"JPM", true}, // 3 letters

		// Mixed / edge cases
		{"btc", false},  // lowercase crypto
		{"aapl", true},  // lowercase stock (uppercased internally)
		{"BTC123", false}, // not pure letters
		{"123456", false}, // digits
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.sym, func(t *testing.T) {
			got := isStockSymbol(tt.sym)
			if got != tt.want {
				t.Errorf("isStockSymbol(%q) = %v, want %v", tt.sym, got, tt.want)
			}
		})
	}
}
