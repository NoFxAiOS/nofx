package market

import (
	"sync"
	"time"
)

// OrderBookTrigger represents a detected order book anomaly that should trigger trading analysis
type OrderBookTrigger struct {
	Symbol          string
	TriggerType     string  // "order_imbalance", "volume_spike", "price_movement"
	Severity        float64 // 0.0 to 1.0, higher = more severe
	Timestamp       time.Time
	CurrentPrice    float64
	PriceChange2Min float64 // % change in last 2 minutes
	VolumeRatio     float64 // Current volume / baseline
	OrderImbalance  float64 // Buy volume / (Buy volume + Sell volume) - 0.5 indicates perfect balance
	Details         map[string]interface{}
}

// OrderBookMonitor tracks market conditions and detects triggers for trading analysis
type OrderBookMonitor struct {
	symbol              string
	mu                  sync.RWMutex
	lastPrice           float64
	priceHistory        []float64            // Last 20 price points (10 seconds each)
	priceTimestamps     []time.Time          // Timestamps for price history
	volumeHistory       []float64            // Last 60 volume points (1 minute each)
	volumeTimestamps    []time.Time          // Timestamps for volume history
	orderBookImbalance  float64              // Buy/sell ratio, 0.5 = balanced
	lastTriggerTime     map[string]time.Time // Last time each trigger fired (to avoid spam)
	volumeBaseline      float64              // Moving average of normal volume
	imbalanceThreshold  float64              // 0.3 = 70/30 split (significant imbalance)
	volumeSpikeMultiple float64              // How many times baseline = spike (e.g., 2.0 = 2x)
	priceMoveThreshold  float64              // % price movement threshold (0.005 = 0.5%)
	cooldownDuration    time.Duration        // Minimum time between same trigger type
}

// NewOrderBookMonitor creates a new order book monitor for a symbol
func NewOrderBookMonitor(symbol string) *OrderBookMonitor {
	return &OrderBookMonitor{
		symbol:              symbol,
		lastPrice:           0,
		priceHistory:        make([]float64, 0, 20),
		priceTimestamps:     make([]time.Time, 0, 20),
		volumeHistory:       make([]float64, 0, 60),
		volumeTimestamps:    make([]time.Time, 0, 60),
		orderBookImbalance:  0.5,
		lastTriggerTime:     make(map[string]time.Time),
		volumeBaseline:      0,
		imbalanceThreshold:  0.35,             // 65/35 or worse
		volumeSpikeMultiple: 2.0,              // 2x or more
		priceMoveThreshold:  0.005,            // 0.5%
		cooldownDuration:    30 * time.Second, // Don't fire same trigger more than once per 30s
	}
}

// UpdatePrice updates the current price and checks for price movement triggers
func (m *OrderBookMonitor) UpdatePrice(price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Add to history (keep last 20 points = ~3-4 minutes of data)
	m.priceHistory = append(m.priceHistory, price)
	m.priceTimestamps = append(m.priceTimestamps, now)

	if len(m.priceHistory) > 20 {
		m.priceHistory = m.priceHistory[1:]
		m.priceTimestamps = m.priceTimestamps[1:]
	}

	m.lastPrice = price
}

// UpdateVolume updates the volume and checks for volume spike triggers
func (m *OrderBookMonitor) UpdateVolume(volume float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Add to history (keep last 60 points = ~1 hour of data)
	m.volumeHistory = append(m.volumeHistory, volume)
	m.volumeTimestamps = append(m.volumeTimestamps, now)

	if len(m.volumeHistory) > 60 {
		m.volumeHistory = m.volumeHistory[1:]
		m.volumeTimestamps = m.volumeTimestamps[1:]
	}

	// Update baseline (average volume)
	if len(m.volumeHistory) > 20 {
		total := 0.0
		for i := 0; i < len(m.volumeHistory)-10; i++ {
			total += m.volumeHistory[i]
		}
		m.volumeBaseline = total / float64(len(m.volumeHistory)-10)
	} else if len(m.volumeHistory) > 0 {
		total := 0.0
		for _, v := range m.volumeHistory {
			total += v
		}
		m.volumeBaseline = total / float64(len(m.volumeHistory))
	}
}

// UpdateOrderBook updates order book imbalance (buy/sell ratio)
// buyVolume: total buy side volume at top levels
// sellVolume: total sell side volume at top levels
func (m *OrderBookMonitor) UpdateOrderBook(buyVolume, sellVolume float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := buyVolume + sellVolume
	if total > 0 {
		m.orderBookImbalance = buyVolume / total
	}
}

// CheckTriggers checks for all order book triggers and returns any detected
func (m *OrderBookMonitor) CheckTriggers() []OrderBookTrigger {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	triggers := []OrderBookTrigger{}

	// Check 1: Order Book Imbalance
	imbalanceTrigger := m.checkOrderImbalanceTrigger(now)
	if imbalanceTrigger != nil {
		triggers = append(triggers, *imbalanceTrigger)
	}

	// Check 2: Volume Spike
	volumeTrigger := m.checkVolumeSpikerigger(now)
	if volumeTrigger != nil {
		triggers = append(triggers, *volumeTrigger)
	}

	// Check 3: Price Movement
	priceTrigger := m.checkPriceMovementTrigger(now)
	if priceTrigger != nil {
		triggers = append(triggers, *priceTrigger)
	}

	return triggers
}

