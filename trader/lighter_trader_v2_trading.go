package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"time"

	"github.com/elliottech/lighter-go/types"
)

// OpenLong Open long position (implements Trader interface)
func (t *LighterTraderV2) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	logger.Infof("📈 LIGHTER opening long: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// 1. First cancel all pending orders for this symbol (clean up old stop-loss and take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel old pending orders: %v", err)
	}

	// 2. Set leverage (if needed)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️  Failed to set leverage: %v", err)
	}

	// 3. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 4. Create market buy order (open long)
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market", false)
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

	// 1. First cancel all pending orders for this symbol (clean up old stop-loss and take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel old pending orders: %v", err)
	}

	// 2. Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️  Failed to set leverage: %v", err)
	}

	// 3. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 4. Create market sell order (open short)
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market", false)
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

	// Cancel pending orders before closing
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel orders: %v", err)
	}

	// Create market sell order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market", true)
	if err != nil {
		return nil, fmt.Errorf("failed to close long: %w", err)
	}

	txHash, _ := orderResult["orderId"].(string)
	logger.Infof("✓ LIGHTER closed long successfully: %s (tx: %s)", symbol, txHash)

	return map[string]interface{}{
		"orderId": txHash,
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

	// Cancel pending orders before closing
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel orders: %v", err)
	}

	// Create market buy order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market", true)
	if err != nil {
		return nil, fmt.Errorf("failed to close short: %w", err)
	}

	txHash, _ := orderResult["orderId"].(string)
	logger.Infof("✓ LIGHTER closed short successfully: %s (tx: %s)", symbol, txHash)

	return map[string]interface{}{
		"orderId": txHash,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CreateOrder Create order (market or limit) - uses official SDK for signing
func (t *LighterTraderV2) CreateOrder(symbol string, isAsk bool, quantity float64, price float64, orderType string, reduceOnly bool) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// Get market info (includes market_id and precision)
	marketInfo, err := t.getMarketInfo(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market info: %w", err)
	}
	marketIndex := uint8(marketInfo.MarketID) // SDK expects uint8

	// Build order request
	// Use ClientOrderIndex=0 for market orders (same as web UI)
	clientOrderIndex := int64(0)

	var orderTypeValue uint8 = 0 // 0=limit, 1=market
	if orderType == "market" {
		orderTypeValue = 1
	}

	// Convert quantity to LIGHTER base_amount format using dynamic precision from API
	baseAmount := int64(quantity * float64(pow10(marketInfo.SizeDecimals)))
	logger.Infof("🔸 Using size precision: %d decimals, quantity=%.4f → baseAmount=%d",
		marketInfo.SizeDecimals, quantity, baseAmount)

	// Set price based on order type
	priceValue := uint32(0)
	if orderType == "limit" {
		priceValue = uint32(price * float64(pow10(marketInfo.PriceDecimals)))
		logger.Infof("🔸 LIMIT order - Price: %.2f (precision: %d decimals)", price, marketInfo.PriceDecimals)
	} else {
		// Market order - Price field is used as PRICE PROTECTION (slippage limit)
		// NOT as the execution price! Set it wider to allow order to fill.
		marketPrice, err := t.GetMarketPrice(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get market price: %w", err)
		}

		// For BUY: set price protection ABOVE market (allow buying up to 105% of market price)
		// For SELL: set price protection BELOW market (allow selling down to 95% of market price)
		var protectedPrice float64
		if isAsk {
			// Selling: accept down to 95% of market price
			protectedPrice = marketPrice * 0.95
			logger.Infof("🔸 MARKET SELL order - Price protection: %.2f (95%% of market %.2f, precision: %d decimals)",
				protectedPrice, marketPrice, marketInfo.PriceDecimals)
		} else {
			// Buying: accept up to 105% of market price
			protectedPrice = marketPrice * 1.05
			logger.Infof("🔸 MARKET BUY order - Price protection: %.2f (105%% of market %.2f, precision: %d decimals)",
				protectedPrice, marketPrice, marketInfo.PriceDecimals)
		}
		priceValue = uint32(protectedPrice * float64(pow10(marketInfo.PriceDecimals)))
	}

	// TimeInForce and Expiry based on order type
	// Market orders MUST use TimeInForce=0 (ImmediateOrCancel)
	// Limit orders use TimeInForce=1 (GoodTillTime)
	var orderExpiry int64 = 0
	var timeInForce uint8 = 0 // Default: ImmediateOrCancel for market orders

	if orderType == "limit" {
		timeInForce = 1 // GoodTillTime for limit orders
		orderExpiry = time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	}

	// Set reduceOnly flag
	var reduceOnlyValue uint8 = 0
	if reduceOnly {
		reduceOnlyValue = 1
	}

	txReq := &types.CreateOrderTxReq{
		MarketIndex:      marketIndex,
		ClientOrderIndex: clientOrderIndex,
		BaseAmount:       baseAmount,
		Price:            priceValue,
		IsAsk:            boolToUint8(isAsk),
		Type:             orderTypeValue,
		TimeInForce:      timeInForce,
		ReduceOnly:       reduceOnlyValue,
		TriggerPrice:     0,
		OrderExpiry:      orderExpiry,
	}

	// Sign transaction using SDK (nonce will be auto-fetched)
	nonce := int64(-1) // -1 means auto-fetch
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign order: %w", err)
	}

	// Get tx_info from SDK (uses json.Marshal which produces base64 for []byte)
	txInfo, err := tx.GetTxInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get tx info: %w", err)
	}

	// Debug: Log the tx_info content
	logger.Infof("DEBUG tx_type: %d, tx_info: %s", tx.GetTxType(), txInfo)

	// Submit order to LIGHTER API
	orderResp, err := t.submitOrder(int(tx.GetTxType()), txInfo)
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

