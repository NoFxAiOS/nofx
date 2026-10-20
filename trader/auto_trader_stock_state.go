package trader

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"nofx/kernel/stockengine"
	"nofx/logger"
	"nofx/store"

	notify "nofx/telegram/notify"
)

// us_stock (design 2026-10-09): account / holdings state of one cycle or tick.

// stockHolding is a PROGRAM-OWNED position (the quantity recorded in the
// program's own ai_managed trader_positions rows, or the paper ledger row).
type stockHolding struct {
	Symbol             string
	Owned              float64 // program-owned quantity (never above the exchange balance)
	Balance            float64 // exchange base-asset balance (live); = Owned in paper
	Avg                float64 // program's own entry average (NOT the account-wide FIFO cost)
	Mark               float64
	Stop               float64 // live protective stop (0 = none)
	TakeProfit         float64
	InitialStop        float64
	OpenedAt           time.Time
	Rows               []*store.TraderPosition // live
	ProtMissing        bool                    // live: protection queried and found nil
	ProtectionVerified bool                    // live: recorded IDs checked successfully, no fills this pass
	Quarantined        bool                    // live: no automatic operations for this pass
	QuarantineReason   string
}

// stockState is the account view the cycle decides on.
type stockState struct {
	Holdings  map[string]*stockHolding // program-owned positions
	Balances  map[string]float64       // exchange balances by pair (live; includes manual holdings)
	Marks     map[string]float64
	Available float64 // free USDT (paper: cash)
	Equity    float64
	Exposure  float64 // value of ALL holdings, manual ones included (risk is account-wide)
	Notes     []string
}

