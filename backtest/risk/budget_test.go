// backtest/risk/budget_test.go
package risk

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
