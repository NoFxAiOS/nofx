package gate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"nofx/trader/testutil"
	"nofx/trader/types"
)

// ============================================================
// Part 1: GateTraderTestSuite - Inherits base test suite
// ============================================================

// GateTraderTestSuite Gate trader test suite
// Inherits TraderTestSuite and adds Gate-specific mock logic
type GateTraderTestSuite struct {
	*testutil.TraderTestSuite
	mockServer *httptest.Server
}

// NewGateTraderTestSuite creates Gate test suite with mock server
func NewGateTraderTestSuite(t *testing.T) *GateTraderTestSuite {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		var respBody interface{}

		switch {
		// Mock GetBalance - /api/v4/futures/usdt/accounts
		case strings.Contains(path, "/futures/usdt/accounts"):
			respBody = map[string]interface{}{
				"total":          "10000.00",
				"unrealised_pnl": "100.50",
				"available":      "8000.00",
				"currency":       "USDT",
			}

		// Mock GetPositions - /api/v4/futures/usdt/positions
		case strings.Contains(path, "/futures/usdt/positions"):
			respBody = []map[string]interface{}{
				{
					"contract":       "BTC_USDT",
					"size":           500,
					"entry_price":    "50000.00",
					"mark_price":     "50500.00",
					"unrealised_pnl": "250.00",
					"liq_price":      "45000.00",
					"leverage":       "10",
				},
			}

		// Mock GetContract - /api/v4/futures/usdt/contracts/{contract}
		case strings.Contains(path, "/futures/usdt/contracts/"):
			respBody = map[string]interface{}{
				"name":              "BTC_USDT",
				"quanto_multiplier": "0.001",
				"order_price_round": "0.1",
			}

		// Mock ListFuturesContracts - /api/v4/futures/usdt/contracts
		case strings.Contains(path, "/futures/usdt/contracts"):
			respBody = []map[string]interface{}{
				{
					"name":              "BTC_USDT",
					"quanto_multiplier": "0.001",
					"order_price_round": "0.1",
				},
				{
					"name":              "ETH_USDT",
					"quanto_multiplier": "0.01",
					"order_price_round": "0.01",
				},
			}

		// Mock ListFuturesTickers - /api/v4/futures/usdt/tickers
		case strings.Contains(path, "/futures/usdt/tickers"):
			contract := r.URL.Query().Get("contract")
			if contract == "" {
				contract = "BTC_USDT"
			}
			price := "50000.00"
			if contract == "ETH_USDT" {
				price = "3000.00"
			}
			respBody = []map[string]interface{}{
				{
					"contract": contract,
					"last":     price,
				},
			}

		// Mock CreateFuturesOrder - /api/v4/futures/usdt/orders (POST)
		case strings.Contains(path, "/futures/usdt/orders") && r.Method == "POST":
			respBody = map[string]interface{}{
				"id":         123456,
				"contract":   "BTC_USDT",
				"size":       100,
				"status":     "finished",
				"finish_as":  "filled",
				"fill_price": "50000.00",
			}

		// Mock ListFuturesOrders - /api/v4/futures/usdt/orders
		case strings.Contains(path, "/futures/usdt/orders"):
			respBody = []map[string]interface{}{}

		// Mock GetFuturesOrder - /api/v4/futures/usdt/orders/{order_id}
		case strings.Contains(path, "/futures/usdt/orders/"):
			respBody = map[string]interface{}{
				"id":          123456,
				"contract":    "BTC_USDT",
				"size":        100,
				"status":      "finished",
				"finish_as":   "filled",
				"fill_price":  "50000.00",
				"create_time": 1234567890.0,
				"update_time": 1234567890.0,
				"tkfr":        "0.0005",
				"mkfr":        "0.0002",
			}

		// Mock UpdatePositionLeverage
		case strings.Contains(path, "/futures/usdt/positions/") && strings.Contains(path, "/leverage"):
			respBody = map[string]interface{}{
				"leverage": 10,
			}

		// Mock ListPriceTriggeredOrders
		case strings.Contains(path, "/futures/usdt/price_orders"):
			respBody = []map[string]interface{}{}

		// Mock ListPositionClose
		case strings.Contains(path, "/futures/usdt/position_close"):
			respBody = []map[string]interface{}{}

		// Default: empty response
		default:
			respBody = map[string]interface{}{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// Create trader instance (will need to override URL in actual usage)
	traderInstance := NewGateTrader("test_api_key", "test_secret_key")

	// Create base suite
	baseSuite := testutil.NewTraderTestSuite(t, traderInstance)

	return &GateTraderTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
	}
}

// Cleanup cleans up resources
func (s *GateTraderTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// Part 2: Interface compliance tests
// ============================================================

// TestGateTrader_InterfaceCompliance tests interface compliance
func TestGateTrader_InterfaceCompliance(t *testing.T) {
	var _ types.Trader = (*GateTrader)(nil)
}

// ============================================================
// Part 3: Gate-specific feature unit tests
// ============================================================

// TestNewGateTrader tests creating Gate trader
func TestNewGateTrader(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		secretKey string
		wantNil   bool
	}{
		{
			name:      "Successfully create",
			apiKey:    "test_api_key",
			secretKey: "test_secret_key",
			wantNil:   false,
		},
		{
			name:      "Empty API Key can still create",
			apiKey:    "",
			secretKey: "test_secret_key",
			wantNil:   false,
		},
		{
			name:      "Empty Secret Key can still create",
			apiKey:    "test_api_key",
			secretKey: "",
			wantNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := NewGateTrader(tt.apiKey, tt.secretKey)

			if tt.wantNil {
				assert.Nil(t, gt)
			} else {
				assert.NotNil(t, gt)
				assert.NotNil(t, gt.client)
				assert.Equal(t, tt.apiKey, gt.apiKey)
				assert.Equal(t, tt.secretKey, gt.secretKey)
			}
		})
	}
}