func (s *stockState) sortedHoldings() []*stockHolding {
	out := make([]*stockHolding, 0, len(s.Holdings))
	for _, h := range s.Holdings {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

// manualNotes lists balances above the program-owned quantity. Manual
// holdings are NOT shown to the model (the program never manages them); they
// only count in Exposure/Equity.
func (s *stockState) manualNotes() []string {
	var notes []string
	for sym, bal := range s.Balances {
		owned := 0.0
		if h := s.Holdings[sym]; h != nil {
			owned = h.Owned
		}
		if extra := bal - owned; extra > 1e-9 {
			notes = append(notes, fmt.Sprintf("manual/external holding %s qty=%.6f (not managed by the program)", sym, extra))
		}
	}
	sort.Strings(notes)
	return notes
}

func numberOf(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, err := SafeFloat64(m, k); err == nil {
			return v
		}
	}
	return 0
}

// stockOwnedRows loads the program's OPEN ai_managed LONG rows grouped by pair.
func (at *AutoTrader) stockOwnedRows() (map[string][]*store.TraderPosition, error) {
	rows, err := at.store.Position().GetOpenPositions(at.id)
	if err != nil {
		return nil, err
	}
	out := map[string][]*store.TraderPosition{}
	for _, r := range rows {
		if r.AIManaged && strings.EqualFold(r.Side, "LONG") && strings.HasSuffix(r.Symbol, "BUSDT") && r.Quantity > 0 {
			out[r.Symbol] = append(out[r.Symbol], r)
		}
	}
	return out, nil
}

func sumRows(rows []*store.TraderPosition) (qty, avg float64, initial float64, opened time.Time) {
	cost := 0.0
	for _, r := range rows {
		qty += r.Quantity
		cost += r.Quantity * r.EntryPrice
		if initial == 0 {
			initial = r.InitialStopLoss
		}
		if t := time.UnixMilli(r.EntryTime); opened.IsZero() || t.Before(opened) {
			opened = t
		}
	}
	if qty > 0 {
		avg = cost / qty
	}
	return
}

// stockGatherLive reads the spot account. With reconcile, a balance below the
// owned quantity shrinks the program position (external reduction: no PnL
// guess). withProtection also queries each holding's protective orders.
func (at *AutoTrader) stockGatherLive(ctx context.Context, now time.Time, reconcile, withProtection bool) (*stockState, error) {
	return at.stockGatherLiveWithQuarantine(ctx, now, reconcile, withProtection, nil)
}

// Quarantines from pending-entry reconciliation belong to this same pass.
// A later pass starts with a fresh map and verifies every symbol again.
func (at *AutoTrader) stockGatherLiveWithQuarantine(ctx context.Context, now time.Time, reconcile, withProtection bool, quarantined map[string]string) (*stockState, error) {
	if quarantined == nil {
		quarantined = map[string]string{}
	}
	quarantine := func(sym, class string, err error) {
		quarantined[sym] = err.Error()
		at.stockProtectionAlert(sym, class, err)
	}
	checks := map[string]stockProtectionCheck{}
	intents := map[string]bool{}
	if reconcile {
		// Query IDs before reading balances: manual inventory cannot tell us
		// whether our protection executed. Discover legacy OPEN sets as well.
		owned, e := at.stockOwnedRows()
		if e != nil {
			return nil, e
		}
		recorded, e := at.store.StockPaper().ListProtectionOrders(at.id, "")
		if e != nil {
			at.stockProtectionAlert("账户", "list_protection", e)
			return nil, e
		}
		// A buy fill whose FIRST protection is still to be confirmed is known to be
		// the program's own and was never covered by a protection that could have
		// sold it: it is retried (ProtMissing path) instead of being quarantined
		// as a position of unknown provenance.
		intents, e = at.store.StockPaper().ListProtectionIntents(at.id)
		if e != nil {
			at.stockProtectionAlert("账户", "list_protection", e)
			return nil, e
		}
		symbols := map[string]bool{}
		for _, r := range recorded {
			symbols[r.Symbol] = true
		}
		// Manual reconciliation may remove an unverifiable position entirely.
		// Check that symbol once more so it can receive its recovery notice.
		for sym := range at.stock.protectionAlerts {
			if sym != "账户" {
				symbols[sym] = true
			}
		}
		live := map[string]bool{}
		for sym := range owned {
			symbols[sym] = true
			if quarantined[sym] != "" {
				continue
			}
			p, e := at.stockTrader.GetProtection(sym)
			if e != nil {
				quarantine(sym, "get_protection", e)
				continue
			}
			if p != nil {
				live[sym] = true
				if e := at.stockSaveProtection(p); e != nil {
					quarantine(sym, "save_protection", e)
				}
			}
		}
		for sym := range symbols {
			if quarantined[sym] != "" {
				continue
			}
			check, e := at.stockCheckProtection(sym, now)
			if e == nil && check.Filled {
				_, e = at.stockCancelProtection(sym, now)
				check.Open = false
			}
			if e != nil {
				if stockProtectionReadFailed(e) {
					return nil, e
				}
				quarantine(sym, "check_protection", e)
				continue
			}
			checks[sym] = check
			if len(owned[sym]) > 0 && !live[sym] && !check.Known && !intents[sym] {
				// A legacy protection may already have sold OUR shares. Neither
				// re-protect nor shrink potentially manual inventory by balance.
				quarantine(sym, "missing_records", fmt.Errorf("无在途保护单且无已记录的保护单 ID，无法核实持仓归属；请人工核对历史成交：若为本人持仓，请确认并清理程序持仓记录；若仍为程序持仓，请手动平仓或补挂保护单后确认"))
			} else if !live[sym] && check.Open && len(owned[sym]) > 0 {
				quarantine(sym, "missing_open_orders", fmt.Errorf("已记录的保护单仍为未完成状态，但未出现在当前挂单中，等待下轮重试"))
			}
		}
	}
	bal, err := at.stockTrader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}
	poss, err := at.stockTrader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}
	st := &stockState{Holdings: map[string]*stockHolding{}, Balances: map[string]float64{}, Marks: map[string]float64{}}
	st.Available = numberOf(bal, "availableBalance", "available_balance")
	for _, p := range poss {
		sym, _ := p["symbol"].(string)
		qty := numberOf(p, "positionAmt", "position_amt", "quantity")
		if qty < 0 {
			qty = -qty
		}
		if sym == "" || qty <= 0 {
			continue
		}
		mark := numberOf(p, "markPrice", "mark_price", "price")
		st.Balances[sym] += qty
		if mark > 0 {
			st.Marks[sym] = mark
			at.stock.setPrice(sym, mark)
		}
		st.Exposure += qty * mark
	}
	if eq := firstPositive(bal, "totalEquity", "total_equity"); eq > 0 {
		st.Equity = eq
	} else {
		st.Equity = st.Available + st.Exposure
	}

	owned, err := at.stockOwnedRows()
	if err != nil {
		return nil, fmt.Errorf("load owned positions: %w", err)
	}
	for sym, rows := range owned {
		qty, avg, initial, opened := sumRows(rows)
		balance := st.Balances[sym]
		info := stockSymbolInfo(ctx, sym)
		replaceProtection := checks[sym].Filled
		if reconcile && quarantined[sym] == "" && qty-balance > maxFloat(info.StepSize, 1e-9)/2 && checks[sym].Known {
			// A fill can race the first status read and the balance snapshot.
			// Freeze the old set and book FINAL executions before shrinking.
			check, e := at.stockCancelProtection(sym, now)
			if e != nil {
				if stockProtectionReadFailed(e) {
					return nil, e
				}
				quarantine(sym, "check_protection", e)
			} else {
				previous := checks[sym]
				previous.Filled = previous.Filled || check.Filled
				replaceProtection = replaceProtection || previous.Open || check.Filled
				previous.Open = false
				checks[sym] = previous
			}
			// Cancellation can have booked a partial fill even on error.
			rows, e = at.store.Position().StockOwnedRows(at.id, sym)
			if e != nil {
				return nil, e
			}
			qty, avg, initial, opened = sumRows(rows)
			_, balance, e = at.stockLiveQuantities(sym)
			if e != nil {
				return nil, e
			}
			st.Exposure += (balance - st.Balances[sym]) * st.Marks[sym]
			st.Balances[sym] = balance
		}
		if reconcile && quarantined[sym] == "" && qty-balance > maxFloat(info.StepSize, 1e-9)/2 {
			newQty := at.stockShrinkOwned(sym, rows, qty, balance, st.Marks[sym], now, st)
			if newQty <= 0 {
				continue
			}
			owned2, e := at.stockOwnedRows()
			if e != nil {
				return nil, e
			}
			rows = owned2[sym]
			qty, avg, initial, opened = sumRows(rows)
		}
		if qty > balance {
			qty = balance // never act on more than the balance
		}
		if qty <= 0 && quarantined[sym] == "" {
			continue
		}
		h := &stockHolding{Symbol: sym, Owned: qty, Balance: balance, Avg: avg, Mark: st.Marks[sym],
			InitialStop: initial, OpenedAt: opened, Rows: rows,
			ProtectionVerified: reconcile && (checks[sym].Known || intents[sym]) && !checks[sym].Filled}
		if reconcile && quarantined[sym] == "" && replaceProtection {
			stop, tp := at.persistedStop(sym, h)
			if e := at.stockPlaceProtection(ctx, sym, qty, stop, tp, h.Mark); e != nil {
				if stockProtectionReadFailed(e) {
					return nil, e
				}
				// No fallback or further replacement on an ambiguous fill pass.
				quarantine(sym, "protection_change", fmt.Errorf("protect remaining %s: %w", sym, e))
			}
		}
		if withProtection && quarantined[sym] == "" {
			prot, perr := at.stockTrader.GetProtection(sym)
			switch {
			case perr != nil:
				if reconcile {
					quarantine(sym, "get_protection", perr)
				} else {
					quarantined[sym] = perr.Error()
				}
			case prot == nil:
				if reconcile && checks[sym].Open {
					quarantine(sym, "missing_open_orders", fmt.Errorf("已记录的保护单仍为未完成状态，但未出现在当前挂单中，等待下轮重试"))
				} else {
					h.ProtMissing = true
				}
			default:
				h.Stop, h.TakeProfit = prot.StopPrice, prot.TakeProfit
				if reconcile { // API readers must not write run state
					at.stock.lastProtect[sym] = stockProtect{Stop: prot.StopPrice, TakeProfit: prot.TakeProfit, Qty: prot.Quantity}
				}
			}
		}
		h.QuarantineReason = quarantined[sym]
		h.Quarantined = h.QuarantineReason != ""
		if h.Quarantined {
			h.ProtectionVerified = false
			if reconcile {
				at.stock.stopTicks[sym] = 0
			}
		}
		st.Holdings[sym] = h
	}
	if reconcile && withProtection {
		for sym := range checks {
			if quarantined[sym] == "" {
				at.stockProtectionRecovered(sym)
			}
		}
	}
	return st, nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// stockShrinkOwned reduces the program-owned rows of sym from owned to the
