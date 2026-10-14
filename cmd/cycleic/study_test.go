package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"nofx/kernel/stockengine"
	"nofx/market/usstock"
)

func closeTo(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if !finite(got) || math.Abs(got-want) > tolerance {
		t.Fatalf("got %.12g, want %.12g (tolerance %g)", got, want, tolerance)
	}
}

func TestCorrelations(t *testing.T) {
	// Average ranks: [1.5,1.5,3,4] and [1,2.5,2.5,4]. Centered
	// cross product = 3.75; both sums of squares = 4.5 => rho = 5/6.
	r, ok := spearman([]float64{0, 0, 1, 2}, []float64{10, 20, 20, 30})
	if !ok {
		t.Fatal("Spearman unexpectedly undefined")
	}
	closeTo(t, r, 5.0/6, 1e-12)
	for _, tc := range []struct {
		y    []float64
		want float64
	}{
		{[]float64{3, 5, 7}, 1}, {[]float64{7, 5, 3}, -1}, {[]float64{1, 0, 1}, 0},
	} {
		r, ok := pearson([]float64{1, 2, 3}, tc.y)
		if !ok {
			t.Fatal("Pearson unexpectedly undefined")
		}
		closeTo(t, r, tc.want, 1e-12)
	}
	if _, ok := spearman([]float64{0, 0, 0}, []float64{1, 2, 3}); ok {
		t.Fatal("constant scores must produce missing IC")
	}
	if _, ok := pearson([]float64{1}, []float64{2, 3}); ok {
		t.Fatal("mismatched inputs must produce missing IC")
	}
}

func TestNeweyWest(t *testing.T) {
	values := make([]float64, 100)
	for i := range values {
		values[i] = float64(i + 1)
	}
	plain := sampleStd(values) / math.Sqrt(float64(len(values)))
	closeTo(t, nwSE(values, 0), plain, 1e-12)
	if nwSE(values, 5) <= plain {
		t.Fatalf("positive autocorrelation should increase HAC SE: HAC=%g plain=%g", nwSE(values, 5), plain)
	}
	// A hand calculation for lag one: demeaned [-1,0,1], gamma
	// cross-products zero, so the finite-sample corrected SE is sqrt(1/3).
	closeTo(t, nwSE([]float64{1, 2, 3}, 1), math.Sqrt(1.0/3), 1e-12)
	indices := make([]int, len(values))
	for i := range indices {
		indices[i] = i * 10
	}
	closeTo(t, nwSEAt(values, indices, 5), plain, 1e-12)
	points := []ValuePoint{{Index: 259, Value: 1}, {Index: 260, Value: 2}, {Index: 264, Value: 3}, {Index: 269, Value: 4}}
	s := summarizePoints(points, 5)
	if s.NonOverlapN != 3 {
		t.Fatalf("non-overlap must follow calendar, not compressed observations: %+v", s)
	}
	closeTo(t, *s.NonOverlapMean, 8.0/3, 1e-12)
	closeTo(t, *s.CILow, s.Mean-1.96*s.NWSE, 1e-12)
	closeTo(t, *s.CIHigh, s.Mean+1.96*s.NWSE, 1e-12)
}

func testBar(day int, price, volume float64) usstock.Bar {
	start := time.Date(2020, 1, 6, 14, 30, 0, 0, time.UTC)
	// Only weekdays, so year grouping and SPY calendar indexing differ
	// from UTC calendar-day arithmetic.
	date := start.AddDate(0, 0, (day/5)*7+day%5)
	return usstock.Bar{OpenTime: date, Open: price, High: price * 1.015, Low: price * 0.985, Close: price, Volume: volume}
}

