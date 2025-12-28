// Package experience handles product telemetry
package experience

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	telemetryEndpoint = "https://www.google-analytics.com/mp/collect"
	tid               = "G-14J8SY6F0J"
	tk                = "sgPLmshGTPiF-X57rzEIKA"
)

var (
	client     *Client
	clientOnce sync.Once
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

type Client struct {
	enabled        bool
	installationID string
	mu             sync.RWMutex
}

type TradeEvent struct {
	Exchange   string
	TradeType  string
	Symbol     string
	AmountUSD  float64
	Leverage   int
	UserID     string
	TraderID   string
}

type AIUsageEvent struct {
	UserID        string
	TraderID      string
	ModelProvider string // openai, deepseek, anthropic, etc.
	ModelName     string // gpt-4o, deepseek-chat, claude-3, etc.
	InputTokens   int
	OutputTokens  int
}

type telemetryPayload struct {
	ClientID string           `json:"client_id"`
	Events   []telemetryEvent `json:"events"`
}

type telemetryEvent struct {
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

func Init(enabled bool, installationID string) {
	clientOnce.Do(func() {
		client = &Client{
			enabled:        enabled,
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
	if client == nil {
		return
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	client.enabled = enabled
}

func IsEnabled() bool {
	if client == nil {
		return false
	}
	client.mu.RLock()
	defer client.mu.RUnlock()
	return client.enabled
}

func TrackTrade(event TradeEvent) {
	if client == nil || !IsEnabled() {
		return
	}

	// Send asynchronously to not block trading
	go func() {
		_ = sendTradeEvent(event)
	}()
}

// sendTradeEvent sends the trade event to GA4
func sendTradeEvent(event TradeEvent) error {
	client.mu.RLock()
	installationID := client.installationID
	client.mu.RUnlock()

	payload := telemetryPayload{
		ClientID: installationID,
		Events: []telemetryEvent{
			{
				Name: "trade",
				Params: map[string]interface{}{
					"exchange":             event.Exchange,
					"trade_type":           event.TradeType,
					"symbol":               event.Symbol,
					"amount_usd":           event.AmountUSD,
					"leverage":             event.Leverage,
					"installation_id":      installationID,  // For counting active installations
					"user_id":              event.UserID,    // For counting active users
					"trader_id":            event.TraderID,  // For counting active traders
					"engagement_time_msec": 1,               // Required by GA4
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := telemetryEndpoint + "?measurement_id=" + tid + "&api_secret=" + tk
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func TrackStartup(version string) {
	if client == nil || !IsEnabled() {
		return
	}

	go func() {
		client.mu.RLock()
		installationID := client.installationID
		client.mu.RUnlock()

		payload := telemetryPayload{
			ClientID: installationID,
			Events: []telemetryEvent{
				{
					Name: "app_startup",
					Params: map[string]interface{}{
						"version":              version,
						"installation_id":      installationID,
						"engagement_time_msec": 1,
					},
				},
			},
		}

		jsonData, _ := json.Marshal(payload)
		url := telemetryEndpoint + "?measurement_id=" + tid + "&api_secret=" + tk
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if req != nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}
	}()
}

func TrackAIUsage(event AIUsageEvent) {
	if client == nil || !IsEnabled() {
		return
	}

	go func() {
		client.mu.RLock()
		installationID := client.installationID
		client.mu.RUnlock()

		payload := telemetryPayload{
			ClientID: installationID,
			Events: []telemetryEvent{
				{
					Name: "ai_usage",
					Params: map[string]interface{}{
						"model_provider":       event.ModelProvider,
						"model_name":           event.ModelName,
						"input_tokens":         event.InputTokens,
						"output_tokens":        event.OutputTokens,
						"total_tokens":         event.InputTokens + event.OutputTokens,
						"installation_id":      installationID,
						"user_id":              event.UserID,
						"trader_id":            event.TraderID,
						"engagement_time_msec": 1,
					},
				},
			},
		}

		jsonData, _ := json.Marshal(payload)
		url := telemetryEndpoint + "?measurement_id=" + tid + "&api_secret=" + tk
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if req != nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}
	}()
}
