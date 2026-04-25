// Package agent implements the NOFXi Agent Core.
//
// Architecture: ALL user messages go to the LLM. The LLM understands intent
// and calls tools to execute actions. No regex routing, no pattern matching.
// The LLM IS the brain — just like how OpenClaw works.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/manager"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

type Agent struct {
	traderManager *manager.TraderManager
	store         *store.Store
	aiClient      mcp.AIClient
	config        *Config
	sentinel      *Sentinel
	brain         *Brain
	scheduler     *Scheduler
	logger        *slog.Logger
	history       *chatHistory
	pending       *pendingTrades
	stopCh        chan struct{} // signals background goroutines to stop
	stopOnce      sync.Once
	NotifyFunc    func(userID int64, text string) error
}

type Config struct {
	Language       string   `json:"language"`
	WatchSymbols   []string `json:"watch_symbols"`
	EnableBriefs   bool     `json:"enable_briefs"`
	EnableNews     bool     `json:"enable_news"`
	EnableSentinel bool     `json:"enable_sentinel"`
	BriefTimes     []int    `json:"brief_times"`
}

func DefaultConfig() *Config {
	return &Config{
		Language: "zh", WatchSymbols: []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"},
		EnableBriefs: true, EnableNews: true, EnableSentinel: true, BriefTimes: []int{8, 20},
	}
}

func New(tm *manager.TraderManager, st *store.Store, cfg *Config, logger *slog.Logger) *Agent {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Agent{traderManager: tm, store: st, config: cfg, logger: logger, history: newChatHistory(100), pending: newPendingTrades(), stopCh: make(chan struct{})}
}

func (a *Agent) SetAIClient(c mcp.AIClient) { a.aiClient = c }

func (a *Agent) ensureHistory() {
	if a.history == nil {
		a.history = newChatHistory(100)
	}
}

func (a *Agent) log() *slog.Logger {
	if a != nil && a.logger != nil {
		return a.logger
	}
	return slog.Default()
}

func (a *Agent) EnsureAIClient() {
	a.ensureAIClientForStoreUser("default")
}

func (a *Agent) ensureAIClientForStoreUser(storeUserID string) {
	if storeUserID == "" {
		storeUserID = "default"
	}
	if a.store != nil {
		if client, modelName, ok := a.loadAIClientFromStoreUser(storeUserID); ok {
			a.aiClient = client
			a.log().Info("agent AI client ready", "store_user_id", storeUserID, "model", modelName)
			return
		}
	}
	if a.aiClient != nil {
		a.log().Warn("clearing stale AI client for store user", "store_user_id", storeUserID)
		a.aiClient = nil
	}
	a.log().Warn("no AI client — agent will have limited capabilities", "store_user_id", storeUserID)
}

