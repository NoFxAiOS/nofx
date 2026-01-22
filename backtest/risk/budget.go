// backtest/risk/budget.go
package risk

import (
	"math"
	"sort"
)

// BudgetAllocator implements UCB-based budget allocation with water-fill method.
// It distributes risk budget between a champion strategy and challenger strategies
// using Upper Confidence Bound scores and a water-fill algorithm that avoids
// order bias through budget snapshots per iteration.
type BudgetAllocator struct {
	explorationFactor float64
	priorNCycles      int
	priorMeanReturn   float64
}

// NewBudgetAllocator creates a new budget allocator with default settings.
// Uses virtual prior (n=1, mean=0) for new challengers to prevent cold-start
// infinite UCB scores.
func NewBudgetAllocator() *BudgetAllocator {
	return &BudgetAllocator{
		explorationFactor: 1.0,
		priorNCycles:      1,   // Virtual prior: n=1
		priorMeanReturn:   0.0, // Virtual prior: mean=0
	}
}

// Allocate returns budget allocation for champion and challengers.
// The allocation follows these rules:
//   - Champion always gets at least ChampionMinBudget (0.50) initially
//   - Champion never falls below ChampionAbsoluteFloor (0.40)
//   - Each challenger gets between ChallengerMinBudget (0.05) and ChallengerMaxBudget (0.25)
//   - At most MaxChallengers (10) challengers are allocated budget
//   - Total allocation sums to 1.0
func (b *BudgetAllocator) Allocate(
	championID string,
	challengerIDs []string,
	historicalPerf map[string][]float64,
) BudgetAllocation {
	if len(challengerIDs) == 0 {
		return BudgetAllocation{championID: 1.0}
	}

	// Enforce MAX_CHALLENGERS limit by selecting top-K by UCB score
	challengers := b.selectTopChallengers(challengerIDs, historicalPerf)

	// Step 1: Champion gets min budget
	allocations := make(BudgetAllocation)
	allocations[championID] = ChampionMinBudget

	// Step 2: Each challenger gets min budget
	for _, cid := range challengers {
		allocations[cid] = ChallengerMinBudget
	}

	// Check if already over budget
	usedBudget := b.sumBudget(allocations)
	remainingBudget := 1.0 - usedBudget

	if remainingBudget < 0 {
		// Reduce champion to floor, recalculate
		shortfall := -remainingBudget
		newChampionBudget := math.Max(ChampionMinBudget-shortfall, ChampionAbsoluteFloor)
		allocations[championID] = newChampionBudget
		remainingBudget = 0
	}

	if remainingBudget <= 1e-9 {
		return b.normalize(allocations, championID)
	}

	// Step 3: Calculate UCB weights for challengers
	weights := b.calcUCBWeights(challengers, historicalPerf)

	// Step 4: Water-fill allocation with budget snapshot per iteration
	headroom := make(map[string]float64)
	for _, cid := range challengers {
		headroom[cid] = ChallengerMaxBudget - allocations[cid]
	}

	maxIterations := len(challengers) + 1
	for iter := 0; iter < maxIterations && remainingBudget > 1e-9; iter++ {
		// Find eligible challengers (have headroom)
		eligible := make([]string, 0)
		for _, cid := range challengers {
			if headroom[cid] > 1e-9 {
				eligible = append(eligible, cid)
			}
		}
		if len(eligible) == 0 {
			break
		}

		// Snapshot budget for this round (v1.1.2: avoid order bias)
		budgetSnapshot := remainingBudget

		// Calculate normalized weights for eligible challengers
		eligibleWeightSum := 0.0
		for _, cid := range eligible {
			eligibleWeightSum += weights[cid]
		}

		// Distribute proportionally based on UCB weights
		adds := make(map[string]float64)
		if eligibleWeightSum <= 1e-9 {
			// Equal distribution if weights sum to zero
			share := budgetSnapshot / float64(len(eligible))
			for _, cid := range eligible {
				adds[cid] = math.Min(share, headroom[cid])
			}
		} else {
			for _, cid := range eligible {
				normalizedWeight := weights[cid] / eligibleWeightSum
				desired := budgetSnapshot * normalizedWeight
				adds[cid] = math.Min(desired, headroom[cid])
			}
		}

		// Apply additions from snapshot
		totalAdded := 0.0
		for cid, add := range adds {
			allocations[cid] += add
			headroom[cid] -= add
			totalAdded += add
		}
		remainingBudget -= totalAdded
	}

	// Step 5: Normalize and ensure champion floor
	return b.normalize(allocations, championID)
}

