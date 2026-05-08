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

var (
	firstIntegerPattern = regexp.MustCompile(`\d+`)
	timeframeTokenRE    = regexp.MustCompile(`(?i)\b\d{1,2}[mhdw]\b`)
)

func parseStandaloneInteger(text string) (int, bool) {
	match := firstIntegerPattern.FindString(strings.TrimSpace(text))
	if match == "" {
		return 0, false
	}
	value, err := strconv.Atoi(match)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseEnabledValue(text string) (bool, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(lower, []string{"启用", "打开", "开启", "enable", "enabled", "on"}):
		return true, true
	case containsAny(lower, []string{"禁用", "关闭", "停用", "disable", "disabled", "off"}):
		return false, true
	default:
		return false, false
	}
}

func parseFlagValue(text string, keywords []string) (bool, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || !containsAny(lower, keywords) {
		return false, false
	}
	switch {
	case containsAny(lower, []string{"启用", "打开", "开启", "使用", "用", "是", "true", "enable", "enabled", "on", "use"}):
		return true, true
	case containsAny(lower, []string{"禁用", "关闭", "停用", "不用", "不要", "否", "false", "disable", "disabled", "off", "don't use", "do not use"}):
		return false, true
	default:
		return false, false
	}
}

func extractCredentialValue(text string, keywords []string) string {
	if value := extractQuotedContent(text); value != "" && containsAny(strings.ToLower(text), keywords) {
		return value
	}
	return extractPostKeywordName(text, keywords)
}

func parseScanIntervalMinutes(text string) (int, bool) {
	if value, ok := extractLabeledInt(text, []string{"扫描间隔", "扫描频率", "scan interval", "scan frequency"}); ok {
		return value, true
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if !containsAny(lower, []string{"扫描间隔", "扫描频率", "scan interval", "scan frequency"}) {
		return 0, false
	}
	return parseStandaloneInteger(text)
}

func detectStrategyConfigField(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(lower, []string{"最大持仓", "最多持仓", "max positions"}):
		return "max_positions"
	case containsAny(lower, []string{"最低置信度", "最小置信度", "min confidence"}):
		return "min_confidence"
	case containsAny(lower, []string{"btc/eth杠杆", "btc eth杠杆", "btc eth leverage", "btc/eth leverage", "主流币杠杆"}):
		return "btceth_max_leverage"
	case containsAny(lower, []string{"山寨币杠杆", "altcoin leverage", "alts leverage"}):
		return "altcoin_max_leverage"
	case containsAny(lower, []string{"ema"}):
		return "enable_ema"
	case containsAny(lower, []string{"macd"}):
		return "enable_macd"
	case containsAny(lower, []string{"rsi"}):
		return "enable_rsi"
	case containsAny(lower, []string{"atr"}):
		return "enable_atr"
	case containsAny(lower, []string{"boll", "bollinger", "布林"}):
		return "enable_boll"
	case containsAny(lower, []string{"核心指标"}) && containsAny(lower, []string{"全选", "全部", "全开", "都开", "都启用", "全部启用"}):
		return "enable_all_core_indicators"
	case containsAny(lower, []string{"主周期", "主时间周期", "primary timeframe"}):
		return "primary_timeframe"
	case containsAny(lower, []string{"多周期", "时间框架", "timeframes", "selected timeframes"}):
		return "selected_timeframes"
	default:
		return ""
	}
}

func strategyConfigFieldDisplayName(field, lang string) string {
	switch field {
	case "max_positions":
		if lang == "zh" {
			return "最大持仓"
		}
		return "max positions"
	case "min_confidence":
		if lang == "zh" {
			return "最小置信度"
		}
		return "min confidence"
	case "btceth_max_leverage":
		if lang == "zh" {
			return "BTC/ETH 最大杠杆"
		}
		return "BTC/ETH max leverage"
	case "altcoin_max_leverage":
		if lang == "zh" {
			return "山寨币最大杠杆"
		}
		return "altcoin max leverage"
	case "enable_ema":
		if lang == "zh" {
			return "EMA"
		}
		return "EMA"
	case "enable_macd":
		if lang == "zh" {
			return "MACD"
		}
		return "MACD"
	case "enable_rsi":
		if lang == "zh" {
			return "RSI"
		}
		return "RSI"
	case "enable_atr":
		if lang == "zh" {
			return "ATR"
		}
		return "ATR"
	case "enable_boll":
		if lang == "zh" {
			return "Bollinger"
		}
		return "Bollinger"
	case "enable_all_core_indicators":
		if lang == "zh" {
			return "全部核心指标"
		}
		return "all core indicators"
	case "primary_timeframe":
		if lang == "zh" {
			return "主周期"
		}
		return "primary timeframe"
	case "selected_timeframes":
		if lang == "zh" {
			return "多周期时间框架"
		}
		return "selected timeframes"
	default:
		return field
	}
}

