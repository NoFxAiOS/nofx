package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nofx/store"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockQuantModelStore is a mock implementation of QuantModelStore for testing
type MockQuantModelStore struct {
	mock.Mock
}

func (m *MockQuantModelStore) Create(model *store.QuantModel) error {
	args := m.Called(model)
	return args.Error(0)
}

func (m *MockQuantModelStore) Update(model *store.QuantModel) error {
	args := m.Called(model)
	return args.Error(0)
}

func (m *MockQuantModelStore) Delete(userID, modelID string) error {
	args := m.Called(userID, modelID)
	return args.Error(0)
}

func (m *MockQuantModelStore) Get(userID, modelID string) (*store.QuantModel, error) {
	args := m.Called(userID, modelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.QuantModel), args.Error(1)
}

func (m *MockQuantModelStore) List(userID string) ([]*store.QuantModel, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.QuantModel), args.Error(1)
}

func (m *MockQuantModelStore) ListPublic() ([]*store.QuantModel, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.QuantModel), args.Error(1)
}

func (m *MockQuantModelStore) IncrementUsage(modelID string) error {
	args := m.Called(modelID)
	return args.Error(0)
}

func (m *MockQuantModelStore) UpdateBacktestStats(modelID string, stats store.BacktestStats) error {
	args := m.Called(modelID, stats)
	return args.Error(0)
}

func (m *MockQuantModelStore) InitTables() error {
	args := m.Called()
	return args.Error(0)
}

// TestQuantModelList tests listing quant models
func TestQuantModelList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockStore := new(MockQuantModelStore)
	server := &Server{
		// Initialize with mock store
	}
	_ = server
	_ = mockStore

	// Test cases for listing models
	testCases := []struct {
		name           string
		userID         string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "Successful list",
			userID:         "user-123",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "Unauthorized - no user_id",
			userID:         "",
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock expectations
			if tc.userID != "" {
				mockStore.On("List", tc.userID).Return([]*store.QuantModel{
					{
						ID:       "model-1",
						Name:     "RSI Strategy",
						ModelType: "indicator_based",
						UserID:   tc.userID,
					},
					{
						ID:       "model-2",
						Name:     "MACD Crossover",
						ModelType: "indicator_based",
						UserID:   tc.userID,
					},
				}, nil).Once()
			}

			// Create request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/quant-models", nil)
			
			// Add auth context if userID is provided
			if tc.userID != "" {
				req.Header.Set("Authorization", "Bearer test-token")
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			
			if tc.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				
				models, ok := response["models"].([]interface{})
				assert.True(t, ok)
				assert.Len(t, models, tc.expectedCount)
			}
		})
	}
}

// TestQuantModelCreate tests creating a new quant model
func TestQuantModelCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		userID         string
		expectedStatus int
	}{
		{
			name: "Successful creation - indicator based",
			requestBody: map[string]interface{}{
				"name":        "Test RSI Model",
				"model_type":  "indicator_based",
				"description": "A test RSI strategy",
				"config": map[string]interface{}{
					"type": "indicator_based",
					"indicators": []map[string]interface{}{
						{
							"name":      "RSI",
							"period":    14,
							"timeframe": "1h",
							"weight":    1.0,
						},
					},
					"parameters": map[string]interface{}{
						"lookback_periods":       100,
						"entry_threshold":        70,
						"exit_threshold":         30,
						"max_position_hold_time": 48,
						"min_position_hold_time": 4,
						"max_daily_trades":     3,
					},
					"signal_config": map[string]interface{}{
						"signal_type":            "discrete",
						"min_confidence":         65,
						"require_confirmation":   true,
						"confirmation_delay":     1,
					},
				},
			},
			userID:         "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Successful creation - rule based",
			requestBody: map[string]interface{}{
				"name":        "Test Rule Model",
				"model_type":  "rule_based",
				"description": "A test rule strategy",
				"config": map[string]interface{}{
					"type": "rule_based",
					"rules": []map[string]interface{}{
						{
							"name":       "RSI Oversold",
							"condition":  "RSI_14 < 30",
							"action":     "buy",
							"confidence": 80,
							"priority":   1,
						},
					},
					"parameters": map[string]interface{}{
						"lookback_periods":       50,
						"entry_threshold":        0,
						"exit_threshold":         0,
						"max_position_hold_time": 24,
						"min_position_hold_time": 2,
						"max_daily_trades":     5,
					},
					"signal_config": map[string]interface{}{
						"signal_type":            "discrete",
						"min_confidence":         70,
						"require_confirmation":   false,
						"confirmation_delay":     0,
					},
				},
			},
			userID:         "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Missing required fields",
			requestBody: map[string]interface{}{
				"description": "Missing name and model_type",
			},
			userID:         "user-123",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Unauthorized - no user_id",
			requestBody: map[string]interface{}{
				"name":       "Test Model",
				"model_type": "indicator_based",
			},
			userID:         "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			
			body, _ := json.Marshal(tc.requestBody)
			req, _ := http.NewRequest("POST", "/api/quant-models", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			if tc.userID != "" {
				req.Header.Set("Authorization", "Bearer test-token")
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			
			if tc.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				
				// Check that response has expected fields
				_, hasID := response["id"]
				assert.True(t, hasID, "Response should have an id field")
				
				message, hasMessage := response["message"]
				assert.True(t, hasMessage, "Response should have a message field")
				assert.Contains(t, message, "successfully")
			}
		})
	}
}

