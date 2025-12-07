package store

import (
	"database/sql"
	"fmt"
	"time"
)

// EquityStore account equity storage (for plotting return curves)
type EquityStore struct {
	db *sql.DB
}

// EquitySnapshot equity snapshot
type EquitySnapshot struct {
	ID            int64     `json:"id"`
	TraderID      string    `json:"trader_id"`
	Timestamp     time.Time `json:"timestamp"`
	TotalEquity   float64   `json:"total_equity"`    // Account equity (balance + unrealized PnL)
	Balance       float64   `json:"balance"`         // Account balance
	UnrealizedPnL float64   `json:"unrealized_pnl"`  // Unrealized profit and loss
	PositionCount int       `json:"position_count"`  // Position count
	MarginUsedPct float64   `json:"margin_used_pct"` // Margin usage percentage
}

// initTables initializes equity tables
func (s *EquityStore) initTables() error {
	queries := []string{
		// Equity snapshot table - specifically for return curves
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
		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_equity_trader_time ON trader_equity_snapshots(trader_id, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_equity_timestamp ON trader_equity_snapshots(timestamp DESC)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute SQL: %w", err)
		}
	}

	return nil
}

// Save saves equity snapshot
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
		return fmt.Errorf("failed to save equity snapshot: %w", err)
	}

	id, _ := result.LastInsertId()
	snapshot.ID = id
	return nil
}

// GetLatest gets the latest N equity records for specified trader (sorted in ascending chronological order: old to new)
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
		return nil, fmt.Errorf("failed to query equity records: %w", err)
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

	// Reverse the array to sort time from old to new (suitable for plotting curves)
	for i, j := 0, len(snapshots)-1; i < j; i, j = i+1, j-1 {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	}

	return snapshots, nil
}

// GetByTimeRange gets equity records within specified time range
func (s *EquityStore) GetByTimeRange(traderID string, start, end time.Time) ([]*EquitySnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, trader_id, timestamp, total_equity, balance,
		       unrealized_pnl, position_count, margin_used_pct
		FROM trader_equity_snapshots
		WHERE trader_id = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`, traderID, start.Format(time.RFC3339), end.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to query equity records: %w", err)
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

// GetAllTradersLatest gets latest equity for all traders (for leaderboards)
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
		return nil, fmt.Errorf("failed to query latest equity: %w", err)
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

// CleanOldRecords cleans old records from N days ago
func (s *EquityStore) CleanOldRecords(traderID string, days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

	result, err := s.db.Exec(`
		DELETE FROM trader_equity_snapshots
		WHERE trader_id = ? AND timestamp < ?
	`, traderID, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to clean old records: %w", err)
	}

	return result.RowsAffected()
}

// GetCount gets record count for specified trader
func (s *EquityStore) GetCount(traderID string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM trader_equity_snapshots WHERE trader_id = ?
	`, traderID).Scan(&count)
	return count, err
}

// MigrateFromDecision migrates data from old decision_account_snapshots table
func (s *EquityStore) MigrateFromDecision() (int64, error) {
	// Check if migration is needed (whether new table is empty)
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM trader_equity_snapshots`).Scan(&count)
	if count > 0 {
		return 0, nil // Already has data, skip migration
	}

	// Check if old table exists
	var tableName string
	err := s.db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='decision_account_snapshots'
	`).Scan(&tableName)
	if err != nil {
		return 0, nil // Old table doesn't exist, skip
	}

	// Migrate data: join query from decision_records + decision_account_snapshots
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
		return 0, fmt.Errorf("failed to migrate data: %w", err)
	}

	return result.RowsAffected()
}
