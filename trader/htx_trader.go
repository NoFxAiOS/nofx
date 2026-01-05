package trader

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nofx/logger"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// HTX API endpoints
const (
	// Updated to HTX contract-specific API domain (2026-01-05)
	// Note: HTX uses different domains for different services:
	// - Spot trading: api.htx.com
	// - Linear Swap (USDT contracts): api.hbdm.com
	// Previous: api.huobi.pro -> New: api.hbdm.com (for linear swap)
	htxBaseURL               = "https://api.hbdm.com"
	htxAccountPath           = "/v2/account/asset-valuation"
	htxContractAccountPath   = "/linear-swap-api/v1/swap_account_info"
	htxPositionPath          = "/linear-swap-api/v1/swap_position_info"
	htxOrderPath             = "/linear-swap-api/v1/swap_order"
	htxLeveragePath          = "/linear-swap-api/v1/swap_switch_lever_rate"
	htxTickerPath            = "/linear-swap-ex/market/detail/merged"
	htxContractInfoPath      = "/linear-swap-api/v1/swap_contract_info"
	htxCancelOrderPath       = "/linear-swap-api/v1/swap_cancel"
	htxOpenOrdersPath        = "/linear-swap-api/v1/swap_openorders"
	htxTriggerOrderPath      = "/linear-swap-api/v1/swap_trigger_order"
	htxCancelTriggerPath     = "/linear-swap-api/v1/swap_trigger_cancel"
	htxTriggerOpenOrdersPath = "/linear-swap-api/v1/swap_trigger_openorders"
)

// HTXTrader HTX (Huobi) futures trader
type HTXTrader struct {
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
	contractsCache      map[string]*HTXContract
	contractsCacheTime  time.Time
	contractsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// HTXContract HTX contract info
type HTXContract struct {
	Symbol         string  // Contract code (e.g., "BTC-USDT")
	ContractCode   string  // Contract code
	ContractSize   float64 // Contract value
	PriceTick      float64 // Minimum price increment
	MinOrderVolume int     // Minimum order volume (contracts)
	MaxOrderVolume int     // Maximum order volume (contracts)
}

// HTXResponse HTX API response
type HTXResponse struct {
	Status  string          `json:"status"`
	Ts      int64           `json:"ts"`
	Data    json.RawMessage `json:"data"`
	ErrCode string          `json:"err_code"`
	ErrMsg  string          `json:"err_msg"`
}

// NewHTXTrader creates an HTX trader
func NewHTXTrader(apiKey, secretKey string) *HTXTrader {
	// HTX (Huobi) uses domestic servers but may still have network issues
	// Increased timeout to 60s for better reliability
	httpClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: http.DefaultTransport,
	}

	trader := &HTXTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		httpClient:     httpClient,
		cacheDuration:  15 * time.Second,
		contractsCache: make(map[string]*HTXContract),
	}

	logger.Infof("✅ [HTX] Trader initialized (API: %s, Timeout: 60s)", htxBaseURL)

	return trader
}

// sign generates HTX API signature
func (t *HTXTrader) sign(method, host, path string, params map[string]string) string {
	// HTX signature: BASE64(HMAC_SHA256(payload, secretKey))
	// payload = method + "\n" + host + "\n" + path + "\n" + sortedParams

	// Sort parameters
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramParts []string
	for _, k := range keys {
		paramParts = append(paramParts, fmt.Sprintf("%s=%s", k, url.QueryEscape(params[k])))
	}
	sortedParams := strings.Join(paramParts, "&")

	payload := method + "\n" + host + "\n" + path + "\n" + sortedParams

	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest executes HTTP request with signature
func (t *HTXTrader) doRequest(method, path string, params map[string]string, body interface{}) ([]byte, error) {
	u, _ := url.Parse(htxBaseURL + path)

	// Add timestamp and signature
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05")
	if params == nil {
		params = make(map[string]string)
	}
	params["AccessKeyId"] = t.apiKey
	params["SignatureMethod"] = "HmacSHA256"
	params["SignatureVersion"] = "2"
	params["Timestamp"] = timestamp

	// Generate signature
	signature := t.sign(method, u.Host, u.Path, params)
	params["Signature"] = signature

	// Build query string
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip response: %w", err)
		}
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp HTXResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Status != "ok" {
		return nil, fmt.Errorf("API error: %s - %s", apiResp.ErrCode, apiResp.ErrMsg)
	}

	return apiResp.Data, nil
}

