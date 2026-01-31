package aster

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"nofx/trader/testutil"
	"nofx/trader/types"
)

// ============================================================
// 1. AsterTraderTestSuite - inherits base test suite
// ============================================================

// AsterTraderTestSuite Aster trader test suite
// Inherits TraderTestSuite and adds Aster specific mock logic
type AsterTraderTestSuite struct {
	*testutil.TraderTestSuite // Embeds base test suite
	mockServer              *httptest.Server
}

// NewAsterTraderTestSuite creates Aster test suite
func NewAsterTraderTestSuite(t *testing.T) *AsterTraderTestSuite {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different mock responses based on URL path
		path := r.URL.Path

		var respBody interface{}

		switch {
		// Mock GetBalance - /fapi/v3/balance (returns array)
		case path == "/fapi/v3/balance":
			respBody = []map[string]interface{}{
				{
					"asset":              "USDT",
					"walletBalance":      "10000.00",
					"unrealizedProfit":   "100.50",
					"marginBalance":      "10100.50",
					"maintMargin":        "200.00",
					"initialMargin":      "2000.00",
					"maxWithdrawAmount":  "8000.00",
					"crossWalletBalance": "10000.00",
					"crossUnPnl":         "100.50",
					"availableBalance":   "8000.00",
				},
			}

		// Mock GetPositions - /fapi/v3/positionRisk
		case path == "/fapi/v3/positionRisk":
			respBody = []map[string]interface{}{
				{
					"symbol":           "BTCUSDT",
					"positionAmt":      "0.5",
					"entryPrice":       "50000.00",
					"markPrice":        "50500.00",
					"unRealizedProfit": "250.00",
					"liquidationPrice": "45000.00",
					"leverage":         "10",
					"positionSide":     "LONG",
				},
			}

		// Mock GetMarketPrice - /fapi/v3/ticker/price (returns single object)
		case path == "/fapi/v3/ticker/price":
			// Get symbol from query parameters
			symbol := r.URL.Query().Get("symbol")
			if symbol == "" {
				symbol = "BTCUSDT"
			}
			// Return different price based on symbol
			price := "50000.00"
			if symbol == "ETHUSDT" {
				price = "3000.00"
			} else if symbol == "INVALIDUSDT" {
				// Return error response
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1121,
					"msg":  "Invalid symbol",
				})
				return
			}
			respBody = map[string]interface{}{
				"symbol": symbol,
				"price":  price,
			}

		// Mock ExchangeInfo - /fapi/v3/exchangeInfo
		case path == "/fapi/v3/exchangeInfo":
			respBody = map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol":             "BTCUSDT",
						"pricePrecision":     1,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.1",
							},
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
						},
					},
					{
						"symbol":             "ETHUSDT",
						"pricePrecision":     2,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
						},
					},
				},
			}

		// Mock CreateOrder - /fapi/v1/order and /fapi/v3/order
		case (path == "/fapi/v1/order" || path == "/fapi/v3/order") && r.Method == "POST":
			// Parse parameters from request to determine symbol
			bodyBytes, _ := io.ReadAll(r.Body)
			var orderParams map[string]interface{}
			json.Unmarshal(bodyBytes, &orderParams)

			symbol := "BTCUSDT"
			if s, ok := orderParams["symbol"].(string); ok {
				symbol = s
			}

			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  symbol,
				"status":  "FILLED",
				"side":    orderParams["side"],
				"type":    orderParams["type"],
			}

		// Mock CancelOrder - /fapi/v1/order (DELETE)
		case path == "/fapi/v1/order" && r.Method == "DELETE":
			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  "BTCUSDT",
				"status":  "CANCELED",
			}

		// Mock ListOpenOrders - /fapi/v1/openOrders and /fapi/v3/openOrders
		case path == "/fapi/v1/openOrders" || path == "/fapi/v3/openOrders":
			respBody = []map[string]interface{}{}

		// Mock SetLeverage - /fapi/v1/leverage
		case path == "/fapi/v1/leverage":
			respBody = map[string]interface{}{
				"leverage": 10,
				"symbol":   "BTCUSDT",
			}

		// Mock SetMarginMode - /fapi/v1/marginType
		case path == "/fapi/v1/marginType":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}

		// Default: empty response
		default:
			respBody = map[string]interface{}{}
		}

		// Serialize response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// Generate a private key for testing
	privateKey, _ := crypto.GenerateKey()

	// Create mock trader using mock server's URL
	traderInstance := &AsterTrader{
		ctx:             context.Background(),
		user:            "0x1234567890123456789012345678901234567890",
		signer:          "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		privateKey:      privateKey,
		client:          mockServer.Client(),
		baseURL:         mockServer.URL, // Use mock server's URL
		symbolPrecision: make(map[string]SymbolPrecision),
	}

	// Create base suite
	baseSuite := testutil.NewTraderTestSuite(t, traderInstance)

	return &AsterTraderTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
	}
}

// Cleanup cleans up resources
func (s *AsterTraderTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// 2. Run common tests using AsterTraderTestSuite
// ============================================================

// TestAsterTrader_InterfaceCompliance tests interface compliance
func TestAsterTrader_InterfaceCompliance(t *testing.T) {
	var _ types.Trader = (*AsterTrader)(nil)
}

// TestAsterTrader_CommonInterface runs all common interface tests using test suite
func TestAsterTrader_CommonInterface(t *testing.T) {
	// Create test suite
	suite := NewAsterTraderTestSuite(t)
	defer suite.Cleanup()

	// Run all common interface tests
	suite.RunAllTests()
}

// ============================================================
// 3. Aster specific unit tests
// ============================================================

// TestNewAsterTrader tests creating Aster trader
func TestNewAsterTrader(t *testing.T) {
	tests := []struct {
		name          string
		user          string
		signer        string
		privateKeyHex string
		wantError     bool
		errorContains string
	}{
		{
			name:          "successful creation",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantError:     false,
		},
		{
			name:          "invalid private key format",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "invalid_key",
			wantError:     true,
			errorContains: "failed to parse private key",
		},
		{
			name:          "private key with 0x prefix",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at, err := NewAsterTrader(tt.user, tt.signer, tt.privateKeyHex)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, at)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, at)
				if at != nil {
					assert.Equal(t, tt.user, at.user)
					assert.Equal(t, tt.signer, at.signer)
					assert.NotNil(t, at.privateKey)
				}
			}
		})
	}
}
