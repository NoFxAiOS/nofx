package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"nofx/mcp"
)

func TestIsConfigOrTraderIntent(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "帮我创建一个交易员", want: true},
		{text: "我已经配置好了 OKX 和 DeepSeek", want: true},
		{text: "List my traders", want: true},
		{text: "BTC 接下来怎么看", want: false},
	}
	for _, tc := range cases {
		if got := isConfigOrTraderIntent(tc.text); got != tc.want {
			t.Fatalf("isConfigOrTraderIntent(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestIsRealtimeAccountIntent(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "现在余额多少", want: true},
		{text: "我的仓位还在吗", want: true},
		{text: "show recent trade history", want: true},
		{text: "帮我创建交易员", want: false},
	}
	for _, tc := range cases {
		if got := isRealtimeAccountIntent(tc.text); got != tc.want {
			t.Fatalf("isRealtimeAccountIntent(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestDetectReadFastPath(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{text: "/traders", want: "list_traders"},
		{text: "/strategies", want: "get_strategies"},
		{text: "/models", want: "get_model_configs"},
		{text: "/exchanges", want: "get_exchange_configs"},
		{text: "/balance", want: "get_balance"},
		{text: "/positions", want: "get_positions"},
		{text: "/history", want: "get_trade_history"},
		{text: "/trades", want: "get_trade_history"},
		{text: "列出我当前的策略", want: ""},
		{text: "查看当前交易员", want: ""},
		{text: "现在余额多少", want: ""},
		{text: "我的仓位还在吗", want: ""},
		{text: "我现在有哪些账户", want: ""},
		{text: "我的余额", want: ""},
		{text: "根据我的余额帮我分析我应该买什么", want: ""},
		{text: "我的策略是AI100，但是No candidate coins available, cycle skipped", want: ""},
		{text: "帮我创建一个 trader", want: ""},
	}
	for _, tc := range cases {
		req := detectReadFastPath(tc.text)
		got := ""
		if req != nil {
			got = req.Kind
		}
		if got != tc.want {
			t.Fatalf("detectReadFastPath(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestShouldResetExecutionStateForNewAttempt(t *testing.T) {
	state := ExecutionState{
		SessionID: "sess_1",
		Status:    executionStatusWaitingUser,
	}
	if !shouldResetExecutionStateForNewAttempt("我已经配置好了，继续创建交易员", state) {
		t.Fatalf("expected retry-style config request to reset execution state")
	}
	if shouldResetExecutionStateForNewAttempt("BTC 价格多少", state) {
		t.Fatalf("did not expect generic market query to reset execution state")
	}
}

func TestLatestAskedQuestion(t *testing.T) {
	state := ExecutionState{
		Status: executionStatusWaitingUser,
		Steps: []PlanStep{
			{ID: "step_1", Type: planStepTypeTool, Status: planStepStatusCompleted},
			{ID: "step_2", Type: planStepTypeAskUser, Status: planStepStatusCompleted, Instruction: "需要我用正确的参数重试创建交易员 lky 吗？"},
		},
	}
	got := latestAskedQuestion(state)
	want := "需要我用正确的参数重试创建交易员 lky 吗？"
	if got != want {
		t.Fatalf("latestAskedQuestion() = %q, want %q", got, want)
	}
}

func TestLatestAskedQuestionPrefersStructuredWaitingState(t *testing.T) {
	state := ExecutionState{
		Status: executionStatusWaitingUser,
		Waiting: &WaitingState{
			Question: "请确认是否继续创建交易员 lky",
			Intent:   "confirm_action",
		},
		Steps: []PlanStep{
			{ID: "step_2", Type: planStepTypeAskUser, Status: planStepStatusCompleted, Instruction: "旧问题"},
		},
	}
	if got := latestAskedQuestion(state); got != "请确认是否继续创建交易员 lky" {
		t.Fatalf("latestAskedQuestion() = %q, want structured waiting question", got)
	}
}

func TestRefreshStateForDynamicRequestsAddsFreshSnapshots(t *testing.T) {
	a := newTestAgentWithStore(t)

	_ = a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"custom_api_url":"https://api.openai.com/v1",
		"custom_model_name":"gpt-5-mini"
	}`)
	_ = a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"Main",
		"enabled":true
	}`)

	state := ExecutionState{
		SessionID: "sess_1",
		UserID:    1,
		DynamicSnapshots: []Observation{
			{Kind: "current_model_configs", Summary: "stale"},
		},
		ExecutionLog: []Observation{{Kind: "user_reply", Summary: "continue"}},
	}

	refreshed := a.refreshStateForDynamicRequests("user-1", "帮我创建交易员", state)

	if len(refreshed.DynamicSnapshots) < 3 {
		t.Fatalf("expected refreshed observations to include snapshots, got %+v", refreshed.DynamicSnapshots)
	}

	var foundModel, foundExchange, foundTraders bool
	for _, obs := range refreshed.DynamicSnapshots {
		switch obs.Kind {
		case "current_model_configs":
			foundModel = strings.Contains(obs.RawJSON, "openai")
		case "current_exchange_configs":
			foundExchange = strings.Contains(obs.RawJSON, "okx")
		case "current_traders":
			foundTraders = strings.Contains(obs.RawJSON, `"traders"`)
		}
	}

	if !foundModel || !foundExchange || !foundTraders {
		t.Fatalf("missing fresh snapshots: %+v", refreshed.DynamicSnapshots)
	}
}

func TestRefreshStateForRealtimeAccountRequestsAddsFreshSnapshots(t *testing.T) {
	a := newTestAgentWithStore(t)

	state := ExecutionState{
		SessionID: "sess_2",
		UserID:    1,
		DynamicSnapshots: []Observation{
			{Kind: "current_balances", Summary: "stale balances"},
			{Kind: "current_positions", Summary: "stale positions"},
		},
		ExecutionLog: []Observation{{Kind: "user_reply", Summary: "现在余额多少"}},
	}

	refreshed := a.refreshStateForDynamicRequests("user-1", "现在余额多少，我的仓位还在吗", state)

	var keptBalances, keptPositions, foundHistory bool
	for _, obs := range refreshed.DynamicSnapshots {
		switch obs.Kind {
		case "current_balances":
			keptBalances = strings.Contains(obs.Summary, "stale balances")
		case "current_positions":
			keptPositions = strings.Contains(obs.Summary, "stale positions")
		case "recent_trade_history":
			foundHistory = obs.RawJSON != ""
		}
	}

	if !keptBalances || !keptPositions || foundHistory {
		t.Fatalf("expected realtime snapshots to stay untouched, got %+v", refreshed.DynamicSnapshots)
	}
}

func TestThinkAndActNaturalLanguageReadCanBeHandledByHighLevelSkill(t *testing.T) {
	a := newTestAgentWithStore(t)
	_ = a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"激进",
		"description":"激进策略模板",
		"lang":"zh"
	}`)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 1, "zh", "列出我当前的策略")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "当前策略") || !strings.Contains(resp, "激进") {
		t.Fatalf("expected natural-language read to be handled by high-level skill, got %q", resp)
	}
}

func TestNormalizeExecutionStateMigratesLegacyObservations(t *testing.T) {
	state := normalizeExecutionState(ExecutionState{
		SessionID: "sess_legacy",
		UserID:    1,
		Observations: []Observation{
			{Kind: "tool_result", Summary: "legacy tool result"},
		},
	})

	if len(state.Observations) != 0 {
		t.Fatalf("expected legacy observations field to be cleared, got %+v", state.Observations)
	}
	if len(state.ExecutionLog) != 1 || state.ExecutionLog[0].Summary != "legacy tool result" {
		t.Fatalf("expected legacy observations to migrate into execution log, got %+v", state.ExecutionLog)
	}
}

func TestBuildWaitingStateForTraderConfirmation(t *testing.T) {
	state := ExecutionState{Goal: "创建交易员 lky"}
	step := PlanStep{
		ID:                   "step_ask_1",
		Type:                 planStepTypeAskUser,
		Instruction:          "需要我用正确的参数重试创建交易员 lky 吗？",
		RequiresConfirmation: true,
	}

	waiting := buildWaitingState(state, step, step.Instruction)
	if waiting == nil {
		t.Fatal("expected waiting state")
	}
	if waiting.Intent != "confirm_action" {
		t.Fatalf("unexpected waiting intent: %+v", waiting)
	}
	if waiting.ConfirmationTarget != "trader" {
		t.Fatalf("unexpected confirmation target: %+v", waiting)
	}
}

func TestNormalizeWaitingStateCleansFields(t *testing.T) {
	state := normalizeExecutionState(ExecutionState{
		SessionID: "sess_waiting",
		UserID:    1,
		Waiting: &WaitingState{
			Question:           "  请提供 strategy_id  ",
			Intent:             "  complete_trader_setup ",
			PendingFields:      []string{" strategy_id ", "strategy_id"},
			ConfirmationTarget: " trader ",
		},
	})

	if state.Waiting == nil {
		t.Fatal("expected normalized waiting state")
	}
	if state.Waiting.Question != "请提供 strategy_id" {
		t.Fatalf("unexpected normalized question: %+v", state.Waiting)
	}
	if len(state.Waiting.PendingFields) != 1 || state.Waiting.PendingFields[0] != "strategy_id" {
		t.Fatalf("unexpected pending fields: %+v", state.Waiting)
	}
	if state.Waiting.ConfirmationTarget != "trader" {
		t.Fatalf("unexpected confirmation target: %+v", state.Waiting)
	}
}

func TestRefreshCurrentReferencesForUserTextMatchesStrategyName(t *testing.T) {
	a := newTestAgentWithStore(t)
	_ = a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"激进",
		"description":"激进策略模板",
		"lang":"zh"
	}`)

	state := newExecutionState(1, "帮我改一下激进这个策略")
	a.refreshCurrentReferencesForUserText("user-1", "帮我改一下激进这个策略", &state)

	if state.CurrentReferences == nil || state.CurrentReferences.Strategy == nil {
		t.Fatalf("expected strategy reference, got %+v", state.CurrentReferences)
	}
	if state.CurrentReferences.Strategy.Name != "激进" {
		t.Fatalf("unexpected strategy reference: %+v", state.CurrentReferences.Strategy)
	}
}

func TestUpdateCurrentReferencesFromToolResultTracksCreatedStrategy(t *testing.T) {
	state := newExecutionState(1, "创建策略")
	changed := updateCurrentReferencesFromToolResult(&state, "manage_strategy", `{
		"status":"ok",
		"action":"create",
		"strategy":{"id":"strategy_1","name":"激进"}
	}`)

	if !changed {
		t.Fatalf("expected reference update to report changed")
	}
	if state.CurrentReferences == nil || state.CurrentReferences.Strategy == nil {
		t.Fatalf("expected strategy reference after tool result, got %+v", state.CurrentReferences)
	}
	if state.CurrentReferences.Strategy.ID != "strategy_1" {
		t.Fatalf("unexpected strategy reference: %+v", state.CurrentReferences.Strategy)
	}
}

func TestShouldAttemptReplan(t *testing.T) {
	state := ExecutionState{
		Steps: []PlanStep{
			{ID: "step_1", Type: planStepTypeTool, Status: planStepStatusCompleted},
			{ID: "step_2", Type: planStepTypeRespond, Status: planStepStatusPending},
		},
	}

	if !shouldAttemptReplan(state, PlanStep{
		Type:          planStepTypeTool,
		ToolName:      "manage_trader",
		ToolArgs:      map[string]any{"action": "create"},
		OutputSummary: `{"status":"ok","action":"create"}`,
	}, false) {
		t.Fatalf("expected create trader step to trigger replan")
	}

	if shouldAttemptReplan(state, PlanStep{
		Type:          planStepTypeTool,
		ToolName:      "get_balance",
		OutputSummary: `{"balances":[]}`,
	}, false) {
		t.Fatalf("did not expect read-only balance step to trigger replan")
	}

	if !shouldAttemptReplan(state, PlanStep{
		Type:          planStepTypeTool,
		ToolName:      "get_balance",
		OutputSummary: `{"error":"ai_model_id is required"}`,
	}, false) {
		t.Fatalf("expected dependency/error result to trigger replan")
	}
}

type failingAIClient struct{}

func (f *failingAIClient) SetAPIKey(string, string, string) {}
func (f *failingAIClient) SetTimeout(_ time.Duration)       {}
func (f *failingAIClient) CallWithMessages(string, string) (string, error) {
	return "", errors.New("unexpected CallWithMessages")
}
func (f *failingAIClient) CallWithRequest(*mcp.Request) (string, error) {
	return "", errors.New("API returned error (status 402): insufficient balance")
}
func (f *failingAIClient) CallWithRequestStream(*mcp.Request, func(string)) (string, error) {
	return "", errors.New("unexpected CallWithRequestStream")
}
func (f *failingAIClient) CallWithRequestFull(*mcp.Request) (*mcp.LLMResponse, error) {
	return nil, errors.New("API returned error (status 402): insufficient balance")
}

type capturePlannerAIClient struct {
	systemPrompt string
	userPrompt   string
}

func (c *capturePlannerAIClient) SetAPIKey(string, string, string) {}
func (c *capturePlannerAIClient) SetTimeout(time.Duration)         {}
func (c *capturePlannerAIClient) CallWithMessages(string, string) (string, error) {
	return "", errors.New("unexpected CallWithMessages")
}
func (c *capturePlannerAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	if len(req.Messages) > 0 {
		c.systemPrompt = req.Messages[0].Content
	}
	if len(req.Messages) > 1 {
		c.userPrompt = req.Messages[1].Content
	}
	return `{"goal":"test goal","steps":[{"id":"step_1","type":"respond","instruction":"ok"}]}`, nil
}
func (c *capturePlannerAIClient) CallWithRequestStream(*mcp.Request, func(string)) (string, error) {
	return "", errors.New("unexpected CallWithRequestStream")
}
func (c *capturePlannerAIClient) CallWithRequestFull(*mcp.Request) (*mcp.LLMResponse, error) {
	return nil, errors.New("unexpected CallWithRequestFull")
}

type blockingAIClient struct{}

func (b *blockingAIClient) SetAPIKey(string, string, string) {}
func (b *blockingAIClient) SetTimeout(time.Duration)         {}
func (b *blockingAIClient) CallWithMessages(string, string) (string, error) {
	return "", errors.New("unexpected CallWithMessages")
}
func (b *blockingAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	<-req.Ctx.Done()
	return "", req.Ctx.Err()
}
func (b *blockingAIClient) CallWithRequestStream(*mcp.Request, func(string)) (string, error) {
	return "", errors.New("unexpected CallWithRequestStream")
}
func (b *blockingAIClient) CallWithRequestFull(*mcp.Request) (*mcp.LLMResponse, error) {
	return nil, errors.New("unexpected CallWithRequestFull")
}

type directReplyAIClient struct {
	lastSystemPrompt  string
	lastUserPrompt    string
	routerPrompt      string
	skillRouterPrompt string
	plannerPrompt     string
}

func (d *directReplyAIClient) SetAPIKey(string, string, string) {}
func (d *directReplyAIClient) SetTimeout(time.Duration)         {}
func (d *directReplyAIClient) CallWithMessages(string, string) (string, error) {
	return "", errors.New("unexpected CallWithMessages")
}
func (d *directReplyAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	if len(req.Messages) > 0 {
		d.lastSystemPrompt = req.Messages[0].Content
	}
	if len(req.Messages) > 1 {
		d.lastUserPrompt = req.Messages[1].Content
	}
	if strings.Contains(d.lastSystemPrompt, "first-pass router for NOFXi") {
		d.routerPrompt = d.lastSystemPrompt
		if strings.Contains(d.lastUserPrompt, "你好") {
			return `{"action":"direct_answer","answer":"你好，我在。想聊策略、配置还是排障？"}`, nil
		}
		return `{"action":"defer","answer":""}`, nil
	}
	if strings.Contains(d.lastSystemPrompt, "lightweight skill router for NOFXi") {
		d.skillRouterPrompt = d.lastSystemPrompt
		if strings.Contains(d.lastUserPrompt, "运行中的trader") || strings.Contains(d.lastUserPrompt, "有没有 trader 在跑") {
			return `{"route":"skill","skill":"trader_management","action":"query","filter":"running_only"}`, nil
		}
		return `{"route":"planner","skill":"","action":"","filter":""}`, nil
	}
	if strings.Contains(d.lastSystemPrompt, "planning module for NOFXi") {
		d.plannerPrompt = d.lastSystemPrompt
	}
	return `{"goal":"test goal","steps":[{"id":"step_1","type":"respond","instruction":"ok"}]}`, nil
}
func (d *directReplyAIClient) CallWithRequestStream(*mcp.Request, func(string)) (string, error) {
	return "", errors.New("unexpected CallWithRequestStream")
}
func (d *directReplyAIClient) CallWithRequestFull(*mcp.Request) (*mcp.LLMResponse, error) {
	return nil, errors.New("unexpected CallWithRequestFull")
}

func TestThinkAndActLegacyReturnsProviderFailureInsteadOfNoAIFallback(t *testing.T) {
	a := &Agent{
		aiClient: &failingAIClient{},
		config:   DefaultConfig(),
		logger:   slog.Default(),
		history:  newChatHistory(10),
	}

	resp, err := a.thinkAndActLegacy(context.Background(), 42, "zh", "你好", nil)
	if err != nil {
		t.Fatalf("thinkAndActLegacy() error = %v", err)
	}
	if strings.Contains(resp, "发送 *开始配置* 配置 AI 模型") {
		t.Fatalf("expected provider failure message, got fallback: %q", resp)
	}
	if !strings.Contains(resp, "AI 服务调用失败") {
		t.Fatalf("expected provider failure message, got %q", resp)
	}
}

func TestThinkAndActUsesDirectReplyGateForConversationalQuestion(t *testing.T) {
	client := &directReplyAIClient{}
	a := &Agent{
		aiClient: client,
		config:   DefaultConfig(),
		logger:   slog.Default(),
		history:  newChatHistory(10),
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 88, "zh", "你好")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "你好，我在") {
		t.Fatalf("expected direct reply response, got %q", resp)
	}
	if !strings.Contains(client.routerPrompt, "first-pass router for NOFXi") {
		t.Fatalf("expected direct reply router prompt, got %q", client.routerPrompt)
	}
}

func TestThinkAndActDefersFromDirectReplyGateToHardSkill(t *testing.T) {
	a := newTestAgentWithStore(t)
	a.aiClient = &directReplyAIClient{}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 89, "zh", "帮我创建一个 DeepSeek 模型配置")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "已创建模型配置") {
		t.Fatalf("expected direct reply gate to defer to hard skill, got %q", resp)
	}
}

