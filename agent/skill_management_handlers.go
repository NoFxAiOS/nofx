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

// strategyCreateDraftConfigField stores the materialized, product-normalized
// draft between turns. User-visible strategy proposals should still be rendered
// from the post-merge structured config, not from free-form LLM text.
const strategyCreateDraftConfigField = "strategy_create_draft_config"
const strategyCreateConfigPatchField = "config_patch"

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
	normalizeLegacyStrategyCreateSession(&session)
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
	applyStrategyCreateTypeDefaults(&cfg)
	beforeClamp := cfg
	cfg.ClampLimits()
	rawCfg, _ := json.Marshal(cfg)
	var configMap map[string]any
	_ = json.Unmarshal(rawCfg, &configMap)
	removeLockedStrategyCreateFields(configMap)
	return cfg, configMap, store.StrategyClampWarnings(beforeClamp, cfg, cfg.Language), nil
}

func resolveStrategyCreateName(session *skillSession, text string) string {
	if session == nil {
		return ""
	}
	name := strings.TrimSpace(fieldValue(*session, "name"))
	if name == "" {
		if inferred := inferStandaloneStrategyName(text); inferred != "" {
			name = inferred
		}
	}
	if name != "" {
		setField(session, "name", name)
	}
	return name
}

func normalizeLegacyStrategyCreateSession(session *skillSession) {
	if session == nil || session.Action != "create" {
		return
	}
	strategyType := explicitStrategyCreateType(*session)
	if strategyType == "" {
		return
	}
	filterLegacyStrategyCreateFieldsForType(session, strategyType)
	if patchRaw := strings.TrimSpace(fieldValue(*session, strategyCreateConfigPatchField)); patchRaw != "" {
		if sanitized := sanitizeStrategyCreateConfigPatchForType(patchRaw, strategyType); len(sanitized) > 0 {
			raw, _ := json.Marshal(sanitized)
			setField(session, strategyCreateConfigPatchField, string(raw))
		} else {
			delete(session.Fields, strategyCreateConfigPatchField)
		}
	}
}

func filterLegacyStrategyCreateFieldsForType(session *skillSession, strategyType string) {
	if session == nil || len(session.Fields) == 0 {
		return
	}
	allowed := map[string]struct{}{}
	for _, key := range []string{
		"name",
		"description",
		"is_public",
		"config_visible",
		"lang",
		"strategy_type",
		strategyCreateDraftConfigField,
		strategyCreateConfigPatchField,
		skillDAGStepField,
		"awaiting_final_confirmation",
	} {
		allowed[key] = struct{}{}
	}
	for key := range session.Fields {
		if _, ok := allowed[key]; !ok {
			delete(session.Fields, key)
		}
	}
}

func resetLegacyStrategyCreateSessionForType(session *skillSession, strategyType string) {
	if session == nil {
		return
	}
	keep := map[string]string{}
	for _, key := range []string{"name", "description", "is_public", "config_visible", "lang"} {
		if value := fieldValue(*session, key); strings.TrimSpace(value) != "" {
			keep[key] = value
		}
	}
	session.Fields = keep
	setField(session, "strategy_type", strategyType)
}

func setStrategyCreateType(session *skillSession, strategyType string) {
	if session == nil || strategyType == "" {
		return
	}
	current := explicitStrategyCreateType(*session)
	if current != "" && current != strategyType {
		resetLegacyStrategyCreateSessionForType(session, strategyType)
		return
	}
	setField(session, "strategy_type", strategyType)
	filterLegacyStrategyCreateFieldsForType(session, strategyType)
}

func applyStrategyCreateTypeDefaults(cfg *store.StrategyConfig) {
	if cfg == nil {
		return
	}
	switch strings.TrimSpace(cfg.StrategyType) {
	case "grid_trading":
		defaultGrid := store.DefaultGridStrategyConfig()
		if cfg.GridConfig == nil {
			cfg.GridConfig = &defaultGrid
			return
		}
		if strings.TrimSpace(cfg.GridConfig.Symbol) == "" {
			cfg.GridConfig.Symbol = defaultGrid.Symbol
		}
		if cfg.GridConfig.GridCount <= 0 {
			cfg.GridConfig.GridCount = defaultGrid.GridCount
		}
		if cfg.GridConfig.TotalInvestment <= 0 {
			cfg.GridConfig.TotalInvestment = defaultGrid.TotalInvestment
		}
		if cfg.GridConfig.Leverage <= 0 {
			cfg.GridConfig.Leverage = defaultGrid.Leverage
		}
		if cfg.GridConfig.ATRMultiplier <= 0 {
			cfg.GridConfig.ATRMultiplier = defaultGrid.ATRMultiplier
		}
		if strings.TrimSpace(cfg.GridConfig.Distribution) == "" {
			cfg.GridConfig.Distribution = defaultGrid.Distribution
		}
		if cfg.GridConfig.MaxDrawdownPct <= 0 {
			cfg.GridConfig.MaxDrawdownPct = defaultGrid.MaxDrawdownPct
		}
		if cfg.GridConfig.StopLossPct <= 0 {
			cfg.GridConfig.StopLossPct = defaultGrid.StopLossPct
		}
		if cfg.GridConfig.DailyLossLimitPct <= 0 {
			cfg.GridConfig.DailyLossLimitPct = defaultGrid.DailyLossLimitPct
		}
		if cfg.GridConfig.DirectionBiasRatio <= 0 {
			cfg.GridConfig.DirectionBiasRatio = defaultGrid.DirectionBiasRatio
		}
		if cfg.GridConfig.UpperPrice <= 0 && cfg.GridConfig.LowerPrice <= 0 {
			cfg.GridConfig.UseATRBounds = true
		}
	case "":
		cfg.StrategyType = "ai_trading"
	}
}

