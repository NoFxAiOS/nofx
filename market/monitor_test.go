package market

import (
	"sync"
	"testing"
	"time"
)

// TestCalculateOIChange4h_SufficientData tests 4-hour OI change calculation with sufficient data
func TestCalculateOIChange4h_SufficientData(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	// Create 4-hour history with precise 4-hour-old data point
	now := time.Now()
	history := []OISnapshot{
		{Value: 10000.0, Timestamp: now.Add(-4 * time.Hour)}, // Exactly 4h ago
		{Value: 10200.0, Timestamp: now.Add(-3 * time.Hour)},
		{Value: 10500.0, Timestamp: now.Add(-2 * time.Hour)},
		{Value: 10800.0, Timestamp: now.Add(-1 * time.Hour)},
		{Value: 11000.0, Timestamp: now.Add(-30 * time.Minute)},
	}

	monitor.oiHistoryMap.Store("BTCUSDT", history)

	// Current OI: 12000 (20% increase from 10000)
	change, period := monitor.CalculateOIChange4h("BTCUSDT", 12000.0)

	// Should calculate 4-hour change
	if period != "4h" {
		t.Errorf("Expected period '4h', got '%s'", period)
	}

	// The function finds closest to 4h ago, which is the first data point (10000)
	expectedChange := 20.0 // (12000 - 10000) / 10000 * 100
	if change < expectedChange-0.5 || change > expectedChange+0.5 {
		t.Errorf("Expected change around %.2f%%, got %.2f%%", expectedChange, change)
	}
}

// TestCalculateOIChange4h_InsufficientData tests degraded calculation with <4h data
func TestCalculateOIChange4h_InsufficientData(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	// Create only 2-hour history (8 data points)
	now := time.Now()
	history := []OISnapshot{
		{Value: 10000.0, Timestamp: now.Add(-2 * time.Hour)},
		{Value: 10200.0, Timestamp: now.Add(-105 * time.Minute)},
		{Value: 10400.0, Timestamp: now.Add(-90 * time.Minute)},
		{Value: 10600.0, Timestamp: now.Add(-75 * time.Minute)},
		{Value: 10800.0, Timestamp: now.Add(-60 * time.Minute)},
		{Value: 11000.0, Timestamp: now.Add(-45 * time.Minute)},
		{Value: 11200.0, Timestamp: now.Add(-30 * time.Minute)},
		{Value: 11400.0, Timestamp: now.Add(-15 * time.Minute)},
	}

	monitor.oiHistoryMap.Store("ETHUSDT", history)

	change, period := monitor.CalculateOIChange4h("ETHUSDT", 11500.0)

	// Should use degraded calculation (2.0h)
	if period == "4h" {
		t.Error("Expected degraded period (not 4h), but got '4h'")
	}

	// Should calculate from oldest available data (10000)
	expectedChange := 15.0 // (11500 - 10000) / 10000 * 100
	if change < expectedChange-0.1 || change > expectedChange+0.1 {
		t.Errorf("Expected change around %.2f%%, got %.2f%%", expectedChange, change)
	}
}

// TestCalculateOIChange4h_SingleDataPoint tests cold start scenario with only 1 data point
func TestCalculateOIChange4h_SingleDataPoint(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	// Only 1 data point (system just started)
	history := []OISnapshot{
		{Value: 10000.0, Timestamp: time.Now()},
	}

	monitor.oiHistoryMap.Store("SOLUSDT", history)

	change, period := monitor.CalculateOIChange4h("SOLUSDT", 10500.0)

	// Should return 0% change with "0m" period
	if period != "0m" {
		t.Errorf("Expected period '0m' for single data point, got '%s'", period)
	}

	if change != 0.0 {
		t.Errorf("Expected 0%% change for single data point, got %.2f%%", change)
	}
}

// TestCalculateOIChange4h_EmptyHistory tests scenario with no historical data
func TestCalculateOIChange4h_EmptyHistory(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	// No history for this symbol (will attempt API fallback, which will fail in test)
	change, period := monitor.CalculateOIChange4h("NEWCOIN", 10000.0)

	// Should return "N/A" when no data available
	if period != "N/A" {
		t.Errorf("Expected period 'N/A' for empty history, got '%s'", period)
	}

	if change != 0.0 {
		t.Errorf("Expected 0%% change for empty history, got %.2f%%", change)
	}
}

