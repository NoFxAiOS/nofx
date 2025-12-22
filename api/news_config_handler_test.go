package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"nofx/database"
)

// 错误定义
var (
	ErrorConfigNotFound      = errors.New("配置不存在")
	ErrorConfigAlreadyExists = errors.New("配置已存在")
)

// MockNewsConfigRepository 用于测试的mock repository
type MockNewsConfigRepository struct {
	configs map[string]*database.UserNewsConfig
}

func NewMockNewsConfigRepository() *MockNewsConfigRepository {
	return &MockNewsConfigRepository{
		configs: make(map[string]*database.UserNewsConfig),
	}
}

func (m *MockNewsConfigRepository) GetByUserID(userID string) (*database.UserNewsConfig, error) {
	config, exists := m.configs[userID]
	if !exists {
		return nil, ErrorConfigNotFound
	}
	return config, nil
}

func (m *MockNewsConfigRepository) Create(config *database.UserNewsConfig) error {
	if _, exists := m.configs[config.UserID]; exists {
		return ErrorConfigAlreadyExists
	}
	config.ID = len(m.configs) + 1
	m.configs[config.UserID] = config
	return nil
}

func (m *MockNewsConfigRepository) Update(config *database.UserNewsConfig) error {
	if _, exists := m.configs[config.UserID]; !exists {
		return m.Create(config)
	}
	m.configs[config.UserID] = config
	return nil
}

func (m *MockNewsConfigRepository) Delete(userID string) error {
	if _, exists := m.configs[userID]; !exists {
		return ErrorConfigNotFound
	}
	delete(m.configs, userID)
	return nil
}

func (m *MockNewsConfigRepository) GetOrCreateDefault(userID string) (*database.UserNewsConfig, error) {
	config, exists := m.configs[userID]
	if exists {
		return config, nil
	}
	newConfig := &database.UserNewsConfig{
		UserID:                   userID,
		Enabled:                  false,
		NewsSources:              "mlion",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:      10,
		SentimentThreshold:       0.0,
	}
	m.Create(newConfig)
	return newConfig, nil
}

func (m *MockNewsConfigRepository) ListAllEnabled() ([]database.UserNewsConfig, error) {
	var result []database.UserNewsConfig
	for _, config := range m.configs {
		if config.Enabled {
			result = append(result, *config)
		}
	}
	return result, nil
}

// TestGetUserNewsConfig_Success 测试成功获取用户配置
func TestGetUserNewsConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建mock repository并添加测试数据
	mockRepo := NewMockNewsConfigRepository()
	config := &database.UserNewsConfig{
		ID:                       1,
		UserID:                   "test-user-001",
		Enabled:                  true,
		NewsSources:              "mlion,twitter",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:      10,
		SentimentThreshold:       0.5,
	}
	mockRepo.Create(config)

	handler := NewNewsConfigHandler(mockRepo)

	// 创建带有user_id的请求
	req, _ := http.NewRequest("GET", "/api/user/news-config", nil)
	w := httptest.NewRecorder()

	// 设置context中的user_id
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("user_id", "test-user-001")

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

// TestGetUserNewsConfig_NotFound 测试获取不存在的配置
func TestGetUserNewsConfig_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := NewMockNewsConfigRepository()
	handler := NewNewsConfigHandler(mockRepo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "non-existent-user")

	handler.GetUserNewsConfig(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码404，得到%d", w.Code)
	}
}

// TestCreateOrUpdateUserNewsConfig_Create 测试创建新配置
func TestCreateOrUpdateUserNewsConfig_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := NewMockNewsConfigRepository()
	handler := NewNewsConfigHandler(mockRepo)

	reqBody := CreateOrUpdateUserNewsConfigRequest{
		Enabled:                  ptrBool(true),
		NewsSources:              ptrString("mlion,twitter"),
		AutoFetchIntervalMinutes: ptrInt(10),
		MaxArticlesPerFetch:      ptrInt(20),
		SentimentThreshold:       ptrFloat(0.5),
	}

	bodyBytes, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/user/news-config", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", "new-user-123")

	handler.CreateOrUpdateUserNewsConfig(c)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码201，得到%d", w.Code)
	}

	// 验证配置已创建
	config, err := mockRepo.GetByUserID("new-user-123")
	if err != nil {
		t.Errorf("配置创建失败: %v", err)
	}
	if config.NewsSources != "mlion,twitter" {
		t.Errorf("期望NewsSources为mlion,twitter，得到%s", config.NewsSources)
	}
}

// TestCreateOrUpdateUserNewsConfig_InvalidSource 测试无效的新闻源
func TestCreateOrUpdateUserNewsConfig_InvalidSource(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := NewMockNewsConfigRepository()
	handler := NewNewsConfigHandler(mockRepo)

	reqBody := CreateOrUpdateUserNewsConfigRequest{
		NewsSources: ptrString("invalid-source,another-invalid"),
	}

	bodyBytes, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/user/news-config", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", "user-456")

	handler.CreateOrUpdateUserNewsConfig(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400，得到%d", w.Code)
	}
}

// TestDeleteUserNewsConfig_Success 测试成功删除配置
func TestDeleteUserNewsConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := NewMockNewsConfigRepository()
	config := &database.UserNewsConfig{
		UserID:     "delete-test-user",
		Enabled:    true,
		NewsSources: "mlion",
	}
	mockRepo.Create(config)

	handler := NewNewsConfigHandler(mockRepo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/api/user/news-config", nil)
	c.Set("user_id", "delete-test-user")

	handler.DeleteUserNewsConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200，得到%d", w.Code)
	}

	// 验证配置已删除
	_, err := mockRepo.GetByUserID("delete-test-user")
	if err == nil {
		t.Error("配置应该已删除")
	}
}

// TestGetEnabledNewsSources_Success 测试获取启用的新闻源
func TestGetEnabledNewsSources_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := NewMockNewsConfigRepository()
	config := &database.UserNewsConfig{
		UserID:      "sources-test-user",
		NewsSources: "mlion, twitter, reddit",
	}
	mockRepo.Create(config)

	handler := NewNewsConfigHandler(mockRepo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/user/news-config/sources", nil)
	c.Set("user_id", "sources-test-user")

	handler.GetEnabledNewsSources(c)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200，得到%d", w.Code)
	}

	var response APIResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	dataMap, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Error("响应数据格式不正确")
	}

	sources, ok := dataMap["sources"].([]interface{})
	if !ok || len(sources) != 3 {
		t.Errorf("期望3个新闻源，得到%v", sources)
	}
}

// ===== 辅助函数 =====

func ptrBool(b bool) *bool {
	return &b
}

func ptrString(s string) *string {
	return &s
}

func ptrInt(i int) *int {
	return &i
}

func ptrFloat(f float64) *float64 {
	return &f
}