// exchange balance after an unexplained external reduction. No
// exit is booked and no PnL attributed. Returns the new owned quantity.
func (at *AutoTrader) stockShrinkOwned(sym string, rows []*store.TraderPosition, owned, balance, mark float64, now time.Time, st *stockState) float64 {
	info := stockSymbolInfo(context.Background(), sym)
	target := floorStep(balance, info.StepSize)
	if target < 0 {
		target = 0
	}
	diff := owned - target

	for i := len(rows) - 1; i >= 0 && diff > 1e-12; i-- {
		take := diff
		if rows[i].Quantity < take {
			take = rows[i].Quantity
		}
		if err := at.store.Position().ApplyExternalReduction(rows[i].ID, rows[i].Quantity-take, mark, now); err != nil {
			logger.Errorf("❌ [%s] external-reduction update failed for %s: %v", at.name, sym, err)
			return owned
		}
		diff -= take
	}
	if target <= 0 {
		_ = at.store.AIManaged().Unmark(at.id, sym, "long")
		delete(at.stock.lastProtect, sym)
	}
	pfx := stockPrefix(at.stockPaper())
	note := fmt.Sprintf("%sunexplained external reduction on %s: program-owned %.6f -> exchange balance %.6f; position shrunk, no PnL attributed", pfx, sym, owned, target)
	logger.Warnf("⚠️ [%s] %s", at.name, note)
	if st != nil {
		st.Notes = append(st.Notes, note)
	}
	stockNotify("ALERT", at.name, fmt.Sprintf("<b>⚠️ 美股持仓被外部减少 %s</b>\n<i>程序持仓 %.6f → 交易所余额 %.6f，已按余额收缩（外部减仓，不计盈亏）</i>",
		notify.Escape(sym), owned, target))
	return target
}

