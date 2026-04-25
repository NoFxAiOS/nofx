package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func normalizedEntityName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sameEntityName(a, b string) bool {
	return normalizedEntityName(a) != "" && normalizedEntityName(a) == normalizedEntityName(b)
}

func (a *Agent) ensureUniqueModelName(storeUserID, name, excludeID string) error {
	models, err := a.store.AIModel().List(storeUserID)
	if err != nil {
		return err
	}
	for _, model := range models {
		if model == nil || strings.TrimSpace(model.ID) == strings.TrimSpace(excludeID) {
			continue
		}
		if sameEntityName(model.Name, name) {
			return fmt.Errorf("model name %q already exists", strings.TrimSpace(name))
		}
	}
	return nil
}

func (a *Agent) ensureUniqueExchangeAccountName(storeUserID, accountName, excludeID string) error {
	exchanges, err := a.store.Exchange().List(storeUserID)
	if err != nil {
		return err
	}
	for _, exchange := range exchanges {
		if exchange == nil || strings.TrimSpace(exchange.ID) == strings.TrimSpace(excludeID) {
			continue
		}
		if sameEntityName(exchange.AccountName, accountName) {
			return fmt.Errorf("exchange account name %q already exists", strings.TrimSpace(accountName))
		}
	}
	return nil
}

func (a *Agent) ensureUniqueStrategyName(storeUserID, name, excludeID string) error {
	strategies, err := a.store.Strategy().List(storeUserID)
	if err != nil {
		return err
	}
	for _, strategy := range strategies {
		if strategy == nil || strings.TrimSpace(strategy.ID) == strings.TrimSpace(excludeID) {
			continue
		}
		if sameEntityName(strategy.Name, name) {
			return fmt.Errorf("strategy name %q already exists", strings.TrimSpace(name))
		}
	}
	return nil
}

func (a *Agent) ensureUniqueTraderName(storeUserID, name, excludeID string) error {
	traders, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return err
	}
	for _, trader := range traders {
		if trader == nil || strings.TrimSpace(trader.ID) == strings.TrimSpace(excludeID) {
			continue
		}
		if sameEntityName(trader.Name, name) {
			return fmt.Errorf("trader name %q already exists", strings.TrimSpace(name))
		}
	}
	return nil
}

func stringArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string"},
	}
}

func intArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "number"},
	}
}

func strategyConfigSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Full or partial strategy config. Only include the fields you want to create or update.",
		"properties": map[string]any{
			"strategy_type": map[string]any{"type": "string", "enum": []string{"ai_trading", "grid_trading"}},
			"language":      map[string]any{"type": "string", "enum": []string{"zh", "en"}},
			"coin_source": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_type":      map[string]any{"type": "string", "enum": []string{"static", "ai500", "oi_top", "oi_low", "mixed"}},
					"static_coins":     stringArraySchema("Static coin symbols such as BTCUSDT or ETHUSDT."),
					"excluded_coins":   stringArraySchema("Coin symbols to exclude from all sources."),
					"use_ai500":        map[string]any{"type": "boolean"},
					"ai500_limit":      map[string]any{"type": "number"},
					"use_oi_top":       map[string]any{"type": "boolean"},
					"oi_top_limit":     map[string]any{"type": "number"},
					"use_oi_low":       map[string]any{"type": "boolean"},
					"oi_low_limit":     map[string]any{"type": "number"},
					"use_hyper_all":    map[string]any{"type": "boolean"},
					"use_hyper_main":   map[string]any{"type": "boolean"},
					"hyper_main_limit": map[string]any{"type": "number"},
				},
			},
			"indicators": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"klines": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"primary_timeframe":      map[string]any{"type": "string"},
							"primary_count":          map[string]any{"type": "number"},
							"longer_timeframe":       map[string]any{"type": "string"},
							"longer_count":           map[string]any{"type": "number"},
							"enable_multi_timeframe": map[string]any{"type": "boolean"},
							"selected_timeframes":    stringArraySchema("Selected analysis timeframes, e.g. 5m,15m,1h."),
						},
					},
					"enable_raw_klines":        map[string]any{"type": "boolean"},
					"enable_ema":               map[string]any{"type": "boolean"},
					"enable_macd":              map[string]any{"type": "boolean"},
					"enable_rsi":               map[string]any{"type": "boolean"},
					"enable_atr":               map[string]any{"type": "boolean"},
					"enable_boll":              map[string]any{"type": "boolean"},
					"enable_volume":            map[string]any{"type": "boolean"},
					"enable_oi":                map[string]any{"type": "boolean"},
					"enable_funding_rate":      map[string]any{"type": "boolean"},
					"ema_periods":              intArraySchema("EMA periods such as [20,50]."),
					"rsi_periods":              intArraySchema("RSI periods such as [7,14]."),
					"atr_periods":              intArraySchema("ATR periods such as [14]."),
					"boll_periods":             intArraySchema("BOLL periods such as [20]."),
					"nofxos_api_key":           map[string]any{"type": "string"},
					"enable_quant_data":        map[string]any{"type": "boolean"},
					"enable_quant_oi":          map[string]any{"type": "boolean"},
					"enable_quant_netflow":     map[string]any{"type": "boolean"},
					"enable_oi_ranking":        map[string]any{"type": "boolean"},
					"oi_ranking_duration":      map[string]any{"type": "string"},
					"oi_ranking_limit":         map[string]any{"type": "number"},
					"enable_netflow_ranking":   map[string]any{"type": "boolean"},
					"netflow_ranking_duration": map[string]any{"type": "string"},
					"netflow_ranking_limit":    map[string]any{"type": "number"},
					"enable_price_ranking":     map[string]any{"type": "boolean"},
					"price_ranking_duration":   map[string]any{"type": "string"},
					"price_ranking_limit":      map[string]any{"type": "number"},
				},
			},
			"custom_prompt": map[string]any{"type": "string"},
			"risk_control": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"max_positions":                    map[string]any{"type": "number"},
					"btc_eth_max_leverage":             map[string]any{"type": "number"},
					"altcoin_max_leverage":             map[string]any{"type": "number"},
					"btc_eth_max_position_value_ratio": map[string]any{"type": "number"},
					"altcoin_max_position_value_ratio": map[string]any{"type": "number"},
					"max_margin_usage":                 map[string]any{"type": "number"},
					"min_position_size":                map[string]any{"type": "number"},
					"min_risk_reward_ratio":            map[string]any{"type": "number"},
					"min_confidence":                   map[string]any{"type": "number"},
				},
			},
			"prompt_sections": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"role_definition":   map[string]any{"type": "string"},
					"trading_frequency": map[string]any{"type": "string"},
					"entry_standards":   map[string]any{"type": "string"},
					"decision_process":  map[string]any{"type": "string"},
				},
			},
			"grid_config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"symbol":                  map[string]any{"type": "string"},
					"grid_count":              map[string]any{"type": "number"},
					"total_investment":        map[string]any{"type": "number"},
					"leverage":                map[string]any{"type": "number"},
					"upper_price":             map[string]any{"type": "number"},
					"lower_price":             map[string]any{"type": "number"},
					"use_atr_bounds":          map[string]any{"type": "boolean"},
					"atr_multiplier":          map[string]any{"type": "number"},
					"distribution":            map[string]any{"type": "string", "enum": []string{"uniform", "gaussian", "pyramid"}},
					"max_drawdown_pct":        map[string]any{"type": "number"},
					"stop_loss_pct":           map[string]any{"type": "number"},
					"daily_loss_limit_pct":    map[string]any{"type": "number"},
					"use_maker_only":          map[string]any{"type": "boolean"},
					"enable_direction_adjust": map[string]any{"type": "boolean"},
					"direction_bias_ratio":    map[string]any{"type": "number"},
				},
			},
		},
	}
}

