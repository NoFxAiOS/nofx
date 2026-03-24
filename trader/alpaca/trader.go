package alpaca

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/logger"
	"nofx/safe"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Alpaca API endpoints
const (
	alpacaPaperBaseURL = "https://paper-api.alpaca.markets"
	alpacaLiveBaseURL  = "https://api.alpaca.markets"
	alpacaDataBaseURL  = "https://data.alpaca.markets"
)

// AlpacaTrader implements types.Trader for Alpaca US stock trading.
// Maps the crypto-oriented Trader interface to stock operations:
//   - OpenLong  → Buy shares (leverage ignored, always 1)
//   - CloseLong → Sell shares
//   - OpenShort/CloseShort → Not supported (requires margin account)
//   - GetPositions, GetBalance, GetMarketPrice → Direct mapping
type AlpacaTrader struct {
	apiKey    string
	apiSecret string
	baseURL   string // paper or live
	client    *http.Client

	// Cache
	cachedBalance     map[string]interface{}
	cachedPositions   []map[string]interface{}
	balanceCacheTime  time.Time
	positionCacheTime time.Time
	cacheDuration     time.Duration
	cacheMutex        sync.RWMutex
}

// NewAlpacaTrader creates a new Alpaca trader for paper trading
func NewAlpacaTrader(apiKey, apiSecret string, paper bool) *AlpacaTrader {
	baseURL := alpacaLiveBaseURL
	if paper {
		baseURL = alpacaPaperBaseURL
	}
	return &AlpacaTrader{
		apiKey:        apiKey,
		apiSecret:     apiSecret,
		baseURL:       baseURL,
		client:        &http.Client{Timeout: 30 * time.Second},
		cacheDuration: 10 * time.Second,
	}
}

// --- HTTP helpers ---

func (t *AlpacaTrader) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody *strings.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = strings.NewReader(string(data))
	} else {
		reqBody = strings.NewReader("")
	}

	url := t.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("APCA-API-KEY-ID", t.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", t.apiSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := safe.ReadAllLimited(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respData, resp.StatusCode, nil
}

func (t *AlpacaTrader) doGet(path string) ([]byte, error) {
	data, status, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("Alpaca API error (HTTP %d): %s", status, truncate(string(data), 256))
	}
	return data, nil
}

func (t *AlpacaTrader) doPost(path string, body interface{}) ([]byte, error) {
	data, status, err := t.doRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("Alpaca API error (HTTP %d): %s", status, truncate(string(data), 256))
	}
	return data, nil
}

func (t *AlpacaTrader) doDelete(path string) ([]byte, error) {
	data, status, err := t.doRequest("DELETE", path, nil)
	if err != nil {
		return nil, err
	}
	// 204 No Content is success for DELETE
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("Alpaca API error (HTTP %d): %s", status, truncate(string(data), 256))
	}
	return data, nil
}

// doDataGet makes a GET to the data API (for market data)
func (t *AlpacaTrader) doDataGet(path string) ([]byte, error) {
	url := alpacaDataBaseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("APCA-API-KEY-ID", t.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", t.apiSecret)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := safe.ReadAllLimited(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Alpaca Data API error (HTTP %d): %s", resp.StatusCode, truncate(string(data), 256))
	}

	return data, nil
}

// --- Trader interface implementation ---

// GetBalance returns account balance info
func (t *AlpacaTrader) GetBalance() (map[string]interface{}, error) {
	t.cacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		result := t.cachedBalance
		t.cacheMutex.RUnlock()
		return result, nil
	}
	t.cacheMutex.RUnlock()

	data, err := t.doGet("/v2/account")
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}

	var acct AlpacaAccount
	if err := json.Unmarshal(data, &acct); err != nil {
		return nil, fmt.Errorf("parse account: %w", err)
	}

	equity := parseFloatStr(acct.Equity)
	cash := parseFloatStr(acct.Cash)
	buyingPower := parseFloatStr(acct.BuyingPower)

	result := map[string]interface{}{
		// Standard fields expected by auto_trader (camelCase)
		"totalEquity":           equity,
		"totalWalletBalance":    cash,
		"availableBalance":      cash,
		"totalUnrealizedProfit": equity - cash,
		// Alpaca-specific fields
		"buying_power":      buyingPower,
		"currency":          acct.Currency,
		"status":            acct.Status,
		"account_number":    acct.AccountNumber,
		"pattern_day_trader": acct.PatternDayTrader,
		"day_trade_count":   acct.DaytradeCount,
	}

	t.cacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.cacheMutex.Unlock()

	return result, nil
}

