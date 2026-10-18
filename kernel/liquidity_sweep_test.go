package kernel

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"nofx/market"
	"nofx/store"
)

var sweepEpoch = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

func sweepTestBars(tf string, n int) []market.KlineBar {
	bars := make([]market.KlineBar, n)
	for i := range bars {
		bars[i] = market.KlineBar{Time: sweepEpoch.Add(time.Duration(i) * tfDuration(tf)).UnixMilli(),
			Open: 100, High: 110, Low: 99, Close: 100, Volume: 1000}
		switch i {
		case 3:
			bars[i].Low = 95
		case 7:
			bars[i].High = 115
		case 11:
			bars[i].Low = 94
		case 12:
			bars[i].High, bars[i].Close = 117, 116
		}
	}
	return bars
}

func sweepTestData(tf string, bars []market.KlineBar) *market.Data {
	return &market.Data{Symbol: "SWEEPUSDT", CurrentPrice: 100,
		TimeframeData: map[string]*market.TimeframeSeriesData{tf: {Timeframe: tf, Klines: bars}}}
}

func sweepAtEnd(tf string, bars []market.KlineBar) *LiquiditySweep {
	return computeLiquiditySweep(sweepTestData(tf, bars), sweepEpoch.Add(time.Duration(len(bars))*tfDuration(tf)))
}

func mirrorSweepBars(bars []market.KlineBar) []market.KlineBar {
	out := append([]market.KlineBar(nil), bars...)
	for i, b := range bars {
		out[i].Open, out[i].Close = 200-b.Open, 200-b.Close
		out[i].High, out[i].Low = 200-b.Low, 200-b.High
	}
	return out
}

func TestLiquiditySweepReclaimAndMirror(t *testing.T) {
	for _, tf := range []string{"15m", "1h"} {
		for _, short := range []bool{false, true} {
			for _, tc := range []struct {
				name       string
				low, close float64
				want       bool
			}{
				{"reclaimed", 94, 100, true},
				{"closed_beyond", 94, 94.5, false},
				{"closed_on_level", 94, 95, false},
				{"touched_only", 95, 100, false},
			} {
				t.Run(fmt.Sprintf("%s/short=%v/%s", tf, short, tc.name), func(t *testing.T) {
					bars := sweepTestBars(tf, 12)
					bars[11].Low, bars[11].Close = tc.low, tc.close
					if short {
						bars = mirrorSweepBars(bars)
					}
					got := sweepAtEnd(tf, bars)
					if !tc.want {
						if got != nil {
							t.Fatalf("unexpected sweep: %+v", got)
						}
						return
					}
					if got == nil {
						t.Fatal("missing reclaimed sweep")
					}
					event, other, level, extreme := got.SellsideSwept, got.BuysideSwept, 95.0, 94.0
					if short {
						event, other, level, extreme = got.BuysideSwept, got.SellsideSwept, 105, 106
					}
					want := &LiquiditySweepEvent{TF: tf, Level: level, Extreme: extreme, DepthPct: math.Abs(extreme-level) / level * 100, Reclaimed: true}
					if !reflect.DeepEqual(event, want) || other != nil {
						t.Fatalf("got %+v (other=%+v), want %+v", event, other, want)
					}
					encoded, err := json.Marshal(got)
					if err != nil || !strings.Contains(string(encoded), ":null") {
						t.Fatalf("missing side must be explicit null: %s, %v", encoded, err)
					}
				})
			}
		}
	}
}

