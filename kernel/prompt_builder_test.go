package kernel

import (
	"strings"
	"testing"
	"time"
)

// TestPromptBuilder 测试提示词构建器
func TestPromptBuilder(t *testing.T) {
	t.Run("NewPromptBuilder", func(t *testing.T) {
		builderZH := NewPromptBuilder(LangChinese)
		if builderZH == nil {
			t.Fatal("NewPromptBuilder returned nil")
		}
		if builderZH.lang != LangChinese {
			t.Error("Language not set correctly")
		}

		builderEN := NewPromptBuilder(LangEnglish)
		if builderEN.lang != LangEnglish {
			t.Error("Language not set correctly")
		}
	})

	t.Run("BuildSystemPrompt_Chinese", func(t *testing.T) {
		builder := NewPromptBuilder(LangChinese)
		systemPrompt := builder.BuildSystemPrompt()

		if systemPrompt == "" {
			t.Fatal("System prompt is empty")
		}

		// 验证包含关键内容
		mustContain := []string{
			"量化交易AI助手",
			"分析账户状态",
			"分析当前持仓",
			"分析候选币种",
			"做出决策",
			"风险优先",
			"跟踪止盈",
			"顺势交易",
			"分批操作",
			"JSON",
			"symbol",
			"action",
			"reasoning",
		}

		for _, keyword := range mustContain {
			if !strings.Contains(systemPrompt, keyword) {
				t.Errorf("System prompt should contain '%s'", keyword)
			}
		}

		// 验证包含所有有效的action类型
		actions := []string{"HOLD", "PARTIAL_CLOSE", "FULL_CLOSE", "ADD_POSITION", "OPEN_NEW", "WAIT"}
		for _, action := range actions {
			if !strings.Contains(systemPrompt, action) {
				t.Errorf("System prompt should mention action type '%s'", action)
			}
		}
	})

	t.Run("BuildSystemPrompt_English", func(t *testing.T) {
		builder := NewPromptBuilder(LangEnglish)
		systemPrompt := builder.BuildSystemPrompt()

		if systemPrompt == "" {
			t.Fatal("System prompt is empty")
		}

		// 验证包含关键内容
		mustContain := []string{
			"quantitative trading AI",
			"Analyze Account Status",
			"Analyze Current Positions",
			"Analyze Candidate Coins",
			"Make Decisions",
			"Risk First",
			"Trailing Take-Profit",
			"Trend Following",
			"Scale Operations",
			"JSON",
			"symbol",
			"action",
			"reasoning",
		}

		for _, keyword := range mustContain {
			if !strings.Contains(systemPrompt, keyword) {
				t.Errorf("System prompt should contain '%s'", keyword)
			}
		}
	})

	t.Run("BuildUserPrompt", func(t *testing.T) {
		// 创建测试上下文
		ctx := createTestContext()

		builderZH := NewPromptBuilder(LangChinese)
		userPromptZH := builderZH.BuildUserPrompt(ctx)

		if userPromptZH == "" {
			t.Fatal("User prompt is empty")
		}

		// 验证包含数据字典
		if !strings.Contains(userPromptZH, "数据字典") {
			t.Error("User prompt should contain data dictionary")
		}

		// 验证包含账户信息
		if !strings.Contains(userPromptZH, "3079.40") { // Equity
			t.Error("User prompt should contain account equity")
		}

		// 验证包含持仓信息
		if !strings.Contains(userPromptZH, "PIPPINUSDT") {
			t.Error("User prompt should contain position symbol")
		}

		// 验证包含决策要求
		if !strings.Contains(userPromptZH, "现在请做出决策") {
			t.Error("User prompt should contain decision requirements")
		}

		// 英文版本
		builderEN := NewPromptBuilder(LangEnglish)
		userPromptEN := builderEN.BuildUserPrompt(ctx)

		if !strings.Contains(userPromptEN, "Data Dictionary") {
			t.Error("English user prompt should contain data dictionary")
		}

		if !strings.Contains(userPromptEN, "Make Your Decision Now") {
			t.Error("English user prompt should contain decision requirements")
		}
	})
}

