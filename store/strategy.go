package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Hard limits to prevent token explosion in AI requests
const (
	MaxCandidateCoins = 10
	// MaxPositions is a sanity BOUND on risk_control.max_positions (fat-finger
	// protection), not the default — the configured value is honored up to
	// this cap. The default TEMPLATE for new strategies stays 3.
	MaxPositions  = 10
	MaxTimeframes = 4
	// MinKlineCount = 60 because the data-sufficiency hard gate requires
	// ≥60 closed bars per configured timeframe (EMA50/MACD long-window
	// stability, signal_layer minBars) — a UI count below this made EVERY
	// candidate DATA_INSUFFICIENT and the strategy stopped opening entirely
	// (user set 50 on 09-25 13:27, zero opens for 7+ hours, 2026-09-25).
	// One source of truth: the floor equals the gate.
	MinKlineCount = 60
	// 120: the structured signal derives indicators from the closed-bar
	// window — 30 bars left EMA50/MACD unstable (data_quality.sufficient
	// flagged EVERY candidate as insufficient). 120 keeps ≥60 closed bars
	// even after the forming-bar drop on the longest fetched TF.
	MaxKlineCount = 120
)

// ClampLimits enforces product-level limits on strategy config to prevent token overflow.
func (c *StrategyConfig) ClampLimits() {
	// Clamp coin source limits
	if c.CoinSource.AI500Limit > MaxCandidateCoins {
		c.CoinSource.AI500Limit = MaxCandidateCoins
	}
	if c.CoinSource.OITopLimit > MaxCandidateCoins {
		c.CoinSource.OITopLimit = MaxCandidateCoins
	}
	if c.CoinSource.OILowLimit > MaxCandidateCoins {
		c.CoinSource.OILowLimit = MaxCandidateCoins
	}

	// Clamp static coins
	if c.CoinSource.PiggyDashLimit > MaxCandidateCoins {
		c.CoinSource.PiggyDashLimit = MaxCandidateCoins
	}
	if c.CoinSource.ShortScanLimit > MaxCandidateCoins {
		c.CoinSource.ShortScanLimit = MaxCandidateCoins
	}
	if len(c.CoinSource.StaticCoins) > MaxCandidateCoins {
		c.CoinSource.StaticCoins = c.CoinSource.StaticCoins[:MaxCandidateCoins]
	}

	// Clamp kline count
	if c.Indicators.Klines.PrimaryCount < MinKlineCount {
		c.Indicators.Klines.PrimaryCount = MinKlineCount
	}
	if c.Indicators.Klines.PrimaryCount > MaxKlineCount {
		c.Indicators.Klines.PrimaryCount = MaxKlineCount
	}
	if c.Indicators.Klines.LongerCount > MaxKlineCount {
		c.Indicators.Klines.LongerCount = MaxKlineCount
	}

	// Clamp timeframes
	if len(c.Indicators.Klines.SelectedTimeframes) > MaxTimeframes {
		c.Indicators.Klines.SelectedTimeframes = c.Indicators.Klines.SelectedTimeframes[:MaxTimeframes]
	}

	// Clamp max positions
	if c.RiskControl.MaxPositions > MaxPositions {
		c.RiskControl.MaxPositions = MaxPositions
	}

}

// StrategyStore strategy storage
type StrategyStore struct {
	db *gorm.DB
}

// Strategy strategy configuration
type Strategy struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	UserID        string    `gorm:"column:user_id;not null;default:'';index" json:"user_id"`
	Name          string    `gorm:"not null" json:"name"`
	Description   string    `gorm:"default:''" json:"description"`
	IsActive      bool      `gorm:"column:is_active;default:false;index" json:"is_active"`
	IsDefault     bool      `gorm:"column:is_default;default:false" json:"is_default"`
	IsPublic      bool      `gorm:"column:is_public;default:false;index" json:"is_public"`    // whether visible in strategy market
	ConfigVisible bool      `gorm:"column:config_visible;default:true" json:"config_visible"` // whether config details are visible
	Config        string    `gorm:"not null;default:'{}'" json:"config"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Strategy) TableName() string { return "strategies" }

// StrategyConfig strategy configuration details (JSON structure)
type StrategyConfig struct {
	// Strategy type: "ai_trading" (default), "grid_trading" or "us_stock"
	// (US equities via Binance spot bStock pairs, see StockConfig).
	StrategyType string `json:"strategy_type,omitempty"`

	// language setting: "zh" for Chinese, "en" for English
	// This determines the language used for data formatting and prompt generation
	Language string `json:"language,omitempty"`
	// coin source configuration
	CoinSource CoinSourceConfig `json:"coin_source"`
	// quantitative data configuration
	Indicators IndicatorConfig `json:"indicators"`
	// custom prompt (appended at the end)
	CustomPrompt string `json:"custom_prompt,omitempty"`
	// StatsWindowDays is the rolling window (in days) for the trading stats
	// the AI sees (PF/win-rate/expectancy in strategy_health). Only trades
	// closed within the window feed those numbers. 0 or absent = 30 days
	// (default), a negative value = full history (window disabled). The
	// window changes what is REPORTED to the AI only — no trades are ever
	// deleted or moved.
	StatsWindowDays int `json:"stats_window_days,omitempty"`

	// USStockSessionBoostPct: during US regular trading hours (Eastern
	// Mon–Fri 09:30–16:00), short-scan candidates that are US equity tokens
	// get their score multiplied by (1 + pct/100) BEFORE the top-N cut —
	// equity tokens tracked the underlying session show lower volatility and
	// (so far) a better realized win rate than the crypto pool (user
	// directive 2026-09-23; journal baseline: stocks 11 trades 54.5% win vs
	// crypto 184 trades 40.8%). 0 = default 20, negative = disabled.
	// Weekends stay blocked by stock_weekend_no_open regardless.
	USStockSessionBoostPct float64 `json:"us_stock_session_boost_pct,omitempty"`
	// risk control configuration
	RiskControl RiskControlConfig `json:"risk_control"`
	// editable sections of System Prompt
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`

	// Grid trading configuration (only used when StrategyType == "grid_trading")
	GridConfig *GridStrategyConfig `json:"grid_config,omitempty"`

	// US-stock configuration (only used when StrategyType == "us_stock").
	StockConfig *StockConfig `json:"stock_config,omitempty"`
}

// StrategyTypeUSStock selects the US-equity strategy: long-only spot trading
// of Binance bStock pairs (AAPLBUSDT, SPYBUSDT, ...), driven by its own run
// cycle (design: docs/architecture/US_STOCK_BSTOCK_DESIGN_2026-10-09.md).
const StrategyTypeUSStock = "us_stock"

// StockConfig is the us_stock strategy configuration. Zero values mean "use
// the preset default" (see StockPresetSwing / StockPresetPosition); the
// resolvers live next to Validate.
type StockConfig struct {
	// Symbols: Binance spot bStock pairs to trade, e.g. "AAPLBUSDT". In hybrid
	// mode they are always included; scan mode ignores them.
	Symbols []string `json:"symbols"`
	// SymbolSource: "manual" (default; trade Symbols), "scan" (only the top
	// ScanTopN long setups of Kell's price-cycle scan over every listed bStock
	// pair, plus held names; Symbols ignored) or "hybrid" (Symbols always, plus
	// the top ScanTopN scanner picks and held names).
	SymbolSource string   `json:"symbol_source,omitempty"`
	ScanTopN     int      `json:"scan_top_n,omitempty"`   // scan mode; 0 = default 10, max 50
	ScanExclude  []string `json:"scan_exclude,omitempty"` // pairs the scanner never picks
	// ScanRank: scan/hybrid ranking — "kell" (default; Kell cycle long setups
	// by score) or "momentum" (names above SMA200 ranked by 12-1 momentum).
	ScanRank string `json:"scan_rank,omitempty"`
	// Preset: "swing" (default; days–weeks, 1d trend / 1h entry, decisions at
	// 09:45, 12:30, 15:30 ET) or "position" (weeks–months, 1w trend / 1d
	// entry, one decision at 15:30 ET).
	Preset string `json:"preset,omitempty"`
	// Sessions in which NEW exposure (open/add) may be taken. Exits and
	// stop management run in every session. Default: regular only.
	Sessions StockSessions `json:"sessions"`
	// DataFallbackYahoo: when a timeframe has too few closed bStock bars,
	// use the underlying's Yahoo Finance series for that timeframe. nil = true.
	DataFallbackYahoo *bool `json:"data_fallback_yahoo,omitempty"`
	// MaxDivergencePct: block opens when |bStock − underlying| / underlying
	// exceeds this percent. 0 = default 1.0; negative = check disabled.
	MaxDivergencePct float64 `json:"max_divergence_pct,omitempty"`
	// Position limits, percent of account equity. 0 = default.
	MaxPositionPct      float64 `json:"max_position_pct,omitempty"`       // default 20
	MaxTotalExposurePct float64 `json:"max_total_exposure_pct,omitempty"` // default 80
	MaxPositions        int     `json:"max_positions,omitempty"`          // default 5
	// RiskPerTradePct: loss at the stop, percent of equity. 0 = default 1.0.
	RiskPerTradePct float64 `json:"risk_per_trade_pct,omitempty"`
	// Stop distance band in ATR(1d) multiples. 0 = preset default
	// (swing 1.5–3.0, position 2.0–4.0).
	StopATRMin float64 `json:"stop_atr_min,omitempty"`
	StopATRMax float64 `json:"stop_atr_max,omitempty"`
	// PaperTrading: run data, decisions and risk checks but place no orders.
	// nil = true (first launch is paper by default — user decision 2026-10-09).
	PaperTrading *bool `json:"paper_trading,omitempty"`
	// RegimeScaler: market-state position scaler. "" / "off" = disabled
	// (default); "basket_vol" scales the risk budget, per-symbol cap and total
	// exposure cap of NEW entries by s = clip(target_vol / σ_basket, floor, 1),
	// σ_basket being the annualized volatility of the equal-weight top-10
	// momentum basket over the last 21 sessions (backtest: 2026-10-17-regime-
	// overlay-backtrader-1d.md).
	RegimeScaler string `json:"regime_scaler,omitempty"`
	// RegimeTargetVolPct: target annualized basket volatility in percent.
	// 0 = default 40; valid 10–150.
	RegimeTargetVolPct float64 `json:"regime_target_vol_pct,omitempty"`
	// RegimeFloor: lower bound of the scale s. 0 = default 0.25; valid 0.05–1.
	RegimeFloor float64 `json:"regime_floor,omitempty"`
	// RegimeTrim: trim held program-owned positions when s falls (by at least
	// the 0.20 band from the last applied scale). nil = true.
	RegimeTrim *bool `json:"regime_trim,omitempty"`
}

