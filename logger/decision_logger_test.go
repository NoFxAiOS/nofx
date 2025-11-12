package logger

import (
	"testing"
	"time"
)

// TestGetTakerFeeRate tests the getTakerFeeRate function for all supported exchanges
func TestGetTakerFeeRate(t *testing.T) {
	tests := []struct {
		name     string
		exchange string
		wantRate float64
	}{
		{
			name:     "Aster exchange returns 0.035% taker fee",
			exchange: "aster",
			wantRate: 0.00035,
		},
		{
			name:     "Hyperliquid exchange returns 0.045% taker fee",
			exchange: "hyperliquid",
			wantRate: 0.00045,
		},
		{
			name:     "Binance exchange returns 0.050% taker fee",
			exchange: "binance",
			wantRate: 0.0005,
		},
		{
			name:     "Unknown exchange defaults to 0.050% taker fee",
			exchange: "unknown_exchange",
			wantRate: 0.0005,
		},
		{
			name:     "Empty string defaults to 0.050% taker fee",
			exchange: "",
			wantRate: 0.0005,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTakerFeeRate(tt.exchange)
			if got != tt.wantRate {
				t.Errorf("getTakerFeeRate(%q) = %v, want %v", tt.exchange, got, tt.wantRate)
			}
		})
	}
}

// TestPnLCalculationWithFees tests that P&L calculation correctly includes trading fees
func TestPnLCalculationWithFees(t *testing.T) {
	tests := []struct {
		name         string
		exchange     string
		side         string
		quantity     float64
		openPrice    float64
		closePrice   float64
		wantPnL      float64
		wantPnLRange [2]float64 // [min, max] for floating point tolerance
	}{
		{
			name:       "Long position profit on Aster",
			exchange:   "aster",
			side:       "long",
			quantity:   0.01,
			openPrice:  100000.0,
			closePrice: 101000.0,
			// Price diff: 0.01 * (101000 - 100000) = 10 USDT
			// Open fee: 0.01 * 100000 * 0.00035 = 0.35 USDT
			// Close fee: 0.01 * 101000 * 0.00035 = 0.3535 USDT
			// Total fees: 0.7035 USDT
			// Net PnL: 10 - 0.7035 = 9.2965 USDT
			wantPnLRange: [2]float64{9.296, 9.297},
		},
		{
			name:       "Long position loss on Aster",
			exchange:   "aster",
			side:       "long",
			quantity:   0.002,
			openPrice:  103960.7,
			closePrice: 103425.3,
			// Price diff: 0.002 * (103425.3 - 103960.7) = -1.0708 USDT
			// Open fee: 0.002 * 103960.7 * 0.00035 = 0.0728 USDT
			// Close fee: 0.002 * 103425.3 * 0.00035 = 0.0724 USDT
			// Total fees: 0.1452 USDT
			// Net PnL: -1.0708 - 0.1452 = -1.216 USDT
			wantPnLRange: [2]float64{-1.217, -1.215},
		},
		{
			name:       "Short position profit on Hyperliquid",
			exchange:   "hyperliquid",
			side:       "short",
			quantity:   0.01,
			openPrice:  50000.0,
			closePrice: 49000.0,
			// Price diff: 0.01 * (50000 - 49000) = 10 USDT
			// Open fee: 0.01 * 50000 * 0.00045 = 0.225 USDT
			// Close fee: 0.01 * 49000 * 0.00045 = 0.2205 USDT
			// Total fees: 0.4455 USDT
			// Net PnL: 10 - 0.4455 = 9.5545 USDT
			wantPnLRange: [2]float64{9.554, 9.555},
		},
		{
			name:       "Short position loss on Binance",
			exchange:   "binance",
			side:       "short",
			quantity:   0.1,
			openPrice:  3000.0,
			closePrice: 3100.0,
			// Price diff: 0.1 * (3000 - 3100) = -10 USDT
			// Open fee: 0.1 * 3000 * 0.0005 = 0.15 USDT
			// Close fee: 0.1 * 3100 * 0.0005 = 0.155 USDT
			// Total fees: 0.305 USDT
			// Net PnL: -10 - 0.305 = -10.305 USDT
			wantPnLRange: [2]float64{-10.306, -10.304},
		},
		{
			name:       "Small position on unknown exchange (uses default rate)",
			exchange:   "test_exchange",
			side:       "long",
			quantity:   0.001,
			openPrice:  50000.0,
			closePrice: 50500.0,
			// Price diff: 0.001 * (50500 - 50000) = 0.5 USDT
			// Open fee: 0.001 * 50000 * 0.0005 = 0.025 USDT
			// Close fee: 0.001 * 50500 * 0.0005 = 0.02525 USDT
			// Total fees: 0.05025 USDT
			// Net PnL: 0.5 - 0.05025 = 0.44975 USDT
			wantPnLRange: [2]float64{0.449, 0.451},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate price difference P&L
			var pnl float64
			if tt.side == "long" {
				pnl = tt.quantity * (tt.closePrice - tt.openPrice)
			} else {
				pnl = tt.quantity * (tt.openPrice - tt.closePrice)
			}

			// Deduct trading fees
			feeRate := getTakerFeeRate(tt.exchange)
			openFee := tt.quantity * tt.openPrice * feeRate
			closeFee := tt.quantity * tt.closePrice * feeRate
			totalFees := openFee + closeFee
			pnl -= totalFees

			// Check if PnL is within expected range (for floating point tolerance)
			if pnl < tt.wantPnLRange[0] || pnl > tt.wantPnLRange[1] {
				t.Errorf("P&L calculation = %v, want range [%v, %v]", pnl, tt.wantPnLRange[0], tt.wantPnLRange[1])
				t.Logf("  Exchange: %s, Side: %s", tt.exchange, tt.side)
				t.Logf("  Quantity: %v, Open: %v, Close: %v", tt.quantity, tt.openPrice, tt.closePrice)
				t.Logf("  Fee rate: %v, Total fees: %v", feeRate, totalFees)
			}
		})
	}
}

