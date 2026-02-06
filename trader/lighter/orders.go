package lighter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nofx/logger"
	"strconv"

	"github.com/elliottech/lighter-go/types"
)

// SetStopLoss Set stop-loss order (implements Trader interface)
// IMPORTANT: Uses StopLossOrder type (type=2) with TriggerPrice, NOT regular limit order
func (t *LighterTraderV2) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	logger.Infof("🛑 LIGHTER Setting stop-loss: %s %s qty=%.4f, trigger=%.2f", symbol, positionSide, quantity, stopPrice)

	// Determine order direction (long position uses sell order, short position uses buy order)
	isAsk := (positionSide == "LONG" || positionSide == "long")

	// Create stop-loss order with TriggerPrice (type=2: StopLossOrder)
	_, err := t.CreateStopOrder(symbol, isAsk, quantity, stopPrice, "stop_loss")
	if err != nil {
		return fmt.Errorf("failed to set stop-loss: %w", err)
	}

	logger.Infof("✓ LIGHTER stop-loss set: trigger=%.2f", stopPrice)
	return nil
}

// SetTakeProfit Set take-profit order (implements Trader interface)
// IMPORTANT: Uses TakeProfitOrder type (type=4) with TriggerPrice, NOT regular limit order
func (t *LighterTraderV2) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	logger.Infof("🎯 LIGHTER Setting take-profit: %s %s qty=%.4f, trigger=%.2f", symbol, positionSide, quantity, takeProfitPrice)

	// Determine order direction (long position uses sell order, short position uses buy order)
	isAsk := (positionSide == "LONG" || positionSide == "long")

	// Create take-profit order with TriggerPrice (type=4: TakeProfitOrder)
	_, err := t.CreateStopOrder(symbol, isAsk, quantity, takeProfitPrice, "take_profit")
	if err != nil {
		return fmt.Errorf("failed to set take-profit: %w", err)
	}

	logger.Infof("✓ LIGHTER take-profit set: trigger=%.2f", takeProfitPrice)
	return nil
}

// CancelAllOrders Cancel all orders (implements Trader interface)
func (t *LighterTraderV2) CancelAllOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("invalid auth token: %w", err)
	}

	// Get all active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	if len(orders) == 0 {
		logger.Infof("✓ LIGHTER - No orders to cancel (no active orders)")
		return nil
	}

	// Batch cancel
	canceledCount := 0
	for _, order := range orders {
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️  Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	logger.Infof("✓ LIGHTER - Canceled %d orders", canceledCount)
	return nil
}

// GetOrderStatus Get order status (implements Trader interface)
func (t *LighterTraderV2) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	// LIGHTER market orders are usually filled immediately
	// Try to query order status
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	// URL encode auth token (contains colons that need encoding)
	// Authentication: Use "auth" query parameter (not Authorization header)
	encodedAuth := url.QueryEscape(t.authToken)

	// Build request URL with auth query parameter
	endpoint := fmt.Sprintf("%s/api/v1/order/%s?auth=%s", t.baseURL, orderID, encodedAuth)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// ✅ 正确做法：查询失败返回错误，而不是假设成交
		return nil, fmt.Errorf("failed to query order status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var order OrderResponse
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w, body: %s", err, string(body))
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
		"executedQty": order.FilledBaseAmount,
		"commission":  0.0,
	}, nil
}

