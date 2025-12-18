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

	// Risk-adjusted metrics (NEW)
	SharpeRatio         float64 // Risk-adjusted return (年化)
	MaxDrawdownPercent  float64 // Maximum peak-to-trough drawdown
	ConsecutiveLosses   int     // Current consecutive losses
	MaxConsecutiveLoss  int     // Maximum consecutive losses sequence
	Volatility          float64 // Standard deviation of returns
	WeightedWinRate     float64 // Win rate with time decay weighting

	// Detailed breakdown
	TradeByPairStats map[string]*PairStats
	TradeByHourStats map[int]*HourStats
	SymbolStats      map[string]*SymbolStats // NEW: Per-symbol detailed stats
}

// PairStats holds statistics for a specific trading pair.
type PairStats struct {
	Symbol      string
	TotalTrades int
	WinRate     float64
	AvgProfit   float64
	TotalProfit float64
}

// SymbolStats holds detailed per-symbol performance metrics (NEW)
type SymbolStats struct {
	Symbol         string  // Coin symbol
	TradesCount    int     // Total trades
	WinRate        float64 // Win rate percentage
	AvgProfitPct   float64 // Average profit per winning trade
	AvgLossPct     float64 // Average loss per losing trade
	BestTradePct   float64 // Best single trade
	WorstTradePct  float64 // Worst single trade
	Volatility     float64 // Standard deviation of returns
	MaxDrawdownPct float64 // Max drawdown for this symbol
	ProfitFactor   float64 // Gross wins / Gross losses
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
