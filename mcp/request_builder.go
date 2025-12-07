package mcp

import (
	"errors"
)

// RequestBuilder request builder
type RequestBuilder struct {
	model            string
	messages         []Message
	stream           bool
	temperature      *float64
	maxTokens        *int
	topP             *float64
	frequencyPenalty *float64
	presencePenalty  *float64
	stop             []string
	tools            []Tool
	toolChoice       string
}

// NewRequestBuilder creates request builder
//
// Usage example:
//   request := NewRequestBuilder().
//       WithSystemPrompt("You are helpful").
//       WithUserPrompt("Hello").
//       WithTemperature(0.8).
//       Build()
func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{
		messages: make([]Message, 0),
		tools:    make([]Tool, 0),
	}
}

// ============================================================
// Model and Stream Configuration
// ============================================================

// WithModel sets model name
func (b *RequestBuilder) WithModel(model string) *RequestBuilder {
	b.model = model
	return b
}

// WithStream sets whether to use streaming response
func (b *RequestBuilder) WithStream(stream bool) *RequestBuilder {
	b.stream = stream
	return b
}

// ============================================================
// Message Building Methods
// ============================================================

// WithSystemPrompt adds system prompt (convenience method)
func (b *RequestBuilder) WithSystemPrompt(prompt string) *RequestBuilder {
	if prompt != "" {
		b.messages = append(b.messages, NewSystemMessage(prompt))
	}
	return b
}

// WithUserPrompt adds user prompt (convenience method)
func (b *RequestBuilder) WithUserPrompt(prompt string) *RequestBuilder {
	if prompt != "" {
		b.messages = append(b.messages, NewUserMessage(prompt))
	}
	return b
}

// AddSystemMessage adds system message
func (b *RequestBuilder) AddSystemMessage(content string) *RequestBuilder {
	return b.WithSystemPrompt(content)
}

// AddUserMessage adds user message
func (b *RequestBuilder) AddUserMessage(content string) *RequestBuilder {
	return b.WithUserPrompt(content)
}

// AddAssistantMessage adds assistant message (for multi-turn conversation context)
func (b *RequestBuilder) AddAssistantMessage(content string) *RequestBuilder {
	if content != "" {
		b.messages = append(b.messages, NewAssistantMessage(content))
	}
	return b
}

// AddMessage adds message with custom role
func (b *RequestBuilder) AddMessage(role, content string) *RequestBuilder {
	if content != "" {
		b.messages = append(b.messages, NewMessage(role, content))
	}
	return b
}

// AddMessages adds messages in batch
func (b *RequestBuilder) AddMessages(messages ...Message) *RequestBuilder {
	b.messages = append(b.messages, messages...)
	return b
}

// AddConversationHistory adds conversation history
func (b *RequestBuilder) AddConversationHistory(history []Message) *RequestBuilder {
	b.messages = append(b.messages, history...)
	return b
}

// ClearMessages clears all messages
func (b *RequestBuilder) ClearMessages() *RequestBuilder {
	b.messages = make([]Message, 0)
	return b
}

// ============================================================
// Parameter Control Methods
// ============================================================

// WithTemperature sets temperature parameter (0-2)
// Higher temperature (e.g. 1.2) makes output more random, lower temperature (e.g. 0.2) makes output more deterministic
func (b *RequestBuilder) WithTemperature(t float64) *RequestBuilder {
	if t < 0 || t > 2 {
		// Can choose to panic or silently ignore, here we choose to limit the range
		if t < 0 {
			t = 0
		}
		if t > 2 {
			t = 2
		}
	}
	b.temperature = &t
	return b
}

// WithMaxTokens sets maximum token count
func (b *RequestBuilder) WithMaxTokens(tokens int) *RequestBuilder {
	if tokens > 0 {
		b.maxTokens = &tokens
	}
	return b
}

// WithTopP sets top-p nucleus sampling parameter (0-1)
// Controls the range of tokens considered, smaller values (e.g. 0.1) make output more focused
func (b *RequestBuilder) WithTopP(p float64) *RequestBuilder {
	if p >= 0 && p <= 1 {
		b.topP = &p
	}
	return b
}

// WithFrequencyPenalty sets frequency penalty (-2 to 2)
// Positive values penalize tokens based on their frequency in the text, reducing repetition
func (b *RequestBuilder) WithFrequencyPenalty(penalty float64) *RequestBuilder {
	if penalty >= -2 && penalty <= 2 {
		b.frequencyPenalty = &penalty
	}
	return b
}

