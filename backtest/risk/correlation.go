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
	mu           sync.RWMutex
	matrix       [][]float64          // n x n correlation matrix
	symbols      []string             // symbol order matching matrix indices
	symbolIndex  map[string]int       // symbol -> index for O(1) lookup
	volatilities map[string]float64   // daily volatilities
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
