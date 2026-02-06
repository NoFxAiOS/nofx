package binance

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func skipIfNoLiveTest(t *testing.T) {
	if os.Getenv("BINANCE_LIVE_TEST") != "1" {
		t.Skip("Skipping live test. Set BINANCE_LIVE_TEST=1 to run")
	}
}

func getBinanceTestCredentials(t *testing.T) (string, string) {
	apiKey := os.Getenv("BINANCE_TEST_API_KEY")
	secretKey := os.Getenv("BINANCE_TEST_SECRET_KEY")
	if apiKey == "" || secretKey == "" {
		t.Skip("Skipping test. Set BINANCE_TEST_API_KEY and BINANCE_TEST_SECRET_KEY env vars")
	}
	return apiKey, secretKey
}

func createBinanceTestTrader(t *testing.T) *FuturesTrader {
	apiKey, secretKey := getBinanceTestCredentials(t)
	trader := NewFuturesTrader(apiKey, secretKey, "test-user")
	return trader
}

// TestBinanceConnection tests basic API connectivity
func TestBinanceConnection(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}
	t.Logf("✅ Connection OK - Balance: %v", balance)
}

// TestBinanceGetPositions tests position retrieval
func TestBinanceGetPositions(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("Failed to get positions: %v", err)
	}

	t.Logf("📊 Found %d positions with non-zero amount:", len(positions))
	for i, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		posAmt := pos["positionAmt"].(float64)
		entryPrice := pos["entryPrice"].(float64)
		unrealizedPnl := pos["unRealizedProfit"].(float64)

		t.Logf("  [%d] %s %s: qty=%.6f entry=%.4f pnl=%.4f",
			i+1, symbol, side, posAmt, entryPrice, unrealizedPnl)
	}
}

// TestBinanceGetCommissionSymbols tests COMMISSION income detection
func TestBinanceGetCommissionSymbols(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	// Test different time ranges
	timeRanges := []struct {
		name     string
		duration time.Duration
	}{
		{"1 hour", 1 * time.Hour},
		{"24 hours", 24 * time.Hour},
		{"7 days", 7 * 24 * time.Hour},
		{"30 days", 30 * 24 * time.Hour},
	}

	for _, tr := range timeRanges {
		startTime := time.Now().Add(-tr.duration)
		symbols, err := trader.GetCommissionSymbols(startTime)
		if err != nil {
			t.Logf("❌ %s: Failed to get commission symbols: %v", tr.name, err)
			continue
		}
		t.Logf("📋 %s: COMMISSION symbols = %d - %v", tr.name, len(symbols), symbols)
	}
}

// TestBinanceGetPnLSymbols tests REALIZED_PNL income detection
func TestBinanceGetPnLSymbols(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	timeRanges := []struct {
		name     string
		duration time.Duration
	}{
		{"1 hour", 1 * time.Hour},
		{"24 hours", 24 * time.Hour},
		{"7 days", 7 * 24 * time.Hour},
		{"30 days", 30 * 24 * time.Hour},
	}

	for _, tr := range timeRanges {
		startTime := time.Now().Add(-tr.duration)
		symbols, err := trader.GetPnLSymbols(startTime)
		if err != nil {
			t.Logf("❌ %s: Failed to get PnL symbols: %v", tr.name, err)
			continue
		}
		t.Logf("📋 %s: REALIZED_PNL symbols = %d - %v", tr.name, len(symbols), symbols)
	}
}