// StockSessions toggles the US sessions in which opens/adds are allowed.
type StockSessions struct {
	Regular    *bool `json:"regular,omitempty"`     // 09:30–16:00 ET; nil = true
	PreMarket  bool  `json:"pre_market,omitempty"`  // 04:00–09:30 ET
	AfterHours bool  `json:"after_hours,omitempty"` // 16:00–20:00 ET
}

const (
	StockPresetSwing    = "swing"
	StockPresetPosition = "position"
)

// GridStrategyConfig grid trading specific configuration
type GridStrategyConfig struct {
	// Trading pair (e.g., "BTCUSDT")
	Symbol string `json:"symbol"`
	// Number of grid levels (5-50)
	GridCount int `json:"grid_count"`
	// Total investment in USDT
	TotalInvestment float64 `json:"total_investment"`
	// Leverage (1-20)
	Leverage int `json:"leverage"`
	// Upper price boundary (0 = auto-calculate from ATR)
	UpperPrice float64 `json:"upper_price"`
	// Lower price boundary (0 = auto-calculate from ATR)
	LowerPrice float64 `json:"lower_price"`
	// Use ATR to auto-calculate bounds
	UseATRBounds bool `json:"use_atr_bounds"`
	// ATR multiplier for bound calculation (default 2.0)
	ATRMultiplier float64 `json:"atr_multiplier"`
	// Position distribution: "uniform" | "gaussian" | "pyramid"
	Distribution string `json:"distribution"`
	// Maximum drawdown percentage before emergency exit
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	// Stop loss percentage per position
	StopLossPct float64 `json:"stop_loss_pct"`
	// Daily loss limit percentage
	DailyLossLimitPct float64 `json:"daily_loss_limit_pct"`
	// Use maker-only orders for lower fees
	UseMakerOnly bool `json:"use_maker_only"`
	// Enable automatic grid direction adjustment based on box breakouts
	EnableDirectionAdjust bool `json:"enable_direction_adjust"`
	// Direction bias ratio for long_bias/short_bias modes (default 0.7 = 70%/30%)
	DirectionBiasRatio float64 `json:"direction_bias_ratio"`
}

// PromptSectionsConfig editable sections of System Prompt
type PromptSectionsConfig struct {
	// role definition (title + description)
	RoleDefinition string `json:"role_definition,omitempty"`
	// trading frequency awareness
	TradingFrequency string `json:"trading_frequency,omitempty"`
	// entry standards
	EntryStandards string `json:"entry_standards,omitempty"`
	// decision process
	DecisionProcess string `json:"decision_process,omitempty"`
}

// CoinSourceConfig coin source configuration
type CoinSourceConfig struct {
	// source type: "static" | "ai500" | "oi_top" | "oi_low" | "piggy_dash" | "mixed"
	// "mixed" combines any selection of the individual sources (multi-select)
	SourceType string `json:"source_type"`
	// static coin list (used when source_type = "static", or in mixed mode
	// when UseStatic is set)
	StaticCoins []string `json:"static_coins,omitempty"`
	// excluded coins list (filtered out from all sources)
	ExcludedCoins []string `json:"excluded_coins,omitempty"`
	// mixed mode: whether to include the static coin list as a source.
	// Pointer so absent (legacy configs, which always included static coins)
	// can default to true while an explicit false excludes them.
	UseStatic *bool `json:"use_static,omitempty"`
	// whether to use AI500 coin pool
	UseAI500 bool `json:"use_ai500"`
	// AI500 coin pool maximum count
	AI500Limit int `json:"ai500_limit,omitempty"`
	// whether to use OI Top (OI increase ranking, suitable for long positions)
	UseOITop bool `json:"use_oi_top"`
	// OI Top maximum count
	OITopLimit int `json:"oi_top_limit,omitempty"`
	// whether to use OI Low (OI decrease ranking, suitable for short positions)
	UseOILow bool `json:"use_oi_low"`
	// OI Low maximum count
	OILowLimit int `json:"oi_low_limit,omitempty"`
	// whether to use the breakout engine "piggy dash" source (猪猪冲刺:
	// strongest breakout/breakdown signals from the 5-min scheduler)
	UsePiggyDash       bool   `json:"use_piggy_dash"`
	PiggyDashLimit     int    `json:"piggy_dash_limit,omitempty"`
	PiggyDashDirection string `json:"piggy_dash_direction,omitempty"` // "", "breakout", "breakdown"
	// Minimum breakout-engine grade a signal needs to enter the piggy_dash
	// pool: "weak" | "medium" | "strong"; empty = default "medium" (see
	// EffectivePiggyDashMinGrade). Backtest (data/breakout_backtest.json, 1104
	// signals, 24h net mean): weak 897 signals, 31.9% win, -0.73%; medium 189,
	// 48.2%, +1.30%; strong 18, 88.9%, +2.22%. Default raised from weak at
	// review 2026-10-11.
	PiggyDashMinGrade string `json:"piggy_dash_min_grade,omitempty"`
	// whether to use the "short scan" source (做空扫描: top 24h gainers ranked
	// by short-suitability score computed from Binance indicators)
	UseShortScan bool `json:"use_short_scan"`
	// ShortScan maximum count
	ShortScanLimit int `json:"short_scan_limit,omitempty"`
	// ShortScan funding-rate crowding threshold (percent per 8h, e.g. 0.03 =
	// 0.03%). Injected into the AI prompt as a short-entry condition; injected
	// value falls back to the built-in default (0.03) when unset.
	ShortScanFundingRatePct float64 `json:"short_scan_funding_rate_pct,omitempty"`
	// Minimum open-interest value (millions USD) for a candidate coin to be
	// analyzed — lower-liquidity coins are skipped. 0 = built-in default (15M).
	MinOIValueMillions float64 `json:"min_oi_value_millions,omitempty"`
	// Gainer history pool (历史涨幅池): the past N days of daily top-gainer
	// snapshots are merged (deduped) into the short-scan universe, so coins
	// that pumped hard days ago and have since rolled over — already off
	// today's 24h board — still get scored as short candidates.
	// 0 = built-in default (7 days), negative = pool disabled.
	ShortScanHistoryDays int `json:"short_scan_history_days,omitempty"`
	// Cap on EXTRA history-pool symbols analyzed per scan (each costs ~6
	// fapi calls every 5 min). 0 = built-in default (30).
	ShortScanHistoryMax int `json:"short_scan_history_max,omitempty"`
	// Short-scan pool quality floor: candidates scoring below this short
	// score (0-100) are kept out of the pool. 0 = built-in default (55,
	// review 2026-10-07 S1). Applied every cycle; a strategy save reloads
	// the traders using it, so changes take effect without a restart.
	ShortScanMinScore float64 `json:"short_scan_min_score,omitempty"`
	// whether to use Hyperliquid All coins (all available perp pairs)
	UseHyperAll bool `json:"use_hyper_all"`
	// whether to use Hyperliquid Main coins (top N by 24h volume)
	UseHyperMain bool `json:"use_hyper_main"`
	// Hyperliquid Main maximum count (default 20)
	HyperMainLimit int `json:"hyper_main_limit,omitempty"`
	// Note: API URLs are now built automatically using NofxOSAPIKey from IndicatorConfig
}

// DefaultShortScanMinScore is the built-in short-scan pool quality floor
// applied when the per-strategy short_scan_min_score is unset (0).
const DefaultShortScanMinScore = 55.0

// EffectiveShortScanMinScore returns the configured short-scan quality floor,
// falling back to DefaultShortScanMinScore when unset and capping at 100.
func (c *CoinSourceConfig) EffectiveShortScanMinScore() float64 {
	if c.ShortScanMinScore <= 0 {
		return DefaultShortScanMinScore
	}
	if c.ShortScanMinScore > 100 {
		return 100
	}
	return c.ShortScanMinScore
}

// DefaultMinOIValueMillions is the built-in OI-value floor applied when the
// per-strategy min_oi_value_millions is unset (0).
const DefaultMinOIValueMillions = 15.0

// EffectiveMinOIMillions returns the configured OI-value floor, falling back
// to the built-in default when unset. All OI-filter call sites go through
// this method — the default lives here and nowhere else.
func (c *CoinSourceConfig) EffectiveMinOIMillions() float64 {
	if c.MinOIValueMillions <= 0 {
		return DefaultMinOIValueMillions
	}
	return c.MinOIValueMillions
}

// DefaultPiggyDashMinGrade is the built-in piggy_dash grade floor applied when
// the per-strategy piggy_dash_min_grade is unset or unrecognised.
const DefaultPiggyDashMinGrade = "medium"

// EffectivePiggyDashMinGrade returns the configured piggy_dash grade floor
// (lower-cased), falling back to DefaultPiggyDashMinGrade when unset or when
// the value is not one of weak / medium / strong.
func (c *CoinSourceConfig) EffectivePiggyDashMinGrade() string {
	switch g := strings.ToLower(c.PiggyDashMinGrade); g {
	case "weak", "medium", "strong":
		return g
	default:
		return DefaultPiggyDashMinGrade
	}
}

