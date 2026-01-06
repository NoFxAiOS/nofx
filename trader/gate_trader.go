package trader

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Gate.io API endpoints
const (
	gateBaseURL              = "https://api.gateio.ws"
	gateAccountPath          = "/api/v4/futures/usdt/accounts"
	gatePositionPath         = "/api/v4/futures/usdt/positions"
	gateOrderPath            = "/api/v4/futures/usdt/orders"
	gateLeveragePath         = "/api/v4/futures/usdt/positions/%s/leverage"
	gateMarginModePath       = "/api/v4/futures/usdt/positions/%s/margin_mode"
	gateTickerPath           = "/api/v4/futures/usdt/tickers"
	gateContractsPath        = "/api/v4/futures/usdt/contracts"
	gateCancelOrderPath      = "/api/v4/futures/usdt/orders/%s"
	gatePriceOrdersPath      = "/api/v4/futures/usdt/price_orders"
	gateCancelPriceOrderPath = "/api/v4/futures/usdt/price_orders/%s"
)

// GateTrader Gate.io futures trader
type GateTrader struct {
	apiKey    string
	secretKey string

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
	contractsCache      map[string]*GateContract
	contractsCacheTime  time.Time
	contractsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// GateContract Gate.io contract info
type GateContract struct {
	Name             string  // Contract name (e.g., "BTC_USDT")
	QuantoMultiplier float64 // Contract size multiplier
	OrderSizeMin     int64   // Minimum order size
	OrderSizeMax     int64   // Maximum order size
	OrderPriceRound  float64 // Price precision
}

// NewGateTrader creates a Gate.io trader
func NewGateTrader(apiKey, secretKey string) *GateTrader {
	// Gate.io may require proxy in some regions (e.g., China) due to network restrictions
	// Using DefaultTransport which respects system proxy settings
	// Increased timeout to 60s for better reliability in poor network conditions
	httpClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: http.DefaultTransport,
	}

	trader := &GateTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		httpClient:     httpClient,
		cacheDuration:  15 * time.Second,
		contractsCache: make(map[string]*GateContract),
	}

	logger.Infof("🔵 [Gate.io] Trader initialized (API: %s, Timeout: 60s)", gateBaseURL)

	return trader
}

