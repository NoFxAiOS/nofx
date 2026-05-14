package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
)

func classifyProtectionRegime(data *market.Data) string {
	if data == nil {
		return string(market.RegimeLevelStandard)
	}

	atrPct := 0.0
	if data.CurrentPrice > 0 {
		if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
			atrPct = data.IntradaySeries.ATR14 / data.CurrentPrice * 100
		} else if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
			atrPct = data.LongerTermContext.ATR14 / data.CurrentPrice * 100
		}
	}

	bollWidth := 0.0
	if data.TimeframeData != nil {
		keys := make([]string, 0, len(data.TimeframeData))
		for tf := range data.TimeframeData {
			keys = append(keys, tf)
		}
		sort.Strings(keys)
		for _, tf := range keys {
			series := data.TimeframeData[tf]
			if series == nil || len(series.BOLLUpper) == 0 || len(series.BOLLLower) == 0 || data.CurrentPrice <= 0 {
				continue
			}
			upper := series.BOLLUpper[len(series.BOLLUpper)-1]
			lower := series.BOLLLower[len(series.BOLLLower)-1]
			if upper > 0 && lower > 0 {
				bollWidth = (upper - lower) / data.CurrentPrice * 100
				break
			}
		}
	}

	// Directional trend detection: multi-factor scoring
	// A directional trend needs sustained momentum, not just a single spike
	trendScore := classifyTrendDirection(data)

	// Strong directional trend (score >= 3 of 4 factors)
	if trendScore >= 3 {
		if data.PriceChange4h >= 0 {
			return string(market.RegimeLevelTrendingUp)
		}
		return string(market.RegimeLevelTrendingDown)
	}

	// Moderate directional trend (score == 2) with significant 4h move
	if trendScore >= 2 && math.Abs(data.PriceChange4h) >= 2 {
		if data.PriceChange4h >= 0 {
			return string(market.RegimeLevelTrendingUp)
		}
		return string(market.RegimeLevelTrendingDown)
	}

	// Extreme move — legacy "trending" (either direction, very strong)
	if math.Abs(data.PriceChange4h) >= 5 {
		if data.PriceChange4h >= 0 {
			return string(market.RegimeLevelTrendingUp)
		}
		return string(market.RegimeLevelTrendingDown)
	}

	return string(classifyRegimeLevel(bollWidth, atrPct))
}

// classifyTrendDirection scores how many directional factors align.
// Returns the count of aligned factors (0-4). The direction is determined
// by the majority of factors, but caller uses PriceChange4h for final direction.
func classifyTrendDirection(data *market.Data) int {
	if data == nil {
		return 0
	}

	upScore := 0
	downScore := 0

	// Factor 1: Price vs EMA20
	if data.CurrentPrice > data.CurrentEMA20 {
		upScore++
	} else if data.CurrentPrice < data.CurrentEMA20 {
		downScore++
	}

	// Factor 2: 4h price change direction
	if data.PriceChange4h > 0 {
		upScore++
	} else if data.PriceChange4h < 0 {
		downScore++
	}

	// Factor 3: 1h price change direction
	if data.PriceChange1h > 0 {
		upScore++
	} else if data.PriceChange1h < 0 {
		downScore++
	}

	// Factor 4: MACD direction
	if data.CurrentMACD > 0 {
		upScore++
	} else if data.CurrentMACD < 0 {
		downScore++
	}

	// Return the higher score — represents directional alignment strength
	if upScore >= downScore {
		return upScore
	}
	return downScore
}

