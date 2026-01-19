package trader

import (
	"nofx/store"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestScenario represents a trading scenario to test
type TestScenario struct {
	Name        string
	Trades      []TestTrade
	ExpectedPos []ExpectedPosition
}

// TestTrade represents a single trade in a test scenario
type TestTrade struct {
	Action      string  // open_long, close_short, etc.
	Side        string  // LONG or SHORT
	Symbol      string
	Quantity    float64
	Price       float64
	Fee         float64
	RealizedPnL float64
}

// ExpectedPosition represents expected position state
type ExpectedPosition struct {
	Symbol   string
	Side     string
	Quantity float64
	Status   string // OPEN or CLOSED
}

// Standard test scenarios that all exchanges should pass
func getStandardTestScenarios() []TestScenario {
	return []TestScenario{
		{
			Name: "Simple Open and Close Long",
			Trades: []TestTrade{
				{Action: "open_long", Side: "LONG", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3500, Fee: 0.5, RealizedPnL: 0},
				{Action: "close_long", Side: "LONG", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3600, Fee: 0.5, RealizedPnL: 10},
			},
			ExpectedPos: []ExpectedPosition{}, // Should be fully closed
		},
		{
			Name: "Simple Open and Close Short",
			Trades: []TestTrade{
				{Action: "open_short", Side: "SHORT", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3500, Fee: 0.5, RealizedPnL: 0},
				{Action: "close_short", Side: "SHORT", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3400, Fee: 0.5, RealizedPnL: 10},
			},
			ExpectedPos: []ExpectedPosition{},
		},
		{
			Name: "Position Averaging",
			Trades: []TestTrade{
				{Action: "open_long", Side: "LONG", Symbol: "BTCUSDT", Quantity: 0.01, Price: 50000, Fee: 1.0, RealizedPnL: 0},
				{Action: "open_long", Side: "LONG", Symbol: "BTCUSDT", Quantity: 0.01, Price: 51000, Fee: 1.0, RealizedPnL: 0},
				{Action: "close_long", Side: "LONG", Symbol: "BTCUSDT", Quantity: 0.02, Price: 52000, Fee: 2.0, RealizedPnL: 30},
			},
			ExpectedPos: []ExpectedPosition{},
		},
		{
			Name: "Partial Close",
			Trades: []TestTrade{
				{Action: "open_long", Side: "LONG", Symbol: "SOLUSDT", Quantity: 10, Price: 100, Fee: 2.0, RealizedPnL: 0},
				{Action: "close_long", Side: "LONG", Symbol: "SOLUSDT", Quantity: 3, Price: 105, Fee: 0.6, RealizedPnL: 15},
			},
			ExpectedPos: []ExpectedPosition{
				{Symbol: "SOLUSDT", Side: "LONG", Quantity: 7, Status: "OPEN"},
			},
		},
		{
			Name: "Multiple Symbols",
			Trades: []TestTrade{
				{Action: "open_long", Side: "LONG", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3500, Fee: 0.5, RealizedPnL: 0},
				{Action: "open_short", Side: "SHORT", Symbol: "BTCUSDT", Quantity: 0.01, Price: 50000, Fee: 1.0, RealizedPnL: 0},
				{Action: "close_long", Side: "LONG", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3600, Fee: 0.5, RealizedPnL: 10},
			},
			ExpectedPos: []ExpectedPosition{
				{Symbol: "BTCUSDT", Side: "SHORT", Quantity: 0.01, Status: "OPEN"},
			},
		},
		{
			Name: "Bug Scenario - Short then BUY to Close",
			Trades: []TestTrade{
				// This tests the exact bug we fixed
				{Action: "open_short", Side: "SHORT", Symbol: "ETHUSDT", Quantity: 0.0472, Price: 3500, Fee: 0.2, RealizedPnL: 0},
				{Action: "close_short", Side: "SHORT", Symbol: "ETHUSDT", Quantity: 0.0472, Price: 3400, Fee: 0.2, RealizedPnL: 4.72},
			},
			ExpectedPos: []ExpectedPosition{}, // Must be fully closed!
		},
		{
			Name: "Multiple Opens and Closes",
			Trades: []TestTrade{
				{Action: "open_long", Side: "LONG", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3500, Fee: 0.5, RealizedPnL: 0},
				{Action: "close_long", Side: "LONG", Symbol: "ETHUSDT", Quantity: 0.1, Price: 3600, Fee: 0.5, RealizedPnL: 10},
				{Action: "open_short", Side: "SHORT", Symbol: "ETHUSDT", Quantity: 0.05, Price: 3600, Fee: 0.3, RealizedPnL: 0},
				{Action: "close_short", Side: "SHORT", Symbol: "ETHUSDT", Quantity: 0.05, Price: 3500, Fee: 0.3, RealizedPnL: 5},
				{Action: "open_long", Side: "LONG", Symbol: "ETHUSDT", Quantity: 0.2, Price: 3550, Fee: 1.0, RealizedPnL: 0},
			},
			ExpectedPos: []ExpectedPosition{
				{Symbol: "ETHUSDT", Side: "LONG", Quantity: 0.2, Status: "OPEN"},
			},
		},
	}
}

// runStandardTests runs all standard test scenarios
func runStandardTests(t *testing.T, exchangeName string) {
	scenarios := getStandardTestScenarios()

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
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
			exchangeID := "test-exchange-" + exchangeName
			exchangeType := exchangeName

			// Process all trades
			for i, trade := range scenario.Trades {
				err := posBuilder.ProcessTrade(
					traderID, exchangeID, exchangeType,
					trade.Symbol, trade.Side, trade.Action,
					trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
					time.Now().Add(time.Duration(i)*time.Second).UnixMilli(),
					"",
				)
				if err != nil {
					t.Fatalf("Failed to process trade %d (%s): %v", i, trade.Action, err)
				}
			}

			// Verify expected positions
			positions, err := positionStore.GetOpenPositions(traderID)
			if err != nil {
				t.Fatalf("Failed to get positions: %v", err)
			}

			if len(positions) != len(scenario.ExpectedPos) {
				t.Errorf("Expected %d open positions, got %d", len(scenario.ExpectedPos), len(positions))
				for _, p := range positions {
					t.Errorf("  Got: %s %s qty=%.4f status=%s", p.Symbol, p.Side, p.Quantity, p.Status)
				}
				return
			}

			// Verify each expected position
			for _, expected := range scenario.ExpectedPos {
				found := false
				for _, actual := range positions {
					if actual.Symbol == expected.Symbol && actual.Side == expected.Side {
						found = true
						if actual.Quantity != expected.Quantity {
							t.Errorf("Position %s %s: expected qty %.4f, got %.4f",
								expected.Symbol, expected.Side, expected.Quantity, actual.Quantity)
						}
						if actual.Status != expected.Status {
							t.Errorf("Position %s %s: expected status %s, got %s",
								expected.Symbol, expected.Side, expected.Status, actual.Status)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected position not found: %s %s", expected.Symbol, expected.Side)
				}
			}
		})
	}
}

