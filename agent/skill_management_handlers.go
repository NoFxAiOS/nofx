package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`https://[^\s"'<>]+`)

func detectTraderManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{"交易员", "trader", "agent"}) &&
		containsAny(lower, []string{"修改", "编辑", "更新", "删除", "启动", "停止", "查看", "查询", "列出", "rename", "update", "delete", "start", "stop", "list", "show"})
}

func detectExchangeManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{"交易所", "exchange", "okx", "binance", "bybit", "gate", "kucoin", "hyperliquid"}) &&
		containsAny(lower, []string{"创建", "新建", "修改", "编辑", "更新", "删除", "查询", "查看", "列出", "create", "update", "delete", "list", "show"})
}

func detectModelManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{"模型", "model", "provider", "deepseek", "openai", "claude", "gemini", "qwen", "kimi", "grok", "minimax"}) &&
		containsAny(lower, []string{"创建", "新建", "修改", "编辑", "更新", "删除", "查询", "查看", "列出", "create", "update", "delete", "list", "show"})
}

func detectStrategyManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{"策略", "strategy"}) &&
		containsAny(lower, []string{"创建", "新建", "修改", "编辑", "更新", "删除", "查询", "查看", "列出", "激活", "复制", "create", "update", "delete", "list", "show", "activate", "duplicate"})
}

func detectTraderDiagnosisSkill(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAny(lower, []string{"交易员", "trader"}) &&
		containsAny(lower, []string{"启动失败", "不交易", "没开仓", "无法启动", "异常", "失败", "diagnose", "error", "not trading"})
}

func detectStrategyDiagnosisSkill(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAny(lower, []string{"策略", "strategy", "prompt"}) &&
		containsAny(lower, []string{"不生效", "没生效", "异常", "失败", "不一致", "失效", "diagnose", "error"})
}

func detectManagementAction(text string, domain string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return ""
	}
	switch {
	case containsAny(lower, []string{"删除", "删掉", "remove", "delete"}):
		return "delete"
	case containsAny(lower, []string{"启动", "开始", "run", "start"}) && domain == "trader":
		return "start"
	case containsAny(lower, []string{"停止", "停掉", "stop", "pause"}) && domain == "trader":
		return "stop"
	case containsAny(lower, []string{"激活", "activate"}) && domain == "strategy":
		return "activate"
	case containsAny(lower, []string{"复制", "duplicate"}) && domain == "strategy":
		return "duplicate"
	case containsAny(lower, []string{"修改", "编辑", "更新", "改", "rename", "update"}):
		return "update"
	case containsAny(lower, []string{"查询", "查看", "列出", "list", "show", "有哪些"}):
		return "query"
	case containsAny(lower, []string{"创建", "新建", "加一个", "create", "new"}):
		return "create"
	default:
		return ""
	}
}

func exchangeTypeFromText(text string) string {
	lower := strings.ToLower(text)
	candidates := []string{"binance", "okx", "bybit", "gate", "kucoin", "hyperliquid", "aster", "lighter"}
	for _, candidate := range candidates {
		if strings.Contains(lower, candidate) {
			return candidate
		}
	}
	switch {
	case strings.Contains(text, "币安"):
		return "binance"
	case strings.Contains(text, "欧易"):
		return "okx"
	case strings.Contains(text, "库币"):
		return "kucoin"
	default:
		return ""
	}
}

func providerFromText(text string) string {
	lower := strings.ToLower(text)
	candidates := []string{"openai", "deepseek", "claude", "gemini", "qwen", "kimi", "grok", "minimax"}
	for _, candidate := range candidates {
		if strings.Contains(lower, candidate) {
			return candidate
		}
	}
	if strings.Contains(text, "通义") {
		return "qwen"
	}
	return ""
}

func extractURL(text string) string {
	return strings.TrimSpace(urlPattern.FindString(text))
}

func extractPostKeywordName(text string, keywords []string) string {
	trimmed := strings.TrimSpace(text)
	for _, keyword := range keywords {
		if idx := strings.Index(trimmed, keyword); idx >= 0 {
			name := strings.TrimSpace(trimmed[idx+len(keyword):])
			name = strings.Trim(name, "“”\"'：: ")
			if name != "" && len([]rune(name)) <= 50 {
				return name
			}
		}
	}
	return ""
}

func setField(session *skillSession, key, value string) {
	ensureSkillFields(session)
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	session.Fields[key] = value
}

func fieldValue(session skillSession, key string) string {
	if session.Fields == nil {
		return ""
	}
	return strings.TrimSpace(session.Fields[key])
}

