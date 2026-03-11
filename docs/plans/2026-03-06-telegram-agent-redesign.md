# Telegram Bot Agent Redesign (OpenClaw-Inspired)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Replace the NLU intent-classification architecture with a true AI Agent that handles any user request — including scenarios never explicitly programmed. All code, comments, prompts, and bot responses in English.

**Architecture:** One generic tool (`api_call`) + dynamically generated API docs + unbounded LLM loop. The LLM reads auto-generated API docs and decides which endpoints to call. New features added to the web UI automatically become available via bot — zero code changes required.

**Tech Stack:** Go, `mcp.CallWithRequest` + `RequestBuilder`, `tgbotapi`, `auth.GenerateJWT`

---

## Core Design

OpenClaw gives LLM a `bash` tool — one generic primitive, unlimited capability.
We give LLM an `api_call(method, path, body)` tool — one generic primitive for 74+ REST endpoints.

**Auto-discovery:** Routes are registered via `s.route(group, method, path, description, handler)`.
`api.GetAPIDocs()` returns live documentation at startup — add a route and it's automatically in the bot's context.

```
User: "show positions and stop the trader if loss > 5%"

Iteration 1: api_call GET /api/positions?trader_id=...
Iteration 2: api_call GET /api/account?trader_id=...
Iteration 3: [sees -8% loss] api_call POST /api/traders/xxx/stop
Reply: "Detected -8% loss. Trader stopped."
```

No special code for this scenario. LLM figured it out from the API docs.

---

## What changes

| File | Action |
|------|--------|
| `api/route_registry.go` | **CREATE** — route registration + doc generation |
| `api/server.go` | Migrate all routes from `group.METHOD(path, handler)` to `s.route(group, method, path, desc, handler)` |
| `telegram/intent/parser.go` | **DELETE** |
| `telegram/handler/handler.go` | **DELETE** |
| `telegram/handler/handler_test.go` | **DELETE** |
| `telegram/session/session.go` | Simplify (remove Intent, Params) |
| `telegram/bot.go` | Use `agent.Manager`, pass `api.GetAPIDocs()` |
| `telegram/agent/prompt.go` | **CREATE** — system prompt template (API docs injected at runtime) |
| `telegram/agent/apicall.go` | **CREATE** — the single generic tool |
| `telegram/agent/agent.go` | **CREATE** — agent loop |
| `telegram/agent/manager.go` | **CREATE** — per-chat serialization |
| `telegram/agent/agent_test.go` | **CREATE** — tests |

`telegram/service/nofx.go` and `telegram/session/memory.go` are **unchanged**.

---

## Task 1: Create `api/route_registry.go`

**Files:**
- Create: `api/route_registry.go`

This is the single source of truth for API documentation. Routes registered here are automatically available to the bot.

```go
package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// RouteDoc holds documentation for a single API route.
type RouteDoc struct {
	Method      string
	Path        string
	Description string
}

// routeRegistry stores all documented routes. Populated via s.route() calls in setupRoutes.
var routeRegistry []RouteDoc

// route registers an HTTP route on the given group and records its documentation.
// This is the single registration point — add a route here and it is automatically
// included in GetAPIDocs(), making it available to the Telegram bot agent.
func (s *Server) route(g *gin.RouterGroup, method, path, description string, h gin.HandlerFunc) {
	// Derive the full path: group prefix + local path
	fullPath := strings.TrimSuffix(g.BasePath(), "/") + "/" + strings.TrimPrefix(path, "/")
	routeRegistry = append(routeRegistry, RouteDoc{
		Method:      method,
		Path:        fullPath,
		Description: description,
	})
	switch method {
	case "GET":
		g.GET(path, h)
	case "POST":
		g.POST(path, h)
	case "PUT":
		g.PUT(path, h)
	case "DELETE":
		g.DELETE(path, h)
	}
}

// GetAPIDocs returns formatted API documentation for injection into the LLM system prompt.
// Called once at bot startup — reflects the live set of registered routes.
func GetAPIDocs() string {
	var sb strings.Builder
	for _, r := range routeRegistry {
		sb.WriteString(fmt.Sprintf("%-8s %-50s %s\n", r.Method, r.Path, r.Description))
	}
	return sb.String()
}
```

