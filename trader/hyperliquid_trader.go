package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
)

// HyperliquidTrader Hyperliquid trader
type HyperliquidTrader struct {
	exchange      *hyperliquid.Exchange
	ctx           context.Context
	walletAddr    string
	meta          *hyperliquid.Meta // Cache meta information (including precision)
	metaMutex     sync.RWMutex      // Protect concurrent access to meta field
	isCrossMargin bool              // Whether to use cross margin mode
}

// NewHyperliquidTrader creates a Hyperliquid trader
func NewHyperliquidTrader(privateKeyHex string, walletAddr string, testnet bool) (*HyperliquidTrader, error) {
	// Remove 0x prefix from private key (if present, case-insensitive)
	privateKeyHex = strings.TrimPrefix(strings.ToLower(privateKeyHex), "0x")

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Select API URL
	apiURL := hyperliquid.MainnetAPIURL
	if testnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// Security enhancement: Implement Agent Wallet best practices
	// Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets
	agentAddr := crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey)).Hex()

	if walletAddr == "" {
		return nil, fmt.Errorf("❌ Configuration error: Main wallet address (hyperliquid_wallet_addr) not provided\n" +
			"🔐 Correct configuration pattern:\n" +
			"  1. hyperliquid_private_key = Agent Private Key (for signing only, balance should be ~0)\n" +
			"  2. hyperliquid_wallet_addr = Main Wallet Address (holds funds, never expose private key)\n" +
			"💡 Please create an Agent Wallet on Hyperliquid official website and authorize it before configuration:\n" +
			"   https://app.hyperliquid.xyz/ → Settings → API Wallets")
	}

	// Check if user accidentally uses main wallet private key (security risk)
	if strings.EqualFold(walletAddr, agentAddr) {
		logger.Infof("⚠️⚠️⚠️ WARNING: Main wallet address (%s) matches Agent wallet address!", walletAddr)
		logger.Infof("   This indicates you may be using your main wallet private key, which poses extremely high security risks!")
		logger.Infof("   Recommendation: Immediately create a separate Agent Wallet on Hyperliquid official website")
		logger.Infof("   Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets")
	} else {
		logger.Infof("✓ Using Agent Wallet mode (secure)")
		logger.Infof("  └─ Agent wallet address: %s (for signing)", agentAddr)
		logger.Infof("  └─ Main wallet address: %s (holds funds)", walletAddr)
	}

	ctx := context.Background()

	// Create Exchange client (Exchange includes Info functionality)
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		apiURL,
		nil,        // Meta will be fetched automatically
		"",         // vault address (empty for personal account)
		walletAddr, // wallet address
		nil,        // SpotMeta will be fetched automatically
	)

	logger.Infof("✓ Hyperliquid trader initialized successfully (testnet=%v, wallet=%s)", testnet, walletAddr)

	// Get meta information (including precision and other configurations)
	meta, err := exchange.Info().Meta(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get meta information: %w", err)
	}

	// 🔍 Security check: Validate Agent wallet balance (should be close to 0)
	// Only check if using separate Agent wallet (not when main wallet is used as agent)
	if !strings.EqualFold(walletAddr, agentAddr) {
		agentState, err := exchange.Info().UserState(ctx, agentAddr)
		if err == nil && agentState != nil && agentState.CrossMarginSummary.AccountValue != "" {
			// Parse Agent wallet balance
			agentBalance, _ := strconv.ParseFloat(agentState.CrossMarginSummary.AccountValue, 64)

			if agentBalance > 100 {
				// Critical: Agent wallet holds too much funds
				logger.Infof("🚨🚨🚨 CRITICAL SECURITY WARNING 🚨🚨🚨")
				logger.Infof("   Agent wallet balance: %.2f USDC (exceeds safe threshold of 100 USDC)", agentBalance)
				logger.Infof("   Agent wallet address: %s", agentAddr)
				logger.Infof("   ⚠️  Agent wallets should only be used for signing and hold minimal/zero balance")
				logger.Infof("   ⚠️  High balance in Agent wallet poses security risks")
				logger.Infof("   📖 Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets")
				logger.Infof("   💡 Recommendation: Transfer funds to main wallet and keep Agent wallet balance near 0")
				return nil, fmt.Errorf("security check failed: Agent wallet balance too high (%.2f USDC), exceeds 100 USDC threshold", agentBalance)
			} else if agentBalance > 10 {
				// Warning: Agent wallet has some balance (acceptable but not ideal)
				logger.Infof("⚠️  Notice: Agent wallet address (%s) has some balance: %.2f USDC", agentAddr, agentBalance)
				logger.Infof("   While not critical, it's recommended to keep Agent wallet balance near 0 for security")
			} else {
				// OK: Agent wallet balance is safe
				logger.Infof("✓ Agent wallet balance is safe: %.2f USDC (near zero as recommended)", agentBalance)
			}
		} else if err != nil {
			// Failed to query agent balance - log warning but don't block initialization
			logger.Infof("⚠️  Could not verify Agent wallet balance (query failed): %v", err)
			logger.Infof("   Proceeding with initialization, but please manually verify Agent wallet balance is near 0")
		}
	}

	return &HyperliquidTrader{
		exchange:      exchange,
		ctx:           ctx,
		walletAddr:    walletAddr,
		meta:          meta,
		isCrossMargin: true, // Use cross margin mode by default
	}, nil
}

