package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nofx/mcp"
	"nofx/store"
)

type staticAIClient struct {
	response    string
	lastRequest *mcp.Request
}

func (c *staticAIClient) SetAPIKey(apiKey string, customURL string, customModel string) {}
func (c *staticAIClient) SetTimeout(timeout time.Duration)                              {}
func (c *staticAIClient) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	return c.response, nil
}
func (c *staticAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	c.lastRequest = req
	return c.response, nil
}
func (c *staticAIClient) CallWithRequestStream(req *mcp.Request, onChunk func(string)) (string, error) {
	c.lastRequest = req
	if onChunk != nil {
		onChunk(c.response)
	}
	return c.response, nil
}
func (c *staticAIClient) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	c.lastRequest = req
	return &mcp.LLMResponse{Content: c.response}, nil
}

func TestClassifyWorkflowTaskTreatsTraderEditAsManualPanelUpdate(t *testing.T) {
	task, ok := classifyWorkflowTask("帮我把交易员小爱换策略")
	if !ok {
		t.Fatal("expected trader binding edit to classify")
	}
	if task.Skill != "trader_management" || task.Action != "update_bindings" {
		t.Fatalf("unexpected task: %+v", task)
	}

	task, ok = classifyWorkflowTask("帮我把交易员小爱扫描间隔改成10分钟")
	if !ok {
		t.Fatal("expected trader manual-panel edit to classify")
	}
	if task.Skill != "trader_management" || task.Action != "update_bindings" {
		t.Fatalf("unexpected trader update task: %+v", task)
	}
}

func TestGetDecisionsToolReturnsRecentTraderDecisionEvidence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "decision-evidence.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	traderCfg := &store.Trader{
		ID:                  "trader-claw402",
		UserID:              "default",
		Name:                "claw402",
		AIModelID:           "model-1",
		ExchangeID:          "exchange-1",
		InitialBalance:      6.21,
		ScanIntervalMinutes: 3,
		IsRunning:           true,
	}
	if err := st.Trader().Create(traderCfg); err != nil {
		t.Fatalf("seed trader: %v", err)
	}
	if err := st.Decision().LogDecision(&store.DecisionRecord{
		TraderID:            traderCfg.ID,
		CycleNumber:         150,
		Timestamp:           time.Now().Add(-3 * time.Minute),
		Success:             true,
		AIRequestDurationMs: 12095,
		CandidateCoins:      []string{"BTCUSDT"},
		ExecutionLog:        []string{"AI call duration: 12095 ms", "✓ BTCUSDT wait succeeded"},
		Decisions: []store.DecisionAction{{
			Symbol:  "BTCUSDT",
			Action:  "wait",
			Success: true,
		}},
	}); err != nil {
		t.Fatalf("seed wait decision: %v", err)
	}
	if err := st.Decision().LogDecision(&store.DecisionRecord{
		TraderID:            traderCfg.ID,
		CycleNumber:         151,
		Timestamp:           time.Now(),
		Success:             false,
		ErrorMessage:        "Failed to get AI decision: failed to parse AI response: decision validation failed: decision #1 validation failed: BTCUSDT opening amount too small (28.00 USDT), must be ≥60.00 USDT",
		AIRequestDurationMs: 25878,
		CandidateCoins:      []string{"BTCUSDT"},
		ExecutionLog:        []string{"AI call duration: 25878 ms"},
		DecisionJSON:        `[{"symbol":"BTCUSDT","action":"open_short","position_size_usd":28}]`,
	}); err != nil {
		t.Fatalf("seed rejected decision: %v", err)
	}

	raw := a.toolGetDecisions("default", `{"trader_name":"claw402","limit":2}`)
	for _, want := range []string{"claw402", "BTCUSDT", "wait", "wait succeeded", "opening amount too small", "must be ≥60.00 USDT"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("expected decision evidence %q in tool response, got: %s", want, raw)
		}
	}
}

func TestTraderDiagnosisReadsDecisionsInsteadOfAskingUserForScreenshot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trader-diagnosis-decisions.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	traderCfg := &store.Trader{
		ID:                  "trader-claw402",
		UserID:              "default",
		Name:                "claw402",
		AIModelID:           "model-1",
		ExchangeID:          "exchange-1",
		InitialBalance:      6.21,
		ScanIntervalMinutes: 3,
		IsRunning:           true,
	}
	if err := st.Trader().Create(traderCfg); err != nil {
		t.Fatalf("seed trader: %v", err)
	}
	if err := st.Decision().LogDecision(&store.DecisionRecord{
		TraderID:            traderCfg.ID,
		CycleNumber:         1,
		Timestamp:           time.Now(),
		Success:             true,
		AIRequestDurationMs: 13249,
		CandidateCoins:      []string{"BTCUSDT"},
		ExecutionLog:        []string{"AI call duration: 13249 ms", "✓ BTCUSDT wait succeeded"},
		Decisions: []store.DecisionAction{{
			Symbol:  "BTCUSDT",
			Action:  "wait",
			Success: true,
		}},
	}); err != nil {
		t.Fatalf("seed decision: %v", err)
	}

	reply := a.handleTraderDiagnosisSkill("default", "zh", "为什么我的claw402交易员一直不开单呢")
	for _, want := range []string{"claw402 是运行的", "主动选择等待", "入场标准", "该怎么办"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected diagnosis to include %q, got: %s", want, reply)
		}
	}
	for _, unexpected := range []string{"截图", "自己点", "不能直接帮你查", "诊断证据包", "AI 调用耗时", "status 402", "404", "EOF", "订阅"} {
		if strings.Contains(reply, unexpected) {
			t.Fatalf("diagnosis should not ask user to self-serve %q, got: %s", unexpected, reply)
		}
	}
}

func TestTraderDiagnosisAmountTooSmallUsesUserFacingCauseAndAction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trader-diagnosis-amount-too-small.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	traderCfg := &store.Trader{
		ID:                  "trader-claw402",
		UserID:              "default",
		Name:                "claw402",
		AIModelID:           "model-1",
		ExchangeID:          "exchange-1",
		InitialBalance:      6.21,
		ScanIntervalMinutes: 3,
		IsRunning:           true,
	}
	if err := st.Trader().Create(traderCfg); err != nil {
		t.Fatalf("seed trader: %v", err)
	}
	if err := st.Decision().LogDecision(&store.DecisionRecord{
		TraderID:            traderCfg.ID,
		CycleNumber:         2,
		Timestamp:           time.Now(),
		Success:             false,
		ErrorMessage:        "Failed to get AI decision: failed to parse AI response: decision validation failed: decision #1 validation failed: BTCUSDT opening amount too small (28.00 USDT), must be ≥60.00 USDT",
		AIRequestDurationMs: 25878,
		CandidateCoins:      []string{"BTCUSDT"},
		ExecutionLog:        []string{"AI call duration: 25878 ms"},
		DecisionJSON:        `[{"symbol":"BTCUSDT","action":"open_short","position_size_usd":28}]`,
	}); err != nil {
		t.Fatalf("seed decision: %v", err)
	}

	reply := a.handleTraderDiagnosisSkill("default", "zh", "为什么我的claw402交易员一直不开单呢")
	for _, want := range []string{"不是没运行", "账户资金太小", "开仓金额约 28.00 USDT", "最小下单要求 60.00 USDT", "增加账户资金", "不能手动修改"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected diagnosis to include %q, got: %s", want, reply)
		}
	}
	for _, unexpected := range []string{"诊断证据包", "辅助异常", "status 402", "404", "EOF", "订阅", "数据服务", "position_size_usd", "AI 调用耗时"} {
		if strings.Contains(reply, unexpected) {
			t.Fatalf("diagnosis should stay user-facing and avoid %q, got: %s", unexpected, reply)
		}
	}
}

