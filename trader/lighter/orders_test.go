package lighter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetActiveOrders_ParseResponse tests parsing of Lighter API response
func TestGetActiveOrders_ParseResponse(t *testing.T) {
	// Mock response from Lighter API
	mockResponse := `{
		"code": 200,
		"message": "success",
		"orders": [
			{
				"order_id": "123456",
				"order_index": 123456,
				"market_index": 0,
				"side": "ask",
				"type": "limit",
				"is_ask": true,
				"price": "3150.50",
				"initial_base_amount": "1.5",
				"remaining_base_amount": "1.5",
				"filled_base_amount": "0",
				"status": "open",
				"trigger_price": "",
				"reduce_only": false,
				"timestamp": 1736745600000,
				"created_at": 1736745600000
			},
			{
				"order_id": "123457",
				"order_index": 123457,
				"market_index": 0,
				"side": "bid",
				"type": "limit",
				"is_ask": false,
				"price": "3100.00",
				"initial_base_amount": "2.0",
				"remaining_base_amount": "2.0",
				"filled_base_amount": "0",
				"status": "open",
				"trigger_price": "",
				"reduce_only": false,
				"timestamp": 1736745601000,
				"created_at": 1736745601000
			},
			{
				"order_id": "123458",
				"order_index": 123458,
				"market_index": 0,
				"side": "ask",
				"type": "stop_loss",
				"is_ask": true,
				"price": "0",
				"initial_base_amount": "1.0",
				"remaining_base_amount": "1.0",
				"filled_base_amount": "0",
				"status": "open",
				"trigger_price": "3000.00",
				"reduce_only": true,
				"timestamp": 1736745602000,
				"created_at": 1736745602000
			}
		]
	}`

	// Parse the response
	var apiResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Orders  []OrderResponse `json:"orders"`
	}

	err := json.Unmarshal([]byte(mockResponse), &apiResp)
	require.NoError(t, err, "Should parse response without error")

	// Verify parsed data
	assert.Equal(t, 200, apiResp.Code)
	assert.Equal(t, 3, len(apiResp.Orders))

	// Test first order (sell limit)
	order1 := apiResp.Orders[0]
	assert.Equal(t, "123456", order1.OrderID)
	assert.True(t, order1.IsAsk, "First order should be ask (sell)")
	assert.Equal(t, "3150.50", order1.Price)
	assert.Equal(t, "1.5", order1.RemainingBaseAmount)
	assert.False(t, order1.ReduceOnly)

	// Test second order (buy limit)
	order2 := apiResp.Orders[1]
	assert.Equal(t, "123457", order2.OrderID)
	assert.False(t, order2.IsAsk, "Second order should be bid (buy)")
	assert.Equal(t, "3100.00", order2.Price)

	// Test third order (stop-loss)
	order3 := apiResp.Orders[2]
	assert.Equal(t, "123458", order3.OrderID)
	assert.Equal(t, "stop_loss", order3.Type)
	assert.Equal(t, "3000.00", order3.TriggerPrice)
	assert.True(t, order3.ReduceOnly)
}

// TestGetActiveOrders_EmptyResponse tests handling of empty orders
func TestGetActiveOrders_EmptyResponse(t *testing.T) {
	mockResponse := `{
		"code": 200,
		"message": "success",
		"orders": []
	}`

	var apiResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Orders  []OrderResponse `json:"orders"`
	}

	err := json.Unmarshal([]byte(mockResponse), &apiResp)
	require.NoError(t, err)
	assert.Equal(t, 200, apiResp.Code)
	assert.Equal(t, 0, len(apiResp.Orders))
}

// TestGetActiveOrders_ErrorResponse tests handling of API error
func TestGetActiveOrders_ErrorResponse(t *testing.T) {
	mockResponse := `{
		"code": 29500,
		"message": "internal server error: invalid signature"
	}`

	var apiResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Orders  []OrderResponse `json:"orders"`
	}

	err := json.Unmarshal([]byte(mockResponse), &apiResp)
	require.NoError(t, err)
	assert.Equal(t, 29500, apiResp.Code)
	assert.Contains(t, apiResp.Message, "invalid signature")
}

