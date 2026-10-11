package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"nofx/market/breakout"
	"nofx/provider/hyperliquid"
	"nofx/provider/nofxos"
	"nofx/provider/openbb"
	"nofx/provider/vergex"
	"nofx/security"
	"nofx/store"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// Type Definitions
// ============================================================================

// PositionInfo position information
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"` // 保证金口径 ROI (含杠杆)
	PriceReturnPct   float64 `json:"price_return_pct"`   // 标的价格涨跌幅 (不含杠杆) — ②拆开两种口径
	PeakPnLPct       float64 `json:"peak_pnl_pct"`       // Historical peak profit percentage
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // Position update timestamp (milliseconds)
	// AIManaged: opened by this system's AI (2026-09-25 hands-off rule).
	// false = manually opened — the program never closes/adjusts it and the
	// automation loop leaves it alone; close/adjust decisions on it are
	// rejected at execution.
	Managed         bool    `json:"ai_managed"`
	StopLossPrice   float64 `json:"stop_loss_price"`   // Trigger price of the protective SL order currently on the exchange (0 = none found)
	TakeProfitPrice float64 `json:"take_profit_price"` // Trigger price of the protective TP order currently on the exchange (0 = none found)
	// AutomationStage synchronizes program-owned management with the LLM so
	// it does not repeat an already executed breakeven/trim action.
	AutomationStage      string  `json:"automation_stage"`
	CumulativeReducedPct float64 `json:"cumulative_reduced_pct"` // fraction of original entry quantity already closed, 0..100
}

// AccountInfo account information
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // Account equity
	AvailableBalance float64 `json:"available_balance"` // Available balance
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized profit/loss
	TotalPnL         float64 `json:"total_pnl"`         // Total profit/loss
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total profit/loss percentage
	MarginUsed       float64 `json:"margin_used"`       // Used margin
	MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage rate
	PositionCount    int     `json:"position_count"`    // Number of positions
}

// CandidateCoin candidate coin (from coin pool)
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // Sources: "ai500", "oi_top", "short_scan", ...
	// Short-side metadata (short_scan sourced candidates only):
	ShortScore      float64  `json:"short_score,omitempty"`       // 0-100 short suitability
	ShortGrade      string   `json:"short_grade,omitempty"`       // strong / medium / weak
	ShortConfirmed  bool     `json:"short_confirmed,omitempty"`   // scanner topping confirmation (顶背离/假突破 only; EMA20 break and funding rollover are NOT included, review 2026-10-07) — feeds SHORT_TOP_CONFIRM_MISSING
	ShortReasons    []string `json:"short_reasons,omitempty"`     // topping confirmations printed
	ShortFundingAnn float64  `json:"short_funding_ann,omitempty"` // funding annualized % AT SCAN TIME
	ShortScanAtMs   int64    `json:"short_scan_at_ms,omitempty"`  // when the scanner snapshot was taken
	ShortUniverse   string   `json:"short_universe,omitempty"`
	// NearHighAlso: also passed the grinding-top screen under another universe
	// label — scan time exempted it from the min-OI floor, fetch must too
	// (2026-10-03 review: exemption was dead for collision coins).
	NearHighAlso bool `json:"near_high_also,omitempty"` // "gainer" | "hist_gainer" (历史涨幅池) | "near_high" (磨顶池)
	// ScannerDirection is the direction the scanning engine concluded for
	// this symbol: short_scan ⇒ "short"; piggy_dash ⇒ "up"/"down".
	ScannerDirection string `json:"scanner_direction,omitempty"`
	// ScannerConflict marks the symbol appearing in two scanners with
	// OPPOSITE directional conclusions (short_scan short vs piggy_dash up)
	// — surfaced as signal_conflict type SCANNER_VS_SCANNER so the model
	// must resolve it instead of silently receiving both hints.
	ScannerConflict bool `json:"scanner_conflict,omitempty"`
}

// OITopData open interest growth top data (for AI decision reference)
type OITopData struct {
	Rank              int     // OI Top ranking
	OIDeltaPercent    float64 // Open interest change percentage (1 hour)
	OIDeltaValue      float64 // Open interest change value
	PriceDeltaPercent float64 // Price change percentage
}

