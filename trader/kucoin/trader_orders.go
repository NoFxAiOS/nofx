package kucoin

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
	"time"
)

// OpenLong opens long position
func (t *KuCoinTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel old orders
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ Failed to set leverage: %v", err)
	}

	kcSymbol := t.convertSymbol(symbol)

	// Convert quantity to lots
	lots, err := t.quantityToLots(symbol, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate lots: %w", err)
	}

	body := map[string]interface{}{
		"clientOid":  fmt.Sprintf("nfx%d", time.Now().UnixNano()),
		"symbol":     kcSymbol,
		"side":       "buy",
		"type":       "market",
		"size":       lots,
		"leverage":   fmt.Sprintf("%d", leverage),
		"reduceOnly": false,
		"marginMode": "CROSS", // Use cross margin mode
	}

	data, err := t.doRequest("POST", kucoinOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	var result struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ KuCoin opened long position: %s, lots=%d, orderId=%s", symbol, lots, result.OrderId)

	// Query order to get fill price
	fillPrice := t.queryOrderFillPrice(result.OrderId)

	return map[string]interface{}{
		"orderId":   result.OrderId,
		"symbol":    symbol,
		"status":    "FILLED",
		"fillPrice": fillPrice,
	}, nil
}

// OpenShort opens short position
func (t *KuCoinTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel old orders
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ Failed to set leverage: %v", err)
	}

	kcSymbol := t.convertSymbol(symbol)

	// Convert quantity to lots
	lots, err := t.quantityToLots(symbol, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate lots: %w", err)
	}

	body := map[string]interface{}{
		"clientOid":  fmt.Sprintf("nfx%d", time.Now().UnixNano()),
		"symbol":     kcSymbol,
		"side":       "sell",
		"type":       "market",
		"size":       lots,
		"leverage":   fmt.Sprintf("%d", leverage),
		"reduceOnly": false,
		"marginMode": "CROSS", // Use cross margin mode
	}

	data, err := t.doRequest("POST", kucoinOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	var result struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ KuCoin opened short position: %s, lots=%d, orderId=%s", symbol, lots, result.OrderId)

	// Query order to get fill price
	fillPrice := t.queryOrderFillPrice(result.OrderId)

	return map[string]interface{}{
		"orderId":   result.OrderId,
		"symbol":    symbol,
		"status":    "FILLED",
		"fillPrice": fillPrice,
	}, nil
}

