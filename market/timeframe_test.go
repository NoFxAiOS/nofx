package market

import (
	"testing"
)

func TestNormalizeTimeframe(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{"uppercase 5M", "5M", "5m", false},
		{"uppercase 15M", "15M", "15m", false},
		{"uppercase 1H", "1H", "1h", false},
		{"trimmed 1h", " 1h ", "1h", false},
		{"already lowercase", "5m", "5m", false},
		{"invalid", "invalid", "", true},
		{"empty", "", "", true},
		{"unsupported 2h", "2H", "2h", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTimeframe(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("NormalizeTimeframe(%q) expected error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeTimeframe(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeTimeframe(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
