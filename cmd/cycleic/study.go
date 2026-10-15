package main

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/kernel/stockengine"
	"nofx/market/usstock"
)

const (
	warmupIndex  = 259 // The signal date is the 260th SPY bar.
	windowBars   = 300
	betaDays     = 126
	minBetaPairs = 60
	minNames     = 20
)

type StudyConfig struct {
	Horizons    []int   `json:"horizons"`
	MainHorizon int     `json:"main_horizon"`
	TopN        int     `json:"top_n"`
	Rebalance   int     `json:"rebalance"`
	CostBPS     float64 `json:"cost_bps"`
	Signal      string  `json:"signal"`
}

// signalNames lists the supported ranking signals; kell is the production one.
var signalNames = []string{"kell", "rs63", "ret63", "mom12_1", "trend_rs", "trend_mom", "sma200_mom", "ext10"}

func validSignal(name string) bool {
	for _, n := range signalNames {
		if n == name {
			return true
		}
	}
	return false
}

func isKell(name string) bool { return name == "" || name == "kell" }

func (c StudyConfig) normalized() (StudyConfig, error) {
	if c.MainHorizon <= 0 || c.TopN <= 0 || c.Rebalance <= 0 || !finite(c.CostBPS) || c.CostBPS < 0 {
		return c, fmt.Errorf("main-horizon, topn and rebalance must be positive; cost-bps must be finite and nonnegative")
	}
	if c.Signal == "" {
		c.Signal = "kell"
	}
	if !validSignal(c.Signal) {
		return c, fmt.Errorf("unknown signal %q; use one of %s", c.Signal, strings.Join(signalNames, ", "))
	}
	horizons := map[int]bool{c.MainHorizon: true}
	for _, h := range c.Horizons {
		if h <= 0 {
			return c, fmt.Errorf("horizons must be positive")
		}
		horizons[h] = true
	}
	c.Horizons = make([]int, 0, len(horizons))
	for h := range horizons {
		c.Horizons = append(c.Horizons, h)
	}
	sort.Ints(c.Horizons)
	return c, nil
}

// Undefined ratios are nil (JSON null), never fabricated zeros or NaNs.
// Means/dispersion on empty samples are zero; N=0 makes them unavailable.
type Summary struct {
	N              int      `json:"n_dates"`
	Mean           float64  `json:"mean"`
	Std            float64  `json:"std"`
	ICIR           *float64 `json:"icir"`
	NWSE           float64  `json:"nw_se"`
	NWT            *float64 `json:"nw_t"`
	CILow          *float64 `json:"ci_low"`
	CIHigh         *float64 `json:"ci_high"`
	HitRate        float64  `json:"hit_rate"`
	NonOverlapN    int      `json:"nonoverlap_n"`
	NonOverlapMean *float64 `json:"nonoverlap_mean"`
	NonOverlapT    *float64 `json:"nonoverlap_t"`
}

type ValuePoint struct {
	Date  string  `json:"date"`
	Index int     `json:"spy_index"`
	Value float64 `json:"value"`
}

type ICPoint struct {
	Date     string   `json:"date"`
	Index    int      `json:"spy_index"`
	Names    int      `json:"names"`
	Spearman *float64 `json:"spearman"`
	Pearson  *float64 `json:"pearson"`
}

type ICResult struct {
	Horizon  int       `json:"horizon"`
	Target   string    `json:"target"`
	Spearman Summary   `json:"spearman"`
	Pearson  Summary   `json:"pearson"`
	Series   []ICPoint `json:"series"`
}

type YearResult struct {
	Year    string  `json:"year"`
	Summary Summary `json:"summary"`
}

type EventPoint struct {
	ValuePoint
	Names int `json:"names"`
}

type PhaseResult struct {
	Phase    string       `json:"phase"`
	Summary  Summary      `json:"summary"`
	AvgNames float64      `json:"avg_names"`
	Series   []EventPoint `json:"series"`
}

