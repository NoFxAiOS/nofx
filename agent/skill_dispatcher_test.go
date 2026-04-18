package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateTraderSkillCollectsMissingFieldsAndCreatesTrader(t *testing.T) {
	a := newTestAgentWithStore(t)

	modelResp := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.deepseek.com/v1",
		"custom_model_name":"deepseek-chat"
	}`)
	if strings.Contains(modelResp, `"error"`) {
		t.Fatalf("failed to create model: %s", modelResp)
	}
	exchangeResp := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"主账户",
		"enabled":true
	}`)
	if strings.Contains(exchangeResp, `"error"`) {
		t.Fatalf("failed to create exchange: %s", exchangeResp)
	}
	strategyResp := a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"趋势策略",
		"lang":"zh"
	}`)
	if strings.Contains(strategyResp, `"error"`) {
		t.Fatalf("failed to create strategy: %s", strategyResp)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 1, "zh", "帮我创建一个交易员")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "还缺这些信息") || !strings.Contains(resp, "名称") {
		t.Fatalf("expected missing-field prompt, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 1, "zh", "叫 波段一号")
	if err != nil {
		t.Fatalf("thinkAndAct() second turn error = %v", err)
	}
	if !strings.Contains(resp, "已创建交易员") || !strings.Contains(resp, "波段一号") {
		t.Fatalf("expected trader creation confirmation, got %q", resp)
	}

	listResp := a.toolListTraders("user-1")
	if !strings.Contains(listResp, "波段一号") {
		t.Fatalf("expected created trader in list, got %s", listResp)
	}
}

func TestCreateTraderSkillRequestsStartConfirmation(t *testing.T) {
	a := newTestAgentWithStore(t)

	_ = a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.openai.com/v1",
		"custom_model_name":"gpt-5"
	}`)
	_ = a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"binance",
		"account_name":"Main",
		"enabled":true
	}`)
	_ = a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"保守策略",
		"lang":"zh"
	}`)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 2, "zh", "创建一个叫“实盘一号”的交易员并启动")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "高风险动作") || !strings.Contains(resp, "确认") {
		t.Fatalf("expected start confirmation prompt, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 2, "zh", "先不用")
	if err != nil {
		t.Fatalf("thinkAndAct() confirmation error = %v", err)
	}
	if !strings.Contains(resp, "已创建交易员") || strings.Contains(resp, "已创建并启动") {
		t.Fatalf("expected create-without-start response, got %q", resp)
	}
}

func TestModelDiagnosisSkillHandledWithoutAIClient(t *testing.T) {
	a := newTestAgentWithStore(t)
	resp, err := a.thinkAndAct(context.Background(), "user-1", 3, "zh", "为什么我的模型配置失败了")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "模型配置") {
		t.Fatalf("expected model diagnosis response, got %q", resp)
	}
}

func TestExchangeDiagnosisSkillHandledWithoutAIClient(t *testing.T) {
	a := newTestAgentWithStore(t)
	resp, err := a.thinkAndAct(context.Background(), "user-1", 4, "zh", "交易所 API 报 invalid signature 怎么办")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "invalid signature") && !strings.Contains(resp, "签名") {
		t.Fatalf("expected exchange diagnosis response, got %q", resp)
	}
}

func TestExchangeManagementCreateAndQuerySkill(t *testing.T) {
	a := newTestAgentWithStore(t)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 5, "zh", "帮我创建一个 OKX 交易所配置")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "已创建交易所配置") {
		t.Fatalf("expected exchange create response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 5, "zh", "列出我的交易所配置")
	if err != nil {
		t.Fatalf("thinkAndAct() query error = %v", err)
	}
	if !strings.Contains(resp, "当前交易所配置") && !strings.Contains(resp, "Default") {
		t.Fatalf("expected exchange query response, got %q", resp)
	}
}

func TestModelManagementCreateSkill(t *testing.T) {
	a := newTestAgentWithStore(t)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 6, "zh", "帮我创建一个 DeepSeek 模型配置")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "已创建模型配置") {
		t.Fatalf("expected model create response, got %q", resp)
	}
}

func TestStrategyManagementCreateAndActivateSkill(t *testing.T) {
	a := newTestAgentWithStore(t)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 7, "zh", "创建一个叫“趋势策略B”的策略")
	if err != nil {
		t.Fatalf("thinkAndAct() create error = %v", err)
	}
	if !strings.Contains(resp, "已创建策略") {
		t.Fatalf("expected strategy create response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 7, "zh", "激活趋势策略B")
	if err != nil {
		t.Fatalf("thinkAndAct() activate error = %v", err)
	}
	if !strings.Contains(resp, "已激活策略") {
		t.Fatalf("expected strategy activate response, got %q", resp)
	}
}

func TestTraderManagementQueryAndDiagnosisSkill(t *testing.T) {
	a := newTestAgentWithStore(t)

	modelResp := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.openai.com/v1",
		"custom_model_name":"gpt-5"
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
	_ = a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"测试策略",
		"lang":"zh"
	}`)
	_ = a.toolManageTrader("user-1", `{
		"action":"create",
		"name":"测试交易员",
		"ai_model_id":"`+modelCreated.Model.ID+`",
		"exchange_id":"`+exchangeCreated.Exchange.ID+`",
		"strategy_id":""
	}`)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 8, "zh", "查看我的交易员")
	if err != nil {
		t.Fatalf("thinkAndAct() query error = %v", err)
	}
	if !strings.Contains(resp, "当前交易员") && !strings.Contains(resp, "测试交易员") {
		t.Fatalf("expected trader query response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 8, "zh", "为什么我的交易员不交易")
	if err != nil {
		t.Fatalf("thinkAndAct() diagnosis error = %v", err)
	}
	if !strings.Contains(resp, "交易员运行诊断") {
		t.Fatalf("expected trader diagnosis response, got %q", resp)
	}
}
