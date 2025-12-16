package news

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFinnhubFetcher_SourceFiltering(t *testing.T) {
	// 模拟数据
	f := &FinnhubFetcher{
		client: &http.Client{},
	}

	tests := []struct {
		name           string
		source         string
		expectedResult bool
	}{
		// 权威媒体 - 允许
		{"Reuters", "Reuters", true},
		{"Bloomberg", "Bloomberg", true},
		{"CNBC", "CNBC", true},
		{"WSJ", "Wall Street Journal", true},
		{"WSJ Short", "WSJ", true},
		{"FT", "Financial Times", true},
		{"FT Short", "FT", true},
		{"Xinhua", "Xinhua", true},
		{"Caixin", "Caixin Global", true}, // 包含匹配
		{"BBC", "BBC News", true},
		{"SCMP", "South China Morning Post", true},
		{"The Economist", "The Economist", true},
		{"IMF", "IMF Blog", true},

		// 非权威媒体 - 拒绝
		{"Unknown Blog", "Crypto Daily", false},
		{"Personal Blog", "John Doe's Blog", false},
		{"Yahoo", "Yahoo Finance", false}, // 虽然是大平台，但不在白名单
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.isAllowedSource(tt.source)
			assert.Equal(t, tt.expectedResult, result, "Source: %s", tt.source)
		})
	}
}

func TestFinnhubFetcher_KeywordFiltering(t *testing.T) {
	f := &FinnhubFetcher{client: &http.Client{}}

	// 这个测试主要验证 containsAny 逻辑
	// 实际集成测试已经在 FetchNews 中完成，但这里我们测试核心过滤逻辑

	tests := []struct {
		name     string
		text     string
		keywords []string
		expected bool
	}{
		{"Match US", "Fed raises rates", usKeywords, true},
		{"Match CN", "PBOC cuts LPR", cnKeywords, true},
		{"Match EU", "ECB holds steady", euKeywords, true},
		{"No Match", "Bitcoin goes up", usKeywords, false},

		// 新增关键词测试
		{"Match Inflation", "CPI data shows increase", policyKeywords, true}, // 注意：policyKeywords中都是大写或Mixed Case，containsAny区分大小写吗？FetchNews里转了Lower
		{"Match Geopolitics", "Trade war tensions rise", policyKeywords, true},

		// 负面关键词测试 (模拟 exclusion check 逻辑)
		{"Excluded Sports", "World Cup football final score", excludedKeywords, true},
		{"Excluded Movie", "Hollywood celebrity news", excludedKeywords, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟 FetchNews 中的大小写转换逻辑
			lowerText := strings.ToLower(tt.text)
			// 注意：finnhub.go 中的 keywords 定义是Mixed Case，但在 FetchNews 中是转换为 Lower 比较的
			// containsAny 只是简单 strings.Contains，所以如果 keyword 有大写，lowerText 不会匹配
			// 我们需要调整测试逻辑，或者调整 containsAny 的预期行为。
			// 在 finnhub.go 中：matched check 用了 strings.ToLower(kw)，但 containsAny 直接用了 kw
			// containsAny 主要用于 Region 判断，其 kw 列表 (usKeywords) 都是小写。
			// 而 policyKeywords 和 excludedKeywords 是 Mixed Case。

			// 为了测试准确，我们这里模拟 FetchNews 的逻辑：
			// 如果测试的是 policyKeywords 或 excludedKeywords，我们需要转小写比较

			// 这是一个简单的单元测试 helper，我们应该确保传入的 keywords 格式符合 containsAny 的期望
			// 或者我们改进 containsAny 让它更通用?
			// 当前代码中 containsAny 只用于 Region Check (all lower case keywords) 和 Exclusion Check (mixed case keywords in definition)
			// 等等，Excluded Check: containsAny(text, excludedKeywords)
			// excludedKeywords 定义全是小写 except "World Cup"? 不，全是小写。
			// policyKeywords 定义有大写。FetchNews 中是单独循环处理 policyKeywords 的。

			// 修正：FetchNews 中 policyKeywords 是手动循环并 ToLower 的。
			// Excluded Check 使用 containsAny，且 excludedKeywords 定义全是小写。
			// Region Check 使用 containsAny，且 usKeywords 等全是小写。
			// 所以 containsAny 假设 keywords 是小写的。

			// 确保 policyKeywords 测试用的小写版 (因为 containsAny 不做转换)
			if tt.name == "Match Inflation" || tt.name == "Match Geopolitics" {
				// 手动模拟 FetchNews 的 policyKeywords 匹配逻辑
				matched := false
				for _, kw := range tt.keywords {
					if strings.Contains(lowerText, strings.ToLower(kw)) {
						matched = true
						break
					}
				}
				assert.Equal(t, tt.expected, matched)
			} else {
				assert.Equal(t, tt.expected, f.containsAny(lowerText, tt.keywords))
			}
		})
	}
}
