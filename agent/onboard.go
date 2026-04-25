package agent

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"nofx/store"
)

var titleCaser = cases.Title(language.English)
const setupExchangeAccountName = "Default"

// Onboard handles first-time setup through natural language.
// When there's no trader configured, the agent guides the user.

// SetupState tracks where the user is in the setup flow.
type SetupState struct {
	Step       string // "", "await_exchange", "await_api_key", "await_api_secret", "await_passphrase", "await_ai_model", "await_ai_key"
	Exchange   string
	ExchangeID string
	APIKey     string
	APISecret  string
	Passphrase string
	AIProvider string
	AIModel    string
	AIModelID  string
	AIKey      string
	AIBaseURL  string
}

// needsSetup returns true if no traders are configured.
func (a *Agent) needsSetup() bool {
	if a.traderManager == nil {
		return true
	}
	return len(a.traderManager.GetAllTraders()) == 0
}

// getSetupState loads the current setup state from user preferences.
func (a *Agent) getSetupState(userID int64) *SetupState {
	step, _ := a.store.GetSystemConfig(fmt.Sprintf("setup_step_%d", userID))
	if step == "" {
		return &SetupState{}
	}
	return &SetupState{
		Step:       step,
		Exchange:   getConfig(a.store, userID, "exchange"),
		ExchangeID: getConfig(a.store, userID, "exchange_id"),
		APIKey:     getConfig(a.store, userID, "api_key"),
		APISecret:  getConfig(a.store, userID, "api_secret"),
		Passphrase: getConfig(a.store, userID, "passphrase"),
		AIProvider: getConfig(a.store, userID, "ai_provider"),
		AIModel:    getConfig(a.store, userID, "ai_model"),
		AIModelID:  getConfig(a.store, userID, "ai_model_id"),
		AIKey:      getConfig(a.store, userID, "ai_key"),
		AIBaseURL:  getConfig(a.store, userID, "ai_base_url"),
	}
}

func (a *Agent) saveSetupState(userID int64, s *SetupState) {
	a.store.SetSystemConfig(fmt.Sprintf("setup_step_%d", userID), s.Step)
	setConfig(a.store, userID, "exchange", s.Exchange)
	setConfig(a.store, userID, "exchange_id", s.ExchangeID)
	// Store only a masked marker for secrets — full values stay in memory only.
	// This prevents plaintext credentials from lingering in the config store
	// if the setup flow is interrupted before clearSetupState runs.
	if s.APIKey != "" {
		setConfig(a.store, userID, "api_key", "****")
	}
	if s.APISecret != "" {
		setConfig(a.store, userID, "api_secret", "****")
	}
	if s.Passphrase != "" {
		setConfig(a.store, userID, "passphrase", "****")
	}
	setConfig(a.store, userID, "ai_provider", s.AIProvider)
	setConfig(a.store, userID, "ai_model", s.AIModel)
	setConfig(a.store, userID, "ai_model_id", s.AIModelID)
	if s.AIKey != "" {
		setConfig(a.store, userID, "ai_key", "****")
	}
	setConfig(a.store, userID, "ai_base_url", s.AIBaseURL)
}

func (a *Agent) clearSetupState(userID int64) {
	for _, k := range []string{"step", "exchange", "exchange_id", "api_key", "api_secret", "passphrase", "ai_provider", "ai_model", "ai_model_id", "ai_key", "ai_base_url"} {
		if err := a.store.SetSystemConfig(fmt.Sprintf("setup_%s_%d", k, userID), ""); err != nil {
			a.log().Warn("clearSetupState: failed to clear key", "key", k, "error", err)
		}
	}
}

func getConfig(st *store.Store, uid int64, key string) string {
	v, _ := st.GetSystemConfig(fmt.Sprintf("setup_%s_%d", key, uid))
	return v
}

func setConfig(st *store.Store, uid int64, key, val string) {
	st.SetSystemConfig(fmt.Sprintf("setup_%s_%d", key, uid), val)
}

// handleSetupFlow processes the setup conversation.
// Returns (response, handled). If handled=false, continue to normal routing.
func (a *Agent) handleSetupFlow(userID int64, text string, L string) (string, bool) {
	return a.handleSetupFlowForStoreUser("default", userID, text, L)
}

