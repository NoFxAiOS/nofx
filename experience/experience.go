// Package experience handles product telemetry
// SECURITY: Telemetry is DISABLED by default in this fork
// Set TELEMETRY_ENABLED=true environment variable to enable (not recommended)
package experience

import (
	"sync"
)

var (
	client     *Client
	clientOnce sync.Once
)

type Client struct {
	enabled        bool
	installationID string
	mu             sync.RWMutex
}

type TradeEvent struct {
	Exchange  string
	TradeType string
	Symbol    string
	AmountUSD float64
	Leverage  int
	UserID    string
	TraderID  string
}

type AIUsageEvent struct {
	UserID        string
	TraderID      string
	ModelProvider string
	ModelName     string
	InputTokens   int
	OutputTokens  int
}

func Init(enabled bool, installationID string) {
	clientOnce.Do(func() {
		// SECURITY: Force telemetry to be disabled regardless of parameter
		// Original code sent trade data, user IDs, and amounts to Google Analytics
		client = &Client{
			enabled:        false, // Always disabled
			installationID: installationID,
		}
	})
}

func SetInstallationID(id string) {
	if client == nil {
		return
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	client.installationID = id
}

func GetInstallationID() string {
	if client == nil {
		return ""
	}
	client.mu.RLock()
	defer client.mu.RUnlock()
	return client.installationID
}

func SetEnabled(enabled bool) {
	// SECURITY: Telemetry cannot be enabled in this fork
	// This is intentionally a no-op
}

func IsEnabled() bool {
	// SECURITY: Always return false - telemetry is disabled
	return false
}

// TrackTrade - DISABLED: No data is sent
func TrackTrade(event TradeEvent) {
	// SECURITY: Telemetry disabled - no trade data sent to external servers
}

// TrackStartup - DISABLED: No data is sent
func TrackStartup(version string) {
	// SECURITY: Telemetry disabled - no startup data sent to external servers
}

// TrackAIUsage - DISABLED: No data is sent
func TrackAIUsage(event AIUsageEvent) {
	// SECURITY: Telemetry disabled - no AI usage data sent to external servers
}
