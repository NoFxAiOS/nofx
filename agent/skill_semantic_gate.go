package agent

import (
	"encoding/json"
	"strings"

	"nofx/store"
)

func (a *Agent) skillVisibleFieldSummary(storeUserID, lang, skillName, action string) string {
	fieldNames := make([]string, 0, 20)
	add := func(field string) {
		field = strings.TrimSpace(field)
		if field == "" {
			return
		}
		for _, existing := range fieldNames {
			if existing == field {
				return
			}
		}
		fieldNames = append(fieldNames, field)
	}

	switch skillName {
	case "model_management":
		if lang == "zh" {
			add("Provider")
		} else {
			add("provider")
		}
		add(displayCatalogFieldName("name", lang))
		for _, field := range manualModelEditableFieldKeys() {
			add(displayCatalogFieldName(field, lang))
		}
	case "exchange_management":
		add(slotDisplayName("exchange_type", lang))
		for _, field := range manualExchangeEditableFieldKeys() {
			add(displayCatalogFieldName(field, lang))
		}
	case "trader_management":
		if strings.TrimSpace(action) == "create" {
			add(slotDisplayName("name", lang))
		}
		for _, field := range manualTraderEditableFieldKeys() {
			add(displayCatalogFieldName(field, lang))
		}
	case "strategy_management":
		add(slotDisplayName("name", lang))
		for _, field := range manualStrategyEditableFieldKeys() {
			add(strategyConfigFieldDisplayName(field, lang))
		}
	}
	if len(fieldNames) == 0 {
		return ""
	}
	prefix := "Visible UI fields"
	if lang == "zh" {
		prefix = "当前可见字段"
	}
	return prefix + "：" + strings.Join(fieldNames, "、")
}

func (a *Agent) strategyTypeForTarget(storeUserID string, target *EntityReference) (string, bool) {
	if a == nil || a.store == nil || target == nil {
		return "", false
	}
	var strategy *store.Strategy
	var err error
	if id := strings.TrimSpace(target.ID); id != "" {
		strategy, err = a.store.Strategy().Get(storeUserID, id)
	} else if name := strings.TrimSpace(target.Name); name != "" {
		strategies, listErr := a.store.Strategy().List(storeUserID)
		if listErr != nil {
			return "", false
		}
		for _, item := range strategies {
			if item != nil && strings.EqualFold(strings.TrimSpace(item.Name), name) {
				strategy = item
				break
			}
		}
	} else {
		return "", false
	}
	if err != nil || strategy == nil {
		return "", false
	}
	cfg := store.GetDefaultStrategyConfig("zh")
	if strings.TrimSpace(strategy.Config) != "" {
		_ = json.Unmarshal([]byte(strategy.Config), &cfg)
	}
	strategyType := strings.TrimSpace(cfg.StrategyType)
	if strategyType == "" {
		strategyType = "ai_trading"
	}
	return strategyType, true
}

func (a *Agent) skillVisibleOptionSummary(storeUserID, lang, skillName, action string) string {
	switch skillName {
	case "model_management":
		return a.modelSkillOptionSummary(lang)
	case "exchange_management":
		return a.exchangeSkillOptionSummary(lang)
	case "trader_management":
		return a.traderSkillOptionSummary(storeUserID, lang)
	case "strategy_management":
		return a.strategySkillOptionSummary(storeUserID, lang)
	default:
		return ""
	}
}

func (a *Agent) modelSkillOptionSummary(lang string) string {
	if lang == "zh" {
		return modelProviderChoicePrompt(lang)
	}
	return modelProviderChoicePrompt(lang)
}

func (a *Agent) exchangeSkillOptionSummary(lang string) string {
	options := enumOptionValues("exchange_management", "exchange_type")
	if len(options) == 0 {
		options = []string{"Binance", "Bybit", "OKX", "Bitget", "Gate", "KuCoin", "Hyperliquid", "Aster", "Lighter", "Indodax"}
	}
	if lang == "zh" {
		return "交易所类型选项：" + strings.Join(options, "、")
	}
	return "Exchange type options: " + strings.Join(options, ", ")
}

func enumOptionValues(skillName, field string) []string {
	def, ok := getSkillDefinition(skillName)
	if !ok {
		return nil
	}
	constraint, ok := def.FieldConstraints[field]
	if !ok || len(constraint.Values) == 0 {
		return nil
	}
	values := make([]string, 0, len(constraint.Values))
	for _, value := range constraint.Values {
		if value == "" {
			continue
		}
		switch value {
		case "openai":
			values = append(values, "OpenAI")
		case "deepseek":
			values = append(values, "DeepSeek")
		case "claude":
			values = append(values, "Claude")
		case "gemini":
			values = append(values, "Gemini")
		case "qwen":
			values = append(values, "Qwen")
		case "kimi":
			values = append(values, "Kimi")
		case "grok":
			values = append(values, "Grok")
		case "minimax":
			values = append(values, "Minimax")
		case "binance":
			values = append(values, "Binance")
		case "okx":
			values = append(values, "OKX")
		case "bybit":
			values = append(values, "Bybit")
		case "gate":
			values = append(values, "Gate")
		case "kucoin":
			values = append(values, "KuCoin")
		case "bitget":
			values = append(values, "Bitget")
		case "hyperliquid":
			values = append(values, "Hyperliquid")
		case "aster":
			values = append(values, "Aster")
		case "lighter":
			values = append(values, "Lighter")
		case "indodax":
			values = append(values, "Indodax")
		default:
			values = append(values, value)
		}
	}
	return values
}

func (a *Agent) traderSkillOptionSummary(storeUserID, lang string) string {
	parts := []string{
		formatSkillOptionList(lang, "可选模型", "Available models", a.loadEnabledModelOptions(storeUserID)),
		formatSkillOptionList(lang, "可选交易所", "Available exchanges", a.loadExchangeOptions(storeUserID)),
		formatSkillOptionList(lang, "可选策略", "Available strategies", a.loadStrategyOptions(storeUserID)),
	}
	return strings.Join(filterNonEmptyStrings(parts), "\n")
}

func (a *Agent) strategySkillOptionSummary(storeUserID, lang string) string {
	parts := []string{
		"",
		formatSkillOptionList(lang, "现有策略", "Existing strategies", a.loadStrategyOptions(storeUserID)),
	}
	sourceOptions := []string{"static", "ai500", "oi_top", "oi_low"}
	if lang == "zh" {
		parts[0] = "选币来源选项：static、ai500、oi_top、oi_low"
	} else {
		parts[0] = "Coin source options: static, ai500, oi_top, oi_low"
	}
	_ = sourceOptions
	return strings.Join(filterNonEmptyStrings(parts), "\n")
}

func formatSkillOptionList(lang, zhPrefix, enPrefix string, options []traderSkillOption) string {
	names := make([]string, 0, len(options))
	for _, option := range options {
		label := strings.TrimSpace(defaultIfEmpty(option.Name, option.ID))
		if label == "" {
			continue
		}
		names = append(names, label)
	}
	if len(names) == 0 {
		if lang == "zh" {
			return zhPrefix + "：暂无"
		}
		return enPrefix + ": none"
	}
	if lang == "zh" {
		return zhPrefix + "：" + strings.Join(names, "、")
	}
	return enPrefix + ": " + strings.Join(names, ", ")
}

func filterNonEmptyStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
