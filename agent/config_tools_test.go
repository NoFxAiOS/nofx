package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
)

func newTestAgentWithStore(t *testing.T) *Agent {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return &Agent{store: st}
}

func TestToolManageExchangeConfigLifecycle(t *testing.T) {
	a := newTestAgentWithStore(t)

	createResp := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"binance",
		"account_name":"Main",
		"enabled":true,
		"testnet":true
	}`)

	var created struct {
		Status   string                 `json:"status"`
		Action   string                 `json:"action"`
		Exchange safeExchangeToolConfig `json:"exchange"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal create response: %v\nraw=%s", err, createResp)
	}
	if created.Status != "ok" || created.Action != "create" {
		t.Fatalf("unexpected create response: %+v", created)
	}
	if created.Exchange.AccountName != "Main" || created.Exchange.ExchangeType != "binance" {
		t.Fatalf("unexpected exchange payload: %+v", created.Exchange)
	}

	updateResp := a.toolManageExchangeConfig("user-1", `{
		"action":"update",
		"exchange_id":"`+created.Exchange.ID+`",
		"account_name":"Renamed",
		"enabled":false
	}`)
	var updated struct {
		Status   string                 `json:"status"`
		Action   string                 `json:"action"`
		Exchange safeExchangeToolConfig `json:"exchange"`
	}
	if err := json.Unmarshal([]byte(updateResp), &updated); err != nil {
		t.Fatalf("unmarshal update response: %v\nraw=%s", err, updateResp)
	}
	if updated.Exchange.AccountName != "Renamed" || updated.Exchange.Enabled {
		t.Fatalf("unexpected updated exchange payload: %+v", updated.Exchange)
	}

	deleteResp := a.toolManageExchangeConfig("user-1", `{
		"action":"delete",
		"exchange_id":"`+created.Exchange.ID+`"
	}`)
	var deleted map[string]any
	if err := json.Unmarshal([]byte(deleteResp), &deleted); err != nil {
		t.Fatalf("unmarshal delete response: %v\nraw=%s", err, deleteResp)
	}
	if deleted["status"] != "ok" || deleted["action"] != "delete" {
		t.Fatalf("unexpected delete response: %+v", deleted)
	}
}

func TestToolManageModelConfigLifecycle(t *testing.T) {
	a := newTestAgentWithStore(t)

	createResp := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.openai.com/v1",
		"custom_model_name":"gpt-5-mini"
	}`)

	var created struct {
		Status string              `json:"status"`
		Action string              `json:"action"`
		Model  safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal create response: %v\nraw=%s", err, createResp)
	}
	if created.Status != "ok" || created.Action != "create" {
		t.Fatalf("unexpected create response: %+v", created)
	}
	if created.Model.Provider != "openai" || created.Model.CustomModelName != "gpt-5-mini" {
		t.Fatalf("unexpected model payload: %+v", created.Model)
	}

	updateResp := a.toolManageModelConfig("user-1", `{
		"action":"update",
		"model_id":"`+created.Model.ID+`",
		"enabled":false,
		"custom_model_name":"gpt-5"
	}`)
	var updated struct {
		Status string              `json:"status"`
		Action string              `json:"action"`
		Model  safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(updateResp), &updated); err != nil {
		t.Fatalf("unmarshal update response: %v\nraw=%s", err, updateResp)
	}
	if updated.Model.Enabled || updated.Model.CustomModelName != "gpt-5" {
		t.Fatalf("unexpected updated model payload: %+v", updated.Model)
	}

	deleteResp := a.toolManageModelConfig("user-1", `{
		"action":"delete",
		"model_id":"`+created.Model.ID+`"
	}`)
	var deleted map[string]any
	if err := json.Unmarshal([]byte(deleteResp), &deleted); err != nil {
		t.Fatalf("unmarshal delete response: %v\nraw=%s", err, deleteResp)
	}
	if deleted["status"] != "ok" || deleted["action"] != "delete" {
		t.Fatalf("unexpected delete response: %+v", deleted)
	}
}

func TestToolManageModelConfigRejectsEnableWithoutAPIKey(t *testing.T) {
	a := newTestAgentWithStore(t)

	createResp := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":false,
		"custom_model_name":"gpt-4o"
	}`)
	var created struct {
		Model safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal create response: %v\nraw=%s", err, createResp)
	}

	updateResp := a.toolManageModelConfig("user-1", `{
		"action":"update",
		"model_id":"`+created.Model.ID+`",
		"enabled":true
	}`)
	if !strings.Contains(updateResp, "cannot enable model config before API key is configured") {
		t.Fatalf("expected enabling incomplete model to fail, got %s", updateResp)
	}
}

func TestGetDefaultSkipsEnabledModelWithoutAPIKey(t *testing.T) {
	a := newTestAgentWithStore(t)

	incompleteCreate := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"custom_model_name":"gpt-4o"
	}`)
	var incomplete struct {
		Model safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(incompleteCreate), &incomplete); err != nil {
		t.Fatalf("unmarshal incomplete create response: %v\nraw=%s", err, incompleteCreate)
	}

	completeCreate := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"api_key":"sk-test",
		"custom_model_name":"deepseek-chat"
	}`)
	var complete struct {
		Model safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(completeCreate), &complete); err != nil {
		t.Fatalf("unmarshal complete create response: %v\nraw=%s", err, completeCreate)
	}

	model, err := a.store.AIModel().GetDefault("user-1")
	if err != nil {
		t.Fatalf("GetDefault() error = %v", err)
	}
	if model.ID != complete.Model.ID {
		t.Fatalf("expected GetDefault to skip incomplete enabled model and return %s, got %s", complete.Model.ID, model.ID)
	}
}