// TradingStats trading statistics (for AI input)
type TradingStats struct {
	TotalTrades    int     `json:"total_trades"`        // Total number of trades (closed)
	WinRate        float64 `json:"win_rate"`            // Win rate (%)
	ProfitFactor   float64 `json:"profit_factor"`       // Profit factor
	SharpeRatio    float64 `json:"sharpe_ratio"`        // Sharpe ratio
	TotalPnL       float64 `json:"total_pnl"`           // Total profit/loss
	TotalFee       float64 `json:"total_fee,omitempty"` // Sum of commissions in the window (net-caliber context)
	AvgWin         float64 `json:"avg_win"`             // Average win
	AvgLoss        float64 `json:"avg_loss"`            // Average loss
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`    // Maximum drawdown (%)
	// Equity-curve provenance for the max drawdown (user audit 2026-10-01:
	// "42.7%" looked implausible next to −4.25 USDT total PnL because it
	// described a REAL earlier crash — peak 90.96 → trough 52.14 on 09-15 —
	// against a baseline that predates the manual initial-balance reset).
	// Dates make the number auditable. Empty = fallback cumsum path.
	MaxDDPeakEquity   float64 `json:"max_dd_peak_equity,omitempty"`
	MaxDDTroughEquity float64 `json:"max_dd_trough_equity,omitempty"`
	MaxDDPeakAt       string  `json:"max_dd_peak_at,omitempty"`
	MaxDDTroughAt     string  `json:"max_dd_trough_at,omitempty"`
	WindowDays        int     `json:"window_days,omitempty"` // Rolling stats window in days; 0 = full history

	// MEASURED R distribution (journal rows with a planned stop; E1,
	// QUANT_REVIEW 09-22). Samples >= MinMeasuredRSamples → the prompt's
	// expectancy_r uses these real numbers instead of the "every loser = −1R"
	// assumption. Zero values = not enough data, fallback applies.
	MeasuredAvgWinR     float64 `json:"measured_avg_win_r,omitempty"`
	MeasuredAvgLossR    float64 `json:"measured_avg_loss_r,omitempty"` // positive magnitude
	MeasuredExpectancyR float64 `json:"measured_expectancy_r,omitempty"`
	MeasuredRSamples    int     `json:"measured_r_samples,omitempty"`
}

// MinMeasuredRSamples gates the measured-R fallback: below this the
// measured expectancy is noise and the −1R-assumption estimate renders
// instead (labeled as such in the prompt).
const MinMeasuredRSamples = 5

// RecentOrder recently completed order (for AI input)
type RecentOrder struct {
	Symbol      string  `json:"symbol"`       // Trading pair
	Side        string  `json:"side"`         // long/short
	EntryPrice  float64 `json:"entry_price"`  // Entry price
	ExitPrice   float64 `json:"exit_price"`   // Exit price
	RealizedPnL float64 `json:"realized_pnl"` // Realized profit/loss
	PnLPct      float64 `json:"pnl_pct"`      // PRICE return % (no leverage — 09-19 audit)
	// PositionValue (entry notional) and Fee make the row reconcilable:
	// risk/margin math runs on the notional, fees explain PnL gaps.
	PositionValue float64 `json:"position_value"`
	Fee           float64 `json:"fee"`
	EntryTime     string  `json:"entry_time"`    // Entry time
	ExitTime      string  `json:"exit_time"`     // Exit time
	HoldDuration  string  `json:"hold_duration"` // Hold duration, e.g. "2h30m"
}

// Context trading context (complete information passed to AI)
type Context struct {
	CurrentTime    string `json:"current_time"`
	RuntimeMinutes int    `json:"runtime_minutes"`
	CallCount      int    `json:"call_count"`
	// InitialBalanceUSDT is the trader's starting balance — the baseline the
	// account-level drawdown breaker measures against. Rendered with the
	// account line so the model can see the CURRENT breaker state instead of
	// guessing from the stats block's historical max-drawdown (09-18 audit #5:
	// "最大回撤 80.4%" is a closed-trade-series figure, not equity vs initial).
	InitialBalanceUSDT float64 `json:"-"`
	// DayStartEquityUSDT is the daily-loss halt's day-start anchor (user
	// audit 2026-10-01: the −10% daily baseline was invisible to the model).
	// 0 = not anchored yet this cycle.
	DayStartEquityUSDT float64 `json:"-"`
	// Exchange is the EXECUTION venue of the trader that owns this cycle
	// (F10, 2026-10-01 review). Analysis/anchor data must come from the same
	// venue — the generic GetWithTimeframes pins Binance, which priced
	// Bybit/OKX/Hyperliquid signals off Binance books. Empty = binance.
	Exchange           string                             `json:"-"`
	Account            AccountInfo                        `json:"account"`
	Positions          []PositionInfo                     `json:"positions"`
	CandidateCoins     []CandidateCoin                    `json:"candidate_coins"`
	RulesText          string                             `json:"rules_text,omitempty"` // Review-derived trading rules (hard + soft lessons)
	PromptVariant      string                             `json:"prompt_variant,omitempty"`
	TradingStats       *TradingStats                      `json:"trading_stats,omitempty"`
	RecentOrders       []RecentOrder                      `json:"recent_orders,omitempty"`
	MarketDataMap      map[string]*market.Data            `json:"-"`
	MultiTFMarket      map[string]map[string]*market.Data `json:"-"`
	OITopDataMap       map[string]*OITopData              `json:"-"`
	QuantDataMap       map[string]*QuantData              `json:"-"`
	OIRankingData      *nofxos.OIRankingData              `json:"-"` // Market-wide OI ranking data
	NetFlowRankingData *nofxos.NetFlowRankingData         `json:"-"` // Market-wide fund flow ranking data
	PriceRankingData   *nofxos.PriceRankingData           `json:"-"` // Market-wide price gainers/losers
	SymbolStats        map[string]*TraderHistoryStat      `json:"-"` // per-symbol closed-trade record for this trader
	// LossStreakBanned maps symbol → ban-expiry for symbols currently under
	// the loss-streak circuit breaker (program-computed by the trader from
	// the closed-trade record). The prompt instructs the model to cite
	// LOSS_STREAK_BAN only for symbols present here.
	LossStreakBanned map[string]time.Time    `json:"-"`
	LimitAnchors     map[string]*LimitAnchor `json:"-"` // pre-computed open_*_limit anchors per symbol (prompt-build time)
	// RRCeilings records per symbol the rr_scan best_rr the model was SHOWN
	// per direction at prompt-build time (program values, not model echoes).
	// This powers the "cost of waiting for the micro-trend" dataset (user
	// review 09-16 point 2): when a WATCH_SHORT candidate finally opens after
	// the 15m turn, its realized entry RR compares against the ceiling the
	// wait snapshot promised — quantifying how much RR decays between the
	// trigger event and the fill (lower entry, nearer target).
	RRCeilings map[string]*RRCeiling `json:"-"`

	// GateStates records per symbol the hard_entry_gate verdicts the model
	// was SHOWN (allowed per direction) at prompt-build time. wait_state is
	// derived from these + wait_bias — the model no longer outputs it
	// (schema-redundancy audit 09-16).
	GateStates map[string]*GateState `json:"-"`
	// FilteredCandidates: normalized symbol -> why the coin was dropped before
	// prompt rendering; CandidateOrder: the pool as it was BEFORE the fetch
	// pass removed coins (2026-10-10 per-candidate block reasons).
	FilteredCandidates map[string]string `json:"-"`
	CandidateOrder     []string          `json:"-"`
	BTCETHLeverage     int               `json:"-"`
	AltcoinLeverage    int               `json:"-"`
	Timeframes         []string          `json:"-"`
}

// LimitAnchor carries the pre-computed limit-entry prices for one symbol —
// exactly the values the model was shown as limit_buy_price / limit_sell_price.
type LimitAnchor struct {
	LimitBuy  float64 `json:"limit_buy"`
	LimitSell float64 `json:"limit_sell"`
}

// RRCeiling is the per-symbol rr_scan ceiling captured at prompt-build time.
type RRCeiling struct {
	LongRR      float64 // rr_scan.long best_rr (0 = no scan rendered)
	ShortRR     float64
	LongUsable  bool
	ShortUsable bool
}

// GateState is the per-symbol hard_entry_gate verdict snapshot the wait_state
// derivation reads (both directions' allowed flags as shown to the model).
type GateState struct {
	LongAllowed  bool
	ShortAllowed bool
	// MarketException per direction (B1, QUANT_REVIEW 2026-09-22): the
	// direction-matched market-order exception holds with program evidence.
	// The trader's open dispatch degrades a market open to the anchor limit
	// (or drops it) unless this is true for its direction.
	LongMarketException  bool
	ShortMarketException bool
	// LimitAllowed per direction: a live limit anchor exists (limit path
	// executable). Drives the degrade-vs-drop choice in the same gate.
	LongLimitAllowed  bool
	ShortLimitAllowed bool
	// Precomputed methodology stops per direction (0 = no plan — the gate
	// blocked the direction with STOP_PLAN_* or no floor is configured).
	// Drives the post-parse stop-loss snap: the executed trade must equal
	// the gated trade (09-19 audit: the model echoed its own 2.13% stop on
	// ZEC against the plan and the executor rejected the whole output).
	LongStopPlanPrice  float64
	ShortStopPlanPrice float64
	// LongPullbackActive/Level: the breakout-retest anchor exemption for the
	// executor's supply-zone gate (2026-10-03 review) — the retest anchor
	// intentionally sits at the just-broken level, and the pivots a few
	// tenths above it are the breakout's own extension, not overhead supply.
	// The pending-path supply check skips when the decision price ≈ the
	// pullback level.
	LongPullbackActive bool
	LongPullbackLevel  float64
	// TP menus per direction (user directive 2026-09-29): the qualifying
	// structural targets the model picks between (Decision.TPOption,
	// 1-based). Option 1 IS the legacy first_rr_ge_target;
	// resolveTakeProfitOptions re-points LongTakeProfit/ShortTakeProfit at
	// the CHOSEN level before the post-parse TP snap, so an off-menu TP
	// stays impossible while the regime call (near vs far) is the model's.
	LongTPMenu  []TPOption
	ShortTPMenu []TPOption
	// HardBlocked: BOTH directions carry a no-exception blocker — the coin
	// can only ever produce a mechanical wait this cycle. Drives the
	// regime-level skip (09-19 audit: all-candidates-blocked + no positions
	// ⇒ the LLM call can only return a hold; synthesize it for free).
	HardBlocked bool
	// Shadow-block bookkeeping (09-21 user directive): per-direction would-be
	// trade for the gate-calibration dataset — blocked directions with a
	// complete entry/stop/TP triple are recorded and evaluated against the
	// later price path. Entry is the basis the gate priced the direction at
	// (live price or limit anchor); TP is rr_scan.first_rr_ge_target.
	LongEntryPrice  float64
	ShortEntryPrice float64
	LongEntryBasis  string
	ShortEntryBasis string
	LongTakeProfit  float64
	ShortTakeProfit float64
	LongFailed      []string
	ShortFailed     []string
}

// DeriveWaitStateFromGate resolves the wait_state enum mechanically from the
// program's own hard_entry_gate verdicts plus the model's directional bias
// (schema-redundancy audit 09-16 — the model no longer outputs wait_state;
// every input is backend-known). Precedence per the enum's definition: both
// directions blocked → BLOCKED (regardless of bias — the ZEC case: the bias
// survives a mechanical block as a WATCH at the stage layer); else the bias
// direction decides READY_* (gate allowed, only the price trigger pending)
// vs WATCH_* (gate failed); no bias with a gate still open → "" (no
// directional embryo — NO_SETUP territory, wait_state stays empty).
func DeriveWaitStateFromGate(waitBias string, gs *GateState) string {
	if gs == nil {
		return ""
	}
	if !gs.LongAllowed && !gs.ShortAllowed {
		return "BLOCKED"
	}
	switch waitBias {
	case "long":
		if gs.LongAllowed {
			return "READY_LONG"
		}
		return "WATCH_LONG"
	case "short":
		if gs.ShortAllowed {
			return "READY_SHORT"
		}
		return "WATCH_SHORT"
	}
	return ""
}

// MinPositionSizeDefaultUSDT mirrors the executor's enforceMinPositionSize
// fallback (trader/auto_trader_risk.go: min_position_size ≤ 0 → 12 USDT).
// The BINDING minimum for a planned open is the strategy-config
// min_position_size (web strategy page) — the signal layer reads the live
// config value and falls back to this default only when the config is unset.
// Binance's own exchange minimum (binance GetMinNotional, 10 USDT) is a
// separate, weaker check further down the order path. Keep defaults in sync.
const MinPositionSizeDefaultUSDT = 12.0

// EffectiveMinPositionSize resolves the binding minimum opening notional from
// the strategy config (risk_control.min_position_size, web strategy page);
// ≤0/unset falls back to MinPositionSizeDefaultUSDT. Single source shared by
// the decision validator and the snapshot min_size block — a hardcoded 12 in
// the validator once rejected an 8.66 USDT opening the strategy (min 5) had
// told the model was legal.
func (e *StrategyEngine) EffectiveMinPositionSize() float64 {
	if v := e.config.RiskControl.MinPositionSize; v > 0 {
		return v
	}
	return MinPositionSizeDefaultUSDT
}

// ValidDecisionStages is the setup lifecycle enum (⑯): NO_SETUP = nothing
// forming; WATCH = setup forming, conditions tracked; READY = conditions met,
// waiting for the trigger; TRIGGERED = firing the entry/exit now;
// IN_POSITION = already holding, managing; EXIT = leaving.
var ValidDecisionStages = []string{
	"NO_SETUP", "WATCH", "READY", "TRIGGERED", "IN_POSITION", "EXIT",
}

// ValidBlockingFactors is the closed vocabulary for Decision.BlockingFactors
// — machine-aggregatable no-trade reasons (the Chinese free-text array stays
// for humans). Anything else is stripped at validation.
var ValidBlockingFactors = []string{
	"RR_LOW", "ANCHOR_SUPPRESSED", "TIMING_GATE", "BREAKOUT_UNCONFIRMED",
	"RANGE_NO_DIRECTION", "CONFLICT_UNRESOLVED", "CROWDING_HIGH",
	"LOSS_STREAK_BAN", "VOL_EXTREME", "DATA_INSUFFICIENT", "MIN_SIZE",
	"STRUCTURE_CONFLICT", "WAIT_PULLBACK", "VENDOR_DIVERGENCE", "POOR_HISTORY",
	"EXTENDED_PUMP", "CONSENSUS_OPPOSED", "MARKET_CLOSED",
	// ALL_CANDIDATES_HARD_BLOCKED: the regime-skip SYNTHESIZED wait (2026-
	// 09-27 data-quality fix). The old tag CONFLICT_UNRESOLVED misdescribed
	// these cycles — the actual blockers are RR_MAX/STOP_PLAN/VENDOR_DIVERGENCE/
	// MICRO_TREND/POOR_HISTORY etc. (per-coin, preserved in the compressed
	// prompt lines and gate_shadow_blocks), not directional conflict. A
	// dedicated code keeps the blocker statistics meaningful.
	"ALL_CANDIDATES_HARD_BLOCKED",
}

// ValidWaitStates is the per-coin trading state machine (user review
// 2026-09-15 point 11): "wait" alone carries too little information.
// BLOCKED = one or more hard gates fail and the near-term path needs more
// than a tick (per hard_entry_gate.failed); WATCH_* = the directional read
// stands but an entry blocker must clear first; READY_* = every hard gate
// passes, only the price/trigger event is missing. Meaningful on wait
// decisions only; it maps onto decision_stage via WaitStateToStage.
var ValidWaitStates = []string{"BLOCKED", "WATCH_LONG", "WATCH_SHORT", "READY_LONG", "READY_SHORT"}

// IsValidWaitState reports whether s is in ValidWaitStates.
func IsValidWaitState(s string) bool {
	for _, v := range ValidWaitStates {
		if s == v {
			return true
		}
	}
	return false
}

// WaitStateToStage maps a declared wait_state onto the existing
// decision_stage enum (READY_*→READY, WATCH_*→WATCH; BLOCKED→NO_SETUP only
// when no direction survives the block, otherwise→WATCH) — an explicit
// declaration overrides the DeriveWaitStage fallback so the two never
// contradict in the dataset.
func WaitStateToStage(waitState, waitBias string) string {
	switch waitState {
	case "READY_LONG", "READY_SHORT":
		return "READY"
	case "WATCH_LONG", "WATCH_SHORT":
		return "WATCH"
	case "BLOCKED":
		if waitBias == "long" || waitBias == "short" {
			return "WATCH" // direction stands under a hard block — still a watch, not a no-setup
		}
		return "NO_SETUP"
	}
	return ""
}

// DeriveWaitStage derives the lifecycle stage of a wait decision from its
// direction bias and blockers (schema-redundancy audit 09-13): no bias = no
// directional embryo (NO_SETUP); a bias with only the soft "wait for price"
// blocker = READY; any substantive blocker = WATCH. The model no longer
// declares the stage on waits — one less field to keep consistent.
func DeriveWaitStage(waitBias string, blockingFactors []string) string {
	if waitBias == "" {
		return "NO_SETUP"
	}
	for _, b := range blockingFactors {
		if b != "WAIT_PULLBACK" {
			return "WATCH"
		}
	}
	return "READY"
}

// DeriveDecisionStage resolves the lifecycle stage of a NON-wait decision
// from (action, hasPosition) — a pure lookup (schema-redundancy audit
// 09-16): both inputs are backend-known before the model speaks, so the
// stage carries zero model judgment and is no longer requested as output.
// open_* → TRIGGERED; close_*/partial_close_* → EXIT; adjust_stop_loss and
// hold-on-a-position → IN_POSITION; hold without a position → NO_SETUP.
// wait keeps DeriveWaitStage (wait_bias + blocking_factors).
func DeriveDecisionStage(action string, hasPosition bool) string {
	switch {
	case strings.HasPrefix(action, "open_"):
		return "TRIGGERED"
	case strings.HasPrefix(action, "close_") || strings.HasPrefix(action, "partial_close_"):
		return "EXIT"
	case action == "adjust_stop_loss":
		return "IN_POSITION"
	case action == "hold" && hasPosition:
		return "IN_POSITION"
	default:
		return "NO_SETUP"
	}
}

// NormalizeBlockingFactors drops tags outside the vocabulary (keeps order,
// dedupes) and returns the cleaned slice.
// ValidManagementFlags is the closed vocabulary for Decision.ManagementFlags.
var ValidManagementFlags = []string{
	"BREAKEVEN_WARRANTED", "PARTIAL_WARRANTED", "TRAIL_SUFFICIENT",
	"TREND_INTACT", "STRUCTURE_WEAKENING", "CHOP_RISK", "VOL_SPIKE", "EVENT_RISK",
}

// NormalizeManagementFlags drops tags outside the vocabulary (keeps order,
// dedupes) and returns the cleaned slice.
func NormalizeManagementFlags(tags []string) []string {
	valid := map[string]bool{}
	for _, v := range ValidManagementFlags {
		valid[v] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if valid[t] && !seen[t] {
			out = append(out, t)
			seen[t] = true
		}
	}
	return out
}

func NormalizeBlockingFactors(tags []string) []string {
	valid := map[string]bool{}
	for _, v := range ValidBlockingFactors {
		valid[v] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if valid[t] && !seen[t] {
			out = append(out, t)
			seen[t] = true
		}
	}
	return out
}

// Decision AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // Standard: "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	// Grid actions: "place_buy_limit", "place_sell_limit", "cancel_order", "cancel_all_orders", "pause_grid", "resume_grid", "adjust_grid"

	// Opening position parameters
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// Grid trading parameters
	Price      float64 `json:"price,omitempty"`       // Limit order price (for grid)
	Quantity   float64 `json:"quantity,omitempty"`    // Order quantity (for grid)
	LevelIndex int     `json:"level_index,omitempty"` // Grid level index
	OrderID    string  `json:"order_id,omitempty"`    // Order ID (for cancel)

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`
	// ⑮ Mandatory for hold/wait: WHY no trade (micro trend range, RR short,
	// crowding, circuit breaker…). Powers the no-trade statistics loop.
	NoTradeReasons []string `json:"no_trade_reason,omitempty"`
	// ⑯ Setup lifecycle annotation for every decision (see ValidDecisionStages).
	Stage string `json:"decision_stage,omitempty"`
	// CloseFraction: partial_close_* 的平仓比例, (0, 0.5]——每仓位累计
	// ≤75%(程序强制), 全平请用 close_*。
	CloseFraction float64 `json:"close_fraction,omitempty"`
	// EntryQuality is the model's self-assessed entry quality (0-100) for its
	// preferred direction — the raw datum of the quality→outcome backtest
	// dataset (user 2026-09-11: PF 0.76 means analysis quality is an
	// unproven hypothesis; every decision must produce measurable data).
	EntryQuality *int `json:"entry_quality,omitempty"`
	// ManagementQuality is the model's self-assessed hold quality (0-100)
	// on open-position decisions — the mirror of entry_quality for the
	// position-management backtest ("did 'keep holding' have predictive
	// value"). Required by prompt contract on IN_POSITION holds.
	ManagementQuality *int `json:"management_quality,omitempty"`
	// ManagementFlags names what the model judged about the open position,
	// FIXED enum tags (ValidManagementFlags): when BREAKEVEN_WARRANTED /
	// PARTIAL_WARRANTED the model should emit adjust_stop_loss /
	// partial_close_* instead of hold — hold+flag means "judged, not acting
	// now" (program ladder owns it).
	ManagementFlags []string `json:"management_flags,omitempty"`
	// BlockingFactors names the objective blockers with FIXED enum tags
	// (ValidBlockingFactors) so no-trade reasons aggregate machine-wise —
	// the free-text no_trade_reason stays for humans.
	BlockingFactors []string `json:"blocking_factors,omitempty"`
	// WaitBias separates "no valid entry" from "no directional edge" on wait
	// decisions (user taxonomy 2026-09-11): "long"/"short" = direction
	// identified, entry not compliant yet (WAIT_LONG/WAIT_SHORT); "" = no
	// edge (true NO TRADE). Directional verdicts live here and in
	// directional_score — never re-phrased inside no_trade_reason.
	WaitBias string `json:"wait_bias,omitempty"`
	// MarketDegraded (internal, never serialized): the decision arrived as a
	// market open and the trader's marketExceptionGate rewrote it to the
	// anchor limit for lacking exception evidence. The limit path's
	// crossed-anchor→market conversion must NOT fire for it — that would
	// silently turn the degraded order back into the market chase the gate
	// just refused (B1, QUANT_REVIEW 2026-09-22).
	MarketDegraded bool `json:"-"`
	// CycleReservedRiskUSD (internal, never serialized): the stop-risk this
	// open booked against the account exposure cap once it survived the
	// whole hard-gate chain (R8, QUANT_REVIEW 09-26). Execution failure
	// releases it.
	CycleReservedRiskUSD float64 `json:"-"`
	// WaitState + NextTrigger complete the trade state machine (review
	// 2026-09-15 points 11/12): wait_state ∈ ValidWaitStates, and next_trigger
	// is ONE sentence naming the required event and ending with the mandate
	// that ALL hard gates are re-checked when it fires — a trigger event is
	// never permission to trade.
	WaitState   string `json:"wait_state,omitempty"`
	NextTrigger string `json:"next_trigger,omitempty"`
	// TPOption: 1-based index into rr_scan.tp_options (user directive
	// 2026-09-29) — the model CHOOSES a program-precomputed take-profit plan
	// (near/mid/far structural level with its RR); the price itself is never
	// model-authored. 0/absent = option 1 (the legacy first_rr_ge_target).
	// resolveTakeProfitOptions applies it before the TP snap.
	TPOption int `json:"tp_option,omitempty"`
	// ExitMode selects the exit template the program executes for the
	// position: "trend" (default — split TP + trailing runner, today's
	// ladder), "range" (full TP at the chosen target, no runner), "quick"
	// (full TP + program time stop on dead trades). Qualitative regime
	// classification is the model's; every parameter behind it is code.
	ExitMode string `json:"exit_mode,omitempty"`
}

