package analysis

import "time"

// TradeAnalysisResult holds the calculated statistics for a set of trades.
type TradeAnalysisResult struct {
	TotalTrades         int
	WinningTrades       int
	LosingTrades        int
	WinRate             float64
	AverageProfitPerWin float64
	AverageLossPerLoss  float64
	ProfitFactor        float64
	RiskRewardRatio     float64

	// Time analysis
	WinStreak      int
	LoseStreak     int
	AvgHoldingTime time.Duration

	// Market analysis
	BestPerformingPair  string
	WorstPerformingPair string
	BestTradingHour     int

	// Detailed breakdown
	TradeByPairStats map[string]*PairStats
	TradeByHourStats map[int]*HourStats
}

// PairStats holds statistics for a specific trading pair.
type PairStats struct {
	Symbol      string
	TotalTrades int
	WinRate     float64
	AvgProfit   float64
	TotalProfit float64
}

// HourStats holds statistics for a specific hour of the day (0-23).
type HourStats struct {
	Hour        int
	TotalTrades int
	WinRate     float64
	AvgProfit   float64
}

// FailurePattern represents a recognized pattern of failure in trading behavior.
type FailurePattern struct {
	PatternType    string  // e.g., "high_leverage", "wrong_direction"
	Frequency      int     // Number of occurrences
	Confidence     float64 // 0.0 to 1.0
	AffectedTrades int     // Number of trades matching pattern
	ImpactLoss     float64 // Total loss amount
	Description    string  // Human readable description
}
