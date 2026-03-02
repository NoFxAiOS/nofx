package indodax

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Indodax API endpoints
const (
	indodaxBaseURL    = "https://indodax.com"
	indodaxPublicAPI  = "/api"
	indodaxPrivateAPI = "/tapi"
)

// IndodaxTrader implements types.Trader interface for Indodax Spot Exchange
// Indodax is Indonesia's largest crypto exchange, supporting IDR (Indonesian Rupiah) pairs.
// Since Indodax is spot-only, futures-specific methods (OpenShort, CloseShort, leverage, etc.)
// are gracefully stubbed.
type IndodaxTrader struct {
	apiKey    string
	secretKey string

	httpClient *http.Client
	nonce      int64
	nonceMutex sync.Mutex

	// Cache for pair info
	pairCache      map[string]*IndodaxPair
	pairCacheMutex sync.RWMutex
	pairCacheTime  time.Time

	// Cache for balance
	cachedBalance     map[string]interface{}
	cachedPositions   []map[string]interface{}
	balanceCacheTime  time.Time
	positionCacheTime time.Time
	cacheDuration     time.Duration
	cacheMutex        sync.RWMutex
}

// IndodaxPair represents a trading pair on Indodax
type IndodaxPair struct {
	ID                     string  `json:"id"`
	Symbol                 string  `json:"symbol"`
	BaseCurrency           string  `json:"base_currency"`
	TradedCurrency         string  `json:"traded_currency"`
	TradedCurrencyUnit     string  `json:"traded_currency_unit"`
	Description            string  `json:"description"`
	TickerID               string  `json:"ticker_id"`
	VolumePrecision        int     `json:"volume_precision"`
	PricePrecision         float64 `json:"price_precision"`
	PriceRound             int     `json:"price_round"`
	Pricescale             float64 `json:"pricescale"`
	TradeMinBaseCurrency   float64 `json:"trade_min_base_currency"`
	TradeMinTradedCurrency float64 `json:"trade_min_traded_currency"`
}

// IndodaxResponse represents the standard Indodax private API response
type IndodaxResponse struct {
	Success   int             `json:"success"`
	Return    json.RawMessage `json:"return,omitempty"`
	Error     string          `json:"error,omitempty"`
	ErrorCode string          `json:"error_code,omitempty"`
}

// IndodaxTicker represents ticker data
type IndodaxTicker struct {
	High       string `json:"high"`
	Low        string `json:"low"`
	Last       string `json:"last"`
	Buy        string `json:"buy"`
	Sell       string `json:"sell"`
	ServerTime int64  `json:"server_time"`
}

// IndodaxTickerResponse wraps ticker response
type IndodaxTickerResponse struct {
	Ticker IndodaxTicker `json:"ticker"`
}

// NewIndodaxTrader creates a new Indodax trader instance
func NewIndodaxTrader(apiKey, secretKey string) *IndodaxTrader {
	return &IndodaxTrader{
		apiKey:        apiKey,
		secretKey:     secretKey,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		nonce:         time.Now().UnixMilli(),
		pairCache:     make(map[string]*IndodaxPair),
		cacheDuration: 15 * time.Second,
	}
}

// getNonce returns a unique incrementing nonce for each request
func (t *IndodaxTrader) getNonce() int64 {
	t.nonceMutex.Lock()
	defer t.nonceMutex.Unlock()
	t.nonce++
	return t.nonce
}

// sign generates HMAC-SHA512 signature for request body
func (t *IndodaxTrader) sign(body string) string {
	mac := hmac.New(sha512.New, []byte(t.secretKey))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// doPublicRequest makes a public API GET request
func (t *IndodaxTrader) doPublicRequest(path string) ([]byte, error) {
	reqURL := indodaxBaseURL + indodaxPublicAPI + path

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// doPrivateRequest makes a signed private API POST request
func (t *IndodaxTrader) doPrivateRequest(params url.Values) ([]byte, error) {
	reqURL := indodaxBaseURL + indodaxPrivateAPI

	// Add nonce
	params.Set("nonce", strconv.FormatInt(t.getNonce(), 10))

	body := params.Encode()
	signature := t.sign(body)

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Key", t.apiKey)
	req.Header.Set("Sign", signature)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	}

	// Parse response to check success
	var apiResp IndodaxResponse
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(data))
	}

	if apiResp.Success != 1 {
		return nil, fmt.Errorf("API error: %s (code: %s)", apiResp.Error, apiResp.ErrorCode)
	}

	return apiResp.Return, nil
}