// IndicatorConfig indicator configuration
type IndicatorConfig struct {
	// K-line configuration
	Klines KlineConfig `json:"klines"`
	// raw kline data (OHLCV) - always enabled, required for AI analysis
	EnableRawKlines bool `json:"enable_raw_klines"`
	// technical indicator switches
	EnableEMA         bool `json:"enable_ema"`
	EnableMACD        bool `json:"enable_macd"`
	EnableRSI         bool `json:"enable_rsi"`
	EnableATR         bool `json:"enable_atr"`
	EnableBOLL        bool `json:"enable_boll"` // Bollinger Bands
	EnableVolume      bool `json:"enable_volume"`
	EnableOI          bool `json:"enable_oi"`           // open interest
	EnableFundingRate bool `json:"enable_funding_rate"` // funding rate
	// EMA period configuration
	EMAPeriods []int `json:"ema_periods,omitempty"` // default [20, 50]
	// RSI period configuration
	RSIPeriods []int `json:"rsi_periods,omitempty"` // default [7, 14]
	// ATR period configuration
	ATRPeriods []int `json:"atr_periods,omitempty"` // default [14]
	// BOLL period configuration (period, standard deviation multiplier is fixed at 2)
	BOLLPeriods []int `json:"boll_periods,omitempty"` // default [20] - can select multiple timeframes
	// external data sources
	ExternalDataSources []ExternalDataSource `json:"external_data_sources,omitempty"`

	// ========== NofxOS Unified API Configuration ==========
	// Unified API Key for all NofxOS data sources
	NofxOSAPIKey string `json:"nofxos_api_key,omitempty"`

	// quantitative data sources (capital flow, position changes, price changes)
	EnableQuantData    bool `json:"enable_quant_data"`    // whether to enable quantitative data
	EnableQuantOI      bool `json:"enable_quant_oi"`      // whether to show OI data
	EnableQuantNetflow bool `json:"enable_quant_netflow"` // whether to show Netflow data

	// OI ranking data (market-wide open interest increase/decrease rankings)
	EnableOIRanking   bool   `json:"enable_oi_ranking"`             // whether to enable OI ranking data
	OIRankingDuration string `json:"oi_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	OIRankingLimit    int    `json:"oi_ranking_limit,omitempty"`    // number of entries (default 10)

	// NetFlow ranking data (market-wide fund flow rankings - institution/personal)
	EnableNetFlowRanking   bool   `json:"enable_netflow_ranking"`             // whether to enable NetFlow ranking data
	NetFlowRankingDuration string `json:"netflow_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	NetFlowRankingLimit    int    `json:"netflow_ranking_limit,omitempty"`    // number of entries (default 10)

	// Price ranking data (market-wide gainers/losers)
	EnablePriceRanking   bool   `json:"enable_price_ranking"`             // whether to enable price ranking data
	PriceRankingDuration string `json:"price_ranking_duration,omitempty"` // durations: "1h" or "1h,4h,24h"
	PriceRankingLimit    int    `json:"price_ranking_limit,omitempty"`    // number of entries per ranking (default 10)
}

// KlineConfig K-line configuration
type KlineConfig struct {
	// primary timeframe: "1m", "3m", "5m", "15m", "1h", "4h"
	PrimaryTimeframe string `json:"primary_timeframe"`
	// primary timeframe K-line count
	PrimaryCount int `json:"primary_count"`
	// longer timeframe
	LongerTimeframe string `json:"longer_timeframe,omitempty"`
	// longer timeframe K-line count
	LongerCount int `json:"longer_count,omitempty"`
	// whether to enable multi-timeframe analysis
	EnableMultiTimeframe bool `json:"enable_multi_timeframe"`
	// selected timeframe list (new: supports multi-timeframe selection)
	SelectedTimeframes []string `json:"selected_timeframes,omitempty"`
	// PromptKlineBars caps how many CLOSED bars per timeframe the LLM prompt
	// carries (user review 2026-09-27: 60 bars × 4 TFs ≈ 9.9k chars ≈ 63% of
	// a full signal, while every traded field is already program-computed).
	// The full PrimaryCount history still feeds the indicators and stays
	// re-fetchable from the exchange; the prompt keeps the newest N bars +
	// a per-TF last-closed timestamp. 0 = default 20; negative = full
	// PrimaryCount (legacy / audit escape hatch).
	PromptKlineBars int `json:"prompt_kline_bars,omitempty"`
}

// ExternalDataSource external data source configuration
type ExternalDataSource struct {
	Name        string            `json:"name"`   // data source name
	Type        string            `json:"type"`   // type: "api" | "webhook"
	URL         string            `json:"url"`    // API URL
	Method      string            `json:"method"` // HTTP method
	Headers     map[string]string `json:"headers,omitempty"`
	DataPath    string            `json:"data_path,omitempty"`    // JSON data path
	RefreshSecs int               `json:"refresh_secs,omitempty"` // refresh interval (seconds)
}

// SanitizeCredentials strips every credential from a StrategyConfig in place
// (F9, 2026-10-01 review): the UNAUTHENTICATED public-strategies endpoint
// served the full deserialized config — nofxos_api_key and external data
// source headers (Authorization etc.) included — whenever config_visible was
// on. Public/config-visible output gets: no API key, no auth headers, and no
// URL query strings (tokens ride in queries too). Owner-scoped authenticated
// endpoints keep the real values.
func (c *StrategyConfig) SanitizeCredentials() {
	if c == nil {
		return
	}
	c.Indicators.NofxOSAPIKey = ""
	for i := range c.Indicators.ExternalDataSources {
		src := &c.Indicators.ExternalDataSources[i]
		src.Headers = nil
		// Drop everything from the query string on — `?key=...` is a common
		// token carrier, and public display only needs scheme+host+path.
		if idx := strings.Index(src.URL, "?"); idx >= 0 {
			src.URL = src.URL[:idx]
		}
	}
}