**Step 1: Create the file**

**Step 2: Build**

```bash
cd /Users/yida/gopro/open-nofx && go build ./api/...
```

Expected: clean build.

**Step 3: Commit**

```bash
git add api/route_registry.go
git commit -m "feat(api): add route registry for auto-generated API documentation"
```

---

## Task 2: Migrate routes in `api/server.go`

**Files:**
- Modify: `api/server.go` (the `setupRoutes` / route registration block, lines ~109–230)

Replace every direct `group.METHOD(path, handler)` call with `s.route(group, method, path, description, handler)`.

**Step 1: Read the current route registration block**

```bash
sed -n '109,230p' api/server.go
```

**Step 2: Replace all route registrations**

The full replacement (covers all routes found in lines 117–223):

```go
// Public routes
s.route(api, "GET",  "/supported-models",          "List supported AI model providers",           s.handleGetSupportedModels)
s.route(api, "GET",  "/supported-exchanges",        "List supported exchange types",                s.handleGetSupportedExchanges)
s.route(api, "GET",  "/config",                     "Get system configuration",                     s.handleGetSystemConfig)
s.route(api, "GET",  "/traders",                    "Public trader list",                           s.handlePublicTraderList)
s.route(api, "GET",  "/competition",                "Public competition data",                      s.handlePublicCompetition)
s.route(api, "GET",  "/top-traders",                "Top traders leaderboard",                      s.handleTopTraders)
s.route(api, "GET",  "/equity-history",             "Equity history for a trader",                  s.handleEquityHistory)
s.route(api, "POST", "/equity-history-batch",       "Batch equity history for multiple traders",    s.handleEquityHistoryBatch)
s.route(api, "GET",  "/traders/:id/public-config",  "Public trader configuration",                  s.handleGetPublicTraderConfig)
s.route(api, "GET",  "/klines",                     "Candlestick data (?symbol=&interval=&limit=)", s.handleKlines)
s.route(api, "GET",  "/symbols",                    "Available trading symbols",                    s.handleSymbols)
s.route(api, "GET",  "/strategies/public",          "Public strategy market",                       s.handlePublicStrategies)
s.route(api, "POST", "/register",                   "Register new user",                            s.handleRegister)
s.route(api, "POST", "/login",                      "User login, returns JWT token",                s.handleLogin)

// Protected routes (JWT required)
s.route(protected, "POST",   "/logout",                       "Logout (blacklist token)",                        s.handleLogout)
s.route(protected, "GET",    "/server-ip",                    "Get server public IP (for exchange whitelist)",   s.handleGetServerIP)

// Trader management
s.route(protected, "GET",    "/my-traders",                   "List user's traders",                             s.handleTraderList)
s.route(protected, "GET",    "/traders/:id/config",           "Get full trader configuration",                   s.handleGetTraderConfig)
s.route(protected, "POST",   "/traders",                      "Create trader (body: name, strategy_id, exchange_id, model_id)", s.handleCreateTrader)
s.route(protected, "PUT",    "/traders/:id",                  "Update trader configuration",                     s.handleUpdateTrader)
s.route(protected, "DELETE", "/traders/:id",                  "Delete trader",                                   s.handleDeleteTrader)
s.route(protected, "POST",   "/traders/:id/start",            "Start trader",                                    s.handleStartTrader)
s.route(protected, "POST",   "/traders/:id/stop",             "Stop trader",                                     s.handleStopTrader)
s.route(protected, "PUT",    "/traders/:id/prompt",           "Update trader prompt (body: prompt)",             s.handleUpdateTraderPrompt)
s.route(protected, "POST",   "/traders/:id/sync-balance",     "Sync account balance from exchange",              s.handleSyncBalance)
s.route(protected, "POST",   "/traders/:id/close-position",   "Close position (body: symbol)",                   s.handleClosePosition)
s.route(protected, "PUT",    "/traders/:id/competition",      "Toggle competition visibility",                   s.handleToggleCompetition)
s.route(protected, "GET",    "/traders/:id/grid-risk",        "Get grid risk info",                              s.handleGetGridRiskInfo)

// AI model configuration
s.route(protected, "GET", "/models", "List AI model configurations",   s.handleGetModelConfigs)
s.route(protected, "PUT", "/models", "Update AI model configurations", s.handleUpdateModelConfigs)

// Exchange configuration
s.route(protected, "GET",    "/exchanges",     "List exchange configurations",                                      s.handleGetExchangeConfigs)
s.route(protected, "POST",   "/exchanges",     "Create exchange (body: exchange_type, api_key, secret_key, account_name)", s.handleCreateExchange)
s.route(protected, "PUT",    "/exchanges",     "Update exchange configurations",                                    s.handleUpdateExchangeConfigs)
s.route(protected, "DELETE", "/exchanges/:id", "Delete exchange",                                                   s.handleDeleteExchange)

// Telegram configuration
s.route(protected, "GET",    "/telegram",         "Get Telegram bot configuration",       s.handleGetTelegramConfig)
s.route(protected, "POST",   "/telegram",         "Update Telegram bot token/model",      s.handleUpdateTelegramConfig)
s.route(protected, "POST",   "/telegram/model",   "Update Telegram bot AI model only",   s.handleUpdateTelegramModel)
s.route(protected, "DELETE", "/telegram/binding", "Unbind Telegram account",             s.handleUnbindTelegram)

// Strategy management
s.route(protected, "GET",    "/strategies",                  "List user's strategies",                            s.handleGetStrategies)
s.route(protected, "GET",    "/strategies/active",           "Get active strategy",                               s.handleGetActiveStrategy)
s.route(protected, "GET",    "/strategies/default-config",   "Get default strategy config template",             s.handleGetDefaultStrategyConfig)
s.route(protected, "POST",   "/strategies/preview-prompt",   "Preview generated strategy prompt",                s.handlePreviewPrompt)
s.route(protected, "POST",   "/strategies/test-run",         "Test-run strategy AI analysis",                    s.handleStrategyTestRun)
s.route(protected, "GET",    "/strategies/:id",              "Get strategy by ID",                               s.handleGetStrategy)
s.route(protected, "POST",   "/strategies",                  "Create strategy (body: name, config)",              s.handleCreateStrategy)
s.route(protected, "PUT",    "/strategies/:id",              "Update strategy",                                   s.handleUpdateStrategy)
s.route(protected, "DELETE", "/strategies/:id",              "Delete strategy",                                   s.handleDeleteStrategy)
s.route(protected, "POST",   "/strategies/:id/activate",     "Activate strategy",                                s.handleActivateStrategy)
s.route(protected, "POST",   "/strategies/:id/duplicate",    "Duplicate strategy",                               s.handleDuplicateStrategy)

// Debate arena
s.route(protected, "GET",    "/debates",                   "List debates",                    s.debateHandler.HandleListDebates)
s.route(protected, "GET",    "/debates/personalities",     "Available AI personalities",      s.debateHandler.HandleGetPersonalities)
s.route(protected, "GET",    "/debates/:id",               "Get debate details",              s.debateHandler.HandleGetDebate)
s.route(protected, "POST",   "/debates",                   "Create debate",                   s.debateHandler.HandleCreateDebate)
s.route(protected, "POST",   "/debates/:id/start",         "Start debate",                    s.debateHandler.HandleStartDebate)
s.route(protected, "POST",   "/debates/:id/cancel",        "Cancel debate",                   s.debateHandler.HandleCancelDebate)
s.route(protected, "POST",   "/debates/:id/execute",       "Execute debate consensus decision", s.debateHandler.HandleExecuteDebate)
s.route(protected, "DELETE", "/debates/:id",               "Delete debate",                   s.debateHandler.HandleDeleteDebate)
s.route(protected, "GET",    "/debates/:id/messages",      "Get debate messages",             s.debateHandler.HandleGetMessages)
s.route(protected, "GET",    "/debates/:id/votes",         "Get debate votes",                s.debateHandler.HandleGetVotes)
s.route(protected, "GET",    "/debates/:id/stream",        "SSE stream for live debate",      s.debateHandler.HandleDebateStream)

// Account and trading data (use ?trader_id=xxx query param)
s.route(protected, "GET", "/status",             "Trader running status (?trader_id=)",      s.handleStatus)
s.route(protected, "GET", "/account",            "Account balance and equity (?trader_id=)", s.handleAccount)
s.route(protected, "GET", "/positions",          "Current open positions (?trader_id=)",     s.handlePositions)
s.route(protected, "GET", "/positions/history",  "Position history (?trader_id=)",           s.handlePositionHistory)
s.route(protected, "GET", "/trades",             "Trade records (?trader_id=)",              s.handleTrades)
s.route(protected, "GET", "/orders",             "All orders (?trader_id=)",                 s.handleOrders)
s.route(protected, "GET", "/orders/:id/fills",   "Order fill details",                       s.handleOrderFills)
s.route(protected, "GET", "/open-orders",        "Open orders from exchange (?trader_id=)",  s.handleOpenOrders)
s.route(protected, "GET", "/decisions",          "AI trading decisions (?trader_id=)",       s.handleDecisions)
s.route(protected, "GET", "/decisions/latest",   "Latest AI decisions (?trader_id=)",        s.handleLatestDecisions)
s.route(protected, "GET", "/statistics",         "Trading statistics (?trader_id=)",         s.handleStatistics)
```

