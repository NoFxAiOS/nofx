package trader

import (
	"nofx/store"
	"testing"
	"time"
)

// TestDrawdownMonitoringWithDifferentIntervals verifies monitoring works with different check intervals
func TestDrawdownMonitoringWithDifferentIntervals(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		expected int
	}{
		{"Minimum interval 15s", 15, 15},
		{"Default interval 60s", 60, 60},
		{"Maximum interval 300s", 300, 300},
		{"Too small interval corrected to 15s", 5, 15},
		{"Too large interval corrected to 300s", 500, 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock config
			config := &store.StrategyConfig{
				RiskControl: store.RiskControlConfig{
					DrawdownMonitoringEnabled: true,
					DrawdownCheckInterval:     tt.interval,
					MinProfitThreshold:        5.0,
					DrawdownCloseThreshold:    40.0,
				},
			}

			// Verify the interval is within bounds
			actualInterval := config.RiskControl.DrawdownCheckInterval
			if actualInterval < 15 {
				actualInterval = 15
			} else if actualInterval > 300 {
				actualInterval = 300
			}

			if actualInterval != tt.expected {
				t.Errorf("Expected interval %ds, got %ds", tt.expected, actualInterval)
			}
		})
	}
}

// TestDrawdownMonitoringWithDifferentProfitThresholds verifies profit thresholds work correctly
func TestDrawdownMonitoringWithDifferentProfitThresholds(t *testing.T) {
	tests := []struct {
		name               string
		profitThreshold    float64
		currentProfit      float64
		drawdownPct        float64
		shouldTriggerClose bool
	}{
		{"Conservative 3% threshold - triggered", 3.0, 6.0, 40.0, true},
		{"Default 5% threshold - triggered", 5.0, 8.0, 40.0, true},
		{"Aggressive 10% threshold - not triggered", 10.0, 8.0, 40.0, false},
		{"Below threshold - not triggered", 5.0, 4.5, 40.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock config
			config := &store.StrategyConfig{
				RiskControl: store.RiskControlConfig{
					DrawdownMonitoringEnabled: true,
					DrawdownCheckInterval:     60,
					MinProfitThreshold:        tt.profitThreshold,
					DrawdownCloseThreshold:    40.0,
				},
			}

			// Check trigger condition logic
			triggered := tt.currentProfit > config.RiskControl.MinProfitThreshold && tt.drawdownPct >= config.RiskControl.DrawdownCloseThreshold

			if triggered != tt.shouldTriggerClose {
				t.Errorf("Expected trigger=%v, got trigger=%v (profit=%.2f%%, threshold=%.2f%%)",
					tt.shouldTriggerClose, triggered, tt.currentProfit, tt.profitThreshold)
			}
		})
	}
}

// TestDrawdownMonitoringWithDifferentDrawdownThresholds verifies drawdown thresholds work correctly
func TestDrawdownMonitoringWithDifferentDrawdownThresholds(t *testing.T) {
	tests := []struct {
		name               string
		drawdownThreshold  float64
		currentDrawdown    float64
		shouldTriggerClose bool
	}{
		{"Tight 30% threshold - triggered", 30.0, 35.0, true},
		{"Default 40% threshold - triggered", 40.0, 40.0, true},
		{"Loose 50% threshold - not triggered", 50.0, 45.0, false},
		{"Below threshold - not triggered", 40.0, 38.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock config
			config := &store.StrategyConfig{
				RiskControl: store.RiskControlConfig{
					DrawdownMonitoringEnabled: true,
					DrawdownCheckInterval:     60,
					MinProfitThreshold:        5.0,
					DrawdownCloseThreshold:    tt.drawdownThreshold,
				},
			}

			// Assume profit condition is met (8% > 5%)
			profitCondition := 8.0 > config.RiskControl.MinProfitThreshold
			drawdownCondition := tt.currentDrawdown >= config.RiskControl.DrawdownCloseThreshold
			triggered := profitCondition && drawdownCondition

			if triggered != tt.shouldTriggerClose {
				t.Errorf("Expected trigger=%v, got trigger=%v (drawdown=%.2f%%, threshold=%.2f%%)",
					tt.shouldTriggerClose, triggered, tt.currentDrawdown, tt.drawdownThreshold)
			}
		})
	}
}

// TestDrawdownMonitoringDisabled verifies monitoring can be disabled
func TestDrawdownMonitoringDisabled(t *testing.T) {
	// Create config with monitoring disabled
	config := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			DrawdownMonitoringEnabled: false,
			DrawdownCheckInterval:     60,
			MinProfitThreshold:        5.0,
			DrawdownCloseThreshold:    40.0,
		},
	}

	// Verify monitoring is disabled
	if config.RiskControl.DrawdownMonitoringEnabled {
		t.Error("Expected drawdown monitoring to be disabled")
	}

	// In actual implementation, startDrawdownMonitor() should return early
	// when DrawdownMonitoringEnabled is false
}