// FullDecision AI's complete decision (including chain of thought)
type FullDecision struct {
	SystemPrompt        string     `json:"system_prompt"`
	UserPrompt          string     `json:"user_prompt"`
	CoTTrace            string     `json:"cot_trace"`
	Decisions           []Decision `json:"decisions"`
	RawResponse         string     `json:"raw_response"`
	Timestamp           time.Time  `json:"timestamp"`
	AIRequestDurationMs int64      `json:"ai_request_duration_ms,omitempty"`
}

// QuantData quantitative data structure (fund flow, position changes, price changes)
type QuantData struct {
	Symbol      string                    `json:"symbol"`
	Price       float64                   `json:"price"`
	Netflow     *NetflowData              `json:"netflow,omitempty"`
	OI          map[string]*OIData        `json:"oi,omitempty"`
	PriceChange map[string]float64        `json:"price_change,omitempty"`
	Liquidation *market.LiquidationWindow `json:"liquidation,omitempty"`
	News        []openbb.NewsItem         `json:"news,omitempty"`
}

type NetflowData struct {
	Institution *FlowTypeData `json:"institution,omitempty"`
	Personal    *FlowTypeData `json:"personal,omitempty"`
}

type FlowTypeData struct {
	Future map[string]float64 `json:"future,omitempty"`
	Spot   map[string]float64 `json:"spot,omitempty"`
}

