package agent

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
)

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

	exchangeID, err := st.Exchange().Create("default", "okx", "小偶", true, "api-test", "secret-test", "pass", false, "", false, "", "", "", "", "", "", 0)
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
	exchangeID, err := st.Exchange().Create("default", "okx", "OKX 主账户", true, "api-test", "secret-test", "pass", false, "", false, "", "", "", "", "", "", 0)
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
			"max_positions":  1,
			"min_confidence": 80,
		},
		"prompt_sections": map[string]any{
			"entry_standards": "只在 BTC 下跌趋势确认时考虑做空，禁止把做多作为主方向。",
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

	reply := a.handleStrategyCreateSkill("default", 1, "zh", "BTC趋势做空", session)
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
	if !cfg.CoinSource.UseOILow {
		t.Fatalf("expected OI low enabled for short-biased strategy")
	}
	if cfg.RiskControl.MaxPositions != 1 || cfg.RiskControl.MinConfidence != 80 {
		t.Fatalf("expected risk patch to apply, got %+v", cfg.RiskControl)
	}
	if !strings.Contains(cfg.CustomPrompt, "BTC 趋势做空") || !strings.Contains(cfg.PromptSections.EntryStandards, "做空") {
		t.Fatalf("expected prompt patch to apply, got custom=%q entry=%q", cfg.CustomPrompt, cfg.PromptSections.EntryStandards)
	}
}

func TestStrategyCreateDraftPreservesTwoTurnNaturalLanguageRequirements(t *testing.T) {
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
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected created reply, got: %s", reply)
	}

	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	if len(strategies) != 1 {
		t.Fatalf("expected one strategy, got %d", len(strategies))
	}
	var cfg store.StrategyConfig
	if err := json.Unmarshal([]byte(strategies[0].Config), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.CoinSource.SourceType != "static" || cfg.CoinSource.UseAI500 || strings.Join(cfg.CoinSource.StaticCoins, ",") != "BTCUSDT,ETHUSDT" {
		t.Fatalf("expected static BTC/ETH and AI500 disabled, got %+v", cfg.CoinSource)
	}
	if cfg.Indicators.Klines.PrimaryTimeframe != "15m" {
		t.Fatalf("expected primary timeframe 15m, got %s", cfg.Indicators.Klines.PrimaryTimeframe)
	}
	if cfg.RiskControl.BTCETHMaxLeverage != 5 || cfg.RiskControl.AltcoinMaxLeverage != 5 {
		t.Fatalf("expected leverage 5x, got btceth=%d alt=%d", cfg.RiskControl.BTCETHMaxLeverage, cfg.RiskControl.AltcoinMaxLeverage)
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
	if !strings.Contains(reply, "配置整理好了") || !strings.Contains(reply, "BTCUSDT") || !strings.Contains(reply, "ATR 动态范围") {
		t.Fatalf("expected grid default confirmation draft, got: %s", reply)
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
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected strategy to be created after confirmation with defaults, got: %s", reply)
	}
	for _, want := range []string{"总投入：1000.00 USDT", "网格数量：10", "杠杆：5倍", "价格范围：ATR 自动计算"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected create reply to report actual defaulted value %q, got: %s", want, reply)
		}
	}
	for _, unexpected := range []string{"系统已自动收敛", "最大持仓数", "最低置信度"} {
		if strings.Contains(reply, unexpected) {
			t.Fatalf("create reply should not expose internal clamp detail %q, got: %s", unexpected, reply)
		}
	}
	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	var cfg store.StrategyConfig
	for _, strategy := range strategies {
		if strategy.Name == "餐巾纸" {
			if err := json.Unmarshal([]byte(strategy.Config), &cfg); err != nil {
				t.Fatalf("unmarshal config: %v", err)
			}
			break
		}
	}
	if cfg.GridConfig == nil {
		t.Fatalf("expected grid config")
	}
	if cfg.GridConfig.Symbol != "BTCUSDT" || cfg.GridConfig.GridCount <= 0 || cfg.GridConfig.TotalInvestment <= 0 || cfg.GridConfig.Leverage <= 0 {
		t.Fatalf("expected explicit symbol and defaulted grid fields, got %+v", cfg.GridConfig)
	}
	if !cfg.GridConfig.UseATRBounds || cfg.GridConfig.Distribution == "" || cfg.GridConfig.MaxDrawdownPct <= 0 {
		t.Fatalf("expected defaulted optional grid fields, got %+v", cfg.GridConfig)
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

func TestAllowedStrategyCreateFieldsFollowSelectedStrategyType(t *testing.T) {
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
	for _, expected := range []string{"symbol", "grid_count", "total_investment", "leverage", "max_drawdown_pct"} {
		if !gridKeys[expected] {
			t.Fatalf("expected grid field %q in specs", expected)
		}
	}
	for _, unexpected := range []string{"source_type", "selected_timeframes", "min_confidence", "min_risk_reward_ratio"} {
		if gridKeys[unexpected] {
			t.Fatalf("grid specs should not expose AI-only field %q", unexpected)
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
			"symbol":           "ETHUSDT",
			"grid_count":       12,
			"total_investment": 1000,
			"leverage":         3,
			"use_atr_bounds":   true,
			"atr_multiplier":   2,
			"distribution":     "gaussian",
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
			"symbol":           "BTCUSDT",
			"grid_count":       20,
			"total_investment": 200,
			"leverage":         2,
			"use_atr_bounds":   true,
			"atr_multiplier":   2,
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
	if !strings.Contains(reply, "已创建策略") {
		t.Fatalf("expected create reply, got: %s", reply)
	}

	strategies, err := st.Strategy().List("default")
	if err != nil {
		t.Fatalf("list strategies: %v", err)
	}
	var cfg store.StrategyConfig
	for _, strategy := range strategies {
		if strategy.Name == "餐巾纸" {
			if err := json.Unmarshal([]byte(strategy.Config), &cfg); err != nil {
				t.Fatalf("unmarshal config: %v", err)
			}
			break
		}
	}
	if cfg.GridConfig == nil {
		t.Fatalf("expected grid config")
	}
	if !cfg.GridConfig.UseATRBounds || cfg.GridConfig.ATRMultiplier != 2 || cfg.GridConfig.Distribution != "gaussian" {
		t.Fatalf("expected grid defaults for bounds/distribution, got %+v", cfg.GridConfig)
	}
	if cfg.GridConfig.MaxDrawdownPct != 15 || cfg.GridConfig.StopLossPct != 5 || cfg.GridConfig.DailyLossLimitPct != 10 || !cfg.GridConfig.UseMakerOnly {
		t.Fatalf("expected grid risk defaults, got %+v", cfg.GridConfig)
	}
	if cfg.GridConfig.DirectionBiasRatio != 0.7 {
		t.Fatalf("expected direction bias default, got %+v", cfg.GridConfig)
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
