package agent

import (
	"fmt"
	"strings"

	"nofx/store"
)

// Onboard handles first-time setup through natural language.
// When there's no trader configured, the agent guides the user.

// SetupState tracks where the user is in the setup flow.
type SetupState struct {
	Step       string // "", "await_exchange", "await_api_key", "await_api_secret", "await_passphrase", "await_ai_model", "await_ai_key"
	Exchange   string
	APIKey     string
	APISecret  string
	Passphrase string
	AIModel    string
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
		Step:      step,
		Exchange:  getConfig(a.store, userID, "exchange"),
		APIKey:    getConfig(a.store, userID, "api_key"),
		APISecret: getConfig(a.store, userID, "api_secret"),
		Passphrase: getConfig(a.store, userID, "passphrase"),
		AIModel:   getConfig(a.store, userID, "ai_model"),
		AIKey:     getConfig(a.store, userID, "ai_key"),
		AIBaseURL: getConfig(a.store, userID, "ai_base_url"),
	}
}

func (a *Agent) saveSetupState(userID int64, s *SetupState) {
	a.store.SetSystemConfig(fmt.Sprintf("setup_step_%d", userID), s.Step)
	setConfig(a.store, userID, "exchange", s.Exchange)
	setConfig(a.store, userID, "api_key", s.APIKey)
	setConfig(a.store, userID, "api_secret", s.APISecret)
	setConfig(a.store, userID, "passphrase", s.Passphrase)
	setConfig(a.store, userID, "ai_model", s.AIModel)
	setConfig(a.store, userID, "ai_key", s.AIKey)
	setConfig(a.store, userID, "ai_base_url", s.AIBaseURL)
}

func (a *Agent) clearSetupState(userID int64) {
	for _, k := range []string{"step", "exchange", "api_key", "api_secret", "passphrase", "ai_model", "ai_key", "ai_base_url"} {
		a.store.SetSystemConfig(fmt.Sprintf("setup_%s_%d", k, userID), "")
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
	state := a.getSetupState(userID)

	// Cancel setup
	lower := strings.ToLower(text)
	if lower == "cancel" || lower == "取消" || lower == "/cancel" {
		a.clearSetupState(userID)
		return a.setupMsg(L, "cancelled"), true
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
		state.Step = "await_ai_model"
		a.saveSetupState(userID, state)
		return a.setupMsg(L, "ask_ai"), true
	case "await_passphrase":
		state.Passphrase = strings.TrimSpace(text)
		state.Step = "await_ai_model"
		a.saveSetupState(userID, state)
		return a.setupMsg(L, "ask_ai"), true
	case "await_ai_model":
		return a.handleAIChoice(userID, text, state, L)
	case "await_ai_key":
		state.AIKey = strings.TrimSpace(text)
		return a.finishSetup(userID, state, L)
	}

	// Not in setup flow — check if setup is needed and user seems to want to start
	if a.needsSetup() {
		// Any message triggers setup prompt when no trader exists
		if lower == "/help" || lower == "/status" || lower == "help" {
			return a.setupMsg(L, "welcome"), true
		}
		// Check if user is trying to set up
		if containsAny(lower, []string{"connect", "setup", "配置", "连接", "设置", "开始", "start", "初始化"}) {
			state.Step = "await_exchange"
			a.saveSetupState(userID, state)
			return a.setupMsg(L, "ask_exchange"), true
		}
		// First time — show welcome
		if state.Step == "" {
			return a.setupMsg(L, "welcome"), true
		}
	}

	return "", false
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
		return fmt.Sprintf("✅ 选择了 *%s*\n\n请发送你的 API Key：", strings.Title(ex)), true
	}
	return fmt.Sprintf("✅ Selected *%s*\n\nPlease send your API Key:", strings.Title(ex)), true
}