func syntheticData(names, days int) (map[string][]usstock.Bar, []usstock.Bar) {
	spy := make([]usstock.Bar, days)
	price := 200.0
	for d := range spy {
		price *= 1 + 0.0002 + 0.003*math.Sin(float64(d)*0.37) + 0.001*math.Cos(float64(d)*0.11)
		spy[d] = testBar(d, price, 1000)
	}
	stocks := make(map[string][]usstock.Bar)
	for i := 0; i < names; i++ {
		bars := make([]usstock.Bar, days)
		price := 70 + float64(i)*3
		for d := range bars {
			market := 0.0
			if d > 0 {
				market = spy[d].Close/spy[d-1].Close - 1
			}
			// Different wave phases and drifts create both long and non-long
			// cycle states, with variation in relative strength and score.
			price *= 1 + (0.7+float64(i%7)*0.1)*market + 0.004*math.Sin(float64(d+i*7)*0.15) + float64(i%5-2)*0.00008
			volume := 100.0
			if (d+i)%17 == 0 {
				volume = 180
			}
			bars[d] = testBar(d, price, volume)
			bars[d].OpenTime = bars[d].OpenTime.Add(-14*time.Hour - 30*time.Minute)
		}
		stocks[fmt.Sprintf("S%02dBUSDT", i)] = bars
	}
	return stocks, spy
}

func TestSignalNoLookahead(t *testing.T) {
	stocks, spyBars := syntheticData(1, 700)
	bars := stocks["S00BUSDT"]
	spy := makeHistory(spyBars)
	const at = 420
	before, ok := signalAt("S00BUSDT", makeHistory(bars), spy, at)
	if !ok {
		t.Fatal("signal unavailable")
	}
	mutated := append([]usstock.Bar(nil), bars...)
	changedSPY := append([]usstock.Bar(nil), spyBars...)
	for j := at + 1; j < len(mutated); j++ {
		mutated[j].Open *= 8
		mutated[j].High *= 8
		mutated[j].Low *= 8
		mutated[j].Close *= 8
		mutated[j].Volume *= 100
		changedSPY[j].Open *= 4
		changedSPY[j].High *= 4
		changedSPY[j].Low *= 4
		changedSPY[j].Close *= 4
	}
	after, ok := signalAt("S00BUSDT", makeHistory(mutated), makeHistory(changedSPY), at)
	if !ok || !reflect.DeepEqual(before, after) {
		t.Fatalf("future prices changed signal: before=%+v after=%+v", before, after)
	}
	assessment := stockengine.ClassifyCycle(bars[at+1-windowBars:at+1], spyBars[at+1-windowBars:at+1])
	if before.Phase != assessment.Phase || before.LongSetup != assessment.LongSetup || before.Score != assessment.Score || before.RS63Pct != assessment.RS63Pct {
		t.Fatalf("must call production classifier on last 300 bars: signal=%+v classifier=%+v", before, assessment)
	}
	if !before.LongSetup && before.S != 0 {
		t.Fatal("non-long signal must be zero")
	}
	missing := append(append([]usstock.Bar(nil), bars[:at]...), bars[at+1:]...)
	if _, ok := signalAt("S00BUSDT", makeHistory(missing), spy, at); ok {
		t.Fatal("no participation without a bar on signal date")
	}
}

func TestForwardSignalLagAndMissingDates(t *testing.T) {
	var bars, spyBars []usstock.Bar
	for i := 0; i < 10; i++ {
		bars = append(bars, testBar(i, 100, 100))
		spyBars = append(spyBars, testBar(i, 200, 100))
	}
	bars[3], bars[4], bars[9] = testBar(3, 50, 100), testBar(4, 110, 100), testBar(9, 121, 100)
	spyBars[4], spyBars[9] = testBar(4, 200, 100), testBar(9, 202, 100)
	raw, residual, ok := forwardReturn(makeHistory(bars), makeHistory(spyBars), 3, 5, 2)
	if !ok {
		t.Fatal("forward return unavailable")
	}
	closeTo(t, raw, 0.1, 1e-12)
	closeTo(t, residual, 0.08, 1e-12)
	for _, missing := range []int{4, 9} {
		short := append(append([]usstock.Bar(nil), bars[:missing]...), bars[missing+1:]...)
		if _, _, ok := forwardReturn(makeHistory(short), makeHistory(spyBars), 3, 5, 2); ok {
			t.Fatalf("missing exact entry/exit date %d must not shift to another session", missing)
		}
	}
	if _, _, ok := forwardReturn(makeHistory(bars), makeHistory(spyBars), 4, 5, 2); ok {
		t.Fatal("out-of-range exit must be missing")
	}
}

