package trader

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/store"
	"sort"
	"strings"
	"time"
)

// startDrawdownMonitor 启动运行态回撤监控协程。
// 这条链路属于持仓后的风险保护，和开仓前的 AI 决策风控不同，
// 它的职责是在仓位已存在时继续兜底处理利润回撤与异常退出。
const (
	minNativeDrawdownCallbackRatio = 0.003 // 0.3% price callback; OKX minimum 0.1% is too tight for strategy-level drawdown
)

type nativeDrawdownCallbackAdjustment struct {
	CallbackRatio float64
	Adjusted      bool
	Reason        string
}

type nativeDrawdownRejection struct {
	CallbackRatio float64
	SafetyFloor   float64
	Reason        string
}

func (e nativeDrawdownRejection) Error() string {
	return fmt.Sprintf("drawdown callback %.6f below native safety floor %.6f; use managed drawdown fallback instead of widening callback", e.CallbackRatio, e.SafetyFloor)
}

func adjustNativeDrawdownCallbackRatio(entryPrice float64, side string, rule store.DrawdownTakeProfitRule, callbackRatio float64) (nativeDrawdownCallbackAdjustment, error) {
	adjustment := nativeDrawdownCallbackAdjustment{CallbackRatio: callbackRatio}
	activationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
	if entryPrice <= 0 || activationPrice <= 0 || callbackRatio <= 0 {
		return adjustment, fmt.Errorf("invalid drawdown callback inputs")
	}
	if callbackRatio >= minNativeDrawdownCallbackRatio {
		return adjustment, nil
	}
	return adjustment, nativeDrawdownRejection{CallbackRatio: callbackRatio, SafetyFloor: minNativeDrawdownCallbackRatio, Reason: "below_native_safety_floor"}
}

func calculateImmediateTrailingCallback(entryPrice float64, side string, ladderSLOrders []ProtectionOrder) (float64, bool) {
	if entryPrice <= 0 || len(ladderSLOrders) == 0 {
		return 0, false
	}
	firstSL := ladderSLOrders[0].Price
	if firstSL <= 0 {
		return 0, false
	}
	callback := math.Abs(entryPrice-firstSL) / entryPrice
	if callback < minNativeDrawdownCallbackRatio {
		return 0, false
	}
	return callback, true
}

func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		interval := at.getDrawdownMonitorInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logger.Infof("📊 Started position drawdown monitoring (check every %s)", interval)

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped position drawdown monitoring")
				return
			}
		}
	}()
}

func (at *AutoTrader) getDrawdownMonitorInterval() time.Duration {
	if at.config.StrategyConfig == nil {
		return time.Minute
	}

	drawdown := at.config.StrategyConfig.Protection.DrawdownTakeProfit
	if !drawdown.Enabled {
		return time.Minute
	}

	minSeconds := 60
	for _, rule := range drawdown.Rules {
		if rule.PollIntervalSeconds > 0 && rule.PollIntervalSeconds < minSeconds {
			minSeconds = rule.PollIntervalSeconds
		}
	}
	if minSeconds < 5 {
		minSeconds = 5
	}
	return time.Duration(minSeconds) * time.Second
}

// checkPositionDrawdown checks position drawdown situation
func (at *AutoTrader) checkPositionDrawdown() {
	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("❌ Drawdown monitoring: failed to get positions: %v", err)
		return
	}

	drawdownCfg := store.DrawdownTakeProfitConfig{}
	if at.config.StrategyConfig != nil {
		drawdownCfg = at.config.StrategyConfig.Protection.DrawdownTakeProfit
	}

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		rules := at.getActiveDrawdownRulesForPosition(symbol, side)
		if len(rules) == 0 {
			continue
		}

		// Calculate current P&L percentage using pure price move, not leveraged return on margin.
		// Protection logic must stay invariant when leverage changes.
		currentPnLPct := calculatePositionPnLPct(side, entryPrice, markPrice)

		// Construct unique position identifier (distinguish long/short)
		posKey := symbol + "_" + side

		// Get historical peak profit for this position
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// If no historical peak record, use current P&L as initial value
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// Update peak cache
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// Calculate drawdown (magnitude of decline from peak)
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		if fingerprintChanged := at.refreshDrawdownExecutionFingerprint(symbol, side, entryPrice); fingerprintChanged {
			logger.Infof("🟠 Drawdown monitor: %s %s drawdown entry fingerprint changed, clearing previous execution guard", symbol, side)
		}

		matchedBreakEvenRules := at.getActiveBreakEvenRules()
		if len(matchedBreakEvenRules) > 0 {
			if at.isBreakEvenSuppressedByRunner(symbol, side) {
				logger.Infof("🟠 Break-even monitor: %s %s suppressed by runner semantics, skipping mechanical BE apply", symbol, side)
			} else {
				if err := at.applyBreakEvenStops(symbol, side, quantity, entryPrice, currentPnLPct, matchedBreakEvenRules); err != nil {
					logger.Infof("❌ Break-even stop apply failed (%s %s): %v", symbol, side, err)
				}
			}
		}

		structureCtx := at.buildDrawdownStructureContext(symbol, side)

		// For exchange-native trailing protections, arm all tiers whose min-profit gate is already met.
		// Do NOT wait for drawdown to happen first — the exchange trailing order itself is responsible
		// for tracking the drawdown once armed.
		if at.supportsNativeTrailingStop() {
			executionMode := at.getDrawdownExecutionMode(symbol, side)
			armRules := at.getDrawdownArmRulesForNativeExposure(currentPnLPct, entryPrice, quantity, symbol, side, rules)
			if len(armRules) == 0 && isNativeTrailingProtectionState(at.getProtectionState(symbol, side)) {
				logger.Infof("🟣 Drawdown monitor: %s %s already has all satisfied native trailing tiers armed (%s), skipping duplicate arm pass", symbol, side, executionMode)
			} else {
				armedAny := false
				for _, armRule := range armRules {
					if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, armRule) {
						armedAny = true
					}
				}
				if armedAny {
					continue
				}
			}
		}

		triggeredRules := at.getTriggeredDrawdownRules(currentPnLPct, drawdownPct, rules)
		if len(triggeredRules) > 0 {
			triggeredRules = []store.DrawdownTakeProfitRule{enforceDrawdownRunnerPolicy(drawdownCfg, normalizeDrawdownRule(triggeredRules[0]))}
		}
		if drawdownCfg.Enabled && drawdownCfg.Mode == store.ProtectionModeAI && drawdownCfg.EngineMode == store.DrawdownEngineModeAI {
			if eval := evaluateAIDrawdownRule(drawdownCfg, currentPnLPct, peakPnLPct, drawdownPct, rules, structureCtx, side, markPrice); eval != nil {
				triggeredRules = []store.DrawdownTakeProfitRule{eval.Rule}
			} else {
				triggeredRules = nil
			}
		}
		if len(triggeredRules) == 0 {
			if currentPnLPct > 0 {
				logger.Infof("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
					symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
			}
			continue
		}

		if at.supportsNativeTrailingStop() {
			for _, triggeredRule := range triggeredRules {
				at.applyNativeTrailingDrawdown(symbol, side, entryPrice, triggeredRule)
			}
			if at.hasArmedNativeDrawdownForPosition(symbol, side, entryPrice) {
				logger.Infof("🟣 Drawdown monitor: %s %s native trailing drawdown is armed; skipping managed market close fallback", symbol, side)
				continue
			}
		}

		matchedRule := normalizeDrawdownRule(triggeredRules[0])
		ruleFingerprint := stableDrawdownRuleFingerprint(entryPrice, matchedRule)
		if at.getDrawdownExecutionFingerprint(symbol, side) == ruleFingerprint {
			logger.Infof("🟠 Drawdown monitor: %s %s rule already executed (fingerprint=%s), skipping duplicate close", symbol, side, ruleFingerprint)
			continue
		}
		closeQty := quantity * matchedRule.CloseRatioPct / 100.0
		if closeQty <= 0 || matchedRule.CloseRatioPct >= 99.999 {
			closeQty = 0 // exchange adapters use 0 to mean close all
		}

		logger.Infof("🚨 Drawdown take-profit triggered: %s %s | Current profit: %.2f%% | Peak profit: %.2f%% | Drawdown: %.2f%% | CloseRatio: %.2f%%",
			symbol, side, currentPnLPct, peakPnLPct, drawdownPct, matchedRule.CloseRatioPct)

		if err := at.closePositionByReason(symbol, side, closeQty, "managed_drawdown"); err != nil {
			logger.Infof("❌ Drawdown take-profit failed (%s %s): %v", symbol, side, err)
			continue
		}

		logger.Infof("✅ Drawdown take-profit succeeded: %s %s", symbol, side)
		at.setDrawdownExecutionFingerprint(symbol, side, ruleFingerprint)
		at.setProtectionState(symbol, side, "drawdown_triggered")
		at.setDrawdownRunnerState(symbol, side, buildDrawdownRunnerState(matchedRule))
		if closeQty == 0 {
			at.ClearPeakPnLCache(symbol, side)
			at.clearBreakEvenState(symbol, side)
			at.clearDrawdownExecutionFingerprint(symbol, side)
		} else {
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}
	}
}

func (at *AutoTrader) setAIDrawdownRules(symbol, side string, rules []store.DrawdownTakeProfitRule) {
	if len(rules) == 0 {
		return
	}
	key := positionKey(symbol, side)
	cloned := make([]store.DrawdownTakeProfitRule, 0, len(rules))
	for _, rule := range rules {
		rule = normalizeDrawdownRule(rule)
		if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
			continue
		}
		cloned = append(cloned, rule)
	}
	if len(cloned) == 0 {
		return
	}
	at.protectionStateMutex.Lock()
	if at.drawdownAIRules == nil {
		at.drawdownAIRules = make(map[string][]store.DrawdownTakeProfitRule)
	}
	at.drawdownAIRules[key] = cloned
	at.drawdownSource[key] = "ai_decision"
	at.protectionStateMutex.Unlock()
}

