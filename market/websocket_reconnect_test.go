package market

import (
	"sync"
	"testing"
	"time"
)

// TestWSMonitor_GetCurrentKlines_StaleDataDetection tests that stale data is detected
// TDD Red: This test should FAIL initially, demonstrating the bug
func TestWSMonitor_GetCurrentKlines_StaleDataDetection(t *testing.T) {
	monitor := &WSMonitor{
		klineDataMap3m: sync.Map{},
		klineDataMap4h: sync.Map{},
	}

	symbol := "BTCUSDT"

	// Create klines with old ReceivedAt (6 hours ago)
	sixHoursAgo := time.Now().Add(-6 * time.Hour)
	staleEntry := &KlineCacheEntry{
		Klines: []Kline{
			{
				OpenTime:  sixHoursAgo.Add(-3 * time.Minute).UnixMilli(),
				CloseTime: sixHoursAgo.UnixMilli(),
				Close:     43500.0,
				High:      43600.0,
				Low:       43400.0,
				Open:      43450.0,
				Volume:    1000.0,
			},
			{
				OpenTime:  sixHoursAgo.UnixMilli(),
				CloseTime: sixHoursAgo.Add(3 * time.Minute).UnixMilli(),
				Close:     43505.5,
				High:      43605.5,
				Low:       43405.5,
				Open:      43455.5,
				Volume:    1100.0,
			},
		},
		ReceivedAt: sixHoursAgo,
	}

	// Store stale data in cache (simulating old WebSocket data that hasn't been updated)
	monitor.klineDataMap3m.Store(symbol, staleEntry)

	// Try to get current klines
	klines, err := monitor.GetCurrentKlines(symbol, "3m")

	// TDD EXPECTATION: Should detect that ReceivedAt is 6 hours ago and return an error
	if err == nil {
		t.Logf("❌ BUG REPRODUCED: GetCurrentKlines returned stale data (6 hours old) without error")
		t.Logf("   Current implementation does not check data age")
		t.Logf("   Returned %d klines", len(klines))
		t.Logf("   Expected: Error indicating data is stale")
		t.Fatal("TDD RED: This test should pass after fixing the stale data detection")
	}

	// After fix, error message should mention the issue
	if !contains(err.Error(), "过期") && !contains(err.Error(), "WebSocket") {
		t.Errorf("Error should mention data staleness or WebSocket issue, got: %v", err)
	}

	t.Logf("✅ Test PASSED: Stale data correctly rejected with error: %v", err)
}

// TestWSMonitor_GetCurrentKlines_FreshDataPasses tests that fresh data is accepted
// This test should PASS even before the fix (verifies we don't break existing behavior)
func TestWSMonitor_GetCurrentKlines_FreshDataPasses(t *testing.T) {
	monitor := &WSMonitor{
		klineDataMap3m: sync.Map{},
		klineDataMap4h: sync.Map{},
	}

	symbol := "ETHUSDT"

	// Create fresh klines (1 minute ago)
	oneMinuteAgo := time.Now().Add(-1 * time.Minute)
	freshEntry := &KlineCacheEntry{
		Klines: []Kline{
			{
				OpenTime:  oneMinuteAgo.Add(-3 * time.Minute).UnixMilli(),
				CloseTime: oneMinuteAgo.UnixMilli(),
				Close:     2500.0,
				High:      2510.0,
				Low:       2490.0,
				Open:      2495.0,
				Volume:    5000.0,
			},
			{
				OpenTime:  oneMinuteAgo.UnixMilli(),
				CloseTime: oneMinuteAgo.Add(3 * time.Minute).UnixMilli(),
				Close:     2505.5,
				High:      2515.5,
				Low:       2495.5,
				Open:      2500.5,
				Volume:    5100.0,
			},
		},
		ReceivedAt: oneMinuteAgo,
	}

	// Store fresh data in cache
	monitor.klineDataMap3m.Store(symbol, freshEntry)

	// Try to get current klines
	klines, err := monitor.GetCurrentKlines(symbol, "3m")

	// Fresh data should be returned without error
	if err != nil {
		t.Fatalf("Fresh data (1 minute old) should not return error, got: %v", err)
	}

	if klines == nil || len(klines) != 2 {
		t.Errorf("Expected 2 fresh klines, got %d", len(klines))
	}

	// Verify the data is correct
	if klines[0].Close != 2500.0 {
		t.Errorf("Expected close price 2500.0, got %.2f", klines[0].Close)
	}

	t.Logf("✅ Test PASSED: Fresh data correctly accepted")
}