func TestBetaAndResidual(t *testing.T) {
	var stockBars, spyBars []usstock.Bar
	stockPrice, spyPrice := 100.0, 200.0
	for d := 0; d < 700; d++ {
		market := 0.003*math.Sin(float64(d)*0.37) + 0.001*math.Cos(float64(d)*0.11)
		noise := 0.0001
		if d%2 == 0 {
			noise = -noise
		}
		stockPrice *= 1 + 2*market + noise
		spyPrice *= 1 + market
		stockBars = append(stockBars, testBar(d, stockPrice, 100))
		spyBars = append(spyBars, testBar(d, spyPrice, 100))
	}
	stock, spy := makeHistory(stockBars), makeHistory(spyBars)
	var betas, residuals []float64
	for at := warmupIndex; at < 698; at++ {
		beta, ok := betaAt(stock, spy, at)
		if !ok {
			t.Fatal("synthetic beta unavailable")
		}
		closeTo(t, beta, 2, 0.002)
		_, residual, ok := forwardReturn(stock, spy, at, 1, beta)
		if !ok {
			t.Fatal("synthetic return unavailable")
		}
		betas, residuals = append(betas, beta), append(residuals, residual)
	}
	closeTo(t, mean(betas), 2, 0.0002)
	closeTo(t, mean(residuals), 0, 1e-6)
	// Fewer than 60 adjacent return pairs: reject beta. Missing sessions
	// cannot be paired as multi-day stock returns against daily SPY moves.
	var sparse []usstock.Bar
	for d, b := range stockBars {
		if d%3 == 0 {
			sparse = append(sparse, b)
		}
	}
	if _, ok := betaAt(makeHistory(sparse), spy, 400); ok {
		t.Fatal("sparse sessions must not produce enough daily beta pairs")
	}
	if _, ok := betaAt(stock, spy, 58); ok {
		t.Fatal("59 observations are insufficient for beta")
	}
	if _, ok := betaAt(stock, spy, 60); !ok {
		t.Fatal("60 observations should be sufficient for beta")
	}
	// Shock just outside the 126-day window must have no effect.
	changed := append([]usstock.Bar(nil), stockBars...)
	changed[200] = testBar(200, stockBars[200].Close*2, 100)
	b1, _ := betaAt(stock, spy, 400)
	b2, _ := betaAt(makeHistory(changed), spy, 400)
	closeTo(t, b2, b1, 0)
}

func TestTopLongProductionOrderAndTurnover(t *testing.T) {
	signals := []Signal{
		{Symbol: "NONLONG", Score: 100, RS63Pct: 100},
		{Symbol: "B", LongSetup: true, Score: 60, RS63Pct: 4},
		{Symbol: "Z", LongSetup: true, Score: 65, RS63Pct: -5},
		{Symbol: "C", LongSetup: true, Score: 60, RS63Pct: 5},
		{Symbol: "A", LongSetup: true, Score: 60, RS63Pct: 5},
	}
	before := append([]Signal(nil), signals...)
	selected := topLong(signals, 3)
	var symbols []string
	var production stockengine.ScanReport
	for _, s := range topLong(signals, 99) {
		production.Results = append(production.Results, stockengine.ScanResult{Symbol: s.Symbol, Cycle: stockengine.CycleAssessment{LongSetup: s.LongSetup, Score: s.Score, RS63Pct: s.RS63Pct}})
	}
	for _, s := range selected {
		symbols = append(symbols, s.Symbol)
	}
	if !reflect.DeepEqual(symbols, []string{"Z", "A", "C"}) || !reflect.DeepEqual(signals, before) {
		t.Fatalf("incorrect production order or input mutation: %v", symbols)
	}
	for i, s := range production.TopLong(3, nil) {
		if selected[i].Symbol != s.Symbol {
			t.Fatal("TopLong contract mismatch")
		}
	}
	if len(topLong(signals, 99)) != 4 || len(topLong(signals, 0)) != 0 {
		t.Fatal("selection must respect N and available long setups")
	}
	for _, tc := range []struct {
		prev, curr []string
		want       float64
	}{
		{nil, nil, 0}, {nil, []string{"A", "B"}, 1},
		{[]string{"A", "B"}, nil, 1}, {[]string{"A", "B"}, []string{"A", "B"}, 0},
		{[]string{"A", "B"}, []string{"B", "C"}, 0.5}, {[]string{"A"}, []string{"A", "B"}, 0.5},
	} {
		closeTo(t, turnover(tc.prev, tc.curr), tc.want, 1e-12)
	}
}

