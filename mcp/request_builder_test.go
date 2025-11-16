package mcp

import (
	"encoding/json"
	"testing"
)

// ============================================================
// 测试 RequestBuilder 基本功能
// ============================================================

func TestRequestBuilder_BasicUsage(t *testing.T) {
	request, err := NewRequestBuilder().
		WithSystemPrompt("You are helpful").
		WithUserPrompt("Hello").
		Build()

	if err != nil {
		t.Fatalf("Build should not error: %v", err)
	}

	if len(request.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(request.Messages))
	}

	if request.Messages[0].Role != "system" {
		t.Errorf("first message should be system, got %s", request.Messages[0].Role)
	}

	if request.Messages[1].Role != "user" {
		t.Errorf("second message should be user, got %s", request.Messages[1].Role)
	}
}

func TestRequestBuilder_EmptyMessages(t *testing.T) {
	_, err := NewRequestBuilder().Build()

	if err == nil {
		t.Error("Build should error when no messages")
	}

	if err.Error() != "至少需要一条消息" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// 测试消息构建方法
// ============================================================

func TestRequestBuilder_MultipleMessages(t *testing.T) {
	request := NewRequestBuilder().
		AddSystemMessage("You are helpful").
		AddUserMessage("What is Go?").
		AddAssistantMessage("Go is a programming language").
		AddUserMessage("Tell me more").
		MustBuild()

	if len(request.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(request.Messages))
	}

	expectedRoles := []string{"system", "user", "assistant", "user"}
	for i, expected := range expectedRoles {
		if request.Messages[i].Role != expected {
			t.Errorf("message %d: expected role %s, got %s", i, expected, request.Messages[i].Role)
		}
	}
}

func TestRequestBuilder_AddConversationHistory(t *testing.T) {
	history := []Message{
		NewUserMessage("Previous question"),
		NewAssistantMessage("Previous answer"),
	}

	request := NewRequestBuilder().
		AddConversationHistory(history).
		AddUserMessage("New question").
		MustBuild()

	if len(request.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(request.Messages))
	}
}

// ============================================================
// 测试参数控制方法
// ============================================================

func TestRequestBuilder_WithTemperature(t *testing.T) {
	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		WithTemperature(0.8).
		MustBuild()

	if request.Temperature == nil {
		t.Fatal("Temperature should be set")
	}

	if *request.Temperature != 0.8 {
		t.Errorf("expected temperature 0.8, got %f", *request.Temperature)
	}
}

func TestRequestBuilder_WithMaxTokens(t *testing.T) {
	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		WithMaxTokens(2000).
		MustBuild()

	if request.MaxTokens == nil {
		t.Fatal("MaxTokens should be set")
	}

	if *request.MaxTokens != 2000 {
		t.Errorf("expected maxTokens 2000, got %d", *request.MaxTokens)
	}
}

func TestRequestBuilder_WithTopP(t *testing.T) {
	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		WithTopP(0.9).
		MustBuild()

	if request.TopP == nil {
		t.Fatal("TopP should be set")
	}

	if *request.TopP != 0.9 {
		t.Errorf("expected topP 0.9, got %f", *request.TopP)
	}
}

func TestRequestBuilder_WithPenalties(t *testing.T) {
	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		WithFrequencyPenalty(0.5).
		WithPresencePenalty(0.6).
		MustBuild()

	if request.FrequencyPenalty == nil || *request.FrequencyPenalty != 0.5 {
		t.Error("FrequencyPenalty should be 0.5")
	}

	if request.PresencePenalty == nil || *request.PresencePenalty != 0.6 {
		t.Error("PresencePenalty should be 0.6")
	}
}

func TestRequestBuilder_WithStopSequences(t *testing.T) {
	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		WithStopSequences([]string{"STOP", "END"}).
		MustBuild()

	if len(request.Stop) != 2 {
		t.Fatalf("expected 2 stop sequences, got %d", len(request.Stop))
	}

	if request.Stop[0] != "STOP" || request.Stop[1] != "END" {
		t.Error("stop sequences not set correctly")
	}
}

// ============================================================
// 测试工具/函数调用
// ============================================================

func TestRequestBuilder_AddTool(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: FunctionDef{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{"type": "string"},
				},
			},
		},
	}

	request := NewRequestBuilder().
		WithUserPrompt("What's the weather?").
		AddTool(tool).
		WithToolChoice("auto").
		MustBuild()

	if len(request.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(request.Tools))
	}

	if request.Tools[0].Function.Name != "get_weather" {
		t.Error("tool not added correctly")
	}

	if request.ToolChoice != "auto" {
		t.Error("tool choice not set correctly")
	}
}

func TestRequestBuilder_AddFunction(t *testing.T) {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string"},
		},
	}

	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		AddFunction("get_weather", "Get current weather", params).
		MustBuild()

	if len(request.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(request.Tools))
	}

	if request.Tools[0].Type != "function" {
		t.Error("tool type should be function")
	}

	if request.Tools[0].Function.Name != "get_weather" {
		t.Error("function name not set correctly")
	}
}

// ============================================================
// 测试便捷方法
// ============================================================

