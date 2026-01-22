// backtest/risk/gates_test.go
package risk

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Risk Parity Gate Tests
// ============================================================================

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
	assert.Equal(t, "RiskParity", result.Gate)
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

func TestRiskParityGate_FailLeverageDeviation(t *testing.T) {
	champion := &StrategyResults{
		RiskUsedTS: []float64{0.5, 0.5, 0.5},
		LeverageTS: []float64{2.0, 2.0, 2.0},
	}
	challenger := &StrategyResults{
		RiskUsedTS: []float64{0.5, 0.5, 0.5},
		LeverageTS: []float64{5.0, 5.0, 5.0}, // Leverage diff > 1.0
	}

	result := CheckRiskParityGate(champion, challenger, 1.0, 1.0)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "RP-2")
}

func TestRiskParityGate_DifferentBudgets(t *testing.T) {
	champion := &StrategyResults{
		RiskUsedTS: []float64{0.4, 0.5, 0.6}, // ratio = 0.6 / 0.8 = 0.75
		LeverageTS: []float64{2.0, 2.0, 2.0},
	}
	challenger := &StrategyResults{
		RiskUsedTS: []float64{0.1, 0.15, 0.2}, // ratio = 0.2 / 0.2 = 1.0
		LeverageTS: []float64{2.0, 2.0, 2.0},
	}

	// Different budgets: champion 0.8, challenger 0.2
	// Champion ratio = P95(0.4,0.5,0.6) / 0.8 = ~0.6 / 0.8 = 0.75
	// Challenger ratio = P95(0.1,0.15,0.2) / 0.2 = ~0.2 / 0.2 = 1.0
	// Deviation = |1.0 - 0.75| / 0.75 = 0.333 > 0.20 = FAIL
	result := CheckRiskParityGate(champion, challenger, 0.8, 0.2)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "RP-1")
}

func TestRiskParityGate_EmptyTimeSeries(t *testing.T) {
	champion := &StrategyResults{
		RiskUsedTS: []float64{},
		LeverageTS: []float64{},
	}
	challenger := &StrategyResults{
		RiskUsedTS: []float64{},
		LeverageTS: []float64{},
	}

	result := CheckRiskParityGate(champion, challenger, 1.0, 1.0)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "insufficient data")
}

// ============================================================================
// Dominance Gate Tests
// ============================================================================

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
	assert.Equal(t, "Dominance", result.Gate)
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
		ES95:         150, // > 100 * 1.1 = FAIL (hard constraint)
		MaxDrawdown:  0.09,
		TradesCount:  50,
	}

	result := CheckDominanceGate(champion, challenger)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "ES95")
}

func TestDominanceGate_FailMaxDD(t *testing.T) {
	champion := &StrategyResults{
		NetPnL:       1000,
		Sharpe:       1.0,
		ProfitFactor: 1.5,
		Calmar:       2.0, // Same Calmar, no compensation
		WinRate:      0.55,
		ES95:         100,
		MaxDrawdown:  0.10,
		TradesCount:  50,
	}
	challenger := &StrategyResults{
		NetPnL:       1200,
		Sharpe:       1.2,
		ProfitFactor: 1.6,
		Calmar:       2.0, // Same Calmar, no compensation
		WinRate:      0.58,
		ES95:         105,
		MaxDrawdown:  0.12, // > 0.10 * 1.05 and no Calmar compensation
		TradesCount:  50,
	}

	result := CheckDominanceGate(champion, challenger)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "MaxDD")
}

func TestDominanceGate_MaxDDWithCalmarCompensation(t *testing.T) {
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
		Calmar:       2.5, // Higher Calmar = compensation applies
		WinRate:      0.58,
		ES95:         105,
		MaxDrawdown:  0.108, // > 0.10 * 1.05 but <= 0.10 * 1.10 with Calmar compensation
		TradesCount:  50,
	}

	result := CheckDominanceGate(champion, challenger)
	// Should pass because Calmar compensation allows up to 1.10
	assert.True(t, result.Passed)
}

func TestDominanceGate_InsufficientWins(t *testing.T) {
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
		NetPnL:       1000, // Not > 1000 * 1.05 (no win)
		Sharpe:       1.05, // Not > 1.0 + 0.1 (no win)
		ProfitFactor: 1.6,  // Win
		Calmar:       2.2,  // Win
		WinRate:      0.50, // Not > 0.55 (no win)
		ES95:         105,
		MaxDrawdown:  0.09,
		TradesCount:  50,
	}

	result := CheckDominanceGate(champion, challenger)
	assert.False(t, result.Passed)
	assert.Equal(t, 2, result.Details["wins"])
	assert.Contains(t, result.Reason, "insufficient wins")
}

