// Package paper implements a durable, USDT-linear futures simulator in Binance
// hedge mode (independent LONG/SHORT positions).
//
// Matching rules:
//   - Market buys/sells execute at quote +/- adverse slippage; taker fees apply.
//   - Resting limits fill at their limit when a later 1m low/high touches it;
//     maker fees apply. There are no partial fills, queue or volume simulation.
//   - SL closes the whole hedge side at the executable price +/- adverse
//     slippage (taker). The executable price is the trigger, unless the market
//     gapped through it: then the current quote (or, in replay, the candle open
//     or its nearest bound when the whole candle lies beyond the trigger).
//     TP closes the specified quantity, or the whole side for quantity <= 0,
//     as a market order at trigger +/- adverse slippage (taker), never better
//     than the trigger. Close orders can never create opposite exposure.
//     Within the same candle SL precedes TP, then limits, with FIFO tie breaks.
//   - Completed candles replay chronologically, followed by the current quote.
//     The placement minute's pre-order extremes are excluded; only subsequent
//     full candles and sampled quotes are usable (OHLC has no intrabar path).
//   - Available = wallet - mark notional/leverage; entry fees must also fit.
//     Resting entries reserve no margin and are rejected at fill if unfunded.
//     Cross/isolated settings are recorded, but both use account-level margin.
//     Equity <= maintenance notional closes ALL positions at their marks,
//     charging taker fees, cancelling orders, and tagging liquidation receipts.
//     Replay also tests each candle's intrabar extremes (after SL/TP/limits):
//     every symbol of that minute is valued at its Low, then at its High, and a
//     breach liquidates at that extreme. Positions opened inside the minute
//     keep the close mark. Hedges therefore see one correlated move.
//   - Funding, tiered margin, liquidation penalties, insurance/ADL, latency,
//     post-only rejection and venue price filters are not simulated.
//
// State has one mutex. Quotes, candles and symbol-rule requests run outside it;
// Advance discards stale observations using an optimistic version check. Each
// successful mutation commits its ledger and accounting rows atomically. A
// second instance on the same account must retry a version-conflicted command.
package paper

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/store"
	"nofx/trader/types"
)

type Config struct {
	ExchangeID                            string
	InitialBalance                        float64
	TakerFeeBps, MakerFeeBps, SlippageBps float64
	DefaultLeverage                       int
	MaintenanceMarginPct                  float64
	// Optional deterministic dependencies; a PriceSource may itself implement
	// SymbolRules. Explicit rules take precedence over the public SDK lookup.
	SymbolRules SymbolRules
	Now         func() time.Time
}

type position struct {
	Symbol, Side         string
	Quantity, EntryPrice float64
	Leverage             int
	OpenedAt             time.Time
}

type order struct {
	types.OpenOrder
	Number                            int64
	Leverage                          int
	CreatedAt, UpdatedAt              time.Time
	AvgPrice, ExecutedQty, Commission float64
}

type ledgerState struct {
	Wallet, Realized, Fees float64
	TradeCount             int
	Sequence               int64
	LastAdvance            time.Time
	Positions              map[string]*position
	Orders                 map[string]*order // includes terminal orders for restart-safe status
	Marks                  map[string]float64
	Leverage               map[string]int
	CrossMargin            map[string]bool
	Closed                 []types.ClosedPnLRecord
}

type PaperFuturesTrader struct {
	mu          sync.Mutex
	cfg         Config
	st          *store.Store
	price       PriceSource
	rules       SymbolRules
	row         *store.PaperLedger
	s           ledgerState
	initErr     error
	displayName string
	stop, done  chan struct{}
	// lotSteps remembers each symbol's LOT_SIZE step (set by quantity()) so a
	// float64 remainder below half a step is recognised as a flat position.
	lotSteps map[string]float64
}

var _ types.GridTrader = (*PaperFuturesTrader)(nil)

