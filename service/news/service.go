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
	store          StateStore
	fetchers       []Fetcher      // 支持多个 Fetcher
	topicRouter    map[string]int // 路由表: Source Name -> Telegram Topic ID
	notifier       Notifier
	enabled        bool
	sentArticleIDs map[int64]bool // 全局消息ID去重集合
}

// NewService 创建新闻服务
func NewService(store StateStore) *Service {
	return &Service{
		store:          store,
		fetchers:       []Fetcher{},
		topicRouter:    make(map[string]int),
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

	// Initialize lists
	s.fetchers = []Fetcher{}
	s.topicRouter = make(map[string]int)

	// --- Common Config ---
	botToken, _ := s.store.GetSystemConfig("telegram_bot_token")
	chatID, _ := s.store.GetSystemConfig("telegram_chat_id")
	defaultThreadIDStr, _ := s.store.GetSystemConfig("telegram_message_thread_id")
	defaultThreadID, _ := strconv.Atoi(defaultThreadIDStr)

	// --- Finnhub Config ---
	finnhubKey, _ := s.store.GetSystemConfig("finnhub_api_key")
	if finnhubKey != "" {
		s.fetchers = append(s.fetchers, NewFinnhubFetcher(finnhubKey))
		s.topicRouter["Finnhub"] = defaultThreadID
	}

	// --- Mlion Config ---
	mlionKey, _ := s.store.GetSystemConfig("mlion_api_key")
	mlionTopicStr, _ := s.store.GetSystemConfig("mlion_target_topic_id")
	mlionEnabledStr, _ := s.store.GetSystemConfig("mlion_news_enabled")

	if mlionEnabledStr == "true" && mlionKey != "" {
		s.fetchers = append(s.fetchers, NewMlionFetcher(mlionKey))
		tid, err := strconv.Atoi(mlionTopicStr)
		if err != nil {
			log.Printf("⚠️ Mlion 话题 ID 解析失败 (%s), 使用默认 ID", mlionTopicStr)
			tid = defaultThreadID
		}
		s.topicRouter["Mlion"] = tid
	}

	if botToken == "" || chatID == "" {
		return fmt.Errorf("缺少必要的 Telegram 配置")
	}

	s.notifier = NewTelegramNotifier(botToken, chatID)

	return nil
}

func (s *Service) processAllCategories() {
	// 每个周期开始时，清空上个周期的已发送消息ID记录
	s.sentArticleIDs = make(map[int64]bool)

	for _, fetcher := range s.fetchers {
		if fetcher.Name() == "Finnhub" {
			// Finnhub supports categories
			categories := []string{"crypto", "general"}
			for _, cat := range categories {
				if err := s.ProcessFetcher(fetcher, cat); err != nil {
					log.Printf("⚠️ 处理新闻失败 [%s-%s]: %v", fetcher.Name(), cat, err)
				}
			}
		} else {
			// Mlion or others (default category "crypto" or ignored)
			if err := s.ProcessFetcher(fetcher, "crypto"); err != nil {
				log.Printf("⚠️ 处理新闻失败 [%s]: %v", fetcher.Name(), err)
			}
		}
	}
}

// ProcessFetcher 处理特定 Fetcher 的新闻
func (s *Service) ProcessFetcher(f Fetcher, category string) error {
	// 1. 获取新闻
	articles, err := f.FetchNews(category)
	if err != nil {
		return err
	}

	if len(articles) == 0 {
		return nil
	}

	// 2. 获取上次状态 (Per Source & Category ideally, but current schema is category-based)
	// Risk: Mlion and Finnhub both use "crypto" category key in DB.
	// Since ID spaces might collide or be vastly different, we should probably prefix the category in DB state?
	// Existing schema uses `category` string. If we use "crypto" for both, `lastID` might be messed up
	// because Finnhub IDs might be small/large and Mlion IDs different.
	// MITIGATION: Use "Mlion-crypto" as DB key for Mlion?
	// The `UpdateNewsState` and `GetNewsState` use `category` string key.
	// Let's modify the DB key used here, but keep article.Category as "crypto" for display.

	dbCategoryKey := category
	if f.Name() == "Mlion" {
		dbCategoryKey = "mlion_" + category
	}

	lastID, lastTime, err := s.store.GetNewsState(dbCategoryKey)
	if err != nil {
		return fmt.Errorf("获取状态失败: %w", err)
	}

	// 3. 过滤和排序
	var newArticles []Article

	for _, a := range articles {
		// 基础去重：按分类时间戳
		// Note: We check against the SOURCE-specific lastID/Time
		if int64(a.ID) <= lastID || a.Datetime <= lastTime {
			continue
		}

		// 全局消息ID去重 (Current Cycle)
		if s.sentArticleIDs[int64(a.ID)] {
			continue
		}

		newArticles = append(newArticles, a)
	}

	// 按时间升序排序（旧 -> 新）
	sort.Slice(newArticles, func(i, j int) bool {
		return newArticles[i].Datetime < newArticles[j].Datetime
	})

	// 4. 处理、发送并更新状态
	// Resolve Topic ID
	threadID := s.topicRouter[f.Name()]

	for i := range newArticles {
		a := &newArticles[i]

		// 原 AI 处理逻辑已移除

		msg := formatMessage(*a)

		if err := s.notifier.Send(msg, threadID); err != nil {
			log.Printf("❌ 发送Telegram消息失败: %v", err)
			continue
		}

		s.sentArticleIDs[int64(a.ID)] = true

		// 更新状态 using the prefixed key
		if err := s.store.UpdateNewsState(dbCategoryKey, int64(a.ID), a.Datetime); err != nil {
			log.Printf("⚠️ 更新新闻状态失败: %v", err)
		}

		log.Printf("📢 已推送新闻: [%s] %s", f.Name(), a.Headline)
		time.Sleep(2 * time.Second)
	}

	return nil
}

func formatMessage(a Article) string {
	// Ensure display in Beijing Time
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	t := time.Unix(a.Datetime, 0).In(loc)
	timeStr := t.Format("15:04")

	var icon string
	if a.Category == "crypto" {
		icon = "🪙"
	} else {
		icon = "📰"
	}

	// Check source specific formatting if needed, but generic is fine
	sourceTag := ""
	if a.Source != "" {
		sourceTag = fmt.Sprintf(" | %s", a.Source)
	}

	// 原 AI 处理逻辑分支已移除，只保留原生格式
	headline := strings.ReplaceAll(a.Headline, "<", "&lt;")
	headline = strings.ReplaceAll(headline, ">", "&gt;")
	summary := strings.ReplaceAll(a.Summary, "<", "&lt;")
	summary = strings.ReplaceAll(summary, ">", "&gt;")

	return fmt.Sprintf("<b>%s %s</b>\n\n📅 %s | #%s%s\n\n%s\n\n🔗 <a href=\" %s \">Read More</a>",
		icon, headline, timeStr, strings.ToUpper(a.Category), sourceTag, summary, a.URL)
}
