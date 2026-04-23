package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"nofx/telemetry"
	"strings"
	"time"
)

// saveEquitySnapshot saves equity snapshot independently (for drawing profit curve, decoupled from AI decision)
func (at *AutoTrader) saveEquitySnapshot(ctx *kernel.Context) {
	if at.store == nil || ctx == nil {
		return
	}

	snapshot := &store.EquitySnapshot{
		TraderID:      at.id,
		Timestamp:     time.Now().UTC(),
		TotalEquity:   ctx.Account.TotalEquity,
		Balance:       ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		UnrealizedPnL: ctx.Account.UnrealizedPnL,
		PositionCount: ctx.Account.PositionCount,
		MarginUsedPct: ctx.Account.MarginUsedPct,
	}

	if err := at.store.Equity().Save(snapshot); err != nil {
		logger.Infof("⚠️ Failed to save equity snapshot: %v", err)
	}
}

// saveDecision saves AI decision log to database (only records AI input/output, for debugging)
func (at *AutoTrader) saveDecision(record *store.DecisionRecord) error {
	if at.store == nil {
		return nil
	}

	at.cycleNumber++
	record.CycleNumber = at.cycleNumber
	record.TraderID = at.id

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	if err := at.store.Decision().LogDecision(record); err != nil {
		logger.Infof("⚠️ Failed to save decision record: %v", err)
		return err
	}

	logger.Infof("📝 Decision record saved: trader=%s, cycle=%d", at.id, at.cycleNumber)
	return nil
}

// GetStatus gets system status (for API)
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	at.isRunningMutex.RLock()
	isRunning := at.isRunning
	at.isRunningMutex.RUnlock()

	result := map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      isRunning,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}

	// Add strategy info
	if at.config.StrategyConfig != nil {
		result["strategy_type"] = at.config.StrategyConfig.StrategyType
		if at.config.StrategyConfig.GridConfig != nil {
			result["grid_symbol"] = at.config.StrategyConfig.GridConfig.Symbol
		}
	}

	return result
}

// GetAccountInfo gets account information (for API)
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get account fields
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0
	totalEquity := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Use totalEquity directly if provided by trader (more accurate)
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
	} else {
		// Fallback: Total Equity = Wallet balance + Unrealized profit
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}

	// Get positions to calculate total margin
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// Verify unrealized P&L consistency (API value vs calculated from positions)
	// Note: Lighter API may return 0 for unrealized PnL, this is a known limitation
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > 5.0 { // Only warn if difference is significant (> 5 USDT)
		logger.Infof("⚠️ Unrealized P&L inconsistency (Lighter API limitation): API=%.4f, Calculated=%.4f, Diff=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		logger.Infof("⚠️ Initial Balance abnormal: %.2f, cannot calculate P&L percentage", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// Core fields
		"total_equity":      totalEquity,           // Account equity = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // Wallet balance (excluding unrealized P&L)
		"unrealized_profit": totalUnrealizedProfit, // Unrealized P&L (official value from exchange API)
		"available_balance": availableBalance,      // Available balance

		// P&L statistics
		"total_pnl":       totalPnL,          // Total P&L = equity - initial
		"total_pnl_pct":   totalPnLPct,       // Total P&L percentage
		"initial_balance": at.initialBalance, // Initial balance
		"daily_pnl":       at.dailyPnL,       // Daily P&L

		// Position information
		"position_count":  len(positions),  // Position count
		"margin_used":     totalMarginUsed, // Margin used
		"margin_used_pct": marginUsedPct,   // Margin usage rate
	}, nil
}