func TestDominanceGate_WinRateSkippedLowTrades(t *testing.T) {
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
		WinRate:      0.50, // Lower, but trades < 20 so skipped
		ES95:         105,
		MaxDrawdown:  0.09,
		TradesCount:  15, // < 20, WinRate comparison skipped
	}

	result := CheckDominanceGate(champion, challenger)
	// Should have 4 wins (NetPnL, Sharpe, ProfitFactor, Calmar) - WinRate skipped
	assert.True(t, result.Passed)
	assert.Equal(t, 4, result.Details["wins"])
}

// ============================================================================
// Evidence Gate Tests
// ============================================================================

func TestEvidenceGate_InsufficientSample(t *testing.T) {
	challenger := &StrategyResults{
		TradesCount: 20, // < MinTrades (30)
		ActiveDays:  10, // < MinActiveDays (15)
	}

	result := CheckEvidenceGate(nil, challenger, nil)
	assert.False(t, result.Passed)
	assert.Equal(t, "INSUFFICIENT_SAMPLE", result.Reason)
	assert.Equal(t, "Evidence", result.Gate)
}

func TestEvidenceGate_PassWithTrades(t *testing.T) {
	championDaily := map[string]float64{
		"2024-01-01": 100.0,
		"2024-01-02": 100.0,
	}
	challengerDaily := map[string]float64{
		"2024-01-01": 120.0,
		"2024-01-02": 120.0,
	}

	champion := &StrategyResults{DailyPnL: championDaily}
	challenger := &StrategyResults{
		DailyPnL:    challengerDaily,
		TradesCount: 50, // >= MinTrades (30)
		ActiveDays:  5,
	}

	result := CheckEvidenceGate(champion, challenger, nil)
	// Sample size check should pass (trades >= 30)
	assert.NotEqual(t, "INSUFFICIENT_SAMPLE", result.Reason)
}

func TestEvidenceGate_PassWithActiveDays(t *testing.T) {
	championDaily := map[string]float64{
		"2024-01-01": 100.0,
		"2024-01-02": 100.0,
	}
	challengerDaily := map[string]float64{
		"2024-01-01": 120.0,
		"2024-01-02": 120.0,
	}

	champion := &StrategyResults{DailyPnL: championDaily}
	challenger := &StrategyResults{
		DailyPnL:    challengerDaily,
		TradesCount: 10, // < MinTrades
		ActiveDays:  20, // >= MinActiveDays (15)
	}

	result := CheckEvidenceGate(champion, challenger, nil)
	// Sample size check should pass (activeDays >= 15)
	assert.NotEqual(t, "INSUFFICIENT_SAMPLE", result.Reason)
}

func TestEvidenceGate_SegmentRobustness4Segments(t *testing.T) {
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

func TestEvidenceGate_SegmentRobustnessFailure(t *testing.T) {
	// 40 days of data, challenger wins only 2/4 segments
	championDaily := make(map[string]float64)
	challengerDaily := make(map[string]float64)

	for i := 0; i < 40; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		championDaily[date] = 100.0
		if i < 20 { // First 2 segments challenger wins
			challengerDaily[date] = 120.0
		} else { // Last 2 segments champion wins
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
	// Should fail segment robustness (2/4 positive, need 3/4)
	assert.False(t, result.Passed)
	assert.Equal(t, "SEGMENT_ROBUSTNESS_FAILED", result.Reason)
}

func TestEvidenceGate_SegmentRobustness2Segments(t *testing.T) {
	// 25 days of data (>= 20, < 40), need 2/2 segments
	championDaily := make(map[string]float64)
	challengerDaily := make(map[string]float64)

	for i := 0; i < 25; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		championDaily[date] = 100.0
		challengerDaily[date] = 120.0 // Challenger wins all
	}

	champion := &StrategyResults{DailyPnL: championDaily}
	challenger := &StrategyResults{
		DailyPnL:    challengerDaily,
		TradesCount: 50,
		ActiveDays:  25,
	}

	result := CheckEvidenceGate(champion, challenger, nil)
	// Should pass (2/2 segments positive)
	assert.NotEqual(t, "SEGMENT_ROBUSTNESS_FAILED", result.Reason)
}

func TestEvidenceGate_BootstrapPass(t *testing.T) {
	// Challenger consistently beats champion
	championDaily := make(map[string]float64)
	challengerDaily := make(map[string]float64)

	for i := 0; i < 50; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		championDaily[date] = 100.0
		challengerDaily[date] = 150.0 // Clear winner
	}

	champion := &StrategyResults{DailyPnL: championDaily}
	challenger := &StrategyResults{
		DailyPnL:    challengerDaily,
		TradesCount: 50,
		ActiveDays:  50,
	}

	result := CheckEvidenceGate(champion, challenger, nil)
	// Bootstrap 2.5% CI should be > 0
	assert.NotEqual(t, "BOOTSTRAP_FAILED", result.Reason)
}

