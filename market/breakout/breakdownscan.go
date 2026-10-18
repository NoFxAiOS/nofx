package breakout

import (
	"fmt"
	"math"

	"nofx/logger"
)

// ============================================================================
// Breakdown universe (A1, QUANT_REVIEW_2026-09-22): the short-side pool used
// to be built ONLY from coins that had pumped (24h gainers + gainer history +
// grinding tops) — a survivorship universe. Coins that had already broken
// down and were grinding lower never became short candidates, and the
// multi-source long pool vs pump-only short pool made the candidate flow
// structurally long-biased (the code said so itself in a comment).
//
// This universe takes the 24h LOSER board (same ticker snapshot the gainer
// board already uses — zero extra API cost, same liquidity floor) and ranks
// it for CONTINUATION shorts (sell the bounce in an established downtrend)
// with its own transparent composite — NOT the pump-fade composite, whose
// dimensions (stretch/overbought/parabolic) measure pump-ness and would
// correctly score a dumped coin near zero, burying every breakdown candidate
// below the pump-fade pool and making this fix cosmetic.
//
// The composite is gated: no established 4h downtrend → the coin is not a
// breakdown-continuation candidate (a flash-crash rebound belongs to other
// universes) and is skipped. Like every scanner output it is NEUTRAL
// EVIDENCE for the model, never a trade conclusion.
// ============================================================================

// BreakdownUniverse is how many bottom-of-board (24h losers) coins each
// short-scan run analyzes.
const BreakdownUniverse = 30

// Breakdown composite weights (sum 1.0) — transparent by design: structure
// continuation evidence dominates, then room-to-fall, then the tie-breakers.
//
// 2026-10-18 rework: the old composite reused the pump-fade Structure
// (fake breakout / fresh EMA20 break) and bearish-Divergence dimensions, which
// are ~0 for a coin already in an established downtrend — 45% of the weight
// was dead, the score topped out near 26 and never cleared the pool floor.
// Structure is now a continuation measure (breakdownContinuation) and the
// relative-weakness vs BTC replaces Divergence.
const (
	bkwContinuation  = 0.35
	bkwRoomToFall    = 0.25
	bkwRejection     = 0.15
	bkwRelWeak       = 0.10
	bkwFundingHealth = 0.15
)

// breakdownContinuation scores whether a downtrend is actively continuing:
// a failed bounce into the 1h EMA20 (weight .45), a fresh 24-bar low (.30)
// and a lower 4h swing high (.25). printed=true when a bounce failure or a
// fresh-low close is on the tape — the continuation analogue of a topping
// confirmation. Inputs are closed bars, oldest first.
func breakdownContinuation(k1h, k4h []Kline) (score float64, printed bool) {
	n := len(k1h)
	if n < 30 || len(k4h) < 12 {
		return 0, false
	}
	c := closes(k1h)
	e20 := ema(c, 20)
	last := n - 1
	bounceFail := 0.0
	if c[last] < e20[last] {
		for i := n - 13; i < last; i++ {
			if k1h[i].High >= e20[i] && k1h[i].Close < e20[i] {
				bounceFail = 100
				break
			}
		}
	}
	newLow := 0.0
	prevMin := math.Inf(1)
	for i := n - 25; i < last; i++ {
		prevMin = math.Min(prevMin, k1h[i].Low)
	}
	if c[last] < prevMin {
		newLow = 100
	} else if k1h[last].Low < prevMin {
		newLow = 60
	}
	lowerHigh := 0.0
	m := len(k4h)
	recentHi, priorHi := math.Inf(-1), math.Inf(-1)
	for i := m - 6; i < m; i++ {
		recentHi = math.Max(recentHi, k4h[i].High)
	}
	for i := m - 12; i < m-6; i++ {
		priorHi = math.Max(priorHi, k4h[i].High)
	}
	if recentHi < priorHi {
		lowerHigh = 100
	}
	score = 0.45*bounceFail + 0.30*newLow + 0.25*lowerHigh
	return score, bounceFail == 100 || newLow == 100
}

