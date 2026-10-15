package stockengine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"nofx/logger"
	"nofx/market/usstock"
)

const (
	scanDailyBars       = 260
	scanWarmTimeout     = 10 * time.Minute
	scanBenchmarkSymbol = "SPYBUSDT"
)

// scanFundamentalsBudget caps the best-effort earnings/sector enrichment that
// follows the daily-bar classification (a variable so tests can shrink it).
var scanFundamentalsBudget = 90 * time.Second

// Network and clock seams for offline tests.
var (
	scanListSymbols = usstock.ListSymbols
	scanGetSeries   = usstock.GetSeries
	scanNow         = time.Now

	scanGetFundamentals = usstock.GetFundamentals
)

type ScanResult struct {
	Symbol     string             `json:"symbol"`
	Underlying string             `json:"underlying"`
	Source     string             `json:"source"`
	Cycle      CycleAssessment    `json:"cycle"`
	Momentum   MomentumAssessment `json:"momentum"`
	// Universe is the row of the prompt's universe table (zero Price when the
	// symbol had too few bars for one).
	Universe UniverseRow `json:"universe"`
}

type ScanReport struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Results     []ScanResult      `json:"results"`
	Skipped     map[string]string `json:"skipped,omitempty"`
}

// TopLong returns up to n results with Cycle.LongSetup, in report order,
// skipping symbols in exclude.
func (r *ScanReport) TopLong(n int, exclude map[string]bool) []ScanResult {
	if r == nil || n <= 0 {
		return nil
	}
	out := make([]ScanResult, 0, min(n, len(r.Results)))
	for _, result := range r.Results {
		if result.Cycle.LongSetup && !exclude[result.Symbol] {
			out = append(out, result)
			if len(out) == n {
				break
			}
		}
	}
	return out
}

// TopMomentum returns up to n results ranked by 12-1 momentum among names
// above their 200-day SMA (Valid && AboveSMA200 only — long-only, so names
// below the SMA200 are never candidates), skipping exclude. Order: Mom12_1Pct
// desc, Cycle.RS63Pct desc, Symbol asc.
func (r *ScanReport) TopMomentum(n int, exclude map[string]bool) []ScanResult {
	if r == nil || n <= 0 {
		return nil
	}
	picks := make([]ScanResult, 0, len(r.Results))
	for _, result := range r.Results {
		if result.Momentum.Valid && result.Momentum.AboveSMA200 && !exclude[result.Symbol] {
			picks = append(picks, result)
		}
	}
	sort.Slice(picks, func(i, j int) bool {
		a, b := picks[i], picks[j]
		if a.Momentum.Mom12_1Pct != b.Momentum.Mom12_1Pct {
			return a.Momentum.Mom12_1Pct > b.Momentum.Mom12_1Pct
		}
		if a.Cycle.RS63Pct != b.Cycle.RS63Pct {
			return a.Cycle.RS63Pct > b.Cycle.RS63Pct
		}
		return a.Symbol < b.Symbol
	})
	if len(picks) > n {
		picks = picks[:n]
	}
	return picks
}

// Lookup returns the result for symbol.
func (r *ScanReport) Lookup(symbol string) (ScanResult, bool) {
	if r != nil {
		for _, result := range r.Results {
			if result.Symbol == symbol {
				return result, true
			}
		}
	}
	return ScanResult{}, false
}

// Scan classifies every symbol of the universe sequentially. Cancellation
// returns the sorted partial report together with ctx.Err(). Individual data
// failures are recorded in Skipped and do not stop the remaining universe.
func Scan(ctx context.Context, universe []usstock.SymbolInfo, spyBars []usstock.Bar,
	fetch func(ctx context.Context, symbol string) ([]usstock.Bar, string, error), now time.Time) (*ScanReport, error) {
	report := &ScanReport{GeneratedAt: now, Results: make([]ScanResult, 0, len(universe)), Skipped: make(map[string]string)}
	defer func() { sortScanResults(report.Results) }()
	for _, info := range universe {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		bars, source, err := fetch(ctx, info.Symbol)
		if cancelErr := ctx.Err(); cancelErr != nil {
			return report, cancelErr
		}
		if err != nil && !(errors.Is(err, usstock.ErrInsufficientBars) && len(bars) >= cycleMinBars) {
			report.Skipped[info.Symbol] = err.Error()
			continue
		}
		if len(bars) < cycleMinBars {
			report.Skipped[info.Symbol] = fmt.Sprintf("insufficient daily bars: have %d, need at least %d", len(bars), cycleMinBars)
			continue
		}
		row, ok := BuildUniverseRow(info.Symbol, info.Underlying, bars)
		if !ok {
			row = UniverseRow{Symbol: info.Symbol, Underlying: info.Underlying, DaysToEarnings: -1}
		}
		report.Results = append(report.Results, ScanResult{
			Symbol: info.Symbol, Underlying: info.Underlying, Source: source,
			Cycle: ClassifyCycle(bars, spyBars), Momentum: ClassifyMomentum(bars),
			Universe: row,
		})
	}
	return report, ctx.Err()
}

func sortScanResults(results []ScanResult) {
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Cycle.LongSetup != b.Cycle.LongSetup {
			return a.Cycle.LongSetup
		}
		if a.Cycle.Score != b.Cycle.Score {
			return a.Cycle.Score > b.Cycle.Score
		}
		if a.Cycle.RS63Pct != b.Cycle.RS63Pct {
			return a.Cycle.RS63Pct > b.Cycle.RS63Pct
		}
		return a.Symbol < b.Symbol
	})
}