func modelConfigFieldsSchema() map[string]any {
	return map[string]any{
		"model_id": map[string]any{
			"type":        "string",
			"description": "Existing model id for update/delete, or the desired id for create.",
		},
		"provider": map[string]any{
			"type":        "string",
			"description": "Provider slug such as openai, claude, gemini, deepseek, qwen, kimi, grok, minimax, claw402, blockrun-base, or blockrun-sol.",
		},
		"name": map[string]any{
			"type":        "string",
			"description": "Display name for the model binding.",
		},
		"enabled": map[string]any{
			"type":        "boolean",
			"description": "Whether this model binding is enabled.",
		},
		"api_key": map[string]any{
			"type":        "string",
			"description": "Provider credential. For standard providers this is an API key; for claw402/blockrun it is the wallet private key. Sensitive and never returned in full.",
		},
		"custom_api_url": map[string]any{
			"type":        "string",
			"description": "Custom API base URL or endpoint override. Optional for standard providers; not used by claw402/blockrun.",
		},
		"custom_model_name": map[string]any{
			"type":        "string",
			"description": "Actual upstream model name to send to the provider. Optional when the provider has a default model.",
		},
	}
}

func exchangeConfigFieldsSchema() map[string]any {
	return map[string]any{
		"exchange_id": map[string]any{
			"type":        "string",
			"description": "Existing exchange account id. Required for update and delete.",
		},
		"exchange_type": map[string]any{
			"type":        "string",
			"description": "Exchange type such as binance, bybit, okx, bitget, gate, kucoin, hyperliquid, aster, lighter, or indodax.",
		},
		"account_name": map[string]any{
			"type":        "string",
			"description": "User-visible account name like Main, Testnet, or Mom Account.",
		},
		"enabled": map[string]any{
			"type":        "boolean",
			"description": "Whether this exchange binding should be enabled.",
		},
		"api_key":                     map[string]any{"type": "string", "description": "API key for CEX-style exchanges."},
		"secret_key":                  map[string]any{"type": "string", "description": "Secret key for CEX-style exchanges."},
		"passphrase":                  map[string]any{"type": "string", "description": "Optional passphrase, required by exchanges like OKX, Bitget, and KuCoin."},
		"testnet":                     map[string]any{"type": "boolean", "description": "Whether to use the exchange testnet/sandbox."},
		"hyperliquid_wallet_addr":     map[string]any{"type": "string", "description": "Hyperliquid wallet address."},
		"hyperliquid_unified_account": map[string]any{"type": "boolean", "description": "Whether Hyperliquid unified account mode is enabled."},
		"aster_user":                  map[string]any{"type": "string", "description": "Aster user address."},
		"aster_signer":                map[string]any{"type": "string", "description": "Aster signer address."},
		"aster_private_key":           map[string]any{"type": "string", "description": "Aster private key."},
		"lighter_wallet_addr":         map[string]any{"type": "string", "description": "LIGHTER wallet address."},
		"lighter_private_key":         map[string]any{"type": "string", "description": "LIGHTER private key."},
		"lighter_api_key_private_key": map[string]any{"type": "string", "description": "LIGHTER API key private key."},
		"lighter_api_key_index":       map[string]any{"type": "number", "description": "LIGHTER API key index."},
	}
}