type Signal struct {
	Symbol    string                 `json:"symbol"`
	Phase     stockengine.CyclePhase `json:"phase"`
	LongSetup bool                   `json:"long_setup"`
	Score     float64                `json:"score"`
	S         float64                `json:"signal"`
	RS63Pct   float64                `json:"rs63_pct"`
	Beta      float64                `json:"beta"`
}

type PortfolioPeriod struct {
	Date        string   `json:"signal_date"`
	EntryDate   string   `json:"entry_date"`
	ExitDate    string   `json:"exit_date"`
	Symbols     []string `json:"symbols"`
	Raw         float64  `json:"raw"`
	Residual    float64  `json:"residual"`
	UniverseRaw float64  `json:"universe_raw"`
	Excess      float64  `json:"excess"`
	Turnover    float64  `json:"one_way_turnover"`
	Cost        float64  `json:"cost"`
	NetExcess   float64  `json:"net_excess"`
	Unpriced    []string `json:"unpriced_selected,omitempty"`
}

type ReturnSummary struct {
	N          int      `json:"n_periods"`
	AnnualMean float64  `json:"annualized_mean"`
	AnnualVol  float64  `json:"annualized_vol"`
	Sharpe     *float64 `json:"sharpe_like"`
	HitRate    float64  `json:"hit_rate"`
}

type PortfolioResult struct {
	Periods     []PortfolioPeriod `json:"periods"`
	Residual    ReturnSummary     `json:"residual"`
	Excess      ReturnSummary     `json:"excess"`
	NetExcess   ReturnSummary     `json:"net_excess"`
	AvgNames    float64           `json:"avg_names"`
	AvgTurnover float64           `json:"avg_one_way_turnover"`
	Skipped     map[string]string `json:"skipped_periods,omitempty"`
}

type Coverage struct {
	UniverseSize    int               `json:"universe_size"`
	Loaded          []string          `json:"symbols_loaded"`
	Skipped         map[string]string `json:"symbols_skipped"`
	Warnings        map[string]string `json:"load_warnings,omitempty"`
	Sources         map[string]string `json:"sources,omitempty"`
	Excluded        []string          `json:"excluded,omitempty"`
	RemovedBars     map[string]int    `json:"removed_invalid_or_duplicate_bars,omitempty"`
	BenchmarkBars   int               `json:"benchmark_bars"`
	CandidateDates  int               `json:"candidate_dates"`
	EvaluationDates int               `json:"main_horizon_evaluation_dates"`
	FirstDate       string            `json:"first_evaluation_date"`
	LastDate        string            `json:"last_evaluation_date"`
	AvgStocks       float64           `json:"avg_stocks_per_date"`
	AvgLongSetups   float64           `json:"avg_long_setups_per_date"`
}

type StudyResult struct {
	Config       StudyConfig     `json:"config"`
	IC           []ICResult      `json:"ic"`
	Years        []YearResult    `json:"ic_by_year"`
	LongOnly     ICResult        `json:"long_setup_only"`
	Phases       []PhaseResult   `json:"phase_events"`
	Spread       Summary         `json:"long_vs_rest_spread"`
	SpreadSeries []ValuePoint    `json:"spread_series"`
	Portfolio    PortfolioResult `json:"portfolio"`
	Coverage     Coverage        `json:"coverage"`
}

type history struct {
	bars   []usstock.Bar
	dates  []time.Time
	byDate map[time.Time]int
}

func utcDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Normalize copies inputs and keeps the latest valid bar on a UTC date.
func makeHistory(bars []usstock.Bar) history {
	ordered := append([]usstock.Bar(nil), bars...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].OpenTime.Before(ordered[j].OpenTime) })
	h := history{byDate: make(map[time.Time]int)}
	for _, b := range ordered {
		if b.OpenTime.IsZero() || !finite(b.Open) || !finite(b.High) || !finite(b.Low) || !finite(b.Close) || !finite(b.Volume) || b.Open <= 0 || b.Low <= 0 || b.Close <= 0 || b.High < math.Max(b.Open, b.Close) || b.Low > math.Min(b.Open, b.Close) || b.Volume < 0 {
			continue
		}
		d := utcDate(b.OpenTime)
		if j, ok := h.byDate[d]; ok {
			h.bars[j] = b
			continue
		}
		h.byDate[d] = len(h.bars)
		h.bars = append(h.bars, b)
		h.dates = append(h.dates, d)
	}
	return h
}

