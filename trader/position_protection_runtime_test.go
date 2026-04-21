package trader

import (
	"testing"
	"time"

	"nofx/store"
	tradertypes "nofx/trader/types"
)

type runtimeProtectionTestTrader struct{}

func (f *runtimeProtectionTestTrader) GetBalance() (map[string]interface{}, error) { return nil, nil }
func (f *runtimeProtectionTestTrader) GetPositions() ([]map[string]interface{}, error) {
	return []map[string]interface{}{{
		"symbol": "BTCUSDT",
		"side": "long",
		"markPrice": 104.0,
	}}, nil
}
func (f *runtimeProtectionTestTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *runtimeProtectionTestTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *runtimeProtectionTestTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}
func (f *runtimeProtectionTestTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}
func (f *runtimeProtectionTestTrader) SetLeverage(symbol string, leverage int) error { return nil }
func (f *runtimeProtectionTestTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (f *runtimeProtectionTestTrader) GetMarketPrice(symbol string) (float64, error) { return 0, nil }
func (f *runtimeProtectionTestTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}
func (f *runtimeProtectionTestTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}
func (f *runtimeProtectionTestTrader) CancelStopLossOrders(symbol string) error   { return nil }
func (f *runtimeProtectionTestTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (f *runtimeProtectionTestTrader) CancelAllOrders(symbol string) error        { return nil }
func (f *runtimeProtectionTestTrader) CancelStopOrders(symbol string) error       { return nil }
func (f *runtimeProtectionTestTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}
func (f *runtimeProtectionTestTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *runtimeProtectionTestTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return nil, nil
}
func (f *runtimeProtectionTestTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	return nil, nil
}

func TestBuildPositionProtectionRuntimeSurfacesLadderDegradation(t *testing.T) {
	at := &AutoTrader{
		exchange: "okx",
		trader:   &runtimeProtectionTestTrader{},
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
	}
	at.config.StrategyConfig.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeManual,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual},
		StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual},
		StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual},
		FallbackMaxLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 8},
		Rules: []store.LadderTPSLRule{{
			TakeProfitPct:           5,
			TakeProfitCloseRatioPct: 50,
			StopLossPct:             3,
			StopLossCloseRatioPct:   50,
		}, {
			TakeProfitPct:           10,
			TakeProfitCloseRatioPct: 50,
			StopLossPct:             6,
			StopLossCloseRatioPct:   50,
		}},
	}

	runtime := at.buildPositionProtectionRuntime("BTCUSDT", "long", 1, 100, []OpenOrder{
		{
			OrderID:        "1",
			Symbol:         "BTCUSDT",
			PositionSide:   "LONG",
			Type:           "STOP_MARKET",
			StopPrice:      97,
			Quantity:       1,
			ClientOrderID:  "full_sl_1",
			Status:         "NEW",
			ProtectionRole: "stop_loss",
		},
		{
			OrderID:        "2",
			Symbol:         "BTCUSDT",
			PositionSide:   "LONG",
			Type:           "STOP_MARKET",
			StopPrice:      92,
			Quantity:       1,
			ClientOrderID:  "fallback_maxloss_sl_1",
			Status:         "NEW",
			ProtectionRole: "stop_loss",
		},
		{
			OrderID:        "3",
			Symbol:         "BTCUSDT",
			PositionSide:   "LONG",
			Type:           "TAKE_PROFIT_MARKET",
			StopPrice:      105,
			Quantity:       0.5,
			ClientOrderID:  "ladder_tp_1",
			Status:         "NEW",
			ProtectionRole: "take_profit",
		},
	})

	if got := runtime["planned_ladder_stop_count"]; got != 2 {
		t.Fatalf("expected two planned ladder stops, got %#v", got)
	}
	if got := runtime["planned_ladder_take_profit_count"]; got != 2 {
		t.Fatalf("expected two planned ladder take-profits, got %#v", got)
	}
	if got := runtime["live_ladder_stop_count"]; got != 0 {
		t.Fatalf("expected no live ladder stops, got %#v", got)
	}
	if got := runtime["live_ladder_take_profit_count"]; got != 1 {
		t.Fatalf("expected one live ladder take-profit, got %#v", got)
	}
	if got := runtime["live_full_stop_count"]; got != 1 {
		t.Fatalf("expected one live full stop, got %#v", got)
	}
	if got := runtime["live_fallback_stop_count"]; got != 1 {
		t.Fatalf("expected one live fallback stop, got %#v", got)
	}
	if got := runtime["ladder_stop_degraded"]; got != true {
		t.Fatalf("expected stop ladder degradation, got %#v", got)
	}
	if got := runtime["ladder_stop_degraded_to_full"]; got != true {
		t.Fatalf("expected stop ladder degraded to full, got %#v", got)
	}
	if got := runtime["ladder_take_profit_degraded"]; got != true {
		t.Fatalf("expected take-profit ladder degradation, got %#v", got)
	}
	if got := runtime["fallback_order_detected"]; got != true {
		t.Fatalf("expected live fallback detection, got %#v", got)
	}
}