// New restores the account, creating a 10,000 USDT ledger on first use when
// InitialBalance is zero. Defaults: taker 5bp, maker 2bp, slippage 3bp,
// leverage 1, maintenance 0.5%. InitialBalance never resets an existing ledger.
// A fully resolved configuration (InitialBalance > 0 and DefaultLeverage >= 1,
// as produced from store.PaperConfig) keeps explicit zero fees/slippage; only
// partially filled configurations get the default costs for zero fields.
// The required signature has no error result: initialization errors are kept
// and returned by operations; an unavailable DB never becomes a memory ledger.
func New(cfg Config, st *store.Store, price PriceSource) *PaperFuturesTrader {
	if cfg.ExchangeID == "" {
		cfg.ExchangeID = "paper"
	}
	resolved := cfg.InitialBalance > 0 && cfg.DefaultLeverage >= 1
	if cfg.InitialBalance == 0 {
		cfg.InitialBalance = 10000
	}
	if !resolved {
		if cfg.TakerFeeBps == 0 {
			cfg.TakerFeeBps = 5
		}
		if cfg.MakerFeeBps == 0 {
			cfg.MakerFeeBps = 2
		}
		if cfg.SlippageBps == 0 {
			cfg.SlippageBps = 3
		}
	}
	if cfg.DefaultLeverage == 0 {
		cfg.DefaultLeverage = 1
	}
	if cfg.MaintenanceMarginPct == 0 {
		cfg.MaintenanceMarginPct = 0.5
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if price == nil {
		price = NewMarketPriceSource()
	}
	p := &PaperFuturesTrader{cfg: cfg, st: st, price: price, rules: cfg.SymbolRules, lotSteps: map[string]float64{}}
	if p.rules == nil {
		p.rules, _ = price.(SymbolRules)
	}
	if p.rules == nil {
		p.rules = NewMarketPriceSource().(SymbolRules)
	}
	for _, v := range []float64{cfg.InitialBalance, cfg.MaintenanceMarginPct} {
		if !positive(v) {
			p.initErr = fmt.Errorf("paper configuration must be finite and positive")
			return p
		}
	}
	for _, v := range []float64{cfg.TakerFeeBps, cfg.MakerFeeBps, cfg.SlippageBps} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			p.initErr = fmt.Errorf("paper fees and slippage must be finite and non-negative")
			return p
		}
	}
	if cfg.DefaultLeverage < 1 || cfg.SlippageBps >= 10000 || cfg.MaintenanceMarginPct >= 100 {
		p.initErr = fmt.Errorf("invalid paper leverage/slippage/maintenance configuration")
		return p
	}
	if st == nil {
		p.initErr = fmt.Errorf("paper trader requires a persistent store")
		return p
	}
	p.s = ledgerState{Wallet: cfg.InitialBalance, LastAdvance: cfg.Now().UTC(), Positions: map[string]*position{}, Orders: map[string]*order{}, Marks: map[string]float64{}, Leverage: map[string]int{}, CrossMargin: map[string]bool{}}
	b, err := json.Marshal(p.s)
	if err != nil {
		p.initErr = err
		return p
	}
	p.row, err = st.PaperLedger().Ensure(&store.PaperLedger{ExchangeID: cfg.ExchangeID, ExchangeType: "paper", InitialBalance: cfg.InitialBalance, WalletBalance: cfg.InitialBalance, LastAdvanceAt: p.s.LastAdvance.UnixMilli(), Version: 1, State: string(b)})
	if err == nil {
		err = json.Unmarshal([]byte(p.row.State), &p.s)
	}
	p.initErr = err
	return p
}

