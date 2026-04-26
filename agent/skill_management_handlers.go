package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"nofx/store"
)

var urlPattern = regexp.MustCompile(`https://[^\s"'<>]+`)

func hasExplicitCreateIntentForDomain(text, domain string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || !hasExplicitManagementDomainCue(text, domain) {
		return false
	}
	return containsAny(lower, []string{"创建", "新建", "创一个", "创个", "建一个", "create", "new"})
}

func hasExplicitDiagnosisIntentForDomain(text, domain string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || !hasExplicitManagementDomainCue(text, domain) {
		return false
	}
	switch strings.TrimSpace(domain) {
	case "trader":
		return containsAny(lower, []string{"启动失败", "不交易", "没开仓", "无法启动", "诊断", "报错", "错误", "diagnose", "not trading"})
	case "strategy":
		return containsAny(lower, []string{"不生效", "没生效", "失效", "不一致", "诊断", "报错", "错误", "diagnose"})
	case "model":
		return containsAny(lower, []string{"api key", "base url", "custom_api_url", "模型配置失败", "模型不可用", "ai unavailable", "无效", "报错", "错误", "失败", "不可用", "invalid", "error", "failed", "诊断", "diagnose"})
	case "exchange":
		return containsAny(lower, []string{"invalid signature", "timestamp", "ip not allowed", "permission denied", "签名错误", "签名失败", "时间戳", "白名单", "权限不足", "交易所 api 报错", "交易所连接不上", "报错", "错误", "失败", "诊断", "diagnose"})
	default:
		return false
	}
}

func extractURL(text string) string {
	return strings.TrimSpace(urlPattern.FindString(text))
}

func setField(session *skillSession, key, value string) {
	ensureSkillFields(session)
	key = normalizeFieldKey(session, key)
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if session != nil && session.Name == "trader_management" && key == "name" {
		value = normalizeTraderDraftName(value)
		if value == "" {
			return
		}
	}
	session.Fields[key] = value
	syncTraderCreateSlotMirror(session)
}

func fieldValue(session skillSession, key string) string {
	key = normalizeFieldKey(&session, key)
	if session.Fields != nil {
		if value := strings.TrimSpace(session.Fields[key]); value != "" {
			return value
		}
	}
	if session.Name == "trader_management" && session.Slots != nil {
		switch key {
		case "name":
			return strings.TrimSpace(session.Slots.Name)
		case "exchange_id":
			return strings.TrimSpace(session.Slots.ExchangeID)
		case "exchange_name":
			return strings.TrimSpace(session.Slots.ExchangeName)
		case "model_id":
			return strings.TrimSpace(session.Slots.ModelID)
		case "model_name":
			return strings.TrimSpace(session.Slots.ModelName)
		case "strategy_id":
			return strings.TrimSpace(session.Slots.StrategyID)
		case "strategy_name":
			return strings.TrimSpace(session.Slots.StrategyName)
		case "auto_start":
			if session.Slots.AutoStart != nil {
				if *session.Slots.AutoStart {
					return "true"
				}
				return "false"
			}
		}
	}
	return ""
}

func normalizeFieldKey(session *skillSession, key string) string {
	key = strings.TrimSpace(key)
	if session == nil || session.Name != "trader_management" {
		return key
	}
	switch key {
	case "ai_model_id":
		return "model_id"
	default:
		return key
	}
}

func syncTraderCreateSlotMirror(session *skillSession) {
	if session == nil || session.Name != "trader_management" {
		return
	}
	if session.Slots == nil {
		session.Slots = &createTraderSkillSlots{}
	}
	if session.Fields == nil {
		return
	}
	if value := strings.TrimSpace(session.Fields["name"]); value != "" {
		session.Slots.Name = value
	}
	if value := strings.TrimSpace(session.Fields["exchange_id"]); value != "" {
		session.Slots.ExchangeID = value
	}
	if value := strings.TrimSpace(session.Fields["exchange_name"]); value != "" {
		session.Slots.ExchangeName = value
	}
	if value := strings.TrimSpace(session.Fields["model_id"]); value != "" {
		session.Slots.ModelID = value
	}
	if value := strings.TrimSpace(session.Fields["model_name"]); value != "" {
		session.Slots.ModelName = value
	}
	if value := strings.TrimSpace(session.Fields["strategy_id"]); value != "" {
		session.Slots.StrategyID = value
	}
	if value := strings.TrimSpace(session.Fields["strategy_name"]); value != "" {
		session.Slots.StrategyName = value
	}
	if value := strings.TrimSpace(session.Fields["auto_start"]); value != "" {
		b := strings.EqualFold(value, "true")
		session.Slots.AutoStart = &b
	}
}

func textMeansAllTargets(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"全部", "所有", "全都", "全部策略", "所有策略", "全部删除", "全部删掉", "全部删了",
		"全删", "全删了", "都删", "都删了", "全清", "全清掉",
		"all", "all strategies", "every strategy",
	})
}

func supportsBulkTargetSelection(skillName, action string) bool {
	switch skillName {
	case "strategy_management", "trader_management":
		return action == "delete"
	default:
		return false
	}
}

func resolveTargetFromText(text string, options []traderSkillOption, existing *EntityReference) *EntityReference {
	return resolveTargetSelection(text, options, existing).Ref
}

func hasStrictOptionMention(text string, options []traderSkillOption) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, option := range options {
		name := strings.ToLower(strings.TrimSpace(option.Name))
		if name != "" && strings.Contains(lower, name) {
			return true
		}
		id := strings.ToLower(strings.TrimSpace(option.ID))
		if id != "" && strings.Contains(lower, id) {
			return true
		}
	}
	return false
}

func isSimpleEntityMutationAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "update", "update_name", "update_status", "update_endpoint", "update_bindings",
		"configure_strategy", "configure_exchange", "configure_model",
		"update_prompt", "update_config", "activate", "duplicate":
		return true
	default:
		return false
	}
}

func hasExplicitManagementDomainCue(text, domain string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	switch strings.TrimSpace(domain) {
	case "trader":
		return containsAny(lower, []string{"交易员", "trader", "agent"})
	case "exchange":
		return containsAny(lower, []string{"交易所", "exchange", "okx", "binance", "bybit", "gate", "kucoin", "hyperliquid"})
	case "model":
		return containsAny(lower, []string{"模型", "model"})
	case "strategy":
		return containsAny(lower, []string{"策略", "strategy"})
	default:
		return false
	}
}

func ensureLiveTargetReference(session *skillSession, options []traderSkillOption) bool {
	if session == nil || session.TargetRef == nil {
		return true
	}
	var match *traderSkillOption
	if id := strings.TrimSpace(session.TargetRef.ID); id != "" {
		match = findOptionByIDOrName(options, id)
	}
	if match == nil {
		if name := strings.TrimSpace(session.TargetRef.Name); name != "" {
			match = findOptionByIDOrName(options, name)
			if match == nil {
				match = findUniqueContainingOption(options, name)
			}
		}
	}
	if match == nil {
		session.TargetRef = nil
		return false
	}
	session.TargetRef.ID = match.ID
	session.TargetRef.Name = defaultIfEmpty(match.Name, session.TargetRef.Name)
	return true
}

func (a *Agent) buildSimpleEntityConversationResources(storeUserID string, session skillSession, options []traderSkillOption) map[string]any {
	missing := missingFieldKeysForSkillSession(session)
	resources := map[string]any{}
	for _, field := range missing {
		switch strings.TrimSpace(field) {
		case "target_ref":
			if len(options) > 0 {
				resources["targets"] = options
			}
		case "exchange_name", "exchange_id", "exchange":
			resources["exchanges"] = a.loadExchangeOptions(storeUserID)
		case "model_name", "model_id", "ai_model_id", "model":
			resources["models"] = a.loadEnabledModelOptions(storeUserID)
		case "strategy_name", "strategy_id", "strategy":
			resources["strategies"] = a.loadStrategyOptions(storeUserID)
		}
	}
	return resources
}

func (a *Agent) inferredCurrentReferenceForSkill(userID int64, skillName string) *EntityReference {
	refs := a.semanticCurrentReferences(userID)
	if refs == nil {
		return nil
	}
	switch skillName {
	case "trader_management":
		return normalizeEntityReference(refs.Trader)
	case "exchange_management":
		return normalizeEntityReference(refs.Exchange)
	case "model_management":
		return normalizeEntityReference(refs.Model)
	case "strategy_management":
		return normalizeEntityReference(refs.Strategy)
	default:
		return nil
	}
}

func (a *Agent) handleTraderManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	if session.Name != "trader_management" || session.Action == "" {
		return "", false
	}
	action := session.Action
	if action == "query_running" {
		answer := formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
		return applyTraderQueryFilter(lang, answer, a.toolListTraders(storeUserID), "running_only"), true
	}
	if action == "query_detail" {
		if detail, ok := a.describeTrader(storeUserID, lang, session.TargetRef); ok {
			return detail, true
		}
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID)), true
	}
	return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "trader_management", action, a.loadTraderOptions(storeUserID))
}