func TestThinkAndActUsesLLMSkillRouterForNaturalLanguageTraderQuery(t *testing.T) {
	client := &directReplyAIClient{}
	a := newTestAgentWithStore(t)
	a.aiClient = client
	a.history = newChatHistory(10)

	modelResp := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"custom_api_url":"https://api.openai.com/v1",
		"custom_model_name":"gpt-5-mini"
	}`)
	var modelCreated struct {
		Model safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(modelResp), &modelCreated); err != nil {
		t.Fatalf("unmarshal model response: %v", err)
	}

	exchangeResp := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"binance",
		"account_name":"Main",
		"enabled":true
	}`)
	var exchangeCreated struct {
		Exchange safeExchangeToolConfig `json:"exchange"`
	}
	if err := json.Unmarshal([]byte(exchangeResp), &exchangeCreated); err != nil {
		t.Fatalf("unmarshal exchange response: %v", err)
	}

	createResp := a.toolManageTrader("user-1", `{
		"action":"create",
		"name":"Momentum Trader",
		"ai_model_id":"`+modelCreated.Model.ID+`",
		"exchange_id":"`+exchangeCreated.Exchange.ID+`",
		"scan_interval_minutes":5
	}`)
	var created struct {
		Trader safeTraderToolConfig `json:"trader"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal create trader response: %v\nraw=%s", err, createResp)
	}
	if err := a.store.Trader().UpdateStatus("user-1", created.Trader.ID, true); err != nil {
		t.Fatalf("update trader status: %v", err)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 90, "zh", "当前有运行中的trader吗")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "运行中的交易员") || !strings.Contains(resp, "Momentum Trader") {
		t.Fatalf("expected routed running-trader answer, got %q", resp)
	}
	if client.skillRouterPrompt == "" {
		t.Fatal("expected lightweight skill router prompt to be used")
	}
	if client.plannerPrompt != "" {
		t.Fatalf("expected planner to be skipped, got prompt %q", client.plannerPrompt)
	}
}

func TestThinkAndActPrioritizesActiveExecutionStateOverDirectReply(t *testing.T) {
	client := &directReplyAIClient{}
	a := newTestAgentWithStore(t)
	a.aiClient = client
	a.history = newChatHistory(10)
	a.logger = slog.Default()

	userID := int64(90)
	state := newExecutionState(userID, "继续完成当前任务")
	state.Status = executionStatusWaitingUser
	state.Waiting = &WaitingState{
		Question: "请确认是否继续",
		Intent:   "confirm_action",
	}
	if err := a.saveExecutionState(state); err != nil {
		t.Fatalf("saveExecutionState() error = %v", err)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", userID, "zh", "你好")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if strings.Contains(resp, "你好，我在") {
		t.Fatalf("expected active execution state to bypass direct reply gate, got %q", resp)
	}
	if !strings.Contains(client.plannerPrompt, "planning module for NOFXi") {
		t.Fatalf("expected planner prompt when execution state is active, got %q", client.plannerPrompt)
	}
}

func TestThinkAndActInterruptsWaitingExecutionStateForNewTopic(t *testing.T) {
	a := newTestAgentWithStore(t)
	a.history = newChatHistory(10)

	_ = a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"激进",
		"lang":"zh"
	}`)

	userID := int64(91)
	state := newExecutionState(userID, "创建交易员")
	state.Status = executionStatusWaitingUser
	state.Waiting = &WaitingState{
		Question:      "请告诉我交易员名称",
		PendingFields: []string{"name"},
	}
	if err := a.saveExecutionState(state); err != nil {
		t.Fatalf("saveExecutionState() error = %v", err)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", userID, "zh", "列出我当前的策略")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "当前策略") || !strings.Contains(resp, "激进") {
		t.Fatalf("expected new topic to be handled, got %q", resp)
	}
	if got := a.getExecutionState(userID); got.SessionID != "" {
		t.Fatalf("expected execution state to be cleared, got %+v", got)
	}
}

