package analysis

import "fmt"

// PatternDetector detects failure patterns in trade analysis.
type PatternDetector struct{}

// NewPatternDetector creates a new PatternDetector.
func NewPatternDetector() *PatternDetector {
	return &PatternDetector{}
}

// DetectFailurePatterns identifies patterns from the analysis result.
func (pd *PatternDetector) DetectFailurePatterns(analysis *TradeAnalysisResult) []FailurePattern {
	var patterns []FailurePattern

	// 1. High Leverage Risk / Streak Risk
	// Logic: Low ProfitFactor combined with high losing streaks suggests risk management issues often associated with high leverage or poor stop loss.
	if analysis.ProfitFactor < 1.5 && analysis.LoseStreak >= 3 {
		patterns = append(patterns, FailurePattern{
			PatternType:    "high_risk_streaks",
			Frequency:      analysis.LoseStreak,
			Confidence:     0.8,
			AffectedTrades: analysis.LosingTrades, // Broad estimate
			ImpactLoss:     analysis.AverageLossPerLoss * float64(analysis.LosingTrades),
			Description:    fmt.Sprintf("Detected consecutive loss streak of %d with low profit factor (%.2f). Potential over-leveraging or poor stop-loss strategy.", analysis.LoseStreak, analysis.ProfitFactor),
		})
	}

	// 2. Poor Pair Selection
	// Logic: If there is a worst performing pair with negative average profit.
	if analysis.WorstPerformingPair != "" {
		worstPair := analysis.TradeByPairStats[analysis.WorstPerformingPair]
		if worstPair != nil && worstPair.AvgProfit < -1.0 { // Arbitrary threshold -1%
			patterns = append(patterns, FailurePattern{
				PatternType:    "poor_pair_selection",
				Frequency:      worstPair.TotalTrades,
				Confidence:     0.9,
				AffectedTrades: worstPair.TotalTrades,
				ImpactLoss:     worstPair.AvgProfit * float64(worstPair.TotalTrades), // Approx Total Loss
				Description:    fmt.Sprintf("Consistent losses on %s (Avg: %.2f%%). Consider avoiding this pair.", worstPair.Symbol, worstPair.AvgProfit),
			})
		}
	}

	// 3. Poor Timing
	// Logic: If the best hour performs significantly better than average (or worst hour significantly worse).
	// Simplified: If WinRate is low (<40%) but BestHour has high WinRate (>60%).
	if analysis.WinRate < 40 && analysis.BestTradingHour != -1 {
		bestHourStat := analysis.TradeByHourStats[analysis.BestTradingHour]
		if bestHourStat != nil && bestHourStat.WinRate > 60 {
			patterns = append(patterns, FailurePattern{
				PatternType:    "poor_timing",
				Frequency:      1, // Abstract frequency
				Confidence:     0.7,
				AffectedTrades: analysis.TotalTrades - bestHourStat.TotalTrades,
				ImpactLoss:     0, // Hard to calc without iteration
				Description:    fmt.Sprintf("Overall win rate is low (%.1f%%) but hour %d has high win rate (%.1f%%). Consider restricting trading to this window.", analysis.WinRate, analysis.BestTradingHour, bestHourStat.WinRate),
			})
		}
	}

	return patterns
}
