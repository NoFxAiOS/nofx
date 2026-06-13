package nofxos

import (
	"errors"
	"testing"
	"time"
)

func withStubbedAI500Fetch(t *testing.T, fn func(c *Client) ([]CoinData, error)) {
	t.Helper()
	original := fetchAI500ListFn
	fetchAI500ListFn = fn
	ai500Cache.mu.Lock()
	ai500Cache.coins = nil
	ai500Cache.fetchedAt = time.Time{}
	ai500Cache.mu.Unlock()
	t.Cleanup(func() {
		fetchAI500ListFn = original
		ai500Cache.mu.Lock()
		ai500Cache.coins = nil
		ai500Cache.fetchedAt = time.Time{}
		ai500Cache.mu.Unlock()
	})
}

func TestGetAI500ListCachedWithinTTL(t *testing.T) {
	calls := 0
	withStubbedAI500Fetch(t, func(c *Client) ([]CoinData, error) {
		calls++
		return []CoinData{{Pair: "BTCUSDT", Score: 95.5}}, nil
	})

	client := NewClient("", "")
	first, err := GetAI500ListCached(client)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := GetAI500ListCached(client)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1 (second call must hit cache)", calls)
	}
	if len(first) != 1 || len(second) != 1 || second[0].Pair != "BTCUSDT" {
		t.Fatalf("unexpected results: first=%v second=%v", first, second)
	}
}

func TestGetAI500ListCachedServesStaleOnError(t *testing.T) {
	calls := 0
	withStubbedAI500Fetch(t, func(c *Client) ([]CoinData, error) {
		calls++
		if calls == 1 {
			return []CoinData{{Pair: "ETHUSDT", Score: 88}}, nil
		}
		return nil, errors.New("API returned status 429")
	})

	client := NewClient("", "")
	if _, err := GetAI500ListCached(client); err != nil {
		t.Fatalf("first call: %v", err)
	}

	ai500Cache.mu.Lock()
	ai500Cache.fetchedAt = time.Now().Add(-2 * ai500CacheTTL)
	ai500Cache.mu.Unlock()

	coins, err := GetAI500ListCached(client)
	if err != nil {
		t.Fatalf("expected stale data instead of error, got: %v", err)
	}
	if len(coins) != 1 || coins[0].Pair != "ETHUSDT" {
		t.Fatalf("expected stale ETH entry, got %v", coins)
	}
}

func TestGetAI500ListCachedErrorsWithoutCache(t *testing.T) {
	withStubbedAI500Fetch(t, func(c *Client) ([]CoinData, error) {
		return nil, errors.New("upstream down")
	})

	if _, err := GetAI500ListCached(NewClient("", "")); err == nil {
		t.Fatal("expected error when upstream fails with empty cache")
	}
}
