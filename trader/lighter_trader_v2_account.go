package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetBalance 獲取賬戶余額（實現 Trader 接口）
func (t *LighterTraderV2) GetBalance() (map[string]interface{}, error) {
	balance, err := t.GetAccountBalance()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_equity":       balance.TotalEquity,
		"available_balance":  balance.AvailableBalance,
		"margin_used":        balance.MarginUsed,
		"unrealized_pnl":     balance.UnrealizedPnL,
		"maintenance_margin": balance.MaintenanceMargin,
	}, nil
}

// GetAccountBalance 獲取賬戶詳細余額信息
func (t *LighterTraderV2) GetAccountBalance() (*AccountBalance, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("認證令牌無效: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	authToken := t.authToken
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/balance", t.baseURL, accountIndex)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// 添加認證頭
	req.Header.Set("Authorization", authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取余額失敗 (status %d): %s", resp.StatusCode, string(body))
	}

	var balance AccountBalance
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("解析余額響應失敗: %w", err)
	}

	return &balance, nil
}

// GetPositions 獲取所有持倉（實現 Trader 接口）
func (t *LighterTraderV2) GetPositions() ([]map[string]interface{}, error) {
	positions, err := t.GetPositionsRaw("")
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		result = append(result, map[string]interface{}{
			"symbol":             pos.Symbol,
			"side":               pos.Side,
			"size":               pos.Size,
			"entry_price":        pos.EntryPrice,
			"mark_price":         pos.MarkPrice,
			"liquidation_price":  pos.LiquidationPrice,
			"unrealized_pnl":     pos.UnrealizedPnL,
			"leverage":           pos.Leverage,
			"margin_used":        pos.MarginUsed,
		})
	}

	return result, nil
}

// GetPositionsRaw 獲取所有持倉（返回原始類型）
func (t *LighterTraderV2) GetPositionsRaw(symbol string) ([]Position, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("認證令牌無效: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	authToken := t.authToken
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/positions", t.baseURL, accountIndex)
	if symbol != "" {
		endpoint += fmt.Sprintf("?symbol=%s", symbol)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取持倉失敗 (status %d): %s", resp.StatusCode, string(body))
	}

	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("解析持倉響應失敗: %w", err)
	}

	return positions, nil
}

// GetPosition 獲取指定幣種的持倉
func (t *LighterTraderV2) GetPosition(symbol string) (*Position, error) {
	positions, err := t.GetPositionsRaw(symbol)
	if err != nil {
		return nil, err
	}

	for _, pos := range positions {
		if pos.Symbol == symbol && pos.Size > 0 {
			return &pos, nil
		}
	}

	return nil, nil // 無持倉
}

// GetMarketPrice 獲取市場價格（實現 Trader 接口）
func (t *LighterTraderV2) GetMarketPrice(symbol string) (float64, error) {
	endpoint := fmt.Sprintf("%s/api/v1/market/ticker?symbol=%s", t.baseURL, symbol)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return 0, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("獲取市場價格失敗 (status %d): %s", resp.StatusCode, string(body))
	}

	var ticker map[string]interface{}
	if err := json.Unmarshal(body, &ticker); err != nil {
		return 0, fmt.Errorf("解析價格響應失敗: %w", err)
	}

	price, err := SafeFloat64(ticker, "last_price")
	if err != nil {
		return 0, fmt.Errorf("無法獲取價格: %w", err)
	}

	return price, nil
}

// FormatQuantity 格式化數量到正確的精度（實現 Trader 接口）
func (t *LighterTraderV2) FormatQuantity(symbol string, quantity float64) (string, error) {
	// TODO: 從 API 獲取幣種精度
	// 暫時使用默認精度
	return fmt.Sprintf("%.4f", quantity), nil
}
