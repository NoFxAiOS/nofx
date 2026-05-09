package trader

import "strings"

type unexpectedProtectionOrderCategory string

const (
	unexpectedCategoryStaleBotDuplicate    unexpectedProtectionOrderCategory = "stale_bot_duplicate"
	unexpectedCategoryOrphanForInactive    unexpectedProtectionOrderCategory = "orphan_for_inactive_position"
	unexpectedCategoryManualOrForeign      unexpectedProtectionOrderCategory = "manual_or_foreign"
	unexpectedCategoryExpectedDynamicOwner unexpectedProtectionOrderCategory = "expected_dynamic_owner"
	unexpectedCategoryExpectedStaticOwner  unexpectedProtectionOrderCategory = "expected_static_owner"
)

type unexpectedProtectionOrderClassification struct {
	OrderID  string
	Kind     string
	Category unexpectedProtectionOrderCategory
}

type unexpectedProtectionSummary struct {
	StaleBotDuplicate    int
	OrphanForInactive    int
	ManualOrForeign      int
	ExpectedDynamicOwner int
	ExpectedStaticOwner  int
	StaleBotDuplicateIDs []string
	OrphanForInactiveIDs []string
	ManualOrForeignIDs   []string
}

func classifyUnexpectedProtectionOrders(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan, breakEvenArmed bool, nativeTrailingArmed bool, positionActive bool) unexpectedProtectionSummary {
	allowedStops, allowedTPs := allowedProtectionPricesForPlan(plan)
	summary := unexpectedProtectionSummary{}
	for _, order := range openOrders {
		if positionSide != "" && order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		classification := classifyProtectionOrder(order, &allowedStops, &allowedTPs, breakEvenArmed, nativeTrailingArmed, positionActive)
		if classification.Category == unexpectedCategoryExpectedDynamicOwner && looksLikeStopLoss(order) && breakEvenArmed {
			breakEvenArmed = false
		}
		switch classification.Category {
		case unexpectedCategoryStaleBotDuplicate:
			summary.StaleBotDuplicate++
			if classification.OrderID != "" {
				summary.StaleBotDuplicateIDs = append(summary.StaleBotDuplicateIDs, classification.OrderID)
			}
		case unexpectedCategoryOrphanForInactive:
			summary.OrphanForInactive++
			if classification.OrderID != "" {
				summary.OrphanForInactiveIDs = append(summary.OrphanForInactiveIDs, classification.OrderID)
			}
		case unexpectedCategoryManualOrForeign:
			summary.ManualOrForeign++
			if classification.OrderID != "" {
				summary.ManualOrForeignIDs = append(summary.ManualOrForeignIDs, classification.OrderID)
			}
		case unexpectedCategoryExpectedDynamicOwner:
			summary.ExpectedDynamicOwner++
		case unexpectedCategoryExpectedStaticOwner:
			summary.ExpectedStaticOwner++
		}
	}
	return summary
}

func allowedProtectionPricesForPlan(plan *ProtectionPlan) ([]float64, []float64) {
	allowedStops := make([]float64, 0)
	allowedTPs := make([]float64, 0)
	if plan == nil {
		return allowedStops, allowedTPs
	}
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
	return allowedStops, allowedTPs
}

func classifyProtectionOrder(order OpenOrder, allowedStops, allowedTPs *[]float64, breakEvenArmed bool, nativeTrailingArmed bool, positionActive bool) unexpectedProtectionOrderClassification {
	classification := unexpectedProtectionOrderClassification{OrderID: order.OrderID}
	upperType := strings.ToUpper(order.Type)
	if strings.Contains(upperType, "TRAILING") {
		classification.Kind = "trailing"
		if !positionActive {
			classification.Category = unexpectedCategoryOrphanForInactive
		} else if nativeTrailingArmed {
			classification.Category = unexpectedCategoryExpectedDynamicOwner
		} else if isLikelyBotProtectionOrder(order) {
			classification.Category = unexpectedCategoryStaleBotDuplicate
		} else {
			classification.Category = unexpectedCategoryManualOrForeign
		}
		return classification
	}

	price := order.StopPrice
	if price <= 0 {
		price = order.Price
	}
	if looksLikeTakeProfit(order) {
		classification.Kind = "take_profit"
		if consumeAllowedProtectionPrice(allowedTPs, price) {
			classification.Category = unexpectedCategoryExpectedStaticOwner
		} else if !positionActive {
			classification.Category = unexpectedCategoryOrphanForInactive
		} else if isLikelyBotProtectionOrder(order) {
			classification.Category = unexpectedCategoryStaleBotDuplicate
		} else {
			classification.Category = unexpectedCategoryManualOrForeign
		}
		return classification
	}
	if looksLikeStopLoss(order) {
		classification.Kind = "stop_loss"
		if consumeAllowedProtectionPrice(allowedStops, price) {
			classification.Category = unexpectedCategoryExpectedStaticOwner
		} else if !positionActive {
			classification.Category = unexpectedCategoryOrphanForInactive
		} else if breakEvenArmed {
			classification.Category = unexpectedCategoryExpectedDynamicOwner
		} else if isLikelyBotProtectionOrder(order) {
			classification.Category = unexpectedCategoryStaleBotDuplicate
		} else {
			classification.Category = unexpectedCategoryManualOrForeign
		}
		return classification
	}
	classification.Kind = "unknown"
	classification.Category = unexpectedCategoryManualOrForeign
	return classification
}

func isLikelyBotProtectionOrder(order OpenOrder) bool {
	id := strings.ToLower(order.OrderID)
	clientID := strings.ToLower(order.ClientOrderID)
	combined := id + " " + clientID
	botMarkers := []string{
		"native_trailing",
		"managed_drawdown",
		"break_even",
		"ladder_",
		"full_",
		"fallback",
		"4c363c81edc5bcde",
		"be-stop",
		"new-tier",
		"stale-",
	}
	for _, marker := range botMarkers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}
