package lighter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
)

// getFullAccountInfo Fetch full account info from Lighter API (includes balance and positions)
// Supports both main accounts and sub-accounts
func (t *LighterTraderV2) getFullAccountInfo() (*AccountInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/account?by=l1_address&value=%s", t.baseURL, t.walletAddr)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get account (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response - Lighter may return accounts in "accounts" or "sub_accounts" field
	var accountResp AccountResponse
	if err := json.Unmarshal(body, &accountResp); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}

	// Check for API error code
	if accountResp.Code != 0 && accountResp.Code != 200 {
		return nil, fmt.Errorf("Lighter API error (code %d): %s", accountResp.Code, accountResp.Message)
	}

	// Combine both accounts and sub_accounts - some users have sub-accounts
	var allAccounts []AccountInfo
	allAccounts = append(allAccounts, accountResp.Accounts...)
	allAccounts = append(allAccounts, accountResp.SubAccounts...)

	if len(allAccounts) == 0 {
		return nil, fmt.Errorf("no account found for wallet address: %s (try depositing funds first at app.lighter.xyz)", t.walletAddr)
	}

	// Find the account that matches our stored accountIndex, or use the first one
	var account *AccountInfo
	for i := range allAccounts {
		acc := &allAccounts[i]
		// Use index field if account_index is 0
		if acc.AccountIndex == 0 && acc.Index != 0 {
			acc.AccountIndex = acc.Index
		}
		// Match by stored accountIndex if we have one
		if t.accountIndex != 0 && acc.AccountIndex == t.accountIndex {
			account = acc
			break
		}
	}

	// If no specific match, use the first account
	if account == nil {
		account = &allAccounts[0]
		if account.AccountIndex == 0 && account.Index != 0 {
			account.AccountIndex = account.Index
		}
	}

	return account, nil
}

// GetBalance Get account balance (implements Trader interface)
func (t *LighterTraderV2) GetBalance() (map[string]interface{}, error) {
	balance, err := t.GetAccountBalance()
	if err != nil {
		return nil, err
	}

	// Calculate wallet balance (total equity - unrealized PnL)
	walletBalance := balance.TotalEquity - balance.UnrealizedPnL

	// Return in standard format compatible with auto_types.go
	// (totalEquity = totalWalletBalance + totalUnrealizedProfit)
	return map[string]interface{}{
		"totalWalletBalance":    walletBalance,           // Wallet balance (excluding unrealized PnL)
		"totalUnrealizedProfit": balance.UnrealizedPnL,   // Unrealized PnL
		"availableBalance":      balance.AvailableBalance, // Available balance
		// Keep additional fields for reference
		"total_equity":       balance.TotalEquity,
		"margin_used":        balance.MarginUsed,
		"maintenance_margin": balance.MaintenanceMargin,
	}, nil
}

// GetAccountBalance Get detailed account balance information
func (t *LighterTraderV2) GetAccountBalance() (*AccountBalance, error) {
	// Get full account info from Lighter API
	accountInfo, err := t.getFullAccountInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	// Parse string values to float64
	availableBalance, _ := strconv.ParseFloat(accountInfo.AvailableBalance, 64)
	collateral, _ := strconv.ParseFloat(accountInfo.Collateral, 64)
	crossAssetValue, _ := strconv.ParseFloat(accountInfo.CrossAssetValue, 64)
	totalEquity, _ := strconv.ParseFloat(accountInfo.TotalEquity, 64)
	unrealizedPnl, _ := strconv.ParseFloat(accountInfo.UnrealizedPnl, 64)

	// Use collateral as total equity if total_equity is 0
	if totalEquity == 0 {
		totalEquity = collateral
	}

	// Calculate margin used (collateral - available)
	marginUsed := collateral - availableBalance
	if marginUsed < 0 {
		marginUsed = 0
	}

	// Calculate maintenance margin from positions
	// Lighter API doesn't return maintenance_margin directly, estimate from initial_margin_fraction
	var maintenanceMargin float64
	for _, pos := range accountInfo.Positions {
		posValue, _ := strconv.ParseFloat(pos.PositionValue, 64)
		imf, _ := strconv.ParseFloat(pos.InitialMarginFraction, 64)
		// Maintenance margin is typically ~half of initial margin
		if imf > 0 {
			maintenanceMargin += posValue * (imf / 100.0) * 0.5
		}
	}

	balance := &AccountBalance{
		TotalEquity:       totalEquity,
		AvailableBalance:  availableBalance,
		MarginUsed:        marginUsed,
		UnrealizedPnL:     unrealizedPnl,
		MaintenanceMargin: maintenanceMargin,
	}

	logger.Infof("✓ Lighter balance: equity=%.2f, available=%.2f, crossValue=%.2f",
		totalEquity, availableBalance, crossAssetValue)

	return balance, nil
}

