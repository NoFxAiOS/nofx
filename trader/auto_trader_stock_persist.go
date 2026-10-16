package trader

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"
	"nofx/trader/types"

	notify "nofx/telegram/notify"
)

// Protection IDs outlive openOrders: a filled OCO leg disappears from that
// endpoint, but GET /api/v3/order still supplies its cumulative executions.
func (at *AutoTrader) stockSaveProtection(p *types.ProtectionOrders) error {
	if p == nil {
		return fmt.Errorf("nil protection response")
	}
	var rows []store.StockProtectionOrder
	for _, leg := range []struct{ id, client, role string }{
		{p.StopOrderID, p.StopClientID, "stop_loss"}, {p.TakeProfitID, p.TPClientID, "take_profit"},
	} {
		if leg.id == "" {
			continue
		}
		rows = append(rows, store.StockProtectionOrder{TraderID: at.id, Symbol: p.Symbol, OrderID: leg.id,
			OrderListID: p.OrderListID, ClientOrderID: leg.client, ListClientID: p.ListClientID, Role: leg.role,
			Quantity: p.Quantity, StopPrice: p.StopPrice, StopLimitPrice: p.StopLimitPrice, TakeProfit: p.TakeProfit,
			UpdatedAt: at.stockClock().UTC().UnixMilli()})
	}
	return at.store.StockPaper().SaveProtectionOrders(rows)
}

type stockProtectionCheck struct {
	Known, Filled, Open bool
}

// Account/store reads abort the pass, even when reached through a per-symbol
// protection check or replacement.
type stockProtectionReadError struct{ error }

func (e *stockProtectionReadError) Unwrap() error { return e.error }

func stockProtectionReadFailed(err error) bool {
	var readErr *stockProtectionReadError
	return errors.As(err, &readErr)
}

func stockOrderTerminal(status string) bool {
	switch status {
	case "FILLED", "CANCELED", "CANCELLED", "EXPIRED", "REJECTED", "EXPIRED_IN_MATCH":
		return true
	}
	return false
}

// stockCheckProtection fails closed on ANY unreadable leg; absence from open
// orders alone is never evidence that the protection did not execute.
func (at *AutoTrader) stockCheckProtection(sym string, now time.Time) (out stockProtectionCheck, err error) {
	rows, err := at.store.StockPaper().ListProtectionOrders(at.id, sym)
	if err != nil {
		return out, &stockProtectionReadError{err}
	}
	out.Known = len(rows) > 0
	for _, r := range rows {
		at.stock.lastProtect[r.Symbol] = stockProtect{Stop: r.StopPrice, TakeProfit: r.TakeProfit, Qty: r.Quantity}
		if r.Retired {
			continue
		}
		if stockOrderTerminal(r.Status) {
			out.Filled = out.Filled || r.BookedQty > 0
			continue // final executions and watermark already committed atomically
		}
		status, e := at.stockTrader.GetOrderStatus(r.Symbol, r.OrderID)
		if e != nil {
			return out, fmt.Errorf("protection %s order %s status: %w", r.Symbol, r.OrderID, e)
		}
		if client, _ := status["clientOrderId"].(string); client != "" && (!strings.HasPrefix(client, "nxbs_") || (r.ClientOrderID != "" && client != r.ClientOrderID)) {
			return out, fmt.Errorf("protection %s order %s client ID mismatch", r.Symbol, r.OrderID)
		}
		s := strings.ToUpper(fmt.Sprint(status["status"]))
		switch s {
		case "NEW", "PENDING_NEW", "PENDING_CANCEL", "PARTIALLY_FILLED", "FILLED", "CANCELED", "CANCELLED", "EXPIRED", "REJECTED", "EXPIRED_IN_MATCH":
		default:
			return out, fmt.Errorf("protection %s order %s unknown status %q", r.Symbol, r.OrderID, s)
		}
		executed := numberOf(status, "executedQty")
		if (s == "FILLED" || s == "PARTIALLY_FILLED") && executed <= 0 {
			return out, fmt.Errorf("protection %s order %s reports %s without execution quantity", r.Symbol, r.OrderID, s)
		}
		quote := numberOf(status, "cummulativeQuoteQty")
		if quote == 0 && executed > 0 {
			quote = executed * numberOf(status, "avgPrice")
		}
		q, px, pnl, closed, e := at.store.StockPaper().BookProtectionStatus(r, executed, quote, numberOf(status, "commission"), s, now)
		if e != nil {
			return out, fmt.Errorf("book protection %s order %s: %w", r.Symbol, r.OrderID, e)
		}
		out.Open = out.Open || !stockOrderTerminal(s)
		// A partial execution booked before a crash still requires retirement
		// of the old set before any remainder can be protected or sold.
		out.Filled = out.Filled || q > 1e-12 || executed > 0
		if q > 1e-12 {
			logger.Infof("🛑 [%s] %s %s order %s filled %.9f @ %.6f (pnl %.2f)", at.name, r.Symbol, r.Role, r.OrderID, q, px, pnl)
			at.stockNotifyTrade(false, "交易所端 "+r.Role+" 成交", r.Symbol, &stockOutcome{Qty: q, Price: px, PnL: pnl, OrderID: r.OrderID, Closed: closed}, "")
			if closed {
				_ = at.store.AIManaged().Unmark(at.id, r.Symbol, "long")
				delete(at.stock.stopTicks, r.Symbol)
				at.syncStockJournal()
			}
		}
	}
	return out, nil
}

