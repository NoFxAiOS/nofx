package kucoin

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// Test credentials - set via environment variables
func getKuCoinTestCredentials(t *testing.T) (string, string, string) {
	apiKey := os.Getenv("KUCOIN_TEST_API_KEY")
	secretKey := os.Getenv("KUCOIN_TEST_SECRET_KEY")
	passphrase := os.Getenv("KUCOIN_TEST_PASSPHRASE")

	if apiKey == "" || secretKey == "" || passphrase == "" {
		t.Skip("KuCoin test credentials not set (KUCOIN_TEST_API_KEY, KUCOIN_TEST_SECRET_KEY, KUCOIN_TEST_PASSPHRASE)")
	}

	return apiKey, secretKey, passphrase
}

func createKuCoinTestTrader(t *testing.T) *KuCoinTrader {
	apiKey, secretKey, passphrase := getKuCoinTestCredentials(t)
	trader := NewKuCoinTrader(apiKey, secretKey, passphrase)
	return trader
}

// TestKuCoinConnection tests basic API connectivity
func TestKuCoinConnection(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}

	t.Logf("✅ Connection OK")
	t.Logf("  totalWalletBalance: %v", balance["totalWalletBalance"])
	t.Logf("  availableBalance: %v", balance["availableBalance"])
	t.Logf("  totalUnrealizedProfit: %v", balance["totalUnrealizedProfit"])
	t.Logf("  totalEquity: %v", balance["totalEquity"])
}

// TestKuCoinGetPositions tests position retrieval
func TestKuCoinGetPositions(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("Failed to get positions: %v", err)
	}

	t.Logf("📊 Found %d positions:", len(positions))
	for i, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		posAmt := pos["positionAmt"].(float64)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		leverage := pos["leverage"].(float64)
		mgnMode := pos["mgnMode"].(string)

		t.Logf("  [%d] %s %s: qty=%.6f entry=%.4f mark=%.4f pnl=%.4f lev=%.0f mode=%s",
			i+1, symbol, side, posAmt, entryPrice, markPrice, unrealizedPnl, leverage, mgnMode)
	}
}

// TestKuCoinGetTrades tests trade history retrieval with proper JSON parsing
func TestKuCoinGetTrades(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	// Get trades from last 24 hours (KuCoin API quirk: >24h startAt returns 0)
	startTime := time.Now().Add(-24 * time.Hour)

	trades, err := trader.GetTrades(startTime, 100)
	if err != nil {
		t.Fatalf("Failed to get trades: %v", err)
	}

	t.Logf("📋 Retrieved %d trades from KuCoin:", len(trades))
	for i, trade := range trades {
		t.Logf("  [%d] %s | TradeID: %s | OrderID: %s", i+1, trade.ExecTime.Format("2006-01-02 15:04:05"), trade.TradeID, trade.OrderID)
		t.Logf("       Symbol: %s | Side: %s | Action: %s", trade.Symbol, trade.Side, trade.OrderAction)
		t.Logf("       Price: %.4f | Qty: %.6f | Fee: %.6f %s", trade.FillPrice, trade.FillQty, trade.Fee, trade.FeeAsset)
		t.Logf("       PnL: %.4f", trade.ProfitLoss)
	}

	// Verify trade data integrity
	for i, trade := range trades {
		if trade.TradeID == "" {
			t.Errorf("Trade %d has empty TradeID", i)
		}
		if trade.Symbol == "" {
			t.Errorf("Trade %d has empty Symbol", i)
		}
		if trade.Side != "BUY" && trade.Side != "SELL" {
			t.Errorf("Trade %d has invalid Side: %s (expected BUY or SELL)", i, trade.Side)
		}
		if trade.OrderAction != "open_long" && trade.OrderAction != "open_short" &&
			trade.OrderAction != "close_long" && trade.OrderAction != "close_short" {
			t.Errorf("Trade %d has invalid OrderAction: %s", i, trade.OrderAction)
		}
		if trade.FillPrice <= 0 {
			t.Errorf("Trade %d has invalid FillPrice: %.6f", i, trade.FillPrice)
		}
		if trade.FillQty <= 0 {
			t.Errorf("Trade %d has invalid FillQty: %.6f", i, trade.FillQty)
		}
	}
}

