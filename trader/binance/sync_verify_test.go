package binance

import (
	"context"
	"math"
	"nofx/store"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func repeatStr(s string, n int) string {
	return strings.Repeat(s, n)
}

// TestBinanceSyncVerification verifies synced data matches exchange data exactly
func TestBinanceSyncVerification(t *testing.T) {
	skipIfNoLiveTest(t)

	// Get credentials from environment
	apiKey, secretKey := getBinanceTestCredentials(t)

	// Create test database
	testDBPath := "/tmp/test_binance_verify.db"
	os.Remove(testDBPath)

	st, err := store.New(testDBPath)
	if err != nil {
		t.Fatalf("Failed to init test store: %v", err)
	}
	db := st.GormDB()

	trader := NewFuturesTrader(apiKey, secretKey, "test-user")

	traderID := "test-trader-id"
	exchangeID := "test-exchange-id"
	exchangeType := "binance"

	// Step 1: Run sync
	t.Logf("%s", repeatStr("=", 60))
	t.Logf("STEP 1: Running order sync...")
	t.Logf("%s", repeatStr("=", 60))

	err = trader.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Step 2: Get all trades from exchange for verification
	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("STEP 2: Fetching trades from exchange for verification...")
	t.Logf("%s", repeatStr("=", 60))

	startTime := time.Now().UTC().Add(-7 * 24 * time.Hour)

	// Get symbols from DB
	var symbols []string
	db.Model(&store.TraderFill{}).
		Select("DISTINCT symbol").
		Where("exchange_id = ?", exchangeID).
		Pluck("symbol", &symbols)

	t.Logf("Symbols to verify: %v", symbols)

	// Fetch all trades from exchange
	type ExchangeTrade struct {
		TradeID     string
		Symbol      string
		Side        string
		Price       float64
		Quantity    float64
		Fee         float64
		RealizedPnL float64
		Time        time.Time
	}

	var exchangeTrades []ExchangeTrade
	for _, symbol := range symbols {
		trades, err := trader.GetTradesForSymbol(symbol, startTime, 1000)
		if err != nil {
			t.Logf("⚠️ Failed to get trades for %s: %v", symbol, err)
			continue
		}
		for _, trade := range trades {
			exchangeTrades = append(exchangeTrades, ExchangeTrade{
				TradeID:     trade.TradeID,
				Symbol:      trade.Symbol,
				Side:        trade.Side,
				Price:       trade.Price,
				Quantity:    trade.Quantity,
				Fee:         trade.Fee,
				RealizedPnL: trade.RealizedPnL,
				Time:        trade.Time,
			})
		}
	}

	t.Logf("Total trades from exchange: %d", len(exchangeTrades))

	// Step 3: Get all fills from DB
	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("STEP 3: Comparing with local database...")
	t.Logf("%s", repeatStr("=", 60))

	var dbFills []store.TraderFill
	db.Where("exchange_id = ?", exchangeID).Find(&dbFills)

	t.Logf("Total fills in DB: %d", len(dbFills))

	// Create maps for comparison
	exchangeTradeMap := make(map[string]ExchangeTrade)
	for _, t := range exchangeTrades {
		exchangeTradeMap[t.TradeID] = t
	}

	dbFillMap := make(map[string]store.TraderFill)
	for _, f := range dbFills {
		dbFillMap[f.ExchangeTradeID] = f
	}

	// Step 4: Check for missing trades
	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("STEP 4: Checking for MISSING trades (in exchange but not in DB)...")
	t.Logf("%s", repeatStr("=", 60))

	var missingTrades []ExchangeTrade
	for tradeID, trade := range exchangeTradeMap {
		if _, exists := dbFillMap[tradeID]; !exists {
			missingTrades = append(missingTrades, trade)
		}
	}

	if len(missingTrades) > 0 {
		t.Logf("❌ MISSING %d trades:", len(missingTrades))
		for i, trade := range missingTrades {
			if i >= 10 {
				t.Logf("   ... and %d more", len(missingTrades)-10)
				break
			}
			t.Logf("   - %s %s %s qty=%.6f price=%.4f time=%s",
				trade.TradeID, trade.Symbol, trade.Side,
				trade.Quantity, trade.Price, trade.Time.Format(time.RFC3339))
		}
	} else {
		t.Logf("✅ No missing trades")
	}

	// Step 5: Check for extra/duplicate trades
	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("STEP 5: Checking for EXTRA trades (in DB but not in exchange)...")
	t.Logf("%s", repeatStr("=", 60))

	var extraTrades []store.TraderFill
	for tradeID, fill := range dbFillMap {
		if _, exists := exchangeTradeMap[tradeID]; !exists {
			extraTrades = append(extraTrades, fill)
		}
	}

	if len(extraTrades) > 0 {
		t.Logf("❌ EXTRA %d trades in DB:", len(extraTrades))
		for i, fill := range extraTrades {
			if i >= 10 {
				t.Logf("   ... and %d more", len(extraTrades)-10)
				break
			}
			t.Logf("   - %s %s %s qty=%.6f price=%.4f",
				fill.ExchangeTradeID, fill.Symbol, fill.Side,
				fill.Quantity, fill.Price)
		}
	} else {
		t.Logf("✅ No extra/duplicate trades")
	}

	// Step 6: Check for data accuracy
	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("STEP 6: Verifying data accuracy (price, qty, fee, pnl)...")
	t.Logf("%s", repeatStr("=", 60))

	type DataMismatch struct {
		TradeID string
		Field   string
		DB      float64
		Exchange float64
	}

	var mismatches []DataMismatch
	for tradeID, exchangeTrade := range exchangeTradeMap {
		dbFill, exists := dbFillMap[tradeID]
		if !exists {
			continue
		}

		// Compare price
		if !floatEqual(dbFill.Price, exchangeTrade.Price, 0.0001) {
			mismatches = append(mismatches, DataMismatch{
				TradeID: tradeID, Field: "Price",
				DB: dbFill.Price, Exchange: exchangeTrade.Price,
			})
		}

		// Compare quantity
		if !floatEqual(dbFill.Quantity, exchangeTrade.Quantity, 0.000001) {
			mismatches = append(mismatches, DataMismatch{
				TradeID: tradeID, Field: "Quantity",
				DB: dbFill.Quantity, Exchange: exchangeTrade.Quantity,
			})
		}

		// Compare fee
		if !floatEqual(dbFill.Commission, exchangeTrade.Fee, 0.000001) {
			mismatches = append(mismatches, DataMismatch{
				TradeID: tradeID, Field: "Fee",
				DB: dbFill.Commission, Exchange: exchangeTrade.Fee,
			})
		}

		// Compare realized PnL
		if !floatEqual(dbFill.RealizedPnL, exchangeTrade.RealizedPnL, 0.01) {
			mismatches = append(mismatches, DataMismatch{
				TradeID: tradeID, Field: "RealizedPnL",
				DB: dbFill.RealizedPnL, Exchange: exchangeTrade.RealizedPnL,
			})
		}
	}

	if len(mismatches) > 0 {
		t.Logf("❌ DATA MISMATCHES: %d", len(mismatches))
		for i, m := range mismatches {
			if i >= 20 {
				t.Logf("   ... and %d more", len(mismatches)-20)
				break
			}
			t.Logf("   - %s %s: DB=%.6f, Exchange=%.6f",
				m.TradeID, m.Field, m.DB, m.Exchange)
		}
	} else {
		t.Logf("✅ All data matches exactly")
	}

	// Step 7: Summary by symbol
	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("STEP 7: Summary by symbol...")
	t.Logf("%s", repeatStr("=", 60))

	type SymbolSummary struct {
		Symbol          string
		ExchangeCount   int
		DBCount         int
		TotalQty        float64
		TotalFee        float64
		TotalPnL        float64
		ExchangeTotalQty float64
		ExchangeTotalFee float64
		ExchangeTotalPnL float64
	}

	summaryMap := make(map[string]*SymbolSummary)

	for _, trade := range exchangeTrades {
		if summaryMap[trade.Symbol] == nil {
			summaryMap[trade.Symbol] = &SymbolSummary{Symbol: trade.Symbol}
		}
		s := summaryMap[trade.Symbol]
		s.ExchangeCount++
		s.ExchangeTotalQty += trade.Quantity
		s.ExchangeTotalFee += trade.Fee
		s.ExchangeTotalPnL += trade.RealizedPnL
	}

	for _, fill := range dbFills {
		if summaryMap[fill.Symbol] == nil {
			summaryMap[fill.Symbol] = &SymbolSummary{Symbol: fill.Symbol}
		}
		s := summaryMap[fill.Symbol]
		s.DBCount++
		s.TotalQty += fill.Quantity
		s.TotalFee += fill.Commission
		s.TotalPnL += fill.RealizedPnL
	}

	t.Logf("\n%-15s %10s %10s %15s %15s %15s", "Symbol", "Exchange", "DB", "Fee(Exc/DB)", "PnL(Exc/DB)", "Match")
	t.Logf("%s", repeatStr("-", 80))

	for _, s := range summaryMap {
		countMatch := s.ExchangeCount == s.DBCount
		feeMatch := floatEqual(s.ExchangeTotalFee, s.TotalFee, 0.01)
		pnlMatch := floatEqual(s.ExchangeTotalPnL, s.TotalPnL, 0.01)

		matchStr := "✅"
		if !countMatch || !feeMatch || !pnlMatch {
			matchStr = "❌"
		}

		t.Logf("%-15s %10d %10d %7.2f/%-7.2f %7.2f/%-7.2f %s",
			s.Symbol, s.ExchangeCount, s.DBCount,
			s.ExchangeTotalFee, s.TotalFee,
			s.ExchangeTotalPnL, s.TotalPnL,
			matchStr)
	}

	// Step 8: Position verification
	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("STEP 8: Verifying position calculations...")
	t.Logf("%s", repeatStr("=", 60))

	// Get positions from DB
	var dbPositions []store.TraderPosition
	db.Where("exchange_id = ? AND status = ?", exchangeID, "closed").Find(&dbPositions)

	t.Logf("Closed positions in DB: %d", len(dbPositions))

	// Get current positions from exchange
	exchangePositions, err := trader.GetPositions()
	if err != nil {
		t.Logf("⚠️ Failed to get exchange positions: %v", err)
	} else {
		t.Logf("Active positions on exchange: %d", len(exchangePositions))
		for _, pos := range exchangePositions {
			t.Logf("   - %s %s qty=%.6f entry=%.4f pnl=%.4f",
				pos["symbol"], pos["side"],
				pos["positionAmt"], pos["entryPrice"], pos["unRealizedProfit"])
		}
	}

	// Calculate total PnL from trades
	var totalRealizedPnL float64
	var totalFees float64
	for _, fill := range dbFills {
		totalRealizedPnL += fill.RealizedPnL
		totalFees += fill.Commission
	}

	t.Logf("\n📊 PnL Summary from DB:")
	t.Logf("   Total Realized PnL: %.4f USDT", totalRealizedPnL)
	t.Logf("   Total Fees:         %.4f USDT", totalFees)
	t.Logf("   Net PnL:            %.4f USDT", totalRealizedPnL-totalFees)

	// Calculate from exchange
	var exchangeTotalPnL float64
	var exchangeTotalFees float64
	for _, trade := range exchangeTrades {
		exchangeTotalPnL += trade.RealizedPnL
		exchangeTotalFees += trade.Fee
	}

	t.Logf("\n📊 PnL Summary from Exchange:")
	t.Logf("   Total Realized PnL: %.4f USDT", exchangeTotalPnL)
	t.Logf("   Total Fees:         %.4f USDT", exchangeTotalFees)
	t.Logf("   Net PnL:            %.4f USDT", exchangeTotalPnL-exchangeTotalFees)

	// Compare
	pnlMatch := floatEqual(totalRealizedPnL, exchangeTotalPnL, 0.01)
	feeMatch := floatEqual(totalFees, exchangeTotalFees, 0.01)

	t.Logf("\n%s", repeatStr("=", 60))
	t.Logf("FINAL VERIFICATION RESULT")
	t.Logf("%s", repeatStr("=", 60))

	allPassed := true

	if len(missingTrades) > 0 {
		t.Logf("❌ Missing trades: %d", len(missingTrades))
		allPassed = false
	} else {
		t.Logf("✅ No missing trades")
	}

	if len(extraTrades) > 0 {
		t.Logf("❌ Extra/duplicate trades: %d", len(extraTrades))
		allPassed = false
	} else {
		t.Logf("✅ No extra/duplicate trades")
	}

	if len(mismatches) > 0 {
		t.Logf("❌ Data mismatches: %d", len(mismatches))
		allPassed = false
	} else {
		t.Logf("✅ All data accurate")
	}

	if !pnlMatch {
		t.Logf("❌ PnL mismatch: DB=%.4f, Exchange=%.4f", totalRealizedPnL, exchangeTotalPnL)
		allPassed = false
	} else {
		t.Logf("✅ PnL matches")
	}

	if !feeMatch {
		t.Logf("❌ Fee mismatch: DB=%.4f, Exchange=%.4f", totalFees, exchangeTotalFees)
		allPassed = false
	} else {
		t.Logf("✅ Fees match")
	}

	if allPassed {
		t.Logf("\n🎉 ALL VERIFICATIONS PASSED!")
	} else {
		t.Logf("\n⚠️ SOME VERIFICATIONS FAILED - CHECK ABOVE FOR DETAILS")
	}

	// Cleanup
	os.Remove(testDBPath)
}

// floatEqual compares two floats with tolerance
func floatEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// TestBinanceDetailedTradeComparison shows detailed trade-by-trade comparison
func TestBinanceDetailedTradeComparison(t *testing.T) {
	skipIfNoLiveTest(t)

	// Get credentials from environment
	apiKey, secretKey := getBinanceTestCredentials(t)
	trader := NewFuturesTrader(apiKey, secretKey, "test-user")

	startTime := time.Now().UTC().Add(-24 * time.Hour)

	// Get all income (to find symbols with activity)
	incomes, err := trader.client.NewGetIncomeHistoryService().
		StartTime(startTime.UnixMilli()).
		Limit(100).
		Do(context.Background())
	if err != nil {
		t.Fatalf("Failed to get income: %v", err)
	}

	// Find unique symbols
	symbolMap := make(map[string]bool)
	for _, inc := range incomes {
		if inc.Symbol != "" {
			symbolMap[inc.Symbol] = true
		}
	}

	if len(symbolMap) == 0 {
		t.Log("No trading activity in the last 24 hours")
		return
	}

	t.Logf("=%s", repeatStr("=", 100))
	t.Logf("DETAILED TRADE REPORT (Last 24 hours)")
	t.Logf("=%s", repeatStr("=", 100))

	var grandTotalQty float64
	var grandTotalFee float64
	var grandTotalPnL float64

	for symbol := range symbolMap {
		trades, err := trader.GetTradesForSymbol(symbol, startTime, 500)
		if err != nil {
			t.Logf("⚠️ Failed to get trades for %s: %v", symbol, err)
			continue
		}

		if len(trades) == 0 {
			continue
		}

		// Sort by time
		sort.Slice(trades, func(i, j int) bool {
			return trades[i].Time.Before(trades[j].Time)
		})

		t.Logf("\n%s", repeatStr("-", 100))
		t.Logf("📊 %s - %d trades", symbol, len(trades))
		t.Logf("%s", repeatStr("-", 100))
		t.Logf("%-15s %-6s %12s %12s %12s %12s %20s",
			"TradeID", "Side", "Quantity", "Price", "Fee", "PnL", "Time")

		var totalQty, totalFee, totalPnL float64
		var buyQty, sellQty float64

		for _, trade := range trades {
			t.Logf("%-15s %-6s %12.6f %12.4f %12.6f %12.4f %20s",
				trade.TradeID, trade.Side,
				trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
				trade.Time.Format("2006-01-02 15:04:05"))

			totalQty += trade.Quantity
			totalFee += trade.Fee
			totalPnL += trade.RealizedPnL

			if trade.Side == "BUY" {
				buyQty += trade.Quantity
			} else {
				sellQty += trade.Quantity
			}
		}

		t.Logf("%s", repeatStr("-", 100))
		t.Logf("SUBTOTAL: %d trades, Buy=%.6f, Sell=%.6f, Fee=%.6f, PnL=%.4f",
			len(trades), buyQty, sellQty, totalFee, totalPnL)

		grandTotalQty += totalQty
		grandTotalFee += totalFee
		grandTotalPnL += totalPnL
	}

	t.Logf("\n%s", repeatStr("=", 100))
	t.Logf("GRAND TOTAL")
	t.Logf("=%s", repeatStr("=", 100))
	t.Logf("Total Fee:  %.6f USDT", grandTotalFee)
	t.Logf("Total PnL:  %.4f USDT", grandTotalPnL)
	t.Logf("Net PnL:    %.4f USDT", grandTotalPnL-grandTotalFee)
}
