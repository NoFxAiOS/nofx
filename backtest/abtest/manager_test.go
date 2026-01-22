// backtest/abtest/manager_test.go
package abtest

import (
	"nofx/backtest/risk"
	"testing"
	"time"

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

func TestManager_PromotionNotFound(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")

	err := m.Promote("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_EndCycleNoActiveCycle(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")

	_, err := m.EndCycle(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active cycle")
}

func TestCycleState_AddRemoveChallenger(t *testing.T) {
	state := NewCycleState("champ")

	state.AddChallenger("c1")
	state.AddChallenger("c2")
	assert.Len(t, state.ChallengerIDs, 2)

	// Adding duplicate should not change list
	state.AddChallenger("c1")
	assert.Len(t, state.ChallengerIDs, 2)

	state.RemoveChallenger("c1")
	assert.Len(t, state.ChallengerIDs, 1)
	assert.NotContains(t, state.ChallengerIDs, "c1")
}

func TestCycleState_PromoteChallenger(t *testing.T) {
	state := NewCycleState("champ")
	state.AddChallenger("c1")
	state.AddChallenger("c2")

	state.PromoteChallenger("c1")

	assert.Equal(t, "c1", state.ChampionID)
	assert.Contains(t, state.ChallengerIDs, "champ")
	assert.Contains(t, state.ChallengerIDs, "c2")
	assert.NotContains(t, state.ChallengerIDs, "c1")
}

func TestDefaultCycleConfig(t *testing.T) {
	cfg := DefaultCycleConfig()
	assert.Equal(t, 24*time.Hour, cfg.CycleDuration)
	assert.Equal(t, 3, cfg.MinCyclesForPromo)
}

func TestNewCycleState(t *testing.T) {
	state := NewCycleState("my_champion")

	assert.Equal(t, "my_champion", state.ChampionID)
	assert.NotNil(t, state.ChallengerIDs)
	assert.Empty(t, state.ChallengerIDs)
	assert.NotNil(t, state.HistoricalCycles)
	assert.Empty(t, state.HistoricalCycles)
	assert.Equal(t, 1.0, state.BudgetAlloc["my_champion"])
	assert.Nil(t, state.CurrentCycle)
}

func TestManager_GetRiskCalculator(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	rc := m.GetRiskCalculator()
	assert.NotNil(t, rc)
}

func TestManager_GetRegimeCalculator(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	rc := m.GetRegimeCalculator()
	assert.NotNil(t, rc)
}

func TestManager_GetBudgetAllocation(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	alloc := m.GetBudgetAllocation()
	assert.Equal(t, 1.0, alloc["champion"])
}

func TestManager_MultipleChallengers(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	m.AddChallenger("c1")
	m.AddChallenger("c2")
	m.AddChallenger("c3")

	state := m.GetState()
	assert.Len(t, state.ChallengerIDs, 3)

	// All strategies should have budget allocation
	alloc := m.GetBudgetAllocation()
	assert.Greater(t, alloc["champion"], 0.0)
	assert.Greater(t, alloc["c1"], 0.0)
	assert.Greater(t, alloc["c2"], 0.0)
	assert.Greater(t, alloc["c3"], 0.0)

	// Total should sum to 1.0
	total := alloc["champion"] + alloc["c1"] + alloc["c2"] + alloc["c3"]
	assert.InDelta(t, 1.0, total, 0.001)
}

func TestManager_CycleWithMultipleChallengers(t *testing.T) {
	m := NewManager(DefaultCycleConfig(), "champion")
	m.AddChallenger("c1")
	m.AddChallenger("c2")

	regime := risk.RegimeSummary{PrimaryRegime: "high_trending"}
	m.StartCycle(regime)

	cycle := m.GetState().CurrentCycle
	require.NotNil(t, cycle)
	assert.Equal(t, "champion", cycle.ChampionID)
	assert.Len(t, cycle.ChallengerIDs, 2)
	assert.Contains(t, cycle.ChallengerIDs, "c1")
	assert.Contains(t, cycle.ChallengerIDs, "c2")
}
