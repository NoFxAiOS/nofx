package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/manager"
	"nofx/security"
	"nofx/market"
	"nofx/provider/alpaca"
	"nofx/provider/coinank/coinank_api"
	"nofx/provider/coinank/coinank_enum"
	"nofx/provider/hyperliquid"
	"nofx/provider/twelvedata"
	"nofx/store"
	"nofx/trader"
	"nofx/trader/aster"
	"nofx/trader/binance"
	"nofx/trader/bitget"
	"nofx/trader/bybit"
	"nofx/trader/gate"
	hyperliquidtrader "nofx/trader/hyperliquid"
	"nofx/trader/kucoin"
	"nofx/trader/lighter"
	"nofx/trader/okx"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Server HTTP API server
type Server struct {
	router           *gin.Engine
	traderManager    *manager.TraderManager
	store            *store.Store
	cryptoHandler    *CryptoHandler
	backtestManager  *backtest.Manager
	debateHandler    *DebateHandler
	httpServer       *http.Server
	port             int
	telegramReloadCh chan<- struct{} // signal Telegram bot to reload
}

// NewServer Creates API server
func NewServer(traderManager *manager.TraderManager, st *store.Store, cryptoService *crypto.CryptoService, backtestManager *backtest.Manager, port int) *Server {
	// Set to Release mode (reduce log output)
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Enable CORS
	router.Use(corsMiddleware())

	// Create crypto handler
	cryptoHandler := NewCryptoHandler(cryptoService)

	// Create debate store and handler
	debateStore := store.NewDebateStore(st.GormDB())
	if err := debateStore.InitSchema(); err != nil {
		logger.Errorf("Failed to initialize debate schema: %v", err)
	}
	debateHandler := NewDebateHandler(debateStore, st.Strategy(), st.AIModel())
	debateHandler.SetTraderManager(traderManager)

	s := &Server{
		router:          router,
		traderManager:   traderManager,
		store:           st,
		cryptoHandler:   cryptoHandler,
		backtestManager: backtestManager,
		debateHandler:   debateHandler,
		port:            port,
	}

	// Setup routes
	s.setupRoutes()

	return s
}

