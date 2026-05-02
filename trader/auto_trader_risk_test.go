package trader

import (
	"fmt"
	"math"
	"nofx/store"
	tradertypes "nofx/trader/types"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProtectionTrader struct {
	mu                  sync.Mutex
	cancelStopLossCalls int
	setStopLossCalls    int
	lastSymbol          string
	lastPositionSide    string
	lastQuantity        float64
	lastStopPrice       float64
	cancelErr           error
	setStopLossErr      error
	trailingCalls       int
	cancelTrailingCalls int
	trailingSymbol      string
	trailingSide        string
	trailingActivation  float64
	trailingCallback    float64
	openOrders          []tradertypes.OpenOrder
	positions           []map[string]interface{}
	closeLongCalls      int
	closeShortCalls     int
	closeLongQtys       []float64
	closeShortQtys      []float64
	taggedCloseLongs    []string
	taggedCloseShorts   []string
	trailingDelay       time.Duration
}

func (f *fakeProtectionTrader) GetBalance() (map[string]interface{}, error) { return nil, nil }
func (f *fakeProtectionTrader) GetPositions() ([]map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.positions != nil {
		return append([]map[string]interface{}(nil), f.positions...), nil
	}
	return nil, nil
}
func (f *fakeProtectionTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	f.closeLongCalls++
	f.closeLongQtys = append(f.closeLongQtys, quantity)
	return map[string]interface{}{"orderId": fmt.Sprintf("close-long-%d", f.closeLongCalls)}, nil
}
func (f *fakeProtectionTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	f.closeShortCalls++
	f.closeShortQtys = append(f.closeShortQtys, quantity)
	return map[string]interface{}{"orderId": fmt.Sprintf("close-short-%d", f.closeShortCalls)}, nil
}
func (f *fakeProtectionTrader) CloseLongTagged(symbol string, quantity float64, reasonTag string) (map[string]interface{}, error) {
	f.taggedCloseLongs = append(f.taggedCloseLongs, reasonTag)
	return f.CloseLong(symbol, quantity)
}
func (f *fakeProtectionTrader) CloseShortTagged(symbol string, quantity float64, reasonTag string) (map[string]interface{}, error) {
	f.taggedCloseShorts = append(f.taggedCloseShorts, reasonTag)
	return f.CloseShort(symbol, quantity)
}
func (f *fakeProtectionTrader) SetLeverage(symbol string, leverage int) error         { return nil }
func (f *fakeProtectionTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (f *fakeProtectionTrader) GetMarketPrice(symbol string) (float64, error)         { return 0, nil }
func (f *fakeProtectionTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	f.setStopLossCalls++
	f.lastSymbol = symbol
	f.lastPositionSide = positionSide
	f.lastQuantity = quantity
	f.lastStopPrice = stopPrice
	if f.setStopLossErr != nil {
		return f.setStopLossErr
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
func (f *fakeProtectionTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}
func (f *fakeProtectionTrader) CancelStopLossOrders(symbol string) error {
	f.cancelStopLossCalls++
	f.openOrders = nil
	return f.cancelErr
}
func (f *fakeProtectionTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (f *fakeProtectionTrader) CancelAllOrders(symbol string) error        { return nil }
func (f *fakeProtectionTrader) CancelStopOrders(symbol string) error       { return nil }
func (f *fakeProtectionTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}
func (f *fakeProtectionTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]tradertypes.OpenOrder(nil), f.openOrders...), nil
}
func (f *fakeProtectionTrader) SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error {
	if f.trailingDelay > 0 {
		time.Sleep(f.trailingDelay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trailingCalls++
	f.trailingSymbol = symbol
	f.trailingSide = positionSide
	f.trailingActivation = activationPrice
	f.trailingCallback = callbackRate
	f.openOrders = append(f.openOrders, tradertypes.OpenOrder{
		OrderID:      fmt.Sprintf("new-tier-%d", f.trailingCalls),
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "TRAILING_STOP_MARKET",
		StopPrice:    activationPrice,
		CallbackRate: callbackRate,
		Quantity:     quantity,
		Status:       "NEW",
	})
	return nil
}
func (f *fakeProtectionTrader) SetTrailingStopLossTagged(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64, reasonTag string) error {
	_, err := f.SetTrailingStopLossTaggedWithID(symbol, positionSide, activationPrice, callbackRate, quantity, reasonTag)
	return err
}
func (f *fakeProtectionTrader) SetTrailingStopLossTaggedWithID(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64, reasonTag string) (string, error) {
	before := f.trailingCalls
	if err := f.SetTrailingStopLoss(symbol, positionSide, activationPrice, callbackRate, quantity); err != nil {
		return "", err
	}
	return fmt.Sprintf("new-tier-%d", before+1), nil
}
func (f *fakeProtectionTrader) CancelTrailingStopOrders(symbol string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	removed := 0
	filtered := make([]tradertypes.OpenOrder, 0, len(f.openOrders))
	for _, order := range f.openOrders {
		if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			removed++
			continue
		}
		filtered = append(filtered, order)
	}
	f.cancelTrailingCalls += removed
	f.openOrders = filtered
	return nil
}
func (f *fakeProtectionTrader) CancelTrailingStopOrdersByIDs(symbol string, orderIDs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelTrailingCalls += len(orderIDs)
	filtered := make([]tradertypes.OpenOrder, 0, len(f.openOrders))
	set := make(map[string]struct{}, len(orderIDs))
	for _, id := range orderIDs {
		set[id] = struct{}{}
	}
	for _, order := range f.openOrders {
		if _, ok := set[order.OrderID]; ok {
			continue
		}
		filtered = append(filtered, order)
	}
	f.openOrders = filtered
	return nil
}

func TestCheckPositionDrawdownSkipsDuplicateManagedPartialCloseForSameRule(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "ADAUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   101.0,
			"positionAmt": 170.0,
		}},
	}
	at := &AutoTrader{
		exchange: "paper",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					DrawdownTakeProfit: store.DrawdownTakeProfitConfig{Enabled: true, Rules: []store.DrawdownTakeProfitRule{{MinProfitPct: 0.7, MaxDrawdownPct: 55, CloseRatioPct: 70}}},
				},
			},
		},
		protectionState: make(map[string]string),
		drawdownState:   make(map[string]string),
		peakPnLCache:    map[string]float64{"ADAUSDT_long": 3.0},
	}

	at.checkPositionDrawdown()
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected first drawdown pass to close once, got %d", fake.closeLongCalls)
	}
	if len(fake.closeLongQtys) != 1 || fake.closeLongQtys[0] != 119 {
		t.Fatalf("expected close qty 119 on first pass, got %+v", fake.closeLongQtys)
	}

	at.checkPositionDrawdown()
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected duplicate drawdown pass to be skipped, got %d closes", fake.closeLongCalls)
	}
}

