package trader

import (
	"fmt"
	"nofx/store"
	tradertypes "nofx/trader/types"
	"strings"
	"testing"
	"time"
)

type fakeProtectionTrader struct {
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
}

func (f *fakeProtectionTrader) GetBalance() (map[string]interface{}, error) { return nil, nil }
func (f *fakeProtectionTrader) GetPositions() ([]map[string]interface{}, error) {
	if f.positions != nil {
		return f.positions, nil
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
	return nil, nil
}
func (f *fakeProtectionTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
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
	return f.openOrders, nil
}
func (f *fakeProtectionTrader) SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error {
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
	return f.SetTrailingStopLoss(symbol, positionSide, activationPrice, callbackRate, quantity)
}
func (f *fakeProtectionTrader) CancelTrailingStopOrders(symbol string) error {
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

func TestApplyNativeTrailingDrawdownForBinance(t *testing.T) {
	fake := &fakeProtectionTrader{}
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
		t.Fatal("expected native trailing drawdown to be applied")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected 1 trailing call, got %d", fake.trailingCalls)
	}
	if fake.trailingActivation <= 100 {
		t.Fatalf("expected activation above entry for long, got %.4f", fake.trailingActivation)
	}
	if fake.trailingCallback != 0.1 {
		t.Fatalf("expected callback rate 0.1, got %.4f", fake.trailingCallback)
	}
	if at.getProtectionState("BTCUSDT", "long") != "native_trailing_armed" {
		t.Fatalf("expected protection state native_trailing_armed, got %q", at.getProtectionState("BTCUSDT", "long"))
	}
}

func TestApplyNativeTrailingDrawdownForBitget(t *testing.T) {
	fake := &fakeProtectionTrader{}
	at := &AutoTrader{
		exchange: "bitget",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   4,
		MaxDrawdownPct: 1.5,
		CloseRatioPct:  100,
	}

	ok := at.applyNativeTrailingDrawdown("ETHUSDT", "short", 100, rule)
	if !ok {
		t.Fatal("expected native trailing drawdown to be applied for bitget")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected 1 trailing call, got %d", fake.trailingCalls)
	}
	if fake.trailingActivation >= 100 {
		t.Fatalf("expected activation below entry for short, got %.4f", fake.trailingActivation)
	}
	if fake.trailingCallback != 0.1 {
		t.Fatalf("expected callback rate 0.1, got %.4f", fake.trailingCallback)
	}
	if at.getProtectionState("ETHUSDT", "short") != "native_trailing_armed" {
		t.Fatalf("expected protection state native_trailing_armed, got %q", at.getProtectionState("ETHUSDT", "short"))
	}
}

func TestApplyNativeTrailingDrawdownForOKX(t *testing.T) {
	fake := &fakeProtectionTrader{}
	at := &AutoTrader{
		exchange: "okx",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   6,
		MaxDrawdownPct: 2.5,
		CloseRatioPct:  100,
	}

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected native trailing drawdown to be applied for okx")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected 1 trailing call, got %d", fake.trailingCalls)
	}
	if fake.trailingActivation <= 100 {
		t.Fatalf("expected activation above entry for long, got %.4f", fake.trailingActivation)
	}
	if fake.trailingCallback != 0.0014150943396226417 {
		t.Fatalf("expected callback rate 0.001415, got %.4f", fake.trailingCallback)
	}
	if at.getProtectionState("BTCUSDT", "long") != "native_trailing_armed" {
		t.Fatalf("expected protection state native_trailing_armed, got %q", at.getProtectionState("BTCUSDT", "long"))
	}
}

