package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nofx/kernel"
	"nofx/mcp"
	"nofx/safe"
	"nofx/security"
	"nofx/store"
)

// cachedTools holds the static tool definitions (built once, reused per message).
var cachedTools = buildAgentTools()

// agentTools returns the tools available to the LLM for autonomous action.
func agentTools() []mcp.Tool { return cachedTools }

func buildAgentTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_preferences",
				Description: "Get all persistent user preferences that the agent should remember long-term.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "manage_preferences",
				Description: "Add, update, or delete a persistent user preference. Use this when the user asks to remember something long-term, change an existing long-term preference, or remove one.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"add", "update", "delete"},
							"description": "What to do with the persistent preference.",
						},
						"text": map[string]any{
							"type":        "string",
							"description": "The new preference text. Required for add and update.",
						},
						"match": map[string]any{
							"type":        "string",
							"description": "How to find the existing preference to update or delete. Can be an id or distinctive text like '每天8点'.",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_backend_logs",
				Description: "Get recent backend log lines for a trader diagnosis. Prefer this when the user asks why a specific trader failed, stopped, or behaved unexpectedly. Returns recent matching log lines for the authenticated user's trader.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"trader_id": map[string]any{
							"type":        "string",
							"description": "Trader id to diagnose. The backend verifies that this trader belongs to the authenticated user before returning logs.",
						},
						"limit":       map[string]any{"type": "number", "description": "Maximum number of recent log lines to return. Default 30."},
						"errors_only": map[string]any{"type": "boolean", "description": "When true, only return error-like log lines. Default true."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_exchange_configs",
				Description: "Get the user's current exchange account bindings. Returns safe metadata only and whether credentials are already stored.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "manage_exchange_config",
				Description: "Create, update, or delete an exchange account binding. Use this when the user asks to add/edit/remove an exchange account, API key, secret, passphrase, wallet address, or account name. Sensitive fields are stored securely and are never returned in full.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"create", "update", "delete"},
						},
						"exchange_id": map[string]any{
							"type":        "string",
							"description": "Existing exchange account id. Required for update and delete.",
						},
						"exchange_type": map[string]any{
							"type":        "string",
							"description": "Exchange type for a new binding, such as binance, bybit, okx, hyperliquid, aster, lighter, gate, kucoin, alpaca, forex, or metals.",
						},
						"account_name": map[string]any{
							"type":        "string",
							"description": "User-visible account name like Main, Testnet, or Mom Account.",
						},
						"enabled": map[string]any{
							"type":        "boolean",
							"description": "Whether this exchange binding should be enabled.",
						},
						"api_key":                     map[string]any{"type": "string"},
						"secret_key":                  map[string]any{"type": "string"},
						"passphrase":                  map[string]any{"type": "string"},
						"testnet":                     map[string]any{"type": "boolean"},
						"hyperliquid_wallet_addr":     map[string]any{"type": "string"},
						"hyperliquid_unified_account": map[string]any{"type": "boolean"},
						"aster_user":                  map[string]any{"type": "string"},
						"aster_signer":                map[string]any{"type": "string"},
						"aster_private_key":           map[string]any{"type": "string"},
						"lighter_wallet_addr":         map[string]any{"type": "string"},
						"lighter_private_key":         map[string]any{"type": "string"},
						"lighter_api_key_private_key": map[string]any{"type": "string"},
						"lighter_api_key_index":       map[string]any{"type": "number"},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_model_configs",
				Description: "Get the user's current AI model bindings. Returns safe metadata only and whether an API key is already stored.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "manage_model_config",
				Description: "Create, update, or delete an AI model binding. Use this when the user asks to add/edit/remove a model provider, API key, custom API URL, or custom model name. Sensitive fields are stored securely and are never returned in full.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"create", "update", "delete"},
						},
						"model_id": map[string]any{
							"type":        "string",
							"description": "Existing model id for update/delete, or the desired id for create.",
						},
						"provider": map[string]any{
							"type":        "string",
							"description": "Provider slug such as openai, claude, gemini, deepseek, qwen, kimi, grok, minimax, claw402, or blockrun-base.",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Display name for a newly created model binding.",
						},
						"enabled":           map[string]any{"type": "boolean"},
						"api_key":           map[string]any{"type": "string"},
						"custom_api_url":    map[string]any{"type": "string"},
						"custom_model_name": map[string]any{"type": "string"},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_strategies",
				Description: "Get the user's current strategy templates, including system default strategies available to that user.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "manage_strategy",
				Description: "List, create, update, delete, activate, duplicate strategies, or get the default strategy config template. Use this when the user asks to create or edit a strategy template. Strategy templates are independent assets and do not require exchange/model bindings unless the user asks to run them via a trader.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"list", "create", "update", "delete", "activate", "duplicate", "get_default_config"},
						},
						"strategy_id":    map[string]any{"type": "string"},
						"name":           map[string]any{"type": "string"},
						"description":    map[string]any{"type": "string"},
						"lang":           map[string]any{"type": "string", "enum": []string{"zh", "en"}},
						"is_public":      map[string]any{"type": "boolean"},
						"config_visible": map[string]any{"type": "boolean"},
						"config":         map[string]any{"type": "object", "description": "Full or partial strategy config JSON object, depending on action."},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "manage_trader",
				Description: "List, create, update, delete, start, or stop traders. Use this when the user asks to create a trader, rename one, switch its exchange/model/strategy, delete it, or control its running state.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"list", "create", "update", "delete", "start", "stop"},
						},
						"trader_id": map[string]any{
							"type":        "string",
							"description": "Required for update, delete, start, and stop.",
						},
						"name":                   map[string]any{"type": "string"},
						"ai_model_id":            map[string]any{"type": "string"},
						"exchange_id":            map[string]any{"type": "string"},
						"strategy_id":            map[string]any{"type": "string"},
						"initial_balance":        map[string]any{"type": "number"},
						"scan_interval_minutes":  map[string]any{"type": "number"},
						"is_cross_margin":        map[string]any{"type": "boolean"},
						"show_in_competition":    map[string]any{"type": "boolean"},
						"btc_eth_leverage":       map[string]any{"type": "number"},
						"altcoin_leverage":       map[string]any{"type": "number"},
						"trading_symbols":        map[string]any{"type": "string"},
						"custom_prompt":          map[string]any{"type": "string"},
						"override_base_prompt":   map[string]any{"type": "boolean"},
						"system_prompt_template": map[string]any{"type": "string"},
						"use_ai500":              map[string]any{"type": "boolean"},
						"use_oi_top":             map[string]any{"type": "boolean"},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "search_stock",
				Description: "Search for a stock by name, ticker symbol, or keyword. Searches across A-share (沪深), Hong Kong, and US markets. Returns a list of matching stocks with their codes. Use this when the user asks about a stock not in your known list, or when you need to find the exact code for a stock.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{
							"type":        "string",
							"description": "Search keyword: stock name (e.g. '宁德时代', '腾讯'), ticker (e.g. 'TSLA', 'AAPL'), or stock code (e.g. '300750')",
						},
					},
					"required": []string{"keyword"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "execute_trade",
				Description: "Execute a trade order (crypto or US stocks). Use this when the user explicitly asks to open/close a position. For stocks (e.g. AAPL, TSLA), use open_long to buy and close_long to sell. This creates a pending trade that requires user confirmation.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"open_long", "open_short", "close_long", "close_short"},
							"description": "Trade action: open_long (做多/buy), open_short (做空/sell), close_long (平多), close_short (平空)",
						},
						"symbol": map[string]any{
							"type":        "string",
							"description": "Trading symbol. For crypto: BTCUSDT, ETHUSDT. For US stocks: AAPL, TSLA, NVDA (no suffix needed).",
						},
						"quantity": map[string]any{
							"type":        "number",
							"description": "Trade quantity/amount. Required for opening positions. Use 0 to close entire position.",
						},
						"leverage": map[string]any{
							"type":        "number",
							"description": "Leverage multiplier (e.g. 5, 10, 20). Optional, defaults to trader's current setting.",
						},
					},
					"required": []string{"action", "symbol", "quantity"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_positions",
				Description: "Get all current open positions across all traders. Returns symbol, side, size, entry price, mark price, and unrealized PnL.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_balance",
				Description: "Get account balance and equity across all traders.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_market_price",
				Description: "Get the current market price for a crypto or stock symbol.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol": map[string]any{
							"type":        "string",
							"description": "Trading symbol, e.g. BTCUSDT for crypto, AAPL for stocks",
						},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_trade_history",
				Description: "Get recent closed trade history with PnL. Use when user asks about past trades, performance, or trade results. Returns the most recent closed positions.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"limit": map[string]any{
							"type":        "number",
							"description": "Number of recent trades to return (default 10, max 50)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_candidate_coins",
				Description: "Get the current candidate coin list for a trader or strategy, including AI500 coin-source settings and the selected symbols.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"trader_id": map[string]any{
							"type":        "string",
							"description": "Optional trader id. Prefer this when asking about a running trader.",
						},
						"strategy_id": map[string]any{
							"type":        "string",
							"description": "Optional strategy id. Use this when asking about a strategy template directly.",
						},
					},
				},
			},
		},
	}
}