type OIData struct {
	CurrentOI float64                 `json:"current_oi"`
	Delta     map[string]*OIDeltaData `json:"delta,omitempty"`
}

type OIDeltaData struct {
	OIDelta        float64 `json:"oi_delta"`
	OIDeltaValue   float64 `json:"oi_delta_value"`
	OIDeltaPercent float64 `json:"oi_delta_percent"`
}

// ============================================================================
// StrategyEngine - Core Strategy Execution Engine
// ============================================================================

// StrategyEngine strategy execution engine
type StrategyEngine struct {
	config       *store.StrategyConfig
	vergexClient *vergex.Client
}

// NewStrategyEngine creates strategy execution engine.
func NewStrategyEngine(config *store.StrategyConfig) *StrategyEngine {
	return &StrategyEngine{
		config:       config,
		vergexClient: vergex.NewClient(),
	}
}

// GetRiskControlConfig gets risk control configuration
func (e *StrategyEngine) GetRiskControlConfig() store.RiskControlConfig {
	return e.config.RiskControl
}

// GetLanguage returns the language from config or falls back to auto-detection
func (e *StrategyEngine) GetLanguage() Language {
	switch e.config.Language {
	case "zh":
		return LangChinese
	case "en":
		return LangEnglish
	default:
		// Fall back to auto-detection from prompt content for backward compatibility
		return detectLanguage(e.config.PromptSections.RoleDefinition)
	}
}

// GetConfig gets complete strategy configuration
func (e *StrategyEngine) GetConfig() *store.StrategyConfig {
	return e.config
}

// ============================================================================
// Candidate Coins
// ============================================================================

// GetCandidateCoins gets candidate coins based on strategy configuration
// appendUniqueSource tags a candidate with a universe source, skipping exact
// duplicates — one coin passing two sources keeps both tags, but the same
// source fetched twice must not double-render in Sources (A5,
// QUANT_REVIEW_2026-09-22).
func appendUniqueSource(sources []string, tag string) []string {
	for _, s := range sources {
		if s == tag {
			return sources
		}
	}
	return append(sources, tag)
}

// staticCoinCandidates expands the configured static list with per-symbol
// dedup (09-28 review P3): a repeated entry in custom_coins rendered twice
// in the prompt's candidate section — duplicated market data, duplicated
// decision budget. Empty normalized symbols are skipped.
func (e *StrategyEngine) staticCoinCandidates(coinSource store.CoinSourceConfig) []CandidateCoin {
	candidates := make([]CandidateCoin, 0, len(coinSource.StaticCoins))
	seen := make(map[string]bool, len(coinSource.StaticCoins))
	for _, raw := range coinSource.StaticCoins {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		symbol := market.Normalize(raw)
		if seen[symbol] {
			continue
		}
		seen[symbol] = true
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"static"},
		})
	}
	return candidates
}

