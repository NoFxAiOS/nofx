package trader

import (
	"nofx/decision"
	"testing"
)

// TestDetectClosedPositions_StopLossTriggered tests detection of positions closed by stop-loss
func TestDetectClosedPositions_StopLossTriggered(t *testing.T) {
	at := &AutoTrader{
		lastPositions: make(map[string]decision.PositionInfo),
	}

	// Setup: Previous cycle had a long position
	at.lastPositions["BTCUSDT_long"] = decision.PositionInfo{
		Symbol:     "BTCUSDT",
		Side:       "long",
		EntryPrice: 50000.0,
		MarkPrice:  49500.0,
		Quantity:   0.1,
		Leverage:   10,
	}

	// Current cycle: Position disappeared (stop-loss triggered)
	currentPositions := []decision.PositionInfo{} // Empty - position closed

	// Detect closed positions
	closedPositions := at.detectClosedPositions(currentPositions)

	// Verify
	if len(closedPositions) != 1 {
		t.Fatalf("Expected 1 closed position, got %d", len(closedPositions))
	}

	closed := closedPositions[0]
	if closed.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", closed.Symbol)
	}
	if closed.Side != "long" {
		t.Errorf("Expected side long, got %s", closed.Side)
	}
	if closed.EntryPrice != 50000.0 {
		t.Errorf("Expected entry price 50000, got %f", closed.EntryPrice)
	}
	if closed.Quantity != 0.1 {
		t.Errorf("Expected quantity 0.1, got %f", closed.Quantity)
	}
}

// TestDetectClosedPositions_TakeProfitTriggered tests detection of positions closed by take-profit
func TestDetectClosedPositions_TakeProfitTriggered(t *testing.T) {
	at := &AutoTrader{
		lastPositions: make(map[string]decision.PositionInfo),
	}

	// Setup: Previous cycle had a short position
	at.lastPositions["ETHUSDT_short"] = decision.PositionInfo{
		Symbol:     "ETHUSDT",
		Side:       "short",
		EntryPrice: 3000.0,
		MarkPrice:  2900.0,
		Quantity:   1.0,
		Leverage:   5,
	}

	// Current cycle: Position disappeared (take-profit triggered)
	currentPositions := []decision.PositionInfo{}

	// Detect
	closedPositions := at.detectClosedPositions(currentPositions)

	// Verify
	if len(closedPositions) != 1 {
		t.Fatalf("Expected 1 closed position, got %d", len(closedPositions))
	}

	closed := closedPositions[0]
	if closed.Symbol != "ETHUSDT" {
		t.Errorf("Expected symbol ETHUSDT, got %s", closed.Symbol)
	}
	if closed.Side != "short" {
		t.Errorf("Expected side short, got %s", closed.Side)
	}
}

// TestDetectClosedPositions_MultiplePositionsClosed tests multiple positions closed simultaneously
func TestDetectClosedPositions_MultiplePositionsClosed(t *testing.T) {
	at := &AutoTrader{
		lastPositions: make(map[string]decision.PositionInfo),
	}

	// Setup: Multiple positions
	at.lastPositions["BTCUSDT_long"] = decision.PositionInfo{
		Symbol:   "BTCUSDT",
		Side:     "long",
		Quantity: 0.1,
	}
	at.lastPositions["ETHUSDT_short"] = decision.PositionInfo{
		Symbol:   "ETHUSDT",
		Side:     "short",
		Quantity: 1.0,
	}
	at.lastPositions["SOLUSDT_long"] = decision.PositionInfo{
		Symbol:   "SOLUSDT",
		Side:     "long",
		Quantity: 5.0,
	}

	// Current cycle: Only SOL position remains
	currentPositions := []decision.PositionInfo{
		{
			Symbol: "SOLUSDT",
			Side:   "long",
		},
	}

	// Detect
	closedPositions := at.detectClosedPositions(currentPositions)

	// Verify: BTC and ETH should be detected as closed
	if len(closedPositions) != 2 {
		t.Fatalf("Expected 2 closed positions, got %d", len(closedPositions))
	}

	// Check that both BTC and ETH are in the closed list
	foundBTC := false
	foundETH := false
	for _, closed := range closedPositions {
		if closed.Symbol == "BTCUSDT" && closed.Side == "long" {
			foundBTC = true
		}
		if closed.Symbol == "ETHUSDT" && closed.Side == "short" {
			foundETH = true
		}
	}

	if !foundBTC {
		t.Errorf("BTCUSDT long position not detected as closed")
	}
	if !foundETH {
		t.Errorf("ETHUSDT short position not detected as closed")
	}
}

