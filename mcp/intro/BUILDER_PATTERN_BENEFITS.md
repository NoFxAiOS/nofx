# 构建器模式在 MCP 模块中的应用价值

## 📋 目录
1. [当前实现的局限性](#当前实现的局限性)
2. [构建器模式的好处](#构建器模式的好处)
3. [实际应用场景](#实际应用场景)
4. [对比示例](#对比示例)
5. [是否需要引入](#是否需要引入)

---

## 当前实现的局限性

### 现状分析

**当前 buildMCPRequestBody 实现**:
```go
func (client *Client) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
    messages := []map[string]string{}

    if systemPrompt != "" {
        messages = append(messages, map[string]string{
            "role":    "system",
            "content": systemPrompt,
        })
    }
    messages = append(messages, map[string]string{
        "role":    "user",
        "content": userPrompt,
    })

    return map[string]interface{}{
        "model":       client.Model,
        "messages":    messages,
        "temperature": client.config.Temperature,
        "max_tokens":  client.MaxTokens,
    }
}
```

### 存在的限制

1. **只支持简单对话**
   - ❌ 无法添加多轮对话历史
   - ❌ 无法添加 assistant 回复
   - ❌ 无法构建复杂的对话上下文

2. **参数固定**
   - ❌ 无法动态添加可选参数（如 top_p、frequency_penalty）
   - ❌ 无法为单次请求自定义 temperature（会影响全局配置）
   - ❌ 无法添加 function calling、tools 等高级功能

3. **扩展性差**
   - ❌ 每次添加新参数都需要修改方法签名
   - ❌ 参数列表会越来越长
   - ❌ 子类重写时需要处理所有参数

---

## 构建器模式的好处

### 1. 🎯 **灵活性和可读性**

#### 当前方式（参数传递）
```go
// 问题：参数多了会很混乱
client.CallWithCustomParams(
    "system prompt",
    "user prompt",
    0.8,              // temperature - 这是什么？
    2000,             // max_tokens - 这是什么？
    0.9,              // top_p - 这是什么？
    0.5,              // frequency_penalty
    nil,              // stop sequences
    false,            // stream
)
```

#### 构建器方式
```go
// 清晰、自解释
request := NewRequestBuilder().
    WithSystemPrompt("You are a helpful assistant").
    WithUserPrompt("Tell me about Go").
    WithTemperature(0.8).
    WithMaxTokens(2000).
    WithTopP(0.9).
    Build()

result, err := client.CallWithRequest(request)
```

---

### 2. 📚 **支持复杂场景**

#### 场景1: 多轮对话

**当前方式**: 😢 不支持
```go
// ❌ 无法实现
client.CallWithMessages("system", "user prompt")
```

**构建器方式**: ✅ 支持
```go
request := NewRequestBuilder().
    AddSystemMessage("You are a helpful assistant").
    AddUserMessage("What is the weather?").
    AddAssistantMessage("It's sunny today").
    AddUserMessage("What about tomorrow?").  // 继续对话
    WithTemperature(0.7).
    Build()
```

#### 场景2: 函数调用（Function Calling）

**当前方式**: 😢 不支持
```go
// ❌ 无法添加 tools/functions
```

**构建器方式**: ✅ 支持
```go
request := NewRequestBuilder().
    WithUserPrompt("What's the weather in Beijing?").
    AddTool(Tool{
        Type: "function",
        Function: FunctionDef{
            Name:        "get_weather",
            Description: "Get current weather",
            Parameters:  weatherParamsSchema,
        },
    }).
    WithToolChoice("auto").
    Build()
```

#### 场景3: 流式响应

**当前方式**: 😢 需要修改整个架构
```go
// ❌ CallWithMessages 不支持流式
```

**构建器方式**: ✅ 易于扩展
```go
request := NewRequestBuilder().
    WithUserPrompt("Write a long story").
    WithStream(true).
    Build()

stream, err := client.CallStream(request)
for chunk := range stream {
    fmt.Print(chunk)
}
```

---

### 3. 🔧 **易于扩展和维护**

#### 添加新参数

**当前方式**: 😢 破坏性修改
```go
// 需要修改方法签名（破坏现有代码）
func (client *Client) buildMCPRequestBody(
    systemPrompt, userPrompt string,
    // 新增参数会导致所有调用处都要修改
    topP float64,
    presencePenalty float64,
) map[string]any
```

**构建器方式**: ✅ 向后兼容
```go
// 只需添加新方法，不影响现有代码
func (b *RequestBuilder) WithPresencePenalty(p float64) *RequestBuilder {
    b.presencePenalty = p
    return b
}

// 旧代码不受影响
request := builder.WithUserPrompt("Hello").Build()

// 新代码可以使用新功能
request := builder.
    WithUserPrompt("Hello").
    WithPresencePenalty(0.6).  // 新参数
    Build()
```

---

### 4. 🎨 **可选参数处理**

**当前方式**: 😢 难以处理可选参数
```go
// 方案1: 传 nil/0 值（不优雅）
client.CallWithParams(system, user, 0, 0, nil, nil)

// 方案2: 使用选项模式（但每次调用都要传）
client.CallWithParams(system, user, WithTopP(0.9), WithPenalty(0.5))

// 方案3: 配置对象（需要创建临时对象）
config := &RequestConfig{
    SystemPrompt: system,
    UserPrompt:   user,
    TopP:         0.9,
}
```

**构建器方式**: ✅ 优雅处理
```go
// 只设置需要的参数，其他使用默认值
request := NewRequestBuilder().
    WithUserPrompt("Hello").
    // 不设置 temperature，使用默认值
    // 不设置 topP，使用默认值
    Build()

// 也可以全部自定义
request := NewRequestBuilder().
    WithUserPrompt("Hello").
    WithTemperature(0.8).
    WithTopP(0.9).
    WithMaxTokens(2000).
    Build()
```

---

### 5. ✅ **类型安全和验证**

**当前方式**: 😢 运行时才发现错误
```go
// ❌ 编译时无法发现问题
client.CallWithMessages("", "")  // 空 prompt
client.CallWithMessages("system", "user")  // temperature 可能不合法
```

**构建器方式**: ✅ 提前验证
```go
type RequestBuilder struct {
    messages    []Message
    temperature float64
    maxTokens   int
}

func (b *RequestBuilder) WithTemperature(t float64) *RequestBuilder {
    if t < 0 || t > 2 {
        panic("temperature must be between 0 and 2")  // 或返回 error
    }
    b.temperature = t
    return b
}

func (b *RequestBuilder) Build() (*Request, error) {
    if len(b.messages) == 0 {
        return nil, errors.New("at least one message is required")
    }
    if b.maxTokens <= 0 {
        return nil, errors.New("maxTokens must be positive")
    }
    return &Request{...}, nil
}
```

---

## 实际应用场景

### 场景1: 量化交易 AI 顾问（多轮对话）

```go
// 构建包含市场数据的上下文对话
request := NewRequestBuilder().
    AddSystemMessage("You are a quantitative trading advisor").
    AddUserMessage("Analyze BTC trend").
    AddAssistantMessage("BTC is in an upward trend based on...").
    AddUserMessage("What about entry points?").  // 继续对话
    WithTemperature(0.3).  // 低温度，更精确
    WithMaxTokens(1000).
    Build()

analysis, err := client.CallWithRequest(request)
```

### 场景2: 代码生成（需要精确控制）

```go
request := NewRequestBuilder().
    WithSystemPrompt("You are a Go expert").
    WithUserPrompt("Generate a HTTP server").
    WithTemperature(0.2).        // 低温度，更确定性
    WithTopP(0.1).               // 低 top_p，更聚焦
    WithMaxTokens(2000).
    WithStopSequences([]string{"```"}).  // 遇到代码块结束符停止
    Build()
```

### 场景3: 创意写作（需要随机性）

```go
request := NewRequestBuilder().
    WithSystemPrompt("You are a creative writer").
    WithUserPrompt("Write a sci-fi story").
    WithTemperature(1.2).        // 高温度，更创意
    WithTopP(0.95).              // 高 top_p，更多样性
    WithPresencePenalty(0.6).    // 避免重复
    WithFrequencyPenalty(0.5).
    WithMaxTokens(4000).
    Build()
```

### 场景4: 函数调用（工具使用）

```go
// 定义工具
weatherTool := Tool{
    Type: "function",
    Function: FunctionDef{
        Name:        "get_weather",
        Description: "Get current weather for a location",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "location": map[string]any{
                    "type":        "string",
                    "description": "City name",
                },
            },
            "required": []string{"location"},
        },
    },
}

request := NewRequestBuilder().
    WithUserPrompt("What's the weather in Beijing?").
    AddTool(weatherTool).
    WithToolChoice("auto").
    Build()

response, err := client.CallWithRequest(request)
// 解析 response.ToolCalls 并执行实际的天气查询
```

---

## 对比示例

### 示例1: 基础用法

#### 当前实现
```go
result, err := client.CallWithMessages(
    "You are a helpful assistant",
    "What is Go?",
)
```

#### 构建器模式
```go
request := NewRequestBuilder().
    WithSystemPrompt("You are a helpful assistant").
    WithUserPrompt("What is Go?").
    Build()

result, err := client.CallWithRequest(request)
```

**分析**: 基础用法下，构建器稍显冗长，但更清晰。

---

### 示例2: 复杂用法

#### 当前实现（假设扩展后）
```go
// 😢 参数太多，难以理解
result, err := client.CallWithMessagesAdvanced(
    "system prompt",
    "user prompt",
    nil,    // messages history?
    0.8,    // temperature
    2000,   // max_tokens
    0.9,    // top_p
    0.5,    // frequency_penalty
    0.6,    // presence_penalty
    nil,    // stop sequences
    false,  // stream
    nil,    // tools
    "",     // tool_choice
)
```

#### 构建器模式
```go
// ✅ 清晰、自解释
request := NewRequestBuilder().
    WithSystemPrompt("system prompt").
    WithUserPrompt("user prompt").
    WithTemperature(0.8).
    WithMaxTokens(2000).
    WithTopP(0.9).
    WithFrequencyPenalty(0.5).
    WithPresencePenalty(0.6).
    Build()

result, err := client.CallWithRequest(request)
```

**分析**: 复杂场景下，构建器模式优势明显。

---

## 是否需要引入？

### ✅ 建议引入的情况

1. **需要支持多轮对话**
   - 聊天机器人
   - 上下文相关的 AI 助手

2. **需要精细控制 AI 参数**
   - 不同任务需要不同 temperature
   - 需要使用 top_p、penalty 等高级参数

3. **需要使用 AI 高级功能**
   - Function Calling / Tools
   - 流式响应
   - Vision API（图片输入）

4. **API 接口可能频繁变化**
   - AI 提供商经常添加新参数
   - 需要向后兼容

### ⚠️ 可以暂缓的情况

1. **只有简单的单轮对话**
   - 当前 `CallWithMessages` 已足够

2. **参数固定不变**
   - 所有请求使用相同配置

3. **团队规模小，代码量少**
   - 引入新模式的学习成本 > 收益

---

## 推荐方案

### 方案1: 渐进式引入（推荐）

**第一阶段**: 保留现有 API，新增构建器
```go
// 旧 API 继续工作（向后兼容）
result, err := client.CallWithMessages("system", "user")

// 新 API 提供高级功能
request := NewRequestBuilder().
    WithUserPrompt("user").
    WithTemperature(0.8).
    Build()
result, err := client.CallWithRequest(request)
```

**第二阶段**: 逐步迁移
```go
// 在文档中推荐使用构建器
// 旧 API 标记为 Deprecated（但不删除）
```

### 方案2: 仅用于高级场景

只在需要复杂功能时使用构建器：
```go
// 简单场景：使用现有 API
client.CallWithMessages("system", "user")

// 复杂场景：使用构建器
client.CallWithRequest(
    NewRequestBuilder().
        AddConversationHistory(history).
        AddUserMessage("new question").
        WithTools(tools).
        Build(),
)
```

---

## 实现示例

### 完整的构建器实现

```go
package mcp

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type Tool struct {
    Type     string      `json:"type"`
    Function FunctionDef `json:"function"`
}

type Request struct {
    Model             string    `json:"model"`
    Messages          []Message `json:"messages"`
    Temperature       float64   `json:"temperature,omitempty"`
    MaxTokens         int       `json:"max_tokens,omitempty"`
    TopP              float64   `json:"top_p,omitempty"`
    FrequencyPenalty  float64   `json:"frequency_penalty,omitempty"`
    PresencePenalty   float64   `json:"presence_penalty,omitempty"`
    Stop              []string  `json:"stop,omitempty"`
    Tools             []Tool    `json:"tools,omitempty"`
    ToolChoice        string    `json:"tool_choice,omitempty"`
    Stream            bool      `json:"stream,omitempty"`
}

type RequestBuilder struct {
    model            string
    messages         []Message
    temperature      *float64
    maxTokens        *int
    topP             *float64
    frequencyPenalty *float64
    presencePenalty  *float64
    stop             []string
    tools            []Tool
    toolChoice       string
    stream           bool
}

func NewRequestBuilder() *RequestBuilder {
    return &RequestBuilder{
        messages: make([]Message, 0),
    }
}

func (b *RequestBuilder) WithModel(model string) *RequestBuilder {
    b.model = model
    return b
}

func (b *RequestBuilder) WithSystemPrompt(prompt string) *RequestBuilder {
    if prompt != "" {
        b.messages = append(b.messages, Message{
            Role:    "system",
            Content: prompt,
        })
    }
    return b
}

func (b *RequestBuilder) WithUserPrompt(prompt string) *RequestBuilder {
    b.messages = append(b.messages, Message{
        Role:    "user",
        Content: prompt,
    })
    return b
}

func (b *RequestBuilder) AddUserMessage(content string) *RequestBuilder {
    return b.WithUserPrompt(content)
}

func (b *RequestBuilder) AddSystemMessage(content string) *RequestBuilder {
    return b.WithSystemPrompt(content)
}

func (b *RequestBuilder) AddAssistantMessage(content string) *RequestBuilder {
    b.messages = append(b.messages, Message{
        Role:    "assistant",
        Content: content,
    })
    return b
}

func (b *RequestBuilder) AddMessage(role, content string) *RequestBuilder {
    b.messages = append(b.messages, Message{
        Role:    role,
        Content: content,
    })
    return b
}

func (b *RequestBuilder) AddConversationHistory(history []Message) *RequestBuilder {
    b.messages = append(b.messages, history...)
    return b
}

func (b *RequestBuilder) WithTemperature(t float64) *RequestBuilder {
    if t < 0 || t > 2 {
        panic("temperature must be between 0 and 2")
    }
    b.temperature = &t
    return b
}

func (b *RequestBuilder) WithMaxTokens(tokens int) *RequestBuilder {
    b.maxTokens = &tokens
    return b
}

func (b *RequestBuilder) WithTopP(p float64) *RequestBuilder {
    b.topP = &p
    return b
}

func (b *RequestBuilder) WithFrequencyPenalty(p float64) *RequestBuilder {
    b.frequencyPenalty = &p
    return b
}

func (b *RequestBuilder) WithPresencePenalty(p float64) *RequestBuilder {
    b.presencePenalty = &p
    return b
}

func (b *RequestBuilder) WithStopSequences(sequences []string) *RequestBuilder {
    b.stop = sequences
    return b
}

func (b *RequestBuilder) AddTool(tool Tool) *RequestBuilder {
    b.tools = append(b.tools, tool)
    return b
}

func (b *RequestBuilder) WithToolChoice(choice string) *RequestBuilder {
    b.toolChoice = choice
    return b
}

func (b *RequestBuilder) WithStream(stream bool) *RequestBuilder {
    b.stream = stream
    return b
}

func (b *RequestBuilder) Build() (*Request, error) {
    if len(b.messages) == 0 {
        return nil, errors.New("at least one message is required")
    }

    req := &Request{
        Model:      b.model,
        Messages:   b.messages,
        Stop:       b.stop,
        Tools:      b.tools,
        ToolChoice: b.toolChoice,
        Stream:     b.stream,
    }

    // 只设置非 nil 的可选参数
    if b.temperature != nil {
        req.Temperature = *b.temperature
    }
    if b.maxTokens != nil {
        req.MaxTokens = *b.maxTokens
    }
    if b.topP != nil {
        req.TopP = *b.topP
    }
    if b.frequencyPenalty != nil {
        req.FrequencyPenalty = *b.frequencyPenalty
    }
    if b.presencePenalty != nil {
        req.PresencePenalty = *b.presencePenalty
    }

    return req, nil
}
```

### Client 集成

```go
// 新增方法（不影响现有代码）
func (client *Client) CallWithRequest(req *Request) (string, error) {
    // 使用 req 中的参数发送请求
    // ...
}
```

---

## 总结

### 核心优势
1. ✅ **灵活性** - 轻松支持复杂场景
2. ✅ **可读性** - 代码自解释，易于理解
3. ✅ **可扩展性** - 添加新功能不破坏现有代码
4. ✅ **类型安全** - 编译时检查，提前发现错误
5. ✅ **向后兼容** - 可以与现有 API 共存

### 建议
- **当前阶段**: 如果只需要简单对话，现有实现已足够
- **未来扩展**: 当需要以下功能时再引入
  - 多轮对话
  - Function Calling
  - 流式响应
  - 精细参数控制

### 最佳实践
采用**渐进式引入**策略：
1. 保留现有 `CallWithMessages` API
2. 新增 `CallWithRequest` + 构建器
3. 在文档中推荐新 API，但不强制迁移
4. 根据实际需求逐步完善构建器功能

这样既能保持向后兼容，又能为未来的功能扩展做好准备。
