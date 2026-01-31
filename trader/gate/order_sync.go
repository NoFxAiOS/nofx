package gate

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
)

// GateTrade represents a trade record from Gate fill history
type GateTrade struct {
	Symbol      string
	TradeID     string
	OrderID     string
	Side        string // buy or sell
	FillPrice   float64
	FillQty     float64 // In base currency (e.g., ETH), not contracts
	Fee         float64
	FeeAsset    string
	ExecTime    time.Time
	ProfitLoss  float64
	OrderType   string
	OrderAction string // open_long, open_short, close_long, close_short
}

// GetTrades retrieves trade/fill records from Gate
func (t *GateTrader) GetTrades(startTime time.Time, limit int) ([]GateTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // Gate max limit
	}

	opts := &gateapi.GetMyTradesOpts{
		Limit: optional.NewInt32(int32(limit)),
	}

	// Get trades from Gate API
	trades, _, err := t.client.FuturesApi.GetMyTrades(t.ctx, "usdt", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history: %w", err)
	}

	logger.Infof("📥 Received %d trades from Gate", len(trades))

	result := make([]GateTrade, 0, len(trades))

	for _, trade := range trades {
		// Filter by start time
		createTime := int64(trade.CreateTime)
		if createTime < startTime.Unix() {
			continue
		}

		fillPrice, _ := strconv.ParseFloat(trade.Price, 64)

		// Get quanto_multiplier for this contract to convert size to base currency
		quantoMultiplier := 1.0
		contract, err := t.getContract(trade.Contract)
		if err == nil && contract != nil {
			qm, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
			if qm > 0 {
				quantoMultiplier = qm
			}
		}

		// Convert contract size to actual quantity
		absSize := trade.Size
		if absSize < 0 {
			absSize = -absSize
		}
		fillQty := float64(absSize) * quantoMultiplier

		// Determine side and order action based on size and close_size
		// Gate close_size field determines if trade is opening or closing:
		// close_size=0 && size>0: Open long
		// close_size=0 && size<0: Open short
		// close_size>0 && size>0: Close short (and possibly open long if size > close_size)
		// close_size<0 && size<0: Close long (and possibly open short if |size| > |close_size|)
		side := "BUY"
		orderAction := "open_long"

		if trade.Size > 0 {
			side = "BUY"
			if trade.CloseSize > 0 {
				// Closing short position
				orderAction = "close_short"
			} else {
				// Opening long position
				orderAction = "open_long"
			}
		} else if trade.Size < 0 {
			side = "SELL"
			if trade.CloseSize < 0 {
				// Closing long position
				orderAction = "close_long"
			} else {
				// Opening short position
				orderAction = "open_short"
			}
		}

		// Calculate fee (Gate returns fee as negative value)
		fee, _ := strconv.ParseFloat(trade.Fee, 64)
		if fee < 0 {
			fee = -fee
		}

		// For closed positions, estimate PnL (Gate doesn't directly provide it in trade record)
		pnl := 0.0
		if strings.Contains(orderAction, "close") {
			// PnL would need to be calculated from position history
			// For now, we leave it as 0 and let position builder handle it
		}

		gateTrade := GateTrade{
			Symbol:      trade.Contract,
			TradeID:     fmt.Sprintf("%d", trade.Id),
			OrderID:     trade.OrderId,
			Side:        side,
			FillPrice:   fillPrice,
			FillQty:     fillQty,
			Fee:         fee,
			FeeAsset:    "USDT",
			ExecTime:    time.Unix(createTime, 0).UTC(),
			ProfitLoss:  pnl,
			OrderType:   "MARKET",
			OrderAction: orderAction,
		}

		result = append(result, gateTrade)
	}

	return result, nil
}

// SyncOrdersFromGate syncs Gate exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("gate")
func (t *GateTrader) SyncOrdersFromGate(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Gate trades from: %s", startTime.Format(time.RFC3339))

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 100)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from Gate", len(trades))

	// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].ExecTime.UnixMilli() < trades[j].ExecTime.UnixMilli()
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

		// Normalize symbol (Gate uses BTC_USDT, normalize to BTCUSDT)
		symbol := market.Normalize(strings.ReplaceAll(trade.Symbol, "_", ""))

		// Determine position side from order action
		positionSide := "LONG"
		if strings.Contains(trade.OrderAction, "short") {
			positionSide = "SHORT"
		}

		// Normalize side for storage
		side := strings.ToUpper(trade.Side)

		// Create order record - use UTC time in milliseconds to avoid timezone issues
		execTimeMs := trade.ExecTime.UTC().UnixMilli()
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    "BOTH", // Gate uses one-way position mode
			Type:            trade.OrderType,
			OrderAction:     trade.OrderAction,
			Quantity:        trade.FillQty,
			Price:           trade.FillPrice,
			Status:          "FILLED",
			FilledQuantity:  trade.FillQty,
			AvgFillPrice:    trade.FillPrice,
			Commission:      trade.Fee,
			FilledAt:        execTimeMs,
			CreatedAt:       execTimeMs,
			UpdatedAt:       execTimeMs,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.TradeID, err)
			continue
		}

		// Create fill record - use UTC time in milliseconds
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.OrderID,
			ExchangeTradeID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.FillPrice,
			Quantity:        trade.FillQty,
			QuoteQuantity:   trade.FillPrice * trade.FillQty,
			Commission:      trade.Fee,
			CommissionAsset: trade.FeeAsset,
			RealizedPnL:     trade.ProfitLoss,
			IsMaker:         false,
			CreatedAt:       execTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, trade.OrderAction,
			trade.FillQty, trade.FillPrice, trade.Fee, trade.ProfitLoss,
			execTimeMs, trade.TradeID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, trade.OrderAction, trade.FillQty)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s",
			trade.TradeID, symbol, side, trade.FillQty, trade.FillPrice, trade.ProfitLoss, trade.Fee, trade.OrderAction)
	}

	logger.Infof("✅ Gate order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for Gate
func (t *GateTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromGate(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Gate order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Gate order sync started (interval: %v)", interval)
}