func (at *AutoTrader) restoreAIDrawdownRulesForPosition(symbol, side string) []store.DrawdownTakeProfitRule {
	return at.restoreAIDrawdownRulesForPositionWithEntry(symbol, side, 0)
}

func (at *AutoTrader) restoreAIDrawdownRulesForPositionWithEntry(symbol, side string, fallbackEntryPrice float64) []store.DrawdownTakeProfitRule {
	if at == nil || at.store == nil || symbol == "" || side == "" {
		return nil
	}
	pos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side))
	if err != nil || pos == nil {
		return nil
	}
	if pos.EntryDecisionCycle <= 0 {
		if inferred := at.store.Position().FindEntryDecisionCycleForPosition(at.id, symbol, strings.ToUpper(side), pos.EntryTime); inferred > 0 {
			pos.EntryDecisionCycle = inferred
			_ = at.store.Position().BackfillEntryDecisionCycle(pos.ID, inferred)
		}
	}
	if pos.EntryDecisionCycle <= 0 {
		return nil
	}
	record, err := at.store.Decision().GetRecordByCycle(at.id, pos.EntryDecisionCycle)
	if err != nil || record == nil {
		return nil
	}
	action := sideToOpenAction(side)
	entryPrice := pos.EntryPrice
	if entryPrice <= 0 {
		entryPrice = fallbackEntryPrice
	}
	if entryPrice <= 0 {
		entryPrice = recordActionPrice(record, symbol, action)
	}
	if entryPrice <= 0 {
		return nil
	}
	for _, payload := range []string{record.DecisionJSON, record.RawResponse} {
		if strings.TrimSpace(payload) == "" {
			continue
		}
		var decisions []kernel.Decision
		if err := json.Unmarshal([]byte(payload), &decisions); err != nil {
			continue
		}
		for i := range decisions {
			decision := decisions[i]
			if !strings.EqualFold(decision.Symbol, symbol) || !strings.EqualFold(decision.Action, action) || decision.ProtectionPlan == nil {
				continue
			}
			plan, err := buildAIProtectionPlan(entryPrice, decision.Action, decision.ProtectionPlan, at.config.StrategyConfig)
			if err != nil || plan == nil || len(plan.DrawdownRules) == 0 {
				continue
			}
			at.setAIDrawdownRules(symbol, side, plan.DrawdownRules)
			return plan.DrawdownRules
		}
	}
	return nil
}

func (at *AutoTrader) restoreAIProtectionPlanForPositionWithEntry(symbol, side string, fallbackEntryPrice float64) *ProtectionPlan {
	if at == nil || at.store == nil || symbol == "" || side == "" {
		return nil
	}
	entryDecisionCycle := 0
	entryPrice := fallbackEntryPrice
	pos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side))
	if err == nil && pos != nil {
		entryDecisionCycle = pos.EntryDecisionCycle
		if entryPrice <= 0 {
			entryPrice = pos.EntryPrice
		}
		if entryDecisionCycle <= 0 {
			if inferred := at.store.Position().FindEntryDecisionCycleForPosition(at.id, symbol, strings.ToUpper(side), pos.EntryTime); inferred > 0 {
				entryDecisionCycle = inferred
				_ = at.store.Position().BackfillEntryDecisionCycle(pos.ID, inferred)
			}
		}
	}
	// If local position state is stale/missing but exchange still reports an active
	// position, fall back to the latest matching open decision. This keeps mandatory
	// ladder SL repair alive even when position sync marked a live position CLOSED.
	if entryDecisionCycle <= 0 {
		entryDecisionCycle = at.store.Position().FindEntryDecisionCycleForPosition(at.id, symbol, strings.ToUpper(side), 0)
	}
	if entryDecisionCycle <= 0 {
		return nil
	}
	record, err := at.store.Decision().GetRecordByCycle(at.id, entryDecisionCycle)
	if err != nil || record == nil {
		return nil
	}
	action := sideToOpenAction(side)
	if entryPrice <= 0 {
		entryPrice = recordActionPrice(record, symbol, action)
	}
	if entryPrice <= 0 {
		return nil
	}
	for _, payload := range []string{record.DecisionJSON, record.RawResponse} {
		if strings.TrimSpace(payload) == "" {
			continue
		}
		var decisions []kernel.Decision
		if err := json.Unmarshal([]byte(payload), &decisions); err != nil {
			continue
		}
		for i := range decisions {
			decision := decisions[i]
			if !strings.EqualFold(decision.Symbol, symbol) || !strings.EqualFold(decision.Action, action) || decision.ProtectionPlan == nil {
				continue
			}
			plan, err := buildAIProtectionPlan(entryPrice, decision.Action, decision.ProtectionPlan, at.config.StrategyConfig)
			if err != nil || plan == nil {
				continue
			}
			if len(plan.DrawdownRules) > 0 {
				at.setAIDrawdownRules(symbol, side, plan.DrawdownRules)
			}
			return plan
		}
	}
	return nil
}

func recordActionPrice(record *store.DecisionRecord, symbol, action string) float64 {
	candidate := findMatchedDecisionAction(record, symbol, action)
	if candidate == nil {
		return 0
	}
	return candidate.Price
}

func (at *AutoTrader) getActiveDrawdownRules() []store.DrawdownTakeProfitRule {
	return at.getActiveDrawdownRulesForPosition("", "")
}

func (at *AutoTrader) getActiveDrawdownRulesForPosition(symbol, side string) []store.DrawdownTakeProfitRule {
	if at.config.StrategyConfig == nil {
		return nil
	}

	cfg := at.config.StrategyConfig.Protection.DrawdownTakeProfit
	if !cfg.Enabled {
		return nil
	}

	if symbol != "" || side != "" {
		key := positionKey(symbol, side)
		at.protectionStateMutex.RLock()
		if rules := at.drawdownAIRules[key]; len(rules) > 0 {
			out := make([]store.DrawdownTakeProfitRule, 0, len(rules))
			for _, rule := range rules {
				out = append(out, normalizeDrawdownRule(rule))
			}
			at.protectionStateMutex.RUnlock()
			return out
		}
		at.protectionStateMutex.RUnlock()

		if cfg.Mode == store.ProtectionModeAI && at.store != nil {
			if restored := at.restoreAIDrawdownRulesForPositionWithEntry(symbol, side, 0); len(restored) > 0 {
				return restored
			}
			// AI restore failed — fall through to configured fallback rules
		}
	}

	if len(cfg.Rules) == 0 {
		return nil
	}

	rules := make([]store.DrawdownTakeProfitRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
			continue
		}
		if rule.CloseRatioPct > 100 {
			rule.CloseRatioPct = 100
		}
		rules = append(rules, normalizeDrawdownRule(rule))
	}
	return rules
}

func isNativeTrailingProtectionState(state string) bool {
	return state == "native_trailing_arming" || state == "native_trailing_armed" || state == "native_partial_trailing_arming" || state == "native_partial_trailing_armed"
}

func isDrawdownRuleSatisfied(currentPnLPct float64, rule store.DrawdownTakeProfitRule) bool {
	return currentPnLPct >= rule.MinProfitPct
}

func (at *AutoTrader) getArmedDrawdownRecords(symbol, side string) []store.DynamicProtectionRecord {
	return at.getArmedDrawdownRecordsForPosition(symbol, side, 0, 0)
}

func (at *AutoTrader) getArmedDrawdownRecordsForPosition(symbol, side string, entryPrice, quantity float64) []store.DynamicProtectionRecord {
	if at.store == nil {
		return nil
	}
	state, err := at.store.LoadDynamicProtectionState()
	if err != nil || state == nil {
		return nil
	}
	currentFingerprint := positionFingerprint(entryPrice, quantity)
	currentEntryFingerprint := entryPositionFingerprint(entryPrice)
	records := make([]store.DynamicProtectionRecord, 0)
	for _, record := range state.Records {
		if record.TraderID != "" && record.TraderID != at.id {
			continue
		}
		if !strings.EqualFold(record.Symbol, symbol) || !strings.EqualFold(record.Side, side) {
			continue
		}
		if record.Status != "armed" || !isDynamicNativeProtectionType(record.ProtectionType) {
			continue
		}
		if currentEntryFingerprint != "" && record.PositionFingerprint != "" {
			if recordEntryFingerprint(record.PositionFingerprint) != currentEntryFingerprint {
				logger.Infof("🟣 Drawdown record ignored for current position: %s %s record_fp=%s current_fp=%s type=%s", symbol, side, record.PositionFingerprint, currentFingerprint, record.ProtectionType)
				continue
			}
		}
		records = append(records, record)
	}
	return records
}

func positionFingerprint(entryPrice, quantity float64) string {
	if entryPrice <= 0 || quantity <= 0 {
		return ""
	}
	return fmt.Sprintf("%.8f|%.8f", entryPrice, quantity)
}

func entryPositionFingerprint(entryPrice float64) string {
	if entryPrice <= 0 {
		return ""
	}
	return fmt.Sprintf("%.8f", entryPrice)
}

