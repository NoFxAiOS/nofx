// Signal layer: converts raw per-timeframe market data into compact,
// normalized feature blocks for the AI prompt. The program owns everything
// mechanical — candle settlement, time alignment (UTC), missing-value
// handling, indicator math, outlier filtering — so the model reasons over
// clean features instead of raw candle dumps.
package kernel

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/market"
	"nofx/market/breakout"
	"nofx/provider/openbb"
)

// TFSignal is the normalized feature block for one timeframe. Pointer fields
// are omitted from the prompt JSON when the underlying indicator is disabled
// or data is insufficient.
type TFSignal struct {
	Timeframe string `json:"timeframe"` // e.g. "5m", "1h"
	Trend     string `json:"trend"`     // up / down / pullback / range
	// Trend-window return: the change over the last `lookback` closed bars —
	// a WINDOW, not the bar timeframe (1h bars × 20 = 20h window). The window
	// span itself is in return_window_hours.
	ReturnPct         float64  `json:"trend_window_return_pct"`
	ReturnWindowHrs   float64  `json:"return_window_hours"`                  // real-world span of the trend window (lookback × bar duration)
	PrevHourChangePct *float64 `json:"prev_hour_close_change_pct,omitempty"` // last FULLY CLOSED hour vs the one before (closed data only)
	ATRPct            float64  `json:"atr_pct"`                              // ATR14 / price × 100
	ATRPercentile     *float64 `json:"atr_percentile,omitempty"`             // current ATR% rank within this TF's own ATR history (0-100)
	VolumeRatio       *float64 `json:"volume_ratio"`                         // last closed volume / 20-bar avg
	Volume            float64  `json:"volume"`                               // last CLOSED bar volume, base units (kline source native)
	VolumeChangePct   float64  `json:"volume_change_pct"`                    // last closed volume vs previous closed bar, %
	// Support/Resistance are ALWAYS emitted (possibly []) — an empty array is
	// meaningful ("no swing level remains on this side of the LIVE price;
	// price is making new highs/lows inside the window") and must stay
	// legible, not vanish from the JSON (09-18 audit #6: BTC 1h/4h showed no
	// resistance key at all while price was above every 4h structure high).
	Support    []float64 `json:"support"`    // nearest swing lows, ascending; [] = none below live price
	Resistance []float64 `json:"resistance"` // nearest swing highs, descending; [] = none above live price
	// Full swing-pivot sets used by the hard-gate RR/stop calculations. The
	// prompt arrays above are intentionally capped and classified against the
	// live price; a limit entry can sit on the other side of that price, so the
	// gate must retain uncapped pivots and re-filter them against its entry.
	StructuralSupport    []float64 `json:"-"`
	StructuralResistance []float64 `json:"-"`
	// BOLLSourced tags which Support/Resistance values came from the Bollinger
	// band rather than a swing pivot. NOT serialized (prompt JSON unchanged):
	// scanRR uses it to keep decaying band values out of first_rr_ge_target
	// (QUANT_REVIEW_2026-09-22 C1).
	BOLLSourced       map[float64]bool `json:"-"`
	SupportDistPct    []float64        `json:"support_dist_pct"`      // pre-computed (level-anchor)/anchor×100, index-aligned with Support
	ResistanceDistPct []float64        `json:"resistance_dist_pct"`   // pre-computed, index-aligned with Resistance
	MACDHist          *float64         `json:"macd_hist,omitempty"`   // normalized by price
	MACDTrend         string           `json:"macd_trend,omitempty"`  // rising / falling / flat
	RSI14             *float64         `json:"rsi,omitempty"`         // 0-100
	StochRSIK         *float64         `json:"stoch_rsi_k,omitempty"` // StochRSI %K (14,14,3,3), 0-100
	StochRSID         *float64         `json:"stoch_rsi_d,omitempty"` // StochRSI %D (3-SMA of K), 0-100
	EMAFast           *float64         `json:"ema_fast,omitempty"`    // EMA20
	EMASlow           *float64         `json:"ema_slow,omitempty"`    // EMA50
	LastClosedCandle  string           `json:"last_closed_candle"`    // bullish / bearish / doji
	LastClose         float64          `json:"last_close"`            // last CLOSED candle close
	StructureHigh     *float64         `json:"structure_high"`        // highest close in window
	StructureLow      *float64         `json:"structure_low"`         // lowest close in window
	// Structure distances use the SAME convention as support_dist_pct /
	// resistance_dist_pct: (level − LIVE price)/live×100. NEGATIVE
	// structure_high_dist_pct means the live price has already cleared the
	// window high (price is printing new highs — the closed-candle structure
	// lags and offers no overhead reference). 09-18 audit #6: BTC 4h showed
	// structure_high 1.6% BELOW the live price with no marker at all.
	StructureHighDistPct *float64 `json:"structure_high_dist_pct,omitempty"`
	StructureLowDistPct  *float64 `json:"structure_low_dist_pct,omitempty"`
	BarsUsed             int      `json:"closed_bars_used"`
	UnclosedDropped      bool     `json:"unclosed_dropped"` // forming candle was dropped
}

// DerivSignal carries derivatives context.
type DerivSignal struct {
	FundingRate           *float64 `json:"funding_rate,omitempty"`
	FundingAnnualizedPct  *float64 `json:"funding_annualized_pct,omitempty"`    // 预计算: rate × (24/settle_hours) × 365 × 100,按实测结算间隔年化,模型直接使用不再自行年化
	FundingSettleHours    *float64 `json:"funding_settle_hours,omitempty"`      // 实测结算间隔(小时);缺失时年化按 8h 口径
	OIChange1hPct         *float64 `json:"oi_change_1h_pct,omitempty"`          // true 1h change (quant layer)
	OIVsAvgPct            *float64 `json:"oi_vs_avg_pct,omitempty"`             // current OI vs its period average
	PriceChange1hLivePct  *float64 `json:"price_change_60m_live_pct,omitempty"` // LIVE price vs ~60 minutes ago (self-computed from the finest ≤1h klines). NOT a candle-close figure.
	PriceChange24hLivePct *float64 `json:"price_change_24h_live_pct,omitempty"` // LIVE price vs 24 hours ago (rolling, exchange ticker basis), PERCENT — converted from the quant layer's decimal at ingest
	OICurrentBase         *float64 `json:"oi_current_base,omitempty"`           // current open interest, base-asset units
	// Crowd-positioning metrics from Binance futures data endpoints:
	LongShortAccountRatio  *float64 `json:"long_short_account_ratio,omitempty"`  // global accounts long/short
	TopTraderPositionRatio *float64 `json:"top_trader_position_ratio,omitempty"` // top-20%-by-position traders long/short
	TakerBuySellRatio      *float64 `json:"taker_buy_sell_ratio,omitempty"`      // taker buy/sell volume, 1h
	// Liquidation feed (user directive 2026-09-25): real Binance force-order
	// stream (!forceOrder@arr), 24h rolling per symbol. Absent during cold
	// start = the window has no events yet, never "zero liquidations".
	Liquidation *market.LiquidationWindow `json:"liquidation,omitempty"`
	// News headlines via the OpenBB sidecar (optional enrichment). Absent
	// when the sidecar is down or no news provider key is configured.
	News []openbb.NewsItem `json:"news,omitempty"`
	// FundingRollover is the PROGRAM-verified short-side crowding condition ②
	// ("费率刚从高位回落"), computed from the settled funding history vs the
	// live forward rate (user review 2026-09-15 point 6: the rule demanded
	// this evidence but no structured-signal field carried it, so the model
	// could only trust — or distrust — the scanner's snapshot pattern text).
	// Absent = history unavailable → the condition is UNKNOWN, never "not
	// rolled over".
	FundingRollover *FundingRolloverState `json:"funding_rollover,omitempty"`
	// LongSqueeze is the MIRROR of the short-side crowding condition: a
	// negative funding rate (shorts paying), retail-heavy long/short ratio
	// and institution net inflow together form the standard squeeze setup —
	// one independent confirmation for longs (09-19 audit 七, ZEC case:
	// funding −10.47%, ratio 0.43, inflow +3.1M).
	LongSqueeze *LongSqueezeState `json:"long_squeeze,omitempty"`
}

// LongSqueezeState: program-detected squeeze setup — annualized funding
// below −5% (shorts paying), long/short account ratio below 1 (retail net
// short), and positive institution net inflow.
type LongSqueezeState struct {
	Detected              bool     `json:"detected"`
	FundingAnnualizedPct  *float64 `json:"funding_annualized_pct,omitempty"`
	LongShortAccountRatio *float64 `json:"long_short_account_ratio,omitempty"`
	NetInflowUSDT         *float64 `json:"net_inflow_usdt,omitempty"`
}

// FundingRolloverState: detected via market.FundingRolloverDetected — the
// settled rate 3 intervals ago sat above the symbol-scaled crowding threshold
// and the live forward rate is now below it. from/to are annualized on the
// symbol's real settlement interval.
type FundingRolloverState struct {
	Detected          bool     `json:"detected"`
	FromAnnualizedPct *float64 `json:"from_annualized_pct,omitempty"` // the "high" it rolled off, annualized
	ToAnnualizedPct   *float64 `json:"to_annualized_pct,omitempty"`   // the live forward rate, annualized
	SettlesBack       int      `json:"settles_back"`                  // reference point: N settlements ago
}

// LiquiditySignal carries order-book tradability metrics.
type LiquiditySignal struct {
	SpreadBps         *float64 `json:"spread_bps,omitempty"`
	SpreadPct         *float64 `json:"spread_pct,omitempty"` // LIVE book spread, % of mid (09-19 audit 五-7: the enforced gate was invisible)
	Depth05PctUSD     *float64 `json:"depth_0_5pct_usd,omitempty"`
	QuoteVolume24hUsd *float64 `json:"quote_volume_24h_usd,omitempty"` // absolute 24h turnover, for size/slip judgment
}

// SymbolSignal is the complete per-symbol block injected into the prompt.
type SymbolSignal struct {
	rrCostBps    float64
	Symbol       string  `json:"symbol"`
	TimestampUTC string  `json:"timestamp_utc"`
	Price        float64 `json:"price"`
	// PriceChange4hPct: market.Data.PriceChange4h (live price vs the previous
	// 4h bar close, PERCENT), carried for the CHASE_MOVE4H hard gate only —
	// not serialized into the model-facing JSON. 0 = unavailable (fail-open).
	PriceChange4hPct float64 `json:"-"`
	LimitBuyPrice    float64 `json:"limit_buy_price"`  // live price ×(1-offset) — 预计算的做多限价挂单价
	LimitSellPrice   float64 `json:"limit_sell_price"` // live price ×(1+offset) — 预计算的做空限价挂单价
	// LimitEntryOffsetPct is the offset actually applied to the anchors above
	// (ATR-scaled per AnchorOffsetConfig; the model copies the prices, this
	// field documents the distance it is being asked to wait for).
	LimitEntryOffsetPct float64 `json:"limit_entry_offset_pct"`
	PrimaryTF           string  `json:"primary_tf"`
	// Explicit timeframe roles (⑨): one field, three jobs — which TF drives
	// execution timing, which carries the tradeable trend, which sets regime.
	RoleTFs    RoleTimeframes    `json:"role_tfs"`
	RegimeLine *RegimeLineSignal `json:"regime_line,omitempty"`
	// MarketRegime is the deterministic trend x volatility classification
	// derived from CLOSED 1h/4h bars. It is execution evidence, not an LLM
	// sentiment label: market-order chase exceptions fail closed unless the
	// confirmed regime matches their direction.
	MarketRegime   *MarketRegime        `json:"market_regime,omitempty"`
	Timeframes     map[string]*TFSignal `json:"timeframes"`
	Derivatives    *DerivSignal         `json:"derivatives,omitempty"`
	Liquidity      *LiquiditySignal     `json:"liquidity,omitempty"`
	EntryTriggered bool                 `json:"entry_rule_triggered"`
	ExitTriggered  bool                 `json:"exit_rule_triggered"`
	// ⑧ Precomputed execution filter — mirrors the trader's entry-timing
	// gate so the model never has to derive "can I even open long here".
	ExecutionFilter *ExecutionFilter `json:"execution_filter,omitempty"`
	// ⑫ Precomputed breakout state vs the 1h structure levels.
	Breakout *BreakoutState `json:"breakout,omitempty"`
	// ⑬ Long pullback plan (user design 2026-10-01): when a LONG breakout is
	// confirmed/retesting, the limit anchor IS the broken level and this block
	// carries the entry geometry plus the symmetric evidence badges (funding
	// not overheated / volume+OI confirmation / EMA20 distance). nil = the
	// breakout-pullback path is not active for this symbol.
	LongPullback *LongPullbackPlan `json:"long_pullback,omitempty"`
	// BBRide is the pre-computed 15m upper-band ride verdict (bb_ride):
	// volume surge + ≥3 consecutive bullish closed 15m bars hugging the upper
	// Bollinger band — the momentum-ride market-entry evidence.
	BBRide *BBRide `json:"bb_ride,omitempty"`
	// ShortRide is the lower-band plunge-ride mirror (short_ride, 09-16):
	// volume surge + ≥3 consecutive bearish closed 15m bars hugging the lower
	// Bollinger band — the market-open_short evidence for plunges where the
	// short limit anchor sits suppressed against the swing-low.
	ShortRide *BBShortRide `json:"short_ride,omitempty"`
	// LiquiditySweep is opt-in reference evidence; it never affects hard gates.
	LiquiditySweep *LiquiditySweep `json:"liquidity_sweep,omitempty"`
	// FactorEnsemble is opt-in reference evidence; no gate or score reads it.
	FactorEnsemble *FactorEnsembleRef `json:"factor_ensemble,omitempty"`
	// ⑩ Data completeness ≠ data sufficiency: per-TF closed-bar counts vs
	// indicator requirements.
	DataQuality *DataQuality `json:"data_quality,omitempty"`
	// DataComplete means the core OHLC/kline data was sufficient this cycle.
	// It does NOT cover derivatives coverage — missing funding/long-short
	// data (e.g. tokenized-stock perps) is flagged in warnings instead.
	DataComplete bool     `json:"data_complete"`
	Warnings     []string `json:"warnings,omitempty"`

	BTC           *BTCCorrelation    `json:"btc_correlation,omitempty"`
	DataFreshness *DataFreshness     `json:"data_freshness,omitempty"`
	TraderHistory *TraderHistoryStat `json:"trader_history,omitempty"`
	// LossStreak is the program's circuit-breaker verdict for this symbol.
	// Present ONLY when the symbol is currently banned; the model may cite
	// LOSS_STREAK_BAN for this symbol and no other.
	LossStreak *LossStreakState `json:"loss_streak,omitempty"`
	// MinSize is the precomputed minimum-notional feasibility for this	// symbol at the current equity (see MinSizeCheck).
	MinSize *MinSizeCheck `json:"min_size,omitempty"`

	// HardGate is the program's per-direction open verdict: entry basis,
	// anchor/limit availability, the exhaustive rr_scan, and the machine
	// blocker list (see HardEntryGate / DirectionGate). The model must open
	// only in a direction this says allowed (or argue one of the two
	// documented market-order exceptions), and adopt rr_scan.first_rr_ge_target
	// as the take-profit instead of re-scanning structure.
	HardGate *HardEntryGate `json:"hard_entry_gate,omitempty"`
	// PumpGuard is the extended-pump long guard's evidence + verdict —
	// rendered so the model sees WHY a hot coin's long side is locked
	// (nil when the guard is disabled via a negative pump_guard_4h_pct).
	PumpGuard *PumpGuardBlock `json:"pump_guard,omitempty"`
	// Bias names the SOURCE of each directional read (see BiasBlock).
	Bias *BiasBlock `json:"bias,omitempty"`

	SignalConflict *SignalConflict `json:"signal_conflict,omitempty"`
	// OHLCV raw candles per configured timeframe (user directive 2026-09-25):
	// the strategy UI's 市场数据 panel (enable_raw_klines) promises raw bars
	// alongside the derived metrics. CLOSED bars only, oldest→newest, prompt
	// keeps the most recent ResolvePromptKlineBars per timeframe (default 20
	// — 09-27 review: the full 60-bar dump was ~63% of the prompt and the
	// derived fields already encode what the model trades on). Full history
	// stays re-fetchable from the exchange for audit/backtest.
	// [open, high, low, close, volume]
	OHLCV map[string][][5]float64 `json:"ohlcv,omitempty"`
	// OHLCVLastClosed maps timeframe → close time (UTC) of the newest bar in
	// OHLCV[tf] — the freshness stamp the trimmed series needs (09-27
	// review: with 20 shipped bars the model must know WHEN the newest
	// closed bar is without counting back from Time).
	OHLCVLastClosed map[string]string `json:"ohlcv_last_closed_utc,omitempty"`

	// TP-menu touch-stat source (user directive 2026-09-29, NOT serialized):
	// closed 1h bar extremes for crypto / 1d for equity tokens, capped at
	// ~30 days. scanRRForSymbol counts per-option reaches against these.
	touchHighs, touchLows []float64
	touchWindowDays       int
}

// RoleTimeframes names the job of each timeframe (⑨): execution TF times the
// entry, trend TF carries the directional call, regime TF sets the context.
type RoleTimeframes struct {
	ExecutionTF string `json:"execution_tf"` // entry timing (usually 15m)
	TrendTF     string `json:"trend_tf"`     // directional call (1h)
	RegimeTF    string `json:"regime_tf"`    // market regime context (4h)
}

// ExecutionFilter precomputes whether each direction may even be opened
// under the micro-trend alignment rule (⑧): longs need up/pullback, shorts
// need down, range blocks both. When the strategy enables the entry-timing
// gate these are HARD rules (the trader rejects blocked opens); otherwise
// they are advisory.
type ExecutionFilter struct {
	MicroTF      string `json:"micro_tf"`
	MicroTrend   string `json:"micro_trend"`
	LongAllowed  bool   `json:"long_allowed"`
	ShortAllowed bool   `json:"short_allowed"`
	Reason       string `json:"reason,omitempty"`
}

// Breakout-state geometry: the fixed pre-push level is the extreme of the
// boLevelWindow closed bars BEFORE the recent boCrossWindow-bar cross window
// (see computeBreakoutState — review 2026-10-04 C1).
const (
	boCrossWindow = 8
	boLevelWindow = 30
)

// BreakoutState is the precomputed breakout verdict vs the 1h structure
// level in the direction the 1h trend implies (⑫): the model should read one
// field instead of assembling price/levels/volume/OI itself.
type BreakoutState struct {
	Status        string  `json:"status"`              // below | approach | broken_unconfirmed | confirmed | fake_break | retest_hold | extended
	Direction     string  `json:"direction"`           // breakout (long) | breakdown (short)
	Level         float64 `json:"level"`               // the structure level being tested (fixed pre-push extreme)
	DistancePct   float64 `json:"distance_pct"`        // price vs level, % (positive = beyond)
	VolumeConfirm bool    `json:"volume_confirmation"` // breakout bars carry ≥1.5× volume
	OIConfirm     bool    `json:"oi_confirmation"`     // OI expanded in the breakout direction (1h, beyond noise)
}

// DataQuality separates "the fetch worked" from "there is enough history for
// the indicators to mean anything" (⑩): a 3-day-old listing has complete but
// insufficient 4h bars.

// LongPullbackPlan is the breakout-retest entry geometry for LONGS (user
// design 2026-10-01, "保留突破,改入场方式"): the limit anchor IS the broken
// resistance (old resistance → support), the stop lives below it (structural;
// "closing back under the level" is the failure signal), and the evidence
// badges mirror the short-side discipline (funding not overheated, volume+OI
// confirmation, EMA20 distance). Everything is neutral evidence + geometry —
// the trade decision stays with the model under the same hard gates.
type LongPullbackPlan struct {
	Active         bool     `json:"active"`                           // the limit anchor IS the retest level
	Entry          float64  `json:"entry"`                            // the broken level (== limit_buy_price when active)
	Level          float64  `json:"level"`                            // the breakout structure level
	Status         string   `json:"breakout_status"`                  // confirmed | retest_hold
	ChaseDistPct   float64  `json:"chase_dist_pct"`                   // live price above the level, % — the stretch the retest must close
	FundingOK      *bool    `json:"funding_not_overheated,omitempty"` // annualized ≤ the market-exception cap; nil = unknown
	VolumeConfirm  bool     `json:"volume_confirmation"`
	OIConfirm      bool     `json:"oi_confirmation"`
	EMA20Dist4hPct *float64 `json:"ema20_4h_dist_pct,omitempty"` // live price vs 4h EMA20, % — the stretch gate reads this
}