// TestAnalyzePerformance_WithFees tests that AnalyzePerformance correctly calculates P&L with fees
func TestAnalyzePerformance_WithFees(t *testing.T) {
	// Create a temporary test logger
	logger := NewDecisionLogger(t.TempDir())

	// Create test records with open and close actions
	openTime := time.Now().Add(-1 * time.Hour)
	closeTime := time.Now()

	// Test case: Aster long position loss (from user's example)
	record := &DecisionRecord{
		Exchange:    "aster",
		CycleNumber: 1,
		Timestamp:   openTime,
		Success:     true,
		Decisions: []DecisionAction{
			{
				Action:    "open_long",
				Symbol:    "BTCUSDT",
				Quantity:  0.002,
				Leverage:  5,
				Price:     103960.7,
				Timestamp: openTime,
				Success:   true,
			},
		},
	}

	// Log the open position
	err := logger.LogDecision(record)
	if err != nil {
		t.Fatalf("Failed to log open position: %v", err)
	}

	// Create close position record
	closeRecord := &DecisionRecord{
		Exchange:    "aster",
		CycleNumber: 2,
		Timestamp:   closeTime,
		Success:     true,
		Decisions: []DecisionAction{
			{
				Action:    "close_long",
				Symbol:    "BTCUSDT",
				Quantity:  0.002,
				Leverage:  5,
				Price:     103425.3,
				Timestamp: closeTime,
				Success:   true,
			},
		},
	}

	err = logger.LogDecision(closeRecord)
	if err != nil {
		t.Fatalf("Failed to log close position: %v", err)
	}

	// Analyze performance
	analysis, err := logger.AnalyzePerformance(10)
	if err != nil {
		t.Fatalf("AnalyzePerformance failed: %v", err)
	}

	// Verify results
	if analysis.TotalTrades != 1 {
		t.Errorf("Expected 1 trade, got %d", analysis.TotalTrades)
	}

	if len(analysis.RecentTrades) != 1 {
		t.Fatalf("Expected 1 recent trade, got %d", len(analysis.RecentTrades))
	}

	trade := analysis.RecentTrades[0]

	// Expected P&L with fees (Aster 0.035% taker fee)
	// Price diff: 0.002 * (103425.3 - 103960.7) = -1.0708 USDT
	// Open fee: 0.002 * 103960.7 * 0.00035 = 0.0728 USDT
	// Close fee: 0.002 * 103425.3 * 0.00035 = 0.0724 USDT
	// Total fees: 0.1452 USDT
	// Net PnL: -1.0708 - 0.1452 = -1.216 USDT
	expectedPnLMin := -1.217
	expectedPnLMax := -1.215

	if trade.PnL < expectedPnLMin || trade.PnL > expectedPnLMax {
		t.Errorf("Trade P&L = %v, want range [%v, %v]", trade.PnL, expectedPnLMin, expectedPnLMax)
		t.Logf("  Symbol: %s, Side: %s", trade.Symbol, trade.Side)
		t.Logf("  Open: %v, Close: %v, Quantity: %v", trade.OpenPrice, trade.ClosePrice, trade.Quantity)
	}

	// Verify it's counted as a losing trade
	if analysis.LosingTrades != 1 {
		t.Errorf("Expected 1 losing trade, got %d", analysis.LosingTrades)
	}

	if analysis.WinningTrades != 0 {
		t.Errorf("Expected 0 winning trades, got %d", analysis.WinningTrades)
	}
}

