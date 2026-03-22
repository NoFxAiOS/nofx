package memory

import "time"

// TradeRecord stores a completed or active trade.
type TradeRecord struct {
	ID         int64     `json:"id"`
	Exchange   string    `json:"exchange"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`       // "buy" or "sell"
	Type       string    `json:"type"`       // "market", "limit"
	Price      float64   `json:"price"`
	Quantity   float64   `json:"quantity"`
	PnL        float64   `json:"pnl"`        // Realized P/L
	Fee        float64   `json:"fee"`
	Status     string    `json:"status"`     // "open", "closed", "cancelled"
	AIModel    string    `json:"ai_model"`   // Which AI model made the decision
	AIReason   string    `json:"ai_reason"`  // Why AI made this decision
	StrategyID string    `json:"strategy_id"`
	CreatedAt  time.Time `json:"created_at"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
}

// UserPreference stores user settings and preferences.
type UserPreference struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`    // Telegram user ID
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Conversation stores chat messages for context.
type Conversation struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Role      string    `json:"role"`       // "user", "assistant", "system"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// StrategyPerformance tracks how well a strategy has performed.
type StrategyPerformance struct {
	ID           int64     `json:"id"`
	StrategyID   string    `json:"strategy_id"`
	StrategyName string    `json:"strategy_name"`
	TotalTrades  int       `json:"total_trades"`
	WinRate      float64   `json:"win_rate"`
	TotalPnL     float64   `json:"total_pnl"`
	MaxDrawdown  float64   `json:"max_drawdown"`
	SharpeRatio  float64   `json:"sharpe_ratio"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MarketSnapshot stores a point-in-time market state for pattern recognition.
type MarketSnapshot struct {
	ID        int64     `json:"id"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume24h float64   `json:"volume_24h"`
	Change24h float64   `json:"change_24h"`
	Note      string    `json:"note"`       // AI observation about this snapshot
	CreatedAt time.Time `json:"created_at"`
}
