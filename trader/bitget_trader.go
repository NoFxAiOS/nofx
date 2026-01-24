package trader

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bitget API endpoints (V2)
const (
	bitgetBaseURL         = "https://api.bitget.com"
	bitgetAccountPath     = "/api/v2/mix/account/accounts"
	bitgetPositionPath    = "/api/v2/mix/position/all-position"
	bitgetOrderPath       = "/api/v2/mix/order/place-order"
	bitgetLeveragePath    = "/api/v2/mix/account/set-leverage"
	bitgetTickerPath      = "/api/v2/mix/market/ticker"
	bitgetContractsPath   = "/api/v2/mix/market/contracts"
	bitgetCancelOrderPath = "/api/v2/mix/order/cancel-order"
	bitgetPendingPath     = "/api/v2/mix/order/orders-pending"
	bitgetHistoryPath     = "/api/v2/mix/order/orders-history"
	bitgetMarginModePath  = "/api/v2/mix/account/set-margin-mode"
	bitgetPositionModePath = "/api/v2/mix/account/set-position-mode"
)

// BitgetTrader Bitget futures trader
type BitgetTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// HTTP client
	httpClient *http.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Positions cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Contract info cache
	contractsCache      map[string]*BitgetContract
	contractsCacheTime  time.Time
	contractsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
	
	// Rate limiting
	rateLimiter *time.Ticker
	requestsMutex sync.Mutex
	lastRequestTime time.Time
	minRequestInterval time.Duration // Minimum interval between requests
}

// BitgetContract Bitget contract info
type BitgetContract struct {
	Symbol       string  // Symbol name
	BaseCoin     string  // Base coin
	QuoteCoin    string  // Quote coin
	MinTradeNum  float64 // Minimum trade amount
	MaxTradeNum  float64 // Maximum trade amount
	SizeMultiplier float64 // Contract size multiplier
	PricePlace   int     // Price decimal places
	VolumePlace  int     // Volume decimal places
}