func TestCreateExecutionPlanIncludesRecentConversation(t *testing.T) {
	client := &capturePlannerAIClient{}
	a := &Agent{
		aiClient: client,
		config:   DefaultConfig(),
		logger:   slog.Default(),
		history:  newChatHistory(10),
	}

	userID := int64(42)
	a.history.Add(userID, "user", "先帮我看一下当前trader")
	a.history.Add(userID, "assistant", "当前只有测试1这个trader。")
	a.history.Add(userID, "user", "好的，那就按当前trader来")

	_, err := a.createExecutionPlan(context.Background(), userID, "zh", "好的，那就按当前trader来", newExecutionState(userID, "好的，那就按当前trader来"))
	if err != nil {
		t.Fatalf("createExecutionPlan() error = %v", err)
	}
	if !strings.Contains(client.userPrompt, "Recent conversation:") {
		t.Fatalf("expected planner prompt to include recent conversation, got %q", client.userPrompt)
	}
	if !strings.Contains(client.userPrompt, "先帮我看一下当前trader") {
		t.Fatalf("expected previous user turn in recent conversation, got %q", client.userPrompt)
	}
	if !strings.Contains(client.userPrompt, "当前只有测试1这个trader") {
		t.Fatalf("expected previous assistant turn in recent conversation, got %q", client.userPrompt)
	}
	recentIdx := strings.Index(client.userPrompt, "Recent conversation:\n")
	toolsIdx := strings.Index(client.userPrompt, "\n\nAvailable tools JSON:")
	if recentIdx == -1 || toolsIdx == -1 || toolsIdx <= recentIdx {
		t.Fatalf("expected recent conversation block boundaries, got %q", client.userPrompt)
	}
	recentBlock := client.userPrompt[recentIdx:toolsIdx]
	if strings.Contains(recentBlock, "好的，那就按当前trader来") {
		t.Fatalf("expected current user text to stay out of recent conversation block, got %q", recentBlock)
	}
	if !strings.Contains(client.systemPrompt, "Memory priority order:") {
		t.Fatalf("expected planner system prompt to include memory priority guidance, got %q", client.systemPrompt)
	}
	if !strings.Contains(client.systemPrompt, "Execution state JSON = current operational truth") {
		t.Fatalf("expected planner system prompt to prioritize execution state, got %q", client.systemPrompt)
	}
	if !strings.Contains(client.systemPrompt, "Do not ask the user to repeat a fact") {
		t.Fatalf("expected planner system prompt to forbid unnecessary repeated questions, got %q", client.systemPrompt)
	}
}