// GetBalance returns account balance
func (t *HTXTrader) GetBalance() (map[string]interface{}, error) {
	t.balanceCacheMutex.RLock()
	if time.Since(t.balanceCacheTime) < t.cacheDuration && t.cachedBalance != nil {
		defer t.balanceCacheMutex.RUnlock()
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	data, err := t.doRequest("POST", htxContractAccountPath, nil, nil)
	if err != nil {
		return nil, err
	}

	var accounts []struct {
		MarginBalance     float64 `json:"margin_balance"`
		MarginAvailable   float64 `json:"margin_available"`
		WithdrawAvailable float64 `json:"withdraw_available"`
		UnrealizedProfit  float64 `json:"unrealized_profit"`
	}

	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("failed to parse balance: %w", err)
	}

	// Use decimal for precise financial calculations
	totalBalanceDec := decimal.Zero
	availableBalanceDec := decimal.Zero
	unrealizedPnLDec := decimal.Zero

	for _, acc := range accounts {
		totalBalanceDec = totalBalanceDec.Add(decimal.NewFromFloat(acc.MarginBalance))
		availableBalanceDec = availableBalanceDec.Add(decimal.NewFromFloat(acc.MarginAvailable))
		unrealizedPnLDec = unrealizedPnLDec.Add(decimal.NewFromFloat(acc.UnrealizedProfit))
	}

	// Convert to float64 for interface compatibility
	totalBalance, _ := totalBalanceDec.Float64()
	availableBalance, _ := availableBalanceDec.Float64()
	unrealizedPnL, _ := unrealizedPnLDec.Float64()

	result := map[string]interface{}{
		// Standard fields (camelCase) for compatibility with AutoTrader.GetAccountInfo
		"totalEquity":           totalBalance,
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": unrealizedPnL,

		// Legacy fields (snake_case) for backward compatibility
		"total_equity":      totalBalance,
		"available_balance": availableBalance,
		"unrealized_pnl":    unrealizedPnL,
	}

	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions returns all positions
func (t *HTXTrader) GetPositions() ([]map[string]interface{}, error) {
	t.positionsCacheMutex.RLock()
	if time.Since(t.positionsCacheTime) < t.cacheDuration && t.cachedPositions != nil {
		defer t.positionsCacheMutex.RUnlock()
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	data, err := t.doRequest("POST", htxPositionPath, nil, nil)
	if err != nil {
		return nil, err
	}

	var positions []struct {
		Symbol    string  `json:"contract_code"`
		Volume    float64 `json:"volume"`
		Direction string  `json:"direction"` // "buy" or "sell"
		AvgCost   float64 `json:"cost_open"`
		LeverRate int     `json:"lever_rate"`
		Profit    float64 `json:"profit"`
		LastPrice float64 `json:"last_price"`
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		if pos.Volume == 0 {
			continue
		}

		side := "long"
		if pos.Direction == "sell" {
			side = "short"
		}

		// Use decimal for precise financial calculations
		avgCostDec := decimal.NewFromFloat(pos.AvgCost)
		lastPriceDec := decimal.NewFromFloat(pos.LastPrice)
		// profitDec := decimal.NewFromFloat(pos.Profit) // Not needed for calculations

		// Calculate PnL percentage using decimal for precision
		pnlPct := 0.0
		if !avgCostDec.IsZero() {
			if side == "long" {
				pnlPct, _ = lastPriceDec.Sub(avgCostDec).Div(avgCostDec).Mul(decimal.NewFromInt(100)).Float64()
			} else {
				pnlPct, _ = avgCostDec.Sub(lastPriceDec).Div(avgCostDec).Mul(decimal.NewFromInt(100)).Float64()
			}
		}

		result = append(result, map[string]interface{}{
			// Standard fields (camelCase) for compatibility with AutoTrader.GetAccountInfo
			"symbol":           pos.Symbol,
			"side":             side,
			"markPrice":        pos.LastPrice, // Mark price (camelCase)
			"positionAmt":      pos.Volume,    // Position amount (camelCase)
			"entryPrice":       pos.AvgCost,
			"unRealizedProfit": pos.Profit, // Unrealized PnL (camelCase)
			"leverage":         pos.LeverRate,
			"liquidationPrice": 0.0, // HTX doesn't provide liquidation price in this API

			// Additional fields
			"quantity":           pos.Volume,
			"unrealized_pnl":     pos.Profit, // Legacy snake_case
			"unrealized_pnl_pct": pnlPct,
			"entry_price":        pos.AvgCost,   // Legacy snake_case
			"mark_price":         pos.LastPrice, // Legacy snake_case
		})
	}

	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// OpenLong opens a long position
func (t *HTXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// HTX uses contract notation (e.g., "BTC-USDT")
	symbol = t.normalizeSymbol(symbol)

	// Check if contract is supported
	supported, checkErr := t.isContractSupported(symbol)
	if checkErr != nil {
		logger.Warnf("[HTX] Failed to verify contract support: %v", checkErr)
		// Continue anyway, will fail at order placement if not supported
	} else if !supported {
		return nil, fmt.Errorf("contract %s is not supported on HTX USDT perpetual futures", symbol)
	}

	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ [HTX] Failed to set leverage: %v", err)
	}

	// Generate unique client order ID
	clientOrderID := fmt.Sprintf("nofx_%d", time.Now().UnixNano())

	// HTX requires volume as integer (number of contracts)
	quantityDec := decimal.NewFromFloat(quantity)
	volume := int(quantityDec.Round(0).IntPart())

	// HTX minimum order volume is 1 contract
	if volume < 1 {
		return nil, fmt.Errorf("order quantity %.8f rounds to %d, minimum is 1 contract (increase position size or adjust price)", quantity, volume)
	}

	logger.Infof("[HTX] OpenLong: quantity=%.8f -> volume=%d", quantity, volume)

	body := map[string]interface{}{
		"contract_code":    symbol,
		"direction":        "buy",
		"offset":           "open",
		"lever_rate":       leverage,
		"volume":           volume,
		"order_price_type": "optimal_20", // Market order
		"client_order_id":  clientOrderID,
	}

	data, err := t.doRequest("POST", htxOrderPath, nil, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	return map[string]interface{}{
		"order_id": strconv.FormatInt(result.OrderID, 10),
		"symbol":   symbol,
		"side":     "long",
		"quantity": quantity,
	}, nil
}

// OpenShort opens a short position
func (t *HTXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	// Check if contract is supported
	supported, checkErr := t.isContractSupported(symbol)
	if checkErr != nil {
		logger.Warnf("[HTX] Failed to verify contract support: %v", checkErr)
		// Continue anyway, will fail at order placement if not supported
	} else if !supported {
		return nil, fmt.Errorf("contract %s is not supported on HTX USDT perpetual futures", symbol)
	}

	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ [HTX] Failed to set leverage: %v", err)
	}

	// Generate unique client order ID
	clientOrderID := fmt.Sprintf("nofx_%d", time.Now().UnixNano())

	// HTX requires volume as integer (number of contracts)
	quantityDec := decimal.NewFromFloat(quantity)
	volume := int(quantityDec.Round(0).IntPart())

	// HTX minimum order volume is 1 contract
	if volume < 1 {
		return nil, fmt.Errorf("order quantity %.8f rounds to %d, minimum is 1 contract (increase position size or adjust price)", quantity, volume)
	}

	logger.Infof("[HTX] OpenShort: quantity=%.8f -> volume=%d", quantity, volume)

	body := map[string]interface{}{
		"contract_code":    symbol,
		"direction":        "sell",
		"offset":           "open",
		"lever_rate":       leverage,
		"volume":           volume,
		"order_price_type": "optimal_20",
		"client_order_id":  clientOrderID,
	}

	data, err := t.doRequest("POST", htxOrderPath, nil, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	return map[string]interface{}{
		"order_id": strconv.FormatInt(result.OrderID, 10),
		"symbol":   symbol,
		"side":     "short",
		"quantity": quantity,
	}, nil
}

// CloseLong closes a long position
func (t *HTXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	// Get current position to determine quantity if not specified
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["quantity"].(float64)
				break
			}
		}
	}

	// Generate unique client order ID
	clientOrderID := fmt.Sprintf("nofx_%d", time.Now().UnixNano())

	quantityDec := decimal.NewFromFloat(quantity)
	volume := int(quantityDec.Round(0).IntPart())

	// HTX minimum order volume is 1 contract
	if volume < 1 {
		return nil, fmt.Errorf("close quantity %.8f rounds to %d, minimum is 1 contract", quantity, volume)
	}

	logger.Infof("[HTX] CloseLong: quantity=%.8f -> volume=%d", quantity, volume)

	body := map[string]interface{}{
		"contract_code":    symbol,
		"direction":        "sell",
		"offset":           "close",
		"volume":           volume,
		"order_price_type": "optimal_20",
		"client_order_id":  clientOrderID,
	}

	data, err := t.doRequest("POST", htxOrderPath, nil, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	return map[string]interface{}{
		"order_id": strconv.FormatInt(result.OrderID, 10),
		"symbol":   symbol,
		"side":     "close_long",
		"quantity": quantity,
	}, nil
}

// CloseShort closes a short position
func (t *HTXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["quantity"].(float64)
				break
			}
		}
	}

	// Generate unique client order ID
	clientOrderID := fmt.Sprintf("nofx_%d", time.Now().UnixNano())

	quantityDec := decimal.NewFromFloat(quantity)
	volume := int(quantityDec.Round(0).IntPart())

	// HTX minimum order volume is 1 contract
	if volume < 1 {
		return nil, fmt.Errorf("close quantity %.8f rounds to %d, minimum is 1 contract", quantity, volume)
	}

	logger.Infof("[HTX] CloseShort: quantity=%.8f -> volume=%d", quantity, volume)

	body := map[string]interface{}{
		"contract_code":    symbol,
		"direction":        "buy",
		"offset":           "close",
		"volume":           volume,
		"order_price_type": "optimal_20",
		"client_order_id":  clientOrderID,
	}

	data, err := t.doRequest("POST", htxOrderPath, nil, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	return map[string]interface{}{
		"order_id": strconv.FormatInt(result.OrderID, 10),
		"symbol":   symbol,
		"side":     "close_short",
		"quantity": quantity,
	}, nil
}

