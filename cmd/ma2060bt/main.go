// ma2060bt compares mechanical momentum selection and 20/60 MA timing.
// Network and filesystem access are confined to this file; engine/rules are pure.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"nofx/market/usstock"
)

const defaultExclude = "TQQQBUSDT,SQQQBUSDT,SOXLBUSDT,SOXSBUSDT,UVXYBUSDT,MUUBUSDT,KORUBUSDT,SPYBUSDT,QQQBUSDT"

func main() {
	if err := cli(); err != nil {
		fmt.Fprintln(os.Stderr, "ma2060bt:", err)
		os.Exit(1)
	}
}

func cli() error {
	cache := flag.String("cache", filepath.Join(os.TempDir(), "cycleic-cache"), "cache directory, compatible with cycleic daily JSON; hourly data in 1h/")
	bars := flag.Int("bars", 1300, "daily bars requested per symbol (at least 254); hourly request is always 3000")
	tf := flag.String("tf", "1d", "execution timeframe: 1d or 1h; momentum always uses daily data")
	maxpos := flag.Int("maxpos", 10, "maximum active names; each active slot targets 1/maxpos equity")
	rebalance := flag.Int("rebalance", 10, "momentum rebalance interval in SPY trading days")
	cost := flag.Float64("cost-bps", 10, "one-way transaction cost in basis points")
	exclude := flag.String("exclude", defaultExclude, "comma-separated exclusions (replaces defaults); SPY and QQQ always benchmark-only")
	out := flag.String("out", "", "markdown output path (default stdout)")
	jsonOut := flag.String("json", "", "optional JSON including equity curves, decisions, and closed lots")
	flag.Parse()
	c := Config{*tf, *maxpos, *rebalance, *cost}
	if err := c.validate(); err != nil {
		return err
	}
	if flag.NArg() != 0 || *bars < MomentumLookback+2 || strings.TrimSpace(*cache) == "" {
		return fmt.Errorf("no positional arguments allowed; bars must be >=254; cache must not be empty")
	}
	if *out != "" && *jsonOut != "" {
		a, err := filepath.Abs(*out)
		if err != nil {
			return err
		}
		b, err := filepath.Abs(*jsonOut)
		if err != nil {
			return err
		}
		if a == b {
			return fmt.Errorf("out and json must be different paths")
		}
	}
	if err := os.MkdirAll(*cache, 0755); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	progress := log.New(os.Stderr, "ma2060bt: ", log.Ltime)
	registry, universeWarning, err := loadUniverse(ctx, *cache, usstock.ListSymbols)
	if err != nil {
		return err
	}
	symbols, excluded := selectUniverse(registry, *exclude)
	warnings, sources, skipped := make(map[string]string), make(map[string]string), make(map[string]string)
	if universeWarning != "" {
		warnings["universe"] = universeWarning
	}
	input, daily := make(map[string][]usstock.Bar), make(map[string][]usstock.Bar)
	var spyBars, spyDaily []usstock.Bar
	all := append([]string{"SPYBUSDT"}, symbols...)
	for i, s := range all {
		if err := ctx.Err(); err != nil {
			return err
		}
		d, _, warning, err := loadSeries(ctx, s, usstock.TF1d, *bars, *cache, time.Now(), usstock.GetSeries)
		if warning != "" {
			warnings[s+"/1d"] = warning
		}
		if err != nil {
			if s == "SPYBUSDT" {
				return fmt.Errorf("SPY daily: %w", err)
			}
			// MA strategies and EW can use a name whose daily rank is missing.
			if c.Timeframe == usstock.TF1d {
				skipped[s] = err.Error()
				continue
			}
			warnings[s+"/1d"] = "daily momentum unavailable: " + err.Error()
		} else {
			daily[s] = d.Bars
			sources[s+"/1d"] = d.Source
		}
		var execution *usstock.Series
		if c.Timeframe == usstock.TF1h {
			execution, _, warning, err = loadSeries(ctx, s, usstock.TF1h, 3000, *cache, time.Now(), usstock.GetSeries)
			if warning != "" {
				warnings[s+"/1h"] = warning
			}
		} else {
			execution = d
		}
		if err != nil {
			if s == "SPYBUSDT" {
				return fmt.Errorf("SPY execution: %w", err)
			}
			skipped[s] = err.Error()
			continue
		}
		sources[s+"/"+c.Timeframe] = execution.Source
		if s == "SPYBUSDT" {
			spyBars, spyDaily = execution.Bars, d.Bars
		} else {
			input[s] = execution.Bars
		}
		if (i+1)%10 == 0 || i+1 == len(all) {
			progress.Printf("loaded %d/%d symbols (%d accepted, %d skipped)", i+1, len(all), len(input), len(skipped))
		}
	}
	progress.Printf("running six portfolios, timeframe=%s, one-way cost=%.2f bps", c.Timeframe, c.CostBPS)
	r, err := runBacktest(input, daily, spyBars, spyDaily, c)
	if err != nil {
		return err
	}
	r.Coverage.UniverseSize = len(symbols)
	r.Coverage.Warnings, r.Coverage.Sources, r.Coverage.Excluded = warnings, sources, excluded
	for s, reason := range skipped {
		r.Coverage.Skipped[s] = reason
	}
	if *jsonOut != "" {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		if err := atomicWrite(*jsonOut, append(data, '\n')); err != nil {
			return err
		}
	}
	report := renderReport(r)
	if *out != "" {
		return atomicWrite(*out, []byte(report))
	}
	_, err = io.WriteString(os.Stdout, report)
	return err
}

