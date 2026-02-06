package kucoin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KuCoin Futures API endpoints
const (
	kucoinBaseURL          = "https://api-futures.kucoin.com"
	kucoinAccountPath      = "/api/v1/account-overview"
	kucoinPositionPath     = "/api/v1/positions"
	kucoinOrderPath        = "/api/v1/orders"
	kucoinLeveragePath     = "/api/v1/position/margin/leverage"
	kucoinTickerPath       = "/api/v1/ticker"
	kucoinContractsPath    = "/api/v1/contracts/active"
	kucoinCancelOrderPath  = "/api/v1/orders"
	kucoinStopOrderPath    = "/api/v1/stopOrders"
	kucoinCancelStopPath   = "/api/v1/stopOrders"
	kucoinPositionModePath = "/api/v1/position/margin/auto-deposit-status"
	kucoinFillsPath        = "/api/v1/fills"
	kucoinRecentFillsPath  = "/api/v1/recentFills"
)

// API channel configuration
const (
	kcPartnerID  = "NoFxFutures"
	kcPartnerKey = "d7c05b0c-c81b-4630-8fa8-ca6d049d3aae"
)

// KuCoinTrader implements types.Trader interface for KuCoin Futures
type KuCoinTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// HTTP client
	httpClient *http.Client

	// Server time offset (local - server) in milliseconds
	serverTimeOffset int64
	serverTimeMutex  sync.RWMutex

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Positions cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Contract info cache
	contractsCache      map[string]*KuCoinContract
	contractsCacheTime  time.Time
	contractsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// KuCoinContract represents contract info
type KuCoinContract struct {
	Symbol          string  // Symbol
	BaseCurrency    string  // Base currency
	Multiplier      float64 // Contract multiplier
	LotSize         float64 // Minimum order quantity (lot size)
	TickSize        float64 // Minimum price increment
	MaxOrderQty     float64 // Maximum order quantity
	MaxLeverage     float64 // Maximum leverage
	MarkPrice       float64 // Current mark price
	IsInverse       bool    // Is inverse contract
	QuoteCurrency   string  // Quote currency
	IndexPriceScale int     // Index price decimal places
}

// KuCoinResponse represents KuCoin API response
type KuCoinResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// NewKuCoinTrader creates a new KuCoin trader instance
func NewKuCoinTrader(apiKey, secretKey, passphrase string) *KuCoinTrader {
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
	}

	trader := &KuCoinTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		passphrase:     passphrase,
		httpClient:     httpClient,
		cacheDuration:  15 * time.Second,
		contractsCache: make(map[string]*KuCoinContract),
	}

	// Sync server time on initialization
	if err := trader.syncServerTime(); err != nil {
		logger.Warnf("⚠️ Failed to sync KuCoin server time: %v (will retry on first request)", err)
	}

	logger.Infof("✓ KuCoin Futures trader initialized")
	return trader
}

