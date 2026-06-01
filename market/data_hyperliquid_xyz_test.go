package market

import "testing"

func TestHyperliquidXYZAliasesNormalizeForAIDecisionData(t *testing.T) {
	tests := []struct {
		input      string
		normalized string
		isXyzAsset bool
	}{
		{input: "SMSN-USDC", normalized: "xyz:SMSN", isXyzAsset: true},
		{input: "SAMSUNG-USDC", normalized: "xyz:SMSN", isXyzAsset: true},
		{input: "xyz:SMSN", normalized: "xyz:SMSN", isXyzAsset: true},
		{input: "TESLA-USDC", normalized: "xyz:TSLA", isXyzAsset: true},
		{input: "TSLA-USDC", normalized: "xyz:TSLA", isXyzAsset: true},
	}

	for _, tt := range tests {
		if got := Normalize(tt.input); got != tt.normalized {
			t.Fatalf("Normalize(%q) = %q, want %q", tt.input, got, tt.normalized)
		}
		if got := IsXyzDexAsset(tt.normalized); got != tt.isXyzAsset {
			t.Fatalf("IsXyzDexAsset(%q) = %v, want %v", tt.normalized, got, tt.isXyzAsset)
		}
	}
}