// SetLeverage sets leverage
func (t *HTXTrader) SetLeverage(symbol string, leverage int) error {
	symbol = t.normalizeSymbol(symbol)

	body := map[string]interface{}{
		"contract_code": symbol,
		"lever_rate":    leverage,
	}

	_, err := t.doRequest("POST", htxLeveragePath, nil, body)
	return err
}

// SetMarginMode sets margin mode (HTX uses cross margin by default)
func (t *HTXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// HTX linear swaps use cross margin mode by default
	// Isolated margin requires separate API calls not commonly used
	return nil
}

// GetMarketPrice returns current market price
func (t *HTXTrader) GetMarketPrice(symbol string) (float64, error) {
	symbol = t.normalizeSymbol(symbol)

	params := map[string]string{
		"contract_code": symbol,
	}

	req, _ := http.NewRequest("GET", htxBaseURL+htxTickerPath, nil)
	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var result struct {
		Tick struct {
			Close float64 `json:"close"`
		} `json:"tick"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return 0, err
	}

	return result.Tick.Close, nil
}

// SetStopLoss sets stop-loss order
func (t *HTXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	symbol = t.normalizeSymbol(symbol)

	direction := "sell"
	if positionSide == "short" {
		direction = "buy"
	}

	body := map[string]interface{}{
		"contract_code":    symbol,
		"trigger_type":     "le", // Less than or equal
		"trigger_price":    fmt.Sprintf("%.8f", stopPrice),
		"order_price":      fmt.Sprintf("%.8f", stopPrice),
		"order_price_type": "limit",
		"volume":           int(quantity),
		"direction":        direction,
		"offset":           "close",
	}

	_, err := t.doRequest("POST", htxTriggerOrderPath, nil, body)
	return err
}

// SetTakeProfit sets take-profit order
func (t *HTXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	symbol = t.normalizeSymbol(symbol)

	direction := "sell"
	if positionSide == "short" {
		direction = "buy"
	}

	body := map[string]interface{}{
		"contract_code":    symbol,
		"trigger_type":     "ge", // Greater than or equal
		"trigger_price":    fmt.Sprintf("%.8f", takeProfitPrice),
		"order_price":      fmt.Sprintf("%.8f", takeProfitPrice),
		"order_price_type": "limit",
		"volume":           int(quantity),
		"direction":        direction,
		"offset":           "close",
	}

	_, err := t.doRequest("POST", htxTriggerOrderPath, nil, body)
	return err
}

// CancelStopLossOrders cancels stop-loss orders
func (t *HTXTrader) CancelStopLossOrders(symbol string) error {
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders cancels take-profit orders
func (t *HTXTrader) CancelTakeProfitOrders(symbol string) error {
	return t.CancelStopOrders(symbol)
}

// CancelAllOrders cancels all pending orders
func (t *HTXTrader) CancelAllOrders(symbol string) error {
	symbol = t.normalizeSymbol(symbol)

	// 先查询所有挂单
	body := map[string]interface{}{
		"contract_code": symbol,
	}

	data, err := t.doRequest("POST", htxOpenOrdersPath, nil, body)
	if err != nil {
		return err
	}

	var response struct {
		Orders []struct {
			OrderID int64 `json:"order_id"`
		} `json:"orders"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}

	// 取消每个订单
	for _, order := range response.Orders {
		cancelBody := map[string]interface{}{
			"contract_code": symbol,
			"order_id":      strconv.FormatInt(order.OrderID, 10),
		}
		_, _ = t.doRequest("POST", htxCancelOrderPath, nil, cancelBody)
	}

	return nil
}

