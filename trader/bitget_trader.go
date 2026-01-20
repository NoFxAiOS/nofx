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
	}

	// Set one-way position mode (net mode)
	if err := trader.setPositionMode(); err != nil {
		logger.Infof("⚠️ Failed to set Bitget position mode: %v (ignore if already set)", err)
	}

	logger.Infof("🟢 [Bitget] Trader initialized")

	return trader
}

// setPositionMode sets one-way position mode
func (t *BitgetTrader) setPositionMode() error {
	body := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"posMode":     "one_way_mode",
	}

	_, err := t.doRequest("POST", bitgetPositionModePath, body)
	if err != nil {
		if strings.Contains(err.Error(), "same") || strings.Contains(err.Error(), "already") {
			return nil
		}
		return err
	}

	logger.Infof("  ✓ Bitget account switched to one-way position mode")
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

// convertSymbol converts generic symbol to Bitget format
// e.g., BTCUSDT -> BTCUSDT
func (t *BitgetTrader) convertSymbol(symbol string) string {
	// Bitget uses same format as input, just ensure uppercase
	return strings.ToUpper(symbol)
}

// deconvertSymbol converts Bitget symbol format back to standard format
func (t *BitgetTrader) deconvertSymbol(symbol string) string {
	// Bitget uses same format, just ensure uppercase
	return strings.ToUpper(symbol)
}

// GetBalance gets account balance
func (t *BitgetTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetAccountPath, params)
	if err != nil {
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
		return nil, fmt.Errorf("failed to parse balance data: %w, raw: %s", err, string(data))
	}

	var totalEquity, availableBalance, unrealizedPnL float64
	for _, acc := range accounts {
		if acc.MarginCoin == "USDT" {
			totalEquity, _ = strconv.ParseFloat(acc.AccountEquity, 64)
			availableBalance, _ = strconv.ParseFloat(acc.Available, 64)
			unrealizedPnL, _ = strconv.ParseFloat(acc.UnrealizedPL, 64)
			logger.Infof("✓ [Bitget] Balance: equity=%.2f, available=%.2f", totalEquity, availableBalance)
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

// GetPositions gets all positions
func (t *BitgetTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
	}

	data, err := t.doRequest("GET", bitgetPositionPath, params)
	if err != nil {
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
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		total, _ := strconv.ParseFloat(pos.Total, 64)
		if total == 0 {
			continue
		}

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
		if strings.Contains(err.Error(), "same") || strings.Contains(err.Error(), "already") {
			return nil
		}
		if strings.Contains(err.Error(), "position") {
			logger.Infof("  ⚠️ %s has positions, cannot change margin mode", symbol)
			return nil
		}
		return err
	}

	logger.Infof("  ✓ %s margin mode set to %s", symbol, marginMode)
	return nil
}

// SetLeverage sets leverage
func (t *BitgetTrader) SetLeverage(symbol string, leverage int) error {
	symbol = t.convertSymbol(symbol)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"leverage":    fmt.Sprintf("%d", leverage),
	}

	_, err := t.doRequest("POST", bitgetLeveragePath, body)
	if err != nil {
		if strings.Contains(err.Error(), "same") {
			return nil
		}
		logger.Infof("  ⚠️ Failed to set %s leverage: %v", symbol, err)
		return err
	}

	logger.Infof("  ✓ %s leverage set to %dx", symbol, leverage)
	return nil
}