func (h history) window(d time.Time) []usstock.Bar {
	end := sort.Search(len(h.dates), func(i int) bool { return h.dates[i].After(d) })
	return h.bars[max(0, end-windowBars):end]
}

// Beta uses daily returns with BOTH adjacent SPY dates present. A missing
// stock session must not turn a multi-day return into a daily beta observation.
func betaAt(stock, spy history, t int) (float64, bool) {
	x, y := make([]float64, 0, betaDays), make([]float64, 0, betaDays)
	for j := max(1, t-betaDays+1); j <= t; j++ {
		a, okA := stock.byDate[spy.dates[j-1]]
		b, okB := stock.byDate[spy.dates[j]]
		if !okA || !okB {
			continue
		}
		rx := spy.bars[j].Close/spy.bars[j-1].Close - 1
		ry := stock.bars[b].Close/stock.bars[a].Close - 1
		if finite(rx) && finite(ry) {
			x, y = append(x, rx), append(y, ry)
		}
	}
	if len(x) < minBetaPairs {
		return 0, false
	}
	mx, my := mean(x), mean(y)
	var covariance, variance float64
	for j := range x {
		covariance += (x[j] - mx) * (y[j] - my)
		variance += (x[j] - mx) * (x[j] - mx)
	}
	if variance <= 0 {
		return 0, false
	}
	beta := covariance / variance
	return beta, finite(beta)
}

// localEMA replicates kernel/stockengine's unexported ema: zeros before the
// period-th value, an SMA seed there, then standard exponential smoothing.
func localEMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 0 || len(values) < period {
		return out
	}
	for _, v := range values[:period] {
		out[period-1] += v / float64(period)
	}
	alpha := 2 / float64(period+1)
	for i := period; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

// signalValue maps the already computed window and ClassifyCycle assessment to
// the ranking value of the named signal. It only looks at the window (bars with
// date <= t), so it is point-in-time by construction. ok=false skips the
// stock-date.
func signalValue(kind string, stockWin, spyWin []usstock.Bar, a stockengine.CycleAssessment) (float64, bool) {
	n := len(stockWin)
	switch kind {
	case "", "kell":
		if a.LongSetup {
			return a.Score, true
		}
		return 0, true
	case "rs63":
		// ClassifyCycle reports 0 when relative strength is unavailable; that
		// is a valid zero only when both windows are long enough to compute it.
		return a.RS63Pct, n > 64 && len(spyWin) > 64
	case "ret63":
		if n < 64 {
			return 0, false
		}
		return stockWin[n-1].Close/stockWin[n-64].Close - 1, true
	case "mom12_1":
		if n < 253 {
			return 0, false
		}
		return stockWin[n-22].Close/stockWin[n-253].Close - 1, true
	case "trend_rs":
		if n < 50 {
			return 0, false
		}
		closes := make([]float64, n)
		for i, b := range stockWin {
			closes[i] = b.Close
		}
		ema20 := localEMA(closes, 20)[n-1]
		sma50 := mean(closes[n-50:])
		if closes[n-1] > ema20 && ema20 > sma50 {
			return a.RS63Pct + 1000, true
		}
		return a.RS63Pct - 1000, true
	case "trend_mom", "sma200_mom":
		// Tiered: trend names outrank every non-trend name, 12-1 momentum
		// ranks within each tier (momentum is a return, |x| << 1000).
		if n < 253 {
			return 0, false
		}
		closes := make([]float64, n)
		for i, b := range stockWin {
			closes[i] = b.Close
		}
		mom := closes[n-22]/closes[n-253] - 1
		inTrend := closes[n-1] > mean(closes[n-200:])
		if kind == "trend_mom" {
			ema20 := localEMA(closes, 20)[n-1]
			inTrend = closes[n-1] > ema20 && ema20 > mean(closes[n-50:])
		}
		if inTrend {
			return mom + 1000, true
		}
		return mom - 1000, true
	case "ext10":
		return a.Ext10ATR, true
	}
	return 0, false
}