func recordEntryFingerprint(positionFingerprint string) string {
	parts := strings.Split(positionFingerprint, "|")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func (at *AutoTrader) hasArmedNativeDrawdownForPosition(symbol, side string, entryPrice float64) bool {
	return len(at.getArmedDrawdownRecordsForPosition(symbol, side, entryPrice, 0)) > 0
}

func (at *AutoTrader) getArmedDrawdownRuleFingerprints(symbol, side string) map[string]struct{} {
	return at.getArmedDrawdownRuleFingerprintsForPosition(symbol, side, 0, 0)
}

func (at *AutoTrader) getArmedDrawdownRuleFingerprintsForPosition(symbol, side string, entryPrice, quantity float64) map[string]struct{} {
	armed := make(map[string]struct{})
	for _, record := range at.getArmedDrawdownRecordsForPosition(symbol, side, entryPrice, quantity) {
		if record.RuleFingerprint != "" {
			armed[record.RuleFingerprint] = struct{}{}
		}
	}
	return armed
}

func (at *AutoTrader) hasMatchingNativeTrailingOrderForRule(symbol, side string, entryPrice float64, rule store.DrawdownTakeProfitRule, openOrders []OpenOrder) bool {
	if len(openOrders) == 0 {
		return false
	}
	plannedActivationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
	plannedCallbackRate := calculateDrawdownRuleCallbackRatio(entryPrice, side, rule)
	for _, order := range openOrders {
		if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
			continue
		}
		if !strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			continue
		}
		callback := order.CallbackRate
		if callback <= 0 && order.CallbackRatePct > 0 {
			callback = order.CallbackRatePct / 100.0
		}
		if callback > 1 {
			callback = callback / 100.0
		}
		callbackTolerance := 0.00025
		activationOK := true
		if order.StopPrice > 0 && plannedActivationPrice > 0 {
			activationDrift := math.Abs(order.StopPrice-plannedActivationPrice) / math.Max(math.Abs(order.StopPrice), math.Abs(plannedActivationPrice))
			activationOK = activationDrift <= 0.004
		}
		if activationOK && math.Abs(callback-plannedCallbackRate) <= callbackTolerance {
			return true
		}
	}
	return false
}

func (at *AutoTrader) getDrawdownArmRulesForNativeExposure(currentPnLPct, entryPrice, quantity float64, symbol, side string, rules []store.DrawdownTakeProfitRule) []store.DrawdownTakeProfitRule {
	// Exchange-native trailing coverage is intentionally single-tier on OKX-style
	// venues. Live OKX behaviour showed multiple simultaneous trailing tiers on the
	// same symbol/side can churn (place/cancel/place). Keep one exchange tier stable
	// and let local managed drawdown + reconciler migrate it as profit advances.
	rule, ok := selectNativeDrawdownExposureRule(currentPnLPct, rules)
	if !ok {
		return nil
	}
	return at.getDrawdownArmRulesForSelectedRule(entryPrice, quantity, symbol, side, rule)
}

func selectNativeDrawdownExposureRule(currentPnLPct float64, rules []store.DrawdownTakeProfitRule) (store.DrawdownTakeProfitRule, bool) {
	bestSatisfied := store.DrawdownTakeProfitRule{}
	hasSatisfied := false
	next := store.DrawdownTakeProfitRule{}
	hasNext := false
	for _, raw := range rules {
		rule := normalizeDrawdownRule(raw)
		if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
			continue
		}
		if currentPnLPct >= rule.MinProfitPct {
			if !hasSatisfied || rule.MinProfitPct > bestSatisfied.MinProfitPct {
				bestSatisfied = rule
				hasSatisfied = true
			}
			continue
		}
		if !hasNext || rule.MinProfitPct < next.MinProfitPct {
			next = rule
			hasNext = true
		}
	}
	if hasSatisfied {
		return bestSatisfied, true
	}
	return next, hasNext
}

func (at *AutoTrader) getDrawdownArmRulesForSelectedRule(entryPrice, quantity float64, symbol, side string, rule store.DrawdownTakeProfitRule) []store.DrawdownTakeProfitRule {
	rule = normalizeDrawdownRule(rule)
	armedFingerprints := at.getArmedDrawdownRuleFingerprintsForPosition(symbol, side, entryPrice, quantity)
	openOrders, _ := at.trader.GetOpenOrders(symbol)
	fingerprint := stableDrawdownRuleFingerprint(entryPrice, rule)
	if _, ok := armedFingerprints[fingerprint]; ok {
		if at.hasMatchingNativeTrailingOrderForRule(symbol, side, entryPrice, rule, openOrders) {
			logger.Infof("🟣 Drawdown native exposure skipped: %s %s already armed fingerprint=%s", symbol, side, fingerprint)
			return nil
		}
		logger.Infof("⚠️ Drawdown native exposure record stale: %s %s fingerprint=%s has no matching exchange trailing order, re-arming", symbol, side, fingerprint)
	}
	logger.Infof("🟣 Drawdown native exposure selected: %s %s min=%.4f close=%.1f%% fingerprint=%s", symbol, side, rule.MinProfitPct, rule.CloseRatioPct, fingerprint)
	return []store.DrawdownTakeProfitRule{rule}
}

func (at *AutoTrader) getDrawdownArmRules(currentPnLPct, entryPrice, quantity float64, symbol, side string, rules []store.DrawdownTakeProfitRule) []store.DrawdownTakeProfitRule {
	armedFingerprints := at.getArmedDrawdownRuleFingerprintsForPosition(symbol, side, entryPrice, quantity)
	openOrders, _ := at.trader.GetOpenOrders(symbol)
	matched := make([]store.DrawdownTakeProfitRule, 0, len(rules))
	for _, rule := range rules {
		rule = normalizeDrawdownRule(rule)
		if !isDrawdownRuleSatisfied(currentPnLPct, rule) {
			logger.Infof("🟣 Drawdown arm pending: %s %s profit %.4f below min %.4f (close=%.1f%%)", symbol, side, currentPnLPct, rule.MinProfitPct, rule.CloseRatioPct)
			continue
		}
		fingerprint := stableDrawdownRuleFingerprint(entryPrice, rule)
		if _, ok := armedFingerprints[fingerprint]; ok {
			if at.hasMatchingNativeTrailingOrderForRule(symbol, side, entryPrice, rule, openOrders) {
				logger.Infof("🟣 Drawdown arm skipped: %s %s already armed fingerprint=%s", symbol, side, fingerprint)
				continue
			}
			logger.Infof("⚠️ Drawdown arm record stale: %s %s fingerprint=%s has no matching exchange trailing order, re-arming", symbol, side, fingerprint)
		}
		logger.Infof("🟣 Drawdown arm eligible: %s %s profit %.4f >= min %.4f close=%.1f%% fingerprint=%s", symbol, side, currentPnLPct, rule.MinProfitPct, rule.CloseRatioPct, fingerprint)
		matched = append(matched, rule)
	}
	return matched
}

func sideToOpenAction(side string) string {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "LONG":
		return "open_long"
	case "SHORT":
		return "open_short"
	default:
		return ""
	}
}

func findMatchedDecisionAction(record *store.DecisionRecord, symbol, action string) *store.DecisionAction {
	if record == nil {
		return nil
	}
	for i := range record.Decisions {
		candidate := &record.Decisions[i]
		if symbol != "" && !strings.EqualFold(candidate.Symbol, symbol) {
			continue
		}
		if action != "" && !strings.EqualFold(candidate.Action, action) {
			continue
		}
		return candidate
	}
	return nil
}

func extractDecisionReviewMap(actionReview *store.DecisionActionReviewContext) map[string]interface{} {
	if actionReview == nil {
		return nil
	}
	payload, err := json.Marshal(actionReview)
	if err != nil {
		return nil
	}
	decoded := map[string]interface{}{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil
	}
	return decoded
}

func buildEntryReviewSummaryFromDecisionReview(review map[string]interface{}) map[string]interface{} {
	if review == nil {
		return nil
	}
	summary := map[string]interface{}{}
	for _, key := range []string{"timeframe_context", "risk_reward", "key_levels", "anchors", "alignment_notes", "protection", "control", "execution_constraints"} {
		if value, ok := review[key]; ok {
			summary[key] = value
		}
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}

func (at *AutoTrader) buildDrawdownStructureContext(symbol, side string) *drawdownStructureContext {
	if at.store == nil {
		return nil
	}
	positionStore := at.store.Position()
	decisionStore := at.store.Decision()
	if positionStore == nil || decisionStore == nil {
		return nil
	}
	openPos, err := positionStore.GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side))
	if err != nil || openPos == nil {
		return nil
	}
	entryDecisionCycle := openPos.EntryDecisionCycle
	if entryDecisionCycle <= 0 {
		if inferred := positionStore.FindEntryDecisionCycleForPosition(at.id, symbol, strings.ToUpper(side), openPos.EntryTime); inferred > 0 {
			entryDecisionCycle = inferred
			if err := positionStore.BackfillEntryDecisionCycle(openPos.ID, inferred); err != nil {
				logger.Infof("⚠️ Failed to backfill entry decision cycle for %s %s position %d: %v", symbol, side, openPos.ID, err)
			} else {
				logger.Infof("🧷 Backfilled entry decision cycle for %s %s position %d -> cycle %d", symbol, side, openPos.ID, inferred)
			}
		}
	}
	if entryDecisionCycle <= 0 {
		return nil
	}
	record, err := decisionStore.GetRecordByCycle(at.id, entryDecisionCycle)
	if err != nil || record == nil {
		return nil
	}

	ctx := &drawdownStructureContext{}
	candidate := findMatchedDecisionAction(record, symbol, sideToOpenAction(strings.ToUpper(side)))
	decoded := extractDecisionReviewMap(func() *store.DecisionActionReviewContext {
		if candidate != nil {
			return candidate.ReviewContext
		}
		return nil
	}())
	if decoded == nil {
		return nil
	}
	if review, ok := decoded["timeframe_context"].(map[string]interface{}); ok {
		ctx.PrimaryTimeframe, _ = review["primary"].(string)
		ctx.LowerTimeframes = readStringSlice(review["lower"])
		ctx.HigherTimeframes = readStringSlice(review["higher"])
	}
	if rr, ok := decoded["risk_reward"].(map[string]interface{}); ok {
		ctx.Entry = readFloat(rr["entry"])
		ctx.Invalidation = readFloat(rr["invalidation"])
		ctx.FirstTarget = readFloat(rr["first_target"])
	}
	if levels, ok := decoded["key_levels"].(map[string]interface{}); ok {
		ctx.Support = readFloatSlice(levels["support"])
		ctx.Resistance = readFloatSlice(levels["resistance"])
		if fib, ok := levels["fibonacci"].(map[string]interface{}); ok {
			ctx.FibLevels = readFloatSlice(fib["levels"])
		}
	}
	ctx.Anchors = append(ctx.Anchors, readDecisionAnchors(decoded["anchors"])...)
	ctx.Anchors = append(ctx.Anchors, readDecisionAnchors(decoded["higher_timeframe_anchors"])...)
	if structuresRaw, ok := decoded["timeframe_structures"].([]interface{}); ok {
		for _, raw := range structuresRaw {
			item, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			tf := readString(item["timeframe"])
			for _, v := range readFloatSlice(item["support"]) {
				ctx.Support = append(ctx.Support, v)
			}
			for _, v := range readFloatSlice(item["resistance"]) {
				ctx.Resistance = append(ctx.Resistance, v)
			}
			if fib, ok := item["fibonacci"].(map[string]interface{}); ok {
				ctx.FibLevels = append(ctx.FibLevels, readFloatSlice(fib["levels"])...)
			}
			anchors := readDecisionAnchors(item["anchors"])
			for i := range anchors {
				if anchors[i].Timeframe == "" {
					anchors[i].Timeframe = tf
				}
			}
			ctx.Anchors = append(ctx.Anchors, anchors...)
		}
	}
	if ctx.PrimaryTimeframe == "" && len(ctx.Support) == 0 && len(ctx.Resistance) == 0 && len(ctx.FibLevels) == 0 && len(ctx.Anchors) == 0 && ctx.Entry == 0 && ctx.FirstTarget == 0 {
		return nil
	}
	return ctx
}

