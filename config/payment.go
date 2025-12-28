package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PaymentOrder 支付订单 - Crossmint集成
type PaymentOrder struct {
	ID                  string          `json:"id"`
	CrossmintOrderID    string          `json:"crossmint_order_id,omitempty"`
	UserID              string          `json:"user_id"`
	PackageID           string          `json:"package_id"`
	Amount              float64         `json:"amount"`
	Currency            string          `json:"currency"`
	Credits             int             `json:"credits"`
	Status              string          `json:"status"`
	PaymentMethod       string          `json:"payment_method,omitempty"`
	CrossmintClientSecret string        `json:"crossmint_client_secret,omitempty"`
	WebhookReceivedAt   *time.Time      `json:"webhook_received_at,omitempty"`
	CompletedAt         *time.Time      `json:"completed_at,omitempty"`
	FailedReason        string          `json:"failed_reason,omitempty"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// PaymentOrderStatus 订单状态常量
const (
	PaymentStatusPending    = "pending"
	PaymentStatusProcessing = "processing"
	PaymentStatusCompleted  = "completed"
	PaymentStatusFailed     = "failed"
	PaymentStatusCancelled  = "cancelled"
	PaymentStatusRefunded   = "refunded"
)

// CrossmintWebhookEvent Crossmint webhook事件结构
type CrossmintWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		OrderID  string  `json:"orderId"`
		Status   string  `json:"status"`
		Amount   string  `json:"amount"`
		Currency string  `json:"currency"`
		Metadata struct {
			PackageID string `json:"packageId"`
			Credits   int    `json:"credits"`
			UserID    string `json:"userId"`
		} `json:"metadata"`
		PaidAt string `json:"paidAt,omitempty"`
	} `json:"data"`
}

// CreatePaymentOrder 创建支付订单
func (d *Database) CreatePaymentOrder(order *PaymentOrder) error {
	// 参数验证
	if order.UserID == "" {
		return fmt.Errorf("用户ID不能为空")
	}
	if order.PackageID == "" {
		return fmt.Errorf("套餐ID不能为空")
	}
	if order.Amount <= 0 {
		return fmt.Errorf("订单金额必须大于0")
	}
	if order.Credits <= 0 {
		return fmt.Errorf("积分数量必须大于0")
	}

	// 生成UUID作为订单ID
	if order.ID == "" {
		order.ID = uuid.New().String()
	}

	// 设置默认状态
	if order.Status == "" {
		order.Status = PaymentStatusPending
	}

	// 设置默认货币
	if order.Currency == "" {
		order.Currency = "USDT"
	}

	// 序列化metadata
	var metadataJSON []byte
	if order.Metadata != nil {
		metadataJSON = order.Metadata
	}

	return withRetry(func() (struct{}, error) {
		_, err := d.exec(`
			INSERT INTO payment_orders (
				id, crossmint_order_id, user_id, package_id, amount,
				currency, credits, status, payment_method,
				crossmint_client_secret, metadata, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		`,
			order.ID, order.CrossmintOrderID, order.UserID, order.PackageID,
			order.Amount, order.Currency, order.Credits, order.Status,
			order.PaymentMethod, order.CrossmintClientSecret, metadataJSON,
		)
		return struct{}{}, err
	})
}

// GetPaymentOrderByID 根据订单ID查询订单
func (d *Database) GetPaymentOrderByID(orderID string) (*PaymentOrder, error) {
	return withRetry(func() (*PaymentOrder, error) {
		var order PaymentOrder
		var crossmintOrderID, paymentMethod, clientSecret, failedReason sql.NullString
		var webhookReceivedAt, completedAt sql.NullTime
		var metadataJSON []byte

		err := d.queryRow(`
			SELECT id, crossmint_order_id, user_id, package_id, amount,
				   currency, credits, status, payment_method,
				   crossmint_client_secret, webhook_received_at,
				   completed_at, failed_reason, metadata, created_at, updated_at
			FROM payment_orders
			WHERE id = $1
		`, orderID).Scan(
			&order.ID, &crossmintOrderID, &order.UserID, &order.PackageID,
			&order.Amount, &order.Currency, &order.Credits, &order.Status,
			&paymentMethod, &clientSecret, &webhookReceivedAt,
			&completedAt, &failedReason, &metadataJSON, &order.CreatedAt, &order.UpdatedAt,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("订单不存在: %s", orderID)
			}
			return nil, fmt.Errorf("查询订单失败: %w", err)
		}

		// 处理可空字段
		if crossmintOrderID.Valid {
			order.CrossmintOrderID = crossmintOrderID.String
		}
		if paymentMethod.Valid {
			order.PaymentMethod = paymentMethod.String
		}
		if clientSecret.Valid {
			order.CrossmintClientSecret = clientSecret.String
		}
		if failedReason.Valid {
			order.FailedReason = failedReason.String
		}
		if webhookReceivedAt.Valid {
			order.WebhookReceivedAt = &webhookReceivedAt.Time
		}
		if completedAt.Valid {
			order.CompletedAt = &completedAt.Time
		}
		if len(metadataJSON) > 0 {
			order.Metadata = metadataJSON
		}

		return &order, nil
	})
}

// GetPaymentOrderByCrossmintID 根据Crossmint订单ID查询订单（用于webhook）
func (d *Database) GetPaymentOrderByCrossmintID(crossmintOrderID string) (*PaymentOrder, error) {
	return withRetry(func() (*PaymentOrder, error) {
		var order PaymentOrder
		var crossmintID, paymentMethod, clientSecret, failedReason sql.NullString
		var webhookReceivedAt, completedAt sql.NullTime
		var metadataJSON []byte

		err := d.queryRow(`
			SELECT id, crossmint_order_id, user_id, package_id, amount,
				   currency, credits, status, payment_method,
				   crossmint_client_secret, webhook_received_at,
				   completed_at, failed_reason, metadata, created_at, updated_at
			FROM payment_orders
			WHERE crossmint_order_id = $1
		`, crossmintOrderID).Scan(
			&order.ID, &crossmintID, &order.UserID, &order.PackageID,
			&order.Amount, &order.Currency, &order.Credits, &order.Status,
			&paymentMethod, &clientSecret, &webhookReceivedAt,
			&completedAt, &failedReason, &metadataJSON, &order.CreatedAt, &order.UpdatedAt,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("订单不存在: %s", crossmintOrderID)
			}
			return nil, fmt.Errorf("查询订单失败: %w", err)
		}

		// 处理可空字段
		if crossmintID.Valid {
			order.CrossmintOrderID = crossmintID.String
		}
		if paymentMethod.Valid {
			order.PaymentMethod = paymentMethod.String
		}
		if clientSecret.Valid {
			order.CrossmintClientSecret = clientSecret.String
		}
		if failedReason.Valid {
			order.FailedReason = failedReason.String
		}
		if webhookReceivedAt.Valid {
			order.WebhookReceivedAt = &webhookReceivedAt.Time
		}
		if completedAt.Valid {
			order.CompletedAt = &completedAt.Time
		}
		if len(metadataJSON) > 0 {
			order.Metadata = metadataJSON
		}

		return &order, nil
	})
}

// UpdatePaymentOrderStatus 更新订单状态
func (d *Database) UpdatePaymentOrderStatus(orderID, status string, failedReason ...string) error {
	// 验证状态值
	validStatuses := map[string]bool{
		PaymentStatusPending:    true,
		PaymentStatusProcessing: true,
		PaymentStatusCompleted:  true,
		PaymentStatusFailed:     true,
		PaymentStatusCancelled:  true,
		PaymentStatusRefunded:   true,
	}
	if !validStatuses[status] {
		return fmt.Errorf("无效的订单状态: %s", status)
	}

	return withRetry(func() (struct{}, error) {
		var err error
		if status == PaymentStatusCompleted {
			// 完成状态设置completed_at时间戳
			_, err = d.exec(`
				UPDATE payment_orders
				SET status = $1, completed_at = NOW(), updated_at = NOW()
				WHERE id = $2
			`, status, orderID)
		} else if status == PaymentStatusFailed && len(failedReason) > 0 {
			// 失败状态记录原因
			_, err = d.exec(`
				UPDATE payment_orders
				SET status = $1, failed_reason = $2, updated_at = NOW()
				WHERE id = $3
			`, status, failedReason[0], orderID)
		} else {
			_, err = d.exec(`
				UPDATE payment_orders
				SET status = $1, updated_at = NOW()
				WHERE id = $2
			`, status, orderID)
		}
		return struct{}{}, err
	})
}

// UpdatePaymentOrderWithCrossmintID 更新订单关联Crossmint订单ID
func (d *Database) UpdatePaymentOrderWithCrossmintID(orderID, crossmintOrderID, clientSecret string) error {
	if orderID == "" || crossmintOrderID == "" {
		return fmt.Errorf("订单ID和Crossmint订单ID不能为空")
	}

	return withRetry(func() (struct{}, error) {
		_, err := d.exec(`
			UPDATE payment_orders
			SET crossmint_order_id = $1,
			    crossmint_client_secret = $2,
			    status = $3,
			    updated_at = NOW()
			WHERE id = $4
		`, crossmintOrderID, clientSecret, PaymentStatusProcessing, orderID)
		return struct{}{}, err
	})
}

// MarkPaymentOrderWebhookReceived 标记订单webhook已接收
func (d *Database) MarkPaymentOrderWebhookReceived(crossmintOrderID string) error {
	return withRetry(func() (struct{}, error) {
		_, err := d.exec(`
			UPDATE payment_orders
			SET webhook_received_at = NOW(), updated_at = NOW()
			WHERE crossmint_order_id = $1
		`, crossmintOrderID)
		return struct{}{}, err
	})
}

// GetUserPaymentOrders 获取用户支付订单列表
func (d *Database) GetUserPaymentOrders(userID string, page, limit int) ([]*PaymentOrder, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	type result struct {
		orders []*PaymentOrder
		total  int
	}

	res, err := withRetry(func() (result, error) {
		// 查询总数
		var total int
		err := d.queryRow(`SELECT COUNT(*) FROM payment_orders WHERE user_id = $1`, userID).Scan(&total)
		if err != nil {
			return result{}, fmt.Errorf("查询订单总数失败: %w", err)
		}

		// 查询订单列表
		rows, err := d.query(`
			SELECT id, crossmint_order_id, user_id, package_id, amount,
				   currency, credits, status, payment_method,
				   crossmint_client_secret, webhook_received_at,
				   completed_at, failed_reason, metadata, created_at, updated_at
			FROM payment_orders
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`, userID, limit, offset)
		if err != nil {
			return result{}, fmt.Errorf("查询订单列表失败: %w", err)
		}
		defer rows.Close()

		orders := make([]*PaymentOrder, 0)
		for rows.Next() {
			var order PaymentOrder
			var crossmintOrderID, paymentMethod, clientSecret, failedReason sql.NullString
			var webhookReceivedAt, completedAt sql.NullTime
			var metadataJSON []byte

			err := rows.Scan(
				&order.ID, &crossmintOrderID, &order.UserID, &order.PackageID,
				&order.Amount, &order.Currency, &order.Credits, &order.Status,
				&paymentMethod, &clientSecret, &webhookReceivedAt,
				&completedAt, &failedReason, &metadataJSON, &order.CreatedAt, &order.UpdatedAt,
			)
			if err != nil {
				return result{}, fmt.Errorf("扫描订单数据失败: %w", err)
			}

			// 处理可空字段
			if crossmintOrderID.Valid {
				order.CrossmintOrderID = crossmintOrderID.String
			}
			if paymentMethod.Valid {
				order.PaymentMethod = paymentMethod.String
			}
			if clientSecret.Valid {
				order.CrossmintClientSecret = clientSecret.String
			}
			if failedReason.Valid {
				order.FailedReason = failedReason.String
			}
			if webhookReceivedAt.Valid {
				order.WebhookReceivedAt = &webhookReceivedAt.Time
			}
			if completedAt.Valid {
				order.CompletedAt = &completedAt.Time
			}
			if len(metadataJSON) > 0 {
				order.Metadata = metadataJSON
			}

			orders = append(orders, &order)
		}

		return result{orders: orders, total: total}, nil
	})

	if err != nil {
		return nil, 0, err
	}

	return res.orders, res.total, nil
}