// convertSymbol converts standard symbol to Indodax format
// e.g. BTCIDR -> btc_idr, ETHIDR -> eth_idr
func (t *IndodaxTrader) convertSymbol(symbol string) string {
	s := strings.ToLower(symbol)

	// Already in Indodax format (contains underscore)
	if strings.Contains(s, "_") {
		return s
	}

	// Try to split by known base currencies
	for _, base := range []string{"idr", "btc", "usdt"} {
		if strings.HasSuffix(s, base) {
			traded := strings.TrimSuffix(s, base)
			if traded != "" {
				return traded + "_" + base
			}
		}
	}

	return s
}

// convertSymbolBack converts Indodax format back to standard
// e.g. btc_idr -> BTCIDR
func (t *IndodaxTrader) convertSymbolBack(indodaxSymbol string) string {
	return strings.ToUpper(strings.ReplaceAll(indodaxSymbol, "_", ""))
}

// getCoinFromSymbol extracts the traded currency from a symbol
// e.g. btc_idr -> btc, eth_idr -> eth
func (t *IndodaxTrader) getCoinFromSymbol(symbol string) string {
	pair := t.convertSymbol(symbol)
	parts := strings.Split(pair, "_")
	if len(parts) >= 1 {
		return parts[0]
	}
	return strings.ToLower(symbol)
}

// loadPairs loads trading pair information from the public API
func (t *IndodaxTrader) loadPairs() error {
	t.pairCacheMutex.RLock()
	if len(t.pairCache) > 0 && time.Since(t.pairCacheTime) < 5*time.Minute {
		t.pairCacheMutex.RUnlock()
		return nil
	}
	t.pairCacheMutex.RUnlock()

	data, err := t.doPublicRequest("/pairs")
	if err != nil {
		return fmt.Errorf("failed to load pairs: %w", err)
	}

	var pairs []IndodaxPair
	if err := json.Unmarshal(data, &pairs); err != nil {
		return fmt.Errorf("failed to parse pairs: %w", err)
	}

	t.pairCacheMutex.Lock()
	defer t.pairCacheMutex.Unlock()

	t.pairCache = make(map[string]*IndodaxPair)
	for i := range pairs {
		p := pairs[i]
		t.pairCache[p.TickerID] = &p
		// Also index by ID (e.g. "btcidr")
		t.pairCache[p.ID] = &p
	}
	t.pairCacheTime = time.Now()

	logger.Infof("[Indodax] Loaded %d trading pairs", len(pairs))
	return nil
}

// getPair gets pair info for a symbol
func (t *IndodaxTrader) getPair(symbol string) (*IndodaxPair, error) {
	if err := t.loadPairs(); err != nil {
		return nil, err
	}

	pairID := t.convertSymbol(symbol)

	t.pairCacheMutex.RLock()
	defer t.pairCacheMutex.RUnlock()

	if pair, ok := t.pairCache[pairID]; ok {
		return pair, nil
	}

	// Try without underscore
	noUnderscore := strings.ReplaceAll(pairID, "_", "")
	if pair, ok := t.pairCache[noUnderscore]; ok {
		return pair, nil
	}

	return nil, fmt.Errorf("pair not found: %s", symbol)
}

// clearCache clears cached data
func (t *IndodaxTrader) clearCache() {
	t.cacheMutex.Lock()
	defer t.cacheMutex.Unlock()
	t.cachedBalance = nil
	t.cachedPositions = nil
}

// ============================================================
// types.Trader interface implementation
// ============================================================

