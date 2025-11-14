package mcp

import (
	"log"
	"net/http"
)

const (
	ProviderDeepSeek       = "deepseek"
	DefaultDeepSeekBaseURL = "https://api.deepseek.com/v1"
	DefaultDeepSeekModel   = "deepseek-chat"
)

type DeepSeekClient struct {
	*Client
}

func NewDeepSeekClient() AIClient {
	client := New().(*Client)
	client.Provider = ProviderDeepSeek
	client.Model = DefaultDeepSeekModel
	client.BaseURL = DefaultDeepSeekBaseURL
	return &DeepSeekClient{
		Client: client,
	}
}

func (dsClient *DeepSeekClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	if dsClient.Client == nil {
		dsClient.Client = New().(*Client)
	}
	dsClient.Client.APIKey = apiKey

	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] DeepSeek API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		dsClient.Client.BaseURL = customURL
		log.Printf("🔧 [MCP] DeepSeek 使用自定义 BaseURL: %s", customURL)
	} else {
		log.Printf("🔧 [MCP] DeepSeek 使用默认 BaseURL: %s", dsClient.Client.BaseURL)
	}
	if customModel != "" {
		dsClient.Client.Model = customModel
		log.Printf("🔧 [MCP] DeepSeek 使用自定义 Model: %s", customModel)
	} else {
		log.Printf("🔧 [MCP] DeepSeek 使用默认 Model: %s", dsClient.Client.Model)
	}
}

func (dsClient *DeepSeekClient) setAuthHeader(reqHeaders http.Header) {
	dsClient.Client.setAuthHeader(reqHeaders)
}
