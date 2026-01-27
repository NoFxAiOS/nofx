package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	wxPusherBaseURL = "https://wxpusher.zjiecode.com/api/send/message"

	// ContentType constants
	ContentTypeText     = 1
	ContentTypeHTML     = 2
	ContentTypeMarkdown = 3

	// VerifyPayType constants
	VerifyPayTypeNone   = 0 // No verification
	VerifyPayTypePaid   = 1 // Only paid users
	VerifyPayTypeUnpaid = 2 // Only unpaid users
)

// WxPusherClient WxPusher client for sending messages
type WxPusherClient struct {
	appToken   string
	httpClient *http.Client
}

// NewWxPusherClient creates a new WxPusher client
func NewWxPusherClient(appToken string) *WxPusherClient {
	return &WxPusherClient{
		appToken: appToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// MessageRequest WxPusher message request
type MessageRequest struct {
	AppToken       string   `json:"appToken"`
	Content        string   `json:"content"`
	Summary        string   `json:"summary,omitempty"`
	ContentType    int      `json:"contentType,omitempty"`
	TopicIds       []int    `json:"topicIds,omitempty"`
	UIDs           []string `json:"uids,omitempty"`
	URL            string   `json:"url,omitempty"`
	VerifyPayType  int      `json:"verifyPayType,omitempty"`
}

// MessageResponse WxPusher message response
type MessageResponse struct {
	Code    int              `json:"code"`
	Msg     string           `json:"msg"`
	Data    []MessageData    `json:"data"`
	Success bool             `json:"success"`
}

// MessageData message data for each user/topic
type MessageData struct {
	UID                string `json:"uid"`
	TopicID            *int   `json:"topicId"`
	MessageID          int    `json:"messageId"`
	MessageContentID   int    `json:"messageContentId"`
	SendRecordID       int    `json:"sendRecordId"`
	Code               int    `json:"code"`
	Status             string `json:"status"`
}

// SendMessage sends a message to users/topics
func (c *WxPusherClient) SendMessage(content string, uids []string, topicIds []int, opts ...MessageOption) ([]MessageData, error) {
	if c.appToken == "" {
		return nil, fmt.Errorf("wxpusher app token is not set")
	}

	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	if len(uids) == 0 && len(topicIds) == 0 {
		return nil, fmt.Errorf("at least one uid or topicId is required")
	}

	req := &MessageRequest{
		AppToken:      c.appToken,
		Content:       content,
		UIDs:          uids,
		TopicIds:      topicIds,
		ContentType:   ContentTypeHTML, // Default to HTML
		VerifyPayType: VerifyPayTypeNone,
	}

	// Apply options
	for _, opt := range opts {
		opt(req)
	}

	// Set default summary if not provided
	if req.Summary == "" {
		req.Summary = truncateString(stripHTML(content), 20)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", wxPusherBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var msgResp MessageResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if msgResp.Code != 1000 {
		return nil, fmt.Errorf("wxpusher error: code=%d, msg=%s", msgResp.Code, msgResp.Msg)
	}

	return msgResp.Data, nil
}

// SendToUIDs sends a message to specific users
func (c *WxPusherClient) SendToUIDs(content string, uids []string, opts ...MessageOption) ([]MessageData, error) {
	return c.SendMessage(content, uids, nil, opts...)
}

// SendToTopics sends a message to specific topics
func (c *WxPusherClient) SendToTopics(content string, topicIds []int, opts ...MessageOption) ([]MessageData, error) {
	return c.SendMessage(content, nil, topicIds, opts...)
}

// MessageOption is a functional option for MessageRequest
type MessageOption func(*MessageRequest)

// WithSummary sets the message summary
func WithSummary(summary string) MessageOption {
	return func(req *MessageRequest) {
		req.Summary = summary
	}
}

// WithContentType sets the content type
func WithContentType(contentType int) MessageOption {
	return func(req *MessageRequest) {
		req.ContentType = contentType
	}
}

// WithURL sets the message URL
func WithURL(url string) MessageOption {
	return func(req *MessageRequest) {
		req.URL = url
	}
}

// WithVerifyPayType sets the verify pay type
func WithVerifyPayType(verifyPayType int) MessageOption {
	return func(req *MessageRequest) {
		req.VerifyPayType = verifyPayType
	}
}

// Helper functions

// truncateString truncates a string to specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// stripHTML removes HTML tags from a string
func stripHTML(s string) string {
	// Simple HTML tag removal - more complex parsing can be added if needed
	var result []byte
	inTag := false

	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			inTag = true
		} else if s[i] == '>' {
			inTag = false
		} else if !inTag {
			result = append(result, s[i])
		}
	}

	return string(result)
}

// IsValidAppToken checks if app token is set
func (c *WxPusherClient) IsValidAppToken() bool {
	return c.appToken != ""
}
