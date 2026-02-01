package lighter

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
	tradertypes "nofx/trader/types"
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
	// Must provide FromAccountIndex and ApiKeyIndex for nonce auto-fetch to work
	nonce := int64(-1) // -1 means auto-fetch
	apiKeyIdx := t.apiKeyIndex
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		FromAccountIndex: &t.accountIndex,
		ApiKeyIndex:      &apiKeyIdx,
		Nonce:            &nonce,
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
	logger.Debugf("tx_type: %d, tx_info: %s", tx.GetTxType(), txInfo)

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

	// For limit orders, poll for the actual order_index after submission
	// This is needed because CancelOrder requires the numeric order_index, not tx_hash
	if orderType == "limit" {
		txHash, _ := orderResp["tx_hash"].(string)
		if orderIndex, err := t.pollForOrderIndex(symbol, txHash); err == nil && orderIndex > 0 {
			orderResp["orderId"] = fmt.Sprintf("%d", orderIndex)
			orderResp["order_index"] = orderIndex
		}
	}

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
	logger.Debugf("API response: %s", string(respBody))

	// Check response code
	if sendResp.Code != 200 {
		// Provide more specific error message for signature errors
		// Code 21120: invalid signature (order submission)
		// Code 29500: internal server error: invalid signature (authenticated GET APIs)
		if (sendResp.Code == 21120 || sendResp.Code == 29500) && strings.Contains(sendResp.Message, "invalid signature") {
			if !t.apiKeyValid {
				return nil, fmt.Errorf("API Key MISMATCH (code %d): The API key stored in NOFX does not match the one registered on Lighter. Please update your Lighter API key in Exchange settings at app.lighter.xyz", sendResp.Code)
			}
			return nil, fmt.Errorf("API Key signature invalid (code %d): Please verify your Lighter API Key in Exchange settings matches the key registered at app.lighter.xyz", sendResp.Code)
		}
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

	logger.Infof("✓ Order submitted to LIGHTER - tx_hash: %s", txHash)

	result := map[string]interface{}{
		"tx_hash": txHash,
		"status":  "submitted",
		"orderId": txHash, // Use tx_hash as orderId initially
	}

	return result, nil
}

// pollForOrderIndex polls active orders to find the order_index for a newly created order
// Returns the highest order_index (newest order) for the given symbol
func (t *LighterTraderV2) pollForOrderIndex(symbol string, txHash string) (int64, error) {
	// Wait a moment for the order to be processed
	time.Sleep(500 * time.Millisecond)

	// Get active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to get active orders: %w", err)
	}

	if len(orders) == 0 {
		return 0, fmt.Errorf("no active orders found (order may have been filled immediately)")
	}

	// Find the highest order_index (newest order)
	var highestIndex int64
	for _, order := range orders {
		if order.OrderIndex > highestIndex {
			highestIndex = order.OrderIndex
		}
	}

	logger.Infof("✓ Order created with order_index: %d (tx_hash: %s)", highestIndex, txHash)
	return highestIndex, nil
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

	// Fetch market list from API (cached for 1 hour)
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

