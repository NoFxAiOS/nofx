package trader

import (
	"fmt"
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

		if err := at.reconcileProtectionForPosition(symbol, side, quantity, entryPrice); err != nil {
			logger.Infof("❌ Protection reconciler: %s %s reconcile failed: %v", symbol, side, err)
			at.setProtectionState(symbol, side, "reconcile_failed: "+err.Error())
			continue
		}

		currentState := at.getProtectionState(symbol, side)
		if currentState == "" {
			at.setProtectionState(symbol, side, "exchange_protection_verified")
		}
		logger.Infof("✅ Protection reconciler: %s %s exchange protection verified", symbol, side)
	}

	at.cleanupInactiveProtectionState(active)
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

func (at *AutoTrader) reconcileProtectionForPosition(symbol, side string, quantity, entryPrice float64) error {
	positionSide := strings.ToUpper(side)
	currentProtectionState := at.getProtectionState(symbol, side)
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("get open orders: %w", err)
	}

	// If native trailing drawdown is already armed, generic take-profit plans should not be
	// re-applied on top of it. But stop-loss protection must still be preserved and repaired.
	nativeTrailingArmed := currentProtectionState == "native_trailing_armed" || currentProtectionState == "native_partial_trailing_armed"

	plan, err := at.BuildConfiguredProtectionPlan(entryPrice, actionFromPositionSide(side))
	if err != nil {
		return fmt.Errorf("build configured plan: %w", err)
	}

	// Drawdown/native trailing owns the profit-taking side. If drawdown profit-control is enabled,
	// proactively remove old generic TP orders for the active position while keeping SL orders intact.
	drawdownEnabled := at.config.StrategyConfig != nil && at.config.StrategyConfig.Protection.DrawdownTakeProfit.Enabled && len(at.config.StrategyConfig.Protection.DrawdownTakeProfit.Rules) > 0
	if nativeTrailingArmed && plan != nil {
		plan.NeedsTakeProfit = false
		plan.TakeProfitPrice = 0
		plan.TakeProfitOrders = nil
	}
	if drawdownEnabled {
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
		if hasGenericTP {
			if canceller, ok := at.trader.(interface{ CancelTakeProfitOrders(symbol string) error }); ok {
				logger.Infof("🧹 Drawdown owner: removing legacy generic take-profit orders for %s %s while preserving stop-loss legs", symbol, positionSide)
				if err := canceller.CancelTakeProfitOrders(symbol); err != nil {
					logger.Warnf("⚠️ Failed to cancel legacy generic take-profit orders for %s: %v", symbol, err)
				}
				openOrders, _ = at.trader.GetOpenOrders(symbol)
			}
		}
	}

	if plan != nil {
		missingSL, missingTP := detectMissingProtection(openOrders, positionSide, plan)
		planOrderCount := protectionOrderCountForPlan(plan)
		breakEvenArmed := at.getBreakEvenState(symbol, side) == "armed"
		unexpectedStops, unexpectedTPs := detectUnexpectedProtectionOrders(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed)
		logger.Infof("🔎 Protection snapshot: %s %s | %s", symbol, positionSide, at.describeProtectionSnapshot(symbol, side, openOrders, plan, breakEvenArmed, nativeTrailingArmed))

		// Detect duplicate/stale orders by explicit order-role mismatch, not only coarse order counts.
		// This keeps valid break-even / trailing orders while removing old ladder/fallback debris.
		if unexpectedStops > 0 || unexpectedTPs > 0 {
			logger.Warnf("🧹 Protection reconciler: %s %s found unexpected exchange protection orders (unexpectedSL=%d unexpectedTP=%d, planned=%d), cleaning and re-applying",
				symbol, positionSide, unexpectedStops, unexpectedTPs, planOrderCount)
			at.cancelProtectionOrdersForCleanup(symbol)
			// Re-apply clean protection plan.
			if err := at.placeAndVerifyProtectionPlanWithRetry(symbol, positionSide, quantity, plan); err != nil {
				at.setReconcileCooldown(positionKey(symbol, side))
				return fmt.Errorf("cleanup re-apply protection plan: %w", err)
			}
			at.setReconcileCooldown(positionKey(symbol, side))
			return nil
		}

		if missingSL || missingTP {
			logger.Infof("🛠 Protection reconciler: %s %s missing exchange orders (SL=%v TP=%v), re-applying plan", symbol, positionSide, missingSL, missingTP)
			if err := at.placeAndVerifyProtectionPlanWithRetry(symbol, positionSide, quantity, plan); err != nil {
				at.setReconcileCooldown(positionKey(symbol, side))
				return fmt.Errorf("re-apply manual protection plan: %w", err)
			}
			at.setReconcileCooldown(positionKey(symbol, side))
			return nil
		}
	}

	markPrice, _ := at.getPositionMarkPrice(symbol, side)
	currentPnLPct := calculatePositionPnLPct(side, entryPrice, markPrice)

	be := at.getActiveBreakEvenConfigForPlan(nil)
	fingerprintChanged := at.refreshBreakEvenFingerprint(symbol, side, entryPrice, quantity)
	prevBreakEvenArmed := at.getBreakEvenState(symbol, side) == "armed"
	if be != nil && at.GetProtectionCapabilities().NativeStopLoss {
		if prevBreakEvenArmed && fingerprintChanged {
			logger.Infof("🛠 Protection reconciler: %s %s break-even fingerprint changed, re-arming native stop", symbol, positionSide)
			if err := at.applyBreakEvenStop(symbol, side, quantity, entryPrice, currentPnLPct, *be); err != nil {
				return fmt.Errorf("re-arm break-even native stop: %w", err)
			}
			at.setBreakEvenState(symbol, side, "armed")
		} else if at.getBreakEvenState(symbol, side) != "armed" && currentPnLPct >= be.TriggerValue {
			logger.Infof("🛠 Protection reconciler: %s %s break-even trigger met (%.2f%% >= %.2f%%), applying native stop", symbol, positionSide, currentPnLPct, be.TriggerValue)
			// Mark as arming before placement so overlapping reconcile turns do not race and
			// place duplicate native break-even stops for the same position snapshot.
			at.setBreakEvenState(symbol, side, "arming")
			if err := at.applyBreakEvenStop(symbol, side, quantity, entryPrice, currentPnLPct, *be); err != nil {
				at.setBreakEvenState(symbol, side, "pending")
				return fmt.Errorf("apply break-even native stop: %w", err)
			}
			at.setBreakEvenState(symbol, side, "armed")
		}
	}

	rules := at.getActiveDrawdownRules()
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
		for _, armRule := range at.getDrawdownArmRules(currentPnLPct, rules) {
			if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, armRule) {
				logger.Infof("🛠 Protection reconciler: %s %s ensured native drawdown protection (arm close=%.1f%%)", symbol, positionSide, armRule.CloseRatioPct)
			}
		}
		for _, triggeredRule := range at.getTriggeredDrawdownRules(currentPnLPct, drawdownPct, rules) {
			if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, triggeredRule) {
				logger.Infof("🛠 Protection reconciler: %s %s ensured native drawdown protection (trigger close=%.1f%%)", symbol, positionSide, triggeredRule.CloseRatioPct)
			}
		}
	}

	return nil
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
	allowedStops := make(map[string]int)
	allowedTPs := make(map[string]int)

	if plan != nil {
		for _, target := range plan.StopLossOrders {
			allowedStops[fmt.Sprintf("%.8f", target.Price)]++
		}
		if len(plan.StopLossOrders) == 0 && plan.NeedsStopLoss && plan.StopLossPrice > 0 {
			allowedStops[fmt.Sprintf("%.8f", plan.StopLossPrice)]++
		}
		if plan.FallbackMaxLossPrice > 0 {
			allowedStops[fmt.Sprintf("%.8f", plan.FallbackMaxLossPrice)]++
		}
		for _, target := range plan.TakeProfitOrders {
			allowedTPs[fmt.Sprintf("%.8f", target.Price)]++
		}
		if len(plan.TakeProfitOrders) == 0 && plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 {
			allowedTPs[fmt.Sprintf("%.8f", plan.TakeProfitPrice)]++
		}
	}

	for _, order := range openOrders {
		if positionSide != "" && order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}

		if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			if nativeTrailingArmed {
				continue
			}
			unexpectedStops++
			continue
		}

		key := fmt.Sprintf("%.8f", order.StopPrice)
		if looksLikeTakeProfit(order) {
			if allowedTPs[key] > 0 {
				allowedTPs[key]--
				continue
			}
			unexpectedTPs++
			continue
		}
		if looksLikeStopLoss(order) {
			if allowedStops[key] > 0 {
				allowedStops[key]--
				continue
			}
			// One additional stop can be valid when break-even is independently armed.
			if breakEvenArmed {
				breakEvenArmed = false
				continue
			}
			unexpectedStops++
		}
	}

	return unexpectedStops, unexpectedTPs
}