// BitgetResponse Bitget API response
type BitgetResponse struct {
	Code    string          `json:"code"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
	RequestTime int64       `json:"requestTime"`
}

// NewBitgetTrader creates a Bitget trader
func NewBitgetTrader(apiKey, secretKey, passphrase string) *BitgetTrader {
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
	}

	trader := &BitgetTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		passphrase:     passphrase,
		httpClient:     httpClient,
		cacheDuration:  15 * time.Second,
		contractsCache: make(map[string]*BitgetContract),
	minRequestInterval: 100 * time.Millisecond, // Rate limit: max 10 requests per second
	lastRequestTime: time.Now(),
	}

	logger.Infof("🔧 [Bitget] Initializing trader...")

	// Set dual-long-short position mode
	if err := trader.setPositionMode(); err != nil {
		logger.Infof("  ⚠️ Failed to set position mode: %v (ignore if already set)", err)
	}

	// Test API connectivity
	if _, err := trader.GetBalance(); err != nil {
		logger.Infof("  ⚠️ Failed to verify API connectivity: %v", err)
	} else {
		logger.Infof("  ✓ API connectivity verified")
	}

	logger.Infof("✅ [Bitget] Trader initialized successfully")

	return trader
}

// setPositionMode sets dual position mode with fallback handling
func (t *BitgetTrader) setPositionMode() error {
	// Try dual position mode first (hedge_mode per Bitget V2 API docs)
	body := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"posMode":     "hedge_mode", // 双向持仓
	}

	_, err := t.doRequest("POST", bitgetPositionModePath, body)
	if err != nil {
		if strings.Contains(err.Error(), "same") || strings.Contains(err.Error(), "already") {
			logger.Infof("  ✓ Bitget account already in hedge (dual) position mode")
			return nil
		}
		// If dual mode fails (e.g., has open positions), log and continue
		// The order methods will auto-detect and adapt to the actual position mode
		logger.Warnf("  ⚠️ Cannot switch to hedge mode (may have open positions): %v", err)
		logger.Infof("  ℹ️ Will auto-detect position mode when placing orders")
		return nil // Don't fail initialization, let order methods handle it
	}

	logger.Infof("  ✓ Bitget account switched to hedge (dual) position mode")
	return nil
}

// sign generates Bitget API signature
func (t *BitgetTrader) sign(timestamp, method, requestPath, body string) string {
	// Signature = BASE64(HMAC_SHA256(timestamp + method + requestPath + body, secretKey))
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest executes HTTP request
func (t *BitgetTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error
	var queryString string

	if body != nil {
		if method == "GET" {
			// For GET requests, body is query parameters
			if params, ok := body.(map[string]interface{}); ok {
				var parts []string
				for k, v := range params {
					parts = append(parts, fmt.Sprintf("%s=%v", k, v))
				}
				queryString = strings.Join(parts, "&")
				if queryString != "" {
					path = path + "?" + queryString
				}
			}
		} else {
			bodyBytes, err = json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize request body: %w", err)
			}
		}
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Signature includes body for POST, nothing for GET (query is in path)
	signBody := ""
	if method != "GET" && bodyBytes != nil {
		signBody = string(bodyBytes)
	}
	signature := t.sign(timestamp, method, path, signBody)

	url := bitgetBaseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("ACCESS-KEY", t.apiKey)
	req.Header.Set("ACCESS-SIGN", signature)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("locale", "en-US")
	// Channel code only for order endpoints
	if strings.Contains(path, "/order/") {
		req.Header.Set("X-CHANNEL-API-CODE", "7fygt")
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var bitgetResp BitgetResponse
	if err := json.Unmarshal(respBody, &bitgetResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(respBody))
	}

	if bitgetResp.Code != "00000" {
		return nil, fmt.Errorf("Bitget API error: code=%s, msg=%s", bitgetResp.Code, bitgetResp.Msg)
	}

	return bitgetResp.Data, nil
}
// doRequest executes HTTP request with automatic rate limiting and exponential backoff retry
func (t *BitgetTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	return t.doRequestWithRetry(method, path, body, 0, 1*time.Second)
}

// doRequestWithRetry executes HTTP request with exponential backoff retry for rate limiting
func (t *BitgetTrader) doRequestWithRetry(method, path string, body interface{}, retryCount int, initialBackoff time.Duration) ([]byte, error) {
	const maxRetries = 3

	// Apply rate limiting: ensure minimum interval between requests
	t.requestsMutex.Lock()
	timeSinceLastRequest := time.Since(t.lastRequestTime)
	if timeSinceLastRequest < t.minRequestInterval {
		sleepDuration := t.minRequestInterval - timeSinceLastRequest
		t.requestsMutex.Unlock()
		time.Sleep(sleepDuration)
		t.requestsMutex.Lock()
	}
	t.lastRequestTime = time.Now()
	t.requestsMutex.Unlock()

	var bodyBytes []byte
	var err error
	var queryString string

	if body != nil {
		if method == "GET" {
			// For GET requests, body is query parameters
			if params, ok := body.(map[string]interface{}); ok {
				var parts []string
				for k, v := range params {
					parts = append(parts, fmt.Sprintf("%s=%v", k, v))
				}
				queryString = strings.Join(parts, "&")
				if queryString != "" {
					path = path + "?" + queryString
				}
			}
		} else {
			bodyBytes, err = json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize request body: %w", err)
			}
		}
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Signature includes body for POST, nothing for GET (query is in path)
	signBody := ""
	if method != "GET" && bodyBytes != nil {
		signBody = string(bodyBytes)
	}
	signature := t.sign(timestamp, method, path, signBody)

	url := bitgetBaseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("ACCESS-KEY", t.apiKey)
	req.Header.Set("ACCESS-SIGN", signature)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("locale", "en-US")
	// Channel code only for order endpoints
	if strings.Contains(path, "/order/") {
		req.Header.Set("X-CHANNEL-API-CODE", "7fygt")
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var bitgetResp BitgetResponse
	if err := json.Unmarshal(respBody, &bitgetResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(respBody))
	}

	if bitgetResp.Code != "00000" {
		// Handle rate limiting with exponential backoff
		if bitgetResp.Code == "429" && retryCount < maxRetries {
			backoffTime := initialBackoff * time.Duration(1<<uint(retryCount)) // Exponential backoff: 1s, 2s, 4s
			logger.Warnf("⏳ [Bitget] Rate limited (429). Retry %d/%d after %v...", retryCount+1, maxRetries, backoffTime)
			time.Sleep(backoffTime)
			return t.doRequestWithRetry(method, path, body, retryCount+1, initialBackoff)
		}
		// For other errors, return the error
		return nil, fmt.Errorf("Bitget API error: code=%s, msg=%s", bitgetResp.Code, bitgetResp.Msg)
	}

	return bitgetResp.Data, nil
}

// convertSymbol converts generic symbol to Bitget format
// e.g., BTCUSDT -> BTCUSDT
func (t *BitgetTrader) convertSymbol(symbol string) string {
	// Bitget uses same format as input, just ensure uppercase
	return strings.ToUpper(symbol)
}

// GetBalance gets account balance (with cache)
func (t *BitgetTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache first
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ [Bitget] Using cached account balance (cache age: %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	logger.Infof("🔄 [Bitget] Cache expired, calling API to get account balance...")

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetAccountPath, params)
	if err != nil {
		logger.Infof("❌ [Bitget] API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	var accounts []struct {
		MarginCoin      string `json:"marginCoin"`
		Available       string `json:"available"`       // Available balance
		AccountEquity   string `json:"accountEquity"`   // Total equity
		UsdtEquity      string `json:"usdtEquity"`      // USDT equity
		UnrealizedPL    string `json:"unrealizedPL"`    // Unrealized P&L
	}

	if err := json.Unmarshal(data, &accounts); err != nil {
		logger.Infof("❌ [Bitget] Failed to parse balance data: %w, raw: %s", err, string(data))
		return nil, fmt.Errorf("failed to parse balance data: %w", err)
	}

	var totalEquity, availableBalance, unrealizedPnL float64
	for _, acc := range accounts {
		if acc.MarginCoin == "USDT" {
			totalEquity, _ = strconv.ParseFloat(acc.AccountEquity, 64)
			availableBalance, _ = strconv.ParseFloat(acc.Available, 64)
			unrealizedPnL, _ = strconv.ParseFloat(acc.UnrealizedPL, 64)
			logger.Infof("✓ [Bitget] API returned: total balance=%.2f, available=%.2f, unrealized PnL=%.2f",
				totalEquity, availableBalance, unrealizedPnL)
			break
		}
	}

	result := map[string]interface{}{
		"totalWalletBalance":    totalEquity - unrealizedPnL,
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": unrealizedPnL,
		"total_equity":          totalEquity,
	}

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions gets all positions (with cache)
func (t *BitgetTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache first
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		logger.Infof("✓ [Bitget] Using cached positions (cache age: %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	logger.Infof("🔄 [Bitget] Cache expired, calling API to get positions...")

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
	}

	data, err := t.doRequest("GET", bitgetPositionPath, params)
	if err != nil {
		logger.Infof("❌ [Bitget] API call failed: %v", err)
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []struct {
		Symbol           string `json:"symbol"`
		HoldSide         string `json:"holdSide"`         // long, short
		OpenPriceAvg     string `json:"openPriceAvg"`     // Average entry price
		MarkPrice        string `json:"markPrice"`        // Mark price
		Total            string `json:"total"`            // Total position size
		Available        string `json:"available"`        // Available to close
		UnrealizedPL     string `json:"unrealizedPL"`     // Unrealized P&L
		Leverage         string `json:"leverage"`         // Leverage
		LiquidationPrice string `json:"liquidationPrice"` // Liquidation price
		MarginSize       string `json:"marginSize"`       // Position margin
		CTime            string `json:"cTime"`            // Create time
		UTime            string `json:"uTime"`            // Update time
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		logger.Infof("❌ [Bitget] Failed to parse position data: %w", err)
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	var result []map[string]interface{}
	var activeCount int
	for _, pos := range positions {
		total, _ := strconv.ParseFloat(pos.Total, 64)
		if total == 0 {
			continue
		}
		activeCount++

		entryPrice, _ := strconv.ParseFloat(pos.OpenPriceAvg, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
		unrealizedPnL, _ := strconv.ParseFloat(pos.UnrealizedPL, 64)
		leverage, _ := strconv.ParseFloat(pos.Leverage, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiquidationPrice, 64)
		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)

		// Normalize side
		side := "long"
		if pos.HoldSide == "short" {
			side = "short"
		}

		posMap := map[string]interface{}{
			"symbol":           pos.Symbol,
			"positionAmt":      total,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealizedPnL,
			"leverage":         leverage,
			"liquidationPrice": liqPrice,
			"side":             side,
			"createdTime":      cTime,
			"updatedTime":      uTime,
		}
		result = append(result, posMap)
	}

	logger.Infof("✓ [Bitget] API returned: %d active positions", activeCount)

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// getContract gets contract info
func (t *BitgetTrader) getContract(symbol string) (*BitgetContract, error) {
	symbol = t.convertSymbol(symbol)

	// Check cache
	t.contractsCacheMutex.RLock()
	if contract, ok := t.contractsCache[symbol]; ok && time.Since(t.contractsCacheTime) < 5*time.Minute {
		t.contractsCacheMutex.RUnlock()
		return contract, nil
	}
	t.contractsCacheMutex.RUnlock()

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"symbol":      symbol,
	}

	data, err := t.doRequest("GET", bitgetContractsPath, params)
	if err != nil {
		return nil, err
	}

	var contracts []struct {
		Symbol         string `json:"symbol"`
		BaseCoin       string `json:"baseCoin"`
		QuoteCoin      string `json:"quoteCoin"`
		MinTradeNum    string `json:"minTradeNum"`
		MaxTradeNum    string `json:"maxTradeNum"`
		SizeMultiplier string `json:"sizeMultiplier"`
		PricePlace     string `json:"pricePlace"`
		VolumePlace    string `json:"volumePlace"`
	}

	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, err
	}

	// Find matching contract
	for _, c := range contracts {
		if c.Symbol == symbol {
			minTrade, _ := strconv.ParseFloat(c.MinTradeNum, 64)
			maxTrade, _ := strconv.ParseFloat(c.MaxTradeNum, 64)
			sizeMult, _ := strconv.ParseFloat(c.SizeMultiplier, 64)
			pricePlace, _ := strconv.Atoi(c.PricePlace)
			volumePlace, _ := strconv.Atoi(c.VolumePlace)

			contract := &BitgetContract{
				Symbol:         c.Symbol,
				BaseCoin:       c.BaseCoin,
				QuoteCoin:      c.QuoteCoin,
				MinTradeNum:    minTrade,
				MaxTradeNum:    maxTrade,
				SizeMultiplier: sizeMult,
				PricePlace:     pricePlace,
				VolumePlace:    volumePlace,
			}

			// Update cache
			t.contractsCacheMutex.Lock()
			t.contractsCache[symbol] = contract
			t.contractsCacheTime = time.Now()
			t.contractsCacheMutex.Unlock()

			return contract, nil
		}
	}

	return nil, fmt.Errorf("contract info not found: %s", symbol)
}

// SetMarginMode sets margin mode
func (t *BitgetTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	symbol = t.convertSymbol(symbol)

	marginMode := "isolated"
	if isCrossMargin {
		marginMode = "crossed"
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"marginMode":  marginMode,
	}

	_, err := t.doRequest("POST", bitgetMarginModePath, body)
	if err != nil {
		// Margin mode already set
		if contains(err.Error(), "same") || contains(err.Error(), "already") {
			logger.Infof("  ✓ %s margin mode is already %s", symbol, marginMode)
			return nil
		}
		// Has open positions, cannot change
		if contains(err.Error(), "position") {
			logger.Infof("  ⚠️ %s has positions, cannot change margin mode, continuing with current mode", symbol)
			return nil
		}
		// Other errors
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		return nil // Don't fail trading, let it continue
	}

	logger.Infof("  ✓ %s margin mode set to %s", symbol, marginMode)
	return nil
}

// SetLeverage sets leverage with smart detection
func (t *BitgetTrader) SetLeverage(symbol string, leverage int) error {
	symbol = t.convertSymbol(symbol)

	// Try to get current leverage (from position information)
	currentLeverage := 0
	positions, err := t.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == symbol {
				if lev, ok := pos["leverage"].(float64); ok {
					currentLeverage = int(lev)
					break
				}
			}
		}
	}

	// If current leverage is already the target leverage, skip
	if currentLeverage == leverage && currentLeverage > 0 {
		logger.Infof("  ✓ %s leverage is already %d, no need to change", symbol, leverage)
		return nil
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"leverage":    fmt.Sprintf("%d", leverage),
	}

	_, err = t.doRequest("POST", bitgetLeveragePath, body)
	if err != nil {
		// Leverage is already set
		if contains(err.Error(), "same") || contains(err.Error(), "No need") {
			logger.Infof("  ✓ %s leverage is already %d", symbol, leverage)
			return nil
		}
		logger.Infof("  ⚠️ Failed to set %s leverage: %v", symbol, err)
		// Don't fail trading, let it continue
		return nil
	}

	logger.Infof("  ✓ %s leverage set to %d", symbol, leverage)
	return nil
}

// OpenLong opens long position
func (t *BitgetTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)
	logger.Infof("🟢 [Bitget] Opening long position: symbol=%s, qty=%.6f, leverage=%d", symbol, quantity, leverage)

	// Cancel old orders first
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	// Format quantity to correct precision
	qtyStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to format quantity: %w", err)
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(qtyStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("position size too small, rounded to 0 (original: %.8f → formatted: %s). Suggest increasing position amount or selecting a lower-priced coin", quantity, qtyStr)
	}

	// Check minimum notional value
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        "buy",
		"posSide":     "long",
		"tradeSide":   "open",
		"orderType":   "market",
		"size":        qtyStr,
		"clientOid":   genBitgetClientOid(),
	}

	logger.Infof("  📊 Bitget OpenLong: symbol=%s, qty=%s, leverage=%d", symbol, qtyStr, leverage)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		// If dual position mode fails, try single position mode
		if strings.Contains(err.Error(), "40774") || strings.Contains(err.Error(), "unilateral position") {
			logger.Infof("  🔄 Dual position failed, trying single position mode...")
			// Remove dual position parameters
			delete(body, "posSide")
			delete(body, "tradeSide")
			data, err = t.doRequest("POST", bitgetOrderPath, body)
		}
		if err != nil {
			logger.Infof("❌ [Bitget] Failed to open long position: %v", err)
			return nil, fmt.Errorf("failed to open long position: %w", err)
		}
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		logger.Infof("❌ [Bitget] Failed to parse order response: %v", err)
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Invalidate relevant caches after trade
	t.invalidateBalanceCache()
	t.invalidatePositionsCache()

	logger.Infof("✅ [Bitget] Long position opened successfully: symbol=%s, orderId=%s", symbol, order.OrderId)

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort opens short position
func (t *BitgetTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)
	logger.Infof("🔴 [Bitget] Opening short position: symbol=%s, qty=%.6f, leverage=%d", symbol, quantity, leverage)

	// Cancel old orders first
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	// Format quantity to correct precision
	qtyStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to format quantity: %w", err)
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(qtyStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("position size too small, rounded to 0 (original: %.8f → formatted: %s). Suggest increasing position amount or selecting a lower-priced coin", quantity, qtyStr)
	}

	// Check minimum notional value
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        "sell",
		"posSide":     "short",
		"tradeSide":   "open",
		"orderType":   "market",
		"size":        qtyStr,
		"clientOid":   genBitgetClientOid(),
	}

	// Only add dual position parameters if needed
	// For now, keep it simple for single position mode compatibility

	logger.Infof("  📊 Bitget OpenShort: symbol=%s, qty=%s, leverage=%d", symbol, qtyStr, leverage)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		// If dual position mode fails, try single position mode
		if strings.Contains(err.Error(), "40774") || strings.Contains(err.Error(), "unilateral position") {
			logger.Infof("  🔄 Dual position failed, trying single position mode...")
			// Remove dual position parameters
			delete(body, "posSide")
			delete(body, "tradeSide")
			data, err = t.doRequest("POST", bitgetOrderPath, body)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to open short position: %w", err)
		}
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Invalidate relevant caches after trade
	t.invalidateBalanceCache()
	t.invalidatePositionsCache()

	logger.Infof("✓ Bitget opened short position successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong closes long position
func (t *BitgetTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)
	logger.Infof("🟢 [Bitget] Closing long position: symbol=%s, qty=%.6f", symbol, quantity)

	// If quantity is 0, get current position
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			logger.Infof("❌ [Bitget] Failed to get positions: %v", err)
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}
		if quantity == 0 {
			logger.Infof("❌ [Bitget] Long position not found for %s", symbol)
			return nil, fmt.Errorf("long position not found for %s", symbol)
		}
	}

	// Format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        "sell",
		"posSide":     "long",
		"tradeSide":   "close",
		"orderType":   "market",
		"size":        qtyStr,
		"reduceOnly":  "YES",
		"clientOid":   genBitgetClientOid(),
	}

	// Only add dual position parameters if needed
	// For now, keep it simple for single position mode compatibility

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		// If dual position mode fails, try single position mode
		if strings.Contains(err.Error(), "40774") || strings.Contains(err.Error(), "unilateral position") {
			logger.Infof("  🔄 Dual position failed, trying single position mode...")
			// Remove dual position parameters
			delete(body, "posSide")
			delete(body, "tradeSide")
			data, err = t.doRequest("POST", bitgetOrderPath, body)
		}
		if err != nil {
			logger.Infof("❌ [Bitget] Failed to close long position: %v", err)
			return nil, fmt.Errorf("failed to close long position: %w", err)
		}
	}

	var order struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		logger.Infof("❌ [Bitget] Failed to parse order response: %v", err)
		return nil, err
	}

	// Invalidate relevant caches after trade
	t.invalidateBalanceCache()
	t.invalidatePositionsCache()

	logger.Infof("✅ [Bitget] Long position closed successfully: symbol=%s, orderId=%s", symbol, order.OrderId)

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort closes short position
func (t *BitgetTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// If quantity is 0, get current position
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}
		if quantity == 0 {
			return nil, fmt.Errorf("short position not found for %s", symbol)
		}
	}

	// Ensure quantity is positive
	if quantity < 0 {
		quantity = -quantity
	}

	// Format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        "buy",
		"posSide":     "short",
		"tradeSide":   "close",
		"orderType":   "market",
		"size":        qtyStr,
		"reduceOnly":  "YES",
		"clientOid":   genBitgetClientOid(),
	}

	// Only add dual position parameters if needed
	// For now, keep it simple for single position mode compatibility

	logger.Infof("  📊 Bitget CloseShort: symbol=%s, qty=%s", symbol, qtyStr)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		// If dual position mode fails, try single position mode
		if strings.Contains(err.Error(), "40774") || strings.Contains(err.Error(), "unilateral position") {
			logger.Infof("  🔄 Dual position failed, trying single position mode...")
			// Remove dual position parameters
			delete(body, "posSide")
			delete(body, "tradeSide")
			data, err = t.doRequest("POST", bitgetOrderPath, body)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to close short position: %w", err)
		}
	}

	var order struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}

	// Invalidate relevant caches after trade
	t.invalidateBalanceCache()
	t.invalidatePositionsCache()

	logger.Infof("✅ [Bitget] Short position closed successfully: symbol=%s, orderId=%s", symbol, order.OrderId)

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice gets market price
func (t *BitgetTrader) GetMarketPrice(symbol string) (float64, error) {
	symbol = t.convertSymbol(symbol)

	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetTickerPath, params)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	var tickers []struct {
		LastPr string `json:"lastPr"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, err
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("no price data received")
	}

	price, err := strconv.ParseFloat(tickers[0].LastPr, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetStopLoss sets stop loss order
func (t *BitgetTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	// Bitget V2 uses TPSL order for stop loss
	symbol = t.convertSymbol(symbol)

	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	// Try with holdSide first (dual position mode)
	holdSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		holdSide = "short"
	}

	body := map[string]interface{}{
		"marginCoin":   "USDT",
		"productType":  "USDT-FUTURES",
		"symbol":       symbol,
		"planType":     "loss_plan",
		"triggerPrice": fmt.Sprintf("%.8f", stopPrice),
		"triggerType":  "mark_price",
		"executePrice": "0", // 0 means market execution
		"holdSide":     holdSide,
		"size":         qtyStr,
		"clientOid":    genBitgetClientOid(),
	}

	_, err := t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
	if err != nil {
		// If holdSide error, try without holdSide (single position mode)
		if strings.Contains(err.Error(), "43011") || strings.Contains(err.Error(), "holdSide") {
			logger.Infof("  🔄 [Bitget] Dual position mode failed, trying single position mode...")
			delete(body, "holdSide")
			body["clientOid"] = genBitgetClientOid() // New client order ID
			_, err = t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
		}
		if err != nil {
			return fmt.Errorf("failed to set stop loss: %w", err)
		}
	}

	logger.Infof("  ✓ [Bitget] Stop loss set: %s @ %.4f", symbol, stopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *BitgetTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// Bitget V2 uses TPSL order for take profit
	symbol = t.convertSymbol(symbol)

	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	// Try with holdSide first (dual position mode)
	holdSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		holdSide = "short"
	}

	body := map[string]interface{}{
		"marginCoin":   "USDT",
		"productType":  "USDT-FUTURES",
		"symbol":       symbol,
		"planType":     "profit_plan",
		"triggerPrice": fmt.Sprintf("%.8f", takeProfitPrice),
		"triggerType":  "mark_price",
		"executePrice": "0", // 0 means market execution
		"holdSide":     holdSide,
		"size":         qtyStr,
		"clientOid":    genBitgetClientOid(),
	}

	_, err := t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
	if err != nil {
		// If holdSide error, try without holdSide (single position mode)
		if strings.Contains(err.Error(), "43011") || strings.Contains(err.Error(), "holdSide") {
			logger.Infof("  🔄 [Bitget] Dual position mode failed, trying single position mode...")
			delete(body, "holdSide")
			body["clientOid"] = genBitgetClientOid() // New client order ID
			_, err = t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
		}
		if err != nil {
			return fmt.Errorf("failed to set take profit: %w", err)
		}
	}

	logger.Infof("  ✓ [Bitget] Take profit set: %s @ %.4f", symbol, takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop loss orders
func (t *BitgetTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelPlanOrders(symbol, "loss_plan") // Use specific loss_plan type
}

// CancelTakeProfitOrders cancels take profit orders
func (t *BitgetTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelPlanOrders(symbol, "profit_plan") // Use specific profit_plan type
}

// cancelPlanOrders cancels plan orders
func (t *BitgetTrader) cancelPlanOrders(symbol string, planType string) error {
	symbol = t.convertSymbol(symbol)

	// Get pending plan orders (planType is required)
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"planType":    planType, // This was missing and causing the error
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/orders-plan-pending", params)
	if err != nil {
		return err
	}

	var orders struct {
		EntrustedList []struct {
			OrderId string `json:"orderId"`
		} `json:"entrustedList"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	// Cancel each order
	for _, order := range orders.EntrustedList {
		body := map[string]interface{}{
			"symbol":      symbol,
			"productType": "USDT-FUTURES",
			"marginCoin":  "USDT",
			"orderId":     order.OrderId,
		}
		t.doRequest("POST", "/api/v2/mix/order/cancel-plan-order", body)
	}

	return nil
}

// CancelAllOrders cancels all pending orders
func (t *BitgetTrader) CancelAllOrders(symbol string) error {
	symbol = t.convertSymbol(symbol)

	// Get pending orders
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetPendingPath, params)
	if err != nil {
		return err
	}

	var orders struct {
		EntrustedList []struct {
			OrderId string `json:"orderId"`
		} `json:"entrustedList"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	// Cancel each order
	for _, order := range orders.EntrustedList {
		body := map[string]interface{}{
			"symbol":      symbol,
			"productType": "USDT-FUTURES",
			"marginCoin":  "USDT",
			"orderId":     order.OrderId,
		}
		t.doRequest("POST", bitgetCancelOrderPath, body)
	}

	// Cancel all plan orders (SL/TP) - need to call separately for each type
	t.cancelPlanOrders(symbol, "loss_plan")   // Cancel stop loss orders
	t.cancelPlanOrders(symbol, "profit_plan") // Cancel take profit orders

	return nil
}

// CancelStopOrders cancels stop loss and take profit orders
func (t *BitgetTrader) CancelStopOrders(symbol string) error {
	t.CancelStopLossOrders(symbol)
	t.CancelTakeProfitOrders(symbol)
	return nil
}

// FormatPrice formats price to correct precision
func (t *BitgetTrader) FormatPrice(symbol string, price float64) (string, error) {
	precision, err := t.GetSymbolPricePrecision(symbol)
	if err != nil {
		// If retrieval fails, use default format
		return fmt.Sprintf("%.2f", price), nil
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, price), nil
}

// CheckMinNotional checks if the order amount meets minimum notional requirements
func (t *BitgetTrader) CheckMinNotional(symbol string, quantity float64) error {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	notionalValue := quantity * price
	// Bitget minimum notional value is typically 5 USDT
	minNotional := 5.0

	if notionalValue < minNotional {
		return fmt.Errorf(
			"order amount %.2f USDT is below minimum requirement %.2f USDT (quantity: %.4f, price: %.4f). Suggest increasing position amount or selecting a lower-priced coin",
			notionalValue, minNotional, quantity, price,
		)
	}

	return nil
}

// CalculatePositionSize calculates position size based on risk percentage
func (t *BitgetTrader) CalculatePositionSize(balance, riskPercent, price float64, leverage int) float64 {
	if price <= 0 || leverage <= 0 {
		return 0
	}
	// Position size = (balance * risk%) / price / leverage
	return (balance * riskPercent / 100) / price / float64(leverage)
}

// GetSymbolPrecision gets the quantity precision for a trading pair
func (t *BitgetTrader) GetSymbolPrecision(symbol string) (int, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		// If retrieval fails, use default precision 4
		return 4, nil
	}

	return contract.VolumePlace, nil
}

// GetSymbolPricePrecision gets the price precision for a trading pair
func (t *BitgetTrader) GetSymbolPricePrecision(symbol string) (int, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		// If retrieval fails, use default precision 2
		return 2, nil
	}

	return contract.PricePlace, nil
}

// FormatQuantity formats quantity to correct precision
func (t *BitgetTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	precision, err := t.GetSymbolPrecision(symbol)
	if err != nil {
		// If retrieval fails, use default format
		return fmt.Sprintf("%.4f", quantity), nil
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// GetOrderStatus gets order status
func (t *BitgetTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"orderId":     orderID,
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/detail", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var order struct {
		OrderId      string `json:"orderId"`
		State        string `json:"state"`        // filled, canceled, partially_filled, new
		PriceAvg     string `json:"priceAvg"`     // Average fill price
		BaseVolume   string `json:"baseVolume"`   // Filled quantity
		Fee          string `json:"fee"`          // Fee
		Side         string `json:"side"`
		OrderType    string `json:"orderType"`
		CTime        string `json:"cTime"`
		UTime        string `json:"uTime"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}

	avgPrice, _ := strconv.ParseFloat(order.PriceAvg, 64)
	fillQty, _ := strconv.ParseFloat(order.BaseVolume, 64)
	fee, _ := strconv.ParseFloat(order.Fee, 64)
	cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
	uTime, _ := strconv.ParseInt(order.UTime, 10, 64)

	// Status mapping
	statusMap := map[string]string{
		"filled":           "FILLED",
		"new":              "NEW",
		"partially_filled": "PARTIALLY_FILLED",
		"canceled":         "CANCELED",
	}

	status := statusMap[order.State]
	if status == "" {
		status = order.State
	}

	return map[string]interface{}{
		"orderId":     order.OrderId,
		"symbol":      symbol,
		"status":      status,
		"avgPrice":    avgPrice,
		"executedQty": fillQty,
		"side":        order.Side,
		"type":        order.OrderType,
		"time":        cTime,
		"updateTime":  uTime,
		"commission":  -fee,
	}, nil
}

// GetTrades retrieves trade history from Bitget (unified interface, returns TradeRecord)
// Converts internal BitgetTrade format to standard TradeRecord format
func (t *BitgetTrader) GetTrades(startTime time.Time, limit int) ([]TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", startTime.UnixMilli()),
		"limit":       fmt.Sprintf("%d", limit),
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/fill-history", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get fill history: %w", err)
	}

	var resp struct {
		FillList []struct {
			TradeID    string `json:"tradeId"`
			Symbol     string `json:"symbol"`
			OrderID    string `json:"orderId"`
			Side       string `json:"side"`       // buy, sell
			Price      string `json:"price"`      // Fill price
			BaseVolume string `json:"baseVolume"` // Fill size in base currency
			Fee        string `json:"fee"`        // Fee (negative for cost)
			FeeCcy     string `json:"feeCcy"`     // Fee currency
			Profit     string `json:"profit"`     // Realized PnL
			CTime      string `json:"cTime"`      // Fill time (ms)
			TradeSide  string `json:"tradeSide"`  // open, close
		} `json:"fillList"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse fills: %w", err)
	}

	trades := make([]TradeRecord, 0, len(resp.FillList))

	for _, fill := range resp.FillList {
		fillPrice, _ := strconv.ParseFloat(fill.Price, 64)
		fillQty, _ := strconv.ParseFloat(fill.BaseVolume, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
		profit, _ := strconv.ParseFloat(fill.Profit, 64)
		cTime, _ := strconv.ParseInt(fill.CTime, 10, 64)

		// Determine position side from order action
		side := strings.ToUpper(fill.Side)
		tradeSide := strings.ToLower(fill.TradeSide)
		
		var positionSide string
		var orderAction string
		
		if tradeSide == "open" {
			if side == "BUY" {
				orderAction = "open_long"
				positionSide = "LONG"
			} else {
				orderAction = "open_short"
				positionSide = "SHORT"
			}
		} else if tradeSide == "close" {
			if side == "SELL" {
				orderAction = "close_long"
				positionSide = "LONG"
			} else {
				orderAction = "close_short"
				positionSide = "SHORT"
			}
		}

		trade := TradeRecord{
			TradeID:      fill.TradeID,
			Symbol:       fill.Symbol,
			Side:         side,
			PositionSide: positionSide, // Derived from side and tradeSide combination
			OrderAction:  orderAction,
			Price:        fillPrice,
			Quantity:     fillQty,
			RealizedPnL:  profit,
			Fee:          -fee, // Bitget returns negative fee
			Time:         time.UnixMilli(cTime).UTC(),
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// GetTradesForSymbol retrieves trade history for a specific symbol
// This is more reliable than using general GetTrades which may have delays
func (t *BitgetTrader) GetTradesForSymbol(symbol string, startTime time.Time, limit int) ([]TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	symbol = t.convertSymbol(symbol)

	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", startTime.UnixMilli()),
		"limit":       fmt.Sprintf("%d", limit),
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/fill-history", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get fill history for %s: %w", symbol, err)
	}

	var resp struct {
		FillList []struct {
			TradeID    string `json:"tradeId"`
			Symbol     string `json:"symbol"`
			OrderID    string `json:"orderId"`
			Side       string `json:"side"`
			Price      string `json:"price"`
			BaseVolume string `json:"baseVolume"`
			Fee        string `json:"fee"`
			FeeCcy     string `json:"feeCcy"`
			Profit     string `json:"profit"`
			CTime      string `json:"cTime"`
			TradeSide  string `json:"tradeSide"`
		} `json:"fillList"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse fills for %s: %w", symbol, err)
	}

	trades := make([]TradeRecord, 0, len(resp.FillList))

	for _, fill := range resp.FillList {
		fillPrice, _ := strconv.ParseFloat(fill.Price, 64)
		fillQty, _ := strconv.ParseFloat(fill.BaseVolume, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
		profit, _ := strconv.ParseFloat(fill.Profit, 64)
		cTime, _ := strconv.ParseInt(fill.CTime, 10, 64)

		side := strings.ToUpper(fill.Side)
		tradeSide := strings.ToLower(fill.TradeSide)

		var orderAction string
		if tradeSide == "open" {
			if side == "BUY" {
				orderAction = "open_long"
			} else {
				orderAction = "open_short"
			}
		} else if tradeSide == "close" {
			if side == "SELL" {
				orderAction = "close_long"
			} else {
				orderAction = "close_short"
			}
		}

		trade := TradeRecord{
			TradeID:      fill.TradeID,
			Symbol:       fill.Symbol,
			Side:         side,
			PositionSide: "BOTH",
			OrderAction:  orderAction,
			Price:        fillPrice,
			Quantity:     fillQty,
			RealizedPnL:  profit,
			Fee:          -fee,
			Time:         time.UnixMilli(cTime).UTC(),
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// GetTradesForSymbolFromID retrieves trade history for a specific symbol starting from a given trade ID
// This is used for incremental sync - only fetch new trades since last sync
// Note: Bitget API doesn't support fromID directly, but we can use pagination more efficiently
func (t *BitgetTrader) GetTradesForSymbolFromID(symbol string, fromID int64, limit int) ([]TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	symbol = t.convertSymbol(symbol)

	// Use a smart time window: start from 1 hour ago and expand if needed
	startTime := time.Now().Add(-1 * time.Hour)
	maxIterations := 3 // Limit iterations to prevent infinite loops

	for iteration := 0; iteration < maxIterations; iteration++ {
		trades, err := t.GetTradesForSymbol(symbol, startTime, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get trades for %s from ID %d: %w", symbol, fromID, err)
		}

		// Filter trades with ID > fromID
		filtered := make([]TradeRecord, 0, len(trades))
		foundOlderTrade := false

		for _, trade := range trades {
			tradeIDInt, err := strconv.ParseInt(trade.TradeID, 10, 64)
			if err != nil {
				// If trade ID is not numeric, use string comparison as fallback
				if trade.TradeID > strconv.FormatInt(fromID, 10) {
					filtered = append(filtered, trade)
				}
				continue
			}

			if tradeIDInt > fromID {
				filtered = append(filtered, trade)
			} else if tradeIDInt <= fromID {
				foundOlderTrade = true
			}
		}

		// If we found trades older than fromID, we have all new trades
		if foundOlderTrade || len(filtered) > 0 {
			return filtered, nil
		}

		// If no older trades found, expand time window and try again
		startTime = startTime.Add(-6 * time.Hour)
		logger.Infof("  [Bitget] Expanding time window to %s for symbol %s (iteration %d)",
			startTime.Format("01-02 15:04:05"), symbol, iteration+1)
	}

	// If still no trades found after all iterations, return empty result
	logger.Infof("  ⚠️ [Bitget] No trades found for %s after %d iterations", symbol, maxIterations)
	return []TradeRecord{}, nil
}

// GetClosedPnL retrieves closed position PnL records
func (t *BitgetTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", startTime.UnixMilli()),
		"limit":       fmt.Sprintf("%d", limit),
	}

	data, err := t.doRequest("GET", "/api/v2/mix/position/history-position", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions history: %w", err)
	}

	var resp struct {
		List []struct {
			Symbol       string `json:"symbol"`
			HoldSide     string `json:"holdSide"`
			OpenPriceAvg string `json:"openPriceAvg"`
			ClosePriceAvg string `json:"closePriceAvg"`
			CloseVol     string `json:"closeVol"`
			AchievedProfits string `json:"achievedProfits"`
			TotalFee     string `json:"totalFee"`
			Leverage     string `json:"leverage"`
			CTime        string `json:"cTime"`
			UTime        string `json:"uTime"`
		} `json:"list"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	records := make([]ClosedPnLRecord, 0, len(resp.List))
	for _, pos := range resp.List {
		record := ClosedPnLRecord{
			Symbol: pos.Symbol,
			Side:   pos.HoldSide,
		}

		record.EntryPrice, _ = strconv.ParseFloat(pos.OpenPriceAvg, 64)
		record.ExitPrice, _ = strconv.ParseFloat(pos.ClosePriceAvg, 64)
		record.Quantity, _ = strconv.ParseFloat(pos.CloseVol, 64)
		record.RealizedPnL, _ = strconv.ParseFloat(pos.AchievedProfits, 64)
		fee, _ := strconv.ParseFloat(pos.TotalFee, 64)
		record.Fee = -fee
		lev, _ := strconv.ParseFloat(pos.Leverage, 64)
		record.Leverage = int(lev)

		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)
		record.EntryTime = time.UnixMilli(cTime).UTC()
		record.ExitTime = time.UnixMilli(uTime).UTC()

		record.CloseType = "unknown"
		records = append(records, record)
	}

	return records, nil
}

// clearCache clears all caches (called after trades to ensure fresh data)
func (t *BitgetTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheTime = time.Time{} // Reset cache time
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheTime = time.Time{} // Reset cache time
	t.positionsCacheMutex.Unlock()

	t.contractsCacheMutex.Lock()
	t.contractsCache = make(map[string]*BitgetContract) // Clear contract cache
	t.contractsCacheTime = time.Time{}                  // Reset cache time
	t.contractsCacheMutex.Unlock()
}

// invalidateBalanceCache invalidates only balance cache
func (t *BitgetTrader) invalidateBalanceCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheTime = time.Time{}
	t.balanceCacheMutex.Unlock()
}

// invalidatePositionsCache invalidates only positions cache
func (t *BitgetTrader) invalidatePositionsCache() {
	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()
}

// genBitgetClientOid generates unique client order ID
func genBitgetClientOid() string {
	timestamp := time.Now().UnixNano() % 10000000000000
	rand := time.Now().Nanosecond() % 100000
	return fmt.Sprintf("nofx%d%05d", timestamp, rand)
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *BitgetTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	var result []OpenOrder

	// 1. Get pending limit orders
	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
	}
	// Only add symbol if provided (empty means get all)
	if symbol != "" {
		params["symbol"] = t.convertSymbol(symbol)
	}

	data, err := t.doRequest("GET", bitgetPendingPath, params)
	if err != nil {
		logger.Warnf("[Bitget] Failed to get pending orders: %v", err)
	}
	if err == nil && data != nil {
		var orders struct {
			EntrustedList []struct {
				OrderId      string `json:"orderId"`
				Symbol       string `json:"symbol"`
				Side         string `json:"side"`         // buy/sell
				TradeSide    string `json:"tradeSide"`    // open/close
				PosSide      string `json:"posSide"`      // long/short
				OrderType    string `json:"orderType"`    // limit/market
				Price        string `json:"price"`
				Size         string `json:"size"`
				State        string `json:"state"`
			} `json:"entrustedList"`
		}
		if err := json.Unmarshal(data, &orders); err == nil {
			for _, order := range orders.EntrustedList {
				price, _ := strconv.ParseFloat(order.Price, 64)
				quantity, _ := strconv.ParseFloat(order.Size, 64)

				// Convert side to standard format
				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)

				result = append(result, OpenOrder{
					OrderID:      order.OrderId,
					Symbol:       order.Symbol, // Use symbol from API response
					Side:         side,
					PositionSide: positionSide,
					Type:         strings.ToUpper(order.OrderType),
					Price:        price,
					StopPrice:    0,
					Quantity:     quantity,
					Status:       "NEW",
				})
			}
		}
	}

	// 2. Get pending plan orders (normal plan orders like trailing stop)
	normalPlanParams := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"planType":    "normal_plan",
	}
	if symbol != "" {
		normalPlanParams["symbol"] = t.convertSymbol(symbol)
	}

	normalPlanData, err := t.doRequest("GET", "/api/v2/mix/order/orders-plan-pending", normalPlanParams)
	if err != nil {
		logger.Warnf("[Bitget] Failed to get normal plan orders: %v", err)
	}
	
	if normalPlanData != nil {
		var planOrders struct {
			EntrustedList []struct {
				OrderId       string `json:"orderId"`
				Symbol        string `json:"symbol"`
				Side          string `json:"side"`
				PosSide       string `json:"posSide"`
				PlanType      string `json:"planType"`
				TriggerPrice  string `json:"triggerPrice"`
				Size          string `json:"size"`
			} `json:"entrustedList"`
		}
		if err := json.Unmarshal(normalPlanData, &planOrders); err == nil {
			for _, order := range planOrders.EntrustedList {
				triggerPrice, _ := strconv.ParseFloat(order.TriggerPrice, 64)
				quantity, _ := strconv.ParseFloat(order.Size, 64)

				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)

				result = append(result, OpenOrder{
					OrderID:      order.OrderId,
					Symbol:       order.Symbol, // Use symbol from API response
					Side:         side,
					PositionSide: positionSide,
					Type:         "STOP_MARKET",
					Price:        0,
					StopPrice:    triggerPrice,
					Quantity:     quantity,
					Status:       "NEW",
				})
			}
		}
	}

	// 3. Get pending stop-loss/take-profit orders using planType=profit_loss
	// This includes: profit_plan, loss_plan, moving_plan, pos_profit, pos_loss
	tpslParams := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"planType":    "profit_loss",
	}
	if symbol != "" {
		tpslParams["symbol"] = t.convertSymbol(symbol)
	}

	tpslData, err := t.doRequest("GET", "/api/v2/mix/order/orders-plan-pending", tpslParams)
	if err != nil {
		logger.Warnf("[Bitget] Failed to get TPSL orders: %v", err)
	}
	
	if tpslData != nil {
		var tpslOrders struct {
			EntrustedList []struct {
				OrderId       string `json:"orderId"`
				Symbol        string `json:"symbol"`
				PlanType      string `json:"planType"` // profit_plan/loss_plan/pos_profit/pos_loss/moving_plan
				TriggerPrice  string `json:"triggerPrice"`
				Side          string `json:"side"`    // buy/sell
				PosSide       string `json:"posSide"` // long/short/net
				Size          string `json:"size"`
			} `json:"entrustedList"`
		}
		if err := json.Unmarshal(tpslData, &tpslOrders); err == nil {
			for _, order := range tpslOrders.EntrustedList {
				triggerPrice, _ := strconv.ParseFloat(order.TriggerPrice, 64)
				quantity, _ := strconv.ParseFloat(order.Size, 64)

				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)
				if positionSide == "NET" {
					positionSide = "BOTH"
				}

				// Validate inputs
				if symbol == "" || quantity <= 0 || stopPrice <= 0 {
					return fmt.Errorf("invalid stop loss parameters: symbol=%s, quantity=%.4f, stopPrice=%.4f", symbol, quantity, stopPrice)
				}

				// Map Bitget plan type to order type
				orderType := "STOP_MARKET"
				if strings.Contains(order.PlanType, "profit") {
					orderType = "TAKE_PROFIT_MARKET"
				}

				result = append(result, OpenOrder{
					OrderID:      order.OrderId,
					Symbol:       order.Symbol, // Use symbol from API response
					Side:         side,
					PositionSide: positionSide,
					Type:         orderType,
					Price:        0,
					StopPrice:    triggerPrice,
					Quantity:     quantity,
					Status:       "NEW",
				})
			}
		}
	}

	if symbol == "" {
		logger.Infof("✓ BITGET GetOpenOrders: found %d open orders (all symbols)", len(result))
	} else {
		logger.Infof("✓ BITGET GetOpenOrders: found %d open orders for %s", len(result), symbol)
	}
	return result, nil
}