func selectUniverse(registry []usstock.SymbolInfo, exclude string) ([]string, []string) {
	drop := map[string]bool{"SPYBUSDT": true, "QQQBUSDT": true}
	for _, s := range strings.Split(exclude, ",") {
		if s = strings.ToUpper(strings.TrimSpace(s)); s != "" {
			drop[s] = true
		}
	}
	keep := make(map[string]bool)
	for _, info := range registry {
		s := strings.ToUpper(info.Symbol)
		if !drop[s] && strings.HasSuffix(s, "BUSDT") && !strings.ContainsAny(s, "/\\") {
			keep[s] = true
		}
	}
	return sortedKeys(keep), sortedKeys(drop)
}

// A persisted registry makes subsequent runs completely cache-only. Existing
// cycleic caches have no registry: discover their daily Series files and label
// this potentially incomplete universe explicitly, without attempting a network
// request. With an empty cache, fetch and persist the current listed registry.
func loadUniverse(ctx context.Context, dir string, fetch func(context.Context) ([]usstock.SymbolInfo, error)) ([]usstock.SymbolInfo, string, error) {
	path := filepath.Join(dir, "symbols.json")
	if data, err := os.ReadFile(path); err == nil {
		var symbols []usstock.SymbolInfo
		if json.Unmarshal(data, &symbols) == nil && len(symbols) > 0 {
			return symbols, "universe from cached symbols.json; listings may have changed since it was captured", nil
		}
	}
	files, err := filepath.Glob(filepath.Join(dir, "*BUSDT.json"))
	if err != nil {
		return nil, "", err
	}
	var symbols []usstock.SymbolInfo
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var e cachedSeries
		if json.Unmarshal(data, &e) == nil && e.Series != nil && e.Series.Timeframe == usstock.TF1d && filepath.Base(path) == e.Series.Symbol+".json" {
			symbols = append(symbols, usstock.SymbolInfo{Symbol: e.Series.Symbol, Underlying: e.Series.Underlying})
		}
	}
	if len(symbols) > 0 {
		return symbols, "universe inferred from daily cache filenames; cached subset may omit currently listed bStock names (supply symbols.json for a complete registry)", nil
	}
	symbols, err = fetch(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("list bStock universe (or supply cached Series / symbols.json): %w", err)
	}
	if len(symbols) == 0 {
		return nil, "", fmt.Errorf("empty bStock universe")
	}
	data, err := json.MarshalIndent(symbols, "", "  ")
	if err != nil {
		return nil, "", err
	}
	if err := atomicWrite(path, append(data, '\n')); err != nil {
		return symbols, "registry cache write failed: " + err.Error(), nil
	}
	return symbols, "", nil
}