func TestApplyNativePartialTrailingDrawdownAllowsMultipleTiers(t *testing.T) {
	fake := &fakeProtectionTrader{
		openOrders: []tradertypes.OpenOrder{{
			Symbol:       "BTCUSDT",
			PositionSide: "LONG",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    105,
			CallbackRate: 0.001,
			Quantity:     0.2,
			Status:       "NEW",
		}},
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		exchange: "okx",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
		protectionState: make(map[string]string),
	}
	at.setProtectionState("BTCUSDT", "long", "native_partial_trailing_armed")

	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   6,
		MaxDrawdownPct: 2.5,
		CloseRatioPct:  50,
	}

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected native partial trailing drawdown to be applied for second tier")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected a new trailing call for unmatched second tier, got %d", fake.trailingCalls)
	}
	if at.getProtectionState("BTCUSDT", "long") != "native_partial_trailing_armed" {
		t.Fatalf("expected protection state native_partial_trailing_armed, got %q", at.getProtectionState("BTCUSDT", "long"))
	}
}

func TestApplyNativePartialTrailingDrawdownSkipsEquivalentTier(t *testing.T) {
	entryPrice := 100.0
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   6,
		MaxDrawdownPct: 2.5,
		CloseRatioPct:  50,
	}
	callback := calculateProfitBasedTrailingCallbackRatio(entryPrice, "long", rule.MinProfitPct, rule.MaxDrawdownPct)
	fake := &fakeProtectionTrader{
		openOrders: []tradertypes.OpenOrder{{
			Symbol:       "BTCUSDT",
			PositionSide: "LONG",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    106,
			CallbackRate: callback,
			Quantity:     0.5,
			Status:       "NEW",
		}},
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		exchange: "okx",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
		protectionState: make(map[string]string),
	}
	at.setProtectionState("BTCUSDT", "long", "native_partial_trailing_armed")

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", entryPrice, rule)
	if !ok {
		t.Fatal("expected equivalent native partial trailing tier to be treated as already armed")
	}
	if fake.trailingCalls != 0 {
		t.Fatalf("expected no new trailing call when equivalent tier exists, got %d", fake.trailingCalls)
	}
}

func TestApplyNativePartialTrailingDrawdownAppendsVisibleNewTierBeforeAnyCleanup(t *testing.T) {
	fake := &fakeProtectionTrader{
		openOrders: []tradertypes.OpenOrder{{
			Symbol:       "BTCUSDT",
			PositionSide: "LONG",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    105,
			CallbackRate: 0.001,
			Quantity:     0.2,
			Status:       "NEW",
		}},
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: make(map[string]string),
	}
	at.setProtectionState("BTCUSDT", "long", "native_partial_trailing_armed")

	rule := store.DrawdownTakeProfitRule{MinProfitPct: 6, MaxDrawdownPct: 2.5, CloseRatioPct: 50}
	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected native partial trailing drawdown to succeed")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected exactly one new trailing order placement, got %d", fake.trailingCalls)
	}
	if len(fake.openOrders) < 2 {
		t.Fatalf("expected new trailing order to be visible in open orders before any cleanup logic, got %d orders", len(fake.openOrders))
	}
	if fake.cancelTrailingCalls != 0 {
		t.Fatalf("expected no blanket trailing cleanup during append path, got %d", fake.cancelTrailingCalls)
	}
}

func TestApplyNativePartialTrailingDrawdownReplacesOldTierAfterNewTierVisible(t *testing.T) {
	entryPrice := 100.0
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 6, MaxDrawdownPct: 2.5, CloseRatioPct: 50}
	callback := calculateProfitBasedTrailingCallbackRatio(entryPrice, "long", rule.MinProfitPct, rule.MaxDrawdownPct)
	fake := &fakeProtectionTrader{
		openOrders: []tradertypes.OpenOrder{{
			OrderID:      "old-tier",
			Symbol:       "BTCUSDT",
			PositionSide: "LONG",
			Type:         "TRAILING_STOP_MARKET",
			StopPrice:    103,
			CallbackRate: callback,
			Quantity:     0.5,
			Status:       "NEW",
		}},
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"positionAmt": 1.0,
		}},
	}
	at := &AutoTrader{
		exchange:        "okx",
		trader:          fake,
		config:          AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}},
		protectionState: make(map[string]string),
	}
	at.setProtectionState("BTCUSDT", "long", "native_partial_trailing_armed")

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", entryPrice, rule)
	if !ok {
		t.Fatal("expected partial trailing replacement to succeed")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected one new tier placement, got %d", fake.trailingCalls)
	}
	if fake.cancelTrailingCalls == 0 {
		t.Fatal("expected old tier to be canceled after new tier became visible")
	}
	for _, order := range fake.openOrders {
		if order.OrderID == "old-tier" {
			t.Fatalf("expected old tier to be removed, still found %+v", order)
		}
	}
}

