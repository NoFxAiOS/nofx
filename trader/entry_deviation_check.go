package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
)

const maxEntryDeviationPct = 1.5

// enforceEntryPriceDeviation blocks execution when the current market price
// deviates too far from the AI's planned entry price. This prevents the system
// from opening positions at prices that invalidate the original risk/reward setup.
func enforceEntryPriceDeviation(decision *kernel.Decision, currentPrice float64, side string) error {
	if decision.EntryProtection == nil {
		return nil
	}
	plannedEntry := decision.EntryProtection.RiskReward.Entry
	if plannedEntry <= 0 || currentPrice <= 0 {
		return nil
	}

	deviationPct := math.Abs(currentPrice-plannedEntry) / plannedEntry * 100

	// For longs, current price above planned entry is adverse (buying higher)
	// For shorts, current price below planned entry is adverse (selling lower)
	isAdverse := false
	switch side {
	case "long":
		isAdverse = currentPrice > plannedEntry
	case "short":
		isAdverse = currentPrice < plannedEntry
	}

	if !isAdverse {
		return nil
	}

	if deviationPct > maxEntryDeviationPct {
		logger.Infof("🚫 Entry deviation blocked %s %s: planned=%.6f actual=%.6f deviation=%.2f%% (max %.1f%%)",
			decision.Symbol, side, plannedEntry, currentPrice, deviationPct, maxEntryDeviationPct)
		return fmt.Errorf("🚫 entry price deviation too large for %s %s: planned %.6f vs current %.6f (%.2f%% > %.1f%% max)",
			decision.Symbol, side, plannedEntry, currentPrice, deviationPct, maxEntryDeviationPct)
	}

	if deviationPct > 0.5 {
		logger.Infof("⚠️ Entry deviation warning %s %s: planned=%.6f actual=%.6f deviation=%.2f%%",
			decision.Symbol, side, plannedEntry, currentPrice, deviationPct)
	}

	return nil
}