// DataQuality separates "the fetch worked" from "there is enough history for
// the indicators to mean anything" (⑩): a 3-day-old listing has complete but
// insufficient 4h bars.
type DataQuality struct {
	Complete      bool           `json:"complete"`
	Sufficient    bool           `json:"sufficient"`
	AvailableBars map[string]int `json:"available_bars"`
	MinimumBars   map[string]int `json:"minimum_bars"`
	Shortfall     []string       `json:"shortfall,omitempty"`
}

// SignalConflict tallies directional evidence (timeframe trends + live 60m
// change + scanner bias) and flags when the scanner's bias contradicts market
// structure — the model must resolve it explicitly before trading.
// DirectionalScore is the net consensus on a -100..+100 scale (⑥): far more
// readable for the model than raw counts. Types classifies WHAT conflicts
// (⑦): scanner-vs-structure is meaningful, a 15m/5m range split is noise.
type SignalConflict struct {
	DirectionalConflict bool     `json:"directional_conflict"`
	BullishEvidence     int      `json:"bullish_evidence"`
	BearishEvidence     int      `json:"bearish_evidence"`
	DirectionalScore    int      `json:"directional_score"` // -100..+100 net consensus
	Types               []string `json:"types,omitempty"`   // SCANNER_VS_STRUCTURE | TIMEFRAME_SPLIT
	Note                string   `json:"note,omitempty"`
}

// BTCCorrelation quantifies co-movement with BTC on 1h returns (concentration
// risk: a basket of high-beta/high-corr alts is one big BTC position).
type BTCCorrelation struct {
	Pearson float64 `json:"pearson"` // -1..1, hourly returns vs BTC
	Beta    float64 `json:"beta"`    // cov(sym,btc)/var(btc)
	Samples int     `json:"samples"` // return observations used
	Window  string  `json:"window"`  // e.g. "1h"
}

// DataFreshness lets the model judge how trustworthy the block is.
type DataFreshness struct {
	SettlementAgeSeconds float64  `json:"settlement_age_seconds"`                  // now - last closed candle close (primary TF)
	VendorDivergencePct  *float64 `json:"vendor_vs_live_divergence_pct,omitempty"` // vendor forming close vs live ticker before the live-price patch
}

// TraderHistoryStat is this trader's own closed-trade record on one symbol.
type TraderHistoryStat struct {
	ClosedTrades int     `json:"closed_trades"`
	Wins         int     `json:"wins"`
	WinRatePct   float64 `json:"win_rate_pct"`
	RealizedPnL  float64 `json:"realized_pnl"`
}

// LossStreakState is the PROGRAM-computed loss-streak circuit-breaker verdict
// for one symbol (auto_trader_lossstreak.go is the enforcement mirror). The
// model must read this field instead of self-judging the ban — an audit on
// 09-15 found the model tagging just-won symbols (e.g. CAPUSDT ×17) with
// LOSS_STREAK_BAN, poisoning the entry_quality dataset.
type LossStreakState struct {
	Banned    bool    `json:"banned"`
	UntilUTC  string  `json:"until_utc,omitempty"`  // ban expiry (program-computed)
	HoursLeft float64 `json:"hours_left,omitempty"` // remaining ban duration
}

// MinSizeCheck precomputes whether any ALLOWED stop on this symbol can meet
// the strategy's minimum position size at the current equity (risk_control.
// min_position_size from the web strategy page — the same value
// enforceMinPositionSize rejects opens under). Small accounts have a
// structural dead zone: notional = equity×risk% ÷ stop%, so a wide stop floor
// (high-ATR coins) yields a notional below the minimum — every order would be
// rejected. Surfacing this per-symbol stops the model from burning decision
// budget on settings that can never execute (09-15 CAPUSDT case: 10.4-11.4%
// stop floor → 7-7.6U notional < 10U minimum, re-derived every cycle).
type MinSizeCheck struct {
	Feasible           bool    `json:"feasible"`                  // false = even the tightest allowed stop breaks the minimum
	MinPositionSizeUsd float64 `json:"min_position_size_usd"`     // risk_control.min_position_size in effect
	MaxStopPct         float64 `json:"max_stop_pct_for_min_size"` // stop distance beyond this → notional < minimum
	StopFloorPct       float64 `json:"stop_floor_pct,omitempty"`  // SLMinATRMult×ATR(1h); 0 = no noise floor configured
	Reason             string  `json:"reason,omitempty"`          // set when !Feasible
}

// RRScan is the program's exhaustive structural take-profit scan for one
// direction (user review 2026-09-15 points 1/2/8): walk EVERY resistance
// (long) / support (short) element across ALL timeframe blocks, near→far,
// deduped, and compute the RR of each at the TIGHTEST ALLOWED stop (the noise
// floor). The floor stop is deliberately the best case — a wide structural
// stop only lowers RR — so best_rr < min_rr is a DEFINITIVE verdict that the
// RR gate cannot be passed in this direction, whatever structure the model
// picks. first_rr_ge_target is the level the TP rule demands: the nearest
// target whose RR clears min_rr — the model adopts it instead of re-deriving.
type RRScan struct {
	CostBps         float64   `json:"round_trip_cost_bps,omitempty"`
	Direction       string    `json:"direction"`                    // long | short
	EntryPrice      float64   `json:"entry_price"`                  // the anchor the scan assumed
	EntryBasis      string    `json:"entry_basis"`                  // limit_anchor | live_price
	StopDistancePct float64   `json:"stop_distance_pct"`            // METHODOLOGY stop (stop_plan) distance in % — the same stop the gate verdict is computed at
	StopPrice       float64   `json:"stop_price"`                   // the methodology stop price — adopt as stop_loss verbatim
	MinRR           float64   `json:"min_rr"`                       // strategy min_risk_reward_ratio in effect
	TargetsScanned  int       `json:"targets_scanned"`              // distinct structural levels scanned
	Targets         []float64 `json:"targets,omitempty"`            // the deduped scanned levels themselves — self-verification (09-19 audit 五-6)
	BestTarget      float64   `json:"best_target,omitempty"`        // farthest scanned level (max RR)
	BestRR          float64   `json:"best_rr"`                      // RR upper bound AT THE METHODOLOGY STOP — < min_rr ⇒ RR fails for sure
	FirstRRGeTarget float64   `json:"first_rr_ge_target,omitempty"` // nearest level with RR ≥ min_rr — the TP to use
	// FirstTargetRR is the RR AT FirstRRGeTarget (the rule-adopted TP) — the
	// NEGATIVE_EDGE health gate's "first_target_rr ≥ threshold" reads this,
	// so a 1.6R-min-qualifying target stays visible as < 2.0 (BRUSDT case).
	FirstTargetRR float64 `json:"first_target_rr,omitempty"`
	// FirstTargetBeyondStructure: the adopted TP sits beyond EVERY timeframe's
	// structure_high/low — no historical resistance/support reference exists
	// there (the prompt requires the model to note the uncertainty).
	FirstTargetBeyondStructure bool `json:"first_target_beyond_structure,omitempty"`
	Usable                     bool `json:"usable"` // a qualifying target exists
	// FirstObstacle / FirstObstacleRR (2026-10-07 review S2): the NEAREST
	// structural (non-BOLL) level in the trade's path and the net RR at it —
	// whether or not it reaches min_rr. first_rr_ge_target skips every level
	// below min_rr, so a qualifying far TP can sit behind a wall the price
	// meets first (live: planned TP 17.9% away on average, 2/109 reached,
	// mean MFE 2.86%). Evidence only — no gate reads it.
	FirstObstacle   float64 `json:"first_obstacle,omitempty"`
	FirstObstacleRR float64 `json:"first_obstacle_rr,omitempty"`
	// TPOptions is the menu the model picks from (user directive 2026-09-29):
	// up to three QUALIFYING (RR ≥ min_rr, non-BOLL) structural targets —
	// near/mid/far — so the regime call ("trend keeps running" vs "range
	// fades at the edge") is the model's, while every price stays 100%
	// program-computed. Option 1 IS first_rr_ge_target (the legacy verbatim
	// TP); farther options only RAISE the RR. Empty when unusable.
	Options []TPOption `json:"tp_options,omitempty"`
}

// TPOption is one precomputed take-profit plan. The model answers with a
// 1-based index (Decision.TPOption); it never authors the price.
type TPOption struct {
	Level           float64 `json:"level"`                 // the structural target price
	RR              float64 `json:"rr"`                    // RR at the gated stop_plan
	TouchCount      int     `json:"touch_count,omitempty"` // closed 1h (bstock 1d) bars in the window whose range reached the level — neutral "price has been here" evidence, NOT a probability
	TouchWindowDays int     `json:"touch_window_days,omitempty"`
	BeyondStructure bool    `json:"beyond_structure,omitempty"` // beyond every timeframe's structure extreme
	Default         bool    `json:"default,omitempty"`          // option 1 (nearest qualifying) — the legacy first_rr_ge_target
}

// DirectionGate is the program's per-direction open verdict for one symbol —
// the "three hard gates" (stop band / RR≥min / timing + anchor + breaker +
// data + size) evaluated ONCE by code so the model sorts, explains and
// chooses instead of re-assembling gates per cycle (review point 10).
type DirectionGate struct {
	Allowed      bool    `json:"allowed"`
	EntryPrice   float64 `json:"entry_price"`
	EntryBasis   string  `json:"entry_basis"`   // limit_anchor | live_price
	LimitAllowed bool    `json:"limit_allowed"` // false = anchor suppressed (limit path dead; the two documented market-order exceptions may still apply)
	// MarketException: the direction-matched market-order exception holds with
	// program evidence (bb_ride/short_ride ride, or confirmed breakout with
	// volume+OI and |directional_score| ≥ MarketExceptionMinScore). Enforced
	// at the open dispatch: without it a market open in limit mode is
	// degraded to the anchor limit or dropped (QUANT_REVIEW_2026-09-22 B1).
	// omitempty keeps the common (false) prompt JSON unchanged.
	MarketException bool    `json:"market_exception,omitempty"`
	StopFloorPct    float64 `json:"stop_floor_pct,omitempty"`
	// StopPlan is the precomputed METHODOLOGY stop (nearest opposite-side
	// structure + direction-aware buffer, clamped into the band). 09-19 RR
	// audit: the gate used to score RR at the noise-floor stop while actual
	// orders stopped at structure+buffer (systematically wider) — trades
	// passed the gate at "1.5R" but executed at 1.3-1.4R. Now the plan, the
	// gate and the executor's checkRR all price the same stop.
	StopPlanPrice float64 `json:"stop_plan_price,omitempty"`
	StopPlanPct   float64 `json:"stop_plan_pct,omitempty"`
	// StopPlanSource: "structure" — the plan is a real structure level ±
	// buffer (step-out, 09-19). A plan is never clamped into no-man's-land.
	StopPlanSource string   `json:"stop_plan_source,omitempty"`
	RR             *RRScan  `json:"rr_scan,omitempty"` // nil when no noise floor is configured (best-case RR undefined)
	Failed         []string `json:"failed"`            // machine codes, ALWAYS present ([] when clean): MICRO_TREND_NOT_LONG/SHORT, EXTENDED_PUMP_UNCONFIRMED, REGIME_LINE_BROKEN, LIMIT_ANCHOR_SUPPRESSED, RR_MAX_x.xx, STOP_PLAN_NO_STRUCTURE, STOP_PLAN_OUT_OF_BAND, BSTOCK_DAILY_DATA_UNAVAILABLE, DATA_INSUFFICIENT, MIN_SIZE_DEAD_ZONE, LOSS_STREAK_BANNED, STOCK_WEEKEND, VENDOR_DIVERGENCE_x.xx/UNKNOWN, POOR_HISTORY, CONSENSUS_OPPOSED_±score, NEG_EDGE_SCORE_±s_LT_t / NEG_EDGE_TREND_MISALIGNED / NEG_EDGE_RR_x.xx_LT_t / NEG_EDGE_LOSING_SYMBOL_x.xxU, WIDE_STOP_x_GT_y, BTC_4H_DOWNTREND, BTC_WEAK_LONG_x_VS_y, EMA20_STRETCH_x_GT_y, CHASE_EMA20_1H_x_GT_y, COUNTER_EMA20_1H_x_GT_y, RSI1H_EXHAUSTED_x, CHASE_MOVE4H_x_GT_y, BTC_4H_STRONGBULL, SHORT_TOP_CONFIRM_MISSING
}

// HardEntryGate holds both direction verdicts.
type HardEntryGate struct {
	Long  *DirectionGate `json:"long"`
	Short *DirectionGate `json:"short"`
}

// PumpGuardBlock is the extended-pump long guard's evidence + verdict
// (user directive 2026-09-20 收紧). Vertical pumps carry an EMA-lagged "up"
// label for days while they round-trip — FILUSDT/SAGAUSDT (2026-09-19, 4h
// +29%/+30%, 1d +38%/+64%) were bought as "trend pullbacks" and stopped out
// −5.2%/−3.6% within hours. When the 4h trend-window return is extended, a
// long needs its pullback CONFIRMED: the 15m close back above the 15m EMA20
// AND a higher 15m swing low — the dip must be demonstrating buyers again,
// not just be cheaper.
type PumpGuardBlock struct {
	Extended      bool    `json:"extended"`       // 4h trend-window return >= threshold
	ThresholdPct  float64 `json:"threshold_pct"`  // resolved guard threshold
	Return4hPct   float64 `json:"return_4h_pct"`  // the 4h trend-window return that was measured
	Confirmed     bool    `json:"confirmed"`      // pullback confirmed; always true when !Extended (guard dormant)
	FastRecovered bool    `json:"fast_recovered"` // 15m last closed close back above the 15m EMA20
	HigherLow     bool    `json:"higher_low"`     // most recent 15m swing low above the prior one
}

// computePumpGuard evaluates the extended-pump long guard. Fail-closed: a
// missing 4h series can never mark a coin extended, but a missing 15m series
// on an extended coin blocks the long (no evidence of confirmation).
func computePumpGuard(sig *SymbolSignal, data *market.Data, thresholdPct float64, now time.Time) *PumpGuardBlock {
	if thresholdPct <= 0 {
		return nil
	}
	pg := &PumpGuardBlock{ThresholdPct: round2(thresholdPct), Confirmed: true}
	if t4h := data.TimeframeData["4h"]; t4h != nil {
		closed := ClosedKlines(t4h, now, tfDuration("4h"))
		// The prompt defines the guard as a five-candle 4h window (20 hours).
		// TFSignal.ReturnPct is a generic 20-bar window, which would stretch
		// this particular guard to 80 hours and keep old pumps armed too long.
		if len(closed) >= 6 {
			from, to := closed[len(closed)-6].Close, closed[len(closed)-1].Close
			if from > 0 {
				ret := (to/from - 1) * 100
				pg.Return4hPct = round2(ret)
				pg.Extended = ret >= thresholdPct
			}
		}
	}
	if !pg.Extended {
		return pg // guard dormant — not an extended pump
	}
	pg.Confirmed = false
	if t15 := sig.Timeframes["15m"]; t15 != nil && t15.EMAFast != nil && t15.LastClose > 0 {
		pg.FastRecovered = t15.LastClose > *t15.EMAFast
	}
	if tfd := data.TimeframeData["15m"]; tfd != nil {
		// Closed-bar settlement is time-based (C2 convention — the comment
		// used to cite the anchor-suppression unconditional drop, which this
		// same batch moved to ClosedKlines). A laggy feed whose newest bar is
		// already closed must not lose a pivot (review 2026-10-06 P2-1).
		bars := ClosedKlines(tfd, now, tfDuration("15m"))
		if len(bars) > 8 {
			_, lows := swingPivots(bars, 2)
			if len(lows) >= 2 {
				pg.HigherLow = lows[len(lows)-1] > lows[len(lows)-2]
			}
		}
	}
	pg.Confirmed = pg.FastRecovered && pg.HigherLow
	return pg
}

// BiasBlock separates the three bias SOURCES so "scanner says short" is never
// phrased as "short evidence is strong" (review point 3, AKE case): scanner =
// the snapshot engine's stance; structure = market evidence (directional
// score sign); execution = which direction the micro-trend gate currently
// allows. The final trade bias stays the model's job.
type BiasBlock struct {
	Scanner   string `json:"scanner"`   // long | short | none
	Structure string `json:"structure"` // long | short | mixed
	Execution string `json:"execution"` // long_only | short_only | both | none | unknown
}