// PlaceLimitOrder places a limit order for grid trading
// Implements GridTrader interface
func (t *BitgetTrader) PlaceLimitOrder(req *LimitOrderRequest) (*LimitOrderResult, error) {
	symbol := t.convertSymbol(req.Symbol)

	// Set leverage if specified
	if req.Leverage > 0 {
		if err := t.SetLeverage(symbol, req.Leverage); err != nil {
			logger.Warnf("[Bitget] Failed to set leverage: %v", err)
		}
	}

	// Format quantity
	qtyStr, _ := t.FormatQuantity(symbol, req.Quantity)

	// Determine side and position side for dual position mode
				// Validate inputs
				if symbol == "" || quantity <= 0 || takeProfitPrice <= 0 {
					return fmt.Errorf("invalid take profit parameters: symbol=%s, quantity=%.4f, takeProfitPrice=%.4f", symbol, quantity, takeProfitPrice)
				}

	side := "buy"
	posSide := "long"
	tradeSide := "open"
	
	if req.Side == "SELL" {
		side = "sell"
	}
	
	// Determine position side based on PositionSide if specified
	if req.PositionSide != "" {
		if strings.ToUpper(req.PositionSide) == "SHORT" {
			posSide = "short"
		} else {
			posSide = "long"
		}
	}
	
	// If ReduceOnly is true, this is a closing order
	if req.ReduceOnly {
		tradeSide = "close"
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        side,
		"posSide":     posSide,
		"tradeSide":   tradeSide,
		"orderType":   "limit",
		"size":        qtyStr,
		"price":       fmt.Sprintf("%.8f", req.Price),
		"force":       "GTC", // Good Till Cancel
		"clientOid":   genBitgetClientOid(),
	}

	// Add reduce only if specified
	if req.ReduceOnly {
		body["reduceOnly"] = "YES"
	}

	logger.Infof("[Bitget] PlaceLimitOrder: %s %s @ %.4f, qty=%s", symbol, side, req.Price, qtyStr)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		// If dual position mode fails, try single position mode
		if strings.Contains(err.Error(), "40774") || strings.Contains(err.Error(), "unilateral position") {
			logger.Infof("  🔄 Dual position failed, trying single position mode...")
			// Remove dual position parameters
			delete(body, "posSide")
			delete(body, "tradeSide")
			data, err = t.doRequest("POST", bitgetOrderPath, body)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to place limit order: %w", err)
		}
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ [Bitget] Limit order placed: %s %s @ %.4f, orderID=%s",
		symbol, side, req.Price, order.OrderId)

	return &LimitOrderResult{
		OrderID:      order.OrderId,
		ClientID:     order.ClientOid,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       "NEW",
	}, nil
}

