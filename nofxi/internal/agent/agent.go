// Package agent implements the NOFXi Agent Core.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"nofx/nofxi/internal/execution"
	"nofx/nofxi/internal/memory"
	"nofx/nofxi/internal/perception"
	"nofx/nofxi/internal/thinking"
)

// Agent is the NOFXi agent core.
type Agent struct {
	config         *Config
	memory         *memory.Store
	thinker        thinking.Engine
	bridge         *execution.Bridge
	monitor        *perception.MarketMonitor
	logger         *slog.Logger
	NotifyFunc     func(userID int64, text string) error
	strategyRunner *StrategyRunner
}

func New(cfg *Config, mem *memory.Store, thinker thinking.Engine, logger *slog.Logger) *Agent {
	a := &Agent{config: cfg, memory: mem, thinker: thinker, logger: logger}
	a.strategyRunner = NewStrategyRunner(a, logger)
	return a
}

func (a *Agent) SetBridge(b *execution.Bridge)         { a.bridge = b }
func (a *Agent) SetMonitor(m *perception.MarketMonitor) { a.monitor = m }

func (a *Agent) getLang(userID int64) string {
	l, _ := a.memory.GetPreference(userID, "lang")
	if l == "" {
		l = a.config.Agent.Language
	}
	if l != "zh" && l != "en" {
		l = "en"
	}
	return l
}

// HandleMessage processes a user message and returns a response.
func (a *Agent) HandleMessage(ctx context.Context, userID int64, text string) (string, error) {
	// Extract lang prefix from web UI
	if strings.HasPrefix(text, "[lang:") {
		if end := strings.Index(text, "] "); end > 0 {
			lang := text[6:end]
			if lang == "zh" || lang == "en" {
				a.memory.SetPreference(userID, "lang", lang)
			}
			text = text[end+2:]
		}
	}

	a.logger.Info("incoming message", "user_id", userID, "text", text)
	a.memory.SaveMessage(userID, "user", text)

	intent := Route(text)
	a.logger.Info("routed intent", "type", intent.Type, "params", intent.Params)

	L := a.getLang(userID)
	var resp string
	var err error

	switch intent.Type {
	case IntentHelp:
		resp = msg(L, "help")
	case IntentStatus:
		resp = a.handleStatus(L)
	case IntentQuery:
		resp, err = a.handleQuery(ctx, L, intent)
	case IntentAnalyze:
		resp, err = a.handleAnalyze(ctx, L, intent)
	case IntentTrade:
		resp, err = a.handleTrade(ctx, userID, L, intent)
	case IntentWatch:
		resp = a.HandleWatchCommand(intent.Raw)
	case IntentStrategy:
		resp = a.handleStrategyCommand(intent.Raw)
	case IntentSettings:
		resp = fmt.Sprintf(msg(L, "settings"), L, a.config.LLM.Model, a.config.LLM.Provider, len(a.config.Exchanges))
	default:
		resp, err = a.handleChat(ctx, userID, L, text)
	}

	if err != nil {
		a.logger.Error("handle message", "intent", intent.Type, "error", err)
		resp = fmt.Sprintf("⚠️ Error: %v", err)
	}

	a.memory.SaveMessage(userID, "assistant", resp)
	return resp, nil
}

func (a *Agent) handleStatus(L string) string {
	wc := 0
	if a.monitor != nil {
		wc = len(a.monitor.GetAllSnapshots())
	}
	bs := msg(L, "bridge_disconnected")
	if a.bridge != nil {
		bs = msg(L, "bridge_connected")
	}
	return fmt.Sprintf(msg(L, "status_title"),
		a.config.Agent.Name, a.config.LLM.Model, a.config.LLM.Provider,
		bs, wc, time.Now().Format("2006-01-02 15:04:05"))
}

func (a *Agent) handleQuery(ctx context.Context, L string, intent Intent) (string, error) {
	raw := strings.ToLower(intent.Raw)

	if a.bridge != nil && (strings.Contains(raw, "position") || strings.Contains(raw, "持仓")) {
		return a.queryPositions(L)
	}
	if a.bridge != nil && (strings.Contains(raw, "balance") || strings.Contains(raw, "余额")) {
		return a.queryBalance(L)
	}

	trades, err := a.memory.GetRecentTrades(10)
	if err != nil {
		return "", fmt.Errorf("get trades: %w", err)
	}
	if len(trades) == 0 {
		return msg(L, "no_trades"), nil
	}

	var sb strings.Builder
	sb.WriteString(msg(L, "recent_trades"))
	total := 0.0
	for _, t := range trades {
		e := "🟢"
		if t.PnL < 0 {
			e = "🔴"
		}
		sb.WriteString(fmt.Sprintf("%s %s %s %s — $%.2f (P/L: $%.2f)\n",
			e, t.Side, t.Symbol, t.Exchange, t.Price*t.Quantity, t.PnL))
		total += t.PnL
	}
	sb.WriteString(fmt.Sprintf(msg(L, "total_pnl"), total))
	return sb.String(), nil
}

