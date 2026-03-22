package perception

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// PriceAlert triggers when a symbol hits a threshold.
type PriceAlert struct {
	Symbol    string
	Price     float64
	Direction string // "above" or "below"
	Threshold float64
	Triggered time.Time
}

// AlertCallback is called when a price alert triggers.
type AlertCallback func(alert PriceAlert)

// MarketMonitor watches symbols and detects price anomalies.
type MarketMonitor struct {
	mu          sync.RWMutex
	watching    map[string]*watchState
	alerts      []alertRule
	onAlert     AlertCallback
	httpClient  *http.Client
	logger      *slog.Logger
	stopCh      chan struct{}
	pollInterval time.Duration
}

type watchState struct {
	Symbol    string
	LastPrice float64
	Change1h  float64
	Change24h float64
	UpdatedAt time.Time
}

type alertRule struct {
	Symbol    string
	Direction string  // "above" or "below"
	Threshold float64
	Fired     bool
}

// NewMarketMonitor creates a new market monitor.
func NewMarketMonitor(onAlert AlertCallback, logger *slog.Logger) *MarketMonitor {
	return &MarketMonitor{
		watching:     make(map[string]*watchState),
		onAlert:      onAlert,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		logger:       logger,
		stopCh:       make(chan struct{}),
		pollInterval: 30 * time.Second,
	}
}

// Watch starts monitoring a symbol.
func (m *MarketMonitor) Watch(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watching[symbol] = &watchState{Symbol: symbol}
	m.logger.Info("watching symbol", "symbol", symbol)
}

// Unwatch stops monitoring a symbol.
func (m *MarketMonitor) Unwatch(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.watching, symbol)
}

// AddAlert adds a price alert rule.
func (m *MarketMonitor) AddAlert(symbol, direction string, threshold float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, alertRule{
		Symbol:    symbol,
		Direction: direction,
		Threshold: threshold,
	})
	m.logger.Info("alert added", "symbol", symbol, "direction", direction, "threshold", threshold)
}

// GetSnapshot returns the latest state for a symbol.
func (m *MarketMonitor) GetSnapshot(symbol string) (*watchState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.watching[symbol]
	return s, ok
}

// GetAllSnapshots returns all watched symbols.
func (m *MarketMonitor) GetAllSnapshots() map[string]*watchState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*watchState)
	for k, v := range m.watching {
		result[k] = v
	}
	return result
}

// Start begins the monitoring loop.
func (m *MarketMonitor) Start() {
	go m.loop()
}

// Stop stops the monitoring loop.
func (m *MarketMonitor) Stop() {
	close(m.stopCh)
}

func (m *MarketMonitor) loop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Initial fetch
	m.updatePrices()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.updatePrices()
		}
	}
}

func (m *MarketMonitor) updatePrices() {
	m.mu.RLock()
	symbols := make([]string, 0, len(m.watching))
	for s := range m.watching {
		symbols = append(symbols, s)
	}
	m.mu.RUnlock()

	for _, symbol := range symbols {
		price, err := m.fetchPrice(symbol)
		if err != nil {
			m.logger.Error("fetch price", "symbol", symbol, "error", err)
			continue
		}

		m.mu.Lock()
		if state, ok := m.watching[symbol]; ok {
			state.LastPrice = price
			state.UpdatedAt = time.Now()
		}

		// Check alerts
		for i := range m.alerts {
			a := &m.alerts[i]
			if a.Symbol != symbol || a.Fired {
				continue
			}
			triggered := false
			if a.Direction == "above" && price >= a.Threshold {
				triggered = true
			} else if a.Direction == "below" && price <= a.Threshold {
				triggered = true
			}
			if triggered {
				a.Fired = true
				if m.onAlert != nil {
					m.onAlert(PriceAlert{
						Symbol:    symbol,
						Price:     price,
						Direction: a.Direction,
						Threshold: a.Threshold,
						Triggered: time.Now(),
					})
				}
			}
		}
		m.mu.Unlock()
	}
}

// fetchPrice gets current price from Binance public API (no auth needed).
func (m *MarketMonitor) fetchPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/price?symbol=%s", symbol)
	resp, err := m.httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	var price float64
	fmt.Sscanf(result.Price, "%f", &price)
	if price == 0 {
		return 0, fmt.Errorf("invalid price for %s", symbol)
	}
	return price, nil
}
