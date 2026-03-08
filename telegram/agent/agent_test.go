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

type mockLLM struct {
	responses []string
	calls     int
	lastMsgs  []mcp.Message
}

func (m *mockLLM) SetAPIKey(_, _, _ string)                    {}
func (m *mockLLM) SetTimeout(_ time.Duration)                  {}
func (m *mockLLM) CallWithMessages(_, _ string) (string, error) { return m.next() }
func (m *mockLLM) CallWithRequest(req *mcp.Request) (string, error) {
	m.lastMsgs = req.Messages
	return m.next()
}
func (m *mockLLM) CallWithRequestStream(req *mcp.Request, onChunk func(string)) (string, error) {
	m.lastMsgs = req.Messages
	r, err := m.next()
	if onChunk != nil {
		onChunk(r)
	}
	return r, err
}
func (m *mockLLM) next() (string, error) {
	if m.calls < len(m.responses) {
		r := m.responses[m.calls]
		m.calls++
		return r, nil
	}
	return "OK", nil
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

// TestAgentDirectReply: LLM replies without api_call — one call, direct reply.
func TestAgentDirectReply(t *testing.T) {
	llm := &mockLLM{responses: []string{"Hello! How can I help you?"}}
	a := New(8080, "tok", "test-user", mockGetLLM(llm), testPrompt)

	reply := a.Run("hello", nil)

	if reply != "Hello! How can I help you?" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if llm.calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", llm.calls)
	}
}

// TestAgentAPICall: LLM calls API, gets result, gives final reply — two LLM calls.
func TestAgentAPICall(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/my-traders": `[{"trader_id":"t1","trader_name":"BTC Trader","is_running":false}]`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []string{
		`<api_call>{"method":"GET","path":"/api/my-traders","body":{}}</api_call>`,
		"You have one trader: BTC Trader.",
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

// TestAgentMultiStep: LLM chains two API calls before final reply — three LLM calls.
func TestAgentMultiStep(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account":   `{"total_equity":1000}`,
		"/api/positions": `[]`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []string{
		`<api_call>{"method":"GET","path":"/api/account","body":{}}</api_call>`,
		`<api_call>{"method":"GET","path":"/api/positions","body":{}}</api_call>`,
		"Account looks healthy and no open positions.",
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)

	reply := a.Run("show me account status", nil)

	if llm.calls != 3 {
		t.Fatalf("expected 3 LLM calls (2 api + 1 final), got %d", llm.calls)
	}
	if reply != "Account looks healthy and no open positions." {
		t.Fatalf("unexpected final reply: %q", reply)
	}
}

// TestAgentAPIResultInContext: API result must appear in next LLM message.
func TestAgentAPIResultInContext(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account": `{"balance":1234.56}`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []string{
		`<api_call>{"method":"GET","path":"/api/account","body":{}}</api_call>`,
		"Balance is 1234.56 USDT.",
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	a.Run("show balance", nil)

	found := false
	for _, msg := range llm.lastMsgs {
		if strings.Contains(msg.Content, "API result") || strings.Contains(msg.Content, "balance") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("API result not found in subsequent LLM context")
	}
}

// ── NO NARRATION tests ─────────────────────────────────────────────────────

// TestNoNarrationBeforeAPICall: any text before <api_call> must NOT reach the user.
// The agent strips text-before-tag and only forwards it as assistant context.
func TestNoNarrationBeforeAPICall(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/strategies": `[{"id":"s1","name":"BTC Trend"}]`,
	})
	defer srv.Close()

	narrations := []string{
		"现在我将为您创建策略。\n",
		"好的，我来帮你查询。",
		"Let me check this for you. ",
		"正在处理...",
		"I will call the API now. ",
	}

	for _, narration := range narrations {
		llm := &mockLLM{responses: []string{
			// LLM outputs narration before the api_call tag (bad behaviour we must handle)
			narration + `<api_call>{"method":"GET","path":"/api/strategies","body":{}}</api_call>`,
			"你有1个策略：BTC Trend。",
		}}
		a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
		reply := a.Run("查询我的策略", nil)

		// Final reply must not contain narration fragments
		if strings.Contains(reply, "现在我将") || strings.Contains(reply, "Let me") ||
			strings.Contains(reply, "正在处理") || strings.Contains(reply, "好的，我来") ||
			strings.Contains(reply, "I will call") {
			t.Fatalf("narration leaked into reply for input %q: got %q", narration, reply)
		}
		// api_call tag must not appear in reply
		if strings.Contains(reply, "<api_call>") {
			t.Fatalf("api_call tag leaked into reply: %q", reply)
		}
	}
}