// GetPositions gets position list (for API)
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// Calculate margin used
		marginUsed := (quantity * markPrice) / float64(leverage)

		// Calculate P&L percentage (based on margin)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		openOrders, _ := at.trader.GetOpenOrders(symbol)
		positionSideUpper := strings.ToUpper(side)
		openOrders = at.enrichProtectionOrders(openOrders)
		protectionRuntime := at.buildPositionProtectionRuntime(symbol, side, quantity, entryPrice, openOrders)
		entryDecisionCycle := 0
		var entryReviewSummary map[string]interface{}
		var entryStructureAudit map[string]interface{}
		if at.store != nil {
			if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, positionSideUpper); err == nil && openPos != nil {
				entryDecisionCycle = openPos.EntryDecisionCycle
				if decisionStore := at.store.Decision(); decisionStore != nil {
					if record, err := decisionStore.GetRecordByCycle(at.id, openPos.EntryDecisionCycle); err == nil && record != nil {
						candidate := findMatchedDecisionAction(record, symbol, sideToOpenAction(positionSideUpper))
						decoded := extractDecisionReviewMap(func() *store.DecisionActionReviewContext {
							if candidate != nil {
								return candidate.ReviewContext
							}
							return nil
						}())
						entryReviewSummary = buildEntryReviewSummaryFromDecisionReview(decoded)
					}
				}
				if traderRecord, err := at.store.Trader().GetByID(at.id); err == nil && traderRecord != nil {
					if fullCfg, err := at.store.Trader().GetFullConfig(traderRecord.UserID, at.id); err == nil && fullCfg != nil && fullCfg.Strategy != nil {
						if parsed, err := fullCfg.Strategy.ParseConfig(); err == nil && parsed != nil {
							es := parsed.EntryStructure
							entryStructureAudit = map[string]interface{}{
								"audit_primary_timeframe":             es.AuditPrimaryTimeframe,
								"audit_adjacent_timeframes":           es.AuditAdjacentTimeframes,
								"audit_support_resistance":            es.AuditSupportResistance,
								"audit_structural_anchors":            es.AuditStructuralAnchors,
								"audit_fibonacci":                     es.AuditFibonacci,
								"require_invalidation_target_linkage": es.RequireInvalidationTargetLinkage,
							}
						}
					}
				}
			}
		}

		result = append(result, map[string]interface{}{
			"symbol":                    symbol,
			"side":                      side,
			"entry_price":               entryPrice,
			"mark_price":                markPrice,
			"quantity":                  quantity,
			"leverage":                  leverage,
			"unrealized_pnl":            unrealizedPnl,
			"unrealized_pnl_pct":        pnlPct,
			"liquidation_price":         liquidationPrice,
			"margin_used":               marginUsed,
			"protection_state":          at.getProtectionState(symbol, side),
			"break_even_state":          at.getBreakEvenState(symbol, side),
			"drawdown_execution_mode":   at.getDrawdownExecutionMode(symbol, side),
			"break_even_execution_mode": at.getBreakEvenExecutionMode(symbol, side),
			"protection_runtime":        protectionRuntime,
			"position_side":             positionSideUpper,
			"entry_decision_cycle":      entryDecisionCycle,
			"entry_review_summary":      entryReviewSummary,
			"entry_structure_audit":     entryStructureAudit,
		})
	}

	return result, nil
}