// TestValidateDecisionFormat 测试决策格式验证
func TestValidateDecisionFormat(t *testing.T) {
	t.Run("ValidDecision", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:          "BTCUSDT",
				Action:          "OPEN_NEW",
				Leverage:        3,
				PositionSizeUSD: 1000,
				StopLoss:        42000,
				TakeProfit:      48000,
				Confidence:      85,
				Reasoning:       "详细的推理过程",
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err != nil {
			t.Errorf("Valid decision should not return error: %v", err)
		}
	})

	t.Run("EmptyDecisions", func(t *testing.T) {
		decisions := []Decision{}

		err := ValidateDecisionFormat(decisions)
		if err == nil {
			t.Error("Empty decisions should return error")
		}

		if !strings.Contains(err.Error(), "不能为空") {
			t.Errorf("Error message should mention '不能为空', got: %v", err)
		}
	})

	t.Run("MissingSymbol", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:    "", // Missing
				Action:    "HOLD",
				Reasoning: "Test",
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err == nil {
			t.Error("Missing symbol should return error")
		}

		if !strings.Contains(err.Error(), "symbol") {
			t.Errorf("Error should mention 'symbol', got: %v", err)
		}
	})

	t.Run("MissingAction", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:    "BTCUSDT",
				Action:    "", // Missing
				Reasoning: "Test",
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err == nil {
			t.Error("Missing action should return error")
		}
	})

	t.Run("MissingReasoning", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:    "BTCUSDT",
				Action:    "HOLD",
				Reasoning: "", // Missing
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err == nil {
			t.Error("Missing reasoning should return error")
		}
	})

	t.Run("InvalidAction", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:    "BTCUSDT",
				Action:    "INVALID_ACTION",
				Reasoning: "Test",
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err == nil {
			t.Error("Invalid action should return error")
		}

		if !strings.Contains(err.Error(), "无效的action") {
			t.Errorf("Error should mention '无效的action', got: %v", err)
		}
	})

	t.Run("OpenNewMissingLeverage", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:          "BTCUSDT",
				Action:          "OPEN_NEW",
				Leverage:        0, // Missing
				PositionSizeUSD: 1000,
				Reasoning:       "Test",
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err == nil {
			t.Error("OPEN_NEW without leverage should return error")
		}

		if !strings.Contains(err.Error(), "leverage") {
			t.Errorf("Error should mention 'leverage', got: %v", err)
		}
	})

	t.Run("OpenNewMissingPositionSize", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:          "BTCUSDT",
				Action:          "OPEN_NEW",
				Leverage:        3,
				PositionSizeUSD: 0, // Missing
				Reasoning:       "Test",
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err == nil {
			t.Error("OPEN_NEW without position_size_usd should return error")
		}

		if !strings.Contains(err.Error(), "position_size_usd") {
			t.Errorf("Error should mention 'position_size_usd', got: %v", err)
		}
	})

	t.Run("MultipleDecisions", func(t *testing.T) {
		decisions := []Decision{
			{
				Symbol:    "BTCUSDT",
				Action:    "HOLD",
				Reasoning: "Hold BTC",
			},
			{
				Symbol:          "ETHUSDT",
				Action:          "OPEN_NEW",
				Leverage:        3,
				PositionSizeUSD: 500,
				Reasoning:       "Open ETH",
			},
		}

		err := ValidateDecisionFormat(decisions)
		if err != nil {
			t.Errorf("Multiple valid decisions should not return error: %v", err)
		}
	})

	t.Run("ValidActions", func(t *testing.T) {
		validActions := []string{"HOLD", "PARTIAL_CLOSE", "FULL_CLOSE", "ADD_POSITION", "OPEN_NEW", "WAIT"}

		for _, action := range validActions {
			decisions := []Decision{
				{
					Symbol:    "BTCUSDT",
					Action:    action,
					Reasoning: "Test " + action,
				},
			}

			// OPEN_NEW需要额外字段
			if action == "OPEN_NEW" {
				decisions[0].Leverage = 3
				decisions[0].PositionSizeUSD = 1000
			}

			err := ValidateDecisionFormat(decisions)
			if err != nil {
				t.Errorf("Valid action '%s' should not return error: %v", action, err)
			}
		}
	})
}