// GetBalance gets account balance
func (t *HyperliquidTrader) GetBalance() (map[string]interface{}, error) {
	logger.Infof("🔄 Calling Hyperliquid API to get account balance...")

	// ✅ Step 1: Query Spot account balance
	spotState, err := t.exchange.Info().SpotUserState(t.ctx, t.walletAddr)
	var spotUSDCBalance float64 = 0.0
	if err != nil {
		logger.Infof("⚠️ Failed to query Spot balance (may have no spot assets): %v", err)
	} else if spotState != nil && len(spotState.Balances) > 0 {
		for _, balance := range spotState.Balances {
			if balance.Coin == "USDC" {
				spotUSDCBalance, _ = strconv.ParseFloat(balance.Total, 64)
				logger.Infof("✓ Found Spot balance: %.2f USDC", spotUSDCBalance)
				break
			}
		}
	}

	// ✅ Step 2: Query Perpetuals contract account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		logger.Infof("❌ Hyperliquid Perpetuals API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account information: %w", err)
	}

	// Parse balance information (MarginSummary fields are all strings)
	result := make(map[string]interface{})

	// ✅ Step 3: Dynamically select correct summary based on margin mode (CrossMarginSummary or MarginSummary)
	var accountValue, totalMarginUsed float64
	var summaryType string
	var summary interface{}

	if t.isCrossMargin {
		// Cross margin mode: use CrossMarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.CrossMarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.CrossMarginSummary.TotalMarginUsed, 64)
		summaryType = "CrossMarginSummary (cross margin)"
		summary = accountState.CrossMarginSummary
	} else {
		// Isolated margin mode: use MarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.MarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.MarginSummary.TotalMarginUsed, 64)
		summaryType = "MarginSummary (isolated margin)"
		summary = accountState.MarginSummary
	}

	// 🔍 Debug: Print complete summary structure returned by API
	summaryJSON, _ := json.MarshalIndent(summary, "  ", "  ")
	logger.Infof("🔍 [DEBUG] Hyperliquid API %s complete data:", summaryType)
	logger.Infof("%s", string(summaryJSON))

	// ⚠️ Critical fix: Accumulate actual unrealized PnL from all positions
	totalUnrealizedPnl := 0.0
	for _, assetPos := range accountState.AssetPositions {
		unrealizedPnl, _ := strconv.ParseFloat(assetPos.Position.UnrealizedPnl, 64)
		totalUnrealizedPnl += unrealizedPnl
	}

	// ✅ Correctly understand Hyperliquid fields:
	// AccountValue = Total account equity (includes idle funds + position value + unrealized PnL)
	// TotalMarginUsed = Margin used by positions (included in AccountValue, for display only)
	//
	// To be compatible with auto_trader.go calculation logic (totalEquity = totalWalletBalance + totalUnrealizedProfit)
	// Need to return "wallet balance without unrealized PnL"
	walletBalanceWithoutUnrealized := accountValue - totalUnrealizedPnl

	// ✅ Step 4: Use Withdrawable field (PR #443)
	// Withdrawable is the official real withdrawable balance, more reliable than simple calculation
	availableBalance := 0.0
	if accountState.Withdrawable != "" {
		withdrawable, err := strconv.ParseFloat(accountState.Withdrawable, 64)
		if err == nil && withdrawable > 0 {
			availableBalance = withdrawable
			logger.Infof("✓ Using Withdrawable as available balance: %.2f", availableBalance)
		}
	}

	// Fallback: If no Withdrawable, use simple calculation
	if availableBalance == 0 && accountState.Withdrawable == "" {
		availableBalance = accountValue - totalMarginUsed
		if availableBalance < 0 {
			logger.Infof("⚠️ Calculated available balance is negative (%.2f), reset to 0", availableBalance)
			availableBalance = 0
		}
	}

	// ✅ Step 5: Correctly handle Spot + Perpetuals balance
	// Important: Spot is only added to total assets, not to available balance
	//      Reason: Spot and Perpetuals are independent accounts, manual ClassTransfer required for transfers
	totalWalletBalance := walletBalanceWithoutUnrealized + spotUSDCBalance

	result["totalWalletBalance"] = totalWalletBalance    // Total assets (Perp + Spot)
	result["availableBalance"] = availableBalance        // Available balance (Perpetuals only, excluding Spot)
	result["totalUnrealizedProfit"] = totalUnrealizedPnl // Unrealized PnL (from Perpetuals only)
	result["spotBalance"] = spotUSDCBalance              // Spot balance (returned separately)

	logger.Infof("✓ Hyperliquid complete account:")
	logger.Infof("  • Spot balance: %.2f USDC (manual transfer to Perpetuals required for opening positions)", spotUSDCBalance)
	logger.Infof("  • Perpetuals equity: %.2f USDC (wallet %.2f + unrealized %.2f)",
		accountValue,
		walletBalanceWithoutUnrealized,
		totalUnrealizedPnl)
	logger.Infof("  • Perpetuals available balance: %.2f USDC (directly usable for opening positions)", availableBalance)
	logger.Infof("  • Margin used: %.2f USDC", totalMarginUsed)
	logger.Infof("  • Total assets (Perp+Spot): %.2f USDC", totalWalletBalance)
	logger.Infof("  ⭐ Total assets: %.2f USDC | Perp available: %.2f USDC | Spot balance: %.2f USDC",
		totalWalletBalance, availableBalance, spotUSDCBalance)

	return result, nil
}

