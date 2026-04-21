package trader

import (
	"strings"
	"sync"
	"testing"
	"time"

	"nofx/store"
	tradertypes "nofx/trader/types"
)

type fakeReconcileTrader struct {
	fakeOrderProtectionTrader
	positions             []map[string]interface{}
	cancelStopOrdersCalls []string
	cancelTrailingCalls   []string
	cancelStopLossCalls   []string
	cancelTakeProfitCalls []string
}

func (f *fakeReconcileTrader) GetPositions() ([]map[string]interface{}, error) {
	return f.positions, nil
}

func (f *fakeReconcileTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if err := f.fakeOrderProtectionTrader.SetStopLoss(symbol, positionSide, quantity, stopPrice); err != nil {
		return err
	}
	f.openOrders = append(f.openOrders, tradertypes.OpenOrder{
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "STOP_MARKET",
		StopPrice:    stopPrice,
		Quantity:     quantity,
	})
	return nil
}

func (f *fakeReconcileTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if err := f.fakeOrderProtectionTrader.SetTakeProfit(symbol, positionSide, quantity, takeProfitPrice); err != nil {
		return err
	}
	f.openOrders = append(f.openOrders, tradertypes.OpenOrder{
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "TAKE_PROFIT_MARKET",
		StopPrice:    takeProfitPrice,
		Quantity:     quantity,
	})
	return nil
}

func (f *fakeReconcileTrader) CancelStopOrders(symbol string) error {
	f.cancelStopOrdersCalls = append(f.cancelStopOrdersCalls, symbol)
	filtered := f.openOrders[:0]
	for _, order := range f.openOrders {
		if order.Symbol == symbol && (looksLikeStopLoss(order) || strings.Contains(strings.ToUpper(order.Type), "TRAILING")) {
			continue
		}
		filtered = append(filtered, order)
	}
	f.openOrders = filtered
	return nil
}

func (f *fakeReconcileTrader) CancelTrailingStopOrders(symbol string) error {
	f.cancelTrailingCalls = append(f.cancelTrailingCalls, symbol)
	filtered := f.openOrders[:0]
	for _, order := range f.openOrders {
		if order.Symbol == symbol && strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			continue
		}
		filtered = append(filtered, order)
	}
	f.openOrders = filtered
	return nil
}

func (f *fakeReconcileTrader) CancelStopLossOrders(symbol string) error {
	f.cancelStopLossCalls = append(f.cancelStopLossCalls, symbol)
	filtered := f.openOrders[:0]
	for _, order := range f.openOrders {
		if order.Symbol == symbol && looksLikeStopLoss(order) {
			continue
		}
		filtered = append(filtered, order)
	}
	f.openOrders = filtered
	return nil
}

func (f *fakeReconcileTrader) CancelTakeProfitOrders(symbol string) error {
	f.cancelTakeProfitCalls = append(f.cancelTakeProfitCalls, symbol)
	filtered := f.openOrders[:0]
	for _, order := range f.openOrders {
		if order.Symbol == symbol && looksLikeTakeProfit(order) {
			continue
		}
		filtered = append(filtered, order)
	}
	f.openOrders = filtered
	return nil
}

func TestDetectMissingProtectionRequiresFallbackMaxLossStop(t *testing.T) {
	orders := []OpenOrder{{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98}}
	plan := &ProtectionPlan{
		NeedsStopLoss:        true,
		StopLossPrice:        98,
		FallbackMaxLossPrice: 95,
	}

	missingSL, missingTP := detectMissingProtection(orders, "LONG", plan)
	if !missingSL {
		t.Fatal("expected missingSL when fallback max-loss stop is absent")
	}
	if missingTP {
		t.Fatal("did not expect take-profit to be missing")
	}
}

func TestDetectMissingProtectionAcceptsFallbackMaxLossStopWhenPresent(t *testing.T) {
	orders := []OpenOrder{
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98},
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 95},
	}
	plan := &ProtectionPlan{
		NeedsStopLoss:        true,
		StopLossPrice:        98,
		FallbackMaxLossPrice: 95,
	}

	missingSL, missingTP := detectMissingProtection(orders, "LONG", plan)
	if missingSL || missingTP {
		t.Fatalf("expected stop protections satisfied, got missingSL=%v missingTP=%v", missingSL, missingTP)
	}
}

func TestDetectMissingProtectionAcceptsDegradedFullStopAndFallbackInsteadOfMissingLadderStops(t *testing.T) {
	orders := []OpenOrder{
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98},
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 95},
	}
	plan := &ProtectionPlan{
		NeedsStopLoss:        true,
		StopLossPrice:        98,
		FallbackMaxLossPrice: 95,
		StopLossOrders: []ProtectionOrder{
			{Price: 98, CloseRatioPct: 50},
			{Price: 96, CloseRatioPct: 50},
		},
	}

	missingSL, missingTP := detectMissingProtection(orders, "LONG", plan)
	if missingSL || missingTP {
		t.Fatalf("expected degraded full+fallback stop ownership to satisfy protection, got missingSL=%v missingTP=%v", missingSL, missingTP)
	}
}