// GetPositions returns all open positions
func (t *AlpacaTrader) GetPositions() ([]map[string]interface{}, error) {
	t.cacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionCacheTime) < t.cacheDuration {
		result := t.cachedPositions
		t.cacheMutex.RUnlock()
		return result, nil
	}
	t.cacheMutex.RUnlock()

	data, err := t.doGet("/v2/positions")
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}

	var positions []AlpacaPosition
	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("parse positions: %w", err)
	}

	var result []map[string]interface{}
	for _, p := range positions {
		qty := parseFloatStr(p.Qty)
		side := "long"
		if p.Side == "short" {
			side = "short"
		}

		result = append(result, map[string]interface{}{
			"symbol":           p.Symbol,
			"side":             side,
			"size":             qty,
			"positionAmt":      qty, // Standard field expected by auto_trader
			"entryPrice":       parseFloatStr(p.AvgEntryPrice),
			"markPrice":        parseFloatStr(p.CurrentPrice),
			"unrealizedPnl":    parseFloatStr(p.UnrealizedPL),
			"unRealizedProfit": parseFloatStr(p.UnrealizedPL), // Standard field
			"marketValue":      parseFloatStr(p.MarketValue),
			"leverage":         float64(1),
			"exchange":         "alpaca",
		})
	}

	t.cacheMutex.Lock()
	t.cachedPositions = result
	t.positionCacheTime = time.Now()
	t.cacheMutex.Unlock()

	return result, nil
}

// OpenLong buys shares (market order)
func (t *AlpacaTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	logger.Infof("[Alpaca] BUY %s qty=%.4f", symbol, quantity)

	order := AlpacaOrderRequest{
		Symbol:      symbol,
		Qty:         fmt.Sprintf("%.4f", quantity), // Alpaca supports fractional shares
		Side:        "buy",
		Type:        "market",
		TimeInForce: "day",
	}

	data, err := t.doPost("/v2/orders", order)
	if err != nil {
		return nil, fmt.Errorf("buy %s: %w", symbol, err)
	}

	var resp AlpacaOrder
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse order response: %w", err)
	}

	t.clearCache()

	return map[string]interface{}{
		"orderId":  resp.ID,
		"clientId": resp.ClientOrderID,
		"symbol":   resp.Symbol,
		"side":     resp.Side,
		"qty":      resp.Qty,
		"type":     resp.Type,
		"status":   resp.Status,
	}, nil
}

// OpenShort is not supported for basic stock accounts
func (t *AlpacaTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling not supported on Alpaca basic account")
}

// CloseLong sells shares (market order). quantity=0 means close entire position.
func (t *AlpacaTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if quantity == 0 {
		// Close entire position via DELETE endpoint
		logger.Infof("[Alpaca] CLOSE ALL %s", symbol)
		data, err := t.doDelete("/v2/positions/" + symbol)
		if err != nil {
			return nil, fmt.Errorf("close position %s: %w", symbol, err)
		}

		var resp AlpacaOrder
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parse close response: %w", err)
		}

		t.clearCache()
		return map[string]interface{}{
			"orderId": resp.ID,
			"symbol":  resp.Symbol,
			"status":  resp.Status,
		}, nil
	}

	// Partial close via sell order
	logger.Infof("[Alpaca] SELL %s qty=%.4f", symbol, quantity)
	order := AlpacaOrderRequest{
		Symbol:      symbol,
		Qty:         fmt.Sprintf("%.4f", quantity),
		Side:        "sell",
		Type:        "market",
		TimeInForce: "day",
	}

	data, err := t.doPost("/v2/orders", order)
	if err != nil {
		return nil, fmt.Errorf("sell %s: %w", symbol, err)
	}

	var resp AlpacaOrder
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse order response: %w", err)
	}

	t.clearCache()
	return map[string]interface{}{
		"orderId": resp.ID,
		"symbol":  resp.Symbol,
		"side":    resp.Side,
		"qty":     resp.Qty,
		"status":  resp.Status,
	}, nil
}

// CloseShort is not supported
func (t *AlpacaTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling not supported on Alpaca basic account")
}

// SetLeverage is a no-op for stocks (always 1x)
func (t *AlpacaTrader) SetLeverage(symbol string, leverage int) error {
	// Stocks don't have configurable leverage
	return nil
}

