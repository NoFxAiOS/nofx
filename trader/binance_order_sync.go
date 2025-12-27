package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
	"time"
)

// SyncOrdersFromBinance syncs Binance Futures trade history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
func (t *FuturesTrader) SyncOrdersFromBinance(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Binance trades from: %s", startTime.Format(time.RFC3339))

	// Get list of symbols to sync from current positions and recent income
	symbols, err := t.getActiveSymbols(startTime)
	if err != nil {
		return fmt.Errorf("failed to get active symbols: %w", err)
	}

	if len(symbols) == 0 {
		logger.Infof("📭 No active symbols to sync")
		return nil
	}

	logger.Infof("📊 Found %d symbols to sync: %v", len(symbols), symbols)

	// Collect all trades from all symbols
	var allTrades []TradeRecord
	for _, symbol := range symbols {
		trades, err := t.GetTradesForSymbol(symbol, startTime, 500)
		if err != nil {
			logger.Infof("  ⚠️ Failed to get trades for %s: %v", symbol, err)
			continue
		}
		allTrades = append(allTrades, trades...)
	}

	logger.Infof("📥 Received %d trades from Binance", len(allTrades))

	if len(allTrades) == 0 {
		return nil
	}

	// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(allTrades, func(i, j int) bool {
		return allTrades[i].Time.Before(allTrades[j].Time)
	})

	// Process trades one by one
	orderStore := st.Order()
	positionStore := st.Position()
	posBuilder := store.NewPositionBuilder(positionStore)
	syncedCount := 0

	for _, trade := range allTrades {
		// Check if trade already exists
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.TradeID)
		if err == nil && existing != nil {
			continue // Trade already exists, skip
		}

		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

		// Determine order action based on side and position side
		orderAction := t.determineOrderAction(trade.Side, trade.PositionSide, trade.RealizedPnL)

		// Determine position side for position builder
		positionSide := trade.PositionSide
		if positionSide == "" || positionSide == "BOTH" {
			// Infer from order action
			if strings.Contains(orderAction, "long") {
				positionSide = "LONG"
			} else {
				positionSide = "SHORT"
			}
		}

		// Normalize side
		side := strings.ToUpper(trade.Side)

		// Create order record
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        trade.Quantity,
			Price:           trade.Price,
			Status:          "FILLED",
			FilledQuantity:  trade.Quantity,
			AvgFillPrice:    trade.Price,
			Commission:      trade.Fee,
			FilledAt:        trade.Time,
			CreatedAt:       trade.Time,
			UpdatedAt:       trade.Time,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.TradeID, err)
			continue
		}

		// Create fill record
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.TradeID,
			ExchangeTradeID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.Price,
			Quantity:        trade.Quantity,
			QuoteQuantity:   trade.Price * trade.Quantity,
			Commission:      trade.Fee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.RealizedPnL,
			IsMaker:         false,
			CreatedAt:       trade.Time,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, orderAction,
			trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
			trade.Time, trade.TradeID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, orderAction, trade.Quantity)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s",
			trade.TradeID, symbol, side, trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee, orderAction)
	}

	logger.Infof("✅ Binance order sync completed: %d new trades synced", syncedCount)
	return nil
}

// getActiveSymbols returns list of symbols that have positions or recent trades
func (t *FuturesTrader) getActiveSymbols(startTime time.Time) ([]string, error) {
	symbolMap := make(map[string]bool)

	// Get symbols from current positions
	positions, err := t.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if symbol, ok := pos["symbol"].(string); ok && symbol != "" {
				symbolMap[symbol] = true
			}
		}
	}

	// Get symbols from recent income (REALIZED_PNL = closures)
	incomes, err := t.GetTrades(startTime, 500)
	if err == nil {
		for _, income := range incomes {
			if income.Symbol != "" {
				symbolMap[income.Symbol] = true
			}
		}
	}

	var symbols []string
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// determineOrderAction determines the order action based on trade data
func (t *FuturesTrader) determineOrderAction(side, positionSide string, realizedPnL float64) string {
	side = strings.ToUpper(side)
	positionSide = strings.ToUpper(positionSide)

	// If there's realized PnL, it's likely a close trade
	isClose := realizedPnL != 0

	if positionSide == "LONG" || positionSide == "" {
		if side == "BUY" {
			if isClose {
				return "close_short" // Buying to close short
			}
			return "open_long"
		} else {
			if isClose {
				return "close_long" // Selling to close long
			}
			return "open_short"
		}
	} else if positionSide == "SHORT" {
		if side == "SELL" {
			if isClose {
				return "close_long"
			}
			return "open_short"
		} else {
			if isClose {
				return "close_short"
			}
			return "open_long"
		}
	}

	// Default fallback
	if side == "BUY" {
		return "open_long"
	}
	return "open_short"
}

// StartOrderSync starts background order sync task for Binance
func (t *FuturesTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Binance order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Binance order sync started (interval: %v)", interval)
}
