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
	fallbackVols map[string]float64
	fallbackMu   sync.RWMutex
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
	// Check if symbol exists in correlation matrix by checking if it's in the symbol list
	p.corrMatrix.mu.RLock()
	if vol, ok := p.corrMatrix.volatilities[symbol]; ok {
		p.corrMatrix.mu.RUnlock()
		return vol
	}
	p.corrMatrix.mu.RUnlock()

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
// Formula: Sigma(notional * vol * leverage) / equity / target_vol
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
// Formula: sigma_p = sqrt(r^T * Sigma * r) where Sigma = diag(sigma) * C * diag(sigma)
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
	// r_i = (notional / equity) * leverage * direction
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

	// Build covariance matrix: Sigma = diag(sigma) * C * diag(sigma)
	cov := make([][]float64, n)
	for i := 0; i < n; i++ {
		cov[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			cov[i][j] = sigmas[i] * C[i][j] * sigmas[j]
		}
	}

	// Portfolio variance: r^T * Sigma * r
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
