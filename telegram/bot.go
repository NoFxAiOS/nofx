package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/api"
	"nofx/config"
	"nofx/logger"
	"nofx/mcp"
	"nofx/store"
	"nofx/telegram/agent"
	"os"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Start initializes and runs the Telegram bot in a blocking supervisor loop.
// Supports hot-reload: when a signal is sent on reloadCh, the bot restarts
// with the latest token (re-read from DB or env). Must be called as a goroutine from main.go.
// Deployment note: uses long-polling (not webhook) — safe for private networks,
// no inbound ports required.
func Start(cfg *config.Config, st *store.Store, reloadCh <-chan struct{}) {
	for {
		token := resolveToken(cfg, st)
		if token == "" {
			logger.Info("Telegram bot disabled (no token configured), waiting for reload signal...")
			// Block until a reload signal arrives, then re-check for a token.
			<-reloadCh
			continue
		}

		stopped := runBot(token, cfg, st)
		if !stopped {
			// Bot exited with an unrecoverable error; do not restart automatically.
			return
		}

		// Bot was stopped cleanly. Wait for a reload signal before restarting.
		select {
		case <-reloadCh:
			logger.Info("Reloading Telegram bot with new token...")
		}
	}
}

// resolveToken returns the bot token, preferring the DB-stored value over the env/config value.
func resolveToken(cfg *config.Config, st *store.Store) string {
	dbCfg, err := st.TelegramConfig().Get()
	if err == nil && dbCfg.BotToken != "" {
		return dbCfg.BotToken
	}
	return cfg.TelegramBotToken
}

