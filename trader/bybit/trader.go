package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	bybit "github.com/bybit-exchange/bybit.go.api"
	"nofx/trader/types"
)

// BybitTrader Bybit USDT Perpetual Futures Trader
type BybitTrader struct {
	client    *bybit.Client
	apiKey    string
	secretKey string

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Trading pair precision cache (symbol -> qtyStep)
	qtyStepCache      map[string]float64
	qtyStepCacheMutex sync.RWMutex

	// Cache duration (15 seconds)
	cacheDuration time.Duration
}

// NewBybitTrader creates a Bybit trader
func NewBybitTrader(apiKey, secretKey string) *BybitTrader {
	const src = "Up000938"

	client := bybit.NewBybitHttpClient(apiKey, secretKey, bybit.WithBaseURL(bybit.MAINNET))

	// Set HTTP transport
	if client != nil && client.HTTPClient != nil {
		defaultTransport := client.HTTPClient.Transport
		if defaultTransport == nil {
			defaultTransport = http.DefaultTransport
		}

		client.HTTPClient.Transport = &headerRoundTripper{
			base:      defaultTransport,
			refererID: src,
		}
	}

	trader := &BybitTrader{
		client:        client,
		apiKey:        apiKey,
		secretKey:     secretKey,
		cacheDuration: 15 * time.Second,
		qtyStepCache:  make(map[string]float64),
	}

	logger.Infof("🔵 [Bybit] Trader initialized")

	return trader
}

// headerRoundTripper HTTP RoundTripper for adding custom headers
type headerRoundTripper struct {
	base      http.RoundTripper
	refererID string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Referer", h.refererID)
	return h.base.RoundTrip(req)
}

