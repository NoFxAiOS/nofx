package trader

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/store"
)

const (
	reconcileCooldownDuration = 60 * time.Second // cooldown after successful re-apply
)

var (
	reconcileCooldowns     = make(map[string]time.Time)
	reconcileCooldownMutex sync.RWMutex
)

func (at *AutoTrader) startProtectionReconciler() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		logger.Infof("🛡 Started protection reconciler (check every %s)", 20*time.Second)

		for {
			select {
			case <-ticker.C:
				at.reconcilePositionProtections()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped protection reconciler")
				return
			}
		}
	}()
}

type protectionReconcileResult struct {
	ExchangeVerified bool
	Summary          string
}

func (at *AutoTrader) reconcilePositionProtections() {
	if at == nil || at.trader == nil || at.config.StrategyConfig == nil {
		return
	}

	c := at.GetProtectionCapabilities()
	if !c.NativeStopLoss && !c.NativeTakeProfit {
		return
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("❌ Protection reconciler: failed to get positions: %v", err)
		return
	}

	active := make(map[string]struct{})
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		entryPrice, _ := pos["entryPrice"].(float64)
		quantity, _ := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		if symbol == "" || side == "" || entryPrice <= 0 || quantity <= 0 {
			continue
		}
		key := positionKey(symbol, side)
		active[key] = struct{}{}

		// Skip reconciliation for positions still in cooldown after a recent re-apply.
		if at.isReconcileCooldownActive(key) {
			logger.Infof("⏳ Protection reconciler: %s %s in cooldown, skipping", symbol, side)
			continue
		}

		result, err := at.reconcileProtectionForPosition(symbol, side, quantity, entryPrice)
		if err != nil {
			logger.Infof("❌ Protection reconciler: %s %s reconcile failed: %v", symbol, side, err)
			at.setProtectionState(symbol, side, "reconcile_failed: "+err.Error())
			continue
		}

		if result.ExchangeVerified {
			currentState := at.getProtectionState(symbol, side)
			if currentState == "native_trailing_armed" || currentState == "native_partial_trailing_armed" || currentState == "native_trailing_arming" || currentState == "native_partial_trailing_arming" || currentState == "managed_partial_drawdown_armed" {
				logger.Infof("✅ Protection reconciler: %s %s exchange protection verified (preserving dynamic state=%s)", symbol, side, currentState)
			} else {
				at.setProtectionState(symbol, side, "exchange_protection_verified")
				logger.Infof("✅ Protection reconciler: %s %s exchange protection verified", symbol, side)
			}
		} else {
			if result.Summary == "" {
				result.Summary = "no exchange protection ownership verified"
			}
			logger.Warnf("⚠️ Protection reconciler: %s %s exchange protection not verified: %s", symbol, side, result.Summary)
		}
	}

	at.cleanupInactiveProtectionState(active)
	if at.store != nil {
		if err := at.store.DeleteDynamicProtectionRecordsForInactive(active); err != nil {
			logger.Warnf("⚠️ Dynamic protection state: failed to cleanup inactive records: %v", err)
		}
	}
}

func (at *AutoTrader) describeProtectionSnapshot(symbol, side string, openOrders []OpenOrder, plan *ProtectionPlan, breakEvenArmed bool, nativeTrailingArmed bool) string {
	parts := make([]string, 0, len(openOrders)+8)
	for _, order := range openOrders {
		if side != "" && order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
			continue
		}
		kind := order.Type
		if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			kind = fmt.Sprintf("%s@%.6f/cb=%.6f", order.Type, order.StopPrice, order.CallbackRate)
		} else {
			kind = fmt.Sprintf("%s@%.6f", order.Type, order.StopPrice)
		}
		parts = append(parts, kind)
	}
	if plan != nil {
		parts = append(parts, fmt.Sprintf("planStops=%d", len(plan.StopLossOrders)))
		parts = append(parts, fmt.Sprintf("planTPs=%d", len(plan.TakeProfitOrders)))
		if plan.NeedsStopLoss && len(plan.StopLossOrders) == 0 {
			parts = append(parts, fmt.Sprintf("fullSL=%.6f", plan.StopLossPrice))
		}
		if plan.NeedsTakeProfit && len(plan.TakeProfitOrders) == 0 {
			parts = append(parts, fmt.Sprintf("fullTP=%.6f", plan.TakeProfitPrice))
		}
		if plan.FallbackMaxLossPrice > 0 {
			parts = append(parts, fmt.Sprintf("fallback=%.6f", plan.FallbackMaxLossPrice))
		}
	}
	parts = append(parts, fmt.Sprintf("beArmed=%t", breakEvenArmed))
	parts = append(parts, fmt.Sprintf("trailingArmed=%t", nativeTrailingArmed))
	return strings.Join(parts, " | ")
}

