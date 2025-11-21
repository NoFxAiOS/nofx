package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AccountBalance 账户余额信息
type AccountBalance struct {
	TotalEquity       float64 `json:"total_equity"`        // 总权益
	AvailableBalance  float64 `json:"available_balance"`   // 可用余额
	MarginUsed        float64 `json:"margin_used"`         // 已用保证金
	UnrealizedPnL     float64 `json:"unrealized_pnl"`      // 未实现盈亏
	MaintenanceMargin float64 `json:"maintenance_margin"`  // 维持保证金
}

// Position 持仓信息
type Position struct {
	Symbol           string  `json:"symbol"`             // 交易对
	Side             string  `json:"side"`               // "long" 或 "short"
	Size             float64 `json:"size"`               // 持仓大小
	EntryPrice       float64 `json:"entry_price"`        // 开仓均价
	MarkPrice        float64 `json:"mark_price"`         // 标记价格
	LiquidationPrice float64 `json:"liquidation_price"`  // 强平价格
	UnrealizedPnL    float64 `json:"unrealized_pnl"`     // 未实现盈亏
	Leverage         float64 `json:"leverage"`           // 杠杆倍数
	MarginUsed       float64 `json:"margin_used"`        // 已用保证金
}

// GetBalance 获取账户余额（实现 Trader 接口）
func (t *LighterTrader) GetBalance() (map[string]interface{}, error) {
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

// GetAccountBalance 获取账户详细余额信息
func (t *LighterTrader) GetAccountBalance() (*AccountBalance, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("认证令牌无效: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/balance", t.baseURL, accountIndex)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// 添加认证头
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

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
		return nil, fmt.Errorf("获取余额失败 (status %d): %s", resp.StatusCode, string(body))
	}

	var balance AccountBalance
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("解析余额响应失败: %w", err)
	}

	return &balance, nil
}

// GetPositionsRaw 获取所有持仓（返回原始类型）
func (t *LighterTrader) GetPositionsRaw(symbol string) ([]Position, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("认证令牌无效: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/positions", t.baseURL, accountIndex)
	if symbol != "" {
		endpoint += fmt.Sprintf("?symbol=%s", symbol)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// 添加认证头
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

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
		return nil, fmt.Errorf("获取持仓失败 (status %d): %s", resp.StatusCode, string(body))
	}

	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("解析持仓响应失败: %w", err)
	}

	return positions, nil
}

// GetPositions 获取所有持仓（实现 Trader 接口）
func (t *LighterTrader) GetPositions() ([]map[string]interface{}, error) {
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

// GetPosition 获取指定币种的持仓
func (t *LighterTrader) GetPosition(symbol string) (*Position, error) {
	positions, err := t.GetPositionsRaw(symbol)
	if err != nil {
		return nil, err
	}

	// 找到指定币种的持仓
	for _, pos := range positions {
		if pos.Symbol == symbol && pos.Size > 0 {
			return &pos, nil
		}
	}

	// 无持仓
	return nil, nil
}

// GetMarketPrice 获取市场价格
func (t *LighterTrader) GetMarketPrice(symbol string) (float64, error) {
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
		return 0, fmt.Errorf("获取市场价格失败 (status %d): %s", resp.StatusCode, string(body))
	}

	var ticker map[string]interface{}
	if err := json.Unmarshal(body, &ticker); err != nil {
		return 0, fmt.Errorf("解析价格响应失败: %w", err)
	}

	// 提取最新价格
	price, err := SafeFloat64(ticker, "last_price")
	if err != nil {
		return 0, fmt.Errorf("无法获取价格: %w", err)
	}

	return price, nil
}

// GetAccountInfo 获取账户完整信息（用于AutoTrader）
func (t *LighterTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := t.GetAccountBalance()
	if err != nil {
		return nil, err
	}

	positions, err := t.GetPositionsRaw("")
	if err != nil {
		return nil, err
	}

	// 构建返回信息
	info := map[string]interface{}{
		"total_equity":       balance.TotalEquity,
		"available_balance":  balance.AvailableBalance,
		"margin_used":        balance.MarginUsed,
		"unrealized_pnl":     balance.UnrealizedPnL,
		"maintenance_margin": balance.MaintenanceMargin,
		"positions":          positions,
		"position_count":     len(positions),
	}

	return info, nil
}

// SetLeverage 设置杠杆倍数
func (t *LighterTrader) SetLeverage(symbol string, leverage int) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("认证令牌无效: %w", err)
	}

	// TODO: 实现设置杠杆的API调用
	// LIGHTER可能需要签名交易来设置杠杆

	return fmt.Errorf("SetLeverage未实现")
}

// GetMaxLeverage 获取最大杠杆倍数
func (t *LighterTrader) GetMaxLeverage(symbol string) (int, error) {
	// LIGHTER支持BTC/ETH最高50x杠杆
	// TODO: 从API获取实际限制

	if symbol == "BTC-PERP" || symbol == "ETH-PERP" {
		return 50, nil
	}

	// 其他币种默认20x
	return 20, nil
}
