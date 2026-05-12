package trader

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/store"
	"sort"
	"strings"
)

// computeDrawdownTierAllocations computes the fixed position allocations for each drawdown tier
// at position open time. Each tier gets a fixed quantity that never changes.
// The allocation is based on the opening quantity and each tier's close_ratio_pct, which refers
// to the percentage of the ORIGINAL opening position, not the remaining position after prior tiers.
func computeDrawdownTierAllocations(totalQuantity float64, rules []store.DrawdownTakeProfitRule) []store.DrawdownTierAllocation {
	if len(rules) == 0 || totalQuantity <= 0 {
		return nil
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].MinProfitPct < rules[j].MinProfitPct
	})

	allocs := make([]store.DrawdownTierAllocation, 0, len(rules))
	allocatedPct := 0.0

	for i, rule := range rules {
		if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
			continue
		}

		ratioPct := rule.CloseRatioPct
		if allocatedPct+ratioPct > 100 {
			ratioPct = 100 - allocatedPct
		}
		if ratioPct <= 0 {
			continue
		}

		qty := totalQuantity * ratioPct / 100.0

		stageName := rule.StageName
		if stageName == "" {
			stageName = fmt.Sprintf("T%d", i+1)
		}

		allocs = append(allocs, store.DrawdownTierAllocation{
			TierIndex:      i,
			StageName:      stageName,
			Quantity:       qty,
			CloseRatioPct:  ratioPct,
			MinProfitPct:   rule.MinProfitPct,
			MaxDrawdownPct: rule.MaxDrawdownPct,
			PeakPnLPct:     0,
			Status:         "pending",
		})

		allocatedPct += ratioPct
	}

	return allocs
}

// setDrawdownTierAllocs stores the fixed tier allocations for a position.
func (at *AutoTrader) setDrawdownTierAllocs(symbol, side string, allocs []store.DrawdownTierAllocation) {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()
	if at.drawdownTierAllocs == nil {
		at.drawdownTierAllocs = make(map[string][]store.DrawdownTierAllocation)
	}
	at.drawdownTierAllocs[key] = allocs
	logger.Infof("📊 Drawdown tier allocations set for %s %s: %d tiers", symbol, side, len(allocs))
	for _, a := range allocs {
		logger.Infof("  → %s: qty=%.6f (%.1f%%) | peak_trigger=%.2f%% | drawdown=%.2f%%",
			a.StageName, a.Quantity, a.CloseRatioPct, a.MinProfitPct, a.MaxDrawdownPct)
	}
}

// getDrawdownTierAllocs returns the fixed tier allocations for a position.
func (at *AutoTrader) getDrawdownTierAllocs(symbol, side string) []store.DrawdownTierAllocation {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.RLock()
	defer at.drawdownTierAllocMu.RUnlock()
	allocs := at.drawdownTierAllocs[key]
	if len(allocs) == 0 {
		return nil
	}
	out := make([]store.DrawdownTierAllocation, len(allocs))
	copy(out, allocs)
	return out
}

// clearDrawdownTierAllocs removes tier allocations when position is fully closed.
func (at *AutoTrader) clearDrawdownTierAllocs(symbol, side string) {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()
	delete(at.drawdownTierAllocs, key)
}

// updateTierAlloc updates a single tier allocation's state.
func (at *AutoTrader) updateTierAlloc(symbol, side string, tierIndex int, update func(*store.DrawdownTierAllocation)) {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()
	allocs := at.drawdownTierAllocs[key]
	for i := range allocs {
		if allocs[i].TierIndex == tierIndex {
			update(&allocs[i])
			return
		}
	}
}

