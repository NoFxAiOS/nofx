package stockengine

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nofx/market/usstock"
)

func TestScanSortingAndSelection(t *testing.T) {
	universe := []usstock.SymbolInfo{
		{Symbol: "ABUSDT", Underlying: "A"},
		{Symbol: "SHORTBUSDT", Underlying: "SHORT"},
		{Symbol: "ERRORBUSDT", Underlying: "ERROR"},
		{Symbol: "ZBUSDT", Underlying: "Z"},
	}
	weak, strong := cycleTestPop(), cycleTestPop()
	weak[len(weak)-1].Volume = 100
	var calls []string
	fetch := func(ctx context.Context, symbol string) ([]usstock.Bar, string, error) {
		calls = append(calls, symbol)
		switch symbol {
		case "ABUSDT":
			return weak, usstock.SourceYahoo, nil
		case "ZBUSDT":
			return strong, usstock.SourceBStock, nil
		case "SHORTBUSDT":
			return strong[:59], usstock.SourceYahoo, nil
		default:
			return nil, "", errors.New("data source unavailable")
		}
	}
	now := time.Date(2026, 10, 14, 1, 0, 0, 0, time.UTC)
	report, err := Scan(context.Background(), universe, nil, fetch, now)
	if err != nil {
		t.Fatal(err)
	}
	if !report.GeneratedAt.Equal(now) || len(report.Results) != 2 || len(report.Skipped) != 2 || report.Results[0].Symbol != "ZBUSDT" || report.Results[1].Symbol != "ABUSDT" {
		t.Fatalf("report: %+v", report)
	}
	if !reflect.DeepEqual(calls, []string{"ABUSDT", "SHORTBUSDT", "ERRORBUSDT", "ZBUSDT"}) {
		t.Fatalf("fetch order: %v", calls)
	}
	if !strings.Contains(report.Skipped["SHORTBUSDT"], "59") || !strings.Contains(report.Skipped["ERRORBUSDT"], "unavailable") {
		t.Fatalf("skip reasons: %v", report.Skipped)
	}
	if got := report.TopLong(1, nil); len(got) != 1 || got[0].Symbol != "ZBUSDT" {
		t.Fatalf("top one: %+v", got)
	}
	if got := report.TopLong(10, map[string]bool{"ZBUSDT": true}); len(got) != 1 || got[0].Symbol != "ABUSDT" {
		t.Fatalf("exclude: %+v", got)
	}
	if got := report.TopLong(10, nil); len(got) != 2 {
		t.Fatalf("top bounded by available setups: %+v", got)
	}
	for _, n := range []int{0, -1} {
		if len(report.TopLong(n, nil)) != 0 {
			t.Fatalf("TopLong(%d) should be empty", n)
		}
	}
	if got, ok := report.Lookup("ABUSDT"); !ok || got.Source != usstock.SourceYahoo || got.Underlying != "A" {
		t.Fatalf("lookup: %+v %v", got, ok)
	}
	if _, ok := report.Lookup("SHORTBUSDT"); ok {
		t.Fatal("skipped symbol should not be found")
	}
	var empty *ScanReport
	if len(empty.TopLong(10, nil)) != 0 {
		t.Fatal("nil report should have no setups")
	}
	if _, ok := empty.Lookup("A"); ok {
		t.Fatal("nil report lookup should fail")
	}
}

func TestScanOrderTieBreaks(t *testing.T) {
	results := []ScanResult{
		{Symbol: "BEAR", Cycle: CycleAssessment{Score: 99, RS63Pct: 100}},
		{Symbol: "LOW_SCORE", Cycle: CycleAssessment{LongSetup: true, Score: 40, RS63Pct: 90}},
		{Symbol: "Z", Cycle: CycleAssessment{LongSetup: true, Score: 50, RS63Pct: 10}},
		{Symbol: "A", Cycle: CycleAssessment{LongSetup: true, Score: 50, RS63Pct: 10}},
		{Symbol: "STRONG_RS", Cycle: CycleAssessment{LongSetup: true, Score: 50, RS63Pct: 20}},
	}
	sortScanResults(results)
	var order []string
	for _, result := range results {
		order = append(order, result.Symbol)
	}
	if want := []string{"STRONG_RS", "A", "Z", "LOW_SCORE", "BEAR"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order=%v want=%v", order, want)
	}
	report := &ScanReport{Results: results}
	if got := report.TopLong(10, nil); len(got) != 4 {
		t.Fatalf("bearish result was selected: %+v", got)
	}
}