func resolveTargetFromText(text string, options []traderSkillOption, existing *EntityReference) *EntityReference {
	if existing != nil && (existing.ID != "" || existing.Name != "") {
		return existing
	}
	if match := pickMentionedOption(text, options); match != nil {
		return &EntityReference{ID: match.ID, Name: match.Name}
	}
	if choice := choosePreferredOption(options); choice != nil {
		return &EntityReference{ID: choice.ID, Name: choice.Name}
	}
	return nil
}

func (a *Agent) handleTraderManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	action := detectManagementAction(text, "trader")
	if session.Name == "trader_management" && session.Action != "" {
		action = session.Action
	}
	if action == "" || action == "create" {
		return "", false
	}
	return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "trader_management", action, a.loadTraderOptions(storeUserID))
}

func (a *Agent) handleExchangeManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	action := detectManagementAction(text, "exchange")
	if session.Name == "exchange_management" && session.Action != "" {
		action = session.Action
	}
	if action == "" {
		return "", false
	}
	options := a.loadExchangeOptions(storeUserID)
	switch action {
	case "query":
		return formatReadFastPathResponse(lang, "get_exchange_configs", a.toolGetExchangeConfigs(storeUserID)), true
	case "create":
		return a.handleExchangeCreateSkill(storeUserID, userID, lang, text, session), true
	default:
		return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "exchange_management", action, options)
	}
}

func (a *Agent) handleModelManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	action := detectManagementAction(text, "model")
	if session.Name == "model_management" && session.Action != "" {
		action = session.Action
	}
	if action == "" {
		return "", false
	}
	options := a.loadEnabledModelOptions(storeUserID)
	switch action {
	case "query":
		return formatReadFastPathResponse(lang, "get_model_configs", a.toolGetModelConfigs(storeUserID)), true
	case "create":
		return a.handleModelCreateSkill(storeUserID, userID, lang, text, session), true
	default:
		return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "model_management", action, options)
	}
}

func (a *Agent) handleStrategyManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	action := detectManagementAction(text, "strategy")
	if session.Name == "strategy_management" && session.Action != "" {
		action = session.Action
	}
	if action == "" {
		return "", false
	}
	options := a.loadStrategyOptions(storeUserID)
	switch action {
	case "query":
		return formatReadFastPathResponse(lang, "get_strategies", a.toolGetStrategies(storeUserID)), true
	case "create":
		return a.handleStrategyCreateSkill(storeUserID, userID, lang, text, session), true
	default:
		return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "strategy_management", action, options)
	}
}

func (a *Agent) loadTraderOptions(storeUserID string) []traderSkillOption {
	if a.store == nil {
		return nil
	}
	traders, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return nil
	}
	out := make([]traderSkillOption, 0, len(traders))
	for _, trader := range traders {
		out = append(out, traderSkillOption{ID: trader.ID, Name: trader.Name, Enabled: trader.IsRunning})
	}
	return out
}

func (a *Agent) handleExchangeCreateSkill(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.Name == "" {
		session = skillSession{Name: "exchange_management", Action: "create", Phase: "collecting"}
	}
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前创建交易所配置流程。"
		}
		return "Cancelled the current exchange creation flow."
	}
	if v := exchangeTypeFromText(text); fieldValue(session, "exchange_type") == "" && v != "" {
		setField(&session, "exchange_type", v)
	}
	if v := extractTraderName(text); fieldValue(session, "account_name") == "" && v != "" {
		setField(&session, "account_name", v)
	}
	exType := fieldValue(session, "exchange_type")
	if actionRequiresSlot("exchange_management", "create", "exchange_type") && exType == "" {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建交易所配置，我还需要：" + slotDisplayName("exchange_type", lang) + "。例如：OKX、Binance、Bybit。"
		}
		return "To create an exchange config, tell me which exchange to use, for example OKX, Binance, or Bybit."
	}
	accountName := fieldValue(session, "account_name")
	if accountName == "" {
		accountName = "Default"
	}
	args := map[string]any{
		"action":        "create",
		"exchange_type": exType,
		"account_name":  accountName,
	}
	raw, _ := json.Marshal(args)
	resp := a.toolManageExchangeConfig(storeUserID, string(raw))
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建交易所配置失败：" + errMsg
		}
		return "Failed to create exchange config: " + errMsg
	}
	a.clearSkillSession(userID)
	if lang == "zh" {
		return fmt.Sprintf("已创建交易所配置：%s（%s）。如需继续补 API Key、Secret 或 Passphrase，可以直接继续说。", accountName, exType)
	}
	return fmt.Sprintf("Created exchange config %s (%s). You can continue by adding API key, secret, or passphrase.", accountName, exType)
}

