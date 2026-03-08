package telegram

import (
	"nofx/api"
	"nofx/config"
	"nofx/logger"
	"nofx/mcp"
	"nofx/store"
	"nofx/telegram/agent"
	"os"
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

	// Resolve the real user ID: use the first registered user so that bot-made
	// changes (model/exchange configs) are visible in the frontend under that user.
	// Falls back to "default" if no users exist yet (fresh install).
	botUserID := "default"
	if ids, err := st.User().GetAllIDs(); err == nil && len(ids) > 0 {
		botUserID = ids[0]
	}

	// Generate a bot JWT for authenticated API calls. Re-generated on each bot start.
	botToken, err := agent.GenerateBotToken(botUserID)
	if err != nil {
		logger.Errorf("Failed to generate bot JWT: %v", err)
		return false
	}

	// Wire the AI agent manager. API docs are auto-generated from registered routes.
	agents := agent.NewManager(cfg.APIServerPort, botToken, botUserID,
		func() mcp.AIClient { return newLLMClient(st) },
		api.GetAPIDocs(),
	)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		chatID := update.Message.Chat.ID
		text := update.Message.Text

		// Handle /start: auto-bind or welcome
		if text == "/start" {
			if allowedChatID == 0 {
				// First user to /start becomes the bound admin
				username := update.Message.From.UserName
				if err := st.TelegramConfig().BindUser(chatID, "@"+username); err != nil {
					logger.Errorf("Failed to bind Telegram user: %v", err)
					sendMsg(bot, chatID, "Binding failed, please check server logs.")
					continue
				}
				allowedChatID = chatID
				logger.Infof("Telegram bound to @%s (chatID: %d)", username, chatID)
				sendMsg(bot, chatID, "Bound successfully! "+welcomeMessage())
			} else if chatID == allowedChatID {
				// Already bound, same user: reset session and show welcome
				agents.Reset(chatID)
				sendMsg(bot, chatID, welcomeMessage())
			} else {
				sendMsg(bot, chatID, "This bot is already bound to another user.")
			}
			continue
		}

		// Handle /help
		if text == "/help" {
			sendMsg(bot, chatID, helpMessage())
			continue
		}

		// Access control
		if allowedChatID != 0 && chatID != allowedChatID {
			sendMsg(bot, chatID, "Unauthorized access.")
			continue
		}
		if allowedChatID == 0 {
			sendMsg(bot, chatID, "Please send /start to bind your account first.")
			continue
		}

		if text == "" {
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

// newLLMClient builds an LLM client for the agent.
// Priority: DB-configured model (Web UI) > environment variables.
// Uses provider-specific constructors to ensure correct default base URLs and models.
func newLLMClient(st *store.Store) mcp.AIClient {
	// 1. Try any enabled model from DB (user configured via Web UI, any user_id)
	if model, err := st.AIModel().GetAnyEnabled(); err == nil {
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
		logger.Warnf("Telegram: no enabled model in DB (%v), trying env vars", err)
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
		// Anthropic Messages API — different wire format (x-api-key, input_schema, tool_use blocks).
		return mcp.NewClaudeClient()
	default:
		// Qwen, Kimi, Grok, Gemini, custom: OpenAI-compatible APIs.
		// CustomAPIURL and CustomModelName are required for these providers.
		return mcp.NewDeepSeekClient()
	}
}

func welcomeMessage() string {
	return `*NOFX Trading Assistant Connected!*

You can manage your trading system with natural language:

*Query*
- Show current positions
- Show account balance

*Control*
- Start trader
- Stop trader

*Configure*
- Create a BTC strategy with 8% stop loss
- Configure Binance exchange API
- Add DeepSeek AI model
- Update strategy prompt

Send /help for detailed help
Send /start to reset session`
}

func helpMessage() string {
	return `*NOFX Trading Assistant Guide*

*Query examples:*
- "Show current positions"
- "Show account balance"
- "List my traders"

*Control examples:*
- "Start trader"
- "Stop trader [name]"

*Configure examples:*
- "Create a BTC strategy with RSI+MACD, 8% stop loss, 20% max position"
- "Configure Binance exchange, API Key is xxx, Secret is xxx"
- "Add DeepSeek model, Key is xxx"
- "Update strategy prompt for my main strategy to: you are a conservative trader..."

*Other commands:*
- /start - Reset current session
- /help - Show this help

You can use natural language — no need to memorize specific command formats.`
}