// handleToolCall processes a single tool call from the LLM and returns the result.
func (a *Agent) handleToolCall(ctx context.Context, storeUserID string, userID int64, lang string, tc mcp.ToolCall) string {
	switch tc.Function.Name {
	case "get_preferences":
		return a.toolGetPreferences(userID)
	case "manage_preferences":
		return a.toolManagePreferences(userID, tc.Function.Arguments)
	case "get_backend_logs":
		return a.toolGetBackendLogs(storeUserID, tc.Function.Arguments)
	case "get_exchange_configs":
		return a.toolGetExchangeConfigs(storeUserID)
	case "manage_exchange_config":
		return a.toolManageExchangeConfig(storeUserID, tc.Function.Arguments)
	case "get_model_configs":
		return a.toolGetModelConfigs(storeUserID)
	case "manage_model_config":
		return a.toolManageModelConfig(storeUserID, tc.Function.Arguments)
	case "get_strategies":
		return a.toolGetStrategies(storeUserID)
	case "manage_strategy":
		return a.toolManageStrategy(storeUserID, tc.Function.Arguments)
	case "manage_trader":
		return a.toolManageTrader(storeUserID, tc.Function.Arguments)
	case "search_stock":
		return a.toolSearchStock(tc.Function.Arguments)
	case "execute_trade":
		return a.toolExecuteTrade(ctx, userID, lang, tc.Function.Arguments)
	case "get_positions":
		return a.toolGetPositions()
	case "get_balance":
		return a.toolGetBalance()
	case "get_market_price":
		return a.toolGetMarketPrice(tc.Function.Arguments)
	case "get_trade_history":
		return a.toolGetTradeHistory(tc.Function.Arguments)
	case "get_candidate_coins":
		return a.toolGetCandidateCoins(storeUserID, userID, tc.Function.Arguments)
	default:
		return fmt.Sprintf(`{"error": "unknown tool: %s"}`, tc.Function.Name)
	}
}

type safeExchangeToolConfig struct {
	ID                    string `json:"id"`
	ExchangeType          string `json:"exchange_type"`
	AccountName           string `json:"account_name"`
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	Enabled               bool   `json:"enabled"`
	HasAPIKey             bool   `json:"has_api_key"`
	HasSecretKey          bool   `json:"has_secret_key"`
	HasPassphrase         bool   `json:"has_passphrase"`
	Testnet               bool   `json:"testnet"`
	HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr,omitempty"`
	HasAsterPrivateKey    bool   `json:"has_aster_private_key"`
	AsterUser             string `json:"aster_user,omitempty"`
	AsterSigner           string `json:"aster_signer,omitempty"`
	LighterWalletAddr     string `json:"lighter_wallet_addr,omitempty"`
	HasLighterPrivateKey  bool   `json:"has_lighter_private_key"`
	HasLighterAPIKey      bool   `json:"has_lighter_api_key_private_key"`
}

type safeModelToolConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	HasAPIKey       bool   `json:"has_api_key"`
	CustomAPIURL    string `json:"custom_api_url,omitempty"`
	CustomModelName string `json:"custom_model_name,omitempty"`
}

type safeTraderToolConfig struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	AIModelID            string  `json:"ai_model_id"`
	ExchangeID           string  `json:"exchange_id"`
	StrategyID           string  `json:"strategy_id,omitempty"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	IsRunning            bool    `json:"is_running"`
	IsCrossMargin        bool    `json:"is_cross_margin"`
	ShowInCompetition    bool    `json:"show_in_competition"`
	BTCETHLeverage       int     `json:"btc_eth_leverage,omitempty"`
	AltcoinLeverage      int     `json:"altcoin_leverage,omitempty"`
	TradingSymbols       string  `json:"trading_symbols,omitempty"`
	CustomPrompt         string  `json:"custom_prompt,omitempty"`
	SystemPromptTemplate string  `json:"system_prompt_template,omitempty"`
}

type safeStrategyToolConfig struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	IsActive      bool           `json:"is_active"`
	IsDefault     bool           `json:"is_default"`
	IsPublic      bool           `json:"is_public"`
	ConfigVisible bool           `json:"config_visible"`
	Config        map[string]any `json:"config,omitempty"`
	HasConfig     bool           `json:"has_config"`
}

type manageTraderArgs struct {
	Action               string   `json:"action"`
	TraderID             string   `json:"trader_id"`
	Name                 string   `json:"name"`
	AIModelID            string   `json:"ai_model_id"`
	ExchangeID           string   `json:"exchange_id"`
	StrategyID           string   `json:"strategy_id"`
	InitialBalance       *float64 `json:"initial_balance"`
	ScanIntervalMinutes  *int     `json:"scan_interval_minutes"`
	IsCrossMargin        *bool    `json:"is_cross_margin"`
	ShowInCompetition    *bool    `json:"show_in_competition"`
	BTCETHLeverage       *int     `json:"btc_eth_leverage"`
	AltcoinLeverage      *int     `json:"altcoin_leverage"`
	TradingSymbols       string   `json:"trading_symbols"`
	CustomPrompt         string   `json:"custom_prompt"`
	OverrideBasePrompt   *bool    `json:"override_base_prompt"`
	SystemPromptTemplate string   `json:"system_prompt_template"`
	UseAI500             *bool    `json:"use_ai500"`
	UseOITop             *bool    `json:"use_oi_top"`
}

func safeExchangeForTool(ex *store.Exchange) safeExchangeToolConfig {
	return safeExchangeToolConfig{
		ID:                    ex.ID,
		ExchangeType:          ex.ExchangeType,
		AccountName:           ex.AccountName,
		Name:                  ex.Name,
		Type:                  ex.Type,
		Enabled:               ex.Enabled,
		HasAPIKey:             ex.APIKey != "",
		HasSecretKey:          ex.SecretKey != "",
		HasPassphrase:         ex.Passphrase != "",
		Testnet:               ex.Testnet,
		HyperliquidWalletAddr: ex.HyperliquidWalletAddr,
		HasAsterPrivateKey:    ex.AsterPrivateKey != "",
		AsterUser:             ex.AsterUser,
		AsterSigner:           ex.AsterSigner,
		LighterWalletAddr:     ex.LighterWalletAddr,
		HasLighterPrivateKey:  ex.LighterPrivateKey != "",
		HasLighterAPIKey:      ex.LighterAPIKeyPrivateKey != "",
	}
}

func safeModelForTool(model *store.AIModel) safeModelToolConfig {
	return safeModelToolConfig{
		ID:              model.ID,
		Name:            model.Name,
		Provider:        model.Provider,
		Enabled:         model.Enabled,
		HasAPIKey:       model.APIKey != "",
		CustomAPIURL:    model.CustomAPIURL,
		CustomModelName: model.CustomModelName,
	}
}

func modelConfigUsable(provider, modelID, apiKey, customAPIURL, customModelName string) bool {
	if strings.TrimSpace(apiKey) == "" {
		return false
	}
	resolvedURL, resolvedModel := resolveModelRuntimeConfig(provider, customAPIURL, customModelName, modelID)
	return strings.TrimSpace(resolvedURL) != "" && strings.TrimSpace(resolvedModel) != ""
}

func safeTraderForTool(trader *store.Trader, isRunning bool) safeTraderToolConfig {
	return safeTraderToolConfig{
		ID:                   trader.ID,
		Name:                 trader.Name,
		AIModelID:            trader.AIModelID,
		ExchangeID:           trader.ExchangeID,
		StrategyID:           trader.StrategyID,
		InitialBalance:       trader.InitialBalance,
		ScanIntervalMinutes:  trader.ScanIntervalMinutes,
		IsRunning:            isRunning,
		IsCrossMargin:        trader.IsCrossMargin,
		ShowInCompetition:    trader.ShowInCompetition,
		BTCETHLeverage:       trader.BTCETHLeverage,
		AltcoinLeverage:      trader.AltcoinLeverage,
		TradingSymbols:       trader.TradingSymbols,
		CustomPrompt:         trader.CustomPrompt,
		SystemPromptTemplate: trader.SystemPromptTemplate,
	}
}

func safeStrategyForTool(strategy *store.Strategy) safeStrategyToolConfig {
	out := safeStrategyToolConfig{
		ID:            strategy.ID,
		Name:          strategy.Name,
		Description:   strategy.Description,
		IsActive:      strategy.IsActive,
		IsDefault:     strategy.IsDefault,
		IsPublic:      strategy.IsPublic,
		ConfigVisible: strategy.ConfigVisible,
		HasConfig:     strings.TrimSpace(strategy.Config) != "",
	}
	if out.HasConfig {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(strategy.Config), &cfg); err == nil {
			out.Config = cfg
		}
	}
	return out
}

func (a *Agent) toolGetExchangeConfigs(storeUserID string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	exchanges, err := a.store.Exchange().List(storeUserID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to load exchange configs: %s"}`, err)
	}
	safe := make([]safeExchangeToolConfig, 0, len(exchanges))
	for _, ex := range exchanges {
		safe = append(safe, safeExchangeForTool(ex))
	}
	result, _ := json.Marshal(map[string]any{
		"exchange_configs": safe,
		"count":            len(safe),
	})
	return string(result)
}