func removeLockedStrategyCreateFields(configMap map[string]any) {
	if configMap == nil {
		return
	}
	risk, ok := configMap["risk_control"].(map[string]any)
	if ok {
		removeLockedAIRiskFields(risk)
	}
	if aiConfig, ok := configMap["ai_config"].(map[string]any); ok {
		if risk, ok := aiConfig["risk_control"].(map[string]any); ok {
			removeLockedAIRiskFields(risk)
		}
	}
}

func removeLockedAIRiskFields(risk map[string]any) {
	delete(risk, "max_positions")
	delete(risk, "btc_eth_max_position_value_ratio")
	delete(risk, "btceth_max_position_value_ratio")
	delete(risk, "altcoin_max_position_value_ratio")
	delete(risk, "max_margin_usage")
	delete(risk, "min_position_size")
}

func strategyCreateConfirmationReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, exact := range []string{
		"确认创建", "确认", "创建吧", "就按这个创建", "按这个创建", "确认应用", "就按这个应用",
		"可以", "好的", "好", "没问题", "就这样", "按这个", "ok", "okay", "yes", "yep", "looks good",
	} {
		if lower == exact {
			return true
		}
	}
	return false
}

func strategyCreateDefaultConfigReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"默认", "先创建", "直接创建", "不用配置", "其他默认", "用默认", "按默认", "默认配置",
		"use default", "use defaults", "default config", "create now", "create directly",
	})
}

func explicitStrategyCreateType(session skillSession) string {
	if value := strings.TrimSpace(fieldValue(session, "strategy_type")); value != "" {
		return value
	}
	patchRaw := strings.TrimSpace(fieldValue(session, strategyCreateConfigPatchField))
	if patchRaw == "" {
		return ""
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(patchRaw), &patch); err != nil {
		return ""
	}
	if value, ok := patch["strategy_type"].(string); ok {
		return strings.TrimSpace(value)
	}
	if gridConfig, ok := patch["grid_config"]; ok && gridConfig != nil {
		return "grid_trading"
	}
	if aiConfig, ok := patch["ai_config"]; ok && aiConfig != nil {
		return "ai_trading"
	}
	return ""
}

func strategyCreateConfigReady(session skillSession, cfg store.StrategyConfig, text string) (bool, string) {
	strategyType := explicitStrategyCreateType(session)
	if strategyType == "" {
		return false, "strategy_type"
	}
	if missing := strategyCreateMissingTemplateFields(session, cfg); len(missing) > 0 {
		return false, strings.Join(missing, ",")
	}
	return true, ""
}

func strategyCreateFinalConfirmationReady(session skillSession) bool {
	return strings.EqualFold(strings.TrimSpace(fieldValue(session, "awaiting_final_confirmation")), "true")
}

func strategyCreateHasExplicitConfigBeyondType(session skillSession) bool {
	for _, key := range manualStrategyEditableFieldKeys() {
		switch key {
		case "name", "description", "is_public", "config_visible", "strategy_type":
			continue
		}
		if strings.TrimSpace(fieldValue(session, key)) != "" {
			return true
		}
	}
	patchRaw := strings.TrimSpace(fieldValue(session, strategyCreateConfigPatchField))
	if patchRaw == "" {
		return false
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(patchRaw), &patch); err != nil {
		return true
	}
	for key := range patch {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(key) != "strategy_type" {
			return true
		}
	}
	return false
}

func strategyCreateMissingTemplateFields(session skillSession, cfg store.StrategyConfig) []string {
	switch explicitStrategyCreateType(session) {
	case "ai_trading":
		return strategyCreateMissingAIFields(session, cfg)
	case "grid_trading":
		return strategyCreateMissingGridFields(session)
	default:
		return []string{"strategy_type"}
	}
}

func strategyCreateMissingAIFields(session skillSession, cfg store.StrategyConfig) []string {
	required := []string{
		"source_type",
		"primary_timeframe",
		"selected_timeframes",
		"btceth_max_leverage",
		"altcoin_max_leverage",
		"min_confidence",
		"min_risk_reward_ratio",
		"trading_frequency",
		"entry_standards",
	}
	missing := make([]string, 0, len(required)+1)
	for _, field := range required {
		if !strategyCreateFieldExplicit(session, field) {
			missing = append(missing, field)
		}
	}
	if strings.EqualFold(strings.TrimSpace(cfg.CoinSource.SourceType), "static") && !strategyCreateFieldExplicit(session, "static_coins") {
		missing = append(missing, "static_coins")
	}
	return missing
}

func strategyCreateMissingGridFields(session skillSession) []string {
	required := []string{
		"symbol",
		"grid_count",
		"total_investment",
		"leverage",
		"distribution",
		"max_drawdown_pct",
		"stop_loss_pct",
		"daily_loss_limit_pct",
		"use_maker_only",
	}
	missing := make([]string, 0, len(required)+1)
	for _, field := range required {
		if !strategyCreateFieldExplicit(session, field) {
			missing = append(missing, field)
		}
	}
	if !strategyCreateFieldExplicit(session, "use_atr_bounds") && (!strategyCreateFieldExplicit(session, "upper_price") || !strategyCreateFieldExplicit(session, "lower_price")) {
		missing = append(missing, "use_atr_bounds 或 upper_price/lower_price")
	}
	return missing
}

func strategyCreateFieldExplicit(session skillSession, field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	if strings.TrimSpace(fieldValue(session, field)) != "" {
		return true
	}
	patchRaw := strings.TrimSpace(fieldValue(session, strategyCreateConfigPatchField))
	if patchRaw == "" {
		return false
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(patchRaw), &patch); err != nil {
		return false
	}
	for _, path := range strategyCreatePatchPaths(field) {
		if strategyCreatePatchHasPath(patch, path...) {
			return true
		}
	}
	return false
}