// corsMiddleware CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// setupRoutes Setup routes
func (s *Server) setupRoutes() {
	// API route group
	api := s.router.Group("/api")
	{
		// Health check
		api.Any("/health", s.handleHealth)

		// Admin login (used in admin mode, public)

		// System supported models and exchanges (no authentication required)
		s.route(api, "GET", "/supported-models", "List supported AI model providers", s.handleGetSupportedModels)
		s.route(api, "GET", "/supported-exchanges", "List supported exchange types", s.handleGetSupportedExchanges)

		// System config (no authentication required, for frontend to determine admin mode/registration status)
		s.route(api, "GET", "/config", "Get system configuration", s.handleGetSystemConfig)

		// Crypto related endpoints (no authentication required, not exposed to bot)
		api.GET("/crypto/config", s.cryptoHandler.HandleGetCryptoConfig)
		api.GET("/crypto/public-key", s.cryptoHandler.HandleGetPublicKey)
		api.POST("/crypto/decrypt", s.cryptoHandler.HandleDecryptSensitiveData)

		// Public competition data (no authentication required)
		s.route(api, "GET", "/traders", "Public trader list", s.handlePublicTraderList)
		s.route(api, "GET", "/competition", "Public competition data", s.handlePublicCompetition)
		s.route(api, "GET", "/top-traders", "Top traders leaderboard", s.handleTopTraders)
		s.route(api, "GET", "/equity-history", "Equity history for a trader", s.handleEquityHistory)
		s.route(api, "POST", "/equity-history-batch", "Batch equity history for multiple traders", s.handleEquityHistoryBatch)
		s.route(api, "GET", "/traders/:id/public-config", "Public trader configuration", s.handleGetPublicTraderConfig)

		// Market data (no authentication required)
		s.route(api, "GET", "/klines", "Candlestick data (?symbol=&interval=&limit=)", s.handleKlines)
		s.route(api, "GET", "/symbols", "Available trading symbols", s.handleSymbols)

		// Public strategy market (no authentication required)
		s.route(api, "GET", "/strategies/public", "Public strategy market", s.handlePublicStrategies)

		// Authentication related routes (no authentication required)
		s.route(api, "POST", "/register", "Register new user", s.handleRegister)
		s.route(api, "POST", "/login", "User login, returns JWT token", s.handleLogin)
		s.route(api, "POST", "/reset-password", "Reset password", s.handleResetPassword)

		// Routes requiring authentication
		protected := api.Group("/", s.authMiddleware())
		{
			// Logout (add to blacklist)
			s.route(protected, "POST", "/logout", "Logout (blacklist token)", s.handleLogout)

			// User account management
			s.routeWithSchema(protected, "PUT", "/user/password", "Change current user password",
				`Body: {"new_password":"<string, min 8 chars>"}`,
				s.handleChangePassword)

			// Server IP query (requires authentication, for whitelist configuration)
			s.route(protected, "GET", "/server-ip", "Get server public IP (for exchange whitelist)", s.handleGetServerIP)

			// AI trader management
			s.routeWithSchema(protected, "GET", "/my-traders", "List user's traders with status",
				`Returns: [{"trader_id":"<EXACT id — use this as trader_id in all ?trader_id= queries and POST /traders/:id/start|stop>","trader_name":"<string>","is_running":<bool>}]
NOTE: The id field is "trader_id" (NOT "id"). Always read trader_id from this endpoint before querying data.`,
				s.handleTraderList)
			s.routeWithSchema(protected, "GET", "/traders/:id/config", "Get full trader configuration",
				`:id = trader_id from GET /api/my-traders`,
				s.handleGetTraderConfig)
			s.routeWithSchema(protected, "POST", "/traders", "Create a new AI trader",
				`Body: {"name":"<string, required>","ai_model_id":"<EXACT id field from GET /api/models — e.g. 'abc123_deepseek', NOT the provider name 'deepseek'>","exchange_id":"<EXACT id field from GET /api/exchanges — e.g. '05785d3b-841e-...', NOT the type name>","strategy_id":"<EXACT id field from GET /api/strategies>","scan_interval_minutes":<int, default 3, minimum 3>}
IMPORTANT: ai_model_id and exchange_id must be the full "id" value from the Account State, not the provider/type name.`,
				s.handleCreateTrader)
			s.routeWithSchema(protected, "PUT", "/traders/:id", "Update trader configuration",
				`:id = trader_id from GET /api/my-traders
Body: {"name":"<string>","ai_model_id":"<EXACT id from GET /api/models>","exchange_id":"<EXACT id from GET /api/exchanges>","strategy_id":"<EXACT id from GET /api/strategies>","scan_interval_minutes":<int, min 3>,"is_cross_margin":<bool>}
Only include fields you want to change.`,
				s.handleUpdateTrader)
			s.routeWithSchema(protected, "DELETE", "/traders/:id", "Delete trader",
				`:id = trader_id from GET /api/my-traders. Stops and permanently removes the trader and all its data.`,
				s.handleDeleteTrader)
			s.routeWithSchema(protected, "POST", "/traders/:id/start", "Start trader — begins live trading",
				`:id = trader_id from GET /api/my-traders. No request body needed. The trader must have a valid exchange and AI model configured.`,
				s.handleStartTrader)
			s.routeWithSchema(protected, "POST", "/traders/:id/stop", "Stop trader — halts live trading",
				`:id = trader_id from GET /api/my-traders. No request body needed. Gracefully stops the trading loop.`,
				s.handleStopTrader)
			s.routeWithSchema(protected, "PUT", "/traders/:id/prompt", "Override the trader's AI system prompt",
				`Body: {"prompt":"<string — the full custom prompt text>"}`,
				s.handleUpdateTraderPrompt)
			s.routeWithSchema(protected, "POST", "/traders/:id/sync-balance", "Sync account balance from exchange",
				`:id = trader_id from GET /api/my-traders. No request body needed. Refreshes initial_balance from the exchange.`,
				s.handleSyncBalance)
			s.routeWithSchema(protected, "POST", "/traders/:id/close-position", "Force-close an open position",
				`:id = trader_id from GET /api/my-traders.
Body: {"symbol":"<string, e.g. BTCUSDT — must match an open position symbol from GET /api/positions>"}`,
				s.handleClosePosition)
			s.routeWithSchema(protected, "PUT", "/traders/:id/competition", "Toggle competition leaderboard visibility",
				`:id = trader_id from GET /api/my-traders.
Body: {"show_in_competition":<bool>}`,
				s.handleToggleCompetition)
			s.routeWithSchema(protected, "GET", "/traders/:id/grid-risk", "Get grid trading risk info",
				`:id = trader_id from GET /api/my-traders.`,
				s.handleGetGridRiskInfo)

			// AI model configuration
			s.routeWithSchema(protected, "GET", "/models", "List AI model configs",
				`Returns: [{"id":"<EXACT id — use this as ai_model_id when creating/updating a trader>","name":"<display name>","provider":"<short provider name — NOT a valid id>","enabled":<bool>}]
CRITICAL: The "id" field (e.g. "abc123_deepseek") is what you must use for ai_model_id. The "provider" field ("deepseek") is NOT valid as an id.`,
				s.handleGetModelConfigs)
			s.routeWithSchema(protected, "PUT", "/models", "Configure an AI model provider",
				`Body: {"models":{"<model_id>":{"enabled":<bool>,"api_key":"<string>","custom_api_url":"<string, leave empty to use provider default>","custom_model_name":"<string, leave empty to use provider default>"}}}
model_id values: "openai","deepseek","qwen","kimi","grok","gemini","claude"
Defaults when custom fields empty: openai→api.openai.com/v1, deepseek→api.deepseek.com, qwen→dashscope.aliyuncs.com/compatible-mode/v1, kimi→api.moonshot.ai/v1, grok→api.x.ai/v1, gemini→generativelanguage.googleapis.com/v1beta/openai, claude→api.anthropic.com/v1`,
				s.handleUpdateModelConfigs)

			// Exchange configuration
			s.routeWithSchema(protected, "GET", "/exchanges", "List exchange accounts",
				`Returns: [{"id":"<EXACT id — use this as exchange_id when creating/updating a trader>","exchange_type":"<e.g. okx, binance>","account_name":"<user label>","enabled":<bool>}]
CRITICAL: Always use the "id" field for exchange_id. Do not use "exchange_type" as an id.`,
				s.handleGetExchangeConfigs)
			s.routeWithSchema(protected, "POST", "/exchanges", "Create a new exchange account",
				`Body: {"exchange_type":"<string>","account_name":"<string, user label>","enabled":true,"api_key":"<string>","secret_key":"<string>","passphrase":"<string, required for okx/gate/kucoin>"}
exchange_type values: "binance","bybit","okx","bitget","gate","kucoin","indodax" (CEX) | "hyperliquid","aster","lighter" (DEX)
Required fields by exchange:
  binance/bybit/bitget/indodax: api_key + secret_key
  okx/gate/kucoin: api_key + secret_key + passphrase
  hyperliquid: hyperliquid_wallet_addr
  aster: aster_user + aster_signer + aster_private_key
  lighter: lighter_wallet_addr + lighter_private_key + lighter_api_key_private_key + lighter_api_key_index`,
				s.handleCreateExchange)
			s.routeWithSchema(protected, "PUT", "/exchanges", "Update an existing exchange account configuration",
				`Body: {"id":"<EXACT id from GET /api/exchanges>","exchange_type":"<string>","account_name":"<string>","enabled":<bool>,"api_key":"<string>","secret_key":"<string>","passphrase":"<string, for okx/gate/kucoin>"}
Use this to enable/disable an exchange or update API credentials. The "id" field is required to identify which exchange to update.`,
				s.handleUpdateExchangeConfigs)
			s.routeWithSchema(protected, "DELETE", "/exchanges/:id", "Delete exchange account",
				`:id = EXACT id from GET /api/exchanges. Permanently removes the exchange account and disconnects any traders using it.`,
				s.handleDeleteExchange)

			// Telegram bot configuration
			s.routeWithSchema(protected, "GET", "/telegram", "Get Telegram bot configuration",
				`Returns: {"bot_token":"<string>","model_id":"<EXACT id of configured AI model>","chat_id":"<bound Telegram chat id, empty if not bound>"}`,
				s.handleGetTelegramConfig)
			s.routeWithSchema(protected, "POST", "/telegram", "Set Telegram bot token and AI model",
				`Body: {"bot_token":"<string — Telegram BotFather token>","model_id":"<EXACT id from GET /api/models>"}
Both fields are required. After saving, the user must send /start in Telegram to bind their account.`,
				s.handleUpdateTelegramConfig)
			s.routeWithSchema(protected, "POST", "/telegram/model", "Update Telegram bot AI model only",
				`Body: {"model_id":"<EXACT id from GET /api/models>"}`,
				s.handleUpdateTelegramModel)
			s.routeWithSchema(protected, "DELETE", "/telegram/binding", "Unbind Telegram account",
				`No body needed. Clears the Telegram chat_id binding so the user can re-bind with /start.`,
				s.handleUnbindTelegram)

			// Strategy management
			s.routeWithSchema(protected, "GET", "/strategies", "List user's strategies",
				`Returns: [{"id":"<EXACT id — use as strategy_id when creating/updating a trader>","name":"<string>","is_active":<bool>,"is_default":<bool>}]
CRITICAL: Always use the "id" field for strategy_id.`,
				s.handleGetStrategies)
			s.routeWithSchema(protected, "GET", "/strategies/active", "Get the currently active strategy",
				`Returns the strategy marked is_active=true for this user, or the system default. Use this to find which strategy is currently in use.`,
				s.handleGetActiveStrategy)
			s.routeWithSchema(protected, "GET", "/strategies/default-config", "Get default strategy config with all fields and sensible values — use as reference for building configs",
				`No parameters needed. Returns a complete StrategyConfig object with all fields populated with recommended defaults. Read this before building a custom config.`,
				s.handleGetDefaultStrategyConfig)
			s.route(protected, "POST", "/strategies/preview-prompt", "Preview the AI prompt that will be generated from a config", s.handlePreviewPrompt)
			s.route(protected, "POST", "/strategies/test-run", "Test-run strategy AI analysis", s.handleStrategyTestRun)
			s.route(protected, "GET", "/strategies/:id", "Get strategy by ID", s.handleGetStrategy)
			s.routeWithSchema(protected, "POST", "/strategies", "Create a new trading strategy",
				`Body: {"name":"<string, required>","description":"<string, optional>","lang":"zh|en","config":<StrategyConfig object, OPTIONAL — if omitted the system applies complete working defaults automatically (ai500 top coins, all standard indicators, standard risk control)>}
IMPORTANT: For most use cases just POST {"name":"<name>"} — the backend fills everything in. Only include "config" when the user explicitly requests custom settings (specific coins, custom leverage, custom timeframes).

StrategyConfig fields:
  coin_source.source_type: "static"(fixed coin list) | "ai500"(AI top500 ranking) | "oi_top"(OI increasing, suited for long) | "oi_low"(OI decreasing, suited for short) | "mixed"
  coin_source.static_coins: ["BTCUSDT","ETHUSDT"] — only when source_type="static"
  coin_source.use_ai500, ai500_limit: number of coins from AI500 pool (default 10)
  coin_source.use_oi_top/use_oi_low, oi_top_limit/oi_low_limit: OI-based coin selection
  indicators.klines.primary_timeframe: "1m"|"3m"|"5m"|"15m"|"1h"|"4h" — scalping→"5m", trend/swing→"1h"/"4h"
  indicators.klines.primary_count: number of candles (20-100)
  indicators.klines.enable_multi_timeframe: true for trend/swing analysis
  indicators.klines.selected_timeframes: e.g. ["5m","15m","1h","4h"]
  indicators.enable_raw_klines: ALWAYS true (raw OHLCV required)
  indicators.enable_ema: true for trend-following (EMA crossover signals)
  indicators.enable_macd: true for trend + momentum confirmation
  indicators.enable_rsi: true for overbought/oversold, divergence detection
  indicators.enable_boll: true for volatility, range trading, breakout strategies
  indicators.enable_atr: true for volatility measurement and stop-loss sizing
  indicators.enable_volume: ALWAYS true
  indicators.enable_oi: ALWAYS true (open interest data)
  indicators.enable_funding_rate: ALWAYS true
  indicators.ema_periods: [20,50] default, [9,21] for faster signals
  indicators.rsi_periods: [7,14] default
  indicators.atr_periods: [14] default
  indicators.boll_periods: [20] default
  indicators.nofxos_api_key: ALWAYS "cm_568c67eae410d912c54c"
  indicators.enable_quant_data: ALWAYS true
  indicators.enable_quant_oi: ALWAYS true
  indicators.enable_quant_netflow: ALWAYS true
  indicators.enable_oi_ranking: ALWAYS true, oi_ranking_duration:"1h", oi_ranking_limit:10
  indicators.enable_netflow_ranking: ALWAYS true, netflow_ranking_duration:"1h", netflow_ranking_limit:10
  indicators.enable_price_ranking: ALWAYS true, price_ranking_duration:"1h,4h,24h", price_ranking_limit:10
  risk_control.max_positions: max simultaneous positions (1=single coin, 3=diversified, 5=wide)
  risk_control.btc_eth_max_leverage: BTC/ETH leverage (conservative:3-5, moderate:5-10, aggressive:10-20)
  risk_control.altcoin_max_leverage: altcoin leverage (usually lower than BTC leverage)
  risk_control.btc_eth_max_position_value_ratio: max position size as multiple of equity (default 5)
  risk_control.altcoin_max_position_value_ratio: default 1
  risk_control.max_margin_usage: 0.5-0.95 (default 0.9 = use up to 90% margin)
  risk_control.min_position_size: minimum USDT per trade (default 12)
  risk_control.min_risk_reward_ratio: minimum profit/loss ratio required (default 3 = 3:1)
  risk_control.min_confidence: minimum AI confidence to open position (default 75, range 60-90)
  prompt_sections.role_definition: describe the AI's trading persona and goal
  prompt_sections.trading_frequency: guidelines on how often to trade
  prompt_sections.entry_standards: conditions that must align before entering a position
  prompt_sections.decision_process: step-by-step decision-making framework`,
				s.handleCreateStrategy)
			s.routeWithSchema(protected, "PUT", "/strategies/:id", "Update an existing strategy — WORKFLOW: 1) GET /api/strategies/:id first to read current config 2) Merge your changes into the full config 3) PUT with complete merged config 4) GET again to verify saved values",
				`Body: {"name":"<string>","description":"<string>","config":<complete StrategyConfig — same structure as POST /api/strategies>}
IMPORTANT: config is merged with existing values server-side, but always send the complete section you are modifying.
After updating, always GET /api/strategies/:id to verify and show the user actual saved values.`,
				s.handleUpdateStrategy)
			s.routeWithSchema(protected, "DELETE", "/strategies/:id", "Delete strategy",
				`:id = EXACT id from GET /api/strategies. Cannot delete a strategy that is currently assigned to a running trader.`,
				s.handleDeleteStrategy)
			s.routeWithSchema(protected, "POST", "/strategies/:id/activate", "Mark a strategy as the active strategy for this user",
				`:id = EXACT id from GET /api/strategies.
No request body needed. Sets this strategy as is_active=true (and deactivates the previous active strategy).
After activating, create or update a trader with this strategy_id to apply it.`,
				s.handleActivateStrategy)
			s.routeWithSchema(protected, "POST", "/strategies/:id/duplicate", "Duplicate an existing strategy",
				`:id = EXACT id from GET /api/strategies. Creates a copy with " (copy)" appended to the name.`,
				s.handleDuplicateStrategy)

			// Debate Arena
			s.route(protected, "GET", "/debates", "List debates", s.debateHandler.HandleListDebates)
			s.route(protected, "GET", "/debates/personalities", "Available AI personalities", s.debateHandler.HandleGetPersonalities)
			s.route(protected, "GET", "/debates/:id", "Get debate details", s.debateHandler.HandleGetDebate)
			s.route(protected, "POST", "/debates", "Create debate", s.debateHandler.HandleCreateDebate)
			s.route(protected, "POST", "/debates/:id/start", "Start debate", s.debateHandler.HandleStartDebate)
			s.route(protected, "POST", "/debates/:id/cancel", "Cancel debate", s.debateHandler.HandleCancelDebate)
			s.route(protected, "POST", "/debates/:id/execute", "Execute debate consensus decision", s.debateHandler.HandleExecuteDebate)
			s.route(protected, "DELETE", "/debates/:id", "Delete debate", s.debateHandler.HandleDeleteDebate)
			s.route(protected, "GET", "/debates/:id/messages", "Get debate messages", s.debateHandler.HandleGetMessages)
			s.route(protected, "GET", "/debates/:id/votes", "Get debate votes", s.debateHandler.HandleGetVotes)
			s.route(protected, "GET", "/debates/:id/stream", "SSE stream for live debate", s.debateHandler.HandleDebateStream)

			// Data for specified trader (using query parameter ?trader_id=xxx)
			// IMPORTANT: All ?trader_id= values must be the EXACT "trader_id" field from GET /api/my-traders
			s.routeWithSchema(protected, "GET", "/status", "Trader running status",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>
Returns: {"is_running":<bool>,"trader_id":"<string>"}`,
				s.handleStatus)
			s.routeWithSchema(protected, "GET", "/account", "Account balance and equity",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>
Returns: {"balance":<float>,"equity":<float>,"unrealized_pnl":<float>,"initial_balance":<float>,"total_return_pct":<float>}`,
				s.handleAccount)
			s.routeWithSchema(protected, "GET", "/positions", "Current open positions",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>
Returns: [{"symbol":"<string>","side":"long|short","size":<float>,"entry_price":<float>,"mark_price":<float>,"unrealized_pnl":<float>,"leverage":<int>}]`,
				s.handlePositions)
			s.routeWithSchema(protected, "GET", "/positions/history", "Closed position history",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>&limit=<int, default 20>`,
				s.handlePositionHistory)
			s.routeWithSchema(protected, "GET", "/trades", "Trade records",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>&limit=<int, default 20>`,
				s.handleTrades)
			s.routeWithSchema(protected, "GET", "/orders", "All order records",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>&limit=<int, default 20>`,
				s.handleOrders)
			s.routeWithSchema(protected, "GET", "/orders/:id/fills", "Order fill details",
				`:id = order id from GET /api/orders`,
				s.handleOrderFills)
			s.routeWithSchema(protected, "GET", "/open-orders", "Open orders currently on exchange",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>`,
				s.handleOpenOrders)
			s.routeWithSchema(protected, "GET", "/decisions", "AI trading decisions (decision records)",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>&limit=<int, default 20>
Returns: [{"id":"<string>","symbol":"<string>","action":"open_long|open_short|close_long|close_short|hold","confidence":<int>,"reasoning":"<string>","created_at":"<timestamp>"}]`,
				s.handleDecisions)
			s.routeWithSchema(protected, "GET", "/decisions/latest", "Latest AI decisions (most recent scan results)",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>
Returns the most recent AI decision for each symbol analyzed in the last scan cycle.`,
				s.handleLatestDecisions)
			s.routeWithSchema(protected, "GET", "/statistics", "Trading performance statistics",
				`Query: ?trader_id=<EXACT trader_id from GET /api/my-traders>
Returns: {"total_trades":<int>,"winning_trades":<int>,"win_rate":<float>,"total_pnl":<float>,"sharpe_ratio":<float>,"max_drawdown":<float>}`,
				s.handleStatistics)

			// Backtest routes
			backtest := protected.Group("/backtest")
			s.registerBacktestRoutes(backtest)
		}
	}
}

// handleHealth Health check
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
}

// handleGetSystemConfig Get system configuration (configuration that client needs to know)
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	userCount, _ := s.store.User().Count()
	c.JSON(http.StatusOK, gin.H{
		"initialized":      userCount > 0,
		"btc_eth_leverage": 10,
		"altcoin_leverage": 5,
	})
}

// handleGetServerIP Get server IP address (for whitelist configuration)
func (s *Server) handleGetServerIP(c *gin.Context) {
	// Try to get public IP via third-party API
	publicIP := getPublicIPFromAPI()

	// If third-party API fails, get first public IP from network interface
	if publicIP == "" {
		publicIP = getPublicIPFromInterface()
	}

	// If still cannot get it, return error
	if publicIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to get public IP address"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   "Please add this IP address to the whitelist",
	})
}

// getPublicIPFromAPI Get public IP via third-party API (IPv4 only)
func getPublicIPFromAPI() string {
	// Try multiple public IP query services (IPv4-only endpoints)
	services := []string{
		"https://api4.ipify.org?format=text", // IPv4 only
		"https://ipv4.icanhazip.com",         // IPv4 only
		"https://v4.ident.me",                // IPv4 only
		"https://api.ipify.org?format=text",  // May return IPv4 or IPv6
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body := make([]byte, 128)
			n, err := resp.Body.Read(body)
			if err != nil && err.Error() != "EOF" {
				continue
			}

			ip := strings.TrimSpace(string(body[:n]))
			parsedIP := net.ParseIP(ip)
			// Verify if it's a valid IPv4 address (not containing ":")
			if parsedIP != nil && parsedIP.To4() != nil {
				return ip
			}
		}
	}

	return ""
}

// getPublicIPFromInterface Get first public IP from network interface
func getPublicIPFromInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// Skip disabled interfaces and loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Only consider IPv4 addresses
			if ip.To4() != nil {
				ipStr := ip.String()
				// Exclude private IP address ranges
				if !isPrivateIP(ip) {
					return ipStr
				}
			}
		}
	}

	return ""
}

// isPrivateIP Determine if it's a private IP address
func isPrivateIP(ip net.IP) bool {
	// Private IP address ranges:
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// getTraderFromQuery Get trader from query parameter
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// Ensure user's traders are loaded into memory
	err := s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to load traders for user %s: %v", userID, err)
	}

	if traderID == "" {
		// If no trader_id specified, return first trader for this user
		ids := s.traderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("No available traders")
		}

		// Get user's trader list, prioritize returning user's own traders
		userTraders, err := s.store.Trader().List(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			traderID = ids[0]
		}
	}

	return s.traderManager, traderID, nil
}

// AI trader management related structures
type CreateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`     // Pointer type, nil means use default value true
	ShowInCompetition   *bool   `json:"show_in_competition"` // Pointer type, nil means use default value true
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"` // System prompt template name
	UseAI500             bool   `json:"use_ai500"`
	UseOITop             bool   `json:"use_oi_top"`
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

// SafeModelConfig Safe model configuration structure (does not contain sensitive information)
type SafeModelConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	CustomAPIURL    string `json:"customApiUrl"`    // Custom API URL (usually not sensitive)
	CustomModelName string `json:"customModelName"` // Custom model name (not sensitive)
}

type ExchangeConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "cex" or "dex"
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Testnet   bool   `json:"testnet,omitempty"`
}

// SafeExchangeConfig Safe exchange configuration structure (does not contain sensitive information)
type SafeExchangeConfig struct {
	ID                    string `json:"id"`            // UUID
	ExchangeType          string `json:"exchange_type"` // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
	AccountName           string `json:"account_name"`  // User-defined account name
	Name                  string `json:"name"`          // Display name
	Type                  string `json:"type"`          // "cex" or "dex"
	Enabled               bool   `json:"enabled"`
	Testnet               bool   `json:"testnet,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Hyperliquid wallet address (not sensitive)
	AsterUser             string `json:"asterUser"`             // Aster username (not sensitive)
	AsterSigner           string `json:"asterSigner"`           // Aster signer (not sensitive)
	LighterWalletAddr     string `json:"lighterWalletAddr"`     // LIGHTER wallet address (not sensitive)
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	} `json:"models"`
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]struct {
		Enabled                 bool   `json:"enabled"`
		APIKey                  string `json:"api_key"`
		SecretKey               string `json:"secret_key"`
		Passphrase              string `json:"passphrase"` // OKX specific
		Testnet                 bool   `json:"testnet"`
		HyperliquidWalletAddr   string `json:"hyperliquid_wallet_addr"`
		HyperliquidUnifiedAcct  bool   `json:"hyperliquid_unified_account"` // Unified Account mode
		AsterUser               string `json:"aster_user"`
		AsterSigner             string `json:"aster_signer"`
		AsterPrivateKey         string `json:"aster_private_key"`
		LighterWalletAddr       string `json:"lighter_wallet_addr"`
		LighterPrivateKey       string `json:"lighter_private_key"`
		LighterAPIKeyPrivateKey string `json:"lighter_api_key_private_key"`
		LighterAPIKeyIndex      int    `json:"lighter_api_key_index"`
	} `json:"exchanges"`
}

// handleCreateTrader Create new AI trader
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Validate leverage values
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH leverage must be between 1-50x"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Altcoin leverage must be between 1-20x"})
		return
	}

	// Validate trading symbol format
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid symbol format: %s, must end with USDT", symbol)})
				return
			}
		}
	}

	// Generate trader ID (use short UUID prefix for readability)
	exchangeIDShort := req.ExchangeID
	if len(exchangeIDShort) > 8 {
		exchangeIDShort = exchangeIDShort[:8]
	}
	traderID := fmt.Sprintf("%s_%s_%d", exchangeIDShort, req.AIModelID, time.Now().Unix())

	// Set default values
	isCrossMargin := true // Default to cross margin mode
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := true // Default to show in competition
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values
	btcEthLeverage := 10 // Default value
	altcoinLeverage := 5 // Default value
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	}

	// Set system prompt template default value
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// Set scan interval default value
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3 // Default 3 minutes, not allowed to be less than 3
	}

	// Query exchange actual balance, override user input
	actualBalance := req.InitialBalance // Default to use user input
	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		logger.Infof("⚠️ Failed to get exchange config, using user input for initial balance: %v", err)
	}

	// Find matching exchange configuration
	var exchangeCfg *store.Exchange
	for _, ex := range exchanges {
		if ex.ID == req.ExchangeID {
			exchangeCfg = ex
			break
		}
	}

	if exchangeCfg == nil {
		logger.Infof("⚠️ Exchange %s configuration not found, using user input for initial balance", req.ExchangeID)
	} else if !exchangeCfg.Enabled {
		logger.Infof("⚠️ Exchange %s not enabled, using user input for initial balance", req.ExchangeID)
	} else {
		// Create temporary trader based on exchange type to query balance
		var tempTrader trader.Trader
		var createErr error

		// Use ExchangeType (e.g., "binance") instead of ID (UUID)
		// Convert EncryptedString fields to string
		switch exchangeCfg.ExchangeType {
		case "binance":
			tempTrader = binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID)
		case "hyperliquid":
			tempTrader, createErr = hyperliquidtrader.NewHyperliquidTrader(
				string(exchangeCfg.APIKey), // private key
				exchangeCfg.HyperliquidWalletAddr,
				exchangeCfg.Testnet,
				exchangeCfg.HyperliquidUnifiedAcct,
			)
		case "aster":
			tempTrader, createErr = aster.NewAsterTrader(
				exchangeCfg.AsterUser,
				exchangeCfg.AsterSigner,
				string(exchangeCfg.AsterPrivateKey),
			)
		case "bybit":
			tempTrader = bybit.NewBybitTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
			)
		case "okx":
			tempTrader = okx.NewOKXTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
				string(exchangeCfg.Passphrase),
			)
		case "bitget":
			tempTrader = bitget.NewBitgetTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
				string(exchangeCfg.Passphrase),
			)
		case "gate":
			tempTrader = gate.NewGateTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
			)
		case "kucoin":
			tempTrader = kucoin.NewKuCoinTrader(
				string(exchangeCfg.APIKey),
				string(exchangeCfg.SecretKey),
				string(exchangeCfg.Passphrase),
			)
		case "lighter":
			if exchangeCfg.LighterWalletAddr != "" && string(exchangeCfg.LighterAPIKeyPrivateKey) != "" {
				// Lighter only supports mainnet
				tempTrader, createErr = lighter.NewLighterTraderV2(
					exchangeCfg.LighterWalletAddr,
					string(exchangeCfg.LighterAPIKeyPrivateKey),
					exchangeCfg.LighterAPIKeyIndex,
					false, // Always use mainnet for Lighter
				)
			} else {
				createErr = fmt.Errorf("Lighter requires wallet address and API Key private key")
			}
		default:
			logger.Infof("⚠️ Unsupported exchange type: %s, using user input for initial balance", exchangeCfg.ExchangeType)
		}

		if createErr != nil {
			logger.Infof("⚠️ Failed to create temporary trader, using user input for initial balance: %v", createErr)
		} else if tempTrader != nil {
			// Query actual balance
			balanceInfo, balanceErr := tempTrader.GetBalance()
			if balanceErr != nil {
				logger.Infof("⚠️ Failed to query exchange balance, using user input for initial balance: %v", balanceErr)
			} else {
				// Extract total equity (account total value = wallet balance + unrealized PnL)
				// Priority: total_equity > totalWalletBalance > wallet_balance > totalEq > balance
				// Note: Must use total_equity (not availableBalance) for accurate P&L calculation
				balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
				for _, key := range balanceKeys {
					if balance, ok := balanceInfo[key].(float64); ok && balance > 0 {
						actualBalance = balance
						logger.Infof("✓ Queried exchange total equity (%s): %.2f USDT (user input: %.2f USDT)", key, actualBalance, req.InitialBalance)
						break
					}
				}
				if actualBalance <= 0 {
					logger.Infof("⚠️ Unable to extract total equity from balance info, balanceInfo=%v, using user input for initial balance", balanceInfo)
				}
			}
		}
	}

	// Create trader configuration (database entity)
	logger.Infof("🔧 DEBUG: Starting to create trader config, ID=%s, Name=%s, AIModel=%s, Exchange=%s, StrategyID=%s", traderID, req.Name, req.AIModelID, req.ExchangeID, req.StrategyID)
	traderRecord := &store.Trader{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		StrategyID:           req.StrategyID, // Associated strategy ID (new version)
		InitialBalance:       actualBalance,  // Use actual queried balance
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseAI500:             req.UseAI500,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// Save to database
	logger.Infof("🔧 DEBUG: Preparing to call CreateTrader")
	err = s.store.Trader().Create(traderRecord)
	if err != nil {
		logger.Infof("❌ Failed to create trader: %v", err)
		SafeInternalError(c, "Failed to create trader", err)
		return
	}
	logger.Infof("🔧 DEBUG: CreateTrader succeeded")

	// Immediately load new trader into TraderManager
	logger.Infof("🔧 DEBUG: Preparing to call LoadUserTraders")
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to load user traders into memory: %v", err)
		// Don't return error here since trader was successfully created in database
	}
	logger.Infof("🔧 DEBUG: LoadUserTraders completed")

	logger.Infof("✓ Trader created successfully: %s (model: %s, exchange: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest Update trader request
type UpdateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	IsCrossMargin       *bool   `json:"is_cross_margin"`
	ShowInCompetition   *bool   `json:"show_in_competition"`
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"`
}

// handleUpdateTrader Update trader configuration
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Check if trader exists and belongs to current user
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
		return
	}

	var existingTrader *store.Trader
	for _, t := range traders {
		if t.ID == traderID {
			existingTrader = t
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Set default values
	isCrossMargin := existingTrader.IsCrossMargin // Keep original value
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := existingTrader.ShowInCompetition // Keep original value
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // Keep original value
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // Keep original value
	}

	// Set scan interval, allow updates
	scanIntervalMinutes := req.ScanIntervalMinutes
	logger.Infof("📊 Update trader scan_interval: req=%d, existing=%d", req.ScanIntervalMinutes, existingTrader.ScanIntervalMinutes)
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // Keep original value
	} else if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3
	}
	logger.Infof("📊 Final scan_interval_minutes: %d", scanIntervalMinutes)

	// Set system prompt template
	systemPromptTemplate := req.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = existingTrader.SystemPromptTemplate // Keep original value
	}

	// Handle strategy ID (if not provided, keep original value)
	strategyID := req.StrategyID
	if strategyID == "" {
		strategyID = existingTrader.StrategyID
	}

	// Update trader configuration
	traderRecord := &store.Trader{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		StrategyID:           strategyID, // Associated strategy ID
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // Keep original value
	}

	// Check if trader was running before update (we'll restart it after)
	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(traderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
			logger.Infof("🔄 Trader %s was running, will restart with new config after update", traderID)
		}
	}

	// Update database
	logger.Infof("🔄 Updating trader: ID=%s, Name=%s, AIModelID=%s, StrategyID=%s, ScanInterval=%d min",
		traderRecord.ID, traderRecord.Name, traderRecord.AIModelID, traderRecord.StrategyID, scanIntervalMinutes)
	err = s.store.Trader().Update(traderRecord)
	if err != nil {
		SafeInternalError(c, "Failed to update trader", err)
		return
	}

	// Remove old trader from memory first (this also stops if running)
	s.traderManager.RemoveTrader(traderID)

	// Reload traders into memory with fresh config
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	// If trader was running before, restart it with new config
	if wasRunning {
		if reloadedTrader, getErr := s.traderManager.GetTrader(traderID); getErr == nil {
			go func() {
				logger.Infof("▶️ Restarting trader %s with new config...", traderID)
				if runErr := reloadedTrader.Run(); runErr != nil {
					logger.Infof("❌ Trader %s runtime error: %v", traderID, runErr)
				}
			}()
		}
	}

	logger.Infof("✓ Trader updated successfully: %s (model: %s, exchange: %s, strategy: %s)", req.Name, req.AIModelID, req.ExchangeID, strategyID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "Trader updated successfully",
	})
}