// GetPositions Get all positions (implements Trader interface)
func (t *LighterTraderV2) GetPositions() ([]map[string]interface{}, error) {
	positions, err := t.GetPositionsRaw("")
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		// Return in standard format compatible with auto_types.go
		result = append(result, map[string]interface{}{
			"symbol":           pos.Symbol,
			"side":             pos.Side,
			"positionAmt":      pos.Size,             // Standard field name
			"entryPrice":       pos.EntryPrice,       // Standard field name
			"markPrice":        pos.MarkPrice,        // Standard field name
			"liquidationPrice": pos.LiquidationPrice, // Standard field name
			"unRealizedProfit": pos.UnrealizedPnL,    // Standard field name
			"leverage":         pos.Leverage,
			"marginUsed":       pos.MarginUsed,
		})
	}

	return result, nil
}

// GetPositionsRaw Get all positions (returns raw type)
func (t *LighterTraderV2) GetPositionsRaw(symbol string) ([]Position, error) {
	// Get full account info from Lighter API
	accountInfo, err := t.getFullAccountInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	// Normalize symbol for filtering
	normalizedSymbol := ""
	if symbol != "" {
		normalizedSymbol = normalizeSymbol(symbol)
	}

	// Convert Lighter positions to our Position type
	var positions []Position
	for _, lPos := range accountInfo.Positions {
		// Filter by symbol if specified
		if normalizedSymbol != "" && !strings.EqualFold(lPos.Symbol, normalizedSymbol) {
			continue
		}

		// Parse fields from Lighter API response
		size, _ := strconv.ParseFloat(lPos.Position, 64)        // API returns "position" not "size"
		entryPrice, _ := strconv.ParseFloat(lPos.AvgEntryPrice, 64) // API returns "avg_entry_price"
		positionValue, _ := strconv.ParseFloat(lPos.PositionValue, 64)
		liqPrice, _ := strconv.ParseFloat(lPos.LiquidationPrice, 64)
		pnl, _ := strconv.ParseFloat(lPos.UnrealizedPnl, 64)
		initialMarginFraction, _ := strconv.ParseFloat(lPos.InitialMarginFraction, 64)
		allocatedMargin, _ := strconv.ParseFloat(lPos.AllocatedMargin, 64)

		// Skip empty positions
		if size == 0 {
			continue
		}

		// Calculate mark price from position value: mark_price = position_value / position
		markPrice := 0.0
		if size != 0 {
			markPrice = positionValue / size
		}

		// Calculate leverage from initial margin fraction: leverage = 100 / margin_fraction
		leverage := 1.0
		if initialMarginFraction > 0 {
			leverage = 100.0 / initialMarginFraction
		}

		// Calculate margin used (for cross margin, use position_value / leverage)
		marginUsed := allocatedMargin
		if marginUsed == 0 && leverage > 0 {
			marginUsed = positionValue / leverage
		}

		// Determine side based on sign field (1 = long, -1 = short)
		side := "long"
		if lPos.Sign < 0 {
			side = "short"
		}

		pos := Position{
			Symbol:           lPos.Symbol,
			Side:             side,
			Size:             size,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			LiquidationPrice: liqPrice,
			UnrealizedPnL:    pnl,
			Leverage:         leverage,
			MarginUsed:       marginUsed,
		}
		positions = append(positions, pos)

		logger.Infof("✓ Lighter position: %s %s size=%.4f entry=%.2f mark=%.2f lev=%.1fx pnl=%.4f",
			lPos.Symbol, side, size, entryPrice, markPrice, leverage, pnl)
	}

	logger.Infof("✓ Lighter positions: found %d positions", len(positions))
	return positions, nil
}

// GetPosition Get position for specified symbol
func (t *LighterTraderV2) GetPosition(symbol string) (*Position, error) {
	positions, err := t.GetPositionsRaw(symbol)
	if err != nil {
		return nil, err
	}

	normalizedSymbol := normalizeSymbol(symbol)
	for _, pos := range positions {
		if strings.EqualFold(pos.Symbol, normalizedSymbol) && pos.Size > 0 {
			return &pos, nil
		}
	}

	return nil, nil // No position
}

