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
	CallWithRequest(req *Request) (string, error) // Builder pattern API (supports advanced features)
}

// clientHooks internal hook interface (for subclass to override specific steps)
// These methods are only used inside the package to implement dynamic dispatch
type clientHooks interface {
	// Hook methods that can be overridden by subclass

	call(systemPrompt, userPrompt string) (string, error)

	buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any
	buildUrl() string
	buildRequest(url string, jsonData []byte) (*http.Request, error)
	setAuthHeader(reqHeaders http.Header)
	marshalRequestBody(requestBody map[string]any) ([]byte, error)
	parseMCPResponse(body []byte) (string, error)
	isRetryableError(err error) bool
}
