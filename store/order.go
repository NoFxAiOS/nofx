package store

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// TraderOrder trader order record
type TraderOrder struct {
	ID            int64     `json:"id"`
	TraderID      string    `json:"trader_id"`       // Trader ID
	OrderID       string    `json:"order_id"`        // Exchange order ID
	ClientOrderID string    `json:"client_order_id"` // Client order ID
	Symbol        string    `json:"symbol"`          // Trading pair
	Side          string    `json:"side"`            // BUY/SELL
	PositionSide  string    `json:"position_side"`   // LONG/SHORT/BOTH
	Action        string    `json:"action"`          // open_long/close_long/open_short/close_short
	OrderType     string    `json:"order_type"`      // MARKET/LIMIT
	Quantity      float64   `json:"quantity"`        // Order quantity
	Price         float64   `json:"price"`           // Order price
	AvgPrice      float64   `json:"avg_price"`       // Actual average execution price
	ExecutedQty   float64   `json:"executed_qty"`    // Executed quantity
	Leverage      int       `json:"leverage"`        // Leverage multiplier
	Status        string    `json:"status"`          // NEW/FILLED/CANCELED/EXPIRED
	Fee           float64   `json:"fee"`             // Fee
	FeeAsset      string    `json:"fee_asset"`       // Fee asset
	RealizedPnL   float64   `json:"realized_pnl"`    // Realized PnL (when closing)
	EntryPrice    float64   `json:"entry_price"`     // Entry price (recorded when closing)
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	FilledAt      time.Time `json:"filled_at"` // Filled time
}

// TraderStats trading statistics metrics
type TraderStats struct {
	TotalTrades    int     `json:"total_trades"`     // Total trades (closed)
	WinTrades      int     `json:"win_trades"`       // Winning trades
	LossTrades     int     `json:"loss_trades"`      // Losing trades
	WinRate        float64 `json:"win_rate"`         // Win rate (%)
	ProfitFactor   float64 `json:"profit_factor"`    // Profit factor
	SharpeRatio    float64 `json:"sharpe_ratio"`     // Sharpe ratio
	TotalPnL       float64 `json:"total_pnl"`        // Total PnL
	TotalFee       float64 `json:"total_fee"`        // Total fees
	AvgWin         float64 `json:"avg_win"`          // Average win
	AvgLoss        float64 `json:"avg_loss"`         // Average loss
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // Max drawdown (%)
}

// CompletedOrder completed order (for AI input)
type CompletedOrder struct {
	Symbol      string    `json:"symbol"`       // Trading pair
	Action      string    `json:"action"`       // close_long/close_short
	Side        string    `json:"side"`         // long/short
	Quantity    float64   `json:"quantity"`     // Quantity
	EntryPrice  float64   `json:"entry_price"`  // Entry price
	ExitPrice   float64   `json:"exit_price"`   // Exit price
	RealizedPnL float64   `json:"realized_pnl"` // Realized PnL
	PnLPct      float64   `json:"pnl_pct"`      // PnL percentage
	Fee         float64   `json:"fee"`          // Fee
	Leverage    int       `json:"leverage"`     // Leverage
	FilledAt    time.Time `json:"filled_at"`    // Filled time
}

// OrderStore order storage
type OrderStore struct {
	db *sql.DB
}

// NewOrderStore creates order storage instance
func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