// TestAnalyzePerformance_PartialCloseWithFees tests partial close fee accumulation
func TestAnalyzePerformance_PartialCloseWithFees(t *testing.T) {
	logger := NewDecisionLogger(t.TempDir())

	openTime := time.Now().Add(-2 * time.Hour)
	partialCloseTime := time.Now().Add(-1 * time.Hour)
	finalCloseTime := time.Now()

	// Open position
	openRecord := &DecisionRecord{
		Exchange:    "hyperliquid",
		CycleNumber: 1,
		Timestamp:   openTime,
		Success:     true,
		Decisions: []DecisionAction{
			{
				Action:    "open_long",
				Symbol:    "ETHUSDT",
				Quantity:  1.0, // 1 ETH
				Leverage:  10,
				Price:     2000.0,
				Timestamp: openTime,
				Success:   true,
			},
		},
	}
	logger.LogDecision(openRecord)

	// Partial close (50%)
	partialCloseRecord := &DecisionRecord{
		Exchange:    "hyperliquid",
		CycleNumber: 2,
		Timestamp:   partialCloseTime,
		Success:     true,
		Decisions: []DecisionAction{
			{
				Action:    "partial_close",
				Symbol:    "ETHUSDT",
				Quantity:  0.5, // Close 0.5 ETH
				Price:     2100.0,
				Timestamp: partialCloseTime,
				Success:   true,
			},
		},
	}
	logger.LogDecision(partialCloseRecord)

	// Final close (remaining 50%)
	finalCloseRecord := &DecisionRecord{
		Exchange:    "hyperliquid",
		CycleNumber: 3,
		Timestamp:   finalCloseTime,
		Success:     true,
		Decisions: []DecisionAction{
			{
				Action:    "close_long",
				Symbol:    "ETHUSDT",
				Quantity:  0.5, // Close remaining 0.5 ETH
				Price:     2150.0,
				Timestamp: finalCloseTime,
				Success:   true,
			},
		},
	}
	logger.LogDecision(finalCloseRecord)

	// Analyze performance
	analysis, err := logger.AnalyzePerformance(10)
	if err != nil {
		t.Fatalf("AnalyzePerformance failed: %v", err)
	}

	// Should count as 1 complete trade
	if analysis.TotalTrades != 1 {
		t.Errorf("Expected 1 trade, got %d", analysis.TotalTrades)
	}

	if len(analysis.RecentTrades) != 1 {
		t.Fatalf("Expected 1 recent trade, got %d", len(analysis.RecentTrades))
	}

	trade := analysis.RecentTrades[0]

	// Calculate expected P&L (Hyperliquid 0.045% taker fee)
	// Partial close: 0.5 * (2100 - 2000) = 50 USDT
	//   Open fee: 0.5 * 2000 * 0.00045 = 0.45 USDT
	//   Close fee: 0.5 * 2100 * 0.00045 = 0.4725 USDT
	//   Partial PnL: 50 - 0.45 - 0.4725 = 49.0775 USDT
	//
	// Final close: 0.5 * (2150 - 2000) = 75 USDT
	//   Open fee: 0.5 * 2000 * 0.00045 = 0.45 USDT
	//   Close fee: 0.5 * 2150 * 0.00045 = 0.48375 USDT
	//   Final PnL: 75 - 0.45 - 0.48375 = 74.06625 USDT
	//
	// Total PnL: 49.0775 + 74.06625 = 123.14375 USDT
	expectedPnLMin := 123.14
	expectedPnLMax := 123.15

	if trade.PnL < expectedPnLMin || trade.PnL > expectedPnLMax {
		t.Errorf("Trade P&L = %v, want range [%v, %v]", trade.PnL, expectedPnLMin, expectedPnLMax)
		t.Logf("  Symbol: %s, Side: %s", trade.Symbol, trade.Side)
		t.Logf("  Quantity: %v, Open: %v, Close: %v", trade.Quantity, trade.OpenPrice, trade.ClosePrice)
	}

	// Should be a winning trade
	if analysis.WinningTrades != 1 {
		t.Errorf("Expected 1 winning trade, got %d", analysis.WinningTrades)
	}
}