// GetMarketPrice Get market price (implements Trader interface)
func (t *LighterTraderV2) GetMarketPrice(symbol string) (float64, error) {
	// Normalize symbol to Lighter format
	normalizedSymbol := normalizeSymbol(symbol)

	// Get market_id first
	marketID, err := t.getMarketIndex(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to get market ID: %w", err)
	}

	// Use orderBookDetails endpoint which contains price info
	endpoint := fmt.Sprintf("%s/api/v1/orderBookDetails?market_id=%d", t.baseURL, marketID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return 0, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get market price (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Code             int `json:"code"`
		OrderBookDetails []struct {
			Symbol         string  `json:"symbol"`
			LastTradePrice float64 `json:"last_trade_price"`
			DailyPriceLow  float64 `json:"daily_price_low"`
			DailyPriceHigh float64 `json:"daily_price_high"`
		} `json:"order_book_details"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return 0, fmt.Errorf("API error code: %d", apiResp.Code)
	}

	// Find the market
	for _, ob := range apiResp.OrderBookDetails {
		if strings.EqualFold(ob.Symbol, normalizedSymbol) {
			price := ob.LastTradePrice
			if price <= 0 {
				return 0, fmt.Errorf("invalid price for %s: %.2f", normalizedSymbol, price)
			}

			logger.Infof("✓ Lighter %s price: %.2f", normalizedSymbol, price)
			return price, nil
		}
	}

	return 0, fmt.Errorf("market not found: %s", normalizedSymbol)
}

// FormatQuantity Format quantity to correct precision (implements Trader interface)
func (t *LighterTraderV2) FormatQuantity(symbol string, quantity float64) (string, error) {
	// TODO: Get symbol precision from API
	// Using default precision for now
	return fmt.Sprintf("%.4f", quantity), nil
}

// GetOrderBook Get order book (implements GridTrader interface)
// Returns bids and asks as [][]float64 where each element is [price, quantity]
func (t *LighterTraderV2) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	// Get market_id first
	marketID, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get market ID: %w", err)
	}

	// Get order book from Lighter API
	endpoint := fmt.Sprintf("%s/api/v1/orderBook?market_id=%d", t.baseURL, marketID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("failed to get order book (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Code int `json:"code"`
		Data struct {
			Bids [][]interface{} `json:"bids"` // [[price, quantity], ...]
			Asks [][]interface{} `json:"asks"` // [[price, quantity], ...]
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	if apiResp.Code != 200 {
		return nil, nil, fmt.Errorf("API error code: %d", apiResp.Code)
	}

	// Helper to parse price/quantity from interface{}
	parseFloat := func(v interface{}) float64 {
		if f, ok := v.(float64); ok {
			return f
		}
		if s, ok := v.(string); ok {
			f, _ := strconv.ParseFloat(s, 64)
			return f
		}
		return 0
	}

	// Convert bids to [][]float64
	maxBids := len(apiResp.Data.Bids)
	if depth > 0 && depth < maxBids {
		maxBids = depth
	}
	bids = make([][]float64, 0, maxBids)
	for i := 0; i < maxBids; i++ {
		if len(apiResp.Data.Bids[i]) >= 2 {
			price := parseFloat(apiResp.Data.Bids[i][0])
			qty := parseFloat(apiResp.Data.Bids[i][1])
			if price > 0 && qty > 0 {
				bids = append(bids, []float64{price, qty})
			}
		}
	}

	// Convert asks to [][]float64
	maxAsks := len(apiResp.Data.Asks)
	if depth > 0 && depth < maxAsks {
		maxAsks = depth
	}
	asks = make([][]float64, 0, maxAsks)
	for i := 0; i < maxAsks; i++ {
		if len(apiResp.Data.Asks[i]) >= 2 {
			price := parseFloat(apiResp.Data.Asks[i][0])
			qty := parseFloat(apiResp.Data.Asks[i][1])
			if price > 0 && qty > 0 {
				asks = append(asks, []float64{price, qty})
			}
		}
	}

	if len(bids) > 0 && len(asks) > 0 {
		logger.Infof("✓ Lighter order book: %s best_bid=%.2f, best_ask=%.2f, depth=%d/%d",
			symbol, bids[0][0], asks[0][0], len(bids), len(asks))
	}

	return bids, asks, nil
}