// SendTxResponse Send transaction response
type SendTxResponse struct {
	Code                    int                    `json:"code"`
	Message                 string                 `json:"message"`
	TxHash                  string                 `json:"tx_hash"`
	PredictedExecutionTime  int64                  `json:"predicted_execution_time_ms"`
	Data                    map[string]interface{} `json:"data"`
}

// CreateOrderTxInfoAPI Order transaction info with CamelCase JSON tags (matching SDK) + hex signature
type CreateOrderTxInfoAPI struct {
	AccountIndex     int64  `json:"AccountIndex"`
	ApiKeyIndex      uint8  `json:"ApiKeyIndex"`
	MarketIndex      uint8  `json:"MarketIndex"`
	ClientOrderIndex int64  `json:"ClientOrderIndex"`
	BaseAmount       int64  `json:"BaseAmount"`
	Price            uint32 `json:"Price"`
	IsAsk            uint8  `json:"IsAsk"`
	Type             uint8  `json:"Type"`
	TimeInForce      uint8  `json:"TimeInForce"`
	ReduceOnly       uint8  `json:"ReduceOnly"`
	TriggerPrice     uint32 `json:"TriggerPrice"`
	OrderExpiry      int64  `json:"OrderExpiry"`
	ExpiredAt        int64  `json:"ExpiredAt"`
	Nonce            int64  `json:"Nonce"`
	Sig              string `json:"Sig"` // Hex-encoded signature (string)
}

// submitOrder Submit signed order to LIGHTER API using multipart/form-data
func (t *LighterTraderV2) submitOrder(txType int, txInfo string) (map[string]interface{}, error) {
	// Build multipart form data (Lighter API requires form-data, not JSON)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add tx_type field
	if err := writer.WriteField("tx_type", strconv.Itoa(txType)); err != nil {
		return nil, fmt.Errorf("failed to write tx_type: %w", err)
	}

	// Add tx_info field
	if err := writer.WriteField("tx_info", txInfo); err != nil {
		return nil, fmt.Errorf("failed to write tx_info: %w", err)
	}

	// Add price_protection field (false = use Price field as slippage protection)
	if err := writer.WriteField("price_protection", "false"); err != nil {
		return nil, fmt.Errorf("failed to write price_protection: %w", err)
	}

	// Close multipart writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Send POST request to /api/v1/sendTx
	endpoint := fmt.Sprintf("%s/api/v1/sendTx", t.baseURL)
	httpReq, err := http.NewRequest("POST", endpoint, &body)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response
	var sendResp SendTxResponse
	if err := json.Unmarshal(respBody, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(respBody))
	}

	// Log full response for debugging
	logger.Infof("DEBUG API response: %s", string(respBody))

	// Check response code
	if sendResp.Code != 200 {
		return nil, fmt.Errorf("failed to submit order (code %d): %s", sendResp.Code, sendResp.Message)
	}

	// Extract transaction hash and order ID
	// tx_hash is at top level in response, not in data
	txHash := sendResp.TxHash
	if txHash == "" {
		// Fallback to data.tx_hash if present
		if th, ok := sendResp.Data["tx_hash"].(string); ok {
			txHash = th
		}
	}

	result := map[string]interface{}{
		"tx_hash": txHash,
		"status":  "submitted",
		"orderId": txHash, // Use tx_hash as orderId
	}

	logger.Infof("✓ Order submitted to LIGHTER - tx_hash: %s", txHash)

	return result, nil
}