func TestGetDrawdownArmRulesReturnsOnlyHighestSatisfiedProfitStage(t *testing.T) {
	at := &AutoTrader{}
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 0.7, MaxDrawdownPct: 55, CloseRatioPct: 60},
		{MinProfitPct: 1.5, MaxDrawdownPct: 40, CloseRatioPct: 85},
		{MinProfitPct: 1.5, MaxDrawdownPct: 35, CloseRatioPct: 90},
		{MinProfitPct: 3.0, MaxDrawdownPct: 30, CloseRatioPct: 100},
	}
	matched := at.getDrawdownArmRules(1.8, rules)
	if len(matched) != 2 {
		t.Fatalf("expected 2 armed tiers from highest satisfied profit stage, got %d", len(matched))
	}
	if matched[0].CloseRatioPct != 85 || matched[1].CloseRatioPct != 90 {
		t.Fatalf("unexpected arm tiers: %+v", matched)
	}
}

func TestGetTriggeredDrawdownRulesReturnsAllTriggeredTiers(t *testing.T) {
	at := &AutoTrader{}
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 0.7, MaxDrawdownPct: 55, CloseRatioPct: 60},
		{MinProfitPct: 1.5, MaxDrawdownPct: 40, CloseRatioPct: 85},
		{MinProfitPct: 3.0, MaxDrawdownPct: 30, CloseRatioPct: 100},
	}
	matched := at.getTriggeredDrawdownRules(2.0, 60, rules)
	if len(matched) != 2 {
		t.Fatalf("expected 2 triggered tiers, got %d", len(matched))
	}
	if matched[0].CloseRatioPct != 60 || matched[1].CloseRatioPct != 85 {
		t.Fatalf("unexpected triggered tiers: %+v", matched)
	}
}

func TestGetActiveDrawdownRulesFiltersInvalidRules(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
	}
	at.config.StrategyConfig.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{
		Enabled: true,
		Rules: []store.DrawdownTakeProfitRule{
			{MinProfitPct: 0, MaxDrawdownPct: 20, CloseRatioPct: 50},
			{MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 150},
		},
	}

	rules := at.getActiveDrawdownRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 valid rule, got %d", len(rules))
	}
	if rules[0].CloseRatioPct != 100 {
		t.Fatalf("expected close ratio clamped to 100, got %.2f", rules[0].CloseRatioPct)
	}
}

func TestGetActiveBreakEvenConfig(t *testing.T) {
	at := &AutoTrader{}
	if cfg := at.getActiveBreakEvenConfig(); cfg != nil {
		t.Fatal("expected nil config when strategy config is missing")
	}

	at.config.StrategyConfig = &store.StrategyConfig{}
	at.config.StrategyConfig.Protection.BreakEvenStop = store.BreakEvenStopConfig{
		Enabled:      true,
		TriggerMode:  store.BreakEvenTriggerProfitPct,
		TriggerValue: 3,
		OffsetPct:    0.1,
	}

	cfg := at.getActiveBreakEvenConfig()
	if cfg == nil {
		t.Fatal("expected active break-even config")
	}
	if cfg.TriggerValue != 3 {
		t.Fatalf("unexpected trigger value: %.2f", cfg.TriggerValue)
	}
}

