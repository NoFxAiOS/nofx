package trader

import (
	"fmt"
	"math"
	"strings"
	"time"

	"nofx/kernel"
	"nofx/logger"
	"nofx/store"
	tradertypes "nofx/trader/types"
)

const (
	protectionPriceTolerancePct = 0.005 // 0.5% — widened to handle exchange price precision truncation
	protectionSetupMaxAttempts  = 2
	protectionVerifyMaxAttempts = 6 // retry GetOpenOrders verification up to 6 times for OKX TP visibility lag
)

var protectionVerifyDelay = 700 * time.Millisecond // delay between verification attempts

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

	configuredPlan, err := at.BuildConfiguredProtectionPlan(req.EntryPrice, req.Action)
	if err != nil {
		return err
	}
	plan := configuredPlan

	if req.Decision.ProtectionPlan != nil {
		decisionPlan, err := buildAIProtectionPlan(req.EntryPrice, req.Action, req.Decision.ProtectionPlan)
		if err != nil {
			return err
		}
		plan = mergeProtectionPlans(configuredPlan, decisionPlan)
	}

	var planErr error
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
			planErr = err
			logger.Warnf("  ⚠️ Primary protection plan failed for %s %s: %v", req.Symbol, req.PositionSide, err)
		}
	}

	if err := at.applyNativeProtectionTargetsAfterOpen(req, plan); err != nil {
		if planErr != nil {
			return fmt.Errorf("primary protection failed: %v; native targets failed: %w", planErr, err)
		}
		return err
	}

	if plan != nil && planErr == nil {
		return nil
	}

	needsStopLoss := req.Decision.StopLoss > 0
	needsTakeProfit := req.Decision.TakeProfit > 0
	if !needsStopLoss && !needsTakeProfit {
		if planErr != nil {
			return planErr
		}
		return nil
	}

	logger.Infof("  🛡 Applying AI decision protection fallback: stop=%v@%.6f tp=%v@%.6f",
		needsStopLoss, req.Decision.StopLoss, needsTakeProfit, req.Decision.TakeProfit)
	if err := at.placeAndVerifyProtectionWithRetry(req.Symbol, req.PositionSide, req.Quantity, needsStopLoss, req.Decision.StopLoss, needsTakeProfit, req.Decision.TakeProfit); err != nil {
		if planErr != nil {
			return fmt.Errorf("primary protection failed: %v; fallback failed: %w", planErr, err)
		}
		return err
	}
	return nil
}

func (at *AutoTrader) applyNativeProtectionTargetsAfterOpen(req *protectionExecutionRequest, plan *ProtectionPlan) error {
	if req == nil || req.Decision == nil || at.config.StrategyConfig == nil {
		return nil
	}

	prot := at.config.StrategyConfig.Protection
	drawdownRules := prot.DrawdownTakeProfit.Rules
	if plan != nil && len(plan.DrawdownRules) > 0 {
		drawdownRules = plan.DrawdownRules
	} else if prot.DrawdownTakeProfit.Enabled && prot.DrawdownTakeProfit.Mode == store.ProtectionModeAI {
		return fmt.Errorf("drawdown protection is in AI mode but decision did not provide drawdown_rules")
	} else if !prot.DrawdownTakeProfit.Enabled || prot.DrawdownTakeProfit.Mode == store.ProtectionModeDisabled {
		drawdownRules = nil
	}

	// 1. Native drawdown/trailing should be armed as early as safely possible.
	for _, rule := range drawdownRules {
		if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
			continue
		}
		if rule.CloseRatioPct >= 99.999 {
			_ = at.applyNativeTrailingDrawdown(req.Symbol, strings.TrimPrefix(strings.ToLower(req.PositionSide), ""), req.EntryPrice, rule)
			continue
		}

		// For partial drawdown, prefer exchange-native trailing when supported.
		// Only fall back to managed partial TP when native partial trailing is unavailable or fails.
		if at.applyNativeTrailingDrawdown(req.Symbol, strings.TrimPrefix(strings.ToLower(req.PositionSide), ""), req.EntryPrice, rule) {
			continue
		}

		candidate := buildManagedPartialDrawdownPlanCandidate(req.EntryPrice, req.Action, rule)
		if candidate == nil {
			continue
		}
		if at.canApplyManagedPartialDrawdownPlan(candidate) {
			logger.Infof("  🛡 Applying managed partial drawdown: symbol=%s side=%s close=%.1f%%",
				req.Symbol, req.PositionSide, rule.CloseRatioPct)
			if err := at.placeAndVerifyProtectionPlanWithRetry(req.Symbol, req.PositionSide, req.Quantity, candidate); err != nil {
				// Verification failed but OKX may have accepted the orders. Cancel them to avoid orphans.
				logger.Warnf("  ⚠️ Managed partial drawdown failed for %s %s: %v — cancelling orphaned orders", req.Symbol, req.PositionSide, err)
				at.cancelOrphanedDrawdownOrders(req.Symbol, candidate)
			} else {
				at.setProtectionState(req.Symbol, strings.ToLower(req.PositionSide), "managed_partial_drawdown_armed")
			}
		}
	}

	// 2. Break-even should prefer exchange-native stop as soon as trigger condition is met.
	if be := at.getActiveBreakEvenConfig(); be != nil && be.TriggerValue <= 0 {
		if err := at.applyBreakEvenStop(req.Symbol, strings.ToLower(req.PositionSide), req.Quantity, req.EntryPrice, be.TriggerValue, *be); err == nil {
			at.setBreakEvenState(req.Symbol, strings.ToLower(req.PositionSide), "armed")
		}
	}

	return nil
}

