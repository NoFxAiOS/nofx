package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"nofx/logger"
	"nofx/store"
	"net/http"
	"sort"
	"strings"
	"time"
)

// LighterOrderHistory order history record
type LighterOrderHistory struct {
	OrderID       string    `json:"order_id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`           // "buy" or "sell"
	Type          string    `json:"type"`           // "limit" or "market"
	Price         string    `json:"price"`
	Size          string    `json:"size"`
	FilledSize    string    `json:"filled_size"`
	Status        string    `json:"status"`         // "filled", "cancelled", etc.
	CreatedAt     int64     `json:"created_at"`
	UpdatedAt     int64     `json:"updated_at"`
	FilledAt      int64     `json:"filled_at"`
}

// SyncOrdersFromLighter syncs Lighter exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("lighter")
func (t *LighterTraderV2) SyncOrdersFromLighter(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Ensure we have account index
	if t.accountIndex == 0 {
		if err := t.initializeAccount(); err != nil {
			return fmt.Errorf("failed to get account index: %w", err)
		}
	}

	// Get recent orders (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour).Unix()
	endpoint := fmt.Sprintf("%s/api/v1/orders?account_index=%d&start_time=%d&limit=100",
		t.baseURL, t.accountIndex, startTime)

	logger.Infof("🔄 Syncing Lighter orders from: %s", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}
	req.Header.Set("Authorization", t.authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get orders: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Don't spam logs for 404 errors (API endpoint might not be available)
		if resp.StatusCode != http.StatusNotFound {
			logger.Infof("⚠️  Lighter orders API returned %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse response
	var apiResp struct {
		Code   int                    `json:"code"`
		Orders []LighterOrderHistory  `json:"orders"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		logger.Infof("⚠️  Failed to parse orders response: %v, body: %s", err, string(body))
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return fmt.Errorf("API returned code %d", apiResp.Code)
	}

	logger.Infof("📥 Received %d orders from Lighter", len(apiResp.Orders))

	// Sort orders by filled_at ASC (oldest first) for proper position building
	sort.Slice(apiResp.Orders, func(i, j int) bool {
		return apiResp.Orders[i].FilledAt < apiResp.Orders[j].FilledAt
	})

	// Process orders one by one (no transaction to avoid deadlock)
	orderStore := st.Order()
	positionStore := st.Position()
	posBuilder := store.NewPositionBuilder(positionStore)

	// Get current open positions to help determine action for each order
	openPositions, _ := positionStore.GetOpenPositions(traderID)

	syncedCount := 0
	for _, order := range apiResp.Orders {
		// Only sync filled orders
		if order.Status != "filled" {
			continue
		}

		// Check if order already exists (use exchangeID which is UUID, not exchange type)
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, order.OrderID)
		if err == nil && existing != nil {
			continue // Order already exists, skip
		}

		// Parse price and quantity
		price, _ := parseFloat(order.Price)
		size, _ := parseFloat(order.Size)
		filledSize, _ := parseFloat(order.FilledSize)

		if filledSize == 0 {
			filledSize = size
		}

		// Determine order action based on existing positions
		// Lighter can have both LONG and SHORT positions simultaneously
		var positionSide, orderAction, side string
		symbol := order.Symbol

		if order.Side == "buy" {
			side = "BUY"

			// Check if we have an open SHORT position for this symbol
			hasShort := false
			for _, pos := range openPositions {
				if pos.Symbol == symbol && pos.Side == "SHORT" && pos.Status == "OPEN" {
					hasShort = true
					break
				}
			}

			if hasShort {
				positionSide = "SHORT"
				orderAction = "close_short"
			} else {
				positionSide = "LONG"
				orderAction = "open_long"
			}
		} else {
			side = "SELL"

			// Check if we have an open LONG position
			hasLong := false
			for _, pos := range openPositions {
				if pos.Symbol == symbol && pos.Side == "LONG" && pos.Status == "OPEN" {
					hasLong = true
					break
				}
			}

			if hasLong {
				positionSide = "LONG"
				orderAction = "close_long"
			} else {
				positionSide = "SHORT"
				orderAction = "open_short"
			}
		}

		// Estimate fee
		fee := price * filledSize * 0.0004

		// Create order record
		filledAt := time.Unix(order.FilledAt, 0)
		if order.FilledAt == 0 {
			filledAt = time.Unix(order.UpdatedAt, 0)
		}

		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			ExchangeOrderID: order.OrderID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        filledSize,
			Price:           price,
			Status:          "FILLED",
			FilledQuantity:  filledSize,
			AvgFillPrice:    price,
			Commission:      fee,
			FilledAt:        filledAt,
			CreatedAt:       time.Unix(order.CreatedAt, 0),
			UpdatedAt:       time.Unix(order.UpdatedAt, 0),
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync order %s: %v", order.OrderID, err)
			continue
		}

		// Create fill record
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			OrderID:         orderRecord.ID,
			ExchangeOrderID: order.OrderID,
			ExchangeTradeID: fmt.Sprintf("%s-%d", order.OrderID, time.Now().UnixNano()),
			Symbol:          symbol,
			Side:            side,
			Price:           price,
			Quantity:        filledSize,
			QuoteQuantity:   price * filledSize,
			Commission:      fee,
			CommissionAsset: "USDT",
			RealizedPnL:     0,
			IsMaker:         order.Type == "limit",
			CreatedAt:       filledAt,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for order %s: %v", order.OrderID, err)
		}

		// Calculate PnL for close orders
		var realizedPnL float64
		if strings.HasPrefix(orderAction, "close_") {
			// Get the open position to calculate PnL
			openPos, _ := positionStore.GetOpenPositionBySymbol(traderID, symbol, positionSide)
			if openPos != nil {
				if positionSide == "LONG" {
					realizedPnL = (price - openPos.EntryPrice) * filledSize
				} else {
					realizedPnL = (openPos.EntryPrice - price) * filledSize
				}
				realizedPnL -= fee
			}
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, orderAction,
			filledSize, price, fee, realizedPnL,
			filledAt, order.OrderID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for order %s: %v", order.OrderID, err)
		}

		// Update openPositions list dynamically
		if strings.HasPrefix(orderAction, "open_") {
			// Add to openPositions (approximate)
			openPositions = append(openPositions, &store.TraderPosition{
				Symbol: symbol,
				Side:   positionSide,
				Status: "OPEN",
			})
		} else if strings.HasPrefix(orderAction, "close_") {
			// Remove from openPositions (approximate)
			for i, pos := range openPositions {
				if pos.Symbol == symbol && pos.Side == positionSide && pos.Status == "OPEN" {
					openPositions = append(openPositions[:i], openPositions[i+1:]...)
					break
				}
			}
		}

		syncedCount++
		logger.Infof("  ✅ Synced order: %s %s %s qty=%.6f price=%.6f", order.OrderID, symbol, side, filledSize, price)
	}

	logger.Infof("✅ Order sync completed: %d new orders synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task
func (t *LighterTraderV2) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromLighter(traderID, exchangeID, exchangeType, st); err != nil {
				// Only log non-404 errors to reduce log spam
				if !strings.Contains(err.Error(), "status 404") {
					logger.Infof("⚠️  Order sync failed: %v", err)
				}
			}
		}
	}()
	logger.Infof("🔄 Lighter order+position sync started (interval: %v)", interval)
}
