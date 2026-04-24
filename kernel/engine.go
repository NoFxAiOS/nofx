package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"nofx/provider/hyperliquid"
	"nofx/provider/nofxos"
	"nofx/security"
	"nofx/store"
	"strings"
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
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // Historical peak profit percentage
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // Position update timestamp (milliseconds)
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
	Sources []string `json:"sources"` // Sources: "ai500" and/or "oi_top"
	Score   float64  `json:"score,omitempty"` // Relevance score from screener
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
	TotalTrades    int     `json:"total_trades"`     // Total number of trades (closed)
	WinRate        float64 `json:"win_rate"`         // Win rate (%)
	ProfitFactor   float64 `json:"profit_factor"`    // Profit factor
	SharpeRatio    float64 `json:"sharpe_ratio"`     // Sharpe ratio
	TotalPnL       float64 `json:"total_pnl"`        // Total profit/loss
	AvgWin         float64 `json:"avg_win"`          // Average win
	AvgLoss        float64 `json:"avg_loss"`         // Average loss
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // Maximum drawdown (%)
}

// RecentOrder recently completed order (for AI input)
type RecentOrder struct {
	Symbol       string  `json:"symbol"`        // Trading pair
	Side         string  `json:"side"`          // long/short
	EntryPrice   float64 `json:"entry_price"`   // Entry price
	ExitPrice    float64 `json:"exit_price"`    // Exit price
	RealizedPnL  float64 `json:"realized_pnl"`  // Realized profit/loss
	PnLPct       float64 `json:"pnl_pct"`       // Profit/loss percentage
	EntryTime    string  `json:"entry_time"`    // Entry time
	ExitTime     string  `json:"exit_time"`     // Exit time
	HoldDuration string  `json:"hold_duration"` // Hold duration, e.g. "2h30m"
}

// Context trading context (complete information passed to AI)
type Context struct {
	CurrentTime        string                             `json:"current_time"`
	RuntimeMinutes     int                                `json:"runtime_minutes"`
	CallCount          int                                `json:"call_count"`
	Account            AccountInfo                        `json:"account"`
	Positions          []PositionInfo                     `json:"positions"`
	CandidateCoins     []CandidateCoin                    `json:"candidate_coins"`
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
	BTCETHLeverage     int                                `json:"-"`
	AltcoinLeverage    int                                `json:"-"`
	Timeframes         []string                           `json:"-"`
	Indicators         store.IndicatorConfig              `json:"-"` // Indicator config for prompt filtering
}