func (a *Agent) handleExchangeManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	if session.Name != "exchange_management" || session.Action == "" {
		return "", false
	}
	action := session.Action
	options := a.loadExchangeOptions(storeUserID)
	switch action {
	case "query_list":
		return formatReadFastPathResponse(lang, "get_exchange_configs", a.toolGetExchangeConfigs(storeUserID)), true
	case "query_detail":
		if detail, ok := a.describeExchange(storeUserID, lang, session.TargetRef); ok {
			return detail, true
		}
		return formatReadFastPathResponse(lang, "get_exchange_configs", a.toolGetExchangeConfigs(storeUserID)), true
	case "create":
		return a.handleExchangeCreateSkill(storeUserID, userID, lang, text, session), true
	default:
		return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "exchange_management", action, options)
	}
}

func (a *Agent) handleModelManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	if session.Name != "model_management" || session.Action == "" {
		return "", false
	}
	action := session.Action
	options := a.loadEnabledModelOptions(storeUserID)
	switch action {
	case "query_list":
		return formatReadFastPathResponse(lang, "get_model_configs", a.toolGetModelConfigs(storeUserID)), true
	case "query_detail":
		if detail, ok := a.describeModel(storeUserID, lang, session.TargetRef); ok {
			return detail, true
		}
		return formatReadFastPathResponse(lang, "get_model_configs", a.toolGetModelConfigs(storeUserID)), true
	case "create":
		return a.handleModelCreateSkill(storeUserID, userID, lang, text, session), true
	default:
		return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "model_management", action, options)
	}
}

func (a *Agent) handleStrategyManagementSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	if session.Name != "strategy_management" || session.Action == "" {
		return "", false
	}
	action := session.Action
	options := a.loadStrategyOptions(storeUserID)
	switch action {
	case "query_detail":
		if detail, ok := a.describeStrategy(storeUserID, lang, session.TargetRef); ok {
			return detail, true
		}
		return formatReadFastPathResponse(lang, "get_strategies", a.toolGetStrategies(storeUserID)), true
	case "query_list":
		return formatReadFastPathResponse(lang, "get_strategies", a.toolGetStrategies(storeUserID)), true
	case "create":
		return a.handleStrategyCreateSkill(storeUserID, userID, lang, text, session), true
	default:
		return a.handleSimpleEntitySkill(storeUserID, userID, lang, text, session, "strategy_management", action, options)
	}
}

const strategyCreateDraftConfigField = "strategy_create_draft_config"
const strategyCreateConfigPatchField = "config_patch"

func applyStrategyCreateIntentToConfig(cfg *store.StrategyConfig, text, lang string) []string {
	return nil
}

func marshalStrategyCreateDraft(cfg store.StrategyConfig) string {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(raw)
}

func unmarshalStrategyCreateDraft(raw, lang string) store.StrategyConfig {
	cfg := store.GetDefaultStrategyConfig(lang)
	if strings.TrimSpace(raw) == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return store.GetDefaultStrategyConfig(lang)
	}
	return cfg
}

func strategyCreateConfigFromSession(session skillSession, lang string) (store.StrategyConfig, map[string]any, []string, error) {
	cfg := unmarshalStrategyCreateDraft(fieldValue(session, strategyCreateDraftConfigField), lang)
	patchRaw := strings.TrimSpace(fieldValue(session, strategyCreateConfigPatchField))
	var patch map[string]any
	if patchRaw != "" {
		if err := json.Unmarshal([]byte(patchRaw), &patch); err != nil {
			return cfg, nil, nil, fmt.Errorf("策略配置 patch 不是合法 JSON：%w", err)
		}
		merged, err := store.MergeStrategyConfig(cfg, patch)
		if err != nil {
			return cfg, nil, nil, fmt.Errorf("策略配置 patch 无法应用：%w", err)
		}
		cfg = merged
	}
	beforeClamp := cfg
	cfg.ClampLimits()
	return cfg, patch, store.StrategyClampWarnings(beforeClamp, cfg, cfg.Language), nil
}

func strategyCreateConfirmationReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return isYesReply(text) || lower == "确认创建" || lower == "创建吧" || lower == "就按这个创建" || lower == "按这个创建" || lower == "确认应用" || lower == "应用" || lower == "就按这个应用"
}

func formatStrategyCreateDraftSummary(lang, name string, changedFields, warnings []string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if lang == "zh" {
			name = "未命名策略"
		} else {
			name = "unnamed strategy"
		}
	}
	if lang == "zh" {
		lines := []string{
			fmt.Sprintf("我先把策略草稿整理成了“%s”。", name),
		}
		if len(changedFields) > 0 {
			lines = append(lines, "我已经识别到这些配置意图：")
			for _, field := range changedFields {
				lines = append(lines, "- "+field)
			}
		}
		if len(warnings) > 0 {
			lines = append(lines, "其中有些参数超出了当前安全范围，我先拦下来了：")
			for _, warning := range warnings {
				lines = append(lines, "- "+warning)
			}
			lines = append(lines, "你可以继续告诉我其他字段怎么设计；如果接受当前安全范围，也可以直接回复“确认创建”。")
			return strings.Join(lines, "\n")
		}
		lines = append(lines, "你可以继续补充其他字段，比如选币来源、最大持仓、置信度、盈亏比、多周期；如果现在就创建，直接回复“确认创建”。")
		return strings.Join(lines, "\n")
	}

	lines := []string{
		fmt.Sprintf("I turned that into a draft strategy named %q.", name),
	}
	if len(changedFields) > 0 {
		lines = append(lines, "Recognized fields:")
		for _, field := range changedFields {
			lines = append(lines, "- "+field)
		}
	}
	if len(warnings) > 0 {
		lines = append(lines, "Some values exceeded the current safety limits, so I stopped before creating it:")
		for _, warning := range warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "You can keep refining the draft, or reply 'confirm' to create it with the safe adjusted values.")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "You can keep refining the draft, or reply 'confirm' to create it now.")
	return strings.Join(lines, "\n")
}

func createConfirmationReply(text string) bool {
	return strategyCreateConfirmationReply(text)
}

func formatMissingFieldList(lang string, fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	if lang == "zh" {
		return strings.Join(fields, "、")
	}
	return strings.Join(fields, ", ")
}

func availableModelProvidersMessage(lang string) string {
	return modelProviderChoicePrompt(lang)
}

func inferCreateDisplayName(text string) string {
	clean := func(value string) string {
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "“”\"'：: ，,。.;；")
		for _, sep := range []string{"，", ",", "。", "；", ";", "\n"} {
			if idx := strings.Index(value, sep); idx >= 0 {
				value = strings.TrimSpace(value[:idx])
			}
		}
		for _, marker := range []string{" 交易所", " 模型", " 策略", " exchange", " model", " strategy"} {
			if idx := strings.Index(value, marker); idx >= 0 {
				value = strings.TrimSpace(value[:idx])
			}
		}
		for _, suffix := range []string{"的交易员", "的模型", "的策略", "的交易所", "这个交易员", "这个模型", "这个策略", "这个交易所"} {
			if strings.HasSuffix(value, suffix) {
				value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
			}
		}
		return strings.TrimSpace(value)
	}
	if value := extractDelimitedSegmentAfterKeywords(text, []string{"名称叫", "名字叫", "配置名", "叫", "名为", "名称", "名字是", "called"}); value != "" {
		return clean(value)
	}
	if value := extractQuotedContent(text); value != "" && !containsAny(strings.ToLower(text), []string{"api key", "apikey", "api_key", "secret", "passphrase"}) {
		return clean(value)
	}
	return ""
}

func formatModelCreateDraftSummary(lang string, session skillSession) string {
	providerID := fieldValue(session, "provider")
	name := defaultIfEmpty(fieldValue(session, "name"), defaultIfEmpty(defaultModelConfigName(providerID), "未命名模型"))
	provider := defaultIfEmpty(providerID, "未选择")
	modelName := defaultIfEmpty(fieldValue(session, "custom_model_name"), defaultIfEmpty(defaultModelNameForProvider(providerID), "未设置"))
	apiURL := defaultIfEmpty(fieldValue(session, "custom_api_url"), "默认官方地址")
	if lang != "zh" {
		apiURL = defaultIfEmpty(fieldValue(session, "custom_api_url"), "provider default endpoint")
	}
	enabled := fieldValue(session, "enabled") != "false"
	if lang == "zh" {
		lines := []string{
			fmt.Sprintf("我先整理了一份模型配置草稿“%s”。", name),
			fmt.Sprintf("- Provider：%s", provider),
			fmt.Sprintf("- 配置名称：%s", name),
			fmt.Sprintf("- 模型名称：%s", modelName),
			fmt.Sprintf("- 接口地址：%s", apiURL),
			fmt.Sprintf("- 启用状态：%t（未指定时默认 true）", enabled),
			modelProviderDetailedGuidance(lang, providerID),
			"如果这些字段没问题，直接回复“确认创建”；也可以继续补充或修改任意字段。",
		}
		return strings.Join(lines, "\n")
	}
	lines := []string{
		fmt.Sprintf("I prepared a draft model config %q.", name),
		fmt.Sprintf("- Provider: %s", provider),
		fmt.Sprintf("- Config name: %s", name),
		fmt.Sprintf("- Model name: %s", modelName),
		fmt.Sprintf("- API URL: %s", apiURL),
		fmt.Sprintf("- Enabled: %t (defaults to true if omitted)", enabled),
		modelProviderDetailedGuidance(lang, providerID),
		"Reply 'confirm' to create it, or keep refining any field.",
	}
	return strings.Join(lines, "\n")
}