// InitTables initializes order tables
func (s *OrderStore) InitTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS trader_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			order_id TEXT NOT NULL,
			client_order_id TEXT DEFAULT '',
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			position_side TEXT DEFAULT '',
			action TEXT NOT NULL,
			order_type TEXT DEFAULT 'MARKET',
			quantity REAL NOT NULL,
			price REAL DEFAULT 0,
			avg_price REAL DEFAULT 0,
			executed_qty REAL DEFAULT 0,
			leverage INTEGER DEFAULT 1,
			status TEXT DEFAULT 'NEW',
			fee REAL DEFAULT 0,
			fee_asset TEXT DEFAULT 'USDT',
			realized_pnl REAL DEFAULT 0,
			entry_price REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			filled_at DATETIME,
			UNIQUE(trader_id, order_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create trader_orders table: %w", err)
	}

	// Create indexes
	indices := []string{
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_trader ON trader_orders(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_status ON trader_orders(trader_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_symbol ON trader_orders(trader_id, symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_filled ON trader_orders(trader_id, filled_at DESC)`,
	}
	for _, idx := range indices {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// Create creates order record
func (s *OrderStore) Create(order *TraderOrder) error {
	now := time.Now().Format(time.RFC3339)
	result, err := s.db.Exec(`
		INSERT INTO trader_orders (
			trader_id, order_id, client_order_id, symbol, side, position_side,
			action, order_type, quantity, price, avg_price, executed_qty,
			leverage, status, fee, fee_asset, realized_pnl, entry_price,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		order.TraderID, order.OrderID, order.ClientOrderID, order.Symbol,
		order.Side, order.PositionSide, order.Action, order.OrderType,
		order.Quantity, order.Price, order.AvgPrice, order.ExecutedQty,
		order.Leverage, order.Status, order.Fee, order.FeeAsset,
		order.RealizedPnL, order.EntryPrice, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create order record: %w", err)
	}

	id, _ := result.LastInsertId()
	order.ID = id
	return nil
}

// Update updates order record
func (s *OrderStore) Update(order *TraderOrder) error {
	now := time.Now().Format(time.RFC3339)
	filledAt := ""
	if !order.FilledAt.IsZero() {
		filledAt = order.FilledAt.Format(time.RFC3339)
	}

	_, err := s.db.Exec(`
		UPDATE trader_orders SET
			avg_price = ?, executed_qty = ?, status = ?, fee = ?,
			realized_pnl = ?, entry_price = ?, updated_at = ?, filled_at = ?
		WHERE trader_id = ? AND order_id = ?
	`,
		order.AvgPrice, order.ExecutedQty, order.Status, order.Fee,
		order.RealizedPnL, order.EntryPrice, now, filledAt,
		order.TraderID, order.OrderID,
	)
	if err != nil {
		return fmt.Errorf("failed to update order record: %w", err)
	}
	return nil
}

// GetByOrderID gets order by order ID
func (s *OrderStore) GetByOrderID(traderID, orderID string) (*TraderOrder, error) {
	var order TraderOrder
	var createdAt, updatedAt, filledAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, trader_id, order_id, client_order_id, symbol, side, position_side,
			action, order_type, quantity, price, avg_price, executed_qty,
			leverage, status, fee, fee_asset, realized_pnl, entry_price,
			created_at, updated_at, filled_at
		FROM trader_orders WHERE trader_id = ? AND order_id = ?
	`, traderID, orderID).Scan(
		&order.ID, &order.TraderID, &order.OrderID, &order.ClientOrderID,
		&order.Symbol, &order.Side, &order.PositionSide, &order.Action,
		&order.OrderType, &order.Quantity, &order.Price, &order.AvgPrice,
		&order.ExecutedQty, &order.Leverage, &order.Status, &order.Fee,
		&order.FeeAsset, &order.RealizedPnL, &order.EntryPrice,
		&createdAt, &updatedAt, &filledAt,
	)
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		order.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		order.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	if filledAt.Valid {
		order.FilledAt, _ = time.Parse(time.RFC3339, filledAt.String)
	}

	return &order, nil
}

// GetLatestOpenOrder gets the latest open order for a symbol (for calculating close PnL)
func (s *OrderStore) GetLatestOpenOrder(traderID, symbol, side string) (*TraderOrder, error) {
	// side: long -> find open_long, short -> find open_short
	action := "open_long"
	if side == "short" {
		action = "open_short"
	}

	var order TraderOrder
	var createdAt, updatedAt, filledAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, trader_id, order_id, client_order_id, symbol, side, position_side,
			action, order_type, quantity, price, avg_price, executed_qty,
			leverage, status, fee, fee_asset, realized_pnl, entry_price,
			created_at, updated_at, filled_at
		FROM trader_orders
		WHERE trader_id = ? AND symbol = ? AND action = ? AND status = 'FILLED'
		ORDER BY filled_at DESC LIMIT 1
	`, traderID, symbol, action).Scan(
		&order.ID, &order.TraderID, &order.OrderID, &order.ClientOrderID,
		&order.Symbol, &order.Side, &order.PositionSide, &order.Action,
		&order.OrderType, &order.Quantity, &order.Price, &order.AvgPrice,
		&order.ExecutedQty, &order.Leverage, &order.Status, &order.Fee,
		&order.FeeAsset, &order.RealizedPnL, &order.EntryPrice,
		&createdAt, &updatedAt, &filledAt,
	)
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		order.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		order.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	if filledAt.Valid {
		order.FilledAt, _ = time.Parse(time.RFC3339, filledAt.String)
	}

	return &order, nil
}

// GetRecentCompletedOrders gets recent completed close orders
func (s *OrderStore) GetRecentCompletedOrders(traderID string, limit int) ([]CompletedOrder, error) {
	rows, err := s.db.Query(`
		SELECT symbol, action, side, executed_qty, entry_price, avg_price,
			realized_pnl, fee, leverage, filled_at
		FROM trader_orders
		WHERE trader_id = ? AND status = 'FILLED'
			AND (action = 'close_long' OR action = 'close_short')
		ORDER BY filled_at DESC
		LIMIT ?
	`, traderID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query completed orders: %w", err)
	}
	defer rows.Close()

	var orders []CompletedOrder
	for rows.Next() {
		var o CompletedOrder
		var filledAt sql.NullString
		var side sql.NullString

		err := rows.Scan(
			&o.Symbol, &o.Action, &side, &o.Quantity, &o.EntryPrice, &o.ExitPrice,
			&o.RealizedPnL, &o.Fee, &o.Leverage, &filledAt,
		)
		if err != nil {
			continue
		}

		// Infer side from action
		if o.Action == "close_long" {
			o.Side = "long"
		} else if o.Action == "close_short" {
			o.Side = "short"
		} else if side.Valid {
			o.Side = side.String
		}

		// Calculate PnL percentage
		if o.EntryPrice > 0 {
			if o.Side == "long" {
				o.PnLPct = (o.ExitPrice - o.EntryPrice) / o.EntryPrice * 100 * float64(o.Leverage)
			} else {
				o.PnLPct = (o.EntryPrice - o.ExitPrice) / o.EntryPrice * 100 * float64(o.Leverage)
			}
		}

		if filledAt.Valid {
			o.FilledAt, _ = time.Parse(time.RFC3339, filledAt.String)
		}

		orders = append(orders, o)
	}

	return orders, nil
}

// GetTraderStats gets trading statistics metrics
func (s *OrderStore) GetTraderStats(traderID string) (*TraderStats, error) {
	stats := &TraderStats{}

	// Query all completed close orders
	rows, err := s.db.Query(`
		SELECT realized_pnl, fee, filled_at
		FROM trader_orders
		WHERE trader_id = ? AND status = 'FILLED'
			AND (action = 'close_long' OR action = 'close_short')
		ORDER BY filled_at ASC
	`, traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order statistics: %w", err)
	}
	defer rows.Close()

	var pnls []float64
	var totalWin, totalLoss float64

	for rows.Next() {
		var pnl, fee float64
		var filledAt sql.NullString
		if err := rows.Scan(&pnl, &fee, &filledAt); err != nil {
			continue
		}

		stats.TotalTrades++
		stats.TotalPnL += pnl
		stats.TotalFee += fee
		pnls = append(pnls, pnl)

		if pnl > 0 {
			stats.WinTrades++
			totalWin += pnl
		} else if pnl < 0 {
			stats.LossTrades++
			totalLoss += math.Abs(pnl)
		}
	}

	// Calculate win rate
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades) * 100
	}

	// Calculate profit factor
	if totalLoss > 0 {
		stats.ProfitFactor = totalWin / totalLoss
	}

	// Calculate average win/loss
	if stats.WinTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinTrades)
	}
	if stats.LossTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LossTrades)
	}

	// Calculate Sharpe ratio (using PnL sequence)
	if len(pnls) > 1 {
		stats.SharpeRatio = calculateSharpeRatio(pnls)
	}

	// Calculate max drawdown
	if len(pnls) > 0 {
		stats.MaxDrawdownPct = calculateMaxDrawdown(pnls)
	}

	return stats, nil
}

