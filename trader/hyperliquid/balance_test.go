package hyperliquid

import (
	"os"
	"testing"
	"time"
)

// TestHyperliquidBalanceCalculation tests the balance calculation for Hyperliquid
// including perp, spot, and xyz dex (stocks, forex, metals) accounts
// Run with: TEST_PRIVATE_KEY=xxx TEST_WALLET_ADDR=xxx go test -v -run TestHyperliquidBalanceCalculation ./trader/
func TestHyperliquidBalanceCalculation(t *testing.T) {
	// Get credentials from environment
	privateKeyHex := os.Getenv("TEST_PRIVATE_KEY")
	walletAddr := os.Getenv("TEST_WALLET_ADDR")

	if privateKeyHex == "" || walletAddr == "" {
		t.Skip("TEST_PRIVATE_KEY and TEST_WALLET_ADDR env vars required")
	}

	t.Logf("=== Testing Hyperliquid Balance Calculation ===")
	t.Logf("Wallet: %s", walletAddr)

	// Create trader instance
	trader, err := NewHyperliquidTrader(privateKeyHex, walletAddr, false)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	// Test GetBalance
	t.Log("\n--- Testing GetBalance ---")
	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}

	// Extract values
	totalWalletBalance, _ := balance["totalWalletBalance"].(float64)
	totalEquity, _ := balance["totalEquity"].(float64)
	totalUnrealizedProfit, _ := balance["totalUnrealizedProfit"].(float64)
	availableBalance, _ := balance["availableBalance"].(float64)
	spotBalance, _ := balance["spotBalance"].(float64)
	xyzDexBalance, _ := balance["xyzDexBalance"].(float64)
	xyzDexUnrealizedPnl, _ := balance["xyzDexUnrealizedPnl"].(float64)
	perpAccountValue, _ := balance["perpAccountValue"].(float64)

	t.Logf("\n📊 Balance Results:")
	t.Logf("  Perp Account Value:     %.4f USDC", perpAccountValue)
	t.Logf("  Spot Balance:           %.4f USDC", spotBalance)
	t.Logf("  xyz Dex Balance:        %.4f USDC", xyzDexBalance)
	t.Logf("  xyz Dex Unrealized PnL: %.4f USDC", xyzDexUnrealizedPnl)
	t.Logf("  ---")
	t.Logf("  Total Wallet Balance:   %.4f USDC", totalWalletBalance)
	t.Logf("  Total Unrealized PnL:   %.4f USDC", totalUnrealizedProfit)
	t.Logf("  Total Equity:           %.4f USDC", totalEquity)
	t.Logf("  Available Balance:      %.4f USDC", availableBalance)

	// Verify calculation: totalEquity should equal perpAccountValue + spotBalance + xyzDexBalance
	expectedEquity := perpAccountValue + spotBalance + xyzDexBalance
	t.Logf("\n🔍 Verification:")
	t.Logf("  Expected Equity (Perp + Spot + xyz): %.4f", expectedEquity)
	t.Logf("  Actual Total Equity:                 %.4f", totalEquity)

	if abs(totalEquity-expectedEquity) > 0.01 {
		t.Errorf("❌ Equity mismatch! Expected %.4f, got %.4f", expectedEquity, totalEquity)
	} else {
		t.Logf("✅ Equity calculation correct!")
	}

	// Verify: totalWalletBalance + totalUnrealizedProfit should equal totalEquity
	calculatedEquity := totalWalletBalance + totalUnrealizedProfit
	t.Logf("\n🔍 Secondary Verification:")
	t.Logf("  Wallet + Unrealized = %.4f + %.4f = %.4f", totalWalletBalance, totalUnrealizedProfit, calculatedEquity)
	t.Logf("  Total Equity:        %.4f", totalEquity)

	if abs(calculatedEquity-totalEquity) > 0.01 {
		t.Errorf("❌ Secondary check failed! Wallet+Unrealized=%.4f != Equity=%.4f", calculatedEquity, totalEquity)
	} else {
		t.Logf("✅ Secondary verification passed!")
	}

	// Test GetPositions
	t.Log("\n--- Testing GetPositions ---")
	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("GetPositions failed: %v", err)
	}

	t.Logf("Found %d positions:", len(positions))
	totalPositionValue := 0.0
	totalPositionPnL := 0.0

	for i, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		positionAmt, _ := pos["positionAmt"].(float64)
		entryPrice, _ := pos["entryPrice"].(float64)
		markPrice, _ := pos["markPrice"].(float64)
		unrealizedPnL, _ := pos["unRealizedProfit"].(float64)
		leverage, _ := pos["leverage"].(float64)
		isXyzDex, _ := pos["isXyzDex"].(bool)

		posValue := positionAmt * markPrice
		totalPositionValue += posValue
		totalPositionPnL += unrealizedPnL

		assetType := "Crypto"
		if isXyzDex {
			assetType = "xyz Dex"
		}

		t.Logf("  [%d] %s (%s)", i+1, symbol, assetType)
		t.Logf("      Side: %s, Qty: %.4f, Leverage: %.0fx", side, positionAmt, leverage)
		t.Logf("      Entry: %.4f, Mark: %.4f", entryPrice, markPrice)
		t.Logf("      Value: %.4f, PnL: %.4f", posValue, unrealizedPnL)

		// Verify xyz dex position has valid entry/mark prices
		if isXyzDex {
			if entryPrice == 0 {
				t.Errorf("❌ xyz dex position %s has zero entry price!", symbol)
			}
			if markPrice == 0 {
				t.Errorf("❌ xyz dex position %s has zero mark price!", symbol)
			}
		}
	}

	t.Logf("\n📊 Position Summary:")
	t.Logf("  Total Position Value: %.4f USDC", totalPositionValue)
	t.Logf("  Total Position PnL:   %.4f USDC", totalPositionPnL)

	// Compare position PnL with balance unrealized PnL
	t.Logf("\n🔍 PnL Comparison:")
	t.Logf("  Balance Unrealized PnL: %.4f", totalUnrealizedProfit)
	t.Logf("  Position Sum PnL:       %.4f", totalPositionPnL)

	if abs(totalUnrealizedProfit-totalPositionPnL) > 0.1 {
		t.Logf("⚠️  PnL mismatch (may be due to funding fees or timing)")
	} else {
		t.Logf("✅ PnL values match!")
	}
}

