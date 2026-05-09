package store

func AttachSyncedOrderToPosition(st *Store, orderStore *OrderStore, positionStore *PositionStore, orderRecord *TraderOrder, traderID, symbol, positionSide, canonicalAction, exchangeOrderID string) {
	if st == nil || orderStore == nil || positionStore == nil || orderRecord == nil || orderRecord.ID == 0 {
		return
	}
	attach := func(positionID int64) {
		if positionID == 0 {
			return
		}
		if err := orderStore.UpdateOrderRelatedPosition(orderRecord.ID, positionID); err == nil {
			orderRecord.RelatedPositionID = positionID
		}
		_ = orderStore.UpdateFillsRelatedPositionByOrderID(orderRecord.ID, positionID)
	}
	if isOpenAction(canonicalAction) {
		if pos, err := positionStore.GetOpenPositionBySymbol(traderID, symbol, positionSide); err == nil && pos != nil {
			attach(pos.ID)
		}
		return
	}
	if isCloseAction(canonicalAction) {
		if event, err := st.PositionClose().GetByTraderAndExchangeOrderID(traderID, exchangeOrderID); err == nil && event != nil {
			attach(event.PositionID)
		}
	}
}

func isOpenAction(action string) bool {
	return len(action) >= len("open_") && action[:len("open_")] == "open_"
}

func isCloseAction(action string) bool {
	return len(action) >= len("close_") && action[:len("close_")] == "close_"
}
