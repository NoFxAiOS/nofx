// Exit metadata enrichment (2026-09-29): the fill sync stamps EVERY close
// 'sync' — exchange-side SL/TP triggers, AI closes and manual exchange-UI
// closes all land in the same userTrades — so 251/254 closed rows were
// indistinguishable and exit-path attribution (which ladder tier actually
// exits trades) was impossible. This file adds the two missing datasets:
//
//  1. close_reason classification — a per-cycle pass that narrows recent
//     'sync' rows using state only the AutoTrader holds: the recorded
//     SL/TP (price-matched against the exit fill), the TP-runner/split-TP
//     marker, and a close-INTENT registry that program close paths mark
//     before placing their orders.
//  2. MAE/MFE replay — every closed row gets its max adverse/favorable
//     excursion from 1m klines (entry → exit), in % of entry and in R
//     against the write-once opening stop. This is the raw material for
//     offline exit-ladder ablation (MFE distribution → where the TP
//     belongs, MAE distribution → how tight the stop can be).
package trader

import (
	"math"
	"nofx/logger"
	"nofx/market"
	"strings"
	"time"
)

// closeIntentTTL bounds how long a program close intent stays classifiable —
// a trim intent must not swallow a stop-out that fires an hour later (the
// price-match branches outrank intents anyway; the TTL only bounds the
// fallback).
const closeIntentTTL = 30 * time.Minute

// exitClassifyWindow: how far back the classifier looks for unclassified
// 'sync' rows. Covers a restart gap plus a few cycles.
const exitClassifyWindow = 60 * time.Minute

// exitBackfillPerCycle bounds the MAE/MFE replay's API spend per cycle
// (GetKlinesRange pages at 1500 bars; a 4h hold is one page).
const (
	exitBackfillPerCycle    = 3
	exitReplayMaxAgeDays    = 30 // beyond this the 1m history is not worth fetching
	exitReplaySentinel      = -1.0
	exitPriceMatchTolerance = 0.005 // exit fill within 0.5% of the recorded level
)

type closeIntent struct {
	reason string
	at     time.Time
}

// markCloseIntent registers WHY a program path is about to close (part of)
// this position. The per-cycle classifier consumes it when the exit price
// matches no recorded level. side is lowercase ("long"/"short").
func (at *AutoTrader) markCloseIntent(symbol, side, reason string) {
	at.closeIntentsMu.Lock()
	defer at.closeIntentsMu.Unlock()
	if at.closeIntents == nil {
		at.closeIntents = make(map[string]closeIntent)
	}
	at.closeIntents[keyOf(symbol, side)] = closeIntent{reason: reason, at: time.Now()}
}

// consumeCloseIntent pops the freshest intent for a position ("" when none or
// stale).
func (at *AutoTrader) consumeCloseIntent(symbol, side string) string {
	at.closeIntentsMu.Lock()
	defer at.closeIntentsMu.Unlock()
	key := keyOf(symbol, side)
	intent, ok := at.closeIntents[key]
	if !ok {
		return ""
	}
	delete(at.closeIntents, key)
	if time.Since(intent.at) > closeIntentTTL {
		return ""
	}
	return intent.reason
}

func keyOf(symbol, side string) string {
	return strings.ToUpper(symbol) + "|" + strings.ToLower(side)
}

// classifyExitReason maps a closed row to its exit path. Priority:
// price-evidence (recorded SL, then recorded TP) outranks intent markers,
// because a stop that fires after a trim must not inherit the trim's intent;
// the runner marker covers trailing-managed exits (TP-runner conversion and
// split-TP remainders) whose fill sits near no static level; intents are the
// fallback for program closes at market; anything else left the position via
// the exchange UI.
func classifyExitReason(side string, exitPrice, entryPrice, recordedSL, recordedTP float64, runnerDone bool, intentReason string) string {
	if exitPrice <= 0 {
		return "sync"
	}
	if recordedSL > 0 && math.Abs(exitPrice-recordedSL)/recordedSL <= exitPriceMatchTolerance {
		// The ratchet only ever moves the stop to/past entry — an SL at or
		// beyond entry is a trailing/breakeven exit, not a loss-side stop.
		if (side == "long" && entryPrice > 0 && recordedSL >= entryPrice) ||
			(side == "short" && entryPrice > 0 && recordedSL <= entryPrice) {
			return "trailing_stop"
		}
		return "stop_loss"
	}
	if recordedTP > 0 && math.Abs(exitPrice-recordedTP)/recordedTP <= exitPriceMatchTolerance {
		return "take_profit"
	}
	if runnerDone {
		return "trend_runner"
	}
	if intentReason != "" {
		return intentReason
	}
	// 2026-10-03 review: a LOSS beyond the recorded stop that missed the
	// 0.5% price match is a slipped stop (fast-market gap — meme-coin
	// territory), not "external". Distinct label keeps the gap visible
	// without polluting the manual-close bucket.
	lossBeyondStop := recordedSL > 0 &&
		((side == "long" && entryPrice > 0 && exitPrice < recordedSL) ||
			(side == "short" && entryPrice > 0 && exitPrice > recordedSL))
	if lossBeyondStop {
		return "stop_loss_slipped"
	}
	return "external"
}

