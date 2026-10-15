// Package stockengine is the decision layer of the us_stock strategy: it turns
// market/usstock series into per-symbol signal snapshots, builds the LLM
// prompts, parses and validates long-only decisions, and sizes orders. It is
// pure (no network, no exchange): the run cycle in package trader fetches the
// data, calls the model through mcp.AIClient and executes the result.
// Design: docs/architecture/US_STOCK_BSTOCK_DESIGN_2026-10-09.md §4, §6.
//
// Exported API (the contract the run cycle builds on):
//
//	ResolvePreset(cfg *store.StockConfig) Preset
//	BuildSnapshot(symbol, underlying string, series map[string]*usstock.Series, quote *usstock.Quote, now time.Time) *SymbolSnapshot
//	BuildSystemPrompt(cfg *store.StockConfig, preset Preset, lang string) string
//	BuildUserPrompt(ctx *Context, lang string) string
//	ParseDecisions(response string) ([]Decision, error)
//	Validate(ctx *Context, decisions []Decision) []Verdict
//	SizeOrder(ctx *Context, d Decision) (Sizing, error)
//	Decide(ctx *Context, client mcp.AIClient, lang string) (*Result, error)   // prompts → model → parse → validate
package stockengine

import (
	"time"

	"nofx/market/usstock"
	"nofx/store"
)

// Actions a decision may carry. Long-only: there is no short.
const (
	ActionOpenLong   = "open_long"   // new position
	ActionAddLong    = "add_long"    // scale into an existing position
	ActionReduceLong = "reduce_long" // sell ReduceFraction of the position
	ActionCloseLong  = "close_long"  // sell everything
	ActionAdjustStop = "adjust_stop" // move the protective stop (and/or take-profit)
	ActionHold       = "hold"        // keep a position as is
	ActionWait       = "wait"        // no position, no entry
)

// Entry order types for open_long / add_long.
const (
	EntryMarket = "market"
	EntryLimit  = "limit"
)

// Preset is the resolved trading style (swing / position) with its timing
// and stop band; values come from store.StockConfig with preset defaults.
type Preset struct {
	Name          string   // store.StockPresetSwing | store.StockPresetPosition
	TrendTF       string   // usstock.TF1d (swing) | usstock.TF1w (position)
	EntryTF       string   // usstock.TF1h (swing) | usstock.TF1d (position)
	DecisionTimes []string // "HH:MM" America/New_York on trading days: swing 09:45,12:30,15:30; position 15:30
	StopATRMin    float64  // ATR(1d) multiples
	StopATRMax    float64
	MaxHoldDays   int // review horizon in trading days: swing 30, position 0 (none)
}

// SymbolSnapshot is the program-computed view of one symbol the model reads.
// All prices are bStock prices except where named Ref*.
type SymbolSnapshot struct {
	Symbol     string            `json:"symbol"`     // AAPLBUSDT
	Underlying string            `json:"underlying"` // AAPL
	Price      float64           `json:"price"`      // bStock last
	Sources    map[string]string `json:"sources"`    // timeframe → usstock.SourceBStock | SourceYahoo
	// Data sufficiency: timeframes whose series had fewer bars than needed.
	MissingTF []string `json:"missing_tf,omitempty"`

	// Daily structure (1d series).
	EMA20, EMA50, EMA200 float64 `json:"-"`
	TrendDaily           string  `json:"trend_daily"`  // up | down | range (EMA alignment + slope)
	TrendWeekly          string  `json:"trend_weekly"` // up | down | range | unknown (1w series)
	ATR1d                float64 `json:"atr_1d"`       // ATR(14) on 1d, price units
	ATR1dPct             float64 `json:"atr_1d_pct"`
	High52w, Low52w      float64 `json:"-"`
	PctFrom52wHigh       float64 `json:"pct_from_52w_high"` // negative = below the high
	VolumeRatio20_50     float64 `json:"volume_ratio"`      // avg vol 20d / 50d
	Return20dPct         float64 `json:"return_20d_pct"`
	SwingLow, SwingHigh  float64 `json:"-"` // most recent confirmed daily pivots
	// Entry timeframe (1h for swing, 1d for position).
	TrendEntry string `json:"trend_entry"` // up | down | range

	Quote *usstock.Quote `json:"quote,omitempty"`

	// Cycle is the Kell price-cycle assessment on daily bars (nil when no
	// daily series was available).
	Cycle *CycleAssessment `json:"cycle,omitempty"`
	// Momentum is the trend-filtered 12-1 momentum read (nil when unknown).
	Momentum *MomentumAssessment `json:"momentum,omitempty"`

	// RSI is Wilder RSI(14) on closes per timeframe ("4h", "1d", "1w"); a
	// timeframe is present only when its series had ≥ 15 closed bars.
	RSI map[string]float64 `json:"rsi,omitempty"`
	// Candles holds the last closed bars per timeframe for the prompt (1w 26,
	// 1d 30, 4h 24), oldest first. Copies, never aliases of the series.
	Candles map[string][]usstock.Bar `json:"-"`
}

