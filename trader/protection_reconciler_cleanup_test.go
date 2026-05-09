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

func TestCancelProtectionOrdersForCleanupUsesTaggedCleanupOnly(t *testing.T) {
	ft := &taggedCleanupTrader{}
	at := &AutoTrader{trader: ft}

	at.cancelProtectionOrdersForCleanup("SOLUSDT")

	if len(ft.taggedSLCalls) == 0 || len(ft.taggedTPCalls) == 0 {
		t.Fatalf("expected tagged cleanup calls, got sl=%v tp=%v", ft.taggedSLCalls, ft.taggedTPCalls)
	}
	if len(ft.cancelStopLossCalls) != 0 {
		t.Fatalf("expected no broad stop-loss cleanup for active cleanup helper, got %+v", ft.cancelStopLossCalls)
	}
	if len(ft.cancelTakeProfitCalls) != 0 {
		t.Fatalf("expected no broad take-profit cleanup for active cleanup helper, got %+v", ft.cancelTakeProfitCalls)
	}
}
