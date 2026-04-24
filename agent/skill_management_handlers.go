package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"nofx/store"
)

var urlPattern = regexp.MustCompile(`https://[^\s"'<>]+`)

func detectTraderManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{"交易员", "trader", "agent"}) &&
		containsAny(lower, []string{"修改", "编辑", "更新", "改", "改一下", "删除", "删了", "启动", "停止", "查看", "查询", "列出", "rename", "update", "delete", "start", "stop", "list", "show"})
}

func detectExchangeManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{"交易所", "exchange", "okx", "binance", "bybit", "gate", "kucoin", "hyperliquid"}) &&
		containsAny(lower, []string{"创建", "新建", "修改", "编辑", "更新", "改", "改一下", "删除", "删了", "查询", "查看", "列出", "启用", "禁用", "改名", "rename", "create", "update", "delete", "list", "show", "enable", "disable"})
}

func detectModelManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{"模型", "model", "provider", "deepseek", "openai", "claude", "gemini", "qwen", "kimi", "grok", "minimax"}) &&
		containsAny(lower, []string{"创建", "新建", "修改", "编辑", "更新", "改", "改一下", "删除", "删了", "查询", "查看", "列出", "启用", "禁用", "改名", "rename", "create", "update", "delete", "list", "show", "enable", "disable"})
}

func detectStrategyManagementIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if wantsDefaultStrategyConfig(text) {
		return true
	}
	return containsAny(lower, []string{"策略", "strategy"}) &&
		containsAny(lower, []string{"创建", "新建", "修改", "编辑", "更新", "改", "改一下", "改成", "改为", "删除", "删了", "查询", "查看", "列出", "激活", "复制", "参数", "配置", "详情", "详细", "prompt", "提示词", "什么样", "怎么样", "create", "update", "delete", "list", "show", "activate", "duplicate", "detail", "details", "config", "configuration", "parameter", "prompt", "what kind"})
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
	hasUpdateVerb := containsAny(lower, []string{"修改", "编辑", "更新", "改", "rename", "update", "切换", "换成", "换到"})
	switch {
	case containsAny(lower, []string{"删除", "删掉", "删了", "remove", "delete"}):
		return "delete"
	case containsAny(lower, []string{"启动", "开始", "run", "start"}) && domain == "trader":
		return "start"
	case containsAny(lower, []string{"停止", "停掉", "stop", "pause"}) && domain == "trader":
		return "stop"
	case containsAny(lower, []string{"激活", "activate"}) && domain == "strategy":
		return "activate"
	case containsAny(lower, []string{"复制", "duplicate"}) && domain == "strategy":
		return "duplicate"
	case containsAny(lower, []string{"改名", "重命名", "rename"}):
		return "update_name"
	case domain == "trader" && containsAny(lower, []string{"换模型", "换交易所", "换策略", "绑定", "切换模型", "切换交易所", "切换策略"}):
		return "update_bindings"
	case (domain == "exchange" || domain == "model") && containsAny(lower, []string{"启用", "禁用", "enable", "disable"}):
		return "update_status"
	case domain == "model" && hasUpdateVerb && containsAny(lower, []string{"url", "endpoint", "地址", "接口"}):
		return "update_endpoint"
	case domain == "strategy" && hasUpdateVerb && containsAny(lower, []string{"prompt", "提示词"}):
		return "update_prompt"
	case domain == "strategy" && hasUpdateVerb && containsAny(lower, []string{
		"参数", "配置", "config", "configuration", "parameter",
		"最大持仓", "最小置信度", "最低置信度", "主周期", "多周期", "时间框架",
		"btc/eth杠杆", "btc eth杠杆", "山寨币杠杆",
		"核心指标", "ema", "macd", "rsi", "atr", "boll", "bollinger", "布林",
	}):
		return "update_config"
	case containsAny(lower, []string{"修改", "编辑", "更新", "改", "rename", "update"}):
		return "update"
	case domain == "trader" && containsAny(lower, []string{"运行中的", "在跑", "running"}):
		return "query_running"
	case !containsAny(lower, []string{"创建", "新建", "create", "new"}) &&
		containsAny(lower, []string{"详情", "详细", "prompt", "提示词", "什么样", "怎么样", "detail", "details", "what kind"}):
		return "query_detail"
	case containsAny(lower, []string{"查询", "查看", "列出", "list", "show", "有哪些"}):
		return "query_list"
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

func textMeansAllTargets(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"全部", "所有", "全都", "全部策略", "所有策略",
		"all", "all strategies", "every strategy",
	})
}