// TestGateTrader_SymbolConversion tests symbol format conversion
func TestGateTrader_SymbolConversion(t *testing.T) {
	gt := NewGateTrader("test", "test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "BTCUSDT to BTC_USDT",
			input:    "BTCUSDT",
			expected: "BTC_USDT",
		},
		{
			name:     "ETHUSDT to ETH_USDT",
			input:    "ETHUSDT",
			expected: "ETH_USDT",
		},
		{
			name:     "Already converted format",
			input:    "BTC_USDT",
			expected: "BTC_USDT",
		},
		{
			name:     "SOL symbol",
			input:    "SOLUSDT",
			expected: "SOL_USDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gt.convertSymbol(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGateTrader_RevertSymbol tests symbol reversion
func TestGateTrader_RevertSymbol(t *testing.T) {
	gt := NewGateTrader("test", "test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "BTC_USDT to BTCUSDT",
			input:    "BTC_USDT",
			expected: "BTCUSDT",
		},
		{
			name:     "ETH_USDT to ETHUSDT",
			input:    "ETH_USDT",
			expected: "ETHUSDT",
		},
		{
			name:     "Already standard format",
			input:    "BTCUSDT",
			expected: "BTCUSDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gt.revertSymbol(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGateTrader_CacheDuration tests cache duration
func TestGateTrader_CacheDuration(t *testing.T) {
	gt := NewGateTrader("test", "test")

	// Verify default cache time is 15 seconds
	assert.Equal(t, 15*time.Second, gt.cacheDuration)
}

// TestGateTrader_ClearCache tests cache clearing
func TestGateTrader_ClearCache(t *testing.T) {
	gt := NewGateTrader("test", "test")

	// Set some cached data
	gt.cachedBalance = map[string]interface{}{"test": "data"}
	gt.cachedPositions = []map[string]interface{}{{"test": "data"}}

	// Clear cache
	gt.clearCache()

	// Verify cache is cleared
	assert.Nil(t, gt.cachedBalance)
	assert.Nil(t, gt.cachedPositions)
}

// ============================================================
// Part 4: Mock server integration tests
// ============================================================

// TestGateTrader_MockServerResponseFormat tests mock server response format
func TestGateTrader_MockServerResponseFormat(t *testing.T) {
	suite := NewGateTraderTestSuite(t)
	defer suite.Cleanup()

	// Verify mock server is running
	assert.NotNil(t, suite.mockServer)
	assert.NotEmpty(t, suite.mockServer.URL)
}
