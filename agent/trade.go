package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"nofx/store"
	"strings"
	"sync"
	"time"
)

const (
	tradeAbsoluteMaxQuantity        = 1_000_000.0
	tradeLargeOrderNotionalUSDT     = 5_000.0
	tradeHardMaxOrderNotionalUSDT   = 100_000.0
	tradeLargeOrderEquityRatio      = 0.25
	tradeHardMaxOrderEquityRatio    = 1.00
	tradeLargeOrderConfirmCommandZH = "确认大额 %s"
	tradeLargeOrderConfirmCommandEN = "confirm large %s"
)

type tradeSelectedTrader interface {
	GetStrategyConfig() *store.StrategyConfig
	GetAccountInfo() (map[string]interface{}, error)
}

type tradeUnderlyingTrader interface {
	OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)
	OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)
	CloseLong(symbol string, quantity float64) (map[string]interface{}, error)
	CloseShort(symbol string, quantity float64) (map[string]interface{}, error)
	GetMarketPrice(symbol string) (float64, error)
}

// TradeAction represents a parsed trade intent from the LLM or user.
type TradeAction struct {
	ID                             string  `json:"id"`
	Action                         string  `json:"action"`    // "open_long", "open_short", "close_long", "close_short"
	Symbol                         string  `json:"symbol"`    // e.g. "BTCUSDT"
	Quantity                       float64 `json:"quantity"`  // amount
	Leverage                       int     `json:"leverage"`  // leverage multiplier
	TraderID                       string  `json:"trader_id"` // which trader to use
	Status                         string  `json:"status"`    // "pending", "confirmed", "executed", "failed", "expired"
	CreatedAt                      int64   `json:"created_at"`
	EstimatedPrice                 float64 `json:"estimated_price,omitempty"`
	EstimatedNotional              float64 `json:"estimated_notional,omitempty"`
	RequiresLargeOrderConfirmation bool    `json:"requires_large_order_confirmation,omitempty"`
	Error                          string  `json:"error,omitempty"`
}

// pendingTrades stores pending trade confirmations.
type pendingTrades struct {
	mu     sync.RWMutex
	trades map[string]*TradeAction // id -> trade
}

func newPendingTrades() *pendingTrades {
	return &pendingTrades{trades: make(map[string]*TradeAction)}
}

func (p *pendingTrades) Add(t *TradeAction) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.trades[t.ID] = t
}

func (p *pendingTrades) Get(id string) *TradeAction {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.trades[id]
}

func (p *pendingTrades) Remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.trades, id)
}

// CleanExpired removes trades older than 5 minutes.
func (p *pendingTrades) CleanExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute).Unix()
	for id, t := range p.trades {
		if t.CreatedAt < cutoff {
			delete(p.trades, id)
		}
	}
}

// parseTradeCommand parses natural language trade commands.
// Returns nil if the message is not a trade command.
func parseTradeCommand(text string) *TradeAction {
	upper := strings.ToUpper(strings.TrimSpace(text))

	// Pattern: "做多 BTC 0.01" / "做空 ETH 0.1" / "long BTC 0.01" / "short ETH 0.1"
	// Also: "平多 BTC" / "平空 ETH" / "close long BTC" / "close short ETH"

	var action, symbol string
	var quantity float64
	var leverage int

	words := strings.Fields(upper)
	if len(words) < 2 {
		return nil
	}

	switch words[0] {
	case "做多", "LONG", "BUY":
		action = "open_long"
	case "做空", "SHORT", "SELL":
		action = "open_short"
	case "平多":
		action = "close_long"
	case "平空":
		action = "close_short"
	case "CLOSE":
		if len(words) >= 3 {
			switch words[1] {
			case "LONG":
				action = "close_long"
				words = append(words[:1], words[2:]...) // remove "LONG"
			case "SHORT":
				action = "close_short"
				words = append(words[:1], words[2:]...) // remove "SHORT"
			}
		}
		if action == "" {
			return nil
		}
	default:
		return nil
	}

	// Parse symbol
	if len(words) < 2 {
		return nil
	}
	symbol = words[1]
	// Only append USDT for crypto symbols, not stock tickers
	if !isStockSymbol(symbol) && !strings.HasSuffix(symbol, "USDT") {
		symbol += "USDT"
	}

	// Parse quantity (optional)
	if len(words) >= 3 {
		fmt.Sscanf(words[2], "%f", &quantity)
	}

	// Parse leverage (optional, "x10" or "10x")
	if len(words) >= 4 {
		lev := strings.TrimSuffix(strings.TrimPrefix(words[3], "X"), "X")
		fmt.Sscanf(lev, "%d", &leverage)
	}

	if action == "" || symbol == "" {
		return nil
	}

	return &TradeAction{
		ID:        fmt.Sprintf("trade_%d", time.Now().UnixNano()),
		Action:    action,
		Symbol:    symbol,
		Quantity:  quantity,
		Leverage:  leverage,
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}
}