func signalAt(symbol string, stock, spy history, t int, kind string) (Signal, bool) {
	if _, ok := stock.byDate[spy.dates[t]]; !ok {
		return Signal{}, false
	}
	stockWin := stock.window(spy.dates[t])
	if len(stockWin) < warmupIndex+1 {
		return Signal{}, false
	}
	beta, ok := betaAt(stock, spy, t)
	if !ok {
		return Signal{}, false
	}
	spyWin := spy.window(spy.dates[t])
	a := stockengine.ClassifyCycle(stockWin, spyWin)
	if !finite(a.Score) || !finite(a.RS63Pct) {
		return Signal{}, false
	}
	value, ok := signalValue(kind, stockWin, spyWin, a)
	if !ok || !finite(value) {
		return Signal{}, false
	}
	return Signal{Symbol: symbol, Phase: a.Phase, LongSetup: a.LongSetup, Score: a.Score, S: value, RS63Pct: a.RS63Pct, Beta: beta}, true
}

func forwardReturn(stock, spy history, t, horizon int, beta float64) (raw, residual float64, ok bool) {
	if horizon <= 0 || t < 0 || horizon > len(spy.bars)-t-2 {
		return 0, 0, false
	}
	entry, exit := t+1, t+1+horizon
	a, okA := stock.byDate[spy.dates[entry]]
	b, okB := stock.byDate[spy.dates[exit]]
	if !okA || !okB {
		return 0, 0, false
	}
	raw = stock.bars[b].Close/stock.bars[a].Close - 1
	market := spy.bars[exit].Close/spy.bars[entry].Close - 1
	residual = raw - beta*market
	return raw, residual, finite(raw) && finite(residual)
}

type observation struct {
	Signal
	raw, residual float64
}

func observations(signals []Signal, stocks map[string]history, spy history, t, h int) []observation {
	rows := make([]observation, 0, len(signals))
	for _, s := range signals {
		if r, e, ok := forwardReturn(stocks[s.Symbol], spy, t, h, s.Beta); ok {
			rows = append(rows, observation{s, r, e})
		}
	}
	return rows
}