func TestLiquiditySweepConsumesAllPiercedLevels(t *testing.T) {
	for _, tf := range []string{"15m", "1h"} {
		for _, short := range []bool{false, true} {
			for _, tc := range []struct {
				name   string
				modify func([]market.KlineBar)
				want   float64
			}{
				{"failed_reclaim_consumes", func(b []market.KlineBar) {
					b[9].Low, b[9].Close = 94, 94.5
				}, 0},
				{"successful_reclaim_consumes", func(b []market.KlineBar) {
					b[10].Low = 94
					b[11].Low = 93
				}, 95},
				{"non_selected_older_level_consumed", func(b []market.KlineBar) {
					b[3].Low, b[7].Low = 90, 95
					b[11].Low, b[11].Close = 89, 94
					b[13].Low, b[13].Close = 89, 94
				}, 0},
				{"unpierced_older_level_survives", func(b []market.KlineBar) {
					b[3].Low, b[7].Low = 90, 95
					b[11].Low, b[11].Close = 94, 94.5
					b[13].Low = 89
				}, 90},
				{"only_latest_ref_can_trigger", func(b []market.KlineBar) {
					b[3].Low, b[7].Low = 95, 90
					b[7].Close = 94 // forming the newer low must not reclaim the old one
				}, 0},
			} {
				t.Run(fmt.Sprintf("%s/short=%v/%s", tf, short, tc.name), func(t *testing.T) {
					bars := sweepTestBars(tf, 14)
					bars[12].High, bars[12].Close = 110, 100
					tc.modify(bars)
					if short {
						bars = mirrorSweepBars(bars)
					}
					got := sweepAtEnd(tf, bars)
					if tc.want == 0 {
						if got != nil {
							t.Fatalf("consumed/non-selected level reused: %+v", got)
						}
						return
					}
					if got == nil {
						t.Fatal("missing sweep of surviving level")
					}
					event, level := got.SellsideSwept, tc.want
					if short {
						event, level = got.BuysideSwept, 200-level
					}
					if event == nil || event.Level != level {
						t.Fatalf("event = %+v, want level %v", event, level)
					}
					if tc.name == "successful_reclaim_consumes" && event.BarsAgo != 3 {
						t.Fatalf("original event must be retained, got %+v", event)
					}
				})
			}
		}
	}
}

func TestLiquiditySweepHistoryAndStrictPivots(t *testing.T) {
	for _, tf := range []string{"15m", "1h"} {
		bars := sweepTestBars(tf, 24)
		bars[11].Close, bars[12].Close, bars[12].High, bars[12].Low = 94.5, 100, 110, 94
		bars[22].Low = 94.5 // would falsely sweep 95 if pre-window consumption were lost
		if got := sweepAtEnd(tf, bars); got != nil {
			t.Fatalf("%s: consumption before reporting window lost: %+v", tf, got)
		}
		bars = sweepTestBars(tf, 12)
		bars[4].Low = bars[3].Low // equal lows are not strict pivots
		if got := sweepAtEnd(tf, bars); got != nil {
			t.Fatalf("%s: equal lows must not form a pivot: %+v", tf, got)
		}
	}
	for _, data := range []*market.Data{nil, {}, {TimeframeData: map[string]*market.TimeframeSeriesData{"15m": nil}}} {
		if got := computeLiquiditySweep(data, sweepEpoch); got != nil {
			t.Fatalf("missing data must omit sweep: %+v", got)
		}
	}
}

func TestLiquiditySweepMSS(t *testing.T) {
	for _, tf := range []string{"15m", "1h"} {
		for _, short := range []bool{false, true} {
			for _, tc := range []struct {
				name   string
				modify func([]market.KlineBar)
				want   bool
			}{
				{"later_close_breaks_structure", func([]market.KlineBar) {}, true},
				{"wick_only", func(b []market.KlineBar) { b[12].Close = 110 }, false},
				{"close_equals_structure", func(b []market.KlineBar) { b[12].Close = 115 }, false},
				{"sweep_bar_cannot_confirm_mss", func(b []market.KlineBar) {
					b[11].Close, b[11].High, b[12].Close = 116, 117, 100
				}, false},
				{"latest_opposite_not_older", func(b []market.KlineBar) {
					b[3].High, b[7].High = 115, 120
				}, false},
				{"no_opposite_pivot", func(b []market.KlineBar) { b[7].High = 110 }, false},
				{"opposite_confirmed_at_sweep_close", func(b []market.KlineBar) {
					b[7].High = 110
					i := 9
					if tf == "1h" {
						i = 8
					}
					b[i].High = 115
				}, true},
				{"opposite_confirmed_after_sweep_is_ineligible", func(b []market.KlineBar) {
					b[7].High, b[10].High = 110, 115
					b[12].High, b[12].Close = 110, 100
					b[14].High, b[14].Close = 117, 116
				}, false},
			} {
				t.Run(fmt.Sprintf("%s/short=%v/%s", tf, short, tc.name), func(t *testing.T) {
					bars := sweepTestBars(tf, 15)
					tc.modify(bars)
					if short {
						bars = mirrorSweepBars(bars)
					}
					got := sweepAtEnd(tf, bars)
					if got == nil {
						t.Fatal("missing sweep")
					}
					event := got.SellsideSwept
					if short {
						event = got.BuysideSwept
					}
					if event == nil || event.MSSConfirmed != tc.want {
						t.Fatalf("event = %+v, want mss=%v", event, tc.want)
					}
				})
			}
		}
	}
}