type cachedSeries struct {
	RequestedBars int             `json:"requested_bars"`
	Series        *usstock.Series `json:"series"`
	Warning       string          `json:"warning,omitempty"`
}

type seriesFetcher func(context.Context, string, string, int, bool) (*usstock.Series, error)

// Reuse valid caches regardless of age so historical research works offline.
// Stale/short-cache use is disclosed. Request size (not returned size) controls
// reuse of Yahoo's short histories, compatible with cycleic's cachedSeries.
func loadSeries(ctx context.Context, symbol, tf string, need int, dir string, now time.Time, fetch seriesFetcher) (*usstock.Series, bool, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, "", err
	}
	if symbol == "" || strings.ContainsAny(symbol, "/\\") || symbol == "." || symbol == ".." || need < 2 || (tf != usstock.TF1d && tf != usstock.TF1h) {
		return nil, false, "", fmt.Errorf("invalid symbol, timeframe or bar request")
	}
	if tf == usstock.TF1h {
		dir = filepath.Join(dir, "1h")
	}
	path := filepath.Join(dir, symbol+".json")
	var cached *usstock.Series
	var cacheWarning string
	if data, err := os.ReadFile(path); err == nil {
		var e cachedSeries
		if json.Unmarshal(data, &e) == nil && e.Series != nil && e.Series.Symbol == symbol && e.Series.Timeframe == tf {
			cached, _ = cleanSeries(e.Series, need)
			cacheWarning = e.Warning
			if info, err := os.Stat(path); err == nil && now.Sub(info.ModTime()) >= 24*time.Hour {
				cacheWarning = joinWarning(cacheWarning, "using cache older than 24h (cache-first research mode)")
			}
			if cached != nil && (e.RequestedBars >= need || len(cached.Bars) >= need) {
				return cached, true, cacheWarning, nil
			}
		}
	}
	s, fetchErr := fetch(ctx, symbol, tf, need, true)
	if err := ctx.Err(); err != nil {
		return nil, false, "", err
	}
	var warning string
	if fetchErr != nil && !errors.Is(fetchErr, usstock.ErrInsufficientBars) {
		if cached != nil {
			return cached, true, joinWarning(cacheWarning, "using shorter cache after fetch failed: "+fetchErr.Error()), nil
		}
		return nil, false, "", fetchErr
	}
	if fetchErr != nil {
		warning = fetchErr.Error()
	}
	if s == nil || s.Symbol != symbol || s.Timeframe != tf {
		return nil, false, "", fmt.Errorf("returned series symbol/timeframe mismatch for %s/%s", symbol, tf)
	}
	s, err := cleanSeries(s, need)
	if err != nil {
		if cached != nil {
			return cached, true, joinWarning(cacheWarning, "using cache after invalid fetch: "+err.Error()), nil
		}
		return nil, false, "", err
	}
	data, err := json.Marshal(cachedSeries{need, s, warning})
	if err == nil {
		err = os.MkdirAll(dir, 0755)
	}
	if err == nil {
		err = atomicWrite(path, append(data, '\n'))
	}
	if err != nil {
		warning = joinWarning(warning, "cache write failed: "+err.Error())
	}
	return s, false, warning, nil
}

func cleanSeries(s *usstock.Series, need int) (*usstock.Series, error) {
	if s == nil {
		return nil, fmt.Errorf("no series")
	}
	h := makeHistory(s.Bars, s.Timeframe)
	if len(h.bars) < 2 {
		return nil, fmt.Errorf("have %d valid %s bars; need at least 2", len(h.bars), s.Timeframe)
	}
	out := *s
	out.Bars = h.bars[max(0, len(h.bars)-need):]
	return &out, nil
}

func joinWarning(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".ma2060bt-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func pct(v float64) string { return fmt.Sprintf("%.2f%%", 100*v) }
func pctPtr(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return pct(*v)
}
func decimal(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", *v)
}
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}
func stamp(t time.Time, tf string) string {
	if tf == usstock.TF1h {
		return t.UTC().Format("2006-01-02 15:04Z")
	}
	return t.Format(time.DateOnly)
}

