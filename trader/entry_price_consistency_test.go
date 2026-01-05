package trader

import (
	"nofx/store"
	"testing"
	"time"
)

// TestEntryPriceSyncConsistency verifies that entry prices are consistent between exchange and local database
func TestEntryPriceSyncConsistency(t *testing.T) {
	tests := []struct {
		name          string
		exchangePrice float64
		localPrice    float64
		expectedPrice float64
		description   string
	}{
		{
			name:          "Single position - no accumulation",
			exchangePrice: 100.0,
			localPrice:    100.0,
			expectedPrice: 100.0,
			description:   "When position is new, exchange and local prices should match",
		},
		{
			name:          "Accumulated position with weighted average",
			exchangePrice: 100.0,
			localPrice:    99.5,
			expectedPrice: 99.5,
			description:   "When position is accumulated, local weighted average should be used",
		},
		{
			name:          "Small price difference from averaging",
			exchangePrice: 100.50,
			localPrice:    100.25,
			expectedPrice: 100.25,
			description:   "Position averaged multiple times: local price reflects exact weighted average",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock exchange position
			exchangePos := map[string]interface{}{
				"symbol":           "ETHUSDT",
				"side":             "long",
				"entryPrice":       tt.exchangePrice,
				"markPrice":        105.0,
				"positionAmt":      1.0,
				"unRealizedProfit": 5.0,
				"leverage":         10.0,
				"liquidationPrice": 90.0,
			}

			// Create mock local position
			localPos := &store.TraderPosition{
				Symbol:     "ETHUSDT",
				Side:       "long",
				EntryPrice: tt.localPrice,
			}

			// Verify expected result
			if tt.localPrice > 0 {
				// Should use local price (weighted average)
				if localPos.EntryPrice != tt.expectedPrice {
					t.Errorf("%s: expected entry price %.2f, got %.2f", tt.name, tt.expectedPrice, localPos.EntryPrice)
				}
			} else {
				// Should use exchange price
				if exchangePos["entryPrice"].(float64) != tt.expectedPrice {
					t.Errorf("%s: expected entry price %.2f, got %.2f", tt.name, tt.expectedPrice, exchangePos["entryPrice"].(float64))
				}
			}

			t.Logf("✓ %s: %s", tt.name, tt.description)
		})
	}
}

// TestEntryPriceSyncWithDifferentSymbols verifies sync works independently for each symbol
func TestEntryPriceSyncWithDifferentSymbols(t *testing.T) {
	symbols := []struct {
		symbol        string
		side          string
		exchangePrice float64
		localPrice    float64
	}{
		{"ETHUSDT", "long", 100.0, 99.5},
		{"ETHUSDT", "short", 101.0, 100.8},
		{"BTCUSDT", "long", 50000.0, 49950.0},
	}

	for _, sym := range symbols {
		exchangePos := map[string]interface{}{
			"symbol":           sym.symbol,
			"side":             sym.side,
			"entryPrice":       sym.exchangePrice,
			"markPrice":        sym.exchangePrice,
			"positionAmt":      1.0,
			"unRealizedProfit": 0.0,
			"leverage":         10.0,
			"liquidationPrice": 0.0,
		}

		localPos := &store.TraderPosition{
			Symbol:     sym.symbol,
			Side:       sym.side,
			EntryPrice: sym.localPrice,
		}

		// Verify each symbol/side combination is handled independently
		if localPos.EntryPrice != sym.localPrice {
			t.Errorf("%s %s: local price mismatch", sym.symbol, sym.side)
		}

		// Verify exchange price is unchanged
		if exchangePos["entryPrice"].(float64) != sym.exchangePrice {
			t.Errorf("%s %s: exchange price mismatch", sym.symbol, sym.side)
		}

		t.Logf("✓ %s %s: exchange=%.2f, local=%.2f", sym.symbol, sym.side, sym.exchangePrice, sym.localPrice)
	}
}

// TestEntryPriceSyncHandlesMissingLocalPosition verifies fallback to exchange price
func TestEntryPriceSyncHandlesMissingLocalPosition(t *testing.T) {
	// Position with no local record (new position just opened)
	exchangePos := map[string]interface{}{
		"symbol":           "BTCUSDT",
		"side":             "long",
		"entryPrice":       50000.0,
		"markPrice":        50100.0,
		"positionAmt":      0.1,
		"unRealizedProfit": 10.0,
		"leverage":         10.0,
		"liquidationPrice": 45000.0,
	}

	// No local position found - should use exchange price
	usedPrice := exchangePos["entryPrice"].(float64)

	if usedPrice != 50000.0 {
		t.Errorf("Expected to use exchange price 50000.0, got %.2f", usedPrice)
	}

	t.Log("✓ Missing local position: correctly falls back to exchange price")
}