// RiskControlConfig risk control configuration
type RiskControlConfig struct {
	// Max number of coins held simultaneously (CODE ENFORCED)
	MaxPositions int `json:"max_positions"`

	// BTC/ETH exchange leverage ceiling for opening positions (CODE ENFORCED)
	BTCETHMaxLeverage int `json:"btc_eth_max_leverage"` // CODE ENFORCED: open decisions are clamped to this ceiling
	// Altcoin exchange leverage ceiling for opening positions (CODE ENFORCED)
	AltcoinMaxLeverage int `json:"altcoin_max_leverage"` // CODE ENFORCED: open decisions are clamped to this ceiling

	// BTC/ETH single position max value = equity × this ratio (CODE ENFORCED, default: 5)
	BTCETHMaxPositionValueRatio float64 `json:"btc_eth_max_position_value_ratio"`
	// Altcoin single position max value = equity × this ratio (CODE ENFORCED, default: 1)
	AltcoinMaxPositionValueRatio float64 `json:"altcoin_max_position_value_ratio"`

	// Max margin utilization (e.g. 0.9 = 90%) (CODE ENFORCED)
	MaxMarginUsage float64 `json:"max_margin_usage"`
	// Min position size in USDT (CODE ENFORCED)
	MinPositionSize float64 `json:"min_position_size"`

	// Min take_profit / stop_loss ratio (CODE ENFORCED at open: entries with a
	// lower computed ratio are rejected)
	MinRiskRewardRatio float64 `json:"min_risk_reward_ratio"`
	// Min AI confidence to open position. Decisions below it are rejected by
	// the executor; confidence remains a model assessment, but it cannot lower
	// the configured admission threshold. (CODE ENFORCED)
	MinConfidence int `json:"min_confidence"`

	// Min holding period in minutes before AI-initiated closes are allowed.
	// Exchange stop-loss/take-profit triggers bypass this lock. 0 = disabled. (CODE ENFORCED)
	MinHoldMinutes int `json:"min_hold_minutes"`
	// MaxSpreadPct: reject opens when the order-book spread exceeds this % of
	// mid (a wide spread eats the limit-order edge and taxes market fills).
	// 0 = default 0.5%; negative = disabled. Fail-open when no book. (CODE ENFORCED)
	MaxSpreadPct float64 `json:"max_spread_pct"`
	// MaxVendorDivergencePct: hard-block both directions when the vendor
	// forming-close vs live ticker diverges beyond this % — entry/SL/TP are
	// all priced off the live tick, so a large vendor gap invalidates the
	// whole setup (MYXUSDT 09-18: −2.57%). 0 = default 1%; negative =
	// disabled. Surfaced via hard_entry_gate.failed VENDOR_DIVERGENCE_x.xx.
	MaxVendorDivergencePct float64 `json:"max_vendor_divergence_pct"`
	// NegativeEdgeGate: when the strategy is on the NEGATIVE_EDGE (rolling
	// stats window PF < 0.9), opens must ADDITIONALLY pass a health gate —
	// |directional_score| ≥ negative_edge_min_score, trend+regime EMA
	// direction aligned with the trade, first-target RR ≥
	// negative_edge_min_rr, and no net-losing recent history on the symbol.
	// Codes NEG_EDGE_* land in hard_entry_gate.failed with no exception
	// path (user review 2026-09-27 #4 — the prose "证据极强/RR 明显占优"
	// had no numbers, so open and wait were both arguable). nil/true =
	// enabled; false = gate off. Thresholds are backtest-calibratable
	// knobs, not final constants. (CODE ENFORCED)
	NegativeEdgeGate *bool `json:"negative_edge_gate,omitempty"`
	// NEG_EDGE score threshold. 0 = default 80; negative = condition off.
	NegativeEdgeMinScore float64 `json:"negative_edge_min_score"`
	// NEG_EDGE first-target RR floor. 0 = default 2.0; negative = off.
	NegativeEdgeMinRR float64 `json:"negative_edge_min_rr"`
	// NegativeEdgeBlockLosingSymbol: block opens on symbols this trader is
	// recently net-losing on (≥2 closed trades, realized PnL < 0).
	// nil/true = block; false = off.
	NegativeEdgeBlockLosingSymbol *bool `json:"negative_edge_block_losing_symbol,omitempty"`
	// StockWeekendNoOpen: block new opens on Binance tokenized stocks
	// (underlyingSubType "Stocks") during the US-market weekend (Sat/Sun ET)
	// only. Weekday pre-market, regular session, after-hours, and overnight
	// remain eligible and continue to use the 15m execution gate. nil/true =
	// block weekends (default ON); false = allow. Closes, SL/TP fills and
	// drawdown-protect are unaffected. (CODE ENFORCED)
	StockWeekendNoOpen *bool `json:"stock_weekend_no_open,omitempty"`
	// ProfitLockAtR: the R-multiple (PnL ÷ initial stop distance) that arms
	// the profit lock — at this level the program market-trims 50% of the
	// position (once) AND moves the stop-loss to entry (breakeven). The
	// remaining half rides to the structural TP. 0 = default 1R; negative =
	// disabled. Supersedes the ROE trim tier of the TP ladder while active;
	// the 25% full-close backstop and drawdown-protect stay. (CODE ENFORCED)
	ProfitLockAtR float64 `json:"profit_lock_at_r"`
	// ProfitLockBEOffsetR: how far PAST entry (long; mirror for short) the
	// 1R lock parks the stop, in units of the opening risk R. 0.2 = entry
	// +0.2R — locks a sliver of profit so a post-lock pullback can't scratch
	// the runner back to flat. 0 = default 0.2 (09-21 user experiment);
	// negative = pure breakeven at entry (legacy).
	ProfitLockBEOffsetR float64 `json:"profit_lock_be_offset_r"`
	// TPCloseFraction: the fraction of the position the take-profit algo
	// closes at the planned structure level. The remainder stays on as a
	// trend-runner under the trailing stop (2×ATR ratchet) — a resting
	// full-size TP structurally sold every spike top (BTCUSDT 2026-09-21:
	// TP filled 83000, price printed 84275 in the same minute). 0 = default
	// 0.5 (09-21 user experiment); negative = 1.0 (legacy full close).
	// Collapses to 1.0 when trailing_stop_enabled is off — a runner without
	// a ratchet just gives the move back. (CODE ENFORCED)
	TPCloseFraction float64 `json:"tp_close_fraction"`
	// TP ladder on leveraged PnL% (CODE ENFORCED): at >= TpTrimProfitPct the
	// program market-trims 1/3 of the position (once per position); at >=
	// TpFullProfitPct it closes the rest. 0 = defaults 10/25; negative = that
	// tier off. Exchange SL/TP and drawdown-protect unaffected.
	TpTrimProfitPct float64 `json:"tp_trim_profit_pct"`
	TpFullProfitPct float64 `json:"tp_full_profit_pct"`
	// TpFullYieldsToLock: when the 1R profit lock is active, the ROE full
	// tier (default 25% leveraged) ALSO yields to it, not just the trim tier.
	// Without this, a 5%-price spike at 5x closes the whole position at the
	// ladder step while the trend-runner design wants the rest riding to the
	// structural target (QUANT_REVIEW_2026-09-22 C3). Default OFF = current
	// behavior — flipping it mid-experiment would corrupt the 2-week R
	// distribution retest, so the user opts in. (CODE ENFORCED when true)
	TpFullYieldsToLock bool `json:"tp_full_yields_to_lock"`
	// BreakevenArmR: intermediate breakeven tier — when a position's
	// favorable excursion reaches this many × initial stop distance, the
	// stop moves to entry(+profit_lock_be_offset_r) EARLY, without any
	// trim. Fills the (0, 1R) give-back zone: SOLUSDT 2026-09-23 peaked
	// 1.03R for 8 minutes but the 5-min cycle check saw 0.99R and the
	// trade died at −1.07R; the R-normalized give-back table shows 9/21
	// losers had peaked ≥0.3R. 0 = disabled (default), negative = disabled.
	// Simulated on the 09-17→09-24 cohort: arming at 0.5R ≈ +6.4R vs
	// actual with zero winners scratched. (CODE ENFORCED when >0)
	BreakevenArmR float64 `json:"breakeven_arm_r"`
	// TPMaxR: take-profit cap in R (initial risk = |entry − stop_loss| of the
	// open decision). The planned TP is clamped to the CLOSER of itself and
	// entry ± TPMaxR × R before the order is routed (market or limit), so the
	// exchange TP leg and the recorded TP both carry the capped price. Backtest
	// 2026-10-11 (242 live trades replayed with the live exit rules): a 2R cap
	// lifts expectancy by +0.024R/trade (paired 95% CI [+0.001, +0.049]),
	// positive in both halves of every subset — the median planned TP is 2.6R
	// and 79% of plans exceed 2R, but price rarely gets there. 0 = no cap
	// (default); negative = no cap; values in (0, 1) are invalid (a cap below
	// 1R would contradict min_risk_reward_ratio) and are treated as no cap.
	// The capped plan must still satisfy min_risk_reward_ratio, else the open
	// is rejected. Crypto USDT-M futures opens only. (CODE ENFORCED when ≥1)
	TPMaxR float64 `json:"tp_max_r"`

	// TpTrimAtR / TPFullAtR: R-based TP-ladder tiers — 2026-09-29 unit
	// unification. The legacy tp_trim_profit_pct / tp_full_profit_pct price
	// their tiers in leveraged ROE, so the SAME signal exits at different
	// points depending on the leverage the model happens to report (15% ROE
	// ≈ 0.5R at 10× but 1.67R at 3× on a 3.5% stop), and the live 120%-ROE
	// full tier is unreachable at 3× (a 40% price move). R units make the
	// ladder a function of the trade's risk structure, not of leverage.
	//   >0  — the R tier overrides the legacy ROE field;
	//   <0  — that tier is OFF (the full tier's job is owned by the structure
	//         TP algo + trailing stop + drawdown-protect);
	//    0  — fall back to the legacy ROE field (back-compat).
	// Exchange SL/TP and drawdown-protect unaffected. (CODE ENFORCED when ≠0)
	TPTrimAtR float64 `json:"tp_trim_at_r"`
	TPFullAtR float64 `json:"tp_full_at_r"`
	// PeakDrawdownArmR / PeakDrawdownGivebackR: R-based drawdown protect —
	// when the peak favorable excursion reaches ArmR × initial risk and then
	// gives back ≥ GivebackR of that peak, the program closes. The legacy
	// peak_drawdown_min_profit_pct/max_dd_pct pair (5%/55% leveraged ROE)
	// arms at 0.42R on a typical 3×/4%-stop position — the same price zone
	// as the 0.5R breakeven arm, i.e. it closes winners in pure noise.
	// ArmR > 0 switches the drawdown monitor to R units (GivebackR is a
	// fraction of the peak, 0 = default 0.5); 0 = legacy ROE fields.
	// (CODE ENFORCED when ArmR > 0)
	PeakDrawdownArmR      float64 `json:"peak_drawdown_arm_r"`
	PeakDrawdownGivebackR float64 `json:"peak_drawdown_giveback_r"`

	// TpTrimYieldsToLock: whether the ROE trim tier (tp_trim_profit_pct)
	// yields to an active 1R lock. nil/true = 09-21 default — the lock
	// supersedes the trim tier entirely, so tp_trim_profit_pct is DEAD TEXT
	// while the lock is on (the CAPUSDT 2026-09-24 surprise: user set trim=15
	// but the peak at 17% ROE sat in the lock's dead zone below 1R=21.6% and
	// nothing ever fired). false = division of labor: the trim tier fires at
	// its threshold (trim 1/3, once), the 1R lock moves the stop to
	// breakeven(+offset) but does NOT reduce — no double harvest.
	// (CODE ENFORCED either way)
	TpTrimYieldsToLock *bool `json:"tp_trim_yields_to_lock,omitempty"`
	// EarlyCloseMinHours: AI-initiated closes before this many hours of hold
	// time are blocked unless the 1h timeframe shows ≥2 closed candles against
	// the position direction (trend-change evidence). Exchange SL/TP triggers
	// and the drawdown-protect close bypass it by construction (neither is an
	// AI close decision). 0 = default 4h; negative = disabled. (CODE ENFORCED)
	EarlyCloseMinHours int `json:"early_close_min_hours"`
	// MaxEntrySlippageBps: the maximum ADVERSE fill slippage a MARKET open
	// may realize vs its pre-trade checked price (F21c, 2026-10-01 review:
	// only a 100bps alert existed). 0/negative = disabled (alert-only). When
	// armed and breached, the just-opened position is closed immediately
	// (emergency close) and the open is recorded as FAILED — a fill this far
	// from the validated price is a different trade than the gates approved.
	// (CODE ENFORCED)
	MaxEntrySlippageBps int `json:"max_entry_slippage_bps"`
	// EntryRoundTripCostBps estimates total fees and exit slippage for net RR.
	// Zero uses a conservative 20 bps estimate; configure for the venue/tier.
	EntryRoundTripCostBps float64 `json:"entry_round_trip_cost_bps"`
	// ---- Long-side entry discipline (user design 2026-10-01) ----
	// LongPullbackEntry: nil/true = breakout-confirmed LONG anchors at the
	// BROKEN level (retest entry); false = legacy offset anchors.
	LongPullbackEntry *bool `json:"long_pullback_entry,omitempty"`
	// LongMaxEMA20DistPct: chase entries above this distance from the 4h
	// EMA20 are blocked (EMA20_STRETCH); the retest-anchor path is exempt.
	// 0 = default 12; negative = off.
	LongMaxEMA20DistPct float64 `json:"long_max_ema20_dist_pct"`
	// ---- Anti-chase symmetric entry gates (Opus analysis of 273 live trades,
	// 2026-10-11): entries far extended from the 1h EMA20, entries into an
	// exhausted directional 1h RSI, entries after a big 4h move in the trade
	// direction, and counter-trend entries far on the wrong side of the 1h
	// EMA20 all lost heavily. Filtered set (all four gates) actual +0.003R vs
	// rejected set -0.241R over the 273 trades. All four are symmetric
	// (long/short mirror), measured from the gate's entry price, exempt for
	// US-equity (bstock) symbols, and fail OPEN on missing data. ----
	// ChaseMaxEMA20_1hPct: block an entry whose price is more than this many
	// percent beyond the 1h EMA20 IN the trade direction (long: entry above
	// the EMA20 by > X%; short: entry below it by > X%) -> CHASE_EMA20_1H_x_GT_y.
	// Evidence: >8% in the trade direction -> next-4h -4.8%, win rate 32%.
	// Unit: percent. 0 = default 6; negative = off. (CODE ENFORCED)
	ChaseMaxEMA20_1hPct float64 `json:"chase_max_ema20_1h_pct"`
	// CounterMaxEMA20_1hPct: block an entry whose price is more than this many
	// percent on the WRONG side of the 1h EMA20 (long: entry below the EMA20 by
	// > X% = knife-catching; short: entry above it by > X% = fading a pump)
	// -> COUNTER_EMA20_1H_x_GT_y. Evidence: wrong side by > 2% -> next-4h -5.3%.
	// Unit: percent. 0 = default 3; negative = off. (CODE ENFORCED)
	CounterMaxEMA20_1hPct float64 `json:"counter_max_ema20_1h_pct"`
	// EntryRSI1hMax: directional 1h RSI14 exhaustion cap. Long: block when the
	// 1h RSI14 >= X; short: block when it <= 100-X -> RSI1H_EXHAUSTED_x.
	// Evidence: directional RSI1h > 80 -> next-4h -2.25%.
	// Unit: RSI points (0-100). 0 = default 78; negative = off. (CODE ENFORCED)
	EntryRSI1hMax float64 `json:"entry_rsi1h_max"`
	// ChaseMaxMove4hPct: block an entry after the price already moved more than
	// this many percent over the last 4h IN the trade direction (long: +X% up;
	// short: X% down) -> CHASE_MOVE4H_x_GT_y. Evidence: 4h move > 8% in the
	// trade direction -> next-4h -3.1%.
	// Unit: percent. 0 = default 6; negative = off. (CODE ENFORCED)
	ChaseMaxMove4hPct float64 `json:"chase_max_move4h_pct"`
	// BTCFilterLong: nil/true = altcoin longs need coin 24h return ≥ BTC's
	// and BTC 4h closes not declining (BTC_WEAK_LONG).
	BTCFilterLong *bool `json:"btc_filter_long,omitempty"`
	// BTCFilterShort: nil/false = OFF. true = open_short is blocked while
	// the shared BTC 4h classifier reads strongBull (EMA20>EMA50, close
	// above, RSI≥60 → BTC_4H_STRONGBULL). Deliberately opt-in: the playbook
	// handles BTC strength for shorts at the SCAN level (btc_bull ×0.85
	// haircut, same shared classifier), so a hard gate default-on would
	// contradict §9.2 — flip it on to hard-pause alt shorts in BTC bulls.
	BTCFilterShort *bool `json:"btc_filter_short,omitempty"`
	// ShortTopConfirmGate: nil/true = open_short on a short_scan candidate
	// requires the scanner's topping confirmation (顶背离/假突破/破EMA20/
	// 费率回落 — the playbook's 默认姿态, previously prompt-only; now the
	// program cites SHORT_TOP_CONFIRM_MISSING). Candidates WITHOUT short_scan
	// evidence (piggy breakdown, model-initiated) are not gated — the 15m
	// MICRO_TREND gate still applies to them. false = off.
	ShortTopConfirmGate *bool `json:"short_top_confirm_gate,omitempty"`
	// SentimentLongDeweightPts: directional-score penalty for LONGS while
	// crypto FNG ≥ SentimentLongDeweightFNG. 0 = default 10; negative = off.
	SentimentLongDeweightPts int `json:"sentiment_long_deweight_pts"`
	// SentimentLongDeweightFNG: the FNG threshold arming the deweight.
	// 0/unset = 70.
	SentimentLongDeweightFNG int `json:"sentiment_long_deweight_fng"`
	// MaxStopDistancePct: a stop plan further than this from the entry blocks
	// the open (WIDE_STOP) — a wide stop on a violent coin fits the 2%-risk
	// sizing geometry but not its win rate (USUSDT 16.8% stop died in 4
	// minutes). Applies to BOTH directions. 0 = default 10; negative = off.
	MaxStopDistancePct float64 `json:"max_stop_distance_pct"`
	// TPMenuEnabled: nil/true = the tp_options menu + exit_mode contract is
	// active — the model picks a program-precomputed TP plan (tp_option) and
	// classifies the regime (exit_mode trend/range/quick); false = the legacy
	// verbatim first_rr_ge_target contract. The menu can only move the TP
	// BETWEEN precomputed levels; missing/invalid choices degrade to option 1
	// + trend, i.e. exactly the old behavior.
	TPMenuEnabled *bool `json:"tp_menu_enabled,omitempty"`
	// TimeStopHours: the quick exit-mode's time stop — a position still at a
	// loss (price basis) after this many hours is closed by the program (a
	// program path: early-close/min-hold gates don't apply). 0 = default 4h;
	// negative = disabled (quick degrades to range). (CODE ENFORCED when >0)
	TimeStopHours int `json:"time_stop_hours"`
	// Block open_short when the 1d trend is up (counter-trend protection). (CODE ENFORCED)
	BlockShort1dUptrend bool `json:"block_short_1d_uptrend"`
	// Entry timing gate: the finest sub-hour timeframe (15m/30m) trend must
	// align with the entry direction — longs need up/pullback, shorts need
	// down/rally (selling the bounce in a downtrend); "range" blocks both.
	// Symmetric four-quadrant policy (audit 09-13). (CODE ENFORCED)
	EntryTimingGate bool `json:"entry_timing_gate"`
	// Risk-based position sizing: position value is capped at
	// equity × RiskPerTradePct% ÷ stop-distance% (defaults to 1.5% when
	// unset). Sizing derives FROM the stop, not the other way round. (CODE ENFORCED)
	RiskPerTradePct float64 `json:"risk_per_trade_pct"`
	// Stop-distance floor: reject entries whose stop is closer than
	// SLMinATRMult × ATR(1h) — inside normal noise. 0 disables. (CODE ENFORCED)
	SLMinATRMult float64 `json:"sl_min_atr_mult"`
	// Limit entry state machine: the AI may emit open_long_limit /
	// open_short_limit with price = trigger level; the system places a LIMIT
	// order cancels it after max(30min, LimitEntryMaxCycles × scan interval)
	// unfilled — time-based, since cycle length varies with the scan
	// interval. (CODE ENFORCED)
	LimitEntryEnabled   bool `json:"limit_entry_enabled"`
	LimitEntryMaxCycles int  `json:"limit_entry_max_cycles"`
	// LimitEntryOffsetPct is the pre-computed limit-entry anchor offset from
	// the snapshot live price, in percent: buy limit below / sell limit above.
	// Default 0.5 when unset; used as the fixed-mode value AND as the
	// fallback when ATR is unavailable. In the default "atr" mode the offset
	// is clamped to [LimitEntryOffsetMinPct, LimitEntryOffsetMaxPct]
	// (defaults 0.15%-1.2%, kernel/anchor_offset.go). (CODE ENFORCED)
	LimitEntryOffsetPct float64 `json:"limit_entry_offset_pct"`
	// LimitEntryOffsetMode selects how the anchor offset scales:
	// "atr" (default when empty) = OffsetATRMult × ATR(execution TF) clamped
	// to [OffsetMinPct, OffsetMaxPct] — a fixed percent is either most of a
	// quiet symbol's ATR or a hair off the live price for violent movers;
	// "fixed" = LimitEntryOffsetPct as-is. The same scaling drives the
	// anchor-vs-structure breathing-room threshold (suppression pre-filter
	// and the execution supply-zone gate share it). (CODE ENFORCED)
	LimitEntryOffsetMode    string  `json:"limit_entry_offset_mode,omitempty"`
	LimitEntryOffsetATRMult float64 `json:"limit_entry_offset_atr_mult"`
	LimitEntryOffsetMinPct  float64 `json:"limit_entry_offset_min_pct"`
	LimitEntryOffsetMaxPct  float64 `json:"limit_entry_offset_max_pct"`
	// LimitEntryMarketFallback: the anchor is computed at prompt-build time,
	// but the AI latency window (minutes) lets the live price cross it. A long
	// anchor at/above the live price (short anchor at/below) means the planned
	// pullback/rally already arrived — the resting order would fill
	// immediately at a price at least as good as the anchor — so the entry is
	// converted to MARKET with every risk gate re-validated at the live price
	// (and rejected outright when the price is already at/beyond the SL).
	// Default on (nil = enabled); set false to keep rejecting crossed
	// anchors. (CODE ENFORCED)
	LimitEntryMarketFallback *bool `json:"limit_entry_market_fallback,omitempty"`
	// Volatility-targeted position sizing, recomputed every cycle with an
	// 80/120 hysteresis band (reduce-only automation; adds stay AI-driven).
	VolTargetEnabled bool `json:"vol_target_enabled"`
	// Rule-based trailing stop: arms at 1.5× initial stop distance, trails at
	// 2×ATR(1h), monotonic tighten-only. (CODE ENFORCED)
	TrailingStopEnabled bool `json:"trailing_stop_enabled"`
	// Resistance-breakout hold: block an AI-initiated close of a LOSING
	// position when the nearest opposite-side structure level (overhead
	// resistance for longs / support below for shorts) is within
	// CloseRejectBreakoutPct% and the near-TF (15m) structure is NOT yet
	// broken — give the breakout/breakdown room instead of exiting on
	// "resistance rejection". 0 disables. (CODE ENFORCED)
	CloseRejectBreakoutPct float64 `json:"close_reject_breakout_pct"`
	// Supply-zone entry block: reject an open_long_limit / open_short_limit
	// whose anchor sits within OpenRejectSupplyPct% of the opposite-side
	// structure (below resistance for longs / above support for shorts) —
	// the fill would land inside the supply/demand zone with no room.
	// 0 disables. (CODE ENFORCED)
	OpenRejectSupplyPct float64 `json:"open_reject_supply_pct"`
	// Extended-pump long guard (暴涨延伸做多确认门): when a coin's 4h
	// trend-window return is at/above this percent, the EMA "up" label lags
	// the round-trip by days — longs then need a CONFIRMED pullback (15m
	// close back above its EMA20 AND a higher 15m swing low), otherwise the
	// long direction is blocked (EXTENDED_PUMP_UNCONFIRMED). Vertical pumps
	// bought as "trend pullbacks" round-trip hard (FILUSDT/SAGAUSDT
	// 2026-09-19: 4h +29%, −5.2%/−3.6% stop-outs within hours).
	// 0 = built-in default (20), negative = guard disabled. (CODE ENFORCED)
	PumpGuard4hPct float64 `json:"pump_guard_4h_pct"`
	// Loss-streak circuit breaker: when a symbol closes LossStreakMaxLosses
	// consecutive losing trades within the last 24h, new opens on that symbol
	// are blocked for 24h from the third loss (recomputed statelessly from
	// the closed-trade record, restart-safe). (CODE ENFORCED)
	LossStreakBanEnabled bool `json:"loss_streak_ban_enabled"`
	LossStreakMaxLosses  int  `json:"loss_streak_max_losses"`
	// Drawdown protection close: when a position's peak profit (margin
	// basis) reaches PeakDrawdownMinProfitPct% and then gives back
	// PeakDrawdownMaxDrawdownPct% of that peak, the program closes it to
	// lock the remainder. Defaults 5 / 55 when unset. (CODE ENFORCED)
	PeakDrawdownMinProfitPct float64 `json:"peak_drawdown_min_profit_pct"`
	PeakDrawdownMaxDDPct     float64 `json:"peak_drawdown_max_dd_pct"`
	// Account-level circuit breaker: when account equity sits
	// AccountMaxDrawdownPct% below the initial balance, ALL new opens are
	// blocked (closes/reduces still allowed) — the per-symbol loss-streak
	// breaker cannot catch a negative-edge strategy bleeding across many
	// symbols. 0 disables. (CODE ENFORCED)
	AccountMaxDrawdownPct float64 `json:"account_max_drawdown_pct"`
	// MaxAccountRiskPct caps TOTAL open stop-risk: Σ over positions of
	// qty×|entry−SL| (unprotected positions worst-cased at the 8% stop-band
	// cap) plus the new trade's risk ≤ pct×equity. Position COUNT caps say
	// nothing about correlation — five same-direction altcoin stops are one
	// big position; this is the cap that bounds a full-load stop-out.
	// 0 = default 10, negative = disabled. (CODE ENFORCED, QUANT_REVIEW
	// 2026-09-22 D2)
	MaxAccountRiskPct float64 `json:"max_account_risk_pct"`
	// MaxNetDirectionalRiskPct caps |long stop-risk - short stop-risk| as a
	// percentage of equity, including resting entries and decisions already
	// admitted in the current cycle. This cheaply limits correlated one-way
	// concentration that a gross-risk sum cannot see. 0 = default 6,
	// negative = disabled. (CODE ENFORCED)
	MaxNetDirectionalRiskPct float64 `json:"max_net_directional_risk_pct"`
	// DailyMaxLossPct halts new opens for the rest of the UTC day once equity
	// is down pct% from that day's first-seen equity. Closes/SL/TP unaffected.
	// 0 = default 10, negative = disabled. (CODE ENFORCED, QUANT_REVIEW
	// 2026-09-22 D2 — restores the lost daily-halt design; dailyPnL existed
	// but nothing consumed it)
	DailyMaxLossPct float64 `json:"daily_max_loss_pct"`
}

