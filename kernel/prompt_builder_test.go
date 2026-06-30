package kernel

import (
	"strings"
	"testing"
	"time"
)

// TestPromptBuilder tests the prompt builder
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

		// Verify it contains key content
		mustContain := []string{
			"quantitative trading AI assistant",
			"Analyze account status",
			"Analyze current positions",
			"Analyze candidate symbols",
			"Make decisions",
			"Risk First",
			"Trailing Take-Profit",
			"Trend Following",
			"Scaling",
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

		// Verify it contains all valid action types
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

		// Verify it contains key content
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
		// Create test context
		ctx := createTestContext()

		builderZH := NewPromptBuilder(LangChinese)
		userPromptZH := builderZH.BuildUserPrompt(ctx)

		if userPromptZH == "" {
			t.Fatal("User prompt is empty")
		}

		// Verify it contains the data dictionary
		if !strings.Contains(userPromptZH, "Data Dictionary") {
			t.Error("User prompt should contain data dictionary")
		}

		// Verify it contains account information
		if !strings.Contains(userPromptZH, "3079.40") { // Equity
			t.Error("User prompt should contain account equity")
		}

		// Verify it contains position information
		if !strings.Contains(userPromptZH, "PIPPINUSDT") {
			t.Error("User prompt should contain position symbol")
		}

		// Verify it contains decision requirements
		if !strings.Contains(userPromptZH, "Now Make Your Decision") {
			t.Error("User prompt should contain decision requirements")
		}

		// English version
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

// TestValidateDecisionFormat tests decision format validation
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
				Reasoning:       "Detailed reasoning",
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

		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("Error message should mention 'cannot be empty', got: %v", err)
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

		if !strings.Contains(err.Error(), "invalid action") {
			t.Errorf("Error should mention 'invalid action', got: %v", err)
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

			// OPEN_NEW requires extra fields
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

// TestFormatDecisionExample tests decision example formatting
func TestFormatDecisionExample(t *testing.T) {
	t.Run("Chinese", func(t *testing.T) {
		example := FormatDecisionExample(LangChinese)

		if example == "" {
			t.Fatal("Decision example is empty")
		}

		// Should be valid JSON
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

		// Verify it is valid JSON format
		if !strings.HasPrefix(strings.TrimSpace(example), "[") {
			t.Error("Example should be a JSON array")
		}
	})
}

// BenchmarkBuildSystemPrompt performance benchmark
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

// BenchmarkBuildUserPrompt performance benchmark
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

// createTestContext creates a trading context for tests
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
