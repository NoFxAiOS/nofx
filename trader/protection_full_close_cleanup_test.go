package trader

import (
	"sync"
	"testing"

	"nofx/store"
	tradertypes "nofx/trader/types"
)

func TestHandleSyncedFullCloseCleansStateAndBlocksLaterProtectionWrites(t *testing.T) {
	fake := &fakeReconcileTrader{
		positions: nil,
		fakeOrderProtectionTrader: fakeOrderProtectionTrader{
			openOrders: []tradertypes.OpenOrder{
				{OrderID: "be-stop", Symbol: "BTCUSDT", PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 78000, Quantity: 0.0002},
				{OrderID: "native-trailing", Symbol: "BTCUSDT", PositionSide: "SHORT", Type: "TRAILING_STOP_MARKET", StopPrice: 78100, CallbackRate: 0.02, Quantity: 0.0001},
			},
		},
	}
	at := &AutoTrader{
		exchange:              "okx",
		trader:                fake,
		config:                AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState:       map[string]string{"BTCUSDT_short": "native_partial_trailing_armed"},
		breakEvenState:        map[string]string{"BTCUSDT_short": "armed"},
		breakEvenFingerprints: map[string]string{"BTCUSDT_short": "78867.80000000|0.00020000"},
		drawdownRunnerState:   map[string]DrawdownRunnerState{"BTCUSDT_short": {StageName: "runner"}},
		drawdownState:         map[string]string{"BTCUSDT_short": "rule-fingerprint"},
		peakPnLCache:          map[string]float64{"BTCUSDT_short": 2.4},
		protectionStateMutex:  sync.RWMutex{},
		breakEvenStateMutex:   sync.RWMutex{},
		peakPnLCacheMutex:     sync.RWMutex{},
	}

	at.handleSyncedFullClose("BTCUSDT", "SHORT")

	if got := at.getProtectionState("BTCUSDT", "short"); got != "" {
		t.Fatalf("expected full-close cleanup to clear protection state, got %q", got)
	}
	if got := at.getBreakEvenState("BTCUSDT", "short"); got != "" {
		t.Fatalf("expected full-close cleanup to clear break-even state, got %q", got)
	}
	if state := at.getDrawdownRunnerState("BTCUSDT", "short"); state != nil {
		t.Fatalf("expected full-close cleanup to clear drawdown runner state, got %+v", state)
	}
	if _, ok := at.GetPeakPnLCache()["BTCUSDT_short"]; ok {
		t.Fatal("expected full-close cleanup to clear peak pnl cache")
	}
	if len(fake.cancelStopOrdersCalls) != 1 || fake.cancelStopOrdersCalls[0] != "BTCUSDT" {
		t.Fatalf("expected full-close cleanup to cancel orphan stop orders, got %v", fake.cancelStopOrdersCalls)
	}
	if len(fake.cancelTrailingCalls) != 1 || fake.cancelTrailingCalls[0] != "BTCUSDT" {
		t.Fatalf("expected full-close cleanup to cancel orphan trailing orders, got %v", fake.cancelTrailingCalls)
	}

	err := at.applyBreakEvenStop("BTCUSDT", "short", 0.0002, 78867.8, 1.2, store.BreakEvenStopConfig{
		Enabled:      true,
		TriggerMode:  store.BreakEvenTriggerProfitPct,
		TriggerValue: 0.7,
		OffsetPct:    0.3,
	})
	if err != nil {
		t.Fatalf("expected post-cleanup break-even guard to skip without error, got %v", err)
	}
	if len(fake.stopLossOrders) != 0 {
		t.Fatalf("expected no break-even write after full-close cleanup, got %d", len(fake.stopLossOrders))
	}
	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "short", 78867.8, store.DrawdownTakeProfitRule{
		MinProfitPct:   0.8,
		MaxDrawdownPct: 70,
		CloseRatioPct:  50,
	})
	if ok {
		t.Fatal("expected no native trailing arm after full-close cleanup")
	}
	trailingCount := 0
	for _, order := range fake.openOrders {
		if order.OrderID != "native-trailing" && order.Type == "TRAILING_STOP_MARKET" {
			trailingCount++
		}
	}
	if trailingCount != 0 {
		t.Fatalf("expected no native trailing write after full-close cleanup, got %d new trailing orders", trailingCount)
	}
}

func TestHandleSyncedFullClosePreservesOtherActiveSide(t *testing.T) {
	fake := &fakeReconcileTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		exchange:              "okx",
		trader:                fake,
		config:                AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState:       map[string]string{"BTCUSDT_short": "native_partial_trailing_armed", "BTCUSDT_long": "exchange_protection_verified"},
		breakEvenState:        map[string]string{"BTCUSDT_short": "armed", "BTCUSDT_long": "armed"},
		breakEvenFingerprints: map[string]string{"BTCUSDT_short": "short", "BTCUSDT_long": "long"},
		drawdownRunnerState:   map[string]DrawdownRunnerState{"BTCUSDT_short": {StageName: "short-runner"}, "BTCUSDT_long": {StageName: "long-runner"}},
		peakPnLCache:          map[string]float64{"BTCUSDT_short": 2.4, "BTCUSDT_long": 1.8},
		protectionStateMutex:  sync.RWMutex{},
		breakEvenStateMutex:   sync.RWMutex{},
		peakPnLCacheMutex:     sync.RWMutex{},
	}

	at.handleSyncedFullClose("BTCUSDT", "SHORT")

	if got := at.getProtectionState("BTCUSDT", "short"); got != "" {
		t.Fatalf("expected closed short protection state cleared, got %q", got)
	}
	if got := at.getProtectionState("BTCUSDT", "long"); got == "" {
		t.Fatal("expected active long protection state preserved")
	}
	if got := at.getBreakEvenState("BTCUSDT", "short"); got != "" {
		t.Fatalf("expected closed short break-even state cleared, got %q", got)
	}
	if got := at.getBreakEvenState("BTCUSDT", "long"); got == "" {
		t.Fatal("expected active long break-even state preserved")
	}
	if state := at.getDrawdownRunnerState("BTCUSDT", "short"); state != nil {
		t.Fatal("expected closed short runner state cleared")
	}
	if state := at.getDrawdownRunnerState("BTCUSDT", "long"); state == nil {
		t.Fatal("expected active long runner state preserved")
	}
	cache := at.GetPeakPnLCache()
	if _, ok := cache["BTCUSDT_short"]; ok {
		t.Fatal("expected closed short peak cache cleared")
	}
	if _, ok := cache["BTCUSDT_long"]; !ok {
		t.Fatal("expected active long peak cache preserved")
	}
	if len(fake.cancelStopOrdersCalls) != 0 || len(fake.cancelTrailingCalls) != 0 {
		t.Fatalf("expected no symbol-wide orphan cleanup while opposite side remains active, got stop=%v trailing=%v", fake.cancelStopOrdersCalls, fake.cancelTrailingCalls)
	}
}