// handleDeleteTrader Delete trader
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Delete from database
	err := s.store.Trader().Delete(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to delete trader", err)
		return
	}

	// If trader is running, stop it first
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			logger.Infof("⏹  Stopped running trader: %s", traderID)
		}
	}

	// Remove trader from memory
	s.traderManager.RemoveTrader(traderID)

	logger.Infof("✓ Trader deleted: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "Trader deleted"})
}

// handleStartTrader Start trader
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Verify trader belongs to current user
	_, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}

	// Check if trader exists in memory and if it's running
	existingTrader, _ := s.traderManager.GetTrader(traderID)
	if existingTrader != nil {
		status := existingTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already running"})
			return
		}
		// Trader exists but is stopped - remove from memory to reload fresh config
		logger.Infof("🔄 Removing stopped trader %s from memory to reload config...", traderID)
		s.traderManager.RemoveTrader(traderID)
	}

	// Load trader from database (always reload to get latest config)
	logger.Infof("🔄 Loading trader %s from database...", traderID)
	if loadErr := s.traderManager.LoadUserTradersFromStore(s.store, userID); loadErr != nil {
		logger.Infof("❌ Failed to load user traders: %v", loadErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load trader: " + loadErr.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		// Check detailed reason
		fullCfg, _ := s.store.Trader().GetFullConfig(userID, traderID)
		if fullCfg != nil && fullCfg.Trader != nil {
			// Check strategy
			if fullCfg.Strategy == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader has no strategy configured, please create a strategy in Strategy Studio and associate it with the trader"})
				return
			}
			// Check AI model
			if fullCfg.AIModel == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's AI model does not exist, please check AI model configuration"})
				return
			}
			if !fullCfg.AIModel.Enabled {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's AI model is not enabled, please enable the AI model first"})
				return
			}
			// Check exchange
			if fullCfg.Exchange == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's exchange does not exist, please check exchange configuration"})
				return
			}
			if !fullCfg.Exchange.Enabled {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Trader's exchange is not enabled, please enable the exchange first"})
				return
			}
		}
		// Check if there's a specific load error
		if loadErr := s.traderManager.GetLoadError(traderID); loadErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load trader: " + loadErr.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Failed to load trader, please check AI model, exchange and strategy configuration"})
		return
	}

	// Start trader
	go func() {
		logger.Infof("▶️  Starting trader %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			logger.Infof("❌ Trader %s runtime error: %v", trader.GetName(), err)
		}
	}()

	// Update running status in database
	err = s.store.Trader().UpdateStatus(userID, traderID, true)
	if err != nil {
		logger.Infof("⚠️  Failed to update trader status: %v", err)
	}

	logger.Infof("✓ Trader %s started", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "Trader started"})
}