// GetBalance gets account balance from Indodax
func (t *IndodaxTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.cacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cached := t.cachedBalance
		t.cacheMutex.RUnlock()
		return cached, nil
	}
	t.cacheMutex.RUnlock()

	params := url.Values{}
	params.Set("method", "getInfo")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	var result struct {
		ServerTime  int64                  `json:"server_time"`
		Balance     map[string]interface{} `json:"balance"`
		BalanceHold map[string]interface{} `json:"balance_hold"`
		UserID      string                 `json:"user_id"`
		Name        string                 `json:"name"`
		Email       string                 `json:"email"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse balance: %w", err)
	}

	// Calculate total balance in IDR
	idrBalance := parseFloat(result.Balance["idr"])
	idrHold := parseFloat(result.BalanceHold["idr"])
	totalIDR := idrBalance + idrHold

	balance := map[string]interface{}{
		"totalWalletBalance":    totalIDR,
		"availableBalance":      idrBalance,
		"totalUnrealizedProfit": 0.0,
		"totalEquity":           totalIDR,
		"balance":               totalIDR,
		"idr_balance":           idrBalance,
		"idr_hold":              idrHold,
		"currency":              "IDR",
		"user_id":               result.UserID,
		"server_time":           result.ServerTime,
	}

	// Add individual crypto balances
	for currency, amount := range result.Balance {
		if currency != "idr" {
			balance["balance_"+currency] = parseFloat(amount)
		}
	}
	for currency, amount := range result.BalanceHold {
		if currency != "idr" {
			balance["hold_"+currency] = parseFloat(amount)
		}
	}

	// Update cache
	t.cacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.cacheMutex.Unlock()

	return balance, nil
}

// GetPositions returns currently held crypto balances as "positions"
// Since Indodax is spot-only, each non-zero crypto balance is treated as a position
func (t *IndodaxTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.cacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionCacheTime) < t.cacheDuration {
		cached := t.cachedPositions
		t.cacheMutex.RUnlock()
		return cached, nil
	}
	t.cacheMutex.RUnlock()

	params := url.Values{}
	params.Set("method", "getInfo")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result struct {
		Balance     map[string]interface{} `json:"balance"`
		BalanceHold map[string]interface{} `json:"balance_hold"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse positions: %w", err)
	}

	var positions []map[string]interface{}

	for currency, amountRaw := range result.Balance {
		if currency == "idr" {
			continue
		}

		amount := parseFloat(amountRaw)
		holdAmount := parseFloat(result.BalanceHold[currency])
		totalAmount := amount + holdAmount

		if totalAmount <= 0 {
			continue
		}

		// Get market price for this coin
		markPrice, _ := t.GetMarketPrice(strings.ToUpper(currency) + "IDR")

		// Calculate position value in IDR
		notionalValue := totalAmount * markPrice

		position := map[string]interface{}{
			"symbol":           strings.ToUpper(currency) + "IDR",
			"side":             "LONG",
			"positionAmt":      totalAmount,
			"entryPrice":       markPrice, // Spot doesn't track entry price
			"markPrice":        markPrice,
			"unRealizedProfit": 0.0, // Spot doesn't track unrealized PnL
			"leverage":         1.0,
			"mgnMode":          "spot",
			"notionalValue":    notionalValue,
			"currency":         currency,
			"available":        amount,
			"hold":             holdAmount,
		}

		positions = append(positions, position)
	}

	// Update cache
	t.cacheMutex.Lock()
	t.cachedPositions = positions
	t.positionCacheTime = time.Now()
	t.cacheMutex.Unlock()

	return positions, nil
}

// OpenLong opens a spot buy order
func (t *IndodaxTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	t.clearCache()

	pair := t.convertSymbol(symbol)
	coin := t.getCoinFromSymbol(symbol)

	// Get market price to calculate IDR amount
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	params := url.Values{}
	params.Set("method", "trade")
	params.Set("pair", pair)
	params.Set("type", "buy")
	params.Set("price", strconv.FormatFloat(price, 'f', 0, 64))
	params.Set(coin, strconv.FormatFloat(quantity, 'f', 8, 64))
	params.Set("order_type", "limit")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to place buy order: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse trade response: %w", err)
	}

	logger.Infof("[Indodax] Buy order placed: %s qty=%.8f price=%.0f", symbol, quantity, price)

	return map[string]interface{}{
		"orderId": result["order_id"],
		"symbol":  symbol,
		"side":    "BUY",
		"price":   price,
		"qty":     quantity,
		"status":  "NEW",
	}, nil
}

// OpenShort is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling is not supported on Indodax (spot-only exchange)")
}