func strategyCreatePatchPaths(field string) [][]string {
	switch strings.TrimSpace(field) {
	case "strategy_type":
		return [][]string{{"strategy_type"}}
	case "source_type":
		return [][]string{
			{"ai_config", "coin_source", "source_type"}, {"coin_source", "source_type"},
			{"ai_config", "coin_source", "static_coins"}, {"coin_source", "static_coins"},
			{"ai_config", "coin_source", "use_ai500"}, {"coin_source", "use_ai500"},
			{"ai_config", "coin_source", "use_oi_top"}, {"coin_source", "use_oi_top"},
			{"ai_config", "coin_source", "use_oi_low"}, {"coin_source", "use_oi_low"},
		}
	case "static_coins":
		return [][]string{{"ai_config", "coin_source", "static_coins"}, {"coin_source", "static_coins"}}
	case "primary_timeframe":
		return [][]string{{"ai_config", "indicators", "klines", "primary_timeframe"}, {"indicators", "klines", "primary_timeframe"}}
	case "selected_timeframes":
		return [][]string{{"ai_config", "indicators", "klines", "selected_timeframes"}, {"indicators", "klines", "selected_timeframes"}}
	case "btceth_max_leverage":
		return [][]string{{"ai_config", "risk_control", "btc_eth_max_leverage"}, {"risk_control", "btc_eth_max_leverage"}, {"ai_config", "risk_control", "btceth_max_leverage"}, {"risk_control", "btceth_max_leverage"}}
	case "altcoin_max_leverage":
		return [][]string{{"ai_config", "risk_control", "altcoin_max_leverage"}, {"risk_control", "altcoin_max_leverage"}}
	case "min_confidence":
		return [][]string{{"ai_config", "risk_control", "min_confidence"}, {"risk_control", "min_confidence"}}
	case "min_risk_reward_ratio":
		return [][]string{{"ai_config", "risk_control", "min_risk_reward_ratio"}, {"risk_control", "min_risk_reward_ratio"}}
	case "trading_frequency":
		return [][]string{{"ai_config", "prompt_sections", "trading_frequency"}, {"prompt_sections", "trading_frequency"}}
	case "entry_standards":
		return [][]string{{"ai_config", "prompt_sections", "entry_standards"}, {"prompt_sections", "entry_standards"}}
	case "symbol", "grid_count", "total_investment", "leverage", "distribution", "max_drawdown_pct", "stop_loss_pct", "daily_loss_limit_pct", "use_maker_only", "use_atr_bounds", "upper_price", "lower_price":
		return [][]string{{"grid_config", field}}
	default:
		return [][]string{{field}}
	}
}

func strategyCreatePatchHasPath(value any, path ...string) bool {
	current := value
	for _, part := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return false
		}
		next, ok := obj[part]
		if !ok {
			return false
		}
		current = next
	}
	return true
}

func formatStrategyCreateConfigNeeded(lang, missingKind string) string {
	if lang == "zh" {
		if missingKind == "strategy_type" {
			return "先选择策略类型：grid_trading（网格策略）或 ai_trading（AI 策略）。类型确认后我会继续收集对应配置，配置好后再创建。"
		}
		if hints := formatStrategyMissingFieldHints(lang, missingKind); hints != "" {
			return "这份策略模板还没填完整，还缺这些字段。你可以按下面选，也可以直接说“你帮我按稳健/高频/激进来推荐”：\n" + hints
		}
		return "这份策略模板还没填完整，还缺：" + formatStrategyMissingFieldNames(lang, missingKind) + "。你可以一句话告诉我这些字段，我会继续填模板。"
	}
	if missingKind == "strategy_type" {
		return "Choose the strategy type first: grid_trading or ai_trading. I will collect the matching config before creating it."
	}
	if hints := formatStrategyMissingFieldHints(lang, missingKind); hints != "" {
		return "This strategy template is not complete yet. You can choose from these options, or ask me to recommend a conservative/balanced/high-frequency setup:\n" + hints
	}
	return "This strategy template is not complete yet. Missing: " + formatStrategyMissingFieldNames(lang, missingKind) + ". Tell me these fields in one message and I will keep filling the template."
}

func formatStrategyMissingFieldHints(lang, missingKind string) string {
	parts := strings.Split(missingKind, ",")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		field := strings.TrimSpace(part)
		if field == "" {
			continue
		}
		hint := strategyCreateFieldInlineHint(lang, field)
		if hint == "" {
			hint = strategyCreateFieldDisplayName(lang, field)
		}
		if lang == "zh" {
			lines = append(lines, "- "+hint)
		} else {
			lines = append(lines, "- "+hint)
		}
	}
	return strings.Join(lines, "\n")
}

