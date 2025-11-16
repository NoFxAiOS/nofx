package mcp_test

import (
	"fmt"
	"net/http"
	"time"

	"nofx/mcp"
)

// ============================================================
// 示例 1: 基础用法（向前兼容）
// ============================================================

func Example_backward_compatible() {
	// ✅ 旧代码继续工作，无需修改
	client := mcp.New()
	client.SetAPIKey("sk-xxx", "https://api.custom.com", "gpt-4")

	// 使用
	result, _ := client.CallWithMessages("system prompt", "user prompt")
	fmt.Println(result)
}

func Example_deepseek_backward_compatible() {
	// ✅ DeepSeek 旧代码继续工作
	client := mcp.NewDeepSeekClient()
	client.SetAPIKey("sk-xxx", "", "")

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

// ============================================================
// 示例 2: 新的推荐用法（选项模式）
// ============================================================

func Example_new_client_basic() {
	// 使用默认配置
	client := mcp.NewClient()

	// 使用 DeepSeek
	client = mcp.NewClient(
		mcp.WithDeepSeekConfig("sk-xxx"),
	)

	// 使用 Qwen
	client = mcp.NewClient(
		mcp.WithQwenConfig("sk-xxx"),
	)

	_ = client
}

func Example_new_client_with_options() {
	// 组合多个选项
	client := mcp.NewClient(
		mcp.WithDeepSeekConfig("sk-xxx"),
		mcp.WithTimeout(60*time.Second),
		mcp.WithMaxRetries(5),
		mcp.WithMaxTokens(4000),
		mcp.WithTemperature(0.7),
	)

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

// ============================================================
// 示例 3: 自定义日志器
// ============================================================

// CustomLogger 自定义日志器示例
type CustomLogger struct{}

func (l *CustomLogger) Debugf(format string, args ...any) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

func (l *CustomLogger) Infof(format string, args ...any) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (l *CustomLogger) Warnf(format string, args ...any) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (l *CustomLogger) Errorf(format string, args ...any) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

func Example_custom_logger() {
	// 使用自定义日志器
	customLogger := &CustomLogger{}

	client := mcp.NewClient(
		mcp.WithDeepSeekConfig("sk-xxx"),
		mcp.WithLogger(customLogger),
	)

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

func Example_no_logger_for_testing() {
	// 测试时禁用日志
	client := mcp.NewClient(
		mcp.WithLogger(mcp.NewNoopLogger()),
	)

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

// ============================================================
// 示例 4: 自定义 HTTP 客户端
// ============================================================

func Example_custom_http_client() {
	// 自定义 HTTP 客户端（添加代理、TLS等）
	customHTTP := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			// 自定义 TLS、连接池等
		},
	}

	client := mcp.NewClient(
		mcp.WithDeepSeekConfig("sk-xxx"),
		mcp.WithHTTPClient(customHTTP),
	)

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

// ============================================================
// 示例 5: DeepSeek 客户端（新 API）
// ============================================================

func Example_deepseek_new_api() {
	// 基础用法
	client := mcp.NewDeepSeekClientWithOptions(
		mcp.WithAPIKey("sk-xxx"),
	)

	// 高级用法
	client = mcp.NewDeepSeekClientWithOptions(
		mcp.WithAPIKey("sk-xxx"),
		mcp.WithLogger(&CustomLogger{}),
		mcp.WithTimeout(90*time.Second),
		mcp.WithMaxTokens(8000),
	)

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

// ============================================================
// 示例 6: Qwen 客户端（新 API）
// ============================================================

func Example_qwen_new_api() {
	// 基础用法
	client := mcp.NewQwenClientWithOptions(
		mcp.WithAPIKey("sk-xxx"),
	)

	// 高级用法
	client = mcp.NewQwenClientWithOptions(
		mcp.WithAPIKey("sk-xxx"),
		mcp.WithLogger(&CustomLogger{}),
		mcp.WithTimeout(90*time.Second),
	)

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

// ============================================================
// 示例 7: 在 trader/auto_trader.go 中的迁移示例
// ============================================================

func Example_trader_migration() {
	// === 旧代码（继续工作）===
	oldStyleClient := func(apiKey, customURL, customModel string) mcp.AIClient {
		client := mcp.NewDeepSeekClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client
	}

	// === 新代码（推荐）===
	newStyleClient := func(apiKey, customURL, customModel string) mcp.AIClient {
		opts := []mcp.ClientOption{
			mcp.WithAPIKey(apiKey),
		}

		if customURL != "" {
			opts = append(opts, mcp.WithBaseURL(customURL))
		}

		if customModel != "" {
			opts = append(opts, mcp.WithModel(customModel))
		}

		return mcp.NewDeepSeekClientWithOptions(opts...)
	}

	// 两种方式都能工作
	_ = oldStyleClient("sk-xxx", "", "")
	_ = newStyleClient("sk-xxx", "", "")
}

// ============================================================
// 示例 8: 测试场景
// ============================================================

// MockHTTPClient Mock HTTP 客户端
type MockHTTPClient struct {
	Response string
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// 返回预设的响应
	return &http.Response{
		StatusCode: 200,
		Body:       nil, // 实际测试中需要实现
	}, nil
}

func Example_testing_with_mock() {
	// 测试时使用 Mock
	// mockHTTP := &MockHTTPClient{
	// 	Response: `{"choices":[{"message":{"content":"test response"}}]}`,
	// }

	client := mcp.NewClient(
		// mcp.WithHTTPClient(mockHTTP), // 实际测试中使用 mockHTTP
		mcp.WithLogger(mcp.NewNoopLogger()), // 禁用日志
	)

	result, _ := client.CallWithMessages("system", "user")
	fmt.Println(result)
}

// ============================================================
// 示例 9: 环境特定配置
// ============================================================

func Example_environment_specific() {
	// 开发环境：详细日志
	devClient := mcp.NewClient(
		mcp.WithDeepSeekConfig("sk-xxx"),
		mcp.WithLogger(&CustomLogger{}), // 详细日志
	)

	// 生产环境：结构化日志 + 超时保护
	prodClient := mcp.NewClient(
		mcp.WithDeepSeekConfig("sk-xxx"),
		// mcp.WithLogger(&ZapLogger{}), // 生产级日志
		mcp.WithTimeout(30*time.Second),
		mcp.WithMaxRetries(3),
	)

	_, _ = devClient.CallWithMessages("system", "user")
	_, _ = prodClient.CallWithMessages("system", "user")
}

// ============================================================
// 示例 10: 完整实战示例
// ============================================================

func Example_real_world_usage() {
	// 创建带有完整配置的客户端
	client := mcp.NewDeepSeekClientWithOptions(
		mcp.WithAPIKey("sk-xxxxxxxxxx"),
		mcp.WithTimeout(60*time.Second),
		mcp.WithMaxRetries(5),
		mcp.WithMaxTokens(4000),
		mcp.WithTemperature(0.5),
		mcp.WithLogger(&CustomLogger{}),
	)

	// 使用客户端
	systemPrompt := "你是一个专业的量化交易顾问"
	userPrompt := "分析 BTC 当前走势"

	result, err := client.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("AI 响应: %s\n", result)
}