// TestWSMonitor_GetCurrentKlines_BoundaryCase tests the 15-minute boundary
func TestWSMonitor_GetCurrentKlines_BoundaryCase(t *testing.T) {
	monitor := &WSMonitor{
		klineDataMap3m: sync.Map{},
		klineDataMap4h: sync.Map{},
	}

	symbol := "SOLUSDT"

	// Create klines exactly 15 minutes and 1 second old (should be rejected)
	fifteenMinOneSecAgo := time.Now().Add(-15*time.Minute - 1*time.Second)
	boundaryKlines := &KlineCacheEntry{
		Klines: []Kline{
			{
				OpenTime:  fifteenMinOneSecAgo.Add(-3 * time.Minute).UnixMilli(),
				CloseTime: fifteenMinOneSecAgo.UnixMilli(),
				Close:     100.0,
				High:      101.0,
				Low:       99.0,
				Open:      100.5,
				Volume:    3000.0,
			},
		},
		ReceivedAt: fifteenMinOneSecAgo,
	}

	monitor.klineDataMap3m.Store(symbol, boundaryKlines)

	klines, err := monitor.GetCurrentKlines(symbol, "3m")

	// Should reject data older than 15 minutes
	if err == nil {
		t.Logf("❌ BUG: Data exactly 15min1sec old should be rejected, but was accepted")
		t.Logf("   Returned %d klines", len(klines))
		t.Fatal("TDD RED: Should reject data older than 15 minutes")
	}
}

// TestWSMonitor_GetCurrentKlines_NoDataFallsBackToAPI tests API fallback
func TestWSMonitor_GetCurrentKlines_NoDataFallsBackToAPI(t *testing.T) {
	t.Skip("Skipping API test - requires network connection")

	monitor := &WSMonitor{
		klineDataMap3m: sync.Map{},
		klineDataMap4h: sync.Map{},
	}

	symbol := "BTCUSDT"

	// Don't store any data in cache

	// Should fall back to API
	klines, err := monitor.GetCurrentKlines(symbol, "3m")

	if err != nil {
		t.Fatalf("API fallback should work, got error: %v", err)
	}

	if klines == nil || len(klines) == 0 {
		t.Error("Should return klines from API")
	}

	t.Logf("✓ API fallback worked, returned %d klines", len(klines))
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestDataAgeCalculation tests time calculation logic
func TestDataAgeCalculation(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name           string
		dataTime       time.Time
		maxAge         time.Duration
		shouldBeStale  bool
	}{
		{
			name:           "Fresh data - 1 minute old",
			dataTime:       now.Add(-1 * time.Minute),
			maxAge:         5 * time.Minute,
			shouldBeStale:  false,
		},
		{
			name:           "Boundary - exactly 5 minutes old",
			dataTime:       now.Add(-5 * time.Minute),
			maxAge:         5 * time.Minute,
			shouldBeStale:  false,
		},
		{
			name:           "Stale - 5 minutes 1 second old",
			dataTime:       now.Add(-5*time.Minute - 1*time.Second),
			maxAge:         5 * time.Minute,
			shouldBeStale:  true,
		},
		{
			name:           "Very stale - 6 hours old",
			dataTime:       now.Add(-6 * time.Hour),
			maxAge:         5 * time.Minute,
			shouldBeStale:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			age := now.Sub(tc.dataTime)
			isStale := age > tc.maxAge

			if isStale != tc.shouldBeStale {
				t.Errorf("Expected stale=%v, got stale=%v (age: %v, maxAge: %v)",
					tc.shouldBeStale, isStale, age, tc.maxAge)
			}
		})
	}
}