func (at *AutoTrader) canApplyManagedPartialDrawdownPlan(plan *ProtectionPlan) bool {
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

func (at *AutoTrader) validateProtectionPlanExecution(symbol, positionSide string, quantity float64, plan *ProtectionPlan) (*ProtectionPlan, error) {
	if plan == nil {
		return nil, nil
	}

	adjusted := *plan
	if len(plan.StopLossOrders) > 0 {
		adjusted.StopLossOrders = append([]ProtectionOrder(nil), plan.StopLossOrders...)
	}
	if len(plan.TakeProfitOrders) > 0 {
		adjusted.TakeProfitOrders = append([]ProtectionOrder(nil), plan.TakeProfitOrders...)
	}

	if okxTrader, ok := at.trader.(interface {
		ValidateProtectionQuantity(symbol string, quantity float64) error
	}); ok {
		filterExecutable := func(orders []ProtectionOrder) []ProtectionOrder {
			if len(orders) == 0 {
				return orders
			}
			filtered := make([]ProtectionOrder, 0, len(orders))
			for _, order := range orders {
				orderQty := quantity * order.CloseRatioPct / 100.0
				if orderQty <= 0 {
					continue
				}
				if err := okxTrader.ValidateProtectionQuantity(symbol, orderQty); err != nil {
					logger.Warnf("  ⚠️ Protection tier dropped as non-executable: symbol=%s side=%s price=%.6f qty=%.6f err=%v", symbol, positionSide, order.Price, orderQty, err)
					continue
				}
				filtered = append(filtered, order)
			}
			return filtered
		}

		adjusted.StopLossOrders = filterExecutable(adjusted.StopLossOrders)
		adjusted.TakeProfitOrders = filterExecutable(adjusted.TakeProfitOrders)

		if len(plan.StopLossOrders) > 0 && len(adjusted.StopLossOrders) == 0 && plan.NeedsStopLoss && plan.StopLossPrice > 0 {
			logger.Warnf("  ⚠️ Ladder stop-loss tiers all below exchange minimum; degrading to full stop for %s %s", symbol, positionSide)
		}
		if len(plan.TakeProfitOrders) > 0 && len(adjusted.TakeProfitOrders) == 0 && plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 {
			logger.Warnf("  ⚠️ Ladder take-profit tiers all below exchange minimum; degrading to full TP for %s %s", symbol, positionSide)
		}
	}

	adjusted.NeedsStopLoss = plan.NeedsStopLoss && (plan.StopLossPrice > 0 || len(adjusted.StopLossOrders) > 0)
	adjusted.NeedsTakeProfit = plan.NeedsTakeProfit && (plan.TakeProfitPrice > 0 || len(adjusted.TakeProfitOrders) > 0)

	if len(adjusted.StopLossOrders) > 0 {
		adjusted.StopLossPrice = 0
	}
	if len(adjusted.TakeProfitOrders) > 0 {
		adjusted.TakeProfitPrice = 0
	}

	if !adjusted.NeedsStopLoss && !adjusted.NeedsTakeProfit && adjusted.FallbackMaxLossPrice <= 0 {
		return nil, nil
	}
	return &adjusted, nil
}

func (at *AutoTrader) placeAndVerifyProtectionPlan(symbol, positionSide string, quantity float64, plan *ProtectionPlan) error {
	if plan == nil {
		return nil
	}

	validatedPlan, err := at.validateProtectionPlanExecution(symbol, positionSide, quantity, plan)
	if err != nil {
		return err
	}
	plan = validatedPlan

	if plan == nil {
		return nil
	}

	hasLadderSL := len(plan.StopLossOrders) > 0
	hasLadderTP := len(plan.TakeProfitOrders) > 0

	logger.Infof("  🧾 Protection plan materialized: symbol=%s side=%s mode=%s ladderSL=%d ladderTP=%d fullSL=%v@%.6f fullTP=%v@%.6f fallback=%.6f",
		symbol, positionSide, plan.Mode, len(plan.StopLossOrders), len(plan.TakeProfitOrders), plan.NeedsStopLoss, plan.StopLossPrice, plan.NeedsTakeProfit, plan.TakeProfitPrice, plan.FallbackMaxLossPrice)

	// Apply ladder legs first when present.
	if hasLadderSL || hasLadderTP {
		if err := at.placeAndVerifyLadderProtection(symbol, positionSide, quantity, plan); err != nil {
			return err
		}
	}

	// Full-position TP/SL should still be applied for directions NOT already covered by ladder orders.
	fullStop := plan.NeedsStopLoss && plan.StopLossPrice > 0 && !hasLadderSL
	fullTP := plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 && !hasLadderTP
	if fullStop || fullTP {
		if err := at.placeAndVerifyProtection(symbol, positionSide, quantity, fullStop, plan.StopLossPrice, fullTP, plan.TakeProfitPrice); err != nil {
			return err
		}
	}

	if plan.FallbackMaxLossPrice > 0 {
		fallbackNeeded := !fullStop || plan.StopLossPrice == 0 || !approximatelyEqualPrice(plan.StopLossPrice, plan.FallbackMaxLossPrice)
		logger.Infof("  🧾 Fallback evaluation: symbol=%s side=%s fallback=%.6f fullStop=%v stopPrice=%.6f needed=%v",
			symbol, positionSide, plan.FallbackMaxLossPrice, fullStop, plan.StopLossPrice, fallbackNeeded)
		if fallbackNeeded {
			if err := at.placeFallbackMaxLossProtection(symbol, positionSide, quantity, plan.FallbackMaxLossPrice); err != nil {
				return err
			}
		}
	}

	if plan.FallbackMaxLossPrice > 0 {
		openOrders, err := at.trader.GetOpenOrders(symbol)
		if err != nil {
			return fmt.Errorf("failed to verify fallback max-loss stop loss: %w", err)
		}
		if !hasMatchingProtectionOrder(openOrders, positionSide, false, plan.FallbackMaxLossPrice) {
			return fmt.Errorf("fallback max-loss stop verification failed for %s %s at %.6f", symbol, positionSide, plan.FallbackMaxLossPrice)
		}
		logger.Infof("  ✅ Fallback max-loss stop verified: symbol=%s side=%s stop=%.6f", symbol, positionSide, plan.FallbackMaxLossPrice)
	}

	return nil
}

func (at *AutoTrader) placeFallbackMaxLossProtection(symbol, positionSide string, quantity float64, stopLossPrice float64) error {
	if stopLossPrice <= 0 {
		return nil
	}
	logger.Infof("  🛡 Placing fallback max-loss stop: symbol=%s side=%s qty=%.6f stop=%.6f", symbol, positionSide, quantity, stopLossPrice)
	if setter, ok := at.trader.(interface {
		SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error
		SetStopLossTagged(symbol string, positionSide string, quantity, stopPrice float64, reasonTag string) error
	}); ok {
		if err := setter.SetStopLossTagged(symbol, positionSide, quantity, stopLossPrice, "fallback_maxloss_sl"); err != nil {
			return fmt.Errorf("failed to set fallback max-loss stop loss: %w", err)
		}
		return nil
	}
	if err := at.trader.SetStopLoss(symbol, positionSide, quantity, stopLossPrice); err != nil {
		return fmt.Errorf("failed to set fallback max-loss stop loss: %w", err)
	}
	return nil
}

func (at *AutoTrader) placeAndVerifyLadderProtection(symbol, positionSide string, quantity float64, plan *ProtectionPlan) error {
	if plan == nil {
		return nil
	}

	existingOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to inspect existing ladder protection orders: %w", err)
	}

	for _, order := range plan.StopLossOrders {
		orderQty := quantity * order.CloseRatioPct / 100.0
		if orderQty <= 0 {
			continue
		}
		if hasExistingEquivalentProtection(existingOrders, positionSide, false, order.Price, orderQty) {
			continue
		}
		if setter, ok := at.trader.(interface {
			SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error
			SetStopLossTagged(symbol string, positionSide string, quantity, stopPrice float64, reasonTag string) error
		}); ok {
			if err := setter.SetStopLossTagged(symbol, positionSide, orderQty, order.Price, "ladder_sl"); err != nil {
				return fmt.Errorf("failed to set ladder stop loss %.6f (ratio %.2f%%): %w", order.Price, order.CloseRatioPct, err)
			}
		} else if err := at.trader.SetStopLoss(symbol, positionSide, orderQty, order.Price); err != nil {
			return fmt.Errorf("failed to set ladder stop loss %.6f (ratio %.2f%%): %w", order.Price, order.CloseRatioPct, err)
		}
	}
	for _, order := range plan.TakeProfitOrders {
		orderQty := quantity * order.CloseRatioPct / 100.0
		if orderQty <= 0 {
			continue
		}
		if hasExistingEquivalentProtection(existingOrders, positionSide, true, order.Price, orderQty) {
			continue
		}
		if setter, ok := at.trader.(interface {
			SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error
			SetTakeProfitTagged(symbol string, positionSide string, quantity, takeProfitPrice float64, reasonTag string) error
		}); ok {
			if err := setter.SetTakeProfitTagged(symbol, positionSide, orderQty, order.Price, "ladder_tp"); err != nil {
				return fmt.Errorf("failed to set ladder take profit %.6f (ratio %.2f%%): %w", order.Price, order.CloseRatioPct, err)
			}
		} else if err := at.trader.SetTakeProfit(symbol, positionSide, orderQty, order.Price); err != nil {
			return fmt.Errorf("failed to set ladder take profit %.6f (ratio %.2f%%): %w", order.Price, order.CloseRatioPct, err)
		}
	}

	// Retry verification with delay to handle exchange propagation latency.
	for attempt := 1; attempt <= protectionVerifyMaxAttempts; attempt++ {
		at.sleepForVerification(protectionVerifyDelay)

		openOrders, err := at.trader.GetOpenOrders(symbol)
		if err != nil {
			return fmt.Errorf("failed to verify ladder protection orders: %w", err)
		}

		slErr := verifyProtectionOrders(openOrders, positionSide, plan.StopLossOrders, false)
		tpErr := verifyProtectionOrders(openOrders, positionSide, plan.TakeProfitOrders, true)
		if slErr == nil && tpErr == nil {
			logger.Infof("  ✅ Ladder protection orders verified: symbol=%s side=%s ladderSL=%d ladderTP=%d (attempt %d/%d)",
				symbol, positionSide, len(plan.StopLossOrders), len(plan.TakeProfitOrders), attempt, protectionVerifyMaxAttempts)
			return nil
		}

		if attempt < protectionVerifyMaxAttempts {
			logger.Infof("  ⏳ Ladder verification pending (attempt %d/%d), retrying...", attempt, protectionVerifyMaxAttempts)
		} else {
			if slErr != nil {
				return slErr
			}
			return tpErr
		}
	}

	return nil
}