// TestBinanceGetAllIncomeTypes tests all income types to understand data availability
func TestBinanceGetAllIncomeTypes(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	// All possible income types from Binance API
	incomeTypes := []string{
		"TRANSFER",
		"WELCOME_BONUS",
		"REALIZED_PNL",
		"FUNDING_FEE",
		"COMMISSION",
		"INSURANCE_CLEAR",
		"REFERRAL_KICKBACK",
		"COMMISSION_REBATE",
		"API_REBATE",
		"CONTEST_REWARD",
		"CROSS_COLLATERAL_TRANSFER",
		"OPTIONS_PREMIUM_FEE",
		"OPTIONS_SETTLE_PROFIT",
		"INTERNAL_TRANSFER",
		"AUTO_EXCHANGE",
		"DELIVERED_SETTELMENT",
		"COIN_SWAP_DEPOSIT",
		"COIN_SWAP_WITHDRAW",
		"POSITION_LIMIT_INCREASE_FEE",
	}

	startTime := time.Now().Add(-7 * 24 * time.Hour)
	t.Logf("🔍 Checking all income types from %s:", startTime.Format(time.RFC3339))

	for _, incomeType := range incomeTypes {
		incomes, err := trader.client.NewGetIncomeHistoryService().
			IncomeType(incomeType).
			StartTime(startTime.UnixMilli()).
			Limit(100).
			Do(context.Background())
		if err != nil {
			t.Logf("  ❌ %s: error - %v", incomeType, err)
			continue
		}

		if len(incomes) > 0 {
			symbolMap := make(map[string]int)
			for _, inc := range incomes {
				if inc.Symbol != "" {
					symbolMap[inc.Symbol]++
				}
			}
			t.Logf("  ✅ %s: %d records, symbols: %v", incomeType, len(incomes), symbolMap)
		} else {
			t.Logf("  ⚪ %s: 0 records", incomeType)
		}
	}
}

// TestBinanceGetTradesForSymbol tests trade retrieval for specific symbols
func TestBinanceGetTradesForSymbol(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	// Common trading pairs
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT"}
	startTime := time.Now().Add(-7 * 24 * time.Hour)

	t.Logf("🔍 Checking trades for common symbols from %s:", startTime.Format(time.RFC3339))

	for _, symbol := range symbols {
		trades, err := trader.GetTradesForSymbol(symbol, startTime, 100)
		if err != nil {
			t.Logf("  ❌ %s: error - %v", symbol, err)
			continue
		}

		if len(trades) > 0 {
			t.Logf("  ✅ %s: %d trades", symbol, len(trades))
			// Print first and last trade
			first := trades[0]
			last := trades[len(trades)-1]
			t.Logf("      First: %s %s %s qty=%.6f price=%.4f pnl=%.4f time=%s",
				first.TradeID, first.Symbol, first.Side,
				first.Quantity, first.Price, first.RealizedPnL,
				first.Time.Format(time.RFC3339))
			if len(trades) > 1 {
				t.Logf("      Last:  %s %s %s qty=%.6f price=%.4f pnl=%.4f time=%s",
					last.TradeID, last.Symbol, last.Side,
					last.Quantity, last.Price, last.RealizedPnL,
					last.Time.Format(time.RFC3339))
			}
		} else {
			t.Logf("  ⚪ %s: 0 trades", symbol)
		}
	}
}

// TestBinanceTimestampFormats tests different timestamp formats
func TestBinanceTimestampFormats(t *testing.T) {
	skipIfNoLiveTest(t)

	now := time.Now()
	nowUTC := time.Now().UTC()

	t.Logf("🕐 Time comparison:")
	t.Logf("  time.Now():        %s (UnixMilli: %d)", now.Format(time.RFC3339), now.UnixMilli())
	t.Logf("  time.Now().UTC():  %s (UnixMilli: %d)", nowUTC.Format(time.RFC3339), nowUTC.UnixMilli())
	t.Logf("  Difference: %v", now.Sub(nowUTC))

	// The key insight: UnixMilli() should be the SAME regardless of timezone
	if now.UnixMilli() != nowUTC.UnixMilli() {
		t.Errorf("❌ UnixMilli() differs between local and UTC! This should never happen.")
	} else {
		t.Logf("  ✅ UnixMilli() is the same (correct behavior)")
	}

	// Test what happens when we parse a time stored in DB
	// Simulate old DB value stored in local time
	oldLocalTime := time.Date(2026, 1, 6, 18, 0, 0, 0, time.Local) // 18:00 local
	oldLocalTimeAsUTC := time.Date(2026, 1, 6, 18, 0, 0, 0, time.UTC) // Same numbers but UTC

	t.Logf("\n🔍 Timezone mismatch scenario:")
	t.Logf("  Old DB time (local):     %s (UnixMilli: %d)", oldLocalTime.Format(time.RFC3339), oldLocalTime.UnixMilli())
	t.Logf("  Same time parsed as UTC: %s (UnixMilli: %d)", oldLocalTimeAsUTC.Format(time.RFC3339), oldLocalTimeAsUTC.UnixMilli())
	t.Logf("  Difference: %v", time.Duration(oldLocalTimeAsUTC.UnixMilli()-oldLocalTime.UnixMilli())*time.Millisecond)

	// If server is in +8 timezone, the difference should be 8 hours
	_, offset := now.Zone()
	t.Logf("  Local timezone offset: %d seconds (%d hours)", offset, offset/3600)
}

