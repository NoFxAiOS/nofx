package hyperliquid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
)

func (t *HyperliquidTrader) apiBaseURL() string {
	if t.apiURL != "" {
		return t.apiURL
	}
	if t.isTestnet {
		return hyperliquid.TestnetAPIURL
	}
	return hyperliquid.MainnetAPIURL
}

// CheckTradingAuthorization is read-only: balances alone cannot validate an API
// wallet. Never attempt to approve an agent or substitute another signing key.
func (t *HyperliquidTrader) CheckTradingAuthorization() error {
	if t.privateKey == nil || t.walletAddr == "" {
		return fmt.Errorf("Hyperliquid signing agent or account is not configured")
	}
	agent := crypto.PubkeyToAddress(t.privateKey.PublicKey).Hex()
	if strings.EqualFold(agent, t.walletAddr) {
		return nil
	}
	ctx, cancel := context.WithTimeout(t.ctx, 10*time.Second)
	defer cancel()
	return checkAgentAuthorization(ctx, http.DefaultClient, t.apiBaseURL(), t.walletAddr, agent, time.Now())
}

func checkAgentAuthorization(ctx context.Context, client *http.Client, baseURL, owner, agent string, now time.Time) error {
	payload, _ := json.Marshal(map[string]string{"type": "extraAgents", "user": owner})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/info", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot verify Hyperliquid trading authorization; no orders will be sent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot verify Hyperliquid trading authorization (HTTP %d); no orders will be sent", resp.StatusCode)
	}
	var agents []struct {
		Address    string `json:"address"`
		ValidUntil int64  `json:"validUntil"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&agents); err != nil {
		return fmt.Errorf("invalid Hyperliquid authorization response: %w", err)
	}
	for _, approved := range agents {
		if strings.EqualFold(approved.Address, agent) && approved.ValidUntil > now.UnixMilli() {
			return nil
		}
	}
	return fmt.Errorf("configured Hyperliquid agent %s is not authorized or has expired. Reconnect Hyperliquid and sign trading authorization with your main wallet. Automatic opening and closing are unavailable", agent)
}

// The SDK's default exchange indexes only core perps. HIP-3 leverage must use
// its own dex metadata (including the dex offset), not the core asset index.
func (t *HyperliquidTrader) leverageExchange(coin string) (*hyperliquid.Exchange, error) {
	if !strings.HasPrefix(coin, "xyz:") {
		return t.exchange, nil
	}
	t.xyzExchangeMu.Lock()
	defer t.xyzExchangeMu.Unlock()
	if t.xyzExchange != nil {
		return t.xyzExchange, nil
	}
	ctx, cancel := context.WithTimeout(t.ctx, 15*time.Second)
	defer cancel()
	// v0.36's constructor Meta(ctx) does not inherit PerpDexName. Fetch the
	// builder universe explicitly, otherwise core coins receive xyz asset IDs.
	info := hyperliquid.NewInfo(ctx, t.apiBaseURL(), true, &hyperliquid.Meta{}, &hyperliquid.SpotMeta{}, nil)
	meta, err := info.Meta(ctx, "xyz")
	if err != nil {
		return nil, fmt.Errorf("cannot load xyz market metadata: %w", err)
	}
	xyzMeta := &xyzDexMeta{}
	for _, asset := range meta.Universe {
		xyzMeta.Universe = append(xyzMeta.Universe, xyzAssetInfo{Name: asset.Name, SzDecimals: asset.SzDecimals, MaxLeverage: asset.MaxLeverage})
	}
	t.xyzMetaMutex.Lock()
	t.xyzMeta = xyzMeta
	t.xyzMetaMutex.Unlock()
	ex, err := initExchangeClient(func() *hyperliquid.Exchange {
		return hyperliquid.NewExchange(ctx, t.privateKey, t.apiBaseURL(), meta, "", t.walletAddr,
			&hyperliquid.SpotMeta{}, nil, hyperliquid.ExchangeOptPerpDex("xyz"),
			hyperliquid.ExchangeOptClientOptions(hyperliquid.ClientOptHTTPClient(checkedActionHTTPClient())))
	})
	if err != nil {
		return nil, fmt.Errorf("cannot load xyz market metadata: %w", err)
	}
	t.xyzExchange = ex
	return ex, nil
}