// analyzeBreakdownShort reuses AnalyzeShort's data pipeline and component
// machinery, then re-ranks with the breakdown composite. Returns nil for a
// coin that is not in an established 4h downtrend (gate, not score).
func analyzeBreakdownShort(symbol string, chg24 float64, btc4h []Kline, ds DataSource) (*ShortSignal, error) {
	sig, err := AnalyzeShort(symbol, chg24, btc4h, ds)
	if err != nil {
		return nil, err
	}

	// Gate: established downtrend on 4h (EMA20 < EMA50 and price below
	// EMA20). Without it this is not a continuation setup — a coin that
	// simply had one bad day is not what this universe shortlists.
	k4h, err := ds.Klines("4h", 84)
	if err != nil {
		return nil, err
	}
	k1h, err := ds.Klines("1h", 60)
	if err != nil {
		return nil, err
	}
	c4h := closes(k4h)
	e20 := ema(c4h, 20)
	e50 := ema(c4h, 50)
	if len(e20) < 2 || len(e50) < 2 {
		return nil, fmt.Errorf("insufficient 4h history for %s", symbol)
	}
	last4h := c4h[len(c4h)-1]
	downTrend := e20[len(e20)-1] < e50[len(e50)-1] && last4h < e20[len(e20)-1]
	if !downTrend {
		return nil, nil
	}

	// Room to fall: distance BELOW the 4h EMA20. Right at the EMA = maximum
	// room (a rally back to the EMA is the entry zone); far stretched below
	// = exhausted, chasing the climax is exactly the -1R-blade short the
	// system wants to avoid.
	downExt := 0.0
	if e20[len(e20)-1] > 0 {
		downExt = (e20[len(e20)-1] - last4h) / e20[len(e20)-1] * 100
	}
	if downExt < 0 {
		downExt = 0
	}
	// Centre -3 / width 1.5: at the EMA ≈ 88, 3% below ≈ 50, 6% below ≈ 12 (the
	// old -1.5/4.0 curve capped at 59 even at the entry zone).
	roomToFall := clamp100(sigmoidScore(-downExt, -3.0, 1.5))

	// Funding health for a NEW short: positive funding = longs pay shorts
	// (favorable carry, uncrowded) → high; deeply negative = shorts are the
	// crowded side (squeeze fuel) → low.
	fundingHealth := clamp100(sigmoidScore(sig.FundingAnnualPct, 0, 12))

	cont, printed := breakdownContinuation(k1h, k4h)
	sig.Components.Structure = cont
	sig.Confirmed = printed
	// Underperforming BTC over 5d = relative weakness (0 when BTC series absent → neutral 50).
	relWeak := 50.0
	if len(btc4h) >= 56 {
		relWeak = clamp100(sigmoidScore(-sig.BtcOutperform, 0, 8))
	}

	score := bkwContinuation*cont +
		bkwRoomToFall*roomToFall +
		bkwRejection*sig.Components.Rejection +
		bkwRelWeak*relWeak +
		bkwFundingHealth*fundingHealth

	sig.Score = round2(score)
	// Same discipline as the pump side: no printed confirmation → never
	// strong (a weak-looking coin is "looks weak", not a confirmed short).
	// Routed through the single capped grading exit (review 2026-10-04 #5).
	sig.Grade = finalizeShortGrade(sig.Score, sig.Confirmed)
	sig.Universe = "breakdown"
	sig.Reasons = append(sig.Reasons,
		fmt.Sprintf("破位宇宙:24h跌幅%.1f%%,4h趋势向下", chg24),
		fmt.Sprintf("距4h EMA20 %.1f%%(反弹入场的下行空间)", downExt))
	logger.Debugf("🩸 breakdown candidate %s: score %.0f grade %s confirmed=%v", symbol, sig.Score, sig.Grade, sig.Confirmed)
	return sig, nil
}