func extractStrategyConfigValue(text, field string) (string, bool) {
	switch field {
	case "max_positions":
		if value, ok := extractLabeledInt(text, []string{"最大持仓", "最多持仓", "max positions"}); ok {
			return strconv.Itoa(value), true
		}
		if value, ok := parseStandaloneInteger(text); ok {
			return strconv.Itoa(value), true
		}
	case "min_confidence":
		if value, ok := extractLabeledInt(text, []string{"最低置信度", "最小置信度", "min confidence"}); ok {
			return strconv.Itoa(value), true
		}
		if value, ok := parseStandaloneInteger(text); ok {
			return strconv.Itoa(value), true
		}
	case "btceth_max_leverage":
		if value, ok := extractLabeledInt(text, []string{"btc/eth杠杆", "btc eth杠杆", "btc/eth leverage", "btc eth leverage", "主流币杠杆"}); ok {
			return strconv.Itoa(value), true
		}
		if value, ok := parseStandaloneInteger(text); ok {
			return strconv.Itoa(value), true
		}
	case "altcoin_max_leverage":
		if value, ok := extractLabeledInt(text, []string{"山寨币杠杆", "altcoin leverage", "alts leverage"}); ok {
			return strconv.Itoa(value), true
		}
		if value, ok := parseStandaloneInteger(text); ok {
			return strconv.Itoa(value), true
		}
	case "enable_ema", "enable_macd", "enable_rsi", "enable_atr", "enable_boll":
		if enabled, ok := parseEnabledValue(text); ok {
			return strconv.FormatBool(enabled), true
		}
	case "enable_all_core_indicators":
		lower := strings.ToLower(strings.TrimSpace(text))
		switch {
		case containsAny(lower, []string{"全选", "全部", "全开", "都开", "都启用", "全部启用"}):
			return "true", true
		case containsAny(lower, []string{"关闭", "停用", "禁用", "全部关闭", "全部禁用"}):
			return "false", true
		}
	case "primary_timeframe":
		if tf := extractTimeframeAfterKeywords(text, []string{"主周期", "主时间周期", "primary timeframe", "timeframe"}); tf != "" {
			return tf, true
		}
	case "selected_timeframes":
		if tfs := extractTimeframes(text); len(tfs) > 0 {
			return strings.Join(tfs, ","), true
		}
	}
	return "", false
}

type strategyConfigPatch struct {
	Field string
	Value string
}

func detectStrategyConfigPatches(text string) []strategyConfigPatch {
	seen := map[string]string{}
	addPatch := func(field, value string) {
		field = strings.TrimSpace(field)
		value = strings.TrimSpace(value)
		if field == "" || value == "" {
			return
		}
		seen[field] = value
	}

	for _, field := range []string{
		"max_positions",
		"min_confidence",
		"btceth_max_leverage",
		"altcoin_max_leverage",
		"primary_timeframe",
		"selected_timeframes",
		"enable_ema",
		"enable_macd",
		"enable_rsi",
		"enable_atr",
		"enable_boll",
		"enable_all_core_indicators",
	} {
		if value, ok := extractStrategyConfigValue(text, field); ok {
			if field == "enable_all_core_indicators" {
				addPatch("enable_ema", value)
				addPatch("enable_macd", value)
				addPatch("enable_rsi", value)
				addPatch("enable_atr", value)
				addPatch("enable_boll", value)
				continue
			}
			addPatch(field, value)
		}
	}

	fields := make([]string, 0, len(seen))
	for field := range seen {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	patches := make([]strategyConfigPatch, 0, len(fields))
	for _, field := range fields {
		patches = append(patches, strategyConfigPatch{Field: field, Value: seen[field]})
	}
	return patches
}

func applyStrategyConfigPatch(cfg *store.StrategyConfig, field, value string) error {
	switch field {
	case "max_positions":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("最大持仓需要是整数")
		}
		cfg.RiskControl.MaxPositions = parsed
	case "min_confidence":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("最小置信度需要是整数")
		}
		cfg.RiskControl.MinConfidence = parsed
	case "btceth_max_leverage":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("BTC/ETH 最大杠杆需要是整数")
		}
		cfg.RiskControl.BTCETHMaxLeverage = parsed
	case "altcoin_max_leverage":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("山寨币最大杠杆需要是整数")
		}
		cfg.RiskControl.AltcoinMaxLeverage = parsed
	case "primary_timeframe":
		cfg.Indicators.Klines.PrimaryTimeframe = value
	case "selected_timeframes":
		tfs := strings.Split(value, ",")
		cfg.Indicators.Klines.SelectedTimeframes = tfs
		cfg.Indicators.Klines.EnableMultiTimeframe = len(tfs) > 1
	case "enable_ema":
		cfg.Indicators.EnableEMA = value == "true"
	case "enable_macd":
		cfg.Indicators.EnableMACD = value == "true"
	case "enable_rsi":
		cfg.Indicators.EnableRSI = value == "true"
	case "enable_atr":
		cfg.Indicators.EnableATR = value == "true"
	case "enable_boll":
		cfg.Indicators.EnableBOLL = value == "true"
	default:
		return fmt.Errorf("unsupported strategy config field: %s", field)
	}
	return nil
}