// EffectiveMaxAccountRiskPct resolves the account risk-exposure cap:
// 0/unset → DefaultMaxAccountRiskPct, negative → disabled (0).
func (r RiskControlConfig) EffectiveMaxAccountRiskPct() float64 {
	if r.MaxAccountRiskPct < 0 {
		return 0
	}
	if r.MaxAccountRiskPct == 0 {
		return DefaultMaxAccountRiskPct
	}
	return r.MaxAccountRiskPct
}

// EffectiveMaxNetDirectionalRiskPct resolves the one-way concentration cap:
// 0/unset -> DefaultMaxNetDirectionalRiskPct, negative -> disabled (0).
func (r RiskControlConfig) EffectiveMaxNetDirectionalRiskPct() float64 {
	if r.MaxNetDirectionalRiskPct < 0 {
		return 0
	}
	if r.MaxNetDirectionalRiskPct == 0 {
		return DefaultMaxNetDirectionalRiskPct
	}
	return r.MaxNetDirectionalRiskPct
}

// ============================================================================
// Long-side entry discipline (user design 2026-10-01, "保留突破,改入场方式"):
// breakout-confirmed longs enter on the RETEST of the broken level instead of
// chasing the extension, with symmetric evidence and market-level filters.
// All knobs follow the house style: pointer nil = default, negative = off.
// ============================================================================