// handleStopTrader Stop trader
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Verify trader belongs to current user
	_, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Check if trader is running
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already stopped"})
		return
	}

	// Stop trader
	trader.Stop()

	// Update running status in database
	err = s.store.Trader().UpdateStatus(userID, traderID, false)
	if err != nil {
		logger.Infof("⚠️  Failed to update trader status: %v", err)
	}

	logger.Infof("⏹  Trader %s stopped", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "Trader stopped"})
}

// handleUpdateTraderPrompt Update trader custom prompt
func (s *Server) handleUpdateTraderPrompt(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		CustomPrompt       string `json:"custom_prompt"`
		OverrideBasePrompt bool   `json:"override_base_prompt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Update database
	err := s.store.Trader().UpdateCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		SafeInternalError(c, "Failed to update custom prompt", err)
		return
	}

	// If trader is in memory, update its custom prompt and override settings
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		logger.Infof("✓ Updated trader %s custom prompt (override base=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Custom prompt updated"})
}

// handleToggleCompetition Toggle trader competition visibility
func (s *Server) handleToggleCompetition(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		ShowInCompetition bool `json:"show_in_competition"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Update database
	err := s.store.Trader().UpdateShowInCompetition(userID, traderID, req.ShowInCompetition)
	if err != nil {
		SafeInternalError(c, "Update competition visibility", err)
		return
	}

	// Update in-memory trader if it exists
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		trader.SetShowInCompetition(req.ShowInCompetition)
	}

	status := "shown"
	if !req.ShowInCompetition {
		status = "hidden"
	}
	logger.Infof("✓ Trader %s competition visibility updated: %s", traderID, status)
	c.JSON(http.StatusOK, gin.H{
		"message":             "Competition visibility updated",
		"show_in_competition": req.ShowInCompetition,
	})
}

// handleGetGridRiskInfo returns current risk information for a grid trader
func (s *Server) handleGetGridRiskInfo(c *gin.Context) {
	traderID := c.Param("id")

	autoTrader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found"})
		return
	}

	riskInfo := autoTrader.GetGridRiskInfo()
	c.JSON(http.StatusOK, riskInfo)
}

// handleSyncBalance Sync exchange balance to initial_balance (Option B: Manual Sync + Option C: Smart Detection)
func (s *Server) handleSyncBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	logger.Infof("🔄 User %s requested balance sync for trader %s", userID, traderID)

	// Get trader configuration from database (including exchange info)
	fullConfig, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	traderConfig := fullConfig.Trader
	exchangeCfg := fullConfig.Exchange

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange not configured or not enabled"})
		return
	}

	// Create temporary trader to query balance
	var tempTrader trader.Trader
	var createErr error

	// Use ExchangeType (e.g., "binance") instead of ExchangeID (which is now UUID)
	// Convert EncryptedString fields to string
	switch exchangeCfg.ExchangeType {
	case "binance":
		tempTrader = binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID)
	case "hyperliquid":
		tempTrader, createErr = hyperliquidtrader.NewHyperliquidTrader(
			string(exchangeCfg.APIKey),
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
			exchangeCfg.HyperliquidUnifiedAcct,
		)
	case "aster":
		tempTrader, createErr = aster.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			string(exchangeCfg.AsterPrivateKey),
		)
	case "bybit":
		tempTrader = bybit.NewBybitTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "okx":
		tempTrader = okx.NewOKXTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "bitget":
		tempTrader = bitget.NewBitgetTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "gate":
		tempTrader = gate.NewGateTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "kucoin":
		tempTrader = kucoin.NewKuCoinTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "lighter":
		if exchangeCfg.LighterWalletAddr != "" && string(exchangeCfg.LighterAPIKeyPrivateKey) != "" {
			// Lighter only supports mainnet
			tempTrader, createErr = lighter.NewLighterTraderV2(
				exchangeCfg.LighterWalletAddr,
				string(exchangeCfg.LighterAPIKeyPrivateKey),
				exchangeCfg.LighterAPIKeyIndex,
				false, // Always use mainnet for Lighter
			)
		} else {
			createErr = fmt.Errorf("Lighter requires wallet address and API Key private key")
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange type"})
		return
	}

	if createErr != nil {
		logger.Infof("⚠️ Failed to create temporary trader: %v", createErr)
		SafeInternalError(c, "Failed to connect to exchange", createErr)
		return
	}

	// Query actual balance
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		logger.Infof("⚠️ Failed to query exchange balance: %v", balanceErr)
		SafeInternalError(c, "Failed to query balance", balanceErr)
		return
	}

	// Extract total equity (for P&L calculation, we need total account value, not available balance)
	var actualBalance float64
	// Priority: total_equity > totalWalletBalance > wallet_balance > totalEq > balance
	balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
	for _, key := range balanceKeys {
		if balance, ok := balanceInfo[key].(float64); ok && balance > 0 {
			actualBalance = balance
			break
		}
	}
	if actualBalance <= 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to get total equity"})
		return
	}

	oldBalance := traderConfig.InitialBalance

	// ✅ Option C: Smart balance change detection
	changePercent := ((actualBalance - oldBalance) / oldBalance) * 100
	changeType := "increase"
	if changePercent < 0 {
		changeType = "decrease"
	}

	logger.Infof("✓ Queried actual exchange balance: %.2f USDT (current config: %.2f USDT, change: %.2f%%)",
		actualBalance, oldBalance, changePercent)

	// Update initial_balance in database
	err = s.store.Trader().UpdateInitialBalance(userID, traderID, actualBalance)
	if err != nil {
		logger.Infof("❌ Failed to update initial_balance: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
		return
	}

	// Reload traders into memory
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	logger.Infof("✅ Synced balance: %.2f → %.2f USDT (%s %.2f%%)", oldBalance, actualBalance, changeType, changePercent)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Balance synced successfully",
		"old_balance":    oldBalance,
		"new_balance":    actualBalance,
		"change_percent": changePercent,
		"change_type":    changeType,
	})
}

// handleClosePosition One-click close position
func (s *Server) handleClosePosition(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req struct {
		Symbol string `json:"symbol" binding:"required"`
		Side   string `json:"side" binding:"required"` // "LONG" or "SHORT"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter error: symbol and side are required"})
		return
	}

	logger.Infof("🔻 User %s requested position close: trader=%s, symbol=%s, side=%s", userID, traderID, req.Symbol, req.Side)

	// Get trader configuration from database (including exchange info)
	fullConfig, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	exchangeCfg := fullConfig.Exchange

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange not configured or not enabled"})
		return
	}

	// Create temporary trader to execute close position
	var tempTrader trader.Trader
	var createErr error

	// Use ExchangeType (e.g., "binance") instead of ExchangeID (which is now UUID)
	// Convert EncryptedString fields to string
	switch exchangeCfg.ExchangeType {
	case "binance":
		tempTrader = binance.NewFuturesTrader(string(exchangeCfg.APIKey), string(exchangeCfg.SecretKey), userID)
	case "hyperliquid":
		tempTrader, createErr = hyperliquidtrader.NewHyperliquidTrader(
			string(exchangeCfg.APIKey),
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
			exchangeCfg.HyperliquidUnifiedAcct,
		)
	case "aster":
		tempTrader, createErr = aster.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			string(exchangeCfg.AsterPrivateKey),
		)
	case "bybit":
		tempTrader = bybit.NewBybitTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "okx":
		tempTrader = okx.NewOKXTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "bitget":
		tempTrader = bitget.NewBitgetTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "gate":
		tempTrader = gate.NewGateTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
		)
	case "kucoin":
		tempTrader = kucoin.NewKuCoinTrader(
			string(exchangeCfg.APIKey),
			string(exchangeCfg.SecretKey),
			string(exchangeCfg.Passphrase),
		)
	case "lighter":
		if exchangeCfg.LighterWalletAddr != "" && string(exchangeCfg.LighterAPIKeyPrivateKey) != "" {
			// Lighter only supports mainnet
			tempTrader, createErr = lighter.NewLighterTraderV2(
				exchangeCfg.LighterWalletAddr,
				string(exchangeCfg.LighterAPIKeyPrivateKey),
				exchangeCfg.LighterAPIKeyIndex,
				false, // Always use mainnet for Lighter
			)
		} else {
			createErr = fmt.Errorf("Lighter requires wallet address and API Key private key")
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange type"})
		return
	}

	if createErr != nil {
		logger.Infof("⚠️ Failed to create temporary trader: %v", createErr)
		SafeInternalError(c, "Failed to connect to exchange", createErr)
		return
	}

	// Get current position info BEFORE closing (to get quantity and price)
	positions, err := tempTrader.GetPositions()
	if err != nil {
		logger.Infof("⚠️ Failed to get positions: %v", err)
	}

	var posQty float64
	var entryPrice float64
	for _, pos := range positions {
		if pos["symbol"] == req.Symbol && pos["side"] == strings.ToLower(req.Side) {
			if amt, ok := pos["positionAmt"].(float64); ok {
				posQty = amt
				if posQty < 0 {
					posQty = -posQty // Make positive
				}
			}
			if price, ok := pos["entryPrice"].(float64); ok {
				entryPrice = price
			}
			break
		}
	}

	// Execute close position operation
	var result map[string]interface{}
	var closeErr error

	if req.Side == "LONG" {
		result, closeErr = tempTrader.CloseLong(req.Symbol, 0) // 0 means close all
	} else if req.Side == "SHORT" {
		result, closeErr = tempTrader.CloseShort(req.Symbol, 0) // 0 means close all
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be LONG or SHORT"})
		return
	}

	if closeErr != nil {
		logger.Infof("❌ Close position failed: symbol=%s, side=%s, error=%v", req.Symbol, req.Side, closeErr)
		SafeInternalError(c, "Close position", closeErr)
		return
	}

	logger.Infof("✅ Position closed successfully: symbol=%s, side=%s, qty=%.6f, result=%v", req.Symbol, req.Side, posQty, result)

	// Record order to database (for chart markers and history)
	s.recordClosePositionOrder(traderID, exchangeCfg.ID, exchangeCfg.ExchangeType, req.Symbol, req.Side, posQty, entryPrice, result)

	c.JSON(http.StatusOK, gin.H{
		"message": "Position closed successfully",
		"symbol":  req.Symbol,
		"side":    req.Side,
		"result":  result,
	})
}