// queryOrderFillPrice queries order status and returns fill price
func (t *KuCoinTrader) queryOrderFillPrice(orderId string) float64 {
	// Wait a bit for order to fill
	time.Sleep(500 * time.Millisecond)

	path := fmt.Sprintf("%s/%s", kucoinOrderPath, orderId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		logger.Warnf("Failed to query order %s: %v", orderId, err)
		return 0
	}

	var order struct {
		DealAvgPrice float64 `json:"dealAvgPrice"`
		Status       string  `json:"status"`
		DealSize     int64   `json:"dealSize"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return 0
	}

	return order.DealAvgPrice
}

// CloseLong closes long position
func (t *KuCoinTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// Invalidate position cache and get fresh positions
	t.InvalidatePositionCache()
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Find actual position and get margin mode
	var actualQty float64
	var posFound bool
	var marginMode string = "CROSS" // Default to CROSS
	for _, pos := range positions {
		if pos["symbol"] == symbol && pos["side"] == "long" {
			actualQty = pos["positionAmt"].(float64)
			posFound = true
			// Get margin mode from position
			if mgnMode, ok := pos["mgnMode"].(string); ok {
				marginMode = strings.ToUpper(mgnMode)
			}
			break
		}
	}

	if !posFound || actualQty == 0 {
		return map[string]interface{}{
			"status":  "NO_POSITION",
			"message": fmt.Sprintf("No long position found for %s on KuCoin", symbol),
		}, nil
	}

	// Use actual quantity from exchange
	if quantity == 0 || quantity > actualQty {
		quantity = actualQty
	}

	kcSymbol := t.convertSymbol(symbol)

	// Convert quantity to lots
	lots, err := t.quantityToLots(symbol, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate lots: %w", err)
	}

	body := map[string]interface{}{
		"clientOid":  fmt.Sprintf("nfx%d", time.Now().UnixNano()),
		"symbol":     kcSymbol,
		"side":       "sell",
		"type":       "market",
		"size":       lots,
		"reduceOnly": true,
		"closeOrder": true,
		"marginMode": marginMode, // Use position's margin mode
	}

	data, err := t.doRequest("POST", kucoinOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	var result struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ KuCoin closed long position: %s", symbol)

	// Cancel pending orders
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": result.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort closes short position
func (t *KuCoinTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// Invalidate position cache and get fresh positions
	t.InvalidatePositionCache()
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Find actual position and get margin mode
	var actualQty float64
	var posFound bool
	var marginMode string = "CROSS" // Default to CROSS
	for _, pos := range positions {
		if pos["symbol"] == symbol && pos["side"] == "short" {
			actualQty = pos["positionAmt"].(float64)
			posFound = true
			// Get margin mode from position
			if mgnMode, ok := pos["mgnMode"].(string); ok {
				marginMode = strings.ToUpper(mgnMode)
			}
			break
		}
	}

	if !posFound || actualQty == 0 {
		return map[string]interface{}{
			"status":  "NO_POSITION",
			"message": fmt.Sprintf("No short position found for %s on KuCoin", symbol),
		}, nil
	}

	// Use actual quantity from exchange
	if quantity == 0 || quantity > actualQty {
		quantity = actualQty
	}

	kcSymbol := t.convertSymbol(symbol)

	// Convert quantity to lots
	lots, err := t.quantityToLots(symbol, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate lots: %w", err)
	}

	body := map[string]interface{}{
		"clientOid":  fmt.Sprintf("nfx%d", time.Now().UnixNano()),
		"symbol":     kcSymbol,
		"side":       "buy",
		"type":       "market",
		"size":       lots,
		"reduceOnly": true,
		"closeOrder": true,
		"marginMode": marginMode, // Use position's margin mode
	}

	data, err := t.doRequest("POST", kucoinOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	var result struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ KuCoin closed short position: %s", symbol)

	// Cancel pending orders
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": result.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice gets market price
func (t *KuCoinTrader) GetMarketPrice(symbol string) (float64, error) {
	kcSymbol := t.convertSymbol(symbol)
	path := fmt.Sprintf("%s?symbol=%s", kucoinTickerPath, kcSymbol)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	var ticker struct {
		Price string `json:"price"`
	}

	if err := json.Unmarshal(data, &ticker); err != nil {
		return 0, err
	}

	price, _ := strconv.ParseFloat(ticker.Price, 64)
	return price, nil
}

// SetStopLoss sets stop loss order
func (t *KuCoinTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	kcSymbol := t.convertSymbol(symbol)

	// Convert quantity to lots
	lots, err := t.quantityToLots(symbol, quantity)
	if err != nil {
		return fmt.Errorf("failed to calculate lots: %w", err)
	}

	// Determine side: close long = sell, close short = buy
	side := "sell"
	stop := "down" // Long position: stop loss triggers when price goes down
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		stop = "up" // Short position: stop loss triggers when price goes up
	}

	body := map[string]interface{}{
		"clientOid":     fmt.Sprintf("nfxsl%d", time.Now().UnixNano()),
		"symbol":        kcSymbol,
		"side":          side,
		"type":          "market",
		"size":          lots,
		"stop":          stop,
		"stopPriceType": "MP", // Mark Price
		"stopPrice":     fmt.Sprintf("%.8f", stopPrice),
		"reduceOnly":    true,
		"closeOrder":    true,
	}

	_, err = t.doRequest("POST", kucoinStopOrderPath, body)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("✓ Stop loss set: %.4f", stopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *KuCoinTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	kcSymbol := t.convertSymbol(symbol)

	// Convert quantity to lots
	lots, err := t.quantityToLots(symbol, quantity)
	if err != nil {
		return fmt.Errorf("failed to calculate lots: %w", err)
	}

	// Determine side: close long = sell, close short = buy
	side := "sell"
	stop := "up" // Long position: take profit triggers when price goes up
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		stop = "down" // Short position: take profit triggers when price goes down
	}

	body := map[string]interface{}{
		"clientOid":     fmt.Sprintf("nfxtp%d", time.Now().UnixNano()),
		"symbol":        kcSymbol,
		"side":          side,
		"type":          "market",
		"size":          lots,
		"stop":          stop,
		"stopPriceType": "MP", // Mark Price
		"stopPrice":     fmt.Sprintf("%.8f", takeProfitPrice),
		"reduceOnly":    true,
		"closeOrder":    true,
	}

	_, err = t.doRequest("POST", kucoinStopOrderPath, body)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	logger.Infof("✓ Take profit set: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop loss orders
func (t *KuCoinTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelStopOrdersByType(symbol, "sl")
}

// CancelTakeProfitOrders cancels take profit orders
func (t *KuCoinTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelStopOrdersByType(symbol, "tp")
}

// cancelStopOrdersByType cancels stop orders by type
func (t *KuCoinTrader) cancelStopOrdersByType(symbol string, orderType string) error {
	kcSymbol := t.convertSymbol(symbol)

	// Get pending stop orders
	path := fmt.Sprintf("%s?symbol=%s", kucoinStopOrderPath, kcSymbol)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return err
	}

	var response struct {
		Items []struct {
			Id        string `json:"id"`
			ClientOid string `json:"clientOid"`
			Stop      string `json:"stop"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		// Try alternate format (direct array)
		var items []struct {
			Id        string `json:"id"`
			ClientOid string `json:"clientOid"`
			Stop      string `json:"stop"`
		}
		if err := json.Unmarshal(data, &items); err != nil {
			return err
		}
		response.Items = items
	}

	// Cancel matching orders
	for _, order := range response.Items {
		// Check if order matches type based on clientOid prefix
		if orderType == "sl" && !strings.Contains(order.ClientOid, "sl") {
			continue
		}
		if orderType == "tp" && !strings.Contains(order.ClientOid, "tp") {
			continue
		}

		cancelPath := fmt.Sprintf("%s/%s", kucoinCancelStopPath, order.Id)
		_, err := t.doRequest("DELETE", cancelPath, nil)
		if err != nil {
			logger.Warnf("Failed to cancel stop order %s: %v", order.Id, err)
		}
	}

	return nil
}

