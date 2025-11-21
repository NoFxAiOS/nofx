package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/elliottech/lighter-go/types"
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

	// 獲取市場索引
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("獲取市場索引失敗: %w", err)
	}

	// 構建請求 URL
	endpoint := fmt.Sprintf("%s/api/v1/accountActiveOrders?account_index=%d&market_id=%d",
		t.baseURL, t.accountIndex, marketIndex)

	// 發送 GET 請求
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("創建請求失敗: %w", err)
	}

	// 添加認證頭
	req.Header.Set("Authorization", t.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("請求失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	// 解析響應
	var apiResp struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Data    []OrderResponse  `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w, body: %s", err, string(body))
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("獲取活躍訂單失敗 (code %d): %s", apiResp.Code, apiResp.Message)
	}

	log.Printf("✓ LIGHTER - 獲取到 %d 個活躍訂單", len(apiResp.Data))
	return apiResp.Data, nil
}

// CancelOrder 取消單個訂單
func (t *LighterTraderV2) CancelOrder(symbol, orderID string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	// 獲取市場索引
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return fmt.Errorf("獲取市場索引失敗: %w", err)
	}

	// 將 orderID 轉換為 int64
	orderIndex, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("無效的訂單ID: %w", err)
	}

	// 構建取消訂單請求
	txReq := &types.CancelOrderTxReq{
		MarketIndex: marketIndex,
		Index:       orderIndex,
	}

	// 使用 SDK 簽名交易
	nonce := int64(-1) // -1 表示自動獲取
	tx, err := t.txClient.GetCancelOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return fmt.Errorf("簽名取消訂單失敗: %w", err)
	}

	// 序列化交易
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("序列化交易失敗: %w", err)
	}

	// 提交取消訂單到 LIGHTER API
	_, err = t.submitCancelOrder(txBytes)
	if err != nil {
		return fmt.Errorf("提交取消訂單失敗: %w", err)
	}

	log.Printf("✓ LIGHTER訂單已取消 - ID: %s", orderID)
	return nil
}

// submitCancelOrder 提交已簽名的取消訂單到 LIGHTER API
func (t *LighterTraderV2) submitCancelOrder(signedTx []byte) (map[string]interface{}, error) {
	const TX_TYPE_CANCEL_ORDER = 15

	// 構建請求
	req := SendTxRequest{
		TxType:          TX_TYPE_CANCEL_ORDER,
		TxInfo:          string(signedTx),
		PriceProtection: false, // 取消訂單不需要價格保護
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化請求失敗: %w", err)
	}

	// 發送 POST 請求到 /api/v1/sendTx
	endpoint := fmt.Sprintf("%s/api/v1/sendTx", t.baseURL)
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析響應
	var sendResp SendTxResponse
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w, body: %s", err, string(body))
	}

	// 檢查響應碼
	if sendResp.Code != 200 {
		return nil, fmt.Errorf("提交取消訂單失敗 (code %d): %s", sendResp.Code, sendResp.Message)
	}

	result := map[string]interface{}{
		"tx_hash": sendResp.Data["tx_hash"],
		"status":  "cancelled",
	}

	log.Printf("✓ 取消訂單已提交到 LIGHTER - tx_hash: %v", sendResp.Data["tx_hash"])
	return result, nil
}
