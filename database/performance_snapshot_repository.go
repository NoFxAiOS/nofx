package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PerformanceSnapshot 性能快照数据结构
type PerformanceSnapshot struct {
	ID                    int
	UserID                int
	TraderID              string
	SnapshotDate          time.Time
	TotalTrades           int
	WinningTrades         int
	LosingTrades          int
	WinRate               float64
	SharpeRatio           float64
	MaxDrawdownPct        float64
	ConsecutiveLosses     int
	MaxConsecutiveLoss    int
	Volatility            float64
	WeightedWinRate       float64
	ProfitFactor          float64
	AvgProfitPerWin       float64
	AvgLossPerLoss        float64
	BestPerformingPair    string
	WorstPerformingPair   string
	BestTradingHour       int
	TotalPnL              float64
	TotalPnLPct           float64
	Equity                float64
	CreatedAt             time.Time
}

// SavePerformanceSnapshot 保存性能快照
func (db *DatabaseImpl) SavePerformanceSnapshot(snapshot *PerformanceSnapshot) error {
	query := `INSERT INTO performance_snapshots (
		user_id, trader_id, snapshot_date, total_trades, winning_trades, losing_trades,
		win_rate, sharpe_ratio, max_drawdown_pct, consecutive_losses, max_consecutive_loss,
		volatility, weighted_win_rate, profit_factor, avg_profit_per_win, avg_loss_per_loss,
		best_performing_pair, worst_performing_pair, best_trading_hour, total_pnl, total_pnl_pct, equity
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	ON CONFLICT (trader_id, snapshot_date) DO UPDATE SET
		total_trades = EXCLUDED.total_trades,
		winning_trades = EXCLUDED.winning_trades,
		losing_trades = EXCLUDED.losing_trades,
		win_rate = EXCLUDED.win_rate,
		sharpe_ratio = EXCLUDED.sharpe_ratio,
		max_drawdown_pct = EXCLUDED.max_drawdown_pct,
		consecutive_losses = EXCLUDED.consecutive_losses,
		max_consecutive_loss = EXCLUDED.max_consecutive_loss,
		volatility = EXCLUDED.volatility,
		weighted_win_rate = EXCLUDED.weighted_win_rate,
		profit_factor = EXCLUDED.profit_factor,
		avg_profit_per_win = EXCLUDED.avg_profit_per_win,
		avg_loss_per_loss = EXCLUDED.avg_loss_per_loss,
		best_performing_pair = EXCLUDED.best_performing_pair,
		worst_performing_pair = EXCLUDED.worst_performing_pair,
		best_trading_hour = EXCLUDED.best_trading_hour,
		total_pnl = EXCLUDED.total_pnl,
		total_pnl_pct = EXCLUDED.total_pnl_pct,
		equity = EXCLUDED.equity`

	// SQLite不支持ON CONFLICT DO UPDATE的复杂形式，需要特殊处理
	if !db.usingNeon {
		query = `INSERT OR REPLACE INTO performance_snapshots (
			user_id, trader_id, snapshot_date, total_trades, winning_trades, losing_trades,
			win_rate, sharpe_ratio, max_drawdown_pct, consecutive_losses, max_consecutive_loss,
			volatility, weighted_win_rate, profit_factor, avg_profit_per_win, avg_loss_per_loss,
			best_performing_pair, worst_performing_pair, best_trading_hour, total_pnl, total_pnl_pct, equity
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		query = strings.ReplaceAll(query, "$", "?")
	}

	_, err := db.currentDB.Exec(query,
		snapshot.UserID, snapshot.TraderID, snapshot.SnapshotDate,
		snapshot.TotalTrades, snapshot.WinningTrades, snapshot.LosingTrades,
		snapshot.WinRate, snapshot.SharpeRatio, snapshot.MaxDrawdownPct,
		snapshot.ConsecutiveLosses, snapshot.MaxConsecutiveLoss,
		snapshot.Volatility, snapshot.WeightedWinRate, snapshot.ProfitFactor,
		snapshot.AvgProfitPerWin, snapshot.AvgLossPerLoss,
		snapshot.BestPerformingPair, snapshot.WorstPerformingPair, snapshot.BestTradingHour,
		snapshot.TotalPnL, snapshot.TotalPnLPct, snapshot.Equity)

	return err
}

// GetLatestPerformanceSnapshot 获取最新的性能快照
func (db *DatabaseImpl) GetLatestPerformanceSnapshot(traderID string) (*PerformanceSnapshot, error) {
	query := `SELECT id, user_id, trader_id, snapshot_date, total_trades, winning_trades, losing_trades,
		win_rate, sharpe_ratio, max_drawdown_pct, consecutive_losses, max_consecutive_loss,
		volatility, weighted_win_rate, profit_factor, avg_profit_per_win, avg_loss_per_loss,
		best_performing_pair, worst_performing_pair, best_trading_hour, total_pnl, total_pnl_pct, equity, created_at
	FROM performance_snapshots WHERE trader_id = $1 ORDER BY snapshot_date DESC LIMIT 1`

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
	}

	row := db.currentDB.QueryRow(query, traderID)

	snapshot := &PerformanceSnapshot{}
	err := row.Scan(
		&snapshot.ID, &snapshot.UserID, &snapshot.TraderID, &snapshot.SnapshotDate,
		&snapshot.TotalTrades, &snapshot.WinningTrades, &snapshot.LosingTrades,
		&snapshot.WinRate, &snapshot.SharpeRatio, &snapshot.MaxDrawdownPct,
		&snapshot.ConsecutiveLosses, &snapshot.MaxConsecutiveLoss,
		&snapshot.Volatility, &snapshot.WeightedWinRate, &snapshot.ProfitFactor,
		&snapshot.AvgProfitPerWin, &snapshot.AvgLossPerLoss,
		&snapshot.BestPerformingPair, &snapshot.WorstPerformingPair, &snapshot.BestTradingHour,
		&snapshot.TotalPnL, &snapshot.TotalPnLPct, &snapshot.Equity, &snapshot.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// GetPerformanceSnapshotsByDateRange 获取日期范围内的性能快照
func (db *DatabaseImpl) GetPerformanceSnapshotsByDateRange(traderID string, startDate, endDate time.Time) ([]*PerformanceSnapshot, error) {
	query := `SELECT id, user_id, trader_id, snapshot_date, total_trades, winning_trades, losing_trades,
		win_rate, sharpe_ratio, max_drawdown_pct, consecutive_losses, max_consecutive_loss,
		volatility, weighted_win_rate, profit_factor, avg_profit_per_win, avg_loss_per_loss,
		best_performing_pair, worst_performing_pair, best_trading_hour, total_pnl, total_pnl_pct, equity, created_at
	FROM performance_snapshots
	WHERE trader_id = $1 AND snapshot_date >= $2 AND snapshot_date <= $3
	ORDER BY snapshot_date DESC`

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
		query = strings.ReplaceAll(query, "$3", "?")
	}

	rows, err := db.currentDB.Query(query, traderID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []*PerformanceSnapshot
	for rows.Next() {
		snapshot := &PerformanceSnapshot{}
		if err := rows.Scan(
			&snapshot.ID, &snapshot.UserID, &snapshot.TraderID, &snapshot.SnapshotDate,
			&snapshot.TotalTrades, &snapshot.WinningTrades, &snapshot.LosingTrades,
			&snapshot.WinRate, &snapshot.SharpeRatio, &snapshot.MaxDrawdownPct,
			&snapshot.ConsecutiveLosses, &snapshot.MaxConsecutiveLoss,
			&snapshot.Volatility, &snapshot.WeightedWinRate, &snapshot.ProfitFactor,
			&snapshot.AvgProfitPerWin, &snapshot.AvgLossPerLoss,
			&snapshot.BestPerformingPair, &snapshot.WorstPerformingPair, &snapshot.BestTradingHour,
			&snapshot.TotalPnL, &snapshot.TotalPnLPct, &snapshot.Equity, &snapshot.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, rows.Err()
}

// GetPerformanceTrend 获取性能趋势（最近N个快照）
func (db *DatabaseImpl) GetPerformanceTrend(traderID string, limit int) ([]*PerformanceSnapshot, error) {
	query := fmt.Sprintf(`SELECT id, user_id, trader_id, snapshot_date, total_trades, winning_trades, losing_trades,
		win_rate, sharpe_ratio, max_drawdown_pct, consecutive_losses, max_consecutive_loss,
		volatility, weighted_win_rate, profit_factor, avg_profit_per_win, avg_loss_per_loss,
		best_performing_pair, worst_performing_pair, best_trading_hour, total_pnl, total_pnl_pct, equity, created_at
	FROM performance_snapshots
	WHERE trader_id = $1
	ORDER BY snapshot_date DESC
	LIMIT %d`, limit)

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
	}

	rows, err := db.currentDB.Query(query, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []*PerformanceSnapshot
	for rows.Next() {
		snapshot := &PerformanceSnapshot{}
		if err := rows.Scan(
			&snapshot.ID, &snapshot.UserID, &snapshot.TraderID, &snapshot.SnapshotDate,
			&snapshot.TotalTrades, &snapshot.WinningTrades, &snapshot.LosingTrades,
			&snapshot.WinRate, &snapshot.SharpeRatio, &snapshot.MaxDrawdownPct,
			&snapshot.ConsecutiveLosses, &snapshot.MaxConsecutiveLoss,
			&snapshot.Volatility, &snapshot.WeightedWinRate, &snapshot.ProfitFactor,
			&snapshot.AvgProfitPerWin, &snapshot.AvgLossPerLoss,
			&snapshot.BestPerformingPair, &snapshot.WorstPerformingPair, &snapshot.BestTradingHour,
			&snapshot.TotalPnL, &snapshot.TotalPnLPct, &snapshot.Equity, &snapshot.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, rows.Err()
}
