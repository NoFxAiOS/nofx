package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// MockTPSLOrderInfo mimics the TPSLOrderInfo structure for testing
type MockTPSLOrderInfo struct {
	OrderID    int64
	Symbol     string
	OrderType  string
	Side       string
	StopPrice  float64
	Price      float64
	Quantity   float64
	Status     string
	ReduceOnly bool
	CreateTime int64
	UpdateTime int64
}

// MockPosition mimics a Binance position for testing
type MockPosition struct {
	Symbol          string
	PositionAmt     string
	EntryPrice      string
	MarkPrice       string
	UnRealizedProfit string
	Leverage        string
	LiquidationPrice string
}

// TestTPSLMatch tests the TP/SL matching logic
func TestTPSLMatch() {
	// Test case 1: Long position with TP/SL orders
	fmt.Println("=== Test Case 1: Long Position with TP/SL ===")
	longPosition := MockPosition{
		Symbol:          "ETHUSDT",
		PositionAmt:     "0.1690", // Long position (positive amount)
		EntryPrice:      "3267.0100",
		MarkPrice:       "3272.1400",
		UnRealizedProfit: "0.87",
		Leverage:        "8",
		LiquidationPrice: "2769.5594",
	}

	// Mock TP/SL orders (Algo orders created by SetStopLoss and SetTakeProfit)
	mockOrders := []MockTPSLOrderInfo{
		{
			OrderID:    123456,
			Symbol:     "ETHUSDT",
			OrderType:  "TAKE_PROFIT_MARKET",
			Side:       "SELL",
			StopPrice:  3300.00,
			Price:      0,
			Quantity:   0.1690,
			Status:     "NEW",
			ReduceOnly: true,
			CreateTime: time.Now().UnixMilli(),
			UpdateTime: time.Now().UnixMilli(),
		},
		{
			OrderID:    789012,
			Symbol:     "ETHUSDT",
			OrderType:  "STOP_MARKET",
			Side:       "SELL",
			StopPrice:  3250.00,
			Price:      0,
			Quantity:   0.1690,
			Status:     "NEW",
			ReduceOnly: true,
			CreateTime: time.Now().UnixMilli(),
			UpdateTime: time.Now().UnixMilli(),
		},
	}

	// Test the matching logic
	testPositionMatch(longPosition, mockOrders)

	// Test case 2: Short position with TP/SL orders
	fmt.Println("\n=== Test Case 2: Short Position with TP/SL ===")
	shortPosition := MockPosition{
		Symbol:          "BTCUSDT",
		PositionAmt:     "-0.05", // Short position (negative amount)
		EntryPrice:      "45000.00",
		MarkPrice:       "44800.00",
		UnRealizedProfit: "10.00",
		Leverage:        "10",
		LiquidationPrice: "46000.00",
	}

	shortMockOrders := []MockTPSLOrderInfo{
		{
			OrderID:    345678,
			Symbol:     "BTCUSDT",
			OrderType:  "TAKE_PROFIT_MARKET",
			Side:       "BUY",
			StopPrice:  44500.00,
			Price:      0,
			Quantity:   0.05,
			Status:     "NEW",
			ReduceOnly: true,
			CreateTime: time.Now().UnixMilli(),
			UpdateTime: time.Now().UnixMilli(),
		},
		{
			OrderID:    901234,
			Symbol:     "BTCUSDT",
			OrderType:  "STOP_MARKET",
			Side:       "BUY",
			StopPrice:  45200.00,
			Price:      0,
			Quantity:   0.05,
			Status:     "NEW",
			ReduceOnly: true,
			CreateTime: time.Now().UnixMilli(),
			UpdateTime: time.Now().UnixMilli(),
		},
	}

	testPositionMatch(shortPosition, shortMockOrders)

	// Test case 3: Multiple positions with TP/SL orders
	fmt.Println("\n=== Test Case 3: Multiple Positions ===")
	// This would be tested by running the actual code with multiple positions

	// Test case 4: No TP/SL orders
	fmt.Println("\n=== Test Case 4: No TP/SL Orders ===")
	noTPSLPosition := MockPosition{
		Symbol:          "SOLUSDT",
		PositionAmt:     "1.0",
		EntryPrice:      "100.00",
		MarkPrice:       "102.00",
		UnRealizedProfit: "2.00",
		Leverage:        "5",
		LiquidationPrice: "90.00",
	}

	testPositionMatch(noTPSLPosition, []MockTPSLOrderInfo{})
}

// testPositionMatch tests the matching logic for a single position and orders
func testPositionMatch(pos MockPosition, orders []MockTPSLOrderInfo) {
	// Parse position amount to determine side
	posAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
	var side string
	if posAmt > 0 {
		side = "long"
	} else {
		side = "short"
	}

	// Initialize TP/SL prices
	takeProfitPrice := 0.0
	stopLossPrice := 0.0

	// Simulate the matching logic from GetPositions()
	fmt.Printf("Position: %s %s (Amt: %s, Entry: %s, Current: %s)\n", 
		pos.Symbol, strings.ToUpper(side), pos.PositionAmt, pos.EntryPrice, pos.MarkPrice)

	for _, tpslOrder := range orders {
		// Skip orders for different symbols
		if tpslOrder.Symbol != pos.Symbol {
			continue
		}

		tpslOrderType := tpslOrder.OrderType
		tpslSide := tpslOrder.Side

		// Map order side to position side for matching (same logic as in the code)
		if side == "long" {
			// Long position: TP/SL orders should be SELL side
			if tpslSide == "SELL" {
				if tpslOrderType == "TAKE_PROFIT_MARKET" || tpslOrderType == "TAKE_PROFIT_LIMIT" {
					takeProfitPrice = tpslOrder.StopPrice
					fmt.Printf("✓ Matched TAKE_PROFIT order: %s @ %.2f\n", tpslOrderType, tpslOrder.StopPrice)
				} else if tpslOrderType == "STOP_MARKET" || tpslOrderType == "STOP_LIMIT" {
					stopLossPrice = tpslOrder.StopPrice
					fmt.Printf("✓ Matched STOP_LOSS order: %s @ %.2f\n", tpslOrderType, tpslOrder.StopPrice)
				}
			}
		} else {
			// Short position: TP/SL orders should be BUY side
			if tpslSide == "BUY" {
				if tpslOrderType == "TAKE_PROFIT_MARKET" || tpslOrderType == "TAKE_PROFIT_LIMIT" {
					takeProfitPrice = tpslOrder.StopPrice
					fmt.Printf("✓ Matched TAKE_PROFIT order: %s @ %.2f\n", tpslOrderType, tpslOrder.StopPrice)
				} else if tpslOrderType == "STOP_MARKET" || tpslOrderType == "STOP_LIMIT" {
					stopLossPrice = tpslOrder.StopPrice
					fmt.Printf("✓ Matched STOP_LOSS order: %s @ %.2f\n", tpslOrderType, tpslOrder.StopPrice)
				}
			}
		}
	}

	// Format output like the actual system
	takeProfitStr := fmt.Sprintf("%.4f", takeProfitPrice)
	if takeProfitPrice == 0 {
		takeProfitStr = "Not set"
	}
	stopLossStr := fmt.Sprintf("%.4f", stopLossPrice)
	if stopLossPrice == 0 {
		stopLossStr = "Not set"
	}

	fmt.Printf("Final Result: TP: %s | SL: %s\n", takeProfitStr, stopLossStr)
}

func main() {
	fmt.Println("Testing TP/SL Matching Logic\n")
	TestTPSLMatch()
	fmt.Println("\n=== All Tests Completed ===")
}