func (a *Agent) queryPositions(L string) (string, error) {
	var all []execution.Position
	for _, ex := range a.config.Exchanges {
		pos, err := a.bridge.GetPositions(ex.Name)
		if err != nil {
			continue
		}
		all = append(all, pos...)
	}
	if len(all) == 0 {
		return msg(L, "no_positions"), nil
	}

	var sb strings.Builder
	sb.WriteString(msg(L, "open_positions"))
	total := 0.0
	for _, p := range all {
		e := "🟢"
		if p.PnL < 0 {
			e = "🔴"
		}
		sb.WriteString(fmt.Sprintf("%s *%s* %s\n   Size: %.4f | Entry: $%.2f\n   Mark: $%.2f | P/L: $%.2f\n",
			e, p.Symbol, strings.ToUpper(p.Side), p.Size, p.EntryPrice, p.MarkPrice, p.PnL))
		if p.Leverage > 0 {
			sb.WriteString(fmt.Sprintf("   Leverage: %.0fx | Exchange: %s\n", p.Leverage, p.Exchange))
		}
		sb.WriteString("\n")
		total += p.PnL
	}
	sb.WriteString(fmt.Sprintf(msg(L, "total_unrealized"), total))
	return sb.String(), nil
}

func (a *Agent) queryBalance(L string) (string, error) {
	var sb strings.Builder
	sb.WriteString(msg(L, "account_balance"))
	for _, ex := range a.config.Exchanges {
		bal, err := a.bridge.GetBalance(ex.Name)
		if err != nil {
			sb.WriteString(fmt.Sprintf("• %s: ⚠️ Error\n", ex.Name))
			continue
		}
		sb.WriteString(fmt.Sprintf("*%s*\n", ex.Name))
		sb.WriteString(fmt.Sprintf(msg(L, "balance_total"), bal.Total))
		sb.WriteString(fmt.Sprintf(msg(L, "balance_available"), bal.Available))
		sb.WriteString(fmt.Sprintf(msg(L, "balance_in_position"), bal.InPosition))
	}
	return sb.String(), nil
}

func (a *Agent) handleAnalyze(ctx context.Context, L string, intent Intent) (string, error) {
	symbol := "BTC"
	if d, ok := intent.Params["detail"]; ok && d != "" {
		symbol = strings.ToUpper(strings.TrimSpace(d))
	}

	priceInfo := ""
	if a.monitor != nil {
		if snap, ok := a.monitor.GetSnapshot(symbol + "USDT"); ok && snap.LastPrice > 0 {
			priceInfo = fmt.Sprintf("\nCurrent price: $%.2f", snap.LastPrice)
		}
	}

	prompt := fmt.Sprintf("Analyze %s/USDT for trading. %s\nConsider: trend, support/resistance, momentum, volume, sentiment.\nGive specific entry/exit levels and stop loss. Be concise. Respond in %s.",
		symbol, priceInfo, map[string]string{"zh": "Chinese", "en": "English"}[L])

	analysis, err := a.thinker.Analyze(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("AI analyze: %w", err)
	}

	emojiMap := map[string]string{"buy": "🟢 BUY", "sell": "🔴 SELL", "hold": "🟡 HOLD", "wait": "⏳ WAIT"}
	action := emojiMap[analysis.Action]
	if action == "" {
		action = "🤔 " + analysis.Action
	}

	result := fmt.Sprintf(msg(L, "analysis_signal"), symbol, action, analysis.Confidence*100, analysis.Reasoning)
	if analysis.StopLoss > 0 {
		result += fmt.Sprintf(msg(L, "stop_loss"), analysis.StopLoss)
	}
	if analysis.TakeProfit > 0 {
		result += fmt.Sprintf(msg(L, "take_profit"), analysis.TakeProfit)
	}
	return result, nil
}

