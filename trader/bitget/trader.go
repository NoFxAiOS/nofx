package bitget

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
	"nofx/trader/types"
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
	// Bitget V2 uses plan order for stop loss
	symbol = t.convertSymbol(symbol)

	side := "sell"
	holdSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		holdSide = "short"
	}

	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"planType":     "loss_plan",
		"symbol":       symbol,
		"productType":  "USDT-FUTURES",
		"marginMode":   "crossed",
		"marginCoin":   "USDT",
		"triggerPrice": fmt.Sprintf("%.8f", stopPrice),
		"triggerType":  "mark_price",
		"side":         side,
		"tradeSide":    "close",
		"orderType":    "market",
		"size":         qtyStr,
		"holdSide":     holdSide,
		"clientOid":    genBitgetClientOid(),
	}

	_, err := t.doRequest("POST", "/api/v2/mix/order/place-plan-order", body)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("  ✓ [Bitget] Stop loss set: %s @ %.4f", symbol, stopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *BitgetTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// Bitget V2 uses plan order for take profit
	symbol = t.convertSymbol(symbol)

	side := "sell"
	holdSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		holdSide = "short"
	}

	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	body := map[string]interface{}{
		"planType":     "profit_plan",
		"symbol":       symbol,
		"productType":  "USDT-FUTURES",
		"marginMode":   "crossed",
		"marginCoin":   "USDT",
		"triggerPrice": fmt.Sprintf("%.8f", takeProfitPrice),
		"triggerType":  "mark_price",
		"side":         side,
		"tradeSide":    "close",
		"orderType":    "market",
		"size":         qtyStr,
		"holdSide":     holdSide,
		"clientOid":    genBitgetClientOid(),
	}

	_, err := t.doRequest("POST", "/api/v2/mix/order/place-plan-order", body)
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
func (t *BitgetTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
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

	records := make([]types.ClosedPnLRecord, 0, len(resp.List))
	for _, pos := range resp.List {
		record := types.ClosedPnLRecord{
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
func (t *BitgetTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	symbol = t.convertSymbol(symbol)
	var result []types.OpenOrder

	// 1. Get pending limit orders
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
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

				result = append(result, types.OpenOrder{
					OrderID:      order.OrderId,
					Symbol:       symbol,
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

	// 2. Get pending plan orders (stop-loss/take-profit)
	// Bitget V2 API requires planType parameter: profit_loss for SL/TP orders
	planParams := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"planType":    "profit_loss",
	}

	planData, err := t.doRequest("GET", "/api/v2/mix/order/orders-plan-pending", planParams)
	if err != nil {
		logger.Warnf("[Bitget] Failed to get plan orders: %v", err)
	}
	if err == nil && planData != nil {
		var planOrders struct {
			EntrustedList []struct {
				OrderId                 string `json:"orderId"`
				Symbol                  string `json:"symbol"`
				Side                    string `json:"side"`
				PosSide                 string `json:"posSide"`
				PlanType                string `json:"planType"` // pos_loss, pos_profit
				TriggerPrice            string `json:"triggerPrice"`
				StopLossTriggerPrice    string `json:"stopLossTriggerPrice"`
				StopSurplusTriggerPrice string `json:"stopSurplusTriggerPrice"`
				Size                    string `json:"size"`
				PlanStatus              string `json:"planStatus"`
			} `json:"entrustedList"`
		}
		if err := json.Unmarshal(planData, &planOrders); err == nil {
			for _, order := range planOrders.EntrustedList {
				// Filter by symbol if specified
				if symbol != "" && order.Symbol != symbol {
					continue
				}

				// Determine trigger price based on plan type
				var triggerPrice float64
				orderType := "STOP_MARKET"

				if order.PlanType == "pos_profit" {
					// Take profit order
					orderType = "TAKE_PROFIT_MARKET"
					if order.StopSurplusTriggerPrice != "" {
						triggerPrice, _ = strconv.ParseFloat(order.StopSurplusTriggerPrice, 64)
					} else {
						triggerPrice, _ = strconv.ParseFloat(order.TriggerPrice, 64)
					}
				} else {
					// Stop loss order (pos_loss)
					if order.StopLossTriggerPrice != "" {
						triggerPrice, _ = strconv.ParseFloat(order.StopLossTriggerPrice, 64)
					} else {
						triggerPrice, _ = strconv.ParseFloat(order.TriggerPrice, 64)
					}
				}

				quantity, _ := strconv.ParseFloat(order.Size, 64)
				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)

				result = append(result, types.OpenOrder{
					OrderID:      order.OrderId,
					Symbol:       order.Symbol,
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

	logger.Infof("✓ BITGET GetOpenOrders: found %d open orders for %s", len(result), symbol)
	return result, nil
}

// PlaceLimitOrder places a limit order for grid trading
// Implements GridTrader interface
func (t *BitgetTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	symbol := t.convertSymbol(req.Symbol)

	// Set leverage if specified
	if req.Leverage > 0 {
		if err := t.SetLeverage(symbol, req.Leverage); err != nil {
			logger.Warnf("[Bitget] Failed to set leverage: %v", err)
		}
	}

	// Format quantity
	qtyStr, _ := t.FormatQuantity(symbol, req.Quantity)

	// Determine side
	side := "buy"
	if req.Side == "SELL" {
		side = "sell"
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        side,
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
		return nil, fmt.Errorf("failed to place limit order: %w", err)
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

	return &types.LimitOrderResult{
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
