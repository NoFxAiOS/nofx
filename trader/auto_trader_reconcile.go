package trader

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"nofx/logger"
	"nofx/store"
	notify "nofx/telegram/notify"
	"nofx/trader/binance"
	"nofx/trader/types"
	"strings"
	"time"
)

// ============================================================================
// Startup reconciliation for limit-entry orders
//
// pendingEntries is memory-only; a restart loses ownership of any resting
// GTC limit entry: nobody manages its expiry and a fill would go unprotected
// (SL/TP are placed by the pending lifecycle on FILLED). Three mechanisms
// close that hole:
//
//  1. Write-through shadow rows (store.PendingEntryDB) — set/drop mirror the
//     in-memory map on every transition.
//  2. On-exchange class tags — limit entries are placed with a "lim-" client
//     order ID (getBrOrderIDFor), so the exchange itself says which resting
//     LIMIT orders belong to this subsystem.
//  3. ReconcilePendingEntries on startup — shadow rows re-claim live orders;
//     anything left tagged "lim-" but unclaimed is an orphan and gets
//     cancelled; untagged orders (grid, protective, pre-tag legacy) are NEVER
//     touched.
// ============================================================================

// entryTagPrefix marks AI limit-ENTRY orders inside the client order ID
// (binance.getBrOrderIDFor carries it after the broker referral prefix).
// Grid orders, protective algo orders, and everything placed before this tag
// existed carry no entry tag — reconciliation treats those as unowned and
// never cancels them.
const entryTagPrefix = "lim-"

// entryClientID builds the per-order client ID tag: "lim-<hash4>-<ms>"
// (exactly 22 chars — the space left by the broker prefix inside Binance's
// 32-char cap). The 4-hex trader hash disambiguates multiple traders sharing
// one exchange account — a restart of trader A must not claim trader B's
// order. getBrOrderIDFor truncates from the right, so the "lim-<hash>" head
// always survives.
func (at *AutoTrader) entryClientID() string {
	return fmt.Sprintf("lim-%s-%d", traderHash4(at.id), time.Now().UnixMilli())
}

// traderHash4 is the stable 4-hex trader fingerprint embedded in entry tags.
func traderHash4(traderID string) string {
	sum := sha256.Sum256([]byte(traderID))
	return hex.EncodeToString(sum[:2])
}

// entryOrderOwnedBy reports whether an exchange client order ID carries the
// entry tag AND this trader's hash. The ID arrives in the full on-exchange
// form "<broker prefix>lim-<hash4>-<ms>" (binance.BrIDPrefix) — strip that
// first; empty/untagged IDs (pre-tag legacy, grid, protective) → false, never
// ours to judge.
func entryOrderOwnedBy(clientID, traderID string) bool {
	tag := strings.TrimPrefix(clientID, binance.BrIDPrefix)
	if tag == clientID || !strings.HasPrefix(tag, entryTagPrefix) {
		return false
	}
	rest := tag[len(entryTagPrefix):]
	return len(rest) >= 5 && rest[:4] == traderHash4(traderID) && rest[4] == '-'
}