func (a *Agent) loadAIClientFromStoreUser(storeUserID string) (mcp.AIClient, string, bool) {
	if a.store == nil {
		a.log().Warn("cannot load AI client: store unavailable", "store_user_id", storeUserID)
		return nil, "", false
	}

	if storeUserID == "" {
		storeUserID = "default"
	}

	model, err := a.store.AIModel().GetDefault(storeUserID)
	if err != nil || model == nil {
		a.log().Warn("no enabled AI model found for store user", "store_user_id", storeUserID, "error", err)
		return nil, "", false
	}

	a.log().Info(
		"agent selected AI model config",
		"store_user_id", storeUserID,
		"model_id", model.ID,
		"provider", model.Provider,
		"enabled", model.Enabled,
		"has_api_key", len(model.APIKey) > 0,
		"custom_api_url", strings.TrimSpace(model.CustomAPIURL),
		"custom_model_name", strings.TrimSpace(model.CustomModelName),
	)

	apiKey := string(model.APIKey)
	customAPIURL := strings.TrimSpace(model.CustomAPIURL)
	modelName := strings.TrimSpace(model.CustomModelName)
	provider := strings.ToLower(strings.TrimSpace(model.Provider))

	// Use the provider registry for providers like claw402 that have their own
	// client implementation (x402 payment, custom auth, etc.).
	if client := mcp.NewAIClientByProvider(provider); client != nil {
		if modelName == "" {
			modelName = model.ID
		}
		client.SetAPIKey(apiKey, customAPIURL, modelName)
		return client, modelName, true
	}

	customAPIURL, modelName = resolveModelRuntimeConfig(provider, customAPIURL, modelName, model.ID)
	if apiKey == "" || customAPIURL == "" {
		a.log().Warn(
			"enabled AI model is incomplete",
			"store_user_id", storeUserID,
			"model_id", model.ID,
			"provider", model.Provider,
			"has_api_key", apiKey != "",
			"has_custom_api_url", customAPIURL != "",
		)
		return nil, "", false
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	client := mcp.NewClient(mcp.WithHTTPClient(httpClient))
	name := modelName
	client.SetAPIKey(apiKey, customAPIURL, name)
	return client, name, true
}

func resolveModelRuntimeConfig(provider, customAPIURL, customModelName, fallbackModelID string) (string, string) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	customAPIURL = strings.TrimSpace(customAPIURL)
	customModelName = strings.TrimSpace(customModelName)
	fallbackModelID = strings.TrimSpace(fallbackModelID)

	type providerDefaults struct {
		url   string
		model string
	}
	defaults := map[string]providerDefaults{
		"deepseek": {url: "https://api.deepseek.com/v1", model: "deepseek-chat"},
		"qwen":     {url: "https://dashscope.aliyuncs.com/compatible-mode/v1", model: "qwen3-max"},
		"openai":   {url: "https://api.openai.com/v1", model: "gpt-5.2"},
		"claude":   {url: "https://api.anthropic.com/v1", model: "claude-opus-4-6"},
		"gemini":   {url: "https://generativelanguage.googleapis.com/v1beta/openai", model: "gemini-3-pro-preview"},
		"grok":     {url: "https://api.x.ai/v1", model: "grok-3-latest"},
		"kimi":     {url: "https://api.moonshot.ai/v1", model: "moonshot-v1-auto"},
		"minimax":  {url: "https://api.minimax.chat/v1", model: "MiniMax-M2.5"},
	}

	if customAPIURL == "" {
		if cfg, ok := defaults[provider]; ok {
			customAPIURL = cfg.url
		}
	}
	if customModelName == "" {
		if cfg, ok := defaults[provider]; ok {
			customModelName = cfg.model
		}
	}
	if customModelName == "" {
		customModelName = fallbackModelID
	}
	return customAPIURL, customModelName
}

func (a *Agent) Start() {
	a.logger.Info("starting NOFXi agent...")
	a.EnsureAIClient()

	if a.config.EnableSentinel {
		a.sentinel = NewSentinel(a.config.WatchSymbols, a.handleSignal, a.logger)
		a.sentinel.Start()
	}
	a.brain = NewBrain(a, a.logger)
	if a.config.EnableNews {
		a.brain.StartNewsScan(5 * time.Minute)
	}
	if a.config.EnableBriefs {
		a.brain.StartMarketBriefs(a.config.BriefTimes)
	}
	a.scheduler = NewScheduler(a, a.logger)
	a.scheduler.Start(context.Background())

	a.logger.Info("NOFXi agent is online 🚀")
}

func (a *Agent) Stop() {
	// Signal all background goroutines (e.g. chat-history-cleanup) to exit.
	a.stopOnce.Do(func() { close(a.stopCh) })
	if a.sentinel != nil {
		a.sentinel.Stop()
	}
	if a.brain != nil {
		a.brain.Stop()
	}
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
}

// HandleMessage — the core. Everything goes through the LLM.
func (a *Agent) HandleMessage(ctx context.Context, userID int64, text string) (string, error) {
	a.EnsureAIClient()
	return a.handleMessageForStoreUser(ctx, "default", userID, text)
}

// HandleMessageForStoreUser is like HandleMessage but stores setup artifacts
// (exchange/model) under the provided authenticated store user ID.
func (a *Agent) HandleMessageForStoreUser(ctx context.Context, storeUserID string, userID int64, text string) (string, error) {
	return a.handleMessageForStoreUser(ctx, storeUserID, userID, text)
}

