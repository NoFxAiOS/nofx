package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

// ============================================================
// 测试 Config 字段真正被使用（验证问题2修复）
// ============================================================

func TestConfig_MaxRetries_IsUsed(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// 设置 HTTP 客户端返回错误
	callCount := 0
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		callCount++
		return nil, errors.New("connection reset")
	}

	// 创建客户端并设置自定义重试次数为 5
	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
		WithMaxRetries(5), // ✅ 设置重试5次
	)

	// 调用 API（应该失败）
	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Error("should error")
	}

	// 验证确实重试了5次（而不是默认的3次）
	if callCount != 5 {
		t.Errorf("expected 5 retry attempts (from WithMaxRetries(5)), got %d", callCount)
	}

	// 验证日志中显示正确的重试次数
	logs := mockLogger.GetLogsByLevel("WARN")
	expectedWarningCount := 4 // 第2、3、4、5次重试时会打印警告
	actualWarningCount := 0
	for _, log := range logs {
		if log.Message == "⚠️  AI API调用失败，正在重试 (2/5)..." ||
			log.Message == "⚠️  AI API调用失败，正在重试 (3/5)..." ||
			log.Message == "⚠️  AI API调用失败，正在重试 (4/5)..." ||
			log.Message == "⚠️  AI API调用失败，正在重试 (5/5)..." {
			actualWarningCount++
		}
	}

	if actualWarningCount != expectedWarningCount {
		t.Errorf("expected %d warning logs, got %d", expectedWarningCount, actualWarningCount)
		for _, log := range logs {
			t.Logf("  WARN: %s", log.Message)
		}
	}
}

func TestConfig_Temperature_IsUsed(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("AI response")
	mockLogger := NewMockLogger()

	customTemperature := 0.8

	// 创建客户端并设置自定义 temperature
	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
		WithTemperature(customTemperature), // ✅ 设置自定义 temperature
	)

	c := client.(*Client)

	// 构建请求体
	requestBody := c.buildMCPRequestBody("system", "user")

	// 验证 temperature 字段
	temp, ok := requestBody["temperature"].(float64)
	if !ok {
		t.Fatal("temperature should be float64")
	}

	if temp != customTemperature {
		t.Errorf("expected temperature %f (from WithTemperature), got %f", customTemperature, temp)
	}

	// 也可以通过实际 HTTP 请求验证
	_, err := client.CallWithMessages("system", "user")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	// 检查发送的请求体
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	// 解析请求体
	var body map[string]interface{}
	decoder := json.NewDecoder(requests[0].Body)
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	// 验证 temperature
	if body["temperature"] != customTemperature {
		t.Errorf("expected temperature %f in HTTP request, got %v", customTemperature, body["temperature"])
	}
}

func TestConfig_RetryWaitBase_IsUsed(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// 设置成功响应（在 ResponseFunc 之前）
	mockHTTP.SetSuccessResponse("AI response")

	// 设置 HTTP 客户端前2次返回错误，第3次成功
	callCount := 0
	successResponse := mockHTTP.Response // 保存成功响应字符串
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount <= 2 {
			return nil, errors.New("timeout exceeded")
		}
		// 第3次返回成功响应
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(successResponse)),
			Header:     make(http.Header),
		}, nil
	}

	// 设置自定义重试等待基数为 1 秒（而不是默认的 2 秒）
	customWaitBase := 1 * time.Second

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
		WithRetryWaitBase(customWaitBase), // ✅ 设置自定义等待时间
		WithMaxRetries(3),
	)

	// 记录开始时间
	start := time.Now()

	// 调用 API
	_, err := client.CallWithMessages("system", "user")

	// 记录结束时间
	elapsed := time.Since(start)

	// 第3次成功，但前面失败了2次
	if err != nil {
		t.Fatalf("should succeed on 3rd attempt, got error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}

	// 验证等待时间
	// 第1次失败后等待 1s (customWaitBase * 1)
	// 第2次失败后等待 2s (customWaitBase * 2)
	// 总等待时间应该约为 3s (允许一些误差)
	expectedWait := 3 * time.Second
	tolerance := 200 * time.Millisecond

	if elapsed < expectedWait-tolerance || elapsed > expectedWait+tolerance {
		t.Errorf("expected total time ~%v (with RetryWaitBase=%v), got %v", expectedWait, customWaitBase, elapsed)
	}
}

func TestConfig_RetryableErrors_IsUsed(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// 自定义可重试错误列表（只包含 "custom error"）
	customRetryableErrors := []string{"custom error"}

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	c := client.(*Client)

	// 修改 config 的 RetryableErrors（暂时没有 WithRetryableErrors 选项）
	c.config.RetryableErrors = customRetryableErrors

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "custom error should be retryable",
			err:       errors.New("custom error occurred"),
			retryable: true,
		},
		{
			name:      "EOF should NOT be retryable (not in custom list)",
			err:       errors.New("unexpected EOF"),
			retryable: false,
		},
		{
			name:      "timeout should NOT be retryable (not in custom list)",
			err:       errors.New("timeout exceeded"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.isRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("expected isRetryableError(%v) = %v, got %v", tt.err, tt.retryable, result)
			}
		})
	}
}

// ============================================================
// 测试默认值
// ============================================================

func TestConfig_DefaultValues(t *testing.T) {
	client := NewClient()
	c := client.(*Client)

	// 验证默认值
	if c.config.MaxRetries != 3 {
		t.Errorf("default MaxRetries should be 3, got %d", c.config.MaxRetries)
	}

	if c.config.Temperature != 0.5 {
		t.Errorf("default Temperature should be 0.5, got %f", c.config.Temperature)
	}

	if c.config.RetryWaitBase != 2*time.Second {
		t.Errorf("default RetryWaitBase should be 2s, got %v", c.config.RetryWaitBase)
	}

	if len(c.config.RetryableErrors) == 0 {
		t.Error("default RetryableErrors should not be empty")
	}
}