func TestCheckPositionDrawdownSkipsSameStageAfterPositionQuantityChanges(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "ADAUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   101.0,
			"positionAmt": 170.0,
		}},
	}
	at := &AutoTrader{
		exchange: "paper",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					DrawdownTakeProfit: store.DrawdownTakeProfitConfig{Enabled: true, Rules: []store.DrawdownTakeProfitRule{{MinProfitPct: 0.7, MaxDrawdownPct: 55, CloseRatioPct: 70}}},
				},
			},
		},
		protectionState: make(map[string]string),
		drawdownState:   make(map[string]string),
		peakPnLCache:    map[string]float64{"ADAUSDT_long": 3.0},
	}

	at.checkPositionDrawdown()
	fake.positions = []map[string]interface{}{{
		"symbol":      "ADAUSDT",
		"side":        "long",
		"entryPrice":  100.0,
		"markPrice":   101.0,
		"positionAmt": 51.0,
	}}
	at.checkPositionDrawdown()
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected same stage to stay guarded after quantity changed, got %d closes", fake.closeLongCalls)
	}
	if len(fake.closeLongQtys) != 1 {
		t.Fatalf("expected only first close qty to be recorded, got %+v", fake.closeLongQtys)
	}
}

func TestApplyNativeTrailingDrawdownRejectsNoiseCallbackForNativeAndFallsBack(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   106.0,
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		exchange: "binance",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   5,
		MaxDrawdownPct: 2,
		CloseRatioPct:  100,
	}

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected native trailing noise rejection to arm managed fallback and report protection armed")
	}
	if fake.trailingCalls != 0 {
		t.Fatalf("expected no native trailing call, got %d", fake.trailingCalls)
	}
	if state := at.getProtectionState("BTCUSDT", "long"); state != "managed_drawdown_armed" {
		t.Fatalf("expected managed fallback state, got %s", state)
	}
}

