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

	if math.Abs(data.PriceChange4h) >= 5 {
		return string(market.RegimeLevelTrending)
	}

	return string(classifyRegimeLevel(bollWidth, atrPct))
}

func isTrendAligned(action string, data *market.Data) bool {
	if data == nil {
		return true
	}
	trendUp := data.CurrentPrice >= data.CurrentEMA20 && data.PriceChange4h >= 0
	trendDown := data.CurrentPrice <= data.CurrentEMA20 && data.PriceChange4h <= 0

	switch strings.ToLower(action) {
	case "open_long":
		return trendUp
	case "open_short":
		return trendDown
	default:
		return true
	}
}

func (at *AutoTrader) allowDecisionByRegime(decision *kernel.Decision, data *market.Data) bool {
	if at == nil || at.config.StrategyConfig == nil || decision == nil {
		return true
	}
	cfg := at.config.StrategyConfig.Protection.RegimeFilter
	if !cfg.Enabled {
		return true
	}

	regime := classifyProtectionRegime(data)
	if len(cfg.AllowedRegimes) > 0 {
		allowed := false
		for _, item := range cfg.AllowedRegimes {
			if strings.EqualFold(item, regime) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	if cfg.BlockHighFunding && cfg.MaxFundingRateAbs > 0 && data != nil {
		if math.Abs(data.FundingRate) > cfg.MaxFundingRateAbs {
			return false
		}
	}

	if cfg.BlockHighVolatility && cfg.MaxATR14Pct > 0 && data != nil && data.CurrentPrice > 0 {
		atrPct := 0.0
		if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
			atrPct = data.IntradaySeries.ATR14 / data.CurrentPrice * 100
		} else if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
			atrPct = data.LongerTermContext.ATR14 / data.CurrentPrice * 100
		}
		if atrPct > cfg.MaxATR14Pct {
			return false
		}
	}

	if cfg.RequireTrendAlignment && !isTrendAligned(decision.Action, data) {
		return false
	}

	return true
}

func buildAIProtectionPlan(entryPrice float64, action string, plan *kernel.AIProtectionPlan) (*ProtectionPlan, error) {
	if plan == nil || entryPrice <= 0 {
		return nil, nil
	}

	mode := strings.ToLower(plan.Mode)
	if mode == "" || mode == "full" {
		full := store.FullTPSLConfig{
			Enabled:    true,
			Mode:       store.ProtectionModeAI,
			TakeProfit: store.ProtectionThresholdRule{Enabled: plan.TakeProfitPct > 0, PriceMovePct: plan.TakeProfitPct},
			StopLoss:   store.ProtectionThresholdRule{Enabled: plan.StopLossPct > 0, PriceMovePct: plan.StopLossPct},
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

	if full.StopLoss.Enabled && full.StopLoss.PriceMovePct > 0 {
		move := full.StopLoss.PriceMovePct / 100.0
		if isLong {
			plan.StopLossPrice = entryPrice * (1 - move)
		} else {
			plan.StopLossPrice = entryPrice * (1 + move)
		}
		plan.StopLossPrice = roundProtectionPrice(plan.StopLossPrice)
		plan.NeedsStopLoss = true
	}

	if full.TakeProfit.Enabled && full.TakeProfit.PriceMovePct > 0 {
		move := full.TakeProfit.PriceMovePct / 100.0
		if isLong {
			plan.TakeProfitPrice = entryPrice * (1 + move)
		} else {
			plan.TakeProfitPrice = entryPrice * (1 - move)
		}
		plan.TakeProfitPrice = roundProtectionPrice(plan.TakeProfitPrice)
		plan.NeedsTakeProfit = true
	}

	if !plan.NeedsStopLoss && !plan.NeedsTakeProfit {
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