Note: keep the existing special-case handlers that don't use `s.route` unchanged:
- `api.Any("/health", ...)` — health check, no need to document
- `api.GET("/crypto/...")` — crypto/encryption routes, bot doesn't need these
- `backtest.*` routes (registered separately) — add descriptions to the backtest group similarly

**Step 3: Build**

```bash
go build ./api/...
```

Expected: clean build. Fix any compilation errors (method signature mismatches).

**Step 4: Verify docs are generated**

```bash
go test ./api/... -run TestGetAPIDocs -v
```

(Write a quick inline test or just print in main to verify)

**Step 5: Commit**

```bash
git add api/route_registry.go api/server.go
git commit -m "feat(api): migrate routes to self-documenting s.route() registration"
```

---

## Task 3: Create `telegram/agent/prompt.go`

**Files:**
- Create: `telegram/agent/prompt.go`

The system prompt template. API docs are injected at runtime via `BuildAgentPrompt(apiDocs)`.

```go
package agent

import "fmt"

// BuildAgentPrompt constructs the full system prompt with live API documentation injected.
// apiDocs is the output of api.GetAPIDocs() — reflects all currently registered routes.
func BuildAgentPrompt(apiDocs string) string {
	return fmt.Sprintf(`You are the NOFX quantitative trading system AI assistant.