// TestAPICallTagNotLeakedToUser: <api_call> tag must never appear in returned reply.
func TestAPICallTagNotLeakedToUser(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account": `{"total_equity":500}`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []string{
		`<api_call>{"method":"GET","path":"/api/account","body":{}}</api_call>`,
		`账户余额 500 USDT。<api_call>{"method":"GET","path":"/api/account","body":{}}</api_call>`,
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("show balance", nil)

	if strings.Contains(reply, "<api_call>") {
		t.Fatalf("api_call tag leaked to user: %q", reply)
	}
}

// ── Workflow tests ─────────────────────────────────────────────────────────

// TestCreateStrategyWorkflow: simulates creating a BTC trend strategy.
// Verifies: POST strategy → GET verify → final reply shows strategy info.
func TestCreateStrategyWorkflow(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"POST /api/strategies":    `{"id":"s1","name":"BTC趋势"}`,
		"GET /api/strategies/s1":  `{"id":"s1","name":"BTC趋势","config":{"coin_source":{"source_type":"static","static_coins":["BTC/USDT"]},"leverage":5}}`,
	})
	defer srv.Close()

	llm := &mockLLM{responses: []string{
		// Step 1: create strategy
		`<api_call>{"method":"POST","path":"/api/strategies","body":{"name":"BTC趋势","config":{}}}</api_call>`,
		// Step 2: verify strategy
		`<api_call>{"method":"GET","path":"/api/strategies/s1","body":{}}</api_call>`,
		// Step 3: final reply
		"策略已创建：BTC趋势，币种 BTC/USDT，杠杆 5x。",
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("帮我配置个btc趋势交易的策略", nil)

	if llm.calls != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", llm.calls)
	}
	if reply == "" || strings.Contains(reply, "<api_call>") {
		t.Fatalf("bad final reply: %q", reply)
	}
}