func strategyCreateFieldInlineHint(lang, field string) string {
	field = strings.TrimSpace(field)
	if lang != "zh" {
		switch field {
		case "source_type":
			return "Coin source: ai500 / oi_top / oi_low / static"
		case "static_coins":
			return "Static coins: up to 10 symbols, e.g. BTCUSDT, ETHUSDT"
		case "primary_timeframe":
			return "Primary timeframe: 1m / 3m / 5m / 15m / 30m / 1h / 2h / 4h / 6h / 8h / 12h / 1d / 3d / 1w"
		case "selected_timeframes":
			return "Multi-timeframes: up to 4, e.g. 5m,15m,1h"
		case "btceth_max_leverage", "altcoin_max_leverage":
			return strategyCreateFieldDisplayName(lang, field) + ": 1-20"
		case "min_confidence":
			return "Minimum confidence: 50-100"
		case "min_risk_reward_ratio":
			return "Minimum risk/reward ratio: 1-10, step 0.5"
		case "trading_frequency":
			return "Trading frequency rule: free text, e.g. max 2-4 trades per day"
		case "entry_standards":
			return "Entry standards: free text, e.g. enter only when trend and risk/reward align"
		case "symbol":
			return "Symbol: BTCUSDT / ETHUSDT / SOLUSDT / BNBUSDT / XRPUSDT / DOGEUSDT"
		case "grid_count":
			return "Grid count: 5-50"
		case "total_investment":
			return "Total investment: user's capital/margin budget, minimum 100 USDT; not leveraged notional exposure"
		case "leverage":
			return "Grid leverage: 1-5"
		case "distribution":
			return "Distribution: uniform / gaussian / pyramid"
		case "max_drawdown_pct":
			return "Max drawdown: 5%-50%"
		case "stop_loss_pct":
			return "Stop loss: 1%-20%"
		case "daily_loss_limit_pct":
			return "Daily loss limit: 1%-30%"
		case "use_maker_only":
			return "Maker only: on / off"
		}
		return ""
	}
	switch field {
	case "source_type":
		return "选币来源：AI500 / OI Top / OI Low / 静态币种（没有混合模式）"
	case "static_coins":
		return "静态币种：最多 10 个，例如 BTCUSDT、ETHUSDT"
	case "primary_timeframe":
		return "主周期：1m / 3m / 5m / 15m / 30m / 1h / 2h / 4h / 6h / 8h / 12h / 1d / 3d / 1w"
	case "selected_timeframes":
		return "多周期时间框架：最多 4 个，例如 5m,15m,1h"
	case "btceth_max_leverage":
		return "BTC/ETH 最大杠杆：1～20 倍"
	case "altcoin_max_leverage":
		return "山寨币最大杠杆：1～20 倍"
	case "min_confidence":
		return "最低置信度：50～100，越高越谨慎"
	case "min_risk_reward_ratio":
		return "最小盈亏比：1～10，步进 0.5"
	case "trading_frequency":
		return "交易频率规则：文本，例如“每天最多 2～4 笔，避免连续追单”"
	case "entry_standards":
		return "开仓标准：文本，例如“趋势明确、成交量配合、风险收益合理才开仓”"
	case "symbol":
		return "交易对：BTCUSDT / ETHUSDT / SOLUSDT / BNBUSDT / XRPUSDT / DOGEUSDT"
	case "grid_count":
		return "网格数量：5～50"
	case "total_investment":
		return "总投入：用户实际投入/保证金预算，最低 100 USDT；不是杠杆后的名义仓位"
	case "leverage":
		return "杠杆：1～5 倍"
	case "distribution":
		return "网格分布：uniform（均匀）/ gaussian（正态）/ pyramid（金字塔）"
	case "max_drawdown_pct":
		return "最大回撤：5%～50%"
	case "stop_loss_pct":
		return "止损：1%～20%"
	case "daily_loss_limit_pct":
		return "日亏损限制：1%～30%"
	case "use_maker_only":
		return "只挂 Maker：开启 / 关闭"
	case "use_atr_bounds 或 upper_price/lower_price":
		return "价格边界：开启 ATR 自动边界，或手动填写上边界/下边界"
	}
	return ""
}

func formatStrategyCreateFieldOptionsReply(lang, text, missingKind string) string {
	if !strategyCreateAsksFieldOptions(text) {
		return ""
	}
	field := firstStrategyMissingField(missingKind)
	if field == "" {
		return ""
	}
	if lang != "zh" {
		switch field {
		case "source_type":
			return "Coin source options: ai500, oi_top, oi_low, or static. Pick one and I will continue filling the AI strategy template."
		case "primary_timeframe", "selected_timeframes":
			return "Timeframe options: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w."
		}
		return "For " + strategyCreateFieldDisplayName(lang, field) + ", tell me the value you want and I will keep filling the selected strategy template."
	}
	switch field {
	case "strategy_type":
		return "策略类型只有两个：\n- AI 策略：让 AI 根据行情和策略规则判断开平仓。\n- 网格策略：在价格区间内按网格低买高卖。\n你直接回复“AI 策略”或“网格策略”就行。"
	case "source_type":
		return "AI 策略的选币来源有 4 个：\n- AI500：从 NOFX AI500 榜单自动选币。\n- OI Top：选持仓量靠前/更活跃的币。\n- OI Low：选持仓量较低或变化较弱的币。\n- 静态币种：你指定固定币种，比如 BTCUSDT、ETHUSDT。\n没有混合模式。你选一个，我继续填模板。"
	case "primary_timeframe":
		return "主周期可选：1m、3m、5m、15m、30m、1h、2h、4h、6h、8h、12h、1d、3d、1w。高频一般偏 1m/3m/5m，稳健一点可以用 15m/1h。"
	case "selected_timeframes":
		return "多周期最多选 4 个，可选：1m、3m、5m、15m、30m、1h、2h、4h、6h、8h、12h、1d、3d、1w。常见组合比如 5m,15m,1h。"
	case "btceth_max_leverage", "altcoin_max_leverage":
		return strategyCreateFieldDisplayName(lang, field) + "范围是 1～20 倍。数值越高风险越大。"
	case "min_confidence":
		return "最低置信度范围是 50～100。数值越高越谨慎，开单会更少。"
	case "min_risk_reward_ratio":
		return "最小盈亏比范围是 1～10，步进 0.5。比如 1.5 表示预期收益至少是风险的 1.5 倍。"
	case "trading_frequency":
		return "交易频率规则是文本规则，例如“每天最多 2～4 笔，避免连续追单”。你也可以说“你帮我按高频但不过度交易来写”。"
	case "entry_standards":
		return "开仓标准是文本规则，例如“只在趋势明确、成交量配合、风险收益合理时开仓”。你也可以说“你帮我写一版稳健开仓标准”。"
	case "symbol":
		return "网格交易对可选：BTCUSDT、ETHUSDT、SOLUSDT、BNBUSDT、XRPUSDT、DOGEUSDT。"
	case "grid_count":
		return "网格数量范围是 5～50。数量越多越密，交易更频繁；数量越少，每格空间更大。"
	case "total_investment":
		return "网格总投入是用户实际投入/保证金预算，不是杠杆后的名义仓位；最小 100 USDT，按 100 USDT 步进。"
	case "leverage":
		return "网格杠杆范围是 1～5 倍。稳健一般用 1 倍。"
	case "distribution":
		return "网格分布可选：uniform（均匀）、gaussian（正态）、pyramid（金字塔）。"
	case "max_drawdown_pct":
		return "最大回撤范围是 5%～50%。"
	case "stop_loss_pct":
		return "止损范围是 1%～20%。"
	case "daily_loss_limit_pct":
		return "日亏损限制范围是 1%～30%。"
	case "use_maker_only":
		return "只挂 Maker 是开关项：开启会更偏向低手续费挂单，成交可能慢一些；关闭则更灵活。"
	}
	return strategyCreateFieldDisplayName(lang, field) + "是当前模板字段。你告诉我想怎么设置，我继续填模板。"
}