// executeTrade performs the actual trade execution via TraderManager.
func (a *Agent) executeTrade(ctx context.Context, trade *TradeAction) error {
	if a.traderManager == nil {
		return fmt.Errorf("no trader manager available")
	}

	wantStock, selectedTrader, underlyingTrader, err := a.resolveTradeExecutionContext(trade)
	if err != nil {
		return err
	}
	if err := validateTradeAction(trade, wantStock, selectedTrader, underlyingTrader); err != nil {
		return err
	}

	switch trade.Action {
	case "open_long":
		if trade.Quantity <= 0 {
			return fmt.Errorf("quantity must be > 0")
		}
		_, err := underlyingTrader.OpenLong(trade.Symbol, trade.Quantity, trade.Leverage)
		return err
	case "open_short":
		if trade.Quantity <= 0 {
			return fmt.Errorf("quantity must be > 0")
		}
		_, err := underlyingTrader.OpenShort(trade.Symbol, trade.Quantity, trade.Leverage)
		return err
	case "close_long":
		_, err := underlyingTrader.CloseLong(trade.Symbol, trade.Quantity)
		return err
	case "close_short":
		_, err := underlyingTrader.CloseShort(trade.Symbol, trade.Quantity)
		return err
	default:
		return fmt.Errorf("unknown action: %s", trade.Action)
	}
}

func (a *Agent) resolveTradeExecutionContext(trade *TradeAction) (bool, tradeSelectedTrader, tradeUnderlyingTrader, error) {
	if a.traderManager == nil {
		return false, nil, nil, fmt.Errorf("no trader manager available")
	}
	traders := a.traderManager.GetAllTraders()
	if len(traders) == 0 {
		return false, nil, nil, fmt.Errorf("no traders configured")
	}

	wantStock := isStockSymbol(trade.Symbol)
	for _, t := range traders {
		s := t.GetStatus()
		running, _ := s["is_running"].(bool)
		if !running {
			continue
		}
		ut := t.GetUnderlyingTrader()
		if ut == nil {
			continue
		}
		exchange := t.GetExchange()
		isAlpaca := exchange == "alpaca"
		if wantStock && !isAlpaca {
			continue
		}
		if !wantStock && isAlpaca {
			continue
		}
		return wantStock, t, ut, nil
	}

	if wantStock {
		return true, nil, nil, fmt.Errorf("no running stock trader (Alpaca) found — configure one to trade stocks")
	}
	return false, nil, nil, fmt.Errorf("no running trader supports trade execution")
}