func TestLiquiditySweepLookbackAndRecency(t *testing.T) {
	for _, cfg := range []struct {
		tf               string
		lookback, recent int
	}{{"15m", 40, 8}, {"1h", 48, 4}} {
		for _, extra := range []int{0, 1} {
			s := 3 + cfg.lookback + extra
			bars := sweepTestBars(cfg.tf, s+1)
			bars[11].Low, bars[12].High, bars[12].Close = 99, 110, 100
			bars[s].Low = 94
			got := sweepAtEnd(cfg.tf, bars)
			if (got != nil) != (extra == 0) {
				t.Fatalf("%s pivot age=%d: got %+v", cfg.tf, cfg.lookback+extra, got)
			}
		}
		for _, age := range []int{cfg.recent - 1, cfg.recent} {
			got := sweepAtEnd(cfg.tf, sweepTestBars(cfg.tf, 12+age))
			if (got != nil) != (age < cfg.recent) {
				t.Fatalf("%s bars_ago=%d: got %+v", cfg.tf, age, got)
			}
		}
	}
}

func TestLiquiditySweepClosedCandlesAndCausality(t *testing.T) {
	for _, tf := range []string{"15m", "1h"} {
		dur := tfDuration(tf)
		bars := sweepTestBars(tf, 12)
		if got := computeLiquiditySweep(sweepTestData(tf, bars), sweepEpoch.Add(12*dur-dur/2)); got != nil {
			t.Fatalf("%s forming sweep must be ignored: %+v", tf, got)
		}
		bars = sweepTestBars(tf, 13)
		got := computeLiquiditySweep(sweepTestData(tf, bars), sweepEpoch.Add(13*dur-dur/2))
		if got == nil || got.SellsideSwept == nil || got.SellsideSwept.MSSConfirmed {
			t.Fatalf("%s forming MSS must be ignored: %+v", tf, got)
		}
		full := sweepTestBars(tf, 20)
		full[15].Low, full[17].High, full[17].Close = 93, 121, 120
		for end := 0; end <= len(full); end++ {
			now := sweepEpoch.Add(time.Duration(end) * dur)
			withFuture := computeLiquiditySweep(sweepTestData(tf, full), now)
			truncated := computeLiquiditySweep(sweepTestData(tf, full[:end]), now)
			if !reflect.DeepEqual(withFuture, truncated) {
				t.Fatalf("%s prefix=%d changed by future bars: %+v vs %+v", tf, end, withFuture, truncated)
			}
		}
	}
}

