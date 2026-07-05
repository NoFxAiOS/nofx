package trader

import (
	"strings"

	"nofx/kernel"
)

// ensureLongShortCoverage keeps a balanced book each cycle: it fills toward
// roughly half the MaxPositions slots long and half short. The AI still drives
// selection/sizing whenever it acts; this is a deterministic top-up — if the
// AI's decisions plus existing positions fall short of the per-direction target,
// the engine force-opens the strongest unused bullish/bearish candidates to
// reach it (never exceeding MaxPositions).
//
// Forced opens are sized from account equity via applyAutopilotFullSizeOpen and
// run through the same code-enforced risk checks (position-value ratio, minimum
// size, margin) as any other open. Guards:
//   - skipped entirely in safe mode (AI unhealthy),
//   - scoped to the vergex_signal source (the only one with directional bias),
//   - never exceeds MaxPositions,
//   - never doubles a base symbol already held or already in the decision set.
func (at *AutoTrader) ensureLongShortCoverage(decisions []kernel.Decision, ctx *kernel.Context, equity float64) []kernel.Decision {
	if at == nil || ctx == nil || at.isSafeMode() {
		return decisions
	}
	if at.config.StrategyConfig == nil || at.config.StrategyConfig.CoinSource.SourceType != "vergex_signal" {
		return decisions
	}
	if at.strategyEngine == nil || equity <= 0 {
		return decisions
	}

	maxPos := at.config.StrategyConfig.RiskControl.MaxPositions
	if maxPos < 2 {
		return decisions
	}
	// Aim to hold a balanced book: roughly half the slots long, half short.
	targetLong := (maxPos + 1) / 2
	targetShort := maxPos / 2

	held := make(map[string]bool)
	longCount, shortCount, posCount := 0, 0, 0
	for _, p := range ctx.Positions {
		held[universeBaseKey(p.Symbol)] = true
		posCount++
		if strings.EqualFold(p.Side, "long") {
			longCount++
		} else if strings.EqualFold(p.Side, "short") {
			shortCount++
		}
	}
	for _, d := range decisions {
		held[universeBaseKey(d.Symbol)] = true
		switch d.Action {
		case "open_long":
			longCount++
			posCount++
		case "open_short":
			shortCount++
			posCount++
		}
	}

	bullish, bearish := at.strategyEngine.DirectionalCandidates()

	// fill a direction up to its target, drawing from the strongest unused
	// candidates, never exceeding MaxPositions.
	fill := func(action string, cands []string, have, target int) {
		for _, c := range cands {
			if have >= target {
				return
			}
			if maxPos > 0 && posCount >= maxPos {
				return
			}
			b := universeBaseKey(c)
			if b == "" || held[b] {
				continue
			}
			d := kernel.Decision{
				Action:     action,
				Symbol:     c,
				Confidence: 70,
				Reasoning:  "Forced " + action + " to fill the balanced long/short book (autopilot)",
			}
			at.applyAutopilotFullSizeOpen(&d, equity)
			decisions = append(decisions, d)
			held[b] = true
			have++
			posCount++
			at.logInfof("⚖️ Forced %s %s (account-sized %.2f USDT, %dx)", action, c, d.PositionSizeUSD, d.Leverage)
		}
	}

	fill("open_long", bullish, longCount, targetLong)
	fill("open_short", bearish, shortCount, targetShort)
	return decisions
}