// TestBinanceFullSyncSimulation simulates the full sync process
func TestBinanceFullSyncSimulation(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	t.Logf("🔄 Simulating full sync process...")

	// Step 1: Determine lastSyncTime (simulating first run)
	lastSyncTime := time.Now().UTC().Add(-7 * 24 * time.Hour)
	t.Logf("\n📅 Step 1: lastSyncTime = %s", lastSyncTime.Format(time.RFC3339))

	// Step 2: Detect symbols using all methods
	symbolMap := make(map[string]bool)

	// Method 1: COMMISSION
	commissionSymbols, err := trader.GetCommissionSymbols(lastSyncTime)
	if err != nil {
		t.Logf("  ⚠️ COMMISSION failed: %v", err)
	} else {
		t.Logf("  📋 COMMISSION symbols: %d - %v", len(commissionSymbols), commissionSymbols)
		for _, s := range commissionSymbols {
			symbolMap[s] = true
		}
	}

	// Method 2: Positions
	positions, err := trader.GetPositions()
	if err != nil {
		t.Logf("  ⚠️ GetPositions failed: %v", err)
	} else {
		var posSymbols []string
		for _, pos := range positions {
			if symbol, ok := pos["symbol"].(string); ok && symbol != "" {
				posSymbols = append(posSymbols, symbol)
				symbolMap[symbol] = true
			}
		}
		t.Logf("  📋 Position symbols: %d - %v", len(posSymbols), posSymbols)
	}

	// Method 3: REALIZED_PNL (fallback)
	pnlSymbols, err := trader.GetPnLSymbols(lastSyncTime)
	if err != nil {
		t.Logf("  ⚠️ REALIZED_PNL failed: %v", err)
	} else {
		t.Logf("  📋 REALIZED_PNL symbols: %d - %v", len(pnlSymbols), pnlSymbols)
		for _, s := range pnlSymbols {
			symbolMap[s] = true
		}
	}

	// Collect all symbols
	var allSymbols []string
	for s := range symbolMap {
		allSymbols = append(allSymbols, s)
	}
	t.Logf("\n📊 Step 2: Total unique symbols to sync: %d - %v", len(allSymbols), allSymbols)

	if len(allSymbols) == 0 {
		t.Logf("❌ No symbols found! This is the bug - nothing to sync")
		t.Logf("\n🔍 Investigating why no symbols found...")

		// Try to query all income (without type filter) to see if there's ANY activity
		incomes, err := trader.client.NewGetIncomeHistoryService().
			StartTime(lastSyncTime.UnixMilli()).
			Limit(100).
			Do(context.Background())
		if err != nil {
			t.Logf("  Failed to get all income: %v", err)
		} else {
			t.Logf("  All income records (no type filter): %d", len(incomes))
			typeCount := make(map[string]int)
			for _, inc := range incomes {
				typeCount[inc.IncomeType]++
			}
			t.Logf("  Income types breakdown: %v", typeCount)
		}
		return
	}

	// Step 3: Query trades for each symbol
	t.Logf("\n📥 Step 3: Querying trades for each symbol...")
	totalTrades := 0
	for _, symbol := range allSymbols {
		trades, err := trader.GetTradesForSymbol(symbol, lastSyncTime, 500)
		if err != nil {
			t.Logf("  ❌ %s: error - %v", symbol, err)
			continue
		}
		totalTrades += len(trades)
		t.Logf("  ✅ %s: %d trades", symbol, len(trades))

		// Print sample trades
		for i, trade := range trades {
			if i >= 3 {
				t.Logf("      ... and %d more trades", len(trades)-3)
				break
			}
			t.Logf("      [%d] %s %s %s qty=%.6f price=%.4f pnl=%.4f fee=%.6f time=%s",
				i+1, trade.TradeID, trade.Symbol, trade.Side,
				trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee,
				trade.Time.Format(time.RFC3339))
		}
	}

	t.Logf("\n✅ Sync simulation complete: %d total trades found across %d symbols",
		totalTrades, len(allSymbols))
}

