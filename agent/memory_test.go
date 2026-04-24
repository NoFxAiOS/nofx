package agent

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nofx/mcp"
	"nofx/store"
)

type fakeAIClient struct {
	callCount int
}

func (f *fakeAIClient) SetAPIKey(string, string, string) {}
func (f *fakeAIClient) SetTimeout(time.Duration)         {}
func (f *fakeAIClient) CallWithMessages(string, string) (string, error) {
	return "", nil
}
func (f *fakeAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	f.callCount++
	return `{"current_goal":"continue setup","active_flow":"onboarding","open_loops":["finish trader setup after external exchange/model configuration is ready"],"important_facts":["user selected OKX"],"last_decision":{"action":"paused setup","reason":"user asked a market question","still_valid":true},"updated_at":"2026-04-01T00:00:00Z"}`, nil
}
func (f *fakeAIClient) CallWithRequestStream(req *mcp.Request, onChunk func(string)) (string, error) {
	return "", nil
}
func (f *fakeAIClient) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	return nil, nil
}

func TestMaybeCompressHistoryKeepsRecentThreeRounds(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "nofxi-test.db"))
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}

	fakeClient := &fakeAIClient{}
	a := &Agent{
		store:    st,
		logger:   slog.Default(),
		history:  newChatHistory(100),
		aiClient: fakeClient,
	}

	userID := int64(42)
	payload := strings.Repeat("BTC ETH market context ", 20)
	for i := 0; i < 6; i++ {
		a.history.Add(userID, "user", "user turn #"+string(rune('0'+i))+" "+payload)
		a.history.Add(userID, "assistant", "assistant turn #"+string(rune('0'+i))+" "+payload)
	}

	a.maybeCompressHistory(context.Background(), userID)

	msgs := a.history.Get(userID)
	if len(msgs) != recentConversationMessages {
		t.Fatalf("expected %d recent messages, got %d", recentConversationMessages, len(msgs))
	}
	if fakeClient.callCount != 1 {
		t.Fatalf("expected summarizer to be called once, got %d", fakeClient.callCount)
	}

	state := a.getTaskState(userID)
	if state.CurrentGoal != "continue setup" {
		t.Fatalf("expected persisted task state goal, got %#v", state)
	}
	if state.LastDecision == nil || state.LastDecision.Action != "paused setup" {
		t.Fatalf("expected persisted last_decision, got %#v", state.LastDecision)
	}
	if len(state.OpenLoops) != 1 || state.OpenLoops[0] != "finish trader setup after external exchange/model configuration is ready" {
		t.Fatalf("expected high-level open loop, got %#v", state.OpenLoops)
	}
	if strings.Contains(msgs[0].Content, "#0") {
		t.Fatalf("expected oldest round to be compressed away, first recent message = %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "#3") {
		t.Fatalf("expected recent window to start from round #3, got %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[len(msgs)-1].Content, "#5") {
		t.Fatalf("expected latest round to remain in short-term history, got %q", msgs[len(msgs)-1].Content)
	}
}

func TestNormalizeTaskStateDropsExecutionLevelOpenLoops(t *testing.T) {
	state := normalizeTaskState(TaskState{
		OpenLoops: []string{
			"wait for API secret",
			"call get_exchange_configs",
			"finish trader setup after external configuration is ready",
		},
	})

	if len(state.OpenLoops) != 1 {
		t.Fatalf("expected only one high-level open loop to remain, got %#v", state.OpenLoops)
	}
	if state.OpenLoops[0] != "finish trader setup after external configuration is ready" {
		t.Fatalf("unexpected open loop after normalization: %#v", state.OpenLoops)
	}
}

func TestMaybeUpdateTaskStateIncrementallyPersistsShortConversationFacts(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "nofxi-test.db"))
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}

	fakeClient := &fakeAIClient{}
	a := &Agent{
		store:    st,
		logger:   slog.Default(),
		history:  newChatHistory(100),
		aiClient: fakeClient,
	}

	userID := int64(7)
	a.history.Add(userID, "user", "我是在运行测试1交易员时遇到的，错误是运行时出现的")
	a.history.Add(userID, "assistant", "我会继续排查测试1交易员的运行时错误")

	a.maybeUpdateTaskStateIncrementally(context.Background(), userID)

	if fakeClient.callCount != 1 {
		t.Fatalf("expected incremental summarizer to be called once, got %d", fakeClient.callCount)
	}

	state := a.getTaskState(userID)
	if state.CurrentGoal != "continue setup" {
		t.Fatalf("expected incrementally persisted task state, got %#v", state)
	}
}
