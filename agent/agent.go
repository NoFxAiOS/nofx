// Package agent implements the NOFXi Agent Core.
//
// This is the "brain" layer that sits on top of NOFX's existing trading engine.
// It adds: natural language interaction, proactive market monitoring,
// trading memory/learning, and autonomous decision-making.
//
// Architecture:
//   NOFX (engine) provides: kernel, trader, market, store, mcp
//   Agent (brain) adds: perception, thinking, memory, interaction
//
// The agent does NOT replace any NOFX functionality — it enhances it.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"nofx/manager"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

// Agent is the NOFXi intelligence layer on top of NOFX.
type Agent struct {
	// NOFX core (injected)
	traderManager *manager.TraderManager
	store         *store.Store
	aiClient      mcp.AIClient

	// Agent components
	config    *Config
	sentinel  *Sentinel
	brain     *Brain
	scheduler *Scheduler
	router    *Router
	logger    *slog.Logger

	// Notification callback (set by telegram/web)
	NotifyFunc func(userID int64, text string) error
}

// Config holds agent-specific configuration.
type Config struct {
	Language       string   `json:"language"`         // "zh" or "en"
	WatchSymbols   []string `json:"watch_symbols"`    // Default symbols to watch
	EnableBriefs   bool     `json:"enable_briefs"`    // Morning/evening market briefs
	EnableNews     bool     `json:"enable_news"`      // News scanning
	EnableSentinel bool     `json:"enable_sentinel"`  // Market anomaly detection
	BriefTimes     []int    `json:"brief_times"`      // Hours to send briefs (e.g. [8, 20])
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Language:       "zh",
		WatchSymbols:   []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"},
		EnableBriefs:   true,
		EnableNews:     true,
		EnableSentinel: true,
		BriefTimes:     []int{8, 20},
	}
}

// New creates a new NOFXi Agent.
func New(
	traderMgr *manager.TraderManager,
	st *store.Store,
	cfg *Config,
	logger *slog.Logger,
) *Agent {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	a := &Agent{
		traderManager: traderMgr,
		store:         st,
		config:        cfg,
		logger:        logger,
	}

	a.router = NewRouter()
	a.scheduler = NewScheduler(a, logger)

	return a
}

// SetAIClient sets the MCP AI client for LLM calls.
func (a *Agent) SetAIClient(c mcp.AIClient) {
	a.aiClient = c
}



// Start starts all agent services.
func (a *Agent) Start() {
	a.logger.Info("starting NOFXi agent...")

	// Start sentinel (market anomaly detection)
	if a.config.EnableSentinel {
		a.sentinel = NewSentinel(a.config.WatchSymbols, a.handleSignal, a.logger)
		a.sentinel.Start()
		a.logger.Info("sentinel started", "symbols", a.config.WatchSymbols)
	}

	// Start brain (proactive intelligence)
	a.brain = NewBrain(a, a.logger)
	if a.config.EnableNews {
		a.brain.StartNewsScan(5 * time.Minute)
		a.logger.Info("news scanner started")
	}
	if a.config.EnableBriefs {
		a.brain.StartMarketBriefs(a.config.BriefTimes)
		a.logger.Info("market briefs enabled", "hours", a.config.BriefTimes)
	}

	// Start scheduler
	a.scheduler.Start(context.Background())
	a.logger.Info("scheduler started")

	a.logger.Info("NOFXi agent is online 🚀")
}

// Stop stops all agent services.
func (a *Agent) Stop() {
	if a.sentinel != nil {
		a.sentinel.Stop()
	}
	if a.brain != nil {
		a.brain.Stop()
	}
	a.scheduler.Stop()
	a.logger.Info("NOFXi agent stopped")
}

// HandleMessage processes a user message and returns a response.
// This is the main entry point for Telegram/Web interaction.
func (a *Agent) HandleMessage(ctx context.Context, userID int64, text string) (string, error) {
	// Extract language from prefix [lang:xx]
	lang := a.config.Language
	if strings.HasPrefix(text, "[lang:") {
		if end := strings.Index(text, "] "); end > 0 {
			lang = text[6:end]
			text = text[end+2:]
		}
	}

	a.logger.Info("agent message", "user_id", userID, "text", text)

	intent := a.router.Route(text)

	switch intent.Type {
	case IntentHelp:
		return a.msg(lang, "help"), nil
	case IntentStatus:
		return a.handleStatus(lang), nil
	case IntentQuery:
		return a.handleQuery(lang, intent)
	case IntentAnalyze:
		return a.handleAnalyze(ctx, lang, intent)
	case IntentTrade:
		return a.handleTrade(lang, intent)
	case IntentWatch:
		return a.handleWatch(lang, intent), nil
	case IntentStrategy:
		return a.handleStrategyCmd(lang, intent), nil
	default:
		return a.handleChat(ctx, lang, userID, text)
	}
}

