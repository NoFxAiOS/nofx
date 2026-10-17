package store

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func stockBool(b bool) *bool { return &b }

func validStock() *StockConfig {
	return &StockConfig{Symbols: []string{"AAPLBUSDT", "SPYBUSDT"}}
}

func TestStockConfigResolvers_Defaults(t *testing.T) {
	for name, s := range map[string]*StockConfig{"nil": nil, "zero": {}} {
		t.Run(name, func(t *testing.T) {
			if s.EffectivePreset() != StockPresetSwing {
				t.Errorf("preset = %q", s.EffectivePreset())
			}
			if !s.AllowRegular() || s.AllowPreMarket() || s.AllowAfterHours() {
				t.Errorf("sessions regular/pre/after = %v/%v/%v", s.AllowRegular(), s.AllowPreMarket(), s.AllowAfterHours())
			}
			if !s.YahooFallback() || !s.IsPaper() {
				t.Errorf("yahoo=%v paper=%v", s.YahooFallback(), s.IsPaper())
			}
			if s.EffectiveMaxDivergencePct() != 1.0 || s.EffectiveMaxPositionPct() != 20 ||
				s.EffectiveMaxTotalExposurePct() != 80 || s.EffectiveMaxPositions() != 5 ||
				s.EffectiveRiskPerTradePct() != 1.0 {
				t.Errorf("risk defaults wrong")
			}
			if lo, hi := s.EffectiveStopATRBand(); lo != 1.5 || hi != 3.0 {
				t.Errorf("swing band = %v-%v", lo, hi)
			}
		})
	}
}

func TestStockConfigSymbolSourceHelpers(t *testing.T) {
	var nilCfg *StockConfig
	for name, tc := range map[string]struct {
		cfg                *StockConfig
		source             string
		scanMode, manualOK bool
	}{
		"nil":    {nilCfg, StockSymbolSourceManual, false, true},
		"empty":  {&StockConfig{}, StockSymbolSourceManual, false, true},
		"manual": {&StockConfig{SymbolSource: StockSymbolSourceManual}, StockSymbolSourceManual, false, true},
		"scan":   {&StockConfig{SymbolSource: StockSymbolSourceScan}, StockSymbolSourceScan, true, false},
		"hybrid": {&StockConfig{SymbolSource: StockSymbolSourceHybrid}, StockSymbolSourceHybrid, true, true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.cfg.EffectiveSymbolSource(); got != tc.source {
				t.Errorf("EffectiveSymbolSource() = %q, want %q", got, tc.source)
			}
			if got := tc.cfg.IsScanMode(); got != tc.scanMode {
				t.Errorf("IsScanMode() = %v, want %v", got, tc.scanMode)
			}
			if got := tc.cfg.UsesManualSymbols(); got != tc.manualOK {
				t.Errorf("UsesManualSymbols() = %v, want %v", got, tc.manualOK)
			}
		})
	}
}

func TestStockConfigScanHelpers(t *testing.T) {
	var nilCfg *StockConfig
	if nilCfg.IsScanMode() || nilCfg.EffectiveScanTopN() != DefaultStockScanTopN {
		t.Fatal("nil config: manual mode, default top N")
	}
	for _, c := range []struct {
		n, want int
	}{{0, 10}, {-3, 10}, {7, 7}, {50, 50}, {80, 50}} {
		if got := (&StockConfig{ScanTopN: c.n}).EffectiveScanTopN(); got != c.want {
			t.Errorf("EffectiveScanTopN(%d) = %d, want %d", c.n, got, c.want)
		}
	}
	if (&StockConfig{}).IsScanMode() || (&StockConfig{SymbolSource: StockSymbolSourceManual}).IsScanMode() {
		t.Fatal("manual must not be scan mode")
	}
	if !(&StockConfig{SymbolSource: StockSymbolSourceScan}).IsScanMode() {
		t.Fatal("scan mode not detected")
	}
}