// TestKuCoinGetRecentTrades tests recent trades endpoint
func TestKuCoinGetRecentTrades(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	trades, err := trader.GetRecentTrades()
	if err != nil {
		t.Fatalf("Failed to get recent trades: %v", err)
	}

	t.Logf("📋 Retrieved %d recent trades from KuCoin:", len(trades))
	for i, trade := range trades {
		t.Logf("  [%d] %s %s %s qty=%.6f price=%.4f pnl=%.4f action=%s",
			i+1, trade.ExecTime.Format("01-02 15:04:05"), trade.Symbol, trade.Side,
			trade.FillQty, trade.FillPrice, trade.ProfitLoss, trade.OrderAction)
	}
}

// TestKuCoinTradeToRecord tests conversion to TradeRecord
func TestKuCoinTradeToRecord(t *testing.T) {
	// Test open_long
	trade1 := KuCoinTrade{
		TradeID:     "test-trade-1",
		Symbol:      "BTCUSDT",
		Side:        "BUY",
		OrderAction: "open_long",
		FillPrice:   50000.0,
		FillQty:     0.01,
		Fee:         0.5,
		ProfitLoss:  0,
	}
	record1 := trade1.ToTradeRecord()
	if record1.PositionSide != "LONG" {
		t.Errorf("open_long should have PositionSide=LONG, got %s", record1.PositionSide)
	}

	// Test close_long
	trade2 := KuCoinTrade{
		TradeID:     "test-trade-2",
		Symbol:      "BTCUSDT",
		Side:        "SELL",
		OrderAction: "close_long",
		FillPrice:   51000.0,
		FillQty:     0.01,
		Fee:         0.5,
		ProfitLoss:  10.0,
	}
	record2 := trade2.ToTradeRecord()
	if record2.PositionSide != "LONG" {
		t.Errorf("close_long should have PositionSide=LONG, got %s", record2.PositionSide)
	}

	// Test open_short
	trade3 := KuCoinTrade{
		TradeID:     "test-trade-3",
		Symbol:      "ETHUSDT",
		Side:        "SELL",
		OrderAction: "open_short",
		FillPrice:   3000.0,
		FillQty:     0.1,
		Fee:         0.3,
		ProfitLoss:  0,
	}
	record3 := trade3.ToTradeRecord()
	if record3.PositionSide != "SHORT" {
		t.Errorf("open_short should have PositionSide=SHORT, got %s", record3.PositionSide)
	}

	// Test close_short
	trade4 := KuCoinTrade{
		TradeID:     "test-trade-4",
		Symbol:      "ETHUSDT",
		Side:        "BUY",
		OrderAction: "close_short",
		FillPrice:   2900.0,
		FillQty:     0.1,
		Fee:         0.3,
		ProfitLoss:  10.0,
	}
	record4 := trade4.ToTradeRecord()
	if record4.PositionSide != "SHORT" {
		t.Errorf("close_short should have PositionSide=SHORT, got %s", record4.PositionSide)
	}

	t.Logf("✅ TradeRecord conversion tests passed")
}

// TestKuCoinOrderActionDetermination tests that order action is correctly determined
func TestKuCoinOrderActionDetermination(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	startTime := time.Now().Add(-24 * time.Hour)
	trades, err := trader.GetTrades(startTime, 100)
	if err != nil {
		t.Fatalf("Failed to get trades: %v", err)
	}

	// Analyze trade patterns
	actionCounts := make(map[string]int)
	for _, trade := range trades {
		actionCounts[trade.OrderAction]++
	}

	t.Logf("📊 Order action distribution:")
	for action, count := range actionCounts {
		t.Logf("  %s: %d", action, count)
	}

	// Verify logical consistency:
	// - BUY + open_long: opening a long position
	// - BUY + close_short: closing a short position
	// - SELL + open_short: opening a short position
	// - SELL + close_long: closing a long position
	for i, trade := range trades {
		switch trade.OrderAction {
		case "open_long":
			if trade.Side != "BUY" {
				t.Errorf("Trade %d: open_long should have Side=BUY, got %s", i, trade.Side)
			}
		case "close_short":
			if trade.Side != "BUY" {
				t.Errorf("Trade %d: close_short should have Side=BUY, got %s", i, trade.Side)
			}
		case "open_short":
			if trade.Side != "SELL" {
				t.Errorf("Trade %d: open_short should have Side=SELL, got %s", i, trade.Side)
			}
		case "close_long":
			if trade.Side != "SELL" {
				t.Errorf("Trade %d: close_long should have Side=SELL, got %s", i, trade.Side)
			}
		}
	}
}