func (a *Agent) executeTraderManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.TargetRef == nil && session.Action != "query" && session.Action != "query_list" && session.Action != "create" {
		if lang == "zh" {
			return "请先告诉我你要操作哪个交易员。"
		}
		return "Please specify which trader you want to manage."
	}
	switch session.Action {
	case "query", "query_list":
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
	case "query_detail":
		if detail, ok := a.describeTrader(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
	case "start", "stop", "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		var resp string
		switch session.Action {
		case "start":
			setSkillDAGStep(&session, "execute_start")
			resp = a.toolStartTrader(storeUserID, session.TargetRef.ID)
		case "stop":
			setSkillDAGStep(&session, "execute_stop")
			resp = a.toolStopTrader(storeUserID, session.TargetRef.ID)
		case "delete":
			setSkillDAGStep(&session, "execute_delete")
			resp = a.toolDeleteTrader(storeUserID, session.TargetRef.ID)
		}
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "执行失败：" + errMsg
			}
			return "Action failed: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已完成交易员操作：%s。", session.Action)
		}
		return fmt.Sprintf("Completed trader action: %s.", session.Action)
	case "update", "update_name", "update_bindings":
		if session.Action == "update_bindings" {
			if fieldValue(session, skillDAGStepField) == "" {
				setSkillDAGStep(&session, "collect_bindings")
			}
			args := manageTraderArgs{Action: "update", TraderID: session.TargetRef.ID}
			if match := pickMentionedOption(text, a.loadEnabledModelOptions(storeUserID)); match != nil {
				args.AIModelID = match.ID
			}
			if match := pickMentionedOption(text, a.loadExchangeOptions(storeUserID)); match != nil {
				args.ExchangeID = match.ID
			}
			if match := pickMentionedOption(text, a.loadStrategyOptions(storeUserID)); match != nil {
				args.StrategyID = match.ID
			}
			if args.AIModelID != "" {
				setField(&session, "ai_model_id", args.AIModelID)
			}
			if args.ExchangeID != "" {
				setField(&session, "exchange_id", args.ExchangeID)
			}
			if args.StrategyID != "" {
				setField(&session, "strategy_id", args.StrategyID)
			}
			if value := fieldValue(session, "ai_model_id"); value != "" {
				args.AIModelID = value
			}
			if value := fieldValue(session, "exchange_id"); value != "" {
				args.ExchangeID = value
			}
			if value := fieldValue(session, "strategy_id"); value != "" {
				args.StrategyID = value
			}
			if args.AIModelID == "" && args.ExchangeID == "" && args.StrategyID == "" {
				setSkillDAGStep(&session, "collect_bindings")
				a.saveSkillSession(userID, session)
				if lang == "zh" {
					return "这次是更新交易员绑定，请直接说要换成哪个模型、交易所或策略。"
				}
				return "This action updates trader bindings. Tell me which model, exchange, or strategy to switch to."
			}
			setSkillDAGStep(&session, "execute_update")
			resp := a.toolUpdateTrader(storeUserID, args)
			a.clearSkillSession(userID)
			if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
				if lang == "zh" {
					return "更新交易员绑定失败：" + errMsg
				}
				return "Failed to update trader bindings: " + errMsg
			}
			if lang == "zh" {
				return "已更新交易员绑定。"
			}
			return "Updated trader bindings."
		}
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "collect_name")
		}
		args := manageTraderArgs{Action: "update", TraderID: session.TargetRef.ID}
		if minutes, ok := parseScanIntervalMinutes(text); ok && minutes > 0 {
			args.ScanIntervalMinutes = &minutes
		}
		if value, ok := extractStrategyConfigValue(text, "btceth_max_leverage"); ok {
			if parsed, err := strconv.Atoi(value); err == nil {
				args.BTCETHLeverage = &parsed
			}
		}
		if value, ok := extractStrategyConfigValue(text, "altcoin_max_leverage"); ok {
			if parsed, err := strconv.Atoi(value); err == nil {
				args.AltcoinLeverage = &parsed
			}
		}
		if prompt := extractCredentialValue(text, []string{"自定义提示词", "提示词", "custom prompt", "prompt"}); prompt != "" &&
			containsAny(strings.ToLower(text), []string{"提示词", "prompt"}) {
			args.CustomPrompt = prompt
		}
		if enabled, ok := parseFlagValue(text, []string{"ai500"}); ok {
			args.UseAI500 = &enabled
		}
		if enabled, ok := parseFlagValue(text, []string{"oi top", "oitop", "持仓量排名"}); ok {
			args.UseOITop = &enabled
		}
		if args.ScanIntervalMinutes != nil || args.BTCETHLeverage != nil || args.AltcoinLeverage != nil || args.CustomPrompt != "" || args.UseAI500 != nil || args.UseOITop != nil {
			setSkillDAGStep(&session, "execute_update")
			resp := a.toolUpdateTrader(storeUserID, args)
			a.clearSkillSession(userID)
			if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
				if lang == "zh" {
					return "更新交易员失败：" + errMsg
				}
				return "Failed to update trader: " + errMsg
			}
			if lang == "zh" {
				return "已更新交易员配置。"
			}
			return "Updated trader config."
		}
		newName := extractTraderName(text)
		if newName == "" {
			newName = extractPostKeywordName(text, []string{"改成", "改为", "rename to"})
		}
		if newName != "" {
			setField(&session, "name", newName)
		}
		newName = fieldValue(session, "name")
		if newName == "" {
			setSkillDAGStep(&session, "collect_name")
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新交易员这条 skill 先支持改名。请直接告诉我新的名字。"
			}
			return "This trader update skill currently supports renaming first. Tell me the new name."
		}
		args = manageTraderArgs{Action: "update", TraderID: session.TargetRef.ID, Name: newName}
		setSkillDAGStep(&session, "execute_update")
		resp := a.toolUpdateTrader(storeUserID, args)
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "更新交易员失败：" + errMsg
			}
			return "Failed to update trader: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已将交易员改名为“%s”。", newName)
		}
		return fmt.Sprintf("Renamed trader to %q.", newName)
	default:
		return ""
	}
}