You can have natural conversations with the user and call the API to operate the system.

## Tool

You have one tool: api_call

Call format (append at end of reply):
<api_call>{"method":"GET","path":"/api/xxx","body":{}}</api_call>

- method: "GET" | "POST" | "PUT" | "DELETE"
- path: API path from the documentation below
- body: request body as JSON object (use {} for GET requests)
- query parameters go in the path, e.g. /api/positions?trader_id=xxx

## NOFX API Documentation

All requests are pre-authenticated. Focus on paths and parameters.

%s

## Rules
1. When you need to perform a system operation, append <api_call>...</api_call> at the end of your reply
2. Only call one API per response; after receiving the result, decide whether to call another or give a final reply
3. For conversations, questions, or analysis that don't require system operations, reply directly without calling the API
4. If required parameters are unclear, ask the user — do not guess critical values like trader_id
5. Always reply in English`, apiDocs)
}
```

**Step 1: Create the file**

**Step 2: Build**

```bash
go build ./telegram/agent/...
```

**Step 3: Commit**

```bash
git add telegram/agent/prompt.go
git commit -m "feat(telegram/agent): add dynamic system prompt builder"
```

---

## Task 4: Create `telegram/agent/apicall.go`

**Files:**
- Create: `telegram/agent/apicall.go`

