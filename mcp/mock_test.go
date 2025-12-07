package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// ============================================================
// Mock Logger
// ============================================================

// MockLogger Mock logger (for testing)
type MockLogger struct {
	mu      sync.Mutex
	Logs    []LogEntry
	Enabled bool // Whether logging is enabled
}

// LogEntry log entry
type LogEntry struct {
	Level   string
	Format  string
	Args    []any
	Message string // Formatted message
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		Logs:    make([]LogEntry, 0),
		Enabled: true,
	}
}

func (m *MockLogger) Debugf(format string, args ...any) {
	m.log("DEBUG", format, args...)
}

func (m *MockLogger) Infof(format string, args ...any) {
	m.log("INFO", format, args...)
}

func (m *MockLogger) Warnf(format string, args ...any) {
	m.log("WARN", format, args...)
}

func (m *MockLogger) Errorf(format string, args ...any) {
	m.log("ERROR", format, args...)
}

func (m *MockLogger) log(level, format string, args ...any) {
	if !m.Enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	message := fmt.Sprintf(format, args...)
	m.Logs = append(m.Logs, LogEntry{
		Level:   level,
		Format:  format,
		Args:    args,
		Message: message,
	})
}

// GetLogs gets all logs
func (m *MockLogger) GetLogs() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]LogEntry{}, m.Logs...)
}

// GetLogsByLevel gets logs by specified level
func (m *MockLogger) GetLogsByLevel(level string) []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []LogEntry
	for _, log := range m.Logs {
		if log.Level == level {
			result = append(result, log)
		}
	}
	return result
}

// Clear clears all logs
func (m *MockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logs = make([]LogEntry, 0)
}

// HasLog checks if contains specified message
func (m *MockLogger) HasLog(level, message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, log := range m.Logs {
		if log.Level == level && log.Message == message {
			return true
		}
	}
	return false
}

// ============================================================
// Mock HTTP Client (implements http.RoundTripper)
// ============================================================

// MockHTTPClient Mock HTTP client (implements http.RoundTripper)
type MockHTTPClient struct {
	mu sync.Mutex

	// Configuration
	Response     string
	StatusCode   int
	Error        error
	ResponseFunc func(req *http.Request) (*http.Response, error) // Custom response function

	// Recording
	Requests []*http.Request
}

func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		StatusCode: http.StatusOK,
		Requests:   make([]*http.Request, 0),
	}
}

// ToHTTPClient converts to http.Client
func (m *MockHTTPClient) ToHTTPClient() *http.Client {
	return &http.Client{
		Transport: m,
	}
}

// RoundTrip implements http.RoundTripper interface
func (m *MockHTTPClient) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record request
	m.Requests = append(m.Requests, req)

	// If custom response function exists, use it
	if m.ResponseFunc != nil {
		return m.ResponseFunc(req)
	}

	// If error is set, return error
	if m.Error != nil {
		return nil, m.Error
	}

	// Return mock response
	resp := &http.Response{
		StatusCode: m.StatusCode,
		Body:       io.NopCloser(bytes.NewBufferString(m.Response)),
		Header:     make(http.Header),
	}

	return resp, nil
}

// GetRequests gets all requests
func (m *MockHTTPClient) GetRequests() []*http.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*http.Request{}, m.Requests...)
}

// GetLastRequest gets last request
func (m *MockHTTPClient) GetLastRequest() *http.Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.Requests) == 0 {
		return nil
	}
	return m.Requests[len(m.Requests)-1]
}

// Reset resets state
func (m *MockHTTPClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Requests = make([]*http.Request, 0)
}

// SetSuccessResponse sets success response
func (m *MockHTTPClient) SetSuccessResponse(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StatusCode = http.StatusOK
	m.Response = `{"choices":[{"message":{"content":"` + content + `"}}]}`
	m.Error = nil
}

// SetErrorResponse sets error response
func (m *MockHTTPClient) SetErrorResponse(statusCode int, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StatusCode = statusCode
	m.Response = message
	m.Error = nil
}

// SetNetworkError sets network error
func (m *MockHTTPClient) SetNetworkError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Error = err
}

// ============================================================
// Mock Client Hooks (for testing hook mechanism)
// ============================================================

// MockClientHooks Mock client hooks
type MockClientHooks struct {
	BuildRequestBodyCalled int
	BuildUrlCalled         int
	SetAuthHeaderCalled    int
	MarshalRequestCalled   int
	ParseResponseCalled    int
	IsRetryableErrorCalled int

	// Custom return values
	BuildUrlFunc           func() string
	ParseResponseFunc      func([]byte) (string, error)
	IsRetryableErrorFunc   func(error) bool
	BuildRequestBodyFunc   func(string, string) map[string]any
	MarshalRequestBodyFunc func(map[string]any) ([]byte, error)
}

func NewMockClientHooks() *MockClientHooks {
	return &MockClientHooks{}
}

func (m *MockClientHooks) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	m.BuildRequestBodyCalled++
	if m.BuildRequestBodyFunc != nil {
		return m.BuildRequestBodyFunc(systemPrompt, userPrompt)
	}
	return map[string]any{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
}

func (m *MockClientHooks) buildUrl() string {
	m.BuildUrlCalled++
	if m.BuildUrlFunc != nil {
		return m.BuildUrlFunc()
	}
	return "https://api.test.com/chat/completions"
}

func (m *MockClientHooks) setAuthHeader(headers http.Header) {
	m.SetAuthHeaderCalled++
	headers.Set("Authorization", "Bearer test-key")
}

func (m *MockClientHooks) marshalRequestBody(body map[string]any) ([]byte, error) {
	m.MarshalRequestCalled++
	if m.MarshalRequestBodyFunc != nil {
		return m.MarshalRequestBodyFunc(body)
	}
	return json.Marshal(body)
}

func (m *MockClientHooks) parseMCPResponse(body []byte) (string, error) {
	m.ParseResponseCalled++
	if m.ParseResponseFunc != nil {
		return m.ParseResponseFunc(body)
	}
	return "mocked response", nil
}

func (m *MockClientHooks) isRetryableError(err error) bool {
	m.IsRetryableErrorCalled++
	if m.IsRetryableErrorFunc != nil {
		return m.IsRetryableErrorFunc(err)
	}
	return false
}

func (m *MockClientHooks) buildRequest(url string, jsonData []byte) (*http.Request, error) {
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	m.setAuthHeader(req.Header)
	return req, nil
}

func (m *MockClientHooks) call(systemPrompt, userPrompt string) (string, error) {
	return "mocked call result", nil
}
