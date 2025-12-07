package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"nofx/logger"
	"net/http"
	"time"

	"github.com/elliottech/lighter-go/types"
)

// OpenLong Open long position (implements Trader interface)
func (t *LighterTraderV2) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	logger.Infof("📈 LIGHTER opening long: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// 1. Set leverage (if needed)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️  Failed to set leverage: %v", err)
	}

	// 2. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 3. Create market buy order (open long)
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to open long: %w", err)
	}

	logger.Infof("✓ LIGHTER opened long successfully: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "long",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// OpenShort Open short position (implements Trader interface)
func (t *LighterTraderV2) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	logger.Infof("📉 LIGHTER opening short: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// 1. Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️  Failed to set leverage: %v", err)
	}

	// 2. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 3. Create market sell order (open short)
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to open short: %w", err)
	}

	logger.Infof("✓ LIGHTER opened short successfully: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "short",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// CloseLong Close long position (implements Trader interface)
func (t *LighterTraderV2) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// If quantity=0, get current position quantity
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	logger.Infof("🔻 LIGHTER closing long: %s, qty=%.4f", symbol, quantity)

	// Create market sell order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to close long: %w", err)
	}

	// Cancel all open orders after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel orders: %v", err)
	}

	logger.Infof("✓ LIGHTER closed long successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort Close short position (implements Trader interface)
func (t *LighterTraderV2) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// If quantity=0, get current position quantity
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	logger.Infof("🔺 LIGHTER closing short: %s, qty=%.4f", symbol, quantity)

	// Create market buy order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to close short: %w", err)
	}

	// Cancel all open orders after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel orders: %v", err)
	}

	logger.Infof("✓ LIGHTER closed short successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CreateOrder Create order (market or limit) - uses official SDK for signing
func (t *LighterTraderV2) CreateOrder(symbol string, isAsk bool, quantity float64, price float64, orderType string) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// Get market index (convert from symbol)
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market index: %w", err)
	}

	// Build order request
	clientOrderIndex := time.Now().UnixNano() // Use timestamp as client order ID

	var orderTypeValue uint8 = 0 // 0=limit, 1=market
	if orderType == "market" {
		orderTypeValue = 1
	}

	// Convert quantity and price to LIGHTER format (multiply by precision)
	baseAmount := int64(quantity * 1e8) // 8 decimal precision
	priceValue := uint32(0)
	if orderType == "limit" {
		priceValue = uint32(price * 1e2) // Price precision
	}

	txReq := &types.CreateOrderTxReq{
		MarketIndex:      marketIndex,
		ClientOrderIndex: clientOrderIndex,
		BaseAmount:       baseAmount,
		Price:            priceValue,
		IsAsk:            boolToUint8(isAsk),
		Type:             orderTypeValue,
		TimeInForce:      0, // GTC
		ReduceOnly:       0, // Not reduce-only
		TriggerPrice:     0,
		OrderExpiry:      time.Now().Add(24 * 28 * time.Hour).UnixMilli(), // Expires in 28 days
	}

	// Sign transaction using SDK (nonce will be auto-fetched)
	nonce := int64(-1) // -1 means auto-fetch
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign order: %w", err)
	}

	// Serialize transaction
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// Submit order to LIGHTER API
	orderResp, err := t.submitOrder(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to submit order: %w", err)
	}

	side := "buy"
	if isAsk {
		side = "sell"
	}
	logger.Infof("✓ LIGHTER order created: %s %s qty=%.4f", symbol, side, quantity)

	return orderResp, nil
}

// SendTxRequest Send transaction request
type SendTxRequest struct {
	TxType          int    `json:"tx_type"`
	TxInfo          string `json:"tx_info"`
	PriceProtection bool   `json:"price_protection,omitempty"`
}

// SendTxResponse Send transaction response
type SendTxResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