```go
package agent

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

// apiCallTool executes HTTP requests against the NOFX API server.
// This is the only tool available to the agent.
type apiCallTool struct {
	baseURL string
	token   string
	client  *http.Client
}

// apiRequest is the parsed structure from the LLM's <api_call> tag.
type apiRequest struct {
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Body   map[string]any `json:"body"`
}

func newAPICallTool(port int, token string) *apiCallTool {
	return &apiCallTool{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// execute calls the API and returns the response as a string for LLM consumption.
func (t *apiCallTool) execute(req *apiRequest) string {
	if req.Method == "" || req.Path == "" {
		return "error: method and path are required"
	}
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}

	var bodyReader io.Reader
	if req.Method != "GET" && len(req.Body) > 0 {
		b, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Sprintf("error marshaling body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	httpReq, err := http.NewRequest(req.Method, t.baseURL+req.Path, bodyReader)
	if err != nil {
		return fmt.Sprintf("error creating request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+t.token)

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Sprintf("API call failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("error reading response: %v", err)
	}

	logger.Infof("Agent api_call: %s %s -> %d", req.Method, req.Path, resp.StatusCode)

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Pretty-print JSON for better LLM readability
	var v any
	if json.Unmarshal(body, &v) == nil {
		if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(pretty)
		}
	}
	return string(body)
}

// parseAPICall extracts <api_call>...</api_call> from LLM response.
// Returns (nil, original) if not found or malformed JSON.
func parseAPICall(resp string) (*apiRequest, string) {
	const openTag = "<api_call>"
	const closeTag = "</api_call>"

	start := strings.Index(resp, openTag)
	end := strings.Index(resp, closeTag)
	if start < 0 || end < 0 || end <= start {
		return nil, resp
	}

	jsonStr := strings.TrimSpace(resp[start+len(openTag) : end])
	var req apiRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		logger.Warnf("Agent: failed to parse api_call JSON %q: %v", jsonStr, err)
		return nil, resp
	}

	return &req, strings.TrimSpace(resp[:start])
}
```

**Step 1: Create the file**

**Step 2: Commit**

```bash
git add telegram/agent/apicall.go
git commit -m "feat(telegram/agent): add generic api_call tool"
```

---

## Task 5: Create `telegram/agent/agent.go`

**Files:**
- Create: `telegram/agent/agent.go`

