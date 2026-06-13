package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"nofx/provider/nofxos"
	"nofx/store"
)

const (
	ai500DefaultLimit = 20
	ai500MaxLimit     = 100
)

// fetchAI500ForTool is swappable in tests. It resolves a nofxos client
// (routed through claw402 when a wallet key is available) and returns the
// cached AI500 board.
var fetchAI500ForTool = func(walletKey string) ([]nofxos.CoinData, error) {
	return nofxos.GetAI500ListCached(nofxos.ResolveClient(walletKey))
}

// Claw402WalletKeyForStoreUser returns the wallet private key of the user's
// enabled claw402 model, if any, so data requests can be routed through the
// claw402 payment gateway on the user's own account.
func Claw402WalletKeyForStoreUser(st *store.Store, storeUserID string) string {
	if st == nil {
		return ""
	}
	if strings.TrimSpace(storeUserID) == "" {
		storeUserID = "default"
	}
	models, err := st.AIModel().List(storeUserID)
	if err != nil {
		return ""
	}
	for _, model := range models {
		if model == nil || !model.Enabled {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(model.Provider), "claw402") && len(model.APIKey) > 0 {
			return string(model.APIKey)
		}
	}
	return ""
}

// AI500BoardEntry is the display shape for one AI500 constituent.
type AI500BoardEntry struct {
	Pair            string  `json:"pair"`
	Score           float64 `json:"score"`
	MaxScore        float64 `json:"max_score"`
	IncreasePercent float64 `json:"increase_percent"`
	StartPrice      float64 `json:"start_price"`
	StartTime       int64   `json:"start_time"`
}

// AI500Board returns the AI500 constituents sorted by score (descending),
// truncated to limit.
func AI500Board(walletKey string, limit int) ([]AI500BoardEntry, error) {
	if limit <= 0 {
		limit = ai500DefaultLimit
	}
	if limit > ai500MaxLimit {
		limit = ai500MaxLimit
	}
	coins, err := fetchAI500ForTool(walletKey)
	if err != nil {
		return nil, err
	}
	sorted := make([]nofxos.CoinData, len(coins))
	copy(sorted, coins)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	out := make([]AI500BoardEntry, 0, len(sorted))
	for _, coin := range sorted {
		out = append(out, AI500BoardEntry{
			Pair:            coin.Pair,
			Score:           coin.Score,
			MaxScore:        coin.MaxScore,
			IncreasePercent: coin.IncreasePercent,
			StartPrice:      coin.StartPrice,
			StartTime:       coin.StartTime,
		})
	}
	return out, nil
}

// toolGetAI500List exposes the AI500 board to the chat agent.
func (a *Agent) toolGetAI500List(storeUserID, argsJSON string) string {
	var args struct {
		Limit int `json:"limit"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
		}
	}

	walletKey := Claw402WalletKeyForStoreUser(a.store, storeUserID)
	entries, err := AI500Board(walletKey, args.Limit)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to fetch AI500 list: %s"}`, err)
	}

	payload, err := json.Marshal(map[string]any{
		"status": "ok",
		"count":  len(entries),
		"coins":  entries,
		"note":   "AI500 is an AI-scored crypto index; score is 0-100, increase_percent is the gain since the coin entered the index. Present this in chat as a short numbered list, one coin per line, e.g. \"1. BEAT — 评分 84.2，入选以来 +404.1%\". NEVER use a markdown table — the chat UI does not render tables.",
	})
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to serialize AI500 list: %s"}`, err)
	}
	return string(payload)
}
