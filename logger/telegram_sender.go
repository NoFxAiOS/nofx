package logger

import (
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramSender Telegram消息发送器（异步）
type TelegramSender struct {
	bot           *tgbotapi.BotAPI
	chatID        int64
	msgChan       chan string
	retryCount    int
	retryInterval time.Duration
	wg            sync.WaitGroup
	stopChan      chan struct{}
	once          sync.Once
}

// NewTelegramSender 创建Telegram发送器（使用默认参数）
func NewTelegramSender(botToken string, chatID int64) (*TelegramSender, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("创建telegram bot失败: %w", err)
	}

	// 设置为静默模式（不打印bot信息）
	bot.Debug = false

	sender := &TelegramSender{
		bot:           bot,
		chatID:        chatID,
		msgChan:       make(chan string, 20), // 固定缓冲区大小: 20
		retryCount:    3,                     // 固定重试次数: 3
		retryInterval: 3 * time.Second,       // 固定重试间隔: 3秒
		stopChan:      make(chan struct{}),
	}

	// 启动异步发送协程
	sender.Start()

	return sender, nil
}

// Start 启动异步发送协程
func (s *TelegramSender) Start() {
	s.wg.Add(1)
	go s.listenAndSend()
}

// SendAsync 异步发送消息（非阻塞）
func (s *TelegramSender) SendAsync(message string) {
	select {
	case s.msgChan <- message:
		// 成功写入缓冲区
	default:
		// 缓冲区满，丢弃消息（不阻塞主流程）
		fmt.Printf("[Telegram] 消息缓冲区已满，消息被丢弃\n")
	}
}

// listenAndSend 监听channel并发送消息
func (s *TelegramSender) listenAndSend() {
	defer s.wg.Done()

	for {
		select {
		case msg := <-s.msgChan:
			s.sendWithRetry(msg)
		case <-s.stopChan:
			// 清空缓冲区后退出
			for len(s.msgChan) > 0 {
				msg := <-s.msgChan
				s.sendWithRetry(msg)
			}
			return
		}
	}
}

// sendWithRetry 发送消息（带重试）
func (s *TelegramSender) sendWithRetry(message string) {
	var err error
	for i := 0; i < s.retryCount; i++ {
		err = s.send(message)
		if err == nil {
			return // 发送成功
		}

		// 重试前等待
		if i < s.retryCount-1 {
			time.Sleep(s.retryInterval)
		}
	}

	// 所有重试都失败
	if err != nil {
		fmt.Printf("[Telegram] 发送消息失败（已重试%d次）: %v\n", s.retryCount, err)
	}
}

// send 发送单条消息
func (s *TelegramSender) send(message string) error {
	msg := tgbotapi.NewMessage(s.chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown

	_, err := s.bot.Send(msg)
	return err
}

// Stop 停止发送器（优雅关闭）
func (s *TelegramSender) Stop() {
	s.once.Do(func() {
		close(s.stopChan)
		s.wg.Wait()
	})
}