func TestTraderDiagnosisUsesLLMToReasonOverCollectedEvidence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trader-diagnosis-llm.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	llm := &staticAIClient{response: "claw402 的最终原因是账户资金太小，最近想开 BTCUSDT 空单但金额低于最小下单要求。该怎么办：增加账户资金，或换更适合小资金的策略/标的。"}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(llm)
	traderCfg := &store.Trader{
		ID:                  "trader-claw402",
		UserID:              "default",
		Name:                "claw402",
		AIModelID:           "model-1",
		ExchangeID:          "exchange-1",
		InitialBalance:      6.21,
		ScanIntervalMinutes: 3,
		IsRunning:           true,
	}
	if err := st.Trader().Create(traderCfg); err != nil {
		t.Fatalf("seed trader: %v", err)
	}
	if err := st.Decision().LogDecision(&store.DecisionRecord{
		TraderID:       traderCfg.ID,
		CycleNumber:    3,
		Timestamp:      time.Now(),
		Success:        false,
		ErrorMessage:   "BTCUSDT opening amount too small (28.00 USDT), must be ≥60.00 USDT",
		CandidateCoins: []string{"BTCUSDT"},
		DecisionJSON:   `[{"symbol":"BTCUSDT","action":"open_short","position_size_usd":28}]`,
	}); err != nil {
		t.Fatalf("seed decision: %v", err)
	}

	reply := a.handleTraderDiagnosisSkill("default", "zh", "为什么我的claw402交易员一直不开单呢")
	if reply != llm.response {
		t.Fatalf("expected LLM diagnosis response, got: %s", reply)
	}
	if llm.lastRequest == nil || len(llm.lastRequest.Messages) < 2 {
		t.Fatalf("expected LLM request to be captured")
	}
	prompt := llm.lastRequest.Messages[1].Content
	for _, want := range []string{"Evidence JSON", "claw402", "BTCUSDT", "opening amount too small", "decision_json"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected LLM evidence prompt to include %q, got: %s", want, prompt)
		}
	}
}

func TestTraderDomainPrimerExplainsInternalConfigBoundary(t *testing.T) {
	primer := buildSkillDomainPrimer("zh", "trader_management")
	for _, want := range []string{
		"交易员是装配层",
		"默认只处理绑定关系",
		"应切到对应 management skill",
	} {
		if !strings.Contains(primer, want) {
			t.Fatalf("expected primer to contain %q, got: %s", want, primer)
		}
	}
}

func TestStrategyDomainPrimerKeepsSourceCountsWithinEditorBounds(t *testing.T) {
	primer := buildSkillDomainPrimerForSession("zh", skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"strategy_type": "ai_trading",
		},
	})
	for _, want := range []string{
		"AI500/OI Top/OI Low 选币数量范围 1～10",
		"没有 mixed/混合模式",
		"BTC/ETH 最大杠杆 1～20",
		"min_confidence 50～100",
	} {
		if !strings.Contains(primer, want) {
			t.Fatalf("expected primer to contain %q, got: %s", want, primer)
		}
	}
}

func TestStrategyConfigSchemaOnlyExposesEditorCoinSourceFields(t *testing.T) {
	schema := strategyConfigSchema()
	properties := schema["properties"].(map[string]any)
	aiConfig := properties["ai_config"].(map[string]any)
	aiProperties := aiConfig["properties"].(map[string]any)
	coinSource := aiProperties["coin_source"].(map[string]any)
	coinProperties := coinSource["properties"].(map[string]any)
	for _, unexpected := range []string{"use_hyper_all", "use_hyper_main", "hyper_main_limit"} {
		if _, ok := coinProperties[unexpected]; ok {
			t.Fatalf("strategy config schema should not expose non-editor coin source field %s", unexpected)
		}
	}
	ai500 := coinProperties["ai500_limit"].(map[string]any)
	if ai500["maximum"] != 10 {
		t.Fatalf("expected AI500 maximum 10, got %+v", ai500)
	}
}

func TestLoadEnabledModelOptionsUseConfigNameAsPrimaryLabel(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trader-model-options.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	if err := st.AIModel().UpdateWithName("default", "default_deepseek", "DeepSeek AI", true, "sk-test-12345", "", "deepseek-chat"); err != nil {
		t.Fatalf("seed model: %v", err)
	}

	options := a.loadEnabledModelOptions("default")
	if len(options) != 1 {
		t.Fatalf("expected one model option, got %d", len(options))
	}
	if options[0].Name != "DeepSeek AI" {
		t.Fatalf("expected primary option label to stay on config name, got %q", options[0].Name)
	}
	if !strings.Contains(options[0].Hint, "deepseek-chat") || !strings.Contains(options[0].Hint, "deepseek") {
		t.Fatalf("expected hint to retain runtime model/provider context, got %q", options[0].Hint)
	}
}

func TestHydrateCreateTraderSlotReferencesNormalizesModelIDFromVisibleName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trader-model-id-normalize.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	if err := st.AIModel().UpdateWithName("default", "default_deepseek", "DeepSeek AI", true, "sk-test-12345", "", "deepseek-chat"); err != nil {
		t.Fatalf("seed model: %v", err)
	}

	session := skillSession{
		Name:   "trader_management",
		Action: "create",
		Fields: map[string]string{
			"model_id": "DeepSeek AI",
		},
	}
	a.hydrateCreateTraderSlotReferences("default", &session)
	if got := fieldValue(session, "model_id"); got != "default_deepseek" {
		t.Fatalf("expected visible model name in model_id slot to normalize to actual id, got %q", got)
	}
	if got := fieldValue(session, "model_name"); got != "DeepSeek AI" {
		t.Fatalf("expected normalized model name to be preserved, got %q", got)
	}
}

func TestHydrateCreateTraderSlotReferencesNormalizesExchangeIDFromVisibleName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trader-exchange-id-normalize.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	exchangeID, err := st.Exchange().Create("default", "okx", "小偶", true, "api-test", "secret-test", "pass", false, "", false, false, "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatalf("seed exchange: %v", err)
	}

	session := skillSession{
		Name:   "trader_management",
		Action: "create",
		Fields: map[string]string{
			"exchange_id": "小偶",
		},
	}
	a.hydrateCreateTraderSlotReferences("default", &session)
	if got := fieldValue(session, "exchange_id"); got != exchangeID {
		t.Fatalf("expected visible exchange name in exchange_id slot to normalize to actual id, got %q", got)
	}
	if got := fieldValue(session, "exchange_name"); got != "小偶" {
		t.Fatalf("expected normalized exchange name to be preserved, got %q", got)
	}
}

func TestToolDeleteTraderRejectsRunningTrader(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "delete-running-trader.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	if err := st.Trader().Create(&store.Trader{
		ID:                  "trader-running",
		UserID:              "default",
		Name:                "运行中",
		AIModelID:           "model-1",
		ExchangeID:          "exchange-1",
		InitialBalance:      100,
		ScanIntervalMinutes: 3,
		IsRunning:           true,
	}); err != nil {
		t.Fatalf("seed trader: %v", err)
	}

	resp := a.toolDeleteTrader("default", "trader-running")
	if !strings.Contains(resp, "stop it before deleting") {
		t.Fatalf("expected running trader delete to be rejected, got: %s", resp)
	}
	traders, err := st.Trader().List("default")
	if err != nil {
		t.Fatalf("list traders: %v", err)
	}
	if len(traders) != 1 {
		t.Fatalf("expected running trader to remain, got %d traders", len(traders))
	}
}

func TestBulkTraderDeleteDeletesOnlyStoppedTraders(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bulk-delete-traders.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	for _, trader := range []*store.Trader{
		{ID: "trader-stopped", UserID: "default", Name: "已停止", AIModelID: "model-1", ExchangeID: "exchange-1", InitialBalance: 100, ScanIntervalMinutes: 3, IsRunning: false},
		{ID: "trader-running", UserID: "default", Name: "运行中", AIModelID: "model-1", ExchangeID: "exchange-1", InitialBalance: 100, ScanIntervalMinutes: 3, IsRunning: true},
	} {
		if err := st.Trader().Create(trader); err != nil {
			t.Fatalf("seed trader %s: %v", trader.ID, err)
		}
	}

	session := skillSession{
		Name:   "trader_management",
		Action: "delete",
		Phase:  "await_confirmation",
		Fields: map[string]string{
			"bulk_scope":      "all",
			skillDAGStepField: "await_confirmation",
		},
	}
	resp := a.executeBulkTraderDelete("default", 99, "zh", "确认", session)
	if !strings.Contains(resp, "成功删除 1 个") || !strings.Contains(resp, "运行中") {
		t.Fatalf("expected stopped trader deleted and running trader skipped, got: %s", resp)
	}
	traders, err := st.Trader().List("default")
	if err != nil {
		t.Fatalf("list traders: %v", err)
	}
	if len(traders) != 1 || traders[0].ID != "trader-running" {
		t.Fatalf("expected only running trader to remain, got: %+v", traders)
	}
}

