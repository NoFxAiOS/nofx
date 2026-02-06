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

// ============================================================================
// Grid Direction Tests
// ============================================================================

func TestGetBuySellRatio(t *testing.T) {
	tests := []struct {
		name      string
		direction market.GridDirection
		biasRatio float64
		wantBuy   float64
		wantSell  float64
	}{
		{"neutral", market.GridDirectionNeutral, 0.7, 0.5, 0.5},
		{"long", market.GridDirectionLong, 0.7, 1.0, 0.0},
		{"short", market.GridDirectionShort, 0.7, 0.0, 1.0},
		{"long_bias_default", market.GridDirectionLongBias, 0.7, 0.7, 0.3},
		{"short_bias_default", market.GridDirectionShortBias, 0.7, 0.3, 0.7},
		{"long_bias_custom", market.GridDirectionLongBias, 0.8, 0.8, 0.2},
		{"short_bias_custom", market.GridDirectionShortBias, 0.8, 0.2, 0.8},
		{"invalid_bias_uses_default", market.GridDirectionLongBias, 0, 0.7, 0.3},
		{"negative_bias_uses_default", market.GridDirectionLongBias, -1, 0.7, 0.3},
	}

	const tolerance = 0.0001
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buy, sell := tt.direction.GetBuySellRatio(tt.biasRatio)
			buyDiff := buy - tt.wantBuy
			sellDiff := sell - tt.wantSell
			if buyDiff < -tolerance || buyDiff > tolerance || sellDiff < -tolerance || sellDiff > tolerance {
				t.Errorf("GetBuySellRatio(%v, %v) = (%v, %v), want (%v, %v)",
					tt.direction, tt.biasRatio, buy, sell, tt.wantBuy, tt.wantSell)
			}
		})
	}
}