// runBot runs the bot until StopReceivingUpdates is called (clean stop → true)
// or a fatal error occurs (false).
func runBot(token string, cfg *config.Config, st *store.Store) bool {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logger.Errorf("Telegram bot failed to start: %v", err)
		return false
	}
	logger.Infof("Telegram bot @%s started (long-polling mode)", bot.Self.UserName)

	// Determine allowed chat ID:
	// Priority 1: env var TELEGRAM_ADMIN_CHAT_ID (explicit)
	// Priority 2: DB-stored bound chat ID (set by /start)
	// Priority 3: 0 = no binding yet, first /start will bind
	allowedChatID := cfg.TelegramAdminChatID
	if allowedChatID == 0 {
		if id, err := st.TelegramConfig().GetBoundChatID(); err == nil && id != 0 {
			allowedChatID = id
		}
	}

	// botUserID/botToken/agents are resolved lazily on the first message.
	// They refresh automatically when the user registers on the web UI.
	var (
		botUserID string
		botToken  string
		agents    *agent.Manager
	)

	// resolveBotUser reads the first registered user from DB.
	// Returns true and refreshes botUserID/botToken/agents when the user changes.
	// Returns false if no user is registered yet (no "default" fallback).
	resolveBotUser := func() bool {
		ids, err := st.User().GetAllIDs()
		if err != nil || len(ids) == 0 {
			return false
		}
		newID := ids[0]
		if newID == botUserID {
			return true // already up-to-date
		}
		// User changed (or first resolve) — regenerate JWT and agent manager.
		newToken, tokenErr := agent.GenerateBotToken(newID)
		if tokenErr != nil {
			logger.Errorf("Failed to generate bot JWT for user %s: %v", newID, tokenErr)
			return false
		}
		prevID := botUserID
		botUserID = newID
		botToken = newToken
		agents = agent.NewManager(cfg.APIServerPort, botToken, botUserID,
			func() mcp.AIClient { return newLLMClient(st, botUserID) },
			api.GetAPIDocs(),
		)
		if prevID == "" {
			logger.Infof("Bot: resolved user %s", botUserID)
		} else {
			logger.Infof("Bot: user changed %s → %s, agent manager refreshed", prevID, botUserID)
		}
		return true
	}

	// Initial resolve — may fail if no user registered yet, which is fine.
	resolveBotUser()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// awaitingLang is true when the bot is waiting for the user to pick a language (1 or 2).
	// It resets to false once a valid choice is received or /lang is re-issued.
	awaitingLang := false

	for update := range updates {
		if update.Message == nil {
			continue
		}
		chatID := update.Message.Chat.ID
		text := update.Message.Text

		// Language selection state: user must choose "1" or "2" after /start on first use.
		// awaitingLang is true only until the user makes a choice (or we fall back to "en").
		if awaitingLang && chatID == allowedChatID {
			lang := parseLangChoice(text)
			if lang != "" {
				awaitingLang = false
				st.TelegramConfig().SetLanguage(lang) //nolint:errcheck
				sendMarkdownMsg(bot, chatID, buildSetupGuide(st, botUserID, cfg.APIServerPort, botToken, lang))
			} else {
				sendMarkdownMsg(bot, chatID, langSelectionMsg())
			}
			continue
		}

		// Handle /start: auto-bind or language selection / welcome
		if text == "/start" {
			// Re-resolve user on every /start (handles: registered after bot launch).
			resolveBotUser()
			if botUserID == "" {
				sendMsg(bot, chatID, "No account found. Please register on the web UI first, then send /start.")
				continue
			}
			if allowedChatID == 0 {
				username := update.Message.From.UserName
				if err := st.TelegramConfig().BindUser(chatID, "@"+username); err != nil {
					logger.Errorf("Failed to bind Telegram user: %v", err)
					sendMsg(bot, chatID, "Binding failed. Please try again.")
					continue
				}
				allowedChatID = chatID
				logger.Infof("Telegram bound to @%s (chatID: %d)", username, chatID)
			} else if chatID != allowedChatID {
				sendMsg(bot, chatID, "This bot is already bound to another user.")
				continue
			} else {
				agents.Reset(chatID)
			}
			// Show language selection if user has never chosen (Language == ""); go straight to guide otherwise.
			if isLangDefault(st) {
				awaitingLang = true
				sendMarkdownMsg(bot, chatID, langSelectionMsg())
			} else {
				lang := st.TelegramConfig().GetLanguage()
				sendMarkdownMsg(bot, chatID, buildSetupGuide(st, botUserID, cfg.APIServerPort, botToken, lang))
			}
			continue
		}

		// Handle /lang: change language at any time
		if text == "/lang" {
			awaitingLang = true
			sendMarkdownMsg(bot, chatID, langSelectionMsg())
			continue
		}

		// Handle /help
		if text == "/help" {
			lang := st.TelegramConfig().GetLanguage()
			sendMarkdownMsg(bot, chatID, helpMessage(lang))
			continue
		}

		// Access control
		if allowedChatID != 0 && chatID != allowedChatID {
			sendMsg(bot, chatID, "Unauthorized.")
			continue
		}
		if allowedChatID == 0 {
			sendMsg(bot, chatID, "Send /start first.")
			continue
		}
		if text == "" {
			continue
		}

		// Refresh user on every message (handles registration that happened mid-session).
		resolveBotUser()
		if botUserID == "" {
			sendMsg(bot, chatID, "No account found. Please register on the web UI first.")
			continue
		}

		// Direct setup commands (no LLM needed): "configure deepseek sk-xxx" / "配置 deepseek sk-xxx"
		lang := st.TelegramConfig().GetLanguage()
		if reply, handled := tryHandleSetupCommand(text, cfg.APIServerPort, botToken, st, botUserID, lang); handled {
			sendMarkdownMsg(bot, chatID, reply)
			continue
		}

		// Guard: if no AI model configured, show setup guide instead of failing.
		if newLLMClient(st, botUserID) == nil {
			sendMarkdownMsg(bot, chatID, buildSetupGuide(st, botUserID, cfg.APIServerPort, botToken, lang))
			continue
		}

		// Send a placeholder immediately, then stream-edit as reply arrives.
		go func(chatID int64, text string) {
			// Send ⏳ placeholder so the user sees an instant response.
			sent, err := bot.Send(tgbotapi.NewMessage(chatID, "⏳"))
			placeholderID := 0
			if err == nil {
				placeholderID = sent.MessageID
			}

			// Rate-limited edit helper: edits the placeholder at most once per second.
			// Exception: "⏳" thinking-indicator resets always go through immediately
			// so the user always sees the state change between agent iterations.
			var (
				mu       sync.Mutex
				lastEdit time.Time
			)
			onChunk := func(accumulated string) {
				if placeholderID == 0 {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				isThinking := accumulated == "⏳"
				if !isThinking && time.Since(lastEdit) < time.Second {
					return
				}
				lastEdit = time.Now()
				edit := tgbotapi.NewEditMessageText(chatID, placeholderID, accumulated)
				bot.Send(edit) //nolint:errcheck
			}

			reply := agents.Run(chatID, text, onChunk)

			// Final edit: use Markdown, fall back to plain text on parse error.
			if placeholderID != 0 {
				edit := tgbotapi.NewEditMessageText(chatID, placeholderID, reply)
				edit.ParseMode = "Markdown"
				if _, err := bot.Send(edit); err != nil {
					edit2 := tgbotapi.NewEditMessageText(chatID, placeholderID, reply)
					bot.Send(edit2) //nolint:errcheck
				}
			} else {
				msg := tgbotapi.NewMessage(chatID, reply)
				msg.ParseMode = "Markdown"
				if _, err := bot.Send(msg); err != nil {
					msg.ParseMode = ""
					bot.Send(msg) //nolint:errcheck
				}
			}
		}(chatID, text)
	}

	// updates channel was closed — bot stopped cleanly
	return true
}

func sendMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg) //nolint:errcheck
}

// sendMarkdownMsg sends a message with Markdown formatting; falls back to plain text on parse error.
func sendMarkdownMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := bot.Send(msg); err != nil {
		// Markdown rejected (e.g. unescaped special chars) — fall back to plain text.
		plain := tgbotapi.NewMessage(chatID, text)
		bot.Send(plain) //nolint:errcheck
	}
}

// newLLMClient builds an LLM client for the agent using the bound user's enabled model.
// Priority: bound user's DB model > environment variables.
// Uses provider-specific constructors to ensure correct default base URLs and models.
func newLLMClient(st *store.Store, userID string) mcp.AIClient {
	// 1. Try the bound user's enabled model from DB (configured via Web UI)
	if model, err := st.AIModel().GetDefault(userID); err == nil {
		apiKey := string(model.APIKey)
		if apiKey != "" {
			client := clientForProvider(model.Provider)
			client.SetAPIKey(apiKey, model.CustomAPIURL, model.CustomModelName)
			logger.Infof("Telegram agent: provider=%s user_id=%s model=%q url=%q",
				model.Provider, model.UserID, model.CustomModelName, model.CustomAPIURL)
			return client
		}
		logger.Warnf("Telegram: DB model found (provider=%s) but API key is empty after decryption", model.Provider)
	} else {
		logger.Warnf("Telegram: no enabled model for user %s (%v), trying env vars", userID, err)
	}

	// 2. Fall back to environment variables
	for _, pair := range []struct{ provider, key, url string }{
		{"deepseek", os.Getenv("DEEPSEEK_API_KEY"), mcp.DefaultDeepSeekBaseURL},
		{"openai", os.Getenv("OPENAI_API_KEY"), ""},
		{"claude", os.Getenv("ANTHROPIC_API_KEY"), ""},
	} {
		if pair.key != "" {
			client := clientForProvider(pair.provider)
			client.SetAPIKey(pair.key, pair.url, "")
			logger.Infof("Telegram agent: using %s from env var", pair.provider)
			return client
		}
	}

	logger.Warn("Telegram: no AI key found in DB or env — agent will fail. Configure a model in the Web UI.")
	return mcp.NewDeepSeekClient() // return a typed client so caller gets a clear API error
}

// clientForProvider returns the appropriate provider-specific client.
// Each constructor sets correct default base URL and model for that provider.
func clientForProvider(provider string) mcp.AIClient {
	switch provider {
	case "openai":
		return mcp.NewOpenAIClient()
	case "deepseek":
		return mcp.NewDeepSeekClient()
	case "claude":
		return mcp.NewClaudeClient()
	case "qwen":
		return mcp.NewQwenClient()
	case "kimi":
		return mcp.NewKimiClient()
	case "grok":
		return mcp.NewGrokClient()
	case "gemini":
		return mcp.NewGeminiClient()
	default:
		// Unknown/custom provider: fall back to OpenAI-compatible format.
		return mcp.NewDeepSeekClient()
	}
}