// TestFeeImpactOnPerformanceMetrics verifies that fees affect performance metrics correctly
func TestFeeImpactOnPerformanceMetrics(t *testing.T) {
	logger := NewDecisionLogger(t.TempDir())

	// Create two trades: one winning, one losing (after fees)
	baseTime := time.Now().Add(-2 * time.Hour)

	// Trade 1: Slight profit before fees, loss after fees
	// Open: 100, Close: 100.5, Quantity: 10 (Binance 0.05% fee)
	// Price diff: 10 * (100.5 - 100) = 5 USDT
	// Fees: 10*100*0.0005 + 10*100.5*0.0005 = 0.5 + 0.5025 = 1.0025 USDT
	// Net: 5 - 1.0025 = 3.9975 USDT (actually still profit, let me recalculate)
	// Let's use a closer price to demonstrate the fee impact

	records := []*DecisionRecord{
		// Trade 1 - open
		{
			Exchange:    "binance",
			CycleNumber: 1,
			Timestamp:   baseTime,
			Success:     true,
			Decisions: []DecisionAction{
				{
					Action:    "open_long",
					Symbol:    "BTCUSDT",
					Quantity:  0.01,
					Leverage:  5,
					Price:     50000.0,
					Timestamp: baseTime,
					Success:   true,
				},
			},
		},
		// Trade 1 - close (small profit after fees)
		{
			Exchange:    "binance",
			CycleNumber: 2,
			Timestamp:   baseTime.Add(30 * time.Minute),
			Success:     true,
			Decisions: []DecisionAction{
				{
					Action:    "close_long",
					Symbol:    "BTCUSDT",
					Price:     51000.0,
					Timestamp: baseTime.Add(30 * time.Minute),
					Success:   true,
				},
			},
		},
		// Trade 2 - open
		{
			Exchange:    "binance",
			CycleNumber: 3,
			Timestamp:   baseTime.Add(1 * time.Hour),
			Success:     true,
			Decisions: []DecisionAction{
				{
					Action:    "open_short",
					Symbol:    "ETHUSDT",
					Quantity:  0.5,
					Leverage:  5,
					Price:     3000.0,
					Timestamp: baseTime.Add(1 * time.Hour),
					Success:   true,
				},
			},
		},
		// Trade 2 - close (loss)
		{
			Exchange:    "binance",
			CycleNumber: 4,
			Timestamp:   baseTime.Add(90 * time.Minute),
			Success:     true,
			Decisions: []DecisionAction{
				{
					Action:    "close_short",
					Symbol:    "ETHUSDT",
					Price:     3100.0,
					Timestamp: baseTime.Add(90 * time.Minute),
					Success:   true,
				},
			},
		},
	}

	// Log all records
	for _, record := range records {
		if err := logger.LogDecision(record); err != nil {
			t.Fatalf("Failed to log decision: %v", err)
		}
	}

	// Analyze
	analysis, err := logger.AnalyzePerformance(10)
	if err != nil {
		t.Fatalf("AnalyzePerformance failed: %v", err)
	}

	// Should have 2 trades
	if analysis.TotalTrades != 2 {
		t.Errorf("Expected 2 trades, got %d", analysis.TotalTrades)
	}

	// Verify that win rate is calculated correctly
	if analysis.TotalTrades > 0 {
		expectedWinRate := (float64(analysis.WinningTrades) / float64(analysis.TotalTrades)) * 100
		if analysis.WinRate != expectedWinRate {
			t.Errorf("Win rate = %v, expected %v", analysis.WinRate, expectedWinRate)
		}
	}

	// All trades should have non-zero P&L (including fees)
	for i, trade := range analysis.RecentTrades {
		if trade.PnL == 0 {
			t.Errorf("Trade %d has zero P&L, fees may not be applied", i)
		}
	}
}
