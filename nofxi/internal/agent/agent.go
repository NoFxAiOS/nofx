// Package agent implements the NOFXi Agent Core.
//
// The Agent is the central orchestrator that:
// 1. Receives user messages (via Interaction layer)
// 2. Routes intents (trade, query, analyze, chat)
// 3. Uses Thinking layer for AI decisions
// 4. Stores context in Memory layer
// 5. Executes trades via Execution bridge (→ NOFX engine)
// 6. Monitors markets via Perception layer
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
	config  *Config
	memory  *memory.Store
	thinker thinking.Engine
	bridge  *execution.Bridge
	monitor *perception.MarketMonitor
	logger  *slog.Logger

	// NotifyFunc sends proactive notifications to users (set by interaction layer)
	NotifyFunc func(userID int64, text string) error

	// Strategy runner
	strategyRunner *StrategyRunner
}

// New creates a new Agent with the given config.
func New(cfg *Config, mem *memory.Store, thinker thinking.Engine, logger *slog.Logger) *Agent {
	a := &Agent{
		config:  cfg,
		memory:  mem,
		thinker: thinker,
		logger:  logger,
	}
	a.strategyRunner = NewStrategyRunner(a, logger)
	return a
}

// SetBridge attaches the execution bridge.
func (a *Agent) SetBridge(bridge *execution.Bridge) {
	a.bridge = bridge
}

// SetMonitor attaches the market monitor and wires up alert notifications.
func (a *Agent) SetMonitor(monitor *perception.MarketMonitor) {
	a.monitor = monitor
}

// HandleMessage processes a user message and returns a response.
func (a *Agent) HandleMessage(ctx context.Context, userID int64, text string) (string, error) {
	a.logger.Info("incoming message", "user_id", userID, "text", text)

	// Save user message to memory
	if err := a.memory.SaveMessage(userID, "user", text); err != nil {
		a.logger.Error("save user message", "error", err)
	}

	// Route intent
	intent := Route(text)
	a.logger.Info("routed intent", "type", intent.Type, "params", intent.Params)

	var response string
	var err error

	switch intent.Type {
	case IntentHelp:
		response = a.handleHelp()
	case IntentStatus:
		response = a.handleStatus()
	case IntentQuery:
		response, err = a.handleQuery(ctx, intent)
	case IntentAnalyze:
		response, err = a.handleAnalyze(ctx, intent)
	case IntentTrade:
		response, err = a.handleTrade(ctx, userID, intent)
	case IntentWatch:
		response = a.HandleWatchCommand(intent.Raw)
	case IntentStrategy:
		response = a.handleStrategyCommand(intent.Raw)
	case IntentSettings:
		response = a.handleSettings(intent)
	default:
		response, err = a.handleChat(ctx, userID, text)
	}

	if err != nil {
		a.logger.Error("handle message", "intent", intent.Type, "error", err)
		response = fmt.Sprintf("⚠️ Error: %v", err)
	}

	// Save assistant response to memory
	if err := a.memory.SaveMessage(userID, "assistant", response); err != nil {
		a.logger.Error("save assistant message", "error", err)
	}

	return response, nil
}

func (a *Agent) handleHelp() string {
	return `🤖 *NOFXi — Your AI Trading Agent*

*Trading:*
/buy BTC 0.01 — Open long (market order)
/sell BTC 0.01 — Open short
/close BTC — Close position
/positions — View open positions
/balance — Check balance
/pnl — Profit & Loss history

*Analysis:*
/analyze BTC — AI market analysis
/watch BTC — Monitor price
/alert BTC above 100000 — Price alert

*System:*
/status — Agent status
/settings — Preferences
/help — This menu

*Natural Language:*
• "帮我做多 ETH，2x 杠杆，0.1 个"
• "分析一下 BTC 走势"
• "现在持仓情况怎样"
• "BTC 到 10 万提醒我"

Just talk to me 💬`
}

func (a *Agent) handleStatus() string {
	// Market monitor status
	watchCount := 0
	if a.monitor != nil {
		watchCount = len(a.monitor.GetAllSnapshots())
	}
	bridgeStatus := "❌ Not connected"
	if a.bridge != nil {
		bridgeStatus = "✅ Connected"
	}

	return fmt.Sprintf(`📊 *NOFXi Status*

• Agent: %s
• Model: %s
• Provider: %s
• Memory: ✅ Online
• Execution: %s
• Watching: %d symbols
• Time: %s`,
		a.config.Agent.Name,
		a.config.LLM.Model,
		a.config.LLM.Provider,
		bridgeStatus,
		watchCount,
		time.Now().Format("2006-01-02 15:04:05"),
	)
}