// ── Language selection ────────────────────────────────────────────────────────

// langSelectionMsg is always bilingual so it works before a language is chosen.
func langSelectionMsg() string {
	return `🌐 *Please select your language / 请选择语言*

1️⃣  English
2️⃣  中文

Reply with 1 or 2 / 发送 1 或 2`
}

// parseLangChoice returns "en", "zh", or "" (unrecognised).
func parseLangChoice(text string) string {
	switch strings.TrimSpace(text) {
	case "1", "English", "english", "en", "EN":
		return "en"
	case "2", "中文", "zh", "ZH", "chinese", "Chinese":
		return "zh"
	}
	return ""
}

// isLangDefault returns true if the user has never explicitly picked a language
// (i.e. the stored value is empty — the "en" default from GetLanguage() is a fallback).
func isLangDefault(st *store.Store) bool {
	cfg, err := st.TelegramConfig().Get()
	if err != nil {
		return true
	}
	return cfg.Language == ""
}

// ── Setup guide ───────────────────────────────────────────────────────────────

// buildSetupGuide returns a context-aware onboarding message in the chosen language.
func buildSetupGuide(st *store.Store, userID string, apiPort int, botToken string, lang string) string {
	// Step 1: AI model configured?
	if _, err := st.AIModel().GetDefault(userID); err != nil {
		if lang == "zh" {
			return `🤖 *NOFX 个人 AI 交易助手*

欢迎！开始前需要配置 AI 模型。

*第一步：配置 AI 模型*

发送以下格式（选一个你有账号的服务商）：

` + "```" + `
配置 deepseek  你的API-Key
配置 openai    你的API-Key
配置 claude    你的API-Key
配置 qwen      你的API-Key
配置 kimi      你的API-Key
配置 grok      你的API-Key
配置 gemini    你的API-Key
` + "```" + `

*推荐*：DeepSeek 价格低、效果好
获取 Key：https://platform.deepseek.com/api_keys

发送 /lang 切换语言`
		}
		return `🤖 *NOFX Personal AI Trading Bot*

Welcome! You need to configure an AI model before trading.

*Step 1: Configure AI Model*

Send a message in this format (pick a provider you have access to):

` + "```" + `
configure deepseek  your-api-key
configure openai    your-api-key
configure claude    your-api-key
configure qwen      your-api-key
configure kimi      your-api-key
configure grok      your-api-key
configure gemini    your-api-key
` + "```" + `

*Recommended*: DeepSeek — low cost, great performance
Get your key: https://platform.deepseek.com/api_keys

Send /lang to change language`
	}

	// Step 2: Exchange configured?
	exchanges, _ := st.Exchange().List(userID)
	hasEnabled := false
	for _, e := range exchanges {
		if e.Enabled {
			hasEnabled = true
			break
		}
	}
	if !hasEnabled {
		if lang == "zh" {
			return `✅ AI 模型已配置！

*第二步：配置交易所*

直接发消息告诉我交易所信息，例如：

_"帮我配置 OKX，API Key 是 xxx，Secret 是 xxx，Passphrase 是 xxx"_
_"帮我配置 Binance，API Key 是 xxx，Secret Key 是 xxx"_
_"帮我配置 Bybit，API Key 是 xxx，Secret Key 是 xxx"_

去交易所官网 → 账户设置 → API 管理 → 新建（需开启合约交易权限）`
		}
		return `✅ AI model configured!

*Step 2: Configure Exchange*

Just tell me your exchange credentials, for example:

_"Configure OKX, API Key is xxx, Secret is xxx, Passphrase is xxx"_
_"Configure Binance, API Key is xxx, Secret Key is xxx"_
_"Configure Bybit, API Key is xxx, Secret Key is xxx"_

Go to your exchange → Account → API Management → Create Key (enable futures/contract trading)`
	}

	// All configured
	if lang == "zh" {
		return `✅ *NOFX 交易助手已就绪*

直接发消息告诉我你要做什么：

*查询*
_"查看我的持仓"_、_"查看账户余额"_

*创建并启动交易*
_"帮我创建一个 BTC 趋势策略并跑起来"_
_"创建保守型策略，只交易 BTC 和 ETH"_

*控制*
_"启动交易员"_、_"暂停交易员"_

/start 重置对话 | /help 帮助 | /lang 切换语言`
	}
	return `✅ *NOFX Trading Bot Ready*

Just tell me what you want to do:

*Query*
_"Show my positions"_, _"Show account balance"_

*Create & Start Trading*
_"Create a BTC trend strategy and start it"_
_"Create a conservative strategy, BTC and ETH only"_

*Control*
_"Start trader"_, _"Stop trader"_

/start reset session | /help | /lang change language`
}