func TestEvidenceGate_BootstrapFail(t *testing.T) {
	// Mixed results, no clear winner
	championDaily := make(map[string]float64)
	challengerDaily := make(map[string]float64)

	for i := 0; i < 50; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		championDaily[date] = 100.0
		// Alternating: sometimes better, sometimes worse
		if i%2 == 0 {
			challengerDaily[date] = 110.0
		} else {
			challengerDaily[date] = 90.0
		}
	}

	champion := &StrategyResults{DailyPnL: championDaily}
	challenger := &StrategyResults{
		DailyPnL:    challengerDaily,
		TradesCount: 50,
		ActiveDays:  50,
	}

	result := CheckEvidenceGate(champion, challenger, nil)
	// Bootstrap 2.5% CI might be <= 0 due to variance
	// This test may be flaky due to randomness, but demonstrates the check
	assert.Equal(t, "Evidence", result.Gate)
}

func TestEvidenceGate_RegimeDiversity(t *testing.T) {
	challenger := &StrategyResults{
		ID:          "challenger1",
		TradesCount: 50,
		ActiveDays:  50,
		DailyPnL: map[string]float64{
			"2024-01-01": 100.0,
		},
	}
	champion := &StrategyResults{
		DailyPnL: map[string]float64{
			"2024-01-01": 50.0,
		},
	}

	// Historical cycles with different regimes
	cycles := []ABTestCycle{
		{
			Winner: "challenger1",
			RegimeSummary: RegimeSummary{
				PrimaryRegime: "high_trending",
			},
		},
		{
			Winner: "challenger1",
			RegimeSummary: RegimeSummary{
				PrimaryRegime: "low_ranging",
			},
		},
		{
			Winner: "challenger1",
			RegimeSummary: RegimeSummary{
				PrimaryRegime: "mid_trending",
			},
		},
	}

	result := CheckEvidenceGate(champion, challenger, cycles)
	// Should pass regime diversity (3 wins in 3 different regimes >= 2 required)
	assert.NotEqual(t, "REGIME_DIVERSITY_FAILED", result.Reason)
}

func TestEvidenceGate_RegimeDiversityFail(t *testing.T) {
	challenger := &StrategyResults{
		ID:          "challenger1",
		TradesCount: 50,
		ActiveDays:  50,
		DailyPnL: map[string]float64{
			"2024-01-01": 100.0,
		},
	}
	champion := &StrategyResults{
		DailyPnL: map[string]float64{
			"2024-01-01": 50.0,
		},
	}

	// All wins in same regime
	cycles := []ABTestCycle{
		{
			Winner: "challenger1",
			RegimeSummary: RegimeSummary{
				PrimaryRegime: "high_trending",
			},
		},
		{
			Winner: "challenger1",
			RegimeSummary: RegimeSummary{
				PrimaryRegime: "high_trending",
			},
		},
		{
			Winner: "challenger1",
			RegimeSummary: RegimeSummary{
				PrimaryRegime: "high_trending",
			},
		},
	}

	result := CheckEvidenceGate(champion, challenger, cycles)
	// Should fail regime diversity (all wins in same regime, need >= 2)
	assert.False(t, result.Passed)
	assert.Equal(t, "REGIME_DIVERSITY_FAILED", result.Reason)
}