// GetBalance retrieves account balance
func (t *BybitTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		balance := t.cachedBalance
		t.balanceCacheMutex.RUnlock()
		return balance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Call API
	params := map[string]interface{}{
		"accountType": "UNIFIED",
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetAccountWallet(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get Bybit balance: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	// Extract balance information
	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Bybit balance return format error")
	}

	list, _ := resultData["list"].([]interface{})

	var totalEquity, availableBalance, totalWalletBalance, totalPerpUPL float64 = 0, 0, 0, 0

	if len(list) > 0 {
		account, _ := list[0].(map[string]interface{})
		if equityStr, ok := account["totalEquity"].(string); ok {
			totalEquity, _ = strconv.ParseFloat(equityStr, 64)
		}
		if availStr, ok := account["totalAvailableBalance"].(string); ok {
			availableBalance, _ = strconv.ParseFloat(availStr, 64)
		}
		// Bybit UNIFIED account wallet balance field
		if walletStr, ok := account["totalWalletBalance"].(string); ok {
			totalWalletBalance, _ = strconv.ParseFloat(walletStr, 64)
		}
		// Bybit perpetual contract unrealized PnL
		if uplStr, ok := account["totalPerpUPL"].(string); ok {
			totalPerpUPL, _ = strconv.ParseFloat(uplStr, 64)
		}
	}

	// If no totalWalletBalance, use totalEquity
	if totalWalletBalance == 0 {
		totalWalletBalance = totalEquity
	}

	balance := map[string]interface{}{
		"totalEquity":           totalEquity,
		"totalWalletBalance":    totalWalletBalance,
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": totalPerpUPL,
		"balance":               totalEquity, // Compatible with other exchange formats
	}

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return balance, nil
}

// GetPositions retrieves all positions
func (t *BybitTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		positions := t.cachedPositions
		t.positionsCacheMutex.RUnlock()
		return positions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// Call API
	params := map[string]interface{}{
		"category":   "linear",
		"settleCoin": "USDT",
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetPositionList(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get Bybit positions: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Bybit positions return format error")
	}

	list, _ := resultData["list"].([]interface{})

	var positions []map[string]interface{}

	for _, item := range list {
		pos, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		sizeStr, _ := pos["size"].(string)
		size, _ := strconv.ParseFloat(sizeStr, 64)

		// Skip empty positions
		if size == 0 {
			continue
		}

		entryPriceStr, _ := pos["avgPrice"].(string)
		entryPrice, _ := strconv.ParseFloat(entryPriceStr, 64)

		unrealisedPnlStr, _ := pos["unrealisedPnl"].(string)
		unrealisedPnl, _ := strconv.ParseFloat(unrealisedPnlStr, 64)

		leverageStr, _ := pos["leverage"].(string)
		leverage, _ := strconv.ParseFloat(leverageStr, 64)

		// Mark price
		markPriceStr, _ := pos["markPrice"].(string)
		markPrice, _ := strconv.ParseFloat(markPriceStr, 64)

		// Liquidation price
		liqPriceStr, _ := pos["liqPrice"].(string)
		liqPrice, _ := strconv.ParseFloat(liqPriceStr, 64)

		// Position created/updated time (milliseconds timestamp)
		createdTimeStr, _ := pos["createdTime"].(string)
		createdTime, _ := strconv.ParseInt(createdTimeStr, 10, 64)
		updatedTimeStr, _ := pos["updatedTime"].(string)
		updatedTime, _ := strconv.ParseInt(updatedTimeStr, 10, 64)

		positionSide, _ := pos["side"].(string) // Buy = long, Sell = short

		// Log raw position data for debugging
		logger.Infof("[Bybit] GetPositions raw: symbol=%v, side=%s, size=%v", pos["symbol"], positionSide, sizeStr)

		// Convert to unified format (use lowercase for consistency with other exchanges)
		// Bybit returns "Buy" for long, "Sell" for short
		side := "long"
		positionAmt := size
		positionSideLower := strings.ToLower(positionSide)
		if positionSideLower == "sell" {
			side = "short"
			positionAmt = -size
		}

		logger.Infof("[Bybit] GetPositions converted: symbol=%v, rawSide=%s -> side=%s", pos["symbol"], positionSide, side)

		position := map[string]interface{}{
			"symbol":           pos["symbol"],
			"side":             side,
			"positionAmt":      positionAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealisedPnl,
			"unrealizedPnL":    unrealisedPnl,
			"liquidationPrice": liqPrice,
			"leverage":         leverage,
			"createdTime":      createdTime, // Position open time (ms)
			"updatedTime":      updatedTime, // Position last update time (ms)
		}

		positions = append(positions, position)
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = positions
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return positions, nil
}

// OpenLong opens a long position
func (t *BybitTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	logger.Infof("[Bybit] ===== OpenLong called: symbol=%s, qty=%.6f, leverage=%d =====", symbol, quantity, leverage)

	// First cancel all pending orders for this symbol (clean up old orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to cancel old pending orders: %v", err)
	}
	// Also cancel conditional orders (stop-loss/take-profit) - Bybit keeps them separate
	if err := t.CancelStopOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to cancel old stop orders: %v", err)
	}

	// Set leverage first
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to set leverage: %v", err)
	}

	// Use FormatQuantity to format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Buy",
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0, // One-way position mode
	}

	logger.Infof("[Bybit] OpenLong placing order: %+v", params)

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit open long failed: %w", err)
	}

	// Clear cache
	t.clearCache()

	return t.parseOrderResult(result)
}

// OpenShort opens a short position
func (t *BybitTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	logger.Infof("[Bybit] ===== OpenShort called: symbol=%s, qty=%.6f, leverage=%d =====", symbol, quantity, leverage)

	// First cancel all pending orders for this symbol (clean up old orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to cancel old pending orders: %v", err)
	}
	// Also cancel conditional orders (stop-loss/take-profit) - Bybit keeps them separate
	if err := t.CancelStopOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to cancel old stop orders: %v", err)
	}

	// Set leverage first
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to set leverage: %v", err)
	}

	// Use FormatQuantity to format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Sell",
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0, // One-way position mode
	}

	logger.Infof("[Bybit] OpenShort placing order: %+v", params)

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit open short failed: %w", err)
	}

	// Clear cache
	t.clearCache()

	return t.parseOrderResult(result)
}

