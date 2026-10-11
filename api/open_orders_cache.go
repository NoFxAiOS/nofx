package api

import (
	"sync"
	"time"

	"nofx/trader"
)

// The positions page polls /api/positions every few seconds and, for every
// open position, used to hit the exchange LIVE for its resting orders (on
// Binance two requests per symbol: openOrders + the algo SL/TP legs). With a
// handful of positions and a few traders in the UI that was hundreds of
// request-weight per minute from page refreshes alone, on the same IP budget
// the trading loop needs. Account/positions are already served from the
// trader's own 15 s cache; this gives the per-symbol order read the same
// treatment. The trading loop does NOT use this cache (it reads the exchange
// directly), so protection logic never sees stale orders.
const openOrdersUITTL = 10 * time.Second

type openOrdersEntry struct {
	orders []trader.OpenOrder
	at     time.Time
}

type openOrdersCache struct {
	mu      sync.Mutex
	entries map[string]openOrdersEntry
	keyMu   map[string]*sync.Mutex // serialises misses per key so a page burst fetches once
	now     func() time.Time
}

var uiOpenOrders = &openOrdersCache{
	entries: map[string]openOrdersEntry{},
	keyMu:   map[string]*sync.Mutex{},
	now:     time.Now,
}

// get returns the cached orders for key or calls fetch (errors are never cached).
func (c *openOrdersCache) get(key string, fetch func() ([]trader.OpenOrder, error)) ([]trader.OpenOrder, error) {
	if orders, ok := c.lookup(key); ok {
		return orders, nil
	}
	c.mu.Lock()
	km := c.keyMu[key]
	if km == nil {
		km = &sync.Mutex{}
		c.keyMu[key] = km
	}
	c.mu.Unlock()

	km.Lock()
	defer km.Unlock()
	if orders, ok := c.lookup(key); ok { // filled while we waited
		return orders, nil
	}
	orders, err := fetch()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if len(c.entries) > 1000 {
		c.entries = map[string]openOrdersEntry{}
		c.keyMu = map[string]*sync.Mutex{}
	}
	c.entries[key] = openOrdersEntry{orders: append([]trader.OpenOrder(nil), orders...), at: c.now()}
	c.mu.Unlock()
	return orders, nil
}

func (c *openOrdersCache) lookup(key string) ([]trader.OpenOrder, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || c.now().Sub(e.at) >= openOrdersUITTL {
		return nil, false
	}
	return append([]trader.OpenOrder(nil), e.orders...), true
}

// uiOpenOrdersFor reads a trader's resting orders for a UI endpoint through
// the short cache.
func uiOpenOrdersFor(traderID, symbol string, at *trader.AutoTrader) ([]trader.OpenOrder, error) {
	return uiOpenOrders.get(traderID+"|"+symbol, func() ([]trader.OpenOrder, error) {
		return at.GetOpenOrders(symbol)
	})
}
