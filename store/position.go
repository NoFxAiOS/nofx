package store

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// TraderPosition 仓位记录（完整的开平仓追踪）
type TraderPosition struct {
	ID           int64      `json:"id"`
	TraderID     string     `json:"trader_id"`
	Symbol       string     `json:"symbol"`
	Side         string     `json:"side"`           // LONG/SHORT
	Quantity     float64    `json:"quantity"`       // 开仓数量
	EntryPrice   float64    `json:"entry_price"`    // 开仓均价
	EntryOrderID string     `json:"entry_order_id"` // 开仓订单ID
	EntryTime    time.Time  `json:"entry_time"`     // 开仓时间
	ExitPrice    float64    `json:"exit_price"`     // 平仓均价
	ExitOrderID  string     `json:"exit_order_id"`  // 平仓订单ID
	ExitTime     *time.Time `json:"exit_time"`      // 平仓时间
	RealizedPnL  float64    `json:"realized_pnl"`   // 已实现盈亏
	Fee          float64    `json:"fee"`            // 手续费
	Leverage     int        `json:"leverage"`       // 杠杆倍数
	Status       string     `json:"status"`         // OPEN/CLOSED
	CloseReason  string     `json:"close_reason"`   // 平仓原因: ai_decision/manual/stop_loss/take_profit
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PositionStore 仓位存储
type PositionStore struct {
	db *sql.DB
}

// NewPositionStore 创建仓位存储实例
func NewPositionStore(db *sql.DB) *PositionStore {
	return &PositionStore{db: db}
}

// InitTables 初始化仓位表
func (s *PositionStore) InitTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS trader_positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
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
		return fmt.Errorf("创建trader_positions表失败: %w", err)
	}

	// 创建索引
	indices := []string{
		`CREATE INDEX IF NOT EXISTS idx_positions_trader ON trader_positions(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_status ON trader_positions(trader_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_symbol ON trader_positions(trader_id, symbol, side, status)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_entry ON trader_positions(trader_id, entry_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_exit ON trader_positions(trader_id, exit_time DESC)`,
	}
	for _, idx := range indices {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	return nil
}

// Create 创建仓位记录（开仓时调用）
func (s *PositionStore) Create(pos *TraderPosition) error {
	now := time.Now()
	pos.CreatedAt = now
	pos.UpdatedAt = now
	pos.Status = "OPEN"

	result, err := s.db.Exec(`
		INSERT INTO trader_positions (
			trader_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, leverage, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		pos.TraderID, pos.Symbol, pos.Side, pos.Quantity, pos.EntryPrice,
		pos.EntryOrderID, pos.EntryTime.Format(time.RFC3339), pos.Leverage,
		pos.Status, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("创建仓位记录失败: %w", err)
	}

	id, _ := result.LastInsertId()
	pos.ID = id
	return nil
}

// ClosePosition 平仓（更新仓位记录）
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
		return fmt.Errorf("更新仓位记录失败: %w", err)
	}
	return nil
}

// GetOpenPositions 获取所有未平仓位
func (s *PositionStore) GetOpenPositions(traderID string) ([]*TraderPosition, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE trader_id = ? AND status = 'OPEN'
		ORDER BY entry_time DESC
	`, traderID)
	if err != nil {
		return nil, fmt.Errorf("查询未平仓位失败: %w", err)
	}
	defer rows.Close()

	return s.scanPositions(rows)
}

// GetOpenPositionBySymbol 获取指定币种方向的未平仓位
func (s *PositionStore) GetOpenPositionBySymbol(traderID, symbol, side string) (*TraderPosition, error) {
	var pos TraderPosition
	var entryTime, exitTime, createdAt, updatedAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, trader_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE trader_id = ? AND symbol = ? AND side = ? AND status = 'OPEN'
		ORDER BY entry_time DESC LIMIT 1
	`, traderID, symbol, side).Scan(
		&pos.ID, &pos.TraderID, &pos.Symbol, &pos.Side, &pos.Quantity,
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

// GetClosedPositions 获取已平仓位（历史记录）
func (s *PositionStore) GetClosedPositions(traderID string, limit int) ([]*TraderPosition, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE trader_id = ? AND status = 'CLOSED'
		ORDER BY exit_time DESC
		LIMIT ?
	`, traderID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询已平仓位失败: %w", err)
	}
	defer rows.Close()

	return s.scanPositions(rows)
}

