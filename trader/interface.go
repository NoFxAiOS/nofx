package trader

import "time"

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
