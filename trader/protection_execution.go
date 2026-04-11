package trader

import (
	"fmt"
	"math"
	"strings"

	"nofx/kernel"
	"nofx/logger"
	tradertypes "nofx/trader/types"
)

const (
	protectionPriceTolerancePct = 0.002 // 0.2%
	protectionSetupMaxAttempts  = 2
)

type protectionExecutionRequest struct {
	Symbol       string
	Action       string
	PositionSide string
	Quantity     float64
	EntryPrice   float64
	Decision     *kernel.Decision
}

func (at *AutoTrader) applyPostOpenProtection(req *protectionExecutionRequest) error {
	if req == nil || req.Decision == nil {
		return nil
	}

	plan, err := at.BuildManualProtectionPlan(req.EntryPrice, req.Symbol, req.Action)
	if err != nil {
		return err
	}

	if plan == nil && req.Decision.ProtectionPlan != nil {
		plan, err = buildAIProtectionPlan(req.EntryPrice, req.Action, req.Decision.ProtectionPlan)
		if err != nil {
			return err
		}
	}

	if plan != nil {
		caps := at.GetProtectionCapabilities()
		if plan.RequiresNativeOrders && (!caps.NativeStopLoss && plan.NeedsStopLoss || !caps.NativeTakeProfit && plan.NeedsTakeProfit) {
			return fmt.Errorf("exchange %s cannot safely support required native protection orders", at.exchange)
		}
		if plan.RequiresPartialClose && !caps.NativePartialClose {
			return fmt.Errorf("exchange %s cannot safely support ladder partial-close protection", at.exchange)
		}

		logger.Infof("  🛡 Applying %s protection plan: stop=%v tp=%v ladderSL=%d ladderTP=%d",
			plan.Mode, plan.NeedsStopLoss, plan.NeedsTakeProfit, len(plan.StopLossOrders), len(plan.TakeProfitOrders))
		if err := at.placeAndVerifyProtectionPlanWithRetry(req.Symbol, req.PositionSide, req.Quantity, plan); err != nil {
			return err
		}
	}

	if err := at.applyNativeProtectionTargetsAfterOpen(req); err != nil {
		return err
	}

	if plan != nil {
		return nil
	}

	needsStopLoss := req.Decision.StopLoss > 0
	needsTakeProfit := req.Decision.TakeProfit > 0
	if !needsStopLoss && !needsTakeProfit {
		return nil
	}

	logger.Infof("  🛡 Applying AI decision protection fallback: stop=%v@%.6f tp=%v@%.6f",
		needsStopLoss, req.Decision.StopLoss, needsTakeProfit, req.Decision.TakeProfit)
	return at.placeAndVerifyProtectionWithRetry(req.Symbol, req.PositionSide, req.Quantity, needsStopLoss, req.Decision.StopLoss, needsTakeProfit, req.Decision.TakeProfit)
}

func (at *AutoTrader) applyNativeProtectionTargetsAfterOpen(req *protectionExecutionRequest) error {
	if req == nil || req.Decision == nil || at.config.StrategyConfig == nil {
		return nil
	}

	prot := at.config.StrategyConfig.Protection

	// 1. Native drawdown/trailing should be armed as early as safely possible.
	for _, rule := range prot.DrawdownTakeProfit.Rules {
		if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
			continue
		}
		if rule.CloseRatioPct >= 99.999 {
			_ = at.applyNativeTrailingDrawdown(req.Symbol, strings.TrimPrefix(strings.ToLower(req.PositionSide), ""), req.EntryPrice, rule)
			continue
		}

		candidate := buildPartialDrawdownNativePlanCandidate(req.EntryPrice, req.Action, rule)
		if candidate == nil {
			continue
		}
		if at.canApplyNativePartialDrawdownPlan(candidate) {
			logger.Infof("  🛡 Applying native partial drawdown candidate: symbol=%s side=%s close=%.1f%%",
				req.Symbol, req.PositionSide, rule.CloseRatioPct)
			if err := at.placeAndVerifyProtectionPlanWithRetry(req.Symbol, req.PositionSide, req.Quantity, candidate); err != nil {
				logger.Warnf("  ⚠️ Native partial drawdown candidate failed for %s %s: %v", req.Symbol, req.PositionSide, err)
			} else {
				at.setProtectionState(req.Symbol, strings.ToLower(req.PositionSide), "native_partial_trailing_armed")
			}
		}
	}

	// 2. Break-even should prefer exchange-native stop as soon as trigger condition is met.
	if be := at.getActiveBreakEvenConfig(); be != nil && be.TriggerValue <= 0 {
		if err := at.applyBreakEvenStop(req.Symbol, strings.ToLower(req.PositionSide), req.Quantity, req.EntryPrice, be.TriggerValue, *be); err == nil {
			at.setBreakEvenState(req.Symbol, strings.ToLower(req.PositionSide), "armed")
			at.setProtectionState(req.Symbol, strings.ToLower(req.PositionSide), "break_even_armed")
		}
	}

	return nil
}