func verifyProtectionOrders(orders []tradertypes.OpenOrder, positionSide string, targets []ProtectionOrder, wantTakeProfit bool) error {
	for _, target := range targets {
		if !hasMatchingProtectionOrder(orders, positionSide, wantTakeProfit, target.Price) {
			kind := "stop loss"
			if wantTakeProfit {
				kind = "take profit"
			}
			// Debug: log what orders we actually see so we can diagnose verification mismatches
			var candidates []string
			for _, o := range orders {
				if positionSide != "" && !strings.EqualFold(o.PositionSide, positionSide) && o.PositionSide != "" {
					continue
				}
				price := o.StopPrice
				if price <= 0 {
					price = o.Price
				}
				matches := ""
				if wantTakeProfit && looksLikeTakeProfit(o) {
					matches = "match-type"
				} else if !wantTakeProfit && looksLikeStopLoss(o) {
					matches = "match-type"
				}
				candidates = append(candidates, fmt.Sprintf("%s|side=%s|price=%.6f|qty=%.4f|%s", o.Type, o.PositionSide, price, o.Quantity, matches))
			}
			logger.Infof("  🔍 %s verify failed: target=%.6f side=%s | candidates=%v", kind, target.Price, positionSide, candidates)
			return fmt.Errorf("%s ladder verification failed for %s at %.6f", kind, positionSide, target.Price)
		}
	}
	return nil
}

