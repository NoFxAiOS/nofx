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
		{0.01, -0.02, 0.03, -0.01, 0.02},      // BTC
		{0.015, -0.025, 0.035, -0.015, 0.025}, // ETH (highly correlated)
		{-0.01, 0.02, -0.03, 0.01, -0.02},     // Inverse asset
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

func TestCorrelationMatrix_EmptySymbols(t *testing.T) {
	cm := NewCorrelationMatrix()

	err := cm.Update([]string{}, [][]float64{})
	assert.Error(t, err)
}

func TestCorrelationMatrix_UnequalReturnLengths(t *testing.T) {
	cm := NewCorrelationMatrix()

	returns := [][]float64{
		{0.01, 0.02, 0.03},
		{0.01, 0.02}, // Different length
	}
	err := cm.Update([]string{"A", "B"}, returns)
	assert.Error(t, err)
}

func TestCorrelationMatrix_GetUnknownSymbol(t *testing.T) {
	cm := NewCorrelationMatrix()
	returns := [][]float64{
		{0.01, -0.02, 0.03},
	}
	err := cm.Update([]string{"BTCUSDT"}, returns)
	require.NoError(t, err)

	// Unknown symbol should return 0.0
	assert.Equal(t, 0.0, cm.Get("BTCUSDT", "UNKNOWN"))
	assert.Equal(t, 0.0, cm.Get("UNKNOWN", "BTCUSDT"))
	assert.Equal(t, 0.0, cm.Get("UNKNOWN1", "UNKNOWN2"))
}

func TestCorrelationMatrix_GetVolatility(t *testing.T) {
	cm := NewCorrelationMatrix()
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},
	}
	err := cm.Update([]string{"BTCUSDT"}, returns)
	require.NoError(t, err)

	// Volatility should be positive
	vol := cm.GetVolatility("BTCUSDT")
	assert.Greater(t, vol, 0.0)

	// Unknown symbol should return default 3%
	assert.Equal(t, 0.03, cm.GetVolatility("UNKNOWN"))
}

func TestCorrelationMatrix_Symbols(t *testing.T) {
	cm := NewCorrelationMatrix()
	returns := [][]float64{
		{0.01, -0.02, 0.03},
		{0.015, -0.025, 0.035},
	}
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	err := cm.Update(symbols, returns)
	require.NoError(t, err)

	// Symbols should return a copy
	result := cm.Symbols()
	assert.Equal(t, symbols, result)

	// Modifying the result should not affect internal state
	result[0] = "MODIFIED"
	assert.Equal(t, "BTCUSDT", cm.Symbols()[0])
}

func TestCorrelationMatrix_GetSubMatrix_UnknownSymbol(t *testing.T) {
	cm := NewCorrelationMatrix()
	returns := [][]float64{
		{0.01, -0.02, 0.03},
	}
	err := cm.Update([]string{"BTCUSDT"}, returns)
	require.NoError(t, err)

	// SubMatrix with unknown symbol should have identity for that symbol
	subMatrix := cm.GetSubMatrix([]string{"UNKNOWN", "BTCUSDT"})
	assert.Equal(t, 1.0, subMatrix[0][0]) // Unknown diagonal = 1
	assert.Equal(t, 1.0, subMatrix[1][1]) // BTCUSDT diagonal = 1
	assert.Equal(t, 0.0, subMatrix[0][1]) // Unknown-BTCUSDT = 0
}

func TestCorrelationMatrix_ZeroVariance(t *testing.T) {
	cm := NewCorrelationMatrix()
	// Constant returns = zero variance
	returns := [][]float64{
		{0.01, 0.01, 0.01},
		{0.02, 0.03, 0.04},
	}
	err := cm.Update([]string{"CONSTANT", "NORMAL"}, returns)
	require.NoError(t, err)

	// Correlation with zero-variance should be 0
	assert.Equal(t, 0.0, cm.Get("CONSTANT", "NORMAL"))
	assert.Equal(t, 0.0, cm.Get("NORMAL", "CONSTANT"))

	// Self-correlation should still be 1
	assert.Equal(t, 1.0, cm.Get("CONSTANT", "CONSTANT"))
}

func TestCorrelationMatrix_PerfectNegativeCorrelation(t *testing.T) {
	cm := NewCorrelationMatrix()
	// Perfectly negatively correlated
	returns := [][]float64{
		{0.01, -0.02, 0.03, -0.01, 0.02},
		{-0.01, 0.02, -0.03, 0.01, -0.02},
	}
	err := cm.Update([]string{"A", "B"}, returns)
	require.NoError(t, err)

	// Should be exactly -1
	assert.InDelta(t, -1.0, cm.Get("A", "B"), 0.0001)
}

func TestCorrelationMatrix_DDof1_Variance(t *testing.T) {
	// Test that ddof=1 is used (Bessel's correction)
	// For n=2, variance with ddof=1 should be different from ddof=0
	cm := NewCorrelationMatrix()
	returns := [][]float64{
		{0.0, 1.0}, // mean=0.5, sum of squared diff=0.5
	}
	err := cm.Update([]string{"TEST"}, returns)
	require.NoError(t, err)

	// With ddof=1: variance = 0.5 / (2-1) = 0.5, std = sqrt(0.5) = 0.707...
	// Daily vol = std * sqrt(24) = 0.707 * 4.899 = 3.464...
	vol := cm.GetVolatility("TEST")

	// Expected: sqrt(0.5) * sqrt(24) = sqrt(12) = 3.464...
	expectedVol := 3.4641016151377544
	assert.InDelta(t, expectedVol, vol, 0.0001)
}