func TestToolManageTraderLifecycle(t *testing.T) {
	a := newTestAgentWithStore(t)

	modelResp := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"api_key":"sk-test",
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
		Status string               `json:"status"`
		Action string               `json:"action"`
		Trader safeTraderToolConfig `json:"trader"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal create trader response: %v\nraw=%s", err, createResp)
	}
	if created.Status != "ok" || created.Action != "create" {
		t.Fatalf("unexpected create trader response: %+v", created)
	}
	if created.Trader.Name != "Momentum Trader" || created.Trader.ScanIntervalMinutes != 5 {
		t.Fatalf("unexpected created trader: %+v", created.Trader)
	}

	listResp := a.toolManageTrader("user-1", `{"action":"list"}`)
	var listed struct {
		Count   int                    `json:"count"`
		Traders []safeTraderToolConfig `json:"traders"`
	}
	if err := json.Unmarshal([]byte(listResp), &listed); err != nil {
		t.Fatalf("unmarshal list response: %v\nraw=%s", err, listResp)
	}
	if listed.Count != 1 || len(listed.Traders) != 1 {
		t.Fatalf("unexpected trader list: %+v", listed)
	}

	updateResp := a.toolManageTrader("user-1", `{
		"action":"update",
		"trader_id":"`+created.Trader.ID+`",
		"name":"Renamed Trader",
		"scan_interval_minutes":8
	}`)
	var updated struct {
		Status string               `json:"status"`
		Action string               `json:"action"`
		Trader safeTraderToolConfig `json:"trader"`
	}
	if err := json.Unmarshal([]byte(updateResp), &updated); err != nil {
		t.Fatalf("unmarshal update trader response: %v\nraw=%s", err, updateResp)
	}
	if updated.Trader.Name != "Renamed Trader" || updated.Trader.ScanIntervalMinutes != 8 {
		t.Fatalf("unexpected updated trader: %+v", updated.Trader)
	}

	deleteResp := a.toolManageTrader("user-1", `{
		"action":"delete",
		"trader_id":"`+created.Trader.ID+`"
	}`)
	var deleted map[string]any
	if err := json.Unmarshal([]byte(deleteResp), &deleted); err != nil {
		t.Fatalf("unmarshal delete trader response: %v\nraw=%s", err, deleteResp)
	}
	if deleted["status"] != "ok" || deleted["action"] != "delete" {
		t.Fatalf("unexpected delete trader response: %+v", deleted)
	}
}

func TestToolManageStrategyLifecycle(t *testing.T) {
	a := newTestAgentWithStore(t)

	createResp := a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"激进",
		"description":"激进策略模板",
		"lang":"zh"
	}`)

	var created struct {
		Status   string                 `json:"status"`
		Action   string                 `json:"action"`
		Strategy safeStrategyToolConfig `json:"strategy"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal create response: %v\nraw=%s", err, createResp)
	}
	if created.Status != "ok" || created.Action != "create" {
		t.Fatalf("unexpected create response: %+v", created)
	}
	if created.Strategy.Name != "激进" {
		t.Fatalf("unexpected strategy payload: %+v", created.Strategy)
	}

	listResp := a.toolGetStrategies("user-1")
	if !strings.Contains(listResp, "激进") {
		t.Fatalf("expected created strategy in list, got %s", listResp)
	}

	updateResp := a.toolManageStrategy("user-1", `{
		"action":"update",
		"strategy_id":"`+created.Strategy.ID+`",
		"description":"更新后的描述"
	}`)
	var updated struct {
		Status   string                 `json:"status"`
		Action   string                 `json:"action"`
		Strategy safeStrategyToolConfig `json:"strategy"`
	}
	if err := json.Unmarshal([]byte(updateResp), &updated); err != nil {
		t.Fatalf("unmarshal update response: %v\nraw=%s", err, updateResp)
	}
	if updated.Strategy.Description != "更新后的描述" {
		t.Fatalf("unexpected updated strategy payload: %+v", updated.Strategy)
	}

	activateResp := a.toolManageStrategy("user-1", `{
		"action":"activate",
		"strategy_id":"`+created.Strategy.ID+`"
	}`)
	if !strings.Contains(activateResp, `"action":"activate"`) {
		t.Fatalf("unexpected activate response: %s", activateResp)
	}

	deleteResp := a.toolManageStrategy("user-1", `{
		"action":"delete",
		"strategy_id":"`+created.Strategy.ID+`"
	}`)
	if !strings.Contains(deleteResp, `"action":"delete"`) {
		t.Fatalf("unexpected delete response: %s", deleteResp)
	}
}

func TestLoadAIClientFromStoreUserUsesUserSpecificEnabledModel(t *testing.T) {
	a := newTestAgentWithStore(t)

	if err := a.store.AIModel().Update("user-42", "openai", true, "sk-test", "https://api.openai.com/v1", "gpt-5-mini"); err != nil {
		t.Fatalf("seed model: %v", err)
	}

	client, modelName, ok := a.loadAIClientFromStoreUser("user-42")
	if !ok {
		t.Fatal("expected AI client to load from user-specific model")
	}
	if client == nil {
		t.Fatal("expected non-nil AI client")
	}
	if modelName != "gpt-5-mini" {
		t.Fatalf("unexpected model name: %s", modelName)
	}

	// After the provider registry refactor, registered providers (like openai)
	// return their own AIClient implementation, not *mcp.Client.
	if client == nil {
		t.Fatal("expected non-nil AI client from provider registry")
	}
}
