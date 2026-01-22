package trader

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
)

// MAX Exchange API endpoints (Taiwan cryptocurrency exchange)
// Documentation: https://max.maicoin.com/documents/api
const (
	maxBaseURL           = "https://max-api.maicoin.com"
	maxAccountsPath      = "/api/v2/members/accounts"
	maxOrdersPath        = "/api/v2/orders"
	maxOrderPath         = "/api/v2/order"
	maxCancelOrderPath   = "/api/v2/order/delete"
	maxMyTradesPath      = "/api/v2/trades/my"
	maxTickerPath        = "/api/v2/tickers"
	maxMarketsPath       = "/api/v2/markets"
	maxDepthPath         = "/api/v2/depth"
)

// MAXTrader MAX Exchange spot trader
// NOTE: MAX is a SPOT exchange, not futures. This trader adapts spot trading to the Trader interface:
// - OpenLong = Buy order (acquire asset)
// - CloseLong = Sell order (dispose asset)
// - Short positions are NOT supported
// - Leverage is always 1x (spot trading)
type MAXTrader struct {
	apiKey    string
	secretKey string

	// HTTP client
	httpClient *http.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Market info cache (for precision)
	marketsCache      map[string]*MAXMarket
	marketsCacheTime  time.Time
	marketsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// MAXMarket market info from MAX
type MAXMarket struct {
	ID             string  `json:"id"`              // e.g., "btctwd"
	Name           string  `json:"name"`            // e.g., "BTC/TWD"
	BaseUnit       string  `json:"base_unit"`       // e.g., "btc"
	QuoteUnit      string  `json:"quote_unit"`      // e.g., "twd"
	BaseUnitPrecision  int `json:"base_unit_precision"`
	QuoteUnitPrecision int `json:"quote_unit_precision"`
	MinBaseAmount  float64 `json:"min_base_amount,string"`
	MinQuoteAmount float64 `json:"min_quote_amount,string"`
}

// MAXResponse generic MAX API response
type MAXResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Error   *MAXError       `json:"error"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// MAXError error response
type MAXError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewMAXTrader creates MAX Exchange trader
func NewMAXTrader(apiKey, secretKey string) *MAXTrader {
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
	}

	trader := &MAXTrader{
		apiKey:        apiKey,
		secretKey:     secretKey,
		httpClient:    httpClient,
		cacheDuration: 15 * time.Second,
		marketsCache:  make(map[string]*MAXMarket),
	}

	// Load markets info
	if err := trader.loadMarkets(); err != nil {
		logger.Warnf("⚠️ Failed to load MAX markets: %v", err)
	}

	logger.Infof("✓ MAX Exchange trader initialized (SPOT trading mode)")
	return trader
}

// loadMarkets loads market info from MAX
func (t *MAXTrader) loadMarkets() error {
	data, err := t.doPublicRequest("GET", maxMarketsPath)
	if err != nil {
		return err
	}

	var markets []MAXMarket
	if err := json.Unmarshal(data, &markets); err != nil {
		return fmt.Errorf("failed to parse markets: %w", err)
	}

	t.marketsCacheMutex.Lock()
	for i := range markets {
		t.marketsCache[markets[i].ID] = &markets[i]
	}
	t.marketsCacheTime = time.Now()
	t.marketsCacheMutex.Unlock()

	logger.Infof("✓ Loaded %d MAX markets", len(markets))
	return nil
}

// sign generates MAX API signature
// MAX uses HMAC-SHA256 signature of the payload encoded in hex
func (t *MAXTrader) sign(payload string) string {
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// doPublicRequest executes public API request (no auth)
func (t *MAXTrader) doPublicRequest(method, path string) ([]byte, error) {
	req, err := http.NewRequest(method, maxBaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("MAX API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return body, nil
}

// doRequest executes authenticated API request
// MAX API authentication:
// - X-MAX-ACCESSKEY: API key
// - X-MAX-PAYLOAD: Base64 encoded JSON payload containing path and nonce
// - X-MAX-SIGNATURE: HMAC-SHA256 signature of the payload (hex encoded)
func (t *MAXTrader) doRequest(method, path string, params map[string]interface{}) ([]byte, error) {
	nonce := time.Now().UnixMilli()

	// Build payload
	payload := map[string]interface{}{
		"path":  path,
		"nonce": nonce,
	}

	// Add additional params to payload
	for k, v := range params {
		payload[k] = v
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	payloadBase64 := base64.StdEncoding.EncodeToString(payloadJSON)
	signature := t.sign(payloadBase64)

	// Build request URL with query params for GET
	url := maxBaseURL + path
	var req *http.Request

	if method == "GET" {
		// For GET requests, params go in query string
		if len(params) > 0 {
			queryParts := make([]string, 0, len(params))
			for k, v := range params {
				queryParts = append(queryParts, fmt.Sprintf("%s=%v", k, v))
			}
			url += "?" + strings.Join(queryParts, "&")
		}
		req, err = http.NewRequest(method, url, nil)
	} else {
		// For POST requests, params go in body
		var bodyJSON []byte
		if len(params) > 0 {
			bodyJSON, err = json.Marshal(params)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %w", err)
			}
		}
		req, err = http.NewRequest(method, url, strings.NewReader(string(bodyJSON)))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MAX-ACCESSKEY", t.apiKey)
	req.Header.Set("X-MAX-PAYLOAD", payloadBase64)
	req.Header.Set("X-MAX-SIGNATURE", signature)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("MAX API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// Check for error in response
	var errResp struct {
		Error *MAXError `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		return nil, fmt.Errorf("MAX API error: code=%d, message=%s", errResp.Error.Code, errResp.Error.Message)
	}

	return body, nil
}

