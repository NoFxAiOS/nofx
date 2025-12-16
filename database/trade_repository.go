package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// TradeRecord 交易记录结构体
type TradeRecord struct {
	ID                 int64
	TraderID           string
	Symbol             string
	EntryPrice         float64
	ExitPrice          float64
	ProfitPct          float64
	Leverage           int
	HoldingTimeSeconds int64
	MarginMode         string
	CreatedAt          time.Time
}

// TradeRepository 交易记录数据库操作
type TradeRepository struct {
	db *sql.DB
}

// NewTradeRepository 创建交易记录repository
func NewTradeRepository(db *sql.DB) *TradeRepository {
	return &TradeRepository{db: db}
}

// InsertTradeRecord 插入交易记录
func (tr *TradeRepository) InsertTradeRecord(record TradeRecord) error {
	query := `
		INSERT INTO trade_records
		(trader_id, symbol, entry_price, exit_price, profit_pct, leverage, holding_time_seconds, margin_mode, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := tr.db.Exec(
		query,
		record.TraderID,
		record.Symbol,
		record.EntryPrice,
		record.ExitPrice,
		record.ProfitPct,
		record.Leverage,
		record.HoldingTimeSeconds,
		record.MarginMode,
		time.Now(),
	)

	if err != nil {
		log.Printf("❌ 插入交易记录失败: %v", err)
		return fmt.Errorf("插入交易记录失败: %w", err)
	}

	return nil
}

// LoadRecentTradesForTrader 加载指定trader最近的N笔交易
func (tr *TradeRepository) LoadRecentTradesForTrader(traderID string, limit int) ([]TradeRecord, error) {
	query := `
		SELECT id, trader_id, symbol, entry_price, exit_price, profit_pct, leverage, holding_time_seconds, margin_mode, created_at
		FROM trade_records
		WHERE trader_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := tr.db.Query(query, traderID, limit)
	if err != nil {
		log.Printf("❌ 查询交易记录失败: %v", err)
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer rows.Close()

	var records []TradeRecord
	for rows.Next() {
		var r TradeRecord
		err := rows.Scan(
			&r.ID, &r.TraderID, &r.Symbol, &r.EntryPrice, &r.ExitPrice,
			&r.ProfitPct, &r.Leverage, &r.HoldingTimeSeconds, &r.MarginMode, &r.CreatedAt,
		)
		if err != nil {
			log.Printf("❌ 扫描交易记录失败: %v", err)
			return nil, fmt.Errorf("扫描交易记录失败: %w", err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		log.Printf("❌ 遍历结果集失败: %v", err)
		return nil, fmt.Errorf("遍历结果集失败: %w", err)
	}

	return records, nil
}

// LoadTradesForSymbol 加载指定币种的交易记录
func (tr *TradeRepository) LoadTradesForSymbol(traderID, symbol string, limit int) ([]TradeRecord, error) {
	query := `
		SELECT id, trader_id, symbol, entry_price, exit_price, profit_pct, leverage, holding_time_seconds, margin_mode, created_at
		FROM trade_records
		WHERE trader_id = $1 AND symbol = $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := tr.db.Query(query, traderID, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer rows.Close()

	var records []TradeRecord
	for rows.Next() {
		var r TradeRecord
		err := rows.Scan(
			&r.ID, &r.TraderID, &r.Symbol, &r.EntryPrice, &r.ExitPrice,
			&r.ProfitPct, &r.Leverage, &r.HoldingTimeSeconds, &r.MarginMode, &r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描交易记录失败: %w", err)
		}
		records = append(records, r)
	}

	return records, nil
}

// DeleteOldTradeRecords 删除超过指定天数的交易记录 (定期清理)
func (tr *TradeRepository) DeleteOldTradeRecords(daysOld int) error {
	query := `
		DELETE FROM trade_records
		WHERE created_at < NOW() - INTERVAL '1 day' * $1
	`

	result, err := tr.db.Exec(query, daysOld)
	if err != nil {
		return fmt.Errorf("删除旧记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取受影响行数失败: %w", err)
	}

	log.Printf("✓ 删除了%d条超过%d天的交易记录", rowsAffected, daysOld)
	return nil
}

// GetTradesInPeriod 获取指定时间段内的交易记录
func (tr *TradeRepository) GetTradesInPeriod(traderID string, startDate, endDate time.Time) ([]TradeRecord, error) {
	query := `
		SELECT id, trader_id, symbol, entry_price, exit_price, profit_pct, leverage, holding_time_seconds, margin_mode, created_at
		FROM trade_records
		WHERE trader_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at ASC
	`

	rows, err := tr.db.Query(query, traderID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("查询时间段内交易记录失败: %w", err)
	}
	defer rows.Close()

	var records []TradeRecord
	for rows.Next() {
		var r TradeRecord
		err := rows.Scan(
			&r.ID, &r.TraderID, &r.Symbol, &r.EntryPrice, &r.ExitPrice,
			&r.ProfitPct, &r.Leverage, &r.HoldingTimeSeconds, &r.MarginMode, &r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描交易记录失败: %w", err)
		}
		records = append(records, r)
	}

	return records, nil
}

// KellyStats Kelly统计数据
type KellyStats struct {
	ID              int64
	TraderID        string
	Symbol          string
	TotalTrades     int
	ProfitableTrades int
	WinRate         float64
	AvgWinPct       float64
	AvgLossPct      float64
	MaxProfitPct    float64
	MaxDrawdownPct  float64
	Volatility      float64
	WeightedWinRate float64
	UpdatedAt       time.Time
}

// KellyStatsRepository Kelly统计数据库操作
type KellyStatsRepository struct {
	db *sql.DB
}

// NewKellyStatsRepository 创建Kelly统计repository
func NewKellyStatsRepository(db *sql.DB) *KellyStatsRepository {
	return &KellyStatsRepository{db: db}
}

// SaveKellyStats 保存或更新Kelly统计
func (kr *KellyStatsRepository) SaveKellyStats(stats KellyStats) error {
	query := `
		INSERT INTO kelly_stats
		(trader_id, symbol, total_trades, profitable_trades, win_rate, avg_win_pct, avg_loss_pct, max_profit_pct, max_drawdown_pct, volatility, weighted_win_rate, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (trader_id, symbol) DO UPDATE SET
		total_trades = EXCLUDED.total_trades,
		profitable_trades = EXCLUDED.profitable_trades,
		win_rate = EXCLUDED.win_rate,
		avg_win_pct = EXCLUDED.avg_win_pct,
		avg_loss_pct = EXCLUDED.avg_loss_pct,
		max_profit_pct = EXCLUDED.max_profit_pct,
		max_drawdown_pct = EXCLUDED.max_drawdown_pct,
		volatility = EXCLUDED.volatility,
		weighted_win_rate = EXCLUDED.weighted_win_rate,
		updated_at = NOW()
	`

	_, err := kr.db.Exec(
		query,
		stats.TraderID,
		stats.Symbol,
		stats.TotalTrades,
		stats.ProfitableTrades,
		stats.WinRate,
		stats.AvgWinPct,
		stats.AvgLossPct,
		stats.MaxProfitPct,
		stats.MaxDrawdownPct,
		stats.Volatility,
		stats.WeightedWinRate,
		time.Now(),
	)

	if err != nil {
		log.Printf("❌ 保存Kelly统计失败: %v", err)
		return fmt.Errorf("保存Kelly统计失败: %w", err)
	}

	return nil
}

// GetKellyStats 获取指定币种的Kelly统计
func (kr *KellyStatsRepository) GetKellyStats(traderID, symbol string) (*KellyStats, error) {
	query := `
		SELECT id, trader_id, symbol, total_trades, profitable_trades, win_rate, avg_win_pct, avg_loss_pct,
			   max_profit_pct, max_drawdown_pct, volatility, weighted_win_rate, updated_at
		FROM kelly_stats
		WHERE trader_id = $1 AND symbol = $2
	`

	var stats KellyStats
	err := kr.db.QueryRow(query, traderID, symbol).Scan(
		&stats.ID, &stats.TraderID, &stats.Symbol, &stats.TotalTrades, &stats.ProfitableTrades,
		&stats.WinRate, &stats.AvgWinPct, &stats.AvgLossPct, &stats.MaxProfitPct, &stats.MaxDrawdownPct,
		&stats.Volatility, &stats.WeightedWinRate, &stats.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // 不存在数据，返回nil而不是错误
	}

	if err != nil {
		return nil, fmt.Errorf("查询Kelly统计失败: %w", err)
	}

	return &stats, nil
}

// GetAllKellyStatsForTrader 获取指定trader的所有币种Kelly统计
func (kr *KellyStatsRepository) GetAllKellyStatsForTrader(traderID string) (map[string]*KellyStats, error) {
	query := `
		SELECT id, trader_id, symbol, total_trades, profitable_trades, win_rate, avg_win_pct, avg_loss_pct,
			   max_profit_pct, max_drawdown_pct, volatility, weighted_win_rate, updated_at
		FROM kelly_stats
		WHERE trader_id = $1
	`

	rows, err := kr.db.Query(query, traderID)
	if err != nil {
		return nil, fmt.Errorf("查询Kelly统计失败: %w", err)
	}
	defer rows.Close()

	statsMap := make(map[string]*KellyStats)
	for rows.Next() {
		var stats KellyStats
		err := rows.Scan(
			&stats.ID, &stats.TraderID, &stats.Symbol, &stats.TotalTrades, &stats.ProfitableTrades,
			&stats.WinRate, &stats.AvgWinPct, &stats.AvgLossPct, &stats.MaxProfitPct, &stats.MaxDrawdownPct,
			&stats.Volatility, &stats.WeightedWinRate, &stats.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描Kelly统计失败: %w", err)
		}
		statsMap[stats.Symbol] = &stats
	}

	return statsMap, nil
}

// BatchInsertTradeRecords 批量插入交易记录 (提高性能)
func (tr *TradeRepository) BatchInsertTradeRecords(records []TradeRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := tr.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO trade_records
		(trader_id, symbol, entry_price, exit_price, profit_pct, leverage, holding_time_seconds, margin_mode, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	for _, record := range records {
		_, err := stmt.Exec(
			record.TraderID,
			record.Symbol,
			record.EntryPrice,
			record.ExitPrice,
			record.ProfitPct,
			record.Leverage,
			record.HoldingTimeSeconds,
			record.MarginMode,
			time.Now(),
		)
		if err != nil {
			return fmt.Errorf("执行批量插入失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("✓ 批量插入%d条交易记录成功", len(records))
	return nil
}
