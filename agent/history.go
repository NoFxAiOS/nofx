package agent

import (
	"sync"
	"time"
)

// chatMessage represents a single message in conversation history.
type chatMessage struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// chatHistory stores conversation history per user.
type chatHistory struct {
	mu       sync.RWMutex
	sessions map[int64][]chatMessage
	maxTurns int // max messages per user (user+assistant pairs)
}

func newChatHistory(maxTurns int) *chatHistory {
	if maxTurns <= 0 {
		maxTurns = 20 // default: keep last 20 messages (10 turns)
	}
	return &chatHistory{
		sessions: make(map[int64][]chatMessage),
		maxTurns: maxTurns,
	}
}

// Add appends a message to the user's history.
func (h *chatHistory) Add(userID int64, role, content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sessions[userID] = append(h.sessions[userID], chatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})

	// Trim to maxTurns
	msgs := h.sessions[userID]
	if len(msgs) > h.maxTurns {
		h.sessions[userID] = msgs[len(msgs)-h.maxTurns:]
	}
}

// Get returns the conversation history for a user.
func (h *chatHistory) Get(userID int64) []chatMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()

	msgs := h.sessions[userID]
	if msgs == nil {
		return nil
	}
	// Return a copy
	result := make([]chatMessage, len(msgs))
	copy(result, msgs)
	return result
}

// Clear resets conversation history for a user.
func (h *chatHistory) Clear(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, userID)
}

// CleanOld removes sessions older than the given duration.
func (h *chatHistory) CleanOld(maxAge time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for uid, msgs := range h.sessions {
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			if now.Sub(lastMsg.Timestamp) > maxAge {
				delete(h.sessions, uid)
			}
		}
	}
}
