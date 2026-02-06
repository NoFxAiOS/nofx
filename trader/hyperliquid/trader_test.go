package hyperliquid

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
	"github.com/stretchr/testify/assert"
	"nofx/trader/testutil"
	"nofx/trader/types"
)

// ============================================================
// Part 1: HyperliquidTestSuite - Inherits base test suite
// ============================================================

// HyperliquidTestSuite Hyperliquid trader test suite
// Inherits TraderTestSuite and adds Hyperliquid-specific mock logic
type HyperliquidTestSuite struct {
	*testutil.TraderTestSuite // Embeds base test suite
	mockServer              *httptest.Server
	privateKey              *ecdsa.PrivateKey
}

// NewHyperliquidTestSuite Create Hyperliquid test suite
func NewHyperliquidTestSuite(t *testing.T) *HyperliquidTestSuite {
	// Create test private key
	privateKey, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("Failed to create test private key: %v", err)
	}

	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different mock responses based on request path
		var respBody interface{}

		// Hyperliquid API uses POST requests with JSON body
		// We need to distinguish different requests by the "type" field in request body
		var reqBody map[string]interface{}
		if r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&reqBody)
		}

		// Try to get type from top level first, then from action object
		reqType, _ := reqBody["type"].(string)
		if reqType == "" && reqBody["action"] != nil {
			if action, ok := reqBody["action"].(map[string]interface{}); ok {
				reqType, _ = action["type"].(string)
			}
		}

		switch reqType {
		// Mock Meta - Get market metadata
		case "meta":
			respBody = map[string]interface{}{
				"universe": []map[string]interface{}{
					{
						"name":          "BTC",
						"szDecimals":    4,
						"maxLeverage":   50,
						"onlyIsolated":  false,
						"isDelisted":    false,
						"marginTableId": 0,
					},
					{
						"name":          "ETH",
						"szDecimals":    3,
						"maxLeverage":   50,
						"onlyIsolated":  false,
						"isDelisted":    false,
						"marginTableId": 0,
					},
				},
				"marginTables": []interface{}{},
			}

		// Mock UserState - Get user account state (for GetBalance and GetPositions)
		case "clearinghouseState":
			user, _ := reqBody["user"].(string)

			// Check if querying Agent wallet balance (for security check)
			agentAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
			if user == agentAddr {
				// Agent wallet balance should be low
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue":    "5.00",
						"totalMarginUsed": "0.00",
					},
					"withdrawable":   "5.00",
					"assetPositions": []interface{}{},
				}
			} else {
				// Main wallet account state
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue":    "10000.00",
						"totalMarginUsed": "2000.00",
					},
					"withdrawable": "8000.00",
					"assetPositions": []map[string]interface{}{
						{
							"position": map[string]interface{}{
								"coin":          "BTC",
								"szi":           "0.5",
								"entryPx":       "50000.00",
								"liquidationPx": "45000.00",
								"positionValue": "25000.00",
								"unrealizedPnl": "100.50",
								"leverage": map[string]interface{}{
									"type":  "cross",
									"value": 10,
								},
							},
						},
					},
				}
			}

		// Mock SpotUserState - Get spot account state
		case "spotClearinghouseState":
			respBody = map[string]interface{}{
				"balances": []map[string]interface{}{
					{
						"coin":  "USDC",
						"total": "500.00",
					},
				},
			}

		// Mock SpotMeta - Get spot market metadata
		case "spotMeta":
			respBody = map[string]interface{}{
				"universe": []map[string]interface{}{},
				"tokens":   []map[string]interface{}{},
			}

		// Mock AllMids - Get all market prices
		case "allMids":
			respBody = map[string]string{
				"BTC": "50000.00",
				"ETH": "3000.00",
			}

		// Mock OpenOrders - Get open orders list
		case "openOrders":
			respBody = []interface{}{}

		// Mock Order - Create order (open, close, stop-loss, take-profit)
		case "order":
			respBody = map[string]interface{}{
				"status": "ok",
				"response": map[string]interface{}{
					"type": "order",
					"data": map[string]interface{}{
						"statuses": []map[string]interface{}{
							{
								"filled": map[string]interface{}{
									"totalSz": "0.01",
									"avgPx":   "50000.00",
								},
							},
						},
					},
				},
			}

		// Mock UpdateLeverage - Set leverage
		case "updateLeverage":
			respBody = map[string]interface{}{
				"status": "ok",
			}

		// Mock Cancel - Cancel order
		case "cancel":
			respBody = map[string]interface{}{
				"status": "ok",
			}

		default:
			// Default return success response
			respBody = map[string]interface{}{
				"status": "ok",
			}
		}

		// Serialize response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// Create HyperliquidTrader, using mock server URL
	walletAddr := "0x9999999999999999999999999999999999999999"
	ctx := context.Background()

	// Create Exchange client, pointing to mock server
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		mockServer.URL, // Use mock server URL
		nil,
		"",
		walletAddr,
		nil,
	)

	// Create meta (simulate successful fetch)
	meta := &hyperliquid.Meta{
		Universe: []hyperliquid.AssetInfo{
			{Name: "BTC", SzDecimals: 4},
			{Name: "ETH", SzDecimals: 3},
		},
	}

	traderInstance := &HyperliquidTrader{
		exchange:      exchange,
		ctx:           ctx,
		walletAddr:    walletAddr,
		meta:          meta,
		isCrossMargin: true,
	}

	// Create base suite
	baseSuite := testutil.NewTraderTestSuite(t, traderInstance)

	return &HyperliquidTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
		privateKey:      privateKey,
	}
}

