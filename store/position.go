package store

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

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

// TraderPosition position record (complete open/close position tracking)
type TraderPosition struct {
	ID           int64      `json:"id"`
	TraderID     string     `json:"trader_id"`
	ExchangeID   string     `json:"exchange_id"`    // Exchange ID: binance/bybit/hyperliquid/aster/lighter
	Symbol       string     `json:"symbol"`
	Side         string     `json:"side"`           // LONG/SHORT
	Quantity     float64    `json:"quantity"`       // Opening quantity
	EntryPrice   float64    `json:"entry_price"`    // Entry price
	EntryOrderID string     `json:"entry_order_id"` // Entry order ID
	EntryTime    time.Time  `json:"entry_time"`     // Entry time
	ExitPrice    float64    `json:"exit_price"`     // Exit price
	ExitOrderID  string     `json:"exit_order_id"`  // Exit order ID
	ExitTime     *time.Time `json:"exit_time"`      // Exit time
	RealizedPnL  float64    `json:"realized_pnl"`   // Realized profit and loss
	Fee          float64    `json:"fee"`            // Fee
	Leverage     int        `json:"leverage"`       // Leverage multiplier
	Status       string     `json:"status"`         // OPEN/CLOSED
	CloseReason  string     `json:"close_reason"`   // Close reason: ai_decision/manual/stop_loss/take_profit
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PositionStore position storage
type PositionStore struct {
	db *sql.DB
}

// NewPositionStore creates position storage instance
func NewPositionStore(db *sql.DB) *PositionStore {
	return &PositionStore{db: db}
}

// InitTables initializes position tables
func (s *PositionStore) InitTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS trader_positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL DEFAULT '',
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			quantity REAL NOT NULL,
			entry_price REAL NOT NULL,
			entry_order_id TEXT DEFAULT '',
			entry_time DATETIME NOT NULL,
			exit_price REAL DEFAULT 0,
			exit_order_id TEXT DEFAULT '',
			exit_time DATETIME,
			realized_pnl REAL DEFAULT 0,
			fee REAL DEFAULT 0,
			leverage INTEGER DEFAULT 1,
			status TEXT DEFAULT 'OPEN',
			close_reason TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create trader_positions table: %w", err)
	}

	// Migration: add exchange_id column to existing table (if not exists)
	// Must be executed before creating indexes!
	s.db.Exec(`ALTER TABLE trader_positions ADD COLUMN exchange_id TEXT NOT NULL DEFAULT ''`)

	// Create indexes (after migration)
	indices := []string{
		`CREATE INDEX IF NOT EXISTS idx_positions_trader ON trader_positions(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_exchange ON trader_positions(exchange_id)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_status ON trader_positions(trader_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_symbol ON trader_positions(trader_id, symbol, side, status)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_entry ON trader_positions(trader_id, entry_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_exit ON trader_positions(trader_id, exit_time DESC)`,
	}
	for _, idx := range indices {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// Create creates position record (called when opening position)
func (s *PositionStore) Create(pos *TraderPosition) error {
	now := time.Now()
	pos.CreatedAt = now
	pos.UpdatedAt = now
	pos.Status = "OPEN"

	result, err := s.db.Exec(`
		INSERT INTO trader_positions (
			trader_id, exchange_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, leverage, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		pos.TraderID, pos.ExchangeID, pos.Symbol, pos.Side, pos.Quantity, pos.EntryPrice,
		pos.EntryOrderID, pos.EntryTime.Format(time.RFC3339), pos.Leverage,
		pos.Status, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create position record: %w", err)
	}

	id, _ := result.LastInsertId()
	pos.ID = id
	return nil
}

// ClosePosition closes position (updates position record)
func (s *PositionStore) ClosePosition(id int64, exitPrice float64, exitOrderID string, realizedPnL float64, fee float64, closeReason string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE trader_positions SET
			exit_price = ?, exit_order_id = ?, exit_time = ?,
			realized_pnl = ?, fee = ?, status = 'CLOSED',
			close_reason = ?, updated_at = ?
		WHERE id = ?
	`,
		exitPrice, exitOrderID, now.Format(time.RFC3339),
		realizedPnL, fee, closeReason, now.Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update position record: %w", err)
	}
	return nil
}

// GetOpenPositions gets all open positions
func (s *PositionStore) GetOpenPositions(traderID string) ([]*TraderPosition, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, exchange_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE trader_id = ? AND status = 'OPEN'
		ORDER BY entry_time DESC
	`, traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query open positions: %w", err)
	}
	defer rows.Close()

	return s.scanPositions(rows)
}

// GetOpenPositionBySymbol gets open position for specified symbol and direction
func (s *PositionStore) GetOpenPositionBySymbol(traderID, symbol, side string) (*TraderPosition, error) {
	var pos TraderPosition
	var entryTime, exitTime, createdAt, updatedAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, trader_id, exchange_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE trader_id = ? AND symbol = ? AND side = ? AND status = 'OPEN'
		ORDER BY entry_time DESC LIMIT 1
	`, traderID, symbol, side).Scan(
		&pos.ID, &pos.TraderID, &pos.ExchangeID, &pos.Symbol, &pos.Side, &pos.Quantity,
		&pos.EntryPrice, &pos.EntryOrderID, &entryTime, &pos.ExitPrice,
		&pos.ExitOrderID, &exitTime, &pos.RealizedPnL, &pos.Fee,
		&pos.Leverage, &pos.Status, &pos.CloseReason, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	s.parsePositionTimes(&pos, entryTime, exitTime, createdAt, updatedAt)
	return &pos, nil
}

// GetClosedPositions gets closed positions (historical records)
func (s *PositionStore) GetClosedPositions(traderID string, limit int) ([]*TraderPosition, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, exchange_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE trader_id = ? AND status = 'CLOSED'
		ORDER BY exit_time DESC
		LIMIT ?
	`, traderID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query closed positions: %w", err)
	}
	defer rows.Close()

	return s.scanPositions(rows)
}

// GetAllOpenPositions gets all traders' open positions (for global sync)
func (s *PositionStore) GetAllOpenPositions() ([]*TraderPosition, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, exchange_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE status = 'OPEN'
		ORDER BY trader_id, entry_time DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all open positions: %w", err)
	}
	defer rows.Close()

	return s.scanPositions(rows)
}

// GetPositionStats gets position statistics (simplified version)
func (s *PositionStore) GetPositionStats(traderID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total trades
	var totalTrades, winTrades int
	var totalPnL, totalFee float64

	err := s.db.QueryRow(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END) as wins,
			COALESCE(SUM(realized_pnl), 0) as total_pnl,
			COALESCE(SUM(fee), 0) as total_fee
		FROM trader_positions
		WHERE trader_id = ? AND status = 'CLOSED'
	`, traderID).Scan(&totalTrades, &winTrades, &totalPnL, &totalFee)
	if err != nil {
		return nil, err
	}

	stats["total_trades"] = totalTrades
	stats["win_trades"] = winTrades
	stats["total_pnl"] = totalPnL
	stats["total_fee"] = totalFee
	if totalTrades > 0 {
		stats["win_rate"] = float64(winTrades) / float64(totalTrades) * 100
	} else {
		stats["win_rate"] = 0.0
	}

	return stats, nil
}

