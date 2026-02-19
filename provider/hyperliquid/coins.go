package hyperliquid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/logger"
	"sort"
	"sync"
	"time"
)

const (
	hyperliquidInfoURL = "https://api.hyperliquid.xyz/info"
	cacheDuration      = 24 * time.Hour // Cache for 24 hours
)

// CoinInfo represents basic coin information
type CoinInfo struct {
	Symbol   string  `json:"symbol"`
	Volume24h float64 `json:"volume_24h"` // 24h volume in USD
}

// CoinProvider provides Hyperliquid coin lists
type CoinProvider struct {
	mu            sync.RWMutex
	allCoins      []CoinInfo
	mainCoins     []CoinInfo
	lastUpdated   time.Time
	httpClient    *http.Client
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
		Name string `json:"name"`
	} `json:"universe"`
}

// assetCtx represents asset context with volume data
type assetCtx struct {
	DayNtlVlm string `json:"dayNtlVlm"` // 24h notional volume
}

// fetchCoins fetches all coins from Hyperliquid API and sorts by volume
func (p *CoinProvider) fetchCoins(ctx context.Context) error {
	// Request metaAndAssetCtxs to get both coin names and volume data
	reqBody := []byte(`{"type": "metaAndAssetCtxs"}`)
	
	req, err := http.NewRequestWithContext(ctx, "POST", hyperliquidInfoURL, 
		bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch coin data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Response is an array: [meta, [assetCtxs...]]
	var rawResp []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(rawResp) < 2 {
		return fmt.Errorf("unexpected response format")
	}

	// Parse meta
	var meta metaResponse
	if err := json.Unmarshal(rawResp[0], &meta); err != nil {
		return fmt.Errorf("failed to parse meta: %w", err)
	}

	// Parse asset contexts
	var ctxs []assetCtx
	if err := json.Unmarshal(rawResp[1], &ctxs); err != nil {
		return fmt.Errorf("failed to parse asset contexts: %w", err)
	}

	// Build coin list with volume
	var coins []CoinInfo
	for i, u := range meta.Universe {
		var vol float64
		if i < len(ctxs) {
			fmt.Sscanf(ctxs[i].DayNtlVlm, "%f", &vol)
		}
		coins = append(coins, CoinInfo{
			Symbol:    u.Name,
			Volume24h: vol,
		})
	}

	// Sort by volume descending
	sort.Slice(coins, func(i, j int) bool {
		return coins[i].Volume24h > coins[j].Volume24h
	})

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
