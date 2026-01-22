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

	// Set up volatilities via UpdateCorrelation
	// We need returns that produce specific volatilities
	// For vol = std * sqrt(24), if we want 2% daily vol, hourly std = 0.02/sqrt(24) = ~0.00408
	// For vol = 3% daily vol, hourly std = 0.03/sqrt(24) = ~0.00612

	// Instead of engineering returns, use fallback volatilities for precise control
	calc.SetFallbackVolatility("BTCUSDT", 0.02) // 2% daily vol
	calc.SetFallbackVolatility("ETHUSDT", 0.03) // 3% daily vol

	positions := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 2},
		{Symbol: "ETHUSDT", Side: "short", Quantity: 10.0, MarkPrice: 3000, Leverage: 3},
	}
	equity := 100000.0

	fastRisk := calc.CalcFastRisk(positions, equity)

	// Fast risk = Sigma(notional * vol * leverage) / equity / target_vol
	// BTC: 50000 * 0.02 * 2 / 100000 = 0.02
	// ETH: 30000 * 0.03 * 3 / 100000 = 0.027
	// Total: 0.047 / 0.02 = 2.35
	assert.InDelta(t, 2.35, fastRisk, 0.01)
}

func TestPortfolioRiskCalculator_FastRisk_WithCorrMatrix(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Update correlation matrix - this also sets volatilities
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	// Create returns that produce known volatilities
	// vol = std * sqrt(24), so std = vol / sqrt(24)
	// For 2% vol: std = 0.00408, for 3% vol: std = 0.00612
	// With ddof=1 and 5 samples, we need to engineer the returns carefully

	// Use simple returns to get approximate volatilities
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},
		{0.015, -0.03, 0.045, -0.015, 0.03},
	}
	err := calc.UpdateCorrelation(symbols, returns)
	require.NoError(t, err)

	positions := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
	}
	equity := 100000.0

	fastRisk := calc.CalcFastRisk(positions, equity)
	assert.Greater(t, fastRisk, 0.0)
}

func TestPortfolioRiskCalculator_AccurateRisk(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Set up correlation matrix with perfect positive correlation
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},
		{0.01, -0.02, 0.03, -0.01, 0.02}, // Same returns = correlation 1
	}
	err := calc.UpdateCorrelation(symbols, returns)
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
	err := calc.UpdateCorrelation(symbols, returns)
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
	err := calc.UpdateCorrelation(symbols, returns)
	require.NoError(t, err)

	// GetSymbolVolatility should return same value as corrMatrix
	vol := calc.GetSymbolVolatility("BTCUSDT")
	expectedVol := calc.corrMatrix.GetVolatility("BTCUSDT")
	assert.Equal(t, expectedVol, vol)
}

func TestPortfolioRiskCalculator_VolatilityPriority(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Set fallback volatility
	calc.SetFallbackVolatility("NEWCOIN", 0.05)

	// Unknown symbol should use fallback
	vol := calc.GetSymbolVolatility("NEWCOIN")
	assert.Equal(t, 0.05, vol)

	// Completely unknown symbol should use default
	vol2 := calc.GetSymbolVolatility("UNKNOWN")
	assert.Equal(t, 0.03, vol2)

	// Now add to correlation matrix - should take priority
	symbols := []string{"NEWCOIN"}
	returns := [][]float64{{0.01, -0.01, 0.01, -0.01, 0.01}}
	err := calc.UpdateCorrelation(symbols, returns)
	require.NoError(t, err)

	vol3 := calc.GetSymbolVolatility("NEWCOIN")
	// Should now use correlation matrix volatility, not fallback
	assert.NotEqual(t, 0.05, vol3)
}

