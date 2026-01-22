# Champion-Challenger A/B Testing Risk Framework Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the complete v1.1.2 Champion-Challenger A/B testing framework with portfolio risk control in the backtest module.

**Architecture:**
- New `backtest/risk/` package for risk calculations (portfolio risk, correlation matrix, budget allocation)
- New `backtest/abtest/` package for Champion-Challenger framework (cycle management, gates, promotion)
- Integration with existing `Runner` and `DataFeed` for data access

**Tech Stack:** Go 1.25.3, testify for testing, gonum for matrix operations

---

## Prerequisites

```bash
# Add gonum dependency for matrix operations
cd /mnt/c/Users/david/nofx
go get gonum.org/v1/gonum/mat
go get gonum.org/v1/gonum/stat
```

---

## Task 1: Core Types Definition

**Files:**
- Create: `backtest/risk/types.go`
- Test: `backtest/risk/types_test.go`

**Step 1: Write the types file**

```go
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
	ES95         float64            `json:"es95"`          // Expected Shortfall 95%
	MaxDrawdown  float64            `json:"max_drawdown"`
	TradesCount  int                `json:"trades_count"`
	ActiveDays   int                `json:"active_days"`
	DailyPnL     map[string]float64 `json:"daily_pnl"`     // date string -> pnl
	RiskUsedTS   []float64          `json:"risk_used_ts"`  // Risk usage time series
	LeverageTS   []float64          `json:"leverage_ts"`   // Leverage time series
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
	ID            string                   `json:"id"`
	StartTime     time.Time                `json:"start_time"`
	EndTime       time.Time                `json:"end_time"`
	ChampionID    string                   `json:"champion_id"`
	ChallengerIDs []string                 `json:"challenger_ids"`
	Results       map[string]StrategyResults `json:"results"`
	RegimeSummary RegimeSummary            `json:"regime_summary"`
	Winner        string                   `json:"winner"` // champion or challenger ID
}

// GateResult holds the result of a gate check.
type GateResult struct {
	Passed  bool              `json:"passed"`
	Gate    string            `json:"gate"`
	Reason  string            `json:"reason"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// BudgetAllocation maps strategy ID to risk budget (0.0 - 1.0).
type BudgetAllocation map[string]float64

// Config constants per v1.1.2 spec.
const (
	// Portfolio risk
	TargetPortfolioVol      = 0.02  // 2% daily volatility target
	CorrelationWindow       = 60    // 60 hourly bars
	AccurateUpdateInterval  = time.Hour

	// Budget allocation
	ChampionMinBudget       = 0.50
	ChampionAbsoluteFloor   = 0.40
	ChallengerMaxBudget     = 0.25
	ChallengerMinBudget     = 0.05
	MaxChallengers          = 10    // floor((1-0.50)/0.05)

	// Regime thresholds
	VolHighPercentile       = 0.70
	VolLowPercentile        = 0.30
	TrendADXThreshold       = 25.0

	// Evidence gate
	MinDaysForSegment       = 20
	MinDaysFor4Segments     = 40
	MinTrades               = 30
	MinActiveDays           = 15

	// Risk Parity gate
	RiskDeviationThreshold  = 0.20  // 20%
	LeverageDeviationMax    = 1.0
	PortfolioRiskP95Max     = 1.1

	// Dominance gate
	DominanceWinsRequired   = 3
)
```

**Step 2: Write tests for Position methods**

```go
// backtest/risk/types_test.go
package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPosition_Direction(t *testing.T) {
	tests := []struct {
		name     string
		side     string
		expected float64
	}{
		{"long position", "long", 1.0},
		{"short position", "short", -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Position{Side: tt.side}
			assert.Equal(t, tt.expected, p.Direction())
		})
	}
}

func TestPosition_Notional(t *testing.T) {
	p := &Position{
		Quantity:  10.0,
		MarkPrice: 50000.0,
	}
	assert.Equal(t, 500000.0, p.Notional())
}
```

**Step 3: Run tests**

```bash
cd /mnt/c/Users/david/nofx
go test ./backtest/risk/... -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add backtest/risk/
git commit -m "feat(risk): add core types for Champion-Challenger framework

- Position with Direction() and Notional() methods
- PortfolioRiskMetrics for dual-metric risk
- StrategyResults for performance metrics
- RegimeSummary for market regime classification
- ABTestCycle for A/B test cycle tracking
- GateResult for gate check results
- Config constants per v1.1.2 spec"
```

---

## Task 2: Correlation Matrix Calculator

**Files:**
- Create: `backtest/risk/correlation.go`
- Test: `backtest/risk/correlation_test.go`

**Step 1: Write the failing test**

```go
// backtest/risk/correlation_test.go
package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorrelationMatrix_Calculate(t *testing.T) {
	// 3 symbols, 5 time periods
	// returns[symbol_idx][time_idx]
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},  // BTC
		{0.015, -0.025, 0.035, -0.015, 0.025}, // ETH (highly correlated)
		{-0.01, 0.02, -0.03, 0.01, -0.02}, // Inverse asset
	}
	symbols := []string{"BTCUSDT", "ETHUSDT", "INVERSE"}

	cm := NewCorrelationMatrix()
	err := cm.Update(symbols, returns)
	require.NoError(t, err)

	// BTC-ETH should be highly correlated (close to 1)
	corrBtcEth := cm.Get("BTCUSDT", "ETHUSDT")
	assert.Greater(t, corrBtcEth, 0.9)

	// BTC-INVERSE should be negatively correlated (close to -1)
	corrBtcInv := cm.Get("BTCUSDT", "INVERSE")
	assert.Less(t, corrBtcInv, -0.9)

	// Self-correlation should be 1
	assert.Equal(t, 1.0, cm.Get("BTCUSDT", "BTCUSDT"))
}

func TestCorrelationMatrix_SymbolOrder(t *testing.T) {
	returns := [][]float64{
		{0.01, -0.02, 0.03},
		{0.015, -0.025, 0.035},
	}
	symbols := []string{"BTCUSDT", "ETHUSDT"}

	cm := NewCorrelationMatrix()
	err := cm.Update(symbols, returns)
	require.NoError(t, err)

	// Query with different order should still work
	subMatrix := cm.GetSubMatrix([]string{"ETHUSDT", "BTCUSDT"})
	require.NotNil(t, subMatrix)

	// Diagonal should be 1s
	assert.Equal(t, 1.0, subMatrix[0][0])
	assert.Equal(t, 1.0, subMatrix[1][1])

	// Off-diagonal should match correlation
	assert.InDelta(t, cm.Get("ETHUSDT", "BTCUSDT"), subMatrix[0][1], 0.0001)
}

func TestCorrelationMatrix_InvalidShape(t *testing.T) {
	cm := NewCorrelationMatrix()

	// Mismatched symbols and returns
	err := cm.Update([]string{"A", "B"}, [][]float64{{0.01}})
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./backtest/risk/... -run TestCorrelationMatrix -v
```

Expected: FAIL - "undefined: NewCorrelationMatrix"

**Step 3: Write the implementation**

```go
// backtest/risk/correlation.go
package risk

import (
	"fmt"
	"math"
	"sync"
)

// CorrelationMatrix calculates and caches correlation matrix.
// Thread-safe for concurrent reads.
type CorrelationMatrix struct {
	mu          sync.RWMutex
	matrix      [][]float64       // n x n correlation matrix
	symbols     []string          // symbol order matching matrix indices
	symbolIndex map[string]int    // symbol -> index for O(1) lookup
	volatilities map[string]float64 // daily volatilities
}

// NewCorrelationMatrix creates a new correlation matrix calculator.
func NewCorrelationMatrix() *CorrelationMatrix {
	return &CorrelationMatrix{
		symbolIndex:  make(map[string]int),
		volatilities: make(map[string]float64),
	}
}

// Update recalculates correlation matrix from returns data.
// returns shape: (n_symbols, window) - each row is one symbol's return series.
func (c *CorrelationMatrix) Update(symbols []string, returns [][]float64) error {
	if len(symbols) != len(returns) {
		return fmt.Errorf("symbols count %d != returns rows %d", len(symbols), len(returns))
	}
	if len(symbols) == 0 {
		return fmt.Errorf("empty symbols list")
	}

	n := len(symbols)
	window := len(returns[0])

	// Validate all rows have same length
	for i, row := range returns {
		if len(row) != window {
			return fmt.Errorf("symbol %s has %d returns, expected %d", symbols[i], len(row), window)
		}
	}

	// Calculate means
	means := make([]float64, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		for _, r := range returns[i] {
			sum += r
		}
		means[i] = sum / float64(window)
	}

	// Calculate standard deviations (ddof=1 per v1.1.2 spec)
	stds := make([]float64, n)
	for i := 0; i < n; i++ {
		variance := 0.0
		for _, r := range returns[i] {
			diff := r - means[i]
			variance += diff * diff
		}
		if window > 1 {
			variance /= float64(window - 1) // ddof=1
		}
		stds[i] = math.Sqrt(variance)
	}

	// Calculate correlation matrix
	matrix := make([][]float64, n)
	for i := 0; i < n; i++ {
		matrix[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				matrix[i][j] = 1.0
				continue
			}
			if stds[i] < 1e-10 || stds[j] < 1e-10 {
				matrix[i][j] = 0.0
				continue
			}

			// Covariance
			cov := 0.0
			for t := 0; t < window; t++ {
				cov += (returns[i][t] - means[i]) * (returns[j][t] - means[j])
			}
			if window > 1 {
				cov /= float64(window - 1)
			}

			matrix[i][j] = cov / (stds[i] * stds[j])
		}
	}

	// Calculate daily volatilities (hourly std * sqrt(24))
	volatilities := make(map[string]float64, n)
	for i, sym := range symbols {
		volatilities[sym] = stds[i] * math.Sqrt(24)
	}

	// Build symbol index
	symbolIndex := make(map[string]int, n)
	for i, sym := range symbols {
		symbolIndex[sym] = i
	}

	// Atomic update
	c.mu.Lock()
	c.matrix = matrix
	c.symbols = append([]string{}, symbols...) // copy
	c.symbolIndex = symbolIndex
	c.volatilities = volatilities
	c.mu.Unlock()

	return nil
}