func latestBackendLogFilePath() string {
	matches, err := filepath.Glob(filepath.Join("data", "nofx_*.log"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}

func isBackendErrorLikeLogLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "[erro]") ||
		strings.Contains(lower, " panic") ||
		strings.Contains(lower, "🔥") ||
		strings.Contains(lower, "❌") ||
		strings.Contains(lower, " failed") ||
		strings.Contains(lower, " error") ||
		strings.Contains(lower, "invalid ")
}

func readBackendLogEntries(limit int, contains string, errorsOnly bool) (string, []string, error) {
	path := latestBackendLogFilePath()
	if path == "" {
		return "", nil, fmt.Errorf("backend log file not found")
	}
	file, err := os.Open(path)
	if err != nil {
		return path, nil, err
	}
	defer file.Close()

	filter := strings.ToLower(strings.TrimSpace(contains))
	matches := make([]string, 0, max(limit, 1))
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if errorsOnly && !isBackendErrorLikeLogLine(line) {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(line), filter) {
			continue
		}
		matches = append(matches, line)
	}
	if err := scanner.Err(); err != nil {
		return path, nil, err
	}
	if limit <= 0 {
		limit = 30
	}
	if len(matches) > limit {
		matches = matches[len(matches)-limit:]
	}
	return path, matches, nil
}

func (a *Agent) toolGetBackendLogs(storeUserID, argsJSON string) string {
	var args struct {
		TraderID   string `json:"trader_id"`
		Limit      int    `json:"limit"`
		ErrorsOnly *bool  `json:"errors_only"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
		}
	}
	errorsOnly := true
	if args.ErrorsOnly != nil {
		errorsOnly = *args.ErrorsOnly
	}
	traderID := strings.TrimSpace(args.TraderID)
	if traderID == "" {
		return `{"error":"trader_id is required"}`
	}
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	trader, err := a.store.Trader().GetByID(traderID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to load trader: %s"}`, err)
	}
	if trader.UserID != storeUserID {
		return `{"error":"trader not found for current user"}`
	}
	path, entries, err := readBackendLogEntries(args.Limit, traderID, errorsOnly)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to read backend logs: %s"}`, err)
	}
	result, _ := json.Marshal(map[string]any{
		"trader_id":   traderID,
		"log_file":    path,
		"entries":     entries,
		"count":       len(entries),
		"errors_only": errorsOnly,
	})
	return string(result)
}

func (a *Agent) toolManageExchangeConfig(storeUserID, argsJSON string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	var args struct {
		Action                    string `json:"action"`
		ExchangeID                string `json:"exchange_id"`
		ExchangeType              string `json:"exchange_type"`
		AccountName               string `json:"account_name"`
		Enabled                   *bool  `json:"enabled"`
		APIKey                    string `json:"api_key"`
		SecretKey                 string `json:"secret_key"`
		Passphrase                string `json:"passphrase"`
		Testnet                   *bool  `json:"testnet"`
		HyperliquidWalletAddr     string `json:"hyperliquid_wallet_addr"`
		HyperliquidUnifiedAccount *bool  `json:"hyperliquid_unified_account"`
		AsterUser                 string `json:"aster_user"`
		AsterSigner               string `json:"aster_signer"`
		AsterPrivateKey           string `json:"aster_private_key"`
		LighterWalletAddr         string `json:"lighter_wallet_addr"`
		LighterPrivateKey         string `json:"lighter_private_key"`
		LighterAPIKeyPrivateKey   string `json:"lighter_api_key_private_key"`
		LighterAPIKeyIndex        *int   `json:"lighter_api_key_index"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
	}
	action := strings.TrimSpace(args.Action)
	switch action {
	case "create":
		if strings.TrimSpace(args.ExchangeType) == "" {
			return `{"error":"exchange_type is required for create"}`
		}
		enabled := false
		if args.Enabled != nil {
			enabled = *args.Enabled
		}
		testnet := false
		if args.Testnet != nil {
			testnet = *args.Testnet
		}
		unified := true
		if args.HyperliquidUnifiedAccount != nil {
			unified = *args.HyperliquidUnifiedAccount
		}
		lighterIndex := 0
		if args.LighterAPIKeyIndex != nil {
			lighterIndex = *args.LighterAPIKeyIndex
		}
		id, err := a.store.Exchange().Create(
			storeUserID,
			strings.TrimSpace(args.ExchangeType),
			strings.TrimSpace(args.AccountName),
			enabled,
			strings.TrimSpace(args.APIKey),
			strings.TrimSpace(args.SecretKey),
			strings.TrimSpace(args.Passphrase),
			testnet,
			strings.TrimSpace(args.HyperliquidWalletAddr),
			unified,
			strings.TrimSpace(args.AsterUser),
			strings.TrimSpace(args.AsterSigner),
			strings.TrimSpace(args.AsterPrivateKey),
			strings.TrimSpace(args.LighterWalletAddr),
			strings.TrimSpace(args.LighterPrivateKey),
			strings.TrimSpace(args.LighterAPIKeyPrivateKey),
			lighterIndex,
		)
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to create exchange config: %s"}`, err)
		}
		created, err := a.store.Exchange().GetByID(storeUserID, id)
		if err != nil {
			return fmt.Sprintf(`{"error":"exchange created but failed to reload: %s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "create",
			"exchange": safeExchangeForTool(created),
		})
		return string(result)
	case "update":
		if strings.TrimSpace(args.ExchangeID) == "" {
			return `{"error":"exchange_id is required for update"}`
		}
		existing, err := a.store.Exchange().GetByID(storeUserID, strings.TrimSpace(args.ExchangeID))
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to load exchange config: %s"}`, err)
		}
		enabled := existing.Enabled
		if args.Enabled != nil {
			enabled = *args.Enabled
		}
		testnet := existing.Testnet
		if args.Testnet != nil {
			testnet = *args.Testnet
		}
		unified := existing.HyperliquidUnifiedAcct
		if args.HyperliquidUnifiedAccount != nil {
			unified = *args.HyperliquidUnifiedAccount
		}
		lighterIndex := existing.LighterAPIKeyIndex
		if args.LighterAPIKeyIndex != nil {
			lighterIndex = *args.LighterAPIKeyIndex
		}
		hyperWallet := existing.HyperliquidWalletAddr
		if strings.TrimSpace(args.HyperliquidWalletAddr) != "" {
			hyperWallet = strings.TrimSpace(args.HyperliquidWalletAddr)
		}
		asterUser := existing.AsterUser
		if strings.TrimSpace(args.AsterUser) != "" {
			asterUser = strings.TrimSpace(args.AsterUser)
		}
		asterSigner := existing.AsterSigner
		if strings.TrimSpace(args.AsterSigner) != "" {
			asterSigner = strings.TrimSpace(args.AsterSigner)
		}
		lighterWallet := existing.LighterWalletAddr
		if strings.TrimSpace(args.LighterWalletAddr) != "" {
			lighterWallet = strings.TrimSpace(args.LighterWalletAddr)
		}
		if err := a.store.Exchange().Update(
			storeUserID,
			existing.ID,
			enabled,
			strings.TrimSpace(args.APIKey),
			strings.TrimSpace(args.SecretKey),
			strings.TrimSpace(args.Passphrase),
			testnet,
			hyperWallet,
			unified,
			asterUser,
			asterSigner,
			strings.TrimSpace(args.AsterPrivateKey),
			lighterWallet,
			strings.TrimSpace(args.LighterPrivateKey),
			strings.TrimSpace(args.LighterAPIKeyPrivateKey),
			lighterIndex,
		); err != nil {
			return fmt.Sprintf(`{"error":"failed to update exchange config: %s"}`, err)
		}
		if trimmed := strings.TrimSpace(args.AccountName); trimmed != "" && trimmed != existing.AccountName {
			if err := a.store.Exchange().UpdateAccountName(storeUserID, existing.ID, trimmed); err != nil {
				return fmt.Sprintf(`{"error":"exchange updated but failed to rename account: %s"}`, err)
			}
		}
		updated, err := a.store.Exchange().GetByID(storeUserID, existing.ID)
		if err != nil {
			return fmt.Sprintf(`{"error":"exchange updated but failed to reload: %s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "update",
			"exchange": safeExchangeForTool(updated),
		})
		return string(result)
	case "delete":
		if strings.TrimSpace(args.ExchangeID) == "" {
			return `{"error":"exchange_id is required for delete"}`
		}
		if err := a.store.Exchange().Delete(storeUserID, strings.TrimSpace(args.ExchangeID)); err != nil {
			return fmt.Sprintf(`{"error":"failed to delete exchange config: %s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":      "ok",
			"action":      "delete",
			"exchange_id": strings.TrimSpace(args.ExchangeID),
		})
		return string(result)
	default:
		return `{"error":"invalid action"}`
	}
}