// TestXyzDexBalanceDirectQuery directly queries xyz dex balance for debugging
func TestXyzDexBalanceDirectQuery(t *testing.T) {
	privateKeyHex := os.Getenv("TEST_PRIVATE_KEY")
	walletAddr := os.Getenv("TEST_WALLET_ADDR")

	if privateKeyHex == "" || walletAddr == "" {
		t.Skip("TEST_PRIVATE_KEY and TEST_WALLET_ADDR env vars required")
	}

	trader, err := NewHyperliquidTrader(privateKeyHex, walletAddr, false)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	t.Log("=== Direct xyz Dex Balance Query ===")

	accountValue, unrealizedPnl, positions, err := trader.getXYZDexBalance()
	if err != nil {
		t.Fatalf("getXYZDexBalance failed: %v", err)
	}

	t.Logf("xyz Dex Account Value: %.4f", accountValue)
	t.Logf("xyz Dex Unrealized PnL: %.4f", unrealizedPnl)
	t.Logf("xyz Dex Wallet Balance: %.4f", accountValue-unrealizedPnl)
	t.Logf("xyz Dex Positions: %d", len(positions))

	for i, pos := range positions {
		entryPx := "nil"
		if pos.Position.EntryPx != nil {
			entryPx = *pos.Position.EntryPx
		}
		liqPx := "nil"
		if pos.Position.LiquidationPx != nil {
			liqPx = *pos.Position.LiquidationPx
		}

		t.Logf("  [%d] %s:", i+1, pos.Position.Coin)
		t.Logf("      Size: %s", pos.Position.Szi)
		t.Logf("      Entry Price: %s", entryPx)
		t.Logf("      Position Value: %s", pos.Position.PositionValue)
		t.Logf("      Unrealized PnL: %s", pos.Position.UnrealizedPnl)
		t.Logf("      Liquidation Price: %s", liqPx)
		t.Logf("      Leverage: %d (%s)", pos.Position.Leverage.Value, pos.Position.Leverage.Type)
	}
}