// Cancel, then read final cumulative executions. An order can fill between a
// status read and the cancellation; do not discard its IDs at that boundary.
func (at *AutoTrader) stockCancelProtection(sym string, now time.Time) (stockProtectionCheck, error) {
	if err := at.stockTrader.CancelProtection(sym); err != nil {
		return stockProtectionCheck{}, err
	}
	check, err := at.stockCheckProtection(sym, now)
	if err == nil && check.Open {
		err = fmt.Errorf("protection %s still open after cancellation; retry next tick", sym)
	}
	if err == nil {
		err = at.store.StockPaper().RetireProtectionOrders(at.id, sym)
	}
	return check, err
}

type stockProtectionNotice struct {
	LastAlert time.Time
	Reason    string
}

func (at *AutoTrader) stockProtectionAlert(sym, class string, err error) {
	logger.Warnf("⚠️ [%s] protection reconciliation blocked for %s (%s): %v", at.name, sym, class, err)
	if at.stock.protectionAlerts == nil {
		at.stock.protectionAlerts = map[string]map[string]stockProtectionNotice{}
	}
	if at.stock.protectionAlerts[sym] == nil {
		at.stock.protectionAlerts[sym] = map[string]stockProtectionNotice{}
	}
	now := at.stockClock()
	prior, alerted := at.stock.protectionAlerts[sym][class]
	if !alerted || now.Sub(prior.LastAlert) >= time.Hour {
		prior.LastAlert = now
		stockNotify("ALERT", at.name, fmt.Sprintf("美股 %s 保护单对账失败，暂停该标的本轮自动操作，下轮重试：%s", notify.Escape(sym), notify.Escape(err.Error())))
	}
	prior.Reason = err.Error()
	at.stock.protectionAlerts[sym][class] = prior
}

func (at *AutoTrader) stockProtectionRecovered(sym string) {
	alerts := at.stock.protectionAlerts[sym]
	if len(alerts) == 0 {
		return
	}
	var reasons []string
	for _, notice := range alerts {
		if notice.Reason != "" {
			reasons = append(reasons, notice.Reason)
		}
	}
	if len(reasons) == 0 {
		return
	}
	sort.Strings(reasons)
	stockNotify("INFO", at.name, fmt.Sprintf("美股 %s 保护单对账已恢复，恢复自动管理；此前原因：%s", notify.Escape(sym), notify.Escape(strings.Join(reasons, "；"))))
	// Keep the de-dup timestamps across recovery: a flapping error still gets
	// at most one ALERT per (symbol, class) in the 60-minute window.
	for class, notice := range alerts {
		notice.Reason = ""
		alerts[class] = notice
	}
}