// normalizeSymbol Convert NOFX symbol format to Lighter format
// NOFX uses "BTC-PERP", "BTCUSDT", etc. Lighter uses "BTC", "ETH", etc.
func normalizeSymbol(symbol string) string {
	// Remove common suffixes
	s := strings.TrimSuffix(symbol, "-PERP")
	s = strings.TrimSuffix(s, "USDT")
	s = strings.TrimSuffix(s, "USDC")
	s = strings.TrimSuffix(s, "/USDT")
	s = strings.TrimSuffix(s, "/USDC")
	return strings.ToUpper(s)
}

// getMarketInfo Get market info including precision - dynamically fetch from API
func (t *LighterTraderV2) getMarketInfo(symbol string) (*MarketInfo, error) {
	// Normalize symbol to Lighter format
	normalizedSymbol := normalizeSymbol(symbol)

	// 1. Fetch market list from API (TODO: cache this)
	markets, err := t.fetchMarketList()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch market list: %w", err)
	}

	// 2. Find market by symbol
	for _, market := range markets {
		if market.Symbol == normalizedSymbol {
			return &market, nil
		}
	}

	return nil, fmt.Errorf("unknown market symbol: %s (normalized: %s)", symbol, normalizedSymbol)
}

// getMarketIndex Get market index (convert from symbol) - dynamically fetch from API
func (t *LighterTraderV2) getMarketIndex(symbol string) (uint16, error) {
	marketInfo, err := t.getMarketInfo(symbol)
	if err != nil {
		// Fallback to hardcoded mapping
		logger.Infof("⚠️  Failed to get market info from API, using hardcoded mapping: %v", err)
		normalizedSymbol := normalizeSymbol(symbol)
		return t.getFallbackMarketIndex(normalizedSymbol)
	}
	return marketInfo.MarketID, nil
}

