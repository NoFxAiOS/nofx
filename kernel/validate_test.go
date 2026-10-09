package kernel

import (
	"testing"
)

// TestLeverageFallback tests automatic correction when leverage exceeds limit
func TestLeverageFallback(t *testing.T) {
	tests := []struct {
		name            string
		decision        Decision
		accountEquity   float64
		btcEthLeverage  int
		altcoinLeverage int
		wantLeverage    int // Expected leverage after correction
		wantError       bool
	}{
		{
			name: "Altcoin leverage exceeded - auto-correct to limit",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        20, // Exceeds limit
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5, // Limit 5x
			wantLeverage:    5, // Should be corrected to 5
			wantError:       false,
		},
		{
			name: "BTC leverage exceeded - auto-correct to limit",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        20, // Exceeds limit
				PositionSizeUSD: 1000,
				StopLoss:        90000,
				TakeProfit:      110000,
			},
			accountEquity:   100,
			btcEthLeverage:  10, // Limit 10x
			altcoinLeverage: 5,
			wantLeverage:    10, // Should be corrected to 10
			wantError:       false,
		},
		{
			name: "Leverage within limit - no correction",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "open_short",
				Leverage:        5, // Not exceeded
				PositionSizeUSD: 500,
				StopLoss:        4000,
				TakeProfit:      3000,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    5, // Stays unchanged
			wantError:       false,
		},
		{
			name: "Leverage is 0 - should error",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        0, // Invalid
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    0,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use default position value ratios for testing (10x for BTC/ETH, 1.5x for altcoins)
			err := validateDecision(&tt.decision, tt.accountEquity, tt.btcEthLeverage, tt.altcoinLeverage, 10.0, 1.5)

			// Check error status
			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// If shouldn't error, check if leverage was correctly corrected
			if !tt.wantError && tt.decision.Leverage != tt.wantLeverage {
				t.Errorf("Leverage not corrected: got %d, want %d", tt.decision.Leverage, tt.wantLeverage)
			}
		})
	}
}

func TestClaw402XyzAllowsFullTenXNotional(t *testing.T) {
	decision := Decision{
		Symbol:          "xyz:SP500",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 306.8,
		StopLoss:        95,
		TakeProfit:      120,
	}

	if err := validateDecision(&decision, 30.68, 10, 10, 10.0, 10.0); err != nil {
		t.Fatalf("xyz TradeFi Claw402 full 10x notional should pass validation: %v", err)
	}
}

func TestSignalManagedDecisionAllowsZeroTakeProfitWithProtectiveStop(t *testing.T) {
	decision := Decision{
		Symbol:          "BTC",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 300,
		StopLoss:        95000,
		TakeProfit:      0,
	}

	if err := validateDecisionForMode(&decision, 30, 10, 10, 10.0, 10.0, true); err != nil {
		t.Fatalf("signal-managed open with a protective stop should pass validation: %v", err)
	}
}

func TestSignalManagedDecisionStillRequiresProtectiveStop(t *testing.T) {
	decision := Decision{
		Symbol:          "BTC",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 300,
		StopLoss:        0,
		TakeProfit:      0,
	}

	if err := validateDecisionForMode(&decision, 30, 10, 10, 10.0, 10.0, true); err == nil {
		t.Fatal("signal-managed open without a protective stop should fail validation")
	}
}

func TestFixedExitDecisionStillRequiresTakeProfit(t *testing.T) {
	decision := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 300,
		StopLoss:        95000,
		TakeProfit:      0,
	}

	if err := validateDecision(&decision, 30, 10, 10, 10.0, 10.0); err == nil {
		t.Fatal("ordinary fixed-exit strategy should still reject zero take profit")
	}
}

// contains checks if string contains substring (helper function)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