// isTrendAligned checks whether the action direction is compatible with the
// current market regime. This is called when RequireTrendAlignment is enabled.
//
// The logic is regime-aware:
// - trending_up: only longs allowed
// - trending_down: only shorts allowed
// - narrow/standard/wide: both directions allowed (range trading is valid)
// - volatile: both directions allowed (but other filters may block)
// - If regime is not directional, fall back to multi-factor scoring
func isTrendAlignedWithMode(action string, setupType string, data *market.Data, mode store.RegimeTrendAlignmentMode) bool {
	if data == nil {
		return true
	}

	regime := classifyProtectionRegime(data)
	act := strings.ToLower(action)
	setup := strings.ToLower(strings.TrimSpace(setupType))

	if mode == store.RegimeTrendAlignmentAllowRangeEdgeReversal && setup == "range_edge" {
		if isRangeEdgeReversalStructurallyPlausible(act, data) {
			return true
		}
	}

	switch regime {
	case string(market.RegimeLevelTrendingUp):
		// Uptrend: longs OK, shorts blocked
		if act == "open_short" {
			return false
		}
		return true

	case string(market.RegimeLevelTrendingDown):
		// Downtrend: shorts OK, longs blocked
		if act == "open_long" {
			return false
		}
		return true

	case string(market.RegimeLevelTrending):
		// Legacy "trending" (either direction) — use direction from 4h change
		if act == "open_long" && data.PriceChange4h < -1 {
			return false
		}
		if act == "open_short" && data.PriceChange4h > 1 {
			return false
		}
		return true

	case string(market.RegimeLevelNarrow), string(market.RegimeLevelStandard), string(market.RegimeLevelWide):
		// Range regimes: both directions are valid (range trading)
		// Only block if there's strong counter-trend evidence (3+ factors against)
		return !isStrongCounterTrend(act, data)

	default:
		// volatile or unknown: allow
		return true
	}
}

// isStrongCounterTrend returns true if 3+ of 4 factors oppose the action direction.
// Used in range regimes to block clearly counter-trend entries.
func isStrongCounterTrend(action string, data *market.Data) bool {
	if data == nil {
		return false
	}
	counterScore := 0
	switch action {
	case "open_long":
		if data.CurrentPrice < data.CurrentEMA20 {
			counterScore++
		}
		if data.PriceChange4h < 0 {
			counterScore++
		}
		if data.PriceChange1h < 0 {
			counterScore++
		}
		if data.CurrentMACD < 0 {
			counterScore++
		}
	case "open_short":
		if data.CurrentPrice > data.CurrentEMA20 {
			counterScore++
		}
		if data.PriceChange4h > 0 {
			counterScore++
		}
		if data.PriceChange1h > 0 {
			counterScore++
		}
		if data.CurrentMACD > 0 {
			counterScore++
		}
	}
	return counterScore >= 3
}

func isTrendAligned(action string, data *market.Data) bool {
	return isTrendAlignedWithMode(action, "", data, store.RegimeTrendAlignmentStrict)
}

// isRangeEdgeReversalStructurallyPlausible allows deliberate support/resistance
// fade entries to pass the directional trend gate when the strategy explicitly
// opts in. It is intentionally conservative: only range_edge setups, near a
// Bollinger edge, with non-extreme short-term momentum may bypass strict trend
// direction. RR/structure/protection gates still run after this.
func isRangeEdgeReversalStructurallyPlausible(action string, data *market.Data) bool {
	if data == nil || data.CurrentPrice <= 0 {
		return false
	}
	atrPct := 0.0
	if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
		atrPct = data.IntradaySeries.ATR14 / data.CurrentPrice * 100
	} else if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
		atrPct = data.LongerTermContext.ATR14 / data.CurrentPrice * 100
	}
	if atrPct <= 0 || atrPct > 1.5 {
		return false
	}
	lower, upper := latestBollingerBand(data)
	if lower <= 0 || upper <= 0 || upper <= lower {
		return false
	}
	edgeTolerance := data.CurrentPrice * math.Max(atrPct*1.2, 0.15) / 100.0
	switch strings.ToLower(action) {
	case "open_long":
		return data.CurrentPrice <= lower+edgeTolerance && data.PriceChange1h > -2.5
	case "open_short":
		return data.CurrentPrice >= upper-edgeTolerance && data.PriceChange1h < 2.5
	default:
		return true
	}
}

func latestBollingerBand(data *market.Data) (float64, float64) {
	if data == nil || data.TimeframeData == nil {
		return 0, 0
	}
	for _, tf := range []string{"15m", "5m", "3m", "1h"} {
		series := data.TimeframeData[tf]
		if series == nil || len(series.BOLLLower) == 0 || len(series.BOLLUpper) == 0 {
			continue
		}
		return series.BOLLLower[len(series.BOLLLower)-1], series.BOLLUpper[len(series.BOLLUpper)-1]
	}
	return 0, 0
}

