package stockengine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"nofx/market/usstock"
	"nofx/store"
)

// The live scan enriches rows from Yahoo; unit tests must never touch the
// network, so the default seam fails fast. Tests that need fundamentals set
// their own stub and restore this one.
func init() {
	scanGetFundamentals = func(context.Context, string) (usstock.Fundamentals, error) {
		return usstock.Fundamentals{}, errors.New("fundamentals not stubbed")
	}
}

func universeTestBars(n int, f func(i int) float64) []usstock.Bar {
	bars := make([]usstock.Bar, n)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range bars {
		c := f(i)
		bars[i] = usstock.Bar{OpenTime: start.AddDate(0, 0, i), Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1000}
	}
	return bars
}

func TestBuildUniverseRow(t *testing.T) {
	// 60 bars of constant +1% daily growth: every return is identical (vol 0).
	bars := universeTestBars(60, func(i int) float64 { return 100 * math.Pow(1.01, float64(i)) })
	row, ok := BuildUniverseRow("SPYBUSDT", "SPY", bars)
	if !ok {
		t.Fatal("row not built")
	}
	last := bars[59].Close
	near2 := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-6 {
			t.Fatalf("%s = %.9f want %.9f", name, got, want)
		}
	}
	near2("price", row.Price, last)
	near2("r1d", row.Ret1dPct, 1)
	near2("r10d", row.Ret10dPct, (math.Pow(1.01, 10)-1)*100)
	near2("r30d", row.Ret30dPct, (math.Pow(1.01, 30)-1)*100)
	near2("vol30", row.Vol30Pct, 0)
	near2("rsi", row.RSI14, 100)
	near2("from high", row.DistFrom30dHighPct, (last/(last+1)-1)*100)
	var adv float64
	for _, b := range bars[40:] {
		adv += b.Close * b.Volume / 20
	}
	near2("adv20", row.AvgDollarVol20, adv)
	if row.Class != "ETF" || row.Sector != "" || row.DaysToEarnings != -1 || row.Symbol != "SPYBUSDT" || row.Underlying != "SPY" {
		t.Fatalf("row = %+v", row)
	}

	// Alternating +2% / −2% log-ish returns give a known annualized volatility.
	alt := universeTestBars(40, func(i int) float64 {
		if i%2 == 0 {
			return 100
		}
		return 102
	})
	row, ok = BuildUniverseRow("XBUSDT", "X", alt)
	if !ok || row.Class != "" {
		t.Fatalf("row=%+v ok=%v", row, ok)
	}
	up, down := math.Log(1.02), -math.Log(1.02)
	mean := (up + down) / 2 // 30 returns: 15 up, 15 down
	variance := (15*(up-mean)*(up-mean) + 15*(down-mean)*(down-mean)) / 29
	near2("vol30 alt", row.Vol30Pct, math.Sqrt(variance)*math.Sqrt(252)*100)

	if _, ok := BuildUniverseRow("XBUSDT", "X", bars[:30]); ok {
		t.Fatal("30 bars must not build a row")
	}
	if _, ok := BuildUniverseRow("XBUSDT", "X", bars[:31]); !ok {
		t.Fatal("31 bars must build a row")
	}
	// Pulled back from the 30-bar high: from-high is negative.
	pull := universeTestBars(40, func(i int) float64 {
		if i == 20 {
			return 150
		}
		return 100
	})
	row, _ = BuildUniverseRow("XBUSDT", "X", pull)
	near2("pullback", row.DistFrom30dHighPct, (100.0/151-1)*100)
}

func TestApplyFundamentals(t *testing.T) {
	now := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC) // 2026-10-14 23:00 ET
	row := UniverseRow{DaysToEarnings: -1}
	ApplyFundamentals(&row, usstock.Fundamentals{QuoteType: "EQUITY", Sector: "Technology", NextEarnings: time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC)}, now)
	if row.Class != "stock" || row.Sector != "Technology" || row.DaysToEarnings != 15 {
		t.Fatalf("row = %+v", row)
	}
	row = UniverseRow{Class: "stock", DaysToEarnings: 4}
	ApplyFundamentals(&row, usstock.Fundamentals{QuoteType: "ETF"}, now)
	if row.Class != "ETF" || row.DaysToEarnings != -1 {
		t.Fatalf("row = %+v", row)
	}
	// A past earnings date is unknown; today's ET date counts as 0 days.
	if got := daysUntil(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC), now); got != 0 {
		t.Fatalf("same ET day = %d", got)
	}
	if got := daysUntil(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC), now); got != -1 {
		t.Fatalf("past = %d", got)
	}
	// Unknown quote types keep the static class.
	row = UniverseRow{Class: "ETF", DaysToEarnings: -1}
	ApplyFundamentals(&row, usstock.Fundamentals{QuoteType: "MUTUALFUND"}, now)
	if row.Class != "ETF" {
		t.Fatalf("row = %+v", row)
	}
}

