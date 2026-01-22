// backtest/risk/types_test.go
package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPosition_Direction(t *testing.T) {
	tests := []struct {
		name     string
		side     string
		expected float64
	}{
		{"long position", "long", 1.0},
		{"short position", "short", -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Position{Side: tt.side}
			assert.Equal(t, tt.expected, p.Direction())
		})
	}
}

func TestPosition_Notional(t *testing.T) {
	p := &Position{
		Quantity:  10.0,
		MarkPrice: 50000.0,
	}
	assert.Equal(t, 500000.0, p.Notional())
}
