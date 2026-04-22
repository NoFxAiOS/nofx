package trader

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/store"
	"strings"
	"time"
)

// startDrawdownMonitor 启动运行态回撤监控协程。
// 这条链路属于持仓后的风险保护，和开仓前的 AI 决策风控不同，
// 它的职责是在仓位已存在时继续兜底处理利润回撤与异常退出。
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
	if !drawdown.Enabled || len(drawdown.Rules) == 0 {
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

	rules := at.getActiveDrawdownRules()
	if len(rules) == 0 {
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

		if fingerprintChanged := at.refreshDrawdownExecutionFingerprint(symbol, side, entryPrice, quantity); fingerprintChanged {
			logger.Infof("🟠 Drawdown monitor: %s %s drawdown fingerprint changed, clearing previous execution guard", symbol, side)
		}

		matchedBreakEven := at.getActiveBreakEvenConfig()
		if matchedBreakEven != nil {
			if at.isBreakEvenSuppressedByRunner(symbol, side) {
				logger.Infof("🟠 Break-even monitor: %s %s suppressed by runner semantics, skipping mechanical BE apply", symbol, side)
			} else {
				beState := at.getBreakEvenState(symbol, side)
				if beState == "armed" || beState == "arming" {
					logger.Infof("🟠 Break-even monitor: %s %s already %s, skipping duplicate apply", symbol, side, beState)
				} else {
					if err := at.applyBreakEvenStop(symbol, side, quantity, entryPrice, currentPnLPct, *matchedBreakEven); err != nil {
						logger.Infof("❌ Break-even stop apply failed (%s %s): %v", symbol, side, err)
					} else if currentPnLPct >= matchedBreakEven.TriggerValue {
						at.setBreakEvenState(symbol, side, "armed")
					}
				}
			}
		}

		structureCtx := at.buildDrawdownStructureContext(symbol, side)
		armRules := at.getDrawdownArmRules(currentPnLPct, rules)
		if drawdownCfg.Enabled && drawdownCfg.Mode == store.ProtectionModeAI && drawdownCfg.EngineMode == store.DrawdownEngineModeAI {
			if eval := evaluateAIDrawdownRule(drawdownCfg, currentPnLPct, peakPnLPct, drawdownPct, rules, structureCtx, side, markPrice); eval != nil {
				armRules = []store.DrawdownTakeProfitRule{eval.Rule}
			}
		}

		// For exchange-native trailing protections, arm all tiers whose min-profit gate is already met.
		// Do NOT wait for drawdown to happen first — the exchange trailing order itself is responsible
		// for tracking the drawdown once armed.
		if at.supportsNativeTrailingStop() {
			executionMode := at.getDrawdownExecutionMode(symbol, side)
			if executionMode == "native_trailing_full" || executionMode == "native_partial_trailing" || executionMode == "managed_partial_drawdown" {
				logger.Infof("🟣 Drawdown monitor: %s %s already in %s, skipping duplicate arm pass", symbol, side, executionMode)
				continue
			}
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

		triggeredRules := at.getTriggeredDrawdownRules(currentPnLPct, drawdownPct, rules)
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
				if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, triggeredRule) {
					continue
				}
			}
		}

		matchedRule := normalizeDrawdownRule(triggeredRules[0])
		ruleFingerprint := drawdownRuleFingerprint(entryPrice, quantity, matchedRule)
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