func formatExchangeCreateDraftSummary(lang string, session skillSession) string {
	exType := defaultIfEmpty(fieldValue(session, "exchange_type"), "未选择")
	accountName := defaultIfEmpty(fieldValue(session, "account_name"), "未命名账户")
	enabled := fieldValue(session, "enabled") != "false"
	testnet := fieldValue(session, "testnet") == "true"
	if lang == "zh" {
		lines := []string{
			fmt.Sprintf("我先整理了一份交易所配置草稿“%s”。", accountName),
			fmt.Sprintf("- 交易所：%s", exType),
			fmt.Sprintf("- 账户名：%s", accountName),
			fmt.Sprintf("- 启用状态：%t（未指定时默认 true）", enabled),
			fmt.Sprintf("- 测试网：%t（未指定时默认 false）", testnet),
		}
		switch exType {
		case "binance", "bybit", "gate", "indodax":
			lines = append(lines,
				fmt.Sprintf("- 已提供 API Key：%t", fieldValue(session, "api_key") != ""),
				fmt.Sprintf("- 已提供 Secret：%t", fieldValue(session, "secret_key") != ""),
			)
		case "okx", "bitget", "kucoin":
			lines = append(lines,
				fmt.Sprintf("- 已提供 API Key：%t", fieldValue(session, "api_key") != ""),
				fmt.Sprintf("- 已提供 Secret：%t", fieldValue(session, "secret_key") != ""),
				fmt.Sprintf("- 已提供 Passphrase：%t", fieldValue(session, "passphrase") != ""),
			)
		case "hyperliquid":
			lines = append(lines,
				fmt.Sprintf("- 已提供 API Key：%t", fieldValue(session, "api_key") != ""),
				fmt.Sprintf("- Hyperliquid 钱包地址：%s", defaultIfEmpty(fieldValue(session, "hyperliquid_wallet_addr"), "未设置")),
			)
		case "aster":
			lines = append(lines,
				fmt.Sprintf("- Aster User：%s", defaultIfEmpty(fieldValue(session, "aster_user"), "未设置")),
				fmt.Sprintf("- Aster Signer：%s", defaultIfEmpty(fieldValue(session, "aster_signer"), "未设置")),
				fmt.Sprintf("- 已提供 Aster 私钥：%t", fieldValue(session, "aster_private_key") != ""),
			)
		case "lighter":
			lines = append(lines,
				fmt.Sprintf("- Lighter 钱包地址：%s", defaultIfEmpty(fieldValue(session, "lighter_wallet_addr"), "未设置")),
				fmt.Sprintf("- 已提供 Lighter API Key 私钥：%t", fieldValue(session, "lighter_api_key_private_key") != ""),
			)
			if value := fieldValue(session, "lighter_api_key_index"); value != "" {
				lines = append(lines, fmt.Sprintf("- Lighter API Key Index：%s", value))
			}
		default:
			lines = append(lines,
				fmt.Sprintf("- 已提供 API Key：%t", fieldValue(session, "api_key") != ""),
				fmt.Sprintf("- 已提供 Secret：%t", fieldValue(session, "secret_key") != ""),
			)
		}
		lines = append(lines, "如果这些字段没问题，直接回复“确认创建”；也可以继续补充或修改任意字段。")
		return strings.Join(lines, "\n")
	}
	lines := []string{
		fmt.Sprintf("I prepared a draft exchange config %q.", accountName),
		fmt.Sprintf("- Exchange: %s", exType),
		fmt.Sprintf("- Account name: %s", accountName),
		fmt.Sprintf("- Enabled: %t (defaults to true if omitted)", enabled),
		fmt.Sprintf("- Testnet: %t (defaults to false if omitted)", testnet),
	}
	switch exType {
	case "binance", "bybit", "gate", "indodax":
		lines = append(lines,
			fmt.Sprintf("- API key provided: %t", fieldValue(session, "api_key") != ""),
			fmt.Sprintf("- Secret provided: %t", fieldValue(session, "secret_key") != ""),
		)
	case "okx", "bitget", "kucoin":
		lines = append(lines,
			fmt.Sprintf("- API key provided: %t", fieldValue(session, "api_key") != ""),
			fmt.Sprintf("- Secret provided: %t", fieldValue(session, "secret_key") != ""),
			fmt.Sprintf("- Passphrase provided: %t", fieldValue(session, "passphrase") != ""),
		)
	case "hyperliquid":
		lines = append(lines,
			fmt.Sprintf("- API key provided: %t", fieldValue(session, "api_key") != ""),
			fmt.Sprintf("- Hyperliquid wallet address: %s", defaultIfEmpty(fieldValue(session, "hyperliquid_wallet_addr"), "not set")),
		)
	case "aster":
		lines = append(lines,
			fmt.Sprintf("- Aster user: %s", defaultIfEmpty(fieldValue(session, "aster_user"), "not set")),
			fmt.Sprintf("- Aster signer: %s", defaultIfEmpty(fieldValue(session, "aster_signer"), "not set")),
			fmt.Sprintf("- Aster private key provided: %t", fieldValue(session, "aster_private_key") != ""),
		)
	case "lighter":
		lines = append(lines,
			fmt.Sprintf("- Lighter wallet address: %s", defaultIfEmpty(fieldValue(session, "lighter_wallet_addr"), "not set")),
			fmt.Sprintf("- Lighter API key private key provided: %t", fieldValue(session, "lighter_api_key_private_key") != ""),
		)
		if value := fieldValue(session, "lighter_api_key_index"); value != "" {
			lines = append(lines, fmt.Sprintf("- Lighter API key index: %s", value))
		}
	default:
		lines = append(lines,
			fmt.Sprintf("- API key provided: %t", fieldValue(session, "api_key") != ""),
			fmt.Sprintf("- Secret provided: %t", fieldValue(session, "secret_key") != ""),
		)
	}
	lines = append(lines, "Reply 'confirm' to create it, or keep refining any field.")
	return strings.Join(lines, "\n")
}

func formatTraderCreateDraftSummary(lang string, session skillSession) string {
	args := buildTraderUpdateArgsFromSession(session)
	args, warnings := normalizeTraderArgsToManualLimits(lang, args)
	scanInterval := 3
	if args.ScanIntervalMinutes != nil && *args.ScanIntervalMinutes > 0 {
		scanInterval = *args.ScanIntervalMinutes
	}
	isCrossMargin := true
	if args.IsCrossMargin != nil {
		isCrossMargin = *args.IsCrossMargin
	}
	showInCompetition := true
	if args.ShowInCompetition != nil {
		showInCompetition = *args.ShowInCompetition
	}
	autoStart := fieldValue(session, "auto_start") == "true"
	name := defaultIfEmpty(fieldValue(session, "name"), "未命名交易员")
	if lang != "zh" {
		name = defaultIfEmpty(fieldValue(session, "name"), "unnamed trader")
	}
	if lang == "zh" {
		lines := []string{
			fmt.Sprintf("我先整理了一份交易员草稿“%s”。", name),
			fmt.Sprintf("- 名称：%s", name),
			fmt.Sprintf("- 交易所：%s", traderCreateExchangeNameOrID(session)),
			fmt.Sprintf("- 模型：%s", traderCreateModelNameOrID(session)),
			fmt.Sprintf("- 策略：%s", traderCreateStrategyNameOrID(session)),
			fmt.Sprintf("- 扫描间隔：%d 分钟（未指定时默认 3）", scanInterval),
			"- 初始余额：创建时由系统自动读取绑定交易所账户净值",
			fmt.Sprintf("- 全仓模式：%t（未指定时默认 true）", isCrossMargin),
			fmt.Sprintf("- 竞技场显示：%t（未指定时默认 true）", showInCompetition),
		}
		if autoStart {
			lines = append(lines, "- 创建后立即启动：true")
			if len(warnings) > 0 {
				lines = append(lines, "这些字段里有超出手动面板范围的值，我已经先按风控范围收敛：")
				for _, warning := range warnings {
					lines = append(lines, "- "+warning)
				}
			}
			lines = append(lines, "如果这些字段没问题，直接回复“确认创建并启动”；也可以继续补充或修改任意字段。")
		} else {
			if len(warnings) > 0 {
				lines = append(lines, "这些字段里有超出手动面板范围的值，我已经先按风控范围收敛：")
				for _, warning := range warnings {
					lines = append(lines, "- "+warning)
				}
			}
			lines = append(lines, "如果这些字段没问题，直接回复“确认创建”；也可以继续补充或修改任意字段。")
		}
		return strings.Join(lines, "\n")
	}
	lines := []string{
		fmt.Sprintf("I prepared a draft trader %q.", name),
		fmt.Sprintf("- Name: %s", name),
		fmt.Sprintf("- Exchange: %s", traderCreateExchangeNameOrID(session)),
		fmt.Sprintf("- Model: %s", traderCreateModelNameOrID(session)),
		fmt.Sprintf("- Strategy: %s", traderCreateStrategyNameOrID(session)),
		fmt.Sprintf("- Scan interval: %d minutes (defaults to 3)", scanInterval),
		"- Initial balance: auto-read from the bound exchange account equity at creation time",
		fmt.Sprintf("- Cross margin: %t (defaults to true)", isCrossMargin),
		fmt.Sprintf("- Show in competition: %t (defaults to true)", showInCompetition),
	}
	if autoStart {
		lines = append(lines, "- Start immediately after creation: true")
		if len(warnings) > 0 {
			lines = append(lines, "Some values exceeded the manual editor limits, so I normalized them first:")
			for _, warning := range warnings {
				lines = append(lines, "- "+warning)
			}
		}
		lines = append(lines, "Reply 'confirm' to create and start it, or keep refining any field.")
	} else {
		if len(warnings) > 0 {
			lines = append(lines, "Some values exceeded the manual editor limits, so I normalized them first:")
			for _, warning := range warnings {
				lines = append(lines, "- "+warning)
			}
		}
		lines = append(lines, "Reply 'confirm' to create it, or keep refining any field.")
	}
	return strings.Join(lines, "\n")
}

