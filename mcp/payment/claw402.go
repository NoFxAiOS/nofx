package payment

import (
	"crypto/ecdsa"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"nofx/mcp"
	"nofx/mcp/provider"
)

const (
	DefaultClaw402URL   = "https://claw402.ai"
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

func init() {
	mcp.RegisterProvider(mcp.ProviderClaw402, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewClaw402ClientWithOptions(opts...)
	})
}

// Claw402Client implements AIClient using claw402.ai's x402 v2 USDC payment gateway.
// When the selected model routes to an Anthropic endpoint, it automatically uses
// the Anthropic wire format for requests and responses (via an internal ClaudeClient).
type Claw402Client struct {
	*mcp.Client
	privateKey  *ecdsa.PrivateKey
	claudeProxy *provider.ClaudeClient // non-nil when endpoint is /anthropic/
}

func (c *Claw402Client) BaseClient() *mcp.Client { return c.Client }

// NewClaw402Client creates a claw402 client (backward compatible).
func NewClaw402Client() mcp.AIClient {
	return NewClaw402ClientWithOptions()
}

// NewClaw402ClientWithOptions creates a claw402 client with options.
func NewClaw402ClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	baseOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderClaw402),
		mcp.WithModel(DefaultClaw402Model),
		mcp.WithBaseURL(DefaultClaw402URL),
		mcp.WithTimeout(X402Timeout),
		mcp.WithMaxRetries(1), // disable outer retry — inner x402 loop handles retries; outer retry causes duplicate payments
	}
	allOpts := append(baseOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)
	baseClient.UseFullURL = true
	baseClient.BaseURL = DefaultClaw402URL + claw402ModelEndpoints[DefaultClaw402Model]

	c := &Claw402Client{Client: baseClient}
	baseClient.Hooks = c
	return c
}

// SetAPIKey stores the EVM private key and selects the model endpoint.
func (c *Claw402Client) SetAPIKey(apiKey string, _ string, customModel string) {
	hexKey := strings.TrimPrefix(apiKey, "0x")
	privKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		c.Log.Warnf("⚠️  [MCP] Claw402: invalid private key: %v", err)
	} else {
		c.privateKey = privKey
		c.APIKey = apiKey
		addr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()
		c.Log.Infof("🔧 [MCP] Claw402 wallet: %s", addr)
	}
	if customModel != "" {
		c.Model = customModel
	}
	endpoint := c.resolveEndpoint()
	c.BaseURL = DefaultClaw402URL + endpoint

	// Anthropic endpoints need different wire format (Messages API)
	if strings.Contains(endpoint, "/anthropic/") {
		c.claudeProxy = &provider.ClaudeClient{Client: c.Client}
		c.Log.Infof("🔧 [MCP] Claw402 model: %s → %s (Anthropic format)", c.Model, endpoint)
	} else {
		c.claudeProxy = nil
		c.Log.Infof("🔧 [MCP] Claw402 model: %s → %s", c.Model, endpoint)
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

func (c *Claw402Client) SetAuthHeader(h http.Header) { X402SetAuthHeader(h) }

func (c *Claw402Client) Call(systemPrompt, userPrompt string) (string, error) {
	return X402Call(c.Client, c.signPayment, "Claw402", systemPrompt, userPrompt)
}

func (c *Claw402Client) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	return X402CallFull(c.Client, c.signPayment, "Claw402", req)
}

// signPayment signs x402 v2 EIP-712 payment (same Base chain + USDC as BlockRunBase).
func (c *Claw402Client) signPayment(paymentHeaderB64 string) (string, error) {
	return SignBasePaymentHeader(c.privateKey, paymentHeaderB64, "Claw402")
}

// ── Format overrides for Anthropic endpoints ─────────────────────────────────

func (c *Claw402Client) BuildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	if c.claudeProxy != nil {
		return c.claudeProxy.BuildMCPRequestBody(systemPrompt, userPrompt)
	}
	return c.Client.BuildMCPRequestBody(systemPrompt, userPrompt)
}

func (c *Claw402Client) BuildRequestBodyFromRequest(req *mcp.Request) map[string]any {
	if c.claudeProxy != nil {
		return c.claudeProxy.BuildRequestBodyFromRequest(req)
	}
	return c.Client.BuildRequestBodyFromRequest(req)
}

func (c *Claw402Client) ParseMCPResponse(body []byte) (string, error) {
	if c.claudeProxy != nil {
		return c.claudeProxy.ParseMCPResponse(body)
	}
	return c.Client.ParseMCPResponse(body)
}

func (c *Claw402Client) ParseMCPResponseFull(body []byte) (*mcp.LLMResponse, error) {
	if c.claudeProxy != nil {
		return c.claudeProxy.ParseMCPResponseFull(body)
	}
	return c.Client.ParseMCPResponseFull(body)
}

// BuildUrl returns the full claw402 endpoint URL.
func (c *Claw402Client) BuildUrl() string {
	return c.BaseURL
}

func (c *Claw402Client) BuildRequest(url string, jsonData []byte) (*http.Request, error) {
	return X402BuildRequest(url, jsonData)
}