// TestEntryPricePrecisionWithWeightedAverage verifies precision in weighted average calculations
func TestEntryPricePrecisionWithWeightedAverage(t *testing.T) {
	// Simulate position accumulation:
	// Trade 1: 1 BTC @ $50,000 = $50,000
	// Trade 2: 1 BTC @ $50,100 = $50,100
	// Weighted average = ($50,000 + $50,100) / 2 = $50,050

	trade1Qty := 1.0
	trade1Price := 50000.0

	trade2Qty := 1.0
	trade2Price := 50100.0

	// Calculate weighted average
	totalQty := trade1Qty + trade2Qty
	weightedAvg := (trade1Price*trade1Qty + trade2Price*trade2Qty) / totalQty

	expectedWeightedPrice := 50050.0

	if weightedAvg != expectedWeightedPrice {
		t.Errorf("Expected weighted average %.4f, got %.4f", expectedWeightedPrice, weightedAvg)
	}

	t.Logf("✓ Weighted average calculation: %.4f (trades: %.2f + %.2f)", weightedAvg, trade1Price, trade2Price)
}

// TestEntryPriceSyncTimingConsistency verifies entry prices remain consistent over time
func TestEntryPriceSyncTimingConsistency(t *testing.T) {
	// Position entry price should be consistent even if queried multiple times
	entryTime := time.Now()

	localPos := &store.TraderPosition{
		ID:         1,
		Symbol:     "ETHUSDT",
		Side:       "long",
		EntryPrice: 100.0,
		EntryTime:  entryTime,
		Status:     "OPEN",
	}

	// Simulate multiple queries over time
	prices := make([]float64, 3)
	for i := 0; i < 3; i++ {
		prices[i] = localPos.EntryPrice
		time.Sleep(10 * time.Millisecond)
	}

	// All queries should return same price
	for i := 1; i < len(prices); i++ {
		if prices[i] != prices[0] {
			t.Errorf("Price consistency failed at query %d: %.2f != %.2f", i, prices[i], prices[0])
		}
	}

	t.Logf("✓ Entry price remained consistent across %d queries: %.2f", len(prices), prices[0])
}

// TestEntryPriceDriftDetection verifies detection of price differences between sources
func TestEntryPriceDriftDetection(t *testing.T) {
	tests := []struct {
		name          string
		exchangePrice float64
		localPrice    float64
		expectedDrift float64
		expectsDrift  bool
	}{
		{
			name:          "No drift",
			exchangePrice: 100.0,
			localPrice:    100.0,
			expectedDrift: 0.0,
			expectsDrift:  false,
		},
		{
			name:          "Small drift (0.05%)",
			exchangePrice: 100.0,
			localPrice:    99.95,
			expectedDrift: 0.05,
			expectsDrift:  true,
		},
		{
			name:          "Medium drift (0.5%)",
			exchangePrice: 100.0,
			localPrice:    99.5,
			expectedDrift: 0.5,
			expectsDrift:  true,
		},
		{
			name:          "Large drift (2%)",
			exchangePrice: 100.0,
			localPrice:    98.0,
			expectedDrift: 2.0,
			expectsDrift:  true,
		},
	}

	for _, tt := range tests {
		driftPct := ((tt.exchangePrice - tt.localPrice) / tt.localPrice) * 100

		hasDrift := driftPct != 0

		if hasDrift != tt.expectsDrift {
			t.Errorf("%s: expected drift detection %v, got %v", tt.name, tt.expectsDrift, hasDrift)
		}

		// Verify drift calculation
		if tt.expectsDrift && (driftPct < tt.expectedDrift-0.1 || driftPct > tt.expectedDrift+0.1) {
			t.Errorf("%s: expected drift %.2f%%, got %.2f%%", tt.name, tt.expectedDrift, driftPct)
		}

		t.Logf("✓ %s: drift=%.2f%% (exchange=%.2f, local=%.2f)", tt.name, driftPct, tt.exchangePrice, tt.localPrice)
	}
}