func (at *AutoTrader) reconcileProtectionForPosition(symbol, side string, quantity, entryPrice float64) (protectionReconcileResult, error) {
	result := protectionReconcileResult{}
	positionSide := strings.ToUpper(side)
	currentProtectionState := at.getProtectionState(symbol, side)
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return result, fmt.Errorf("get open orders: %w", err)
	}
	at.reconcileLocalOpenOrderStatuses(symbol, openOrders)

	if currentProtectionState == "native_trailing_armed" || currentProtectionState == "native_partial_trailing_armed" || currentProtectionState == "native_trailing_arming" || currentProtectionState == "native_partial_trailing_arming" {
		if len(at.getArmedDrawdownRecordsForPosition(symbol, side, entryPrice, quantity)) == 0 {
			logger.Infof("🟣 Protection reconciler: %s %s native trailing state belongs to an old position fingerprint, re-arming current position", symbol, positionSide)
			currentProtectionState = ""
			at.clearProtectionState(symbol, side)
		}
	}

	// If native trailing drawdown is already armed/arming, generic take-profit plans should not be
	// re-applied on top of it. But stop-loss protection must still be preserved and repaired.
	nativeTrailingArmed := currentProtectionState == "native_trailing_armed" || currentProtectionState == "native_partial_trailing_armed" || currentProtectionState == "native_trailing_arming" || currentProtectionState == "native_partial_trailing_arming"

	plan, err := at.BuildConfiguredProtectionPlan(entryPrice, actionFromPositionSide(side))
	if err != nil {
		return result, fmt.Errorf("build configured plan: %w", err)
	}
	if !at.verifyLivePositionForProtection(symbol, side, "protection reconcile") {
		result.Summary = "inactive position; protection state cleaned"
		return result, nil
	}
	protectionConfigured := at.hasConfiguredProtectionOwner()

	// Drawdown/native trailing owns the profit-taking side. If drawdown profit-control is enabled,
	// proactively remove old generic TP orders for the active position while keeping SL orders intact.
	drawdownEnabled := at.config.StrategyConfig != nil && at.config.StrategyConfig.Protection.DrawdownTakeProfit.Enabled && len(at.config.StrategyConfig.Protection.DrawdownTakeProfit.Rules) > 0
	if nativeTrailingArmed && plan != nil {
		plan.NeedsTakeProfit = false
		plan.TakeProfitPrice = 0
		plan.TakeProfitOrders = nil
	}
	// Preserve the configured/open-time ladder shape during held-position reconciliation.
	// OKX can keep multiple conditional stop legs; only degrade later if exchange validation
	// proves a tier is non-executable, not preemptively on every reconcile pass.
	if drawdownEnabled && nativeTrailingArmed {
		hasGenericTP := false
		for _, order := range openOrders {
			if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
				continue
			}
			if looksLikeTakeProfit(order) && !strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
				hasGenericTP = true
				break
			}
		}
		if hasGenericTP && !hasVisiblePlanProfitOwner(openOrders, positionSide, plan) {
			if canceller, ok := at.trader.(interface{ CancelTakeProfitOrders(symbol string) error }); ok {
				logger.Infof("🧹 Drawdown owner: removing legacy generic take-profit orders for %s %s while preserving stop-loss legs", symbol, positionSide)
				if err := canceller.CancelTakeProfitOrders(symbol); err != nil {
					logger.Warnf("⚠️ Failed to cancel legacy generic take-profit orders for %s: %v", symbol, err)
				}
				openOrders, _ = at.trader.GetOpenOrders(symbol)
			}
		} else if hasGenericTP {
			logger.Infof("🛡 Drawdown owner: preserving existing take-profit orders for %s %s until dynamic protection is visibly armed", symbol, positionSide)
		}
	}

	if plan == nil {
		result = reconcileResultForUnmaterializedPlan(openOrders, positionSide, protectionConfigured)
	}

	if plan != nil {
		breakEvenArmed := at.getBreakEvenState(symbol, side) == "armed"
		if at.isBreakEvenSuppressedByRunner(symbol, side) {
			breakEvenArmed = false
		}
		missingSL, missingTP := detectMissingProtection(openOrders, positionSide, plan, breakEvenArmed)
		planOrderCount := protectionOrderCountForPlan(plan)
		unexpectedStops, unexpectedTPs := detectUnexpectedProtectionOrders(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed)
		unexpectedSummary := classifyUnexpectedProtectionOrders(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed, true)
		ownership := evaluateProtectionOwnership(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed)
		logger.Infof("🧭 Protection ownership: %s %s | state=%s verified=%t stopOwner=%s profitOwner=%s missingSL=%t missingTP=%t unexpectedSL=%d unexpectedTP=%d staleBot=%d manualForeign=%d dynamicOwner=%d reasons=%s",
			symbol, positionSide, ownership.State, ownership.Verified, ownership.StopOwner, ownership.ProfitOwner, ownership.MissingStop, ownership.MissingProfit, ownership.UnexpectedStops, ownership.UnexpectedProfits, unexpectedSummary.StaleBotDuplicate, unexpectedSummary.ManualOrForeign, unexpectedSummary.ExpectedDynamicOwner, strings.Join(ownership.Reasons, "; "))
		if ownership.State == "unprotected" && ownership.Verified {
			return result, fmt.Errorf("invalid protection ownership invariant: unprotected but verified")
		}

		// Detect duplicate/stale orders by explicit order-role mismatch, not only coarse order counts.
		// This keeps valid break-even / trailing orders while removing old ladder/fallback debris.
		if unexpectedStops > 0 && unexpectedTPs == 0 && !missingSL && ownership.StopOwner != "" {
			logger.Infof("🛡 Protection reconciler: %s %s preserving extra protective stop orders (unexpectedSL=%d) because stop coverage is already satisfied", symbol, positionSide, unexpectedStops)
			unexpectedStops = 0
			ownership.UnexpectedStops = 0
			ownership.Reasons = removeUnexpectedProtectionReason(ownership.Reasons)
			if ownership.StopOwner != "" && (!planRequiresProfitOwner(plan) || ownership.ProfitOwner != "") && ownership.UnexpectedProfits == 0 {
				ownership.State = "protected"
				ownership.Verified = true
			}
		}
		if unexpectedStops > 0 || unexpectedTPs > 0 {
			logger.Warnf("🧹 Protection reconciler: %s %s found unexpected exchange protection orders (unexpectedSL=%d unexpectedTP=%d, planned=%d), staging replacement before cleanup",
				symbol, positionSide, unexpectedStops, unexpectedTPs, planOrderCount)
			unexpectedIDs := collectUnexpectedProtectionOrderIDs(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed)
			if !at.verifyLivePositionForProtection(symbol, side, "unexpected protection replacement") {
				result.Summary = "inactive position before unexpected protection replacement; protection state cleaned"
				return result, nil
			}
			if err := at.placeAndVerifyProtectionPlanWithRetry(symbol, positionSide, quantity, plan); err != nil {
				at.setReconcileCooldown(positionKey(symbol, side))
				return result, fmt.Errorf("stage replacement before cleanup: %w", err)
			}
			at.cancelUnexpectedProtectionOrdersByID(symbol, unexpectedIDs)
			remainingOrders, cleanErr := at.trader.GetOpenOrders(symbol)
			if cleanErr != nil {
				at.setReconcileCooldown(positionKey(symbol, side))
				return result, fmt.Errorf("verify unexpected cleanup open orders: %w", cleanErr)
			}
			remainingUnexpectedStops, remainingUnexpectedTPs := detectUnexpectedProtectionOrders(remainingOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed)
			if remainingUnexpectedStops > 0 || remainingUnexpectedTPs > 0 {
				at.setReconcileCooldown(positionKey(symbol, side))
				return result, fmt.Errorf("unexpected cleanup incomplete after replacement (unexpectedSL=%d unexpectedTP=%d)", remainingUnexpectedStops, remainingUnexpectedTPs)
			}
			at.setReconcileCooldown(positionKey(symbol, side))
			result.ExchangeVerified = true
			result.Summary = "staged replacement before cleaning unexpected protection"
			return result, nil
		}

		if missingSL || missingTP {
			logger.Infof("🛠 Protection reconciler: %s %s missing exchange orders (SL=%v TP=%v), re-applying plan", symbol, positionSide, missingSL, missingTP)
			if !at.verifyLivePositionForProtection(symbol, side, "missing protection re-apply") {
				result.Summary = "inactive position before missing protection re-apply; protection state cleaned"
				return result, nil
			}
			if missingSL && plan.NeedsStopLoss && plan.StopLossPrice > 0 && plan.FallbackMaxLossPrice > 0 {
				if markPrice, ok := at.getPositionMarkPrice(symbol, side); ok && !isExecutableHeldStopPrice(side, plan.StopLossPrice, markPrice) {
					logger.Warnf("🛟 Protection reconciler: %s %s primary stop %.6f is non-executable against mark %.6f; keeping/restoring fallback %.6f",
						symbol, positionSide, plan.StopLossPrice, markPrice, plan.FallbackMaxLossPrice)
					if hasMatchingProtectionOrder(openOrders, positionSide, false, plan.FallbackMaxLossPrice) {
						result.ExchangeVerified = true
						result.Summary = "fallback retained: primary stop non-executable"
						return result, nil
					}
					if fallbackErr := at.placeAndVerifyFallbackMaxLoss(symbol, positionSide, quantity, plan.FallbackMaxLossPrice); fallbackErr == nil {
						at.setReconcileCooldown(positionKey(symbol, side))
						result.ExchangeVerified = true
						result.Summary = "fallback restored: primary stop non-executable"
						return result, nil
					}
				}
			}
			if missingSL && hasAnyProtectionOrder(openOrders, positionSide, false) {
				logger.Infof("🛡 Protection reconciler: %s %s preserving existing stop owner while staging missing protection replacement", symbol, positionSide)
			}
			if !at.verifyLivePositionForProtection(symbol, side, "missing protection plan placement") {
				result.Summary = "inactive position before missing protection plan placement; protection state cleaned"
				return result, nil
			}
			if err := at.placeAndVerifyProtectionPlanWithRetry(symbol, positionSide, quantity, plan); err != nil {
				if missingSL && plan.FallbackMaxLossPrice > 0 {
					logger.Warnf("🛟 Protection reconciler: %s %s primary stop re-apply failed, restoring fallback max-loss stop %.6f: %v", symbol, positionSide, plan.FallbackMaxLossPrice, err)
					if fallbackErr := at.placeAndVerifyFallbackMaxLoss(symbol, positionSide, quantity, plan.FallbackMaxLossPrice); fallbackErr == nil {
						at.setReconcileCooldown(positionKey(symbol, side))
						result.ExchangeVerified = true
						result.Summary = "fallback restored after primary stop re-apply failure"
						return result, nil
					} else {
						logger.Warnf("🛟 Protection reconciler: %s %s fallback restore also failed: %v", symbol, positionSide, fallbackErr)
					}
				}
				at.setReconcileCooldown(positionKey(symbol, side))
				return result, fmt.Errorf("re-apply manual protection plan: %w", err)
			}
			at.setReconcileCooldown(positionKey(symbol, side))
			result.ExchangeVerified = true
			result.Summary = "re-applied missing protection"
			return result, nil
		}
		result.ExchangeVerified = ownership.Verified
		result.Summary = strings.Join(ownership.Reasons, "; ")
		if ownership.ProfitOwner == "drawdown" && (at.getProtectionState(symbol, side) == "native_trailing_armed" || at.getProtectionState(symbol, side) == "native_partial_trailing_armed" || at.getProtectionState(symbol, side) == "native_trailing_arming" || at.getProtectionState(symbol, side) == "native_partial_trailing_arming") {
			result.Summary = "dynamic protection owner armed; exchange static ownership verified"
		}
	}

	markPrice, _ := at.getPositionMarkPrice(symbol, side)
	currentPnLPct := calculatePositionPnLPct(side, entryPrice, markPrice)

	beRules := at.getActiveBreakEvenRules()
	fingerprintChanged := at.refreshBreakEvenFingerprint(symbol, side, entryPrice, quantity)
	prevBreakEvenArmed := at.getBreakEvenState(symbol, side) == "armed"
	if at.isBreakEvenSuppressedByRunner(symbol, side) {
		beRules = nil
	}
	if len(beRules) > 0 && at.GetProtectionCapabilities().NativeStopLoss {
		if prevBreakEvenArmed && fingerprintChanged {
			logger.Infof("🛠 Protection reconciler: %s %s break-even fingerprint changed, re-arming native stop", symbol, positionSide)
			if err := at.applyBreakEvenStops(symbol, side, quantity, entryPrice, currentPnLPct, beRules); err != nil {
				return result, fmt.Errorf("re-arm break-even native stop: %w", err)
			}
		} else if at.getBreakEvenState(symbol, side) != "armed" {
			if err := at.applyBreakEvenStops(symbol, side, quantity, entryPrice, currentPnLPct, beRules); err != nil {
				at.setBreakEvenState(symbol, side, "pending")
				return result, fmt.Errorf("apply break-even native stop: %w", err)
			}
		}
	}

	rules := at.getActiveDrawdownRulesForPosition(symbol, side)
	if len(rules) > 0 {
		peakPnLPct := currentPnLPct
		at.peakPnLCacheMutex.RLock()
		if peak, ok := at.peakPnLCache[positionKey(symbol, side)]; ok && peak > peakPnLPct {
			peakPnLPct = peak
		}
		at.peakPnLCacheMutex.RUnlock()

		drawdownPct := 0.0
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}
		armedAny := false
		for _, armRule := range at.getDrawdownArmRulesForNativeExposure(currentPnLPct, entryPrice, quantity, symbol, side, rules) {
			if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, armRule) {
				armedAny = true
				logger.Infof("🛠 Protection reconciler: %s %s ensured native drawdown protection (arm close=%.1f%%)", symbol, positionSide, armRule.CloseRatioPct)
			}
		}
		for _, triggeredRule := range at.getTriggeredDrawdownRules(currentPnLPct, drawdownPct, rules) {
			if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, triggeredRule) {
				armedAny = true
				logger.Infof("🛠 Protection reconciler: %s %s ensured native drawdown protection (trigger close=%.1f%%)", symbol, positionSide, triggeredRule.CloseRatioPct)
			}
		}
		if !armedAny && isNativeTrailingProtectionState(at.getProtectionState(symbol, side)) {
			logger.Infof("🟣 Protection reconciler: %s %s already has all satisfied native trailing tiers armed (%s)", symbol, positionSide, at.getDrawdownExecutionMode(symbol, side))
		}
	}

	if !result.ExchangeVerified && hasMissingMandatoryLadderStops(openOrders, positionSide, plan) {
		result.Summary = "mandatory ladder SL missing; dynamic protection cannot satisfy static ladder ownership"
	} else if !result.ExchangeVerified && (at.getBreakEvenState(symbol, side) == "armed" || at.getProtectionState(symbol, side) == "native_trailing_armed" || at.getProtectionState(symbol, side) == "native_partial_trailing_armed" || at.getProtectionState(symbol, side) == "native_trailing_arming" || at.getProtectionState(symbol, side) == "native_partial_trailing_arming" || at.getProtectionState(symbol, side) == "managed_drawdown_armed") {
		result.Summary = "dynamic protection owner armed; exchange static ownership not fully verified"
	}
	return result, nil
}