func (a *Agent) handleModelCreateSkill(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.Name == "" {
		session = skillSession{Name: "model_management", Action: "create", Phase: "collecting"}
	}
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前创建模型配置流程。"
		}
		return "Cancelled the current model creation flow."
	}
	if v := providerFromText(text); fieldValue(session, "provider") == "" && v != "" {
		setField(&session, "provider", v)
	}
	if v := extractTraderName(text); fieldValue(session, "name") == "" && v != "" {
		setField(&session, "name", v)
	}
	if v := extractURL(text); fieldValue(session, "custom_api_url") == "" && v != "" {
		setField(&session, "custom_api_url", v)
	}
	provider := fieldValue(session, "provider")
	if actionRequiresSlot("model_management", "create", "provider") && provider == "" {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建模型配置，我还需要：" + slotDisplayName("provider", lang) + "，例如：OpenAI、DeepSeek、Claude、Gemini。"
		}
		return "To create a model config, I need the provider first, for example OpenAI, DeepSeek, Claude, or Gemini."
	}
	args := map[string]any{
		"action":           "create",
		"provider":         provider,
		"name":             defaultIfEmpty(fieldValue(session, "name"), provider),
		"custom_api_url":   fieldValue(session, "custom_api_url"),
		"custom_model_name": fieldValue(session, "custom_model_name"),
	}
	raw, _ := json.Marshal(args)
	resp := a.toolManageModelConfig(storeUserID, string(raw))
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建模型配置失败：" + errMsg
		}
		return "Failed to create model config: " + errMsg
	}
	a.clearSkillSession(userID)
	if lang == "zh" {
		return fmt.Sprintf("已创建模型配置：%s。你后续还可以继续补 API Key、URL 或模型名。", provider)
	}
	return fmt.Sprintf("Created model config for %s. You can continue by adding API key, URL, or model name.", provider)
}

func (a *Agent) handleStrategyCreateSkill(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.Name == "" {
		session = skillSession{Name: "strategy_management", Action: "create", Phase: "collecting"}
	}
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前创建策略流程。"
		}
		return "Cancelled the current strategy creation flow."
	}
	name := fieldValue(session, "name")
	if name == "" {
		name = extractTraderName(text)
		if name == "" {
			name = extractPostKeywordName(text, []string{"叫", "名为", "策略叫", "strategy called"})
		}
		if name != "" {
			setField(&session, "name", name)
		}
	}
	if actionRequiresSlot("strategy_management", "create", "name") && name == "" {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建策略，我还需要：" + slotDisplayName("name", lang) + "。你可以直接说：创建一个叫“趋势策略A”的策略。"
		}
		return "To create a strategy, I need a strategy name. You can say: create a strategy called 'Trend A'."
	}
	args := map[string]any{"action": "create", "name": name, "lang": "zh"}
	raw, _ := json.Marshal(args)
	resp := a.toolManageStrategy(storeUserID, string(raw))
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建策略失败：" + errMsg
		}
		return "Failed to create strategy: " + errMsg
	}
	a.clearSkillSession(userID)
	if lang == "zh" {
		return fmt.Sprintf("已创建策略“%s”。默认配置已就绪，你后续可以继续让我帮你改细节。", name)
	}
	return fmt.Sprintf("Created strategy %q with the default configuration.", name)
}

func (a *Agent) handleSimpleEntitySkill(storeUserID string, userID int64, lang, text string, session skillSession, skillName, action string, options []traderSkillOption) (string, bool) {
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前流程。", true
		}
		return "Cancelled the current flow.", true
	}
	if session.Name == "" {
		session = skillSession{Name: skillName, Action: action, Phase: "collecting"}
	}
	if session.Name != skillName || session.Action != action {
		return "", false
	}
	session.TargetRef = resolveTargetFromText(text, options, session.TargetRef)
	if session.TargetRef == nil && action != "query" {
		a.saveSkillSession(userID, session)
		label := formatOptionList("可选对象：", options)
		if lang == "zh" {
			reply := "我还需要你明确要操作的是哪一个对象。"
			if label != "" {
				reply += "\n" + label
			}
			return reply, true
		}
		reply := "I still need you to specify which object to operate on."
		if label != "" {
			reply += "\n" + label
		}
		return reply, true
	}

	switch skillName {
	case "trader_management":
		return a.executeTraderManagementAction(storeUserID, userID, lang, text, session), true
	case "exchange_management":
		return a.executeExchangeManagementAction(storeUserID, userID, lang, text, session), true
	case "model_management":
		return a.executeModelManagementAction(storeUserID, userID, lang, text, session), true
	case "strategy_management":
		return a.executeStrategyManagementAction(storeUserID, userID, lang, text, session), true
	default:
		return "", false
	}
}

func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	return value
}
