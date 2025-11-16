# MCP 模块重构迁移指南

## 📋 重构概览

本次重构采用**渐进式、向前兼容**的设计，现有代码**无需修改**即可继续使用，同时提供了更强大的新 API。

### 重构目标

- ✅ **100% 向前兼容** - 所有现有 API 继续工作
- ✅ **模块独立** - 可作为独立 Go module 发布
- ✅ **依赖可替换** - 日志、HTTP 客户端都可自定义
- ✅ **易于测试** - 支持依赖注入和 mock
- ✅ **配置灵活** - 支持选项模式 (Functional Options)

---

## 🔄 向前兼容保证

### ✅ 所有现有代码继续工作

```go
// ✅ 这些代码无需修改，继续正常工作
mcpClient := mcp.New()
mcpClient.SetAPIKey(apiKey, url, model)

// ✅ 这些也继续工作
dsClient := mcp.NewDeepSeekClient()
qwenClient := mcp.NewQwenClient()
```

**重要**：虽然标记为 `Deprecated`，但这些函数会一直保留，不会被删除。

---

## 🆕 新特性使用指南

### 1. 基础用法（推荐）

```go
// 新的推荐用法
client := mcp.NewClient(
    mcp.WithDeepSeekConfig("sk-xxx"),
    mcp.WithTimeout(60 * time.Second),
)
```

### 2. 自定义日志

```go
// 使用自定义日志器（如 zap, logrus）
type MyLogger struct {
    zapLogger *zap.Logger
}

func (l *MyLogger) Info(msg string, args ...any) {
    l.zapLogger.Sugar().Infof(msg, args...)
}

// 注入自定义日志器
client := mcp.NewClient(
    mcp.WithLogger(&MyLogger{zapLogger}),
)
```

### 3. 自定义 HTTP 客户端

```go
// 添加代理、追踪、自定义 TLS 等
customHTTP := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        Proxy: http.ProxyFromEnvironment,
        TLSClientConfig: &tls.Config{/* ... */},
    },
}

client := mcp.NewClient(
    mcp.WithHTTPClient(customHTTP),
)
```

### 4. 测试场景

```go
func TestMyCode(t *testing.T) {
    // Mock HTTP 客户端
    mockHTTP := &MockHTTPClient{
        // 返回预设的响应
    }

    // 禁用日志
    client := mcp.NewClient(
        mcp.WithHTTPClient(mockHTTP),
        mcp.WithLogger(mcp.NewNoopLogger()),
    )

    // 测试...
}
```

### 5. 组合多个选项

```go
client := mcp.NewDeepSeekClientWithOptions(
    mcp.WithAPIKey("sk-xxx"),
    mcp.WithLogger(customLogger),
    mcp.WithTimeout(60 * time.Second),
    mcp.WithMaxRetries(5),
    mcp.WithMaxTokens(4000),
)
```

---

## 📊 API 对比表

### 构造函数对比

| 旧 API (仍可用) | 新 API (推荐) | 说明 |
|----------------|--------------|------|
| `mcp.New()` | `mcp.NewClient(opts...)` | 支持选项模式 |
| `mcp.NewDeepSeekClient()` | `mcp.NewDeepSeekClientWithOptions(opts...)` | 支持自定义配置 |
| `mcp.NewQwenClient()` | `mcp.NewQwenClientWithOptions(opts...)` | 支持自定义配置 |

### 配置选项

| 选项函数 | 说明 | 使用示例 |
|---------|------|---------|
| `WithLogger(logger)` | 自定义日志器 | `WithLogger(zapLogger)` |
| `WithHTTPClient(client)` | 自定义 HTTP 客户端 | `WithHTTPClient(customHTTP)` |
| `WithTimeout(duration)` | 设置超时 | `WithTimeout(60*time.Second)` |
| `WithMaxRetries(n)` | 设置重试次数 | `WithMaxRetries(5)` |
| `WithMaxTokens(n)` | 设置最大 token | `WithMaxTokens(4000)` |
| `WithTemperature(t)` | 设置温度参数 | `WithTemperature(0.7)` |
| `WithAPIKey(key)` | 设置 API Key | `WithAPIKey("sk-xxx")` |
| `WithDeepSeekConfig(key)` | 快速配置 DeepSeek | `WithDeepSeekConfig("sk-xxx")` |
| `WithQwenConfig(key)` | 快速配置 Qwen | `WithQwenConfig("sk-xxx")` |

---

## 🔧 迁移步骤

### Phase 1: 继续使用现有代码（无需改动）

```go
// trader/auto_trader.go 中的现有代码
mcpClient := mcp.New()

if config.AIModel == "qwen" {
    mcpClient = mcp.NewQwenClient()
    mcpClient.SetAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
} else {
    mcpClient = mcp.NewDeepSeekClient()
    mcpClient.SetAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
}

// ✅ 继续工作，无需修改
```