func hasMissingMandatoryLadderStops(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan) bool {
	if plan == nil || len(plan.StopLossOrders) == 0 {
		return false
	}
	for _, target := range plan.StopLossOrders {
		if countMatchingProtectionOrders(openOrders, positionSide, false, target.Price) == 0 {
			return true
		}
	}
	return false
}

func isExecutableHeldStopPrice(side string, stopPrice, markPrice float64) bool {
	if stopPrice <= 0 || markPrice <= 0 {
		return true
	}
	switch strings.ToLower(side) {
	case "short":
		return stopPrice > markPrice
	case "long":
		return stopPrice < markPrice
	default:
		return true
	}
}

func isExecutableHeldTakeProfitPrice(side string, takeProfitPrice, markPrice float64) bool {
	if takeProfitPrice <= 0 || markPrice <= 0 {
		return true
	}
	switch strings.ToLower(side) {
	case "short":
		return takeProfitPrice < markPrice
	case "long":
		return takeProfitPrice > markPrice
	default:
		return true
	}
}

func (at *AutoTrader) getPositionMarkPrice(symbol, side string) (float64, bool) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return 0, false
	}
	for _, pos := range positions {
		ps, _ := pos["symbol"].(string)
		pd, _ := pos["side"].(string)
		if ps != symbol || !strings.EqualFold(pd, side) {
			continue
		}
		markPrice, _ := pos["markPrice"].(float64)
		if markPrice > 0 {
			return markPrice, true
		}
	}
	return 0, false
}