// LongPullbackEntry: nil/true = a breakout-confirmed LONG's limit anchor is
// the BROKEN resistance level (old resistance becomes support) instead of
// snapshot−offset — the model waits for the retest rather than chasing.
// false = legacy offset anchors. (CODE ENFORCED)
func (r RiskControlConfig) EffectiveLongPullbackEntry() bool {
	return r.LongPullbackEntry == nil || *r.LongPullbackEntry
}

// DefaultLongMaxEMA20DistPct: how far above the 4h EMA20 a CHASE entry may
// still be considered; beyond it the coin is "extended from the mean" and
// longs wait for the pullback anchor (which by construction closes most of
// the stretch). The retest-anchor path is EXEMPT — waiting for the level is
// the designed alternative, not a chase.
const DefaultLongMaxEMA20DistPct = 12.0

// EffectiveLongMaxEMA20DistPct: 0/unset -> default 12 (%), negative -> off.
func (r RiskControlConfig) EffectiveLongMaxEMA20DistPct() float64 {
	if r.LongMaxEMA20DistPct < 0 {
		return 0
	}
	if r.LongMaxEMA20DistPct == 0 {
		return DefaultLongMaxEMA20DistPct
	}
	return r.LongMaxEMA20DistPct
}

// Anti-chase gate defaults (Opus analysis 2026-10-11, 273 live trades; see the
// field docs on RiskControlConfig for the evidence numbers).
const (
	DefaultChaseMaxEMA20_1hPct   = 6.0
	DefaultCounterMaxEMA20_1hPct = 3.0
	DefaultEntryRSI1hMax         = 78.0
	DefaultChaseMaxMove4hPct     = 6.0
)

// antiChaseKnob resolves a 0=default / negative=off(0) / positive=value knob.
func antiChaseKnob(v, def float64) float64 {
	if v < 0 {
		return 0
	}
	if v == 0 {
		return def
	}
	return v
}

// EffectiveChaseMaxEMA20_1hPct: 0/unset -> default 6 (%), negative -> 0 (off).
func (r RiskControlConfig) EffectiveChaseMaxEMA20_1hPct() float64 {
	return antiChaseKnob(r.ChaseMaxEMA20_1hPct, DefaultChaseMaxEMA20_1hPct)
}

// EffectiveCounterMaxEMA20_1hPct: 0/unset -> default 3 (%), negative -> 0 (off).
func (r RiskControlConfig) EffectiveCounterMaxEMA20_1hPct() float64 {
	return antiChaseKnob(r.CounterMaxEMA20_1hPct, DefaultCounterMaxEMA20_1hPct)
}

// EffectiveEntryRSI1hMax: 0/unset -> default 78, negative -> 0 (off).
func (r RiskControlConfig) EffectiveEntryRSI1hMax() float64 {
	return antiChaseKnob(r.EntryRSI1hMax, DefaultEntryRSI1hMax)
}

// EffectiveChaseMaxMove4hPct: 0/unset -> default 6 (%), negative -> 0 (off).
func (r RiskControlConfig) EffectiveChaseMaxMove4hPct() float64 {
	return antiChaseKnob(r.ChaseMaxMove4hPct, DefaultChaseMaxMove4hPct)
}

// EffectiveBTCFilterLong: nil/true = altcoin LONGs require the coin's 24h
// return ≥ BTC's AND BTC's 4h closes not in a declining sequence. false = off.
func (r RiskControlConfig) EffectiveBTCFilterLong() bool {
	return r.BTCFilterLong == nil || *r.BTCFilterLong
}

// EffectiveBTCFilterShort: nil/false = OFF (opt-in hard gate — see the field
// comment: the playbook treats BTC strength for shorts at scan level).
func (r RiskControlConfig) EffectiveBTCFilterShort() bool {
	return r.BTCFilterShort != nil && *r.BTCFilterShort
}

// EffectiveShortTopConfirmGate: nil/true = the playbook's 默认姿态
// (topping confirmation required for short_scan shorts) is program-enforced.
func (r RiskControlConfig) EffectiveShortTopConfirmGate() bool {
	return r.ShortTopConfirmGate == nil || *r.ShortTopConfirmGate
}

// DefaultSentimentLongDeweightPts: while crypto FNG reads ≥ the threshold,
// a long's directional score carries this many penalty points at the gate
// reads (CONSENSUS_OPPOSED / NEG_EDGE_SCORE / breakout exception) — the
// greed backdrop de-ranks chasing, it does not block.
const DefaultSentimentLongDeweightPts = 10

// EffectiveSentimentLongDeweightPts: 0/unset -> default 10, negative -> off.
func (r RiskControlConfig) EffectiveSentimentLongDeweightPts() int {
	if r.SentimentLongDeweightPts < 0 {
		return 0
	}
	if r.SentimentLongDeweightPts == 0 {
		return DefaultSentimentLongDeweightPts
	}
	return r.SentimentLongDeweightPts
}

// SentimentLongDeweightFNG: the crypto Fear&Greed value at or above which
// the long deweight arms. 0/unset -> 70.
func (r RiskControlConfig) EffectiveSentimentLongDeweightFNG() int {
	if r.SentimentLongDeweightFNG <= 0 {
		return 70
	}
	return r.SentimentLongDeweightFNG
}

// DefaultMaxStopDistancePct: the stop-plan distance cap for BOTH directions
// (user directive 2026-10-02, WIDE_STOP).
const DefaultMaxStopDistancePct = 10.0

// EffectiveMaxStopDistancePct: 0/unset -> default 10 (%), negative -> off.
func (r RiskControlConfig) EffectiveMaxStopDistancePct() float64 {
	if r.MaxStopDistancePct < 0 {
		return 0
	}
	if r.MaxStopDistancePct == 0 {
		return DefaultMaxStopDistancePct
	}
	return r.MaxStopDistancePct
}

// EffectiveDailyMaxLossPct resolves the daily-loss halt threshold:
// 0/unset → DefaultDailyMaxLossPct, negative → disabled (0).
func (r RiskControlConfig) EffectiveDailyMaxLossPct() float64 {
	if r.DailyMaxLossPct < 0 {
		return 0
	}
	if r.DailyMaxLossPct == 0 {
		return DefaultDailyMaxLossPct
	}
	return r.DailyMaxLossPct
}

// NewStrategyStore creates a new StrategyStore
func NewStrategyStore(db *gorm.DB) *StrategyStore {
	return &StrategyStore{db: db}
}

// DefaultStatsWindowDays is the rolling stats window when the strategy
// config leaves stats_window_days unset.
const DefaultStatsWindowDays = 30

// DefaultUSStockSessionBoostPct backs EffectiveUSStockSessionBoostPct.
const DefaultUSStockSessionBoostPct = 20.0

// TrimYieldsToLock resolves the trim-tier-vs-lock relationship: nil/true =
// the 09-21 default (trim tier superseded by the lock), false = trim tier
// active alongside a breakeven-only lock.
func (r RiskControlConfig) TrimYieldsToLock() bool {
	return r.TpTrimYieldsToLock == nil || *r.TpTrimYieldsToLock
}

// EffectiveUSStockSessionBoostPct resolves the US-session equity weighting:
// 0/unset → DefaultUSStockSessionBoostPct, negative → disabled (0).
func (c *StrategyConfig) EffectiveUSStockSessionBoostPct() float64 {
	if c.USStockSessionBoostPct < 0 {
		return 0
	}
	if c.USStockSessionBoostPct == 0 {
		return DefaultUSStockSessionBoostPct
	}
	return c.USStockSessionBoostPct
}

// DefaultMaxAccountRiskPct / DefaultDailyMaxLossPct back the effective
// helpers above — rendered into the prompt from these constants so the text
// can never drift from the enforced value.
const (
	DefaultMaxAccountRiskPct        = 10.0
	DefaultMaxNetDirectionalRiskPct = 6.0
	DefaultDailyMaxLossPct          = 10.0
)

// EffectiveStatsWindowDays resolves the stats window in days: 0 means
// "full history" (explicitly negative config), otherwise the window length
// (DefaultStatsWindowDays when unset/0).
func (c *StrategyConfig) EffectiveStatsWindowDays() int {
	switch {
	case c.StatsWindowDays < 0:
		return 0
	case c.StatsWindowDays == 0:
		return DefaultStatsWindowDays
	default:
		return c.StatsWindowDays
	}
}

func (s *StrategyStore) initTables() error {
	// AutoMigrate will add missing columns without dropping existing data
	if err := s.db.AutoMigrate(&Strategy{}); err != nil {
		return err
	}
	// Append-only config version history (2026-10-04: strategy iteration
	// traceability — see store/strategy_version.go).
	return s.initVersionTables()
}

func (s *StrategyStore) initDefaultData() error {
	// No longer pre-populate strategies - create on demand when user configures
	return nil
}