func TestLiquiditySweepLatestAcrossTimeframes(t *testing.T) {
	now := sweepEpoch.Add(24 * time.Hour)
	for _, lag := range []time.Duration{0, 45 * time.Minute} {
		data := &market.Data{TimeframeData: map[string]*market.TimeframeSeriesData{}}
		for _, tf := range []string{"15m", "1h"} {
			bars := sweepTestBars(tf, 13)
			end := now
			if tf == "1h" {
				end = end.Add(-lag)
			}
			for i := range bars {
				bars[i].Time = end.Add(time.Duration(i-len(bars)) * tfDuration(tf)).UnixMilli()
			}
			data.TimeframeData[tf] = &market.TimeframeSeriesData{Klines: bars}
		}
		got := computeLiquiditySweep(data, now)
		if got == nil || got.SellsideSwept == nil || got.SellsideSwept.TF != "15m" {
			t.Fatalf("compare event close times across TFs: %+v", got)
		}
		// A 1h event at the latest close is newer than the 15m event one bar ago.
		data.TimeframeData["1h"].Klines = sweepTestBars("1h", 12)
		for i := range data.TimeframeData["1h"].Klines {
			data.TimeframeData["1h"].Klines[i].Time = now.Add(time.Duration(i-12) * time.Hour).UnixMilli()
		}
		got = computeLiquiditySweep(data, now)
		if got == nil || got.SellsideSwept == nil || got.SellsideSwept.TF != "1h" {
			t.Fatalf("newest hourly event must win: %+v", got)
		}
	}
	for _, tf := range []string{"15m", "1h"} {
		bars := sweepTestBars(tf, 16)
		bars[15].Low = 93
		got := sweepAtEnd(tf, bars)
		if got == nil || got.SellsideSwept == nil || got.SellsideSwept.Level != 94 || got.SellsideSwept.BarsAgo != 0 {
			t.Fatalf("%s latest same-TF event must win: %+v", tf, got)
		}
	}
}

func sweepCompatibilityData(now time.Time) *market.Data {
	data := &market.Data{Symbol: "SWEEPUSDT", CurrentPrice: 100, TimeframeData: map[string]*market.TimeframeSeriesData{}}
	for _, tf := range []string{"15m", "1h", "4h"} {
		dur := tfDuration(tf)
		bars := make([]market.KlineBar, 0, 74)
		for i := 0; i < 60; i++ {
			bars = append(bars, market.KlineBar{Open: 100, High: 110, Low: 99, Close: 100, Volume: 1000})
		}
		bars = append(bars, sweepTestBars(tf, 14)...)
		for i := range bars {
			bars[i].Time = now.Add(time.Duration(i-74) * dur).UnixMilli()
		}
		data.TimeframeData[tf] = &market.TimeframeSeriesData{Timeframe: tf, Klines: bars}
	}
	return data
}

func TestLiquiditySweepDisabledCompatibilityAndGateIsolation(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	data := sweepCompatibilityData(now)
	opt := SignalOptions{Now: now, PrimaryTF: "15m"}
	disabled, err := ComputeSymbolSignals(data.Symbol, data, opt)
	if err != nil {
		t.Fatal(err)
	}
	before := RenderSignalJSON(disabled)
	if strings.Contains(before, "liquidity_sweep") {
		t.Fatal("disabled JSON contains liquidity_sweep")
	}
	// Captured from the unmodified code before this feature was implemented.
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(before))); got != "d8a44aac26fb26fd372e937ed0c059816f237118c4f23ccabc877dddd1f5081e" {
		t.Fatalf("disabled JSON differs from pre-feature bytes: sha256=%s", got)
	}
	opt.EnableLiquiditySweep = true
	enabled, err := ComputeSymbolSignals(data.Symbol, data, opt)
	if err != nil {
		t.Fatal(err)
	}
	if enabled.LiquiditySweep == nil || !strings.Contains(RenderSignalJSON(enabled), "liquidity_sweep") {
		t.Fatal("enabled signal missing sweep evidence")
	}
	enabled.LiquiditySweep = nil
	if !reflect.DeepEqual(enabled, disabled) || RenderSignalJSON(enabled) != before {
		t.Fatal("enabling evidence changed other signal fields, gates, or entry/exit verdicts")
	}
	quiet := sweepTestData("15m", sweepTestBars("15m", 12))
	quiet.TimeframeData["15m"].Klines[11].Low = 99
	noEvent, err := ComputeSymbolSignals(quiet.Symbol, quiet, opt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(RenderSignalJSON(noEvent), "liquidity_sweep") {
		t.Fatal("no events must omit entire field")
	}
}