func traderConfigFieldsSchema() map[string]any {
	return map[string]any{
		"trader_id": map[string]any{
			"type":        "string",
			"description": "Required for update, delete, start, and stop.",
		},
		"name":                   map[string]any{"type": "string", "description": "Trader display name."},
		"ai_model_id":            map[string]any{"type": "string", "description": "Bound AI model id."},
		"exchange_id":            map[string]any{"type": "string", "description": "Bound exchange id."},
		"strategy_id":            map[string]any{"type": "string", "description": "Bound strategy id."},
		"initial_balance":        map[string]any{"type": "number", "description": "Initial balance / bankroll."},
		"scan_interval_minutes":  map[string]any{"type": "number", "description": "Trading scan interval in minutes."},
		"is_cross_margin":        map[string]any{"type": "boolean", "description": "Whether cross margin is enabled."},
		"show_in_competition":    map[string]any{"type": "boolean", "description": "Whether to show this trader in competition views."},
		"btc_eth_leverage":       map[string]any{"type": "number", "description": "BTC/ETH leverage override."},
		"altcoin_leverage":       map[string]any{"type": "number", "description": "Altcoin leverage override."},
		"trading_symbols":        map[string]any{"type": "string", "description": "Comma-separated symbol list such as BTCUSDT,ETHUSDT."},
		"custom_prompt":          map[string]any{"type": "string", "description": "Additional trader custom prompt."},
		"override_base_prompt":   map[string]any{"type": "boolean", "description": "Whether to override the base system prompt."},
		"system_prompt_template": map[string]any{"type": "string", "description": "Prompt template preset such as default."},
		"use_ai500":              map[string]any{"type": "boolean", "description": "Whether to use AI500 candidate sourcing."},
		"use_oi_top":             map[string]any{"type": "boolean", "description": "Whether to use OI Top candidate sourcing."},
	}
}

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
				Description: "Get recent backend log lines for a trader diagnosis. Prefer this when the user asks why a specific trader failed, stopped, or behaved unexpectedly. Returns recent matching log lines for the authenticated user's trader. You can identify the trader by name or id — name is preferred when the user provides it.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"trader_id":   map[string]any{"type": "string", "description": "Trader id to diagnose."},
						"trader_name": map[string]any{"type": "string", "description": "Trader name to diagnose. Used to look up the trader when id is not known."},
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
				Description: "Create, update, or delete an exchange account binding. Use this when the user asks to add/edit/remove an exchange account, API key, secret, passphrase, wallet address, or account name. Prefer passing exact field values instead of vague summaries. Sensitive fields are stored securely and are never returned in full.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"create", "update", "delete"},
						},
						"exchange_id":                 exchangeConfigFieldsSchema()["exchange_id"],
						"exchange_type":               exchangeConfigFieldsSchema()["exchange_type"],
						"account_name":                exchangeConfigFieldsSchema()["account_name"],
						"enabled":                     exchangeConfigFieldsSchema()["enabled"],
						"api_key":                     exchangeConfigFieldsSchema()["api_key"],
						"secret_key":                  exchangeConfigFieldsSchema()["secret_key"],
						"passphrase":                  exchangeConfigFieldsSchema()["passphrase"],
						"testnet":                     exchangeConfigFieldsSchema()["testnet"],
						"hyperliquid_wallet_addr":     exchangeConfigFieldsSchema()["hyperliquid_wallet_addr"],
						"hyperliquid_unified_account": exchangeConfigFieldsSchema()["hyperliquid_unified_account"],
						"aster_user":                  exchangeConfigFieldsSchema()["aster_user"],
						"aster_signer":                exchangeConfigFieldsSchema()["aster_signer"],
						"aster_private_key":           exchangeConfigFieldsSchema()["aster_private_key"],
						"lighter_wallet_addr":         exchangeConfigFieldsSchema()["lighter_wallet_addr"],
						"lighter_private_key":         exchangeConfigFieldsSchema()["lighter_private_key"],
						"lighter_api_key_private_key": exchangeConfigFieldsSchema()["lighter_api_key_private_key"],
						"lighter_api_key_index":       exchangeConfigFieldsSchema()["lighter_api_key_index"],
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
				Description: "Create, update, or delete an AI model binding. Use this when the user asks to add/edit/remove a model provider, API key, custom API URL, or custom model name. Prefer passing exact field values instead of vague summaries. Sensitive fields are stored securely and are never returned in full.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"create", "update", "delete"},
						},
						"model_id":          modelConfigFieldsSchema()["model_id"],
						"provider":          modelConfigFieldsSchema()["provider"],
						"name":              modelConfigFieldsSchema()["name"],
						"enabled":           modelConfigFieldsSchema()["enabled"],
						"api_key":           modelConfigFieldsSchema()["api_key"],
						"custom_api_url":    modelConfigFieldsSchema()["custom_api_url"],
						"custom_model_name": modelConfigFieldsSchema()["custom_model_name"],
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
				Description: "List, create, update, delete, activate, duplicate strategies, or get the default strategy config template. Use this when the user asks to create or edit a strategy template. Prefer passing precise field-level config patches in `config` instead of vague natural-language summaries.",
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
						"config":         strategyConfigSchema(),
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "manage_trader",
				Description: "List, create, update, delete, start, or stop traders. Use this when the user asks to create a trader, rename one, switch its exchange/model/strategy, tune leverage, prompts, symbol scope, scan interval, or control its running state.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"list", "create", "update", "delete", "start", "stop"},
						},
						"trader_id":              traderConfigFieldsSchema()["trader_id"],
						"name":                   traderConfigFieldsSchema()["name"],
						"ai_model_id":            traderConfigFieldsSchema()["ai_model_id"],
						"exchange_id":            traderConfigFieldsSchema()["exchange_id"],
						"strategy_id":            traderConfigFieldsSchema()["strategy_id"],
						"initial_balance":        traderConfigFieldsSchema()["initial_balance"],
						"scan_interval_minutes":  traderConfigFieldsSchema()["scan_interval_minutes"],
						"is_cross_margin":        traderConfigFieldsSchema()["is_cross_margin"],
						"show_in_competition":    traderConfigFieldsSchema()["show_in_competition"],
						"btc_eth_leverage":       traderConfigFieldsSchema()["btc_eth_leverage"],
						"altcoin_leverage":       traderConfigFieldsSchema()["altcoin_leverage"],
						"trading_symbols":        traderConfigFieldsSchema()["trading_symbols"],
						"custom_prompt":          traderConfigFieldsSchema()["custom_prompt"],
						"override_base_prompt":   traderConfigFieldsSchema()["override_base_prompt"],
						"system_prompt_template": traderConfigFieldsSchema()["system_prompt_template"],
						"use_ai500":              traderConfigFieldsSchema()["use_ai500"],
						"use_oi_top":             traderConfigFieldsSchema()["use_oi_top"],
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
				Description: "Execute a trade order (crypto or US stocks). Use this only when the user explicitly asks to trade. For stocks (e.g. AAPL, TSLA), use open_long to buy and close_long to sell. This creates a pending trade first; it does not execute immediately. Large orders require an extra confirmation with 确认大额 trade_xxx / confirm large trade_xxx, and pending trades expire after 5 minutes.",
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
				Name:        "get_kline",
				Description: "Get recent kline/candlestick data for a crypto symbol. Use this when the user asks for recent candles, K 线, recent price structure, or a short-term chart context.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol": map[string]any{
							"type":        "string",
							"description": "Crypto trading symbol, for example BTC, ETH, BTCUSDT, or ETHUSDT.",
						},
						"interval": map[string]any{
							"type":        "string",
							"description": "Kline interval, for example 1m, 5m, 15m, 1h, 4h, or 1d. Defaults to 15m.",
						},
						"limit": map[string]any{
							"type":        "number",
							"description": "Number of recent candles to fetch. Defaults to 50 and is capped at 300.",
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
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "get_watchlist",
				Description: "Get the current Sentinel watchlist of monitored crypto symbols. Use this when the user asks which coins are being watched or monitored right now.",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: mcp.FunctionDef{
				Name:        "manage_watchlist",
				Description: "Add or remove a monitored crypto symbol from the Sentinel watchlist at runtime. Use this when the user asks to watch, monitor, unwatch, or stop monitoring a coin.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"add", "remove"},
							"description": "Whether to add or remove the symbol from the watchlist.",
						},
						"symbol": map[string]any{
							"type":        "string",
							"description": "Crypto symbol to watch, such as BTC, ETH, SOL, BTCUSDT, or ETHUSDT.",
						},
					},
					"required": []string{"action", "symbol"},
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
	case "get_kline":
		return a.toolGetKline(tc.Function.Arguments)
	case "get_trade_history":
		return a.toolGetTradeHistory(tc.Function.Arguments)
	case "get_candidate_coins":
		return a.toolGetCandidateCoins(storeUserID, userID, tc.Function.Arguments)
	case "get_watchlist":
		return a.toolGetWatchlist(lang)
	case "manage_watchlist":
		return a.toolManageWatchlist(lang, tc.Function.Arguments)
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
	LighterAPIKeyIndex    int    `json:"lighter_api_key_index,omitempty"`
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