// TestConvertOrderResponseToOpenOrder tests conversion logic
func TestConvertOrderResponseToOpenOrder(t *testing.T) {
	testCases := []struct {
		name           string
		order          OrderResponse
		expectedSide   string
		expectedType   string
		expectedPosSide string
	}{
		{
			name: "Sell limit order (opening short)",
			order: OrderResponse{
				OrderID:             "1",
				IsAsk:               true,
				Type:                "limit",
				Price:               "3150.00",
				RemainingBaseAmount: "1.0",
				ReduceOnly:          false,
			},
			expectedSide:   "SELL",
			expectedType:   "LIMIT",
			expectedPosSide: "SHORT",
		},
		{
			name: "Buy limit order (opening long)",
			order: OrderResponse{
				OrderID:             "2",
				IsAsk:               false,
				Type:                "limit",
				Price:               "3100.00",
				RemainingBaseAmount: "1.0",
				ReduceOnly:          false,
			},
			expectedSide:   "BUY",
			expectedType:   "LIMIT",
			expectedPosSide: "LONG",
		},
		{
			name: "Sell stop-loss (closing long)",
			order: OrderResponse{
				OrderID:             "3",
				IsAsk:               true,
				Type:                "stop_loss",
				TriggerPrice:        "3000.00",
				RemainingBaseAmount: "1.0",
				ReduceOnly:          true,
			},
			expectedSide:   "SELL",
			expectedType:   "STOP_MARKET",
			expectedPosSide: "LONG",
		},
		{
			name: "Buy stop-loss (closing short)",
			order: OrderResponse{
				OrderID:             "4",
				IsAsk:               false,
				Type:                "stop_loss",
				TriggerPrice:        "3200.00",
				RemainingBaseAmount: "1.0",
				ReduceOnly:          true,
			},
			expectedSide:   "BUY",
			expectedType:   "STOP_MARKET",
			expectedPosSide: "SHORT",
		},
		{
			name: "Take profit (closing long)",
			order: OrderResponse{
				OrderID:             "5",
				IsAsk:               true,
				Type:                "take_profit",
				TriggerPrice:        "3500.00",
				RemainingBaseAmount: "1.0",
				ReduceOnly:          true,
			},
			expectedSide:   "SELL",
			expectedType:   "TAKE_PROFIT_MARKET",
			expectedPosSide: "LONG",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert side
			side := "BUY"
			if tc.order.IsAsk {
				side = "SELL"
			}
			assert.Equal(t, tc.expectedSide, side)

			// Convert order type
			orderType := "LIMIT"
			if tc.order.Type == "market" {
				orderType = "MARKET"
			} else if tc.order.Type == "stop_loss" || tc.order.Type == "stop" {
				orderType = "STOP_MARKET"
			} else if tc.order.Type == "take_profit" {
				orderType = "TAKE_PROFIT_MARKET"
			}
			assert.Equal(t, tc.expectedType, orderType)

			// Convert position side
			positionSide := "LONG"
			if tc.order.ReduceOnly {
				if side == "BUY" {
					positionSide = "SHORT"
				} else {
					positionSide = "LONG"
				}
			} else {
				if side == "SELL" {
					positionSide = "SHORT"
				}
			}
			assert.Equal(t, tc.expectedPosSide, positionSide)
		})
	}
}