// sign generates Gate.io API signature
func (t *GateTrader) sign(method, path, queryString, bodyPayload string, timestamp int64) string {
	// Gate.io signature = HEX(HMAC_SHA512(payload, secretKey))
	// payload = method + "\n" + path + "\n" + query + "\n" + hashBody + "\n" + timestamp

	// Hash body with SHA512
	hasher := sha512.New()
	hasher.Write([]byte(bodyPayload))
	bodyHash := hex.EncodeToString(hasher.Sum(nil))

	payload := fmt.Sprintf("%s\n%s\n%s\n%s\n%d",
		method,
		path,
		queryString,
		bodyHash,
		timestamp,
	)

	mac := hmac.New(sha512.New, []byte(t.secretKey))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// doRequest executes HTTP request with signature
func (t *GateTrader) doRequest(method, path string, query map[string]string, body interface{}) ([]byte, error) {
	timestamp := time.Now().Unix()

	// Build query string
	queryString := ""
	if len(query) > 0 {
		var parts []string
		for k, v := range query {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		queryString = strings.Join(parts, "&")
	}

	// Build body
	bodyPayload := ""
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyPayload = string(jsonData)
		reqBody = bytes.NewReader(jsonData)
	}

	// Generate signature
	signature := t.sign(method, path, queryString, bodyPayload, timestamp)

	// Build URL
	url := gateBaseURL + path
	if queryString != "" {
		url += "?" + queryString
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("KEY", t.apiKey)
	req.Header.Set("Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("SIGN", signature)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		// Check if it's a timeout error
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return nil, fmt.Errorf("Gate.io API request timeout (consider using proxy if in restricted region): %w", err)
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode >= 400 {
		var errResp struct {
			Label   string `json:"label"`
			Message string `json:"message"`
		}
		json.Unmarshal(bodyBytes, &errResp)
		return nil, fmt.Errorf("API error %d: %s - %s", resp.StatusCode, errResp.Label, errResp.Message)
	}

	return bodyBytes, nil
}

// GetBalance returns account balance
func (t *GateTrader) GetBalance() (map[string]interface{}, error) {
	t.balanceCacheMutex.RLock()
	if time.Since(t.balanceCacheTime) < t.cacheDuration && t.cachedBalance != nil {
		defer t.balanceCacheMutex.RUnlock()
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	query := map[string]string{
		"settle": "usdt",
	}

	data, err := t.doRequest("GET", gateAccountPath, query, nil)
	if err != nil {
		return nil, err
	}

	logger.Infof("🔵 [Gate.io] GetBalance raw response: %s", string(data))

	var account struct {
		Total          string `json:"total"`
		UnrealisedPnl  string `json:"unrealised_pnl"`
		PositionMargin string `json:"position_margin"`
		OrderMargin    string `json:"order_margin"`
		Available      string `json:"available"`
	}

	if err := json.Unmarshal(data, &account); err != nil {
		return nil, fmt.Errorf("failed to parse balance: %w", err)
	}

	logger.Infof("🔵 [Gate.io] Parsed account: Total=%s, Available=%s, UnrealisedPnl=%s",
		account.Total, account.Available, account.UnrealisedPnl)

	// Use decimal for precise financial calculations
	totalBalanceDec, err := decimal.NewFromString(account.Total)
	if err != nil {
		logger.Warnf("⚠️  [Gate.io] Failed to parse Total balance '%s': %v", account.Total, err)
		totalBalanceDec = decimal.Zero
	}

	availableBalanceDec, err := decimal.NewFromString(account.Available)
	if err != nil {
		logger.Warnf("⚠️  [Gate.io] Failed to parse Available balance '%s': %v", account.Available, err)
		availableBalanceDec = decimal.Zero
	}

	unrealizedPnLDec, err := decimal.NewFromString(account.UnrealisedPnl)
	if err != nil {
		logger.Warnf("⚠️  [Gate.io] Failed to parse UnrealisedPnl '%s': %v", account.UnrealisedPnl, err)
		unrealizedPnLDec = decimal.Zero
	}

	// Convert to float64 for interface compatibility
	totalBalance, _ := totalBalanceDec.Float64()
	availableBalance, _ := availableBalanceDec.Float64()
	unrealizedPnL, _ := unrealizedPnLDec.Float64()

	result := map[string]interface{}{
		// Standard fields (camelCase) for compatibility with AutoTrader.GetAccountInfo
		"totalEquity":           totalBalance,     // Total account value
		"availableBalance":      availableBalance, // Available balance
		"totalUnrealizedProfit": unrealizedPnL,    // Unrealized PnL

		// Legacy fields (snake_case) for backward compatibility
		"total_equity":      totalBalance,
		"available_balance": availableBalance,
		"unrealized_pnl":    unrealizedPnL,
	}

	logger.Infof("🔵 [Gate.io] Balance result: totalEquity=%.2f, available=%.2f, unrealizedPnL=%.2f",
		totalBalance, availableBalance, unrealizedPnL)

	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions returns all positions
func (t *GateTrader) GetPositions() ([]map[string]interface{}, error) {
	t.positionsCacheMutex.RLock()
	if time.Since(t.positionsCacheTime) < t.cacheDuration && t.cachedPositions != nil {
		defer t.positionsCacheMutex.RUnlock()
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	query := map[string]string{
		"settle": "usdt",
	}

	data, err := t.doRequest("GET", gatePositionPath, query, nil)
	if err != nil {
		return nil, err
	}

	logger.Infof("🔵 [Gate.io] GetPositions raw response: %s", string(data))

	var positions []struct {
		Contract         string `json:"contract"`
		Size             int64  `json:"size"`
		EntryPrice       string `json:"entry_price"`
		MarkPrice        string `json:"mark_price"`
		UnrealisedPnl    string `json:"unrealised_pnl"`
		Leverage         string `json:"leverage"`
		LiquidationPrice string `json:"liq_price"` // Gate.io uses liq_price
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse positions: %w", err)
	}

	logger.Infof("🔵 [Gate.io] Parsed %d positions", len(positions))

	var result []map[string]interface{}
	for _, pos := range positions {
		if pos.Size == 0 {
			continue
		}

		side := "long"
		quantity := float64(pos.Size)
		if pos.Size < 0 {
			side = "short"
			quantity = -quantity
		}

		// Use decimal for precise financial calculations
		entryPriceDec, _ := decimal.NewFromString(pos.EntryPrice)
		markPriceDec, _ := decimal.NewFromString(pos.MarkPrice)
		unrealizedPnlDec, _ := decimal.NewFromString(pos.UnrealisedPnl)
		leverageDec, _ := decimal.NewFromString(pos.Leverage)
		liquidationPriceDec, _ := decimal.NewFromString(pos.LiquidationPrice)

		// Convert to float64 for interface compatibility
		entryPrice, _ := entryPriceDec.Float64()
		markPrice, _ := markPriceDec.Float64()
		unrealizedPnl, _ := unrealizedPnlDec.Float64()
		leverage, _ := leverageDec.Float64()
		liquidationPrice, _ := liquidationPriceDec.Float64()

		// Calculate PnL percentage using decimal for precision
		pnlPct := 0.0
		if !entryPriceDec.IsZero() {
			if side == "long" {
				pnlPct, _ = markPriceDec.Sub(entryPriceDec).Div(entryPriceDec).Mul(decimal.NewFromInt(100)).Float64()
			} else {
				pnlPct, _ = entryPriceDec.Sub(markPriceDec).Div(entryPriceDec).Mul(decimal.NewFromInt(100)).Float64()
			}
		}

		result = append(result, map[string]interface{}{
			// Standard fields (camelCase) for compatibility with AutoTrader.GetAccountInfo
			"symbol":           pos.Contract,
			"side":             side,
			"markPrice":        markPrice,         // Mark price (camelCase)
			"positionAmt":      float64(pos.Size), // Position amount with sign (camelCase)
			"entryPrice":       entryPrice,
			"unRealizedProfit": unrealizedPnl, // Unrealized PnL (camelCase)
			"leverage":         int(leverage),
			"liquidationPrice": liquidationPrice, // Liquidation price

			// Additional fields
			"quantity":           quantity,      // Absolute quantity
			"unrealized_pnl":     unrealizedPnl, // Legacy snake_case
			"unrealized_pnl_pct": pnlPct,
			"entry_price":        entryPrice, // Legacy snake_case
			"mark_price":         markPrice,  // Legacy snake_case
		})
	}

	logger.Infof("🔵 [Gate.io] Positions result: %d active positions", len(result))

	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// OpenLong opens a long position
func (t *GateTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	// First cancel all pending orders for this symbol (clean up old stop-loss and take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders (may not have any): %v", err)
	}
	// Also cancel conditional orders (stop-loss/take-profit) - Gate keeps them separate
	if err := t.CancelStopOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old stop orders (may not have any): %v", err)
	}

	// Check if contract is supported
	supported, err := t.isContractSupported(symbol)
	if err != nil {
		logger.Warnf("[Gate.io] Failed to verify contract support: %v", err)
		// Continue anyway, will fail at order placement if not supported
	} else if !supported {
		return nil, fmt.Errorf("contract %s is not supported on Gate.io USDT perpetual futures", symbol)
	}

	// Try to set leverage (may fail if position doesn't exist yet, will use default)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("[Gate.io] SetLeverage failed (will use account default): %v", err)
	}

	// Gate.io requires size as integer (number of contracts)
	// Convert quantity carefully using decimal to avoid precision loss
	quantityDec := decimal.NewFromFloat(quantity)
	size := quantityDec.Round(0).IntPart() // Round to nearest integer

	// Gate.io minimum order size is 1 contract
	if size < 1 {
		return nil, fmt.Errorf("order quantity %.8f rounds to %d, minimum is 1 contract (increase position size or adjust price)", quantity, size)
	}

	logger.Infof("[Gate.io] OpenLong: quantity=%.8f -> size=%d", quantity, size)

	body := map[string]interface{}{
		"contract": symbol,
		"size":     size,
		"price":    "0", // Market order
		"tif":      "ioc",
	}

	query := map[string]string{
		"settle": "usdt",
	}

	data, err := t.doRequest("POST", gateOrderPath, query, body)
	if err != nil {
		return nil, err
	}

	var order struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Gate.io opened long position successfully: %s", symbol)
	logger.Infof("  Order ID: %d", order.ID)

	return map[string]interface{}{
		"order_id": strconv.FormatInt(order.ID, 10),
		"symbol":   symbol,
		"side":     "long",
		"quantity": quantity,
	}, nil
}

// OpenShort opens a short position
func (t *GateTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	// First cancel all pending orders for this symbol (clean up old stop-loss and take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders (may not have any): %v", err)
	}
	// Also cancel conditional orders (stop-loss/take-profit) - Gate keeps them separate
	if err := t.CancelStopOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old stop orders (may not have any): %v", err)
	}

	// Check if contract is supported
	supported, err := t.isContractSupported(symbol)
	if err != nil {
		logger.Warnf("[Gate.io] Failed to verify contract support: %v", err)
		// Continue anyway, will fail at order placement if not supported
	} else if !supported {
		return nil, fmt.Errorf("contract %s is not supported on Gate.io USDT perpetual futures", symbol)
	}

	// Try to set leverage (may fail if position doesn't exist yet, will use default)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("[Gate.io] SetLeverage failed (will use account default): %v", err)
	}

	// Gate.io requires size as integer (number of contracts)
	// Negative for short position
	quantityDec := decimal.NewFromFloat(quantity)
	size := -quantityDec.Round(0).IntPart() // Negative and rounded

	// Gate.io minimum order size is 1 contract
	if size > -1 {
		return nil, fmt.Errorf("order quantity %.8f rounds to %d, minimum is 1 contract (increase position size or adjust price)", quantity, size)
	}

	logger.Infof("[Gate.io] OpenShort: quantity=%.8f -> size=%d", quantity, size)

	body := map[string]interface{}{
		"contract": symbol,
		"size":     size,
		"price":    "0",
		"tif":      "ioc",
	}

	query := map[string]string{
		"settle": "usdt",
	}

	data, err := t.doRequest("POST", gateOrderPath, query, body)
	if err != nil {
		return nil, err
	}

	var order struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Gate.io opened short position successfully: %s", symbol)
	logger.Infof("  Order ID: %d", order.ID)

	return map[string]interface{}{
		"order_id": strconv.FormatInt(order.ID, 10),
		"symbol":   symbol,
		"side":     "short",
		"quantity": quantity,
	}, nil
}

// CloseLong closes a long position
func (t *GateTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	// Get current position to determine quantity if not specified
	quantityDec := decimal.NewFromFloat(quantity)
	if quantityDec.LessThanOrEqual(decimal.Zero) {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["quantity"].(float64)
				quantityDec = decimal.NewFromFloat(quantity)
				break
			}
		}
	}

	if quantityDec.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("no long position found for %s", symbol)
	}

	// Close by placing opposite order with reduce_only
	size := -quantityDec.Round(0).IntPart() // Negative to close long

	// Gate.io minimum order size is 1 contract
	if size > -1 {
		return nil, fmt.Errorf("close quantity %.8f rounds to %d, minimum is 1 contract", quantity, size)
	}

	logger.Infof("[Gate.io] CloseLong: quantity=%.8f -> size=%d", quantity, size)

	// Gate.io close position using reduce_only mode:
	// - size: negative for closing long (opposite direction)
	// - price: "0" for market order
	// - tif: "ioc" (Immediate-or-Cancel) for market execution
	// - reduce_only: true to ensure only reduce position
	body := map[string]interface{}{
		"contract":    symbol,
		"size":        size,
		"price":       "0",
		"tif":         "ioc",
		"reduce_only": true,
	}

	query := map[string]string{
		"settle": "usdt",
	}

	data, err := t.doRequest("POST", gateOrderPath, query, body)
	if err != nil {
		return nil, err
	}

	var order struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Gate.io closed long position successfully: %s", symbol)
	logger.Infof("  Order ID: %d", order.ID)

	// After closing position, cancel all pending orders for this symbol (stop-loss and take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	return map[string]interface{}{
		"order_id": strconv.FormatInt(order.ID, 10),
		"symbol":   symbol,
		"side":     "close_long",
		"quantity": quantity,
	}, nil
}