// TestKuCoinPositionBuilding tests that trades can be used to build position state
func TestKuCoinPositionBuilding(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	startTime := time.Now().Add(-24 * time.Hour)
	trades, err := trader.GetTrades(startTime, 100)
	if err != nil {
		t.Fatalf("Failed to get trades: %v", err)
	}

	// Group trades by symbol and build position state
	type PositionState struct {
		LongQty    float64
		ShortQty   float64
		LongPnL    float64
		ShortPnL   float64
		TradeCount int
	}
	positions := make(map[string]*PositionState)

	for _, trade := range trades {
		if positions[trade.Symbol] == nil {
			positions[trade.Symbol] = &PositionState{}
		}
		pos := positions[trade.Symbol]
		pos.TradeCount++

		switch trade.OrderAction {
		case "open_long":
			pos.LongQty += trade.FillQty
		case "close_long":
			pos.LongQty -= trade.FillQty
			pos.LongPnL += trade.ProfitLoss
		case "open_short":
			pos.ShortQty += trade.FillQty
		case "close_short":
			pos.ShortQty -= trade.FillQty
			pos.ShortPnL += trade.ProfitLoss
		}
	}

	t.Logf("📊 Calculated position states from %d trades:", len(trades))
	for symbol, pos := range positions {
		t.Logf("  %s: trades=%d longQty=%.6f shortQty=%.6f longPnL=%.4f shortPnL=%.4f",
			symbol, pos.TradeCount, pos.LongQty, pos.ShortQty, pos.LongPnL, pos.ShortPnL)
	}

	// Now compare with actual positions from exchange
	actualPositions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("Failed to get actual positions: %v", err)
	}

	t.Logf("\n📊 Actual positions from exchange:")
	for _, pos := range actualPositions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		qty := pos["positionAmt"].(float64)
		t.Logf("  %s %s: qty=%.6f", symbol, side, qty)
	}
}

// TestKuCoinRawAPIResponse tests raw API response to verify field types
func TestKuCoinRawAPIResponse(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	// Make raw request to fills endpoint
	startTime := time.Now().Add(-24 * time.Hour)
	path := fmt.Sprintf("%s?pageSize=10&startAt=%d", kucoinFillsPath, startTime.UnixMilli())

	data, err := trader.doRequest("GET", path, nil)
	if err != nil {
		t.Fatalf("Failed to get raw fills data: %v", err)
	}

	t.Logf("📋 Raw API response (first 2000 chars):")
	response := string(data)
	if len(response) > 2000 {
		response = response[:2000] + "..."
	}
	t.Logf("%s", response)
}

