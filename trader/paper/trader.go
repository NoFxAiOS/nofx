package paper

import (
	"fmt"
	"time"

	tradertypes "nofx/trader/types"
)

// Trader is a minimal simulated trader for paper-trading / replay scaffolding.
// It intentionally focuses on deterministic state transitions instead of exchange-grade matching.
type Trader struct {
	priceMap   map[string]float64
	positions  map[string]map[string]interface{}
	openOrders []tradertypes.OpenOrder
	closedPnL  []tradertypes.ClosedPnLRecord
	balance    map[string]interface{}
	orderSeq   int64
}

func NewTrader() *Trader {
	return &Trader{
		priceMap:   map[string]float64{},
		positions:  map[string]map[string]interface{}{},
		openOrders: []tradertypes.OpenOrder{},
		closedPnL:  []tradertypes.ClosedPnLRecord{},
		balance: map[string]interface{}{
			"totalWalletBalance": 10000.0,
			"availableBalance":   10000.0,
			"totalEquity":        10000.0,
		},
	}
}

func (t *Trader) SetPrice(symbol string, price float64) {
	t.priceMap[symbol] = price
}

func (t *Trader) GetBalance() (map[string]interface{}, error) { return t.balance, nil }

func (t *Trader) GetPositions() ([]map[string]interface{}, error) {
	positions := make([]map[string]interface{}, 0, len(t.positions))
	for _, pos := range t.positions {
		positions = append(positions, pos)
	}
	return positions, nil
}

func (t *Trader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	t.orderSeq++
	t.positions[symbol+":long"] = map[string]interface{}{
		"symbol":      symbol,
		"side":        "long",
		"positionAmt": quantity,
		"entryPrice":  price,
		"markPrice":   price,
		"leverage":    float64(leverage),
	}
	return map[string]interface{}{"orderId": t.orderSeq, "avgPrice": price}, nil
}

func (t *Trader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	t.orderSeq++
	t.positions[symbol+":short"] = map[string]interface{}{
		"symbol":      symbol,
		"side":        "short",
		"positionAmt": -quantity,
		"entryPrice":  price,
		"markPrice":   price,
		"leverage":    float64(leverage),
	}
	return map[string]interface{}{"orderId": t.orderSeq, "avgPrice": price}, nil
}

func (t *Trader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.closePosition(symbol, "long", quantity)
}

func (t *Trader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.closePosition(symbol, "short", quantity)
}

func (t *Trader) closePosition(symbol, side string, quantity float64) (map[string]interface{}, error) {
	key := symbol + ":" + side
	pos, ok := t.positions[key]
	if !ok {
		return nil, fmt.Errorf("position not found: %s %s", symbol, side)
	}
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	entryPrice, _ := pos["entryPrice"].(float64)
	positionAmt, _ := pos["positionAmt"].(float64)
	actualQty := quantity
	if actualQty <= 0 {
		actualQty = positionAmt
		if actualQty < 0 {
			actualQty = -actualQty
		}
	}
	t.orderSeq++
	delete(t.positions, key)
	t.closedPnL = append(t.closedPnL, tradertypes.ClosedPnLRecord{
		Symbol:      symbol,
		Side:        side,
		EntryPrice:  entryPrice,
		ExitPrice:   price,
		Quantity:    actualQty,
		Leverage:    int(pos["leverage"].(float64)),
		ExitTime:    time.Now(),
		CloseType:   "paper",
		RealizedPnL: 0,
	})
	return map[string]interface{}{"orderId": t.orderSeq, "avgPrice": price}, nil
}

func (t *Trader) SetLeverage(symbol string, leverage int) error         { return nil }
func (t *Trader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (t *Trader) GetMarketPrice(symbol string) (float64, error) {
	price, ok := t.priceMap[symbol]
	if !ok || price <= 0 {
		return 0, fmt.Errorf("paper trader price missing for %s", symbol)
	}
	return price, nil
}
func (t *Trader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	t.orderSeq++
	t.openOrders = append(t.openOrders, tradertypes.OpenOrder{OrderID: fmt.Sprintf("%d", t.orderSeq), Symbol: symbol, PositionSide: positionSide, Type: "STOP_MARKET", StopPrice: stopPrice, Quantity: quantity, Status: "NEW"})
	return nil
}
func (t *Trader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	t.orderSeq++
	t.openOrders = append(t.openOrders, tradertypes.OpenOrder{OrderID: fmt.Sprintf("%d", t.orderSeq), Symbol: symbol, PositionSide: positionSide, Type: "TAKE_PROFIT_MARKET", StopPrice: takeProfitPrice, Quantity: quantity, Status: "NEW"})
	return nil
}
func (t *Trader) CancelStopLossOrders(symbol string) error {
	return t.filterOrders(symbol, "STOP_MARKET")
}
func (t *Trader) CancelTakeProfitOrders(symbol string) error {
	return t.filterOrders(symbol, "TAKE_PROFIT_MARKET")
}
func (t *Trader) CancelAllOrders(symbol string) error  { return t.filterOrders(symbol, "") }
func (t *Trader) CancelStopOrders(symbol string) error { return t.filterOrders(symbol, "") }
func (t *Trader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.6f", quantity), nil
}
func (t *Trader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "FILLED", "avgPrice": t.priceMap[symbol], "executedQty": 1.0, "commission": 0.0}, nil
}
func (t *Trader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return t.closedPnL, nil
}
func (t *Trader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	if symbol == "" {
		return t.openOrders, nil
	}
	filtered := make([]tradertypes.OpenOrder, 0, len(t.openOrders))
	for _, order := range t.openOrders {
		if order.Symbol == symbol {
			filtered = append(filtered, order)
		}
	}
	return filtered, nil
}

func (t *Trader) filterOrders(symbol, kind string) error {
	filtered := make([]tradertypes.OpenOrder, 0, len(t.openOrders))
	for _, order := range t.openOrders {
		if order.Symbol != symbol {
			filtered = append(filtered, order)
			continue
		}
		if kind != "" && order.Type != kind {
			filtered = append(filtered, order)
		}
	}
	t.openOrders = filtered
	return nil
}
