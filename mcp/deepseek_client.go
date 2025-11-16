package mcp

import (
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

// NewDeepSeekClient 创建 DeepSeek 客户端（向前兼容）
//
// Deprecated: 推荐使用 NewDeepSeekClientWithOptions 以获得更好的灵活性
func NewDeepSeekClient() AIClient {
	return NewDeepSeekClientWithOptions()
}

// NewDeepSeekClientWithOptions 创建 DeepSeek 客户端（支持选项模式）
//
// 使用示例：
//   // 基础用法
//   client := mcp.NewDeepSeekClientWithOptions()
//
//   // 自定义配置
//   client := mcp.NewDeepSeekClientWithOptions(
//       mcp.WithAPIKey("sk-xxx"),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewDeepSeekClientWithOptions(opts ...ClientOption) AIClient {
	// 1. 创建 DeepSeek 预设选项
	deepseekOpts := []ClientOption{
		WithProvider(ProviderDeepSeek),
		WithModel(DefaultDeepSeekModel),
		WithBaseURL(DefaultDeepSeekBaseURL),
	}

	// 2. 合并用户选项（用户选项优先级更高）
	allOpts := append(deepseekOpts, opts...)

	// 3. 创建基础客户端
	baseClient := NewClient(allOpts...).(*Client)

	// 4. 创建 DeepSeek 客户端
	dsClient := &DeepSeekClient{
		Client: baseClient,
	}

	// 5. 设置 hooks 指向 DeepSeekClient（实现动态分派）
	baseClient.hooks = dsClient

	return dsClient
}

func (dsClient *DeepSeekClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	dsClient.APIKey = apiKey

	if len(apiKey) > 8 {
		dsClient.logger.Infof("🔧 [MCP] DeepSeek API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		dsClient.BaseURL = customURL
		dsClient.logger.Infof("🔧 [MCP] DeepSeek 使用自定义 BaseURL: %s", customURL)
	} else {
		dsClient.logger.Infof("🔧 [MCP] DeepSeek 使用默认 BaseURL: %s", dsClient.BaseURL)
	}
	if customModel != "" {
		dsClient.Model = customModel
		dsClient.logger.Infof("🔧 [MCP] DeepSeek 使用自定义 Model: %s", customModel)
	} else {
		dsClient.logger.Infof("🔧 [MCP] DeepSeek 使用默认 Model: %s", dsClient.Model)
	}
}

func (dsClient *DeepSeekClient) setAuthHeader(reqHeaders http.Header) {
	dsClient.Client.setAuthHeader(reqHeaders)
}