func TestFormatPriceAndVolume(t *testing.T) {
	for in, want := range map[float64]string{
		0: "0", 123.456: "123.5", 1234.56: "1235", 12345.6: "12346", 9.87654: "9.877", 1.5: "1.5", 100: "100",
		0.123456: "0.1235", 0.5: "0.5", 0.00004: "0", 99.995: "100",
	} {
		if got := fmtPx(in); got != want {
			t.Errorf("fmtPx(%v) = %q want %q", in, got, want)
		}
	}
	for in, want := range map[float64]string{
		0: "0", 950: "950", 999.6: "1K", 12300: "12.3K", 1234567: "1.23M", 4.5e9: "4.5B", 999999: "1M", 1e12: "1T", 123456: "123K", 10500: "10.5K",
	} {
		if got := fmtVol(in); got != want {
			t.Errorf("fmtVol(%v) = %q want %q", in, got, want)
		}
	}
}

func TestScanFillsUniverseRow(t *testing.T) {
	bars := universeTestBars(260, func(i int) float64 { return 100 + float64(i)*0.1 })
	report, err := Scan(context.Background(), []usstock.SymbolInfo{{Symbol: "QQQBUSDT", Underlying: "QQQ"}}, nil,
		func(context.Context, string) ([]usstock.Bar, string, error) { return bars, usstock.SourceBStock, nil }, time.Now())
	if err != nil || len(report.Results) != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	u := report.Results[0].Universe
	if u.Symbol != "QQQBUSDT" || u.Underlying != "QQQ" || u.Class != "ETF" || u.Price != bars[259].Close || u.DaysToEarnings != -1 {
		t.Fatalf("universe row = %+v", u)
	}
}

func TestLiveScanEnrichesFundamentalsBestEffort(t *testing.T) {
	env := setupScanTest(t)
	env.universe = []usstock.SymbolInfo{{Symbol: "AAPLBUSDT", Underlying: "AAPL"}, {Symbol: "ZZZBUSDT", Underlying: "ZZZ"}, {Symbol: "SPYBUSDT", Underlying: "SPY"}}
	scanGetSeries = func(ctx context.Context, symbol, timeframe string, need int, allowYahoo bool) (*usstock.Series, error) {
		return &usstock.Series{Bars: universeTestBars(260, func(i int) float64 { return 100 + float64(i)*0.1 }), Source: usstock.SourceBStock}, nil
	}
	old := scanGetFundamentals
	t.Cleanup(func() { scanGetFundamentals = old })
	scanGetFundamentals = func(_ context.Context, underlying string) (usstock.Fundamentals, error) {
		switch underlying {
		case "AAPL":
			return usstock.Fundamentals{QuoteType: "EQUITY", Sector: "Technology", NextEarnings: scanNow().AddDate(0, 0, 10)}, nil
		case "SPY":
			return usstock.Fundamentals{QuoteType: "ETF"}, nil
		}
		return usstock.Fundamentals{}, errors.New("not found")
	}
	report, err := GetScanReport(context.Background(), time.Hour)
	if err != nil || len(report.Results) != 3 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	by := map[string]UniverseRow{}
	for _, r := range report.Results {
		by[r.Symbol] = r.Universe
	}
	if a := by["AAPLBUSDT"]; a.Class != "stock" || a.Sector != "Technology" || a.DaysToEarnings != 10 {
		t.Fatalf("AAPL = %+v", a)
	}
	if s := by["SPYBUSDT"]; s.Class != "ETF" || s.DaysToEarnings != -1 {
		t.Fatalf("SPY = %+v", s)
	}
	if z := by["ZZZBUSDT"]; z.Class != "" || z.Sector != "" || z.DaysToEarnings != -1 {
		t.Fatalf("failed fundamentals must leave the row unknown: %+v", z)
	}
}

