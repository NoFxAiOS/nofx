package trader

import (
	"fmt"
	"log"
)

// OpenLong 开多仓
func (t *LighterTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// TODO: 实现完整的开多仓逻辑
	log.Printf("🚧 LIGHTER OpenLong 暂未完全实现 (symbol=%s, qty=%.4f, leverage=%d)", symbol, quantity, leverage)

	// 使用市价买入单
	orderID, err := t.CreateOrder(symbol, "buy", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort 开空仓
func (t *LighterTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// TODO: 实现完整的开空仓逻辑
	log.Printf("🚧 LIGHTER OpenShort 暂未完全实现 (symbol=%s, qty=%.4f, leverage=%d)", symbol, quantity, leverage)

	// 使用市价卖出单
	orderID, err := t.CreateOrder(symbol, "sell", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong 平多仓（quantity=0表示全部平仓）
func (t *LighterTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果quantity=0，获取当前持仓数量
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("获取持仓失败: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	// 使用市价卖出单平仓
	orderID, err := t.CreateOrder(symbol, "sell", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
	}

	// 平仓后取消所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort 平空仓（quantity=0表示全部平仓）
func (t *LighterTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果quantity=0，获取当前持仓数量
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("获取持仓失败: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	// 使用市价买入单平仓
	orderID, err := t.CreateOrder(symbol, "buy", quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
	}

	// 平仓后取消所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]interface{}{
		"orderId": orderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// SetStopLoss 设置止损单
func (t *LighterTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	// TODO: 实现完整的止损单逻辑
	log.Printf("🚧 LIGHTER SetStopLoss 暂未完全实现 (symbol=%s, side=%s, qty=%.4f, stop=%.2f)", symbol, positionSide, quantity, stopPrice)

	// 确定订单方向（做空止损用买单，做多止损用卖单）
	side := "sell"
	if positionSide == "SHORT" {
		side = "buy"
	}

	// 创建限价止损单
	_, err := t.CreateOrder(symbol, side, quantity, stopPrice, "limit")
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	log.Printf("✓ LIGHTER - 止损已设置: %.2f (side: %s)", stopPrice, side)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *LighterTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// TODO: 实现完整的止盈单逻辑
	log.Printf("🚧 LIGHTER SetTakeProfit 暂未完全实现 (symbol=%s, side=%s, qty=%.4f, tp=%.2f)", symbol, positionSide, quantity, takeProfitPrice)

	// 确定订单方向（做空止盈用买单，做多止盈用卖单）
	side := "sell"
	if positionSide == "SHORT" {
		side = "buy"
	}

	// 创建限价止盈单
	_, err := t.CreateOrder(symbol, side, quantity, takeProfitPrice, "limit")
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	log.Printf("✓ LIGHTER - 止盈已设置: %.2f (side: %s)", takeProfitPrice, side)
	return nil
}

// SetMarginMode 设置仓位模式 (true=全仓, false=逐仓)
func (t *LighterTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// TODO: 实现仓位模式设置
	modeStr := "逐仓"
	if isCrossMargin {
		modeStr = "全仓"
	}
	log.Printf("🚧 LIGHTER SetMarginMode 暂未实现 (symbol=%s, mode=%s)", symbol, modeStr)
	return nil
}

// FormatQuantity 格式化数量到正确的精度
func (t *LighterTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// TODO: 根据LIGHTER API获取币种精度
	// 暂时使用默认精度
	return fmt.Sprintf("%.4f", quantity), nil
}