type scanFlight struct {
	done   chan struct{}
	report *ScanReport
	err    error
}

var scanCache struct {
	sync.Mutex
	report *ScanReport
	flight *scanFlight
}

func scanFresh(report *ScanReport, maxAge time.Duration) bool {
	return report != nil && maxAge > 0 && scanNow().Sub(report.GeneratedAt) < maxAge
}

// GetScanReport returns the cached report if younger than maxAge; otherwise
// callers share one sequential live scan. A waiter may cancel independently.
// A failed refresh returns the previous report (if any) along with the error.
// Published reports are read-only and are never mutated by the cache.
func GetScanReport(ctx context.Context, maxAge time.Duration) (*ScanReport, error) {
	if err := ctx.Err(); err != nil {
		return CachedScanReport(), err
	}
	scanCache.Lock()
	if scanFresh(scanCache.report, maxAge) {
		report := scanCache.report
		scanCache.Unlock()
		return report, nil
	}
	if flight := scanCache.flight; flight != nil {
		scanCache.Unlock()
		select {
		case <-ctx.Done():
			return CachedScanReport(), ctx.Err()
		case <-flight.done:
			return flight.report, flight.err
		}
	}
	flight := &scanFlight{done: make(chan struct{})}
	scanCache.flight = flight
	scanCache.Unlock()
	refreshScan(ctx, flight)
	return flight.report, flight.err
}

// CachedScanReport returns the last successful, read-only report without IO.
func CachedScanReport() *ScanReport {
	scanCache.Lock()
	defer scanCache.Unlock()
	return scanCache.report
}

// WarmScanReport starts a background refresh if stale and no scan is running.
func WarmScanReport(maxAge time.Duration) {
	scanCache.Lock()
	if scanFresh(scanCache.report, maxAge) || scanCache.flight != nil {
		scanCache.Unlock()
		return
	}
	flight := &scanFlight{done: make(chan struct{})}
	scanCache.flight = flight
	scanCache.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), scanWarmTimeout)
		defer cancel()
		refreshScan(ctx, flight)
	}()
}

func refreshScan(ctx context.Context, flight *scanFlight) {
	report, err := liveScan(ctx)
	scanCache.Lock()
	defer scanCache.Unlock()
	if err == nil {
		scanCache.report = report
	}
	flight.report, flight.err = scanCache.report, err
	scanCache.flight = nil
	close(flight.done)
}

func liveScan(ctx context.Context) (report *ScanReport, err error) {
	started := time.Now()
	defer func() {
		classified, longs, momentum, skipped := 0, 0, 0, 0
		if report != nil {
			classified, skipped = len(report.Results), len(report.Skipped)
			for _, result := range report.Results {
				if result.Cycle.LongSetup {
					longs++
				}
				if result.Momentum.Valid && result.Momentum.AboveSMA200 {
					momentum++
				}
			}
		}
		logger.Infof("🔭 bStock cycle scan: %d classified, %d long setups, %d momentum candidates, %d skipped in %s", classified, longs, momentum, skipped, time.Since(started).Round(time.Millisecond))
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	universe, err := scanListSymbols(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var spyBars []usstock.Bar
	for _, info := range universe {
		if info.Symbol == scanBenchmarkSymbol {
			bars, _, fetchErr := fetchScanBars(ctx, scanBenchmarkSymbol)
			if fetchErr == nil {
				spyBars = bars
			}
			break
		}
	}
	report, err = Scan(ctx, universe, spyBars, fetchScanBars, scanNow())
	if err == nil && len(universe) > 0 && len(report.Results) == 0 {
		// A universe-wide data outage must not replace a useful cached report
		// with an empty one. Partial scans with usable symbols still succeed.
		err = errors.New("bStock cycle scan: no symbols had usable daily bars")
	}
	if err == nil {
		enrichFundamentals(ctx, report)
	}
	return report, err
}

// enrichFundamentals fills Class / Sector / DaysToEarnings of every result's
// universe row from Yahoo, sequentially (Yahoo is serialized anyway), within
// scanFundamentalsBudget. It is best effort: failures, the budget running out
// or ctx being canceled just leave the remaining rows unknown, never failing
// the scan. Results arrive in report order, so the best-ranked names are
// enriched first.
func enrichFundamentals(ctx context.Context, report *ScanReport) {
	if report == nil {
		return
	}
	fctx, cancel := context.WithTimeout(ctx, scanFundamentalsBudget)
	defer cancel()
	enriched, failed := 0, 0
	for i := range report.Results {
		if fctx.Err() != nil {
			break
		}
		r := &report.Results[i]
		f, err := scanGetFundamentals(fctx, r.Underlying)
		if err != nil {
			failed++
			continue
		}
		ApplyFundamentals(&r.Universe, f, report.GeneratedAt)
		enriched++
	}
	logger.Infof("🔭 bStock scan fundamentals: %d enriched, %d unavailable of %d", enriched, failed, len(report.Results))
}

func fetchScanBars(ctx context.Context, symbol string) ([]usstock.Bar, string, error) {
	series, err := scanGetSeries(ctx, symbol, usstock.TF1d, scanDailyBars, true)
	if series == nil {
		if err == nil {
			err = errors.New("daily series is unavailable")
		}
		return nil, "", err
	}
	if errors.Is(err, usstock.ErrInsufficientBars) && len(series.Bars) >= cycleMinBars {
		err = nil
	}
	return series.Bars, series.Source, err
}