func TestStockConfigScanRank(t *testing.T) {
	var nilCfg *StockConfig
	if nilCfg.EffectiveScanRank() != StockScanRankKell || (&StockConfig{}).EffectiveScanRank() != StockScanRankKell {
		t.Fatal("nil and empty scan_rank must resolve to kell")
	}
	if (&StockConfig{ScanRank: StockScanRankKell}).EffectiveScanRank() != StockScanRankKell ||
		(&StockConfig{ScanRank: StockScanRankMomentum}).EffectiveScanRank() != StockScanRankMomentum {
		t.Fatal("explicit scan_rank not honored")
	}
	data, _ := json.Marshal(&StockConfig{Symbols: []string{"AAPLBUSDT"}})
	if strings.Contains(string(data), "scan_rank") {
		t.Fatalf("empty scan_rank must be omitted from JSON: %s", data)
	}
}

func TestStockConfigResolvers_Overrides(t *testing.T) {
	s := &StockConfig{
		Preset:              StockPresetPosition,
		Sessions:            StockSessions{Regular: stockBool(false), PreMarket: true, AfterHours: true},
		DataFallbackYahoo:   stockBool(false),
		MaxDivergencePct:    2.5,
		MaxPositionPct:      10,
		MaxTotalExposurePct: 50,
		MaxPositions:        3,
		RiskPerTradePct:     0.5,
		PaperTrading:        stockBool(false),
	}
	if s.EffectivePreset() != StockPresetPosition || s.AllowRegular() || !s.AllowPreMarket() || !s.AllowAfterHours() {
		t.Errorf("preset/sessions override not honored")
	}
	if s.YahooFallback() || s.IsPaper() {
		t.Errorf("yahoo/paper override not honored")
	}
	if s.EffectiveMaxDivergencePct() != 2.5 || s.EffectiveMaxPositionPct() != 10 ||
		s.EffectiveMaxTotalExposurePct() != 50 || s.EffectiveMaxPositions() != 3 ||
		s.EffectiveRiskPerTradePct() != 0.5 {
		t.Errorf("numeric overrides not honored")
	}
	// position preset defaults, then per-end overrides
	if lo, hi := s.EffectiveStopATRBand(); lo != 2.0 || hi != 4.0 {
		t.Errorf("position band = %v-%v", lo, hi)
	}
	s.StopATRMin = 2.5
	if lo, hi := s.EffectiveStopATRBand(); lo != 2.5 || hi != 4.0 {
		t.Errorf("min override band = %v-%v", lo, hi)
	}
	s.StopATRMin, s.StopATRMax = 0, 3.5
	if lo, hi := s.EffectiveStopATRBand(); lo != 2.0 || hi != 3.5 {
		t.Errorf("max override band = %v-%v", lo, hi)
	}
	// unknown preset resolves to swing
	if (&StockConfig{Preset: "bogus"}).EffectivePreset() != StockPresetSwing {
		t.Errorf("unknown preset must resolve to swing")
	}
}