func (e *StrategyEngine) GetCandidateCoins() ([]CandidateCoin, error) {
	var candidates []CandidateCoin
	symbolSources := make(map[string][]string)

	coinSource := e.config.CoinSource

	switch coinSource.SourceType {
	case "static":
		candidates = append(candidates, e.staticCoinCandidates(coinSource)...)

		return e.filterExcludedCoins(candidates), nil

	case "ai500":
		// Check use_ai500 flag; if false, fall back to static coins
		if !coinSource.UseAI500 {
			logger.Infof("⚠️  source_type is 'ai500' but use_ai500 is false, falling back to static coins")
			candidates = append(candidates, e.staticCoinCandidates(coinSource)...)
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getAI500Coins(coinSource.AI500Limit)
		if err != nil {
			return nil, err
		}
		// Empty list is a normal condition, return directly
		return e.filterExcludedCoins(coins), nil

	case "oi_top":
		// Check use_oi_top flag; if false, fall back to static coins
		if !coinSource.UseOITop {
			logger.Infof("⚠️  source_type is 'oi_top' but use_oi_top is false, falling back to static coins")
			candidates = append(candidates, e.staticCoinCandidates(coinSource)...)
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getOITopCoins(coinSource.OITopLimit)
		if err != nil {
			return nil, err
		}
		// Empty list is a normal condition, return directly
		return e.filterExcludedCoins(coins), nil

	case "oi_low":
		// OI decrease ranking, suitable for short positions
		if !coinSource.UseOILow {
			logger.Infof("⚠️  source_type is 'oi_low' but use_oi_low is false, falling back to static coins")
			candidates = append(candidates, e.staticCoinCandidates(coinSource)...)
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getOILowCoins(coinSource.OILowLimit)
		if err != nil {
			return nil, err
		}
		// Empty list is a normal condition, return directly
		return e.filterExcludedCoins(coins), nil

	case "piggy_dash":
		// 猪猪冲刺: strongest breakout/breakdown signals from the 5-min engine.
		// Note: unlike ai500/oi_*, there is no separate enable gate — selecting
		// this source type IS the switch.
		coins, err := e.getPiggyDashCoins(coinSource.PiggyDashLimit, coinSource.PiggyDashDirection)
		if err != nil {
			return nil, err
		}
		return e.filterExcludedCoins(coins), nil

	case "short_scan":
		// 做空扫描: top 24h gainers ranked by short-suitability score.
		// Like piggy_dash, selecting this source type IS the switch.
		coins, err := e.getShortScanCoins(coinSource.ShortScanLimit, coinSource.EffectiveMinOIMillions(), coinSource.EffectiveShortScanMinScore(),
			coinSource.ShortScanHistoryDays, coinSource.ShortScanHistoryMax)
		if err != nil {
			return nil, err
		}
		return e.filterExcludedCoins(coins), nil

	case "hyper_all":
		// All Hyperliquid perp coins
		if !coinSource.UseHyperAll {
			logger.Infof("⚠️  source_type is 'hyper_all' but use_hyper_all is false, falling back to static coins")
			candidates = append(candidates, e.staticCoinCandidates(coinSource)...)
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getHyperAllCoins()
		if err != nil {
			return nil, err
		}
		return e.filterExcludedCoins(coins), nil

	case "hyper_main":
		// Top N Hyperliquid coins by 24h volume
		if !coinSource.UseHyperMain {
			logger.Infof("⚠️  source_type is 'hyper_main' but use_hyper_main is false, falling back to static coins")
			candidates = append(candidates, e.staticCoinCandidates(coinSource)...)
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getHyperMainCoins(coinSource.HyperMainLimit)
		if err != nil {
			return nil, err
		}
		return e.filterExcludedCoins(coins), nil

	case "mixed":
		if coinSource.UseAI500 {
			poolCoins, err := e.getAI500Coins(coinSource.AI500Limit)
			if err != nil {
				logger.Infof("⚠️  Failed to get AI500 coins: %v", err)
			} else {
				for _, coin := range poolCoins {
					symbolSources[coin.Symbol] = appendUniqueSource(symbolSources[coin.Symbol], "ai500")
				}
			}
		}

		if coinSource.UseOITop {
			oiCoins, err := e.getOITopCoins(coinSource.OITopLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Top: %v", err)
			} else {
				for _, coin := range oiCoins {
					symbolSources[coin.Symbol] = appendUniqueSource(symbolSources[coin.Symbol], "oi_top")
				}
			}
		}

		if coinSource.UseOILow {
			oiLowCoins, err := e.getOILowCoins(coinSource.OILowLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Low: %v", err)
			} else {
				for _, coin := range oiLowCoins {
					symbolSources[coin.Symbol] = appendUniqueSource(symbolSources[coin.Symbol], "oi_low")
				}
			}
		}

		// Short candidates: the same pre-scored "pump quality + contract
		// overheating + topping confirmation" engine the short_scan source
		// type uses. Without this the mixed pool is long-only (AI500/OI-top/
		// piggy-dash are all up-side selectors) and the model drifts long.
		// Fetched ONCE — a previous shape called getShortScanCoins three
		// times (universe assembly twice + shortMeta below), double-tagging
		// merged symbols' Sources with duplicate "short_scan" entries
		// (QUANT_REVIEW_2026-09-22 A5).
		var shortCoins []CandidateCoin
		if coinSource.UseShortScan {
			sc, err := e.getShortScanCoins(coinSource.ShortScanLimit, coinSource.EffectiveMinOIMillions(), coinSource.EffectiveShortScanMinScore(),
				coinSource.ShortScanHistoryDays, coinSource.ShortScanHistoryMax)
			if err != nil {
				logger.Infof("⚠️  Failed to get short-scan coins: %v", err)
			} else {
				shortCoins = sc
				for _, coin := range sc {
					symbolSources[coin.Symbol] = appendUniqueSource(symbolSources[coin.Symbol], "short_scan")
				}
			}
		}

		if coinSource.UseHyperAll {
			hyperCoins, err := e.getHyperAllCoins()
			if err != nil {
				logger.Infof("⚠️  Failed to get Hyperliquid All coins: %v", err)
			} else {
				for _, coin := range hyperCoins {
					symbolSources[coin.Symbol] = appendUniqueSource(symbolSources[coin.Symbol], "hyper_all")
				}
			}
		}

		if coinSource.UseHyperMain {
			hyperMainCoins, err := e.getHyperMainCoins(coinSource.HyperMainLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get Hyperliquid Main coins: %v", err)
			} else {
				for _, coin := range hyperMainCoins {
					symbolSources[coin.Symbol] = appendUniqueSource(symbolSources[coin.Symbol], "hyper_main")
				}
			}
		}

		var piggyCoins []CandidateCoin
		if coinSource.UsePiggyDash {
			// Single fetch per cycle (2026-10-03 review): the stale-snapshot
			// path pays a synchronous refresh — calling getPiggyDashCoins
			// twice (fan-in + piggyMeta below) doubled that cost on failure.
			// 2026-10-04 review #6: this used to be `piggyCoins, piggyErr := …`
			// — := redeclared BOTH names inside the if-block, the outer
			// piggyCoins stayed nil, piggyMeta below stayed empty, and
			// ScannerDirection/SCANNER_VS_SCANNER never fired in mixed (the
			// production主力 source). Declare the error outside, assign with =.
			var piggyErr error
			piggyCoins, piggyErr = e.getPiggyDashCoins(coinSource.PiggyDashLimit, coinSource.PiggyDashDirection)
			if piggyErr != nil {
				logger.Infof("⚠️  Failed to get Piggy Dash coins: %v", piggyErr)
			} else {
				for _, coin := range piggyCoins {
					symbolSources[coin.Symbol] = appendUniqueSource(symbolSources[coin.Symbol], "piggy_dash")
				}
			}
		}

		// Static coins join the mixed pool when listed. Legacy configs
		// (UseStatic unset) always included them; an explicit false opts out.
		includeStatic := coinSource.UseStatic == nil || *coinSource.UseStatic
		if includeStatic {
			for _, symbol := range coinSource.StaticCoins {
				// Skip blanks: Normalize("") yields "USDT", which would enter
				// the pool as a phantom candidate (2026-10-03 review).
				if strings.TrimSpace(symbol) == "" {
					continue
				}
				symbol = market.Normalize(symbol)
				if _, exists := symbolSources[symbol]; !exists {
					symbolSources[symbol] = []string{"static"}
				} else {
					symbolSources[symbol] = appendUniqueSource(symbolSources[symbol], "static")
				}
			}
		}

		// Short-scan metadata (score/grade/reasons) must survive the
		// symbolSources collapse — it labels the direction hints in the prompt.
		// Reuses the fetch from the universe assembly above (A5 dedupe).
		shortMeta := make(map[string]CandidateCoin)
		for _, c := range shortCoins {
			shortMeta[c.Symbol] = c
		}
		// Piggy-dash direction metadata survives the collapse too — the
		// cross-scanner conflict check needs it (audit 2026-09-12 #11).
		piggyMeta := make(map[string]CandidateCoin)
		for _, c := range piggyCoins {
			piggyMeta[c.Symbol] = c
		}
		for symbol, sources := range symbolSources {
			c := CandidateCoin{Symbol: symbol, Sources: sources}
			if meta, ok := shortMeta[symbol]; ok {
				c.ShortScore = meta.ShortScore
				c.ShortGrade = meta.ShortGrade
				// 2026-10-04 batch-2: the top-confirmation gate reads this —
				// dropping it here would silently gate every mixed-pool
				// short into SHORT_TOP_CONFIRM_MISSING.
				c.ShortConfirmed = meta.ShortConfirmed
				c.ShortReasons = meta.ShortReasons
				c.ShortFundingAnn = meta.ShortFundingAnn
				c.ShortUniverse = meta.ShortUniverse
				// 2026-10-03 review P1: the mixed collapse dropped
				// NearHighAlso (the 09-17 gainer-vs-grinding-top OI exemption
				// died again for collision coins — mixed is the production
				//主力 source) and ShortScanAtMs (generated_at_utc vanished).
				c.NearHighAlso = meta.NearHighAlso
				c.ShortScanAtMs = meta.ShortScanAtMs
			}
			if meta, ok := piggyMeta[symbol]; ok && meta.ScannerDirection != "" {
				c.ScannerDirection = meta.ScannerDirection
			}
			c.ScannerConflict = hasScannerConflict(c.Sources, c.ScannerDirection)
			candidates = append(candidates, c)
		}
		sortMixedCandidates(candidates)
		return e.filterExcludedCoins(candidates), nil

	default:
		return nil, fmt.Errorf("unknown coin source type: %s", coinSource.SourceType)
	}
}

// sortMixedCandidates gives the mixed pool a deterministic order (2026-10-07
// review: the map-collapse emitted a random order every cycle — prompt order
// shifted LLM position bias, broke run-to-run reproducibility and prompt-cache
// reuse). Multi-source confluence first, then symbol.
func sortMixedCandidates(c []CandidateCoin) {
	sort.SliceStable(c, func(i, j int) bool {
		if len(c[i].Sources) != len(c[j].Sources) {
			return len(c[i].Sources) > len(c[j].Sources)
		}
		return c[i].Symbol < c[j].Symbol
	})
}

// hasScannerConflict reports whether a candidate carries OPPOSITE
// directional conclusions from the two scanners: short_scan is inherently
// short; piggy_dash "up" against it is a SCANNER_VS_SCANNER conflict the
// model must resolve explicitly (audit 2026-09-12 #11).
func hasScannerConflict(sources []string, piggyDirection string) bool {
	// 2026-10-03 review P1: piggy ScannerDirection carries the production
	// literals "breakout"(=up)/"breakdown"(=down) (breakout.DirUp/DirDown) —
	// comparing against "up" made this conflict detector dead since launch.
	// Only an UP-direction piggy read contradicts the short scanner.
	if piggyDirection != "breakout" {
		return false
	}
	var shortScan, piggy bool
	for _, s := range sources {
		switch s {
		case "short_scan":
			shortScan = true
		case "piggy_dash":
			piggy = true
		}
	}
	return shortScan && piggy
}

// filterExcludedCoins removes excluded coins from the candidates list
func (e *StrategyEngine) filterExcludedCoins(candidates []CandidateCoin) []CandidateCoin {
	// XYZ-prefixed symbols (Hyperliquid "xyz:" listings) are tokenized
	// stock/commodity perps (SKHX, XAU, CL, ...). Their derivatives metrics
	// (funding, long/short accounts) don't map to the crypto market context
	// the strategies reason over — drop them at the single choke point every
	// coin source passes through.
	filtered0 := make([]CandidateCoin, 0, len(candidates))
	for _, c := range candidates {
		if strings.Contains(strings.ToUpper(c.Symbol), ":") {
			logger.Infof("🚫 Excluded XYZ (tokenized stock/commodity) symbol: %s", c.Symbol)
			continue
		}
		filtered0 = append(filtered0, c)
	}
	candidates = filtered0

	if len(e.config.CoinSource.ExcludedCoins) == 0 {
		return candidates
	}

	// Build excluded set for O(1) lookup
	excluded := make(map[string]bool)
	for _, coin := range e.config.CoinSource.ExcludedCoins {
		normalized := market.Normalize(coin)
		excluded[normalized] = true
	}

	// Filter out excluded coins
	filtered := make([]CandidateCoin, 0, len(candidates))
	for _, c := range candidates {
		if !excluded[c.Symbol] {
			filtered = append(filtered, c)
		} else {
			logger.Infof("🚫 Excluded coin: %s", c.Symbol)
		}
	}

	return filtered
}

// piggyDashMaxAge: how long a non-empty piggy-dash snapshot stays eligible
// to FEED NEW TRADES (F20, 2026-10-01 review). The scan cadence is 5 minutes
// (scheduler Interval), so anything older than two full scans means the
// background scanner has been failing — the old list used to be consumed
// forever, silently decoupling "why these symbols" from reality. Display of
// an old board (Data page) is unaffected; only candidate selection checks.
const piggyDashMaxAge = 12 * time.Minute

// getPiggyDashCoins returns the strongest breakout-engine signals (猪猪冲刺).
// Data comes exclusively from the breakout scheduler, which computes
// everything from Binance fapi endpoints (klines / OI / depth / funding) —
// no third-party ranking feeds involved. On a cold start (server just
// booted, first 5-min scan still running) we trigger a synchronous refresh
// and re-read instead of falling back to external rankings. On a STALE
// snapshot (background scan failing > piggyDashMaxAge) one synchronous
// refresh is attempted, and its failure HALTS this source for the cycle —
// an old board must not keep minting entries.
func (e *StrategyEngine) getPiggyDashCoins(limit int, direction string) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 5
	}
	scheduler := breakout.DefaultScheduler()
	// Over-fetch 3× so the quality floor can decline weak rows and still
	// fill the configured slot count from the rest of the ranking.
	fetch := limit * 3
	symbols, age := scheduler.TopSymbolsWithDirectionAge(fetch, direction)
	// 2026-10-03 review P2: distinguish "empty board" from "direction has no
	// match on a FRESH board" — the old condition re-ran a full synchronous
	// scan every cycle just to confirm the direction filter still matches
	// nothing. Refresh only on staleness or a genuinely empty board.
	anyCount, _ := scheduler.TopSymbolsWithDirectionAge(fetch, "")
	directionEmpty := len(anyCount) == 0
	if directionEmpty || age > piggyDashMaxAge {
		// review 2026-10-07 S9: only a FRESH board may take this early return;
		// a stale board with no match must fall through to the refresh below.
		if !directionEmpty && len(symbols) == 0 && age <= piggyDashMaxAge {
			logger.Infof("🐷 Piggy-dash board fresh but no %q match — not rescanning", direction)
			return nil, fmt.Errorf("piggy-dash: no %q symbols on a fresh board", direction)
		}
		if !directionEmpty {
			logger.Infof("🐷 Piggy-dash snapshot is stale (%s old > %s) — refreshing synchronously", age.Round(time.Second), piggyDashMaxAge)
		} else {
			logger.Infof("🐷 Piggy-dash snapshot cold — running synchronous Binance scan")
		}
		scheduler.RefreshNow(60 * time.Second)
		symbols, age = scheduler.TopSymbolsWithDirectionAge(fetch, direction)
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("piggy-dash scan produced no signals (Binance data unavailable)")
	}
	if age > piggyDashMaxAge {
		// The synchronous refresh failed too — halt this SOURCE for the
		// cycle rather than minting candidates off a dead board.
		return nil, fmt.Errorf("piggy-dash snapshot stale (%s old, refresh failed) — source halted this cycle", age.Round(time.Second))
	}
	// Quality floor (review 2026-10-04 #4): the snapshot is a RANKING, not a
	// filter — in a dead market noise-graded rows and not-yet-crossed
	// approach patterns filled every slot. weak+ grade on a real cross
	// pattern only; an empty result is an ERROR (the source halts for the
	// cycle and the kernel's regime-skip synthesizes the wait) — never a
	// quiet backfill of weak rows.
	picked := make([]breakout.SymbolDirection, 0, limit)
	for _, row := range symbols {
		if row.Grade == "noise" || row.Pattern == "" || row.Pattern == breakout.PatternApproach {
			continue
		}
		picked = append(picked, row)
		if len(picked) >= limit {
			break
		}
	}
	if len(picked) == 0 {
		return nil, fmt.Errorf("piggy-dash: %d rows on the board, none above the weak+cross floor — pool empty this cycle", len(symbols))
	}
	logger.Infof("🐷 Piggy-dash source: %d symbols (direction=%q, floor %d/%d rows)", len(picked), direction, len(picked), len(symbols))
	var candidates []CandidateCoin
	for _, row := range picked {
		candidates = append(candidates, CandidateCoin{
			Symbol:           row.Symbol,
			Sources:          []string{"piggy_dash"},
			ScannerDirection: row.Direction,
		})
	}
	return candidates, nil
}

// dedupeShortCandidates keeps one entry per symbol (the higher ShortScore;
// first wins ties) preserving the relative order of the survivors. The
// history pool and breakdown universe can emit the same symbol twice.
func dedupeShortCandidates(in []CandidateCoin) []CandidateCoin {
	best := make(map[string]int, len(in))
	for i, c := range in {
		if j, ok := best[c.Symbol]; !ok || c.ShortScore > in[j].ShortScore {
			best[c.Symbol] = i
		}
	}
	out := make([]CandidateCoin, 0, len(best))
	for i, c := range in {
		if best[c.Symbol] == i {
			out = append(out, c)
		}
	}
	return out
}

// getShortScanCoins returns the top 24h gainers ranked by short-suitability
// (做空扫描). Data is 100% Binance-derived: the 24h gainer list comes from
// fapi ticker and each candidate is scored on RSI exhaustion, EMA extension,
// upper-wick rejection, volume fade, funding crowding and OI build-up by
// breakout.AnalyzeShort. The gainer history pool (历史涨幅池) merges the
// past N days' recorded top-gainer snapshots back into the universe, so
// coins that pumped days ago and have since rolled over — already off
// today's board — still get scored.
// The strategy's min OI value threshold is applied BEFORE taking the top-N:
// the full ranked list is fetched, low-OI symbols are dropped, then the
// strongest remaining short setups fill the candidate slots.
// histDays/histMax are the RAW strategy config values; they sync into the
// scanner here every cycle (UI saves take effect next cycle), because the
// scheduler shares one scan cache and must resolve the same universe.
func (e *StrategyEngine) getShortScanCoins(limit int, minOIMillions, minScore float64, histDays, histMax int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 5
	}
	if minScore <= 0 {
		// Defensive: callers pass coinSource.EffectiveShortScanMinScore().
		minScore = store.DefaultShortScanMinScore
	}
	if minOIMillions <= 0 {
		// Defensive: callers should pass coinSource.EffectiveMinOIMillions();
		// this keeps the built-in floor for any direct call.
		minOIMillions = store.DefaultMinOIValueMillions
	}
	// Full ranked universe (cached per resolved config — A4), so the OI
	// filter happens before the top-N cut instead of wasting slots on coins
	// that would be skipped later. The strategy's history knobs travel as
	// parameters; no process-global state.
	signals, scanAt, err := breakout.ScanShorts(breakout.ShortScanUniverse, histDays, histMax)
	if err != nil {
		return nil, fmt.Errorf("short scan failed: %w", err)
	}
	var candidates []CandidateCoin
	var bestNearHigh *CandidateCoin // strongest grinding-top setup, reserved a slot
	var skipped []string
	var floorSkipped []string
	for _, sig := range signals {
		// The near_high universe passed a $30M/day liquidity prefilter at
		// scan time; its OI is structurally lower (low OI = less squeeze
		// fuel) and the prompt's funding-crowding exemption assumes these
		// candidates actually reach the AI — so the gainer-side OI floor
		// must not silently drop the whole universe (it did: every
		// near_high symbol sat below 15M OI and never appeared).
		if sig.Universe == "near_high" {
			c := shortSignalToCandidate(sig, scanAt)
			if bestNearHigh == nil || c.ShortScore > bestNearHigh.ShortScore {
				bestNearHigh = &c
			}
			continue
		}
		// Quality floor (review 2026-10-04 #4): the scan is a ranking — in a
		// dead market noise-graded rows (score < 40) filled slots and the AI
		// faced only weak evidence. weak+ only; the near_high slot above has
		// its own ≥55 bar. Applies to every universe (gainer/hist/breakdown)
		// — a sub-noise breakdown signal is not a continuation setup either.
		// review 2026-10-07 S1: the journal showed score 40-55 averaging -0.43%
		// (24h short return) vs +0.5% at 55-70, so the default floor is medium+
		// (55); the strategy's short_scan_min_score overrides it.
		if sig.Score < minScore {
			floorSkipped = append(floorSkipped, fmt.Sprintf("%s(%.0f)", sig.Symbol, sig.Score))
			continue
		}
		// OI data unavailable (0) can't be judged — keep, matching the
		// market-data layer which only filters when OI is present.
		// NearHighAlso: the coin ALSO passed the grinding-top screen (whose
		// $30M/day liquidity prefilter justifies the same OI-floor exemption
		// the near_high universe gets). Without this the exemption died
		// silently whenever a grinding top also ranked in the 24h Top50 —
		// the collision kept the gainer label and the floor ate the coin
		// (user audit 2026-09-17). The label stays "gainer" on purpose: the
		// prompt's funding-crowding exemption is keyed on universe=near_high
		// and its rationale (normalized funding) does not hold for an active
		// top-50 pumper.
		if sig.OIValueMillions > 0 && sig.OIValueMillions < minOIMillions && !sig.NearHighAlso {
			skipped = append(skipped, fmt.Sprintf("%s(%.1fM)", sig.Symbol, sig.OIValueMillions))
			continue
		}
		c := shortSignalToCandidate(sig, scanAt)
		// A6 (QUANT_REVIEW 09-22): OI-unknown coins ride in the pool by the
		// fail-open rule, but they must SAY so — an unverified candidate must
		// not carry the implied "passed the liquidity floor" status.
		if sig.OIValueMillions <= 0 && !sig.NearHighAlso {
			c.ShortReasons = append(c.ShortReasons, "OI数据缺失:流动性门槛未能核验")
		}
		candidates = append(candidates, c)
	}
	// US-session equity weighting (user directive 2026-09-23): during US
	// regular hours, US equity tokens in the pool rank ahead of the crypto
	// pool — lower realized volatility and (journal baseline, small n)
	// better win rate. Applied BEFORE the top-N cut so the weighting actually
	// changes selection, not just prompt order.
	// Dedupe BEFORE the boost/sort and the reserved near_high slot: a
	// duplicate row must not push a distinct gainer out of the top-N cut.
	candidates = dedupeShortCandidates(candidates)
	e.applyUSStockSessionBoost(candidates, time.Now())
	// Re-rank after the boost: the top-N cut below takes the slice head in
	// scan order, so a boosted equity token must actually rise in the order
	// to change selection (scan order alone is the pre-boost ranking).
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].ShortScore > candidates[j].ShortScore })

	// Reserve one slot for the strongest grinding-top setup when the gainer
	// fill left no room for it — without this the near_high universe stays
	// invisible to the AI in every cycle and its exemption is dead text.
	// Audit 2026-09-12 #6: near_high symbols are OI-exempt (structurally
	// lower liquidity, squeeze fuel) — the reserved slot requires at least a
	// MEDIUM grade (score ≥ 55) so a marginal grinding top can't occupy a
	// slot under the same bar as liquid gainers. The reserved slot also
	// honours the strategy's short_scan_min_score (review 2026-10-11): the
	// bar is the higher of that floor and 55.
	const nearHighReservedMinScore = 55
	reservedMin := math.Max(minScore, nearHighReservedMinScore)
	if bestNearHigh != nil && bestNearHigh.ShortScore < reservedMin {
		logger.Infof("🩸 Grinding-top reserved slot skipped: %s score %.0f < %.0f (low-OI squeeze risk bar)",
			bestNearHigh.Symbol, bestNearHigh.ShortScore, reservedMin)
		bestNearHigh = nil
	}
	if bestNearHigh != nil {
		hasIt := false
		for _, c := range candidates {
			if c.Symbol == bestNearHigh.Symbol {
				hasIt = true
				break
			}
		}
		if !hasIt {
			if len(candidates) >= limit {
				candidates = candidates[:limit-1] // drop the weakest gainer for the reserved slot
			}
			logger.Infof("🩸 Reserved short-scan slot for grinding-top: %s (score %.0f)",
				bestNearHigh.Symbol, bestNearHigh.ShortScore)
			candidates = append(candidates, *bestNearHigh)
		}
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	if len(skipped) > 0 {
		logger.Infof("🩸 Short-scan OI filter (< %.1fM): skipped %d low-liquidity candidates %v",
			minOIMillions, len(skipped), skipped)
	}
	if len(floorSkipped) > 0 {
		logger.Infof("🩸 Short-scan quality floor (score<%.0f): skipped %d candidates %v",
			minScore, len(floorSkipped), floorSkipped)
	}
	var syms []string
	for _, c := range candidates {
		if c.ShortUniverse != "" && c.ShortUniverse != "gainer" {
			syms = append(syms, c.Symbol+"("+c.ShortUniverse+")")
		} else {
			syms = append(syms, c.Symbol)
		}
	}
	logger.Infof("🩸 Short-scan source: %d symbols %v", len(candidates), syms)
	return candidates, nil
}