func (at *AutoTrader) evaluateDecisionRegimeGate(decision *kernel.Decision, data *market.Data) regimeGateResult {
	result := regimeGateResult{Allowed: true}
	if at == nil || at.config.StrategyConfig == nil || decision == nil {
		return result
	}
	cfg := at.config.StrategyConfig.Protection.RegimeFilter
	if !cfg.Enabled {
		return result
	}

	regime := classifyProtectionRegime(data)
	result.CurrentRegime = regime
	result.AllowedRegimes = append([]string{}, cfg.AllowedRegimes...)
	if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
		result.PrimaryTimeframe = at.strategyEngine.GetConfig().Indicators.Klines.PrimaryTimeframe
	}
	if data != nil {
		result.FundingRate = data.FundingRate
		if data.CurrentPrice > 0 {
			if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
				result.ATR14Pct = data.IntradaySeries.ATR14 / data.CurrentPrice * 100
			} else if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
				result.ATR14Pct = data.LongerTermContext.ATR14 / data.CurrentPrice * 100
			}
		}
	}

	if len(cfg.AllowedRegimes) > 0 {
		allowed := false
		for _, item := range cfg.AllowedRegimes {
			if strings.EqualFold(item, regime) {
				allowed = true
				break
			}
			// "trending" in config matches trending_up and trending_down
			if strings.EqualFold(item, "trending") &&
				(strings.EqualFold(regime, "trending_up") || strings.EqualFold(regime, "trending_down")) {
				allowed = true
				break
			}
		}
		if !allowed {
			result.Allowed = false
			result.ReasonCode = "regime_not_allowed"
			result.Reason = fmt.Sprintf("regime %s not allowed (allowed=%s)", regime, strings.Join(cfg.AllowedRegimes, ","))
			return result
		}
	}

	if cfg.BlockHighFunding && cfg.MaxFundingRateAbs > 0 && data != nil {
		if math.Abs(data.FundingRate) > cfg.MaxFundingRateAbs {
			result.Allowed = false
			result.ReasonCode = "funding_above_max"
			result.Reason = fmt.Sprintf("funding %.6f exceeds max %.6f", data.FundingRate, cfg.MaxFundingRateAbs)
			return result
		}
	}

	if cfg.BlockHighVolatility && cfg.MaxATR14Pct > 0 && result.ATR14Pct > cfg.MaxATR14Pct {
		result.Allowed = false
		result.ReasonCode = "atr_above_max"
		result.Reason = fmt.Sprintf("atr14_pct %.2f exceeds max %.2f", result.ATR14Pct, cfg.MaxATR14Pct)
		return result
	}

	if cfg.RequireTrendAlignment {
		aligned := isTrendAlignedWithMode(decision.Action, decision.SetupType, data, cfg.TrendAlignmentMode)
		result.TrendAligned = &aligned
		if !aligned {
			result.Allowed = false
			result.ReasonCode = "trend_misaligned"
			if cfg.TrendAlignmentMode == store.RegimeTrendAlignmentAllowRangeEdgeReversal {
				result.Reason = fmt.Sprintf("trend alignment failed for %s under regime gate (range_edge reversal exception not satisfied)", decision.Action)
			} else {
				result.Reason = fmt.Sprintf("trend alignment failed for %s under regime gate", decision.Action)
			}
			return result
		}
	}

	return result
}

func (at *AutoTrader) allowDecisionByRegime(decision *kernel.Decision, data *market.Data) bool {
	return at.evaluateDecisionRegimeGate(decision, data).Allowed
}

