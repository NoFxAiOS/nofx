# MCP - Model Context Protocol Client

一个灵活、可扩展的 AI 模型客户端库，支持 DeepSeek、Qwen 等多种 AI 提供商。

## ✨ 特性

- 🔌 **多 Provider 支持** - DeepSeek、Qwen、OpenAI 兼容 API
- 🎯 **模板方法模式** - 固定流程，可扩展步骤
- 🏗️ **构建器模式** - 支持多轮对话、Function Calling、精细参数控制
- 📦 **零外部依赖** - 仅使用 Go 标准库
- 🔧 **高度可配置** - 支持 Functional Options 模式
- 🧪 **易于测试** - 支持依赖注入和 Mock
- ⚡ **向前兼容** - 现有代码无需修改
- 📝 **丰富的日志** - 可替换的日志接口

## 🚀 快速开始

### 基础用法

```go
import "nofx/mcp"

// 创建客户端
client := mcp.NewClient(
    mcp.WithDeepSeekConfig("sk-xxx"),
)

// 调用 AI
result, err := client.CallWithMessages("system prompt", "user prompt")
if err != nil {
    log.Fatal(err)
}

fmt.Println(result)
```

### DeepSeek 客户端

```go
client := mcp.NewDeepSeekClientWithOptions(
    mcp.WithAPIKey("sk-xxx"),
    mcp.WithTimeout(60 * time.Second),
)
```

### Qwen 客户端

```go
client := mcp.NewQwenClientWithOptions(
    mcp.WithAPIKey("sk-xxx"),
    mcp.WithMaxTokens(4000),
)
```

### 🏗️ 构建器模式（高级功能）

构建器模式支持多轮对话、精细参数控制、Function Calling 等高级功能。

#### 简单用法

```go
// 使用构建器创建请求
request := mcp.NewRequestBuilder().
    WithSystemPrompt("You are helpful").
    WithUserPrompt("What is Go?").
    WithTemperature(0.8).
    Build()

result, err := client.CallWithRequest(request)
```

#### 多轮对话

```go
// 构建包含历史的多轮对话
request := mcp.NewRequestBuilder().
    AddSystemMessage("You are a trading advisor").
    AddUserMessage("Analyze BTC").
    AddAssistantMessage("BTC is bullish...").
    AddUserMessage("What about entry point?").  // 继续对话
    WithTemperature(0.3).
    Build()

result, err := client.CallWithRequest(request)
```

#### 预设场景

```go
// 代码生成（低温度、精确）
request := mcp.ForCodeGeneration().
    WithUserPrompt("Generate a HTTP server").
    Build()

// 创意写作（高温度、随机）
request := mcp.ForCreativeWriting().
    WithUserPrompt("Write a story").
    Build()

// 聊天（平衡参数）
request := mcp.ForChat().
    WithUserPrompt("Hello").
    Build()
```

#### Function Calling

```go
// 定义工具
weatherParams := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "location": map[string]any{"type": "string"},
    },
}

request := mcp.NewRequestBuilder().
    WithUserPrompt("北京天气怎么样？").
    AddFunction("get_weather", "Get weather", weatherParams).
    WithToolChoice("auto").
    Build()

result, err := client.CallWithRequest(request)
```

## 📖 详细文档

- [构建器模式完整示例](./BUILDER_EXAMPLES.md) - 多轮对话、Function Calling、参数控制
- [构建器模式价值分析](./BUILDER_PATTERN_BENEFITS.md) - 为什么引入构建器模式
- [迁移指南](./MIGRATION_GUIDE.md) - 从旧 API 迁移到新 API
- [Logrus 集成](./LOGRUS_INTEGRATION.md) - 日志框架集成示例
- [代码审查报告](./CODE_REVIEW.md) - 问题分析和修复记录

## 🎛️ 配置选项

### 依赖注入

```go
// 自定义日志器
mcp.WithLogger(customLogger)

// 自定义 HTTP 客户端
mcp.WithHTTPClient(customHTTP)
```

### 超时和重试

```go
mcp.WithTimeout(60 * time.Second)
mcp.WithMaxRetries(5)
mcp.WithRetryWaitBase(3 * time.Second)
```

### AI 参数

```go
mcp.WithMaxTokens(4000)
mcp.WithTemperature(0.7)
```

### Provider 配置

```go
// 快速配置 DeepSeek
mcp.WithDeepSeekConfig("sk-xxx")

// 快速配置 Qwen
mcp.WithQwenConfig("sk-xxx")

// 自定义配置
mcp.WithAPIKey("sk-xxx")
mcp.WithBaseURL("https://api.custom.com")
mcp.WithModel("gpt-4")
```