// recordClosePositionOrder Record close position order to database (Lighter version - direct FILLED status)
func (s *Server) recordClosePositionOrder(traderID, exchangeID, exchangeType, symbol, side string, quantity, exitPrice float64, result map[string]interface{}) {
	// Skip for exchanges with OrderSync - let the background sync handle it to avoid duplicates
	switch exchangeType {
	case "binance", "lighter", "hyperliquid", "bybit", "okx", "bitget", "aster", "gate":
		logger.Infof("  📝 Close order will be synced by OrderSync, skipping immediate record")
		return
	}

	// Check if order was placed (skip if NO_POSITION)
	status, _ := result["status"].(string)
	if status == "NO_POSITION" {
		logger.Infof("  ⚠️ No position to close, skipping order record")
		return
	}

	// Get order ID from result
	var orderID string
	switch v := result["orderId"].(type) {
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

	// Determine order action based on side
	var orderAction string
	if side == "LONG" {
		orderAction = "close_long"
	} else {
		orderAction = "close_short"
	}

	// Use entry price if exit price not available
	if exitPrice == 0 {
		exitPrice = quantity * 100 // Rough estimate if we don't have price
	}

	// Estimate fee (0.04% for Lighter taker)
	fee := exitPrice * quantity * 0.0004

	// Create order record - DIRECTLY as FILLED (Lighter market orders fill immediately)
	orderRecord := &store.TraderOrder{
		TraderID:        traderID,
		ExchangeID:      exchangeID,
		ExchangeType:    exchangeType,
		ExchangeOrderID: orderID,
		Symbol:          symbol,
		PositionSide:    side,
		OrderAction:     orderAction,
		Type:            "MARKET",
		Side:            getSideFromAction(orderAction),
		Quantity:        quantity,
		Price:           0, // Market order
		Status:          "FILLED",
		FilledQuantity:  quantity,
		AvgFillPrice:    exitPrice,
		Commission:      fee,
		FilledAt:        time.Now().UTC().UnixMilli(),
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}

	if err := s.store.Order().CreateOrder(orderRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record order: %v", err)
		return
	}

	logger.Infof("  ✅ Order recorded as FILLED: %s [%s] %s qty=%.6f price=%.6f", orderID, orderAction, symbol, quantity, exitPrice)

	// Create fill record immediately
	tradeID := fmt.Sprintf("%s-%d", orderID, time.Now().UnixNano())
	fillRecord := &store.TraderFill{
		TraderID:        traderID,
		ExchangeID:      exchangeID,
		ExchangeType:    exchangeType,
		OrderID:         orderRecord.ID,
		ExchangeOrderID: orderID,
		ExchangeTradeID: tradeID,
		Symbol:          symbol,
		Side:            getSideFromAction(orderAction),
		Price:           exitPrice,
		Quantity:        quantity,
		QuoteQuantity:   exitPrice * quantity,
		Commission:      fee,
		CommissionAsset: "USDT",
		RealizedPnL:     0,
		IsMaker:         false,
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}

	if err := s.store.Order().CreateFill(fillRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record fill: %v", err)
	} else {
		logger.Infof("  ✅ Fill record created: price=%.6f qty=%.6f", exitPrice, quantity)
	}
}

// pollAndUpdateOrderStatus Poll order status and update with fill data
func (s *Server) pollAndUpdateOrderStatus(orderRecordID int64, traderID, exchangeID, exchangeType, orderID, symbol, orderAction string, tempTrader trader.Trader) {
	var actualPrice float64
	var actualQty float64
	var fee float64

	// Wait a bit for order to be filled
	time.Sleep(500 * time.Millisecond)

	// For Lighter, use GetTrades instead of GetOrderStatus (market orders are filled immediately)
	if exchangeType == "lighter" {
		s.pollLighterTradeHistory(orderRecordID, traderID, exchangeID, exchangeType, orderID, symbol, orderAction, tempTrader)
		return
	}

	// For other exchanges, poll GetOrderStatus
	for i := 0; i < 5; i++ {
		status, err := tempTrader.GetOrderStatus(symbol, orderID)
		if err != nil {
			logger.Infof("  ⚠️ GetOrderStatus failed (attempt %d/5): %v", i+1, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
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
				if err := s.store.Order().UpdateOrderStatus(orderRecordID, "FILLED", actualQty, actualPrice, fee); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
					return
				}

				// Record fill details
				tradeID := fmt.Sprintf("%s-%d", orderID, time.Now().UnixNano())
				fillRecord := &store.TraderFill{
					TraderID:        traderID,
					ExchangeID:      exchangeID,
					ExchangeType:    exchangeType,
					OrderID:         orderRecordID,
					ExchangeOrderID: orderID,
					ExchangeTradeID: tradeID,
					Symbol:          symbol,
					Side:            getSideFromAction(orderAction),
					Price:           actualPrice,
					Quantity:        actualQty,
					QuoteQuantity:   actualPrice * actualQty,
					Commission:      fee,
					CommissionAsset: "USDT",
					RealizedPnL:     0,
					IsMaker:         false,
					CreatedAt:       time.Now().UTC().UnixMilli(),
				}

				if err := s.store.Order().CreateFill(fillRecord); err != nil {
					logger.Infof("  ⚠️ Failed to record fill: %v", err)
				} else {
					logger.Infof("  📝 Fill recorded: price=%.6f, qty=%.6f", actualPrice, actualQty)
				}

				return
			} else if statusStr == "CANCELED" || statusStr == "EXPIRED" || statusStr == "REJECTED" {
				logger.Infof("  ⚠️ Order %s, updating status", statusStr)
				s.store.Order().UpdateOrderStatus(orderRecordID, statusStr, 0, 0, 0)
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	logger.Infof("  ⚠️ Failed to confirm order fill after polling, order may still be pending")
}

// pollLighterTradeHistory No longer used - Lighter orders are marked as FILLED immediately
// Keeping this function stub for compatibility with other exchanges
func (s *Server) pollLighterTradeHistory(orderRecordID int64, traderID, exchangeID, exchangeType, orderID, symbol, orderAction string, tempTrader trader.Trader) {
	// For Lighter, orders are now recorded as FILLED immediately in recordClosePositionOrder
	// This function is no longer called for Lighter exchange
	logger.Infof("  ℹ️ pollLighterTradeHistory called but not needed (order already marked FILLED)")
}

// getSideFromAction Get order side (BUY/SELL) from order action
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}

// handleGetModelConfigs Get AI model configurations
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying AI model configs for user %s", userID)
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		logger.Infof("❌ Failed to get AI model configs: %v", err)
		SafeInternalError(c, "Failed to get AI model configs", err)
		return
	}

	// If no models in database, return default models
	if len(models) == 0 {
		logger.Infof("⚠️ No AI models in database, returning defaults")
		defaultModels := []SafeModelConfig{
			{ID: "deepseek", Name: "DeepSeek AI", Provider: "deepseek", Enabled: false},
			{ID: "qwen", Name: "Qwen AI", Provider: "qwen", Enabled: false},
			{ID: "openai", Name: "OpenAI", Provider: "openai", Enabled: false},
			{ID: "claude", Name: "Claude AI", Provider: "claude", Enabled: false},
			{ID: "gemini", Name: "Gemini AI", Provider: "gemini", Enabled: false},
			{ID: "grok", Name: "Grok AI", Provider: "grok", Enabled: false},
			{ID: "kimi", Name: "Kimi AI", Provider: "kimi", Enabled: false},
			{ID: "minimax", Name: "MiniMax AI", Provider: "minimax", Enabled: false},
		}
		c.JSON(http.StatusOK, defaultModels)
		return
	}

	logger.Infof("✅ Found %d AI model configs", len(models))

	// Convert to safe response structure, remove sensitive information
	safeModels := make([]SafeModelConfig, len(models))
	for i, model := range models {
		safeModels[i] = SafeModelConfig{
			ID:              model.ID,
			Name:            model.Name,
			Provider:        model.Provider,
			Enabled:         model.Enabled,
			CustomAPIURL:    model.CustomAPIURL,
			CustomModelName: model.CustomModelName,
		}
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleUpdateModelConfigs Update AI model configurations (supports both encrypted and plain text based on config)
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req UpdateModelConfigRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		logger.Infof("📝 Received plain text model config (UserID: %s)", userID)
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			logger.Infof("❌ Failed to parse encrypted payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		// Verify encrypted data
		if encryptedPayload.WrappedKey == "" {
			logger.Infof("❌ Detected unencrypted request (UserID: %s)", userID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		// Decrypt data
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			logger.Infof("❌ Failed to decrypt model config (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			logger.Infof("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		logger.Infof("🔓 Decrypted model config data (UserID: %s)", userID)
	}

	// Update each model's configuration and track traders that need reload
	tradersToReload := make(map[string]bool)
	for modelID, modelData := range req.Models {
		// SSRF protection: validate custom_api_url before storing
		if modelData.CustomAPIURL != "" {
			cleanURL := strings.TrimSuffix(modelData.CustomAPIURL, "#")
			if err := security.ValidateURL(cleanURL); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom_api_url for model %s: %s", modelID, err.Error())})
				return
			}
		}

		// Find traders using this AI model BEFORE updating
		traders, _ := s.store.Trader().ListByAIModelID(userID, modelID)
		for _, t := range traders {
			tradersToReload[t.ID] = true
		}

		err := s.store.AIModel().Update(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			SafeInternalError(c, fmt.Sprintf("Update model %s", modelID), err)
			return
		}
	}

	// Remove affected traders from memory BEFORE reloading to pick up new config
	for traderID := range tradersToReload {
		logger.Infof("🔄 Removing trader %s from memory to reload with new AI model config", traderID)
		s.traderManager.RemoveTrader(traderID)
	}

	// Reload all traders for this user to make new config take effect immediately
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here since model config was successfully updated to database
	}

	logger.Infof("✓ AI model config updated: %+v", req.Models)
	c.JSON(http.StatusOK, gin.H{"message": "Model configuration updated"})
}

// handleGetExchangeConfigs Get exchange configurations
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying exchange configs for user %s", userID)
	exchanges, err := s.store.Exchange().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get exchange configs", err)
		return
	}

	// If no exchanges in database, return empty array (user needs to create accounts)
	if len(exchanges) == 0 {
		logger.Infof("⚠️ No exchanges in database for user %s", userID)
		c.JSON(http.StatusOK, []SafeExchangeConfig{})
		return
	}

	logger.Infof("✅ Found %d exchange configs", len(exchanges))

	// Convert to safe response structure, remove sensitive information
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ID,
			ExchangeType:          exchange.ExchangeType,
			AccountName:           exchange.AccountName,
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: exchange.HyperliquidWalletAddr,
			AsterUser:             exchange.AsterUser,
			AsterSigner:           exchange.AsterSigner,
			LighterWalletAddr:     exchange.LighterWalletAddr,
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs Update exchange configurations (supports both encrypted and plain text based on config)
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req UpdateExchangeConfigRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		logger.Infof("📝 Received plain text exchange config (UserID: %s)", userID)
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			logger.Infof("❌ Failed to parse encrypted payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		// Verify encrypted data
		if encryptedPayload.WrappedKey == "" {
			logger.Infof("❌ Detected unencrypted request (UserID: %s)", userID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		// Decrypt data
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			logger.Infof("❌ Failed to decrypt exchange config (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			logger.Infof("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		logger.Infof("🔓 Decrypted exchange config data (UserID: %s)", userID)
	}

	// Update each exchange's configuration and track traders that need reload
	tradersToReload := make(map[string]bool)
	for exchangeID, exchangeData := range req.Exchanges {
		// Find traders using this exchange BEFORE updating
		traders, _ := s.store.Trader().ListByExchangeID(userID, exchangeID)
		for _, t := range traders {
			tradersToReload[t.ID] = true
		}

		err := s.store.Exchange().Update(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Passphrase, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.HyperliquidUnifiedAcct, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.LighterWalletAddr, exchangeData.LighterPrivateKey, exchangeData.LighterAPIKeyPrivateKey, exchangeData.LighterAPIKeyIndex)
		if err != nil {
			SafeInternalError(c, fmt.Sprintf("Update exchange %s", exchangeID), err)
			return
		}
	}

	// Remove affected traders from memory BEFORE reloading to pick up new config
	for traderID := range tradersToReload {
		logger.Infof("🔄 Removing trader %s from memory to reload with new exchange config", traderID)
		s.traderManager.RemoveTrader(traderID)
	}

	// Reload all traders for this user to make new config take effect immediately
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here since exchange config was successfully updated to database
	}

	logger.Infof("✓ Exchange config updated: %+v", req.Exchanges)
	c.JSON(http.StatusOK, gin.H{"message": "Exchange configuration updated"})
}

// CreateExchangeRequest request structure for creating a new exchange account
type CreateExchangeRequest struct {
	ExchangeType            string `json:"exchange_type" binding:"required"` // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
	AccountName             string `json:"account_name"`                     // User-defined account name
	Enabled                 bool   `json:"enabled"`
	APIKey                  string `json:"api_key"`
	SecretKey               string `json:"secret_key"`
	Passphrase              string `json:"passphrase"`
	Testnet                 bool   `json:"testnet"`
	HyperliquidWalletAddr   string `json:"hyperliquid_wallet_addr"`
	HyperliquidUnifiedAcct  bool   `json:"hyperliquid_unified_account"` // Unified Account mode: Spot as Perp collateral
	AsterUser               string `json:"aster_user"`
	AsterSigner             string `json:"aster_signer"`
	AsterPrivateKey         string `json:"aster_private_key"`
	LighterWalletAddr       string `json:"lighter_wallet_addr"`
	LighterPrivateKey       string `json:"lighter_private_key"`
	LighterAPIKeyPrivateKey string `json:"lighter_api_key_private_key"`
	LighterAPIKeyIndex      int    `json:"lighter_api_key_index"`
}

// handleCreateExchange Create a new exchange account
func (s *Server) handleCreateExchange(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req CreateExchangeRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		if encryptedPayload.WrappedKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
	}

	// Validate exchange type
	validTypes := map[string]bool{
		"binance": true, "bybit": true, "okx": true, "bitget": true,
		"hyperliquid": true, "aster": true, "lighter": true, "gate": true, "kucoin": true, "indodax": true,
	}
	if !validTypes[req.ExchangeType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid exchange type: %s", req.ExchangeType)})
		return
	}

	// Create new exchange account
	id, err := s.store.Exchange().Create(
		userID, req.ExchangeType, req.AccountName, req.Enabled,
		req.APIKey, req.SecretKey, req.Passphrase, req.Testnet,
		req.HyperliquidWalletAddr, req.HyperliquidUnifiedAcct,
		req.AsterUser, req.AsterSigner, req.AsterPrivateKey,
		req.LighterWalletAddr, req.LighterPrivateKey, req.LighterAPIKeyPrivateKey, req.LighterAPIKeyIndex,
	)
	if err != nil {
		logger.Infof("❌ Failed to create exchange account: %v", err)
		SafeInternalError(c, "Failed to create exchange account", err)
		return
	}

	logger.Infof("✓ Created exchange account: type=%s, name=%s, id=%s", req.ExchangeType, req.AccountName, id)
	c.JSON(http.StatusOK, gin.H{
		"message": "Exchange account created",
		"id":      id,
	})
}

// handleDeleteExchange Delete an exchange account
func (s *Server) handleDeleteExchange(c *gin.Context) {
	userID := c.GetString("user_id")
	exchangeID := c.Param("id")

	if exchangeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange ID is required"})
		return
	}

	// Check if any traders are using this exchange
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check traders"})
		return
	}

	for _, trader := range traders {
		if trader.ExchangeID == exchangeID {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       "Cannot delete exchange account that is in use by traders",
				"trader_id":   trader.ID,
				"trader_name": trader.Name,
			})
			return
		}
	}

	// Delete exchange account
	err = s.store.Exchange().Delete(userID, exchangeID)
	if err != nil {
		logger.Infof("❌ Failed to delete exchange account: %v", err)
		SafeInternalError(c, "Failed to delete exchange account", err)
		return
	}

	logger.Infof("✓ Deleted exchange account: id=%s", exchangeID)
	c.JSON(http.StatusOK, gin.H{"message": "Exchange account deleted"})
}