func TestApplyNativeTrailingDrawdownForBinance(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   106.0,
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		exchange: "binance",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   5,
		MaxDrawdownPct: 40,
		CloseRatioPct:  100,
	}

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected native trailing drawdown to be applied")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected 1 trailing call, got %d", fake.trailingCalls)
	}
	if fake.trailingActivation <= 100 {
		t.Fatalf("expected activation above entry for long, got %.4f", fake.trailingActivation)
	}
	if math.Abs(fake.trailingCallback-1.9048) > 0.0001 {
		t.Fatalf("expected callback rate about 1.9048, got %.4f", fake.trailingCallback)
	}
	if at.getProtectionState("BTCUSDT", "long") != "native_trailing_armed" {
		t.Fatalf("expected protection state native_trailing_armed, got %q", at.getProtectionState("BTCUSDT", "long"))
	}
}

func TestApplyNativeTrailingDrawdownSkipsDuplicateWhenEquivalentFullTrailingAlreadyExists(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   106.0,
			"positionAmt": 1.0,
		}},
		openOrders: []tradertypes.OpenOrder{{
			OrderID:      "existing-full",
			Symbol:       "BTCUSDT",
			PositionSide: "LONG",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    105.0,
			CallbackRate: 0.019048,
			Quantity:     0,
			Status:       "NEW",
		}},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: map[string]string{"BTCUSDT_long": "native_trailing_armed"},
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100}
	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected equivalent full trailing order to satisfy duplicate-arm guard")
	}
	if fake.trailingCalls != 0 {
		t.Fatalf("expected no replacement for equivalent full trailing order, got %d new trailing calls", fake.trailingCalls)
	}
	if fake.cancelTrailingCalls != 0 {
		t.Fatalf("expected no cancellation for equivalent full trailing order, got %d", fake.cancelTrailingCalls)
	}
}

func TestApplyNativeTrailingDrawdownReplacesStaleFullTrailingOrder(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   106.0,
			"positionAmt": 1.0,
		}},
		openOrders: []tradertypes.OpenOrder{{
			OrderID:      "stale-full",
			Symbol:       "BTCUSDT",
			PositionSide: "LONG",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    103.0,
			CallbackRate: 0.05,
			Quantity:     0,
			Status:       "NEW",
		}},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: map[string]string{"BTCUSDT_long": "native_trailing_armed"},
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100}
	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected stale full trailing order to be replaced")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected one replacement trailing call, got %d", fake.trailingCalls)
	}
	if fake.cancelTrailingCalls == 0 {
		t.Fatal("expected stale full trailing order to be canceled after replacement")
	}
	for _, order := range fake.openOrders {
		if order.OrderID == "stale-full" {
			t.Fatal("expected stale full trailing order removed from open orders after replacement")
		}
	}
}

func TestApplyNativeTrailingDrawdownPersistsFullTrailingOrderID(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "native-trailing-full.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   106.0,
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		id:              "trader-1",
		exchangeID:      "exchange-1",
		store:           st,
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100}
	if ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule); !ok {
		t.Fatal("expected native trailing drawdown to be applied")
	}

	state, err := st.LoadDynamicProtectionState()
	if err != nil {
		t.Fatalf("load dynamic protection state: %v", err)
	}
	key := store.BuildDynamicProtectionKey("trader-1", "exchange-1", "BTCUSDT", "long", "100.00000000|0.00000000", "native_trailing", drawdownRuleFingerprint(100, 0, rule), 100)
	record, ok := state.Records[key]
	if !ok {
		t.Fatalf("expected native trailing dynamic protection record for key %q", key)
	}
	if record.ExchangeOrderID != "new-tier-1" {
		t.Fatalf("expected persisted order id new-tier-1, got %q", record.ExchangeOrderID)
	}
	if record.ActivationPrice <= 100 {
		t.Fatalf("expected activation price persisted above entry, got %.4f", record.ActivationPrice)
	}
	if record.CallbackRatio <= 0 {
		t.Fatalf("expected callback ratio persisted, got %.6f", record.CallbackRatio)
	}
}