var sensitiveToolKeys = map[string]struct{}{
	"api_key":                     {},
	"secret_key":                  {},
	"passphrase":                  {},
	"private_key":                 {},
	"password_hash":               {},
	"lighter_api_key_private_key": {},
}

func stripSensitiveToolFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, inner := range typed {
			if _, blocked := sensitiveToolKeys[strings.ToLower(strings.TrimSpace(key))]; blocked {
				continue
			}
			cleaned[key] = stripSensitiveToolFields(inner)
		}
		return cleaned
	case []any:
		out := make([]any, 0, len(typed))
		for _, inner := range typed {
			out = append(out, stripSensitiveToolFields(inner))
		}
		return out
	default:
		return value
	}
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
		LighterAPIKeyIndex:    ex.LighterAPIKeyIndex,
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
		if !store.IsVisibleExchange(ex) {
			continue
		}
		safe = append(safe, safeExchangeForTool(ex))
	}
	result, _ := json.Marshal(map[string]any{
		"exchange_configs": safe,
		"count":            len(safe),
	})
	var payload any
	if err := json.Unmarshal(result, &payload); err == nil {
		result, _ = json.Marshal(stripSensitiveToolFields(payload))
	}
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

func filterBackendLogEntriesAny(entries []string, needles ...string) []string {
	if len(entries) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(needles))
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle == "" {
			continue
		}
		normalized = append(normalized, needle)
	}
	if len(normalized) == 0 {
		return entries
	}
	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		lower := strings.ToLower(entry)
		for _, needle := range normalized {
			if strings.Contains(lower, needle) {
				filtered = append(filtered, entry)
				break
			}
		}
	}
	return filtered
}