func TestLiveScanFundamentalsBudgetStopsEnrichment(t *testing.T) {
	env := setupScanTest(t)
	env.universe = []usstock.SymbolInfo{{Symbol: "ABUSDT", Underlying: "A"}, {Symbol: "BBUSDT", Underlying: "B"}, {Symbol: "CBUSDT", Underlying: "C"}}
	scanGetSeries = func(ctx context.Context, symbol, timeframe string, need int, allowYahoo bool) (*usstock.Series, error) {
		return &usstock.Series{Bars: universeTestBars(260, func(i int) float64 { return 100 + float64(i)*0.1 }), Source: usstock.SourceBStock}, nil
	}
	oldBudget, oldFn := scanFundamentalsBudget, scanGetFundamentals
	t.Cleanup(func() { scanFundamentalsBudget, scanGetFundamentals = oldBudget, oldFn })
	scanFundamentalsBudget = 30 * time.Millisecond
	calls := 0
	scanGetFundamentals = func(ctx context.Context, underlying string) (usstock.Fundamentals, error) {
		calls++
		<-ctx.Done() // a stuck Yahoo: only the budget ends the call
		return usstock.Fundamentals{}, ctx.Err()
	}
	report, err := GetScanReport(context.Background(), time.Hour)
	if err != nil || len(report.Results) != 3 {
		t.Fatalf("a slow fundamentals source must not fail the scan: report=%+v err=%v", report, err)
	}
	if calls != 1 {
		t.Fatalf("enrichment should stop once the budget is spent, calls=%d", calls)
	}
}

func universeTestContext(zh bool) *Context {
	c := testContext()
	c.Config.Symbols = []string{"AAPLBUSDT"}
	c.Config.ScanExclude = []string{"TSLABUSDT", "AAPLBUSDT"}
	c.Snapshots = c.Snapshots[:1]
	c.Universe = []ScanResult{
		{Symbol: "TSLABUSDT", Underlying: "TSLA", Cycle: CycleAssessment{Phase: "downtrend"}, Universe: UniverseRow{Symbol: "TSLABUSDT", Class: "stock", Price: 250.123, Ret1dPct: -1.04, DaysToEarnings: -1, AvgDollarVol20: 950}},
		{Symbol: "AAPLBUSDT", Underlying: "AAPL", Momentum: MomentumAssessment{Valid: true, Mom12_1Pct: 12.34, AboveSMA200: true}, Cycle: CycleAssessment{Phase: "wedge_pop"},
			Universe: UniverseRow{Symbol: "AAPLBUSDT", Class: "stock", Sector: "Technology", Price: 189.5, Ret1dPct: 0.5, Ret10dPct: 2, Ret30dPct: 4, Vol30Pct: 28.2, RSI14: 61, DistFrom30dHighPct: -1.5, AvgDollarVol20: 12.3e6, DaysToEarnings: 12}},
		{Symbol: "NOPRICEBUSDT"}, // too little history for a row: skipped
		{Symbol: "SPYBUSDT", Underlying: "SPY", Momentum: MomentumAssessment{Valid: true, Mom12_1Pct: -3, AboveSMA200: false}, Cycle: CycleAssessment{Phase: "uptrend"},
			Universe: UniverseRow{Symbol: "SPYBUSDT", Class: "ETF", Price: 600, AvgDollarVol20: 4.5e9, DaysToEarnings: -1}},
	}
	return c
}

func TestUserPromptUniverseTable(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		p := BuildUserPrompt(universeTestContext(lang == "zh"), lang)
		heading := "Universe (all listed bStock pairs; reference only — open/add only symbols in the candidate list above)"
		if lang == "zh" {
			heading = "全市场概览（所有 bStock；仅供参考——只能对上面的候选标的开仓/加仓）"
		}
		i := strings.Index(p, heading)
		if i < 0 {
			t.Fatalf("%s: missing heading:\n%s", lang, p)
		}
		if j := strings.LastIndex(p, "AAPLBUSDT ("); j < 0 || j > i {
			t.Fatalf("%s: universe must follow the candidate sections", lang)
		}
		table := p[i:]
		for _, want := range []string{"symbol|class|sector|candidate",
			// candidate wins over excluded; momentum and phase come from the same result.
			"AAPLBUSDT|stock|Technology|Y|189.5|0.5|2.0|4.0|28.2|61.0|-1.5|$12.3M|12|12.3|Y|wedge_pop",
			"SPYBUSDT|ETF|||600|", "|$4.5B||-3.0|N|uptrend",
			// excluded marker, unknown mom/earn left blank, tiny adv in plain dollars.
			"TSLABUSDT|stock||x|250.1|-1.0|0.0|0.0|0.0|0.0|0.0|$950|||"} {
			if !strings.Contains(table, want) {
				t.Fatalf("%s: table missing %q:\n%s", lang, want, table)
			}
		}
		if strings.Contains(table, "NOPRICE") {
			t.Fatalf("%s: rows without data must be skipped", lang)
		}
		// Sorted by symbol.
		if !(strings.Index(table, "AAPLBUSDT|") < strings.Index(table, "SPYBUSDT|") && strings.Index(table, "SPYBUSDT|") < strings.Index(table, "TSLABUSDT|")) {
			t.Fatalf("%s: rows not sorted:\n%s", lang, table)
		}
	}
	c := universeTestContext(false)
	c.Universe = nil
	if p := BuildUserPrompt(c, "en"); strings.Contains(p, "Universe (") {
		t.Fatal("no universe section without rows")
	}
}