func TestBulkTraderDeleteRequiresConfirmationBeforeDeleting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bulk-delete-traders-confirmation.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	if err := st.Trader().Create(&store.Trader{
		ID:                  "trader-stopped",
		UserID:              "default",
		Name:                "已停止",
		AIModelID:           "model-1",
		ExchangeID:          "exchange-1",
		InitialBalance:      100,
		ScanIntervalMinutes: 3,
		IsRunning:           false,
	}); err != nil {
		t.Fatalf("seed trader: %v", err)
	}

	session := skillSession{
		Name:   "trader_management",
		Action: "delete",
		Fields: map[string]string{
			"bulk_scope": "all",
		},
	}
	resp := a.executeBulkTraderDelete("default", 99, "zh", "全部删除", session)
	if !strings.Contains(resp, "请回复“确认”继续") {
		t.Fatalf("expected confirmation prompt, got: %s", resp)
	}
	traders, err := st.Trader().List("default")
	if err != nil {
		t.Fatalf("list traders: %v", err)
	}
	if len(traders) != 1 {
		t.Fatalf("expected trader to remain before confirmation, got %d traders", len(traders))
	}
}

func TestResolveTargetSelectionMatchesUniqueNameInUserText(t *testing.T) {
	options := []traderSkillOption{
		{ID: "exchange-a", Name: "okx"},
		{ID: "exchange-b", Name: "为：小易"},
		{ID: "exchange-c", Name: "小偶"},
	}
	resolved := resolveTargetSelection("先把 为：小易 删掉，其他 5 个先保留", options, nil)
	if resolved.Ref == nil {
		t.Fatal("expected target ref to resolve from user text")
	}
	if resolved.Ref.ID != "exchange-b" || resolved.Ref.Name != "为：小易" {
		t.Fatalf("unexpected resolved target: %+v", resolved.Ref)
	}
}

func TestStrategyUpdateUsesExplicitTargetOverCurrentReference(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-explicit-target-over-current.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	userID := int64(99)

	cfg := store.GetDefaultStrategyConfig("zh")
	rawCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal strategy config: %v", err)
	}
	for _, strategy := range []*store.Strategy{
		{ID: "strategy-short", UserID: "default", Name: "BTC趋势做空", ConfigVisible: true, Config: string(rawCfg)},
		{ID: "strategy-long", UserID: "default", Name: "AI500 做多策略", ConfigVisible: true, Config: string(rawCfg)},
	} {
		if err := st.Strategy().Create(strategy); err != nil {
			t.Fatalf("seed strategy %s: %v", strategy.ID, err)
		}
	}
	a.saveReferenceMemory(userID, &CurrentReferences{
		Strategy: &EntityReference{ID: "strategy-short", Name: "BTC趋势做空", Source: "tool_output"},
	}, nil)

	patch := map[string]any{
		"coin_source": map[string]any{
			"source_type": "ai500",
			"use_ai500":   true,
			"ai500_limit": 5,
		},
		"custom_prompt": "AI500 强做多策略：只寻找强趋势多头机会。",
	}
	rawPatch, _ := json.Marshal(patch)
	session := skillSession{
		Name:   "strategy_management",
		Action: "update_config",
		Phase:  "collecting",
		Fields: map[string]string{strategyCreateConfigPatchField: string(rawPatch)},
	}

	reply, handled := a.handleSimpleEntitySkill(
		"default",
		userID,
		"zh",
		"我想基于AI500 做多策略来调整成更强的做多逻辑",
		session,
		"strategy_management",
		"update_config",
		a.loadStrategyOptions("default"),
	)
	if !handled {
		t.Fatalf("expected handler to handle request")
	}
	if !strings.Contains(reply, "已更新策略配置") {
		t.Fatalf("expected strategy update reply, got: %s", reply)
	}

	shortStrategy, err := st.Strategy().Get("default", "strategy-short")
	if err != nil {
		t.Fatalf("load short strategy: %v", err)
	}
	longStrategy, err := st.Strategy().Get("default", "strategy-long")
	if err != nil {
		t.Fatalf("load long strategy: %v", err)
	}
	if strings.Contains(shortStrategy.Config, "强做多") {
		t.Fatalf("current reference strategy was incorrectly updated: %s", shortStrategy.Config)
	}
	if !strings.Contains(longStrategy.Config, "强做多") {
		t.Fatalf("explicitly named strategy was not updated: %s", longStrategy.Config)
	}
}

func TestStrategyUpdateDoesNotInferTargetFromCurrentReference(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-no-current-reference-fallback.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	userID := int64(100)

	cfg := store.GetDefaultStrategyConfig("zh")
	rawCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal strategy config: %v", err)
	}
	if err := st.Strategy().Create(&store.Strategy{
		ID:            "strategy-short",
		UserID:        "default",
		Name:          "BTC趋势做空",
		ConfigVisible: true,
		Config:        string(rawCfg),
	}); err != nil {
		t.Fatalf("seed strategy: %v", err)
	}
	a.saveReferenceMemory(userID, &CurrentReferences{
		Strategy: &EntityReference{ID: "strategy-short", Name: "BTC趋势做空", Source: "tool_output"},
	}, nil)

	patch := map[string]any{"custom_prompt": "不应被写入"}
	rawPatch, _ := json.Marshal(patch)
	session := skillSession{
		Name:   "strategy_management",
		Action: "update_config",
		Phase:  "collecting",
		Fields: map[string]string{strategyCreateConfigPatchField: string(rawPatch)},
	}

	reply, handled := a.handleSimpleEntitySkill(
		"default",
		userID,
		"zh",
		"帮我把策略改强一点",
		session,
		"strategy_management",
		"update_config",
		a.loadStrategyOptions("default"),
	)
	if !handled {
		t.Fatalf("expected handler to ask for target")
	}
	if !strings.Contains(reply, "确定目标对象") && !strings.Contains(reply, "明确要操作的是哪一个对象") {
		t.Fatalf("expected target clarification, got: %s", reply)
	}
	strategy, err := st.Strategy().Get("default", "strategy-short")
	if err != nil {
		t.Fatalf("load strategy: %v", err)
	}
	if strings.Contains(strategy.Config, "不应被写入") {
		t.Fatalf("strategy was incorrectly updated through current reference fallback: %s", strategy.Config)
	}
}

func TestBulkStrategyDeleteRequiresConfirmationBeforeDeleting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bulk-delete-strategies-confirmation.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	cfg := store.GetDefaultStrategyConfig("zh")
	rawCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal strategy config: %v", err)
	}
	if err := st.Strategy().Create(&store.Strategy{
		ID:            "strategy-custom",
		UserID:        "default",
		Name:          "自定义策略",
		ConfigVisible: true,
		Config:        string(rawCfg),
	}); err != nil {
		t.Fatalf("seed strategy: %v", err)
	}

	session := skillSession{
		Name:   "strategy_management",
		Action: "delete",
		Fields: map[string]string{
			"bulk_scope": "all",
		},
	}
	resp := a.executeStrategyManagementAction("default", 99, "zh", "全部删除", session)
	if !strings.Contains(resp, "请回复“确认”继续") {
		t.Fatalf("expected confirmation prompt, got: %s", resp)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	found := false
	for _, strategy := range strategies {
		if strategy.ID == "strategy-custom" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected strategy to remain before confirmation")
	}
}

func TestEnsureLiveTargetReferenceFallsBackFromStaleIDToName(t *testing.T) {
	session := skillSession{
		TargetRef: &EntityReference{
			ID:   "stale-id",
			Name: "小易",
		},
	}
	options := []traderSkillOption{
		{ID: "exchange-a", Name: "okx"},
		{ID: "exchange-b", Name: "为：小易"},
	}
	if !ensureLiveTargetReference(&session, options) {
		t.Fatal("expected stale id with matching name to resolve")
	}
	if session.TargetRef == nil || session.TargetRef.ID != "exchange-b" || session.TargetRef.Name != "为：小易" {
		t.Fatalf("unexpected target ref after live check: %+v", session.TargetRef)
	}
}