// GetPositions gets all positions
func (t *HyperliquidTrader) GetPositions() ([]map[string]interface{}, error) {
	// Get account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}

	// Iterate through all positions
	for _, assetPos := range accountState.AssetPositions {
		position := assetPos.Position

		// Position amount (string type)
		posAmt, _ := strconv.ParseFloat(position.Szi, 64)

		if posAmt == 0 {
			continue // Skip positions with zero amount
		}

		posMap := make(map[string]interface{})

		// Normalize symbol format (Hyperliquid uses "BTC", we convert to "BTCUSDT")
		symbol := position.Coin + "USDT"
		posMap["symbol"] = symbol

		// Position amount and direction
		if posAmt > 0 {
			posMap["side"] = "long"
			posMap["positionAmt"] = posAmt
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -posAmt // Convert to positive number
		}

		// Price information (EntryPx and LiquidationPx are pointer types)
		var entryPrice, liquidationPx float64
		if position.EntryPx != nil {
			entryPrice, _ = strconv.ParseFloat(*position.EntryPx, 64)
		}
		if position.LiquidationPx != nil {
			liquidationPx, _ = strconv.ParseFloat(*position.LiquidationPx, 64)
		}

		positionValue, _ := strconv.ParseFloat(position.PositionValue, 64)
		unrealizedPnl, _ := strconv.ParseFloat(position.UnrealizedPnl, 64)

		// Calculate mark price (positionValue / abs(posAmt))
		var markPrice float64
		if posAmt != 0 {
			markPrice = positionValue / absFloat(posAmt)
		}

		posMap["entryPrice"] = entryPrice
		posMap["markPrice"] = markPrice
		posMap["unRealizedProfit"] = unrealizedPnl
		posMap["leverage"] = float64(position.Leverage.Value)
		posMap["liquidationPrice"] = liquidationPx

		result = append(result, posMap)
	}

	return result, nil
}