func (a *Agent) executeExchangeManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.TargetRef == nil && session.Action != "query" && session.Action != "query_list" && session.Action != "create" {
		if lang == "zh" {
			return "请先告诉我你要操作哪个交易所配置。"
		}
		return "Please specify which exchange config you want to manage."
	}
	switch session.Action {
	case "query_detail":
		if detail, ok := a.describeExchange(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "get_exchange_configs", a.toolGetExchangeConfigs(storeUserID))
	case "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		setSkillDAGStep(&session, "execute_delete")
		args, _ := json.Marshal(map[string]any{"action": "delete", "exchange_id": session.TargetRef.ID})
		resp := a.toolManageExchangeConfig(storeUserID, string(args))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "删除交易所配置失败：" + errMsg
			}
			return "Failed to delete exchange config: " + errMsg
		}
		if lang == "zh" {
			return "已删除交易所配置。"
		}
		return "Deleted exchange config."
	case "update", "update_name", "update_status":
		if fieldValue(session, skillDAGStepField) == "" {
			if session.Action == "update_status" {
				setSkillDAGStep(&session, "collect_enabled")
			} else {
				setSkillDAGStep(&session, "collect_account_name")
			}
		}
		accountName := extractTraderName(text)
		if accountName == "" {
			accountName = extractPostKeywordName(text, []string{"改成", "改为", "账户名改成", "rename to"})
		}
		if accountName != "" {
			setField(&session, "account_name", accountName)
		}
		if enabled, ok := parseEnabledValue(text); ok {
			setField(&session, "enabled", strconv.FormatBool(enabled))
		}
		if value := extractCredentialValue(text, []string{"api key", "apikey", "api_key"}); value != "" {
			setField(&session, "api_key", value)
		}
		if value := extractCredentialValue(text, []string{"secret key", "secret", "secret_key"}); value != "" {
			setField(&session, "secret_key", value)
		}
		if value := extractCredentialValue(text, []string{"passphrase", "密码短语"}); value != "" {
			setField(&session, "passphrase", value)
		}
		if testnet, ok := parseFlagValue(text, []string{"testnet", "测试网"}); ok {
			setField(&session, "testnet", strconv.FormatBool(testnet))
		}
		payload := map[string]any{"action": "update", "exchange_id": session.TargetRef.ID}
		accountName = fieldValue(session, "account_name")
		if accountName != "" && session.Action != "update_status" {
			payload["account_name"] = accountName
		}
		if enabledRaw := fieldValue(session, "enabled"); enabledRaw != "" {
			payload["enabled"] = enabledRaw == "true"
		}
		if value := fieldValue(session, "api_key"); value != "" {
			payload["api_key"] = value
		}
		if value := fieldValue(session, "secret_key"); value != "" {
			payload["secret_key"] = value
		}
		if value := fieldValue(session, "passphrase"); value != "" {
			payload["passphrase"] = value
		}
		if value := fieldValue(session, "testnet"); value != "" {
			payload["testnet"] = value == "true"
		}
		if session.Action == "update_status" {
			delete(payload, "account_name")
		}
		if len(payload) == 2 {
			if session.Action == "update_status" {
				setSkillDAGStep(&session, "collect_enabled")
			} else {
				setSkillDAGStep(&session, "collect_account_name")
			}
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新交易所 skill 支持改账户名、启用状态、API Key、Secret、Passphrase 和 testnet。请告诉我你要改什么。"
			}
			return "This exchange update skill supports account name, enabled state, API key, secret, passphrase, and testnet."
		}
		setSkillDAGStep(&session, "execute_update")
		raw, _ := json.Marshal(payload)
		resp := a.toolManageExchangeConfig(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "更新交易所配置失败：" + errMsg
			}
			return "Failed to update exchange config: " + errMsg
		}
		if lang == "zh" {
			return "已更新交易所配置。"
		}
		return "Updated exchange config."
	default:
		return ""
	}
}