// TestFormatDecisionExample 测试决策示例格式化
func TestFormatDecisionExample(t *testing.T) {
	t.Run("Chinese", func(t *testing.T) {
		example := FormatDecisionExample(LangChinese)

		if example == "" {
			t.Fatal("Decision example is empty")
		}

		// 应该是有效的JSON
		if !strings.HasPrefix(strings.TrimSpace(example), "[") {
			t.Error("Example should be a JSON array")
		}

		if !strings.Contains(example, "BTCUSDT") {
			t.Error("Example should contain BTCUSDT")
		}
	})

	t.Run("English", func(t *testing.T) {
		example := FormatDecisionExample(LangEnglish)

		if example == "" {
			t.Fatal("Decision example is empty")
		}

		// 验证是有效的JSON格式
		if !strings.HasPrefix(strings.TrimSpace(example), "[") {
			t.Error("Example should be a JSON array")
		}
	})
}

// BenchmarkBuildSystemPrompt 性能测试
func BenchmarkBuildSystemPrompt(b *testing.B) {
	builder := NewPromptBuilder(LangChinese)

	b.Run("Chinese", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = builder.BuildSystemPrompt()
		}
	})

	builderEN := NewPromptBuilder(LangEnglish)
	b.Run("English", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = builderEN.BuildSystemPrompt()
		}
	})
}

// BenchmarkBuildUserPrompt 性能测试
func BenchmarkBuildUserPrompt(b *testing.B) {
	builder := NewPromptBuilder(LangChinese)
	ctx := createTestContext()

	b.Run("Chinese", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = builder.BuildUserPrompt(ctx)
		}
	})

	builderEN := NewPromptBuilder(LangEnglish)
	b.Run("English", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = builderEN.BuildUserPrompt(ctx)
		}
	})
}

// createTestContext 创建测试用的交易上下文
func createTestContext() *Context {
	return &Context{
		CurrentTime:    time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes: 78,
		CallCount:      27,
		Account: AccountInfo{
			TotalEquity:      3079.40,
			AvailableBalance: 2353.02,
			UnrealizedPnL:    21.48,
			TotalPnL:         470.89,
			TotalPnLPct:      15.87,
			MarginUsed:       726.38,
			MarginUsedPct:    23.6,
			PositionCount:    1,
		},
		Positions: []PositionInfo{
			{
				Symbol:           "PIPPINUSDT",
				Side:             "long",
				EntryPrice:       0.4888,
				MarkPrice:        0.4937,
				Quantity:         4414.0,
				Leverage:         3,
				UnrealizedPnL:    21.48,
				UnrealizedPnLPct: 2.96,
				PeakPnLPct:       2.99,
				LiquidationPrice: 0.0000,
				MarginUsed:       726.0,
				UpdateTime:       time.Now().UnixMilli(),
			},
		},
		RecentOrders: []RecentOrder{
			{
				Symbol:       "PIPPINUSDT",
				Side:         "long",
				EntryPrice:   0.4756,
				ExitPrice:    0.4862,
				RealizedPnL:  46.10,
				PnLPct:       6.71,
				EntryTime:    "12-24 04:36 UTC",
				ExitTime:     "12-24 05:35 UTC",
				HoldDuration: "58m",
			},
		},
		CandidateCoins: []CandidateCoin{
			{
				Symbol:  "BTCUSDT",
				Sources: []string{"ai500"},
			},
			{
				Symbol:  "ETHUSDT",
				Sources: []string{"oi_top"},
			},
		},
		Timeframes: []string{"5M", "15M", "1H", "4H"},
	}
}
