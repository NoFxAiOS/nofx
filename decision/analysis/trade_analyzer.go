package analysis

import (
	"math"
	"nofx/database"
	"time"
)

// TradeDataProvider defines the interface for fetching trade data.
type TradeDataProvider interface {
	GetTradesInPeriod(traderID string, startDate, endDate time.Time) ([]database.TradeRecord, error)
}

// TradeAnalyzer performs statistical analysis on trade records.
type TradeAnalyzer struct {
	provider TradeDataProvider
}

// NewTradeAnalyzer creates a new TradeAnalyzer.
func NewTradeAnalyzer(provider TradeDataProvider) *TradeAnalyzer {
	return &TradeAnalyzer{provider: provider}
}

// AnalyzeTradesForPeriod fetches trades and analyzes them.
func (ta *TradeAnalyzer) AnalyzeTradesForPeriod(traderID string, start, end time.Time) (*TradeAnalysisResult, error) {
	trades, err := ta.provider.GetTradesInPeriod(traderID, start, end)
	if err != nil {
		return nil, err
	}
	return ta.Analyze(trades), nil
}

// Analyze calculates statistics from a slice of trades.
func (ta *TradeAnalyzer) Analyze(trades []database.TradeRecord) *TradeAnalysisResult {
	res := &TradeAnalysisResult{
		TotalTrades:      len(trades),
		TradeByPairStats: make(map[string]*PairStats),
		TradeByHourStats: make(map[int]*HourStats),
	}

	if len(trades) == 0 {
		return res
	}

	var totalWinPct, totalLossPct float64
	var grossWinPct, grossLossPct float64
	var currentWinStreak, currentLoseStreak int
	var totalHoldingTime int64

	for _, t := range trades {
		// Pair Stats
		if _, ok := res.TradeByPairStats[t.Symbol]; !ok {
			res.TradeByPairStats[t.Symbol] = &PairStats{Symbol: t.Symbol}
		}
		pairStat := res.TradeByPairStats[t.Symbol]
		pairStat.TotalTrades++
		pairStat.TotalProfit += t.ProfitPct

		// Hour Stats
		hour := t.CreatedAt.Hour()
		if _, ok := res.TradeByHourStats[hour]; !ok {
			res.TradeByHourStats[hour] = &HourStats{Hour: hour}
		}
		hourStat := res.TradeByHourStats[hour]
		hourStat.TotalTrades++
		hourStat.AvgProfit += t.ProfitPct // Accumulated for now, averaged later

		// Win/Loss
		if t.ProfitPct > 0 {
			res.WinningTrades++
			totalWinPct += t.ProfitPct
			grossWinPct += t.ProfitPct

			pairStat.WinRate++ // Accumulated count for now
			hourStat.WinRate++ // Accumulated count for now

			// Streaks
			currentWinStreak++
			if currentWinStreak > res.WinStreak {
				res.WinStreak = currentWinStreak
			}
			currentLoseStreak = 0
		} else {
			res.LosingTrades++
			totalLossPct += t.ProfitPct // Negative value
			grossLossPct += math.Abs(t.ProfitPct)

			currentLoseStreak++
			if currentLoseStreak > res.LoseStreak {
				res.LoseStreak = currentLoseStreak
			}
			currentWinStreak = 0
		}

		totalHoldingTime += t.HoldingTimeSeconds
	}

	// Final Calculations
	if res.TotalTrades > 0 {
		res.WinRate = float64(res.WinningTrades) / float64(res.TotalTrades) * 100
		res.AvgHoldingTime = time.Duration(totalHoldingTime/int64(res.TotalTrades)) * time.Second
	}

	if res.WinningTrades > 0 {
		res.AverageProfitPerWin = totalWinPct / float64(res.WinningTrades)
	}

	if res.LosingTrades > 0 {
		res.AverageLossPerLoss = totalLossPct / float64(res.LosingTrades) // Negative value
	}

	if grossLossPct > 0 {
		res.ProfitFactor = grossWinPct / grossLossPct
	} else if grossWinPct > 0 {
		res.ProfitFactor = 999.0 // Infinite profit factor
	}

	if res.AverageLossPerLoss != 0 {
		res.RiskRewardRatio = math.Abs(res.AverageProfitPerWin / res.AverageLossPerLoss)
	}

	// Finalize Map Stats & Find Best/Worst
	var bestPairVal, worstPairVal float64 = -999999, 999999

	for _, ps := range res.TradeByPairStats {
		if ps.TotalTrades > 0 {
			ps.AvgProfit = ps.TotalProfit / float64(ps.TotalTrades)
			// Convert accumulated count to percentage
			ps.WinRate = (ps.WinRate / float64(ps.TotalTrades)) * 100
		}

		if ps.AvgProfit > bestPairVal {
			bestPairVal = ps.AvgProfit
			res.BestPerformingPair = ps.Symbol
		}
		if ps.AvgProfit < worstPairVal {
			worstPairVal = ps.AvgProfit
			res.WorstPerformingPair = ps.Symbol
		}
	}

	var bestHourVal float64 = -999999
	for _, hs := range res.TradeByHourStats {
		if hs.TotalTrades > 0 {
			// Convert accumulated sum to avg
			totalProfit := hs.AvgProfit
			hs.AvgProfit = totalProfit / float64(hs.TotalTrades)
			// Convert accumulated count to percentage
			hs.WinRate = (hs.WinRate / float64(hs.TotalTrades)) * 100
		}

		if hs.AvgProfit > bestHourVal {
			bestHourVal = hs.AvgProfit
			res.BestTradingHour = hs.Hour
		}
	}

	// NEW: Calculate advanced metrics
	ta.calculateSharpeRatio(res, trades)
	ta.calculateMaxDrawdown(res, trades)
	ta.calculateConsecutiveLosses(res, trades)
	ta.calculateVolatility(res, trades)
	ta.calculateSymbolStats(res, trades)
	ta.calculateWeightedWinRate(res, trades)

	return res
}

