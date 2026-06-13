package hyperliquid

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// withStubbedPerpDexFetch swaps the live fetch function and resets the cache,
// restoring both when the test finishes.
func withStubbedPerpDexFetch(t *testing.T, fn func(ctx context.Context, client *http.Client, dex string) ([]CoinInfo, error)) {
	t.Helper()
	original := fetchPerpDexCoinsFn
	fetchPerpDexCoinsFn = fn
	perpDexCoinCache.mu.Lock()
	perpDexCoinCache.entries = map[string]perpDexCacheEntry{}
	perpDexCoinCache.mu.Unlock()
	t.Cleanup(func() {
		fetchPerpDexCoinsFn = original
		perpDexCoinCache.mu.Lock()
		perpDexCoinCache.entries = map[string]perpDexCacheEntry{}
		perpDexCoinCache.mu.Unlock()
	})
}

func TestGetPerpDexCoinsCachesWithinTTL(t *testing.T) {
	calls := 0
	withStubbedPerpDexFetch(t, func(ctx context.Context, client *http.Client, dex string) ([]CoinInfo, error) {
		calls++
		return []CoinInfo{{Symbol: "xyz:TSLA", MarkPrice: 400}}, nil
	})

	first, err := GetPerpDexCoins(context.Background(), "xyz")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := GetPerpDexCoins(context.Background(), "xyz")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1 (second call must hit cache)", calls)
	}
	if len(first) != 1 || len(second) != 1 || second[0].Symbol != "xyz:TSLA" {
		t.Fatalf("unexpected results: first=%v second=%v", first, second)
	}
}

func TestGetPerpDexCoinsServesStaleOnUpstreamError(t *testing.T) {
	calls := 0
	withStubbedPerpDexFetch(t, func(ctx context.Context, client *http.Client, dex string) ([]CoinInfo, error) {
		calls++
		if calls == 1 {
			return []CoinInfo{{Symbol: "xyz:NVDA", MarkPrice: 1000}}, nil
		}
		return nil, errors.New("API returned status 429")
	})

	if _, err := GetPerpDexCoins(context.Background(), "xyz"); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Expire the cache so the next call must attempt a refresh.
	perpDexCoinCache.mu.Lock()
	entry := perpDexCoinCache.entries["xyz"]
	entry.fetchedAt = time.Now().Add(-2 * perpDexCacheTTL)
	perpDexCoinCache.entries["xyz"] = entry
	perpDexCoinCache.mu.Unlock()

	coins, err := GetPerpDexCoins(context.Background(), "xyz")
	if err != nil {
		t.Fatalf("expected stale data instead of error, got: %v", err)
	}
	if len(coins) != 1 || coins[0].Symbol != "xyz:NVDA" {
		t.Fatalf("expected stale NVDA entry, got %v", coins)
	}
	if calls != 2 {
		t.Fatalf("fetch calls = %d, want 2 (refresh attempted)", calls)
	}
}

func TestGetPerpDexCoinsErrorsWithoutAnyCache(t *testing.T) {
	withStubbedPerpDexFetch(t, func(ctx context.Context, client *http.Client, dex string) ([]CoinInfo, error) {
		return nil, errors.New("API returned status 429")
	})

	if _, err := GetPerpDexCoins(context.Background(), "xyz"); err == nil {
		t.Fatal("expected error when upstream fails and no cache exists")
	}
}

func TestGetPerpDexCoinsCachesPerDex(t *testing.T) {
	withStubbedPerpDexFetch(t, func(ctx context.Context, client *http.Client, dex string) ([]CoinInfo, error) {
		if dex == "xyz" {
			return []CoinInfo{{Symbol: "xyz:AAPL"}}, nil
		}
		return []CoinInfo{{Symbol: "BTC"}}, nil
	})

	xyz, err := GetPerpDexCoins(context.Background(), "xyz")
	if err != nil {
		t.Fatalf("xyz: %v", err)
	}
	def, err := GetPerpDexCoins(context.Background(), "")
	if err != nil {
		t.Fatalf("default dex: %v", err)
	}
	if xyz[0].Symbol != "xyz:AAPL" || def[0].Symbol != "BTC" {
		t.Fatalf("cache keys collided: xyz=%v default=%v", xyz, def)
	}
}
