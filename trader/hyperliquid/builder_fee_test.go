package hyperliquid

import "testing"

func TestDefaultBuilderIsHardcodedToApprovedFeeTier(t *testing.T) {
	if defaultBuilder == nil {
		t.Fatal("defaultBuilder is nil")
	}
	if got := defaultBuilder.Builder; got != "0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d" {
		t.Fatalf("defaultBuilder.Builder = %s, want hardcoded NOFX builder", got)
	}
	if got := defaultBuilder.Fee; got != 100 {
		t.Fatalf("defaultBuilder.Fee = %d, want hardcoded 100 for 0.1%%", got)
	}
}