// TestDetectClosedPositions_NoPositionsClosed tests that existing positions are not flagged
func TestDetectClosedPositions_NoPositionsClosed(t *testing.T) {
	at := &AutoTrader{
		lastPositions: make(map[string]decision.PositionInfo),
	}

	// Setup: One position
	at.lastPositions["BTCUSDT_long"] = decision.PositionInfo{
		Symbol:   "BTCUSDT",
		Side:     "long",
		Quantity: 0.1,
	}

	// Current cycle: Same position still exists
	currentPositions := []decision.PositionInfo{
		{
			Symbol: "BTCUSDT",
			Side:   "long",
		},
	}

	// Detect
	closedPositions := at.detectClosedPositions(currentPositions)

	// Verify: Should be empty
	if len(closedPositions) != 0 {
		t.Errorf("Expected 0 closed positions, got %d", len(closedPositions))
	}
}

// TestDetectClosedPositions_NewPositionOpened tests that new positions don't trigger auto-close
func TestDetectClosedPositions_NewPositionOpened(t *testing.T) {
	at := &AutoTrader{
		lastPositions: make(map[string]decision.PositionInfo),
	}

	// Setup: No previous positions
	// (empty lastPositions)

	// Current cycle: New position opened
	currentPositions := []decision.PositionInfo{
		{
			Symbol: "BTCUSDT",
			Side:   "long",
		},
	}

	// Detect
	closedPositions := at.detectClosedPositions(currentPositions)

	// Verify: Should be empty (new positions shouldn't trigger auto-close)
	if len(closedPositions) != 0 {
		t.Errorf("Expected 0 closed positions, got %d", len(closedPositions))
	}
}

// TestDetectClosedPositions_FirstRun tests that first run with no cache doesn't trigger false positives
func TestDetectClosedPositions_FirstRun(t *testing.T) {
	at := &AutoTrader{
		lastPositions: nil, // First run, no cache
	}

	// Current cycle: Has positions
	currentPositions := []decision.PositionInfo{
		{Symbol: "BTCUSDT", Side: "long"},
		{Symbol: "ETHUSDT", Side: "short"},
	}

	// Detect
	closedPositions := at.detectClosedPositions(currentPositions)

	// Verify: Should be empty (first run, no previous state)
	if len(closedPositions) != 0 {
		t.Errorf("Expected 0 closed positions on first run, got %d", len(closedPositions))
	}
}

// TestGenerateAutoCloseActions tests generation of DecisionActions for closed positions
func TestGenerateAutoCloseActions(t *testing.T) {
	at := &AutoTrader{}

	closedPositions := []decision.PositionInfo{
		{
			Symbol:     "BTCUSDT",
			Side:       "long",
			EntryPrice: 50000.0,
			MarkPrice:  49500.0,
			Quantity:   0.1,
			Leverage:   10,
		},
		{
			Symbol:     "ETHUSDT",
			Side:       "short",
			EntryPrice: 3000.0,
			MarkPrice:  2900.0,
			Quantity:   1.0,
			Leverage:   5,
		},
	}

	// Generate actions
	actions := at.generateAutoCloseActions(closedPositions)

	// Verify
	if len(actions) != 2 {
		t.Fatalf("Expected 2 actions, got %d", len(actions))
	}

	// Check first action (BTCUSDT long close)
	if actions[0].Action != "auto_close_long" {
		t.Errorf("Expected action auto_close_long, got %s", actions[0].Action)
	}
	if actions[0].Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", actions[0].Symbol)
	}
	if actions[0].Price != 49500.0 {
		t.Errorf("Expected price 49500, got %f", actions[0].Price)
	}
	if actions[0].Quantity != 0.1 {
		t.Errorf("Expected quantity 0.1, got %f", actions[0].Quantity)
	}
	if actions[0].Leverage != 10 {
		t.Errorf("Expected leverage 10, got %d", actions[0].Leverage)
	}
	if !actions[0].Success {
		t.Errorf("Expected success=true")
	}

	// Check second action (ETHUSDT short close)
	if actions[1].Action != "auto_close_short" {
		t.Errorf("Expected action auto_close_short, got %s", actions[1].Action)
	}
	if actions[1].Symbol != "ETHUSDT" {
		t.Errorf("Expected symbol ETHUSDT, got %s", actions[1].Symbol)
	}
}