// GetAllOpenPositions 获取所有trader的未平仓位（用于全局同步）
func (s *PositionStore) GetAllOpenPositions() ([]*TraderPosition, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, symbol, side, quantity, entry_price, entry_order_id,
			entry_time, exit_price, exit_order_id, exit_time, realized_pnl, fee,
			leverage, status, close_reason, created_at, updated_at
		FROM trader_positions
		WHERE status = 'OPEN'
		ORDER BY trader_id, entry_time DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("查询所有未平仓位失败: %w", err)
	}
	defer rows.Close()

	return s.scanPositions(rows)
}

// GetPositionStats 获取仓位统计（简单版）
func (s *PositionStore) GetPositionStats(traderID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总交易数
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

// GetFullStats 获取完整的交易统计（与 TraderStats 兼容）
func (s *PositionStore) GetFullStats(traderID string) (*TraderStats, error) {
	stats := &TraderStats{}

	// 查询所有已平仓位
	rows, err := s.db.Query(`
		SELECT realized_pnl, fee, exit_time
		FROM trader_positions
		WHERE trader_id = ? AND status = 'CLOSED'
		ORDER BY exit_time ASC
	`, traderID)
	if err != nil {
		return nil, fmt.Errorf("查询仓位统计失败: %w", err)
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
			totalLoss += -pnl // 转为正数
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

	// 计算夏普比
	if len(pnls) > 1 {
		stats.SharpeRatio = calculateSharpeRatioFromPnls(pnls)
	}

	// 计算最大回撤
	if len(pnls) > 0 {
		stats.MaxDrawdownPct = calculateMaxDrawdownFromPnls(pnls)
	}

	return stats, nil
}

// RecentTrade 最近的交易记录（用于AI输入）
type RecentTrade struct {
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"` // long/short
	EntryPrice  float64 `json:"entry_price"`
	ExitPrice   float64 `json:"exit_price"`
	RealizedPnL float64 `json:"realized_pnl"`
	PnLPct      float64 `json:"pnl_pct"`
	ExitTime    string  `json:"exit_time"`
}

// GetRecentTrades 获取最近的已平仓交易
func (s *PositionStore) GetRecentTrades(traderID string, limit int) ([]RecentTrade, error) {
	rows, err := s.db.Query(`
		SELECT symbol, side, entry_price, exit_price, realized_pnl, leverage, exit_time
		FROM trader_positions
		WHERE trader_id = ? AND status = 'CLOSED'
		ORDER BY exit_time DESC
		LIMIT ?
	`, traderID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询最近交易失败: %w", err)
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

		// 转换 side 格式
		if t.Side == "LONG" {
			t.Side = "long"
		} else if t.Side == "SHORT" {
			t.Side = "short"
		}

		// 计算盈亏百分比
		if t.EntryPrice > 0 {
			if t.Side == "long" {
				t.PnLPct = (t.ExitPrice - t.EntryPrice) / t.EntryPrice * 100 * float64(leverage)
			} else {
				t.PnLPct = (t.EntryPrice - t.ExitPrice) / t.EntryPrice * 100 * float64(leverage)
			}
		}

		// 格式化时间
		if exitTime.Valid {
			if parsed, err := time.Parse(time.RFC3339, exitTime.String); err == nil {
				t.ExitTime = parsed.Format("01-02 15:04")
			}
		}

		trades = append(trades, t)
	}

	return trades, nil
}

// calculateSharpeRatioFromPnls 计算夏普比
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

// calculateMaxDrawdownFromPnls 计算最大回撤
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

// scanPositions 扫描仓位行到结构体
func (s *PositionStore) scanPositions(rows *sql.Rows) ([]*TraderPosition, error) {
	var positions []*TraderPosition
	for rows.Next() {
		var pos TraderPosition
		var entryTime, exitTime, createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&pos.ID, &pos.TraderID, &pos.Symbol, &pos.Side, &pos.Quantity,
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

// parsePositionTimes 解析时间字段
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