// stockGatherPaper reads the paper ledger. prices (latest known) value the
// holdings; a pair without a price is valued at its average cost.
func (at *AutoTrader) stockGatherPaper(prices map[string]float64) (*stockState, error) {
	acct, err := at.store.StockPaper().EnsureAccount(at.id, at.initialBalance)
	if err != nil {
		return nil, err
	}
	open, err := at.store.StockPaper().ListOpen(at.id)
	if err != nil {
		return nil, err
	}
	st := &stockState{Holdings: map[string]*stockHolding{}, Balances: map[string]float64{}, Marks: map[string]float64{}, Available: acct.Cash}
	for _, p := range open {
		mark := prices[p.Symbol]
		if mark <= 0 {
			mark = at.stock.price(p.Symbol)
		}
		if mark <= 0 {
			mark = p.AvgPrice
		}
		st.Holdings[p.Symbol] = &stockHolding{Symbol: p.Symbol, Owned: p.Quantity, Balance: p.Quantity, Avg: p.AvgPrice, Mark: mark,
			Stop: p.Stop, TakeProfit: p.TakeProfit, InitialStop: p.InitialStop, OpenedAt: time.UnixMilli(p.OpenedAt)}
		st.Balances[p.Symbol] = p.Quantity
		st.Marks[p.Symbol] = mark
		st.Exposure += p.Quantity * mark
	}
	st.Equity = st.Available + st.Exposure
	return st, nil
}

// reprice revalues a paper state with fresh prices (live equity comes from
// the exchange and is left alone).
func (s *stockState) repricePaper(prices map[string]float64) {
	exposure := 0.0
	for sym, h := range s.Holdings {
		if p := prices[sym]; p > 0 {
			h.Mark = p
			s.Marks[sym] = p
		}
		exposure += h.Owned * h.Mark
	}
	s.Exposure = exposure
	s.Equity = s.Available + exposure
}

func (s *stockState) enginePositions(prices map[string]float64) []stockengine.Position {
	var out []stockengine.Position
	for _, h := range s.sortedHoldings() {
		price := h.Mark
		if p := prices[h.Symbol]; p > 0 {
			price = p
		}
		out = append(out, stockengine.Position{Symbol: h.Symbol, Quantity: h.Owned, AvgPrice: h.Avg, Price: price,
			StopPrice: h.Stop, TakeProfit: h.TakeProfit, InitialStop: h.InitialStop, OpenedAt: h.OpenedAt})
	}
	return out
}

func stockPrefix(paper bool) string {
	if paper {
		return "[PAPER] "
	}
	return ""
}