func strategyCreateAsksFieldOptions(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"有哪些", "有什么", "可选", "选项", "怎么选", "怎么填", "不知道", "不会填",
		"what options", "which options", "options", "how to choose", "how should i fill",
	})
}

func firstStrategyMissingField(missingKind string) string {
	for _, part := range strings.Split(missingKind, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			return part
		}
	}
	return ""
}

func formatStrategyMissingFieldNames(lang, missingKind string) string {
	parts := strings.Split(missingKind, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "或") || strings.Contains(part, "/") {
			names = append(names, part)
			continue
		}
		names = append(names, strategyCreateFieldDisplayName(lang, part))
	}
	if lang == "zh" {
		return strings.Join(names, "、")
	}
	return strings.Join(names, ", ")
}

func strategyCreateFieldDisplayName(lang, field string) string {
	if lang != "zh" {
		return field
	}
	switch strings.TrimSpace(field) {
	case "source_type":
		return "选币来源"
	case "static_coins":
		return "静态币种"
	case "primary_timeframe":
		return "主周期"
	case "selected_timeframes":
		return "多周期时间框架"
	case "btceth_max_leverage":
		return "BTC/ETH 最大杠杆"
	case "altcoin_max_leverage":
		return "山寨币最大杠杆"
	case "min_confidence":
		return "最低置信度"
	case "min_risk_reward_ratio":
		return "最小盈亏比"
	case "trading_frequency":
		return "交易频率规则"
	case "entry_standards":
		return "开仓标准"
	case "symbol":
		return "交易对"
	case "grid_count":
		return "网格数量"
	case "total_investment":
		return "总投入"
	case "leverage":
		return "杠杆"
	case "distribution":
		return "网格分布"
	case "max_drawdown_pct":
		return "最大回撤"
	case "stop_loss_pct":
		return "止损"
	case "daily_loss_limit_pct":
		return "日亏损限制"
	case "use_maker_only":
		return "只挂 Maker"
	default:
		return field
	}
}

func formatStrategyCreateDraftSummary(lang, name, strategyType string, changedFields, warnings []string) string {
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
		switch strategyType {
		case "grid_trading":
			lines = append(lines, "这是网格策略草稿。请继续补充交易对、网格数量、总投入、杠杆、价格区间和网格风控；我只会按产品编辑页模板填你明确给出或明确委托我设计的字段。")
		case "ai_trading":
			lines = append(lines, "这是 AI 策略草稿。请继续补充选币来源、时间周期、风险参数和提示词方向；我只会按产品编辑页模板填你明确给出或明确委托我设计的字段。")
		default:
			lines = append(lines, "你可以继续补充策略类型和对应参数；如果现在就创建，直接回复“确认创建”。")
		}
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
	switch strategyType {
	case "grid_trading":
		lines = append(lines, "This is a grid strategy draft. Keep refining symbol, grid count, total investment, leverage, price bounds, and grid risk settings; I will only fill fields you explicitly provide or ask me to design.")
	case "ai_trading":
		lines = append(lines, "This is an AI strategy draft. Keep refining coin source, timeframes, risk settings, and prompt direction; I will only fill fields you explicitly provide or ask me to design.")
	default:
		lines = append(lines, "You can keep refining the strategy type and matching parameters, or reply 'confirm' to create it now.")
	}
	return strings.Join(lines, "\n")
}

