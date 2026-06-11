package hyperliquid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/logger"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hyperliquidInfoURL = "https://api.hyperliquid.xyz/info"
	cacheDuration      = 24 * time.Hour // Cache for 24 hours
)

// CoinInfo represents basic Hyperliquid market information.
type CoinInfo struct {
	Symbol       string  `json:"symbol"`
	Volume24h    float64 `json:"volume_24h"` // 24h notional volume in USD
	MarkPrice    float64 `json:"mark_price"`
	PrevDayPrice float64 `json:"prev_day_price,omitempty"`
	Change24hPct float64 `json:"change_24h_pct,omitempty"`
	MaxLeverage  int     `json:"max_leverage,omitempty"`
	SzDecimals   int     `json:"sz_decimals,omitempty"`
}

// XYZCategory returns the NOFX product category for a Hyperliquid XYZ base symbol.
func XYZCategory(baseSymbol string) string {
	baseSymbol = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(baseSymbol, "xyz:")))
	switch baseSymbol {
	case "TSLA", "NVDA", "AAPL", "MSFT", "GOOGL", "GOOG", "AMZN", "META", "NFLX", "AMD", "INTC", "COIN", "MSTR", "PLTR", "HOOD", "CRCL", "SNDK", "MU", "SMSN", "DRAM", "SKHX", "BABA", "ASML", "AVGO", "IONQ", "RGTI", "RKLB", "SMCI", "MARA", "RIOT", "MRVL", "SNOW", "CRM", "ORCL", "ADBE", "PYPL", "SHOP", "UBER", "SPOT", "ABNB", "RDDT", "ARM", "SOFI", "XYZ", "LVMH", "PDD", "NVO", "SONY", "DIS", "WMT", "NKE", "JPM", "BAC", "V", "MA", "JNJ", "PG", "UNH", "HD", "XOM", "CVX", "TM", "RACE", "VOW3", "BMW", "MBG":
		return "stock"
	case "GOLD", "SILVER", "COPPER", "NATGAS", "URANIUM", "ALUMINIUM", "PLATINUM", "PALLADIUM", "BRENTOIL", "CL", "CORN", "WHEAT", "TTF":
		return "commodity"
	case "SPX", "NDX", "DJI", "VIX", "DAX", "FTSE", "NIKKEI", "HSI", "CSI300", "XYZ100", "XYZ25", "XYZ50":
		return "index"
	case "EUR", "GBP", "JPY", "AUD", "CAD", "CHF", "MXN", "BRL", "TRY", "ZAR", "CNH", "KRW":
		return "forex"
	case "OPENAI", "ANTHROPIC", "SPACEX", "STRIPE", "FIGMA", "DATBRICKS", "PERPLEXITY", "XAI", "BYTEDANCE", "REVOLUT":
		return "pre_ipo"
	default:
		return "stock"
	}
}

// CoinProvider provides Hyperliquid coin lists
type CoinProvider struct {
	mu          sync.RWMutex
	allCoins    []CoinInfo
	mainCoins   []CoinInfo
	lastUpdated time.Time
	httpClient  *http.Client
}

var (
	defaultProvider *CoinProvider
	providerOnce    sync.Once
)

// GetProvider returns the singleton CoinProvider instance
func GetProvider() *CoinProvider {
	providerOnce.Do(func() {
		defaultProvider = &CoinProvider{
			httpClient: &http.Client{Timeout: 30 * time.Second},
		}
	})
	return defaultProvider
}

// metaResponse represents the response from Hyperliquid meta endpoint
type metaResponse struct {
	Universe []struct {
		Name        string `json:"name"`
		SzDecimals  int    `json:"szDecimals"`
		MaxLeverage int    `json:"maxLeverage"`
	} `json:"universe"`
}

// assetCtx represents asset context with market data.
type assetCtx struct {
	DayNtlVlm string `json:"dayNtlVlm"` // 24h notional volume
	MarkPx    string `json:"markPx"`
	PrevDayPx string `json:"prevDayPx"`
}

func fetchPerpDexCoins(ctx context.Context, client *http.Client, dex string) ([]CoinInfo, error) {
	reqPayload := map[string]string{"type": "metaAndAssetCtxs"}
	if dex != "" {
		reqPayload["dex"] = dex
	}
	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", hyperliquidInfoURL,
		bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch coin data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Response is an array: [meta, [assetCtxs...]]
	var rawResp []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(rawResp) < 2 {
		return nil, fmt.Errorf("unexpected response format")
	}

	var meta metaResponse
	if err := json.Unmarshal(rawResp[0], &meta); err != nil {
		return nil, fmt.Errorf("failed to parse meta: %w", err)
	}

	var ctxs []assetCtx
	if err := json.Unmarshal(rawResp[1], &ctxs); err != nil {
		return nil, fmt.Errorf("failed to parse asset contexts: %w", err)
	}

	coins := make([]CoinInfo, 0, len(meta.Universe))
	for i, u := range meta.Universe {
		var vol, mark, prevDay, change24hPct float64
		if i < len(ctxs) {
			vol, _ = strconv.ParseFloat(ctxs[i].DayNtlVlm, 64)
			mark, _ = strconv.ParseFloat(ctxs[i].MarkPx, 64)
			prevDay, _ = strconv.ParseFloat(ctxs[i].PrevDayPx, 64)
			if prevDay > 0 && mark > 0 {
				change24hPct = ((mark - prevDay) / prevDay) * 100
			}
		}
		coins = append(coins, CoinInfo{
			Symbol:       u.Name,
			Volume24h:    vol,
			MarkPrice:    mark,
			PrevDayPrice: prevDay,
			Change24hPct: change24hPct,
			MaxLeverage:  u.MaxLeverage,
			SzDecimals:   u.SzDecimals,
		})
	}

	sort.Slice(coins, func(i, j int) bool {
		return coins[i].Volume24h > coins[j].Volume24h
	})
	return coins, nil
}