func TestDetermineGridDirection(t *testing.T) {
	box := &market.BoxData{
		ShortUpper:   100,
		ShortLower:   90,
		MidUpper:     105,
		MidLower:     85,
		LongUpper:    110,
		LongLower:    80,
		CurrentPrice: 95,
	}

	tests := []struct {
		name             string
		currentDirection market.GridDirection
		breakoutLevel    market.BreakoutLevel
		direction        string
		expected         market.GridDirection
	}{
		// Short box breakouts
		{
			name:             "short_breakout_up_neutral",
			currentDirection: market.GridDirectionNeutral,
			breakoutLevel:    market.BreakoutShort,
			direction:        "up",
			expected:         market.GridDirectionLongBias,
		},
		{
			name:             "short_breakout_down_neutral",
			currentDirection: market.GridDirectionNeutral,
			breakoutLevel:    market.BreakoutShort,
			direction:        "down",
			expected:         market.GridDirectionShortBias,
		},
		// Mid box breakouts
		{
			name:             "mid_breakout_up",
			currentDirection: market.GridDirectionLongBias,
			breakoutLevel:    market.BreakoutMid,
			direction:        "up",
			expected:         market.GridDirectionLong,
		},
		{
			name:             "mid_breakout_down",
			currentDirection: market.GridDirectionShortBias,
			breakoutLevel:    market.BreakoutMid,
			direction:        "down",
			expected:         market.GridDirectionShort,
		},
		// Long box breakout - maintains current (emergency handling)
		{
			name:             "long_breakout_maintains",
			currentDirection: market.GridDirectionLong,
			breakoutLevel:    market.BreakoutLong,
			direction:        "up",
			expected:         market.GridDirectionLong,
		},
		// No breakout - tests recovery logic
		{
			name:             "no_breakout_neutral_stays",
			currentDirection: market.GridDirectionNeutral,
			breakoutLevel:    market.BreakoutNone,
			direction:        "",
			expected:         market.GridDirectionNeutral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineGridDirection(box, tt.currentDirection, tt.breakoutLevel, tt.direction)
			if result != tt.expected {
				t.Errorf("determineGridDirection() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetermineRecoveryDirection(t *testing.T) {
	box := &market.BoxData{
		ShortUpper:   100,
		ShortLower:   90,
		MidUpper:     105,
		MidLower:     85,
		LongUpper:    110,
		LongLower:    80,
		CurrentPrice: 95, // Inside short box
	}

	tests := []struct {
		name             string
		price            float64
		currentDirection market.GridDirection
		expected         market.GridDirection
	}{
		// Inside short box - should recover
		{"long_to_long_bias", 95, market.GridDirectionLong, market.GridDirectionLongBias},
		{"long_bias_to_neutral", 95, market.GridDirectionLongBias, market.GridDirectionNeutral},
		{"short_to_short_bias", 95, market.GridDirectionShort, market.GridDirectionShortBias},
		{"short_bias_to_neutral", 95, market.GridDirectionShortBias, market.GridDirectionNeutral},
		{"neutral_stays_neutral", 95, market.GridDirectionNeutral, market.GridDirectionNeutral},

		// Outside short box - should maintain
		{"long_outside_stays", 101, market.GridDirectionLong, market.GridDirectionLong},
		{"short_outside_stays", 89, market.GridDirectionShort, market.GridDirectionShort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineRecoveryDirection(tt.price, box, tt.currentDirection)
			if result != tt.expected {
				t.Errorf("determineRecoveryDirection(%v, %v) = %v, want %v",
					tt.price, tt.currentDirection, result, tt.expected)
			}
		})
	}
}

func TestGetBreakoutActionWithDirection(t *testing.T) {
	tests := []struct {
		name                  string
		level                 market.BreakoutLevel
		enableDirectionAdjust bool
		expected              BreakoutAction
	}{
		// Direction adjustment disabled - original behavior
		{"short_disabled", market.BreakoutShort, false, BreakoutActionReducePosition},
		{"mid_disabled", market.BreakoutMid, false, BreakoutActionPauseGrid},
		{"long_disabled", market.BreakoutLong, false, BreakoutActionCloseAll},

		// Direction adjustment enabled
		{"short_enabled", market.BreakoutShort, true, BreakoutActionAdjustDirection},
		{"mid_enabled", market.BreakoutMid, true, BreakoutActionAdjustDirection},
		{"long_enabled", market.BreakoutLong, true, BreakoutActionCloseAll}, // Long always triggers emergency
		{"none_enabled", market.BreakoutNone, true, BreakoutActionNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := getBreakoutActionWithDirection(tt.level, tt.enableDirectionAdjust)
			if action != tt.expected {
				t.Errorf("getBreakoutActionWithDirection(%v, %v) = %v, want %v",
					tt.level, tt.enableDirectionAdjust, action, tt.expected)
			}
		})
	}
}

func TestShouldRecoverDirection(t *testing.T) {
	box := &market.BoxData{
		ShortUpper:   100,
		ShortLower:   90,
		MidUpper:     105,
		MidLower:     85,
		LongUpper:    110,
		LongLower:    80,
		CurrentPrice: 95,
	}

	tests := []struct {
		name      string
		price     float64
		direction market.GridDirection
		expected  bool
	}{
		{"neutral_inside_no_recovery", 95, market.GridDirectionNeutral, false},
		{"long_inside_should_recover", 95, market.GridDirectionLong, true},
		{"long_outside_no_recovery", 101, market.GridDirectionLong, false},
		{"short_inside_should_recover", 95, market.GridDirectionShort, true},
		{"short_outside_no_recovery", 89, market.GridDirectionShort, false},
		{"long_bias_inside_should_recover", 95, market.GridDirectionLongBias, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			box.CurrentPrice = tt.price
			result := shouldRecoverDirection(box, tt.direction)
			if result != tt.expected {
				t.Errorf("shouldRecoverDirection(price=%v, %v) = %v, want %v",
					tt.price, tt.direction, result, tt.expected)
			}
		})
	}
}
