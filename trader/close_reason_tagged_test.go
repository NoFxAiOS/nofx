package trader

import "testing"

func TestClosePositionByReasonPrefersTaggedCloser(t *testing.T) {
	fake := &fakeProtectionTrader{}
	at := &AutoTrader{trader: fake}

	if err := at.closePositionByReason("ADAUSDT", "long", 12.5, "managed_drawdown"); err != nil {
		t.Fatalf("closePositionByReason returned error: %v", err)
	}
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected 1 close long call, got %d", fake.closeLongCalls)
	}
	if len(fake.taggedCloseLongs) != 1 || fake.taggedCloseLongs[0] != "managed_drawdown" {
		t.Fatalf("expected tagged close reason managed_drawdown, got %+v", fake.taggedCloseLongs)
	}
}

func TestClosePositionByReasonFallsBackWithoutTaggedReason(t *testing.T) {
	fake := &fakeProtectionTrader{}
	at := &AutoTrader{trader: fake}

	if err := at.closePositionByReason("ADAUSDT", "short", 3.0, ""); err != nil {
		t.Fatalf("closePositionByReason returned error: %v", err)
	}
	if fake.closeShortCalls != 1 {
		t.Fatalf("expected 1 close short call, got %d", fake.closeShortCalls)
	}
	if len(fake.taggedCloseShorts) != 0 {
		t.Fatalf("expected no tagged close short calls, got %+v", fake.taggedCloseShorts)
	}
}
