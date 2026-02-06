package hyperliquid

import (
	"context"
	"sync"
	"testing"

	"github.com/sonirico/go-hyperliquid"
)

// TestMetaConcurrentAccess tests that concurrent access to meta field is safe
func TestMetaConcurrentAccess(t *testing.T) {
	// Create a HyperliquidTrader instance with meta initialized
	ht := &HyperliquidTrader{
		ctx: context.Background(),
		meta: &hyperliquid.Meta{
			Universe: []hyperliquid.AssetInfo{
				{Name: "BTC", SzDecimals: 5},
				{Name: "ETH", SzDecimals: 4},
			},
		},
		metaMutex: sync.RWMutex{},
	}

	// Number of concurrent goroutines
	concurrency := 100
	var wg sync.WaitGroup

	// Test concurrent reads (getSzDecimals)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This should not cause race conditions
			decimals := ht.getSzDecimals("BTC")
			if decimals != 5 {
				t.Errorf("Expected decimals 5, got %d", decimals)
			}
		}()
	}

	wg.Wait()
}

// TestMetaConcurrentReadWrite tests concurrent reads and writes to meta field
func TestMetaConcurrentReadWrite(t *testing.T) {
	ht := &HyperliquidTrader{
		ctx: context.Background(),
		meta: &hyperliquid.Meta{
			Universe: []hyperliquid.AssetInfo{
				{Name: "BTC", SzDecimals: 5},
			},
		},
		metaMutex: sync.RWMutex{},
	}

	var wg sync.WaitGroup
	concurrency := 50

	// Concurrent readers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ht.getSzDecimals("BTC")
		}()
	}

	// Concurrent writers (simulating meta refresh)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			// Simulate meta update
			ht.metaMutex.Lock()
			ht.meta = &hyperliquid.Meta{
				Universe: []hyperliquid.AssetInfo{
					{Name: "BTC", SzDecimals: 5 + iteration%3},
					{Name: "ETH", SzDecimals: 4},
				},
			}
			ht.metaMutex.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify meta is not nil after all operations
	ht.metaMutex.RLock()
	if ht.meta == nil {
		t.Error("Meta should not be nil after concurrent operations")
	}
	ht.metaMutex.RUnlock()
}

// TestGetSzDecimals_NilMeta tests getSzDecimals with nil meta
func TestGetSzDecimals_NilMeta(t *testing.T) {
	ht := &HyperliquidTrader{
		meta:      nil,
		metaMutex: sync.RWMutex{},
	}

	// Should return default value 4 when meta is nil
	decimals := ht.getSzDecimals("BTC")
	expectedDecimals := 4

	if decimals != expectedDecimals {
		t.Errorf("Expected default decimals %d for nil meta, got %d", expectedDecimals, decimals)
	}
}

// TestGetSzDecimals_ValidMeta tests getSzDecimals with valid meta
func TestGetSzDecimals_ValidMeta(t *testing.T) {
	ht := &HyperliquidTrader{
		meta: &hyperliquid.Meta{
			Universe: []hyperliquid.AssetInfo{
				{Name: "BTC", SzDecimals: 5},
				{Name: "ETH", SzDecimals: 4},
				{Name: "SOL", SzDecimals: 3},
			},
		},
		metaMutex: sync.RWMutex{},
	}

	tests := []struct {
		coin             string
		expectedDecimals int
	}{
		{"BTC", 5},
		{"ETH", 4},
		{"SOL", 3},
	}

	for _, tt := range tests {
		t.Run(tt.coin, func(t *testing.T) {
			decimals := ht.getSzDecimals(tt.coin)
			if decimals != tt.expectedDecimals {
				t.Errorf("For coin %s, expected decimals %d, got %d", tt.coin, tt.expectedDecimals, decimals)
			}
		})
	}
}

// TestMetaMutex_NoRaceCondition tests that using -race detector finds no issues
// Run with: go test -race -run TestMetaMutex_NoRaceCondition
func TestMetaMutex_NoRaceCondition(t *testing.T) {
	ht := &HyperliquidTrader{
		ctx: context.Background(),
		meta: &hyperliquid.Meta{
			Universe: []hyperliquid.AssetInfo{
				{Name: "BTC", SzDecimals: 5},
				{Name: "ETH", SzDecimals: 4},
			},
		},
		metaMutex: sync.RWMutex{},
	}

	var wg sync.WaitGroup
	iterations := 1000

	// Massive concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ht.getSzDecimals("BTC")
			ht.getSzDecimals("ETH")
		}()
	}

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ht.metaMutex.Lock()
			ht.meta = &hyperliquid.Meta{
				Universe: []hyperliquid.AssetInfo{
					{Name: "BTC", SzDecimals: 5},
					{Name: "ETH", SzDecimals: 4},
					{Name: "SOL", SzDecimals: 3},
				},
			}
			ht.metaMutex.Unlock()
		}(i)
	}

	wg.Wait()

	// If we reach here without race detector errors, the test passes
	t.Log("No race conditions detected in concurrent meta access")
}