func (a *Agent) toolGetModelConfigs(storeUserID string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	models, err := a.store.AIModel().List(storeUserID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to load model configs: %s"}`, err)
	}
	safe := make([]safeModelToolConfig, 0, len(models))
	for _, model := range models {
		safe = append(safe, safeModelForTool(model))
	}
	result, _ := json.Marshal(map[string]any{
		"model_configs": safe,
		"count":         len(safe),
	})
	return string(result)
}

func (a *Agent) toolManageModelConfig(storeUserID, argsJSON string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	var args struct {
		Action          string `json:"action"`
		ModelID         string `json:"model_id"`
		Provider        string `json:"provider"`
		Name            string `json:"name"`
		Enabled         *bool  `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
	}
	if trimmed := strings.TrimSpace(args.CustomAPIURL); trimmed != "" {
		if err := security.ValidateURL(strings.TrimSuffix(trimmed, "#")); err != nil {
			return fmt.Sprintf(`{"error":"invalid custom_api_url: %s"}`, err)
		}
	}
	action := strings.TrimSpace(args.Action)
	switch action {
	case "create":
		provider := strings.TrimSpace(args.Provider)
		if provider == "" {
			return `{"error":"provider is required for create"}`
		}
		modelID := strings.TrimSpace(args.ModelID)
		if modelID == "" {
			modelID = provider
		}
		enabled := false
		if args.Enabled != nil {
			enabled = *args.Enabled
		}
		if err := a.store.AIModel().Update(storeUserID, modelID, enabled, strings.TrimSpace(args.APIKey), strings.TrimSpace(args.CustomAPIURL), strings.TrimSpace(args.CustomModelName)); err != nil {
			return fmt.Sprintf(`{"error":"failed to create model config: %s"}`, err)
		}
		createdID := modelID
		if modelID == provider {
			createdID = fmt.Sprintf("%s_%s", storeUserID, provider)
		}
		model, err := a.store.AIModel().Get(storeUserID, createdID)
		if err != nil {
			model, err = a.store.AIModel().Get(storeUserID, modelID)
		}
		if err != nil {
			return fmt.Sprintf(`{"error":"model created but failed to reload: %s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status": "ok",
			"action": "create",
			"model":  safeModelForTool(model),
		})
		return string(result)
	case "update":
		modelID := strings.TrimSpace(args.ModelID)
		if modelID == "" {
			return `{"error":"model_id is required for update"}`
		}
		existing, err := a.store.AIModel().Get(storeUserID, modelID)
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to load model config: %s"}`, err)
		}
		enabled := existing.Enabled
		if args.Enabled != nil {
			enabled = *args.Enabled
		}
		customAPIURL := existing.CustomAPIURL
		if strings.TrimSpace(args.CustomAPIURL) != "" {
			customAPIURL = strings.TrimSpace(args.CustomAPIURL)
		}
		customModelName := existing.CustomModelName
		if strings.TrimSpace(args.CustomModelName) != "" {
			customModelName = strings.TrimSpace(args.CustomModelName)
		}
		apiKey := strings.TrimSpace(args.APIKey)
		effectiveAPIKey := string(existing.APIKey)
		if apiKey != "" {
			effectiveAPIKey = apiKey
		}
		if enabled && !modelConfigUsable(existing.Provider, existing.ID, effectiveAPIKey, customAPIURL, customModelName) {
			return `{"error":"cannot enable model config before API key is configured"}`
		}
		if err := a.store.AIModel().Update(storeUserID, existing.ID, enabled, apiKey, customAPIURL, customModelName); err != nil {
			return fmt.Sprintf(`{"error":"failed to update model config: %s"}`, err)
		}
		updated, err := a.store.AIModel().Get(storeUserID, existing.ID)
		if err != nil {
			return fmt.Sprintf(`{"error":"model updated but failed to reload: %s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status": "ok",
			"action": "update",
			"model":  safeModelForTool(updated),
		})
		return string(result)
	case "delete":
		modelID := strings.TrimSpace(args.ModelID)
		if modelID == "" {
			return `{"error":"model_id is required for delete"}`
		}
		if err := a.store.AIModel().Delete(storeUserID, modelID); err != nil {
			return fmt.Sprintf(`{"error":"failed to delete model config: %s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "delete",
			"model_id": modelID,
		})
		return string(result)
	default:
		return `{"error":"invalid action"}`
	}
}

func (a *Agent) toolGetStrategies(storeUserID string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	strategies, err := a.store.Strategy().List(storeUserID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to load strategies: %s"}`, err)
	}
	safeStrategies := make([]safeStrategyToolConfig, 0, len(strategies))
	for _, strategy := range strategies {
		safeStrategies = append(safeStrategies, safeStrategyForTool(strategy))
	}
	result, _ := json.Marshal(map[string]any{
		"strategies": safeStrategies,
		"count":      len(safeStrategies),
	})
	return string(result)
}

func (a *Agent) toolManageStrategy(storeUserID, argsJSON string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	var args struct {
		Action        string         `json:"action"`
		StrategyID    string         `json:"strategy_id"`
		Name          string         `json:"name"`
		Description   string         `json:"description"`
		Lang          string         `json:"lang"`
		IsPublic      *bool          `json:"is_public"`
		ConfigVisible *bool          `json:"config_visible"`
		Config        map[string]any `json:"config"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
	}

	switch strings.TrimSpace(args.Action) {
	case "list":
		return a.toolGetStrategies(storeUserID)
	case "get_default_config":
		lang := strings.TrimSpace(args.Lang)
		if lang != "zh" {
			lang = "en"
		}
		cfg := store.GetDefaultStrategyConfig(lang)
		payload, _ := json.Marshal(map[string]any{
			"status": "ok",
			"action": "get_default_config",
			"config": cfg,
		})
		return string(payload)
	case "create":
		name := strings.TrimSpace(args.Name)
		if name == "" {
			return `{"error":"name is required for create"}`
		}
		var cfg any = store.GetDefaultStrategyConfig(strings.TrimSpace(args.Lang))
		if len(args.Config) > 0 {
			cfg = args.Config
		}
		configJSON, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to serialize strategy config: %s"}`, err)
		}
		record := &store.Strategy{
			ID:            fmt.Sprintf("strategy_%d", time.Now().UnixNano()),
			UserID:        storeUserID,
			Name:          name,
			Description:   strings.TrimSpace(args.Description),
			IsActive:      false,
			IsDefault:     false,
			IsPublic:      args.IsPublic != nil && *args.IsPublic,
			ConfigVisible: args.ConfigVisible == nil || *args.ConfigVisible,
			Config:        string(configJSON),
		}
		if err := a.store.Strategy().Create(record); err != nil {
			return fmt.Sprintf(`{"error":"failed to create strategy: %s"}`, err)
		}
		payload, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "create",
			"strategy": safeStrategyForTool(record),
		})
		return string(payload)
	case "update":
		strategyID := strings.TrimSpace(args.StrategyID)
		if strategyID == "" {
			return `{"error":"strategy_id is required for update"}`
		}
		existing, err := a.store.Strategy().Get(storeUserID, strategyID)
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to load strategy: %s"}`, err)
		}
		if existing.IsDefault {
			return `{"error":"cannot modify system default strategy"}`
		}
		name := existing.Name
		if trimmed := strings.TrimSpace(args.Name); trimmed != "" {
			name = trimmed
		}
		description := existing.Description
		if trimmed := strings.TrimSpace(args.Description); trimmed != "" {
			description = trimmed
		}
		isPublic := existing.IsPublic
		if args.IsPublic != nil {
			isPublic = *args.IsPublic
		}
		configVisible := existing.ConfigVisible
		if args.ConfigVisible != nil {
			configVisible = *args.ConfigVisible
		}
		configJSON := existing.Config
		if len(args.Config) > 0 {
			raw, err := json.Marshal(args.Config)
			if err != nil {
				return fmt.Sprintf(`{"error":"failed to serialize strategy config: %s"}`, err)
			}
			configJSON = string(raw)
		}
		record := &store.Strategy{
			ID:            existing.ID,
			UserID:        storeUserID,
			Name:          name,
			Description:   description,
			IsPublic:      isPublic,
			ConfigVisible: configVisible,
			Config:        configJSON,
		}
		if err := a.store.Strategy().Update(record); err != nil {
			return fmt.Sprintf(`{"error":"failed to update strategy: %s"}`, err)
		}
		updated, err := a.store.Strategy().Get(storeUserID, existing.ID)
		if err != nil {
			return fmt.Sprintf(`{"error":"strategy updated but failed to reload: %s"}`, err)
		}
		payload, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "update",
			"strategy": safeStrategyForTool(updated),
		})
		return string(payload)
	case "delete":
		strategyID := strings.TrimSpace(args.StrategyID)
		if strategyID == "" {
			return `{"error":"strategy_id is required for delete"}`
		}
		if err := a.store.Strategy().Delete(storeUserID, strategyID); err != nil {
			if strings.Contains(err.Error(), "cannot delete active strategy") {
				strategies, listErr := a.store.Strategy().List(storeUserID)
				if listErr != nil {
					return fmt.Sprintf(`{"error":"failed to prepare active strategy deletion: %s"}`, listErr)
				}

				var fallbackID string
				for _, strategy := range strategies {
					if strategy == nil || strategy.ID == strategyID {
						continue
					}
					if strategy.IsDefault {
						fallbackID = strategy.ID
						break
					}
					if fallbackID == "" {
						fallbackID = strategy.ID
					}
				}
				if fallbackID == "" {
					defaultConfig := store.GetDefaultStrategyConfig("zh")
					defaultConfig.ClampLimits()
					configJSON, marshalErr := json.Marshal(defaultConfig)
					if marshalErr != nil {
						return fmt.Sprintf(`{"error":"failed to create fallback strategy config: %s"}`, marshalErr)
					}

					fallbackID = fmt.Sprintf("strategy_%d", time.Now().UnixNano())
					fallbackStrategy := &store.Strategy{
						ID:          fallbackID,
						UserID:      storeUserID,
						Name:        "默认策略",
						Description: "Agent-generated fallback strategy",
						Config:      string(configJSON),
					}
					if createErr := a.store.Strategy().Create(fallbackStrategy); createErr != nil {
						return fmt.Sprintf(`{"error":"failed to create fallback strategy before deletion: %s"}`, createErr)
					}
				}
				if activateErr := a.store.Strategy().SetActive(storeUserID, fallbackID); activateErr != nil {
					return fmt.Sprintf(`{"error":"failed to switch active strategy before deletion: %s"}`, activateErr)
				}
				if retryErr := a.store.Strategy().Delete(storeUserID, strategyID); retryErr != nil {
					return fmt.Sprintf(`{"error":"failed to delete strategy: %s"}`, retryErr)
				}
			} else {
				return fmt.Sprintf(`{"error":"failed to delete strategy: %s"}`, err)
			}
		}
		payload, _ := json.Marshal(map[string]any{
			"status":      "ok",
			"action":      "delete",
			"strategy_id": strategyID,
		})
		return string(payload)
	case "activate":
		strategyID := strings.TrimSpace(args.StrategyID)
		if strategyID == "" {
			return `{"error":"strategy_id is required for activate"}`
		}
		if err := a.store.Strategy().SetActive(storeUserID, strategyID); err != nil {
			return fmt.Sprintf(`{"error":"failed to activate strategy: %s"}`, err)
		}
		updated, err := a.store.Strategy().Get(storeUserID, strategyID)
		if err != nil {
			return fmt.Sprintf(`{"error":"strategy activated but failed to reload: %s"}`, err)
		}
		payload, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "activate",
			"strategy": safeStrategyForTool(updated),
		})
		return string(payload)
	case "duplicate":
		sourceID := strings.TrimSpace(args.StrategyID)
		name := strings.TrimSpace(args.Name)
		if sourceID == "" {
			return `{"error":"strategy_id is required for duplicate"}`
		}
		if name == "" {
			return `{"error":"name is required for duplicate"}`
		}
		newID := fmt.Sprintf("strategy_%d", time.Now().UnixNano())
		if err := a.store.Strategy().Duplicate(storeUserID, sourceID, newID, name); err != nil {
			return fmt.Sprintf(`{"error":"failed to duplicate strategy: %s"}`, err)
		}
		created, err := a.store.Strategy().Get(storeUserID, newID)
		if err != nil {
			return fmt.Sprintf(`{"error":"strategy duplicated but failed to reload: %s"}`, err)
		}
		payload, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "duplicate",
			"strategy": safeStrategyForTool(created),
		})
		return string(payload)
	default:
		return `{"error":"invalid action"}`
	}
}