// Get returns correlation between two symbols.
func (c *CorrelationMatrix) Get(sym1, sym2 string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	i, ok1 := c.symbolIndex[sym1]
	j, ok2 := c.symbolIndex[sym2]
	if !ok1 || !ok2 {
		return 0.0
	}
	return c.matrix[i][j]
}

// GetSubMatrix returns correlation submatrix for given symbols.
// Handles symbol order differences via index mapping.
func (c *CorrelationMatrix) GetSubMatrix(symbols []string) [][]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	n := len(symbols)
	result := make([][]float64, n)

	for i := 0; i < n; i++ {
		result[i] = make([]float64, n)
		idx1, ok1 := c.symbolIndex[symbols[i]]
		if !ok1 {
			// Unknown symbol, use identity
			result[i][i] = 1.0
			continue
		}

		for j := 0; j < n; j++ {
			idx2, ok2 := c.symbolIndex[symbols[j]]
			if !ok2 {
				if i == j {
					result[i][j] = 1.0
				}
				continue
			}
			result[i][j] = c.matrix[idx1][idx2]
		}
	}

	return result
}

// GetVolatility returns daily volatility for a symbol.
func (c *CorrelationMatrix) GetVolatility(symbol string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if vol, ok := c.volatilities[symbol]; ok {
		return vol
	}
	return 0.03 // default 3% daily vol
}

// Symbols returns the current symbol list (copy).
func (c *CorrelationMatrix) Symbols() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string{}, c.symbols...)
}
```

**Step 4: Run tests**

```bash
go test ./backtest/risk/... -run TestCorrelationMatrix -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backtest/risk/correlation.go backtest/risk/correlation_test.go
git commit -m "feat(risk): add correlation matrix calculator

- Thread-safe CorrelationMatrix with symbol order tracking
- ddof=1 for unbiased standard deviation per v1.1.2 spec
- Daily volatility calculation (hourly std * sqrt(24))
- GetSubMatrix handles symbol order differences via index mapping"
```

---

## Task 3: Portfolio Risk Calculator (Dual Metric)

**Files:**
- Create: `backtest/risk/portfolio.go`
- Test: `backtest/risk/portfolio_test.go`

**Step 1: Write the failing test**

```go
// backtest/risk/portfolio_test.go
package risk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortfolioRiskCalculator_FastRisk(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Set up volatilities
	calc.corrMatrix.volatilities = map[string]float64{
		"BTCUSDT": 0.02, // 2% daily vol
		"ETHUSDT": 0.03, // 3% daily vol
	}

	positions := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 2},
		{Symbol: "ETHUSDT", Side: "short", Quantity: 10.0, MarkPrice: 3000, Leverage: 3},
	}
	equity := 100000.0

	fastRisk := calc.CalcFastRisk(positions, equity)

	// Fast risk = Σ(notional × vol × leverage) / equity / target_vol
	// BTC: 50000 * 0.02 * 2 / 100000 = 0.02
	// ETH: 30000 * 0.03 * 3 / 100000 = 0.027
	// Total: 0.047 / 0.02 = 2.35
	assert.InDelta(t, 2.35, fastRisk, 0.01)
}

func TestPortfolioRiskCalculator_AccurateRisk(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Set up correlation matrix with perfect positive correlation
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},
		{0.01, -0.02, 0.03, -0.01, 0.02}, // Same returns = correlation 1
	}
	err := calc.corrMatrix.Update(symbols, returns)
	require.NoError(t, err)

	positions := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
		{Symbol: "ETHUSDT", Side: "long", Quantity: 10.0, MarkPrice: 3000, Leverage: 1},
	}
	equity := 100000.0

	accurateRisk := calc.CalcAccurateRisk(positions, equity)

	// With perfect correlation, positions add linearly
	assert.Greater(t, accurateRisk, 0.0)
}

func TestPortfolioRiskCalculator_HedgingEffect(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Perfect negative correlation
	symbols := []string{"BTCUSDT", "INVERSE"}
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},
		{-0.01, 0.02, -0.03, 0.01, -0.02}, // Opposite = correlation -1
	}
	err := calc.corrMatrix.Update(symbols, returns)
	require.NoError(t, err)

	// Long both with same notional - should hedge
	positions := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
		{Symbol: "INVERSE", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
	}
	equity := 100000.0

	accurateRisk := calc.CalcAccurateRisk(positions, equity)

	// With perfect negative correlation and same exposure, risk should be near zero
	assert.Less(t, accurateRisk, 0.5)
}

func TestPortfolioRiskCalculator_VolatilitySameSource(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	symbols := []string{"BTCUSDT"}
	returns := [][]float64{{0.01, -0.02, 0.03, -0.01, 0.02}}
	err := calc.corrMatrix.Update(symbols, returns)
	require.NoError(t, err)

	// GetSymbolVolatility should return same value as corrMatrix
	vol := calc.GetSymbolVolatility("BTCUSDT")
	expectedVol := calc.corrMatrix.GetVolatility("BTCUSDT")
	assert.Equal(t, expectedVol, vol)
}
```

**Step 2: Run test to verify failure**

```bash
go test ./backtest/risk/... -run TestPortfolioRiskCalculator -v
```

Expected: FAIL

**Step 3: Write the implementation**

```go
// backtest/risk/portfolio.go
package risk

import (
	"math"
	"sync"
	"time"
)

// PortfolioRiskCalculator implements dual-metric portfolio risk calculation.
type PortfolioRiskCalculator struct {
	mu               sync.RWMutex
	targetVol        float64
	corrMatrix       *CorrelationMatrix
	lastAccurateCalc time.Time

	// Fallback volatilities (ATR-based)
	fallbackVols     map[string]float64
	fallbackMu       sync.RWMutex
}

// NewPortfolioRiskCalculator creates a new calculator.
func NewPortfolioRiskCalculator(targetVol float64) *PortfolioRiskCalculator {
	return &PortfolioRiskCalculator{
		targetVol:    targetVol,
		corrMatrix:   NewCorrelationMatrix(),
		fallbackVols: make(map[string]float64),
	}
}

// UpdateCorrelation updates the correlation matrix with new returns data.
// returns shape: (n_symbols, window)
func (p *PortfolioRiskCalculator) UpdateCorrelation(symbols []string, returns [][]float64) error {
	err := p.corrMatrix.Update(symbols, returns)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.lastAccurateCalc = time.Now()
	p.mu.Unlock()

	return nil
}

// SetFallbackVolatility sets ATR-based volatility for a symbol (used when correlation data unavailable).
func (p *PortfolioRiskCalculator) SetFallbackVolatility(symbol string, vol float64) {
	p.fallbackMu.Lock()
	p.fallbackVols[symbol] = vol
	p.fallbackMu.Unlock()
}

// GetSymbolVolatility returns daily volatility for a symbol.
// Per v1.1.2: fast risk uses same source as accurate risk.
func (p *PortfolioRiskCalculator) GetSymbolVolatility(symbol string) float64 {
	// Priority 1: correlation matrix volatility (same source as accurate)
	if vol := p.corrMatrix.GetVolatility(symbol); vol > 0 {
		return vol
	}

	// Priority 2: fallback (ATR-based)
	p.fallbackMu.RLock()
	if vol, ok := p.fallbackVols[symbol]; ok {
		p.fallbackMu.RUnlock()
		return vol
	}
	p.fallbackMu.RUnlock()

	// Priority 3: conservative default
	return 0.03 // 3% daily vol
}

