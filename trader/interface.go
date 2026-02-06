package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/trader/types"
)

// Re-export types for backward compatibility
type (
	ClosedPnLRecord   = types.ClosedPnLRecord
	TradeRecord       = types.TradeRecord
	Trader            = types.Trader
	OpenOrder         = types.OpenOrder
	LimitOrderRequest = types.LimitOrderRequest
	LimitOrderResult  = types.LimitOrderResult
	GridTrader        = types.GridTrader
)

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