func readDecisionAnchors(value interface{}) []store.DecisionActionReasonAnchor {
	anchorsRaw, ok := value.([]interface{})
	if !ok {
		return nil
	}
	anchors := make([]store.DecisionActionReasonAnchor, 0, len(anchorsRaw))
	for _, raw := range anchorsRaw {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		anchors = append(anchors, store.DecisionActionReasonAnchor{
			Type:      readString(item["type"]),
			Timeframe: readString(item["timeframe"]),
			Price:     readFloat(item["price"]),
			Reason:    readString(item["reason"]),
		})
	}
	return anchors
}

func readString(value interface{}) string {
	str, _ := value.(string)
	return strings.TrimSpace(str)
}

func readStringSlice(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		if existing, ok := value.([]string); ok {
			return existing
		}
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			result = append(result, strings.TrimSpace(s))
		}
	}
	return result
}

func readFloat(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

func readFloatSlice(value interface{}) []float64 {
	items, ok := value.([]interface{})
	if !ok {
		if existing, ok := value.([]float64); ok {
			return existing
		}
		return nil
	}
	result := make([]float64, 0, len(items))
	for _, item := range items {
		if f := readFloat(item); f > 0 {
			result = append(result, f)
		}
	}
	return result
}

func (at *AutoTrader) supportsNativeTrailingStop() bool {
	caps := at.GetProtectionCapabilities()
	exchange := strings.ToLower(at.exchange)
	switch exchange {
	case "binance":
		return caps.SupportsAlgoOrders && caps.CanAmendProtection
	case "bitget":
		return caps.NativeStopLoss && caps.NativeTakeProfit && caps.NativePartialClose
	case "okx":
		return caps.SupportsAlgoOrders && caps.CanAmendProtection
	default:
		return false
	}
}

type nativeTrailingOrder struct {
	PositionSide string
	StopPrice    float64
	CallbackRate float64
	Quantity     float64
	OrderID      string
}

func (at *AutoTrader) findExistingFullTrailingOrder(side string, openOrders []OpenOrder) *nativeTrailingOrder {
	for _, order := range openOrders {
		if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
			continue
		}
		if !strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			continue
		}
		return &nativeTrailingOrder{
			PositionSide: order.PositionSide,
			StopPrice:    order.StopPrice,
			CallbackRate: order.CallbackRate,
			Quantity:     order.Quantity,
			OrderID:      order.OrderID,
		}
	}
	return nil
}

func (at *AutoTrader) findEquivalentPartialTrailingOrder(symbol, side string, rule store.DrawdownTakeProfitRule, entryPrice float64, openOrders []OpenOrder) (*nativeTrailingOrder, float64, float64, float64) {
	plannedActivationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
	plannedCallbackRate := calculateDrawdownRuleCallbackRatio(entryPrice, side, rule)
	qtyTarget := 0.0
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			ps, _ := pos["symbol"].(string)
			pd, _ := pos["side"].(string)
			if ps != symbol || !strings.EqualFold(pd, side) {
				continue
			}
			qtyTarget, _ = pos["positionAmt"].(float64)
			if qtyTarget < 0 {
				qtyTarget = -qtyTarget
			}
			break
		}
	}
	if qtyTarget > 0 {
		qtyTarget = qtyTarget * rule.CloseRatioPct / 100.0
		callbackTolerance := 0.0002
		qtyTolerance := math.Max(0.0001, qtyTarget*0.1)
		for _, order := range openOrders {
			if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
				continue
			}
			if !strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
				continue
			}
			if math.Abs(order.Quantity-qtyTarget) <= qtyTolerance && math.Abs(order.CallbackRate-plannedCallbackRate) <= callbackTolerance && activationMatches(order.StopPrice, plannedActivationPrice) {
				return &nativeTrailingOrder{
					PositionSide: order.PositionSide,
					StopPrice:    order.StopPrice,
					CallbackRate: order.CallbackRate,
					Quantity:     order.Quantity,
					OrderID:      order.OrderID,
				}, qtyTarget, plannedActivationPrice, plannedCallbackRate
			}
		}
	}
	return nil, qtyTarget, plannedActivationPrice, plannedCallbackRate
}

func activationMatches(actual, planned float64) bool {
	if actual <= 0 || planned <= 0 {
		return false
	}
	return math.Abs(actual-planned)/math.Max(math.Abs(actual), math.Abs(planned)) <= 0.003
}

func (at *AutoTrader) findPartialTrailingReplacementCandidate(side string, openOrders []OpenOrder, qtyTarget, plannedActivationPrice, plannedCallbackRate float64) *nativeTrailingOrder {
	var best *nativeTrailingOrder
	bestScore := math.MaxFloat64
	for _, order := range openOrders {
		if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
			continue
		}
		if !strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			continue
		}
		qtyScore := math.Abs(order.Quantity - qtyTarget)
		callbackScore := math.Abs(order.CallbackRate-plannedCallbackRate) * 100
		activationScore := 0.0
		if order.StopPrice > 0 && plannedActivationPrice > 0 {
			activationScore = math.Abs(order.StopPrice-plannedActivationPrice) / math.Max(math.Abs(order.StopPrice), math.Abs(plannedActivationPrice))
		}
		score := qtyScore + callbackScore + activationScore
		if best == nil || score < bestScore {
			bestScore = score
			best = &nativeTrailingOrder{
				PositionSide: order.PositionSide,
				StopPrice:    order.StopPrice,
				CallbackRate: order.CallbackRate,
				Quantity:     order.Quantity,
				OrderID:      order.OrderID,
			}
		}
	}
	return best
}

func (at *AutoTrader) shouldReplacePartialTrailingTier(existing *nativeTrailingOrder, plannedActivationPrice, plannedCallbackRate float64) bool {
	if existing == nil {
		return false
	}
	if existing.StopPrice <= 0 || plannedActivationPrice <= 0 {
		return false
	}
	activationDrift := math.Abs(existing.StopPrice-plannedActivationPrice) / math.Max(math.Abs(existing.StopPrice), math.Abs(plannedActivationPrice))
	callbackDrift := math.Abs(existing.CallbackRate - plannedCallbackRate)
	return activationDrift > 0.003 || callbackDrift > 0.0002
}

func findNewestMatchingTrailingOrderID(openOrders []OpenOrder, positionSide string, existingTier *nativeTrailingOrder, qtyTarget, plannedCallbackRate float64) string {
	for i := len(openOrders) - 1; i >= 0; i-- {
		order := openOrders[i]
		if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		if !strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			continue
		}
		if existingTier != nil && order.OrderID == existingTier.OrderID {
			continue
		}
		qtyTolerance := math.Max(0.0001, qtyTarget*0.1)
		if math.Abs(order.Quantity-qtyTarget) <= qtyTolerance && math.Abs(order.CallbackRate-plannedCallbackRate) <= 0.0002 {
			return order.OrderID
		}
	}
	return ""
}

func (at *AutoTrader) applyManagedDrawdownFallback(symbol, side string, entryPrice float64, rule store.DrawdownTakeProfitRule, activationPrice float64, callbackRatio float64) bool {
	if !at.verifyLivePositionForProtection(symbol, side, "managed drawdown fallback") {
		return false
	}
	if rule.CloseRatioPct <= 0 || entryPrice <= 0 {
		return false
	}
	positionSide := strings.ToUpper(side)
	positionAction := "open_" + strings.ToLower(side)
	if rule.CloseRatioPct < 99.999 {
		positions, err := at.trader.GetPositions()
		if err != nil {
			logger.Infof("❌ Managed partial drawdown fallback failed to fetch positions (%s %s): %v", symbol, side, err)
			return false
		}
		quantity := 0.0
		for _, pos := range positions {
			ps, _ := pos["symbol"].(string)
			pd, _ := pos["side"].(string)
			if ps != symbol || !strings.EqualFold(pd, side) {
				continue
			}
			quantity, _ = pos["positionAmt"].(float64)
			if quantity < 0 {
				quantity = -quantity
			}
			break
		}
		if quantity <= 0 {
			logger.Infof("❌ Managed partial drawdown fallback missing quantity (%s %s)", symbol, side)
			return false
		}
		candidate := buildManagedPartialDrawdownPlanCandidate(entryPrice, positionAction, rule)
		if candidate == nil || !at.canApplyManagedPartialDrawdownPlan(candidate) {
			return false
		}
		logger.Infof("🟣 Managed partial drawdown fallback armed after native rejection: %s %s | activation=%.6f callbackRatio=%.6f close=%.1f%%", symbol, side, activationPrice, callbackRatio, rule.CloseRatioPct)
		if err := at.placeAndVerifyProtectionPlanWithRetry(symbol, positionSide, quantity, candidate); err != nil {
			logger.Infof("❌ Managed partial drawdown fallback apply failed (%s %s): %v", symbol, side, err)
			return false
		}
		at.setProtectionState(symbol, side, "managed_partial_drawdown_armed")
		at.persistDynamicProtectionRecordWithDetails(symbol, side, "managed_drawdown", stableDrawdownRuleFingerprint(entryPrice, rule), rule.CloseRatioPct, "armed", "", activationPrice, callbackRatio, 0)
		return true
	}
	at.setProtectionState(symbol, side, "managed_drawdown_armed")
	at.persistDynamicProtectionRecordWithDetails(symbol, side, "managed_drawdown", stableDrawdownRuleFingerprint(entryPrice, rule), rule.CloseRatioPct, "armed", "", activationPrice, callbackRatio, 0)
	logger.Infof("🟣 Managed full drawdown fallback armed after native rejection: %s %s | activation=%.6f callbackRatio=%.6f close=100%%", symbol, side, activationPrice, callbackRatio)
	return true
}