func (a *Agent) toolGetBackendLogs(storeUserID, argsJSON string) string {
	var args struct {
		TraderID   string `json:"trader_id"`
		TraderName string `json:"trader_name"`
		Limit      int    `json:"limit"`
		ErrorsOnly *bool  `json:"errors_only"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
		}
	}
	if a.store == nil {
		return `{"error":"store unavailable"}`
	}
	errorsOnly := true
	if args.ErrorsOnly != nil {
		errorsOnly = *args.ErrorsOnly
	}
	traderID := strings.TrimSpace(args.TraderID)
	traderName := strings.TrimSpace(args.TraderName)
	if traderID == "" && traderName == "" {
		return `{"error":"trader_id or trader_name is required"}`
	}
	// resolve by name if id not provided
	if traderID == "" {
		traders, err := a.store.Trader().List(storeUserID)
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to list traders: %s"}`, err)
		}
		for _, t := range traders {
			if strings.EqualFold(strings.TrimSpace(t.Name), traderName) {
				traderID = t.ID
				traderName = t.Name
				break
			}
		}
		if traderID == "" {
			return fmt.Sprintf(`{"error":"trader %q not found"}`, traderName)
		}
	} else {
		trader, err := a.store.Trader().GetByID(traderID)
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to load trader: %s"}`, err)
		}
		if trader.UserID != storeUserID {
			return `{"error":"trader not found for current user"}`
		}
		traderName = trader.Name
	}
	path, entries, err := readBackendLogEntries(args.Limit, "", errorsOnly)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to read backend logs: %s"}`, err)
	}
	entries = filterBackendLogEntriesAny(entries, traderID, traderName)
	if args.Limit <= 0 {
		args.Limit = 30
	}
	if len(entries) > args.Limit {
		entries = entries[len(entries)-args.Limit:]
	}
	result, _ := json.Marshal(map[string]any{
		"trader_id":   traderID,
		"trader_name": traderName,
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
		missing := missingRequiredActionSlots("exchange_management", "create", map[string]string{
			"exchange_type": strings.TrimSpace(args.ExchangeType),
			"account_name":  strings.TrimSpace(args.AccountName),
			"api_key":       strings.TrimSpace(args.APIKey),
			"secret_key":    strings.TrimSpace(args.SecretKey),
		})
		if len(missing) > 0 {
			return fmt.Sprintf(`{"error":"missing required fields for create: %s"}`, strings.Join(missing, ", "))
		}
		exchangeType := strings.TrimSpace(args.ExchangeType)
		if exchangeType == "" {
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
		if err := (exchangeConfigValidator{
			exchangeType:            exchangeType,
			enabled:                 enabled,
			apiKey:                  strings.TrimSpace(args.APIKey),
			secretKey:               strings.TrimSpace(args.SecretKey),
			passphrase:              strings.TrimSpace(args.Passphrase),
			hyperliquidWalletAddr:   strings.TrimSpace(args.HyperliquidWalletAddr),
			asterUser:               strings.TrimSpace(args.AsterUser),
			asterSigner:             strings.TrimSpace(args.AsterSigner),
			asterPrivateKey:         strings.TrimSpace(args.AsterPrivateKey),
			lighterWalletAddr:       strings.TrimSpace(args.LighterWalletAddr),
			lighterPrivateKey:       strings.TrimSpace(args.LighterPrivateKey),
			lighterAPIKeyPrivateKey: strings.TrimSpace(args.LighterAPIKeyPrivateKey),
		}).Validate(); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
		}
		if err := a.ensureUniqueExchangeAccountName(storeUserID, strings.TrimSpace(args.AccountName), ""); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
		}
		id, err := a.store.Exchange().Create(
			storeUserID,
			exchangeType,
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
		var payload any
		if err := json.Unmarshal(result, &payload); err == nil {
			result, _ = json.Marshal(stripSensitiveToolFields(payload))
		}
		return string(result)
	case "query":
		if strings.TrimSpace(args.ExchangeID) == "" {
			return `{"error":"exchange_id is required for query"}`
		}
		existing, err := a.store.Exchange().GetByID(storeUserID, strings.TrimSpace(args.ExchangeID))
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to load exchange config: %s"}`, err)
		}
		result, _ := json.Marshal(map[string]any{
			"status":   "ok",
			"action":   "query",
			"exchange": safeExchangeForTool(existing),
		})
		var payload any
		if err := json.Unmarshal(result, &payload); err == nil {
			result, _ = json.Marshal(stripSensitiveToolFields(payload))
		}
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
		effectiveAPIKey := strings.TrimSpace(string(existing.APIKey))
		if trimmed := strings.TrimSpace(args.APIKey); trimmed != "" {
			effectiveAPIKey = trimmed
		}
		effectiveSecretKey := strings.TrimSpace(string(existing.SecretKey))
		if trimmed := strings.TrimSpace(args.SecretKey); trimmed != "" {
			effectiveSecretKey = trimmed
		}
		effectivePassphrase := strings.TrimSpace(string(existing.Passphrase))
		if trimmed := strings.TrimSpace(args.Passphrase); trimmed != "" {
			effectivePassphrase = trimmed
		}
		effectiveAsterPrivateKey := strings.TrimSpace(string(existing.AsterPrivateKey))
		if trimmed := strings.TrimSpace(args.AsterPrivateKey); trimmed != "" {
			effectiveAsterPrivateKey = trimmed
		}
		effectiveLighterPrivateKey := strings.TrimSpace(string(existing.LighterPrivateKey))
		if trimmed := strings.TrimSpace(args.LighterPrivateKey); trimmed != "" {
			effectiveLighterPrivateKey = trimmed
		}
		effectiveLighterAPIKeyPrivateKey := strings.TrimSpace(string(existing.LighterAPIKeyPrivateKey))
		if trimmed := strings.TrimSpace(args.LighterAPIKeyPrivateKey); trimmed != "" {
			effectiveLighterAPIKeyPrivateKey = trimmed
		}
		if err := (exchangeConfigValidator{
			exchangeType:            existing.ExchangeType,
			enabled:                 enabled,
			apiKey:                  effectiveAPIKey,
			secretKey:               effectiveSecretKey,
			passphrase:              effectivePassphrase,
			hyperliquidWalletAddr:   hyperWallet,
			asterUser:               asterUser,
			asterSigner:             asterSigner,
			asterPrivateKey:         effectiveAsterPrivateKey,
			lighterWalletAddr:       lighterWallet,
			lighterPrivateKey:       effectiveLighterPrivateKey,
			lighterAPIKeyPrivateKey: effectiveLighterAPIKeyPrivateKey,
		}).Validate(); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
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
			if err := a.ensureUniqueExchangeAccountName(storeUserID, trimmed, existing.ID); err != nil {
				return fmt.Sprintf(`{"error":"%s"}`, err)
			}
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
		var payload any
		if err := json.Unmarshal(result, &payload); err == nil {
			result, _ = json.Marshal(stripSensitiveToolFields(payload))
		}
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
		if !store.IsVisibleAIModel(model) {
			continue
		}
		safe = append(safe, safeModelForTool(model))
	}
	result, _ := json.Marshal(map[string]any{
		"model_configs": safe,
		"count":         len(safe),
	})
	var payload any
	if err := json.Unmarshal(result, &payload); err == nil {
		result, _ = json.Marshal(stripSensitiveToolFields(payload))
	}
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
		missing := missingRequiredActionSlots("model_management", "create", map[string]string{
			"provider": strings.TrimSpace(args.Provider),
		})
		if len(missing) > 0 {
			return fmt.Sprintf(`{"error":"missing required fields for create: %s"}`, strings.Join(missing, ", "))
		}
		provider := strings.TrimSpace(args.Provider)
		if provider == "" {
			return `{"error":"provider is required for create"}`
		}
		if strings.TrimSpace(args.APIKey) == "" {
			return `{"error":"api_key is required for create"}`
		}
		modelID := strings.TrimSpace(args.ModelID)
		if modelID == "" {
			modelID = provider
		}
		enabled := false
		if args.Enabled != nil {
			enabled = *args.Enabled
		}
		name := strings.TrimSpace(args.Name)
		if name == "" {
			name = defaultModelConfigName(provider)
		}
		customModelName := strings.TrimSpace(args.CustomModelName)
		if customModelName == "" && modelProviderSupportsCustomModel(provider) {
			customModelName = defaultModelNameForProvider(provider)
		}
		customAPIURL := strings.TrimSpace(args.CustomAPIURL)
		if !modelProviderSupportsCustomAPIURL(provider) {
			customAPIURL = ""
		}
		if err := (modelConfigValidator{
			provider:        provider,
			enabled:         enabled,
			apiKey:          strings.TrimSpace(args.APIKey),
			customAPIURL:    customAPIURL,
			customModelName: customModelName,
			modelID:         modelID,
		}).Validate(); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
		}
		if err := a.ensureUniqueModelName(storeUserID, name, ""); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
		}
		if err := a.store.AIModel().UpdateWithName(
			storeUserID,
			modelID,
			name,
			enabled,
			strings.TrimSpace(args.APIKey),
			customAPIURL,
			customModelName,
		); err != nil {
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
		var payload any
		if err := json.Unmarshal(result, &payload); err == nil {
			result, _ = json.Marshal(stripSensitiveToolFields(payload))
		}
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
		if err := (modelConfigValidator{
			provider:        existing.Provider,
			enabled:         enabled,
			apiKey:          effectiveAPIKey,
			customAPIURL:    customAPIURL,
			customModelName: customModelName,
			modelID:         existing.ID,
		}).Validate(); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
		}
		if trimmed := strings.TrimSpace(args.Name); trimmed != "" && !sameEntityName(trimmed, existing.Name) {
			if err := a.ensureUniqueModelName(storeUserID, trimmed, existing.ID); err != nil {
				return fmt.Sprintf(`{"error":"%s"}`, err)
			}
		}
		if err := a.store.AIModel().UpdateWithName(
			storeUserID,
			existing.ID,
			strings.TrimSpace(args.Name),
			enabled,
			apiKey,
			customAPIURL,
			customModelName,
		); err != nil {
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
		var payload any
		if err := json.Unmarshal(result, &payload); err == nil {
			result, _ = json.Marshal(stripSensitiveToolFields(payload))
		}
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
		if !store.IsVisibleStrategy(strategy) {
			continue
		}
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
		if err := a.ensureUniqueStrategyName(storeUserID, name, ""); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
		}
		defaultConfig := store.GetDefaultStrategyConfig(strings.TrimSpace(args.Lang))
		var cfg any = defaultConfig
		var warnings []string
		if len(args.Config) > 0 {
			merged, err := store.MergeStrategyConfig(defaultConfig, args.Config)
			if err != nil {
				return fmt.Sprintf(`{"error":"invalid strategy config: %s"}`, err)
			}
			before := merged
			merged.ClampLimits()
			warnings = store.StrategyClampWarnings(before, merged, merged.Language)
			cfg = merged
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
			"warnings": warnings,
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
		if !sameEntityName(name, existing.Name) {
			if err := a.ensureUniqueStrategyName(storeUserID, name, existing.ID); err != nil {
				return fmt.Sprintf(`{"error":"%s"}`, err)
			}
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
		var warnings []string
		if len(args.Config) > 0 {
			var existingConfig store.StrategyConfig
			if strings.TrimSpace(existing.Config) != "" {
				if err := json.Unmarshal([]byte(existing.Config), &existingConfig); err != nil {
					return fmt.Sprintf(`{"error":"failed to load existing strategy config: %s"}`, err)
				}
			}
			merged, err := store.MergeStrategyConfig(existingConfig, args.Config)
			if err != nil {
				return fmt.Sprintf(`{"error":"invalid strategy config: %s"}`, err)
			}
			before := merged
			merged.ClampLimits()
			warnings = store.StrategyClampWarnings(before, merged, merged.Language)
			normalized, err := json.Marshal(merged)
			if err != nil {
				return fmt.Sprintf(`{"error":"failed to serialize strategy config: %s"}`, err)
			}
			configJSON = string(normalized)
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
			"warnings": warnings,
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
		if err := a.ensureUniqueStrategyName(storeUserID, name, ""); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
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
	if len(traders) == 0 && a != nil && a.store != nil {
		if all, listErr := a.store.Trader().ListAll(); listErr == nil && len(all) > 0 {
			counts := make(map[string]int)
			for _, trader := range all {
				uid := strings.TrimSpace(trader.UserID)
				if uid == "" {
					uid = "default"
				}
				counts[uid]++
			}
			a.log().Warn("toolListTraders returned empty for current store user while traders exist under other user scopes",
				"store_user_id", storeUserID,
				"known_user_scopes", counts,
			)
		}
	}
	safeTraders := make([]safeTraderToolConfig, 0, len(traders))
	for _, traderCfg := range traders {
		if !store.IsVisibleTrader(traderCfg) {
			continue
		}
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
	return (traderBindingValidator{
		store:       a.store,
		storeUserID: storeUserID,
		aiModelID:   aiModelID,
		exchangeID:  exchangeID,
		strategyID:  strategyID,
	}).Validate()
}

func (a *Agent) toolCreateTrader(storeUserID string, args manageTraderArgs) string {
	name := strings.TrimSpace(args.Name)
	if name == "" {
		return `{"error":"name is required for create"}`
	}
	if err := a.ensureUniqueTraderName(storeUserID, name, ""); err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err)
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
	altcoinLeverage := 5
	overrideBasePrompt := false
	useAI500 := false
	useOITop := false
	systemPromptTemplate := "default"
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
		TradingSymbols:       "",
		UseAI500:             useAI500,
		UseOITop:             useOITop,
		CustomPrompt:         "",
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
	if !sameEntityName(name, existing.Name) {
		if err := a.ensureUniqueTraderName(storeUserID, name, existing.ID); err != nil {
			return fmt.Sprintf(`{"error":"%s"}`, err)
		}
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

func (a *Agent) toolExecuteTrade(ctx context.Context, userID int64, lang, argsJSON string) string {
	policy := sessionPolicyFromContext(ctx)
	if !policy.Authenticated {
		return `{"error": "trade execution requires an authenticated session"}`
	}
	if !policy.CanExecuteTrade || a == nil || a.config == nil || !a.config.AllowTradeExecution {
		return `{"error": "trade execution is blocked by server policy for this session"}`
	}

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
	if _, selectedTrader, underlyingTrader, err := a.resolveTradeExecutionContext(trade); err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	} else if err := validateTradeAction(trade, isStockSymbol(sym), selectedTrader, underlyingTrader); err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}

	a.pending.Add(trade)
	a.pending.CleanExpired()

	confirmMessage := fmt.Sprintf("Trade created. User must confirm with: 确认 %s (or: confirm %s)", trade.ID, trade.ID)
	if trade.RequiresLargeOrderConfirmation {
		confirmMessage = fmt.Sprintf("Trade created but flagged as high-risk. User must confirm with: 确认大额 %s (or: confirm large %s)", trade.ID, trade.ID)
	}

	// Return confirmation info to LLM so it can present it to the user
	resultMap := map[string]any{
		"status":                            "pending_confirmation",
		"trade_id":                          trade.ID,
		"action":                            trade.Action,
		"symbol":                            trade.Symbol,
		"quantity":                          trade.Quantity,
		"leverage":                          trade.Leverage,
		"estimated_price":                   trade.EstimatedPrice,
		"estimated_notional":                trade.EstimatedNotional,
		"requires_large_order_confirmation": trade.RequiresLargeOrderConfirmation,
		"message":                           confirmMessage,
		"expires":                           "5 minutes",
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

func validKlineInterval(interval string) bool {
	switch strings.TrimSpace(strings.ToLower(interval)) {
	case "1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w", "1mo":
		return true
	default:
		return false
	}
}

func (a *Agent) toolGetKline(argsJSON string) string {
	var args struct {
		Symbol   string `json:"symbol"`
		Interval string `json:"interval"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "invalid arguments: %s"}`, err)
	}

	symbol := strings.ToUpper(strings.TrimSpace(args.Symbol))
	if symbol == "" {
		return `{"error": "symbol is required"}`
	}
	if !strings.HasSuffix(symbol, "USDT") {
		symbol += "USDT"
	}

	interval := strings.TrimSpace(strings.ToLower(args.Interval))
	if interval == "" {
		interval = "15m"
	}
	if !validKlineInterval(interval) {
		return fmt.Sprintf(`{"error":"invalid interval %q"}`, interval)
	}

	limit := args.Limit
	switch {
	case limit <= 0:
		limit = 50
	case limit > 300:
		limit = 300
	}

	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=%d", symbol, interval, limit)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to create request: %s"}`, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to fetch kline for %s: %s"}`, symbol, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf(`{"error":"kline source returned status %d for %s"}`, resp.StatusCode, symbol)
	}

	var raw [][]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Sprintf(`{"error":"failed to parse kline response: %s"}`, err)
	}

	candles := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		if len(row) < 7 {
			continue
		}
		candles = append(candles, map[string]any{
			"open_time":  row[0],
			"open":       row[1],
			"high":       row[2],
			"low":        row[3],
			"close":      row[4],
			"volume":     row[5],
			"close_time": row[6],
		})
	}

	out, _ := json.Marshal(map[string]any{
		"symbol":   symbol,
		"interval": interval,
		"limit":    limit,
		"klines":   candles,
	})
	return string(out)
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