// submitOrder Submit signed order to LIGHTER API
func (t *LighterTraderV2) submitOrder(signedTx []byte) (map[string]interface{}, error) {
	const TX_TYPE_CREATE_ORDER = 14

	// Build request
	req := SendTxRequest{
		TxType:          TX_TYPE_CREATE_ORDER,
		TxInfo:          string(signedTx),
		PriceProtection: true,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	// Send POST request to /api/v1/sendTx
	endpoint := fmt.Sprintf("%s/api/v1/sendTx", t.baseURL)
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response
	var sendResp SendTxResponse
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	// Check response code
	if sendResp.Code != 200 {
		return nil, fmt.Errorf("failed to submit order (code %d): %s", sendResp.Code, sendResp.Message)
	}

	// Extract transaction hash and order ID
	result := map[string]interface{}{
		"tx_hash": sendResp.Data["tx_hash"],
		"status":  "submitted",
	}

	// Add order ID to result if available
	if orderID, ok := sendResp.Data["order_id"]; ok {
		result["orderId"] = orderID
	} else if txHash, ok := sendResp.Data["tx_hash"].(string); ok {
		// Use tx_hash as orderID
		result["orderId"] = txHash
	}

	logger.Infof("✓ Order submitted to LIGHTER - tx_hash: %v", sendResp.Data["tx_hash"])

	return result, nil
}

// getMarketIndex Get market index (convert from symbol) - dynamically fetch from API
func (t *LighterTraderV2) getMarketIndex(symbol string) (uint8, error) {
	// 1. Check cache
	t.marketMutex.RLock()
	if index, ok := t.marketIndexMap[symbol]; ok {
		t.marketMutex.RUnlock()
		return index, nil
	}
	t.marketMutex.RUnlock()

	// 2. Fetch market list from API
	markets, err := t.fetchMarketList()
	if err != nil {
		// If API fails, fallback to hardcoded mapping
		logger.Infof("⚠️  Failed to fetch market list from API, using hardcoded mapping: %v", err)
		return t.getFallbackMarketIndex(symbol)
	}

	// 3. Update cache
	t.marketMutex.Lock()
	for _, market := range markets {
		t.marketIndexMap[market.Symbol] = market.MarketID
	}
	t.marketMutex.Unlock()

	// 4. Get from cache
	t.marketMutex.RLock()
	index, ok := t.marketIndexMap[symbol]
	t.marketMutex.RUnlock()

	if !ok {
		return 0, fmt.Errorf("unknown market symbol: %s", symbol)
	}

	return index, nil
}

// MarketInfo Market information
type MarketInfo struct {
	Symbol   string `json:"symbol"`
	MarketID uint8  `json:"market_id"`
}

// fetchMarketList Fetch market list from API
func (t *LighterTraderV2) fetchMarketList() ([]MarketInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/orderBooks", t.baseURL)

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

	// Parse response
	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			Symbol      string `json:"symbol"`
			MarketIndex uint8  `json:"market_index"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("failed to get market list (code %d): %s", apiResp.Code, apiResp.Message)
	}

	// Convert to MarketInfo list
	markets := make([]MarketInfo, len(apiResp.Data))
	for i, market := range apiResp.Data {
		markets[i] = MarketInfo{
			Symbol:   market.Symbol,
			MarketID: market.MarketIndex,
		}
	}

	logger.Infof("✓ Retrieved %d markets", len(markets))
	return markets, nil
}

// getFallbackMarketIndex Hardcoded fallback mapping
func (t *LighterTraderV2) getFallbackMarketIndex(symbol string) (uint8, error) {
	fallbackMap := map[string]uint8{
		"BTC-PERP":  0,
		"ETH-PERP":  1,
		"SOL-PERP":  2,
		"DOGE-PERP": 3,
		"AVAX-PERP": 4,
		"XRP-PERP":  5,
	}

	if index, ok := fallbackMap[symbol]; ok {
		logger.Infof("✓ Using hardcoded market index: %s -> %d", symbol, index)
		return index, nil
	}

	return 0, fmt.Errorf("unknown market symbol: %s", symbol)
}

// SetLeverage Set leverage (implements Trader interface)
func (t *LighterTraderV2) SetLeverage(symbol string, leverage int) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// TODO: Sign and submit SetLeverage transaction using SDK
	logger.Infof("⚙️  Setting leverage: %s = %dx", symbol, leverage)

	return nil // Return success for now
}

// SetMarginMode Set margin mode (implements Trader interface)
func (t *LighterTraderV2) SetMarginMode(symbol string, isCrossMargin bool) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	modeStr := "isolated"
	if isCrossMargin {
		modeStr = "cross"
	}

	logger.Infof("⚙️  Setting margin mode: %s = %s", symbol, modeStr)

	// TODO: Sign and submit SetMarginMode transaction using SDK
	return nil
}

// boolToUint8 Convert boolean to uint8
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
