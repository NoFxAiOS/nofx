package wallet

import (
	"strings"
	"sync"
	"time"
)

// balanceCacheTTL is how long a balance reading is trusted before re-querying.
const balanceCacheTTL = 30 * time.Second

type balanceEntry struct {
	value     float64
	fetchedAt time.Time
}

var (
	balanceCache   sync.Map
	balanceFetchMu sync.Map
)

// QueryUSDCBalanceCached returns the USDC balance for an address, using a
// short-lived cache to avoid hammering the Base RPC. Addresses are
// case-insensitive.
func QueryUSDCBalanceCached(address string) (float64, error) {
	key := strings.ToLower(strings.TrimSpace(address))
	if key == "" {
		return 0, nil
	}

	if v, ok := balanceCache.Load(key); ok {
		e := v.(balanceEntry)
		if time.Since(e.fetchedAt) < balanceCacheTTL {
			return e.value, nil
		}
	}

	muAny, _ := balanceFetchMu.LoadOrStore(key, &sync.Mutex{})
	mu := muAny.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	if v, ok := balanceCache.Load(key); ok {
		e := v.(balanceEntry)
		if time.Since(e.fetchedAt) < balanceCacheTTL {
			return e.value, nil
		}
	}

	balance, err := QueryUSDCBalance(address)
	if err != nil {
		return 0, err
	}
	balanceCache.Store(key, balanceEntry{value: balance, fetchedAt: time.Now()})
	return balance, nil
}

// InvalidateBalanceCache drops the cached balance for an address, forcing the
// next query to hit the chain. Use after a known-spending action or when the
// caller suspects the cache is stale.
func InvalidateBalanceCache(address string) {
	key := strings.ToLower(strings.TrimSpace(address))
	if key == "" {
		return
	}
	balanceCache.Delete(key)
}