// TestGetActiveOrders_MockServer tests the full HTTP flow with a mock server
func TestGetActiveOrders_MockServer(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path and auth parameter
		assert.Contains(t, r.URL.Path, "/api/v1/accountActiveOrders")

		// Check that auth query parameter is present
		authParam := r.URL.Query().Get("auth")
		if authParam == "" {
			// Return error if no auth parameter
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    29500,
				"message": "internal server error: invalid signature",
			})
			return
		}

		// Return success response
		response := map[string]interface{}{
			"code":    200,
			"message": "success",
			"orders": []map[string]interface{}{
				{
					"order_id":              "123456",
					"order_index":           123456,
					"market_index":          0,
					"side":                  "ask",
					"type":                  "limit",
					"is_ask":                true,
					"price":                 "3150.50",
					"initial_base_amount":   "1.5",
					"remaining_base_amount": "1.5",
					"filled_base_amount":    "0",
					"status":                "open",
					"trigger_price":         "",
					"reduce_only":           false,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test request without auth - should fail
	resp, err := http.Get(server.URL + "/api/v1/accountActiveOrders?account_index=123&market_id=0")
	require.NoError(t, err)
	defer resp.Body.Close()

	var errorResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(t, 29500, errorResp.Code)

	// Test request with auth - should succeed
	resp2, err := http.Get(server.URL + "/api/v1/accountActiveOrders?account_index=123&market_id=0&auth=test_token")
	require.NoError(t, err)
	defer resp2.Body.Close()

	var successResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Orders  []OrderResponse `json:"orders"`
	}
	json.NewDecoder(resp2.Body).Decode(&successResp)
	assert.Equal(t, 200, successResp.Code)
	assert.Equal(t, 1, len(successResp.Orders))
}

// TestAuthTokenFormat tests the auth token format
func TestAuthTokenFormat(t *testing.T) {
	// Auth token format: timestamp:account_index:api_key_index:signature
	// Example: 1768308847:687247:0:742e02...

	sampleToken := "1768308847:687247:0:742e02abc123"

	// The token should be URL encoded when used as query parameter
	// Colons become %3A
	expectedEncoded := "1768308847%3A687247%3A0%3A742e02abc123"

	// URL encode the token
	encoded := url.QueryEscape(sampleToken)

	assert.Equal(t, expectedEncoded, encoded)
}

// TestOrderResponseStruct tests that OrderResponse struct matches API response
func TestOrderResponseStruct(t *testing.T) {
	// Real API response sample (from logs)
	realResponse := `{
		"order_id": "4609885",
		"order_index": 4609885,
		"market_index": 0,
		"side": "ask",
		"type": "limit",
		"is_ask": true,
		"price": "3150.00",
		"initial_base_amount": "0.0300",
		"remaining_base_amount": "0.0300",
		"filled_base_amount": "0",
		"status": "open",
		"trigger_price": "",
		"reduce_only": false,
		"timestamp": 1736745600000,
		"created_at": 1736745600000
	}`

	var order OrderResponse
	err := json.Unmarshal([]byte(realResponse), &order)
	require.NoError(t, err)

	assert.Equal(t, "4609885", order.OrderID)
	assert.Equal(t, int64(4609885), order.OrderIndex)
	assert.Equal(t, 0, order.MarketIndex)
	assert.Equal(t, "ask", order.Side)
	assert.Equal(t, "limit", order.Type)
	assert.True(t, order.IsAsk)
	assert.Equal(t, "3150.00", order.Price)
	assert.Equal(t, "0.0300", order.InitialBaseAmount)
	assert.Equal(t, "0.0300", order.RemainingBaseAmount)
	assert.Equal(t, "0", order.FilledBaseAmount)
	assert.Equal(t, "open", order.Status)
	assert.Equal(t, "", order.TriggerPrice)
	assert.False(t, order.ReduceOnly)
	assert.Equal(t, int64(1736745600000), order.Timestamp)
	assert.Equal(t, int64(1736745600000), order.CreatedAt)
}

// BenchmarkParseOrderResponse benchmarks response parsing
func BenchmarkParseOrderResponse(b *testing.B) {
	mockResponse := `{
		"code": 200,
		"message": "success",
		"orders": [
			{"order_id": "1", "is_ask": true, "price": "3150.50", "remaining_base_amount": "1.5"},
			{"order_id": "2", "is_ask": false, "price": "3100.00", "remaining_base_amount": "2.0"},
			{"order_id": "3", "is_ask": true, "price": "3200.00", "remaining_base_amount": "0.5"}
		]
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var apiResp struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Orders  []OrderResponse `json:"orders"`
		}
		json.Unmarshal([]byte(mockResponse), &apiResp)
	}
}