// calculateSharpeRatio 计算夏普比率（风险调整收益）
// 公式: (平均收益 - 无风险率) / 标准差 * sqrt(252年化)
func (ta *TradeAnalyzer) calculateSharpeRatio(res *TradeAnalysisResult, trades []database.TradeRecord) {
	if len(trades) < 2 {
		res.SharpeRatio = 0
		return
	}

	// 收集收益率
	var returns []float64
	for _, t := range trades {
		returns = append(returns, t.ProfitPct)
	}

	// 计算平均收益
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	// 计算标准差
	var variance float64
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	stdDev := math.Sqrt(variance / float64(len(returns)))

	// 计算年化夏普比率
	if stdDev == 0 {
		res.SharpeRatio = 0
	} else {
		riskFreeRate := 0.02 / 252 // 假设年化无风险率2%
		res.SharpeRatio = (mean - riskFreeRate) / stdDev * math.Sqrt(252)
	}
}

// calculateMaxDrawdown 计算最大回撤百分比
// 从高峰到低谷的最大下跌百分比
func (ta *TradeAnalyzer) calculateMaxDrawdown(res *TradeAnalysisResult, trades []database.TradeRecord) {
	if len(trades) == 0 {
		res.MaxDrawdownPercent = 0
		return
	}

	var accountValue float64 = 100.0 // 初始账户价值
	var peak float64 = 100.0
	var maxDD float64 = 0

	for _, t := range trades {
		accountValue = accountValue * (1 + t.ProfitPct/100.0)

		if accountValue > peak {
			peak = accountValue
		}

		dd := ((peak - accountValue) / peak) * 100
		if dd > maxDD {
			maxDD = dd
		}
	}

	res.MaxDrawdownPercent = maxDD
}

// calculateConsecutiveLosses 计算连续亏损
func (ta *TradeAnalyzer) calculateConsecutiveLosses(res *TradeAnalysisResult, trades []database.TradeRecord) {
	if len(trades) == 0 {
		res.ConsecutiveLosses = 0
		res.MaxConsecutiveLoss = 0
		return
	}

	// 从最后向前计数
	currentLosses := 0
	maxLosses := 0

	for i := len(trades) - 1; i >= 0; i-- {
		if trades[i].ProfitPct < 0 {
			currentLosses++
			if currentLosses > maxLosses {
				maxLosses = currentLosses
			}
		} else {
			currentLosses = 0
		}
	}

	res.ConsecutiveLosses = currentLosses
	res.MaxConsecutiveLoss = maxLosses
}

