package news

import (
		"context"
		"fmt"
		"log"
		"sort"
		"strconv"
		"strings"
		"time"
)
// Service 新闻服务
type Service struct {
        store         StateStore
        fetcher       Fetcher
        notifier      Notifier
        aiProcessor   AIProcessor
        enabled       bool
        sentArticleIDs map[int64]bool // 全局消息ID去重集合：防止同一条新闻在不同分类重复发送
}

// NewService 创建新闻服务
func NewService(store StateStore) *Service {
        // 从数据库获取配置
        // 注意：这里假设配置已经加载到数据库中。在实际运行时，Start() 会被调用。
        // 更好的做法是在 Start 内部获取配置，支持动态更新。
        return &Service{
                store:          store,
                sentArticleIDs: make(map[int64]bool),
        }
}

// Start 启动新闻服务
func (s *Service) Start(ctx context.Context) {
        log.Println("📰 正在启动金融新闻推送服务...")

        // 初始配置加载
        if err := s.loadConfig(); err != nil {
                log.Printf("❌ 新闻服务配置加载失败: %v", err)
                return
        }

        if !s.enabled {
                log.Println("🔕 新闻推送服务未启用 (telegram_news_enabled=false)")
                return
        }

        // 立即执行一次
        s.processAllCategories()

        // 设置定时器 (每5分钟)
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()

        for {
                select {
                case <-ctx.Done():
                        log.Println("🛑 新闻服务已停止")
                        return
                case <-ticker.C:
                        // 重新加载配置（允许动态开启/关闭）
                        s.loadConfig()
                        if s.enabled {
                                s.processAllCategories()
                        }
                }
        }
}

// loadConfig 加载配置
func (s *Service) loadConfig() error {
        enabledStr, _ := s.store.GetSystemConfig("telegram_news_enabled")
        s.enabled = enabledStr == "true"

        if !s.enabled {
                return nil
        }

        apiKey, _ := s.store.GetSystemConfig("finnhub_api_key")
        botToken, _ := s.store.GetSystemConfig("telegram_bot_token")
        chatID, _ := s.store.GetSystemConfig("telegram_chat_id")
        
        // DeepSeek Config
        deepseekKey, _ := s.store.GetSystemConfig("deepseek_api_key")
        deepseekURL, _ := s.store.GetSystemConfig("deepseek_api_url")
        targetLang, _ := s.store.GetSystemConfig("news_language")
        if targetLang == "" {
                targetLang = "zh-CN"
        }

        if apiKey == "" || botToken == "" || chatID == "" {
                return fmt.Errorf("缺少必要的API配置")
        }

        // 仅当依赖未初始化或配置变更时重新创建
        // 这里简化处理：总是使用最新配置创建
        s.fetcher = NewFinnhubFetcher(apiKey)
        s.notifier = NewTelegramNotifier(botToken, chatID)
        
        if deepseekKey != "" {
                s.aiProcessor = NewDeepSeekProcessor(deepseekKey, deepseekURL, targetLang)
        } else {
                s.aiProcessor = nil
        }

        return nil
}

func (s *Service) processAllCategories() {
        // 每个周期开始时，清空上个周期的已发送消息ID记录
        // 这样可以允许隔离的消息跨周期重新发送，但在同一周期内防止重复
        s.sentArticleIDs = make(map[int64]bool)

        categories := []string{"crypto", "general"}
        for _, cat := range categories {
                if err := s.ProcessCategory(cat); err != nil {
                        log.Printf("⚠️ 处理新闻分类 %s 失败: %v", cat, err)
                }
        }
}