// runStudy is pure: it neither loads data nor writes reports. Each worker owns
// one date's output slot; aggregation always follows calendar/symbol order.
func runStudy(input map[string][]usstock.Bar, spyBars []usstock.Bar, config StudyConfig) (*StudyResult, error) {
	c, err := config.normalized()
	if err != nil {
		return nil, err
	}
	spy := makeHistory(spyBars)
	maxH := c.Horizons[len(c.Horizons)-1]
	if maxH > len(spy.bars)-warmupIndex-2 || c.Rebalance > len(spy.bars)-warmupIndex-2 {
		return nil, fmt.Errorf("need at least 260 + 1 + max(horizons, rebalance) valid SPY dates; have %d", len(spy.bars))
	}
	r := &StudyResult{Config: c, Coverage: Coverage{Skipped: make(map[string]string), RemovedBars: make(map[string]int), BenchmarkBars: len(spy.bars)}}
	if removed := len(spyBars) - len(spy.bars); removed > 0 {
		r.Coverage.RemovedBars["SPYBUSDT"] = removed
	}
	stocks := make(map[string]history)
	symbols := make([]string, 0, len(input))
	for symbol := range input {
		if symbol != "SPYBUSDT" {
			symbols = append(symbols, symbol)
		}
	}
	sort.Strings(symbols)
	r.Coverage.UniverseSize = len(symbols)
	for _, symbol := range symbols {
		h := makeHistory(input[symbol])
		if removed := len(input[symbol]) - len(h.bars); removed > 0 {
			r.Coverage.RemovedBars[symbol] = removed
		}
		if len(h.bars) < warmupIndex+1 {
			r.Coverage.Skipped[symbol] = fmt.Sprintf("have %d valid dates; need 260", len(h.bars))
			continue
		}
		stocks[symbol] = h
		r.Coverage.Loaded = append(r.Coverage.Loaded, symbol)
	}
	lastIC, lastPortfolio := len(spy.bars)-2-maxH, len(spy.bars)-2-c.Rebalance
	r.Coverage.CandidateDates = lastIC - warmupIndex + 1
	lastSignal := max(lastIC, lastPortfolio)
	signals := make([][]Signal, lastSignal-warmupIndex+1)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < min(runtime.GOMAXPROCS(0), len(signals)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for offset := range jobs {
				for _, symbol := range r.Coverage.Loaded {
					if s, ok := signalAt(symbol, stocks[symbol], spy, offset+warmupIndex, c.Signal); ok {
						signals[offset] = append(signals[offset], s)
					}
				}
			}
		}()
	}
	for offset := range signals {
		jobs <- offset
	}
	close(jobs)
	wg.Wait()

	years := make(map[string][]ValuePoint)
	phaseEvents := make(map[string][]EventPoint)
	r.LongOnly = ICResult{Horizon: c.MainHorizon, Target: "residual"}
	for _, h := range c.Horizons {
		residual, raw := ICResult{Horizon: h, Target: "residual"}, ICResult{Horizon: h, Target: "raw"}
		for t := warmupIndex; t <= lastIC; t++ {
			rows := observations(signals[t-warmupIndex], stocks, spy, t, h)
			if len(rows) < minNames {
				continue
			}
			date := spy.dates[t].Format(time.DateOnly)
			residual.Series = append(residual.Series, icPoint(rows, date, t, true))
			raw.Series = append(raw.Series, icPoint(rows, date, t, false))
			if h != c.MainHorizon {
				continue
			}
			r.Coverage.EvaluationDates++
			if r.Coverage.FirstDate == "" {
				r.Coverage.FirstDate = date
			}
			r.Coverage.LastDate = date
			r.Coverage.AvgStocks += float64(len(rows))
			longs := make([]observation, 0)
			var longReturns, restReturns []float64
			groups := map[string][]float64{}
			for _, row := range rows {
				phase := string(row.Phase)
				groups[phase] = append(groups[phase], row.residual)
				groups["all stocks"] = append(groups["all stocks"], row.residual)
				if row.LongSetup {
					longs = append(longs, row)
					longReturns = append(longReturns, row.residual)
				} else {
					restReturns = append(restReturns, row.residual)
				}
			}
			r.Coverage.AvgLongSetups += float64(len(longs))
			if len(longs) >= 5 {
				r.LongOnly.Series = append(r.LongOnly.Series, icPoint(longs, date, t, true))
			}
			if len(longReturns) > 0 && len(restReturns) > 0 {
				r.SpreadSeries = append(r.SpreadSeries, ValuePoint{date, t, mean(longReturns) - mean(restReturns)})
			}
			for phase, returns := range groups {
				phaseEvents[phase] = append(phaseEvents[phase], EventPoint{ValuePoint{date, t, mean(returns)}, len(returns)})
			}
			point := residual.Series[len(residual.Series)-1]
			if point.Spearman != nil {
				years[date[:4]] = append(years[date[:4]], ValuePoint{date, t, *point.Spearman})
			}
		}
		summarizeIC(&residual)
		summarizeIC(&raw)
		r.IC = append(r.IC, residual, raw)
	}
	summarizeIC(&r.LongOnly)
	r.Spread = summarizePoints(r.SpreadSeries, c.MainHorizon)
	for _, year := range sortedKeys(years) {
		r.Years = append(r.Years, YearResult{year, summarizePoints(years[year], c.MainHorizon)})
	}
	// Include zero-coverage phases explicitly, rather than hiding absent events.
	for _, phase := range []stockengine.CyclePhase{stockengine.PhaseReversalExtension, stockengine.PhaseWedgePop, stockengine.PhaseEMACrossback, stockengine.PhaseBaseNBreak, stockengine.PhaseBaseForming, stockengine.PhaseUptrend, stockengine.PhaseExhaustionExtension, stockengine.PhaseWedgeDrop, stockengine.PhaseDowntrend, stockengine.PhaseNeutral, stockengine.PhaseInsufficient} {
		if _, ok := phaseEvents[string(phase)]; !ok {
			phaseEvents[string(phase)] = nil
		}
	}
	phaseEvents["all stocks"] = append([]EventPoint(nil), phaseEvents["all stocks"]...)
	for _, phase := range sortedKeys(phaseEvents) {
		p := PhaseResult{Phase: phase, Series: phaseEvents[phase]}
		var values []ValuePoint
		for _, event := range p.Series {
			values = append(values, event.ValuePoint)
			p.AvgNames += float64(event.Names)
		}
		p.Summary = summarizePoints(values, c.MainHorizon)
		if len(values) > 0 {
			p.AvgNames /= float64(len(values))
		}
		r.Phases = append(r.Phases, p)
	}
	if r.Coverage.EvaluationDates > 0 {
		n := float64(r.Coverage.EvaluationDates)
		r.Coverage.AvgStocks /= n
		r.Coverage.AvgLongSetups /= n
	}
	r.Portfolio = buildPortfolio(signals, stocks, spy, c, lastPortfolio)
	return r, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func topLong(signals []Signal, n int) []Signal {
	ordered := append([]Signal(nil), signals...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.LongSetup != b.LongSetup {
			return a.LongSetup
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.RS63Pct != b.RS63Pct {
			return a.RS63Pct > b.RS63Pct
		}
		return a.Symbol < b.Symbol
	})
	var selected []Signal
	for _, s := range ordered {
		if s.LongSetup && len(selected) < n {
			selected = append(selected, s)
		}
	}
	return selected
}