func (a *Agent) handleQuery(ctx context.Context, intent Intent) (string, error) {
	raw := strings.ToLower(intent.Raw)

	// Try live positions from exchange first
	if a.bridge != nil && (strings.Contains(raw, "position") || strings.Contains(raw, "持仓")) {
		return a.queryPositions()
	}
	if a.bridge != nil && (strings.Contains(raw, "balance") || strings.Contains(raw, "余额")) {
		return a.queryBalance()
	}

	// Fall back to trade history from memory
	trades, err := a.memory.GetRecentTrades(10)
	if err != nil {
		return "", fmt.Errorf("get trades: %w", err)
	}

	if len(trades) == 0 {
		return "📭 No trades yet. Start with `/buy BTC 0.01` or ask me to `/analyze BTC`.", nil
	}

	var sb strings.Builder
	sb.WriteString("📋 *Recent Trades*\n\n")
	totalPnL := 0.0
	for _, t := range trades {
		emoji := "🟢"
		if t.PnL < 0 {
			emoji = "🔴"
		}
		sb.WriteString(fmt.Sprintf("%s %s %s %s — $%.2f (P/L: $%.2f)\n",
			emoji, t.Side, t.Symbol, t.Exchange, t.Price*t.Quantity, t.PnL))
		totalPnL += t.PnL
	}
	sb.WriteString(fmt.Sprintf("\n💰 Total P/L: $%.2f", totalPnL))
	return sb.String(), nil
}

func (a *Agent) queryPositions() (string, error) {
	var allPositions []execution.Position
	for _, ex := range a.config.Exchanges {
		positions, err := a.bridge.GetPositions(ex.Name)
		if err != nil {
			a.logger.Error("get positions", "exchange", ex.Name, "error", err)
			continue
		}
		allPositions = append(allPositions, positions...)
	}

	if len(allPositions) == 0 {
		return "📭 No open positions.", nil
	}

	var sb strings.Builder
	sb.WriteString("📊 *Open Positions*\n\n")
	totalPnL := 0.0
	for _, p := range allPositions {
		emoji := "🟢"
		if p.PnL < 0 {
			emoji = "🔴"
		}
		sb.WriteString(fmt.Sprintf("%s *%s* %s\n", emoji, p.Symbol, strings.ToUpper(p.Side)))
		sb.WriteString(fmt.Sprintf("   Size: %.4f | Entry: $%.2f\n", p.Size, p.EntryPrice))
		sb.WriteString(fmt.Sprintf("   Mark: $%.2f | P/L: $%.2f\n", p.MarkPrice, p.PnL))
		if p.Leverage > 0 {
			sb.WriteString(fmt.Sprintf("   Leverage: %.0fx | Exchange: %s\n", p.Leverage, p.Exchange))
		}
		sb.WriteString("\n")
		totalPnL += p.PnL
	}
	sb.WriteString(fmt.Sprintf("💰 *Total Unrealized P/L: $%.2f*", totalPnL))
	return sb.String(), nil
}

func (a *Agent) queryBalance() (string, error) {
	var sb strings.Builder
	sb.WriteString("💰 *Account Balance*\n\n")

	for _, ex := range a.config.Exchanges {
		bal, err := a.bridge.GetBalance(ex.Name)
		if err != nil {
			a.logger.Error("get balance", "exchange", ex.Name, "error", err)
			sb.WriteString(fmt.Sprintf("• %s: ⚠️ Error\n", ex.Name))
			continue
		}
		sb.WriteString(fmt.Sprintf("*%s*\n", strings.Title(ex.Name)))
		sb.WriteString(fmt.Sprintf("   Total: $%.2f\n", bal.Total))
		sb.WriteString(fmt.Sprintf("   Available: $%.2f\n", bal.Available))
		sb.WriteString(fmt.Sprintf("   In Position: $%.2f\n\n", bal.InPosition))
	}
	return sb.String(), nil
}