// TestCalculateOIChange4h_ZeroOldValue tests edge case where historical OI is 0
func TestCalculateOIChange4h_ZeroOldValue(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	now := time.Now()
	history := []OISnapshot{
		{Value: 0.0, Timestamp: now.Add(-4 * time.Hour)}, // Old value is 0
		{Value: 5000.0, Timestamp: now.Add(-2 * time.Hour)},
	}

	monitor.oiHistoryMap.Store("TESTUSDT", history)

	change, period := monitor.CalculateOIChange4h("TESTUSDT", 10000.0)

	// Should return "N/A" when old value is 0 (division by zero prevention)
	if period != "N/A" {
		t.Errorf("Expected period 'N/A' for zero old value, got '%s'", period)
	}

	if change != 0.0 {
		t.Errorf("Expected 0%% change for zero old value, got %.2f%%", change)
	}
}

// TestCalculateOIChange4h_NegativeChange tests OI decrease scenario
func TestCalculateOIChange4h_NegativeChange(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	now := time.Now()
	history := []OISnapshot{
		{Value: 15000.0, Timestamp: now.Add(-4 * time.Hour)}, // Exactly 4h ago
		{Value: 14500.0, Timestamp: now.Add(-3 * time.Hour)},
		{Value: 14000.0, Timestamp: now.Add(-2 * time.Hour)},
		{Value: 13500.0, Timestamp: now.Add(-1 * time.Hour)},
		{Value: 13000.0, Timestamp: now.Add(-30 * time.Minute)},
	}

	monitor.oiHistoryMap.Store("AVAXUSDT", history)

	// Current OI: 12000 (20% decrease from 15000)
	change, period := monitor.CalculateOIChange4h("AVAXUSDT", 12000.0)

	if period != "4h" {
		t.Errorf("Expected period '4h', got '%s'", period)
	}

	// Should be negative change
	expectedChange := -20.0 // (12000 - 15000) / 15000 * 100
	if change > expectedChange+0.5 || change < expectedChange-0.5 {
		t.Errorf("Expected change around %.2f%%, got %.2f%%", expectedChange, change)
	}

	if change >= 0 {
		t.Errorf("Expected negative change, got %.2f%%", change)
	}
}

// TestCalculateOIChange4h_CaseInsensitive tests that symbol lookup is case-insensitive
func TestCalculateOIChange4h_CaseInsensitive(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	now := time.Now()
	history := []OISnapshot{
		{Value: 10000.0, Timestamp: now.Add(-4 * time.Hour)},
		{Value: 11000.0, Timestamp: now.Add(-2 * time.Hour)},
	}

	// Store with uppercase
	monitor.oiHistoryMap.Store("BTCUSDT", history)

	// Query with lowercase (should work due to strings.ToUpper in function)
	change, _ := monitor.CalculateOIChange4h("btcusdt", 12000.0)

	expectedChange := 20.0
	if change < expectedChange-0.1 || change > expectedChange+0.1 {
		t.Errorf("Case-insensitive lookup failed. Expected %.2f%%, got %.2f%%", expectedChange, change)
	}
}

// TestGetOIHistory tests the GetOIHistory helper function
func TestGetOIHistory(t *testing.T) {
	monitor := &WSMonitor{
		oiHistoryMap: sync.Map{},
	}

	// Test with existing data
	expectedHistory := []OISnapshot{
		{Value: 10000.0, Timestamp: time.Now()},
		{Value: 11000.0, Timestamp: time.Now().Add(15 * time.Minute)},
	}
	monitor.oiHistoryMap.Store("BTCUSDT", expectedHistory)

	result := monitor.GetOIHistory("BTCUSDT")
	if len(result) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(result))
	}

	// Test with non-existing symbol
	result = monitor.GetOIHistory("NONEXISTENT")
	if result != nil {
		t.Errorf("Expected nil for non-existent symbol, got %v", result)
	}

	// Test case insensitivity
	result = monitor.GetOIHistory("btcusdt")
	if len(result) != 2 {
		t.Errorf("Case-insensitive lookup failed. Expected 2 snapshots, got %d", len(result))
	}
}
