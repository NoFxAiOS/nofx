package nofxos

import (
	"log"
	"sync"
	"time"
)

// ai500CacheTTL bounds how often the AI500 board is re-fetched. The list is
// refreshed upstream on the order of minutes, every claw402-routed call costs
// money, and the agent UI polls this for display — so short staleness is
// preferable to per-render upstream calls.
const ai500CacheTTL = 5 * time.Minute

type ai500CacheStore struct {
	mu        sync.Mutex
	coins     []CoinData
	fetchedAt time.Time
}

var ai500Cache = &ai500CacheStore{}

// fetchAI500ListFn is swappable in tests.
var fetchAI500ListFn = func(c *Client) ([]CoinData, error) {
	return c.GetAI500List()
}

// GetAI500ListCached returns the AI500 coin list served from a TTL cache.
// When the upstream fetch fails and stale data exists, the stale board is
// served instead of an error so displays keep working through flakiness.
func GetAI500ListCached(c *Client) ([]CoinData, error) {
	ai500Cache.mu.Lock()
	defer ai500Cache.mu.Unlock()

	hasCache := len(ai500Cache.coins) > 0
	if hasCache && time.Since(ai500Cache.fetchedAt) < ai500CacheTTL {
		return copyCoinData(ai500Cache.coins), nil
	}

	coins, err := fetchAI500ListFn(c)
	if err != nil {
		if hasCache {
			log.Printf("⚠️ AI500 fetch failed (%v); serving cached list from %s",
				err, ai500Cache.fetchedAt.Format(time.RFC3339))
			return copyCoinData(ai500Cache.coins), nil
		}
		return nil, err
	}

	ai500Cache.coins = coins
	ai500Cache.fetchedAt = time.Now()
	return copyCoinData(coins), nil
}

// copyCoinData returns a defensive copy so callers cannot mutate the cache.
func copyCoinData(coins []CoinData) []CoinData {
	out := make([]CoinData, len(coins))
	copy(out, coins)
	return out
}
