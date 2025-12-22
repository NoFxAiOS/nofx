package decision

import (
	"fmt"
	"log"
)

// InitializeEnricherChain 初始化增强链（包含所有可用的上下文增强器）
// 这个函数在决策引擎启动时被调用
func InitializeEnricherChain(newsEnricher *NewsEnricher) *EnrichmentChain {
	chain := NewEnrichmentChain()

	if newsEnricher != nil {
		chain.AddEnricher(newsEnricher)
	}

	// 未来可以添加更多增强器：
	// chain.AddEnricher(twitterEnricher)
	// chain.AddEnricher(chainDataEnricher)
	// chain.AddEnricher(panicIndexEnricher)

	return chain
}

// EnrichContextWithAllSources 使用所有可用的数据源增强上下文
// 这个函数在buildUserPrompt之前被调用
func EnrichContextWithAllSources(ctx *Context, chain *EnrichmentChain) {
	if ctx == nil || chain == nil {
		return
	}

	// 执行所有启用的增强器（非致命性失败）
	errors := chain.ExecuteAll(ctx)

	// 记录任何失败（用于监控，但不影响决策）
	for _, err := range errors {
		log.Printf("⚠️ Context enrichment error: %v", err)
	}
}

// BuildUserPromptWithNews 构建包含新闻数据的用户提示
// 这是原有buildUserPrompt的增强版本，融合新闻信息
func BuildUserPromptWithNews(ctx *Context) string {
	if ctx == nil {
		return ""
	}

	// 获取新闻上下文（如果启用）
	newsCtx := ctx.GetNewsContext()
	newsSection := BuildSafeNewsPromptSection(newsCtx)

	// 构建原始的市场数据部分（保持兼容性）
	marketSection := buildOriginalUserPrompt(ctx)

	// 组合市场数据和新闻部分
	if newsSection != "" {
		return marketSection + "\n\n" + newsSection
	}

	return marketSection
}

// buildOriginalUserPrompt 保留原有的用户提示构建逻辑（不含新闻）
// 这是对原有buildUserPrompt的包装，确保兼容性
func buildOriginalUserPrompt(ctx *Context) string {
	// 这里应该调用原有的buildUserPrompt()逻辑
	// 由于原有的buildUserPrompt()是局部函数，我们在这里模拟其功能
	// 实际上应该将其提取为公共函数

	var prompt string

	// 账户信息
	prompt += fmt.Sprintf("## Account Status\n")
	prompt += fmt.Sprintf("Total Equity: $%.2f\n", ctx.Account.TotalEquity)
	prompt += fmt.Sprintf("Available Balance: $%.2f\n", ctx.Account.AvailableBalance)
	prompt += fmt.Sprintf("Total P&L: $%.2f (%.2f%%)\n", ctx.Account.TotalPnL, ctx.Account.TotalPnLPct)
	prompt += fmt.Sprintf("Margin Used: %.2f%%\n\n", ctx.Account.MarginUsedPct*100)

	// 持仓信息
	if len(ctx.Positions) > 0 {
		prompt += fmt.Sprintf("## Current Positions (%d)\n", len(ctx.Positions))
		for _, pos := range ctx.Positions {
			prompt += fmt.Sprintf("- %s: %.2f @ %.2f (P&L: $%.2f, %.2f%%)\n",
				pos.Symbol, pos.Quantity, pos.MarkPrice, pos.UnrealizedPnL, pos.UnrealizedPnLPct*100)
		}
		prompt += "\n"
	}

	// 候选币种
	if len(ctx.CandidateCoins) > 0 {
		prompt += fmt.Sprintf("## Candidate Coins (%d)\n", len(ctx.CandidateCoins))
		for i, coin := range ctx.CandidateCoins {
			if i < 20 { // 限制前20个
				prompt += fmt.Sprintf("- %s (from: %s)\n", coin.Symbol, coin.Sources[0])
			}
		}
		prompt += "\n"
	}

	return prompt
}