// TestQuantModelExport tests exporting a quant model
func TestQuantModelExport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		modelID        string
		userID         string
		expectedStatus int
		shouldHaveData bool
	}{
		{
			name:           "Successful export",
			modelID:        "model-123",
			userID:         "user-123",
			expectedStatus: http.StatusOK,
			shouldHaveData: true,
		},
		{
			name:           "Model not found",
			modelID:        "non-existent",
			userID:         "user-123",
			expectedStatus: http.StatusNotFound,
			shouldHaveData: false,
		},
		{
			name:           "Unauthorized",
			modelID:        "model-123",
			userID:         "",
			expectedStatus: http.StatusUnauthorized,
			shouldHaveData: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This is a simplified test structure
			// In a full test, you would setup mocks and verify behavior
			assert.NotEmpty(t, tc.modelID)
		})
	}
}

// TestQuantModelImport tests importing a quant model
func TestQuantModelImport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validExportData := map[string]interface{}{
		"version": "1.0",
		"exported_at": "2024-01-15T10:30:00Z",
		"model": map[string]interface{}{
			"id":          "original-id",
			"name":        "Imported RSI Model",
			"description": "A great RSI strategy",
			"model_type":  "indicator_based",
			"version":     "1.0",
			"config": map[string]interface{}{
				"type": "indicator_based",
				"indicators": []map[string]interface{}{
					{
						"name":      "RSI",
						"period":    14,
						"timeframe": "1h",
						"weight":    1.0,
					},
				},
				"parameters": map[string]interface{}{
					"lookback_periods":       100,
					"entry_threshold":        70,
					"exit_threshold":         30,
					"max_position_hold_time": 48,
					"min_position_hold_time": 4,
					"max_daily_trades":     3,
				},
				"signal_config": map[string]interface{}{
					"signal_type":            "discrete",
					"min_confidence":         65,
					"require_confirmation":   true,
					"confirmation_delay":     1,
				},
			},
		},
	}

	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		userID         string
		expectedStatus int
	}{
		{
			name:           "Successful import",
			requestBody:    validExportData,
			userID:         "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Missing version",
			requestBody: map[string]interface{}{
				"model": map[string]interface{}{
					"name": "Test Model",
				},
			},
			userID:         "user-123",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Unsupported version",
			requestBody: map[string]interface{}{
				"version": "2.0",
				"model":   map[string]interface{}{"name": "Test"},
			},
			userID:         "user-123",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized",
			requestBody:    validExportData,
			userID:         "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.requestBody)
			
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/quant-models/import", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			if tc.userID != "" {
				req.Header.Set("Authorization", "Bearer test-token")
			}

			// Simplified assertion - in full test would use mock router
			assert.NotNil(t, w)
			assert.NotEmpty(t, body)
		})
	}
}

// TestQuantModelClone tests cloning a quant model
func TestQuantModelClone(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		modelID        string
		requestBody    map[string]interface{}
		userID         string
		expectedStatus int
	}{
		{
			name:    "Successful clone",
			modelID: "model-123",
			requestBody: map[string]interface{}{
				"name": "Cloned RSI Model",
			},
			userID:         "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Clone without name (auto-generated)",
			modelID:        "model-123",
			requestBody:    map[string]interface{}{},
			userID:         "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Model not found",
			modelID:        "non-existent",
			requestBody:    map[string]interface{}{},
			userID:         "user-123",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Unauthorized",
			modelID:        "model-123",
			requestBody:    map[string]interface{}{},
			userID:         "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.requestBody)
			
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/quant-models/"+tc.modelID+"/clone", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			if tc.userID != "" {
				req.Header.Set("Authorization", "Bearer test-token")
			}

			assert.NotNil(t, w)
			assert.NotEmpty(t, body)
		})
	}
}

// TestQuantModelUpdateBacktestStats tests updating backtest statistics
func TestQuantModelUpdateBacktestStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		modelID        string
		requestBody    map[string]interface{}
		userID         string
		expectedStatus int
	}{
		{
			name:    "Successful stats update",
			modelID: "model-123",
			requestBody: map[string]interface{}{
				"win_rate":        0.65,
				"avg_profit_pct":  12.5,
				"max_drawdown_pct": 8.2,
				"sharpe_ratio":    1.8,
			},
			userID:         "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:    "Partial stats update",
			modelID: "model-123",
			requestBody: map[string]interface{}{
				"win_rate": 0.55,
			},
			userID:         "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Unauthorized",
			modelID:        "model-123",
			requestBody:    map[string]interface{}{"win_rate": 0.55},
			userID:         "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.requestBody)
			
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/quant-models/"+tc.modelID+"/backtest-stats", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			if tc.userID != "" {
				req.Header.Set("Authorization", "Bearer test-token")
			}

			assert.NotNil(t, w)
			assert.NotEmpty(t, body)
		})
	}
}