// shortEvidenceAtMs: review 2026-10-07 S8 — slow-top (near_high) signals are
// merged into the board from a cache up to ~60 min old; stamping them with
// the board's scan time hid that. Prefer the signal's own GeneratedAt when it
// is set and older than the board scan.
func shortEvidenceAtMs(sig breakout.ShortSignal, scanAt time.Time) int64 {
	if !sig.GeneratedAt.IsZero() && sig.GeneratedAt.Before(scanAt) {
		return sig.GeneratedAt.UnixMilli()
	}
	return scanAt.UnixMilli()
}

// shortSignalToCandidate converts a scanner ShortSignal into a CandidateCoin.
func shortSignalToCandidate(sig breakout.ShortSignal, scanAt time.Time) CandidateCoin {
	c := CandidateCoin{
		Symbol:           sig.Symbol,
		Sources:          []string{"short_scan"},
		ScannerDirection: "short",
		ShortScore:       sig.Score,
		ShortGrade:       sig.Grade,
		ShortConfirmed:   sig.Confirmed,
		ShortFundingAnn:  sig.FundingAnnualPct,
		ShortScanAtMs:    shortEvidenceAtMs(sig, scanAt),
		ShortUniverse:    sig.Universe,
		NearHighAlso:     sig.NearHighAlso,
	}
	if sig.Universe == "near_high" {
		c.ShortReasons = append(c.ShortReasons, "磨顶:距90日高点<5%")
	}
	if sig.Universe == "hist_gainer" {
		// Provenance, true by construction: the coin is in this universe
		// because it recorded a top gainer day within the window and has
		// since left the live 24h board — neutral evidence, not a verdict.
		c.ShortReasons = append(c.ShortReasons, "历史涨幅池:近几日曾大涨,已淡出24h涨幅榜")
	}
	if sig.Universe == "breakdown" {
		// A1 (QUANT_REVIEW 09-22): provenance again — it is here because it
		// sits on the 24h LOSER board with a 4h downtrend; the continuation
		// composite ranked it. Still neutral evidence.
		c.ShortReasons = append(c.ShortReasons, "破位宇宙:24h跌幅榜,4h趋势向下(顺势反弹做空)")
	}
	// Keep the topping confirmations compact: the reasons list can be long.
	if sig.BearishDiv4h {
		c.ShortReasons = append(c.ShortReasons, "4h顶背离")
	}
	if sig.FakeBreakout {
		c.ShortReasons = append(c.ShortReasons, "假突破")
	}
	if sig.MABreak {
		c.ShortReasons = append(c.ShortReasons, "跌破EMA20")
	}
	if sig.FundingRollover {
		c.ShortReasons = append(c.ShortReasons, "资金费率回落")
	}
	return c
}

