package store

import (
	"math"
	"testing"
)

func TestGPT6Pricing(t *testing.T) {
	if got, want := GetModelPrice("gpt-6"), 0.24; got != want {
		t.Fatalf("GetModelPrice(gpt-6) = %v, want %v", got, want)
	}

	tests := []struct {
		name             string
		promptTokens     int
		completionTokens int
		want             float64
	}{
		{
			name:             "standard context",
			promptTokens:     100000,
			completionTokens: 10000,
			want:             (100000*12.5 + 10000*50.0) / 1e6 * uptoSafetyMargin,
		},
		{
			name:             "long context",
			promptTokens:     272001,
			completionTokens: 10000,
			want:             (272001*25.0 + 10000*75.0) / 1e6 * uptoSafetyMargin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ComputeUsageCost("gpt-6", tt.promptTokens, tt.completionTokens)
			if !ok {
				t.Fatal("ComputeUsageCost(gpt-6) reported unsupported model")
			}
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("ComputeUsageCost(gpt-6) = %v, want %v", got, tt.want)
			}
		})
	}
}
