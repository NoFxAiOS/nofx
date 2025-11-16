package mcp

import (
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

// NewQwenClient 创建 Qwen 客户端（向前兼容）
//
// Deprecated: 推荐使用 NewQwenClientWithOptions 以获得更好的灵活性
func NewQwenClient() AIClient {
	return NewQwenClientWithOptions()
}

// NewQwenClientWithOptions 创建 Qwen 客户端（支持选项模式）
//
// 使用示例：
//   // 基础用法
//   client := mcp.NewQwenClientWithOptions()
//
//   // 自定义配置
//   client := mcp.NewQwenClientWithOptions(
//       mcp.WithAPIKey("sk-xxx"),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewQwenClientWithOptions(opts ...ClientOption) AIClient {
	// 1. 创建 Qwen 预设选项
	qwenOpts := []ClientOption{
		WithProvider(ProviderQwen),
		WithModel(DefaultQwenModel),
		WithBaseURL(DefaultQwenBaseURL),
	}

	// 2. 合并用户选项（用户选项优先级更高）
	allOpts := append(qwenOpts, opts...)

	// 3. 创建基础客户端
	baseClient := NewClient(allOpts...).(*Client)

	// 4. 创建 Qwen 客户端
	qwenClient := &QwenClient{
		Client: baseClient,
	}

	// 5. 设置 hooks 指向 QwenClient（实现动态分派）
	baseClient.hooks = qwenClient

	return qwenClient
}

func (qwenClient *QwenClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	qwenClient.APIKey = apiKey

	if len(apiKey) > 8 {
		qwenClient.logger.Infof("🔧 [MCP] Qwen API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		qwenClient.BaseURL = customURL
		qwenClient.logger.Infof("🔧 [MCP] Qwen 使用自定义 BaseURL: %s", customURL)
	} else {
		qwenClient.logger.Infof("🔧 [MCP] Qwen 使用默认 BaseURL: %s", qwenClient.BaseURL)
	}
	if customModel != "" {
		qwenClient.Model = customModel
		qwenClient.logger.Infof("🔧 [MCP] Qwen 使用自定义 Model: %s", customModel)
	} else {
		qwenClient.logger.Infof("🔧 [MCP] Qwen 使用默认 Model: %s", qwenClient.Model)
	}
}

func (qwenClient *QwenClient) setAuthHeader(reqHeaders http.Header) {
	qwenClient.Client.setAuthHeader(reqHeaders)
}
