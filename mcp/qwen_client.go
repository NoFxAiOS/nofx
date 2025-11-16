package mcp

import (
	"log"
	"net/http"
)

const (
	ProviderQwen       = "qwen"
	DefaultQwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	DefaultQwenModel   = "qwen3-max"
)

type QwenClient struct {
	*Client
}

func NewQwenClient() AIClient {
	client := New().(*Client)
	client.Provider = ProviderQwen
	client.Model = DefaultQwenModel
	client.BaseURL = DefaultQwenBaseURL
	return &QwenClient{
		Client: client,
	}
}

func (qwenClient *QwenClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	if qwenClient.Client == nil {
		qwenClient.Client = New().(*Client)
	}
	qwenClient.Client.APIKey = apiKey

	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] Qwen API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		qwenClient.Client.BaseURL = customURL
		log.Printf("🔧 [MCP] Qwen 使用自定义 BaseURL: %s", customURL)
	} else {
		log.Printf("🔧 [MCP] Qwen 使用默认 BaseURL: %s", qwenClient.Client.BaseURL)
	}
	if customModel != "" {
		qwenClient.Client.Model = customModel
		log.Printf("🔧 [MCP] Qwen 使用自定义 Model: %s", customModel)
	} else {
		log.Printf("🔧 [MCP] Qwen 使用默认 Model: %s", qwenClient.Client.Model)
	}
}

func (qwenClient *QwenClient) setAuthHeader(reqHeaders http.Header) {
	qwenClient.Client.setAuthHeader(reqHeaders)
}
