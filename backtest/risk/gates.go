// backtest/risk/gates.go
package risk

import (
	"math"
	"math/rand"
	"sort"
)

// alignedDay represents a single day with aligned PnL data from both strategies.
type alignedDay struct {
	date     string
	champPnL float64
	challPnL float64
}

// CheckRiskParityGate validates that challenger maintains similar risk profile to champion.
// RP-1: P95 risk usage ratio deviation <= 20%
// RP-2: P95 leverage difference <= 1.0
func CheckRiskParityGate(champion, challenger *StrategyResults, champBudget, challBudget float64) GateResult {
	result := GateResult{
		Gate:    "RiskParity",
		Passed:  false,
		Details: make(map[string]interface{}),
	}

	// Validate input data
	if len(champion.RiskUsedTS) == 0 || len(challenger.RiskUsedTS) == 0 {
		result.Reason = "RP-1: insufficient data for risk parity check"
		return result
	}
	if len(champion.LeverageTS) == 0 || len(challenger.LeverageTS) == 0 {
		result.Reason = "RP-2: insufficient data for leverage check"
		return result
	}

	// RP-1: P95 risk usage ratio deviation
	champP95Risk := percentile(champion.RiskUsedTS, 0.95)
	challP95Risk := percentile(challenger.RiskUsedTS, 0.95)

	// Avoid division by zero
	if champBudget <= 0 {
		champBudget = 1.0
	}
	if challBudget <= 0 {
		challBudget = 1.0
	}

	champRatio := champP95Risk / champBudget
	challRatio := challP95Risk / challBudget

	var riskDeviation float64
	if champRatio > 0 {
		riskDeviation = math.Abs(challRatio-champRatio) / champRatio
	}

	result.Details["champ_p95_risk"] = champP95Risk
	result.Details["chall_p95_risk"] = challP95Risk
	result.Details["champ_ratio"] = champRatio
	result.Details["chall_ratio"] = challRatio
	result.Details["risk_deviation"] = riskDeviation

	if riskDeviation > RiskDeviationThreshold {
		result.Reason = "RP-1: risk ratio deviation exceeds threshold"
		result.Details["threshold"] = RiskDeviationThreshold
		return result
	}

	// RP-2: P95 leverage difference
	champP95Lev := percentile(champion.LeverageTS, 0.95)
	challP95Lev := percentile(challenger.LeverageTS, 0.95)
	leverageDiff := math.Abs(challP95Lev - champP95Lev)

	result.Details["champ_p95_leverage"] = champP95Lev
	result.Details["chall_p95_leverage"] = challP95Lev
	result.Details["leverage_diff"] = leverageDiff

	if leverageDiff > LeverageDeviationMax {
		result.Reason = "RP-2: leverage deviation exceeds threshold"
		result.Details["threshold"] = LeverageDeviationMax
		return result
	}

	result.Passed = true
	result.Reason = "all risk parity checks passed"
	return result
}

