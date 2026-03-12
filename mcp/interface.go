package mcp

import (
	"net/http"
	"time"
)

// ClientEmbedder is implemented by provider types that embed *Client,
// allowing generic extraction of the underlying base client (e.g. for cloning).
type ClientEmbedder interface {
	BaseClient() *Client
}

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

// ClientHooks is an alias for clientHooks to maintain compatibility
type ClientHooks clientHooks

// clientHooks is the internal dispatch interface used to implement per-provider
// polymorphism without Go's lack of virtual methods.
type clientHooks interface {
	// ── Simple CallWithMessages path ────────────────────────────────────────
	Call(systemPrompt, userPrompt string) (string, error)
	BuildMCPRequestBody(systemPrompt, userPrompt string) map[string]any

	// ── Shared request plumbing ─────────────────────────────────────────────
	BuildUrl() string
	BuildRequest(url string, jsonData []byte) (*http.Request, error)
	SetAuthHeader(reqHeaders http.Header)
	MarshalRequestBody(requestBody map[string]any) ([]byte, error)

	// ── Advanced (Request-object) path ──────────────────────────────────────
	BuildRequestBodyFromRequest(req *Request) map[string]any
	ParseMCPResponse(body []byte) (string, error)
	ParseMCPResponseFull(body []byte) (*LLMResponse, error)

	IsRetryableError(err error) bool
}