// TestEquityAfterOpeningPosition simulates opening a position and verifies equity
func TestEquityAfterOpeningPosition(t *testing.T) {
	privateKeyHex := os.Getenv("TEST_PRIVATE_KEY")
	walletAddr := os.Getenv("TEST_WALLET_ADDR")

	if privateKeyHex == "" || walletAddr == "" {
		t.Skip("TEST_PRIVATE_KEY and TEST_WALLET_ADDR env vars required")
	}

	if os.Getenv("XYZ_DEX_LIVE_TEST") != "1" {
		t.Skip("Set XYZ_DEX_LIVE_TEST=1 to run live position test")
	}

	trader, err := NewHyperliquidTrader(privateKeyHex, walletAddr, false)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	// Step 1: Record initial balance
	t.Log("=== Step 1: Record Initial Balance ===")
	initialBalance, _ := trader.GetBalance()
	initialEquity, _ := initialBalance["totalEquity"].(float64)
	t.Logf("Initial Equity: %.4f", initialEquity)

	// Step 2: Fetch xyz meta
	if err := trader.fetchXyzMeta(); err != nil {
		t.Fatalf("Failed to fetch xyz meta: %v", err)
	}

	// Step 3: Get current price and place a small order
	price, err := trader.getXyzMarketPrice("xyz:SILVER")
	if err != nil {
		t.Fatalf("Failed to get price: %v", err)
	}
	t.Logf("Current xyz:SILVER price: %.4f", price)

	// Place a small buy order (minimum ~$10)
	testSize := 0.14
	testPrice := price * 1.05 // 5% above for IOC

	t.Log("\n=== Step 2: Place Test Order ===")
	t.Logf("Opening position: xyz:SILVER BUY %.4f @ %.4f", testSize, testPrice)

	err = trader.placeXyzOrder("xyz:SILVER", true, testSize, testPrice, false)
	if err != nil {
		t.Logf("Order result: %v", err)
		// Even if IOC doesn't fill, continue to check balance
	}

	// Wait a moment for the order to process
	time.Sleep(2 * time.Second)

	// Step 3: Check balance after order
	t.Log("\n=== Step 3: Check Balance After Order ===")
	afterBalance, _ := trader.GetBalance()
	afterEquity, _ := afterBalance["totalEquity"].(float64)
	afterPerpAV, _ := afterBalance["perpAccountValue"].(float64)
	afterXyzAV, _ := afterBalance["xyzDexBalance"].(float64)

	t.Logf("After Order:")
	t.Logf("  Perp Account Value: %.4f", afterPerpAV)
	t.Logf("  xyz Dex Balance:    %.4f", afterXyzAV)
	t.Logf("  Total Equity:       %.4f", afterEquity)

	equityChange := afterEquity - initialEquity
	t.Logf("\nEquity Change: %.4f (%.2f%%)", equityChange, (equityChange/initialEquity)*100)

	// Equity should not change significantly (only by trading fees/slippage)
	if abs(equityChange) > initialEquity*0.05 { // More than 5% change is suspicious
		t.Errorf("❌ Equity changed too much! Initial=%.4f, After=%.4f, Change=%.4f",
			initialEquity, afterEquity, equityChange)
	} else {
		t.Logf("✅ Equity change is within acceptable range")
	}

	// Step 4: Close position if opened
	t.Log("\n=== Step 4: Close Position ===")
	positions, _ := trader.GetPositions()
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		if symbol == "xyz:SILVER" {
			posAmt, _ := pos["positionAmt"].(float64)
			if posAmt > 0 {
				closePrice := price * 0.95 // 5% below for IOC sell
				t.Logf("Closing position: SELL %.4f @ %.4f", posAmt, closePrice)
				trader.placeXyzOrder("xyz:SILVER", false, posAmt, closePrice, true)
			}
		}
	}

	time.Sleep(2 * time.Second)

	// Final balance check
	t.Log("\n=== Step 5: Final Balance ===")
	finalBalance, _ := trader.GetBalance()
	finalEquity, _ := finalBalance["totalEquity"].(float64)
	t.Logf("Final Equity: %.4f", finalEquity)
	t.Logf("Net Change: %.4f", finalEquity-initialEquity)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