func TestBuildTraderCreateMissingPromptListsAllMissingSlots(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trader-create-missing-prompt.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	if err := st.AIModel().UpdateWithName("default", "default_deepseek", "DeepSeek AI", true, "sk-test-12345", "", "deepseek-chat"); err != nil {
		t.Fatalf("seed model: %v", err)
	}
	exchangeID, err := st.Exchange().Create("default", "okx", "OKX 主账户", true, "api-test", "secret-test", "pass", false, "", false, false, "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatalf("seed exchange: %v", err)
	}
	_ = exchangeID
	cfg := store.GetDefaultStrategyConfig("zh")
	rawCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal strategy config: %v", err)
	}
	if err := st.Strategy().Create(&store.Strategy{
		ID:            "strategy-ai500",
		UserID:        "default",
		Name:          "AI500稳重策略",
		Description:   "test",
		IsPublic:      false,
		ConfigVisible: true,
		Config:        string(rawCfg),
	}); err != nil {
		t.Fatalf("seed strategy: %v", err)
	}

	session := skillSession{
		Name:   "trader_management",
		Action: "create",
		Phase:  "collecting",
		Fields: map[string]string{},
	}
	prompt := a.buildTraderCreateMissingPrompt("default", "zh", session, a.buildTraderCreateConversationResources("default", session))
	for _, want := range []string{"名称", "交易所", "模型", "策略"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected missing prompt to include %q, got: %s", want, prompt)
		}
	}
	for _, want := range []string{"现有交易所", "现有模型", "现有策略"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected missing prompt to include options line %q, got: %s", want, prompt)
		}
	}
}

func TestTraderCreateRequiresResolvedResourceIDs(t *testing.T) {
	session := skillSession{
		Name:   "trader_management",
		Action: "create",
		Fields: map[string]string{
			"name":          "凯茵",
			"exchange_name": "Binance",
			"model_name":    "deepseek",
			"strategy_name": "BTC趋势做空",
		},
	}

	missing := missingFieldKeysForSkillSession(session)
	for _, want := range []string{"exchange_name", "model_name", "strategy_name"} {
		if !containsString(missing, want) {
			t.Fatalf("expected unresolved %s to remain missing, got %v", want, missing)
		}
	}

	active := ActiveSkillSession{
		SkillName:  "trader_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":          "凯茵",
			"exchange_name": "Binance",
			"model_name":    "deepseek",
			"strategy_name": "BTC趋势做空",
		},
	}
	activeMissing := missingRequiredFields(active)
	for _, want := range []string{"exchange", "model", "strategy"} {
		if !containsString(activeMissing, want) {
			t.Fatalf("expected unresolved active slot %s to remain missing, got %v", want, activeMissing)
		}
	}
}

func TestStrategyCreateUsesConfigPatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-config-patch.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	patch := map[string]any{
		"strategy_type": "ai_trading",
		"coin_source": map[string]any{
			"source_type":  "static",
			"static_coins": []any{"BTCUSDT"},
			"use_ai500":    false,
			"use_oi_low":   true,
			"oi_low_limit": 1,
		},
		"risk_control": map[string]any{
			"max_positions":         1,
			"btc_eth_max_leverage":  5,
			"altcoin_max_leverage":  5,
			"min_confidence":        80,
			"min_risk_reward_ratio": 3,
		},
		"indicators": map[string]any{
			"klines": map[string]any{
				"primary_timeframe":   "5m",
				"selected_timeframes": []any{"5m", "15m"},
			},
		},
		"prompt_sections": map[string]any{
			"trading_frequency": "每天最多 2-4 笔，避免过度交易。",
			"entry_standards":   "只在 BTC 下跌趋势确认时考虑做空，禁止把做多作为主方向。",
		},
		"custom_prompt": "BTC 趋势做空策略：仅关注 BTCUSDT，趋势向下且反弹受阻时才考虑开空。",
	}
	rawPatch, _ := json.Marshal(patch)
	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name":                         "BTC趋势做空",
			strategyCreateConfigPatchField: string(rawPatch),
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "确认创建", session)
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected created reply, got: %s", reply)
	}

	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	var created *store.Strategy
	for _, strategy := range strategies {
		if strategy.Name == "BTC趋势做空" {
			created = strategy
			break
		}
	}
	if created == nil {
		t.Fatalf("expected strategy to be created")
	}

	var cfg store.StrategyConfig
	if err := json.Unmarshal([]byte(created.Config), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.CoinSource.SourceType != "static" || len(cfg.CoinSource.StaticCoins) != 1 || cfg.CoinSource.StaticCoins[0] != "BTCUSDT" {
		t.Fatalf("expected BTC static coin source, got %+v", cfg.CoinSource)
	}
	if cfg.CoinSource.UseAI500 {
		t.Fatalf("expected AI500 disabled for explicit BTC strategy")
	}
	if cfg.CoinSource.UseOILow {
		t.Fatalf("expected OI low disabled when source_type is static, got %+v", cfg.CoinSource)
	}
	if cfg.RiskControl.MaxPositions != 3 || cfg.RiskControl.MinConfidence != 80 {
		t.Fatalf("expected risk patch to apply, got %+v", cfg.RiskControl)
	}
	if !strings.Contains(cfg.CustomPrompt, "BTC 趋势做空") || !strings.Contains(cfg.PromptSections.EntryStandards, "做空") {
		t.Fatalf("expected prompt patch to apply, got custom=%q entry=%q", cfg.CustomPrompt, cfg.PromptSections.EntryStandards)
	}
}

func TestAIStrategySystemEnforcedFieldsAreDisplayedButNotEditable(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("zh")
	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name": "我的AI策略",
		},
	}
	reply := formatStrategyCreateFinalConfirmation("zh", session, cfg)
	for _, want := range []string{"最大持仓数（System enforced）", "BTC/ETH 单币仓位上限（System enforced）", "最大保证金使用率（System enforced）", "最小开仓金额（System enforced）"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected final summary to display %q, got: %s", want, reply)
		}
	}

	resp := applyStrategyConfigPatch(&cfg, "max_margin_usage", "0.5")
	if resp == nil || !strings.Contains(resp.Error(), "System enforced") {
		t.Fatalf("expected system enforced edit to be rejected, got: %v", resp)
	}
}

func TestStrategyCreateNaturalLanguageDoesNotBypassTemplateType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-draft-two-turn.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	active := ActiveSkillSession{
		SessionID:  "as_test",
		UserID:     1,
		SkillName:  "strategy_management",
		ActionName: "create",
		Goal:       "真的去创建一个趋势策略，交易BTC和ETH，15m，杠杆 5 倍",
		CollectedFields: map[string]any{
			"name": "BTCETH_15m_趋势",
		},
		LocalHistory: []chatMessage{
			{Role: "user", Content: "真的去创建一个趋势策略，交易BTC和ETH，15m，杠杆 5 倍"},
			{Role: "assistant", Content: "现在只差一个名称。"},
			{Role: "user", Content: "BTCETH_15m_趋势"},
		},
	}
	session := activeToLegacySkillSession(active)
	reply := a.handleStrategyCreateSkill("default", 1, "zh", "BTCETH_15m_趋势", session)
	if !strings.Contains(reply, "先选择策略类型") {
		t.Fatalf("expected strategy type question instead of legacy natural-language parsing, got: %s", reply)
	}

	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	if len(strategies) != 0 {
		t.Fatalf("expected no strategy before template is complete, got %d", len(strategies))
	}
}

func TestStrategyCreateAsksTypeBeforeUsingDefaultTemplateType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-ask-type.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name": "我的策略",
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "我的策略", session)
	if !strings.Contains(reply, "先选择策略类型") || strings.Contains(reply, "交易所") {
		t.Fatalf("expected strategy type question without exchange binding, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	for _, strategy := range strategies {
		if strategy.Name == "我的策略" {
			t.Fatalf("strategy should not be created before type is confirmed")
		}
	}
}

func TestStrategyCreateConfirmationStillRequiresType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-confirm-no-type.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name": "我的策略",
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "确认创建", session)
	if !strings.Contains(reply, "先选择策略类型") {
		t.Fatalf("expected type question before create, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	for _, strategy := range strategies {
		if strategy.Name == "我的策略" {
			t.Fatalf("strategy should not be created before type is known")
		}
	}
}

func TestStrategyCreateStandaloneNameCanContainStrategyWord(t *testing.T) {
	active := ActiveSkillSession{
		SessionID:       "as_test",
		UserID:          1,
		SkillName:       "strategy_management",
		ActionName:      "create",
		Goal:            "创建一个趋势策略，交易BTC和ETH，15m，杠杆 5 倍",
		CollectedFields: map[string]any{},
		LocalHistory: []chatMessage{
			{Role: "user", Content: "创建一个趋势策略，交易BTC和ETH，15m，杠杆 5 倍"},
			{Role: "assistant", Content: "现在只差一个名称。"},
			{Role: "user", Content: "趋势策略A"},
		},
	}

	session := activeToLegacySkillSession(active)
	if got := fieldValue(session, "name"); got != "趋势策略A" {
		t.Fatalf("expected standalone strategy name to be preserved, got %q", got)
	}
}

