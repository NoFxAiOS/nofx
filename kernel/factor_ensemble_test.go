package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"nofx/market"
	"nofx/store"
)

var factorEnsembleTestNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func factorEnsembleFixture(asOf string) string {
	return fmt.Sprintf(`{"version":1,"market":"crypto","generated_at":"2026-10-20T00:00:00+00:00","as_of":%q,"universe_size":2,"horizons":{
		"1":{"factor_count":4,"forward":{"inconclusive":4},"scores":{"BTCUSDT":{"z":0.83,"pct":0.91}}},
		"3":{"factor_count":2,"forward":{"negative":1,"inconclusive":1},"scores":{"BTCUSDT":{"z":-0.2,"pct":0.25}}},
		"7":{"factor_count":1,"forward":{"inconclusive":1},"scores":{"ETHUSDT":{"z":0,"pct":0}}}}}`, asOf)
}

func factorEnsembleTestFile(t *testing.T, raw string) string {
	t.Helper()
	t.Setenv("FACTOR_MINING_DIR", t.TempDir())
	path := factorEnsembleScoresPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFactorEnsemblePath(t *testing.T) {
	t.Setenv("FACTOR_MINING_DIR", "")
	if got := factorEnsembleScoresPath(); got != filepath.Join("data", "factor_mining", "crypto", "live_scores.json") {
		t.Fatalf("default path = %q", got)
	}
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	if got := factorEnsembleScoresPath(); got != filepath.Join(dir, "crypto", "live_scores.json") {
		t.Fatalf("injected path = %q", got)
	}
}

func TestFactorEnsembleInvalidFiles(t *testing.T) {
	valid := factorEnsembleFixture("2026-10-19")
	for _, tc := range []struct{ name, raw string }{
		{"missing", valid},
		{"directory", valid},
		{"bad_json", "{"},
		{"trailing_json", valid + "{}"},
		{"null", "null"},
		{"wrong_version", strings.Replace(valid, `"version":1`, `"version":2`, 1)},
		{"wrong_market", strings.Replace(valid, `"market":"crypto"`, `"market":"stocks"`, 1)},
		{"bad_as_of", factorEnsembleFixture("not-a-date")},
		{"stale_as_of", factorEnsembleFixture("2026-10-16")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := factorEnsembleTestFile(t, tc.raw)
			if tc.name == "missing" || tc.name == "directory" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if tc.name == "directory" {
					if err := os.Mkdir(path, 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			if got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow); got != nil {
				t.Fatalf("invalid file returned evidence: %+v", got)
			}
		})
	}
}

func TestFactorEnsembleSymbolAndHorizons(t *testing.T) {
	factorEnsembleTestFile(t, factorEnsembleFixture("2026-10-19"))
	want := &FactorEnsembleRef{AsOf: "2026-10-19", Horizons: map[string]FactorHorizonRef{
		"1": {Pct: 0.91, Z: 0.83, Factors: 4, Forward: "inconclusive×4"},
		"3": {Pct: 0.25, Z: -0.2, Factors: 2, Forward: "inconclusive×1,negative×1"},
	}}
	for i := 0; i < 10; i++ {
		if got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow); !reflect.DeepEqual(got, want) {
			t.Fatalf("BTC reference = %+v, want %+v", got, want)
		}
	}
	eth := computeFactorEnsemble(&market.Data{Symbol: "ETHUSDT"}, factorEnsembleTestNow)
	if eth == nil || len(eth.Horizons) != 1 || eth.Horizons["7"] != (FactorHorizonRef{Factors: 1, Forward: "inconclusive×1"}) {
		t.Fatalf("only the symbol's horizons should appear; zero scores are valid: %+v", eth)
	}
	for _, data := range []*market.Data{nil, {}, {Symbol: "XRPUSDT"}, {Symbol: "btcusdt"}} {
		if got := computeFactorEnsemble(data, factorEnsembleTestNow); got != nil {
			t.Fatalf("unknown symbol returned evidence: %+v", got)
		}
	}
}

func TestFactorEnsembleUTCDateExpiryOnCacheHit(t *testing.T) {
	factorEnsembleTestFile(t, factorEnsembleFixture("2026-10-17"))
	for _, tc := range []struct {
		at   time.Time
		want bool
	}{
		{time.Date(2026, 10, 20, 23, 59, 59, 0, time.UTC), true},
		{time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), false},
		{time.Date(2026, 10, 21, 7, 59, 59, 0, time.FixedZone("UTC+8", 8*3600)), true},
		{time.Date(2026, 10, 21, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)), false},
	} {
		if got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, tc.at); (got != nil) != tc.want {
			t.Fatalf("at %s: reference = %+v, want valid=%v", tc.at, got, tc.want)
		}
	}
}

