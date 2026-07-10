package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"nofx/logger"
)

const reconcileQuantityTolerance = 0.0001

// LivePositionKey builds the map key used by ReconcileOpenPositionsWithLive.
func LivePositionKey(symbol, side string) string {
	return strings.ToUpper(strings.TrimSpace(symbol)) + "|" + strings.ToUpper(strings.TrimSpace(side))
}

// ReconcileOpenPositionsWithLive force-closes local OPEN position rows that the
// exchange no longer holds, and trims rows whose quantity exceeds what is live.
//
// Why this exists: missed or unmatched fills (position flips, liquidations,
// sync gaps) leave "zombie" OPEN rows behind. Every later close on the same
// symbol+side then lands as a partial close against the zombie, so the row
// never reaches CLOSED and its realized PnL never enters the closed-trade
// statistics — the dashboard, the Edge Profile and the AI's own track-record
// context all silently under-report. Reconciling against the exchange's live
// book is the self-healing fix: local OPEN rows must always be a subset of
// what the exchange actually holds.
//
// liveQty maps LivePositionKey(symbol, side) → live quantity on the exchange.
// Rows are matched newest-first so the freshest row survives as the live
// position's bookkeeping and older duplicates get closed.
//
// Scope is by exchange account (all trader IDs), so rows left by prior
// autopilot incarnations on the same exchange are reconciled too.
func (s *PositionStore) ReconcileOpenPositionsWithLive(exchangeID string, liveQty map[string]float64) (int, error) {
	openRows, err := s.GetOpenPositionsByExchange(exchangeID)
	if err != nil {
		return 0, fmt.Errorf("failed to list open positions: %w", err)
	}
	if len(openRows) == 0 {
		return 0, nil
	}

	// Copy so we can consume quantities without mutating the caller's map.
	remaining := make(map[string]float64, len(liveQty))
	for k, v := range liveQty {
		remaining[strings.ToUpper(k)] = v
	}

	// Newest first: the most recent row keeps representing the live position.
	sort.Slice(openRows, func(i, j int) bool {
		return openRows[i].EntryTime > openRows[j].EntryTime
	})

	nowMs := time.Now().UTC().UnixMilli()
	closed := 0

	for _, row := range openRows {
		key := LivePositionKey(row.Symbol, row.Side)
		live := remaining[key]

		if live > reconcileQuantityTolerance {
			// The exchange still holds (part of) this key — this row survives.
			if row.Quantity > live+reconcileQuantityTolerance {
				// Trim the row down to what is actually live so the next real
				// close matches sizes and can fully close it. No PnL is
				// fabricated: the trimmed residue is stale bookkeeping, not a
				// real fill.
				trim := row.Quantity - live
				exitPrice := row.ExitPrice
				if exitPrice <= 0 {
					exitPrice = row.EntryPrice
				}
				if err := s.ReducePositionQuantity(row.ID, trim, exitPrice, 0, 0); err != nil {
					logger.Infof("  ⚠️ Reconcile: failed to trim position %d (%s %s): %v", row.ID, row.Symbol, row.Side, err)
				} else {
					logger.Infof("  🧹 Reconcile: trimmed %s %s row %d by %.6f to match live %.6f", row.Symbol, row.Side, row.ID, trim, live)
				}
				remaining[key] = 0
			} else {
				remaining[key] = live - row.Quantity
			}
			continue
		}

		// Nothing (left) on the exchange for this key — the row is a zombie.
		// Close it with whatever it accumulated; exit info falls back to the
		// last known bookkeeping on the row.
		exitPrice := row.ExitPrice
		if exitPrice <= 0 {
			exitPrice = row.EntryPrice
		}
		exitTime := row.UpdatedAt
		if exitTime <= 0 {
			exitTime = nowMs
		}
		if err := s.ClosePositionFully(row.ID, exitPrice, row.ExitOrderID, exitTime, row.RealizedPnL, row.Fee, "reconcile"); err != nil {
			logger.Infof("  ⚠️ Reconcile: failed to close zombie position %d (%s %s): %v", row.ID, row.Symbol, row.Side, err)
			continue
		}
		closed++
		logger.Infof("  🧹 Reconcile: closed zombie %s %s row %d (qty %.6f, accumulated PnL %.2f) — not held on exchange", row.Symbol, row.Side, row.ID, row.Quantity, row.RealizedPnL)
	}

	if closed > 0 {
		logger.Infof("✅ Position reconcile: closed %d zombie row(s) on exchange %s", closed, exchangeID)
	}
	return closed, nil
}
