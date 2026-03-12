package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/trader/types"
	"strconv"
	"time"
)

// GetBalance retrieves account balance
func (t *BybitTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		balance := t.cachedBalance
		t.balanceCacheMutex.RUnlock()
		return balance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Call API
	params := map[string]interface{}{
		"accountType": "UNIFIED",
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetAccountWallet(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get Bybit balance: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	// Extract balance information
	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Bybit balance return format error")
	}

	list, _ := resultData["list"].([]interface{})

	var totalEquity, availableBalance, totalWalletBalance, totalPerpUPL float64 = 0, 0, 0, 0

	if len(list) > 0 {
		account, _ := list[0].(map[string]interface{})
		if equityStr, ok := account["totalEquity"].(string); ok {
			totalEquity, _ = strconv.ParseFloat(equityStr, 64)
		}
		if availStr, ok := account["totalAvailableBalance"].(string); ok {
			availableBalance, _ = strconv.ParseFloat(availStr, 64)
		}
		// Bybit UNIFIED account wallet balance field
		if walletStr, ok := account["totalWalletBalance"].(string); ok {
			totalWalletBalance, _ = strconv.ParseFloat(walletStr, 64)
		}
		// Bybit perpetual contract unrealized PnL
		if uplStr, ok := account["totalPerpUPL"].(string); ok {
			totalPerpUPL, _ = strconv.ParseFloat(uplStr, 64)
		}
	}

	// If no totalWalletBalance, use totalEquity
	if totalWalletBalance == 0 {
		totalWalletBalance = totalEquity
	}

	balance := map[string]interface{}{
		"totalEquity":           totalEquity,
		"totalWalletBalance":    totalWalletBalance,
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": totalPerpUPL,
		"balance":               totalEquity, // Compatible with other exchange formats
	}

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return balance, nil
}

// GetClosedPnL retrieves closed position PnL records from Bybit via direct HTTP API
func (t *BybitTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// The Bybit SDK doesn't expose the closed-pnl endpoint, use direct HTTP call
	return t.getClosedPnLViaHTTP(startTime, limit)
}

// getClosedPnLViaHTTP makes direct HTTP call to Bybit API for closed PnL with proper signing
func (t *BybitTrader) getClosedPnLViaHTTP(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// Build query string
	queryParams := fmt.Sprintf("category=linear&startTime=%d&limit=%d", startTime.UnixMilli(), limit)
	url := "https://api.bybit.com/v5/position/closed-pnl?" + queryParams

	// Generate timestamp
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	recvWindow := "5000"

	// Build signature payload: timestamp + api_key + recv_window + queryString
	signPayload := timestamp + t.apiKey + recvWindow + queryParams

	// Generate HMAC-SHA256 signature
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(signPayload))
	signature := hex.EncodeToString(h.Sum(nil))

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Bybit V5 API headers
	req.Header.Set("X-BAPI-API-KEY", t.apiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-SIGN-TYPE", "2")
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	req.Header.Set("Content-Type", "application/json")

	// Use http.DefaultClient for the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Bybit API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		RetCode int                    `json:"retCode"`
		RetMsg  string                 `json:"retMsg"`
		Result  map[string]interface{} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	return t.parseClosedPnLResult(result.Result)
}

// parseClosedPnLResult parses the closed PnL result from Bybit API
func (t *BybitTrader) parseClosedPnLResult(resultData interface{}) ([]types.ClosedPnLRecord, error) {
	data, ok := resultData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format")
	}

	list, _ := data["list"].([]interface{})
	var records []types.ClosedPnLRecord

	for _, item := range list {
		pnl, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse fields
		symbol, _ := pnl["symbol"].(string)
		side, _ := pnl["side"].(string)
		orderId, _ := pnl["orderId"].(string)

		avgEntryPriceStr, _ := pnl["avgEntryPrice"].(string)
		avgExitPriceStr, _ := pnl["avgExitPrice"].(string)
		qtyStr, _ := pnl["qty"].(string)
		closedPnLStr, _ := pnl["closedPnl"].(string)
		cumEntryValueStr, _ := pnl["cumEntryValue"].(string)
		cumExitValueStr, _ := pnl["cumExitValue"].(string)
		leverageStr, _ := pnl["leverage"].(string)
		createdTimeStr, _ := pnl["createdTime"].(string)
		updatedTimeStr, _ := pnl["updatedTime"].(string)

		avgEntryPrice, _ := strconv.ParseFloat(avgEntryPriceStr, 64)
		avgExitPrice, _ := strconv.ParseFloat(avgExitPriceStr, 64)
		qty, _ := strconv.ParseFloat(qtyStr, 64)
		closedPnL, _ := strconv.ParseFloat(closedPnLStr, 64)
		leverage, _ := strconv.ParseInt(leverageStr, 10, 64)
		createdTime, _ := strconv.ParseInt(createdTimeStr, 10, 64)
		updatedTime, _ := strconv.ParseInt(updatedTimeStr, 10, 64)

		// Calculate approximate fee from value difference
		cumEntryValue, _ := strconv.ParseFloat(cumEntryValueStr, 64)
		cumExitValue, _ := strconv.ParseFloat(cumExitValueStr, 64)
		expectedPnL := cumExitValue - cumEntryValue
		if side == "Sell" {
			expectedPnL = cumEntryValue - cumExitValue
		}
		fee := expectedPnL - closedPnL
		if fee < 0 {
			fee = 0
		}

		// Normalize side
		normalizedSide := "long"
		if side == "Sell" {
			normalizedSide = "short"
		}

		record := types.ClosedPnLRecord{
			Symbol:      symbol,
			Side:        normalizedSide,
			EntryPrice:  avgEntryPrice,
			ExitPrice:   avgExitPrice,
			Quantity:    qty,
			RealizedPnL: closedPnL,
			Fee:         fee,
			Leverage:    int(leverage),
			EntryTime:   time.UnixMilli(createdTime).UTC(),
			ExitTime:    time.UnixMilli(updatedTime).UTC(),
			OrderID:     orderId,
			CloseType:   "unknown", // Bybit doesn't provide close type directly
			ExchangeID:  orderId,   // Use orderId as exchange ID
		}

		records = append(records, record)
	}

	return records, nil
}
