package telegram

import (
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
func Start(cfg *config.Config, st *store.Store, reloadCh <-chan struct{}) {
	for {
		token := resolveToken(cfg, st)
		if token == "" {
			logger.Info("Telegram bot disabled (no token configured), waiting for reload signal...")
			<-reloadCh
			continue
		}

		stopped := runBot(token, cfg, st)
		if !stopped {
			return
		}

		select {
		case <-reloadCh:
			logger.Info("Reloading Telegram bot with new token...")
		}
	}
}

// resolveToken returns the bot token from DB (configured via Web UI).
func resolveToken(cfg *config.Config, st *store.Store) string {
	dbCfg, err := st.TelegramConfig().Get()
	if err == nil && dbCfg.BotToken != "" {
		return dbCfg.BotToken
	}
	return ""
}

// runBot runs the bot until the updates channel closes (clean stop → true) or a fatal error (false).
func runBot(token string, cfg *config.Config, st *store.Store) bool {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logger.Errorf("Telegram bot failed to start: %v", err)
		return false
	}
	logger.Infof("Telegram bot @%s started", bot.Self.UserName)

	// Allowed chat ID: read from DB binding (0 = unbound, first /start will bind).
	allowedChatID := int64(0)
	if id, err := st.TelegramConfig().GetBoundChatID(); err == nil && id != 0 {
		allowedChatID = id
	}

	// botUserID / botToken / agents are resolved lazily and refresh when user registers.
	var (
		botUserID    string
		botUserEmail string
		botToken     string
		agents       *agent.Manager
	)

	resolveBotUser := func() bool {
		users, err := st.User().GetAll()
		if err != nil || len(users) == 0 {
			return false
		}
		u := users[0]
		if u.ID == botUserID {
			return true
		}
		newToken, err := agent.GenerateBotToken(u.ID)
		if err != nil {
			logger.Errorf("Failed to generate bot JWT for user %s: %v", u.ID, err)
			return false
		}
		prev := botUserID
		botUserID = u.ID
		botUserEmail = u.Email
		botToken = newToken
		agents = agent.NewManager(cfg.APIServerPort, botToken, botUserEmail, botUserID,
			func() mcp.AIClient { return newLLMClient(st, botUserID) },
			api.GetAPIDocs(),
		)
		if prev == "" {
			logger.Infof("Bot: resolved user %s (%s)", botUserID, botUserEmail)
		} else {
			logger.Infof("Bot: user changed → %s (%s)", botUserID, botUserEmail)
		}
		return true
	}
	resolveBotUser()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// awaitingLang is set only when the user explicitly runs /lang.
	awaitingLang := false

	for update := range updates {
		if update.Message == nil {
			continue
		}
		chatID := update.Message.Chat.ID
		text := strings.TrimSpace(update.Message.Text)

		// ── Language selection (triggered only by /lang) ──────────────────────
		if awaitingLang && chatID == allowedChatID {
			if lang := parseLangChoice(text); lang != "" {
				awaitingLang = false
				st.TelegramConfig().SetLanguage(lang) //nolint:errcheck
				sendMarkdownMsg(bot, chatID, statusMsg(st, botUserID, cfg.APIServerPort, lang))
			} else {
				sendMarkdownMsg(bot, chatID, langMenuMsg())
			}
			continue
		}

		// ── /start ────────────────────────────────────────────────────────────
		if text == "/start" {
			resolveBotUser()
			if botUserID == "" {
				sendMsg(bot, chatID,
					"No account found.\nOpen the web dashboard to register, then send /start.")
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
				sendMsg(bot, chatID, "This bot is already bound to another account.")
				continue
			} else {
				agents.Reset(chatID)
			}
			lang := st.TelegramConfig().GetLanguage()
			sendMarkdownMsg(bot, chatID, statusMsg(st, botUserID, cfg.APIServerPort, lang))
			continue
		}

		// ── /lang ─────────────────────────────────────────────────────────────
		if text == "/lang" {
			awaitingLang = true
			sendMarkdownMsg(bot, chatID, langMenuMsg())
			continue
		}

		// ── /help ─────────────────────────────────────────────────────────────
		if text == "/help" {
			lang := st.TelegramConfig().GetLanguage()
			sendMarkdownMsg(bot, chatID, helpMsg(lang))
			continue
		}

		// ── Access control ────────────────────────────────────────────────────
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

		// ── Refresh user before every AI call ────────────────────────────────
		resolveBotUser()
		if botUserID == "" {
			sendMsg(bot, chatID, "No account found. Open the web dashboard to register.")
			continue
		}

		lang := st.TelegramConfig().GetLanguage()

		// ── Guard: show status if not ready for trading ───────────────────────
		if newLLMClient(st, botUserID) == nil {
			sendMarkdownMsg(bot, chatID, statusMsg(st, botUserID, cfg.APIServerPort, lang))
			continue
		}

		// ── AI agent ─────────────────────────────────────────────────────────
		go func(chatID int64, text string) {
			sent, err := bot.Send(tgbotapi.NewMessage(chatID, "⏳"))
			placeholderID := 0
			if err == nil {
				placeholderID = sent.MessageID
			}

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
				if accumulated != "⏳" && time.Since(lastEdit) < time.Second {
					return
				}
				lastEdit = time.Now()
				edit := tgbotapi.NewEditMessageText(chatID, placeholderID, accumulated)
				bot.Send(edit) //nolint:errcheck
			}

			reply := agents.Run(chatID, text, onChunk)

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

	return true
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func sendMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg) //nolint:errcheck
}

