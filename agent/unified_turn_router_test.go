package agent

import (
	"context"
	"strings"
	"testing"
)

func TestParseUnifiedTurnDecisionNormalizesContextPolicy(t *testing.T) {
	raw := `{
		"topic_intent": "start_new",
		"business_action": "new_skill",
		"target_skill": "strategy_management:update_config",
		"context_mode": "fresh_context",
		"extracted_data": {"name": "BTC趋势"},
		"confidence": 0.82
	}`

	decision, err := parseUnifiedTurnDecision(raw)
	if err != nil {
		t.Fatalf("parse unified decision: %v", err)
	}
	if decision.TopicIntent != "start_new" {
		t.Fatalf("expected normalized topic intent, got %q", decision.TopicIntent)
	}
	if decision.BusinessAction != "new_skill" {
		t.Fatalf("expected business action new_skill, got %q", decision.BusinessAction)
	}
	if decision.ContextMode != "fresh_context" {
		t.Fatalf("expected fresh_context, got %q", decision.ContextMode)
	}
	if !decision.reliable() {
		t.Fatalf("expected decision to be reliable: %+v", decision)
	}
}

func TestUnifiedTurnDecisionRejectsLowConfidenceAndIncompleteDirectAnswer(t *testing.T) {
	lowConfidence := unifiedTurnDecision{
		TopicIntent:    "start_new",
		BusinessAction: "planned_agent",
		ContextMode:    "fresh_context",
		Confidence:     0.2,
	}
	lowConfidence = normalizeUnifiedTurnDecision(lowConfidence)
	if lowConfidence.reliable() {
		t.Fatalf("expected low confidence decision to fall back")
	}

	emptyDirect := unifiedTurnDecision{
		TopicIntent:    "instant_reply",
		BusinessAction: "direct_answer",
		ContextMode:    "use_current",
		Confidence:     0.9,
	}
	emptyDirect = normalizeUnifiedTurnDecision(emptyDirect)
	if emptyDirect.reliable() {
		t.Fatalf("expected direct_answer without reply_to_user to fall back")
	}
}

func TestExecuteUnifiedTurnDecisionDirectAnswerRecordsHistory(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), nil)
	userID := int64(101)
	decision := normalizeUnifiedTurnDecision(unifiedTurnDecision{
		TopicIntent:    "instant_reply",
		BusinessAction: "direct_answer",
		ContextMode:    "use_current",
		ReplyToUser:    "你好，我在。",
		Confidence:     0.9,
	})

	answer, handled, err := a.executeUnifiedTurnDecision(context.Background(), "default", userID, "zh", "你好", decision, nil)
	if err != nil {
		t.Fatalf("execute unified decision: %v", err)
	}
	if !handled {
		t.Fatal("expected direct answer to be handled")
	}
	if answer != "你好，我在。" {
		t.Fatalf("unexpected answer: %q", answer)
	}

	history := a.history.Get(userID)
	if len(history) != 2 {
		t.Fatalf("expected user and assistant history entries, got %d", len(history))
	}
	if history[0].Role != "user" || history[0].Content != "你好" {
		t.Fatalf("unexpected user history entry: %+v", history[0])
	}
	if history[1].Role != "assistant" || history[1].Content != "你好，我在。" {
		t.Fatalf("unexpected assistant history entry: %+v", history[1])
	}
}

func TestBuildUnifiedTurnRouterPromptNamesContextPolicy(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), nil)
	systemPrompt, userPrompt := a.buildUnifiedTurnRouterPrompt(42, "zh", "不是交易员，是策略")
	for _, want := range []string{
		"context_mode values",
		"fresh_context",
		"downstream modules",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected system prompt to contain %q", want)
		}
	}
	if !strings.Contains(userPrompt, "不是交易员，是策略") {
		t.Fatalf("expected user prompt to contain current user message")
	}
}