func TestStrategyCreateProposesGridDefaultsBeforeCreate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-grid-create-draft.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name":          "我的网格策略",
			"strategy_type": "grid_trading",
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "grid_trading", session)
	if !strings.Contains(reply, "还缺") || !strings.Contains(reply, "交易对") || !strings.Contains(reply, "网格数量") {
		t.Fatalf("expected grid template missing-fields prompt, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	for _, strategy := range strategies {
		if strategy.Name == "我的网格策略" {
			t.Fatalf("strategy should not be created before grid config is ready")
		}
	}
}

func TestStrategyCreateSwitchingTypeDropsPreviousTemplateFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-switch-type.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	aiPatch := map[string]any{
		"strategy_type": "ai_trading",
		"ai_config": map[string]any{
			"coin_source": map[string]any{"source_type": "ai500"},
			"risk_control": map[string]any{
				"min_confidence":        80,
				"min_risk_reward_ratio": 3,
			},
		},
	}
	rawPatch, _ := json.Marshal(aiPatch)
	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Phase:  "collecting",
		Fields: map[string]string{
			"name":                         "我的网格大大",
			"strategy_type":                "ai_trading",
			strategyCreateConfigPatchField: string(rawPatch),
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "算了选网格策略吧", session)
	if !strings.Contains(reply, "还缺") || !strings.Contains(reply, "交易对") {
		t.Fatalf("expected grid missing fields after type switch, got: %s", reply)
	}
	if strings.Contains(reply, "AI500") || strings.Contains(reply, "置信度") {
		t.Fatalf("type switch should not reuse AI fields or default BTC summary, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	for _, strategy := range strategies {
		if strategy.Name == "我的网格大大" {
			t.Fatalf("strategy should not be created after switching type with missing grid fields")
		}
	}
}

func TestActiveStrategyCreateFilterIsolatesTemplateOnTypeSwitch(t *testing.T) {
	session := ActiveSkillSession{
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":          "我的网格大大",
			"strategy_type": "ai_trading",
			strategyCreateConfigPatchField: map[string]any{
				"strategy_type": "ai_trading",
				"ai_config": map[string]any{
					"coin_source": map[string]any{"source_type": "ai500"},
				},
			},
		},
	}
	filtered := filterExtractedDataForActiveSession(session, map[string]any{
		"strategy_type": "grid_trading",
		strategyCreateConfigPatchField: map[string]any{
			"strategy_type": "grid_trading",
			"grid_config":   map[string]any{"symbol": "ETHUSDT"},
			"ai_config":     map[string]any{"coin_source": map[string]any{"source_type": "ai500"}},
		},
	}, "zh")
	mergeExtractedData(&session, filtered)
	if got := session.CollectedFields["strategy_type"]; got != "grid_trading" {
		t.Fatalf("expected switched strategy type, got %+v", session.CollectedFields)
	}
	if _, ok := session.CollectedFields["source_type"]; ok {
		t.Fatalf("expected AI-only flat fields to be dropped, got %+v", session.CollectedFields)
	}
	patch := session.CollectedFields[strategyCreateConfigPatchField].(map[string]any)
	if _, ok := patch["ai_config"]; ok {
		t.Fatalf("expected ai_config to be removed from grid patch, got %+v", patch)
	}
	if _, ok := patch["grid_config"]; !ok {
		t.Fatalf("expected grid_config to remain, got %+v", patch)
	}
}

func TestStrategyCreateConfirmationFillsMissingGridDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-grid-create-confirm-defaults.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name":                        "餐巾纸",
			"strategy_type":               "grid_trading",
			"symbol":                      "BTCUSDT",
			"awaiting_final_confirmation": "true",
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "好的，就这样", session)
	if !strings.Contains(reply, "还缺") || strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected missing grid fields instead of default create, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	for _, strategy := range strategies {
		if strategy.Name == "餐巾纸" {
			t.Fatalf("strategy should not be created before grid template is complete")
		}
	}
}

func TestStrategyCreateGridDraftSummaryDoesNotMentionAIFields(t *testing.T) {
	reply := formatStrategyCreateDraftSummary("zh", "我的网格策略", "grid_trading", nil, nil)
	for _, unexpected := range []string{"选币来源", "最大持仓", "置信度", "盈亏比", "多周期"} {
		if strings.Contains(reply, unexpected) {
			t.Fatalf("grid draft summary should not mention AI-only field %q: %s", unexpected, reply)
		}
	}
	for _, expected := range []string{"网格策略", "交易对", "网格数量", "总投入", "杠杆", "价格区间"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("grid draft summary should mention %q, got: %s", expected, reply)
		}
	}
}

func TestAllowedStrategyCreateFieldsUseConfigPatchOnly(t *testing.T) {
	gridSession := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"strategy_type": "grid_trading",
		},
	}
	gridSpecs := allowedFieldSpecsForSkillSession(gridSession, "zh")
	gridKeys := make(map[string]bool, len(gridSpecs))
	for _, spec := range gridSpecs {
		gridKeys[spec.Key] = true
	}
	for _, expected := range []string{"strategy_type", "name", strategyCreateConfigPatchField, "awaiting_final_confirmation"} {
		if !gridKeys[expected] {
			t.Fatalf("expected grid field %q in specs", expected)
		}
	}
	for _, unexpected := range []string{"symbol", "grid_count", "total_investment", "source_type", "selected_timeframes", "min_confidence", "min_risk_reward_ratio"} {
		if gridKeys[unexpected] {
			t.Fatalf("strategy create specs should not expose template field %q outside config_patch", unexpected)
		}
	}
}

func TestStrategyCreateReadyConfigRequiresFinalConfirmation(t *testing.T) {
	patch := map[string]any{
		"strategy_type": "grid_trading",
		"grid_config": map[string]any{
			"symbol":                  "BTCUSDT",
			"grid_count":              20,
			"total_investment":        200,
			"leverage":                2,
			"use_atr_bounds":          true,
			"atr_multiplier":          2,
			"distribution":            "uniform",
			"max_drawdown_pct":        15,
			"stop_loss_pct":           8,
			"daily_loss_limit_pct":    6,
			"use_maker_only":          true,
			"enable_direction_adjust": false,
		},
	}
	rawPatch, _ := json.Marshal(patch)
	session := ActiveSkillSession{
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":                         "小白策略",
			"strategy_type":                "grid_trading",
			strategyCreateConfigPatchField: string(rawPatch),
		},
	}

	reply, blocked := guardStrategyCreateBeforeFinalConfirmation("zh", session)
	if !blocked {
		t.Fatalf("expected ready strategy create config to require final confirmation")
	}
	if !strings.Contains(reply, "确认后我再创建") || !strings.Contains(reply, "BTCUSDT") || !strings.Contains(reply, "20") {
		t.Fatalf("expected final confirmation summary, got: %s", reply)
	}

	session.CollectedFields["awaiting_final_confirmation"] = true
	if _, blocked := guardStrategyCreateBeforeFinalConfirmation("zh", session); !blocked {
		t.Fatalf("same-turn awaiting flag without prior assistant confirmation should still be blocked")
	}
	session.LocalHistory = append(session.LocalHistory, chatMessage{Role: "assistant", Content: reply})
	if _, blocked := guardStrategyCreateBeforeFinalConfirmation("zh", session); blocked {
		t.Fatalf("already-confirmable session should not be blocked")
	}
}