// evaluateDrawdownTiers checks all pending/tracking tiers against current P&L.
// Returns the tier that should be executed now, or nil if none.
// Single-direction upgrade: when a higher tier activates, all lower tiers are superseded.
// Each tier's CloseRatioPct is relative to the original entry quantity.
// globalPeakPnL is used to initialize a tier's peak when it first enters tracking,
// ensuring continuity for positions where peak was already reached before tier initialization.
func (at *AutoTrader) evaluateDrawdownTiers(symbol, side string, currentPnLPct, globalPeakPnL float64) *store.DrawdownTierAllocation {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()

	allocs := at.drawdownTierAllocs[key]
	if len(allocs) == 0 {
		return nil
	}

	var triggered *store.DrawdownTierAllocation

	for i := range allocs {
		tier := &allocs[i]

		if tier.Status == "executed" || tier.Status == "be_covered" || tier.Status == "superseded" {
			continue
		}

		// Tier becomes "tracking" once current P&L reaches the min_profit threshold
		if tier.Status == "pending" {
			if currentPnLPct >= tier.MinProfitPct {
				tier.Status = "tracking"
				// Initialize peak from global peak to preserve history for late-initialized tiers
				tier.PeakPnLPct = globalPeakPnL
				if currentPnLPct > tier.PeakPnLPct {
					tier.PeakPnLPct = currentPnLPct
				}
				logger.Infof("📈 Drawdown %s now tracking: %s %s | pnl=%.2f%% >= trigger=%.2f%% | peak=%.2f%%",
					tier.StageName, symbol, side, currentPnLPct, tier.MinProfitPct, tier.PeakPnLPct)
				// Single-direction upgrade: supersede all lower tiers
				for j := 0; j < i; j++ {
					if allocs[j].Status == "tracking" || allocs[j].Status == "pending" {
						allocs[j].Status = "superseded"
						logger.Infof("⏭️ Drawdown %s superseded by %s: %s %s",
							allocs[j].StageName, tier.StageName, symbol, side)
					}
				}
				// Fall through to check drawdown immediately in the same cycle
			} else {
				continue
			}
		}

		// Status == "tracking": update peak and check drawdown
		if currentPnLPct > tier.PeakPnLPct {
			tier.PeakPnLPct = currentPnLPct
		}

		// Calculate drawdown from THIS tier's peak (not global peak)
		drawdownFromPeak := 0.0
		if tier.PeakPnLPct > 0 && currentPnLPct < tier.PeakPnLPct {
			drawdownFromPeak = ((tier.PeakPnLPct - currentPnLPct) / tier.PeakPnLPct) * 100
		}

		if drawdownFromPeak >= tier.MaxDrawdownPct {
			// This tier is triggered! Return it for execution.
			// We only trigger one tier per evaluation cycle (the lowest-index active one).
			tierCopy := *tier
			triggered = &tierCopy
			tier.Status = "executed"
			logger.Infof("🚨 Drawdown %s triggered: %s %s | peak=%.2f%% current=%.2f%% drawdown=%.2f%% >= threshold=%.2f%%",
				tier.StageName, symbol, side, tier.PeakPnLPct, currentPnLPct, drawdownFromPeak, tier.MaxDrawdownPct)
			break
		}
	}

	return triggered
}

// updateDrawdownTierStates updates tier states (pending→tracking, supersede lower tiers)
// based on current P&L without triggering market closes. Used when native trailing handles
// exchange orders but we still need tier state for high-water-mark tracking.
func (at *AutoTrader) updateDrawdownTierStates(symbol, side string, currentPnLPct, globalPeakPnL float64) {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()

	allocs := at.drawdownTierAllocs[key]
	if len(allocs) == 0 {
		return
	}

	for i := range allocs {
		tier := &allocs[i]
		if tier.Status == "executed" || tier.Status == "be_covered" || tier.Status == "superseded" {
			continue
		}
		if tier.Status == "pending" {
			if currentPnLPct >= tier.MinProfitPct {
				tier.Status = "tracking"
				tier.PeakPnLPct = globalPeakPnL
				if currentPnLPct > tier.PeakPnLPct {
					tier.PeakPnLPct = currentPnLPct
				}
				logger.Infof("📈 Drawdown %s now tracking (native): %s %s | pnl=%.2f%% >= trigger=%.2f%%",
					tier.StageName, symbol, side, currentPnLPct, tier.MinProfitPct)
				for j := 0; j < i; j++ {
					if allocs[j].Status == "tracking" || allocs[j].Status == "pending" {
						allocs[j].Status = "superseded"
						logger.Infof("⏭️ Drawdown %s superseded by %s (native): %s %s",
							allocs[j].StageName, tier.StageName, symbol, side)
					}
				}
			}
		} else if tier.Status == "tracking" {
			if currentPnLPct > tier.PeakPnLPct {
				tier.PeakPnLPct = currentPnLPct
			}
		}
	}
}

// getExecutedTierCloseRatio returns the cumulative CloseRatioPct of all executed tiers for a position.
func (at *AutoTrader) getExecutedTierCloseRatio(symbol, side string) float64 {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()
	allocs := at.drawdownTierAllocs[key]
	var total float64
	for _, tier := range allocs {
		if tier.Status == "executed" {
			total += tier.CloseRatioPct
		}
	}
	return total
}

