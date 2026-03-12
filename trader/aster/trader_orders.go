package aster

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
)

// OpenLong Open long position
func (t *AsterTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all pending orders before opening position to prevent position stacking from residual orders
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders (continuing to open position): %v", err)
	}

	// Set leverage first (non-fatal if position already exists)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		// Error -2030: Cannot adjust leverage when position exists
		// This is expected when adding to an existing position, continue with current leverage
		if strings.Contains(err.Error(), "-2030") {
			logger.Infof("  ⚠ Cannot change leverage (position exists), using current leverage: %v", err)
		} else {
			return nil, fmt.Errorf("failed to set leverage: %w", err)
		}
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Use limit order to simulate market order (price set slightly higher to ensure execution)
	limitPrice := price * 1.01

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	logger.Infof("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "BUY",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// OpenShort Open short position
func (t *AsterTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all pending orders before opening position to prevent position stacking from residual orders
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders (continuing to open position): %v", err)
	}

	// Set leverage first (non-fatal if position already exists)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		// Error -2030: Cannot adjust leverage when position exists
		// This is expected when adding to an existing position, continue with current leverage
		if strings.Contains(err.Error(), "-2030") {
			logger.Infof("  ⚠ Cannot change leverage (position exists), using current leverage: %v", err)
		} else {
			return nil, fmt.Errorf("failed to set leverage: %w", err)
		}
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Use limit order to simulate market order (price set slightly lower to ensure execution)
	limitPrice := price * 0.99

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	logger.Infof("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "SELL",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// CloseLong Close long position
func (t *AsterTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
		logger.Infof("  📊 Retrieved long position quantity: %.8f", quantity)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	limitPrice := price * 0.99

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	logger.Infof("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "SELL",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	logger.Infof("✓ Successfully closed long position: %s quantity: %s", symbol, qtyStr)

	// Cancel all pending orders for this symbol after closing position (stop-loss/take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	return result, nil
}

// CloseShort Close short position
func (t *AsterTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				// Aster's GetPositions has already converted short position quantity to positive, use directly
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
		logger.Infof("  📊 Retrieved short position quantity: %.8f", quantity)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	limitPrice := price * 1.01

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	logger.Infof("  📏 Precision handling: price %.8f -> %s (precision=%d), quantity %.8f -> %s (precision=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         "BUY",
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	logger.Infof("✓ Successfully closed short position: %s quantity: %s", symbol, qtyStr)

	// Cancel all pending orders for this symbol after closing position (stop-loss/take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	return result, nil
}

// SetStopLoss Set stop loss
func (t *AsterTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := "SELL"
	if positionSide == "SHORT" {
		side = "BUY"
	}

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, stopPrice)
	if err != nil {
		return err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "STOP_MARKET",
		"side":         side,
		"stopPrice":    priceStr,
		"quantity":     qtyStr,
		"timeInForce":  "GTC",
	}

	_, err = t.request("POST", "/fapi/v3/order", params)
	return err
}

// SetTakeProfit Set take profit
func (t *AsterTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := "SELL"
	if positionSide == "SHORT" {
		side = "BUY"
	}

	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(symbol, takeProfitPrice)
	if err != nil {
		return err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// Get precision information
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return err
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	params := map[string]interface{}{
		"symbol":       symbol,
		"positionSide": "BOTH",
		"type":         "TAKE_PROFIT_MARKET",
		"side":         side,
		"stopPrice":    priceStr,
		"quantity":     qtyStr,
		"timeInForce":  "GTC",
	}

	_, err = t.request("POST", "/fapi/v3/order", params)
	return err
}

// CancelStopLossOrders Cancel stop-loss orders only (does not affect take-profit orders)
func (t *AsterTrader) CancelStopLossOrders(symbol string) error {
	// Get all open orders for this symbol
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return fmt.Errorf("failed to parse order data: %w", err)
	}

	// Filter and cancel stop-loss orders (cancel all directions including LONG and SHORT)
	canceledCount := 0
	var cancelErrors []error
	for _, order := range orders {
		orderType, _ := order["type"].(string)

		// Only cancel stop-loss orders (don't cancel take-profit orders)
		if orderType == "STOP_MARKET" || orderType == "STOP" {
			orderID, _ := order["orderId"].(float64)
			positionSide, _ := order["positionSide"].(string)
			cancelParams := map[string]interface{}{
				"symbol":  symbol,
				"orderId": int64(orderID),
			}

			_, err := t.request("DELETE", "/fapi/v1/order", cancelParams)
			if err != nil {
				errMsg := fmt.Sprintf("order ID %d: %v", int64(orderID), err)
				cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
				logger.Infof("  ⚠ Failed to cancel stop-loss order: %s", errMsg)
				continue
			}

			canceledCount++
			logger.Infof("  ✓ Canceled stop-loss order (order ID: %d, type: %s, direction: %s)", int64(orderID), orderType, positionSide)
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s no stop-loss orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d stop-loss order(s) for %s", canceledCount, symbol)
	}

	// Return error if all cancellations failed
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel stop-loss orders: %v", cancelErrors)
	}

	return nil
}

// CancelTakeProfitOrders Cancel take-profit orders only (does not affect stop-loss orders)
func (t *AsterTrader) CancelTakeProfitOrders(symbol string) error {
	// Get all open orders for this symbol
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return fmt.Errorf("failed to parse order data: %w", err)
	}

	// Filter and cancel take-profit orders (cancel all directions including LONG and SHORT)
	canceledCount := 0
	var cancelErrors []error
	for _, order := range orders {
		orderType, _ := order["type"].(string)

		// Only cancel take-profit orders (don't cancel stop-loss orders)
		if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
			orderID, _ := order["orderId"].(float64)
			positionSide, _ := order["positionSide"].(string)
			cancelParams := map[string]interface{}{
				"symbol":  symbol,
				"orderId": int64(orderID),
			}

			_, err := t.request("DELETE", "/fapi/v1/order", cancelParams)
			if err != nil {
				errMsg := fmt.Sprintf("order ID %d: %v", int64(orderID), err)
				cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
				logger.Infof("  ⚠ Failed to cancel take-profit order: %s", errMsg)
				continue
			}

			canceledCount++
			logger.Infof("  ✓ Canceled take-profit order (order ID: %d, type: %s, direction: %s)", int64(orderID), orderType, positionSide)
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s no take-profit orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d take-profit order(s) for %s", canceledCount, symbol)
	}

	// Return error if all cancellations failed
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel take-profit orders: %v", cancelErrors)
	}

	return nil
}

// CancelAllOrders Cancel all orders
func (t *AsterTrader) CancelAllOrders(symbol string) error {
	params := map[string]interface{}{
		"symbol": symbol,
	}

	_, err := t.request("DELETE", "/fapi/v3/allOpenOrders", params)
	return err
}

// CancelStopOrders Cancel take-profit/stop-loss orders for this symbol (used to adjust TP/SL positions)
func (t *AsterTrader) CancelStopOrders(symbol string) error {
	// Get all open orders for this symbol
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return fmt.Errorf("failed to parse order data: %w", err)
	}

	// Filter and cancel take-profit/stop-loss orders
	canceledCount := 0
	for _, order := range orders {
		orderType, _ := order["type"].(string)

		// Only cancel stop-loss and take-profit orders
		if orderType == "STOP_MARKET" ||
			orderType == "TAKE_PROFIT_MARKET" ||
			orderType == "STOP" ||
			orderType == "TAKE_PROFIT" {

			orderID, _ := order["orderId"].(float64)
			cancelParams := map[string]interface{}{
				"symbol":  symbol,
				"orderId": int64(orderID),
			}

			_, err := t.request("DELETE", "/fapi/v3/order", cancelParams)
			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order %d: %v", int64(orderID), err)
				continue
			}

			canceledCount++
			logger.Infof("  ✓ Canceled take-profit/stop-loss order for %s (order ID: %d, type: %s)",
				symbol, int64(orderID), orderType)
		}
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ %s no take-profit/stop-loss orders to cancel", symbol)
	} else {
		logger.Infof("  ✓ Canceled %d take-profit/stop-loss order(s) for %s", canceledCount, symbol)
	}

	return nil
}