// CancelOrder cancels a specific order by ID
// Implements GridTrader interface
func (t *BitgetTrader) CancelOrder(symbol, orderID string) error {
	symbol = t.convertSymbol(symbol)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"orderId":     orderID,
	}

	_, err := t.doRequest("POST", "/api/v2/mix/order/cancel-order", body)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ [Bitget] Order cancelled: %s %s", symbol, orderID)
	return nil
}

// GetOrderBook gets the order book for a symbol
// Implements GridTrader interface
func (t *BitgetTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	symbol = t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v2/mix/market/depth?symbol=%s&productType=USDT-FUTURES&limit=%d", symbol, depth)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	var result struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	// Parse bids
	for _, b := range result.Bids {
		if len(b) >= 2 {
			price, _ := strconv.ParseFloat(b[0], 64)
			qty, _ := strconv.ParseFloat(b[1], 64)
			bids = append(bids, []float64{price, qty})
		}
	}

	// Parse asks
	for _, a := range result.Asks {
		if len(a) >= 2 {
			price, _ := strconv.ParseFloat(a[0], 64)
			qty, _ := strconv.ParseFloat(a[1], 64)
			asks = append(asks, []float64{price, qty})
		}
	}

	return bids, asks, nil
}

// PlaceOrder is a wrapper for PlaceLimitOrder (API compatibility method)
func (t *BitgetTrader) PlaceOrder(symbol string, side string, quantity float64, price float64, orderType string) (map[string]interface{}, error) {
	req := &LimitOrderRequest{
		Symbol:   symbol,
		Side:     side,
		Quantity: quantity,
		Price:    price,
	}
	result, err := t.PlaceLimitOrder(req)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"orderId": result.OrderID,
		"symbol":  symbol,
		"status":  "NEW",
	}, nil
}