// ProcessCategory 处理特定分类的新闻 (Public for testing)
func (s *Service) ProcessCategory(category string) error {
        // 1. 获取新闻
        articles, err := s.fetcher.FetchNews(category)
        if err != nil {
                return err
        }

        if len(articles) == 0 {
                return nil
        }

        // 2. 获取上次状态
        lastID, lastTime, err := s.store.GetNewsState(category)
        if err != nil {
                return fmt.Errorf("获取状态失败: %w", err)
        }

        // 3. 过滤和排序
        var newArticles []Article

        for _, a := range articles {
                // 基础去重：按分类时间戳
                if int64(a.ID) <= lastID || a.Datetime <= lastTime {
                        continue
                }

                // 新增：全局消息ID去重 - 防止同一条新闻在不同分类重复发送
                // 在同一周期内，如果这条新闻已经在其他分类发送过，就跳过
                if s.sentArticleIDs[int64(a.ID)] {
                        log.Printf("ℹ️ 消息ID %d 已在其他分类发送，跳过重复: %s", a.ID, a.Headline)
                        continue
                }

                newArticles = append(newArticles, a)
        }

        // 按时间升序排序（旧 -> 新）
        sort.Slice(newArticles, func(i, j int) bool {
                return newArticles[i].Datetime < newArticles[j].Datetime
        })

        // 4. 处理、发送并更新状态
        for i := range newArticles {
                // 使用指针以便修改内容
                a := &newArticles[i]

                // AI 处理 (Fail-Open: 如果失败，仅记录日志，继续发送原始新闻)
                if s.aiProcessor != nil {
                        log.Printf("🤖 AI 正在处理新闻: %s", a.Headline)
                        if err := s.aiProcessor.Process(a); err != nil {
                                log.Printf("⚠️ AI 处理失败 (降级发送原版): %v", err)
                                a.AIProcessed = false
                        }
                }

                msg := formatMessage(*a)

                // 从配置中读取 message_thread_id
                threadIDStr, _ := s.store.GetSystemConfig("telegram_message_thread_id")
                threadID, err := strconv.Atoi(threadIDStr)
                if err != nil {
                        log.Printf("⚠️ 无法解析 Telegram 消息话题 ID (%s)，使用默认 0: %v", threadIDStr, err)
                        threadID = 0 // Fallback to 0 if parsing fails
                }

                if err := s.notifier.Send(msg, threadID); err != nil {
                        log.Printf("❌ 发送Telegram消息失败: %v", err)
                        continue // 继续尝试下一条
                }

                // 记录已发送的消息ID，防止在其他分类重复发送
                s.sentArticleIDs[int64(a.ID)] = true

                // 立即更新状态
                if err := s.store.UpdateNewsState(category, int64(a.ID), a.Datetime); err != nil {
                        log.Printf("⚠️ 更新新闻状态失败: %v", err)
                }

                log.Printf("📢 已推送新闻: [%s] %s", category, a.Headline)
                time.Sleep(2 * time.Second) // 限流保护
        }

        return nil
}
func formatMessage(a Article) string {
	// 将Unix时间戳转换为可读时间
	t := time.Unix(a.Datetime, 0)
	timeStr := t.Format("15:04") // 只显示时间，更紧凑

	var icon string
	if a.Category == "crypto" {
		icon = "🪙"
	} else {
		icon = "📰"
	}
        
        // AI 增强版格式
        if a.AIProcessed {
                sentimentIcon := ""
                switch a.Sentiment {
                case "POSITIVE": sentimentIcon = "🟢"
                case "NEGATIVE": sentimentIcon = "🔴"
                default: sentimentIcon = "⚪"
                }
                
                return fmt.Sprintf("<b>%s %s %s</b>\n\n📅 %s | #%s\n\n📝 <b>摘要</b>: %s\n\n---------------\n原文: <a href=\" %s \">%s</a>",
                        icon, a.TranslatedHeadline, sentimentIcon, 
                        timeStr, strings.ToUpper(a.Category), 
                        a.TranslatedSummary, 
                        a.URL, a.Headline)
        }

	// 原始格式 (降级)
	// 转义 HTML 特殊字符
	headline := strings.ReplaceAll(a.Headline, "<", "&lt;")
	headline = strings.ReplaceAll(headline, ">", "&gt;")
	summary := strings.ReplaceAll(a.Summary, "<", "&lt;")
	summary = strings.ReplaceAll(summary, ">", "&gt;")

	return fmt.Sprintf("<b>%s %s</b>\n\n📅 %s | #%s\n\n%s\n\n🔗 <a href=\" %s \">Read More</a>",
			icon, headline, timeStr, strings.ToUpper(a.Category), summary, a.URL)
}