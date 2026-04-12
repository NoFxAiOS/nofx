package trader

import (
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

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Calculate current P&L percentage
		leverage := 10 // Default value
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

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

		matchedBreakEven := at.getActiveBreakEvenConfig()
		if matchedBreakEven != nil {
			currentProtectionState := at.getProtectionState(symbol, side)
			nativeTrailingArmed := currentProtectionState == "native_trailing_armed" || currentProtectionState == "native_partial_trailing_armed"
			if !nativeTrailingArmed && at.getBreakEvenState(symbol, side) != "armed" {
				if err := at.applyBreakEvenStop(symbol, side, quantity, entryPrice, currentPnLPct, *matchedBreakEven); err != nil {
					logger.Infof("❌ Break-even stop apply failed (%s %s): %v", symbol, side, err)
				} else if currentPnLPct >= matchedBreakEven.TriggerValue {
					at.setBreakEvenState(symbol, side, "armed")
					at.setProtectionState(symbol, side, "break_even_armed")
				}
			}
		}

		// For exchange-native trailing protections, arm as soon as the position reaches
		// the minimum profit threshold. Do NOT wait for drawdown to happen first — the
		// exchange trailing order itself is responsible for tracking the drawdown.
		if at.supportsNativeTrailingStop() {
			if armRule := at.matchDrawdownArmRule(currentPnLPct, rules); armRule != nil {
				if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, *armRule) {
					continue
				}
			}
		}

		matchedRule := at.matchDrawdownRule(currentPnLPct, drawdownPct, rules)
		if matchedRule == nil {
			if currentPnLPct > 0 {
				logger.Infof("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
					symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
			}
			continue
		}

		if at.applyNativeTrailingDrawdown(symbol, side, entryPrice, *matchedRule) {
			continue
		}

		closeQty := quantity * matchedRule.CloseRatioPct / 100.0
		if closeQty <= 0 || matchedRule.CloseRatioPct >= 99.999 {
			closeQty = 0 // exchange adapters use 0 to mean close all
		}

		logger.Infof("🚨 Drawdown take-profit triggered: %s %s | Current profit: %.2f%% | Peak profit: %.2f%% | Drawdown: %.2f%% | CloseRatio: %.2f%%",
			symbol, side, currentPnLPct, peakPnLPct, drawdownPct, matchedRule.CloseRatioPct)

		if err := at.closePositionBySide(symbol, side, closeQty); err != nil {
			logger.Infof("❌ Drawdown take-profit failed (%s %s): %v", symbol, side, err)
			continue
		}

		logger.Infof("✅ Drawdown take-profit succeeded: %s %s", symbol, side)
		at.setProtectionState(symbol, side, "drawdown_triggered")
		if closeQty == 0 {
			at.ClearPeakPnLCache(symbol, side)
			at.clearBreakEvenState(symbol, side)
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

func (at *AutoTrader) applyNativeTrailingDrawdown(symbol, side string, entryPrice float64, rule store.DrawdownTakeProfitRule) bool {
	if !at.supportsNativeTrailingStop() {
		return false
	}
	currentState := at.getProtectionState(symbol, side)
	if currentState == "native_trailing_armed" || currentState == "native_partial_trailing_armed" {
		// Do not trust in-memory state alone. Verify the exchange still has a live trailing order.
		if openOrders, err := at.trader.GetOpenOrders(symbol); err == nil {
			for _, order := range openOrders {
				if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, strings.ToUpper(side)) {
					continue
				}
				if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
					return true
				}
			}
			logger.Infof("⚠️ Native trailing state exists but no trailing order found on exchange (%s %s), re-arming", symbol, side)
		}
	}
	// For partial close rules, check if exchange supports native partial close
	isPartial := rule.CloseRatioPct < 99.999
	if isPartial {
		caps := at.GetProtectionCapabilities()
		if !caps.NativePartialClose {
			return false
		}
	}
	if entryPrice <= 0 || rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 {
		return false
	}

	activationPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
	priceBasedCallbackRatio := calculateProfitBasedTrailingCallbackRatio(entryPrice, side, rule.MinProfitPct, rule.MaxDrawdownPct)
	if activationPrice <= 0 || priceBasedCallbackRatio <= 0 {
		return false
	}

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
					if err := okxTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, okxCallbackRatio, partialQty); err == nil {
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
		if err := binanceTrader.CancelTrailingStopOrders(symbol); err != nil {
			logger.Infof("⚠️ Native trailing reconcile cancel failed (%s %s): %v", symbol, side, err)
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
		if err := bitgetTrader.CancelTrailingStopOrders(symbol); err != nil {
			logger.Infof("⚠️ Native trailing reconcile cancel failed (%s %s): %v", symbol, side, err)
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
		if err := okxTrader.CancelTrailingStopOrders(symbol); err != nil {
			logger.Infof("⚠️ Native trailing reconcile cancel failed (%s %s): %v", symbol, side, err)
		}
		okxCallbackRatio := priceBasedCallbackRatio
		if okxCallbackRatio < 0.001 {
			okxCallbackRatio = 0.001
		}
		if okxCallbackRatio > 1 {
			okxCallbackRatio = 1
		}
		if err := okxTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, okxCallbackRatio, 0); err != nil {
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
	if err := at.trader.SetStopLoss(symbol, positionSide, quantity, breakEvenPrice); err != nil {
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
	switch strings.ToLower(side) {
	case "long":
		order, err := at.trader.CloseLong(symbol, quantity)
		if err != nil {
			return err
		}
		logger.Infof("✅ Close long position succeeded, order ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, quantity)
		if err != nil {
			return err
		}
		logger.Infof("✅ Close short position succeeded, order ID: %v", order["orderId"])
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	return nil
}

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	return at.closePositionBySide(symbol, side, 0)
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