func (a *Agent) continueStrategyCreateDraft(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if answer, ok := a.answerSkillSessionExplanation(storeUserID, lang, session, text); ok {
		a.saveSkillSession(userID, session)
		return answer
	}
	name := fieldValue(session, "name")
	if actionRequiresSlot("strategy_management", "create", "name") && strings.TrimSpace(name) == "" {
		setSkillDAGStep(&session, "resolve_name")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建策略，我还需要策略名。你可以直接说：创建一个叫“趋势策略A”的策略。"
		}
		return "One more thing: give this strategy a name."
	}

	cfg := unmarshalStrategyCreateDraft(fieldValue(session, strategyCreateDraftConfigField), lang)
	changedFields := applyStrategyCreateIntentToConfig(&cfg, text, lang)
	if fieldValue(session, strategyCreateDraftConfigField) == "" && len(changedFields) == 0 {
		cfg = store.GetDefaultStrategyConfig(lang)
	}
	beforeClamp := cfg
	cfg.ClampLimits()
	warnings := store.StrategyClampWarnings(beforeClamp, cfg, cfg.Language)

	setField(&session, strategyCreateDraftConfigField, marshalStrategyCreateDraft(cfg))
	setSkillDAGStep(&session, "await_create_confirmation")
	session.Phase = "draft_create"

	if strategyCreateConfirmationReply(text) {
		args := map[string]any{
			"action": "create",
			"name":   name,
			"lang":   defaultIfEmpty(lang, "zh"),
		}
		rawCfg, _ := json.Marshal(cfg)
		var configMap map[string]any
		if err := json.Unmarshal(rawCfg, &configMap); err == nil && len(configMap) > 0 {
			args["config"] = configMap
		}
		raw, _ := json.Marshal(args)
		resp := a.toolManageStrategy(storeUserID, string(raw))
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "创建策略失败：" + errMsg
			}
			return "That create request did not go through: " + errMsg
		}
		a.clearSkillSession(userID)
		if lang == "zh" {
			return fmt.Sprintf("已按当前草稿创建策略“%s”。后续如果还想继续细化参数，直接告诉我就行。", name)
		}
		return fmt.Sprintf("Created strategy %q from the current draft.", name)
	}

	a.saveSkillSession(userID, session)
	return formatStrategyCreateDraftSummary(lang, name, changedFields, warnings)
}

func hasExplicitStrategyDetailIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if !hasExplicitManagementDomainCue(text, "strategy") {
		return false
	}
	return containsAny(lower, []string{
		"什么样", "怎么样", "详情", "详细", "prompt", "提示词",
		"哪个策略", "哪一个策略", "你改的是哪个策略", "你把哪个策略",
		"what kind", "details", "detail", "prompt", "which strategy",
	})
}

func shouldPreferStrategyQueryDetail(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if !containsAny(lower, []string{"?", "？", "哪个", "哪一个", "哪条", "which"}) {
		return false
	}
	return containsAny(lower, []string{"策略", "strategy"})
}

func shouldExplainStrategyRuntimeBoundary(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if !containsAny(lower, []string{"策略", "strategy"}) {
		return false
	}
	if !containsAny(lower, []string{"启动", "运行", "run", "start", "deploy"}) {
		return false
	}
	if containsAny(lower, []string{"交易员", "trader", "机器人", "bot"}) {
		return false
	}
	return true
}

func wantsDefaultStrategyConfig(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"默认配置", "默认策略", "默认模板", "模板配置",
		"default config", "default strategy", "default template",
	})
}

func (a *Agent) describeStrategy(storeUserID, lang string, target *EntityReference) (string, bool) {
	if a.store == nil {
		return "", false
	}

	var strategy *store.Strategy
	var err error
	if target != nil && strings.TrimSpace(target.ID) != "" {
		strategy, err = a.store.Strategy().Get(storeUserID, strings.TrimSpace(target.ID))
	} else if target != nil && strings.TrimSpace(target.Name) != "" {
		strategies, listErr := a.store.Strategy().List(storeUserID)
		if listErr != nil {
			return "", false
		}
		for _, item := range strategies {
			if item != nil && strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(target.Name)) {
				strategy = item
				break
			}
		}
	} else {
		strategies, listErr := a.store.Strategy().List(storeUserID)
		if listErr != nil || len(strategies) != 1 {
			return "", false
		}
		strategy = strategies[0]
	}
	if err != nil || strategy == nil {
		return "", false
	}

	var cfg store.StrategyConfig
	if strings.TrimSpace(strategy.Config) != "" {
		_ = json.Unmarshal([]byte(strategy.Config), &cfg)
	}

	return formatStrategyDetailResponse(lang, strategy, cfg), true
}