// UniverseRow is one compact row of the universe table in the user prompt:
// the cross-sectional view of one listed bStock pair, computed from its daily
// bars (BuildUniverseRow) and enriched with best-effort fundamentals.
type UniverseRow struct {
	Symbol, Underlying string
	Class              string // "ETF" | "stock" | "" unknown
	Sector             string // "" unknown
	Price              float64
	Ret1dPct           float64
	Ret10dPct          float64
	Ret30dPct          float64
	Vol30Pct           float64 // annualized stdev of the last 30 daily log returns, ×sqrt(252)×100
	RSI14              float64 // daily
	DistFrom30dHighPct float64 // close / max(high of last 30 bars) − 1, ×100 (≤ 0)
	AvgDollarVol20     float64 // mean(close×volume) over the last 20 bars
	DaysToEarnings     int     // −1 unknown
}

// Position is a held bStock position as the engine sees it.
type Position struct {
	Symbol      string    `json:"symbol"`
	Quantity    float64   `json:"quantity"`
	AvgPrice    float64   `json:"avg_price"`
	Price       float64   `json:"price"`
	StopPrice   float64   `json:"stop_price"` // live protective stop (0 = none)
	TakeProfit  float64   `json:"take_profit,omitempty"`
	InitialStop float64   `json:"initial_stop,omitempty"` // opening-risk stop, R anchor
	OpenedAt    time.Time `json:"opened_at"`
}

// Account is the spot account summary in USDT.
type Account struct {
	Equity    float64 `json:"equity"`    // USDT + Σ bStock value
	Available float64 `json:"available"` // free USDT
	Exposure  float64 `json:"exposure"`  // Σ bStock value
}

// Context is everything one decision cycle needs.
type Context struct {
	Now       time.Time
	Session   usstock.Session
	Config    *store.StockConfig
	Preset    Preset
	Account   Account
	Positions []Position
	Snapshots []*SymbolSnapshot // configured symbols, plus market context symbols
	Market    []*SymbolSnapshot // SPYBUSDT / QQQBUSDT context (may overlap Snapshots)
	Paper     bool
	// Universe is the FRESH scan report's results (one per listed bStock pair)
	// rendered as the prompt's universe table; empty = no table. It carries
	// ScanResult rather than UniverseRow because the table also shows each
	// pair's momentum and Kell phase.
	Universe []ScanResult `json:"-"`
}

// Decision is one model decision (prices in USDT, bStock pair units).
type Decision struct {
	Symbol         string  `json:"symbol"`
	Action         string  `json:"action"`
	EntryType      string  `json:"entry_type,omitempty"`      // market | limit (open/add)
	LimitPrice     float64 `json:"limit_price,omitempty"`     // for entry_type limit
	StopLoss       float64 `json:"stop_loss,omitempty"`       // required for open/add; new stop for adjust_stop
	TakeProfit     float64 `json:"take_profit,omitempty"`     // optional
	ReduceFraction float64 `json:"reduce_fraction,omitempty"` // (0,1) for reduce_long
	Confidence     int     `json:"confidence,omitempty"`      // 0–100
	Reasoning      string  `json:"reasoning,omitempty"`
}

// Verdict is the program's validation of one decision. Accepted=false means
// it must not be executed; Codes are machine reason codes.
type Verdict struct {
	Decision Decision `json:"decision"`
	Accepted bool     `json:"accepted"`
	Codes    []string `json:"codes,omitempty"` // e.g. SESSION_CLOSED, BSTOCK_DIVERGENCE, DATA_INSUFFICIENT, STOP_OUT_OF_BAND, MAX_POSITIONS, ...
	Note     string   `json:"note,omitempty"`
}

// Sizing is the program-computed order size for an accepted open/add.
type Sizing struct {
	Quantity  float64 `json:"quantity"`
	Notional  float64 `json:"notional"`   // USDT
	RiskUSDT  float64 `json:"risk_usdt"`  // loss at the stop
	LimitedBy string  `json:"limited_by"` // risk | position_cap | exposure_cap | available | min_notional
}

// Result is one full decision cycle's output.
type Result struct {
	SystemPrompt string
	UserPrompt   string
	RawResponse  string
	Decisions    []Decision
	Verdicts     []Verdict
	DurationMs   int64
}