// calculateVolatility 计算波动率（标准差）
func (ta *TradeAnalyzer) calculateVolatility(res *TradeAnalysisResult, trades []database.TradeRecord) {
	if len(trades) < 2 {
		res.Volatility = 0
		return
	}

	// 计算平均值
	var sum float64
	for _, t := range trades {
		sum += t.ProfitPct
	}
	mean := sum / float64(len(trades))

	// 计算标准差
	var variance float64
	for _, t := range trades {
		variance += math.Pow(t.ProfitPct-mean, 2)
	}
	stdDev := math.Sqrt(variance / float64(len(trades)))

	res.Volatility = stdDev
}

// calculateSymbolStats 计算每个币种的详细统计
func (ta *TradeAnalyzer) calculateSymbolStats(res *TradeAnalysisResult, trades []database.TradeRecord) {
	res.SymbolStats = make(map[string]*SymbolStats)

	symbolTrades := make(map[string][]database.TradeRecord)
	for _, t := range trades {
		symbolTrades[t.Symbol] = append(symbolTrades[t.Symbol], t)
	}

	for symbol, sTrades := range symbolTrades {
		stats := &SymbolStats{Symbol: symbol}
		stats.TradesCount = len(sTrades)

		if len(sTrades) == 0 {
			continue
		}

		var wins, totalProfit, totalLoss float64
		var bestTrade, worstTrade float64 = -999999, 999999

		for _, t := range sTrades {
			if t.ProfitPct > 0 {
				wins++
				totalProfit += t.ProfitPct
			} else {
				totalLoss += math.Abs(t.ProfitPct)
			}

			if t.ProfitPct > bestTrade {
				bestTrade = t.ProfitPct
			}
			if t.ProfitPct < worstTrade {
				worstTrade = t.ProfitPct
			}
		}

		stats.WinRate = (wins / float64(len(sTrades))) * 100
		if wins > 0 {
			stats.AvgProfitPct = totalProfit / wins
		}
		if wins < float64(len(sTrades)) {
			stats.AvgLossPct = totalLoss / (float64(len(sTrades)) - wins)
		}
		stats.BestTradePct = bestTrade
		stats.WorstTradePct = worstTrade

		// Profit factor
		if totalLoss > 0 {
			stats.ProfitFactor = totalProfit / totalLoss
		} else if totalProfit > 0 {
			stats.ProfitFactor = 999.0
		}

		// Symbol volatility
		mean := (totalProfit - totalLoss) / float64(len(sTrades))
		var variance float64
		for _, t := range sTrades {
			variance += math.Pow(t.ProfitPct-mean, 2)
		}
		stats.Volatility = math.Sqrt(variance / float64(len(sTrades)))

		// Symbol max drawdown
		var symbolAccountValue float64 = 100.0
		var peak float64 = 100.0
		var maxDD float64 = 0

		for _, t := range sTrades {
			symbolAccountValue = symbolAccountValue * (1 + t.ProfitPct/100.0)
			if symbolAccountValue > peak {
				peak = symbolAccountValue
			}
			dd := ((peak - symbolAccountValue) / peak) * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
		stats.MaxDrawdownPct = maxDD

		res.SymbolStats[symbol] = stats
	}
}

// calculateWeightedWinRate 计算加权胜率（时间衰减）
// 最近的交易权重更高
func (ta *TradeAnalyzer) calculateWeightedWinRate(res *TradeAnalysisResult, trades []database.TradeRecord) {
	if len(trades) == 0 {
		res.WeightedWinRate = 0
		return
	}

	lambda := 0.01 // 时间衰减参数

	var weightedWins, totalWeight float64

	for i, t := range trades {
		// 最近的交易权重更高
		weight := math.Exp(lambda * float64(i))

		totalWeight += weight

		if t.ProfitPct > 0 {
			weightedWins += weight
		}
	}

	if totalWeight > 0 {
		res.WeightedWinRate = (weightedWins / totalWeight) * 100
	}
}