func formatStrategyDetailResponse(lang string, strategy *store.Strategy, cfg store.StrategyConfig) string {
	name := strings.TrimSpace(strategy.Name)
	if name == "" {
		name = strings.TrimSpace(strategy.ID)
	}

	sourceBits := make([]string, 0, 4)
	if strings.TrimSpace(cfg.CoinSource.SourceType) != "" {
		sourceBits = append(sourceBits, cfg.CoinSource.SourceType)
	}
	if cfg.CoinSource.UseAI500 {
		sourceBits = append(sourceBits, fmt.Sprintf("AI500=%d", cfg.CoinSource.AI500Limit))
	}
	if cfg.CoinSource.UseOITop {
		sourceBits = append(sourceBits, fmt.Sprintf("OITop=%d", cfg.CoinSource.OITopLimit))
	}
	if cfg.CoinSource.UseOILow {
		sourceBits = append(sourceBits, fmt.Sprintf("OILow=%d", cfg.CoinSource.OILowLimit))
	}
	if len(cfg.CoinSource.StaticCoins) > 0 {
		sourceBits = append(sourceBits, "static="+strings.Join(cfg.CoinSource.StaticCoins, ","))
	}
	if len(cfg.CoinSource.ExcludedCoins) > 0 {
		sourceBits = append(sourceBits, "excluded="+strings.Join(cfg.CoinSource.ExcludedCoins, ","))
	}

	timeframes := append([]string(nil), cfg.Indicators.Klines.SelectedTimeframes...)
	if len(timeframes) == 0 {
		timeframes = cleanStringList([]string{cfg.Indicators.Klines.PrimaryTimeframe, cfg.Indicators.Klines.LongerTimeframe})
	}

	indicatorBits := make([]string, 0, 8)
	if cfg.Indicators.EnableRawKlines {
		indicatorBits = append(indicatorBits, "raw_klines")
	}
	if cfg.Indicators.EnableVolume {
		indicatorBits = append(indicatorBits, "volume")
	}
	if cfg.Indicators.EnableOI {
		indicatorBits = append(indicatorBits, "oi")
	}
	if cfg.Indicators.EnableFundingRate {
		indicatorBits = append(indicatorBits, "funding_rate")
	}
	if cfg.Indicators.EnableEMA {
		indicatorBits = append(indicatorBits, "ema")
	}
	if cfg.Indicators.EnableMACD {
		indicatorBits = append(indicatorBits, "macd")
	}
	if cfg.Indicators.EnableRSI {
		indicatorBits = append(indicatorBits, "rsi")
	}
	if cfg.Indicators.EnableATR {
		indicatorBits = append(indicatorBits, "atr")
	}
	if cfg.Indicators.EnableBOLL {
		indicatorBits = append(indicatorBits, "boll")
	}
	sort.Strings(indicatorBits)

	promptBits := make([]string, 0, 5)
	if strings.TrimSpace(cfg.PromptSections.RoleDefinition) != "" {
		promptBits = append(promptBits, "role_definition")
	}
	if strings.TrimSpace(cfg.PromptSections.TradingFrequency) != "" {
		promptBits = append(promptBits, "trading_frequency")
	}
	if strings.TrimSpace(cfg.PromptSections.EntryStandards) != "" {
		promptBits = append(promptBits, "entry_standards")
	}
	if strings.TrimSpace(cfg.PromptSections.DecisionProcess) != "" {
		promptBits = append(promptBits, "decision_process")
	}

	customPrompt := strings.TrimSpace(cfg.CustomPrompt)
	customPromptPreview := customPrompt
	if len([]rune(customPromptPreview)) > 120 {
		runes := []rune(customPromptPreview)
		customPromptPreview = string(runes[:120]) + "..."
	}

	publishStatusZh := "未发布"
	publishStatusEn := "private"
	if strategy.IsPublic {
		publishStatusZh = "已发布到市场"
		publishStatusEn = "public"
	}
	configVisibleZh := "隐藏"
	configVisibleEn := "hidden"
	if strategy.ConfigVisible {
		configVisibleZh = "可见"
		configVisibleEn = "visible"
	}

	if lang == "zh" {
		lines := []string{
			fmt.Sprintf("策略“%s”概览：", name),
			fmt.Sprintf("- 类型：%s", defaultIfEmpty(strings.TrimSpace(cfg.StrategyType), "ai_trading")),
			fmt.Sprintf("- 语言：%s", defaultIfEmpty(strings.TrimSpace(cfg.Language), "zh")),
			fmt.Sprintf("- 发布设置：%s；配置%s", publishStatusZh, configVisibleZh),
		}
		if strings.TrimSpace(strategy.Description) != "" {
			lines = append(lines, fmt.Sprintf("- 描述：%s", strings.TrimSpace(strategy.Description)))
		}
		if cfg.GridConfig != nil {
			lines = append(lines, fmt.Sprintf("- 网格参数：交易对 %s；网格 %d；总投资 %.2f；杠杆 %d；分布 %s",
				defaultIfEmpty(strings.TrimSpace(cfg.GridConfig.Symbol), "未设置"),
				cfg.GridConfig.GridCount,
				cfg.GridConfig.TotalInvestment,
				cfg.GridConfig.Leverage,
				defaultIfEmpty(strings.TrimSpace(cfg.GridConfig.Distribution), "未设置"),
			))
			if cfg.GridConfig.UseATRBounds {
				lines = append(lines, fmt.Sprintf("- 网格边界：ATR 自动边界，倍数 %.2f", cfg.GridConfig.ATRMultiplier))
			} else if cfg.GridConfig.UpperPrice > 0 || cfg.GridConfig.LowerPrice > 0 {
				lines = append(lines, fmt.Sprintf("- 网格边界：上沿 %.4f，下沿 %.4f", cfg.GridConfig.UpperPrice, cfg.GridConfig.LowerPrice))
			}
		}
		if len(sourceBits) > 0 {
			lines = append(lines, "- 标的来源："+strings.Join(sourceBits, " | "))
		}
		if len(timeframes) > 0 {
			lines = append(lines, "- K线周期："+strings.Join(timeframes, " / "))
		}
		lines = append(lines, fmt.Sprintf("- 仓位风险：最多持仓 %d，BTC/ETH 最大杠杆 %d，山寨最大杠杆 %d，最低置信度 %d",
			cfg.RiskControl.MaxPositions, cfg.RiskControl.BTCETHMaxLeverage, cfg.RiskControl.AltcoinMaxLeverage, cfg.RiskControl.MinConfidence))
		lines = append(lines, fmt.Sprintf("- 风控阈值：最小盈亏比 %.2f；最大保证金使用率 %.2f；最小开仓金额 %.2f",
			cfg.RiskControl.MinRiskRewardRatio, cfg.RiskControl.MaxMarginUsage, cfg.RiskControl.MinPositionSize))
		if len(indicatorBits) > 0 {
			lines = append(lines, "- 已启用指标："+strings.Join(indicatorBits, "、"))
		}
		if strings.TrimSpace(cfg.Indicators.NofxOSAPIKey) != "" || cfg.Indicators.EnableQuantData || cfg.Indicators.EnableOIRanking || cfg.Indicators.EnableNetFlowRanking || cfg.Indicators.EnablePriceRanking {
			lines = append(lines, fmt.Sprintf("- NofxOS 数据：API Key=%t，量化数据=%t，OI 排行=%t，净流入排行=%t，价格排行=%t",
				strings.TrimSpace(cfg.Indicators.NofxOSAPIKey) != "",
				cfg.Indicators.EnableQuantData,
				cfg.Indicators.EnableOIRanking,
				cfg.Indicators.EnableNetFlowRanking,
				cfg.Indicators.EnablePriceRanking,
			))
		}
		if len(promptBits) > 0 {
			lines = append(lines, "- Prompt 模块："+strings.Join(promptBits, "、"))
		}
		if customPromptPreview != "" {
			lines = append(lines, "- 自定义 Prompt："+customPromptPreview)
		} else {
			lines = append(lines, "- 自定义 Prompt：当前为空，主要使用策略模板内置 prompt sections。")
		}
		lines = append(lines, "- 如果你要，我还可以继续展开这条策略的完整参数 JSON，或者逐段解释它的 prompt。")
		return strings.Join(lines, "\n")
	}

	lines := []string{
		fmt.Sprintf("Strategy %q overview:", name),
		fmt.Sprintf("- Type: %s", defaultIfEmpty(strings.TrimSpace(cfg.StrategyType), "ai_trading")),
		fmt.Sprintf("- Language: %s", defaultIfEmpty(strings.TrimSpace(cfg.Language), "en")),
		fmt.Sprintf("- Publish settings: %s; config %s", publishStatusEn, configVisibleEn),
	}
	if strings.TrimSpace(strategy.Description) != "" {
		lines = append(lines, fmt.Sprintf("- Description: %s", strings.TrimSpace(strategy.Description)))
	}
	if cfg.GridConfig != nil {
		lines = append(lines, fmt.Sprintf("- Grid config: symbol %s; grids %d; investment %.2f; leverage %d; distribution %s",
			defaultIfEmpty(strings.TrimSpace(cfg.GridConfig.Symbol), "not set"),
			cfg.GridConfig.GridCount,
			cfg.GridConfig.TotalInvestment,
			cfg.GridConfig.Leverage,
			defaultIfEmpty(strings.TrimSpace(cfg.GridConfig.Distribution), "not set"),
		))
		if cfg.GridConfig.UseATRBounds {
			lines = append(lines, fmt.Sprintf("- Grid bounds: ATR auto bounds with multiplier %.2f", cfg.GridConfig.ATRMultiplier))
		} else if cfg.GridConfig.UpperPrice > 0 || cfg.GridConfig.LowerPrice > 0 {
			lines = append(lines, fmt.Sprintf("- Grid bounds: upper %.4f, lower %.4f", cfg.GridConfig.UpperPrice, cfg.GridConfig.LowerPrice))
		}
	}
	if len(sourceBits) > 0 {
		lines = append(lines, "- Coin source: "+strings.Join(sourceBits, " | "))
	}
	if len(timeframes) > 0 {
		lines = append(lines, "- Timeframes: "+strings.Join(timeframes, " / "))
	}
	lines = append(lines, fmt.Sprintf("- Risk: max positions %d, BTC/ETH max leverage %d, alt max leverage %d, min confidence %d",
		cfg.RiskControl.MaxPositions, cfg.RiskControl.BTCETHMaxLeverage, cfg.RiskControl.AltcoinMaxLeverage, cfg.RiskControl.MinConfidence))
	lines = append(lines, fmt.Sprintf("- Risk thresholds: min RR %.2f, max margin usage %.2f, min position size %.2f",
		cfg.RiskControl.MinRiskRewardRatio, cfg.RiskControl.MaxMarginUsage, cfg.RiskControl.MinPositionSize))
	if len(indicatorBits) > 0 {
		lines = append(lines, "- Enabled indicators: "+strings.Join(indicatorBits, ", "))
	}
	if strings.TrimSpace(cfg.Indicators.NofxOSAPIKey) != "" || cfg.Indicators.EnableQuantData || cfg.Indicators.EnableOIRanking || cfg.Indicators.EnableNetFlowRanking || cfg.Indicators.EnablePriceRanking {
		lines = append(lines, fmt.Sprintf("- NofxOS data: API key=%t, quant data=%t, OI ranking=%t, netflow ranking=%t, price ranking=%t",
			strings.TrimSpace(cfg.Indicators.NofxOSAPIKey) != "",
			cfg.Indicators.EnableQuantData,
			cfg.Indicators.EnableOIRanking,
			cfg.Indicators.EnableNetFlowRanking,
			cfg.Indicators.EnablePriceRanking,
		))
	}
	if len(promptBits) > 0 {
		lines = append(lines, "- Prompt modules: "+strings.Join(promptBits, ", "))
	}
	if customPromptPreview != "" {
		lines = append(lines, "- Custom prompt: "+customPromptPreview)
	} else {
		lines = append(lines, "- Custom prompt: empty right now; it mainly uses the built-in prompt sections from the strategy template.")
	}
	lines = append(lines, "- I can also expand the full strategy config JSON or walk through the prompt section by section.")
	return strings.Join(lines, "\n")
}