// GetDefaultStrategyConfig returns the default strategy configuration for the given language
func GetDefaultStrategyConfig(lang string) StrategyConfig {
	// Normalize language to "zh" or "en"
	normalizedLang := "en"
	if lang == "zh" {
		normalizedLang = "zh"
	}

	config := StrategyConfig{
		Language: normalizedLang,
		CoinSource: CoinSourceConfig{
			SourceType: "ai500",
			UseAI500:   true,
			AI500Limit: 3,
			UseOITop:   false,
			OITopLimit: 3,
			UseOILow:   false,
			OILowLimit: 3,
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "5m",
				PrimaryCount:         20,
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableRawKlines:   true, // Required - raw OHLCV data for AI analysis
			EnableEMA:         false,
			EnableMACD:        false,
			EnableRSI:         false,
			EnableATR:         false,
			EnableBOLL:        false,
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
			EMAPeriods:        []int{20, 50},
			RSIPeriods:        []int{7, 14},
			ATRPeriods:        []int{14},
			BOLLPeriods:       []int{20},
			// NofxOS unified API key
			NofxOSAPIKey: "cm_568c67eae410d912c54c",
			// Quant data
			EnableQuantData:    true,
			EnableQuantOI:      true,
			EnableQuantNetflow: true,
			// OI ranking data
			EnableOIRanking:   true,
			OIRankingDuration: "1h",
			OIRankingLimit:    10,
			// NetFlow ranking data
			EnableNetFlowRanking:   true,
			NetFlowRankingDuration: "1h",
			NetFlowRankingLimit:    10,
			// Price ranking data
			EnablePriceRanking:   true,
			PriceRankingDuration: "1h,4h,24h",
			PriceRankingLimit:    10,
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                 3,   // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:            5,   // BTC/ETH exchange leverage ceiling (CODE ENFORCED)
			AltcoinMaxLeverage:           5,   // Altcoin exchange leverage ceiling (CODE ENFORCED)
			BTCETHMaxPositionValueRatio:  5.0, // BTC/ETH: max position = 5x equity (CODE ENFORCED)
			AltcoinMaxPositionValueRatio: 1.0, // Altcoin: max position = 1x equity (CODE ENFORCED)
			MaxMarginUsage:               0.9, // Max 90% margin usage (CODE ENFORCED)
			MinPositionSize:              12,  // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:           3.0, // Min 3:1 profit/loss ratio (CODE ENFORCED at open — struct field comment is the source of truth)
			MinConfidence:                75,  // Min 75% confidence (CODE ENFORCED)

			MaxAccountRiskPct:        10.0, // Σ open stop-risk + new risk ≤ 10% equity (CODE ENFORCED)
			MaxNetDirectionalRiskPct: 6.0,  // |long risk-short risk| ≤ 6% equity (CODE ENFORCED)
			DailyMaxLossPct:          10.0, // Daily-loss halt: opens blocked at −10% from day-start equity (CODE ENFORCED)
		},
	}

	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# 你是一个专业的加密货币交易AI

你的任务是根据提供的市场数据做出交易决策。你是一个经验丰富的量化交易员，擅长技术分析和风险管理。`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟
如果你发现自己每个周期都在交易 → 标准太低；如果持仓不到30分钟就平仓 → 太冲动。`,
			EntryStandards: `# 🎯 入场标准（严格）

只入场程序硬门放行的设置——hard_entry_gate.allowed 及其 failed 阻断码是唯一权威,禁止用原始指标自行重算推翻。放行的设置里按结构与风险回报择优,避免信号矛盾、横盘震荡、或平仓后立即重新开仓等低质量行为。`,
			DecisionProcess: `# 📋 决策流程

1. 检查持仓 → 是否止盈/止损
2. 扫描候选币种 + 多时间框架 → 是否存在强信号
3. 先写思维链，再输出结构化JSON`,
		}
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# You are a professional cryptocurrency trading AI

Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in technical analysis and risk management.`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent trader: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- >2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes
If you find yourself trading every cycle → standards are too low; if closing positions in <30 minutes → too impulsive.`,
			EntryStandards: `# 🎯 Entry Standards (Strict)

Only enter setups the program's hard entry gate allows — hard_entry_gate.allowed and its failed codes are the single source of truth; never re-derive or overturn them from raw indicators. Among allowed setups, pick by structure and risk-reward; avoid contradictory signals, sideways oscillation, or immediately restarting after closing positions.`,
			DecisionProcess: `# 📋 Decision Process

1. Check positions → whether to take profit/stop loss
2. Scan candidate coins + multi-timeframe → whether strong signals exist
3. Write a SHORT decision_summary first (cited program fields/block codes only), then output the strict JSON`,
		}
	}

	return config
}

// Create create a strategy
func (s *StrategyStore) Create(strategy *Strategy) error {
	return s.db.Create(strategy).Error
}

// Update update a strategy
var ErrStrategyConflict = fmt.Errorf("strategy changed since it was loaded")

func (s *StrategyStore) Update(strategy *Strategy, expectedUpdatedAt ...time.Time) error {
	query := s.db.Model(&Strategy{}).Where("id = ? AND user_id = ?", strategy.ID, strategy.UserID)
	if len(expectedUpdatedAt) > 0 {
		query = query.Where("updated_at = ?", expectedUpdatedAt[0])
	}
	writtenAt := time.Now().UTC()
	result := query.Updates(map[string]interface{}{
		"name":           strategy.Name,
		"description":    strategy.Description,
		"config":         strategy.Config,
		"is_public":      strategy.IsPublic,
		"config_visible": strategy.ConfigVisible,
		"updated_at":     writtenAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrStrategyConflict
	}
	strategy.UpdatedAt = writtenAt
	return nil
}

// Delete delete a strategy
func (s *StrategyStore) Delete(userID, id string) error {
	// do not allow deleting system default strategy
	var st Strategy
	if err := s.db.Where("id = ?", id).First(&st).Error; err == nil {
		if st.IsDefault {
			return fmt.Errorf("cannot delete system default strategy")
		}
	}

	// Check if any trader references this strategy
	var count int64
	if err := s.db.Model(&Trader{}).
		Where("user_id = ? AND strategy_id = ?", userID, id).
		Count(&count).Error; err == nil && count > 0 {
		return fmt.Errorf("cannot delete strategy in use by %d trader(s) - reassign those traders first", count)
	}

	return s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&Strategy{}).Error
}

// List get user's strategy list
func (s *StrategyStore) List(userID string) ([]*Strategy, error) {
	var strategies []*Strategy
	err := s.db.Where("user_id = ? OR is_default = ?", userID, true).
		Order("is_default DESC, created_at DESC").
		Find(&strategies).Error
	if err != nil {
		return nil, err
	}
	return strategies, nil
}

// ListPublic get all public strategies for the strategy market
func (s *StrategyStore) ListPublic() ([]*Strategy, error) {
	var strategies []*Strategy
	err := s.db.Where("is_public = ?", true).
		Order("created_at DESC").
		Find(&strategies).Error
	if err != nil {
		return nil, err
	}
	return strategies, nil
}

// Get get a single strategy
func (s *StrategyStore) Get(userID, id string) (*Strategy, error) {
	var st Strategy
	err := s.db.Where("id = ? AND (user_id = ? OR is_default = ?)", id, userID, true).
		First(&st).Error
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// GetActive get user's currently active strategy
func (s *StrategyStore) GetActive(userID string) (*Strategy, error) {
	var st Strategy
	err := s.db.Where("user_id = ? AND is_active = ?", userID, true).First(&st).Error
	if err == gorm.ErrRecordNotFound {
		// no active strategy, return system default strategy
		return s.GetDefault()
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// GetDefault get system default strategy
func (s *StrategyStore) GetDefault() (*Strategy, error) {
	var st Strategy
	err := s.db.Where("is_default = ?", true).First(&st).Error
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// SetActive set active strategy (will first deactivate other strategies)
func (s *StrategyStore) SetActive(userID, strategyID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var target Strategy
		if err := tx.Where("id = ? AND user_id = ? AND is_default = ?", strategyID, userID, false).First(&target).Error; err != nil {
			return err
		}
		if err := tx.Model(&Strategy{}).Where("user_id = ? AND is_default = ?", userID, false).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&Strategy{}).Where("id = ? AND user_id = ?", strategyID, userID).Update("is_active", true).Error
	})
}

// Duplicate duplicate a strategy (used to create custom strategy based on default strategy)
func (s *StrategyStore) Duplicate(userID, sourceID, newID, newName string) error {
	// get source strategy
	source, err := s.Get(userID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source strategy: %w", err)
	}

	// create new strategy
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "Created based on [" + source.Name + "]",
		IsActive:    false,
		IsDefault:   false,
		Config:      source.Config,
	}

	return s.Create(newStrategy)
}

// ParseConfig parse strategy configuration JSON
func (s *Strategy) ParseConfig() (*StrategyConfig, error) {
	var config StrategyConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse strategy configuration: %w", err)
	}
	return &config, nil
}

// SetConfig set strategy configuration
func (s *Strategy) SetConfig(config *StrategyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize strategy configuration: %w", err)
	}
	s.Config = string(data)
	return nil
}

// ============================================================================
// Token Estimation
// ============================================================================

// TokenEstimate holds the result of token estimation
type TokenEstimate struct {
	Total       int            `json:"total"`
	Breakdown   TokenBreakdown `json:"breakdown"`
	ModelLimits []ModelLimit   `json:"model_limits"`
	Suggestions []string       `json:"suggestions"`
}

// TokenBreakdown shows estimated tokens per component
type TokenBreakdown struct {
	SystemPrompt  int `json:"system_prompt"`
	MarketData    int `json:"market_data"`
	RankingData   int `json:"ranking_data"`
	QuantData     int `json:"quant_data"`
	FixedOverhead int `json:"fixed_overhead"`
}

// ModelLimit shows token usage against a specific model's context limit
type ModelLimit struct {
	Name         string `json:"name"`
	ContextLimit int    `json:"context_limit"`
	UsagePct     int    `json:"usage_pct"`
	Level        string `json:"level"` // "ok" | "warning" | "danger"
}

// Context window sizes (tokens) for each model family
const (
	contextLimitDeepSeek = 131_072   // 128K
	contextLimitOpenAI   = 128_000   // 128K
	contextLimitClaude   = 200_000   // 200K
	contextLimitQwen     = 131_072   // 128K
	contextLimitGemini   = 1_000_000 // 1M
	contextLimitGrok     = 131_072   // 128K
	contextLimitKimi     = 131_072   // 128K
	contextLimitMinimax  = 1_000_000 // 1M
	contextLimitGLM      = 131_072   // 128K
)