func buildAIBreakEvenConfig(plan *kernel.AIProtectionPlan) *store.BreakEvenStopConfig {
	if plan == nil {
		return nil
	}
	mode := strings.ToLower(plan.Mode)
	if mode != "break_even" && mode != "combined" {
		return nil
	}
	if plan.BreakEvenTrigger == "" || plan.BreakEvenValue <= 0 || plan.BreakEvenOffset < 0 {
		return nil
	}
	triggerMode := store.BreakEvenTriggerMode(plan.BreakEvenTrigger)
	if triggerMode != store.BreakEvenTriggerProfitPct && triggerMode != store.BreakEvenTriggerRMultiple {
		return nil
	}
	cfg := &store.BreakEvenStopConfig{
		Enabled:      true,
		TriggerMode:  triggerMode,
		TriggerValue: plan.BreakEvenValue,
		OffsetPct:    plan.BreakEvenOffset,
	}
	return cfg
}

func clampAIDrawdownCloseRatio(rule store.DrawdownTakeProfitRule, cfg store.DrawdownTakeProfitConfig) store.DrawdownTakeProfitRule {
	// No clamping needed — close_ratio_pct comes directly from strategy config
	// (manual value) or AI decision. The strategy config's close_ratio_pct is
	// applied at the tier-matching stage in clampAIDrawdownTierCeilings, not here.
	return rule
}

// clampAIDrawdownTierCeilings applies strategy-configured close_ratio_pct to AI drawdown rules.
// Strategy config close_ratio_pct is always authoritative — AI only provides min_profit and max_drawdown
// when their mode is "ai". The number of tiers comes from AI (can be 3, 4, 5, etc.).
func clampAIDrawdownTierCeilings(rules []store.DrawdownTakeProfitRule, cfg store.DrawdownTakeProfitConfig) []store.DrawdownTakeProfitRule {
	if len(rules) == 0 {
		return rules
	}

	// Sort AI rules by min_profit_pct ascending for tier matching.
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].MinProfitPct < rules[j].MinProfitPct
	})

	// Apply strategy config close_ratio_pct where available (positional match).
	for i := range rules {
		if i < len(cfg.Rules) && cfg.Rules[i].CloseRatioPct > 0 {
			rules[i].CloseRatioPct = cfg.Rules[i].CloseRatioPct
		}
	}

	// Deduplicate: ensure strictly increasing MinProfitPct across tiers.
	for i := 1; i < len(rules); i++ {
		if rules[i].MinProfitPct <= rules[i-1].MinProfitPct {
			rules[i].MinProfitPct = rules[i-1].MinProfitPct + 0.3
		}
	}

	return rules
}

