package agent

import (
	"encoding/json"
	"fmt"
	"nofx/auth"
	"nofx/logger"
	"nofx/mcp"
	"nofx/telegram/session"
	"strings"
)

const maxIterations = 10

// apiRequestTool is the single tool exposed to the LLM.
// Native function calling means the LLM returns EITHER ToolCalls OR Content — never both.
// This makes narration structurally impossible: text cannot appear alongside a tool call.
var apiRequestTool = mcp.Tool{
	Type: "function",
	Function: mcp.FunctionDef{
		Name:        "api_request",
		Description: "Call the NOFX trading system REST API",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"method": map[string]any{
					"type":        "string",
					"enum":        []string{"GET", "POST", "PUT", "DELETE"},
					"description": "HTTP method",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "API path; include query params in path: /api/positions?trader_id=xxx",
				},
				"body": map[string]any{
					"type":        "object",
					"description": "Request body; use {} for GET requests",
				},
			},
			"required": []string{"method", "path", "body"},
		},
	},
}

// Agent is a stateful AI agent for one Telegram chat.
// It exposes a single "api_request" tool and runs a loop until the LLM
// returns a plain-text reply (no tool calls).
type Agent struct {
	apiTool      *apiCallTool
	getLLM       func() mcp.AIClient
	memory       *session.Memory
	systemPrompt string
	userID       string
}

// New creates an Agent for one chat session.
func New(apiPort int, botToken, userID string, getLLM func() mcp.AIClient, systemPrompt string) *Agent {
	return &Agent{
		apiTool:      newAPICallTool(apiPort, botToken),
		getLLM:       getLLM,
		memory:       session.NewMemory(getLLM()),
		systemPrompt: systemPrompt,
		userID:       userID,
	}
}

// GenerateBotToken creates a long-lived JWT for the bot's internal API calls.
// userID must match the actual registered user's ID so bot-made changes
// are visible in the frontend (shared user namespace).
func GenerateBotToken(userID string) (string, error) {
	return auth.GenerateJWT(userID, "bot@internal")
}

// buildAccountContext fetches the live account state (models, exchanges, strategies, traders,
// and per-trader account summary + statistics) and returns it as a formatted string for
// injection into the LLM context at the start of each conversation.
func (a *Agent) buildAccountContext() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Current Account State — User: %s]\n\n", a.userID))

	// ── AI Models ─────────────────────────────────────────────────────────────
	modelsRaw := a.apiTool.execute(&apiRequest{Method: "GET", Path: "/api/models"})
	sb.WriteString("## AI Models\n")
	sb.WriteString("⚠️  When creating a trader, use the EXACT \"id\" value below for \"ai_model_id\".\n")
	sb.WriteString("    DO NOT use the \"provider\" field — it is NOT a valid ai_model_id.\n\n")

	var models []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(modelsRaw), &models); err == nil && len(models) > 0 {
		for _, m := range models {
			status := "disabled"
			if m.Enabled {
				status = "ENABLED"
			}
			sb.WriteString(fmt.Sprintf("  • ai_model_id=\"%s\"  provider=%s  name=%s  [%s]\n", m.ID, m.Provider, m.Name, status))
		}
	} else {
		sb.WriteString(modelsRaw)
	}
	sb.WriteString("\n")

	// ── Exchanges ─────────────────────────────────────────────────────────────
	exchangesRaw := a.apiTool.execute(&apiRequest{Method: "GET", Path: "/api/exchanges"})
	sb.WriteString("## Exchanges\n")
	sb.WriteString("⚠️  Use the EXACT \"id\" value below for \"exchange_id\" when creating a trader.\n\n")

	var exchanges []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ExchangeType string `json:"exchange_type"`
		AccountName  string `json:"account_name"`
		Enabled      bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(exchangesRaw), &exchanges); err == nil && len(exchanges) > 0 {
		for _, e := range exchanges {
			status := "disabled"
			if e.Enabled {
				status = "ENABLED"
			}
			sb.WriteString(fmt.Sprintf("  • exchange_id=\"%s\"  type=%s  account=%s  [%s]\n", e.ID, e.ExchangeType, e.AccountName, status))
		}
	} else {
		sb.WriteString(exchangesRaw)
	}
	sb.WriteString("\n")

	// ── Strategies ────────────────────────────────────────────────────────────
	strategiesRaw := a.apiTool.execute(&apiRequest{Method: "GET", Path: "/api/strategies"})
	sb.WriteString("## Strategies\n")

	var strategies []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(strategiesRaw), &strategies); err == nil && len(strategies) > 0 {
		for _, s := range strategies {
			sb.WriteString(fmt.Sprintf("  • strategy_id=\"%s\"  name=%s\n", s.ID, s.Name))
		}
	} else {
		sb.WriteString(strategiesRaw)
	}
	sb.WriteString("\n")

	// ── Traders ───────────────────────────────────────────────────────────────
	tradersRaw := a.apiTool.execute(&apiRequest{Method: "GET", Path: "/api/my-traders"})
	sb.WriteString("## Traders\n")

	var traders []struct {
		TraderID  string `json:"trader_id"`
		Name      string `json:"trader_name"`
		IsRunning bool   `json:"is_running"`
	}
	if err := json.Unmarshal([]byte(tradersRaw), &traders); err == nil && len(traders) > 0 {
		for _, t := range traders {
			status := "stopped"
			if t.IsRunning {
				status = "RUNNING"
			}
			sb.WriteString(fmt.Sprintf("  • trader_id=\"%s\"  name=%s  [%s]\n", t.TraderID, t.Name, status))
		}
	} else {
		sb.WriteString(tradersRaw)
	}
	sb.WriteString("\n")

	// ── Per-trader live data (running traders only) ────────────────────────────
	for _, t := range traders {
		if !t.IsRunning {
			continue
		}
		acct := a.apiTool.execute(&apiRequest{Method: "GET", Path: "/api/account?trader_id=" + t.TraderID})
		sb.WriteString(fmt.Sprintf("Account [%s]:\n%s\n\n", t.Name, acct))
		stats := a.apiTool.execute(&apiRequest{Method: "GET", Path: "/api/statistics?trader_id=" + t.TraderID})
		sb.WriteString(fmt.Sprintf("Statistics [%s]:\n%s\n\n", t.Name, stats))
	}

	return sb.String()
}