func (a *Agent) handleSetupFlowForStoreUser(storeUserID string, userID int64, text string, L string) (string, bool) {
	state := a.getSetupState(userID)

	lower := strings.ToLower(text)

	// Cancel setup — explicit or implicit (user asking unrelated questions)
	if lower == "cancel" || lower == "取消" || lower == "/cancel" {
		a.clearSetupState(userID)
		return a.setupMsg(L, "cancelled"), true
	}

	// If in a step that expects a key/secret, check if user is NOT sending a key
	// Keys are typically long strings without spaces and Chinese characters
	if state.Step == "await_api_key" || state.Step == "await_api_secret" || state.Step == "await_passphrase" || state.Step == "await_ai_key" {
		trimmed := strings.TrimSpace(text)
		hasChinese := false
		for _, r := range trimmed {
			if r >= 0x4e00 && r <= 0x9fff {
				hasChinese = true
				break
			}
		}
		hasSpaces := strings.Contains(trimmed, " ") && !strings.HasPrefix(trimmed, "sk-")
		tooShort := len(trimmed) < 8

		if hasChinese || hasSpaces || tooShort {
			// User is probably asking a question, not providing a key
			a.clearSetupState(userID)
			if L == "zh" {
				return "👌 配置已暂停。我先回答你的问题——\n\n随时发送 *开始配置* 继续配置。", false
			}
			return "👌 Setup paused. Let me answer your question first—\n\nSend *setup* anytime to continue.", false
		}
	}

	switch state.Step {
	case "await_exchange":
		return a.handleExchangeChoice(userID, text, state, L)
	case "await_api_key":
		state.APIKey = strings.TrimSpace(text)
		state.Step = "await_api_secret"
		a.saveSetupState(userID, state)
		return a.setupMsg(L, "ask_secret"), true
	case "await_api_secret":
		state.APISecret = strings.TrimSpace(text)
		// OKX/Bitget/KuCoin need passphrase
		if needsPassphrase(state.Exchange) {
			state.Step = "await_passphrase"
			a.saveSetupState(userID, state)
			return a.setupMsg(L, "ask_passphrase"), true
		}
		exchangeID, err := a.saveSetupExchange(storeUserID, state)
		if err != nil {
			a.logger.Error("save exchange from setup failed", "error", err, "exchange", state.Exchange, "store_user_id", storeUserID)
			if L == "zh" {
				return fmt.Sprintf("⚠️ 交易所配置保存失败: %v\n请再试一次，或稍后去 Web UI 继续。", err), true
			}
			return fmt.Sprintf("⚠️ Failed to save exchange config: %v\nPlease try again, or continue later in the Web UI.", err), true
		}
		state.ExchangeID = exchangeID
		state.Step = "await_ai_model"
		a.saveSetupState(userID, state)
		if L == "zh" {
			return "✅ 交易所配置已保存，在配置页里现在就能看到。\n\n" + a.setupMsg(L, "ask_ai"), true
		}
		return "✅ Exchange config saved. It should now be visible in the config page.\n\n" + a.setupMsg(L, "ask_ai"), true
	case "await_passphrase":
		state.Passphrase = strings.TrimSpace(text)
		exchangeID, err := a.saveSetupExchange(storeUserID, state)
		if err != nil {
			a.logger.Error("save exchange from setup failed", "error", err, "exchange", state.Exchange, "store_user_id", storeUserID)
			if L == "zh" {
				return fmt.Sprintf("⚠️ 交易所配置保存失败: %v\n请再试一次，或稍后去 Web UI 继续。", err), true
			}
			return fmt.Sprintf("⚠️ Failed to save exchange config: %v\nPlease try again, or continue later in the Web UI.", err), true
		}
		state.ExchangeID = exchangeID
		state.Step = "await_ai_model"
		a.saveSetupState(userID, state)
		if L == "zh" {
			return "✅ 交易所配置已保存，在配置页里现在就能看到。\n\n" + a.setupMsg(L, "ask_ai"), true
		}
		return "✅ Exchange config saved. It should now be visible in the config page.\n\n" + a.setupMsg(L, "ask_ai"), true
	case "await_ai_model":
		return a.handleAIChoice(storeUserID, userID, text, state, L)
	case "await_ai_key":
		state.AIKey = strings.TrimSpace(text)
		aiModelID, err := a.saveSetupAIModel(storeUserID, state)
		if err != nil {
			a.logger.Error("save AI model from setup failed", "error", err, "provider", state.AIProvider, "store_user_id", storeUserID)
			if L == "zh" {
				return fmt.Sprintf("⚠️ AI 模型配置保存失败: %v\n请再试一次，或稍后去 Web UI 继续。", err), true
			}
			return fmt.Sprintf("⚠️ Failed to save AI model config: %v\nPlease try again, or continue later in the Web UI.", err), true
		}
		state.AIModelID = aiModelID
		return a.finishSetup(storeUserID, userID, state, L)
	}

	// Not in setup flow — only enter setup for a tiny set of explicit commands.
	// Natural-language configuration requests should go to the planner first,
	// including phrases like "开始配置" or "帮我配置交易所".
	if isDirectSetupCommand(lower) {
		state.Step = "await_exchange"
		a.saveSetupState(userID, state)
		return a.setupMsg(L, "ask_exchange"), true
	}

	// Everything else — let normal routing handle it
	return "", false
}