// SignalOptions carries the evaluation moment, the strategy's primary
// timeframe, and the quant layer's true 1h OI change. All indicator features
// are self-computed from closed bars — no display toggles involved.
type SignalOptions struct {
	EntryRoundTripCostBps float64   // disclosed cost estimate; raw research callers may pass zero
	Now                   time.Time // zero falls back to time.Now()
	PrimaryTF             string
	EnableLiquiditySweep  bool // opt-in experimental evidence; default false
	EnableFactorEnsemble  bool // opt-in experimental evidence; default false
	OI1hPct               *float64
	// CurrentPrice is the live ticker price used as the support/resistance
	// anchor (which pivot levels count as above/below). Zero falls back to
	// data.CurrentPrice, then to each timeframe's last closed close.
	CurrentPrice float64
	// LimitEntryOffsetPct is the fixed limit-entry anchor offset in percent:
	// limit_buy = price×(1-x/100), limit_sell = price×(1+x/100).
	// Zero falls back to 0.5% (the default strategy preference). It is the
	// fixed-mode value AND the ATR-unavailable fallback — see the
	// LimitEntryOffset* fields below.
	LimitEntryOffsetPct float64
	// Volatility-scaled anchor parameters (risk_control.limit_entry_offset_*):
	// mode "atr" (default when empty) = ATRMult × ATR(execution TF) clamped
	// to [MinPct, MaxPct]; "fixed" = LimitEntryOffsetPct as-is. The same
	// scaling drives the anchor breathing-room threshold.
	LimitEntryOffsetMode    string
	LimitEntryOffsetATRMult float64
	LimitEntryOffsetMinPct  float64
	LimitEntryOffsetMaxPct  float64
	// SupplyZonePct mirrors risk_control.open_reject_supply_pct: when > 0 the
	// pre-computed anchors are CROSS-VALIDATED against the 15m/1h structure —
	// an anchor parked within this % of the opposite-side level is zeroed out
	// (the trader would reject the order anyway; the model must not see a
	// tradable anchor that cannot execute).
	SupplyZonePct float64
	// PumpGuard4hPct mirrors risk_control.pump_guard_4h_pct (resolved): when
	// > 0, coins whose 4h trend-window return is at/above this percent are
	// EXTENDED pumps — longs then need a confirmed pullback (15m back above
	// its EMA20 AND a higher 15m swing low) or the long direction is blocked
	// (EXTENDED_PUMP_UNCONFIRMED). Resolved via kernel.PumpGuard4h — the
	// prompt builder and the trader's execution recompute must pass the SAME
	// value.
	PumpGuard4hPct float64

	QuoteVolume24hUsd      float64    // absolute 24h turnover (USDT)
	ScannerBias            string     // "short" when the symbol carries a short-scanner hint
	ScannerConflict        bool       // two scanners give OPPOSITE directional conclusions for this symbol
	Quant                  *QuantData // per-symbol quant snapshot (24h rolling change, current OI)
	LongShortAccountRatio  *float64   // global accounts long/short
	TopTraderPositionRatio *float64   // top traders' position long/short
	TakerBuySellRatio      *float64   // taker buy/sell volume ratio
	BtcCloses              []float64  // closed 1h BTC closes for correlation/beta
	// BtcTrendCloses: cached TRUE-4h BTC closes for the long-side BTC
	// filter (btc4hShape). Separate from BtcCloses so the correlation
	// input length stays untouched.
	BtcTrendCloses     []float64
	VendorStalenessPct *float64           // vendor forming close vs live ticker
	TraderHistory      *TraderHistoryStat // this trader's closed-trade record on the symbol
	// LossStreakBannedUntil is non-zero when the trader's circuit breaker has
	// banned this symbol from new opens until that moment (program-computed
	// from the closed-trade record — never the model's judgment).
	LossStreakBannedUntil time.Time
	// Min-position-size feasibility inputs: with notional = equity×risk% ÷
	// stop%, a stop beyond equity×risk%/minPositionSize cannot meet the
	// strategy's minimum position size (risk_control.min_position_size, web
	// strategy page — the same gate enforceMinPositionSize enforces).
	// Zero equity/risk/minPositionSize disables the MinSize block.
	EquityUSDT          float64
	RiskPct             float64 // RiskPerTradePct (prompt default 1.5)
	MinPositionSizeUSDT float64 // risk_control.min_position_size; default 12 when unset
	SLMinATRMult        float64 // stop noise floor multiplier; <=0 = no floor
	// Hard-entry gate inputs (09-15 review): the program re-evaluates the
	// strategy's own gates per direction so the model never re-derives them.
	MinRR             float64 // risk_control.min_risk_reward_ratio; <=0 = RR verdict skipped
	LimitEntryEnabled bool    // anchors gate the limit path when true; false = market-order default
	EntryTimingGate   bool    // micro-trend alignment is a HARD block only when enabled (executor parity)
	StockWeekendBlock bool    // symbol is a bstock AND US market weekend AND policy enabled
	// ShortFundingCrowdPctP is the configured per-8h settlement crowding
	// threshold in PERCENT (coin_source.short_scan_funding_rate_pct, default
	// 0.03). Scaled to the symbol's real settle interval, it defines the
	// "high" the funding_rollover condition rolls off — same number that
	// annualizes into strategy prompt condition ①.
	ShortFundingCrowdPctP float64
	// NetInflowUSDT: this symbol's institution futures net inflow (1h, from
	// the netflow ranking) — one input of the long-squeeze mirror condition.
	// nil = not in the inflow ranking.
	NetInflowUSDT *float64
	// SpreadPct: live order-book spread as % of mid (09-19 audit 五-7 — the
	// spread gate is code-enforced but invisible to the model). 0/unset =
	// book unavailable.
	SpreadPct *float64
	// MaxVendorDivergencePct hard-blocks both directions when the vendor
	// forming-close vs live ticker diverges beyond this percent
	// (risk_control.max_vendor_divergence_pct; resolved by
	// EffectiveMaxVendorDivergencePct: 0 = default 2, negative = disabled).
	// A 2.6% vendor/live gap shifts every anchor/SL/TP off the real market
	// (MYXUSDT 09-18) — entry, stop and target are all priced off the wrong
	// tick. <=0 (disabled) skips the check.
	MaxVendorDivergencePct float64
	// ---- Long-side entry discipline (user design 2026-10-01) ----
	// LongPullbackEntry: breakout-confirmed LONG anchors at the BROKEN level
	// (retest entry) instead of snapshot−offset (nil=true).
	LongPullbackEntry bool
	// LongMaxEMA20DistPct: chase entries beyond this distance above the 4h
	// EMA20 fail with EMA20_STRETCH; the retest anchor path is exempt.
	// <=0 = check disabled.
	LongMaxEMA20DistPct float64
	// Anti-chase symmetric entry gates (Opus analysis 2026-10-11, 273 live
	// trades; risk_control.chase_max_ema20_1h_pct / counter_max_ema20_1h_pct /
	// entry_rsi1h_max / chase_max_move4h_pct, resolved by the Effective*
	// accessors). All <=0 = that check disabled. Measured from the gate's
	// entry price; bstock symbols exempt; missing data fails open.
	ChaseMaxEMA20_1hPct   float64 // CHASE_EMA20_1H: entry > X% beyond 1h EMA20 in trade direction
	CounterMaxEMA20_1hPct float64 // COUNTER_EMA20_1H: entry > X% on the wrong side of 1h EMA20
	EntryRSI1hMax         float64 // RSI1H_EXHAUSTED: long RSI1h >= X / short RSI1h <= 100-X
	ChaseMaxMove4hPct     float64 // CHASE_MOVE4H: 4h move > X% in trade direction
	// BTCFilterLong: altcoin longs need coin 24h return ≥ BTC's and BTC 4h
	// closes not declining; failure emits BTC_WEAK_LONG.
	BTCFilterLong bool
	// BTCFilterShort: opt-in hard gate (nil config = OFF — the playbook
	// handles BTC strength for shorts at scan level). While on, open_short
	// is blocked when the SHARED BTC 4h classifier reads strongBull
	// (BTC_4H_STRONGBULL), mirroring the long side's market-level filter.
	BTCFilterShort bool
	// ShortTopConfirmGate: the playbook's 默认姿态 program-enforced — a
	// short_scan candidate may only open short with the scanner's topping
	// confirmation (顶背离/假突破; 费率回落 via live funding_rollover.detected); missing confirmation
	// emits SHORT_TOP_CONFIRM_MISSING. Candidates without short_scan
	// evidence (ShortScanConfirmed == nil) are not gated — the 15m
	// MICRO_TREND gate still applies to them.
	ShortTopConfirmGate bool
	// ShortScanConfirmed carries the short_scan candidate's topping
	// confirmation (nil = this candidate has no short_scan evidence).
	ShortScanConfirmed *bool
	// SentimentLongDeweightPts / SentimentLongDeweightArmed: while ARMED
	// (engine-side: crypto FNG ≥ threshold), a LONG's directional score loses
	// this many points at the gate reads (CONSENSUS_OPPOSED / NEG_EDGE_SCORE
	// / breakout market exception). Pts <=0 or !Armed = off.
	SentimentLongDeweightPts   int
	SentimentLongDeweightArmed bool
	// MaxStopDistancePct: a stop plan further than this from the entry fails
	// the direction with WIDE_STOP (both directions; <=0 = check disabled).
	MaxStopDistancePct float64
	// NEGATIVE_EDGE health gate (user review 2026-09-27 #4): active only
	// while the strategy's rolling stats sit on the negative edge (PF<0.9).
	// Resolved via the NegativeEdge* resolvers in anchor_offset.go — the
	// prompt prose and this gate always quote the same numbers.
	NegativeEdge            bool    // strategy_health == NEGATIVE_EDGE this cycle
	NegativeEdgeMinScore    float64 // |directional_score| floor; 0 = condition off
	NegativeEdgeMinRR       float64 // first-target RR floor; 0 = condition off
	NegativeEdgeBlockLosing bool    // block net-losing symbols
	// ConfiguredTimeframes is the strategy's fetched timeframe list
	// (Indicators.Klines.SelectedTimeframes as expanded by the fetch path).
	// DataQuality requires 60+ bars only for timeframes the strategy ACTUALLY
	// fetches — the static 4h requirement pinned every candidate of a
	// ["5m","15m","1h"] strategy to DATA_INSUFFICIENT (GetWithTimeframes
	// fetches only the configured list; there is no 4h fallback on the kernel
	// path — round-4 review R4-3). Empty = legacy execution-side recompute on
	// GetWithExchange data, which always carries 3m/4h/1h + aggregated 15m:
	// the historical {15m,1h,4h} requirement is kept there.
	ConfiguredTimeframes []string
	// PositionSide threads the HELD position's direction ("long"/"short";
	// empty = not held) into exit_rule_triggered — the rule evaluates the
	// matching side's mirror only (2026-10-09 ONUSDT: a short in a working
	// 15m downtrend must not see the long-side exit flag). Empty → the flag
	// is false; the prompt builder looks the side up from Context.Positions.
	PositionSide string
}