func TestCreateExecutionPlanIncludesRecentConversationForFreshRequest(t *testing.T) {
	client := &capturePlannerAIClient{}
	a := &Agent{
		aiClient: client,
		config:   DefaultConfig(),
		logger:   slog.Default(),
		history:  newChatHistory(10),
	}

	userID := int64(99)
	a.history.Add(userID, "user", "先帮我看一下当前trader")
	a.history.Add(userID, "assistant", "当前只有测试1这个trader。")

	_, err := a.createExecutionPlan(context.Background(), userID, "zh", "帮我分析一下比特币", ExecutionState{})
	if err != nil {
		t.Fatalf("createExecutionPlan() error = %v", err)
	}
	if !strings.Contains(client.userPrompt, "Recent conversation:") {
		t.Fatalf("expected fresh request to still include recent conversation block, got %q", client.userPrompt)
	}
	if !strings.Contains(client.userPrompt, "先帮我看一下当前trader") {
		t.Fatalf("expected previous user turn in recent conversation, got %q", client.userPrompt)
	}
	if !strings.Contains(client.userPrompt, "当前只有测试1这个trader") {
		t.Fatalf("expected previous assistant turn in recent conversation, got %q", client.userPrompt)
	}
}

func TestCreateExecutionPlanIncludesQuotedEarlierAssistantClaim(t *testing.T) {
	client := &capturePlannerAIClient{}
	a := &Agent{
		aiClient: client,
		config:   DefaultConfig(),
		logger:   slog.Default(),
		history:  newChatHistory(10),
	}

	userID := int64(100)
	a.history.Add(userID, "user", "配置页怎么只有三个交易所")
	a.history.Add(userID, "assistant", "目前你看到的是三个交易所。")

	_, err := a.createExecutionPlan(context.Background(), userID, "zh", "你前面也跟我说只有三个交易所", ExecutionState{})
	if err != nil {
		t.Fatalf("createExecutionPlan() error = %v", err)
	}
	if !strings.Contains(client.userPrompt, "目前你看到的是三个交易所") {
		t.Fatalf("expected planner prompt to include earlier assistant claim, got %q", client.userPrompt)
	}
	if !strings.Contains(client.userPrompt, "配置页怎么只有三个交易所") {
		t.Fatalf("expected planner prompt to include earlier user complaint, got %q", client.userPrompt)
	}
}

