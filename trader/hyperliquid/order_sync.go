package hyperliquid

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
	"time"
)

// SyncOrdersFromHyperliquid syncs Hyperliquid exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("hyperliquid")
func (t *HyperliquidTrader) SyncOrdersFromHyperliquid(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Hyperliquid trades from: %s", startTime.Format(time.RFC3339))

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 1000)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from Hyperliquid", len(trades))

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

			// Use order action from trade (parsed from Hyperliquid Dir field)
			// Dir field values: "Open Long", "Open Short", "Close Long", "Close Short"
			orderAction := trade.OrderAction
			positionSide := "LONG"
			if strings.Contains(orderAction, "short") {
				positionSide = "SHORT"
			}

			// Create order record - use Unix milliseconds UTC
			tradeTimeMs := trade.Time.UTC().UnixMilli()
			orderRecord := &store.TraderOrder{
				TraderID:        traderID,
				ExchangeID:      exchangeID,   // UUID
				ExchangeType:    exchangeType, // Exchange type
				ExchangeOrderID: trade.TradeID,
				Symbol:          symbol,
				Side:            trade.Side,
				PositionSide:    "BOTH", // Hyperliquid uses one-way position mode
				Type:            "MARKET",
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
				Side:            trade.Side,
				Price:           trade.Price,
				Quantity:        trade.Quantity,
				QuoteQuantity:   trade.Price * trade.Quantity,
				Commission:      trade.Fee,
				CommissionAsset: "USDT",
				RealizedPnL:     trade.RealizedPnL,
				IsMaker:         false, // Hyperliquid GetTrades doesn't provide maker/taker info
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
			trade.TradeID, symbol, trade.Side, trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee, orderAction)
	}

	logger.Infof("✅ Order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task
func (t *HyperliquidTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromHyperliquid(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Hyperliquid order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Hyperliquid order sync started (interval: %v)", interval)
}
