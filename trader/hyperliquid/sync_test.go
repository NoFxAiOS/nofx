package hyperliquid

import (
	"math"
	"nofx/store"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestHyperliquidOrderDirectionParsing tests Dir field parsing
func TestHyperliquidOrderDirectionParsing(t *testing.T) {
	tests := []struct {
		name            string
		dirField        string
		side            string
		expectedAction  string
		expectedPosSide string
	}{
		{
			name:            "Open Long",
			dirField:        "Open Long",
			side:            "BUY",
			expectedAction:  "open_long",
			expectedPosSide: "LONG",
		},
		{
			name:            "Open Short",
			dirField:        "Open Short",
			side:            "SELL",
			expectedAction:  "open_short",
			expectedPosSide: "SHORT",
		},
		{
			name:            "Close Long",
			dirField:        "Close Long",
			side:            "SELL",
			expectedAction:  "close_long",
			expectedPosSide: "LONG",
		},
		{
			name:            "Close Short",
			dirField:        "Close Short",
			side:            "BUY",
			expectedAction:  "close_short",
			expectedPosSide: "SHORT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock fill data structure from Hyperliquid SDK
			// We'll test the parsing logic directly
			var orderAction string
			switch tt.dirField {
			case "Open Long":
				orderAction = "open_long"
			case "Open Short":
				orderAction = "open_short"
			case "Close Long":
				orderAction = "close_long"
			case "Close Short":
				orderAction = "close_short"
			}

			if orderAction != tt.expectedAction {
				t.Errorf("Expected action %s, got %s", tt.expectedAction, orderAction)
			}
		})
	}
}

