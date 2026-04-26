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