// SetMarginMode sets margin mode (set together with SetLeverage)
func (t *HyperliquidTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Hyperliquid's margin mode is set in SetLeverage, only record here
	t.isCrossMargin = isCrossMargin
	marginModeStr := "cross margin"
	if !isCrossMargin {
		marginModeStr = "isolated margin"
	}
	logger.Infof("  ✓ %s will use %s mode", symbol, marginModeStr)
	return nil
}

// SetLeverage sets leverage
func (t *HyperliquidTrader) SetLeverage(symbol string, leverage int) error {
	// Hyperliquid symbol format (remove USDT suffix)
	coin := convertSymbolToHyperliquid(symbol)

	// Call UpdateLeverage (leverage int, name string, isCross bool)
	// Third parameter: true=cross margin mode, false=isolated margin mode
	_, err := t.exchange.UpdateLeverage(t.ctx, leverage, coin, t.isCrossMargin)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	logger.Infof("  ✓ %s leverage switched to %dx", symbol, leverage)
	return nil
}

// refreshMetaIfNeeded refreshes meta information when invalid (triggered when Asset ID is 0)
func (t *HyperliquidTrader) refreshMetaIfNeeded(coin string) error {
	assetID := t.exchange.Info().NameToAsset(coin)
	if assetID != 0 {
		return nil // Meta is normal, no refresh needed
	}

	logger.Infof("⚠️  Asset ID for %s is 0, attempting to refresh Meta information...", coin)

	// Refresh Meta information
	meta, err := t.exchange.Info().Meta(t.ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh Meta information: %w", err)
	}

	// ✅ Concurrency safe: Use write lock to protect meta field update
	t.metaMutex.Lock()
	t.meta = meta
	t.metaMutex.Unlock()

	logger.Infof("✅ Meta information refreshed, contains %d assets", len(meta.Universe))

	// Verify Asset ID after refresh
	assetID = t.exchange.Info().NameToAsset(coin)
	if assetID == 0 {
		return fmt.Errorf("❌ Even after refreshing Meta, Asset ID for %s is still 0. Possible reasons:\n"+
			"  1. This coin is not listed on Hyperliquid\n"+
			"  2. Coin name is incorrect (should be BTC not BTCUSDT)\n"+
			"  3. API connection issue", coin)
	}

	logger.Infof("✅ Asset ID check passed after refresh: %s -> %d", coin, assetID)
	return nil
}

