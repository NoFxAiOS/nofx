package trader

import (
	"math"
	"strings"

	"nofx/kernel"
)

// forcedCoverageMinScore is the minimum absolute board z-score a candidate
// needs before the engine will force-open it for book balance. Live trade
// history showed forced entries on near-neutral signals (|z| < 0.3) were a
// systematic money loser — especially shorts — while trades on strong signals
// carried the edge. Below this bar the book is simply left unbalanced.
const forcedCoverageMinScore = 0.75

// ensureLongShortCoverage tops the book up toward roughly half the
// MaxPositions slots long and half short — but only with candidates whose
// directional signal is actually strong (see forcedCoverageMinScore). The AI
// still drives selection/sizing whenever it acts; this is a deterministic
// top-up, and an unbalanced book is preferred over a forced weak trade.
//
// Forced opens are sized from account equity via applyAutopilotFullSizeOpen and
// run through the same code-enforced risk checks (position-value ratio, minimum
// size, margin) as any other open. Guards:
//   - skipped entirely in safe mode (AI unhealthy),
//   - scoped to the vergex_signal source (the only one with directional bias),
//   - requires |signal score| >= forcedCoverageMinScore,
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
	// candidates that clear the signal-strength floor, never exceeding
	// MaxPositions.
	fill := func(action string, cands []kernel.DirectionalCandidate, have, target int) {
		for _, c := range cands {
			if have >= target {
				return
			}
			if maxPos > 0 && posCount >= maxPos {
				return
			}
			if math.Abs(c.Score) < forcedCoverageMinScore {
				// candidates are rank-ordered; weaker ones may still follow,
				// so keep scanning instead of breaking
				at.logInfof("⚖️ Skipped forced %s %s: signal score %.2f below %.2f floor", action, c.Symbol, c.Score, forcedCoverageMinScore)
				continue
			}
			b := universeBaseKey(c.Symbol)
			if b == "" || held[b] {
				continue
			}
			d := kernel.Decision{
				Action:     action,
				Symbol:     c.Symbol,
				Confidence: 70,
				Reasoning:  "Forced " + action + " to fill the balanced long/short book (autopilot)",
			}
			at.applyAutopilotFullSizeOpen(&d, equity)
			decisions = append(decisions, d)
			held[b] = true
			have++
			posCount++
			at.logInfof("⚖️ Forced %s %s (score %.2f, account-sized %.2f USDT, %dx)", action, c.Symbol, c.Score, d.PositionSizeUSD, d.Leverage)
		}
	}

	fill("open_long", bullish, longCount, targetLong)
	fill("open_short", bearish, shortCount, targetShort)
	return decisions
}
