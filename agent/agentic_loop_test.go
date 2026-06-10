package agent

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nofx/mcp"
	"nofx/store"
)

// scriptedAIClient returns queued LLMResponses (or errors) in order for
// CallWithRequestFull, and queued plain strings for CallWithRequest.
type scriptedAIClient struct {
	fullResponses []*mcp.LLMResponse
	fullErrs      []error
	fullCalls     int
	fullRequests  []*mcp.Request

	plainResponse string
	plainErr      error
	plainCalls    int
}

func (c *scriptedAIClient) SetAPIKey(apiKey string, customURL string, customModel string) {}
func (c *scriptedAIClient) SetTimeout(timeout time.Duration)                              {}
func (c *scriptedAIClient) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	return c.plainResponse, c.plainErr
}
func (c *scriptedAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	c.plainCalls++
	return c.plainResponse, c.plainErr
}
func (c *scriptedAIClient) CallWithRequestStream(req *mcp.Request, onChunk func(string)) (string, error) {
	if onChunk != nil && c.plainErr == nil {
		onChunk(c.plainResponse)
	}
	return c.plainResponse, c.plainErr
}
func (c *scriptedAIClient) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	idx := c.fullCalls
	c.fullCalls++
	c.fullRequests = append(c.fullRequests, req)
	var err error
	if idx < len(c.fullErrs) {
		err = c.fullErrs[idx]
	}
	if err != nil {
		return nil, err
	}
	if idx < len(c.fullResponses) {
		return c.fullResponses[idx], nil
	}
	return &mcp.LLMResponse{Content: "fallthrough"}, nil
}

func newAgenticTestAgent(client mcp.AIClient) *Agent {
	a := New(nil, nil, DefaultConfig(), slog.Default())
	a.SetAIClient(client)
	return a
}

func toolCall(id, name, args string) mcp.ToolCall {
	return mcp.ToolCall{
		ID:   id,
		Type: "function",
		Function: mcp.ToolCallFunction{
			Name:      name,
			Arguments: args,
		},
	}
}

func TestRunAgenticTurnDirectAnswer(t *testing.T) {
	client := &scriptedAIClient{
		fullResponses: []*mcp.LLMResponse{{Content: "你好，我是 NOFX 助手。"}},
	}
	a := newAgenticTestAgent(client)

	answer, handled, err := a.runAgenticTurn(context.Background(), "default", 1, "zh", "你好", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected turn to be handled")
	}
	if answer != "你好，我是 NOFX 助手。" {
		t.Fatalf("answer = %q", answer)
	}
	if client.fullCalls != 1 {
		t.Fatalf("fullCalls = %d, want 1", client.fullCalls)
	}
	// Tools must be offered on the request.
	if len(client.fullRequests[0].Tools) == 0 {
		t.Fatal("expected tools to be attached to the LLM request")
	}
	// Conversation must be recorded in history.
	msgs := a.history.Get(1)
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("history = %+v, want user+assistant turns", msgs)
	}
}

func TestRunAgenticTurnToolRoundTrip(t *testing.T) {
	client := &scriptedAIClient{
		fullResponses: []*mcp.LLMResponse{
			{ToolCalls: []mcp.ToolCall{toolCall("call_1", "definitely_not_a_tool", "{}")}},
			{Content: "done"},
		},
	}
	a := newAgenticTestAgent(client)

	var toolEvents []string
	onEvent := func(event, data string) {
		if event == StreamEventTool {
			toolEvents = append(toolEvents, data)
		}
	}

	answer, handled, err := a.runAgenticTurn(context.Background(), "default", 2, "en", "do something", onEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || answer != "done" {
		t.Fatalf("handled=%v answer=%q", handled, answer)
	}
	if client.fullCalls != 2 {
		t.Fatalf("fullCalls = %d, want 2", client.fullCalls)
	}
	if len(toolEvents) != 1 || toolEvents[0] != "definitely_not_a_tool" {
		t.Fatalf("toolEvents = %v", toolEvents)
	}

	// The second request must carry the assistant tool-call message and the
	// tool result message with matching ToolCallID.
	second := client.fullRequests[1]
	var sawAssistantToolCall, sawToolResult bool
	for _, m := range second.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) == 1 && m.ToolCalls[0].ID == "call_1" {
			sawAssistantToolCall = true
		}
		if m.Role == "tool" && m.ToolCallID == "call_1" {
			sawToolResult = true
			if !strings.Contains(m.Content, "unknown tool") {
				t.Fatalf("tool result = %q, want unknown-tool error payload", m.Content)
			}
		}
	}
	if !sawAssistantToolCall || !sawToolResult {
		t.Fatalf("tool round-trip messages missing: assistant=%v tool=%v", sawAssistantToolCall, sawToolResult)
	}
}

func TestRunAgenticTurnFirstCallFailureFallsBack(t *testing.T) {
	client := &scriptedAIClient{
		fullErrs: []error{errors.New("upstream 500")},
	}
	a := newAgenticTestAgent(client)

	_, handled, err := a.runAgenticTurn(context.Background(), "default", 3, "zh", "hi", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected fallback (handled=false) when the first LLM call fails")
	}
	if got := a.history.Get(3); len(got) != 0 {
		t.Fatalf("history should stay empty on fallback, got %+v", got)
	}
}