// FormatQuantity Format quantity (implements Trader interface)
func (t *AsterTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	formatted, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", formatted), nil
}

// GetOrderStatus Get order status
func (t *AsterTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"symbol":  symbol,
		"orderId": orderID,
	}

	body, err := t.request("GET", "/fapi/v3/order", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Standardize return fields
	response := map[string]interface{}{
		"orderId":    result["orderId"],
		"symbol":     result["symbol"],
		"status":     result["status"],
		"side":       result["side"],
		"type":       result["type"],
		"time":       result["time"],
		"updateTime": result["updateTime"],
		"commission": 0.0, // Aster may require separate query
	}

	// Parse numeric fields
	if avgPrice, ok := result["avgPrice"].(string); ok {
		if v, err := strconv.ParseFloat(avgPrice, 64); err == nil {
			response["avgPrice"] = v
		}
	} else if avgPrice, ok := result["avgPrice"].(float64); ok {
		response["avgPrice"] = avgPrice
	}

	if executedQty, ok := result["executedQty"].(string); ok {
		if v, err := strconv.ParseFloat(executedQty, 64); err == nil {
			response["executedQty"] = v
		}
	} else if executedQty, ok := result["executedQty"].(float64); ok {
		response["executedQty"] = executedQty
	}

	return response, nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *AsterTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	params := map[string]interface{}{
		"symbol": symbol,
	}

	body, err := t.request("GET", "/fapi/v3/openOrders", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []struct {
		OrderID      int64  `json:"orderId"`
		Symbol       string `json:"symbol"`
		Side         string `json:"side"`
		PositionSide string `json:"positionSide"`
		Type         string `json:"type"`
		Price        string `json:"price"`
		StopPrice    string `json:"stopPrice"`
		OrigQty      string `json:"origQty"`
		Status       string `json:"status"`
	}

	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse open orders: %w", err)
	}

	var result []types.OpenOrder
	for _, order := range orders {
		price, _ := strconv.ParseFloat(order.Price, 64)
		stopPrice, _ := strconv.ParseFloat(order.StopPrice, 64)
		quantity, _ := strconv.ParseFloat(order.OrigQty, 64)

		result = append(result, types.OpenOrder{
			OrderID:      fmt.Sprintf("%d", order.OrderID),
			Symbol:       order.Symbol,
			Side:         order.Side,
			PositionSide: order.PositionSide,
			Type:         order.Type,
			Price:        price,
			StopPrice:    stopPrice,
			Quantity:     quantity,
			Status:       order.Status,
		})
	}

	logger.Infof("✓ ASTER GetOpenOrders: found %d open orders for %s", len(result), symbol)
	return result, nil
}