// CloseLong closes a spot position by selling
func (t *IndodaxTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	t.clearCache()

	pair := t.convertSymbol(symbol)
	coin := t.getCoinFromSymbol(symbol)

	// If quantity is 0, sell all available balance
	if quantity <= 0 {
		balance, err := t.GetBalance()
		if err != nil {
			return nil, fmt.Errorf("failed to get balance for close all: %w", err)
		}
		available := parseFloat(balance["balance_"+coin])
		if available <= 0 {
			return nil, fmt.Errorf("no %s balance to sell", coin)
		}
		quantity = available
	}

	// Get market price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	params := url.Values{}
	params.Set("method", "trade")
	params.Set("pair", pair)
	params.Set("type", "sell")
	params.Set("price", strconv.FormatFloat(price, 'f', 0, 64))
	params.Set(coin, strconv.FormatFloat(quantity, 'f', 8, 64))
	params.Set("order_type", "limit")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to place sell order: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse trade response: %w", err)
	}

	logger.Infof("[Indodax] Sell order placed: %s qty=%.8f price=%.0f", symbol, quantity, price)

	return map[string]interface{}{
		"orderId": result["order_id"],
		"symbol":  symbol,
		"side":    "SELL",
		"price":   price,
		"qty":     quantity,
		"status":  "NEW",
	}, nil
}

// CloseShort is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling is not supported on Indodax (spot-only exchange)")
}

// SetLeverage is a no-op for Indodax (spot-only, no leverage)
func (t *IndodaxTrader) SetLeverage(symbol string, leverage int) error {
	logger.Infof("[Indodax] SetLeverage ignored (spot-only exchange, no leverage support)")
	return nil
}

// SetMarginMode is a no-op for Indodax (spot-only, no margin)
func (t *IndodaxTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	logger.Infof("[Indodax] SetMarginMode ignored (spot-only exchange, no margin support)")
	return nil
}

// GetMarketPrice gets the current market price for a symbol
func (t *IndodaxTrader) GetMarketPrice(symbol string) (float64, error) {
	pairID := strings.ToLower(strings.ReplaceAll(t.convertSymbol(symbol), "_", ""))

	data, err := t.doPublicRequest("/ticker/" + pairID)
	if err != nil {
		return 0, fmt.Errorf("failed to get ticker: %w", err)
	}

	var tickerResp IndodaxTickerResponse
	if err := json.Unmarshal(data, &tickerResp); err != nil {
		return 0, fmt.Errorf("failed to parse ticker: %w", err)
	}

	price, err := strconv.ParseFloat(tickerResp.Ticker.Last, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price '%s': %w", tickerResp.Ticker.Last, err)
	}

	return price, nil
}

// SetStopLoss is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return fmt.Errorf("stop-loss orders are not supported on Indodax (spot-only exchange)")
}

// SetTakeProfit is not supported on Indodax (spot-only exchange)
func (t *IndodaxTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return fmt.Errorf("take-profit orders are not supported on Indodax (spot-only exchange)")
}

