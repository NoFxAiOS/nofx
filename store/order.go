package store

import (
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// TraderOrder order record
// All time fields use int64 millisecond timestamps (UTC) to avoid timezone issues
type TraderOrder struct {
	ID                int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TraderID          string  `gorm:"column:trader_id;not null;index:idx_orders_trader_id" json:"trader_id"`
	ExchangeID        string  `gorm:"column:exchange_id;not null;default:''" json:"exchange_id"`
	ExchangeType      string  `gorm:"column:exchange_type;not null;default:''" json:"exchange_type"`
	ExchangeOrderID   string  `gorm:"column:exchange_order_id;not null;uniqueIndex:idx_orders_exchange_unique,priority:2" json:"exchange_order_id"`
	ClientOrderID     string  `gorm:"column:client_order_id;default:''" json:"client_order_id"`
	Symbol            string  `gorm:"column:symbol;not null;index:idx_orders_symbol" json:"symbol"`
	Side              string  `gorm:"column:side;not null" json:"side"`
	PositionSide      string  `gorm:"column:position_side;default:''" json:"position_side"`
	Type              string  `gorm:"column:type;not null" json:"type"`
	TimeInForce       string  `gorm:"column:time_in_force;default:GTC" json:"time_in_force"`
	Quantity          float64 `gorm:"column:quantity;not null" json:"quantity"`
	Price             float64 `gorm:"column:price;default:0" json:"price"`
	StopPrice         float64 `gorm:"column:stop_price;default:0" json:"stop_price"`
	Status            string  `gorm:"column:status;not null;default:NEW;index:idx_orders_status" json:"status"`
	FilledQuantity    float64 `gorm:"column:filled_quantity;default:0" json:"filled_quantity"`
	AvgFillPrice      float64 `gorm:"column:avg_fill_price;default:0" json:"avg_fill_price"`
	Commission        float64 `gorm:"column:commission;default:0" json:"commission"`
	CommissionAsset   string  `gorm:"column:commission_asset;default:USDT" json:"commission_asset"`
	Leverage          int     `gorm:"column:leverage;default:1" json:"leverage"`
	ReduceOnly        bool    `gorm:"column:reduce_only;default:false" json:"reduce_only"`
	ClosePosition     bool    `gorm:"column:close_position;default:false" json:"close_position"`
	WorkingType       string  `gorm:"column:working_type;default:CONTRACT_PRICE" json:"working_type"`
	PriceProtect      bool    `gorm:"column:price_protect;default:false" json:"price_protect"`
	OrderAction       string  `gorm:"column:order_action;default:''" json:"order_action"`
	RelatedPositionID int64   `gorm:"column:related_position_id;default:0" json:"related_position_id"`
	CreatedAt         int64   `gorm:"column:created_at" json:"created_at"`         // Unix milliseconds UTC
	UpdatedAt         int64   `gorm:"column:updated_at" json:"updated_at"`         // Unix milliseconds UTC
	FilledAt          int64   `gorm:"column:filled_at" json:"filled_at"`           // Unix milliseconds UTC
}

// TableName returns the table name for TraderOrder
func (TraderOrder) TableName() string {
	return "trader_orders"
}

// TraderFill trade record
// All time fields use int64 millisecond timestamps (UTC) to avoid timezone issues
type TraderFill struct {
	ID              int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	TraderID        string  `gorm:"column:trader_id;not null;index:idx_fills_trader_id" json:"trader_id"`
	ExchangeID      string  `gorm:"column:exchange_id;not null;default:''" json:"exchange_id"`
	ExchangeType    string  `gorm:"column:exchange_type;not null;default:''" json:"exchange_type"`
	OrderID         int64   `gorm:"column:order_id;not null;index:idx_fills_order_id" json:"order_id"`
	ExchangeOrderID string  `gorm:"column:exchange_order_id;not null" json:"exchange_order_id"`
	ExchangeTradeID string  `gorm:"column:exchange_trade_id;not null;uniqueIndex:idx_fills_exchange_unique,priority:2" json:"exchange_trade_id"`
	Symbol          string  `gorm:"column:symbol;not null" json:"symbol"`
	Side            string  `gorm:"column:side;not null" json:"side"`
	Price           float64 `gorm:"column:price;not null" json:"price"`
	Quantity        float64 `gorm:"column:quantity;not null" json:"quantity"`
	QuoteQuantity   float64 `gorm:"column:quote_quantity;not null" json:"quote_quantity"`
	Commission      float64 `gorm:"column:commission;not null" json:"commission"`
	CommissionAsset string  `gorm:"column:commission_asset;not null" json:"commission_asset"`
	RealizedPnL     float64 `gorm:"column:realized_pnl;default:0" json:"realized_pnl"`
	IsMaker         bool    `gorm:"column:is_maker;default:false" json:"is_maker"`
	CreatedAt       int64   `gorm:"column:created_at" json:"created_at"` // Unix milliseconds UTC
}

// TableName returns the table name for TraderFill
func (TraderFill) TableName() string {
	return "trader_fills"
}

// OrderStore order storage
type OrderStore struct {
	db *gorm.DB
}

// NewOrderStore creates order storage instance
func NewOrderStore(db *gorm.DB) *OrderStore {
	return &OrderStore{db: db}
}

// InitTables initializes order tables
func (s *OrderStore) InitTables() error {
	// For PostgreSQL, check if tables exist to avoid AutoMigrate index conflicts
	if s.db.Dialector.Name() == "postgres" {
		var ordersExist, fillsExist int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'trader_orders'`).Scan(&ordersExist)
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'trader_fills'`).Scan(&fillsExist)

		if ordersExist > 0 && fillsExist > 0 {
			// Tables exist - fix INTEGER columns to BOOLEAN (from earlier migrations)
			// Need to: drop default -> change type -> set new default
			boolColumns := []struct{ table, col string }{
				{"trader_orders", "reduce_only"},
				{"trader_orders", "close_position"},
				{"trader_orders", "price_protect"},
				{"trader_fills", "is_maker"},
			}
			for _, c := range boolColumns {
				s.db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT", c.table, c.col))
				s.db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE BOOLEAN USING %s::int::boolean", c.table, c.col, c.col))
				s.db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT false", c.table, c.col))
			}

			// Migrate timestamp columns to bigint (Unix milliseconds UTC)
			// Check if column is still timestamp type before migrating
			timestampColumns := []struct{ table, col string }{
				{"trader_orders", "created_at"},
				{"trader_orders", "updated_at"},
				{"trader_orders", "filled_at"},
				{"trader_fills", "created_at"},
			}
			for _, c := range timestampColumns {
				var dataType string
				s.db.Raw(`SELECT data_type FROM information_schema.columns WHERE table_name = ? AND column_name = ?`, c.table, c.col).Scan(&dataType)
				if dataType == "timestamp with time zone" || dataType == "timestamp without time zone" {
					// Convert timestamp to Unix milliseconds (bigint)
					s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE BIGINT USING EXTRACT(EPOCH FROM %s) * 1000`, c.table, c.col, c.col))
				}
			}

			// Ensure indexes exist
			s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_exchange_unique ON trader_orders(exchange_id, exchange_order_id)`)
			s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_fills_exchange_unique ON trader_fills(exchange_id, exchange_trade_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_trader_id ON trader_orders(trader_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_symbol ON trader_orders(symbol)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_status ON trader_orders(status)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_fills_trader_id ON trader_fills(trader_id)`)
			s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_fills_order_id ON trader_fills(order_id)`)
			return nil
		}
	}

	if err := s.db.AutoMigrate(&TraderOrder{}, &TraderFill{}); err != nil {
		return fmt.Errorf("failed to migrate order tables: %w", err)
	}

	// Create unique composite index for exchange_id + exchange_order_id
	s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_exchange_unique ON trader_orders(exchange_id, exchange_order_id)`)
	// Create unique composite index for exchange_id + exchange_trade_id
	s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_fills_exchange_unique ON trader_fills(exchange_id, exchange_trade_id)`)

	return nil
}

// CreateOrder creates order record
func (s *OrderStore) CreateOrder(order *TraderOrder) error {
	// Check if order already exists
	existing, err := s.GetOrderByExchangeID(order.ExchangeID, order.ExchangeOrderID)
	if err != nil {
		return fmt.Errorf("failed to check existing order: %w", err)
	}
	if existing != nil {
		order.ID = existing.ID
		order.CreatedAt = existing.CreatedAt
		order.UpdatedAt = existing.UpdatedAt
		return nil
	}

	return s.db.Create(order).Error
}

// UpdateOrderStatus updates order status
func (s *OrderStore) UpdateOrderStatus(id int64, status string, filledQty, avgPrice, commission float64) error {
	updates := map[string]interface{}{
		"status":          status,
		"filled_quantity": filledQty,
		"avg_fill_price":  avgPrice,
		"commission":      commission,
		"updated_at":      time.Now().UTC().UnixMilli(),
	}

	if status == "FILLED" {
		updates["filled_at"] = time.Now().UTC().UnixMilli()
	}

	return s.db.Model(&TraderOrder{}).Where("id = ?", id).Updates(updates).Error
}

// CreateFill creates fill record
func (s *OrderStore) CreateFill(fill *TraderFill) error {
	// Check if fill already exists
	existing, err := s.GetFillByExchangeTradeID(fill.ExchangeID, fill.ExchangeTradeID)
	if err != nil {
		return fmt.Errorf("failed to check existing fill: %w", err)
	}
	if existing != nil {
		fill.ID = existing.ID
		fill.CreatedAt = existing.CreatedAt
		return nil
	}

	return s.db.Create(fill).Error
}

// GetFillByExchangeTradeID gets fill by exchange trade ID
func (s *OrderStore) GetFillByExchangeTradeID(exchangeID, exchangeTradeID string) (*TraderFill, error) {
	var fill TraderFill
	err := s.db.Where("exchange_id = ? AND exchange_trade_id = ?", exchangeID, exchangeTradeID).First(&fill).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get fill: %w", err)
	}
	return &fill, nil
}