// convertSymbol converts generic symbol to MAX format
// e.g., BTCUSDT -> btcusdt, BTCTWD -> btctwd
func (t *MAXTrader) convertSymbol(symbol string) string {
	return strings.ToLower(symbol)
}

// convertSymbolBack converts MAX format to generic
// e.g., btcusdt -> BTCUSDT
func (t *MAXTrader) convertSymbolBack(market string) string {
	return strings.ToUpper(market)
}

// GetBalance gets account balance
func (t *MAXTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ Using cached MAX account balance")
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	logger.Infof("🔄 Calling MAX API to get account balance...")
	data, err := t.doRequest("GET", maxAccountsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	var accounts []struct {
		Currency  string `json:"currency"`
		Balance   string `json:"balance"`
		Locked    string `json:"locked"`
		Type      string `json:"type"` // "spot" or "m_wallet" (margin)
	}

	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("failed to parse balance data: %w", err)
	}

	// Calculate total balance in TWD/USDT
	var totalBalance, availableBalance float64
	for _, acc := range accounts {
		if acc.Type != "spot" {
			continue
		}
		balance, _ := strconv.ParseFloat(acc.Balance, 64)
		locked, _ := strconv.ParseFloat(acc.Locked, 64)

		// For simplicity, sum up all balances
		// In a real scenario, you'd want to convert to a common currency
		if acc.Currency == "twd" || acc.Currency == "usdt" {
			totalBalance += balance + locked
			availableBalance += balance
		}
	}

	result := map[string]interface{}{
		"totalWalletBalance":    totalBalance,
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": 0.0, // Spot trading has no unrealized profit concept
	}

	logger.Infof("✓ MAX balance: Total=%.2f, Available=%.2f", totalBalance, availableBalance)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions gets "positions" (in spot, this means holdings)
func (t *MAXTrader) GetPositions() ([]map[string]interface{}, error) {
	logger.Infof("🔄 Getting MAX spot holdings as positions...")
	data, err := t.doRequest("GET", maxAccountsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	var accounts []struct {
		Currency string `json:"currency"`
		Balance  string `json:"balance"`
		Locked   string `json:"locked"`
		Type     string `json:"type"`
	}

	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("failed to parse accounts: %w", err)
	}

	var result []map[string]interface{}
	for _, acc := range accounts {
		if acc.Type != "spot" {
			continue
		}
		balance, _ := strconv.ParseFloat(acc.Balance, 64)
		locked, _ := strconv.ParseFloat(acc.Locked, 64)
		total := balance + locked

		// Skip zero balances and quote currencies
		if total == 0 || acc.Currency == "twd" || acc.Currency == "usdt" {
			continue
		}

		// Create a "position" representation for spot holdings
		posMap := map[string]interface{}{
			"symbol":           strings.ToUpper(acc.Currency) + "TWD", // Assume TWD pair
			"positionAmt":      total,
			"entryPrice":       0.0, // Not tracked in spot
			"markPrice":        0.0,
			"unRealizedProfit": 0.0,
			"leverage":         1,
			"liquidationPrice": 0.0,
			"side":             "long", // Spot = long only
		}
		result = append(result, posMap)
	}

	return result, nil
}

// OpenLong buys asset (spot buy)
func (t *MAXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if leverage > 1 {
		logger.Warnf("⚠️ MAX is spot exchange, leverage is always 1x (requested %dx ignored)", leverage)
	}

	market := t.convertSymbol(symbol)

	// Get market info for precision
	t.marketsCacheMutex.RLock()
	marketInfo := t.marketsCache[market]
	t.marketsCacheMutex.RUnlock()

	var volumeStr string
	if marketInfo != nil {
		format := fmt.Sprintf("%%.%df", marketInfo.BaseUnitPrecision)
		volumeStr = fmt.Sprintf(format, quantity)
	} else {
		volumeStr = fmt.Sprintf("%.8f", quantity)
	}

	params := map[string]interface{}{
		"market":   market,
		"side":     "buy",
		"volume":   volumeStr,
		"ord_type": "market",
	}

	logger.Infof("📈 MAX OpenLong (spot buy): market=%s, volume=%s", market, volumeStr)

	data, err := t.doRequest("POST", maxOrdersPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to place buy order: %w", err)
	}

	var order struct {
		ID              int    `json:"id"`
		Side            string `json:"side"`
		OrdType         string `json:"ord_type"`
		State           string `json:"state"`
		Market          string `json:"market"`
		Volume          string `json:"volume"`
		RemainingVolume string `json:"remaining_volume"`
		ExecutedVolume  string `json:"executed_volume"`
		AvgPrice        string `json:"avg_price"`
		CreatedAt       int64  `json:"created_at"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ MAX buy order placed: ID=%d, state=%s", order.ID, order.State)

	return map[string]interface{}{
		"orderId": fmt.Sprintf("%d", order.ID),
		"symbol":  symbol,
		"status":  t.mapOrderState(order.State),
	}, nil
}

// OpenShort not supported in spot trading
func (t *MAXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("MAX Exchange is a spot exchange - short selling is not supported")
}

// CloseLong sells asset (spot sell)
func (t *MAXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	market := t.convertSymbol(symbol)

	// Get market info for precision
	t.marketsCacheMutex.RLock()
	marketInfo := t.marketsCache[market]
	t.marketsCacheMutex.RUnlock()

	var volumeStr string
	if marketInfo != nil {
		format := fmt.Sprintf("%%.%df", marketInfo.BaseUnitPrecision)
		volumeStr = fmt.Sprintf(format, quantity)
	} else {
		volumeStr = fmt.Sprintf("%.8f", quantity)
	}

	params := map[string]interface{}{
		"market":   market,
		"side":     "sell",
		"volume":   volumeStr,
		"ord_type": "market",
	}

	logger.Infof("📉 MAX CloseLong (spot sell): market=%s, volume=%s", market, volumeStr)

	data, err := t.doRequest("POST", maxOrdersPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to place sell order: %w", err)
	}

	var order struct {
		ID              int    `json:"id"`
		Side            string `json:"side"`
		State           string `json:"state"`
		ExecutedVolume  string `json:"executed_volume"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ MAX sell order placed: ID=%d, state=%s", order.ID, order.State)

	return map[string]interface{}{
		"orderId": fmt.Sprintf("%d", order.ID),
		"symbol":  symbol,
		"status":  t.mapOrderState(order.State),
	}, nil
}

// CloseShort not supported in spot trading
func (t *MAXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("MAX Exchange is a spot exchange - short positions are not supported")
}

// SetLeverage not applicable for spot trading
func (t *MAXTrader) SetLeverage(symbol string, leverage int) error {
	if leverage > 1 {
		logger.Warnf("⚠️ MAX is spot exchange, leverage is always 1x")
	}
	return nil
}

// SetMarginMode not applicable for spot trading
func (t *MAXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	logger.Infof("⚠️ MAX is spot exchange, margin mode setting is not applicable")
	return nil
}

// SetStopLoss not directly supported on MAX spot
func (t *MAXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	// MAX doesn't have native stop-loss for spot
	// Would need to implement using stop-limit orders
	logger.Warnf("⚠️ MAX spot: stop-loss orders require manual implementation via stop-limit")
	return fmt.Errorf("stop-loss orders not directly supported on MAX spot exchange")
}

// SetTakeProfit not directly supported on MAX spot
func (t *MAXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	logger.Warnf("⚠️ MAX spot: take-profit orders require manual implementation via limit orders")
	return fmt.Errorf("take-profit orders not directly supported on MAX spot exchange")
}

// CancelAllOrders cancels all open orders for a symbol
func (t *MAXTrader) CancelAllOrders(symbol string) error {
	market := t.convertSymbol(symbol)

	// Get open orders first
	params := map[string]interface{}{
		"market": market,
		"state":  "wait",
	}

	data, err := t.doRequest("GET", maxOrdersPath, params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var orders []struct {
		ID int `json:"id"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return fmt.Errorf("failed to parse orders: %w", err)
	}

	// Cancel each order
	canceledCount := 0
	for _, order := range orders {
		cancelParams := map[string]interface{}{
			"id": order.ID,
		}
		_, err := t.doRequest("POST", maxCancelOrderPath, cancelParams)
		if err != nil {
			logger.Warnf("⚠️ Failed to cancel order %d: %v", order.ID, err)
			continue
		}
		canceledCount++
	}

	if canceledCount > 0 {
		logger.Infof("✓ Canceled %d orders for %s", canceledCount, symbol)
	}

	return nil
}

// CancelStopLossOrders not applicable
func (t *MAXTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

// CancelTakeProfitOrders not applicable
func (t *MAXTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

// GetOrderStatus gets order status
func (t *MAXTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": orderID,
	}

	data, err := t.doRequest("GET", maxOrderPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	var order struct {
		ID              int    `json:"id"`
		Side            string `json:"side"`
		OrdType         string `json:"ord_type"`
		State           string `json:"state"`
		Market          string `json:"market"`
		Price           string `json:"price"`
		Volume          string `json:"volume"`
		ExecutedVolume  string `json:"executed_volume"`
		RemainingVolume string `json:"remaining_volume"`
		AvgPrice        string `json:"avg_price"`
		TradesCount     int    `json:"trades_count"`
		CreatedAt       int64  `json:"created_at"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order: %w", err)
	}

	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedVolume, 64)

	return map[string]interface{}{
		"orderId":     orderID,
		"symbol":      symbol,
		"status":      t.mapOrderState(order.State),
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"side":        order.Side,
		"type":        order.OrdType,
		"time":        order.CreatedAt * 1000,
	}, nil
}

// GetMarketPrice gets current market price
func (t *MAXTrader) GetMarketPrice(symbol string) (float64, error) {
	market := t.convertSymbol(symbol)
	path := fmt.Sprintf("%s/%s", maxTickerPath, market)

	data, err := t.doPublicRequest("GET", path)
	if err != nil {
		return 0, fmt.Errorf("failed to get ticker: %w", err)
	}

	var ticker struct {
		At     int64  `json:"at"`
		Buy    string `json:"buy"`
		Sell   string `json:"sell"`
		Open   string `json:"open"`
		Low    string `json:"low"`
		High   string `json:"high"`
		Last   string `json:"last"`
		Vol    string `json:"vol"`
		VolInBTC string `json:"vol_in_btc"`
	}

	if err := json.Unmarshal(data, &ticker); err != nil {
		return 0, fmt.Errorf("failed to parse ticker: %w", err)
	}

	price, err := strconv.ParseFloat(ticker.Last, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price: %w", err)
	}

	return price, nil
}

// FormatQuantity formats quantity based on market precision
func (t *MAXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	market := t.convertSymbol(symbol)

	t.marketsCacheMutex.RLock()
	marketInfo := t.marketsCache[market]
	t.marketsCacheMutex.RUnlock()

	if marketInfo != nil {
		format := fmt.Sprintf("%%.%df", marketInfo.BaseUnitPrecision)
		return fmt.Sprintf(format, quantity), nil
	}

	return fmt.Sprintf("%.8f", quantity), nil
}

// GetClosedPnL gets closed trade history
func (t *MAXTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	params := map[string]interface{}{
		"limit": limit,
	}
	if !startTime.IsZero() {
		params["timestamp"] = startTime.Unix()
	}

	data, err := t.doRequest("GET", maxMyTradesPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get trades: %w", err)
	}

	var trades []struct {
		ID        int    `json:"id"`
		Price     string `json:"price"`
		Volume    string `json:"volume"`
		Funds     string `json:"funds"`
		Market    string `json:"market"`
		Side      string `json:"side"`
		Fee       string `json:"fee"`
		FeeCurrency string `json:"fee_currency"`
		CreatedAt int64  `json:"created_at"`
	}

	if err := json.Unmarshal(data, &trades); err != nil {
		return nil, fmt.Errorf("failed to parse trades: %w", err)
	}

	records := make([]ClosedPnLRecord, 0, len(trades))
	for _, trade := range trades {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		volume, _ := strconv.ParseFloat(trade.Volume, 64)
		fee, _ := strconv.ParseFloat(trade.Fee, 64)

		record := ClosedPnLRecord{
			Symbol:      t.convertSymbolBack(trade.Market),
			Side:        trade.Side,
			EntryPrice:  price,
			ExitPrice:   price,
			Quantity:    volume,
			RealizedPnL: 0, // Spot doesn't track PnL per trade
			Fee:         fee,
			Leverage:    1,
			EntryTime:   time.Unix(trade.CreatedAt, 0).UTC(),
			ExitTime:    time.Unix(trade.CreatedAt, 0).UTC(),
			CloseType:   "trade",
			ExchangeID:  fmt.Sprintf("%d", trade.ID),
		}
		records = append(records, record)
	}

	return records, nil
}

// GetOpenOrders gets open orders
func (t *MAXTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	market := t.convertSymbol(symbol)

	params := map[string]interface{}{
		"market": market,
		"state":  "wait",
	}

	data, err := t.doRequest("GET", maxOrdersPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	var orders []struct {
		ID              int    `json:"id"`
		Side            string `json:"side"`
		OrdType         string `json:"ord_type"`
		Price           string `json:"price"`
		StopPrice       string `json:"stop_price"`
		Volume          string `json:"volume"`
		RemainingVolume string `json:"remaining_volume"`
		State           string `json:"state"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse orders: %w", err)
	}

	result := make([]OpenOrder, 0, len(orders))
	for _, order := range orders {
		price, _ := strconv.ParseFloat(order.Price, 64)
		stopPrice, _ := strconv.ParseFloat(order.StopPrice, 64)
		quantity, _ := strconv.ParseFloat(order.Volume, 64)

		result = append(result, OpenOrder{
			OrderID:      fmt.Sprintf("%d", order.ID),
			Symbol:       symbol,
			Side:         strings.ToUpper(order.Side),
			PositionSide: "LONG", // Spot is always long
			Type:         strings.ToUpper(order.OrdType),
			Price:        price,
			StopPrice:    stopPrice,
			Quantity:     quantity,
			Status:       t.mapOrderState(order.State),
		})
	}

	return result, nil
}

// mapOrderState maps MAX order state to standard format
func (t *MAXTrader) mapOrderState(state string) string {
	switch state {
	case "wait":
		return "NEW"
	case "done":
		return "FILLED"
	case "cancel":
		return "CANCELED"
	case "convert":
		return "FILLED"
	default:
		return state
	}
}