// fetchMarketList Fetch market list from API with caching (TTL: 1 hour)
func (t *LighterTraderV2) fetchMarketList() ([]MarketInfo, error) {
	// Check cache (TTL: 1 hour)
	t.marketMutex.RLock()
	if len(t.marketListCache) > 0 && time.Since(t.marketListCacheTime) < time.Hour {
		cached := t.marketListCache
		t.marketMutex.RUnlock()
		return cached, nil
	}
	t.marketMutex.RUnlock()

	// Fetch from API
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
				Symbol:        market.Symbol,
				MarketID:      market.MarketID,
				SizeDecimals:  market.SupportedSizeDecimals,
				PriceDecimals: market.SupportedPriceDecimals,
			})
		}
	}

	// Update cache
	t.marketMutex.Lock()
	t.marketListCache = markets
	t.marketListCacheTime = time.Now()
	t.marketMutex.Unlock()

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
// Lighter uses InitialMarginFraction to represent leverage:
//   - InitialMarginFraction = (100 / leverage) * 100  (stored as percentage * 100)
//   - e.g., 5x leverage = 20% margin = 2000 in API
//   - e.g., 20x leverage = 5% margin = 500 in API
func (t *LighterTraderV2) SetLeverage(symbol string, leverage int) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Validate leverage range (1x to 50x typical max)
	if leverage < 1 || leverage > 50 {
		return fmt.Errorf("leverage must be between 1 and 50, got %d", leverage)
	}

	// Get market info (includes market_id)
	marketInfo, err := t.getMarketInfo(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market info: %w", err)
	}
	marketIndex := uint8(marketInfo.MarketID)

	// Calculate InitialMarginFraction from leverage
	// leverage = 100 / margin_fraction_percent
	// margin_fraction_percent = 100 / leverage
	// API value = margin_fraction_percent * 100
	marginFractionPercent := 100.0 / float64(leverage)
	initialMarginFraction := uint16(marginFractionPercent * 100) // e.g., 5x => 20% => 2000

	logger.Infof("⚙️  Setting leverage: %s = %dx (margin_fraction=%.2f%%, API value=%d)",
		symbol, leverage, marginFractionPercent, initialMarginFraction)

	// Build UpdateLeverage request
	txReq := &types.UpdateLeverageTxReq{
		MarketIndex:           marketIndex,
		InitialMarginFraction: initialMarginFraction,
		MarginMode:            0, // 0 = cross margin (default)
	}

	// Sign transaction using SDK
	nonce := int64(-1) // Auto-fetch nonce
	tx, err := t.txClient.GetUpdateLeverageTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return fmt.Errorf("failed to sign leverage transaction: %w", err)
	}

	// Get tx_info from SDK
	txInfo, err := tx.GetTxInfo()
	if err != nil {
		return fmt.Errorf("failed to get tx info: %w", err)
	}

	// Submit to Lighter API (reuse submitOrder which handles any transaction type)
	result, err := t.submitOrder(int(tx.GetTxType()), txInfo)
	if err != nil {
		return fmt.Errorf("failed to submit leverage transaction: %w", err)
	}

	logger.Infof("✓ Leverage set successfully: %s = %dx (tx_hash: %v)", symbol, leverage, result["tx_hash"])
	return nil
}

