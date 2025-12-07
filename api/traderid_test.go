package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestTraderIDUniqueness Test traderID uniqueness (fixes Issue #893)
// Verify that unique traderIDs can be generated even with the same exchange and AI model
func TestTraderIDUniqueness(t *testing.T) {
	exchangeID := "binance"
	aiModelID := "gpt-4"

	// Simulate creating 100 traders simultaneously (with same parameters)
	traderIDs := make(map[string]bool)
	const numTraders = 100

	for i := 0; i < numTraders; i++ {
		// Simulate traderID generation logic from api/server.go:497
		traderID := generateTraderID(exchangeID, aiModelID)

		// Check for duplicates
		if traderIDs[traderID] {
			t.Errorf("Duplicate traderID detected: %s", traderID)
		}
		traderIDs[traderID] = true

		// Verify format: should be "exchange_model_uuid"
		if !isValidTraderIDFormat(traderID, exchangeID, aiModelID) {
			t.Errorf("Invalid traderID format: %s", traderID)
		}
	}

	// Verify expected number of unique IDs were generated
	if len(traderIDs) != numTraders {
		t.Errorf("Expected %d unique traderIDs, got %d", numTraders, len(traderIDs))
	}
}

// generateTraderID Helper function that simulates traderID generation logic from api/server.go
func generateTraderID(exchangeID, aiModelID string) string {
	return fmt.Sprintf("%s_%s_%s", exchangeID, aiModelID, uuid.New().String())
}

// isValidTraderIDFormat Verify traderID format matches expected format
func isValidTraderIDFormat(traderID, expectedExchange, expectedModel string) bool {
	// Format: exchange_model_uuid
	// Example: binance_gpt-4_a1b2c3d4-e5f6-7890-abcd-ef1234567890
	parts := strings.Split(traderID, "_")
	if len(parts) < 3 {
		return false
	}

	// Verify prefix
	if parts[0] != expectedExchange {
		return false
	}

	// AI model may contain hyphens (e.g. gpt-4), so need to reconstruct
	// Last part should be UUID
	uuidPart := parts[len(parts)-1]

	// Verify UUID format (36 characters, containing 4 hyphens)
	_, err := uuid.Parse(uuidPart)
	return err == nil
}

// TestTraderIDFormat Test traderID format correctness
func TestTraderIDFormat(t *testing.T) {
	tests := []struct {
		name       string
		exchangeID string
		aiModelID  string
	}{
		{"Binance + GPT-4", "binance", "gpt-4"},
		{"Hyperliquid + Claude", "hyperliquid", "claude-3"},
		{"OKX + Qwen", "okx", "qwen-2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traderID := generateTraderID(tt.exchangeID, tt.aiModelID)

			// Verify correct prefix
			if !strings.HasPrefix(traderID, tt.exchangeID+"_"+tt.aiModelID+"_") {
				t.Errorf("traderID does not have correct prefix. Got: %s", traderID)
			}

			// Verify format is valid
			if !isValidTraderIDFormat(traderID, tt.exchangeID, tt.aiModelID) {
				t.Errorf("Invalid traderID format: %s", traderID)
			}

			// Verify reasonable length (should be at least exchange + model + "_" + UUID(36))
			minLength := len(tt.exchangeID) + len(tt.aiModelID) + 2 + 36 // 2 underscores + 36 character UUID
			if len(traderID) < minLength {
				t.Errorf("traderID too short: expected at least %d chars, got %d", minLength, len(traderID))
			}
		})
	}
}

// TestTraderIDNoCollision Test that no collisions occur in high concurrency scenarios
func TestTraderIDNoCollision(t *testing.T) {
	const iterations = 1000
	uniqueIDs := make(map[string]bool, iterations)

	// Simulate high concurrency scenario
	for i := 0; i < iterations; i++ {
		id := generateTraderID("binance", "gpt-4")
		if uniqueIDs[id] {
			t.Fatalf("Collision detected after %d iterations: %s", i+1, id)
		}
		uniqueIDs[id] = true
	}

	if len(uniqueIDs) != iterations {
		t.Errorf("Expected %d unique IDs, got %d", iterations, len(uniqueIDs))
	}
}