// ModifyOrder modifies an existing order (not implemented for Bitget - requires CancelOrder + PlaceOrder)
func (t *BitgetTrader) ModifyOrder(symbol, orderID string, quantity float64, price float64) (map[string]interface{}, error) {
	logger.Infof("⚠️ [Bitget] ModifyOrder not directly supported - use CancelOrder + PlaceOrder instead")
	return nil, fmt.Errorf("ModifyOrder not directly supported for Bitget")
}

// ClosePositionPartial closes a partial position (market order)
func (t *BitgetTrader) ClosePositionPartial(symbol string, quantity float64) (map[string]interface{}, error) {
	// Get current positions to determine side
	positions, err := t.GetPositions()
	if err != nil {
		return nil, err
	}

	for _, pos := range positions {
		if pos["symbol"] == symbol {
			side := pos["side"].(string)
			if side == "long" {
				return t.CloseLong(symbol, quantity)
			} else if side == "short" {
				return t.CloseShort(symbol, quantity)
			}
		}
	}

	return nil, fmt.Errorf("no open position found for %s", symbol)
}

// SetMultipleStopLoss sets multiple stop loss tiers (not directly supported - sets single SL)
func (t *BitgetTrader) SetMultipleStopLoss(symbol string, positionSide string, quantity float64, tiers []float64) ([]map[string]interface{}, error) {
	if len(tiers) == 0 {
		return nil, fmt.Errorf("no tiers provided")
	}
	// Use the first tier as the stop loss level
	stopPrice := tiers[0]
	logger.Infof("⚠️ [Bitget] SetMultipleStopLoss: Using first tier (%.6f) only, multiple tiers not supported", stopPrice)
	err := t.SetStopLoss(symbol, positionSide, quantity, stopPrice)
	if err != nil {
		return nil, err
	}
	// Return as array of map results for consistency
	results := []map[string]interface{}{
		{
			"order_id":   fmt.Sprintf("%s_%s_sl", symbol, positionSide),
			"symbol":     symbol,
			"stop_price": stopPrice,
			"status":     "success",
		},
	}
	return results, nil
}

