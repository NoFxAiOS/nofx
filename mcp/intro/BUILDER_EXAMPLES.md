# RequestBuilder 使用示例

## 📋 目录
1. [基础用法](#基础用法)
2. [多轮对话](#多轮对话)
3. [参数精细控制](#参数精细控制)
4. [Function Calling](#function-calling)
5. [预设场景](#预设场景)
6. [完整示例](#完整示例)

---

## 基础用法

### 简单对话

```go
package main

import (
    "fmt"
    "nofx/mcp"
)

func main() {
    // 创建客户端
    client := mcp.NewDeepSeekClientWithOptions(
        mcp.WithAPIKey("sk-xxx"),
    )

    // 使用构建器创建请求
    request := mcp.NewRequestBuilder().
        WithSystemPrompt("You are a helpful assistant").
        WithUserPrompt("What is Go programming language?").
        Build()

    // 调用 API
    result, err := client.CallWithRequest(request)
    if err != nil {
        panic(err)
    }

    fmt.Println(result)
}
```

### 与传统方式对比

```go
// 传统方式（仍然可用）
result, err := client.CallWithMessages(
    "You are a helpful assistant",
    "What is Go?",
)

// 构建器方式（新API，功能更强大）
request := mcp.NewRequestBuilder().
    WithSystemPrompt("You are a helpful assistant").
    WithUserPrompt("What is Go?").
    Build()
result, err := client.CallWithRequest(request)
```

---

## 多轮对话

### 带上下文的对话

```go
// 构建包含历史的多轮对话
request := mcp.NewRequestBuilder().
    AddSystemMessage("You are a trading advisor").
    AddUserMessage("Analyze BTC price").
    AddAssistantMessage("BTC is currently in an upward trend...").
    AddUserMessage("What's the best entry point?").  // 继续对话
    WithTemperature(0.3).  // 低温度，更精确
    Build()

result, err := client.CallWithRequest(request)
```

### 从历史记录构建

```go
// 假设你有保存的对话历史
history := []mcp.Message{
    mcp.NewUserMessage("Hello"),
    mcp.NewAssistantMessage("Hi! How can I help?"),
    mcp.NewUserMessage("What's the weather?"),
    mcp.NewAssistantMessage("It's sunny today"),
}

// 继续对话
request := mcp.NewRequestBuilder().
    AddSystemMessage("You are helpful").
    AddConversationHistory(history).  // 添加历史
    AddUserMessage("What about tomorrow?").  // 新问题
    Build()

result, err := client.CallWithRequest(request)
```

---

## 参数精细控制

### 代码生成（低温度、精确）

```go
request := mcp.NewRequestBuilder().
    WithSystemPrompt("You are a Go expert").
    WithUserPrompt("Generate a HTTP server").
    WithTemperature(0.2).        // 低温度 = 更确定
    WithTopP(0.1).               // 低 top_p = 更聚焦
    WithMaxTokens(2000).
    AddStopSequence("```").      // 遇到代码块结束符停止
    Build()

code, err := client.CallWithRequest(request)
```

### 创意写作（高温度、随机）

```go
request := mcp.NewRequestBuilder().
    WithSystemPrompt("You are a creative writer").
    WithUserPrompt("Write a sci-fi story about AI").
    WithTemperature(1.2).        // 高温度 = 更创意
    WithTopP(0.95).              // 高 top_p = 更多样
    WithPresencePenalty(0.6).    // 避免重复主题
    WithFrequencyPenalty(0.5).   // 避免重复词汇
    WithMaxTokens(4000).
    Build()

story, err := client.CallWithRequest(request)
```

### 精确分析（平衡参数）

```go
request := mcp.NewRequestBuilder().
    WithSystemPrompt("You are a quantitative analyst").
    WithUserPrompt("Analyze BTC/USDT chart pattern").
    WithTemperature(0.5).        // 中等温度
    WithMaxTokens(1500).
    WithStopSequences([]string{"---", "END"}).  // 多个停止序列
    Build()

analysis, err := client.CallWithRequest(request)
```

---

## Function Calling

### 天气查询工具

```go
// 定义工具参数 schema（JSON Schema 格式）
weatherParams := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "location": map[string]any{
            "type":        "string",
            "description": "City name, e.g., Beijing, Shanghai",
        },
        "unit": map[string]any{
            "type": "string",
            "enum": []string{"celsius", "fahrenheit"},
        },
    },
    "required": []string{"location"},
}

// 构建请求
request := mcp.NewRequestBuilder().
    WithUserPrompt("北京今天天气怎么样？").
    AddFunction(
        "get_weather",                 // 函数名
        "Get current weather",         // 函数描述
        weatherParams,                 // 参数定义
    ).
    WithToolChoice("auto").            // 让 AI 自动决定是否调用
    Build()

response, err := client.CallWithRequest(request)

// AI 可能返回 tool_calls，你需要执行函数并返回结果
// （具体实现取决于 AI provider 的响应格式）
```

### 多个工具

```go
// 定义多个工具
request := mcp.NewRequestBuilder().
    WithUserPrompt("帮我查询北京天气，并计算100的平方根").
    AddFunction("get_weather", "Get weather", weatherParams).
    AddFunction("calculate", "Calculate math", calcParams).
    AddFunction("search_web", "Search web", searchParams).
    WithToolChoice("auto").
    Build()

response, err := client.CallWithRequest(request)
// AI 会选择调用相应的工具
```

### 强制使用特定工具

```go
request := mcp.NewRequestBuilder().
    WithUserPrompt("北京").
    AddFunction("get_weather", "Get weather", weatherParams).
    WithToolChoice(`{"type": "function", "function": {"name": "get_weather"}}`).
    Build()

// AI 必须调用 get_weather 函数
```

---

## 预设场景

### ForChat - 聊天场景

```go
// 预设参数：temperature=0.7, maxTokens=2000
request := mcp.ForChat().
    WithSystemPrompt("You are a friendly chatbot").
    WithUserPrompt("Hello!").
    Build()

// 等价于
request := mcp.NewRequestBuilder().
    WithSystemPrompt("You are a friendly chatbot").
    WithUserPrompt("Hello!").
    WithTemperature(0.7).
    WithMaxTokens(2000).
    Build()
```

### ForCodeGeneration - 代码生成场景

```go
// 预设参数：temperature=0.2, topP=0.1, maxTokens=2000
request := mcp.ForCodeGeneration().
    WithUserPrompt("Generate a REST API in Go").
    Build()

// 自动使用低温度和低 top_p，确保代码准确性
```

### ForCreativeWriting - 创意写作场景

```go
// 预设参数：
// temperature=1.2, topP=0.95, maxTokens=4000
// presencePenalty=0.6, frequencyPenalty=0.5
request := mcp.ForCreativeWriting().
    WithSystemPrompt("You are a novelist").
    WithUserPrompt("Write a fantasy story").
    Build()

// 自动使用高温度和惩罚参数，增加创意和多样性
```

---

## 完整示例

### 量化交易 AI 顾问

```go
package main

import (
    "fmt"
    "log"
    "nofx/mcp"
    "os"
)

func main() {
    // 创建客户端
    client := mcp.NewDeepSeekClientWithOptions(
        mcp.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
        mcp.WithMaxRetries(5),
        mcp.WithTimeout(60 * time.Second),
    )

    // 场景1: 市场分析（需要精确）
    analysisRequest := mcp.NewRequestBuilder().
        WithSystemPrompt("You are a professional quantitative trader").
        WithUserPrompt("Analyze BTC/USDT 1H chart, current price $45,000").
        WithTemperature(0.3).  // 低温度，更精确
        WithMaxTokens(1500).
        Build()

    analysis, err := client.CallWithRequest(analysisRequest)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("=== Market Analysis ===")
    fmt.Println(analysis)

    // 场景2: 继续对话，询问入场点
    followUpRequest := mcp.NewRequestBuilder().
        AddSystemMessage("You are a professional quantitative trader").
        AddUserMessage("Analyze BTC/USDT 1H chart, current price $45,000").
        AddAssistantMessage(analysis).  // 添加之前的回复
        AddUserMessage("Based on your analysis, what's the best entry point?").
        WithTemperature(0.3).
        Build()

    entryPoint, err := client.CallWithRequest(followUpRequest)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("\n=== Entry Point Suggestion ===")
    fmt.Println(entryPoint)
}
```

### 代码评审助手

```go
func reviewCode(client mcp.AIClient, code string) (string, error) {
    request := mcp.ForCodeGeneration().  // 使用代码场景预设
        WithSystemPrompt("You are a senior Go developer reviewing code").
        WithUserPrompt(fmt.Sprintf("Review this code:\n\n```go\n%s\n```", code)).
        WithMaxTokens(2000).
        AddStopSequence("---END---").
        Build()

    return client.CallWithRequest(request)
}

func main() {
    client := mcp.NewDeepSeekClientWithOptions(
        mcp.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
    )

    code := `
func Add(a, b int) int {
    return a + b
}
`

    review, err := reviewCode(client, code)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(review)
}
```

### AI 聊天机器人（带历史记录）

```go
type ChatBot struct {
    client  mcp.AIClient
    history []mcp.Message
}

func NewChatBot(client mcp.AIClient, systemPrompt string) *ChatBot {
    return &ChatBot{
        client: client,
        history: []mcp.Message{
            mcp.NewSystemMessage(systemPrompt),
        },
    }
}

func (bot *ChatBot) Chat(userMessage string) (string, error) {
    // 添加用户消息到历史
    bot.history = append(bot.history, mcp.NewUserMessage(userMessage))

    // 构建请求（包含完整历史）
    request := mcp.ForChat().
        AddMessages(bot.history...).
        Build()

    // 调用 API
    response, err := bot.client.CallWithRequest(request)
    if err != nil {
        return "", err
    }

    // 添加 AI 回复到历史
    bot.history = append(bot.history, mcp.NewAssistantMessage(response))

    return response, nil
}

func main() {
    client := mcp.NewDeepSeekClientWithOptions(
        mcp.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
    )

    bot := NewChatBot(client, "You are a friendly and helpful assistant")

    // 对话1
    resp1, _ := bot.Chat("What is Go?")
    fmt.Println("User: What is Go?")
    fmt.Println("Bot:", resp1)

    // 对话2（带上下文）
    resp2, _ := bot.Chat("What are its main features?")
    fmt.Println("\nUser: What are its main features?")
    fmt.Println("Bot:", resp2)

    // 对话3（继续上下文）
    resp3, _ := bot.Chat("Show me an example")
    fmt.Println("\nUser: Show me an example")
    fmt.Println("Bot:", resp3)
}
```

### Function Calling 完整示例

```go
package main

import (
    "encoding/json"
    "fmt"
    "nofx/mcp"
    "os"
)

// 天气查询函数（模拟）
func getWeather(location string) string {
    return fmt.Sprintf("Weather in %s: Sunny, 25°C", location)
}

func main() {
    client := mcp.NewDeepSeekClientWithOptions(
        mcp.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
    )

    // 定义工具
    weatherParams := map[string]any{
        "type": "object",
        "properties": map[string]any{
            "location": map[string]any{
                "type":        "string",
                "description": "City name",
            },
        },
        "required": []string{"location"},
    }

    // 第一步：发送带工具的请求
    request := mcp.NewRequestBuilder().
        WithUserPrompt("北京天气怎么样？").
        AddFunction("get_weather", "Get current weather", weatherParams).
        WithToolChoice("auto").
        Build()

    response, err := client.CallWithRequest(request)
    if err != nil {
        panic(err)
    }

    fmt.Println("AI Response:", response)

    // 第二步：如果 AI 返回了 tool_call（实际需要解析 JSON 响应）
    // 这里是示例，实际需要根据 provider 的响应格式解析
    // toolCall := parseToolCall(response)
    // weatherResult := getWeather(toolCall.Arguments.Location)

    // 第三步：将工具结果返回给 AI
    // followUp := mcp.NewRequestBuilder().
    //     AddConversationHistory(previousMessages).
    //     AddToolResult(toolCall.ID, weatherResult).
    //     Build()
    //
    // finalResponse, _ := client.CallWithRequest(followUp)
}
```

---

## 最佳实践

### 1. 使用 MustBuild() vs Build()

```go
// Build() - 返回 error，需要处理
request, err := NewRequestBuilder().
    WithUserPrompt("Hello").
    Build()
if err != nil {
    log.Fatal(err)
}

// MustBuild() - 如果失败会 panic，适用于确定不会错的场景
request := NewRequestBuilder().
    WithSystemPrompt("You are helpful").
    WithUserPrompt("Hello").
    MustBuild()  // 构建失败会 panic
```

### 2. 重用构建器

```go
// 创建基础构建器
baseBuilder := mcp.NewRequestBuilder().
    WithSystemPrompt("You are a trading advisor").
    WithTemperature(0.3)

// 为不同问题添加用户消息
question1 := baseBuilder.
    AddUserMessage("Analyze BTC").
    Build()

question2 := baseBuilder.
    ClearMessages().  // 清空之前的消息
    AddSystemMessage("You are a trading advisor").
    AddUserMessage("Analyze ETH").
    Build()
```

### 3. 选择合适的预设

```go
// ✅ 代码生成 - 使用 ForCodeGeneration
ForCodeGeneration().WithUserPrompt("Generate code")

// ✅ 聊天 - 使用 ForChat
ForChat().WithUserPrompt("Hello")

// ✅ 创意写作 - 使用 ForCreativeWriting
ForCreativeWriting().WithUserPrompt("Write a story")

// ✅ 自定义 - 使用 NewRequestBuilder
NewRequestBuilder().WithTemperature(0.6).WithUserPrompt("...")
```

---

## 迁移指南

### 从旧 API 迁移

```go
// 旧 API（仍然可用）
result, err := client.CallWithMessages("system", "user")

// 迁移到新 API
request := mcp.NewRequestBuilder().
    WithSystemPrompt("system").
    WithUserPrompt("user").
    Build()
result, err := client.CallWithRequest(request)

// 如果需要更多控制
request := mcp.NewRequestBuilder().
    WithSystemPrompt("system").
    WithUserPrompt("user").
    WithTemperature(0.8).      // 新功能
    WithMaxTokens(2000).       // 新功能
    Build()
result, err := client.CallWithRequest(request)
```

---

更多信息请参考：
- [构建器模式价值分析](./BUILDER_PATTERN_BENEFITS.md)
- [MCP 使用指南](./README.md)
