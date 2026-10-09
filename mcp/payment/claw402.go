package payment

import (
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"nofx/mcp"
	"nofx/mcp/provider"
	"nofx/store"
	"nofx/wallet"
)

// Per-call cost buffers for preflight. Reasoner models emit long chain-of-thought
// tokens whose cost can far exceed the flat per-call estimate in store.GetModelPrice,
// so they use a larger multiplier.
const (
	preflightSafetyMultiplier         = 1.5
	preflightReasonerSafetyMultiplier = 4.0
)

// ErrInsufficientFunds is returned when the claw402 wallet does not hold
// enough USDC to cover the estimated cost of a call. Callers can type-assert
// to surface balance/needed/address to the UI.
type ErrInsufficientFunds struct {
	Address string
	Balance float64
	Needed  float64
	Model   string
}

func (e *ErrInsufficientFunds) Error() string {
	return fmt.Sprintf(
		"claw402 insufficient USDC: wallet=%s balance=$%.4f needed=$%.4f model=%s",
		shortAddr(e.Address), e.Balance, e.Needed, e.Model,
	)
}

// shortAddr renders 0x1234…abcd for log/error strings that may leak into
// telemetry bundles. The full address stays on the struct for programmatic use.
func shortAddr(addr string) string {
	if len(addr) < 10 {
		return addr
	}
	return addr[:6] + "…" + addr[len(addr)-4:]
}

const (
	DefaultClaw402URL   = "https://claw402.ai"
	DefaultClaw402Model = "gpt-5.6"
)

// claw402ModelEndpoints maps user-friendly model names to claw402 API paths.
// Must stay in sync with the claw402 catalog (GET /api/v1/catalog).
var claw402ModelEndpoints = map[string]string{
	// OpenAI
	"gpt-6":         "/api/v1/ai/openai/chat/6",
	"gpt-5.6":       "/api/v1/ai/openai/chat/5.6",
	"gpt-5.6-terra": "/api/v1/ai/openai/chat/5.6-terra",
	"gpt-5.6-luna":  "/api/v1/ai/openai/chat/5.6-luna",
	// Anthropic
	"claude-fable": "/api/v1/ai/anthropic/messages/fable",
	"claude-opus":  "/api/v1/ai/anthropic/messages/opus",
	// DeepSeek
	"deepseek":          "/api/v1/ai/deepseek/chat",
	"deepseek-reasoner": "/api/v1/ai/deepseek/chat/reasoner",
	"deepseek-v4-flash": "/api/v1/ai/deepseek/v4-flash",
	"deepseek-v4-pro":   "/api/v1/ai/deepseek/v4-pro",
	// Z.AI (Zhipu)
	"glm-5": "/api/v1/ai/zhipu/chat",
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

// LastCallCostUSD reports the actually-settled cost of the most recent call:
// the gateway's X-Claw402-Settled-Usd header when available (non-streaming),
// otherwise derived from streamed token usage via the gateway's own formula —
// on SSE responses the header cannot be delivered because HTTP headers are
// flushed before usage is known. ok is false when neither source is available;
// callers should then fall back to the flat catalog price.
func (c *Claw402Client) LastCallCostUSD() (float64, bool) {
	if c.Client.LastCallSettledUSD > 0 {
		return c.Client.LastCallSettledUSD, true
	}
	if u := c.Client.LastCallUsage; u != nil && u.PromptTokens+u.CompletionTokens > 0 {
		if cost, ok := store.ComputeUsageCost(c.Model, u.PromptTokens, u.CompletionTokens); ok {
			return cost, true
		}
	}
	return 0, false
}

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
	// Clear first so a malformed rotation can never keep spending from the
	// previously configured wallet.
	c.privateKey = nil
	c.APIKey = ""
	hexKey := strings.TrimSpace(apiKey)
	if len(hexKey) >= 2 && strings.EqualFold(hexKey[:2], "0x") {
		hexKey = hexKey[2:]
	}
	privKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		c.Log.Warnf("⚠️  [MCP] Claw402: invalid private key: %v", err)
	} else {
		c.privateKey = privKey
		c.APIKey = hexKey
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
	if err := c.preflightBalance(); err != nil {
		return "", err
	}
	return X402CallStream(c.Client, c.signPayment, "Claw402", systemPrompt, userPrompt, nil)
}

