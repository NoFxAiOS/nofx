package trader

import (
	"fmt"
	"math"
	"strings"

	"nofx/kernel"
	"nofx/logger"
)

// fillRiskBudgetTolerance lets the actual fill exceed the per-trade risk budget
// by this fraction. Sizing spends the full budget at the pre-order price, so
// the few seconds of drift before a market fill (leverage cooldown etc.)
// otherwise reject fills that are 0.1–1.5% over; real slippage still trips it.
const fillRiskBudgetTolerance = 0.05

// A structural invalidation level cannot move merely because execution slipped.
func (at *AutoTrader) actualFillRisk(d *kernel.Decision, fill, quantity, equity float64) error {
	if fill <= 0 || quantity <= 0 || math.IsNaN(fill) || math.IsInf(fill, 0) || math.IsNaN(quantity) || math.IsInf(quantity, 0) {
		return fmt.Errorf("actual fill price/quantity unavailable")
	}
	risk, reward := fill-d.StopLoss, d.TakeProfit-fill
	if strings.HasPrefix(d.Action, "open_short") {
		risk, reward = d.StopLoss-fill, fill-d.TakeProfit
	}
	if d.StopLoss <= 0 || d.TakeProfit <= 0 || math.IsNaN(risk) || math.IsInf(risk, 0) || math.IsNaN(reward) || math.IsInf(reward, 0) || risk <= 0 || reward <= 0 {
		return fmt.Errorf("actual fill crossed structural SL/TP")
	}
	if d.Leverage > 0 && risk/fill*100 >= 72/float64(d.Leverage) {
		return fmt.Errorf("actual fill stop exceeds liquidation safety distance")
	}
	if at.config.StrategyConfig == nil {
		return nil
	}
	rc := at.config.StrategyConfig.RiskControl
	costBps := rc.EffectiveEntryRoundTripCostBps()
	cost := fill * costBps / 10000
	if rc.MinRiskRewardRatio > 0 && (reward-cost)/(risk+cost) < rc.MinRiskRewardRatio {
		return fmt.Errorf("actual fill net RR %.3f below %.3f", (reward-cost)/(risk+cost), rc.MinRiskRewardRatio)
	}
	if equity <= 0 {
		balance, err := at.trader.GetBalance()
		if err != nil {
			return fmt.Errorf("actual fill equity unavailable: %w", err)
		}
		equity = gridAccountEquity(balance)
	}
	if equity <= 0 || math.IsNaN(equity) || math.IsInf(equity, 0) {
		return fmt.Errorf("actual fill equity unavailable")
	}
	budget := equity * rc.EffectiveRiskPerTradePct() / 100
	if quantity*(risk+cost) > budget*(1+fillRiskBudgetTolerance)+1e-8 {
		return fmt.Errorf("actual fill risk %.6g exceeds budget %.6g (+%.0f%% tolerance)", quantity*(risk+cost), budget, fillRiskBudgetTolerance*100)
	}
	return nil
}

func (at *AutoTrader) rejectMarketFill(d *kernel.Decision, side string, quantity float64, cause error) error {
	at.setProtectionFault("recovery:"+d.Symbol+"_"+side, "invalid fill exit awaiting reconciliation: "+cause.Error())
	if err := at.emergencyClosePosition(d.Symbol, side); err != nil {
		// A rejected close must not abandon the original protection plan.
		at.placeProtectiveOrders(d, strings.ToUpper(side), quantity, d.Price, 0)
		return fmt.Errorf("invalid fill (%v); emergency close failed: %w", cause, err)
	}
	return fmt.Errorf("invalid fill (%v); exit submitted, position reconciliation required", cause)
}

func (at *AutoTrader) invalidateExecutionCache() {
	if c, ok := at.trader.(interface{ InvalidateAccountCache() }); ok {
		c.InvalidateAccountCache()
	}
	if c, ok := at.trader.(interface{ InvalidatePositionCache() }); ok {
		c.InvalidatePositionCache()
	}
}

// Keep the durable abort state until both cancellation and liquidation are
// observed. A cancel acknowledgement alone says nothing about a racing fill.
func (at *AutoTrader) recoverRejectedPendingFill(pe *pendingEntry, status map[string]interface{}) bool {
	at.setProtectionFault("recovery:"+pe.Symbol+"_"+pe.Side, "invalid pending fill: "+pe.RecoveryReason)
	at.markAIManaged(pe.Symbol, pe.Side, pe.OrderID)
	st, _ := status["status"].(string)
	terminal := isTerminalEntryStatus(st)
	if !terminal {
		if c, ok := at.trader.(interface{ CancelOrder(string, string) error }); ok {
			if err := c.CancelOrder(pe.Symbol, pe.OrderID); err != nil {
				logger.Warnf("invalid fill cancel %s: %v", pe.Symbol, err)
			}
		}
	}
	at.invalidateExecutionCache()
	positions, err := at.trader.GetPositions()
	if err != nil {
		return false
	}
	for _, p := range positions {
		if p["symbol"] != pe.Symbol || p["side"] != pe.Side {
			continue
		}
		qty, ok := p["positionAmt"].(float64)
		if !ok || math.IsNaN(qty) || math.IsInf(qty, 0) {
			return false
		}
		if math.Abs(qty) > 0 {
			if err := at.emergencyClosePosition(pe.Symbol, pe.Side); err != nil {
				logger.Warnf("invalid fill exit %s: %v", pe.Symbol, err)
			}
			return false // confirmation comes from a subsequent fresh snapshot
		}
	}
	return terminal
}

func isTerminalEntryStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "CANCELED", "EXPIRED", "REJECTED", "FILLED":
		return true
	}
	return false
}
