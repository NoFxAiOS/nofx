package provider

import (
	"net/http"

	"nofx/mcp"
)

const (
	DefaultQwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	DefaultQwenModel   = "qwen3-max"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderQwen, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewQwenClientWithOptions(opts...)
	})
}

type QwenClient struct {
	*mcp.Client
}

func (c *QwenClient) BaseClient() *mcp.Client { return c.Client }

// NewQwenClient creates Qwen client (backward compatible)
//
// Deprecated: Recommend using NewQwenClientWithOptions for better flexibility
func NewQwenClient() mcp.AIClient {
	return NewQwenClientWithOptions()
}

// NewQwenClientWithOptions creates Qwen client (supports options pattern)
func NewQwenClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	qwenOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderQwen),
		mcp.WithModel(DefaultQwenModel),
		mcp.WithBaseURL(DefaultQwenBaseURL),
	}

	allOpts := append(qwenOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	qwenClient := &QwenClient{
		Client: baseClient,
	}

	baseClient.Hooks = qwenClient
	return qwenClient
}

func (qwenClient *QwenClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	qwenClient.APIKey = apiKey

	if len(apiKey) > 8 {
		qwenClient.Log.Infof("🔧 [MCP] Qwen API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		qwenClient.BaseURL = customURL
		qwenClient.Log.Infof("🔧 [MCP] Qwen using custom BaseURL: %s", customURL)
	} else {
		qwenClient.Log.Infof("🔧 [MCP] Qwen using default BaseURL: %s", qwenClient.BaseURL)
	}
	if customModel != "" {
		qwenClient.Model = customModel
		qwenClient.Log.Infof("🔧 [MCP] Qwen using custom Model: %s", customModel)
	} else {
		qwenClient.Log.Infof("🔧 [MCP] Qwen using default Model: %s", qwenClient.Model)
	}
}

func (qwenClient *QwenClient) SetAuthHeader(reqHeaders http.Header) {
	qwenClient.Client.SetAuthHeader(reqHeaders)
}
