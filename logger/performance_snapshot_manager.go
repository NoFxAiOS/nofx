package logger

import (
	"fmt"
	"log"
	"nofx/database"
	"nofx/decision/analysis"
	"time"
)

// PerformanceSnapshotManager 性能快照管理器
type PerformanceSnapshotManager struct {
	db          *database.DatabaseImpl
	userID      int
	traderID    string
	lastSnapshot *database.PerformanceSnapshot
}

// NewPerformanceSnapshotManager 创建性能快照管理器
func NewPerformanceSnapshotManager(db *database.DatabaseImpl, userID int, traderID string) *PerformanceSnapshotManager {
	return &PerformanceSnapshotManager{
		db:       db,
		userID:   userID,
		traderID: traderID,
	}
}

// SaveSnapshot 保存性能快照（通常每日调用）
func (psm *PerformanceSnapshotManager) SaveSnapshot(analysisResult *analysis.TradeAnalysisResult) error {
	if psm.db == nil || analysisResult == nil {
		return fmt.Errorf("数据库或分析结果为空")
	}

	snapshot := &database.PerformanceSnapshot{
		UserID:              psm.userID,
		TraderID:            psm.traderID,
		SnapshotDate:        time.Now(),
		TotalTrades:         analysisResult.TotalTrades,
		WinningTrades:       analysisResult.WinningTrades,
		LosingTrades:        analysisResult.LosingTrades,
		WinRate:             analysisResult.WinRate,
		SharpeRatio:         analysisResult.SharpeRatio,
		MaxDrawdownPct:      analysisResult.MaxDrawdownPercent,
		ConsecutiveLosses:   analysisResult.ConsecutiveLosses,
		MaxConsecutiveLoss:  analysisResult.MaxConsecutiveLoss,
		Volatility:          analysisResult.Volatility,
		WeightedWinRate:     analysisResult.WeightedWinRate,
		ProfitFactor:        analysisResult.ProfitFactor,
		AvgProfitPerWin:     analysisResult.AverageProfitPerWin,
		AvgLossPerLoss:      analysisResult.AverageLossPerLoss,
		BestPerformingPair:  analysisResult.BestPerformingPair,
		WorstPerformingPair: analysisResult.WorstPerformingPair,
		BestTradingHour:     analysisResult.BestTradingHour,
		TotalPnL:            0, // 需要从trader获取
		TotalPnLPct:         0, // 需要从trader获取
		Equity:              0, // 需要从trader获取
	}

	if err := psm.db.SavePerformanceSnapshot(snapshot); err != nil {
		return fmt.Errorf("保存性能快照失败: %w", err)
	}

	psm.lastSnapshot = snapshot
	log.Printf("✅ [%s] 性能快照已保存: %d笔交易，胜率%.1f%%，Sharpe%.2f",
		psm.traderID, snapshot.TotalTrades, snapshot.WinRate, snapshot.SharpeRatio)

	return nil
}

// SaveSymbolStats 保存币种性能统计
func (psm *PerformanceSnapshotManager) SaveSymbolStats(analysisResult *analysis.TradeAnalysisResult) error {
	if psm.db == nil || analysisResult == nil {
		return fmt.Errorf("数据库或分析结果为空")
	}

	if analysisResult.SymbolStats == nil || len(analysisResult.SymbolStats) == 0 {
		return nil // 没有币种统计数据
	}

	var records []*database.SymbolPerformanceRecord
	snapshotDate := time.Now()

	for symbol, stats := range analysisResult.SymbolStats {
		record := &database.SymbolPerformanceRecord{
			UserID:         psm.userID,
			TraderID:       psm.traderID,
			Symbol:         symbol,
			SnapshotDate:   snapshotDate,
			TradesCount:    stats.TradesCount,
			WinRate:        stats.WinRate,
			AvgProfitPct:   stats.AvgProfitPct,
			AvgLossPct:     stats.AvgLossPct,
			BestTradePct:   stats.BestTradePct,
			WorstTradePct:  stats.WorstTradePct,
			Volatility:     stats.Volatility,
			MaxDrawdownPct: stats.MaxDrawdownPct,
			ProfitFactor:   stats.ProfitFactor,
			TotalProfit:    0, // 从stats中计算
			TotalLoss:      0, // 从stats中计算
		}

		records = append(records, record)
	}

	if err := psm.db.SaveBatchSymbolPerformance(records); err != nil {
		return fmt.Errorf("批量保存币种性能统计失败: %w", err)
	}

	log.Printf("✅ [%s] 已保存 %d 个币种的性能统计", psm.traderID, len(records))
	return nil
}

// GetLatestSnapshot 获取最新快照
func (psm *PerformanceSnapshotManager) GetLatestSnapshot() (*database.PerformanceSnapshot, error) {
	if psm.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	return psm.db.GetLatestPerformanceSnapshot(psm.traderID)
}

// GetPerformanceTrend 获取性能趋势（最近N天）
func (psm *PerformanceSnapshotManager) GetPerformanceTrend(days int) ([]*database.PerformanceSnapshot, error) {
	if psm.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	return psm.db.GetPerformanceSnapshotsByDateRange(psm.traderID, startDate, endDate)
}

