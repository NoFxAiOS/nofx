package hyperliquid

import "testing"

func TestConvertSymbolToHyperliquidXYZAliases(t *testing.T) {
	cases := map[string]string{
		"SAMSUNG-USDC":  "xyz:SMSN",
		"SK-HYNIX-USDC": "xyz:SKHX",
		"TSLAUSDT":      "xyz:TSLA",
		"xyz:SMSN":      "xyz:SMSN",
		"HYPEUSDT":      "HYPE",
	}
	for input, want := range cases {
		if got := convertSymbolToHyperliquid(input); got != want {
			t.Fatalf("convertSymbolToHyperliquid(%q) = %q, want %q", input, got, want)
		}
	}
}