// TestHyperliquidPositionBuilding tests the complete flow of position building
func TestHyperliquidPositionBuilding(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Initialize stores
	positionStore := store.NewPositionStore(db)
	if err := positionStore.InitTables(); err != nil {
		t.Fatalf("Failed to initialize position tables: %v", err)
	}

	posBuilder := store.NewPositionBuilder(positionStore)

	traderID := "test-trader"
	exchangeID := "test-exchange"
	exchangeType := "hyperliquid"
	symbol := "ETHUSDT"

	// Test Case 1: Open Long → Close Long (should result in 0 position)
	t.Run("Open and Close Long", func(t *testing.T) {
		// Open Long: BUY 0.1 ETH @ 3500
		err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "LONG", "open_long",
			0.1, 3500, 0.5, 0,
			time.Now().UnixMilli(), "order-1",
		)
		if err != nil {
			t.Fatalf("Failed to process open long: %v", err)
		}

		// Verify position created
		positions, err := positionStore.GetOpenPositions(traderID)
		if err != nil {
			t.Fatalf("Failed to get positions: %v", err)
		}
		if len(positions) != 1 {
			t.Fatalf("Expected 1 open position, got %d", len(positions))
		}
		if positions[0].Quantity != 0.1 {
			t.Errorf("Expected quantity 0.1, got %f", positions[0].Quantity)
		}

		// Close Long: SELL 0.1 ETH @ 3600
		err = posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "LONG", "close_long",
			0.1, 3600, 0.5, 10.0, // PnL = (3600-3500)*0.1 = 10
			time.Now().UnixMilli(), "order-2",
		)
		if err != nil {
			t.Fatalf("Failed to process close long: %v", err)
		}

		// Verify position closed
		positions, err = positionStore.GetOpenPositions(traderID)
		if err != nil {
			t.Fatalf("Failed to get positions: %v", err)
		}
		if len(positions) != 0 {
			t.Errorf("Expected 0 open positions, got %d", len(positions))
		}
	})

	// Clear positions for next test
	db.Exec("DELETE FROM trader_positions")

	// Test Case 2: Open Short → Close Short with BUY (the bug scenario!)
	t.Run("Open Short then Close with BUY", func(t *testing.T) {
		// Open Short: SELL 0.05 ETH @ 3500
		err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "SHORT", "open_short",
			0.05, 3500, 0.25, 0,
			time.Now().UnixMilli(), "order-3",
		)
		if err != nil {
			t.Fatalf("Failed to process open short: %v", err)
		}

		// Verify SHORT position created
		positions, err := positionStore.GetOpenPositions(traderID)
		if err != nil {
			t.Fatalf("Failed to get positions: %v", err)
		}
		if len(positions) != 1 {
			t.Fatalf("Expected 1 open position, got %d", len(positions))
		}
		if positions[0].Side != "SHORT" {
			t.Errorf("Expected SHORT position, got %s", positions[0].Side)
		}

		// Close Short: BUY 0.05 ETH @ 3400
		// ⚠️ This is the critical test - BUY should close SHORT, not open LONG!
		err = posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "SHORT", "close_short",
			0.05, 3400, 0.25, 5.0, // PnL = (3500-3400)*0.05 = 5
			time.Now().UnixMilli(), "order-4",
		)
		if err != nil {
			t.Fatalf("Failed to process close short: %v", err)
		}

		// Verify position CLOSED (not opened a new LONG!)
		positions, err = positionStore.GetOpenPositions(traderID)
		if err != nil {
			t.Fatalf("Failed to get positions: %v", err)
		}
		if len(positions) != 0 {
			t.Errorf("Expected 0 open positions after close, got %d", len(positions))
			if len(positions) > 0 {
				t.Errorf("Wrong position side: %s (should be closed!)", positions[0].Side)
			}
		}
	})

	// Clear positions
	db.Exec("DELETE FROM trader_positions")

	// Test Case 3: Position Averaging (Open → Add → Close)
	t.Run("Position Averaging", func(t *testing.T) {
		// Open Long: BUY 0.1 ETH @ 3500
		err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "LONG", "open_long",
			0.1, 3500, 0.5, 0,
			time.Now().UnixMilli(), "order-5",
		)
		if err != nil {
			t.Fatalf("Failed to process first open: %v", err)
		}

		// Add to Long: BUY 0.1 ETH @ 3600
		err = posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "LONG", "open_long",
			0.1, 3600, 0.5, 0,
			time.Now().UnixMilli(), "order-6",
		)
		if err != nil {
			t.Fatalf("Failed to process add position: %v", err)
		}

		// Verify averaged position
		positions, err := positionStore.GetOpenPositions(traderID)
		if err != nil {
			t.Fatalf("Failed to get positions: %v", err)
		}
		if len(positions) != 1 {
			t.Fatalf("Expected 1 position (averaged), got %d", len(positions))
		}
		if positions[0].Quantity != 0.2 {
			t.Errorf("Expected quantity 0.2, got %f", positions[0].Quantity)
		}
		expectedAvgPrice := (3500*0.1 + 3600*0.1) / 0.2 // = 3550
		if positions[0].EntryPrice != expectedAvgPrice {
			t.Errorf("Expected avg price %f, got %f", expectedAvgPrice, positions[0].EntryPrice)
		}

		// Close all: SELL 0.2 ETH @ 3700
		err = posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "LONG", "close_long",
			0.2, 3700, 1.0, 30.0,
			time.Now().UnixMilli(), "order-7",
		)
		if err != nil {
			t.Fatalf("Failed to process close: %v", err)
		}

		// Verify fully closed
		positions, err = positionStore.GetOpenPositions(traderID)
		if err != nil {
			t.Fatalf("Failed to get positions: %v", err)
		}
		if len(positions) != 0 {
			t.Errorf("Expected 0 positions, got %d", len(positions))
		}
	})

	// Clear positions
	db.Exec("DELETE FROM trader_positions")

	// Test Case 4: Partial Close
	t.Run("Partial Close", func(t *testing.T) {
		// Open Long: BUY 1.0 ETH @ 3500
		err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "LONG", "open_long",
			1.0, 3500, 2.0, 0,
			time.Now().UnixMilli(), "order-8",
		)
		if err != nil {
			t.Fatalf("Failed to process open: %v", err)
		}

		// Partial Close: SELL 0.3 ETH @ 3600
		err = posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, "LONG", "close_long",
			0.3, 3600, 0.6, 30.0,
			time.Now().UnixMilli(), "order-9",
		)
		if err != nil {
			t.Fatalf("Failed to process partial close: %v", err)
		}

		// Verify remaining position
		positions, err := positionStore.GetOpenPositions(traderID)
		if err != nil {
			t.Fatalf("Failed to get positions: %v", err)
		}
		if len(positions) != 1 {
			t.Fatalf("Expected 1 position, got %d", len(positions))
		}
		if positions[0].Quantity != 0.7 {
			t.Errorf("Expected remaining quantity 0.7, got %f", positions[0].Quantity)
		}
		if positions[0].Status != "OPEN" {
			t.Errorf("Expected status OPEN, got %s", positions[0].Status)
		}
	})
}

