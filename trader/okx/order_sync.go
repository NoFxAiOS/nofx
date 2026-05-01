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

func protectionReasonFromTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	switch {
	case strings.Contains(tag, "break_even"):
		return "break_even_stop"
	case strings.Contains(tag, "native_trailing"):
		return "native_trailing"
	case strings.Contains(tag, "managed_drawdown"):
		return "managed_drawdown"
	case strings.Contains(tag, "ladder_tp"):
		return "ladder_tp"
	case strings.Contains(tag, "ladder_sl"):
		return "ladder_sl"
	case strings.Contains(tag, "full_tp"):
		return "full_tp"
	case strings.Contains(tag, "full_sl"):
		return "full_sl"
	case strings.Contains(tag, "fallback_maxloss"):
		return "fallback_maxloss_sl"
	}
	return ""
}

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
	Tag         string
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
			Tag:         fill.Tag,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

func (t *OKXTrader) SyncOpenProtectionOrdersToStore(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil || st.Order() == nil {
		return nil
	}
	state, _ := st.LoadDynamicProtectionState()
	byAlgoID := map[string]store.DynamicProtectionRecord{}
	if state != nil {
		for _, record := range state.Records {
			if record.TraderID != traderID || record.ExchangeID != exchangeID || record.ExchangeOrderID == "" {
				continue
			}
			byAlgoID[record.ExchangeOrderID] = record
		}
	}
	positions, _ := t.GetPositions()
	activeQty := func(symbol, side string) float64 {
		for _, pos := range positions {
			if fmt.Sprint(pos["symbol"]) != symbol || !strings.EqualFold(fmt.Sprint(pos["side"]), side) {
				continue
			}
			if q, ok := pos["positionAmt"].(float64); ok {
				if q < 0 {
					return -q
				}
				return q
			}
		}
		return 0
	}
	for _, query := range []struct{ ordType, reason string }{{"move_order_stop", "native_trailing"}, {"conditional", ""}} {
		// OKX pending algo API requires instId. Use currently active symbols plus symbols from dynamic records.
		symbolSet := map[string]struct{}{}
		for _, pos := range positions {
			if sym := fmt.Sprint(pos["symbol"]); sym != "" {
				symbolSet[sym] = struct{}{}
			}
		}
		for _, record := range byAlgoID {
			if record.Symbol != "" {
				symbolSet[record.Symbol] = struct{}{}
			}
		}
		for symbol := range symbolSet {
			instId := t.convertSymbol(symbol)
			path := fmt.Sprintf("%s?instType=SWAP&instId=%s&ordType=%s", okxAlgoPendingPath, instId, query.ordType)
			data, err := t.doRequest("GET", path, nil)
			if err != nil {
				return err
			}
			var orders []struct {
				AlgoID        string `json:"algoId"`
				InstID        string `json:"instId"`
				Side          string `json:"side"`
				PosSide       string `json:"posSide"`
				OrdType       string `json:"ordType"`
				Sz            string `json:"sz"`
				TriggerPx     string `json:"triggerPx"`
				SlTriggerPx   string `json:"slTriggerPx"`
				TpTriggerPx   string `json:"tpTriggerPx"`
				ActivePx      string `json:"activePx"`
				CallbackRatio string `json:"callbackRatio"`
				Tag           string `json:"tag"`
			}
			if err := json.Unmarshal(data, &orders); err != nil {
				return err
			}
			for _, order := range orders {
				if order.AlgoID == "" {
					continue
				}
				reason := query.reason
				if tagged := protectionReasonFromTag(order.Tag); tagged != "" {
					reason = tagged
				}
				if reason == "" {
					if strings.EqualFold(order.OrdType, "move_order_stop") {
						reason = "native_trailing"
					} else if order.TpTriggerPx != "" {
						reason = "full_tp"
					} else {
						reason = "full_sl"
					}
				}
				posSide := strings.ToUpper(order.PosSide)
				if posSide == "" {
					if strings.EqualFold(order.Side, "buy") {
						posSide = "SHORT"
					} else {
						posSide = "LONG"
					}
				}
				qty, _ := strconv.ParseFloat(order.Sz, 64)
				if inst, err := t.getInstrument(symbol); err == nil && inst.CtVal > 0 {
					qty *= inst.CtVal
				}
				if qty <= 0 {
					qty = activeQty(symbol, posSide)
				}
				activation, _ := strconv.ParseFloat(firstNonEmpty(order.ActivePx, order.TriggerPx, order.SlTriggerPx, order.TpTriggerPx), 64)
				callback, _ := strconv.ParseFloat(order.CallbackRatio, 64)
				// OKX callbackRatio is reported in percentage units; store/order runtime
				// comparisons use decimal ratios. Keep persisted metadata consistent with
				// GetOpenOrders so synced native trailing orders can be matched reliably.
				callbackRate := normalizeOKXCallbackRatio(callback)
				t.recordProtectionOrder(st, traderID, exchangeID, exchangeType, symbol, order.Side, posSide, order.AlgoID, reason, activation, callbackRate, qty)
			}
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (t *OKXTrader) recordProtectionOrder(st *store.Store, traderID string, exchangeID string, exchangeType string, symbol string, side string, positionSide string, algoID string, reason string, activationPrice float64, callbackRatio float64, quantity float64) {
	if st == nil || st.Order() == nil || algoID == "" {
		return
	}
	orderSide := "SELL"
	if strings.EqualFold(positionSide, "SHORT") {
		orderSide = "BUY"
	}
	orderType := "ALGO"
	if reason == "native_trailing" || strings.Contains(reason, "trailing") {
		orderType = "TRAILING_STOP_MARKET"
	}
	ord := &store.TraderOrder{
		TraderID:        traderID,
		ExchangeID:      exchangeID,
		ExchangeType:    exchangeType,
		ExchangeOrderID: algoID,
		ClientOrderID:   okxReasonTag(reason),
		Symbol:          market.Normalize(symbol),
		Side:            orderSide,
		PositionSide:    strings.ToUpper(positionSide),
		Type:            orderType,
		OrderAction:     reason,
		Quantity:        quantity,
		StopPrice:       activationPrice,
		Status:          "NEW",
		ReduceOnly:      true,
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}
	if err := st.Order().CreateOrder(ord); err != nil {
		logger.Infof("  ⚠️ Failed to record protection algo order %s %s: %v", symbol, algoID, err)
	}
}

// SyncOrdersFromOKX syncs OKX exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("okx")
func (t *OKXTrader) SyncOrdersFromOKX(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	return t.SyncOrdersFromOKXWithFullCloseHandler(traderID, exchangeID, exchangeType, st, nil)
}

func (t *OKXTrader) SyncOrdersFromOKXWithFullCloseHandler(traderID string, exchangeID string, exchangeType string, st *store.Store, onFullClose func(symbol, side string)) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}
	if err := t.SyncOpenProtectionOrdersToStore(traderID, exchangeID, exchangeType, st); err != nil {
		logger.Infof("  ⚠️ Failed to sync OKX open protection orders before fill attribution: %v", err)
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
		canonicalAction := trade.OrderAction
		requestedReason := canonicalAction
		parentOrderID := strings.TrimSpace(trade.OrderID)
		if canonicalAction == "close_long" || canonicalAction == "close_short" {
			if reason := protectionReasonFromTag(trade.Tag); reason != "" {
				requestedReason = reason
			} else if parentOrderID != "" {
				if parentOrder, err := orderStore.GetOrderByExchangeID(exchangeID, parentOrderID); err == nil && parentOrder != nil {
					parentReason := strings.ToLower(strings.TrimSpace(parentOrder.OrderAction + " " + parentOrder.ClientOrderID))
					switch {
					case strings.Contains(parentReason, "native_trailing") || strings.Contains(parentReason, "trailing"):
						requestedReason = "native_trailing"
					case strings.Contains(parentReason, "break_even"):
						requestedReason = "break_even_stop"
					case strings.Contains(parentReason, "managed_drawdown"):
						requestedReason = "managed_drawdown"
					case strings.Contains(parentReason, "ladder_tp"):
						requestedReason = "ladder_tp"
					case strings.Contains(parentReason, "ladder_sl"):
						requestedReason = "ladder_sl"
					case strings.Contains(parentReason, "full_tp"):
						requestedReason = "full_tp"
					case strings.Contains(parentReason, "full_sl") || strings.Contains(parentReason, "fallback_maxloss"):
						requestedReason = "full_sl"
					}
				}
			}
		}
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			ExchangeOrderID: trade.TradeID,
			ClientOrderID:   trade.Tag,
			ParentOrderID:   parentOrderID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            trade.OrderType,
			OrderAction:     requestedReason,
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
			ParentOrderID:   parentOrderID,
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
		preClosePosition, _ := positionStore.GetOpenPositionBySymbol(traderID, symbol, positionSide)
		preCloseQty := 0.0
		if preClosePosition != nil {
			preCloseQty = preClosePosition.Quantity
		}
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, canonicalAction,
			trade.FillQtyBase, trade.FillPrice, trade.Fee, 0, // No per-trade PnL from OKX
			execTimeMs, trade.TradeID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			store.AttachSyncedOrderToPosition(st, orderStore, positionStore, orderRecord, traderID, symbol, positionSide, canonicalAction, trade.TradeID)
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, canonicalAction, trade.FillQtyBase)
			if onFullClose != nil && strings.HasPrefix(canonicalAction, "close_") && preClosePosition != nil && trade.FillQtyBase >= preCloseQty-0.0001 {
				onFullClose(symbol, positionSide)
			}
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f fee=%.6f action=%s source=%s",
			trade.TradeID, trade.Symbol, side, trade.FillQtyBase, trade.FillPrice, trade.Fee, trade.OrderAction, requestedReason)
	}

	logger.Infof("✅ OKX order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for OKX
func (t *OKXTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	t.StartOrderSyncWithFullCloseHandler(traderID, exchangeID, exchangeType, st, interval, nil)
}

func (t *OKXTrader) StartOrderSyncWithFullCloseHandler(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration, onFullClose func(symbol, side string)) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromOKXWithFullCloseHandler(traderID, exchangeID, exchangeType, st, onFullClose); err != nil {
				logger.Infof("⚠️  OKX order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 OKX order sync started (interval: %v)", interval)
}