// SetMarginMode Set margin mode (implements Trader interface)
// Lighter uses UpdateLeverage transaction which includes both leverage and margin mode
// MarginMode: 0 = cross, 1 = isolated
func (t *LighterTraderV2) SetMarginMode(symbol string, isCrossMargin bool) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get market info
	marketInfo, err := t.getMarketInfo(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market info: %w", err)
	}
	marketIndex := uint8(marketInfo.MarketID)

	// Determine margin mode value
	var marginMode uint8 = 0 // cross
	modeStr := "cross"
	if !isCrossMargin {
		marginMode = 1 // isolated
		modeStr = "isolated"
	}

	// Get current position to preserve leverage, or use default 10x if no position
	var initialMarginFraction uint16 = 1000 // Default 10x leverage (10% margin = 1000)
	pos, err := t.GetPosition(symbol)
	if err == nil && pos != nil && pos.Leverage > 0 {
		// Calculate InitialMarginFraction from current leverage
		marginFractionPercent := 100.0 / pos.Leverage
		initialMarginFraction = uint16(marginFractionPercent * 100)
	}

	logger.Infof("⚙️  Setting margin mode: %s = %s (margin_mode=%d, preserving leverage)", symbol, modeStr, marginMode)

	// Build UpdateLeverage request (also updates margin mode)
	txReq := &types.UpdateLeverageTxReq{
		MarketIndex:           marketIndex,
		InitialMarginFraction: initialMarginFraction,
		MarginMode:            marginMode,
	}

	// Sign transaction
	nonce := int64(-1)
	tx, err := t.txClient.GetUpdateLeverageTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return fmt.Errorf("failed to sign margin mode transaction: %w", err)
	}

	// Get tx_info
	txInfo, err := tx.GetTxInfo()
	if err != nil {
		return fmt.Errorf("failed to get tx info: %w", err)
	}

	// Submit to Lighter API
	result, err := t.submitOrder(int(tx.GetTxType()), txInfo)
	if err != nil {
		return fmt.Errorf("failed to submit margin mode transaction: %w", err)
	}

	logger.Infof("✓ Margin mode set successfully: %s = %s (tx_hash: %v)", symbol, modeStr, result["tx_hash"])
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

	logger.Debugf("stop order - type: %d, trigger: %.2f, price: %.2f, isAsk: %v", orderTypeValue, triggerPrice, float64(priceValue)/100, isAsk)

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
func (t *LighterTraderV2) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	// Get active orders from Lighter API
	activeOrders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get active orders: %w", err)
	}

	var result []tradertypes.OpenOrder
	for _, order := range activeOrders {
		// Convert side: Lighter uses is_ask (true=sell, false=buy)
		side := "BUY"
		if order.IsAsk {
			side = "SELL"
		}

		// Determine order type from Lighter's type field
		orderType := "LIMIT"
		if order.Type == "market" {
			orderType = "MARKET"
		} else if order.Type == "stop_loss" || order.Type == "stop" {
			orderType = "STOP_MARKET"
		} else if order.Type == "take_profit" {
			orderType = "TAKE_PROFIT_MARKET"
		}

		// Determine position side based on order direction and reduce-only flag
		positionSide := "LONG"
		if order.ReduceOnly {
			// For reduce-only orders, position side is opposite to order side
			if side == "BUY" {
				positionSide = "SHORT" // Buying to close short
			} else {
				positionSide = "LONG" // Selling to close long
			}
		} else {
			// For opening orders
			if side == "SELL" {
				positionSide = "SHORT"
			}
		}

		// Parse price and quantity from string fields
		price, _ := strconv.ParseFloat(order.Price, 64)
		quantity, _ := strconv.ParseFloat(order.RemainingBaseAmount, 64)
		if quantity == 0 {
			quantity, _ = strconv.ParseFloat(order.InitialBaseAmount, 64)
		}
		triggerPrice, _ := strconv.ParseFloat(order.TriggerPrice, 64)

		openOrder := tradertypes.OpenOrder{
			OrderID:      order.OrderID,
			Symbol:       symbol,
			Side:         side,
			PositionSide: positionSide,
			Type:         orderType,
			Price:        price,
			StopPrice:    triggerPrice,
			Quantity:     quantity,
			Status:       "NEW",
		}
		result = append(result, openOrder)
	}

	logger.Infof("✓ LIGHTER GetOpenOrders: found %d open orders for %s", len(result), symbol)
	return result, nil
}

// PlaceLimitOrder implements GridTrader interface for grid trading
// Places a limit order at the specified price
func (t *LighterTraderV2) PlaceLimitOrder(req *tradertypes.LimitOrderRequest) (*tradertypes.LimitOrderResult, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// Determine if this is a sell (ask) order
	isAsk := req.Side == "SELL"

	logger.Infof("📝 LIGHTER placing limit order: %s %s @ %.4f, qty=%.4f, leverage=%dx",
		req.Symbol, req.Side, req.Price, req.Quantity, req.Leverage)

	// Set leverage before placing order (important for grid trading)
	if req.Leverage > 0 {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			logger.Warnf("⚠️  Failed to set leverage: %v (continuing with current leverage)", err)
		}
	}

	// Create limit order using existing CreateOrder function
	orderResult, err := t.CreateOrder(req.Symbol, isAsk, req.Quantity, req.Price, "limit", req.ReduceOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", err)
	}

	// Extract order ID from result
	orderID := ""
	if id, ok := orderResult["orderId"]; ok {
		orderID = fmt.Sprintf("%v", id)
	} else if txHash, ok := orderResult["tx_hash"]; ok {
		orderID = fmt.Sprintf("%v", txHash)
	}

	logger.Infof("✓ LIGHTER limit order placed: %s %s @ %.4f, OrderID: %s",
		req.Symbol, req.Side, req.Price, orderID)

	return &tradertypes.LimitOrderResult{
		OrderID:      orderID,
		ClientID:     req.ClientID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       "NEW",
	}, nil
}