func (a *Agent) describeDefaultStrategyConfig(lang string) string {
	if lang != "zh" {
		lang = "en"
	}
	cfg := store.GetDefaultStrategyConfig(lang)
	name := "Default Strategy Template"
	description := "System default strategy configuration template"
	if lang == "zh" {
		name = "默认策略模板"
		description = "系统默认策略配置模板"
	}
	return formatStrategyDetailResponse(lang, &store.Strategy{
		ID:          "default_strategy_template",
		Name:        name,
		Description: description,
	}, cfg)
}

func (a *Agent) describeTrader(storeUserID, lang string, target *EntityReference) (string, bool) {
	raw := a.toolListTraders(storeUserID)
	var payload struct {
		Traders []safeTraderToolConfig `json:"traders"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", false
	}
	trader := findTraderByReference(payload.Traders, target)
	if trader == nil {
		if len(payload.Traders) != 1 {
			return "", false
		}
		trader = &payload.Traders[0]
	}
	if lang == "zh" {
		status := "未运行"
		if trader.IsRunning {
			status = "运行中"
		}
		return fmt.Sprintf("交易员“%s”详情：\n- 状态：%s\n- 模型：%s\n- 交易所：%s\n- 策略：%s\n- 扫描间隔：%d 分钟\n- 初始余额：%.2f",
			trader.Name, status, trader.AIModelID, trader.ExchangeID, defaultIfEmpty(trader.StrategyID, "未绑定"), trader.ScanIntervalMinutes, trader.InitialBalance), true
	}
	status := "stopped"
	if trader.IsRunning {
		status = "running"
	}
	return fmt.Sprintf("Trader %q details:\n- Status: %s\n- Model: %s\n- Exchange: %s\n- Strategy: %s\n- Scan interval: %d minutes\n- Initial balance: %.2f",
		trader.Name, status, trader.AIModelID, trader.ExchangeID, defaultIfEmpty(trader.StrategyID, "none"), trader.ScanIntervalMinutes, trader.InitialBalance), true
}

func (a *Agent) describeExchange(storeUserID, lang string, target *EntityReference) (string, bool) {
	raw := a.toolGetExchangeConfigs(storeUserID)
	var payload struct {
		ExchangeConfigs []safeExchangeToolConfig `json:"exchange_configs"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", false
	}
	exchange := findExchangeByReference(payload.ExchangeConfigs, target)
	if exchange == nil {
		if len(payload.ExchangeConfigs) != 1 {
			return "", false
		}
		exchange = &payload.ExchangeConfigs[0]
	}
	name := defaultIfEmpty(exchange.AccountName, exchange.ID)
	credentialLinesZh := make([]string, 0, 8)
	credentialLinesEn := make([]string, 0, 8)
	addCredentialLine := func(labelZh, labelEn string, present bool) {
		credentialLinesZh = append(credentialLinesZh, fmt.Sprintf("- %s：%t", labelZh, present))
		credentialLinesEn = append(credentialLinesEn, fmt.Sprintf("- %s: %t", labelEn, present))
	}
	switch exchange.ExchangeType {
	case "binance", "bybit", "gate", "indodax":
		addCredentialLine("API Key", "API key present", exchange.HasAPIKey)
		addCredentialLine("Secret", "Secret present", exchange.HasSecretKey)
	case "okx", "bitget", "kucoin":
		addCredentialLine("API Key", "API key present", exchange.HasAPIKey)
		addCredentialLine("Secret", "Secret present", exchange.HasSecretKey)
		addCredentialLine("Passphrase", "Passphrase present", exchange.HasPassphrase)
	case "hyperliquid":
		addCredentialLine("API Key", "API key present", exchange.HasAPIKey)
		credentialLinesZh = append(credentialLinesZh, fmt.Sprintf("- Hyperliquid 钱包地址：%s", defaultIfEmpty(exchange.HyperliquidWalletAddr, "未设置")))
		credentialLinesEn = append(credentialLinesEn, fmt.Sprintf("- Hyperliquid wallet address: %s", defaultIfEmpty(exchange.HyperliquidWalletAddr, "not set")))
	case "aster":
		credentialLinesZh = append(credentialLinesZh,
			fmt.Sprintf("- Aster User：%s", defaultIfEmpty(exchange.AsterUser, "未设置")),
			fmt.Sprintf("- Aster Signer：%s", defaultIfEmpty(exchange.AsterSigner, "未设置")),
			fmt.Sprintf("- Aster 私钥：%t", exchange.HasAsterPrivateKey),
		)
		credentialLinesEn = append(credentialLinesEn,
			fmt.Sprintf("- Aster user: %s", defaultIfEmpty(exchange.AsterUser, "not set")),
			fmt.Sprintf("- Aster signer: %s", defaultIfEmpty(exchange.AsterSigner, "not set")),
			fmt.Sprintf("- Aster private key present: %t", exchange.HasAsterPrivateKey),
		)
	case "lighter":
		credentialLinesZh = append(credentialLinesZh,
			fmt.Sprintf("- Lighter 钱包地址：%s", defaultIfEmpty(exchange.LighterWalletAddr, "未设置")),
			fmt.Sprintf("- Lighter API Key 私钥：%t", exchange.HasLighterAPIKey),
			fmt.Sprintf("- Lighter API Key Index：%d", exchange.LighterAPIKeyIndex),
		)
		credentialLinesEn = append(credentialLinesEn,
			fmt.Sprintf("- Lighter wallet address: %s", defaultIfEmpty(exchange.LighterWalletAddr, "not set")),
			fmt.Sprintf("- Lighter API key private key present: %t", exchange.HasLighterAPIKey),
			fmt.Sprintf("- Lighter API key index: %d", exchange.LighterAPIKeyIndex),
		)
	default:
		addCredentialLine("API Key", "API key present", exchange.HasAPIKey)
		addCredentialLine("Secret", "Secret present", exchange.HasSecretKey)
		if exchange.HasPassphrase {
			addCredentialLine("Passphrase", "Passphrase present", true)
		}
	}
	if lang == "zh" {
		lines := []string{
			fmt.Sprintf("交易所配置“%s”详情：", name),
			fmt.Sprintf("- 交易所：%s", exchange.ExchangeType),
			fmt.Sprintf("- 账户名：%s", name),
			fmt.Sprintf("- 已启用：%t", exchange.Enabled),
			fmt.Sprintf("- Testnet：%t", exchange.Testnet),
		}
		lines = append(lines, credentialLinesZh...)
		return strings.Join(lines, "\n"), true
	}
	lines := []string{
		fmt.Sprintf("Exchange config %q details:", name),
		fmt.Sprintf("- Exchange: %s", exchange.ExchangeType),
		fmt.Sprintf("- Account name: %s", name),
		fmt.Sprintf("- Enabled: %t", exchange.Enabled),
		fmt.Sprintf("- Testnet: %t", exchange.Testnet),
	}
	lines = append(lines, credentialLinesEn...)
	return strings.Join(lines, "\n"), true
}

