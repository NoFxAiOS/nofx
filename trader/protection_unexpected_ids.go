package trader

func collectUnexpectedProtectionOrderIDs(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan, breakEvenArmed bool, nativeTrailingArmed bool) []string {
	summary := classifyUnexpectedProtectionOrders(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed, true)
	ids := make([]string, 0, len(summary.StaleBotDuplicateIDs)+len(summary.OrphanForInactiveIDs))
	ids = append(ids, summary.StaleBotDuplicateIDs...)
	ids = append(ids, summary.OrphanForInactiveIDs...)
	return ids
}