// CancelStopOrders cancels all stop orders for symbol
func (t *KuCoinTrader) CancelStopOrders(symbol string) error {
	kcSymbol := t.convertSymbol(symbol)

	path := fmt.Sprintf("%s?symbol=%s", kucoinCancelStopPath, kcSymbol)
	_, err := t.doRequest("DELETE", path, nil)
	if err != nil {
		// Ignore if no orders to cancel
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "400100") {
			return nil
		}
		return err
	}

	logger.Infof("✓ Cancelled stop orders for %s", symbol)
	return nil
}

// CancelAllOrders cancels all pending orders for symbol
func (t *KuCoinTrader) CancelAllOrders(symbol string) error {
	kcSymbol := t.convertSymbol(symbol)

	// Cancel regular orders
	path := fmt.Sprintf("%s?symbol=%s", kucoinCancelOrderPath, kcSymbol)
	_, err := t.doRequest("DELETE", path, nil)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		logger.Warnf("Failed to cancel regular orders: %v", err)
	}

	// Cancel stop orders
	t.CancelStopOrders(symbol)

	return nil
}

// SetMarginMode sets margin mode
func (t *KuCoinTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// KuCoin sets margin mode per position, handled automatically
	logger.Infof("✓ KuCoin margin mode: %v (handled per position)", isCrossMargin)
	return nil
}

// SetLeverage sets leverage for a symbol
func (t *KuCoinTrader) SetLeverage(symbol string, leverage int) error {
	kcSymbol := t.convertSymbol(symbol)

	body := map[string]interface{}{
		"symbol":   kcSymbol,
		"leverage": fmt.Sprintf("%d", leverage),
	}

	_, err := t.doRequest("POST", kucoinLeveragePath, body)
	if err != nil {
		// Ignore if already at target leverage
		if strings.Contains(err.Error(), "same") || strings.Contains(err.Error(), "already") {
			logger.Infof("✓ %s leverage is already %dx", symbol, leverage)
			return nil
		}
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	logger.Infof("✓ %s leverage set to %dx", symbol, leverage)
	return nil
}

// FormatQuantity formats quantity to correct precision
func (t *KuCoinTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		return "", err
	}

	// Calculate lots
	lots := quantity / contract.Multiplier

	// Round to integer
	lotsInt := int64(math.Round(lots))

	return strconv.FormatInt(lotsInt, 10), nil
}