// Decision AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // Standard: "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	// Grid actions: "place_buy_limit", "place_sell_limit", "cancel_order", "cancel_all_orders", "pause_grid", "resume_grid", "adjust_grid"

	// Opening position parameters
	Leverage        int                         `json:"leverage,omitempty"`
	PositionSizeUSD float64                     `json:"position_size_usd,omitempty"`
	StopLoss        float64                     `json:"stop_loss,omitempty"`
	TakeProfit      float64                     `json:"take_profit,omitempty"`
	ProtectionPlan  *AIProtectionPlan           `json:"protection_plan,omitempty"`
	EntryProtection *AIEntryProtectionRationale `json:"entry_protection_rationale,omitempty"`

	// Grid trading parameters
	Price      float64 `json:"price,omitempty"`       // Limit order price (for grid)
	Quantity   float64 `json:"quantity,omitempty"`    // Order quantity (for grid)
	LevelIndex int     `json:"level_index,omitempty"` // Grid level index
	OrderID    string  `json:"order_id,omitempty"`    // Order ID (for cancel)

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`
}

type AIEntryProtectionRationale struct {
	TimeframeContext     AIEntryTimeframeContext     `json:"timeframe_context,omitempty"`
	KeyLevels            AIEntryKeyLevels            `json:"key_levels,omitempty"`
	StructuralKeyLevels  []AIStructuralKeyLevel      `json:"structural_key_levels,omitempty"`
	VolatilityAdjustment AIEntryVolatilityAdjustment `json:"volatility_adjustment,omitempty"`
	RiskReward           AIRiskRewardRationale       `json:"risk_reward,omitempty"`
	ExecutionConstraints AIEntryExecutionConstraints `json:"execution_constraints,omitempty"`
	DerivativesContext   AIEntryDerivativesContext   `json:"derivatives_context,omitempty"`
	Anchors              []AIEntryProtectionAnchor   `json:"anchors,omitempty"`
	AlignmentNotes       []string                    `json:"alignment_notes,omitempty"`
}

// AIStructuralKeyLevel represents a structural level that influenced protection placement
type AIStructuralKeyLevel struct {
	Price     float64 `json:"price"`
	Type      string  `json:"type"`      // "support" or "resistance"
	Timeframe string  `json:"timeframe"`
	Source    string  `json:"source"`    // "auto_detected", "fibonacci_0.618", etc.
	UsedFor   string  `json:"used_for"` // "tp1", "tp2", "stop_loss", "invalidation"
}

type AIEntryTimeframeContext struct {
	Primary string   `json:"primary,omitempty"`
	Lower   []string `json:"lower,omitempty"`
	Higher  []string `json:"higher,omitempty"`
}

type AIEntryKeyLevels struct {
	Support    []float64         `json:"support,omitempty"`
	Resistance []float64         `json:"resistance,omitempty"`
	SwingHighs []float64         `json:"swing_highs,omitempty"`
	SwingLows  []float64         `json:"swing_lows,omitempty"`
	Fibonacci  *AIEntryFibonacci `json:"fibonacci,omitempty"`
}

// UnmarshalJSON accepts common model aliases and normalizes them into the canonical key-level schema.
func (k *AIEntryKeyLevels) UnmarshalJSON(data []byte) error {
	type alias AIEntryKeyLevels
	var aux struct {
		alias
		SupportLevels    []float64       `json:"support_levels,omitempty"`
		ResistanceLevels []float64       `json:"resistance_levels,omitempty"`
		FibLevels        []float64       `json:"fib_levels,omitempty"`
		FibonacciLevels  []float64       `json:"fibonacci_levels,omitempty"`
		SwingHigh        float64         `json:"swing_high,omitempty"`
		SwingLow         float64         `json:"swing_low,omitempty"`
		Fibonacci        *AIEntryFibonacci `json:"fibonacci,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*k = AIEntryKeyLevels(aux.alias)
	if len(k.Support) == 0 && len(aux.SupportLevels) > 0 {
		k.Support = aux.SupportLevels
	}
	if len(k.Resistance) == 0 && len(aux.ResistanceLevels) > 0 {
		k.Resistance = aux.ResistanceLevels
	}
	if k.Fibonacci == nil && aux.Fibonacci != nil {
		k.Fibonacci = aux.Fibonacci
	}
	if k.Fibonacci == nil && (len(aux.FibLevels) > 0 || len(aux.FibonacciLevels) > 0 || aux.SwingHigh > 0 || aux.SwingLow > 0) {
		levels := aux.FibLevels
		if len(levels) == 0 {
			levels = aux.FibonacciLevels
		}
		k.Fibonacci = &AIEntryFibonacci{SwingHigh: aux.SwingHigh, SwingLow: aux.SwingLow, Levels: levels}
	}
	return nil
}

type AIEntryFibonacci struct {
	SwingHigh float64   `json:"swing_high,omitempty"`
	SwingLow  float64   `json:"swing_low,omitempty"`
	Levels    []float64 `json:"levels,omitempty"`
}