// SetMultipleTakeProfit sets multiple take profit tiers (not directly supported - sets single TP)
func (t *BitgetTrader) SetMultipleTakeProfit(symbol string, positionSide string, quantity float64, tiers []float64) ([]map[string]interface{}, error) {
	if len(tiers) == 0 {
		return nil, fmt.Errorf("no tiers provided")
	}
	// Use the first tier as the take profit level
	tpPrice := tiers[0]
	logger.Infof("⚠️ [Bitget] SetMultipleTakeProfit: Using first tier (%.6f) only, multiple tiers not supported", tpPrice)
	err := t.SetTakeProfit(symbol, positionSide, quantity, tpPrice)
	if err != nil {
		return nil, err
	}
	// Return as array of map results for consistency
	results := []map[string]interface{}{
		{
			"order_id":          fmt.Sprintf("%s_%s_tp", symbol, positionSide),
			"symbol":            symbol,
			"take_profit_price": tpPrice,
			"status":            "success",
		},
	}
	return results, nil
}

// ModifyStopLossTier modifies a specific stop loss tier (not supported - requires cancel + new)
func (t *BitgetTrader) ModifyStopLossTier(symbol string, tierLevel int, newPrice float64) (map[string]interface{}, error) {
	logger.Infof("⚠️ [Bitget] ModifyStopLossTier not directly supported - use CancelStopLossOrders + SetStopLoss instead")
	return nil, fmt.Errorf("ModifyStopLossTier not directly supported for Bitget")
}

