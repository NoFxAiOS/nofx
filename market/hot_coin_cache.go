package market

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type hotCoinCacheEntry struct {
	coins     []HotCoin
	updatedAt time.Time
}

var hotCoinCache sync.Map

func cachedHotCoinList(key string, ttl time.Duration, fetch func() ([]HotCoin, error)) ([]HotCoin, error) {
	if ttl <= 0 {
		ttl = 180 * time.Second
	}
	if cached, ok := hotCoinCache.Load(key); ok {
		entry := cached.(*hotCoinCacheEntry)
		if time.Since(entry.updatedAt) < ttl {
			return cloneHotCoins(entry.coins), nil
		}
	}
	coins, err := fetch()
	if err != nil {
		return nil, err
	}
	hotCoinCache.Store(key, &hotCoinCacheEntry{coins: cloneHotCoins(coins), updatedAt: time.Now().UTC()})
	return coins, nil
}

func hotCoinCacheKey(kind string, limit int, excludedCoins []string, exchange string) string {
	ex := strings.ToLower(exchange)
	excluded := append([]string(nil), excludedCoins...)
	for i := range excluded {
		excluded[i] = strings.ToUpper(strings.TrimSpace(excluded[i]))
	}
	sort.Strings(excluded)
	bucket := ""
	if strings.HasPrefix(kind, "oi_") {
		bucket = fmt.Sprintf("|b%d", time.Now().Unix()/180)
	}
	return kind + "|" + ex + "|" + strings.Join(excluded, ",") + fmt.Sprintf("|%d", limit) + bucket
}

func cloneHotCoins(in []HotCoin) []HotCoin {
	out := make([]HotCoin, len(in))
	copy(out, in)
	return out
}