// handleTraderList Trader list
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.store.Trader().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to get trader list", err)
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// Get real-time running status
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// Get strategy name if strategy_id is set
		var strategyName string
		if trader.StrategyID != "" {
			if strategy, err := s.store.Strategy().Get(userID, trader.StrategyID); err == nil {
				strategyName = strategy.Name
			}
		}

		// Return complete AIModelID (e.g. "admin_deepseek"), don't truncate
		// Frontend needs complete ID to verify model exists (consistent with handleGetTraderConfig)
		result = append(result, map[string]interface{}{
			"trader_id":           trader.ID,
			"trader_name":         trader.Name,
			"ai_model":            trader.AIModelID, // Use complete ID
			"exchange_id":         trader.ExchangeID,
			"is_running":          isRunning,
			"show_in_competition": trader.ShowInCompetition,
			"initial_balance":     trader.InitialBalance,
			"strategy_id":         trader.StrategyID,
			"strategy_name":       strategyName,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetTraderConfig Get trader detailed configuration
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader ID cannot be empty"})
		return
	}

	fullCfg, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader config")
		return
	}
	traderConfig := fullCfg.Trader

	// Get real-time running status
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// Return complete model ID without conversion, consistent with frontend model list
	aiModelID := traderConfig.AIModelID

	result := map[string]interface{}{
		"trader_id":             traderConfig.ID,
		"trader_name":           traderConfig.Name,
		"ai_model":              aiModelID,
		"exchange_id":           traderConfig.ExchangeID,
		"strategy_id":           traderConfig.StrategyID,
		"initial_balance":       traderConfig.InitialBalance,
		"scan_interval_minutes": traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":      traderConfig.BTCETHLeverage,
		"altcoin_leverage":      traderConfig.AltcoinLeverage,
		"trading_symbols":       traderConfig.TradingSymbols,
		"custom_prompt":         traderConfig.CustomPrompt,
		"override_base_prompt":  traderConfig.OverrideBasePrompt,
		"is_cross_margin":       traderConfig.IsCrossMargin,
		"use_ai500":             traderConfig.UseAI500,
		"use_oi_top":            traderConfig.UseOITop,
		"is_running":            isRunning,
	}

	c.JSON(http.StatusOK, result)
}

// handleStatus System status
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleAccount Account information
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	logger.Infof("📊 Received account info request [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		SafeInternalError(c, "Get account info", err)
		return
	}

	logger.Infof("✓ Returning account info [%s]: equity=%.2f, available=%.2f, pnl=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions Position list
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		SafeInternalError(c, "Get positions", err)
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handlePositionHistory Historical closed positions with statistics
func (s *Server) handlePositionHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get optional query parameters
	limitStr := c.DefaultQuery("limit", "100")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}

	// Get store
	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	// Get closed positions
	positions, err := store.Position().GetClosedPositions(trader.GetID(), limit)
	if err != nil {
		SafeInternalError(c, "Get position history", err)
		return
	}

	// Get statistics
	stats, _ := store.Position().GetFullStats(trader.GetID())

	// Get symbol stats
	symbolStats, _ := store.Position().GetSymbolStats(trader.GetID(), 10)

	// Get direction stats
	directionStats, _ := store.Position().GetDirectionStats(trader.GetID())

	c.JSON(http.StatusOK, gin.H{
		"positions":       positions,
		"stats":           stats,
		"symbol_stats":    symbolStats,
		"direction_stats": directionStats,
	})
}

// handleTrades Historical trades list
func (s *Server) handleTrades(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get optional query parameters
	symbol := c.Query("symbol")
	limitStr := c.DefaultQuery("limit", "100")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	// Normalize symbol (add USDT suffix if not present)
	if symbol != "" {
		symbol = market.Normalize(symbol)
	}

	// Get trades from store
	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	allTrades, err := store.Position().GetRecentTrades(trader.GetID(), limit)
	if err != nil {
		SafeInternalError(c, "Get trades", err)
		return
	}

	// Filter by symbol if specified
	if symbol != "" {
		var result []interface{}
		for _, trade := range allTrades {
			if trade.Symbol == symbol {
				result = append(result, trade)
			}
		}
		c.JSON(http.StatusOK, result)
		return
	}

	c.JSON(http.StatusOK, allTrades)
}

// handleOrders Order list (all orders including open, close, stop loss, take profit, etc.)
func (s *Server) handleOrders(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get optional query parameters
	symbol := c.Query("symbol")
	statusFilter := c.Query("status") // NEW, FILLED, CANCELED, etc.
	limitStr := c.DefaultQuery("limit", "100")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	// Normalize symbol (add USDT suffix if not present)
	if symbol != "" {
		symbol = market.Normalize(symbol)
	}

	// Get orders from store
	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	// Get orders with filters applied at database level
	orders, err := store.Order().GetTraderOrdersFiltered(trader.GetID(), symbol, statusFilter, limit)
	if err != nil {
		SafeInternalError(c, "Get orders", err)
		return
	}

	c.JSON(http.StatusOK, orders)
}

// handleOrderFills Order fill details (all fills for a specific order)
func (s *Server) handleOrderFills(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	// Get fills for this order
	fills, err := store.Order().GetOrderFills(orderID)
	if err != nil {
		SafeInternalError(c, "Get order fills", err)
		return
	}

	c.JSON(http.StatusOK, fills)
}

// handleOpenOrders Get open orders (pending SL/TP) from exchange
func (s *Server) handleOpenOrders(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get symbol parameter (required for exchange query)
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	// Normalize symbol
	symbol = market.Normalize(symbol)

	// Get open orders from exchange
	openOrders, err := trader.GetOpenOrders(symbol)
	if err != nil {
		SafeInternalError(c, "Get open orders", err)
		return
	}

	c.JSON(http.StatusOK, openOrders)
}

// handleKlines K-line data (supports multiple exchanges via coinank)
func (s *Server) handleKlines(c *gin.Context) {
	// Get query parameters
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol parameter is required"})
		return
	}

	interval := c.DefaultQuery("interval", "5m")
	exchange := c.DefaultQuery("exchange", "binance") // Default to binance for backward compatibility
	limitStr := c.DefaultQuery("limit", "1000")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 1000
	}

	// Coinank API has a maximum limit of 1500 klines per request
	if limit > 1500 {
		limit = 1500
	}

	var klines []market.Kline
	exchangeLower := strings.ToLower(exchange)

	// Route to appropriate data source based on exchange type
	switch exchangeLower {
	case "alpaca":
		// US Stocks via Alpaca
		klines, err = s.getKlinesFromAlpaca(symbol, interval, limit)
		if err != nil {
			SafeInternalError(c, "Get klines from Alpaca", err)
			return
		}
	case "forex", "metals":
		// Forex and Metals via Twelve Data
		klines, err = s.getKlinesFromTwelveData(symbol, interval, limit)
		if err != nil {
			SafeInternalError(c, "Get klines from TwelveData", err)
			return
		}
	case "hyperliquid", "hyperliquid-xyz", "xyz":
		// Hyperliquid native API - supports both crypto perps and stock perps (xyz dex)
		klines, err = s.getKlinesFromHyperliquid(symbol, interval, limit)
		if err != nil {
			SafeInternalError(c, "Get klines from Hyperliquid", err)
			return
		}
	default:
		// Crypto exchanges via CoinAnk
		symbol = market.Normalize(symbol)
		klines, err = s.getKlinesFromCoinank(symbol, interval, exchange, limit)
		if err != nil {
			SafeInternalError(c, "Get klines from CoinAnk", err)
			return
		}
	}

	c.JSON(http.StatusOK, klines)
}

// getKlinesFromCoinank fetches kline data from coinank free/open API for multiple exchanges
func (s *Server) getKlinesFromCoinank(symbol, interval, exchange string, limit int) ([]market.Kline, error) {
	// Map exchange string to coinank enum
	var coinankExchange coinank_enum.Exchange
	switch strings.ToLower(exchange) {
	case "binance":
		coinankExchange = coinank_enum.Binance
	case "bybit":
		coinankExchange = coinank_enum.Bybit
	case "okx":
		coinankExchange = coinank_enum.Okex
	case "bitget":
		coinankExchange = coinank_enum.Bitget
	case "gate":
		coinankExchange = coinank_enum.Gate
	case "aster":
		coinankExchange = coinank_enum.Aster
	case "lighter":
		// Lighter doesn't have direct CoinAnk support, use Binance data as fallback
		coinankExchange = coinank_enum.Binance
	case "kucoin":
		// KuCoin doesn't have direct CoinAnk support, use Binance data as fallback
		coinankExchange = coinank_enum.Binance
	default:
		// For any unknown exchange, default to Binance
		logger.Warnf("⚠️ Unknown exchange '%s', defaulting to Binance for CoinAnk", exchange)
		coinankExchange = coinank_enum.Binance
	}

	// Map interval string to coinank enum
	var coinankInterval coinank_enum.Interval
	switch interval {
	case "1s":
		coinankInterval = coinank_enum.Second1
	case "5s":
		coinankInterval = coinank_enum.Second5
	case "10s":
		coinankInterval = coinank_enum.Second10
	case "30s":
		coinankInterval = coinank_enum.Second30
	case "1m":
		coinankInterval = coinank_enum.Minute1
	case "3m":
		coinankInterval = coinank_enum.Minute3
	case "5m":
		coinankInterval = coinank_enum.Minute5
	case "10m":
		coinankInterval = coinank_enum.Minute10
	case "15m":
		coinankInterval = coinank_enum.Minute15
	case "30m":
		coinankInterval = coinank_enum.Minute30
	case "1h":
		coinankInterval = coinank_enum.Hour1
	case "2h":
		coinankInterval = coinank_enum.Hour2
	case "4h":
		coinankInterval = coinank_enum.Hour4
	case "6h":
		coinankInterval = coinank_enum.Hour6
	case "8h":
		coinankInterval = coinank_enum.Hour8
	case "12h":
		coinankInterval = coinank_enum.Hour12
	case "1d":
		coinankInterval = coinank_enum.Day1
	case "3d":
		coinankInterval = coinank_enum.Day3
	case "1w":
		coinankInterval = coinank_enum.Week1
	case "1M":
		coinankInterval = coinank_enum.Month1
	default:
		return nil, fmt.Errorf("unsupported interval for coinank: %s", interval)
	}

	// Convert symbol format for different exchanges
	// OKX uses "BTC-USDT-SWAP" format instead of "BTCUSDT"
	apiSymbol := symbol
	if coinankExchange == coinank_enum.Okex {
		// Convert BTCUSDT -> BTC-USDT-SWAP
		if strings.HasSuffix(symbol, "USDT") {
			base := strings.TrimSuffix(symbol, "USDT")
			apiSymbol = fmt.Sprintf("%s-USDT-SWAP", base)
		}
	}

	// Call coinank free/open API (no authentication required)
	ctx := context.Background()
	ts := time.Now().UnixMilli()
	// Use "To" side to search backward from current time (get historical klines)
	coinankKlines, err := coinank_api.Kline(ctx, apiSymbol, coinankExchange, ts, coinank_enum.To, limit, coinankInterval)
	if err != nil {
		// Free API doesn't support all exchanges (e.g., OKX, Bitget)
		// Fallback to Binance data as reference
		if coinankExchange != coinank_enum.Binance {
			logger.Warnf("⚠️ CoinAnk free API doesn't support %s, falling back to Binance data", coinankExchange)
			coinankKlines, err = coinank_api.Kline(ctx, symbol, coinank_enum.Binance, ts, coinank_enum.To, limit, coinankInterval)
			if err != nil {
				return nil, fmt.Errorf("coinank API error (fallback): %w", err)
			}
		} else {
			return nil, fmt.Errorf("coinank API error: %w", err)
		}
	}

	// Convert coinank kline format to market.Kline format
	// Coinank: Volume = BTC 数量, Quantity = USDT 成交额
	klines := make([]market.Kline, len(coinankKlines))
	for i, ck := range coinankKlines {
		klines[i] = market.Kline{
			OpenTime:    ck.StartTime,
			Open:        ck.Open,
			High:        ck.High,
			Low:         ck.Low,
			Close:       ck.Close,
			Volume:      ck.Volume,   // BTC 数量
			QuoteVolume: ck.Quantity, // USDT 成交额
			CloseTime:   ck.EndTime,
		}
	}

	return klines, nil
}