func (a *Agent) handleMessageForStoreUser(ctx context.Context, storeUserID string, userID int64, text string) (string, error) {
	a.ensureAIClientForStoreUser(storeUserID)

	lang := a.config.Language
	if strings.HasPrefix(text, "[lang:") {
		if end := strings.Index(text, "] "); end > 0 {
			lang = text[6:end]
			text = text[end+2:]
		}
	}

	a.logger.Info("message", "user_id", userID, "text", text)

	// Only keep a tiny command surface outside the planner.
	if text == "/status" {
		return a.handleStatus(lang), nil
	}
	if text == "/clear" {
		a.history.Clear(userID)
		a.clearTaskState(userID)
		a.clearExecutionState(userID)
		if lang == "zh" {
			return "🧹 对话记忆已清除。", nil
		}
		return "🧹 Conversation history cleared.", nil
	}
	if reply, handled := a.handleTradeConfirmation(ctx, userID, text, lang); handled {
		return reply, nil
	}

	// Everything else goes through the planner and tool system.
	return a.thinkAndAct(ctx, storeUserID, userID, lang, text)
}

// HandleMessageStream is like HandleMessage but streams the final LLM response via SSE.
// onEvent is called with (eventType, data) — see StreamEvent* constants.
// Non-streamable responses (commands, trade confirmations) return immediately without events.
func (a *Agent) HandleMessageStream(ctx context.Context, userID int64, text string, onEvent func(event, data string)) (string, error) {
	a.EnsureAIClient()
	return a.handleMessageStreamForStoreUser(ctx, "default", userID, text, onEvent)
}

// HandleMessageStreamForStoreUser mirrors HandleMessageForStoreUser for SSE responses.
func (a *Agent) HandleMessageStreamForStoreUser(ctx context.Context, storeUserID string, userID int64, text string, onEvent func(event, data string)) (string, error) {
	return a.handleMessageStreamForStoreUser(ctx, storeUserID, userID, text, onEvent)
}

func (a *Agent) handleMessageStreamForStoreUser(ctx context.Context, storeUserID string, userID int64, text string, onEvent func(event, data string)) (string, error) {
	a.ensureAIClientForStoreUser(storeUserID)

	lang := a.config.Language
	if strings.HasPrefix(text, "[lang:") {
		if end := strings.Index(text, "] "); end > 0 {
			lang = text[6:end]
			text = text[end+2:]
		}
	}

	a.logger.Info("message (stream)", "user_id", userID, "text", text)

	if text == "/status" {
		return a.handleStatus(lang), nil
	}
	if text == "/clear" {
		a.history.Clear(userID)
		a.clearTaskState(userID)
		a.clearExecutionState(userID)
		if lang == "zh" {
			return "🧹 对话记忆已清除。", nil
		}
		return "🧹 Conversation history cleared.", nil
	}
	if reply, handled := a.handleTradeConfirmation(ctx, userID, text, lang); handled {
		if onEvent != nil {
			onEvent(StreamEventDelta, reply)
		}
		return reply, nil
	}
	return a.thinkAndActStream(ctx, storeUserID, userID, lang, text, onEvent)
}

// StreamEvent types sent via SSE to the frontend.
const (
	StreamEventPlanning     = "planning"
	StreamEventPlan         = "plan"
	StreamEventStepStart    = "step_start"
	StreamEventStepComplete = "step_complete"
	StreamEventReplan       = "replan"
	StreamEventTool         = "tool"  // Tool is being called (shows status to user)
	StreamEventDelta        = "delta" // Text chunk from LLM streaming
	StreamEventDone         = "done"  // Stream complete
	StreamEventError        = "error" // Error occurred
)