func normalizeWatchSymbol(raw string) string {
	symbol := strings.ToUpper(strings.TrimSpace(raw))
	symbol = strings.ReplaceAll(symbol, " ", "")
	if symbol == "" {
		return ""
	}
	hasQuoteSuffix := strings.HasSuffix(symbol, "USDT") || strings.HasSuffix(symbol, "BUSD") || strings.HasSuffix(symbol, "USDC")
	if !hasQuoteSuffix && isStockSymbol(symbol) == false {
		return symbol + "USDT"
	}
	return symbol
}

func (a *Agent) toolGetWatchlist(lang string) string {
	if a.sentinel == nil {
		return fmt.Sprintf(`{"error":"%s"}`, a.msg(lang, "sentinel_off"))
	}
	symbols := a.sentinel.Symbols()
	payload := map[string]any{
		"enabled": true,
		"count":   len(symbols),
		"symbols": symbols,
		"text":    a.sentinel.FormatWatchlist(lang),
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func (a *Agent) toolManageWatchlist(lang, argsJSON string) string {
	if a.sentinel == nil {
		return fmt.Sprintf(`{"error":"%s"}`, a.msg(lang, "sentinel_off"))
	}

	var args struct {
		Action string `json:"action"`
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	symbol := normalizeWatchSymbol(args.Symbol)
	if symbol == "" {
		return `{"error":"symbol is required"}`
	}

	switch action {
	case "add":
		a.sentinel.AddSymbol(symbol)
	case "remove":
		a.sentinel.RemoveSymbol(symbol)
	default:
		return `{"error":"unsupported action"}`
	}

	symbols := a.sentinel.Symbols()
	if a.config != nil {
		a.config.WatchSymbols = symbols
	}

	message := ""
	if lang == "zh" {
		if action == "add" {
			message = fmt.Sprintf("已把 %s 加入监控。", symbol)
		} else {
			message = fmt.Sprintf("已把 %s 移出监控。", symbol)
		}
	} else {
		if action == "add" {
			message = fmt.Sprintf("Added %s to the watchlist.", symbol)
		} else {
			message = fmt.Sprintf("Removed %s from the watchlist.", symbol)
		}
	}

	payload := map[string]any{
		"ok":      true,
		"action":  action,
		"symbol":  symbol,
		"count":   len(symbols),
		"symbols": symbols,
		"message": message,
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
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
