package trader

import (
	"sync"
	"time"
)

// EventType defines the type of trading event
type EventType string

const (
	EventTypePriceSpike     EventType = "price_spike"     // Significant price movement
	EventTypeVolumSpike     EventType = "volume_spike"    // Abnormal volume increase
	EventTypeOrderImbalance EventType = "order_imbalance" // Order book imbalance
	EventTypeOrderFilled    EventType = "order_filled"    // Order execution
	EventTypePositionOpened EventType = "position_opened" // New position created
	EventTypePositionClosed EventType = "position_closed" // Position closed
	EventTypeRiskEvent      EventType = "risk_event"      // Risk control triggered
	EventTypeLiquidation    EventType = "liquidation"     // Liquidation warning
)

// TradingEvent represents a market or trading event
type TradingEvent struct {
	Type      EventType              // Type of event
	Symbol    string                 // Trading symbol (e.g., "BTCUSDT")
	Timestamp time.Time              // When the event occurred
	Source    string                 // Source of the event (e.g., "websocket", "order_sync", "risk_monitor")
	Severity  float64                // 0.0 to 1.0, higher = more important
	Data      map[string]interface{} // Event-specific data
}

// EventHandler is a callback function for event subscribers
type EventHandler func(event TradingEvent)

// EventBus provides centralized event publishing and subscription
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]EventHandler
	history     []TradingEvent
	maxHistory  int
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]EventHandler),
		history:     make([]TradingEvent, 0, 100),
		maxHistory:  100,
	}
}

// Subscribe registers a handler for a specific event type
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if handler == nil {
		return
	}

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

// Unsubscribe removes a handler for an event type
// Note: This is a simple implementation that removes all handlers of a type
// For more granular control, consider using handler IDs
func (eb *EventBus) UnsubscribeAll(eventType EventType) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.subscribers, eventType)
}

// Publish sends an event to all subscribers of that event type
func (eb *EventBus) Publish(event TradingEvent) {
	eb.mu.Lock()

	// Store in history
	eb.history = append(eb.history, event)
	if len(eb.history) > eb.maxHistory {
		eb.history = eb.history[len(eb.history)-eb.maxHistory:]
	}

	// Get subscribers
	handlers := eb.subscribers[event.Type]
	eb.mu.Unlock()

	// Call handlers asynchronously to avoid blocking
	for _, handler := range handlers {
		// Use goroutine to prevent handler panic from affecting other handlers
		go func(h EventHandler) {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but continue processing other handlers
				}
			}()
			h(event)
		}(handler)
	}
}

// PublishAsync publishes an event asynchronously in a new goroutine
func (eb *EventBus) PublishAsync(event TradingEvent) {
	go eb.Publish(event)
}

// GetHistory returns recent events matching a type (use empty string for all)
func (eb *EventBus) GetHistory(eventType EventType, limit int) []TradingEvent {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if limit <= 0 || limit > len(eb.history) {
		limit = len(eb.history)
	}

	var result []TradingEvent
	for i := len(eb.history) - 1; i >= 0 && len(result) < limit; i-- {
		if eventType == "" || eb.history[i].Type == eventType {
			result = append(result, eb.history[i])
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// ClearHistory removes all events from history
func (eb *EventBus) ClearHistory() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.history = make([]TradingEvent, 0, 100)
}

// GetSubscriberCount returns the number of subscribers for an event type
func (eb *EventBus) GetSubscriberCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.subscribers[eventType])
}