func positive(v float64) bool           { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
func symbolName(s string) string        { return strings.ToUpper(strings.TrimSpace(s)) }
func posKey(symbol, side string) string { return symbol + ":" + side }

// mutateLocked rolls memory back as well as DB writes on any failure.
func (p *PaperFuturesTrader) mutateLocked(fn func() ([]store.PaperLedgerTrade, error)) error {
	if p.initErr != nil {
		return p.initErr
	}
	before, err := json.Marshal(p.s)
	if err != nil {
		return err
	}
	var snapshot ledgerState
	if err := json.Unmarshal(before, &snapshot); err != nil {
		return err
	}
	previous := *p.row
	trades, err := fn()
	if err == nil {
		var b []byte
		b, err = json.Marshal(p.s)
		if err == nil {
			p.row.State = string(b)
			p.row.WalletBalance = p.s.Wallet
			p.row.RealizedPnL = p.s.Realized
			p.row.LastAdvanceAt = p.s.LastAdvance.UnixMilli()
			p.row.Version = previous.Version + 1
			changes := map[string]int{}
			for symbol, leverage := range p.s.Leverage {
				if snapshot.Leverage[symbol] != leverage {
					changes[symbol] = leverage
				}
			}
			err = p.st.PaperLedger().Commit(p.row, &previous, trades, changes)
		}
	}
	if err != nil {
		p.s = snapshot
		*p.row = previous
		if err == store.ErrPaperLedgerConflict {
			if latest, e := p.st.PaperLedger().Get(p.cfg.ExchangeID); e == nil {
				p.s = ledgerState{}
				if e = json.Unmarshal([]byte(latest.State), &p.s); e == nil {
					p.row = latest
				} else {
					p.initErr = e
				}
			}
		}
	}
	return err
}

func (p *PaperFuturesTrader) leverage(symbol string) int {
	if v := p.s.Leverage[symbol]; v > 0 {
		return v
	}
	return p.cfg.DefaultLeverage
}

func (p *PaperFuturesTrader) account() (unrealized, margin, notional float64) {
	return p.accountAt(nil)
}

// accountAt values positions at p.s.Marks, except symbols present in override.
func (p *PaperFuturesTrader) accountAt(override map[string]float64) (unrealized, margin, notional float64) {
	for _, v := range p.s.Positions {
		mark := p.s.Marks[v.Symbol]
		if m, ok := override[v.Symbol]; ok {
			mark = m
		}
		direction := 1.0
		if v.Side == "SHORT" {
			direction = -1
		}
		unrealized += direction * (mark - v.EntryPrice) * v.Quantity
		n := mark * v.Quantity
		notional += n
		margin += n / float64(v.Leverage)
	}
	return
}

func (p *PaperFuturesTrader) newOrder(symbol, side, ps, kind string, quantity, price float64, now time.Time) *order {
	p.s.Sequence++
	id := strconv.FormatInt(p.s.Sequence, 10)
	o := &order{OpenOrder: types.OpenOrder{OrderID: id, ClientID: "x-KzrpZaP9paper-" + id, Symbol: symbol, Side: side, PositionSide: ps, Type: kind, Price: price, Quantity: quantity, Status: "NEW"}, Number: p.s.Sequence, Leverage: p.leverage(symbol), CreatedAt: now, UpdatedAt: now}
	p.s.Orders[id] = o
	return o
}

func opening(o *order) bool {
	return o.PositionSide == "LONG" && o.Side == "BUY" || o.PositionSide == "SHORT" && o.Side == "SELL"
}

func (p *PaperFuturesTrader) execute(o *order, price float64, maker bool, reason string, now time.Time) (store.PaperLedgerTrade, error) {
	key := posKey(o.Symbol, o.PositionSide)
	pos := p.s.Positions[key]
	qty := o.Quantity
	isOpen := opening(o) && !o.ReduceOnly && !o.ClosePosition
	if !isOpen {
		if pos == nil {
			return store.PaperLedgerTrade{}, fmt.Errorf("-2022 ReduceOnly Order is rejected: no %s position for %s", o.PositionSide, o.Symbol)
		}
		if o.ClosePosition || qty > pos.Quantity {
			qty = pos.Quantity
		}
	}
	if !positive(qty) || !positive(price) {
		return store.PaperLedgerTrade{}, fmt.Errorf("invalid execution quantity or price")
	}
	feeBps := p.cfg.TakerFeeBps
	if maker {
		feeBps = p.cfg.MakerFeeBps
	}
	fee := price * qty * feeBps / 10000
	pnl := 0.0
	action := "open_" + strings.ToLower(o.PositionSide)
	if isOpen {
		_, margin, _ := p.account()
		// Leverage is a per-symbol setting in Binance hedge mode.
		lev := p.leverage(o.Symbol)
		o.Leverage = lev
		if p.s.Wallet-margin+1e-9 < price*qty/float64(lev)+fee {
			return store.PaperLedgerTrade{}, fmt.Errorf("-2019 Margin is insufficient")
		}
		if pos == nil {
			pos = &position{Symbol: o.Symbol, Side: o.PositionSide, Leverage: lev, OpenedAt: now}
			p.s.Positions[key] = pos
		}
		pos.EntryPrice = (pos.EntryPrice*pos.Quantity + price*qty) / (pos.Quantity + qty)
		pos.Quantity += qty
	} else {
		action = "close_" + strings.ToLower(o.PositionSide)
		o.Leverage = pos.Leverage
		pnl = (price - pos.EntryPrice) * qty
		if pos.Side == "SHORT" {
			pnl = -pnl
		}
		p.s.Closed = append(p.s.Closed, types.ClosedPnLRecord{Symbol: o.Symbol, Side: strings.ToLower(pos.Side), EntryPrice: pos.EntryPrice, ExitPrice: price, Quantity: qty, RealizedPnL: pnl, Fee: fee, Leverage: pos.Leverage, EntryTime: pos.OpenedAt, ExitTime: now, OrderID: o.OrderID, CloseType: reason, ExchangeID: o.OrderID})
		pos.Quantity = p.normalizeQuantity(o.Symbol, pos.Quantity-qty, qty)
		if pos.Quantity <= 0 {
			delete(p.s.Positions, key)
			p.cancelMatching(func(other *order) bool {
				return other.Symbol == o.Symbol && other.PositionSide == o.PositionSide && other.Type != "LIMIT" && other.OrderID != o.OrderID
			}, now)
		}
	}
	p.s.Wallet += pnl - fee
	p.s.Realized += pnl
	p.s.Fees += fee
	p.s.TradeCount++
	o.Status = "FILLED"
	o.AvgPrice = price
	o.ExecutedQty = qty
	o.Commission = fee
	o.UpdatedAt = now
	owner := p.row.TraderID
	if owner == "" {
		owner = "paper:" + p.cfg.ExchangeID
	}
	ms := now.UTC().UnixMilli()
	or := &store.TraderOrder{TraderID: owner, ExchangeID: p.cfg.ExchangeID, ExchangeType: p.row.ExchangeType, ExchangeOrderID: o.OrderID, ClientOrderID: o.ClientID, Symbol: o.Symbol, Side: o.Side, PositionSide: o.PositionSide, Type: o.Type, OrderAction: action, Quantity: qty, Price: price, StopPrice: o.StopPrice, Status: "FILLED", FilledQuantity: qty, AvgFillPrice: price, Commission: fee, CommissionAsset: "USDT", Leverage: o.Leverage, ReduceOnly: o.ReduceOnly, ClosePosition: o.ClosePosition, FilledAt: ms, CreatedAt: ms, UpdatedAt: ms}
	fill := &store.TraderFill{TraderID: owner, ExchangeID: p.cfg.ExchangeID, ExchangeType: p.row.ExchangeType, ExchangeOrderID: o.OrderID, ExchangeTradeID: o.OrderID, Symbol: o.Symbol, Side: o.Side, Price: price, Quantity: qty, QuoteQuantity: price * qty, Commission: fee, CommissionAsset: "USDT", RealizedPnL: pnl, IsMaker: maker, CreatedAt: ms}
	remaining := 0.0
	if current := p.s.Positions[key]; current != nil {
		remaining = current.Quantity
	}
	return store.PaperLedgerTrade{Order: or, Fill: fill, CloseReason: reason, RemainingQuantity: remaining}, nil
}

// normalizeQuantity removes float64 residue left by subtracting step-multiple
// quantities (0.8-0.1-0.7 = 1.1e-16). With a known lot step the remainder snaps
// to a step multiple; otherwise a relative 1e-9 of the closed size counts as 0.
func (p *PaperFuturesTrader) normalizeQuantity(symbol string, remaining, closed float64) float64 {
	if step := p.lotSteps[symbol]; step > 0 {
		return math.Round(remaining/step) * step
	}
	if remaining <= closed*1e-9+1e-12 {
		return 0
	}
	return remaining
}

func (p *PaperFuturesTrader) cancelMatching(match func(*order) bool, now time.Time) {
	for _, o := range p.s.Orders {
		if o.Status == "NEW" && match(o) {
			o.Status = "CANCELED"
			o.UpdatedAt = now
		}
	}
}

func (p *PaperFuturesTrader) slipped(price float64, side string) float64 {
	if side == "BUY" {
		return price * (1 + p.cfg.SlippageBps/10000)
	}
	return price * (1 - p.cfg.SlippageBps/10000)
}

// Advance obtains observations without holding mu and rechecks the version
// before using them. Errors preserve the cursor so missed candles can retry.
func (p *PaperFuturesTrader) Advance(now time.Time) error {
	now = now.UTC()
	for {
		p.mu.Lock()
		if p.initErr != nil {
			err := p.initErr
			p.mu.Unlock()
			return err
		}
		if !now.After(p.s.LastAdvance) {
			p.mu.Unlock()
			return nil
		}
		version, since := p.row.Version, p.s.LastAdvance
		symbols := map[string]bool{}
		for _, v := range p.s.Positions {
			symbols[v.Symbol] = true
		}
		for _, o := range p.s.Orders {
			if o.Status == "NEW" {
				symbols[o.Symbol] = true
			}
		}
		p.mu.Unlock()
		type observation struct {
			symbol  string
			mark    float64
			candles []Candle
		}
		list := make([]observation, 0, len(symbols))
		for symbol := range symbols {
			mark, err := p.price.Price(symbol)
			if err != nil {
				return err
			}
			if !positive(mark) {
				return fmt.Errorf("invalid mark price for %s", symbol)
			}
			bars, err := p.price.Candles1m(symbol, since)
			if err != nil {
				return err
			}
			for _, c := range bars {
				if !positive(c.High) || !positive(c.Low) || !positive(c.Close) || c.Low > c.High || c.Close < c.Low || c.Close > c.High || c.Time.IsZero() {
					return fmt.Errorf("invalid candle for %s", symbol)
				}
			}
			list = append(list, observation{symbol, mark, bars})
		}
		p.mu.Lock()
		if version != p.row.Version {
			p.mu.Unlock()
			continue
		}
		err := p.mutateLocked(func() ([]store.PaperLedgerTrade, error) {
			type barEvent struct {
				symbol string
				c      Candle
			}
			events := []barEvent{}
			for _, v := range list {
				for _, c := range v.candles {
					end := c.Time.Add(time.Minute)
					if end.After(since) && !end.After(now) {
						events = append(events, barEvent{v.symbol, c})
					}
				}
			}
			sort.Slice(events, func(i, j int) bool {
				if events[i].c.Time.Equal(events[j].c.Time) {
					return events[i].symbol < events[j].symbol
				}
				return events[i].c.Time.Before(events[j].c.Time)
			})
			trades := []store.PaperLedgerTrade{}
			for i := 0; i < len(events); {
				end := i + 1
				for end < len(events) && events[end].c.Time.Equal(events[i].c.Time) {
					end++
				}
				// Account equity must use the whole timestamp's marks. A
				// losing leg must not liquidate before its hedge is repriced.
				for _, e := range events[i:end] {
					p.s.Marks[e.symbol] = e.c.Close
				}
				for _, e := range events[i:end] {
					trades = append(trades, p.match(e.symbol, e.c.Open, e.c.Low, e.c.High, e.c.Time, e.c.Time.Add(time.Minute), false)...)
				}
				group, groupSymbols := make([]Candle, 0, end-i), make([]string, 0, end-i)
				for _, e := range events[i:end] {
					group, groupSymbols = append(group, e.c), append(groupSymbols, e.symbol)
				}
				trades = append(trades, p.liquidateIntrabar(groupSymbols, group)...)
				trades = append(trades, p.liquidate(events[i].c.Time.Add(time.Minute))...)
				i = end
			}
			// Publish every current mark before testing account liquidation.
			for _, v := range list {
				p.s.Marks[v.symbol] = v.mark
			}
			sort.Slice(list, func(i, j int) bool { return list[i].symbol < list[j].symbol })
			for _, v := range list {
				trades = append(trades, p.match(v.symbol, 0, v.mark, v.mark, now, now, true)...)
			}
			trades = append(trades, p.liquidate(now)...)
			p.s.LastAdvance = now
			return trades, nil
		})
		p.mu.Unlock()
		return err
	}
}

// match fills triggered orders. open is the candle open (0 when unknown or for
// quotes); low/high bound the candle, or both equal the quote.
func (p *PaperFuturesTrader) match(symbol string, open, low, high float64, barStart, at time.Time, quote bool) []store.PaperLedgerTrade {
	orders := []*order{}
	for _, o := range p.s.Orders {
		if o.Symbol == symbol && o.Status == "NEW" && !o.CreatedAt.After(barStart) {
			orders = append(orders, o)
		}
	}
	priority := func(o *order) int {
		if o.Type == "STOP_MARKET" {
			return 0
		}
		if o.Type == "TAKE_PROFIT_MARKET" {
			return 1
		}
		return 2
	}
	sort.Slice(orders, func(i, j int) bool {
		if priority(orders[i]) == priority(orders[j]) {
			return orders[i].Number < orders[j].Number
		}
		return priority(orders[i]) < priority(orders[j])
	})
	trades := []store.PaperLedgerTrade{}
	for _, o := range orders {
		if o.Status != "NEW" {
			continue
		}
		price, maker, reason, hit := o.Price, true, "", false
		switch o.Type {
		case "LIMIT":
			// Quotes are not evidence of a later candle touching a limit.
			if !quote {
				hit = o.Side == "BUY" && low <= o.Price || o.Side == "SELL" && high >= o.Price
			}
			if !opening(o) || o.ReduceOnly {
				reason = "manual"
			}
		case "STOP_MARKET":
			hit = o.PositionSide == "LONG" && low <= o.StopPrice || o.PositionSide == "SHORT" && high >= o.StopPrice
			price, maker, reason = p.slipped(stopReference(o, open, low, high), o.Side), false, "stop_loss"
		case "TAKE_PROFIT_MARKET":
			// A market order once triggered: taker fee and adverse slippage,
			// never a price better than the trigger.
			hit = o.PositionSide == "LONG" && high >= o.StopPrice || o.PositionSide == "SHORT" && low <= o.StopPrice
			price, maker, reason = p.slipped(o.StopPrice, o.Side), false, "take_profit"
		}
		if !hit {
			continue
		}
		trade, err := p.execute(o, price, maker, reason, at)
		if err != nil {
			o.Status = "REJECTED"
			o.UpdatedAt = at
			continue
		}
		trades = append(trades, trade)
	}
	return trades
}

// stopReference is the price a triggered stop-market order can actually trade
// at before slippage. It is the trigger unless the market gapped through it:
// a quote already beyond the trigger, a candle opening beyond it, or (without an
// open) a candle lying entirely beyond it, in which case the bound nearest the
// trigger is used. Sell stops (LONG) never fill above, buy stops (SHORT) never
// below, the trigger.
func stopReference(o *order, open, low, high float64) float64 {
	stop := o.StopPrice
	if open < low || open > high {
		open = 0 // unusable open
	}
	if o.PositionSide == "LONG" {
		switch {
		case low == high && low < stop: // quote
			return low
		case open > 0 && open < stop:
			return open
		case open <= 0 && high < stop:
			return high
		}
		return stop
	}
	switch {
	case low == high && low > stop:
		return low
	case open > 0 && open > stop:
		return open
	case open <= 0 && low > stop:
		return low
	}
	return stop
}

// liquidateIntrabar tests whether a candle's extremes breached maintenance
// margin even though its close recovered. All symbols of the minute are valued
// together at their Lows, then at their Highs (one correlated move, so hedges
// are not liquidated by a leg repriced alone). Symbols holding a position
// opened after the candle started stay at their close: that candle's earlier
// range predates the position. On a breach the account liquidates at the
// extreme marks, then marks return to the closes.
func (p *PaperFuturesTrader) liquidateIntrabar(symbols []string, candles []Candle) []store.PaperLedgerTrade {
	if len(p.s.Positions) == 0 {
		return nil
	}
	skip := map[string]bool{}
	for _, pos := range p.s.Positions {
		for i, s := range symbols {
			if s == pos.Symbol && pos.OpenedAt.After(candles[i].Time) {
				skip[s] = true
			}
		}
	}
	var worst map[string]float64
	worstExcess := 0.0
	for _, useLow := range []bool{true, false} {
		marks := map[string]float64{}
		for i, s := range symbols {
			if skip[s] {
				continue
			}
			if useLow {
				marks[s] = candles[i].Low
			} else {
				marks[s] = candles[i].High
			}
		}
		u, _, n := p.accountAt(marks)
		if excess := p.s.Wallet + u - n*p.cfg.MaintenanceMarginPct/100; excess <= 0 && (worst == nil || excess < worstExcess) {
			worst, worstExcess = marks, excess
		}
	}
	if worst == nil {
		return nil
	}
	closes := map[string]float64{}
	for s, m := range worst {
		closes[s] = p.s.Marks[s]
		p.s.Marks[s] = m
	}
	at := candles[0].Time.Add(time.Minute)
	trades := p.liquidate(at)
	for s, m := range closes {
		p.s.Marks[s] = m
	}
	return trades
}

func (p *PaperFuturesTrader) liquidate(now time.Time) []store.PaperLedgerTrade {
	u, _, n := p.account()
	if len(p.s.Positions) == 0 || p.s.Wallet+u > n*p.cfg.MaintenanceMarginPct/100 {
		return nil
	}
	keys := make([]string, 0, len(p.s.Positions))
	for key := range p.s.Positions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	trades := []store.PaperLedgerTrade{}
	for _, key := range keys {
		pos := p.s.Positions[key]
		side := "SELL"
		if pos.Side == "SHORT" {
			side = "BUY"
		}
		o := p.newOrder(pos.Symbol, side, pos.Side, "MARKET", pos.Quantity, 0, now)
		o.ReduceOnly = true
		trade, err := p.execute(o, p.s.Marks[pos.Symbol], false, "liquidation", now)
		if err == nil {
			trades = append(trades, trade)
		}
	}
	p.cancelMatching(func(*order) bool { return true }, now)
	return trades
}