func TestSnapshotCandlesAndRSI(t *testing.T) {
	mk := func(tf string, n int) *usstock.Series {
		return &usstock.Series{Timeframe: tf, Source: usstock.SourceBStock, Bars: universeTestBars(n, func(i int) float64 { return 100 + float64(i) })}
	}
	series := map[string]*usstock.Series{usstock.TF1d: mk(usstock.TF1d, 220), usstock.TF1w: mk(usstock.TF1w, 60), usstock.TF4h: mk(usstock.TF4h, 14), usstock.TF1h: mk(usstock.TF1h, 300)}
	s := BuildSnapshot("AAPLBUSDT", "AAPL", series, nil, time.Now())
	if len(s.Candles[usstock.TF1w]) != promptCandles1w || len(s.Candles[usstock.TF1d]) != promptCandles1d {
		t.Fatalf("candle counts 1w=%d 1d=%d", len(s.Candles[usstock.TF1w]), len(s.Candles[usstock.TF1d]))
	}
	if got := s.Candles[usstock.TF1d]; got[len(got)-1].Close != 319 || got[0].Close != 290 {
		t.Fatalf("daily tail wrong: %v .. %v", got[0].Close, got[len(got)-1].Close)
	}
	if _, ok := s.Candles[usstock.TF1h]; ok {
		t.Fatal("1h is not a prompt timeframe")
	}
	// Candles are copies: mutating the series must not leak into the snapshot.
	series[usstock.TF1d].Bars[219].Close = -1
	if s.Candles[usstock.TF1d][promptCandles1d-1].Close != 319 {
		t.Fatal("candles alias the series")
	}
	// 4h with 14 bars: candles shown, RSI undefined (needs 15 bars).
	if _, ok := s.RSI[usstock.TF4h]; ok || len(s.Candles[usstock.TF4h]) != 14 {
		t.Fatalf("4h rsi=%v candles=%d", s.RSI, len(s.Candles[usstock.TF4h]))
	}
	if s.RSI[usstock.TF1d] != 100 || s.RSI[usstock.TF1w] != 100 {
		t.Fatalf("rsi = %v", s.RSI)
	}
	// No weekly/4h series at all: nothing printed, nothing flagged beyond MissingTF.
	bare := BuildSnapshot("AAPLBUSDT", "AAPL", map[string]*usstock.Series{usstock.TF1d: mk(usstock.TF1d, 220)}, nil, time.Now())
	if _, ok := bare.Candles[usstock.TF1w]; ok || bare.RSI[usstock.TF1w] != 0 {
		t.Fatalf("bare = %+v", bare)
	}
}