// ComputeSymbolSignals builds the normalized signal block for one symbol from
// its multi-timeframe market data. Returns an error when the data is too
// incomplete to trade on — the caller must then EXCLUDE the symbol (missing
// data = no trading).
func ComputeSymbolSignals(symbol string, data *market.Data, opt SignalOptions) (*SymbolSignal, error) {
	if data == nil {
		return nil, fmt.Errorf("no market data")
	}
	if data.CurrentPrice <= 0 {
		return nil, fmt.Errorf("invalid price %.8f", data.CurrentPrice)
	}
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	// Limit-entry anchor offset (percent). Strategy-configurable via
	// risk_control.limit_entry_offset_*; ATR-scaled with a fixed fallback —
	// the anchors themselves are computed AFTER the TF loop below (they read
	// the primary TF's ATR).
	offsetCfg := AnchorOffsetConfig{
		Mode:     opt.LimitEntryOffsetMode,
		FixedPct: opt.LimitEntryOffsetPct,
		ATRMult:  opt.LimitEntryOffsetATRMult,
		MinPct:   opt.LimitEntryOffsetMinPct,
		MaxPct:   opt.LimitEntryOffsetMaxPct,
	}

	sig := &SymbolSignal{
		rrCostBps:        opt.EntryRoundTripCostBps,
		Symbol:           strings.ToUpper(symbol),
		TimestampUTC:     now.UTC().Format(time.RFC3339),
		Price:            data.CurrentPrice,
		PriceChange4hPct: data.PriceChange4h,
		PrimaryTF:        opt.PrimaryTF,
		Timeframes:       map[string]*TFSignal{},
	}

	// Deterministic timeframe ordering. Sub-15m blocks are DROPPED (09-19
	// audit 八: ~450 chars/coin of dead JSON — role_tfs is 15m/1h/4h and no
	// strategy rule reads anything finer; the micro-trend gate uses 15m/30m).
	tfs := make([]string, 0, len(data.TimeframeData))
	for tf := range data.TimeframeData {
		if tfDuration(tf) < 15*time.Minute && tf != opt.PrimaryTF {
			continue
		}
		tfs = append(tfs, tf)
	}
	sort.Slice(tfs, func(i, j int) bool { return tfDuration(tfs[i]) < tfDuration(tfs[j]) })

	complete := len(tfs) > 0
	// Support/resistance anchor: the live ticker price when available, else
	// the fetch's current price. Filtering pivot levels against a stale
	// per-timeframe close misclassifies levels the live price already crossed
	// (e.g. a daily "resistance" below the live price after an intraday pump).
	anchorPrice := opt.CurrentPrice
	if anchorPrice <= 0 {
		anchorPrice = data.CurrentPrice
	}
	// Closed 1h closes (for BTC correlation) and the primary TF's settlement
	// age (for the freshness marker).
	var sym1hCloses []float64
	var primarySettlementAge float64
	for _, tf := range tfs {
		tfData := data.TimeframeData[tf]
		if tfData == nil || len(tfData.Klines) < 10 {
			complete = false
			sig.Warnings = append(sig.Warnings, fmt.Sprintf("%s: insufficient bars (%d)", tf, len(tfData.Klines)))
			continue
		}
		tfsig, warns := computeTFSignal(tf, tfData, now, anchorPrice)
		sig.Timeframes[tf] = tfsig
		if len(warns) > 0 {
			complete = false
			sig.Warnings = append(sig.Warnings, warns...)
		}
		if tf == "1h" {
			sym1hCloses = closedCloses(tfData, now, tfDuration(tf))
		}
		if tf == opt.PrimaryTF || (opt.PrimaryTF == "" && primarySettlementAge == 0) {
			closes := closedCloses(tfData, now, tfDuration(tf))
			if len(closes) >= 2 {
				kl := ClosedKlines(tfData, now, tfDuration(tf))
				lastClose := kl[len(kl)-1].Time + tfDuration(tf).Milliseconds()
				primarySettlementAge = now.Sub(time.UnixMilli(lastClose)).Seconds()
			}
		}
	}

	// TP-menu touch source (user directive 2026-09-29): the closed 1h series
	// for crypto, 1d for equity tokens — the same rhythm class the structure
	// scan prices stops and targets on. Capped at ~30 days.
	{
		touchTF := "1h"
		maxBars := 720
		if market.IsBStockSymbol(symbol) {
			touchTF, maxBars = "1d", 30
		}
		if td := data.TimeframeData[touchTF]; td != nil {
			kl := ClosedKlines(td, now, tfDuration(touchTF))
			if len(kl) > maxBars {
				kl = kl[len(kl)-maxBars:]
			}
			if len(kl) > 0 {
				sig.touchHighs = make([]float64, len(kl))
				sig.touchLows = make([]float64, len(kl))
				for i, k := range kl {
					sig.touchHighs[i] = k.High
					sig.touchLows[i] = k.Low
				}
				sig.touchWindowDays = int(float64(len(kl))*tfDuration(touchTF).Hours()/24 + 0.5)
			}
		}
	}

	// Derivatives: funding + OI. The 1h OI change comes from the quant layer
	// when available; latest-vs-average is reported separately (it is NOT a
	// 1h change).
	d := &DerivSignal{}
	if data.FundingRateOK {
		// A real zero (bstock tokens pay no funding) is DATA, not absence —
		// keep it visible as 0; only a failed fetch is reported as missing.
		f := data.FundingRate
		d.FundingRate = &f
	}
	if opt.OI1hPct != nil {
		d.OIChange1hPct = opt.OI1hPct
	}
	if opt.Quant != nil {
		if pc, ok := opt.Quant.PriceChange["24h"]; ok {
			// QuantData.PriceChange is a DECIMAL fraction (0.0723 = 7.23% —
			// the quant renderer multiplies by 100). This field's JSON key is
			// a _pct name rounded at 2dp, so the conversion happens HERE.
			// Round-4 review R4-2: the raw pass-through shipped 100×-too-small
			// values (7.23% read as 0.07) while sibling
			// price_change_60m_live_pct is a true percent.
			v := pc * 100
			d.PriceChange24hLivePct = &v
		}
		if qoi := opt.Quant.OI["binance"]; qoi != nil && qoi.CurrentOI > 0 {
			v := qoi.CurrentOI
			d.OICurrentBase = &v
		}
	}
	if data.OpenInterest != nil && data.OpenInterest.Average > 0 {
		vsAvg := (data.OpenInterest.Latest - data.OpenInterest.Average) / data.OpenInterest.Average * 100
		d.OIVsAvgPct = &vsAvg
	}
	// Rolling live change (Binance 15m×4) when provided — the raw
	// data.PriceChange1h anchors on the primary TF's candle boundary and, with
	// a stale vendor forming close, can degenerate into the vendor divergence
	// figure (identical numbers, different concept).
	// Self-computed rolling 1h change from the symbol's own klines — shares
	// no anchor with the vendor's frozen forming candle, so it can no longer
	// degenerate into a copy of the vendor divergence figure. Falls back to
	// the fetch-layer value only when no ≤1h timeframe has enough bars.
	p := rolling1hChangePct(data, now)
	d.PriceChange1hLivePct = &p
	if opt.LongShortAccountRatio != nil {
		d.LongShortAccountRatio = opt.LongShortAccountRatio
	}
	if opt.TopTraderPositionRatio != nil {
		d.TopTraderPositionRatio = opt.TopTraderPositionRatio
	}
	if opt.TakerBuySellRatio != nil {
		d.TakerBuySellRatio = opt.TakerBuySellRatio
	}
	if opt.Quant != nil && opt.Quant.Liquidation != nil {
		d.Liquidation = opt.Quant.Liquidation
	}
	if opt.Quant != nil && len(opt.Quant.News) > 0 {
		d.News = opt.Quant.News
	}
	sig.Derivatives = d
	// ⑤ Precomputed funding annualization on the symbol's REAL settlement
	// interval (measured from settlement timestamps, see market.Data).
	// Falls back to the 8h×3 convention when the interval is unknown — a
	// fixed ×3 understates 4h/1h listings by 2-8× and made the structured
	// signal disagree with the scanner's properly annualized figure.
	if d.FundingRate != nil && *d.FundingRate != 0 { // rate 0 → ann 0, no annualization needed
		ann := annualizeFunding(*d.FundingRate, data.FundingSettleHours)
		d.FundingAnnualizedPct = &ann
		if data.FundingSettleHours > 0 {
			h := data.FundingSettleHours
			d.FundingSettleHours = &h
		}
	}
	// Program-verified crowding condition ② (review 09-15 point 6): the
	// settled rate 3 intervals ago sat above the symbol-scaled crowding
	// threshold and the live forward rate is now below it. Same definition the
	// short scanner uses (market.FundingRolloverDetected) — the scanner keeps
	// its looser snapshot threshold, but "回落" can no longer mean two things
	// in one prompt. History missing → field absent = UNKNOWN.
	if len(data.FundingHistory) >= 4 && d.FundingRate != nil {
		settle := 8.0
		if data.FundingSettleHours > 0 {
			settle = data.FundingSettleHours
		}
		// Condition ①'s threshold (per 8h, percent) scaled to this symbol's
		// real interval: annualized X over frPct×3×365 ⇒ raw > frPct/100 ×
		// settle/8 per settlement.
		threshPct := opt.ShortFundingCrowdPctP
		if threshPct <= 0 {
			threshPct = 0.03
		}
		threshRaw := threshPct / 100 * settle / 8
		prev := data.FundingHistory[len(data.FundingHistory)-4]
		cur := *d.FundingRate
		from := annualizeFunding(prev, settle)
		to := annualizeFunding(cur, settle)
		d.FundingRollover = &FundingRolloverState{
			Detected:          market.FundingRolloverDetected(prev, cur, threshRaw),
			FromAnnualizedPct: &from,
			ToAnnualizedPct:   &to,
			SettlesBack:       3,
		}
	}

	if opt.QuoteVolume24hUsd > 0 || opt.SpreadPct != nil {
		sig.Liquidity = &LiquiditySignal{SpreadPct: opt.SpreadPct}
		if opt.QuoteVolume24hUsd > 0 {
			v := opt.QuoteVolume24hUsd
			sig.Liquidity.QuoteVolume24hUsd = &v
		}
	}
	// Long-squeeze mirror (09-19 audit 七): negative funding + retail net
	// short + institution inflow — three independent legs, ALL required.
	if d.FundingAnnualizedPct != nil && *d.FundingAnnualizedPct < -5 &&
		d.LongShortAccountRatio != nil && *d.LongShortAccountRatio < 1 &&
		opt.NetInflowUSDT != nil && *opt.NetInflowUSDT > 0 {
		d.LongSqueeze = &LongSqueezeState{
			Detected:              true,
			FundingAnnualizedPct:  d.FundingAnnualizedPct,
			LongShortAccountRatio: d.LongShortAccountRatio,
			NetInflowUSDT:         opt.NetInflowUSDT,
		}
	}
	if opt.TraderHistory != nil {
		sig.TraderHistory = opt.TraderHistory
	}
	if len(opt.BtcCloses) > 0 && len(sym1hCloses) > 0 {
		sig.BTC = btcCorrelation(sym1hCloses, opt.BtcCloses)
	}
	fresh := &DataFreshness{SettlementAgeSeconds: primarySettlementAge}
	if opt.VendorStalenessPct != nil {
		fresh.VendorDivergencePct = opt.VendorStalenessPct
	}
	sig.DataFreshness = fresh

	// Directional evidence tally + typed conflict flag (⑥⑦).
	// Range/unknown timeframes contribute NOTHING — a 15m/5m range split is
	// noise, not a directional conflict. A genuine TIMEFRAME_SPLIT requires
	// the 1h and 4h trends to actually oppose each other.
	bull, bear := 0, 0
	for _, tf := range sig.Timeframes {
		if tf == nil {
			continue
		}
		switch tf.Trend {
		case "up":
			bull++
		case "down":
			bear++
		}
	}
	if sig.Derivatives != nil && sig.Derivatives.PriceChange1hLivePct != nil {
		switch {
		case *sig.Derivatives.PriceChange1hLivePct > 0:
			bull++
		case *sig.Derivatives.PriceChange1hLivePct < 0:
			bear++
		}
	}
	if opt.ScannerBias == "short" {
		bear++ // the scanner's own bias is one unit of bearish evidence
	} else if opt.ScannerBias == "long" {
		bull++
	}
	conflict := &SignalConflict{BullishEvidence: bull, BearishEvidence: bear}
	if total := bull + bear; total > 0 {
		conflict.DirectionalScore = int(math.Round(100 * float64(bull-bear) / float64(total)))
	}
	var tfUp, tfDown bool
	t1h := sig.Timeframes["1h"]
	t4h := sig.Timeframes["4h"]
	if t1h != nil {
		tfUp = t1h.Trend == "up"
		tfDown = t1h.Trend == "down"
	}
	if t4h != nil {
		tfUp = tfUp && t4h.Trend == "up"
		tfDown = tfDown && t4h.Trend == "down"
		if t1h != nil &&
			((t1h.Trend == "up" && t4h.Trend == "down") || (t1h.Trend == "down" && t4h.Trend == "up")) {
			conflict.DirectionalConflict = true
			conflict.Types = append(conflict.Types, "TIMEFRAME_SPLIT")
		}
	}
	switch {
	case opt.ScannerBias == "short" && tfUp:
		conflict.DirectionalConflict = true
		conflict.Types = append(conflict.Types, "SCANNER_VS_STRUCTURE")
		conflict.Note = fmt.Sprintf("scanner bias SHORT vs bullish 1h+4h structure — resolve before trading")
	case opt.ScannerBias == "long" && tfDown:
		conflict.DirectionalConflict = true
		conflict.Types = append(conflict.Types, "SCANNER_VS_STRUCTURE")
		conflict.Note = "scanner bias LONG vs bearish 1h+4h structure — resolve before trading"
	}
	// Cross-scanner conflict (audit 2026-09-12 #11): short_scan says SHORT
	// while piggy_dash selected UP for the same symbol — the model must
	// resolve which scanner to trust, never average them away.
	if opt.ScannerConflict {
		conflict.DirectionalConflict = true
		conflict.Types = append(conflict.Types, "SCANNER_VS_SCANNER")
		conflict.Note = "short_scan (SHORT) and piggy_dash (UP) disagree on this symbol — resolve the conflict before trading"
	}
	if conflict.DirectionalConflict && conflict.Note == "" {
		conflict.Note = "1h and 4h trends oppose — resolve the regime before trading"
	}
	sig.SignalConflict = conflict

	// ⑧ Execution filter — mirror of the trader's micro-trend gate.
	sig.ExecutionFilter = executionFilterFromTimeframes(sig.Timeframes, sig.Price)

	// 09-19 audit: execution↔structure opposition. ZEC 09-18 read
	// directional_score +100 (4 bull / 0 bear) while the execution filter
	// permitted shorts only — the model was told to short a +100 consensus
	// and the conflict machinery stayed silent. When the ONLY direction the
	// micro window permits OPPOSES the 1h+4h structure read, flag it: the
	// model must resolve the contradiction with evidence, and the
	// market-order exceptions (which require directional_conflict=false)
	// stay closed.
	if sig.ExecutionFilter != nil {
		execDir := ""
		switch {
		case sig.ExecutionFilter.LongAllowed && !sig.ExecutionFilter.ShortAllowed:
			execDir = "long"
		case sig.ExecutionFilter.ShortAllowed && !sig.ExecutionFilter.LongAllowed:
			execDir = "short"
		}
		// Structure direction = the SAME derivation as bias.structure
		// (directional_score ±20) — the strict 1h&&4h=="up" reading missed
		// WLFI (score +50 with 4h "rally"): the flag never fired on exactly
		// the textbook case the system prompt cites.
		structDir := ""
		if conflict.DirectionalScore >= 20 {
			structDir = "long"
		} else if conflict.DirectionalScore <= -20 {
			structDir = "short"
		}
		if execDir != "" && structDir != "" && execDir != structDir {
			conflict.DirectionalConflict = true
			conflict.Types = append(conflict.Types, "EXECUTION_VS_STRUCTURE")
			conflict.Note = fmt.Sprintf("execution filter (%s %s) allows only %s while 1h+4h structure reads %s — resolve the contradiction with evidence before trading",
				sig.ExecutionFilter.MicroTF, sig.ExecutionFilter.MicroTrend, execDir, structDir)
		}
	}

	// ⑨ Role timeframes.
	sig.RoleTFs = ResolveRoleTimeframes(nil)
	sig.RoleTFs.ExecutionTF = opt.PrimaryTF
	if line := RegimeLine(data, sig.RoleTFs.TrendTF, now); line > 0 {
		rl := &RegimeLineSignal{TF: sig.RoleTFs.TrendTF, EMA50: line}
		// Precompute the comparisons so the model never re-derives "above
		// the line" from raw decimals (2026-10-09 ONUSDT misread).
		if td := data.TimeframeData[sig.RoleTFs.TrendTF]; td != nil {
			if bars := ClosedKlines(td, now, market.TimeframeDuration(sig.RoleTFs.TrendTF)); len(bars) > 0 {
				rl.CloseAbove = bars[len(bars)-1].Close > line
			}
		}
		if data.CurrentPrice > 0 {
			rl.LiveAbove = data.CurrentPrice > line
		}
		sig.RegimeLine = rl
	}
	// First-class market regime (closed 1h/4h bars only). This is computed
	// before hard_entry_gate because market-order exceptions consume it.
	sig.MarketRegime = computeMarketRegime(data, now)

	// ⑩ Data quality — complete (fetched) vs sufficient (enough history for
	// the indicator stack: RSI14/MACD(26)/EMA50 need ≥60 closed bars).
	// Only timeframes the analysis stack actually reads are required. The
	// old static "1d" entry was never fetched, so its 0 bars pinned
	// sufficient=false forever (review 2026-09-07: dead flag).
	// Round-4 review R4-3: 4h (and any other TF beyond 15m/1h) is required
	// only when the strategy's fetch list carries it — see
	// SignalOptions.ConfiguredTimeframes.
	// 2026-10-03 review (HIGH): "15m" was STATIC here while 4h had been made
	// dynamic in R4-3 — a strategy whose fetch list omits 15m (e.g.
	// ["5m","1h","4h"]) saw 0 bars for it, sufficient=false, and EVERY
	// candidate failed DATA_INSUFFICIENT forever (silent permanent all-wait).
	// The base is now 1h only; 15m enters through ConfiguredTimeframes (or
	// PrimaryTF) like every other TF.
	minBars := map[string]int{"1h": 60}
	if opt.PrimaryTF != "" {
		if _, ok := minBars[opt.PrimaryTF]; !ok {
			minBars[opt.PrimaryTF] = 60
		}
	}
	if len(opt.ConfiguredTimeframes) > 0 {
		for _, tf := range opt.ConfiguredTimeframes {
			if tf == "" {
				continue
			}
			if _, ok := minBars[tf]; !ok {
				minBars[tf] = 60
			}
		}
	} else if _, ok := minBars["4h"]; !ok {
		// Legacy caller (execution-side recompute): GetWithExchange always
		// provides 4h, keep the historical requirement.
		minBars["4h"] = 60
	}
	avail := map[string]int{}
	sufficient := true
	var shortfall []string
	for tfName, min := range minBars {
		n := 0
		if tfData, ok := data.TimeframeData[tfName]; ok && tfData != nil {
			n = len(tfData.Klines)
		}
		avail[tfName] = n
		if n > 0 && n < min {
			sufficient = false
			shortfall = append(shortfall, fmt.Sprintf("%s: %d bars < %d minimum — long-window indicators (EMA50/MACD) are unstable", tfName, n, min))
		}
		if n == 0 {
			sufficient = false
			// Name the missing timeframe: a bare sufficient=false with no
			// shortfall entry read as a mystery block (configured-but-unfetched
			// is the usual suspect after R4-3).
			shortfall = append(shortfall, fmt.Sprintf("%s: 0 bars — required timeframe has no data (configured but fetch failed?)", tfName))
		}
	}
	sig.DataQuality = &DataQuality{
		Complete:      complete && sig.Price > 0,
		Sufficient:    sufficient,
		AvailableBars: avail,
		MinimumBars:   minBars,
		Shortfall:     shortfall,
	}

	// ⑪ Pre-computed limit-entry anchors — volatility-scaled: offset =
	// ATRMult × ATR(1h) clamped to [MinPct, MaxPct], falling back to the
	// fixed percent when ATR is unavailable (review 2026-09-07: a fixed 0.3%
	// demanded most of a quiet book's ATR and rode the live price for violent
	// movers; user directive 2026-09-10: the yardstick is ATR(1h), not the
	// B2 (QUANT_REVIEW 09-22): both yardsticks now use the SAME exact-TF
	// lookup as the trader's gate (AnchorATRPct / ExecutionATRPct). The old
	// inline fallback went through primaryTFSignal, which hands back the
	// LONGEST available TF when the wanted one is missing — the prompt-side
	// offset/breathing then silently ran on a 4h scale while the executor
	// priced the same anchor off the exact execution TF (or the fixed
	// fallback). One setting, two thresholds, divergence by data arrival.
	var anchorATRPct float64
	if t1h, ok := sig.Timeframes["1h"]; ok && t1h != nil {
		anchorATRPct = t1h.ATRPct
	} else if tExec, ok := sig.Timeframes[opt.PrimaryTF]; ok && tExec != nil {
		anchorATRPct = tExec.ATRPct
	}
	sig.LimitEntryOffsetPct = AnchorOffsetPct(anchorATRPct, offsetCfg)
	sig.LimitBuyPrice = data.CurrentPrice * (1 - sig.LimitEntryOffsetPct/100)
	sig.LimitSellPrice = data.CurrentPrice * (1 + sig.LimitEntryOffsetPct/100)

	execATRPct := 0.0
	if tExec, ok := sig.Timeframes[opt.PrimaryTF]; ok && tExec != nil {
		execATRPct = tExec.ATRPct
	}
	if opt.SupplyZonePct > 0 {
		breathing := AnchorBreathingPct(execATRPct, offsetCfg, opt.SupplyZonePct)
		suppressAnchorsAgainstStructure(sig, data, breathing, now)
	}

	// ⑫⑬ Breakout state + retest anchor (user design 2026-10-01, "保留突破,改入场
	// 方式"): a confirmed LONG breakout does NOT chase the extension — the
	// limit anchor moves to the BROKEN level (old resistance → support) so
	// the fill lands on the retest, and the stop plan / RR below compute from
	// that anchor (structural stop under the level → short distance → honest
	// RR). MUST run AFTER sig.Breakout is assigned (2026-10-03 P0 review: the
	// block originally sat before computeBreakoutState and read a nil
	// Breakout — the whole feature was dead in production while its
	// builder-level tests passed) and AFTER suppressAnchorsAgainstStructure,
	// which must NOT zero this anchor: it intentionally sits at the broken
	// level, and the pivots just above it are the breakout's own extension,
	// not overhead supply (the plan carries the geometry for the executor's
	// supply exemption via GateState.LongPullback*).
	sig.Breakout = computeBreakoutState(data, sig, now)
	sig.LongPullback = buildLongPullbackPlan(sig, data, opt)
	if sig.LongPullback != nil && sig.LongPullback.Active {
		sig.LimitBuyPrice = sig.LongPullback.Entry
	}
	// 2026-10-09 regime line → trend TF (user option B): after retest
	// override, before any entry-price/stop/RR consumer or prompt rendering.
	suppressAnchorsAgainstRegimeLine(sig, opt.EntryTimingGate)
	sig.BBRide = computeBBRide(data, now)
	sig.ShortRide = computeBBShortRide(data, now)
	if opt.EnableLiquiditySweep {
		sig.LiquiditySweep = computeLiquiditySweep(data, now)
	}
	if opt.EnableFactorEnsemble {
		sig.FactorEnsemble = computeFactorEnsemble(data, now)
	}

	// Entry / exit heuristic rules — deterministic, computed so the model sees
	// the program's verdict and can agree or disagree with reasoning.
	primary := primaryTFSignal(sig, opt.PrimaryTF)
	if primary != nil {
		sig.EntryTriggered = primary.Trend == "up" &&
			primary.RSI14 != nil && *primary.RSI14 > 40 && *primary.RSI14 < 72 &&
			primary.VolumeRatio != nil && *primary.VolumeRatio >= 1.0 &&
			primary.LastClosedCandle == "bullish"
		structLow, structHigh := 0.0, 0.0
		if primary.StructureLow != nil {
			structLow = *primary.StructureLow
		}
		if primary.StructureHigh != nil {
			structHigh = *primary.StructureHigh
		}
		// Exit rules are DIRECTION-AWARE (2026-10-09 ONUSDT incident): the
		// old block evaluated only the long-side semantics (break of the
		// structure low / RSI>80 / trend down + bearish close) and rendered
		// the flag for ANY held position — for a SHORT those conditions are
		// the thesis WORKING (downtrend continuing), yet the model read
		// "exit_rule_triggered": true as a program exit directive and closed
		// a profitable short into a loss. Each side now gets its own mirror;
		// an unknown side (not held) stays false — the render site's phantom
		// suppression inherits this default.
		longExit := (structLow > 0 && sig.Price < structLow) ||
			(primary.RSI14 != nil && *primary.RSI14 > 80) ||
			(primary.Trend == "down" && primary.LastClosedCandle == "bearish")
		shortExit := (structHigh > 0 && sig.Price > structHigh) ||
			(primary.RSI14 != nil && *primary.RSI14 < 20) ||
			(primary.Trend == "up" && primary.LastClosedCandle == "bullish")
		switch opt.PositionSide {
		case "long":
			sig.ExitTriggered = longExit
		case "short":
			sig.ExitTriggered = shortExit
		default:
			sig.ExitTriggered = false
		}
	}

	// Loss-streak circuit breaker: the trader computes the ban from the
	// closed-trade record; the signal only mirrors the verdict so the model
	// never has to (and must not) self-judge it.
	if !opt.LossStreakBannedUntil.IsZero() {
		hoursLeft := opt.LossStreakBannedUntil.Sub(now).Hours()
		if hoursLeft < 0 {
			hoursLeft = 0
		}
		sig.LossStreak = &LossStreakState{
			Banned:    true,
			UntilUTC:  opt.LossStreakBannedUntil.UTC().Format(time.RFC3339),
			HoursLeft: math.Round(hoursLeft*10) / 10,
		}
	}

	// Min-position-size feasibility: notional = equity×risk% ÷ (stop%+cost%), so the
	// widest stop the rules allow must still satisfy equity×risk%/stop ≥
	// min_position_size. The binding constraint is the NOISE FLOOR
	// (SLMinATRMult×ATR(1h)) — if even that floor overshoots, no permitted
	// stop can produce a tradable notional (structural dead zone). Mirrors
	// the sizing formula and the executor's enforceMinPositionSize rejection.
	//
	// 09-29 user report (QNTUSDT "MIN_QTY: quantity 0.0534 rounds down to
	// 0.0, stepSize=0.1"): the strategy minimum is not the only floor — the
	// EXCHANGE's lot structure is. A 0.1-step symbol at price 260 cannot
	// trade below ≈26U notional; sizing under that dies at placement with
	// the quantity rounded to zero. The binding minimum is therefore
	// max(strategy min, exchange min-tradable notional).
	if opt.EquityUSDT > 0 && opt.RiskPct > 0 && opt.MinPositionSizeUSDT > 0 {
		riskUSD := opt.EquityUSDT * opt.RiskPct / 100
		floorPct := stopFloorPct(sig, opt.SLMinATRMult)
		minSize := opt.MinPositionSizeUSDT
		lotNote := ""
		if f := market.GetSymbolLotFilter(sig.Symbol); f.MinQty > 0 || f.MinNotional > 0 {
			if exch := f.MinTradableNotional(sig.Price); exch > minSize {
				minSize = exch
				lotNote = fmt.Sprintf(" (exchange lot floor: step %.4g minQty %.4g → ≥%.0fU at price %.6g)",
					f.StepSize, f.MinQty, exch, sig.Price)
			}
		}
		// Remaining stop-distance budget after round-trip costs at the minimum.
		maxStopPct := riskUSD/minSize*100 - opt.EntryRoundTripCostBps/100
		ms := &MinSizeCheck{
			Feasible:           floorPct <= maxStopPct,
			MinPositionSizeUsd: math.Round(minSize*100) / 100,
			MaxStopPct:         math.Round(maxStopPct*100) / 100,
			StopFloorPct:       math.Round(floorPct*100) / 100,
		}
		if !ms.Feasible {
			ms.Reason = fmt.Sprintf(
				"stop floor %.2f%% (SLMinATR %.1f×ATR(1h)) exceeds %.2f%% max for the %.0fU minimum position size at equity %.1fU%s — no allowed stop can meet the binding minimum, wait+MIN_SIZE",
				floorPct, opt.SLMinATRMult, maxStopPct, minSize, opt.EquityUSDT, lotNote)
		}
		sig.MinSize = ms
	}

	// Program verdict of the "three hard gates" per direction + bias source
	// layering (review 2026-09-15 points 1-3/5/7/8/10): the model reads the
	// verdict and explains it instead of re-scanning structure and
	// re-assembling blockers every cycle.
	sig.PumpGuard = computePumpGuard(sig, data, opt.PumpGuard4hPct, now)
	sig.HardGate = computeHardEntryGate(sig, opt)
	sig.Bias = computeBiasBlock(sig, opt)

	sig.DataComplete = complete && sig.Price > 0
	if !complete {
		sig.Warnings = append(sig.Warnings, "DATA INCOMPLETE — trading this symbol is prohibited this cycle")
	}
	// Derivatives coverage gaps are informational (tokenized-stock perps and
	// some listings have no funding/long-short data) — flag them so the model
	// does not treat the absence as "no crowding".
	if !data.FundingRateOK {
		sig.Warnings = append(sig.Warnings, "derivatives: funding_rate fetch FAILED this cycle — treat crowding as UNKNOWN, not absent; retries next cycle")
	}
	if opt.LongShortAccountRatio == nil {
		sig.Warnings = append(sig.Warnings, "derivatives: long/short account and top-trader ratios unavailable — crowd-positioning UNKNOWN")
	}
	return sig, nil
}

// annualizeFunding converts a raw per-settlement funding rate to an
// annualized percent on the symbol's real settlement interval (8h×3/day
// fallback when unknown, per-day clamped to [1, 24] — the upper bound caps
// sub-hourly settlement-hours artifacts, keeping 1h the fastest credited
// interval, 09-28 review P3).
func annualizeFunding(rate, settleHours float64) float64 {
	perDay := 3.0
	if settleHours > 0 {
		perDay = 24.0 / settleHours
		if perDay < 1 {
			perDay = 1
		}
		if perDay > 24 {
			perDay = 24
		}
	}
	return rate * perDay * 365 * 100
}

