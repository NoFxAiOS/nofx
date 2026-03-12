package indodax

import (
	"encoding/json"
	"fmt"
	"net/url"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
	"time"
)

// GetBalance gets account balance from Indodax
func (t *IndodaxTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.cacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cached := t.cachedBalance
		t.cacheMutex.RUnlock()
		return cached, nil
	}
	t.cacheMutex.RUnlock()

	params := url.Values{}
	params.Set("method", "getInfo")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	var result struct {
		ServerTime  int64                  `json:"server_time"`
		Balance     map[string]interface{} `json:"balance"`
		BalanceHold map[string]interface{} `json:"balance_hold"`
		UserID      string                 `json:"user_id"`
		Name        string                 `json:"name"`
		Email       string                 `json:"email"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse balance: %w", err)
	}

	// Calculate total balance in IDR
	idrBalance := parseFloat(result.Balance["idr"])
	idrHold := parseFloat(result.BalanceHold["idr"])
	totalIDR := idrBalance + idrHold

	balance := map[string]interface{}{
		"totalWalletBalance":    totalIDR,
		"availableBalance":      idrBalance,
		"totalUnrealizedProfit": 0.0,
		"totalEquity":           totalIDR,
		"balance":               totalIDR,
		"idr_balance":           idrBalance,
		"idr_hold":              idrHold,
		"currency":              "IDR",
		"user_id":               result.UserID,
		"server_time":           result.ServerTime,
	}

	// Add individual crypto balances
	for currency, amount := range result.Balance {
		if currency != "idr" {
			balance["balance_"+currency] = parseFloat(amount)
		}
	}
	for currency, amount := range result.BalanceHold {
		if currency != "idr" {
			balance["hold_"+currency] = parseFloat(amount)
		}
	}

	// Update cache
	t.cacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.cacheMutex.Unlock()

	return balance, nil
}

// GetPositions returns currently held crypto balances as "positions"
// Since Indodax is spot-only, each non-zero crypto balance is treated as a position
func (t *IndodaxTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.cacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionCacheTime) < t.cacheDuration {
		cached := t.cachedPositions
		t.cacheMutex.RUnlock()
		return cached, nil
	}
	t.cacheMutex.RUnlock()

	params := url.Values{}
	params.Set("method", "getInfo")

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result struct {
		Balance     map[string]interface{} `json:"balance"`
		BalanceHold map[string]interface{} `json:"balance_hold"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse positions: %w", err)
	}

	var positions []map[string]interface{}

	for currency, amountRaw := range result.Balance {
		if currency == "idr" {
			continue
		}

		amount := parseFloat(amountRaw)
		holdAmount := parseFloat(result.BalanceHold[currency])
		totalAmount := amount + holdAmount

		if totalAmount <= 0 {
			continue
		}

		// Get market price for this coin
		markPrice, _ := t.GetMarketPrice(strings.ToUpper(currency) + "IDR")

		// Calculate position value in IDR
		notionalValue := totalAmount * markPrice

		position := map[string]interface{}{
			"symbol":           strings.ToUpper(currency) + "IDR",
			"side":             "LONG",
			"positionAmt":      totalAmount,
			"entryPrice":       markPrice, // Spot doesn't track entry price
			"markPrice":        markPrice,
			"unRealizedProfit": 0.0, // Spot doesn't track unrealized PnL
			"leverage":         1.0,
			"mgnMode":          "spot",
			"notionalValue":    notionalValue,
			"currency":         currency,
			"available":        amount,
			"hold":             holdAmount,
		}

		positions = append(positions, position)
	}

	// Update cache
	t.cacheMutex.Lock()
	t.cachedPositions = positions
	t.positionCacheTime = time.Now()
	t.cacheMutex.Unlock()

	return positions, nil
}

// GetClosedPnL gets closed position PnL records (trade history)
func (t *IndodaxTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// Indodax trade history is limited to 7 days range
	params := url.Values{}
	params.Set("method", "tradeHistory")
	params.Set("pair", "btc_idr") // Default pair; Indodax requires a pair
	if limit > 0 {
		params.Set("count", strconv.Itoa(limit))
	}
	if !startTime.IsZero() {
		params.Set("since", strconv.FormatInt(startTime.Unix(), 10))
	}

	data, err := t.doPrivateRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history: %w", err)
	}

	var result struct {
		Trades []struct {
			TradeID       string `json:"trade_id"`
			OrderID       string `json:"order_id"`
			Type          string `json:"type"`
			Price         string `json:"price"`
			Fee           string `json:"fee"`
			TradeTime     string `json:"trade_time"`
			ClientOrderID string `json:"client_order_id"`
		} `json:"trades"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		// Trade history might return empty, that's fine
		logger.Infof("[Indodax] Trade history parse note: %v", err)
		return nil, nil
	}

	var records []types.ClosedPnLRecord
	for _, trade := range result.Trades {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		fee, _ := strconv.ParseFloat(trade.Fee, 64)
		tradeTime, _ := strconv.ParseInt(trade.TradeTime, 10, 64)

		side := "long"
		if trade.Type == "sell" {
			side = "long" // Selling from a spot position is closing long
		}

		records = append(records, types.ClosedPnLRecord{
			Symbol:    "BTCIDR",
			Side:      side,
			ExitPrice: price,
			Fee:       fee,
			ExitTime:  time.Unix(tradeTime, 0),
			OrderID:   trade.OrderID,
			CloseType: "manual",
		})
	}

	return records, nil
}
