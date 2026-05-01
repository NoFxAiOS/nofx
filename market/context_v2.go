package market

import "time"

// DerivativesContext summarizes free derivatives/crowding signals for a symbol.
// It is intentionally compact so it can be stored in decision review context and
// safely passed to AI without dumping raw noisy provider payloads.
type DerivativesContext struct {
	OpenInterest        float64   `json:"oi,omitempty"`
	OIChange15mPct      float64   `json:"oi_change_15m_pct,omitempty"`
	OIChange1hPct       float64   `json:"oi_change_1h_pct,omitempty"`
	FundingRate         float64   `json:"funding_rate,omitempty"`
	FundingBias         string    `json:"funding_bias,omitempty"`
	MarkIndexPremiumPct float64   `json:"mark_index_premium_pct,omitempty"`
	VolumeZScore        float64   `json:"volume_zscore,omitempty"`
	SqueezeRisk         string    `json:"squeeze_risk,omitempty"`
	DataQuality         string    `json:"data_quality,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}

// MarketContextV2 is the record-only normalized market context that will later
// feed regime/setup gates. Phase 1 stores or tests this structure without using
// it to change live order behavior.
type MarketContextV2 struct {
	Symbol       string                `json:"symbol"`
	Timeframes   []string              `json:"timeframes,omitempty"`
	PrimaryTF    string                `json:"primary_timeframe,omitempty"`
	TriggerTF    string                `json:"trigger_timeframe,omitempty"`
	BiasTF       string                `json:"bias_timeframe,omitempty"`
	MacroTFs     []string              `json:"macro_timeframes,omitempty"`
	DataQuality  string                `json:"data_quality,omitempty"`
	MissingTFs   []string              `json:"missing_timeframes,omitempty"`
	RegimeRules  *RegimeEntryGuidance  `json:"regime_entry_guidance,omitempty"`
	Derivatives  *DerivativesContext   `json:"derivatives,omitempty"`
	Structure    *MarketStructureBrief `json:"structure,omitempty"`
	Quant        *QuantContext         `json:"quant,omitempty"`
	ExchangeFlow *ExchangeFlowContext  `json:"exchange_flow,omitempty"`
}

type RegimeEntryGuidance struct {
	Regime             string   `json:"regime,omitempty"`
	AllowedSetups      []string `json:"allowed_setups,omitempty"`
	StructureMode      string   `json:"structure_mode,omitempty"`
	FibonacciMode      string   `json:"fibonacci_mode,omitempty"`
	RequiredAnchors    []string `json:"required_anchors,omitempty"`
	ProtectionGuidance string   `json:"protection_guidance,omitempty"`
	Notes              []string `json:"notes,omitempty"`
}

// MarketStructureBrief is a compact structural summary suitable for review context.
type MarketStructureBrief struct {
	Supports       []float64 `json:"supports,omitempty"`
	Resistances    []float64 `json:"resistances,omitempty"`
	FibLevels      []float64 `json:"fib_levels,omitempty"`
	RangePosition  string    `json:"range_position,omitempty"`
	NearestSupport float64   `json:"nearest_support,omitempty"`
	NearestResist  float64   `json:"nearest_resistance,omitempty"`
}

// BuildMarketContextV2 creates a compact context from already-fetched market data.
// It performs no network calls and has no trading side effects.
func BuildMarketContextV2(symbol string, data *Data, expectedTFs []string, primaryTF string) *MarketContextV2 {
	ctx := &MarketContextV2{
		Symbol:      symbol,
		Timeframes:  append([]string(nil), expectedTFs...),
		PrimaryTF:   primaryTF,
		TriggerTF:   pickKnownTimeframe(expectedTFs, "3m"),
		BiasTF:      pickKnownTimeframe(expectedTFs, "1h"),
		MacroTFs:    pickKnownTimeframes(expectedTFs, []string{"4h", "1d"}),
		DataQuality: "missing",
	}
	if data == nil {
		ctx.MissingTFs = append([]string(nil), expectedTFs...)
		return ctx
	}

	ctx.Derivatives = BuildDerivativesContext(data)
	ctx.Structure = BuildMarketStructureBrief(data)
	ctx.Quant = data.QuantContext
	ctx.ExchangeFlow = BuildExchangeFlowContext(data)
	ctx.RegimeRules = BuildRegimeEntryGuidance(data, ctx.Structure, ctx.Derivatives, ctx.Quant, ctx.ExchangeFlow)

	missing := make([]string, 0)
	for _, tf := range expectedTFs {
		series := data.TimeframeData[tf]
		if series == nil || len(series.Klines) == 0 {
			missing = append(missing, tf)
		}
	}
	ctx.MissingTFs = missing
	switch {
	case len(expectedTFs) == 0:
		ctx.DataQuality = "partial"
	case len(missing) == 0:
		ctx.DataQuality = "ok"
	case len(missing) < len(expectedTFs):
		ctx.DataQuality = "partial"
	default:
		ctx.DataQuality = "missing"
	}
	return ctx
}

func BuildDerivativesContext(data *Data) *DerivativesContext {
	if data == nil {
		return &DerivativesContext{DataQuality: "missing"}
	}
	ctx := &DerivativesContext{DataQuality: "partial", UpdatedAt: time.Now().UTC()}
	if data.OpenInterest != nil {
		ctx.OpenInterest = data.OpenInterest.Latest
		if data.OpenInterest.Average > 0 {
			ctx.OIChange1hPct = ((data.OpenInterest.Latest - data.OpenInterest.Average) / data.OpenInterest.Average) * 100
		}
	}
	ctx.FundingRate = data.FundingRate
	ctx.FundingBias = classifyFundingBias(data.FundingRate)
	ctx.VolumeZScore = estimateVolumeZScore(data)
	ctx.SqueezeRisk = classifySqueezeRisk(ctx.OIChange1hPct, data.PriceChange1h, ctx.FundingRate)
	if data.OpenInterest != nil || data.FundingRate != 0 || ctx.VolumeZScore != 0 {
		ctx.DataQuality = "ok"
	}
	return ctx
}

func BuildMarketStructureBrief(data *Data) *MarketStructureBrief {
	if data == nil {
		return nil
	}
	brief := &MarketStructureBrief{}
	for _, level := range data.StructuralLevels {
		switch level.Type {
		case "support":
			if len(brief.Supports) < 3 {
				brief.Supports = append(brief.Supports, level.Price)
			}
		case "resistance":
			if len(brief.Resistances) < 3 {
				brief.Resistances = append(brief.Resistances, level.Price)
			}
		}
	}
	if data.FibonacciLevels != nil {
		brief.FibLevels = compactFibLevels(data.FibonacciLevels)
	}
	brief.NearestSupport = nearestBelow(data.CurrentPrice, brief.Supports)
	brief.NearestResist = nearestAbove(data.CurrentPrice, brief.Resistances)
	brief.RangePosition = classifyRangePosition(data.CurrentPrice, brief.NearestSupport, brief.NearestResist)
	return brief
}

func pickKnownTimeframe(tfs []string, target string) string {
	for _, tf := range tfs {
		if tf == target {
			return tf
		}
	}
	return ""
}

func pickKnownTimeframes(tfs []string, targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if tf := pickKnownTimeframe(tfs, target); tf != "" {
			out = append(out, tf)
		}
	}
	return out
}

func classifyFundingBias(rate float64) string {
	switch {
	case rate >= 0.0005:
		return "long_crowded"
	case rate <= -0.0005:
		return "short_crowded"
	case rate == 0:
		return "unknown"
	default:
		return "neutral"
	}
}

func classifySqueezeRisk(oiChangePct, priceChangePct, fundingRate float64) string {
	absOI := oiChangePct
	if absOI < 0 {
		absOI = -absOI
	}
	absPrice := priceChangePct
	if absPrice < 0 {
		absPrice = -absPrice
	}
	if absOI >= 10 || (absOI >= 5 && absPrice >= 2) || fundingRate >= 0.001 || fundingRate <= -0.001 {
		return "high"
	}
	if absOI >= 3 || absPrice >= 1 {
		return "medium"
	}
	if absOI == 0 && absPrice == 0 && fundingRate == 0 {
		return "unknown"
	}
	return "low"
}

func estimateVolumeZScore(data *Data) float64 {
	if data == nil || data.LongerTermContext == nil || data.LongerTermContext.AverageVolume <= 0 {
		return 0
	}
	return (data.LongerTermContext.CurrentVolume - data.LongerTermContext.AverageVolume) / data.LongerTermContext.AverageVolume
}

func compactFibLevels(levels *FibonacciLevels) []float64 {
	if levels == nil || len(levels.Levels) == 0 {
		return nil
	}
	out := make([]float64, 0, 5)
	for _, key := range []string{"0.236", "0.382", "0.5", "0.618", "0.786"} {
		if v := levels.Levels[key]; v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func nearestBelow(price float64, levels []float64) float64 {
	best := 0.0
	for _, level := range levels {
		if level <= 0 || level > price {
			continue
		}
		if best == 0 || price-level < price-best {
			best = level
		}
	}
	return best
}

func nearestAbove(price float64, levels []float64) float64 {
	best := 0.0
	for _, level := range levels {
		if level <= price {
			continue
		}
		if best == 0 || level-price < best-price {
			best = level
		}
	}
	return best
}

func classifyRangePosition(price, support, resistance float64) string {
	if price <= 0 || support <= 0 || resistance <= 0 || resistance <= support {
		return "unknown"
	}
	pos := (price - support) / (resistance - support)
	switch {
	case pos <= 0.3:
		return "near_support"
	case pos >= 0.7:
		return "near_resistance"
	default:
		return "middle"
	}
}

func BuildRegimeEntryGuidance(data *Data, structure *MarketStructureBrief, derivatives *DerivativesContext, quant *QuantContext, exchangeFlow ...*ExchangeFlowContext) *RegimeEntryGuidance {
	var ex *ExchangeFlowContext
	if len(exchangeFlow) > 0 {
		ex = exchangeFlow[0]
	}
	regime := inferExecutionRegime(data, structure, derivatives, quant, ex)
	g := &RegimeEntryGuidance{Regime: regime}
	switch regime {
	case "trend_up":
		g.AllowedSetups = []string{"trend_pullback", "breakout_retest"}
		g.StructureMode = "support_as_invalidation_resistance_or_fib_as_target"
		g.FibonacciMode = "prefer_retracement_confluence"
		g.RequiredAnchors = []string{"primary_support", "higher_tf_trend_or_target"}
		g.ProtectionGuidance = "allow_runner; drawdown tolerance should be wider than normal retests"
	case "trend_down":
		g.AllowedSetups = []string{"trend_pullback", "breakout_retest"}
		g.StructureMode = "resistance_as_invalidation_support_or_fib_as_target"
		g.FibonacciMode = "prefer_retracement_confluence"
		g.RequiredAnchors = []string{"primary_resistance", "higher_tf_trend_or_target"}
		g.ProtectionGuidance = "allow_runner; drawdown tolerance should be wider than normal retests"
	case "range_edge":
		g.AllowedSetups = []string{"range_edge"}
		g.StructureMode = "trade_only_near_support_or_resistance_edge"
		g.FibonacciMode = "audit_only"
		g.RequiredAnchors = []string{"range_edge", "opposite_range_target"}
		g.ProtectionGuidance = "take profit sooner; avoid wide runner assumptions"
	case "squeeze_risk":
		g.AllowedSetups = []string{"breakout_retest"}
		g.StructureMode = "wait_for_retest_or_sweep_confirmation"
		g.FibonacciMode = "audit_only"
		g.RequiredAnchors = []string{"breakout_level", "failed_sweep_or_retest"}
		g.ProtectionGuidance = "reduce size; require wider wick buffer; avoid chasing first breakout candle"
	case "crowded":
		g.AllowedSetups = []string{"post_sweep_reversal", "range_edge"}
		g.StructureMode = "require_liquidity_sweep_or_clear_invalidation"
		g.FibonacciMode = "audit_only"
		g.RequiredAnchors = []string{"sweep_level", "invalidation_after_sweep"}
		g.ProtectionGuidance = "smaller size; higher net RR; no momentum chasing"
	default:
		g.AllowedSetups = []string{"range_edge", "trend_pullback"}
		g.StructureMode = "standard_support_resistance_anchor_linkage"
		g.FibonacciMode = "optional_confluence"
		g.RequiredAnchors = []string{"invalidation", "first_target"}
		g.ProtectionGuidance = "balanced ladder/drawdown; avoid arbitrary round percentages"
	}
	g.Notes = regimeNotes(derivatives, quant)
	return g
}

func inferExecutionRegime(data *Data, structure *MarketStructureBrief, derivatives *DerivativesContext, quant *QuantContext, exchangeFlow *ExchangeFlowContext) string {
	if derivatives != nil && derivatives.SqueezeRisk == "high" {
		return "squeeze_risk"
	}
	if quant != nil && quant.CrowdingRisk == "high" {
		return "crowded"
	}
	if exchangeFlow != nil && exchangeFlow.CrowdingRisk == "high" {
		return "crowded"
	}
	if structure != nil && (structure.RangePosition == "near_support" || structure.RangePosition == "near_resistance") && absMarketChange(data) < 1.5 {
		return "range_edge"
	}
	if data != nil {
		if data.PriceChange1h > 0.8 && data.PriceChange4h > 1.2 {
			return "trend_up"
		}
		if data.PriceChange1h < -0.8 && data.PriceChange4h < -1.2 {
			return "trend_down"
		}
	}
	return "balanced"
}

func regimeNotes(derivatives *DerivativesContext, quant *QuantContext) []string {
	notes := []string{}
	if derivatives != nil {
		if derivatives.FundingBias != "" && derivatives.FundingBias != "neutral" && derivatives.FundingBias != "unknown" {
			notes = append(notes, "funding_bias="+derivatives.FundingBias)
		}
		if derivatives.SqueezeRisk != "" && derivatives.SqueezeRisk != "low" && derivatives.SqueezeRisk != "unknown" {
			notes = append(notes, "squeeze_risk="+derivatives.SqueezeRisk)
		}
	}
	if quant != nil {
		if quant.FlowBias != "" && quant.FlowBias != "unknown" {
			notes = append(notes, "flow_bias="+quant.FlowBias)
		}
		if quant.CrowdingRisk != "" && quant.CrowdingRisk != "low" && quant.CrowdingRisk != "unknown" {
			notes = append(notes, "crowding="+quant.CrowdingRisk)
		}
	}
	return notes
}

func absMarketChange(data *Data) float64 {
	if data == nil {
		return 0
	}
	v := data.PriceChange1h
	if v < 0 {
		return -v
	}
	return v
}