// CalculateTrendingMetrics 计算趋势性能指标
func (psm *PerformanceSnapshotManager) CalculateTrendingMetrics(days int) (map[string]interface{}, error) {
	snapshots, err := psm.GetPerformanceTrend(days)
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("没有性能快照数据")
	}

	// 计算平均值、最大值、最小值
	var avgWinRate, avgSharpe, avgMaxDD, avgVol float64
	var minSharpe, maxSharpe float64 = 999, -999
	var totalTrades int

	for _, snap := range snapshots {
		avgWinRate += snap.WinRate
		avgSharpe += snap.SharpeRatio
		avgMaxDD += snap.MaxDrawdownPct
		avgVol += snap.Volatility
		totalTrades += snap.TotalTrades

		if snap.SharpeRatio < minSharpe {
			minSharpe = snap.SharpeRatio
		}
		if snap.SharpeRatio > maxSharpe {
			maxSharpe = snap.SharpeRatio
		}
	}

	n := float64(len(snapshots))
	avgWinRate /= n
	avgSharpe /= n
	avgMaxDD /= n
	avgVol /= n

	// 计算趋势方向（比较最新和最旧）
	trend := "stable"
	if len(snapshots) > 1 {
		latest := snapshots[0]
		oldest := snapshots[len(snapshots)-1]

		if latest.SharpeRatio > oldest.SharpeRatio {
			trend = "improving"
		} else if latest.SharpeRatio < oldest.SharpeRatio {
			trend = "declining"
		}
	}

	return map[string]interface{}{
		"period_days":       days,
		"snapshots_count":   len(snapshots),
		"total_trades":      totalTrades,
		"avg_win_rate":      fmt.Sprintf("%.1f%%", avgWinRate),
		"avg_sharpe_ratio":  fmt.Sprintf("%.2f", avgSharpe),
		"sharpe_range":      fmt.Sprintf("%.2f - %.2f", minSharpe, maxSharpe),
		"avg_max_drawdown":  fmt.Sprintf("%.2f%%", avgMaxDD),
		"avg_volatility":    fmt.Sprintf("%.2f%%", avgVol),
		"trend":             trend,
		"recommendation": getTrendRecommendation(avgSharpe, avgWinRate, avgMaxDD),
	}, nil
}

// getTrendRecommendation 根据趋势指标获取建议
func getTrendRecommendation(sharpe, winRate, maxDD float64) string {
	if sharpe > 2.0 && winRate > 60 && maxDD < 15 {
		return "✅ 表现优秀，可增加杠杆或仓位"
	} else if sharpe > 1.0 && winRate > 50 {
		return "✅ 表现良好，继续保持当前策略"
	} else if sharpe < 0 && winRate < 40 {
		return "⚠️ 表现不佳，建议降低杠杆或暂停交易"
	} else if maxDD > 25 {
		return "⚠️ 回撤过大，需加强风险控制"
	}
	return "➡️ 表现平稳，监控后续表现"
}

// GetSymbolPerformanceTrend 获取单个币种的性能趋势
func (psm *PerformanceSnapshotManager) GetSymbolPerformanceTrend(symbol string, limit int) ([]*database.SymbolPerformanceRecord, error) {
	if psm.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	return psm.db.GetSymbolPerformanceTrend(psm.traderID, symbol, limit)
}

// GetBestSymbols 获取表现最好的币种
func (psm *PerformanceSnapshotManager) GetBestSymbols(snapshotDate time.Time, limit int) ([]*database.SymbolPerformanceRecord, error) {
	if psm.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	return psm.db.GetBestPerformingSymbols(psm.traderID, snapshotDate, limit)
}

// GetPerformanceComparisonOverTime 获取时间范围内的性能对比
func (psm *PerformanceSnapshotManager) GetPerformanceComparisonOverTime(startDate, endDate time.Time) (map[string]interface{}, error) {
	snapshots, err := psm.db.GetPerformanceSnapshotsByDateRange(psm.traderID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	if len(snapshots) < 2 {
		return nil, fmt.Errorf("数据点不足，无法进行比较")
	}

	first := snapshots[len(snapshots)-1]
	last := snapshots[0]

	comparison := map[string]interface{}{
		"period": fmt.Sprintf("%s 到 %s", first.SnapshotDate.Format("2006-01-02"), last.SnapshotDate.Format("2006-01-02")),
		"start_metrics": map[string]interface{}{
			"total_trades":  first.TotalTrades,
			"win_rate":      fmt.Sprintf("%.1f%%", first.WinRate),
			"sharpe_ratio":  fmt.Sprintf("%.2f", first.SharpeRatio),
			"max_drawdown":  fmt.Sprintf("%.2f%%", first.MaxDrawdownPct),
		},
		"end_metrics": map[string]interface{}{
			"total_trades":  last.TotalTrades,
			"win_rate":      fmt.Sprintf("%.1f%%", last.WinRate),
			"sharpe_ratio":  fmt.Sprintf("%.2f", last.SharpeRatio),
			"max_drawdown":  fmt.Sprintf("%.2f%%", last.MaxDrawdownPct),
		},
		"changes": map[string]interface{}{
			"trades_added":      last.TotalTrades - first.TotalTrades,
			"win_rate_change":   fmt.Sprintf("%+.1f%%", last.WinRate-first.WinRate),
			"sharpe_change":     fmt.Sprintf("%+.2f", last.SharpeRatio-first.SharpeRatio),
			"drawdown_change":   fmt.Sprintf("%+.2f%%", last.MaxDrawdownPct-first.MaxDrawdownPct),
		},
	}

	return comparison, nil
}