func hasExplicitBreakEvenConfig(config *store.StrategyConfig) bool {
	if config == nil {
		return false
	}
	be := config.Protection.BreakEvenStop
	return be.Enabled && be.TriggerMode == store.BreakEvenTriggerProfitPct && be.TriggerValue > 0
}

func detectMissingProtection(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan) (missingSL bool, missingTP bool) {
	if plan == nil {
		return false, false
	}

	fallbackSatisfied := plan.FallbackMaxLossPrice > 0 && hasMatchingProtectionOrder(openOrders, positionSide, false, plan.FallbackMaxLossPrice)

	// For stop-loss side, treat ladder plans as requiring ALL configured stop orders, not only when >1 tiers.
	// This matters because break-even / trailing can add extra stop-like orders that would otherwise make a
	// single-tier ladder look "present" while the intended ladder stop is actually missing.
	if len(plan.StopLossOrders) > 0 {
		for _, target := range plan.StopLossOrders {
			if countMatchingProtectionOrders(openOrders, positionSide, false, target.Price) == 0 {
				missingSL = true
				break
			}
		}
	} else if plan.NeedsStopLoss {
		missingSL = !hasMatchingProtectionOrder(openOrders, positionSide, false, plan.StopLossPrice)
	}

	if !missingSL && plan.FallbackMaxLossPrice > 0 && !fallbackSatisfied {
		missingSL = true
	}

	// Same rule for take-profit: when ladder TP orders exist, require each configured tier explicitly.
	if len(plan.TakeProfitOrders) > 0 {
		for _, target := range plan.TakeProfitOrders {
			if countMatchingProtectionOrders(openOrders, positionSide, true, target.Price) == 0 {
				missingTP = true
				break
			}
		}
	} else if plan.NeedsTakeProfit {
		missingTP = !hasMatchingProtectionOrder(openOrders, positionSide, true, plan.TakeProfitPrice)
	}

	return missingSL, missingTP
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

func (at *AutoTrader) setProtectionState(symbol, side, state string) {
	at.protectionStateMutex.Lock()
	defer at.protectionStateMutex.Unlock()
	at.protectionState[symbol+"_"+strings.ToLower(side)] = state
}

func (at *AutoTrader) getProtectionState(symbol, side string) string {
	at.protectionStateMutex.RLock()
	defer at.protectionStateMutex.RUnlock()
	return at.protectionState[symbol+"_"+strings.ToLower(side)]
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
	switch state {
	case "native_trailing_armed":
		return "native_trailing_full"
	case "native_partial_trailing_armed":
		return "native_partial_trailing"
	case "managed_partial_drawdown_armed":
		return "managed_partial_drawdown"
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
	// For inactive symbols, cancel plain stop/take-profit protection orders.
	// Native trailing orders live in separate exchange endpoints and should only be
	// cancelled by their dedicated trailing-order APIs, not by generic TP/SL cleanup.
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
	return nil
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

type okxTaggedProtectionCanceller interface {
	CancelStopLossOrdersTagged(symbol string, reasonTag string) error
	CancelTakeProfitOrdersTagged(symbol string, reasonTag string) error
}

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
	} else if canceller, ok := at.trader.(okxProtectionCanceller); ok {
		if err := canceller.CancelStopLossOrders(symbol); err != nil {
			logger.Warnf("  ⚠️ Cleanup: failed to cancel SL orders for %s: %v", symbol, err)
		}
		if err := canceller.CancelTakeProfitOrders(symbol); err != nil {
			logger.Warnf("  ⚠️ Cleanup: failed to cancel TP orders for %s: %v", symbol, err)
		}
	}
	// Small delay to let exchange process cancellations.
	time.Sleep(500 * time.Millisecond)
}
