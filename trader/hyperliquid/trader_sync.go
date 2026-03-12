package hyperliquid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strings"
	"time"
)

// refreshMetaIfNeeded refreshes meta information when invalid (triggered when Asset ID is 0)
func (t *HyperliquidTrader) refreshMetaIfNeeded(coin string) error {
	assetID := t.exchange.Info().NameToAsset(coin)
	if assetID != 0 {
		return nil // Meta is normal, no refresh needed
	}

	logger.Infof("⚠️  Asset ID for %s is 0, attempting to refresh Meta information...", coin)

	// Refresh Meta information
	meta, err := t.exchange.Info().Meta(t.ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh Meta information: %w", err)
	}

	// Concurrency safe: Use write lock to protect meta field update
	t.metaMutex.Lock()
	t.meta = meta
	t.metaMutex.Unlock()

	logger.Infof("✅ Meta information refreshed, contains %d assets", len(meta.Universe))

	// Verify Asset ID after refresh
	assetID = t.exchange.Info().NameToAsset(coin)
	if assetID == 0 {
		return fmt.Errorf("❌ Even after refreshing Meta, Asset ID for %s is still 0. Possible reasons:\n"+
			"  1. This coin is not listed on Hyperliquid\n"+
			"  2. Coin name is incorrect (should be BTC not BTCUSDT)\n"+
			"  3. API connection issue", coin)
	}

	logger.Infof("✅ Asset ID check passed after refresh: %s -> %d", coin, assetID)
	return nil
}

// fetchXyzMeta fetches metadata for xyz dex assets (stocks, forex, commodities)
func (t *HyperliquidTrader) fetchXyzMeta() error {
	// Build request for xyz dex meta
	reqBody := map[string]string{
		"type": "meta",
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := "https://api.hyperliquid.xyz/info"

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xyz dex meta API error (status %d): %s", resp.StatusCode, string(body))
	}

	var meta xyzDexMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	t.xyzMetaMutex.Lock()
	t.xyzMeta = &meta
	t.xyzMetaMutex.Unlock()

	logger.Infof("✅ xyz dex meta fetched, contains %d assets", len(meta.Universe))
	return nil
}

// getXyzSzDecimals gets quantity precision for xyz dex asset
func (t *HyperliquidTrader) getXyzSzDecimals(coin string) int {
	t.xyzMetaMutex.RLock()
	defer t.xyzMetaMutex.RUnlock()

	if t.xyzMeta == nil {
		logger.Infof("⚠️  xyz meta information is empty, using default precision 2")
		return 2 // Default precision for stocks/forex
	}

	// The meta API returns names with xyz: prefix, so ensure we match correctly
	lookupName := coin
	if !strings.HasPrefix(lookupName, "xyz:") {
		lookupName = "xyz:" + lookupName
	}

	// Find corresponding asset in xyzMeta.Universe
	for _, asset := range t.xyzMeta.Universe {
		if asset.Name == lookupName {
			return asset.SzDecimals
		}
	}

	logger.Infof("⚠️  Precision information not found for %s, using default precision 2", lookupName)
	return 2 // Default precision for stocks/forex
}

// getXyzAssetIndex gets the asset index for an xyz dex asset
func (t *HyperliquidTrader) getXyzAssetIndex(baseCoin string) int {
	t.xyzMetaMutex.RLock()
	defer t.xyzMetaMutex.RUnlock()

	if t.xyzMeta == nil {
		return -1
	}

	// The meta API returns names with xyz: prefix, so ensure we match correctly
	lookupName := baseCoin
	if !strings.HasPrefix(lookupName, "xyz:") {
		lookupName = "xyz:" + lookupName
	}

	for i, asset := range t.xyzMeta.Universe {
		if asset.Name == lookupName {
			return i
		}
	}
	return -1
}