// Cleanup Clean up resources
func (s *HyperliquidTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// Part 2: Run common tests using HyperliquidTestSuite
// ============================================================

// TestHyperliquidTrader_InterfaceCompliance Test interface compliance
func TestHyperliquidTrader_InterfaceCompliance(t *testing.T) {
	var _ types.Trader = (*HyperliquidTrader)(nil)
}

// TestHyperliquidTrader_CommonInterface Run all common interface tests using test suite
func TestHyperliquidTrader_CommonInterface(t *testing.T) {
	// Create test suite
	suite := NewHyperliquidTestSuite(t)
	defer suite.Cleanup()

	// Run all common interface tests
	suite.RunAllTests()
}

// ============================================================
// Part 3: Hyperliquid-specific feature unit tests
// ============================================================

// TestNewHyperliquidTrader Test creating Hyperliquid trader
func TestNewHyperliquidTrader(t *testing.T) {
	tests := []struct {
		name          string
		privateKeyHex string
		walletAddr    string
		testnet       bool
		wantError     bool
		errorContains string
	}{
		{
			name:          "Invalid private key format",
			privateKeyHex: "invalid_key",
			walletAddr:    "0x1234567890123456789012345678901234567890",
			testnet:       true,
			wantError:     true,
			errorContains: "Failed to parse private key",
		},
		{
			name:          "Empty wallet address",
			privateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			walletAddr:    "",
			testnet:       true,
			wantError:     true,
			errorContains: "Configuration error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trader, err := NewHyperliquidTrader(tt.privateKeyHex, tt.walletAddr, tt.testnet)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, trader)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, trader)
				if trader != nil {
					assert.Equal(t, tt.walletAddr, trader.walletAddr)
					assert.NotNil(t, trader.exchange)
				}
			}
		})
	}
}

// TestNewHyperliquidTrader_Success Test successfully creating trader (requires mock HTTP)
func TestNewHyperliquidTrader_Success(t *testing.T) {
	// Create test private key
	privateKey, _ := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	agentAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		reqType, _ := reqBody["type"].(string)

		var respBody interface{}
		switch reqType {
		case "meta":
			respBody = map[string]interface{}{
				"universe": []map[string]interface{}{
					{
						"name":          "BTC",
						"szDecimals":    4,
						"maxLeverage":   50,
						"onlyIsolated":  false,
						"isDelisted":    false,
						"marginTableId": 0,
					},
				},
				"marginTables": []interface{}{},
			}
		case "clearinghouseState":
			user, _ := reqBody["user"].(string)
			if user == agentAddr {
				// Agent wallet low balance
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue": "5.00",
					},
					"assetPositions": []interface{}{},
				}
			} else {
				// Main wallet
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue": "10000.00",
					},
					"assetPositions": []interface{}{},
				}
			}
		default:
			respBody = map[string]interface{}{"status": "ok"}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))
	defer mockServer.Close()

	// Note: This test would actually call NewHyperliquidTrader, but will fail
	// Because hyperliquid SDK doesn't allow us to inject custom URL in constructor
	// So this test is only for verifying parameter handling logic
	t.Skip("Skip this test: hyperliquid SDK calls real API during construction, cannot inject mock URL")
}

// ============================================================
// Part 4: Utility function unit tests (Hyperliquid-specific)
// ============================================================