func TestStudyEndToEndDeterministic(t *testing.T) {
	stocks, spyBars := syntheticData(30, 700)
	c := StudyConfig{[]int{1, 5, 10, 20}, 10, 7, 10, 10}
	originalWorkers := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(originalWorkers)
	one, err := runStudy(stocks, spyBars, c)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GOMAXPROCS(4)
	two, err := runStudy(stocks, spyBars, c)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatal("different worker counts produced different study results")
	}
	if len(one.IC) != 8 || one.IC[4].Spearman.N < 100 || len(one.Years) < 2 || one.LongOnly.Spearman.N == 0 || one.Spread.N == 0 {
		t.Fatalf("incomplete synthetic study: IC=%+v years=%d long=%d spread=%d", one.IC[4].Spearman, len(one.Years), one.LongOnly.Spearman.N, one.Spread.N)
	}
	if _, err := json.Marshal(one); err != nil {
		t.Fatalf("stats contain nonfinite numbers: %v", err)
	}
	spy := makeHistory(spyBars)
	invested := 0
	for _, period := range one.Portfolio.Periods {
		at := spy.byDate[mustDate(t, period.Date)]
		var signals []Signal
		for _, symbol := range sortedKeys(stocks) {
			if s, ok := signalAt(symbol, makeHistory(stocks[symbol]), spy, at); ok {
				signals = append(signals, s)
			}
		}
		var expected []string
		for _, s := range topLong(signals, c.TopN) {
			expected = append(expected, s.Symbol)
		}
		if len(period.Symbols) > c.TopN || !reflect.DeepEqual(period.Symbols, expected) {
			t.Fatalf("portfolio violates point-in-time ordering at %s: got %v want %v", period.Date, period.Symbols, expected)
		}
		closeTo(t, period.NetExcess, period.Excess-period.Turnover*2*c.CostBPS/1e4, 1e-12)
		if len(period.Symbols) > 0 {
			invested++
		}
	}
	if invested == 0 || one.Portfolio.Residual.Sharpe == nil {
		t.Fatal("synthetic portfolio never invested or lacks finite statistics")
	}
	closeTo(t, one.Coverage.AvgStocks, 30, 1e-12)
	if one.Coverage.CandidateDates != 700-1-20-259 {
		t.Fatalf("incorrect warmup/end boundaries: %+v", one.Coverage)
	}
	checkReport(t, one)
	t.Logf("synthetic: %d stocks, %d dates, residual IC=%.4f, NW t=%s, %d portfolio periods", len(stocks), one.Coverage.EvaluationDates, one.IC[4].Spearman.Mean, decimal(one.IC[4].Spearman.NWT, 2), len(one.Portfolio.Periods))
}