// CancelStopLossOrders is a no-op for Indodax
func (t *IndodaxTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

// CancelTakeProfitOrders is a no-op for Indodax
func (t *IndodaxTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

// CancelAllOrders cancels all open orders for a given symbol
func (t *IndodaxTrader) CancelAllOrders(symbol string) error {
	t.clearCache()

	pair := t.convertSymbol(symbol)

	// First get open orders
	params := url.Values{}
	params.Set("method", "openOrders")
	params.Set("pair", pair)

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	var result struct {
		Orders []struct {
			OrderID   json.Number `json:"order_id"`
			Type      string      `json:"type"`
			OrderType string      `json:"order_type"`
		} `json:"orders"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to parse open orders: %w", err)
	}

	// Cancel each order
	for _, order := range result.Orders {
		cancelParams := url.Values{}
		cancelParams.Set("method", "cancelOrder")
		cancelParams.Set("pair", pair)
		cancelParams.Set("order_id", order.OrderID.String())
		cancelParams.Set("type", order.Type)

		if _, err := t.doPrivateRequest(cancelParams); err != nil {
			logger.Warnf("[Indodax] Failed to cancel order %s: %v", order.OrderID, err)
		} else {
			logger.Infof("[Indodax] Cancelled order: %s", order.OrderID)
		}
	}

	return nil
}

// CancelStopOrders is a no-op for Indodax (no stop orders)
func (t *IndodaxTrader) CancelStopOrders(symbol string) error {
	return nil
}

// FormatQuantity formats quantity to correct precision for Indodax
func (t *IndodaxTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	pair, err := t.getPair(symbol)
	if err != nil {
		// Default: 8 decimal places
		return strconv.FormatFloat(quantity, 'f', 8, 64), nil
	}

	precision := pair.PriceRound
	if precision <= 0 {
		precision = 8
	}

	// Round down to avoid exceeding balance
	factor := math.Pow(10, float64(precision))
	rounded := math.Floor(quantity*factor) / factor

	return strconv.FormatFloat(rounded, 'f', precision, 64), nil
}

// GetOrderStatus gets the status of a specific order
func (t *IndodaxTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	pair := t.convertSymbol(symbol)

	params := url.Values{}
	params.Set("method", "getOrder")
	params.Set("pair", pair)
	params.Set("order_id", orderID)

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var result struct {
		Order struct {
			OrderID       string `json:"order_id"`
			Price         string `json:"price"`
			Type          string `json:"type"`
			Status        string `json:"status"`
			SubmitTime    string `json:"submit_time"`
			FinishTime    string `json:"finish_time"`
			ClientOrderID string `json:"client_order_id"`
		} `json:"order"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order: %w", err)
	}

	// Map Indodax status to standard status
	status := "NEW"
	switch result.Order.Status {
	case "filled":
		status = "FILLED"
	case "cancelled":
		status = "CANCELED"
	case "open":
		status = "NEW"
	}

	price, _ := strconv.ParseFloat(result.Order.Price, 64)

	return map[string]interface{}{
		"status":      status,
		"avgPrice":    price,
		"executedQty": 0.0, // Indodax doesn't return executed qty in getOrder
		"commission":  0.0,
		"orderId":     result.Order.OrderID,
	}, nil
}

// GetClosedPnL gets closed position PnL records (trade history)
func (t *IndodaxTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// Indodax trade history is limited to 7 days range
	params := url.Values{}
	params.Set("method", "tradeHistory")
	params.Set("pair", "btc_idr") // Default pair; Indodax requires a pair
	if limit > 0 {
		params.Set("count", strconv.Itoa(limit))
	}
	if !startTime.IsZero() {
		params.Set("since", strconv.FormatInt(startTime.Unix(), 10))
	}

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history: %w", err)
	}

	var result struct {
		Trades []struct {
			TradeID       string `json:"trade_id"`
			OrderID       string `json:"order_id"`
			Type          string `json:"type"`
			Price         string `json:"price"`
			Fee           string `json:"fee"`
			TradeTime     string `json:"trade_time"`
			ClientOrderID string `json:"client_order_id"`
		} `json:"trades"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		// Trade history might return empty, that's fine
		return nil, nil
	}

	var records []types.ClosedPnLRecord
	for _, trade := range result.Trades {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		fee, _ := strconv.ParseFloat(trade.Fee, 64)
		tradeTime, _ := strconv.ParseInt(trade.TradeTime, 10, 64)

		side := "long"
		if trade.Type == "sell" {
			side = "long" // Selling from a spot position is closing long
		}

		records = append(records, types.ClosedPnLRecord{
			Symbol:    "BTCIDR",
			Side:      side,
			ExitPrice: price,
			Fee:       fee,
			ExitTime:  time.Unix(tradeTime, 0),
			OrderID:   trade.OrderID,
			CloseType: "manual",
		})
	}

	return records, nil
}

// GetOpenOrders gets open/pending orders
func (t *IndodaxTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	pair := t.convertSymbol(symbol)

	params := url.Values{}
	params.Set("method", "openOrders")
	if pair != "" {
		params.Set("pair", pair)
	}

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var result struct {
		Orders []struct {
			OrderID       json.Number `json:"order_id"`
			ClientOrderID string      `json:"client_order_id"`
			SubmitTime    string      `json:"submit_time"`
			Price         string      `json:"price"`
			Type          string      `json:"type"`
			OrderType     string      `json:"order_type"`
		} `json:"orders"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse open orders: %w", err)
	}

	var orders []types.OpenOrder
	for _, order := range result.Orders {
		price, _ := strconv.ParseFloat(order.Price, 64)

		side := "BUY"
		if order.Type == "sell" {
			side = "SELL"
		}

		orders = append(orders, types.OpenOrder{
			OrderID:      order.OrderID.String(),
			Symbol:       t.convertSymbolBack(pair),
			Side:         side,
			PositionSide: "LONG",
			Type:         "LIMIT",
			Price:        price,
			Status:       "NEW",
		})
	}

	return orders, nil
}

// ============================================================
// Helper functions
// ============================================================

// parseFloat safely parses a float from interface{}
func parseFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
