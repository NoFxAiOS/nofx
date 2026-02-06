package gate

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"nofx/logger"
	"nofx/trader/types"
)

// GateTrader implements types.Trader interface for Gate.io Futures
type GateTrader struct {
	apiKey    string
	secretKey string
	client    *gateapi.APIClient
	ctx       context.Context

	// Cache fields
	cachedBalance       map[string]interface{}
	balanceCacheTime    time.Time
	balanceCacheMutex   sync.RWMutex
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex
	contractsCache      map[string]*gateapi.Contract
	contractsCacheMutex sync.RWMutex
	cacheDuration       time.Duration
}

// NewGateTrader creates a new Gate trader instance
func NewGateTrader(apiKey, secretKey string) *GateTrader {
	config := gateapi.NewConfiguration()
	config.AddDefaultHeader("X-Gate-Channel-Id", "nofx")
	client := gateapi.NewAPIClient(config)

	ctx := context.WithValue(context.Background(),
		gateapi.ContextGateAPIV4,
		gateapi.GateAPIV4{
			Key:    apiKey,
			Secret: secretKey,
		},
	)

	return &GateTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		client:         client,
		ctx:            ctx,
		contractsCache: make(map[string]*gateapi.Contract),
		cacheDuration:  15 * time.Second,
	}
}

// GetBalance retrieves account balance
func (t *GateTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cached := t.cachedBalance
		t.balanceCacheMutex.RUnlock()
		return cached, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Fetch from API
	accounts, _, err := t.client.FuturesApi.ListFuturesAccounts(t.ctx, "usdt")
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	total, _ := strconv.ParseFloat(accounts.Total, 64)
	available, _ := strconv.ParseFloat(accounts.Available, 64)
	unrealizedPnl, _ := strconv.ParseFloat(accounts.UnrealisedPnl, 64)

	result := map[string]interface{}{
		"totalWalletBalance":    total,
		"availableBalance":      available,
		"totalUnrealizedProfit": unrealizedPnl,
	}

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions retrieves all open positions
func (t *GateTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cached := t.cachedPositions
		t.positionsCacheMutex.RUnlock()
		return cached, nil
	}
	t.positionsCacheMutex.RUnlock()

	// Fetch from API
	positions, _, err := t.client.FuturesApi.ListPositions(t.ctx, "usdt", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		if pos.Size == 0 {
			continue // Skip empty positions
		}

		entryPrice, _ := strconv.ParseFloat(pos.EntryPrice, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiqPrice, 64)
		unrealizedPnl, _ := strconv.ParseFloat(pos.UnrealisedPnl, 64)
		leverage, _ := strconv.ParseFloat(pos.Leverage, 64)

		// Gate returns position size in contracts, need to convert to base currency
		// Each contract = quanto_multiplier base currency
		contractSize := float64(pos.Size)
		if pos.Size < 0 {
			contractSize = float64(-pos.Size)
		}

		// Get quanto_multiplier from contract info to convert contracts to actual quantity
		quantoMultiplier := 1.0
		contract, err := t.getContract(pos.Contract)
		if err == nil && contract != nil {
			qm, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
			if qm > 0 {
				quantoMultiplier = qm
			}
		}

		// Convert contract count to actual token quantity
		positionAmt := contractSize * quantoMultiplier

		// Determine side based on position size
		side := "long"
		if pos.Size < 0 {
			side = "short"
		}

		result = append(result, map[string]interface{}{
			"symbol":           pos.Contract,
			"positionAmt":      positionAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealizedPnl,
			"leverage":         int(leverage),
			"liquidationPrice": liqPrice,
			"side":             side,
		})
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// convertSymbol converts symbol format (e.g., BTCUSDT -> BTC_USDT)
func (t *GateTrader) convertSymbol(symbol string) string {
	// If already in correct format
	if strings.Contains(symbol, "_") {
		return symbol
	}
	// Convert BTCUSDT to BTC_USDT
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return base + "_USDT"
	}
	return symbol
}

// revertSymbol converts symbol back to standard format (e.g., BTC_USDT -> BTCUSDT)
func (t *GateTrader) revertSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "_", "")
}

// getContract fetches contract info with caching
func (t *GateTrader) getContract(symbol string) (*gateapi.Contract, error) {
	symbol = t.convertSymbol(symbol)

	// Check cache
	t.contractsCacheMutex.RLock()
	if contract, ok := t.contractsCache[symbol]; ok {
		t.contractsCacheMutex.RUnlock()
		return contract, nil
	}
	t.contractsCacheMutex.RUnlock()

	// Fetch from API
	contract, _, err := t.client.FuturesApi.GetFuturesContract(t.ctx, "usdt", symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract info: %w", err)
	}

	// Update cache
	t.contractsCacheMutex.Lock()
	t.contractsCache[symbol] = &contract
	t.contractsCacheMutex.Unlock()

	return &contract, nil
}

