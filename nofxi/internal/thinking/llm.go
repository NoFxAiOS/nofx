package thinking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMEngine implements Engine using an OpenAI-compatible API.
// Works with OpenAI, claw402 (x402), DeepSeek, Dashscope (Qwen), etc.
type LLMEngine struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewLLMEngine creates a new LLM-backed thinking engine.
func NewLLMEngine(baseURL, apiKey, model string) *LLMEngine {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &LLMEngine{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// chatRequest is the OpenAI chat completions request body.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// chatResponse handles both standard and thinking-mode responses.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          *string `json:"content"`           // Can be null in thinking mode
			ReasoningContent string  `json:"reasoning_content"` // Qwen3 thinking mode
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends messages to the LLM and returns the response.
func (e *LLMEngine) Chat(ctx context.Context, messages []Message) (string, error) {
	reqBody := chatRequest{
		Model:    e.model,
		Messages: messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	// Extract content — handle thinking mode where content can be null
	choice := chatResp.Choices[0]
	content := ""
	if choice.Message.Content != nil {
		content = *choice.Message.Content
	}

	// If content is empty but reasoning_content exists, use that
	if content == "" && choice.Message.ReasoningContent != "" {
		content = choice.Message.ReasoningContent
	}

	if content == "" {
		return "🤔 (AI returned empty response)", nil
	}

	return content, nil
}

// Analyze sends an analysis prompt and parses the AI response.
func (e *LLMEngine) Analyze(ctx context.Context, prompt string) (*Analysis, error) {
	systemPrompt := `You are NOFXi, an expert AI trading analyst. Analyze the given market data and provide a trading recommendation.

Respond in JSON format:
{
  "action": "buy|sell|hold|wait",
  "symbol": "BTC/USDT",
  "confidence": 0.85,
  "reasoning": "Brief explanation",
  "stop_loss": 0.0,
  "take_profit": 0.0
}

Be concise. Only recommend high-confidence trades.`

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	resp, err := e.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	var analysis Analysis
	if err := json.Unmarshal([]byte(resp), &analysis); err != nil {
		// If JSON parsing fails, return the raw text as reasoning
		return &Analysis{
			Action:    "hold",
			Reasoning: resp,
		}, nil
	}

	return &analysis, nil
}