// ModifyTakeProfitTier modifies a specific take profit tier (not supported - requires cancel + new)
func (t *BitgetTrader) ModifyTakeProfitTier(symbol string, tierLevel int, newPrice float64) (map[string]interface{}, error) {
	logger.Infof("⚠️ [Bitget] ModifyTakeProfitTier not directly supported - use CancelTakeProfitOrders + SetTakeProfit instead")
	return nil, fmt.Errorf("ModifyTakeProfitTier not directly supported for Bitget")
}

// GetCommissionSymbols returns symbols that have new trades since lastSyncTime
// Bitget doesn't have commission-specific API, so we use fill history to detect active symbols
func (t *BitgetTrader) GetCommissionSymbols(lastSyncTime time.Time) ([]string, error) {
	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", lastSyncTime.UnixMilli()),
		"limit":       "100", // Limit to avoid too many API calls
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/fill-history", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get commission symbols: %w", err)
	}

	var resp struct {
		FillList []struct {
			Symbol string `json:"symbol"`
			CTime  string `json:"cTime"`
		} `json:"fillList"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse fills for commission detection: %w", err)
	}

	symbolMap := make(map[string]bool)
	for _, fill := range resp.FillList {
		if fill.Symbol != "" {
			symbolMap[fill.Symbol] = true
		}
	}

	var symbols []string
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// GetPnLSymbols returns symbols that have realized PnL records since lastSyncTime
// This is a fallback when commission detection fails
func (t *BitgetTrader) GetPnLSymbols(lastSyncTime time.Time) ([]string, error) {
	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", lastSyncTime.UnixMilli()),
		"limit":       "100",
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/fill-history", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get PnL symbols: %w", err)
	}

	var resp struct {
		FillList []struct {
			Symbol string `json:"symbol"`
			Profit string `json:"profit"` // Realized PnL
		} `json:"fillList"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse fills for PnL detection: %w", err)
	}

	symbolMap := make(map[string]bool)
	for _, fill := range resp.FillList {
		if fill.Symbol != "" {
			profit, _ := strconv.ParseFloat(fill.Profit, 64)
			// Only include symbols with actual PnL (closing trades)
			if profit != 0 {
				symbolMap[fill.Symbol] = true
			}
		}
	}

	var symbols []string
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// getPositionSymbols returns list of symbols that have active positions
// Used as fallback when commission detection fails
func (t *BitgetTrader) getPositionSymbols() []string {
	positions, err := t.GetPositions()
	if err != nil {
		return nil
	}

// GetPnLSymbols returns symbols that have realized PnL records since lastSyncTime
// This is a fallback when commission detection fails
func (t *BitgetTrader) GetPnLSymbols(lastSyncTime time.Time) ([]string, error) {
	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", lastSyncTime.UnixMilli()),
		"limit":       "100",
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/fill-history", params)
	if err != nil {
		// Rate limiting and "Too Many Requests" errors should be reported gracefully
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Too Many Requests") {
			logger.Warnf("⏳ [Bitget] Rate limited while getting PnL symbols: %v (will retry next cycle)", err)
		} else if strings.Contains(err.Error(), "40309") || strings.Contains(err.Error(), "The symbol has been removed") {
			// This is a data fetch issue, not specific to a symbol
			logger.Warnf("⚠️ [Bitget] API error during PnL lookup (might be API issue): %v", err)
		}
		return nil, err
	}

	var resp struct {
		FillList []struct {
			Symbol string `json:"symbol"`
			Profit string `json:"profit"` // Realized PnL
		} `json:"fillList"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		logger.Warnf("⚠️ [Bitget] Failed to parse fills for PnL detection: %v", err)
		return nil, fmt.Errorf("failed to parse fills for PnL detection: %w", err)
	}

	symbolMap := make(map[string]bool)
	for _, fill := range resp.FillList {
		if fill.Symbol == "" {
			continue
		}
		
		profit, err := strconv.ParseFloat(fill.Profit, 64)
		if err != nil {
			logger.Warnf("⚠️ [Bitget] Failed to parse profit for %s: %v", fill.Symbol, err)
			continue
		}
		
		// Only include symbols with actual realized PnL (closing trades have non-zero PnL)
		if profit != 0 {
			symbolMap[fill.Symbol] = true
		}
	}

	var symbols []string
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}
	var symbols []string
	for _, pos := range positions {
		if symbol, ok := pos["symbol"].(string); ok && symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// determineOrderAction determines the order action based on trade data
// Bitget uses tradeSide (open/close) which makes this easier than Binance
func (t *BitgetTrader) determineOrderAction(side, tradeSide string, realizedPnL float64) string {
	side = strings.ToUpper(side)
	tradeSide = strings.ToLower(tradeSide)

	// Bitget explicitly provides tradeSide, so we can use it directly
	if tradeSide == "open" {
		if side == "BUY" {
			return "open_long"
		} else {
			return "open_short"
		}
	} else if tradeSide == "close" {
		if side == "SELL" {
			return "close_long"
		} else {
			return "close_short"
		}
	}

	// Fallback: use PnL to determine if it's a close trade
	isClose := realizedPnL != 0
	if side == "BUY" {
		if isClose {
			return "close_short"
		}
		return "open_long"
	} else {
		if isClose {
			return "close_long"
		}
		return "open_short"
	}
}
