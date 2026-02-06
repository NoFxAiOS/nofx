package kucoin

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/store"
	"nofx/trader/types"
	"sort"
	"strings"
	"time"
)

// KuCoinTrade represents a trade record from KuCoin fill history
type KuCoinTrade struct {
	Symbol      string
	TradeID     string
	OrderID     string
	Side        string // buy or sell
	FillPrice   float64
	FillQty     float64 // In base currency (e.g., ETH), not lots
	Fee         float64
	FeeAsset    string
	ExecTime    time.Time
	ProfitLoss  float64
	OrderAction string // open_long, open_short, close_long, close_short
}

// GetTrades retrieves trade/fill records from KuCoin
func (t *KuCoinTrader) GetTrades(startTime time.Time, limit int) ([]KuCoinTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // KuCoin max limit
	}

	// Build query path
	path := fmt.Sprintf("%s?pageSize=%d", kucoinFillsPath, limit)
	if !startTime.IsZero() {
		path += fmt.Sprintf("&startAt=%d", startTime.UnixMilli())
	}

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history: %w", err)
	}

	var response struct {
		CurrentPage int `json:"currentPage"`
		PageSize    int `json:"pageSize"`
		TotalNum    int `json:"totalNum"`
		TotalPage   int `json:"totalPage"`
		Items       []struct {
			Symbol      string `json:"symbol"`
			TradeId     string `json:"tradeId"`
			OrderId     string `json:"orderId"`
			Side        string `json:"side"`
			Price       string `json:"price"`
			Size        int64  `json:"size"`
			Value       string `json:"value"`       // Trade value in quote currency
			Fee         string `json:"fee"`         // Total fee
			FeeRate     string `json:"feeRate"`     // Fee rate
			FeeCurrency string `json:"feeCurrency"` // Fee currency (USDT)
			OpenFeePay  string `json:"openFeePay"`  // Fee for opening (>0 means opening trade)
			CloseFeePay string `json:"closeFeePay"` // Fee for closing (>0 means closing trade)
			TradeTime   int64  `json:"tradeTime"`   // Nanoseconds
			MarginMode  string `json:"marginMode"`  // CROSS or ISOLATED
			OrderType   string `json:"orderType"`   // market, limit
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse trade history: %w", err)
	}

	logger.Infof("📥 Received %d trades from KuCoin", len(response.Items))

	result := make([]KuCoinTrade, 0, len(response.Items))

	for _, trade := range response.Items {
		// Parse numeric values from strings
		var fillPrice, fee, openFeePay, closeFeePay float64
		fmt.Sscanf(trade.Price, "%f", &fillPrice)
		fmt.Sscanf(trade.Fee, "%f", &fee)
		fmt.Sscanf(trade.OpenFeePay, "%f", &openFeePay)
		fmt.Sscanf(trade.CloseFeePay, "%f", &closeFeePay)

		// Get multiplier from contract info
		symbol := t.convertSymbolBack(trade.Symbol)
		var multiplier float64
		contract, err := t.getContract(symbol)
		if err == nil && contract != nil {
			multiplier = contract.Multiplier
		} else {
			// Default multipliers based on symbol
			if strings.Contains(symbol, "BTC") {
				multiplier = 0.001
			} else {
				multiplier = 0.01 // Default for altcoins
			}
		}

		// Convert lots to actual quantity
		absSize := trade.Size
		if absSize < 0 {
			absSize = -absSize
		}
		fillQty := float64(absSize) * multiplier

		// Determine side and order action
		// KuCoin uses openFeePay/closeFeePay to indicate if trade is opening or closing
		side := strings.ToUpper(trade.Side) // BUY or SELL
		isClosing := closeFeePay > 0

		var orderAction string
		if trade.Side == "buy" {
			if isClosing {
				// Buying to close short
				orderAction = "close_short"
			} else {
				// Buying to open long
				orderAction = "open_long"
			}
		} else { // sell
			if isClosing {
				// Selling to close long
				orderAction = "close_long"
			} else {
				// Selling to open short
				orderAction = "open_short"
			}
		}

		// Trade time is in nanoseconds
		execTime := time.Unix(0, trade.TradeTime)

		result = append(result, KuCoinTrade{
			Symbol:      symbol,
			TradeID:     trade.TradeId,
			OrderID:     trade.OrderId,
			Side:        side,
			FillPrice:   fillPrice,
			FillQty:     fillQty,
			Fee:         fee,
			FeeAsset:    trade.FeeCurrency,
			ExecTime:    execTime,
			ProfitLoss:  0, // KuCoin fills API doesn't return PnL per trade
			OrderAction: orderAction,
		})
	}

	// Sort by execution time (oldest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ExecTime.Before(result[j].ExecTime)
	})

	return result, nil
}