// CloseLong closes a long position
func (t *BybitTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity = 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			side, _ := pos["side"].(string)
			if pos["symbol"] == symbol && strings.ToLower(side) == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}
	}

	if quantity <= 0 {
		return nil, fmt.Errorf("no long position to close")
	}

	// Use FormatQuantity to format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Sell", // Close long with Sell
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0,
		"reduceOnly":  true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit close long failed: %w", err)
	}

	// Clear cache
	t.clearCache()

	return t.parseOrderResult(result)
}

// CloseShort closes a short position
func (t *BybitTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity = 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			side, _ := pos["side"].(string)
			if pos["symbol"] == symbol && strings.ToLower(side) == "short" {
				quantity = -pos["positionAmt"].(float64) // Short position is negative
				break
			}
		}
	}

	if quantity <= 0 {
		return nil, fmt.Errorf("no short position to close")
	}

	// Use FormatQuantity to format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Buy", // Close short with Buy
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0,
		"reduceOnly":  true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit close short failed: %w", err)
	}

	// Clear cache
	t.clearCache()

	return t.parseOrderResult(result)
}

// SetLeverage sets leverage
func (t *BybitTrader) SetLeverage(symbol string, leverage int) error {
	params := map[string]interface{}{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  fmt.Sprintf("%d", leverage),
		"sellLeverage": fmt.Sprintf("%d", leverage),
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).SetPositionLeverage(context.Background())
	if err != nil {
		// If leverage is already at target value, Bybit will return an error, ignore this case
		if strings.Contains(err.Error(), "leverage not modified") {
			return nil
		}
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	if result.RetCode != 0 && result.RetCode != 110043 { // 110043 = leverage not modified
		return fmt.Errorf("failed to set leverage: %s", result.RetMsg)
	}

	return nil
}

// SetMarginMode sets position margin mode
func (t *BybitTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	tradeMode := 1 // Isolated margin
	if isCrossMargin {
		tradeMode = 0 // Cross margin
	}

	params := map[string]interface{}{
		"category":  "linear",
		"symbol":    symbol,
		"tradeMode": tradeMode,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).SwitchPositionMargin(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "Cross/isolated margin mode is not modified") {
			return nil
		}
		return fmt.Errorf("failed to set margin mode: %w", err)
	}

	if result.RetCode != 0 && result.RetCode != 110026 { // already in target mode
		return fmt.Errorf("failed to set margin mode: %s", result.RetMsg)
	}

	return nil
}