func TestRunAgenticTurnMidLoopFailureReportsExecutedTools(t *testing.T) {
	client := &scriptedAIClient{
		fullResponses: []*mcp.LLMResponse{
			{ToolCalls: []mcp.ToolCall{toolCall("call_1", "definitely_not_a_tool", "{}")}},
		},
		fullErrs: []error{nil, errors.New("upstream timeout")},
	}
	a := newAgenticTestAgent(client)

	answer, handled, err := a.runAgenticTurn(context.Background(), "default", 4, "zh", "do it", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("mid-loop failure must be handled (tools already executed)")
	}
	if !strings.Contains(answer, "definitely_not_a_tool") {
		t.Fatalf("answer should mention the executed tool, got %q", answer)
	}
}

func TestRunAgenticTurnRoundLimitTriggersWrapUp(t *testing.T) {
	responses := make([]*mcp.LLMResponse, 0, agenticMaxToolRounds)
	for i := 0; i < agenticMaxToolRounds; i++ {
		responses = append(responses, &mcp.LLMResponse{
			ToolCalls: []mcp.ToolCall{toolCall("call_x", "definitely_not_a_tool", "{}")},
		})
	}
	client := &scriptedAIClient{
		fullResponses: responses,
		plainResponse: "summary of what happened",
	}
	a := newAgenticTestAgent(client)

	answer, handled, err := a.runAgenticTurn(context.Background(), "default", 5, "en", "loop forever", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true at round limit")
	}
	if answer != "summary of what happened" {
		t.Fatalf("answer = %q, want wrap-up summary", answer)
	}
	if client.fullCalls != agenticMaxToolRounds {
		t.Fatalf("fullCalls = %d, want %d", client.fullCalls, agenticMaxToolRounds)
	}
	if client.plainCalls != 1 {
		t.Fatalf("plainCalls = %d, want 1 wrap-up call", client.plainCalls)
	}
}

func TestRunAgenticTurnIncludesRecentHistory(t *testing.T) {
	client := &scriptedAIClient{
		fullResponses: []*mcp.LLMResponse{{Content: "answer"}},
	}
	a := newAgenticTestAgent(client)
	a.history.Add(6, "user", "前一个问题")
	a.history.Add(6, "assistant", "前一个回答")

	if _, handled, err := a.runAgenticTurn(context.Background(), "default", 6, "zh", "新问题", nil); err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}

	req := client.fullRequests[0]
	var sawPrevUser, sawPrevAssistant bool
	for _, m := range req.Messages {
		if m.Role == "user" && m.Content == "前一个问题" {
			sawPrevUser = true
		}
		if m.Role == "assistant" && m.Content == "前一个回答" {
			sawPrevAssistant = true
		}
	}
	if !sawPrevUser || !sawPrevAssistant {
		t.Fatalf("recent history missing from request: user=%v assistant=%v", sawPrevUser, sawPrevAssistant)
	}
}

func TestShouldUseAgenticTurn(t *testing.T) {
	t.Setenv("NOFX_AGENT_V2", "")

	a := newAgenticTestAgent(&scriptedAIClient{})
	if !a.shouldUseAgenticTurn(10) {
		t.Fatal("fresh conversation with AI client should use the agentic turn")
	}

	t.Run("disabled by env", func(t *testing.T) {
		t.Setenv("NOFX_AGENT_V2", "off")
		if a.shouldUseAgenticTurn(10) {
			t.Fatal("env kill switch must disable the agentic turn")
		}
	})

	t.Run("no AI client", func(t *testing.T) {
		noAI := New(nil, nil, DefaultConfig(), slog.Default())
		if noAI.shouldUseAgenticTurn(10) {
			t.Fatal("agentic turn requires an AI client")
		}
	})

	t.Run("suspended task snapshot stays on legacy stack", func(t *testing.T) {
		st, err := store.New(filepath.Join(t.TempDir(), "agentic-snapshot.db"))
		if err != nil {
			t.Fatalf("create store: %v", err)
		}
		b := New(nil, st, DefaultConfig(), slog.Default())
		b.SetAIClient(&scriptedAIClient{})
		if !b.shouldUseAgenticTurn(12) {
			t.Fatal("fresh conversation should use the agentic turn")
		}
		b.SnapshotManager(12).Save(SuspendedTask{
			SnapshotID: "snap_test",
			Kind:       "skill",
			ResumeHint: "continue strategy create",
		})
		if b.shouldUseAgenticTurn(12) {
			t.Fatal("suspended task snapshot must stay on the legacy stack so it can be resumed")
		}
	})

	t.Run("active legacy session stays on legacy stack", func(t *testing.T) {
		st, err := store.New(filepath.Join(t.TempDir(), "agentic-guard.db"))
		if err != nil {
			t.Fatalf("create store: %v", err)
		}
		b := New(nil, st, DefaultConfig(), slog.Default())
		b.SetAIClient(&scriptedAIClient{})
		if !b.shouldUseAgenticTurn(11) {
			t.Fatal("fresh conversation should use the agentic turn")
		}
		b.saveActiveSkillSession(newActiveSkillSession(11, "strategy_management", "create"))
		if b.shouldUseAgenticTurn(11) {
			t.Fatal("active skill session must stay on the legacy stack")
		}
	})
}

func TestAgentV2Enabled(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", true},
		{"1", true},
		{"true", true},
		{"on", true},
		{"0", false},
		{"false", false},
		{"off", false},
		{"disabled", false},
	}
	for _, tc := range cases {
		t.Run("value="+tc.value, func(t *testing.T) {
			t.Setenv("NOFX_AGENT_V2", tc.value)
			if got := agentV2Enabled(); got != tc.want {
				t.Errorf("agentV2Enabled() with %q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}