func hasExistingEquivalentProtection(orders []tradertypes.OpenOrder, positionSide string, wantTakeProfit bool, targetPrice, targetQty float64) bool {
	for _, order := range orders {
		if hasEquivalentProtectionOrder(order, positionSide, wantTakeProfit, targetPrice, targetQty) {
			return true
		}
	}
	return false
}

func (at *AutoTrader) placeAndVerifyProtection(symbol, positionSide string, quantity float64, needsStopLoss bool, stopLossPrice float64, needsTakeProfit bool, takeProfitPrice float64) error {
	if needsStopLoss {
		if setter, ok := at.trader.(interface {
			SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error
			SetStopLossTagged(symbol string, positionSide string, quantity, stopPrice float64, reasonTag string) error
		}); ok {
			if err := setter.SetStopLossTagged(symbol, positionSide, quantity, stopLossPrice, "full_sl"); err != nil {
				return fmt.Errorf("failed to set stop loss: %w", err)
			}
		} else if err := at.trader.SetStopLoss(symbol, positionSide, quantity, stopLossPrice); err != nil {
			return fmt.Errorf("failed to set stop loss: %w", err)
		}
	}
	if needsTakeProfit {
		if setter, ok := at.trader.(interface {
			SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error
			SetTakeProfitTagged(symbol string, positionSide string, quantity, takeProfitPrice float64, reasonTag string) error
		}); ok {
			if err := setter.SetTakeProfitTagged(symbol, positionSide, quantity, takeProfitPrice, "full_tp"); err != nil {
				return fmt.Errorf("failed to set take profit: %w", err)
			}
		} else if err := at.trader.SetTakeProfit(symbol, positionSide, quantity, takeProfitPrice); err != nil {
			return fmt.Errorf("failed to set take profit: %w", err)
		}
	}

	if !needsStopLoss && !needsTakeProfit {
		return nil
	}

	// Retry verification with delay to handle exchange propagation latency.
	for attempt := 1; attempt <= protectionVerifyMaxAttempts; attempt++ {
		at.sleepForVerification(protectionVerifyDelay)

		openOrders, err := at.trader.GetOpenOrders(symbol)
		if err != nil {
			return fmt.Errorf("failed to verify protection orders: %w", err)
		}

		slOK := !needsStopLoss || hasMatchingProtectionOrder(openOrders, positionSide, false, stopLossPrice)
		tpOK := !needsTakeProfit || hasMatchingProtectionOrder(openOrders, positionSide, true, takeProfitPrice)
		if slOK && tpOK {
			logger.Infof("  ✅ Protection orders verified: symbol=%s side=%s stop=%v tp=%v (attempt %d/%d)",
				symbol, positionSide, needsStopLoss, needsTakeProfit, attempt, protectionVerifyMaxAttempts)
			return nil
		}

		if attempt < protectionVerifyMaxAttempts {
			logger.Infof("  ⏳ Protection verification pending (attempt %d/%d), retrying...", attempt, protectionVerifyMaxAttempts)
		}
	}

	// Final check failed — return specific error.
	if needsStopLoss {
		return fmt.Errorf("stop loss verification failed for %s %s at %.6f after %d attempts", symbol, positionSide, stopLossPrice, protectionVerifyMaxAttempts)
	}
	return fmt.Errorf("take profit verification failed for %s %s at %.6f after %d attempts", symbol, positionSide, takeProfitPrice, protectionVerifyMaxAttempts)
}