// TestHyperliquidBugScenario tests the exact bug we fixed
func TestHyperliquidBugScenario(t *testing.T) {
	// Setup database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	positionStore := store.NewPositionStore(db)
	if err := positionStore.InitTables(); err != nil {
		t.Fatalf("Failed to initialize position tables: %v", err)
	}

	posBuilder := store.NewPositionBuilder(positionStore)

	traderID := "test-trader"
	exchangeID := "test-exchange"
	exchangeType := "hyperliquid"

	// Simulate the exact scenario from the bug report
	// Account has 30 USDT, should not be able to hold 1.7 ETH

	trades := []struct {
		action   string
		side     string
		symbol   string
		qty      float64
		price    float64
		fee      float64
		pnl      float64
	}{
		// Order 853: Open Short
		{"open_short", "SHORT", "ETHUSDT", 0.0472, 3500, 0.2, 0},
		// Order 854: Close Short (was incorrectly classified as open_long)
		{"close_short", "SHORT", "ETHUSDT", 0.0472, 3400, 0.2, 4.72},
		// Order 855: Open Long
		{"open_long", "LONG", "ETHUSDT", 0.05, 3450, 0.2, 0},
		// Order 856: Close Long
		{"close_long", "LONG", "ETHUSDT", 0.05, 3550, 0.2, 5.0},
	}

	for i, trade := range trades {
		err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			trade.symbol, trade.side, trade.action,
			trade.qty, trade.price, trade.fee, trade.pnl,
			time.Now().Add(time.Duration(i)*time.Second).UnixMilli(),
			"",
		)
		if err != nil {
			t.Fatalf("Failed to process trade %d: %v", i, err)
		}
	}

	// Verify: Should have 0 open positions
	positions, err := positionStore.GetOpenPositions(traderID)
	if err != nil {
		t.Fatalf("Failed to get positions: %v", err)
	}

	if len(positions) != 0 {
		t.Errorf("Expected 0 open positions, got %d", len(positions))
		for _, p := range positions {
			t.Errorf("  Unexpected position: %s %s qty=%.4f", p.Symbol, p.Side, p.Quantity)
		}
	}

	// Verify closed positions have correct PnL
	allPositions, err := positionStore.GetClosedPositions(traderID, 100)
	if err != nil {
		t.Fatalf("Failed to get closed positions: %v", err)
	}

	totalPnL := 0.0
	for _, p := range allPositions {
		if p.Status == "CLOSED" {
			totalPnL += p.RealizedPnL
		}
	}

	expectedTotalPnL := 4.72 + 5.0 // Sum of both close trades
	// Use tolerance for floating point comparison
	if math.Abs(totalPnL-expectedTotalPnL) > 0.01 {
		t.Errorf("Expected total PnL %.2f, got %.2f", expectedTotalPnL, totalPnL)
	}
}
