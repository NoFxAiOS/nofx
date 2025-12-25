package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"nofx/logger"
	"nofx/store"
	"net/http"
	"strings"
	"time"
)

// LighterOrderHistory 订单历史记录
type LighterOrderHistory struct {
	OrderID       string    `json:"order_id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`           // "buy" or "sell"
	Type          string    `json:"type"`           // "limit" or "market"
	Price         string    `json:"price"`
	Size          string    `json:"size"`
	FilledSize    string    `json:"filled_size"`
	Status        string    `json:"status"`         // "filled", "cancelled", etc.
	CreatedAt     int64     `json:"created_at"`
	UpdatedAt     int64     `json:"updated_at"`
	FilledAt      int64     `json:"filled_at"`
}

// SyncOrdersFromLighter 同步 Lighter 交易所的订单历史到本地数据库
func (t *LighterTraderV2) SyncOrdersFromLighter(traderID string, orderStore *store.OrderStore) error {
	// 确保有 account index
	if t.accountIndex == 0 {
		if err := t.initializeAccount(); err != nil {
			return fmt.Errorf("failed to get account index: %w", err)
		}
	}

	// 获取最近的订单（过去24小时）
	startTime := time.Now().Add(-24 * time.Hour).Unix()
	endpoint := fmt.Sprintf("%s/api/v1/orders?account_index=%d&start_time=%d&limit=100",
		t.baseURL, t.accountIndex, startTime)

	logger.Infof("🔄 Syncing Lighter orders from: %s", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 添加认证头
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}
	req.Header.Set("Authorization", t.authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get orders: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Don't spam logs for 404 errors (API endpoint might not be available)
		if resp.StatusCode != http.StatusNotFound {
			logger.Infof("⚠️  Lighter orders API returned %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// 解析响应
	var apiResp struct {
		Code   int                    `json:"code"`
		Orders []LighterOrderHistory  `json:"orders"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		logger.Infof("⚠️  Failed to parse orders response: %v, body: %s", err, string(body))
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return fmt.Errorf("API returned code %d", apiResp.Code)
	}

	logger.Infof("📥 Received %d orders from Lighter", len(apiResp.Orders))

	// 同步每个订单
	syncedCount := 0
	for _, order := range apiResp.Orders {
		// 只同步已成交的订单
		if order.Status != "filled" {
			continue
		}

		// 检查订单是否已存在
		existing, err := orderStore.GetOrderByExchangeID("lighter", order.OrderID)
		if err == nil && existing != nil {
			continue // 订单已存在，跳过
		}

		// 解析价格和数量
		price, _ := parseFloat(order.Price)
		size, _ := parseFloat(order.Size)
		filledSize, _ := parseFloat(order.FilledSize)

		if filledSize == 0 {
			filledSize = size
		}

		// 确定订单方向和动作
		var positionSide, orderAction, side string
		if order.Side == "buy" {
			side = "BUY"
			// 买入可能是开多或平空，这里假设是开多
			positionSide = "LONG"
			orderAction = "open_long"
		} else {
			side = "SELL"
			// 卖出可能是平多或开空，这里假设是平多
			positionSide = "LONG"
			orderAction = "close_long"
		}

		// 估算手续费
		fee := price * filledSize * 0.0004

		// 创建订单记录
		filledAt := time.Unix(order.FilledAt, 0)
		if order.FilledAt == 0 {
			filledAt = time.Unix(order.UpdatedAt, 0)
		}

		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      "lighter",
			ExchangeOrderID: order.OrderID,
			Symbol:          order.Symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        filledSize,
			Price:           price,
			Status:          "FILLED",
			FilledQuantity:  filledSize,
			AvgFillPrice:    price,
			Commission:      fee,
			FilledAt:        filledAt,
			CreatedAt:       time.Unix(order.CreatedAt, 0),
			UpdatedAt:       time.Unix(order.UpdatedAt, 0),
		}

		// 插入订单记录
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync order %s: %v", order.OrderID, err)
			continue
		}

		// 创建成交记录
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      "lighter",
			OrderID:         orderRecord.ID,
			ExchangeOrderID: order.OrderID,
			ExchangeTradeID: fmt.Sprintf("%s-%d", order.OrderID, time.Now().UnixNano()),
			Symbol:          order.Symbol,
			Side:            side,
			Price:           price,
			Quantity:        filledSize,
			QuoteQuantity:   price * filledSize,
			Commission:      fee,
			CommissionAsset: "USDT",
			RealizedPnL:     0,
			IsMaker:         order.Type == "limit",
			CreatedAt:       filledAt,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for order %s: %v", order.OrderID, err)
		}

		syncedCount++
		logger.Infof("  ✅ Synced order: %s %s %s qty=%.6f price=%.6f", order.OrderID, order.Symbol, side, filledSize, price)
	}

	logger.Infof("✅ Order sync completed: %d new orders synced", syncedCount)
	return nil
}

// StartOrderSync 启动订单同步后台任务
func (t *LighterTraderV2) StartOrderSync(traderID string, orderStore *store.OrderStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromLighter(traderID, orderStore); err != nil {
				// Only log non-404 errors to reduce log spam
				if !strings.Contains(err.Error(), "status 404") {
					logger.Infof("⚠️  Order sync failed: %v", err)
				}
			}
		}
	}()
	logger.Infof("🔄 Lighter order sync started (interval: %v)", interval)
}
