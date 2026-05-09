package trader

import (
	"sync"
	"time"
)

const defaultEntryCooldownDuration = 90 * time.Minute

type entryCooldownManager struct {
	mu        sync.Mutex
	cooldowns map[string]time.Time
}

func newEntryCooldownManager() *entryCooldownManager {
	return &entryCooldownManager{
		cooldowns: make(map[string]time.Time),
	}
}

func (m *entryCooldownManager) SetCooldown(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cooldowns[symbol] = time.Now().Add(defaultEntryCooldownDuration)
}

func (m *entryCooldownManager) IsCoolingDown(symbol string) (bool, time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	until, ok := m.cooldowns[symbol]
	if !ok {
		return false, 0
	}
	remaining := time.Until(until)
	if remaining <= 0 {
		delete(m.cooldowns, symbol)
		return false, 0
	}
	return true, remaining
}

func (m *entryCooldownManager) Clear(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cooldowns, symbol)
}

func (m *entryCooldownManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for symbol, until := range m.cooldowns {
		if now.After(until) {
			delete(m.cooldowns, symbol)
		}
	}
}