// CheckDominanceGate validates that challenger demonstrates statistical dominance.
// Scoring metrics (5 items, need >= 3 wins):
//   - S-1: NetPnL (challenger > champion * 1.05)
//   - S-2: Sharpe (challenger > champion + 0.1)
//   - S-3: ProfitFactor (challenger > champion)
//   - S-4: Calmar (challenger > champion)
//   - S-5: WinRate (challenger > champion, only if trades >= 20)
//
// Constraint metrics (hard thresholds):
//   - C-1: ES95 <= champion * 1.1 (MUST pass)
//   - C-2: MaxDD <= champion * 1.05, or <= 1.10 with Calmar compensation
func CheckDominanceGate(champion, challenger *StrategyResults) GateResult {
	result := GateResult{
		Gate:    "Dominance",
		Passed:  false,
		Details: make(map[string]interface{}),
	}

	// C-1: ES95 constraint (hard threshold - MUST pass)
	es95Threshold := champion.ES95 * PortfolioRiskP95Max // 1.1
	if challenger.ES95 > es95Threshold {
		result.Reason = "ES95 exceeds maximum threshold (C-1)"
		result.Details["champion_es95"] = champion.ES95
		result.Details["challenger_es95"] = challenger.ES95
		result.Details["threshold"] = es95Threshold
		return result
	}

	// C-2: MaxDD constraint with Calmar compensation
	maxDDThreshold := champion.MaxDrawdown * 1.05
	maxDDWithCalmar := champion.MaxDrawdown * 1.10

	calmarCompensation := challenger.Calmar > champion.Calmar
	if challenger.MaxDrawdown > maxDDThreshold {
		if !calmarCompensation || challenger.MaxDrawdown > maxDDWithCalmar {
			result.Reason = "MaxDD exceeds threshold (C-2)"
			result.Details["champion_maxdd"] = champion.MaxDrawdown
			result.Details["challenger_maxdd"] = challenger.MaxDrawdown
			result.Details["threshold"] = maxDDThreshold
			result.Details["calmar_compensation"] = calmarCompensation
			return result
		}
	}

	// Scoring metrics
	wins := 0
	scoringDetails := make(map[string]bool)

	// S-1: NetPnL (challenger > champion * 1.05)
	if challenger.NetPnL > champion.NetPnL*1.05 {
		wins++
		scoringDetails["S1_NetPnL"] = true
	} else {
		scoringDetails["S1_NetPnL"] = false
	}

	// S-2: Sharpe (challenger > champion + 0.1)
	if challenger.Sharpe > champion.Sharpe+0.1 {
		wins++
		scoringDetails["S2_Sharpe"] = true
	} else {
		scoringDetails["S2_Sharpe"] = false
	}

	// S-3: ProfitFactor (challenger > champion)
	if challenger.ProfitFactor > champion.ProfitFactor {
		wins++
		scoringDetails["S3_ProfitFactor"] = true
	} else {
		scoringDetails["S3_ProfitFactor"] = false
	}

	// S-4: Calmar (challenger > champion)
	if challenger.Calmar > champion.Calmar {
		wins++
		scoringDetails["S4_Calmar"] = true
	} else {
		scoringDetails["S4_Calmar"] = false
	}

	// S-5: WinRate (only if trades >= 20)
	if challenger.TradesCount >= 20 && champion.TradesCount >= 20 {
		if challenger.WinRate > champion.WinRate {
			wins++
			scoringDetails["S5_WinRate"] = true
		} else {
			scoringDetails["S5_WinRate"] = false
		}
	} else {
		scoringDetails["S5_WinRate_skipped"] = true
	}

	result.Details["wins"] = wins
	result.Details["required"] = DominanceWinsRequired
	result.Details["scoring"] = scoringDetails

	if wins < DominanceWinsRequired {
		result.Reason = "insufficient wins in scoring metrics"
		return result
	}

	result.Passed = true
	result.Reason = "dominance gate passed"
	return result
}

// CheckEvidenceGate validates statistical evidence for challenger's performance.
// E-1: Minimum sample (trades >= MinTrades OR activeDays >= MinActiveDays)
// E-2: Segment robustness (4 segments if >= 40 days, need 3/4; 2 segments if >= 20 days, need 2/2)
// E-3: Bootstrap test (1000 resamples of daily PnL diff, 2.5% CI > 0)
// E-4: Regime diversity (last 3 wins must span >= 2 regimes)
func CheckEvidenceGate(champion, challenger *StrategyResults, historicalCycles []ABTestCycle) GateResult {
	result := GateResult{
		Gate:    "Evidence",
		Passed:  false,
		Details: make(map[string]interface{}),
	}

	// E-1: Minimum sample size
	if challenger.TradesCount < MinTrades && challenger.ActiveDays < MinActiveDays {
		result.Reason = "INSUFFICIENT_SAMPLE"
		result.Details["trades"] = challenger.TradesCount
		result.Details["active_days"] = challenger.ActiveDays
		result.Details["min_trades"] = MinTrades
		result.Details["min_active_days"] = MinActiveDays
		return result
	}

	// Need champion data for remaining checks
	if champion == nil || champion.DailyPnL == nil || challenger.DailyPnL == nil {
		// Can't perform segment/bootstrap checks without daily data
		result.Passed = true
		result.Reason = "sample size check passed (daily data unavailable for additional checks)"
		return result
	}

	// E-2: Segment robustness
	passed, positiveSegs, totalSegs := checkSegmentRobustness(champion.DailyPnL, challenger.DailyPnL)
	result.Details["segment_positive"] = positiveSegs
	result.Details["segment_total"] = totalSegs

	if totalSegs > 0 && !passed {
		result.Reason = "SEGMENT_ROBUSTNESS_FAILED"
		return result
	}

	// E-3: Bootstrap test
	bootstrapPassed, ci25 := checkBootstrap(champion.DailyPnL, challenger.DailyPnL)
	result.Details["bootstrap_ci_2.5"] = ci25

	if !bootstrapPassed {
		result.Reason = "BOOTSTRAP_FAILED"
		return result
	}

	// E-4: Regime diversity (only if we have historical cycles)
	if len(historicalCycles) >= 3 && challenger.ID != "" {
		if !checkRegimeDiversity(challenger.ID, historicalCycles) {
			result.Reason = "REGIME_DIVERSITY_FAILED"
			return result
		}
	}

	result.Passed = true
	result.Reason = "all evidence checks passed"
	return result
}

