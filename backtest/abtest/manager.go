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
	config      CycleConfig
	state       *CycleState
	riskCalc    *risk.PortfolioRiskCalculator
	budgetAlloc *risk.BudgetAllocator
	regimeCalc  *risk.RegimeCalculator

	// Performance tracking
	strategyPerf map[string][]float64 // strategy_id -> cycle returns
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
	CycleID            string
	Winner             string
	ShouldPromote      bool
	PromotionCandidate string
	GateResults        []risk.GateResult
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