// SetLeverage sets the leverage for a symbol
func (t *GateTrader) SetLeverage(symbol string, leverage int) error {
	symbol = t.convertSymbol(symbol)

	_, _, err := t.client.FuturesApi.UpdatePositionLeverage(t.ctx, "usdt", symbol, fmt.Sprintf("%d", leverage), nil)
	if err != nil {
		// Gate.io may return error if leverage is already set
		if strings.Contains(err.Error(), "RISK_LIMIT_EXCEEDED") {
			logger.Warnf("  [Gate] Leverage %d exceeds limit for %s", leverage, symbol)
			return nil
		}
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	logger.Infof("  [Gate] Leverage set to %dx for %s", leverage, symbol)
	return nil
}

// SetMarginMode sets margin mode (cross or isolated)
func (t *GateTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Gate.io uses leverage=0 for cross margin, positive number for isolated
	// This is handled through UpdatePositionLeverage with cross_leverage_limit
	// For now, we'll skip explicit margin mode setting as it's tied to leverage
	logger.Infof("  [Gate] Margin mode is set through leverage (0=cross)")
	return nil
}

// OpenLong opens a long position
func (t *GateTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// Cancel old orders first
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Warnf("  [Gate] Failed to set leverage: %v", err)
	}

	// Get contract info for size calculation
	contract, err := t.getContract(symbol)
	if err != nil {
		return nil, err
	}

	// Gate uses contract size units (each contract = quanto_multiplier base currency)
	// size = quantity / quanto_multiplier
	quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
	size := int64(quantity / quantoMultiplier)
	if size <= 0 {
		size = 1
	}

	order := gateapi.FuturesOrder{
		Contract: symbol,
		Size:     size, // Positive for long
		Price:    "0",  // Market order
		Tif:      "ioc",
		Text:     "t-nofx",
	}

	logger.Infof("  [Gate] OpenLong: symbol=%s, size=%d, leverage=%d", symbol, size, leverage)

	result, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	// Clear cache
	t.clearCache()

	// Parse fill price from result
	fillPrice, _ := strconv.ParseFloat(result.FillPrice, 64)

	logger.Infof("  [Gate] Opened long position: orderId=%d, fillPrice=%.4f", result.Id, fillPrice)

	return map[string]interface{}{
		"orderId":   fmt.Sprintf("%d", result.Id),
		"symbol":    t.revertSymbol(symbol),
		"status":    "FILLED",
		"fillPrice": fillPrice,
		"avgPrice":  fillPrice,
	}, nil
}

// OpenShort opens a short position
func (t *GateTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// Cancel old orders first
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Warnf("  [Gate] Failed to set leverage: %v", err)
	}

	// Get contract info for size calculation
	contract, err := t.getContract(symbol)
	if err != nil {
		return nil, err
	}

	// Gate uses contract size units
	quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
	size := int64(quantity / quantoMultiplier)
	if size <= 0 {
		size = 1
	}

	order := gateapi.FuturesOrder{
		Contract: symbol,
		Size:     -size, // Negative for short
		Price:    "0",   // Market order
		Tif:      "ioc",
		Text:     "t-nofx",
	}

	logger.Infof("  [Gate] OpenShort: symbol=%s, size=%d, leverage=%d", symbol, -size, leverage)

	result, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	// Clear cache
	t.clearCache()

	// Parse fill price from result
	fillPrice, _ := strconv.ParseFloat(result.FillPrice, 64)

	logger.Infof("  [Gate] Opened short position: orderId=%d, fillPrice=%.4f", result.Id, fillPrice)

	return map[string]interface{}{
		"orderId":   fmt.Sprintf("%d", result.Id),
		"symbol":    t.revertSymbol(symbol),
		"status":    "FILLED",
		"fillPrice": fillPrice,
		"avgPrice":  fillPrice,
	}, nil
}