// topBySignal ranks every stock with a valid signal (no LongSetup filter) by S
// descending, then RS63Pct descending, then Symbol ascending.
func topBySignal(signals []Signal, n int) []Signal {
	ordered := append([]Signal(nil), signals...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.S != b.S {
			return a.S > b.S
		}
		if a.RS63Pct != b.RS63Pct {
			return a.RS63Pct > b.RS63Pct
		}
		return a.Symbol < b.Symbol
	})
	if n < 0 {
		n = 0
	}
	return ordered[:min(n, len(ordered))]
}

// Equal-weight turnover = 1/2 sum(abs(new weight - old weight)), including
// cash. With unchanged basket size it is exactly the fraction replaced. The
// first trade starts from cash (turnover=1 when invested).
func turnover(previous, current []string) float64 {
	weights := map[string]float64{}
	if len(previous) == 0 {
		weights[""] = -1
	} else {
		for _, s := range previous {
			weights[s] -= 1 / float64(len(previous))
		}
	}
	if len(current) == 0 {
		weights[""] += 1
	} else {
		for _, s := range current {
			weights[s] += 1 / float64(len(current))
		}
	}
	var total float64
	for _, s := range sortedKeys(weights) {
		total += math.Abs(weights[s])
	}
	return total / 2
}

func buildPortfolio(signals [][]Signal, stocks map[string]history, spy history, c StudyConfig, last int) PortfolioResult {
	p := PortfolioResult{Skipped: make(map[string]string)}
	var previous []string
	var residual, excess, net []float64
	for t := warmupIndex; t <= last; t += c.Rebalance {
		date := spy.dates[t].Format(time.DateOnly)
		// Turnover compares consecutive scheduled baskets, even if an
		// intervening period's return is unobservable and excluded below.
		var selected []Signal
		if isKell(c.Signal) {
			selected = topLong(signals[t-warmupIndex], c.TopN)
		} else {
			selected = topBySignal(signals[t-warmupIndex], c.TopN)
		}
		period := PortfolioPeriod{Date: date, EntryDate: spy.dates[t+1].Format(time.DateOnly), ExitDate: spy.dates[t+1+c.Rebalance].Format(time.DateOnly)}
		for _, s := range selected {
			period.Symbols = append(period.Symbols, s.Symbol)
		}
		period.Turnover = turnover(previous, period.Symbols)
		previous = period.Symbols
		rows := observations(signals[t-warmupIndex], stocks, spy, t, c.Rebalance)
		if len(rows) < minNames {
			p.Skipped[date] = fmt.Sprintf("only %d names with signal, beta and holding-period returns; need 20", len(rows))
			continue
		}
		for _, row := range rows {
			period.UniverseRaw += row.raw / float64(len(rows))
		}
		// Rank BEFORE checking future availability. Do not promote a lower
		// ranked name because a selected stock has a missing future price.
		for _, s := range selected {
			raw, e, ok := forwardReturn(stocks[s.Symbol], spy, t, c.Rebalance, s.Beta)
			if !ok {
				period.Unpriced = append(period.Unpriced, s.Symbol)
				continue
			}
			period.Raw += raw / float64(len(selected))
			period.Residual += e / float64(len(selected))
		}
		if len(period.Unpriced) > 0 {
			p.Skipped[date] = "missing holding-period prices for selected names: " + fmt.Sprint(period.Unpriced)
			continue
		}
		period.Excess = period.Raw - period.UniverseRaw
		period.Cost = period.Turnover * 2 * c.CostBPS / 1e4
		period.NetExcess = period.Excess - period.Cost
		p.Periods = append(p.Periods, period)
		residual, excess, net = append(residual, period.Residual), append(excess, period.Excess), append(net, period.NetExcess)
		p.AvgNames += float64(len(period.Symbols))
		p.AvgTurnover += period.Turnover
	}
	p.Residual, p.Excess, p.NetExcess = summarizeReturns(residual, c.Rebalance), summarizeReturns(excess, c.Rebalance), summarizeReturns(net, c.Rebalance)
	if len(p.Periods) > 0 {
		p.AvgNames /= float64(len(p.Periods))
		p.AvgTurnover /= float64(len(p.Periods))
	}
	return p
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func sampleStd(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	var sum float64
	for _, v := range values {
		sum += (v - m) * (v - m)
	}
	return math.Sqrt(sum / float64(len(values)-1))
}

func pearson(x, y []float64) (float64, bool) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, false
	}
	mx, my := mean(x), mean(y)
	var xx, yy, xy float64
	for i := range x {
		xx += (x[i] - mx) * (x[i] - mx)
		yy += (y[i] - my) * (y[i] - my)
		xy += (x[i] - mx) * (y[i] - my)
	}
	if xx <= 0 || yy <= 0 {
		return 0, false
	}
	r := xy / math.Sqrt(xx*yy)
	return math.Max(-1, math.Min(1, r)), finite(r)
}

