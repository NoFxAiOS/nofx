package analytics

import (
	"fmt"
	"sort"
	"time"
)

// PerformanceAttribution 绩效归因分析
type PerformanceAttribution struct {
	ByAsset          []AssetAttribution     `json:"by_asset"`
	ByStrategy       []StrategyAttribution  `json:"by_strategy"`
	ByTimeframe      []TimeframeAttribution `json:"by_timeframe"`
	Summary          *AttributionSummary    `json:"summary"`
	TopContributors  []AssetAttribution     `json:"top_contributors"`
	WorstPerformers  []AssetAttribution     `json:"worst_performers"`
	CalculatedAt     time.Time              `json:"calculated_at"`
}

// AssetAttribution 按资产归因
type AssetAttribution struct {
	Symbol          string  `json:"symbol"`
	TotalPnL        float64 `json:"total_pnl"`
	TotalPnLPercent float64 `json:"total_pnl_percent"`
	WinRate         float64 `json:"win_rate"`
	TradesCount     int     `json:"trades_count"`
	AvgTradeReturn  float64 `json:"avg_trade_return"`
	BestTrade       float64 `json:"best_trade"`
	WorstTrade      float64 `json:"worst_trade"`
	Contribution    float64 `json:"contribution_percent"` // 对总收益的贡献百分比
	SharpeRatio     float64 `json:"sharpe_ratio"`
}

// StrategyAttribution 按策略归因
type StrategyAttribution struct {
	StrategyType    string  `json:"strategy_type"` // 如：Long, Short, Range Trading
	TotalPnL        float64 `json:"total_pnl"`
	WinRate         float64 `json:"win_rate"`
	TradesCount     int     `json:"trades_count"`
	Contribution    float64 `json:"contribution_percent"`
}

// TimeframeAttribution 按时间段归因
type TimeframeAttribution struct {
	Period          string  `json:"period"` // 如：Morning, Afternoon, Evening, Night
	StartHour       int     `json:"start_hour"`
	EndHour         int     `json:"end_hour"`
	TotalPnL        float64 `json:"total_pnl"`
	WinRate         float64 `json:"win_rate"`
	TradesCount     int     `json:"trades_count"`
	Contribution    float64 `json:"contribution_percent"`
}

// AttributionSummary 归因总结
type AttributionSummary struct {
	TotalPnL            float64 `json:"total_pnl"`
	TotalTrades         int     `json:"total_trades"`
	OverallWinRate      float64 `json:"overall_win_rate"`
	BestAsset           string  `json:"best_asset"`
	WorstAsset          string  `json:"worst_asset"`
	BestStrategy        string  `json:"best_strategy"`
	BestTimeframe       string  `json:"best_timeframe"`
	ConcentrationRisk   float64 `json:"concentration_risk"` // 前3资产占比
}

// TradeRecord 交易记录
type TradeRecord struct {
	Symbol      string
	EntryTime   time.Time
	ExitTime    time.Time
	Side        string  // Long or Short
	PnL         float64
	PnLPercent  float64
	EntryPrice  float64
	ExitPrice   float64
}

// CalculatePerformanceAttribution 计算绩效归因
func CalculatePerformanceAttribution(trades []TradeRecord) (*PerformanceAttribution, error) {
	if len(trades) == 0 {
		return nil, fmt.Errorf("没有交易记录")
	}

	// 按资产归因
	byAsset := calculateAssetAttribution(trades)

	// 按策略归因
	byStrategy := calculateStrategyAttribution(trades)

	// 按时间段归因
	byTimeframe := calculateTimeframeAttribution(trades)

	// 计算总结
	summary := calculateAttributionSummary(trades, byAsset, byStrategy, byTimeframe)

	// 找出最佳和最差
	topContributors := getTopN(byAsset, 5, true)
	worstPerformers := getTopN(byAsset, 5, false)

	return &PerformanceAttribution{
		ByAsset:         byAsset,
		ByStrategy:      byStrategy,
		ByTimeframe:     byTimeframe,
		Summary:         summary,
		TopContributors: topContributors,
		WorstPerformers: worstPerformers,
		CalculatedAt:    time.Now(),
	}, nil
}

