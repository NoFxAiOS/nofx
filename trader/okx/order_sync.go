package okx

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strconv"
	"strings"
	"time"
)

// OKXTrade represents a trade record from OKX fills history
type OKXTrade struct {
	InstID      string
	Symbol      string
	TradeID     string
	OrderID     string
	Side        string // buy or sell
	PosSide     string // long or short
	FillPrice   float64
	FillQty     float64 // In contracts
	FillQtyBase float64 // In base asset (BTC, ETH, etc)
	Fee         float64
	FeeAsset    string
	ExecTime    time.Time
	IsMaker     bool
	OrderType   string
	OrderAction string // open_long, open_short, close_long, close_short
}

// GetTrades retrieves trade/fill records from OKX
func (t *OKXTrader) GetTrades(startTime time.Time, limit int) ([]OKXTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // OKX max limit is 100
	}

	// Build query path
	// OKX fills-history endpoint for historical fills
	path := fmt.Sprintf("/api/v5/trade/fills-history?instType=SWAP&limit=%d", limit)
	if !startTime.IsZero() {
		path += fmt.Sprintf("&begin=%d", startTime.UnixMilli())
	}

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get fills history: %w", err)
	}

	var fills []struct {
		InstID   string `json:"instId"`   // e.g., "BTC-USDT-SWAP"
		TradeID  string `json:"tradeId"`  // Trade ID
		OrdID    string `json:"ordId"`    // Order ID
		BillID   string `json:"billId"`   // Bill ID
		Side     string `json:"side"`     // buy or sell
		PosSide  string `json:"posSide"`  // long, short, or net
		FillPx   string `json:"fillPx"`   // Fill price
		FillSz   string `json:"fillSz"`   // Fill size (contracts)
		Fee      string `json:"fee"`      // Fee (negative for cost)
		FeeCcy   string `json:"feeCcy"`   // Fee currency
		Ts       string `json:"ts"`       // Trade timestamp (ms)
		ExecType string `json:"execType"` // T: taker, M: maker
		Tag      string `json:"tag"`      // Order tag
	}

	if err := json.Unmarshal(data, &fills); err != nil {
		return nil, fmt.Errorf("failed to parse fills: %w", err)
	}

	trades := make([]OKXTrade, 0, len(fills))

	for _, fill := range fills {
		fillPrice, _ := strconv.ParseFloat(fill.FillPx, 64)
		fillSz, _ := strconv.ParseFloat(fill.FillSz, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
		ts, _ := strconv.ParseInt(fill.Ts, 10, 64)

		// Convert symbol: BTC-USDT-SWAP -> BTCUSDT
		symbol := t.convertSymbolBack(fill.InstID)

		// Convert contract count to base asset quantity
		fillQtyBase := fillSz
		inst, err := t.getInstrument(symbol)
		if err == nil && inst.CtVal > 0 {
			fillQtyBase = fillSz * inst.CtVal
		}

		// Determine order action based on side and posSide
		// OKX uses dual position mode:
		// - buy + long = open long
		// - sell + long = close long
		// - sell + short = open short
		// - buy + short = close short
		orderAction := "open_long"
		posSide := strings.ToLower(fill.PosSide)
		side := strings.ToLower(fill.Side)

		if posSide == "long" {
			if side == "buy" {
				orderAction = "open_long"
			} else {
				orderAction = "close_long"
			}
		} else if posSide == "short" {
			if side == "sell" {
				orderAction = "open_short"
			} else {
				orderAction = "close_short"
			}
		} else {
			// One-way mode (net position)
			if side == "buy" {
				orderAction = "open_long"
			} else {
				orderAction = "open_short"
			}
		}

		trade := OKXTrade{
			InstID:      fill.InstID,
			Symbol:      symbol,
			TradeID:     fill.TradeID,
			OrderID:     fill.OrdID,
			Side:        fill.Side,
			PosSide:     fill.PosSide,
			FillPrice:   fillPrice,
			FillQty:     fillSz,
			FillQtyBase: fillQtyBase,
			Fee:         -fee, // OKX returns negative fee
			FeeAsset:    fill.FeeCcy,
			ExecTime:    time.UnixMilli(ts).UTC(),
			IsMaker:     fill.ExecType == "M",
			OrderType:   "MARKET",
			OrderAction: orderAction,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// SyncOrdersFromOKX syncs OKX exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("okx")
func (t *OKXTrader) SyncOrdersFromOKX(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing OKX trades from: %s", startTime.Format(time.RFC3339))

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 100)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from OKX", len(trades))

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

		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

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
			PositionSide:    positionSide,
			Type:            trade.OrderType,
			OrderAction:     trade.OrderAction,
			Quantity:        trade.FillQtyBase,
			Price:           trade.FillPrice,
			Status:          "FILLED",
			FilledQuantity:  trade.FillQtyBase,
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
			Quantity:        trade.FillQtyBase,
			QuoteQuantity:   trade.FillPrice * trade.FillQtyBase,
			Commission:      trade.Fee,
			CommissionAsset: trade.FeeAsset,
			RealizedPnL:     0, // OKX fills don't include PnL per trade
			IsMaker:         trade.IsMaker,
			CreatedAt:       execTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, trade.OrderAction,
			trade.FillQtyBase, trade.FillPrice, trade.Fee, 0, // No per-trade PnL from OKX
			execTimeMs, trade.TradeID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, trade.OrderAction, trade.FillQtyBase)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f fee=%.6f action=%s",
			trade.TradeID, trade.Symbol, side, trade.FillQtyBase, trade.FillPrice, trade.Fee, trade.OrderAction)
	}

	logger.Infof("✅ OKX order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for OKX
func (t *OKXTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromOKX(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  OKX order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 OKX order sync started (interval: %v)", interval)
}
