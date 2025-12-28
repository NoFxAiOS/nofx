package payment

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nofx/config"
	"nofx/service/payment"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestHandler 创建测试用HTTP处理器
func setupTestHandler(t *testing.T) (*Handler, *gin.Engine, *config.Database) {
	// 创建内存数据库
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	schema := `
	CREATE TABLE users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL
	);

	CREATE TABLE credit_packages (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		price_usdt REAL NOT NULL,
		credits INTEGER NOT NULL,
		bonus_credits INTEGER DEFAULT 0,
		is_active BOOLEAN DEFAULT TRUE
	);

	CREATE TABLE payment_orders (
		id TEXT PRIMARY KEY,
		crossmint_order_id TEXT UNIQUE,
		user_id TEXT NOT NULL,
		package_id TEXT NOT NULL,
		amount REAL NOT NULL,
		currency TEXT DEFAULT 'USDT',
		credits INTEGER NOT NULL,
		status TEXT DEFAULT 'pending',
		payment_method TEXT,
		crossmint_client_secret TEXT,
		webhook_received_at DATETIME,
		completed_at DATETIME,
		failed_reason TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE user_credits (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL UNIQUE,
		available_credits INTEGER DEFAULT 0,
		total_credits INTEGER DEFAULT 0,
		used_credits INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err)

	// 插入测试数据
	_, err = db.Exec(`INSERT INTO users (id, email, password_hash) VALUES ('user1', 'test@example.com', 'hash')`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO credit_packages (id, name, price_usdt, credits, bonus_credits, is_active)
		VALUES ('pkg1', 'Test Package', 10.00, 500, 100, 1)
	`)
	require.NoError(t, err)

	database := &config.Database{}
	// Note: Simplified for testing

	// 创建服务和处理器
	service := payment.NewPaymentService(database)
	handler := NewHandler(service)

	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)
	router := gin.New()

	return handler, router, database
}

// authMiddleware 简化的认证中间件（用于测试）
func authMiddleware(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}
}

// TestCreateOrder 测试创建订单接口
func TestCreateOrder(t *testing.T) {
	handler, router, db := setupTestHandler(t)
	defer db.Close()

	router.POST("/orders", authMiddleware("user1"), handler.CreateOrder)

	t.Run("Valid Order Creation", func(t *testing.T) {
		reqBody := CreateOrderRequest{
			PackageID: "pkg1",
		}
		jsonData, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp CreateOrderResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)
		assert.NotEmpty(t, resp.OrderID)
		assert.NotEmpty(t, resp.ClientSecret)
		assert.Equal(t, 10.00, resp.Amount)
		assert.Equal(t, 600, resp.Credits)
	})

	t.Run("Missing PackageID", func(t *testing.T) {
		reqBody := CreateOrderRequest{}
		jsonData, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp CreateOrderResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Equal(t, "INVALID_REQUEST", resp.Code)
	})

	t.Run("Invalid PackageID", func(t *testing.T) {
		reqBody := CreateOrderRequest{
			PackageID: "non_existing_pkg",
		}
		jsonData, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp CreateOrderResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Equal(t, "ORDER_CREATION_FAILED", resp.Code)
	})

	t.Run("Unauthorized Request", func(t *testing.T) {
		routerNoAuth := gin.New()
		routerNoAuth.POST("/orders", handler.CreateOrder)

		reqBody := CreateOrderRequest{PackageID: "pkg1"}
		jsonData, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		routerNoAuth.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestGetOrder 测试查询订单接口
func TestGetOrder(t *testing.T) {
	handler, router, db := setupTestHandler(t)
	defer db.Close()

	router.GET("/orders/:id", authMiddleware("user1"), handler.GetOrder)

	// 先创建一个订单
	service := payment.NewPaymentService(db)
	order, err := service.CreatePaymentOrder(router.(*gin.Context).Request.Context(), "user1", "pkg1")
	require.NoError(t, err)

	t.Run("Get Existing Order", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders/"+order.ID, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp GetOrderResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, order.ID, resp.Order.ID)
		// 敏感信息应该被隐藏
		assert.Empty(t, resp.Order.CrossmintClientSecret)
	})

	t.Run("Get Non-existing Order", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders/non_existing_id", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var resp GetOrderResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Equal(t, "ORDER_NOT_FOUND", resp.Code)
	})

	t.Run("Access Other User's Order", func(t *testing.T) {
		// 使用不同用户的认证
		routerOtherUser := gin.New()
		routerOtherUser.GET("/orders/:id", authMiddleware("user2"), handler.GetOrder)

		req := httptest.NewRequest("GET", "/orders/"+order.ID, nil)
		w := httptest.NewRecorder()

		routerOtherUser.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var resp GetOrderResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Equal(t, "FORBIDDEN", resp.Code)
	})
}