// --- Handlers using NOFX core ---

func (a *Agent) handleStatus(L string) string {
	traderCount := 0
	runningCount := 0
	if a.traderManager != nil {
		all := a.traderManager.GetAllTraders()
		traderCount = len(all)
		for _, t := range all {
			status := t.GetStatus()
			if isRunning, ok := status["is_running"].(bool); ok && isRunning {
				runningCount++
			}
		}
	}
	watchCount := 0
	if a.sentinel != nil {
		watchCount = a.sentinel.SymbolCount()
	}

	return fmt.Sprintf(a.msg(L, "status"),
		runningCount, traderCount, watchCount, time.Now().Format("2006-01-02 15:04:05"))
}

func (a *Agent) handleQuery(L string, intent Intent) (string, error) {
	raw := strings.ToLower(intent.Raw)

	// Get live data from NOFX trader manager
	if a.traderManager == nil {
		return a.msg(L, "no_traders"), nil
	}

	// List all positions across all traders
	if strings.Contains(raw, "position") || strings.Contains(raw, "持仓") {
		return a.queryAllPositions(L)
	}
	if strings.Contains(raw, "balance") || strings.Contains(raw, "余额") {
		return a.queryAllBalances(L)
	}
	if strings.Contains(raw, "trader") || strings.Contains(raw, "交易员") {
		return a.queryTraders(L, nil)
	}

	return a.queryAllPositions(L)
}

func (a *Agent) queryAllPositions(L string) (string, error) {
	traders := a.traderManager.GetAllTraders()
	if len(traders) == 0 {
		return a.msg(L, "no_traders"), nil
	}

	var sb strings.Builder
	sb.WriteString(a.msg(L, "positions_header"))
	totalPnL := 0.0
	hasPosition := false

	for id, t := range traders {
		positions, err := t.GetPositions()
		if err != nil {
			continue
		}
		for _, p := range positions {
			size := toFloat(p["size"])
			if size == 0 {
				continue
			}
			hasPosition = true
			pnl := toFloat(p["unrealizedPnl"])
			e := "🟢"
			if pnl < 0 {
				e = "🔴"
			}
			sb.WriteString(fmt.Sprintf("%s *%s* %s\n   Entry: $%.4f → $%.4f | P/L: $%.2f\n   Trader: %s\n\n",
				e, p["symbol"], p["side"],
				toFloat(p["entryPrice"]), toFloat(p["markPrice"]), pnl,
				id[:8]))
			totalPnL += pnl
		}
	}

	if !hasPosition {
		return a.msg(L, "no_positions"), nil
	}
	sb.WriteString(fmt.Sprintf(a.msg(L, "total_pnl"), totalPnL))
	return sb.String(), nil
}

func (a *Agent) queryAllBalances(L string) (string, error) {
	traders := a.traderManager.GetAllTraders()
	if len(traders) == 0 {
		return a.msg(L, "no_traders"), nil
	}

	var sb strings.Builder
	sb.WriteString(a.msg(L, "balance_header"))

	for id, t := range traders {
		info, err := t.GetAccountInfo()
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("*%s* (%s)\n   Total: $%.2f | Available: $%.2f | P/L: $%.2f\n\n",
			t.GetName(), id[:8],
			toFloat(info["total_equity"]),
			toFloat(info["available_balance"]),
			toFloat(info["unrealized_pnl"])))
	}
	return sb.String(), nil
}

func (a *Agent) queryTraders(L string, _ interface{}) (string, error) {
	traders := a.traderManager.GetAllTraders()
	if len(traders) == 0 {
		return a.msg(L, "no_traders"), nil
	}

	var sb strings.Builder
	sb.WriteString(a.msg(L, "traders_header"))

	for id, t := range traders {
		s := t.GetStatus()
		running, _ := s["is_running"].(bool)
		icon := "⏹"
		if running {
			icon = "▶️"
		}
		sb.WriteString(fmt.Sprintf("%s *%s* `%s`\n   Exchange: %s | AI: %s\n\n",
			icon, t.GetName(), id[:8], t.GetExchange(), t.GetAIModel()))
	}
	return sb.String(), nil
}