func TestStockConfigRegimeResolvers(t *testing.T) {
	var nilCfg *StockConfig
	if nilCfg.RegimeEnabled() || nilCfg.EffectiveRegimeScaler() != StockRegimeOff ||
		nilCfg.EffectiveRegimeTargetVolPct() != 40 || nilCfg.EffectiveRegimeFloor() != 0.25 || !nilCfg.RegimeTrimEnabled() {
		t.Errorf("nil config regime defaults wrong")
	}
	s := &StockConfig{}
	if s.RegimeEnabled() || s.EffectiveRegimeScaler() != StockRegimeOff {
		t.Errorf("regime must default to off")
	}
	s.RegimeScaler = StockRegimeOff
	if s.RegimeEnabled() {
		t.Errorf("explicit off must stay off")
	}
	s.RegimeScaler = "bogus" // Validate rejects it at save time; resolves to off
	if s.RegimeEnabled() {
		t.Errorf("unknown scaler must resolve to off")
	}
	s = &StockConfig{RegimeScaler: StockRegimeBasketVol, RegimeTargetVolPct: 55, RegimeFloor: 0.4, RegimeTrim: stockBool(false)}
	if !s.RegimeEnabled() || s.EffectiveRegimeScaler() != StockRegimeBasketVol ||
		s.EffectiveRegimeTargetVolPct() != 55 || s.EffectiveRegimeFloor() != 0.4 || s.RegimeTrimEnabled() {
		t.Errorf("regime overrides not honored")
	}
	s = &StockConfig{RegimeScaler: StockRegimeBasketVol}
	if s.EffectiveRegimeTargetVolPct() != 40 || s.EffectiveRegimeFloor() != 0.25 || !s.RegimeTrimEnabled() {
		t.Errorf("regime defaults wrong")
	}
	// JSON names are the frontend contract.
	var parsed StockConfig
	raw := `{"symbols":["AAPLBUSDT"],"regime_scaler":"basket_vol","regime_target_vol_pct":35,"regime_floor":0.3,"regime_trim":false}`
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.RegimeScaler != StockRegimeBasketVol || parsed.RegimeTargetVolPct != 35 || parsed.RegimeFloor != 0.3 || parsed.RegimeTrimEnabled() {
		t.Errorf("regime JSON tags wrong: %+v", parsed)
	}
	// Default (off) config serializes without any regime keys.
	out, _ := json.Marshal(&StockConfig{Symbols: []string{"AAPLBUSDT"}})
	if strings.Contains(string(out), "regime") {
		t.Errorf("off config must omit regime fields: %s", out)
	}
}

func TestStockConfigResolvers_DivergenceDisabled(t *testing.T) {
	if got := (&StockConfig{MaxDivergencePct: -3}).EffectiveMaxDivergencePct(); got != -1 {
		t.Errorf("negative divergence = %v, want -1", got)
	}
}

