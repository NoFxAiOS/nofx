package trader

import (
	"fmt"
	"nofx/store"
)

// ProtectionPlan is the normalized execution representation produced from strategy config
// (and later AI protection plans) before hitting exchange adapters.
type ProtectionPlan struct {
	Mode                 string
	NeedsStopLoss        bool
	NeedsTakeProfit      bool
	StopLossPrice        float64
	TakeProfitPrice      float64
	RequiresNativeOrders bool
}

// BuildManualProtectionPlan creates a minimal full-position protection plan based on manual strategy config.
// This is the Phase-1 foundation; ladder TP/SL and break-even will be added in later phases.
func (at *AutoTrader) BuildManualProtectionPlan(entryPrice float64, decisionSymbol string, action string) (*ProtectionPlan, error) {
	if at.config.StrategyConfig == nil {
		return nil, nil
	}

	protection := at.config.StrategyConfig.Protection
	full := protection.FullTPSL
	if !full.Enabled || full.Mode != store.ProtectionModeManual {
		return nil, nil
	}

	if entryPrice <= 0 {
		return nil, fmt.Errorf("invalid entry price %.8f for protection plan", entryPrice)
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil, nil
	}

	plan := &ProtectionPlan{Mode: string(full.Mode), RequiresNativeOrders: true}

	if full.StopLoss.Enabled && full.StopLoss.PriceMovePct > 0 {
		move := full.StopLoss.PriceMovePct / 100.0
		if isLong {
			plan.StopLossPrice = entryPrice * (1 - move)
		} else {
			plan.StopLossPrice = entryPrice * (1 + move)
		}
		plan.NeedsStopLoss = true
	}

	if full.TakeProfit.Enabled && full.TakeProfit.PriceMovePct > 0 {
		move := full.TakeProfit.PriceMovePct / 100.0
		if isLong {
			plan.TakeProfitPrice = entryPrice * (1 + move)
		} else {
			plan.TakeProfitPrice = entryPrice * (1 - move)
		}
		plan.NeedsTakeProfit = true
	}

	if !plan.NeedsStopLoss && !plan.NeedsTakeProfit {
		return nil, nil
	}

	return plan, nil
}