// stopFloorPct mirrors the executor's noise floor (SLMinATRMult × ATR(1h),
// closed bars) — 0 when no floor is configured. The multiplier comes from
// strategy config (sl_min_atr_mult), NOT a fixed 1.5. Single source for the
// min_size block and the hard-entry gate.
func stopFloorPct(sig *SymbolSignal, mult float64) float64 {
	if mult <= 0 {
		return 0
	}
	// Equity tokens use a DAILY-scale floor. Missing daily data must fail
	// closed in computeHardEntryGate; falling back to 1h would silently switch
	// the stop methodology back to crypto scale.
	if market.IsBStockSymbol(sig.Symbol) {
		if td, ok := sig.Timeframes["1d"]; ok && td != nil && td.ATRPct > 0 {
			return mult * td.ATRPct
		}
		return 0
	}
	if t1h, ok := sig.Timeframes["1h"]; ok && t1h != nil {
		return mult * t1h.ATRPct
	}
	return 0
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// Direction-aware structure-stop buffer, in ×ATR(1h). These are the WIDE end
// of the methodology's 0.3-0.5 band (longs take the lower half 0.3-0.4 → 0.4;
// shorts — especially bounce/chase shorts with longer wicks — take the upper
// half 0.4-0.5 → 0.5). Pricing the stop plan at the wide end makes the gate's
// RR a true LOWER bound: the model can only tighten into the band, never
// manufacture a passing RR.
const (
	methodStopBufferLong  = 0.4
	methodStopBufferShort = 0.5
)

// methodStopPlan precomputes the METHODOLOGY stop for one direction by
// STEPPING OUT along the ≥15m opposite-side structure levels, near→far
// (5m pivots excluded — noise at a 1h-ATR buffer scale): the plan is the
// FIRST structure whose buffer-extended stop distance lands inside the band
// [noiseFloor, max(2×ATR(4h), 8%)]. The buffer takes the WIDE end of the
// methodology's 0.3-0.5 band (short 0.5 / long 0.4 ×ATR(1h)) so the gate's
// RR is a lower bound.
//
// 09-19 audit (user): the first cut CLAMPED a too-close structure up to the
// noise floor — manufacturing a stop in no-man's-land (ZEC long: plan
// 1503.59 between structures 1545.95 and 1475.71) that scored RR 1.95 where
// the real methodology stop (≈1463, d 5.79%) scores 1.06. Clamping is gone:
// too-close structures step out; an accepted plan is always a real
// structure ± buffer.
//
// Codes: "" = ok; STOP_PLAN_NO_STRUCTURE = no ≥15m level beyond entry (new
// highs/lows); STOP_PLAN_OUT_OF_BAND = no structure lands inside the band
// (nearest too tight, rest too wide — 放弃该设置 per methodology).
func methodStopPlan(sig *SymbolSignal, entry, floorPct float64, isLong bool) (price, distPct float64, code string) {
	// Buffer yardstick and structure horizon must use the same market rhythm:
	// crypto uses 15m–4h structures with ATR(1h), while equity tokens with
	// daily data use 4h–1d structures with ATR(1d). Mixing 15m pivots with a
	// daily buffer produced structurally meaningless, over-wide bstock stops.
	bufTF := "1h"
	minStructureTF, maxStructureTF := 15*time.Minute, 4*time.Hour
	if market.IsBStockSymbol(sig.Symbol) {
		bufTF = "1d"
		minStructureTF, maxStructureTF = 4*time.Hour, 24*time.Hour
	}
	t1h := sig.Timeframes[bufTF]
	if t1h == nil || t1h.ATRPct <= 0 {
		return 0, 0, "STOP_PLAN_NO_STRUCTURE"
	}
	bufMult := methodStopBufferLong
	if !isLong {
		bufMult = methodStopBufferShort
	}
	bufPrice := bufMult * t1h.ATRPct / 100 * entry

	// Stop-side structure levels, NEAR→FAR from the entry, ≥15m only,
	// deduped at the same 0.05% tolerance as the S/R arrays.
	var levels []float64
	for name, tf := range sig.Timeframes {
		d := tfDuration(name)
		if tf == nil || d < minStructureTF || d > maxStructureTF {
			continue
		}
		src := tf.StructuralSupport
		if src == nil {
			src = tf.Support
		}
		if !isLong {
			src = tf.StructuralResistance
			if src == nil {
				src = tf.Resistance
			}
		}
		for _, l := range src {
			if l <= 0 {
				continue
			}
			if (isLong && l < entry) || (!isLong && l > entry) {
				if !srcIsStructural(tf, isLong) && tf.BOLLSourced[l] {
					continue // dynamic bands are not valid methodology stops
				}
				levels = append(levels, l)
			}
		}
	}
	if isLong {
		sort.Sort(sort.Reverse(sort.Float64Slice(levels))) // closest-to-entry first
	} else {
		sort.Float64s(levels) // closest-to-entry first
	}
	var uniq []float64
	for _, l := range levels {
		if !inList(uniq, l, entry) {
			uniq = append(uniq, l)
		}
	}
	if len(uniq) == 0 {
		return 0, 0, "STOP_PLAN_NO_STRUCTURE"
	}

	// Cap: max(2×ATR(4h), 8%) — same wide-of-the-two as the executor.
	// NOTE the deliberate ATR-timescale asymmetry of the band: the FLOOR
	// (stopFloorPct, 1.5×ATR(1h)) uses the 1h scale because identifying
	// noise-tight stops needs the fine-grained yardstick; the CAP uses the
	// 4h scale because a ceiling must be STABLE — a 1h ATR spike must not
	// blow the maximum stop width open mid-cycle. Not a typo (09-21 audit).
	// Equity tokens price the whole band off the DAILY scale instead (user
	// directive 2026-09-25): floor 1.5×ATR(1d), buffer ×ATR(1d), cap
	// 2×ATR(1d) with the 8% minimum retained.
	capPct := 8.0
	capTF := "4h"
	if market.IsBStockSymbol(sig.Symbol) {
		capTF = "1d"
	}
	if t4h := sig.Timeframes[capTF]; t4h != nil && t4h.ATRPct > 0 {
		if c := 2 * t4h.ATRPct; c > capPct {
			capPct = c
		}
	}

	for _, l := range uniq {
		var sp float64
		if isLong {
			sp = l - bufPrice
		} else {
			sp = l + bufPrice
		}
		d := math.Abs(sp-entry) / entry * 100
		if d < floorPct {
			continue // too tight — step out to the next structure
		}
		if d > capPct {
			break // farther structures only widen — nothing lands inside the band
		}
		return sp, d, ""
	}
	return 0, 0, "STOP_PLAN_OUT_OF_BAND"
}

// MarketExceptionMinScore is the directional_score bar the prompt has always
// quoted for the breakout-chase market exception (condition ④). It lives here
// so the gate, the executor and the prompt text render ONE number.
const MarketExceptionMinScore = 80

// UnprotectedStopWorstCasePct is the worst-case stop distance assumed when
// accounting a position with no SL on the exchange against the account
// risk-exposure cap — the stop band's own floor cap (max(2×ATR(4h), 8%))
// bottoms out here, bounding what an unprotected position can lose before
// the watchdog re-arms protection. Single source for kernel prompt text and
// the trader's exposure gate (QUANT_REVIEW_2026-09-22 D2).
const UnprotectedStopWorstCasePct = 8.0

// marketExceptionEvidence reports whether the direction-matched market-order
// exception carries program-side evidence: breakout confirmed with volume AND
// OI confirmation AND |directional_score| ≥ MarketExceptionMinScore in the
// trade's favor (exception one, six conditions ①②③④ — ⑤ conflict and ⑥
// loss-streak are separate gate codes), the 15m upper-band ride (exception
// two, longs), the lower-band ride (exception three, shorts).
//
// effectiveScore is the SIDE-ADJUSTED directional score (the sentiment
// deweight already applied for longs — user design 2026-10-01).
//
// 09-19 audit: with the limit anchor suppressed these are the ONLY entry
// paths — when none holds, the direction is unexecutable and must FAIL
// CLOSED (ZEC long shipped allowed=true with limit_buy_price=0,
// bb_ride=false, breakout=below: the model had to choose between breaking
// the exception rules and fighting the gate).
//
// 2026-09-22 (QUANT_REVIEW B1): direction-matched and score-gated. Previously
// a long bb_ride could evidence a SHORT market open and the breakout leg
// ignored the score the prompt demanded — the exception was prompt-advisory.
// The kernel verdict lands on DirectionGate.MarketException, which the
// trader enforces at the open dispatch.
func marketExceptionEvidence(sig *SymbolSignal, isLong bool, effectiveScore int) bool {
	// A chase is permitted only after two closed 1h evaluations agree on a
	// direction-matched trend regime. RANGE/CHOP/UNKNOWN fail closed; ordinary
	// structure-based limit entries continue through their existing gates.
	if !marketRegimeAllowsException(sig, isLong) {
		return false
	}
	if isLong {
		if sig.BBRide != nil && sig.BBRide.Ride {
			return fundingAllowsMarketException(sig, isLong)
		}
	} else {
		if sig.ShortRide != nil && sig.ShortRide.Ride {
			return fundingAllowsMarketException(sig, isLong)
		}
	}
	if sig.Breakout != nil && sig.Breakout.Status == "confirmed" &&
		sig.Breakout.VolumeConfirm && sig.Breakout.OIConfirm &&
		breakoutDirectionMatches(sig, isLong) &&
		directionScoreAtLeastScore(effectiveScore, isLong, MarketExceptionMinScore) {
		return fundingAllowsMarketException(sig, isLong)
	}
	return false
}

// MarketExceptionMaxFundingAnnPct: a market-order exception chasing into a
// band-ride/breakout is exactly where crowded-leverage squeezes bite — the
// 2026-09-27 external review's cheap structural filter. Funding ANNUALIZED
// beyond this on the CROWDED side (positive = longs crowded, blocks longs;
// negative = shorts crowded, blocks shorts) downgrades the exception: the
// setup must take the limit-anchor path instead of paying top-of-book at the
// most squeeze-prone price.
const MarketExceptionMaxFundingAnnPct = 50.0

// fundingAllowsMarketException reports whether the crowd-side funding is
// cool enough for a market-order chase. UNKNOWN → false (fail-closed: no
// funding data ⇒ no market exception).
func fundingAllowsMarketException(sig *SymbolSignal, isLong bool) bool {
	if sig.Derivatives == nil || sig.Derivatives.FundingAnnualizedPct == nil {
		return false
	}
	f := *sig.Derivatives.FundingAnnualizedPct
	if isLong {
		return f <= MarketExceptionMaxFundingAnnPct
	}
	return f >= -MarketExceptionMaxFundingAnnPct
}

// breakoutDirectionMatches: a confirmed BREAKOUT is a long exception, a
// confirmed BREAKDOWN a short one — the other side may not borrow it.
func breakoutDirectionMatches(sig *SymbolSignal, isLong bool) bool {
	if isLong {
		return sig.Breakout.Direction != "breakdown"
	}
	return sig.Breakout.Direction == "breakdown"
}

// buildLongPullbackPlan returns the breakout-retest entry geometry for
// LONGS when the 1h breakout state machine is in a long-eligible window:
// status "confirmed" (price beyond the level) or "retest_hold" (at the level
// and holding). The retest anchor activates only under the strategy switch
// AND with the level still below the live price; nil = the path is inactive
// and the legacy offset anchor stands (evidence badges still attach whenever
// the breakout is long-directional).
func buildLongPullbackPlan(sig *SymbolSignal, data *market.Data, opt SignalOptions) *LongPullbackPlan {
	b := sig.Breakout
	if b == nil || b.Level <= 0 || b.Direction == "breakdown" || data.CurrentPrice <= 0 {
		return nil
	}
	p := &LongPullbackPlan{
		Level:         b.Level,
		Status:        b.Status,
		VolumeConfirm: b.VolumeConfirm,
		OIConfirm:     b.OIConfirm,
	}
	if sig.Derivatives != nil && sig.Derivatives.FundingAnnualizedPct != nil {
		ok := *sig.Derivatives.FundingAnnualizedPct <= MarketExceptionMaxFundingAnnPct
		p.FundingOK = &ok
	}
	if t4h := sig.Timeframes["4h"]; t4h != nil && t4h.EMAFast != nil && *t4h.EMAFast > 0 {
		d := (data.CurrentPrice - *t4h.EMAFast) / *t4h.EMAFast * 100
		p.EMA20Dist4hPct = &d
	}
	if opt.LongPullbackEntry && (b.Status == "confirmed" || b.Status == "retest_hold") &&
		b.Level < data.CurrentPrice {
		p.Active = true
		p.Entry = b.Level
		p.ChaseDistPct = (data.CurrentPrice - b.Level) / b.Level * 100
	}
	return p
}

// ema20Stretch4hPct: live price above the 4h EMA20 in %. Missing evidence →
// 0 (the gate must not fire on absent data).
func ema20Stretch4hPct(sig *SymbolSignal) float64 {
	if sig.LongPullback != nil && sig.LongPullback.EMA20Dist4hPct != nil {
		return *sig.LongPullback.EMA20Dist4hPct
	}
	if t4h := sig.Timeframes["4h"]; t4h != nil && t4h.EMAFast != nil && *t4h.EMAFast > 0 && sig.Price > 0 {
		return (sig.Price - *t4h.EMAFast) / *t4h.EMAFast * 100
	}
	return 0
}

// antiChaseCodes evaluates the four symmetric anti-chase hard gates (Opus
// analysis of 273 live trades, 2026-10-11): entries far extended from the 1h
// EMA20 in the trade direction (CHASE_EMA20_1H), far on the wrong side of it
// (COUNTER_EMA20_1H, knife-catching / fading a pump), into an exhausted
// directional 1h RSI (RSI1H_EXHAUSTED) or after a big 4h move in the trade
// direction (CHASE_MOVE4H). Distances are measured from entry (the limit
// anchor or live price), so a genuine retest anchor near the EMA passes
// naturally — no pullback exemption. Equity tokens (bstock) are exempt (a
// different volatility scale); missing evidence fails OPEN.
func antiChaseCodes(sig *SymbolSignal, entry float64, isLong bool, opt SignalOptions) []string {
	if market.IsBStockSymbol(sig.Symbol) {
		return nil
	}
	var codes []string
	t1h := sig.Timeframes["1h"]
	if t1h != nil && t1h.EMAFast != nil && *t1h.EMAFast > 0 && entry > 0 {
		ema := *t1h.EMAFast
		// Signed distance of entry from the 1h EMA20, positive = in the trade direction.
		d := (entry - ema) / ema * 100
		if !isLong {
			d = -d
		}
		// Retest-anchor basis (review 2026-10-11): with market-default entry
		// g.EntryPrice is the LIVE price, but while the long pullback plan is
		// active the long's real entry is the retest anchor — measure the
		// chase distance from min(entry, anchor) so a designed "wait for the
		// retest" order is not blocked for an extension it will not buy.
		// LONG chase only; COUNTER keeps the plain entry, shorts unchanged.
		chaseD := d
		if isLong && sig.LongPullback != nil && sig.LongPullback.Active &&
			sig.LongPullback.Entry > 0 && sig.LongPullback.Entry < entry {
			chaseD = (sig.LongPullback.Entry - ema) / ema * 100
		}
		if opt.ChaseMaxEMA20_1hPct > 0 && chaseD > opt.ChaseMaxEMA20_1hPct {
			codes = append(codes, fmt.Sprintf("CHASE_EMA20_1H_%.1f_GT_%.0f", chaseD, opt.ChaseMaxEMA20_1hPct))
		}
		if opt.CounterMaxEMA20_1hPct > 0 && -d > opt.CounterMaxEMA20_1hPct {
			codes = append(codes, fmt.Sprintf("COUNTER_EMA20_1H_%.1f_GT_%.0f", -d, opt.CounterMaxEMA20_1hPct))
		}
	}
	if opt.EntryRSI1hMax > 0 && t1h != nil && t1h.RSI14 != nil {
		rsi := *t1h.RSI14
		if (isLong && rsi >= opt.EntryRSI1hMax) || (!isLong && rsi <= 100-opt.EntryRSI1hMax) {
			codes = append(codes, fmt.Sprintf("RSI1H_EXHAUSTED_%.0f", rsi))
		}
	}
	if opt.ChaseMaxMove4hPct > 0 && sig.PriceChange4hPct != 0 {
		m := sig.PriceChange4hPct
		if !isLong {
			m = -m
		}
		if m > opt.ChaseMaxMove4hPct {
			codes = append(codes, fmt.Sprintf("CHASE_MOVE4H_%.1f_GT_%.0f", m, opt.ChaseMaxMove4hPct))
		}
	}
	return codes
}

// IsAntiChaseCode reports whether a hard-gate code is one of the four
// anti-chase codes. The trader's market-open branch refuses these (a market
// order buys at the live price the gate evaluated); they are deliberately NOT
// absolute bans for resting limits, which buy at their anchor.
func IsAntiChaseCode(code string) bool {
	for _, p := range []string{"CHASE_EMA20_1H_", "COUNTER_EMA20_1H_", "RSI1H_EXHAUSTED_", "CHASE_MOVE4H_"} {
		if strings.HasPrefix(code, p) {
			return true
		}
	}
	return false
}

// btc4hShape reads the TRUE 4h BTC closes through the SHARED classifier
// (breakout.BTC4hRegime — same EMA20/50+RSI definition the short-scan
// haircut and the scheduler regime label use, review 2026-10-04 #3: the
// 1h-aggregate version had phase-arbitrary buckets that drifted hourly, and
// an EMA-only kernel variant could contradict the scanners on the same bar)
// and reports the downtrend verdict and BTC's 24h return (6 bars). known =
// ≥60 4h closes (EMA50 converged).
func btc4hShape(closes4h []float64) (down bool, ret24 float64, known bool) {
	n := len(closes4h)
	if n < 60 {
		return false, 0, false
	}
	_, _, down, known = breakout.BTC4hRegime(closes4h)
	if !known {
		return false, 0, false
	}
	ret24 = (closes4h[n-1] - closes4h[n-7]) / closes4h[n-7] * 100
	return down, ret24, true
}

// emaOf computes the EMA over a float slice (last value).
func emaOf(vals []float64, n int) float64 {
	if len(vals) == 0 {
		return 0
	}
	k := 2.0 / float64(n+1)
	e := vals[0]
	for _, v := range vals[1:] {
		e = v*k + e*(1-k)
	}
	return e
}

// btcWeakLongCodes: the market-level BTC filter for altcoin LONGS (user
// design 2026-10-01): BTC 4h must not be in a downtrend, and the coin must
// be at least as strong as BTC over the last 24h. Missing data passes — a
// data gap must not freeze every altcoin long.
func btcWeakLongCodes(sig *SymbolSignal, opt SignalOptions) []string {
	var codes []string
	down, closedRet24, known := btc4hShape(opt.BtcTrendCloses)
	if !known {
		return codes
	}
	if down {
		codes = append(codes, "BTC_4H_DOWNTREND")
	}
	// Compare like with like (review 2026-10-06 P2-5): the coin side is a LIVE
	// rolling-24h ticker — BTC's 24h now comes from the SAME fapi ticker batch
	// instead of a window truncated at the last 4h settlement (0-4h stale).
	// Direction of the residual error before this fix: BTC pumping → weak-mid
	// coins WRONGLY PASSED; BTC dumping → strong-mid coins WRONGLY BANNED.
	// Missing ticker falls back to the closed-bar computation (old behavior).
	ret24, ok := binanceBTC24hLivePct()
	if !ok {
		ret24 = closedRet24
	}
	if sig.Derivatives != nil && sig.Derivatives.PriceChange24hLivePct != nil {
		coin24 := *sig.Derivatives.PriceChange24hLivePct
		if coin24 < ret24 {
			codes = append(codes, fmt.Sprintf("BTC_WEAK_LONG_%.1f_VS_%.1f", coin24, ret24))
		}
	}
	return codes
}

// sentimentGreedy: the crypto Fear&Greed index reads at or above the
// threshold (missing index → not greedy, no deweight). Lives engine-side —
// ComputeSymbolSignals stays pure so the gates stay unit-testable.
func sentimentGreedy(threshold int) bool {
	if threshold <= 0 {
		return false
	}
	// 2026-10-03: Crypto is a POINTER and stays nil while its fetch is in
	// flight or has failed — the unconditional deref crashed the cycle.
	if s := market.GetMarketSentiment(); s != nil && s.Crypto != nil {
		return s.Crypto.Value >= threshold
	}
	return false
}

func directionScoreAtLeastScore(score int, isLong bool, min int) bool {
	if isLong {
		return score >= min
	}
	return score <= -min
}

// negativeEdgeAligned: the trend TF and regime TF EMA directions must BOTH
// match the trade side (fast>slow = long side, fast<slow = short side) —
// pullback/rally quadrants keep their EMA side, so alignment follows the
// slope, not the price-vs-fast label. Missing EMA evidence = not demonstrably
// aligned (fail-closed: the NEGATIVE_EDGE prose demands exceptional evidence,
// and absent evidence cannot demonstrate it).
func negativeEdgeAligned(sig *SymbolSignal, isLong bool) bool {
	trendTF, regimeTF := sig.RoleTFs.TrendTF, sig.RoleTFs.RegimeTF
	if trendTF == "" {
		trendTF = "1h"
	}
	if regimeTF == "" {
		regimeTF = "4h"
	}
	for _, name := range []string{trendTF, regimeTF} {
		t := sig.Timeframes[name]
		if t == nil || t.EMAFast == nil || t.EMASlow == nil {
			return false
		}
		longSide := *t.EMAFast > *t.EMASlow
		if longSide != isLong {
			return false
		}
	}
	return true
}

// computeHardEntryGate evaluates, per direction, every program-decidable
// open blocker — micro-trend (when the timing gate is enabled, mirroring the
// executor's config switch), limit-anchor suppression (fail-closed unless a
// market exception carries program evidence), structural RR at the
// METHODOLOGY stop (stop_plan), data sufficiency, min-size dead zone,
// loss-streak ban, stock weekend. allowed = nothing failed.
func computeHardEntryGate(sig *SymbolSignal, opt SignalOptions) *HardEntryGate {
	suppressAnchorsAgainstRegimeLine(sig, opt.EntryTimingGate)
	floorPct := stopFloorPct(sig, opt.SLMinATRMult)
	bstockDailyReady := true
	if market.IsBStockSymbol(sig.Symbol) {
		td := sig.Timeframes["1d"]
		bstockDailyReady = td != nil && td.ATRPct > 0
	}
	evaluate := func(isLong bool) *DirectionGate {
		g := &DirectionGate{Failed: []string{}}
		// Effective directional score for THIS side (user design 2026-10-01):
		// while the crypto FNG reads at/above the greed threshold, a LONG
		// carries a score penalty at every gate read (CONSENSUS_OPPOSED /
		// NEG_EDGE_SCORE / breakout market exception) — the greed backdrop
		// de-ranks chasing without blocking it outright.
		effectiveScore := 0
		if sig.SignalConflict != nil {
			effectiveScore = sig.SignalConflict.DirectionalScore
		}
		if isLong && opt.SentimentLongDeweightPts > 0 && opt.SentimentLongDeweightArmed {
			effectiveScore -= opt.SentimentLongDeweightPts
		}
		anchor := sig.LimitBuyPrice
		if !isLong {
			anchor = sig.LimitSellPrice
		}
		g.LimitAllowed = anchor > 0
		if opt.LimitEntryEnabled && anchor > 0 {
			g.EntryPrice, g.EntryBasis = anchor, "limit_anchor"
		} else {
			g.EntryPrice, g.EntryBasis = sig.Price, "live_price"
		}
		g.StopFloorPct = round2(floorPct)
		add := func(code string) { g.Failed = append(g.Failed, code) }
		if !bstockDailyReady {
			add("BSTOCK_DAILY_DATA_UNAVAILABLE")
		}
		if floorPct > 0 {
			if price, dist, code := methodStopPlan(sig, g.EntryPrice, floorPct, isLong); code != "" {
				add(code)
			} else {
				g.StopPlanPrice = price
				g.StopPlanPct = round2(dist)
				g.StopPlanSource = "structure"
				g.RR = scanRRForSymbol(sig, g.EntryPrice, g.EntryBasis, dist, price, isLong, opt.MinRR)
				// WIDE_STOP (user directive 2026-10-02): a stop plan beyond
				// the cap means the coin's volatility structure doesn't fit
				// the sizing geometry — USUSDT's 16.8% stop died in 4 minutes
				// with risk capped at 2%. BOTH directions: a wide stop is a
				// position-geometry problem, not a directional one.
				//
				// bstock exemption (2026-10-03 review): equity tokens price
				// the whole stop band off the DAILY scale — floor
				// 1.5×ATR(1d), cap max(2×ATR(1d), 8%) — so a band-legal stop
				// can legitimately sit at 10-16% on a high-ATR ticker. Their
				// own band governs; the absolute 10% cap applies to crypto.
				if opt.MaxStopDistancePct > 0 && !market.IsBStockSymbol(sig.Symbol) &&
					g.StopPlanPct > opt.MaxStopDistancePct {
					add(fmt.Sprintf("WIDE_STOP_%.1f_GT_%.0f", g.StopPlanPct, opt.MaxStopDistancePct))
				}
			}
		}
		if opt.EntryTimingGate && sig.ExecutionFilter != nil {
			if sig.RegimeLine != nil && RegimeLineApplies(isLong, sig.ExecutionFilter.MicroTrend) &&
				RegimeLineBreached(isLong, sig.Price, sig.RegimeLine.EMA50) {
				add("REGIME_LINE_BROKEN")
			}
			if isLong && !sig.ExecutionFilter.LongAllowed {
				add("MICRO_TREND_NOT_LONG")
			}
			if !isLong && !sig.ExecutionFilter.ShortAllowed {
				add("MICRO_TREND_NOT_SHORT")
			}
		}
		// Extended-pump long guard (2026-09-20): a hot coin's EMA-up label
		// lags the round-trip — longs into the first pullback of a vertical
		// pump need the dip CONFIRMED, else the direction is blocked.
		if isLong && sig.PumpGuard != nil && sig.PumpGuard.Extended && !sig.PumpGuard.Confirmed {
			add("EXTENDED_PUMP_UNCONFIRMED")
		}
		// 09-19 audit 三 (second round): the LimitEntryEnabled precondition
		// made this fail-closed rule DEAD on market-default strategies whose
		// strategy contract still says 默认限价 (WLFI short shipped
		// allowed=true / failed=[] with every path dead). The anchor is
		// suppressed by the breathing rule regardless of the mode flag, and
		// the executor's supply-zone gate rejects those fills too — block it.
		if !g.LimitAllowed && !marketExceptionEvidence(sig, isLong, effectiveScore) {
			add("LIMIT_ANCHOR_SUPPRESSED")
		}
		// B1 (QUANT_REVIEW 2026-09-22): the market-order exception verdict is
		// program-decided here and enforced at the trader's open dispatch —
		// previously it existed only as prompt prose, so a market open without
		// any exception evidence went straight to the exchange.
		g.MarketException = marketExceptionEvidence(sig, isLong, effectiveScore)
		// Long-side entry discipline (user design 2026-10-01):
		// EMA20 stretch cap — a CHASE entry far above the 4h EMA20 is the
		// exact "买强势" slice the 177-long attribution blamed (entries >5%
		// above EMA20 net −8.55U; 0-2% −8.06U at 28% win). The retest-anchor
		// path is EXEMPT: waiting at the broken level is the designed
		// alternative, and by the time it fills the stretch has closed.
		if isLong && opt.LongMaxEMA20DistPct > 0 &&
			(sig.LongPullback == nil || !sig.LongPullback.Active) {
			if d := ema20Stretch4hPct(sig); d > opt.LongMaxEMA20DistPct {
				add(fmt.Sprintf("EMA20_STRETCH_%.1f_GT_%.0f", d, opt.LongMaxEMA20DistPct))
			}
		}
		for _, code := range antiChaseCodes(sig, g.EntryPrice, isLong, opt) {
			add(code)
		}
		// BTC relative strength (market-level filter): BTC 4h must not be in
		// a downtrend and the coin must be at least as strong as BTC over
		// 24h — chasing laggards in a decaying market was the second long
		// loss pool (deep-pullback knife catches, 39 trades −13.56U).
		if isLong && opt.BTCFilterLong {
			for _, code := range btcWeakLongCodes(sig, opt) {
				add(code)
			}
		}
		// Short-side market filter (OPT-IN — BTCFilterShort, nil config =
		// off): the shared BTC 4h classifier's strongBull read blocks
		// altcoin shorts market-wide. The scan side already haircuts shorts
		// ×0.85 off the SAME classifier; this is the hard pause switch.
		if !isLong && opt.BTCFilterShort {
			// The hard pause is OPT-IN: the user explicitly asked "no shorts
			// while BTC reads strongBull". Missing BTC data cannot demonstrate
			// "not strong bull" — house convention (funding_rollover /
			// VENDOR_DIVERGENCE / NEG_EDGE_RR): absent evidence = 按不满足处理.
			// This also aligns the kernel gate with the executor's pending
			// recompute, which already fail-closes on the same state (P2-2:
			// the two layers used to disagree on identical input). The LONG
			// side keeps its documented fail-open — btcWeakLongCodes says a
			// data gap must not freeze every altcoin long.
			if _, strongBull, _, known := breakout.BTC4hRegime(opt.BtcTrendCloses); known {
				if strongBull {
					add("BTC_4H_STRONGBULL")
				}
			} else {
				add("BTC_REGIME_UNKNOWN")
			}
		}
		// Short top-confirmation gate (playbook 默认姿态, was prompt-only —
		// review 2026-10-04 C3): a short_scan candidate needs the scanner's
		// topping confirmation (顶背离/假突破 from the scanner, or live funding_rollover.detected; 破EMA20 is NOT one — review 2026-10-07). Candidates
		// WITHOUT short_scan evidence (piggy breakdown, model-initiated) are
		// not gated here — the 15m MICRO_TREND gate still applies to them.
		// review 2026-10-07 S3: the scanner's Confirmed covers divergence /
		// fake breakout only; the funding-rollover confirmation is the live
		// derivatives.funding_rollover.detected (strategy crowding threshold),
		// per the prompt rule — either one satisfies the gate.
		if !isLong && opt.ShortTopConfirmGate && opt.ShortScanConfirmed != nil && !*opt.ShortScanConfirmed {
			liveRollover := sig.Derivatives != nil && sig.Derivatives.FundingRollover != nil &&
				sig.Derivatives.FundingRollover.Detected
			if !liveRollover {
				add("SHORT_TOP_CONFIRM_MISSING")
			}
		}
		if g.RR != nil && opt.MinRR > 0 && !g.RR.Usable {
			add(fmt.Sprintf("RR_MAX_%.2f", g.RR.BestRR))
		}
		if sig.DataQuality != nil && !sig.DataQuality.Sufficient {
			add("DATA_INSUFFICIENT")
		}
		if sig.MinSize != nil && !sig.MinSize.Feasible {
			add("MIN_SIZE_DEAD_ZONE")
		}
		if sig.LossStreak != nil {
			add("LOSS_STREAK_BANNED")
		}
		if opt.StockWeekendBlock {
			add("STOCK_WEEKEND")
		}
		// Poor-symbol history gate (09-19 audit 七): ≥5 closed trades with
		// <35% win rate on THIS symbol — the prose warning ("连亏的币把机会
		// 让给趋势健康的标的") was ignored, and the one tradeable coin kept
		// being the worst performer (ZEC: 7 trades, 29%, −1.61U).
		if opt.TraderHistory != nil && opt.TraderHistory.ClosedTrades >= 5 && opt.TraderHistory.WinRatePct < 35 {
			add("POOR_HISTORY")
		}
		// 09-19 audit 六-③: opening AGAINST a ≥50 directional consensus is the
		// counter-trend slice of the loss ledger — zero-cost, score-based.
		// effectiveScore carries the sentiment deweight for longs.
		if sig.SignalConflict != nil {
			sc := effectiveScore
			if (isLong && sc <= -50) || (!isLong && sc >= 50) {
				add(fmt.Sprintf("CONSENSUS_OPPOSED_%d", sc))
			}
		}
		// NEGATIVE_EDGE health gate (user review 2026-09-27 #4): while the
		// strategy's rolling stats have PF<0.9, "only exceptional setups" is
		// a program verdict, not prose — every condition lands its own code
		// so the wait reason is citable and gate_shadow_blocks can calibrate
		// the thresholds later.
		if opt.NegativeEdge {
			// effectiveScore carries the sentiment deweight for longs —
			// greed raises the evidence bar for longs symmetrically.
			// Signed comparison (review 2026-10-04 C4): a LONG needs score
			// ≥ min, a SHORT score ≤ −min — the old math.Abs let a long
			// pass on the strength of |−70| of OPPOSING evidence whenever
			// the configured min sat below the CONSENSUS_OPPOSED ±50 net.
			score := effectiveScore
			minScore := int(opt.NegativeEdgeMinScore)
			if minScore > 0 && !directionScoreAtLeastScore(score, isLong, minScore) {
				add(fmt.Sprintf("NEG_EDGE_SCORE_%+d_LT_%.0f", score, opt.NegativeEdgeMinScore))
			}
			if !negativeEdgeAligned(sig, isLong) {
				add("NEG_EDGE_TREND_MISALIGNED")
			}
			// NEG_EDGE_RR: rr_scan absent (SLMinATRMult=0 disables the
			// stop-plan machinery) leaves RR nil ⇒ the condition is UNKNOWN
			// ⇒ house convention (funding_rollover/vendor precedent):
			// 按不满足处理 → fires NEG_EDGE_RR_0.00. Fail-closed is
			// deliberate — NEG_EDGE demands exceptional EVIDENCE, and absent
			// evidence cannot demonstrate it (2026-10-03: considered making
			// this lenient, reverted for convention consistency; strategies
			// running SLMinATRMult=0 + NEG_EDGE will all-wait by design).
			if opt.NegativeEdgeMinRR > 0 {
				firstRR := 0.0
				if g.RR != nil {
					firstRR = g.RR.FirstTargetRR
				}
				if firstRR < opt.NegativeEdgeMinRR {
					add(fmt.Sprintf("NEG_EDGE_RR_%.2f_LT_%.1f", firstRR, opt.NegativeEdgeMinRR))
				}
			}
			if opt.NegativeEdgeBlockLosing && sig.TraderHistory != nil &&
				sig.TraderHistory.ClosedTrades >= 2 && sig.TraderHistory.RealizedPnL < 0 {
				add(fmt.Sprintf("NEG_EDGE_LOSING_SYMBOL_%+.2fU", sig.TraderHistory.RealizedPnL))
			}
		}
		if opt.MaxVendorDivergencePct > 0 {
			// Same convention as funding_rollover: field absent = UNKNOWN =
			// 按不满足处理 (09-19 audit 六: the ONE tradable coin silently
			// lacked the divergence measurement). Absent is now rare — the
			// market layer measures ~0 for a fresh vendor too.
			if sig.DataFreshness == nil || sig.DataFreshness.VendorDivergencePct == nil {
				add("VENDOR_DIVERGENCE_UNKNOWN")
			} else if math.Abs(*sig.DataFreshness.VendorDivergencePct) > opt.MaxVendorDivergencePct {
				add(fmt.Sprintf("VENDOR_DIVERGENCE_%.2f", *sig.DataFreshness.VendorDivergencePct))
			}
		}
		g.Allowed = len(g.Failed) == 0
		return g
	}
	return &HardEntryGate{Long: evaluate(true), Short: evaluate(false)}
}

// scanRR walks crypto structural targets over the 15m-4h horizon. The
// symbol-aware wrapper below switches bstock targets to 4h-1d so TP and SL
// represent the same holding rhythm.
func scanRR(entry float64, basis string, stopPct float64, stopPrice float64, tfs map[string]*TFSignal, isLong bool, minRR float64) *RRScan {
	return scanRRWindow(entry, basis, stopPct, stopPrice, tfs, isLong, minRR, 15*time.Minute, 4*time.Hour)
}

func scanRRForSymbol(sig *SymbolSignal, entry float64, basis string, stopPct float64, stopPrice float64, isLong bool, minRR float64) *RRScan {
	var out *RRScan
	if market.IsBStockSymbol(sig.Symbol) {
		out = scanRRWindowWithCost(entry, basis, stopPct, stopPrice, sig.Timeframes, isLong, minRR, 4*time.Hour, 24*time.Hour, sig.rrCostBps)
	} else {
		out = scanRRWindowWithCost(entry, basis, stopPct, stopPrice, sig.Timeframes, isLong, minRR, 15*time.Minute, 4*time.Hour, sig.rrCostBps)
	}
	// Touch counts (user directive 09-29): neutral "price has been here"
	// evidence per menu option — closed bars whose range reached the level
	// inside the window. Absent series simply leave the fields at zero.
	if len(out.Options) > 0 && len(sig.touchHighs) > 0 {
		for i := range out.Options {
			out.Options[i].TouchCount = touchCount(sig.touchHighs, sig.touchLows, out.Options[i].Level, isLong)
			out.Options[i].TouchWindowDays = sig.touchWindowDays
		}
	}
	return out
}

// touchCount counts the closed bars whose range reached the target level —
// for a long target any bar high at/above it, for a short target any bar low
// at/below it. Evidence of reachability, deliberately NOT labeled a
// probability: a well-touched level is reachable AND contested.
func touchCount(highs, lows []float64, level float64, isLongTarget bool) int {
	if level <= 0 {
		return 0
	}
	n := 0
	for i := range highs {
		if isLongTarget {
			if highs[i] >= level {
				n++
			}
		} else if lows[i] > 0 && lows[i] <= level {
			n++
		}
	}
	return n
}

// scanRRWindow walks EVERY structural target on the take-profit side inside
// the supplied timeframe window, near→far, deduped within 0.05% (same
// tolerance as the S/R builder), computing each target's RR at the adopted
// methodology stop.
// first_rr_ge_target is the
// rule-mandated pick (nearest qualifying level); best_rr is the definitive
// upper bound used to declare the RR gate structurally failed.
func scanRRWindow(entry float64, basis string, stopPct float64, stopPrice float64, tfs map[string]*TFSignal, isLong bool, minRR float64, minTF, maxTF time.Duration) *RRScan {
	return scanRRWindowWithCost(entry, basis, stopPct, stopPrice, tfs, isLong, minRR, minTF, maxTF, 0)
}
func scanRRWindowWithCost(entry float64, basis string, stopPct float64, stopPrice float64, tfs map[string]*TFSignal, isLong bool, minRR float64, minTF, maxTF time.Duration, costBps float64) *RRScan {
	type tfLevels struct {
		name string
		tf   *TFSignal
	}
	var pairs []tfLevels
	for name, tf := range tfs {
		if tf == nil || tfDuration(name) < minTF || tfDuration(name) > maxTF {
			continue
		}
		pairs = append(pairs, tfLevels{name, tf})
	}
	var targets []float64
	// BOLL-sourced targets: visible for best_rr, but NEVER first_rr_ge_target —
	// a band value decays with its window, so anchoring the rule-mandated TP
	// to it lets the exit drift after placement (QUANT_REVIEW_2026-09-22 C1).
	bollT := map[float64]bool{}
	structuralT := map[float64]bool{}
	for _, p := range pairs {
		src := p.tf.StructuralResistance
		displaySrc := p.tf.Resistance
		if !isLong {
			src = p.tf.StructuralSupport
			displaySrc = p.tf.Support
		}
		// Older/manual signals may not carry the internal full pivot set.
		// Fall back to the displayed levels in that case.
		if src == nil {
			src = displaySrc
		}
		for _, l := range src {
			if l <= 0 {
				continue
			}
			if isLong && l > entry {
				targets = append(targets, l)
				if srcIsStructural(p.tf, isLong) || !p.tf.BOLLSourced[l] {
					structuralT[l] = true
				} else {
					bollT[l] = true
				}
			}
			if !isLong && l < entry {
				targets = append(targets, l)
				if srcIsStructural(p.tf, isLong) || !p.tf.BOLLSourced[l] {
					structuralT[l] = true
				} else {
					bollT[l] = true
				}
			}
		}
		// Bollinger levels remain context for best_rr, but never qualify as
		// an adopted target. If a band exactly overlaps a real swing pivot,
		// the real structural level takes precedence below.
		if srcIsStructural(p.tf, isLong) {
			for _, l := range displaySrc {
				if !p.tf.BOLLSourced[l] || l <= 0 {
					continue
				}
				if (isLong && l > entry) || (!isLong && l < entry) {
					targets = append(targets, l)
					bollT[l] = true
				}
			}
		}
	}
	for l := range structuralT {
		delete(bollT, l)
	}
	if isLong {
		sort.Float64s(targets) // ascending = near→far above
	} else {
		sort.Sort(sort.Reverse(sort.Float64Slice(targets))) // descending = near→far below
	}
	var uniq []float64
	for _, t := range targets {
		if !inList(uniq, t, entry) {
			uniq = append(uniq, t)
		}
	}
	direction := "long"
	if !isLong {
		direction = "short"
	}
	// 09-19 RR audit: stopPct/stopPrice are the METHODOLOGY stop (structure +
	// buffer, band-clamped — see methodStopPlan), not the noise floor. The
	// gate verdict, the adopted stop_loss and the executor's checkRR all
	// price this same stop, so a passing scan IS a ≥min-RR trade.
	if stopPct <= 0 {
		stopPct = math.Abs(stopPrice-entry) / entry * 100
	}
	out := &RRScan{
		CostBps:         costBps,
		Direction:       direction,
		EntryPrice:      entry,
		EntryBasis:      basis,
		StopDistancePct: round2(stopPct),
		StopPrice:       stopPrice,
		MinRR:           round2(minRR),
		TargetsScanned:  len(uniq),
		Targets:         uniq,
	}
	// Global structure extremes across the SAME target horizon — a TP beyond them has
	// no historical reference (09-19 audit 六: TP 1648.08 vs structure_high 1588).
	maxSH, minSL := 0.0, 0.0
	for _, p := range pairs {
		tf := p.tf
		if tf == nil {
			continue
		}
		if tf.StructureHigh != nil && *tf.StructureHigh > maxSH {
			maxSH = *tf.StructureHigh
		}
		if tf.StructureLow != nil && (minSL == 0 || *tf.StructureLow < minSL) {
			minSL = *tf.StructureLow
		}
	}
	best, bestT := 0.0, 0.0
	type qualTarget struct {
		level float64
		rr    float64
	}
	var qualifying []qualTarget
	for _, t := range uniq {
		dist := (t - entry) / entry * 100
		if !isLong {
			dist = (entry - t) / entry * 100
		}
		costPct := math.Max(0, costBps) / 100
		v := (dist - costPct) / (stopPct + costPct)
		if v > best {
			best, bestT = v, t
		}
		if out.FirstObstacle == 0 && !bollT[t] {
			out.FirstObstacle = t
			out.FirstObstacleRR = round2(v)
		}
		if minRR > 0 && !out.Usable && v >= minRR-1e-9 && !bollT[t] {
			out.FirstRRGeTarget = t
			out.FirstTargetRR = round2(v)
			out.Usable = true
			if isLong && maxSH > 0 && t > maxSH {
				out.FirstTargetBeyondStructure = true
			}
			if !isLong && minSL > 0 && t < minSL {
				out.FirstTargetBeyondStructure = true
			}
		}
		if minRR > 0 && v >= minRR-1e-9 && !bollT[t] {
			qualifying = append(qualifying, qualTarget{level: t, rr: v})
		}
	}
	if len(uniq) > 0 {
		out.BestTarget = bestT
		out.BestRR = round2(best)
	}
	// Menu: near / mid / far among the qualifying (non-BOLL) targets. The
	// model picks an index; the default stays option 1 = the nearest
	// qualifying level (the legacy first_rr_ge_target), so absent or invalid
	// choices degrade to exactly the old behavior.
	if len(qualifying) > 0 {
		idx := []int{0}
		if len(qualifying) >= 3 {
			idx = append(idx, len(qualifying)/2)
		}
		if len(qualifying) >= 2 {
			idx = append(idx, len(qualifying)-1)
		}
		seen := map[int]bool{}
		for _, i := range idx {
			if seen[i] {
				continue
			}
			seen[i] = true
			q := qualifying[i]
			opt := TPOption{Level: q.level, RR: round2(q.rr)}
			if isLong && maxSH > 0 && q.level > maxSH {
				opt.BeyondStructure = true
			}
			if !isLong && minSL > 0 && q.level < minSL {
				opt.BeyondStructure = true
			}
			if len(out.Options) == 0 {
				opt.Default = true
			}
			out.Options = append(out.Options, opt)
		}
	}
	return out
}

// computeBiasBlock names each directional read by its SOURCE (review point
// 3): the scanner's snapshot stance, the market-structure consensus, and the
// execution window — the model must phrase them as different things, never
// as one "strong short evidence".
func computeBiasBlock(sig *SymbolSignal, opt SignalOptions) *BiasBlock {
	b := &BiasBlock{Scanner: "none"}
	if opt.ScannerBias == "short" || opt.ScannerBias == "long" {
		b.Scanner = opt.ScannerBias
	}
	score := 0
	if sig.SignalConflict != nil {
		score = sig.SignalConflict.DirectionalScore
	}
	switch {
	case score >= 20:
		b.Structure = "long"
	case score <= -20:
		b.Structure = "short"
	default:
		b.Structure = "mixed"
	}
	switch {
	case sig.ExecutionFilter == nil:
		b.Execution = "unknown"
	case sig.ExecutionFilter.LongAllowed && sig.ExecutionFilter.ShortAllowed:
		b.Execution = "both"
	case sig.ExecutionFilter.LongAllowed:
		b.Execution = "long_only"
	case sig.ExecutionFilter.ShortAllowed:
		b.Execution = "short_only"
	default:
		b.Execution = "none"
	}
	return b
}

// computeTFSignal scores one timeframe: closed-candle settlement, trend
// classification, swing S/R, and the indicator features.
func computeTFSignal(tf string, tfData *market.TimeframeSeriesData, now time.Time, anchorPrice float64) (*TFSignal, []string) {
	var warns []string
	dur := tfDuration(tf)

	// ── Closed-candle settlement ──
	klines := tfData.Klines
	dropped := 0
	if len(klines) > 0 {
		last := klines[len(klines)-1]
		if time.UnixMilli(last.Time).Add(dur).After(now) {
			// The most recent candle is still forming — indicators and
			// structure are computed on CLOSED candles only.
			klines = klines[:len(klines)-1]
			dropped = 1
		}
	}
	n := len(klines)
	if n < 10 {
		return &TFSignal{
			Timeframe: tf,
			Trend:     "unknown", LastClosedCandle: "unknown",
			Support: []float64{}, Resistance: []float64{},
			SupportDistPct: []float64{}, ResistanceDistPct: []float64{},
			UnclosedDropped: dropped > 0, BarsUsed: n,
		}, []string{fmt.Sprintf("%s: only %d closed bars after settlement", tf, n)}
	}

	c := make([]float64, n)
	for i, k := range klines {
		c[i] = k.Close
	}
	last := klines[n-1]

	sig := &TFSignal{
		Timeframe:        tf,
		LastClose:        last.Close,
		BarsUsed:         n,
		UnclosedDropped:  dropped > 0,
		LastClosedCandle: candleColor(last),
	}

	// ── Return over this timeframe's lookback (min(20, n-1) bars) ──
	lookback := 20
	if n < lookback+1 {
		lookback = n - 1
	}
	if c[n-1-lookback] > 0 {
		sig.ReturnPct = (c[n-1]/c[n-1-lookback] - 1) * 100
	}
	// Make the return window explicit: 20 bars means a different real-world
	// span per timeframe (5h on 15m vs 20 days on 1d).
	sig.ReturnWindowHrs = float64(lookback) * dur.Hours()

	// ── Timeframes with bar duration ≤ 1h report the previous FULL hour's
	// close-over-close change. Purely closed data — deliberately different
	// from the symbol-level rolling price_change_1h_pct, which anchors on the
	// live price. The Summary labels both. ──
	if tfDuration(tf) <= time.Hour {
		bars1h := int(time.Hour / dur)
		if n > bars1h && c[n-1-bars1h] > 0 {
			r := (c[n-1]/c[n-1-bars1h] - 1) * 100
			sig.PrevHourChangePct = &r
		}
	}

	// ── Indicators: self-computed from the CLOSED bars only — independent of
	// the strategy's prompt-display switches. ──
	kb := barsToKlines(klines)
	atr := wilderATR(kb, 14)
	if atr > 0 && last.Close > 0 {
		sig.ATRPct = atr / last.Close * 100
		// Percentile of the current ATR% within this TF's own ATR history —
		// separates "this coin always moves like this" from "unusual vol".
		if series := wilderATRSeries(kb, 14); len(series) > 5 {
			atrPcts := make([]float64, 0, len(series))
			for i, a := range series {
				barIdx := i + 14 // series[i] is the ATR settled at bar i+period
				if barIdx < len(klines) && klines[barIdx].Close > 0 {
					atrPcts = append(atrPcts, a/klines[barIdx].Close*100)
				}
			}
			if len(atrPcts) > 5 {
				rank := 0
				for _, v := range atrPcts {
					if v <= sig.ATRPct {
						rank++
					}
				}
				pct := float64(rank) / float64(len(atrPcts)) * 100
				sig.ATRPercentile = &pct
			}
		}
	}

	if n >= 6 {
		avgLen := 20
		if n-1 < avgLen {
			avgLen = n - 1
		}
		var sum float64
		for _, k := range klines[n-1-avgLen : n-1] {
			sum += k.Volume
		}
		avg := sum / float64(avgLen)
		if avg > 0 {
			v := last.Volume / avg
			sig.VolumeRatio = &v
		}
	}
	// Absolute volume of the last closed bar and its bar-over-bar change.
	sig.Volume = last.Volume
	if n >= 2 && klines[n-2].Volume > 0 {
		sig.VolumeChangePct = (last.Volume/klines[n-2].Volume - 1) * 100
	}

	// Adaptive EMA periods: a 20/50 pair needs 50+ bars — with thinner
	// history fall back to shorter pairs so the trend is never "unknown".
	fastP, slowP := 20, 50
	if n < 50 {
		fastP, slowP = 10, 20
	}
	if n < 25 {
		fastP, slowP = 6, 12
	}
	fast := market.ExportCalculateEMA(kb, fastP)
	slow := market.ExportCalculateEMA(kb, slowP)
	if fast > 0 && slow > 0 {
		sig.EMAFast = &fast
		sig.EMASlow = &slow
	}
	sig.Trend = classifyTrend(c, fast, slow, fast > 0 && slow > 0)

	if macdLine := market.ExportCalculateMACD(kb); macdLine != 0 {
		// ④ division guard: a single zero close in the vendor tail (the
		// CoinAnk path does not filter Close>0) would push h to ±Inf and
		// fail the ENTIRE signal JSON serialization.
		if last.Close > 0 {
			h := macdLine / last.Close * 100 // price-normalized
			sig.MACDHist = &h
		}
		// ③ the trend label needs a VALID previous-bar MACD: with exactly
		// EMA-slow bars, ExportCalculateMACD on kb[:len-1] returns 0
		// (insufficient data) and the sign comparison mislabels a rising
		// line as falling and vice versa. prev==0 stays unlabeled
		// (macd_trend is omitempty).
		if prevLine := market.ExportCalculateMACD(kb[:len(kb)-1]); prevLine != 0 {
			switch {
			case macdLine > prevLine:
				sig.MACDTrend = "rising"
			case macdLine < prevLine:
				sig.MACDTrend = "falling"
			default:
				sig.MACDTrend = "flat"
			}
		}
	}

	// 09-28 review P2: the `rsi > 0` sentinel conflated "insufficient data"
	// (calculateRSI returns 0 for len ≤ period) with the max-oversold
	// extreme — a pure monotonic decline drives Wilder avgGain to exactly 0
	// and RSI to exactly 0, silently dropping the most directional reading
	// from the signal block. Filter on data length, accept rsi >= 0.
	if len(kb) > 14 {
		if rsi := market.ExportCalculateRSI(kb, 14); rsi >= 0 {
			sig.RSI14 = &rsi
		}
	}
	// StochRSI (14,14,3,3) on the closed bars: stochastic of the Wilder RSI
	// series, %K smoothed 3, %D = 3-SMA of %K. >80 overbought / <20 oversold.
	if k, d, ok := stochRSI(c, 14, 14, 3, 3); ok {
		sig.StochRSIK = &k
		sig.StochRSID = &d
	}

	// ── Swing support / resistance from the last ~30 closed bars ──
	structureWindow := 30
	if n < structureWindow {
		structureWindow = n
	}
	window := klines[n-structureWindow:]

	hi, lo := last.Close, last.Close
	var structureHigh, structureLow float64
	for _, k := range window {
		if k.High > hi {
			hi = k.High
		}
		if k.Low < lo {
			lo = k.Low
		}
	}
	structureHigh = hi
	structureLow = lo
	sig.StructureHigh = &structureHigh
	sig.StructureLow = &structureLow

	// Support/resistance are classified against the live price (anchor), not
	// this timeframe's last closed close — a pivot "high" the live price has
	// already cleared is not resistance.
	anchor := anchorPrice
	if anchor <= 0 {
		anchor = last.Close
	}
	// Structure-vs-live distances: negative high-dist = live price ABOVE the
	// window high (new highs printing, no overhead reference in this window);
	// negative low-dist is the normal state (window low below price). Same
	// (level − anchor)/anchor convention as the S/R dist arrays.
	if anchor > 0 {
		shDist := (structureHigh - anchor) / anchor * 100
		slDist := (structureLow - anchor) / anchor * 100
		sig.StructureHighDistPct = &shDist
		sig.StructureLowDistPct = &slDist
	}

	pivHighs, pivLows := swingPivots(window, 2)
	// Keep the full, uncapped pivot set for the hidden program gate. The
	// prompt-facing S/R lists remain capped and live-price-relative.
	sig.StructuralResistance = append([]float64{}, pivHighs...)
	sig.StructuralSupport = append([]float64{}, pivLows...)
	// Resistance: swing highs above price, nearest first. Starts non-nil so
	// an empty side still marshals as [] — "no level on this side of the
	// live price" is evidence, not an absence of data.
	// Cap 3 (was 2): with only two slots per side per TF the nearest two
	// pivots crowded out the level that actually offered min-RR, and the
	// "exhaustive TP check" instruction walked a list of ~6 values
	// (QUANT_REVIEW_2026-09-22 C1).
	resist := []float64{}
	for _, p := range pivHighs {
		if p > anchor {
			resist = append(resist, p)
		}
	}
	sort.Float64s(resist)
	if len(resist) > 3 {
		resist = resist[:3]
	}
	// Supplement with Bollinger upper when thin. BOLL-sourced levels are
	// tagged in BOLLSourced so scanRR excludes them from first_rr_ge_target:
	// a band value decays with its window and must not anchor the TP decision
	// — it may still serve as best_rr context (QUANT_REVIEW_2026-09-22 C1).
	bollSourced := map[float64]bool{}
	if len(tfData.BOLLUpper) > 0 {
		if bu := closedBOLLValue(tfData.BOLLUpper, dropped); bu > anchor {
			bollSourced[bu] = true
			if !inList(resist, bu, anchor) {
				resist = append(resist, bu)
				sort.Float64s(resist)
				if len(resist) > 3 {
					resist = resist[:3]
				}
			}
		}
	}
	if len(tfData.BOLLLower) > 0 {
		if bl := closedBOLLValue(tfData.BOLLLower, dropped); bl < anchor {
			bollSourced[bl] = true
		}
	}
	sig.Resistance = resist

	// Support: swing lows below price, nearest first.
	supp := []float64{}
	for _, p := range pivLows {
		if p < anchor {
			supp = append(supp, p)
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(supp)))
	if len(supp) > 3 {
		supp = supp[:3]
	}
	if len(tfData.BOLLLower) > 0 {
		bl := closedBOLLValue(tfData.BOLLLower, dropped)
		if bl < anchor && !inList(supp, bl, anchor) {
			supp = append(supp, bl)
			sort.Sort(sort.Reverse(sort.Float64Slice(supp)))
			if len(supp) > 3 {
				supp = supp[:3]
			}
		}
	}
	sig.Support = supp
	sig.BOLLSourced = bollSourced

	// Pre-computed percentage distances from the live anchor — the model
	// should not do its own arithmetic on levels.
	sig.SupportDistPct = []float64{}
	sig.ResistanceDistPct = []float64{}
	if anchor > 0 {
		for _, lvl := range sig.Support {
			sig.SupportDistPct = append(sig.SupportDistPct, (lvl-anchor)/anchor*100)
		}
		for _, lvl := range sig.Resistance {
			sig.ResistanceDistPct = append(sig.ResistanceDistPct, (lvl-anchor)/anchor*100)
		}
	}

	return sig, warns
}

// classifyTrend: deterministic EMA/close based trend.
func classifyTrend(c []float64, fast, slow float64, hasEMA bool) string {
	last := c[len(c)-1]
	if !hasEMA || fast <= 0 || slow <= 0 {
		return "unknown"
	}
	// Symmetric 4-quadrant classification (audit 09-13): the EMA pair gives
	// the trend context (fast>slow = up-regime, fast<slow = down-regime) and
	// the price-vs-fast-EMA position gives the state — above = trend
	// continuation (up/down), the other side = dip/bounce entry window
	// (pullback/rally). fast≈slow = range.
	// History: the original down condition tested last<slow and the pullback
	// branch tested the inverted EMA-gap sign (unreachable) — uptrend dips
	// fed "range" (fixed 09-11) and downtrend bounces fed "down", leaving
	// shorts without the mirrored entry window (fixed 09-13).
	switch {
	case fast > slow && last > fast:
		return "up"
	case fast > slow:
		return "pullback"
	case fast < slow && last < fast:
		return "down"
	case fast < slow:
		return "rally"
	default:
		return "range"
	}
}

// srcIsStructural reports whether the direction-specific full pivot set is
// available. A non-nil empty set is meaningful: the timeframe had no pivots
// on that side, so callers must not fall back to the prompt's capped list.
func srcIsStructural(tf *TFSignal, isLong bool) bool {
	if isLong {
		return tf.StructuralResistance != nil
	}
	return tf.StructuralSupport != nil
}

func candleColor(k market.KlineBar) string {
	switch {
	case k.Close > k.Open*1.0005:
		return "bullish"
	case k.Close < k.Open*0.9995:
		return "bearish"
	default:
		return "doji"
	}
}

// swingPivots returns local highs/lows with a 2-bar shoulder on each side.
func swingPivots(k []market.KlineBar, shoulder int) (highs, lows []float64) {
	for i := shoulder; i < len(k)-shoulder; i++ {
		isHigh, isLow := true, true
		for j := i - shoulder; j <= i+shoulder; j++ {
			if j == i {
				continue
			}
			if k[j].High >= k[i].High {
				isHigh = false
			}
			if k[j].Low <= k[i].Low {
				isLow = false
			}
		}
		if isHigh {
			highs = append(highs, k[i].High)
		}
		if isLow {
			lows = append(lows, k[i].Low)
		}
	}
	return highs, lows
}

func inList(list []float64, v, ref float64) bool {
	for _, x := range list {
		if math.Abs(x-v) <= ref*0.0005 {
			return true
		}
	}
	return false
}

func lastNonZero(xs []float64) float64 {
	for i := len(xs) - 1; i >= 0; i-- {
		if xs[i] != 0 {
			return xs[i]
		}
	}
	return 0
}

// closedBOLLValue returns the Bollinger band value aligned with the last
// CLOSED candle: the series arrays are index-aligned with the FULL klines
// (forming bar included), and taking lastNonZero let S/R, the RR scan and
// the limit-suppression breathe with the forming price (2026-09-25 P2).
// dropped = forming candles the settlement logic cut from this window.
func closedBOLLValue(arr []float64, dropped int) float64 {
	if len(arr) == 0 {
		return 0
	}
	idx := len(arr) - 1 - dropped
	if idx < 0 {
		idx = 0
	}
	for idx >= 0 && arr[idx] == 0 {
		idx--
	}
	if idx < 0 {
		return 0
	}
	return arr[idx]
}

// barsToKlines converts KlineBar slices to the full Kline shape the market
// indicator functions expect.
func barsToKlines(k []market.KlineBar) []market.Kline {
	out := make([]market.Kline, len(k))
	for i, b := range k {
		out[i] = market.Kline{
			OpenTime: b.Time, Open: b.Open, High: b.High, Low: b.Low,
			Close: b.Close, Volume: b.Volume,
		}
	}
	return out
}

// wilderATR computes Wilder-smoothed ATR on closed bars.
func wilderATR(bars []market.Kline, period int) float64 {
	n := len(bars)
	if n == 0 {
		return 0
	}
	tr := make([]float64, n)
	tr[0] = bars[0].High - bars[0].Low
	for i := 1; i < n; i++ {
		pc := bars[i-1].Close
		tr[i] = math.Max(bars[i].High-bars[i].Low,
			math.Max(math.Abs(bars[i].High-pc), math.Abs(bars[i].Low-pc)))
	}
	a := tr[0]
	for i := 1; i < n; i++ {
		if i < period {
			a = a + (tr[i]-a)/float64(i+1)
		} else {
			a = (a*float64(period-1) + tr[i]) / float64(period)
		}
	}
	return a
}

// wilderATRSeries returns the Wilder-smoothed ATR series; entry i corresponds
// to bar i+period (the first complete window lands at bar `period`).
func wilderATRSeries(bars []market.Kline, period int) []float64 {
	n := len(bars)
	if n < period+1 {
		return nil
	}
	tr := make([]float64, n)
	tr[0] = bars[0].High - bars[0].Low
	for i := 1; i < n; i++ {
		pc := bars[i-1].Close
		tr[i] = math.Max(bars[i].High-bars[i].Low,
			math.Max(math.Abs(bars[i].High-pc), math.Abs(bars[i].Low-pc)))
	}
	a := 0.0
	for i := 1; i <= period; i++ {
		a += tr[i]
	}
	a /= float64(period)
	series := []float64{a}
	for i := period + 1; i < n; i++ {
		a = (a*float64(period-1) + tr[i]) / float64(period)
		series = append(series, a)
	}
	return series
}

// wilderRSISeries returns the Wilder RSI series over closes (index-aligned;
// the entry at `period` is the first complete value).
func wilderRSISeries(c []float64, period int) []float64 {
	n := len(c)
	if n < period+1 {
		return nil
	}
	rsi := make([]float64, n)
	var gains, losses float64
	for i := 1; i <= period; i++ {
		d := c[i] - c[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	ag, al := gains/float64(period), losses/float64(period)
	setRSI := func(i int, ag, al float64) {
		switch {
		case ag == 0 && al == 0:
			rsi[i] = 50 // perfectly flat series — no direction, not overbought
		case al == 0:
			rsi[i] = 100
		default:
			rsi[i] = 100 - 100/(1+ag/al)
		}
	}
	setRSI(period, ag, al)
	for i := period + 1; i < n; i++ {
		d := c[i] - c[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		ag = (ag*float64(period-1) + g) / float64(period)
		al = (al*float64(period-1) + l) / float64(period)
		setRSI(i, ag, al)
	}
	return rsi
}

// stochRSI computes the StochRSI %K/%D over closes: raw %K places the latest
// RSI within its `stochPeriod` range; %K is a smoothK-SMA of the raw values
// and %D a smoothD-SMA of %K. ok=false when history is insufficient.
func stochRSI(c []float64, rsiPeriod, stochPeriod, smoothK, smoothD int) (k, d float64, ok bool) {
	rsi := wilderRSISeries(c, rsiPeriod)
	// rsi[0..rsiPeriod-1] are unset zeros — a stochastic window may only
	// start where real RSI values exist, otherwise the zero head forces
	// lo=0 and poisons %K/%D on thin history.
	if len(rsi)-rsiPeriod < stochPeriod {
		return 0, 0, false
	}
	rawK := make([]float64, 0, len(rsi))
	for i := rsiPeriod + stochPeriod - 1; i < len(rsi); i++ {
		hi, lo := rsi[i], rsi[i]
		for j := i - stochPeriod + 1; j <= i; j++ {
			if rsi[j] > hi {
				hi = rsi[j]
			}
			if rsi[j] < lo {
				lo = rsi[j]
			}
		}
		if hi == lo {
			rawK = append(rawK, 50)
		} else {
			rawK = append(rawK, (rsi[i]-lo)/(hi-lo)*100)
		}
	}
	kSeries := smaSeries(rawK, smoothK)
	if len(kSeries) == 0 {
		return 0, 0, false
	}
	dSeries := smaSeries(kSeries, smoothD)
	if len(dSeries) == 0 {
		return 0, 0, false
	}
	return kSeries[len(kSeries)-1], dSeries[len(dSeries)-1], true
}

// smaSeries returns the `period`-simple moving average series (the first
// complete window lands at index period-1).
func smaSeries(xs []float64, period int) []float64 {
	if len(xs) < period {
		return nil
	}
	out := make([]float64, 0, len(xs)-period+1)
	for i := period - 1; i < len(xs); i++ {
		s := 0.0
		for j := i - period + 1; j <= i; j++ {
			s += xs[j]
		}
		out = append(out, s/float64(period))
	}
	return out
}

// primaryTFSignal returns the timeframe signal driving the deterministic
// entry/exit rules: the strategy's configured primary timeframe when it is
// present in the computed block, falling back to the longest timeframe
// (direction context) when the primary data is missing.
func primaryTFSignal(sig *SymbolSignal, primaryTF string) *TFSignal {
	if primaryTF != "" {
		if tf, ok := sig.Timeframes[primaryTF]; ok && tf != nil {
			return tf
		}
	}
	keys := make([]string, 0, len(sig.Timeframes))
	for k := range sig.Timeframes {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Slice(keys, func(i, j int) bool { return tfDuration(keys[i]) > tfDuration(keys[j]) })
	return sig.Timeframes[keys[0]]
}

// TimeframeTrend returns the deterministic trend classification ("up", "down",
// "pullback", "rally", "range", "unknown") for one timeframe of a symbol, or "" when
// the data is unavailable. Used by the trader's hard risk gates (e.g. blocking
// counter-trend shorts on the 1d timeframe).
func TimeframeTrend(data *market.Data, tf string) string {
	if data == nil || data.CurrentPrice <= 0 {
		return ""
	}
	sig, err := ComputeSymbolSignals(data.Symbol, data, SignalOptions{Now: time.Now()})
	if err != nil {
		return ""
	}
	if s, ok := sig.Timeframes[tf]; ok && s != nil {
		return s.Trend
	}
	return ""
}

func tfDuration(tf string) time.Duration {
	return marketTFDuration(tf)
}

// marketTFDuration reads the market package's supported timeframe table,
// defaulting unknown labels to 1h.
func marketTFDuration(tf string) time.Duration {
	if d := market.TimeframeDuration(tf); d > 0 {
		return d
	}
	return time.Hour
}

// rolling1hChangePct measures the live price against the close ~60 minutes
// ago, using the finest ≤1h timeframe available in the block (15m×4 for most
// strategies, 30m×2 or 1h×1 as fallbacks). This works for every listing —
// including tokenized stock/commodity perps that Binance doesn't carry.
func rolling1hChangePct(data *market.Data, now time.Time) float64 {
	if data == nil || data.CurrentPrice <= 0 {
		if data != nil {
			return data.PriceChange1h
		}
		return 0
	}
	bestTf := ""
	bestDur := time.Duration(0)
	for tf := range data.TimeframeData {
		dur := market.TimeframeDuration(tf)
		if dur <= 0 || dur > time.Hour {
			continue
		}
		if bestDur == 0 || dur < bestDur {
			bestTf, bestDur = tf, dur
		}
	}
	if bestTf == "" {
		return data.PriceChange1h
	}
	barsBack := int(time.Hour / bestDur)
	if barsBack < 1 {
		barsBack = 1
	}
	closes := closedCloses(data.TimeframeData[bestTf], now, bestDur)
	idx := len(closes) - 1 - barsBack
	if idx < 0 || closes[idx] <= 0 {
		return data.PriceChange1h
	}
	return (data.CurrentPrice/closes[idx] - 1) * 100
}

// closedCloses returns the closes of bars fully closed at `now`.
func closedCloses(tfData *market.TimeframeSeriesData, now time.Time, dur time.Duration) []float64 {
	kl := ClosedKlines(tfData, now, dur)
	out := make([]float64, 0, len(kl))
	for _, k := range kl {
		out = append(out, k.Close)
	}
	return out
}

// closedKlines drops the still-forming tail candle, mirroring computeTFSignal.
func ClosedKlines(tfData *market.TimeframeSeriesData, now time.Time, dur time.Duration) []market.KlineBar {
	kl := tfData.Klines
	if len(kl) > 0 && time.UnixMilli(kl[len(kl)-1].Time).Add(dur).After(now) {
		kl = kl[:len(kl)-1]
	}
	return kl
}

// btcCorrelation computes Pearson correlation and beta of hourly returns
// (symbol vs BTC), aligning both series from the most recent closed bar.
func btcCorrelation(symCloses, btcCloses []float64) *BTCCorrelation {
	n := len(symCloses)
	if len(btcCloses) < n {
		n = len(btcCloses)
	}
	const maxSamples = 72
	const minReturns = 20
	if n > maxSamples {
		n = maxSamples
	}
	if n < minReturns+1 {
		return nil
	}
	sym := symCloses[len(symCloses)-n:]
	btc := btcCloses[len(btcCloses)-n:]
	var rs, rb []float64
	for i := 1; i < n; i++ {
		if sym[i-1] > 0 && btc[i-1] > 0 {
			rs = append(rs, sym[i]/sym[i-1]-1)
			rb = append(rb, btc[i]/btc[i-1]-1)
		}
	}
	if len(rs) < minReturns {
		return nil
	}
	var ms, mb float64
	for i := range rs {
		ms += rs[i]
		mb += rb[i]
	}
	ms /= float64(len(rs))
	mb /= float64(len(rb))
	var cov, vs, vb float64
	for i := range rs {
		cov += (rs[i] - ms) * (rb[i] - mb)
		vs += (rs[i] - ms) * (rs[i] - ms)
		vb += (rb[i] - mb) * (rb[i] - mb)
	}
	pearson := 0.0
	if vs > 0 && vb > 0 {
		pearson = cov / (math.Sqrt(vs) * math.Sqrt(vb))
	}
	beta := 0.0
	if vb > 0 {
		beta = cov / vb
	}
	return &BTCCorrelation{Pearson: pearson, Beta: beta, Samples: len(rs), Window: "1h"}
}

// RenderSignalJSON renders one symbol's signal block as compact JSON.
// RenderSignalJSON renders the structured signal as compact JSON with
// decision-grade numeric precision (09-18 token audit ④): raw float64
// marshaling printed 15-18% junk digits (1.5343639999999998,
// -19.394719896973594). Prices keep 6 significant digits; any *_pct /
// *percent* field keeps 2 decimals — both far below the granularity any
// entry/SL/TP decision operates at. Counts/strings/timestamps untouched.
func RenderSignalJSON(sig *SymbolSignal) string {
	data, err := json.Marshal(sig)
	if err != nil {
		return fmt.Sprintf("signal serialization failed: %v", err)
	}
	var generic interface{}
	if json.Unmarshal(data, &generic) != nil {
		return string(data)
	}
	rounded, err := json.Marshal(roundJSONNumbers(generic, ""))
	if err != nil {
		return string(data)
	}
	return string(rounded)
}

// roundJSONNumbers walks decoded JSON and rounds floats by field convention:
// keys ending _pct or containing "percent" → 2 decimals, everything else → 6
// significant digits (funding_rate 0.00033705 survives intact; oi/volume
// bases lose only sub-1-unit noise). Array children inherit their key.
func roundJSONNumbers(v interface{}, key string) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			t[k] = roundJSONNumbers(child, k)
		}
		return t
	case []interface{}:
		for i, child := range t {
			t[i] = roundJSONNumbers(child, key)
		}
		return t
	case float64:
		if strings.HasSuffix(key, "_pct") || strings.Contains(key, "percent") {
			return roundToDecimalPlaces(t, 2)
		}
		return roundSignificant(t, 6)
	default:
		return v
	}
}

// roundToDecimalPlaces rounds to n digits after the decimal point via
// FormatFloat/ParseFloat — math.Round(v*10^n)/10^n re-multiplies the scaling
// error back in (100198×0.001 → 100.19800000000001).
func roundToDecimalPlaces(v float64, n int) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return v
	}
	out, err := strconv.ParseFloat(strconv.FormatFloat(v, 'f', n, 64), 64)
	if err != nil {
		return v
	}
	return out
}