// CloseShort closes a short position
func (t *GateTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	quantityDec := decimal.NewFromFloat(quantity)
	if quantityDec.LessThanOrEqual(decimal.Zero) {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["quantity"].(float64)
				quantityDec = decimal.NewFromFloat(quantity)
				break
			}
		}
	}

	if quantityDec.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("no short position found for %s", symbol)
	}

	// Close by placing opposite order with reduce_only
	size := quantityDec.Round(0).IntPart() // Positive to close short

	// Gate.io minimum order size is 1 contract
	if size < 1 {
		return nil, fmt.Errorf("close quantity %.8f rounds to %d, minimum is 1 contract", quantity, size)
	}

	logger.Infof("[Gate.io] CloseShort: quantity=%.8f -> size=%d", quantity, size)

	// Gate.io close position using reduce_only mode:
	// - size: positive for closing short (opposite direction)
	// - price: "0" for market order
	// - tif: "ioc" (Immediate-or-Cancel) for market execution
	// - reduce_only: true to ensure only reduce position
	body := map[string]interface{}{
		"contract":    symbol,
		"size":        size,
		"price":       "0",
		"tif":         "ioc",
		"reduce_only": true,
	}

	query := map[string]string{
		"settle": "usdt",
	}

	data, err := t.doRequest("POST", gateOrderPath, query, body)
	if err != nil {
		return nil, err
	}

	var order struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Gate.io closed short position successfully: %s", symbol)
	logger.Infof("  Order ID: %d", order.ID)

	// After closing position, cancel all pending orders for this symbol (stop-loss and take-profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	return map[string]interface{}{
		"order_id": strconv.FormatInt(order.ID, 10),
		"symbol":   symbol,
		"side":     "close_short",
		"quantity": quantity,
	}, nil
}