func TestApplyNativeTrailingDrawdownSkipsDuplicateWhenEquivalentPartialTierAlreadyExists(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "short",
			"entryPrice":  100.0,
			"markPrice":   94.0,
			"positionAmt": 2.0,
		}},
		openOrders: []tradertypes.OpenOrder{{
			OrderID:      "existing-partial",
			Symbol:       "BTCUSDT",
			PositionSide: "SHORT",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    95.0,
			CallbackRate: 0.021053,
			Quantity:     1.0,
			Status:       "NEW",
		}},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: map[string]string{"BTCUSDT_short": "native_partial_trailing_armed"},
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 50}
	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "short", 100, rule)
	if !ok {
		t.Fatal("expected equivalent partial trailing tier to satisfy duplicate-arm guard")
	}
	if fake.trailingCalls != 0 {
		t.Fatalf("expected no new partial trailing tier, got %d", fake.trailingCalls)
	}
	if fake.cancelTrailingCalls != 0 {
		t.Fatalf("expected no partial trailing cancellation, got %d", fake.cancelTrailingCalls)
	}
}

func TestApplyNativeTrailingDrawdownReplacesPartialTierWhenQuantityDrifts(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "short",
			"entryPrice":  100.0,
			"markPrice":   94.0,
			"positionAmt": 2.0,
		}},
		openOrders: []tradertypes.OpenOrder{{
			OrderID:      "stale-partial",
			Symbol:       "BTCUSDT",
			PositionSide: "SHORT",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    95.0,
			CallbackRate: 0.021053,
			Quantity:     0.6,
			Status:       "NEW",
		}},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: map[string]string{"BTCUSDT_short": "native_partial_trailing_armed"},
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 50}
	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "short", 100, rule)
	if !ok {
		t.Fatal("expected stale partial tier with qty drift to be replaced")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected one replacement partial tier, got %d", fake.trailingCalls)
	}
	if fake.cancelTrailingCalls == 0 {
		t.Fatal("expected stale partial tier to be canceled after replacement")
	}
	for _, order := range fake.openOrders {
		if order.OrderID == "stale-partial" {
			t.Fatal("expected stale partial tier removed from open orders after replacement")
		}
	}
}

func TestApplyNativeTrailingDrawdownConcurrentPartialArmingPlacesOnlyOneOrder(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "short",
			"entryPrice":  100.0,
			"markPrice":   94.0,
			"positionAmt": 2.0,
		}},
		trailingDelay: 20 * time.Millisecond,
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 50}
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- at.applyNativeTrailingDrawdown("BTCUSDT", "short", 100, rule)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for ok := range results {
		if !ok {
			t.Fatal("expected concurrent native partial trailing apply calls to resolve successfully")
		}
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected exactly one trailing placement under concurrent arming, got %d", fake.trailingCalls)
	}
	if got := at.getProtectionState("BTCUSDT", "short"); got != "native_partial_trailing_armed" {
		t.Fatalf("expected native_partial_trailing_armed after concurrent arming, got %q", got)
	}
}

func TestApplyNativeTrailingDrawdownReplacementPrefersBestMatchingPartialTierAmongMultiple(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "short",
			"entryPrice":  100.0,
			"markPrice":   94.0,
			"positionAmt": 2.0,
		}},
		openOrders: []tradertypes.OpenOrder{
			{
				OrderID:      "wrong-candidate",
				Symbol:       "BTCUSDT",
				PositionSide: "SHORT",
				Type:         "TRAILING_STOP_MARKET",
				StopPrice:    92.0,
				CallbackRate: 0.08,
				Quantity:     0.95,
				Status:       "NEW",
			},
			{
				OrderID:      "best-candidate",
				Symbol:       "BTCUSDT",
				PositionSide: "SHORT",
				Type:         "TRAILING_STOP_MARKET",
				StopPrice:    95.4,
				CallbackRate: 0.021053,
				Quantity:     0.8,
				Status:       "NEW",
			},
		},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: map[string]string{"BTCUSDT_short": "native_partial_trailing_armed"},
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 50}
	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "short", 100, rule)
	if !ok {
		t.Fatal("expected multi-tier stale partial replacement to succeed")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected one replacement partial tier, got %d", fake.trailingCalls)
	}
	foundWrong := false
	foundBest := false
	for _, order := range fake.openOrders {
		if order.OrderID == "wrong-candidate" {
			foundWrong = true
		}
		if order.OrderID == "best-candidate" {
			foundBest = true
		}
	}
	if !foundWrong {
		t.Fatal("expected unrelated lower-quality tier to remain")
	}
	if foundBest {
		t.Fatal("expected best-matching stale tier to be canceled")
	}
}

