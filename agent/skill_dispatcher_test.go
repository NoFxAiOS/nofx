package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"nofx/mcp"
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

func TestCreateTraderSkillReportsAllMissingPrerequisitesAtOnce(t *testing.T) {
	a := newTestAgentWithStore(t)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 11, "zh", "帮我创建一个交易员")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	for _, want := range []string{"名称", "交易所", "模型", "策略"} {
		if !strings.Contains(resp, want) {
			t.Fatalf("expected response to mention %q, got %q", want, resp)
		}
	}
	for _, want := range []string{"当前还没有可用交易所配置", "当前还没有可用模型配置", "当前还没有可用策略"} {
		if !strings.Contains(resp, want) {
			t.Fatalf("expected response to mention prerequisite %q, got %q", want, resp)
		}
	}
}

func TestActiveSkillSessionYieldsToNewTopic(t *testing.T) {
	a := newTestAgentWithStore(t)

	_ = a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"测试策略",
		"lang":"zh"
	}`)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 13, "zh", "帮我创建一个交易员")
	if err != nil {
		t.Fatalf("thinkAndAct() error = %v", err)
	}
	if !strings.Contains(resp, "还缺这些信息") {
		t.Fatalf("expected trader creation flow prompt, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 13, "zh", "列出我当前的策略")
	if err != nil {
		t.Fatalf("thinkAndAct() interrupt error = %v", err)
	}
	if !strings.Contains(resp, "当前策略") || !strings.Contains(resp, "测试策略") {
		t.Fatalf("expected new topic to be handled, got %q", resp)
	}
	if a.hasActiveSkillSession(13) {
		t.Fatal("expected skill session to be cleared after interruption")
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

func TestStrategyManagementQueryCanExplainStrategyDetails(t *testing.T) {
	a := newTestAgentWithStore(t)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 12, "zh", "创建一个叫“激进的”的策略")
	if err != nil {
		t.Fatalf("thinkAndAct() create error = %v", err)
	}
	if !strings.Contains(resp, "已创建策略") {
		t.Fatalf("expected strategy create response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 12, "zh", "这个策略里面的参数和prompt分别是什么样的")
	if err != nil {
		t.Fatalf("thinkAndAct() detail query error = %v", err)
	}
	for _, want := range []string{"策略“激进的”概览", "K线周期", "仓位风险", "Prompt"} {
		if !strings.Contains(resp, want) {
			t.Fatalf("expected response to mention %q, got %q", want, resp)
		}
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

func TestExchangeManagementAtomicUpdates(t *testing.T) {
	a := newTestAgentWithStore(t)

	createResp := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"主账户",
		"enabled":true
	}`)
	var created struct {
		Exchange safeExchangeToolConfig `json:"exchange"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal exchange response: %v", err)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 14, "zh", "更新交易所，把主账户改名为备用账户")
	if err != nil {
		t.Fatalf("rename exchange error = %v", err)
	}
	if !strings.Contains(resp, "已更新交易所配置") {
		t.Fatalf("expected exchange update response, got %q", resp)
	}

	raw := a.toolGetExchangeConfigs("user-1")
	if !strings.Contains(raw, "备用账户") {
		t.Fatalf("expected renamed exchange in list, got %s", raw)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 14, "zh", "禁用这个交易所配置")
	if err != nil {
		t.Fatalf("disable exchange error = %v", err)
	}
	if !strings.Contains(resp, "已更新交易所配置") {
		t.Fatalf("expected exchange status update response, got %q", resp)
	}

	raw = a.toolGetExchangeConfigs("user-1")
	if strings.Contains(raw, `"enabled":true`) && strings.Contains(raw, "备用账户") {
		t.Fatalf("expected exchange to be disabled, got %s", raw)
	}
}

func TestModelManagementAtomicUpdates(t *testing.T) {
	a := newTestAgentWithStore(t)

	createResp := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"custom_api_url":"https://api.deepseek.com/v1",
		"custom_model_name":"deepseek-chat"
	}`)
	var created struct {
		Model safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(createResp), &created); err != nil {
		t.Fatalf("unmarshal model response: %v", err)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 15, "zh", "更新模型，把模型名称改成 deepseek-reasoner")
	if err != nil {
		t.Fatalf("rename model error = %v", err)
	}
	if !strings.Contains(resp, "已更新模型配置") {
		t.Fatalf("expected model update response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 15, "zh", "更新模型，把接口地址改成 https://api.deepseek.com/beta")
	if err != nil {
		t.Fatalf("update model endpoint error = %v", err)
	}
	if !strings.Contains(resp, "已更新模型配置") {
		t.Fatalf("expected model endpoint update response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 15, "zh", "禁用这个模型配置")
	if err != nil {
		t.Fatalf("disable model error = %v", err)
	}
	if !strings.Contains(resp, "已更新模型配置") {
		t.Fatalf("expected model status update response, got %q", resp)
	}

	raw := a.toolGetModelConfigs("user-1")
	if !strings.Contains(raw, "deepseek-reasoner") || !strings.Contains(raw, "https://api.deepseek.com/beta") {
		t.Fatalf("expected updated model fields, got %s", raw)
	}
	if strings.Contains(raw, `"enabled":true`) && strings.Contains(raw, created.Model.ID) {
		t.Fatalf("expected model to be disabled, got %s", raw)
	}
}

func TestStrategyManagementAtomicUpdates(t *testing.T) {
	a := newTestAgentWithStore(t)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 16, "zh", "创建一个叫“激进策略C”的策略")
	if err != nil {
		t.Fatalf("create strategy error = %v", err)
	}
	if !strings.Contains(resp, "已创建策略") {
		t.Fatalf("expected strategy create response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 16, "zh", "更新这个策略的prompt，把提示词改成“优先观察BTC和ETH，信号不一致时不要开仓”")
	if err != nil {
		t.Fatalf("update strategy prompt error = %v", err)
	}
	if !strings.Contains(resp, "已更新策略 prompt") {
		t.Fatalf("expected strategy prompt update response, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 16, "zh", "更新这个策略参数，把最大持仓改成2，最低置信度改成80，主周期改成15m，并使用15m 1h 4h")
	if err != nil {
		t.Fatalf("update strategy config error = %v", err)
	}
	if !strings.Contains(resp, "已更新策略参数") {
		t.Fatalf("expected strategy config update response, got %q", resp)
	}

	listRaw := a.toolGetStrategies("user-1")
	if !strings.Contains(listRaw, "优先观察BTC和ETH") || !strings.Contains(listRaw, `"max_positions":2`) || !strings.Contains(listRaw, `"min_confidence":80`) || !strings.Contains(listRaw, `"primary_timeframe":"15m"`) {
		t.Fatalf("expected updated strategy config, got %s", listRaw)
	}
}

func TestTraderManagementAtomicBindingUpdate(t *testing.T) {
	a := newTestAgentWithStore(t)

	modelOpenAI := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"openai",
		"enabled":true,
		"custom_api_url":"https://api.openai.com/v1",
		"custom_model_name":"gpt-5-mini"
	}`)
	var openAI struct {
		Model safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(modelOpenAI), &openAI); err != nil {
		t.Fatalf("unmarshal openai model: %v", err)
	}
	modelDeepSeek := a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"custom_api_url":"https://api.deepseek.com/v1",
		"custom_model_name":"deepseek-chat"
	}`)
	var deepSeek struct {
		Model safeModelToolConfig `json:"model"`
	}
	if err := json.Unmarshal([]byte(modelDeepSeek), &deepSeek); err != nil {
		t.Fatalf("unmarshal deepseek model: %v", err)
	}

	exchangeBinance := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"binance",
		"account_name":"Binance 主账户",
		"enabled":true
	}`)
	var binance struct {
		Exchange safeExchangeToolConfig `json:"exchange"`
	}
	if err := json.Unmarshal([]byte(exchangeBinance), &binance); err != nil {
		t.Fatalf("unmarshal binance exchange: %v", err)
	}
	exchangeOKX := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"OKX 主账户",
		"enabled":true
	}`)
	var okx struct {
		Exchange safeExchangeToolConfig `json:"exchange"`
	}
	if err := json.Unmarshal([]byte(exchangeOKX), &okx); err != nil {
		t.Fatalf("unmarshal okx exchange: %v", err)
	}

	strategyA := a.toolManageStrategy("user-1", `{"action":"create","name":"策略A","lang":"zh"}`)
	var stA struct {
		Strategy safeStrategyToolConfig `json:"strategy"`
	}
	if err := json.Unmarshal([]byte(strategyA), &stA); err != nil {
		t.Fatalf("unmarshal strategy A: %v", err)
	}
	strategyB := a.toolManageStrategy("user-1", `{"action":"create","name":"策略B","lang":"zh"}`)
	var stB struct {
		Strategy safeStrategyToolConfig `json:"strategy"`
	}
	if err := json.Unmarshal([]byte(strategyB), &stB); err != nil {
		t.Fatalf("unmarshal strategy B: %v", err)
	}

	createTrader := a.toolManageTrader("user-1", `{
		"action":"create",
		"name":"实盘一号",
		"ai_model_id":"`+openAI.Model.ID+`",
		"exchange_id":"`+binance.Exchange.ID+`",
		"strategy_id":"`+stA.Strategy.ID+`"
	}`)
	var trader struct {
		Trader safeTraderToolConfig `json:"trader"`
	}
	if err := json.Unmarshal([]byte(createTrader), &trader); err != nil {
		t.Fatalf("unmarshal trader: %v", err)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 17, "zh", "更新交易员绑定，把实盘一号换成 deepseek-chat、OKX 主账户 和 策略B")
	if err != nil {
		t.Fatalf("update trader bindings error = %v", err)
	}
	if !strings.Contains(resp, "已更新交易员绑定") {
		t.Fatalf("expected trader binding update response, got %q", resp)
	}

	listRaw := a.toolListTraders("user-1")
	if !strings.Contains(listRaw, deepSeek.Model.ID) || !strings.Contains(listRaw, okx.Exchange.ID) || !strings.Contains(listRaw, stB.Strategy.ID) {
		t.Fatalf("expected trader bindings to change, got %s", listRaw)
	}
}

func TestStrategyManagementDeleteAllUserStrategies(t *testing.T) {
	a := newTestAgentWithStore(t)

	for _, name := range []string{"趋势策略A", "趋势策略B"} {
		resp := a.toolManageStrategy("user-1", `{
			"action":"create",
			"name":"`+name+`",
			"lang":"zh"
		}`)
		if strings.Contains(resp, `"error"`) {
			t.Fatalf("failed to create strategy %q: %s", name, resp)
		}
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 21, "zh", "现在把所有的策略全部删除")
	if err != nil {
		t.Fatalf("thinkAndAct() bulk delete start error = %v", err)
	}
	if !strings.Contains(resp, "确认") || !strings.Contains(resp, "全部自定义策略") {
		t.Fatalf("expected bulk delete confirmation, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 21, "zh", "确认")
	if err != nil {
		t.Fatalf("thinkAndAct() bulk delete confirm error = %v", err)
	}
	if !strings.Contains(resp, "成功删除 2 个") {
		t.Fatalf("expected bulk delete success summary, got %q", resp)
	}

	listResp := a.toolGetStrategies("user-1")
	if strings.Contains(listResp, "趋势策略A") || strings.Contains(listResp, "趋势策略B") {
		t.Fatalf("expected created strategies to be deleted, got %s", listResp)
	}
}

func TestCreateTraderSkillRejectsDisabledExchangeWithClearPrompt(t *testing.T) {
	a := newTestAgentWithStore(t)

	_ = a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.deepseek.com/v1",
		"custom_model_name":"deepseek-chat"
	}`)
	enabledExchange := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"test",
		"enabled":true
	}`)
	if strings.Contains(enabledExchange, `"error"`) {
		t.Fatalf("failed to create enabled exchange: %s", enabledExchange)
	}
	anotherEnabledExchange := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"lky",
		"enabled":true
	}`)
	if strings.Contains(anotherEnabledExchange, `"error"`) {
		t.Fatalf("failed to create second enabled exchange: %s", anotherEnabledExchange)
	}
	disabledExchange := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"new",
		"enabled":false
	}`)
	if strings.Contains(disabledExchange, `"error"`) {
		t.Fatalf("failed to create disabled exchange: %s", disabledExchange)
	}
	_ = a.toolManageStrategy("user-1", `{"action":"create","name":"激进","lang":"zh"}`)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 24, "zh", "给我创建一个trader")
	if err != nil {
		t.Fatalf("create trader start error = %v", err)
	}
	if !strings.Contains(resp, "new（已禁用）") {
		t.Fatalf("expected disabled exchange to be labelled, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 24, "zh", "名称叫test，交易所用new、策略用激进")
	if err != nil {
		t.Fatalf("disabled exchange selection error = %v", err)
	}
	if !strings.Contains(resp, "当前已禁用") {
		t.Fatalf("expected disabled exchange warning, got %q", resp)
	}
}

func TestCancelReplyExitsExchangeUpdateFlow(t *testing.T) {
	a := newTestAgentWithStore(t)
	_ = a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.deepseek.com/v1",
		"custom_model_name":"deepseek-chat"
	}`)

	exchangeResp := a.toolManageExchangeConfig("user-1", `{
		"action":"create",
		"exchange_type":"okx",
		"account_name":"test",
		"enabled":true
	}`)
	if strings.Contains(exchangeResp, `"error"`) {
		t.Fatalf("failed to create exchange: %s", exchangeResp)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 25, "zh", "把test这个交易所改一下")
	if err != nil {
		t.Fatalf("enter exchange update flow error = %v", err)
	}
	if !strings.Contains(resp, "请告诉我你要改什么") {
		t.Fatalf("expected exchange update prompt, got %q", resp)
	}

	resp, err = a.thinkAndAct(context.Background(), "user-1", 25, "zh", "不改")
	if err != nil {
		t.Fatalf("cancel exchange flow error = %v", err)
	}
	if !strings.Contains(resp, "已取消当前流程") {
		t.Fatalf("expected flow cancellation, got %q", resp)
	}
}

func TestClassifySkillSessionInputInterruptsOnDeflection(t *testing.T) {
	session := skillSession{Name: "exchange_management", Action: "update"}
	a := &Agent{}

	if got := a.classifySkillSessionInput(context.Background(), 0, "zh", session, "你能帮我看下报错吗"); got != "interrupt" {
		t.Fatalf("expected diagnosis deflection to interrupt current skill flow, got %q", got)
	}
	if got := a.classifySkillSessionInput(context.Background(), 0, "zh", session, "换话题了大哥"); got != "cancel" {
		t.Fatalf("expected topic shift to cancel current skill flow, got %q", got)
	}
}

type skillSessionClassifierAIClient struct {
	lastSystemPrompt string
	lastUserPrompt   string
	response         string
}

func (c *skillSessionClassifierAIClient) SetAPIKey(string, string, string) {}
func (c *skillSessionClassifierAIClient) SetTimeout(time.Duration)         {}
func (c *skillSessionClassifierAIClient) CallWithMessages(string, string) (string, error) {
	return "", errors.New("unexpected CallWithMessages")
}
func (c *skillSessionClassifierAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	if len(req.Messages) > 0 {
		c.lastSystemPrompt = req.Messages[0].Content
	}
	if len(req.Messages) > 1 {
		c.lastUserPrompt = req.Messages[1].Content
	}
	return c.response, nil
}
func (c *skillSessionClassifierAIClient) CallWithRequestStream(*mcp.Request, func(string)) (string, error) {
	return "", errors.New("unexpected CallWithRequestStream")
}
func (c *skillSessionClassifierAIClient) CallWithRequestFull(*mcp.Request) (*mcp.LLMResponse, error) {
	return nil, errors.New("unexpected CallWithRequestFull")
}

func TestClassifySkillSessionInputUsesSlotExpectationWithoutLLM(t *testing.T) {
	client := &skillSessionClassifierAIClient{response: `{"decision":"interrupt"}`}
	a := &Agent{aiClient: client}
	session := skillSession{
		Name:   "strategy_management",
		Action: "update_config",
		Fields: map[string]string{
			skillDAGStepField: "resolve_config_value",
			"config_field":    "min_confidence",
		},
	}

	if got := a.classifySkillSessionInput(context.Background(), 0, "zh", session, "70"); got != "continue" {
		t.Fatalf("expected numeric slot fill to continue, got %q", got)
	}
	if client.lastSystemPrompt != "" {
		t.Fatalf("expected no LLM call for direct slot expectation, got prompt %q", client.lastSystemPrompt)
	}
}

func TestClassifySkillSessionInputUsesLLMOnlyForAmbiguousDeflection(t *testing.T) {
	client := &skillSessionClassifierAIClient{response: `{"decision":"interrupt"}`}
	a := &Agent{
		aiClient: client,
		history:  newChatHistory(10),
	}
	session := skillSession{
		Name:   "exchange_management",
		Action: "update",
		Fields: map[string]string{
			skillDAGStepField: "collect_account_name",
		},
	}

	if got := a.classifySkillSessionInput(context.Background(), 0, "zh", session, "你能帮我看下报错吗"); got != "interrupt" {
		t.Fatalf("expected ambiguous deflection to interrupt, got %q", got)
	}
	if !strings.Contains(client.lastSystemPrompt, "classify one user message while a NOFXi structured management flow is active") {
		t.Fatalf("expected LLM classifier prompt, got %q", client.lastSystemPrompt)
	}
}

func TestClassifySkillSessionInputUsesLLMForUnmatchedActiveSessionInput(t *testing.T) {
	client := &skillSessionClassifierAIClient{response: `{"decision":"continue"}`}
	a := &Agent{
		aiClient: client,
		history:  newChatHistory(10),
	}
	session := skillSession{
		Name:   "model_management",
		Action: "create",
		Fields: map[string]string{
			skillDAGStepField: "collect_optional_fields",
			"provider":        "openai",
		},
	}

	if got := a.classifySkillSessionInput(context.Background(), 0, "zh", session, "新增一个"); got != "continue" {
		t.Fatalf("expected unmatched active-session input to follow LLM decision, got %q", got)
	}
	if !strings.Contains(client.lastSystemPrompt, "classify one user message while a NOFXi structured management flow is active") {
		t.Fatalf("expected LLM classifier prompt, got %q", client.lastSystemPrompt)
	}
}

func TestStrategyManagementCanDescribeDefaultConfig(t *testing.T) {
	a := newTestAgentWithStore(t)
	_ = a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.deepseek.com/v1",
		"custom_model_name":"deepseek-chat"
	}`)

	resp, err := a.thinkAndAct(context.Background(), "user-1", 22, "zh", "看一下默认配置")
	if err != nil {
		t.Fatalf("thinkAndAct() default config error = %v", err)
	}
	if !strings.Contains(resp, "默认策略模板") || !strings.Contains(resp, "最低置信度") {
		t.Fatalf("expected default strategy config response, got %q", resp)
	}
}