// OpenLong opens a long position
func (t *HyperliquidTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price (for market order)
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: Price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Create market buy order (using IOC limit order with aggressive price)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc, // Immediate or Cancel (similar to market order)
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	logger.Infof("✓ Long position opened successfully: %s quantity: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0 // Hyperliquid does not return order ID
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// OpenShort opens a short position
func (t *HyperliquidTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: Price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Create market sell order
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	logger.Infof("✓ Short position opened successfully: %s quantity: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseLong closes a long position
func (t *HyperliquidTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: Price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Create close position order (sell + ReduceOnly)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true, // Only close position, don't open new position
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	logger.Infof("✓ Long position closed successfully: %s quantity: %.4f", symbol, roundedQuantity)

	// Cancel all pending orders for this coin after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseShort closes a short position
func (t *HyperliquidTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: Round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: Price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Create close position order (buy + ReduceOnly)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	logger.Infof("✓ Short position closed successfully: %s quantity: %.4f", symbol, roundedQuantity)

	// Cancel all pending orders for this coin after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CancelStopLossOrders only cancels stop loss orders (Hyperliquid cannot distinguish stop loss and take profit, cancel all)
func (t *HyperliquidTrader) CancelStopLossOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Cannot distinguish stop loss and take profit orders, so cancel all pending orders for this coin
	logger.Infof("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders only cancels take profit orders (Hyperliquid cannot distinguish stop loss and take profit, cancel all)
func (t *HyperliquidTrader) CancelTakeProfitOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Cannot distinguish stop loss and take profit orders, so cancel all pending orders for this coin
	logger.Infof("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelAllOrders cancels all pending orders for this coin
func (t *HyperliquidTrader) CancelAllOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all pending orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Cancel all pending orders for this coin
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
			}
		}
	}

	logger.Infof("  ✓ Cancelled all pending orders for %s", symbol)
	return nil
}

// CancelStopOrders cancels take profit/stop loss orders for this coin (used to adjust TP/SL positions)
func (t *HyperliquidTrader) CancelStopOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all pending orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Note: Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Therefore temporarily cancel all pending orders for this coin (including TP/SL orders)
	// This is safe because all old orders should be cleaned up before setting new TP/SL
	canceledCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
				continue
			}
			canceledCount++
		}
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ No pending orders to cancel for %s", symbol)
	} else {
		logger.Infof("  ✓ Cancelled %d pending orders for %s (including TP/SL orders)", canceledCount, symbol)
	}

	return nil
}

// GetMarketPrice gets market price
func (t *HyperliquidTrader) GetMarketPrice(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all market prices
	allMids, err := t.exchange.Info().AllMids(t.ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	// Find price for corresponding coin (allMids is map[string]string)
	if priceStr, ok := allMids[coin]; ok {
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			return priceFloat, nil
		}
		return 0, fmt.Errorf("price format error: %v", err)
	}

	return 0, fmt.Errorf("price not found for %s", symbol)
}

// SetStopLoss sets stop loss order
func (t *HyperliquidTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position stop loss = buy, long position stop loss = sell

	// ⚠️ Critical: Round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ⚠️ Critical: Price also needs to be processed to 5 significant figures
	roundedStopPrice := t.roundPriceToSigfigs(stopPrice)

	// Create stop loss order (Trigger Order)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,  // Use rounded quantity
		Price: roundedStopPrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedStopPrice,
				IsMarket:  true,
				Tpsl:      "sl", // stop loss
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("  Stop loss price set: %.4f", roundedStopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *HyperliquidTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position take profit = buy, long position take profit = sell

	// ⚠️ Critical: Round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ⚠️ Critical: Price also needs to be processed to 5 significant figures
	roundedTakeProfitPrice := t.roundPriceToSigfigs(takeProfitPrice)

	// Create take profit order (Trigger Order)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,        // Use rounded quantity
		Price: roundedTakeProfitPrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedTakeProfitPrice,
				IsMarket:  true,
				Tpsl:      "tp", // take profit
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	logger.Infof("  Take profit price set: %.4f", roundedTakeProfitPrice)
	return nil
}

// FormatQuantity formats quantity to correct precision
func (t *HyperliquidTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	coin := convertSymbolToHyperliquid(symbol)
	szDecimals := t.getSzDecimals(coin)

	// Format quantity using szDecimals
	formatStr := fmt.Sprintf("%%.%df", szDecimals)
	return fmt.Sprintf(formatStr, quantity), nil
}

// getSzDecimals gets quantity precision for coin
func (t *HyperliquidTrader) getSzDecimals(coin string) int {
	// ✅ Concurrency safe: Use read lock to protect meta field access
	t.metaMutex.RLock()
	defer t.metaMutex.RUnlock()

	if t.meta == nil {
		logger.Infof("⚠️  meta information is empty, using default precision 4")
		return 4 // Default precision
	}

	// Find corresponding coin in meta.Universe
	for _, asset := range t.meta.Universe {
		if asset.Name == coin {
			return asset.SzDecimals
		}
	}

	logger.Infof("⚠️  Precision information not found for %s, using default precision 4", coin)
	return 4 // Default precision
}

// roundToSzDecimals rounds quantity to correct precision
func (t *HyperliquidTrader) roundToSzDecimals(coin string, quantity float64) float64 {
	szDecimals := t.getSzDecimals(coin)

	// Calculate multiplier (10^szDecimals)
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}

	// Round
	return float64(int(quantity*multiplier+0.5)) / multiplier
}

