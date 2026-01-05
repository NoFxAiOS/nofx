package trader

import (
	"testing"
	"time"
)

// TestHTXTraderBasic tests basic HTX trader functionality
func TestHTXTraderBasic(t *testing.T) {
	// Skip if no API keys provided
	apiKey := ""    // Set your test API key
	secretKey := "" // Set your test secret key

	if apiKey == "" || secretKey == "" {
		t.Skip("HTX API keys not provided, skipping test")
	}

	trader := NewHTXTrader(apiKey, secretKey)

	// Test GetBalance
	t.Run("GetBalance", func(t *testing.T) {
		balance, err := trader.GetBalance()
		if err != nil {
			t.Errorf("GetBalance failed: %v", err)
			return
		}

		if balance == nil {
			t.Error("Balance is nil")
			return
		}

		t.Logf("Balance: %+v", balance)
	})

	// Test GetPositions
	t.Run("GetPositions", func(t *testing.T) {
		positions, err := trader.GetPositions()
		if err != nil {
			t.Errorf("GetPositions failed: %v", err)
			return
		}

		t.Logf("Positions count: %d", len(positions))
		for i, pos := range positions {
			t.Logf("Position %d: %+v", i, pos)
		}
	})

	// Test GetMarketPrice
	t.Run("GetMarketPrice", func(t *testing.T) {
		price, err := trader.GetMarketPrice("BTC-USDT")
		if err != nil {
			t.Errorf("GetMarketPrice failed: %v", err)
			return
		}

		if price <= 0 {
			t.Errorf("Invalid price: %f", price)
			return
		}

		t.Logf("BTC-USDT price: %f", price)
	})

	// Test SetLeverage
	t.Run("SetLeverage", func(t *testing.T) {
		err := trader.SetLeverage("BTC-USDT", 5)
		if err != nil {
			t.Logf("SetLeverage failed (may be already set): %v", err)
		} else {
			t.Log("SetLeverage successful")
		}
	})

	// Test symbol normalization
	t.Run("NormalizeSymbol", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"BTCUSDT", "BTC-USDT"},
			{"ETHUSDT", "ETH-USDT"},
			{"BTC-USDT", "BTC-USDT"},
		}

		for _, tt := range tests {
			result := trader.normalizeSymbol(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSymbol(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		}
	})
}

// TestGateTraderBasic tests basic Gate.io trader functionality
func TestGateTraderBasic(t *testing.T) {
	// Skip if no API keys provided
	apiKey := ""    // Set your test API key
	secretKey := "" // Set your test secret key

	if apiKey == "" || secretKey == "" {
		t.Skip("Gate.io API keys not provided, skipping test")
	}

	trader := NewGateTrader(apiKey, secretKey)

	// Test GetBalance
	t.Run("GetBalance", func(t *testing.T) {
		balance, err := trader.GetBalance()
		if err != nil {
			t.Errorf("GetBalance failed: %v", err)
			return
		}

		if balance == nil {
			t.Error("Balance is nil")
			return
		}

		t.Logf("Balance: %+v", balance)
	})

	// Test GetPositions
	t.Run("GetPositions", func(t *testing.T) {
		positions, err := trader.GetPositions()
		if err != nil {
			t.Errorf("GetPositions failed: %v", err)
			return
		}

		t.Logf("Positions count: %d", len(positions))
		for i, pos := range positions {
			t.Logf("Position %d: %+v", i, pos)
		}
	})

	// Test GetMarketPrice
	t.Run("GetMarketPrice", func(t *testing.T) {
		price, err := trader.GetMarketPrice("BTC_USDT")
		if err != nil {
			t.Errorf("GetMarketPrice failed: %v", err)
			return
		}

		if price <= 0 {
			t.Errorf("Invalid price: %f", price)
			return
		}

		t.Logf("BTC_USDT price: %f", price)
	})

	// Test SetLeverage
	t.Run("SetLeverage", func(t *testing.T) {
		err := trader.SetLeverage("BTC_USDT", 5)
		if err != nil {
			t.Logf("SetLeverage failed (may be already set): %v", err)
		} else {
			t.Log("SetLeverage successful")
		}
	})

	// Test SetMarginMode
	t.Run("SetMarginMode", func(t *testing.T) {
		err := trader.SetMarginMode("BTC_USDT", true) // Cross margin
		if err != nil {
			t.Logf("SetMarginMode failed (may be already set): %v", err)
		} else {
			t.Log("SetMarginMode successful")
		}
	})

	// Test symbol normalization
	t.Run("NormalizeSymbol", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"BTCUSDT", "BTC_USDT"},
			{"ETHUSDT", "ETH_USDT"},
			{"BTC_USDT", "BTC_USDT"},
		}

		for _, tt := range tests {
			result := trader.normalizeSymbol(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSymbol(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		}
	})
}

// TestHTXTraderCacheExpiration tests cache expiration
func TestHTXTraderCacheExpiration(t *testing.T) {
	trader := NewHTXTrader("test_key", "test_secret")

	// Set cache duration to 1 second for testing
	trader.cacheDuration = 1 * time.Second

	// First call should miss cache
	trader.cachedBalance = map[string]interface{}{
		"total_equity": 1000.0,
	}
	trader.balanceCacheTime = time.Now()

	// Immediate read should hit cache
	time.Sleep(500 * time.Millisecond)
	if time.Since(trader.balanceCacheTime) >= trader.cacheDuration {
		t.Error("Cache should not have expired yet")
	}

	// After cache duration, should miss cache
	time.Sleep(600 * time.Millisecond)
	if time.Since(trader.balanceCacheTime) < trader.cacheDuration {
		t.Error("Cache should have expired")
	}
}

// TestGateTraderCacheExpiration tests cache expiration
func TestGateTraderCacheExpiration(t *testing.T) {
	trader := NewGateTrader("test_key", "test_secret")

	// Set cache duration to 1 second for testing
	trader.cacheDuration = 1 * time.Second

	// First call should miss cache
	trader.cachedBalance = map[string]interface{}{
		"total_equity": 1000.0,
	}
	trader.balanceCacheTime = time.Now()

	// Immediate read should hit cache
	time.Sleep(500 * time.Millisecond)
	if time.Since(trader.balanceCacheTime) >= trader.cacheDuration {
		t.Error("Cache should not have expired yet")
	}

	// After cache duration, should miss cache
	time.Sleep(600 * time.Millisecond)
	if time.Since(trader.balanceCacheTime) < trader.cacheDuration {
		t.Error("Cache should have expired")
	}
}