func validateTradeAction(
	trade *TradeAction,
	wantStock bool,
	selectedTrader tradeSelectedTrader,
	underlyingTrader tradeUnderlyingTrader,
) error {
	if trade == nil {
		return fmt.Errorf("trade is required")
	}
	if math.IsNaN(trade.Quantity) || math.IsInf(trade.Quantity, 0) {
		return fmt.Errorf("quantity must be a finite number")
	}
	if !strings.HasPrefix(trade.Action, "open_") {
		return nil
	}
	if trade.Quantity <= 0 {
		return fmt.Errorf("quantity must be > 0")
	}
	if trade.Quantity > tradeAbsoluteMaxQuantity {
		return fmt.Errorf("quantity %.4f exceeds hard sanity cap %.0f", trade.Quantity, tradeAbsoluteMaxQuantity)
	}

	price, err := underlyingTrader.GetMarketPrice(trade.Symbol)
	if err != nil {
		return fmt.Errorf("failed to fetch market price for %s: %w", trade.Symbol, err)
	}
	if price <= 0 {
		return fmt.Errorf("invalid market price for %s", trade.Symbol)
	}
	positionValue := trade.Quantity * price
	trade.EstimatedPrice = price
	trade.EstimatedNotional = positionValue

	if positionValue > tradeHardMaxOrderNotionalUSDT {
		return fmt.Errorf("position value %.2f exceeds hard safety cap %.2f USDT", positionValue, tradeHardMaxOrderNotionalUSDT)
	}

	var equity float64
	if selectedTrader != nil {
		accountInfo, err := selectedTrader.GetAccountInfo()
		if err != nil {
			return fmt.Errorf("failed to load trader account info: %w", err)
		}
		equity = toFloat(accountInfo["total_equity"])
		if equity <= 0 {
			equity = toFloat(accountInfo["totalEquity"])
		}
		if equity <= 0 {
			return fmt.Errorf("invalid trader equity for risk validation")
		}
		if positionValue > equity*tradeHardMaxOrderEquityRatio {
			return fmt.Errorf(
				"position value %.2f USDT exceeds hard safety cap %.2f USDT (equity %.2f x %.2f)",
				positionValue,
				equity*tradeHardMaxOrderEquityRatio,
				equity,
				tradeHardMaxOrderEquityRatio,
			)
		}
		if positionValue >= equity*tradeLargeOrderEquityRatio {
			trade.RequiresLargeOrderConfirmation = true
		}
	}
	if positionValue >= tradeLargeOrderNotionalUSDT {
		trade.RequiresLargeOrderConfirmation = true
	}

	if wantStock {
		if trade.Leverage < 0 {
			return fmt.Errorf("leverage must be >= 0")
		}
		return nil
	}

	cfg := store.GetDefaultStrategyConfig("zh")
	if selectedTrader != nil && selectedTrader.GetStrategyConfig() != nil {
		cfg = *selectedTrader.GetStrategyConfig()
	}
	riskControl := cfg.RiskControl

	maxLeverage := riskControl.AltcoinMaxLeverage
	maxPositionValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if isBTCETHSymbol(trade.Symbol) {
		maxLeverage = riskControl.BTCETHMaxLeverage
		maxPositionValueRatio = riskControl.BTCETHMaxPositionValueRatio
	}
	if maxLeverage <= 0 {
		maxLeverage = 5
	}
	if trade.Leverage <= 0 {
		return fmt.Errorf("leverage must be > 0")
	}
	if trade.Leverage > maxLeverage {
		return fmt.Errorf("leverage exceeds configured limit (%dx > %dx)", trade.Leverage, maxLeverage)
	}

	minPositionSize := riskControl.MinPositionSize
	if minPositionSize <= 0 {
		minPositionSize = 12
	}
	if positionValue < minPositionSize {
		return fmt.Errorf("position value %.2f USDT is below configured minimum %.2f USDT", positionValue, minPositionSize)
	}

	if maxPositionValueRatio <= 0 {
		if isBTCETHSymbol(trade.Symbol) {
			maxPositionValueRatio = 5.0
		} else {
			maxPositionValueRatio = 1.0
		}
	}
	maxPositionValue := equity * maxPositionValueRatio
	if positionValue > maxPositionValue {
		return fmt.Errorf(
			"position value %.2f USDT exceeds configured limit %.2f USDT (equity %.2f x %.2f)",
			positionValue,
			maxPositionValue,
			equity,
			maxPositionValueRatio,
		)
	}
	return nil
}

