// backtest/risk/types.go
package risk

import "time"

// Position represents a trading position for risk calculation.
// Quantity is always >= 0, direction comes from Side.
type Position struct {
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`       // "long" or "short"
	Quantity  float64 `json:"quantity"`   // Always >= 0
	MarkPrice float64 `json:"mark_price"`
	Leverage  int     `json:"leverage"`
}

// Direction returns +1 for long, -1 for short.
func (p *Position) Direction() float64 {
	if p.Side == "long" {
		return 1.0
	}
	return -1.0
}

// Notional returns the absolute notional value.
func (p *Position) Notional() float64 {
	return p.Quantity * p.MarkPrice
}

// PortfolioRiskMetrics holds dual-metric risk measurements.
type PortfolioRiskMetrics struct {
	FastRisk           float64   `json:"fast_risk"`            // Real-time conservative estimate
	AccurateRisk       float64   `json:"accurate_risk"`        // With correlation matrix
	LastAccurateUpdate time.Time `json:"last_accurate_update"`
}

// StrategyResults holds performance metrics for a strategy.
type StrategyResults struct {
	ID           string             `json:"id"`
	NetPnL       float64            `json:"net_pnl"`
	Sharpe       float64            `json:"sharpe"`
	ProfitFactor float64            `json:"profit_factor"`
	Calmar       float64            `json:"calmar"`
	WinRate      float64            `json:"win_rate"`
	ES95         float64            `json:"es95"`         // Expected Shortfall 95%
	MaxDrawdown  float64            `json:"max_drawdown"`
	TradesCount  int                `json:"trades_count"`
	ActiveDays   int                `json:"active_days"`
	DailyPnL     map[string]float64 `json:"daily_pnl"`    // date string -> pnl
	RiskUsedTS   []float64          `json:"risk_used_ts"` // Risk usage time series
	LeverageTS   []float64          `json:"leverage_ts"`  // Leverage time series
}

// RegimeSummary holds market regime classification.
type RegimeSummary struct {
	VolRegime     string    `json:"vol_regime"`     // "high", "mid", "low"
	TrendRegime   string    `json:"trend_regime"`   // "trending", "ranging"
	PrimaryRegime string    `json:"primary_regime"` // "high_trending", etc.
	ATRPercentile float64   `json:"atr_percentile"`
	ADX           float64   `json:"adx"`
	CalculatedAt  time.Time `json:"calculated_at"`
}

// ABTestCycle represents one complete A/B test cycle.
type ABTestCycle struct {
	ID            string                     `json:"id"`
	StartTime     time.Time                  `json:"start_time"`
	EndTime       time.Time                  `json:"end_time"`
	ChampionID    string                     `json:"champion_id"`
	ChallengerIDs []string                   `json:"challenger_ids"`
	Results       map[string]StrategyResults `json:"results"`
	RegimeSummary RegimeSummary              `json:"regime_summary"`
	Winner        string                     `json:"winner"` // champion or challenger ID
}

// GateResult holds the result of a gate check.
type GateResult struct {
	Passed  bool                   `json:"passed"`
	Gate    string                 `json:"gate"`
	Reason  string                 `json:"reason"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// BudgetAllocation maps strategy ID to risk budget (0.0 - 1.0).
type BudgetAllocation map[string]float64

// Config constants per v1.1.2 spec.
const (
	// Portfolio risk
	TargetPortfolioVol     = 0.02 // 2% daily volatility target
	CorrelationWindow      = 60   // 60 hourly bars
	AccurateUpdateInterval = time.Hour

	// Budget allocation
	ChampionMinBudget     = 0.50
	ChampionAbsoluteFloor = 0.40
	ChallengerMaxBudget   = 0.25
	ChallengerMinBudget   = 0.05
	MaxChallengers        = 10 // floor((1-0.50)/0.05)

	// Regime thresholds
	VolHighPercentile = 0.70
	VolLowPercentile  = 0.30
	TrendADXThreshold = 25.0

	// Evidence gate
	MinDaysForSegment  = 20
	MinDaysFor4Segments = 40
	MinTrades          = 30
	MinActiveDays      = 15

	// Risk Parity gate
	RiskDeviationThreshold = 0.20 // 20%
	LeverageDeviationMax   = 1.0
	PortfolioRiskP95Max    = 1.1

	// Dominance gate
	DominanceWinsRequired = 3
)
