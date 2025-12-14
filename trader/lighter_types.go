package trader

import "fmt"

// AccountBalance Account balance information (Lighter)
type AccountBalance struct {
	TotalEquity       float64 `json:"total_equity"`       // Total equity
	AvailableBalance  float64 `json:"available_balance"`  // Available balance
	MarginUsed        float64 `json:"margin_used"`        // Used margin
	UnrealizedPnL     float64 `json:"unrealized_pnl"`     // Unrealized PnL
	MaintenanceMargin float64 `json:"maintenance_margin"` // Maintenance margin
}

// Position Position information (Lighter)
type Position struct {
	Symbol           string  `json:"symbol"`            // Trading pair
	Side             string  `json:"side"`              // "long" or "short"
	Size             float64 `json:"size"`              // Position size
	EntryPrice       float64 `json:"entry_price"`       // Average entry price
	MarkPrice        float64 `json:"mark_price"`        // Mark price
	LiquidationPrice float64 `json:"liquidation_price"` // Liquidation price
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized PnL
	Leverage         float64 `json:"leverage"`          // Leverage multiplier
	MarginUsed       float64 `json:"margin_used"`       // Used margin
}

// CreateOrderRequest Create order request (Lighter)
type CreateOrderRequest struct {
	Symbol      string  `json:"symbol"`        // Trading pair
	Side        string  `json:"side"`          // "buy" or "sell"
	OrderType   string  `json:"order_type"`    // "market" or "limit"
	Quantity    float64 `json:"quantity"`      // Quantity
	Price       float64 `json:"price"`         // Price (required for limit orders)
	ReduceOnly  bool    `json:"reduce_only"`   // Reduce-only flag
	TimeInForce string  `json:"time_in_force"` // "GTC", "IOC", "FOK"
	PostOnly    bool    `json:"post_only"`     // Post-only (maker only)
}

// OrderResponse Order response (Lighter)
type OrderResponse struct {
	OrderID      string  `json:"order_id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	OrderType    string  `json:"order_type"`
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price"`
	Status       string  `json:"status"` // "open", "filled", "cancelled"
	FilledQty    float64 `json:"filled_qty"`
	RemainingQty float64 `json:"remaining_qty"`
	CreateTime   int64   `json:"create_time"`
}

// LighterTradeResponse represents the response from Lighter trades API
type LighterTradeResponse struct {
	Trades []LighterTrade `json:"trades"`
}

// LighterTrade represents a single trade from Lighter
type LighterTrade struct {
	TradeID      string `json:"trade_id"`
	AccountIndex int64  `json:"account_index"`
	MarketIndex  int    `json:"market_index"`
	Symbol       string `json:"symbol"`
	Side         string `json:"side"` // "buy" or "sell"
	Price        string `json:"price"`
	Size         string `json:"size"`
	RealizedPnl  string `json:"realized_pnl"`
	Fee          string `json:"fee"`
	Timestamp    int64  `json:"timestamp"`
	IsMaker      bool   `json:"is_maker"`
}

// parseFloat parses a string to float64, returns 0 for empty string
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