// OpenLong opens long position
func (t *BitgetTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// Cancel old orders first
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	// Format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        "buy",
		"orderType":   "market",
		"size":        qtyStr,
		"clientOid":   genBitgetClientOid(),
	}

	logger.Infof("  📊 Bitget OpenLong: symbol=%s, qty=%s, leverage=%d", symbol, qtyStr, leverage)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Clear cache
	t.clearCache()

	logger.Infof("✓ Bitget opened long position successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort opens short position
func (t *BitgetTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// Cancel old orders first
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	// Format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        "sell",
		"orderType":   "market",
		"size":        qtyStr,
		"clientOid":   genBitgetClientOid(),
	}

	logger.Infof("  📊 Bitget OpenShort: symbol=%s, qty=%s, leverage=%d", symbol, qtyStr, leverage)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Clear cache
	t.clearCache()

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

	// If quantity is 0, get current position
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
		"orderType":   "market",
		"size":        qtyStr,
		"reduceOnly":  "YES",
		"clientOid":   genBitgetClientOid(),
	}

	logger.Infof("  📊 Bitget CloseLong: symbol=%s, qty=%s", symbol, qtyStr)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	var order struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}

	// Clear cache
	t.clearCache()

	logger.Infof("✓ Bitget closed long position successfully: %s", symbol)

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
		"orderType":   "market",
		"size":        qtyStr,
		"reduceOnly":  "YES",
		"clientOid":   genBitgetClientOid(),
	}

	logger.Infof("  📊 Bitget CloseShort: symbol=%s, qty=%s", symbol, qtyStr)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	var order struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}

	// Clear cache
	t.clearCache()

	logger.Infof("✓ Bitget closed short position successfully: %s", symbol)

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

	// For one-way position mode, determine order direction for stop loss
	// Long position stop loss = sell, Short position stop loss = buy
	positionSideLower := strings.ToLower(positionSide)
	holdSide := "sell" // Default for long position
	if positionSideLower == "short" {
		holdSide = "buy" // Short position stop loss is buy
	}

	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"symbol":              symbol,
		"productType":         "USDT-FUTURES",
		"marginCoin":          "USDT",
		"planType":            "loss_plan",
		"triggerPrice":        t.FormatPrice(symbol, stopPrice),
		"triggerType":         "mark_price",
		"executePrice":        "0",
		"holdSide":            holdSide,
		"size":                qtyStr,
		"clientOid":           genBitgetClientOid(),
	}

	_, err := t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("  ✓ [Bitget] Stop loss set: %s @ %.4f", symbol, stopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *BitgetTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// Bitget V2 uses TPSL order for take profit
	symbol = t.convertSymbol(symbol)

	// For one-way position mode, determine order direction for take profit
	// Long position take profit = sell, Short position take profit = buy
	positionSideLower := strings.ToLower(positionSide)
	holdSide := "sell" // Default for long position
	if positionSideLower == "short" {
		holdSide = "buy" // Short position take profit is buy
	}

	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"symbol":              symbol,
		"productType":         "USDT-FUTURES",
		"marginCoin":          "USDT",
		"planType":            "profit_plan",
		"triggerPrice":        t.FormatPrice(symbol, takeProfitPrice),
		"triggerType":         "mark_price",
		"executePrice":        "0",
		"holdSide":            holdSide,
		"size":                qtyStr,
		"clientOid":           genBitgetClientOid(),
	}

	_, err := t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	logger.Infof("  ✓ [Bitget] Take profit set: %s @ %.4f", symbol, takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop loss orders
func (t *BitgetTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelPlanOrders(symbol, "loss_plan")
}

// CancelTakeProfitOrders cancels take profit orders
func (t *BitgetTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelPlanOrders(symbol, "profit_plan")
}

// cancelPlanOrders cancels plan orders
func (t *BitgetTrader) cancelPlanOrders(symbol string, planType string) error {
	symbol = t.convertSymbol(symbol)

	// Get pending plan orders
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"planType":    planType,
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

	// Also cancel plan orders
	t.cancelPlanOrders(symbol, "loss_plan")
	t.cancelPlanOrders(symbol, "profit_plan")

	return nil
}

// CancelStopOrders cancels stop loss and take profit orders
func (t *BitgetTrader) CancelStopOrders(symbol string) error {
	t.CancelStopLossOrders(symbol)
	t.CancelTakeProfitOrders(symbol)
	return nil
}

// FormatQuantity formats quantity
func (t *BitgetTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		return fmt.Sprintf("%.4f", quantity), nil
	}

	// Format according to volume precision
	format := fmt.Sprintf("%%.%df", contract.VolumePlace)
	return fmt.Sprintf(format, quantity), nil
}