// computeExitExcursions walks 1m bars for the position's lifetime and returns
// the max adverse / favorable excursion as % of entry (≥0), side-aware.
// The bar containing the entry fill may open seconds before the fill — sub-
// minute boundary noise accepted in exchange for not needing tick data.
func computeExitExcursions(bars []market.Kline, side string, entry float64) (maePct, mfePct float64) {
	if entry <= 0 || len(bars) == 0 {
		return 0, 0
	}
	best, worst := entry, entry
	for _, k := range bars {
		if k.High > best {
			best = k.High
		}
		if k.Low > 0 && k.Low < worst {
			worst = k.Low
		}
	}
	if side == "long" {
		mfePct = math.Max(0, (best-entry)/entry*100)
		maePct = math.Max(0, (entry-worst)/entry*100)
	} else {
		// short: favorable = price DOWN (the lowest low), adverse = the
		// highest high.
		mfePct = math.Max(0, (entry-worst)/entry*100)
		maePct = math.Max(0, (best-entry)/entry*100)
	}
	return maePct, mfePct
}

// excursionR converts a price-pct excursion to R against the opening stop.
func excursionR(pct, entry, initialSL float64) float64 {
	if entry <= 0 || initialSL <= 0 {
		return 0 // no write-once anchor — R undefined, price-pct is the record
	}
	riskPct := math.Abs(entry-initialSL) / entry * 100
	if riskPct <= 0 {
		return 0
	}
	return pct / riskPct
}

// classifyAndEnrichExits runs once per decision cycle: narrows recent 'sync'
// close reasons, then backfills MAE/MFE for closed rows missing a replay.
func (at *AutoTrader) classifyAndEnrichExits() {
	if at.store == nil {
		return
	}
	at.classifyRecentExitReasons()
	at.backfillExitExcursions()
}

func (at *AutoTrader) classifyRecentExitReasons() {
	rows, err := at.store.Position().GetRecentSyncClosed(at.id, time.Now().Add(-exitClassifyWindow).UnixMilli(), 50)
	if err != nil {
		return
	}
	for _, row := range rows {
		side := strings.ToLower(row.Side)
		if side != "long" && side != "short" {
			continue
		}
		posKey := row.Symbol + "_" + side
		reason := classifyExitReason(side, row.ExitPrice, row.EntryPrice,
			at.GetRecordedStopLoss(row.Symbol, side),
			at.getOpenTakeProfit(row.Symbol, side),
			at.tpRunnerDone(posKey),
			at.consumeCloseIntent(row.Symbol, side))
		if reason == "" || reason == "sync" {
			continue // no evidence this cycle — leave for the next pass
		}
		if err := at.store.Position().UpdateCloseReason(row.ID, reason); err == nil {
			logger.Infof("🏷️ [%s] %s %s exit classified: %s (exit %.6g, entry %.6g)",
				at.name, row.Symbol, side, reason, row.ExitPrice, row.EntryPrice)
		}
	}
}

func (at *AutoTrader) backfillExitExcursions() {
	// market.GetKlinesRange is Binance-fapi-only — wrong prices for any other
	// exchange deployment; paper uses the same public Binance futures prices.
	if !isBinanceFuturesLike(at.exchange) {
		return
	}
	rows, err := at.store.Position().GetClosedMissingExcursions(at.id, exitBackfillPerCycle)
	if err != nil {
		return
	}
	for _, row := range rows {
		side := strings.ToLower(row.Side)
		entry := time.UnixMilli(row.EntryTime)
		exit := time.UnixMilli(row.ExitTime)
		if side != "long" && side != "short" || row.EntryPrice <= 0 {
			continue
		}
		if exit.Sub(entry) > exitReplayMaxAgeDays*24*time.Hour {
			at.store.Position().UpdateExitExcursions(row.ID, exitReplaySentinel, exitReplaySentinel, exitReplaySentinel, exitReplaySentinel)
			continue
		}
		bars, err := market.GetKlinesRange(row.Symbol, "1m", entry, exit)
		if err != nil || len(bars) == 0 {
			// Transient (proxy/API) — retried next cycle by the mae_pct=0 filter.
			logger.Infof("📉 [%s] MAE/MFE replay deferred for %s #%d: %v", at.name, row.Symbol, row.ID, err)
			continue
		}
		maePct, mfePct := computeExitExcursions(bars, side, row.EntryPrice)
		maeR, mfeR := 0.0, 0.0
		if row.InitialStopLoss > 0 {
			maeR = excursionR(maePct, row.EntryPrice, row.InitialStopLoss)
			mfeR = excursionR(mfePct, row.EntryPrice, row.InitialStopLoss)
		}
		if err := at.store.Position().UpdateExitExcursions(row.ID, maePct, mfePct, maeR, mfeR); err == nil {
			logger.Infof("📉 [%s] %s %s #%d replayed: MAE %.2f%% (%.2fR) / MFE %.2f%% (%.2fR) over %d 1m bars",
				at.name, row.Symbol, side, row.ID, maePct, maeR, mfePct, mfeR, len(bars))
		}
	}
}

// closePositionReasoned marks the intent then market-closes the position —
// every program close goes through here so its fills classify by intent when
// the exit price matches no recorded level.
func (at *AutoTrader) closePositionReasoned(symbol, side, reason string) error {
	at.markCloseIntent(symbol, side, reason)
	return at.emergencyClosePosition(symbol, side)
}