func (at *AutoTrader) verifyLivePositionForProtection(symbol, side, reason string) bool {
	if at == nil || at.trader == nil {
		return false
	}
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Warnf("⚠️ Protection liveness gate: failed to verify live position before %s (%s %s): %v", reason, symbol, side, err)
		return false
	}
	active := make(map[string]struct{})
	for _, pos := range positions {
		ps, _ := pos["symbol"].(string)
		pd, _ := pos["side"].(string)
		qty, _ := pos["positionAmt"].(float64)
		if qty < 0 {
			qty = -qty
		}
		if ps == "" || pd == "" || qty <= 0 {
			continue
		}
		active[positionKey(ps, pd)] = struct{}{}
		if ps == symbol && strings.EqualFold(pd, side) {
			return true
		}
	}
	logger.Warnf("🧯 Protection liveness gate: skipping %s for inactive %s %s; cleaning orphaned protection state", reason, symbol, side)
	at.cleanupInactiveProtectionState(active)
	return false
}

func calculatePositionPnLPct(side string, entryPrice, markPrice float64) float64 {
	if entryPrice <= 0 || markPrice <= 0 {
		return 0
	}
	if strings.EqualFold(side, "long") {
		return ((markPrice - entryPrice) / entryPrice) * 100
	}
	if strings.EqualFold(side, "short") {
		return ((entryPrice - markPrice) / entryPrice) * 100
	}
	return 0
}

func protectionOrderCountForPlan(plan *ProtectionPlan) int {
	if plan == nil {
		return 0
	}
	count := 0
	if len(plan.StopLossOrders) > 0 {
		count += len(plan.StopLossOrders)
	} else if plan.NeedsStopLoss && plan.StopLossPrice > 0 {
		count++
	}
	if len(plan.TakeProfitOrders) > 0 {
		count += len(plan.TakeProfitOrders)
	} else if plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 {
		count++
	}
	if plan.FallbackMaxLossPrice > 0 {
		count++
	}
	return count
}

func detectUnexpectedProtectionOrders(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan, breakEvenArmed bool, nativeTrailingArmed bool) (unexpectedStops int, unexpectedTPs int) {
	summary := classifyUnexpectedProtectionOrders(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed, true)
	for _, order := range openOrders {
		if positionSide != "" && order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		if !isUnexpectedProtectionOrder(order, summary) {
			continue
		}
		if looksLikeTakeProfit(order) {
			unexpectedTPs++
		} else if looksLikeStopLoss(order) || strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			unexpectedStops++
		}
	}
	return unexpectedStops, unexpectedTPs
}