// CancelStopLossOrders Cancel only stop-loss orders (implements Trader interface)
func (t *LighterTraderV2) CancelStopLossOrders(symbol string) error {
	// LIGHTER cannot distinguish between stop-loss and take-profit orders yet, will cancel all stop orders
	logger.Infof("⚠️  LIGHTER cannot distinguish stop-loss/take-profit orders, will cancel all stop orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders Cancel only take-profit orders (implements Trader interface)
func (t *LighterTraderV2) CancelTakeProfitOrders(symbol string) error {
	// LIGHTER cannot distinguish between stop-loss and take-profit orders yet, will cancel all stop orders
	logger.Infof("⚠️  LIGHTER cannot distinguish stop-loss/take-profit orders, will cancel all stop orders")
	return t.CancelStopOrders(symbol)
}

// CancelStopOrders Cancel stop-loss/take-profit orders for this symbol (implements Trader interface)
func (t *LighterTraderV2) CancelStopOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

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
		// TODO: Check order type, only cancel stop orders
		// For now, cancel all orders
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️  Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	logger.Infof("✓ LIGHTER - Canceled %d stop orders", canceledCount)
	return nil
}

// GetActiveOrders Get active orders
func (t *LighterTraderV2) GetActiveOrders(symbol string) ([]OrderResponse, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	// Get market index
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market index: %w", err)
	}

	// URL encode auth token (contains colons that need encoding)
	// Authentication: Use "auth" query parameter (not Authorization header)
	encodedAuth := url.QueryEscape(t.authToken)

	// Build request URL with auth query parameter
	endpoint := fmt.Sprintf("%s/api/v1/accountActiveOrders?account_index=%d&market_id=%d&auth=%s",
		t.baseURL, t.accountIndex, marketIndex, encodedAuth)

	logger.Debugf("📋 LIGHTER GetActiveOrders: endpoint=%s", endpoint[:min(len(endpoint), 120)]+"...")

	// Send GET request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	logger.Debugf("📋 LIGHTER GetActiveOrders raw response: %s", string(body))

	// Parse response - Lighter API uses "orders" field, not "data"
	var apiResp struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Orders  []OrderResponse  `json:"orders"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("failed to get active orders (code %d): %s", apiResp.Code, apiResp.Message)
	}

	logger.Infof("✓ LIGHTER - Retrieved %d active orders", len(apiResp.Orders))
	for i, order := range apiResp.Orders {
		logger.Debugf("   Order[%d]: order_id=%s, order_index=%d, market=%d", i, order.OrderID, order.OrderIndex, order.MarketIndex)
	}
	return apiResp.Orders, nil
}

// CancelOrder Cancel a single order
// orderID can be either a numeric order_index or a tx_hash string
func (t *LighterTraderV2) CancelOrder(symbol, orderID string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get market index
	marketIndexU16, err := t.getMarketIndex(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market index: %w", err)
	}
	marketIndex := uint8(marketIndexU16) // SDK expects uint8

	// Try to parse orderID as numeric order_index first
	orderIndex, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		// orderID is a tx_hash, need to query order to get numeric order_index
		logger.Debugf("📋 LIGHTER CancelOrder: orderID is tx_hash, querying order...")
		orderIndex, err = t.getOrderIndexByTxHash(symbol, orderID)
		if err != nil {
			return fmt.Errorf("failed to get order index from tx_hash: %w", err)
		}
	}

	// Build cancel order request
	txReq := &types.CancelOrderTxReq{
		MarketIndex: marketIndex,
		Index:       orderIndex,
	}

	// Sign transaction using SDK
	// Must provide FromAccountIndex and ApiKeyIndex for nonce auto-fetch to work
	nonce := int64(-1) // -1 means auto-fetch
	apiKeyIdx := t.apiKeyIndex
	tx, err := t.txClient.GetCancelOrderTransaction(txReq, &types.TransactOpts{
		FromAccountIndex: &t.accountIndex,
		ApiKeyIndex:      &apiKeyIdx,
		Nonce:            &nonce,
	})
	if err != nil {
		return fmt.Errorf("failed to sign cancel order: %w", err)
	}

	// Get tx_info from SDK (consistent with CreateOrder and other transactions)
	txInfo, err := tx.GetTxInfo()
	if err != nil {
		return fmt.Errorf("failed to get tx info: %w", err)
	}

	// Submit cancel order to LIGHTER API using unified submitOrder function
	_, err = t.submitOrder(int(tx.GetTxType()), txInfo)
	if err != nil {
		return fmt.Errorf("failed to submit cancel order: %w", err)
	}

	logger.Infof("✓ LIGHTER order canceled - ID: %s", orderID)
	return nil
}

// getOrderIndexByTxHash finds the numeric order_index by searching active orders for the tx_hash
func (t *LighterTraderV2) getOrderIndexByTxHash(symbol, txHash string) (int64, error) {
	// Get all active orders for this symbol
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to get active orders: %w", err)
	}

	// Search for the order with matching tx_hash (order_id)
	for _, order := range orders {
		if order.OrderID == txHash {
			logger.Debugf("📋 LIGHTER Found order_index %d for tx_hash %s", order.OrderIndex, txHash)
			return order.OrderIndex, nil
		}
	}

	return 0, fmt.Errorf("order not found with tx_hash: %s (may already be filled or cancelled)", txHash)
}