// calculateAssetAttribution 计算按资产归因
func calculateAssetAttribution(trades []TradeRecord) []AssetAttribution {
	assetMap := make(map[string]*AssetAttribution)

	// 计算总PnL用于贡献度计算
	var totalPnL float64
	for _, trade := range trades {
		totalPnL += trade.PnL
	}

	// 按资产聚合
	for _, trade := range trades {
		if _, exists := assetMap[trade.Symbol]; !exists {
			assetMap[trade.Symbol] = &AssetAttribution{
				Symbol:      trade.Symbol,
				TradesCount: 0,
				BestTrade:   trade.PnL,
				WorstTrade:  trade.PnL,
			}
		}

		attr := assetMap[trade.Symbol]
		attr.TotalPnL += trade.PnL
		attr.TradesCount++

		if trade.PnL > attr.BestTrade {
			attr.BestTrade = trade.PnL
		}
		if trade.PnL < attr.WorstTrade {
			attr.WorstTrade = trade.PnL
		}
	}

	// 计算衍生指标
	results := []AssetAttribution{}
	for _, attr := range assetMap {
		// 计算胜率
		wins := 0
		returns := []float64{}
		for _, trade := range trades {
			if trade.Symbol == attr.Symbol {
				if trade.PnL > 0 {
					wins++
				}
				returns = append(returns, trade.PnLPercent)
			}
		}
		attr.WinRate = float64(wins) / float64(attr.TradesCount) * 100

		// 平均交易回报
		if attr.TradesCount > 0 {
			attr.AvgTradeReturn = attr.TotalPnL / float64(attr.TradesCount)
		}

		// 贡献度
		if totalPnL != 0 {
			attr.Contribution = (attr.TotalPnL / totalPnL) * 100
		}

		// 总PnL百分比（相对初始投资）
		if attr.TradesCount > 0 && len(returns) > 0 {
			attr.TotalPnLPercent = calculateMean(returns)
		}

		// Sharpe Ratio (简化版)
		if len(returns) > 1 {
			meanReturn := calculateMean(returns)
			stdDev := calculateStdDev(returns)
			if stdDev != 0 {
				attr.SharpeRatio = meanReturn / stdDev
			}
		}

		results = append(results, *attr)
	}

	// 按贡献度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalPnL > results[j].TotalPnL
	})

	return results
}

// calculateStrategyAttribution 计算按策略归因
func calculateStrategyAttribution(trades []TradeRecord) []StrategyAttribution {
	strategyMap := make(map[string]*StrategyAttribution)

	var totalPnL float64
	for _, trade := range trades {
		totalPnL += trade.PnL
	}

	for _, trade := range trades {
		strategy := trade.Side // Long or Short

		if _, exists := strategyMap[strategy]; !exists {
			strategyMap[strategy] = &StrategyAttribution{
				StrategyType: strategy,
			}
		}

		attr := strategyMap[strategy]
		attr.TotalPnL += trade.PnL
		attr.TradesCount++
	}

	// 计算胜率和贡献度
	results := []StrategyAttribution{}
	for strategyType, attr := range strategyMap {
		wins := 0
		for _, trade := range trades {
			if trade.Side == strategyType && trade.PnL > 0 {
				wins++
			}
		}

		attr.WinRate = float64(wins) / float64(attr.TradesCount) * 100

		if totalPnL != 0 {
			attr.Contribution = (attr.TotalPnL / totalPnL) * 100
		}

		results = append(results, *attr)
	}

	// 按PnL排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalPnL > results[j].TotalPnL
	})

	return results
}

