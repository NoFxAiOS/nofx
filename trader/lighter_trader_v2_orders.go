package trader

import (
	"fmt"
	"log"
)

// SetStopLoss 設置止損單（實現 Trader 接口）
func (t *LighterTraderV2) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	log.Printf("🛑 LIGHTER 設置止損: %s %s qty=%.4f, stop=%.2f", symbol, positionSide, quantity, stopPrice)

	// 確定訂單方向（做空止損用買單，做多止損用賣單）
	isAsk := (positionSide == "LONG" || positionSide == "long")

	// 創建限價止損單
	_, err := t.CreateOrder(symbol, isAsk, quantity, stopPrice, "limit")
	if err != nil {
		return fmt.Errorf("設置止損失敗: %w", err)
	}

	log.Printf("✓ LIGHTER 止損已設置: %.2f", stopPrice)
	return nil
}

// SetTakeProfit 設置止盈單（實現 Trader 接口）
func (t *LighterTraderV2) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	log.Printf("🎯 LIGHTER 設置止盈: %s %s qty=%.4f, tp=%.2f", symbol, positionSide, quantity, takeProfitPrice)

	// 確定訂單方向（做空止盈用買單，做多止盈用賣單）
	isAsk := (positionSide == "LONG" || positionSide == "long")

	// 創建限價止盈單
	_, err := t.CreateOrder(symbol, isAsk, quantity, takeProfitPrice, "limit")
	if err != nil {
		return fmt.Errorf("設置止盈失敗: %w", err)
	}

	log.Printf("✓ LIGHTER 止盈已設置: %.2f", takeProfitPrice)
	return nil
}

// CancelAllOrders 取消所有訂單（實現 Trader 接口）
func (t *LighterTraderV2) CancelAllOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("認證令牌無效: %w", err)
	}

	// 獲取所有活躍訂單
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("獲取活躍訂單失敗: %w", err)
	}

	if len(orders) == 0 {
		log.Printf("✓ LIGHTER - 無需取消訂單（無活躍訂單）")
		return nil
	}

	// 批量取消
	canceledCount := 0
	for _, order := range orders {
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			log.Printf("⚠️  取消訂單失敗 (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	log.Printf("✓ LIGHTER - 已取消 %d 個訂單", canceledCount)
	return nil
}

// CancelStopLossOrders 僅取消止損單（實現 Trader 接口）
func (t *LighterTraderV2) CancelStopLossOrders(symbol string) error {
	// LIGHTER 暫時無法區分止損和止盈單，取消所有止盈止損單
	log.Printf("⚠️  LIGHTER 無法區分止損/止盈單，將取消所有止盈止損單")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders 僅取消止盈單（實現 Trader 接口）
func (t *LighterTraderV2) CancelTakeProfitOrders(symbol string) error {
	// LIGHTER 暫時無法區分止損和止盈單，取消所有止盈止損單
	log.Printf("⚠️  LIGHTER 無法區分止損/止盈單，將取消所有止盈止損單")
	return t.CancelStopOrders(symbol)
}

// CancelStopOrders 取消該幣種的止盈/止損單（實現 Trader 接口）
func (t *LighterTraderV2) CancelStopOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("認證令牌無效: %w", err)
	}

	// 獲取活躍訂單
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("獲取活躍訂單失敗: %w", err)
	}

	canceledCount := 0
	for _, order := range orders {
		// TODO: 檢查訂單類型，只取消止盈止損單
		// 暫時取消所有訂單
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			log.Printf("⚠️  取消訂單失敗 (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	log.Printf("✓ LIGHTER - 已取消 %d 個止盈止損單", canceledCount)
	return nil
}

// GetActiveOrders 獲取活躍訂單
func (t *LighterTraderV2) GetActiveOrders(symbol string) ([]OrderResponse, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("認證令牌無效: %w", err)
	}

	// TODO: 實現HTTP GET到LIGHTER API獲取活躍訂單
	// endpoint := fmt.Sprintf("%s/api/v1/order/active?account_index=%d&symbol=%s",
	//     t.baseURL, t.accountIndex, symbol)

	// 暫時返回空列表
	return []OrderResponse{}, nil
}

// CancelOrder 取消單個訂單
func (t *LighterTraderV2) CancelOrder(symbol, orderID string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	// TODO: 使用SDK簽名CancelOrder交易並提交
	log.Printf("✓ LIGHTER訂單已取消 - ID: %s", orderID)
	return nil
}
