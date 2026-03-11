package session

import (
	"fmt"
	"nofx/mcp"
	"strings"
)

const (
	compactionThresholdTokens = 3000
	charsPerToken             = 3 // rough estimate for token counting
)

type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// Memory manages conversation history with automatic compaction.
// Inspired by openclaw's compaction pattern:
// when ShortTerm exceeds threshold, LLM silently summarizes it into LongTerm.
type Memory struct {
	LongTerm  string    // Durable summary (survives compaction, user never sees this happen)
	ShortTerm []Message // Recent conversation (cleared on compaction)
	llm       mcp.AIClient
}

func NewMemory(llm mcp.AIClient) *Memory {
	return &Memory{llm: llm}
}

// Add appends a message and triggers compaction if threshold exceeded
func (m *Memory) Add(role, content string) {
	m.ShortTerm = append(m.ShortTerm, Message{Role: role, Content: content})
	if m.estimateTokens() > compactionThresholdTokens {
		m.compact()
	}
}

// BuildContext returns context string for the agent's conversation history.
func (m *Memory) BuildContext() string {
	var sb strings.Builder
	if m.LongTerm != "" {
		sb.WriteString("[Summary of earlier conversation]\n")
		sb.WriteString(m.LongTerm)
		sb.WriteString("\n\n")
	}
	if len(m.ShortTerm) > 0 {
		sb.WriteString("[Recent conversation]\n")
		for _, msg := range m.ShortTerm {
			sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		}
	}
	return sb.String()
}

// Reset clears short-term history (LongTerm preserved intentionally)
func (m *Memory) Reset() {
	m.ShortTerm = []Message{}
}

// ResetFull clears everything including long-term memory
func (m *Memory) ResetFull() {
	m.ShortTerm = []Message{}
	m.LongTerm = ""
}

func (m *Memory) estimateTokens() int {
	total := len(m.LongTerm)
	for _, msg := range m.ShortTerm {
		total += len(msg.Content)
	}
	return total / charsPerToken
}

// compact summarizes short-term history into long-term memory.
// This runs silently - the user never sees it happen.
// If LLM call fails, short-term is preserved as-is (no data loss).
func (m *Memory) compact() {
	if m.llm == nil || len(m.ShortTerm) == 0 {
		return
	}
	history := m.BuildContext()
	systemPrompt := `You are a conversation summarizer. Compress the following trading assistant conversation into a concise summary.

Must preserve:
- What the user is configuring (strategy/exchange/model/trader)
- Confirmed parameters (trading pairs, leverage, stop loss, indicators, etc.)
- Pending or missing parameters
- User preferences and requirements

Output: plain text summary, under 200 words.`

	summary, err := m.llm.CallWithMessages(systemPrompt, history)
	if err != nil {
		// Compaction failed: keep short-term as-is, never lose user data
		return
	}
	if m.LongTerm != "" {
		m.LongTerm = m.LongTerm + "\n" + summary
	} else {
		m.LongTerm = summary
	}
	m.ShortTerm = []Message{}
}
