// Package payment Crossmint支付服务层
// 设计哲学：单一职责，最小依赖，高内聚低耦合
package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nofx/config"
	"os"
	"time"

	"github.com/google/uuid"
)

// Service 支付服务接口
type Service interface {
	// 订单管理
	CreatePaymentOrder(ctx context.Context, userID, packageID string) (*config.PaymentOrder, error)
	GetPaymentOrder(ctx context.Context, orderID string) (*config.PaymentOrder, error)
	GetUserPaymentOrders(ctx context.Context, userID string, page, limit int) ([]*config.PaymentOrder, int, error)

	// Crossmint集成
	CreateCrossmintOrder(ctx context.Context, order *config.PaymentOrder) (crossmintOrderID, clientSecret string, err error)
	ProcessWebhook(ctx context.Context, signature string, body []byte) error
	VerifyWebhookSignature(signature string, body []byte) bool
}

// PaymentService 支付服务实现
type PaymentService struct {
	db                   *config.Database
	crossmintServerKey   string
	crossmintWebhookSecret string
	crossmintAPIURL      string
	httpClient           *http.Client
}

// NewPaymentService 创建支付服务
func NewPaymentService(db *config.Database) Service {
	serverKey := os.Getenv("CROSSMINT_SERVER_API_KEY")
	webhookSecret := os.Getenv("CROSSMINT_WEBHOOK_SECRET")
	apiURL := os.Getenv("CROSSMINT_API_URL")

	// 默认使用staging环境
	if apiURL == "" {
		env := os.Getenv("CROSSMINT_ENVIRONMENT")
		if env == "production" {
			apiURL = "https://api.crossmint.com"
		} else {
			apiURL = "https://staging.crossmint.com/api"
		}
	}

	return &PaymentService{
		db:                     db,
		crossmintServerKey:     serverKey,
		crossmintWebhookSecret: webhookSecret,
		crossmintAPIURL:        apiURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreatePaymentOrder 创建支付订单
func (s *PaymentService) CreatePaymentOrder(ctx context.Context, userID, packageID string) (*config.PaymentOrder, error) {
	// 参数验证
	if userID == "" {
		return nil, fmt.Errorf("用户ID不能为空")
	}
	if packageID == "" {
		return nil, fmt.Errorf("套餐ID不能为空")
	}

	log.Printf("🔄 创建支付订单: userID=%s, packageID=%s", userID, packageID)

	// 获取套餐信息
	pkg, err := s.db.GetPackageByID(packageID)
	if err != nil {
		return nil, fmt.Errorf("获取套餐信息失败: %w", err)
	}

	if !pkg.IsActive {
		return nil, fmt.Errorf("套餐已下架")
	}

	// 创建订单
	order := &config.PaymentOrder{
		ID:        uuid.New().String(),
		UserID:    userID,
		PackageID: packageID,
		Amount:    pkg.PriceUSDT,
		Currency:  "USDT",
		Credits:   pkg.Credits + pkg.BonusCredits, // 基础积分 + 赠送积分
		Status:    config.PaymentStatusPending,
	}

	// 保存到数据库
	if err := s.db.CreatePaymentOrder(order); err != nil {
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}

	log.Printf("✅ 支付订单创建成功: orderID=%s, amount=%.2f USDT, credits=%d",
		order.ID, order.Amount, order.Credits)

	return order, nil
}

// GetPaymentOrder 获取支付订单
func (s *PaymentService) GetPaymentOrder(ctx context.Context, orderID string) (*config.PaymentOrder, error) {
	if orderID == "" {
		return nil, fmt.Errorf("订单ID不能为空")
	}

	return s.db.GetPaymentOrderByID(orderID)
}

// GetUserPaymentOrders 获取用户支付订单列表
func (s *PaymentService) GetUserPaymentOrders(ctx context.Context, userID string, page, limit int) ([]*config.PaymentOrder, int, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("用户ID不能为空")
	}

	return s.db.GetUserPaymentOrders(userID, page, limit)
}

// CreateCrossmintOrder 调用Crossmint API创建订单
func (s *PaymentService) CreateCrossmintOrder(ctx context.Context, order *config.PaymentOrder) (crossmintOrderID, clientSecret string, err error) {
	if s.crossmintServerKey == "" {
		return "", "", fmt.Errorf("Crossmint服务未配置：缺少API密钥")
	}

	log.Printf("🔄 调用Crossmint API创建订单: orderID=%s, amount=%.2f %s",
		order.ID, order.Amount, order.Currency)

	// 构建Crossmint API请求
	requestBody := map[string]interface{}{
		"payment": map[string]interface{}{
			"currency": order.Currency,
			"amount":   fmt.Sprintf("%.2f", order.Amount),
			"method":   "crypto",
		},
		"locale": "en-US",
		"metadata": map[string]interface{}{
			"orderId":   order.ID,
			"packageId": order.PackageID,
			"credits":   order.Credits,
			"userId":    order.UserID,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 发送HTTP请求到Crossmint
	apiURL := fmt.Sprintf("%s/v1-alpha2/orders", s.crossmintAPIURL)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", s.crossmintServerKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Crossmint API调用失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("❌ Crossmint API错误 (状态码 %d): %s", resp.StatusCode, string(respBody))
		return "", "", fmt.Errorf("Crossmint API返回错误 (状态码 %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var crossmintResp struct {
		OrderID      string `json:"orderId"`
		ClientSecret string `json:"clientSecret"`
	}

	if err := json.Unmarshal(respBody, &crossmintResp); err != nil {
		return "", "", fmt.Errorf("解析Crossmint响应失败: %w", err)
	}

	if crossmintResp.OrderID == "" || crossmintResp.ClientSecret == "" {
		return "", "", fmt.Errorf("Crossmint响应缺少必要字段")
	}

	// 更新订单关联Crossmint订单ID
	if err := s.db.UpdatePaymentOrderWithCrossmintID(order.ID, crossmintResp.OrderID, crossmintResp.ClientSecret); err != nil {
		log.Printf("⚠️ 更新订单Crossmint ID失败: %v", err)
		// 不返回错误，因为Crossmint订单已创建成功
	}

	log.Printf("✅ Crossmint订单创建成功: crossmintOrderID=%s", crossmintResp.OrderID)

	return crossmintResp.OrderID, crossmintResp.ClientSecret, nil
}

// VerifyWebhookSignature 验证Crossmint webhook签名
func (s *PaymentService) VerifyWebhookSignature(signature string, body []byte) bool {
	if s.crossmintWebhookSecret == "" {
		log.Printf("⚠️ Webhook签名验证跳过：未配置webhook secret")
		return true // 开发环境允许跳过
	}

	// 使用HMAC-SHA256验证签名
	mac := hmac.New(sha256.New, []byte(s.crossmintWebhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// ProcessWebhook 处理Crossmint webhook事件
func (s *PaymentService) ProcessWebhook(ctx context.Context, signature string, body []byte) error {
	// 验证签名
	if !s.VerifyWebhookSignature(signature, body) {
		return fmt.Errorf("webhook签名验证失败")
	}

	// 解析webhook事件
	var event config.CrossmintWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("解析webhook事件失败: %w", err)
	}

	log.Printf("📥 收到Crossmint webhook: type=%s, orderID=%s, status=%s",
		event.Type, event.Data.OrderID, event.Data.Status)

	// 查询订单
	order, err := s.db.GetPaymentOrderByCrossmintID(event.Data.OrderID)
	if err != nil {
		return fmt.Errorf("查询订单失败: %w", err)
	}

	// 标记webhook已接收
	if err := s.db.MarkPaymentOrderWebhookReceived(event.Data.OrderID); err != nil {
		log.Printf("⚠️ 标记webhook接收失败: %v", err)
	}

	// 处理不同事件类型
	switch event.Type {
	case "order.paid":
		return s.handleOrderPaid(ctx, order, &event)
	case "order.failed":
		return s.handleOrderFailed(ctx, order, &event)
	case "order.cancelled":
		return s.handleOrderCancelled(ctx, order, &event)
	default:
		log.Printf("⚠️ 未知的webhook事件类型: %s", event.Type)
		return nil // 返回nil避免重试
	}
}

// handleOrderPaid 处理支付成功事件
func (s *PaymentService) handleOrderPaid(ctx context.Context, order *config.PaymentOrder, event *config.CrossmintWebhookEvent) error {
	// 幂等性检查：避免重复处理
	if order.Status == config.PaymentStatusCompleted {
		log.Printf("⚠️ 订单已处理过，跳过: orderID=%s", order.ID)
		return nil
	}

	log.Printf("🔄 处理支付成功: orderID=%s, userID=%s, credits=%d",
		order.ID, order.UserID, order.Credits)

	// 更新订单状态为已完成
	if err := s.db.UpdatePaymentOrderStatus(order.ID, config.PaymentStatusCompleted); err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}

	// 增加用户积分（使用已有的积分服务）
	err := s.db.AddCredits(
		order.UserID,
		order.Credits,
		"purchase",
		fmt.Sprintf("购买套餐: %s", order.PackageID),
		order.CrossmintOrderID, // 使用Crossmint订单ID作为reference_id
	)

	if err != nil {
		log.Printf("❌ 增加用户积分失败: %v", err)
		// 标记订单为失败状态
		_ = s.db.UpdatePaymentOrderStatus(order.ID, config.PaymentStatusFailed, err.Error())
		return fmt.Errorf("增加用户积分失败: %w", err)
	}

	log.Printf("✅ 支付处理完成: orderID=%s, 积分已到账", order.ID)
	return nil
}

// handleOrderFailed 处理支付失败事件
func (s *PaymentService) handleOrderFailed(ctx context.Context, order *config.PaymentOrder, event *config.CrossmintWebhookEvent) error {
	log.Printf("❌ 支付失败: orderID=%s", order.ID)

	reason := fmt.Sprintf("Crossmint支付失败: %s", event.Data.Status)
	return s.db.UpdatePaymentOrderStatus(order.ID, config.PaymentStatusFailed, reason)
}

// handleOrderCancelled 处理订单取消事件
func (s *PaymentService) handleOrderCancelled(ctx context.Context, order *config.PaymentOrder, event *config.CrossmintWebhookEvent) error {
	log.Printf("🚫 订单已取消: orderID=%s", order.ID)

	return s.db.UpdatePaymentOrderStatus(order.ID, config.PaymentStatusCancelled)
}