func TestFactorEnsembleInvalidScoresOnlyDropTheirHorizon(t *testing.T) {
	// JSON has no NaN/Infinity literals. Exponent overflow represents a
	// non-finite float; quoted variants must also never reach the prompt.
	for _, score := range []string{
		`{"pct":1e999,"z":0}`, `{"pct":-1e999,"z":0}`,
		`{"pct":0.5,"z":1e999}`, `{"pct":0.5,"z":-1e999}`,
		`{"pct":"NaN","z":0}`, `{"pct":0.5,"z":"Infinity"}`,
		`{"pct":null,"z":0}`, `{"pct":0.5}`, `{"pct":-0.1,"z":0}`, `{"pct":1.1,"z":0}`,
	} {
		t.Run(score, func(t *testing.T) {
			raw := strings.Replace(factorEnsembleFixture("2026-10-19"), `{"z":0.83,"pct":0.91}`, score, 1)
			factorEnsembleTestFile(t, raw)
			got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow)
			if got == nil || len(got.Horizons) != 1 || got.Horizons["3"].Pct != 0.25 {
				t.Fatalf("invalid score discarded a valid horizon: %+v", got)
			}
			if _, err := json.Marshal(got); err != nil {
				t.Fatalf("reference must serialize: %v", err)
			}
		})
	}
	raw := strings.ReplaceAll(factorEnsembleFixture("2026-10-19"), `"BTCUSDT"`, `"OTHERUSDT"`)
	raw = strings.Replace(raw, `"ETHUSDT":{"z":0,"pct":0}`, `"BTCUSDT":{"z":1e999,"pct":0}`, 1)
	factorEnsembleTestFile(t, raw)
	if got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow); got != nil {
		t.Fatalf("all invalid horizons must omit the reference: %+v", got)
	}
}

func TestFactorEnsembleCacheRefreshAndInvalidation(t *testing.T) {
	valid := factorEnsembleFixture("2026-10-19")
	path := factorEnsembleTestFile(t, valid)
	write := func(raw string, seconds int) {
		t.Helper()
		if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
		mtime := factorEnsembleTestNow.Add(time.Duration(seconds) * time.Second)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want float64) {
		t.Helper()
		got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow)
		if got == nil || got.Horizons["1"].Pct != want {
			t.Fatalf("cached reference = %+v, want pct=%v", got, want)
		}
	}
	write(valid, 0)
	check(0.91)
	changed := strings.Replace(valid, `"pct":0.91`, `"pct":0.75`, 1)
	write(changed, 0)
	check(0.91) // unchanged mtime reuses the snapshot
	write(changed, 1)
	check(0.75)
	write("{", 2)
	if got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow); got != nil {
		t.Fatal("invalid replacement served the previous valid snapshot")
	}
	write(valid, 2)
	if got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow); got != nil {
		t.Fatal("invalid snapshot was not cached by mtime")
	}
	write(valid, 3)
	check(0.91)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow); got != nil {
		t.Fatal("missing replacement served the previous valid snapshot")
	}
	write(changed, 3)
	check(0.75)
	// Equal mtimes on different injected paths must not share a snapshot.
	path = factorEnsembleTestFile(t, valid)
	write(valid, 3)
	check(0.91)
}

func TestFactorEnsembleFileSizeLimit(t *testing.T) {
	valid := factorEnsembleFixture("2026-10-19")
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprintf("extra=%d", extra), func(t *testing.T) {
			factorEnsembleTestFile(t, valid+strings.Repeat(" ", factorEnsembleMaxFileSize-len(valid)+extra))
			got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow)
			if (got != nil) != (extra == 0) {
				t.Fatalf("8 MiB + %d bytes: reference = %+v", extra, got)
			}
		})
	}
}

func TestFactorEnsembleConcurrentReaders(t *testing.T) {
	factorEnsembleTestFile(t, factorEnsembleFixture("2026-10-19"))
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				got := computeFactorEnsemble(&market.Data{Symbol: "BTCUSDT"}, factorEnsembleTestNow)
				if got == nil || got.Horizons["1"].Pct != 0.91 {
					t.Errorf("concurrent reference = %+v", got)
					return
				}
				delete(got.Horizons, "1") // caller-owned maps cannot mutate the cache
			}
		}()
	}
	wg.Wait()
}

