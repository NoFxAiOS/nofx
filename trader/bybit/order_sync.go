package bybit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BybitTrade represents a trade record from Bybit execution list
type BybitTrade struct {
	Symbol      string
	OrderID     string
	ExecID      string
	Side        string // Buy or Sell
	ExecPrice   float64
	ExecQty     float64
	ExecFee     float64
	ExecTime    time.Time
	IsMaker     bool
	OrderType   string
	ClosedSize  float64 // For close orders
	ClosedPnL   float64
	OrderAction string // open_long, open_short, close_long, close_short
}

// GetTrades retrieves trade/execution records from Bybit
func (t *BybitTrader) GetTrades(startTime time.Time, limit int) ([]BybitTrade, error) {
	return t.getTradesViaHTTP(startTime, limit)
}

// getTradesViaHTTP makes direct HTTP call to Bybit API for execution list
func (t *BybitTrader) getTradesViaHTTP(startTime time.Time, limit int) ([]BybitTrade, error) {
	// Build query string
	queryParams := fmt.Sprintf("category=linear&startTime=%d&limit=%d", startTime.UnixMilli(), limit)
	url := "https://api.bybit.com/v5/execution/list?" + queryParams

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
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	return t.parseTradesResult(result.Result.List)
}

// parseTradesResult parses the execution list result from Bybit API
func (t *BybitTrader) parseTradesResult(list []map[string]interface{}) ([]BybitTrade, error) {
	var trades []BybitTrade

	for _, item := range list {
		symbol, _ := item["symbol"].(string)
		orderID, _ := item["orderId"].(string)
		execID, _ := item["execId"].(string)
		side, _ := item["side"].(string)
		orderType, _ := item["orderType"].(string)
		isMaker, _ := item["isMaker"].(bool)

		execPriceStr, _ := item["execPrice"].(string)
		execQtyStr, _ := item["execQty"].(string)
		execFeeStr, _ := item["execFee"].(string)
		closedSizeStr, _ := item["closedSize"].(string)
		closedPnlStr, _ := item["closedPnl"].(string)
		execTimeStr, _ := item["execTime"].(string)

		execPrice, _ := strconv.ParseFloat(execPriceStr, 64)
		execQty, _ := strconv.ParseFloat(execQtyStr, 64)
		execFee, _ := strconv.ParseFloat(execFeeStr, 64)
		closedSize, _ := strconv.ParseFloat(closedSizeStr, 64)
		closedPnl, _ := strconv.ParseFloat(closedPnlStr, 64)
		execTimeMs, _ := strconv.ParseInt(execTimeStr, 10, 64)
		execTime := time.UnixMilli(execTimeMs).UTC()

		// Determine order action based on side and closedSize
		// If closedSize > 0, it's a close trade
		// Side: Buy = long direction, Sell = short direction
		orderAction := "open_long"
		if closedSize > 0 {
			// This is a close trade
			if strings.ToLower(side) == "sell" {
				orderAction = "close_long" // Selling to close a long
			} else {
				orderAction = "close_short" // Buying to close a short
			}
		} else {
			// This is an open trade
			if strings.ToLower(side) == "buy" {
				orderAction = "open_long"
			} else {
				orderAction = "open_short"
			}
		}

		trade := BybitTrade{
			Symbol:      symbol,
			OrderID:     orderID,
			ExecID:      execID,
			Side:        side,
			ExecPrice:   execPrice,
			ExecQty:     execQty,
			ExecFee:     execFee,
			ExecTime:    execTime,
			IsMaker:     isMaker,
			OrderType:   orderType,
			ClosedSize:  closedSize,
			ClosedPnL:   closedPnl,
			OrderAction: orderAction,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// SyncOrdersFromBybit syncs Bybit exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("bybit")
func (t *BybitTrader) SyncOrdersFromBybit(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Bybit trades from: %s", startTime.Format(time.RFC3339))

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 1000)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from Bybit", len(trades))

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
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.ExecID)
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
			ExchangeOrderID: trade.ExecID, // Use ExecID as unique identifier
			Symbol:          symbol,
			Side:            side,
			PositionSide:    "BOTH", // Bybit uses one-way position mode
			Type:            trade.OrderType,
			OrderAction:     trade.OrderAction,
			Quantity:        trade.ExecQty,
			Price:           trade.ExecPrice,
			Status:          "FILLED",
			FilledQuantity:  trade.ExecQty,
			AvgFillPrice:    trade.ExecPrice,
			Commission:      trade.ExecFee,
			FilledAt:        execTimeMs,
			CreatedAt:       execTimeMs,
			UpdatedAt:       execTimeMs,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.ExecID, err)
			continue
		}

		// Create fill record - use UTC time
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.OrderID,
			ExchangeTradeID: trade.ExecID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.ExecPrice,
			Quantity:        trade.ExecQty,
			QuoteQuantity:   trade.ExecPrice * trade.ExecQty,
			Commission:      trade.ExecFee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.ClosedPnL,
			IsMaker:         trade.IsMaker,
			CreatedAt:       execTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.ExecID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, trade.OrderAction,
			trade.ExecQty, trade.ExecPrice, trade.ExecFee, trade.ClosedPnL,
			execTimeMs, trade.ExecID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.ExecID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.ExecID, trade.OrderAction, trade.ExecQty)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s",
			trade.ExecID, symbol, side, trade.ExecQty, trade.ExecPrice, trade.ClosedPnL, trade.ExecFee, trade.OrderAction)
	}

	logger.Infof("✅ Bybit order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for Bybit
func (t *BybitTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromBybit(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Bybit order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Bybit order sync started (interval: %v)", interval)
}
