package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"nofx/database"
)

// IntegrationTestHelper 集成测试辅助函数
type IntegrationTestHelper struct {
	router *gin.Engine
	repo   *database.UserNewsConfigRepository
	token  string
	userID string
}

// SetupIntegrationTest 设置集成测试
func SetupIntegrationTest(t *testing.T) *IntegrationTestHelper {
	gin.SetMode(gin.TestMode)

	// 创建router
	router := gin.New()

	// 创建mock repository
	mockRepo := NewMockNewsConfigRepository()
	handler := NewNewsConfigHandler(mockRepo)

	// 注册路由
	apiGroup := router.Group("/api")
	handler.RegisterRoutes(apiGroup)

	return &IntegrationTestHelper{
		router: router,
		repo:   nil, // 在实际应用中应该是真实的repo
		token:  "test-token",
		userID: "test-user",
	}
}

// TestAPI_CreateNewsConfig_Success 测试成功创建新闻配置
func TestAPI_CreateNewsConfig_Success(t *testing.T) {
	helper := SetupIntegrationTest(t)

	requestBody := map[string]interface{}{
		"enabled":                     true,
		"news_sources":                "mlion,twitter",
		"auto_fetch_interval_minutes": 10,
		"max_articles_per_fetch":      20,
		"sentiment_threshold":         0.5,
	}

	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(
		"POST",
		"/api/user/news-config",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+helper.token)

	w := httptest.NewRecorder()
	// 模拟context中的user_id
	c := createTestContext(w, req, helper.userID)

	// 获取handler
	handler := NewNewsConfigHandler(NewMockNewsConfigRepository())
	handler.CreateOrUpdateUserNewsConfig(c)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码201，得到%d", w.Code)
	}

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	if response.Code != 201 {
		t.Errorf("期望响应code为201，得到%d", response.Code)
	}
}

// TestAPI_GetNewsConfig_Success 测试成功获取新闻配置
func TestAPI_GetNewsConfig_Success(t *testing.T) {
	helper := SetupIntegrationTest(t)

	// 先创建配置
	mockRepo := NewMockNewsConfigRepository()
	config := &database.UserNewsConfig{
		UserID:                   "test-user",
		Enabled:                  true,
		NewsSources:              "mlion",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:      10,
		SentimentThreshold:       0.0,
	}
	mockRepo.Create(config)

	// 测试获取
	handler := NewNewsConfigHandler(mockRepo)

	req := httptest.NewRequest("GET", "/api/user/news-config", nil)
	req.Header.Set("Authorization", "Bearer " + helper.token)

	w := httptest.NewRecorder()
	c := createTestContext(w, req, helper.userID)
	handler.GetUserNewsConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200，得到%d", w.Code)
	}

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	if response.Code != 200 {
		t.Errorf("期望响应code为200，得到%d", response.Code)
	}
}

// TestAPI_UpdateNewsConfig_Success 测试成功更新新闻配置
func TestAPI_UpdateNewsConfig_Success(t *testing.T) {
	// 创建初始配置
	mockRepo := NewMockNewsConfigRepository()
	config := &database.UserNewsConfig{
		UserID:                   "test-user",
		Enabled:                  true,
		NewsSources:              "mlion",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:      10,
		SentimentThreshold:       0.0,
	}
	mockRepo.Create(config)

	// 更新配置
	updateBody := map[string]interface{}{
		"enabled":                     false,
		"news_sources":                "twitter,reddit",
		"auto_fetch_interval_minutes": 15,
		"max_articles_per_fetch":      25,
		"sentiment_threshold":         0.3,
	}

	body, _ := json.Marshal(updateBody)
	req := httptest.NewRequest(
		"PUT",
		"/api/user/news-config",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c := createTestContext(w, req, "test-user")

	handler := NewNewsConfigHandler(mockRepo)
	handler.UpdateUserNewsConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200，得到%d", w.Code)
	}

	// 验证更新
	updated, _ := mockRepo.GetByUserID("test-user")
	if updated.NewsSources != "twitter,reddit" {
		t.Errorf("期望新闻源为twitter,reddit，得到%s", updated.NewsSources)
	}
	if updated.AutoFetchIntervalMinutes != 15 {
		t.Errorf("期望间隔为15，得到%d", updated.AutoFetchIntervalMinutes)
	}
}

// TestAPI_DeleteNewsConfig_Success 测试成功删除新闻配置
func TestAPI_DeleteNewsConfig_Success(t *testing.T) {
	mockRepo := NewMockNewsConfigRepository()
	config := &database.UserNewsConfig{
		UserID:                   "test-user",
		Enabled:                  true,
		NewsSources:              "mlion",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:      10,
		SentimentThreshold:       0.0,
	}
	mockRepo.Create(config)

	req := httptest.NewRequest("DELETE", "/api/user/news-config", nil)
	w := httptest.NewRecorder()
	c := createTestContext(w, req, "test-user")

	handler := NewNewsConfigHandler(mockRepo)
	handler.DeleteUserNewsConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200，得到%d", w.Code)
	}

	// 验证删除
	_, err := mockRepo.GetByUserID("test-user")
	if err == nil {
		t.Error("配置应该已删除")
	}
}

// TestAPI_ValidationErrors 测试表单验证错误
func TestAPI_ValidationErrors(t *testing.T) {
	mockRepo := NewMockNewsConfigRepository()
	handler := NewNewsConfigHandler(mockRepo)

	tests := []struct {
		name       string
		body       map[string]interface{}
		expectCode int
	}{
		{
			name: "无效的新闻源",
			body: map[string]interface{}{
				"news_sources": "invalid-source",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "间隔过小",
			body: map[string]interface{}{
				"news_sources":                "mlion",
				"auto_fetch_interval_minutes": 0,
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "文章数过大",
			body: map[string]interface{}{
				"news_sources":           "mlion",
				"max_articles_per_fetch": 150,
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "情绪阈值超出范围",
			body: map[string]interface{}{
				"news_sources":        "mlion",
				"sentiment_threshold": 2.0,
			},
			expectCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, _ := json.Marshal(test.body)
			req := httptest.NewRequest(
				"POST",
				"/api/user/news-config",
				bytes.NewBuffer(body),
			)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c := createTestContext(w, req, "test-user")
			handler.CreateOrUpdateUserNewsConfig(c)

			if w.Code != test.expectCode {
				t.Errorf("期望状态码%d，得到%d", test.expectCode, w.Code)
			}
		})
	}
}

// TestAPI_UnauthorizedAccess 测试未授权访问
func TestAPI_UnauthorizedAccess(t *testing.T) {
	mockRepo := NewMockNewsConfigRepository()
	handler := NewNewsConfigHandler(mockRepo)

	req := httptest.NewRequest("GET", "/api/user/news-config", nil)
	w := httptest.NewRecorder()

	// 不设置user_id
	c := createTestContext(w, req, "")
	handler.GetUserNewsConfig(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码401，得到%d", w.Code)
	}
}

// ===== 辅助函数 =====

// createTestContext 创建测试context
func createTestContext(w http.ResponseWriter, r *http.Request, userID string) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Request = r
	if userID != "" {
		c.Set("user_id", userID)
	}
	return c
}