func buildAIProtectionPlan(entryPrice float64, action string, plan *kernel.AIProtectionPlan, cfgs ...*store.StrategyConfig) (*ProtectionPlan, error) {
	if plan == nil || entryPrice <= 0 {
		return nil, nil
	}

	mode := strings.ToLower(plan.Mode)
	drawdownCfg := store.DrawdownTakeProfitConfig{EngineMode: store.DrawdownEngineModeAI, RunnerEnabled: true, MinRunnerKeepPct: 30, MaxFirstReducePct: 65}
	if len(cfgs) > 0 && cfgs[0] != nil {
		drawdownCfg = cfgs[0].Protection.DrawdownTakeProfit
	}
	if mode == "break_even" {
		be := buildAIBreakEvenConfig(plan)
		if be == nil {
			return nil, nil
		}
		return &ProtectionPlan{Mode: string(store.ProtectionModeAI), BreakEvenConfig: be}, nil
	}
	if mode == "" || mode == "full" {
		return buildAIDecisionFullProtectionPlan(entryPrice, action, plan)
	}

	if mode == "ladder" {
		return buildAIDecisionLadderProtectionPlan(entryPrice, action, plan.LadderRules, cfgs...)
	}

	if mode == "drawdown" {
		rules := make([]store.DrawdownTakeProfitRule, 0, len(plan.DrawdownRules))
		for _, rule := range plan.DrawdownRules {
			drawdownRule := clampAIDrawdownCloseRatio(store.DrawdownTakeProfitRule{
				Timeframe:           rule.Timeframe,
				MinProfitPct:        rule.MinProfitPct,
				MaxDrawdownPct:      rule.MaxDrawdownPct,
				MaxDrawdownAbsPct:   rule.MaxDrawdownAbsPct,
				CloseRatioPct:       rule.CloseRatioPct,
				PollIntervalSeconds: rule.PollIntervalSeconds,
				ReasonAnchor:        rule.ReasonAnchor,
				StageName:           rule.StageName,
				RunnerKeepPct:       rule.RunnerKeepPct,
				RunnerStopMode:      rule.RunnerStopMode,
				RunnerStopSource:    rule.RunnerStopSource,
				RunnerTargetMode:    rule.RunnerTargetMode,
				RunnerTargetSource:  rule.RunnerTargetSource,
			}, drawdownCfg)
			if drawdownRule.CloseRatioPct > 0 {
				rules = append(rules, drawdownRule)
			}
		}
		rules = clampAIDrawdownTierCeilings(rules, drawdownCfg)
		return &ProtectionPlan{Mode: string(store.ProtectionModeAI), DrawdownRules: rules}, nil
	}

	if mode == "combined" {
		if len(plan.LadderRules) == 0 || len(plan.DrawdownRules) == 0 {
			return nil, nil
		}
		parts := make([]*ProtectionPlan, 0, 3)
		if len(plan.LadderRules) > 0 {
			if p, err := buildAIDecisionLadderProtectionPlan(entryPrice, action, plan.LadderRules, cfgs...); err != nil {
				return nil, err
			} else if p != nil {
				parts = append(parts, p)
			}
		}
		if len(plan.DrawdownRules) > 0 {
			rules := make([]store.DrawdownTakeProfitRule, 0, len(plan.DrawdownRules))
			for _, rule := range plan.DrawdownRules {
				drawdownRule := clampAIDrawdownCloseRatio(store.DrawdownTakeProfitRule{
					Timeframe:           rule.Timeframe,
					MinProfitPct:        rule.MinProfitPct,
					MaxDrawdownPct:      rule.MaxDrawdownPct,
					MaxDrawdownAbsPct:   rule.MaxDrawdownAbsPct,
					CloseRatioPct:       rule.CloseRatioPct,
					PollIntervalSeconds: rule.PollIntervalSeconds,
					ReasonAnchor:        rule.ReasonAnchor,
					StageName:           rule.StageName,
					RunnerKeepPct:       rule.RunnerKeepPct,
					RunnerStopMode:      rule.RunnerStopMode,
					RunnerStopSource:    rule.RunnerStopSource,
					RunnerTargetMode:    rule.RunnerTargetMode,
					RunnerTargetSource:  rule.RunnerTargetSource,
				}, drawdownCfg)
			if drawdownRule.CloseRatioPct > 0 {
					rules = append(rules, drawdownRule)
				}
			}
			rules = clampAIDrawdownTierCeilings(rules, drawdownCfg)
			parts = append(parts, &ProtectionPlan{Mode: string(store.ProtectionModeAI), DrawdownRules: rules})
		}
		if be := buildAIBreakEvenConfig(plan); be != nil {
			parts = append(parts, &ProtectionPlan{Mode: string(store.ProtectionModeAI), BreakEvenConfig: be})
		}
		return mergeProtectionPlans(parts...), nil
	}

	return nil, nil
}

func buildAIDecisionFullProtectionPlan(entryPrice float64, action string, plan *kernel.AIProtectionPlan) (*ProtectionPlan, error) {
	if plan == nil || entryPrice <= 0 {
		return nil, nil
	}
	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil, nil
	}
	out := &ProtectionPlan{Mode: string(store.ProtectionModeAI), RequiresNativeOrders: true}
	if plan.StopLossPrice > 0 && isExecutableStopLossPrice(entryPrice, action, plan.StopLossPrice) {
		out.StopLossPrice = roundProtectionPrice(plan.StopLossPrice)
		out.NeedsStopLoss = true
	} else if plan.StopLossPct > 0 {
		move := plan.StopLossPct / 100.0
		if isLong {
			out.StopLossPrice = roundProtectionPrice(entryPrice * (1 - move))
		} else {
			out.StopLossPrice = roundProtectionPrice(entryPrice * (1 + move))
		}
		out.NeedsStopLoss = true
	}
	if plan.TakeProfitPrice > 0 && isExecutableTakeProfitPrice(entryPrice, action, plan.TakeProfitPrice) {
		out.TakeProfitPrice = roundProtectionPrice(plan.TakeProfitPrice)
		out.NeedsTakeProfit = true
	} else if plan.TakeProfitPct > 0 {
		move := plan.TakeProfitPct / 100.0
		if isLong {
			out.TakeProfitPrice = roundProtectionPrice(entryPrice * (1 + move))
		} else {
			out.TakeProfitPrice = roundProtectionPrice(entryPrice * (1 - move))
		}
		out.NeedsTakeProfit = true
	}
	if !out.NeedsStopLoss && !out.NeedsTakeProfit {
		return nil, nil
	}
	return out, nil
}