func (a *Agent) executeModelManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.TargetRef == nil && session.Action != "query" && session.Action != "query_list" && session.Action != "create" {
		if lang == "zh" {
			return "请先告诉我你要操作哪个模型配置。"
		}
		return "Please specify which model config you want to manage."
	}
	switch session.Action {
	case "query_detail":
		if detail, ok := a.describeModel(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "get_model_configs", a.toolGetModelConfigs(storeUserID))
	case "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		setSkillDAGStep(&session, "execute_delete")
		raw, _ := json.Marshal(map[string]any{"action": "delete", "model_id": session.TargetRef.ID})
		resp := a.toolManageModelConfig(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "删除模型配置失败：" + errMsg
			}
			return "Failed to delete model config: " + errMsg
		}
		if lang == "zh" {
			return "已删除模型配置。"
		}
		return "Deleted model config."
	case "update", "update_name", "update_endpoint", "update_status":
		if fieldValue(session, skillDAGStepField) == "" {
			switch session.Action {
			case "update_status":
				setSkillDAGStep(&session, "collect_enabled")
			case "update_endpoint":
				setSkillDAGStep(&session, "collect_custom_api_url")
			default:
				setSkillDAGStep(&session, "collect_custom_model_name")
			}
		}
		payload := map[string]any{"action": "update", "model_id": session.TargetRef.ID}
		if url := extractURL(text); url != "" {
			setField(&session, "custom_api_url", url)
		}
		if enabled, ok := parseEnabledValue(text); ok {
			setField(&session, "enabled", strconv.FormatBool(enabled))
		}
		if apiKey := extractCredentialValue(text, []string{"api key", "apikey", "api_key"}); apiKey != "" {
			setField(&session, "api_key", apiKey)
		}
		if modelName := extractPostKeywordName(text, []string{"model name", "模型名", "模型名称", "改成", "改为", "修改为", "换成", "换到", "切换为", "切换到", "change to", "switch to"}); modelName != "" {
			setField(&session, "custom_model_name", normalizeModelName(modelName))
		}
		if value := fieldValue(session, "custom_api_url"); value != "" {
			payload["custom_api_url"] = value
		}
		if value := fieldValue(session, "enabled"); value != "" {
			payload["enabled"] = value == "true"
		}
		if value := fieldValue(session, "api_key"); value != "" {
			payload["api_key"] = value
		}
		if value := fieldValue(session, "custom_model_name"); value != "" {
			payload["custom_model_name"] = value
		}
		if session.Action == "update_name" {
			delete(payload, "custom_api_url")
			delete(payload, "enabled")
			delete(payload, "api_key")
		}
		if session.Action == "update_status" {
			delete(payload, "custom_api_url")
			delete(payload, "custom_model_name")
			delete(payload, "api_key")
		}
		if session.Action == "update_endpoint" {
			delete(payload, "custom_model_name")
			delete(payload, "enabled")
			delete(payload, "api_key")
		}
		if len(payload) == 2 {
			switch session.Action {
			case "update_status":
				setSkillDAGStep(&session, "collect_enabled")
			case "update_endpoint":
				setSkillDAGStep(&session, "collect_custom_api_url")
			default:
				setSkillDAGStep(&session, "collect_custom_model_name")
			}
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新模型 skill 支持改 API Key、URL、模型名和启用状态。请告诉我你要改什么。"
			}
			return "This model update skill supports API key, URL, model name, and enabled state."
		}
		setSkillDAGStep(&session, "execute_update")
		raw, _ := json.Marshal(payload)
		resp := a.toolManageModelConfig(storeUserID, string(raw))
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				if strings.Contains(errMsg, "cannot enable model config before API key is configured") {
					return "更新模型配置失败：这个模型还没有配置 API Key，暂时不能启用。你可以直接把 API Key 发给我，我帮你继续配置。"
				}
				return "更新模型配置失败：" + errMsg
			}
			a.saveSkillSession(userID, session)
			return "Failed to update model config: " + errMsg
		}
		a.clearSkillSession(userID)
		if lang == "zh" {
			if session.Action == "update_status" {
				return "已更新模型配置启用状态。"
			}
			return "已更新模型配置。"
		}
		return "Updated model config."
	default:
		return ""
	}
}

// normalizeModelName maps common user-friendly model aliases to the canonical
// names used by claw402 and other providers (e.g. "claude opus4.6" → "claude-opus").
func normalizeModelName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	aliases := map[string]string{
		// Claude
		"claude opus":     "claude-opus",
		"claude opus4.6":  "claude-opus",
		"claude opus 4.6": "claude-opus",
		"claude-opus-4-6": "claude-opus",
		"claude sonnet":     "claude-sonnet",
		"claude sonnet4.6":  "claude-sonnet",
		"claude sonnet 4.6": "claude-sonnet",
		"claude haiku":      "claude-haiku",
		// GPT
		"gpt5.4":      "gpt-5.4",
		"gpt 5.4":     "gpt-5.4",
		"gpt5.4pro":   "gpt-5.4-pro",
		"gpt 5.4pro":  "gpt-5.4-pro",
		"gpt 5.4 pro": "gpt-5.4-pro",
		"gpt5 mini":   "gpt-5-mini",
		"gpt 5 mini":  "gpt-5-mini",
		"gpt5.3":      "gpt-5.3",
		"gpt 5.3":     "gpt-5.3",
		// DeepSeek
		"deepseek reasoner": "deepseek-reasoner",
		"deepseek chat":     "deepseek-chat",
		// Qwen (通义千问)
		"qwen max":   "qwen-max",
		"qwen plus":  "qwen-plus",
		"qwen turbo": "qwen-turbo",
		"qwen flash": "qwen-flash",
		"通义千问":       "qwen-max",
		// Gemini
		"gemini 3.1 pro": "gemini-3.1-pro",
		"gemini 3.1pro":  "gemini-3.1-pro",
		// Kimi
		"kimi k2.5": "kimi-k2.5",
		// GLM (智谱清言)
		"glm5":        "glm-5",
		"glm 5":       "glm-5",
		"glm5 turbo":  "glm-5-turbo",
		"glm 5 turbo": "glm-5-turbo",
		"glm5-turbo":  "glm-5-turbo",
		"智谱清言":        "glm-5",
	}
	if canonical, ok := aliases[lower]; ok {
		return canonical
	}
	// Replace spaces with hyphens as a general fallback
	return strings.ReplaceAll(strings.TrimSpace(name), " ", "-")
}