// CloseLong closes a long position
func (t *GateTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// If quantity is 0, get current position
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			posSymbol := t.convertSymbol(pos["symbol"].(string))
			if posSymbol == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}
		if quantity == 0 {
			return nil, fmt.Errorf("long position not found for %s", symbol)
		}
	}

	// Get contract info for size calculation
	contract, err := t.getContract(symbol)
	if err != nil {
		return nil, err
	}

	quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
	size := int64(quantity / quantoMultiplier)
	if size <= 0 {
		size = 1
	}

	// Close long = sell (use ReduceOnly, not Close which requires Size=0)
	order := gateapi.FuturesOrder{
		Contract:   symbol,
		Size:       -size, // Negative to close long
		Price:      "0",
		Tif:        "ioc",
		ReduceOnly: true,
		Text:       "t-nofx-close",
	}

	logger.Infof("  [Gate] CloseLong: symbol=%s, size=%d", symbol, -size)

	result, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	// Clear cache
	t.clearCache()

	// Parse fill price from result
	fillPrice, _ := strconv.ParseFloat(result.FillPrice, 64)

	logger.Infof("  [Gate] Closed long position: orderId=%d, fillPrice=%.4f", result.Id, fillPrice)

	return map[string]interface{}{
		"orderId":   fmt.Sprintf("%d", result.Id),
		"symbol":    t.revertSymbol(symbol),
		"status":    "FILLED",
		"fillPrice": fillPrice,
		"avgPrice":  fillPrice,
	}, nil
}