// buildSystemPrompt creates the system prompt that makes NOFXi behave like a real agent.
func (a *Agent) buildSystemPrompt(lang string) string {
	// Gather live system state
	traderInfo := a.getTradersSummary()
	watchlist := ""
	if a.sentinel != nil {
		watchlist = a.sentinel.FormatWatchlist(lang)
	}
	skillCatalog := skillCatalogPrompt(lang)

	if lang == "zh" {
		return fmt.Sprintf(`你是 NOFXi，一个专业的 AI 交易 Agent。你不是一个简单的聊天机器人——你是用户的交易伙伴。

## 你的核心能力
1. **市场分析** — 加密货币（BTC/ETH/SOL等）有实时数据，A股/港股/美股/外汇你可以基于知识分析
2. **交易管理** — 查看持仓、余额、交易历史、Trader 状态
3. **策略建议** — 根据用户需求制定交易策略
4. **策略模板管理** — 创建、查看、修改、删除、激活策略模板
5. **风险管理** — 评估风险、建议止损止盈
6. **配置引导** — 用户说"开始配置"时引导配置交易所和AI模型

## 当前系统状态
%s
%s

## 数据说明（极其重要，违反即失职！）
- 加密货币（BTC/ETH等）：交易所实时数据，标注 [Real-time]
- A股/港股/美股：**必须调用 search_stock 工具**获取实时行情。不调工具就没有数据。
- 美股盘前盘后：search_stock 返回的 quote 中 ext_price/ext_change_pct/ext_time
- 外汇/指数期货：当前没有数据源，如实告知

### 铁律：禁止编造任何价格！
- **你的训练数据中的价格全部过时，不可使用**
- **没有通过工具获取的价格 = 你不知道 = 不能说**
- 用户问多只股票的盘前数据？→ 对每只股票调用 search_stock 工具
- 用户问"盘前概览"？→ 调用 search_stock 查主要股票（AAPL、TSLA、NVDA、MSFT、GOOGL、AMZN、META等），用真实数据回答
- **绝对不允许**不调工具就给出具体价格数字（如 $421.85）
- 如果某只股票 search_stock 查不到数据，就说"暂时无法获取该股票数据"
- 指数期货（纳指、标普、道琼斯期货）我们目前没有数据源，直接说"暂不支持指数期货数据"

## 工具使用
你可以调用以下工具来执行操作：
- **search_stock** — 搜索股票（支持中文名、英文名、代码）。当用户提到你不认识的股票时，先用这个工具搜索。
- **execute_trade** — 下单交易（加密货币或美股）。美股：open_long=买入，close_long=卖出。调用后创建待确认订单，用户需回复"确认 trade_xxx"。
- **get_positions** — 查看当前所有持仓（加密货币 + 股票）
- **get_balance** — 查看账户余额
- **get_market_price** — 获取实时价格（加密货币或股票代码）
- **get_exchange_configs / manage_exchange_config** — 查看、新增、修改、删除交易所绑定配置
- **get_model_configs / manage_model_config** — 查看、新增、修改、删除 AI 模型配置
- **get_strategies / manage_strategy** — 查看、新增、修改、删除、激活、复制策略模板
- **manage_trader** — 查看、新增、修改、删除、启动、停止交易员

### 配置、策略与交易员管理规则
- 当用户要求创建、修改、删除、激活、复制策略模板时，优先使用 get_strategies / manage_strategy
- **策略模板本身是独立资源，不默认依赖交易所或 AI 模型**
- 只有当用户要求“运行策略 / 创建交易员 / 把策略部署到账户”时，才需要进一步关联交易所、模型或 trader
- 当用户要求配置交易所、绑定 API Key、修改交易所账户时，优先使用 manage_exchange_config
- 当用户要求配置大模型、设置 API Key、切换模型、修改模型地址时，优先使用 manage_model_config
- 当用户要求创建、修改、删除、启动、停止交易员时，优先使用 manage_trader
- 如果缺少必要字段，先追问缺失信息，再调用工具
- **在这些工具存在时，不要说“系统没有这个能力”**
- 对敏感信息（API Key、Secret、Private Key）只保存，不要在最终回复中完整回显

%s

### 交易安全规则
- 用户明确要求交易时才调用 execute_trade
- 分析和建议不需要调用工具，直接回复即可
- 交易确认信息要清晰展示：品种、方向、数量、杠杆
- 提醒用户确认命令格式

### 数据真实性规则（极其重要！）
- **持仓信息必须且只能通过 get_positions 工具获取**，绝对禁止编造持仓
- **余额信息必须且只能通过 get_balance 工具获取**，绝对禁止编造余额
- 如果用户问持仓但 get_positions 返回空，就说"当前没有持仓"，不要编造
- 如果工具返回 error（如未配置交易所），如实告知用户
- **你不知道用户持有什么股票/币种，除非工具返回了数据**
- 查股票行情 ≠ 用户持有该股票。不要混淆"查价格"和"有持仓"

## 行为准则
- 简洁、专业、有观点。不说废话。
- 用户问什么答什么，不要推销配置。
- 有实时数据时给具体价位，没有时给策略框架和思路。
- **诚实是第一原则** — 不确定就说不确定，没数据就说没数据。绝不编造。
- 用交易相关的 emoji 让回复更直观。
- 用中文回复。

当前时间: %s`, traderInfo, watchlist, skillCatalog, time.Now().Format("2006-01-02 15:04:05"))
	}

	return fmt.Sprintf(`You are NOFXi, a professional AI trading agent. Not a chatbot — a trading partner.

## Capabilities
1. Market analysis — crypto with real-time data, stocks/forex with knowledge
2. Trade management — positions, balance, history, trader status
3. Strategy — build trading strategies based on user needs
4. Strategy template management — create, inspect, update, delete, and activate strategy templates
5. Risk management — assess risk, suggest stop-loss/take-profit
6. Setup — guide exchange/AI configuration when user asks

## Current System State
%s
%s

## Data Notice (CRITICAL — violating this is unacceptable!)
- Crypto (BTC/ETH): Exchange real-time data, marked [Real-time]
- Stocks: You MUST call search_stock tool to get real-time quotes. No tool call = no data.
- US stocks pre/after-hours: ext_price/ext_change_pct/ext_time in search_stock results
- Forex/Index futures: No data source currently — tell user honestly

### ABSOLUTE RULE: NEVER fabricate any price!
- Your training data prices are ALL outdated and MUST NOT be used
- No tool result = you don't know = you cannot state a price
- User asks multiple stocks? → Call search_stock for EACH one
- User asks "pre-market overview"? → Call search_stock for major stocks (AAPL, TSLA, NVDA, MSFT, GOOGL, AMZN, META etc.) and use real data
- NEVER output a specific price number (like $421.85) without a tool having returned it
- If search_stock fails for a stock, say "unable to fetch data for this stock"
- Index futures (NDX, SPX, DJI futures) — we have no data source, say "index futures not supported yet"

## Tools
You can call these tools to take action:
- **search_stock** — Search for stocks by name, ticker, or code. Covers A-share, HK, and US markets. Use when the user mentions an unknown stock.
- **execute_trade** — Place a trade order (crypto or US stocks). For stocks: open_long=buy, close_long=sell. Creates a pending order that requires user confirmation.
- **get_positions** — View all current open positions (crypto + stocks)
- **get_balance** — View account balance and equity
- **get_market_price** — Get real-time price from the exchange (crypto or stock symbol)
- **get_exchange_configs / manage_exchange_config** — View, create, update, and delete exchange bindings
- **get_model_configs / manage_model_config** — View, create, update, and delete AI model bindings
- **get_strategies / manage_strategy** — View, create, update, delete, activate, and duplicate strategy templates
- **manage_trader** — List, create, update, delete, start, and stop traders

### Configuration, Strategy, and Trader Rules
- When the user wants to create, edit, delete, activate, or duplicate a strategy template, prefer get_strategies / manage_strategy
- **A strategy template is an independent asset and does not require exchange or model bindings by default**
- Only ask for exchange/model/trader details when the user wants to run, deploy, or attach a strategy to a trader
- When the user wants to bind or edit an exchange account, prefer manage_exchange_config
- When the user wants to bind or edit an AI model, prefer manage_model_config
- When the user wants to create, edit, delete, start, or stop a trader, prefer manage_trader
- If required fields are missing, ask a focused follow-up question first, then call the tool
- **Do not claim the system lacks these capabilities when the tools exist**
- For secrets such as API keys, secrets, and private keys: store them, but never echo them back in full

%s

### Trade Safety Rules
- Only call execute_trade when user explicitly requests a trade
- Analysis and advice don't need tools — just reply directly
- Show trade details clearly: symbol, direction, quantity, leverage
- Remind user of the confirmation command format

### Data Truthfulness Rules (CRITICAL!)
- **Position data MUST come from get_positions tool only** — NEVER fabricate positions
- **Balance data MUST come from get_balance tool only** — NEVER fabricate balances
- If get_positions returns empty, say "no open positions" — do NOT make up holdings
- If a tool returns an error (e.g. no exchange configured), tell the user honestly
- **You do NOT know what the user holds unless a tool tells you**
- Checking a stock price ≠ user owns that stock. Never confuse "quote lookup" with "holding"

## Behavior
- Concise, professional, opinionated. No fluff.
- Answer what's asked. Don't push setup.
- With real-time data: give specific levels. Without: give strategy frameworks.
- **Honesty is rule #1** — uncertain = say uncertain, no data = say no data.
- Use trading emojis.

Current time: %s`, traderInfo, watchlist, skillCatalog, time.Now().Format("2006-01-02 15:04:05"))
}