// GetOrderStatus gets order status
func (t *KuCoinTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("%s/%s", kucoinOrderPath, orderID)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var order struct {
		Id           string  `json:"id"`
		Symbol       string  `json:"symbol"`
		Status       string  `json:"status"`
		DealAvgPrice float64 `json:"dealAvgPrice"`
		DealSize     int64   `json:"dealSize"`
		Fee          float64 `json:"fee"`
		Side         string  `json:"side"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}

	// Convert status
	status := "NEW"
	if order.Status == "done" {
		status = "FILLED"
	} else if order.Status == "cancelled" || order.Status == "canceled" {
		status = "CANCELED"
	}

	return map[string]interface{}{
		"orderId":     order.Id,
		"symbol":      t.convertSymbolBack(order.Symbol),
		"status":      status,
		"avgPrice":    order.DealAvgPrice,
		"executedQty": order.DealSize,
		"commission":  order.Fee,
	}, nil
}

// GetClosedPnL gets closed position PnL records
func (t *KuCoinTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	// KuCoin closed positions API
	path := fmt.Sprintf("/api/v1/history-positions?status=CLOSE&limit=%d", limit)
	if !startTime.IsZero() {
		path += fmt.Sprintf("&from=%d", startTime.UnixMilli())
	}

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get closed PnL: %w", err)
	}

	var response struct {
		HasMore  bool `json:"hasMore"`
		DataList []struct {
			Symbol         string  `json:"symbol"`
			OpenPrice      float64 `json:"avgEntryPrice"`
			ClosePrice     float64 `json:"avgClosePrice"`
			Qty            int64   `json:"qty"`
			RealisedPnl    float64 `json:"realisedGrossCost"`
			CloseTime      int64   `json:"closeTime"`
			OpenTime       int64   `json:"openTime"`
			PositionId     string  `json:"id"`
			CloseType      string  `json:"type"`
			Leverage       int     `json:"leverage"`
			SettleCurrency string  `json:"settleCurrency"`
		} `json:"dataList"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse closed PnL: %w", err)
	}

	var records []types.ClosedPnLRecord
	for _, item := range response.DataList {
		side := "long"
		qty := item.Qty
		if qty < 0 {
			side = "short"
			qty = -qty
		}

		// Map close type
		closeType := "unknown"
		switch strings.ToUpper(item.CloseType) {
		case "CLOSE", "MANUAL":
			closeType = "manual"
		case "STOP", "STOPLOSS":
			closeType = "stop_loss"
		case "TAKEPROFIT", "TP":
			closeType = "take_profit"
		case "LIQUIDATION", "LIQ", "ADL":
			closeType = "liquidation"
		}

		records = append(records, types.ClosedPnLRecord{
			Symbol:      t.convertSymbolBack(item.Symbol),
			Side:        side,
			EntryPrice:  item.OpenPrice,
			ExitPrice:   item.ClosePrice,
			Quantity:    float64(qty),
			RealizedPnL: item.RealisedPnl,
			Leverage:    item.Leverage,
			EntryTime:   time.UnixMilli(item.OpenTime),
			ExitTime:    time.UnixMilli(item.CloseTime),
			ExchangeID:  item.PositionId,
			CloseType:   closeType,
		})
	}

	return records, nil
}

// GetOpenOrders gets open/pending orders
func (t *KuCoinTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	kcSymbol := t.convertSymbol(symbol)

	// Get regular orders
	path := fmt.Sprintf("%s?symbol=%s&status=active", kucoinOrderPath, kcSymbol)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var response struct {
		Items []struct {
			Id       string `json:"id"`
			Symbol   string `json:"symbol"`
			Side     string `json:"side"`
			Type     string `json:"type"`
			Price    string `json:"price"`
			Size     int64  `json:"size"`
			StopType string `json:"stopType"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		// Try alternate format
		var items []struct {
			Id       string `json:"id"`
			Symbol   string `json:"symbol"`
			Side     string `json:"side"`
			Type     string `json:"type"`
			Price    string `json:"price"`
			Size     int64  `json:"size"`
			StopType string `json:"stopType"`
		}
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
		response.Items = items
	}

	var orders []types.OpenOrder
	for _, item := range response.Items {
		// Determine position side based on order side
		positionSide := "LONG"
		if item.Side == "sell" {
			positionSide = "SHORT"
		}

		price, _ := strconv.ParseFloat(item.Price, 64)

		orders = append(orders, types.OpenOrder{
			OrderID:      item.Id,
			Symbol:       t.convertSymbolBack(item.Symbol),
			Side:         strings.ToUpper(item.Side),
			PositionSide: positionSide,
			Type:         strings.ToUpper(item.Type),
			Price:        price,
			Quantity:     float64(item.Size),
			Status:       "NEW",
		})
	}

	// Get stop orders
	stopPath := fmt.Sprintf("%s?symbol=%s", kucoinStopOrderPath, kcSymbol)
	stopData, err := t.doRequest("GET", stopPath, nil)
	if err == nil {
		var stopResponse struct {
			Items []struct {
				Id        string `json:"id"`
				Symbol    string `json:"symbol"`
				Side      string `json:"side"`
				StopPrice string `json:"stopPrice"`
				Size      int64  `json:"size"`
			} `json:"items"`
		}

		if json.Unmarshal(stopData, &stopResponse) == nil {
			for _, item := range stopResponse.Items {
				positionSide := "LONG"
				if item.Side == "sell" {
					positionSide = "SHORT"
				}

				stopPrice, _ := strconv.ParseFloat(item.StopPrice, 64)

				orders = append(orders, types.OpenOrder{
					OrderID:      item.Id,
					Symbol:       t.convertSymbolBack(item.Symbol),
					Side:         strings.ToUpper(item.Side),
					PositionSide: positionSide,
					Type:         "STOP_MARKET",
					StopPrice:    stopPrice,
					Quantity:     float64(item.Size),
					Status:       "NEW",
				})
			}
		}
	}

	return orders, nil
}
