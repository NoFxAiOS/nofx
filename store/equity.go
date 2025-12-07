package store

import (
	"database/sql"
	"fmt"
	"time"
)

// EquityStore 账户净值存储（用于绘制收益率曲线）
type EquityStore struct {
	db *sql.DB
}

// EquitySnapshot 净值快照
type EquitySnapshot struct {
	ID            int64     `json:"id"`
	TraderID      string    `json:"trader_id"`
	Timestamp     time.Time `json:"timestamp"`
	TotalEquity   float64   `json:"total_equity"`    // 账户净值 (余额 + 未实现盈亏)
	Balance       float64   `json:"balance"`         // 账户余额
	UnrealizedPnL float64   `json:"unrealized_pnl"`  // 未实现盈亏
	PositionCount int       `json:"position_count"`  // 持仓数量
	MarginUsedPct float64   `json:"margin_used_pct"` // 保证金使用率
}

// initTables 初始化净值表
func (s *EquityStore) initTables() error {
	queries := []string{
		// 净值快照表 - 专门用于收益率曲线
		`CREATE TABLE IF NOT EXISTS trader_equity_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			total_equity REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			unrealized_pnl REAL NOT NULL DEFAULT 0,
			position_count INTEGER DEFAULT 0,
			margin_used_pct REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 索引
		`CREATE INDEX IF NOT EXISTS idx_equity_trader_time ON trader_equity_snapshots(trader_id, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_equity_timestamp ON trader_equity_snapshots(timestamp DESC)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败: %w", err)
		}
	}

	return nil
}

// Save 保存净值快照
func (s *EquityStore) Save(snapshot *EquitySnapshot) error {
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now().UTC()
	} else {
		snapshot.Timestamp = snapshot.Timestamp.UTC()
	}

	result, err := s.db.Exec(`
		INSERT INTO trader_equity_snapshots (
			trader_id, timestamp, total_equity, balance,
			unrealized_pnl, position_count, margin_used_pct
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		snapshot.TraderID,
		snapshot.Timestamp.Format(time.RFC3339),
		snapshot.TotalEquity,
		snapshot.Balance,
		snapshot.UnrealizedPnL,
		snapshot.PositionCount,
		snapshot.MarginUsedPct,
	)
	if err != nil {
		return fmt.Errorf("保存净值快照失败: %w", err)
	}

	id, _ := result.LastInsertId()
	snapshot.ID = id
	return nil
}

// GetLatest 获取指定交易员最近N条净值记录（按时间正序：从旧到新）
func (s *EquityStore) GetLatest(traderID string, limit int) ([]*EquitySnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, timestamp, total_equity, balance,
		       unrealized_pnl, position_count, margin_used_pct
		FROM trader_equity_snapshots
		WHERE trader_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, traderID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询净值记录失败: %w", err)
	}
	defer rows.Close()

	var snapshots []*EquitySnapshot
	for rows.Next() {
		snap := &EquitySnapshot{}
		var timestampStr string
		err := rows.Scan(
			&snap.ID, &snap.TraderID, &timestampStr, &snap.TotalEquity,
			&snap.Balance, &snap.UnrealizedPnL, &snap.PositionCount, &snap.MarginUsedPct,
		)
		if err != nil {
			continue
		}
		snap.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
		snapshots = append(snapshots, snap)
	}

	// 反转数组，让时间从旧到新排列（适合绘制曲线）
	for i, j := 0, len(snapshots)-1; i < j; i, j = i+1, j-1 {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	}

	return snapshots, nil
}

// GetByTimeRange 获取指定时间范围内的净值记录
func (s *EquityStore) GetByTimeRange(traderID string, start, end time.Time) ([]*EquitySnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, timestamp, total_equity, balance,
		       unrealized_pnl, position_count, margin_used_pct
		FROM trader_equity_snapshots
		WHERE trader_id = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`, traderID, start.Format(time.RFC3339), end.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("查询净值记录失败: %w", err)
	}
	defer rows.Close()

	var snapshots []*EquitySnapshot
	for rows.Next() {
		snap := &EquitySnapshot{}
		var timestampStr string
		err := rows.Scan(
			&snap.ID, &snap.TraderID, &timestampStr, &snap.TotalEquity,
			&snap.Balance, &snap.UnrealizedPnL, &snap.PositionCount, &snap.MarginUsedPct,
		)
		if err != nil {
			continue
		}
		snap.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// GetAllTradersLatest 获取所有交易员的最新净值（用于排行榜）
func (s *EquityStore) GetAllTradersLatest() (map[string]*EquitySnapshot, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.trader_id, e.timestamp, e.total_equity, e.balance,
		       e.unrealized_pnl, e.position_count, e.margin_used_pct
		FROM trader_equity_snapshots e
		INNER JOIN (
			SELECT trader_id, MAX(timestamp) as max_ts
			FROM trader_equity_snapshots
			GROUP BY trader_id
		) latest ON e.trader_id = latest.trader_id AND e.timestamp = latest.max_ts
	`)
	if err != nil {
		return nil, fmt.Errorf("查询最新净值失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*EquitySnapshot)
	for rows.Next() {
		snap := &EquitySnapshot{}
		var timestampStr string
		err := rows.Scan(
			&snap.ID, &snap.TraderID, &timestampStr, &snap.TotalEquity,
			&snap.Balance, &snap.UnrealizedPnL, &snap.PositionCount, &snap.MarginUsedPct,
		)
		if err != nil {
			continue
		}
		snap.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
		result[snap.TraderID] = snap
	}

	return result, nil
}

// CleanOldRecords 清理N天前的旧记录
func (s *EquityStore) CleanOldRecords(traderID string, days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

	result, err := s.db.Exec(`
		DELETE FROM trader_equity_snapshots
		WHERE trader_id = ? AND timestamp < ?
	`, traderID, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("清理旧记录失败: %w", err)
	}

	return result.RowsAffected()
}

// GetCount 获取指定交易员的记录数
func (s *EquityStore) GetCount(traderID string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM trader_equity_snapshots WHERE trader_id = ?
	`, traderID).Scan(&count)
	return count, err
}

// MigrateFromDecision 从旧的 decision_account_snapshots 迁移数据
func (s *EquityStore) MigrateFromDecision() (int64, error) {
	// 检查是否需要迁移（新表是否为空）
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM trader_equity_snapshots`).Scan(&count)
	if count > 0 {
		return 0, nil // 已有数据，跳过迁移
	}

	// 检查旧表是否存在
	var tableName string
	err := s.db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='decision_account_snapshots'
	`).Scan(&tableName)
	if err != nil {
		return 0, nil // 旧表不存在，跳过
	}

	// 迁移数据：从 decision_records + decision_account_snapshots 联合查询
	result, err := s.db.Exec(`
		INSERT INTO trader_equity_snapshots (
			trader_id, timestamp, total_equity, balance,
			unrealized_pnl, position_count, margin_used_pct
		)
		SELECT
			dr.trader_id,
			dr.timestamp,
			das.total_balance,
			das.available_balance,
			das.total_unrealized_profit,
			das.position_count,
			das.margin_used_pct
		FROM decision_records dr
		JOIN decision_account_snapshots das ON dr.id = das.decision_id
		ORDER BY dr.timestamp ASC
	`)
	if err != nil {
		return 0, fmt.Errorf("迁移数据失败: %w", err)
	}

	return result.RowsAffected()
}