// GetOrderByExchangeID gets order by exchange order ID
func (s *OrderStore) GetOrderByExchangeID(exchangeID, exchangeOrderID string) (*TraderOrder, error) {
	var order TraderOrder
	err := s.db.Where("exchange_id = ? AND exchange_order_id = ?", exchangeID, exchangeOrderID).First(&order).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	return &order, nil
}

// GetTraderOrders gets trader's order list
func (s *OrderStore) GetTraderOrders(traderID string, limit int) ([]*TraderOrder, error) {
	var orders []*TraderOrder
	err := s.db.Where("trader_id = ?", traderID).
		Order("created_at DESC").
		Limit(limit).
		Find(&orders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	return orders, nil
}

// GetTraderOrdersFiltered gets trader's order list with optional symbol and status filters
func (s *OrderStore) GetTraderOrdersFiltered(traderID string, symbol string, status string, limit int) ([]*TraderOrder, error) {
	var orders []*TraderOrder
	query := s.db.Where("trader_id = ?", traderID)

	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").
		Limit(limit).
		Find(&orders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	return orders, nil
}

// GetOrderFills gets order's fill records
func (s *OrderStore) GetOrderFills(orderID int64) ([]*TraderFill, error) {
	var fills []*TraderFill
	err := s.db.Where("order_id = ?", orderID).
		Order("created_at ASC").
		Find(&fills).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query fills: %w", err)
	}
	return fills, nil
}

// GetTraderOrderStats gets trader's order statistics
func (s *OrderStore) GetTraderOrderStats(traderID string) (map[string]interface{}, error) {
	type result struct {
		TotalOrders     int
		FilledOrders    int
		CanceledOrders  int
		TotalCommission float64
		TotalVolume     float64
	}
	var r result

	err := s.db.Model(&TraderOrder{}).
		Select(`COUNT(*) as total_orders,
				SUM(CASE WHEN status = 'FILLED' THEN 1 ELSE 0 END) as filled_orders,
				SUM(CASE WHEN status = 'CANCELED' THEN 1 ELSE 0 END) as canceled_orders,
				SUM(commission) as total_commission,
				SUM(filled_quantity * avg_fill_price) as total_volume`).
		Where("trader_id = ?", traderID).
		Scan(&r).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get order stats: %w", err)
	}

	return map[string]interface{}{
		"total_orders":     r.TotalOrders,
		"filled_orders":    r.FilledOrders,
		"canceled_orders":  r.CanceledOrders,
		"total_commission": r.TotalCommission,
		"total_volume":     r.TotalVolume,
	}, nil
}