// sleepForVerification waits before re-checking exchange orders. Extracted for test override.
func (at *AutoTrader) sleepForVerification(d time.Duration) {
	time.Sleep(d)
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

func countMatchingProtectionOrders(orders []tradertypes.OpenOrder, positionSide string, wantTakeProfit bool, targetPrice float64) int {
	count := 0
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
			count++
		}
	}
	return count
}

func hasEquivalentProtectionOrder(order tradertypes.OpenOrder, positionSide string, wantTakeProfit bool, targetPrice, targetQty float64) bool {
	if positionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) && order.PositionSide != "" {
		return false
	}
	if wantTakeProfit {
		if !looksLikeTakeProfit(order) {
			return false
		}
	} else {
		if !looksLikeStopLoss(order) {
			return false
		}
	}
	price := order.StopPrice
	if price <= 0 {
		price = order.Price
	}
	if !approximatelyEqualPrice(price, targetPrice) {
		return false
	}
	if targetQty > 0 && order.Quantity > 0 && math.Abs(order.Quantity-targetQty)/math.Max(order.Quantity, targetQty) > 0.05 {
		return false
	}
	return true
}

func looksLikeStopLoss(order tradertypes.OpenOrder) bool {
	kind := strings.ToUpper(order.Type)
	return (strings.Contains(kind, "STOP") || strings.Contains(kind, "SL")) && !strings.Contains(kind, "TAKE_PROFIT") && !strings.Contains(kind, "TP")
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

// cancelOrphanedDrawdownOrders cancels only the TP orders created by the failed
// drawdown plan. It must not wipe ladder/full TP orders for the same symbol.
func (at *AutoTrader) cancelOrphanedDrawdownOrders(symbol string, plan *ProtectionPlan) {
	if plan == nil || len(plan.TakeProfitOrders) == 0 {
		return
	}

	targetPrices := make([]float64, 0, len(plan.TakeProfitOrders))
	for _, order := range plan.TakeProfitOrders {
		if order.Price > 0 {
			targetPrices = append(targetPrices, order.Price)
		}
	}

	if targetedCanceller, ok := at.trader.(interface {
		CancelTakeProfitOrdersByPrices(symbol string, prices []float64) error
	}); ok && len(targetPrices) > 0 {
		if err := targetedCanceller.CancelTakeProfitOrdersByPrices(symbol, targetPrices); err != nil {
			logger.Warnf("  ⚠️ Failed to cancel targeted orphaned drawdown TP orders for %s: %v", symbol, err)
		} else {
			logger.Infof("  🧹 Cancelled targeted orphaned drawdown TP orders for %s at prices=%v", symbol, targetPrices)
		}
		return
	}

	if canceller, ok := at.trader.(interface {
		CancelTakeProfitOrders(symbol string) error
	}); ok {
		if err := canceller.CancelTakeProfitOrders(symbol); err != nil {
			logger.Warnf("  ⚠️ Failed to cancel orphaned drawdown TP orders for %s: %v", symbol, err)
		} else {
			logger.Infof("  🧹 Cancelled orphaned drawdown TP orders for %s", symbol)
		}
	}
}