func TestRequestBuilder_ForChat(t *testing.T) {
	request := ForChat().
		WithUserPrompt("Hello").
		MustBuild()

	if request.Temperature == nil {
		t.Fatal("ForChat should set temperature")
	}

	if *request.Temperature != 0.7 {
		t.Errorf("ForChat should set temperature to 0.7, got %f", *request.Temperature)
	}

	if request.MaxTokens == nil {
		t.Fatal("ForChat should set maxTokens")
	}

	if *request.MaxTokens != 2000 {
		t.Errorf("ForChat should set maxTokens to 2000, got %d", *request.MaxTokens)
	}
}

func TestRequestBuilder_ForCodeGeneration(t *testing.T) {
	request := ForCodeGeneration().
		WithUserPrompt("Generate code").
		MustBuild()

	if request.Temperature == nil || *request.Temperature != 0.2 {
		t.Error("ForCodeGeneration should set low temperature")
	}

	if request.TopP == nil || *request.TopP != 0.1 {
		t.Error("ForCodeGeneration should set low topP")
	}
}

func TestRequestBuilder_ForCreativeWriting(t *testing.T) {
	request := ForCreativeWriting().
		WithUserPrompt("Write a story").
		MustBuild()

	if request.Temperature == nil || *request.Temperature != 1.2 {
		t.Error("ForCreativeWriting should set high temperature")
	}

	if request.PresencePenalty == nil || *request.PresencePenalty != 0.6 {
		t.Error("ForCreativeWriting should set presence penalty")
	}

	if request.FrequencyPenalty == nil || *request.FrequencyPenalty != 0.5 {
		t.Error("ForCreativeWriting should set frequency penalty")
	}
}

// ============================================================
// 测试 CallWithRequest 集成
// ============================================================

func TestClient_CallWithRequest_Success(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("Builder response")
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	request := NewRequestBuilder().
		WithSystemPrompt("You are helpful").
		WithUserPrompt("Hello").
		WithTemperature(0.8).
		MustBuild()

	result, err := client.CallWithRequest(request)

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	if result != "Builder response" {
		t.Errorf("expected 'Builder response', got '%s'", result)
	}

	// 验证请求体
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	// 解析请求体验证参数
	var body map[string]interface{}
	decoder := json.NewDecoder(requests[0].Body)
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	// 验证 temperature
	if body["temperature"] != 0.8 {
		t.Errorf("expected temperature 0.8, got %v", body["temperature"])
	}

	// 验证 messages
	messages, ok := body["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Error("messages not correctly formatted")
	}
}

func TestClient_CallWithRequest_MultiRound(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("Multi-round response")
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	// 构建多轮对话
	request := NewRequestBuilder().
		AddSystemMessage("You are a trading advisor").
		AddUserMessage("Analyze BTC").
		AddAssistantMessage("BTC is bullish").
		AddUserMessage("What about entry point?").
		WithTemperature(0.3).
		MustBuild()

	result, err := client.CallWithRequest(request)

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	if result != "Multi-round response" {
		t.Errorf("expected 'Multi-round response', got '%s'", result)
	}

	// 验证请求体包含所有消息
	requests := mockHTTP.GetRequests()
	var body map[string]interface{}
	json.NewDecoder(requests[0].Body).Decode(&body)

	messages := body["messages"].([]interface{})
	if len(messages) != 4 {
		t.Errorf("expected 4 messages in request, got %d", len(messages))
	}
}

func TestClient_CallWithRequest_WithTools(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("Tool response")
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	request := NewRequestBuilder().
		WithUserPrompt("What's the weather in Beijing?").
		AddFunction("get_weather", "Get weather", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{"type": "string"},
			},
		}).
		WithToolChoice("auto").
		MustBuild()

	_, err := client.CallWithRequest(request)

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	// 验证请求体包含 tools
	requests := mockHTTP.GetRequests()
	var body map[string]interface{}
	json.NewDecoder(requests[0].Body).Decode(&body)

	tools, ok := body["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		t.Error("tools should be present in request")
	}

	toolChoice, ok := body["tool_choice"].(string)
	if !ok || toolChoice != "auto" {
		t.Error("tool_choice should be 'auto'")
	}
}

func TestClient_CallWithRequest_NoAPIKey(t *testing.T) {
	client := NewClient()

	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		MustBuild()

	_, err := client.CallWithRequest(request)

	if err == nil {
		t.Error("should error when API key not set")
	}

	if err.Error() != "AI API密钥未设置，请先调用 SetAPIKey" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClient_CallWithRequest_UsesClientModel(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("Response")
	mockLogger := NewMockLogger()

	client := NewDeepSeekClientWithOptions(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	// Request 不设置 model，应该使用 Client 的 model
	request := NewRequestBuilder().
		WithUserPrompt("Hello").
		MustBuild()

	if request.Model != "" {
		t.Error("request.Model should be empty initially")
	}

	client.CallWithRequest(request)

	// 验证使用了 DeepSeek 的 model
	requests := mockHTTP.GetRequests()
	var body map[string]interface{}
	json.NewDecoder(requests[0].Body).Decode(&body)

	if body["model"] != DefaultDeepSeekModel {
		t.Errorf("expected model %s, got %v", DefaultDeepSeekModel, body["model"])
	}
}