func isBTCETHSymbol(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

// formatTradeConfirmation creates a confirmation message for a pending trade.
func formatTradeConfirmation(trade *TradeAction, lang string) string {
	actionNames := map[string]string{
		"open_long":   "做多 (Long)",
		"open_short":  "做空 (Short)",
		"close_long":  "平多 (Close Long)",
		"close_short": "平空 (Close Short)",
	}

	symbol := trade.Symbol
	if strings.HasSuffix(symbol, "USDT") {
		symbol = strings.TrimSuffix(symbol, "USDT")
	}
	actionName := actionNames[trade.Action]
	if actionName == "" {
		actionName = trade.Action
	}

	if lang == "zh" {
		msg := fmt.Sprintf("⚠️ **交易确认**\n\n"+
			"操作: %s\n"+
			"品种: %s\n", actionName, symbol)
		if trade.Quantity > 0 {
			msg += fmt.Sprintf("数量: %.4f\n", trade.Quantity)
		}
		if trade.Leverage > 0 {
			msg += fmt.Sprintf("杠杆: %dx\n", trade.Leverage)
		}
		if trade.EstimatedNotional > 0 {
			msg += fmt.Sprintf("估算仓位价值: %.2f USDT\n", trade.EstimatedNotional)
		}
		if trade.RequiresLargeOrderConfirmation {
			msg += fmt.Sprintf("\n⚠️ 该订单已触发大额风控，请发送 `"+tradeLargeOrderConfirmCommandZH+"` 执行交易，或忽略取消。", trade.ID)
			return msg
		}
		msg += fmt.Sprintf("\n发送 `确认 %s` 执行交易，或忽略取消。", trade.ID)
		return msg
	}

	msg := fmt.Sprintf("⚠️ **Trade Confirmation**\n\n"+
		"Action: %s\n"+
		"Symbol: %s\n", actionName, symbol)
	if trade.Quantity > 0 {
		msg += fmt.Sprintf("Quantity: %.4f\n", trade.Quantity)
	}
	if trade.Leverage > 0 {
		msg += fmt.Sprintf("Leverage: %dx\n", trade.Leverage)
	}
	if trade.EstimatedNotional > 0 {
		msg += fmt.Sprintf("Estimated notional: %.2f USDT\n", trade.EstimatedNotional)
	}
	if trade.RequiresLargeOrderConfirmation {
		msg += fmt.Sprintf("\n⚠️ This order triggered high-risk protection. Send `"+tradeLargeOrderConfirmCommandEN+"` to execute, or ignore to cancel.", trade.ID)
		return msg
	}
	msg += fmt.Sprintf("\nSend `confirm %s` to execute, or ignore to cancel.", trade.ID)
	return msg
}

// handleTradeConfirmation processes a trade confirmation message.
func (a *Agent) handleTradeConfirmation(ctx context.Context, userID int64, text, lang string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(text))

	var tradeID string
	largeConfirm := false
	if strings.HasPrefix(upper, "确认大额 ") || strings.HasPrefix(upper, "CONFIRM LARGE ") {
		largeConfirm = true
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			tradeID = parts[len(parts)-1]
		}
	} else if strings.HasPrefix(upper, "确认 ") || strings.HasPrefix(upper, "CONFIRM ") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			tradeID = parts[1]
		}
	}

	if tradeID == "" {
		return "", false
	}

	if a.pending == nil {
		return "", false
	}

	trade := a.pending.Get(tradeID)
	if trade == nil {
		if lang == "zh" {
			return "❌ 交易已过期或不存在。", true
		}
		return "❌ Trade expired or not found.", true
	}
	if trade.RequiresLargeOrderConfirmation && !largeConfirm {
		if lang == "zh" {
			return fmt.Sprintf("⚠️ 这是一笔大额订单，请发送 `"+tradeLargeOrderConfirmCommandZH+"` 继续执行。", trade.ID), true
		}
		return fmt.Sprintf("⚠️ This is a high-risk order. Send `"+tradeLargeOrderConfirmCommandEN+"` to continue.", trade.ID), true
	}

	a.pending.Remove(tradeID)
	trade.Status = "confirmed"

	a.logger.Info("executing trade",
		slog.String("id", trade.ID),
		slog.String("action", trade.Action),
		slog.String("symbol", trade.Symbol),
		slog.Float64("quantity", trade.Quantity),
	)

	err := a.executeTrade(ctx, trade)
	if err != nil {
		trade.Status = "failed"
		trade.Error = err.Error()
		if lang == "zh" {
			return fmt.Sprintf("❌ 交易执行失败: %s", err.Error()), true
		}
		return fmt.Sprintf("❌ Trade execution failed: %s", err.Error()), true
	}

	trade.Status = "executed"
	symbol := trade.Symbol
	if strings.HasSuffix(symbol, "USDT") {
		symbol = strings.TrimSuffix(symbol, "USDT")
	}
	actionEmoji := "📈"
	if strings.Contains(trade.Action, "short") {
		actionEmoji = "📉"
	}
	if strings.Contains(trade.Action, "close") {
		actionEmoji = "✅"
	}

	qtyStr := ""
	if trade.Quantity > 0 {
		qtyStr = fmt.Sprintf(" %.4f", trade.Quantity)
	}

	if lang == "zh" {
		return fmt.Sprintf("%s 交易已执行！\n%s %s%s", actionEmoji, trade.Action, symbol, qtyStr), true
	}
	return fmt.Sprintf("%s Trade executed!\n%s %s%s", actionEmoji, trade.Action, symbol, qtyStr), true
}

// marshals trade action to JSON for embedding in responses
func marshalTradeAction(trade *TradeAction) string {
	b, _ := json.Marshal(trade)
	return string(b)
}
