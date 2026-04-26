package trader

import "strings"

func collectUnexpectedProtectionOrderIDs(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan, breakEvenArmed bool, nativeTrailingArmed bool) []string {
	allowedStops := make([]float64, 0)
	allowedTPs := make([]float64, 0)
	if plan != nil {
		for _, target := range plan.StopLossOrders {
			allowedStops = append(allowedStops, target.Price)
		}
		if len(plan.StopLossOrders) == 0 && plan.NeedsStopLoss && plan.StopLossPrice > 0 {
			allowedStops = append(allowedStops, plan.StopLossPrice)
		}
		if plan.FallbackMaxLossPrice > 0 {
			allowedStops = append(allowedStops, plan.FallbackMaxLossPrice)
		}
		for _, target := range plan.TakeProfitOrders {
			allowedTPs = append(allowedTPs, target.Price)
		}
		if len(plan.TakeProfitOrders) == 0 && plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 {
			allowedTPs = append(allowedTPs, plan.TakeProfitPrice)
		}
	}

	ids := make([]string, 0)
	for _, order := range openOrders {
		if positionSide != "" && order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			if nativeTrailingArmed {
				continue
			}
			if order.OrderID != "" {
				ids = append(ids, order.OrderID)
			}
			continue
		}
		price := order.StopPrice
		if price <= 0 {
			price = order.Price
		}
		if looksLikeTakeProfit(order) {
			if consumeAllowedProtectionPrice(&allowedTPs, price) {
				continue
			}
			if order.OrderID != "" {
				ids = append(ids, order.OrderID)
			}
			continue
		}
		if looksLikeStopLoss(order) {
			if consumeAllowedProtectionPrice(&allowedStops, price) {
				continue
			}
			if breakEvenArmed {
				breakEvenArmed = false
				continue
			}
			if order.OrderID != "" {
				ids = append(ids, order.OrderID)
			}
		}
	}
	return ids
}