func sendMarkdownMsg(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := bot.Send(msg); err != nil {
		plain := tgbotapi.NewMessage(chatID, text)
		bot.Send(plain) //nolint:errcheck
	}
}

// ── LLM client ───────────────────────────────────────────────────────────────

func newLLMClient(st *store.Store, userID string) mcp.AIClient {
	// 1. Prefer the model explicitly configured for Telegram (Settings → Telegram → AI Model)
	if tgCfg, err := st.TelegramConfig().Get(); err == nil && tgCfg.ModelID != "" {
		if model, err := st.AIModel().Get(userID, tgCfg.ModelID); err == nil && model.Enabled {
			apiKey := string(model.APIKey)
			if apiKey != "" {
				client := clientForProvider(model.Provider)
				client.SetAPIKey(apiKey, model.CustomAPIURL, model.CustomModelName)
				if isUSDCProvider(model.Provider) {
					logger.Infof("Telegram agent: provider=%s (USDC payment) user=%s", model.Provider, userID)
				} else {
					logger.Infof("Telegram agent: provider=%s user=%s", model.Provider, userID)
				}
				return client
			}
		}
	}

	// 2. Fall back to first enabled model
	if model, err := st.AIModel().GetDefault(userID); err == nil {
		apiKey := string(model.APIKey)
		if apiKey != "" {
			client := clientForProvider(model.Provider)
			client.SetAPIKey(apiKey, model.CustomAPIURL, model.CustomModelName)
			if isUSDCProvider(model.Provider) {
				logger.Infof("Telegram agent: provider=%s (USDC payment) user=%s", model.Provider, userID)
			} else {
				logger.Infof("Telegram agent: provider=%s user=%s", model.Provider, userID)
			}
			return client
		}
	}

	// 3. Environment variable fallback
	for _, pair := range []struct{ provider, key, url string }{
		{"deepseek", os.Getenv("DEEPSEEK_API_KEY"), mcp.DefaultDeepSeekBaseURL},
		{"openai", os.Getenv("OPENAI_API_KEY"), ""},
		{"claude", os.Getenv("ANTHROPIC_API_KEY"), ""},
	} {
		if pair.key != "" {
			client := clientForProvider(pair.provider)
			client.SetAPIKey(pair.key, pair.url, "")
			return client
		}
	}
	return nil
}