func TestApplyNativeTrailingDrawdownPersistsPartialTrailingOrderID(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "native-trailing-partial.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "short",
			"entryPrice":  100.0,
			"markPrice":   94.0,
			"positionAmt": 2.0,
		}},
	}
	at := &AutoTrader{
		id:              "trader-1",
		exchangeID:      "exchange-1",
		store:           st,
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 50}
	if ok := at.applyNativeTrailingDrawdown("BTCUSDT", "short", 100, rule); !ok {
		t.Fatal("expected native partial trailing drawdown to be applied")
	}

	state, err := st.LoadDynamicProtectionState()
	if err != nil {
		t.Fatalf("load dynamic protection state: %v", err)
	}
	key := store.BuildDynamicProtectionKey("trader-1", "exchange-1", "BTCUSDT", "short", "100.00000000|0.00000000", "native_partial_trailing", stableDrawdownRuleFingerprint(100, rule), 50)
	record, ok := state.Records[key]
	if !ok {
		t.Fatalf("expected native partial trailing dynamic protection record for key %q", key)
	}
	if record.ExchangeOrderID != "new-tier-1" {
		t.Fatalf("expected persisted order id new-tier-1, got %q", record.ExchangeOrderID)
	}
	if math.Abs(record.Quantity-1.0) > 0.0001 {
		t.Fatalf("expected persisted partial quantity 1.0, got %.4f", record.Quantity)
	}
	if record.CallbackRatio <= 0 {
		t.Fatalf("expected callback ratio persisted, got %.6f", record.CallbackRatio)
	}
}

func TestGetDrawdownArmRulesIgnoresArmedRecordsFromOldPositionFingerprint(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "drawdown-fingerprint.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	traderID := "trader-1"
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 0.8, MaxDrawdownPct: 70, CloseRatioPct: 50}
	oldFingerprint := stableDrawdownRuleFingerprint(0.10527, rule)
	if err := st.SaveDynamicProtectionRecord(store.DynamicProtectionRecord{
		TraderID:            traderID,
		ExchangeID:          "exchange-1",
		Symbol:              "DOGEUSDT",
		Side:                "short",
		PositionFingerprint: "0.10527000|610.00000000",
		ProtectionType:      "native_partial_trailing",
		RuleFingerprint:     oldFingerprint,
		CloseRatioPct:       50,
		Status:              "armed",
	}); err != nil {
		t.Fatalf("save old record: %v", err)
	}

	at := &AutoTrader{id: traderID, store: st, trader: &fakeProtectionTrader{}}
	if got := at.getDrawdownArmRules(1.0, 0.10504, 610, "DOGEUSDT", "short", []store.DrawdownTakeProfitRule{rule}); len(got) != 1 {
		t.Fatalf("expected current position to arm despite stale old entry fingerprint, got %d", len(got))
	}
	at.trader = &fakeProtectionTrader{openOrders: []tradertypes.OpenOrder{{
		Symbol:       "DOGEUSDT",
		PositionSide: "SHORT",
		Type:         "TRAILING_STOP_MARKET",
		StopPrice:    calculateProfitBasedTrailingTriggerPrice(0.10527, "short", rule.MinProfitPct),
		CallbackRate: calculateProfitBasedTrailingCallbackRatio(0.10527, "short", rule.MinProfitPct, rule.MaxDrawdownPct),
		Quantity:     305,
	}}}
	if got := at.getDrawdownArmRules(1.0, 0.10527, 305, "DOGEUSDT", "short", []store.DrawdownTakeProfitRule{rule}); len(got) != 0 {
		t.Fatalf("expected same entry/stage to remain guarded when exchange trailing still exists, got %d", len(got))
	}
	at.trader = &fakeProtectionTrader{}
	if got := at.getDrawdownArmRules(1.0, 0.10527, 305, "DOGEUSDT", "short", []store.DrawdownTakeProfitRule{rule}); len(got) != 1 {
		t.Fatalf("expected stale armed record without exchange order to re-arm, got %d", len(got))
	}
}