func (a *Agent) toolManageTrader(storeUserID, argsJSON string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	var args manageTraderArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
	}

	switch strings.TrimSpace(args.Action) {
	case "list":
		return a.toolListTraders(storeUserID)
	case "create":
		return a.toolCreateTrader(storeUserID, args)
	case "update":
		return a.toolUpdateTrader(storeUserID, args)
	case "delete":
		return a.toolDeleteTrader(storeUserID, strings.TrimSpace(args.TraderID))
	case "start":
		return a.toolStartTrader(storeUserID, strings.TrimSpace(args.TraderID))
	case "stop":
		return a.toolStopTrader(storeUserID, strings.TrimSpace(args.TraderID))
	default:
		return `{"error":"invalid action"}`
	}
}

func (a *Agent) toolListTraders(storeUserID string) string {
	traders, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to list traders: %s"}`, err)
	}
	safeTraders := make([]safeTraderToolConfig, 0, len(traders))
	for _, traderCfg := range traders {
		isRunning := traderCfg.IsRunning
		if a.traderManager != nil {
			if memTrader, err := a.traderManager.GetTrader(traderCfg.ID); err == nil {
				if running, ok := memTrader.GetStatus()["is_running"].(bool); ok {
					isRunning = running
				}
			}
		}
		safeTraders = append(safeTraders, safeTraderForTool(traderCfg, isRunning))
	}
	result, _ := json.Marshal(map[string]any{
		"traders": safeTraders,
		"count":   len(safeTraders),
	})
	return string(result)
}

func (a *Agent) validateTraderReferences(storeUserID, aiModelID, exchangeID, strategyID string) error {
	if strings.TrimSpace(aiModelID) == "" {
		return fmt.Errorf("ai_model_id is required")
	}
	if strings.TrimSpace(exchangeID) == "" {
		return fmt.Errorf("exchange_id is required")
	}
	model, err := a.store.AIModel().Get(storeUserID, strings.TrimSpace(aiModelID))
	if err != nil {
		return fmt.Errorf("invalid ai_model_id: %w", err)
	}
	if !model.Enabled {
		return fmt.Errorf("ai model is disabled")
	}
	exchange, err := a.store.Exchange().GetByID(storeUserID, strings.TrimSpace(exchangeID))
	if err != nil {
		return fmt.Errorf("invalid exchange_id: %w", err)
	}
	if !exchange.Enabled {
		return fmt.Errorf("exchange is disabled")
	}
	if trimmed := strings.TrimSpace(strategyID); trimmed != "" {
		if _, err := a.store.Strategy().Get(storeUserID, trimmed); err != nil {
			return fmt.Errorf("invalid strategy_id: %w", err)
		}
	}
	return nil
}

func (a *Agent) toolCreateTrader(storeUserID string, args manageTraderArgs) string {
	name := strings.TrimSpace(args.Name)
	if name == "" {
		return `{"error":"name is required for create"}`
	}
	if err := a.validateTraderReferences(storeUserID, args.AIModelID, args.ExchangeID, args.StrategyID); err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err)
	}
	scanInterval := 3
	if args.ScanIntervalMinutes != nil && *args.ScanIntervalMinutes > 0 {
		scanInterval = *args.ScanIntervalMinutes
		if scanInterval < 3 {
			scanInterval = 3
		}
	}
	initialBalance := 0.0
	if args.InitialBalance != nil && *args.InitialBalance > 0 {
		initialBalance = *args.InitialBalance
	}
	isCrossMargin := true
	if args.IsCrossMargin != nil {
		isCrossMargin = *args.IsCrossMargin
	}
	showInCompetition := true
	if args.ShowInCompetition != nil {
		showInCompetition = *args.ShowInCompetition
	}
	btcEthLeverage := 10
	if args.BTCETHLeverage != nil && *args.BTCETHLeverage > 0 {
		btcEthLeverage = *args.BTCETHLeverage
	}
	altcoinLeverage := 5
	if args.AltcoinLeverage != nil && *args.AltcoinLeverage > 0 {
		altcoinLeverage = *args.AltcoinLeverage
	}
	overrideBasePrompt := false
	if args.OverrideBasePrompt != nil {
		overrideBasePrompt = *args.OverrideBasePrompt
	}
	useAI500 := false
	if args.UseAI500 != nil {
		useAI500 = *args.UseAI500
	}
	useOITop := false
	if args.UseOITop != nil {
		useOITop = *args.UseOITop
	}
	systemPromptTemplate := strings.TrimSpace(args.SystemPromptTemplate)
	if systemPromptTemplate == "" {
		systemPromptTemplate = "default"
	}
	exchangeIDShort := strings.TrimSpace(args.ExchangeID)
	if len(exchangeIDShort) > 8 {
		exchangeIDShort = exchangeIDShort[:8]
	}
	traderID := fmt.Sprintf("%s_%s_%d", exchangeIDShort, strings.TrimSpace(args.AIModelID), time.Now().Unix())
	record := &store.Trader{
		ID:                   traderID,
		UserID:               storeUserID,
		Name:                 name,
		AIModelID:            strings.TrimSpace(args.AIModelID),
		ExchangeID:           strings.TrimSpace(args.ExchangeID),
		StrategyID:           strings.TrimSpace(args.StrategyID),
		InitialBalance:       initialBalance,
		ScanIntervalMinutes:  scanInterval,
		IsRunning:            false,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       strings.TrimSpace(args.TradingSymbols),
		UseAI500:             useAI500,
		UseOITop:             useOITop,
		CustomPrompt:         strings.TrimSpace(args.CustomPrompt),
		OverrideBasePrompt:   overrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
	}
	if err := a.store.Trader().Create(record); err != nil {
		return fmt.Sprintf(`{"error":"failed to create trader: %s"}`, err)
	}
	if a.traderManager != nil {
		_ = a.traderManager.LoadUserTradersFromStore(a.store, storeUserID)
	}
	result, _ := json.Marshal(map[string]any{
		"status": "ok",
		"action": "create",
		"trader": safeTraderForTool(record, false),
	})
	return string(result)
}

