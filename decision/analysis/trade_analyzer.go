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

	return res
}
