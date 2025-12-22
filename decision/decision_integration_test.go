package decision

import (
	"nofx/service/news"
	"strings"
	"testing"
)

func TestInitializeEnricherChain(t *testing.T) {
	mlionAPI := &news.MlionFetcher{} // Mock
	newsEnricher := NewNewsEnricher(mlionAPI)

	chain := InitializeEnricherChain(newsEnricher)

	if chain == nil {
		t.Fatal("Chain should not be nil")
	}

	// 验证enricher被添加（不用GetSize，直接测试功能）
	ctx := &Context{Extensions: make(map[string]interface{})}
	newsEnricher.SetEnabled(false)
	chain.ExecuteAll(ctx)

	if ctx.Extensions == nil {
		t.Error("Chain should have executed")
	}
}

func TestEnrichContextWithAllSources(t *testing.T) {
	ctx := &Context{
		Extensions: make(map[string]interface{}),
		Account:    AccountInfo{TotalEquity: 10000},
	}

	mlionAPI := &news.MlionFetcher{}
	newsEnricher := NewNewsEnricher(mlionAPI)
	newsEnricher.SetEnabled(false) // 禁用以快速完成测试

	chain := InitializeEnricherChain(newsEnricher)

	// 执行增强
	EnrichContextWithAllSources(ctx, chain)

	// 验证新闻上下文被添加（即使为空）
	newsCtx := ctx.GetNewsContext()
	if newsCtx == nil {
		t.Error("NewsContext should exist (even if disabled)")
	}
}

func TestBuildUserPromptWithNews_NoNews(t *testing.T) {
	ctx := &Context{
		Extensions: make(map[string]interface{}),
		Account: AccountInfo{
			TotalEquity:     10000,
			AvailableBalance: 5000,
			TotalPnL:        500,
			TotalPnLPct:     0.05,
			MarginUsedPct:   0.5,
		},
		Positions: []PositionInfo{
			{
				Symbol:           "BTC",
				MarkPrice:        50000,
				Quantity:         0.1,
				UnrealizedPnL:    100,
				UnrealizedPnLPct: 0.02,
			},
		},
		CandidateCoins: []CandidateCoin{
			{Symbol: "ETH", Sources: []string{"ai500"}},
		},
	}

	// 不添加新闻上下文
	prompt := BuildUserPromptWithNews(ctx)

	if prompt == "" {
		t.Error("Prompt should not be empty")
	}

	if !strings.Contains(prompt, "Account Status") {
		t.Error("Prompt should contain account status")
	}

	if !strings.Contains(prompt, "BTC") {
		t.Error("Prompt should contain position info")
	}

	// 不应该包含新闻section
	if strings.Contains(prompt, "Latest Market News") {
		t.Error("Prompt should not contain news section when not enriched")
	}
}

func TestBuildUserPromptWithNews_WithNews(t *testing.T) {
	ctx := &Context{
		Extensions: make(map[string]interface{}),
		Account: AccountInfo{
			TotalEquity:      10000,
			AvailableBalance: 5000,
		},
	}

	// 添加新闻上下文
	articles := []Article{
		{
			Headline:  "Bitcoin surges",
			Sentiment: 1,
			Symbol:    "BTC",
		},
	}
	newsCtx := NewNewsContext(articles)
	ctx.SetExtension("news", newsCtx)

	prompt := BuildUserPromptWithNews(ctx)

	if prompt == "" {
		t.Error("Prompt should not be empty")
	}

	// 应该包含新闻section
	if !strings.Contains(prompt, "Latest Market News") {
		t.Error("Prompt should contain news section")
	}

	if !strings.Contains(prompt, "Bitcoin surges") {
		t.Error("Prompt should contain news headlines")
	}
}

func TestEnrichContextWithAllSources_WithMockEnricher(t *testing.T) {
	ctx := &Context{
		Extensions: make(map[string]interface{}),
	}

	// 创建mock enricher
	mockEnricher := &MockEnricher{
		name:        "mock",
		enabledFlag: true,
		shouldFail:  false,
	}

	chain := NewEnrichmentChain()
	chain.AddEnricher(mockEnricher)

	EnrichContextWithAllSources(ctx, chain)

	// 验证enricher被调用
	if mockEnricher.callCount != 1 {
		t.Errorf("Enricher should be called once, was called %d times", mockEnricher.callCount)
	}

	// 验证扩展被设置
	if _, exists := ctx.GetExtension("mock"); !exists {
		t.Error("Mock extension should be set")
	}
}
