package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nofx/mcp"
)

// mockLLM implements mcp.AIClient using pre-programmed LLMResponse objects.
// Native function calling: CallWithRequestFull is the primary method;
// CallWithRequest and CallWithRequestStream are stubs kept for interface compliance.
type mockLLM struct {
	responses []*mcp.LLMResponse
	calls     int
	lastMsgs  []mcp.Message
}

func (m *mockLLM) SetAPIKey(_, _, _ string)   {}
func (m *mockLLM) SetTimeout(_ time.Duration) {}

func (m *mockLLM) CallWithMessages(_, _ string) (string, error) { return "", nil }

func (m *mockLLM) CallWithRequest(req *mcp.Request) (string, error) {
	r, err := m.next()
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

func (m *mockLLM) CallWithRequestStream(req *mcp.Request, onChunk func(string)) (string, error) {
	r, err := m.next()
	if err != nil {
		return "", err
	}
	if onChunk != nil {
		onChunk(r.Content)
	}
	return r.Content, nil
}

func (m *mockLLM) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	m.lastMsgs = req.Messages
	return m.next()
}

func (m *mockLLM) next() (*mcp.LLMResponse, error) {
	if m.calls < len(m.responses) {
		r := m.responses[m.calls]
		m.calls++
		return r, nil
	}
	return &mcp.LLMResponse{Content: "OK"}, nil
}

// toolCall builds a mock LLM response that contains a single tool invocation.
func toolCall(id, method, path string, body string) *mcp.LLMResponse {
	if body == "" {
		body = "{}"
	}
	return &mcp.LLMResponse{
		ToolCalls: []mcp.ToolCall{{
			ID:   id,
			Type: "function",
			Function: mcp.ToolCallFunction{
				Name:      "api_request",
				Arguments: fmt.Sprintf(`{"method":%q,"path":%q,"body":%s}`, method, path, body),
			},
		}},
	}
}

// textReply builds a mock LLM response with a plain-text final answer.
func textReply(content string) *mcp.LLMResponse {
	return &mcp.LLMResponse{Content: content}
}

func mockGetLLM(llm *mockLLM) func() mcp.AIClient {
	return func() mcp.AIClient { return llm }
}

const testPrompt = "You are a test assistant."

