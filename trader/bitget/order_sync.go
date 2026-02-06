package bitget

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

// BitgetTrade represents a trade record from Bitget fill history
type BitgetTrade struct {
	Symbol      string
	TradeID     string
	OrderID     string
	Side        string // buy or sell
	FillPrice   float64
	FillQty     float64
	Fee         float64
	FeeAsset    string
	ExecTime    time.Time
	ProfitLoss  float64
	OrderType   string
	OrderAction string // open_long, open_short, close_long, close_short
}

// GetTrades retrieves trade/fill records from Bitget
func (t *BitgetTrader) GetTrades(startTime time.Time, limit int) ([]BitgetTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // Bitget max limit is 100
	}

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", startTime.UnixMilli()),
		"limit":       fmt.Sprintf("%d", limit),
	}

	data, err := t.doRequest("GET", "/api/v2/mix/order/fill-history", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get fill history: %w", err)
	}


	// Bitget fill structure - supports both one-way and hedge mode
	type BitgetFill struct {
		TradeID    string `json:"tradeId"`
		Symbol     string `json:"symbol"`
		OrderID    string `json:"orderId"`
		Side       string `json:"side"`       // buy, sell
		Price      string `json:"price"`      // Fill price
		BaseVolume string `json:"baseVolume"` // Fill size in base currency
		Profit     string `json:"profit"`     // Realized PnL
		CTime      string `json:"cTime"`      // Fill time (ms)
		TradeSide  string `json:"tradeSide"`  // one-way: buy_single/sell_single, hedge: open/close
		FeeDetail  []struct {
			FeeCoin  string `json:"feeCoin"`
			TotalFee string `json:"totalFee"`
		} `json:"feeDetail"`
	}

	// Try parsing as wrapped response first (fillList field)
	var wrappedResp struct {
		FillList []BitgetFill `json:"fillList"`
	}

	// Try direct array format (Bitget V2 API returns data as direct array)
	var directFills []BitgetFill

	// Try wrapped format first
	if err := json.Unmarshal(data, &wrappedResp); err == nil && len(wrappedResp.FillList) > 0 {
		logger.Infof("🔍 Bitget: parsed as wrapped format, fillList count: %d", len(wrappedResp.FillList))
		directFills = wrappedResp.FillList
	} else {
		// Try direct array format
		if err := json.Unmarshal(data, &directFills); err != nil {
			logger.Infof("⚠️ Bitget fill-history parse failed, raw: %s", string(data))
			return nil, fmt.Errorf("failed to parse fills: %w", err)
		}
		logger.Infof("🔍 Bitget: parsed as direct array, fills count: %d", len(directFills))
	}

	trades := make([]BitgetTrade, 0, len(directFills))

	for _, fill := range directFills {
		fillPrice, _ := strconv.ParseFloat(fill.Price, 64)
		fillQty, _ := strconv.ParseFloat(fill.BaseVolume, 64)
		profit, _ := strconv.ParseFloat(fill.Profit, 64)
		cTime, _ := strconv.ParseInt(fill.CTime, 10, 64)

		// Extract fee from feeDetail array (Bitget V2 API)
		var fee float64
		var feeAsset string
		if len(fill.FeeDetail) > 0 {
			fee, _ = strconv.ParseFloat(fill.FeeDetail[0].TotalFee, 64)
			feeAsset = fill.FeeDetail[0].FeeCoin
		}

		// Determine order action based on side and tradeSide
		// Bitget one-way mode: buy_single (open long), sell_single (close long)
		// Bitget hedge mode: open + buy = open_long, close + sell = close_long
		orderAction := "open_long"
		side := strings.ToLower(fill.Side)
		tradeSide := strings.ToLower(fill.TradeSide)

		// One-way position mode (buy_single/sell_single)
		if tradeSide == "buy_single" {
			orderAction = "open_long"
		} else if tradeSide == "sell_single" {
			orderAction = "close_long"
		} else if tradeSide == "open" {
			// Hedge mode: open
			if side == "buy" {
				orderAction = "open_long"
			} else {
				orderAction = "open_short"
			}
		} else if tradeSide == "close" {
			// Hedge mode: close
			if side == "sell" {
				orderAction = "close_long"
			} else {
				orderAction = "close_short"
			}
		}

		trade := BitgetTrade{
			Symbol:      fill.Symbol,
			TradeID:     fill.TradeID,
			OrderID:     fill.OrderID,
			Side:        fill.Side,
			FillPrice:   fillPrice,
			FillQty:     fillQty,
			Fee:         -fee, // Bitget returns negative fee, convert to positive
			FeeAsset:    feeAsset,
			ExecTime:    time.UnixMilli(cTime).UTC(),
			ProfitLoss:  profit,
			OrderType:   "MARKET",
			OrderAction: orderAction,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// SyncOrdersFromBitget syncs Bitget exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("bitget")
func (t *BitgetTrader) SyncOrdersFromBitget(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Bitget trades from: %s", startTime.Format(time.RFC3339))

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 100)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from Bitget", len(trades))

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
			PositionSide:    "BOTH", // Bitget uses one-way position mode
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

	logger.Infof("✅ Bitget order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for Bitget
func (t *BitgetTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromBitget(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Bitget order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Bitget order sync started (interval: %v)", interval)
}
