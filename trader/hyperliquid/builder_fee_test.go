package hyperliquid

import "testing"

func TestDefaultBuilderIsHardcodedToApprovedFeeTier(t *testing.T) {
	if defaultBuilder == nil {
		t.Fatal("defaultBuilder is nil")
	}
	if got := defaultBuilder.Builder; got != "0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d" {
		t.Fatalf("defaultBuilder.Builder = %s, want hardcoded NOFX builder", got)
	}
	// Fee is in tenths of a basis point: 50 = 5 bps = 0.05% (万5).
	// Must match defaultHyperliquidBuilderMaxFee on the API side and the
	// frontend HYPERLIQUID_BUILDER_MAX_FEE constant the user signs against.
	if got := defaultBuilder.Fee; got != 50 {
		t.Fatalf("defaultBuilder.Fee = %d, want hardcoded 50 for 0.05%%", got)
	}
}
