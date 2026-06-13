package market

import (
	"slices"
	"testing"
	"time"
)

func TestNormalizeTimeframe(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid lowercase minute", input: "1m", want: "1m"},
		{name: "valid lowercase hour", input: "4h", want: "4h"},
		{name: "valid lowercase day", input: "1d", want: "1d"},
		{name: "uppercase normalized", input: "1H", want: "1h"},
		{name: "mixed case normalized", input: "15M", want: "15m"},
		{name: "uppercase day", input: "1D", want: "1d"},
		{name: "leading and trailing whitespace", input: "  30m  ", want: "30m"},
		{name: "whitespace and uppercase", input: " 12H ", want: "12h"},
		{name: "empty string", input: "", wantErr: true},
		{name: "whitespace only", input: "   ", wantErr: true},
		{name: "unsupported value", input: "7m", wantErr: true},
		{name: "unsupported week", input: "1w", wantErr: true},
		{name: "garbage input", input: "abc", wantErr: true},
		{name: "internal whitespace not trimmed", input: "1 m", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTimeframe(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeTimeframe(%q) = %q, want error", tt.input, got)
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

func TestTFDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "one minute", input: "1m", want: time.Minute},
		{name: "three minutes", input: "3m", want: 3 * time.Minute},
		{name: "five minutes", input: "5m", want: 5 * time.Minute},
		{name: "fifteen minutes", input: "15m", want: 15 * time.Minute},
		{name: "thirty minutes", input: "30m", want: 30 * time.Minute},
		{name: "one hour", input: "1h", want: time.Hour},
		{name: "two hours", input: "2h", want: 2 * time.Hour},
		{name: "four hours", input: "4h", want: 4 * time.Hour},
		{name: "six hours", input: "6h", want: 6 * time.Hour},
		{name: "twelve hours", input: "12h", want: 12 * time.Hour},
		{name: "one day", input: "1d", want: 24 * time.Hour},
		{name: "uppercase with whitespace", input: " 1D ", want: 24 * time.Hour},
		{name: "empty string", input: "", wantErr: true},
		{name: "unsupported value", input: "2d", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TFDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TFDuration(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("TFDuration(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("TFDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSupportedTimeframes(t *testing.T) {
	got := SupportedTimeframes()

	if len(got) == 0 {
		t.Fatal("SupportedTimeframes() returned empty slice")
	}

	if !slices.IsSorted(got) {
		t.Errorf("SupportedTimeframes() not sorted: %v", got)
	}

	for _, required := range []string{"1m", "1d"} {
		if !slices.Contains(got, required) {
			t.Errorf("SupportedTimeframes() missing %q: %v", required, got)
		}
	}

	// Every advertised timeframe must round-trip through NormalizeTimeframe and TFDuration.
	for _, tf := range got {
		norm, err := NormalizeTimeframe(tf)
		if err != nil {
			t.Errorf("NormalizeTimeframe(%q) unexpected error: %v", tf, err)
		}
		if norm != tf {
			t.Errorf("NormalizeTimeframe(%q) = %q, want identity", tf, norm)
		}
		if d, err := TFDuration(tf); err != nil || d <= 0 {
			t.Errorf("TFDuration(%q) = %v, %v; want positive duration and nil error", tf, d, err)
		}
	}
}