func TestCalculateBreakEvenStopPrice(t *testing.T) {
	if got := calculateBreakEvenStopPrice("long", 100, 0.1); got != 100.1 {
		t.Fatalf("expected long break-even stop 100.1, got %.4f", got)
	}
	if got := calculateBreakEvenStopPrice("short", 100, 0.1); got != 99.9 {
		t.Fatalf("expected short break-even stop 99.9, got %.4f", got)
	}
	if got := calculateBreakEvenStopPrice("flat", 100, 0.1); got != 0 {
		t.Fatalf("expected invalid side to return 0, got %.4f", got)
	}
}

func TestApplyBreakEvenStopUsesCancelAndSetStopLoss(t *testing.T) {
	fakeTrader := &fakeProtectionTrader{}
	at := &AutoTrader{
		exchange: "binance",
		trader:   fakeTrader,
	}

	cfg := store.BreakEvenStopConfig{
		Enabled:      true,
		TriggerMode:  store.BreakEvenTriggerProfitPct,
		TriggerValue: 3,
		OffsetPct:    0.1,
	}

	if err := at.applyBreakEvenStop("BTCUSDT", "long", 2, 100, 4.5, cfg); err != nil {
		t.Fatalf("expected break-even apply success, got %v", err)
	}
	if fakeTrader.cancelStopLossCalls != 0 {
		t.Fatalf("expected 0 cancel stop-loss call, got %d", fakeTrader.cancelStopLossCalls)
	}
	if fakeTrader.setStopLossCalls != 1 {
		t.Fatalf("expected 1 set stop-loss call, got %d", fakeTrader.setStopLossCalls)
	}
	if fakeTrader.lastSymbol != "BTCUSDT" || fakeTrader.lastPositionSide != "LONG" {
		t.Fatalf("unexpected stop-loss target: symbol=%s side=%s", fakeTrader.lastSymbol, fakeTrader.lastPositionSide)
	}
	if fakeTrader.lastQuantity != 2 {
		t.Fatalf("unexpected stop-loss quantity: %.4f", fakeTrader.lastQuantity)
	}
	if fakeTrader.lastStopPrice != 100.1 {
		t.Fatalf("expected break-even stop price 100.1, got %.4f", fakeTrader.lastStopPrice)
	}
}

func TestApplyBreakEvenStopSkipsBelowTrigger(t *testing.T) {
	fakeTrader := &fakeProtectionTrader{}
	at := &AutoTrader{
		exchange: "binance",
		trader:   fakeTrader,
	}

	cfg := store.BreakEvenStopConfig{
		Enabled:      true,
		TriggerMode:  store.BreakEvenTriggerProfitPct,
		TriggerValue: 3,
		OffsetPct:    0.1,
	}

	if err := at.applyBreakEvenStop("BTCUSDT", "long", 2, 100, 2.5, cfg); err != nil {
		t.Fatalf("expected nil error below trigger, got %v", err)
	}
	if fakeTrader.cancelStopLossCalls != 0 || fakeTrader.setStopLossCalls != 0 {
		t.Fatalf("expected no stop-loss operations below trigger, got cancel=%d set=%d", fakeTrader.cancelStopLossCalls, fakeTrader.setStopLossCalls)
	}
}

func TestApplyBreakEvenStopReturnsCancelError(t *testing.T) {
	fakeTrader := &fakeProtectionTrader{cancelErr: fmt.Errorf("cancel failed")}
	at := &AutoTrader{
		exchange: "binance",
		trader:   fakeTrader,
	}

	cfg := store.BreakEvenStopConfig{
		Enabled:      true,
		TriggerMode:  store.BreakEvenTriggerProfitPct,
		TriggerValue: 3,
		OffsetPct:    0.1,
	}

	err := at.applyBreakEvenStop("BTCUSDT", "short", 1, 100, 5, cfg)
	if err != nil {
		t.Fatalf("expected no cancel step / no cancel error, got %v", err)
	}
	if fakeTrader.setStopLossCalls != 1 {
		t.Fatalf("expected break-even stop to be placed directly, got %d calls", fakeTrader.setStopLossCalls)
	}
}
