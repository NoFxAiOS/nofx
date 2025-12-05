package store

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// TraderOrder 交易员订单记录
type TraderOrder struct {
	ID            int64     `json:"id"`
	TraderID      string    `json:"trader_id"`       // 交易员ID
	OrderID       string    `json:"order_id"`        // 交易所订单ID
	ClientOrderID string    `json:"client_order_id"` // 客户端订单ID
	Symbol        string    `json:"symbol"`          // 交易对
	Side          string    `json:"side"`            // BUY/SELL
	PositionSide  string    `json:"position_side"`   // LONG/SHORT/BOTH
	Action        string    `json:"action"`          // open_long/close_long/open_short/close_short
	OrderType     string    `json:"order_type"`      // MARKET/LIMIT
	Quantity      float64   `json:"quantity"`        // 订单数量
	Price         float64   `json:"price"`           // 订单价格
	AvgPrice      float64   `json:"avg_price"`       // 实际成交均价
	ExecutedQty   float64   `json:"executed_qty"`    // 已成交数量
	Leverage      int       `json:"leverage"`        // 杠杆倍数
	Status        string    `json:"status"`          // NEW/FILLED/CANCELED/EXPIRED
	Fee           float64   `json:"fee"`             // 手续费
	FeeAsset      string    `json:"fee_asset"`       // 手续费资产
	RealizedPnL   float64   `json:"realized_pnl"`    // 已实现盈亏（平仓时）
	EntryPrice    float64   `json:"entry_price"`     // 开仓价（平仓时记录）
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	FilledAt      time.Time `json:"filled_at"` // 成交时间
}

// TraderStats 交易统计指标
type TraderStats struct {
	TotalTrades    int     `json:"total_trades"`     // 总交易数（已平仓）
	WinTrades      int     `json:"win_trades"`       // 盈利交易数
	LossTrades     int     `json:"loss_trades"`      // 亏损交易数
	WinRate        float64 `json:"win_rate"`         // 胜率 (%)
	ProfitFactor   float64 `json:"profit_factor"`    // 盈亏比
	SharpeRatio    float64 `json:"sharpe_ratio"`     // 夏普比
	TotalPnL       float64 `json:"total_pnl"`        // 总盈亏
	TotalFee       float64 `json:"total_fee"`        // 总手续费
	AvgWin         float64 `json:"avg_win"`          // 平均盈利
	AvgLoss        float64 `json:"avg_loss"`         // 平均亏损
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // 最大回撤 (%)
}

// CompletedOrder 已完成订单（用于AI输入）
type CompletedOrder struct {
	Symbol      string    `json:"symbol"`       // 交易对
	Action      string    `json:"action"`       // close_long/close_short
	Side        string    `json:"side"`         // long/short
	Quantity    float64   `json:"quantity"`     // 数量
	EntryPrice  float64   `json:"entry_price"`  // 开仓价
	ExitPrice   float64   `json:"exit_price"`   // 平仓价
	RealizedPnL float64   `json:"realized_pnl"` // 已实现盈亏
	PnLPct      float64   `json:"pnl_pct"`      // 盈亏百分比
	Fee         float64   `json:"fee"`          // 手续费
	Leverage    int       `json:"leverage"`     // 杠杆
	FilledAt    time.Time `json:"filled_at"`    // 成交时间
}

// OrderStore 订单存储
type OrderStore struct {
	db *sql.DB
}

// NewOrderStore 创建订单存储实例
func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

// InitTables 初始化订单表
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
		return fmt.Errorf("创建trader_orders表失败: %w", err)
	}

	// 创建索引
	indices := []string{
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_trader ON trader_orders(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_status ON trader_orders(trader_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_symbol ON trader_orders(trader_id, symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_orders_filled ON trader_orders(trader_id, filled_at DESC)`,
	}
	for _, idx := range indices {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	return nil
}

// Create 创建订单记录
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
		return fmt.Errorf("创建订单记录失败: %w", err)
	}

	id, _ := result.LastInsertId()
	order.ID = id
	return nil
}

// Update 更新订单记录
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
		return fmt.Errorf("更新订单记录失败: %w", err)
	}
	return nil
}

// GetByOrderID 根据订单ID获取订单
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

// GetLatestOpenOrder 获取某币种最近的开仓订单（用于计算平仓盈亏）
func (s *OrderStore) GetLatestOpenOrder(traderID, symbol, side string) (*TraderOrder, error) {
	// side: long -> 找 open_long, short -> 找 open_short
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

// GetRecentCompletedOrders 获取最近已完成的平仓订单
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
		return nil, fmt.Errorf("查询已完成订单失败: %w", err)
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

		// 根据action推断side
		if o.Action == "close_long" {
			o.Side = "long"
		} else if o.Action == "close_short" {
			o.Side = "short"
		} else if side.Valid {
			o.Side = side.String
		}

		// 计算盈亏百分比
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

// GetTraderStats 获取交易统计指标
func (s *OrderStore) GetTraderStats(traderID string) (*TraderStats, error) {
	stats := &TraderStats{}

	// 查询所有已完成的平仓订单
	rows, err := s.db.Query(`
		SELECT realized_pnl, fee, filled_at
		FROM trader_orders
		WHERE trader_id = ? AND status = 'FILLED'
			AND (action = 'close_long' OR action = 'close_short')
		ORDER BY filled_at ASC
	`, traderID)
	if err != nil {
		return nil, fmt.Errorf("查询订单统计失败: %w", err)
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

	// 计算胜率
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades) * 100
	}

	// 计算盈亏比
	if totalLoss > 0 {
		stats.ProfitFactor = totalWin / totalLoss
	}

	// 计算平均盈亏
	if stats.WinTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinTrades)
	}
	if stats.LossTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LossTrades)
	}

	// 计算夏普比（使用盈亏序列）
	if len(pnls) > 1 {
		stats.SharpeRatio = calculateSharpeRatio(pnls)
	}

	// 计算最大回撤
	if len(pnls) > 0 {
		stats.MaxDrawdownPct = calculateMaxDrawdown(pnls)
	}

	return stats, nil
}

// calculateSharpeRatio 计算夏普比
func calculateSharpeRatio(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}

	// 计算平均收益
	var sum float64
	for _, pnl := range pnls {
		sum += pnl
	}
	mean := sum / float64(len(pnls))

	// 计算标准差
	var variance float64
	for _, pnl := range pnls {
		variance += (pnl - mean) * (pnl - mean)
	}
	stdDev := math.Sqrt(variance / float64(len(pnls)-1))

	if stdDev == 0 {
		return 0
	}

	// 夏普比 = 平均收益 / 标准差
	return mean / stdDev
}

// calculateMaxDrawdown 计算最大回撤
func calculateMaxDrawdown(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}

	// 计算累计权益曲线
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

// GetPendingOrders 获取未成交的订单（用于轮询）
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
		return nil, fmt.Errorf("查询未成交订单失败: %w", err)
	}
	defer rows.Close()

	return s.scanOrders(rows)
}

// GetAllPendingOrders 获取所有未成交的订单（用于全局同步）
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
		return nil, fmt.Errorf("查询未成交订单失败: %w", err)
	}
	defer rows.Close()

	return s.scanOrders(rows)
}

// scanOrders 扫描订单行到结构体
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