// mockAPIServer creates a test HTTP server with configurable route handlers.
func mockAPIServer(handlers map[string]string) (*httptest.Server, int) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if body, ok := handlers[key]; ok {
			w.Write([]byte(body)) //nolint:errcheck
			return
		}
		// Also try path-only match (for GET)
		if body, ok := handlers[r.URL.Path]; ok {
			w.Write([]byte(body)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`)) //nolint:errcheck
	}))
	var port int
	fmt.Sscanf(srv.Listener.Addr().String(), "127.0.0.1:%d", &port)
	return srv, port
}

// ── Basic agent behaviour ──────────────────────────────────────────────────

// TestAgentDirectReply: LLM replies with text (no tool calls) — one LLM call.
func TestAgentDirectReply(t *testing.T) {
	llm := &mockLLM{responses: []*mcp.LLMResponse{textReply("Hello! How can I help you?")}}
	a := New(8080, "tok", "test-user", mockGetLLM(llm), testPrompt)

	reply := a.Run("hello", nil)

	if reply != "Hello! How can I help you?" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if llm.calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", llm.calls)
	}
}

// TestAgentAPICall: LLM makes one tool call, gets result, gives final reply — two LLM calls.
func TestAgentAPICall(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/my-traders": `[{"trader_id":"t1","trader_name":"BTC Trader","is_running":false}]`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("c1", "GET", "/api/my-traders", "{}"),
		textReply("You have one trader: BTC Trader."),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)

	reply := a.Run("list my traders", nil)

	if reply != "You have one trader: BTC Trader." {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if llm.calls != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", llm.calls)
	}
}

// TestAgentMultiStep: LLM chains two tool calls before final reply — three LLM calls.
func TestAgentMultiStep(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account":   `{"total_equity":1000}`,
		"/api/positions": `[]`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("c1", "GET", "/api/account", "{}"),
		toolCall("c2", "GET", "/api/positions", "{}"),
		textReply("Account looks healthy and no open positions."),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)

	reply := a.Run("show me account status", nil)

	if llm.calls != 3 {
		t.Fatalf("expected 3 LLM calls (2 tool + 1 final), got %d", llm.calls)
	}
	if reply != "Account looks healthy and no open positions." {
		t.Fatalf("unexpected final reply: %q", reply)
	}
}

// TestAgentAPIResultInContext: tool result must appear as a tool message in the next LLM call.
func TestAgentAPIResultInContext(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account": `{"balance":1234.56}`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("c1", "GET", "/api/account", "{}"),
		textReply("Balance is 1234.56 USDT."),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	a.Run("show balance", nil)

	// The last request must contain a tool-result message with the balance data.
	found := false
	for _, msg := range llm.lastMsgs {
		if msg.Role == "tool" && strings.Contains(msg.Content, "balance") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tool result message not found in subsequent LLM context; messages: %+v", llm.lastMsgs)
	}
}

// ── Narration-free architecture tests ─────────────────────────────────────

// TestNarrationStructurallyImpossible: when ToolCalls are present in the response,
// any Content field is ignored and never surfaced to the user.
// In real LLM APIs, Content is always empty alongside ToolCalls, but we verify
// our agent handles a malformed response defensively.
func TestNarrationStructurallyImpossible(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/strategies": `[{"id":"s1","name":"BTC Trend"}]`,
	})
	defer srv.Close()

	// Simulate a (malformed) response that has both Content and ToolCalls.
	malformed := &mcp.LLMResponse{
		Content: "现在我将为您查询策略。", // narration — must NOT reach user
		ToolCalls: []mcp.ToolCall{{
			ID:   "c1",
			Type: "function",
			Function: mcp.ToolCallFunction{
				Name:      "api_request",
				Arguments: `{"method":"GET","path":"/api/strategies","body":{}}`,
			},
		}},
	}

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		malformed,
		textReply("你有1个策略：BTC Trend。"),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("查询我的策略", nil)

	if strings.Contains(reply, "现在我将") {
		t.Fatalf("narration leaked into final reply: %q", reply)
	}
	if reply != "你有1个策略：BTC Trend。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
}

// TestOnChunkCalledWithFinalReply: onChunk receives the complete final reply.
func TestOnChunkCalledWithFinalReply(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account": `{"equity":500}`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("c1", "GET", "/api/account", "{}"),
		textReply("Equity: 500 USDT."),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)

	var chunks []string
	reply := a.Run("show equity", func(chunk string) {
		chunks = append(chunks, chunk)
	})

	if reply != "Equity: 500 USDT." {
		t.Fatalf("unexpected reply: %q", reply)
	}
	// Should have received ⏳ for the tool call, then the final reply.
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks (⏳ + final), got: %v", chunks)
	}
	lastChunk := chunks[len(chunks)-1]
	if lastChunk != "Equity: 500 USDT." {
		t.Fatalf("last chunk should be final reply, got: %q", lastChunk)
	}
}

// ── Workflow tests ─────────────────────────────────────────────────────────

// TestCreateStrategyWorkflow: simulates creating a BTC trend strategy.
// Verifies: POST strategy → GET verify → final reply shows strategy info.
func TestCreateStrategyWorkflow(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"POST /api/strategies":   `{"id":"s1","name":"BTC趋势"}`,
		"GET /api/strategies/s1": `{"id":"s1","name":"BTC趋势","config":{"coin_source":{"source_type":"static","static_coins":["BTC/USDT"]},"leverage":5}}`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("c1", "POST", "/api/strategies", `{"name":"BTC趋势","config":{}}`),
		toolCall("c2", "GET", "/api/strategies/s1", "{}"),
		textReply("策略已创建：BTC趋势，币种 BTC/USDT，杠杆 5x。"),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("帮我配置个btc趋势交易的策略", nil)

	if llm.calls != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", llm.calls)
	}
	if reply == "" {
		t.Fatalf("empty final reply")
	}
}

// TestFullSetupWorkflow: create strategy → verify → create trader → start trader.
// This is the "帮我配置策略并跑起来" workflow.
func TestFullSetupWorkflow(t *testing.T) {
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		calls[key]++
		switch key {
		case "POST /api/strategies":
			w.Write([]byte(`{"id":"s1","name":"BTC趋势"}`)) //nolint:errcheck
		case "GET /api/strategies/s1":
			w.Write([]byte(`{"id":"s1","name":"BTC趋势","config":{}}`)) //nolint:errcheck
		case "POST /api/traders":
			w.Write([]byte(`{"id":"tr1","name":"BTC趋势交易员"}`)) //nolint:errcheck
		case "POST /api/traders/tr1/start":
			w.Write([]byte(`{"ok":true}`)) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	var port int
	fmt.Sscanf(srv.Listener.Addr().String(), "127.0.0.1:%d", &port)

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("c1", "POST", "/api/strategies", `{"name":"BTC趋势"}`),
		toolCall("c2", "GET", "/api/strategies/s1", "{}"),
		toolCall("c3", "POST", "/api/traders", `{"name":"BTC趋势交易员","strategy_id":"s1"}`),
		toolCall("c4", "POST", "/api/traders/tr1/start", "{}"),
		textReply("策略和交易员已创建并启动！BTC趋势交易员正在运行。"),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("帮我配置个btc趋势交易的策略交易 跑起来", nil)

	if llm.calls != 5 {
		t.Fatalf("expected 5 LLM calls, got %d", llm.calls)
	}
	if calls["POST /api/strategies"] != 1 {
		t.Errorf("expected 1 POST /api/strategies, got %d", calls["POST /api/strategies"])
	}
	if calls["POST /api/traders"] != 1 {
		t.Errorf("expected 1 POST /api/traders, got %d", calls["POST /api/traders"])
	}
	if calls["POST /api/traders/tr1/start"] != 1 {
		t.Errorf("expected 1 POST /api/traders/tr1/start, got %d", calls["POST /api/traders/tr1/start"])
	}
	if reply == "" {
		t.Fatalf("empty final reply")
	}
}

// TestStartExistingTrader: when trader already exists, just start it.
func TestStartExistingTrader(t *testing.T) {
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		calls[key]++
		switch key {
		case "GET /api/my-traders":
			w.Write([]byte(`[{"trader_id":"tr1","trader_name":"BTC Trader","is_running":false}]`)) //nolint:errcheck
		case "POST /api/traders/tr1/start":
			w.Write([]byte(`{"ok":true}`)) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	var port int
	fmt.Sscanf(srv.Listener.Addr().String(), "127.0.0.1:%d", &port)

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("c1", "GET", "/api/my-traders", "{}"),
		toolCall("c2", "POST", "/api/traders/tr1/start", "{}"),
		textReply("交易员 BTC Trader 已启动。"),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("启动交易员", nil)

	if calls["POST /api/traders/tr1/start"] != 1 {
		t.Errorf("expected trader to be started, got %d start calls", calls["POST /api/traders/tr1/start"])
	}
	if reply != "交易员 BTC Trader 已启动。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
}

// ── Safety limit ───────────────────────────────────────────────────────────

// TestMaxIterations: agent terminates after maxIterations and returns fallback message.
func TestMaxIterations(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account": `{"ok":true}`,
	})
	defer srv.Close()

	// Always returns another tool call — should hit max iterations.
	responses := make([]*mcp.LLMResponse, maxIterations+2)
	for i := range responses {
		responses[i] = toolCall(fmt.Sprintf("c%d", i), "GET", "/api/account", "{}")
	}

	llm := &mockLLM{responses: responses}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("loop forever", nil)

	if reply == "" {
		t.Fatalf("expected a fallback reply, got empty string")
	}
	// Agent should have made exactly maxIterations tool-call LLM calls.
	if llm.calls != maxIterations {
		t.Fatalf("expected %d LLM calls (max iterations), got %d", maxIterations, llm.calls)
	}
}

// TestToolCallIDPropagated: tool result messages carry the correct ToolCallID.
func TestToolCallIDPropagated(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account": `{"balance":999}`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []*mcp.LLMResponse{
		toolCall("call-xyz-123", "GET", "/api/account", "{}"),
		textReply("Balance is 999."),
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	a.Run("check balance", nil)

	// Find the tool result message and verify ToolCallID matches.
	found := false
	for _, msg := range llm.lastMsgs {
		if msg.Role == "tool" && msg.ToolCallID == "call-xyz-123" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tool result with ToolCallID='call-xyz-123' not found in messages: %+v", llm.lastMsgs)
	}
}