func buildAIFullProtectionPlan(entryPrice float64, action string, full store.FullTPSLConfig) (*ProtectionPlan, error) {
	if !full.Enabled || full.Mode != store.ProtectionModeAI {
		return nil, nil
	}
	if entryPrice <= 0 {
		return nil, fmt.Errorf("invalid entry price %.8f for ai full protection plan", entryPrice)
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil, nil
	}

	plan := &ProtectionPlan{Mode: string(store.ProtectionModeAI), RequiresNativeOrders: true}

	if stopLossPct, ok := resolveFullStopLoss(full, full.StopLoss.Value); ok {
		move := stopLossPct / 100.0
		if isLong {
			plan.StopLossPrice = entryPrice * (1 - move)
		} else {
			plan.StopLossPrice = entryPrice * (1 + move)
		}
		plan.StopLossPrice = roundProtectionPrice(plan.StopLossPrice)
		plan.NeedsStopLoss = true
	}

	if fallbackPct, ok := resolveFallbackMaxLoss(full); ok {
		move := fallbackPct / 100.0
		if isLong {
			plan.FallbackMaxLossPrice = roundProtectionPrice(entryPrice * (1 - move))
		} else {
			plan.FallbackMaxLossPrice = roundProtectionPrice(entryPrice * (1 + move))
		}
	}

	if takeProfitPct, ok := resolveFullTakeProfit(full, full.TakeProfit.Value); ok {
		move := takeProfitPct / 100.0
		if isLong {
			plan.TakeProfitPrice = entryPrice * (1 + move)
		} else {
			plan.TakeProfitPrice = entryPrice * (1 - move)
		}
		plan.TakeProfitPrice = roundProtectionPrice(plan.TakeProfitPrice)
		plan.NeedsTakeProfit = true
	}

	if !plan.NeedsStopLoss && !plan.NeedsTakeProfit && plan.FallbackMaxLossPrice == 0 {
		return nil, nil
	}

	return plan, nil
}