// roundSignificant rounds to n significant digits. Same FormatFloat trick as
// roundToDecimalPlaces: the naive Round(v/pow)*pow reintroduces float noise.
func roundSignificant(v float64, n int) float64 {
	if v == 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return v
	}
	exp := int(math.Floor(math.Log10(math.Abs(v))))
	digits := n - 1 - exp
	if digits > 20 {
		digits = 20
	}
	if digits < -30 {
		digits = -30
	}
	out, err := strconv.ParseFloat(strconv.FormatFloat(v, 'f', digits, 64), 64)
	if err != nil {
		return v
	}
	return out
}

// computeBreakoutState derives the breakout verdict (⑫) from the 1h closed
// bars and a FIXED pre-push structure level, in the direction the 1h trend
// implies:
//
//	up   → breakout  vs the pre-push structure high (approach/broken/confirmed/fake/retest/extended)
//	down → breakdown vs the pre-push structure low  (mirror)
//
// The level is the extreme of the 30 closed bars BEFORE the recent 8-bar
// cross window — NOT a rolling window that includes the newest bars. The old
// rolling max trailed the price: no closed bar's high could ever exceed it,
// so cross detection never fired, fake_break/retest_hold/extended were
// unreachable, and the retest anchor only ever activated during the single
// hour the live price was printing a rolling 30-bar high — at which point
// the "anchor" was a chase just under the fresh high (review 2026-10-04 C1).
//
// Status semantics:
//
//	below               price has not reached the level
//	approach            within 1×ATR(1h)% of the level
//	broken_unconfirmed  beyond the level but without volume/OI confirmation
//	confirmed           beyond the level WITH volume + OI confirmation
//	fake_break          closed across within the last 8 bars but the last close is back inside
//	retest_hold         closed across within the last 8 bars, later pulled back to the level and held
//	extended            closed across >=2 bars ago, never retested, no vol+OI confirmation (chase context)
//
// (full decision table: see the cross-detection block below, review 2026-10-07 B2-3)
//
// Best-effort: any missing input (no 1h bars, no structure level, no ATR)
// returns nil — the model falls back to reading the raw levels. A series too
// short to carve out a prior window falls back to the TF's rolling
// structure extreme (degraded but non-nil).
func computeBreakoutState(data *market.Data, sig *SymbolSignal, now time.Time) *BreakoutState {
	if data == nil || sig == nil || sig.Price <= 0 {
		return nil
	}
	t1h := sig.Timeframes["1h"]
	if t1h == nil || t1h.StructureHigh == nil || t1h.StructureLow == nil {
		return nil
	}
	isLong := t1h.Trend == "up" || t1h.Trend == "pullback"
	isShort := t1h.Trend == "down" || t1h.Trend == "rally"
	if !isLong && !isShort {
		return nil // no directional call to anchor the level choice
	}
	level := *t1h.StructureHigh
	direction := "breakout"
	if isShort {
		level = *t1h.StructureLow
		direction = "breakdown"
	}
	if level <= 0 {
		return nil
	}

	// Closed 1h bars + the fixed prior-window level. Settlement is
	// time-based (ClosedKlines — the SAME convention as computeTFSignal):
	// the old unconditional [:len-1] drop silently lost one closed bar
	// whenever the feed lagged, review 2026-10-04 C2.
	bars, haveBars := func() ([]market.KlineBar, bool) {
		tfData, ok := data.TimeframeData["1h"]
		if !ok || tfData == nil {
			return nil, false
		}
		kl := ClosedKlines(tfData, now, tfDuration("1h"))
		if len(kl) < 16 {
			return nil, false
		}
		return kl, true
	}()
	if haveBars {
		n := len(bars)
		end := n - boCrossWindow
		start := end - boLevelWindow
		if start < 0 {
			start = 0
		}
		if end-start >= 10 {
			if isLong {
				hi := math.Inf(-1)
				for _, b := range bars[start:end] {
					if b.High > hi {
						hi = b.High
					}
				}
				if hi > 0 && !math.IsInf(hi, -1) {
					level = hi
				}
			} else {
				lo := math.Inf(1)
				for _, b := range bars[start:end] {
					if b.Low > 0 && b.Low < lo {
						lo = b.Low
					}
				}
				if lo > 0 && !math.IsInf(lo, 1) {
					level = lo
				}
			}
		}
	}

	dist := 0.0
	if isLong {
		dist = (sig.Price - level) / level * 100
	} else {
		dist = (level - sig.Price) / level * 100
	}
	beyond := dist > 0

	// ATR(1h)% for the "approach" band.
	atrPct := t1h.ATRPct // TFSignal.ATRPct is a plain float64 (percent)

	st := &BreakoutState{
		Direction:   direction,
		Level:       level,
		DistancePct: math.Round(dist*100) / 100,
	}
	switch {
	case !beyond:
		st.Status = "below"
		if atrPct > 0 && -dist <= atrPct {
			st.Status = "approach"
		}
		return st
	}

	// Beyond the level: volume + OI confirmation.
	if t1h.VolumeRatio != nil {
		st.VolumeConfirm = *t1h.VolumeRatio >= 1.5
	}
	if sig.Derivatives != nil && sig.Derivatives.OIChange1hPct != nil {
		// OI expanding IN the break direction = new positions driving the
		// move (longs on a breakout, shorts on a breakdown). The old `< 0`
		// for shorts confirmed liquidation-driven declines — inverted
		// (audit 2026-09-11). The raw `> 0` confirmed ~half of all hours —
		// require the same meaningful expansion the piggy OI dim centers
		// its sigmoid on (0.3%/1h), review 2026-10-04 C2.
		st.OIConfirm = *sig.Derivatives.OIChange1hPct > 0.3
	}

	// Cross recency + retest/fake detection from the closed 1h bars.
	//
	// review 2026-10-07 B2-3: the level is the extreme of bars[n-38:n-8], so
	// a bar inside the level window can never exceed it and a scan over bars
	// older than the window says nothing about THIS level. The old
	// "olderCross" scan therefore only fired on a pre-window higher high,
	// mislabeling fresh breaks as extended/retest_hold and making a genuine
	// cross-then-retest unreachable. Now everything anchors on the first
	// genuine cross inside the last boCrossWindow bars.
	//
	// A cross is an inside→outside CLOSE transition (previous close on/inside
	// the level, this close beyond it) — same definition as
	// market/breakout computeTF. Wicks alone are not crosses; a bar that
	// already opened/closed outside is not a fresh cross.
	//
	// Decision table (cross = first transition in the last 8 closed bars;
	// "latest" = last closed bar's close):
	//
	//	cross, latest back inside                        → fake_break
	//	cross, latest beyond, a later bar's Low (long) /
	//	  High (short) came within 0.25% of level        → retest_hold
	//	cross, latest beyond, no retest, vol+OI confirm  → confirmed
	//	cross, latest beyond, no retest, no confirm,
	//	  cross is >=2 bars old                          → extended (chase)
	//	cross on the latest bar / no cross in window
	//	  (live price beyond), vol+OI confirm            → confirmed
	//	otherwise                                        → broken_unconfirmed
	if haveBars {
		n := len(bars)
		beyondClose := func(c float64) bool {
			if isLong {
				return c > level
			}
			return c < level
		}
		crossedIdx := -1
		for i := n - boCrossWindow; i < n; i++ {
			if i < 1 {
				continue
			}
			if !beyondClose(bars[i-1].Close) && beyondClose(bars[i].Close) {
				crossedIdx = i
				break
			}
		}
		backInside := !beyondClose(bars[n-1].Close)
		retested := false
		if crossedIdx >= 0 {
			for i := crossedIdx + 1; i < n; i++ {
				if isLong && bars[i].Low <= level*(1+0.0025) {
					retested = true
				}
				if !isLong && bars[i].High >= level*(1-0.0025) {
					retested = true
				}
			}
		}
		switch {
		case crossedIdx >= 0 && backInside:
			st.Status = "fake_break"
		case crossedIdx >= 0 && retested:
			st.Status = "retest_hold"
		case st.VolumeConfirm && st.OIConfirm:
			st.Status = "confirmed"
		case crossedIdx >= 0 && crossedIdx < n-1:
			st.Status = "extended"
		default:
			st.Status = "broken_unconfirmed"
		}
		return st
	}
	if st.VolumeConfirm && st.OIConfirm {
		st.Status = "confirmed"
	} else {
		st.Status = "broken_unconfirmed"
	}
	return st
}