func (at *AutoTrader) applyNativeTrailingDrawdown(symbol, side string, entryPrice float64, rule store.DrawdownTakeProfitRule) bool {
	if !at.supportsNativeTrailingStop() {
		return false
	}
	if !at.verifyLivePositionForProtection(symbol, side, "native trailing drawdown") {
		return false
	}
	currentState := at.getProtectionState(symbol, side)
	if currentState == "native_trailing_arming" || currentState == "native_partial_trailing_arming" {
		logger.Infof("🟣 Native trailing drawdown already arming, skipping duplicate apply (%s %s state=%s)", symbol, side, currentState)
		return true
	}
	isPartial := rule.CloseRatioPct < 99.999
	var staleFullTrailing *nativeTrailingOrder
	if currentState == "native_trailing_armed" || currentState == "native_partial_trailing_armed" {
		if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
			if !isPartial {
				plannedActivationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
				plannedCallbackRate := calculateDrawdownRuleCallbackRatio(entryPrice, side, rule)
				existing := at.findExistingFullTrailingOrder(side, openOrders)
				if existing != nil {
					if !at.shouldReplacePartialTrailingTier(existing, plannedActivationPrice, plannedCallbackRate) {
						return true
					}
					logger.Infof("⚠️ Native trailing state exists but full trailing order drifted from plan (%s %s), re-arming", symbol, side)
					staleFullTrailing = existing
				} else {
					logger.Infof("⚠️ Native trailing state exists but no trailing order found on exchange (%s %s), re-arming", symbol, side)
				}
			} else {
				existingTier, _, plannedActivationPrice, plannedCallbackRate := at.findEquivalentPartialTrailingOrder(symbol, side, rule, entryPrice, openOrders)
				if existingTier != nil && !at.shouldReplacePartialTrailingTier(existingTier, plannedActivationPrice, plannedCallbackRate) {
					if existingTier.StopPrice > 0 && plannedActivationPrice > 0 {
						logger.Infof("ℹ️ Native partial trailing tier already exists on exchange (%s %s close=%.1f%% activation=%.6f callback=%.6f)", symbol, side, rule.CloseRatioPct, existingTier.StopPrice, existingTier.CallbackRate)
					}
					return true
				}
			}
		}
	}
	// For partial close rules, check if exchange supports native partial close
	if isPartial {
		caps := at.GetProtectionCapabilities()
		if !caps.NativePartialClose {
			return false
		}
	}
	if entryPrice <= 0 || rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 {
		return false
	}

	plannedActivationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
	activationPrice := plannedActivationPrice
	if marketPrice, err := at.trader.GetMarketPrice(symbol); err == nil && marketPrice > 0 {
		marketReachedArmGate := false
		switch strings.ToLower(side) {
		case "long":
			marketReachedArmGate = marketPrice >= plannedActivationPrice
		case "short":
			marketReachedArmGate = marketPrice <= plannedActivationPrice
		}
		if marketReachedArmGate {
			activationPrice = marketPrice
		}
	} else if err != nil {
		logger.Infof("⚠️ Failed to get latest market price for trailing activation (%s %s): %v", symbol, side, err)
	}
	priceBasedCallbackRatio := calculateDrawdownRuleCallbackRatio(entryPrice, side, rule)
	if activationPrice <= 0 || priceBasedCallbackRatio <= 0 {
		return false
	}

	logger.Infof("🎯 Trailing activation resolved: %s %s | activation=%.6f planned=%.6f callbackRatio=%.6f rule=minProfit=%.4f maxDrawdown=%.4f close=%.2f%% stage=%s",
		symbol, side, activationPrice, plannedActivationPrice, priceBasedCallbackRatio, rule.MinProfitPct, rule.MaxDrawdownPct, rule.CloseRatioPct, rule.StageName)
	callbackAdjustment, err := adjustNativeDrawdownCallbackRatio(entryPrice, side, rule, priceBasedCallbackRatio)
	if err != nil {
		logger.Warnf("❌ Native trailing drawdown rejected by safety policy (%s %s): %v", symbol, side, err)
		var nativeReject nativeDrawdownRejection
		if errors.As(err, &nativeReject) {
			return at.applyManagedDrawdownFallback(symbol, side, entryPrice, rule, activationPrice, priceBasedCallbackRatio)
		}
		return false
	}
	if callbackAdjustment.Adjusted {
		logger.Warnf("🛠 Native trailing drawdown callback adjusted (%s %s): %.6f -> %.6f reason=%s",
			symbol, side, priceBasedCallbackRatio, callbackAdjustment.CallbackRatio, callbackAdjustment.Reason)
		priceBasedCallbackRatio = callbackAdjustment.CallbackRatio
	}

	positionSide := strings.ToUpper(side)
	positionAction := "open_" + strings.ToLower(side)
	exchange := strings.ToLower(at.exchange)
	armingState := "native_trailing_arming"
	if isPartial {
		armingState = "native_partial_trailing_arming"
	}
	claimed, previousProtectionState, actualProtectionState := at.claimProtectionArmingState(symbol, side, currentState, armingState)
	if !claimed {
		if isNativeTrailingArmingState(actualProtectionState) {
			logger.Infof("🟣 Native trailing drawdown already arming, skipping duplicate apply (%s %s state=%s)", symbol, side, actualProtectionState)
		} else {
			logger.Infof("🟣 Native trailing drawdown state changed during prepare, skipping duplicate apply (%s %s expected=%s actual=%s)", symbol, side, currentState, actualProtectionState)
		}
		return true
	}
	defer func() {
		if at.getProtectionState(symbol, side) != armingState {
			return
		}
		if previousProtectionState == "" {
			at.clearProtectionState(symbol, side)
			return
		}
		at.setProtectionState(symbol, side, previousProtectionState)
	}()

	if isPartial {
		positions, err := at.trader.GetPositions()
		if err != nil {
			logger.Infof("❌ Partial drawdown failed to fetch positions (%s %s): %v", symbol, side, err)
			return false
		}
		var quantity float64
		for _, pos := range positions {
			ps, _ := pos["symbol"].(string)
			pd, _ := pos["side"].(string)
			if ps != symbol || !strings.EqualFold(pd, side) {
				continue
			}
			quantity, _ = pos["positionAmt"].(float64)
			if quantity < 0 {
				quantity = -quantity
			}
			break
		}
		if quantity <= 0 {
			logger.Infof("❌ Partial drawdown missing quantity (%s %s)", symbol, side)
			return false
		}

		partialQty := quantity * rule.CloseRatioPct / 100.0
		if partialQty <= 0 {
			return false
		}

		caps := at.GetProtectionCapabilities()
		if caps.SupportsNativePartialTrailing {
			switch exchange {
			case "binance":
				binanceTrader, ok := at.trader.(interface {
					SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error
					CancelTrailingStopOrders(symbol string) error
				})
				if ok {
					binanceCallbackPercent := priceBasedCallbackRatio * 100.0
					if binanceCallbackPercent < 0.1 {
						binanceCallbackPercent = 0.1
					}
					if binanceCallbackPercent > 10 {
						binanceCallbackPercent = 10
					}
					if err := binanceTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, binanceCallbackPercent, partialQty); err == nil {
						at.setProtectionState(symbol, side, "native_partial_trailing_armed")
						logger.Infof("🟣 Native partial trailing drawdown armed: %s %s | activation=%.6f callback=%.4f close=%.1f%% qty=%.4f", symbol, side, activationPrice, binanceCallbackPercent, rule.CloseRatioPct, partialQty)
						at.cancelImmediateTrailing(symbol, side)
						return true
					} else {
						logger.Infof("❌ Native partial trailing drawdown apply failed (%s %s, binance): %v", symbol, side, err)
					}
				}
			case "bitget":
				bitgetTrader, ok := at.trader.(interface {
					SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error
					CancelTrailingStopOrders(symbol string) error
				})
				if ok {
					bitgetCallbackPercent := priceBasedCallbackRatio * 100.0
					if bitgetCallbackPercent < 0.1 {
						bitgetCallbackPercent = 0.1
					}
					if bitgetCallbackPercent > 10 {
						bitgetCallbackPercent = 10
					}
					if err := bitgetTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, bitgetCallbackPercent, partialQty); err == nil {
						at.setProtectionState(symbol, side, "native_partial_trailing_armed")
						logger.Infof("🟣 Native partial trailing drawdown armed: %s %s | activation=%.6f callback=%.4f close=%.1f%% qty=%.4f", symbol, side, activationPrice, bitgetCallbackPercent, rule.CloseRatioPct, partialQty)
						at.cancelImmediateTrailing(symbol, side)
						return true
					} else {
						logger.Infof("❌ Native partial trailing drawdown apply failed (%s %s, bitget): %v", symbol, side, err)
					}
				}
			case "okx":
				okxTrader, ok := at.trader.(interface {
					SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error
					CancelTrailingStopOrders(symbol string) error
				})
				if ok {
					okxCallbackRatio := priceBasedCallbackRatio
					if okxCallbackRatio < 0.001 {
						okxCallbackRatio = 0.001
					}
					if okxCallbackRatio > 1 {
						okxCallbackRatio = 1
					}
					var existingTier *nativeTrailingOrder
					oldTrailingCount := 0
					qtyTarget := partialQty
					plannedCallbackRate := okxCallbackRatio
					if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
						var plannedActivationPrice float64
						existingTier, qtyTarget, plannedActivationPrice, plannedCallbackRate = at.findEquivalentPartialTrailingOrder(symbol, side, rule, entryPrice, openOrders)
						if existingTier == nil {
							existingTier = at.findPartialTrailingReplacementCandidate(side, openOrders, qtyTarget, plannedActivationPrice, plannedCallbackRate)
						}
						for _, order := range openOrders {
							if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
								continue
							}
							if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
								oldTrailingCount++
							}
						}
					}
					if tagged, ok := at.trader.(interface {
						SetTrailingStopLossTaggedWithID(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64, reasonTag string) (string, error)
						CancelTrailingStopOrdersByIDs(symbol string, orderIDs []string) error
					}); ok {
						placedOrderID, err := tagged.SetTrailingStopLossTaggedWithID(symbol, positionSide, activationPrice, okxCallbackRatio, partialQty, "native_trailing")
						if err == nil {
							verified := false
							for attempt := 1; !verified && attempt <= protectionVerifyMaxAttempts; attempt++ {
								at.sleepForVerification(protectionVerifyDelay)
								openOrders, err := at.trader.GetOpenOrders(symbol)
								if err == nil {
									trailingCount := 0
									_, qtyTarget, _, plannedCallbackRate := at.findEquivalentPartialTrailingOrder(symbol, side, rule, entryPrice, openOrders)
									for _, order := range openOrders {
										if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
											continue
										}
										if !strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
											continue
										}
										trailingCount++
										if existingTier != nil && order.OrderID == existingTier.OrderID {
											continue
										}
										qtyTolerance := math.Max(0.0001, qtyTarget*0.1)
										if math.Abs(order.Quantity-qtyTarget) <= qtyTolerance && math.Abs(order.CallbackRate-plannedCallbackRate) <= 0.0002 {
											verified = true
											break
										}
									}
									if !verified && trailingCount > oldTrailingCount {
										verified = true
									}
									if verified {
										break
									}
								}
							}
							if verified {
								newOrderID := placedOrderID
								if newOrderID == "" {
									if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
										newOrderID = findNewestMatchingTrailingOrderID(openOrders, strings.ToUpper(side), existingTier, qtyTarget, plannedCallbackRate)
									}
								}
								if existingTier != nil && existingTier.OrderID != "" {
									if err := tagged.CancelTrailingStopOrdersByIDs(symbol, []string{existingTier.OrderID}); err != nil {
										logger.Infof("⚠️ Failed to cancel replaced native partial trailing tier (%s %s, okx): %v", symbol, side, err)
									}
								}
								at.setProtectionState(symbol, side, "native_partial_trailing_armed")
								at.persistDynamicProtectionRecordWithDetails(symbol, side, "native_partial_trailing", stableDrawdownRuleFingerprint(entryPrice, rule), rule.CloseRatioPct, "armed", newOrderID, activationPrice, okxCallbackRatio, partialQty)
								logger.Infof("🟣 Native partial trailing drawdown armed: %s %s | activation=%.6f callback=%.6f close=%.1f%% qty=%.4f", symbol, side, activationPrice, okxCallbackRatio, rule.CloseRatioPct, partialQty)
								at.cancelImmediateTrailing(symbol, side)
								return true
							}
							logger.Infof("❌ Native partial trailing drawdown verify failed (%s %s, okx): new tier not visible after placement", symbol, side)
						} else {
							logger.Infof("❌ Native partial trailing drawdown apply failed (%s %s, okx): %v", symbol, side, err)
						}
					} else if err := okxTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, okxCallbackRatio, partialQty); err == nil {
						at.setProtectionState(symbol, side, "native_partial_trailing_armed")
						logger.Infof("🟣 Native partial trailing drawdown armed: %s %s | activation=%.6f callback=%.6f close=%.1f%% qty=%.4f", symbol, side, activationPrice, okxCallbackRatio, rule.CloseRatioPct, partialQty)
						at.cancelImmediateTrailing(symbol, side)
						return true
					} else {
						logger.Infof("❌ Native partial trailing drawdown apply failed (%s %s, okx): %v", symbol, side, err)
					}
				}
			}
			// Native-partial-capable exchanges should not silently fall back to managed TP here.
			return false
		}

		candidate := buildManagedPartialDrawdownPlanCandidate(entryPrice, positionAction, rule)
		if candidate == nil || !at.canApplyManagedPartialDrawdownPlan(candidate) {
			return false
		}
		logger.Infof("🟣 Managed partial drawdown armed: %s %s | activation=%.6f callbackRatio=%.6f close=%.1f%%",
			symbol, side, activationPrice, priceBasedCallbackRatio, rule.CloseRatioPct)
		if err := at.placeAndVerifyProtectionPlanWithRetry(symbol, positionSide, quantity, candidate); err != nil {
			logger.Infof("❌ Managed partial drawdown apply failed (%s %s): %v", symbol, side, err)
			return false
		}
		at.setProtectionState(symbol, side, "managed_partial_drawdown_armed")
		return true
	}

	if !isPartial && currentState == "native_partial_trailing_armed" {
		if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
			ids := make([]string, 0)
			for _, order := range openOrders {
				if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
					continue
				}
				if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
					ids = append(ids, order.OrderID)
				}
			}
			if len(ids) > 0 {
				if tagged, ok := at.trader.(interface {
					CancelTrailingStopOrdersByIDs(symbol string, orderIDs []string) error
				}); ok {
					if err := tagged.CancelTrailingStopOrdersByIDs(symbol, ids); err != nil {
						logger.Infof("⚠️ Failed to collapse old native partial trailing tiers before full-tier migration (%s %s): %v", symbol, side, err)
					} else {
						logger.Infof("🧹 Collapsed %d old native partial trailing tier(s) before full-tier migration: %s %s", len(ids), symbol, side)
					}
				}
			}
		}
	}

	placedOrderID := ""
	switch exchange {
	case "binance":
		binanceTrader, ok := at.trader.(interface {
			SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error
			CancelTrailingStopOrders(symbol string) error
		})
		if !ok {
			return false
		}
		binanceCallbackPercent := priceBasedCallbackRatio * 100.0
		if binanceCallbackPercent < 0.1 {
			binanceCallbackPercent = 0.1
		}
		if binanceCallbackPercent > 10 {
			binanceCallbackPercent = 10
		}
		if err := binanceTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, binanceCallbackPercent, 0); err != nil {
			logger.Infof("❌ Native trailing drawdown apply failed (%s %s): %v", symbol, side, err)
			return false
		}
	case "bitget":
		bitgetTrader, ok := at.trader.(interface {
			SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error
			CancelTrailingStopOrders(symbol string) error
		})
		if !ok {
			return false
		}
		bitgetCallbackPercent := priceBasedCallbackRatio * 100.0
		if bitgetCallbackPercent < 0.1 {
			bitgetCallbackPercent = 0.1
		}
		if bitgetCallbackPercent > 10 {
			bitgetCallbackPercent = 10
		}
		if err := bitgetTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, bitgetCallbackPercent, 0); err != nil {
			logger.Infof("❌ Native trailing drawdown apply failed (%s %s): %v", symbol, side, err)
			return false
		}
	case "okx":
		okxTrader, ok := at.trader.(interface {
			SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error
			CancelTrailingStopOrders(symbol string) error
		})
		if !ok {
			return false
		}
		okxCallbackRatio := priceBasedCallbackRatio
		if okxCallbackRatio < 0.001 {
			okxCallbackRatio = 0.001
		}
		if okxCallbackRatio > 1 {
			okxCallbackRatio = 1
		}
		if tagged, ok := at.trader.(interface {
			SetTrailingStopLossTaggedWithID(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64, reasonTag string) (string, error)
			CancelTrailingStopOrdersByIDs(symbol string, orderIDs []string) error
		}); ok {
			var err error
			placedOrderID, err = tagged.SetTrailingStopLossTaggedWithID(symbol, positionSide, activationPrice, okxCallbackRatio, 0, "native_trailing")
			if err != nil {
				logger.Infof("❌ Native trailing drawdown apply failed (%s %s): %v", symbol, side, err)
				return false
			}
			if staleFullTrailing != nil && staleFullTrailing.OrderID != "" {
				if err := tagged.CancelTrailingStopOrdersByIDs(symbol, []string{staleFullTrailing.OrderID}); err != nil {
					logger.Infof("⚠️ Failed to cancel replaced native full trailing order (%s %s, okx): %v", symbol, side, err)
				}
			}
		} else if err := okxTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, okxCallbackRatio, 0); err != nil {
			logger.Infof("❌ Native trailing drawdown apply failed (%s %s): %v", symbol, side, err)
			return false
		}
	default:
		return false
	}

	if isPartial {
		at.setProtectionState(symbol, side, "managed_partial_drawdown_armed")
		logger.Infof("🟣 Managed partial drawdown armed: %s %s | activation=%.6f callbackRatio=%.6f close=%.1f%%", symbol, side, activationPrice, priceBasedCallbackRatio, rule.CloseRatioPct)
	} else {
		at.setProtectionState(symbol, side, "native_trailing_armed")
		at.persistDynamicProtectionRecordWithDetails(symbol, side, "native_trailing", stableDrawdownRuleFingerprint(entryPrice, rule), rule.CloseRatioPct, "armed", placedOrderID, activationPrice, priceBasedCallbackRatio, 0)
		logger.Infof("🟣 Native trailing drawdown armed: %s %s | activation=%.6f callbackRatio=%.6f", symbol, side, activationPrice, priceBasedCallbackRatio)
	}
	at.cancelImmediateTrailing(symbol, side)
	return true
}