// GetFullStats gets complete trading statistics (compatible with TraderStats)
func (s *PositionStore) GetFullStats(traderID string) (*TraderStats, error) {
	stats := &TraderStats{}

	// Query all closed positions
	rows, err := s.db.Query(`
		SELECT realized_pnl, fee, exit_time
		FROM trader_positions
		WHERE trader_id = ? AND status = 'CLOSED'
		ORDER BY exit_time ASC
	`, traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query position statistics: %w", err)
	}
	defer rows.Close()

	var pnls []float64
	var totalWin, totalLoss float64

	for rows.Next() {
		var pnl, fee float64
		var exitTime sql.NullString
		if err := rows.Scan(&pnl, &fee, &exitTime); err != nil {
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
			totalLoss += -pnl // Convert to positive
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

	// Calculate average profit/loss
	if stats.WinTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinTrades)
	}
	if stats.LossTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LossTrades)
	}

	// Calculate Sharpe ratio
	if len(pnls) > 1 {
		stats.SharpeRatio = calculateSharpeRatioFromPnls(pnls)
	}

	// Calculate maximum drawdown
	if len(pnls) > 0 {
		stats.MaxDrawdownPct = calculateMaxDrawdownFromPnls(pnls)
	}

	return stats, nil
}

// RecentTrade recent trade record (for AI input)
type RecentTrade struct {
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"` // long/short
	EntryPrice  float64 `json:"entry_price"`
	ExitPrice   float64 `json:"exit_price"`
	RealizedPnL float64 `json:"realized_pnl"`
	PnLPct      float64 `json:"pnl_pct"`
	ExitTime    string  `json:"exit_time"`
}