func TestLiquiditySweepPromptOptIn(t *testing.T) {
	for _, tc := range []struct {
		limit    bool
		baseline string
	}{
		{false, "b042604a9c85afb5cbe8b69a49395258ca2536bd97dc20a68e13acda6fd18ac7"},
		{true, "082cf65a2cafabeefb1c01bcc72bf0f0eca5182ea62a15cbcbf10e422d781e78"},
	} {
		cfg := &store.StrategyConfig{Language: "zh"}
		cfg.RiskControl.LimitEntryEnabled = tc.limit
		disabled := NewStrategyEngine(cfg).BuildSystemPrompt(100, "")
		if strings.Contains(disabled, "liquidity_sweep") {
			t.Fatal("disabled prompt contains liquidity_sweep")
		}
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(disabled))); got != tc.baseline {
			t.Fatalf("disabled prompt differs from pre-feature bytes: limit=%v sha256=%s", tc.limit, got)
		}
		cfg.Indicators.EnableLiquiditySweep = true
		enabled := NewStrategyEngine(cfg).BuildSystemPrompt(100, "")
		var addition string
		for _, line := range strings.SplitAfter(enabled, "\n") {
			if strings.HasPrefix(line, "- liquidity_sweep ") {
				addition += line
			}
		}
		for _, want := range []string{"程序预计算", "sellside_swept 支持做多", "buyside_swept 支持做空", "mss_confirmed=true", "不是独立入场理由", "hard_entry_gate / rr_scan", "没有该字段表示近期无扫荡"} {
			if !strings.Contains(addition, want) {
				t.Fatalf("legend missing %q", want)
			}
		}
		if strings.Replace(enabled, addition, "", 1) != disabled {
			t.Fatal("opt-in changed existing prompt bytes")
		}
	}
}

func TestLiquiditySweepICTReference(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Name      string       `json:"name"`
			TF        string       `json:"tf"`
			Bars5m    [][5]float64 `json:"bars_5m"`
			Snapshots []struct {
				ClosedBars int             `json:"closed_bars"`
				Expected   *LiquiditySweep `json:"expected"`
			} `json:"snapshots"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/liquidity_sweep_ict.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("empty ICT fixture")
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			dur := tfDuration(tc.TF)
			count := int(dur / (5 * time.Minute))
			if count < 1 || len(tc.Bars5m)%count != 0 {
				t.Fatal("incomplete fixture buckets")
			}
			var bars []market.KlineBar
			for i := 0; i < len(tc.Bars5m); i += count {
				first := tc.Bars5m[i]
				bar := market.KlineBar{Time: int64(first[0]), Open: first[1], High: first[2], Low: first[3], Close: tc.Bars5m[i+count-1][4]}
				for j := i; j < i+count; j++ {
					row := tc.Bars5m[j]
					if int64(row[0]) != bar.Time+int64(j-i)*300_000 {
						t.Fatal("gapped fixture bucket")
					}
					bar.High, bar.Low = math.Max(bar.High, row[2]), math.Min(bar.Low, row[3])
				}
				bars = append(bars, bar)
			}
			for _, snapshot := range tc.Snapshots {
				n := snapshot.ClosedBars
				if n < 1 || n > len(bars) {
					t.Fatalf("invalid prefix %d", n)
				}
				now := time.UnixMilli(bars[n-1].Time).Add(dur)
				got := computeLiquiditySweep(sweepTestData(tc.TF, bars[:n]), now)
				if !reflect.DeepEqual(got, snapshot.Expected) {
					actual, _ := json.Marshal(got)
					want, _ := json.Marshal(snapshot.Expected)
					t.Fatalf("closed_bars=%d: got %s, ict.py wants %s", n, actual, want)
				}
			}
		})
	}
}
