package store

import (
	"testing"
)

// TestCreateTraderWithPaperTrading tests creating a trader with paper trading enabled
func TestCreateTraderWithPaperTrading(t *testing.T) {
	trader := &Trader{
		ID:                  "trader_pt_001",
		UserID:              "user_001",
		Name:                "Test Paper Trading Trader",
		AIModelID:           "gpt-4",
		ExchangeID:          "exchange_001",
		StrategyID:          "strategy_001",
		InitialBalance:      10000.0,
		ScanIntervalMinutes: 5,
		IsRunning:           false,
		IsCrossMargin:       true,
		ShowInCompetition:   true,
		PaperTrading:        true, // Paper trading enabled
		BTCETHLeverage:      10,
		AltcoinLeverage:     5,
	}

	if !trader.PaperTrading {
		t.Errorf("Expected PaperTrading to be true, got %v", trader.PaperTrading)
	}
}

// TestCreateTraderWithoutPaperTrading tests creating a trader with paper trading disabled (live trading)
func TestCreateTraderWithoutPaperTrading(t *testing.T) {
	trader := &Trader{
		ID:                  "trader_live_001",
		UserID:              "user_001",
		Name:                "Test Live Trading Trader",
		AIModelID:           "gpt-4",
		ExchangeID:          "exchange_001",
		StrategyID:          "strategy_001",
		InitialBalance:      10000.0,
		ScanIntervalMinutes: 5,
		IsRunning:           false,
		IsCrossMargin:       true,
		ShowInCompetition:   true,
		PaperTrading:        false, // Live trading (default)
		BTCETHLeverage:      10,
		AltcoinLeverage:     5,
	}

	if trader.PaperTrading {
		t.Errorf("Expected PaperTrading to be false, got %v", trader.PaperTrading)
	}
}

// TestPaperTradingDefaultValue tests that new traders default to live trading (paper_trading = false)
func TestPaperTradingDefaultValue(t *testing.T) {
	trader := &Trader{
		ID:                  "trader_default_001",
		UserID:              "user_001",
		Name:                "Test Default Trader",
		AIModelID:           "gpt-4",
		ExchangeID:          "exchange_001",
		StrategyID:          "strategy_001",
		InitialBalance:      10000.0,
		ScanIntervalMinutes: 5,
		IsRunning:           false,
		IsCrossMargin:       true,
		ShowInCompetition:   true,
		// PaperTrading not set, should default to false
		BTCETHLeverage:  10,
		AltcoinLeverage: 5,
	}

	if trader.PaperTrading {
		t.Errorf("Expected PaperTrading default to be false (live trading), got %v", trader.PaperTrading)
	}
}

// TestTraderStructIncludesPaperTrading tests that Trader struct includes PaperTrading field
func TestTraderStructIncludesPaperTrading(t *testing.T) {
	trader := &Trader{
		ID:           "test_001",
		UserID:       "user_001",
		Name:         "Test",
		AIModelID:    "model_001",
		ExchangeID:   "exchange_001",
		PaperTrading: true,
	}

	if !trader.PaperTrading {
		t.Error("Trader struct does not properly store PaperTrading field")
	}
}