// UnmarshalJSON accepts fib-level aliases commonly emitted by models.
func (f *AIEntryFibonacci) UnmarshalJSON(data []byte) error {
	type alias AIEntryFibonacci
	var aux struct {
		alias
		FibLevels       []float64 `json:"fib_levels,omitempty"`
		FibonacciLevels []float64 `json:"fibonacci_levels,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*f = AIEntryFibonacci(aux.alias)
	if len(f.Levels) == 0 {
		if len(aux.FibLevels) > 0 {
			f.Levels = aux.FibLevels
		} else if len(aux.FibonacciLevels) > 0 {
			f.Levels = aux.FibonacciLevels
		}
	}
	return nil
}

type AIEntryVolatilityAdjustment struct {
	ATR14Pct     float64 `json:"atr14_pct,omitempty"`
	BollWidthPct float64 `json:"boll_width_pct,omitempty"`
	MarketRegime string  `json:"market_regime,omitempty"`
	WideningPct  float64 `json:"widening_pct,omitempty"`
}

func (v *AIEntryVolatilityAdjustment) UnmarshalJSON(data []byte) error {
	type alias AIEntryVolatilityAdjustment
	var aux struct {
		alias
		ATRPct        float64 `json:"atr_pct,omitempty"`
		ATR14         float64 `json:"atr14,omitempty"`
		BollingerWidth float64 `json:"bollinger_width_pct,omitempty"`
		Regime        string  `json:"regime,omitempty"`
		BufferPct     float64 `json:"buffer_pct,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*v = AIEntryVolatilityAdjustment(aux.alias)
	if v.ATR14Pct <= 0 {
		if aux.ATRPct > 0 { v.ATR14Pct = aux.ATRPct } else if aux.ATR14 > 0 { v.ATR14Pct = aux.ATR14 }
	}
	if v.BollWidthPct <= 0 && aux.BollingerWidth > 0 { v.BollWidthPct = aux.BollingerWidth }
	if v.MarketRegime == "" && aux.Regime != "" { v.MarketRegime = aux.Regime }
	if v.WideningPct <= 0 && aux.BufferPct > 0 { v.WideningPct = aux.BufferPct }
	return nil
}

type AIRiskRewardRationale struct {
	Entry            float64 `json:"entry,omitempty"`
	Invalidation     float64 `json:"invalidation,omitempty"`
	FirstTarget      float64 `json:"first_target,omitempty"`
	GrossEstimatedRR float64 `json:"gross_estimated_rr,omitempty"`
	NetEstimatedRR   float64 `json:"net_estimated_rr,omitempty"`
	MinRequiredRR    float64 `json:"min_required_rr,omitempty"`
	Passed           bool    `json:"passed,omitempty"`
}

// UnmarshalJSON accepts common risk-reward aliases.
func (r *AIRiskRewardRationale) UnmarshalJSON(data []byte) error {
	type alias AIRiskRewardRationale
	var aux struct {
		alias
		EntryPrice        float64 `json:"entry_price,omitempty"`
		InvalidationPrice float64 `json:"invalidation_price,omitempty"`
		FirstTargetPrice  float64 `json:"first_target_price,omitempty"`
		GrossRR           float64 `json:"gross_rr,omitempty"`
		NetRR             float64 `json:"net_rr,omitempty"`
		MinRR             float64 `json:"min_rr,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = AIRiskRewardRationale(aux.alias)
	if r.Entry <= 0 && aux.EntryPrice > 0 { r.Entry = aux.EntryPrice }
	if r.Invalidation <= 0 && aux.InvalidationPrice > 0 { r.Invalidation = aux.InvalidationPrice }
	if r.FirstTarget <= 0 && aux.FirstTargetPrice > 0 { r.FirstTarget = aux.FirstTargetPrice }
	if r.GrossEstimatedRR <= 0 && aux.GrossRR > 0 { r.GrossEstimatedRR = aux.GrossRR }
	if r.NetEstimatedRR <= 0 && aux.NetRR > 0 { r.NetEstimatedRR = aux.NetRR }
	if r.MinRequiredRR <= 0 && aux.MinRR > 0 { r.MinRequiredRR = aux.MinRR }
	return nil
}

type AIEntryExecutionConstraints struct {
	TickSize             float64 `json:"tick_size,omitempty"`
	PricePrecision       int     `json:"price_precision,omitempty"`
	QtyStepSize          float64 `json:"qty_step_size,omitempty"`
	QtyPrecision         int     `json:"qty_precision,omitempty"`
	MinQty               float64 `json:"min_qty,omitempty"`
	MinNotional          float64 `json:"min_notional,omitempty"`
	MarkPrice            float64 `json:"mark_price,omitempty"`
	LastPrice            float64 `json:"last_price,omitempty"`
	IndexPrice           float64 `json:"index_price,omitempty"`
	BestBid              float64 `json:"best_bid,omitempty"`
	BestAsk              float64 `json:"best_ask,omitempty"`
	SpreadBps            float64 `json:"spread_bps,omitempty"`
	TakerFeeRate         float64 `json:"taker_fee_rate,omitempty"`
	MakerFeeRate         float64 `json:"maker_fee_rate,omitempty"`
	EstimatedSlippageBps float64 `json:"estimated_slippage_bps,omitempty"`
}

func (e *AIEntryExecutionConstraints) UnmarshalJSON(data []byte) error {
	type alias AIEntryExecutionConstraints
	var aux struct {
		alias
		Bid              float64 `json:"bid,omitempty"`
		Ask              float64 `json:"ask,omitempty"`
		SlippageBps      float64 `json:"slippage_bps,omitempty"`
		PriceStep        float64 `json:"price_step,omitempty"`
		QuantityStepSize float64 `json:"quantity_step_size,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*e = AIEntryExecutionConstraints(aux.alias)
	if e.BestBid <= 0 && aux.Bid > 0 { e.BestBid = aux.Bid }
	if e.BestAsk <= 0 && aux.Ask > 0 { e.BestAsk = aux.Ask }
	if e.EstimatedSlippageBps <= 0 && aux.SlippageBps > 0 { e.EstimatedSlippageBps = aux.SlippageBps }
	if e.TickSize <= 0 && aux.PriceStep > 0 { e.TickSize = aux.PriceStep }
	if e.QtyStepSize <= 0 && aux.QuantityStepSize > 0 { e.QtyStepSize = aux.QuantityStepSize }
	return nil
}

type AIEntryDerivativesContext struct {
	OICurrent          float64 `json:"oi_current,omitempty"`
	OIDelta5mPct       float64 `json:"oi_delta_5m_pct,omitempty"`
	OIDelta15mPct      float64 `json:"oi_delta_15m_pct,omitempty"`
	OIDelta1hPct       float64 `json:"oi_delta_1h_pct,omitempty"`
	FundingRateCurrent float64 `json:"funding_rate_current,omitempty"`
	FundingRateAvg8h   float64 `json:"funding_rate_avg_8h,omitempty"`
	MarkIndexBasisBps  float64 `json:"mark_index_basis_bps,omitempty"`
	PremiumIndex       float64 `json:"premium_index,omitempty"`
	OrderbookImbalance float64 `json:"orderbook_imbalance,omitempty"`
	Top5BidNotional    float64 `json:"top5_bid_notional,omitempty"`
	Top5AskNotional    float64 `json:"top5_ask_notional,omitempty"`
}

func (d *AIEntryDerivativesContext) UnmarshalJSON(data []byte) error {
	type alias AIEntryDerivativesContext
	var aux struct {
		alias
		OpenInterest      float64 `json:"open_interest,omitempty"`
		FundingRate       float64 `json:"funding_rate,omitempty"`
		BasisBps          float64 `json:"basis_bps,omitempty"`
		DepthImbalance    float64 `json:"depth_imbalance,omitempty"`
		BidNotionalTop5   float64 `json:"bid_notional_top5,omitempty"`
		AskNotionalTop5   float64 `json:"ask_notional_top5,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*d = AIEntryDerivativesContext(aux.alias)
	if d.OICurrent <= 0 && aux.OpenInterest > 0 { d.OICurrent = aux.OpenInterest }
	if d.FundingRateCurrent == 0 && aux.FundingRate != 0 { d.FundingRateCurrent = aux.FundingRate }
	if d.MarkIndexBasisBps == 0 && aux.BasisBps != 0 { d.MarkIndexBasisBps = aux.BasisBps }
	if d.OrderbookImbalance == 0 && aux.DepthImbalance != 0 { d.OrderbookImbalance = aux.DepthImbalance }
	if d.Top5BidNotional <= 0 && aux.BidNotionalTop5 > 0 { d.Top5BidNotional = aux.BidNotionalTop5 }
	if d.Top5AskNotional <= 0 && aux.AskNotionalTop5 > 0 { d.Top5AskNotional = aux.AskNotionalTop5 }
	return nil
}

type AIEntryProtectionAnchor struct {
	Type      string  `json:"type,omitempty"`
	Timeframe string  `json:"timeframe,omitempty"`
	Price     float64 `json:"price,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}

type AIProtectionPlan struct {
	Mode             string                     `json:"mode,omitempty"`
	TakeProfitPct    float64                    `json:"take_profit_pct,omitempty"`
	StopLossPct      float64                    `json:"stop_loss_pct,omitempty"`
	LadderRules      []AIProtectionLadderRule   `json:"ladder_rules,omitempty"`
	DrawdownRules    []AIProtectionDrawdownRule `json:"drawdown_rules,omitempty"`
	BreakEvenTrigger string                     `json:"break_even_trigger_mode,omitempty"`
	BreakEvenValue   float64                    `json:"break_even_trigger_value,omitempty"`
	BreakEvenOffset  float64                    `json:"break_even_offset_pct,omitempty"`
	BreakEvenAnchor  string                     `json:"break_even_reason_anchor,omitempty"`
}

// UnmarshalJSON accepts common break-even aliases emitted by models.
func (p *AIProtectionPlan) UnmarshalJSON(data []byte) error {
	type alias AIProtectionPlan
	var aux struct {
		alias
		BreakevenTrigger string  `json:"breakeven_trigger,omitempty"`
		BreakEvenValue   float64 `json:"break_even_value,omitempty"`
		BreakevenValue   float64 `json:"breakeven_value,omitempty"`
		BreakEvenOffset  float64 `json:"break_even_offset,omitempty"`
		BreakevenOffset  float64 `json:"breakeven_offset_pct,omitempty"`
		BreakEvenReason  string  `json:"break_even_reason,omitempty"`
		BreakevenReason  string  `json:"breakeven_reason_anchor,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*p = AIProtectionPlan(aux.alias)
	if p.BreakEvenTrigger == "" && aux.BreakevenTrigger != "" {
		p.BreakEvenTrigger = aux.BreakevenTrigger
	}
	if p.BreakEvenValue <= 0 {
		if aux.BreakEvenValue > 0 {
			p.BreakEvenValue = aux.BreakEvenValue
		} else if aux.BreakevenValue > 0 {
			p.BreakEvenValue = aux.BreakevenValue
		}
	}
	if p.BreakEvenOffset <= 0 {
		if aux.BreakEvenOffset > 0 {
			p.BreakEvenOffset = aux.BreakEvenOffset
		} else if aux.BreakevenOffset > 0 {
			p.BreakEvenOffset = aux.BreakevenOffset
		}
	}
	if p.BreakEvenAnchor == "" {
		if aux.BreakEvenReason != "" {
			p.BreakEvenAnchor = aux.BreakEvenReason
		} else if aux.BreakevenReason != "" {
			p.BreakEvenAnchor = aux.BreakevenReason
		}
	}
	return nil
}

type AIProtectionDrawdownRule struct {
	Timeframe           string  `json:"timeframe,omitempty"`
	MinProfitPct        float64 `json:"min_profit_pct,omitempty"`
	MaxDrawdownPct      float64 `json:"max_drawdown_pct,omitempty"`
	CloseRatioPct       float64 `json:"close_ratio_pct,omitempty"`
	PollIntervalSeconds int     `json:"poll_interval_seconds,omitempty"`
	ReasonAnchor        string  `json:"reason_anchor,omitempty"`
	StageName           string  `json:"stage_name,omitempty"`
	RunnerKeepPct       float64 `json:"runner_keep_pct,omitempty"`
	RunnerStopMode      string  `json:"runner_stop_mode,omitempty"`
	RunnerStopSource    string  `json:"runner_stop_source,omitempty"`
	RunnerTargetMode    string  `json:"runner_target_mode,omitempty"`
	RunnerTargetSource  string  `json:"runner_target_source,omitempty"`
}

// UnmarshalJSON accepts both close_ratio_pct (canonical) and close_ratio (legacy/model alias)
func (r *AIProtectionDrawdownRule) UnmarshalJSON(data []byte) error {
	type alias AIProtectionDrawdownRule
	var aux struct {
		alias
		CloseRatio float64 `json:"close_ratio,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = AIProtectionDrawdownRule(aux.alias)
	if r.CloseRatioPct <= 0 && aux.CloseRatio > 0 {
		r.CloseRatioPct = aux.CloseRatio
	}
	return nil
}

type AIProtectionLadderRule struct {
	TakeProfitPct           float64 `json:"take_profit_pct,omitempty"`
	TakeProfitCloseRatioPct float64 `json:"take_profit_close_ratio_pct,omitempty"`
	StopLossPct             float64 `json:"stop_loss_pct,omitempty"`
	StopLossCloseRatioPct   float64 `json:"stop_loss_close_ratio_pct,omitempty"`
	StructuralAnchor        string  `json:"structural_anchor,omitempty"`
}

// UnmarshalJSON accepts common ladder-rule aliases such as tp/sl abbreviations.
func (r *AIProtectionLadderRule) UnmarshalJSON(data []byte) error {
	type alias AIProtectionLadderRule
	var aux struct {
		alias
		TPPct        float64 `json:"tp_pct,omitempty"`
		SLPct        float64 `json:"sl_pct,omitempty"`
		TPCloseRatio float64 `json:"tp_close_ratio_pct,omitempty"`
		SLCloseRatio float64 `json:"sl_close_ratio_pct,omitempty"`
		TPLevel      float64 `json:"tp_level,omitempty"`
		SLLevel      float64 `json:"sl_level,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = AIProtectionLadderRule(aux.alias)
	if r.TakeProfitPct <= 0 {
		if aux.TPPct > 0 { r.TakeProfitPct = aux.TPPct } else if aux.TPLevel > 0 { r.TakeProfitPct = aux.TPLevel }
	}
	if r.StopLossPct <= 0 {
		if aux.SLPct > 0 { r.StopLossPct = aux.SLPct } else if aux.SLLevel > 0 { r.StopLossPct = aux.SLLevel }
	}
	if r.TakeProfitCloseRatioPct <= 0 && aux.TPCloseRatio > 0 { r.TakeProfitCloseRatioPct = aux.TPCloseRatio }
	if r.StopLossCloseRatioPct <= 0 && aux.SLCloseRatio > 0 { r.StopLossCloseRatioPct = aux.SLCloseRatio }
	return nil
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
	ParseFallback       bool       `json:"parse_fallback,omitempty"`
	ParseFallbackReason string     `json:"parse_fallback_reason,omitempty"`
}

// QuantData quantitative data structure (fund flow, position changes, price changes)
type QuantData struct {
	Symbol      string             `json:"symbol"`
	Price       float64            `json:"price"`
	Netflow     *NetflowData       `json:"netflow,omitempty"`
	OI          map[string]*OIData `json:"oi,omitempty"`
	PriceChange map[string]float64 `json:"price_change,omitempty"`
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

// StrategyEngine 是策略配置在运行态的核心执行入口。
// 它把 strategy config、NofxOS 数据源、语言选择、候选币筛选、上下文构建等能力收敛到一起，
// 相当于连接“配置层”和“AI 决策层”的桥梁。
type StrategyEngine struct {
	config       *store.StrategyConfig
	nofxosClient *nofxos.Client
}

// NewStrategyEngine creates strategy execution engine
func NewStrategyEngine(config *store.StrategyConfig) *StrategyEngine {
	// Create NofxOS client with API key from config
	apiKey := config.Indicators.NofxOSAPIKey
	if apiKey == "" {
		apiKey = nofxos.DefaultAuthKey
	}
	client := nofxos.NewClient(nofxos.DefaultBaseURL, apiKey)

	return &StrategyEngine{
		config:       config,
		nofxosClient: client,
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
func (e *StrategyEngine) GetCandidateCoins() ([]CandidateCoin, error) {
	var candidates []CandidateCoin
	symbolSources := make(map[string][]string)

	coinSource := e.config.CoinSource

	switch coinSource.SourceType {
	case "static":
		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"static"},
			})
		}

		return e.filterExcludedCoins(candidates), nil

	case "ai500":
		// Check use_ai500 flag; if false, fall back to static coins
		if !coinSource.UseAI500 {
			logger.Infof("⚠️  source_type is 'ai500' but use_ai500 is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
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
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
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
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getOILowCoins(coinSource.OILowLimit)
		if err != nil {
			return nil, err
		}
		// Empty list is a normal condition, return directly
		return e.filterExcludedCoins(coins), nil

	case "hyper_all":
		// All Hyperliquid perp coins
		if !coinSource.UseHyperAll {
			logger.Infof("⚠️  source_type is 'hyper_all' but use_hyper_all is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
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
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
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
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "ai500")
				}
			}
		}

		if coinSource.UseOITop {
			oiCoins, err := e.getOITopCoins(coinSource.OITopLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Top: %v", err)
			} else {
				for _, coin := range oiCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "oi_top")
				}
			}
		}

		if coinSource.UseOILow {
			oiLowCoins, err := e.getOILowCoins(coinSource.OILowLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Low: %v", err)
			} else {
				for _, coin := range oiLowCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "oi_low")
				}
			}
		}

		if coinSource.UseHyperAll {
			hyperCoins, err := e.getHyperAllCoins()
			if err != nil {
				logger.Infof("⚠️  Failed to get Hyperliquid All coins: %v", err)
			} else {
				for _, coin := range hyperCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "hyper_all")
				}
			}
		}

		if coinSource.UseHyperMain {
			hyperMainCoins, err := e.getHyperMainCoins(coinSource.HyperMainLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get Hyperliquid Main coins: %v", err)
			} else {
				for _, coin := range hyperMainCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "hyper_main")
				}
			}
		}

		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			if _, exists := symbolSources[symbol]; !exists {
				symbolSources[symbol] = []string{"static"}
			} else {
				symbolSources[symbol] = append(symbolSources[symbol], "static")
			}
		}

		for symbol, sources := range symbolSources {
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: sources,
			})
		}
		return e.filterExcludedCoins(candidates), nil

	case "market":
		// Consume market layer output: hot coins, OI top, OI low (multi-select)
		marketLimit := coinSource.MarketLimit
		if marketLimit <= 0 {
			marketLimit = 20
		}
		exchangeSrc := coinSource.ExchangeSource
		if exchangeSrc == "" {
			exchangeSrc = "okx"
		}

		// Resolve which lists to fetch: prefer MarketLists (multi), fallback to MarketList (single)
		lists := coinSource.MarketLists
		if len(lists) == 0 {
			if coinSource.MarketList != "" {
				lists = []string{coinSource.MarketList}
			} else {
				lists = []string{"hot"}
			}
		}

		seen := make(map[string]bool)
		var allCoins []market.HotCoin
		for _, list := range lists {
			var coins []market.HotCoin
			var err error
			switch list {
			case "oi_top":
				coins, err = market.GetOITopCoinsWithExchange(marketLimit, coinSource.ExcludedCoins, exchangeSrc)
			case "oi_low":
				coins, err = market.GetOILowCoinsWithExchange(marketLimit, coinSource.ExcludedCoins, exchangeSrc)
			default: // "hot"
				coins, err = market.GetHotCoinsWithExchange(marketLimit, coinSource.ExcludedCoins, exchangeSrc)
			}
			if err != nil {
				logger.Infof("⚠️  Market list %s failed: %v", list, err)
				continue
			}
			for _, coin := range coins {
				if !seen[coin.Symbol] {
					seen[coin.Symbol] = true
					allCoins = append(allCoins, coin)
				}
			}
			logger.Infof("📊 Market list %s (%s): %d coins", list, exchangeSrc, len(coins))
		}

		if len(allCoins) == 0 {
			logger.Infof("⚠️  All market lists failed, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{Symbol: symbol, Sources: []string{"static"}})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		for _, coin := range allCoins {
			candidates = append(candidates, CandidateCoin{
				Symbol:  coin.Symbol,
				Sources: []string{"market"},
				Score:   coin.HotScore,
			})
		}
		logger.Infof("✅ Market source (%v): %d unique candidates", lists, len(candidates))
		return e.filterExcludedCoins(candidates), nil

	default:
		return nil, fmt.Errorf("unknown coin source type: %s", coinSource.SourceType)
	}
}

// filterExcludedCoins removes excluded coins from the candidates list
func (e *StrategyEngine) filterExcludedCoins(candidates []CandidateCoin) []CandidateCoin {
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

func (e *StrategyEngine) getAI500Coins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 30
	}

	symbols, err := e.nofxosClient.GetTopRatedCoins(limit)
	if err != nil {
		return nil, err
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

func (e *StrategyEngine) getOITopCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	positions, err := e.nofxosClient.GetOITopPositions()
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for i, pos := range positions {
		if i >= limit {
			break
		}
		symbol := market.Normalize(pos.Symbol)
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_top"},
		})
	}
	return candidates, nil
}

func (e *StrategyEngine) getOILowCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	positions, err := e.nofxosClient.GetOILowPositions()
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for i, pos := range positions {
		if i >= limit {
			break
		}
		symbol := market.Normalize(pos.Symbol)
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
	exchangeSrc := e.config.CoinSource.ExchangeSource
	if exchangeSrc == "" {
		exchangeSrc = "binance" // backward compat: existing behavior defaults to binance
	}
	return market.GetWithExchange(symbol, exchangeSrc)
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

	body, err := io.ReadAll(resp.Body)
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

// FetchQuantData fetches quantitative data for a single coin
func (e *StrategyEngine) FetchQuantData(symbol string) (*QuantData, error) {
	if !e.config.Indicators.EnableQuantData {
		return nil, nil
	}

	// Use nofxos client with unified API key
	include := "oi,price"
	if e.config.Indicators.EnableQuantNetflow {
		include = "netflow,oi,price"
	}

	nofxosData, err := e.nofxosClient.GetCoinData(symbol, include)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quant data: %w", err)
	}

	if nofxosData == nil {
		return nil, nil
	}

	// Convert nofxos.QuantData to kernel.QuantData
	quantData := &QuantData{
		Symbol:      nofxosData.Symbol,
		Price:       nofxosData.Price,
		PriceChange: nofxosData.PriceChange,
	}

	// Convert OI data
	if nofxosData.OI != nil {
		quantData.OI = make(map[string]*OIData)
		for exchange, oiData := range nofxosData.OI {
			if oiData != nil {
				kData := &OIData{
					CurrentOI: oiData.CurrentOI,
				}
				if oiData.Delta != nil {
					kData.Delta = make(map[string]*OIDeltaData)
					for dur, delta := range oiData.Delta {
						if delta != nil {
							kData.Delta[dur] = &OIDeltaData{
								OIDelta:        delta.OIDelta,
								OIDeltaValue:   delta.OIDeltaValue,
								OIDeltaPercent: delta.OIDeltaPercent,
							}
						}
					}
				}
				quantData.OI[exchange] = kData
			}
		}
	}

	// Convert Netflow data
	if nofxosData.Netflow != nil {
		quantData.Netflow = &NetflowData{}
		if nofxosData.Netflow.Institution != nil {
			quantData.Netflow.Institution = &FlowTypeData{
				Future: nofxosData.Netflow.Institution.Future,
				Spot:   nofxosData.Netflow.Institution.Spot,
			}
		}
		if nofxosData.Netflow.Personal != nil {
			quantData.Netflow.Personal = &FlowTypeData{
				Future: nofxosData.Netflow.Personal.Future,
				Spot:   nofxosData.Netflow.Personal.Spot,
			}
		}
	}

	return quantData, nil
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

// FetchOIRankingData fetches market-wide OI ranking data
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

	logger.Infof("📊 Fetching OI ranking data (duration: %s, limit: %d)", duration, limit)

	data, err := e.nofxosClient.GetOIRanking(duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch OI ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ OI ranking data ready: %d top, %d low positions",
		len(data.TopPositions), len(data.LowPositions))

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

	data, err := e.nofxosClient.GetNetFlowRanking(duration, limit)
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

	logger.Infof("📈 Fetching Price ranking data (durations: %s, limit: %d)", durations, limit)

	data, err := e.nofxosClient.GetPriceRanking(durations, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch Price ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ Price ranking data ready for %d durations", len(data.Durations))

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
