package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"regexp"
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

// apiRequest holds the arguments decoded from the LLM's api_request tool call.
type apiRequest struct {
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Body   map[string]any `json:"body"`
}

// allowedRoute is one entry in the LLM tool allowlist. The bot agent runs with
// a real user JWT, so we MUST default-deny: any path not listed here is rejected
// before the HTTP call is made. This prevents prompt-injection (via account
// names, strategy names, etc. injected into the LLM context) from coercing the
// bot into changing the user's password, swapping exchange credentials, or
// pointing the LLM API key at an attacker-controlled URL.
type allowedRoute struct {
	method  string
	pattern *regexp.Regexp
}

// botAPIAllowlist enumerates the endpoints the Telegram LLM agent is permitted
// to call. Keep this LIST SHORT and DEFAULT-DENY. To grant the bot access to a
// new endpoint, add an explicit entry here — never widen wildcards.
//
// Explicitly NOT allowed (and must never be added without a human-in-the-loop
// confirmation flow):
//   - PUT  /api/user/password   (password takeover)
//   - PUT  /api/models          (LLM API key + endpoint swap → exfil)
//   - POST/PUT/DELETE /api/exchanges*  (exchange credential swap → drain)
//   - POST /api/reset-password, /api/reset-account  (destructive)
//   - POST /api/wallet/generate, /api/wallet/validate
//   - POST /api/telegram/* (rebind bot)
var botAPIAllowlist = []allowedRoute{
	// Read-only endpoints that surface state to the user.
	{"GET", regexp.MustCompile(`^/api/health$`)},
	{"GET", regexp.MustCompile(`^/api/config$`)},
	{"GET", regexp.MustCompile(`^/api/supported-models$`)},
	{"GET", regexp.MustCompile(`^/api/supported-exchanges$`)},
	{"GET", regexp.MustCompile(`^/api/models$`)},
	{"GET", regexp.MustCompile(`^/api/exchanges$`)},
	{"GET", regexp.MustCompile(`^/api/exchanges/account-state$`)},
	{"GET", regexp.MustCompile(`^/api/strategies(/[^/]+)?$`)},
	{"GET", regexp.MustCompile(`^/api/strategies/active$`)},
	{"GET", regexp.MustCompile(`^/api/strategies/default-config$`)},
	{"GET", regexp.MustCompile(`^/api/strategies/public$`)},
	{"GET", regexp.MustCompile(`^/api/my-traders$`)},
	{"GET", regexp.MustCompile(`^/api/traders$`)},
	{"GET", regexp.MustCompile(`^/api/traders/[^/]+/config$`)},
	{"GET", regexp.MustCompile(`^/api/traders/[^/]+/public-config$`)},
	{"GET", regexp.MustCompile(`^/api/traders/[^/]+/grid-risk$`)},
	{"GET", regexp.MustCompile(`^/api/competition$`)},
	{"GET", regexp.MustCompile(`^/api/top-traders$`)},
	{"GET", regexp.MustCompile(`^/api/equity-history$`)},
	{"GET", regexp.MustCompile(`^/api/klines$`)},
	{"GET", regexp.MustCompile(`^/api/symbols$`)},
	{"GET", regexp.MustCompile(`^/api/status$`)},
	{"GET", regexp.MustCompile(`^/api/account$`)},
	{"GET", regexp.MustCompile(`^/api/positions$`)},
	{"GET", regexp.MustCompile(`^/api/positions/history$`)},
	{"GET", regexp.MustCompile(`^/api/trades$`)},
	{"GET", regexp.MustCompile(`^/api/orders$`)},
	{"GET", regexp.MustCompile(`^/api/orders/[^/]+/fills$`)},
	{"GET", regexp.MustCompile(`^/api/open-orders$`)},
	{"GET", regexp.MustCompile(`^/api/decisions$`)},
	{"GET", regexp.MustCompile(`^/api/decisions/latest$`)},
	{"GET", regexp.MustCompile(`^/api/statistics$`)},
	{"GET", regexp.MustCompile(`^/api/ai-costs$`)},
	{"GET", regexp.MustCompile(`^/api/ai-costs/summary$`)},

	// Write endpoints — trader and strategy lifecycle. These let the bot create
	// traders and strategies the user has asked for, and start/stop them. NOT
	// including any endpoint that mutates credentials, passwords, or pointers
	// to external services (LLM API URL, exchange API keys, telegram binding).
	// Strategy configs are server-side-validated for risk caps in the API
	// layer, so strategy create/update here cannot escape the user's risk
	// boundary.
	{"POST", regexp.MustCompile(`^/api/traders$`)},
	{"PUT", regexp.MustCompile(`^/api/traders/[^/]+$`)},
	{"DELETE", regexp.MustCompile(`^/api/traders/[^/]+$`)},
	{"POST", regexp.MustCompile(`^/api/traders/[^/]+/start$`)},
	{"POST", regexp.MustCompile(`^/api/traders/[^/]+/stop$`)},
	{"POST", regexp.MustCompile(`^/api/traders/[^/]+/sync-balance$`)},
	{"POST", regexp.MustCompile(`^/api/traders/[^/]+/close-position$`)},
	{"PUT", regexp.MustCompile(`^/api/traders/[^/]+/prompt$`)},
	{"PUT", regexp.MustCompile(`^/api/traders/[^/]+/competition$`)},
	{"POST", regexp.MustCompile(`^/api/strategies$`)},
	{"PUT", regexp.MustCompile(`^/api/strategies/[^/]+$`)},
	{"DELETE", regexp.MustCompile(`^/api/strategies/[^/]+$`)},
	{"POST", regexp.MustCompile(`^/api/strategies/[^/]+/activate$`)},
	{"POST", regexp.MustCompile(`^/api/strategies/[^/]+/duplicate$`)},
}

// isPathAllowed returns true when the (method, path) pair is in botAPIAllowlist.
// The path argument should already be query-stripped.
func isPathAllowed(method, path string) bool {
	for _, r := range botAPIAllowlist {
		if r.method == method && r.pattern.MatchString(path) {
			return true
		}
	}
	return false
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

	// SECURITY: default-deny allowlist enforcement. Without this, prompt
	// injection via user-controlled fields (account_name, strategy name,
	// trader name) could coerce the LLM into calling sensitive endpoints
	// like PUT /api/user/password or PUT /api/exchanges with the bot's JWT.
	method := strings.ToUpper(req.Method)
	pathOnly := req.Path
	if i := strings.IndexByte(pathOnly, '?'); i >= 0 {
		pathOnly = pathOnly[:i]
	}
	if !isPathAllowed(method, pathOnly) {
		logger.Warnf("Agent: blocked disallowed tool call %s %s (path not in botAPIAllowlist)", method, pathOnly)
		return fmt.Sprintf(
			`{"error":"endpoint not allowed for the chat agent","method":%q,"path":%q,"hint":"ask the user to perform this action in the web UI"}`,
			method, pathOnly,
		)
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

