package trader

import (
	"fmt"
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
			if at.getBreakEvenState(symbol, side) != "armed" {
				if err := at.applyBreakEvenStop(symbol, side, quantity, entryPrice, currentPnLPct, *matchedBreakEven); err != nil {
					logger.Infof("❌ Break-even stop apply failed (%s %s): %v", symbol, side, err)
				} else if currentPnLPct >= matchedBreakEven.TriggerValue {
					at.setBreakEvenState(symbol, side, "armed")
					at.setProtectionState(symbol, side, "break_even_armed")
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
	return strings.EqualFold(at.exchange, "binance") && caps.SupportsAlgoOrders && caps.CanAmendProtection
}

func (at *AutoTrader) applyNativeTrailingDrawdown(symbol, side string, entryPrice float64, rule store.DrawdownTakeProfitRule) bool {
	if !at.supportsNativeTrailingStop() {
		return false
	}
	if at.getProtectionState(symbol, side) == "native_trailing_armed" {
		return true
	}
	if rule.CloseRatioPct < 99.999 {
		return false
	}
	if entryPrice <= 0 || rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 {
		return false
	}

	activationPrice := entryPrice
	callbackRate := rule.MaxDrawdownPct
	move := rule.MinProfitPct / 100.0
	if strings.EqualFold(side, "long") {
		activationPrice = entryPrice * (1 + move)
	} else if strings.EqualFold(side, "short") {
		activationPrice = entryPrice * (1 - move)
	} else {
		return false
	}

	if callbackRate < 0.1 {
		callbackRate = 0.1
	}
	if callbackRate > 10 {
		callbackRate = 10
	}

	binanceTrader, ok := at.trader.(interface {
		SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64) error
		CancelTrailingStopOrders(symbol string) error
	})
	if !ok {
		return false
	}

	positionSide := strings.ToUpper(side)
	if err := binanceTrader.CancelTrailingStopOrders(symbol); err != nil {
		logger.Infof("⚠️ Native trailing reconcile cancel failed (%s %s): %v", symbol, side, err)
	}
	if err := binanceTrader.SetTrailingStopLoss(symbol, positionSide, activationPrice, callbackRate); err != nil {
		logger.Infof("❌ Native trailing drawdown apply failed (%s %s): %v", symbol, side, err)
		return false
	}

	at.setProtectionState(symbol, side, "native_trailing_armed")
	logger.Infof("🟣 Native trailing drawdown armed: %s %s | activation=%.6f callback=%.2f%%", symbol, side, activationPrice, callbackRate)
	return true
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

	if caps.CanAmendProtection {
		if err := at.trader.CancelStopLossOrders(symbol); err != nil {
			return fmt.Errorf("failed to cancel previous stop loss before break-even: %w", err)
		}
	}

	positionSide := strings.ToUpper(side)
	if err := at.trader.SetStopLoss(symbol, positionSide, quantity, breakEvenPrice); err != nil {
		return fmt.Errorf("failed to set break-even stop loss: %w", err)
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
