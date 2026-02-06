package aster

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
	"time"
)

// SyncOrdersFromAster syncs Aster exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("aster")
func (t *AsterTrader) SyncOrdersFromAster(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Aster trades from: %s", startTime.Format(time.RFC3339))

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 500)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from Aster", len(trades))

	// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Time.UnixMilli() < trades[j].Time.UnixMilli()
	})

	// Process trades one by one (no transaction to avoid deadlock)
	orderStore := st.Order()
	positionStore := st.Position()
	posBuilder := store.NewPositionBuilder(positionStore)
	syncedCount := 0

	for _, trade := range trades {
		// Check if trade already exists (use exchangeID which is UUID, not exchange type)
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.TradeID)
		if err == nil && existing != nil {
			continue // Order already exists, skip
		}

		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

		// Determine order action based on side, positionSide, and realizedPnL
		// Aster uses one-way position mode (BOTH), so we need to infer from PnL
		// - RealizedPnL != 0 means it's a close trade
		// - RealizedPnL == 0 means it's an open trade
		orderAction := deriveAsterOrderAction(trade.Side, trade.PositionSide, trade.RealizedPnL)

		// Determine position side from order action
		positionSide := "LONG"
		if strings.Contains(orderAction, "short") {
			positionSide = "SHORT"
		}

		// Normalize side for storage
		side := strings.ToUpper(trade.Side)

		// Create order record - use Unix milliseconds UTC
		tradeTimeMs := trade.Time.UTC().UnixMilli()
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    "BOTH", // Aster uses one-way position mode
			Type:            "LIMIT",
			OrderAction:     orderAction,
			Quantity:        trade.Quantity,
			Price:           trade.Price,
			Status:          "FILLED",
			FilledQuantity:  trade.Quantity,
			AvgFillPrice:    trade.Price,
			Commission:      trade.Fee,
			FilledAt:        tradeTimeMs,
			CreatedAt:       tradeTimeMs,
			UpdatedAt:       tradeTimeMs,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.TradeID, err)
			continue
		}

		// Create fill record - use Unix milliseconds UTC
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
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
			CreatedAt:       tradeTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, orderAction,
			trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
			tradeTimeMs, trade.TradeID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, orderAction, trade.Quantity)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s",
			trade.TradeID, symbol, side, trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee, orderAction)
	}

	logger.Infof("✅ Aster order sync completed: %d new trades synced", syncedCount)
	return nil
}

// deriveAsterOrderAction determines order action from trade details
// Aster uses one-way position mode (BOTH), so we infer from:
// - Side: BUY or SELL
// - RealizedPnL: non-zero means closing trade
func deriveAsterOrderAction(side, positionSide string, realizedPnL float64) string {
	side = strings.ToUpper(side)
	positionSide = strings.ToUpper(positionSide)

	// Check if this is a closing trade (has realized PnL)
	isClose := realizedPnL != 0

	if positionSide == "LONG" {
		if isClose {
			return "close_long"
		}
		return "open_long"
	} else if positionSide == "SHORT" {
		if isClose {
			return "close_short"
		}
		return "open_short"
	} else {
		// BOTH mode - infer from side and PnL
		if side == "BUY" {
			if isClose {
				return "close_short" // Buying to close short
			}
			return "open_long" // Buying to open long
		} else {
			if isClose {
				return "close_long" // Selling to close long
			}
			return "open_short" // Selling to open short
		}
	}
}

// StartOrderSync starts background order sync task for Aster
func (t *AsterTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromAster(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Aster order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Aster order sync started (interval: %v)", interval)
}