func (at *AutoTrader) canApplyNativePartialDrawdownPlan(plan *ProtectionPlan) bool {
	if plan == nil || !plan.RequiresPartialClose {
		return false
	}
	caps := at.GetProtectionCapabilities()
	return caps.NativePartialClose && caps.NativeTakeProfit
}

func (at *AutoTrader) placeAndVerifyProtectionPlanWithRetry(symbol, positionSide string, quantity float64, plan *ProtectionPlan) error {
	var lastErr error
	for attempt := 1; attempt <= protectionSetupMaxAttempts; attempt++ {
		if err := at.placeAndVerifyProtectionPlan(symbol, positionSide, quantity, plan); err != nil {
			lastErr = err
			logger.Warnf("  ⚠️ Protection plan attempt %d/%d failed for %s %s: %v", attempt, protectionSetupMaxAttempts, symbol, positionSide, err)
			continue
		}
		if attempt > 1 {
			logger.Infof("  ✅ Protection plan recovered on retry %d/%d for %s %s", attempt, protectionSetupMaxAttempts, symbol, positionSide)
		}
		return nil
	}
	return fmt.Errorf("protection plan setup failed after %d attempts: %w", protectionSetupMaxAttempts, lastErr)
}

func (at *AutoTrader) placeAndVerifyProtectionWithRetry(symbol, positionSide string, quantity float64, needsStopLoss bool, stopLossPrice float64, needsTakeProfit bool, takeProfitPrice float64) error {
	var lastErr error
	for attempt := 1; attempt <= protectionSetupMaxAttempts; attempt++ {
		if err := at.placeAndVerifyProtection(symbol, positionSide, quantity, needsStopLoss, stopLossPrice, needsTakeProfit, takeProfitPrice); err != nil {
			lastErr = err
			logger.Warnf("  ⚠️ Protection setup attempt %d/%d failed for %s %s: %v", attempt, protectionSetupMaxAttempts, symbol, positionSide, err)
			continue
		}
		if attempt > 1 {
			logger.Infof("  ✅ Protection setup recovered on retry %d/%d for %s %s", attempt, protectionSetupMaxAttempts, symbol, positionSide)
		}
		return nil
	}
	return fmt.Errorf("protection setup failed after %d attempts: %w", protectionSetupMaxAttempts, lastErr)
}

func (at *AutoTrader) placeAndVerifyProtectionPlan(symbol, positionSide string, quantity float64, plan *ProtectionPlan) error {
	if plan == nil {
		return nil
	}

	if len(plan.StopLossOrders) > 1 || len(plan.TakeProfitOrders) > 1 {
		return at.placeAndVerifyLadderProtection(symbol, positionSide, quantity, plan)
	}

	return at.placeAndVerifyProtection(symbol, positionSide, quantity, plan.NeedsStopLoss, plan.StopLossPrice, plan.NeedsTakeProfit, plan.TakeProfitPrice)
}

