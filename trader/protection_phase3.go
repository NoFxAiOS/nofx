package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
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
func isTrendAligned(action string, data *market.Data) bool {
	if data == nil {
		return true
	}

	regime := classifyProtectionRegime(data)
	act := strings.ToLower(action)

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
	atrPct := 0.0
	if data.CurrentPrice > 0 {
		if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
			atrPct = data.IntradaySeries.ATR14 / data.CurrentPrice * 100
		} else if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
			atrPct = data.LongerTermContext.ATR14 / data.CurrentPrice * 100
		}
	}
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
		// Allow shallow-retest / support-bounce longs in range regimes unless the setup
		// is extremely counter-trend and momentum remains broadly negative.
		if counterScore == 3 && atrPct > 0 && atrPct <= 1.2 && data.PriceChange1h > -1.2 {
			return false
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
		if counterScore == 3 && atrPct > 0 && atrPct <= 1.2 && data.PriceChange1h < 1.2 {
			return false
		}
	}
	return counterScore >= 4
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
		aligned := isTrendAligned(decision.Action, data)
		result.TrendAligned = &aligned
		if !aligned {
			result.Allowed = false
			result.ReasonCode = "trend_misaligned"
			result.Reason = fmt.Sprintf("trend alignment failed for %s under regime gate", decision.Action)
			return result
		}
	}

	return result
}

func (at *AutoTrader) allowDecisionByRegime(decision *kernel.Decision, data *market.Data) bool {
	return at.evaluateDecisionRegimeGate(decision, data).Allowed
}

func buildAIBreakEvenConfig(plan *kernel.AIProtectionPlan) *store.BreakEvenStopConfig {
	if plan == nil || strings.ToLower(plan.Mode) != "break_even" {
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

func buildAIProtectionPlan(entryPrice float64, action string, plan *kernel.AIProtectionPlan) (*ProtectionPlan, error) {
	if plan == nil || entryPrice <= 0 {
		return nil, nil
	}

	mode := strings.ToLower(plan.Mode)
	if mode == "break_even" {
		be := buildAIBreakEvenConfig(plan)
		if be == nil {
			return nil, nil
		}
		return &ProtectionPlan{Mode: string(store.ProtectionModeAI), BreakEvenConfig: be}, nil
	}
	if mode == "" || mode == "full" {
		full := store.FullTPSLConfig{
			Enabled:    true,
			Mode:       store.ProtectionModeAI,
			TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: plan.TakeProfitPct},
			StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: plan.StopLossPct},
		}
		return buildAIFullProtectionPlan(entryPrice, action, full)
	}

	if mode == "ladder" {
		ladderRules := make([]store.LadderTPSLRule, 0, len(plan.LadderRules))
		for _, rule := range plan.LadderRules {
			ladderRules = append(ladderRules, store.LadderTPSLRule{
				TakeProfitPct:           rule.TakeProfitPct,
				TakeProfitCloseRatioPct: rule.TakeProfitCloseRatioPct,
				StopLossPct:             rule.StopLossPct,
				StopLossCloseRatioPct:   rule.StopLossCloseRatioPct,
			})
		}
		ladder := store.LadderTPSLConfig{
			Enabled:           true,
			Mode:              store.ProtectionModeAI,
			TakeProfitEnabled: true,
			StopLossEnabled:   true,
			Rules:             ladderRules,
		}
		return buildAILadderProtectionPlan(entryPrice, action, ladder)
	}

	if mode == "drawdown" {
		rules := make([]store.DrawdownTakeProfitRule, 0, len(plan.DrawdownRules))
		for _, rule := range plan.DrawdownRules {
			rules = append(rules, store.DrawdownTakeProfitRule{
				MinProfitPct:        rule.MinProfitPct,
				MaxDrawdownPct:      rule.MaxDrawdownPct,
				CloseRatioPct:       rule.CloseRatioPct,
				PollIntervalSeconds: rule.PollIntervalSeconds,
				StageName:           rule.StageName,
				RunnerKeepPct:       rule.RunnerKeepPct,
				RunnerStopMode:      rule.RunnerStopMode,
				RunnerStopSource:    rule.RunnerStopSource,
				RunnerTargetMode:    rule.RunnerTargetMode,
				RunnerTargetSource:  rule.RunnerTargetSource,
			})
		}
		return &ProtectionPlan{Mode: string(store.ProtectionModeAI), DrawdownRules: rules}, nil
	}

	if mode == "combined" {
		if len(plan.LadderRules) == 0 || len(plan.DrawdownRules) == 0 {
			return nil, nil
		}
		parts := make([]*ProtectionPlan, 0, 2)
		if len(plan.LadderRules) > 0 {
			ladderRules := make([]store.LadderTPSLRule, 0, len(plan.LadderRules))
			for _, rule := range plan.LadderRules {
				ladderRules = append(ladderRules, store.LadderTPSLRule{
					TakeProfitPct:           rule.TakeProfitPct,
					TakeProfitCloseRatioPct: rule.TakeProfitCloseRatioPct,
					StopLossPct:             rule.StopLossPct,
					StopLossCloseRatioPct:   rule.StopLossCloseRatioPct,
				})
			}
			ladder := store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI, TakeProfitEnabled: true, StopLossEnabled: true, Rules: ladderRules}
			if p, err := buildAILadderProtectionPlan(entryPrice, action, ladder); err != nil {
				return nil, err
			} else if p != nil {
				parts = append(parts, p)
			}
		}
		if len(plan.DrawdownRules) > 0 {
			rules := make([]store.DrawdownTakeProfitRule, 0, len(plan.DrawdownRules))
			for _, rule := range plan.DrawdownRules {
				rules = append(rules, store.DrawdownTakeProfitRule{
					MinProfitPct:        rule.MinProfitPct,
					MaxDrawdownPct:      rule.MaxDrawdownPct,
					CloseRatioPct:       rule.CloseRatioPct,
					PollIntervalSeconds: rule.PollIntervalSeconds,
					StageName:           rule.StageName,
					RunnerKeepPct:       rule.RunnerKeepPct,
					RunnerStopMode:      rule.RunnerStopMode,
					RunnerStopSource:    rule.RunnerStopSource,
					RunnerTargetMode:    rule.RunnerTargetMode,
					RunnerTargetSource:  rule.RunnerTargetSource,
				})
			}
			parts = append(parts, &ProtectionPlan{Mode: string(store.ProtectionModeAI), DrawdownRules: rules})
		}
		return mergeProtectionPlans(parts...), nil
	}

	return nil, nil
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