// gatherContext collects real-time market data relevant to the user's message.
func (a *Agent) gatherContext(text string) string {
	var parts []string
	upper := strings.ToUpper(text)

	// Crypto — detect symbols dynamically
	// 1. Check known popular symbols (fast path)
	// 2. Extract any "XXXUSDT" pattern from text (catches arbitrary pairs)
	knownSymbols := []string{
		"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE", "ADA", "AVAX", "DOT", "LINK",
		"PEPE", "SHIB", "ARB", "OP", "SUI", "APT", "SEI", "TIA", "JUP", "WIF",
		"NEAR", "ATOM", "FTM", "MATIC", "INJ", "RENDER", "FET", "TAO", "WLD",
		"AAVE", "UNI", "LDO", "MKR", "CRV", "PENDLE", "ENA", "ONDO", "TRUMP",
	}
	matched := make(map[string]bool)
	for _, sym := range knownSymbols {
		if strings.Contains(upper, sym) {
			matched[sym] = true
		}
	}
	// Also extract "XXXUSDT" patterns for coins not in the known list
	for _, word := range strings.Fields(upper) {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if strings.HasSuffix(word, "USDT") && len(word) > 4 && len(word) <= 15 {
			sym := strings.TrimSuffix(word, "USDT")
			if len(sym) >= 2 && len(sym) <= 10 {
				matched[sym] = true
			}
		}
	}
	// Collect and sort matched symbols for deterministic selection
	sortedSymbols := make([]string, 0, len(matched))
	for sym := range matched {
		sortedSymbols = append(sortedSymbols, sym)
	}
	sort.Strings(sortedSymbols)

	// Cap at 5 symbols to avoid slow context gathering
	count := 0
	for _, sym := range sortedSymbols {
		if count >= 5 {
			break
		}
		md, err := market.Get(sym + "USDT")
		if err == nil && md.CurrentPrice > 0 {
			parts = append(parts, fmt.Sprintf("[%s/USDT Real-time]\nPrice: $%.4f | 1h: %+.2f%% | 4h: %+.2f%% | RSI7: %.1f | EMA20: %.4f | MACD: %.6f | Funding: %.4f%%",
				sym, md.CurrentPrice, md.PriceChange1h, md.PriceChange4h, md.CurrentRSI7, md.CurrentEMA20, md.CurrentMACD, md.FundingRate*100))
			count++
		}
	}

	// A-share / stocks — only call Sina API when text likely references stocks.
	// Skip for purely crypto conversations to avoid unnecessary external API calls.
	if looksLikeStockQuery(text) {
		stockCode, stockName := resolveStockCodeDynamic(text)
		if stockCode != "" {
			quote, err := fetchStockQuote(stockCode)
			if err == nil && quote.Price > 0 {
				parts = append(parts, fmt.Sprintf("[%s(%s) Real-time A-share Data]\n%s", quote.Name, quote.Code, formatStockQuote(quote)))
			} else if err != nil {
				a.logger.Error("fetch stock quote", "code", stockCode, "name", stockName, "error", err)
			}
		}
	}

	// Trader positions
	if a.traderManager != nil {
		for _, t := range a.traderManager.GetAllTraders() {
			positions, err := t.GetPositions()
			if err != nil {
				continue
			}
			for _, p := range positions {
				size := toFloat(p["size"])
				if size == 0 {
					continue
				}
				parts = append(parts, fmt.Sprintf("[Position] %s %s: size=%.4f entry=$%.4f mark=$%.4f pnl=$%.2f",
					p["symbol"], p["side"], size, toFloat(p["entryPrice"]), toFloat(p["markPrice"]), toFloat(p["unrealizedPnl"])))
			}
		}
	}

	return strings.Join(parts, "\n")
}

