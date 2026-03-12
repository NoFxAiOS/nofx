package mcp

import "net/http"

// ClientHooks is the dispatch interface used to implement per-provider
// polymorphism without Go's lack of virtual methods.
//
// Each method can be overridden by an embedding struct (e.g. provider.ClaudeClient).
// The base *Client provides OpenAI-compatible defaults; providers with a
// different wire format (Anthropic, Gemini native, etc.) override only what
// differs.  All call-path methods in client.go invoke these via c.Hooks so
// that the override is always picked up at runtime.
type ClientHooks interface {
	// ── Simple CallWithMessages path ────────────────────────────────────────
	Call(systemPrompt, userPrompt string) (string, error)
	BuildMCPRequestBody(systemPrompt, userPrompt string) map[string]any

	// ── Shared request plumbing ─────────────────────────────────────────────
	BuildUrl() string
	BuildRequest(url string, jsonData []byte) (*http.Request, error)
	SetAuthHeader(reqHeaders http.Header)
	MarshalRequestBody(requestBody map[string]any) ([]byte, error)

	// ── Advanced (Request-object) path ──────────────────────────────────────
	// BuildRequestBodyFromRequest converts a *Request into the provider's
	// native wire-format map.
	BuildRequestBodyFromRequest(req *Request) map[string]any

	// ParseMCPResponse extracts the plain-text reply from a non-streaming
	// response body.
	ParseMCPResponse(body []byte) (string, error)

	// ParseMCPResponseFull extracts both text and tool calls.
	ParseMCPResponseFull(body []byte) (*LLMResponse, error)

	IsRetryableError(err error) bool
}