// PlaceLimitOrder places a limit order for grid trading
func (t *AsterTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	// Format price and quantity to correct precision
	formattedPrice, err := t.formatPrice(req.Symbol, req.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to format price: %w", err)
	}
	formattedQty, err := t.formatQuantity(req.Symbol, req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to format quantity: %w", err)
	}

	// Get precision information
	prec, err := t.getPrecision(req.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get precision: %w", err)
	}

	// Convert to string with correct precision format
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	// Determine side
	side := "BUY"
	if req.Side == "SELL" || req.Side == "Sell" || req.Side == "sell" {
		side = "SELL"
	}

	params := map[string]interface{}{
		"symbol":       req.Symbol,
		"positionSide": "BOTH",
		"type":         "LIMIT",
		"side":         side,
		"timeInForce":  "GTC",
		"quantity":     qtyStr,
		"price":        priceStr,
	}

	// Add reduceOnly if specified
	if req.ReduceOnly {
		params["reduceOnly"] = "true"
	}

	body, err := t.request("POST", "/fapi/v3/order", params)
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Extract order ID
	orderID := ""
	if id, ok := result["orderId"].(float64); ok {
		orderID = fmt.Sprintf("%.0f", id)
	} else if id, ok := result["orderId"].(string); ok {
		orderID = id
	}

	// Extract client order ID
	clientOrderID := ""
	if cid, ok := result["clientOrderId"].(string); ok {
		clientOrderID = cid
	}

	return &types.LimitOrderResult{
		OrderID:  orderID,
		ClientID: clientOrderID,
		Symbol:   req.Symbol,
		Side:     side,
		Price:    formattedPrice,
		Quantity: formattedQty,
		Status:   "NEW",
	}, nil
}

// CancelOrder cancels a specific order by order ID
func (t *AsterTrader) CancelOrder(symbol, orderID string) error {
	params := map[string]interface{}{
		"symbol":  symbol,
		"orderId": orderID,
	}

	_, err := t.request("DELETE", "/fapi/v3/order", params)
	if err != nil {
		return fmt.Errorf("failed to cancel order %s: %w", orderID, err)
	}

	return nil
}