func (a *Agent) handleAIChoice(userID int64, text string, state *SetupState, L string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))

	models := map[string]struct{ model, url string }{
		"deepseek":  {"deepseek-chat", "https://api.deepseek.com/v1"},
		"1":         {"deepseek-chat", "https://api.deepseek.com/v1"},
		"qwen":      {"qwen-plus", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		"通义":       {"qwen-plus", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		"2":         {"qwen-plus", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		"openai":    {"gpt-4o", "https://api.openai.com/v1"},
		"gpt":       {"gpt-4o", "https://api.openai.com/v1"},
		"3":         {"gpt-4o", "https://api.openai.com/v1"},
		"claude":    {"claude-3-5-sonnet-20241022", "https://api.anthropic.com/v1"},
		"4":         {"claude-3-5-sonnet-20241022", "https://api.anthropic.com/v1"},
		"skip":      {"", ""},
		"跳过":       {"", ""},
		"5":         {"", ""},
	}

	choice, ok := models[lower]
	if !ok {
		return a.setupMsg(L, "invalid_ai"), true
	}

	if choice.model == "" {
		// Skip AI, just create trader with exchange
		state.AIModel = ""
		state.AIKey = ""
		return a.finishSetup(userID, state, L)
	}

	state.AIModel = choice.model
	state.AIBaseURL = choice.url
	state.Step = "await_ai_key"
	a.saveSetupState(userID, state)

	if L == "zh" {
		return fmt.Sprintf("✅ AI 模型: *%s*\n\n请发送你的 API Key：", choice.model), true
	}
	return fmt.Sprintf("✅ AI Model: *%s*\n\nPlease send your API Key:", choice.model), true
}

func (a *Agent) finishSetup(userID int64, state *SetupState, L string) (string, bool) {
	// Create exchange in store
	a.logger.Info("creating trader from setup",
		"exchange", state.Exchange,
		"ai_model", state.AIModel,
	)

	// TODO: Use store to create exchange + trader config
	// For now, log the config and tell user
	a.clearSetupState(userID)

	result := ""
	if L == "zh" {
		result = fmt.Sprintf("🎉 *配置完成！*\n\n"+
			"• 交易所: %s\n"+
			"• API Key: %s...%s\n",
			strings.Title(state.Exchange),
			state.APIKey[:4], state.APIKey[len(state.APIKey)-4:])
		if state.AIModel != "" {
			result += fmt.Sprintf("• AI 模型: %s\n", state.AIModel)
		}
		result += "\n正在创建 Trader..."
	} else {
		result = fmt.Sprintf("🎉 *Setup Complete!*\n\n"+
			"• Exchange: %s\n"+
			"• API Key: %s...%s\n",
			strings.Title(state.Exchange),
			state.APIKey[:4], state.APIKey[len(state.APIKey)-4:])
		if state.AIModel != "" {
			result += fmt.Sprintf("• AI Model: %s\n", state.AIModel)
		}
		result += "\nCreating Trader..."
	}

	// Actually create the trader via store
	err := a.createTraderFromSetup(state)
	if err != nil {
		a.logger.Error("create trader failed", "error", err)
		if L == "zh" {
			result += fmt.Sprintf("\n\n⚠️ 创建失败: %v\n请在 Web UI 中手动配置。", err)
		} else {
			result += fmt.Sprintf("\n\n⚠️ Failed: %v\nPlease configure in Web UI.", err)
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
	if a.store == nil {
		return fmt.Errorf("store not available")
	}

	// Create exchange config
	hlWallet := ""
	hlUnified := false
	passphrase := state.Passphrase
	apiKey := state.APIKey
	apiSecret := state.APISecret

	if state.Exchange == "hyperliquid" {
		hlWallet = state.APISecret
		apiKey = ""
		apiSecret = state.APIKey // private key goes as secret
	}

	exchangeID, err := a.store.Exchange().Create(
		"default",        // userID
		state.Exchange,   // exchangeType
		state.Exchange,   // accountName
		true,             // enabled
		apiKey, apiSecret, passphrase,
		false,            // testnet
		hlWallet, hlUnified,
		"", "", "",       // aster
		"", "", "", 0,    // lighter
	)
	if err != nil {
		return fmt.Errorf("save exchange: %w", err)
	}

	// Create AI model config if provided
	aiModelID := ""
	if state.AIModel != "" && state.AIKey != "" {
		aiModelID = state.AIModel
		err := a.store.AIModel().Create(
			"default",      // userID
			state.AIModel,  // id
			state.AIModel,  // name
			"custom",       // provider
			true,           // enabled
			state.AIKey,    // apiKey
			state.AIBaseURL, // customAPIURL
		)
		if err != nil {
			a.logger.Error("save AI model", "error", err)
		}
	}

	// Create trader config
	trader := &store.Trader{
		Name:       fmt.Sprintf("NOFXi-%s", strings.Title(state.Exchange)),
		UserID:     "default",
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
