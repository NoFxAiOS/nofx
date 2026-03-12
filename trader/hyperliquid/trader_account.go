package hyperliquid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
	"time"
)

// GetBalance gets account balance
func (t *HyperliquidTrader) GetBalance() (map[string]interface{}, error) {
	logger.Infof("🔄 Calling Hyperliquid API to get account balance...")

	// Step 1: Query Spot account balance
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

	// Step 2: Query Perpetuals contract account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		logger.Infof("❌ Hyperliquid Perpetuals API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account information: %w", err)
	}

	// Parse balance information (MarginSummary fields are all strings)
	result := make(map[string]interface{})

	// Step 3: Dynamically select correct summary based on margin mode (CrossMarginSummary or MarginSummary)
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

	// Debug: Print complete summary structure returned by API
	summaryJSON, _ := json.MarshalIndent(summary, "  ", "  ")
	logger.Infof("🔍 [DEBUG] Hyperliquid API %s complete data:", summaryType)
	logger.Infof("%s", string(summaryJSON))

	// Critical fix: Accumulate actual unrealized PnL from all positions
	totalUnrealizedPnl := 0.0
	for _, assetPos := range accountState.AssetPositions {
		unrealizedPnl, _ := strconv.ParseFloat(assetPos.Position.UnrealizedPnl, 64)
		totalUnrealizedPnl += unrealizedPnl
	}

	// Correctly understand Hyperliquid fields:
	// AccountValue = Total account equity (includes idle funds + position value + unrealized PnL)
	// TotalMarginUsed = Margin used by positions (included in AccountValue, for display only)
	//
	// To be compatible with auto_types.go calculation logic (totalEquity = totalWalletBalance + totalUnrealizedProfit)
	// Need to return "wallet balance without unrealized PnL"
	walletBalanceWithoutUnrealized := accountValue - totalUnrealizedPnl

	// Step 4: Use Withdrawable field (PR #443)
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

	// Step 5: Query xyz dex balance (stock perps, forex, commodities)
	var xyzAccountValue, xyzUnrealizedPnl float64
	var xyzPositions []xyzAssetPosition
	xyzAccountValue, xyzUnrealizedPnl, xyzPositions, err = t.getXYZDexBalance()
	if err != nil {
		// xyz dex query failed - log warning but don't fail the entire balance query
		logger.Infof("⚠️ Failed to query xyz dex balance: %v", err)
	}
	// Always log xyz dex state for debugging
	logger.Infof("🔍 xyz dex state: accountValue=%.4f, unrealizedPnl=%.4f, positions=%d",
		xyzAccountValue, xyzUnrealizedPnl, len(xyzPositions))
	for _, pos := range xyzPositions {
		entryPx := "nil"
		if pos.Position.EntryPx != nil {
			entryPx = *pos.Position.EntryPx
		}
		logger.Infof("   └─ %s: size=%s, entryPx=%s, posValue=%s, pnl=%s",
			pos.Position.Coin, pos.Position.Szi, entryPx, pos.Position.PositionValue, pos.Position.UnrealizedPnl)
	}
	xyzWalletBalance := xyzAccountValue - xyzUnrealizedPnl

	// Step 6: Correctly handle Spot + Perpetuals + xyz dex balance
	// Important: Each account is independent, manual transfers required
	totalWalletBalance := walletBalanceWithoutUnrealized + spotUSDCBalance + xyzWalletBalance
	totalUnrealizedPnlAll := totalUnrealizedPnl + xyzUnrealizedPnl

	// Calculate total equity properly: perpAccountValue + spotUSDCBalance + xyzAccountValue
	// Note: totalWalletBalance + totalUnrealizedPnlAll should equal this
	totalEquityCalculated := accountValue + spotUSDCBalance + xyzAccountValue

	// Step 7: Unified Account mode - Spot USDC is used as collateral for Perps
	// In this mode, available balance includes Spot USDC since it can be used for Perp margin
	if t.isUnifiedAccount && spotUSDCBalance > 0 {
		// Add Spot balance to available balance for trading
		availableBalance = availableBalance + spotUSDCBalance
		logger.Infof("✓ Unified Account: Spot %.2f USDC added to available balance (total: %.2f)",
			spotUSDCBalance, availableBalance)
	}

	// Suppress unused variable warning
	_ = totalUnrealizedPnlAll

	result["totalWalletBalance"] = totalWalletBalance       // Total assets (Perp + Spot + xyz) - unrealized
	result["totalEquity"] = totalEquityCalculated           // Total equity = Perp AV + Spot + xyz AV
	result["availableBalance"] = availableBalance           // Available balance (Perp + Spot if unified)
	result["totalUnrealizedProfit"] = totalUnrealizedPnlAll // Unrealized PnL (Perpetuals + xyz)
	result["spotBalance"] = spotUSDCBalance                 // Spot balance
	result["xyzDexBalance"] = xyzAccountValue               // xyz dex equity (stock perps, forex, commodities)
	result["xyzDexUnrealizedPnl"] = xyzUnrealizedPnl        // xyz dex unrealized PnL
	result["perpAccountValue"] = accountValue               // Perp account value for debugging

	logger.Infof("✓ Hyperliquid complete account:")
	logger.Infof("  • Spot balance: %.2f USDC", spotUSDCBalance)
	logger.Infof("  • Perpetuals equity: %.2f USDC (wallet %.2f + unrealized %.2f)",
		accountValue,
		walletBalanceWithoutUnrealized,
		totalUnrealizedPnl)
	logger.Infof("  • Perpetuals available balance: %.2f USDC", availableBalance)
	logger.Infof("  • Margin used: %.2f USDC", totalMarginUsed)
	logger.Infof("  • xyz dex equity: %.2f USDC (wallet %.2f + unrealized %.2f)",
		xyzAccountValue,
		xyzWalletBalance,
		xyzUnrealizedPnl)
	logger.Infof("  • Total assets (Perp+Spot+xyz): %.2f USDC", totalWalletBalance)
	logger.Infof("  ⭐ Total: %.2f USDC | Perp: %.2f | Spot: %.2f | xyz: %.2f",
		totalWalletBalance, availableBalance, spotUSDCBalance, xyzAccountValue)

	return result, nil
}