// GetRecentTrades gets recent closed trades
func (s *PositionStore) GetRecentTrades(traderID string, limit int) ([]RecentTrade, error) {
	rows, err := s.db.Query(`
		SELECT symbol, side, entry_price, exit_price, realized_pnl, leverage, exit_time
		FROM trader_positions
		WHERE trader_id = ? AND status = 'CLOSED'
		ORDER BY exit_time DESC
		LIMIT ?
	`, traderID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent trades: %w", err)
	}
	defer rows.Close()

	var trades []RecentTrade
	for rows.Next() {
		var t RecentTrade
		var leverage int
		var exitTime sql.NullString

		err := rows.Scan(&t.Symbol, &t.Side, &t.EntryPrice, &t.ExitPrice, &t.RealizedPnL, &leverage, &exitTime)
		if err != nil {
			continue
		}

		// Convert side format
		if t.Side == "LONG" {
			t.Side = "long"
		} else if t.Side == "SHORT" {
			t.Side = "short"
		}

		// Calculate profit/loss percentage
		if t.EntryPrice > 0 {
			if t.Side == "long" {
				t.PnLPct = (t.ExitPrice - t.EntryPrice) / t.EntryPrice * 100 * float64(leverage)
			} else {
				t.PnLPct = (t.EntryPrice - t.ExitPrice) / t.EntryPrice * 100 * float64(leverage)
			}
		}

		// Format time
		if exitTime.Valid {
			if parsed, err := time.Parse(time.RFC3339, exitTime.String); err == nil {
				t.ExitTime = parsed.Format("01-02 15:04")
			}
		}

		trades = append(trades, t)
	}

	return trades, nil
}

// calculateSharpeRatioFromPnls calculates Sharpe ratio
func calculateSharpeRatioFromPnls(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}

	var sum float64
	for _, pnl := range pnls {
		sum += pnl
	}
	mean := sum / float64(len(pnls))

	var variance float64
	for _, pnl := range pnls {
		variance += (pnl - mean) * (pnl - mean)
	}
	stdDev := math.Sqrt(variance / float64(len(pnls)-1))

	if stdDev == 0 {
		return 0
	}

	return mean / stdDev
}

// calculateMaxDrawdownFromPnls calculates maximum drawdown
func calculateMaxDrawdownFromPnls(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}

	var cumulative, peak, maxDD float64
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

// scanPositions scans position rows into structs
func (s *PositionStore) scanPositions(rows *sql.Rows) ([]*TraderPosition, error) {
	var positions []*TraderPosition
	for rows.Next() {
		var pos TraderPosition
		var entryTime, exitTime, createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&pos.ID, &pos.TraderID, &pos.ExchangeID, &pos.Symbol, &pos.Side, &pos.Quantity,
			&pos.EntryPrice, &pos.EntryOrderID, &entryTime, &pos.ExitPrice,
			&pos.ExitOrderID, &exitTime, &pos.RealizedPnL, &pos.Fee,
			&pos.Leverage, &pos.Status, &pos.CloseReason, &createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		s.parsePositionTimes(&pos, entryTime, exitTime, createdAt, updatedAt)
		positions = append(positions, &pos)
	}

	return positions, nil
}

// parsePositionTimes parses time fields
func (s *PositionStore) parsePositionTimes(pos *TraderPosition, entryTime, exitTime, createdAt, updatedAt sql.NullString) {
	if entryTime.Valid {
		pos.EntryTime, _ = time.Parse(time.RFC3339, entryTime.String)
	}
	if exitTime.Valid {
		t, _ := time.Parse(time.RFC3339, exitTime.String)
		pos.ExitTime = &t
	}
	if createdAt.Valid {
		pos.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		pos.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
}
