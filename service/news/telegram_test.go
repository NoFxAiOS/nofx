package news

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

// MockRoundTripper 用于模拟 http.Client 的请求
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func TestTelegramNotifier_Send(t *testing.T) {
	botToken := "test_token"
	chatID := "123456"
	msgContent := "<b>Test Message</b>"

	tests := []struct {
		name            string
		messageThreadID int
		mockRespCode    int
		mockRespBody    string
		mockErr         error
		expectedErr     bool
		validateReq     func(t *testing.T, req *http.Request)
	}{
		{
			name:            "Success",
			messageThreadID: 0,
			mockRespCode:    http.StatusOK,
			mockRespBody:    `{"ok":true, "result":{}}`,
			expectedErr:     false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.Method != "POST" {
					t.Errorf("expected POST method, got %s", req.Method)
				}
				expectedURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"
				if req.URL.String() != expectedURL {
					t.Errorf("expected URL %s, got %s", expectedURL, req.URL.String())
				}
				
				var body map[string]interface{}
				json.NewDecoder(req.Body).Decode(&body)
				
				if body["chat_id"] != chatID {
					t.Errorf("expected chat_id %s, got %v", chatID, body["chat_id"])
				}
				if body["text"] != msgContent {
					t.Errorf("expected text %s, got %v", msgContent, body["text"])
				}
				if _, ok := body["message_thread_id"]; ok {
					t.Errorf("did not expect message_thread_id in payload")
				}
			},
		},
		{
			name:            "SuccessWithThreadID",
			messageThreadID: 999,
			mockRespCode:    http.StatusOK,
			mockRespBody:    `{"ok":true, "result":{}}`,
			expectedErr:     false,
			validateReq: func(t *testing.T, req *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(req.Body).Decode(&body)
				
				// JSON numbers are often unmarshaled as float64 in generic maps
				tid, ok := body["message_thread_id"].(float64)
				if !ok || int(tid) != 999 {
					t.Errorf("expected message_thread_id 999, got %v", body["message_thread_id"])
				}
			},
		},
		{
			name:         "APIError",
			mockRespCode: http.StatusBadRequest,
			mockRespBody: `{"ok":false, "error_code": 400, "description": "Bad Request: chat not found"}`,
			expectedErr:  true,
		},
		{
			name:        "NetworkError",
			mockErr:     errors.New("network timeout"),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := NewTelegramNotifier(botToken, chatID)
			
			// Replace the Transport with our mock
			notifier.client.Transport = &MockRoundTripper{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					if tt.validateReq != nil {
						tt.validateReq(t, req)
					}
					
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}

					return &http.Response{
						StatusCode: tt.mockRespCode,
						Status:     http.StatusText(tt.mockRespCode),
						Body:       io.NopCloser(bytes.NewBufferString(tt.mockRespBody)),
						Header:     make(http.Header),
					}, nil
				},
			}

			err := notifier.Send(msgContent, tt.messageThreadID)

			if (err != nil) != tt.expectedErr {
				t.Errorf("Send() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}
