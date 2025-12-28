package config

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB 创建测试数据库
func setupPaymentTestDB(t *testing.T) *Database {
	// 使用SQLite内存数据库进行测试
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err, "打开内存数据库失败")

	// 创建必要的表结构
	schema := `
	CREATE TABLE users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		is_active BOOLEAN DEFAULT TRUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE credit_packages (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		name_en TEXT,
		description TEXT,
		price_usdt REAL NOT NULL,
		credits INTEGER NOT NULL,
		bonus_credits INTEGER DEFAULT 0,
		is_active BOOLEAN DEFAULT TRUE,
		is_recommended BOOLEAN DEFAULT FALSE,
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE payment_orders (
		id TEXT PRIMARY KEY,
		crossmint_order_id TEXT UNIQUE,
		user_id TEXT NOT NULL,
		package_id TEXT NOT NULL,
		amount REAL NOT NULL,
		currency TEXT NOT NULL DEFAULT 'USDT',
		credits INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		payment_method TEXT,
		crossmint_client_secret TEXT,
		webhook_received_at DATETIME,
		completed_at DATETIME,
		failed_reason TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (package_id) REFERENCES credit_packages(id)
	);

	CREATE TABLE user_credits (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL UNIQUE,
		available_credits INTEGER DEFAULT 0,
		total_credits INTEGER DEFAULT 0,
		used_credits INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE TABLE credit_transactions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		type TEXT NOT NULL,
		amount INTEGER NOT NULL,
		balance_before INTEGER NOT NULL,
		balance_after INTEGER NOT NULL,
		category TEXT NOT NULL,
		description TEXT,
		reference_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err, "创建表结构失败")

	// 插入测试用户
	_, err = db.Exec(`
		INSERT INTO users (id, email, password_hash)
		VALUES ('test_user_1', 'test@example.com', 'hash123')
	`)
	require.NoError(t, err, "插入测试用户失败")

	// 插入测试套餐
	_, err = db.Exec(`
		INSERT INTO credit_packages (id, name, name_en, description, price_usdt, credits, bonus_credits, is_active)
		VALUES ('pkg_test', '测试套餐', 'Test Package', 'Test description', 10.00, 500, 100, 1)
	`)
	require.NoError(t, err, "插入测试套餐失败")

	// 包装为Database对象
	database := &Database{
		neonDB:    db,
		sqliteDB:  db,
		currentDB: db,
		usingNeon: false,
	}

	return database
}

// TestCreatePaymentOrder 测试创建支付订单
func TestCreatePaymentOrder(t *testing.T) {
	db := setupPaymentTestDB(t)
	defer db.Close()

	t.Run("Valid Order", func(t *testing.T) {
		order := &PaymentOrder{
			UserID:    "test_user_1",
			PackageID: "pkg_test",
			Amount:    10.00,
			Credits:   600,
			Currency:  "USDT",
			Status:    PaymentStatusPending,
		}

		err := db.CreatePaymentOrder(order)
		assert.NoError(t, err, "创建订单应该成功")
		assert.NotEmpty(t, order.ID, "订单ID应该被自动生成")
	})

	t.Run("Missing UserID", func(t *testing.T) {
		order := &PaymentOrder{
			PackageID: "pkg_test",
			Amount:    10.00,
			Credits:   600,
		}

		err := db.CreatePaymentOrder(order)
		assert.Error(t, err, "缺少用户ID应该返回错误")
		assert.Contains(t, err.Error(), "用户ID不能为空")
	})

	t.Run("Missing PackageID", func(t *testing.T) {
		order := &PaymentOrder{
			UserID:  "test_user_1",
			Amount:  10.00,
			Credits: 600,
		}

		err := db.CreatePaymentOrder(order)
		assert.Error(t, err, "缺少套餐ID应该返回错误")
		assert.Contains(t, err.Error(), "套餐ID不能为空")
	})

	t.Run("Invalid Amount", func(t *testing.T) {
		order := &PaymentOrder{
			UserID:    "test_user_1",
			PackageID: "pkg_test",
			Amount:    -10.00,
			Credits:   600,
		}

		err := db.CreatePaymentOrder(order)
		assert.Error(t, err, "负金额应该返回错误")
		assert.Contains(t, err.Error(), "订单金额必须大于0")
	})

	t.Run("Invalid Credits", func(t *testing.T) {
		order := &PaymentOrder{
			UserID:    "test_user_1",
			PackageID: "pkg_test",
			Amount:    10.00,
			Credits:   0,
		}

		err := db.CreatePaymentOrder(order)
		assert.Error(t, err, "零积分应该返回错误")
		assert.Contains(t, err.Error(), "积分数量必须大于0")
	})
}

// TestGetPaymentOrderByID 测试查询订单
func TestGetPaymentOrderByID(t *testing.T) {
	db := setupPaymentTestDB(t)
	defer db.Close()

	// 创建测试订单
	order := &PaymentOrder{
		UserID:    "test_user_1",
		PackageID: "pkg_test",
		Amount:    10.00,
		Credits:   600,
		Currency:  "USDT",
		Status:    PaymentStatusPending,
	}
	err := db.CreatePaymentOrder(order)
	require.NoError(t, err)

	t.Run("Existing Order", func(t *testing.T) {
		retrieved, err := db.GetPaymentOrderByID(order.ID)
		assert.NoError(t, err)
		assert.Equal(t, order.ID, retrieved.ID)
		assert.Equal(t, order.UserID, retrieved.UserID)
		assert.Equal(t, order.Amount, retrieved.Amount)
		assert.Equal(t, order.Credits, retrieved.Credits)
	})

	t.Run("Non-existing Order", func(t *testing.T) {
		_, err := db.GetPaymentOrderByID("non_existing_id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "订单不存在")
	})
}

// TestGetPaymentOrderByCrossmintID 测试通过Crossmint ID查询
func TestGetPaymentOrderByCrossmintID(t *testing.T) {
	db := setupPaymentTestDB(t)
	defer db.Close()

	// 创建订单并关联Crossmint ID
	order := &PaymentOrder{
		UserID:    "test_user_1",
		PackageID: "pkg_test",
		Amount:    10.00,
		Credits:   600,
		Currency:  "USDT",
		Status:    PaymentStatusPending,
	}
	err := db.CreatePaymentOrder(order)
	require.NoError(t, err)

	crossmintID := "crossmint_order_12345"
	err = db.UpdatePaymentOrderWithCrossmintID(order.ID, crossmintID, "secret_abc")
	require.NoError(t, err)

	t.Run("Find by Crossmint ID", func(t *testing.T) {
		retrieved, err := db.GetPaymentOrderByCrossmintID(crossmintID)
		assert.NoError(t, err)
		assert.Equal(t, order.ID, retrieved.ID)
		assert.Equal(t, crossmintID, retrieved.CrossmintOrderID)
		assert.Equal(t, "secret_abc", retrieved.CrossmintClientSecret)
	})

	t.Run("Non-existing Crossmint ID", func(t *testing.T) {
		_, err := db.GetPaymentOrderByCrossmintID("non_existing_crossmint_id")
		assert.Error(t, err)
	})
}

// TestUpdatePaymentOrderStatus 测试更新订单状态
func TestUpdatePaymentOrderStatus(t *testing.T) {
	db := setupPaymentTestDB(t)
	defer db.Close()

	// 创建测试订单
	order := &PaymentOrder{
		UserID:    "test_user_1",
		PackageID: "pkg_test",
		Amount:    10.00,
		Credits:   600,
		Status:    PaymentStatusPending,
	}
	err := db.CreatePaymentOrder(order)
	require.NoError(t, err)

	t.Run("Update to Processing", func(t *testing.T) {
		err := db.UpdatePaymentOrderStatus(order.ID, PaymentStatusProcessing)
		assert.NoError(t, err)

		retrieved, err := db.GetPaymentOrderByID(order.ID)
		assert.NoError(t, err)
		assert.Equal(t, PaymentStatusProcessing, retrieved.Status)
	})

	t.Run("Update to Completed", func(t *testing.T) {
		err := db.UpdatePaymentOrderStatus(order.ID, PaymentStatusCompleted)
		assert.NoError(t, err)

		retrieved, err := db.GetPaymentOrderByID(order.ID)
		assert.NoError(t, err)
		assert.Equal(t, PaymentStatusCompleted, retrieved.Status)
		assert.NotNil(t, retrieved.CompletedAt, "CompletedAt应该被设置")
	})

	t.Run("Update to Failed with Reason", func(t *testing.T) {
		// 创建新订单用于测试失败状态
		failOrder := &PaymentOrder{
			UserID:    "test_user_1",
			PackageID: "pkg_test",
			Amount:    10.00,
			Credits:   600,
			Status:    PaymentStatusPending,
		}
		err := db.CreatePaymentOrder(failOrder)
		require.NoError(t, err)

		failReason := "支付网关错误"
		err = db.UpdatePaymentOrderStatus(failOrder.ID, PaymentStatusFailed, failReason)
		assert.NoError(t, err)

		retrieved, err := db.GetPaymentOrderByID(failOrder.ID)
		assert.NoError(t, err)
		assert.Equal(t, PaymentStatusFailed, retrieved.Status)
		assert.Equal(t, failReason, retrieved.FailedReason)
	})

	t.Run("Invalid Status", func(t *testing.T) {
		err := db.UpdatePaymentOrderStatus(order.ID, "invalid_status")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "无效的订单状态")
	})
}

// TestUpdatePaymentOrderWithCrossmintID 测试关联Crossmint订单ID
func TestUpdatePaymentOrderWithCrossmintID(t *testing.T) {
	db := setupPaymentTestDB(t)
	defer db.Close()

	order := &PaymentOrder{
		UserID:    "test_user_1",
		PackageID: "pkg_test",
		Amount:    10.00,
		Credits:   600,
		Status:    PaymentStatusPending,
	}
	err := db.CreatePaymentOrder(order)
	require.NoError(t, err)

	t.Run("Valid Update", func(t *testing.T) {
		crossmintID := "crossmint_abc123"
		clientSecret := "secret_xyz789"

		err := db.UpdatePaymentOrderWithCrossmintID(order.ID, crossmintID, clientSecret)
		assert.NoError(t, err)

		retrieved, err := db.GetPaymentOrderByID(order.ID)
		assert.NoError(t, err)
		assert.Equal(t, crossmintID, retrieved.CrossmintOrderID)
		assert.Equal(t, clientSecret, retrieved.CrossmintClientSecret)
		assert.Equal(t, PaymentStatusProcessing, retrieved.Status, "状态应该变为processing")
	})

	t.Run("Missing Parameters", func(t *testing.T) {
		err := db.UpdatePaymentOrderWithCrossmintID("", "crossmint_id", "secret")
		assert.Error(t, err)

		err = db.UpdatePaymentOrderWithCrossmintID(order.ID, "", "secret")
		assert.Error(t, err)
	})
}

// TestMarkPaymentOrderWebhookReceived 测试标记webhook接收
func TestMarkPaymentOrderWebhookReceived(t *testing.T) {
	db := setupPaymentTestDB(t)
	defer db.Close()

	order := &PaymentOrder{
		UserID:    "test_user_1",
		PackageID: "pkg_test",
		Amount:    10.00,
		Credits:   600,
		Status:    PaymentStatusPending,
	}
	err := db.CreatePaymentOrder(order)
	require.NoError(t, err)

	crossmintID := "crossmint_webhook_test"
	err = db.UpdatePaymentOrderWithCrossmintID(order.ID, crossmintID, "secret")
	require.NoError(t, err)

	t.Run("Mark Webhook Received", func(t *testing.T) {
		err := db.MarkPaymentOrderWebhookReceived(crossmintID)
		assert.NoError(t, err)

		retrieved, err := db.GetPaymentOrderByCrossmintID(crossmintID)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved.WebhookReceivedAt, "WebhookReceivedAt应该被设置")
	})
}

// TestGetUserPaymentOrders 测试查询用户订单列表
func TestGetUserPaymentOrders(t *testing.T) {
	db := setupPaymentTestDB(t)
	defer db.Close()

	// 创建多个测试订单
	for i := 0; i < 5; i++ {
		order := &PaymentOrder{
			UserID:    "test_user_1",
			PackageID: "pkg_test",
			Amount:    10.00 * float64(i+1),
			Credits:   100 * (i + 1),
			Status:    PaymentStatusPending,
		}
		err := db.CreatePaymentOrder(order)
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond) // 确保created_at不同
	}

	t.Run("Get All Orders", func(t *testing.T) {
		orders, total, err := db.GetUserPaymentOrders("test_user_1", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 5, total)
		assert.Len(t, orders, 5)
		// 应该按created_at降序排列
		assert.True(t, orders[0].Amount >= orders[4].Amount)
	})

	t.Run("Pagination", func(t *testing.T) {
		// 第一页
		orders, total, err := db.GetUserPaymentOrders("test_user_1", 1, 2)
		assert.NoError(t, err)
		assert.Equal(t, 5, total)
		assert.Len(t, orders, 2)

		// 第二页
		orders2, total2, err := db.GetUserPaymentOrders("test_user_1", 2, 2)
		assert.NoError(t, err)
		assert.Equal(t, 5, total2)
		assert.Len(t, orders2, 2)

		// 确保不同页的订单不同
		assert.NotEqual(t, orders[0].ID, orders2[0].ID)
	})

	t.Run("Empty Result", func(t *testing.T) {
		orders, total, err := db.GetUserPaymentOrders("non_existing_user", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Len(t, orders, 0)
	})
}

// TestCrossmintWebhookEventParsing 测试webhook事件解析
func TestCrossmintWebhookEventParsing(t *testing.T) {
	t.Run("Parse order.paid Event", func(t *testing.T) {
		jsonData := `{
			"type": "order.paid",
			"data": {
				"orderId": "order_abc123",
				"status": "paid",
				"amount": "10.00",
				"currency": "USDT",
				"metadata": {
					"packageId": "pkg_starter",
					"credits": 500,
					"userId": "user_xyz"
				},
				"paidAt": "2025-12-28T13:45:00Z"
			}
		}`

		var event CrossmintWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		assert.NoError(t, err)
		assert.Equal(t, "order.paid", event.Type)
		assert.Equal(t, "order_abc123", event.Data.OrderID)
		assert.Equal(t, "paid", event.Data.Status)
		assert.Equal(t, "10.00", event.Data.Amount)
		assert.Equal(t, "pkg_starter", event.Data.Metadata.PackageID)
		assert.Equal(t, 500, event.Data.Metadata.Credits)
		assert.Equal(t, "user_xyz", event.Data.Metadata.UserID)
	})

	t.Run("Parse order.failed Event", func(t *testing.T) {
		jsonData := `{
			"type": "order.failed",
			"data": {
				"orderId": "order_def456",
				"status": "failed",
				"amount": "10.00",
				"currency": "USDT",
				"metadata": {
					"packageId": "pkg_starter",
					"credits": 500,
					"userId": "user_abc"
				}
			}
		}`

		var event CrossmintWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		assert.NoError(t, err)
		assert.Equal(t, "order.failed", event.Type)
		assert.Equal(t, "order_def456", event.Data.OrderID)
		assert.Equal(t, "failed", event.Data.Status)
	})
}