func isUnexpectedProtectionOrder(order OpenOrder, summary unexpectedProtectionSummary) bool {
	if order.OrderID != "" {
		for _, id := range summary.StaleBotDuplicateIDs {
			if order.OrderID == id {
				return true
			}
		}
		for _, id := range summary.OrphanForInactiveIDs {
			if order.OrderID == id {
				return true
			}
		}
	}
	// Manual/foreign orders are intentionally not treated as bot-cleanable unexpected
	// orders. They still make ownership degraded through ManualOrForeign counts/logging,
	// but should not be canceled or trigger re-apply stacking.
	return false
}

func consumeAllowedProtectionPrice(prices *[]float64, actual float64) bool {
	if prices == nil || actual <= 0 {
		return false
	}
	for i, expected := range *prices {
		if approximatelyEqualPrice(actual, expected) {
			items := *prices
			items = append(items[:i], items[i+1:]...)
			*prices = items
			return true
		}
	}
	return false
}

func hasExplicitBreakEvenConfig(config *store.StrategyConfig) bool {
	if config == nil {
		return false
	}
	be := config.Protection.BreakEvenStop
	return be.Enabled && be.TriggerMode == store.BreakEvenTriggerProfitPct && be.TriggerValue > 0
}

func detectMissingProtection(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan, breakEvenArmed bool) (missingSL bool, missingTP bool) {
	if plan == nil {
		return false, false
	}

	breakEvenSatisfied := breakEvenArmed && hasAnyProtectionOrder(openOrders, positionSide, false)
	fallbackSatisfied := plan.FallbackMaxLossPrice > 0 && hasMatchingProtectionOrder(openOrders, positionSide, false, plan.FallbackMaxLossPrice)
	fullStopSatisfied := plan.NeedsStopLoss && plan.StopLossPrice > 0 && hasMatchingProtectionOrder(openOrders, positionSide, false, plan.StopLossPrice)
	fullTPSatisfied := plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 && hasMatchingProtectionOrder(openOrders, positionSide, true, plan.TakeProfitPrice)

	// Ladder SL is a hard static owner: every configured ladder stop tier must be
	// visible. Break-even/full/fallback stops are independent overlays and must not
	// satisfy or mask missing ladder SL tiers.
	if len(plan.StopLossOrders) > 0 {
		for _, target := range plan.StopLossOrders {
			if countMatchingProtectionOrders(openOrders, positionSide, false, target.Price) == 0 {
				missingSL = true
				break
			}
		}
	} else if plan.NeedsStopLoss {
		// For non-ladder static stops, a visible break-even/fallback max-loss stop still counts as protected
		// stop ownership even when tighter static stops are absent or intentionally handled by
		// a dynamic owner. This avoids re-materializing static stops next to an armed break-even stop.
		missingSL = !(breakEvenSatisfied || fullStopSatisfied || fallbackSatisfied || visibleFallbackOwnerSatisfied(openOrders, positionSide))
	}

	// Same rule for take-profit: when ladder TP orders exist, require each configured tier explicitly.
	// But if a full-position TP is already present, accept degraded-to-full ownership.
	if len(plan.TakeProfitOrders) > 0 {
		if !fullTPSatisfied {
			for _, target := range plan.TakeProfitOrders {
				if countMatchingProtectionOrders(openOrders, positionSide, true, target.Price) == 0 {
					missingTP = true
					break
				}
			}
		}
	} else if plan.NeedsTakeProfit {
		missingTP = !fullTPSatisfied
	}

	return missingSL, missingTP
}