func TestWriteCandlesFormat(t *testing.T) {
	s := &SymbolSnapshot{
		RSI: map[string]float64{"4h": 56.24, "1d": 61, "1w": 58.31},
		Candles: map[string][]usstock.Bar{
			usstock.TF1w: {{OpenTime: time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC), Open: 123.4, High: 130.12, Low: 120, Close: 128.66, Volume: 1.23e6}},
			usstock.TF4h: {{OpenTime: time.Date(2026, 10, 14, 13, 30, 0, 0, time.UTC), Open: 0.5, High: 0.6, Low: 0.4, Close: 0.55, Volume: 950}},
		},
	}
	var en, zh strings.Builder
	writeCandles(&en, s, false)
	writeCandles(&zh, s, true)
	for _, want := range []string{"rsi14: 4h=56.2 1d=61.0 1w=58.3\n",
		"candles 1w (1, oldest→newest; date,o,h,l,c,v):\n2026-04-13,123.4,130.1,120,128.7,1.23M\n",
		"candles 4h (1; ET bar start, oldest→newest; date,o,h,l,c,v):\n2026-10-14 09:30,0.5,0.6,0.4,0.55,950\n"} {
		if !strings.Contains(en.String(), want) {
			t.Fatalf("missing %q in:\n%s", want, en.String())
		}
	}
	if strings.Contains(en.String(), "candles 1d") {
		t.Fatal("timeframes without candles must be skipped")
	}
	if !strings.Contains(zh.String(), "K线 1w (1；旧→新；date,o,h,l,c,v):") || !strings.Contains(zh.String(), "2026-10-14 09:30,") {
		t.Fatalf("zh output:\n%s", zh.String())
	}
}

// TestUserPromptSizeBudget renders a realistic prompt (12 candidates, 2
// benchmarks, 84 universe rows) and keeps its size bounded.
func TestUserPromptSizeBudget(t *testing.T) {
	cfg := &store.StockConfig{}
	series := func(tf string, n int, seed int) *usstock.Series {
		return &usstock.Series{Timeframe: tf, Source: usstock.SourceBStock, Bars: universeTestBars(n, func(i int) float64 { return 123.4567 + float64(i)*0.31 + float64(seed) })}
	}
	snap := func(sym string, seed int) *SymbolSnapshot {
		s := BuildSnapshot(sym, strings.TrimSuffix(sym, "BUSDT"), map[string]*usstock.Series{
			usstock.TF1d: series(usstock.TF1d, 220, seed), usstock.TF1w: series(usstock.TF1w, 60, seed), usstock.TF4h: series(usstock.TF4h, 200, seed), usstock.TF1h: series(usstock.TF1h, 300, seed),
		}, &usstock.Quote{BStockPrice: 150.12, RefPrice: 150.1, RefFresh: true, RefTime: time.Date(2026, 10, 14, 14, 0, 0, 0, time.UTC), Session: usstock.SessionRegular}, time.Now())
		for i := range s.Candles[usstock.TF1d] { // realistic volumes
			s.Candles[usstock.TF1d][i].Volume = 12_345_678
		}
		c := ClassifyCycle(series(usstock.TF1d, 220, seed).Bars, nil)
		s.Cycle = &c
		s.Momentum = &MomentumAssessment{Valid: true, Mom12_1Pct: 14.2, AboveSMA200: true}
		return s
	}
	ctx := &Context{Now: time.Date(2026, 10, 14, 14, 0, 0, 0, time.UTC), Session: usstock.SessionRegular, Config: cfg, Account: Account{Equity: 10000, Available: 8000}}
	for i := 0; i < 12; i++ {
		sym := fmt.Sprintf("C%02dBUSDT", i)
		cfg.Symbols = append(cfg.Symbols, sym)
		ctx.Snapshots = append(ctx.Snapshots, snap(sym, i))
	}
	ctx.Market = []*SymbolSnapshot{snap("SPYBUSDT", 1), snap("QQQBUSDT", 2)}
	for i := 0; i < 84; i++ {
		sym := fmt.Sprintf("U%02dBUSDT", i)
		ctx.Universe = append(ctx.Universe, ScanResult{Symbol: sym, Underlying: strings.TrimSuffix(sym, "BUSDT"), Momentum: MomentumAssessment{Valid: true, Mom12_1Pct: 14.2, AboveSMA200: true}, Cycle: CycleAssessment{Phase: "ema_crossback"},
			Universe: UniverseRow{Symbol: sym, Class: "stock", Sector: "Consumer Cyclical", Price: 123.456, Ret1dPct: 1.23, Ret10dPct: 4.56, Ret30dPct: -7.89, Vol30Pct: 33.3, RSI14: 55.5, DistFrom30dHighPct: -4.4, AvgDollarVol20: 123456789, DaysToEarnings: 21}})
	}
	for _, lang := range []string{"en", "zh"} {
		p := BuildUserPrompt(ctx, lang)
		t.Logf("%s user prompt: %d chars, %d bytes", lang, len([]rune(p)), len(p))
		if len([]rune(p)) > 90_000 {
			t.Fatalf("%s prompt too large: %d chars", lang, len([]rune(p)))
		}
	}
}