func TestApplyBreakEvenStopSkipsAndCleansWhenPositionClosedBeforeWrite(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: nil,
		openOrders: []tradertypes.OpenOrder{{
			Symbol:       "BTCUSDT",
			PositionSide: "SHORT",
			Type:         "STOP_MARKET",
			StopPrice:    78000,
			Quantity:     0.0002,
		}},
	}
	at := &AutoTrader{
		exchange:              "okx",
		trader:                fake,
		config:                AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState:       map[string]string{"BTCUSDT_short": "native_partial_trailing_armed"},
		breakEvenState:        map[string]string{"BTCUSDT_short": "pending"},
		breakEvenFingerprints: map[string]string{"BTCUSDT_short": "78867.80000000|0.00020000"},
		peakPnLCache:          map[string]float64{"BTCUSDT_short": 2.0},
	}

	err := at.applyBreakEvenStop("BTCUSDT", "short", 0.0002, 78867.8, 1.2, store.BreakEvenStopConfig{
		Enabled:      true,
		TriggerMode:  store.BreakEvenTriggerProfitPct,
		TriggerValue: 0.7,
		OffsetPct:    0.3,
	})
	if err != nil {
		t.Fatalf("expected closed-position break-even guard to skip without error, got %v", err)
	}
	if fake.setStopLossCalls != 0 {
		t.Fatalf("expected no break-even stop write after position close, got %d", fake.setStopLossCalls)
	}
	if got := at.getProtectionState("BTCUSDT", "short"); got != "" {
		t.Fatalf("expected inactive protection state cleared, got %q", got)
	}
	if got := at.getBreakEvenState("BTCUSDT", "short"); got != "" {
		t.Fatalf("expected inactive break-even state cleared, got %q", got)
	}
}

func TestApplyNativeTrailingDrawdownSkipsWhenPositionClosedBeforeWrite(t *testing.T) {
	fake := &fakeProtectionTrader{positions: nil}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: map[string]string{"BTCUSDT_short": "exchange_protection_verified"},
		breakEvenState:  map[string]string{},
		peakPnLCache:    map[string]float64{},
	}

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "short", 78867.8, store.DrawdownTakeProfitRule{
		MinProfitPct:   0.8,
		MaxDrawdownPct: 70,
		CloseRatioPct:  50,
	})
	if ok {
		t.Fatal("expected native trailing arm to be skipped for closed position")
	}
	if fake.trailingCalls != 0 {
		t.Fatalf("expected no trailing order after position close, got %d", fake.trailingCalls)
	}
	if got := at.getProtectionState("BTCUSDT", "short"); got != "" {
		t.Fatalf("expected inactive protection state cleared, got %q", got)
	}
}

func TestCheckPositionDrawdownActivatesRunnerAndSuppressesBreakEvenAfterPartialClose(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "TAOUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   101.0,
			"positionAmt": 10.0,
		}},
	}
	at := &AutoTrader{
		exchange: "paper",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					DrawdownTakeProfit: store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeManual, EngineMode: store.DrawdownEngineModeAI, RunnerEnabled: true, MinRunnerKeepPct: 20, MaxFirstReducePct: 60, BreakEvenRunnerPolicy: store.DrawdownBreakEvenRunnerFallbackOnly, Rules: []store.DrawdownTakeProfitRule{{
						MinProfitPct:     0.7,
						MaxDrawdownPct:   55,
						CloseRatioPct:    70,
						StageName:        "lock_first_profit",
						RunnerKeepPct:    30,
						RunnerStopMode:   "structure",
						RunnerStopSource: "adjacent_support_flip",
					}}},
					BreakEvenStop: store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 0.5},
				},
			},
		},
		protectionState:       make(map[string]string),
		breakEvenState:        make(map[string]string),
		breakEvenFingerprints: make(map[string]string),
		drawdownState:         make(map[string]string),
		drawdownRunnerState:   make(map[string]DrawdownRunnerState),
		peakPnLCache:          map[string]float64{"TAOUSDT_long": 3.0},
	}

	at.checkPositionDrawdown()
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected partial drawdown close, got %d", fake.closeLongCalls)
	}
	if len(fake.closeLongQtys) != 1 || fake.closeLongQtys[0] != 6.0 {
		t.Fatalf("expected capped first reduce qty 6.0, got %+v", fake.closeLongQtys)
	}
	if fake.setStopLossCalls != 0 {
		t.Fatalf("expected no break-even placement on unsupported paper exchange, got %d", fake.setStopLossCalls)
	}
	if !at.isBreakEvenSuppressedByRunner("TAOUSDT", "long") {
		t.Fatal("expected runner to suppress subsequent break-even")
	}
	state := at.getDrawdownRunnerState("TAOUSDT", "long")
	if state == nil {
		t.Fatal("expected runner state recorded")
	}
	if state.StageName != "lock_first_profit" {
		t.Fatalf("expected stage name preserved from configured rule, got %q", state.StageName)
	}
	if state.RunnerKeepPct != 40 {
		t.Fatalf("expected runner keep pct 40 after cap, got %.2f", state.RunnerKeepPct)
	}
	if state.RunnerStopSource != "adjacent_support_flip" {
		t.Fatalf("expected runner stop source preserved, got %q", state.RunnerStopSource)
	}

	fake.positions[0]["positionAmt"] = 4.0
	at.checkPositionDrawdown()
	if fake.setStopLossCalls != 0 {
		t.Fatalf("expected no break-even apply after runner activation, got %d", fake.setStopLossCalls)
	}
}