// FormatPrice formats price according to contract precision
func (t *BitgetTrader) FormatPrice(symbol string, price float64) string {
	contract, err := t.getContract(symbol)
	if err != nil {
		// Default to 2 decimal places if contract info not available
		return fmt.Sprintf("%.2f", price)
	}

	// Format according to price precision
	format := fmt.Sprintf("%%.%df", contract.PricePlace)
	return fmt.Sprintf(format, price)
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

// GetClosedPnL retrieves closed position PnL records
func (t *BitgetTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	// Bitget requires endTime to be after startTime
	endTime := time.Now()
	
	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", startTime.UnixMilli()),
		"endTime":     fmt.Sprintf("%d", endTime.UnixMilli()),
		"limit":       fmt.Sprintf("%d", limit), // 官方文档使用 limit，不是 pageSize
	}

	logger.Infof("📊 [Bitget] GetClosedPnL request: startTime=%s, endTime=%s, limit=%d", 
		startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"), limit)

	data, err := t.doRequest("GET", "/api/v2/mix/position/history-position", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions history: %w", err)
	}

	// Debug: print raw response
	logger.Infof("📊 [Bitget] History position raw response: %s", string(data))

	var resp struct {
		List []struct {
			PositionId     string `json:"positionId"`
			Symbol         string `json:"symbol"`
			MarginCoin     string `json:"marginCoin"`
			HoldSide       string `json:"holdSide"`        // long/short
			PosMode        string `json:"posMode"`         // one_way_mode/hedge_mode
			OpenAvgPrice   string `json:"openAvgPrice"`    // 开仓均价
			CloseAvgPrice  string `json:"closeAvgPrice"`   // 平仓均价
			MarginMode     string `json:"marginMode"`      // isolated/crossed
			OpenTotalPos   string `json:"openTotalPos"`    // 累计开仓数量
			CloseTotalPos  string `json:"closeTotalPos"`   // 累计已平仓数量
			Pnl            string `json:"pnl"`             // 已实现盈亏
			NetProfit      string `json:"netProfit"`       // 净盈亏
			TotalFunding   string `json:"totalFunding"`    // 累计资金费用
			OpenFee        string `json:"openFee"`         // 开仓手续费
			CloseFee       string `json:"closeFee"`        // 平仓手续费
			CTime          string `json:"ctime"`           // 创建时间（毫秒时间戳）
			UTime          string `json:"utime"`           // 更新时间（毫秒时间戳）
		} `json:"list"`
		EndId string `json:"endId"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Infof("📊 [Bitget] Parsed %d history positions", len(resp.List))

	logger.Infof("📊 [Bitget] Parsed %d history positions", len(resp.List))

	records := make([]ClosedPnLRecord, 0, len(resp.List))
	for _, pos := range resp.List {
		// Skip if essential fields are missing
		if pos.Symbol == "" || pos.CloseTotalPos == "" {
			logger.Infof("⚠️ [Bitget] Skipping invalid history position: symbol=%s, closeTotalPos=%s", pos.Symbol, pos.CloseTotalPos)
			continue
		}

		record := ClosedPnLRecord{
			Symbol: pos.Symbol,
			Side:   pos.HoldSide, // long/short
		}

		// 使用官方文档的字段名
		record.EntryPrice, _ = strconv.ParseFloat(pos.OpenAvgPrice, 64)
		record.ExitPrice, _ = strconv.ParseFloat(pos.CloseAvgPrice, 64)
		record.Quantity, _ = strconv.ParseFloat(pos.CloseTotalPos, 64)
		
		// 净盈亏 = 已实现盈亏 - 手续费
		netProfit, _ := strconv.ParseFloat(pos.NetProfit, 64)
		record.RealizedPnL = netProfit
		
		// 总手续费 = 开仓手续费 + 平仓手续费
		openFee, _ := strconv.ParseFloat(pos.OpenFee, 64)
		closeFee, _ := strconv.ParseFloat(pos.CloseFee, 64)
		record.Fee = openFee + closeFee
		
		// Bitget 不返回杠杆信息，使用默认值
		record.Leverage = 10

		// 时间戳转换（毫秒）
		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)
		record.EntryTime = time.UnixMilli(cTime).UTC()
		record.ExitTime = time.UnixMilli(uTime).UTC()

		record.CloseType = "unknown"
		
		logger.Infof("✅ [Bitget] History position: %s %s, entry=%.2f, exit=%.2f, qty=%.4f, netProfit=%.2f, fee=%.2f",
			record.Symbol, record.Side, record.EntryPrice, record.ExitPrice, record.Quantity, record.RealizedPnL, record.Fee)
		
		records = append(records, record)
	}

	logger.Infof("📊 [Bitget] Returning %d history position records", len(records))
	return records, nil
}

// clearCache clears all caches
func (t *BitgetTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
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
	symbol = t.convertSymbol(symbol)

	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetPendingPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders struct {
		EntrustedList []struct {
			OrderId      string `json:"orderId"`
			Side         string `json:"side"`
			OrderType    string `json:"orderType"`
			Price        string `json:"price"`
			BaseVolume   string `json:"baseVolume"`
			TriggerPrice string `json:"triggerPrice"`
		} `json:"entrustedList"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse orders: %w", err)
	}

	var result []OpenOrder
	for _, order := range orders.EntrustedList {
		price, _ := strconv.ParseFloat(order.Price, 64)
		qty, _ := strconv.ParseFloat(order.BaseVolume, 64)
		stopPrice, _ := strconv.ParseFloat(order.TriggerPrice, 64)

		openOrder := OpenOrder{
			OrderID:  order.OrderId,
			Symbol:   symbol,
			Side:     order.Side,
			Type:     order.OrderType,
			Price:    price,
			StopPrice: stopPrice,
			Quantity: qty,
			Status:   "NEW",
		}
		result = append(result, openOrder)
	}

	return result, nil
}

// GetAllOpenOrders gets all open/pending orders across all symbols in the account
// This is useful when you want to see all pending orders without specifying symbol
func (t *BitgetTrader) GetAllOpenOrders() ([]OpenOrder, error) {
	// Use empty symbol to get all pending orders for the account
	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetPendingPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get all open orders: %w", err)
	}

	var orders struct {
		EntrustedList []struct {
			OrderId      string `json:"orderId"`
			Symbol       string `json:"symbol"`
			Side         string `json:"side"`
			OrderType    string `json:"orderType"`
			Price        string `json:"price"`
			BaseVolume   string `json:"baseVolume"`
			TriggerPrice string `json:"triggerPrice"`
		} `json:"entrustedList"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse all orders: %w", err)
	}

	var result []OpenOrder
	for _, order := range orders.EntrustedList {
		price, _ := strconv.ParseFloat(order.Price, 64)
		qty, _ := strconv.ParseFloat(order.BaseVolume, 64)
		stopPrice, _ := strconv.ParseFloat(order.TriggerPrice, 64)

		openOrder := OpenOrder{
			OrderID:  order.OrderId,
			Symbol:   t.deconvertSymbol(order.Symbol), // Convert back from exchange format
			Side:     order.Side,
			Type:     order.OrderType,
			Price:    price,
			StopPrice: stopPrice,
			Quantity: qty,
			Status:   "NEW",
		}
		result = append(result, openOrder)
	}

	return result, nil
}// PlaceOrder places a limit order or market order
// orderType: "limit" or "market"
// side: "buy" or "sell"
func (t *BitgetTrader) PlaceOrder(symbol string, side string, quantity float64, price float64, orderType string) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)
	side = strings.ToLower(side)
	orderType = strings.ToUpper(orderType)

	// Validate inputs
	if side != "buy" && side != "sell" {
		return nil, fmt.Errorf("invalid side: %s, must be buy or sell", side)
	}

	if orderType != "LIMIT" && orderType != "MARKET" {
		return nil, fmt.Errorf("invalid orderType: %s, must be LIMIT or MARKET", orderType)
	}

	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"marginMode":  "crossed",
		"side":        side,
		"orderType":   orderType,
		"size":        qtyStr,
		"clientOid":   genBitgetClientOid(),
	}

	// For limit orders, set price
	if orderType == "LIMIT" {
		body["price"] = t.FormatPrice(symbol, price)
	}

	logger.Infof("  📊 Bitget PlaceOrder: symbol=%s, side=%s, qty=%s, price=%.4f, type=%s", 
		symbol, side, qtyStr, price, orderType)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Bitget order placed successfully: %s", order.OrderId)

	return map[string]interface{}{
		"orderId":   order.OrderId,
		"symbol":    symbol,
		"side":      side,
		"quantity":  quantity,
		"price":     price,
		"type":      orderType,
		"status":    "NEW",
		"timestamp": time.Now().UnixMilli(),
	}, nil
}

// ModifyOrder modifies an existing pending order
// quantity and price can be 0 to keep unchanged
func (t *BitgetTrader) ModifyOrder(symbol string, orderID string, quantity float64, price float64) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// Get current order details
	orderStatus, err := t.GetOrderStatus(symbol, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order details: %w", err)
	}

	// If quantity or price not specified, use current values
	if quantity == 0 {
		quantity = orderStatus["executedQty"].(float64)
	}
	if price == 0 {
		price = orderStatus["avgPrice"].(float64)
	}

	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	priceStr := t.FormatPrice(symbol, price)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"orderId":     orderID,
		"newSize":     qtyStr,
		"newPrice":    priceStr,
	}

	logger.Infof("  📊 Bitget ModifyOrder: orderId=%s, newQty=%s, newPrice=%s", orderID, qtyStr, priceStr)

	data, err := t.doRequest("POST", "/api/v2/mix/order/amend-order", body)
	if err != nil {
		return nil, fmt.Errorf("failed to modify order: %w", err)
	}

	var result struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse modify response: %w", err)
	}

	logger.Infof("✓ Bitget order modified successfully: %s", result.OrderId)

	return map[string]interface{}{
		"orderId":   result.OrderId,
		"symbol":    symbol,
		"quantity":  quantity,
		"price":     price,
		"status":    "MODIFIED",
		"timestamp": time.Now().UnixMilli(),
	}, nil
}

// CancelOrder cancels a specific pending order
func (t *BitgetTrader) CancelOrder(symbol string, orderID string) error {
	symbol = t.convertSymbol(symbol)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"orderId":     orderID,
	}

	logger.Infof("  📊 Bitget CancelOrder: orderId=%s", orderID)

	_, err := t.doRequest("POST", bitgetCancelOrderPath, body)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "already") {
			logger.Infof("  ⚠️ Order %s not found or already canceled", orderID)
			return nil
		}
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ Bitget order canceled successfully: %s", orderID)
	return nil
}

// ClosePositionPartial closes part of a position
// quantity: amount to close (> 0)
func (t *BitgetTrader) ClosePositionPartial(symbol string, quantity float64) (map[string]interface{}, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than 0")
	}

	symbol = t.convertSymbol(symbol)

	// Get current position
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var currentPos map[string]interface{}
	for _, pos := range positions {
		if posSymbol, ok := pos["symbol"].(string); ok && posSymbol == symbol {
			currentPos = pos
			break
		}
	}

	if currentPos == nil {
		return nil, fmt.Errorf("position not found for symbol: %s", symbol)
	}

	positionAmt := currentPos["positionAmt"].(float64)
	side := currentPos["side"].(string)

	// Validate quantity doesn't exceed position
	if quantity > positionAmt {
		return nil, fmt.Errorf("close quantity %.4f exceeds position size %.4f", quantity, positionAmt)
	}

	// Determine close side
	var closeSide string
	if side == "long" {
		closeSide = "sell"
	} else {
		closeSide = "buy"
	}

	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"side":        closeSide,
		"orderType":   "market",
		"size":        qtyStr,
		"clientOid":   genBitgetClientOid(),
	}

	logger.Infof("  📊 Bitget ClosePositionPartial: symbol=%s, qty=%s, side=%s", symbol, qtyStr, closeSide)

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close position partially: %w", err)
	}

	var order struct {
		OrderId string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	t.clearCache()

	logger.Infof("✓ Bitget partial position closed successfully: %s", symbol)

	return map[string]interface{}{
		"orderId":   order.OrderId,
		"symbol":    symbol,
		"quantity":  quantity,
		"side":      closeSide,
		"status":    "FILLED",
		"timestamp": time.Now().UnixMilli(),
	}, nil
}

// SetMultipleStopLoss sets multiple stop loss orders at different price levels
// stopPrices: list of stop loss prices
func (t *BitgetTrader) SetMultipleStopLoss(symbol string, positionSide string, quantity float64, stopPrices []float64) ([]map[string]interface{}, error) {
	if len(stopPrices) == 0 {
		return nil, fmt.Errorf("stopPrices cannot be empty")
	}

	symbol = t.convertSymbol(symbol)
	results := make([]map[string]interface{}, 0, len(stopPrices))

	// Cancel existing stop loss orders first
	t.CancelStopLossOrders(symbol)

	// Set each stop loss price
	for i, stopPrice := range stopPrices {
		if err := t.SetStopLoss(symbol, positionSide, quantity/float64(len(stopPrices)), stopPrice); err != nil {
			logger.Warnf("  ⚠️ Failed to set stop loss %d: %v", i+1, err)
			continue
		}

		results = append(results, map[string]interface{}{
			"symbol":    symbol,
			"level":     i + 1,
			"stopPrice": stopPrice,
			"quantity":  quantity / float64(len(stopPrices)),
			"status":    "PENDING",
		})
	}

	logger.Infof("✓ Bitget set %d multiple stop loss orders for %s", len(results), symbol)
	return results, nil
}

// SetMultipleTakeProfit sets multiple take profit orders at different price levels
// takeProfitPrices: list of take profit prices
func (t *BitgetTrader) SetMultipleTakeProfit(symbol string, positionSide string, quantity float64, takeProfitPrices []float64) ([]map[string]interface{}, error) {
	if len(takeProfitPrices) == 0 {
		return nil, fmt.Errorf("takeProfitPrices cannot be empty")
	}

	symbol = t.convertSymbol(symbol)
	results := make([]map[string]interface{}, 0, len(takeProfitPrices))

	// Cancel existing take profit orders first
	t.CancelTakeProfitOrders(symbol)

	// Set each take profit price
	for i, tpPrice := range takeProfitPrices {
		if err := t.SetTakeProfit(symbol, positionSide, quantity/float64(len(takeProfitPrices)), tpPrice); err != nil {
			logger.Warnf("  ⚠️ Failed to set take profit %d: %v", i+1, err)
			continue
		}

		results = append(results, map[string]interface{}{
			"symbol":         symbol,
			"level":          i + 1,
			"takeProfitPrice": tpPrice,
			"quantity":       quantity / float64(len(takeProfitPrices)),
			"status":         "PENDING",
		})
	}

	logger.Infof("✓ Bitget set %d multiple take profit orders for %s", len(results), symbol)
	return results, nil
}

// ModifyStopLossTier modifies a specific stop loss order tier
// tierIndex: 0-based index of the tier to modify (0=first, 1=second, etc)
func (t *BitgetTrader) ModifyStopLossTier(symbol string, tierIndex int, stopPrice float64) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// Get pending stop loss orders
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"planType":    "loss_plan",
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/orders-plan-pending", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get stop loss orders: %w", err)
	}

	var orders struct {
		EntrustedList []struct {
			OrderId   string `json:"orderId"`
			Size      string `json:"size"`
			TriggerPrice string `json:"triggerPrice"`
		} `json:"entrustedList"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse orders: %w", err)
	}

	if tierIndex < 0 || tierIndex >= len(orders.EntrustedList) {
		return nil, fmt.Errorf("tierIndex %d out of range [0, %d)", tierIndex, len(orders.EntrustedList))
	}

	targetOrder := orders.EntrustedList[tierIndex]
	qty, _ := strconv.ParseFloat(targetOrder.Size, 64)

	// Cancel the old order
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"orderId":     targetOrder.OrderId,
	}
	t.doRequest("POST", "/api/v2/mix/order/cancel-plan-order", body)

	// Create new order with new price
	// Get position side from current position
	positions, _ := t.GetPositions()
	var positionSide string
	for _, pos := range positions {
		if posSymbol, ok := pos["symbol"].(string); ok && posSymbol == symbol {
			side, ok := pos["side"].(string)
			if ok {
				positionSide = side
			}
			break
		}
	}

	if positionSide == "" {
		positionSide = "long"
	}

	if err := t.SetStopLoss(symbol, positionSide, qty, stopPrice); err != nil {
		return nil, fmt.Errorf("failed to set new stop loss order: %w", err)
	}

	logger.Infof("✓ Bitget modified stop loss tier %d for %s to %.4f", tierIndex+1, symbol, stopPrice)

	return map[string]interface{}{
		"symbol":     symbol,
		"tier":       tierIndex + 1,
		"stopPrice":  stopPrice,
		"quantity":   qty,
		"status":     "PENDING",
		"timestamp":  time.Now().UnixMilli(),
	}, nil
}