// GetRecentTrades retrieves recent trades (faster, no pagination)
func (t *KuCoinTrader) GetRecentTrades() ([]KuCoinTrade, error) {
	data, err := t.doRequest("GET", kucoinRecentFillsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent trades: %w", err)
	}

	var trades []struct {
		Symbol      string `json:"symbol"`
		TradeId     string `json:"tradeId"`
		OrderId     string `json:"orderId"`
		Side        string `json:"side"`
		Price       string `json:"price"`
		Size        int64  `json:"size"`
		Fee         string `json:"fee"`
		FeeCurrency string `json:"feeCurrency"`
		OpenFeePay  string `json:"openFeePay"`
		CloseFeePay string `json:"closeFeePay"`
		TradeTime   int64  `json:"tradeTime"`
	}

	if err := json.Unmarshal(data, &trades); err != nil {
		return nil, fmt.Errorf("failed to parse recent trades: %w", err)
	}

	result := make([]KuCoinTrade, 0, len(trades))

	for _, trade := range trades {
		var fillPrice, fee, openFeePay, closeFeePay float64
		fmt.Sscanf(trade.Price, "%f", &fillPrice)
		fmt.Sscanf(trade.Fee, "%f", &fee)
		fmt.Sscanf(trade.OpenFeePay, "%f", &openFeePay)
		fmt.Sscanf(trade.CloseFeePay, "%f", &closeFeePay)

		// Get multiplier from contract info
		symbol := t.convertSymbolBack(trade.Symbol)
		var multiplier float64
		contract, err := t.getContract(symbol)
		if err == nil && contract != nil {
			multiplier = contract.Multiplier
		} else {
			if strings.Contains(symbol, "BTC") {
				multiplier = 0.001
			} else {
				multiplier = 0.01
			}
		}

		absSize := trade.Size
		if absSize < 0 {
			absSize = -absSize
		}
		fillQty := float64(absSize) * multiplier

		side := strings.ToUpper(trade.Side)
		isClosing := closeFeePay > 0

		var orderAction string
		if trade.Side == "buy" {
			if isClosing {
				orderAction = "close_short"
			} else {
				orderAction = "open_long"
			}
		} else {
			if isClosing {
				orderAction = "close_long"
			} else {
				orderAction = "open_short"
			}
		}

		execTime := time.Unix(0, trade.TradeTime)

		result = append(result, KuCoinTrade{
			Symbol:      symbol,
			TradeID:     trade.TradeId,
			OrderID:     trade.OrderId,
			Side:        side,
			FillPrice:   fillPrice,
			FillQty:     fillQty,
			Fee:         fee,
			FeeAsset:    trade.FeeCurrency,
			ExecTime:    execTime,
			ProfitLoss:  0,
			OrderAction: orderAction,
		})
	}

	return result, nil
}

// ToTradeRecord converts KuCoinTrade to types.TradeRecord
func (t *KuCoinTrade) ToTradeRecord() types.TradeRecord {
	// Determine position side from order action
	positionSide := "LONG"
	if strings.Contains(t.OrderAction, "short") {
		positionSide = "SHORT"
	}

	return types.TradeRecord{
		TradeID:      t.TradeID,
		Symbol:       t.Symbol,
		Side:         t.Side,
		PositionSide: positionSide,
		OrderAction:  t.OrderAction,
		Price:        t.FillPrice,
		Quantity:     t.FillQty,
		RealizedPnL:  t.ProfitLoss,
		Fee:          t.Fee,
		Time:         t.ExecTime,
	}
}

// SyncOrdersFromKuCoin syncs KuCoin exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("kucoin")
func (t *KuCoinTrader) SyncOrdersFromKuCoin(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing KuCoin trades from: %s", startTime.Format(time.RFC3339))

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 100)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from KuCoin", len(trades))

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

		// Symbol is already normalized in GetTrades
		symbol := trade.Symbol

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
			PositionSide:    "BOTH", // KuCoin uses one-way position mode
			Type:            "MARKET",
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

	logger.Infof("✅ KuCoin order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for KuCoin
func (t *KuCoinTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromKuCoin(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  KuCoin order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 KuCoin order sync started (interval: %v)", interval)
}
