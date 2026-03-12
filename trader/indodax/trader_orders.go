package indodax

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
)

// OpenLong opens a spot buy order
func (t *IndodaxTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	t.clearCache()

	pair := t.convertSymbol(symbol)
	coin := t.getCoinFromSymbol(symbol)

	// Get market price to calculate IDR amount
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	params := url.Values{}
	params.Set("method", "trade")
	params.Set("pair", pair)
	params.Set("type", "buy")
	params.Set("price", strconv.FormatFloat(price, 'f', 0, 64))
	params.Set(coin, strconv.FormatFloat(quantity, 'f', 8, 64))
	params.Set("order_type", "limit")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to place buy order: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse trade response: %w", err)
	}

	logger.Infof("[Indodax] Buy order placed: %s qty=%.8f price=%.0f", symbol, quantity, price)

	return map[string]interface{}{
		"orderId": result["order_id"],
		"symbol":  symbol,
		"side":    "BUY",
		"price":   price,
		"qty":     quantity,
		"status":  "NEW",
	}, nil
}

// OpenShort is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling is not supported on Indodax (spot-only exchange)")
}

// CloseLong closes a spot position by selling
func (t *IndodaxTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	t.clearCache()

	pair := t.convertSymbol(symbol)
	coin := t.getCoinFromSymbol(symbol)

	// If quantity is 0, sell all available balance
	if quantity <= 0 {
		balance, err := t.GetBalance()
		if err != nil {
			return nil, fmt.Errorf("failed to get balance for close all: %w", err)
		}
		available := parseFloat(balance["balance_"+coin])
		if available <= 0 {
			return nil, fmt.Errorf("no %s balance to sell", coin)
		}
		quantity = available
	}

	// Get market price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	params := url.Values{}
	params.Set("method", "trade")
	params.Set("pair", pair)
	params.Set("type", "sell")
	params.Set("price", strconv.FormatFloat(price, 'f', 0, 64))
	params.Set(coin, strconv.FormatFloat(quantity, 'f', 8, 64))
	params.Set("order_type", "limit")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to place sell order: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse trade response: %w", err)
	}

	logger.Infof("[Indodax] Sell order placed: %s qty=%.8f price=%.0f", symbol, quantity, price)

	return map[string]interface{}{
		"orderId": result["order_id"],
		"symbol":  symbol,
		"side":    "SELL",
		"price":   price,
		"qty":     quantity,
		"status":  "NEW",
	}, nil
}

// CloseShort is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling is not supported on Indodax (spot-only exchange)")
}

// SetLeverage is a no-op for Indodax (spot-only, no leverage)
func (t *IndodaxTrader) SetLeverage(symbol string, leverage int) error {
	logger.Infof("[Indodax] SetLeverage ignored (spot-only exchange, no leverage support)")
	return nil
}

// SetMarginMode is a no-op for Indodax (spot-only, no margin)
func (t *IndodaxTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	logger.Infof("[Indodax] SetMarginMode ignored (spot-only exchange, no margin support)")
	return nil
}

// GetMarketPrice gets the current market price for a symbol
func (t *IndodaxTrader) GetMarketPrice(symbol string) (float64, error) {
	pairID := strings.ToLower(strings.ReplaceAll(t.convertSymbol(symbol), "_", ""))

	data, err := t.doPublicRequest("/ticker/" + pairID)
	if err != nil {
		return 0, fmt.Errorf("failed to get ticker: %w", err)
	}

	var tickerResp IndodaxTickerResponse
	if err := json.Unmarshal(data, &tickerResp); err != nil {
		return 0, fmt.Errorf("failed to parse ticker: %w", err)
	}

	price, err := strconv.ParseFloat(tickerResp.Ticker.Last, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price '%s': %w", tickerResp.Ticker.Last, err)
	}

	return price, nil
}

// SetStopLoss is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return fmt.Errorf("stop-loss orders are not supported on Indodax (spot-only exchange)")
}

// SetTakeProfit is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return fmt.Errorf("take-profit orders are not supported on Indodax (spot-only exchange)")
}