// SetLeverage sets leverage for a contract
func (t *GateTrader) SetLeverage(symbol string, leverage int) error {
	symbol = t.normalizeSymbol(symbol)

	body := map[string]interface{}{
		"leverage": strconv.Itoa(leverage),
	}

	query := map[string]string{
		"settle": "usdt",
	}

	path := fmt.Sprintf(gateLeveragePath, symbol)
	_, err := t.doRequest("POST", path, query, body)
	return err
}

// SetMarginMode sets margin mode (cross or isolated)
func (t *GateTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	symbol = t.normalizeSymbol(symbol)

	mode := "isolated"
	if isCrossMargin {
		mode = "cross"
	}

	body := map[string]interface{}{
		"margin_mode": mode,
	}

	query := map[string]string{
		"settle": "usdt",
	}

	path := fmt.Sprintf(gateMarginModePath, symbol)
	_, err := t.doRequest("POST", path, query, body)
	return err
}

// GetMarketPrice returns current market price
func (t *GateTrader) GetMarketPrice(symbol string) (float64, error) {
	symbol = t.normalizeSymbol(symbol)

	query := map[string]string{
		"settle":   "usdt",
		"contract": symbol,
	}

	data, err := t.doRequest("GET", gateTickerPath, query, nil)
	if err != nil {
		return 0, err
	}

	var tickers []struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, fmt.Errorf("failed to parse ticker: %w", err)
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("no ticker data for symbol %s", symbol)
	}

	price, _ := strconv.ParseFloat(tickers[0].Last, 64)
	return price, nil
}