func (a *Agent) describeModel(storeUserID, lang string, target *EntityReference) (string, bool) {
	raw := a.toolGetModelConfigs(storeUserID)
	var payload struct {
		ModelConfigs []safeModelToolConfig `json:"model_configs"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", false
	}
	model := findModelByReference(payload.ModelConfigs, target)
	if model == nil {
		if len(payload.ModelConfigs) != 1 {
			return "", false
		}
		model = &payload.ModelConfigs[0]
	}
	if lang == "zh" {
		return fmt.Sprintf("模型配置“%s”详情：\n- Provider：%s\n- 已启用：%t\n- API Key：%t\n- URL：%s\n- Model Name：%s",
			defaultIfEmpty(model.Name, model.ID), model.Provider, model.Enabled, model.HasAPIKey, defaultIfEmpty(model.CustomAPIURL, "未设置"), defaultIfEmpty(model.CustomModelName, "未设置")), true
	}
	return fmt.Sprintf("Model config %q details:\n- Provider: %s\n- Enabled: %t\n- API key present: %t\n- URL: %s\n- Model name: %s",
		defaultIfEmpty(model.Name, model.ID), model.Provider, model.Enabled, model.HasAPIKey, defaultIfEmpty(model.CustomAPIURL, "not set"), defaultIfEmpty(model.CustomModelName, "not set")), true
}

func findTraderByReference(items []safeTraderToolConfig, target *EntityReference) *safeTraderToolConfig {
	if target == nil {
		return nil
	}
	for i := range items {
		if strings.TrimSpace(target.ID) != "" && items[i].ID == strings.TrimSpace(target.ID) {
			return &items[i]
		}
		if strings.TrimSpace(target.Name) != "" && strings.EqualFold(strings.TrimSpace(items[i].Name), strings.TrimSpace(target.Name)) {
			return &items[i]
		}
	}
	return nil
}

func findExchangeByReference(items []safeExchangeToolConfig, target *EntityReference) *safeExchangeToolConfig {
	if target == nil {
		return nil
	}
	for i := range items {
		name := defaultIfEmpty(items[i].AccountName, items[i].Name)
		if strings.TrimSpace(target.ID) != "" && items[i].ID == strings.TrimSpace(target.ID) {
			return &items[i]
		}
		if strings.TrimSpace(target.Name) != "" && strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(target.Name)) {
			return &items[i]
		}
	}
	return nil
}

func findModelByReference(items []safeModelToolConfig, target *EntityReference) *safeModelToolConfig {
	if target == nil {
		return nil
	}
	for i := range items {
		if strings.TrimSpace(target.ID) != "" && items[i].ID == strings.TrimSpace(target.ID) {
			return &items[i]
		}
		if strings.TrimSpace(target.Name) != "" && strings.EqualFold(strings.TrimSpace(items[i].Name), strings.TrimSpace(target.Name)) {
			return &items[i]
		}
	}
	return nil
}

func (a *Agent) loadTraderOptions(storeUserID string) []traderSkillOption {
	if a.store == nil {
		return nil
	}
	traders, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return nil
	}
	exchangeNames := map[string]string{}
	if exchanges, err := a.store.Exchange().List(storeUserID); err == nil {
		for _, exchange := range exchanges {
			name := strings.TrimSpace(exchange.AccountName)
			if name == "" {
				name = strings.TrimSpace(exchange.ExchangeType)
			}
			if name != "" {
				exchangeNames[exchange.ID] = name
			}
		}
	}
	modelNames := map[string]string{}
	if models, err := a.store.AIModel().List(storeUserID); err == nil {
		for _, model := range models {
			name := strings.TrimSpace(model.Name)
			if name == "" {
				name = strings.TrimSpace(model.CustomModelName)
			}
			if name != "" {
				modelNames[model.ID] = name
			}
		}
	}
	out := make([]traderSkillOption, 0, len(traders))
	for _, trader := range traders {
		hints := make([]string, 0, 2)
		if exchangeName := strings.TrimSpace(exchangeNames[trader.ExchangeID]); exchangeName != "" {
			hints = append(hints, "交易所 "+exchangeName)
		}
		if modelName := strings.TrimSpace(modelNames[trader.AIModelID]); modelName != "" {
			hints = append(hints, "模型 "+modelName)
		}
		out = append(out, traderSkillOption{
			ID:      trader.ID,
			Name:    trader.Name,
			Enabled: trader.IsRunning,
			Hint:    strings.Join(hints, "，"),
		})
	}
	return out
}

func (a *Agent) handleExchangeCreateSkill(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.Name == "" {
		session = skillSession{Name: "exchange_management", Action: "create", Phase: "collecting"}
	}
	if fieldValue(session, skillDAGStepField) == "" {
		setSkillDAGStep(&session, "resolve_exchange_type")
	}
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前创建交易所配置流程。"
		}
		return "Cancelled the current exchange creation flow."
	}
	exType := fieldValue(session, "exchange_type")
	accountName := fieldValue(session, "account_name")
	missing := make([]string, 0, 6)
	if actionRequiresSlot("exchange_management", "create", "exchange_type") && exType == "" {
		missing = append(missing, slotDisplayName("exchange_type", lang))
	}
	if accountName == "" {
		missing = append(missing, displayCatalogFieldName("account_name", lang))
	}
	if fieldValue(session, "api_key") == "" {
		missing = append(missing, displayCatalogFieldName("api_key", lang))
	}
	if fieldValue(session, "secret_key") == "" {
		missing = append(missing, displayCatalogFieldName("secret_key", lang))
	}
	switch exType {
	case "okx":
		if fieldValue(session, "passphrase") == "" {
			missing = append(missing, displayCatalogFieldName("passphrase", lang))
		}
	case "hyperliquid":
		if fieldValue(session, "hyperliquid_wallet_addr") == "" {
			missing = append(missing, "Hyperliquid Wallet")
		}
	}
	if len(missing) > 0 {
		setSkillDAGStep(&session, "resolve_exchange_type")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			reply := "要创建交易所配置，还缺这些字段：" + formatMissingFieldList(lang, missing) + "。"
			if exType == "" {
				reply += "\n例如：OKX、Binance、Bybit。"
			}
			return reply
		}
		return "One more thing: please tell me these details: " + formatMissingFieldList(lang, missing) + "."
	}
	validator := exchangeConfigValidator{
		exchangeType:            exType,
		enabled:                 fieldValue(session, "enabled") == "true",
		apiKey:                  fieldValue(session, "api_key"),
		secretKey:               fieldValue(session, "secret_key"),
		passphrase:              fieldValue(session, "passphrase"),
		hyperliquidWalletAddr:   fieldValue(session, "hyperliquid_wallet_addr"),
		asterUser:               fieldValue(session, "aster_user"),
		asterSigner:             fieldValue(session, "aster_signer"),
		asterPrivateKey:         fieldValue(session, "aster_private_key"),
		lighterWalletAddr:       fieldValue(session, "lighter_wallet_addr"),
		lighterAPIKeyPrivateKey: fieldValue(session, "lighter_api_key_private_key"),
	}
	if err := validator.Validate(); err != nil {
		a.saveSkillSession(userID, session)
		return formatValidationFeedback(lang, "exchange", err)
	}
	if !createConfirmationReply(text) {
		session.Phase = "await_create_confirmation"
		setSkillDAGStep(&session, "await_create_confirmation")
		a.saveSkillSession(userID, session)
		return formatExchangeCreateDraftSummary(lang, session)
	}
	setSkillDAGStep(&session, "execute_create")
	args := map[string]any{
		"action":        "create",
		"exchange_type": exType,
		"account_name":  accountName,
	}
	for _, field := range []string{"api_key", "secret_key", "passphrase", "hyperliquid_wallet_addr", "aster_user", "aster_signer", "aster_private_key", "lighter_wallet_addr", "lighter_api_key_private_key"} {
		if value := fieldValue(session, field); value != "" {
			args[field] = value
		}
	}
	if value := fieldValue(session, "enabled"); value != "" {
		args["enabled"] = value == "true"
	}
	if value := fieldValue(session, "testnet"); value != "" {
		args["testnet"] = value == "true"
	}
	if value := fieldValue(session, "lighter_api_key_index"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			args["lighter_api_key_index"] = parsed
		}
	}
	raw, _ := json.Marshal(args)
	resp := a.toolManageExchangeConfig(storeUserID, string(raw))
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建交易所配置失败：" + errMsg
		}
		return "That create request did not go through: " + errMsg
	}
	a.clearSkillSession(userID)
	a.rememberReferencesFromToolResult(userID, "manage_exchange_config", resp)
	if lang == "zh" {
		return fmt.Sprintf("已创建交易所配置：%s（%s）。", accountName, exType)
	}
	return fmt.Sprintf("Created exchange config %s (%s).", accountName, exType)
}

func (a *Agent) handleModelCreateSkill(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.Name == "" {
		session = skillSession{Name: "model_management", Action: "create", Phase: "collecting"}
	}
	if fieldValue(session, skillDAGStepField) == "" {
		setSkillDAGStep(&session, "resolve_provider")
	}
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前创建模型配置流程。"
		}
		return "Cancelled the current model creation flow."
	}
	provider := fieldValue(session, "provider")
	if provider != "" {
		if fieldValue(session, "name") == "" {
			setField(&session, "name", defaultModelConfigName(provider))
		}
		if modelProviderSupportsCustomModel(provider) && fieldValue(session, "custom_model_name") == "" {
			if defaultModel := defaultModelNameForProvider(provider); defaultModel != "" {
				setField(&session, "custom_model_name", defaultModel)
			}
		}
		if !modelProviderSupportsCustomAPIURL(provider) {
			setField(&session, "custom_api_url", "")
		}
	}
	missing := make([]string, 0, 4)
	providerMissing := actionRequiresSlot("model_management", "create", "provider") && provider == ""
	if providerMissing {
		missing = append(missing, slotDisplayName("provider", lang))
	}
	if !providerMissing && fieldValue(session, "api_key") == "" {
		missing = append(missing, modelProviderCredentialLabel(lang, provider))
	}
	if len(missing) > 0 {
		setSkillDAGStep(&session, "resolve_provider")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			reply := "要创建模型配置，还缺这些字段：" + formatMissingFieldList(lang, missing) + "。"
			if provider == "" {
				reply += "\n" + availableModelProvidersMessage(lang)
			} else {
				reply += "\n" + modelProviderDetailedGuidance(lang, provider)
			}
			return reply
		}
		reply := "One more thing: please tell me these details: " + formatMissingFieldList(lang, missing) + "."
		if provider != "" {
			reply += "\n" + modelProviderDetailedGuidance(lang, provider)
		}
		return reply
	}
	validator := modelConfigValidator{
		provider:        provider,
		enabled:         fieldValue(session, "enabled") == "true",
		apiKey:          fieldValue(session, "api_key"),
		customAPIURL:    fieldValue(session, "custom_api_url"),
		customModelName: fieldValue(session, "custom_model_name"),
	}
	if err := validator.Validate(); err != nil {
		a.saveSkillSession(userID, session)
		return formatValidationFeedback(lang, "model", err)
	}
	if !createConfirmationReply(text) {
		session.Phase = "await_create_confirmation"
		setSkillDAGStep(&session, "await_create_confirmation")
		a.saveSkillSession(userID, session)
		return formatModelCreateDraftSummary(lang, session)
	}
	setSkillDAGStep(&session, "execute_create")
	args := map[string]any{
		"action":            "create",
		"provider":          provider,
		"name":              fieldValue(session, "name"),
		"api_key":           fieldValue(session, "api_key"),
		"custom_api_url":    fieldValue(session, "custom_api_url"),
		"custom_model_name": fieldValue(session, "custom_model_name"),
	}
	if value := fieldValue(session, "enabled"); value != "" {
		args["enabled"] = value == "true"
	}
	raw, _ := json.Marshal(args)
	resp := a.toolManageModelConfig(storeUserID, string(raw))
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建模型配置失败：" + errMsg
		}
		return "That create request did not go through: " + errMsg
	}
	a.clearSkillSession(userID)
	a.rememberReferencesFromToolResult(userID, "manage_model_config", resp)
	if lang == "zh" {
		return fmt.Sprintf("已创建模型配置：%s。", fieldValue(session, "name"))
	}
	return fmt.Sprintf("Created model config %s.", fieldValue(session, "name"))
}

func inferModelCredentialFromText(provider, text string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	text = strings.TrimSpace(text)
	if provider == "" || text == "" {
		return ""
	}

	if value := extractQuotedContent(text); value != "" {
		trimmed := strings.TrimSpace(value)
		if credentialLooksCompatibleWithProvider(provider, trimmed) {
			return trimmed
		}
	}

	if credentialLooksCompatibleWithProvider(provider, text) {
		return text
	}
	return ""
}

func credentialLooksCompatibleWithProvider(provider, value string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	value = strings.TrimSpace(value)
	if provider == "" || value == "" {
		return false
	}

	switch provider {
	case "claw402", "blockrun-base", "blockrun-sol":
		return hexCredentialPattern.MatchString(value)
	case "openai":
		return openAIAPIKeyPattern.MatchString(value)
	default:
		return genericAPIKeyPattern.MatchString(value) || hexCredentialPattern.MatchString(value)
	}
}

func (a *Agent) handleStrategyCreateSkill(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.Name == "" {
		session = skillSession{Name: "strategy_management", Action: "create", Phase: "collecting"}
	}
	if fieldValue(session, skillDAGStepField) == "" {
		setSkillDAGStep(&session, "resolve_name")
	}
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前创建策略流程。"
		}
		return "Cancelled the current strategy creation flow."
	}
	name := fieldValue(session, "name")
	hasDescriptiveDraftIntent := session.Phase == "draft_create"
	if hasDescriptiveDraftIntent {
		return a.continueStrategyCreateDraft(storeUserID, userID, lang, text, session)
	}
	if actionRequiresSlot("strategy_management", "create", "name") && name == "" {
		setSkillDAGStep(&session, "resolve_name")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建策略，我还需要：" + slotDisplayName("name", lang) + "。你可以直接说：创建一个叫“趋势策略A”的策略。"
		}
		return "To create a strategy, I need a strategy name. You can say: create a strategy called 'Trend A'."
	}
	_, patch, warnings, cfgErr := strategyCreateConfigFromSession(session, lang)
	if cfgErr != nil {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建策略失败：" + cfgErr.Error()
		}
		return "That strategy config could not be prepared: " + cfgErr.Error()
	}

	setSkillDAGStep(&session, "execute_create")
	args := map[string]any{
		"action":               "create",
		"name":                 name,
		"lang":                 defaultIfEmpty(lang, "zh"),
		"allow_clamped_update": true,
	}
	if len(patch) > 0 {
		args["config"] = patch
	}
	raw, _ := json.Marshal(args)
	resp := a.toolManageStrategy(storeUserID, string(raw))
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建策略失败：" + errMsg
		}
		return "That create request did not go through: " + errMsg
	}
	a.clearSkillSession(userID)
	a.rememberReferencesFromToolResult(userID, "manage_strategy", resp)
	if lang == "zh" {
		reply := fmt.Sprintf("已创建策略“%s”，并已按你的需求生成配置。", name)
		if len(warnings) > 0 {
			reply += "\n有些值超出安全范围，系统已自动收敛：\n- " + strings.Join(warnings, "\n- ")
		}
		return reply
	}
	reply := fmt.Sprintf("Created strategy %q with a config generated from your requirements.", name)
	if len(warnings) > 0 {
		reply += "\nSome values were clamped to product safety limits:\n- " + strings.Join(warnings, "\n- ")
	}
	return reply
}

func (a *Agent) handleSimpleEntitySkill(storeUserID string, userID int64, lang, text string, session skillSession, skillName, action string, options []traderSkillOption) (string, bool) {
	if session.Name == "" {
		session = skillSession{Name: skillName, Action: action, Phase: "collecting"}
	}
	if session.Name != skillName || session.Action != action {
		return "", false
	}
	if supportsBulkTargetSelection(skillName, action) && textMeansAllTargets(text) {
		setField(&session, "bulk_scope", "all")
		session.TargetRef = nil
	}

	if dag, ok := getSkillDAG(skillName, action); ok && len(dag.Steps) > 0 {
		currentStep, _ := currentSkillDAGStep(session)
		if currentStep.ID == "resolve_target" {
			if resolved := resolveTargetSelection(text, options, session.TargetRef); resolved.Ref != nil {
				session.TargetRef = resolved.Ref
			}
			if session.TargetRef == nil {
				session.TargetRef = a.inferredCurrentReferenceForSkill(userID, skillName)
			}
			if session.TargetRef == nil {
				if !(supportsBulkTargetSelection(skillName, action) && fieldValue(session, "bulk_scope") == "all") {
					setSkillDAGStep(&session, "resolve_target")
					a.saveSkillSession(userID, session)
					label := "可选对象："
					if lang != "zh" {
						label = "Available targets:"
					}
					optionList := formatOptionList(label, options)
					if lang == "zh" {
						reply := "当前这一步需要先确定目标对象。请告诉我你要操作哪一个。"
						if optionList != "" {
							reply += "\n" + optionList
						}
						return reply, true
					}
					reply := "One more thing: tell me which one you want me to work on."
					if optionList != "" {
						reply += "\n" + optionList
					}
					return reply, true
				}
			}
			if fieldValue(session, skillDAGStepField) == currentStep.ID {
				advanceSkillDAGStep(&session, currentStep.ID)
			}
		}
	} else {
		if resolved := resolveTargetSelection(text, options, session.TargetRef); resolved.Ref != nil {
			session.TargetRef = resolved.Ref
		}
		if session.TargetRef == nil {
			session.TargetRef = a.inferredCurrentReferenceForSkill(userID, skillName)
		}
		if session.TargetRef == nil && fieldValue(session, "bulk_scope") != "all" && action != "query" && action != "query_list" && action != "query_detail" && action != "query_running" {
			a.saveSkillSession(userID, session)
			label := formatOptionList("可选对象：", options)
			if lang == "zh" {
				reply := "我还需要你明确要操作的是哪一个对象。"
				if label != "" {
					reply += "\n" + label
				}
				return reply, true
			}
			reply := "One more thing: tell me which one you want to work on."
			if label != "" {
				reply += "\n" + label
			}
			return reply, true
		}
	}

	if session.TargetRef != nil && action != "create" && action != "query_list" && action != "query_running" {
		if !ensureLiveTargetReference(&session, options) {
			a.saveSkillSession(userID, session)
			label := formatOptionList("可选对象：", options)
			if lang == "zh" {
				reply := "我刚检查了一下，刚才记住的对象已经不存在或已失效了。请重新告诉我要操作哪一个对象。"
				if label != "" {
					reply += "\n" + label
				}
				return reply, true
			}
			reply := "The object remembered from earlier no longer exists. Please tell me which object to operate on now."
			if label != "" {
				reply += "\n" + label
			}
			return reply, true
		}
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

func (a *Agent) askLLMAmbiguousTargetQuestion(storeUserID string, userID int64, lang, text string, session skillSession, skillName, action string, allOptions, ambiguous []traderSkillOption) string {
	return formatAmbiguousTargetPrompt(lang, ambiguous)
}

func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	return value
}
