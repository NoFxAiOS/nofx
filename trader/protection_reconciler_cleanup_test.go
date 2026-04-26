package trader

import "testing"

type taggedCleanupTrader struct {
	fakeReconcileTrader
	taggedSLCalls []string
	taggedTPCalls []string
}

func (t *taggedCleanupTrader) CancelStopLossOrdersTagged(symbol string, reasonTag string) error {
	t.taggedSLCalls = append(t.taggedSLCalls, symbol+":"+reasonTag)
	return nil
}

func (t *taggedCleanupTrader) CancelTakeProfitOrdersTagged(symbol string, reasonTag string) error {
	t.taggedTPCalls = append(t.taggedTPCalls, symbol+":"+reasonTag)
	return nil
}

func TestCancelProtectionOrdersForCleanupBroadCleansAfterTaggedCleanup(t *testing.T) {
	ft := &taggedCleanupTrader{}
	at := &AutoTrader{trader: ft}

	at.cancelProtectionOrdersForCleanup("SOLUSDT")

	if len(ft.taggedSLCalls) == 0 || len(ft.taggedTPCalls) == 0 {
		t.Fatalf("expected tagged cleanup calls, got sl=%v tp=%v", ft.taggedSLCalls, ft.taggedTPCalls)
	}
	if len(ft.cancelStopLossCalls) != 1 || ft.cancelStopLossCalls[0] != "SOLUSDT" {
		t.Fatalf("expected broad stop-loss cleanup after tagged cleanup, got %+v", ft.cancelStopLossCalls)
	}
	if len(ft.cancelTakeProfitCalls) != 1 || ft.cancelTakeProfitCalls[0] != "SOLUSDT" {
		t.Fatalf("expected broad take-profit cleanup after tagged cleanup, got %+v", ft.cancelTakeProfitCalls)
	}
}