// selectTopChallengers returns top MaxChallengers by UCB score.
// Challengers beyond the limit are effectively sent to a cooling queue
// (not included in allocation).
func (b *BudgetAllocator) selectTopChallengers(
	challengerIDs []string,
	historicalPerf map[string][]float64,
) []string {
	if len(challengerIDs) <= MaxChallengers {
		return challengerIDs
	}

	// Calculate UCB scores for all challengers
	type scored struct {
		id    string
		score float64
	}
	scores := make([]scored, len(challengerIDs))
	totalCycles := b.totalCycles(historicalPerf)

	for i, cid := range challengerIDs {
		perf := historicalPerf[cid]
		nCycles := b.priorNCycles
		meanReturn := b.priorMeanReturn

		if len(perf) > 0 {
			nCycles = len(perf)
			meanReturn = b.mean(perf)
		}

		explorationBonus := b.explorationFactor * math.Sqrt(
			math.Log(float64(totalCycles)+1)/float64(nCycles),
		)
		scores[i] = scored{id: cid, score: meanReturn + explorationBonus}
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Return top MaxChallengers
	result := make([]string, MaxChallengers)
	for i := 0; i < MaxChallengers; i++ {
		result[i] = scores[i].id
	}
	return result
}

// calcUCBWeights calculates softmax weights from UCB scores.
// UCB = mean_return + exploration_factor * sqrt(log(total_cycles+1) / n_cycles)
// Weights are computed via softmax with numerical stability (subtract max).
func (b *BudgetAllocator) calcUCBWeights(
	challengers []string,
	historicalPerf map[string][]float64,
) map[string]float64 {
	totalCycles := b.totalCycles(historicalPerf)

	// Calculate UCB scores
	scores := make([]float64, len(challengers))
	maxScore := math.Inf(-1)

	for i, cid := range challengers {
		perf := historicalPerf[cid]
		nCycles := b.priorNCycles
		meanReturn := b.priorMeanReturn

		if len(perf) > 0 {
			nCycles = len(perf)
			meanReturn = b.mean(perf)
		}

		explorationBonus := b.explorationFactor * math.Sqrt(
			math.Log(float64(totalCycles)+1)/float64(nCycles),
		)
		scores[i] = meanReturn + explorationBonus

		// Clip to [-5, 5] for numerical stability in softmax
		scores[i] = math.Max(-5, math.Min(5, scores[i]))

		if scores[i] > maxScore {
			maxScore = scores[i]
		}
	}

	// Softmax with numerical stability (subtract max)
	expSum := 0.0
	expScores := make([]float64, len(scores))
	for i, s := range scores {
		expScores[i] = math.Exp(s - maxScore)
		expSum += expScores[i]
	}

	weights := make(map[string]float64)
	for i, cid := range challengers {
		weights[cid] = expScores[i] / expSum
	}
	return weights
}

// normalize ensures sum=1 and champion >= ChampionAbsoluteFloor.
// After champion floor adjustment, challenger min/max constraints are re-enforced.
func (b *BudgetAllocator) normalize(alloc BudgetAllocation, championID string) BudgetAllocation {
	total := b.sumBudget(alloc)
	if total <= 0 {
		return alloc
	}

	// First normalize to sum=1
	for k := range alloc {
		alloc[k] /= total
	}

	// Check champion floor
	if alloc[championID] < ChampionAbsoluteFloor {
		alloc[championID] = ChampionAbsoluteFloor

		// Rescale challengers to fit remaining budget
		challengerTotal := 1.0 - ChampionAbsoluteFloor
		challengerSum := 0.0
		for k, v := range alloc {
			if k != championID {
				challengerSum += v
			}
		}

		if challengerSum > 0 {
			scale := challengerTotal / challengerSum
			for k := range alloc {
				if k != championID {
					alloc[k] *= scale
					// Re-enforce min/max after scaling (v1.1.2 requirement)
					alloc[k] = math.Max(ChallengerMinBudget, math.Min(ChallengerMaxBudget, alloc[k]))
				}
			}
		}

		// Final normalize to ensure sum=1 after re-clamping
		total = b.sumBudget(alloc)
		for k := range alloc {
			alloc[k] /= total
		}
	}

	return alloc
}

// sumBudget returns the sum of all allocations.
func (b *BudgetAllocator) sumBudget(alloc BudgetAllocation) float64 {
	sum := 0.0
	for _, v := range alloc {
		sum += v
	}
	return sum
}

// totalCycles returns the total number of performance observations across all strategies.
// Returns 1 if no observations exist to avoid division by zero.
func (b *BudgetAllocator) totalCycles(perf map[string][]float64) int {
	total := 0
	for _, v := range perf {
		total += len(v)
	}
	if total == 0 {
		return 1
	}
	return total
}

// mean returns the arithmetic mean of a slice of float64 values.
// Returns 0 if the slice is empty.
func (b *BudgetAllocator) mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