func (a *Agent) getTradersSummary() string {
	if a.traderManager == nil {
		return "Traders: none configured"
	}
	traders := a.traderManager.GetAllTraders()
	if len(traders) == 0 {
		return "Traders: none configured"
	}

	var lines []string
	for id, t := range traders {
		s := t.GetStatus()
		running, _ := s["is_running"].(bool)
		status := "stopped"
		if running {
			status = "running"
		}
		tid := id
		if len(tid) > 8 {
			tid = tid[:8]
		}
		lines = append(lines, fmt.Sprintf("• %s [%s] %s | %s", t.GetName(), tid, status, t.GetExchange()))
	}
	return "Traders:\n" + strings.Join(lines, "\n")
}

func (a *Agent) handleStatus(L string) string {
	tc, rc := 0, 0
	if a.traderManager != nil {
		all := a.traderManager.GetAllTraders()
		tc = len(all)
		for _, t := range all {
			if s := t.GetStatus(); s["is_running"] == true {
				rc++
			}
		}
	}
	wc := 0
	if a.sentinel != nil {
		wc = a.sentinel.SymbolCount()
	}
	ai := "❌"
	if a.aiClient != nil {
		ai = "✅"
	}
	return fmt.Sprintf(a.msg(L, "status"), rc, tc, wc, ai, time.Now().Format("2006-01-02 15:04:05"))
}

