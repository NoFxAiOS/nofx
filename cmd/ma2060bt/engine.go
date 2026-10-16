package main

import (
	"fmt"
	"math"
	"sort"
	"time"

	"nofx/market/usstock"
)

const initialEquity = 100000.0

var strategyNames = []string{"MOM_TOP", "MA_CROSS", "MA2060_RULES", "MOM_X_MA", "SPY_BH", "EW_BH"}

type Config struct {
	Timeframe string  `json:"timeframe"`
	MaxPos    int     `json:"max_positions"`
	Rebalance int     `json:"rebalance_trading_days"`
	CostBPS   float64 `json:"cost_bps_per_side"`
}

func (c Config) validate() error {
	if c.Timeframe != usstock.TF1d && c.Timeframe != usstock.TF1h {
		return fmt.Errorf("tf must be 1d or 1h")
	}
	if c.MaxPos <= 0 || c.Rebalance <= 0 || !finite(c.CostBPS) || c.CostBPS < 0 || c.CostBPS >= 10000 {
		return fmt.Errorf("maxpos and rebalance must be positive; cost-bps must be finite in [0,10000)")
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func date(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type history struct {
	bars  []usstock.Bar
	keys  []time.Time
	index map[time.Time]int
	ind   []indicator
}

// Normalize on UTC dates for daily data, exact UTC timestamps for hourly data.
// Copy inputs, discard invalid OHLCV, and retain the latest valid duplicate.
func makeHistory(input []usstock.Bar, tf string) history {
	bars := append([]usstock.Bar(nil), input...)
	sort.SliceStable(bars, func(i, j int) bool { return bars[i].OpenTime.Before(bars[j].OpenTime) })
	h := history{index: make(map[time.Time]int)}
	for _, b := range bars {
		if b.OpenTime.IsZero() || !finite(b.Open) || !finite(b.High) || !finite(b.Low) || !finite(b.Close) || !finite(b.Volume) || b.Open <= 0 || b.Low <= 0 || b.Close <= 0 || b.High < math.Max(b.Open, b.Close) || b.Low > math.Min(b.Open, b.Close) || b.Volume < 0 {
			continue
		}
		key := b.OpenTime.UTC()
		if tf == usstock.TF1d {
			key = date(key)
		}
		if j, ok := h.index[key]; ok {
			h.bars[j] = b
			continue
		}
		h.index[key] = len(h.bars)
		h.keys = append(h.keys, key)
		h.bars = append(h.bars, b)
	}
	h.ind = indicators(h.bars)
	return h
}

type Rank struct {
	Symbol   string
	Valid    bool
	Trend    bool
	Momentum float64
}

// Hourly ranking deliberately uses only daily dates strictly before today.
// Daily candles carry their OPEN timestamp, so a <= timestamp join would leak
// today's final daily close into morning hourly decisions.
func rankAt(symbol string, daily history, key time.Time, tf string) Rank {
	d := date(key)
	end := sort.Search(len(daily.keys), func(i int) bool {
		if tf == usstock.TF1h {
			return !daily.keys[i].Before(d)
		}
		return daily.keys[i].After(d)
	})
	i := end - 1
	r := Rank{Symbol: symbol}
	if i >= MomentumLookback {
		r.Valid = true
		r.Trend = daily.bars[i].Close > daily.ind[i].Trend
		r.Momentum = daily.bars[i-MomentumSkip].Close/daily.bars[i-MomentumLookback].Close - 1
	}
	return r
}

func rankLess(a, b Rank) bool {
	if a.Valid != b.Valid {
		return a.Valid
	}
	if a.Valid {
		if a.Trend != b.Trend {
			return a.Trend
		}
		if a.Momentum != b.Momentum {
			return a.Momentum > b.Momentum
		}
	}
	return a.Symbol < b.Symbol
}

type Order struct {
	Symbol string  `json:"symbol"`
	Entry  string  `json:"entry,omitempty"`
	Exit   string  `json:"exit,omitempty"`
	ATR    float64 `json:"entry_atr,omitempty"`
}

type Target struct {
	Symbol string  `json:"symbol"`
	Weight float64 `json:"weight"`
}

type Decision struct {
	Time      time.Time `json:"signal_time"`
	Rebalance bool      `json:"rebalance"`
	Targets   []Target  `json:"targets,omitempty"`
	Orders    []Order   `json:"orders,omitempty"`
}

type EquityPoint struct {
	Time      time.Time `json:"time"`
	Equity    float64   `json:"equity"`
	Cash      float64   `json:"cash"`
	Exposure  float64   `json:"invested_weight"`
	Positions int       `json:"positions"`
}

type Trade struct {
	Symbol     string    `json:"symbol"`
	EntryTime  time.Time `json:"entry_time"`
	ExitTime   time.Time `json:"exit_time"`
	EntryType  string    `json:"entry_type"`
	ExitType   string    `json:"exit_type"`
	Quantity   float64   `json:"quantity"`
	EntryPrice float64   `json:"entry_price"`
	ExitPrice  float64   `json:"exit_price"`
	Costs      float64   `json:"costs"`
	PnL        float64   `json:"net_pnl"`
	Return     float64   `json:"net_return"`
	Holding    float64   `json:"holding_calendar_days"`
}

type Breakdown struct {
	Trades     int      `json:"closed_lots"`
	WinRate    *float64 `json:"win_rate"`
	AvgReturn  *float64 `json:"avg_return"`
	AvgHolding *float64 `json:"avg_holding_calendar_days"`
}

type Metrics struct {
	CAGR          *float64           `json:"cagr"`
	Vol           float64            `json:"annualized_vol"`
	Sharpe        *float64           `json:"sharpe"`
	Sortino       *float64           `json:"sortino"`
	MaxDrawdown   float64            `json:"max_drawdown"`
	Peak          time.Time          `json:"drawdown_peak"`
	Trough        time.Time          `json:"drawdown_trough"`
	Calmar        *float64           `json:"calmar"`
	Exposure      float64            `json:"avg_invested_weight"`
	Trades        int                `json:"closed_lots"`
	WinRate       *float64           `json:"win_rate"`
	AvgWin        *float64           `json:"avg_win_return"`
	AvgLoss       *float64           `json:"avg_loss_return"`
	ProfitFactor  *float64           `json:"profit_factor"`
	AvgHolding    *float64           `json:"avg_holding_calendar_days"`
	Costs         float64            `json:"total_costs"`
	Years         map[string]float64 `json:"calendar_year_returns"`
	Last12Months  float64            `json:"last_12_months_return"`
	Last12Partial bool               `json:"last_12_months_partial"`
}

type StrategyResult struct {
	Name          string               `json:"name"`
	Metrics       Metrics              `json:"metrics"`
	Curve         []EquityPoint        `json:"equity_curve"`
	Decisions     []Decision           `json:"decisions"`
	Trades        []Trade              `json:"closed_lots"`
	Entries       map[string]Breakdown `json:"entry_breakdown,omitempty"`
	Exits         map[string]int       `json:"exit_order_counts,omitempty"`
	Unfilled      int                  `json:"missing_open_orders"`
	OpenPositions int                  `json:"open_positions_at_end"`
}

type SymbolCoverage struct {
	Bars      int       `json:"execution_bars"`
	DailyBars int       `json:"daily_bars"`
	Matched   int       `json:"evaluation_bars_on_spy_calendar"`
	First     time.Time `json:"first_bar"`
	Last      time.Time `json:"last_bar"`
}

type Coverage struct {
	UniverseSize int                       `json:"universe_size"`
	Symbols      map[string]SymbolCoverage `json:"symbols"`
	Skipped      map[string]string         `json:"skipped"`
	Warnings     map[string]string         `json:"warnings"`
	Sources      map[string]string         `json:"sources"`
	Excluded     []string                  `json:"excluded"`
	First        time.Time                 `json:"first_signal_close"`
	Last         time.Time                 `json:"last_mark_close"`
	Bars         int                       `json:"evaluation_bars"`
	SPYBars      int                       `json:"spy_bars_including_warmup"`
	Removed      map[string]int            `json:"invalid_or_duplicate_bars_removed"`
}

type Backtest struct {
	Config     Config           `json:"config"`
	Coverage   Coverage         `json:"coverage"`
	Strategies []StrategyResult `json:"strategies"`
}

type lot struct {
	Quantity, Price, EntryCost, ATR float64
	Time                            time.Time
	Entry                           string
}

type position struct {
	Quantity, Mark float64
	Lots           []lot
}

type portfolio struct {
	Cash, Costs float64
	Positions   map[string]*position
	Trades      []Trade
	Exits       map[string]int
	Unfilled    int
}

func (p *portfolio) equity() float64 {
	e := p.Cash
	for _, s := range sortedKeys(p.Positions) {
		v := p.Positions[s]
		e += v.Quantity * v.Mark
	}
	return e
}

func (p *portfolio) buy(s string, notional, price, fee float64, at time.Time, entry string, atr float64) {
	notional = math.Min(notional, p.Cash/(1+fee))
	if notional <= initialEquity*1e-12 {
		return
	}
	cost, qty := notional*fee, notional/price
	pos := p.Positions[s]
	if pos == nil {
		pos = &position{}
		p.Positions[s] = pos
	}
	pos.Quantity += qty
	pos.Mark = price
	pos.Lots = append(pos.Lots, lot{qty, price, cost, atr, at, entry})
	p.Cash -= notional + cost
	p.Costs += cost
}

// FIFO closes preserve actual acquisition dates and allocate entry costs to
// partial reductions. Metrics count closed lot portions, not buy/sell orders.
func (p *portfolio) sell(s string, qty, price, fee float64, at time.Time, exit string) {
	pos := p.Positions[s]
	qty = math.Min(qty, pos.Quantity)
	if qty <= initialEquity*1e-14 {
		return
	}
	cost := qty * price * fee
	p.Cash += qty*price - cost
	p.Costs += cost
	p.Exits[exit]++
	remaining := qty
	for remaining > 0 && len(pos.Lots) > 0 {
		l := &pos.Lots[0]
		n := math.Min(remaining, l.Quantity)
		entryCost := l.EntryCost * n / l.Quantity
		exitCost := n * price * fee
		basis := n*l.Price + entryCost
		pnl := n*price - exitCost - basis
		p.Trades = append(p.Trades, Trade{s, l.Time, at, l.Entry, exit, n, l.Price, price, entryCost + exitCost, pnl, pnl / basis, at.Sub(l.Time).Hours() / 24})
		remaining -= n
		l.Quantity -= n
		l.EntryCost -= entryCost
		if l.Quantity <= 0 {
			pos.Lots = pos.Lots[1:]
		}
	}
	pos.Quantity -= qty
	if pos.Quantity <= initialEquity*1e-14 || len(pos.Lots) == 0 {
		delete(p.Positions, s)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func barsAt(stocks map[string]history, key time.Time) map[string]usstock.Bar {
	out := make(map[string]usstock.Bar)
	for s, h := range stocks {
		if i, ok := h.index[key]; ok {
			out[s] = h.bars[i]
		}
	}
	return out
}

func (p *portfolio) mark(bars map[string]usstock.Bar, open bool) {
	for s, pos := range p.Positions {
		if b, ok := bars[s]; ok {
			pos.Mark = b.Close
			if open {
				pos.Mark = b.Open
			}
		}
	}
}

func (p *portfolio) fill(d Decision, bars map[string]usstock.Bar, key time.Time, c Config, limit int) {
	p.mark(bars, true)
	fee := c.CostBPS / 10000
	if d.Rebalance {
		p.fillTargets(d.Targets, bars, key, fee, limit)
		return
	}
	for _, o := range d.Orders {
		if o.Exit == "" || p.Positions[o.Symbol] == nil {
			continue
		}
		if b, ok := bars[o.Symbol]; ok {
			p.sell(o.Symbol, p.Positions[o.Symbol].Quantity, b.Open, fee, key, o.Exit)
		} else {
			p.Unfilled++
		}
	}
	budget := p.equity() / float64(c.MaxPos)
	for _, o := range d.Orders {
		if o.Entry == "" || p.Positions[o.Symbol] != nil || len(p.Positions) >= limit {
			continue
		}
		if b, ok := bars[o.Symbol]; ok {
			p.buy(o.Symbol, budget, b.Open, fee, key, o.Entry, o.ATR)
		} else {
			p.Unfilled++
		}
	}
}

// Solve post-cost equity E + fee*turnover(E) = pre-trade equity so normal
// rebalances attain exact target weights after fees. Missing opens are fixed
// holdings; cash and slot checks also handle those constrained rebalances.
func (p *portfolio) fillTargets(targets []Target, bars map[string]usstock.Bar, key time.Time, fee float64, limit int) {
	weights := make(map[string]float64)
	for _, t := range targets {
		if _, ok := bars[t.Symbol]; ok {
			weights[t.Symbol] = t.Weight
		} else {
			p.Unfilled++
		}
	}
	current := make(map[string]float64)
	for s, pos := range p.Positions {
		if b, ok := bars[s]; ok {
			current[s] = pos.Quantity * b.Open
		} else {
			p.Unfilled++
		}
	}
	all := make(map[string]bool)
	for s := range current {
		all[s] = true
	}
	for s := range weights {
		all[s] = true
	}
	symbols := sortedKeys(all)
	pre := p.equity()
	lo, hi := 0.0, pre
	for iter := 0; iter < 64; iter++ {
		mid, turnover := (lo+hi)/2, 0.0
		for _, s := range symbols {
			turnover += math.Abs(mid*weights[s] - current[s])
		}
		if mid+fee*turnover > pre {
			hi = mid
		} else {
			lo = mid
		}
	}
	equity := (lo + hi) / 2
	for _, s := range symbols {
		if excess := current[s] - equity*weights[s]; excess > initialEquity*1e-12 {
			p.sell(s, excess/bars[s].Open, bars[s].Open, fee, key, "REBALANCE")
		}
	}
	// Targets retain signal-date momentum order, including when a missing exit
	// leaves fewer slots or less cash than expected. Never substitute new names.
	for _, t := range targets {
		b, ok := bars[t.Symbol]
		if !ok || (p.Positions[t.Symbol] == nil && len(p.Positions) >= limit) {
			continue
		}
		value := 0.0
		if pos := p.Positions[t.Symbol]; pos != nil {
			value = pos.Quantity * b.Open
		}
		p.buy(t.Symbol, equity*t.Weight-value, b.Open, fee, key, "REBALANCE", 0)
	}
}

func decide(name string, key time.Time, stocks, daily map[string]history, p *portfolio, c Config, rebalance, first bool, ew []string) Decision {
	d := Decision{Time: key}
	ranks := make([]Rank, 0, len(stocks))
	for _, s := range sortedKeys(stocks) {
		if s != "SPYBUSDT" {
			if _, ok := stocks[s].index[key]; ok {
				ranks = append(ranks, rankAt(s, daily[s], key, c.Timeframe))
			}
		}
	}
	sort.Slice(ranks, func(i, j int) bool { return rankLess(ranks[i], ranks[j]) })
	switch name {
	case "MOM_TOP", "MOM_X_MA":
		if rebalance {
			d.Rebalance = true
			selected := 0
			for _, r := range ranks {
				if !r.Valid || !r.Trend || selected >= c.MaxPos {
					continue
				}
				selected++ // Timing filters the selected top N, without backfill.
				h := stocks[r.Symbol]
				i := h.index[key]
				if name == "MOM_X_MA" && !(h.ind[i].Fast > h.ind[i].Slow) {
					continue
				}
				d.Targets = append(d.Targets, Target{r.Symbol, 1 / float64(c.MaxPos)})
			}
		}
	case "SPY_BH":
		if first {
			d.Rebalance = true
			d.Targets = []Target{{"SPYBUSDT", 1}}
		}
	case "EW_BH":
		if rebalance {
			d.Rebalance = true
			for _, s := range ew {
				d.Targets = append(d.Targets, Target{s, 1 / float64(len(ew))})
			}
		}
	}
	for _, s := range sortedKeys(p.Positions) {
		h := stocks[s]
		i, ok := h.index[key]
		if !ok {
			continue
		}
		exit := ""
		switch name {
		case "MA_CROSS", "MOM_X_MA":
			if death(h.ind, i) {
				exit = "X1"
			}
		case "MA2060_RULES":
			l := p.Positions[s].Lots[0]
			exit = exitRule(h.bars, h.ind, i, l.Price, l.ATR, l.Entry == "E4")
		}
		if exit != "" {
			d.Orders = append(d.Orders, Order{Symbol: s, Exit: exit})
			if d.Rebalance { // A death cross also vetoes a rebalance purchase.
				for j, t := range d.Targets {
					if t.Symbol == s {
						d.Targets = append(d.Targets[:j], d.Targets[j+1:]...)
						break
					}
				}
			}
		}
	}
	if name == "MA_CROSS" || name == "MA2060_RULES" {
		// Allocate slots at the signal close, before next-open availability is
		// known. Anticipated exits free slots; failed exit fills can only reduce
		// actual purchases. A missing top-priority open never promotes a name.
		free := c.MaxPos - len(p.Positions) + len(d.Orders)
		for _, r := range ranks {
			if free <= 0 {
				break
			}
			if p.Positions[r.Symbol] != nil {
				continue
			}
			h := stocks[r.Symbol]
			i := h.index[key]
			entry := ""
			if name == "MA_CROSS" && golden(h.ind, i) {
				entry = "E1"
			}
			if name == "MA2060_RULES" {
				entry = entryRule(h.bars, h.ind, i)
			}
			if entry != "" {
				d.Orders = append(d.Orders, Order{Symbol: r.Symbol, Entry: entry, ATR: h.ind[i].ATR})
				free--
			}
		}
	}
	return d
}

// runBacktest is pure: benchmark calendar, forward-only indicators, decisions,
// next-calendar-bar fills, then close marks. No terminal liquidation is assumed.
func runBacktest(input, dailyInput map[string][]usstock.Bar, spyBars, spyDaily []usstock.Bar, c Config) (*Backtest, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	spy := makeHistory(spyBars, c.Timeframe)
	spyD := makeHistory(spyDaily, usstock.TF1d)
	start := -1
	for i, key := range spy.keys {
		lastOfDay := i+1 == len(spy.keys) || !date(spy.keys[i+1]).Equal(date(key))
		if i >= SlowPeriod && rankAt("SPYBUSDT", spyD, key, c.Timeframe).Valid && lastOfDay {
			start = i
			break
		}
	}
	if start < 0 || start+1 >= len(spy.keys) {
		return nil, fmt.Errorf("need 253 completed daily SPY bars, 61 execution bars, and at least one subsequent fill bar")
	}
	r := &Backtest{Config: c, Coverage: Coverage{Symbols: make(map[string]SymbolCoverage), Skipped: make(map[string]string), Warnings: make(map[string]string), Sources: make(map[string]string), Removed: make(map[string]int), First: spy.keys[start], Last: spy.keys[len(spy.keys)-1], Bars: len(spy.keys) - start, SPYBars: len(spy.keys)}}
	stocks := map[string]history{"SPYBUSDT": spy}
	daily := map[string]history{"SPYBUSDT": spyD}
	r.Coverage.Removed["SPYBUSDT"] = len(spyBars) - len(spy.bars)
	var ew []string
	for _, s := range sortedKeys(input) {
		if s == "SPYBUSDT" || s == "QQQBUSDT" {
			continue
		}
		h := makeHistory(input[s], c.Timeframe)
		r.Coverage.Removed[s] = len(input[s]) - len(h.bars)
		if len(h.bars) == 0 {
			r.Coverage.Skipped[s] = "no valid execution bars"
			continue
		}
		stocks[s], daily[s] = h, makeHistory(dailyInput[s], usstock.TF1d)
		cov := SymbolCoverage{Bars: len(h.bars), DailyBars: len(daily[s].bars), First: h.keys[0], Last: h.keys[len(h.keys)-1]}
		for _, key := range spy.keys[start:] {
			if _, ok := h.index[key]; ok {
				cov.Matched++
			}
		}
		r.Coverage.Symbols[s] = cov
		if _, ok := h.index[spy.keys[start]]; ok {
			ew = append(ew, s)
		}
	}
	r.Coverage.UniverseSize = len(r.Coverage.Symbols)
	if len(r.Coverage.Symbols) == 0 {
		return nil, fmt.Errorf("no eligible stocks loaded")
	}
	// Prepare once; every strategy sees identical open availability and ranks.
	barMaps := make([]map[string]usstock.Bar, len(spy.keys)-start)
	day := make([]int, len(spy.keys)-start)
	for j := range barMaps {
		i := start + j
		barMaps[j] = barsAt(stocks, spy.keys[i])
		if j > 0 {
			day[j] = day[j-1]
			if !date(spy.keys[i]).Equal(date(spy.keys[i-1])) {
				day[j]++
			}
		}
	}
	for _, name := range strategyNames {
		p := &portfolio{Cash: initialEquity, Positions: make(map[string]*position), Exits: make(map[string]int)}
		result := StrategyResult{Name: name}
		limit := c.MaxPos
		if name == "SPY_BH" {
			limit = 1
		} else if name == "EW_BH" {
			limit = len(ew) // Explicit all-name benchmark exception to maxpos.
		}
		var pending Decision
		for j, bars := range barMaps {
			i, key := start+j, spy.keys[start+j]
			if j > 0 {
				p.fill(pending, bars, key, c, limit)
			}
			p.mark(bars, false)
			equity := p.equity()
			result.Curve = append(result.Curve, EquityPoint{key, equity, p.Cash, (equity - p.Cash) / equity, len(p.Positions)})
			lastOfDay := i+1 == len(spy.keys) || !date(spy.keys[i+1]).Equal(date(key))
			rebalance := lastOfDay && day[j]%c.Rebalance == 0
			if name == "EW_BH" {
				rebalance = j%63 == 0
			}
			pending = decide(name, key, stocks, daily, p, c, rebalance, j == 0, ew)
			if pending.Rebalance || len(pending.Orders) > 0 {
				result.Decisions = append(result.Decisions, pending)
			}
		}
		result.Trades, result.Exits = p.Trades, p.Exits
		result.OpenPositions, result.Unfilled = len(p.Positions), p.Unfilled
		result.Metrics = computeMetrics(result.Curve, result.Trades, p.Costs, c.Timeframe)
		if name == "MA2060_RULES" {
			result.Entries = entryBreakdown(p.Trades)
		}
		r.Strategies = append(r.Strategies, result)
	}
	return r, nil
}

func number(v float64) *float64 {
	if !finite(v) {
		return nil
	}
	return &v
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var total float64
	for _, x := range xs {
		total += x
	}
	return total / float64(len(xs))
}

func sampleStd(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m, sum := mean(xs), 0.0
	for _, x := range xs {
		sum += (x - m) * (x - m)
	}
	return math.Sqrt(sum / float64(len(xs)-1))
}

func computeMetrics(curve []EquityPoint, trades []Trade, costs float64, tf string) Metrics {
	m := Metrics{Costs: costs, Trades: len(trades), Years: make(map[string]float64)}
	if len(curve) == 0 {
		return m
	}
	first, last := curve[0], curve[len(curve)-1]
	years := last.Time.Sub(first.Time).Hours() / (24 * 365.25)
	if years > 0 && first.Equity > 0 && last.Equity > 0 {
		m.CAGR = number(math.Pow(last.Equity/first.Equity, 1/years) - 1)
	}
	peak, peakTime := first.Equity, first.Time
	m.Peak, m.Trough = first.Time, first.Time
	var returns []float64
	for i, pt := range curve {
		if pt.Equity > peak {
			peak, peakTime = pt.Equity, pt.Time
		}
		if dd := 1 - pt.Equity/peak; dd > m.MaxDrawdown {
			m.MaxDrawdown, m.Peak, m.Trough = dd, peakTime, pt.Time
		}
		if i > 0 {
			returns = append(returns, pt.Equity/curve[i-1].Equity-1)
			m.Exposure += pt.Exposure
			year := pt.Time.Format("2006")
			growth, ok := m.Years[year]
			if !ok {
				growth = 1
			}
			m.Years[year] = growth * pt.Equity / curve[i-1].Equity
		}
	}
	for y, growth := range m.Years {
		m.Years[y] = growth - 1
	}
	periods := 252.0
	if tf == usstock.TF1h {
		// Yahoo includes a final half-hour candle: use observed bars/session,
		// rather than pretending every session has exactly 6.5 or 7 candles.
		days := make(map[time.Time]bool)
		for _, pt := range curve {
			days[date(pt.Time)] = true
		}
		periods *= float64(len(curve)) / float64(len(days))
	}
	if len(returns) > 0 {
		m.Exposure /= float64(len(returns))
		avg, std := mean(returns), sampleStd(returns)
		m.Vol = std * math.Sqrt(periods)
		if std > 0 {
			m.Sharpe = number(avg / std * math.Sqrt(periods))
		}
		down := 0.0
		for _, r := range returns {
			if r < 0 {
				down += r * r
			}
		}
		if down > 0 {
			m.Sortino = number(avg / math.Sqrt(down/float64(len(returns))) * math.Sqrt(periods))
		}
	}
	if m.CAGR != nil && m.MaxDrawdown > 0 {
		m.Calmar = number(*m.CAGR / m.MaxDrawdown)
	}
	cutoff := last.Time.AddDate(-1, 0, 0)
	base := first
	m.Last12Partial = first.Time.After(cutoff)
	for _, pt := range curve {
		if pt.Time.After(cutoff) {
			break
		}
		base = pt
	}
	m.Last12Months = last.Equity/base.Equity - 1
	var wins, losses, holdings []float64
	var grossWin, grossLoss float64
	for _, t := range trades {
		holdings = append(holdings, t.Holding)
		if t.PnL > 0 {
			wins = append(wins, t.Return)
			grossWin += t.PnL
		} else if t.PnL < 0 {
			losses = append(losses, t.Return)
			grossLoss -= t.PnL
		}
	}
	if len(trades) > 0 {
		m.WinRate = number(float64(len(wins)) / float64(len(trades)))
		m.AvgHolding = number(mean(holdings))
	}
	if len(wins) > 0 {
		m.AvgWin = number(mean(wins))
	}
	if len(losses) > 0 {
		m.AvgLoss = number(mean(losses))
	}
	if grossLoss > 0 {
		m.ProfitFactor = number(grossWin / grossLoss)
	}
	return m
}

func entryBreakdown(trades []Trade) map[string]Breakdown {
	out := make(map[string]Breakdown)
	for _, entry := range []string{"E1", "E2", "E3", "E4"} {
		var returns, holdings []float64
		wins := 0
		for _, t := range trades {
			if t.EntryType == entry {
				returns = append(returns, t.Return)
				holdings = append(holdings, t.Holding)
				if t.PnL > 0 {
					wins++
				}
			}
		}
		b := Breakdown{Trades: len(returns)}
		if b.Trades > 0 {
			b.WinRate = number(float64(wins) / float64(b.Trades))
			b.AvgReturn, b.AvgHolding = number(mean(returns)), number(mean(holdings))
		}
		out[entry] = b
	}
	return out
}
