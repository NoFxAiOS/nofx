package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
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

func TestParseUnifiedTurnDecisionAcceptsSkillTaskList(t *testing.T) {
	raw := `{
		"topic_intent": "start_new",
		"business_action": "skill_tasks",
		"context_mode": "fresh_context",
		"tasks": [
			{"id":"task_1","skill":"strategy_management","action":"create","request":"创建高频交易策略","depends_on":[]},
			{"id":"task_2","skill":"trader_management","action":"configure_strategy","request":"绑定到交易员","depends_on":["task_1"]}
		],
		"confidence": 0.86
	}`

	decision, err := parseUnifiedTurnDecision(raw)
	if err != nil {
		t.Fatalf("parse unified decision: %v", err)
	}
	if decision.BusinessAction != "skill_tasks" {
		t.Fatalf("expected skill_tasks, got %q", decision.BusinessAction)
	}
	if len(decision.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %+v", decision.Tasks)
	}
	if decision.Tasks[0].Skill != "strategy_management" || decision.Tasks[0].Action != "create" {
		t.Fatalf("unexpected first task: %+v", decision.Tasks[0])
	}
	if !decision.reliable() {
		t.Fatalf("expected task-list decision to be reliable: %+v", decision)
	}
}

func TestUnifiedTurnDecisionNewSkillCanUseSingleTask(t *testing.T) {
	decision := normalizeUnifiedTurnDecision(unifiedTurnDecision{
		TopicIntent:    "start_new",
		BusinessAction: "new_skill",
		ContextMode:    "fresh_context",
		Tasks: []WorkflowTask{{
			Skill:   "strategy_management",
			Action:  "create",
			Request: "创建高频交易策略",
		}},
		Confidence: 0.9,
	})
	if !decision.reliable() {
		t.Fatalf("expected new_skill with task list to be reliable: %+v", decision)
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

func TestExecuteUnifiedTurnDecisionContinueActiveDoesNotHandOffToPlanner(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "continue-active-router.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), nil)
	userID := int64(102)

	session := newActiveSkillSession(userID, "strategy_management", "create")
	session.Goal = "创建网格策略"
	session.CollectedFields["name"] = "我的网格策略"
	session.CollectedFields["strategy_type"] = "grid_trading"
	setActiveSessionPendingHint(&session, "现在还需要确认网格交易对、网格数量、总投入、杠杆和价格区间。")
	a.saveActiveSkillSession(session)

	decision := normalizeUnifiedTurnDecision(unifiedTurnDecision{
		TopicIntent:    "continue_active",
		BusinessAction: "planned_agent",
		ContextMode:    "use_current",
		Confidence:     0.9,
	})
	answer, handled, err := a.executeUnifiedTurnDecision(context.Background(), "default", userID, "zh", "那你帮我创吧", decision, nil)
	if err != nil {
		t.Fatalf("execute unified decision: %v", err)
	}
	if !handled {
		t.Fatal("expected active session continuation to be handled")
	}
	if !strings.Contains(answer, "还缺") || !strings.Contains(answer, "交易对") || strings.Contains(answer, "交易机器人") || strings.Contains(answer, "AI模型和交易所") {
		t.Fatalf("expected strategy session to continue without planner/trader handoff, got: %s", answer)
	}
	if _, ok := a.getActiveSkillSession(userID); !ok {
		t.Fatalf("expected strategy active session to remain pending")
	}
}

func TestGuardUnexecutedActiveTaskCompletionBlocksCreationClaim(t *testing.T) {
	session := ActiveSkillSession{
		SkillName:  "strategy_management",
		ActionName: "create",
	}
	reply, blocked := guardUnexecutedActiveTaskCompletion("zh", session, "已经创建好了。策略现在就在你的策略列表里。")
	if !blocked {
		t.Fatalf("expected unexecuted active create completion claim to be blocked")
	}
	if !strings.Contains(reply, "还没有真正创建") {
		t.Fatalf("expected honest not-created reply, got: %s", reply)
	}

	_, blocked = guardUnexecutedActiveTaskCompletion("zh", session, "我建议先用 BTCUSDT 做新手网格策略。")
	if blocked {
		t.Fatalf("non-completion proposal should not be blocked")
	}
}

func TestGuardUnsupportedAsyncPromiseBlocksFakeDiagnosisProgress(t *testing.T) {
	reply, blocked := guardUnsupportedAsyncPromise("zh", "诊断还在进行中，请再稍等一下。我马上分析完“小小”的历史交易记录，找到亏损原因后会立刻告诉您。")
	if !blocked {
		t.Fatal("expected fake async diagnosis progress to be blocked")
	}
	for _, want := range []string{"没有后台异步任务", "当前回复"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected guarded reply to contain %q, got: %s", want, reply)
		}
	}

	_, blocked = guardUnsupportedAsyncPromise("zh", "我需要策略名称和历史记录范围，才能开始诊断。")
	if blocked {
		t.Fatal("missing-info diagnosis reply should not be blocked")
	}

	_, blocked = guardUnsupportedAsyncPromise("zh", "好的，参数已确认，正在为您创建“餐巾纸”网格策略。")
	if !blocked {
		t.Fatal("expected fake async strategy create progress to be blocked")
	}
}

func TestFinishTaskGuardBlocksFakeCreateProgressPromise(t *testing.T) {
	reply, blocked := guardUnsupportedAsyncPromise("zh", "策略正在创建中，请稍等一会儿。创建成功后我会立刻告诉您。")
	if !blocked {
		t.Fatal("expected fake create progress promise to be blocked")
	}
	if !strings.Contains(reply, "没有后台异步任务") || !strings.Contains(reply, "实际执行") {
		t.Fatalf("expected honest execution correction, got: %s", reply)
	}
}

func TestBuildUnifiedTurnRouterPromptNamesContextPolicy(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), nil)
	systemPrompt, userPrompt := a.buildUnifiedTurnRouterPrompt(42, "zh", "不是交易员，是策略")
	for _, want := range []string{
		"context_mode values",
		"fresh_context",
		"downstream modules",
		"tasks format",
		"skill_tasks",
		"topic_intent as the primary decision",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected system prompt to contain %q", want)
		}
	}
	if !strings.Contains(userPrompt, "不是交易员，是策略") {
		t.Fatalf("expected user prompt to contain current user message")
	}
}