// isUSDCProvider returns true for providers that pay per call with USDC (x402 protocol).
func isUSDCProvider(provider string) bool {
	return provider == "blockrun-base" || provider == "blockrun-sol" || provider == "claw402"
}

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
	case "minimax":
		return mcp.NewMiniMaxClient()
	case "blockrun-base":
		return mcp.NewBlockRunBaseClient()
	case "blockrun-sol":
		return mcp.NewBlockRunSolClient()
	case "claw402":
		return mcp.NewClaw402Client()
	default:
		return mcp.NewDeepSeekClient()
	}
}

// ── Status message ────────────────────────────────────────────────────────────

// statusMsg is the single entry-point message shown after /start.
// It checks what's configured and shows either a setup prompt or the ready state.
func statusMsg(st *store.Store, userID string, apiPort int, lang string) string {
	webURL := "http://localhost:3000"

	// Determine what's missing.
	hasModel := false
	if _, err := st.AIModel().GetDefault(userID); err == nil {
		hasModel = true
	}

	hasExchange := false
	if exchanges, err := st.Exchange().List(userID); err == nil {
		for _, e := range exchanges {
			if e.Enabled {
				hasExchange = true
				break
			}
		}
	}

	if !hasModel || !hasExchange {
		missing := ""
		if lang == "zh" {
			if !hasModel {
				missing += "\n❌ AI 模型 → 设置 → AI 模型 → 添加"
			}
			if !hasExchange {
				missing += "\n❌ 交易所 → 设置 → 交易所 → 添加"
			}
			return "⚙️ *需要完成初始配置*\n\n打开 Web 管理界面完成配置：\n→ " + webURL + "\n" + missing + "\n\n配置完成后发送 /start"
		}
		if !hasModel {
			missing += "\n❌ AI Model → Settings → AI Models → Add"
		}
		if !hasExchange {
			missing += "\n❌ Exchange → Settings → Exchanges → Add"
		}
		return "⚙️ *Setup required*\n\nOpen the web dashboard to complete setup:\n→ " + webURL + "\n" + missing + "\n\nSend /start when done."
	}

	// All configured — show ready state.
	if lang == "zh" {
		return `✅ *NOFX 就绪，开始交易吧！*

直接告诉我你想做什么：

📊 "查看我的持仓"
💰 "账户余额多少"
🤖 "帮我创建 BTC 趋势策略并启动"
⏹ "停止所有交易员"

/help 查看更多 · /lang 切换语言`
	}
	return `✅ *NOFX is ready!*

Just tell me what you want:

📊 "Show my positions"
💰 "What's my balance?"
🤖 "Create a BTC trend strategy and start it"
⏹ "Stop all traders"

/help for more · /lang to change language`
}

// ── Language ──────────────────────────────────────────────────────────────────

func langMenuMsg() string {
	return "🌐 *Choose your language*\n\n1 — English\n2 — 中文\n\nReply with 1 or 2"
}

func parseLangChoice(text string) string {
	switch strings.TrimSpace(text) {
	case "1", "en", "EN", "English", "english":
		return "en"
	case "2", "zh", "ZH", "中文", "chinese", "Chinese":
		return "zh"
	}
	return ""
}

// ── Help ──────────────────────────────────────────────────────────────────────

func helpMsg(lang string) string {
	if lang == "zh" {
		return `*NOFX 使用指南*

*查询*
• "查看我的持仓"
• "账户余额多少"
• "列出我的交易员"

*创建 & 启动*
• "帮我创建 BTC 趋势策略并跑起来"
• "保守型策略，只交易 BTC 和 ETH"

*控制*
• "启动交易员"
• "暂停交易员"
• "停止所有交易"

*命令*
/start — 刷新状态
/lang  — 切换语言
/help  — 帮助`
	}
	return `*NOFX Help*

*Query*
• "Show my positions"
• "What's my balance?"
• "List my traders"

*Create & start*
• "Create a BTC trend strategy and start it"
• "Conservative strategy, BTC and ETH only"

*Control*
• "Start trader"
• "Stop trader"
• "Stop all trading"

*Commands*
/start — refresh status
/lang  — change language
/help  — show this`
}
