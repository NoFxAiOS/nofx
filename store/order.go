package store

import (
	"database/sql"
	"fmt"
	"time"
)

// TraderOrder 订单记录（完整的订单生命周期追踪）
type TraderOrder struct {
	ID                int64     `json:"id"`
	TraderID          string    `json:"trader_id"`
	ExchangeID        string    `json:"exchange_id"`         // 交易所账户UUID
	ExchangeOrderID   string    `json:"exchange_order_id"`   // 交易所订单ID
	ClientOrderID     string    `json:"client_order_id"`     // 客户端订单ID
	Symbol            string    `json:"symbol"`              // 交易对
	Side              string    `json:"side"`                // BUY/SELL
	PositionSide      string    `json:"position_side"`       // LONG/SHORT (双向持仓模式)
	Type              string    `json:"type"`                // MARKET/LIMIT/STOP/STOP_MARKET/TAKE_PROFIT/TAKE_PROFIT_MARKET
	TimeInForce       string    `json:"time_in_force"`       // GTC/IOC/FOK
	Quantity          float64   `json:"quantity"`            // 订单数量
	Price             float64   `json:"price"`               // 限价单价格
	StopPrice         float64   `json:"stop_price"`          // 止损/止盈触发价格
	Status            string    `json:"status"`              // NEW/PARTIALLY_FILLED/FILLED/CANCELED/REJECTED/EXPIRED
	FilledQuantity    float64   `json:"filled_quantity"`     // 已成交数量
	AvgFillPrice      float64   `json:"avg_fill_price"`      // 平均成交价格
	Commission        float64   `json:"commission"`          // 手续费总额
	CommissionAsset   string    `json:"commission_asset"`    // 手续费资产（USDT等）
	Leverage          int       `json:"leverage"`            // 杠杆倍数
	ReduceOnly        bool      `json:"reduce_only"`         // 是否只减仓
	ClosePosition     bool      `json:"close_position"`      // 是否平仓单
	WorkingType       string    `json:"working_type"`        // CONTRACT_PRICE/MARK_PRICE
	PriceProtect      bool      `json:"price_protect"`       // 价格保护
	OrderAction       string    `json:"order_action"`        // OPEN_LONG/OPEN_SHORT/CLOSE_LONG/CLOSE_SHORT/ADD_LONG/ADD_SHORT/STOP_LOSS/TAKE_PROFIT
	RelatedPositionID int64     `json:"related_position_id"` // 关联的仓位ID
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	FilledAt          time.Time `json:"filled_at"` // 完全成交时间
}

