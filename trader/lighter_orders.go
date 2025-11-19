package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	Symbol       string  `json:"symbol"`        // 交易对，如 "BTC-PERP"
	Side         string  `json:"side"`          // "buy" 或 "sell"
	OrderType    string  `json:"order_type"`    // "market" 或 "limit"
	Quantity     float64 `json:"quantity"`      // 数量
	Price        float64 `json:"price"`         // 价格（限价单必填）
	ReduceOnly   bool    `json:"reduce_only"`   // 是否只减仓
	TimeInForce  string  `json:"time_in_force"` // "GTC", "IOC", "FOK"
	PostOnly     bool    `json:"post_only"`     // 是否只做Maker
}

// OrderResponse 订单响应
type OrderResponse struct {
	OrderID      string  `json:"order_id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	OrderType    string  `json:"order_type"`
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price"`
	Status       string  `json:"status"` // "open", "filled", "cancelled"
	FilledQty    float64 `json:"filled_qty"`
	RemainingQty float64 `json:"remaining_qty"`
	CreateTime   int64   `json:"create_time"`
}

// CreateOrder 创建订单（市价或限价）
func (t *LighterTrader) CreateOrder(symbol, side string, quantity, price float64, orderType string) (string, error) {
	if err := t.ensureAuthToken(); err != nil {
		return "", fmt.Errorf("认证令牌无效: %w", err)
	}

	// 构建订单请求
	req := CreateOrderRequest{
		Symbol:      symbol,
		Side:        side,
		OrderType:   orderType,
		Quantity:    quantity,
		ReduceOnly:  false,
		TimeInForce: "GTC",
		PostOnly:    false,
	}

	if orderType == "limit" {
		req.Price = price
	}

	// 发送订单
	orderResp, err := t.sendOrder(req)
	if err != nil {
		return "", err
	}

	log.Printf("✓ LIGHTER订单已创建 - ID: %s, Symbol: %s, Side: %s, Qty: %.4f",
		orderResp.OrderID, symbol, side, quantity)

	return orderResp.OrderID, nil
}

// sendOrder 发送订单到LIGHTER API
func (t *LighterTrader) sendOrder(orderReq CreateOrderRequest) (*OrderResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/order", t.baseURL)

	// 序列化请求
	jsonData, err := json.Marshal(orderReq)
	if err != nil {
		return nil, err
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// 添加请求头
	req.Header.Set("Content-Type", "application/json")
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	// 发送请求
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
		return nil, fmt.Errorf("创建订单失败 (status %d): %s", resp.StatusCode, string(body))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	return &orderResp, nil
}

// CancelOrder 取消订单
func (t *LighterTrader) CancelOrder(symbol, orderID string) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("认证令牌无效: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/order/%s", t.baseURL, orderID)

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}

	// 添加认证头
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("取消订单失败 (status %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("✓ LIGHTER订单已取消 - ID: %s", orderID)
	return nil
}

// CancelAllOrders 取消所有订单
func (t *LighterTrader) CancelAllOrders(symbol string) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("认证令牌无效: %w", err)
	}

	// 获取所有活跃订单
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("获取活跃订单失败: %w", err)
	}

	if len(orders) == 0 {
		log.Printf("✓ LIGHTER - 无需取消订单（无活跃订单）")
		return nil
	}

	// 批量取消
	for _, order := range orders {
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			log.Printf("⚠️ 取消订单失败 (ID: %s): %v", order.OrderID, err)
		}
	}

	log.Printf("✓ LIGHTER - 已取消 %d 个订单", len(orders))
	return nil
}

// GetActiveOrders 获取活跃订单
func (t *LighterTrader) GetActiveOrders(symbol string) ([]OrderResponse, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("认证令牌无效: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/order/active?account_index=%d", t.baseURL, accountIndex)
	if symbol != "" {
		endpoint += fmt.Sprintf("&symbol=%s", symbol)
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
		return nil, fmt.Errorf("获取活跃订单失败 (status %d): %s", resp.StatusCode, string(body))
	}

	var orders []OrderResponse
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("解析订单列表失败: %w", err)
	}

	return orders, nil
}

// GetOrderStatus 获取订单状态
func (t *LighterTrader) GetOrderStatus(orderID string) (*OrderResponse, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("认证令牌无效: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/order/%s", t.baseURL, orderID)

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
		return nil, fmt.Errorf("获取订单状态失败 (status %d): %s", resp.StatusCode, string(body))
	}

	var order OrderResponse
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	return &order, nil
}

// CancelStopLossOrders 仅取消止损单（LIGHTER 暂无法区分，取消所有止盈止损单）
func (t *LighterTrader) CancelStopLossOrders(symbol string) error {
	// LIGHTER 暂时无法区分止损和止盈单，取消所有止盈止损单
	log.Printf("  ⚠️ LIGHTER 无法区分止损/止盈单，将取消所有止盈止损单")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders 仅取消止盈单（LIGHTER 暂无法区分，取消所有止盈止损单）
func (t *LighterTrader) CancelTakeProfitOrders(symbol string) error {
	// LIGHTER 暂时无法区分止损和止盈单，取消所有止盈止损单
	log.Printf("  ⚠️ LIGHTER 无法区分止损/止盈单，将取消所有止盈止损单")
	return t.CancelStopOrders(symbol)
}

// CancelStopOrders 取消该币种的止盈/止损单
func (t *LighterTrader) CancelStopOrders(symbol string) error {
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("认证令牌无效: %w", err)
	}

	// 获取活跃订单
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("获取活跃订单失败: %w", err)
	}

	canceledCount := 0
	for _, order := range orders {
		// TODO: 需要检查订单类型，只取消止盈止损单
		// 暂时取消所有订单
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			log.Printf("⚠️ 取消订单失败 (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	log.Printf("✓ LIGHTER - 已取消 %d 个止盈止损单", canceledCount)
	return nil
}
