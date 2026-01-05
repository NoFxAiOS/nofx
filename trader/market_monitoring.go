package trader

import (
	"nofx/logger"
	"nofx/market"
	"time"
)

// checkOrderBookTriggers checks for order book anomalies and triggers trading analysis if detected
func (at *AutoTrader) checkOrderBookTriggers() {
	// Only check every 30 seconds to avoid excessive checks
	now := time.Now()
	if now.Sub(at.lastMarketCheckTime) < 30*time.Second {
		return
	}
	at.lastMarketCheckTime = now

	// Get candidate coins from strategy engine
	if at.strategyEngine == nil {
		return
	}

	candidateCoins, err := at.strategyEngine.GetCandidateCoins()
	if err != nil {
		logger.Warnf("⚠️ Failed to get candidate coins for trigger checking: %v", err)
		return
	}

	at.orderBookMonitorsMu.Lock()
	defer at.orderBookMonitorsMu.Unlock()

	hasTriggered := false

	// Check triggers for each candidate coin
	for _, coin := range candidateCoins {
		// Get or create monitor for this symbol
		monitor, exists := at.orderBookMonitors[coin.Symbol]
		if !exists {
			monitor = market.NewOrderBookMonitor(coin.Symbol)
			at.orderBookMonitors[coin.Symbol] = monitor
		}

		// Check for triggers
		triggers := monitor.CheckTriggers()
		if len(triggers) > 0 {
			hasTriggered = true

			for _, trigger := range triggers {
				severity := "LOW"
				if trigger.Severity > 0.7 {
					severity = "HIGH"
				} else if trigger.Severity > 0.4 {
					severity = "MEDIUM"
				}

				logger.Infof("🔔 [%s] Order Book Trigger Detected! Type: %s, Severity: %s (%.0f%%)",
					coin.Symbol, trigger.TriggerType, severity, trigger.Severity*100)

				// Publish event to event bus
				event := TradingEvent{
					Type:      EventTypePriceSpike, // Map order book trigger to event type
					Symbol:    coin.Symbol,
					Timestamp: trigger.Timestamp,
					Source:    "order_book_monitor",
					Severity:  trigger.Severity,
					Data: map[string]interface{}{
						"trigger_type":    trigger.TriggerType,
						"current_price":   trigger.CurrentPrice,
						"price_change_2m": trigger.PriceChange2Min,
						"order_imbalance": trigger.OrderImbalance,
						"volume_ratio":    trigger.VolumeRatio,
						"details":         trigger.Details,
					},
				}
				at.eventBus.PublishAsync(event)
			}
		}
	}

	// If any triggers detected, consider running an extra cycle
	if hasTriggered {
		logger.Infof("📈 Market triggers detected, may expedite next AI cycle")
	}
}

// updateMarketData updates price and volume data for order book monitors
func (at *AutoTrader) updateMarketData(data *market.Data) {
	if data == nil {
		return
	}

	at.orderBookMonitorsMu.Lock()
	defer at.orderBookMonitorsMu.Unlock()

	monitor, exists := at.orderBookMonitors[data.Symbol]
	if !exists {
		monitor = market.NewOrderBookMonitor(data.Symbol)
		at.orderBookMonitors[data.Symbol] = monitor
	}

	// Update price
	if data.CurrentPrice > 0 {
		monitor.UpdatePrice(data.CurrentPrice)
	}

	// Update volume from timeframe data if available
	if data.TimeframeData != nil {
		if td, ok := data.TimeframeData["1m"]; ok && len(td.Klines) > 0 {
			lastBar := td.Klines[len(td.Klines)-1]
			if lastBar.Volume > 0 {
				monitor.UpdateVolume(lastBar.Volume)
			}
		}
	}
}

// publishMarketEvent publishes a market event to the event bus
func (at *AutoTrader) publishMarketEvent(eventType EventType, symbol string, severity float64, data map[string]interface{}) {
	if at.eventBus == nil {
		return
	}

	event := TradingEvent{
		Type:      eventType,
		Symbol:    symbol,
		Timestamp: time.Now(),
		Source:    "market_monitor",
		Severity:  severity,
		Data:      data,
	}

	at.eventBus.PublishAsync(event)
}

// GetEventBus returns the event bus for external subscribers
func (at *AutoTrader) GetEventBus() *EventBus {
	return at.eventBus
}

// GetOrderBookMonitor returns the order book monitor for a specific symbol
func (at *AutoTrader) GetOrderBookMonitor(symbol string) *market.OrderBookMonitor {
	at.orderBookMonitorsMu.RLock()
	defer at.orderBookMonitorsMu.RUnlock()

	return at.orderBookMonitors[symbol]
}

// GetAllOrderBookMetrics returns metrics for all monitored symbols
func (at *AutoTrader) GetAllOrderBookMetrics() map[string]map[string]interface{} {
	at.orderBookMonitorsMu.RLock()
	defer at.orderBookMonitorsMu.RUnlock()

	metrics := make(map[string]map[string]interface{})
	for symbol, monitor := range at.orderBookMonitors {
		metrics[symbol] = monitor.GetCurrentMetrics()
	}

	return metrics
}