// ReconcilePendingEntries runs once at trader startup, after pendingEntries
// maps exist and before the first decision cycle. Steps:
//
//  1. shadow rows → re-claim: verify each persisted order on the exchange;
//     still resting → back into pendingEntries; gone (filled while offline /
//     cancelled externally / expired) → resolve accordingly (fill = place
//     protection NOW at the ACTUAL fill; cancel = protect any executed
//     residue, then drop row).
//  2. exchange scan → orphan sweep (Binance/paper — the lim- client-ID format
//     is verified there): open LIMIT orders tagged lim-<this trader> with no
//     shadow row and no map entry are unowned leftovers — cancelled for
//     re-evaluation. Untagged orders are left alone.
//
// F7 (2026-10-01 review): step 1 is adapter-generic (GetOrderStatus + the
// protective-placement interface) and now runs on EVERY exchange, not just
// Binance — bybit/okx/bitget/hyperliquid/aster/lighter place limit entries
// too and lost all restart recovery. A failed status check re-claims the row
// into the live pending map instead of parking the order unmanaged for the
// session; an offline FILLED resolution places protection from the ACTUAL
// accumulated fill and only deletes the plan row once protection is
// confirmed (or re-claims the row for retry); an offline cancel/expiry
// protects a partial fill's executed slice before dropping the row.
func (at *AutoTrader) ReconcilePendingEntries() {
	if at.store == nil {
		return
	}

	logger.Infof("🔧 [%s] Pending-entry reconciliation: checking shadow rows + open orders", at.name)

	// owned tracks every order with a legitimate claim: map entries (re-claimed
	// or placed this session) plus rows we couldn't verify. The orphan sweep
	// only cancels tagged orders absent from here.
	owned := map[string]bool{}
	at.pendingEntriesMu.RLock()
	for _, pe := range at.pendingEntries {
		owned[pe.Symbol+"|"+pe.OrderID] = true
	}
	at.pendingEntriesMu.RUnlock()

	// --- Step 1: re-claim from shadow rows ------------------------------
	rows, err := at.store.PendingEntry().List(at.id)
	if err != nil {
		// 2026-10-03 review: a failed OWNERSHIP read must skip the whole
		// reconcile — continuing leaves `owned` empty and step 2 cancels
		// every lim- tagged order on the account as "orphaned" on a
		// transient DB error.
		logger.Errorf("⚠️ [%s] Pending-entry reconciliation ABORTED: shadow row list failed: %v — orphan sweep skipped this boot", at.name, err)
		return
	}
	for _, row := range rows {
		status, err := at.trader.GetOrderStatus(row.Symbol, row.OrderID)
		if err != nil {
			// F7b: an unknowable state (network/auth) used to park the order
			// unmanaged for the whole session — re-claim it into the pending
			// map so the live lifecycle (per-cycle status poll, expiry,
			// fill protection) keeps retrying from now on, and count the
			// order as owned so the sweep never cancels on a blind read.
			pe := pendingEntryFromRow(row)
			at.setPendingEntry(&pe)
			owned[row.Symbol+"|"+row.OrderID] = true
			logger.Infof("⚠️ [%s] Pending-entry %s (order %s) status check failed: %v — row re-claimed into the live lifecycle for retry",
				at.name, row.Symbol, row.OrderID, err)
			continue
		}
		// An UNKNOWN-submit row is keyed by client ID; the exchange answered,
		// so continue with the real order ID (marks/attribution need it).
		if isUnknownEntryRef(row.OrderID) {
			if id, ok := orderIDString(status); ok {
				row.OrderID = id
			}
		}
		st, _ := status["status"].(string)
		switch strings.ToUpper(st) {
		case "NEW", "PARTIALLY_FILLED":
			pe := pendingEntryFromRow(row)
			at.setPendingEntry(&pe)
			owned[row.Symbol+"|"+row.OrderID] = true
			logger.Infof("🔧 [%s] Pending-entry re-claimed: %s %s %.6g @ %.6g (order %s)", at.name, pe.Symbol, pe.Side, pe.Quantity, pe.Price, pe.OrderID)
		case "FILLED":
			at.finalizePendingFill(row, status)
		default: // CANCELED / EXPIRED / REJECTED
			// F7c: a cancel/expiry after a partial fill must protect the
			// executed slice BEFORE the plan row goes away — the live
			// CANCELED branch has always done this; the offline path
			// silently dropped the residue.
			pe := pendingEntryFromRow(row)
			// 2026-10-20 review area2 #3: an executed slice may have been
			// closed by hand before this restart — protecting/marking it then
			// would claim a position that no longer exists (and a manual
			// successor on the same symbol+side). Confirmed flat → drop the
			// row and clear only this order's own mark.
			if statusFloat(status, "executedQty") > 0 {
				if found, known := at.offlineFillPositionState(row); known && !found {
					at.dropPendingEntry(row.Symbol, row.Side)
					if err := at.store.PendingEntry().Delete(at.id, row.Symbol, row.Side); err != nil {
						logger.Infof("⚠️ [%s] Pending-entry %s: shadow row delete failed: %v", at.name, row.Symbol, err)
					}
					at.unmarkAIManagedIfEntry(row.Symbol, row.Side, row.OrderID)
					logger.Infof("🔧 [%s] Pending-entry %s (order %s) %s with a fill while offline but no %s position exists now — row dropped, no protection placed",
						at.name, row.Symbol, row.OrderID, st, row.Side)
					continue
				}
			}
			at.setPendingEntry(&pe)
			at.protectExecutedSlice(&pe, status)
			if !pendingProtectionComplete(&pe, status) {
				continue
			}
			at.dropPendingEntry(pe.Symbol, pe.Side)
			if err := at.store.PendingEntry().Delete(at.id, row.Symbol, row.Side); err != nil {
				logger.Infof("⚠️ [%s] Pending-entry %s: shadow row delete failed: %v", at.name, row.Symbol, err)
			}
			logger.Infof("🔧 [%s] Pending-entry %s (order %s) already %s while offline — row dropped (executed slice protected if any)", at.name, row.Symbol, row.OrderID, st)
		}
	}

	// --- Step 2: orphan sweep on the exchange (Binance tag format) ------
	if !isBinanceFuturesLike(at.exchange) {
		return
	}
	grid, ok := at.trader.(interface {
		GetOpenOrders(symbol string) ([]types.OpenOrder, error)
		CancelOrder(symbol, orderID string) error
	})
	if !ok {
		return
	}
	// A tagged open order with no legitimate claim is unowned.
	open, err := grid.GetOpenOrders("")
	if err != nil {
		logger.Infof("⚠️ [%s] Pending-entry reconciliation: open-order scan failed: %v", at.name, err)
		return
	}
	for _, o := range open {
		if !entryOrderOwnedBy(o.ClientID, at.id) {
			continue // grid, protective, legacy — not ours to judge
		}
		if owned[o.Symbol+"|"+o.OrderID] {
			continue
		}
		if err := grid.CancelOrder(o.Symbol, o.OrderID); err != nil {
			logger.Infof("⚠️ [%s] Orphan limit entry %s (order %s) cancel failed: %v", at.name, o.Symbol, o.OrderID, err)
			continue
		}
		logger.Infof("🔧 [%s] Orphan limit entry cancelled: %s (order %s, client %s) — no owner after restart, re-evaluated next cycle",
			at.name, o.Symbol, o.OrderID, o.ClientID)
		notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🔧 孤儿限价单已撤销 %s</b>\n<i>重启后无主(order %s),已撤单重评</i>", notify.Escape(o.Symbol), o.OrderID))
	}
}