func TestEvaluateAIDrawdownRuleRespectsRunnerPolicyAndStageDefaults(t *testing.T) {
	cfg := store.DrawdownTakeProfitConfig{
		Enabled:               true,
		Mode:                  store.ProtectionModeAI,
		EngineMode:            store.DrawdownEngineModeAI,
		RunnerEnabled:         true,
		MinRunnerKeepPct:      25,
		MaxFirstReducePct:     60,
		BreakEvenRunnerPolicy: store.DrawdownBreakEvenRunnerFallbackOnly,
	}
	rules := []store.DrawdownTakeProfitRule{{
		MinProfitPct:   4,
		MaxDrawdownPct: 20,
		CloseRatioPct:  80,
	}}

	eval := evaluateAIDrawdownRule(cfg, 4.2, 6.0, 30.0, rules, nil, "long", 0)
	if eval == nil {
		t.Fatal("expected ai drawdown evaluation")
	}
	if eval.Rule.StageName != "near_primary_target" {
		t.Fatalf("expected near_primary_target stage, got %q", eval.Rule.StageName)
	}
	if eval.Rule.CloseRatioPct != 60 {
		t.Fatalf("expected first reduce capped at 60, got %.2f", eval.Rule.CloseRatioPct)
	}
	if eval.Rule.RunnerKeepPct != 40 {
		t.Fatalf("expected runner keep 40 after cap, got %.2f", eval.Rule.RunnerKeepPct)
	}
	if eval.Rule.RunnerStopMode != "structure" {
		t.Fatalf("expected structure runner stop in fallback_only policy, got %q", eval.Rule.RunnerStopMode)
	}
	if eval.Rule.RunnerStopSource != "primary_target_pullback" {
		t.Fatalf("expected default stop source for near_primary_target, got %q", eval.Rule.RunnerStopSource)
	}
	if eval.Rule.RunnerTargetSource != "primary_resistance" {
		t.Fatalf("expected default target source for near_primary_target, got %q", eval.Rule.RunnerTargetSource)
	}
}

func TestEvaluateAIDrawdownRuleUsesStructureContextForStageAndSources(t *testing.T) {
	cfg := store.DrawdownTakeProfitConfig{
		Enabled:               true,
		Mode:                  store.ProtectionModeAI,
		EngineMode:            store.DrawdownEngineModeAI,
		RunnerEnabled:         true,
		MinRunnerKeepPct:      20,
		MaxFirstReducePct:     60,
		BreakEvenRunnerPolicy: store.DrawdownBreakEvenRunnerFallbackOnly,
	}
	rules := []store.DrawdownTakeProfitRule{{
		MinProfitPct:   4,
		MaxDrawdownPct: 20,
		CloseRatioPct:  70,
	}}
	structure := &drawdownStructureContext{
		Entry:       100,
		FirstTarget: 110,
		Resistance:  []float64{110},
		FibLevels:   []float64{111.8},
		Anchors: []store.DecisionActionReasonAnchor{{
			Type:      "first_target",
			Timeframe: "15m",
			Price:     110,
			Reason:    "primary resistance objective",
		}},
	}

	eval := evaluateAIDrawdownRule(cfg, 9.0, 11.0, 25.0, rules, structure, "long", 111.8)
	if eval == nil {
		t.Fatal("expected ai drawdown evaluation")
	}
	if eval.Rule.StageName != "extension_exhaustion" {
		t.Fatalf("expected structure-driven extension_exhaustion stage, got %q", eval.Rule.StageName)
	}
	if eval.Rule.RunnerStopSource != "extension_swing_trail" {
		t.Fatalf("expected extension stop source, got %q", eval.Rule.RunnerStopSource)
	}
	if eval.Rule.RunnerTargetSource != "extension_fibonacci" {
		t.Fatalf("expected extension fibonacci target source, got %q", eval.Rule.RunnerTargetSource)
	}
}