// noAIFallback — when no AI is available, still try to be useful.
func (a *Agent) noAIFallback(lang, text string) (string, error) {
	upper := strings.ToUpper(text)

	// Try to provide market data directly
	for _, sym := range []string{"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE"} {
		if strings.Contains(upper, sym) {
			md, err := market.Get(sym + "USDT")
			if err == nil {
				return fmt.Sprintf("📊 *%s/USDT*\n\n%s\n\n💡 配置 AI 模型后我能给你更深度的分析。发送 *开始配置* 开始。", sym, market.Format(md)), nil
			}
		}
	}

	// Check if asking about positions/balance
	if strings.Contains(text, "持仓") || strings.Contains(upper, "POSITION") {
		return a.queryPositionsDirect(lang)
	}
	if strings.Contains(text, "余额") || strings.Contains(upper, "BALANCE") {
		return a.queryBalancesDirect(lang)
	}

	if lang == "zh" {
		return "🤖 我是 NOFXi。配置 AI 模型后我就能理解你的任何问题——分析股票、制定策略、管理交易。\n\n现在可用：\n• 加密货币实时行情（试试「BTC」）\n• `/status` 系统状态\n\n发送 *开始配置* 配置 AI 模型。", nil
	}
	return "🤖 I'm NOFXi. Configure an AI model and I can understand anything — analyze stocks, build strategies, manage trades.\n\nAvailable now:\n• Crypto real-time data (try 'BTC')\n• `/status` system status\n\nSend *setup* to configure AI.", nil
}