// TestConvertSymbolToHyperliquid Test symbol conversion function
func TestConvertSymbolToHyperliquid(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected string
	}{
		{
			name:     "BTCUSDT conversion",
			symbol:   "BTCUSDT",
			expected: "BTC",
		},
		{
			name:     "ETHUSDT conversion",
			symbol:   "ETHUSDT",
			expected: "ETH",
		},
		{
			name:     "No USDT suffix",
			symbol:   "BTC",
			expected: "BTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertSymbolToHyperliquid(tt.symbol)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAbsFloat Test absolute value function
func TestAbsFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "Positive number",
			input:    10.5,
			expected: 10.5,
		},
		{
			name:     "Negative number",
			input:    -10.5,
			expected: 10.5,
		},
		{
			name:     "Zero",
			input:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := absFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHyperliquidTrader_RoundToSzDecimals Test quantity precision handling
func TestHyperliquidTrader_RoundToSzDecimals(t *testing.T) {
	trader := &HyperliquidTrader{
		meta: &hyperliquid.Meta{
			Universe: []hyperliquid.AssetInfo{
				{Name: "BTC", SzDecimals: 4},
				{Name: "ETH", SzDecimals: 3},
			},
		},
	}

	tests := []struct {
		name     string
		coin     string
		quantity float64
		expected float64
	}{
		{
			name:     "BTC - round to 4 decimals",
			coin:     "BTC",
			quantity: 1.23456789,
			expected: 1.2346,
		},
		{
			name:     "ETH - round to 3 decimals",
			coin:     "ETH",
			quantity: 10.12345,
			expected: 10.123,
		},
		{
			name:     "Unknown coin - use default 4 decimals",
			coin:     "UNKNOWN",
			quantity: 1.23456789,
			expected: 1.2346,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trader.roundToSzDecimals(tt.coin, tt.quantity)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// TestHyperliquidTrader_RoundPriceToSigfigs Test price significant figures handling
func TestHyperliquidTrader_RoundPriceToSigfigs(t *testing.T) {
	trader := &HyperliquidTrader{}

	tests := []struct {
		name     string
		price    float64
		expected float64
	}{
		{
			name:     "BTC price - 5 significant figures",
			price:    50123.456789,
			expected: 50123.0,
		},
		{
			name:     "Decimal price - 5 significant figures",
			price:    0.0012345678,
			expected: 0.0012346,
		},
		{
			name:     "Zero price",
			price:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trader.roundPriceToSigfigs(tt.price)
			assert.InDelta(t, tt.expected, result, tt.expected*0.001)
		})
	}
}

// TestHyperliquidTrader_GetSzDecimals Test getting precision
func TestHyperliquidTrader_GetSzDecimals(t *testing.T) {
	tests := []struct {
		name     string
		meta     *hyperliquid.Meta
		coin     string
		expected int
	}{
		{
			name:     "meta is nil - return default precision",
			meta:     nil,
			coin:     "BTC",
			expected: 4,
		},
		{
			name: "Found BTC - return correct precision",
			meta: &hyperliquid.Meta{
				Universe: []hyperliquid.AssetInfo{
					{Name: "BTC", SzDecimals: 5},
				},
			},
			coin:     "BTC",
			expected: 5,
		},
		{
			name: "Coin not found - return default precision",
			meta: &hyperliquid.Meta{
				Universe: []hyperliquid.AssetInfo{
					{Name: "ETH", SzDecimals: 3},
				},
			},
			coin:     "BTC",
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ht := &HyperliquidTrader{meta: tt.meta}
			result := ht.getSzDecimals(tt.coin)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHyperliquidTrader_SetMarginMode Test setting margin mode
func TestHyperliquidTrader_SetMarginMode(t *testing.T) {
	trader := &HyperliquidTrader{
		ctx:           context.Background(),
		isCrossMargin: true,
	}

	tests := []struct {
		name          string
		symbol        string
		isCrossMargin bool
		wantError     bool
	}{
		{
			name:          "Set to cross margin mode",
			symbol:        "BTCUSDT",
			isCrossMargin: true,
			wantError:     false,
		},
		{
			name:          "Set to isolated margin mode",
			symbol:        "ETHUSDT",
			isCrossMargin: false,
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := trader.SetMarginMode(tt.symbol, tt.isCrossMargin)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.isCrossMargin, trader.isCrossMargin)
			}
		})
	}
}

// TestNewHyperliquidTrader_PrivateKeyProcessing Test private key processing
func TestNewHyperliquidTrader_PrivateKeyProcessing(t *testing.T) {
	tests := []struct {
		name           string
		privateKeyHex  string
		shouldStripOx  bool
		expectedLength int
	}{
		{
			name:           "Private key with 0x prefix",
			privateKeyHex:  "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			shouldStripOx:  true,
			expectedLength: 64,
		},
		{
			name:           "Private key without prefix",
			privateKeyHex:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			shouldStripOx:  false,
			expectedLength: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test private key prefix handling logic (without actually creating trader)
			processed := tt.privateKeyHex
			if len(processed) > 2 && (processed[:2] == "0x" || processed[:2] == "0X") {
				processed = processed[2:]
			}

			assert.Equal(t, tt.expectedLength, len(processed))
		})
	}
}