// TestGetUserOrders 测试查询用户订单列表
func TestGetUserOrders(t *testing.T) {
	handler, router, db := setupTestHandler(t)
	defer db.Close()

	router.GET("/orders", authMiddleware("user1"), handler.GetUserOrders)

	// 创建多个订单
	service := payment.NewPaymentService(db)
	for i := 0; i < 3; i++ {
		_, err := service.CreatePaymentOrder(router.(*gin.Context).Request.Context(), "user1", "pkg1")
		require.NoError(t, err)
	}

	t.Run("Get All Orders", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp GetOrdersResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, 3, resp.Total)
		assert.Len(t, resp.Orders, 3)

		// 验证敏感信息被隐藏
		for _, order := range resp.Orders {
			assert.Empty(t, order.CrossmintClientSecret)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders?page=1&limit=2", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp GetOrdersResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, 3, resp.Total)
		assert.Len(t, resp.Orders, 2)
		assert.Equal(t, 1, resp.Page)
		assert.Equal(t, 2, resp.Limit)
	})

	t.Run("Default Pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp GetOrdersResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, 1, resp.Page)
		assert.Equal(t, 20, resp.Limit)
	})
}

// TestHandleWebhook 测试webhook处理接口
func TestHandleWebhook(t *testing.T) {
	handler, router, db := setupTestHandler(t)
	defer db.Close()

	router.POST("/webhook", handler.HandleWebhook)

	// 创建测试订单
	service := payment.NewPaymentService(db)
	order, err := service.CreatePaymentOrder(router.(*gin.Context).Request.Context(), "user1", "pkg1")
	require.NoError(t, err)

	// 更新订单关联Crossmint ID
	err = db.UpdatePaymentOrderWithCrossmintID(order.ID, "crossmint_123", "secret")
	require.NoError(t, err)

	t.Run("Valid Webhook", func(t *testing.T) {
		webhookData := map[string]interface{}{
			"type": "order.paid",
			"data": map[string]interface{}{
				"orderId":  "crossmint_123",
				"status":   "paid",
				"amount":   "10.00",
				"currency": "USDT",
				"metadata": map[string]interface{}{
					"packageId": "pkg1",
					"credits":   600,
					"userId":    "user1",
				},
			},
		}
		jsonData, _ := json.Marshal(webhookData)

		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Crossmint-Signature", "test_signature")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp["success"].(bool))
		assert.True(t, resp["received"].(bool))
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Non-existing Order", func(t *testing.T) {
		webhookData := map[string]interface{}{
			"type": "order.paid",
			"data": map[string]interface{}{
				"orderId": "non_existing_crossmint_id",
				"status":  "paid",
			},
		}
		jsonData, _ := json.Marshal(webhookData)

		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Alternative Signature Header", func(t *testing.T) {
		webhookData := map[string]interface{}{
			"type": "order.paid",
			"data": map[string]interface{}{
				"orderId":  "crossmint_123",
				"status":   "paid",
				"metadata": map[string]interface{}{
					"userId": "user1",
				},
			},
		}
		jsonData, _ := json.Marshal(webhookData)

		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Crossmint-Signature", "alternative_header")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// 应该能够处理替代的签名头格式
		assert.NotEqual(t, http.StatusBadRequest, w.Code)
	})
}

// TestRateLimiting 测试频率限制
func TestRateLimiting(t *testing.T) {
	handler, router, db := setupTestHandler(t)
	defer db.Close()

	// Note: 实际频率限制测试需要中间件支持
	// 这里仅测试基本功能

	router.POST("/orders", authMiddleware("user1"), handler.CreateOrder)

	reqBody := CreateOrderRequest{PackageID: "pkg1"}
	jsonData, _ := json.Marshal(reqBody)

	// 连续发送多个请求
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// 前几个请求应该成功
		if i < 3 {
			assert.True(t, w.Code < 500, "前几个请求应该成功")
		}
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	handler, router, db := setupTestHandler(t)
	defer db.Close()

	t.Run("Malformed Request Body", func(t *testing.T) {
		router.POST("/orders", authMiddleware("user1"), handler.CreateOrder)

		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString("{invalid"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Empty Request Body", func(t *testing.T) {
		router.POST("/orders", authMiddleware("user1"), handler.CreateOrder)

		req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer([]byte{}))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