func removeUnexpectedProtectionReason(reasons []string) []string {
	filtered := make([]string, 0, len(reasons))
	for _, r := range reasons {
		if strings.HasPrefix(r, "unexpected protection orders sl=") {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func visibleFallbackOwnerSatisfied(openOrders []OpenOrder, positionSide string) bool {
	for _, order := range openOrders {
		if positionSide != "" && order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		if !looksLikeStopLoss(order) {
			continue
		}
		if strings.Contains(strings.ToLower(order.ClientOrderID), "fallback") {
			return true
		}
	}
	return false
}

func hasAnyProtectionOrder(openOrders []OpenOrder, positionSide string, wantTakeProfit bool) bool {
	for _, order := range openOrders {
		if positionSide != "" && order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		if wantTakeProfit {
			if looksLikeTakeProfit(order) {
				return true
			}
		} else {
			if looksLikeStopLoss(order) {
				return true
			}
		}
	}
	return false
}

func positionKey(symbol, side string) string {
	return symbol + "_" + strings.ToLower(side)
}

func actionFromPositionSide(side string) string {
	switch strings.ToLower(side) {
	case "long":
		return "open_long"
	case "short":
		return "open_short"
	default:
		return ""
	}
}

func (at *AutoTrader) setDrawdownExecutionFingerprint(symbol, side, fingerprint string) {
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.drawdownState == nil {
		at.drawdownState = make(map[string]string)
	}
	at.drawdownState[positionKey(symbol, side)] = fingerprint
	at.persistDynamicProtectionRecord(symbol, side, "managed_drawdown", fingerprint, dynamicCloseRatioFromFingerprint(fingerprint), "executed", "")
}

func (at *AutoTrader) getDrawdownExecutionFingerprint(symbol, side string) string {
	at.protectionStateMutex.RLock()
	defer at.protectionStateMutex.RUnlock()
	if at.drawdownState == nil {
		return ""
	}
	return at.drawdownState[positionKey(symbol, side)]
}

func (at *AutoTrader) clearDrawdownExecutionFingerprint(symbol, side string) {
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.drawdownState == nil {
		return
	}
	delete(at.drawdownState, positionKey(symbol, side))
	delete(at.drawdownRunnerState, positionKey(symbol, side))
}

func dynamicCloseRatioFromFingerprint(fingerprint string) float64 {
	parts := strings.Split(fingerprint, "|")
	if len(parts) < 5 {
		return 0
	}
	value, _ := strconv.ParseFloat(parts[4], 64)
	return value
}

func (at *AutoTrader) persistDynamicProtectionRecord(symbol, side, protectionType, ruleFingerprint string, closeRatioPct float64, status string, exchangeOrderID string) {
	at.persistDynamicProtectionRecordWithDetails(symbol, side, protectionType, ruleFingerprint, closeRatioPct, status, exchangeOrderID, 0, 0, 0)
}

func (at *AutoTrader) persistDynamicProtectionRecordWithDetails(symbol, side, protectionType, ruleFingerprint string, closeRatioPct float64, status string, exchangeOrderID string, activationPrice, callbackRatio, quantity float64) {
	if at == nil || at.store == nil {
		return
	}
	positionFingerprint := ""
	parts := strings.Split(ruleFingerprint, "|")
	if len(parts) >= 2 {
		positionFingerprint = parts[0] + "|" + parts[1]
	}
	record := store.DynamicProtectionRecord{
		TraderID:            at.id,
		ExchangeID:          at.exchangeID,
		Symbol:              symbol,
		Side:                strings.ToLower(side),
		PositionFingerprint: positionFingerprint,
		ProtectionType:      protectionType,
		RuleFingerprint:     ruleFingerprint,
		CloseRatioPct:       closeRatioPct,
		Status:              status,
		ExchangeOrderID:     exchangeOrderID,
		ActivationPrice:     activationPrice,
		TriggerPrice:        activationPrice,
		StopPrice:           activationPrice,
		CallbackRatio:       callbackRatio,
		Quantity:            quantity,
	}
	if err := at.store.SaveDynamicProtectionRecord(record); err != nil {
		logger.Warnf("⚠️ Dynamic protection state: failed to persist %s for %s %s: %v", protectionType, symbol, side, err)
	}
}

func drawdownRuleFingerprint(entryPrice, quantity float64, rule store.DrawdownTakeProfitRule) string {
	rule = normalizeDrawdownRule(rule)
	return fmt.Sprintf("%.8f|%.8f|%.4f|%.4f|%.4f|%s|%.4f|%s|%s|%s|%s", entryPrice, quantity, rule.MinProfitPct, rule.MaxDrawdownPct, rule.CloseRatioPct, rule.StageName, rule.RunnerKeepPct, rule.RunnerStopMode, rule.RunnerStopSource, rule.RunnerTargetMode, rule.RunnerTargetSource)
}

func stableDrawdownRuleFingerprint(entryPrice float64, rule store.DrawdownTakeProfitRule) string {
	return drawdownRuleFingerprint(entryPrice, 0, rule)
}

func (at *AutoTrader) refreshDrawdownExecutionFingerprint(symbol, side string, entryPrice float64) bool {
	key := positionKey(symbol, side)
	base := fmt.Sprintf("%.8f|", entryPrice)

	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.drawdownState == nil {
		at.drawdownState = make(map[string]string)
	}
	prev, ok := at.drawdownState[key]
	if !ok || prev == "" {
		return false
	}
	if !strings.HasPrefix(prev, base) && prev != strings.TrimSuffix(base, "|") {
		delete(at.drawdownState, key)
		return true
	}
	return false
}

func (at *AutoTrader) setDrawdownRunnerState(symbol, side string, state *DrawdownRunnerState) {
	if state == nil {
		return
	}
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.drawdownRunnerState == nil {
		at.drawdownRunnerState = make(map[string]DrawdownRunnerState)
	}
	at.drawdownRunnerState[positionKey(symbol, side)] = *state
}

func (at *AutoTrader) getDrawdownRunnerState(symbol, side string) *DrawdownRunnerState {
	at.protectionStateMutex.RLock()
	defer at.protectionStateMutex.RUnlock()
	if at.drawdownRunnerState == nil {
		return nil
	}
	state, ok := at.drawdownRunnerState[positionKey(symbol, side)]
	if !ok {
		return nil
	}
	copyState := state
	return &copyState
}

func (at *AutoTrader) clearDrawdownRunnerState(symbol, side string) {
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.drawdownRunnerState == nil {
		return
	}
	delete(at.drawdownRunnerState, positionKey(symbol, side))
}

func (at *AutoTrader) isBreakEvenSuppressedByRunner(symbol, side string) bool {
	state := at.getDrawdownRunnerState(symbol, side)
	return state != nil && state.BreakEvenSuppressedByRunner
}

func isNativeTrailingArmingState(state string) bool {
	return state == "native_trailing_arming" || state == "native_partial_trailing_arming"
}

func (at *AutoTrader) claimProtectionArmingState(symbol, side, expectedCurrentState, armingState string) (claimed bool, previousState string, actualState string) {
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.protectionState == nil {
		at.protectionState = make(map[string]string)
	}
	key := symbol + "_" + strings.ToLower(side)
	actualState = at.protectionState[key]
	if isNativeTrailingArmingState(actualState) {
		return false, actualState, actualState
	}
	if actualState != expectedCurrentState {
		return false, actualState, actualState
	}
	at.protectionState[key] = armingState
	return true, actualState, actualState
}

func (at *AutoTrader) getProtectionState(symbol, side string) string {
	at.protectionStateMutex.RLock()
	defer at.protectionStateMutex.RUnlock()
	return at.protectionState[symbol+"_"+strings.ToLower(side)]
}

func (at *AutoTrader) setProtectionState(symbol, side, state string) {
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.protectionState == nil {
		at.protectionState = make(map[string]string)
	}
	at.protectionState[symbol+"_"+strings.ToLower(side)] = state
}

func (at *AutoTrader) clearProtectionState(symbol, side string) {
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	if at.protectionState == nil {
		return
	}
	delete(at.protectionState, symbol+"_"+strings.ToLower(side))
}

func (at *AutoTrader) setBreakEvenState(symbol, side, state string) {
	at.breakEvenStateMutex.Lock()
	defer at.breakEvenStateMutex.Unlock()
	at.breakEvenState[positionKey(symbol, side)] = state
}

func (at *AutoTrader) refreshBreakEvenFingerprint(symbol, side string, entryPrice, quantity float64) bool {
	key := positionKey(symbol, side)
	fingerprint := fmt.Sprintf("%.8f|%.8f", entryPrice, quantity)

	at.breakEvenStateMutex.Lock()
	defer at.breakEvenStateMutex.Unlock()

	changed := false
	if prev, ok := at.breakEvenFingerprints[key]; ok && prev != fingerprint {
		delete(at.breakEvenState, key)
		changed = true
	}
	at.breakEvenFingerprints[key] = fingerprint
	return changed
}

func (at *AutoTrader) getBreakEvenState(symbol, side string) string {
	at.breakEvenStateMutex.RLock()
	defer at.breakEvenStateMutex.RUnlock()
	return at.breakEvenState[symbol+"_"+strings.ToLower(side)]
}

func (at *AutoTrader) clearBreakEvenState(symbol, side string) {
	at.breakEvenStateMutex.Lock()
	defer at.breakEvenStateMutex.Unlock()
	key := positionKey(symbol, side)
	delete(at.breakEvenState, key)
	delete(at.breakEvenFingerprints, key)
}

func (at *AutoTrader) getDrawdownExecutionMode(symbol, side string) string {
	state := at.getProtectionState(symbol, side)
	if state == "managed_partial_drawdown_armed" {
		return "managed_partial_drawdown"
	}
	if state == "native_trailing_arming" || state == "native_partial_trailing_arming" {
		return "native_trailing_arming"
	}
	if state == "native_trailing_armed" || state == "native_partial_trailing_armed" {
		fullCount := 0
		partialCount := 0
		for _, record := range at.getArmedDrawdownRecords(symbol, side) {
			if record.CloseRatioPct >= 99.999 || record.ProtectionType == "native_trailing" {
				fullCount++
			} else {
				partialCount++
			}
		}
		switch {
		case partialCount > 0 && fullCount > 0:
			return "native_trailing_tiers"
		case partialCount > 1:
			return "native_partial_trailing_tiers"
		case partialCount > 0:
			return "native_partial_trailing"
		case fullCount > 0 || state == "native_trailing_armed":
			return "native_trailing_full"
		}
		return "native_partial_trailing"
	}

	rules := at.getActiveDrawdownRules()
	if len(rules) == 0 {
		return "disabled"
	}
	caps := at.GetProtectionCapabilities()
	if caps.SupportsNativePartialTrailing || caps.SupportsNativeFullTrailing {
		return "native_trailing_pending"
	}
	return "local_fallback"
}

func (at *AutoTrader) getBreakEvenExecutionMode(symbol, side string) string {
	state := at.getBreakEvenState(symbol, side)
	if state == "armed" {
		return "native_stop"
	}
	return "local_fallback"
}

func (at *AutoTrader) cleanupInactiveProtectionState(active map[string]struct{}) {
	// Before deleting local state, cancel orphaned protection orders for symbols that no longer
	// have any live positions on the exchange. This handles the case where a position is fully
	// closed but TP/SL algo orders remain on the exchange as empty orphan orders.
	inactiveSymbols := make(map[string]struct{})
	activeSymbols := make(map[string]struct{})
	for key := range active {
		symbol, _ := splitPositionKey(key)
		if symbol != "" {
			activeSymbols[symbol] = struct{}{}
		}
	}
	for key := range at.protectionState {
		if _, ok := active[key]; ok {
			continue
		}
		symbol, _ := splitPositionKey(key)
		if symbol == "" {
			continue
		}
		if _, stillActive := activeSymbols[symbol]; stillActive {
			continue // symbol still has another live side/position, do not touch orders
		}
		inactiveSymbols[symbol] = struct{}{}
	}
	for key := range at.breakEvenState {
		if _, ok := active[key]; ok {
			continue
		}
		symbol, _ := splitPositionKey(key)
		if symbol == "" {
			continue
		}
		if _, stillActive := activeSymbols[symbol]; stillActive {
			continue
		}
		inactiveSymbols[symbol] = struct{}{}
	}
	for symbol := range inactiveSymbols {
		if err := at.cancelOrphanedProtectionOrdersForInactiveSymbol(symbol); err != nil {
			logger.Warnf("⚠️ Protection cleanup: failed to cancel orphaned protection orders for %s: %v", symbol, err)
		} else {
			logger.Infof("🧹 Protection cleanup: canceled orphaned protection orders for inactive symbol %s", symbol)
		}
	}

	at.protectionStateMutex.Lock()
	for key := range at.protectionState {
		if _, ok := active[key]; !ok {
			delete(at.protectionState, key)
		}
	}
	at.protectionStateMutex.Unlock()

	at.breakEvenStateMutex.Lock()
	for key := range at.breakEvenState {
		if _, ok := active[key]; !ok {
			delete(at.breakEvenState, key)
			delete(at.breakEvenFingerprints, key)
		}
	}
	at.breakEvenStateMutex.Unlock()

	at.protectionStateMutex.Lock()
	for key := range at.drawdownRunnerState {
		if _, ok := active[key]; !ok {
			delete(at.drawdownRunnerState, key)
		}
	}
	at.protectionStateMutex.Unlock()

	at.protectionStateMutex.Lock()
	for key := range at.drawdownState {
		if _, ok := active[key]; !ok {
			delete(at.drawdownState, key)
		}
	}
	at.protectionStateMutex.Unlock()

	at.peakPnLCacheMutex.Lock()
	for key := range at.peakPnLCache {
		if _, ok := active[key]; !ok {
			delete(at.peakPnLCache, key)
		}
	}
	at.peakPnLCacheMutex.Unlock()

	// Cleanup cooldowns for inactive positions.
	reconcileCooldownMutex.Lock()
	for key := range reconcileCooldowns {
		if _, ok := active[key]; !ok {
			delete(reconcileCooldowns, key)
		}
	}
	reconcileCooldownMutex.Unlock()
}

func (at *AutoTrader) cancelOrphanedProtectionOrdersForInactiveSymbol(symbol string) error {
	// For fully inactive symbols, cancel all protection orders. In hedge/dual-side mode this
	// path is deliberately only called when no side of the symbol remains active, so broad
	// symbol cleanup is safe and prevents stale same-symbol algo orders from surviving a full close.
	if err := at.trader.CancelStopOrders(symbol); err != nil {
		return err
	}
	if trailingCanceller, ok := at.trader.(interface {
		CancelTrailingStopOrders(symbol string) error
	}); ok {
		if err := trailingCanceller.CancelTrailingStopOrders(symbol); err != nil {
			logger.Warnf("⚠️ Protection cleanup: trailing stop cleanup for inactive symbol %s returned: %v", symbol, err)
		}
	}
	if at.store != nil && at.exchangeID != "" {
		updated, err := at.store.Order().MarkSymbolProtectionOrdersCanceled(at.exchangeID, symbol)
		if err != nil {
			logger.Warnf("⚠️ Protection cleanup: failed to mark local orphan orders canceled for %s: %v", symbol, err)
		} else if updated > 0 {
			logger.Infof("🧹 Protection cleanup: marked %d local orphan order records canceled for inactive symbol %s", updated, symbol)
		}
	}
	return nil
}

func (at *AutoTrader) reconcileLocalOpenOrderStatuses(symbol string, openOrders []OpenOrder) {
	if at == nil || at.store == nil || at.exchangeID == "" || symbol == "" {
		return
	}
	liveIDs := make([]string, 0, len(openOrders))
	for _, order := range openOrders {
		liveIDs = append(liveIDs, order.OrderID)
	}
	updated, err := at.store.Order().MarkMissingOpenOrdersCanceled(at.exchangeID, symbol, liveIDs)
	if err != nil {
		logger.Warnf("⚠️ Protection reconciler: failed to reconcile local open order statuses for %s: %v", symbol, err)
		return
	}
	if updated > 0 {
		logger.Infof("🧹 Protection reconciler: marked %d local stale open order records canceled for %s", updated, symbol)
	}
}

func splitPositionKey(key string) (symbol, side string) {
	idx := strings.LastIndex(key, "_")
	if idx <= 0 || idx >= len(key)-1 {
		return "", ""
	}
	return key[:idx], key[idx+1:]
}

// setReconcileCooldown marks a position as recently reconciled, preventing re-checks for reconcileCooldownDuration.
func (at *AutoTrader) setReconcileCooldown(key string) {
	reconcileCooldownMutex.Lock()
	defer reconcileCooldownMutex.Unlock()
	reconcileCooldowns[key] = time.Now()
}

// isReconcileCooldownActive returns true if the position was reconciled within the cooldown window.
func (at *AutoTrader) isReconcileCooldownActive(key string) bool {
	reconcileCooldownMutex.RLock()
	defer reconcileCooldownMutex.RUnlock()
	if lastTime, ok := reconcileCooldowns[key]; ok {
		return time.Since(lastTime) < reconcileCooldownDuration
	}
	return false
}

// countOrdersForPositionSide counts how many open orders belong to a given position side.
func countOrdersForPositionSide(openOrders []OpenOrder, positionSide string) int {
	count := 0
	for _, order := range openOrders {
		if positionSide == "" {
			count++
			continue
		}
		if order.PositionSide == "" || strings.EqualFold(order.PositionSide, positionSide) {
			count++
		}
	}
	return count
}

// cancelProtectionOrdersForCleanup cancels all SL and TP algo orders for a symbol
// to prepare for a clean re-application of the correct protection plan.
type okxProtectionCanceller interface {
	CancelStopLossOrders(symbol string) error
	CancelTakeProfitOrders(symbol string) error
}

type okxProtectionOrderIDCanceller interface {
	CancelAlgoOrderByID(symbol string, algoID string) error
}

type okxTaggedProtectionCanceller interface {
	CancelStopLossOrdersTagged(symbol string, reasonTag string) error
	CancelTakeProfitOrdersTagged(symbol string, reasonTag string) error
}

func (at *AutoTrader) cancelUnexpectedProtectionOrdersByID(symbol string, orderIDs []string) {
	if len(orderIDs) == 0 {
		return
	}
	canceller, ok := at.trader.(okxProtectionOrderIDCanceller)
	if !ok {
		return
	}
	seen := map[string]bool{}
	for _, id := range orderIDs {
		id = strings.TrimSuffix(strings.TrimSuffix(id, "_sl"), "_tp")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if err := canceller.CancelAlgoOrderByID(symbol, id); err != nil {
			logger.Warnf("  ⚠️ Cleanup: failed to cancel unexpected algo order for %s [%s]: %v", symbol, id, err)
		}
	}
}

// cancelProtectionOrdersForCleanup performs tagged-only cleanup for active-position repair paths.
// It must not broad-cancel symbol protection while a position is active; broad cleanup is reserved
// for fully inactive symbols in cancelOrphanedProtectionOrdersForInactiveSymbol.
func (at *AutoTrader) cancelProtectionOrdersForCleanup(symbol string) {
	if tagged, ok := at.trader.(okxTaggedProtectionCanceller); ok {
		for _, tag := range []string{"ladder_sl", "full_sl", "fallback_maxloss_sl", "break_even_stop"} {
			if err := tagged.CancelStopLossOrdersTagged(symbol, tag); err != nil {
				logger.Warnf("  ⚠️ Cleanup: failed to cancel tagged SL orders for %s [%s]: %v", symbol, tag, err)
			}
		}
		for _, tag := range []string{"ladder_tp", "full_tp"} {
			if err := tagged.CancelTakeProfitOrdersTagged(symbol, tag); err != nil {
				logger.Warnf("  ⚠️ Cleanup: failed to cancel tagged TP orders for %s [%s]: %v", symbol, tag, err)
			}
		}
	}
	// Small delay to let exchange process targeted cancellations.
	time.Sleep(500 * time.Millisecond)
}