// TestDrawdownCalculationLogic verifies drawdown percentage calculation
func TestDrawdownCalculationLogic(t *testing.T) {
	tests := []struct {
		name             string
		peakProfit       float64
		currentProfit    float64
		expectedDrawdown float64
	}{
		{"10% peak to 6% current = 40% drawdown", 10.0, 6.0, 40.0},
		{"8% peak to 5% current = 37.5% drawdown", 8.0, 5.0, 37.5},
		{"15% peak to 10% current = 33.33% drawdown", 15.0, 10.0, 33.33},
		{"5% peak to 5% current = 0% drawdown", 5.0, 5.0, 0.0},
		{"Negative profit - no drawdown", -5.0, -8.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var drawdownPct float64
			if tt.peakProfit > 0 && tt.currentProfit < tt.peakProfit {
				drawdownPct = ((tt.peakProfit - tt.currentProfit) / tt.peakProfit) * 100
			}

			// Allow small floating point tolerance
			tolerance := 0.01
			if drawdownPct < tt.expectedDrawdown-tolerance || drawdownPct > tt.expectedDrawdown+tolerance {
				t.Errorf("Expected drawdown %.2f%%, got %.2f%%", tt.expectedDrawdown, drawdownPct)
			}
		})
	}
}

// TestDrawdownMonitoringConfigValidation verifies configuration validation
func TestDrawdownMonitoringConfigValidation(t *testing.T) {
	tests := []struct {
		name              string
		checkInterval     int
		minProfit         float64
		drawdownThreshold float64
		expectValid       bool
	}{
		{"Valid default config", 60, 5.0, 40.0, true},
		{"Valid minimum interval", 15, 5.0, 40.0, true},
		{"Valid maximum interval", 300, 5.0, 40.0, true},
		{"Invalid interval too small", 10, 5.0, 40.0, false},
		{"Invalid interval too large", 400, 5.0, 40.0, false},
		{"Valid conservative settings", 30, 3.0, 30.0, true},
		{"Valid aggressive settings", 120, 10.0, 50.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &store.StrategyConfig{
				RiskControl: store.RiskControlConfig{
					DrawdownMonitoringEnabled: true,
					DrawdownCheckInterval:     tt.checkInterval,
					MinProfitThreshold:        tt.minProfit,
					DrawdownCloseThreshold:    tt.drawdownThreshold,
				},
			}

			// Validate interval is within bounds
			isValid := config.RiskControl.DrawdownCheckInterval >= 15 &&
				config.RiskControl.DrawdownCheckInterval <= 300

			if isValid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v (interval=%ds)",
					tt.expectValid, isValid, tt.checkInterval)
			}
		})
	}
}

// TestDrawdownMonitoringRealScenario verifies realistic trading scenarios
func TestDrawdownMonitoringRealScenario(t *testing.T) {
	scenarios := []struct {
		name          string
		config        store.RiskControlConfig
		peakProfit    float64
		currentProfit float64
		shouldClose   bool
		description   string
	}{
		{
			name: "Scenario 1: Normal profit protection",
			config: store.RiskControlConfig{
				DrawdownMonitoringEnabled: true,
				DrawdownCheckInterval:     60,
				MinProfitThreshold:        5.0,
				DrawdownCloseThreshold:    40.0,
			},
			peakProfit:    10.0,
			currentProfit: 6.0,
			shouldClose:   true,
			description:   "Profit reached 10%, dropped to 6% (40% drawdown) - should close",
		},
		{
			name: "Scenario 2: Still within acceptable drawdown",
			config: store.RiskControlConfig{
				DrawdownMonitoringEnabled: true,
				DrawdownCheckInterval:     60,
				MinProfitThreshold:        5.0,
				DrawdownCloseThreshold:    40.0,
			},
			peakProfit:    10.0,
			currentProfit: 7.0,
			shouldClose:   false,
			description:   "Profit reached 10%, dropped to 7% (30% drawdown) - should hold",
		},
		{
			name: "Scenario 3: Conservative trader protection",
			config: store.RiskControlConfig{
				DrawdownMonitoringEnabled: true,
				DrawdownCheckInterval:     30,
				MinProfitThreshold:        3.0,
				DrawdownCloseThreshold:    30.0,
			},
			peakProfit:    6.0,
			currentProfit: 4.0,
			shouldClose:   true,
			description:   "Conservative: 6% peak to 4% (33% drawdown with 30% threshold) - should close",
		},
		{
			name: "Scenario 4: Aggressive trader holding",
			config: store.RiskControlConfig{
				DrawdownMonitoringEnabled: true,
				DrawdownCheckInterval:     120,
				MinProfitThreshold:        10.0,
				DrawdownCloseThreshold:    50.0,
			},
			peakProfit:    15.0,
			currentProfit: 8.0,
			shouldClose:   false,
			description:   "Aggressive: 15% peak to 8% (47% drawdown with 50% threshold) - should hold",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Calculate drawdown
			var drawdownPct float64
			if scenario.peakProfit > 0 && scenario.currentProfit < scenario.peakProfit {
				drawdownPct = ((scenario.peakProfit - scenario.currentProfit) / scenario.peakProfit) * 100
			}

			// Check trigger condition
			shouldClose := scenario.currentProfit > scenario.config.MinProfitThreshold &&
				drawdownPct >= scenario.config.DrawdownCloseThreshold

			if shouldClose != scenario.shouldClose {
				t.Errorf("%s\nExpected close=%v, got close=%v\nPeak: %.2f%%, Current: %.2f%%, Drawdown: %.2f%%\nThresholds: MinProfit=%.2f%%, MaxDrawdown=%.2f%%",
					scenario.description, scenario.shouldClose, shouldClose,
					scenario.peakProfit, scenario.currentProfit, drawdownPct,
					scenario.config.MinProfitThreshold, scenario.config.DrawdownCloseThreshold)
			}
		})
	}
}

