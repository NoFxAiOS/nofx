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
	CurrentCycle     *risk.ABTestCycle
	HistoricalCycles []risk.ABTestCycle
	ChampionID       string
	ChallengerIDs    []string
	BudgetAlloc      risk.BudgetAllocation
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
