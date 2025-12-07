package trader

import (
	"fmt"
	"nofx/logger"
)

// OpenLong Open long position
func (t *LighterTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// TODO: Implement complete open long logic
	logger.Infof("🚧 LIGHTER OpenLong not fully implemented (symbol=%s, qty=%.4f, leverage=%d)", symbol, quantity, leverage)

	// Use market buy order
	orderID, err := t.CreateOrder(symbol, "buy", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to open long: %w", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort Open short position
func (t *LighterTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// TODO: Implement complete open short logic
	logger.Infof("🚧 LIGHTER OpenShort not fully implemented (symbol=%s, qty=%.4f, leverage=%d)", symbol, quantity, leverage)

	// Use market sell order
	orderID, err := t.CreateOrder(symbol, "sell", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to open short: %w", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong Close long position (quantity=0 means close all)
func (t *LighterTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity=0, get current position size
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	// Use market sell order to close
	orderID, err := t.CreateOrder(symbol, "sell", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to close long: %w", err)
	}

	// Cancel all pending orders after closing
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort Close short position (quantity=0 means close all)
func (t *LighterTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity=0, get current position size
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	// Use market buy order to close
	orderID, err := t.CreateOrder(symbol, "buy", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to close short: %w", err)
	}

	// Cancel all pending orders after closing
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// SetStopLoss Set stop-loss order
func (t *LighterTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	// TODO: Implement complete stop-loss logic
	logger.Infof("🚧 LIGHTER SetStopLoss not fully implemented (symbol=%s, side=%s, qty=%.4f, stop=%.2f)", symbol, positionSide, quantity, stopPrice)

	// Determine order side (short position uses buy, long position uses sell)
	side := "sell"
	if positionSide == "SHORT" {
		side = "buy"
	}

	// Create limit stop-loss order
	_, err := t.CreateOrder(symbol, side, quantity, stopPrice, "limit")
	if err != nil {
		return fmt.Errorf("failed to set stop-loss: %w", err)
	}

	logger.Infof("✓ LIGHTER - stop-loss set: %.2f (side: %s)", stopPrice, side)
	return nil
}

// SetTakeProfit Set take-profit order
func (t *LighterTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// TODO: Implement complete take-profit logic
	logger.Infof("🚧 LIGHTER SetTakeProfit not fully implemented (symbol=%s, side=%s, qty=%.4f, tp=%.2f)", symbol, positionSide, quantity, takeProfitPrice)

	// Determine order side (short position uses buy, long position uses sell)
	side := "sell"
	if positionSide == "SHORT" {
		side = "buy"
	}

	// Create limit take-profit order
	_, err := t.CreateOrder(symbol, side, quantity, takeProfitPrice, "limit")
	if err != nil {
		return fmt.Errorf("failed to set take-profit: %w", err)
	}

	logger.Infof("✓ LIGHTER - take-profit set: %.2f (side: %s)", takeProfitPrice, side)
	return nil
}

// SetMarginMode Set position mode (true=cross, false=isolated)
func (t *LighterTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// TODO: Implement position mode setting
	modeStr := "isolated"
	if isCrossMargin {
		modeStr = "cross"
	}
	logger.Infof("🚧 LIGHTER SetMarginMode not implemented (symbol=%s, mode=%s)", symbol, modeStr)
	return nil
}

// FormatQuantity Format quantity to correct precision
func (t *LighterTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// TODO: Get symbol precision from LIGHTER API
	// Using default precision for now
	return fmt.Sprintf("%.4f", quantity), nil
}
