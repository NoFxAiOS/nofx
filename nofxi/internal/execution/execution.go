// Package execution implements the Execution Layer.
//
// Phase 2 — stubs for now.
//
// Planned capabilities:
// - Multi-exchange order execution (via NOFX engine)
// - Position management (open/close/adjust)
// - Stop-loss and take-profit management
// - x402 payment integration via Claw402
// - Safe mode and risk controls
package execution

// Executor is the trade execution interface.
type Executor interface {
	// PlaceOrder places a trade order on the exchange.
	PlaceOrder(exchange, symbol, side string, price, quantity float64) (string, error)

	// ClosePosition closes an existing position.
	ClosePosition(exchange, symbol string) error

	// GetPositions returns current open positions.
	GetPositions(exchange string) ([]Position, error)

	// GetBalance returns account balance.
	GetBalance(exchange string) (*Balance, error)
}

// Position represents an open position.
type Position struct {
	Exchange   string  `json:"exchange"`
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Size       float64 `json:"size"`
	EntryPrice float64 `json:"entry_price"`
	MarkPrice  float64 `json:"mark_price"`
	PnL        float64 `json:"pnl"`
	Leverage   float64 `json:"leverage"`
}

// Balance represents account balance.
type Balance struct {
	Exchange   string  `json:"exchange"`
	Total      float64 `json:"total"`
	Available  float64 `json:"available"`
	InPosition float64 `json:"in_position"`
	Currency   string  `json:"currency"`
}