// TestAllExchangesStandardScenarios runs standard scenarios for all exchanges
func TestAllExchangesStandardScenarios(t *testing.T) {
	exchanges := []string{"hyperliquid", "binance", "bybit", "okx", "bitget", "aster", "lighter"}

	for _, exchange := range exchanges {
		t.Run(exchange, func(t *testing.T) {
			runStandardTests(t, exchange)
		})
	}
}

// TestPositionAccumulationBug tests that positions don't accumulate incorrectly
func TestPositionAccumulationBug(t *testing.T) {
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

	// Simulate many trades that should cancel out
	// This tests that we don't accumulate positions incorrectly
	for i := 0; i < 10; i++ {
		// Open Long
		err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			"ETHUSDT", "LONG", "open_long",
			0.1, 3500+float64(i*10), 0.5, 0,
			time.Now().Add(time.Duration(i*2)*time.Second).UnixMilli(),
			"",
		)
		if err != nil {
			t.Fatalf("Failed to open long %d: %v", i, err)
		}

		// Close Long
		err = posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			"ETHUSDT", "LONG", "close_long",
			0.1, 3600+float64(i*10), 0.5, 10,
			time.Now().Add(time.Duration(i*2+1)*time.Second).UnixMilli(),
			"",
		)
		if err != nil {
			t.Fatalf("Failed to close long %d: %v", i, err)
		}
	}

	// Should have 0 open positions
	positions, err := positionStore.GetOpenPositions(traderID)
	if err != nil {
		t.Fatalf("Failed to get positions: %v", err)
	}

	if len(positions) != 0 {
		t.Errorf("Expected 0 positions after 10 open/close cycles, got %d", len(positions))
		for _, p := range positions {
			t.Errorf("  Unexpected: %s %s qty=%.4f", p.Symbol, p.Side, p.Quantity)
		}
	}

	// Should have 10 closed positions with positive PnL
	allPositions, err := positionStore.GetClosedPositions(traderID, 100)
	if err != nil {
		t.Fatalf("Failed to get closed positions: %v", err)
	}

	closedCount := 0
	totalPnL := 0.0
	for _, p := range allPositions {
		if p.Status == "CLOSED" {
			closedCount++
			totalPnL += p.RealizedPnL
		}
	}

	if closedCount != 10 {
		t.Errorf("Expected 10 closed positions, got %d", closedCount)
	}

	if totalPnL <= 0 {
		t.Errorf("Expected positive total PnL, got %.2f", totalPnL)
	}
}

// TestQuantityPrecision tests handling of quantity precision issues
func TestQuantityPrecision(t *testing.T) {
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
	exchangeType := "test"

	// Open position
	err = posBuilder.ProcessTrade(
		traderID, exchangeID, exchangeType,
		"BTCUSDT", "LONG", "open_long",
		0.01, 50000, 1.0, 0,
		time.Now().UnixMilli(),
		"",
	)
	if err != nil {
		t.Fatalf("Failed to open: %v", err)
	}

	// Close with slightly different quantity due to precision (0.00999999 vs 0.01)
	// Should still close fully within tolerance
	err = posBuilder.ProcessTrade(
		traderID, exchangeID, exchangeType,
		"BTCUSDT", "LONG", "close_long",
		0.00999999, 51000, 1.0, 10,
		time.Now().Add(time.Second).UnixMilli(),
		"",
	)
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Should have 0 open positions (within tolerance)
	positions, err := positionStore.GetOpenPositions(traderID)
	if err != nil {
		t.Fatalf("Failed to get positions: %v", err)
	}

	if len(positions) != 0 {
		t.Errorf("Expected 0 positions (precision tolerance), got %d", len(positions))
	}
}