// CalcFastRisk calculates fast (conservative) portfolio risk.
// Formula: Σ(notional × vol × leverage) / equity / target_vol
func (p *PortfolioRiskCalculator) CalcFastRisk(positions []Position, equity float64) float64 {
	if equity <= 0 || len(positions) == 0 {
		return 0
	}

	totalRisk := 0.0
	for _, pos := range positions {
		notional := pos.Notional()
		vol := p.GetSymbolVolatility(pos.Symbol)
		riskContrib := (notional * vol * float64(pos.Leverage)) / equity
		totalRisk += riskContrib
	}

	return totalRisk / p.targetVol
}

// CalcAccurateRisk calculates accurate portfolio risk with correlation.
// Formula: σ_p = sqrt(r^T × Σ × r) where Σ = diag(σ) × C × diag(σ)
func (p *PortfolioRiskCalculator) CalcAccurateRisk(positions []Position, equity float64) float64 {
	if equity <= 0 || len(positions) == 0 {
		return 0
	}

	n := len(positions)
	symbols := make([]string, n)
	for i, pos := range positions {
		symbols[i] = pos.Symbol
	}

	// Build risk exposure vector r
	// r_i = (notional / equity) × leverage × direction
	r := make([]float64, n)
	for i, pos := range positions {
		r[i] = (pos.Notional() / equity) * float64(pos.Leverage) * pos.Direction()
	}

	// Get volatilities (daily)
	sigmas := make([]float64, n)
	for i, sym := range symbols {
		sigmas[i] = p.GetSymbolVolatility(sym)
	}

	// Get correlation submatrix
	C := p.corrMatrix.GetSubMatrix(symbols)

	// Build covariance matrix: Σ = diag(σ) × C × diag(σ)
	cov := make([][]float64, n)
	for i := 0; i < n; i++ {
		cov[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			cov[i][j] = sigmas[i] * C[i][j] * sigmas[j]
		}
	}

	// Portfolio variance: r^T × Σ × r
	var portfolioVar float64
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			portfolioVar += r[i] * cov[i][j] * r[j]
		}
	}

	portfolioVol := math.Sqrt(math.Max(portfolioVar, 0))
	return portfolioVol / p.targetVol
}

// CalcBothRisks calculates both fast and accurate risk metrics.
func (p *PortfolioRiskCalculator) CalcBothRisks(positions []Position, equity float64) PortfolioRiskMetrics {
	p.mu.RLock()
	lastUpdate := p.lastAccurateCalc
	p.mu.RUnlock()

	return PortfolioRiskMetrics{
		FastRisk:           p.CalcFastRisk(positions, equity),
		AccurateRisk:       p.CalcAccurateRisk(positions, equity),
		LastAccurateUpdate: lastUpdate,
	}
}

// NeedsCorrelationUpdate checks if correlation matrix needs refresh.
func (p *PortfolioRiskCalculator) NeedsCorrelationUpdate() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Since(p.lastAccurateCalc) > AccurateUpdateInterval
}
```

**Step 4: Run tests**

```bash
go test ./backtest/risk/... -run TestPortfolioRiskCalculator -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backtest/risk/portfolio.go backtest/risk/portfolio_test.go
git commit -m "feat(risk): add dual-metric portfolio risk calculator

- CalcFastRisk: conservative real-time estimate
- CalcAccurateRisk: with correlation matrix (σ_p = sqrt(r^T Σ r))
- GetSymbolVolatility: same source for fast/accurate per v1.1.2
- Position direction handling: long=+1, short=-1"
```

---

## Task 4: Regime Calculator

**Files:**
- Create: `backtest/risk/regime.go`
- Test: `backtest/risk/regime_test.go`

**Step 1: Write failing test**

```go
// backtest/risk/regime_test.go
package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegimeCalculator_VolRegime(t *testing.T) {
	tests := []struct {
		name          string
		atrPercentile float64
		expected      string
	}{
		{"high volatility", 0.75, "high"},
		{"mid volatility", 0.50, "mid"},
		{"low volatility", 0.25, "low"},
		{"boundary high", 0.70, "high"},
		{"boundary low", 0.30, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := calcVolRegime(tt.atrPercentile)
			assert.Equal(t, tt.expected, regime)
		})
	}
}

func TestRegimeCalculator_TrendRegime(t *testing.T) {
	tests := []struct {
		name     string
		adx      float64
		expected string
	}{
		{"trending", 30.0, "trending"},
		{"ranging", 20.0, "ranging"},
		{"boundary", 25.0, "trending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := calcTrendRegime(tt.adx)
			assert.Equal(t, tt.expected, regime)
		})
	}
}

func TestRegimeCalculator_PrimaryRegime(t *testing.T) {
	rc := NewRegimeCalculator()

	// High vol + trending
	regime := rc.Calculate(0.75, 30.0)
	assert.Equal(t, "high_trending", regime.PrimaryRegime)
	assert.Equal(t, "high", regime.VolRegime)
	assert.Equal(t, "trending", regime.TrendRegime)

	// Low vol + ranging
	regime = rc.Calculate(0.25, 20.0)
	assert.Equal(t, "low_ranging", regime.PrimaryRegime)
}

func TestValidPrimaryRegimes(t *testing.T) {
	expected := []string{
		"high_trending", "high_ranging",
		"mid_trending", "mid_ranging",
		"low_trending", "low_ranging",
	}
	assert.ElementsMatch(t, expected, ValidPrimaryRegimes)
}
```

**Step 2: Run test**

```bash
go test ./backtest/risk/... -run TestRegime -v
```

Expected: FAIL

**Step 3: Write implementation**

```go
// backtest/risk/regime.go
package risk

import "time"

// ValidPrimaryRegimes lists all valid regime combinations.
var ValidPrimaryRegimes = []string{
	"high_trending", "high_ranging",
	"mid_trending", "mid_ranging",
	"low_trending", "low_ranging",
}

// RegimeCalculator calculates market regime from indicators.
type RegimeCalculator struct{}

// NewRegimeCalculator creates a new regime calculator.
func NewRegimeCalculator() *RegimeCalculator {
	return &RegimeCalculator{}
}

// Calculate computes regime from ATR percentile and ADX.
func (rc *RegimeCalculator) Calculate(atrPercentile, adx float64) RegimeSummary {
	volRegime := calcVolRegime(atrPercentile)
	trendRegime := calcTrendRegime(adx)

	return RegimeSummary{
		VolRegime:     volRegime,
		TrendRegime:   trendRegime,
		PrimaryRegime: volRegime + "_" + trendRegime,
		ATRPercentile: atrPercentile,
		ADX:           adx,
		CalculatedAt:  time.Now(),
	}
}

// calcVolRegime determines volatility regime from ATR percentile.
func calcVolRegime(atrPercentile float64) string {
	if atrPercentile >= VolHighPercentile {
		return "high"
	}
	if atrPercentile <= VolLowPercentile {
		return "low"
	}
	return "mid"
}

// calcTrendRegime determines trend regime from ADX.
func calcTrendRegime(adx float64) string {
	if adx >= TrendADXThreshold {
		return "trending"
	}
	return "ranging"
}

// CalculateATRPercentile calculates where current ATR sits in historical distribution.
func CalculateATRPercentile(currentATR float64, historicalATRs []float64) float64 {
	if len(historicalATRs) == 0 {
		return 0.5 // default to mid
	}

	count := 0
	for _, atr := range historicalATRs {
		if atr <= currentATR {
			count++
		}
	}
	return float64(count) / float64(len(historicalATRs))
}
```

**Step 4: Run tests**

```bash
go test ./backtest/risk/... -run TestRegime -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backtest/risk/regime.go backtest/risk/regime_test.go
git commit -m "feat(risk): add regime calculator with hard-coded thresholds

- Vol regime: ATR percentile >= 0.70 = high, <= 0.30 = low
- Trend regime: ADX >= 25 = trending
- ValidPrimaryRegimes enumeration for E-4 diversity check"
```

---

## Task 5: Budget Allocator (Water-Fill Method)

**Files:**
- Create: `backtest/risk/budget.go`
- Test: `backtest/risk/budget_test.go`

**Step 1: Write failing test**

```go
// backtest/risk/budget_test.go
package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetAllocator_ChampionOnly(t *testing.T) {
	alloc := NewBudgetAllocator()
	result := alloc.Allocate("champion", nil, nil)

	assert.Equal(t, 1.0, result["champion"])
}