// TraderFill 成交记录（一个订单可能有多次成交）
type TraderFill struct {
	ID               int64     `json:"id"`
	TraderID         string    `json:"trader_id"`
	ExchangeID       string    `json:"exchange_id"`
	OrderID          int64     `json:"order_id"`           // 关联的订单ID
	ExchangeOrderID  string    `json:"exchange_order_id"`  // 交易所订单ID
	ExchangeTradeID  string    `json:"exchange_trade_id"`  // 交易所成交ID
	Symbol           string    `json:"symbol"`
	Side             string    `json:"side"`           // BUY/SELL
	Price            float64   `json:"price"`          // 成交价格
	Quantity         float64   `json:"quantity"`       // 成交数量
	QuoteQuantity    float64   `json:"quote_quantity"` // 成交金额（USDT）
	Commission       float64   `json:"commission"`     // 手续费
	CommissionAsset  string    `json:"commission_asset"`
	RealizedPnL      float64   `json:"realized_pnl"` // 实现盈亏（平仓时）
	IsMaker          bool      `json:"is_maker"`     // 是否为maker
	CreatedAt        time.Time `json:"created_at"`
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
	// 创建订单表
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS trader_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL DEFAULT '',
			exchange_order_id TEXT NOT NULL,
			client_order_id TEXT DEFAULT '',
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			position_side TEXT DEFAULT '',
			type TEXT NOT NULL,
			time_in_force TEXT DEFAULT 'GTC',
			quantity REAL NOT NULL,
			price REAL DEFAULT 0,
			stop_price REAL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'NEW',
			filled_quantity REAL DEFAULT 0,
			avg_fill_price REAL DEFAULT 0,
			commission REAL DEFAULT 0,
			commission_asset TEXT DEFAULT 'USDT',
			leverage INTEGER DEFAULT 1,
			reduce_only INTEGER DEFAULT 0,
			close_position INTEGER DEFAULT 0,
			working_type TEXT DEFAULT 'CONTRACT_PRICE',
			price_protect INTEGER DEFAULT 0,
			order_action TEXT DEFAULT '',
			related_position_id INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			filled_at DATETIME,
			UNIQUE(exchange_id, exchange_order_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create trader_orders table: %w", err)
	}

	// 创建成交记录表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS trader_fills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL DEFAULT '',
			order_id INTEGER NOT NULL,
			exchange_order_id TEXT NOT NULL,
			exchange_trade_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			price REAL NOT NULL,
			quantity REAL NOT NULL,
			quote_quantity REAL NOT NULL,
			commission REAL NOT NULL,
			commission_asset TEXT NOT NULL,
			realized_pnl REAL DEFAULT 0,
			is_maker INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(exchange_id, exchange_trade_id),
			FOREIGN KEY (order_id) REFERENCES trader_orders(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create trader_fills table: %w", err)
	}

	// 创建索引
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_trader_id ON trader_orders(trader_id)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_symbol ON trader_orders(symbol)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_status ON trader_orders(status)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_exchange_order_id ON trader_orders(exchange_id, exchange_order_id)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_fills_order_id ON trader_fills(order_id)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_fills_trader_id ON trader_fills(trader_id)`)

	return nil
}

// CreateOrder 创建订单记录（去重：如果订单已存在则返回已有记录）
func (s *OrderStore) CreateOrder(order *TraderOrder) error {
	// 1. 先检查订单是否已存在（去重）
	existing, err := s.GetOrderByExchangeID(order.ExchangeID, order.ExchangeOrderID)
	if err != nil {
		return fmt.Errorf("failed to check existing order: %w", err)
	}
	if existing != nil {
		// 订单已存在，返回已有记录的ID
		order.ID = existing.ID
		order.CreatedAt = existing.CreatedAt
		order.UpdatedAt = existing.UpdatedAt
		return nil // 不是错误，只是跳过插入
	}

	// 2. 订单不存在，插入新记录
	now := time.Now()
	order.CreatedAt = now
	order.UpdatedAt = now

	result, err := s.db.Exec(`
		INSERT INTO trader_orders (
			trader_id, exchange_id, exchange_order_id, client_order_id,
			symbol, side, position_side, type, time_in_force,
			quantity, price, stop_price, status,
			filled_quantity, avg_fill_price, commission, commission_asset,
			leverage, reduce_only, close_position, working_type, price_protect,
			order_action, related_position_id,
			created_at, updated_at, filled_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		order.TraderID, order.ExchangeID, order.ExchangeOrderID, order.ClientOrderID,
		order.Symbol, order.Side, order.PositionSide, order.Type, order.TimeInForce,
		order.Quantity, order.Price, order.StopPrice, order.Status,
		order.FilledQuantity, order.AvgFillPrice, order.Commission, order.CommissionAsset,
		order.Leverage, order.ReduceOnly, order.ClosePosition, order.WorkingType, order.PriceProtect,
		order.OrderAction, order.RelatedPositionID,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
		formatTimePtr(order.FilledAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	id, _ := result.LastInsertId()
	order.ID = id
	return nil
}

// UpdateOrderStatus 更新订单状态
func (s *OrderStore) UpdateOrderStatus(id int64, status string, filledQty, avgPrice, commission float64) error {
	now := time.Now()
	updateSQL := `
		UPDATE trader_orders SET
			status = ?,
			filled_quantity = ?,
			avg_fill_price = ?,
			commission = ?,
			updated_at = ?
	`
	args := []interface{}{status, filledQty, avgPrice, commission, now.Format(time.RFC3339)}

	// 如果完全成交，记录成交时间
	if status == "FILLED" {
		updateSQL += `, filled_at = ?`
		args = append(args, now.Format(time.RFC3339))
	}

	updateSQL += ` WHERE id = ?`
	args = append(args, id)

	_, err := s.db.Exec(updateSQL, args...)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}

// CreateFill 创建成交记录（去重：如果成交已存在则跳过）
func (s *OrderStore) CreateFill(fill *TraderFill) error {
	// 1. 先检查成交是否已存在（去重）
	existing, err := s.GetFillByExchangeTradeID(fill.ExchangeID, fill.ExchangeTradeID)
	if err != nil {
		return fmt.Errorf("failed to check existing fill: %w", err)
	}
	if existing != nil {
		// 成交已存在，返回已有记录的ID
		fill.ID = existing.ID
		fill.CreatedAt = existing.CreatedAt
		return nil // 不是错误，只是跳过插入
	}

	// 2. 成交不存在，插入新记录
	now := time.Now()
	fill.CreatedAt = now

	result, err := s.db.Exec(`
		INSERT INTO trader_fills (
			trader_id, exchange_id, order_id, exchange_order_id, exchange_trade_id,
			symbol, side, price, quantity, quote_quantity,
			commission, commission_asset, realized_pnl, is_maker,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		fill.TraderID, fill.ExchangeID, fill.OrderID, fill.ExchangeOrderID, fill.ExchangeTradeID,
		fill.Symbol, fill.Side, fill.Price, fill.Quantity, fill.QuoteQuantity,
		fill.Commission, fill.CommissionAsset, fill.RealizedPnL, fill.IsMaker,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create fill: %w", err)
	}

	id, _ := result.LastInsertId()
	fill.ID = id
	return nil
}

// GetFillByExchangeTradeID 根据交易所成交ID获取成交记录
func (s *OrderStore) GetFillByExchangeTradeID(exchangeID, exchangeTradeID string) (*TraderFill, error) {
	row := s.db.QueryRow(`
		SELECT id, trader_id, exchange_id, order_id, exchange_order_id, exchange_trade_id,
			symbol, side, price, quantity, quote_quantity,
			commission, commission_asset, realized_pnl, is_maker,
			created_at
		FROM trader_fills
		WHERE exchange_id = ? AND exchange_trade_id = ?
	`, exchangeID, exchangeTradeID)

	var fill TraderFill
	var createdAt sql.NullString
	err := row.Scan(
		&fill.ID, &fill.TraderID, &fill.ExchangeID, &fill.OrderID, &fill.ExchangeOrderID, &fill.ExchangeTradeID,
		&fill.Symbol, &fill.Side, &fill.Price, &fill.Quantity, &fill.QuoteQuantity,
		&fill.Commission, &fill.CommissionAsset, &fill.RealizedPnL, &fill.IsMaker,
		&createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get fill: %w", err)
	}

	// Parse time
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			fill.CreatedAt = t
		}
	}

	return &fill, nil
}

// GetOrderByExchangeID 根据交易所订单ID获取订单
func (s *OrderStore) GetOrderByExchangeID(exchangeID, exchangeOrderID string) (*TraderOrder, error) {
	row := s.db.QueryRow(`
		SELECT id, trader_id, exchange_id, exchange_order_id, client_order_id,
			symbol, side, position_side, type, time_in_force,
			quantity, price, stop_price, status,
			filled_quantity, avg_fill_price, commission, commission_asset,
			leverage, reduce_only, close_position, working_type, price_protect,
			order_action, related_position_id,
			created_at, updated_at, filled_at
		FROM trader_orders
		WHERE exchange_id = ? AND exchange_order_id = ?
	`, exchangeID, exchangeOrderID)

	var order TraderOrder
	var createdAt, updatedAt, filledAt sql.NullString
	err := row.Scan(
		&order.ID, &order.TraderID, &order.ExchangeID, &order.ExchangeOrderID, &order.ClientOrderID,
		&order.Symbol, &order.Side, &order.PositionSide, &order.Type, &order.TimeInForce,
		&order.Quantity, &order.Price, &order.StopPrice, &order.Status,
		&order.FilledQuantity, &order.AvgFillPrice, &order.Commission, &order.CommissionAsset,
		&order.Leverage, &order.ReduceOnly, &order.ClosePosition, &order.WorkingType, &order.PriceProtect,
		&order.OrderAction, &order.RelatedPositionID,
		&createdAt, &updatedAt, &filledAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Parse times
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			order.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			order.UpdatedAt = t
		}
	}
	if filledAt.Valid {
		if t, err := time.Parse(time.RFC3339, filledAt.String); err == nil {
			order.FilledAt = t
		}
	}

	return &order, nil
}

// GetTraderOrders 获取trader的订单列表
func (s *OrderStore) GetTraderOrders(traderID string, limit int) ([]*TraderOrder, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, exchange_id, exchange_order_id, client_order_id,
			symbol, side, position_side, type, time_in_force,
			quantity, price, stop_price, status,
			filled_quantity, avg_fill_price, commission, commission_asset,
			leverage, reduce_only, close_position, working_type, price_protect,
			order_action, related_position_id,
			created_at, updated_at, filled_at
		FROM trader_orders
		WHERE trader_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, traderID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []*TraderOrder
	for rows.Next() {
		var order TraderOrder
		var createdAt, updatedAt, filledAt sql.NullString
		err := rows.Scan(
			&order.ID, &order.TraderID, &order.ExchangeID, &order.ExchangeOrderID, &order.ClientOrderID,
			&order.Symbol, &order.Side, &order.PositionSide, &order.Type, &order.TimeInForce,
			&order.Quantity, &order.Price, &order.StopPrice, &order.Status,
			&order.FilledQuantity, &order.AvgFillPrice, &order.Commission, &order.CommissionAsset,
			&order.Leverage, &order.ReduceOnly, &order.ClosePosition, &order.WorkingType, &order.PriceProtect,
			&order.OrderAction, &order.RelatedPositionID,
			&createdAt, &updatedAt, &filledAt,
		)
		if err != nil {
			continue
		}

		// Parse times
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				order.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				order.UpdatedAt = t
			}
		}
		if filledAt.Valid {
			if t, err := time.Parse(time.RFC3339, filledAt.String); err == nil {
				order.FilledAt = t
			}
		}

		orders = append(orders, &order)
	}

	return orders, nil
}

// GetOrderFills 获取订单的成交记录
func (s *OrderStore) GetOrderFills(orderID int64) ([]*TraderFill, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, exchange_id, order_id, exchange_order_id, exchange_trade_id,
			symbol, side, price, quantity, quote_quantity,
			commission, commission_asset, realized_pnl, is_maker,
			created_at
		FROM trader_fills
		WHERE order_id = ?
		ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query fills: %w", err)
	}
	defer rows.Close()

	var fills []*TraderFill
	for rows.Next() {
		var fill TraderFill
		var createdAt sql.NullString
		err := rows.Scan(
			&fill.ID, &fill.TraderID, &fill.ExchangeID, &fill.OrderID, &fill.ExchangeOrderID, &fill.ExchangeTradeID,
			&fill.Symbol, &fill.Side, &fill.Price, &fill.Quantity, &fill.QuoteQuantity,
			&fill.Commission, &fill.CommissionAsset, &fill.RealizedPnL, &fill.IsMaker,
			&createdAt,
		)
		if err != nil {
			continue
		}

		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				fill.CreatedAt = t
			}
		}

		fills = append(fills, &fill)
	}

	return fills, nil
}

// GetTraderOrderStats 获取trader的订单统计
func (s *OrderStore) GetTraderOrderStats(traderID string) (map[string]interface{}, error) {
	var totalOrders, filledOrders, canceledOrders int
	var totalCommission, totalVolume float64

	err := s.db.QueryRow(`
		SELECT
			COUNT(*) as total_orders,
			SUM(CASE WHEN status = 'FILLED' THEN 1 ELSE 0 END) as filled_orders,
			SUM(CASE WHEN status = 'CANCELED' THEN 1 ELSE 0 END) as canceled_orders,
			SUM(commission) as total_commission,
			SUM(filled_quantity * avg_fill_price) as total_volume
		FROM trader_orders
		WHERE trader_id = ?
	`, traderID).Scan(&totalOrders, &filledOrders, &canceledOrders, &totalCommission, &totalVolume)

	if err != nil {
		return nil, fmt.Errorf("failed to get order stats: %w", err)
	}

	return map[string]interface{}{
		"total_orders":     totalOrders,
		"filled_orders":    filledOrders,
		"canceled_orders":  canceledOrders,
		"total_commission": totalCommission,
		"total_volume":     totalVolume,
	}, nil
}

// CleanupDuplicateOrders 清理重复的订单记录（保留最早创建的记录）
func (s *OrderStore) CleanupDuplicateOrders() (int, error) {
	result, err := s.db.Exec(`
		DELETE FROM trader_orders
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM trader_orders
			GROUP BY exchange_id, exchange_order_id
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup duplicate orders: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// CleanupDuplicateFills 清理重复的成交记录（保留最早创建的记录）
func (s *OrderStore) CleanupDuplicateFills() (int, error) {
	result, err := s.db.Exec(`
		DELETE FROM trader_fills
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM trader_fills
			GROUP BY exchange_id, exchange_trade_id
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup duplicate fills: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// GetDuplicateOrdersCount 获取重复订单的数量（用于诊断）
func (s *OrderStore) GetDuplicateOrdersCount() (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) - COUNT(DISTINCT exchange_id || ',' || exchange_order_id)
		FROM trader_orders
	`).Scan(&count)
	return count, err
}

// GetDuplicateFillsCount 获取重复成交的数量（用于诊断）
func (s *OrderStore) GetDuplicateFillsCount() (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) - COUNT(DISTINCT exchange_id || ',' || exchange_trade_id)
		FROM trader_fills
	`).Scan(&count)
	return count, err
}

// formatTimePtr formats time.Time to RFC3339 string, returns NULL for zero time
func formatTimePtr(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}