// xyzDexState represents the clearinghouse state for xyz dex
type xyzDexState struct {
	MarginSummary      *xyzMarginSummary  `json:"marginSummary,omitempty"`
	CrossMarginSummary *xyzMarginSummary  `json:"crossMarginSummary,omitempty"`
	Withdrawable       string             `json:"withdrawable,omitempty"`
	AssetPositions     []xyzAssetPosition `json:"assetPositions,omitempty"`
}

type xyzMarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

type xyzAssetPosition struct {
	Position struct {
		Coin          string  `json:"coin"`
		Szi           string  `json:"szi"`
		EntryPx       *string `json:"entryPx"`
		PositionValue string  `json:"positionValue"`
		UnrealizedPnl string  `json:"unrealizedPnl"`
		LiquidationPx *string `json:"liquidationPx"`
		Leverage      struct {
			Type  string `json:"type"`
			Value int    `json:"value"`
		} `json:"leverage"`
	} `json:"position"`
}

// getXYZDexBalance queries the xyz dex balance (stock perps, forex, commodities)
func (t *HyperliquidTrader) getXYZDexBalance() (accountValue float64, unrealizedPnl float64, positions []xyzAssetPosition, err error) {
	// Build request for xyz dex clearinghouse state
	reqBody := map[string]interface{}{
		"type": "clearinghouseState",
		"user": t.walletAddr,
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Determine API URL
	apiURL := "https://api.hyperliquid.xyz/info"
	// Note: xyz dex may not be available on testnet

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, 0, nil, fmt.Errorf("xyz dex API error (status %d): %s", resp.StatusCode, string(body))
	}

	var state xyzDexState
	if err := json.Unmarshal(body, &state); err != nil {
		return 0, 0, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse account value - xyz dex uses MarginSummary for isolated margin mode
	// CrossMarginSummary may exist but with 0 values, so check MarginSummary first
	if state.MarginSummary != nil && state.MarginSummary.AccountValue != "" {
		av, _ := strconv.ParseFloat(state.MarginSummary.AccountValue, 64)
		if av > 0 {
			accountValue = av
		}
	}
	// Fallback to CrossMarginSummary if MarginSummary is 0
	if accountValue == 0 && state.CrossMarginSummary != nil && state.CrossMarginSummary.AccountValue != "" {
		accountValue, _ = strconv.ParseFloat(state.CrossMarginSummary.AccountValue, 64)
	}

	// Calculate total unrealized PnL from positions
	for _, pos := range state.AssetPositions {
		pnl, _ := strconv.ParseFloat(pos.Position.UnrealizedPnl, 64)
		unrealizedPnl += pnl
	}

	return accountValue, unrealizedPnl, state.AssetPositions, nil
}

// GetMarketPrice gets market price (supports both crypto and xyz dex assets)
func (t *HyperliquidTrader) GetMarketPrice(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	if strings.HasPrefix(coin, "xyz:") {
		return t.getXyzMarketPrice(coin)
	}

	// Get all market prices for crypto
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

// getXyzMarketPrice gets market price for xyz dex assets
func (t *HyperliquidTrader) getXyzMarketPrice(coin string) (float64, error) {
	// Build request for xyz dex allMids
	reqBody := map[string]string{
		"type": "allMids",
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := "https://api.hyperliquid.xyz/info"

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("xyz dex allMids API error (status %d): %s", resp.StatusCode, string(body))
	}

	var mids map[string]string
	if err := json.Unmarshal(body, &mids); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	// The API returns keys with xyz: prefix, so ensure the coin has it
	lookupKey := coin
	if !strings.HasPrefix(lookupKey, "xyz:") {
		lookupKey = "xyz:" + lookupKey
	}

	if priceStr, ok := mids[lookupKey]; ok {
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			return priceFloat, nil
		}
		return 0, fmt.Errorf("price format error: %v", err)
	}

	return 0, fmt.Errorf("xyz dex price not found for %s (lookup key: %s)", coin, lookupKey)
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

// GetClosedPnL gets recent closing trades from Hyperliquid
// Note: Hyperliquid does NOT have a position history API, only fill history.
// This returns individual closing trades for real-time position closure detection.
func (t *HyperliquidTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	trades, err := t.GetTrades(startTime, limit)
	if err != nil {
		return nil, err
	}

	// Filter only closing trades (realizedPnl != 0)
	var records []types.ClosedPnLRecord
	for _, trade := range trades {
		if trade.RealizedPnL == 0 {
			continue
		}

		// Determine side (Hyperliquid uses one-way mode)
		side := "long"
		if trade.Side == "SELL" || trade.Side == "Sell" {
			side = "long" // Selling closes long
		} else {
			side = "short" // Buying closes short
		}

		// Calculate entry price from PnL
		var entryPrice float64
		if trade.Quantity > 0 {
			if side == "long" {
				entryPrice = trade.Price - trade.RealizedPnL/trade.Quantity
			} else {
				entryPrice = trade.Price + trade.RealizedPnL/trade.Quantity
			}
		}

		records = append(records, types.ClosedPnLRecord{
			Symbol:      trade.Symbol,
			Side:        side,
			EntryPrice:  entryPrice,
			ExitPrice:   trade.Price,
			Quantity:    trade.Quantity,
			RealizedPnL: trade.RealizedPnL,
			Fee:         trade.Fee,
			ExitTime:    trade.Time,
			EntryTime:   trade.Time,
			OrderID:     trade.TradeID,
			ExchangeID:  trade.TradeID,
			CloseType:   "unknown",
		})
	}

	return records, nil
}

// GetTrades retrieves trade history from Hyperliquid
func (t *HyperliquidTrader) GetTrades(startTime time.Time, limit int) ([]types.TradeRecord, error) {
	// Use UserFillsByTime API
	startTimeMs := startTime.UnixMilli()
	fills, err := t.exchange.Info().UserFillsByTime(t.ctx, t.walletAddr, startTimeMs, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user fills: %w", err)
	}

	var trades []types.TradeRecord
	for _, fill := range fills {
		price, _ := strconv.ParseFloat(fill.Price, 64)
		qty, _ := strconv.ParseFloat(fill.Size, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
		pnl, _ := strconv.ParseFloat(fill.ClosedPnl, 64)

		// Determine side: "B" = Buy, "S" = Sell (or "A" = Ask, "B" = Bid)
		var side string
		if fill.Side == "B" || fill.Side == "Buy" || fill.Side == "bid" {
			side = "BUY"
		} else {
			side = "SELL"
		}

		// Parse Dir field to get order action
		// Hyperliquid Dir values: "Open Long", "Open Short", "Close Long", "Close Short"
		var orderAction string
		switch strings.ToLower(fill.Dir) {
		case "open long":
			orderAction = "open_long"
		case "open short":
			orderAction = "open_short"
		case "close long":
			orderAction = "close_long"
		case "close short":
			orderAction = "close_short"
		default:
			// Fallback: use RealizedPnL if Dir is missing/unknown
			if pnl != 0 {
				if side == "BUY" {
					orderAction = "close_short"
				} else {
					orderAction = "close_long"
				}
			} else {
				if side == "BUY" {
					orderAction = "open_long"
				} else {
					orderAction = "open_short"
				}
			}
		}

		// Hyperliquid uses one-way mode, so PositionSide is "BOTH"
		trade := types.TradeRecord{
			TradeID:      strconv.FormatInt(fill.Tid, 10),
			Symbol:       fill.Coin,
			Side:         side,
			PositionSide: "BOTH", // Hyperliquid doesn't have hedge mode
			OrderAction:  orderAction,
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(fill.Time).UTC(),
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *HyperliquidTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var result []types.OpenOrder
	for _, order := range openOrders {
		if order.Coin != symbol {
			continue
		}

		side := "BUY"
		if order.Side == "A" {
			side = "SELL"
		}

		result = append(result, types.OpenOrder{
			OrderID:      fmt.Sprintf("%d", order.Oid),
			Symbol:       order.Coin,
			Side:         side,
			PositionSide: "",
			Type:         "LIMIT",
			Price:        order.LimitPx,
			StopPrice:    0,
			Quantity:     order.Size,
			Status:       "NEW",
		})
	}

	return result, nil
}

// GetOrderBook gets the order book for a symbol
// Implements GridTrader interface
func (t *HyperliquidTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	coin := convertSymbolToHyperliquid(symbol)

	l2Book, err := t.exchange.Info().L2Snapshot(t.ctx, coin)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	if l2Book == nil || len(l2Book.Levels) < 2 {
		return nil, nil, fmt.Errorf("invalid order book data")
	}

	// Parse bids (first level array)
	for i, level := range l2Book.Levels[0] {
		if i >= depth {
			break
		}
		bids = append(bids, []float64{level.Px, level.Sz})
	}

	// Parse asks (second level array)
	for i, level := range l2Book.Levels[1] {
		if i >= depth {
			break
		}
		asks = append(asks, []float64{level.Px, level.Sz})
	}

	return bids, asks, nil
}