// applyUSStockSessionBoost multiplies US equity tokens' short scores by
// (1 + pct/100) while the US market is in regular hours, adding a visible
// provenance reason. Disabled (pct ≤ 0) or classification unavailable →
// no-op. Score caps at 100 like every other dimension.
func (e *StrategyEngine) applyUSStockSessionBoost(candidates []CandidateCoin, now time.Time) {
	if e.config == nil {
		return
	}
	pct := e.config.EffectiveUSStockSessionBoostPct()
	if pct <= 0 || !market.IsUSMarketOpen(now) {
		return
	}
	factor := 1 + pct/100
	for i := range candidates {
		if !market.IsUSEquitySymbol(candidates[i].Symbol) {
			continue
		}
		if candidates[i].ShortScore > 0 {
			candidates[i].ShortScore = round2(math.Min(100, candidates[i].ShortScore*factor))
		}
		candidates[i].ShortReasons = append(candidates[i].ShortReasons,
			fmt.Sprintf("美股盘中时段加权+%s%%(波动相对加密更低,流动性更薄注意点差)", fmtFloatPct(pct)))
	}
}

func fmtFloatPct(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64)
}

// ai500FallbackTTL bounds how old a last-good AI500 list may be when vergex
// fails: long enough to ride over a transient 403, short enough not to go stale.
const ai500FallbackTTL = 15 * time.Minute

// ai500LastGood is the most recent successful AI500 list. It is package-level,
// not per engine: the list is global and StrategyEngine is recreated per cycle.
var ai500LastGood struct {
	mu      sync.Mutex
	symbols []string
	at      time.Time
}

// Test seams for getAI500Coins: the vergex fetch and the clock.
var ai500Now = time.Now

var ai500Fetch = func(e *StrategyEngine, limit int) ([]string, error) {
	return e.vergexClient.GetAI500Symbols(limit)
}