```go
package agent

import (
	"fmt"
	"nofx/auth"
	"nofx/logger"
	"nofx/mcp"
	"nofx/telegram/session"
	"strings"
)

const maxIterations = 10

// Agent is a stateful AI agent for one Telegram chat.
// It has a single tool (api_call) and an unbounded decision loop.
type Agent struct {
	apiTool    *apiCallTool
	getLLM     func() mcp.AIClient
	memory     *session.Memory
	systemPrompt string
}

// New creates an Agent for one chat session.
func New(apiPort int, botToken string, getLLM func() mcp.AIClient, systemPrompt string) *Agent {
	return &Agent{
		apiTool:      newAPICallTool(apiPort, botToken),
		getLLM:       getLLM,
		memory:       session.NewMemory(getLLM()),
		systemPrompt: systemPrompt,
	}
}

// GenerateBotToken creates a long-lived JWT for the bot's internal API calls.
// Call once at bot startup before creating any Agent or Manager.
func GenerateBotToken() (string, error) {
	return auth.GenerateJWT("default", "bot@internal")
}

// Run processes one user message through the agent loop.
// Loop: LLM decides -> if <api_call>: execute, append result, loop -> if no tag: return reply.
func (a *Agent) Run(userMessage string) string {
	llm := a.getLLM()
	if llm == nil {
		return "AI assistant unavailable. Please configure an AI model in the Web UI."
	}

	// Build turn messages: history context prefix + current user message
	histCtx := a.memory.BuildContext()
	firstMsg := userMessage
	if histCtx != "" {
		firstMsg = histCtx + "\n---\nUser: " + userMessage
	}
	turnMsgs := []mcp.Message{mcp.NewUserMessage(firstMsg)}

	var lastResp string

	for i := 0; i < maxIterations; i++ {
		req, err := mcp.NewRequestBuilder().
			WithSystemPrompt(a.systemPrompt).
			AddConversationHistory(turnMsgs).
			Build()
		if err != nil {
			logger.Errorf("Agent: failed to build request: %v", err)
			break
		}

		resp, err := llm.CallWithRequest(req)
		if err != nil {
			logger.Errorf("Agent: LLM call failed (iteration %d): %v", i+1, err)
			return "AI assistant temporarily unavailable. Please try again."
		}
		lastResp = resp

		apiReq, textBefore := parseAPICall(resp)
		if apiReq == nil {
			// No api_call tag — LLM gave a final answer
			reply := strings.TrimSpace(resp)
			a.memory.Add("user", userMessage)
			a.memory.Add("assistant", reply)
			return reply
		}

		logger.Infof("Agent: iter=%d %s %s", i+1, apiReq.Method, apiReq.Path)
		result := a.apiTool.execute(apiReq)

		if textBefore != "" {
			turnMsgs = append(turnMsgs, mcp.NewAssistantMessage(textBefore))
		}
		turnMsgs = append(turnMsgs, mcp.NewUserMessage(
			fmt.Sprintf("[API result: %s %s]\n%s", apiReq.Method, apiReq.Path, result),
		))
	}

	// Safety: max iterations reached — ask LLM for a final summary
	logger.Warnf("Agent: max iterations (%d) reached", maxIterations)
	turnMsgs = append(turnMsgs, mcp.NewUserMessage("Please summarize the results and give the user a final reply."))
	if finalReq, err := mcp.NewRequestBuilder().
		WithSystemPrompt(a.systemPrompt).
		AddConversationHistory(turnMsgs).
		Build(); err == nil {
		if finalResp, err := llm.CallWithRequest(finalReq); err == nil {
			lastResp = finalResp
		}
	}

	reply := strings.TrimSpace(lastResp)
	a.memory.Add("user", userMessage)
	a.memory.Add("assistant", reply)
	return reply
}

// ResetMemory clears conversation history (called on /start).
func (a *Agent) ResetMemory() {
	a.memory.ResetFull()
}
```

**Step 1: Create the file**

**Step 2: Build**

```bash
go build ./telegram/agent/...
```

**Step 3: Commit**

```bash
git add telegram/agent/agent.go
git commit -m "feat(telegram/agent): add OpenClaw-style agent loop"
```

---

## Task 6: Create `telegram/agent/manager.go`

**Files:**
- Create: `telegram/agent/manager.go`

```go
package agent

import (
	"nofx/mcp"
	"sync"
)

// Manager holds one Agent per Telegram chat ID.
// Messages for the same chat are serialized (OpenClaw Lane Queue pattern).
type Manager struct {
	mu           sync.Mutex
	agents       map[int64]*Agent
	lanes        map[int64]chan struct{}
	apiPort      int
	botToken     string
	getLLM       func() mcp.AIClient
	systemPrompt string
}

// NewManager creates a Manager. Call api.GetAPIDocs() before this and pass the result as apiDocs.
func NewManager(apiPort int, botToken string, getLLM func() mcp.AIClient, apiDocs string) *Manager {
	return &Manager{
		agents:       make(map[int64]*Agent),
		lanes:        make(map[int64]chan struct{}),
		apiPort:      apiPort,
		botToken:     botToken,
		getLLM:       getLLM,
		systemPrompt: BuildAgentPrompt(apiDocs),
	}
}

// Run processes a message for the given chat ID.
// If the same chat is already processing a message, this call blocks until it completes.
func (m *Manager) Run(chatID int64, userMessage string) string {
	a, lane := m.getOrCreate(chatID)
	lane <- struct{}{}
	defer func() { <-lane }()
	return a.Run(userMessage)
}

// Reset clears memory for the given chat (called on /start).
func (m *Manager) Reset(chatID int64) {
	m.mu.Lock()
	a, ok := m.agents[chatID]
	m.mu.Unlock()
	if ok {
		a.ResetMemory()
	}
}

func (m *Manager) getOrCreate(chatID int64) (*Agent, chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.agents[chatID]
	if !ok {
		a = New(m.apiPort, m.botToken, m.getLLM, m.systemPrompt)
		m.agents[chatID] = a
	}
	lane, ok := m.lanes[chatID]
	if !ok {
		lane = make(chan struct{}, 1) // binary semaphore: one message at a time per chat
		m.lanes[chatID] = lane
	}
	return a, lane
}
```