// CleanupDuplicateOrders cleans up duplicate order records
func (s *OrderStore) CleanupDuplicateOrders() (int, error) {
	result := s.db.Exec(`
		DELETE FROM trader_orders
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM trader_orders
			GROUP BY exchange_id, exchange_order_id
		)
	`)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup duplicate orders: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

// CleanupDuplicateFills cleans up duplicate fill records
func (s *OrderStore) CleanupDuplicateFills() (int, error) {
	result := s.db.Exec(`
		DELETE FROM trader_fills
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM trader_fills
			GROUP BY exchange_id, exchange_trade_id
		)
	`)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup duplicate fills: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

// GetDuplicateOrdersCount gets duplicate orders count
func (s *OrderStore) GetDuplicateOrdersCount() (int, error) {
	var total, distinct int64
	s.db.Model(&TraderOrder{}).Count(&total)

	// Count distinct combinations
	var distinctResult struct{ Count int64 }
	s.db.Model(&TraderOrder{}).
		Select("COUNT(DISTINCT exchange_id || ',' || exchange_order_id) as count").
		Scan(&distinctResult)
	distinct = distinctResult.Count

	return int(total - distinct), nil
}

// GetDuplicateFillsCount gets duplicate fills count
func (s *OrderStore) GetDuplicateFillsCount() (int, error) {
	var total, distinct int64
	s.db.Model(&TraderFill{}).Count(&total)

	var distinctResult struct{ Count int64 }
	s.db.Model(&TraderFill{}).
		Select("COUNT(DISTINCT exchange_id || ',' || exchange_trade_id) as count").
		Scan(&distinctResult)
	distinct = distinctResult.Count

	return int(total - distinct), nil
}

// GetMaxTradeIDsByExchange returns max trade ID for each symbol for a given exchange
func (s *OrderStore) GetMaxTradeIDsByExchange(exchangeID string) (map[string]int64, error) {
	type symbolTradeID struct {
		Symbol          string
		ExchangeTradeID string
	}
	var results []symbolTradeID

	// Query all trade IDs grouped by symbol, find max in Go to avoid database-specific CAST issues
	// (PostgreSQL INTEGER is 32-bit, can't handle Binance trade IDs > 2.1B)
	err := s.db.Model(&TraderFill{}).
		Select("symbol, exchange_trade_id").
		Where("exchange_id = ? AND exchange_trade_id != ''", exchangeID).
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query trade IDs: %w", err)
	}

	// Find max trade ID per symbol in Go (handles 64-bit integers properly)
	result := make(map[string]int64)
	for _, r := range results {
		tradeID, err := strconv.ParseInt(r.ExchangeTradeID, 10, 64)
		if err != nil {
			continue // Skip non-numeric trade IDs
		}
		if tradeID > result[r.Symbol] {
			result[r.Symbol] = tradeID
		}
	}

	return result, nil
}

// GetLastFillTimeByExchange returns the most recent fill time (Unix ms) for a given exchange
// Used to recover sync state after service restart
func (s *OrderStore) GetLastFillTimeByExchange(exchangeID string) (int64, error) {
	var fill TraderFill
	err := s.db.Where("exchange_id = ?", exchangeID).
		Order("created_at DESC").
		First(&fill).Error
	if err != nil {
		return 0, err
	}
	return fill.CreatedAt, nil
}

// GetRecentFillSymbolsByExchange returns distinct symbols with fills since given time (Unix ms)
func (s *OrderStore) GetRecentFillSymbolsByExchange(exchangeID string, sinceMs int64) ([]string, error) {
	var symbols []string
	err := s.db.Model(&TraderFill{}).
		Select("DISTINCT symbol").
		Where("exchange_id = ? AND created_at >= ?", exchangeID, sinceMs).
		Pluck("symbol", &symbols).Error
	if err != nil {
		return nil, err
	}
	return symbols, nil
}