// perpDexCacheTTL bounds how often the perp-dex symbol board is re-fetched.
// The tradable symbol list changes rarely; prices/volume on the board are
// display hints, so short staleness is far better than hammering the
// Hyperliquid API (which rate-limits with 429) on every panel render.
const perpDexCacheTTL = 5 * time.Minute

type perpDexCacheEntry struct {
	coins     []CoinInfo
	fetchedAt time.Time
}

type perpDexCacheStore struct {
	mu      sync.Mutex
	entries map[string]perpDexCacheEntry
}

var perpDexCoinCache = &perpDexCacheStore{entries: map[string]perpDexCacheEntry{}}

// fetchPerpDexCoinsFn is swappable in tests.
var fetchPerpDexCoinsFn = fetchPerpDexCoins

// GetPerpDexCoins returns current tradable USDC perp assets for a given
// Hyperliquid dex, served from a TTL cache. When the upstream fetch fails
// (e.g. HTTP 429 rate limiting) and stale data exists, the stale board is
// served instead of an error so the UI keeps working.
func GetPerpDexCoins(ctx context.Context, dex string) ([]CoinInfo, error) {
	perpDexCoinCache.mu.Lock()
	defer perpDexCoinCache.mu.Unlock()

	entry, hasCache := perpDexCoinCache.entries[dex]
	if hasCache && time.Since(entry.fetchedAt) < perpDexCacheTTL {
		return copyCoins(entry.coins), nil
	}

	coins, err := fetchPerpDexCoinsFn(ctx, &http.Client{Timeout: 30 * time.Second}, dex)
	if err != nil {
		if hasCache {
			logger.Infof("⚠️ Hyperliquid perp-dex fetch failed (%v); serving cached board for dex %q from %s",
				err, dex, entry.fetchedAt.Format(time.RFC3339))
			return copyCoins(entry.coins), nil
		}
		return nil, err
	}

	perpDexCoinCache.entries[dex] = perpDexCacheEntry{coins: coins, fetchedAt: time.Now()}
	return copyCoins(coins), nil
}

// copyCoins returns a defensive copy so callers cannot mutate the cache.
func copyCoins(coins []CoinInfo) []CoinInfo {
	out := make([]CoinInfo, len(coins))
	copy(out, coins)
	return out
}

// fetchCoins fetches all default Hyperliquid crypto coins and sorts by volume
func (p *CoinProvider) fetchCoins(ctx context.Context) error {
	coins, err := fetchPerpDexCoins(ctx, p.httpClient, "")
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.allCoins = coins
	// Main coins are top 20 by volume
	if len(coins) > 20 {
		p.mainCoins = coins[:20]
	} else {
		p.mainCoins = coins
	}
	p.lastUpdated = time.Now()

	logger.Infof("✅ Hyperliquid coin list updated: %d total coins, top 20 by volume cached", len(coins))

	return nil
}

// ensureUpdated checks if cache is stale and refreshes if needed
func (p *CoinProvider) ensureUpdated(ctx context.Context) error {
	p.mu.RLock()
	needsUpdate := time.Since(p.lastUpdated) > cacheDuration || len(p.allCoins) == 0
	p.mu.RUnlock()

	if needsUpdate {
		return p.fetchCoins(ctx)
	}
	return nil
}

// GetAllCoins returns all available Hyperliquid perp coins
func (p *CoinProvider) GetAllCoins(ctx context.Context) ([]CoinInfo, error) {
	if err := p.ensureUpdated(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to avoid mutation
	result := make([]CoinInfo, len(p.allCoins))
	copy(result, p.allCoins)
	return result, nil
}

// GetMainCoins returns top N coins by 24h volume
func (p *CoinProvider) GetMainCoins(ctx context.Context, limit int) ([]CoinInfo, error) {
	if err := p.ensureUpdated(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	// Return top N coins
	count := limit
	if count > len(p.allCoins) {
		count = len(p.allCoins)
	}

	result := make([]CoinInfo, count)
	copy(result, p.allCoins[:count])
	return result, nil
}

// GetCoinSymbols returns just the symbol names (for compatibility)
func GetAllCoinSymbols(ctx context.Context) ([]string, error) {
	coins, err := GetProvider().GetAllCoins(ctx)
	if err != nil {
		return nil, err
	}

	symbols := make([]string, len(coins))
	for i, c := range coins {
		symbols[i] = c.Symbol
	}
	return symbols, nil
}

// GetMainCoinSymbols returns top N coin symbols by volume
func GetMainCoinSymbols(ctx context.Context, limit int) ([]string, error) {
	coins, err := GetProvider().GetMainCoins(ctx, limit)
	if err != nil {
		return nil, err
	}

	symbols := make([]string, len(coins))
	for i, c := range coins {
		symbols[i] = c.Symbol
	}
	return symbols, nil
}

// ForceRefresh forces a refresh of the coin cache
func (p *CoinProvider) ForceRefresh(ctx context.Context) error {
	return p.fetchCoins(ctx)
}