func TestStockConfigValidate(t *testing.T) {
	if err := validStock().Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	many := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		many = append(many, fmt.Sprintf("S%dBUSDT", i))
	}

	cases := []struct {
		name    string
		mutate  func(*StockConfig)
		wantErr string // empty = must pass
	}{
		{"60 symbols ok", func(s *StockConfig) { s.Symbols = many }, ""},
		{"manual source explicit", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceManual }, ""},
		{"manual source empty symbols", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceManual; s.Symbols = nil }, "at least one symbol"},
		{"unknown source", func(s *StockConfig) { s.SymbolSource = "auto" }, "symbol_source"},
		{"unknown source names every mode", func(s *StockConfig) { s.SymbolSource = "auto" }, `must be "manual", "scan" or "hybrid"`},
		{"scan with no symbols ok", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceScan; s.Symbols = nil }, ""},
		{"scan with pins and exclude ok", func(s *StockConfig) {
			s.SymbolSource = StockSymbolSourceScan
			s.ScanTopN = 50
			s.ScanExclude = []string{"TSLABUSDT", "NVDABUSDT"}
		}, ""},
		{"scan with symbols ok", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceScan }, ""},
		{"scan lowercase symbol still checked", func(s *StockConfig) {
			s.SymbolSource = StockSymbolSourceScan
			s.Symbols = []string{"aaplbusdt"}
		}, "invalid stock symbol"},
		{"scan_rank empty ok", func(s *StockConfig) { s.ScanRank = "" }, ""},
		{"scan_rank kell ok", func(s *StockConfig) { s.ScanRank = StockScanRankKell }, ""},
		{"scan_rank momentum ok in manual", func(s *StockConfig) { s.ScanRank = StockScanRankMomentum }, ""},
		{"scan_rank momentum ok in scan", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceScan; s.ScanRank = StockScanRankMomentum }, ""},
		{"scan_rank momentum ok in hybrid", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceHybrid; s.ScanRank = StockScanRankMomentum }, ""},
		{"scan_rank unknown", func(s *StockConfig) { s.ScanRank = "rs63" }, `invalid scan_rank "rs63": must be "kell" or "momentum"`},
		{"hybrid with symbols ok", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceHybrid }, ""},
		{"hybrid with no symbols", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceHybrid; s.Symbols = nil }, "at least one symbol in hybrid mode"},
		{"hybrid bad pin", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceHybrid; s.Symbols = []string{"AAPL"} }, "invalid stock symbol"},
		{"hybrid top n 51", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceHybrid; s.ScanTopN = 51 }, "scan_top_n"},
		{"scan top n 51", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceScan; s.ScanTopN = 51 }, "scan_top_n"},
		{"scan top n negative", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceScan; s.ScanTopN = -1 }, "scan_top_n"},
		{"scan bad pin", func(s *StockConfig) { s.SymbolSource = StockSymbolSourceScan; s.Symbols = []string{"AAPL"} }, "invalid stock symbol"},
		{"scan exclude bad format", func(s *StockConfig) { s.ScanExclude = []string{"tslabusdt"} }, "scan_exclude"},
		{"scan exclude duplicate", func(s *StockConfig) { s.ScanExclude = []string{"TSLABUSDT", "TSLABUSDT"} }, "duplicate"},
		{"empty symbols", func(s *StockConfig) { s.Symbols = nil }, "at least one symbol"},
		{"lowercase symbol", func(s *StockConfig) { s.Symbols = []string{"aaplbusdt"} }, "invalid stock symbol"},
		{"wrong suffix", func(s *StockConfig) { s.Symbols = []string{"AAPLUSDT"} }, "invalid stock symbol"},
		{"suffix only", func(s *StockConfig) { s.Symbols = []string{"BUSDT"} }, "invalid stock symbol"},
		{"duplicate", func(s *StockConfig) { s.Symbols = []string{"AAPLBUSDT", "AAPLBUSDT"} }, "duplicate"},
		{"preset swing", func(s *StockConfig) { s.Preset = "swing" }, ""},
		{"preset position", func(s *StockConfig) { s.Preset = "position" }, ""},
		{"preset bad", func(s *StockConfig) { s.Preset = "day" }, "preset"},
		{"only pre-market", func(s *StockConfig) {
			s.Sessions = StockSessions{Regular: stockBool(false), PreMarket: true}
		}, ""},
		{"no session", func(s *StockConfig) { s.Sessions = StockSessions{Regular: stockBool(false)} }, "session"},
		{"divergence 10 ok", func(s *StockConfig) { s.MaxDivergencePct = 10 }, ""},
		{"divergence 10.5", func(s *StockConfig) { s.MaxDivergencePct = 10.5 }, "max_divergence_pct"},
		{"divergence disabled ok", func(s *StockConfig) { s.MaxDivergencePct = -1 }, ""},
		{"position pct 100 ok", func(s *StockConfig) { s.MaxPositionPct = 100 }, ""},
		{"position pct 101", func(s *StockConfig) { s.MaxPositionPct = 101 }, "max_position_pct"},
		{"position pct negative", func(s *StockConfig) { s.MaxPositionPct = -1 }, "max_position_pct"},
		{"exposure 101", func(s *StockConfig) { s.MaxTotalExposurePct = 101 }, "max_total_exposure_pct"},
		{"exposure negative", func(s *StockConfig) { s.MaxTotalExposurePct = -5 }, "max_total_exposure_pct"},
		{"exposure < position", func(s *StockConfig) { s.MaxPositionPct, s.MaxTotalExposurePct = 30, 20 }, ">= max_position_pct"},
		{"exposure == position ok", func(s *StockConfig) { s.MaxPositionPct, s.MaxTotalExposurePct = 30, 30 }, ""},
		{"max positions 20 ok", func(s *StockConfig) { s.MaxPositions = 20 }, ""},
		{"max positions 21", func(s *StockConfig) { s.MaxPositions = 21 }, "max_positions"},
		{"max positions negative", func(s *StockConfig) { s.MaxPositions = -1 }, "max_positions"},
		{"risk 5 ok", func(s *StockConfig) { s.RiskPerTradePct = 5 }, ""},
		{"risk 5.1", func(s *StockConfig) { s.RiskPerTradePct = 5.1 }, "risk_per_trade_pct"},
		{"risk negative", func(s *StockConfig) { s.RiskPerTradePct = -0.1 }, "risk_per_trade_pct"},
		{"atr band ok", func(s *StockConfig) { s.StopATRMin, s.StopATRMax = 1, 10 }, ""},
		{"atr max > 10", func(s *StockConfig) { s.StopATRMin, s.StopATRMax = 1, 10.5 }, "stop_atr"},
		{"atr min >= max", func(s *StockConfig) { s.StopATRMin, s.StopATRMax = 3, 3 }, "less than"},
		{"atr negative", func(s *StockConfig) { s.StopATRMin = -1 }, "stop_atr"},
		{"atr min above preset max", func(s *StockConfig) { s.StopATRMin = 3.5 }, "less than"}, // swing max 3.0
		{"atr max below preset min", func(s *StockConfig) { s.StopATRMax = 1.0 }, "less than"}, // swing min 1.5
		{"non-finite", func(s *StockConfig) { s.RiskPerTradePct = math.NaN() }, "non-finite"},
		{"regime basket_vol ok", func(s *StockConfig) { s.RegimeScaler = StockRegimeBasketVol }, ""},
		{"regime off ok", func(s *StockConfig) { s.RegimeScaler = StockRegimeOff }, ""},
		{"regime unknown", func(s *StockConfig) { s.RegimeScaler = "vix" }, "regime_scaler"},
		{"regime target 10 ok", func(s *StockConfig) { s.RegimeTargetVolPct = 10 }, ""},
		{"regime target 150 ok", func(s *StockConfig) { s.RegimeTargetVolPct = 150 }, ""},
		{"regime target 9.9", func(s *StockConfig) { s.RegimeTargetVolPct = 9.9 }, "regime_target_vol_pct"},
		{"regime target 151", func(s *StockConfig) { s.RegimeTargetVolPct = 151 }, "regime_target_vol_pct"},
		{"regime target negative", func(s *StockConfig) { s.RegimeTargetVolPct = -1 }, "regime_target_vol_pct"},
		{"regime floor 0.05 ok", func(s *StockConfig) { s.RegimeFloor = 0.05 }, ""},
		{"regime floor 1 ok", func(s *StockConfig) { s.RegimeFloor = 1 }, ""},
		{"regime floor 0.04", func(s *StockConfig) { s.RegimeFloor = 0.04 }, "regime_floor"},
		{"regime floor 1.1", func(s *StockConfig) { s.RegimeFloor = 1.1 }, "regime_floor"},
		{"regime floor negative", func(s *StockConfig) { s.RegimeFloor = -0.5 }, "regime_floor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validStock()
			tc.mutate(s)
			err := s.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
	if err := (*StockConfig)(nil).Validate(); err == nil {
		t.Fatalf("nil StockConfig must be rejected")
	}
}

