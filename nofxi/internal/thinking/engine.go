package thinking

import "context"

// Engine is the AI decision-making interface.
type Engine interface {
	// Chat sends a message to the LLM with conversation context and returns a response.
	Chat(ctx context.Context, messages []Message) (string, error)

	// Analyze asks the AI to analyze market data and provide a trading recommendation.
	Analyze(ctx context.Context, prompt string) (*Analysis, error)
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// Analysis holds an AI trading recommendation.
type Analysis struct {
	Action     string  `json:"action"`      // "buy", "sell", "hold", "wait"
	Symbol     string  `json:"symbol"`
	Confidence float64 `json:"confidence"`  // 0.0 - 1.0
	Reasoning  string  `json:"reasoning"`
	StopLoss   float64 `json:"stop_loss,omitempty"`
	TakeProfit float64 `json:"take_profit,omitempty"`
}