func (a *Agent) handleAnalyze(ctx context.Context, intent Intent) (string, error) {
	symbol := "BTC"
	if detail, ok := intent.Params["detail"]; ok && detail != "" {
		symbol = strings.ToUpper(strings.TrimSpace(detail))
	}

	// Add live price if available
	priceInfo := ""
	if a.monitor != nil {
		if snap, ok := a.monitor.GetSnapshot(symbol + "USDT"); ok && snap.LastPrice > 0 {
			priceInfo = fmt.Sprintf("\nCurrent price: $%.2f", snap.LastPrice)
		}
	}
	if priceInfo == "" && a.bridge != nil {
		for _, ex := range a.config.Exchanges {
			if price, err := a.bridge.GetPrice(ex.Name, symbol+"USDT"); err == nil && price > 0 {
				priceInfo = fmt.Sprintf("\nCurrent price: $%.2f (from %s)", price, ex.Name)
				break
			}
		}
	}

	prompt := fmt.Sprintf(
		"Analyze %s/USDT for trading. %s\n"+
			"Consider: trend, support/resistance, momentum indicators, volume, sentiment.\n"+
			"Give specific entry/exit levels and stop loss. Be concise.",
		symbol, priceInfo)

	analysis, err := a.thinker.Analyze(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("AI analyze: %w", err)
	}

	emoji := map[string]string{
		"buy":  "🟢 BUY",
		"sell": "🔴 SELL",
		"hold": "🟡 HOLD",
		"wait": "⏳ WAIT",
	}

	action := emoji[analysis.Action]
	if action == "" {
		action = "🤔 " + analysis.Action
	}

	result := fmt.Sprintf("🔍 *%s/USDT Analysis*\n\nSignal: %s\nConfidence: %.0f%%\n\n%s",
		symbol, action, analysis.Confidence*100, analysis.Reasoning)

	if analysis.StopLoss > 0 {
		result += fmt.Sprintf("\n\n🛑 Stop Loss: $%.2f", analysis.StopLoss)
	}
	if analysis.TakeProfit > 0 {
		result += fmt.Sprintf("\n🎯 Take Profit: $%.2f", analysis.TakeProfit)
	}

	return result, nil
}

func (a *Agent) handleTrade(ctx context.Context, userID int64, intent Intent) (string, error) {
	action := strings.ToLower(intent.Params["action"])
	detail := intent.Params["detail"]

	if a.bridge == nil {
		return fmt.Sprintf("⚡ *Trade: %s %s*\n\n🔧 No exchange connected. Configure exchanges in config.yaml first.",
			strings.ToUpper(action), detail), nil
	}

	// Parse: "BTC 0.01" or "BTCUSDT 0.01 2x"
	parts := strings.Fields(detail)
	if len(parts) < 1 {
		return "❓ Usage: `/buy BTC 0.01` or `/sell ETH 0.5 3x`", nil
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
			return fmt.Sprintf("❓ Invalid quantity: %s", parts[1]), nil
		}
		quantity = q
	}
	if len(parts) >= 3 {
		levStr := strings.TrimSuffix(strings.ToLower(parts[2]), "x")
		l, err := strconv.Atoi(levStr)
		if err == nil {
			leverage = l
		}
	}

	if quantity <= 0 && (action == "buy" || action == "sell" || action == "long" || action == "short") {
		return "❓ Please specify quantity: `/buy BTC 0.01`", nil
	}

	// Map action to execution side
	var side string
	switch action {
	case "buy", "long", "open_long":
		side = "LONG"
	case "sell", "short", "open_short":
		side = "SHORT"
	case "close":
		side = "CLOSE_LONG" // TODO: detect which side to close
	default:
		side = strings.ToUpper(action)
	}

	// Use first configured exchange
	if len(a.config.Exchanges) == 0 {
		return "⚠️ 还没有配置交易所。请在 config.yaml 的 exchanges 中添加交易所 API Key。", nil
	}
	exchange := a.config.Exchanges[0].Name

	// Confirm with user before executing
	confirmMsg := fmt.Sprintf("⚡ *Confirm Trade*\n\n"+
		"• Action: %s\n"+
		"• Symbol: %s\n"+
		"• Quantity: %.6f\n"+
		"• Leverage: %dx\n"+
		"• Exchange: %s\n\n"+
		"Reply 'yes' to execute.",
		side, symbol, quantity, leverage, exchange)

	// Store pending trade for confirmation
	a.memory.SetPreference(userID, "pending_trade",
		fmt.Sprintf("%s|%s|%f|%d|%s", side, symbol, quantity, leverage, exchange))

	return confirmMsg, nil
}