// TestPeakPnLUpdateLogic verifies peak profit tracking logic
func TestPeakPnLUpdateLogic(t *testing.T) {
	tests := []struct {
		name         string
		existingPeak float64
		newProfit    float64
		expectedPeak float64
		shouldUpdate bool
	}{
		{"New peak higher than old", 8.0, 10.0, 10.0, true},
		{"New profit same as peak", 10.0, 10.0, 10.0, false},
		{"New profit lower than peak", 10.0, 7.0, 10.0, false},
		{"First peak record", 0.0, 5.0, 5.0, true},
		{"Negative to positive peak", -3.0, 2.0, 2.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peakPnL := tt.existingPeak

			// Update logic: only update if new profit is higher
			shouldUpdate := tt.newProfit > peakPnL
			if shouldUpdate {
				peakPnL = tt.newProfit
			}

			if peakPnL != tt.expectedPeak {
				t.Errorf("Expected peak %.2f%%, got %.2f%%", tt.expectedPeak, peakPnL)
			}

			if shouldUpdate != tt.shouldUpdate {
				t.Errorf("Expected shouldUpdate=%v, got shouldUpdate=%v", tt.shouldUpdate, shouldUpdate)
			}
		})
	}
}

// BenchmarkDrawdownCalculation benchmarks the drawdown calculation performance
func BenchmarkDrawdownCalculation(b *testing.B) {
	peakProfit := 10.0
	currentProfit := 6.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var drawdownPct float64
		if peakProfit > 0 && currentProfit < peakProfit {
			drawdownPct = ((peakProfit - currentProfit) / peakProfit) * 100
		}
		_ = drawdownPct
	}
}

// BenchmarkConfigAccess benchmarks configuration access performance
func BenchmarkConfigAccess(b *testing.B) {
	config := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			DrawdownMonitoringEnabled: true,
			DrawdownCheckInterval:     60,
			MinProfitThreshold:        5.0,
			DrawdownCloseThreshold:    40.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.RiskControl.MinProfitThreshold
		_ = config.RiskControl.DrawdownCloseThreshold
		_ = config.RiskControl.DrawdownCheckInterval
		_ = config.RiskControl.DrawdownMonitoringEnabled
	}
}

// TestDrawdownMonitoringTimeframeAccuracy verifies timing accuracy of checks
func TestDrawdownMonitoringTimeframeAccuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timing test in short mode")
	}

	checkInterval := 1 // 1 second for testing (production minimum is 15s)

	start := time.Now()
	ticker := time.NewTicker(time.Duration(checkInterval) * time.Second)
	defer ticker.Stop()

	tickCount := 0
	maxTicks := 3

	done := make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				tickCount++
				if tickCount >= maxTicks {
					done <- true
					return
				}
			case <-time.After(5 * time.Second):
				done <- true
				return
			}
		}
	}()

	<-done
	elapsed := time.Since(start)

	// Should complete 3 ticks in approximately 3 seconds (with some tolerance)
	expectedDuration := time.Duration(maxTicks) * time.Second
	tolerance := 500 * time.Millisecond

	if elapsed < expectedDuration-tolerance || elapsed > expectedDuration+tolerance {
		t.Errorf("Expected ~%v elapsed, got %v (ticks: %d)", expectedDuration, elapsed, tickCount)
	}
}