// getAI500Coins returns AI500 picks from vergex trending — the same source as
// the data page's "AI500 Picks" card (browser-relayed payloads).
// Vergex is the only source: NofxOS public keys were deprecated server-side.
func (e *StrategyEngine) getAI500Coins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 30
	}

	symbols, err := ai500Fetch(e, limit)
	if err != nil {
		// 2026-10-09: transient vergex 403 dropped the AI500 source for a whole
		// cycle — reuse the last-good list while it is younger than the TTL.
		cached, age, ok := ai500LastGoodSymbols(limit)
		if !ok {
			return nil, err
		}
		logger.Infof("📊 AI500 (vergex) failed (%v) — using last-good list from %s ago (%d coins, TTL %s)", err, age.Round(time.Second), len(cached), ai500FallbackTTL)
		symbols = cached
	} else {
		// An empty success must not wipe a usable last-good list.
		if len(symbols) > 0 {
			ai500Remember(symbols)
		}
		logger.Infof("📊 AI500 (vergex) returned %d coins (limit %d)", len(symbols), limit)
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"ai500"},
		})
	}
	return candidates, nil
}

// ai500Remember stores a copy of a successful AI500 list and the time it arrived.
func ai500Remember(symbols []string) {
	cp := append([]string(nil), symbols...)
	ai500LastGood.mu.Lock()
	ai500LastGood.symbols = cp
	ai500LastGood.at = ai500Now()
	ai500LastGood.mu.Unlock()
}

// ai500LastGoodSymbols returns up to limit cached symbols and their age. ok is
// false when nothing is cached or the cached list is at least ai500FallbackTTL old.
func ai500LastGoodSymbols(limit int) (symbols []string, age time.Duration, ok bool) {
	ai500LastGood.mu.Lock()
	defer ai500LastGood.mu.Unlock()
	if ai500LastGood.at.IsZero() {
		return nil, 0, false
	}
	age = ai500Now().Sub(ai500LastGood.at)
	if age >= ai500FallbackTTL {
		return nil, 0, false
	}
	n := len(ai500LastGood.symbols)
	if n > limit {
		n = limit
	}
	return append([]string(nil), ai500LastGood.symbols[:n]...), age, true
}

// getOITopCoins returns the top OI-increase coins from vergex trending (1h
// window, same ranking as the data page's Open Interest top list).
// Vergex is the only source: NofxOS public keys were deprecated server-side.
func (e *StrategyEngine) getOITopCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	symbols, err := e.vergexClient.GetOITopSymbols(limit)
	if err != nil {
		return nil, err
	}
	logger.Infof("📊 OI top (vergex, 1h) returned %d coins (limit %d)", len(symbols), limit)

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_top"},
		})
	}
	return candidates, nil
}

// getOILowCoins returns the top OI-decrease coins from vergex trending (1h
// window, same ranking as the data page's Open Interest low list).
// Vergex is the only source: NofxOS public keys were deprecated server-side.
func (e *StrategyEngine) getOILowCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	symbols, err := e.vergexClient.GetOILowSymbols(limit)
	if err != nil {
		return nil, err
	}
	logger.Infof("📊 OI low (vergex, 1h) returned %d coins (limit %d)", len(symbols), limit)

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_low"},
		})
	}
	return candidates, nil
}

// getHyperAllCoins returns all available Hyperliquid perpetual coins
func (e *StrategyEngine) getHyperAllCoins() ([]CandidateCoin, error) {
	ctx := context.Background()
	symbols, err := hyperliquid.GetAllCoinSymbols(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Hyperliquid coins: %w", err)
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		// Add USDT suffix for compatibility
		normalizedSymbol := market.Normalize(symbol + "USDT")
		candidates = append(candidates, CandidateCoin{
			Symbol:  normalizedSymbol,
			Sources: []string{"hyper_all"},
		})
	}
	logger.Infof("✅ Loaded %d Hyperliquid coins (hyper_all)", len(candidates))
	return candidates, nil
}

// getHyperMainCoins returns top N Hyperliquid coins by 24h volume
func (e *StrategyEngine) getHyperMainCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 20
	}

	ctx := context.Background()
	symbols, err := hyperliquid.GetMainCoinSymbols(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get Hyperliquid main coins: %w", err)
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		// Add USDT suffix for compatibility
		normalizedSymbol := market.Normalize(symbol + "USDT")
		candidates = append(candidates, CandidateCoin{
			Symbol:  normalizedSymbol,
			Sources: []string{"hyper_main"},
		})
	}
	logger.Infof("✅ Loaded %d Hyperliquid main coins (hyper_main) by 24h volume", len(candidates))
	return candidates, nil
}

// ============================================================================
// External & Quant Data
// ============================================================================

// FetchMarketData fetches market data based on strategy configuration
func (e *StrategyEngine) FetchMarketData(symbol string) (*market.Data, error) {
	return market.Get(symbol)
}

// FetchExternalData fetches external data sources
func (e *StrategyEngine) FetchExternalData() (map[string]interface{}, error) {
	externalData := make(map[string]interface{})

	for _, source := range e.config.Indicators.ExternalDataSources {
		data, err := e.fetchSingleExternalSource(source)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch external data source [%s]: %v", source.Name, err)
			continue
		}
		externalData[source.Name] = data
	}

	return externalData, nil
}

func (e *StrategyEngine) fetchSingleExternalSource(source store.ExternalDataSource) (interface{}, error) {
	// SSRF Protection: Validate URL before making request
	if err := security.ValidateURL(source.URL); err != nil {
		return nil, fmt.Errorf("external source URL validation failed: %w", err)
	}

	timeout := time.Duration(source.RefreshSecs) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Use SSRF-safe HTTP client
	client := security.SafeHTTPClient(timeout)

	req, err := http.NewRequest(source.Method, source.URL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range source.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := security.ReadResponseBody(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if source.DataPath != "" {
		result = extractJSONPath(result, source.DataPath)
	}

	return result, nil
}

func extractJSONPath(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// FetchQuantData fetches the per-symbol quantitative block (price, price
// changes, OI level/delta) from Binance Futures directly. Fund flow has no
// Binance equivalent and is omitted; the data-page relay remains the source
// for the AI500 coin list and NetFlow ranking.
func (e *StrategyEngine) FetchQuantData(symbol string) (*QuantData, error) {
	if !e.config.Indicators.EnableQuantData {
		return nil, nil
	}
	return binanceQuantSnapshot(symbol)
}

// FetchQuantDataBatch batch fetches quantitative data
func (e *StrategyEngine) FetchQuantDataBatch(symbols []string) map[string]*QuantData {
	result := make(map[string]*QuantData)

	if !e.config.Indicators.EnableQuantData {
		return result
	}

	for _, symbol := range symbols {
		data, err := e.FetchQuantData(symbol)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch quantitative data for %s: %v", symbol, err)
			continue
		}
		if data != nil {
			result[symbol] = data
		}
	}

	return result
}

// FetchOIRankingData fetches market-wide OI ranking data from Binance
// openInterestHist over the top-volume universe.
func (e *StrategyEngine) FetchOIRankingData() *nofxos.OIRankingData {
	indicators := e.config.Indicators
	if !indicators.EnableOIRanking {
		return nil
	}

	duration := indicators.OIRankingDuration
	if duration == "" {
		duration = "1h"
	}

	limit := indicators.OIRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("📊 Fetching OI ranking data (duration: %s, limit: %d, source: binance)", duration, limit)

	data, err := binanceOIRanking(context.Background(), duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch OI ranking data: %v", err)
		return nil
	}
	return data
}

// FetchNetFlowRankingData fetches market-wide NetFlow ranking data
func (e *StrategyEngine) FetchNetFlowRankingData() *nofxos.NetFlowRankingData {
	indicators := e.config.Indicators
	if !indicators.EnableNetFlowRanking {
		return nil
	}

	duration := indicators.NetFlowRankingDuration
	if duration == "" {
		duration = "1h"
	}

	limit := indicators.NetFlowRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("💰 Fetching NetFlow ranking data (duration: %s, limit: %d)", duration, limit)

	// Vergex only (data page "Net Flow" card source). NofxOS public keys were
	// deprecated server-side, so there is no fallback.
	data, err := e.vergexClient.GetNetFlowRanking(duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch NetFlow ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ NetFlow ranking data ready: inst_in=%d, inst_out=%d, retail_in=%d, retail_out=%d",
		len(data.InstitutionFutureTop), len(data.InstitutionFutureLow),
		len(data.PersonalFutureTop), len(data.PersonalFutureLow))

	return data
}

// FetchPriceRankingData fetches market-wide price ranking data (gainers/losers)
// from Binance tickers and klines.
func (e *StrategyEngine) FetchPriceRankingData() *nofxos.PriceRankingData {
	indicators := e.config.Indicators
	if !indicators.EnablePriceRanking {
		return nil
	}

	durations := indicators.PriceRankingDuration
	if durations == "" {
		durations = "1h"
	}

	limit := indicators.PriceRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("📈 Fetching Price ranking data (durations: %s, limit: %d, source: binance)", durations, limit)

	data, err := binancePriceRanking(context.Background(), durations, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch Price ranking data: %v", err)
		return nil
	}
	return data
}

// ============================================================================
// Helper Functions
// ============================================================================

// detectLanguage detects language from text content
// Returns LangChinese if text contains Chinese characters, otherwise LangEnglish
func detectLanguage(text string) Language {
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return LangChinese
		}
	}
	return LangEnglish
}
