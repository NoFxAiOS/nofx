package mcp

import (
	"net/http"
	"time"
)

// AIClient public AI client interface (for external use)
type AIClient interface {
	SetAPIKey(apiKey string, customURL string, customModel string)
	SetTimeout(timeout time.Duration)
	CallWithMessages(systemPrompt, userPrompt string) (string, error)
	CallWithRequest(req *Request) (string, error)
	// CallWithRequestStream streams the LLM response via SSE.
	// onChunk is called with the full accumulated text so far (not raw deltas).
	// Returns the complete final text when done.
	CallWithRequestStream(req *Request, onChunk func(string)) (string, error)
	// CallWithRequestFull returns both text content and tool calls.
	// Use this when the request includes Tools — the LLM may respond with
	// either a plain text reply (LLMResponse.Content) or tool invocations
	// (LLMResponse.ToolCalls), but not both.
	CallWithRequestFull(req *Request) (*LLMResponse, error)
}

// clientHooks is the internal dispatch interface used to implement per-provider
// polymorphism without Go's lack of virtual methods.
//
// Each method can be overridden by an embedding struct (e.g. ClaudeClient).
// The base *Client provides OpenAI-compatible defaults; providers with a
// different wire format (Anthropic, Gemini native, etc.) override only what
// differs.  All call-path methods in client.go invoke these via c.hooks so
// that the override is always picked up at runtime.
type clientHooks interface {
	// ── Simple CallWithMessages path ────────────────────────────────────────
	call(systemPrompt, userPrompt string) (string, error)
	buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any

	// ── Shared request plumbing ─────────────────────────────────────────────
	buildUrl() string
	buildRequest(url string, jsonData []byte) (*http.Request, error)
	setAuthHeader(reqHeaders http.Header)
	marshalRequestBody(requestBody map[string]any) ([]byte, error)

	// ── Advanced (Request-object) path ──────────────────────────────────────
	// buildRequestBodyFromRequest converts a *Request into the provider's
	// native wire-format map.  Providers that use a different protocol (e.g.
	// Anthropic uses "input_schema" for tools, "tool_use" content blocks, and
	// a top-level "system" field) override this method.
	buildRequestBodyFromRequest(req *Request) map[string]any

	// parseMCPResponse extracts the plain-text reply from a non-streaming
	// response body.
	parseMCPResponse(body []byte) (string, error)

	// parseMCPResponseFull extracts both text and tool calls.  Providers whose
	// response envelope differs from the OpenAI choices[] structure (e.g.
	// Anthropic content[] with tool_use blocks) override this method.
	parseMCPResponseFull(body []byte) (*LLMResponse, error)

	isRetryableError(err error) bool
}
