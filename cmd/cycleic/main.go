// cycleic measures whether the production Kell cycle score predicts raw and
// market-adjusted forward returns. Network access is confined to this file.
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
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/market/usstock"
)

const defaultExclude = "TQQQBUSDT,SQQQBUSDT,SOXLBUSDT,SOXSBUSDT,UVXYBUSDT,QQQBUSDT"

func main() {
	if err := cli(); err != nil {
		fmt.Fprintln(os.Stderr, "cycleic:", err)
		os.Exit(1)
	}
}

func cli() error {
	bars := flag.Int("bars", 1300, "daily bars requested per symbol (at least 300)")
	horizons := flag.String("horizons", "1,5,10,20", "comma-separated forward horizons in SPY trading dates")
	mainH := flag.Int("main-horizon", 10, "primary horizon; automatically included in horizons")
	topN := flag.Int("topn", 10, "maximum number of long setups in the equal-weight portfolio")
	rebalance := flag.Int("rebalance", 10, "rebalance interval and holding period in SPY dates")
	cost := flag.Float64("cost-bps", 10, "one-way cost in basis points; period cost = turnover * 2 * cost-bps / 10000")
	exclude := flag.String("exclude", defaultExclude, "comma-separated symbols excluded from the cross-section; SPY is always excluded")
	cache := flag.String("cache", filepath.Join(os.TempDir(), "cycleic-cache"), "JSON bar cache directory; fresh for 24 hours")
	out := flag.String("out", "", "markdown report path (default stdout)")
	jsonOut := flag.String("json", "", "optional JSON path for statistics and daily/period series, keyed by signal")
	signals := flag.String("signal", "kell", "ranking signal(s), comma-separated: "+strings.Join(signalNames, ", ")+"; one study and report section per signal")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flag.Args())
	}
	if *bars < windowBars {
		return fmt.Errorf("bars must be at least 300")
	}
	if *cache == "" {
		return fmt.Errorf("cache directory must not be empty")
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
	hs, err := parseHorizons(*horizons)
	if err != nil {
		return err
	}
	names, err := parseSignals(*signals)
	if err != nil {
		return err
	}
	configs := make([]StudyConfig, 0, len(names))
	for _, name := range names {
		c, err := (StudyConfig{hs, *mainH, *topN, *rebalance, *cost, name}).normalized()
		if err != nil {
			return err
		}
		configs = append(configs, c)
	}
	if err := os.MkdirAll(*cache, 0755); err != nil {
		return fmt.Errorf("create cache: %w", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	progress := log.New(os.Stderr, "cycleic: ", log.Ltime)
	registry, err := usstock.ListSymbols(ctx)
	if err != nil {
		return fmt.Errorf("list symbols: %w", err)
	}
	symbols, excluded := selectUniverse(registry, *exclude)
	progress.Printf("universe: %d names; loading SPY benchmark", len(symbols))
	spy, cached, warning, err := loadSeries(ctx, "SPYBUSDT", *bars, *cache, time.Now(), usstock.GetSeries)
	if err != nil {
		return fmt.Errorf("benchmark: %w", err)
	}
	progress.Printf("SPY: %d bars, source=%s, cache=%t", len(spy.Bars), spy.Source, cached)
	input := make(map[string][]usstock.Bar)
	sources := map[string]string{"SPYBUSDT": spy.Source}
	warnings := make(map[string]string)
	if warning != "" {
		warnings["SPYBUSDT"] = warning
	}
	skipped := make(map[string]string)
	for i, symbol := range symbols {
		if err := ctx.Err(); err != nil {
			return err
		}
		series, _, warning, err := loadSeries(ctx, symbol, *bars, *cache, time.Now(), usstock.GetSeries)
		if err != nil {
			skipped[symbol] = err.Error()
		} else {
			input[symbol], sources[symbol] = series.Bars, series.Source
			if warning != "" {
				warnings[symbol] = warning
			}
		}
		if (i+1)%10 == 0 || i+1 == len(symbols) {
			progress.Printf("loaded %d/%d symbols (%d accepted, %d skipped)", i+1, len(symbols), len(input), len(skipped))
		}
	}
	results := make([]*StudyResult, 0, len(configs))
	for _, c := range configs {
		progress.Printf("computing point-in-time signals and statistics for signal %s", c.Signal)
		started := time.Now()
		r, err := runStudy(input, spy.Bars, c)
		if err != nil {
			return err
		}
		r.Coverage.UniverseSize = len(symbols)
		for symbol, reason := range skipped {
			r.Coverage.Skipped[symbol] = reason
		}
		r.Coverage.Sources, r.Coverage.Warnings, r.Coverage.Excluded = sources, warnings, excluded
		progress.Printf("study %s complete in %s: %d evaluation dates, %d portfolio periods", c.Signal, time.Since(started).Round(time.Millisecond), r.Coverage.EvaluationDates, len(r.Portfolio.Periods))
		results = append(results, r)
	}
	if *jsonOut != "" {
		keyed := make(map[string]*StudyResult, len(results))
		for _, r := range results {
			keyed[r.Config.Signal] = r
		}
		data, err := json.MarshalIndent(keyed, "", "  ")
		if err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
		if err := atomicWrite(*jsonOut, append(data, '\n')); err != nil {
			return fmt.Errorf("write JSON: %w", err)
		}
	}
	markdown := renderAll(results)
	if *out == "" {
		_, err = io.WriteString(os.Stdout, markdown)
	} else {
		err = atomicWrite(*out, []byte(markdown))
	}
	return err
}

// parseSignals splits a comma-separated signal list, rejecting unknown names
// and duplicates before any data is loaded.
func parseSignals(value string) ([]string, error) {
	var names []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if !validSignal(name) {
			return nil, fmt.Errorf("unknown signal %q; use one of %s", part, strings.Join(signalNames, ", "))
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate signal %q", name)
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, nil
}

func parseHorizons(value string) ([]int, error) {
	var hs []int
	for _, part := range strings.Split(value, ",") {
		h, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || h <= 0 {
			return nil, fmt.Errorf("invalid horizon %q; use positive integers separated by commas", part)
		}
		hs = append(hs, h)
	}
	return hs, nil
}

func selectUniverse(registry []usstock.SymbolInfo, exclude string) (symbols, excluded []string) {
	drop := map[string]bool{"SPYBUSDT": true}
	for _, part := range strings.Split(exclude, ",") {
		if s := strings.ToUpper(strings.TrimSpace(part)); s != "" {
			drop[s] = true
		}
	}
	seen := make(map[string]bool)
	for _, info := range registry {
		if seen[info.Symbol] {
			continue
		}
		seen[info.Symbol] = true
		if drop[info.Symbol] {
			excluded = append(excluded, info.Symbol)
		} else {
			symbols = append(symbols, info.Symbol)
		}
	}
	sort.Strings(symbols)
	sort.Strings(excluded)
	return symbols, excluded
}

type cachedSeries struct {
	RequestedBars int             `json:"requested_bars"`
	Series        *usstock.Series `json:"series"`
	Warning       string          `json:"warning,omitempty"`
}

type seriesFetcher func(context.Context, string, string, int, bool) (*usstock.Series, error)

// The requested bar count is stored so a fresh 300-bar cache does not prevent
// a subsequent 1300-bar study from fetching a longer history. Short Yahoo
// histories requested with the same need remain reusable for 24 hours.
func loadSeries(ctx context.Context, symbol string, need int, dir string, now time.Time, fetch seriesFetcher) (*usstock.Series, bool, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, "", err
	}
	if symbol == "" || strings.ContainsAny(symbol, "/\\") || symbol == "." || symbol == ".." {
		return nil, false, "", fmt.Errorf("invalid symbol %q", symbol)
	}
	path := filepath.Join(dir, symbol+".json")
	if info, err := os.Stat(path); err == nil && now.Sub(info.ModTime()) >= 0 && now.Sub(info.ModTime()) < 24*time.Hour {
		if data, err := os.ReadFile(path); err == nil {
			var entry cachedSeries
			if json.Unmarshal(data, &entry) == nil && entry.Series != nil && entry.Series.Symbol == symbol && entry.Series.Timeframe == usstock.TF1d && (entry.RequestedBars >= need || len(entry.Series.Bars) >= need) {
				if series, err := cleanSeries(entry.Series, need); err == nil {
					return series, true, entry.Warning, nil
				}
			}
		}
	}
	series, err := fetch(ctx, symbol, usstock.TF1d, need, true)
	if cancelErr := ctx.Err(); cancelErr != nil {
		return nil, false, "", cancelErr
	}
	if err != nil && !(errors.Is(err, usstock.ErrInsufficientBars) && series != nil && len(series.Bars) >= windowBars) {
		return nil, false, "", err
	}
	var warning string
	if err != nil {
		warning = err.Error()
	}
	series, err = cleanSeries(series, need)
	if err != nil {
		return nil, false, "", err
	}
	data, err := json.Marshal(cachedSeries{need, series, warning})
	if err == nil {
		err = atomicWrite(path, append(data, '\n'))
	}
	if err != nil {
		if warning != "" {
			warning += "; "
		}
		warning += "cache write failed: " + err.Error()
	}
	return series, false, warning, nil
}

func cleanSeries(series *usstock.Series, need int) (*usstock.Series, error) {
	if series == nil {
		return nil, fmt.Errorf("no series returned")
	}
	h := makeHistory(series.Bars)
	if len(h.bars) < windowBars {
		return nil, fmt.Errorf("have %d valid daily dates; need at least 300", len(h.bars))
	}
	out := *series
	out.Bars = h.bars[max(0, len(h.bars)-need):]
	return &out, nil
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".cycleic-*")
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

func decimal(v *float64, digits int) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.*f", digits, *v)
}

func percent(v float64) string { return fmt.Sprintf("%.2f%%", 100*v) }

func available(n int, v float64, digits int) string {
	if n == 0 {
		return "n/a"
	}
	return decimal(number(v), digits)
}

func availablePct(n int, v float64) string {
	if n == 0 {
		return "n/a"
	}
	return percent(v)
}

func tableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

func renderReport(r *StudyResult) string {
	var b strings.Builder
	write := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	var main Summary
	for _, ic := range r.IC {
		if ic.Horizon == r.Config.MainHorizon && ic.Target == "residual" {
			main = ic.Spearman
		}
	}
	// Exactly six lines, including the label, precede the numbered sections.
	write("**Key results**  \n")
	write("Main %dd residual Spearman: mean IC **%s**; ICIR **%s**.  \n", r.Config.MainHorizon, available(main.N, main.Mean, 4), decimal(main.ICIR, 2))
	write("Main residual IC: NW t **%s**; hit rate **%s** (%d dates).  \n", decimal(main.NWT, 2), availablePct(main.N, main.HitRate), main.N)
	write("Long-setup-only IC: **%s**; NW t **%s** (%d dates).  \n", available(r.LongOnly.Spearman.N, r.LongOnly.Spearman.Mean, 4), decimal(r.LongOnly.Spearman.NWT, 2), r.LongOnly.Spearman.N)
	write("Top-%d annualized residual: **%s**; Sharpe-like **%s**.  \n", r.Config.TopN, availablePct(r.Portfolio.Residual.N, r.Portfolio.Residual.AnnualMean), decimal(r.Portfolio.Residual.Sharpe, 2))
	write("LongSetup vs rest residual spread: **%s** per %dd; NW t **%s**.\n\n", availablePct(r.Spread.N, r.Spread.Mean), r.Config.MainHorizon, decimal(r.Spread.NWT, 2))
	signal := r.Config.Signal
	if signal == "" {
		signal = "kell"
	}
	write("# Cycle IC study: signal `%s`\n\n", signal)
	write("Signal `%s`: %s History: last 300 bars, at least 260. Entry: signal date + 1 close; exit: entry + horizon close. Beta: OLS on 126 daily SPY dates ending at signal, at least 60 matched return pairs. Cross-sections need 20 names. Main horizon: %d; rebalance/hold: %d; top N: %d; one-way cost: %.2f bps.\n\n", signal, signalDescription(signal), r.Config.MainHorizon, r.Config.Rebalance, r.Config.TopN, r.Config.CostBPS)
	write("## 1–2. IC series and summaries\n\n")
	write("| Horizon | Target | Method | Dates | Mean IC | Std | ICIR | NW t | 95%% CI | Hit rate | Non-overlap n | Non-overlap mean | Plain t |\n")
	write("|---:|---|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|\n")
	for _, ic := range r.IC {
		for _, method := range []struct {
			name string
			s    Summary
		}{{"Spearman", ic.Spearman}, {"Pearson", ic.Pearson}} {
			s := method.s
			write("| %d | %s | %s | %d | %s | %s | %s | %s | [%s, %s] | %s | %d | %s | %s |\n", ic.Horizon, ic.Target, method.name, s.N, available(s.N, s.Mean, 4), available(s.N, s.Std, 4), decimal(s.ICIR, 2), decimal(s.NWT, 2), decimal(s.CILow, 4), decimal(s.CIHigh, 4), availablePct(s.N, s.HitRate), s.NonOverlapN, decimal(s.NonOverlapMean, 4), decimal(s.NonOverlapT, 2))
		}
	}
	write("\nConstant signal/target dates have undefined correlation and are excluded from the corresponding summary. NW uses Bartlett weights with lag h−1 and an n/(n−1) correction; at lag zero it equals the plain sample standard error. Missing dates retain their SPY calendar spacing. The non-overlapping check uses every h-th SPY date from warmup, even when intermediate dates are missing. Daily IC series are included in the optional JSON.\n\n")
	write("## 3. IC by calendar year\n\nMain-horizon residual Spearman.\n\n| Year | Dates | Mean IC | ICIR |\n|---|---:|---:|---:|\n")
	for _, y := range r.Years {
		write("| %s | %d | %s | %s |\n", y.Year, y.Summary.N, available(y.Summary.N, y.Summary.Mean, 4), decimal(y.Summary.ICIR, 2))
	}
	write("\n## 4. Long-setup-only IC\n\nMain-horizon residual; at least five long setups per date.\n\n| Method | Dates | Mean IC | Std | ICIR | NW t | 95%% CI | Hit rate | Non-overlap n | Non-overlap mean | Plain t |\n|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|\n")
	for _, m := range []struct {
		name string
		s    Summary
	}{{"Spearman", r.LongOnly.Spearman}, {"Pearson", r.LongOnly.Pearson}} {
		s := m.s
		write("| %s | %d | %s | %s | %s | %s | [%s, %s] | %s | %d | %s | %s |\n", m.name, s.N, available(s.N, s.Mean, 4), available(s.N, s.Std, 4), decimal(s.ICIR, 2), decimal(s.NWT, 2), decimal(s.CILow, 4), decimal(s.CIHigh, 4), availablePct(s.N, s.HitRate), s.NonOverlapN, decimal(s.NonOverlapMean, 4), decimal(s.NonOverlapT, 2))
	}
	write("\n## 5. Phase event study\n\nMain-horizon residual returns; each date has equal weight.\n\n| Phase | Mean return | NW t | Dates | Avg names/date |\n|---|---:|---:|---:|---:|\n")
	for _, p := range r.Phases {
		write("| %s | %s | %s | %d | %.1f |\n", p.Phase, availablePct(p.Summary.N, p.Summary.Mean), decimal(p.Summary.NWT, 2), p.Summary.N, p.AvgNames)
	}
	write("\nThe all-stocks row is the residual baseline. Beta adjustment does not force its cross-sectional mean to zero.\n\n")
	write("## 6. LongSetup vs rest spread\n\n| Dates | Mean residual spread | NW t |\n|---:|---:|---:|\n| %d | %s | %s |\n\n", r.Spread.N, availablePct(r.Spread.N, r.Spread.Mean), decimal(r.Spread.NWT, 2))
	write("## 7. Top-N portfolio\n\n| Return series | Periods | Annualized mean | Annualized vol | Sharpe-like | Hit rate |\n|---|---:|---:|---:|---:|---:|\n")
	for _, m := range []struct {
		name string
		s    ReturnSummary
	}{{"Residual", r.Portfolio.Residual}, {"Excess vs EW universe", r.Portfolio.Excess}, {"Excess net of costs", r.Portfolio.NetExcess}} {
		s := m.s
		write("| %s | %d | %s | %s | %s | %s |\n", m.name, s.N, availablePct(s.N, s.AnnualMean), availablePct(s.N, s.AnnualVol), decimal(s.Sharpe, 2), availablePct(s.N, s.HitRate))
	}
	write("\nAverage names held: %.2f; average one-way turnover: %s; skipped periods: %d.\n\n", r.Portfolio.AvgNames, percent(r.Portfolio.AvgTurnover), len(r.Portfolio.Skipped))
	if isKell(signal) {
		write("Rank: LongSetup first, Score descending, RS63Pct descending, Symbol ascending; buy only LongSetup names, equal-weight fewer than N if necessary, cash if none. ")
	} else {
		write("Rank: signal value descending, RS63Pct descending, Symbol ascending, among all stocks with a valid signal (no LongSetup filter); equal-weight the top N. ")
	}
	write("Rank before checking future prices; a missing selected entry/exit skips the period without substituting another stock. EW universe uses stocks with valid signal, beta and holding-period returns. One-way turnover is half the absolute change in equal weights including cash (fraction replaced for fixed-size baskets); it compares consecutive scheduled baskets, including skipped periods, and charges the initial purchase from cash. Net excess subtracts turnover × 2 × cost-bps / 10000. Annualization is arithmetic mean × 252/rebalance and sample std × sqrt(252/rebalance).\n\n")
	write("| Signal date | Entry | Exit | Names | Portfolio raw | Residual | EW universe raw | Excess | Turnover | Net excess |\n|---|---|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, p := range r.Portfolio.Periods {
		write("| %s | %s | %s | %d | %s | %s | %s | %s | %s | %s |\n", p.Date, p.EntryDate, p.ExitDate, len(p.Symbols), percent(p.Raw), percent(p.Residual), percent(p.UniverseRaw), percent(p.Excess), percent(p.Turnover), percent(p.NetExcess))
	}
	if len(r.Portfolio.Skipped) > 0 {
		write("\n| Skipped signal date | Reason |\n|---|---|\n")
		for _, date := range sortedKeys(r.Portfolio.Skipped) {
			write("| %s | %s |\n", date, tableCell(r.Portfolio.Skipped[date]))
		}
	}
	write("\n## 8. Coverage\n\n| Measure | Value |\n|---|---|\n")
	cov := r.Coverage
	write("| Universe after exclusions | %d |\n| Symbols loaded / skipped | %d / %d |\n| SPY dates | %d |\n| Candidate / valid main-horizon dates | %d / %d |\n| First / last valid evaluation date | %s / %s |\n| Average stocks/date | %.2f |\n| Average long setups/date | %.2f |\n", cov.UniverseSize, len(cov.Loaded), len(cov.Skipped), cov.BenchmarkBars, cov.CandidateDates, cov.EvaluationDates, cov.FirstDate, cov.LastDate, cov.AvgStocks, cov.AvgLongSetups)
	write("\nLoaded: %s.\n\nExcluded: %s. SPY is the benchmark and always excluded from the cross-section.\n", strings.Join(cov.Loaded, ", "), strings.Join(cov.Excluded, ", "))
	if len(cov.Skipped) > 0 {
		write("\n| Skipped symbol | Reason |\n|---|---|\n")
		for _, symbol := range sortedKeys(cov.Skipped) {
			write("| %s | %s |\n", tableCell(symbol), tableCell(cov.Skipped[symbol]))
		}
	}
	if len(cov.Sources) > 0 || len(cov.Warnings) > 0 {
		write("\n| Loaded symbol | Source | Warning |\n|---|---|---|\n")
		for _, symbol := range sortedKeys(cov.Sources) {
			write("| %s | %s | %s |\n", tableCell(symbol), tableCell(cov.Sources[symbol]), tableCell(cov.Warnings[symbol]))
		}
	}
	if len(cov.RemovedBars) > 0 {
		write("\n| Symbol | Invalid/duplicate bars removed |\n|---|---:|\n")
		for _, symbol := range sortedKeys(cov.RemovedBars) {
			write("| %s | %d |\n", tableCell(symbol), cov.RemovedBars[symbol])
		}
	}
	write("\n## Caveats\n\n")
	write("- Survivorship/selection bias: the universe is currently listed bStock names.\n- Yahoo adjusted data can differ from bStock prices.\n- ETFs remain in the universe after the explicit exclusions.\n- Overlapping forward returns are handled by Newey–West HAC; the non-overlapping check is also shown.\n- Single-period beta is estimated at the signal date and held fixed for the forward horizon; residuals are not regression alpha.\n- Missing future prices can select the evaluable sample; unpriced selected portfolios are reported as skipped periods.\n")
	return b.String()
}

func signalDescription(name string) string {
	switch name {
	case "rs63":
		return "63-day relative strength versus SPY in percentage points (`RS63Pct` from `ClassifyCycle`)."
	case "ret63":
		return "63-day own return, close[t] / close[t−63] − 1."
	case "mom12_1":
		return "12-1 momentum, close[t−21] / close[t−252] − 1 (skips the latest month); needs 253 bars."
	case "trend_rs":
		return "tiered trend rank: close > EMA20 > SMA50 gets `RS63Pct` + 1000, otherwise `RS63Pct` − 1000, so all trend names outrank all non-trend names."
	case "trend_mom":
		return "tiered: close > EMA20 > SMA50 gets 12-1 momentum + 1000, otherwise 12-1 momentum − 1000 (trend names first, momentum within each tier)."
	case "sma200_mom":
		return "tiered: close > SMA200 gets 12-1 momentum + 1000, otherwise 12-1 momentum − 1000."
	case "ext10":
		return "10-day extension in ATR units (`Ext10ATR` from `ClassifyCycle`: close minus the fast EMA, divided by ATR)."
	}
	return "production `ClassifyCycle` score for LongSetup, otherwise zero."
}

// renderAll renders one report per signal. With several signals a comparison
// table comes first, then one full section per signal.
func renderAll(results []*StudyResult) string {
	if len(results) == 1 {
		return renderReport(results[0])
	}
	var b strings.Builder
	b.WriteString(renderComparison(results))
	for _, r := range results {
		b.WriteString("\n---\n\n")
		b.WriteString(renderReport(r))
	}
	return b.String()
}

func icSummary(r *StudyResult, horizon int) Summary {
	for _, ic := range r.IC {
		if ic.Horizon == horizon && ic.Target == "residual" {
			return ic.Spearman
		}
	}
	return Summary{}
}

func renderComparison(results []*StudyResult) string {
	var b strings.Builder
	write := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	first := results[0].Config
	write("# Signal comparison\n\n")
	write("One row per ranking signal; main horizon %dd, Top-%d rebalanced every %dd, one-way cost %.2f bps. IC columns are residual Spearman. The phase event study and the LongSetup spread are Kell-specific and identical across signals; only the IC and Top-N portfolio results depend on the signal. Top-N for `kell` takes LongSetup names only (production order); for every other signal it takes the N highest values among all stocks with a valid signal.\n\n", first.MainHorizon, first.TopN, first.Rebalance, first.CostBPS)
	write("| Signal | Main IC | ICIR | NW t | Hit rate | Non-overlap t | 5d IC | 5d NW t | 20d IC | 20d NW t | Top-N resid. ann. | Sharpe-like | Excess ann. (gross) | Excess ann. (net) | Avg turnover |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, r := range results {
		m, h5, h20 := icSummary(r, r.Config.MainHorizon), icSummary(r, 5), icSummary(r, 20)
		p := r.Portfolio
		write("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n", r.Config.Signal,
			available(m.N, m.Mean, 4), decimal(m.ICIR, 2), decimal(m.NWT, 2), availablePct(m.N, m.HitRate), decimal(m.NonOverlapT, 2),
			available(h5.N, h5.Mean, 4), decimal(h5.NWT, 2), available(h20.N, h20.Mean, 4), decimal(h20.NWT, 2),
			availablePct(p.Residual.N, p.Residual.AnnualMean), decimal(p.Residual.Sharpe, 2),
			availablePct(p.Excess.N, p.Excess.AnnualMean), availablePct(p.NetExcess.N, p.NetExcess.AnnualMean), availablePct(len(p.Periods), p.AvgTurnover))
	}
	b.WriteString("\n")
	return b.String()
}