// ModelContextLimits maps provider names to their context window sizes (in tokens)
var ModelContextLimits = map[string]int{
	"deepseek": contextLimitDeepSeek,
	"openai":   contextLimitOpenAI,
	"claude":   contextLimitClaude,
	"qwen":     contextLimitQwen,
	"gemini":   contextLimitGemini,
	"grok":     contextLimitGrok,
	"kimi":     contextLimitKimi,
	"minimax":  contextLimitMinimax,
	"glm":      contextLimitGLM,
}

// GetContextLimit returns the context limit for a given provider
func GetContextLimit(provider string) int {
	if limit, ok := ModelContextLimits[provider]; ok {
		return limit
	}
	return contextLimitDeepSeek // safe default
}

// GetContextLimitForClient returns context limit for a provider+model pair.
func GetContextLimitForClient(provider, model string) int {
	return GetContextLimit(provider)
}

// EstimateTokens estimates the total token count for a strategy configuration.
// This is a pure computation based on config fields — no network calls.
func (c *StrategyConfig) EstimateTokens() TokenEstimate {
	breakdown := TokenBreakdown{}

	// --- System Prompt ---
	// Base system prompt: schema + role + rules + output format
	baseChars := 4000 // English default
	if c.Language == "zh" {
		baseChars = 3000
	}
	// Add prompt sections
	baseChars += len(c.PromptSections.RoleDefinition)
	baseChars += len(c.PromptSections.TradingFrequency)
	baseChars += len(c.PromptSections.EntryStandards)
	baseChars += len(c.PromptSections.DecisionProcess)
	baseChars += len(c.CustomPrompt)

	if c.Language == "zh" {
		breakdown.SystemPrompt = baseChars / 2 // CJK: ~2 chars per token
	} else {
		breakdown.SystemPrompt = baseChars / 4 // English: ~4 chars per token
	}

	// --- Fixed Overhead ---
	// Time, BTC price, account info, section headers
	breakdown.FixedOverhead = 800 / 4 // ~200 tokens

	// --- Market Data ---
	numCoins := c.getEffectiveCoinCount()
	numTimeframes := c.getEffectiveTimeframeCount()
	klineCount := c.Indicators.Klines.PrimaryCount
	if klineCount <= 0 {
		klineCount = 20
	}

	// Raw-kline term must mirror the RENDERER's per-timeframe bar budget —
	// kernel.ResolvePromptKlineBars (store cannot import kernel; mirrored
	// here, contract-tested from kernel). Default 20 bars, clamp [10,
	// primaryCount], negative = legacy full-PrimaryCount escape hatch.
	// (User report 2026-10-01: the estimate still priced the pre-09-27
	// 60-120-bar dumps and warned "108k approaching 131k" while real renders
	// measure ~7-10k tokens.)
	if c.Indicators.Klines.PromptKlineBars >= 0 {
		promptBars := c.Indicators.Klines.PromptKlineBars
		if promptBars == 0 {
			promptBars = 20 // kernel.DefaultPromptKlineBars
		}
		if promptBars < 10 {
			promptBars = 10
		}
		if promptBars > klineCount {
			promptBars = klineCount
		}
		klineCount = promptBars
	}

	// Per coin per timeframe: structured derived fields are constant-size.
	// When raw klines are enabled the prompt also carries five numeric values
	// per closed bar, so include that dominant dynamic cost in the advisory
	// estimate. The execution path still measures the fully rendered prompt.
	charsPerCoinTF := 450

	// Add enabled indicator overhead per timeframe
	indicatorCharsPerLine := 0
	if c.Indicators.EnableEMA {
		indicatorCharsPerLine += 20 // EMA values appended
	}
	if c.Indicators.EnableMACD {
		indicatorCharsPerLine += 30
	}
	if c.Indicators.EnableRSI {
		indicatorCharsPerLine += 15
	}
	if c.Indicators.EnableATR {
		indicatorCharsPerLine += 15
	}
	if c.Indicators.EnableBOLL {
		indicatorCharsPerLine += 25
	}
	if c.Indicators.EnableVolume {
		indicatorCharsPerLine += 10
	}
	charsPerCoinTF += indicatorCharsPerLine
	totalMarketChars := numCoins * numTimeframes * charsPerCoinTF
	if c.Indicators.EnableRawKlines {
		// [open,high,low,close,volume] plus commas/brackets. Sixty chars per
		// bar is conservative for the rounded numeric JSON emitted by the
		// signal renderer.
		totalMarketChars += numCoins * numTimeframes * klineCount * 60
	}

	// OI + Funding per coin
	if c.Indicators.EnableOI || c.Indicators.EnableFundingRate {
		totalMarketChars += numCoins * 100
	}

	breakdown.MarketData = totalMarketChars / 4 // numeric data: ~4 chars per token

	// --- Quant Data ---
	if c.Indicators.EnableQuantData {
		quantCharsPerCoin := 0
		if c.Indicators.EnableQuantOI {
			quantCharsPerCoin += 300
		}
		if c.Indicators.EnableQuantNetflow {
			quantCharsPerCoin += 300
		}
		breakdown.QuantData = (numCoins * quantCharsPerCoin) / 4
	}

	// --- Ranking Data ---
	rankingChars := 0
	if c.Indicators.EnableOIRanking {
		limit := c.Indicators.OIRankingLimit
		if limit <= 0 {
			limit = 10
		}
		rankingChars += limit * 60
	}
	if c.Indicators.EnableNetFlowRanking {
		limit := c.Indicators.NetFlowRankingLimit
		if limit <= 0 {
			limit = 10
		}
		rankingChars += limit * 80
	}
	if c.Indicators.EnablePriceRanking {
		limit := c.Indicators.PriceRankingLimit
		if limit <= 0 {
			limit = 10
		}
		// Count durations (comma-separated)
		numDurations := 1
		if c.Indicators.PriceRankingDuration != "" {
			numDurations = len(strings.Split(c.Indicators.PriceRankingDuration, ","))
		}
		rankingChars += limit * numDurations * 40
	}
	breakdown.RankingData = rankingChars / 4

	// --- Total with 15% safety margin ---
	subtotal := breakdown.SystemPrompt + breakdown.MarketData + breakdown.RankingData + breakdown.QuantData + breakdown.FixedOverhead
	total := subtotal * 115 / 100

	// --- Model limits ---
	modelLimits := make([]ModelLimit, 0, len(ModelContextLimits))
	for name, limit := range ModelContextLimits {
		pct := total * 100 / limit
		level := "ok"
		if pct >= 100 {
			level = "danger"
		} else if pct >= 80 {
			level = "warning"
		}
		modelLimits = append(modelLimits, ModelLimit{
			Name:         name,
			ContextLimit: limit,
			UsagePct:     pct,
			Level:        level,
		})
	}

	// Sort by usage_pct desc, then name asc for deterministic order
	sort.Slice(modelLimits, func(i, j int) bool {
		if modelLimits[i].UsagePct != modelLimits[j].UsagePct {
			return modelLimits[i].UsagePct > modelLimits[j].UsagePct
		}
		return modelLimits[i].Name < modelLimits[j].Name
	})

	// --- Suggestions ---
	var suggestions []string
	// Find the strictest model (smallest context)
	minLimit := 0
	for _, limit := range ModelContextLimits {
		if minLimit == 0 || limit < minLimit {
			minLimit = limit
		}
	}
	if minLimit > 0 && total > minLimit {
		if numTimeframes > 1 {
			savedPerTF := (numCoins * klineCount * (80 + indicatorCharsPerLine)) / 4 * 115 / 100
			suggestions = append(suggestions, fmt.Sprintf("Reduce 1 timeframe to save ~%d tokens", savedPerTF))
		}
		if numCoins > 1 {
			savedPerCoin := (numTimeframes * klineCount * (80 + indicatorCharsPerLine)) / 4 * 115 / 100
			suggestions = append(suggestions, fmt.Sprintf("Reduce 1 coin to save ~%d tokens", savedPerCoin))
		}
		if klineCount > 15 {
			suggestions = append(suggestions, "Reduce K-line count to 15 to save tokens")
		}
	}

	return TokenEstimate{
		Total:       total,
		Breakdown:   breakdown,
		ModelLimits: modelLimits,
		Suggestions: suggestions,
	}
}

// getEffectiveCoinCount returns the estimated number of coins that will be analyzed
func (c *StrategyConfig) getEffectiveCoinCount() int {
	count := 0
	switch c.CoinSource.SourceType {
	case "static":
		count = len(c.CoinSource.StaticCoins)
	case "ai500":
		count = c.CoinSource.AI500Limit
	case "oi_top":
		count = c.CoinSource.OITopLimit
	case "oi_low":
		count = c.CoinSource.OILowLimit
	case "piggy_dash":
		count = c.CoinSource.PiggyDashLimit
	case "short_scan":
		count = c.CoinSource.ShortScanLimit
	case "mixed":
		if c.CoinSource.UseAI500 {
			count += c.CoinSource.AI500Limit
		}
		if c.CoinSource.UseOITop {
			count += c.CoinSource.OITopLimit
		}
		if c.CoinSource.UseOILow {
			count += c.CoinSource.OILowLimit
		}
		if c.CoinSource.UsePiggyDash {
			count += c.CoinSource.PiggyDashLimit
		}
		if c.CoinSource.UseShortScan {
			count += c.CoinSource.ShortScanLimit
		}
		if c.CoinSource.UseStatic == nil || *c.CoinSource.UseStatic {
			count += len(c.CoinSource.StaticCoins)
		}
	default:
		count = c.CoinSource.AI500Limit
	}
	if count <= 0 {
		count = 3
	}
	return count
}

// getEffectiveTimeframeCount returns the number of timeframes that will be used
func (c *StrategyConfig) getEffectiveTimeframeCount() int {
	if len(c.Indicators.Klines.SelectedTimeframes) > 0 {
		return len(c.Indicators.Klines.SelectedTimeframes)
	}
	count := 1
	if c.Indicators.Klines.LongerTimeframe != "" {
		count++
	}
	return count
}

func (r RiskControlConfig) EffectiveEntryRoundTripCostBps() float64 {
	if r.EntryRoundTripCostBps > 0 {
		return r.EntryRoundTripCostBps
	}
	return 20
}

// EffectiveTPMaxR returns the take-profit cap in R, or 0 for "no cap"
// (unset, negative, or the invalid (0, 1) range).
func (r RiskControlConfig) EffectiveTPMaxR() float64 {
	if r.TPMaxR < 1 {
		return 0
	}
	return r.TPMaxR
}

func (r RiskControlConfig) EffectiveRiskPerTradePct() float64 {
	if r.RiskPerTradePct > 0 {
		return r.RiskPerTradePct
	}
	return 1.5
}