func toFloat(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func (a *Agent) handleAnalyze(ctx context.Context, L string, intent Intent) (string, error) {
	symbol := "BTCUSDT"
	if d, ok := intent.Params["detail"]; ok && d != "" {
		symbol = strings.ToUpper(strings.TrimSpace(d))
		if !strings.HasSuffix(symbol, "USDT") {
			symbol += "USDT"
		}
	}

	// Use NOFX's market data — full technical indicators
	md, err := market.Get(symbol)
	if err != nil {
		a.logger.Error("get market data", "symbol", symbol, "error", err)
		if L == "zh" {
			return fmt.Sprintf("🔍 *%s 分析*\n\n⚠️ 市场数据暂不可用，请稍后再试。", strings.TrimSuffix(symbol, "USDT")), nil
		}
		return fmt.Sprintf("🔍 *%s Analysis*\n\n⚠️ Market data unavailable. Try again later.", strings.TrimSuffix(symbol, "USDT")), nil
	}

	// Build rich analysis prompt with real data
	prompt := buildAnalysisPrompt(symbol, md, L)

	// Use AI to analyze the real market data
	if a.aiClient != nil {
		resp, err := a.aiClient.CallWithMessages(a.msg(L, "system_prompt"), prompt)
		if err == nil && resp != "" {
			return fmt.Sprintf(a.msg(L, "analysis_header"), strings.TrimSuffix(symbol, "USDT")) + "\n\n" + resp, nil
		}
		a.logger.Error("AI analysis failed", "error", err)
	}

	// Fallback: format raw data directly
	header := fmt.Sprintf(a.msg(L, "analysis_header"), strings.TrimSuffix(symbol, "USDT"))
	return header + "\n\n" + market.Format(md), nil
}

func buildAnalysisPrompt(symbol string, md *market.Data, L string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Analyze %s for trading.\n\n", symbol))
	sb.WriteString(fmt.Sprintf("Current Price: $%.4f\n", md.CurrentPrice))
	sb.WriteString(fmt.Sprintf("1h Change: %.2f%%\n", md.PriceChange1h))
	sb.WriteString(fmt.Sprintf("4h Change: %.2f%%\n", md.PriceChange4h))
	sb.WriteString(fmt.Sprintf("EMA20: %.4f\n", md.CurrentEMA20))
	sb.WriteString(fmt.Sprintf("MACD: %.4f\n", md.CurrentMACD))
	sb.WriteString(fmt.Sprintf("RSI7: %.2f\n", md.CurrentRSI7))
	sb.WriteString(fmt.Sprintf("Funding Rate: %.4f%%\n", md.FundingRate*100))
	if md.OpenInterest != nil {
		sb.WriteString(fmt.Sprintf("OI: %.0f (avg: %.0f)\n", md.OpenInterest.Latest, md.OpenInterest.Average))
	}
	if L == "zh" {
		sb.WriteString("\n请用中文给出交易建议，包括方向、入场价、止损、止盈。简洁专业。")
	} else {
		sb.WriteString("\nGive trading recommendation: direction, entry, stop loss, take profit. Be concise and professional.")
	}
	return sb.String()
}



func (a *Agent) handleTrade(L string, intent Intent) (string, error) {
	action := strings.ToLower(intent.Params["action"])
	detail := intent.Params["detail"]

	traders := a.traderManager.GetAllTraders()
	if len(traders) == 0 {
		return a.msg(L, "no_traders"), nil
	}

	// Parse symbol and quantity
	parts := strings.Fields(detail)
	if len(parts) < 1 {
		return a.msg(L, "trade_usage"), nil
	}

	symbol := strings.ToUpper(parts[0])
	if !strings.HasSuffix(symbol, "USDT") {
		symbol += "USDT"
	}

	qty := 0.0
	if len(parts) >= 2 {
		q, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return fmt.Sprintf(a.msg(L, "invalid_qty"), parts[1]), nil
		}
		qty = q
	}

	leverage := 1
	if len(parts) >= 3 {
		if l, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(parts[2]), "x")); err == nil {
			leverage = l
		}
	}

	// Find a running trader that can execute
	var traderID string
	for id, t := range traders {
		s := t.GetStatus()
		if running, ok := s["is_running"].(bool); ok && running {
			traderID = id
			break
		}
	}
	if traderID == "" {
		return a.msg(L, "no_running_trader"), nil
	}

	_ = qty
	_ = leverage
	_ = action

	// For now, acknowledge but don't execute (safety)
	if L == "zh" {
		return fmt.Sprintf("⚡ *交易请求*\n\n• 操作: %s\n• 交易对: %s\n• 数量: %.6f\n• 杠杆: %dx\n• Trader: %s\n\n⚠️ 自动执行功能开发中。请在 Web UI 中操作。",
			strings.ToUpper(action), symbol, qty, leverage, traderID[:8]), nil
	}
	return fmt.Sprintf("⚡ *Trade Request*\n\n• Action: %s\n• Symbol: %s\n• Qty: %.6f\n• Leverage: %dx\n• Trader: %s\n\n⚠️ Auto-execution coming soon. Please use Web UI.",
		strings.ToUpper(action), symbol, qty, leverage, traderID[:8]), nil
}