func (at *AutoTrader) placeAndVerifyLadderProtection(symbol, positionSide string, quantity float64, plan *ProtectionPlan) error {
	if plan == nil {
		return nil
	}

	for _, order := range plan.StopLossOrders {
		orderQty := quantity * order.CloseRatioPct / 100.0
		if orderQty <= 0 {
			continue
		}
		if err := at.trader.SetStopLoss(symbol, positionSide, orderQty, order.Price); err != nil {
			return fmt.Errorf("failed to set ladder stop loss %.6f (ratio %.2f%%): %w", order.Price, order.CloseRatioPct, err)
		}
	}
	for _, order := range plan.TakeProfitOrders {
		orderQty := quantity * order.CloseRatioPct / 100.0
		if orderQty <= 0 {
			continue
		}
		if err := at.trader.SetTakeProfit(symbol, positionSide, orderQty, order.Price); err != nil {
			return fmt.Errorf("failed to set ladder take profit %.6f (ratio %.2f%%): %w", order.Price, order.CloseRatioPct, err)
		}
	}

	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to verify ladder protection orders: %w", err)
	}

	if err := verifyProtectionOrders(openOrders, positionSide, plan.StopLossOrders, false); err != nil {
		return err
	}
	if err := verifyProtectionOrders(openOrders, positionSide, plan.TakeProfitOrders, true); err != nil {
		return err
	}

	logger.Infof("  ✅ Ladder protection orders verified: symbol=%s side=%s ladderSL=%d ladderTP=%d",
		symbol, positionSide, len(plan.StopLossOrders), len(plan.TakeProfitOrders))
	return nil
}

func verifyProtectionOrders(orders []tradertypes.OpenOrder, positionSide string, targets []ProtectionOrder, wantTakeProfit bool) error {
	for _, target := range targets {
		if !hasMatchingProtectionOrder(orders, positionSide, wantTakeProfit, target.Price) {
			kind := "stop loss"
			if wantTakeProfit {
				kind = "take profit"
			}
			return fmt.Errorf("%s ladder verification failed for %s at %.6f", kind, positionSide, target.Price)
		}
	}
	return nil
}

func (at *AutoTrader) placeAndVerifyProtection(symbol, positionSide string, quantity float64, needsStopLoss bool, stopLossPrice float64, needsTakeProfit bool, takeProfitPrice float64) error {
	if needsStopLoss {
		if err := at.trader.SetStopLoss(symbol, positionSide, quantity, stopLossPrice); err != nil {
			return fmt.Errorf("failed to set stop loss: %w", err)
		}
	}
	if needsTakeProfit {
		if err := at.trader.SetTakeProfit(symbol, positionSide, quantity, takeProfitPrice); err != nil {
			return fmt.Errorf("failed to set take profit: %w", err)
		}
	}

	if !needsStopLoss && !needsTakeProfit {
		return nil
	}

	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to verify protection orders: %w", err)
	}

	if needsStopLoss && !hasMatchingProtectionOrder(openOrders, positionSide, false, stopLossPrice) {
		return fmt.Errorf("stop loss verification failed for %s %s at %.6f", symbol, positionSide, stopLossPrice)
	}
	if needsTakeProfit && !hasMatchingProtectionOrder(openOrders, positionSide, true, takeProfitPrice) {
		return fmt.Errorf("take profit verification failed for %s %s at %.6f", symbol, positionSide, takeProfitPrice)
	}

	logger.Infof("  ✅ Protection orders verified: symbol=%s side=%s stop=%v tp=%v", symbol, positionSide, needsStopLoss, needsTakeProfit)
	return nil
}

func hasMatchingProtectionOrder(orders []tradertypes.OpenOrder, positionSide string, wantTakeProfit bool, targetPrice float64) bool {
	for _, order := range orders {
		if positionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) && order.PositionSide != "" {
			continue
		}

		if wantTakeProfit {
			if !looksLikeTakeProfit(order) {
				continue
			}
		} else {
			if !looksLikeStopLoss(order) {
				continue
			}
		}

		price := order.StopPrice
		if price <= 0 {
			price = order.Price
		}
		if approximatelyEqualPrice(price, targetPrice) {
			return true
		}
	}
	return false
}

func looksLikeStopLoss(order tradertypes.OpenOrder) bool {
	kind := strings.ToUpper(order.Type)
	return strings.Contains(kind, "STOP") && !strings.Contains(kind, "TAKE_PROFIT") && !strings.Contains(kind, "TP")
}

func looksLikeTakeProfit(order tradertypes.OpenOrder) bool {
	kind := strings.ToUpper(order.Type)
	return strings.Contains(kind, "TAKE_PROFIT") || strings.Contains(kind, "TP")
}

func approximatelyEqualPrice(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	base := math.Max(math.Abs(a), math.Abs(b))
	if base == 0 {
		return false
	}
	return math.Abs(a-b)/base <= protectionPriceTolerancePct
}