func TestStrategyCreateConfirmationForcesSynchronousExecutionRoute(t *testing.T) {
	patch := map[string]any{
		"strategy_type": "ai_trading",
		"ai_config": map[string]any{
			"coin_source": map[string]any{
				"source_type": "ai500",
				"use_ai500":   true,
				"ai500_limit": 5,
			},
			"indicators": map[string]any{
				"klines": map[string]any{
					"primary_timeframe":   "1m",
					"selected_timeframes": []any{"1m", "5m"},
				},
			},
			"risk_control": map[string]any{
				"btc_eth_max_leverage":  3,
				"altcoin_max_leverage":  2,
				"min_confidence":        70,
				"min_risk_reward_ratio": 1.5,
			},
			"prompt_sections": map[string]any{
				"trading_frequency": "高频交易但避免过度交易。",
				"entry_standards":   "只在短周期趋势明确且风险收益合理时开仓。",
			},
		},
	}
	rawPatch, _ := json.Marshal(patch)
	session := ActiveSkillSession{
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":                         "AI500高频交易",
			"strategy_type":                "ai_trading",
			strategyCreateConfigPatchField: string(rawPatch),
		},
		LocalHistory: []chatMessage{
			{Role: "assistant", Content: "请确认是否按以上设置创建？如果没问题，我就执行创建。"},
		},
	}
	for _, confirmation := range []string{"确认创建", "可以", "好的", "没问题", "ok"} {
		t.Run(confirmation, func(t *testing.T) {
			sessionCopy := session
			sessionCopy.CollectedFields = map[string]any{
				"name":                         "AI500高频交易",
				"strategy_type":                "ai_trading",
				strategyCreateConfigPatchField: string(rawPatch),
			}
			decision := activeSessionStepDecision{
				Route: "ask_user",
				Reply: "好的，正在为你创建“AI500高频交易”策略……",
			}

			if !maybeForceStrategyCreateExecutionOnConfirmation("zh", confirmation, &sessionCopy, &decision) {
				t.Fatalf("expected confirmation %q to force execute route", confirmation)
			}
			if decision.Route != "execute_skill" || decision.Reply != "" {
				t.Fatalf("expected synchronous execute route with empty reply, got %+v", decision)
			}
			if !activeFieldBool(sessionCopy.CollectedFields["awaiting_final_confirmation"]) {
				t.Fatalf("expected awaiting_final_confirmation to be set before execution")
			}
		})
	}
}

func TestStrategyCreateConfirmationForcesExecutionWithoutPriorPromptPhrase(t *testing.T) {
	patch := map[string]any{
		"strategy_type": "ai_trading",
		"ai_config": map[string]any{
			"coin_source": map[string]any{
				"source_type": "ai500",
				"use_ai500":   true,
				"ai500_limit": 5,
			},
			"indicators": map[string]any{
				"klines": map[string]any{
					"primary_timeframe":   "3m",
					"selected_timeframes": []any{"3m", "5m", "15m"},
				},
			},
			"risk_control": map[string]any{
				"btc_eth_max_leverage":  3,
				"altcoin_max_leverage":  2,
				"min_confidence":        75,
				"min_risk_reward_ratio": 1.5,
			},
			"prompt_sections": map[string]any{
				"trading_frequency": "高频但避免过度交易。",
				"entry_standards":   "趋势明确、成交量配合、风险收益合理才开仓。",
			},
		},
	}
	rawPatch, _ := json.Marshal(patch)
	session := ActiveSkillSession{
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":                         "高频稳健AI500",
			"strategy_type":                "ai_trading",
			strategyCreateConfigPatchField: string(rawPatch),
		},
		LocalHistory: []chatMessage{
			{Role: "assistant", Content: "这是我建议的一版配置。"},
		},
	}
	decision := activeSessionStepDecision{
		Route: "ask_user",
		Reply: "好的，马上为你创建“高频稳健AI500”策略。",
	}
	if !maybeForceStrategyCreateExecutionOnConfirmation("zh", "确认创建", &session, &decision) {
		t.Fatalf("expected ready strategy confirmation to force execute even without prior prompt phrase")
	}
	if decision.Route != "execute_skill" || decision.Reply != "" {
		t.Fatalf("expected execute route, got %+v", decision)
	}
}

func TestUnifiedPlannedAgentCannotStealActiveStrategyCreateConfirmation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-planner-steal.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(&staticAIClient{response: `{"route":"ask_user","reply":"好的，马上为你创建策略。","extracted_data":{}}`})

	patch := map[string]any{
		"strategy_type": "ai_trading",
		"ai_config": map[string]any{
			"coin_source": map[string]any{
				"source_type": "ai500",
				"use_ai500":   true,
				"ai500_limit": 5,
			},
			"indicators": map[string]any{
				"klines": map[string]any{
					"primary_timeframe":   "5m",
					"selected_timeframes": []any{"1m", "5m", "15m"},
				},
			},
			"risk_control": map[string]any{
				"btc_eth_max_leverage":  3,
				"altcoin_max_leverage":  2,
				"min_confidence":        80,
				"min_risk_reward_ratio": 1.5,
			},
			"prompt_sections": map[string]any{
				"trading_frequency": "每天最多 5-8 笔，避免连续亏损后追单。",
				"entry_standards":   "趋势确认、成交量放大、资金费率正常才开仓。",
			},
		},
	}
	rawPatch, _ := json.Marshal(patch)
	userID := int64(42)
	session := newActiveSkillSession(userID, "strategy_management", "create")
	session.CollectedFields = map[string]any{
		"name":                         "AI500高频",
		"strategy_type":                "ai_trading",
		"awaiting_final_confirmation":  true,
		strategyCreateConfigPatchField: string(rawPatch),
	}
	a.saveActiveSkillSession(session)

	decision := unifiedTurnDecision{
		TopicIntent:    "continue_active",
		BusinessAction: "planned_agent",
	}
	reply, handled, err := a.executeUnifiedTurnDecision(context.Background(), "default", userID, "zh", "确认", decision, nil)
	if err != nil {
		t.Fatalf("execute unified turn: %v", err)
	}
	if !handled {
		t.Fatalf("expected turn to be handled")
	}
	if strings.Contains(reply, "马上") || strings.Contains(reply, "稍后") || strings.Contains(reply, "正在") {
		t.Fatalf("expected planner promise to be bypassed, got: %s", reply)
	}
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected real strategy creation result, got: %s", reply)
	}
}

func TestStrategyCreateRepairPromiseIsNotReturnedOnConfirmation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-repair-promise.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(&staticAIClient{response: `{"route":"ask_user","reply":"好的，马上为你创建AI500高频稳健策略。","extracted_data":{}}`})

	userID := int64(42)
	session := newActiveSkillSession(userID, "strategy_management", "create")
	session.CollectedFields = map[string]any{
		"name":          "AI500高频",
		"strategy_type": "ai_trading",
	}
	session.LocalHistory = []chatMessage{
		{Role: "assistant", Content: "如果你确认没问题，告诉我“确认创建”，我就帮你直接创建。"},
	}
	a.saveActiveSkillSession(session)

	reply, handled, err := a.driveActiveSession(context.Background(), "default", userID, "zh", "确认创建", session, nil)
	if err != nil {
		t.Fatalf("drive active session: %v", err)
	}
	if !handled {
		t.Fatalf("expected confirmation turn to be handled")
	}
	if strings.Contains(reply, "马上") || strings.Contains(reply, "正在") || strings.Contains(reply, "稍后") {
		t.Fatalf("repair promise should not be returned on confirmation, got: %s", reply)
	}
}

func TestModelCreateSessionRedirectsStrategyTypeChoiceToStrategyCreate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-type-choice-not-model-provider.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	const userID int64 = 42
	a.saveSkillSession(userID, skillSession{
		Name:   "model_management",
		Action: "create",
		Phase:  "collecting",
		Fields: map[string]string{},
	})

	reply, ok := a.redirectModelCreateSessionToStrategyCreateIfNeeded("default", userID, "zh", "1.AI交易策略", a.getSkillSession(userID))
	if !ok {
		t.Fatalf("expected strategy type choice to redirect away from model create")
	}
	if strings.Contains(reply, "模型提供商") || strings.Contains(reply, "provider") {
		t.Fatalf("strategy type choice must not ask for model provider, got: %s", reply)
	}
	session := a.getSkillSession(userID)
	if session.Name != "strategy_management" || session.Action != "create" {
		t.Fatalf("expected active session to be strategy create, got %+v", session)
	}
	if got := fieldValue(session, "strategy_type"); got != "ai_trading" {
		t.Fatalf("expected ai strategy type to be captured, got %q in %+v", got, session)
	}
}