func (at *AutoTrader) matchDrawdownArmRule(currentPnLPct float64, rules []store.DrawdownTakeProfitRule) *store.DrawdownTakeProfitRule {
	var matched *store.DrawdownTakeProfitRule
	for i := range rules {
		rule := normalizeDrawdownRule(rules[i])
		if currentPnLPct < rule.MinProfitPct {
			continue
		}
		if matched == nil || rule.MinProfitPct > matched.MinProfitPct {
			matched = &rule
		}
	}
	return matched
}

func (at *AutoTrader) matchDrawdownRule(currentPnLPct, drawdownPct float64, rules []store.DrawdownTakeProfitRule) *store.DrawdownTakeProfitRule {
	var matched *store.DrawdownTakeProfitRule
	for i := range rules {
		rule := rules[i]
		if currentPnLPct < rule.MinProfitPct || !isDrawdownThresholdMet(currentPnLPct, drawdownPct, rule) {
			continue
		}
		if matched == nil || rule.MinProfitPct > matched.MinProfitPct ||
			(rule.MinProfitPct == matched.MinProfitPct && rule.MaxDrawdownPct > matched.MaxDrawdownPct) {
			matched = &rule
		}
	}
	return matched
}

func (at *AutoTrader) getTriggeredDrawdownRules(currentPnLPct, drawdownPct float64, rules []store.DrawdownTakeProfitRule) []store.DrawdownTakeProfitRule {
	matched := make([]store.DrawdownTakeProfitRule, 0, len(rules))
	for _, rule := range rules {
		if currentPnLPct < rule.MinProfitPct || !isDrawdownThresholdMet(currentPnLPct, drawdownPct, rule) {
			continue
		}
		matched = append(matched, rule)
	}
	return matched
}