// CloseShort closes a short position
func (t *GateTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	// If quantity is 0, get current position
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			posSymbol := t.convertSymbol(pos["symbol"].(string))
			if posSymbol == symbol && pos["side"] == "short" {
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

	// Get contract info for size calculation
	contract, err := t.getContract(symbol)
	if err != nil {
		return nil, err
	}

	quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
	size := int64(quantity / quantoMultiplier)
	if size <= 0 {
		size = 1
	}

	// Close short = buy (use ReduceOnly, not Close which requires Size=0)
	order := gateapi.FuturesOrder{
		Contract:   symbol,
		Size:       size, // Positive to close short
		Price:      "0",
		Tif:        "ioc",
		ReduceOnly: true,
		Text:       "t-nofx-close",
	}

	logger.Infof("  [Gate] CloseShort: symbol=%s, size=%d", symbol, size)

	result, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	// Clear cache
	t.clearCache()

	// Parse fill price from result
	fillPrice, _ := strconv.ParseFloat(result.FillPrice, 64)

	logger.Infof("  [Gate] Closed short position: orderId=%d, fillPrice=%.4f", result.Id, fillPrice)

	return map[string]interface{}{
		"orderId":   fmt.Sprintf("%d", result.Id),
		"symbol":    t.revertSymbol(symbol),
		"status":    "FILLED",
		"fillPrice": fillPrice,
		"avgPrice":  fillPrice,
	}, nil
}

// GetMarketPrice gets the current market price
func (t *GateTrader) GetMarketPrice(symbol string) (float64, error) {
	symbol = t.convertSymbol(symbol)

	opts := &gateapi.ListFuturesTickersOpts{
		Contract: optional.NewString(symbol),
	}

	tickers, _, err := t.client.FuturesApi.ListFuturesTickers(t.ctx, "usdt", opts)
	if err != nil {
		return 0, fmt.Errorf("failed to get market price: %w", err)
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("no ticker data for %s", symbol)
	}

	price, _ := strconv.ParseFloat(tickers[0].Last, 64)
	return price, nil
}

// SetStopLoss sets a stop loss order
func (t *GateTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	symbol = t.convertSymbol(symbol)

	contract, err := t.getContract(symbol)
	if err != nil {
		return err
	}

	quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
	size := int64(quantity / quantoMultiplier)
	if size <= 0 {
		size = 1
	}

	// For long position, stop loss means sell when price drops
	// For short position, stop loss means buy when price rises
	if strings.ToUpper(positionSide) == "LONG" {
		size = -size
	}

	// Use price trigger order
	trigger := gateapi.FuturesPriceTriggeredOrder{
		Initial: gateapi.FuturesInitialOrder{
			Contract:   symbol,
			Size:       size,
			Price:      "0", // Market order
			Tif:        "ioc",
			ReduceOnly: true,
			Close:      true,
		},
		Trigger: gateapi.FuturesPriceTrigger{
			StrategyType: 0, // Close position
			PriceType:    0, // Latest price
			Price:        fmt.Sprintf("%.8f", stopPrice),
			Rule:         1, // Price <= trigger price
		},
	}

	if strings.ToUpper(positionSide) == "SHORT" {
		trigger.Trigger.Rule = 2 // Price >= trigger price for short stop loss
	}

	_, _, err = t.client.FuturesApi.CreatePriceTriggeredOrder(t.ctx, "usdt", trigger)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("  [Gate] Stop loss set: %s @ %.4f", symbol, stopPrice)
	return nil
}

// SetTakeProfit sets a take profit order
func (t *GateTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	symbol = t.convertSymbol(symbol)

	contract, err := t.getContract(symbol)
	if err != nil {
		return err
	}

	quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
	size := int64(quantity / quantoMultiplier)
	if size <= 0 {
		size = 1
	}

	// For long position, take profit means sell when price rises
	// For short position, take profit means buy when price drops
	if strings.ToUpper(positionSide) == "LONG" {
		size = -size
	}

	trigger := gateapi.FuturesPriceTriggeredOrder{
		Initial: gateapi.FuturesInitialOrder{
			Contract:   symbol,
			Size:       size,
			Price:      "0", // Market order
			Tif:        "ioc",
			ReduceOnly: true,
			Close:      true,
		},
		Trigger: gateapi.FuturesPriceTrigger{
			StrategyType: 0, // Close position
			PriceType:    0, // Latest price
			Price:        fmt.Sprintf("%.8f", takeProfitPrice),
			Rule:         2, // Price >= trigger price for long take profit
		},
	}

	if strings.ToUpper(positionSide) == "SHORT" {
		trigger.Trigger.Rule = 1 // Price <= trigger price for short take profit
	}

	_, _, err = t.client.FuturesApi.CreatePriceTriggeredOrder(t.ctx, "usdt", trigger)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	logger.Infof("  [Gate] Take profit set: %s @ %.4f", symbol, takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop loss orders
func (t *GateTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelTriggerOrders(symbol, "stop_loss")
}

// CancelTakeProfitOrders cancels take profit orders
func (t *GateTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelTriggerOrders(symbol, "take_profit")
}

// cancelTriggerOrders cancels trigger orders of a specific type
func (t *GateTrader) cancelTriggerOrders(symbol string, orderType string) error {
	symbol = t.convertSymbol(symbol)

	opts := &gateapi.ListPriceTriggeredOrdersOpts{
		Contract: optional.NewString(symbol),
	}

	orders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", opts)
	if err != nil {
		return err
	}

	for _, order := range orders {
		// Determine if it's stop loss or take profit based on trigger rule and position
		// For simplicity, cancel all matching symbol orders
		_, _, err := t.client.FuturesApi.CancelPriceTriggeredOrder(t.ctx, "usdt", fmt.Sprintf("%d", order.Id))
		if err != nil {
			logger.Warnf("  [Gate] Failed to cancel trigger order %d: %v", order.Id, err)
		}
	}

	return nil
}

// CancelAllOrders cancels all pending orders for a symbol
func (t *GateTrader) CancelAllOrders(symbol string) error {
	symbol = t.convertSymbol(symbol)

	// Cancel regular orders
	_, _, err := t.client.FuturesApi.CancelFuturesOrders(t.ctx, "usdt", symbol, nil)
	if err != nil {
		// Ignore if no orders to cancel
		if !strings.Contains(err.Error(), "ORDER_NOT_FOUND") {
			logger.Warnf("  [Gate] Error canceling orders: %v", err)
		}
	}

	// Cancel trigger orders
	t.cancelTriggerOrders(symbol, "")

	return nil
}

// CancelStopOrders cancels all stop orders (stop loss and take profit)
func (t *GateTrader) CancelStopOrders(symbol string) error {
	t.CancelStopLossOrders(symbol)
	t.CancelTakeProfitOrders(symbol)
	return nil
}

// FormatQuantity formats quantity to correct precision
func (t *GateTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		return fmt.Sprintf("%.4f", quantity), nil
	}

	// Gate uses quanto_multiplier for contract size
	quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
	if quantoMultiplier > 0 {
		// Calculate number of contracts
		numContracts := quantity / quantoMultiplier
		return fmt.Sprintf("%.0f", math.Floor(numContracts)), nil
	}

	return fmt.Sprintf("%.4f", quantity), nil
}

// GetOrderStatus gets the status of an order
func (t *GateTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	symbol = t.convertSymbol(symbol)

	order, _, err := t.client.FuturesApi.GetFuturesOrder(t.ctx, "usdt", orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	fillPrice, _ := strconv.ParseFloat(order.FillPrice, 64)
	tkFee, _ := strconv.ParseFloat(order.Tkfr, 64)
	mkFee, _ := strconv.ParseFloat(order.Mkfr, 64)
	totalFee := tkFee + mkFee

	// Get quanto_multiplier to convert contracts to actual quantity
	quantoMultiplier := 1.0
	contract, contractErr := t.getContract(symbol)
	if contractErr == nil && contract != nil {
		qm, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
		if qm > 0 {
			quantoMultiplier = qm
		}
	}

	// Map status
	status := "NEW"
	switch order.Status {
	case "finished":
		if order.FinishAs == "filled" {
			status = "FILLED"
		} else if order.FinishAs == "cancelled" {
			status = "CANCELED"
		} else {
			status = "CLOSED"
		}
	case "open":
		status = "NEW"
	}

	side := "BUY"
	if order.Size < 0 {
		side = "SELL"
	}

	// Convert contract count to actual token quantity
	executedQty := math.Abs(float64(order.Size-order.Left)) * quantoMultiplier

	return map[string]interface{}{
		"orderId":     orderID,
		"symbol":      t.revertSymbol(symbol),
		"status":      status,
		"avgPrice":    fillPrice,
		"executedQty": executedQty,
		"side":        side,
		"type":        order.Tif,
		"time":        int64(order.CreateTime * 1000),
		"updateTime":  int64(order.FinishTime * 1000),
		"commission":  totalFee,
	}, nil
}

// GetClosedPnL retrieves closed position PnL records
func (t *GateTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	opts := &gateapi.ListPositionCloseOpts{
		Limit: optional.NewInt32(int32(limit)),
		From:  optional.NewInt64(startTime.Unix()),
	}

	closedPositions, _, err := t.client.FuturesApi.ListPositionClose(t.ctx, "usdt", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get closed positions: %w", err)
	}

	records := make([]types.ClosedPnLRecord, 0, len(closedPositions))
	for _, pos := range closedPositions {
		pnl, _ := strconv.ParseFloat(pos.Pnl, 64)

		record := types.ClosedPnLRecord{
			Symbol:      t.revertSymbol(pos.Contract),
			Side:        pos.Side,
			RealizedPnL: pnl,
			ExitTime:    time.Unix(int64(pos.Time), 0).UTC(),
			CloseType:   "unknown",
		}

		records = append(records, record)
	}

	return records, nil
}

// GetOpenOrders gets open/pending orders
func (t *GateTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	symbol = t.convertSymbol(symbol)

	opts := &gateapi.ListFuturesOrdersOpts{
		Contract: optional.NewString(symbol),
	}

	orders, _, err := t.client.FuturesApi.ListFuturesOrders(t.ctx, "usdt", "open", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	// Get quanto_multiplier to convert contracts to actual quantity
	quantoMultiplier := 1.0
	contract, err := t.getContract(symbol)
	if err == nil && contract != nil {
		qm, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
		if qm > 0 {
			quantoMultiplier = qm
		}
	}

	var result []types.OpenOrder
	for _, order := range orders {
		price, _ := strconv.ParseFloat(order.Price, 64)

		side := "BUY"
		if order.Size < 0 {
			side = "SELL"
		}

		// Convert contract count to actual token quantity
		quantity := math.Abs(float64(order.Size)) * quantoMultiplier

		result = append(result, types.OpenOrder{
			OrderID:  fmt.Sprintf("%d", order.Id),
			Symbol:   t.revertSymbol(order.Contract),
			Side:     side,
			Type:     "LIMIT",
			Price:    price,
			Quantity: quantity,
			Status:   "NEW",
		})
	}

	// Also get trigger orders
	triggerOpts := &gateapi.ListPriceTriggeredOrdersOpts{
		Contract: optional.NewString(symbol),
	}

	triggerOrders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", triggerOpts)
	if err == nil {
		for _, order := range triggerOrders {
			triggerPrice, _ := strconv.ParseFloat(order.Trigger.Price, 64)

			side := "BUY"
			if order.Initial.Size < 0 {
				side = "SELL"
			}

			orderType := "STOP_MARKET"
			if order.Trigger.Rule == 2 {
				orderType = "TAKE_PROFIT_MARKET"
			}

			// Convert contract count to actual token quantity
			quantity := math.Abs(float64(order.Initial.Size)) * quantoMultiplier

			result = append(result, types.OpenOrder{
				OrderID:   fmt.Sprintf("%d", order.Id),
				Symbol:    t.revertSymbol(order.Initial.Contract),
				Side:      side,
				Type:      orderType,
				StopPrice: triggerPrice,
				Quantity:  quantity,
				Status:    "NEW",
			})
		}
	}

	return result, nil
}

// clearCache clears all caches
func (t *GateTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}

// Ensure GateTrader implements Trader interface
var _ types.Trader = (*GateTrader)(nil)