// WithPresencePenalty sets presence penalty (-2 to 2)
// Positive values penalize tokens based on whether they appear in the text, increasing topic diversity
func (b *RequestBuilder) WithPresencePenalty(penalty float64) *RequestBuilder {
	if penalty >= -2 && penalty <= 2 {
		b.presencePenalty = &penalty
	}
	return b
}

// WithStopSequences sets stop sequences
// Model will stop generating when it generates one of these sequences
func (b *RequestBuilder) WithStopSequences(sequences []string) *RequestBuilder {
	b.stop = sequences
	return b
}

// AddStopSequence adds a single stop sequence
func (b *RequestBuilder) AddStopSequence(sequence string) *RequestBuilder {
	if sequence != "" {
		b.stop = append(b.stop, sequence)
	}
	return b
}

// ============================================================
// Tool/Function Calling Related
// ============================================================

// AddTool adds a tool
func (b *RequestBuilder) AddTool(tool Tool) *RequestBuilder {
	b.tools = append(b.tools, tool)
	return b
}

// AddFunction adds a function (convenience method)
func (b *RequestBuilder) AddFunction(name, description string, parameters map[string]any) *RequestBuilder {
	tool := Tool{
		Type: "function",
		Function: FunctionDef{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
	b.tools = append(b.tools, tool)
	return b
}

// WithToolChoice sets tool choice strategy
// - "auto": automatically choose whether to call tools
// - "none": don't call tools
// - Can also specify a specific tool: `{"type": "function", "function": {"name": "my_function"}}`
func (b *RequestBuilder) WithToolChoice(choice string) *RequestBuilder {
	b.toolChoice = choice
	return b
}

// ============================================================
// Build Methods
// ============================================================

// Build builds request object
func (b *RequestBuilder) Build() (*Request, error) {
	// Validation: at least one message is required
	if len(b.messages) == 0 {
		return nil, errors.New("at least one message is required")
	}

	// Create request
	req := &Request{
		Model:      b.model,
		Messages:   b.messages,
		Stream:     b.stream,
		Stop:       b.stop,
		Tools:      b.tools,
		ToolChoice: b.toolChoice,
	}

	// Only set non-nil optional parameters (avoid sending 0 values that override server defaults)
	if b.temperature != nil {
		req.Temperature = b.temperature
	}
	if b.maxTokens != nil {
		req.MaxTokens = b.maxTokens
	}
	if b.topP != nil {
		req.TopP = b.topP
	}
	if b.frequencyPenalty != nil {
		req.FrequencyPenalty = b.frequencyPenalty
	}
	if b.presencePenalty != nil {
		req.PresencePenalty = b.presencePenalty
	}

	return req, nil
}

// MustBuild builds request object, panics if failed
// Suitable for scenarios where build is guaranteed not to fail
func (b *RequestBuilder) MustBuild() *Request {
	req, err := b.Build()
	if err != nil {
		panic(err)
	}
	return req
}

// ============================================================
// Convenience Methods: Preset Scenarios
// ============================================================

// ForChat creates builder for chat (preset with reasonable parameters)
func ForChat() *RequestBuilder {
	temp := 0.7
	tokens := 2000
	return &RequestBuilder{
		messages:    make([]Message, 0),
		tools:       make([]Tool, 0),
		temperature: &temp,
		maxTokens:   &tokens,
	}
}

// ForCodeGeneration creates builder for code generation (low temperature, more deterministic)
func ForCodeGeneration() *RequestBuilder {
	temp := 0.2
	tokens := 2000
	topP := 0.1
	return &RequestBuilder{
		messages:    make([]Message, 0),
		tools:       make([]Tool, 0),
		temperature: &temp,
		maxTokens:   &tokens,
		topP:        &topP,
	}
}

// ForCreativeWriting creates builder for creative writing (high temperature, more random)
func ForCreativeWriting() *RequestBuilder {
	temp := 1.2
	tokens := 4000
	topP := 0.95
	presencePenalty := 0.6
	frequencyPenalty := 0.5
	return &RequestBuilder{
		messages:         make([]Message, 0),
		tools:            make([]Tool, 0),
		temperature:      &temp,
		maxTokens:        &tokens,
		topP:             &topP,
		presencePenalty:  &presencePenalty,
		frequencyPenalty: &frequencyPenalty,
	}
}
