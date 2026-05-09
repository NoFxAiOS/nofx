package trader

import "nofx/logger"

func (at *AutoTrader) handleSyncedFullClose(symbol, side string) {
	if at == nil || symbol == "" || side == "" {
		return
	}
	logger.Infof("🧹 Protection cleanup: synced full close detected for %s %s; clearing protection state and canceling orphan orders", symbol, side)
	at.cleanupInactiveProtectionState(at.currentActivePositionKeys())
}

func (at *AutoTrader) currentActivePositionKeys() map[string]struct{} {
	active := make(map[string]struct{})
	if at == nil || at.trader == nil {
		return active
	}
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Warnf("⚠️ Protection cleanup: failed to fetch positions after synced full close: %v", err)
		return active
	}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		qty, _ := pos["positionAmt"].(float64)
		if qty < 0 {
			qty = -qty
		}
		if symbol == "" || side == "" || qty <= 0 {
			continue
		}
		active[positionKey(symbol, side)] = struct{}{}
	}
	return active
}
