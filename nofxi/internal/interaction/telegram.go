package interaction

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MessageHandler is called for each incoming message.
type MessageHandler func(ctx context.Context, userID int64, text string) (string, error)

// TelegramBot wraps the Telegram Bot API.
type TelegramBot struct {
	bot        *tgbotapi.BotAPI
	handler    MessageHandler
	allowedIDs map[int64]bool // Empty = allow all
	logger     *slog.Logger
}

// NewTelegramBot creates a new Telegram bot.
func NewTelegramBot(token string, allowedIDs []int64, handler MessageHandler, logger *slog.Logger) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	allowed := make(map[int64]bool)
	for _, id := range allowedIDs {
		allowed[id] = true
	}

	logger.Info("telegram bot authorized", "username", bot.Self.UserName)

	return &TelegramBot{
		bot:        bot,
		handler:    handler,
		allowedIDs: allowed,
		logger:     logger,
	}, nil
}

// Start begins polling for updates. Blocks until context is cancelled.
func (t *TelegramBot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)

	t.logger.Info("telegram bot started, listening for messages...")

	for {
		select {
		case <-ctx.Done():
			t.bot.StopReceivingUpdates()
			t.logger.Info("telegram bot stopped")
			return nil
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			go t.handleUpdate(ctx, update)
		}
	}
}

func (t *TelegramBot) handleUpdate(parentCtx context.Context, update tgbotapi.Update) {
	msg := update.Message
	userID := msg.From.ID

	// Access control
	if len(t.allowedIDs) > 0 && !t.allowedIDs[userID] {
		t.logger.Warn("unauthorized user", "user_id", userID, "username", msg.From.UserName)
		t.sendText(msg.Chat.ID, "⛔ Unauthorized. Contact the admin to get access.")
		return
	}

	text := msg.Text
	if text == "" {
		return
	}

	t.logger.Info("received message",
		"user_id", userID,
		"username", msg.From.UserName,
		"text", text,
	)

	// Send "typing" indicator
	typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
	t.bot.Send(typing)

	// Process message with timeout
	ctx, cancel := context.WithTimeout(parentCtx, 55*time.Second)
	defer cancel()

	response, err := t.handler(ctx, userID, text)
	if err != nil {
		t.logger.Error("handle message", "error", err)
		if ctx.Err() != nil {
			response = "⏱️ AI 响应超时，请稍后再试。"
		} else {
			response = fmt.Sprintf("⚠️ Error: %v", err)
		}
	}

	t.logger.Info("sending response", "user_id", userID, "len", len(response))
	t.sendMarkdown(msg.Chat.ID, response)
}

func (t *TelegramBot) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := t.bot.Send(msg); err != nil {
		t.logger.Error("send message", "error", err)
	}
}

func (t *TelegramBot) sendMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown

	if _, err := t.bot.Send(msg); err != nil {
		// Retry without markdown if parsing fails
		t.logger.Warn("markdown send failed, retrying plain", "error", err)
		t.sendText(chatID, text)
	}
}

// SendNotification sends a proactive message to a user.
func (t *TelegramBot) SendNotification(userID int64, text string) error {
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err := t.bot.Send(msg)
	return err
}