// recordAndConfirmOrder polls order status for actual fill data and records position
// action: open_long, open_short, close_long, close_short
// entryPrice: entry price when closing (0 when opening)
func (at *AutoTrader) recordAndConfirmOrder(orderResult map[string]interface{}, symbol, action string, quantity float64, price float64, leverage int, entryPrice float64) {
	if at.store == nil {
		return
	}

	// Get order ID (supports multiple types)
	var orderID string
	switch v := orderResult["orderId"].(type) {
	case int64:
		orderID = fmt.Sprintf("%d", v)
	case float64:
		orderID = fmt.Sprintf("%.0f", v)
	case string:
		orderID = v
	default:
		orderID = fmt.Sprintf("%v", v)
	}

	if orderID == "" || orderID == "0" {
		logger.Infof("  ⚠️ Order ID is empty, skipping record")
		return
	}

	// Determine positionSide
	var positionSide string
	switch action {
	case "open_long", "close_long":
		positionSide = "LONG"
	case "open_short", "close_short":
		positionSide = "SHORT"
	}

	var actualPrice = price
	var actualQty = quantity
	var fee float64

	// Exchanges with OrderSync: Skip immediate order recording, let OrderSync handle it
	// This ensures accurate data from GetTrades API and avoids duplicate records
	switch at.exchange {
	case "binance", "lighter", "hyperliquid", "bybit", "okx", "bitget", "aster", "kucoin", "gate":
		logger.Infof("  📝 Order submitted (id: %s), will be synced by OrderSync", orderID)
		return
	}

	// For exchanges without OrderSync (e.g., Binance): record immediately and poll for fill data
	orderRecord := at.createOrderRecord(orderID, symbol, action, positionSide, quantity, price, leverage)
	if err := at.store.Order().CreateOrder(orderRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record order: %v", err)
	} else {
		logger.Infof("  📝 Order recorded: %s [%s] %s", orderID, action, symbol)
	}

	// Wait for order to be filled and get actual fill data
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		status, err := at.trader.GetOrderStatus(symbol, orderID)
		if err == nil {
			statusStr, _ := status["status"].(string)
			if statusStr == "FILLED" {
				// Get actual fill price
				if avgPrice, ok := status["avgPrice"].(float64); ok && avgPrice > 0 {
					actualPrice = avgPrice
				}
				// Get actual executed quantity
				if execQty, ok := status["executedQty"].(float64); ok && execQty > 0 {
					actualQty = execQty
				}
				// Get commission/fee
				if commission, ok := status["commission"].(float64); ok {
					fee = commission
				}
				logger.Infof("  ✅ Order filled: avgPrice=%.6f, qty=%.6f, fee=%.6f", actualPrice, actualQty, fee)

				// Update order status to FILLED
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, "FILLED", actualQty, actualPrice, fee); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}

				// Record fill details
				at.recordOrderFill(orderRecord.ID, orderID, symbol, action, actualPrice, actualQty, fee)
				break
			} else if statusStr == "CANCELED" || statusStr == "EXPIRED" || statusStr == "REJECTED" {
				logger.Infof("  ⚠️ Order %s, skipping position record", statusStr)
				// Update order status
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, statusStr, 0, 0, 0); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Normalize symbol for position record consistency
	normalizedSymbolForPosition := market.Normalize(symbol)

	logger.Infof("  📝 Recording position (ID: %s, action: %s, price: %.6f, qty: %.6f, fee: %.4f)",
		orderID, action, actualPrice, actualQty, fee)

	// Record position change with actual fill data (use normalized symbol)
	at.recordPositionChange(orderID, normalizedSymbolForPosition, positionSide, action, actualQty, actualPrice, leverage, entryPrice, fee)

	// Send anonymous trade statistics for experience improvement (async, non-blocking)
	// This helps us understand overall product usage across all deployments
	telemetry.TrackTrade(telemetry.TradeEvent{
		Exchange:  at.exchange,
		TradeType: action,
		Symbol:    symbol,
		AmountUSD: actualPrice * actualQty,
		Leverage:  leverage,
		UserID:    at.userID,
		TraderID:  at.id,
	})
}