**Step 1: Create the file**

**Step 2: Build**

```bash
go build ./telegram/agent/...
```

**Step 3: Commit**

```bash
git add telegram/agent/manager.go
git commit -m "feat(telegram/agent): add per-chat agent manager with lane serialization"
```

---

## Task 7: Write tests

**Files:**
- Create: `telegram/agent/agent_test.go`

```go
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

func (m *mockLLM) SetAPIKey(_, _, _ string)       {}
func (m *mockLLM) SetTimeout(_ time.Duration)      {}
func (m *mockLLM) CallWithMessages(_, _ string) (string, error) { return m.next() }
func (m *mockLLM) CallWithRequest(req *mcp.Request) (string, error) {
	m.lastMsgs = req.Messages
	return m.next()
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

// TestAgentDirectReply: LLM replies without api_call — one call, direct reply.
func TestAgentDirectReply(t *testing.T) {
	llm := &mockLLM{responses: []string{"Hello! How can I help you?"}}
	a := New(8080, "tok", mockGetLLM(llm), testPrompt)

	reply := a.Run("hello")

	if reply != "Hello! How can I help you?" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if llm.calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", llm.calls)
	}
}

// TestAgentAPICall: LLM calls API, gets result, gives final reply — two LLM calls.
func TestAgentAPICall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/my-traders" {
			w.Write([]byte(`[{"id":"t1","name":"BTC Strategy"}]`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.Listener.Addr().String(), "127.0.0.1:%d", &port)

	llm := &mockLLM{responses: []string{
		`Let me check.<api_call>{"method":"GET","path":"/api/my-traders","body":{}}</api_call>`,
		"You have one trader: BTC Strategy.",
	}}
	a := New(port, "tok", mockGetLLM(llm), testPrompt)

	reply := a.Run("list my traders")

	if reply != "You have one trader: BTC Strategy." {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if llm.calls != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", llm.calls)
	}
}

// TestAgentMultiStep: LLM chains two API calls before final reply — three LLM calls.
func TestAgentMultiStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.Listener.Addr().String(), "127.0.0.1:%d", &port)

	llm := &mockLLM{responses: []string{
		`Checking account.<api_call>{"method":"GET","path":"/api/account","body":{}}</api_call>`,
		`Now checking positions.<api_call>{"method":"GET","path":"/api/positions","body":{}}</api_call>`,
		"Account looks healthy and no open positions.",
	}}
	a := New(port, "tok", mockGetLLM(llm), testPrompt)

	reply := a.Run("show me account status")

	if llm.calls != 3 {
		t.Fatalf("expected 3 LLM calls (2 api + 1 final), got %d", llm.calls)
	}
	if reply != "Account looks healthy and no open positions." {
		t.Fatalf("unexpected final reply: %q", reply)
	}
}

// TestAgentAPIResultInContext: API result must appear in next LLM message.
func TestAgentAPIResultInContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"balance":1234.56}`))
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.Listener.Addr().String(), "127.0.0.1:%d", &port)

	llm := &mockLLM{responses: []string{
		`<api_call>{"method":"GET","path":"/api/account","body":{}}</api_call>`,
		"Balance is 1234.56 USDT.",
	}}
	a := New(port, "tok", mockGetLLM(llm), testPrompt)
	a.Run("show balance")

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

