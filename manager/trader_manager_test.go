package manager

import (
	"testing"
)

// TestRemoveTrader tests removing trader from memory
func TestRemoveTrader(t *testing.T) {
	tm := NewTraderManager()

	// Create a mock trader and add it to map
	traderID := "test-trader-123"
	tm.traders[traderID] = nil // Use nil as placeholder, only need to verify deletion logic in test

	// Verify trader exists
	if _, exists := tm.traders[traderID]; !exists {
		t.Fatal("trader should exist in map")
	}

	// Call RemoveTrader
	tm.RemoveTrader(traderID)

	// Verify trader has been removed
	if _, exists := tm.traders[traderID]; exists {
		t.Error("trader should be removed from map")
	}
}

// TestRemoveTrader_NonExistent tests that removing non-existent trader doesn't error
func TestRemoveTrader_NonExistent(t *testing.T) {
	tm := NewTraderManager()

	// Trying to remove non-existent trader should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("removing non-existent trader should not panic: %v", r)
		}
	}()

	tm.RemoveTrader("non-existent-trader")
}

// TestRemoveTrader_Concurrent tests concurrent removal of trader safety
func TestRemoveTrader_Concurrent(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-concurrent"

	// Add trader
	tm.traders[traderID] = nil

	// Concurrently call RemoveTrader
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			tm.RemoveTrader(traderID)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify trader has been removed
	if _, exists := tm.traders[traderID]; exists {
		t.Error("trader should be removed from map")
	}
}

// TestGetTrader_AfterRemove tests that getting trader after removal returns error
func TestGetTrader_AfterRemove(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-get"

	// Add trader
	tm.traders[traderID] = nil

	// Remove trader
	tm.RemoveTrader(traderID)

	// Try to get removed trader
	_, err := tm.GetTrader(traderID)
	if err == nil {
		t.Error("getting removed trader should return error")
	}
}