// checkOrderImbalanceTrigger detects significant buy/sell imbalance
func (m *OrderBookMonitor) checkOrderImbalanceTrigger(now time.Time) *OrderBookTrigger {
	if m.orderBookImbalance < m.imbalanceThreshold || m.orderBookImbalance > (1-m.imbalanceThreshold) {
		lastFireTime := m.lastTriggerTime["order_imbalance"]
		if now.Sub(lastFireTime) < m.cooldownDuration {
			return nil
		}

		m.lastTriggerTime["order_imbalance"] = now

		// Determine if buy or sell dominated
		buyDominated := m.orderBookImbalance > 0.5

		severity := 0.0
		if buyDominated {
			severity = m.orderBookImbalance - 0.5
		} else {
			severity = 0.5 - m.orderBookImbalance
		}
		severity = severity * 2 // Normalize to 0-1

		return &OrderBookTrigger{
			Symbol:         m.symbol,
			TriggerType:    "order_imbalance",
			Severity:       severity,
			Timestamp:      now,
			CurrentPrice:   m.lastPrice,
			OrderImbalance: m.orderBookImbalance,
			Details: map[string]interface{}{
				"buy_dominated": buyDominated,
				"threshold":     m.imbalanceThreshold,
			},
		}
	}
	return nil
}

// checkVolumeSpikerigger detects abnormal volume spikes
func (m *OrderBookMonitor) checkVolumeSpikerigger(now time.Time) *OrderBookTrigger {
	if len(m.volumeHistory) == 0 || m.volumeBaseline == 0 {
		return nil
	}

	currentVolume := m.volumeHistory[len(m.volumeHistory)-1]
	volumeRatio := currentVolume / m.volumeBaseline

	if volumeRatio >= m.volumeSpikeMultiple {
		lastFireTime := m.lastTriggerTime["volume_spike"]
		if now.Sub(lastFireTime) < m.cooldownDuration {
			return nil
		}

		m.lastTriggerTime["volume_spike"] = now

		severity := (volumeRatio - 1) / (m.volumeSpikeMultiple - 1)
		if severity > 1.0 {
			severity = 1.0
		}

		return &OrderBookTrigger{
			Symbol:       m.symbol,
			TriggerType:  "volume_spike",
			Severity:     severity,
			Timestamp:    now,
			CurrentPrice: m.lastPrice,
			VolumeRatio:  volumeRatio,
			Details: map[string]interface{}{
				"current_volume":  currentVolume,
				"baseline_volume": m.volumeBaseline,
				"multiple":        m.volumeSpikeMultiple,
			},
		}
	}
	return nil
}

// checkPriceMovementTrigger detects rapid price movements
func (m *OrderBookMonitor) checkPriceMovementTrigger(now time.Time) *OrderBookTrigger {
	if len(m.priceHistory) < 2 {
		return nil
	}

	// Check 2-minute price change (up to 12 points in history)
	checkPoints := 12
	if checkPoints > len(m.priceHistory) {
		checkPoints = len(m.priceHistory)
	}

	oldPrice := m.priceHistory[len(m.priceHistory)-checkPoints]
	currentPrice := m.priceHistory[len(m.priceHistory)-1]
	priceChange := (currentPrice - oldPrice) / oldPrice

	absChange := priceChange
	if absChange < 0 {
		absChange = -absChange
	}

	if absChange >= m.priceMoveThreshold {
		lastFireTime := m.lastTriggerTime["price_movement"]
		if now.Sub(lastFireTime) < m.cooldownDuration {
			return nil
		}

		m.lastTriggerTime["price_movement"] = now

		severity := absChange / m.priceMoveThreshold
		if severity > 1.0 {
			severity = 1.0
		}

		direction := "down"
		if priceChange > 0 {
			direction = "up"
		}

		return &OrderBookTrigger{
			Symbol:          m.symbol,
			TriggerType:     "price_movement",
			Severity:        severity,
			Timestamp:       now,
			CurrentPrice:    currentPrice,
			PriceChange2Min: priceChange * 100, // Convert to percentage
			Details: map[string]interface{}{
				"direction":     direction,
				"change_pct":    priceChange * 100,
				"threshold_pct": m.priceMoveThreshold * 100,
			},
		}
	}
	return nil
}

// GetCurrentMetrics returns the current market metrics
func (m *OrderBookMonitor) GetCurrentMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	volumeRatio := 0.0
	if m.volumeBaseline > 0 && len(m.volumeHistory) > 0 {
		volumeRatio = m.volumeHistory[len(m.volumeHistory)-1] / m.volumeBaseline
	}

	return map[string]interface{}{
		"symbol":               m.symbol,
		"last_price":           m.lastPrice,
		"order_book_imbalance": m.orderBookImbalance,
		"volume_ratio":         volumeRatio,
		"volume_baseline":      m.volumeBaseline,
		"price_history_len":    len(m.priceHistory),
		"volume_history_len":   len(m.volumeHistory),
	}
}

// SetThresholds allows customization of trigger thresholds
func (m *OrderBookMonitor) SetThresholds(imbalanceThreshold, volumeSpikeMultiple, priceMoveThreshold float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.imbalanceThreshold = imbalanceThreshold
	m.volumeSpikeMultiple = volumeSpikeMultiple
	m.priceMoveThreshold = priceMoveThreshold
}

// SetCooldown sets the minimum time between trigger fires
func (m *OrderBookMonitor) SetCooldown(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cooldownDuration = duration
}