func (a *Agent) handleWatch(L string, intent Intent) string {
	parts := strings.Fields(intent.Raw)
	if len(parts) < 2 {
		if a.sentinel == nil {
			return a.msg(L, "sentinel_off")
		}
		return a.sentinel.FormatWatchlist(L)
	}

	cmd := strings.ToLower(parts[0])
	symbol := strings.ToUpper(parts[1])
	if !strings.HasSuffix(symbol, "USDT") {
		symbol += "USDT"
	}

	if a.sentinel == nil {
		return a.msg(L, "sentinel_off")
	}

	switch cmd {
	case "/watch":
		a.sentinel.AddSymbol(symbol)
		if L == "zh" {
			return fmt.Sprintf("👁️ 已添加 *%s* 到监控列表", symbol)
		}
		return fmt.Sprintf("👁️ Now watching *%s*", symbol)
	case "/unwatch":
		a.sentinel.RemoveSymbol(symbol)
		if L == "zh" {
			return fmt.Sprintf("🚫 已移除 *%s*", symbol)
		}
		return fmt.Sprintf("🚫 Removed *%s* from watchlist", symbol)
	}
	return ""
}

func (a *Agent) handleStrategyCmd(L string, intent Intent) string {
	parts := strings.Fields(intent.Raw)
	if len(parts) < 2 {
		result, _ := a.queryTraders(L, nil)
		return result
	}
	if L == "zh" {
		return "🤖 策略管理请使用 Web UI (http://localhost:8080)"
	}
	return "🤖 Use Web UI for strategy management (http://localhost:8080)"
}

func (a *Agent) handleChat(ctx context.Context, L string, userID int64, text string) (string, error) {
	if a.aiClient == nil {
		if L == "zh" {
			return "🤖 AI 模型未配置。请在 Web UI (:8080) 中创建 Trader 并配置 AI 模型。\n\n命令仍然可用:\n• `/analyze BTC` — 市场分析\n• `/positions` — 查看持仓\n• `/status` — 系统状态", nil
		}
		return "🤖 AI model not configured. Create a Trader in Web UI (:8080) first.\n\nCommands still work:\n• `/analyze BTC`\n• `/positions`\n• `/status`", nil
	}

	systemPrompt := a.msg(L, "system_prompt")

	// If user seems to be asking about a specific coin, enrich with real data
	enrichment := ""
	for _, sym := range []string{"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE"} {
		if strings.Contains(strings.ToUpper(text), sym) {
			md, err := market.Get(sym + "USDT")
			if err == nil {
				enrichment += fmt.Sprintf("\n\n[Real-time %s data]\nPrice: $%.4f | 1h: %.2f%% | 4h: %.2f%% | RSI7: %.1f | EMA20: %.4f | MACD: %.6f | Funding: %.4f%%",
					sym, md.CurrentPrice, md.PriceChange1h, md.PriceChange4h, md.CurrentRSI7, md.CurrentEMA20, md.CurrentMACD, md.FundingRate*100)
			}
			break
		}
	}

	userPrompt := text
	if enrichment != "" {
		userPrompt = text + enrichment
	}

	resp, err := a.aiClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("AI: %w", err)
	}
	return resp, nil
}

func (a *Agent) handleSignal(sig Signal) {
	if a.brain != nil {
		a.brain.HandleSignal(sig)
	}
}

func (a *Agent) notifyAll(text string) {
	// Notify via Telegram (using existing NOFX telegram system)
	if a.NotifyFunc != nil {
		a.NotifyFunc(0, text)
	}
}
