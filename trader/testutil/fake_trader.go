package testutil

import (
	"time"

	tradertypes "nofx/trader/types"
)

// FakeTrader is a reusable in-memory trader harness for protection / replay / paper-trading tests.
// It does not try to simulate full exchange behavior; it only captures enough execution state
// to support deterministic lifecycle tests around open/close/protection/order verification.
type FakeTrader struct {
	Balance             map[string]interface{}
	Positions           []map[string]interface{}
	OpenOrders          []tradertypes.OpenOrder
	ClosedPnL           []tradertypes.ClosedPnLRecord
	SetStopLossErr      error
	SetTakeProfitErr    error
	CancelStopLossErr   error
	CancelTakeProfitErr error
	CancelAllOrdersErr  error
	CancelStopOrdersErr error
	GetOpenOrdersErr    error
	GetPositionsErr     error
	GetBalanceErr       error
	CloseLongErr        error
	CloseShortErr       error
	LastCloseLongQty    float64
	LastCloseShortQty   float64
}

func NewFakeTrader() *FakeTrader {
	return &FakeTrader{
		Balance: map[string]interface{}{
			"totalWalletBalance": 1000.0,
			"availableBalance":   1000.0,
			"totalEquity":        1000.0,
		},
		Positions:  []map[string]interface{}{},
		OpenOrders: []tradertypes.OpenOrder{},
		ClosedPnL:  []tradertypes.ClosedPnLRecord{},
	}
}

func (f *FakeTrader) GetBalance() (map[string]interface{}, error) {
	if f.GetBalanceErr != nil {
		return nil, f.GetBalanceErr
	}
	return f.Balance, nil
}

func (f *FakeTrader) GetPositions() ([]map[string]interface{}, error) {
	if f.GetPositionsErr != nil {
		return nil, f.GetPositionsErr
	}
	return f.Positions, nil
}

func (f *FakeTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": int64(1), "symbol": symbol, "quantity": quantity, "leverage": leverage}, nil
}

func (f *FakeTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": int64(2), "symbol": symbol, "quantity": quantity, "leverage": leverage}, nil
}

func (f *FakeTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if f.CloseLongErr != nil {
		return nil, f.CloseLongErr
	}
	f.LastCloseLongQty = quantity
	return map[string]interface{}{"orderId": int64(3), "symbol": symbol, "quantity": quantity}, nil
}

func (f *FakeTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if f.CloseShortErr != nil {
		return nil, f.CloseShortErr
	}
	f.LastCloseShortQty = quantity
	return map[string]interface{}{"orderId": int64(4), "symbol": symbol, "quantity": quantity}, nil
}

func (f *FakeTrader) SetLeverage(symbol string, leverage int) error         { return nil }
func (f *FakeTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (f *FakeTrader) GetMarketPrice(symbol string) (float64, error)         { return 100, nil }

func (f *FakeTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if f.SetStopLossErr != nil {
		return f.SetStopLossErr
	}
	f.OpenOrders = append(f.OpenOrders, tradertypes.OpenOrder{
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "STOP_MARKET",
		StopPrice:    stopPrice,
		Quantity:     quantity,
		Status:       "NEW",
	})
	return nil
}

func (f *FakeTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if f.SetTakeProfitErr != nil {
		return f.SetTakeProfitErr
	}
	f.OpenOrders = append(f.OpenOrders, tradertypes.OpenOrder{
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "TAKE_PROFIT_MARKET",
		StopPrice:    takeProfitPrice,
		Quantity:     quantity,
		Status:       "NEW",
	})
	return nil
}

func (f *FakeTrader) CancelStopLossOrders(symbol string) error {
	if f.CancelStopLossErr != nil {
		return f.CancelStopLossErr
	}
	filtered := make([]tradertypes.OpenOrder, 0, len(f.OpenOrders))
	for _, order := range f.OpenOrders {
		if order.Symbol == symbol && order.Type == "STOP_MARKET" {
			continue
		}
		filtered = append(filtered, order)
	}
	f.OpenOrders = filtered
	return nil
}

func (f *FakeTrader) CancelTakeProfitOrders(symbol string) error {
	if f.CancelTakeProfitErr != nil {
		return f.CancelTakeProfitErr
	}
	filtered := make([]tradertypes.OpenOrder, 0, len(f.OpenOrders))
	for _, order := range f.OpenOrders {
		if order.Symbol == symbol && order.Type == "TAKE_PROFIT_MARKET" {
			continue
		}
		filtered = append(filtered, order)
	}
	f.OpenOrders = filtered
	return nil
}

func (f *FakeTrader) CancelAllOrders(symbol string) error {
	if f.CancelAllOrdersErr != nil {
		return f.CancelAllOrdersErr
	}
	filtered := make([]tradertypes.OpenOrder, 0, len(f.OpenOrders))
	for _, order := range f.OpenOrders {
		if order.Symbol == symbol {
			continue
		}
		filtered = append(filtered, order)
	}
	f.OpenOrders = filtered
	return nil
}

func (f *FakeTrader) CancelStopOrders(symbol string) error {
	if f.CancelStopOrdersErr != nil {
		return f.CancelStopOrdersErr
	}
	return f.CancelAllOrders(symbol)
}

func (f *FakeTrader) FormatQuantity(symbol string, quantity float64) (string, error) { return "", nil }
func (f *FakeTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "FILLED", "avgPrice": 100.0, "executedQty": 1.0, "commission": 0.0}, nil
}
func (f *FakeTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return f.ClosedPnL, nil
}
func (f *FakeTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	if f.GetOpenOrdersErr != nil {
		return nil, f.GetOpenOrdersErr
	}
	if symbol == "" {
		return f.OpenOrders, nil
	}
	filtered := make([]tradertypes.OpenOrder, 0, len(f.OpenOrders))
	for _, order := range f.OpenOrders {
		if order.Symbol == symbol {
			filtered = append(filtered, order)
		}
	}
	return filtered, nil
}
