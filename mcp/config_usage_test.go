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
// Test Config Fields Are Actually Used (Verify Issue 2 Fix)
// ============================================================

func TestConfig_MaxRetries_IsUsed(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// Set HTTP client to return error
	callCount := 0
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		callCount++
		return nil, errors.New("connection reset")
	}

	// Create client and set custom retry count to 5
	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
		WithMaxRetries(5), // Set to retry 5 times
	)

	// Call API (should fail)
	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Error("should error")
	}

	// Verify indeed retried 5 times (not the default 3 times)
	if callCount != 5 {
		t.Errorf("expected 5 retry attempts (from WithMaxRetries(5)), got %d", callCount)
	}

	// Verify logs show correct retry count
	logs := mockLogger.GetLogsByLevel("WARN")
	expectedWarningCount := 4 // Warnings will be printed on 2nd, 3rd, 4th, 5th retry
	actualWarningCount := 0
	for _, log := range logs {
		if log.Message == "⚠️  AI API call failed, retrying (2/5)..." ||
			log.Message == "⚠️  AI API call failed, retrying (3/5)..." ||
			log.Message == "⚠️  AI API call failed, retrying (4/5)..." ||
			log.Message == "⚠️  AI API call failed, retrying (5/5)..." {
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

	// Create client and set custom temperature
	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
		WithTemperature(customTemperature), // Set custom temperature
	)

	c := client.(*Client)

	// Build request body
	requestBody := c.buildMCPRequestBody("system", "user")

	// Verify temperature field
	temp, ok := requestBody["temperature"].(float64)
	if !ok {
		t.Fatal("temperature should be float64")
	}

	if temp != customTemperature {
		t.Errorf("expected temperature %f (from WithTemperature), got %f", customTemperature, temp)
	}

	// Can also verify through actual HTTP request
	_, err := client.CallWithMessages("system", "user")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	// Check sent request body
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	// Parse request body
	var body map[string]interface{}
	decoder := json.NewDecoder(requests[0].Body)
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	// Verify temperature
	if body["temperature"] != customTemperature {
		t.Errorf("expected temperature %f in HTTP request, got %v", customTemperature, body["temperature"])
	}
}

func TestConfig_RetryWaitBase_IsUsed(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// Set success response (before ResponseFunc)
	mockHTTP.SetSuccessResponse("AI response")

	// Set HTTP client to return error first 2 times, success on 3rd time
	callCount := 0
	successResponse := mockHTTP.Response // Save success response string
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount <= 2 {
			return nil, errors.New("timeout exceeded")
		}
		// 3rd time return success response
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(successResponse)),
			Header:     make(http.Header),
		}, nil
	}

	// Set custom retry wait base to 1 second (instead of default 2 seconds)
	customWaitBase := 1 * time.Second

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
		WithRetryWaitBase(customWaitBase), // Set custom wait time
		WithMaxRetries(3),
	)

	// Record start time
	start := time.Now()

	// Call API
	_, err := client.CallWithMessages("system", "user")

	// Record end time
	elapsed := time.Since(start)

	// 3rd time succeeds, but failed 2 times before
	if err != nil {
		t.Fatalf("should succeed on 3rd attempt, got error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}

	// Verify wait time
	// After 1st failure wait 1s (customWaitBase * 1)
	// After 2nd failure wait 2s (customWaitBase * 2)
	// Total wait time should be about 3s (allow some error)
	expectedWait := 3 * time.Second
	tolerance := 200 * time.Millisecond

	if elapsed < expectedWait-tolerance || elapsed > expectedWait+tolerance {
		t.Errorf("expected total time ~%v (with RetryWaitBase=%v), got %v", expectedWait, customWaitBase, elapsed)
	}
}

func TestConfig_RetryableErrors_IsUsed(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// Custom retryable error list (only contains "custom error")
	customRetryableErrors := []string{"custom error"}

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	c := client.(*Client)

	// Modify config's RetryableErrors (no WithRetryableErrors option yet)
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
// Test Default Values
// ============================================================

func TestConfig_DefaultValues(t *testing.T) {
	client := NewClient()
	c := client.(*Client)

	// Verify default values
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