func (a *Agent) executeStrategyManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if session.TargetRef == nil && session.Action != "query" && session.Action != "query_list" && session.Action != "create" {
		if lang == "zh" {
			return "请先告诉我你要操作哪个策略。"
		}
		return "Please specify which strategy you want to manage."
	}
	switch session.Action {
	case "query", "query_list":
		return formatReadFastPathResponse(lang, "get_strategies", a.toolGetStrategies(storeUserID))
	case "query_detail":
		if detail, ok := a.describeStrategy(storeUserID, lang, session.TargetRef); ok {
			return detail
		}
		return formatReadFastPathResponse(lang, "get_strategies", a.toolGetStrategies(storeUserID))
	case "activate":
		raw, _ := json.Marshal(map[string]any{"action": "activate", "strategy_id": session.TargetRef.ID})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "激活策略失败：" + errMsg
			}
			return "Failed to activate strategy: " + errMsg
		}
		if lang == "zh" {
			return "已激活策略。"
		}
		return "Activated strategy."
	case "duplicate":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "collect_name")
		}
		newName := extractTraderName(text)
		if newName == "" {
			newName = extractPostKeywordName(text, []string{"叫", "名为", "改成", "rename to"})
		}
		if newName != "" {
			setField(&session, "name", newName)
		}
		newName = fieldValue(session, "name")
		if newName == "" {
			setSkillDAGStep(&session, "collect_name")
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "复制策略时，我还需要一个新名称。"
			}
			return "I still need a new name for the duplicated strategy."
		}
		setSkillDAGStep(&session, "execute_duplicate")
		raw, _ := json.Marshal(map[string]any{"action": "duplicate", "strategy_id": session.TargetRef.ID, "name": newName})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "复制策略失败：" + errMsg
			}
			return "Failed to duplicate strategy: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已复制策略，新名称为“%s”。", newName)
		}
		return fmt.Sprintf("Duplicated strategy as %q.", newName)
	case "delete":
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "await_confirmation")
		}
		if fieldValue(session, "bulk_scope") == "all" {
			strategies, err := a.store.Strategy().List(storeUserID)
			if err != nil {
				if lang == "zh" {
					return "读取策略列表失败：" + err.Error()
				}
				return "Failed to load strategies: " + err.Error()
			}

			deletable := make([]*store.Strategy, 0, len(strategies))
			skippedDefault := 0
			for _, strategy := range strategies {
				if strategy == nil {
					continue
				}
				if strategy.IsDefault {
					skippedDefault++
					continue
				}
				deletable = append(deletable, strategy)
			}
			if len(deletable) == 0 {
				a.clearSkillSession(userID)
				if lang == "zh" {
					return "当前没有可删除的自定义策略。"
				}
				return "There are no user-created strategies to delete."
			}

			targetLabel := fmt.Sprintf("全部自定义策略（共 %d 个）", len(deletable))
			if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, targetLabel); waiting {
				a.saveSkillSession(userID, session)
				return msg
			}
			if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
				a.saveSkillSession(userID, session)
				return msg
			}

			setSkillDAGStep(&session, "execute_delete")
			deletedNames := make([]string, 0, len(deletable))
			failedNames := make([]string, 0)
			for _, strategy := range deletable {
				raw, _ := json.Marshal(map[string]any{"action": "delete", "strategy_id": strategy.ID})
				resp := a.toolManageStrategy(storeUserID, string(raw))
				if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
					failedNames = append(failedNames, fmt.Sprintf("%s（%s）", strategy.Name, errMsg))
					continue
				}
				deletedNames = append(deletedNames, strategy.Name)
			}
			a.clearSkillSession(userID)

			if lang == "zh" {
				parts := []string{fmt.Sprintf("批量删除策略已完成：成功删除 %d 个。", len(deletedNames))}
				if skippedDefault > 0 {
					parts = append(parts, fmt.Sprintf("已跳过系统默认策略 %d 个。", skippedDefault))
				}
				if len(failedNames) > 0 {
					parts = append(parts, "删除失败："+strings.Join(failedNames, "；"))
				}
				if len(deletedNames) > 0 {
					parts = append(parts, "已删除："+strings.Join(deletedNames, "、"))
				}
				return strings.Join(parts, "\n")
			}

			parts := []string{fmt.Sprintf("Bulk strategy deletion finished: deleted %d strategy(s).", len(deletedNames))}
			if skippedDefault > 0 {
				parts = append(parts, fmt.Sprintf("Skipped %d default strategy(ies).", skippedDefault))
			}
			if len(failedNames) > 0 {
				parts = append(parts, "Failed: "+strings.Join(failedNames, "; "))
			}
			if len(deletedNames) > 0 {
				parts = append(parts, "Deleted: "+strings.Join(deletedNames, ", "))
			}
			return strings.Join(parts, "\n")
		}
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		setSkillDAGStep(&session, "execute_delete")
		raw, _ := json.Marshal(map[string]any{"action": "delete", "strategy_id": session.TargetRef.ID})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "删除策略失败：" + errMsg
			}
			return "Failed to delete strategy: " + errMsg
		}
		if lang == "zh" {
			return "已删除策略。"
		}
		return "Deleted strategy."
	case "update", "update_name", "update_config", "update_prompt":
		if session.Action == "update_prompt" {
			return a.executeStrategyPromptUpdate(storeUserID, userID, lang, text, session)
		}
		if session.Action == "update_config" {
			return a.executeStrategyConfigUpdate(storeUserID, userID, lang, text, session)
		}
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "collect_name")
		}
		newName := extractTraderName(text)
		if newName == "" {
			newName = extractPostKeywordName(text, []string{"改成", "改为", "rename to"})
		}
		if newName != "" {
			setField(&session, "name", newName)
		}
		newName = fieldValue(session, "name")
		if newName == "" {
			setSkillDAGStep(&session, "collect_name")
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新策略 skill 先支持改名。请告诉我新的策略名称。"
			}
			return "This strategy update skill currently supports renaming first."
		}
		setSkillDAGStep(&session, "execute_update")
		raw, _ := json.Marshal(map[string]any{"action": "update", "strategy_id": session.TargetRef.ID, "name": newName})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "更新策略失败：" + errMsg
			}
			return "Failed to update strategy: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已将策略改名为“%s”。", newName)
		}
		return fmt.Sprintf("Renamed strategy to %q.", newName)
	default:
		return ""
	}
}