// TestParseAPICall: unit tests for the XML tag parser.
func TestParseAPICall(t *testing.T) {
	t.Run("valid call", func(t *testing.T) {
		resp := `Stopping trader.<api_call>{"method":"POST","path":"/api/traders/t1/stop","body":{}}</api_call>`
		req, text := parseAPICall(resp)
		if req == nil {
			t.Fatal("expected api_call, got nil")
		}
		if req.Method != "POST" || req.Path != "/api/traders/t1/stop" {
			t.Fatalf("unexpected req: %+v", req)
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
```

**Step 1: Create the test file**

**Step 2: Run tests**

```bash
go test ./telegram/agent/... -v
```

Expected: all PASS.

**Step 3: Commit**

```bash
git add telegram/agent/agent_test.go
git commit -m "test(telegram/agent): add agent tests with mock HTTP server"
```

---

## Task 8: Simplify `telegram/session/session.go`

Replace file content:

```go
package session

import (
	"nofx/mcp"
	"sync"
	"time"
)

// Session holds conversation memory for a single Telegram chat.
type Session struct {
	ChatID    int64
	Memory    *Memory
	UpdatedAt time.Time
}

func (s *Session) ResetFull() { s.Memory.ResetFull() }

// Manager manages sessions by chat ID.
type Manager struct {
	mu       sync.RWMutex
	sessions map[int64]*Session
	llm      mcp.AIClient
}

func NewManager(llm mcp.AIClient) *Manager {
	return &Manager{sessions: make(map[int64]*Session), llm: llm}
}

func (m *Manager) Get(chatID int64) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[chatID]
	if !ok {
		s = &Session{ChatID: chatID, Memory: NewMemory(m.llm), UpdatedAt: time.Now()}
		m.sessions[chatID] = s
	}
	s.UpdatedAt = time.Now()
	return s
}
```

```bash
go build ./...
git add telegram/session/session.go
git commit -m "refactor(telegram/session): remove intent/params fields"
```

---

## Task 9: Wire `telegram/bot.go`

**Step 1: In `runBot`, replace old wiring with:**

```go
botToken, err := agent.GenerateBotToken()
if err != nil {
    logger.Errorf("Failed to generate bot JWT: %v", err)
    return false
}
agents := agent.NewManager(cfg.APIServerPort, botToken,
    func() mcp.AIClient { return newLLMClient(st) },
    api.GetAPIDocs(),
)
```

**Step 2: Replace `/start` reset:**
```go
// old: sessions.Get(chatID).ResetFull()
agents.Reset(chatID)
```

**Step 3: Replace message processing:**
```go
go func(chatID int64, text string) {
    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)) //nolint:errcheck
    reply := agents.Run(chatID, text)
    msg := tgbotapi.NewMessage(chatID, reply)
    msg.ParseMode = "Markdown"
    if _, err := bot.Send(msg); err != nil {
        msg.ParseMode = ""
        bot.Send(msg) //nolint:errcheck
    }
}(chatID, text)
```

**Step 4: Update imports** — remove `service`, `handler`, `intent`, `session`; add `agent`, `api`:

```go
import (
    "nofx/config"
    "nofx/logger"
    "nofx/manager"
    "nofx/mcp"
    "nofx/store"
    "nofx/api"
    "nofx/telegram/agent"
    "os"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)
```

**Step 5: Full build**

```bash
go build ./...
git add telegram/bot.go
git commit -m "feat(telegram): wire agent.Manager with auto-generated API docs"
```

---

## Task 10: Delete old files

```bash
git rm telegram/intent/parser.go telegram/handler/handler.go telegram/handler/handler_test.go
rmdir telegram/intent telegram/handler 2>/dev/null || true
go build ./... && go test ./...
git commit -m "refactor(telegram): delete old intent/handler packages"
```

---

## Task 11: End-to-end verification

```bash
go test ./telegram/... ./api/... -v -count=1
go build ./...
```

Manual verification — none of these scenarios need any special code:
- [ ] "hello" → natural conversation reply
- [ ] "list my traders" → GET /api/my-traders, formatted reply
- [ ] "show positions" → GET /api/positions
- [ ] "check balance then stop trader if loss > 5%" → multi-step: GET /api/account → POST /api/traders/:id/stop
- [ ] "create a BTC strategy with 5% stop loss" → GET /api/strategies/default-config → POST /api/strategies
- [ ] "show latest trading decisions" → GET /api/decisions/latest
- [ ] "what's the BTC 1h chart looking like" → GET /api/klines?symbol=BTCUSDT&interval=1h
- [ ] "delete trader xxx" → DELETE /api/traders/:id
- [ ] Any unrecognized input → LLM replies naturally, no error
