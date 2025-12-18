package database

import (
	"fmt"
	"strings"
	"time"
)

// SymbolPerformanceRecord 币种性能记录
type SymbolPerformanceRecord struct {
	ID              int
	UserID          int
	TraderID        string
	Symbol          string
	SnapshotDate    time.Time
	TradesCount     int
	WinRate         float64
	AvgProfitPct    float64
	AvgLossPct      float64
	BestTradePct    float64
	WorstTradePct   float64
	Volatility      float64
	MaxDrawdownPct  float64
	ProfitFactor    float64
	TotalProfit     float64
	TotalLoss       float64
	CreatedAt       time.Time
}

// SaveSymbolPerformance 保存币种性能记录
func (db *DatabaseImpl) SaveSymbolPerformance(record *SymbolPerformanceRecord) error {
	query := `INSERT INTO symbol_performance (
		user_id, trader_id, symbol, snapshot_date, trades_count, win_rate,
		avg_profit_pct, avg_loss_pct, best_trade_pct, worst_trade_pct,
		volatility, max_drawdown_pct, profit_factor, total_profit, total_loss
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	ON CONFLICT (trader_id, symbol, snapshot_date) DO UPDATE SET
		trades_count = EXCLUDED.trades_count,
		win_rate = EXCLUDED.win_rate,
		avg_profit_pct = EXCLUDED.avg_profit_pct,
		avg_loss_pct = EXCLUDED.avg_loss_pct,
		best_trade_pct = EXCLUDED.best_trade_pct,
		worst_trade_pct = EXCLUDED.worst_trade_pct,
		volatility = EXCLUDED.volatility,
		max_drawdown_pct = EXCLUDED.max_drawdown_pct,
		profit_factor = EXCLUDED.profit_factor,
		total_profit = EXCLUDED.total_profit,
		total_loss = EXCLUDED.total_loss`

	if !db.usingNeon {
		query = `INSERT OR REPLACE INTO symbol_performance (
			user_id, trader_id, symbol, snapshot_date, trades_count, win_rate,
			avg_profit_pct, avg_loss_pct, best_trade_pct, worst_trade_pct,
			volatility, max_drawdown_pct, profit_factor, total_profit, total_loss
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	}

	_, err := db.currentDB.Exec(query,
		record.UserID, record.TraderID, record.Symbol, record.SnapshotDate,
		record.TradesCount, record.WinRate,
		record.AvgProfitPct, record.AvgLossPct, record.BestTradePct, record.WorstTradePct,
		record.Volatility, record.MaxDrawdownPct, record.ProfitFactor,
		record.TotalProfit, record.TotalLoss)

	return err
}

// SaveBatchSymbolPerformance 批量保存币种性能记录
func (db *DatabaseImpl) SaveBatchSymbolPerformance(records []*SymbolPerformanceRecord) error {
	if len(records) == 0 {
		return nil
	}

	for _, record := range records {
		if err := db.SaveSymbolPerformance(record); err != nil {
			return fmt.Errorf("保存币种性能记录失败 (%s): %w", record.Symbol, err)
		}
	}

	return nil
}

// GetSymbolPerformanceByDate 获取特定日期的币种性能数据
func (db *DatabaseImpl) GetSymbolPerformanceByDate(traderID string, snapshotDate time.Time) ([]*SymbolPerformanceRecord, error) {
	query := `SELECT id, user_id, trader_id, symbol, snapshot_date, trades_count, win_rate,
		avg_profit_pct, avg_loss_pct, best_trade_pct, worst_trade_pct,
		volatility, max_drawdown_pct, profit_factor, total_profit, total_loss, created_at
	FROM symbol_performance
	WHERE trader_id = $1 AND snapshot_date = $2
	ORDER BY profit_factor DESC`

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
	}

	rows, err := db.currentDB.Query(query, traderID, snapshotDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*SymbolPerformanceRecord
	for rows.Next() {
		record := &SymbolPerformanceRecord{}
		if err := rows.Scan(
			&record.ID, &record.UserID, &record.TraderID, &record.Symbol, &record.SnapshotDate,
			&record.TradesCount, &record.WinRate,
			&record.AvgProfitPct, &record.AvgLossPct, &record.BestTradePct, &record.WorstTradePct,
			&record.Volatility, &record.MaxDrawdownPct, &record.ProfitFactor,
			&record.TotalProfit, &record.TotalLoss, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// GetSymbolPerformanceTrend 获取单个币种的性能趋势
func (db *DatabaseImpl) GetSymbolPerformanceTrend(traderID, symbol string, limit int) ([]*SymbolPerformanceRecord, error) {
	query := fmt.Sprintf(`SELECT id, user_id, trader_id, symbol, snapshot_date, trades_count, win_rate,
		avg_profit_pct, avg_loss_pct, best_trade_pct, worst_trade_pct,
		volatility, max_drawdown_pct, profit_factor, total_profit, total_loss, created_at
	FROM symbol_performance
	WHERE trader_id = $1 AND symbol = $2
	ORDER BY snapshot_date DESC
	LIMIT %d`, limit)

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
	}

	rows, err := db.currentDB.Query(query, traderID, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*SymbolPerformanceRecord
	for rows.Next() {
		record := &SymbolPerformanceRecord{}
		if err := rows.Scan(
			&record.ID, &record.UserID, &record.TraderID, &record.Symbol, &record.SnapshotDate,
			&record.TradesCount, &record.WinRate,
			&record.AvgProfitPct, &record.AvgLossPct, &record.BestTradePct, &record.WorstTradePct,
			&record.Volatility, &record.MaxDrawdownPct, &record.ProfitFactor,
			&record.TotalProfit, &record.TotalLoss, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// GetBestPerformingSymbols 获取表现最好的币种（按profit_factor排序）
func (db *DatabaseImpl) GetBestPerformingSymbols(traderID string, snapshotDate time.Time, limit int) ([]*SymbolPerformanceRecord, error) {
	query := fmt.Sprintf(`SELECT id, user_id, trader_id, symbol, snapshot_date, trades_count, win_rate,
		avg_profit_pct, avg_loss_pct, best_trade_pct, worst_trade_pct,
		volatility, max_drawdown_pct, profit_factor, total_profit, total_loss, created_at
	FROM symbol_performance
	WHERE trader_id = $1 AND snapshot_date = $2
	ORDER BY profit_factor DESC
	LIMIT %d`, limit)

	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
	}

	rows, err := db.currentDB.Query(query, traderID, snapshotDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*SymbolPerformanceRecord
	for rows.Next() {
		record := &SymbolPerformanceRecord{}
		if err := rows.Scan(
			&record.ID, &record.UserID, &record.TraderID, &record.Symbol, &record.SnapshotDate,
			&record.TradesCount, &record.WinRate,
			&record.AvgProfitPct, &record.AvgLossPct, &record.BestTradePct, &record.WorstTradePct,
			&record.Volatility, &record.MaxDrawdownPct, &record.ProfitFactor,
			&record.TotalProfit, &record.TotalLoss, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}