func (a *Agent) executeStrategyPromptUpdate(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if fieldValue(session, skillDAGStepField) == "" {
		setSkillDAGStep(&session, "collect_prompt")
	}
	strategy, cfg, err := a.loadStrategyConfigForUpdate(storeUserID, session.TargetRef.ID)
	if err != nil {
		if lang == "zh" {
			return "读取策略失败：" + err.Error()
		}
		return "Failed to load strategy: " + err.Error()
	}

	prompt := extractQuotedContent(text)
	if prompt == "" {
		prompt = extractPostKeywordName(text, []string{"prompt改成", "prompt 改成", "提示词改成", "提示词改为", "custom prompt 改成"})
	}
	if prompt != "" {
		setField(&session, "prompt", prompt)
	}
	prompt = fieldValue(session, "prompt")
	if prompt == "" {
		setSkillDAGStep(&session, "collect_prompt")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "这次是更新策略 prompt，请直接把新的 prompt 内容发给我，最好放在引号里。"
		}
		return "This action updates the strategy prompt. Send me the new prompt text, ideally inside quotes."
	}

	cfg.CustomPrompt = prompt
	setSkillDAGStep(&session, "execute_update")
	return a.persistStrategyConfigUpdate(storeUserID, userID, lang, strategy, cfg, "已更新策略 prompt。", "Updated strategy prompt.")
}

func (a *Agent) executeStrategyConfigUpdate(storeUserID string, userID int64, lang, text string, session skillSession) string {
	if _, ok := getSkillDAG("strategy_management", "update_config"); ok {
		if fieldValue(session, skillDAGStepField) == "" {
			setSkillDAGStep(&session, "resolve_config_field")
		}
	}

	currentStep, _ := currentSkillDAGStep(session)
	strategy, cfg, err := a.loadStrategyConfigForUpdate(storeUserID, session.TargetRef.ID)
	if err != nil {
		if lang == "zh" {
			return "读取策略失败：" + err.Error()
		}
		return "Failed to load strategy: " + err.Error()
	}

	if fieldValue(session, "config_field") == "" && fieldValue(session, "config_value") == "" {
		patches := detectStrategyConfigPatches(text)
		if len(patches) > 1 {
			changed := make([]string, 0, len(patches))
			for _, patch := range patches {
				if err := applyStrategyConfigPatch(&cfg, patch.Field, patch.Value); err != nil {
					a.saveSkillSession(userID, session)
					if lang == "zh" {
						return "更新策略参数失败：" + err.Error()
					}
					return "Failed to update strategy config: " + err.Error()
				}
				changed = append(changed, strategyConfigFieldDisplayName(patch.Field, lang))
			}
			cfg.ClampLimits()
			setSkillDAGStep(&session, "apply_field_update")
			setSkillDAGStep(&session, "execute_update")
			msgZH := "已更新策略参数：" + strings.Join(changed, "、") + "。"
			msgEN := "Updated strategy config fields: " + strings.Join(changed, ", ") + "."
			return a.persistStrategyConfigUpdate(storeUserID, userID, lang, strategy, cfg, msgZH, msgEN)
		}
	}

	field := fieldValue(session, "config_field")
	if field == "" {
		field = detectStrategyConfigField(text)
		if field != "" {
			setField(&session, "config_field", field)
			if currentStep.ID == "resolve_config_field" {
				advanceSkillDAGStep(&session, currentStep.ID)
				currentStep, _ = currentSkillDAGStep(session)
			}
		}
	}
	if field == "" {
		setSkillDAGStep(&session, "resolve_config_field")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "这次是更新策略参数。我当前先支持这些字段：最大持仓、最低置信度、主周期、多周期时间框架。请先告诉我要改哪个字段。"
		}
		return "This action updates strategy config. I currently support max positions, min confidence, primary timeframe, and selected timeframes. Tell me which field to change first."
	}

	if value, ok := extractStrategyConfigValue(text, field); ok {
		setField(&session, "config_value", value)
		if currentStep.ID == "resolve_config_value" {
			advanceSkillDAGStep(&session, currentStep.ID)
			currentStep, _ = currentSkillDAGStep(session)
		}
	}
	value := fieldValue(session, "config_value")
	if value == "" {
		setSkillDAGStep(&session, "resolve_config_value")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return fmt.Sprintf("要更新策略参数，我还需要 %s 的目标值。", strategyConfigFieldDisplayName(field, lang))
		}
		return fmt.Sprintf("I still need the target value for %s.", strategyConfigFieldDisplayName(field, lang))
	}

	if err := applyStrategyConfigPatch(&cfg, field, value); err != nil {
		setSkillDAGStep(&session, "resolve_config_value")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return err.Error()
		}
		return err.Error()
	}

	cfg.ClampLimits()
	changed := []string{field}
	displayChanged := make([]string, 0, len(changed))
	for _, item := range changed {
		displayChanged = append(displayChanged, strategyConfigFieldDisplayName(item, lang))
	}
	msgZH := "已更新策略参数：" + strings.Join(displayChanged, "、") + "。"
	msgEN := "Updated strategy config fields: " + strings.Join(displayChanged, ", ") + "."
	setSkillDAGStep(&session, "apply_field_update")
	setSkillDAGStep(&session, "execute_update")
	return a.persistStrategyConfigUpdate(storeUserID, userID, lang, strategy, cfg, msgZH, msgEN)
}