func formatStrategyCreateFinalConfirmation(lang string, session skillSession, cfg store.StrategyConfig) string {
	name := defaultIfEmpty(fieldValue(session, "name"), "未命名策略")
	if lang != "zh" {
		name = defaultIfEmpty(fieldValue(session, "name"), "unnamed strategy")
	}
	if lang == "zh" {
		lines := []string{fmt.Sprintf("我已经把“%s”的配置整理好了，确认后我再创建到策略列表。", name)}
		switch cfg.StrategyType {
		case "grid_trading":
			grid := cfg.GridConfig
			if grid == nil {
				grid = &store.GridStrategyConfig{}
			}
			lines = append(lines,
				"- 类型：网格策略",
				fmt.Sprintf("- 发布到策略市场：%t", fieldValue(session, "is_public") == "true"),
				fmt.Sprintf("- 发布后配置可见：%t", fieldValue(session, "config_visible") != "false"),
				fmt.Sprintf("- 交易对：%s", defaultIfEmpty(grid.Symbol, "未设置")),
				fmt.Sprintf("- 网格数量：%d", grid.GridCount),
				fmt.Sprintf("- 总投入：%.2f USDT", grid.TotalInvestment),
				fmt.Sprintf("- 杠杆：%d倍", grid.Leverage),
			)
			if grid.UseATRBounds {
				lines = append(lines, fmt.Sprintf("- 价格区间：ATR 动态范围（倍数 %.2f）", grid.ATRMultiplier))
			} else {
				lines = append(lines, fmt.Sprintf("- 价格区间：%.2f ～ %.2f", grid.LowerPrice, grid.UpperPrice))
			}
			lines = append(lines,
				fmt.Sprintf("- 网格分布：%s", defaultIfEmpty(grid.Distribution, "uniform")),
				fmt.Sprintf("- 最大回撤：%.2f%%", grid.MaxDrawdownPct),
				fmt.Sprintf("- 止损：%.2f%%", grid.StopLossPct),
				fmt.Sprintf("- 日亏损限制：%.2f%%", grid.DailyLossLimitPct),
			)
		default:
			lines = append(lines,
				"- 类型：AI 策略",
				fmt.Sprintf("- 发布到策略市场：%t", fieldValue(session, "is_public") == "true"),
				fmt.Sprintf("- 发布后配置可见：%t", fieldValue(session, "config_visible") != "false"),
				fmt.Sprintf("- 选币来源：%s", defaultIfEmpty(cfg.CoinSource.SourceType, "未设置")),
			)
			lines = append(lines, formatAICoinSourceSummaryZH(cfg)...)
			lines = append(lines,
				fmt.Sprintf("- 主周期：%s", defaultIfEmpty(cfg.Indicators.Klines.PrimaryTimeframe, "未设置")),
				fmt.Sprintf("- K线数量：%d", cfg.Indicators.Klines.PrimaryCount),
				fmt.Sprintf("- 多周期：%s", defaultIfEmpty(strings.Join(cfg.Indicators.Klines.SelectedTimeframes, ","), "未设置")),
				fmt.Sprintf("- 指标：%s", formatEnabledAIIndicatorsZH(cfg)),
				fmt.Sprintf("- NofxOS 量化数据：%t", cfg.Indicators.EnableQuantData),
				fmt.Sprintf("- OI 排行数据：%t（%s / %d）", cfg.Indicators.EnableOIRanking, defaultIfEmpty(cfg.Indicators.OIRankingDuration, "未设置"), cfg.Indicators.OIRankingLimit),
				fmt.Sprintf("- 资金流排行数据：%t（%s / %d）", cfg.Indicators.EnableNetFlowRanking, defaultIfEmpty(cfg.Indicators.NetFlowRankingDuration, "未设置"), cfg.Indicators.NetFlowRankingLimit),
				fmt.Sprintf("- 涨跌幅排行数据：%t（%s / %d）", cfg.Indicators.EnablePriceRanking, defaultIfEmpty(cfg.Indicators.PriceRankingDuration, "未设置"), cfg.Indicators.PriceRankingLimit),
				fmt.Sprintf("- BTC/ETH 最大杠杆：%d倍", cfg.RiskControl.BTCETHMaxLeverage),
				fmt.Sprintf("- 山寨币最大杠杆：%d倍", cfg.RiskControl.AltcoinMaxLeverage),
				fmt.Sprintf("- 最小置信度：%d", cfg.RiskControl.MinConfidence),
				fmt.Sprintf("- 最小盈亏比：%.2f", cfg.RiskControl.MinRiskRewardRatio),
				fmt.Sprintf("- 最大持仓数（System enforced）：%d", cfg.RiskControl.MaxPositions),
				fmt.Sprintf("- BTC/ETH 单币仓位上限（System enforced）：账户权益 %.2f 倍", cfg.RiskControl.BTCETHMaxPositionValueRatio),
				fmt.Sprintf("- 山寨币单币仓位上限（System enforced）：账户权益 %.2f 倍", cfg.RiskControl.AltcoinMaxPositionValueRatio),
				fmt.Sprintf("- 最大保证金使用率（System enforced）：%.0f%%", cfg.RiskControl.MaxMarginUsage*100),
				fmt.Sprintf("- 最小开仓金额（System enforced）：%.2f USDT", cfg.RiskControl.MinPositionSize),
				fmt.Sprintf("- 角色定义：%s", compactSummaryText(cfg.PromptSections.RoleDefinition)),
				fmt.Sprintf("- 交易频率规则：%s", compactSummaryText(cfg.PromptSections.TradingFrequency)),
				fmt.Sprintf("- 开仓标准：%s", compactSummaryText(cfg.PromptSections.EntryStandards)),
				fmt.Sprintf("- 决策流程：%s", compactSummaryText(cfg.PromptSections.DecisionProcess)),
				fmt.Sprintf("- 自定义 Prompt：%s", compactSummaryText(cfg.CustomPrompt)),
			)
		}
		lines = append(lines, "确认创建的话，直接回复“确认创建”。要调整也可以直接说改哪项。")
		return strings.Join(lines, "\n")
	}
	lines := []string{fmt.Sprintf("I prepared the config for %q. Confirm and I will create it in the strategy list.", name)}
	if cfg.StrategyType == "grid_trading" && cfg.GridConfig != nil {
		grid := cfg.GridConfig
		lines = append(lines,
			"- Type: grid strategy",
			fmt.Sprintf("- Symbol: %s", defaultIfEmpty(grid.Symbol, "unset")),
			fmt.Sprintf("- Grid count: %d", grid.GridCount),
			fmt.Sprintf("- Total investment: %.2f USDT", grid.TotalInvestment),
			fmt.Sprintf("- Leverage: %dx", grid.Leverage),
		)
	} else {
		lines = append(lines, "- Type: AI strategy")
	}
	lines = append(lines, "Reply 'confirm create' to create it, or tell me what to change.")
	return strings.Join(lines, "\n")
}