func buildAIDecisionLadderProtectionPlan(entryPrice float64, action string, rules []kernel.AIProtectionLadderRule, cfgs ...*store.StrategyConfig) (*ProtectionPlan, error) {
	if entryPrice <= 0 || len(rules) == 0 {
		return nil, nil
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil, nil
	}

	plan := &ProtectionPlan{
		Mode:                 string(store.ProtectionModeAI),
		RequiresNativeOrders: true,
		RequiresPartialClose: true,
	}

	remainingTakeProfitRatio := 100.0
	remainingStopLossRatio := 100.0
	lastSLIndex := -1
	for i := len(rules) - 1; i >= 0; i-- {
		if rules[i].StopLossCloseRatioPct > 0 {
			price := resolveAIDecisionLadderPrice(entryPrice, action, rules[i].StopLossPct, rules[i].StopLossPrice, rules[i].VolatilityBufferPct, false)
			if price > 0 && isExecutableStopLossPrice(entryPrice, action, price) {
				lastSLIndex = i
				break
			}
		}
	}
	for i, rule := range rules {
		if rule.TakeProfitCloseRatioPct > 0 && remainingTakeProfitRatio > 0 {
			price := resolveAIDecisionLadderPrice(entryPrice, action, rule.TakeProfitPct, rule.TakeProfitPrice, rule.VolatilityBufferPct, true)
			if price > 0 && isExecutableTakeProfitPrice(entryPrice, action, price) {
				closeRatio := minPositive(rule.TakeProfitCloseRatioPct, remainingTakeProfitRatio)
				order := ProtectionOrder{Price: roundProtectionPrice(price), CloseRatioPct: closeRatio}
				order.BasisType = rule.BasisType
				if rule.TakeProfitAnchor != "" {
					order.AnchorSource = rule.TakeProfitAnchor
				} else if rule.StructuralAnchor != "" {
					order.AnchorSource = rule.StructuralAnchor
				}
				plan.TakeProfitOrders = append(plan.TakeProfitOrders, order)
				remainingTakeProfitRatio -= closeRatio
			}
		}

		if rule.StopLossCloseRatioPct > 0 && (remainingStopLossRatio > 0 || i == lastSLIndex) {
			price := resolveAIDecisionLadderPrice(entryPrice, action, rule.StopLossPct, rule.StopLossPrice, rule.VolatilityBufferPct, false)
			if price > 0 && isExecutableStopLossPrice(entryPrice, action, price) {
				closeRatio := minPositive(rule.StopLossCloseRatioPct, remainingStopLossRatio)
				if i == lastSLIndex {
					closeRatio = 100
				}
				order := ProtectionOrder{Price: roundProtectionPrice(price), CloseRatioPct: closeRatio}
				order.BasisType = rule.BasisType
				if rule.StopLossAnchor != "" {
					order.AnchorSource = rule.StopLossAnchor
				} else if rule.StructuralAnchor != "" {
					order.AnchorSource = rule.StructuralAnchor
				}
				plan.StopLossOrders = append(plan.StopLossOrders, order)
				remainingStopLossRatio -= closeRatio
			}
		}
	}

	if len(plan.StopLossOrders) == 0 && len(plan.TakeProfitOrders) == 0 {
		return nil, nil
	}

	// Volatility buffer auto-widening: ensure SL orders have enough ATR distance.
	// If SL is too tight but TP/SL ratio still valid after widening, widen SL automatically.
	if len(plan.StopLossOrders) > 0 && len(rules) > 0 {
		var atr14Pct float64
		var volBuf float64
		var bufferMul float64
		if len(cfgs) > 0 && cfgs[0] != nil {
			gd := cfgs[0].EntryStructure.EntryGate.WithDefaults()
			volBuf = gd.VolatilityBufferATRMul
			bufferMul = volBuf * 0.7
			if bufferMul <= 0 {
				bufferMul = 0.35
			}
		} else {
			bufferMul = 0.35
		}
		for _, r := range rules {
			if r.VolatilityBufferPct > 0 {
				atr14Pct = r.VolatilityBufferPct / bufferMul
				break
			}
		}
		if atr14Pct <= 0 && len(cfgs) > 0 && cfgs[0] != nil {
			gd := cfgs[0].EntryStructure.EntryGate.WithDefaults()
			if gd.MinATR14Pct > 0 {
				atr14Pct = gd.MinATR14Pct
			}
		}
		if atr14Pct > 0 && volBuf > 0 {
			atrAbs := entryPrice * (atr14Pct / 100)
			minSLMul := 1.2
			if len(cfgs) > 0 && cfgs[0] != nil && cfgs[0].EntryStructure.EntryGate.MinSLDistanceATRMul > 0 {
				minSLMul = cfgs[0].EntryStructure.EntryGate.MinSLDistanceATRMul
			}
			minSLDist := (minSLMul + volBuf) * atrAbs
			for i := range plan.StopLossOrders {
				sl := &plan.StopLossOrders[i]
				dist := math.Abs(entryPrice - sl.Price)
				if dist < minSLDist {
					if isLong {
						sl.Price = roundProtectionPrice(entryPrice - minSLDist)
					} else {
						sl.Price = roundProtectionPrice(entryPrice + minSLDist)
					}
					logger.Infof("🛡️ volatility auto-widen SL[%d]: dist %.4f < min %.4f (%.2fx ATR), widened to %.8f", i, dist, minSLDist, minSLDist/atrAbs, sl.Price)
				}
			}
		}
	}

	plan.NeedsStopLoss = len(plan.StopLossOrders) > 0
	plan.NeedsTakeProfit = len(plan.TakeProfitOrders) > 0
	return plan, nil
}

