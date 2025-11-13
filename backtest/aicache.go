package backtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"nofx/decision"
	"nofx/market"
)

type cachedDecision struct {
	Key           string                 `json:"key"`
	PromptVariant string                 `json:"prompt_variant"`
	Timestamp     int64                  `json:"ts"`
	Decision      *decision.FullDecision `json:"decision"`
}

// AICache 持久化 AI 决策，便于重复回测或重放。
type AICache struct {
	mu      sync.RWMutex
	path    string
	Entries map[string]cachedDecision `json:"entries"`
}

func LoadAICache(path string) (*AICache, error) {
	if path == "" {
		return nil, fmt.Errorf("ai cache path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	cache := &AICache{
		path:    path,
		Entries: make(map[string]cachedDecision),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return cache, nil
	}
	if err := json.Unmarshal(data, cache); err != nil {
		return nil, err
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]cachedDecision)
	}
	return cache, nil
}

func (c *AICache) Path() string {
	if c == nil {
		return ""
	}
	return c.path
}

func (c *AICache) Get(key string) (*decision.FullDecision, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	c.mu.RLock()
	entry, ok := c.Entries[key]
	c.mu.RUnlock()
	if !ok || entry.Decision == nil {
		return nil, false
	}
	return cloneDecision(entry.Decision), true
}

func (c *AICache) Put(key string, variant string, ts int64, decision *decision.FullDecision) error {
	if c == nil || key == "" || decision == nil {
		return nil
	}
	entry := cachedDecision{
		Key:           key,
		PromptVariant: variant,
		Timestamp:     ts,
		Decision:      cloneDecision(decision),
	}
	c.mu.Lock()
	c.Entries[key] = entry
	c.mu.Unlock()
	return c.save()
}

func (c *AICache) save() error {
	if c == nil || c.path == "" {
		return nil
	}
	c.mu.RLock()
	data, err := json.MarshalIndent(c, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	return writeFileAtomic(c.path, data, 0o644)
}

func cloneDecision(src *decision.FullDecision) *decision.FullDecision {
	if src == nil {
		return nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return nil
	}
	var dst decision.FullDecision
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil
	}
	return &dst
}

func computeCacheKey(ctx *decision.Context, variant string, ts int64) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}
	payload := struct {
		Variant        string                   `json:"variant"`
		Timestamp      int64                    `json:"ts"`
		CurrentTime    string                   `json:"current_time"`
		Account        decision.AccountInfo     `json:"account"`
		Positions      []decision.PositionInfo  `json:"positions"`
		CandidateCoins []decision.CandidateCoin `json:"candidate_coins"`
		MarketData     map[string]market.Data   `json:"market"`
		MarginUsedPct  float64                  `json:"margin_used_pct"`
		Runtime        int                      `json:"runtime_minutes"`
		CallCount      int                      `json:"call_count"`
	}{
		Variant:        variant,
		Timestamp:      ts,
		CurrentTime:    ctx.CurrentTime,
		Account:        ctx.Account,
		Positions:      ctx.Positions,
		CandidateCoins: ctx.CandidateCoins,
		MarginUsedPct:  ctx.Account.MarginUsedPct,
		Runtime:        ctx.RuntimeMinutes,
		CallCount:      ctx.CallCount,
		MarketData:     make(map[string]market.Data, len(ctx.MarketDataMap)),
	}

	for symbol, data := range ctx.MarketDataMap {
		if data == nil {
			continue
		}
		payload.MarketData[symbol] = *data
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:]), nil
}