func (a *Agent) toolUpdateTrader(storeUserID string, args manageTraderArgs) string {
	traderID := strings.TrimSpace(args.TraderID)
	if traderID == "" {
		return `{"error":"trader_id is required for update"}`
	}
	traders, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to load traders: %s"}`, err)
	}
	var existing *store.Trader
	for _, item := range traders {
		if item.ID == traderID {
			existing = item
			break
		}
	}
	if existing == nil {
		return `{"error":"trader not found"}`
	}
	name := existing.Name
	if trimmed := strings.TrimSpace(args.Name); trimmed != "" {
		name = trimmed
	}
	aiModelID := existing.AIModelID
	if trimmed := strings.TrimSpace(args.AIModelID); trimmed != "" {
		aiModelID = trimmed
	}
	exchangeID := existing.ExchangeID
	if trimmed := strings.TrimSpace(args.ExchangeID); trimmed != "" {
		exchangeID = trimmed
	}
	strategyID := existing.StrategyID
	if trimmed := strings.TrimSpace(args.StrategyID); trimmed != "" {
		strategyID = trimmed
	}
	if err := a.validateTraderReferences(storeUserID, aiModelID, exchangeID, strategyID); err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err)
	}
	record := &store.Trader{
		ID:                   existing.ID,
		UserID:               storeUserID,
		Name:                 name,
		AIModelID:            aiModelID,
		ExchangeID:           exchangeID,
		StrategyID:           strategyID,
		InitialBalance:       existing.InitialBalance,
		ScanIntervalMinutes:  existing.ScanIntervalMinutes,
		IsRunning:            existing.IsRunning,
		IsCrossMargin:        existing.IsCrossMargin,
		ShowInCompetition:    existing.ShowInCompetition,
		BTCETHLeverage:       existing.BTCETHLeverage,
		AltcoinLeverage:      existing.AltcoinLeverage,
		TradingSymbols:       existing.TradingSymbols,
		UseAI500:             existing.UseAI500,
		UseOITop:             existing.UseOITop,
		CustomPrompt:         existing.CustomPrompt,
		OverrideBasePrompt:   existing.OverrideBasePrompt,
		SystemPromptTemplate: existing.SystemPromptTemplate,
	}
	if args.InitialBalance != nil && *args.InitialBalance > 0 {
		record.InitialBalance = *args.InitialBalance
	}
	if args.ScanIntervalMinutes != nil && *args.ScanIntervalMinutes > 0 {
		record.ScanIntervalMinutes = *args.ScanIntervalMinutes
		if record.ScanIntervalMinutes < 3 {
			record.ScanIntervalMinutes = 3
		}
	}
	if args.IsCrossMargin != nil {
		record.IsCrossMargin = *args.IsCrossMargin
	}
	if args.ShowInCompetition != nil {
		record.ShowInCompetition = *args.ShowInCompetition
	}
	if args.BTCETHLeverage != nil && *args.BTCETHLeverage > 0 {
		record.BTCETHLeverage = *args.BTCETHLeverage
	}
	if args.AltcoinLeverage != nil && *args.AltcoinLeverage > 0 {
		record.AltcoinLeverage = *args.AltcoinLeverage
	}
	if trimmed := strings.TrimSpace(args.TradingSymbols); trimmed != "" {
		record.TradingSymbols = trimmed
	}
	if trimmed := strings.TrimSpace(args.CustomPrompt); trimmed != "" {
		record.CustomPrompt = trimmed
	}
	if args.OverrideBasePrompt != nil {
		record.OverrideBasePrompt = *args.OverrideBasePrompt
	}
	if trimmed := strings.TrimSpace(args.SystemPromptTemplate); trimmed != "" {
		record.SystemPromptTemplate = trimmed
	}
	if args.UseAI500 != nil {
		record.UseAI500 = *args.UseAI500
	}
	if args.UseOITop != nil {
		record.UseOITop = *args.UseOITop
	}
	if err := a.store.Trader().Update(record); err != nil {
		return fmt.Sprintf(`{"error":"failed to update trader: %s"}`, err)
	}
	if a.traderManager != nil {
		a.traderManager.RemoveTrader(record.ID)
		_ = a.traderManager.LoadUserTradersFromStore(a.store, storeUserID)
	}
	result, _ := json.Marshal(map[string]any{
		"status": "ok",
		"action": "update",
		"trader": safeTraderForTool(record, record.IsRunning),
	})
	return string(result)
}

func (a *Agent) toolDeleteTrader(storeUserID, traderID string) string {
	if traderID == "" {
		return `{"error":"trader_id is required for delete"}`
	}
	if err := a.store.Trader().Delete(storeUserID, traderID); err != nil {
		return fmt.Sprintf(`{"error":"failed to delete trader: %s"}`, err)
	}
	if a.traderManager != nil {
		if trader, err := a.traderManager.GetTrader(traderID); err == nil {
			trader.Stop()
		}
		a.traderManager.RemoveTrader(traderID)
	}
	result, _ := json.Marshal(map[string]any{
		"status":    "ok",
		"action":    "delete",
		"trader_id": traderID,
	})
	return string(result)
}

func (a *Agent) toolStartTrader(storeUserID, traderID string) string {
	if traderID == "" {
		return `{"error":"trader_id is required for start"}`
	}
	if a.traderManager == nil {
		return `{"error":"trader manager unavailable"}`
	}
	if _, err := a.store.Trader().GetFullConfig(storeUserID, traderID); err != nil {
		return fmt.Sprintf(`{"error":"trader not found or inaccessible: %s"}`, err)
	}
	if existing, err := a.traderManager.GetTrader(traderID); err == nil {
		if running, ok := existing.GetStatus()["is_running"].(bool); ok && running {
			return `{"error":"trader is already running"}`
		}
		a.traderManager.RemoveTrader(traderID)
	}
	if err := a.traderManager.LoadUserTradersFromStore(a.store, storeUserID); err != nil {
		return fmt.Sprintf(`{"error":"failed to load trader config: %s"}`, err)
	}
	trader, err := a.traderManager.GetTrader(traderID)
	if err != nil {
		if loadErr := a.traderManager.GetLoadError(traderID); loadErr != nil {
			return fmt.Sprintf(`{"error":"failed to load trader: %s"}`, loadErr)
		}
		return fmt.Sprintf(`{"error":"failed to get trader: %s"}`, err)
	}
	safe.GoNamed("agent-trader-start-"+traderID, func() {
		if runErr := trader.Run(); runErr != nil {
			a.logger.Error("agent tool trader runtime error", "trader_id", traderID, "error", runErr)
		}
	})
	_ = a.store.Trader().UpdateStatus(storeUserID, traderID, true)
	result, _ := json.Marshal(map[string]any{
		"status":    "ok",
		"action":    "start",
		"trader_id": traderID,
		"message":   "Trader started",
	})
	return string(result)
}

func (a *Agent) toolStopTrader(storeUserID, traderID string) string {
	if traderID == "" {
		return `{"error":"trader_id is required for stop"}`
	}
	if a.traderManager == nil {
		return `{"error":"trader manager unavailable"}`
	}
	if _, err := a.store.Trader().GetFullConfig(storeUserID, traderID); err != nil {
		return fmt.Sprintf(`{"error":"trader not found or inaccessible: %s"}`, err)
	}
	trader, err := a.traderManager.GetTrader(traderID)
	if err != nil {
		return fmt.Sprintf(`{"error":"trader not loaded: %s"}`, err)
	}
	if running, ok := trader.GetStatus()["is_running"].(bool); ok && !running {
		return `{"error":"trader is already stopped"}`
	}
	trader.Stop()
	_ = a.store.Trader().UpdateStatus(storeUserID, traderID, false)
	result, _ := json.Marshal(map[string]any{
		"status":    "ok",
		"action":    "stop",
		"trader_id": traderID,
		"message":   "Trader stopped",
	})
	return string(result)
}

func (a *Agent) toolGetPreferences(userID int64) string {
	prefs := a.getPersistentPreferences(userID)
	result, _ := json.Marshal(map[string]any{
		"preferences": prefs,
		"count":       len(prefs),
	})
	return string(result)
}