// pendingEntryFromRow rebuilds the in-memory plan from a durable shadow row.
func pendingEntryFromRow(row *store.PendingEntryDB) pendingEntry {
	return pendingEntry{
		RecoveryReason: row.RecoveryReason,
		Symbol:         row.Symbol, Side: row.Side, Price: row.Price,
		Quantity: row.Quantity, StopLoss: row.StopLoss, TakeProfit: row.TakeProfit,
		Leverage: row.Leverage, OrderID: row.OrderID, PlacedAt: row.PlacedAt,
		ExitMode:    row.ExitMode,
		ExecutedQty: row.ExecutedQty,
		// 2026-10-03 review: carry the protection watermark too — without it
		// a re-claimed entry re-fires the "保护未完成" alert for an already-
		// protected slice (account_execution restores it, this path didn't).
		ProtectedQty: row.ProtectedQty,
	}
}

// protectOfflineResidue places protection for the executed slice of an order
// that was cancelled/expired/rejected while the process was down (F7c).
func (at *AutoTrader) protectOfflineResidue(row *store.PendingEntryDB, status map[string]interface{}) {
	if statusFloat(status, "executedQty") <= 0 {
		return
	}
	at.markAIManaged(row.Symbol, row.Side, row.OrderID)
	pe := pendingEntryFromRow(row)
	at.protectExecutedSlice(&pe, status)
}

