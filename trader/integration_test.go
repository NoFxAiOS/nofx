package trader

import (
	"nofx/market"
	"testing"
	"time"
)

// TestEventBusBasic tests basic event bus functionality
func TestEventBusBasic(t *testing.T) {
	eventBus := NewEventBus()
	received := make(chan TradingEvent, 1)

	// Subscribe to price spike events
	eventBus.Subscribe(EventTypePriceSpike, func(event TradingEvent) {
		received <- event
	})

	// Publish an event
	event := TradingEvent{
		Type:      EventTypePriceSpike,
		Symbol:    "BTCUSDT",
		Timestamp: time.Now(),
		Source:    "test",
		Severity:  0.85,
		Data: map[string]interface{}{
			"price": 45000.0,
		},
	}

	eventBus.PublishAsync(event)

	// Wait for event to be processed
	select {
	case receivedEvent := <-received:
		if receivedEvent.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", receivedEvent.Symbol)
		}
		if receivedEvent.Severity != 0.85 {
			t.Errorf("Expected severity 0.85, got %.2f", receivedEvent.Severity)
		}
	case <-time.After(time.Second):
		t.Error("Event not received within timeout")
	}
}

// TestEventBusHistory tests event history tracking
func TestEventBusHistory(t *testing.T) {
	eventBus := NewEventBus()

	// Publish multiple events
	for i := 0; i < 5; i++ {
		event := TradingEvent{
			Type:      EventTypePriceSpike,
			Symbol:    "BTCUSDT",
			Timestamp: time.Now(),
			Source:    "test",
			Severity:  float64(i) * 0.1,
		}
		eventBus.Publish(event)
	}

	// Get history
	history := eventBus.GetHistory(EventTypePriceSpike, 10)
	if len(history) != 5 {
		t.Errorf("Expected 5 events in history, got %d", len(history))
	}
}

// TestOrderBookMonitorPriceMovement tests price movement trigger detection
func TestOrderBookMonitorPriceMovement(t *testing.T) {
	monitor := market.NewOrderBookMonitor("BTCUSDT")

	// Simulate price history
	monitor.UpdatePrice(100.0)
	time.Sleep(10 * time.Millisecond)
	monitor.UpdatePrice(100.2)
	time.Sleep(10 * time.Millisecond)
	monitor.UpdatePrice(100.4)
	time.Sleep(10 * time.Millisecond)
	monitor.UpdatePrice(100.6) // 0.6% movement - should trigger

	triggers := monitor.CheckTriggers()
	if len(triggers) == 0 {
		t.Error("Expected price movement trigger")
	}

	found := false
	for _, trigger := range triggers {
		if trigger.TriggerType == "price_movement" {
			found = true
			if trigger.Severity <= 0.5 {
				t.Errorf("Expected high severity, got %.2f", trigger.Severity)
			}
		}
	}

	if !found {
		t.Error("Price movement trigger not found in triggers")
	}
}

// TestOrderBookMonitorVolumSpike tests volume spike detection
func TestOrderBookMonitorVolumSpike(t *testing.T) {
	monitor := market.NewOrderBookMonitor("BTCUSDT")
	monitor.SetCooldown(100 * time.Millisecond)

	// Establish baseline
	for i := 0; i < 30; i++ {
		monitor.UpdateVolume(1000.0)
	}

	time.Sleep(200 * time.Millisecond)

	// Create volume spike
	monitor.UpdateVolume(2500.0) // 2.5x baseline

	triggers := monitor.CheckTriggers()
	found := false
	for _, trigger := range triggers {
		if trigger.TriggerType == "volume_spike" {
			found = true
			if trigger.VolumeRatio < 2.0 {
				t.Errorf("Expected volume ratio > 2.0, got %.2f", trigger.VolumeRatio)
			}
		}
	}

	if !found {
		t.Error("Volume spike trigger not found")
	}
}

// TestOrderBookMonitorImbalance tests order book imbalance detection
func TestOrderBookMonitorImbalance(t *testing.T) {
	monitor := market.NewOrderBookMonitor("BTCUSDT")
	monitor.SetCooldown(100 * time.Millisecond)

	// Severe buy imbalance: 80/20
	monitor.UpdateOrderBook(800, 200)

	triggers := monitor.CheckTriggers()
	found := false
	for _, trigger := range triggers {
		if trigger.TriggerType == "order_imbalance" {
			found = true
			if trigger.Severity < 0.5 {
				t.Errorf("Expected high severity for 80/20 imbalance, got %.2f", trigger.Severity)
			}
		}
	}

	if !found {
		t.Error("Order imbalance trigger not found")
	}
}

// TestOrderBookMonitorCooldown tests cooldown between triggers
func TestOrderBookMonitorCooldown(t *testing.T) {
	monitor := market.NewOrderBookMonitor("BTCUSDT")
	monitor.SetCooldown(500 * time.Millisecond)

	// First trigger
	monitor.UpdatePrice(100.0)
	monitor.UpdatePrice(100.6) // Trigger price movement
	triggers1 := monitor.CheckTriggers()
	if len(triggers1) == 0 {
		t.Error("Expected first trigger")
	}

	// Second trigger immediately (should be suppressed)
	triggers2 := monitor.CheckTriggers()
	if len(triggers2) > 0 {
		t.Error("Expected cooldown to suppress second trigger")
	}

	// Wait for cooldown
	time.Sleep(600 * time.Millisecond)

	// Third trigger (should fire)
	monitor.UpdatePrice(101.2) // Another movement
	triggers3 := monitor.CheckTriggers()
	if len(triggers3) == 0 {
		t.Error("Expected trigger after cooldown period")
	}
}

// TestWebSocketClientInterface tests WebSocket interface compliance
func TestWebSocketClientInterface(t *testing.T) {
	client := market.NewBinanceWebSocketClient(false)

	// Test that client implements the interface
	var _ market.WebSocketClient = client

	// Test basic methods exist
	if client.IsConnected() {
		t.Error("Should not be connected initially")
	}

	// Test channels exist
	klineChannel := client.GetKlineChannel()
	if klineChannel == nil {
		t.Error("Kline channel should not be nil")
	}

	orderChannel := client.GetOrderUpdateChannel()
	if orderChannel == nil {
		t.Error("Order channel should not be nil")
	}
}

// TestWebSocketManagerBasic tests WebSocket manager functionality
func TestWebSocketManagerBasic(t *testing.T) {
	manager := market.NewWebSocketManager()

	client := market.NewBinanceWebSocketClient(false)
	err := manager.RegisterClient("binance", client)
	if err != nil {
		t.Errorf("Failed to register client: %v", err)
	}

	retrieved := manager.GetClient("binance")
	if retrieved == nil {
		t.Error("Failed to retrieve registered client")
	}

	notFound := manager.GetClient("unknown")
	if notFound != nil {
		t.Error("Should return nil for unregistered exchange")
	}
}