func (a *Agent) toolManagePreferences(userID int64, argsJSON string) string {
	var args struct {
		Action string `json:"action"`
		Text   string `json:"text"`
		Match  string `json:"match"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	switch args.Action {
	case "add":
		prefs, created, err := a.addPersistentPreference(userID, args.Text)
		if err != nil {
			return fmt.Sprintf(`{"error": "%s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":      "ok",
			"action":      "add",
			"preference":  created,
			"preferences": prefs,
		})
		return string(result)
	case "update":
		prefs, updated, err := a.updatePersistentPreference(userID, args.Match, args.Text)
		if err != nil {
			return fmt.Sprintf(`{"error": "%s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":      "ok",
			"action":      "update",
			"preference":  updated,
			"preferences": prefs,
		})
		return string(result)
	case "delete":
		prefs, removed, err := a.deletePersistentPreference(userID, args.Match)
		if err != nil {
			return fmt.Sprintf(`{"error": "%s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":      "ok",
			"action":      "delete",
			"preference":  removed,
			"preferences": prefs,
		})
		return string(result)
	default:
		return `{"error": "invalid action"}`
	}
}

func (a *Agent) toolSearchStock(argsJSON string) string {
	var args struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	if args.Keyword == "" {
		return `{"error": "keyword is required"}`
	}

	results, err := searchStock(args.Keyword)
	if err != nil {
		return fmt.Sprintf(`{"error": "search failed: %s"}`, err)
	}

	if len(results) == 0 {
		return fmt.Sprintf(`{"results": [], "message": "no stocks found for '%s'"}`, args.Keyword)
	}

	// Limit to top 10 results
	if len(results) > 10 {
		results = results[:10]
	}

	// Also fetch real-time quotes for the top results (up to 3)
	type enrichedResult struct {
		Name   string      `json:"name"`
		Code   string      `json:"code"`
		Market string      `json:"market"`
		Quote  *StockQuote `json:"quote,omitempty"`
	}

	var enriched []enrichedResult
	for i, r := range results {
		er := enrichedResult{Name: r.Name, Code: r.Code, Market: r.Market}
		if i < 3 {
			q, qErr := fetchStockQuote(r.Code)
			if qErr == nil && q.Price > 0 {
				er.Quote = q
			}
		}
		enriched = append(enriched, er)
	}

	result, _ := json.Marshal(map[string]any{
		"keyword": args.Keyword,
		"count":   len(enriched),
		"results": enriched,
	})
	return string(result)
}

func (a *Agent) toolExecuteTrade(_ context.Context, userID int64, lang, argsJSON string) string {
	var args struct {
		Action   string  `json:"action"`
		Symbol   string  `json:"symbol"`
		Quantity float64 `json:"quantity"`
		Leverage int     `json:"leverage"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	// Normalize symbol
	sym := strings.ToUpper(args.Symbol)
	// Only append USDT for crypto symbols; stock tickers (e.g. AAPL, TSLA) stay as-is
	if !isStockSymbol(sym) && !strings.HasSuffix(sym, "USDT") {
		sym += "USDT"
	}

	// Validate action
	validActions := map[string]bool{
		"open_long": true, "open_short": true,
		"close_long": true, "close_short": true,
	}
	if !validActions[args.Action] {
		return fmt.Sprintf(`{"error": "invalid action: %s"}`, args.Action)
	}

	// For open actions, quantity must be > 0
	if (args.Action == "open_long" || args.Action == "open_short") && args.Quantity <= 0 {
		return `{"error": "quantity must be > 0 for opening positions"}`
	}

	// For stock symbols, check market hours and warn if closed
	var marketWarning string
	if isStockSymbol(sym) && a.traderManager != nil {
		for _, t := range a.traderManager.GetAllTraders() {
			if t.GetExchange() == "alpaca" {
				ut := t.GetUnderlyingTrader()
				if ut == nil {
					continue
				}
				type marketChecker interface {
					IsMarketOpen() (bool, string, error)
				}
				if mc, ok := ut.(marketChecker); ok {
					isOpen, status, err := mc.IsMarketOpen()
					if err == nil && !isOpen {
						marketWarning = fmt.Sprintf("⚠️ US market is currently %s. Order will be queued for next market open.", status)
					}
				}
				break
			}
		}
	}

	// Create pending trade — requires user confirmation
	trade := &TradeAction{
		ID:        fmt.Sprintf("trade_%d", time.Now().UnixNano()),
		Action:    args.Action,
		Symbol:    sym,
		Quantity:  args.Quantity,
		Leverage:  args.Leverage,
		Status:    "pending_confirmation",
		CreatedAt: time.Now().Unix(),
	}

	a.pending.Add(trade)
	a.pending.CleanExpired()

	// Return confirmation info to LLM so it can present it to the user
	resultMap := map[string]any{
		"status":   "pending_confirmation",
		"trade_id": trade.ID,
		"action":   trade.Action,
		"symbol":   trade.Symbol,
		"quantity": trade.Quantity,
		"leverage": trade.Leverage,
		"message":  fmt.Sprintf("Trade created. User must confirm with: 确认 %s (or: confirm %s)", trade.ID, trade.ID),
		"expires":  "5 minutes",
	}
	if marketWarning != "" {
		resultMap["market_warning"] = marketWarning
	}
	result, _ := json.Marshal(resultMap)
	return string(result)
}

func (a *Agent) toolGetPositions() string {
	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	var positions []map[string]any
	for id, t := range a.traderManager.GetAllTraders() {
		pos, err := t.GetPositions()
		if err != nil {
			continue
		}
		for _, p := range pos {
			size := toFloat(p["size"])
			if size == 0 {
				continue
			}
			tid := id
			if len(tid) > 8 {
				tid = tid[:8]
			}
			positions = append(positions, map[string]any{
				"trader":         tid,
				"exchange":       t.GetExchange(),
				"symbol":         p["symbol"],
				"side":           p["side"],
				"size":           size,
				"entry_price":    toFloat(p["entryPrice"]),
				"mark_price":     toFloat(p["markPrice"]),
				"unrealized_pnl": toFloat(p["unrealizedPnl"]),
				"leverage":       p["leverage"],
			})
		}
	}

	if len(positions) == 0 {
		return `{"positions": [], "message": "no open positions"}`
	}

	result, _ := json.Marshal(map[string]any{"positions": positions})
	return string(result)
}

func (a *Agent) toolGetBalance() string {
	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	var balances []map[string]any
	for id, t := range a.traderManager.GetAllTraders() {
		info, err := t.GetAccountInfo()
		if err != nil {
			continue
		}
		tid := id
		if len(tid) > 8 {
			tid = tid[:8]
		}
		balances = append(balances, map[string]any{
			"trader":       tid,
			"name":         t.GetName(),
			"exchange":     t.GetExchange(),
			"total_equity": toFloat(info["total_equity"]),
			"available":    toFloat(info["available_balance"]),
			"used_margin":  toFloat(info["used_margin"]),
		})
	}

	result, _ := json.Marshal(map[string]any{"balances": balances})
	return string(result)
}

func (a *Agent) toolGetMarketPrice(argsJSON string) string {
	var args struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	sym := strings.ToUpper(args.Symbol)
	if !isStockSymbol(sym) && !strings.HasSuffix(sym, "USDT") {
		sym += "USDT"
	}

	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	wantStock := isStockSymbol(sym)
	for _, t := range a.traderManager.GetAllTraders() {
		underlying := t.GetUnderlyingTrader()
		if underlying == nil {
			continue
		}
		// Route to correct exchange type (stock vs crypto)
		isAlpaca := t.GetExchange() == "alpaca"
		if wantStock && !isAlpaca {
			continue
		}
		if !wantStock && isAlpaca {
			continue
		}
		price, err := underlying.GetMarketPrice(sym)
		if err == nil && price > 0 {
			priceResult := map[string]any{
				"symbol": sym,
				"price":  price,
			}
			// For stocks, include market status
			if wantStock && isAlpaca {
				type marketChecker interface {
					IsMarketOpen() (bool, string, error)
				}
				if mc, ok := underlying.(marketChecker); ok {
					isOpen, status, mErr := mc.IsMarketOpen()
					if mErr == nil {
						priceResult["market_open"] = isOpen
						priceResult["market_status"] = status
					}
				}
			}
			result, _ := json.Marshal(priceResult)
			return string(result)
		}
	}

	return fmt.Sprintf(`{"error": "could not get price for %s"}`, sym)
}

func (a *Agent) toolGetTradeHistory(argsJSON string) string {
	if a.store == nil {
		return `{"error": "store not available"}`
	}

	var args struct {
		Limit int `json:"limit"`
	}
	if argsJSON != "" {
		_ = json.Unmarshal([]byte(argsJSON), &args)
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if args.Limit > 50 {
		args.Limit = 50
	}

	if a.traderManager == nil {
		return `{"error": "no trader manager configured"}`
	}

	var trades []map[string]any
	var totalPnL float64
	var wins, losses int

	for id, t := range a.traderManager.GetAllTraders() {
		positions, err := a.store.Position().GetClosedPositions(id, args.Limit)
		if err != nil {
			continue
		}
		tid := id
		if len(tid) > 8 {
			tid = tid[:8]
		}
		for _, pos := range positions {
			pnl := pos.RealizedPnL
			totalPnL += pnl
			if pnl >= 0 {
				wins++
			} else {
				losses++
			}

			entryTime := ""
			if pos.EntryTime > 0 {
				entryTime = time.Unix(pos.EntryTime/1000, 0).Format("2006-01-02 15:04")
			}
			exitTime := ""
			if pos.ExitTime > 0 {
				exitTime = time.Unix(pos.ExitTime/1000, 0).Format("2006-01-02 15:04")
			}

			trades = append(trades, map[string]any{
				"trader":      t.GetName(),
				"trader_id":   tid,
				"symbol":      pos.Symbol,
				"side":        pos.Side,
				"entry_price": pos.EntryPrice,
				"exit_price":  pos.ExitPrice,
				"quantity":    pos.Quantity,
				"leverage":    pos.Leverage,
				"pnl":         pnl,
				"entry_time":  entryTime,
				"exit_time":   exitTime,
			})
		}
	}

	if len(trades) == 0 {
		return `{"trades": [], "message": "no closed trades found"}`
	}

	// Sort trades by exit time (most recent first) for consistent ordering across traders
	sort.Slice(trades, func(i, j int) bool {
		ti, _ := trades[i]["exit_time"].(string)
		tj, _ := trades[j]["exit_time"].(string)
		return ti > tj // reverse chronological
	})

	// Only return up to the limit
	if len(trades) > args.Limit {
		trades = trades[:args.Limit]
	}

	winRate := 0.0
	total := wins + losses
	if total > 0 {
		winRate = float64(wins) / float64(total) * 100
	}

	result, _ := json.Marshal(map[string]any{
		"trades": trades,
		"summary": map[string]any{
			"total_trades": total,
			"wins":         wins,
			"losses":       losses,
			"win_rate":     fmt.Sprintf("%.1f%%", winRate),
			"total_pnl":    totalPnL,
		},
	})
	return string(result)
}

func (a *Agent) toolGetCandidateCoins(storeUserID string, userID int64, argsJSON string) string {
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}

	var args struct {
		TraderID   string `json:"trader_id"`
		StrategyID string `json:"strategy_id"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
		}
	}

	traderID := strings.TrimSpace(args.TraderID)
	strategyID := strings.TrimSpace(args.StrategyID)
	state := a.getExecutionState(userID)
	if traderID == "" && state.CurrentReferences != nil && state.CurrentReferences.Trader != nil {
		traderID = strings.TrimSpace(state.CurrentReferences.Trader.ID)
	}
	if strategyID == "" && state.CurrentReferences != nil && state.CurrentReferences.Strategy != nil {
		strategyID = strings.TrimSpace(state.CurrentReferences.Strategy.ID)
	}

	if traderID != "" {
		return a.toolGetCandidateCoinsForTrader(storeUserID, traderID)
	}
	if strategyID != "" {
		return a.toolGetCandidateCoinsForStrategy(storeUserID, strategyID)
	}
	return `{"error":"trader_id or strategy_id is required"}`
}

