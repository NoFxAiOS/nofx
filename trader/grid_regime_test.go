package trader

import (
	"nofx/market"
	"testing"
)

func TestClassifyRegimeLevel(t *testing.T) {
	tests := []struct {
		name           string
		bollingerWidth float64
		atr14Pct       float64
		expected       market.RegimeLevel
	}{
		{"narrow", 1.5, 0.8, market.RegimeLevelNarrow},
		{"standard", 2.5, 1.5, market.RegimeLevelStandard},
		{"wide", 3.5, 2.5, market.RegimeLevelWide},
		{"volatile", 5.0, 4.0, market.RegimeLevelVolatile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRegimeLevel(tt.bollingerWidth, tt.atr14Pct)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDetectBoxBreakout(t *testing.T) {
	box := &market.BoxData{
		ShortUpper:   100,
		ShortLower:   90,
		MidUpper:     105,
		MidLower:     85,
		LongUpper:    110,
		LongLower:    80,
		CurrentPrice: 95,
	}

	// No breakout
	level, direction := detectBoxBreakout(box)
	if level != market.BreakoutNone {
		t.Errorf("Expected no breakout, got %v", level)
	}

	// Short breakout up
	box.CurrentPrice = 101
	level, direction = detectBoxBreakout(box)
	if level != market.BreakoutShort || direction != "up" {
		t.Errorf("Expected short breakout up, got %v %v", level, direction)
	}

	// Mid breakout down
	box.CurrentPrice = 84
	level, direction = detectBoxBreakout(box)
	if level != market.BreakoutMid || direction != "down" {
		t.Errorf("Expected mid breakout down, got %v %v", level, direction)
	}

	// Long breakout up
	box.CurrentPrice = 112
	level, direction = detectBoxBreakout(box)
	if level != market.BreakoutLong || direction != "up" {
		t.Errorf("Expected long breakout up, got %v %v", level, direction)
	}
}

func TestBreakoutConfirmation(t *testing.T) {
	state := &BreakoutState{
		Level:        market.BreakoutNone,
		Direction:    "",
		ConfirmCount: 0,
	}

	// First detection
	confirmed := confirmBreakout(state, market.BreakoutShort, "up")
	if confirmed || state.ConfirmCount != 1 {
		t.Errorf("Expected not confirmed, count=1, got confirmed=%v count=%d", confirmed, state.ConfirmCount)
	}

	// Second confirmation
	confirmed = confirmBreakout(state, market.BreakoutShort, "up")
	if confirmed || state.ConfirmCount != 2 {
		t.Errorf("Expected not confirmed, count=2, got confirmed=%v count=%d", confirmed, state.ConfirmCount)
	}

	// Third confirmation - should confirm
	confirmed = confirmBreakout(state, market.BreakoutShort, "up")
	if !confirmed || state.ConfirmCount != 3 {
		t.Errorf("Expected confirmed, count=3, got confirmed=%v count=%d", confirmed, state.ConfirmCount)
	}

	// Reset on price return
	state.ConfirmCount = 2
	confirmed = confirmBreakout(state, market.BreakoutNone, "")
	if state.ConfirmCount != 0 {
		t.Errorf("Expected count reset to 0, got %d", state.ConfirmCount)
	}
}

func TestGetBreakoutAction(t *testing.T) {
	tests := []struct {
		level    market.BreakoutLevel
		expected BreakoutAction
	}{
		{market.BreakoutNone, BreakoutActionNone},
		{market.BreakoutShort, BreakoutActionReducePosition},
		{market.BreakoutMid, BreakoutActionPauseGrid},
		{market.BreakoutLong, BreakoutActionCloseAll},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			action := getBreakoutAction(tt.level)
			if action != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, action)
			}
		})
	}
}