// GetMarketPrice retrieves market price
func (t *BybitTrader) GetMarketPrice(symbol string) (float64, error) {
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetMarketTickers(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get market price: %w", err)
	}

	if result.RetCode != 0 {
		return 0, fmt.Errorf("API error: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("return format error")
	}

	list, _ := resultData["list"].([]interface{})

	if len(list) == 0 {
		return 0, fmt.Errorf("price data not found for %s", symbol)
	}

	ticker, _ := list[0].(map[string]interface{})
	lastPriceStr, _ := ticker["lastPrice"].(string)
	lastPrice, err := strconv.ParseFloat(lastPriceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price: %w", err)
	}

	return lastPrice, nil
}

// SetStopLoss sets stop loss order
func (t *BybitTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := "Sell" // LONG stop loss uses Sell
	if positionSide == "SHORT" {
		side = "Buy" // SHORT stop loss uses Buy
	}

	// Get current price to determine triggerDirection
	currentPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return err
	}

	triggerDirection := 2 // Price fall trigger (default long stop loss)
	if stopPrice > currentPrice {
		triggerDirection = 1 // Price rise trigger (short stop loss)
	}

	// Use FormatQuantity to format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	params := map[string]interface{}{
		"category":         "linear",
		"symbol":           symbol,
		"side":             side,
		"orderType":        "Market",
		"qty":              qtyStr,
		"triggerPrice":     fmt.Sprintf("%v", stopPrice),
		"triggerDirection": triggerDirection,
		"triggerBy":        "LastPrice",
		"reduceOnly":       true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	if result.RetCode != 0 {
		return fmt.Errorf("failed to set stop loss: %s", result.RetMsg)
	}

	logger.Infof("  ✓ [Bybit] Stop loss order set: %s @ %.2f", symbol, stopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *BybitTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := "Sell" // LONG take profit uses Sell
	if positionSide == "SHORT" {
		side = "Buy" // SHORT take profit uses Buy
	}

	// Get current price to determine triggerDirection
	currentPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return err
	}

	triggerDirection := 1 // Price rise trigger (default long take profit)
	if takeProfitPrice < currentPrice {
		triggerDirection = 2 // Price fall trigger (short take profit)
	}

	// Use FormatQuantity to format quantity
	qtyStr, _ := t.FormatQuantity(symbol, quantity)

	params := map[string]interface{}{
		"category":         "linear",
		"symbol":           symbol,
		"side":             side,
		"orderType":        "Market",
		"qty":              qtyStr,
		"triggerPrice":     fmt.Sprintf("%v", takeProfitPrice),
		"triggerDirection": triggerDirection,
		"triggerBy":        "LastPrice",
		"reduceOnly":       true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	if result.RetCode != 0 {
		return fmt.Errorf("failed to set take profit: %s", result.RetMsg)
	}

	logger.Infof("  ✓ [Bybit] Take profit order set: %s @ %.2f", symbol, takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop loss orders
func (t *BybitTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelConditionalOrders(symbol, "StopLoss")
}

// CancelTakeProfitOrders cancels take profit orders
func (t *BybitTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelConditionalOrders(symbol, "TakeProfit")
}

// CancelAllOrders cancels all pending orders
func (t *BybitTrader) CancelAllOrders(symbol string) error {
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
	}

	_, err := t.client.NewUtaBybitServiceWithParams(params).CancelAllOrders(context.Background())
	if err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}

	return nil
}

// CancelStopOrders cancels all stop loss and take profit orders
func (t *BybitTrader) CancelStopOrders(symbol string) error {
	if err := t.CancelStopLossOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to cancel stop loss orders: %v", err)
	}
	if err := t.CancelTakeProfitOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] Failed to cancel take profit orders: %v", err)
	}
	return nil
}