// getKlinesFromAlpaca fetches kline data from Alpaca API for US stocks
func (s *Server) getKlinesFromAlpaca(symbol, interval string, limit int) ([]market.Kline, error) {
	// Create Alpaca client
	client := alpaca.NewClient()

	// Map interval to Alpaca timeframe format
	timeframe := alpaca.MapTimeframe(interval)

	// Fetch bars from Alpaca
	ctx := context.Background()
	bars, err := client.GetBars(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("alpaca API error: %w", err)
	}

	// Convert Alpaca bars to market.Kline format
	klines := make([]market.Kline, len(bars))
	for i, bar := range bars {
		klines[i] = market.Kline{
			OpenTime:    bar.Timestamp.UnixMilli(),
			Open:        bar.Open,
			High:        bar.High,
			Low:         bar.Low,
			Close:       bar.Close,
			Volume:      float64(bar.Volume),             // 股数
			QuoteVolume: float64(bar.Volume) * bar.Close, // 成交额 = 股数 * 收盘价 (USD)
			CloseTime:   bar.Timestamp.UnixMilli(),
		}
	}

	return klines, nil
}

// getKlinesFromTwelveData fetches kline data from Twelve Data API for forex and metals
func (s *Server) getKlinesFromTwelveData(symbol, interval string, limit int) ([]market.Kline, error) {
	// Create Twelve Data client
	client := twelvedata.NewClient()

	// Map interval to Twelve Data timeframe format
	timeframe := twelvedata.MapTimeframe(interval)

	// Fetch time series from Twelve Data
	ctx := context.Background()
	result, err := client.GetTimeSeries(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("twelvedata API error: %w", err)
	}

	// Convert Twelve Data bars to market.Kline format
	// Note: Twelve Data returns bars in reverse order (newest first)
	klines := make([]market.Kline, len(result.Values))
	for i, bar := range result.Values {
		open, high, low, close, volume, timestamp, err := twelvedata.ParseBar(bar)
		if err != nil {
			logger.Warnf("⚠️ Failed to parse TwelveData bar: %v", err)
			continue
		}

		// Reverse order: put oldest first
		idx := len(result.Values) - 1 - i
		klines[idx] = market.Kline{
			OpenTime:  timestamp,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: timestamp,
		}
	}

	return klines, nil
}

// getKlinesFromHyperliquid fetches kline data from Hyperliquid API
// Supports both crypto perps (default dex) and stock perps/forex/commodities (xyz dex)
func (s *Server) getKlinesFromHyperliquid(symbol, interval string, limit int) ([]market.Kline, error) {
	// Create Hyperliquid client
	client := hyperliquid.NewClient()

	// Map interval to Hyperliquid format
	timeframe := hyperliquid.MapTimeframe(interval)

	// Fetch candles from Hyperliquid
	// FormatCoinForAPI will automatically add xyz: prefix for stock perps
	ctx := context.Background()
	candles, err := client.GetCandles(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("hyperliquid API error: %w", err)
	}

	// Convert Hyperliquid candles to market.Kline format
	klines := make([]market.Kline, len(candles))
	for i, candle := range candles {
		open, _ := strconv.ParseFloat(candle.Open, 64)
		high, _ := strconv.ParseFloat(candle.High, 64)
		low, _ := strconv.ParseFloat(candle.Low, 64)
		close, _ := strconv.ParseFloat(candle.Close, 64)
		volume, _ := strconv.ParseFloat(candle.Volume, 64)

		klines[i] = market.Kline{
			OpenTime:    candle.OpenTime,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       close,
			Volume:      volume,         // 合约数量
			QuoteVolume: volume * close, // 成交额 (USD)
			CloseTime:   candle.CloseTime,
		}
	}

	return klines, nil
}

// handleSymbols returns available symbols for a given exchange
func (s *Server) handleSymbols(c *gin.Context) {
	exchange := c.DefaultQuery("exchange", "hyperliquid")

	type SymbolInfo struct {
		Symbol      string `json:"symbol"`
		Name        string `json:"name"`
		Category    string `json:"category"` // crypto, stock, forex, commodity, index
		MaxLeverage int    `json:"maxLeverage,omitempty"`
	}

	var symbols []SymbolInfo

	switch strings.ToLower(exchange) {
	case "hyperliquid", "hyperliquid-xyz", "xyz":
		// Fetch symbols from Hyperliquid
		client := hyperliquid.NewClient()
		ctx := context.Background()

		// Get crypto perps from default dex
		if exchange == "hyperliquid" || exchange == "hyperliquid-xyz" {
			mids, err := client.GetAllMids(ctx)
			if err == nil {
				for symbol := range mids {
					// Skip spot tokens (start with @)
					if strings.HasPrefix(symbol, "@") {
						continue
					}
					symbols = append(symbols, SymbolInfo{
						Symbol:   symbol,
						Name:     symbol,
						Category: "crypto",
					})
				}
			}
		}

		// Get xyz dex symbols (stocks, forex, commodities)
		xyzMids, err := client.GetAllMidsXYZ(ctx)
		if err == nil {
			for symbol := range xyzMids {
				// Remove xyz: prefix for display
				displaySymbol := strings.TrimPrefix(symbol, "xyz:")
				category := "stock"
				if displaySymbol == "GOLD" || displaySymbol == "SILVER" {
					category = "commodity"
				} else if displaySymbol == "EUR" || displaySymbol == "JPY" {
					category = "forex"
				} else if displaySymbol == "XYZ100" {
					category = "index"
				}
				symbols = append(symbols, SymbolInfo{
					Symbol:   displaySymbol,
					Name:     displaySymbol,
					Category: category,
				})
			}
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange for symbol listing"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"exchange": exchange,
		"symbols":  symbols,
		"count":    len(symbols),
	})
}

// handleDecisions Decision log list
func (s *Server) handleDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get all historical decision records (unlimited)
	records, err := trader.GetStore().Decision().GetLatestRecords(trader.GetID(), 10000)
	if err != nil {
		SafeInternalError(c, "Get decision log", err)
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions Latest decision logs (newest first, supports limit parameter)
func (s *Server) handleLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	// Get limit from query parameter, default to 5
	limit := 5
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 100 {
				limit = 100 // Max 100 to prevent abuse
			}
		}
	}

	records, err := trader.GetStore().Decision().GetLatestRecords(trader.GetID(), limit)
	if err != nil {
		SafeInternalError(c, "Get decision log", err)
		return
	}

	// Reverse array to put newest first (for list display)
	// GetLatestRecords returns oldest to newest (for charts), here we need newest to oldest
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics Statistics information
func (s *Server) handleStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	stats, err := trader.GetStore().Decision().GetStatistics(trader.GetID())
	if err != nil {
		SafeInternalError(c, "Get statistics", err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleCompetition Competition overview (compare all traders)
func (s *Server) handleCompetition(c *gin.Context) {
	userID := c.GetString("user_id")

	// Ensure user's traders are loaded into memory
	err := s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to load traders for user %s: %v", userID, err)
	}

	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		SafeInternalError(c, "Get competition data", err)
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleEquityHistory Return rate historical data
// Query directly from database, not dependent on trader in memory (so historical data can be retrieved after restart)
func (s *Server) handleEquityHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	// Get equity historical data from new equity table
	// Every 3 minutes per cycle: 10000 records = about 20 days of data
	snapshots, err := s.store.Equity().GetLatest(traderID, 10000)
	if err != nil {
		SafeInternalError(c, "Get historical data", err)
		return
	}

	if len(snapshots) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	// Build return rate historical data points
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // Account equity (wallet + unrealized)
		AvailableBalance float64 `json:"available_balance"` // Available balance
		TotalPnL         float64 `json:"total_pnl"`         // Total PnL (unrealized PnL)
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total PnL percentage
		PositionCount    int     `json:"position_count"`    // Position count
		MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin used percentage
	}

	// Use the balance of the first record as initial balance to calculate return rate
	initialBalance := snapshots[0].Balance
	if initialBalance == 0 {
		initialBalance = 1 // Avoid division by zero
	}

	var history []EquityPoint
	for _, snap := range snapshots {
		// Calculate PnL percentage
		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (snap.UnrealizedPnL / initialBalance) * 100
		}

		history = append(history, EquityPoint{
			Timestamp:        snap.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      snap.TotalEquity,
			AvailableBalance: snap.Balance,
			TotalPnL:         snap.UnrealizedPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    snap.PositionCount,
			MarginUsedPct:    snap.MarginUsedPct,
		})
	}

	c.JSON(http.StatusOK, history)
}

// authMiddleware JWT authentication middleware
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			c.Abort()
			return
		}

		// Check Bearer token format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Blacklist check
		if auth.IsTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired, please login again"})
			c.Abort()
			return
		}

		// Validate JWT token
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			logger.Errorf("[Auth] Invalid token: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

// handleLogout Add current token to blacklist
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// handleRegister Handle user registration request.
// handleRegister allows registration only when no users exist yet (first-time setup).
// This is a single-user system; subsequent registrations are permanently closed.
func (s *Server) handleRegister(c *gin.Context) {
	userCount, err := s.store.User().Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user count"})
		return
	}

	if userCount > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "System already initialized"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Check if email already exists
	_, err = s.store.User().GetByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Generate password hash
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password processing failed"})
		return
	}

	// Create user
	userID := uuid.New().String()
	user := &store.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
	}

	err = s.store.User().Create(user)
	if err != nil {
		SafeInternalError(c, "Failed to create user", err)
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Initialize default model and exchange configs for user
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		logger.Infof("Failed to initialize user default configs: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "Registration successful",
	})
}

// handleLogin Handle user login request
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Get user information
	user, err := s.store.User().GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}

	// Issue token directly after password verification.
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "Login successful",
	})
}

// handleChangePassword changes the password for the currently authenticated user.
func (s *Server) handleChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "new_password is required (min 8 chars)")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		SafeInternalError(c, "Password processing failed", err)
		return
	}
	if err := s.store.User().UpdatePassword(userID, hash); err != nil {
		SafeInternalError(c, "Failed to update password", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}

// handleResetPassword Reset password via email and new password
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Query user
	user, err := s.store.User().GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email does not exist"})
		return
	}

	// Generate new password hash
	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password processing failed"})
		return
	}

	// Update password
	err = s.store.User().UpdatePassword(user.ID, newPasswordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password update failed"})
		return
	}

	logger.Infof("✓ User %s password has been reset", user.Email)
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successful, please login with new password"})
}

// initUserDefaultConfigs Initialize default model and exchange configs for new user
func (s *Server) initUserDefaultConfigs(userID string) error {
	// Commented out auto-creation of default configs, let users add manually
	// This way new users won't have config items automatically after registration
	logger.Infof("User %s registration completed, waiting for manual AI model and exchange configuration", userID)
	return nil
}

// handleGetSupportedModels Get list of AI models supported by the system
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// Return static list of supported AI models with default versions
	supportedModels := []map[string]interface{}{
		{"id": "deepseek", "name": "DeepSeek", "provider": "deepseek", "defaultModel": "deepseek-chat"},
		{"id": "qwen", "name": "Qwen", "provider": "qwen", "defaultModel": "qwen3-max"},
		{"id": "openai", "name": "OpenAI", "provider": "openai", "defaultModel": "gpt-5.1"},
		{"id": "claude", "name": "Claude", "provider": "claude", "defaultModel": "claude-opus-4-6"},
		{"id": "gemini", "name": "Google Gemini", "provider": "gemini", "defaultModel": "gemini-3-pro-preview"},
		{"id": "grok", "name": "Grok (xAI)", "provider": "grok", "defaultModel": "grok-3-latest"},
		{"id": "kimi", "name": "Kimi (Moonshot)", "provider": "kimi", "defaultModel": "moonshot-v1-auto"},
		{"id": "minimax", "name": "MiniMax", "provider": "minimax", "defaultModel": "MiniMax-M2.5"},
		{"id": "blockrun-base", "name": "BlockRun (Base Wallet)", "provider": "blockrun-base", "defaultModel": "auto"},
		{"id": "blockrun-sol", "name": "BlockRun (Solana Wallet)", "provider": "blockrun-sol", "defaultModel": "auto"},
		{"id": "claw402", "name": "Claw402 (Base USDC)", "provider": "claw402", "defaultModel": "deepseek"},
	}

	c.JSON(http.StatusOK, supportedModels)
}

