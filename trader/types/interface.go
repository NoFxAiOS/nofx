package types

import (
	"fmt"
	"nofx/logger"
	"time"
)

// ClosedPnLRecord represents a single closed position record from exchange
type ClosedPnLRecord struct {
	Symbol       string    // Trading pair (e.g., "BTCUSDT")
	Side         string    // "long" or "short"
	EntryPrice   float64   // Entry price
	ExitPrice    float64   // Exit/close price
	Quantity     float64   // Position size
	RealizedPnL  float64   // Realized profit/loss
	Fee          float64   // Trading fee/commission
	Leverage     int       // Leverage used
	EntryTime    time.Time // Position open time
	ExitTime     time.Time // Position close time
	OrderID      string    // Close order ID
	CloseType    string    // "manual", "stop_loss", "take_profit", "liquidation", "unknown"
	ExchangeID   string    // Exchange-specific position ID
}

// TradeRecord represents a single trade/fill from exchange
// Used for reconstructing position history with unified algorithm
type TradeRecord struct {
	TradeID      string    // Unique trade ID from exchange
	Symbol       string    // Trading pair (e.g., "BTCUSDT")
	Side         string    // "BUY" or "SELL"
	PositionSide string    // "LONG", "SHORT", or "BOTH" (for one-way mode)
	OrderAction  string    // "open_long", "open_short", "close_long", "close_short" (from exchange Dir field)
	Price        float64   // Execution price
	Quantity     float64   // Executed quantity
	RealizedPnL  float64   // Realized PnL (non-zero for closing trades)
	Fee          float64   // Trading fee/commission
	Time         time.Time // Trade execution time
}

// Trader Unified trader interface
// Supports multiple trading platforms (Binance, Hyperliquid, etc.)
type Trader interface {
	// GetBalance Get account balance
	GetBalance() (map[string]interface{}, error)

	// GetPositions Get all positions
	GetPositions() ([]map[string]interface{}, error)

	// OpenLong Open long position
	OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// OpenShort Open short position
	OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// CloseLong Close long position (quantity=0 means close all)
	CloseLong(symbol string, quantity float64) (map[string]interface{}, error)

	// CloseShort Close short position (quantity=0 means close all)
	CloseShort(symbol string, quantity float64) (map[string]interface{}, error)

	// SetLeverage Set leverage
	SetLeverage(symbol string, leverage int) error

	// SetMarginMode Set position mode (true=cross margin, false=isolated margin)
	SetMarginMode(symbol string, isCrossMargin bool) error

	// GetMarketPrice Get market price
	GetMarketPrice(symbol string) (float64, error)

	// SetStopLoss Set stop-loss order
	SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error

	// SetTakeProfit Set take-profit order
	SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error

	// CancelStopLossOrders Cancel only stop-loss orders (BUG fix: don't delete take-profit when adjusting stop-loss)
	CancelStopLossOrders(symbol string) error

	// CancelTakeProfitOrders Cancel only take-profit orders (BUG fix: don't delete stop-loss when adjusting take-profit)
	CancelTakeProfitOrders(symbol string) error

	// CancelAllOrders Cancel all pending orders for this symbol
	CancelAllOrders(symbol string) error

	// CancelStopOrders Cancel stop-loss/take-profit orders for this symbol (for adjusting stop-loss/take-profit positions)
	CancelStopOrders(symbol string) error

	// FormatQuantity Format quantity to correct precision
	FormatQuantity(symbol string, quantity float64) (string, error)

	// GetOrderStatus Get order status
	// Returns: status(FILLED/NEW/CANCELED), avgPrice, executedQty, commission
	GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error)

	// GetClosedPnL Get closed position PnL records from exchange
	// startTime: start time for query (usually last sync time)
	// limit: max number of records to return
	// Returns accurate exit price, fees, and close reason for positions closed externally
	GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error)

	// GetOpenOrders Get open/pending orders from exchange
	// Returns stop-loss, take-profit, and limit orders that haven't been filled
	GetOpenOrders(symbol string) ([]OpenOrder, error)
}

// OpenOrder represents a pending order on the exchange
type OpenOrder struct {
	OrderID      string  `json:"order_id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`          // BUY/SELL
	PositionSide string  `json:"position_side"` // LONG/SHORT
	Type         string  `json:"type"`          // LIMIT/STOP_MARKET/TAKE_PROFIT_MARKET
	Price        float64 `json:"price"`         // Order price (for limit orders)
	StopPrice    float64 `json:"stop_price"`    // Trigger price (for stop orders)
	Quantity     float64 `json:"quantity"`
	Status       string  `json:"status"` // NEW
}