func TestStrategyCreateAskUserReplyIsNotOverriddenByTemplateMissingFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-llm-ask-reply.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(&staticAIClient{response: `{"route":"ask_user","reply":"我会按 AI 策略模板继续填。你如果想稳健，我建议先用 OI Low、15m 主周期、最低置信度 70。确认这个方向吗？"}`})

	session := ActiveSkillSession{
		UserID:     42,
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":          "AI高频",
			"strategy_type": "ai_trading",
		},
	}
	reply, handled, err := a.driveActiveSession(context.Background(), "default", 42, "zh", "1h", session, nil)
	if err != nil {
		t.Fatalf("drive active session: %v", err)
	}
	if !handled {
		t.Fatalf("expected active session to be handled")
	}
	if strings.Contains(reply, "这份策略模板还没填完整") || strings.Contains(reply, "还缺这些字段") {
		t.Fatalf("LLM ask_user reply should not be overridden by hard template missing list, got: %s", reply)
	}
	if !strings.Contains(reply, "OI Low") || !strings.Contains(reply, "70") {
		t.Fatalf("expected LLM reply to pass through, got: %s", reply)
	}
}

func TestStrategyCreateAIReplyRejectsNonTemplateInvestmentQuestion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-ai-non-template-question.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(&staticAIClient{response: `{"route":"ask_user","reply":"你打算投入多少资金来运行这个策略？比如100U、500U？这样我可以帮你设置止损和仓位。"}`})

	session := ActiveSkillSession{
		UserID:     42,
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":          "AI500稳健",
			"strategy_type": "ai_trading",
		},
	}
	reply, handled, err := a.driveActiveSession(context.Background(), "default", 42, "zh", "全部你定吧，稳健就行", session, nil)
	if err != nil {
		t.Fatalf("drive active session: %v", err)
	}
	if !handled {
		t.Fatalf("expected active session to be handled")
	}
	for _, blocked := range []string{"投入多少", "100U", "500U", "止损", "仓位"} {
		if strings.Contains(reply, blocked) {
			t.Fatalf("AI strategy reply should not ask non-template field %q, got: %s", blocked, reply)
		}
	}
}

func TestStrategyCreateOptionsQuestionExplainsCurrentMissingField(t *testing.T) {
	session := ActiveSkillSession{
		UserID:     42,
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":          "AI500高频交易",
			"strategy_type": "ai_trading",
		},
	}
	reply, blocked := strategyCreateTemplateMissingReply("zh", "有哪些选择吗", session)
	if !blocked {
		t.Fatalf("expected options question to be handled")
	}
	for _, want := range []string{"AI500", "OI Top", "OI Low", "静态币种"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected source options to include %q, got: %s", want, reply)
		}
	}
	if strings.Contains(reply, "还缺") || strings.Contains(reply, "BTC/ETH 最大杠杆") {
		t.Fatalf("options question should not repeat the full missing-field list, got: %s", reply)
	}
}

func TestStrategyCreateMissingFieldsIncludeInlineOptions(t *testing.T) {
	reply := formatStrategyCreateConfigNeeded("zh", "source_type,primary_timeframe,btceth_max_leverage,min_confidence,trading_frequency")
	for _, want := range []string{"AI500", "OI Top", "OI Low", "静态币种", "1m", "1h", "1～20", "50～100", "每天最多"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected missing-field prompt to include option/range %q, got: %s", want, reply)
		}
	}
	if !strings.Contains(reply, "你帮我按稳健/高频/激进来推荐") {
		t.Fatalf("expected prompt to offer recommendation shortcut, got: %s", reply)
	}
}

func TestStrategyCreateConfigPatchReplyUsesStructuredMissingFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-recommendation-structured-missing.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(&staticAIClient{response: `{"route":"ask_user","reply":"我建议按高频但稳健来填：主周期 3m，多周期 3m/5m/15m，BTC/ETH 3倍，山寨币 2倍。确认的话我会继续整理完整模板。","extracted_data":{"config_patch":{"strategy_type":"ai_trading","ai_config":{"indicators":{"klines":{"primary_timeframe":"3m","selected_timeframes":["3m","5m","15m"]}}}}}}`})

	session := ActiveSkillSession{
		UserID:     42,
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":          "AI500高频交易",
			"strategy_type": "ai_trading",
			strategyCreateConfigPatchField: map[string]any{
				"strategy_type": "ai_trading",
				"ai_config": map[string]any{
					"coin_source": map[string]any{"source_type": "ai500", "use_ai500": true, "ai500_limit": 5},
					"risk_control": map[string]any{
						"min_confidence": 70,
					},
				},
			},
		},
	}
	reply, handled, err := a.driveActiveSession(context.Background(), "default", 42, "zh", "继续", session, nil)
	if err != nil {
		t.Fatalf("drive active session: %v", err)
	}
	if !handled {
		t.Fatalf("expected recommendation request to be handled")
	}
	if !strings.Contains(reply, "这份策略模板还没填完整") {
		t.Fatalf("expected structured missing-field prompt after partial config_patch, got: %s", reply)
	}
	if strings.Contains(reply, "我建议按高频但稳健来填") {
		t.Fatalf("LLM free-form recommendation should not be used as the current plan, got: %s", reply)
	}
	if !strings.Contains(reply, "BTC/ETH 最大杠杆") || !strings.Contains(reply, "开仓标准") {
		t.Fatalf("expected deterministic missing template fields, got: %s", reply)
	}
}

func TestStrategyCreateFirstStageConfigProgressUsesStructuredMissingFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-first-stage-structured-missing.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(&staticAIClient{response: `{"route":"ask_user","reply":"收到，AI500 和最低置信度 80 是你指定的；其他我建议按高频稳健来定：主周期 3m，多周期 3m/5m/15m，BTC/ETH 3倍，山寨币 2倍，最小盈亏比 2。","extracted_data":{}}`})

	session := ActiveSkillSession{
		UserID:     42,
		SkillName:  "strategy_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"name":          "高频稳健AI500",
			"strategy_type": "ai_trading",
			strategyCreateConfigProgressThisTurnField: true,
			strategyCreateConfigPatchField: map[string]any{
				"strategy_type": "ai_trading",
				"ai_config": map[string]any{
					"coin_source": map[string]any{"source_type": "ai500", "use_ai500": true},
					"risk_control": map[string]any{
						"min_confidence": 80,
					},
				},
			},
		},
	}
	reply, handled, err := a.driveActiveSession(context.Background(), "default", 42, "zh", "选币选AI500，最新置信度80，其他你定，能高频交易稳定就行", session, nil)
	if err != nil {
		t.Fatalf("drive active session: %v", err)
	}
	if !handled {
		t.Fatalf("expected active session to be handled")
	}
	if !strings.Contains(reply, "这份策略模板还没填完整") {
		t.Fatalf("expected structured missing-field prompt after first-stage config progress, got: %s", reply)
	}
	if strings.Contains(reply, "其他我建议按高频稳健来定") {
		t.Fatalf("LLM free-form recommendation should not be used as the current plan, got: %s", reply)
	}
	if !strings.Contains(reply, "主周期") || !strings.Contains(reply, "BTC/ETH 最大杠杆") {
		t.Fatalf("expected deterministic missing template fields, got: %s", reply)
	}
}

func TestStrategyCreateConfirmationUsesModelRepairForPriorStyleProposal(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-create-style-repair.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())
	a.SetAIClient(&staticAIClient{response: `{"route":"execute_skill","extracted_data":{"config_patch":{"strategy_type":"ai_trading","ai_config":{"coin_source":{"source_type":"ai500","use_ai500":true,"ai500_limit":3},"indicators":{"klines":{"primary_timeframe":"1m","primary_count":20,"selected_timeframes":["1m","5m","15m"],"enable_multi_timeframe":true,"enable_raw_klines":true},"enable_volume":true,"enable_oi":true,"enable_funding_rate":true,"enable_quant_data":true},"risk_control":{"btc_eth_max_leverage":5,"altcoin_max_leverage":5,"min_confidence":75,"min_risk_reward_ratio":3},"prompt_sections":{"trading_frequency":"高频但不过度交易：目标每小时 1-3 笔；单笔持仓通常 10-30 分钟。","entry_standards":"只在短周期趋势、成交量/OI、资金费率或排行信号形成共振时入场。"}}}}}`})

	userID := int64(42)
	session := newActiveSkillSession(userID, "strategy_management", "create")
	session.CollectedFields = map[string]any{
		"name":          "AI500极致稳定高频",
		"strategy_type": "ai_trading",
	}
	session.LocalHistory = []chatMessage{
		{Role: "assistant", Content: "我建议主周期改成1分钟，多周期改成1分钟、5分钟、15分钟，交易频率按高频但稳定来写。"},
	}

	reply, handled, err := a.driveActiveSession(context.Background(), "default", userID, "zh", "好的可以，确认创建", session, nil)
	if err != nil {
		t.Fatalf("drive active session: %v", err)
	}
	if !handled {
		t.Fatalf("expected confirmation to be handled")
	}
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected real strategy creation after model repair, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	if len(strategies) != 1 {
		t.Fatalf("expected one created strategy, got %d", len(strategies))
	}
	var cfg store.StrategyConfig
	if err := json.Unmarshal([]byte(strategies[0].Config), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.CoinSource.SourceType != "ai500" || cfg.Indicators.Klines.PrimaryTimeframe != "1m" {
		t.Fatalf("expected model-repaired AI500 1m strategy, got %+v", cfg)
	}
}

