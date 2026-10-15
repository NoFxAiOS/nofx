package stockengine

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"nofx/market/usstock"
	"nofx/store"
)

func TestSystemPromptsBandsAndContract(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, name := range []string{store.StockPresetSwing, store.StockPresetPosition} {
			cfg := &store.StockConfig{Preset: name}
			p := ResolvePreset(cfg)
			prompt := BuildSystemPrompt(cfg, p, lang)
			band := "[1.50, 3.00]"
			if name == store.StockPresetPosition {
				band = "[2.00, 4.00]"
			}
			for _, token := range []string{band, "JSON", "OUTPUT CONTRACT", "quantity", "Binance bStock", "1R", "symbol", "action", "entry_type", "limit_price", "stop_loss", "take_profit", "reduce_fraction", "confidence", "reasoning"} {
				if !strings.Contains(prompt, token) {
					t.Fatalf("%s %s prompt missing %s", lang, name, token)
				}
			}
		}
		cfg := &store.StockConfig{StopATRMin: 1.75, StopATRMax: 2.25}
		if prompt := BuildSystemPrompt(cfg, ResolvePreset(cfg), lang); !strings.Contains(prompt, "[1.75, 2.25]") {
			t.Fatal(prompt)
		}
	}
	// Example uses exactly the contract's tags; no quantity field.
	var examples []map[string]any
	if err := json.Unmarshal([]byte(decisionContract), &examples); err != nil {
		t.Fatal(err)
	}
	if len(examples[0]) != 9 {
		t.Fatal(examples)
	}
}

func TestUserPromptSourcesPositionsAndR(t *testing.T) {
	c := testContext()
	holding(c)
	c.Account.Exposure = 500
	c.Snapshots[0].Underlying = "AAPL"
	c.Snapshots[0].Sources[usstock.TF1h] = usstock.SourceBStock
	// 1d is required by every preset → warned; 15m is optional for swing → not.
	c.Snapshots[0].MissingTF = []string{usstock.TF15m, usstock.TF1d}
	c.Market = []*SymbolSnapshot{{Symbol: "SPYBUSDT", Underlying: "SPY", TrendDaily: "up"}, {Symbol: "QQQBUSDT", Underlying: "QQQ", TrendDaily: "range"}}
	for _, lang := range []string{"zh", "en"} {
		prompt := BuildUserPrompt(c, lang)
		for _, token := range []string{"2026-10-09 10:00:00 EDT", "session=regular", "equity=10000.00", "available=10000.00", "(5.00%)", "qty=5.000000", "avg=90.00", "unrealized=11.11%", "R=2.00R", "held_days=2.0", "live_stop=94.00", "TP=0.00", "Yahoo", "1h: bStock", "MissingTF", "SPYBUSDT", "QQQBUSDT", "divergence=0.00%", "fresh=true"} {
			if !strings.Contains(prompt, token) {
				t.Fatalf("%s user prompt missing %s:\n%s", lang, token, prompt)
			}
		}
		if !strings.Contains(prompt, "MissingTF WARNING: 1d (") {
			t.Fatalf("%s: only the required 1d should be warned:\n%s", lang, prompt)
		}
		if lang == "zh" && !strings.Contains(prompt, "1d: Yahoo 正股 AAPL") {
			t.Fatal(prompt)
		}
	}
	c.Now = time.Date(2026, 1, 9, 15, 0, 0, 0, time.UTC)
	if prompt := BuildUserPrompt(c, "en"); !strings.Contains(prompt, "10:00:00 EST") {
		t.Fatal(prompt)
	}
}

func TestUserPromptMissingData(t *testing.T) {
	c := testContext()
	holding(c)
	c.Positions[0].InitialStop = 0
	c.Snapshots = nil
	p := BuildUserPrompt(c, "en")
	if !strings.Contains(p, "R=n/a") || !strings.Contains(p, "DATA_INSUFFICIENT") {
		t.Fatal(p)
	}
}

func TestSystemPromptCycleMethodAndScanNote(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		cfg := &store.StockConfig{}
		manual := BuildSystemPrompt(cfg, ResolvePreset(cfg), lang)
		for _, token := range []string{"Kell", "wedge_pop", "ema_crossback", "base_n_break", "reversal_extension", "base_forming", "exhaustion_extension", "wedge_drop", "downtrend", "EMA10/20", "OUTPUT CONTRACT"} {
			if !strings.Contains(manual, token) {
				t.Fatalf("%s manual prompt missing %s", lang, token)
			}
		}
		scanNote, manualPicks := "cycle scanner", "manual picks"
		if lang == "zh" {
			scanNote, manualPicks = "周期扫描器", "用户手动指定"
		}
		if strings.Contains(manual, scanNote) {
			t.Fatalf("%s manual prompt must not carry the scan note", lang)
		}
		cfg = &store.StockConfig{SymbolSource: store.StockSymbolSourceScan}
		scan := BuildSystemPrompt(cfg, ResolvePreset(cfg), lang)
		if !strings.Contains(scan, scanNote) || !strings.Contains(scan, "Kell") {
			t.Fatalf("%s scan prompt missing scan note:\n%s", lang, scan)
		}
		if strings.Contains(scan, manualPicks) {
			t.Fatalf("%s scan-only prompt must not mention the manual picks:\n%s", lang, scan)
		}
		cfg = &store.StockConfig{SymbolSource: store.StockSymbolSourceHybrid}
		hybrid := BuildSystemPrompt(cfg, ResolvePreset(cfg), lang)
		if !strings.Contains(hybrid, scanNote) || !strings.Contains(hybrid, manualPicks) {
			t.Fatalf("%s hybrid prompt missing scan note or manual picks:\n%s", lang, hybrid)
		}
	}
}