func TestFactorEnsembleDisabledCompatibilityAndGateIsolation(t *testing.T) {
	factorEnsembleTestFile(t, factorEnsembleFixture("2026-10-19"))
	data := sweepCompatibilityData(factorEnsembleTestNow)
	data.Symbol = "BTCUSDT"
	opt := SignalOptions{Now: factorEnsembleTestNow, PrimaryTF: "15m"}
	disabled, err := ComputeSymbolSignals(data.Symbol, data, opt)
	if err != nil {
		t.Fatal(err)
	}
	before := RenderSignalJSON(disabled)
	if disabled.FactorEnsemble != nil || strings.Contains(before, "factor_ensemble") {
		t.Fatal("disabled signal contains factor_ensemble")
	}
	opt.EnableFactorEnsemble = true
	enabled, err := ComputeSymbolSignals(data.Symbol, data, opt)
	if err != nil {
		t.Fatal(err)
	}
	if enabled.FactorEnsemble == nil || !strings.Contains(RenderSignalJSON(enabled), `"factor_ensemble":{"as_of":`) {
		t.Fatal("enabled signal missing factor evidence or snake_case as_of")
	}
	enabled.FactorEnsemble = nil
	if !reflect.DeepEqual(enabled, disabled) || RenderSignalJSON(enabled) != before {
		t.Fatal("factor reference changed another signal field, score, gate, or entry/exit verdict")
	}
	// The file key comes from data.Symbol, even when the display symbol differs.
	alias, err := ComputeSymbolSignals("DISPLAYUSDT", data, opt)
	if err != nil || alias.FactorEnsemble == nil {
		t.Fatalf("lookup must use data.Symbol: signal=%+v, err=%v", alias, err)
	}
	data.Symbol = "XRPUSDT"
	unknown, err := ComputeSymbolSignals(data.Symbol, data, opt)
	if err != nil || unknown.FactorEnsemble != nil || strings.Contains(RenderSignalJSON(unknown), "factor_ensemble") {
		t.Fatalf("unknown symbol must omit factor_ensemble: signal=%+v, err=%v", unknown, err)
	}
}

func TestFactorEnsembleConfigDefaultsAndRoundTrip(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		if store.GetDefaultStrategyConfig(lang).Indicators.EnableFactorEnsemble {
			t.Fatalf("%s default enabled factor ensemble", lang)
		}
	}
	for _, raw := range []string{`{}`, `{"indicators":{"enable_ema":true}}`, `{"indicators":{"enable_factor_ensemble":false}}`, `{"indicators":{"enable_factor_ensemble":true}}`} {
		var cfg store.StrategyConfig
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			t.Fatal(err)
		}
		want := strings.Contains(raw, `"enable_factor_ensemble":true`)
		encoded, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Indicators.EnableFactorEnsemble != want || strings.Contains(string(encoded), "enable_factor_ensemble") != want {
			t.Fatalf("incorrect default or serialization: %s", encoded)
		}
		var restored store.StrategyConfig
		if err := json.Unmarshal(encoded, &restored); err != nil || restored.Indicators.EnableFactorEnsemble != want {
			t.Fatalf("opt-in lost in round trip: %+v, %v", restored.Indicators, err)
		}
	}
}

func TestFactorEnsemblePromptOptIn(t *testing.T) {
	for _, limit := range []bool{false, true} {
		for _, sweep := range []bool{false, true} {
			cfg := &store.StrategyConfig{Language: "zh"}
			cfg.RiskControl.LimitEntryEnabled = limit
			cfg.Indicators.EnableLiquiditySweep = sweep
			disabled := NewStrategyEngine(cfg).BuildSystemPrompt(100, "")
			if strings.Contains(disabled, "factor_ensemble") {
				t.Fatal("disabled prompt contains factor_ensemble")
			}
			cfg.Indicators.EnableFactorEnsemble = true
			enabled := NewStrategyEngine(cfg).BuildSystemPrompt(100, "")
			var addition string
			for _, line := range strings.SplitAfter(enabled, "\n") {
				if strings.HasPrefix(line, "- factor_ensemble ") {
					addition += line
				}
			}
			for _, want := range []string{"程序每日预计算的多因子截面参考", "pct 越大越被因子看多、越小越看空", "按 horizon 天数", "前向校验仍未证明有效", "仅是参考证据", "不是独立入场理由", "不能绕过 hard_entry_gate / rr_scan", "没有该字段表示该币不在因子宇宙或数据过期"} {
				if !strings.Contains(addition, want) {
					t.Fatalf("legend missing %q", want)
				}
			}
			if strings.Count(addition, "\n") != 1 || strings.Replace(enabled, addition, "", 1) != disabled {
				t.Fatal("opt-in must add exactly one line and preserve existing prompt bytes")
			}
		}
	}
}

func TestFactorEnsembleEngineWiring(t *testing.T) {
	now := time.Now()
	factorEnsembleTestFile(t, factorEnsembleFixture(now.UTC().Format(time.DateOnly)))
	old := binanceGetFn
	t.Cleanup(func() { binanceGetFn = old })
	binanceGetFn = func(context.Context, string, interface{}) error { return fmt.Errorf("offline factor test") }
	cfg := store.GetDefaultStrategyConfig("zh")
	cfg.Indicators.Klines.PrimaryTimeframe = "15m"
	data := sweepCompatibilityData(now)
	data.Symbol = "BTCUSDT"
	for _, enabled := range []bool{false, true} {
		cfg.Indicators.EnableFactorEnsemble = enabled
		sig := NewStrategyEngine(&cfg).computeCoinSignal(data, nil, &Context{}, nil)
		if sig == nil || (sig.FactorEnsemble != nil) != enabled {
			t.Fatalf("engine opt-in=%v: signal=%+v", enabled, sig)
		}
	}
}
