package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
)

const defaultMaxEntryDeviationPct = 1.5

func enforceEntryPriceDeviation(decision *kernel.Decision, currentPrice float64, side string) error {
	return enforceEntryPriceDeviationWithMax(decision, currentPrice, side, 0)
}

func enforceEntryPriceDeviationWithMax(decision *kernel.Decision, currentPrice float64, side string, maxPct float64) error {
	if decision.EntryProtection == nil {
		return nil
	}
	plannedEntry := decision.EntryProtection.RiskReward.Entry
	if plannedEntry <= 0 || currentPrice <= 0 {
		return nil
	}

	if maxPct <= 0 {
		maxPct = defaultMaxEntryDeviationPct
	}

	deviationPct := math.Abs(currentPrice-plannedEntry) / plannedEntry * 100

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

	if deviationPct > maxPct {
		logger.Infof("🚫 Entry deviation blocked %s %s: planned=%.6f actual=%.6f deviation=%.2f%% (max %.1f%%)",
			decision.Symbol, side, plannedEntry, currentPrice, deviationPct, maxPct)
		return fmt.Errorf("🚫 entry price deviation too large for %s %s: planned %.6f vs current %.6f (%.2f%% > %.1f%% max)",
			decision.Symbol, side, plannedEntry, currentPrice, deviationPct, maxPct)
	}

	if deviationPct > 0.5 {
		logger.Infof("⚠️ Entry deviation warning %s %s: planned=%.6f actual=%.6f deviation=%.2f%%",
			decision.Symbol, side, plannedEntry, currentPrice, deviationPct)
	}

	return nil
}