// getQtyStep retrieves the quantity step for a trading pair
func (t *BybitTrader) getQtyStep(symbol string) float64 {
	// Check cache first
	t.qtyStepCacheMutex.RLock()
	if step, ok := t.qtyStepCache[symbol]; ok {
		t.qtyStepCacheMutex.RUnlock()
		return step
	}
	t.qtyStepCacheMutex.RUnlock()

	// Call public API directly to get contract information
	url := fmt.Sprintf("https://api.bybit.com/v5/market/instruments-info?category=linear&symbol=%s", symbol)
	resp, err := http.Get(url)
	if err != nil {
		logger.Infof("⚠️ [Bybit] Failed to get precision info for %s: %v", symbol, err)
		return 1 // Default to integer
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 1
	}

	var result struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				LotSizeFilter struct {
					QtyStep string `json:"qtyStep"`
				} `json:"lotSizeFilter"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 1
	}

	if result.RetCode != 0 || len(result.Result.List) == 0 {
		return 1
	}

	qtyStep, _ := strconv.ParseFloat(result.Result.List[0].LotSizeFilter.QtyStep, 64)
	if qtyStep <= 0 {
		qtyStep = 1
	}

	// Cache result
	t.qtyStepCacheMutex.Lock()
	t.qtyStepCache[symbol] = qtyStep
	t.qtyStepCacheMutex.Unlock()

	logger.Infof("🔵 [Bybit] %s qtyStep: %v", symbol, qtyStep)

	return qtyStep
}

// FormatQuantity formats quantity
func (t *BybitTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// Get qtyStep for this symbol
	qtyStep := t.getQtyStep(symbol)

	// Align quantity according to qtyStep (round down to nearest step)
	alignedQty := math.Floor(quantity/qtyStep) * qtyStep

	// Calculate required decimal places
	decimals := 0
	if qtyStep < 1 {
		stepStr := strconv.FormatFloat(qtyStep, 'f', -1, 64)
		if idx := strings.Index(stepStr, "."); idx >= 0 {
			decimals = len(stepStr) - idx - 1
		}
	}

	// Format
	format := fmt.Sprintf("%%.%df", decimals)
	formatted := fmt.Sprintf(format, alignedQty)

	return formatted, nil
}

// Helper methods

func (t *BybitTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}

func (t *BybitTrader) parseOrderResult(result *bybit.ServerResponse) (map[string]interface{}, error) {
	if result.RetCode != 0 {
		return nil, fmt.Errorf("order placement failed: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("return format error")
	}

	orderId, _ := resultData["orderId"].(string)

	return map[string]interface{}{
		"orderId": orderId,
		"status":  "NEW",
	}, nil
}

// GetOrderStatus retrieves order status
func (t *BybitTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
		"orderId":  orderID,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetOrderHistory(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("API error: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("return format error")
	}

	list, _ := resultData["list"].([]interface{})
	if len(list) == 0 {
		return nil, fmt.Errorf("order %s not found", orderID)
	}

	order, _ := list[0].(map[string]interface{})

	// Parse order data
	status, _ := order["orderStatus"].(string)
	avgPriceStr, _ := order["avgPrice"].(string)
	cumExecQtyStr, _ := order["cumExecQty"].(string)
	cumExecFeeStr, _ := order["cumExecFee"].(string)

	avgPrice, _ := strconv.ParseFloat(avgPriceStr, 64)
	executedQty, _ := strconv.ParseFloat(cumExecQtyStr, 64)
	commission, _ := strconv.ParseFloat(cumExecFeeStr, 64)

	// Convert status to unified format
	unifiedStatus := status
	switch status {
	case "Filled":
		unifiedStatus = "FILLED"
	case "New", "Created":
		unifiedStatus = "NEW"
	case "Cancelled", "Rejected":
		unifiedStatus = "CANCELED"
	case "PartiallyFilled":
		unifiedStatus = "PARTIALLY_FILLED"
	}

	return map[string]interface{}{
		"orderId":     orderID,
		"status":      unifiedStatus,
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"commission":  commission,
	}, nil
}

func (t *BybitTrader) cancelConditionalOrders(symbol string, orderType string) error {
	// First get all conditional orders
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"orderFilter": "StopOrder", // Conditional orders
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetOpenOrders(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get conditional orders: %w", err)
	}

	if result.RetCode != 0 {
		return nil // No orders
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil
	}

	list, _ := resultData["list"].([]interface{})

	// Cancel matching orders
	for _, item := range list {
		order, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		orderId, _ := order["orderId"].(string)
		stopOrderType, _ := order["stopOrderType"].(string)

		// Filter by type
		shouldCancel := false
		if orderType == "StopLoss" && (stopOrderType == "StopLoss" || stopOrderType == "Stop") {
			shouldCancel = true
		}
		if orderType == "TakeProfit" && (stopOrderType == "TakeProfit" || stopOrderType == "PartialTakeProfit") {
			shouldCancel = true
		}

		if shouldCancel && orderId != "" {
			cancelParams := map[string]interface{}{
				"category": "linear",
				"symbol":   symbol,
				"orderId":  orderId,
			}
			t.client.NewUtaBybitServiceWithParams(cancelParams).CancelOrder(context.Background())
		}
	}

	return nil
}

// GetClosedPnL retrieves closed position PnL records from Bybit via direct HTTP API
func (t *BybitTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// The Bybit SDK doesn't expose the closed-pnl endpoint, use direct HTTP call
	return t.getClosedPnLViaHTTP(startTime, limit)
}

// getClosedPnLViaHTTP makes direct HTTP call to Bybit API for closed PnL with proper signing
func (t *BybitTrader) getClosedPnLViaHTTP(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// Build query string
	queryParams := fmt.Sprintf("category=linear&startTime=%d&limit=%d", startTime.UnixMilli(), limit)
	url := "https://api.bybit.com/v5/position/closed-pnl?" + queryParams

	// Generate timestamp
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	recvWindow := "5000"

	// Build signature payload: timestamp + api_key + recv_window + queryString
	signPayload := timestamp + t.apiKey + recvWindow + queryParams

	// Generate HMAC-SHA256 signature
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(signPayload))
	signature := hex.EncodeToString(h.Sum(nil))

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Bybit V5 API headers
	req.Header.Set("X-BAPI-API-KEY", t.apiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-SIGN-TYPE", "2")
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	req.Header.Set("Content-Type", "application/json")

	// Use http.DefaultClient for the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Bybit API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		RetCode int                    `json:"retCode"`
		RetMsg  string                 `json:"retMsg"`
		Result  map[string]interface{} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	return t.parseClosedPnLResult(result.Result)
}

// parseClosedPnLResult parses the closed PnL result from Bybit API
func (t *BybitTrader) parseClosedPnLResult(resultData interface{}) ([]types.ClosedPnLRecord, error) {
	data, ok := resultData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format")
	}

	list, _ := data["list"].([]interface{})
	var records []types.ClosedPnLRecord

	for _, item := range list {
		pnl, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse fields
		symbol, _ := pnl["symbol"].(string)
		side, _ := pnl["side"].(string)
		orderId, _ := pnl["orderId"].(string)

		avgEntryPriceStr, _ := pnl["avgEntryPrice"].(string)
		avgExitPriceStr, _ := pnl["avgExitPrice"].(string)
		qtyStr, _ := pnl["qty"].(string)
		closedPnLStr, _ := pnl["closedPnl"].(string)
		cumEntryValueStr, _ := pnl["cumEntryValue"].(string)
		cumExitValueStr, _ := pnl["cumExitValue"].(string)
		leverageStr, _ := pnl["leverage"].(string)
		createdTimeStr, _ := pnl["createdTime"].(string)
		updatedTimeStr, _ := pnl["updatedTime"].(string)

		avgEntryPrice, _ := strconv.ParseFloat(avgEntryPriceStr, 64)
		avgExitPrice, _ := strconv.ParseFloat(avgExitPriceStr, 64)
		qty, _ := strconv.ParseFloat(qtyStr, 64)
		closedPnL, _ := strconv.ParseFloat(closedPnLStr, 64)
		leverage, _ := strconv.ParseInt(leverageStr, 10, 64)
		createdTime, _ := strconv.ParseInt(createdTimeStr, 10, 64)
		updatedTime, _ := strconv.ParseInt(updatedTimeStr, 10, 64)

		// Calculate approximate fee from value difference
		cumEntryValue, _ := strconv.ParseFloat(cumEntryValueStr, 64)
		cumExitValue, _ := strconv.ParseFloat(cumExitValueStr, 64)
		expectedPnL := cumExitValue - cumEntryValue
		if side == "Sell" {
			expectedPnL = cumEntryValue - cumExitValue
		}
		fee := expectedPnL - closedPnL
		if fee < 0 {
			fee = 0
		}

		// Normalize side
		normalizedSide := "long"
		if side == "Sell" {
			normalizedSide = "short"
		}

		record := types.ClosedPnLRecord{
			Symbol:      symbol,
			Side:        normalizedSide,
			EntryPrice:  avgEntryPrice,
			ExitPrice:   avgExitPrice,
			Quantity:    qty,
			RealizedPnL: closedPnL,
			Fee:         fee,
			Leverage:    int(leverage),
			EntryTime:   time.UnixMilli(createdTime).UTC(),
			ExitTime:    time.UnixMilli(updatedTime).UTC(),
			OrderID:     orderId,
			CloseType:   "unknown", // Bybit doesn't provide close type directly
			ExchangeID:  orderId,   // Use orderId as exchange ID
		}

		records = append(records, record)
	}

	return records, nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *BybitTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	var result []types.OpenOrder

	// Get conditional orders (stop-loss, take-profit)
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"orderFilter": "StopOrder",
	}

	resp, err := t.client.NewUtaBybitServiceWithParams(params).GetOpenOrders(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	if resp.RetCode == 0 {
		resultData, ok := resp.Result.(map[string]interface{})
		if ok {
			list, _ := resultData["list"].([]interface{})
			for _, item := range list {
				order, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				orderId, _ := order["orderId"].(string)
				sym, _ := order["symbol"].(string)
				side, _ := order["side"].(string)
				orderType, _ := order["orderType"].(string)
				stopOrderType, _ := order["stopOrderType"].(string)
				triggerPrice, _ := order["triggerPrice"].(string)
				qty, _ := order["qty"].(string)

				price, _ := strconv.ParseFloat(triggerPrice, 64)
				quantity, _ := strconv.ParseFloat(qty, 64)

				// Determine type based on stopOrderType
				displayType := orderType
				if stopOrderType != "" {
					displayType = stopOrderType
				}

				result = append(result, types.OpenOrder{
					OrderID:      orderId,
					Symbol:       sym,
					Side:         side,
					PositionSide: "", // Bybit doesn't use positionSide for UTA
					Type:         displayType,
					Price:        0,
					StopPrice:    price,
					Quantity:     quantity,
					Status:       "NEW",
				})
			}
		}
	}

	return result, nil
}

// PlaceLimitOrder places a limit order for grid trading
// Implements GridTrader interface
func (t *BybitTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	// Format quantity
	qtyStr, err := t.FormatQuantity(req.Symbol, req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to format quantity: %w", err)
	}

	// Format price
	priceStr := fmt.Sprintf("%.8f", req.Price)

	// Set leverage if specified
	if req.Leverage > 0 {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			logger.Warnf("[Bybit] Failed to set leverage: %v", err)
		}
	}

	// Determine side
	side := "Buy"
	if req.Side == "SELL" {
		side = "Sell"
	}

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      req.Symbol,
		"side":        side,
		"orderType":   "Limit",
		"qty":         qtyStr,
		"price":       priceStr,
		"timeInForce": "GTC", // Good Till Cancel
		"positionIdx": 0,     // One-way position mode
	}

	// Add reduce only if specified
	if req.ReduceOnly {
		params["reduceOnly"] = true
	}

	logger.Infof("[Bybit] PlaceLimitOrder: %s %s @ %s, qty=%s", req.Symbol, side, priceStr, qtyStr)

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", err)
	}

	// Parse result
	orderID := ""
	if result.RetCode == 0 {
		if resultData, ok := result.Result.(map[string]interface{}); ok {
			if id, ok := resultData["orderId"].(string); ok {
				orderID = id
			}
		}
	} else {
		return nil, fmt.Errorf("Bybit order failed: %s", result.RetMsg)
	}

	logger.Infof("✓ [Bybit] Limit order placed: %s %s @ %s, qty=%s, orderID=%s",
		req.Symbol, side, priceStr, qtyStr, orderID)

	return &types.LimitOrderResult{
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

// CancelOrder cancels a specific order by ID
// Implements GridTrader interface
func (t *BybitTrader) CancelOrder(symbol, orderID string) error {
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
		"orderId":  orderID,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).CancelOrder(context.Background())
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	if result.RetCode != 0 {
		return fmt.Errorf("Bybit cancel order failed: %s", result.RetMsg)
	}

	logger.Infof("✓ [Bybit] Order cancelled: %s %s", symbol, orderID)
	return nil
}

// GetOrderBook gets the order book for a symbol
// Implements GridTrader interface
func (t *BybitTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	if depth <= 0 {
		depth = 25
	}

	// Use HTTP request directly since the SDK doesn't expose GetOrderbook
	url := fmt.Sprintf("https://api.bybit.com/v5/market/orderbook?category=linear&symbol=%s&limit=%d", symbol, depth)
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			S string     `json:"s"` // symbol
			B [][]string `json:"b"` // bids [[price, size], ...]
			A [][]string `json:"a"` // asks [[price, size], ...]
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	if result.RetCode != 0 {
		return nil, nil, fmt.Errorf("Bybit get orderbook failed: %s", result.RetMsg)
	}

	// Parse bids
	for _, b := range result.Result.B {
		if len(b) >= 2 {
			price, _ := strconv.ParseFloat(b[0], 64)
			qty, _ := strconv.ParseFloat(b[1], 64)
			bids = append(bids, []float64{price, qty})
		}
	}

	// Parse asks
	for _, a := range result.Result.A {
		if len(a) >= 2 {
			price, _ := strconv.ParseFloat(a[0], 64)
			qty, _ := strconv.ParseFloat(a[1], 64)
			asks = append(asks, []float64{price, qty})
		}
	}

	return bids, asks, nil
}