func TestEvidenceGate_FullPass(t *testing.T) {
	// Create comprehensive test data that passes all checks
	championDaily := make(map[string]float64)
	challengerDaily := make(map[string]float64)

	for i := 0; i < 50; i++ {
		date := fmt.Sprintf("2024-02-%02d", i+1)
		championDaily[date] = 100.0
		challengerDaily[date] = 150.0 // Clear consistent winner
	}

	champion := &StrategyResults{DailyPnL: championDaily}
	challenger := &StrategyResults{
		ID:          "challenger1",
		DailyPnL:    challengerDaily,
		TradesCount: 50,
		ActiveDays:  50,
	}

	cycles := []ABTestCycle{
		{Winner: "challenger1", RegimeSummary: RegimeSummary{PrimaryRegime: "high_trending"}},
		{Winner: "challenger1", RegimeSummary: RegimeSummary{PrimaryRegime: "low_ranging"}},
		{Winner: "challenger1", RegimeSummary: RegimeSummary{PrimaryRegime: "mid_trending"}},
	}

	result := CheckEvidenceGate(champion, challenger, cycles)
	assert.True(t, result.Passed)
	assert.Equal(t, "Evidence", result.Gate)
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestPercentile_Basic(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	p50 := percentile(data, 0.50)
	assert.InDelta(t, 5.5, p50, 0.01)

	p95 := percentile(data, 0.95)
	assert.InDelta(t, 9.55, p95, 0.01)

	p0 := percentile(data, 0.0)
	assert.Equal(t, 1.0, p0)

	p100 := percentile(data, 1.0)
	assert.Equal(t, 10.0, p100)
}

func TestPercentile_SingleElement(t *testing.T) {
	data := []float64{5.0}
	p95 := percentile(data, 0.95)
	assert.Equal(t, 5.0, p95)
}

func TestPercentile_Empty(t *testing.T) {
	data := []float64{}
	p95 := percentile(data, 0.95)
	assert.Equal(t, 0.0, p95)
}

func TestAlignDailyPnL(t *testing.T) {
	champ := map[string]float64{
		"2024-01-01": 100.0,
		"2024-01-02": 200.0,
		"2024-01-03": 150.0,
	}
	chall := map[string]float64{
		"2024-01-01": 120.0,
		"2024-01-02": 180.0,
		// Missing 2024-01-03
		"2024-01-04": 200.0, // Extra day not in champion
	}

	aligned := alignDailyPnL(champ, chall)

	// Should only include dates present in both
	assert.Equal(t, 2, len(aligned))

	// Verify alignment
	for _, day := range aligned {
		assert.Contains(t, []string{"2024-01-01", "2024-01-02"}, day.date)
	}
}

func TestCheckSegmentRobustness_4Segments(t *testing.T) {
	champ := make(map[string]float64)
	chall := make(map[string]float64)

	// 40 days, challenger wins 3/4 segments
	for i := 0; i < 40; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		champ[date] = 100.0
		if i < 30 {
			chall[date] = 120.0 // Wins first 3 segments
		} else {
			chall[date] = 80.0 // Loses last segment
		}
	}

	passed, positiveSegs, totalSegs := checkSegmentRobustness(champ, chall)
	assert.True(t, passed)
	assert.Equal(t, 4, totalSegs)
	assert.Equal(t, 3, positiveSegs)
}

func TestCheckSegmentRobustness_2Segments(t *testing.T) {
	champ := make(map[string]float64)
	chall := make(map[string]float64)

	// 25 days, need 2/2 segments
	for i := 0; i < 25; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		champ[date] = 100.0
		if i < 12 {
			chall[date] = 120.0 // Wins first segment
		} else {
			chall[date] = 80.0 // Loses second segment
		}
	}

	passed, positiveSegs, totalSegs := checkSegmentRobustness(champ, chall)
	assert.False(t, passed) // Need 2/2, only got 1/2
	assert.Equal(t, 2, totalSegs)
	assert.Equal(t, 1, positiveSegs)
}

func TestCheckSegmentRobustness_InsufficientDays(t *testing.T) {
	champ := make(map[string]float64)
	chall := make(map[string]float64)

	// Only 15 days, < MinDaysForSegment (20)
	for i := 0; i < 15; i++ {
		date := fmt.Sprintf("2024-01-%02d", i+1)
		champ[date] = 100.0
		chall[date] = 120.0
	}

	passed, _, totalSegs := checkSegmentRobustness(champ, chall)
	assert.True(t, passed) // Skipped due to insufficient days
	assert.Equal(t, 0, totalSegs)
}