// ExecutePendingTrade executes a trade that was previously confirmed.
func (a *Agent) ExecutePendingTrade(ctx context.Context, userID int64) (string, error) {
	pending, err := a.memory.GetPreference(userID, "pending_trade")
	if err != nil || pending == "" {
		return "", fmt.Errorf("no pending trade")
	}

	// Clear pending
	a.memory.SetPreference(userID, "pending_trade", "")

	parts := strings.Split(pending, "|")
	if len(parts) != 5 {
		return "", fmt.Errorf("invalid pending trade data")
	}

	side := parts[0]
	symbol := parts[1]
	quantity, _ := strconv.ParseFloat(parts[2], 64)
	leverage, _ := strconv.Atoi(parts[3])
	exchange := parts[4]

	result, err := a.bridge.PlaceOrder(exchange, symbol, side, quantity, leverage)
	if err != nil {
		return "", fmt.Errorf("execute trade: %w", err)
	}

	// Save trade to memory
	tradeRecord := &memory.TradeRecord{
		Exchange: exchange,
		Symbol:   symbol,
		Side:     strings.ToLower(side),
		Type:     "market",
		Quantity: quantity,
		Status:   "open",
	}
	if price, ok := result["avgPrice"].(float64); ok {
		tradeRecord.Price = price
	}
	a.memory.SaveTrade(tradeRecord)

	return fmt.Sprintf("✅ *Trade Executed!*\n\n"+
		"• %s %s\n"+
		"• Qty: %.6f\n"+
		"• Leverage: %dx\n"+
		"• Exchange: %s\n"+
		"• Result: %v",
		side, symbol, quantity, leverage, exchange, result), nil
}

func (a *Agent) handleStrategyCommand(text string) string {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return a.strategyRunner.FormatStrategyList()
	}

	subcmd := strings.ToLower(parts[1])
	switch subcmd {
	case "list":
		return a.strategyRunner.FormatStrategyList()
	case "start":
		if len(parts) < 3 {
			return "Usage: `/strategy start BTC 1h` or `/strategy start ETH 4h binance`"
		}
		symbol := strings.ToUpper(parts[2])
		if !strings.HasSuffix(symbol, "USDT") {
			symbol += "USDT"
		}
		interval := 1 * time.Hour
		if len(parts) >= 4 {
			switch parts[3] {
			case "15m":
				interval = 15 * time.Minute
			case "30m":
				interval = 30 * time.Minute
			case "1h":
				interval = 1 * time.Hour
			case "4h":
				interval = 4 * time.Hour
			}
		}
		exchange := "binance"
		if len(parts) >= 5 {
			exchange = parts[4]
		}
		name := fmt.Sprintf("AI-%s", symbol)
		id, err := a.strategyRunner.StartStrategy(name, symbol, exchange, interval)
		if err != nil {
			return fmt.Sprintf("⚠️ %v", err)
		}
		return fmt.Sprintf("🚀 Strategy started!\n\n• ID: `%s`\n• Symbol: %s\n• Interval: %s\n• Exchange: %s\n\nStop with: `/strategy stop %s`",
			id, symbol, interval, exchange, id)
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
		return "Unknown subcommand. Use: `/strategy list|start|stop|stopall`"
	}
}

func (a *Agent) handleSettings(intent Intent) string {
	return `⚙️ *Settings*

• Language: ` + a.config.Agent.Language + `
• Model: ` + a.config.LLM.Model + `
• Provider: ` + a.config.LLM.Provider + `
• Exchanges: ` + fmt.Sprintf("%d configured", len(a.config.Exchanges))
}

func (a *Agent) handleChat(ctx context.Context, userID int64, text string) (string, error) {
	// Check for trade confirmation
	lower := strings.ToLower(text)
	if lower == "yes" || lower == "y" || lower == "确认" || lower == "是" {
		pending, _ := a.memory.GetPreference(userID, "pending_trade")
		if pending != "" {
			return a.ExecutePendingTrade(ctx, userID)
		}
	}

	// Get conversation history for context
	history, err := a.memory.GetRecentMessages(userID, 20)
	if err != nil {
		a.logger.Error("get history", "error", err)
	}

	// Build messages with system prompt
	messages := []thinking.Message{
		{
			Role: "system",
			Content: fmt.Sprintf(`You are NOFXi, an AI trading agent built on NOFX.

Your capabilities:
- Market analysis and trading recommendations
- Real-time position and balance monitoring
- Trade execution (open/close positions on exchanges)
- Price alerts and market monitoring
- Risk management advice

You support multiple exchanges: Binance, OKX, Bybit, Bitget, KuCoin, Gate, Hyperliquid, etc.

Be concise, confident, and action-oriented. Use trading emojis.
Respond in the same language the user uses.
Current time: %s`, time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	for _, msg := range history {
		messages = append(messages, thinking.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	messages = append(messages, thinking.Message{
		Role:    "user",
		Content: text,
	})

	return a.thinker.Chat(ctx, messages)
}