func (a *Agent) toolGetCandidateCoinsForTrader(storeUserID, traderID string) string {
	if a.traderManager == nil {
		return `{"error":"no trader manager configured"}`
	}
	record, err := a.store.Trader().GetFullConfig(storeUserID, traderID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to load trader: %s"}`, err)
	}
	memTrader, err := a.traderManager.GetTrader(traderID)
	if err != nil {
		return fmt.Sprintf(`{"error":"trader is not loaded in memory: %s"}`, err)
	}

	coins, coinErr := memTrader.GetCandidateCoins()
	cfg := memTrader.GetStrategyConfig()
	status := memTrader.GetStatus()
	isRunning, _ := status["is_running"].(bool)
	payload := map[string]any{
		"trader":            safeTraderForTool(record.Trader, isRunning),
		"coin_source":       candidateCoinSourceSummary(cfg),
		"candidate_count":   len(coins),
		"candidate_symbols": candidateCoinSymbols(coins),
		"candidates":        candidateCoinDetails(coins),
	}
	if coinErr != nil {
		payload["error"] = coinErr.Error()
	}
	result, _ := json.Marshal(payload)
	return string(result)
}

func (a *Agent) toolGetCandidateCoinsForStrategy(storeUserID, strategyID string) string {
	record, err := a.store.Strategy().Get(storeUserID, strategyID)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to load strategy: %s"}`, err)
	}
	cfg, err := record.ParseConfig()
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to parse strategy config: %s"}`, err)
	}

	engine := kernel.NewStrategyEngine(cfg)
	coins, coinErr := engine.GetCandidateCoins()
	payload := map[string]any{
		"strategy":          safeStrategyForTool(record),
		"coin_source":       candidateCoinSourceSummary(cfg),
		"candidate_count":   len(coins),
		"candidate_symbols": candidateCoinSymbols(coins),
		"candidates":        candidateCoinDetails(coins),
	}
	if coinErr != nil {
		payload["error"] = coinErr.Error()
	}
	result, _ := json.Marshal(payload)
	return string(result)
}

func candidateCoinSourceSummary(cfg *store.StrategyConfig) map[string]any {
	if cfg == nil {
		return nil
	}
	return map[string]any{
		"source_type":      cfg.CoinSource.SourceType,
		"use_ai500":        cfg.CoinSource.UseAI500,
		"ai500_limit":      cfg.CoinSource.AI500Limit,
		"use_oi_top":       cfg.CoinSource.UseOITop,
		"oi_top_limit":     cfg.CoinSource.OITopLimit,
		"use_oi_low":       cfg.CoinSource.UseOILow,
		"oi_low_limit":     cfg.CoinSource.OILowLimit,
		"use_hyper_all":    cfg.CoinSource.UseHyperAll,
		"use_hyper_main":   cfg.CoinSource.UseHyperMain,
		"hyper_main_limit": cfg.CoinSource.HyperMainLimit,
		"static_coins":     cfg.CoinSource.StaticCoins,
		"excluded_coins":   cfg.CoinSource.ExcludedCoins,
	}
}

func candidateCoinSymbols(coins []kernel.CandidateCoin) []string {
	out := make([]string, 0, len(coins))
	for _, coin := range coins {
		out = append(out, coin.Symbol)
	}
	return out
}

func candidateCoinDetails(coins []kernel.CandidateCoin) []map[string]any {
	out := make([]map[string]any, 0, len(coins))
	for _, coin := range coins {
		out = append(out, map[string]any{
			"symbol":  coin.Symbol,
			"sources": coin.Sources,
		})
	}
	return out
}

// knownCryptoSymbols is a set of well-known cryptocurrency base symbols.
// Without this, isStockSymbol("BTC") would incorrectly return true because
// "BTC" is 3 uppercase letters and the suffix check only catches "BTCUSDT"-style pairs.
var knownCryptoSymbols = map[string]bool{
	"BTC": true, "ETH": true, "SOL": true, "BNB": true, "XRP": true,
	"DOGE": true, "ADA": true, "AVAX": true, "DOT": true, "LINK": true,
	"PEPE": true, "SHIB": true, "ARB": true, "OP": true, "SUI": true,
	"APT": true, "SEI": true, "TIA": true, "JUP": true, "WIF": true,
	"NEAR": true, "ATOM": true, "FTM": true, "MATIC": true, "INJ": true,
	"RENDER": true, "FET": true, "TAO": true, "WLD": true, "USDT": true,
	"USDC": true, "BUSD": true, "DAI": true, "UNI": true, "AAVE": true,
	"LDO": true, "MKR": true, "CRV": true, "PENDLE": true, "ENA": true,
	"ONDO": true, "TRUMP": true, "TON": true, "TRX": true, "LTC": true,
	"BCH": true, "ETC": true, "FIL": true, "ICP": true, "HBAR": true,
	"VET": true, "ALGO": true, "SAND": true, "MANA": true, "AXS": true,
	"GMT": true, "APE": true, "GALA": true, "IMX": true, "BLUR": true,
	"STRK": true, "ZK": true, "W": true, "IO": true, "ZRO": true,
	"BONK": true, "FLOKI": true, "ORDI": true, "STX": true, "RUNE": true,
}

// isStockSymbol heuristically determines if a symbol is a stock ticker (not crypto).
// Stock tickers are 1-5 uppercase letters without numeric suffixes like "USDT".
// Known crypto base symbols (BTC, ETH, SOL etc.) are excluded.
func isStockSymbol(sym string) bool {
	sym = strings.ToUpper(sym)

	// Check known crypto base symbols first (critical: "BTC", "ETH" etc. are NOT stocks)
	if knownCryptoSymbols[sym] {
		return false
	}

	// If it already has a crypto quote suffix, it's crypto
	cryptoSuffixes := []string{"USDT", "BUSD", "USDC", "BTC", "ETH", "BNB"}
	for _, suffix := range cryptoSuffixes {
		if strings.HasSuffix(sym, suffix) && len(sym) > len(suffix) {
			return false
		}
	}
	// Pure uppercase letters, 1-5 chars = likely a stock ticker
	if len(sym) >= 1 && len(sym) <= 5 {
		allLetters := true
		for _, c := range sym {
			if c < 'A' || c > 'Z' {
				allLetters = false
				break
			}
		}
		if allLetters {
			return true
		}
	}
	return false
}