func TestBudgetAllocator_SingleChallenger(t *testing.T) {
	alloc := NewBudgetAllocator()
	challengers := []string{"challenger1"}
	perf := map[string][]float64{
		"challenger1": {0.01, 0.02, -0.01}, // positive mean
	}

	result := alloc.Allocate("champion", challengers, perf)

	assert.GreaterOrEqual(t, result["champion"], ChampionAbsoluteFloor)
	assert.GreaterOrEqual(t, result["challenger1"], ChallengerMinBudget)
	assert.LessOrEqual(t, result["challenger1"], ChallengerMaxBudget)

	// Sum should be 1.0
	total := 0.0
	for _, v := range result {
		total += v
	}
	assert.InDelta(t, 1.0, total, 0.0001)
}

func TestBudgetAllocator_NewChallengerVirtualPrior(t *testing.T) {
	alloc := NewBudgetAllocator()
	challengers := []string{"new_challenger"}
	perf := map[string][]float64{} // No history

	result := alloc.Allocate("champion", challengers, perf)

	// New challenger should get some budget (not zero, not infinite)
	assert.GreaterOrEqual(t, result["new_challenger"], ChallengerMinBudget)
	assert.LessOrEqual(t, result["new_challenger"], ChallengerMaxBudget)
}

func TestBudgetAllocator_MaxChallengersLimit(t *testing.T) {
	alloc := NewBudgetAllocator()

	// Create 15 challengers (exceeds MAX_CHALLENGERS=10)
	challengers := make([]string, 15)
	perf := make(map[string][]float64)
	for i := 0; i < 15; i++ {
		challengers[i] = fmt.Sprintf("challenger%d", i)
		perf[challengers[i]] = []float64{float64(i) * 0.01} // Varying performance
	}

	result := alloc.Allocate("champion", challengers, perf)

	// Should only have MaxChallengers + 1 (champion)
	assert.LessOrEqual(t, len(result), MaxChallengers+1)

	// Champion should maintain floor
	assert.GreaterOrEqual(t, result["champion"], ChampionAbsoluteFloor)
}

func TestBudgetAllocator_WaterFillNoOrderBias(t *testing.T) {
	alloc := NewBudgetAllocator()

	// Two challengers with same performance
	challengers := []string{"A", "B"}
	perf := map[string][]float64{
		"A": {0.01, 0.02},
		"B": {0.01, 0.02},
	}

	result1 := alloc.Allocate("champion", challengers, perf)

	// Reverse order
	challengers = []string{"B", "A"}
	result2 := alloc.Allocate("champion", challengers, perf)

	// Both should get same allocation regardless of order
	assert.InDelta(t, result1["A"], result2["A"], 0.0001)
	assert.InDelta(t, result1["B"], result2["B"], 0.0001)
}

func TestBudgetAllocator_ChallengerMinMaxEnforced(t *testing.T) {
	alloc := NewBudgetAllocator()

	challengers := []string{"strong", "weak"}
	perf := map[string][]float64{
		"strong": {0.10, 0.10, 0.10}, // Very high performance
		"weak":   {-0.10, -0.10},     // Very low performance
	}

	result := alloc.Allocate("champion", challengers, perf)

	// Strong should not exceed max
	assert.LessOrEqual(t, result["strong"], ChallengerMaxBudget)

	// Weak should still get min
	assert.GreaterOrEqual(t, result["weak"], ChallengerMinBudget)
}
```

**Step 2: Run test**

```bash
go test ./backtest/risk/... -run TestBudgetAllocator -v
```

Expected: FAIL

**Step 3: Write implementation**

```go
// backtest/risk/budget.go
package risk

import (
	"math"
	"sort"
)

// BudgetAllocator implements UCB-based budget allocation with water-fill method.
type BudgetAllocator struct {
	explorationFactor float64
	priorNCycles      int
	priorMeanReturn   float64
}

// NewBudgetAllocator creates a new budget allocator.
func NewBudgetAllocator() *BudgetAllocator {
	return &BudgetAllocator{
		explorationFactor: 1.0,
		priorNCycles:      1,     // Virtual prior: n=1
		priorMeanReturn:   0.0,   // Virtual prior: mean=0
	}
}

// Allocate returns budget allocation for champion and challengers.
func (b *BudgetAllocator) Allocate(
	championID string,
	challengerIDs []string,
	historicalPerf map[string][]float64,
) BudgetAllocation {
	if len(challengerIDs) == 0 {
		return BudgetAllocation{championID: 1.0}
	}

	// Enforce MAX_CHALLENGERS limit
	challengers := b.selectTopChallengers(challengerIDs, historicalPerf)

	// Step 1: Champion gets min budget
	allocations := make(BudgetAllocation)
	allocations[championID] = ChampionMinBudget

	// Step 2: Each challenger gets min budget
	for _, cid := range challengers {
		allocations[cid] = ChallengerMinBudget
	}

	// Check if already over budget
	usedBudget := b.sumBudget(allocations)
	remainingBudget := 1.0 - usedBudget

	if remainingBudget < 0 {
		// Reduce champion to floor, recalculate
		shortfall := -remainingBudget
		newChampionBudget := math.Max(ChampionMinBudget-shortfall, ChampionAbsoluteFloor)
		allocations[championID] = newChampionBudget
		remainingBudget = 0
	}

	if remainingBudget <= 1e-9 {
		return b.normalize(allocations, championID)
	}

	// Step 3: Calculate UCB weights
	weights := b.calcUCBWeights(challengers, historicalPerf)

	// Step 4: Water-fill allocation
	headroom := make(map[string]float64)
	for _, cid := range challengers {
		headroom[cid] = ChallengerMaxBudget - allocations[cid]
	}

	maxIterations := len(challengers) + 1
	for iter := 0; iter < maxIterations && remainingBudget > 1e-9; iter++ {
		// Find eligible challengers (have headroom)
		eligible := make([]string, 0)
		for _, cid := range challengers {
			if headroom[cid] > 1e-9 {
				eligible = append(eligible, cid)
			}
		}
		if len(eligible) == 0 {
			break
		}

		// Snapshot budget for this round (v1.1.2: avoid order bias)
		budgetSnapshot := remainingBudget

		// Calculate normalized weights for eligible
		eligibleWeightSum := 0.0
		for _, cid := range eligible {
			eligibleWeightSum += weights[cid]
		}

		// Distribute proportionally
		adds := make(map[string]float64)
		if eligibleWeightSum <= 1e-9 {
			// Equal distribution
			share := budgetSnapshot / float64(len(eligible))
			for _, cid := range eligible {
				adds[cid] = math.Min(share, headroom[cid])
			}
		} else {
			for _, cid := range eligible {
				normalizedWeight := weights[cid] / eligibleWeightSum
				desired := budgetSnapshot * normalizedWeight
				adds[cid] = math.Min(desired, headroom[cid])
			}
		}

		// Apply adds
		totalAdded := 0.0
		for cid, add := range adds {
			allocations[cid] += add
			headroom[cid] -= add
			totalAdded += add
		}
		remainingBudget -= totalAdded
	}

	// Step 5: Normalize and ensure champion floor
	return b.normalize(allocations, championID)
}