// percentile calculates the p-th percentile of a sorted copy of data.
// p should be in [0, 1].
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0.0
	}

	// Make a sorted copy
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	n := float64(len(sorted))
	if n == 1 {
		return sorted[0]
	}

	// Handle edge cases
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	// Linear interpolation
	index := p * (n - 1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	// Interpolate
	fraction := index - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}

// checkSegmentRobustness checks if challenger wins required segments.
// Returns (passed, positiveSegments, totalSegments)
func checkSegmentRobustness(champDaily, challDaily map[string]float64) (bool, int, int) {
	aligned := alignDailyPnL(champDaily, challDaily)
	n := len(aligned)

	// Check minimum days
	if n < MinDaysForSegment {
		return true, 0, 0 // Skip check if insufficient data
	}

	// Sort by date
	sort.Slice(aligned, func(i, j int) bool {
		return aligned[i].date < aligned[j].date
	})

	var numSegments int
	var requiredPositive int

	if n >= MinDaysFor4Segments {
		numSegments = 4
		requiredPositive = 3 // 3/4
	} else {
		numSegments = 2
		requiredPositive = 2 // 2/2
	}

	segmentSize := n / numSegments
	positiveSegments := 0

	for i := 0; i < numSegments; i++ {
		start := i * segmentSize
		end := start + segmentSize
		if i == numSegments-1 {
			end = n // Include remaining days in last segment
		}

		segmentDiff := 0.0
		for j := start; j < end; j++ {
			segmentDiff += aligned[j].challPnL - aligned[j].champPnL
		}

		if segmentDiff > 0 {
			positiveSegments++
		}
	}

	return positiveSegments >= requiredPositive, positiveSegments, numSegments
}

// checkBootstrap performs bootstrap resampling test.
// Returns (passed, 2.5% CI value)
func checkBootstrap(champDaily, challDaily map[string]float64) (bool, float64) {
	aligned := alignDailyPnL(champDaily, challDaily)
	if len(aligned) < 2 {
		return true, 0 // Skip if insufficient data
	}

	// Calculate daily PnL differences
	diffs := make([]float64, len(aligned))
	for i, day := range aligned {
		diffs[i] = day.challPnL - day.champPnL
	}

	// Bootstrap resampling
	const numResamples = 1000
	resampledMeans := make([]float64, numResamples)

	rng := rand.New(rand.NewSource(42)) // Deterministic seed for reproducibility
	n := len(diffs)

	for i := 0; i < numResamples; i++ {
		sum := 0.0
		for j := 0; j < n; j++ {
			idx := rng.Intn(n)
			sum += diffs[idx]
		}
		resampledMeans[i] = sum / float64(n)
	}

	// Get 2.5% percentile (lower bound of 95% CI)
	ci25 := percentile(resampledMeans, 0.025)

	return ci25 > 0, ci25
}

// checkRegimeDiversity checks if challenger's last 3 wins span >= 2 regimes.
func checkRegimeDiversity(challengerID string, cycles []ABTestCycle) bool {
	// Find last 3 wins for this challenger
	var wins []ABTestCycle
	for i := len(cycles) - 1; i >= 0 && len(wins) < 3; i-- {
		if cycles[i].Winner == challengerID {
			wins = append(wins, cycles[i])
		}
	}

	if len(wins) < 3 {
		return true // Not enough history, skip check
	}

	// Count unique regimes
	regimes := make(map[string]bool)
	for _, cycle := range wins {
		regimes[cycle.RegimeSummary.PrimaryRegime] = true
	}

	return len(regimes) >= 2
}

// alignDailyPnL returns only dates present in both maps.
func alignDailyPnL(champ, chall map[string]float64) []alignedDay {
	var aligned []alignedDay

	for date, champPnL := range champ {
		if challPnL, exists := chall[date]; exists {
			aligned = append(aligned, alignedDay{
				date:     date,
				champPnL: champPnL,
				challPnL: challPnL,
			})
		}
	}

	return aligned
}
