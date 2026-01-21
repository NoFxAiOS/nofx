package mcp

import (
	"fmt"
	"net/http"

	x402 "github.com/coinbase/x402/go"
	x402http "github.com/coinbase/x402/go/http"
	evm "github.com/coinbase/x402/go/mechanisms/evm/exact/client"
	evmsigners "github.com/coinbase/x402/go/signers/evm"
)

const (
	ProviderOpenGradient       = "opengradient"
	DefaultOpenGradientBaseURL = "https://api.opengradient.ai/v1"
	DefaultOpenGradientModel   = "llama-3.3-70b"
)

type OpenGradientClient struct {
	*Client
	privateKey   string
	x402Client   *x402.X402Client
	x402Wrapped  bool
}

// NewOpenGradientClient creates OpenGradient client (backward compatible)
func NewOpenGradientClient() AIClient {
	return NewOpenGradientClientWithOptions()
}

// NewOpenGradientClientWithOptions creates OpenGradient client (supports options pattern)
//
// Usage examples:
//
//	// Basic usage (requires private key to be set later via SetPrivateKey)
//	client := mcp.NewOpenGradientClientWithOptions()
//
//	// With private key for x402 payments
//	client := mcp.NewOpenGradientClientWithOptions(
//	    mcp.WithOpenGradientPrivateKey("0x..."),
//	    mcp.WithModel("custom-model"),
//	)
func NewOpenGradientClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create OpenGradient preset options
	ogOpts := []ClientOption{
		WithProvider(ProviderOpenGradient),
		WithModel(DefaultOpenGradientModel),
		WithBaseURL(DefaultOpenGradientBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(ogOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create OpenGradient client
	ogClient := &OpenGradientClient{
		Client:     baseClient,
		privateKey: baseClient.config.OpenGradientPrivateKey,
	}

	// 5. Initialize x402 if private key is provided
	if ogClient.privateKey != "" {
		if err := ogClient.initX402(); err != nil {
			baseClient.logger.Warnf("⚠️ [MCP] Failed to initialize x402: %v", err)
		}
	}

	// 6. Set hooks to point to OpenGradientClient (implement dynamic dispatch)
	baseClient.hooks = ogClient

	return ogClient
}

// initX402 initializes the x402 client with EVM signer
func (c *OpenGradientClient) initX402() error {
	// Create EVM signer from private key
	signer, err := evmsigners.NewClientSignerFromPrivateKey(c.privateKey)
	if err != nil {
		return fmt.Errorf("failed to create EVM signer: %w", err)
	}

	// Create x402 client with EVM scheme registration
	c.x402Client = x402.Newx402Client().
		Register("eip155:*", evm.NewExactEvmScheme(signer))

	// Wrap HTTP client with x402 payment support
	c.httpClient = x402http.WrapHTTPClientWithPayment(
		c.httpClient,
		x402http.Newx402HTTPClient(c.x402Client),
	)
	c.x402Wrapped = true

	c.logger.Infof("🔐 [MCP] OpenGradient x402 payment initialized")
	return nil
}

// SetPrivateKey sets the EVM private key and initializes x402
func (c *OpenGradientClient) SetPrivateKey(privateKey string) error {
	c.privateKey = privateKey
	return c.initX402()
}

func (c *OpenGradientClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	// For OpenGradient with x402, the apiKey parameter is used as the private key
	c.privateKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] OpenGradient Private Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] OpenGradient using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] OpenGradient using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] OpenGradient using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] OpenGradient using default Model: %s", c.Model)
	}

	// Initialize x402 with the private key
	if c.privateKey != "" && !c.x402Wrapped {
		if err := c.initX402(); err != nil {
			c.logger.Warnf("⚠️ [MCP] Failed to initialize x402: %v", err)
		}
	}

	// Set a placeholder API key for the base client (required for CallWithMessages check)
	c.APIKey = "x402-authenticated"
}

func (c *OpenGradientClient) setAuthHeader(reqHeaders http.Header) {
	// x402 handles authentication via the wrapped HTTP client
	// No Bearer token needed - the x402 wrapper intercepts 402 responses
	// and automatically adds payment signatures
}