// resolveDrawdownRulesWithModes merges AI-provided rules with strategy-configured rules
// based on per-field mode settings. For each rule, if a field's mode is "ai", the AI value
// is used; if "manual", the strategy config value is used.
func resolveDrawdownRulesWithModes(strategyRules, aiRules []store.DrawdownTakeProfitRule) []store.DrawdownTakeProfitRule {
	if len(strategyRules) == 0 {
		return aiRules
	}
	if len(aiRules) == 0 {
		return strategyRules
	}

	sort.Slice(strategyRules, func(i, j int) bool {
		return strategyRules[i].MinProfitPct < strategyRules[j].MinProfitPct
	})
	sort.Slice(aiRules, func(i, j int) bool {
		return aiRules[i].MinProfitPct < aiRules[j].MinProfitPct
	})

	n := len(strategyRules)
	if len(aiRules) > n {
		n = len(aiRules)
	}

	resolved := make([]store.DrawdownTakeProfitRule, 0, n)

	for i := 0; i < n; i++ {
		var base store.DrawdownTakeProfitRule
		if i < len(strategyRules) {
			base = strategyRules[i]
		}
		var ai store.DrawdownTakeProfitRule
		if i < len(aiRules) {
			ai = aiRules[i]
		}

		result := base

		if base.CloseRatioMode == store.ProtectionValueModeAI && i < len(aiRules) && ai.CloseRatioPct > 0 {
			result.CloseRatioPct = ai.CloseRatioPct
		}
		if base.MinProfitMode == store.ProtectionValueModeAI && i < len(aiRules) && ai.MinProfitPct > 0 {
			result.MinProfitPct = ai.MinProfitPct
		}
		if base.MaxDrawdownMode == store.ProtectionValueModeAI && i < len(aiRules) && ai.MaxDrawdownPct > 0 {
			result.MaxDrawdownPct = ai.MaxDrawdownPct
		}

		if i >= len(strategyRules) && i < len(aiRules) {
			result = ai
		}

		if result.MinProfitPct > 0 && result.MaxDrawdownPct > 0 && result.CloseRatioPct > 0 {
			if result.StageName == "" {
				result.StageName = fmt.Sprintf("T%d", i+1)
			}
			resolved = append(resolved, result)
		}
	}

	// Ensure strictly increasing MinProfitPct across resolved tiers.
	for i := 1; i < len(resolved); i++ {
		if resolved[i].MinProfitPct <= resolved[i-1].MinProfitPct {
			resolved[i].MinProfitPct = resolved[i-1].MinProfitPct + 0.3
		}
	}

	return resolved
}

// hasAllTiersCompleted returns true if all tier allocations are executed or be_covered.
func hasAllTiersCompleted(allocs []store.DrawdownTierAllocation) bool {
	if len(allocs) == 0 {
		return false
	}
	for _, a := range allocs {
		if a.Status != "executed" && a.Status != "be_covered" {
			return false
		}
	}
	return true
}

// getPendingTierCount returns the number of tiers that haven't been executed yet.
func getPendingTierCount(allocs []store.DrawdownTierAllocation) int {
	count := 0
	for _, a := range allocs {
		if a.Status == "pending" || a.Status == "tracking" {
			count++
		}
	}
	return count
}

// logTierAllocStatus prints a summary of all tier allocations for debugging.
func logTierAllocStatus(symbol, side string, allocs []store.DrawdownTierAllocation) {
	if len(allocs) == 0 {
		return
	}
	var parts []string
	for _, a := range allocs {
		parts = append(parts, fmt.Sprintf("%s[%s peak=%.2f%%]", a.StageName, a.Status, a.PeakPnLPct))
	}
	logger.Infof("📊 Drawdown tiers %s %s: %s", symbol, side, strings.Join(parts, " | "))
}

// initDrawdownTiersFromResolvedRules resolves AI/manual per-field modes against strategy config,
// then computes and stores the fixed tier allocations.
func (at *AutoTrader) initDrawdownTiersFromResolvedRules(symbol, side string, quantity float64, aiRules []store.DrawdownTakeProfitRule) {
	if quantity <= 0 || len(aiRules) == 0 {
		return
	}

	var strategyRules []store.DrawdownTakeProfitRule
	if at.config.StrategyConfig != nil {
		strategyRules = at.config.StrategyConfig.Protection.DrawdownTakeProfit.Rules
	}

	resolved := resolveDrawdownRulesWithModes(strategyRules, aiRules)
	if len(resolved) == 0 {
		resolved = aiRules
	}

	at.initDrawdownTiersForPosition(symbol, side, quantity, resolved)
}

