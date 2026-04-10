package trader

import (
	"fmt"
	"strings"
	"time"

	"nofx/logger"
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

		if err := at.reconcileProtectionForPosition(symbol, side, quantity, entryPrice); err != nil {
			logger.Infof("❌ Protection reconciler: %s %s reconcile failed: %v", symbol, side, err)
			at.setProtectionState(symbol, side, "reconcile_failed: "+err.Error())
			continue
		}

		at.setProtectionState(symbol, side, "exchange_protection_verified")
		logger.Infof("✅ Protection reconciler: %s %s exchange protection verified", symbol, side)
	}
}

func (at *AutoTrader) reconcileProtectionForPosition(symbol, side string, quantity, entryPrice float64) error {
	positionSide := strings.ToUpper(side)
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("get open orders: %w", err)
	}

	manualPlan, err := at.BuildManualProtectionPlan(entryPrice, symbol, actionFromPositionSide(side))
	if err != nil {
		return fmt.Errorf("build manual plan: %w", err)
	}

	if manualPlan != nil {
		missingSL, missingTP := detectMissingProtection(openOrders, positionSide, manualPlan)
		if missingSL || missingTP {
			logger.Infof("🛠 Protection reconciler: %s %s missing exchange orders (SL=%v TP=%v), re-applying plan", symbol, positionSide, missingSL, missingTP)
			if err := at.placeAndVerifyProtectionPlanWithRetry(symbol, positionSide, quantity, manualPlan); err != nil {
				return fmt.Errorf("re-apply manual protection plan: %w", err)
			}
			return nil
		}
	}

	be := at.getActiveBreakEvenConfig()
	if be != nil && at.GetProtectionCapabilities().NativeStopLoss {
		// If break-even is enabled, ensure at least one SL exists on exchange.
		if !hasAnyProtectionOrder(openOrders, positionSide, false) {
			stopPrice := calculateBreakEvenStopPrice(side, entryPrice, be.OffsetPct)
			if stopPrice > 0 {
				logger.Infof("🛠 Protection reconciler: %s %s break-even protection missing on exchange, setting native stop", symbol, positionSide)
				if err := at.placeAndVerifyProtectionWithRetry(symbol, positionSide, quantity, true, stopPrice, false, 0); err != nil {
					return fmt.Errorf("apply break-even native stop: %w", err)
				}
			}
		}
	}

	return nil
}

func detectMissingProtection(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan) (missingSL bool, missingTP bool) {
	if plan == nil {
		return false, false
	}

	if len(plan.StopLossOrders) > 1 {
		for _, target := range plan.StopLossOrders {
			if !hasMatchingProtectionOrder(openOrders, positionSide, false, target.Price) {
				missingSL = true
				break
			}
		}
	} else if plan.NeedsStopLoss {
		missingSL = !hasMatchingProtectionOrder(openOrders, positionSide, false, plan.StopLossPrice)
	}

	if len(plan.TakeProfitOrders) > 1 {
		for _, target := range plan.TakeProfitOrders {
			if !hasMatchingProtectionOrder(openOrders, positionSide, true, target.Price) {
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