func (a *Agent) handleTrade(ctx context.Context, userID int64, L string, intent Intent) (string, error) {
	action := strings.ToLower(intent.Params["action"])
	detail := intent.Params["detail"]

	if a.bridge == nil || len(a.config.Exchanges) == 0 {
		return msg(L, "no_exchange"), nil
	}

	parts := strings.Fields(detail)
	if len(parts) < 1 {
		return msg(L, "trade_usage"), nil
	}

	symbol := strings.ToUpper(parts[0])
	if !strings.HasSuffix(symbol, "USDT") {
		symbol += "USDT"
	}

	quantity := 0.0
	leverage := 1
	if len(parts) >= 2 {
		q, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return fmt.Sprintf(msg(L, "invalid_quantity"), parts[1]), nil
		}
		quantity = q
	}
	if len(parts) >= 3 {
		if l, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(parts[2]), "x")); err == nil {
			leverage = l
		}
	}

	if quantity <= 0 {
		return msg(L, "specify_quantity"), nil
	}

	var side string
	switch action {
	case "buy", "long", "open_long", "做多":
		side = "LONG"
	case "sell", "short", "open_short", "做空":
		side = "SHORT"
	case "close", "平仓":
		side = "CLOSE_LONG"
	default:
		side = strings.ToUpper(action)
	}

	exchange := a.config.Exchanges[0].Name
	a.memory.SetPreference(userID, "pending_trade",
		fmt.Sprintf("%s|%s|%f|%d|%s", side, symbol, quantity, leverage, exchange))

	return fmt.Sprintf(msg(L, "confirm_trade"), side, symbol, quantity, leverage, exchange), nil
}

// ExecutePendingTrade executes a confirmed trade.
func (a *Agent) ExecutePendingTrade(ctx context.Context, userID int64, L string) (string, error) {
	pending, err := a.memory.GetPreference(userID, "pending_trade")
	if err != nil || pending == "" {
		return "", fmt.Errorf(msg(L, "no_pending"))
	}
	a.memory.SetPreference(userID, "pending_trade", "")

	parts := strings.Split(pending, "|")
	if len(parts) != 5 {
		return "", fmt.Errorf("invalid pending trade")
	}

	side, symbol := parts[0], parts[1]
	quantity, _ := strconv.ParseFloat(parts[2], 64)
	leverage, _ := strconv.Atoi(parts[3])
	exchange := parts[4]

	result, err := a.bridge.PlaceOrder(exchange, symbol, side, quantity, leverage)
	if err != nil {
		return "", fmt.Errorf("execute trade: %w", err)
	}

	tr := &memory.TradeRecord{Exchange: exchange, Symbol: symbol, Side: strings.ToLower(side), Type: "market", Quantity: quantity, Status: "open"}
	if p, ok := result["avgPrice"].(float64); ok {
		tr.Price = p
	}
	a.memory.SaveTrade(tr)

	return fmt.Sprintf(msg(L, "trade_executed"), side, symbol, quantity, leverage, exchange, result), nil
}

func (a *Agent) handleStrategyCommand(text string) string {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return a.strategyRunner.FormatStrategyList()
	}
	switch strings.ToLower(parts[1]) {
	case "list":
		return a.strategyRunner.FormatStrategyList()
	case "start":
		if len(parts) < 3 {
			return "Usage: `/strategy start BTC 1h`"
		}
		sym := strings.ToUpper(parts[2])
		if !strings.HasSuffix(sym, "USDT") {
			sym += "USDT"
		}
		iv := 1 * time.Hour
		if len(parts) >= 4 {
			switch parts[3] {
			case "15m":
				iv = 15 * time.Minute
			case "30m":
				iv = 30 * time.Minute
			case "4h":
				iv = 4 * time.Hour
			}
		}
		ex := "binance"
		if len(parts) >= 5 {
			ex = parts[4]
		}
		id, err := a.strategyRunner.StartStrategy("AI-"+sym, sym, ex, iv)
		if err != nil {
			return fmt.Sprintf("⚠️ %v", err)
		}
		return fmt.Sprintf("🚀 Strategy started!\n\n• ID: `%s`\n• Symbol: %s\n• Interval: %s\n• Exchange: %s", id, sym, iv, ex)
	case "stop":
		if len(parts) < 3 {
			return "Usage: `/strategy stop <id>`"
		}
		if err := a.strategyRunner.StopStrategy(parts[2]); err != nil {
			return fmt.Sprintf("⚠️ %v", err)
		}
		return "✅ Strategy stopped."
	case "stopall":
		a.strategyRunner.StopAll()
		return "✅ All strategies stopped."
	default:
		return "Use: `/strategy list|start|stop|stopall`"
	}
}

func (a *Agent) handleChat(ctx context.Context, userID int64, L string, text string) (string, error) {
	lower := strings.ToLower(text)
	if lower == "yes" || lower == "y" || lower == "确认" || lower == "是" {
		if p, _ := a.memory.GetPreference(userID, "pending_trade"); p != "" {
			return a.ExecutePendingTrade(ctx, userID, L)
		}
	}

	history, _ := a.memory.GetRecentMessages(userID, 20)

	sysPrompt := fmt.Sprintf(msg(L, "system_prompt"), time.Now().Format("2006-01-02 15:04:05"))
	msgs := []thinking.Message{{Role: "system", Content: sysPrompt}}
	for _, m := range history {
		msgs = append(msgs, thinking.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, thinking.Message{Role: "user", Content: text})

	return a.thinker.Chat(ctx, msgs)
}
