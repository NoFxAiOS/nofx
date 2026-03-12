package binance

import (
	"context"
	"fmt"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"time"
)

// GetBalance gets account balance (with cache)
func (t *FuturesTrader) GetBalance() (map[string]interface{}, error) {
	// First check if cache is valid
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ Using cached account balance (cache age: %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	logger.Infof("🔄 Cache expired, calling Binance API to get account balance...")
	account, err := t.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		logger.Infof("❌ Binance API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	result := make(map[string]interface{})
	result["totalWalletBalance"], _ = strconv.ParseFloat(account.TotalWalletBalance, 64)
	result["availableBalance"], _ = strconv.ParseFloat(account.AvailableBalance, 64)
	result["totalUnrealizedProfit"], _ = strconv.ParseFloat(account.TotalUnrealizedProfit, 64)

	logger.Infof("✓ Binance API returned: total balance=%s, available=%s, unrealized PnL=%s",
		account.TotalWalletBalance,
		account.AvailableBalance,
		account.TotalUnrealizedProfit)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetClosedPnL retrieves recent closing trades from Binance Futures
// Note: Binance does NOT have a position history API, only trade history.
// This returns individual closing trades (realizedPnl != 0) for real-time position closure detection.
// NOT suitable for historical position reconstruction - use only for matching recent closures.
func (t *FuturesTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	trades, err := t.GetTrades(startTime, limit)
	if err != nil {
		return nil, err
	}

	// Filter only closing trades (realizedPnl != 0) and convert to ClosedPnLRecord
	var records []types.ClosedPnLRecord
	for _, trade := range trades {
		if trade.RealizedPnL == 0 {
			continue // Skip opening trades
		}

		// Determine side from trade
		side := "long"
		if trade.PositionSide == "SHORT" || trade.PositionSide == "short" {
			side = "short"
		} else if trade.PositionSide == "BOTH" || trade.PositionSide == "" {
			// One-way mode: selling closes long, buying closes short
			if trade.Side == "SELL" || trade.Side == "Sell" {
				side = "long"
			} else {
				side = "short"
			}
		}

		// Calculate entry price from PnL (mathematically accurate for this trade)
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
			EntryTime:   trade.Time, // Approximate
			OrderID:     trade.TradeID,
			ExchangeID:  trade.TradeID,
			CloseType:   "unknown",
		})
	}

	return records, nil
}

// GetTrades retrieves trade history from Binance Futures using Income API
// Note: Income API has delays (~minutes), for real-time use GetTradesForSymbol instead
func (t *FuturesTrader) GetTrades(startTime time.Time, limit int) ([]types.TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Use Income API to get REALIZED_PNL records (all symbols)
	incomes, err := t.client.NewGetIncomeHistoryService().
		IncomeType("REALIZED_PNL").
		StartTime(startTime.UnixMilli()).
		Limit(int64(limit)).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get income history: %w", err)
	}

	var trades []types.TradeRecord
	for _, income := range incomes {
		pnl, _ := strconv.ParseFloat(income.Income, 64)
		if pnl == 0 {
			continue // Skip zero PnL records
		}

		// Income API doesn't provide full trade details, create a minimal record
		// This is mainly used for detecting recent closures, not historical reconstruction
		trade := types.TradeRecord{
			TradeID:     strconv.FormatInt(income.TranID, 10),
			Symbol:      income.Symbol,
			RealizedPnL: pnl,
			Time:        time.UnixMilli(income.Time).UTC(),
			// Note: Income API doesn't provide price, quantity, side, fee
			// For accurate data, use GetTradesForSymbol with specific symbol
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetTradesForSymbol retrieves trade history for a specific symbol
// This is more reliable than using Income API which may have delays
func (t *FuturesTrader) GetTradesForSymbol(symbol string, startTime time.Time, limit int) ([]types.TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	accountTrades, err := t.client.NewListAccountTradeService().
		Symbol(symbol).
		StartTime(startTime.UnixMilli()).
		Limit(limit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history for %s: %w", symbol, err)
	}

	var trades []types.TradeRecord
	for _, at := range accountTrades {
		price, _ := strconv.ParseFloat(at.Price, 64)
		qty, _ := strconv.ParseFloat(at.Quantity, 64)
		fee, _ := strconv.ParseFloat(at.Commission, 64)
		pnl, _ := strconv.ParseFloat(at.RealizedPnl, 64)

		trade := types.TradeRecord{
			TradeID:      strconv.FormatInt(at.ID, 10),
			Symbol:       at.Symbol,
			Side:         string(at.Side),
			PositionSide: string(at.PositionSide),
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(at.Time).UTC(),
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetTradesForSymbolFromID retrieves trade history for a specific symbol starting from a given trade ID
// This is used for incremental sync - only fetch new trades since last sync
func (t *FuturesTrader) GetTradesForSymbolFromID(symbol string, fromID int64, limit int) ([]types.TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	accountTrades, err := t.client.NewListAccountTradeService().
		Symbol(symbol).
		FromID(fromID).
		Limit(limit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history for %s from ID %d: %w", symbol, fromID, err)
	}

	var trades []types.TradeRecord
	for _, at := range accountTrades {
		price, _ := strconv.ParseFloat(at.Price, 64)
		qty, _ := strconv.ParseFloat(at.Quantity, 64)
		fee, _ := strconv.ParseFloat(at.Commission, 64)
		pnl, _ := strconv.ParseFloat(at.RealizedPnl, 64)

		trade := types.TradeRecord{
			TradeID:      strconv.FormatInt(at.ID, 10),
			Symbol:       at.Symbol,
			Side:         string(at.Side),
			PositionSide: string(at.PositionSide),
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(at.Time).UTC(),
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetCommissionSymbols returns symbols that have new commission records since lastSyncTime
// COMMISSION income is generated for every trade, so this is more reliable than REALIZED_PNL
func (t *FuturesTrader) GetCommissionSymbols(lastSyncTime time.Time) ([]string, error) {
	incomes, err := t.client.NewGetIncomeHistoryService().
		IncomeType("COMMISSION").
		StartTime(lastSyncTime.UnixMilli()).
		Limit(1000).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get commission history: %w", err)
	}

	symbolMap := make(map[string]bool)
	for _, income := range incomes {
		if income.Symbol != "" {
			symbolMap[income.Symbol] = true
		}
	}

	var symbols []string
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// GetPnLSymbols returns symbols that have REALIZED_PNL records since lastSyncTime
// This is a fallback when COMMISSION detection fails (VIP users, BNB fee discount)
func (t *FuturesTrader) GetPnLSymbols(lastSyncTime time.Time) ([]string, error) {
	incomes, err := t.client.NewGetIncomeHistoryService().
		IncomeType("REALIZED_PNL").
		StartTime(lastSyncTime.UnixMilli()).
		Limit(1000).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get PnL history: %w", err)
	}

	symbolMap := make(map[string]bool)
	for _, income := range incomes {
		if income.Symbol != "" {
			symbolMap[income.Symbol] = true
		}
	}

	var symbols []string
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}