func averageRanks(values []float64) []float64 {
	order := make([]int, len(values))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return values[order[i]] < values[order[j]] })
	ranks := make([]float64, len(values))
	for start := 0; start < len(order); {
		end := start + 1
		for end < len(order) && values[order[end]] == values[order[start]] {
			end++
		}
		rank := float64(start+1+end) / 2
		for _, i := range order[start:end] {
			ranks[i] = rank
		}
		start = end
	}
	return ranks
}

func spearman(x, y []float64) (float64, bool) { return pearson(averageRanks(x), averageRanks(y)) }

func number(v float64) *float64 {
	if !finite(v) {
		return nil
	}
	return &v
}

func icPoint(rows []observation, date string, index int, residual bool) ICPoint {
	x, y := make([]float64, len(rows)), make([]float64, len(rows))
	for i, row := range rows {
		x[i], y[i] = row.S, row.raw
		if residual {
			y[i] = row.residual
		}
	}
	p := ICPoint{Date: date, Index: index, Names: len(rows)}
	if r, ok := spearman(x, y); ok {
		p.Spearman = number(r)
	}
	if r, ok := pearson(x, y); ok {
		p.Pearson = number(r)
	}
	return p
}

// Bartlett HAC, with n/(n-1) finite-sample correction, so lag=0 is exactly
// sampleStd/sqrt(n). Calendar indices preserve gaps in the eligible dates.
func nwSEAt(values []float64, indices []int, lag int) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}
	m := mean(values)
	var sum float64
	for _, v := range values {
		sum += (v - m) * (v - m)
	}
	for i := range values {
		for j := i - 1; j >= 0; j-- {
			distance := i - j
			if indices != nil {
				distance = indices[i] - indices[j]
			}
			if distance > lag {
				break
			}
			weight := 1 - float64(distance)/float64(lag+1)
			sum += 2 * weight * (values[i] - m) * (values[j] - m)
		}
	}
	return math.Sqrt(math.Max(0, sum) / float64(n*(n-1)))
}