// LimitOrderRequest represents a limit order request for grid trading
type LimitOrderRequest struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`          // BUY/SELL
	PositionSide string  `json:"position_side"` // LONG/SHORT (for hedge mode)
	Price        float64 `json:"price"`         // Limit price
	Quantity     float64 `json:"quantity"`
	Leverage     int     `json:"leverage"`
	PostOnly     bool    `json:"post_only"`     // Maker only order
	ReduceOnly   bool    `json:"reduce_only"`   // Reduce position only
	ClientID     string  `json:"client_id"`     // Client order ID for tracking
}

// LimitOrderResult represents the result of placing a limit order
type LimitOrderResult struct {
	OrderID      string  `json:"order_id"`
	ClientID     string  `json:"client_id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	PositionSide string  `json:"position_side"`
	Price        float64 `json:"price"`
	Quantity     float64 `json:"quantity"`
	Status       string  `json:"status"` // NEW, PARTIALLY_FILLED, FILLED, CANCELED
}

// GridTrader extends Trader interface with limit order support for grid trading
// Exchanges that support grid trading should implement this interface
type GridTrader interface {
	Trader

	// PlaceLimitOrder places a limit order at specified price
	// Returns order ID and status
	PlaceLimitOrder(req *LimitOrderRequest) (*LimitOrderResult, error)

	// CancelOrder cancels a specific order by ID
	CancelOrder(symbol, orderID string) error

	// GetOrderBook gets current order book (for price validation)
	// Returns best bid/ask prices
	GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error)
}

// GridTraderAdapter wraps a basic Trader to provide GridTrader interface
// Uses stop orders as a fallback when limit orders aren't directly available
type GridTraderAdapter struct {
	Trader
}

// NewGridTraderAdapter creates an adapter for basic Trader
func NewGridTraderAdapter(t Trader) *GridTraderAdapter {
	return &GridTraderAdapter{Trader: t}
}

// PlaceLimitOrder implements limit order using available methods
// For exchanges without native limit order support, this uses conditional orders
func (a *GridTraderAdapter) PlaceLimitOrder(req *LimitOrderRequest) (*LimitOrderResult, error) {
	// CRITICAL FIX: Set leverage before placing order
	if req.Leverage > 0 {
		if err := a.Trader.SetLeverage(req.Symbol, req.Leverage); err != nil {
			logger.Warnf("[Grid] Failed to set leverage %dx: %v", req.Leverage, err)
			// Continue anyway - some exchanges don't require explicit leverage setting
		}
	}

	// Use SetStopLoss/SetTakeProfit as conditional limit orders
	// For buy orders below current price, use stop-loss mechanism
	// For sell orders above current price, use take-profit mechanism
	var err error
	if req.Side == "BUY" {
		err = a.Trader.SetStopLoss(req.Symbol, "SHORT", req.Quantity, req.Price)
	} else {
		err = a.Trader.SetTakeProfit(req.Symbol, "LONG", req.Quantity, req.Price)
	}
	if err != nil {
		return nil, err
	}
	return &LimitOrderResult{
		OrderID:      req.ClientID,
		ClientID:     req.ClientID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       "NEW",
	}, nil
}

// CancelOrder cancels a specific order
func (a *GridTraderAdapter) CancelOrder(symbol, orderID string) error {
	// Try to use CancelOrder if trader supports it directly
	if canceler, ok := a.Trader.(interface {
		CancelOrder(symbol, orderID string) error
	}); ok {
		return canceler.CancelOrder(symbol, orderID)
	}

	// For traders that only support CancelAllOrders, log a warning
	// This is a limitation - we cannot cancel individual orders
	logger.Warnf("[Grid] Trader does not support individual order cancellation, "+
		"cannot cancel order %s. Consider using exchange-specific GridTrader implementation.", orderID)

	// Return error instead of canceling all orders
	return fmt.Errorf("individual order cancellation not supported for this exchange")
}

// GetOrderBook returns empty order book (not supported in basic Trader)
func (a *GridTraderAdapter) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	// Not supported, return empty
	return nil, nil, nil
}