func TestPortfolioRiskCalculator_ZeroEquity(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	positions := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
	}

	// Zero equity should return 0 risk
	assert.Equal(t, 0.0, calc.CalcFastRisk(positions, 0))
	assert.Equal(t, 0.0, calc.CalcAccurateRisk(positions, 0))

	// Negative equity should return 0 risk
	assert.Equal(t, 0.0, calc.CalcFastRisk(positions, -1000))
	assert.Equal(t, 0.0, calc.CalcAccurateRisk(positions, -1000))
}

func TestPortfolioRiskCalculator_EmptyPositions(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Empty positions should return 0 risk
	assert.Equal(t, 0.0, calc.CalcFastRisk(nil, 100000))
	assert.Equal(t, 0.0, calc.CalcAccurateRisk(nil, 100000))
	assert.Equal(t, 0.0, calc.CalcFastRisk([]Position{}, 100000))
	assert.Equal(t, 0.0, calc.CalcAccurateRisk([]Position{}, 100000))
}

func TestPortfolioRiskCalculator_CalcBothRisks(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	symbols := []string{"BTCUSDT"}
	returns := [][]float64{{0.01, -0.02, 0.03, -0.01, 0.02}}
	err := calc.UpdateCorrelation(symbols, returns)
	require.NoError(t, err)

	positions := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 2},
	}
	equity := 100000.0

	metrics := calc.CalcBothRisks(positions, equity)

	assert.Greater(t, metrics.FastRisk, 0.0)
	assert.Greater(t, metrics.AccurateRisk, 0.0)
	assert.False(t, metrics.LastAccurateUpdate.IsZero())
}

func TestPortfolioRiskCalculator_NeedsCorrelationUpdate(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Initially needs update (never updated)
	assert.True(t, calc.NeedsCorrelationUpdate())

	// After update, should not need update
	symbols := []string{"BTCUSDT"}
	returns := [][]float64{{0.01, -0.02, 0.03, -0.01, 0.02}}
	err := calc.UpdateCorrelation(symbols, returns)
	require.NoError(t, err)

	assert.False(t, calc.NeedsCorrelationUpdate())
}

func TestPortfolioRiskCalculator_DirectionHandling(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Perfect positive correlation
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},
		{0.01, -0.02, 0.03, -0.01, 0.02},
	}
	err := calc.UpdateCorrelation(symbols, returns)
	require.NoError(t, err)

	equity := 100000.0

	// Same direction (both long) - should have higher risk
	longLong := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
		{Symbol: "ETHUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
	}
	riskLongLong := calc.CalcAccurateRisk(longLong, equity)

	// Opposite direction (long/short) - should hedge with corr=1
	longShort := []Position{
		{Symbol: "BTCUSDT", Side: "long", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
		{Symbol: "ETHUSDT", Side: "short", Quantity: 1.0, MarkPrice: 50000, Leverage: 1},
	}
	riskLongShort := calc.CalcAccurateRisk(longShort, equity)

	// With perfect positive correlation:
	// - Long/Long: risks add (higher portfolio risk)
	// - Long/Short: risks subtract (lower portfolio risk, hedging)
	assert.Greater(t, riskLongLong, riskLongShort)
}

func TestPortfolioRiskCalculator_LastAccurateCalcUpdate(t *testing.T) {
	calc := NewPortfolioRiskCalculator(TargetPortfolioVol)

	// Check initial state
	calc.mu.RLock()
	initialTime := calc.lastAccurateCalc
	calc.mu.RUnlock()
	assert.True(t, initialTime.IsZero())

	// Update correlation
	before := time.Now()
	symbols := []string{"BTCUSDT"}
	returns := [][]float64{{0.01, -0.02, 0.03, -0.01, 0.02}}
	err := calc.UpdateCorrelation(symbols, returns)
	require.NoError(t, err)
	after := time.Now()

	// Check lastAccurateCalc was updated
	calc.mu.RLock()
	lastCalc := calc.lastAccurateCalc
	calc.mu.RUnlock()

	assert.True(t, lastCalc.After(before) || lastCalc.Equal(before))
	assert.True(t, lastCalc.Before(after) || lastCalc.Equal(after))
}
