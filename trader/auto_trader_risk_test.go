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
	closeLongCalls      int
	closeShortCalls     int
	closeLongQtys       []float64
	closeShortQtys      []float64
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
	f.closeLongCalls++
	f.closeLongQtys = append(f.closeLongQtys, quantity)
	return map[string]interface{}{"orderId": fmt.Sprintf("close-long-%d", f.closeLongCalls)}, nil
}
func (f *fakeProtectionTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	f.closeShortCalls++
	f.closeShortQtys = append(f.closeShortQtys, quantity)
	return map[string]interface{}{"orderId": fmt.Sprintf("close-short-%d", f.closeShortCalls)}, nil
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

func TestCheckPositionDrawdownSkipsDuplicateManagedPartialCloseForSameRule(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{ {
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

func TestCheckPositionDrawdownAllowsNextCloseAfterPositionFingerprintChanges(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{ {
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
	fake.positions = []map[string]interface{}{ {
		"symbol":      "ADAUSDT",
		"side":        "long",
		"entryPrice":  100.0,
		"markPrice":   101.0,
		"positionAmt": 51.0,
	}}
	at.checkPositionDrawdown()
	if fake.closeLongCalls != 2 {
		t.Fatalf("expected second close after quantity changed, got %d", fake.closeLongCalls)
	}
	if len(fake.closeLongQtys) != 2 || fake.closeLongQtys[1] != 35.7 {
		t.Fatalf("expected second close qty 35.7 after quantity changed, got %+v", fake.closeLongQtys)
	}
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