// selectTopChallengers returns top MaxChallengers by UCB score.
func (b *BudgetAllocator) selectTopChallengers(
	challengerIDs []string,
	historicalPerf map[string][]float64,
) []string {
	if len(challengerIDs) <= MaxChallengers {
		return challengerIDs
	}

	// Calculate UCB scores
	type scored struct {
		id    string
		score float64
	}
	scores := make([]scored, len(challengerIDs))
	totalCycles := b.totalCycles(historicalPerf)

	for i, cid := range challengerIDs {
		perf := historicalPerf[cid]
		nCycles := b.priorNCycles
		meanReturn := b.priorMeanReturn

		if len(perf) > 0 {
			nCycles = len(perf)
			meanReturn = b.mean(perf)
		}

		explorationBonus := b.explorationFactor * math.Sqrt(
			math.Log(float64(totalCycles)+1)/float64(nCycles),
		)
		scores[i] = scored{id: cid, score: meanReturn + explorationBonus}
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Return top MaxChallengers
	result := make([]string, MaxChallengers)
	for i := 0; i < MaxChallengers; i++ {
		result[i] = scores[i].id
	}
	return result
}

// calcUCBWeights calculates softmax weights from UCB scores.
func (b *BudgetAllocator) calcUCBWeights(
	challengers []string,
	historicalPerf map[string][]float64,
) map[string]float64 {
	totalCycles := b.totalCycles(historicalPerf)

	// Calculate UCB scores
	scores := make([]float64, len(challengers))
	maxScore := math.Inf(-1)

	for i, cid := range challengers {
		perf := historicalPerf[cid]
		nCycles := b.priorNCycles
		meanReturn := b.priorMeanReturn

		if len(perf) > 0 {
			nCycles = len(perf)
			meanReturn = b.mean(perf)
		}

		explorationBonus := b.explorationFactor * math.Sqrt(
			math.Log(float64(totalCycles)+1)/float64(nCycles),
		)
		scores[i] = meanReturn + explorationBonus

		// Clip to [-5, 5]
		scores[i] = math.Max(-5, math.Min(5, scores[i]))

		if scores[i] > maxScore {
			maxScore = scores[i]
		}
	}

	// Softmax
	expSum := 0.0
	expScores := make([]float64, len(scores))
	for i, s := range scores {
		expScores[i] = math.Exp(s - maxScore) // subtract max for numerical stability
		expSum += expScores[i]
	}

	weights := make(map[string]float64)
	for i, cid := range challengers {
		weights[cid] = expScores[i] / expSum
	}
	return weights
}

// normalize ensures sum=1 and champion >= floor.
func (b *BudgetAllocator) normalize(alloc BudgetAllocation, championID string) BudgetAllocation {
	total := b.sumBudget(alloc)
	if total <= 0 {
		return alloc
	}

	// First normalize
	for k := range alloc {
		alloc[k] /= total
	}

	// Check champion floor
	if alloc[championID] < ChampionAbsoluteFloor {
		alloc[championID] = ChampionAbsoluteFloor

		// Rescale challengers
		challengerTotal := 1.0 - ChampionAbsoluteFloor
		challengerSum := 0.0
		for k, v := range alloc {
			if k != championID {
				challengerSum += v
			}
		}

		if challengerSum > 0 {
			scale := challengerTotal / challengerSum
			for k := range alloc {
				if k != championID {
					alloc[k] *= scale
					// Re-enforce min/max after scaling
					alloc[k] = math.Max(ChallengerMinBudget, math.Min(ChallengerMaxBudget, alloc[k]))
				}
			}
		}

		// Final normalize to ensure sum=1
		total = b.sumBudget(alloc)
		for k := range alloc {
			alloc[k] /= total
		}
	}

	return alloc
}

func (b *BudgetAllocator) sumBudget(alloc BudgetAllocation) float64 {
	sum := 0.0
	for _, v := range alloc {
		sum += v
	}
	return sum
}

func (b *BudgetAllocator) totalCycles(perf map[string][]float64) int {
	total := 0
	for _, v := range perf {
		total += len(v)
	}
	return max(total, 1)
}

func (b *BudgetAllocator) mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

**Step 4: Run tests**

```bash
go test ./backtest/risk/... -run TestBudgetAllocator -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backtest/risk/budget.go backtest/risk/budget_test.go
git commit -m "feat(risk): add UCB budget allocator with water-fill method

- Virtual prior (n=1, mean=0) for new challengers
- MAX_CHALLENGERS=10 limit with top-K selection
- Water-fill with budget snapshot (no order bias)
- Champion absolute floor 0.40 with re-clamping"
```

---

## Task 6: Gates Implementation (Risk Parity, Dominance, Evidence)

**Files:**
- Create: `backtest/risk/gates.go`
- Test: `backtest/risk/gates_test.go`

**Step 1: Write failing test**

```go
// backtest/risk/gates_test.go
package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRiskParityGate_Pass(t *testing.T) {
	champion := &StrategyResults{
		RiskUsedTS: []float64{0.5, 0.6, 0.7, 0.8, 0.6, 0.5},
		LeverageTS: []float64{2.0, 2.5, 2.0, 2.5, 2.0, 2.0},
	}
	challenger := &StrategyResults{
		RiskUsedTS: []float64{0.55, 0.65, 0.75, 0.85, 0.65, 0.55},
		LeverageTS: []float64{2.1, 2.6, 2.1, 2.6, 2.1, 2.1},
	}

	result := CheckRiskParityGate(champion, challenger, 1.0, 1.0)
	assert.True(t, result.Passed)
}

func TestRiskParityGate_FailRiskDeviation(t *testing.T) {
	champion := &StrategyResults{
		RiskUsedTS: []float64{0.5, 0.5, 0.5},
		LeverageTS: []float64{2.0, 2.0, 2.0},
	}
	challenger := &StrategyResults{
		RiskUsedTS: []float64{0.9, 0.9, 0.9}, // Much higher
		LeverageTS: []float64{2.0, 2.0, 2.0},
	}

	result := CheckRiskParityGate(champion, challenger, 1.0, 1.0)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "RP-1")
}

func TestDominanceGate_Pass(t *testing.T) {
	champion := &StrategyResults{
		NetPnL:       1000,
		Sharpe:       1.0,
		ProfitFactor: 1.5,
		Calmar:       2.0,
		WinRate:      0.55,
		ES95:         100,
		MaxDrawdown:  0.10,
		TradesCount:  50,
	}
	challenger := &StrategyResults{
		NetPnL:       1200, // > 1000 * 1.05
		Sharpe:       1.2,  // > 1.0 + 0.1
		ProfitFactor: 1.6,  // > 1.5
		Calmar:       2.2,  // > 2.0
		WinRate:      0.58, // > 0.55
		ES95:         105,  // <= 100 * 1.1
		MaxDrawdown:  0.09, // <= 0.10 * 1.05
		TradesCount:  50,
	}

	result := CheckDominanceGate(champion, challenger)
	assert.True(t, result.Passed)
	assert.Equal(t, 5, result.Details["wins"])
}

func TestDominanceGate_FailES95(t *testing.T) {
	champion := &StrategyResults{
		NetPnL:       1000,
		Sharpe:       1.0,
		ProfitFactor: 1.5,
		Calmar:       2.0,
		WinRate:      0.55,
		ES95:         100,
		MaxDrawdown:  0.10,
		TradesCount:  50,
	}
	challenger := &StrategyResults{
		NetPnL:       1200,
		Sharpe:       1.2,
		ProfitFactor: 1.6,
		Calmar:       2.2,
		WinRate:      0.58,
		ES95:         150, // > 100 * 1.1 = FAIL
		MaxDrawdown:  0.09,
		TradesCount:  50,
	}

	result := CheckDominanceGate(champion, challenger)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "ES95")
}

func TestEvidenceGate_InsufficientSample(t *testing.T) {
	challenger := &StrategyResults{
		TradesCount: 20, // < MinTrades
		ActiveDays:  10, // < MinActiveDays
	}

	result := CheckEvidenceGate(nil, challenger, nil)
	assert.False(t, result.Passed)
	assert.Equal(t, "INSUFFICIENT_SAMPLE", result.Reason)
}

func TestEvidenceGate_SegmentRobustness(t *testing.T) {
	// 40 days of data, challenger wins 3/4 segments
	championDaily := make(map[string]float64)
	challengerDaily := make(map[string]float64)

	for i := 0; i < 40; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		championDaily[date] = 100.0
		if i < 30 { // First 3 segments challenger wins
			challengerDaily[date] = 120.0
		} else { // Last segment champion wins
			challengerDaily[date] = 80.0
		}
	}

	champion := &StrategyResults{DailyPnL: championDaily}
	challenger := &StrategyResults{
		DailyPnL:    challengerDaily,
		TradesCount: 50,
		ActiveDays:  40,
	}

	result := CheckEvidenceGate(champion, challenger, nil)
	// Should pass segment robustness (3/4 positive)
	assert.NotEqual(t, "SEGMENT_ROBUSTNESS_FAILED", result.Reason)
}
```

**Step 2: Run test**

```bash
go test ./backtest/risk/... -run TestGate -v
```

Expected: FAIL

**Step 3: Write implementation**

```go
// backtest/risk/gates.go
package risk

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// CheckRiskParityGate checks P95 risk usage deviation.
func CheckRiskParityGate(
	champion, challenger *StrategyResults,
	champBudget, challBudget float64,
) GateResult {
	// RP-1: P95 risk usage ratio deviation
	champRiskP95 := percentile(champion.RiskUsedTS, 95)
	challRiskP95 := percentile(challenger.RiskUsedTS, 95)

	champRatio := champRiskP95 / champBudget
	challRatio := challRiskP95 / challBudget

	if champRatio > 0 {
		deviation := math.Abs(challRatio-champRatio) / champRatio
		if deviation > RiskDeviationThreshold {
			return GateResult{
				Passed: false,
				Gate:   "RiskParity",
				Reason: fmt.Sprintf("RP-1: P95 risk ratio deviation %.1f%% > 20%%", deviation*100),
				Details: map[string]interface{}{
					"champ_ratio": champRatio,
					"chall_ratio": challRatio,
					"deviation":   deviation,
				},
			}
		}
	}

	// RP-2: P95 leverage deviation
	champLevP95 := percentile(champion.LeverageTS, 95)
	challLevP95 := percentile(challenger.LeverageTS, 95)
	levDiff := math.Abs(challLevP95 - champLevP95)

	if levDiff > LeverageDeviationMax {
		return GateResult{
			Passed: false,
			Gate:   "RiskParity",
			Reason: fmt.Sprintf("RP-2: P95 leverage diff %.1f > 1.0", levDiff),
			Details: map[string]interface{}{
				"champ_leverage": champLevP95,
				"chall_leverage": challLevP95,
			},
		}
	}

	return GateResult{Passed: true, Gate: "RiskParity"}
}

// CheckDominanceGate checks scoring metrics and constraint metrics.
func CheckDominanceGate(champion, challenger *StrategyResults) GateResult {
	wins := 0
	details := make(map[string]interface{})

	// Scoring metrics (5 items)
	// S-1: Net PnL (B > A * 1.05)
	if challenger.NetPnL > champion.NetPnL*1.05 {
		wins++
		details["net_pnl"] = true
	}

	// S-2: Sharpe (B > A + 0.1)
	if challenger.Sharpe > champion.Sharpe+0.1 {
		wins++
		details["sharpe"] = true
	}

	// S-3: Profit Factor (B > A)
	if challenger.ProfitFactor > champion.ProfitFactor {
		wins++
		details["profit_factor"] = true
	}

	// S-4: Calmar (B > A)
	if challenger.Calmar > champion.Calmar {
		wins++
		details["calmar"] = true
	}

	// S-5: Win Rate (when trades >= 20)
	if challenger.TradesCount >= 20 && champion.TradesCount >= 20 {
		if challenger.WinRate > champion.WinRate {
			wins++
			details["win_rate"] = true
		}
	}

	details["wins"] = wins

	// Constraint metrics (2 items, hard threshold)
	// C-1: ES95 <= A * 1.1
	es95OK := challenger.ES95 <= champion.ES95*1.1
	details["es95_ok"] = es95OK

	if !es95OK {
		return GateResult{
			Passed:  false,
			Gate:    "Dominance",
			Reason:  fmt.Sprintf("ES95 %.2f > %.2f (threshold)", challenger.ES95, champion.ES95*1.1),
			Details: details,
		}
	}

	// C-2: MaxDD with Calmar compensation
	maxDDOK := false
	if challenger.MaxDrawdown <= champion.MaxDrawdown*1.05 {
		maxDDOK = true
	} else if challenger.MaxDrawdown <= champion.MaxDrawdown*1.10 {
		// Allow 5-10% worse if Calmar compensates
		maxDDOK = challenger.Calmar >= champion.Calmar
	}
	details["maxdd_ok"] = maxDDOK

	if !maxDDOK {
		return GateResult{
			Passed:  false,
			Gate:    "Dominance",
			Reason:  fmt.Sprintf("MaxDD %.2f%% too high without Calmar compensation", challenger.MaxDrawdown*100),
			Details: details,
		}
	}

	// Final check: wins >= 3
	if wins < DominanceWinsRequired {
		return GateResult{
			Passed:  false,
			Gate:    "Dominance",
			Reason:  fmt.Sprintf("Only %d wins, need %d", wins, DominanceWinsRequired),
			Details: details,
		}
	}

	return GateResult{Passed: true, Gate: "Dominance", Details: details}
}

// CheckEvidenceGate checks statistical significance.
func CheckEvidenceGate(
	champion, challenger *StrategyResults,
	historicalCycles []ABTestCycle,
) GateResult {
	// E-1: Minimum sample
	if challenger.TradesCount < MinTrades && challenger.ActiveDays < MinActiveDays {
		return GateResult{
			Passed: false,
			Gate:   "Evidence",
			Reason: "INSUFFICIENT_SAMPLE",
		}
	}

	// E-2: Segment robustness
	if champion != nil && len(champion.DailyPnL) > 0 && len(challenger.DailyPnL) > 0 {
		segmentOK, positiveCount, nSegments := checkSegmentRobustness(
			champion.DailyPnL, challenger.DailyPnL,
		)
		if !segmentOK {
			return GateResult{
				Passed: false,
				Gate:   "Evidence",
				Reason: "SEGMENT_ROBUSTNESS_FAILED",
				Details: map[string]interface{}{
					"positive_segments": positiveCount,
					"total_segments":    nSegments,
				},
			}
		}
	}

	// E-3: Bootstrap (daily PnL difference)
	if champion != nil {
		bootstrapOK, ciLower := checkBootstrap(champion.DailyPnL, challenger.DailyPnL)
		if !bootstrapOK {
			return GateResult{
				Passed: false,
				Gate:   "Evidence",
				Reason: "BOOTSTRAP_FAILED",
				Details: map[string]interface{}{
					"ci_lower": ciLower,
				},
			}
		}
	}

	// E-4: Regime diversity
	if len(historicalCycles) >= 3 {
		diversityOK := checkRegimeDiversity(challenger.ID, historicalCycles)
		if !diversityOK {
			return GateResult{
				Passed: false,
				Gate:   "Evidence",
				Reason: "REGIME_DIVERSITY_FAILED",
			}
		}
	}

	return GateResult{Passed: true, Gate: "Evidence"}
}

// checkSegmentRobustness checks if challenger wins enough segments.
func checkSegmentRobustness(champDaily, challDaily map[string]float64) (bool, int, int) {
	aligned := alignDailyPnL(champDaily, challDaily)
	nDays := len(aligned)

	if nDays < MinDaysForSegment {
		return false, 0, 0
	}

	var nSegments, required int
	if nDays >= MinDaysFor4Segments {
		nSegments = 4
		required = 3
	} else {
		nSegments = 2
		required = 2
	}

	segmentSize := nDays / nSegments
	positiveCount := 0

	for i := 0; i < nSegments; i++ {
		startIdx := i * segmentSize
		endIdx := startIdx + segmentSize
		if i == nSegments-1 {
			endIdx = nDays
		}

		champSum, challSum := 0.0, 0.0
		for j := startIdx; j < endIdx; j++ {
			champSum += aligned[j].champPnL
			challSum += aligned[j].challPnL
		}

		if challSum > champSum {
			positiveCount++
		}
	}

	return positiveCount >= required, positiveCount, nSegments
}

type alignedDay struct {
	date     string
	champPnL float64
	challPnL float64
}

func alignDailyPnL(champ, chall map[string]float64) []alignedDay {
	// Find common dates
	commonDates := make([]string, 0)
	for date := range champ {
		if _, ok := chall[date]; ok {
			commonDates = append(commonDates, date)
		}
	}
	sort.Strings(commonDates)

	result := make([]alignedDay, len(commonDates))
	for i, date := range commonDates {
		result[i] = alignedDay{
			date:     date,
			champPnL: champ[date],
			challPnL: chall[date],
		}
	}
	return result
}

// checkBootstrap performs bootstrap test on daily PnL difference.
func checkBootstrap(champDaily, challDaily map[string]float64) (bool, float64) {
	aligned := alignDailyPnL(champDaily, challDaily)
	if len(aligned) < MinDaysForSegment {
		return false, 0
	}

	// Daily PnL difference
	diffs := make([]float64, len(aligned))
	for i, day := range aligned {
		diffs[i] = day.challPnL - day.champPnL
	}

	// Bootstrap 1000 times
	nBootstrap := 1000
	means := make([]float64, nBootstrap)
	n := len(diffs)

	for b := 0; b < nBootstrap; b++ {
		sum := 0.0
		for i := 0; i < n; i++ {
			idx := rand.Intn(n)
			sum += diffs[idx]
		}
		means[b] = sum / float64(n)
	}

	sort.Float64s(means)
	ciLower := percentile(means, 2.5)

	return ciLower > 0, ciLower
}

// checkRegimeDiversity checks if challenger won in multiple regimes.
func checkRegimeDiversity(challengerID string, cycles []ABTestCycle) bool {
	// Find consecutive wins for this challenger
	consecutiveWins := make([]ABTestCycle, 0)
	for i := len(cycles) - 1; i >= 0; i-- {
		if cycles[i].Winner == challengerID {
			consecutiveWins = append([]ABTestCycle{cycles[i]}, consecutiveWins...)
		} else {
			break
		}
	}

	if len(consecutiveWins) < 3 {
		return false
	}

	// Check regime diversity in last 3 wins
	regimes := make(map[string]bool)
	for i := len(consecutiveWins) - 3; i < len(consecutiveWins); i++ {
		regimes[consecutiveWins[i].RegimeSummary.PrimaryRegime] = true
	}

	return len(regimes) >= 2
}

// percentile calculates the p-th percentile of sorted data.
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	idx := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))

	if lower == upper {
		return sorted[lower]
	}

	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
```

**Step 4: Add missing import and run tests**

```bash
go test ./backtest/risk/... -run TestGate -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backtest/risk/gates.go backtest/risk/gates_test.go
git commit -m "feat(risk): add Risk Parity, Dominance, and Evidence gates

- RiskParityGate: P95 risk ratio and leverage deviation
- DominanceGate: 5 scoring + 2 constraint metrics separated
- EvidenceGate: sample size, segment robustness, bootstrap, regime diversity
- Daily PnL alignment for fair comparison"
```

---

## Task 7: A/B Test Cycle Manager

**Files:**
- Create: `backtest/abtest/cycle.go`
- Create: `backtest/abtest/manager.go`
- Test: `backtest/abtest/manager_test.go`

**Step 1: Create directory and write types**

```bash
mkdir -p /mnt/c/Users/david/nofx/backtest/abtest
```

```go
// backtest/abtest/cycle.go
package abtest

import (
	"nofx/backtest/risk"
	"time"
)

// CycleConfig configures A/B test cycle parameters.
type CycleConfig struct {
	CycleDuration     time.Duration // How long each cycle runs
	MinCyclesForPromo int           // Minimum cycles before promotion
}

// DefaultCycleConfig returns sensible defaults.
func DefaultCycleConfig() CycleConfig {
	return CycleConfig{
		CycleDuration:     24 * time.Hour, // 1 day cycles
		MinCyclesForPromo: 3,
	}
}

// CycleState tracks current cycle state.
type CycleState struct {
	CurrentCycle    *risk.ABTestCycle
	HistoricalCycles []risk.ABTestCycle
	ChampionID      string
	ChallengerIDs   []string
	BudgetAlloc     risk.BudgetAllocation
}

// NewCycleState creates initial state with a champion.
func NewCycleState(championID string) *CycleState {
	return &CycleState{
		ChampionID:       championID,
		ChallengerIDs:    make([]string, 0),
		HistoricalCycles: make([]risk.ABTestCycle, 0),
		BudgetAlloc:      risk.BudgetAllocation{championID: 1.0},
	}
}

// AddChallenger adds a new challenger strategy.
func (cs *CycleState) AddChallenger(id string) {
	for _, cid := range cs.ChallengerIDs {
		if cid == id {
			return // Already exists
		}
	}
	cs.ChallengerIDs = append(cs.ChallengerIDs, id)
}

// RemoveChallenger removes a challenger strategy.
func (cs *CycleState) RemoveChallenger(id string) {
	newList := make([]string, 0, len(cs.ChallengerIDs))
	for _, cid := range cs.ChallengerIDs {
		if cid != id {
			newList = append(newList, cid)
		}
	}
	cs.ChallengerIDs = newList
}

// PromoteChallenger promotes a challenger to champion.
func (cs *CycleState) PromoteChallenger(id string) {
	cs.RemoveChallenger(id)
	oldChampion := cs.ChampionID
	cs.ChampionID = id
	// Old champion becomes challenger
	cs.ChallengerIDs = append(cs.ChallengerIDs, oldChampion)
}
```

**Step 2: Write manager implementation**

```go
// backtest/abtest/manager.go
package abtest

import (
	"fmt"
	"nofx/backtest/risk"
	"time"

	"github.com/google/uuid"
)

// Manager orchestrates Champion-Challenger A/B testing.
type Manager struct {
	config         CycleConfig
	state          *CycleState
	riskCalc       *risk.PortfolioRiskCalculator
	budgetAlloc    *risk.BudgetAllocator
	regimeCalc     *risk.RegimeCalculator

	// Performance tracking
	strategyPerf   map[string][]float64 // strategy_id -> cycle returns
}

// NewManager creates a new A/B test manager.
func NewManager(config CycleConfig, championID string) *Manager {
	return &Manager{
		config:       config,
		state:        NewCycleState(championID),
		riskCalc:     risk.NewPortfolioRiskCalculator(risk.TargetPortfolioVol),
		budgetAlloc:  risk.NewBudgetAllocator(),
		regimeCalc:   risk.NewRegimeCalculator(),
		strategyPerf: make(map[string][]float64),
	}
}

// GetBudgetAllocation returns current risk budget allocation.
func (m *Manager) GetBudgetAllocation() risk.BudgetAllocation {
	return m.state.BudgetAlloc
}

// UpdateBudgets recalculates budget allocation based on performance.
func (m *Manager) UpdateBudgets() {
	m.state.BudgetAlloc = m.budgetAlloc.Allocate(
		m.state.ChampionID,
		m.state.ChallengerIDs,
		m.strategyPerf,
	)
}

// StartCycle begins a new A/B test cycle.
func (m *Manager) StartCycle(regime risk.RegimeSummary) {
	cycle := &risk.ABTestCycle{
		ID:            uuid.New().String(),
		StartTime:     time.Now(),
		ChampionID:    m.state.ChampionID,
		ChallengerIDs: append([]string{}, m.state.ChallengerIDs...),
		Results:       make(map[string]risk.StrategyResults),
		RegimeSummary: regime,
	}
	m.state.CurrentCycle = cycle
	m.UpdateBudgets()
}

// EndCycle completes the current cycle and runs gate checks.
func (m *Manager) EndCycle(results map[string]risk.StrategyResults) (*CycleResult, error) {
	if m.state.CurrentCycle == nil {
		return nil, fmt.Errorf("no active cycle")
	}

	cycle := m.state.CurrentCycle
	cycle.EndTime = time.Now()
	cycle.Results = results

	// Determine winner and run gates
	cycleResult := m.evaluateCycle(cycle)

	// Update historical performance
	for id, res := range results {
		cycleReturn := res.NetPnL / 10000 // Normalize to return
		m.strategyPerf[id] = append(m.strategyPerf[id], cycleReturn)
	}

	// Add to history
	m.state.HistoricalCycles = append(m.state.HistoricalCycles, *cycle)
	m.state.CurrentCycle = nil

	return cycleResult, nil
}

// CycleResult holds the outcome of a cycle evaluation.
type CycleResult struct {
	CycleID          string
	Winner           string
	ShouldPromote    bool
	PromotionCandidate string
	GateResults      []risk.GateResult
}

// evaluateCycle runs all gates and determines winner.
func (m *Manager) evaluateCycle(cycle *risk.ABTestCycle) *CycleResult {
	result := &CycleResult{
		CycleID:     cycle.ID,
		GateResults: make([]risk.GateResult, 0),
	}

	champResults := cycle.Results[cycle.ChampionID]
	budgets := m.state.BudgetAlloc

	// Find best challenger
	var bestChallenger string
	var bestChallengerResults *risk.StrategyResults

	for _, challID := range cycle.ChallengerIDs {
		challResults, ok := cycle.Results[challID]
		if !ok {
			continue
		}

		// Run Risk Parity Gate
		rpResult := risk.CheckRiskParityGate(
			&champResults, &challResults,
			budgets[cycle.ChampionID], budgets[challID],
		)
		result.GateResults = append(result.GateResults, rpResult)
		if !rpResult.Passed {
			continue
		}

		// Run Dominance Gate
		domResult := risk.CheckDominanceGate(&champResults, &challResults)
		result.GateResults = append(result.GateResults, domResult)
		if !domResult.Passed {
			continue
		}

		// This challenger passed both gates
		if bestChallengerResults == nil || challResults.NetPnL > bestChallengerResults.NetPnL {
			bestChallenger = challID
			bestChallengerResults = &challResults
		}
	}

	// Determine winner
	if bestChallenger != "" {
		result.Winner = bestChallenger
		cycle.Winner = bestChallenger

		// Run Evidence Gate for promotion check
		evResult := risk.CheckEvidenceGate(
			&champResults, bestChallengerResults,
			m.state.HistoricalCycles,
		)
		result.GateResults = append(result.GateResults, evResult)

		if evResult.Passed {
			result.ShouldPromote = true
			result.PromotionCandidate = bestChallenger
		}
	} else {
		result.Winner = cycle.ChampionID
		cycle.Winner = cycle.ChampionID
	}

	return result
}

// Promote executes a challenger promotion.
func (m *Manager) Promote(challengerID string) error {
	found := false
	for _, cid := range m.state.ChallengerIDs {
		if cid == challengerID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("challenger %s not found", challengerID)
	}

	m.state.PromoteChallenger(challengerID)
	m.UpdateBudgets()
	return nil
}

// AddChallenger registers a new challenger strategy.
func (m *Manager) AddChallenger(id string) {
	m.state.AddChallenger(id)
	m.UpdateBudgets()
}

// GetState returns current cycle state.
func (m *Manager) GetState() *CycleState {
	return m.state
}

// GetRiskCalculator returns the portfolio risk calculator.
func (m *Manager) GetRiskCalculator() *risk.PortfolioRiskCalculator {
	return m.riskCalc
}

// GetRegimeCalculator returns the regime calculator.
func (m *Manager) GetRegimeCalculator() *risk.RegimeCalculator {
	return m.regimeCalc
}
```

**Step 3: Write tests**

```go
// backtest/abtest/manager_test.go
package abtest

import (
	"nofx/backtest/risk"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_InitialState(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")

	state := m.GetState()
	assert.Equal(t, "champion", state.ChampionID)
	assert.Empty(t, state.ChallengerIDs)
	assert.Equal(t, 1.0, state.BudgetAlloc["champion"])
}

func TestManager_AddChallenger(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	m.AddChallenger("challenger1")

	state := m.GetState()
	assert.Contains(t, state.ChallengerIDs, "challenger1")
	assert.Greater(t, state.BudgetAlloc["champion"], 0.0)
	assert.Greater(t, state.BudgetAlloc["challenger1"], 0.0)
}

func TestManager_CycleLifecycle(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	m.AddChallenger("challenger1")

	regime := risk.RegimeSummary{PrimaryRegime: "mid_trending"}
	m.StartCycle(regime)

	require.NotNil(t, m.GetState().CurrentCycle)
	assert.Equal(t, "mid_trending", m.GetState().CurrentCycle.RegimeSummary.PrimaryRegime)

	results := map[string]risk.StrategyResults{
		"champion": {
			ID:           "champion",
			NetPnL:       1000,
			Sharpe:       1.0,
			ProfitFactor: 1.5,
			Calmar:       2.0,
			WinRate:      0.55,
			ES95:         100,
			MaxDrawdown:  0.10,
			TradesCount:  50,
			RiskUsedTS:   []float64{0.5, 0.6, 0.5},
			LeverageTS:   []float64{2.0, 2.0, 2.0},
		},
		"challenger1": {
			ID:           "challenger1",
			NetPnL:       900,
			Sharpe:       0.9,
			ProfitFactor: 1.4,
			Calmar:       1.8,
			WinRate:      0.50,
			ES95:         110,
			MaxDrawdown:  0.12,
			TradesCount:  45,
			RiskUsedTS:   []float64{0.55, 0.65, 0.55},
			LeverageTS:   []float64{2.1, 2.1, 2.1},
		},
	}

	cycleResult, err := m.EndCycle(results)
	require.NoError(t, err)

	// Champion should win (challenger underperformed)
	assert.Equal(t, "champion", cycleResult.Winner)
	assert.False(t, cycleResult.ShouldPromote)

	// Cycle should be archived
	assert.Nil(t, m.GetState().CurrentCycle)
	assert.Len(t, m.GetState().HistoricalCycles, 1)
}

func TestManager_Promotion(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	m.AddChallenger("challenger1")

	err := m.Promote("challenger1")
	require.NoError(t, err)

	state := m.GetState()
	assert.Equal(t, "challenger1", state.ChampionID)
	assert.Contains(t, state.ChallengerIDs, "champion") // Old champion is now challenger
}
```

**Step 4: Run tests**

```bash
go test ./backtest/abtest/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backtest/abtest/
git commit -m "feat(abtest): add Champion-Challenger A/B test manager

- CycleState tracks champion, challengers, and budget allocation
- Manager orchestrates cycle lifecycle
- Gate checks run in order: RiskParity → Dominance → Evidence
- Promotion logic with state updates"
```

---

## Task 8: Integration with Backtest Runner

**Files:**
- Modify: `backtest/runner.go`
- Modify: `backtest/types.go`
- Test: `backtest/runner_abtest_test.go`

This task integrates the A/B test framework with the existing backtest runner. The integration is optional (enabled via config flag).

**Step 1: Add config fields**

```go
// In backtest/config.go, add to BacktestConfig struct:

// A/B Test configuration
ABTestEnabled     bool     `json:"abtest_enabled"`
ABTestChampionID  string   `json:"abtest_champion_id"`
ABTestChallengerIDs []string `json:"abtest_challenger_ids"`
```

**Step 2: Add A/B test tracking to Runner**

```go
// In backtest/runner.go, add to Runner struct:

import "nofx/backtest/abtest"

// Add field:
abtestManager *abtest.Manager
```

**Step 3: Initialize in NewRunner**

```go
// In NewRunner function, after existing initialization:

if cfg.ABTestEnabled && cfg.ABTestChampionID != "" {
    r.abtestManager = abtest.NewManager(
        abtest.DefaultCycleConfig(),
        cfg.ABTestChampionID,
    )
    for _, cid := range cfg.ABTestChallengerIDs {
        r.abtestManager.AddChallenger(cid)
    }
}
```

**Step 4: Write integration test**

```go
// backtest/runner_abtest_test.go
package backtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunner_ABTestIntegration(t *testing.T) {
	// This is a placeholder for integration testing
	// Full integration requires mock data feed and MCP client

	cfg := BacktestConfig{
		ABTestEnabled:       true,
		ABTestChampionID:    "strategy_a",
		ABTestChallengerIDs: []string{"strategy_b"},
	}

	assert.True(t, cfg.ABTestEnabled)
	assert.Equal(t, "strategy_a", cfg.ABTestChampionID)
}
```

**Step 5: Commit**

```bash
git add backtest/config.go backtest/runner.go backtest/runner_abtest_test.go
git commit -m "feat(backtest): integrate A/B test framework with runner

- ABTestEnabled config flag
- Manager initialization in NewRunner
- Placeholder for full integration testing"
```

---

## Task 9: Documentation Update

**Files:**
- Update: `docs/architecture/CHAMPION_CHALLENGER_RISK_SPEC.md`

**Step 1: Add implementation reference section**

Add to the end of the spec document:

```markdown
---

## Implementation Reference

The v1.1.2 specification is implemented in the following Go packages:

### Package Structure

```
backtest/
├── risk/
│   ├── types.go          # Core types (Position, StrategyResults, etc.)
│   ├── correlation.go    # Correlation matrix with symbol order tracking
│   ├── portfolio.go      # Dual-metric risk calculator
│   ├── regime.go         # Regime calculator with hard thresholds
│   ├── budget.go         # UCB budget allocator with water-fill
│   └── gates.go          # Risk Parity, Dominance, Evidence gates
├── abtest/
│   ├── cycle.go          # Cycle state management
│   └── manager.go        # A/B test orchestration
```

### Usage Example

```go
import (
    "nofx/backtest/abtest"
    "nofx/backtest/risk"
)

// Create manager
mgr := abtest.NewManager(abtest.DefaultCycleConfig(), "champion_strategy")
mgr.AddChallenger("challenger_v2")

// Get budget allocation
budgets := mgr.GetBudgetAllocation()
// budgets["champion_strategy"] = 0.50+
// budgets["challenger_v2"] = 0.05-0.25

// Start cycle
regime := risk.RegimeSummary{PrimaryRegime: "mid_trending"}
mgr.StartCycle(regime)

// ... run strategies with allocated budgets ...

// End cycle with results
results := map[string]risk.StrategyResults{...}
cycleResult, _ := mgr.EndCycle(results)

if cycleResult.ShouldPromote {
    mgr.Promote(cycleResult.PromotionCandidate)
}
```
```

**Step 2: Commit**

```bash
git add docs/architecture/CHAMPION_CHALLENGER_RISK_SPEC.md
git commit -m "docs: add implementation reference to risk spec"
```

---

## Verification

### Run All Tests

```bash
cd /mnt/c/Users/david/nofx
go test ./backtest/risk/... ./backtest/abtest/... -v -cover
```

Expected: All tests PASS with >80% coverage

### Verify Build

```bash
go build ./...
```

Expected: No errors

### Manual Verification Checklist

- [ ] Types compile correctly with proper JSON tags
- [ ] Correlation matrix handles symbol reordering
- [ ] Portfolio risk fast/accurate use same volatility source
- [ ] Budget allocator respects MAX_CHALLENGERS
- [ ] Gates return proper GateResult with details
- [ ] Manager tracks cycle history correctly

---

## Summary

| Task | Component | Key Files |
|------|-----------|-----------|
| 1 | Core Types | `risk/types.go` |
| 2 | Correlation Matrix | `risk/correlation.go` |
| 3 | Portfolio Risk | `risk/portfolio.go` |
| 4 | Regime Calculator | `risk/regime.go` |
| 5 | Budget Allocator | `risk/budget.go` |
| 6 | Gates | `risk/gates.go` |
| 7 | A/B Test Manager | `abtest/manager.go` |
| 8 | Runner Integration | `runner.go` |
| 9 | Documentation | `CHAMPION_CHALLENGER_RISK_SPEC.md` |

Total: ~1500 lines of Go code + tests