// us_stock (design 2026-10-09 §6): live persistence. A program buy creates or
// merges one OPEN ai_managed LONG trader_positions row per pair (the program's
// owned quantity); sells reduce / close it; closed rows feed trade_journal.

// stockRecordBuy books a filled program buy. qty is the NET quantity received
// (commission in the base asset already deducted). Returns the owned quantity
// after the buy.
func (at *AutoTrader) stockRecordBuy(symbol string, qty, price, fee float64, orderID string, stop float64, now time.Time) (float64, error) {
	pos := at.store.Position()
	rows, err := at.store.Position().StockOwnedRows(at.id, symbol)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		nowMs := now.UTC().UnixMilli()
		row := &store.TraderPosition{
			TraderID: at.id, ExchangeID: at.exchangeID, ExchangeType: "binance", Symbol: symbol, Side: "LONG",
			Quantity: qty, EntryQuantity: qty, EntryPrice: price, EntryOrderID: orderID, EntryTime: nowMs,
			Leverage: 1, Status: "OPEN", Source: "system", AIManaged: true, InitialStopLoss: stop, Fee: fee,
			CreatedAt: nowMs, UpdatedAt: nowMs,
		}
		if err := pos.CreateOpenPosition(row); err != nil {
			return 0, err
		}
	} else if err := pos.UpdatePositionQuantityAndPrice(rows[0].ID, qty, price, fee); err != nil {
		return 0, err
	}
	if err := at.store.AIManaged().MarkEntry(at.id, symbol, "long", orderID); err != nil {
		logger.Errorf("🤖 [%s] AI-managed MARK FAILED %s: %v", at.name, symbol, err)
	}
	rows, err = pos.StockOwnedRows(at.id, symbol)
	if err != nil {
		return 0, err
	}
	owned, _, _, _ := sumRows(rows)
	return owned, nil
}

// stockRecordSell books a filled program sell of qty. full closes the row(s);
// otherwise the quantity is reduced. Returns the leg's gross realized PnL
// (vs the program's own entry price).
func (at *AutoTrader) stockRecordSell(symbol string, qty, price, fee float64, orderID, reason string, full bool, now time.Time) float64 {
	rows, err := at.store.Position().StockOwnedRows(at.id, symbol)
	if err != nil || len(rows) == 0 {
		logger.Warnf("⚠️ [%s] sell of %s has no program-owned position row (%v) — nothing recorded", at.name, symbol, err)
		return 0
	}
	nowMs := now.UTC().UnixMilli()
	pnlTotal := 0.0
	remaining := qty
	for i, r := range rows {
		take := remaining
		if take > r.Quantity || (full && i == len(rows)-1) {
			take = r.Quantity
		}
		if take <= 0 {
			continue
		}
		pnl := (price - r.EntryPrice) * take
		pnlTotal += pnl
		share := 0.0
		if qty > 0 {
			share = fee * take / qty
		}
		if full || take >= r.Quantity-1e-9 {
			exit := price
			if closed := r.EntryQuantity - r.Quantity; closed > 0 && r.ExitPrice > 0 {
				exit = (r.ExitPrice*closed + price*take) / (closed + take)
			}
			err = at.store.Position().ClosePositionFully(r.ID, exit, orderID, nowMs, pnl, share, reason)
		} else {
			err = at.store.Position().ReducePositionQuantity(r.ID, take, price, share, pnl)
		}
		if err != nil {
			logger.Errorf("❌ [%s] recording sell of %s failed: %v", at.name, symbol, err)
		}
		remaining -= take
	}
	if full {
		_ = at.store.AIManaged().Unmark(at.id, symbol, "long")
		delete(at.stock.lastProtect, symbol)
		delete(at.stock.stopTicks, symbol)
		at.syncStockJournal()
	}
	return pnlTotal
}

