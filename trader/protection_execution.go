package trader

import (
	"fmt"
	"math"
	"strings"

	"nofx/kernel"
	"nofx/logger"
	tradertypes "nofx/trader/types"
)

const protectionPriceTolerancePct = 0.002 // 0.2%

type protectionExecutionRequest struct {
	Symbol       string
	Action       string
	PositionSide string
	Quantity     float64
	EntryPrice   float64
	Decision     *kernel.Decision
}

func (at *AutoTrader) applyPostOpenProtection(req *protectionExecutionRequest) error {
	if req == nil || req.Decision == nil {
		return nil
	}

	plan, err := at.BuildManualProtectionPlan(req.EntryPrice, req.Symbol, req.Action)
	if err != nil {
		return err
	}

	if plan != nil {
		caps := at.GetProtectionCapabilities()
		if plan.RequiresNativeOrders && (!caps.NativeStopLoss && plan.NeedsStopLoss || !caps.NativeTakeProfit && plan.NeedsTakeProfit) {
			return fmt.Errorf("exchange %s cannot safely support required native protection orders", at.exchange)
		}
		logger.Infof("  🛡 Applying manual protection plan: stop=%v@%.6f tp=%v@%.6f",
			plan.NeedsStopLoss, plan.StopLossPrice, plan.NeedsTakeProfit, plan.TakeProfitPrice)
		if err := at.placeAndVerifyProtection(req.Symbol, req.PositionSide, req.Quantity, plan.NeedsStopLoss, plan.StopLossPrice, plan.NeedsTakeProfit, plan.TakeProfitPrice); err != nil {
			return err
		}
		return nil
	}

	needsStopLoss := req.Decision.StopLoss > 0
	needsTakeProfit := req.Decision.TakeProfit > 0
	if !needsStopLoss && !needsTakeProfit {
		return nil
	}

	logger.Infof("  🛡 Applying AI decision protection fallback: stop=%v@%.6f tp=%v@%.6f",
		needsStopLoss, req.Decision.StopLoss, needsTakeProfit, req.Decision.TakeProfit)
	return at.placeAndVerifyProtection(req.Symbol, req.PositionSide, req.Quantity, needsStopLoss, req.Decision.StopLoss, needsTakeProfit, req.Decision.TakeProfit)
}

func (at *AutoTrader) placeAndVerifyProtection(symbol, positionSide string, quantity float64, needsStopLoss bool, stopLossPrice float64, needsTakeProfit bool, takeProfitPrice float64) error {
	if needsStopLoss {
		if err := at.trader.SetStopLoss(symbol, positionSide, quantity, stopLossPrice); err != nil {
			return fmt.Errorf("failed to set stop loss: %w", err)
		}
	}
	if needsTakeProfit {
		if err := at.trader.SetTakeProfit(symbol, positionSide, quantity, takeProfitPrice); err != nil {
			return fmt.Errorf("failed to set take profit: %w", err)
		}
	}

	if !needsStopLoss && !needsTakeProfit {
		return nil
	}

	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to verify protection orders: %w", err)
	}

	if needsStopLoss && !hasMatchingProtectionOrder(openOrders, positionSide, false, stopLossPrice) {
		return fmt.Errorf("stop loss verification failed for %s %s at %.6f", symbol, positionSide, stopLossPrice)
	}
	if needsTakeProfit && !hasMatchingProtectionOrder(openOrders, positionSide, true, takeProfitPrice) {
		return fmt.Errorf("take profit verification failed for %s %s at %.6f", symbol, positionSide, takeProfitPrice)
	}

	logger.Infof("  ✅ Protection orders verified: symbol=%s side=%s stop=%v tp=%v", symbol, positionSide, needsStopLoss, needsTakeProfit)
	return nil
}

func hasMatchingProtectionOrder(orders []tradertypes.OpenOrder, positionSide string, wantTakeProfit bool, targetPrice float64) bool {
	for _, order := range orders {
		if positionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) && order.PositionSide != "" {
			continue
		}

		if wantTakeProfit {
			if !looksLikeTakeProfit(order) {
				continue
			}
		} else {
			if !looksLikeStopLoss(order) {
				continue
			}
		}

		price := order.StopPrice
		if price <= 0 {
			price = order.Price
		}
		if approximatelyEqualPrice(price, targetPrice) {
			return true
		}
	}
	return false
}

func looksLikeStopLoss(order tradertypes.OpenOrder) bool {
	kind := strings.ToUpper(order.Type)
	return strings.Contains(kind, "STOP") && !strings.Contains(kind, "TAKE_PROFIT") && !strings.Contains(kind, "TP")
}

func looksLikeTakeProfit(order tradertypes.OpenOrder) bool {
	kind := strings.ToUpper(order.Type)
	return strings.Contains(kind, "TAKE_PROFIT") || strings.Contains(kind, "TP")
}

func approximatelyEqualPrice(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	base := math.Max(math.Abs(a), math.Abs(b))
	if base == 0 {
		return false
	}
	return math.Abs(a-b)/base <= protectionPriceTolerancePct
}