func TestStrategyConfigValidate_USStockType(t *testing.T) {
	// us_stock without stock_config → rejected
	if err := (&StrategyConfig{StrategyType: StrategyTypeUSStock}).Validate(); err == nil {
		t.Fatalf("us_stock without stock_config must be rejected")
	}
	// valid us_stock → accepted (no crypto coin-source needed)
	ok := &StrategyConfig{StrategyType: StrategyTypeUSStock, StockConfig: validStock()}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid us_stock rejected: %v", err)
	}
	// invalid stock_config surfaces through StrategyConfig.Validate
	bad := &StrategyConfig{StrategyType: StrategyTypeUSStock, StockConfig: &StockConfig{Symbols: []string{"AAPL"}}}
	if err := bad.Validate(); err == nil {
		t.Fatalf("bad symbol must be rejected")
	}
	// existing types unchanged: empty / ai_trading accepted, unknown rejected,
	// and a stray stock_config on a non-stock strategy is not validated.
	for _, ty := range []string{"", "ai_trading"} {
		c := &StrategyConfig{StrategyType: ty, StockConfig: &StockConfig{}}
		if err := c.Validate(); err != nil {
			t.Fatalf("strategy_type %q rejected: %v", ty, err)
		}
	}
	if err := (&StrategyConfig{StrategyType: "forex"}).Validate(); err == nil {
		t.Fatalf("unknown strategy_type must be rejected")
	}
}