// calculateSharpeRatio calculates Sharpe ratio
func calculateSharpeRatio(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}

	// Calculate average return
	var sum float64
	for _, pnl := range pnls {
		sum += pnl
	}
	mean := sum / float64(len(pnls))

	// Calculate standard deviation
	var variance float64
	for _, pnl := range pnls {
		variance += (pnl - mean) * (pnl - mean)
	}
	stdDev := math.Sqrt(variance / float64(len(pnls)-1))

	if stdDev == 0 {
		return 0
	}

	// Sharpe ratio = average return / standard deviation
	return mean / stdDev
}

// calculateMaxDrawdown calculates max drawdown
func calculateMaxDrawdown(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}

	// Calculate cumulative equity curve
	var cumulative float64
	var peak float64
	var maxDD float64

	for _, pnl := range pnls {
		cumulative += pnl
		if cumulative > peak {
			peak = cumulative
		}
		if peak > 0 {
			dd := (peak - cumulative) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}

	return maxDD
}

// GetPendingOrders gets pending orders (for polling)
func (s *OrderStore) GetPendingOrders(traderID string) ([]*TraderOrder, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, order_id, client_order_id, symbol, side, position_side,
			action, order_type, quantity, price, avg_price, executed_qty,
			leverage, status, fee, fee_asset, realized_pnl, entry_price,
			created_at, updated_at, filled_at
		FROM trader_orders
		WHERE trader_id = ? AND status = 'NEW'
		ORDER BY created_at ASC
	`, traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending orders: %w", err)
	}
	defer rows.Close()

	return s.scanOrders(rows)
}

// GetAllPendingOrders gets all pending orders (for global sync)
func (s *OrderStore) GetAllPendingOrders() ([]*TraderOrder, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, order_id, client_order_id, symbol, side, position_side,
			action, order_type, quantity, price, avg_price, executed_qty,
			leverage, status, fee, fee_asset, realized_pnl, entry_price,
			created_at, updated_at, filled_at
		FROM trader_orders
		WHERE status = 'NEW'
		ORDER BY trader_id, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending orders: %w", err)
	}
	defer rows.Close()

	return s.scanOrders(rows)
}

// scanOrders scans order rows to structs
func (s *OrderStore) scanOrders(rows *sql.Rows) ([]*TraderOrder, error) {
	var orders []*TraderOrder
	for rows.Next() {
		var order TraderOrder
		var createdAt, updatedAt, filledAt sql.NullString

		err := rows.Scan(
			&order.ID, &order.TraderID, &order.OrderID, &order.ClientOrderID,
			&order.Symbol, &order.Side, &order.PositionSide, &order.Action,
			&order.OrderType, &order.Quantity, &order.Price, &order.AvgPrice,
			&order.ExecutedQty, &order.Leverage, &order.Status, &order.Fee,
			&order.FeeAsset, &order.RealizedPnL, &order.EntryPrice,
			&createdAt, &updatedAt, &filledAt,
		)
		if err != nil {
			continue
		}

		if createdAt.Valid {
			order.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		}
		if updatedAt.Valid {
			order.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
		}
		if filledAt.Valid {
			order.FilledAt, _ = time.Parse(time.RFC3339, filledAt.String)
		}

		orders = append(orders, &order)
	}

	return orders, nil
}
