package main

import (
	"fmt"
	"log"
	"nofx/service/news"
	"strings"
	"time"
)

func main() {
	fmt.Println("🚀 开始多语言新闻聚合功能集成测试")
	fmt.Println("========================================")

	// 1. 配置凭证
	// 注意：在实际CI/CD中应使用环境变量，此处为演示测试直接使用提供的 Key
	deepseekKey := "sk-17ae639e2f214d51b85fd38d43bff9bf"
	deepseekURL := "https://api.deepseek.com/chat/completions"
	botToken := "8291537816:AAEQTE7Jd5AGQ9dkq7NMPewlSr8Kun2qXao"
	chatID := "-1002678075016"
	threadID := 2

	// 2. 准备测试数据 (模拟一条重磅新闻)
	article := &news.Article{
		ID:       88888,
		Headline: "Bitcoin Breaks $150,000 Barrier as Global Adoption Accelerates",
		Summary:  "Major central banks announced today they will hold BTC as a reserve asset, triggering a massive supply shock. Market analysts call this the 'Supercycle'.",
		URL:      "https://monnaire.capital/news/btc-150k-test",
		Datetime: time.Now().Unix(),
		Category: "crypto",
		Source:   "IntegrationTest",
	}

	// 3. 测试 AI 处理 (准确性 & 时效性)
	fmt.Println("\n🧪 [Step 1] 测试 DeepSeek AI 翻译与摘要...")
	processor := news.NewDeepSeekProcessor(deepseekKey, deepseekURL, "zh-CN")

	start := time.Now()
	err := processor.Process(article)
	duration := time.Since(start)

	if err != nil {
		log.Fatalf("❌ AI 处理失败: %v", err)
	}

	fmt.Printf("✅ AI 处理成功!\n")
	fmt.Printf("   ⏱️  耗时: %v\n", duration)
	fmt.Printf("   📝 原标题: %s\n", article.Headline)
	fmt.Printf("   🇨🇳 译标题: %s\n", article.TranslatedHeadline)
	fmt.Printf("   📄 译摘要: %s\n", article.TranslatedSummary)
	fmt.Printf("   🎭 情感值: %s\n", article.Sentiment)

	if article.TranslatedHeadline == "" || article.TranslatedSummary == "" {
		log.Fatal("❌ 错误: 翻译内容为空")
	}

	// 4. 测试 Telegram 推送 (用户体验)
	fmt.Println("\n🧪 [Step 2] 测试 Telegram 推送...")
	notifier := news.NewTelegramNotifier(botToken, chatID)

	// 模拟 formatMessage 逻辑 (因为它是私有的)
	msg := formatTestMessage(*article)

	err = notifier.Send(msg, threadID)
	if err != nil {
		log.Fatalf("❌ Telegram 发送失败: %v", err)
	}
	fmt.Printf("✅ Telegram 消息已发送到 Topic %d\n", threadID)

	fmt.Println("\n🎉 测试完成! 请检查 Telegram 群组中的消息格式。")
}

// 复制自 service.go 的逻辑，用于测试脚本
func formatTestMessage(a news.Article) string {
	t := time.Unix(a.Datetime, 0)
	timeStr := t.Format("15:04")

	var icon string
	if a.Category == "crypto" {
		icon = "🪙"
	} else {
		icon = "📰"
	}

	if a.AIProcessed {
		sentimentIcon := ""
		switch a.Sentiment {
		case "POSITIVE":
			sentimentIcon = "🟢"
		case "NEGATIVE":
			sentimentIcon = "🔴"
		default:
			sentimentIcon = "⚪"
		}

		return fmt.Sprintf("<b>%s %s %s</b>\n\n📅 %s | #%s | [TEST]\n\n📝 <b>摘要</b>: %s\n\n---------------\n原文: <a href=\" %s \">%s</a>",
			icon, a.TranslatedHeadline, sentimentIcon,
			timeStr, strings.ToUpper(a.Category),
			a.TranslatedSummary,
			a.URL, a.Headline)
	}
	return "Error: AI Not Processed"
}