// MarketInfo Market information
type MarketInfo struct {
	Symbol        string `json:"symbol"`
	MarketID      uint16 `json:"market_id"`
	SizeDecimals  int    `json:"size_decimals"`
	PriceDecimals int    `json:"price_decimals"`
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

	// Parse response - Lighter API returns { code: 200, order_books: [...] }
	var apiResp struct {
		Code       int `json:"code"`
		OrderBooks []struct {
			Symbol                 string `json:"symbol"`
			MarketID               uint16 `json:"market_id"`
			Status                 string `json:"status"`
			SupportedSizeDecimals  int    `json:"supported_size_decimals"`
			SupportedPriceDecimals int    `json:"supported_price_decimals"`
		} `json:"order_books"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("failed to get market list (code %d)", apiResp.Code)
	}

	// Convert to MarketInfo list (only active markets)
	markets := make([]MarketInfo, 0, len(apiResp.OrderBooks))
	for _, market := range apiResp.OrderBooks {
		if market.Status == "active" {
			markets = append(markets, MarketInfo{
				Symbol:           market.Symbol,
				MarketID:         market.MarketID,
				SizeDecimals:     market.SupportedSizeDecimals,
				PriceDecimals:    market.SupportedPriceDecimals,
			})
		}
	}

	logger.Infof("✓ Retrieved %d active markets from Lighter", len(markets))
	return markets, nil
}

// getFallbackMarketIndex Hardcoded fallback mapping (using Lighter symbol format)
func (t *LighterTraderV2) getFallbackMarketIndex(symbol string) (uint16, error) {
	// Lighter uses simple symbols like "BTC", "ETH" with market_id
	fallbackMap := map[string]uint16{
		"ETH":  0,
		"BTC":  1,
		"SOL":  2,
		"DOGE": 3,
		"AVAX": 9,
		"XRP":  7,
		"LINK": 8,
		"SUI":  16,
		"BNB":  25,
	}

	if index, ok := fallbackMap[symbol]; ok {
		logger.Infof("✓ Using hardcoded market index: %s -> %d", symbol, index)
		return index, nil
	}

	return 0, fmt.Errorf("unknown market symbol: %s (try fetching market list)", symbol)
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

// CreateStopOrder Create stop-loss or take-profit order with TriggerPrice
// Order types: "stop_loss" (type=2), "take_profit" (type=4)
func (t *LighterTraderV2) CreateStopOrder(symbol string, isAsk bool, quantity float64, triggerPrice float64, orderType string) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// Get market info (includes market_id and precision)
	marketInfo, err := t.getMarketInfo(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market info: %w", err)
	}
	marketIndex := uint8(marketInfo.MarketID)

	// Build order request
	clientOrderIndex := time.Now().UnixMilli() % 281474976710655

	// Order type: StopLossOrder=2, TakeProfitOrder=4
	var orderTypeValue uint8 = 2 // Default: StopLossOrder
	if orderType == "take_profit" {
		orderTypeValue = 4 // TakeProfitOrder
	}

	// Convert quantity to base amount using dynamic precision
	baseAmount := int64(quantity * float64(pow10(marketInfo.SizeDecimals)))

	// TriggerPrice: use dynamic price precision from API
	triggerPriceValue := uint32(triggerPrice * float64(pow10(marketInfo.PriceDecimals)))

	// For stop orders, Price should be set to a reasonable execution price
	// Stop-loss sell: price slightly below trigger (95% of trigger)
	// Take-profit sell: price slightly below trigger (95% of trigger)
	// Stop-loss buy: price slightly above trigger (105% of trigger)
	// Take-profit buy: price slightly above trigger (105% of trigger)
	var priceValue uint32
	if isAsk {
		// Sell order - set price at 95% of trigger to ensure execution
		priceValue = uint32(triggerPrice * 0.95 * float64(pow10(marketInfo.PriceDecimals)))
	} else {
		// Buy order - set price at 105% of trigger to ensure execution
		priceValue = uint32(triggerPrice * 1.05 * float64(pow10(marketInfo.PriceDecimals)))
	}

	// Stop orders MUST use ImmediateOrCancel (0) with expiry set
	// Lighter SDK validates: StopLossOrder/TakeProfitOrder require TimeInForce=0 (ImmediateOrCancel)
	orderExpiry := time.Now().Add(30 * 24 * time.Hour).UnixMilli() // 30 days

	txReq := &types.CreateOrderTxReq{
		MarketIndex:      marketIndex,
		ClientOrderIndex: clientOrderIndex,
		BaseAmount:       baseAmount,
		Price:            priceValue,
		IsAsk:            boolToUint8(isAsk),
		Type:             orderTypeValue,
		TimeInForce:      0, // ImmediateOrCancel - REQUIRED for stop/take-profit orders!
		ReduceOnly:       1, // Stop orders should be reduce-only
		TriggerPrice:     triggerPriceValue,
		OrderExpiry:      orderExpiry,
	}

	// Sign transaction
	nonce := int64(-1)
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign stop order: %w", err)
	}

	// Get tx_info
	txInfo, err := tx.GetTxInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get tx info: %w", err)
	}

	logger.Infof("DEBUG stop order - type: %d, trigger: %.2f, price: %.2f, isAsk: %v", orderTypeValue, triggerPrice, float64(priceValue)/100, isAsk)

	// Submit order
	orderResp, err := t.submitOrder(int(tx.GetTxType()), txInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to submit stop order: %w", err)
	}

	side := "buy"
	if isAsk {
		side = "sell"
	}
	logger.Infof("✓ LIGHTER %s order created: %s %s qty=%.4f trigger=%.2f", orderType, symbol, side, quantity, triggerPrice)

	return orderResp, nil
}

// boolToUint8 Convert boolean to uint8
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// pow10 returns 10^n as int64
func pow10(n int) int64 {
	result := int64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *LighterTraderV2) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	// TODO: Implement Lighter open orders
	return []OpenOrder{}, nil
}
