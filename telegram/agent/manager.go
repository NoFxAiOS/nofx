package agent

import (
	"nofx/logger"
	"nofx/mcp"
	"sync"
	"time"
)

// Manager holds one Agent per Telegram chat ID.
// Messages for the same chat are serialized (OpenClaw Lane Queue pattern).
type Manager struct {
	mu           sync.Mutex
	agents       map[int64]*Agent
	lanes        map[int64]chan struct{}
	apiPort      int
	botToken     string
	userID       string
	getLLM       func() mcp.AIClient
	systemPrompt string
}

// NewManager creates a Manager. Call api.GetAPIDocs() before this and pass the result as apiDocs.
// userEmail is the registered email shown to the user when they ask "who am I".
// userID is the internal DB UUID used for API authentication.
func NewManager(apiPort int, botToken, userEmail, userID string, getLLM func() mcp.AIClient, apiDocs string) *Manager {
	return &Manager{
		agents:       make(map[int64]*Agent),
		lanes:        make(map[int64]chan struct{}),
		apiPort:      apiPort,
		botToken:     botToken,
		userID:       userID,
		getLLM:       getLLM,
		systemPrompt: BuildAgentPrompt(apiDocs, userEmail, userID),
	}
}

// Run processes a message for the given chat ID.
// If the same chat is already processing a message, this call blocks until it completes
// or the lane wait times out (60 s), whichever comes first.
// onChunk is optional — when set, LLM reply chunks are forwarded progressively (SSE streaming).
func (m *Manager) Run(chatID int64, userMessage string, onChunk func(string)) string {
	a, lane := m.getOrCreate(chatID)
	select {
	case lane <- struct{}{}:
	case <-time.After(60 * time.Second):
		logger.Warnf("Agent: lane wait timeout for chat %d — previous message still processing", chatID)
		return "上一条消息仍在处理中，请稍等片刻后再试。"
	}
	defer func() { <-lane }()
	return a.Run(userMessage, onChunk)
}

// Reset clears memory for the given chat (called on /start).
func (m *Manager) Reset(chatID int64) {
	m.mu.Lock()
	a, ok := m.agents[chatID]
	m.mu.Unlock()
	if ok {
		a.ResetMemory()
	}
}

func (m *Manager) getOrCreate(chatID int64) (*Agent, chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.agents[chatID]
	if !ok {
		a = New(m.apiPort, m.botToken, m.userID, m.getLLM, m.systemPrompt)
		m.agents[chatID] = a
	}
	lane, ok := m.lanes[chatID]
	if !ok {
		lane = make(chan struct{}, 1) // binary semaphore: one message at a time per chat
		m.lanes[chatID] = lane
	}
	return a, lane
}