func isDirectSetupCommand(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	switch text {
	case "setup", "/setup", "开始配置", "配置", "开始设置":
		return true
	default:
		return false
	}
}

func (a *Agent) handleExchangeChoice(userID int64, text string, state *SetupState, L string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))

	exchanges := map[string]string{
		"binance": "binance", "币安": "binance", "1": "binance",
		"okx": "okx", "欧易": "okx", "2": "okx",
		"bybit": "bybit", "3": "bybit",
		"bitget": "bitget", "4": "bitget",
		"gate": "gate", "5": "gate",
		"kucoin": "kucoin", "库币": "kucoin", "6": "kucoin",
		"hyperliquid": "hyperliquid", "7": "hyperliquid",
	}

	ex, ok := exchanges[lower]
	if !ok {
		return a.setupMsg(L, "invalid_exchange"), true
	}

	state.Exchange = ex
	state.Step = "await_api_key"
	a.saveSetupState(userID, state)

	if L == "zh" {
		return fmt.Sprintf("✅ 选择了 *%s*\n\n请发送你的 API Key：", titleCaser.String(ex)), true
	}
	return fmt.Sprintf("✅ Selected *%s*\n\nPlease send your API Key:", titleCaser.String(ex)), true
}

func (a *Agent) handleAIChoice(storeUserID string, userID int64, text string, state *SetupState, L string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))

	models := map[string]struct{ provider, model, url string }{
		"deepseek":  {"deepseek", "deepseek-chat", "https://api.deepseek.com/v1"},
		"1":         {"deepseek", "deepseek-chat", "https://api.deepseek.com/v1"},
		"qwen":      {"qwen", "qwen-plus", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		"通义":       {"qwen", "qwen-plus", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		"2":         {"qwen", "qwen-plus", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		"openai":    {"openai", "gpt-4o", "https://api.openai.com/v1"},
		"gpt":       {"openai", "gpt-4o", "https://api.openai.com/v1"},
		"3":         {"openai", "gpt-4o", "https://api.openai.com/v1"},
		"claude":    {"claude", "claude-3-5-sonnet-20241022", "https://api.anthropic.com/v1"},
		"4":         {"claude", "claude-3-5-sonnet-20241022", "https://api.anthropic.com/v1"},
		"skip":      {"", "", ""},
		"跳过":       {"", "", ""},
		"5":         {"", "", ""},
	}

	choice, ok := models[lower]
	if !ok {
		return a.setupMsg(L, "invalid_ai"), true
	}

	if choice.model == "" {
		// Skip AI, just create trader with exchange
		state.AIProvider = ""
		state.AIModel = ""
		state.AIModelID = ""
		state.AIKey = ""
		return a.finishSetup(storeUserID, userID, state, L)
	}

	state.AIProvider = choice.provider
	state.AIModel = choice.model
	state.AIBaseURL = choice.url
	state.Step = "await_ai_key"
	a.saveSetupState(userID, state)

	if L == "zh" {
		return fmt.Sprintf("✅ AI 模型: *%s*\n\n请发送你的 API Key：", choice.model), true
	}
	return fmt.Sprintf("✅ AI Model: *%s*\n\nPlease send your API Key:", choice.model), true
}

func (a *Agent) finishSetup(storeUserID string, userID int64, state *SetupState, L string) (string, bool) {
	// Create exchange in store
	a.logger.Info("creating trader from setup",
		"exchange", state.Exchange,
		"ai_model", state.AIModel,
		"store_user_id", storeUserID,
	)

	// TODO: Use store to create exchange + trader config
	// For now, log the config and tell user
	a.clearSetupState(userID)

	result := ""
	maskedKey := maskKey(state.APIKey)
	if L == "zh" {
		result = fmt.Sprintf("🎉 *配置完成！*\n\n"+
			"• 交易所: %s\n"+
			"• API Key: %s\n",
			titleCaser.String(state.Exchange), maskedKey)
		if state.AIModel != "" {
			result += fmt.Sprintf("• AI 模型: %s\n", state.AIModel)
		}
		result += "\n正在创建 Trader..."
	} else {
		result = fmt.Sprintf("🎉 *Setup Complete!*\n\n"+
			"• Exchange: %s\n"+
			"• API Key: %s\n",
			titleCaser.String(state.Exchange), maskedKey)
		if state.AIModel != "" {
			result += fmt.Sprintf("• AI Model: %s\n", state.AIModel)
		}
		result += "\nCreating Trader..."
	}

	// Actually create the trader via store
	err := a.createTraderFromSetupForStoreUser(storeUserID, state)
	if err != nil {
		a.logger.Error("create trader failed", "error", err)
		if L == "zh" {
			result += fmt.Sprintf("\n\n⚠️ 创建失败: %v\n交易所配置已保存，下次配置时可直接复用。\n也可以在 Web UI 中继续完成。", err)
		} else {
			result += fmt.Sprintf("\n\n⚠️ Failed: %v\nYour exchange config was saved, so you can reuse it next time.\nYou can also finish setup in the Web UI.", err)
		}
	} else {
		if L == "zh" {
			result += "\n\n✅ Trader 已创建！现在你可以:\n• `/analyze BTC` — 分析市场\n• `/positions` — 查看持仓\n• 或者直接跟我聊天"
		} else {
			result += "\n\n✅ Trader created! Now you can:\n• `/analyze BTC` — analyze market\n• `/positions` — view positions\n• Or just chat with me"
		}
	}

	return result, true
}

func (a *Agent) createTraderFromSetup(state *SetupState) error {
	return a.createTraderFromSetupForStoreUser("default", state)
}

func (a *Agent) createTraderFromSetupForStoreUser(storeUserID string, state *SetupState) error {
	if a.store == nil {
		return fmt.Errorf("store not available")
	}
	exchangeID := state.ExchangeID
	if exchangeID == "" {
		var err error
		exchangeID, err = a.saveSetupExchange(storeUserID, state)
		if err != nil {
			return fmt.Errorf("save exchange: %w", err)
		}
	}

	aiModelID := state.AIModelID
	if state.AIModel != "" && state.AIKey != "" && aiModelID == "" {
		var err error
		aiModelID, err = a.saveSetupAIModel(storeUserID, state)
		if err != nil {
			a.logger.Error("save AI model", "error", err)
		}
	}

	// Reuse an existing trader if the same exchange/model pair already exists.
	existingTraders, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return fmt.Errorf("list traders: %w", err)
	}
	for _, existing := range existingTraders {
		if existing.ExchangeID == exchangeID && existing.AIModelID == aiModelID {
			a.logger.Info("reusing existing trader created via chat setup",
				"trader", existing.Name,
				"exchange_id", exchangeID,
				"ai_model_id", aiModelID,
			)
			return nil
		}
	}

	// Create trader config
	exchangeIDShort := exchangeID
	if len(exchangeIDShort) > 8 {
		exchangeIDShort = exchangeIDShort[:8]
	}
	modelPart := aiModelID
	if modelPart == "" {
		modelPart = "manual"
	}
	trader := &store.Trader{
		ID:         fmt.Sprintf("%s_%s_%d", exchangeIDShort, modelPart, time.Now().UnixNano()),
		Name:       fmt.Sprintf("NOFXi-%s", titleCaser.String(state.Exchange)),
		UserID:     storeUserID,
		ExchangeID: exchangeID,
		AIModelID:  aiModelID,
		IsRunning:  false,
	}
	if err := a.store.Trader().Create(trader); err != nil {
		return fmt.Errorf("save trader: %w", err)
	}

	a.logger.Info("trader created via chat",
		"trader", trader.Name,
		"exchange", state.Exchange,
		"ai", aiModelID,
	)

	return nil
}

func (a *Agent) saveSetupExchange(storeUserID string, state *SetupState) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not available")
	}

	hlWallet := ""
	hlUnified := false
	passphrase := state.Passphrase
	apiKey := state.APIKey
	apiSecret := state.APISecret

	if state.Exchange == "hyperliquid" {
		hlWallet = state.APISecret
		apiKey = ""
		apiSecret = state.APIKey
	}

	exchanges, err := a.store.Exchange().List(storeUserID)
	if err != nil {
		return "", err
	}
	for _, ex := range exchanges {
		if ex.ExchangeType == state.Exchange && ex.AccountName == setupExchangeAccountName {
			if err := a.store.Exchange().Update(
				storeUserID, ex.ID, true,
				apiKey, apiSecret, passphrase,
				false,
				hlWallet, hlUnified,
				"", "", "",
				"", "", "", 0,
			); err != nil {
				return "", err
			}
			return ex.ID, nil
		}
	}

	return a.store.Exchange().Create(
		storeUserID,
		state.Exchange,
		setupExchangeAccountName,
		true,
		apiKey, apiSecret, passphrase,
		false,
		hlWallet, hlUnified,
		"", "", "",
		"", "", "", 0,
	)
}

