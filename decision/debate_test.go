package decision

import (
	"nofx/mcp"
	"strings"
	"testing"
	"time"
)

// MockAIClient 模拟AI客户端
type MockAIClient struct {
	CallLog []string
}

func (m *MockAIClient) SetAPIKey(apiKey string, customURL string, customModel string) {}
func (m *MockAIClient) SetTimeout(timeout time.Duration)                              {}

func (m *MockAIClient) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	m.CallLog = append(m.CallLog, systemPrompt)

	// Simple response logic based on system prompt to simulate different agents
	if strings.Contains(systemPrompt, "Bullish Agent") {
		return "I am bullish because price is up.", nil
	}
	if strings.Contains(systemPrompt, "Bearish Agent") {
		return "I am bearish because volume is down.", nil
	}
	if strings.Contains(systemPrompt, "Judge Agent") {
		// 使用反引号，但注意 Go 不支持在反引号字符串中转义反引号。
		// 这里内容没有嵌套反引号，应该没问题。
		return `<reasoning>Bullish agent wins.</reasoning>
<decision>
` + "```json" + `
[{"symbol": "BTCUSDT", "action": "wait", "reasoning": "wait and see"}]
` + "```" + `
</decision>`, nil
	}

	return "Default response", nil
}

func (m *MockAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	return "", nil
}

func TestGetDebateDecision_Flow(t *testing.T) {
	// 0. Fix Prompt Path for Test
	// Because tests run in the package dir, we need to look one level up for prompts
	_ = ReloadPromptTemplates() // Try reloading first
	if _, err := GetPromptTemplate("long_agent"); err != nil {
		// If failed, try loading from ../prompts
		if err := globalPromptManager.LoadTemplates("../prompts"); err != nil {
			t.Logf("Warning: Could not load templates from ../prompts: %v", err)
		}
	}

	// 1. Setup Context
	ctx := &Context{
		Account: AccountInfo{
			TotalEquity: 1000,
		},
		Positions:      []PositionInfo{},
		CandidateCoins: []CandidateCoin{},
		BTCETHLeverage: 10,
		AltcoinLeverage: 5,
	}

	mockClient := &MockAIClient{}

	// 2. Run Debate
	decision, err := GetDebateDecision(ctx, mockClient)

	if err != nil {
		t.Fatalf("GetDebateDecision failed: %v", err)
	}

	// 3. Verify Flow
	if decision == nil {
		t.Fatal("Expected decision, got nil")
	}

	// Check CoTTrace to see if it contains the transcript
	if !strings.Contains(decision.CoTTrace, "--- BULLISH AGENT (ROUND 1) ---") {
		t.Error("CoTTrace missing Bullish Agent Round 1")
	}
	if !strings.Contains(decision.CoTTrace, "--- BEARISH AGENT (ROUND 1) ---") {
		t.Error("CoTTrace missing Bearish Agent Round 1")
	}
	if !strings.Contains(decision.CoTTrace, "--- JUDGE VERDICT ---") {
		t.Error("CoTTrace missing Judge Verdict")
	}

	t.Logf("Decision CoT Trace:\n%s", decision.CoTTrace)
}