### Phase 2: 可选升级到新 API（推荐）

```go
// 升级后的代码（可选）
var mcpClient mcp.AIClient

if config.AIModel == "qwen" {
    mcpClient = mcp.NewQwenClientWithOptions(
        mcp.WithAPIKey(config.QwenKey),
        mcp.WithBaseURL(config.CustomAPIURL),
        mcp.WithModel(config.CustomModelName),
    )
} else {
    mcpClient = mcp.NewDeepSeekClientWithOptions(
        mcp.WithAPIKey(config.DeepSeekKey),
        mcp.WithBaseURL(config.CustomAPIURL),
        mcp.WithModel(config.CustomModelName),
    )
}
```

### Phase 3: 添加自定义配置（高级）

```go
// 添加自定义日志
customLogger := &MyZapLogger{zap.NewProduction()}

mcpClient := mcp.NewDeepSeekClientWithOptions(
    mcp.WithAPIKey(config.DeepSeekKey),
    mcp.WithLogger(customLogger),        // 自定义日志
    mcp.WithTimeout(90 * time.Second),   // 自定义超时
    mcp.WithMaxRetries(5),               // 自定义重试次数
)
```

---

## 🎯 实际使用场景

### 场景 1: 开发环境详细日志

```go
// 开发环境：使用详细日志
devClient := mcp.NewClient(
    mcp.WithDeepSeekConfig(apiKey),
    mcp.WithLogger(&defaultLogger{}), // 详细日志
)
```

### 场景 2: 生产环境结构化日志

```go
// 生产环境：使用 zap 结构化日志
zapLogger, _ := zap.NewProduction()
prodClient := mcp.NewClient(
    mcp.WithDeepSeekConfig(apiKey),
    mcp.WithLogger(&ZapLogger{zapLogger}),
)
```

### 场景 3: 测试环境 Mock

```go
// 测试环境：Mock HTTP 响应
mockHTTP := &MockHTTPClient{
    Response: `{"choices":[{"message":{"content":"test"}}]}`,
}

testClient := mcp.NewClient(
    mcp.WithHTTPClient(mockHTTP),
    mcp.WithLogger(mcp.NewNoopLogger()), // 禁用日志
)
```

### 场景 4: 需要代理的网络环境

```go
// 使用代理
proxyURL, _ := url.Parse("http://proxy.company.com:8080")
proxyClient := &http.Client{
    Transport: &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    },
}

client := mcp.NewClient(
    mcp.WithDeepSeekConfig(apiKey),
    mcp.WithHTTPClient(proxyClient),
)
```

---

## 📦 作为独立模块发布

重构后，mcp 模块可以独立发布：

### go.mod
```go
module github.com/yourorg/mcp

go 1.21

// 无外部依赖！
```

### 使用方
```go
import "github.com/yourorg/mcp"

client := mcp.NewClient(
    mcp.WithDeepSeekConfig("sk-xxx"),
)
```

---

## 🧪 测试支持

### Mock 示例

```go
package mypackage_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "nofx/mcp"
)

type MockHTTPClient struct {
    Response string
    Error    error
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
    if m.Error != nil {
        return nil, m.Error
    }

    return &http.Response{
        StatusCode: 200,
        Body:       io.NopCloser(strings.NewReader(m.Response)),
    }, nil
}

func TestAIIntegration(t *testing.T) {
    // Arrange
    mockHTTP := &MockHTTPClient{
        Response: `{"choices":[{"message":{"content":"success"}}]}`,
    }

    client := mcp.NewClient(
        mcp.WithHTTPClient(mockHTTP),
        mcp.WithLogger(mcp.NewNoopLogger()),
    )

    // Act
    result, err := client.CallWithMessages("system", "user")

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "success", result)
}
```

---

## ⚠️ 注意事项

1. **向前兼容性**
   - 所有 `Deprecated` 的 API 会永久保留
   - 现有代码可以继续使用，不会被破坏

2. **渐进式迁移**
   - 不需要一次性迁移所有代码
   - 可以逐步采用新 API

3. **配置优先级**
   - 用户传入的选项优先级最高
   - 环境变量次之
   - 默认配置最低

4. **日志器接口**
   - 可以适配任何日志库（zap, logrus, etc.）
   - 测试时可以使用 `NewNoopLogger()` 禁用日志

---

## 📚 进一步阅读

- [选项模式详解](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
- [依赖注入最佳实践](https://go.dev/blog/wire)
- [Go 接口设计原则](https://go.dev/blog/laws-of-reflection)

---

## 🤝 贡献

欢迎提交 issue 和 PR！

如有问题，请联系：[your-email@example.com]