func TestScanCancellationReturnsSortedPartialReport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	universe := []usstock.SymbolInfo{{Symbol: "WEAK"}, {Symbol: "STRONG"}, {Symbol: "CANCEL"}, {Symbol: "UNREACHED"}}
	calls := 0
	fetch := func(ctx context.Context, symbol string) ([]usstock.Bar, string, error) {
		calls++
		if symbol == "CANCEL" {
			cancel()
			return nil, "", ctx.Err()
		}
		bars := cycleTestPop()
		if symbol == "WEAK" {
			bars[len(bars)-1].Volume = 100
		}
		return bars, usstock.SourceYahoo, nil
	}
	report, err := Scan(ctx, universe, nil, fetch, time.Now())
	if !errors.Is(err, context.Canceled) || calls != 3 || len(report.Results) != 2 || report.Results[0].Symbol != "STRONG" {
		t.Fatalf("partial report=%+v err=%v calls=%d", report, err, calls)
	}
	report, err = Scan(ctx, universe, nil, fetch, time.Now())
	if !errors.Is(err, context.Canceled) || calls != 3 || len(report.Results) != 0 {
		t.Fatalf("pre-canceled report=%+v err=%v calls=%d", report, err, calls)
	}
}

func TestScanAcceptsUsableInsufficientSeries(t *testing.T) {
	wrapped := fmt.Errorf("short history: %w", usstock.ErrInsufficientBars)
	report, err := Scan(context.Background(), []usstock.SymbolInfo{{Symbol: "USABLE"}, {Symbol: "SHORT"}}, nil,
		func(ctx context.Context, symbol string) ([]usstock.Bar, string, error) {
			if symbol == "SHORT" {
				return cycleTestPop()[:59], usstock.SourceYahoo, wrapped
			}
			return cycleTestPop()[:60], usstock.SourceYahoo, wrapped
		}, time.Now())
	if err != nil || len(report.Results) != 1 || report.Results[0].Cycle.Bars != 60 || len(report.Skipped) != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

type scanTestEnvironment struct {
	now      atomic.Int64
	lists    atomic.Int32
	fetches  atomic.Int32
	universe []usstock.SymbolInfo
}

// Cache tests are intentionally not parallel: the live seams are process-wide.
func setupScanTest(t *testing.T) *scanTestEnvironment {
	t.Helper()
	oldList, oldGet, oldNow := scanListSymbols, scanGetSeries, scanNow
	scanCache.Lock()
	if scanCache.flight != nil {
		scanCache.Unlock()
		t.Fatal("unexpected scan already running")
	}
	oldReport := scanCache.report
	scanCache.report = nil
	scanCache.Unlock()
	env := &scanTestEnvironment{universe: []usstock.SymbolInfo{{Symbol: "ABUSDT", Underlying: "A"}}}
	env.now.Store(time.Date(2026, 10, 14, 1, 0, 0, 0, time.UTC).UnixNano())
	scanNow = func() time.Time { return time.Unix(0, env.now.Load()).UTC() }
	scanListSymbols = func(ctx context.Context) ([]usstock.SymbolInfo, error) {
		env.lists.Add(1)
		return env.universe, nil
	}
	scanGetSeries = func(ctx context.Context, symbol, timeframe string, need int, allowYahoo bool) (*usstock.Series, error) {
		env.fetches.Add(1)
		if timeframe != usstock.TF1d || need != 260 || !allowYahoo {
			return nil, fmt.Errorf("incorrect live fetch arguments: %s %d %v", timeframe, need, allowYahoo)
		}
		return &usstock.Series{Bars: cycleTestPop(), Source: usstock.SourceYahoo}, fmt.Errorf("short: %w", usstock.ErrInsufficientBars)
	}
	t.Cleanup(func() {
		scanCache.Lock()
		flight := scanCache.flight
		scanCache.Unlock()
		if flight != nil {
			select {
			case <-flight.done:
			case <-time.After(5 * time.Second):
				t.Error("background scan did not finish")
				return
			}
		}
		scanListSymbols, scanGetSeries, scanNow = oldList, oldGet, oldNow
		scanCache.Lock()
		scanCache.report = oldReport
		scanCache.Unlock()
	})
	return env
}

func TestGetScanReportCacheAge(t *testing.T) {
	env := setupScanTest(t)
	if CachedScanReport() != nil {
		t.Fatal("new cache should be empty")
	}
	first, err := GetScanReport(context.Background(), time.Hour)
	if err != nil || len(first.Results) != 1 || env.lists.Load() != 1 || env.fetches.Load() != 1 {
		t.Fatalf("initial report=%+v err=%v lists=%d fetches=%d", first, err, env.lists.Load(), env.fetches.Load())
	}
	env.now.Add(int64(59 * time.Minute))
	second, err := GetScanReport(context.Background(), time.Hour)
	if err != nil || first != second || env.fetches.Load() != 1 || env.lists.Load() != 1 {
		t.Fatalf("cache was not reused: err=%v lists=%d fetches=%d", err, env.lists.Load(), env.fetches.Load())
	}
	env.now.Add(int64(time.Minute))
	third, err := GetScanReport(context.Background(), time.Hour)
	if err != nil || third == first || env.fetches.Load() != 2 || env.lists.Load() != 2 || CachedScanReport() != third {
		t.Fatalf("expired report was not refreshed: err=%v lists=%d fetches=%d", err, env.lists.Load(), env.fetches.Load())
	}
}

func TestGetScanReportConcurrentSingleFlight(t *testing.T) {
	env := setupScanTest(t)
	entered, release := make(chan struct{}), make(chan struct{})
	scanListSymbols = func(ctx context.Context) ([]usstock.SymbolInfo, error) {
		if env.lists.Add(1) == 1 {
			close(entered)
		}
		select {
		case <-release:
			return env.universe, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type outcome struct {
		report *ScanReport
		err    error
	}
	const callers = 16
	results := make(chan outcome, callers)
	go func() {
		report, err := GetScanReport(ctx, time.Hour)
		results <- outcome{report, err}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("scan did not start")
	}
	for j := 1; j < callers; j++ {
		go func() {
			report, err := GetScanReport(ctx, time.Hour)
			results <- outcome{report, err}
		}()
	}
	// Canceling a waiter must not cancel the initiating caller's shared scan.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer waitCancel()
	if _, err := GetScanReport(waitCtx, time.Hour); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter cancellation: %v", err)
	}
	select {
	case got := <-results:
		t.Fatalf("caller completed before scan was released: %+v", got)
	default:
	}
	close(release)
	var first *ScanReport
	for j := 0; j < callers; j++ {
		select {
		case got := <-results:
			if got.err != nil || got.report == nil {
				t.Fatalf("concurrent scan: %+v", got)
			}
			if first == nil {
				first = got.report
			} else if got.report != first {
				t.Fatal("callers received different reports")
			}
		case <-ctx.Done():
			t.Fatal("concurrent calls did not finish")
		}
	}
	if env.lists.Load() != 1 || env.fetches.Load() != 1 {
		t.Fatalf("multiple scans: lists=%d fetches=%d", env.lists.Load(), env.fetches.Load())
	}
}

func TestGetScanReportFailedRefreshKeepsOldReport(t *testing.T) {
	env := setupScanTest(t)
	old, err := GetScanReport(context.Background(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	env.now.Add(int64(2 * time.Hour))
	goodList := scanListSymbols
	failure := errors.New("registry unavailable")
	scanListSymbols = func(ctx context.Context) ([]usstock.SymbolInfo, error) {
		env.lists.Add(1)
		return nil, failure
	}
	got, err := GetScanReport(context.Background(), time.Hour)
	if !errors.Is(err, failure) || got != old || CachedScanReport() != old || env.fetches.Load() != 1 {
		t.Fatalf("failed refresh: report=%+v err=%v fetches=%d", got, err, env.fetches.Load())
	}
	scanListSymbols = goodList
	got, err = GetScanReport(context.Background(), time.Hour)
	if err != nil || got == old || env.fetches.Load() != 2 {
		t.Fatalf("retry: report=%+v err=%v fetches=%d", got, err, env.fetches.Load())
	}
}

func TestGetScanReportDataFailureKeepsOldReport(t *testing.T) {
	env := setupScanTest(t)
	old, err := GetScanReport(context.Background(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	env.now.Add(int64(2 * time.Hour))
	scanGetSeries = func(ctx context.Context, symbol, timeframe string, need int, allowYahoo bool) (*usstock.Series, error) {
		env.fetches.Add(1)
		return nil, errors.New("daily data source unavailable")
	}
	got, err := GetScanReport(context.Background(), time.Hour)
	if err == nil || got != old || CachedScanReport() != old {
		t.Fatalf("data outage replaced old report: report=%+v err=%v", got, err)
	}

	// Cancellation after a fetch also leaves the previously published report
	// intact, even though Scan can return a partial report to direct callers.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scanGetSeries = func(ctx context.Context, symbol, timeframe string, need int, allowYahoo bool) (*usstock.Series, error) {
		cancel()
		return &usstock.Series{Bars: cycleTestPop(), Source: usstock.SourceYahoo}, nil
	}
	got, err = GetScanReport(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) || got != old || CachedScanReport() != old {
		t.Fatalf("canceled scan replaced old report: report=%+v err=%v", got, err)
	}
}

func TestWarmScanReportSharesFlightAndDeadline(t *testing.T) {
	env := setupScanTest(t)
	deadline := make(chan time.Time, 1)
	release := make(chan struct{})
	scanListSymbols = func(ctx context.Context) ([]usstock.SymbolInfo, error) {
		env.lists.Add(1)
		end, _ := ctx.Deadline()
		deadline <- end
		select {
		case <-release:
			return env.universe, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	WarmScanReport(time.Hour)
	select {
	case end := <-deadline:
		if remaining := time.Until(end); remaining < 9*time.Minute || remaining > 10*time.Minute {
			t.Fatalf("warm timeout: %s", remaining)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("warm scan did not start")
	}
	scanCache.Lock()
	flight := scanCache.flight
	scanCache.Unlock()
	WarmScanReport(time.Hour)
	close(release)
	select {
	case <-flight.done:
	case <-time.After(5 * time.Second):
		t.Fatal("warm scan did not finish")
	}
	if CachedScanReport() == nil || env.lists.Load() != 1 || env.fetches.Load() != 1 {
		t.Fatalf("warm report: lists=%d fetches=%d", env.lists.Load(), env.fetches.Load())
	}
	WarmScanReport(time.Hour)
	if env.lists.Load() != 1 || env.fetches.Load() != 1 {
		t.Fatal("fresh warm call started another scan")
	}
	env.now.Add(int64(2 * time.Hour))
	WarmScanReport(time.Hour)
	if _, err := GetScanReport(context.Background(), time.Hour); err != nil {
		t.Fatal(err)
	}
	if env.lists.Load() != 2 || env.fetches.Load() != 2 {
		t.Fatalf("stale warm refresh: lists=%d fetches=%d", env.lists.Load(), env.fetches.Load())
	}
}

func TestGetScanReportOptionalBenchmark(t *testing.T) {
	for _, scenario := range []string{"not listed", "fails", "usable short series"} {
		t.Run(scenario, func(t *testing.T) {
			env := setupScanTest(t)
			if scenario != "not listed" {
				env.universe = append(env.universe, usstock.SymbolInfo{Symbol: "SPYBUSDT", Underlying: "SPY"})
			}
			stock := cycleTestPop()
			spy := cycleTestBars(cycleTestPath(len(stock), 200, -1), nil)
			scanGetSeries = func(ctx context.Context, symbol, timeframe string, need int, allowYahoo bool) (*usstock.Series, error) {
				env.fetches.Add(1)
				if timeframe != usstock.TF1d || need != 260 || !allowYahoo {
					return nil, errors.New("wrong daily fetch arguments")
				}
				if symbol == "SPYBUSDT" {
					if scenario == "fails" {
						return nil, errors.New("benchmark unavailable")
					}
					return &usstock.Series{Bars: spy, Source: usstock.SourceYahoo}, fmt.Errorf("SPY short: %w", usstock.ErrInsufficientBars)
				}
				return &usstock.Series{Bars: stock, Source: usstock.SourceBStock}, fmt.Errorf("stock short: %w", usstock.ErrInsufficientBars)
			}
			report, err := GetScanReport(context.Background(), time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := report.Lookup("ABUSDT")
			if !ok || got.Source != usstock.SourceBStock || got.Underlying != "A" {
				t.Fatalf("stock missing: %+v", report)
			}
			if scenario == "usable short series" {
				if got.Cycle.RS63Pct <= 0 || len(report.Results) != 2 {
					t.Fatalf("benchmark ignored: %+v", report)
				}
			} else if got.Cycle.RS63Pct != 0 {
				t.Fatalf("missing benchmark RS: %+v", got.Cycle)
			}
		})
	}
}

func TestScanFillsMomentum(t *testing.T) {
	universe := []usstock.SymbolInfo{{Symbol: "UPBUSDT", Underlying: "UP"}}
	bars := make([]usstock.Bar, 260)
	for i := range bars {
		bars[i] = usstock.Bar{OpenTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i), Open: 100 + float64(i), High: 101 + float64(i), Low: 99 + float64(i), Close: 100 + float64(i), Volume: 1000}
	}
	fetch := func(context.Context, string) ([]usstock.Bar, string, error) { return bars, usstock.SourceBStock, nil }
	report, err := Scan(context.Background(), universe, nil, fetch, time.Now())
	if err != nil || len(report.Results) != 1 {
		t.Fatalf("report %+v err %v", report, err)
	}
	if want := ClassifyMomentum(bars); !want.Valid || report.Results[0].Momentum != want {
		t.Fatalf("momentum %+v want %+v", report.Results[0].Momentum, want)
	}
}

func TestTopMomentum(t *testing.T) {
	mk := func(sym string, valid, above bool, mom, rs float64) ScanResult {
		return ScanResult{Symbol: sym, Cycle: CycleAssessment{RS63Pct: rs}, Momentum: MomentumAssessment{Valid: valid, AboveSMA200: above, Mom12_1Pct: mom}}
	}
	report := &ScanReport{Results: []ScanResult{
		mk("LOW", true, true, 10, 0),
		mk("BELOW", true, false, 300, 50),  // below SMA200: never a candidate
		mk("INVALID", false, true, 500, 9), // not enough history
		mk("TOP", true, true, 80, 1),
		mk("TIE_Z", true, true, 40, 5),
		mk("TIE_A", true, true, 40, 5),
		mk("TIE_RS", true, true, 40, 9),
		mk("EXCL", true, true, 99, 0),
	}}
	order := func(rs []ScanResult) (out []string) {
		for _, r := range rs {
			out = append(out, r.Symbol)
		}
		return
	}
	got := order(report.TopMomentum(10, map[string]bool{"EXCL": true}))
	if want := []string{"TOP", "TIE_RS", "TIE_A", "TIE_Z", "LOW"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order=%v want=%v", got, want)
	}
	if got := order(report.TopMomentum(2, nil)); !reflect.DeepEqual(got, []string{"EXCL", "TOP"}) {
		t.Fatalf("top 2: %v", got)
	}
	// Report order (Kell order) must be untouched.
	if report.Results[0].Symbol != "LOW" || report.Results[1].Symbol != "BELOW" {
		t.Fatalf("TopMomentum reordered the report: %v", order(report.Results))
	}
	if report.TopMomentum(0, nil) != nil || (*ScanReport)(nil).TopMomentum(3, nil) != nil {
		t.Fatal("n<=0 and nil report must return nil")
	}
}