// recordPositionChange records position change (create record on open, update record on close)
func (at *AutoTrader) recordPositionChange(orderID, symbol, side, action string, quantity, price float64, leverage int, entryPrice float64, fee float64) {
	if at.store == nil {
		return
	}

	switch action {
	case "open_long", "open_short":
		// Open position: create new position record
		nowMs := time.Now().UTC().UnixMilli()
		pos := &store.TraderPosition{
			TraderID:           at.id,
			ExchangeID:         at.exchangeID, // Exchange account UUID
			ExchangeType:       at.exchange,   // Exchange type: binance/bybit/okx/etc
			Symbol:             symbol,
			Side:               side, // LONG or SHORT
			Quantity:           quantity,
			EntryPrice:         price,
			EntryOrderID:       orderID,
			EntryDecisionCycle: at.cycleNumber,
			EntryTime:          nowMs,
			Leverage:           leverage,
			Status:             "OPEN",
			CreatedAt:          nowMs,
			UpdatedAt:          nowMs,
		}
		if err := at.store.Position().Create(pos); err != nil {
			logger.Infof("  ⚠️ Failed to record position: %v", err)
		} else {
			logger.Infof("  📊 Position recorded [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}

	case "close_long", "close_short":
		// Close position using PositionBuilder for consistent handling
		// PositionBuilder will handle both cases:
		// 1. If open position exists: close it properly
		// 2. If no open position (e.g., table cleared): create a closed position record
		posBuilder := store.NewPositionBuilder(at.store.Position())
		if err := posBuilder.ProcessTrade(
			at.id, at.exchangeID, at.exchange,
			symbol, side, action,
			quantity, price, fee, 0, // realizedPnL will be calculated
			time.Now().UTC().UnixMilli(), orderID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to process close position: %v", err)
		} else {
			closeReason := action
			if action == "close_long" {
				closeReason = "ai_close_long"
			} else if action == "close_short" {
				closeReason = "ai_close_short"
			}
			_ = at.store.Position().UpdateCloseReasonByExitOrderID(at.id, orderID, closeReason)
			_ = at.store.PositionClose().UpdateReasonByOrderID(at.id, orderID, closeReason, closeReason)
			logger.Infof("  ✅ Position closed [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}
	}
}

// createOrderRecord creates an order record struct from order details
func (at *AutoTrader) createOrderRecord(orderID, symbol, action, positionSide string, quantity, price float64, leverage int) *store.TraderOrder {
	// Determine order type (market for auto trader)
	orderType := "MARKET"

	// Determine side (BUY/SELL)
	var side string
	switch action {
	case "open_long", "close_short":
		side = "BUY"
	case "open_short", "close_long":
		side = "SELL"
	}

	// Use action as orderAction directly (keep lowercase format)
	orderAction := action

	// Determine if it's a reduce only order
	reduceOnly := (action == "close_long" || action == "close_short")

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	return &store.TraderOrder{
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeType:    at.exchange,
		ExchangeOrderID: orderID,
		Symbol:          normalizedSymbol,
		Side:            side,
		PositionSide:    positionSide,
		Type:            orderType,
		TimeInForce:     "GTC",
		Quantity:        quantity,
		Price:           price,
		Status:          "NEW",
		FilledQuantity:  0,
		AvgFillPrice:    0,
		Commission:      0,
		CommissionAsset: "USDT",
		Leverage:        leverage,
		ReduceOnly:      reduceOnly,
		ClosePosition:   reduceOnly,
		OrderAction:     orderAction,
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}
}

// recordOrderFill records order fill/trade details
func (at *AutoTrader) recordOrderFill(orderRecordID int64, exchangeOrderID, symbol, action string, price, quantity, fee float64) {
	if at.store == nil {
		return
	}

	// Determine side (BUY/SELL)
	var side string
	switch action {
	case "open_long", "close_short":
		side = "BUY"
	case "open_short", "close_long":
		side = "SELL"
	}

	// Generate a simple trade ID (exchange doesn't always provide one)
	tradeID := fmt.Sprintf("%s-%d", exchangeOrderID, time.Now().UnixNano())

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	fill := &store.TraderFill{
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeType:    at.exchange,
		OrderID:         orderRecordID,
		ExchangeOrderID: exchangeOrderID,
		ExchangeTradeID: tradeID,
		Symbol:          normalizedSymbol,
		Side:            side,
		Price:           price,
		Quantity:        quantity,
		QuoteQuantity:   price * quantity,
		Commission:      fee,
		CommissionAsset: "USDT",
		RealizedPnL:     0,     // Will be calculated for close orders
		IsMaker:         false, // Market orders are usually taker
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}

	// Calculate realized PnL for close orders
	if action == "close_long" || action == "close_short" {
		// Try to get the entry price from the open position
		var positionSide string
		if action == "close_long" {
			positionSide = "LONG"
		} else {
			positionSide = "SHORT"
		}

		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, positionSide); err == nil && openPos != nil {
			if positionSide == "LONG" {
				fill.RealizedPnL = (price - openPos.EntryPrice) * quantity
			} else {
				fill.RealizedPnL = (openPos.EntryPrice - price) * quantity
			}
		}
	}

	if err := at.store.Order().CreateFill(fill); err != nil {
		logger.Infof("  ⚠️ Failed to record fill: %v", err)
	} else {
		logger.Infof("  📋 Fill recorded: %.4f @ %.6f, fee: %.4f", quantity, price, fee)
	}
}

func classifyProtectionOrderRole(order OpenOrder) string {
	kind := strings.ToUpper(order.Type)
	if strings.Contains(kind, "TRAILING") {
		return "trailing"
	}
	if looksLikeTakeProfit(order) {
		return "take_profit"
	}
	if looksLikeStopLoss(order) {
		return "stop_loss"
	}
	return "unknown"
}

func classifyProtectionOrderStatus(order OpenOrder) string {
	kind := strings.ToUpper(order.Type)
	status := strings.ToUpper(order.Status)
	if strings.Contains(kind, "TRAILING") && (order.StopPrice <= 0 && order.Price <= 0) {
		return "pending_activation"
	}
	if status == "" || status == "NEW" || status == "LIVE" || status == "OPEN" || status == "PENDING" {
		return "delegated"
	}
	return "delegated"
}

func (at *AutoTrader) enrichProtectionOrders(openOrders []OpenOrder) []OpenOrder {
	if len(openOrders) == 0 {
		return openOrders
	}
	enriched := make([]OpenOrder, 0, len(openOrders))
	for _, order := range openOrders {
		order.ProtectionRole = classifyProtectionOrderRole(order)
		order.ProtectionStatus = classifyProtectionOrderStatus(order)
		enriched = append(enriched, order)
	}
	return enriched
}

// GetOpenOrders returns open orders (pending SL/TP) from exchange
func (at *AutoTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	orders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return nil, err
	}
	return at.enrichProtectionOrders(orders), nil
}

func (at *AutoTrader) buildPositionProtectionRuntime(symbol, side string, quantity, entryPrice float64, openOrders []OpenOrder) map[string]interface{} {
	positionSide := strings.ToUpper(side)
	currentPnLPct := 0.0
	peakPnLPct := 0.0
	drawdownPct := 0.0
	markPrice, _ := at.getPositionMarkPrice(symbol, side)
	if entryPrice > 0 && markPrice > 0 {
		currentPnLPct = calculatePositionPnLPct(side, entryPrice, markPrice)
		peakPnLPct = currentPnLPct
		at.peakPnLCacheMutex.RLock()
		if peak, ok := at.peakPnLCache[positionKey(symbol, side)]; ok && peak > peakPnLPct {
			peakPnLPct = peak
		}
		at.peakPnLCacheMutex.RUnlock()
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}
	}

	be := at.getActiveBreakEvenConfigForPlan(nil)
	breakEvenTrigger := 0.0
	breakEvenOffset := 0.0
	breakEvenSuppressedByRunner := at.isBreakEvenSuppressedByRunner(symbol, side)
	nextBreakEvenGap := 0.0
	breakEvenSource := at.getBreakEvenConfigSource(symbol, side)
	if be == nil {
		breakEvenSource = "none"
	}
	if be != nil {
		breakEvenTrigger = be.TriggerValue
		breakEvenOffset = be.OffsetPct
		nextBreakEvenGap = be.TriggerValue - currentPnLPct
		if nextBreakEvenGap < 0 {
			nextBreakEvenGap = 0
		}
	}
	if breakEvenSuppressedByRunner {
		nextBreakEvenGap = 0
	}

	drawdownRules := at.getActiveDrawdownRules()
	drawdownSource := at.getDrawdownConfigSource(symbol, side)
	runnerState := at.getDrawdownRunnerState(symbol, side)
	drawdownCfg := store.DrawdownTakeProfitConfig{}
	if at.config.StrategyConfig != nil {
		drawdownCfg = at.config.StrategyConfig.Protection.DrawdownTakeProfit
	}
	armRules := at.getDrawdownArmRules(currentPnLPct, drawdownRules)
	currentStageMinProfit := 0.0
	currentStageRuleCount := 0
	if len(armRules) > 0 {
		currentStageMinProfit = armRules[0].MinProfitPct
		currentStageRuleCount = len(armRules)
	}

	activeOrders := make([]map[string]interface{}, 0)
	trailingOrders := make([]map[string]interface{}, 0)
	liveTrailingTriggerPrice := 0.0
	liveTrailingCallbackRate := 0.0
	liveBreakEvenStopPrice := 0.0
	breakEvenOrderDetected := false
	ladderStopCount := 0
	ladderTakeProfitCount := 0
	fullStopCount := 0
	fullTakeProfitCount := 0
	fallbackStopCount := 0
	for _, order := range openOrders {
		if order.PositionSide != "" && !strings.EqualFold(order.PositionSide, positionSide) {
			continue
		}
		triggerPrice := order.StopPrice
		if triggerPrice <= 0 {
			triggerPrice = order.Price
		}
		clientOrderID := strings.ToLower(strings.TrimSpace(order.ClientOrderID))
		if !breakEvenOrderDetected && looksLikeStopLoss(order) {
			if strings.Contains(clientOrderID, "break_even") || strings.Contains(clientOrderID, "breakeven") {
				breakEvenOrderDetected = true
				if triggerPrice > 0 {
					liveBreakEvenStopPrice = triggerPrice
				}
			}
		}
		if strings.Contains(strings.ToUpper(order.Type), "TRAILING") && triggerPrice > 0 {
			liveTrailingTriggerPrice = triggerPrice
			if order.CallbackRate > 0 {
				liveTrailingCallbackRate = order.CallbackRate
			}
			trailingOrders = append(trailingOrders, map[string]interface{}{
				"order_id":        order.OrderID,
				"type":            order.Type,
				"side":            order.Side,
				"position_side":   order.PositionSide,
				"trigger_price":   triggerPrice,
				"callback_rate":   order.CallbackRate,
				"quantity":        order.Quantity,
				"status":          order.Status,
				"client_order_id": order.ClientOrderID,
			})
		}
		role := strings.ToLower(strings.TrimSpace(order.ProtectionRole))
		clientOrderIDLower := strings.ToLower(strings.TrimSpace(order.ClientOrderID))
		switch role {
		case "stop_loss":
			switch {
			case strings.Contains(clientOrderIDLower, "fallback_maxloss"):
				fallbackStopCount++
			case strings.Contains(clientOrderIDLower, "ladder"):
				ladderStopCount++
			case strings.Contains(clientOrderIDLower, "full"):
				fullStopCount++
			}
		case "take_profit":
			switch {
			case strings.Contains(clientOrderIDLower, "ladder"):
				ladderTakeProfitCount++
			case strings.Contains(clientOrderIDLower, "full"):
				fullTakeProfitCount++
			}
		}
		activeOrders = append(activeOrders, map[string]interface{}{
			"order_id":          order.OrderID,
			"type":              order.Type,
			"side":              order.Side,
			"position_side":     order.PositionSide,
			"trigger_price":     triggerPrice,
			"callback_rate":     order.CallbackRate,
			"quantity":          order.Quantity,
			"status":            order.Status,
			"client_order_id":   order.ClientOrderID,
			"protection_role":   order.ProtectionRole,
			"protection_status": order.ProtectionStatus,
		})
	}

	tiers := make([]map[string]interface{}, 0)
	structureCtx := at.buildDrawdownStructureContext(symbol, side)
	currentStructureStage := ""
	currentStructureStopSource := ""
	currentStructureTargetSource := ""
	currentStructureTargetProgress := 0.0
	currentStructurePrimaryTf := ""
	currentStructureEvidence := []string{}
	currentStructureTrace := []string{}
	currentStructureHealth := "unstructured"
	currentStructureDriftReason := ""
	currentStructureDetached := false
	if drawdownCfg.Enabled && drawdownCfg.Mode == store.ProtectionModeAI && drawdownCfg.EngineMode == store.DrawdownEngineModeAI {
		stage, stopSource, targetSource := classifyAIDrawdownStage(currentPnLPct, peakPnLPct, structureCtx, side, markPrice)
		currentStructureStage = stage
		currentStructureStopSource = stopSource
		currentStructureTargetSource = targetSource
		if structureCtx != nil {
			currentStructureTargetProgress = structuralTargetProgress(side, structureCtx.Entry, structureCtx.FirstTarget, markPrice)
			currentStructurePrimaryTf = structureCtx.PrimaryTimeframe
			currentStructureEvidence = summarizeDrawdownStructureEvidence(structureCtx, side)
			currentStructureTrace = append(currentStructureTrace,
				fmt.Sprintf("tf=%s", currentStructurePrimaryTf),
				fmt.Sprintf("stage=%s", currentStructureStage),
				fmt.Sprintf("progress=%.2f", currentStructureTargetProgress),
				fmt.Sprintf("stop_source=%s", currentStructureStopSource),
				fmt.Sprintf("target_source=%s", currentStructureTargetSource),
			)
			currentStructureHealth = "aligned"
		}
	}
	if at.config.StrategyConfig != nil {
		for idx, rule := range at.config.StrategyConfig.Protection.DrawdownTakeProfit.Rules {
			rule = normalizeDrawdownRule(rule)
			if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
				continue
			}
			executionMode := at.getDrawdownExecutionMode(symbol, side)
			source := executionMode
			if executionMode == "native_partial_trailing" || executionMode == "native_trailing_full" {
				source = "native"
			} else if executionMode == "managed_partial_drawdown" {
				source = "managed"
			}
			plannedActivationPrice := 0.0
			if entryPrice > 0 {
				move := rule.MinProfitPct / 100.0
				if strings.EqualFold(side, "long") {
					plannedActivationPrice = entryPrice * (1 + move)
				} else if strings.EqualFold(side, "short") {
					plannedActivationPrice = entryPrice * (1 - move)
				}
			}
			activationPrice := plannedActivationPrice
			callbackRate := calculateProfitBasedTrailingCallbackRatio(entryPrice, side, rule.MinProfitPct, rule.MaxDrawdownPct)
			activationSource := "planned"
			callbackSource := "planned"
			plannedQty := quantity * rule.CloseRatioPct / 100.0
			if executionMode == "native_partial_trailing" || executionMode == "native_trailing_full" {
				activationSource = "request"
				callbackSource = "request"
				switch strings.ToLower(at.exchange) {
				case "binance", "bitget":
					callbackRate = callbackRate * 100.0
				}
				matchedLive := false
				for _, order := range trailingOrders {
					qtyVal, _ := order["quantity"].(float64)
					cbVal, _ := order["callback_rate"].(float64)
					trVal, _ := order["trigger_price"].(float64)
					qtyTolerance := math.Max(0.0001, plannedQty*0.1)
					callbackTolerance := 0.0002
					if strings.ToLower(at.exchange) == "binance" || strings.ToLower(at.exchange) == "bitget" {
						callbackTolerance = 0.05
					}
					if plannedQty > 0 && math.Abs(qtyVal-plannedQty) <= qtyTolerance && math.Abs(cbVal-callbackRate) <= callbackTolerance {
						if trVal > 0 {
							activationPrice = trVal
							activationSource = "exchange"
						}
						if cbVal > 0 {
							callbackRate = cbVal
							callbackSource = "exchange"
						}
						matchedLive = true
						break
					}
				}
				if !matchedLive {
					if liveTrailingTriggerPrice > 0 {
						activationPrice = liveTrailingTriggerPrice
						activationSource = "exchange"
					}
					if liveTrailingCallbackRate > 0 {
						callbackRate = liveTrailingCallbackRate
						callbackSource = "exchange"
					}
				}
			}
			tiers = append(tiers, map[string]interface{}{
				"index":                    idx + 1,
				"stage_name":               rule.StageName,
				"min_profit_pct":           rule.MinProfitPct,
				"max_drawdown_pct":         rule.MaxDrawdownPct,
				"close_ratio_pct":          rule.CloseRatioPct,
				"runner_keep_pct":          rule.RunnerKeepPct,
				"runner_stop_mode":         rule.RunnerStopMode,
				"runner_stop_source":       rule.RunnerStopSource,
				"runner_target_mode":       rule.RunnerTargetMode,
				"runner_target_source":     rule.RunnerTargetSource,
				"activation_price":         activationPrice,
				"planned_activation_price": plannedActivationPrice,
				"activation_source":        activationSource,
				"callback_rate":            callbackRate,
				"callback_source":          callbackSource,
				"planned_quantity":         quantity * rule.CloseRatioPct / 100.0,
				"source":                   source,
				"execution_mode":           executionMode,
				"is_satisfied":             currentPnLPct >= rule.MinProfitPct,
				"is_triggered":             currentPnLPct >= rule.MinProfitPct && drawdownPct >= rule.MaxDrawdownPct,
			})
		}
	}

	plannedLadderStopCount := 0
	plannedLadderTakeProfitCount := 0
	fullStopPlanned := false
	fullTakeProfitPlanned := false
	fallbackPlanned := false
	if configuredPlan, err := at.BuildConfiguredProtectionPlan(entryPrice, "open_"+strings.ToLower(side)); err == nil && configuredPlan != nil {
		plannedLadderStopCount = len(configuredPlan.StopLossOrders)
		plannedLadderTakeProfitCount = len(configuredPlan.TakeProfitOrders)
		fullStopPlanned = configuredPlan.NeedsStopLoss && configuredPlan.StopLossPrice > 0
		fullTakeProfitPlanned = configuredPlan.NeedsTakeProfit && configuredPlan.TakeProfitPrice > 0
		fallbackPlanned = configuredPlan.FallbackMaxLossPrice > 0
	}
	ladderDegradedStop := plannedLadderStopCount > 0 && ladderStopCount < plannedLadderStopCount
	ladderDegradedTakeProfit := plannedLadderTakeProfitCount > 0 && ladderTakeProfitCount < plannedLadderTakeProfitCount
	ladderDegradedToFullStop := ladderDegradedStop && fullStopCount > 0
	ladderDegradedToFullTakeProfit := ladderDegradedTakeProfit && fullTakeProfitCount > 0
	fallbackActive := fallbackStopCount > 0
	if currentStructureHealth == "aligned" {
		switch {
		case ladderDegradedStop || ladderDegradedTakeProfit:
			currentStructureHealth = "partially_degraded"
			currentStructureDriftReason = "ladder_degraded"
		case ladderDegradedToFullStop || ladderDegradedToFullTakeProfit || fallbackActive:
			currentStructureHealth = "degraded_to_full_fallback"
			currentStructureDriftReason = "degraded_to_full_fallback"
		}
	}
	if len(currentStructureEvidence) == 0 {
		currentStructureDetached = true
		if currentStructureHealth == "aligned" || currentStructureHealth == "unstructured" {
			currentStructureHealth = "structure_detached"
			if currentStructureDriftReason == "" {
				currentStructureDriftReason = "missing_structure_context"
			}
		}
	}

	return map[string]interface{}{
		"protection_state":                at.getProtectionState(symbol, side),
		"break_even_state":                at.getBreakEvenState(symbol, side),
		"break_even_suppressed_by_runner": breakEvenSuppressedByRunner,
		"drawdown_runner_mode_active":     runnerState != nil,
		"drawdown_runner_stage_name": func() string {
			if runnerState != nil {
				return runnerState.StageName
			}
			return ""
		}(),
		"drawdown_runner_keep_pct": func() float64 {
			if runnerState != nil {
				return runnerState.RunnerKeepPct
			}
			return 0
		}(),
		"drawdown_runner_stop_mode": func() string {
			if runnerState != nil {
				return runnerState.RunnerStopMode
			}
			return ""
		}(),
		"drawdown_runner_stop_source": func() string {
			if runnerState != nil {
				return runnerState.RunnerStopSource
			}
			return ""
		}(),
		"drawdown_runner_target_mode": func() string {
			if runnerState != nil {
				return runnerState.RunnerTargetMode
			}
			return ""
		}(),
		"drawdown_runner_target_source": func() string {
			if runnerState != nil {
				return runnerState.RunnerTargetSource
			}
			return ""
		}(),
		"drawdown_structure_stage":             currentStructureStage,
		"drawdown_structure_stop_source":       currentStructureStopSource,
		"drawdown_structure_target_source":     currentStructureTargetSource,
		"drawdown_structure_target_progress":   currentStructureTargetProgress,
		"drawdown_structure_primary_timeframe": currentStructurePrimaryTf,
		"drawdown_structure_evidence":          currentStructureEvidence,
		"drawdown_structure_trace":             currentStructureTrace,
		"structure_protection_health":          currentStructureHealth,
		"structure_protection_drift_reason":    currentStructureDriftReason,
		"structure_protection_detached":        currentStructureDetached,
		"drawdown_execution_mode":              at.getDrawdownExecutionMode(symbol, side),
		"drawdown_config_source":                drawdownSource,
		"break_even_execution_mode":             at.getBreakEvenExecutionMode(symbol, side),
		"current_pnl_pct":                       currentPnLPct,
		"drawdown_peak_pnl_pct":                 peakPnLPct,
		"current_drawdown_pct":                  drawdownPct,
		"current_break_even_trigger_pct":        breakEvenTrigger,
		"break_even_offset_pct":                 breakEvenOffset,
		"next_break_even_gap_pct":               nextBreakEvenGap,
		"break_even_config_source":              breakEvenSource,
		"live_break_even_stop_price":            liveBreakEvenStopPrice,
		"break_even_order_detected":             breakEvenOrderDetected,
		"planned_ladder_stop_count":             plannedLadderStopCount,
		"planned_ladder_take_profit_count":      plannedLadderTakeProfitCount,
		"live_ladder_stop_count":                ladderStopCount,
		"live_ladder_take_profit_count":         ladderTakeProfitCount,
		"live_full_stop_count":                  fullStopCount,
		"live_full_take_profit_count":           fullTakeProfitCount,
		"fallback_order_detected":               fallbackActive,
		"live_fallback_stop_count":              fallbackStopCount,
		"full_stop_planned":                     fullStopPlanned,
		"full_take_profit_planned":              fullTakeProfitPlanned,
		"fallback_planned":                      fallbackPlanned,
		"ladder_stop_degraded":                  ladderDegradedStop,
		"ladder_take_profit_degraded":           ladderDegradedTakeProfit,
		"ladder_stop_degraded_to_full":          ladderDegradedToFullStop,
		"ladder_take_profit_degraded_to_full":   ladderDegradedToFullTakeProfit,
		"current_drawdown_stage_min_profit_pct": currentStageMinProfit,
		"current_drawdown_stage_rule_count":     currentStageRuleCount,
		"active_orders":                         activeOrders,
		"active_trailing_orders":                trailingOrders,
		"scheduled_tiers":                       tiers,
	}
}