func TestStockConfig_ClampLimitsAndJSONRoundTrip(t *testing.T) {
	c := &StrategyConfig{StrategyType: StrategyTypeUSStock, StockConfig: validStock()}
	c.StockConfig.Preset = StockPresetPosition
	c.StockConfig.PaperTrading = stockBool(false)
	before, _ := json.Marshal(c.StockConfig)
	c.ClampLimits()
	after, _ := json.Marshal(c.StockConfig)
	if string(before) != string(after) {
		t.Fatalf("ClampLimits altered stock_config:\n%s\n%s", before, after)
	}
	if c.StrategyType != StrategyTypeUSStock {
		t.Fatalf("ClampLimits changed strategy_type")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("config invalid after ClampLimits: %v", err)
	}
	// JSON tags round-trip with the documented names.
	raw := `{"strategy_type":"us_stock","stock_config":{"symbols":["NVDABUSDT"],"preset":"swing","sessions":{"regular":true,"pre_market":false,"after_hours":false},"data_fallback_yahoo":true,"max_divergence_pct":1,"max_position_pct":20,"max_total_exposure_pct":80,"max_positions":5,"risk_per_trade_pct":1,"stop_atr_min":1.5,"stop_atr_max":3,"paper_trading":true}}`
	var parsed StrategyConfig
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if err := parsed.Validate(); err != nil {
		t.Fatalf("design-doc config rejected: %v", err)
	}
	if !parsed.StockConfig.IsPaper() || parsed.StockConfig.EffectiveMaxPositions() != 5 {
		t.Fatalf("design-doc config parsed wrongly: %+v", parsed.StockConfig)
	}
}

// stock_config edits must show up in the version diff (diff is generic).
func TestConfigVersionDiff_StockConfig(t *testing.T) {
	st := newTestStore(t)
	s := st.Strategy()
	if err := s.initTables(); err != nil {
		t.Fatalf("init: %v", err)
	}
	old := `{"strategy_type":"us_stock","stock_config":{"symbols":["AAPLBUSDT"],"paper_trading":true,"max_positions":5}}`
	next := `{"strategy_type":"us_stock","stock_config":{"symbols":["AAPLBUSDT","NVDABUSDT"],"paper_trading":false,"max_positions":5}}`
	if err := s.RecordConfigChange("STK1", old, 0, next, "save"); err != nil {
		t.Fatalf("record: %v", err)
	}
	rows, err := s.ListConfigVersions("STK1")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %d err=%v, want baseline + save", len(rows), err)
	}
	var diff []ConfigDiffEntry
	if err := json.Unmarshal([]byte(rows[1].Summary), &diff); err != nil {
		t.Fatalf("summary: %v", err)
	}
	paths := map[string]bool{}
	for _, d := range diff {
		paths[d.Path] = true
	}
	if !paths["stock_config.paper_trading"] {
		t.Fatalf("paper_trading change missing from diff: %v", diff)
	}
	hasSymbols := false
	for p := range paths {
		if strings.HasPrefix(p, "stock_config.symbols") {
			hasSymbols = true
		}
	}
	if !hasSymbols {
		t.Fatalf("symbols change missing from diff: %v", diff)
	}
	if paths["stock_config.max_positions"] {
		t.Fatalf("unchanged max_positions must not appear: %v", diff)
	}
}