func (at *AutoTrader) syncStockJournal() {
	if created, err := at.store.TradeJournal().SyncFromPositions(at.id); err != nil {
		logger.Infof("⚠ [%s] Failed to sync trade journal: %v", at.name, err)
	} else if created > 0 {
		logger.Infof("📓 [%s] Trade journal synced: %d new entries pending review", at.name, created)
	}
}

// stockFee converts an order's commission to USDT where that is knowable.
func stockFee(res *types.SpotOrderResult, price float64, baseAsset string) (feeUSDT, baseQty float64) {
	if res == nil || res.Commission <= 0 {
		return 0, 0
	}
	switch strings.ToUpper(res.CommissionAsset) {
	case "USDT", "":
		return res.Commission, 0
	case strings.ToUpper(baseAsset):
		return res.Commission * price, res.Commission
	}
	return 0, 0 // BNB etc.: not convertible here
}

func fillPrice(res *types.SpotOrderResult, fallback float64) float64 {
	switch {
	case res.AvgPrice > 0:
		return res.AvgPrice
	case res.ExecutedQty > 0 && res.QuoteQty > 0:
		return res.QuoteQty / res.ExecutedQty
	}
	return fallback
}

// persistedStop returns the stop the program last placed for a pair, falling
// back to the opening stop of the owned rows.
func (at *AutoTrader) persistedStop(sym string, h *stockHolding) (stop, tp float64) {
	if lp, ok := at.stock.lastProtect[sym]; ok && lp.Stop > 0 {
		return lp.Stop, lp.TakeProfit
	}
	if h != nil && h.InitialStop > 0 {
		return h.InitialStop, 0
	}
	return 0, 0
}

// restoreStockPendings reloads resting limit entries after a restart.
func (at *AutoTrader) restoreStockPendings() {
	rows, err := at.store.StockPaper().ListPending(at.id)
	if err != nil {
		logger.Warnf("⚠️ [%s] cannot load resting us_stock entries: %v", at.name, err)
		return
	}
	for _, r := range rows {
		at.stock.pendings[r.Symbol] = &stockPending{Symbol: r.Symbol, OrderID: r.OrderID, Action: r.Action, Qty: r.Quantity,
			FilledQty: r.FilledQty, Limit: r.LimitPrice, Stop: r.Stop, TakeProfit: r.TakeProfit, PlacedAt: time.UnixMilli(r.PlacedAt)}
		logger.Infof("📌 [%s] resting limit entry restored: %s order %s", at.name, r.Symbol, r.OrderID)
	}
}

func (at *AutoTrader) savePending(p *stockPending) {
	err := at.store.StockPaper().UpsertPending(&store.StockPendingEntry{TraderID: at.id, Symbol: p.Symbol, OrderID: p.OrderID,
		Action: p.Action, Quantity: p.Qty, FilledQty: p.FilledQty, LimitPrice: p.Limit, Stop: p.Stop, TakeProfit: p.TakeProfit, PlacedAt: p.PlacedAt.UnixMilli()})
	if err != nil {
		logger.Errorf("❌ [%s] cannot persist resting entry %s: %v", at.name, p.Symbol, err)
	}
}

func (at *AutoTrader) dropPending(symbol string) {
	delete(at.stock.pendings, symbol)
	if err := at.store.StockPaper().DeletePending(at.id, symbol); err != nil {
		logger.Warnf("⚠️ [%s] cannot delete resting entry row %s: %v", at.name, symbol, err)
	}
}

func fmtQty(v float64) string { return fmt.Sprintf("%.6f", v) }