// Run processes one user message through the native function-calling agent loop.
//
// Architecture:
//   - LLM receives the api_request tool definition alongside conversation history.
//   - LLM response is EITHER ToolCalls (execute API) OR Content (final reply) — never both.
//     This is enforced by the protocol: narration is structurally impossible.
//   - Loop continues until the LLM returns a plain-text reply (no tool calls).
//
// On the first message of a conversation the live account state is fetched and injected.
// onChunk is optional; when set it is called once with the complete final reply text.
func (a *Agent) Run(userMessage string, onChunk func(string)) string {
	llm := a.getLLM()
	if llm == nil {
		return "AI assistant unavailable. Please configure an AI model in the Web UI."
	}

	// Build initial user message: prepend account state on first turn, history on subsequent turns.
	histCtx := a.memory.BuildContext()
	var firstUserContent string
	if histCtx == "" {
		accountCtx := a.buildAccountContext()
		firstUserContent = accountCtx + "\n[User Message]\n" + userMessage
	} else {
		firstUserContent = histCtx + "\n---\nUser: " + userMessage
	}

	turnMsgs := []mcp.Message{mcp.NewUserMessage(firstUserContent)}

	for i := 0; i < maxIterations; i++ {
		req, err := mcp.NewRequestBuilder().
			WithSystemPrompt(a.systemPrompt).
			AddConversationHistory(turnMsgs).
			AddTool(apiRequestTool).
			WithToolChoice("auto").
			Build()
		if err != nil {
			logger.Errorf("Agent: failed to build request: %v", err)
			break
		}

		resp, err := llm.CallWithRequestFull(req)
		if err != nil {
			logger.Errorf("Agent: LLM call failed (iteration %d): %v", i+1, err)
			return "AI assistant temporarily unavailable. Please try again."
		}

		// No tool calls → LLM returned a final text reply.
		if len(resp.ToolCalls) == 0 {
			reply := strings.TrimSpace(resp.Content)
			if onChunk != nil {
				onChunk(reply)
			}
			a.memory.Add("user", userMessage)
			a.memory.Add("assistant", reply)
			return reply
		}

		// Tool call iteration — show thinking indicator.
		if onChunk != nil {
			onChunk("⏳")
		}

		// Append assistant message carrying the tool calls (no content field).
		turnMsgs = append(turnMsgs, mcp.Message{
			Role:      "assistant",
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append the results as tool messages.
		for _, tc := range resp.ToolCalls {
			var apiReq apiRequest
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &apiReq); err != nil {
				logger.Errorf("Agent: invalid tool args for call %s: %v", tc.ID, err)
				turnMsgs = append(turnMsgs, mcp.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err.Error()),
				})
				continue
			}
			logger.Infof("Agent: iter=%d tool=%s %s %s", i+1, tc.ID, apiReq.Method, apiReq.Path)
			result := a.apiTool.execute(&apiReq)
			turnMsgs = append(turnMsgs, mcp.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	// Safety: max iterations reached.
	logger.Warnf("Agent: max iterations (%d) reached for message: %q", maxIterations, userMessage)
	reply := "操作已完成，请检查您的账户查看最新状态。"
	a.memory.Add("user", userMessage)
	a.memory.Add("assistant", reply)
	return reply
}

// ResetMemory clears conversation history (called on /start).
func (a *Agent) ResetMemory() {
	a.memory.ResetFull()
}