func TestDetectMissingProtectionAcceptsDegradedFullTakeProfitInsteadOfMissingLadderTP(t *testing.T) {
	orders := []OpenOrder{{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 110}}
	plan := &ProtectionPlan{
		NeedsTakeProfit: true,
		TakeProfitPrice: 110,
		TakeProfitOrders: []ProtectionOrder{
			{Price: 105, CloseRatioPct: 50},
			{Price: 110, CloseRatioPct: 50},
		},
	}

	missingSL, missingTP := detectMissingProtection(orders, "LONG", plan)
	if missingSL || missingTP {
		t.Fatalf("expected degraded full TP ownership to satisfy protection, got missingSL=%v missingTP=%v", missingSL, missingTP)
	}
}

func TestProtectionReconciler_DoesNotReapplyWhenDustRemainderAlreadyHasFullStopAndFallback(t *testing.T) {
	orders := []OpenOrder{
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 4780, Quantity: 0.001, ClientOrderID: "full_sl_1"},
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 4750, Quantity: 0.001, ClientOrderID: "fallback_maxloss_sl_1"},
	}
	plan := &ProtectionPlan{
		NeedsStopLoss:        true,
		StopLossPrice:        4780,
		FallbackMaxLossPrice: 4750,
		StopLossOrders: []ProtectionOrder{
			{Price: 4780, CloseRatioPct: 50},
			{Price: 4776, CloseRatioPct: 50},
		},
	}

	missingSL, missingTP := detectMissingProtection(orders, "LONG", plan)
	if missingSL || missingTP {
		t.Fatalf("expected degraded dust remainder stop ownership to be accepted, got missingSL=%v missingTP=%v", missingSL, missingTP)
	}
}

func TestProtectionReconciler_DoesNotReapplyBreakEvenWhenAlreadyArmedAndFingerprintStable(t *testing.T) {
	ft := &fakeReconcileTrader{
		fakeOrderProtectionTrader: fakeOrderProtectionTrader{
			openOrders: []tradertypes.OpenOrder{},
		},
		positions: []map[string]interface{}{
			{
				"symbol":      "BTCUSDT",
				"side":        "long",
				"entryPrice":  100.0,
				"positionAmt": 1.0,
				"markPrice":   106.0,
			},
		},
	}

	at := &AutoTrader{
		exchange: "okx",
		trader:   ft,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					BreakEvenStop: store.BreakEvenStopConfig{
						Enabled:      true,
						TriggerMode:  store.BreakEvenTriggerProfitPct,
						TriggerValue: 5,
						OffsetPct:    0,
					},
				},
			},
		},
		protectionState:       make(map[string]string),
		breakEvenState:        make(map[string]string),
		breakEvenFingerprints: make(map[string]string),
	}

	at.reconcilePositionProtections()
	if len(ft.stopLossOrders) != 1 {
		t.Fatalf("expected initial break-even stop placement, got %d", len(ft.stopLossOrders))
	}

	before := len(ft.stopLossOrders)
	at.reconcilePositionProtections()
	if len(ft.stopLossOrders) != before {
		t.Fatalf("expected no duplicate break-even placement when already armed, got %d stop-loss orders", len(ft.stopLossOrders))
	}
}

func TestCleanupInactiveProtectionState_CancelsOrphanedOrdersAndClearsLocalState(t *testing.T) {
	ft := &fakeReconcileTrader{
		fakeOrderProtectionTrader: fakeOrderProtectionTrader{
			openOrders: []tradertypes.OpenOrder{
				{Symbol: "BTCUSDT", PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98, Quantity: 1},
				{Symbol: "BTCUSDT", PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 105, Quantity: 1},
				{Symbol: "BTCUSDT", PositionSide: "LONG", Type: "TRAILING_STOP_MARKET", StopPrice: 104, CallbackRate: 0.02, Quantity: 1},
			},
		},
	}

	at := &AutoTrader{
		trader:                ft,
		protectionState:       map[string]string{"BTCUSDT_long": "native_trailing_armed"},
		breakEvenState:        map[string]string{"BTCUSDT_long": "armed"},
		breakEvenFingerprints: map[string]string{"BTCUSDT_long": "100.00000000|1.00000000"},
		peakPnLCache:          map[string]float64{"BTCUSDT_long": 8.5},
		protectionStateMutex:  sync.RWMutex{},
		breakEvenStateMutex:   sync.RWMutex{},
		peakPnLCacheMutex:     sync.RWMutex{},
	}

	reconcileCooldownMutex.Lock()
	reconcileCooldowns["BTCUSDT_long"] = time.Now()
	reconcileCooldownMutex.Unlock()

	at.cleanupInactiveProtectionState(map[string]struct{}{})

	if len(ft.cancelStopOrdersCalls) != 1 || ft.cancelStopOrdersCalls[0] != "BTCUSDT" {
		t.Fatalf("expected orphan stop cleanup for BTCUSDT, got %+v", ft.cancelStopOrdersCalls)
	}
	if len(ft.cancelTrailingCalls) != 1 || ft.cancelTrailingCalls[0] != "BTCUSDT" {
		t.Fatalf("expected orphan trailing cleanup for BTCUSDT, got %+v", ft.cancelTrailingCalls)
	}
	if got := at.getProtectionState("BTCUSDT", "long"); got != "" {
		t.Fatalf("expected protection state cleared, got %q", got)
	}
	if got := at.getBreakEvenState("BTCUSDT", "long"); got != "" {
		t.Fatalf("expected break-even state cleared, got %q", got)
	}
	if _, ok := at.GetPeakPnLCache()["BTCUSDT_long"]; ok {
		t.Fatal("expected peak cache cleared for inactive position")
	}
	reconcileCooldownMutex.RLock()
	_, cooldownExists := reconcileCooldowns["BTCUSDT_long"]
	reconcileCooldownMutex.RUnlock()
	if cooldownExists {
		t.Fatal("expected reconcile cooldown cleared for inactive position")
	}
}