func formatEnabledAIIndicatorsZH(cfg store.StrategyConfig) string {
	enabled := make([]string, 0, 8)
	if cfg.Indicators.EnableRawKlines {
		enabled = append(enabled, "K线")
	}
	if cfg.Indicators.EnableVolume {
		enabled = append(enabled, "成交量")
	}
	if cfg.Indicators.EnableOI {
		enabled = append(enabled, "OI")
	}
	if cfg.Indicators.EnableFundingRate {
		enabled = append(enabled, "资金费率")
	}
	if cfg.Indicators.EnableEMA {
		enabled = append(enabled, "EMA")
	}
	if cfg.Indicators.EnableMACD {
		enabled = append(enabled, "MACD")
	}
	if cfg.Indicators.EnableRSI {
		enabled = append(enabled, "RSI")
	}
	if cfg.Indicators.EnableATR {
		enabled = append(enabled, "ATR")
	}
	if cfg.Indicators.EnableBOLL {
		enabled = append(enabled, "BOLL")
	}
	if len(enabled) == 0 {
		return "无"
	}
	return strings.Join(enabled, ",")
}

func formatAICoinSourceSummaryZH(cfg store.StrategyConfig) []string {
	lines := make([]string, 0, 4)
	sourceType := strings.ToLower(strings.TrimSpace(cfg.CoinSource.SourceType))
	switch sourceType {
	case "static":
		lines = append(lines, fmt.Sprintf("- 静态币种：%s", defaultIfEmpty(strings.Join(cfg.CoinSource.StaticCoins, ","), "未设置")))
	case "ai500":
		lines = append(lines, fmt.Sprintf("- AI500 数量：%d", cfg.CoinSource.AI500Limit))
	case "oi_top":
		lines = append(lines, fmt.Sprintf("- OI Top 数量：%d", cfg.CoinSource.OITopLimit))
	case "oi_low":
		lines = append(lines, fmt.Sprintf("- OI Low 数量：%d", cfg.CoinSource.OILowLimit))
	default:
		if cfg.CoinSource.UseAI500 {
			lines = append(lines, fmt.Sprintf("- AI500 数量：%d", cfg.CoinSource.AI500Limit))
		}
		if cfg.CoinSource.UseOITop {
			lines = append(lines, fmt.Sprintf("- OI Top 数量：%d", cfg.CoinSource.OITopLimit))
		}
		if cfg.CoinSource.UseOILow {
			lines = append(lines, fmt.Sprintf("- OI Low 数量：%d", cfg.CoinSource.OILowLimit))
		}
	}
	if len(cfg.CoinSource.ExcludedCoins) > 0 {
		lines = append(lines, fmt.Sprintf("- 排除币种：%s", strings.Join(cfg.CoinSource.ExcludedCoins, ",")))
	}
	return lines
}

func compactSummaryText(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		return "未设置"
	}
	const maxLen = 120
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen]) + "..."
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
		lines := []string{
			fmt.Sprintf("模型配置“%s”详情：", defaultIfEmpty(model.Name, model.ID)),
			fmt.Sprintf("- Provider：%s", model.Provider),
			fmt.Sprintf("- 已启用：%t", model.Enabled),
			fmt.Sprintf("- API Key：%t", model.HasAPIKey),
			fmt.Sprintf("- URL：%s", defaultIfEmpty(model.CustomAPIURL, "未设置")),
			fmt.Sprintf("- Model Name：%s", defaultIfEmpty(model.CustomModelName, "未设置")),
		}
		if strings.TrimSpace(model.WalletAddress) != "" {
			lines = append(lines, fmt.Sprintf("- 钱包地址：%s", model.WalletAddress))
		}
		if strings.TrimSpace(model.BalanceUSDC) != "" {
			lines = append(lines, fmt.Sprintf("- 钱包余额：%s USDC", model.BalanceUSDC))
		}
		return strings.Join(lines, "\n"), true
	}
	lines := []string{
		fmt.Sprintf("Model config %q details:", defaultIfEmpty(model.Name, model.ID)),
		fmt.Sprintf("- Provider: %s", model.Provider),
		fmt.Sprintf("- Enabled: %t", model.Enabled),
		fmt.Sprintf("- API key present: %t", model.HasAPIKey),
		fmt.Sprintf("- URL: %s", defaultIfEmpty(model.CustomAPIURL, "not set")),
		fmt.Sprintf("- Model name: %s", defaultIfEmpty(model.CustomModelName, "not set")),
	}
	if strings.TrimSpace(model.WalletAddress) != "" {
		lines = append(lines, fmt.Sprintf("- Wallet address: %s", model.WalletAddress))
	}
	if strings.TrimSpace(model.BalanceUSDC) != "" {
		lines = append(lines, fmt.Sprintf("- Wallet balance: %s USDC", model.BalanceUSDC))
	}
	return strings.Join(lines, "\n"), true
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
			if !store.IsVisibleExchange(exchange) {
				continue
			}
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
	name := resolveStrategyCreateName(&session, text)
	if actionRequiresSlot("strategy_management", "create", "name") && name == "" {
		setSkillDAGStep(&session, "resolve_name")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建策略，我还需要：" + slotDisplayName("name", lang) + "。你可以直接说：创建一个叫“趋势策略A”的策略。"
		}
		return "To create a strategy, I need a strategy name. You can say: create a strategy called 'Trend A'."
	}
	if fieldValue(session, "strategy_type") == "" {
		if strategyType := parseStrategyTypeValue(text); strategyType != "" {
			setStrategyCreateType(&session, strategyType)
		}
	} else if strategyType := parseStrategyTypeValue(text); strategyType != "" {
		setStrategyCreateType(&session, strategyType)
	}
	cfg, configMap, warnings, cfgErr := strategyCreateConfigFromSession(session, lang)
	if cfgErr != nil {
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建策略失败：" + cfgErr.Error()
		}
		return "That strategy config could not be prepared: " + cfgErr.Error()
	}
	if ready, missingKind := strategyCreateConfigReady(session, cfg, text); !ready {
		setField(&session, strategyCreateDraftConfigField, marshalStrategyCreateDraft(cfg))
		setSkillDAGStep(&session, "collect_config")
		session.Phase = "collecting"
		a.saveSkillSession(userID, session)
		if reply := formatStrategyCreateFieldOptionsReply(lang, text, missingKind); reply != "" {
			return reply
		}
		return formatStrategyCreateConfigNeeded(lang, missingKind)
	}
	if !strategyCreateConfirmationReply(text) && !strategyCreateFinalConfirmationReady(session) {
		setField(&session, strategyCreateDraftConfigField, marshalStrategyCreateDraft(cfg))
		setField(&session, "awaiting_final_confirmation", "true")
		setSkillDAGStep(&session, "await_create_confirmation")
		session.Phase = "await_create_confirmation"
		a.saveSkillSession(userID, session)
		return formatStrategyCreateFinalConfirmation(lang, session, cfg)
	}

	setSkillDAGStep(&session, "execute_create")
	args := map[string]any{
		"action":               "create",
		"name":                 name,
		"lang":                 defaultIfEmpty(lang, "zh"),
		"allow_clamped_update": true,
		"confirmed":            true,
	}
	if len(configMap) > 0 {
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
	a.rememberReferencesFromToolResult(userID, "manage_strategy", resp)
	return formatCreatedStrategyReply(lang, name, cfg, warnings)
}

