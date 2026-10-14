package store

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Defaults for the us_stock strategy (design §4 presets, §6 risk defaults,
// docs/architecture/US_STOCK_BSTOCK_DESIGN_2026-10-09.md).
const (
	DefaultStockMaxDivergencePct    = 1.0
	DefaultStockMaxPositionPct      = 20.0
	DefaultStockMaxTotalExposurePct = 80.0
	DefaultStockMaxPositions        = 5
	DefaultStockRiskPerTradePct     = 1.0
	DefaultStockSwingStopATRMin     = 1.5
	DefaultStockSwingStopATRMax     = 3.0
	DefaultStockPositionStopATRMin  = 2.0
	DefaultStockPositionStopATRMax  = 4.0

	// Symbol sources of a us_stock strategy (StockConfig.SymbolSource).
	StockSymbolSourceManual = "manual"
	StockSymbolSourceScan   = "scan"
	DefaultStockScanTopN    = 10

	maxStockScanTopN = 50

	stockSymbolSuffix       = "BUSDT"
	maxStockDivergencePct   = 10.0
	maxStockPositionsCap    = 20
	maxStockRiskPerTradePct = 5.0
	maxStockStopATR         = 10.0
	stockDivergenceDisabled = -1.0
)

// EffectivePreset returns the holding-period preset; empty or unknown values
// resolve to swing (Validate rejects unknown values at save time).
func (s *StockConfig) EffectivePreset() string {
	if s != nil && s.Preset == StockPresetPosition {
		return StockPresetPosition
	}
	return StockPresetSwing
}

// IsScanMode reports whether the symbol list comes from the price-cycle scan.
func (s *StockConfig) IsScanMode() bool {
	return s != nil && s.SymbolSource == StockSymbolSourceScan
}

// EffectiveScanTopN is the number of scanner picks fed to the model per cycle
// (default 10, max 50).
func (s *StockConfig) EffectiveScanTopN() int {
	if s == nil || s.ScanTopN <= 0 {
		return DefaultStockScanTopN
	}
	if s.ScanTopN > maxStockScanTopN {
		return maxStockScanTopN
	}
	return s.ScanTopN
}

// AllowRegular reports whether opens/adds are allowed in the regular session
// (09:30–16:00 ET). nil config or unset flag = true.
func (s *StockConfig) AllowRegular() bool {
	if s == nil || s.Sessions.Regular == nil {
		return true
	}
	return *s.Sessions.Regular
}

// AllowPreMarket reports whether opens/adds are allowed pre-market (04:00–09:30 ET).
func (s *StockConfig) AllowPreMarket() bool { return s != nil && s.Sessions.PreMarket }

// AllowAfterHours reports whether opens/adds are allowed after hours (16:00–20:00 ET).
func (s *StockConfig) AllowAfterHours() bool { return s != nil && s.Sessions.AfterHours }

// YahooFallback reports whether underlying-stock Yahoo data may replace a
// timeframe with too few bStock bars. nil = true.
func (s *StockConfig) YahooFallback() bool {
	if s == nil || s.DataFallbackYahoo == nil {
		return true
	}
	return *s.DataFallbackYahoo
}

// EffectiveMaxDivergencePct resolves the bStock-vs-underlying divergence
// guard: 0 → 1.0, negative → -1 (check disabled), otherwise the set value.
func (s *StockConfig) EffectiveMaxDivergencePct() float64 {
	if s == nil || s.MaxDivergencePct == 0 {
		return DefaultStockMaxDivergencePct
	}
	if s.MaxDivergencePct < 0 {
		return stockDivergenceDisabled
	}
	return s.MaxDivergencePct
}

// EffectiveMaxPositionPct is the per-symbol cap in percent of equity (default 20).
func (s *StockConfig) EffectiveMaxPositionPct() float64 {
	if s == nil || s.MaxPositionPct <= 0 {
		return DefaultStockMaxPositionPct
	}
	return s.MaxPositionPct
}

// EffectiveMaxTotalExposurePct is the total exposure cap in percent of equity (default 80).
func (s *StockConfig) EffectiveMaxTotalExposurePct() float64 {
	if s == nil || s.MaxTotalExposurePct <= 0 {
		return DefaultStockMaxTotalExposurePct
	}
	return s.MaxTotalExposurePct
}

// EffectiveMaxPositions is the maximum number of concurrent holdings (default 5).
func (s *StockConfig) EffectiveMaxPositions() int {
	if s == nil || s.MaxPositions <= 0 {
		return DefaultStockMaxPositions
	}
	return s.MaxPositions
}

// EffectiveRiskPerTradePct is the loss at the stop in percent of equity (default 1.0).
func (s *StockConfig) EffectiveRiskPerTradePct() float64 {
	if s == nil || s.RiskPerTradePct <= 0 {
		return DefaultStockRiskPerTradePct
	}
	return s.RiskPerTradePct
}

// EffectiveStopATRBand returns the allowed stop distance band in ATR(1d)
// multiples. Preset defaults: swing 1.5–3.0, position 2.0–4.0; an explicit
// value overrides its own end only.
func (s *StockConfig) EffectiveStopATRBand() (min, max float64) {
	min, max = DefaultStockSwingStopATRMin, DefaultStockSwingStopATRMax
	if s.EffectivePreset() == StockPresetPosition {
		min, max = DefaultStockPositionStopATRMin, DefaultStockPositionStopATRMax
	}
	if s == nil {
		return min, max
	}
	if s.StopATRMin > 0 {
		min = s.StopATRMin
	}
	if s.StopATRMax > 0 {
		max = s.StopATRMax
	}
	return min, max
}

// IsPaper reports whether the strategy runs in paper mode (no orders placed).
// nil = true: the first launch is paper by default.
func (s *StockConfig) IsPaper() bool {
	if s == nil || s.PaperTrading == nil {
		return true
	}
	return *s.PaperTrading
}

// Validate checks the us_stock configuration. It is format-only: whether a
// symbol is a currently listed bStock pair is checked at runtime elsewhere.
func (s *StockConfig) Validate() error {
	if s == nil {
		return fmt.Errorf("stock_config is required for us_stock strategy")
	}
	if _, err := json.Marshal(s); err != nil {
		return fmt.Errorf("stock_config contains non-finite values: %w", err)
	}
	switch s.SymbolSource {
	case "", StockSymbolSourceManual:
		if len(s.Symbols) == 0 {
			return fmt.Errorf("stock_config.symbols must contain at least one symbol")
		}
	case StockSymbolSourceScan:
	default:
		return fmt.Errorf("invalid symbol_source %q: must be %q or %q", s.SymbolSource, StockSymbolSourceManual, StockSymbolSourceScan)
	}
	if s.ScanTopN < 0 || s.ScanTopN > maxStockScanTopN {
		return fmt.Errorf("scan_top_n must be between 0 and %d", maxStockScanTopN)
	}
	if err := validateStockSymbolList("symbols", s.Symbols); err != nil {
		return err
	}
	if err := validateStockSymbolList("scan_exclude", s.ScanExclude); err != nil {
		return err
	}
	if s.Preset != "" && s.Preset != StockPresetSwing && s.Preset != StockPresetPosition {
		return fmt.Errorf("invalid stock preset %q: must be %q or %q", s.Preset, StockPresetSwing, StockPresetPosition)
	}
	if !s.AllowRegular() && !s.AllowPreMarket() && !s.AllowAfterHours() {
		return fmt.Errorf("at least one trading session must be enabled")
	}
	if s.MaxDivergencePct > maxStockDivergencePct {
		return fmt.Errorf("max_divergence_pct must be at most %v", maxStockDivergencePct)
	}
	if s.MaxPositionPct < 0 || s.MaxPositionPct > 100 {
		return fmt.Errorf("max_position_pct must be in (0, 100]")
	}
	if s.MaxTotalExposurePct < 0 || s.MaxTotalExposurePct > 100 {
		return fmt.Errorf("max_total_exposure_pct must be in (0, 100]")
	}
	if s.MaxPositionPct > 0 && s.MaxTotalExposurePct > 0 && s.MaxTotalExposurePct < s.MaxPositionPct {
		return fmt.Errorf("max_total_exposure_pct must be >= max_position_pct")
	}
	if s.MaxPositions < 0 || s.MaxPositions > maxStockPositionsCap {
		return fmt.Errorf("max_positions must be between 1 and %d", maxStockPositionsCap)
	}
	if s.RiskPerTradePct < 0 || s.RiskPerTradePct > maxStockRiskPerTradePct {
		return fmt.Errorf("risk_per_trade_pct must be in (0, %v]", maxStockRiskPerTradePct)
	}
	if s.StopATRMin < 0 || s.StopATRMax < 0 || s.StopATRMin > maxStockStopATR || s.StopATRMax > maxStockStopATR {
		return fmt.Errorf("stop_atr_min/stop_atr_max must be in (0, %v]", maxStockStopATR)
	}
	if s.StopATRMin > 0 && s.StopATRMax > 0 && s.StopATRMin >= s.StopATRMax {
		return fmt.Errorf("stop_atr_min must be less than stop_atr_max")
	}
	// An explicit end combined with the other end's preset default must also
	// leave a valid band.
	if s.StopATRMin > 0 || s.StopATRMax > 0 {
		lo, hi := s.EffectiveStopATRBand()
		if lo >= hi {
			return fmt.Errorf("stop_atr_min must be less than stop_atr_max (effective band %v-%v)", lo, hi)
		}
	}
	return nil
}

// validateStockSymbolList checks pair format and duplicates of one list.
func validateStockSymbolList(field string, list []string) error {
	seen := make(map[string]bool, len(list))
	for _, sym := range list {
		if sym != strings.ToUpper(strings.TrimSpace(sym)) || !strings.HasSuffix(sym, stockSymbolSuffix) || len(sym) <= len(stockSymbolSuffix) {
			return fmt.Errorf("invalid stock symbol %q in %s: must be uppercase and end with %s", sym, field, stockSymbolSuffix)
		}
		if seen[sym] {
			return fmt.Errorf("duplicate stock symbol %q in %s", sym, field)
		}
		seen[sym] = true
	}
	return nil
}
