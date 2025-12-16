package news

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const telegramBaseURL = "https://api.telegram.org/bot%s/sendMessage"

// TelegramNotifier 实现 Notifier 接口
type TelegramNotifier struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramNotifier 创建 Telegram 通知器
func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Send 发送消息到 Telegram
func (t *TelegramNotifier) Send(msg string, messageThreadID int) error {
	url := fmt.Sprintf(telegramBaseURL, t.botToken)

	payload := map[string]interface{}{
		"chat_id":                  t.chatID,
		"text":                     msg,
		"parse_mode":               "HTML", // 使用 HTML 模式支持加粗等格式
		"disable_web_page_preview": false,
	}

	// 如果指定了 Topic ID，则添加到参数中
	if messageThreadID > 0 {
		payload["message_thread_id"] = messageThreadID
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := t.client.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 读取错误信息
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("telegram api error: %s - %s", resp.Status, buf.String())
	}

	return nil
}