func resolveAIDecisionLadderPrice(entryPrice float64, action string, pct, absolute, bufferPct float64, takeProfit bool) float64 {
	price := absolute
	derivedFromPct := false
	if price <= 0 && pct > 0 {
		move := pct / 100.0
		derivedFromPct = true
		if takeProfit {
			if action == "open_long" {
				price = entryPrice * (1 + move)
			} else {
				price = entryPrice * (1 - move)
			}
		} else {
			if action == "open_long" {
				price = entryPrice * (1 - move)
			} else {
				price = entryPrice * (1 + move)
			}
		}
	}
	// AI already returns absolute structural prices with buffers applied in most cases.
	// Only apply volatility_buffer_pct as a price adjustment when the price was derived
	// locally from a percent fallback, otherwise we double-buffer and detach from structure.
	if derivedFromPct && price > 0 && bufferPct > 0 {
		bufferMove := entryPrice * bufferPct / 100.0
		switch action {
		case "open_long":
			price -= bufferMove
		case "open_short":
			price += bufferMove
		}
	}
	return price
}

func isExecutableTakeProfitPrice(entryPrice float64, action string, price float64) bool {
	if entryPrice <= 0 || price <= 0 {
		return false
	}
	return (action == "open_long" && price > entryPrice) || (action == "open_short" && price < entryPrice)
}

func isExecutableStopLossPrice(entryPrice float64, action string, price float64) bool {
	if entryPrice <= 0 || price <= 0 {
		return false
	}
	return (action == "open_long" && price < entryPrice) || (action == "open_short" && price > entryPrice)
}

func buildAILadderProtectionPlan(entryPrice float64, action string, ladder store.LadderTPSLConfig) (*ProtectionPlan, error) {
	if !ladder.Enabled || ladder.Mode != store.ProtectionModeAI {
		return nil, nil
	}
	if entryPrice <= 0 {
		return nil, fmt.Errorf("invalid entry price %.8f for ai ladder protection plan", entryPrice)
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil, nil
	}

	plan := &ProtectionPlan{
		Mode:                 string(store.ProtectionModeAI),
		RequiresNativeOrders: true,
		RequiresPartialClose: true,
	}

	remainingTakeProfitRatio := 100.0
	remainingStopLossRatio := 100.0
	for _, rule := range ladder.Rules {
		if ladder.TakeProfitEnabled && rule.TakeProfitPct > 0 && rule.TakeProfitCloseRatioPct > 0 && remainingTakeProfitRatio > 0 {
			closeRatio := minPositive(rule.TakeProfitCloseRatioPct, remainingTakeProfitRatio)
			move := rule.TakeProfitPct / 100.0
			price := entryPrice
			if isLong {
				price = entryPrice * (1 + move)
			} else {
				price = entryPrice * (1 - move)
			}
			price = roundProtectionPrice(price)
			plan.TakeProfitOrders = append(plan.TakeProfitOrders, ProtectionOrder{Price: price, CloseRatioPct: closeRatio})
			remainingTakeProfitRatio -= closeRatio
		}

		if ladder.StopLossEnabled && rule.StopLossPct > 0 && rule.StopLossCloseRatioPct > 0 && remainingStopLossRatio > 0 {
			closeRatio := minPositive(rule.StopLossCloseRatioPct, remainingStopLossRatio)
			move := rule.StopLossPct / 100.0
			price := entryPrice
			if isLong {
				price = entryPrice * (1 - move)
			} else {
				price = entryPrice * (1 + move)
			}
			price = roundProtectionPrice(price)
			plan.StopLossOrders = append(plan.StopLossOrders, ProtectionOrder{Price: price, CloseRatioPct: closeRatio})
			remainingStopLossRatio -= closeRatio
		}
	}

	if len(plan.StopLossOrders) == 0 && len(plan.TakeProfitOrders) == 0 {
		return nil, nil
	}
	plan.NeedsStopLoss = len(plan.StopLossOrders) > 0
	plan.NeedsTakeProfit = len(plan.TakeProfitOrders) > 0
	return plan, nil
}