// TestUpdatePositionSnapshot tests that position snapshot is updated correctly
func TestUpdatePositionSnapshot(t *testing.T) {
	at := &AutoTrader{
		lastPositions: make(map[string]decision.PositionInfo),
	}

	// Initial state: no positions
	if len(at.lastPositions) != 0 {
		t.Fatalf("Expected empty lastPositions initially")
	}

	// Update with new positions
	currentPositions := []decision.PositionInfo{
		{
			Symbol:     "BTCUSDT",
			Side:       "long",
			EntryPrice: 50000.0,
			Quantity:   0.1,
		},
		{
			Symbol:     "ETHUSDT",
			Side:       "short",
			EntryPrice: 3000.0,
			Quantity:   1.0,
		},
	}

	at.updatePositionSnapshot(currentPositions)

	// Verify snapshot was updated
	if len(at.lastPositions) != 2 {
		t.Fatalf("Expected 2 positions in snapshot, got %d", len(at.lastPositions))
	}

	// Check BTCUSDT
	btc, exists := at.lastPositions["BTCUSDT_long"]
	if !exists {
		t.Fatalf("BTCUSDT_long not found in snapshot")
	}
	if btc.EntryPrice != 50000.0 {
		t.Errorf("Expected entry price 50000, got %f", btc.EntryPrice)
	}

	// Check ETHUSDT
	eth, exists := at.lastPositions["ETHUSDT_short"]
	if !exists {
		t.Fatalf("ETHUSDT_short not found in snapshot")
	}
	if eth.Quantity != 1.0 {
		t.Errorf("Expected quantity 1.0, got %f", eth.Quantity)
	}

	// Update again with only one position (simulate close)
	currentPositions2 := []decision.PositionInfo{
		{
			Symbol: "BTCUSDT",
			Side:   "long",
		},
	}

	at.updatePositionSnapshot(currentPositions2)

	// Verify snapshot reflects current state
	if len(at.lastPositions) != 1 {
		t.Errorf("Expected 1 position in snapshot after update, got %d", len(at.lastPositions))
	}

	_, exists = at.lastPositions["BTCUSDT_long"]
	if !exists {
		t.Errorf("BTCUSDT_long should still exist")
	}

	_, exists = at.lastPositions["ETHUSDT_short"]
	if exists {
		t.Errorf("ETHUSDT_short should be removed from snapshot")
	}
}

// TestInferCloseDetails_StopLoss tests stop-loss price/reason inference
func TestInferCloseDetails_StopLoss(t *testing.T) {
	at := &AutoTrader{}

	// Test long position stopped out
	pos := decision.PositionInfo{
		Symbol:     "BTCUSDT",
		Side:       "long",
		EntryPrice: 50000.0,
		MarkPrice:  49500.0, // Below stop loss
		StopLoss:   49600.0,
		TakeProfit: 52000.0,
	}

	price, reason := at.inferCloseDetails(pos)

	if reason != "stop_loss" {
		t.Errorf("Expected reason stop_loss, got %s", reason)
	}
	if price != 49600.0 {
		t.Errorf("Expected price 49600, got %.2f", price)
	}

	// Test short position stopped out
	pos2 := decision.PositionInfo{
		Symbol:     "ETHUSDT",
		Side:       "short",
		EntryPrice: 3000.0,
		MarkPrice:  3150.0, // Above stop loss
		StopLoss:   3100.0,
		TakeProfit: 2800.0,
	}

	price2, reason2 := at.inferCloseDetails(pos2)

	if reason2 != "stop_loss" {
		t.Errorf("Expected reason stop_loss, got %s", reason2)
	}
	if price2 != 3100.0 {
		t.Errorf("Expected price 3100, got %.2f", price2)
	}
}