func nwSE(values []float64, lag int) float64 { return nwSEAt(values, nil, lag) }

func summarizePoints(points []ValuePoint, horizon int) Summary {
	s := Summary{N: len(points)}
	if len(points) == 0 {
		return s
	}
	values, indices := make([]float64, len(points)), make([]int, len(points))
	var nonoverlap []float64
	for i, p := range points {
		values[i], indices[i] = p.Value, p.Index
		if p.Value > 0 {
			s.HitRate++
		}
		if (p.Index-warmupIndex)%horizon == 0 {
			nonoverlap = append(nonoverlap, p.Value)
		}
	}
	s.Mean, s.Std = mean(values), sampleStd(values)
	s.HitRate /= float64(s.N)
	if s.Std > 0 {
		s.ICIR = number(s.Mean / s.Std)
	}
	if s.N > 1 {
		s.NWSE = nwSEAt(values, indices, horizon-1)
		s.CILow, s.CIHigh = number(s.Mean-1.96*s.NWSE), number(s.Mean+1.96*s.NWSE)
		if s.NWSE > 0 {
			s.NWT = number(s.Mean / s.NWSE)
		}
	}
	s.NonOverlapN = len(nonoverlap)
	if len(nonoverlap) > 0 {
		s.NonOverlapMean = number(mean(nonoverlap))
	}
	if std := sampleStd(nonoverlap); std > 0 {
		s.NonOverlapT = number(mean(nonoverlap) / (std / math.Sqrt(float64(len(nonoverlap)))))
	}
	return s
}

func summarizeIC(ic *ICResult) {
	var rank, linear []ValuePoint
	for _, p := range ic.Series {
		if p.Spearman != nil {
			rank = append(rank, ValuePoint{p.Date, p.Index, *p.Spearman})
		}
		if p.Pearson != nil {
			linear = append(linear, ValuePoint{p.Date, p.Index, *p.Pearson})
		}
	}
	ic.Spearman, ic.Pearson = summarizePoints(rank, ic.Horizon), summarizePoints(linear, ic.Horizon)
}

func summarizeReturns(values []float64, rebalance int) ReturnSummary {
	s := ReturnSummary{N: len(values)}
	if len(values) == 0 {
		return s
	}
	periodsPerYear := 252 / float64(rebalance)
	m, std := mean(values), sampleStd(values)
	s.AnnualMean, s.AnnualVol = m*periodsPerYear, std*math.Sqrt(periodsPerYear)
	if std > 0 {
		s.Sharpe = number(m / std * math.Sqrt(periodsPerYear))
	}
	for _, v := range values {
		if v > 0 {
			s.HitRate++
		}
	}
	s.HitRate /= float64(len(values))
	return s
}