func (at *AutoTrader) getActiveDrawdownRules() []store.DrawdownTakeProfitRule {
	if at.config.StrategyConfig == nil {
		return nil
	}

	cfg := at.config.StrategyConfig.Protection.DrawdownTakeProfit
	if !cfg.Enabled || len(cfg.Rules) == 0 {
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
		rules = append(rules, rule)
	}
	return rules
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
		candidate := record.Decisions[i]
		if symbol != "" && !strings.EqualFold(candidate.Symbol, symbol) {
			continue
		}
		if action != "" && !strings.EqualFold(candidate.Action, action) {
			continue
		}
		return &candidate
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
	if err != nil || openPos == nil || openPos.EntryDecisionCycle <= 0 {
		return nil
	}
	record, err := decisionStore.GetRecordByCycle(at.id, openPos.EntryDecisionCycle)
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
	if anchorsRaw, ok := decoded["anchors"].([]interface{}); ok {
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
		ctx.Anchors = anchors
	}
	if ctx.PrimaryTimeframe == "" && len(ctx.Support) == 0 && len(ctx.Resistance) == 0 && len(ctx.FibLevels) == 0 && len(ctx.Anchors) == 0 && ctx.Entry == 0 && ctx.FirstTarget == 0 {
		return nil
	}
	return ctx
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

func (at *AutoTrader) findEquivalentPartialTrailingOrder(symbol, side string, rule store.DrawdownTakeProfitRule, entryPrice float64, openOrders []OpenOrder) (*nativeTrailingOrder, float64, float64, float64) {
	plannedActivationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
	plannedCallbackRate := calculateProfitBasedTrailingCallbackRatio(entryPrice, side, rule.MinProfitPct, rule.MaxDrawdownPct)
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
			if math.Abs(order.Quantity-qtyTarget) <= qtyTolerance && math.Abs(order.CallbackRate-plannedCallbackRate) <= callbackTolerance {
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

func (at *AutoTrader) applyNativeTrailingDrawdown(symbol, side string, entryPrice float64, rule store.DrawdownTakeProfitRule) bool {
	if !at.supportsNativeTrailingStop() {
		return false
	}
	currentState := at.getProtectionState(symbol, side)
	isPartial := rule.CloseRatioPct < 99.999
	if currentState == "native_trailing_armed" || currentState == "native_partial_trailing_armed" {
		if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
			if !isPartial {
				for _, order := range openOrders {
					if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
						continue
					}
					if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
						return true
					}
				}
				logger.Infof("⚠️ Native trailing state exists but no trailing order found on exchange (%s %s), re-arming", symbol, side)
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
	priceBasedCallbackRatio := calculateProfitBasedTrailingCallbackRatio(entryPrice, side, rule.MinProfitPct, rule.MaxDrawdownPct)
	if activationPrice <= 0 || priceBasedCallbackRatio <= 0 {
		return false
	}

	logger.Infof("🎯 Trailing activation resolved: %s %s | activation=%.6f planned=%.6f callbackRatio=%.6f", symbol, side, activationPrice, plannedActivationPrice, priceBasedCallbackRatio)

	positionSide := strings.ToUpper(side)
	positionAction := "open_" + strings.ToLower(side)
	exchange := strings.ToLower(at.exchange)

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
					if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
						existingTier, _, _, _ = at.findEquivalentPartialTrailingOrder(symbol, side, rule, entryPrice, openOrders)
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
						SetTrailingStopLossTagged(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64, reasonTag string) error
						CancelTrailingStopOrdersByIDs(symbol string, orderIDs []string) error
					}); ok {
						if err := tagged.SetTrailingStopLossTagged(symbol, positionSide, activationPrice, okxCallbackRatio, partialQty, "native_trailing"); err == nil {
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
								if existingTier != nil && existingTier.OrderID != "" {
									if err := tagged.CancelTrailingStopOrdersByIDs(symbol, []string{existingTier.OrderID}); err != nil {
										logger.Infof("⚠️ Failed to cancel replaced native partial trailing tier (%s %s, okx): %v", symbol, side, err)
									}
								}
								at.setProtectionState(symbol, side, "native_partial_trailing_armed")
								logger.Infof("🟣 Native partial trailing drawdown armed: %s %s | activation=%.6f callback=%.6f close=%.1f%% qty=%.4f", symbol, side, activationPrice, okxCallbackRatio, rule.CloseRatioPct, partialQty)
								return true
							}
							logger.Infof("❌ Native partial trailing drawdown verify failed (%s %s, okx): new tier not visible after placement", symbol, side)
						} else {
							logger.Infof("❌ Native partial trailing drawdown apply failed (%s %s, okx): %v", symbol, side, err)
						}
					} else if err := okxTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, okxCallbackRatio, partialQty); err == nil {
						at.setProtectionState(symbol, side, "native_partial_trailing_armed")
						logger.Infof("🟣 Native partial trailing drawdown armed: %s %s | activation=%.6f callback=%.6f close=%.1f%% qty=%.4f", symbol, side, activationPrice, okxCallbackRatio, rule.CloseRatioPct, partialQty)
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
			SetTrailingStopLossTagged(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64, reasonTag string) error
		}); ok {
			if err := tagged.SetTrailingStopLossTagged(symbol, positionSide, activationPrice, okxCallbackRatio, 0, "native_trailing"); err != nil {
				logger.Infof("❌ Native trailing drawdown apply failed (%s %s): %v", symbol, side, err)
				return false
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
		logger.Infof("🟣 Native trailing drawdown armed: %s %s | activation=%.6f callbackRatio=%.6f", symbol, side, activationPrice, priceBasedCallbackRatio)
	}
	return true
}

