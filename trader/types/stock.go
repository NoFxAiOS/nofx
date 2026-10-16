package types

// SpotStockTrader is the executor contract of the us_stock strategy (Binance
// spot bStock pairs). It embeds Trader so the shared execution plumbing keeps
// working; short/leverage/margin-mode methods return explicit errors. Spot has
// no position object: positions are the bStock base-asset balances, and one
// protective order set per symbol (OCO = take-profit limit + stop-loss limit,
// or a lone STOP_LOSS_LIMIT) holds the stop on the exchange.
// Design: docs/architecture/US_STOCK_BSTOCK_DESIGN_2026-10-09.md §5.
type SpotStockTrader interface {
	Trader

	// Preflight verifies the API key can trade spot bStock pairs, using the
	// exchange's test-order endpoint (validates, never executes).
	Preflight(symbol string) error

	// BuyMarketNotional market-buys `quoteUSDT` worth of symbol.
	BuyMarketNotional(symbol string, quoteUSDT float64) (*SpotOrderResult, error)
	// BuyLimit places a GTC limit buy.
	BuyLimit(symbol string, quantity, price float64) (*SpotOrderResult, error)
	// SellMarket market-sells quantity (0 = the whole free+locked balance,
	// cancelling the symbol's protective orders first).
	SellMarket(symbol string, quantity float64) (*SpotOrderResult, error)

	// SetProtection replaces the symbol's protective orders for `quantity`:
	// takeProfit > 0 → OCO (limit sell at takeProfit + stop-limit sell
	// triggered at stopPrice, limit at stopLimitPrice); takeProfit == 0 →
	// a lone STOP_LOSS_LIMIT. Existing protective orders are cancelled first.
	SetProtection(symbol string, quantity, stopPrice, stopLimitPrice, takeProfit float64) (*ProtectionOrders, error)
	// GetProtection returns the live protective orders (nil when none).
	GetProtection(symbol string) (*ProtectionOrders, error)
	// CancelProtection cancels the symbol's protective orders.
	CancelProtection(symbol string) error

	// GetOrderStatus queries an individual leg even after it disappears from
	// openOrders (including an OCO leg). Executions and quote proceeds are
	// cumulative; terminal CANCELED/EXPIRED orders can have partial fills.
	GetOrderStatus(symbol, orderID string) (map[string]interface{}, error)

	// CancelOrder cancels one PROGRAM-owned open order (nxbs_ client id) by
	// exchange order id; it refuses to touch orders the user placed manually.
	// Used to expire resting limit entries.
	CancelOrder(symbol, orderID string) error

	// CostBasis returns the FIFO average cost and quantity of the current
	// holding, computed from the account's trade history.
	CostBasis(symbol string) (avgPrice, quantity float64, err error)
}

// SpotOrderResult is a placed spot order.
type SpotOrderResult struct {
	Symbol          string  `json:"symbol"`
	OrderID         string  `json:"order_id"`
	ClientOrderID   string  `json:"client_order_id"`
	Side            string  `json:"side"` // BUY / SELL
	Type            string  `json:"type"` // MARKET / LIMIT
	Status          string  `json:"status"`
	Price           float64 `json:"price"`
	OrigQty         float64 `json:"orig_qty"`
	ExecutedQty     float64 `json:"executed_qty"`
	AvgPrice        float64 `json:"avg_price"` // cummulativeQuoteQty / executedQty when filled
	QuoteQty        float64 `json:"quote_qty"` // cummulativeQuoteQty
	Commission      float64 `json:"commission"`
	CommissionAsset string  `json:"commission_asset"`
}

// ProtectionOrders is the live protective set of one symbol.
type ProtectionOrders struct {
	Symbol         string  `json:"symbol"`
	OrderListID    string  `json:"order_list_id,omitempty"` // OCO list id; empty for a lone stop
	StopOrderID    string  `json:"stop_order_id"`
	TakeProfitID   string  `json:"take_profit_id,omitempty"`
	ListClientID   string  `json:"list_client_id,omitempty"`
	StopClientID   string  `json:"stop_client_id,omitempty"`
	TPClientID     string  `json:"tp_client_id,omitempty"`
	Quantity       float64 `json:"quantity"`
	StopPrice      float64 `json:"stop_price"`
	StopLimitPrice float64 `json:"stop_limit_price"`
	TakeProfit     float64 `json:"take_profit"`
}