// suppressAnchorsAgainstStructure zeroes limit_buy_price / limit_sell_price
// when the anchor sits within thresholdPct of the opposite-side structure
// (long: resistance + structure_high above the buy anchor; short: supports
// below the sell anchor). Zero = "no valid anchor this direction" — the
// trader's supply-zone gate would reject the order.
//
// Resistance/support ARRAYS are classified against the LIVE price, so a
// level between the anchor and live itself is invisible to them — yet filling
// the limit right under that level is exactly the supply-zone trap. Raw swing
// pivots close that blind zone, but only on the 1h series: the pivot scan
// used to cover 15m too, and once the threshold moved to the 1h yardstick
// (0.3-1.2%) every 15m minor pivot inside the pullback corridor zeroed the
// anchor — 63% suppression and a whole day without a single open (2026-09-10).
// 15m minors are noise at this scale; the curated 15m/1h arrays stay in.
func suppressAnchorsAgainstStructure(sig *SymbolSignal, data *market.Data, thresholdPct float64, now time.Time) {
	nearestAbove, nearestBelow := 0.0, 0.0
	for _, tfName := range []string{"1h"} {
		tfData, ok := data.TimeframeData[tfName]
		if !ok || tfData == nil {
			continue
		}
		// Time-based settlement, same convention as computeTFSignal (C2):
		// the unconditional [:len-1] drop lost one closed bar on laggy feeds.
		bars := ClosedKlines(tfData, now, tfDuration(tfName))
		if len(bars) < 8 {
			continue
		}
		highs, lows := swingPivots(bars, 2)
		for _, h := range highs {
			if h > sig.LimitBuyPrice && (nearestAbove == 0 || h < nearestAbove) {
				nearestAbove = h
			}
		}
		for _, l := range lows {
			if l > 0 && l < sig.LimitSellPrice && (nearestBelow == 0 || l > nearestBelow) {
				nearestBelow = l
			}
		}
	}
	// Keep the array levels too (they are pivot-derived anyway, and the
	// execution-side gate checks exactly these).
	for _, tfName := range []string{"15m", "1h"} {
		if t := sig.Timeframes[tfName]; t != nil {
			for _, r := range t.Resistance {
				if r > sig.LimitBuyPrice && (nearestAbove == 0 || r < nearestAbove) {
					nearestAbove = r
				}
			}
			for _, sup := range t.Support {
				if sup > 0 && sup < sig.LimitSellPrice && (nearestBelow == 0 || sup > nearestBelow) {
					nearestBelow = sup
				}
			}
		}
	}
	if sig.LimitBuyPrice > 0 && nearestAbove > 0 {
		if (nearestAbove-sig.LimitBuyPrice)/sig.LimitBuyPrice*100 < thresholdPct {
			sig.LimitBuyPrice = 0
			sig.Warnings = append(sig.Warnings, fmt.Sprintf("limit_buy_price suppressed: anchor within %.1f%% of a swing-high pivot — no breathing room, long limit would be rejected", thresholdPct))
		}
	}
	if sig.LimitSellPrice > 0 && nearestBelow > 0 {
		if (sig.LimitSellPrice-nearestBelow)/sig.LimitSellPrice*100 < thresholdPct {
			sig.LimitSellPrice = 0
			sig.Warnings = append(sig.Warnings, fmt.Sprintf("limit_sell_price suppressed: anchor within %.1f%% of a swing-low pivot — no breathing room, short limit would be rejected", thresholdPct))
		}
	}
}