// TestFullSetupWorkflow: create strategy → create trader → start trader.
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

	llm := &mockLLM{responses: []string{
		// 1. create strategy
		`<api_call>{"method":"POST","path":"/api/strategies","body":{"name":"BTC趋势"}}</api_call>`,
		// 2. verify strategy
		`<api_call>{"method":"GET","path":"/api/strategies/s1","body":{}}</api_call>`,
		// 3. create trader
		`<api_call>{"method":"POST","path":"/api/traders","body":{"name":"BTC趋势交易员","strategy_id":"s1"}}</api_call>`,
		// 4. start trader
		`<api_call>{"method":"POST","path":"/api/traders/tr1/start","body":{}}</api_call>`,
		// 5. final reply
		"策略和交易员已创建并启动！BTC趋势交易员正在运行。",
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("帮我配置个btc趋势交易的策略交易 跑起来", nil)

	if llm.calls != 5 {
		t.Fatalf("expected 5 LLM calls, got %d", llm.calls)
	}
	// Verify each API was called
	if calls["POST /api/strategies"] != 1 {
		t.Errorf("expected 1 POST /api/strategies, got %d", calls["POST /api/strategies"])
	}
	if calls["POST /api/traders"] != 1 {
		t.Errorf("expected 1 POST /api/traders, got %d", calls["POST /api/traders"])
	}
	if calls["POST /api/traders/tr1/start"] != 1 {
		t.Errorf("expected 1 POST /api/traders/tr1/start, got %d", calls["POST /api/traders/tr1/start"])
	}
	if strings.Contains(reply, "<api_call>") {
		t.Fatalf("api_call tag in final reply: %q", reply)
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

	llm := &mockLLM{responses: []string{
		`<api_call>{"method":"GET","path":"/api/my-traders","body":{}}</api_call>`,
		`<api_call>{"method":"POST","path":"/api/traders/tr1/start","body":{}}</api_call>`,
		"交易员 BTC Trader 已启动。",
	}}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("启动交易员", nil)

	if calls["POST /api/traders/tr1/start"] != 1 {
		t.Errorf("expected trader to be started, got %d start calls", calls["POST /api/traders/tr1/start"])
	}
	if strings.Contains(reply, "<api_call>") {
		t.Fatalf("api_call tag in reply: %q", reply)
	}
}

// ── Parser tests ───────────────────────────────────────────────────────────

// TestParseAPICall: unit tests for the XML tag parser.
func TestParseAPICall(t *testing.T) {
	t.Run("valid call no text before", func(t *testing.T) {
		resp := `<api_call>{"method":"POST","path":"/api/traders/t1/stop","body":{}}</api_call>`
		req, text := parseAPICall(resp)
		if req == nil {
			t.Fatal("expected api_call, got nil")
		}
		if req.Method != "POST" || req.Path != "/api/traders/t1/stop" {
			t.Fatalf("unexpected req: %+v", req)
		}
		if text != "" {
			t.Fatalf("expected empty text before tag, got: %q", text)
		}
	})

	t.Run("text before tag is captured", func(t *testing.T) {
		resp := `Stopping trader.<api_call>{"method":"POST","path":"/api/traders/t1/stop","body":{}}</api_call>`
		req, text := parseAPICall(resp)
		if req == nil {
			t.Fatal("expected api_call, got nil")
		}
		if text != "Stopping trader." {
			t.Fatalf("unexpected text before tag: %q", text)
		}
	})

	t.Run("no call tag", func(t *testing.T) {
		req, text := parseAPICall("Just a reply.")
		if req != nil {
			t.Fatal("expected nil api_call")
		}
		if text != "Just a reply." {
			t.Fatalf("expected original text, got %q", text)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		req, _ := parseAPICall(`<api_call>NOT JSON</api_call>`)
		if req != nil {
			t.Fatal("expected nil for malformed JSON")
		}
	})
}

// TestStripAPICallTag: defensive cleanup of stray tags in final reply.
func TestStripAPICallTag(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`正常回复`, `正常回复`},
		{`回复<api_call>{"method":"GET","path":"/x"}</api_call>`, `回复`},
		{`<api_call>{"method":"GET","path":"/x"}</api_call>`, ``},
	}
	for _, c := range cases {
		got := stripAPICallTag(c.input)
		if strings.TrimSpace(got) != c.want {
			t.Errorf("stripAPICallTag(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestMaxIterations: agent stops after maxIterations and returns a summary.
func TestMaxIterations(t *testing.T) {
	srv, port := mockAPIServer(map[string]string{
		"/api/account": `{"ok":true}`,
	})
	defer srv.Close()

	// Always returns another api_call — should hit max iterations
	responses := make([]string, maxIterations+2)
	for i := range responses {
		responses[i] = `<api_call>{"method":"GET","path":"/api/account","body":{}}</api_call>`
	}
	responses[maxIterations] = "Final summary after max iterations."

	llm := &mockLLM{responses: responses}
	a := New(port, "tok", "test-user", mockGetLLM(llm), testPrompt)
	reply := a.Run("loop forever", nil)

	if strings.Contains(reply, "<api_call>") {
		t.Fatalf("api_call tag in reply after max iterations: %q", reply)
	}
	_ = reply // just confirm it terminates
}