// TestBinanceTradeIDRange tests trade ID ranges to understand the data
func TestBinanceTradeIDRange(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	// First find symbols with trades
	startTime := time.Now().Add(-30 * 24 * time.Hour)
	commissionSymbols, _ := trader.GetCommissionSymbols(startTime)
	pnlSymbols, _ := trader.GetPnLSymbols(startTime)

	symbolMap := make(map[string]bool)
	for _, s := range commissionSymbols {
		symbolMap[s] = true
	}
	for _, s := range pnlSymbols {
		symbolMap[s] = true
	}

	if len(symbolMap) == 0 {
		t.Log("No symbols with activity found")
		return
	}

	t.Logf("🔍 Checking trade ID ranges for symbols with activity:")

	for symbol := range symbolMap {
		trades, err := trader.GetTradesForSymbol(symbol, startTime, 100)
		if err != nil || len(trades) == 0 {
			continue
		}

		var minID, maxID int64 = 1<<62, 0
		for _, trade := range trades {
			var id int64
			fmt.Sscanf(trade.TradeID, "%d", &id)
			if id < minID {
				minID = id
			}
			if id > maxID {
				maxID = id
			}
		}

		t.Logf("  %s: %d trades, ID range [%d - %d]", symbol, len(trades), minID, maxID)

		// Check if any ID exceeds PostgreSQL INTEGER max
		if maxID > 2147483647 {
			t.Logf("    ⚠️ Max trade ID %d exceeds PostgreSQL INTEGER max (2147483647)", maxID)
		}
	}
}

// TestBinanceIncomeAPIDirectCall makes direct API call to understand response
func TestBinanceIncomeAPIDirectCall(t *testing.T) {
	skipIfNoLiveTest(t)
	trader := createBinanceTestTrader(t)

	startTime := time.Now().Add(-24 * time.Hour)
	t.Logf("🔍 Direct income API call from %s:", startTime.Format(time.RFC3339))
	t.Logf("   StartTime UnixMilli: %d", startTime.UnixMilli())

	// Call without income type filter to get ALL income
	incomes, err := trader.client.NewGetIncomeHistoryService().
		StartTime(startTime.UnixMilli()).
		Limit(1000).
		Do(context.Background())
	if err != nil {
		t.Fatalf("Failed to get income: %v", err)
	}

	t.Logf("📋 Total income records: %d", len(incomes))

	// Group by type and symbol
	typeSymbolCount := make(map[string]map[string]int)
	for _, inc := range incomes {
		if typeSymbolCount[inc.IncomeType] == nil {
			typeSymbolCount[inc.IncomeType] = make(map[string]int)
		}
		typeSymbolCount[inc.IncomeType][inc.Symbol]++
	}

	for incType, symbols := range typeSymbolCount {
		t.Logf("  %s:", incType)
		for symbol, count := range symbols {
			if symbol == "" {
				symbol = "(no symbol)"
			}
			t.Logf("    %s: %d records", symbol, count)
		}
	}

	// Print sample records
	if len(incomes) > 0 {
		t.Logf("\n📝 Sample income records (first 5):")
		for i, inc := range incomes {
			if i >= 5 {
				break
			}
			t.Logf("  [%d] Type=%s Symbol=%s Amount=%s Time=%s",
				i+1, inc.IncomeType, inc.Symbol, inc.Income,
				time.UnixMilli(inc.Time).Format(time.RFC3339))
		}
	}
}