// SetMarginMode is a no-op for stocks
func (t *AlpacaTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

// GetMarketPrice returns the latest trade price for a symbol
func (t *AlpacaTrader) GetMarketPrice(symbol string) (float64, error) {
	// Use Alpaca's latest trade endpoint
	data, err := t.doDataGet("/v2/stocks/" + symbol + "/trades/latest")
	if err != nil {
		return 0, fmt.Errorf("get price %s: %w", symbol, err)
	}

	var resp struct {
		Trade struct {
			Price float64 `json:"p"`
		} `json:"trade"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parse price: %w", err)
	}

	if resp.Trade.Price <= 0 {
		return 0, fmt.Errorf("no price data for %s", symbol)
	}

	return resp.Trade.Price, nil
}

// SetStopLoss places a stop order
// SetStopLoss places a stop order. Uses "nofxi-sl-" client_order_id prefix
// so CancelStopLossOrders can distinguish SL orders from user-placed stop orders.
func (t *AlpacaTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := "sell" // stop loss for long = sell when price drops
	if positionSide == "SHORT" {
		side = "buy"
	}

	order := map[string]string{
		"symbol":          symbol,
		"qty":             fmt.Sprintf("%.4f", quantity),
		"side":            side,
		"type":            "stop",
		"time_in_force":   "gtc",
		"stop_price":      fmt.Sprintf("%.2f", stopPrice),
		"client_order_id": fmt.Sprintf("nofxi-sl-%s-%d", symbol, time.Now().UnixNano()),
	}

	_, err := t.doPost("/v2/orders", order)
	return err
}

// SetTakeProfit places a limit order as take-profit.
// Uses a "nofxi-tp-" client_order_id prefix so CancelTakeProfitOrders can
// distinguish TP orders from user-placed limit orders.
func (t *AlpacaTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := "sell" // take profit for long = sell when price rises
	if positionSide == "SHORT" {
		side = "buy"
	}

	order := map[string]string{
		"symbol":          symbol,
		"qty":             fmt.Sprintf("%.4f", quantity),
		"side":            side,
		"type":            "limit",
		"time_in_force":   "gtc",
		"limit_price":     fmt.Sprintf("%.2f", takeProfitPrice),
		"client_order_id": fmt.Sprintf("nofxi-tp-%s-%d", symbol, time.Now().UnixNano()),
	}

	_, err := t.doPost("/v2/orders", order)
	return err
}

// CancelStopLossOrders cancels stop orders for a symbol
// CancelStopLossOrders cancels only NOFXi-created stop-loss orders
// (identified by "nofxi-sl-" client_order_id prefix).
func (t *AlpacaTrader) CancelStopLossOrders(symbol string) error {
	orders, err := t.getOpenOrdersRaw(symbol)
	if err != nil {
		return err
	}
	for _, o := range orders {
		if strings.EqualFold(o.Type, "stop") && strings.HasPrefix(o.ClientOrderID, "nofxi-sl-") {
			if _, err := t.doDelete("/v2/orders/" + o.ID); err != nil {
				logger.Warnf("[Alpaca] cancel SL order %s: %v", o.ID, err)
			}
		}
	}
	return nil
}

// CancelTakeProfitOrders cancels only NOFXi-created take-profit limit orders
// for a symbol (identified by "nofxi-tp-" client_order_id prefix).
// This avoids accidentally canceling user-placed limit buy/sell orders.
func (t *AlpacaTrader) CancelTakeProfitOrders(symbol string) error {
	orders, err := t.getOpenOrdersRaw(symbol)
	if err != nil {
		return err
	}
	for _, o := range orders {
		if strings.EqualFold(o.Type, "limit") && strings.HasPrefix(o.ClientOrderID, "nofxi-tp-") {
			if _, err := t.doDelete("/v2/orders/" + o.ID); err != nil {
				logger.Warnf("[Alpaca] cancel TP order %s: %v", o.ID, err)
			}
		}
	}
	return nil
}

// CancelAllOrders cancels all pending orders for a symbol.
// If symbol is empty, cancels ALL orders across all symbols.
func (t *AlpacaTrader) CancelAllOrders(symbol string) error {
	if symbol == "" {
		_, err := t.doDelete("/v2/orders")
		return err
	}
	// Filter by symbol: get open orders for this symbol, then cancel each
	orders, err := t.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("get open orders for %s: %w", symbol, err)
	}
	for _, o := range orders {
		if _, err := t.doDelete("/v2/orders/" + o.OrderID); err != nil {
			logger.Warnf("[Alpaca] cancel order %s: %v", o.OrderID, err)
		}
	}
	return nil
}

// CancelStopOrders cancels both stop and limit orders for a symbol
func (t *AlpacaTrader) CancelStopOrders(symbol string) error {
	if err := t.CancelStopLossOrders(symbol); err != nil {
		logger.Warnf("[Alpaca] cancel stop loss orders: %v", err)
	}
	return t.CancelTakeProfitOrders(symbol)
}

// FormatQuantity formats quantity (Alpaca supports fractional shares to 4 decimals)
func (t *AlpacaTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.4f", quantity), nil
}

// GetOrderStatus returns the status of an order
func (t *AlpacaTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	data, err := t.doGet("/v2/orders/" + orderID)
	if err != nil {
		return nil, fmt.Errorf("get order %s: %w", orderID, err)
	}

	var order AlpacaOrder
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("parse order: %w", err)
	}

	return map[string]interface{}{
		"status":       strings.ToUpper(order.Status),
		"avgPrice":     parseFloatStr(order.FilledAvgPrice),
		"executedQty":  parseFloatStr(order.FilledQty),
		"commission":   0.0, // Alpaca is commission-free
	}, nil
}

// GetClosedPnL returns closed position records from Alpaca's closed orders.
// Alpaca doesn't track PnL directly, so we reconstruct from filled sell orders.
func (t *AlpacaTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	path := fmt.Sprintf("/v2/orders?status=closed&direction=desc&limit=%d&after=%s",
		limit, startTime.Format(time.RFC3339))

	data, err := t.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("get closed orders: %w", err)
	}

	var orders []AlpacaOrder
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("parse closed orders: %w", err)
	}

	var records []ClosedPnLRecord
	for _, o := range orders {
		// Only include filled sell orders (closing a long position)
		if o.Status != "filled" || o.Side != "sell" {
			continue
		}

		filledQty := parseFloatStr(o.FilledQty)
		filledPrice := parseFloatStr(o.FilledAvgPrice)
		if filledQty <= 0 || filledPrice <= 0 {
			continue
		}

		closeTime, _ := time.Parse(time.RFC3339Nano, o.FilledAt)
		if closeTime.IsZero() {
			closeTime, _ = time.Parse(time.RFC3339Nano, o.UpdatedAt)
		}

		records = append(records, ClosedPnLRecord{
			Symbol:    o.Symbol,
			Side:      "long", // Sell orders close long positions
			ExitPrice: filledPrice,
			Quantity:  filledQty,
			ExitTime:  closeTime,
			OrderID:   o.ID,
			CloseType: "manual",
			Fee:       0, // Alpaca is commission-free for most stocks
			Leverage:  1,
		})
	}

	return records, nil
}

// GetOpenOrders returns open orders
func (t *AlpacaTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	path := "/v2/orders?status=open"
	if symbol != "" {
		path += "&symbols=" + symbol
	}

	data, err := t.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("get open orders: %w", err)
	}

	var orders []AlpacaOrder
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("parse orders: %w", err)
	}

	var result []OpenOrder
	for _, o := range orders {
		oo := OpenOrder{
			OrderID:  o.ID,
			Symbol:   o.Symbol,
			Side:     strings.ToUpper(o.Side),
			Type:     strings.ToUpper(o.Type),
			Price:    parseFloatStr(o.LimitPrice),
			Quantity: parseFloatStr(o.Qty),
			Status:   strings.ToUpper(o.Status),
		}
		if o.StopPrice != "" {
			oo.StopPrice = parseFloatStr(o.StopPrice)
		}
		result = append(result, oo)
	}

	return result, nil
}

// IsMarketOpen checks Alpaca's clock endpoint to determine if the market is open.
func (t *AlpacaTrader) IsMarketOpen() (bool, string, error) {
	data, err := t.doGet("/v2/clock")
	if err != nil {
		return false, "", fmt.Errorf("get clock: %w", err)
	}

	var clock struct {
		IsOpen    bool   `json:"is_open"`
		NextOpen  string `json:"next_open"`
		NextClose string `json:"next_close"`
	}
	if err := json.Unmarshal(data, &clock); err != nil {
		return false, "", fmt.Errorf("parse clock: %w", err)
	}

	status := "open"
	if !clock.IsOpen {
		status = fmt.Sprintf("closed (opens %s)", clock.NextOpen)
	}
	return clock.IsOpen, status, nil
}

// --- Helper: cancel orders by type ---

// getOpenOrdersRaw returns raw AlpacaOrder structs (with ClientOrderID) for filtering.
func (t *AlpacaTrader) getOpenOrdersRaw(symbol string) ([]AlpacaOrder, error) {
	path := "/v2/orders?status=open"
	if symbol != "" {
		path += "&symbols=" + symbol
	}
	data, err := t.doGet(path)
	if err != nil {
		return nil, fmt.Errorf("get open orders: %w", err)
	}
	var orders []AlpacaOrder
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("parse orders: %w", err)
	}
	return orders, nil
}

func (t *AlpacaTrader) cancelOrdersByType(symbol, orderType string) error {
	orders, err := t.GetOpenOrders(symbol)
	if err != nil {
		return err
	}

	for _, o := range orders {
		if strings.EqualFold(o.Type, orderType) && (symbol == "" || o.Symbol == symbol) {
			if _, err := t.doDelete("/v2/orders/" + o.OrderID); err != nil {
				logger.Warnf("[Alpaca] cancel order %s: %v", o.OrderID, err)
			}
		}
	}
	return nil
}

func (t *AlpacaTrader) clearCache() {
	t.cacheMutex.Lock()
	defer t.cacheMutex.Unlock()
	t.cachedBalance = nil
	t.cachedPositions = nil
}

// --- Helpers ---

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func parseFloatStr(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