func (c *Claw402Client) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	if err := c.preflightBalance(); err != nil {
		return nil, err
	}
	return X402CallFull(c.Client, c.signPayment, "Claw402", req)
}

// walletAddress derives the EVM address from the configured private key.
// Returns "" when no key has been set (client unconfigured).
func (c *Claw402Client) walletAddress() string {
	if c.privateKey == nil {
		return ""
	}
	return crypto.PubkeyToAddress(c.privateKey.PublicKey).Hex()
}

// preflightBalance short-circuits a call when the wallet cannot cover the
// estimated cost. RPC failures fall through — x402 will still reject an
// actually-empty wallet, so we prefer availability over extra strictness.
func (c *Claw402Client) preflightBalance() error {
	addr := c.walletAddress()
	if addr == "" {
		return nil
	}
	balance, err := wallet.QueryUSDCBalanceCached(addr)
	if err != nil {
		c.Log.Warnf("⚠️  [MCP] Claw402 balance preflight skipped (RPC error): %v", err)
		return nil
	}
	multiplier := preflightSafetyMultiplier
	if strings.Contains(strings.ToLower(c.Model), "reasoner") {
		multiplier = preflightReasonerSafetyMultiplier
	}
	needed := store.GetModelPrice(c.Model) * multiplier
	if balance < needed {
		return &ErrInsufficientFunds{
			Address: addr,
			Balance: balance,
			Needed:  needed,
			Model:   c.Model,
		}
	}
	return nil
}

// signPayment signs x402 v2 EIP-712 payment on Base chain + USDC.
func (c *Claw402Client) signPayment(paymentHeaderB64 string) (string, error) {
	return SignBasePaymentHeader(c.privateKey, paymentHeaderB64, "Claw402")
}

// ── Format overrides for Anthropic endpoints ─────────────────────────────────

// stripMaxTokens removes per-call max_tokens caps from a body destined for
// claw402. The gateway already enforces a per-route default/floor/cap
// (see providers/*.yaml token_default_max_out / token_min_max_out /
// token_max_out_cap). Sending a small max_tokens here on a thinking model
// (Kimi K2.5, DeepSeek R1/V4) caused reasoning tokens to consume the entire
// budget and left `delta.content` empty, surfacing as "no content received".
// upto settles on real usage, so removing the cap costs nothing extra.
func stripMaxTokens(body map[string]any) map[string]any {
	if body == nil {
		return body
	}
	delete(body, "max_tokens")
	delete(body, "max_completion_tokens")
	return body
}

// applyClawModelControls prevents DeepSeek V4 from spending the gateway's
// entire output allowance on reasoning_content and returning no final answer.
// The gateway accepts the standard thinking control on these V4 routes.
func applyClawModelControls(body map[string]any, model string) map[string]any {
	if body == nil {
		return body
	}
	switch model {
	case "deepseek-v4-flash", "deepseek-v4-pro":
		body["thinking"] = map[string]any{"type": "disabled"}
	}
	return body
}

func (c *Claw402Client) BuildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	if c.claudeProxy != nil {
		return c.claudeProxy.BuildMCPRequestBody(systemPrompt, userPrompt)
	}
	body := stripMaxTokens(c.Client.BuildMCPRequestBody(systemPrompt, userPrompt))
	return applyClawModelControls(body, c.Model)
}

func (c *Claw402Client) BuildRequestBodyFromRequest(req *mcp.Request) map[string]any {
	if c.claudeProxy != nil {
		return c.claudeProxy.BuildRequestBodyFromRequest(req)
	}
	body := stripMaxTokens(c.Client.BuildRequestBodyFromRequest(req))
	return applyClawModelControls(body, c.Model)
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
