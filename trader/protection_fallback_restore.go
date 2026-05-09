package trader

import (
	"fmt"

	"nofx/logger"
)

func (at *AutoTrader) placeAndVerifyFallbackMaxLoss(symbol, positionSide string, quantity, fallbackPrice float64) error {
	if fallbackPrice <= 0 {
		return fmt.Errorf("invalid fallback max-loss price %.8f", fallbackPrice)
	}
	if err := at.placeFallbackMaxLossProtection(symbol, positionSide, quantity, fallbackPrice); err != nil {
		return err
	}
	for attempt := 1; attempt <= protectionVerifyMaxAttempts; attempt++ {
		at.sleepForVerification(protectionVerifyDelay)
		openOrders, err := at.trader.GetOpenOrders(symbol)
		if err != nil {
			return fmt.Errorf("failed to verify fallback max-loss restore: %w", err)
		}
		if hasMatchingProtectionOrder(openOrders, positionSide, false, fallbackPrice) {
			logger.Infof("  ✅ Fallback max-loss restore verified: symbol=%s side=%s stop=%.6f", symbol, positionSide, fallbackPrice)
			return nil
		}
	}
	return fmt.Errorf("fallback max-loss restore verification failed for %s %s at %.6f", symbol, positionSide, fallbackPrice)
}