func formatCreatedStrategyReply(lang, name string, cfg store.StrategyConfig, warnings []string) string {
	name = defaultIfEmpty(strings.TrimSpace(name), "未命名策略")
	if lang != "zh" {
		name = defaultIfEmpty(strings.TrimSpace(name), "unnamed strategy")
	}
	_ = warnings
	if lang == "zh" {
		lines := []string{fmt.Sprintf("已创建策略“%s”。实际保存配置如下：", name)}
		if cfg.StrategyType == "grid_trading" && cfg.GridConfig != nil {
			grid := cfg.GridConfig
			lines = append(lines,
				"- 类型：网格策略",
				fmt.Sprintf("- 交易对：%s", defaultIfEmpty(grid.Symbol, "未设置")),
				fmt.Sprintf("- 网格数量：%d", grid.GridCount),
				fmt.Sprintf("- 总投入：%.2f USDT", grid.TotalInvestment),
				fmt.Sprintf("- 杠杆：%d倍", grid.Leverage),
				fmt.Sprintf("- 分布方式：%s", defaultIfEmpty(grid.Distribution, "未设置")),
			)
			if grid.UseATRBounds {
				lines = append(lines, fmt.Sprintf("- 价格范围：ATR 自动计算（倍数 %.2f）", grid.ATRMultiplier))
			} else {
				lines = append(lines, fmt.Sprintf("- 价格范围：%.2f ～ %.2f USDT", grid.LowerPrice, grid.UpperPrice))
			}
			lines = append(lines,
				fmt.Sprintf("- 最大回撤：%.2f%%", grid.MaxDrawdownPct),
				fmt.Sprintf("- 止损：%.2f%%", grid.StopLossPct),
				fmt.Sprintf("- 日亏损限制：%.2f%%", grid.DailyLossLimitPct),
				fmt.Sprintf("- 只挂 Maker：%t", grid.UseMakerOnly),
			)
		} else {
			lines = append(lines,
				"- 类型：AI 策略",
				fmt.Sprintf("- 选币来源：%s", defaultIfEmpty(cfg.CoinSource.SourceType, "未设置")),
				fmt.Sprintf("- 主周期：%s", defaultIfEmpty(cfg.Indicators.Klines.PrimaryTimeframe, "未设置")),
				fmt.Sprintf("- BTC/ETH 最大杠杆：%d倍", cfg.RiskControl.BTCETHMaxLeverage),
				fmt.Sprintf("- 山寨币最大杠杆：%d倍", cfg.RiskControl.AltcoinMaxLeverage),
				fmt.Sprintf("- 最小置信度：%d", cfg.RiskControl.MinConfidence),
				fmt.Sprintf("- 最小盈亏比：%.2f", cfg.RiskControl.MinRiskRewardRatio),
			)
		}
		return strings.Join(lines, "\n")
	}

	lines := []string{fmt.Sprintf("Created strategy %q with this saved config:", name)}
	if cfg.StrategyType == "grid_trading" && cfg.GridConfig != nil {
		grid := cfg.GridConfig
		lines = append(lines,
			"- Type: grid strategy",
			fmt.Sprintf("- Symbol: %s", defaultIfEmpty(grid.Symbol, "unset")),
			fmt.Sprintf("- Grid count: %d", grid.GridCount),
			fmt.Sprintf("- Total investment: %.2f USDT", grid.TotalInvestment),
			fmt.Sprintf("- Leverage: %dx", grid.Leverage),
			fmt.Sprintf("- Distribution: %s", defaultIfEmpty(grid.Distribution, "unset")),
		)
		if grid.UseATRBounds {
			lines = append(lines, fmt.Sprintf("- Price range: ATR auto bounds (multiplier %.2f)", grid.ATRMultiplier))
		} else {
			lines = append(lines, fmt.Sprintf("- Price range: %.2f - %.2f USDT", grid.LowerPrice, grid.UpperPrice))
		}
	} else {
		lines = append(lines,
			"- Type: AI strategy",
			fmt.Sprintf("- Coin source: %s", defaultIfEmpty(cfg.CoinSource.SourceType, "unset")),
			fmt.Sprintf("- Primary timeframe: %s", defaultIfEmpty(cfg.Indicators.Klines.PrimaryTimeframe, "unset")),
		)
	}
	return strings.Join(lines, "\n")
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