func renderReport(r *Backtest) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	c, cov := r.Config, r.Coverage
	w("# 20/60 MA vs momentum portfolio backtest\n\n")
	w("Timeframe: %s; maxpos: %d; momentum rebalance: %d trading days; cost: %.2f bps per side; initial equity: %.2f.\n\n", c.Timeframe, c.MaxPos, c.Rebalance, c.CostBPS, initialEquity)
	w("## 1. Strategy comparison\n\n")
	w("| Strategy | CAGR | Ann. vol | Sharpe | Sortino | Max DD | DD peak | DD trough | Calmar | Exposure | Closed trades* | Win rate | Avg win | Avg loss | Profit factor | Avg hold days | Costs | Last 12m |\n")
	w("|---|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, s := range r.Strategies {
		m := s.Metrics
		last12 := pct(m.Last12Months)
		if m.Last12Partial {
			last12 += " (partial)"
		}
		w("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %d | %s | %s | %s | %s | %s | %.2f | %s |\n", s.Name, pctPtr(m.CAGR), pct(m.Vol), decimal(m.Sharpe), decimal(m.Sortino), pct(m.MaxDrawdown), stamp(m.Peak, c.Timeframe), stamp(m.Trough, c.Timeframe), decimal(m.Calmar), pct(m.Exposure), m.Trades, pctPtr(m.WinRate), pctPtr(m.AvgWin), pctPtr(m.AvgLoss), decimal(m.ProfitFactor), decimal(m.AvgHolding), m.Costs, last12)
	}
	w("\nCAGR uses elapsed calendar time (365.25 days/year). Vol/Sharpe use sample bar-return standard deviation, 252 daily sessions/year; hourly annualization uses observed bars/session ×252. Sortino uses RMS of negative returns over all bars, rf=0. Exposure averages post-fill close invested weights (excluding the initial cash baseline). Ratios with zero denominators are n/a. Last 12m uses the last mark on/before the calendar anniversary, or available history when partial.\n\n")
	w("*Trades are closed FIFO lot portions, including partial rebalance reductions; win/loss returns include allocated costs, profit factor uses net monetary P&L, holding uses calendar days. Open lots are excluded from trade statistics, remain marked in equity, and incur no invented terminal sale cost.\n\n")
	w("## 2. Calendar-year returns\n\n| Year |")
	years := make(map[string]bool)
	for _, s := range r.Strategies {
		w(" %s |", s.Name)
		for y := range s.Metrics.Years {
			years[y] = true
		}
	}
	w("\n|---|")
	for range r.Strategies {
		w("---:|")
	}
	w("\n")
	for _, y := range sortedKeys(years) {
		w("| %s |", y)
		for _, s := range r.Strategies {
			if v, ok := s.Metrics.Years[y]; ok {
				w(" %s |", pct(v))
			} else {
				w(" n/a |")
			}
		}
		w("\n")
	}
	w("\nFirst/last calendar years may be partial; returns chain from the previous year's final equity (or initial cash baseline).\n\n")
	w("## 3. MA2060 rule breakdown\n\n| Entry | Closed trades | Win rate | Avg net return | Avg hold days |\n|---|---:|---:|---:|---:|\n")
	for _, s := range r.Strategies {
		if s.Name != "MA2060_RULES" {
			continue
		}
		for _, e := range []string{"E1", "E2", "E3", "E4"} {
			v := s.Entries[e]
			w("| %s | %d | %s | %s | %s |\n", e, v.Trades, pctPtr(v.WinRate), pctPtr(v.AvgReturn), decimal(v.AvgHolding))
		}
		w("\n| Exit | Filled sell orders |\n|---|---:|\n")
		for _, x := range []string{"X1", "X2", "X3", "X4", "TARGET"} {
			w("| %s | %d |\n", x, s.Exits[x])
		}
	}
	w("\nRule interpretation: SMA20/60, Wilder ATR14 seeded with the first 14 true ranges; first TR=high−low. E1–E4 priority assigns overlapping entries; X1–X4 priority then E4's SMA20 TARGET assigns exits. Windows of 3/5/10/20 bars include today; previous-five lows and previous-20 closing highs exclude today. New highs are strict closing highs; ties do not reset the clock. E4's key bar is close > previous high OR a bullish body engulfing the previous bearish body. X4 uses actual entry open minus 2×the last ATR known at entry (signal-close ATR). All MA/ATR windows are execution bars, including in 1h mode.\n\n")
	w("## 4. Coverage\n\nUniverse after exclusions: %d; loaded: %d; skipped: %d; SPY bars including warmup: %d. Evaluation: %s through %s, %d marks (first is initial cash at signal close). Warmup requires 253 completed daily SPY bars and 61 execution bars; all strategies share this start.\n\n", cov.UniverseSize, len(cov.Symbols), len(cov.Skipped), cov.SPYBars, stamp(cov.First, c.Timeframe), stamp(cov.Last, c.Timeframe), cov.Bars)
	w("Excluded (SPY/QQQ benchmark-only): %s.\n\n", strings.Join(cov.Excluded, ", "))
	w("| Symbol | Execution bars | Daily bars | Evaluation matches | First | Last |\n|---|---:|---:|---:|---|---|\n")
	for _, s := range sortedKeys(cov.Symbols) {
		v := cov.Symbols[s]
		w("| %s | %d | %d | %d | %s | %s |\n", cell(s), v.Bars, v.DailyBars, v.Matched, stamp(v.First, c.Timeframe), stamp(v.Last, c.Timeframe))
	}
	w("\n| Strategy | Open names at end | Missing-open orders |\n|---|---:|---:|\n")
	for _, s := range r.Strategies {
		w("| %s | %d | %d |\n", s.Name, s.OpenPositions, s.Unfilled)
	}
	for _, block := range []struct {
		label string
		rows  map[string]string
	}{{"Skipped", cov.Skipped}, {"Data sources", cov.Sources}, {"Load warnings", cov.Warnings}} {
		if len(block.rows) > 0 {
			w("\n%s:\n\n| Key | Detail |\n|---|---|\n", block.label)
			for _, s := range sortedKeys(block.rows) {
				w("| %s | %s |\n", cell(s), cell(block.rows[s]))
			}
		}
	}
	w("\n## 5. Execution and caveats\n\n")
	w("- Survivorship bias: universe is currently listed bStock names, or the explicitly labelled cached registry/subset; delisted names and historical listing changes are unavailable. Missing/short-history ranks remain unavailable, not backfilled.\n")
	w("- Yahoo daily adjusted OHLC can differ from bStock prices, dividends, splits, sessions and liquidity; hourly Yahoo data may have different adjustment treatment. Sources are disclosed above. No LLM layer is backtested: MOM_TOP is the mechanical close>SMA200, 12−1 momentum proxy (close[t−21]/close[t−252]−1).\n")
	w("- Signals use close t; daily fills use next SPY date's open, hourly fills use next SPY bar's open. Missing opens expire that order, with no deferred execution or replacement; missing close marks hold the last price. Entry events missed for lack of slots are not queued. A held exit that fails to fill needs a subsequent rule signal. All timestamps are UTC bar labels (open timestamps), while decisions/marks represent those bars' closes. Exact hourly timestamp alignment is required across data sources.\n")
	w("- Active slots target 1/maxpos equity at entry; fewer names leave cash at 0%%. Buys are cash-capped after costs, fractional shares allowed, no leverage. Momentum rebalances solve equal post-cost target weights, selling dropouts and reweighting retained names. Missing holdings can constrain weights/slots. SPY_BH invests all cash in SPY; EW_BH is the all-name benchmark exception to maxpos, fixes names present at the initial signal close and rebalances every 63 execution bars.\n")
	w("- MA_CROSS enters only on an observed golden cross after the common start (no initial trend position). MOM_X_MA selects the momentum top N first, then vetoes names without SMA20>SMA60; no backfill, death-cross exits between rebalances, cash stays idle until the next rebalance.\n")
	w("- 1h mode requests 3000 bars and accepts shorter histories; Yahoo caps 60m history at roughly 730 days (~2 years), and 3000 requested bars can cover less. Hourly momentum uses prior-day daily bars only, including at the final hourly close. Momentum rebalances on every %d-th SPY trading day's last bar, starting at the common signal date. Costs exclude spread/slippage beyond the configured bps, taxes, funding and lot-size constraints.\n", c.Rebalance)
	return b.String()
}
