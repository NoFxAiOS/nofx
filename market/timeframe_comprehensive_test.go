package market

import (
	"testing"
	"time"
)

// TestAllTimeframesSupported verifies all requested timeframes are supported
func TestAllTimeframesSupported(t *testing.T) {
	requestedTimeframes := []string{"3m", "5m", "30m", "1h", "4h"}

	for _, tf := range requestedTimeframes {
		t.Run(tf, func(t *testing.T) {
			normalized, err := NormalizeTimeframe(tf)
			if err != nil {
				t.Errorf("Timeframe %s should be supported but got error: %v", tf, err)
			}
			if normalized != tf {
				t.Errorf("Expected normalized timeframe %s, got %s", tf, normalized)
			}

			// Verify duration is correct
			duration, err := TFDuration(tf)
			if err != nil {
				t.Errorf("Failed to get duration for %s: %v", tf, err)
			}

			var expectedDuration time.Duration
			switch tf {
			case "3m":
				expectedDuration = 3 * time.Minute
			case "5m":
				expectedDuration = 5 * time.Minute
			case "30m":
				expectedDuration = 30 * time.Minute
			case "1h":
				expectedDuration = time.Hour
			case "4h":
				expectedDuration = 4 * time.Hour
			}

			if duration != expectedDuration {
				t.Errorf("Expected duration %v for %s, got %v", expectedDuration, tf, duration)
			}
		})
	}
}

// TestSupportedTimeframesContainsAll verifies SupportedTimeframes() includes all required timeframes
func TestSupportedTimeframesContainsAll(t *testing.T) {
	supported := SupportedTimeframes()
	requiredTimeframes := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d"}

	for _, required := range requiredTimeframes {
		found := false
		for _, sup := range supported {
			if sup == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Required timeframe %s not found in supported timeframes: %v", required, supported)
		}
	}
}

// TestTimeframeDurations verifies all timeframe durations are correct
func TestTimeframeDurations(t *testing.T) {
	tests := []struct {
		timeframe string
		expected  time.Duration
	}{
		{"1m", time.Minute},
		{"3m", 3 * time.Minute},
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
		{"2h", 2 * time.Hour},
		{"4h", 4 * time.Hour},
		{"6h", 6 * time.Hour},
		{"12h", 12 * time.Hour},
		{"1d", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.timeframe, func(t *testing.T) {
			duration, err := TFDuration(tt.timeframe)
			if err != nil {
				t.Errorf("Failed to get duration for %s: %v", tt.timeframe, err)
			}
			if duration != tt.expected {
				t.Errorf("Expected duration %v for %s, got %v", tt.expected, tt.timeframe, duration)
			}
		})
	}
}