// syncServerTime fetches KuCoin server time and calculates offset
func (t *KuCoinTrader) syncServerTime() error {
	resp, err := t.httpClient.Get(kucoinBaseURL + "/api/v1/timestamp")
	if err != nil {
		return fmt.Errorf("failed to get server time: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Code string `json:"code"`
		Data int64  `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != "200000" {
		return fmt.Errorf("server time API error: %s", result.Code)
	}

	serverTime := result.Data
	localTime := time.Now().UnixMilli()
	offset := localTime - serverTime

	t.serverTimeMutex.Lock()
	t.serverTimeOffset = offset
	t.serverTimeMutex.Unlock()

	logger.Infof("✓ KuCoin time synced: offset=%dms (local %d - server %d)", offset, localTime, serverTime)
	return nil
}

// getTimestamp returns the current timestamp adjusted for server time offset
func (t *KuCoinTrader) getTimestamp() string {
	t.serverTimeMutex.RLock()
	offset := t.serverTimeOffset
	t.serverTimeMutex.RUnlock()

	// Subtract offset to get server time from local time
	timestamp := time.Now().UnixMilli() - offset
	return strconv.FormatInt(timestamp, 10)
}

// sign generates KuCoin API signature
func (t *KuCoinTrader) sign(timestamp, method, requestPath, body string) string {
	// KuCoin signature: base64(HMAC-SHA256(timestamp + method + endpoint + body, secretKey))
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// signPassphrase signs the passphrase with API v2
func (t *KuCoinTrader) signPassphrase(passphrase string) string {
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(passphrase))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// signPartner generates partner signature: base64(HMAC-SHA256(timestamp + partner + apiKey, partnerKey))
func (t *KuCoinTrader) signPartner(timestamp string) string {
	preHash := timestamp + kcPartnerID + t.apiKey
	h := hmac.New(sha256.New, []byte(kcPartnerKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest executes HTTP request
func (t *KuCoinTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request body: %w", err)
		}
	}

	timestamp := t.getTimestamp()
	signature := t.sign(timestamp, method, path, string(bodyBytes))
	signedPassphrase := t.signPassphrase(t.passphrase)

	req, err := http.NewRequest(method, kucoinBaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Authentication headers
	req.Header.Set("KC-API-KEY", t.apiKey)
	req.Header.Set("KC-API-SIGN", signature)
	req.Header.Set("KC-API-TIMESTAMP", timestamp)
	req.Header.Set("KC-API-PASSPHRASE", signedPassphrase)
	req.Header.Set("KC-API-KEY-VERSION", "3")
	req.Header.Set("Content-Type", "application/json")

	// Partner headers
	req.Header.Set("KC-API-PARTNER", kcPartnerID)
	req.Header.Set("KC-API-PARTNER-SIGN", t.signPartner(timestamp))
	req.Header.Set("KC-API-PARTNER-VERIFY", "true")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var kcResp KuCoinResponse
	if err := json.Unmarshal(respBody, &kcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(respBody))
	}

	if kcResp.Code != "200000" {
		// If timestamp error, try to re-sync server time
		if kcResp.Code == "400002" || strings.Contains(kcResp.Msg, "TIMESTAMP") {
			logger.Warnf("⚠️ KuCoin timestamp error, re-syncing server time...")
			if err := t.syncServerTime(); err != nil {
				logger.Warnf("⚠️ Failed to re-sync server time: %v", err)
			}
		}
		return nil, fmt.Errorf("KuCoin API error: code=%s, msg=%s", kcResp.Code, kcResp.Msg)
	}

	return kcResp.Data, nil
}

// convertSymbol converts generic symbol to KuCoin format
// e.g. BTCUSDT -> XBTUSDTM (KuCoin uses XBT for BTC)
func (t *KuCoinTrader) convertSymbol(symbol string) string {
	// Remove USDT suffix
	base := strings.TrimSuffix(symbol, "USDT")
	// KuCoin uses XBT instead of BTC
	if base == "BTC" {
		base = "XBT"
	}
	return fmt.Sprintf("%sUSDTM", base)
}

// convertSymbolBack converts KuCoin format back to generic symbol
// e.g. XBTUSDTM -> BTCUSDT
func (t *KuCoinTrader) convertSymbolBack(kcSymbol string) string {
	// Remove M suffix
	sym := strings.TrimSuffix(kcSymbol, "M")
	// Convert XBT back to BTC
	if strings.HasPrefix(sym, "XBT") {
		sym = "BTC" + strings.TrimPrefix(sym, "XBT")
	}
	return sym
}

// GetBalance gets account balance
func (t *KuCoinTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	data, err := t.doRequest("GET", kucoinAccountPath+"?currency=USDT", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	var account struct {
		AccountEquity    float64 `json:"accountEquity"`
		UnrealisedPNL    float64 `json:"unrealisedPNL"`
		MarginBalance    float64 `json:"marginBalance"`
		PositionMargin   float64 `json:"positionMargin"`
		OrderMargin      float64 `json:"orderMargin"`
		FrozenFunds      float64 `json:"frozenFunds"`
		AvailableBalance float64 `json:"availableBalance"`
		Currency         string  `json:"currency"`
	}

	if err := json.Unmarshal(data, &account); err != nil {
		return nil, fmt.Errorf("failed to parse balance data: %w", err)
	}

	result := map[string]interface{}{
		"totalWalletBalance":    account.MarginBalance,        // Wallet balance (without unrealized PnL)
		"availableBalance":      account.AvailableBalance,
		"totalUnrealizedProfit": account.UnrealisedPNL,
		"total_equity":          account.AccountEquity,
		"totalEquity":           account.AccountEquity,        // For GetAccountInfo compatibility
	}

	logger.Infof("✓ KuCoin balance: Total equity=%.2f, Available=%.2f, Unrealized PnL=%.2f",
		account.AccountEquity, account.AvailableBalance, account.UnrealisedPNL)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions gets all positions
func (t *KuCoinTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	data, err := t.doRequest("GET", kucoinPositionPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []struct {
		Symbol           string  `json:"symbol"`
		CurrentQty       int64   `json:"currentQty"`      // Position quantity (in lots, integer)
		AvgEntryPrice    float64 `json:"avgEntryPrice"`   // Average entry price (string in API)
		MarkPrice        float64 `json:"markPrice"`       // Mark price
		UnrealisedPnl    float64 `json:"unrealisedPnl"`   // Unrealized PnL
		Leverage         float64 `json:"leverage"`        // Leverage setting
		RealLeverage     float64 `json:"realLeverage"`    // Effective leverage (may be nil in cross mode)
		LiquidationPrice float64 `json:"liquidationPrice"`// Liquidation price
		Multiplier       float64 `json:"multiplier"`      // Contract multiplier
		IsOpen           bool    `json:"isOpen"`
		CrossMode        bool    `json:"crossMode"`
		OpeningTimestamp int64   `json:"openingTimestamp"`
		SettleCurrency   string  `json:"settleCurrency"`
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		if !pos.IsOpen || pos.CurrentQty == 0 {
			continue
		}

		// Convert symbol format
		symbol := t.convertSymbolBack(pos.Symbol)

		// Determine side based on position quantity
		// KuCoin: positive qty = long, negative qty = short
		side := "long"
		qty := pos.CurrentQty
		if qty < 0 {
			side = "short"
			qty = -qty
		}

		// Convert lots to actual quantity using multiplier
		// Position quantity = lots * multiplier
		multiplier := pos.Multiplier
		if multiplier == 0 {
			multiplier = 0.001 // Default for BTC
		}
		positionAmt := float64(qty) * multiplier

		// Determine margin mode
		mgnMode := "isolated"
		if pos.CrossMode {
			mgnMode = "cross"
		}

		// Use Leverage field (setting), fallback to RealLeverage (effective), default to 10
		leverage := pos.Leverage
		if leverage == 0 {
			leverage = pos.RealLeverage
		}
		if leverage == 0 {
			leverage = 10 // Default leverage
		}

		posMap := map[string]interface{}{
			"symbol":           symbol,
			"positionAmt":      positionAmt,
			"entryPrice":       pos.AvgEntryPrice,
			"markPrice":        pos.MarkPrice,
			"unRealizedProfit": pos.UnrealisedPnl,
			"leverage":         leverage,
			"liquidationPrice": pos.LiquidationPrice,
			"side":             side,
			"mgnMode":          mgnMode,
			"createdTime":      pos.OpeningTimestamp,
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

// InvalidatePositionCache clears the position cache
func (t *KuCoinTrader) InvalidatePositionCache() {
	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()
}

// getContract gets contract info
func (t *KuCoinTrader) getContract(symbol string) (*KuCoinContract, error) {
	kcSymbol := t.convertSymbol(symbol)

	// Check cache
	t.contractsCacheMutex.RLock()
	if contract, ok := t.contractsCache[kcSymbol]; ok && time.Since(t.contractsCacheTime) < 5*time.Minute {
		t.contractsCacheMutex.RUnlock()
		return contract, nil
	}
	t.contractsCacheMutex.RUnlock()

	// Get contract info
	data, err := t.doRequest("GET", kucoinContractsPath, nil)
	if err != nil {
		return nil, err
	}

	var contracts []struct {
		Symbol        string  `json:"symbol"`
		BaseCurrency  string  `json:"baseCurrency"`
		Multiplier    float64 `json:"multiplier"`
		LotSize       int64   `json:"lotSize"`
		TickSize      float64 `json:"tickSize"`
		MaxOrderQty   int64   `json:"maxOrderQty"`
		MaxLeverage   int     `json:"maxLeverage"`
		MarkPrice     float64 `json:"markPrice"`
		IsInverse     bool    `json:"isInverse"`
		QuoteCurrency string  `json:"quoteCurrency"`
	}

	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, err
	}

	// Update cache with all contracts
	t.contractsCacheMutex.Lock()
	for _, c := range contracts {
		t.contractsCache[c.Symbol] = &KuCoinContract{
			Symbol:        c.Symbol,
			BaseCurrency:  c.BaseCurrency,
			Multiplier:    c.Multiplier,
			LotSize:       float64(c.LotSize),
			TickSize:      c.TickSize,
			MaxOrderQty:   float64(c.MaxOrderQty),
			MaxLeverage:   float64(c.MaxLeverage),
			MarkPrice:     c.MarkPrice,
			IsInverse:     c.IsInverse,
			QuoteCurrency: c.QuoteCurrency,
		}
	}
	t.contractsCacheTime = time.Now()
	t.contractsCacheMutex.Unlock()

	// Return requested contract
	t.contractsCacheMutex.RLock()
	contract, ok := t.contractsCache[kcSymbol]
	t.contractsCacheMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("contract info not found: %s", kcSymbol)
	}

	return contract, nil
}

// quantityToLots converts quantity (in base asset) to lots
func (t *KuCoinTrader) quantityToLots(symbol string, quantity float64) (int64, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		return 0, err
	}

	// lots = quantity / multiplier
	lots := quantity / contract.Multiplier

	// Round to integer (KuCoin uses integer lots)
	lotsInt := int64(math.Round(lots))

	// Check max order quantity
	if contract.MaxOrderQty > 0 && float64(lotsInt) > contract.MaxOrderQty {
		logger.Infof("⚠️ KuCoin order quantity %d exceeds max %d, reducing to max", lotsInt, int64(contract.MaxOrderQty))
		lotsInt = int64(contract.MaxOrderQty)
	}

	return lotsInt, nil
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
			Symbol       string  `json:"symbol"`
			OpenPrice    float64 `json:"avgEntryPrice"`
			ClosePrice   float64 `json:"avgClosePrice"`
			Qty          int64   `json:"qty"`
			RealisedPnl  float64 `json:"realisedGrossCost"`
			CloseTime    int64   `json:"closeTime"`
			OpenTime     int64   `json:"openTime"`
			PositionId   string  `json:"id"`
			CloseType    string  `json:"type"`
			Leverage     int     `json:"leverage"`
			SettleCurrency string `json:"settleCurrency"`
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
			Id       string  `json:"id"`
			Symbol   string  `json:"symbol"`
			Side     string  `json:"side"`
			Type     string  `json:"type"`
			Price    string  `json:"price"`
			Size     int64   `json:"size"`
			StopType string  `json:"stopType"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		// Try alternate format
		var items []struct {
			Id       string  `json:"id"`
			Symbol   string  `json:"symbol"`
			Side     string  `json:"side"`
			Type     string  `json:"type"`
			Price    string  `json:"price"`
			Size     int64   `json:"size"`
			StopType string  `json:"stopType"`
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
