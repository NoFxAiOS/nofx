package main

import (
	"fmt"
	"log"
	"nofx/service/news"
	"os"
)

func main() {
	fmt.Println("🧪 开始 Finnhub 连接测试...")

	apiKey := os.Getenv("FINNHUB_API_KEY")
	if apiKey == "" {
		// Fallback to the one in the prompt if not set
		apiKey = "d4p6v61r01qnosac6v5gd4p6v61r01qnosac6v60"
		fmt.Println("⚠️  使用默认测试 API Key")
	}

	fetcher := news.NewFinnhubFetcher(apiKey)

	fmt.Println("🔄 正在获取 Crypto 新闻...")
	articles, err := fetcher.FetchNews("crypto")
	if err != nil {
		log.Fatalf("❌ 获取失败: %v", err)
	}

	if len(articles) == 0 {
		fmt.Println("⚠️  成功连接，但未返回新闻 (可能是空数据)")
	} else {
		fmt.Printf("✅ 成功获取 %d 条新闻!\n", len(articles))
		fmt.Printf("📰 最新一条: %s (%s)\n", articles[0].Headline, articles[0].URL)
	}

	fmt.Println("--------------------------------------------------")

	fmt.Println("🧪 开始 Telegram 发送测试...")
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" {
		botToken = "8291537816:AAEQTE7Jd5AGQ9dkq7NMPewlSr8Kun2qXao"
		fmt.Println("⚠️  使用默认测试 Bot Token")
	}
	if chatID == "" {
		chatID = "-1002678075016"
		fmt.Println("⚠️  使用默认测试 Chat ID")
	}

	notifier := news.NewTelegramNotifier(botToken, chatID)

	msg := "<b>🧪 集成测试消息</b>\n\n这是来自 CI/CD 流程的自动测试消息，验证系统连通性。\n\n✅ System Check: OK"
	// 使用 2 作为 Thread ID (指定话题)
	err = notifier.Send(msg, 2)
	if err != nil {
		log.Fatalf("❌ 发送失败: %v", err)
	}

	fmt.Println("✅ Telegram 消息发送成功!")
}
