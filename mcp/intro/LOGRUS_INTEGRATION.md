# Logrus 集成指南

本文档展示如何将 MCP 模块与 Logrus 日志库集成。

## 📦 安装 Logrus

```bash
go get github.com/sirupsen/logrus
```

## 🔧 集成步骤

### 1. 创建 Logrus 适配器

创建一个实现 `mcp.Logger` 接口的适配器：

```go
package main

import (
    "github.com/sirupsen/logrus"
    "nofx/mcp"
)

// LogrusLogger Logrus 日志适配器
type LogrusLogger struct {
    logger *logrus.Logger
}

// NewLogrusLogger 创建 Logrus 日志适配器
func NewLogrusLogger(logger *logrus.Logger) *LogrusLogger {
    return &LogrusLogger{logger: logger}
}

// Debugf 实现 Debug 日志
func (l *LogrusLogger) Debugf(format string, args ...any) {
    l.logger.Debugf(format, args...)
}

// Infof 实现 Info 日志
func (l *LogrusLogger) Infof(format string, args ...any) {
    l.logger.Infof(format, args...)
}

// Warnf 实现 Warn 日志
func (l *LogrusLogger) Warnf(format string, args ...any) {
    l.logger.Warnf(format, args...)
}

// Errorf 实现 Error 日志
func (l *LogrusLogger) Errorf(format string, args ...any) {
    l.logger.Errorf(format, args...)
}
```

### 2. 使用 Logrus Logger

```go
package main

import (
    "github.com/sirupsen/logrus"
    "nofx/mcp"
)

func main() {
    // 1. 创建 Logrus logger
    logger := logrus.New()

    // 2. 配置 Logrus
    logger.SetLevel(logrus.DebugLevel)
    logger.SetFormatter(&logrus.JSONFormatter{})

    // 3. 创建适配器
    logrusAdapter := NewLogrusLogger(logger)

    // 4. 使用 MCP 客户端
    client := mcp.NewClient(
        mcp.WithDeepSeekConfig("sk-xxx"),
        mcp.WithLogger(logrusAdapter), // 注入 Logrus 日志器
    )

    // 5. 调用 AI
    result, err := client.CallWithMessages("system", "user")
    if err != nil {
        logger.Errorf("AI 调用失败: %v", err)
        return
    }

    logger.Infof("AI 响应: %s", result)
}
```

## 🎨 高级配置

### JSON 格式输出

```go
logger := logrus.New()
logger.SetFormatter(&logrus.JSONFormatter{
    TimestampFormat: "2006-01-02 15:04:05",
    PrettyPrint:     true,
})
```

输出示例：
```json
{
  "level": "info",
  "msg": "📡 [Provider: deepseek, Model: deepseek-chat] Request AI Server: BaseURL: https://api.deepseek.com/v1",
  "time": "2024-01-15 10:30:45"
}
```

### 添加固定字段

```go
logger := logrus.New()
logger.WithFields(logrus.Fields{
    "service": "trading-bot",
    "version": "1.0.0",
})
```

### 不同环境配置

```go
func createLogger(env string) *logrus.Logger {
    logger := logrus.New()

    switch env {
    case "production":
        // 生产环境：JSON 格式，只记录 Info 以上
        logger.SetLevel(logrus.InfoLevel)
        logger.SetFormatter(&logrus.JSONFormatter{})

    case "development":
        // 开发环境：文本格式，记录所有级别
        logger.SetLevel(logrus.DebugLevel)
        logger.SetFormatter(&logrus.TextFormatter{
            FullTimestamp: true,
        })

    case "test":
        // 测试环境：静默模式
        logger.SetLevel(logrus.FatalLevel)
    }

    return logger
}

// 使用
logger := createLogger("production")
mcpClient := mcp.NewClient(
    mcp.WithLogger(NewLogrusLogger(logger)),
)
```

## 📝 完整示例

```go
package main

import (
    "os"

    "github.com/sirupsen/logrus"
    "nofx/mcp"
)

// LogrusLogger Logrus 适配器
type LogrusLogger struct {
    logger *logrus.Logger
}

func NewLogrusLogger(logger *logrus.Logger) *LogrusLogger {
    return &LogrusLogger{logger: logger}
}

func (l *LogrusLogger) Debugf(format string, args ...any) {
    l.logger.Debugf(format, args...)
}

func (l *LogrusLogger) Infof(format string, args ...any) {
    l.logger.Infof(format, args...)
}

func (l *LogrusLogger) Warnf(format string, args ...any) {
    l.logger.Warnf(format, args...)
}

func (l *LogrusLogger) Errorf(format string, args ...any) {
    l.logger.Errorf(format, args...)
}

func main() {
    // 创建 Logrus logger
    logger := logrus.New()
    logger.SetLevel(logrus.DebugLevel)
    logger.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
        ForceColors:   true,
    })
    logger.SetOutput(os.Stdout)

    // 创建 MCP 客户端
    client := mcp.NewDeepSeekClientWithOptions(
        mcp.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
        mcp.WithLogger(NewLogrusLogger(logger)),
        mcp.WithMaxRetries(5),
    )

    // 调用 AI
    logger.Info("开始调用 AI...")
    result, err := client.CallWithMessages(
        "你是一个专业的量化交易顾问",
        "分析 BTC 当前走势",
    )

    if err != nil {
        logger.WithError(err).Error("AI 调用失败")
        return
    }

    logger.WithField("result", result).Info("AI 调用成功")
}
```

## 🔍 输出示例

### 开发环境（Text 格式）

```
INFO[2024-01-15 10:30:45] 开始调用 AI...
INFO[2024-01-15 10:30:45] 📡 [Provider: deepseek, Model: deepseek-chat] Request AI Server: BaseURL: https://api.deepseek.com/v1
DEBUG[2024-01-15 10:30:45] [Provider: deepseek, Model: deepseek-chat] UseFullURL: false
DEBUG[2024-01-15 10:30:45] [Provider: deepseek, Model: deepseek-chat]   API Key: sk-x...xxx
INFO[2024-01-15 10:30:45] 📡 [MCP Provider: deepseek, Model: deepseek-chat] 请求 URL: https://api.deepseek.com/v1/chat/completions
INFO[2024-01-15 10:30:46] AI 调用成功 result="[AI 响应内容]"
```

### 生产环境（JSON 格式）

```json
{"level":"info","msg":"开始调用 AI...","time":"2024-01-15T10:30:45+08:00"}
{"level":"info","msg":"📡 [Provider: deepseek, Model: deepseek-chat] Request AI Server: BaseURL: https://api.deepseek.com/v1","time":"2024-01-15T10:30:45+08:00"}
{"level":"info","msg":"AI 调用成功","result":"[AI 响应内容]","time":"2024-01-15T10:30:46+08:00"}
```

## 🎯 最佳实践

1. **生产环境使用 JSON 格式**，便于日志收集和分析
2. **开发环境使用 Text 格式**，便于阅读
3. **测试环境关闭日志**，提高测试速度
4. **添加请求 ID**，方便追踪请求链路
5. **记录错误堆栈**，便于问题排查

## 📊 性能优化

Logrus 在高并发场景下可能有性能瓶颈，推荐使用 [Zap](https://github.com/uber-go/zap) 获得更好的性能。

MCP 模块也支持 Zap，集成方式类似。

## 🔗 相关资源

- [Logrus 官方文档](https://github.com/sirupsen/logrus)
- [Zap 集成示例](./ZAP_INTEGRATION.md)
- [MCP README](./README.md)
