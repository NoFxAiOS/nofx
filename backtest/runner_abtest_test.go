// backtest/runner_abtest_test.go
package backtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBacktestConfig_ABTestFields(t *testing.T) {
	cfg := BacktestConfig{
		ABTestEnabled:       true,
		ABTestChampionID:    "strategy_a",
		ABTestChallengerIDs: []string{"strategy_b", "strategy_c"},
	}

	assert.True(t, cfg.ABTestEnabled)
	assert.Equal(t, "strategy_a", cfg.ABTestChampionID)
	assert.Len(t, cfg.ABTestChallengerIDs, 2)
}

func TestBacktestConfig_ABTestDisabled(t *testing.T) {
	cfg := BacktestConfig{
		ABTestEnabled: false,
	}

	assert.False(t, cfg.ABTestEnabled)
	assert.Empty(t, cfg.ABTestChampionID)
}