// CancelStopLossOrders is a no-op for Indodax
func (t *IndodaxTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

// CancelTakeProfitOrders is a no-op for Indodax
func (t *IndodaxTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

// CancelAllOrders cancels all open orders for a given symbol
func (t *IndodaxTrader) CancelAllOrders(symbol string) error {
	t.clearCache()

	pair := t.convertSymbol(symbol)

	// First get open orders
	params := url.Values{}
	params.Set("method", "openOrders")
	params.Set("pair", pair)

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var result struct {
		Orders []struct {
			OrderID   json.Number `json:"order_id"`
			Type      string      `json:"type"`
			OrderType string      `json:"order_type"`
		} `json:"orders"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to parse open orders: %w", err)
	}

	// Cancel each order
	for _, order := range result.Orders {
		cancelParams := url.Values{}
		cancelParams.Set("method", "cancelOrder")
		cancelParams.Set("pair", pair)
		cancelParams.Set("order_id", order.OrderID.String())
		cancelParams.Set("type", order.Type)

		if _, err := t.doPrivateRequest(cancelParams); err != nil {
			logger.Warnf("[Indodax] Failed to cancel order %s: %v", order.OrderID, err)
		} else {
			logger.Infof("[Indodax] Cancelled order: %s", order.OrderID)
		}
	}

	return nil
}

// CancelStopOrders is a no-op for Indodax (no stop orders)
func (t *IndodaxTrader) CancelStopOrders(symbol string) error {
	return nil
}

// FormatQuantity formats quantity to correct precision for Indodax
func (t *IndodaxTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	pair, err := t.getPair(symbol)
	if err != nil {
		// Default: 8 decimal places
		return strconv.FormatFloat(quantity, 'f', 8, 64), nil
	}

	precision := pair.PriceRound
	if precision <= 0 {
		precision = 8
	}

	// Round down to avoid exceeding balance
	factor := math.Pow(10, float64(precision))
	rounded := math.Floor(quantity*factor) / factor

	return strconv.FormatFloat(rounded, 'f', precision, 64), nil
}

// GetOrderStatus gets the status of a specific order
func (t *IndodaxTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	pair := t.convertSymbol(symbol)

	params := url.Values{}
	params.Set("method", "getOrder")
	params.Set("pair", pair)
	params.Set("order_id", orderID)

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var result struct {
		Order struct {
			OrderID       string `json:"order_id"`
			Price         string `json:"price"`
			Type          string `json:"type"`
			Status        string `json:"status"`
			SubmitTime    string `json:"submit_time"`
			FinishTime    string `json:"finish_time"`
			ClientOrderID string `json:"client_order_id"`
		} `json:"order"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order: %w", err)
	}

	// Map Indodax status to standard status
	status := "NEW"
	switch result.Order.Status {
	case "filled":
		status = "FILLED"
	case "cancelled":
		status = "CANCELED"
	case "open":
		status = "NEW"
	}

	price, _ := strconv.ParseFloat(result.Order.Price, 64)

	return map[string]interface{}{
		"status":      status,
		"avgPrice":    price,
		"executedQty": 0.0, // Indodax doesn't return executed qty in getOrder
		"commission":  0.0,
		"orderId":     result.Order.OrderID,
	}, nil
}

// GetOpenOrders gets open/pending orders
func (t *IndodaxTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	pair := t.convertSymbol(symbol)

	params := url.Values{}
	params.Set("method", "openOrders")
	if pair != "" {
		params.Set("pair", pair)
	}

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var result struct {
		Orders []struct {
			OrderID       json.Number `json:"order_id"`
			ClientOrderID string      `json:"client_order_id"`
			SubmitTime    string      `json:"submit_time"`
			Price         string      `json:"price"`
			Type          string      `json:"type"`
			OrderType     string      `json:"order_type"`
		} `json:"orders"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse open orders: %w", err)
	}

	var orders []types.OpenOrder
	for _, order := range result.Orders {
		price, _ := strconv.ParseFloat(order.Price, 64)

		side := "BUY"
		if order.Type == "sell" {
			side = "SELL"
		}

		orders = append(orders, types.OpenOrder{
			OrderID:      order.OrderID.String(),
			Symbol:       t.convertSymbolBack(pair),
			Side:         side,
			PositionSide: "LONG",
			Type:         "LIMIT",
			Price:        price,
			Status:       "NEW",
		})
	}

	return orders, nil
}