// finalizePendingFill resolves a shadow row whose order filled while the
// process was down: place protective orders anchored at the ACTUAL
// accumulated fill (F7d — the old path used the PLANNED quantity and the
// limit price), keep the plan live for retry when protection does not
// complete, and only then drop the row.
func (at *AutoTrader) finalizePendingFill(row *store.PendingEntryDB, status map[string]interface{}) {
	// R6 (2026-09-26 review): the offline fill IS an AI fill (this is the
	// AI's own pending entry), so ownership must exist before the hands-off
	// watchdog can see the position. 2026-10-20 review area2 #3: but only
	// AFTER the exchange confirms a position on this side — marking first
	// left a live mark behind when the user had closed the position by hand,
	// and the next manual position on the same symbol+side was then adopted.
	// Unconfirmed paths below keep the plan live; the pending lifecycle marks
	// at fill time, so they cannot leave the position unowned.
	pe := pendingEntryFromRow(row)
	executed, _, valid := fillReceipt(status)
	if !valid || executed <= 0 {
		at.setPendingEntry(&pe)
		return
	}
	if cache, ok := at.trader.(interface{ InvalidateAccountCache() }); ok {
		cache.InvalidateAccountCache()
	}

	// Position may have been closed externally in the offline window; check
	// before placing protection, otherwise the protective orders would open a
	// position out of thin air (reduce-only semantics differ per exchange).
	// F7c: a FAILED check no longer drops the task — the plan is re-claimed
	// into the live pending map and retried next cycle.
	positions, err := at.trader.GetPositions()
	if err != nil {
		at.setPendingEntry(&pe)
		logger.Infof("⚠️ [%s] Pending-entry %s FILLED while offline: position check failed: %v — plan kept live, retried next cycle",
			at.name, row.Symbol, err)
		notify.Notify("RISK", at.name, fmt.Sprintf("<b>⚠️ %s 限价单离线期间成交</b>\n<i>持仓校验失败(%v),恢复计划已保留,下周期重试</i>", notify.Escape(row.Symbol), err))
		return
	}
	posSide := row.Side
	if posSide != "short" {
		posSide = "long"
	}
	found := false
	for _, pos := range positions {
		if pos["symbol"] == row.Symbol && pos["side"] == posSide {
			found = true
			break
		}
	}
	if !found {
		if err := at.store.PendingEntry().Delete(at.id, row.Symbol, row.Side); err != nil {
			logger.Infof("⚠️ [%s] Pending-entry %s: shadow row delete failed: %v", at.name, row.Symbol, err)
		}
		// A mark left by this very entry order (written before the position
		// went away) would authorize automation on the next manual position.
		// Marks of other entry orders (a newer AI generation) are kept.
		at.unmarkAIManagedIfEntry(row.Symbol, row.Side, row.OrderID)
		logger.Infof("🔧 [%s] Pending-entry %s FILLED while offline but no %s position exists now (closed externally?) — row dropped, no protection placed",
			at.name, row.Symbol, posSide)
		return
	}

	// Position confirmed: now the fill is provably ours to own.
	at.markAIManaged(row.Symbol, row.Side, row.OrderID)
	at.protectExecutedSlice(&pe, status)
	if !pendingProtectionComplete(&pe, status) {
		// Protection did not complete — keep the plan live (in-memory map +
		// durable row) so the pending lifecycle retries the missing legs.
		at.setPendingEntry(&pe)
		logger.Infof("⚠️ [%s] Pending-entry %s FILLED while offline: protection incomplete (%.6g/%.6g) — plan kept live for retry",
			at.name, row.Symbol, pe.ProtectedQty, executed)
		notify.Notify("ALERT", at.name, fmt.Sprintf(
			"<b>⚠️ 离线成交保护未完成 %s</b>\n<i>%s 成交 %.6g,已保护 %.6g —— 恢复计划保留,下周期重试;持续失败请人工核查</i>",
			notify.Escape(row.Symbol), row.Side, executed, pe.ProtectedQty))
		return
	}
	if err := at.store.PendingEntry().Delete(at.id, row.Symbol, row.Side); err != nil {
		logger.Infof("⚠️ [%s] Pending-entry %s: shadow row delete failed: %v", at.name, row.Symbol, err)
	}
	logger.Infof("✅ [%s] Pending-entry %s FILLED while offline: protective orders placed at actual fill (plan SL %.6g / TP %.6g, protected %.6g)",
		at.name, row.Symbol, row.StopLoss, row.TakeProfit, pe.ProtectedQty)
	notify.Notify("ORDER", at.name, fmt.Sprintf("<b>📌 限价入场离线成交 %s</b>\n<i>%s,保护单已按实际成交价补挂</i>", notify.Escape(row.Symbol), row.Side))
}

// offlineFillPositionState reports whether the exchange currently holds a
// position on the shadow row's side. known=false when the check itself failed
// (callers must then treat the state as unconfirmed, never as flat).
func (at *AutoTrader) offlineFillPositionState(row *store.PendingEntryDB) (found, known bool) {
	if cache, ok := at.trader.(interface{ InvalidateAccountCache() }); ok {
		cache.InvalidateAccountCache()
	}
	positions, err := at.trader.GetPositions()
	if err != nil {
		return false, false
	}
	posSide := row.Side
	if posSide != "short" {
		posSide = "long"
	}
	for _, pos := range positions {
		if pos["symbol"] == row.Symbol && pos["side"] == posSide {
			return true, true
		}
	}
	return false, true
}
