package mcp

import (
	"crypto/ecdsa"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	ProviderClaw402    = "claw402"
	DefaultClaw402URL  = "https://claw402.ai"
	DefaultClaw402Model = "deepseek"
)

// claw402ModelEndpoints maps user-friendly model names to claw402 API paths.
var claw402ModelEndpoints = map[string]string{
	// OpenAI
	"gpt-5.4":     "/api/v1/ai/openai/chat/5.4",
	"gpt-5.4-pro": "/api/v1/ai/openai/chat/5.4-pro",
	"gpt-5.3":     "/api/v1/ai/openai/chat/5.3",
	"gpt-5-mini":  "/api/v1/ai/openai/chat/5-mini",
	// Anthropic
	"claude-opus": "/api/v1/ai/anthropic/messages/opus",
	// DeepSeek
	"deepseek":          "/api/v1/ai/deepseek/chat",
	"deepseek-reasoner": "/api/v1/ai/deepseek/chat/reasoner",
	// Qwen
	"qwen-max":   "/api/v1/ai/qwen/chat/max",
	"qwen-plus":  "/api/v1/ai/qwen/chat/plus",
	"qwen-turbo": "/api/v1/ai/qwen/chat/turbo",
	"qwen-flash": "/api/v1/ai/qwen/chat/flash",
	// Grok
	"grok-4.1": "/api/v1/ai/grok/chat/4.1",
	// Gemini
	"gemini-3.1-pro": "/api/v1/ai/gemini/chat/3.1-pro",
	// Kimi
	"kimi-k2.5": "/api/v1/ai/kimi/chat/k2.5",
}

// Claw402Client implements AIClient using claw402.ai's x402 v2 USDC payment gateway.
// Reuses the same EIP-712 signing as BlockRunBaseClient (same Base chain + USDC contract).
// When the selected model routes to an Anthropic endpoint, it automatically uses
// the Anthropic wire format for requests and responses (via an internal ClaudeClient).
type Claw402Client struct {
	*Client
	privateKey  *ecdsa.PrivateKey
	claudeProxy *ClaudeClient // non-nil when endpoint is /anthropic/
}

// NewClaw402Client creates a claw402 client (backward compatible).
func NewClaw402Client() AIClient {
	return NewClaw402ClientWithOptions()
}

// NewClaw402ClientWithOptions creates a claw402 client with options.
func NewClaw402ClientWithOptions(opts ...ClientOption) AIClient {
	baseOpts := []ClientOption{
		WithProvider(ProviderClaw402),
		WithModel(DefaultClaw402Model),
		WithBaseURL(DefaultClaw402URL),
	}
	allOpts := append(baseOpts, opts...)
	baseClient := NewClient(allOpts...).(*Client)
	baseClient.UseFullURL = true
	baseClient.BaseURL = DefaultClaw402URL + claw402ModelEndpoints[DefaultClaw402Model]

	c := &Claw402Client{Client: baseClient}
	baseClient.hooks = c
	return c
}

// SetAPIKey stores the EVM private key and selects the model endpoint.
func (c *Claw402Client) SetAPIKey(apiKey string, _ string, customModel string) {
	hexKey := strings.TrimPrefix(apiKey, "0x")
	privKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		c.logger.Warnf("⚠️  [MCP] Claw402: invalid private key: %v", err)
	} else {
		c.privateKey = privKey
		c.APIKey = apiKey
		addr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()
		c.logger.Infof("🔧 [MCP] Claw402 wallet: %s", addr)
	}
	if customModel != "" {
		c.Model = customModel
	}
	endpoint := c.resolveEndpoint()
	c.BaseURL = DefaultClaw402URL + endpoint

	// Anthropic endpoints need different wire format (Messages API)
	if strings.Contains(endpoint, "/anthropic/") {
		c.claudeProxy = &ClaudeClient{Client: c.Client}
		c.logger.Infof("🔧 [MCP] Claw402 model: %s → %s (Anthropic format)", c.Model, endpoint)
	} else {
		c.claudeProxy = nil
		c.logger.Infof("🔧 [MCP] Claw402 model: %s → %s", c.Model, endpoint)
	}
}

// resolveEndpoint returns the API path for the configured model.
func (c *Claw402Client) resolveEndpoint() string {
	if ep, ok := claw402ModelEndpoints[c.Model]; ok {
		return ep
	}
	// Allow raw path override (e.g. "/api/v1/ai/openai/chat/5.4")
	if strings.HasPrefix(c.Model, "/api/") {
		return c.Model
	}
	return claw402ModelEndpoints[DefaultClaw402Model]
}

func (c *Claw402Client) setAuthHeader(h http.Header) { x402SetAuthHeader(h) }

func (c *Claw402Client) call(systemPrompt, userPrompt string) (string, error) {
	return x402Call(c.Client, c.signPayment, "Claw402", systemPrompt, userPrompt)
}

func (c *Claw402Client) CallWithRequestFull(req *Request) (*LLMResponse, error) {
	return x402CallFull(c.Client, c.signPayment, "Claw402", req)
}

// signPayment signs x402 v2 EIP-712 payment (same Base chain + USDC as BlockRunBase).
func (c *Claw402Client) signPayment(paymentHeaderB64 string) (string, error) {
	return signBasePaymentHeader(c.privateKey, paymentHeaderB64, "Claw402")
}

// ── Format overrides for Anthropic endpoints ─────────────────────────────────

func (c *Claw402Client) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	if c.claudeProxy != nil {
		return c.claudeProxy.buildMCPRequestBody(systemPrompt, userPrompt)
	}
	return c.Client.buildMCPRequestBody(systemPrompt, userPrompt)
}

func (c *Claw402Client) buildRequestBodyFromRequest(req *Request) map[string]any {
	if c.claudeProxy != nil {
		return c.claudeProxy.buildRequestBodyFromRequest(req)
	}
	return c.Client.buildRequestBodyFromRequest(req)
}

func (c *Claw402Client) parseMCPResponse(body []byte) (string, error) {
	if c.claudeProxy != nil {
		return c.claudeProxy.parseMCPResponse(body)
	}
	return c.Client.parseMCPResponse(body)
}

func (c *Claw402Client) parseMCPResponseFull(body []byte) (*LLMResponse, error) {
	if c.claudeProxy != nil {
		return c.claudeProxy.parseMCPResponseFull(body)
	}
	return c.Client.parseMCPResponseFull(body)
}

// buildUrl returns the full claw402 endpoint URL.
func (c *Claw402Client) buildUrl() string {
	return c.BaseURL
}

func (c *Claw402Client) buildRequest(url string, jsonData []byte) (*http.Request, error) {
	return x402BuildRequest(url, jsonData)
}