// SetStopLoss sets stop-loss order
func (t *GateTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	symbol = t.normalizeSymbol(symbol)

	size := -int64(quantity)
	rule := 2 // <= for long stop loss
	if positionSide == "short" {
		size = int64(quantity)
		rule = 1 // >= for short stop loss
	}

	body := map[string]interface{}{
		"initial": map[string]interface{}{
			"contract": symbol,
			"size":     size,
			"price":    "0",
		},
		"trigger": map[string]interface{}{
			"strategy_type": 0, // Stop loss
			"price_type":    0, // Last price
			"price":         strconv.FormatFloat(stopPrice, 'f', -1, 64),
			"rule":          rule,
		},
	}

	query := map[string]string{
		"settle": "usdt",
	}

	_, err := t.doRequest("POST", gatePriceOrdersPath, query, body)
	return err
}

// SetTakeProfit sets take-profit order
func (t *GateTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	symbol = t.normalizeSymbol(symbol)

	size := -int64(quantity)
	rule := 1 // >= for long take profit
	if positionSide == "short" {
		size = int64(quantity)
		rule = 2 // <= for short take profit
	}

	body := map[string]interface{}{
		"initial": map[string]interface{}{
			"contract": symbol,
			"size":     size,
			"price":    "0",
		},
		"trigger": map[string]interface{}{
			"strategy_type": 0,
			"price_type":    0,
			"price":         strconv.FormatFloat(takeProfitPrice, 'f', -1, 64),
			"rule":          rule,
		},
	}

	query := map[string]string{
		"settle": "usdt",
	}

	_, err := t.doRequest("POST", gatePriceOrdersPath, query, body)
	return err
}