// calculateTimeframeAttribution 计算按时间段归因
func calculateTimeframeAttribution(trades []TradeRecord) []TimeframeAttribution {
	// 定义时间段
	timeframes := []struct {
		Period    string
		StartHour int
		EndHour   int
	}{
		{"Asian Session (0-8)", 0, 8},
		{"European Session (8-16)", 8, 16},
		{"US Session (16-24)", 16, 24},
	}

	tfMap := make(map[string]*TimeframeAttribution)

	for _, tf := range timeframes {
		tfMap[tf.Period] = &TimeframeAttribution{
			Period:    tf.Period,
			StartHour: tf.StartHour,
			EndHour:   tf.EndHour,
		}
	}

	var totalPnL float64
	for _, trade := range trades {
		totalPnL += trade.PnL
	}

	// 聚合交易到时间段
	for _, trade := range trades {
		hour := trade.EntryTime.Hour()

		for period, attr := range tfMap {
			if hour >= attr.StartHour && hour < attr.EndHour {
				attr.TotalPnL += trade.PnL
				attr.TradesCount++

				// 计算胜率
				if trade.PnL > 0 {
					// wins++
				}
				break
			}
		}
	}

	// 计算贡献度和胜率
	results := []TimeframeAttribution{}
	for period, attr := range tfMap {
		if attr.TradesCount > 0 {
			wins := 0
			for _, trade := range trades {
				hour := trade.EntryTime.Hour()
				if hour >= attr.StartHour && hour < attr.EndHour && trade.PnL > 0 {
					wins++
				}
			}
			attr.WinRate = float64(wins) / float64(attr.TradesCount) * 100

			if totalPnL != 0 {
				attr.Contribution = (attr.TotalPnL / totalPnL) * 100
			}
		}

		// 只添加有交易的时间段
		if attr.TradesCount > 0 {
			results = append(results, *attr)
		}

		_ = period
	}

	// 按PnL排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalPnL > results[j].TotalPnL
	})

	return results
}

// calculateAttributionSummary 计算归因总结
func calculateAttributionSummary(trades []TradeRecord, byAsset []AssetAttribution, byStrategy []StrategyAttribution, byTimeframe []TimeframeAttribution) *AttributionSummary {
	var totalPnL float64
	wins := 0

	for _, trade := range trades {
		totalPnL += trade.PnL
		if trade.PnL > 0 {
			wins++
		}
	}

	winRate := 0.0
	if len(trades) > 0 {
		winRate = float64(wins) / float64(len(trades)) * 100
	}

	// 最佳/最差资产
	bestAsset := ""
	worstAsset := ""
	if len(byAsset) > 0 {
		bestAsset = byAsset[0].Symbol
		worstAsset = byAsset[len(byAsset)-1].Symbol
	}

	// 最佳策略
	bestStrategy := ""
	if len(byStrategy) > 0 {
		bestStrategy = byStrategy[0].StrategyType
	}

	// 最佳时间段
	bestTimeframe := ""
	if len(byTimeframe) > 0 {
		bestTimeframe = byTimeframe[0].Period
	}

	// 集中度风险（前3资产占比）
	concentrationRisk := 0.0
	if totalPnL != 0 {
		top3PnL := 0.0
		for i := 0; i < 3 && i < len(byAsset); i++ {
			top3PnL += byAsset[i].TotalPnL
		}
		concentrationRisk = (top3PnL / totalPnL) * 100
	}

	return &AttributionSummary{
		TotalPnL:          totalPnL,
		TotalTrades:       len(trades),
		OverallWinRate:    winRate,
		BestAsset:         bestAsset,
		WorstAsset:        worstAsset,
		BestStrategy:      bestStrategy,
		BestTimeframe:     bestTimeframe,
		ConcentrationRisk: concentrationRisk,
	}
}

// getTopN 获取前N个或后N个
func getTopN(assets []AssetAttribution, n int, top bool) []AssetAttribution {
	if n <= 0 || len(assets) == 0 {
		return []AssetAttribution{}
	}

	// 复制
	sorted := make([]AssetAttribution, len(assets))
	copy(sorted, assets)

	// 排序
	if top {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].TotalPnL > sorted[j].TotalPnL
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].TotalPnL < sorted[j].TotalPnL
		})
	}

	if n > len(sorted) {
		n = len(sorted)
	}

	return sorted[:n]
}