func TestStrategyManagementSupportsMultiFieldConfigUpdate(t *testing.T) {
	a := newTestAgentWithStore(t)
	_ = a.toolManageModelConfig("user-1", `{
		"action":"create",
		"provider":"deepseek",
		"enabled":true,
		"api_key":"sk-test",
		"custom_api_url":"https://api.deepseek.com/v1",
		"custom_model_name":"deepseek-chat"
	}`)

	createResp := a.toolManageStrategy("user-1", `{
		"action":"create",
		"name":"趋势策略A",
		"lang":"zh"
	}`)
	if strings.Contains(createResp, `"error"`) {
		t.Fatalf("failed to create strategy: %s", createResp)
	}

	resp, err := a.thinkAndAct(context.Background(), "user-1", 23, "zh", "把趋势策略A的最小置信度改成70，核心指标都全选")
	if err != nil {
		t.Fatalf("thinkAndAct() multi-field update error = %v", err)
	}
	if !strings.Contains(resp, "最小置信度") || !strings.Contains(resp, "EMA") {
		t.Fatalf("expected multi-field update confirmation, got %q", resp)
	}

	strategiesRaw := a.toolGetStrategies("user-1")
	if !strings.Contains(strategiesRaw, `"min_confidence":70`) ||
		!strings.Contains(strategiesRaw, `"enable_ema":true`) ||
		!strings.Contains(strategiesRaw, `"enable_macd":true`) ||
		!strings.Contains(strategiesRaw, `"enable_rsi":true`) ||
		!strings.Contains(strategiesRaw, `"enable_atr":true`) ||
		!strings.Contains(strategiesRaw, `"enable_boll":true`) {
		t.Fatalf("expected strategy config to include updated confidence and indicators, got %s", strategiesRaw)
	}
}