// initDrawdownTiersForPosition computes and stores tier allocations when a position is opened.
func (at *AutoTrader) initDrawdownTiersForPosition(symbol, side string, quantity float64, rules []store.DrawdownTakeProfitRule) {
	if len(rules) == 0 || quantity <= 0 {
		return
	}

	// Apply runner policy enforcement to each rule before allocation
	var cfg store.DrawdownTakeProfitConfig
	if at.config.StrategyConfig != nil {
		cfg = at.config.StrategyConfig.Protection.DrawdownTakeProfit
	}
	enforced := make([]store.DrawdownTakeProfitRule, len(rules))
	for i, rule := range rules {
		rule = normalizeDrawdownRule(rule)
		rule = enforceDrawdownRunnerPolicy(cfg, rule)
		enforced[i] = rule
	}

	allocs := computeDrawdownTierAllocations(quantity, enforced)
	if len(allocs) == 0 {
		return
	}

	at.setDrawdownTierAllocs(symbol, side, allocs)
}

// markUnreachedTiersForBE marks all pending/tracking tiers as "be_covered" so the
// breakeven system takes over for those portions.
func (at *AutoTrader) markUnreachedTiersForBE(symbol, side string) {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()

	allocs := at.drawdownTierAllocs[key]
	for i := range allocs {
		if allocs[i].Status == "pending" || allocs[i].Status == "tracking" {
			allocs[i].Status = "be_covered"
			logger.Infof("🔄 Drawdown %s marked be_covered: %s %s (unreached tier → BE takes over)",
				allocs[i].StageName, symbol, side)
		}
	}
}

// findRuleForTier locates the original DrawdownTakeProfitRule that corresponds to
// a triggered tier allocation, matching by MinProfitPct and MaxDrawdownPct.
func findRuleForTier(rules []store.DrawdownTakeProfitRule, tier *store.DrawdownTierAllocation) *store.DrawdownTakeProfitRule {
	if tier == nil || len(rules) == 0 {
		return nil
	}
	for i := range rules {
		if rules[i].MinProfitPct == tier.MinProfitPct && rules[i].MaxDrawdownPct == tier.MaxDrawdownPct {
			return &rules[i]
		}
	}
	if tier.TierIndex < len(rules) {
		return &rules[tier.TierIndex]
	}
	return nil
}

// computeRemainingQuantityForBE calculates the total quantity of tiers that haven't been
// executed by drawdown — this is the quantity that BE should protect.
func (at *AutoTrader) computeRemainingQuantityForBE(symbol, side string) float64 {
	allocs := at.getDrawdownTierAllocs(symbol, side)
	if len(allocs) == 0 {
		return 0
	}
	remaining := 0.0
	for _, a := range allocs {
		if a.Status != "executed" {
			remaining += math.Abs(a.Quantity)
		}
	}
	return remaining
}

// isDrawdownTierExecuted checks if the tier matching the given rule has already been
// executed (trailing order filled). Matches by MinProfitPct and MaxDrawdownPct.
func (at *AutoTrader) isDrawdownTierExecuted(symbol, side string, rule store.DrawdownTakeProfitRule) bool {
	allocs := at.getDrawdownTierAllocs(symbol, side)
	for _, a := range allocs {
		if a.MinProfitPct == rule.MinProfitPct && a.MaxDrawdownPct == rule.MaxDrawdownPct && a.Status == "executed" {
			return true
		}
	}
	return false
}

// detectNativeTrailingFills detects when a native trailing order has been filled
// by comparing current position quantity against expected remaining quantity from tier allocs.
// If position is smaller than expected, mark the highest "tracking" tier as executed.
func (at *AutoTrader) detectNativeTrailingFills(symbol, side string, currentQuantity float64) {
	key := positionKey(symbol, side)
	at.drawdownTierAllocMu.Lock()
	defer at.drawdownTierAllocMu.Unlock()

	allocs := at.drawdownTierAllocs[key]
	if len(allocs) == 0 {
		return
	}

	// Compute expected remaining quantity (sum of non-executed tiers)
	expectedRemaining := 0.0
	for _, a := range allocs {
		if a.Status != "executed" && a.Status != "be_covered" {
			expectedRemaining += math.Abs(a.Quantity)
		}
	}

	if expectedRemaining <= 0 {
		return
	}

	// If current quantity is significantly less than expected, a tier was filled
	// Use 5% tolerance to account for rounding
	deficit := expectedRemaining - currentQuantity
	if deficit <= expectedRemaining*0.05 {
		return
	}

	// Find the highest-index "tracking" tier and mark it as executed
	for i := len(allocs) - 1; i >= 0; i-- {
		if allocs[i].Status == "tracking" {
			tierQty := math.Abs(allocs[i].Quantity)
			if deficit >= tierQty*0.5 {
				allocs[i].Status = "executed"
				logger.Infof("✅ Drawdown %s detected as filled (native trailing): %s %s | position=%.4f expected=%.4f deficit=%.4f",
					allocs[i].StageName, symbol, side, currentQuantity, expectedRemaining, deficit)
				return
			}
		}
	}
}