// ── Direct setup commands (no LLM required) ───────────────────────────────────

// tryHandleSetupCommand intercepts "configure/配置 <provider> <key>" messages
// and calls PUT /api/models directly — no LLM needed, works during bootstrapping.
func tryHandleSetupCommand(text string, apiPort int, botToken string, st *store.Store, userID string, lang string) (string, bool) {
	text = strings.TrimSpace(text)
	lower := strings.ToLower(text)
	if !strings.HasPrefix(text, "配置 ") && !strings.HasPrefix(lower, "configure ") {
		return "", false
	}

	parts := strings.Fields(text)
	if len(parts) < 3 {
		if lang == "zh" {
			return "格式：配置 <服务商> <API-Key>\n例如：配置 deepseek sk-xxxxxxxxx", true
		}
		return "Format: configure <provider> <api-key>\nExample: configure deepseek sk-xxxxxxxxx", true
	}

	provider := strings.ToLower(parts[1])
	apiKey := parts[2]

	validProviders := map[string]bool{
		"openai": true, "deepseek": true, "claude": true,
		"qwen": true, "kimi": true, "grok": true, "gemini": true,
	}
	if !validProviders[provider] {
		if lang == "zh" {
			return fmt.Sprintf("不支持的服务商：%s\n支持：openai / deepseek / claude / qwen / kimi / grok / gemini", provider), true
		}
		return fmt.Sprintf("Unknown provider: %s\nSupported: openai / deepseek / claude / qwen / kimi / grok / gemini", provider), true
	}

	body, _ := json.Marshal(map[string]any{
		"models": map[string]any{
			provider: map[string]any{"enabled": true, "api_key": apiKey},
		},
	})
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://127.0.0.1:%d/api/models", apiPort), bytes.NewReader(body))
	if err != nil {
		if lang == "zh" {
			return "配置请求失败，请稍后重试", true
		}
		return "Failed to create request, please try again", true
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		if lang == "zh" {
			return "无法连接服务，请确认服务正常运行", true
		}
		return "Cannot reach service, please check it is running", true
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("Error %d: %s", resp.StatusCode, string(respBody)), true
	}

	logger.Infof("Bot: setup command configured provider=%s", provider)
	if lang == "zh" {
		return fmt.Sprintf("✅ %s 配置成功！\n\n发送 /start 查看下一步", provider), true
	}
	return fmt.Sprintf("✅ %s configured successfully!\n\nSend /start to see the next step", provider), true
}

// ── Help message ──────────────────────────────────────────────────────────────

func helpMessage(lang string) string {
	if lang == "zh" {
		return `*NOFX 使用指南*

*查询*
- "查看我的持仓"
- "查看账户余额"
- "列出我的交易员"

*控制*
- "启动交易员"
- "暂停 xxx 交易员"

*创建策略*
- "帮我创建 BTC 趋势策略并跑起来"
- "创建保守型策略，BTC ETH，止损 8%"

*直接配置（不需要 AI）*
- 配置 deepseek sk-xxxx
- 配置 openai sk-xxxx

*命令*
/start - 重置对话 / 查看配置状态
/lang  - 切换语言
/help  - 显示此帮助`
	}
	return `*NOFX Help*

*Query*
- "Show my positions"
- "Show account balance"
- "List my traders"

*Control*
- "Start trader"
- "Stop trader [name]"

*Create strategy*
- "Create a BTC trend strategy and start it"
- "Create a conservative strategy, BTC and ETH, 8% stop loss"

*Direct setup (no AI needed)*
- configure deepseek sk-xxxx
- configure openai sk-xxxx

*Commands*
/start - Reset session / check setup status
/lang  - Change language
/help  - Show this help`
}