func (at *AutoTrader) matchDrawdownArmRule(currentPnLPct float64, rules []store.DrawdownTakeProfitRule) *store.DrawdownTakeProfitRule {
	var matched *store.DrawdownTakeProfitRule
	for i := range rules {
		rule := rules[i]
		if currentPnLPct < rule.MinProfitPct {
			continue
		}
		if matched == nil || rule.MinProfitPct > matched.MinProfitPct {
			matched = &rule
		}
	}
	return matched
}

func (at *AutoTrader) getDrawdownArmRules(currentPnLPct float64, rules []store.DrawdownTakeProfitRule) []store.DrawdownTakeProfitRule {
	matched := make([]store.DrawdownTakeProfitRule, 0, len(rules))
	maxSatisfiedMinProfit := 0.0
	for _, rule := range rules {
		if currentPnLPct < rule.MinProfitPct {
			continue
		}
		if rule.MinProfitPct > maxSatisfiedMinProfit {
			maxSatisfiedMinProfit = rule.MinProfitPct
		}
	}
	for _, rule := range rules {
		if currentPnLPct < rule.MinProfitPct || rule.MinProfitPct < maxSatisfiedMinProfit {
			continue
		}
		matched = append(matched, rule)
	}
	return matched
}

func (at *AutoTrader) matchDrawdownRule(currentPnLPct, drawdownPct float64, rules []store.DrawdownTakeProfitRule) *store.DrawdownTakeProfitRule {
	var matched *store.DrawdownTakeProfitRule
	for i := range rules {
		rule := rules[i]
		if currentPnLPct < rule.MinProfitPct || drawdownPct < rule.MaxDrawdownPct {
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
		if currentPnLPct < rule.MinProfitPct || drawdownPct < rule.MaxDrawdownPct {
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
		if src, ok := at.drawdownSource[positionKey(symbol, side)]; ok && src != "" {
			return src
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

func (at *AutoTrader) getActiveBreakEvenConfig() *store.BreakEvenStopConfig {
	if at.config.StrategyConfig == nil {
		return nil
	}

	cfg := at.config.StrategyConfig.Protection.BreakEvenStop
	if !cfg.Enabled {
		return nil
	}
	if cfg.TriggerMode != store.BreakEvenTriggerProfitPct {
		return nil
	}
	if cfg.TriggerValue <= 0 {
		return nil
	}
	if cfg.OffsetPct < 0 {
		cfg.OffsetPct = 0
	}
	return &cfg
}

func (at *AutoTrader) applyBreakEvenStop(symbol, side string, quantity, entryPrice, currentPnLPct float64, cfg store.BreakEvenStopConfig) error {
	if currentPnLPct < cfg.TriggerValue || entryPrice <= 0 || quantity <= 0 {
		return nil
	}

	caps := at.GetProtectionCapabilities()
	if !caps.NativeStopLoss {
		return fmt.Errorf("exchange %s does not support native stop loss for break-even", at.exchange)
	}

	breakEvenPrice := calculateBreakEvenStopPrice(side, entryPrice, cfg.OffsetPct)
	if breakEvenPrice <= 0 {
		return fmt.Errorf("invalid break-even stop price calculated for %s %s", symbol, side)
	}

	// Break-even stop is managed independently. Do not cancel existing ladder/full stop-loss
	// orders here, otherwise we destroy the long-term stop-loss protection stack.
	// If exchanges later support per-order tags / amend-by-id, we can target only prior
	// break-even stops. For now, preserve existing SL orders and add break-even separately.
	positionSide := strings.ToUpper(side)
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

	logger.Infof("🟠 Break-even stop applied: %s %s | trigger=%.2f%% current=%.2f%% stop=%.6f",
		symbol, side, cfg.TriggerValue, currentPnLPct, breakEvenPrice)
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
func (at *AutoTrader) enforceMinPositionSize(positionSizeUSD float64) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = 12 // Default: 12 USDT
	}

	if positionSizeUSD < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum (%.2f USDT)", positionSizeUSD, minSize)
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
