package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 阿里云 API 配置
const (
	DefaultQwenBaseURL = "https://dashscope.aliyuncs.com/api/v1/apps"
	// 标准 OpenAI 兼容模式 API
	QwenCompatibleURL = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
)

// QwenAgent 阿里云百炼智能体客户端
type QwenAgent struct {
	AppID     string
	APIKey    string
	BaseURL   string
	SessionID string
	Client    *http.Client
}

// QwenRequest 请求结构
type QwenRequest struct {
	Input      QwenInput      `json:"input"`
	Parameters QwenParameters `json:"parameters,omitempty"`
}

// QwenInput 输入结构
type QwenInput struct {
	Prompt    string                 `json:"prompt"`
	BizParams map[string]interface{} `json:"biz_params,omitempty"`
}

// QwenParameters 参数结构
type QwenParameters struct {
	SessionID         string `json:"session_id,omitempty"`
	IncrementalOutput bool   `json:"incremental_output,omitempty"`
}

// QwenResponse 响应结构
type QwenResponse struct {
	Output    QwenOutput `json:"output"`
	Usage     QwenUsage  `json:"usage,omitempty"`
	RequestID string     `json:"request_id"`
	Code      string     `json:"code,omitempty"`
	Message   string     `json:"message,omitempty"`
}

// QwenOutput 输出结构
type QwenOutput struct {
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
}

// QwenUsage 用量统计
type QwenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// NewQwenAgent 创建新的智能体客户端
func NewQwenAgent(appID, apiKey string) *QwenAgent {
	return &QwenAgent{
		AppID:   appID,
		APIKey:  apiKey,
		BaseURL: DefaultQwenBaseURL,
		Client: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

// Chat 同步对话
func (a *QwenAgent) Chat(ctx context.Context, prompt string) (*QwenResponse, error) {
	reqBody := QwenRequest{
		Input: QwenInput{
			Prompt: prompt,
		},
		Parameters: QwenParameters{
			SessionID: a.SessionID,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	url := fmt.Sprintf("%s/%s/completion", a.BaseURL, a.AppID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.APIKey)

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result QwenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w, body: %s", err, string(body))
	}

	// 更新 session_id 用于多轮对话
	if result.Output.SessionID != "" {
		a.SessionID = result.Output.SessionID
	}

	// 检查 API 错误
	if result.Code != "" {
		return &result, fmt.Errorf("API error: code=%s, message=%s", result.Code, result.Message)
	}

	return &result, nil
}

// ChatStream 流式对话
func (a *QwenAgent) ChatStream(ctx context.Context, prompt string, callback func(chunk string)) error {
	reqBody := QwenRequest{
		Input: QwenInput{
			Prompt: prompt,
		},
		Parameters: QwenParameters{
			SessionID:         a.SessionID,
			IncrementalOutput: true,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	url := fmt.Sprintf("%s/%s/completion", a.BaseURL, a.AppID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("X-DashScope-SSE", "enable")

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read stream failed: %w", err)
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		var chunk QwenResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// 更新 session_id
		if chunk.Output.SessionID != "" {
			a.SessionID = chunk.Output.SessionID
		}

		// 回调输出文本
		if chunk.Output.Text != "" {
			callback(chunk.Output.Text)
		}
	}

	return nil
}

// ChatWithBizParams 带业务参数的对话
func (a *QwenAgent) ChatWithBizParams(ctx context.Context, prompt string, bizParams map[string]interface{}) (*QwenResponse, error) {
	reqBody := QwenRequest{
		Input: QwenInput{
			Prompt:    prompt,
			BizParams: bizParams,
		},
		Parameters: QwenParameters{
			SessionID: a.SessionID,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	url := fmt.Sprintf("%s/%s/completion", a.BaseURL, a.AppID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.APIKey)

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result QwenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w, body: %s", err, string(body))
	}

	if result.Output.SessionID != "" {
		a.SessionID = result.Output.SessionID
	}

	if result.Code != "" {
		return &result, fmt.Errorf("API error: code=%s, message=%s", result.Code, result.Message)
	}

	return &result, nil
}

// ResetSession 重置会话
func (a *QwenAgent) ResetSession() {
	a.SessionID = ""
}

// ========== 标准 OpenAI 兼容 API ==========

// ChatCompletionRequest OpenAI 兼容格式请求
type ChatCompletionRequest struct {
	Model    string                   `json:"model"`
	Messages []ChatCompletionMessage  `json:"messages"`
}

// ChatCompletionMessage 消息结构
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse OpenAI 兼容格式响应
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ChatWithModel 使用标准 OpenAI 兼容 API 调用指定模型
func (a *QwenAgent) ChatWithModel(ctx context.Context, model, prompt string) (*ChatCompletionResponse, error) {
	reqBody := ChatCompletionRequest{
		Model: model,
		Messages: []ChatCompletionMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", QwenCompatibleURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.APIKey)

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w, body: %s", err, string(body))
	}

	if result.Error != nil {
		return &result, fmt.Errorf("API error: code=%s, message=%s", result.Error.Code, result.Error.Message)
	}

	return &result, nil
}

// GetContent 从响应中获取内容
func (r *ChatCompletionResponse) GetContent() string {
	if len(r.Choices) > 0 {
		return r.Choices[0].Message.Content
	}
	return ""
}