func TestFindMatchedDecisionActionReturnsUnderlyingSlicePointer(t *testing.T) {
	record := &store.DecisionRecord{
		Decisions: []store.DecisionAction{
			{Symbol: "BTCUSDT", Action: "wait"},
			{Symbol: "BTCUSDT", Action: "open_long", ReviewContext: &store.DecisionActionReviewContext{PrimaryTimeframe: "15m"}},
		},
	}

	matched := findMatchedDecisionAction(record, "BTCUSDT", "open_long")
	if matched == nil {
		t.Fatal("expected matched action")
	}
	matched.ReviewContext.PrimaryTimeframe = "1h"
	if record.Decisions[1].ReviewContext == nil || record.Decisions[1].ReviewContext.PrimaryTimeframe != "1h" {
		t.Fatalf("expected mutation on underlying slice element, got %+v", record.Decisions[1].ReviewContext)
	}
}

func TestBuildEntryReviewSummaryFromDecisionReviewWhitelistsFields(t *testing.T) {
	review := map[string]interface{}{
		"timeframe_context":     map[string]interface{}{"primary": "15m"},
		"risk_reward":           map[string]interface{}{"entry": 100.0, "invalidation": 95.0, "first_target": 110.0},
		"key_levels":            map[string]interface{}{"support": []interface{}{99.0}},
		"anchors":               []interface{}{map[string]interface{}{"type": "support", "price": 99.0}},
		"alignment_notes":       []interface{}{"target above local resistance"},
		"control":               map[string]interface{}{"decision": "accepted"},
		"execution_constraints": map[string]interface{}{"tick_size": 0.1},
		"unexpected":            "drop-me",
	}

	summary := buildEntryReviewSummaryFromDecisionReview(review)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if _, ok := summary["unexpected"]; ok {
		t.Fatalf("unexpected key should be filtered out: %+v", summary)
	}
	for _, key := range []string{"timeframe_context", "risk_reward", "key_levels", "anchors", "alignment_notes", "control", "execution_constraints"} {
		if _, ok := summary[key]; !ok {
			t.Fatalf("expected key %s in summary: %+v", key, summary)
		}
	}
}

func TestGetDrawdownArmRulesForNativeExposureSelectsOneStableTierBeforeProfitGate(t *testing.T) {
	at := &AutoTrader{trader: &fakeProtectionTrader{}}
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 0.8, MaxDrawdownPct: 55, CloseRatioPct: 50},
		{MinProfitPct: 1.5, MaxDrawdownPct: 45, CloseRatioPct: 80},
	}
	got := at.getDrawdownArmRulesForNativeExposure(0.2, 100, 1, "BTCUSDT", "long", rules)
	if len(got) != 1 || got[0].MinProfitPct != 0.8 {
		t.Fatalf("expected nearest native tier before profit gate, got %+v", got)
	}
	got = at.getDrawdownArmRulesForNativeExposure(2.0, 100, 1, "BTCUSDT", "long", rules)
	if len(got) != 1 || got[0].MinProfitPct != 1.5 {
		t.Fatalf("expected highest satisfied native tier after profit advances, got %+v", got)
	}
}

func TestApplyNativeTrailingDrawdownBelowSafetyFloorArmsManagedFullFallback(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{"symbol": "BTCUSDT", "side": "long", "positionAmt": 1.0}},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{Exchange: "okx", StrategyConfig: &store.StrategyConfig{}},
		protectionState: make(map[string]string),
	}
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 0.1, MaxDrawdownPct: 10, CloseRatioPct: 100, StageName: "outer_exit"}
	if !at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule) {
		t.Fatalf("expected managed fallback to be armed")
	}
	if got := at.getProtectionState("BTCUSDT", "long"); got != "managed_drawdown_armed" {
		t.Fatalf("state=%s, want managed_drawdown_armed", got)
	}
	if fake.trailingCalls != 0 {
		t.Fatalf("native trailing should not be placed below safety floor, got %d", fake.trailingCalls)
	}
}
