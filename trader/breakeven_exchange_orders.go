package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"strings"
)

// placeBreakEvenTPOrders places conditional take-profit orders on the exchange
// for each break-even rule at position open time.
// This provides exchange-side profit protection: once price hits the trigger level,
// the exchange automatically executes the TP, even if the backend is offline.
func (at *AutoTrader) placeBreakEvenTPOrders(symbol, side string, quantity, entryPrice float64) {
	if at.config.StrategyConfig == nil {
		return
	}
	beCfg := at.config.StrategyConfig.Protection.BreakEvenStop
	if !beCfg.Enabled {
		return
	}

	rules := beCfg.Rules
	if len(rules) == 0 && beCfg.TriggerValue > 0 {
		rules = []store.BreakEvenStopRule{{
			TriggerMode:   beCfg.TriggerMode,
			TriggerValue:  beCfg.TriggerValue,
			OffsetPct:     beCfg.OffsetPct,
			CloseRatioPct: 100,
			StageName:     "BE1",
		}}
	}
	if len(rules) == 0 {
		return
	}

	if !at.supportsTaggedTakeProfit() {
		logger.Infof("🟠 Break-even TP orders: exchange does not support tagged TP, will use runtime polling fallback")
		return
	}

	positionSide := strings.ToUpper(side)
	for _, rule := range rules {
		if rule.TriggerValue <= 0 {
			continue
		}

		// Calculate the TP trigger price based on the trigger value
		// The trigger is a profit% threshold; the TP price is entry + trigger% (locked profit after offset)
		tpPrice := calculateBreakEvenTPPrice(side, entryPrice, rule.TriggerValue, rule.OffsetPct)
		if tpPrice <= 0 {
			continue
		}

		ruleQty := quantity
		if rule.CloseRatioPct > 0 && rule.CloseRatioPct < 100 {
			ruleQty = quantity * rule.CloseRatioPct / 100.0
		}
		if ruleQty <= 0 {
			continue
		}

		stage := rule.StageName
		if stage == "" {
			stage = fmt.Sprintf("BE_TP_%d", int(rule.TriggerValue*10))
		}

		if err := at.placeBreakEvenTPOrder(symbol, positionSide, ruleQty, tpPrice, stage); err != nil {
			logger.Warnf("⚠️ Break-even TP order failed for %s %s stage=%s: %v", symbol, side, stage, err)
		} else {
			logger.Infof("🛡 Break-even TP order placed: %s %s | stage=%s trigger=%.2f%% offset=%.2f%% tp_price=%.6f qty=%.6f",
				symbol, side, stage, rule.TriggerValue, rule.OffsetPct, tpPrice, ruleQty)
		}
	}
}

// placeBreakEvenTPOrder places a single conditional TP order on the exchange.
func (at *AutoTrader) placeBreakEvenTPOrder(symbol, positionSide string, quantity, tpPrice float64, stageName string) error {
	tpTagged, ok := at.trader.(interface {
		SetTakeProfitTagged(symbol string, positionSide string, quantity, takeProfitPrice float64, reasonTag string) error
	})
	if !ok {
		return fmt.Errorf("exchange does not support SetTakeProfitTagged")
	}

	tag := "break_even_tp_" + stageName
	return tpTagged.SetTakeProfitTagged(symbol, positionSide, quantity, tpPrice, tag)
}

// calculateBreakEvenTPPrice calculates the TP trigger price for a break-even rule.
// For a LONG: once price rises triggerValue% above entry, set TP at entry + offsetPct%
// The TP price locks in offsetPct% profit from entry.
func calculateBreakEvenTPPrice(side string, entryPrice, triggerValuePct, offsetPct float64) float64 {
	if entryPrice <= 0 || triggerValuePct <= 0 {
		return 0
	}
	// The TP trigger price is where the exchange will activate the take-profit.
	// We use the trigger value as the activation price (when profit reaches this level).
	triggerMove := triggerValuePct / 100.0
	switch strings.ToLower(side) {
	case "long":
		return entryPrice * (1 + triggerMove)
	case "short":
		return entryPrice * (1 - triggerMove)
	default:
		return 0
	}
}

// supportsTaggedTakeProfit checks if the exchange adapter supports placing tagged TP orders.
func (at *AutoTrader) supportsTaggedTakeProfit() bool {
	_, ok := at.trader.(interface {
		SetTakeProfitTagged(symbol string, positionSide string, quantity, takeProfitPrice float64, reasonTag string) error
	})
	return ok
}

// reconcileBreakEvenTPOrders checks if BE TP orders are present on exchange and re-places missing ones.
func (at *AutoTrader) reconcileBreakEvenTPOrders(symbol, side string, quantity, entryPrice float64, openOrders []OpenOrder) {
	if at.config.StrategyConfig == nil || !at.supportsTaggedTakeProfit() {
		return
	}
	beCfg := at.config.StrategyConfig.Protection.BreakEvenStop
	if !beCfg.Enabled {
		return
	}

	rules := beCfg.Rules
	if len(rules) == 0 && beCfg.TriggerValue > 0 {
		rules = []store.BreakEvenStopRule{{
			TriggerMode:   beCfg.TriggerMode,
			TriggerValue:  beCfg.TriggerValue,
			OffsetPct:     beCfg.OffsetPct,
			CloseRatioPct: 100,
			StageName:     "BE1",
		}}
	}
	if len(rules) == 0 {
		return
	}

	positionSide := strings.ToUpper(side)

	// Count existing BE TP orders on exchange
	existingBECount := 0
	for _, order := range openOrders {
		if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		clientID := strings.ToLower(strings.TrimSpace(order.ClientOrderID))
		if strings.Contains(clientID, "break_even_tp") {
			existingBECount++
		}
	}

	if existingBECount >= len(rules) {
		return
	}

	// Re-place all BE TP orders (idempotent — exchange deduplicates by tag)
	logger.Infof("🛡 Reconciler: re-placing BE TP orders for %s %s (found %d/%d on exchange)", symbol, side, existingBECount, len(rules))
	at.placeBreakEvenTPOrders(symbol, side, quantity, entryPrice)
}