// roundPriceToSigfigs rounds price to 5 significant figures
// Hyperliquid requires prices to use 5 significant figures
func (t *HyperliquidTrader) roundPriceToSigfigs(price float64) float64 {
	if price == 0 {
		return 0
	}

	const sigfigs = 5 // Hyperliquid standard: 5 significant figures

	// Calculate price magnitude
	var magnitude float64
	if price < 0 {
		magnitude = -price
	} else {
		magnitude = price
	}

	// Calculate required multiplier
	multiplier := 1.0
	for magnitude >= 10 {
		magnitude /= 10
		multiplier /= 10
	}
	for magnitude < 1 {
		magnitude *= 10
		multiplier *= 10
	}

	// Apply significant figures precision
	for i := 0; i < sigfigs-1; i++ {
		multiplier *= 10
	}

	// Round
	rounded := float64(int(price*multiplier+0.5)) / multiplier
	return rounded
}

// convertSymbolToHyperliquid converts standard symbol to Hyperliquid format
// Example: "BTCUSDT" -> "BTC"
func convertSymbolToHyperliquid(symbol string) string {
	// Remove USDT suffix
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}

// GetOrderStatus gets order status
// Hyperliquid uses IOC orders, usually filled or cancelled immediately
// For completed orders, need to query historical records
func (t *HyperliquidTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	// Hyperliquid's IOC orders are completed almost immediately
	// If order was placed through this system, returned status will be FILLED
	// Try to query open orders to determine if still pending
	coin := convertSymbolToHyperliquid(symbol)

	// First check if in open orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		// If query fails, assume order is completed
		return map[string]interface{}{
			"orderId":     orderID,
			"status":      "FILLED",
			"avgPrice":    0.0,
			"executedQty": 0.0,
			"commission":  0.0,
		}, nil
	}

	// Check if order is in open orders list
	for _, order := range openOrders {
		if order.Coin == coin && fmt.Sprintf("%d", order.Oid) == orderID {
			// Order is still pending
			return map[string]interface{}{
				"orderId":     orderID,
				"status":      "NEW",
				"avgPrice":    0.0,
				"executedQty": 0.0,
				"commission":  0.0,
			}, nil
		}
	}

	// Order not in open list, meaning completed or cancelled
	// Hyperliquid IOC orders not in open list are usually filled
	return map[string]interface{}{
		"orderId":     orderID,
		"status":      "FILLED",
		"avgPrice":    0.0, // Hyperliquid does not directly return execution price, need to get from position info
		"executedQty": 0.0,
		"commission":  0.0,
	}, nil
}

// absFloat returns absolute value of float
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