func TestUserPromptCycleLine(t *testing.T) {
	c := testContext()
	c.Snapshots[0].Cycle = &CycleAssessment{Phase: PhaseWedgePop, LongSetup: true, Score: 72.46, Ext10ATR: 0.84, Dist20ATR: -0.3,
		RS63Pct: 12.5, VolRatio: 1.678, Notes: []string{"first reclaim", "tight range"}}
	for lang, label := range map[string]string{"en": "cycle(Kell, daily):", "zh": "价格周期(Kell, 日线):"} {
		prompt := BuildUserPrompt(c, lang)
		want := label + " phase=wedge_pop long_setup=true score=72.5 ext10=+0.8ATR dist20=-0.3ATR rs63_vs_SPY=+12.5% vol=1.68x first reclaim; tight range"
		if !strings.Contains(prompt, want) {
			t.Fatalf("%s user prompt missing cycle line %q:\n%s", lang, want, prompt)
		}
		if strings.Count(prompt, "phase=") != 1 {
			t.Fatalf("%s: snapshots without a cycle must not print a cycle line:\n%s", lang, prompt)
		}
	}
}

func TestSystemPromptMomentumRank(t *testing.T) {
	neverKell := map[string]string{"en": "never open at exhaustion_extension, wedge_drop or downtrend", "zh": "exhaustion_extension、wedge_drop、downtrend 阶段绝不开仓"}
	neverMom := map[string]string{"en": "never open at wedge_drop or downtrend; at exhaustion_extension do not chase", "zh": "wedge_drop、downtrend 阶段绝不开仓，exhaustion_extension 阶段不追高"}
	note := map[string]string{"en": "12-1 momentum among names above their 200-day SMA", "zh": "12-1 动量从高到低"}
	for _, lang := range []string{"zh", "en"} {
		for _, source := range []string{store.StockSymbolSourceScan, store.StockSymbolSourceHybrid} {
			kell := &store.StockConfig{SymbolSource: source}
			prompt := BuildSystemPrompt(kell, ResolvePreset(kell), lang)
			if !strings.Contains(prompt, neverKell[lang]) || strings.Contains(prompt, neverMom[lang]) || strings.Contains(prompt, note[lang]) {
				t.Fatalf("%s/%s kell-rank prompt must keep the original text only:\n%s", lang, source, prompt)
			}
			explicit := &store.StockConfig{SymbolSource: source, ScanRank: store.StockScanRankKell}
			if BuildSystemPrompt(explicit, ResolvePreset(explicit), lang) != prompt {
				t.Fatalf("%s/%s: explicit scan_rank=kell must equal the default prompt", lang, source)
			}
			mom := &store.StockConfig{SymbolSource: source, ScanRank: store.StockScanRankMomentum}
			prompt = BuildSystemPrompt(mom, ResolvePreset(mom), lang)
			if !strings.Contains(prompt, neverMom[lang]) || strings.Contains(prompt, neverKell[lang]) || !strings.Contains(prompt, note[lang]) || !strings.Contains(prompt, "EMA10/EMA20") {
				t.Fatalf("%s/%s momentum-rank prompt wrong:\n%s", lang, source, prompt)
			}
		}
		// Manual mode ignores scan_rank entirely.
		manual := &store.StockConfig{ScanRank: store.StockScanRankMomentum}
		plain := &store.StockConfig{}
		if BuildSystemPrompt(manual, ResolvePreset(manual), lang) != BuildSystemPrompt(plain, ResolvePreset(plain), lang) {
			t.Fatalf("%s: manual mode prompt must not depend on scan_rank", lang)
		}
	}
}

func TestUserPromptMomentumLine(t *testing.T) {
	c := testContext()
	c.Snapshots[0].Momentum = &MomentumAssessment{Valid: true, Mom12_1Pct: 42.34, AboveSMA200: true}
	c.Snapshots[1].Momentum = &MomentumAssessment{} // invalid: not printed
	for _, lang := range []string{"en", "zh"} {
		prompt := BuildUserPrompt(c, lang)
		if !strings.Contains(prompt, "mom12_1=+42.3% above_sma200=true") {
			t.Fatalf("%s: momentum line missing:\n%s", lang, prompt)
		}
		if strings.Count(prompt, "mom12_1=") != 1 {
			t.Fatalf("%s: invalid or absent momentum must not print:\n%s", lang, prompt)
		}
	}
}
