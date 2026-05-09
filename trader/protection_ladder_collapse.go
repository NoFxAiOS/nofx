package trader

func collapseLadderStopsToTightestFullStop(plan *ProtectionPlan, action string) {
	if plan == nil || len(plan.StopLossOrders) == 0 {
		return
	}
	tightest := plan.StopLossOrders[0].Price
	for _, order := range plan.StopLossOrders[1:] {
		if order.Price <= 0 {
			continue
		}
		switch action {
		case "open_long":
			// Long stops are below entry; the highest stop is the tightest protection.
			if order.Price > tightest {
				tightest = order.Price
			}
		case "open_short":
			// Short stops are above entry; the lowest stop is the tightest protection.
			if order.Price < tightest {
				tightest = order.Price
			}
		}
	}
	if tightest <= 0 {
		return
	}
	plan.StopLossOrders = nil
	plan.NeedsStopLoss = true
	plan.StopLossPrice = tightest
	plan.RequiresPartialClose = len(plan.TakeProfitOrders) > 0
}
