package mcp

import "net/http"

// AIClient AI客户端接口
type AIClient interface {
	SetAPIKey(apiKey string, customURL string, customModel string)
	// CallWithMessages 使用 system + user prompt 调用AI API
	CallWithMessages(systemPrompt, userPrompt string) (string, error)

	setAuthHeader(reqHeaders http.Header)
}
