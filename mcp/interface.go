package mcp

// AIClient AI客户端接口
type AIClient interface {
	// SetDeepSeekAPIKey 设置DeepSeek API密钥
	SetDeepSeekAPIKey(apiKey string, customURL string, customModel string)
	// SetQwenAPIKey 设置阿里云Qwen API密钥
	SetQwenAPIKey(apiKey string, customURL string, customModel string)
	// SetCustomAPI 设置自定义OpenAI兼容API
	SetCustomAPI(apiURL, apiKey, modelName string)
	// SetClient 设置完整的AI配置
	SetClient(client Client)
	// CallWithMessages 使用 system + user prompt 调用AI API
	CallWithMessages(systemPrompt, userPrompt string) (string, error)
}