// handleGetSupportedExchanges Get list of exchanges supported by the system
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// Return static list of supported exchange types
	// Note: ID is empty for supported exchanges (they are templates, not actual accounts)
	supportedExchanges := []SafeExchangeConfig{
		{ExchangeType: "binance", Name: "Binance Futures", Type: "cex"},
		{ExchangeType: "bybit", Name: "Bybit Futures", Type: "cex"},
		{ExchangeType: "okx", Name: "OKX Futures", Type: "cex"},
		{ExchangeType: "gate", Name: "Gate.io Futures", Type: "cex"},
		{ExchangeType: "kucoin", Name: "KuCoin Futures", Type: "cex"},
		{ExchangeType: "hyperliquid", Name: "Hyperliquid", Type: "dex"},
		{ExchangeType: "aster", Name: "Aster DEX", Type: "dex"},
		{ExchangeType: "lighter", Name: "LIGHTER DEX", Type: "dex"},
		{ExchangeType: "alpaca", Name: "Alpaca (US Stocks)", Type: "stock"},
		{ExchangeType: "forex", Name: "Forex (TwelveData)", Type: "forex"},
		{ExchangeType: "metals", Name: "Metals (TwelveData)", Type: "metals"},
	}

	c.JSON(http.StatusOK, supportedExchanges)
}

// Start Start server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	logger.Infof("🌐 API server starting at http://localhost%s", addr)
	logger.Infof("📊 API Documentation:")
	logger.Infof("  • GET  /api/health           - Health check")
	logger.Infof("  • GET  /api/traders          - Public AI trader leaderboard top 50 (no auth required)")
	logger.Infof("  • GET  /api/competition      - Public competition data (no auth required)")
	logger.Infof("  • GET  /api/top-traders      - Top 5 trader data (no auth required, for performance comparison)")
	logger.Infof("  • GET  /api/equity-history?trader_id=xxx - Public return rate historical data (no auth required, for competition)")
	logger.Infof("  • GET  /api/equity-history-batch?trader_ids=a,b,c - Batch get historical data (no auth required, performance comparison optimization)")
	logger.Infof("  • GET  /api/traders/:id/public-config - Public trader config (no auth required, no sensitive info)")
	logger.Infof("  • POST /api/traders          - Create new AI trader")
	logger.Infof("  • DELETE /api/traders/:id    - Delete AI trader")
	logger.Infof("  • POST /api/traders/:id/start - Start AI trader")
	logger.Infof("  • POST /api/traders/:id/stop  - Stop AI trader")
	logger.Infof("  • GET  /api/models           - Get AI model config")
	logger.Infof("  • PUT  /api/models           - Update AI model config")
	logger.Infof("  • GET  /api/exchanges        - Get exchange config")
	logger.Infof("  • PUT  /api/exchanges        - Update exchange config")
	logger.Infof("  • GET  /api/status?trader_id=xxx     - Specified trader's system status")
	logger.Infof("  • GET  /api/account?trader_id=xxx    - Specified trader's account info")
	logger.Infof("  • GET  /api/positions?trader_id=xxx  - Specified trader's position list")
	logger.Infof("  • GET  /api/decisions?trader_id=xxx  - Specified trader's decision log")
	logger.Infof("  • GET  /api/decisions/latest?trader_id=xxx - Specified trader's latest decisions")
	logger.Infof("  • GET  /api/statistics?trader_id=xxx - Specified trader's statistics")
	logger.Infof("  • GET  /api/performance?trader_id=xxx - Specified trader's AI learning performance analysis")
	logger.Info()

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown Gracefully shutdown server
func (s *Server) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// handlePublicTraderList Get public trader list (no authentication required)
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// Get trader information from all users
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		SafeInternalError(c, "Get trader list", err)
		return
	}

	// Get traders array
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Trader data format error",
		})
		return
	}

	// Return trader basic information, filter sensitive information
	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		result = append(result, map[string]interface{}{
			"trader_id":       trader["trader_id"],
			"trader_name":     trader["trader_name"],
			"ai_model":        trader["ai_model"],
			"exchange":        trader["exchange"],
			"is_running":      trader["is_running"],
			"total_equity":    trader["total_equity"],
			"total_pnl":       trader["total_pnl"],
			"total_pnl_pct":   trader["total_pnl_pct"],
			"position_count":  trader["position_count"],
			"margin_used_pct": trader["margin_used_pct"],
		})
	}

	c.JSON(http.StatusOK, result)
}

// handlePublicCompetition Get public competition data (no authentication required)
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		SafeInternalError(c, "Get competition data", err)
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders Get top 5 trader data (no authentication required, for performance comparison)
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData()
	if err != nil {
		SafeInternalError(c, "Get top traders data", err)
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch Batch get return rate historical data for multiple traders (no authentication required, for performance comparison)
// Supports optional 'hours' parameter to filter data by time range (e.g., hours=24 for last 24 hours)
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
		Hours     int      `json:"hours"` // Optional: filter by last N hours (0 = all data)
	}

	// Try to parse POST request JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// If JSON parse fails, try to get from query parameters (compatible with GET request)
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// If no trader_ids specified, return historical data for top 5
			topTraders, err := s.traderManager.GetTopTradersData()
			if err != nil {
				SafeInternalError(c, "Get top traders", err)
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Trader data format error"})
				return
			}

			// Extract trader IDs
			traderIDs := make([]string, 0, len(traders))
			for _, trader := range traders {
				if traderID, ok := trader["trader_id"].(string); ok {
					traderIDs = append(traderIDs, traderID)
				}
			}

			// Parse hours parameter from query
			hoursParam := c.Query("hours")
			hours := 0
			if hoursParam != "" {
				fmt.Sscanf(hoursParam, "%d", &hours)
			}

			result := s.getEquityHistoryForTraders(traderIDs, hours)
			c.JSON(http.StatusOK, result)
			return
		}

		// Parse comma-separated trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}

		// Parse hours parameter from query
		hoursParam := c.Query("hours")
		if hoursParam != "" {
			fmt.Sscanf(hoursParam, "%d", &requestBody.Hours)
		}
	}

	// Limit to maximum 20 traders to prevent oversized requests
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs, requestBody.Hours)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders Get historical data for multiple traders
// Query directly from database, not dependent on trader in memory (so historical data can be retrieved after restart)
// Also appends current real-time data point to ensure chart matches leaderboard
// hours: filter by last N hours (0 = use default limit of 500 records)
func (s *Server) getEquityHistoryForTraders(traderIDs []string, hours int) map[string]interface{} {
	result := make(map[string]interface{})
	histories := make(map[string]interface{})
	errors := make(map[string]string)

	// Use a single consistent timestamp for all real-time data points
	now := time.Now()

	// Pre-fetch initial balances for all traders
	initialBalances := make(map[string]float64)
	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}
		// Get trader's initial balance from database (use GetByID which doesn't require userID)
		trader, err := s.store.Trader().GetByID(traderID)
		if err == nil && trader != nil && trader.InitialBalance > 0 {
			initialBalances[traderID] = trader.InitialBalance
		}
	}

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		// Get equity historical data from new equity table
		var snapshots []*store.EquitySnapshot
		var err error

		if hours > 0 {
			// Filter by time range
			startTime := now.Add(-time.Duration(hours) * time.Hour)
			snapshots, err = s.store.Equity().GetByTimeRange(traderID, startTime, now)
		} else {
			// Default: get latest 500 records
			snapshots, err = s.store.Equity().GetLatest(traderID, 500)
		}
		if err != nil {
			logger.Errorf("[API] Failed to get equity history for %s: %v", traderID, err)
			errors[traderID] = "Failed to get historical data"
			continue
		}

		// Get initial balance for calculating PnL percentage
		initialBalance := initialBalances[traderID]
		if initialBalance <= 0 && len(snapshots) > 0 {
			// If no initial balance configured, use the first snapshot's equity as baseline
			initialBalance = snapshots[0].TotalEquity
		}

		// Build return rate historical data with PnL percentage
		history := make([]map[string]interface{}, 0, len(snapshots)+1)
		var lastSnapshotTime time.Time
		for _, snap := range snapshots {
			// Calculate PnL percentage: (current_equity - initial_balance) / initial_balance * 100
			pnlPct := 0.0
			if initialBalance > 0 {
				pnlPct = (snap.TotalEquity - initialBalance) / initialBalance * 100
			}

			history = append(history, map[string]interface{}{
				"timestamp":     snap.Timestamp,
				"total_equity":  snap.TotalEquity,
				"total_pnl":     snap.UnrealizedPnL,
				"total_pnl_pct": pnlPct,
				"balance":       snap.Balance,
			})
			if snap.Timestamp.After(lastSnapshotTime) {
				lastSnapshotTime = snap.Timestamp
			}
		}

		// Append current real-time data point to ensure chart matches leaderboard
		// This ensures the latest point is always current, not from a potentially stale snapshot
		if trader, err := s.traderManager.GetTrader(traderID); err == nil {
			if accountInfo, err := trader.GetAccountInfo(); err == nil {
				// Only append if it's been more than 30 seconds since last snapshot
				if now.Sub(lastSnapshotTime) > 30*time.Second {
					totalEquity := 0.0
					if v, ok := accountInfo["total_equity"].(float64); ok {
						totalEquity = v
					}
					totalPnL := 0.0
					if v, ok := accountInfo["total_pnl"].(float64); ok {
						totalPnL = v
					}
					walletBalance := 0.0
					if v, ok := accountInfo["wallet_balance"].(float64); ok {
						walletBalance = v
					}
					pnlPct := 0.0
					if initialBalance > 0 {
						pnlPct = (totalEquity - initialBalance) / initialBalance * 100
					}

					history = append(history, map[string]interface{}{
						"timestamp":     now,
						"total_equity":  totalEquity,
						"total_pnl":     totalPnL,
						"total_pnl_pct": pnlPct,
						"balance":       walletBalance,
					})
				}
			}
		}

		histories[traderID] = history
	}

	result["histories"] = histories
	result["count"] = len(histories)
	if len(errors) > 0 {
		result["errors"] = errors
	}

	return result
}

// handleGetPublicTraderConfig Get public trader configuration information (no authentication required, does not include sensitive information)
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader ID cannot be empty"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Get trader status information
	status := trader.GetStatus()

	// Only return public configuration information, not including sensitive data like API keys
	result := map[string]interface{}{
		"trader_id":   trader.GetID(),
		"trader_name": trader.GetName(),
		"ai_model":    trader.GetAIModel(),
		"exchange":    trader.GetExchange(),
		"is_running":  status["is_running"],
		"ai_provider": status["ai_provider"],
		"start_time":  status["start_time"],
	}

	c.JSON(http.StatusOK, result)
}

// SetTelegramReloadCh sets the channel used to signal the Telegram bot to reload
func (s *Server) SetTelegramReloadCh(ch chan<- struct{}) {
	s.telegramReloadCh = ch
}

// handleGetTelegramConfig returns current Telegram bot configuration and binding status
func (s *Server) handleGetTelegramConfig(c *gin.Context) {
	cfg, err := s.store.TelegramConfig().Get()
	if err != nil {
		// Not configured yet - return empty state
		c.JSON(http.StatusOK, gin.H{
			"configured":   false,
			"is_bound":     false,
			"token_masked": "",
			"username":     "",
		})
		return
	}

	// Mask bot token for security (show only last 6 chars)
	tokenMasked := ""
	if cfg.BotToken != "" {
		if len(cfg.BotToken) > 6 {
			tokenMasked = "***" + cfg.BotToken[len(cfg.BotToken)-6:]
		} else {
			tokenMasked = "***"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"configured":   cfg.BotToken != "",
		"is_bound":     cfg.ChatID != 0,
		"username":     cfg.Username,
		"bound_at":     cfg.BoundAt,
		"token_masked": tokenMasked,
		"model_id":     cfg.ModelID,
	})
}

// handleUpdateTelegramConfig saves bot token (+ optional model ID) and triggers bot hot-reload
func (s *Server) handleUpdateTelegramConfig(c *gin.Context) {
	var req struct {
		BotToken string `json:"bot_token"`
		ModelID  string `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.BotToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_token is required"})
		return
	}

	if err := s.store.TelegramConfig().Save(req.BotToken, req.ModelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	// Signal bot hot-reload if channel is available
	if s.telegramReloadCh != nil {
		select {
		case s.telegramReloadCh <- struct{}{}:
		default: // non-blocking
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Bot token saved. Bot will reload automatically."})
}

// handleUnbindTelegram removes Telegram user binding
func (s *Server) handleUnbindTelegram(c *gin.Context) {
	if err := s.store.TelegramConfig().Unbind(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unbind"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Telegram binding removed"})
}

// handleUpdateTelegramModel updates only the AI model used for Telegram replies (no token re-entry needed)
func (s *Server) handleUpdateTelegramModel(c *gin.Context) {
	var req struct {
		ModelID string `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	cfg, err := s.store.TelegramConfig().Get()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no Telegram config found, save a bot token first"})
		return
	}

	if err := s.store.TelegramConfig().Save(cfg.BotToken, req.ModelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save model config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "model_id": req.ModelID})
}