func TestStrategyCreateCreatesGridAfterConfigPatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-grid-create-ready.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	patch := map[string]any{
		"strategy_type": "grid_trading",
		"grid_config": map[string]any{
			"symbol":               "ETHUSDT",
			"grid_count":           12,
			"total_investment":     1000,
			"leverage":             3,
			"use_atr_bounds":       true,
			"atr_multiplier":       2,
			"distribution":         "gaussian",
			"max_drawdown_pct":     15,
			"stop_loss_pct":        5,
			"daily_loss_limit_pct": 10,
			"use_maker_only":       true,
		},
	}
	rawPatch, _ := json.Marshal(patch)
	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name":                         "我的网格策略",
			strategyCreateConfigPatchField: string(rawPatch),
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "确认创建", session)
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected create reply, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	var created *store.Strategy
	for _, strategy := range strategies {
		if strategy.Name == "我的网格策略" {
			created = strategy
			break
		}
	}
	if created == nil {
		t.Fatalf("expected grid strategy to be created")
	}
	var cfg store.StrategyConfig
	if err := json.Unmarshal([]byte(created.Config), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.StrategyType != "grid_trading" || cfg.GridConfig == nil || cfg.GridConfig.Symbol != "ETHUSDT" {
		t.Fatalf("expected grid config to persist, got %+v", cfg)
	}
}

func TestManageStrategyToolCreateRequiresConfirmation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-tool-create-confirmation.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	resp := a.toolManageStrategy("default", `{"action":"create","name":"未确认网格","lang":"zh","config":{"strategy_type":"grid_trading","grid_config":{"symbol":"BTCUSDT","total_investment":200,"use_atr_bounds":true}}}`)
	if !strings.Contains(resp, "requires_confirmation") {
		t.Fatalf("expected tool create to require confirmation, got: %s", resp)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	for _, strategy := range strategies {
		if strategy.Name == "未确认网格" {
			t.Fatalf("unconfirmed tool call should not create strategy")
		}
	}

	resp = a.toolManageStrategy("default", `{"action":"create","name":"已确认网格","lang":"zh","confirmed":true,"allow_clamped_update":true,"config":{"strategy_type":"grid_trading","grid_config":{"symbol":"BTCUSDT","total_investment":200,"use_atr_bounds":true}}}`)
	if strings.Contains(resp, `"error"`) {
		t.Fatalf("expected confirmed create to succeed, got: %s", resp)
	}
}

func TestStrategyCreateGridPatchInfersStrategyType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-grid-create-infers-type.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	patch := map[string]any{
		"grid_config": map[string]any{
			"symbol":               "BTCUSDT",
			"grid_count":           20,
			"total_investment":     200,
			"leverage":             2,
			"use_atr_bounds":       true,
			"atr_multiplier":       2,
			"distribution":         "uniform",
			"max_drawdown_pct":     15,
			"stop_loss_pct":        5,
			"daily_loss_limit_pct": 10,
			"use_maker_only":       true,
		},
	}
	rawPatch, _ := json.Marshal(patch)
	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name":                         "小白网格",
			strategyCreateConfigPatchField: string(rawPatch),
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "确认创建", session)
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected create reply, got: %s", reply)
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	var cfg store.StrategyConfig
	for _, strategy := range strategies {
		if strategy.Name == "小白网格" {
			if err := json.Unmarshal([]byte(strategy.Config), &cfg); err != nil {
				t.Fatalf("unmarshal config: %v", err)
			}
			break
		}
	}
	if cfg.StrategyType != "grid_trading" || cfg.GridConfig == nil || cfg.GridConfig.Symbol != "BTCUSDT" {
		t.Fatalf("expected grid patch to infer grid_trading, got %+v", cfg)
	}
}

func TestStrategyCreateGridPatchKeepsBackendGridDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "strategy-grid-create-defaults.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	a := New(nil, st, DefaultConfig(), slog.Default())

	patch := map[string]any{
		"strategy_type": "grid_trading",
		"grid_config": map[string]any{
			"symbol":           "ETHUSDT",
			"grid_count":       20,
			"total_investment": 500,
			"leverage":         3,
		},
	}
	rawPatch, _ := json.Marshal(patch)
	session := skillSession{
		Name:   "strategy_management",
		Action: "create",
		Fields: map[string]string{
			"name":                         "餐巾纸",
			strategyCreateConfigPatchField: string(rawPatch),
		},
	}

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "确认创建", session)
	if !strings.Contains(reply, "还缺") || strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected incomplete grid patch to ask for missing fields, got: %s", reply)
	}
}

func TestLLMFlowExtractionFiltersFieldsToAllowedSchema(t *testing.T) {
	result := llmFlowExtractionResult{
		Intent: "continue",
		Tasks: []llmFlowExtractionTask{{
			Skill:  "exchange_management",
			Action: "create",
			Fields: map[string]string{
				"secret":     "wrong-key",
				"secret_key": "canonical-secret",
				"api_key":    "api",
			},
		}},
	}
	filtered := filterLLMFlowExtractionFields(result, []llmFlowFieldSpec{
		{Key: "secret_key"},
		{Key: "api_key"},
	})
	fields := filtered.Tasks[0].Fields
	if _, ok := fields["secret"]; ok {
		t.Fatalf("expected invented field key to be filtered, got: %+v", fields)
	}
	if fields["secret_key"] != "canonical-secret" || fields["api_key"] != "api" {
		t.Fatalf("expected canonical fields to remain, got: %+v", fields)
	}
}

func TestExchangeCreateAllowedFieldSpecsUseCanonicalSecretKey(t *testing.T) {
	specs := allowedFieldSpecsForSkillSession(skillSession{Name: "exchange_management", Action: "create"}, "zh")
	foundSecretKey := false
	for _, spec := range specs {
		if spec.Key == "secret" {
			t.Fatal("exchange create schema should not expose non-canonical secret key")
		}
		if spec.Key == "secret_key" {
			foundSecretKey = true
		}
	}
	if !foundSecretKey {
		t.Fatal("expected exchange create schema to include canonical secret_key")
	}
}

func TestActiveSessionExtractedDataFiltersToAllowedSchema(t *testing.T) {
	session := ActiveSkillSession{
		SkillName:  "exchange_management",
		ActionName: "create",
		CollectedFields: map[string]any{
			"exchange_type": "okx",
		},
	}
	filtered := filterExtractedDataForActiveSession(session, map[string]any{
		"account_name": "呢呢",
		"api_key":      "api",
		"secret":       "wrong-key",
		"secret_key":   "canonical-secret",
		"passphrase":   "pass",
	}, "zh")
	if _, ok := filtered["secret"]; ok {
		t.Fatalf("expected central brain alias key to be filtered, got: %+v", filtered)
	}
	for _, key := range []string{"account_name", "api_key", "secret_key", "passphrase"} {
		if _, ok := filtered[key]; !ok {
			t.Fatalf("expected canonical key %q to remain, got: %+v", key, filtered)
		}
	}
}

func TestBrainUserPromptIncludesActiveAllowedFieldSchema(t *testing.T) {
	prompt := buildBrainUserPrompt(
		"zh",
		"密钥是abc123456",
		"要创建交易所配置，还缺这些字段：Secret。",
		"",
		"",
		ActiveSkillSession{SkillName: "exchange_management", ActionName: "create"},
		true,
	)
	if !strings.Contains(prompt, "allowed_field_spec_json") || !strings.Contains(prompt, `"secret_key"`) {
		t.Fatalf("expected brain prompt to expose canonical field schema, got:\n%s", prompt)
	}
}