func mustDate(t *testing.T, date string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func checkReport(t *testing.T, r *StudyResult) {
	t.Helper()
	report := renderReport(r)
	if len(strings.Split(strings.SplitN(report, "\n\n", 2)[0], "\n")) != 6 {
		t.Fatal("Key results must be six lines")
	}
	position := -1
	for _, section := range []string{"## 1–2.", "## 3.", "## 4.", "## 5.", "## 6.", "## 7.", "## 8.", "## Caveats"} {
		next := strings.Index(report, section)
		if next <= position {
			t.Fatalf("missing or out-of-order section %s", section)
		}
		position = next
	}
	if strings.Contains(report, "NaN") || strings.Contains(report, "+Inf") {
		t.Fatal("report contains nonfinite stats")
	}
}

func TestSmallCrossSectionsAndCash(t *testing.T) {
	stocks, spyBars := syntheticData(19, 350)
	r, err := runStudy(stocks, spyBars, StudyConfig{[]int{5}, 5, 10, 5, 10})
	if err != nil {
		t.Fatal(err)
	}
	if r.Coverage.EvaluationDates != 0 || len(r.Portfolio.Periods) != 0 || r.IC[0].Spearman.N != 0 {
		t.Fatal("cross-sections with fewer than 20 names must be skipped")
	}
	checkReport(t, r)
	// All signals are non-long: hold cash, while retaining universe returns.
	stocks, spyBars = syntheticData(25, 350)
	spy := makeHistory(spyBars)
	histories := make(map[string]history)
	var rows []Signal
	for _, symbol := range sortedKeys(stocks) {
		histories[symbol] = makeHistory(stocks[symbol])
		rows = append(rows, Signal{Symbol: symbol, Beta: 1})
	}
	p := buildPortfolio([][]Signal{rows}, histories, spy, StudyConfig{Rebalance: 5, TopN: 10, CostBPS: 10}, warmupIndex)
	if len(p.Periods) != 1 || len(p.Periods[0].Symbols) != 0 || p.Periods[0].Raw != 0 || p.Periods[0].Residual != 0 || p.Periods[0].Cost != 0 {
		t.Fatalf("empty long basket must hold cash: %+v", p)
	}
}

func TestPortfolioDoesNotReplaceMissingFuturePrices(t *testing.T) {
	stocks, spyBars := syntheticData(25, 350)
	spy := makeHistory(spyBars)
	histories := make(map[string]history)
	var signals []Signal
	for i, symbol := range sortedKeys(stocks) {
		bars := stocks[symbol]
		if i == 0 {
			bars = append(append([]usstock.Bar(nil), bars[:warmupIndex+1]...), bars[warmupIndex+2:]...)
		}
		histories[symbol] = makeHistory(bars)
		signals = append(signals, Signal{Symbol: symbol, LongSetup: true, Score: float64(100 - i), Beta: 1})
	}
	p := buildPortfolio([][]Signal{signals}, histories, spy, StudyConfig{Rebalance: 5, TopN: 3, CostBPS: 10}, warmupIndex)
	if len(p.Periods) != 0 || !strings.Contains(p.Skipped[spy.dates[warmupIndex].Format(time.DateOnly)], "S00BUSDT") {
		t.Fatalf("missing top-ranked future price must skip period without backfilling: %+v", p)
	}
}

func TestPortfolioTurnoverAcrossSkippedPeriod(t *testing.T) {
	stocks, spyBars := syntheticData(25, 350)
	spy := makeHistory(spyBars)
	histories := make(map[string]history)
	var base []Signal
	for _, symbol := range sortedKeys(stocks) {
		bars := stocks[symbol]
		if symbol == "S00BUSDT" {
			missing := warmupIndex + 5 + 1
			bars = append(append([]usstock.Bar(nil), bars[:missing]...), bars[missing+1:]...)
		}
		histories[symbol] = makeHistory(bars)
		base = append(base, Signal{Symbol: symbol, Beta: 1})
	}
	signals := make([][]Signal, 11)
	for _, offset := range []int{0, 5, 10} {
		signals[offset] = append([]Signal(nil), base...)
		selected := 0
		if offset == 0 {
			selected = 1
		}
		signals[offset][selected].LongSetup = true
		signals[offset][selected].Score = 80
	}
	p := buildPortfolio(signals, histories, spy, StudyConfig{Rebalance: 5, TopN: 1, CostBPS: 10}, warmupIndex+10)
	if len(p.Periods) != 2 || len(p.Skipped) != 1 {
		t.Fatalf("expected middle period to be unpriced: %+v", p)
	}
	closeTo(t, p.Periods[0].Turnover, 1, 0)
	closeTo(t, p.Periods[1].Turnover, 0, 0)
	closeTo(t, p.Periods[1].NetExcess, p.Periods[1].Excess, 0)
}

func TestUniverseAndConfig(t *testing.T) {
	registry := []usstock.SymbolInfo{{Symbol: "SPYBUSDT"}, {Symbol: "QQQBUSDT"}, {Symbol: "TQQQBUSDT"}, {Symbol: "AAPLBUSDT"}, {Symbol: "AAPLBUSDT"}}
	symbols, excluded := selectUniverse(registry, defaultExclude)
	if !reflect.DeepEqual(symbols, []string{"AAPLBUSDT"}) || len(excluded) != 3 {
		t.Fatalf("default exclusions or deduplication incorrect: %v %v", symbols, excluded)
	}
	symbols, _ = selectUniverse(registry, "")
	if len(symbols) != 3 {
		t.Fatal("custom exclusions should still remove SPY")
	}
	c, err := (StudyConfig{[]int{5, 1, 5}, 10, 3, 5, 0}).normalized()
	if err != nil || !reflect.DeepEqual(c.Horizons, []int{1, 5, 10}) {
		t.Fatalf("invalid horizon normalization: %+v, %v", c, err)
	}
	for _, value := range []string{"", "0", "1,-5", "1,abc", "1,"} {
		if _, err := parseHorizons(value); err == nil {
			t.Fatalf("invalid horizons %q accepted", value)
		}
	}
	if _, err := (StudyConfig{[]int{1}, 1, 3, 5, math.NaN()}).normalized(); err == nil {
		t.Fatal("nonfinite cost accepted")
	}
}

func TestOfflineCacheAndShortSeries(t *testing.T) {
	dir := t.TempDir()
	stocks, _ := syntheticData(1, 350)
	calls := 0
	fetch := func(_ context.Context, symbol, tf string, need int, yahoo bool) (*usstock.Series, error) {
		calls++
		if symbol != "S00BUSDT" || tf != usstock.TF1d || !yahoo {
			t.Fatal("wrong data layer request")
		}
		return &usstock.Series{Symbol: symbol, Timeframe: tf, Source: usstock.SourceYahoo, Bars: stocks[symbol]}, fmt.Errorf("wrapped: %w", usstock.ErrInsufficientBars)
	}
	ctx := context.Background()
	series, cached, warning, err := loadSeries(ctx, "S00BUSDT", 1300, dir, time.Now(), fetch)
	if err != nil || cached || warning == "" || len(series.Bars) != 350 {
		t.Fatalf("short series with wrapped ErrInsufficientBars should be accepted: cached=%t warning=%s err=%v", cached, warning, err)
	}
	series, cached, _, err = loadSeries(ctx, "S00BUSDT", 1300, dir, time.Now(), fetch)
	if err != nil || !cached || calls != 1 || len(series.Bars) != 350 {
		t.Fatal("fresh short cache was not reused")
	}
	_, cached, _, err = loadSeries(ctx, "S00BUSDT", 1500, dir, time.Now(), fetch)
	if err != nil || cached || calls != 2 {
		t.Fatal("larger history request must refetch")
	}
	path := filepath.Join(dir, "S00BUSDT.json")
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	_, cached, _, err = loadSeries(ctx, "S00BUSDT", 1500, dir, time.Now(), fetch)
	if err != nil || cached || calls != 3 {
		t.Fatal("expired cache must refetch")
	}
	if err := os.WriteFile(path, []byte("broken JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	_, cached, _, err = loadSeries(ctx, "S00BUSDT", 1300, dir, time.Now(), fetch)
	if err != nil || cached || calls != 4 {
		t.Fatal("corrupt cache must refetch")
	}
	_, _, _, err = loadSeries(ctx, "../bad", 1300, dir, time.Now(), fetch)
	if err == nil || calls != 4 {
		t.Fatal("unsafe cache symbol accepted")
	}
	for _, tc := range []struct {
		n   int
		err error
	}{
		{299, usstock.ErrInsufficientBars}, {350, errors.New("unrelated transport failure")},
	} {
		fetchBad := func(context.Context, string, string, int, bool) (*usstock.Series, error) {
			return &usstock.Series{Bars: stocks["S00BUSDT"][:tc.n]}, tc.err
		}
		if _, _, _, err := loadSeries(ctx, "BADBUSDT", 1300, dir, time.Now(), fetchBad); err == nil {
			t.Fatal("unacceptable series accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, _, err := loadSeries(canceled, "S00BUSDT", 1300, dir, time.Now(), fetch); !errors.Is(err, context.Canceled) || calls != 4 {
		t.Fatal("cancellation must take precedence over a fresh cache")
	}
}