func TestCleanupInactiveProtectionState_DoesNotCancelOrdersWhenOppositeSideStillActive(t *testing.T) {
	ft := &fakeReconcileTrader{}
	at := &AutoTrader{
		trader:                ft,
		protectionState:       map[string]string{"BTCUSDT_long": "native_trailing_armed", "BTCUSDT_short": "exchange_protection_verified"},
		breakEvenState:        map[string]string{"BTCUSDT_long": "armed", "BTCUSDT_short": "armed"},
		breakEvenFingerprints: map[string]string{"BTCUSDT_long": "100|1", "BTCUSDT_short": "100|1"},
		drawdownRunnerState:   map[string]DrawdownRunnerState{"BTCUSDT_long": {StageName: "runner"}, "BTCUSDT_short": {StageName: "runner"}},
		peakPnLCache:          map[string]float64{"BTCUSDT_long": 4.2, "BTCUSDT_short": 3.1},
	}

	active := map[string]struct{}{"BTCUSDT_short": {}}
	at.cleanupInactiveProtectionState(active)

	if len(ft.cancelStopOrdersCalls) != 0 || len(ft.cancelTrailingCalls) != 0 {
		t.Fatalf("expected no orphan order cleanup while opposite side still active, got stop=%v trailing=%v", ft.cancelStopOrdersCalls, ft.cancelTrailingCalls)
	}
	if got := at.getProtectionState("BTCUSDT", "short"); got == "" {
		t.Fatal("expected active short protection state to remain")
	}
	if got := at.getProtectionState("BTCUSDT", "long"); got != "" {
		t.Fatalf("expected inactive long protection state cleared, got %q", got)
	}
	if got := at.getBreakEvenState("BTCUSDT", "short"); got == "" {
		t.Fatal("expected active short break-even state to remain")
	}
	if got := at.getBreakEvenState("BTCUSDT", "long"); got != "" {
		t.Fatalf("expected inactive long break-even state cleared, got %q", got)
	}
	if state := at.getDrawdownRunnerState("BTCUSDT", "short"); state == nil {
		t.Fatal("expected active short runner state to remain")
	}
	if state := at.getDrawdownRunnerState("BTCUSDT", "long"); state != nil {
		t.Fatal("expected inactive long runner state cleared")
	}
	cache := at.GetPeakPnLCache()
	if _, ok := cache["BTCUSDT_short"]; !ok {
		t.Fatal("expected active short peak cache to remain")
	}
	if _, ok := cache["BTCUSDT_long"]; ok {
		t.Fatal("expected inactive long peak cache cleared")
	}
}

func TestProtectionReconciler_SkipsBreakEvenWhenRunnerSuppressesIt(t *testing.T) {
	ft := &fakeReconcileTrader{
		fakeOrderProtectionTrader: fakeOrderProtectionTrader{
			openOrders: []tradertypes.OpenOrder{},
		},
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"positionAmt": 1.0,
			"markPrice":   106.0,
		}},
	}

	at := &AutoTrader{
		exchange: "okx",
		trader:   ft,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					BreakEvenStop: store.BreakEvenStopConfig{
						Enabled:      true,
						TriggerMode:  store.BreakEvenTriggerProfitPct,
						TriggerValue: 5,
						OffsetPct:    0,
					},
				},
			},
		},
		protectionState:       make(map[string]string),
		breakEvenState:        make(map[string]string),
		breakEvenFingerprints: make(map[string]string),
		drawdownRunnerState:   map[string]DrawdownRunnerState{"BTCUSDT_long": {StageName: "lock_first_profit", RunnerKeepPct: 30, RunnerStopMode: "structure", BreakEvenSuppressedByRunner: true}},
	}

	at.reconcilePositionProtections()
	if len(ft.stopLossOrders) != 0 {
		t.Fatalf("expected no break-even stop placement when runner suppresses it, got %d", len(ft.stopLossOrders))
	}
}
