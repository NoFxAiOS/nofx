package agent

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"nofx/provider/nofxos"
)

func withStubbedAI500(t *testing.T, fn func(walletKey string) ([]nofxos.CoinData, error)) {
	t.Helper()
	original := fetchAI500ForTool
	fetchAI500ForTool = fn
	t.Cleanup(func() { fetchAI500ForTool = original })
}

func TestToolGetAI500ListSortsByScoreAndLimits(t *testing.T) {
	withStubbedAI500(t, func(walletKey string) ([]nofxos.CoinData, error) {
		return []nofxos.CoinData{
			{Pair: "LOWUSDT", Score: 10, IncreasePercent: -3},
			{Pair: "TOPUSDT", Score: 99, IncreasePercent: 42},
			{Pair: "MIDUSDT", Score: 55, IncreasePercent: 7},
		}, nil
	})

	a := New(nil, nil, DefaultConfig(), slog.Default())
	raw := a.toolGetAI500List("default", `{"limit": 2}`)

	var resp struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
		Coins  []struct {
			Pair            string  `json:"pair"`
			Score           float64 `json:"score"`
			IncreasePercent float64 `json:"increase_percent"`
		} `json:"coins"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	if resp.Status != "ok" || resp.Count != 2 || len(resp.Coins) != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Coins[0].Pair != "TOPUSDT" || resp.Coins[1].Pair != "MIDUSDT" {
		t.Fatalf("expected score-descending order, got %+v", resp.Coins)
	}
}

func TestToolGetAI500ListDefaultLimit(t *testing.T) {
	coins := make([]nofxos.CoinData, 30)
	for i := range coins {
		coins[i] = nofxos.CoinData{Pair: "C", Score: float64(i)}
	}
	withStubbedAI500(t, func(walletKey string) ([]nofxos.CoinData, error) {
		return coins, nil
	})

	a := New(nil, nil, DefaultConfig(), slog.Default())
	var resp struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(a.toolGetAI500List("default", "")), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Count != 20 {
		t.Fatalf("default limit = %d, want 20", resp.Count)
	}
}

func TestToolGetAI500ListUpstreamError(t *testing.T) {
	withStubbedAI500(t, func(walletKey string) ([]nofxos.CoinData, error) {
		return nil, errors.New("upstream down")
	})

	a := New(nil, nil, DefaultConfig(), slog.Default())
	raw := a.toolGetAI500List("default", "")
	if !strings.Contains(raw, `"error"`) {
		t.Fatalf("expected error payload, got %q", raw)
	}
}

func TestHandleToolCallDispatchesAI500(t *testing.T) {
	withStubbedAI500(t, func(walletKey string) ([]nofxos.CoinData, error) {
		return []nofxos.CoinData{{Pair: "BTCUSDT", Score: 90}}, nil
	})

	a := New(nil, nil, DefaultConfig(), slog.Default())
	raw := a.handleToolCall(t.Context(), "default", 1, "zh", toolCall("c1", "get_ai500_list", "{}"))
	if !strings.Contains(raw, "BTCUSDT") {
		t.Fatalf("dispatch failed, got %q", raw)
	}
}

func TestAgentToolsIncludeAI500(t *testing.T) {
	for _, tool := range agentTools() {
		if tool.Function.Name == "get_ai500_list" {
			return
		}
	}
	t.Fatal("get_ai500_list missing from agent toolset")
}