func (a *Agent) loadStrategyConfigForUpdate(storeUserID, strategyID string) (*store.Strategy, store.StrategyConfig, error) {
	strategy, err := a.store.Strategy().Get(storeUserID, strategyID)
	if err != nil {
		return nil, store.StrategyConfig{}, err
	}
	cfg := store.GetDefaultStrategyConfig("zh")
	if strings.TrimSpace(strategy.Config) != "" {
		_ = json.Unmarshal([]byte(strategy.Config), &cfg)
	}
	return strategy, cfg, nil
}

func (a *Agent) persistStrategyConfigUpdate(storeUserID string, userID int64, lang string, strategy *store.Strategy, cfg store.StrategyConfig, zhMsg, enMsg string) string {
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		if lang == "zh" {
			return "序列化策略配置失败：" + err.Error()
		}
		return "Failed to serialize strategy config: " + err.Error()
	}
	raw, _ := json.Marshal(map[string]any{
		"action":      "update",
		"strategy_id": strategy.ID,
		"config":      json.RawMessage(rawConfig),
	})
	resp := a.toolManageStrategy(storeUserID, string(raw))
	a.clearSkillSession(userID)
	if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
		if lang == "zh" {
			return "更新策略失败：" + errMsg
		}
		return "Failed to update strategy: " + errMsg
	}
	if lang == "zh" {
		return zhMsg
	}
	return enMsg
}

func extractQuotedContent(text string) string {
	if matches := quotedNamePattern.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractLabeledInt(text string, labels []string) (int, bool) {
	lower := strings.ToLower(text)
	for _, label := range labels {
		idx := strings.Index(lower, strings.ToLower(label))
		if idx < 0 {
			continue
		}
		segment := text[idx:]
		if match := firstIntegerPattern.FindString(segment); match != "" {
			if value, err := strconv.Atoi(match); err == nil {
				return value, true
			}
		}
	}
	return 0, false
}

func extractTimeframeAfterKeywords(text string, labels []string) string {
	lower := strings.ToLower(text)
	for _, label := range labels {
		idx := strings.Index(lower, strings.ToLower(label))
		if idx < 0 {
			continue
		}
		segment := text[idx:]
		if match := timeframeTokenRE.FindString(segment); match != "" {
			return strings.ToLower(match)
		}
	}
	return ""
}

func extractTimeframes(text string) []string {
	matches := timeframeTokenRE.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		tf := strings.ToLower(strings.TrimSpace(match))
		if tf == "" {
			continue
		}
		if _, ok := seen[tf]; ok {
			continue
		}
		seen[tf] = struct{}{}
		out = append(out, tf)
	}
	return out
}

func (a *Agent) handleTraderDiagnosisSkill(storeUserID, lang, text string) string {
	raw := a.toolListTraders(storeUserID)
	list := formatReadFastPathResponse(lang, "list_traders", raw)
	if lang == "zh" {
		reply := "现象：这是交易员运行诊断问题。\n优先排查：\n1. 交易员是否已创建并处于运行状态。\n2. 绑定的模型、交易所、策略是否齐全。\n3. 是“没有启动”、还是“启动了但 AI 没有下单”、还是“下单失败”。\n当前交易员概览：\n" + list
		if excerpt := backendLogDiagnosisExcerpt(lang, text, "trader"); excerpt != "" {
			reply += "\n" + excerpt
		}
		return reply
	}
	reply := "This looks like a trader diagnosis issue.\nCheck whether the trader exists, is running, and has model/exchange/strategy bindings.\nCurrent trader overview:\n" + list
	if excerpt := backendLogDiagnosisExcerpt(lang, text, "trader"); excerpt != "" {
		reply += "\n" + excerpt
	}
	return reply
}

func (a *Agent) handleStrategyDiagnosisSkill(storeUserID, lang, text string) string {
	raw := a.toolGetStrategies(storeUserID)
	list := formatReadFastPathResponse(lang, "get_strategies", raw)
	if lang == "zh" {
		reply := "现象：这是策略或提示词生效问题。\n优先排查：\n1. 你改的是策略模板，还是 trader 上的 custom prompt。\n2. 策略是否真的保存成功。\n3. 运行结果不符合预期，是配置问题还是市场条件问题。\n当前策略概览：\n" + list
		if excerpt := backendLogDiagnosisExcerpt(lang, text, "strategy"); excerpt != "" {
			reply += "\n" + excerpt
		}
		return reply
	}
	reply := "This looks like a strategy or prompt diagnosis issue.\nCheck whether you changed the strategy template or a trader-specific prompt override.\nCurrent strategy overview:\n" + list
	if excerpt := backendLogDiagnosisExcerpt(lang, text, "strategy"); excerpt != "" {
		reply += "\n" + excerpt
	}
	return reply
}
