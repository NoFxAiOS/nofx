package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"nofx/logger"
	"net/http"
)

// CreateOrderRequest Create order request
type CreateOrderRequest struct {
	Symbol       string  `json:"symbol"`        // Trading pair, e.g. "BTC-PERP"
	Side         string  `json:"side"`          // "buy" or "sell"
	OrderType    string  `json:"order_type"`    // "market" or "limit"
	Quantity     float64 `json:"quantity"`      // Quantity
	Price        float64 `json:"price"`         // Price (required for limit orders)
	ReduceOnly   bool    `json:"reduce_only"`   // Reduce-only flag
	TimeInForce  string  `json:"time_in_force"` // "GTC", "IOC", "FOK"
	PostOnly     bool    `json:"post_only"`     // Post-only (maker only)
}

// OrderResponse Order response
type OrderResponse struct {
	OrderID      string  `json:"order_id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	OrderType    string  `json:"order_type"`
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price"`
	Status       string  `json:"status"` // "open", "filled", "cancelled"
	FilledQty    float64 `json:"filled_qty"`
	RemainingQty float64 `json:"remaining_qty"`
	CreateTime   int64   `json:"create_time"`
}

// CreateOrder Create order (market or limit)
func (t *LighterTrader) CreateOrder(symbol, side string, quantity, price float64, orderType string) (string, error) {
	if err := t.ensureAuthToken(); err != nil {
		return "", fmt.Errorf("invalid auth token: %w", err)
	}

	// Build order request
	req := CreateOrderRequest{
		Symbol:      symbol,
		Side:        side,
		OrderType:   orderType,
		Quantity:    quantity,
		ReduceOnly:  false,
		TimeInForce: "GTC",
		PostOnly:    false,
	}

	if orderType == "limit" {
		req.Price = price
	}

	// Send order
	orderResp, err := t.sendOrder(req)
	if err != nil {
		return "", err
	}

	logger.Infof("✓ LIGHTER order created - ID: %s, Symbol: %s, Side: %s, Qty: %.4f",
		orderResp.OrderID, symbol, side, quantity)

	return orderResp.OrderID, nil
}

// sendOrder Send order to LIGHTER API
func (t *LighterTrader) sendOrder(orderReq CreateOrderRequest) (*OrderResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/order", t.baseURL)

	// Serialize request
	jsonData, err := json.Marshal(orderReq)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// Add request headers
	req.Header.Set("Content-Type", "application/json")
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create order (status %d): %s", resp.StatusCode, string(body))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	return &orderResp, nil
}

// CancelOrder Cancel order
func (t *LighterTrader) CancelOrder(symbol, orderID string) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("invalid auth token: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/order/%s", t.baseURL, orderID)

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}

	// Add auth header
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cancel order (status %d): %s", resp.StatusCode, string(body))
	}

	logger.Infof("✓ LIGHTER order cancelled - ID: %s", orderID)
	return nil
}

// CancelAllOrders Cancel all orders
func (t *LighterTrader) CancelAllOrders(symbol string) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("invalid auth token: %w", err)
	}

	// Get all active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	if len(orders) == 0 {
		logger.Infof("✓ LIGHTER - no orders to cancel (no active orders)")
		return nil
	}

	// Cancel in batch
	for _, order := range orders {
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️ Failed to cancel order (ID: %s): %v", order.OrderID, err)
		}
	}

	logger.Infof("✓ LIGHTER - cancelled %d orders", len(orders))
	return nil
}

// GetActiveOrders Get active orders
func (t *LighterTrader) GetActiveOrders(symbol string) ([]OrderResponse, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/order/active?account_index=%d", t.baseURL, accountIndex)
	if symbol != "" {
		endpoint += fmt.Sprintf("&symbol=%s", symbol)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add auth header
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get active orders (status %d): %s", resp.StatusCode, string(body))
	}

	var orders []OrderResponse
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse order list: %w", err)
	}

	return orders, nil
}

// GetOrderStatus Get order status (implements Trader interface)
func (t *LighterTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/order/%s", t.baseURL, orderID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add auth header
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get order status (status %d): %s", resp.StatusCode, string(body))
	}

	var order OrderResponse
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Convert status to unified format
	unifiedStatus := order.Status
	switch order.Status {
	case "filled":
		unifiedStatus = "FILLED"
	case "open":
		unifiedStatus = "NEW"
	case "cancelled":
		unifiedStatus = "CANCELED"
	}

	return map[string]interface{}{
		"orderId":     order.OrderID,
		"status":      unifiedStatus,
		"avgPrice":    order.Price,
		"executedQty": order.FilledQty,
		"commission":  0.0,
	}, nil
}

// CancelStopLossOrders Cancel stop-loss orders only (LIGHTER cannot distinguish, cancels all TP/SL orders)
func (t *LighterTrader) CancelStopLossOrders(symbol string) error {
	// LIGHTER currently cannot distinguish between stop-loss and take-profit orders, cancel all TP/SL orders
	logger.Infof("  ⚠️ LIGHTER cannot distinguish SL/TP orders, will cancel all TP/SL orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders Cancel take-profit orders only (LIGHTER cannot distinguish, cancels all TP/SL orders)
func (t *LighterTrader) CancelTakeProfitOrders(symbol string) error {
	// LIGHTER currently cannot distinguish between stop-loss and take-profit orders, cancel all TP/SL orders
	logger.Infof("  ⚠️ LIGHTER cannot distinguish SL/TP orders, will cancel all TP/SL orders")
	return t.CancelStopOrders(symbol)
}

// CancelStopOrders Cancel take-profit/stop-loss orders for this symbol
func (t *LighterTrader) CancelStopOrders(symbol string) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("invalid auth token: %w", err)
	}

	// Get active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	canceledCount := 0
	for _, order := range orders {
		// TODO: Need to check order type, only cancel TP/SL orders
		// Currently cancelling all orders
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️ Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	logger.Infof("✓ LIGHTER - cancelled %d TP/SL orders", canceledCount)
	return nil
}