// TestKuCoinValueCalculation tests that calculated value (price * qty) matches API value
// This is the key test to verify multiplier and qty calculation is correct
func TestKuCoinValueCalculation(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	// Get raw API response to compare
	path := fmt.Sprintf("%s?pageSize=20", kucoinFillsPath)
	data, err := trader.doRequest("GET", path, nil)
	if err != nil {
		t.Fatalf("Failed to get raw fills: %v", err)
	}

	var rawResponse struct {
		Items []struct {
			Symbol    string `json:"symbol"`
			TradeId   string `json:"tradeId"`
			Price     string `json:"price"`
			Size      int64  `json:"size"`
			Value     string `json:"value"` // This is the actual USDT value from API
			Side      string `json:"side"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &rawResponse); err != nil {
		t.Fatalf("Failed to parse raw response: %v", err)
	}

	// Get trades via GetTrades
	trades, err := trader.GetTrades(time.Time{}, 20)
	if err != nil {
		t.Fatalf("Failed to get trades: %v", err)
	}

	// Build a map of tradeID -> calculated value
	calculatedValues := make(map[string]float64)
	for _, trade := range trades {
		calculatedValues[trade.TradeID] = trade.FillPrice * trade.FillQty
	}

	t.Logf("Comparing API value vs calculated value (price * qty):")
	t.Logf("==========================================")

	errorCount := 0
	for i, raw := range rawResponse.Items {
		if i >= 10 {
			break
		}

		var apiValue float64
		fmt.Sscanf(raw.Value, "%f", &apiValue)

		calculatedValue, exists := calculatedValues[raw.TradeId]
		if !exists {
			t.Errorf("Trade %s not found in GetTrades result", raw.TradeId)
			continue
		}

		// Allow 1% tolerance for rounding
		tolerance := apiValue * 0.01
		diff := calculatedValue - apiValue
		if diff < 0 {
			diff = -diff
		}

		status := "✅"
		if diff > tolerance {
			status = "❌"
			errorCount++
		}

		t.Logf("  %s [%d] %s: API value=%.4f, Calculated=%.4f, Diff=%.4f",
			status, i+1, raw.Symbol, apiValue, calculatedValue, diff)
	}

	if errorCount > 0 {
		t.Errorf("Found %d trades with incorrect value calculation", errorCount)
	}
}

// TestKuCoinEntryExitPrice tests that entry/exit prices are correctly captured
func TestKuCoinEntryExitPrice(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	trades, err := trader.GetTrades(time.Time{}, 50)
	if err != nil {
		t.Fatalf("Failed to get trades: %v", err)
	}

	// Group trades by symbol to track entry/exit
	type PositionTracker struct {
		OpenTrades  []KuCoinTrade
		CloseTrades []KuCoinTrade
	}
	positions := make(map[string]*PositionTracker)

	for _, trade := range trades {
		if positions[trade.Symbol] == nil {
			positions[trade.Symbol] = &PositionTracker{}
		}
		if trade.OrderAction == "open_long" || trade.OrderAction == "open_short" {
			positions[trade.Symbol].OpenTrades = append(positions[trade.Symbol].OpenTrades, trade)
		} else {
			positions[trade.Symbol].CloseTrades = append(positions[trade.Symbol].CloseTrades, trade)
		}
	}

	t.Logf("Entry/Exit price analysis:")
	t.Logf("==========================")

	for symbol, pos := range positions {
		if len(pos.OpenTrades) == 0 && len(pos.CloseTrades) == 0 {
			continue
		}

		// Calculate weighted average entry price
		var totalEntryValue, totalEntryQty float64
		for _, trade := range pos.OpenTrades {
			totalEntryValue += trade.FillPrice * trade.FillQty
			totalEntryQty += trade.FillQty
		}
		avgEntryPrice := 0.0
		if totalEntryQty > 0 {
			avgEntryPrice = totalEntryValue / totalEntryQty
		}

		// Calculate weighted average exit price
		var totalExitValue, totalExitQty float64
		for _, trade := range pos.CloseTrades {
			totalExitValue += trade.FillPrice * trade.FillQty
			totalExitQty += trade.FillQty
		}
		avgExitPrice := 0.0
		if totalExitQty > 0 {
			avgExitPrice = totalExitValue / totalExitQty
		}

		// Calculate P&L (simplified: (exit - entry) * qty for long)
		pnl := 0.0
		if totalEntryQty > 0 && totalExitQty > 0 {
			// Use the smaller qty for P&L calculation
			closedQty := totalExitQty
			if totalEntryQty < closedQty {
				closedQty = totalEntryQty
			}
			pnl = (avgExitPrice - avgEntryPrice) * closedQty
		}

		t.Logf("  %s:", symbol)
		t.Logf("    Entry: %d trades, total qty=%.6f, avg price=%.6f, value=%.2f USDT",
			len(pos.OpenTrades), totalEntryQty, avgEntryPrice, totalEntryValue)
		t.Logf("    Exit:  %d trades, total qty=%.6f, avg price=%.6f, value=%.2f USDT",
			len(pos.CloseTrades), totalExitQty, avgExitPrice, totalExitValue)
		t.Logf("    Calculated P&L: %.4f USDT", pnl)

		// Verify entry qty matches exit qty for closed positions
		if len(pos.OpenTrades) > 0 && len(pos.CloseTrades) > 0 {
			qtyDiff := totalEntryQty - totalExitQty
			if qtyDiff < 0 {
				qtyDiff = -qtyDiff
			}
			tolerance := totalEntryQty * 0.001 // 0.1% tolerance
			if qtyDiff > tolerance {
				t.Logf("    ⚠️ Entry/Exit qty mismatch: %.6f", qtyDiff)
			}
		}
	}
}

// TestKuCoinPnLCalculation tests P&L calculation against actual exchange data
func TestKuCoinPnLCalculation(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	// Get current balance for reference
	balance, err := trader.GetBalance()
	if err != nil {
		t.Logf("Warning: Could not get balance: %v", err)
	} else {
		t.Logf("Current account balance:")
		t.Logf("  Total equity: %v", balance["totalEquity"])
		t.Logf("  Available: %v", balance["availableBalance"])
	}

	trades, err := trader.GetTrades(time.Time{}, 50)
	if err != nil {
		t.Fatalf("Failed to get trades: %v", err)
	}

	// Group by symbol and calculate P&L
	type SymbolPnL struct {
		Symbol       string
		TotalFees    float64
		GrossPnL     float64 // From price difference
		NetPnL       float64 // Gross - fees
		OpenQty      float64
		CloseQty     float64
		AvgOpenPrice float64
		AvgClosePrice float64
	}
	pnlBySymbol := make(map[string]*SymbolPnL)

	for _, trade := range trades {
		if pnlBySymbol[trade.Symbol] == nil {
			pnlBySymbol[trade.Symbol] = &SymbolPnL{Symbol: trade.Symbol}
		}
		p := pnlBySymbol[trade.Symbol]
		p.TotalFees += trade.Fee

		if trade.OrderAction == "open_long" || trade.OrderAction == "open_short" {
			p.OpenQty += trade.FillQty
			p.AvgOpenPrice = (p.AvgOpenPrice*(p.OpenQty-trade.FillQty) + trade.FillPrice*trade.FillQty) / p.OpenQty
		} else {
			p.CloseQty += trade.FillQty
			p.AvgClosePrice = (p.AvgClosePrice*(p.CloseQty-trade.FillQty) + trade.FillPrice*trade.FillQty) / p.CloseQty
		}
	}

	t.Logf("\nP&L Summary by Symbol:")
	t.Logf("======================")

	var totalGrossPnL, totalFees, totalNetPnL float64

	for symbol, p := range pnlBySymbol {
		closedQty := p.CloseQty
		if p.OpenQty < closedQty {
			closedQty = p.OpenQty
		}

		// For LONG: P&L = (exitPrice - entryPrice) * qty
		if closedQty > 0 && p.AvgOpenPrice > 0 && p.AvgClosePrice > 0 {
			p.GrossPnL = (p.AvgClosePrice - p.AvgOpenPrice) * closedQty
			p.NetPnL = p.GrossPnL - p.TotalFees
		}

		totalGrossPnL += p.GrossPnL
		totalFees += p.TotalFees
		totalNetPnL += p.NetPnL

		t.Logf("  %s:", symbol)
		t.Logf("    Open:  qty=%.6f @ avg price=%.6f", p.OpenQty, p.AvgOpenPrice)
		t.Logf("    Close: qty=%.6f @ avg price=%.6f", p.CloseQty, p.AvgClosePrice)
		t.Logf("    Fees: %.4f USDT", p.TotalFees)
		t.Logf("    Gross P&L: %.4f USDT", p.GrossPnL)
		t.Logf("    Net P&L: %.4f USDT", p.NetPnL)
	}

	t.Logf("\nTotal Summary:")
	t.Logf("  Total Gross P&L: %.4f USDT", totalGrossPnL)
	t.Logf("  Total Fees: %.4f USDT", totalFees)
	t.Logf("  Total Net P&L: %.4f USDT", totalNetPnL)
}

// TestKuCoinGetTradesDebug tests GetTrades with detailed debugging
func TestKuCoinGetTradesDebug(t *testing.T) {
	trader := createKuCoinTestTrader(t)

	// Test with different time windows
	timeWindows := []struct {
		name     string
		duration time.Duration
	}{
		{"1 hour", 1 * time.Hour},
		{"24 hours", 24 * time.Hour},
		{"7 days", 7 * 24 * time.Hour},
		{"no filter", 0},
	}

	for _, tw := range timeWindows {
		var startTime time.Time
		var path string
		if tw.duration > 0 {
			startTime = time.Now().Add(-tw.duration)
			path = fmt.Sprintf("%s?pageSize=100&startAt=%d", kucoinFillsPath, startTime.UnixMilli())
		} else {
			path = fmt.Sprintf("%s?pageSize=100", kucoinFillsPath)
		}

		data, err := trader.doRequest("GET", path, nil)
		if err != nil {
			t.Errorf("Failed to get fills for %s: %v", tw.name, err)
			continue
		}

		// Parse to count items
		var resp struct {
			TotalNum int `json:"totalNum"`
			Items    []struct {
				TradeTime int64 `json:"tradeTime"`
			} `json:"items"`
		}
		json.Unmarshal(data, &resp)

		t.Logf("📋 %s: totalNum=%d, items=%d", tw.name, resp.TotalNum, len(resp.Items))
		if len(resp.Items) > 0 {
			firstTime := time.Unix(0, resp.Items[0].TradeTime)
			t.Logf("   First trade time: %s", firstTime.Format(time.RFC3339))
		}
	}
}