// TestInferCloseDetails_TakeProfit tests take-profit price/reason inference
func TestInferCloseDetails_TakeProfit(t *testing.T) {
	at := &AutoTrader{}

	// Test long position take-profit hit
	pos := decision.PositionInfo{
		Symbol:     "BTCUSDT",
		Side:       "long",
		EntryPrice: 50000.0,
		MarkPrice:  52000.0, // At take profit
		StopLoss:   49000.0,
		TakeProfit: 51900.0,
	}

	price, reason := at.inferCloseDetails(pos)

	if reason != "take_profit" {
		t.Errorf("Expected reason take_profit, got %s", reason)
	}
	if price != 51900.0 {
		t.Errorf("Expected price 51900, got %.2f", price)
	}

	// Test short position take-profit hit
	pos2 := decision.PositionInfo{
		Symbol:     "ETHUSDT",
		Side:       "short",
		EntryPrice: 3000.0,
		MarkPrice:  2800.0, // At take profit
		StopLoss:   3100.0,
		TakeProfit: 2810.0,
	}

	price2, reason2 := at.inferCloseDetails(pos2)

	if reason2 != "take_profit" {
		t.Errorf("Expected reason take_profit, got %s", reason2)
	}
	if price2 != 2810.0 {
		t.Errorf("Expected price 2810, got %.2f", price2)
	}
}

// TestInferCloseDetails_Liquidation tests liquidation detection
func TestInferCloseDetails_Liquidation(t *testing.T) {
	at := &AutoTrader{}

	// Test long position liquidated
	pos := decision.PositionInfo{
		Symbol:           "BTCUSDT",
		Side:             "long",
		EntryPrice:       50000.0,
		MarkPrice:        45500.0, // Near liquidation
		LiquidationPrice: 45000.0,
		StopLoss:         49000.0,
		TakeProfit:       52000.0,
	}

	price, reason := at.inferCloseDetails(pos)

	if reason != "liquidation" {
		t.Errorf("Expected reason liquidation, got %s", reason)
	}
	if price != 45000.0 {
		t.Errorf("Expected price 45000, got %.2f", price)
	}
}

// TestInferCloseDetails_Unknown tests unknown close reason (manual close)
func TestInferCloseDetails_Unknown(t *testing.T) {
	at := &AutoTrader{}

	// Position closed at normal price (not near SL/TP/liquidation)
	pos := decision.PositionInfo{
		Symbol:           "BTCUSDT",
		Side:             "long",
		EntryPrice:       50000.0,
		MarkPrice:        50500.0, // Normal price
		LiquidationPrice: 45000.0,
		StopLoss:         49000.0,
		TakeProfit:       52000.0,
	}

	price, reason := at.inferCloseDetails(pos)

	if reason != "unknown" {
		t.Errorf("Expected reason unknown (manual close), got %s", reason)
	}
	if price != 50500.0 {
		t.Errorf("Expected price 50500 (mark price), got %.2f", price)
	}
}

// TestIntegration_AutoCloseWorkflow tests the complete workflow
func TestIntegration_AutoCloseWorkflow(t *testing.T) {
	at := &AutoTrader{
		lastPositions: make(map[string]decision.PositionInfo),
		config: AutoTraderConfig{
			Exchange: "binance",
		},
	}

	// Cycle 1: Open position
	positions1 := []decision.PositionInfo{
		{
			Symbol:     "BTCUSDT",
			Side:       "long",
			EntryPrice: 50000.0,
			MarkPrice:  50000.0,
			Quantity:   0.1,
			Leverage:   10,
		},
	}
	at.updatePositionSnapshot(positions1)

	// Cycle 2: Position closed by stop-loss
	positions2 := []decision.PositionInfo{} // Empty

	// Detect and generate actions
	closedPositions := at.detectClosedPositions(positions2)
	actions := at.generateAutoCloseActions(closedPositions)

	// Verify auto_close was generated
	if len(actions) != 1 {
		t.Fatalf("Expected 1 auto_close action, got %d", len(actions))
	}

	if actions[0].Action != "auto_close_long" {
		t.Errorf("Expected auto_close_long, got %s", actions[0].Action)
	}

	// Update snapshot for next cycle
	at.updatePositionSnapshot(positions2)

	// Cycle 3: Same empty state should not generate duplicate actions
	closedPositions3 := at.detectClosedPositions(positions2)
	if len(closedPositions3) != 0 {
		t.Errorf("Expected no closed positions in cycle 3, got %d", len(closedPositions3))
	}
}
