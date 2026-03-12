package bitget

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"strconv"
	"strings"
	"time"
)

// GetBalance gets account balance
func (t *BitgetTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetAccountPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	var accounts []struct {
		MarginCoin    string `json:"marginCoin"`
		Available     string `json:"available"`     // Available balance
		AccountEquity string `json:"accountEquity"` // Total equity
		UsdtEquity    string `json:"usdtEquity"`    // USDT equity
		UnrealizedPL  string `json:"unrealizedPL"`  // Unrealized P&L
	}

	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("failed to parse balance data: %w, raw: %s", err, string(data))
	}

	var totalEquity, availableBalance, unrealizedPnL float64
	for _, acc := range accounts {
		if acc.MarginCoin == "USDT" {
			totalEquity, _ = strconv.ParseFloat(acc.AccountEquity, 64)
			availableBalance, _ = strconv.ParseFloat(acc.Available, 64)
			unrealizedPnL, _ = strconv.ParseFloat(acc.UnrealizedPL, 64)
			logger.Infof("✓ [Bitget] Balance: equity=%.2f, available=%.2f", totalEquity, availableBalance)
			break
		}
	}

	result := map[string]interface{}{
		"totalWalletBalance":    totalEquity - unrealizedPnL,
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": unrealizedPnL,
		"total_equity":          totalEquity,
	}

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// SetMarginMode sets margin mode
func (t *BitgetTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	symbol = t.convertSymbol(symbol)

	marginMode := "isolated"
	if isCrossMargin {
		marginMode = "crossed"
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"marginMode":  marginMode,
	}

	_, err := t.doRequest("POST", bitgetMarginModePath, body)
	if err != nil {
		if strings.Contains(err.Error(), "same") || strings.Contains(err.Error(), "already") {
			return nil
		}
		if strings.Contains(err.Error(), "position") {
			logger.Infof("  ⚠️ %s has positions, cannot change margin mode", symbol)
			return nil
		}
		return err
	}

	logger.Infof("  ✓ %s margin mode set to %s", symbol, marginMode)
	return nil
}

// SetLeverage sets leverage
func (t *BitgetTrader) SetLeverage(symbol string, leverage int) error {
	symbol = t.convertSymbol(symbol)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"leverage":    fmt.Sprintf("%d", leverage),
	}

	_, err := t.doRequest("POST", bitgetLeveragePath, body)
	if err != nil {
		if strings.Contains(err.Error(), "same") {
			return nil
		}
		logger.Infof("  ⚠️ Failed to set %s leverage: %v", symbol, err)
		return err
	}

	logger.Infof("  ✓ %s leverage set to %dx", symbol, leverage)
	return nil
}

// GetMarketPrice gets market price
func (t *BitgetTrader) GetMarketPrice(symbol string) (float64, error) {
	symbol = t.convertSymbol(symbol)

	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}

	data, err := t.doRequest("GET", bitgetTickerPath, params)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	var tickers []struct {
		LastPr string `json:"lastPr"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, err
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("no price data received")
	}

	price, err := strconv.ParseFloat(tickers[0].LastPr, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// GetOrderBook gets the order book for a symbol
// Implements GridTrader interface
func (t *BitgetTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	symbol = t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v2/mix/market/depth?symbol=%s&productType=USDT-FUTURES&limit=%d", symbol, depth)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	var result struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	// Parse bids
	for _, b := range result.Bids {
		if len(b) >= 2 {
			price, _ := strconv.ParseFloat(b[0], 64)
			qty, _ := strconv.ParseFloat(b[1], 64)
			bids = append(bids, []float64{price, qty})
		}
	}

	// Parse asks
	for _, a := range result.Asks {
		if len(a) >= 2 {
			price, _ := strconv.ParseFloat(a[0], 64)
			qty, _ := strconv.ParseFloat(a[1], 64)
			asks = append(asks, []float64{price, qty})
		}
	}

	return bids, asks, nil
}