// CancelStopOrders cancels all stop orders (stop-loss and take-profit)
func (t *HTXTrader) CancelStopOrders(symbol string) error {
	symbol = t.normalizeSymbol(symbol)

	// Get all trigger orders
	params := map[string]string{
		"contract_code": symbol,
	}

	data, err := t.doRequest("POST", htxTriggerOpenOrdersPath, params, nil)
	if err != nil {
		return err
	}

	var orders struct {
		Orders []struct {
			OrderID string `json:"order_id"`
		} `json:"orders"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	// Cancel each order
	for _, order := range orders.Orders {
		body := map[string]interface{}{
			"contract_code": symbol,
			"order_id":      order.OrderID,
		}
		_, _ = t.doRequest("POST", htxCancelTriggerPath, nil, body)
	}

	return nil
}

// FormatQuantity formats quantity to correct precision
func (t *HTXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// HTX uses integer contracts
	return strconv.Itoa(int(quantity)), nil
}

// GetOrderStatus returns order status
func (t *HTXTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	symbol = t.normalizeSymbol(symbol)

	body := map[string]interface{}{
		"contract_code": symbol,
		"order_id":      orderID,
	}

	data, err := t.doRequest("POST", "/linear-swap-api/v1/swap_order_info", nil, body)
	if err != nil {
		return nil, err
	}

	var response []struct {
		OrderID       int64   `json:"order_id"`
		Status        int     `json:"status"` // 1:准备提交 2:准备提交 3:已提交 4:部分成交 5:部分成交已撤销 6:全部成交 7:已撤销
		TradeAvgPrice string  `json:"trade_avg_price"`
		TradeVolume   float64 `json:"trade_volume"`
		Fee           float64 `json:"fee"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse order status: %w", err)
	}

	if len(response) == 0 {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	order := response[0]

	// 转换状态
	var status string
	switch order.Status {
	case 1, 2, 3:
		status = "NEW"
	case 4:
		status = "PARTIALLY_FILLED"
	case 6:
		status = "FILLED"
	case 5, 7:
		status = "CANCELED"
	default:
		status = "UNKNOWN"
	}

	avgPrice, _ := strconv.ParseFloat(order.TradeAvgPrice, 64)

	return map[string]interface{}{
		"status":       status,
		"order_id":     strconv.FormatInt(order.OrderID, 10),
		"avg_price":    avgPrice,
		"executed_qty": order.TradeVolume,
		"commission":   order.Fee,
	}, nil
}

// GetClosedPnL returns closed position P&L records
func (t *HTXTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	// HTX closed P&L query would require historical trades API
	// Returning empty for now
	return []ClosedPnLRecord{}, nil
}

// normalizeSymbol converts symbol to HTX format
func (t *HTXTrader) normalizeSymbol(symbol string) string {
	// Convert BTCUSDT -> BTC-USDT
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") && !strings.Contains(symbol, "-") {
		base := strings.TrimSuffix(symbol, "USDT")
		return base + "-USDT"
	}
	return symbol
}

// isContractSupported checks if a contract is supported on HTX
func (t *HTXTrader) isContractSupported(symbol string) (bool, error) {
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
	data, err := t.doRequest("POST", htxContractInfoPath, nil, nil)
	if err != nil {
		return false, fmt.Errorf("failed to fetch contracts: %w", err)
	}

	var contracts []struct {
		ContractCode string `json:"contract_code"`
	}
	if err := json.Unmarshal(data, &contracts); err != nil {
		return false, fmt.Errorf("failed to parse contracts: %w", err)
	}

	// Update cache
	t.contractsCacheMutex.Lock()
	t.contractsCache = make(map[string]*HTXContract)
	for _, c := range contracts {
		t.contractsCache[c.ContractCode] = &HTXContract{ContractCode: c.ContractCode}
	}
	t.contractsCacheTime = time.Now()
	t.contractsCacheMutex.Unlock()

	_, exists := t.contractsCache[symbol]
	return exists, nil
}