func (at *AutoTrader) getDrawdownConfigSource(symbol, side string) string {
	at.protectionStateMutex.RLock()
	defer at.protectionStateMutex.RUnlock()
	if symbol != "" || side != "" {
		key := positionKey(symbol, side)
		if src, ok := at.drawdownSource[key]; ok && src != "" {
			return src
		}
		if len(at.drawdownAIRules[key]) > 0 {
			return "ai_decision"
		}
	}
	for _, src := range at.drawdownSource {
		if src == "ai_decision" {
			return src
		}
	}
	if len(at.getActiveDrawdownRules()) == 0 {
		return "none"
	}
	return "strategy"
}

func (at *AutoTrader) getBreakEvenConfigSource(symbol, side string) string {
	at.breakEvenStateMutex.RLock()
	defer at.breakEvenStateMutex.RUnlock()
	if symbol != "" || side != "" {
		if src, ok := at.breakEvenSource[positionKey(symbol, side)]; ok && src != "" {
			return src
		}
	}
	for _, src := range at.breakEvenSource {
		if src == "ai_decision" {
			return src
		}
	}
	return "strategy"
}

func (at *AutoTrader) getActiveBreakEvenConfigForPlan(plan *ProtectionPlan) *store.BreakEvenStopConfig {
	if at != nil && at.config.StrategyConfig != nil && at.config.StrategyConfig.Protection.BreakEvenStop.Mode == store.ProtectionModeAI {
		if plan != nil && plan.BreakEvenConfig != nil {
			cfg := *plan.BreakEvenConfig
			if cfg.Enabled && cfg.TriggerValue > 0 {
				if cfg.OffsetPct < 0 {
					cfg.OffsetPct = 0
				}
				return &cfg
			}
		}
		return at.getActiveBreakEvenConfig()
	}
	return at.getActiveBreakEvenConfig()
}

func (at *AutoTrader) getActiveBreakEvenConfig() *store.BreakEvenStopConfig {
	rules := at.getActiveBreakEvenRules()
	if len(rules) == 0 || at == nil || at.config.StrategyConfig == nil {
		return nil
	}
	cfg := at.config.StrategyConfig.Protection.BreakEvenStop
	cfg.TriggerMode = rules[0].TriggerMode
	cfg.TriggerValue = rules[0].TriggerValue
	cfg.OffsetPct = rules[0].OffsetPct
	return &cfg
}

func (at *AutoTrader) getActiveBreakEvenRules() []store.BreakEvenStopRule {
	if at == nil || at.config.StrategyConfig == nil {
		return nil
	}
	cfg := at.config.StrategyConfig.Protection.BreakEvenStop
	if !cfg.Enabled {
		return nil
	}
	rules := cfg.Rules
	if len(rules) == 0 && cfg.TriggerValue > 0 {
		rules = []store.BreakEvenStopRule{{TriggerMode: cfg.TriggerMode, TriggerValue: cfg.TriggerValue, OffsetPct: cfg.OffsetPct, CloseRatioPct: 100, StageName: "BE1"}}
	}
	out := make([]store.BreakEvenStopRule, 0, len(rules))
	for _, rule := range rules {
		if rule.TriggerMode == "" {
			rule.TriggerMode = cfg.TriggerMode
		}
		if rule.TriggerMode != store.BreakEvenTriggerProfitPct || rule.TriggerValue <= 0 {
			continue
		}
		if rule.OffsetPct < 0 {
			rule.OffsetPct = 0
		}
		if rule.CloseRatioPct <= 0 {
			rule.CloseRatioPct = 100
		}
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TriggerValue < out[j].TriggerValue })
	return out
}

func (at *AutoTrader) applyBreakEvenStops(symbol, side string, quantity, entryPrice, currentPnLPct float64, rules []store.BreakEvenStopRule) error {
	if len(rules) == 0 {
		return nil
	}
	// Find the highest satisfied tier — later tiers have higher offset (better protection).
	// Only apply that one to avoid redundant exchange API calls.
	bestIdx := -1
	for idx, rule := range rules {
		if currentPnLPct >= rule.TriggerValue {
			bestIdx = idx
		}
	}
	if bestIdx < 0 {
		return nil
	}
	rule := rules[bestIdx]
	if rule.CloseRatioPct <= 0 {
		rule.CloseRatioPct = 100
	}
	ruleQty := quantity * rule.CloseRatioPct / 100.0
	if ruleQty <= 0 {
		return nil
	}
	cfg := store.BreakEvenStopConfig{Enabled: true, Mode: store.ProtectionModeManual, TriggerMode: rule.TriggerMode, TriggerValue: rule.TriggerValue, OffsetPct: rule.OffsetPct}
	stage := rule.StageName
	if stage == "" {
		stage = fmt.Sprintf("BE%d", bestIdx+1)
	}
	if err := at.applyBreakEvenStop(symbol, side, ruleQty, entryPrice, currentPnLPct, cfg, stage); err != nil {
		return err
	}
	at.setBreakEvenState(symbol, side, "armed")
	return nil
}

func (at *AutoTrader) applyBreakEvenStop(symbol, side string, quantity, entryPrice, currentPnLPct float64, cfg store.BreakEvenStopConfig, stageName ...string) error {
	if currentPnLPct < cfg.TriggerValue || entryPrice <= 0 || quantity <= 0 {
		return nil
	}
	if !at.verifyLivePositionForProtection(symbol, side, "break-even stop") {
		return nil
	}

	caps := at.GetProtectionCapabilities()
	if !caps.NativeStopLoss {
		return fmt.Errorf("exchange %s does not support native stop loss for break-even", at.exchange)
	}

	stage := "BE"
	if len(stageName) > 0 && stageName[0] != "" {
		stage = stageName[0]
	}
	breakEvenPrice := calculateBreakEvenStopPrice(side, entryPrice, cfg.OffsetPct)
	if breakEvenPrice <= 0 {
		return fmt.Errorf("invalid break-even stop price calculated for %s %s", symbol, side)
	}

	positionSide := strings.ToUpper(side)
	// If a matching break-even stop is already live (for example after a restart
	// before local BE state has been restored), treat it as armed instead of
	// placing another native stop.
	if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
		if hasMatchingProtectionOrder(openOrders, positionSide, false, breakEvenPrice) {
			logger.Infof("🟠 Break-even stop already live: %s %s | stop=%.6f", symbol, side, breakEvenPrice)
			at.persistDynamicProtectionRecordWithDetails(symbol, side, "break_even_stop", fmt.Sprintf("%.8f|%.8f|%.4f|%.4f|%s", entryPrice, quantity, cfg.TriggerValue, cfg.OffsetPct, stage), 0, "armed", "", breakEvenPrice, 0, quantity)
			return nil
		}
	} else {
		logger.Warnf("⚠️ Break-even live-order precheck failed (%s %s): %v", symbol, side, err)
	}

	// Break-even stop is managed independently. Do not cancel existing ladder/full stop-loss
	// orders here, otherwise we destroy the long-term stop-loss protection stack.
	// If exchanges later support per-order tags / amend-by-id, we can target only prior
	// break-even stops. For now, preserve existing SL orders and add break-even separately.
	if okxTrader, ok := at.trader.(interface {
		SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error
		SetStopLossTagged(symbol string, positionSide string, quantity, stopPrice float64, reasonTag string) error
	}); ok {
		if err := okxTrader.SetStopLossTagged(symbol, positionSide, quantity, breakEvenPrice, "break_even_stop"); err != nil {
			return fmt.Errorf("failed to set break-even stop loss: %w", err)
		}
	} else if err := at.trader.SetStopLoss(symbol, positionSide, quantity, breakEvenPrice); err != nil {
		return fmt.Errorf("failed to set break-even stop loss: %w", err)
	}

	var verified bool
	for attempt := 1; attempt <= protectionVerifyMaxAttempts; attempt++ {
		at.sleepForVerification(protectionVerifyDelay)
		openOrders, err := at.trader.GetOpenOrders(symbol)
		if err != nil {
			return fmt.Errorf("failed to verify break-even stop loss: %w", err)
		}
		if hasMatchingProtectionOrder(openOrders, positionSide, false, breakEvenPrice) {
			verified = true
			logger.Infof("✅ Break-even stop verified: %s %s | stop=%.6f (attempt %d/%d)",
				symbol, side, breakEvenPrice, attempt, protectionVerifyMaxAttempts)
			break
		}
		if attempt < protectionVerifyMaxAttempts {
			logger.Infof("⏳ Break-even verification pending (attempt %d/%d), retrying...", attempt, protectionVerifyMaxAttempts)
		}
	}
	if !verified {
		return fmt.Errorf("break-even stop verification failed for %s %s at %.6f after %d attempts", symbol, side, breakEvenPrice, protectionVerifyMaxAttempts)
	}

	logger.Infof("🟠 Break-even stop applied: %s %s | stage=%s trigger=%.2f%% current=%.2f%% qty=%.6f stop=%.6f",
		symbol, side, stage, cfg.TriggerValue, currentPnLPct, quantity, breakEvenPrice)
	at.persistDynamicProtectionRecordWithDetails(symbol, side, "break_even_stop", fmt.Sprintf("%.8f|%.8f|%.4f|%.4f|%s", entryPrice, quantity, cfg.TriggerValue, cfg.OffsetPct, stage), 0, "armed", "", breakEvenPrice, 0, quantity)
	return nil
}