func (a *Agent) aiServiceFailure(lang string, err error) (string, error) {
	reason := "unknown error"
	if err != nil {
		reason = summarizeObservation(err.Error())
	}
	a.logger.Error("AI service call failed", "error", reason)
	if lang == "zh" {
		return fmt.Sprintf("当前 AI 服务调用失败：%s\n\n这不是“未配置模型”。更可能是模型服务余额不足、接口报错或超时。请检查当前启用模型的 API 状态后再试。", reason), nil
	}
	return fmt.Sprintf("The AI service call failed: %s\n\nThis is not a missing-model issue. The active model provider likely returned an error, timed out, or has insufficient balance. Please check the active model API and try again.", reason), nil
}

func (a *Agent) queryPositionsDirect(L string) (string, error) {
	if a.traderManager == nil {
		return a.msg(L, "no_traders"), nil
	}
	var sb strings.Builder
	sb.WriteString("📊 *Positions*\n\n")
	hasAny := false
	for id, t := range a.traderManager.GetAllTraders() {
		positions, err := t.GetPositions()
		if err != nil {
			continue
		}
		for _, p := range positions {
			size := toFloat(p["size"])
			if size == 0 {
				continue
			}
			hasAny = true
			pnl := toFloat(p["unrealizedPnl"])
			e := "🟢"
			if pnl < 0 {
				e = "🔴"
			}
			tid := id
			if len(tid) > 8 {
				tid = tid[:8]
			}
			sb.WriteString(fmt.Sprintf("%s *%s* %s — $%.2f | Trader: %s\n", e, p["symbol"], p["side"], pnl, tid))
		}
	}
	if !hasAny {
		return a.msg(L, "no_positions"), nil
	}
	return sb.String(), nil
}

func (a *Agent) queryBalancesDirect(L string) (string, error) {
	if a.traderManager == nil {
		return a.msg(L, "no_traders"), nil
	}
	var sb strings.Builder
	sb.WriteString("💰 *Balance*\n\n")
	for id, t := range a.traderManager.GetAllTraders() {
		info, err := t.GetAccountInfo()
		if err != nil {
			continue
		}
		tid := id
		if len(tid) > 8 {
			tid = tid[:8]
		}
		sb.WriteString(fmt.Sprintf("*%s* (%s): $%.2f\n", t.GetName(), tid, toFloat(info["total_equity"])))
	}
	return sb.String(), nil
}

func (a *Agent) handleSignal(sig Signal) {
	if a.brain != nil {
		a.brain.HandleSignal(sig)
	}
}

func (a *Agent) notifyAll(text string) {
	if a.NotifyFunc != nil {
		a.NotifyFunc(0, text)
	}
}

// looksLikeStockQuery returns true if the text likely references stocks rather
// than being a pure crypto/general query. This avoids hitting the Sina search
// API on every single message (saves ~200ms latency + external API call).
func looksLikeStockQuery(text string) bool {
	upper := strings.ToUpper(text)

	// Check for known stock-related Chinese keywords
	stockKeywords := []string{
		"股", "A股", "港股", "美股", "股票", "涨停", "跌停", "大盘",
		"沪指", "深指", "恒指", "纳指", "标普", "道琼斯",
		"茅台", "比亚迪", "宁德", "腾讯", "阿里", "美团", "小米",
		"京东", "百度", "苹果", "特斯拉", "英伟达", "微软", "谷歌",
		"盘前", "盘后", "开盘", "收盘", "涨幅", "跌幅",
	}
	for _, kw := range stockKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	// Check for US stock ticker patterns (1-5 uppercase letters not matching crypto)
	for _, word := range strings.Fields(upper) {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if len(word) >= 1 && len(word) <= 5 {
			allLetter := true
			for _, c := range word {
				if c < 'A' || c > 'Z' {
					allLetter = false
					break
				}
			}
			if allLetter {
				// Check if it's in the known US ticker map
				if _, ok := usTickerMap[word]; ok {
					return true
				}
			}
		}
	}

	// Check for 6-digit A-share codes or 5-digit HK codes
	for _, w := range strings.Fields(text) {
		w = strings.TrimSpace(w)
		if len(w) == 5 || len(w) == 6 {
			if _, err := strconv.Atoi(w); err == nil {
				return true
			}
		}
	}

	return false
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
	case int32:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}