func supportsBulkTargetSelection(skillName, action string) bool {
	return skillName == "strategy_management" && action == "delete"
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
	if action == "query_running" {
		answer := formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
		return applyTraderQueryFilter(lang, answer, a.toolListTraders(storeUserID), "running_only"), true
	}
	if action == "query_detail" {
		options := a.loadTraderOptions(storeUserID)
		target := resolveTargetFromText(text, options, session.TargetRef)
		if detail, ok := a.describeTrader(storeUserID, lang, target); ok {
			return detail, true
		}
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID)), true
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
	case "query_list":
		return formatReadFastPathResponse(lang, "get_exchange_configs", a.toolGetExchangeConfigs(storeUserID)), true
	case "query_detail":
		target := resolveTargetFromText(text, options, session.TargetRef)
		if detail, ok := a.describeExchange(storeUserID, lang, target); ok {
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
	action := detectManagementAction(text, "model")
	if session.Name == "model_management" && session.Action != "" {
		action = session.Action
	}
	if action == "" {
		return "", false
	}
	options := a.loadEnabledModelOptions(storeUserID)
	switch action {
	case "query_list":
		return formatReadFastPathResponse(lang, "get_model_configs", a.toolGetModelConfigs(storeUserID)), true
	case "query_detail":
		target := resolveTargetFromText(text, options, session.TargetRef)
		if detail, ok := a.describeModel(storeUserID, lang, target); ok {
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
	action := detectManagementAction(text, "strategy")
	if session.Name == "strategy_management" && session.Action != "" {
		action = session.Action
	}
	if action == "" && wantsStrategyDetails(text) {
		action = "query_detail"
	}
	if action == "" {
		return "", false
	}
	options := a.loadStrategyOptions(storeUserID)
	switch action {
	case "query_detail":
		if wantsDefaultStrategyConfig(text) {
			return a.describeDefaultStrategyConfig(lang), true
		}
		target := resolveTargetFromText(text, options, session.TargetRef)
		if detail, ok := a.describeStrategy(storeUserID, lang, target); ok {
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

func wantsStrategyDetails(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"什么样", "怎么样", "详情", "详细", "参数", "配置", "prompt", "提示词",
		"what kind", "details", "detail", "config", "configuration", "parameter", "prompt",
	})
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

	if lang == "zh" {
		lines := []string{
			fmt.Sprintf("策略“%s”概览：", name),
			fmt.Sprintf("- 类型：%s", defaultIfEmpty(strings.TrimSpace(cfg.StrategyType), "ai_trading")),
			fmt.Sprintf("- 语言：%s", defaultIfEmpty(strings.TrimSpace(cfg.Language), "zh")),
		}
		if strings.TrimSpace(strategy.Description) != "" {
			lines = append(lines, fmt.Sprintf("- 描述：%s", strings.TrimSpace(strategy.Description)))
		}
		if len(sourceBits) > 0 {
			lines = append(lines, "- 标的来源："+strings.Join(sourceBits, " | "))
		}
		if len(timeframes) > 0 {
			lines = append(lines, "- K线周期："+strings.Join(timeframes, " / "))
		}
		lines = append(lines, fmt.Sprintf("- 仓位风险：最多持仓 %d，BTC/ETH 最大杠杆 %d，山寨最大杠杆 %d，最低置信度 %d",
			cfg.RiskControl.MaxPositions, cfg.RiskControl.BTCETHMaxLeverage, cfg.RiskControl.AltcoinMaxLeverage, cfg.RiskControl.MinConfidence))
		if len(indicatorBits) > 0 {
			lines = append(lines, "- 已启用指标："+strings.Join(indicatorBits, "、"))
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
	}
	if strings.TrimSpace(strategy.Description) != "" {
		lines = append(lines, fmt.Sprintf("- Description: %s", strings.TrimSpace(strategy.Description)))
	}
	if len(sourceBits) > 0 {
		lines = append(lines, "- Coin source: "+strings.Join(sourceBits, " | "))
	}
	if len(timeframes) > 0 {
		lines = append(lines, "- Timeframes: "+strings.Join(timeframes, " / "))
	}
	lines = append(lines, fmt.Sprintf("- Risk: max positions %d, BTC/ETH max leverage %d, alt max leverage %d, min confidence %d",
		cfg.RiskControl.MaxPositions, cfg.RiskControl.BTCETHMaxLeverage, cfg.RiskControl.AltcoinMaxLeverage, cfg.RiskControl.MinConfidence))
	if len(indicatorBits) > 0 {
		lines = append(lines, "- Enabled indicators: "+strings.Join(indicatorBits, ", "))
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
	if lang == "zh" {
		return fmt.Sprintf("交易所配置“%s”详情：\n- 交易所：%s\n- 已启用：%t\n- API Key：%t\n- Secret：%t\n- Passphrase：%t\n- Testnet：%t",
			defaultIfEmpty(exchange.AccountName, exchange.ID), exchange.ExchangeType, exchange.Enabled, exchange.HasAPIKey, exchange.HasSecretKey, exchange.HasPassphrase, exchange.Testnet), true
	}
	return fmt.Sprintf("Exchange config %q details:\n- Exchange: %s\n- Enabled: %t\n- API key present: %t\n- Secret present: %t\n- Passphrase present: %t\n- Testnet: %t",
		defaultIfEmpty(exchange.AccountName, exchange.ID), exchange.ExchangeType, exchange.Enabled, exchange.HasAPIKey, exchange.HasSecretKey, exchange.HasPassphrase, exchange.Testnet), true
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
	if v := exchangeTypeFromText(text); fieldValue(session, "exchange_type") == "" && v != "" {
		setField(&session, "exchange_type", v)
	}
	if v := extractTraderName(text); fieldValue(session, "account_name") == "" && v != "" {
		setField(&session, "account_name", v)
	}
	exType := fieldValue(session, "exchange_type")
	if actionRequiresSlot("exchange_management", "create", "exchange_type") && exType == "" {
		setSkillDAGStep(&session, "resolve_exchange_type")
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
	setSkillDAGStep(&session, "execute_create")
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
		setSkillDAGStep(&session, "resolve_provider")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建模型配置，我还需要：" + slotDisplayName("provider", lang) + "，例如：OpenAI、DeepSeek、Claude、Gemini。"
		}
		return "To create a model config, I need the provider first, for example OpenAI, DeepSeek, Claude, or Gemini."
	}
	setSkillDAGStep(&session, "execute_create")
	args := map[string]any{
		"action":            "create",
		"provider":          provider,
		"name":              defaultIfEmpty(fieldValue(session, "name"), provider),
		"custom_api_url":    fieldValue(session, "custom_api_url"),
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
		setSkillDAGStep(&session, "resolve_name")
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "要创建策略，我还需要：" + slotDisplayName("name", lang) + "。你可以直接说：创建一个叫“趋势策略A”的策略。"
		}
		return "To create a strategy, I need a strategy name. You can say: create a strategy called 'Trend A'."
	}
	setSkillDAGStep(&session, "execute_create")
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

	if dag, ok := getSkillDAG(skillName, action); ok && len(dag.Steps) > 0 {
		currentStep, _ := currentSkillDAGStep(session)
		if currentStep.ID == "resolve_target" {
			if supportsBulkTargetSelection(skillName, action) && textMeansAllTargets(text) {
				setField(&session, "bulk_scope", "all")
				advanceSkillDAGStep(&session, currentStep.ID)
			} else {
				session.TargetRef = resolveTargetFromText(text, options, session.TargetRef)
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
					reply := "This step needs a target object first. Tell me which one to operate on."
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
		if supportsBulkTargetSelection(skillName, action) && textMeansAllTargets(text) {
			setField(&session, "bulk_scope", "all")
		} else {
			session.TargetRef = resolveTargetFromText(text, options, session.TargetRef)
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
			reply := "I still need you to specify which object to operate on."
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

func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	return value
}