// CancelStopLossOrders cancels stop-loss orders
func (t *GateTrader) CancelStopLossOrders(symbol string) error {
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders cancels take-profit orders
func (t *GateTrader) CancelTakeProfitOrders(symbol string) error {
	return t.CancelStopOrders(symbol)
}

// CancelAllOrders cancels all pending orders
func (t *GateTrader) CancelAllOrders(symbol string) error {
	symbol = t.normalizeSymbol(symbol)

	query := map[string]string{
		"settle":   "usdt",
		"contract": symbol,
	}

	// Get all open orders
	data, err := t.doRequest("GET", gateOrderPath, query, nil)
	if err != nil {
		return err
	}

	var orders []struct {
		ID int64 `json:"id"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	// Cancel each order
	for _, order := range orders {
		path := fmt.Sprintf(gateCancelOrderPath, strconv.FormatInt(order.ID, 10))
		_, _ = t.doRequest("DELETE", path, query, nil)
	}

	return nil
}

// CancelStopOrders cancels all stop orders (stop-loss and take-profit)
func (t *GateTrader) CancelStopOrders(symbol string) error {
	symbol = t.normalizeSymbol(symbol)

	query := map[string]string{
		"settle":   "usdt",
		"contract": symbol,
		"status":   "open",
	}

	// Get all price orders (stop/take-profit)
	data, err := t.doRequest("GET", gatePriceOrdersPath, query, nil)
	if err != nil {
		return err
	}

	var orders []struct {
		ID int64 `json:"id"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	// Cancel each price order
	for _, order := range orders {
		path := fmt.Sprintf(gateCancelPriceOrderPath, strconv.FormatInt(order.ID, 10))
		_, _ = t.doRequest("DELETE", path, query, nil)
	}

	return nil
}

// FormatQuantity formats quantity to correct precision
func (t *GateTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// Gate.io uses integer contracts
	return strconv.Itoa(int(quantity)), nil
}

// GetOrderStatus returns order status
func (t *GateTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	query := map[string]string{
		"settle": "usdt",
	}

	path := fmt.Sprintf(gateCancelOrderPath, orderID)
	data, err := t.doRequest("GET", path, query, nil)
	if err != nil {
		return nil, err
	}

	var order struct {
		Status    string `json:"status"`
		FillPrice string `json:"fill_price"`
		Size      int64  `json:"size"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}

	avgPrice, _ := strconv.ParseFloat(order.FillPrice, 64)

	return map[string]interface{}{
		"status":       strings.ToUpper(order.Status),
		"avg_price":    avgPrice,
		"executed_qty": float64(order.Size),
		"order_id":     orderID,
	}, nil
}

// GetClosedPnL returns closed position P&L records
func (t *GateTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	// Gate.io closed P&L query would require historical trades API
	// Returning empty for now
	return []ClosedPnLRecord{}, nil
}

// normalizeSymbol converts symbol to Gate.io format
func (t *GateTrader) normalizeSymbol(symbol string) string {
	// Convert BTCUSDT -> BTC_USDT
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") && !strings.Contains(symbol, "_") {
		base := strings.TrimSuffix(symbol, "USDT")
		return base + "_USDT"
	}
	return symbol
}

// isContractSupported checks if a contract is supported on Gate.io
func (t *GateTrader) isContractSupported(symbol string) (bool, error) {
	symbol = t.normalizeSymbol(symbol)

	// Check cache first (valid for 1 hour)
	t.contractsCacheMutex.RLock()
	if time.Since(t.contractsCacheTime) < 1*time.Hour && len(t.contractsCache) > 0 {
		_, exists := t.contractsCache[symbol]
		t.contractsCacheMutex.RUnlock()
		return exists, nil
	}
	t.contractsCacheMutex.RUnlock()

	// Fetch contracts list
	query := map[string]string{"settle": "usdt"}
	data, err := t.doRequest("GET", gateContractsPath, query, nil)
	if err != nil {
		return false, fmt.Errorf("failed to fetch contracts: %w", err)
	}

	var contracts []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &contracts); err != nil {
		return false, fmt.Errorf("failed to parse contracts: %w", err)
	}

	// Update cache
	t.contractsCacheMutex.Lock()
	t.contractsCache = make(map[string]*GateContract)
	for _, c := range contracts {
		t.contractsCache[c.Name] = &GateContract{Name: c.Name}
	}
	t.contractsCacheTime = time.Now()
	t.contractsCacheMutex.Unlock()

	_, exists := t.contractsCache[symbol]
	return exists, nil
}
