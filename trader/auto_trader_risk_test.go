package trader

import (
	"fmt"
	"nofx/store"
	tradertypes "nofx/trader/types"
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
	trailingSymbol      string
	trailingSide        string
	trailingActivation  float64
	trailingCallback    float64
}

func (f *fakeProtectionTrader) GetBalance() (map[string]interface{}, error) { return nil, nil }
func (f *fakeProtectionTrader) GetPositions() ([]map[string]interface{}, error) { return nil, nil }
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
func (f *fakeProtectionTrader) SetLeverage(symbol string, leverage int) error { return nil }
func (f *fakeProtectionTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (f *fakeProtectionTrader) GetMarketPrice(symbol string) (float64, error) { return 0, nil }
func (f *fakeProtectionTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	f.setStopLossCalls++
	f.lastSymbol = symbol
	f.lastPositionSide = positionSide
	f.lastQuantity = quantity
	f.lastStopPrice = stopPrice
	return f.setStopLossErr
}
func (f *fakeProtectionTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}
func (f *fakeProtectionTrader) CancelStopLossOrders(symbol string) error {
	f.cancelStopLossCalls++
	return f.cancelErr
}
func (f *fakeProtectionTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (f *fakeProtectionTrader) CancelAllOrders(symbol string) error { return nil }
func (f *fakeProtectionTrader) CancelStopOrders(symbol string) error { return nil }
func (f *fakeProtectionTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}
func (f *fakeProtectionTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) { return nil, nil }
func (f *fakeProtectionTrader) SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64) error {
	f.trailingCalls++
	f.trailingSymbol = symbol
	f.trailingSide = positionSide
	f.trailingActivation = activationPrice
	f.trailingCallback = callbackRate
	return nil
}
func (f *fakeProtectionTrader) CancelTrailingStopOrders(symbol string) error { return nil }

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
	if fake.trailingCallback != 2 {
		t.Fatalf("expected callback rate 2, got %.4f", fake.trailingCallback)
	}
	if at.getProtectionState("BTCUSDT", "long") != "native_trailing_armed" {
		t.Fatalf("expected protection state native_trailing_armed, got %q", at.getProtectionState("BTCUSDT", "long"))
	}
}

func TestMatchDrawdownRule(t *testing.T) {
	at := &AutoTrader{}
	 rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100},
		{MinProfitPct: 10, MaxDrawdownPct: 20, CloseRatioPct: 50},
	}

	matched := at.matchDrawdownRule(12, 25, rules)
	if matched == nil {
		t.Fatal("expected a matched rule")
	}
	if matched.MinProfitPct != 10 || matched.CloseRatioPct != 50 {
		t.Fatalf("expected higher-priority rule, got %+v", *matched)
	}

	if matched := at.matchDrawdownRule(4, 60, rules); matched != nil {
		t.Fatalf("expected no rule below min profit, got %+v", *matched)
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
	if fakeTrader.cancelStopLossCalls != 1 {
		t.Fatalf("expected 1 cancel stop-loss call, got %d", fakeTrader.cancelStopLossCalls)
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
	if err == nil {
		t.Fatal("expected cancel error")
	}
	if fakeTrader.setStopLossCalls != 0 {
		t.Fatalf("expected stop-loss not to be reset after cancel failure, got %d calls", fakeTrader.setStopLossCalls)
	}
}
