package mcp

import (
	"net/http"
	"time"
)

// AIClient AI客户端公开接口（给外部使用）
type AIClient interface {
	SetAPIKey(apiKey string, customURL string, customModel string)
	SetTimeout(timeout time.Duration)
	CallWithMessages(systemPrompt, userPrompt string) (string, error)
	CallWithRequest(req *Request) (string, error) // 构建器模式 API（支持高级功能）
}

// clientHooks 内部钩子接口（用于子类重写特定步骤）
// 这些方法只在包内部使用，实现动态分派
type clientHooks interface {
	// 可被子类重写的钩子方法

	call(systemPrompt, userPrompt string) (string, error)

	buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any
	buildUrl() string
	buildRequest(url string, jsonData []byte) (*http.Request, error)
	setAuthHeader(reqHeaders http.Header)
	marshalRequestBody(requestBody map[string]any) ([]byte, error)
	parseMCPResponse(body []byte) (string, error)
	isRetryableError(err error) bool
}