func TestRunPlannedAgentReturnsTimeoutMessageOnPlannerTimeout(t *testing.T) {
	oldTimeout := plannerCreateTimeout
	plannerCreateTimeout = 10 * time.Millisecond
	defer func() { plannerCreateTimeout = oldTimeout }()

	a := &Agent{
		aiClient: &blockingAIClient{},
		config:   DefaultConfig(),
		logger:   slog.Default(),
		history:  newChatHistory(10),
	}

	resp, err := a.runPlannedAgent(context.Background(), "default", 7, "zh", "帮我分析一下当前市场", nil)
	if err != nil {
		t.Fatalf("runPlannedAgent() error = %v", err)
	}
	if !strings.Contains(resp, "处理超时") {
		t.Fatalf("expected timeout message, got %q", resp)
	}
}

func TestHandleMessageForStoreUserBypassesPlannerForTradeConfirmation(t *testing.T) {
	a := &Agent{
		config:  DefaultConfig(),
		logger:  slog.Default(),
		history: newChatHistory(10),
		pending: newPendingTrades(),
	}

	resp, err := a.handleMessageForStoreUser(context.Background(), "default", 1, "确认 trade_missing")
	if err != nil {
		t.Fatalf("handleMessageForStoreUser() error = %v", err)
	}
	if !strings.Contains(resp, "交易已过期或不存在") {
		t.Fatalf("expected direct trade confirmation handling, got %q", resp)
	}
}

func TestResolveModelRuntimeConfigUsesProviderDefaults(t *testing.T) {
	url, model := resolveModelRuntimeConfig("deepseek", "", "", "user_deepseek")
	if url != "https://api.deepseek.com/v1" {
		t.Fatalf("unexpected deepseek default url: %q", url)
	}
	if model != "deepseek-chat" {
		t.Fatalf("unexpected deepseek default model: %q", model)
	}

	url, model = resolveModelRuntimeConfig("deepseek", "", "deepseek1", "user_deepseek")
	if url != "https://api.deepseek.com/v1" {
		t.Fatalf("unexpected resolved url: %q", url)
	}
	if model != "deepseek1" {
		t.Fatalf("expected existing custom model name to win, got %q", model)
	}
}