func calculateBreakEvenStopPrice(side string, entryPrice, offsetPct float64) float64 {
	if entryPrice <= 0 {
		return 0
	}
	move := offsetPct / 100.0
	switch strings.ToLower(side) {
	case "long":
		return entryPrice * (1 + move)
	case "short":
		return entryPrice * (1 - move)
	default:
		return 0
	}
}

func (at *AutoTrader) closePositionBySide(symbol, side string, quantity float64) error {
	return at.closePositionByReason(symbol, side, quantity, "close_by_side")
}

func (at *AutoTrader) closePositionByReason(symbol, side string, quantity float64, closeReason string) error {
	type taggedCloser interface {
		CloseLongTagged(symbol string, quantity float64, reasonTag string) (map[string]interface{}, error)
		CloseShortTagged(symbol string, quantity float64, reasonTag string) (map[string]interface{}, error)
	}

	switch strings.ToLower(side) {
	case "long":
		var (
			order map[string]interface{}
			err   error
		)
		if tagged, ok := at.trader.(taggedCloser); ok && closeReason != "" {
			order, err = tagged.CloseLongTagged(symbol, quantity, closeReason)
		} else {
			order, err = at.trader.CloseLong(symbol, quantity)
		}
		if err != nil {
			return err
		}
		logger.Infof("✅ Close long position succeeded, order ID: %v", order["orderId"])
		at.persistCloseReasonFromOrderResult(order, closeReason)
	case "short":
		var (
			order map[string]interface{}
			err   error
		)
		if tagged, ok := at.trader.(taggedCloser); ok && closeReason != "" {
			order, err = tagged.CloseShortTagged(symbol, quantity, closeReason)
		} else {
			order, err = at.trader.CloseShort(symbol, quantity)
		}
		if err != nil {
			return err
		}
		logger.Infof("✅ Close short position succeeded, order ID: %v", order["orderId"])
		at.persistCloseReasonFromOrderResult(order, closeReason)
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	return nil
}

func (at *AutoTrader) persistCloseReasonFromOrderResult(order map[string]interface{}, closeReason string) {
	if at.store == nil || closeReason == "" || order == nil {
		return
	}
	var orderID string
	switch v := order["orderId"].(type) {
	case int64:
		orderID = fmt.Sprintf("%d", v)
	case float64:
		orderID = fmt.Sprintf("%.0f", v)
	case string:
		orderID = v
	default:
		orderID = fmt.Sprintf("%v", v)
	}
	if orderID == "" || orderID == "<nil>" {
		return
	}
	_ = at.store.Position().UpdateCloseReasonByExitOrderID(at.id, orderID, closeReason)
	_ = at.store.PositionClose().UpdateReasonByOrderID(at.id, orderID, closeReason, closeReason)
}

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	return at.closePositionByReason(symbol, side, 0, "emergency_protection_close")
}

// GetPeakPnLCache gets peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return a copy of the cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL updates peak profit cache
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// Update peak (if long, take larger value; if short, currentPnLPct is negative, also compare)
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// First time recording
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache clears peak cache for specified position
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}

// ============================================================================
// Risk Control Helpers
// ============================================================================

// isBTCETH checks if a symbol is BTC or ETH
func isBTCETH(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

// enforcePositionValueRatio checks and enforces position value ratio limits (CODE ENFORCED)
// Returns the adjusted position size (capped if necessary) and whether the position was capped
// positionSizeUSD: the original position size in USD
// equity: the account equity
// symbol: the trading symbol
func (at *AutoTrader) enforcePositionValueRatio(positionSizeUSD float64, equity float64, symbol string) (float64, bool) {
	if at.config.StrategyConfig == nil {
		return positionSizeUSD, false
	}

	riskControl := at.config.StrategyConfig.RiskControl

	// Get the appropriate position value ratio limit
	var maxPositionValueRatio float64
	if isBTCETH(symbol) {
		maxPositionValueRatio = riskControl.BTCETHMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 5.0 // Default: 5x for BTC/ETH
		}
	} else {
		maxPositionValueRatio = riskControl.AltcoinMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 1.0 // Default: 1x for altcoins
		}
	}

	// Calculate max allowed position value = equity × ratio
	maxPositionValue := equity * maxPositionValueRatio

	// Check if position size exceeds limit
	if positionSizeUSD > maxPositionValue {
		logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f USDT exceeds limit (equity %.2f × %.1fx = %.2f USDT max for %s), capping",
			positionSizeUSD, equity, maxPositionValueRatio, maxPositionValue, symbol)
		return maxPositionValue, true
	}

	return positionSizeUSD, false
}

// enforceMinPositionSize checks minimum position size (CODE ENFORCED)
func (at *AutoTrader) enforceMinPositionSize(positionSizeUSD float64, symbol ...string) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = 12 // Default: 12 USDT
	}
	if len(symbol) > 0 && strings.TrimSpace(symbol[0]) != "" {
		if snap := at.collectExecutionConstraintsSnapshot(symbol[0]); snap != nil {
			minSize = snap.ExecutableMinPositionUSD(minSize)
		}
	}

	if positionSizeUSD < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum executable size (%.2f USDT)", positionSizeUSD, minSize)
	}
	return nil
}

// enforceMaxPositions checks maximum positions count (CODE ENFORCED)
func (at *AutoTrader) enforceMaxPositions(currentPositionCount int) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	maxPositions := at.config.StrategyConfig.RiskControl.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3 // Default: 3 positions
	}

	if currentPositionCount >= maxPositions {
		return fmt.Errorf("❌ [RISK CONTROL] Already at max positions (%d/%d)", currentPositionCount, maxPositions)
	}
	return nil
}

// getSideFromAction converts order action to side (BUY/SELL)
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}

func isDrawdownThresholdMet(currentPnLPct, drawdownPct float64, rule store.DrawdownTakeProfitRule) bool {
	rule = normalizeDrawdownRule(rule)
	if rule.MaxDrawdownAbsPct > 0 {
		return currentPnLPct <= rule.MinProfitPct-rule.MaxDrawdownAbsPct
	}
	return drawdownPct >= rule.MaxDrawdownPct
}

func calculateProfitBasedTrailingTriggerPrice(entryPrice float64, side string, minProfitPct float64) float64 {
	if entryPrice <= 0 || minProfitPct <= 0 {
		return 0
	}
	move := minProfitPct / 100.0
	switch strings.ToLower(side) {
	case "long":
		return entryPrice * (1 + move)
	case "short":
		return entryPrice * (1 - move)
	default:
		return 0
	}
}

// calculateProfitBasedTrailingCallbackRatio converts a drawdown-on-profit rule into the exchange-native
// trailing callback ratio relative to price, not relative to total entry value.
//
// Example LONG:
// - entry = 10.0
// - minProfitPct = 3.0 => activation at 10.3
// - maxDrawdownPct = 40 => allow 40% giveback of profit (0.3 * 40% = 0.12)
// - stop target = 10.18
// - callback ratio at activation = 0.12 / 10.3 = 0.011650...
//
// Returns ratio in decimal form for OKX (0.001..1), and can be converted to percent for other exchanges.
func calculateProfitBasedTrailingCallbackRatio(entryPrice float64, side string, minProfitPct float64, maxDrawdownPct float64) float64 {
	activationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, minProfitPct)
	if entryPrice <= 0 || activationPrice <= 0 || maxDrawdownPct <= 0 {
		return 0
	}
	profitMoveAbs := math.Abs(activationPrice - entryPrice)
	allowedGivebackAbs := profitMoveAbs * (maxDrawdownPct / 100.0)
	if allowedGivebackAbs <= 0 {
		return 0
	}
	return allowedGivebackAbs / activationPrice
}

func calculateAbsoluteProfitDrawdownCallbackRatio(entryPrice float64, side string, minProfitPct float64, absDrawdownPct float64) float64 {
	activationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, minProfitPct)
	if entryPrice <= 0 || activationPrice <= 0 || absDrawdownPct <= 0 {
		return 0
	}
	allowedGivebackAbs := entryPrice * absDrawdownPct / 100.0
	if allowedGivebackAbs <= 0 {
		return 0
	}
	return allowedGivebackAbs / activationPrice
}

func calculateDrawdownRuleCallbackRatio(entryPrice float64, side string, rule store.DrawdownTakeProfitRule) float64 {
	rule = normalizeDrawdownRule(rule)
	if rule.MaxDrawdownAbsPct > 0 {
		return calculateAbsoluteProfitDrawdownCallbackRatio(entryPrice, side, rule.MinProfitPct, rule.MaxDrawdownAbsPct)
	}
	return calculateProfitBasedTrailingCallbackRatio(entryPrice, side, rule.MinProfitPct, rule.MaxDrawdownPct)
}