func (a *Agent) saveSetupAIModel(storeUserID string, state *SetupState) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not available")
	}
	if state.AIProvider == "" {
		return "", nil
	}

	modelID := state.AIProvider
	if err := a.store.AIModel().Update(
		storeUserID,
		modelID,
		true,
		state.AIKey,
		state.AIBaseURL,
		state.AIModel,
	); err != nil {
		return "", err
	}

	modelID = fmt.Sprintf("%s_%s", storeUserID, state.AIProvider)
	return modelID, nil
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func needsPassphrase(exchange string) bool {
	return exchange == "okx" || exchange == "bitget" || exchange == "kucoin"
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

var setupMessages = map[string]map[string]string{
	"welcome": {
		"zh": "👋 你好！我是 *NOFXi*，你的 AI 交易 Agent。\n\n" +
			"我发现你还没有配置交易所，让我帮你搞定吧！\n\n" +
			"发送 *开始配置* 或 *setup* 开始\n" +
			"发送 *取消* 随时退出",
		"en": "👋 Hi! I'm *NOFXi*, your AI trading agent.\n\n" +
			"I see you haven't configured an exchange yet. Let me help!\n\n" +
			"Send *setup* to begin\n" +
			"Send *cancel* to exit anytime",
	},
	"ask_exchange": {
		"zh": "🏦 *选择你的交易所*\n\n" +
			"1️⃣ Binance（币安）\n" +
			"2️⃣ OKX（欧易）\n" +
			"3️⃣ Bybit\n" +
			"4️⃣ Bitget\n" +
			"5️⃣ Gate\n" +
			"6️⃣ KuCoin（库币）\n" +
			"7️⃣ Hyperliquid\n\n" +
			"发送数字或名称选择：",
		"en": "🏦 *Choose your exchange*\n\n" +
			"1️⃣ Binance\n" +
			"2️⃣ OKX\n" +
			"3️⃣ Bybit\n" +
			"4️⃣ Bitget\n" +
			"5️⃣ Gate\n" +
			"6️⃣ KuCoin\n" +
			"7️⃣ Hyperliquid\n\n" +
			"Send number or name:",
	},
	"invalid_exchange": {
		"zh": "❓ 没有识别到交易所。请发送数字 1-7 或交易所名称。",
		"en": "❓ Exchange not recognized. Send a number 1-7 or exchange name.",
	},
	"ask_secret": {
		"zh": "🔑 收到 API Key。\n\n现在请发送你的 *API Secret*：",
		"en": "🔑 Got API Key.\n\nNow send your *API Secret*:",
	},
	"ask_passphrase": {
		"zh": "🔐 收到 API Secret。\n\n这个交易所还需要 *Passphrase*，请发送：",
		"en": "🔐 Got API Secret.\n\nThis exchange also needs a *Passphrase*. Please send it:",
	},
	"ask_ai": {
		"zh": "🤖 *选择 AI 模型*\n\n" +
			"1️⃣ DeepSeek（推荐，便宜好用）\n" +
			"2️⃣ 通义千问 (Qwen)\n" +
			"3️⃣ OpenAI (GPT-4o)\n" +
			"4️⃣ Claude\n" +
			"5️⃣ 跳过（不配置 AI）\n\n" +
			"发送数字或名称选择：",
		"en": "🤖 *Choose AI model*\n\n" +
			"1️⃣ DeepSeek (recommended, affordable)\n" +
			"2️⃣ Qwen\n" +
			"3️⃣ OpenAI (GPT-4o)\n" +
			"4️⃣ Claude\n" +
			"5️⃣ Skip (no AI)\n\n" +
			"Send number or name:",
	},
	"invalid_ai": {
		"zh": "❓ 没有识别到 AI 模型。请发送数字 1-5 或模型名称。",
		"en": "❓ AI model not recognized. Send a number 1-5 or model name.",
	},
	"cancelled": {
		"zh": "👌 配置已取消。随时发送 *开始配置* 重新开始。",
		"en": "👌 Setup cancelled. Send *setup* anytime to restart.",
	},
}

func (a *Agent) setupMsg(L, key string) string {
	if m, ok := setupMessages[key]; ok {
		if s, ok := m[L]; ok {
			return s
		}
		return m["en"]
	}
	return key
}