## 🧪 测试

```go
// 使用 Mock HTTP 客户端
mockHTTP := &MockHTTPClient{
    Response: `{"choices":[{"message":{"content":"test"}}]}`,
}

client := mcp.NewClient(
    mcp.WithHTTPClient(mockHTTP),
    mcp.WithLogger(mcp.NewNoopLogger()), // 禁用日志
)
```

## 🏗️ 架构设计

### 模板方法模式

```
CallWithMessages (固定重试流程)
    ↓
call (固定调用流程)
    ↓
hooks (可重写的步骤)
    ├─ buildMCPRequestBody
    ├─ marshalRequestBody
    ├─ buildUrl
    ├─ setAuthHeader
    ├─ parseMCPResponse
    └─ isRetryableError
```

### 接口分离

```go
// 公开接口（给外部使用）
type AIClient interface {
    SetAPIKey(...)
    SetTimeout(...)
    CallWithMessages(...) (string, error)
}

// 内部钩子接口（供子类重写）
type clientHooks interface {
    buildMCPRequestBody(...) map[string]any
    buildUrl() string
    setAuthHeader(...)
    marshalRequestBody(...) ([]byte, error)
    parseMCPResponse(...) (string, error)
    isRetryableError(...) bool
}
```

## 🔄 向前兼容

所有旧 API 继续工作：

```go
// ✅ 旧代码无需修改
client := mcp.New()
client.SetAPIKey("sk-xxx", "https://api.custom.com", "gpt-4")

dsClient := mcp.NewDeepSeekClient()
dsClient.SetAPIKey("sk-xxx", "", "")
```

## 📦 作为独立模块使用

```go
// go.mod
module github.com/yourorg/yourproject

require github.com/yourorg/mcp v1.0.0
```

```go
// main.go
import "github.com/yourorg/mcp"

client := mcp.NewClient(
    mcp.WithDeepSeekConfig("sk-xxx"),
)
```

## 🤝 扩展自定义 Provider

```go
type CustomProvider struct {
    *mcp.Client
}

// 重写特定钩子
func (c *CustomProvider) buildUrl() string {
    return c.BaseURL + "/custom/endpoint"
}

func (c *CustomProvider) setAuthHeader(headers http.Header) {
    headers.Set("X-Custom-Auth", c.APIKey)
}
```

## 📝 日志器适配示例

### Zap 日志器

```go
type ZapLogger struct {
    logger *zap.Logger
}

func (l *ZapLogger) Infof(format string, args ...any) {
    l.logger.Sugar().Infof(format, args...)
}

func (l *ZapLogger) Debugf(format string, args ...any) {
    l.logger.Sugar().Debugf(format, args...)
}

// 使用
client := mcp.NewClient(
    mcp.WithLogger(&ZapLogger{zapLogger}),
)
```

### Logrus 日志器

```go
type LogrusLogger struct {
    logger *logrus.Logger
}

func (l *LogrusLogger) Infof(format string, args ...any) {
    l.logger.Infof(format, args...)
}

func (l *LogrusLogger) Debugf(format string, args ...any) {
    l.logger.Debugf(format, args...)
}
```

## 🎯 使用场景

### 开发环境

```go
devClient := mcp.NewClient(
    mcp.WithDeepSeekConfig("sk-xxx"),
    mcp.WithLogger(&customLogger{}), // 详细日志
)
```

### 生产环境

```go
prodClient := mcp.NewClient(
    mcp.WithDeepSeekConfig("sk-xxx"),
    mcp.WithLogger(&zapLogger{}),     // 结构化日志
    mcp.WithTimeout(30*time.Second),  // 超时保护
    mcp.WithMaxRetries(3),            // 重试保护
)
```

### 测试环境

```go
testClient := mcp.NewClient(
    mcp.WithHTTPClient(mockHTTP),
    mcp.WithLogger(mcp.NewNoopLogger()),
)
```

## 📊 性能特性

- ✅ HTTP 连接复用
- ✅ 智能重试机制
- ✅ 可配置超时
- ✅ 零分配日志（使用 NoopLogger）

## 🛡️ 安全性

- ✅ API Key 部分脱敏日志
- ✅ HTTPS 默认启用
- ✅ 支持自定义 TLS 配置
- ✅ 请求超时保护

## 📈 版本兼容性

- Go 1.18+
- 向前兼容保证
- 语义化版本管理

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License

## 🔗 相关链接

- [DeepSeek API 文档](https://platform.deepseek.com/docs)
- [Qwen API 文档](https://help.aliyun.com/zh/dashscope/)
- [OpenAI API 文档](https://platform.openai.com/docs)