// ModifyTakeProfitTier modifies a specific take profit order tier
// tierIndex: 0-based index of the tier to modify (0=first, 1=second, etc)
func (t *BitgetTrader) ModifyTakeProfitTier(symbol string, tierIndex int, takeProfitPrice float64) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// Get pending take profit orders
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"planType":    "profit_plan",
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/orders-plan-pending", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get take profit orders: %w", err)
	}

	var orders struct {
		EntrustedList []struct {
			OrderId   string `json:"orderId"`
			Size      string `json:"size"`
			TriggerPrice string `json:"triggerPrice"`
		} `json:"entrustedList"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse orders: %w", err)
	}

	if tierIndex < 0 || tierIndex >= len(orders.EntrustedList) {
		return nil, fmt.Errorf("tierIndex %d out of range [0, %d)", tierIndex, len(orders.EntrustedList))
	}

	targetOrder := orders.EntrustedList[tierIndex]
	qty, _ := strconv.ParseFloat(targetOrder.Size, 64)

	// Cancel the old order
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"orderId":     targetOrder.OrderId,
	}
	t.doRequest("POST", "/api/v2/mix/order/cancel-plan-order", body)

	// Create new order with new price
	// Get position side from current position
	positions, _ := t.GetPositions()
	var positionSide string
	for _, pos := range positions {
		if posSymbol, ok := pos["symbol"].(string); ok && posSymbol == symbol {
			side, ok := pos["side"].(string)
			if ok {
				positionSide = side
			}
			break
		}
	}

	if positionSide == "" {
		positionSide = "long"
	}

	if err := t.SetTakeProfit(symbol, positionSide, qty, takeProfitPrice); err != nil {
		return nil, fmt.Errorf("failed to set new take profit order: %w", err)
	}

	logger.Infof("✓ Bitget modified take profit tier %d for %s to %.4f", tierIndex+1, symbol, takeProfitPrice)

	return map[string]interface{}{
		"symbol":          symbol,
		"tier":            tierIndex + 1,
		"takeProfitPrice": takeProfitPrice,
		"quantity":        qty,
		"status":          "PENDING",
		"timestamp":       time.Now().UnixMilli(),
	}, nil
}
