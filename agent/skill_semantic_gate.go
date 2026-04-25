package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

type skillSemanticGateDecision struct {
	Decision string `json:"decision,omitempty"`
	Field    string `json:"field,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func (a *Agent) evaluateHardSkillCandidate(ctx context.Context, storeUserID string, userID int64, lang string, session skillSession, skillName, action, text string) skillSemanticGateDecision {
	fallback := fallbackSkillSemanticGate(skillName, action, session, text)
	if a == nil || a.aiClient == nil {
		return fallback
	}

	systemPrompt := `You are the second-stage semantic gate for one NOFXi hard skill.
Return JSON only. No markdown.

Decide exactly one:
- "execute": this request is concrete enough for this hard skill to handle now
- "explain": the user is asking about visible UI fields, options, requirements, or how to fill them
- "planner": this request is too open-ended, strategic, subjective, ambiguous, or not yet UI-ready for this hard skill

Rules:
- Use "execute" when the request maps to the current skill's visible fields, existing entity options, or a normal multi-turn form flow.
- Use "explain" when the user asks what fields/options exist, what a field means, or how to fill it.
- Use "planner" when the user asks for outcome-seeking advice like "make money", "don't lose", "best", "optimize", or otherwise asks you to design the solution before UI-level parameters are clear.
- Use "planner" when semantic readiness is not met: the route points to a skill/action, but the request is still missing core required fields and would otherwise mostly result in a mechanical missing-field error.
- Be conservative. If this hard skill would mostly respond with a misleading missing-field error, choose "planner" instead.

Return JSON:
{"decision":"execute|explain|planner","field":"","reason":""}`

	userPrompt := fmt.Sprintf(
		"Language: %s\nSkill: %s\nAction: %s\nActive skill session JSON: %s\nSemantic readiness summary:\n%s\nVisible field summary:\n%s\nVisible option summary:\n%s\nDomain primer:\n%s\nUser message: %s",
		lang,
		skillName,
		action,
		mustMarshalJSON(session),
		a.skillSemanticReadinessSummary(lang, session, skillName, action, text),
		a.skillVisibleFieldSummary(storeUserID, lang, skillName, action),
		a.skillVisibleOptionSummary(storeUserID, lang, skillName, action),
		buildSkillDomainPrimer(lang, skillName),
		text,
	)

	stageCtx, cancel := withPlannerStageTimeout(ctx, directReplyTimeout)
	defer cancel()
	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	if err != nil {
		return fallback
	}
	if parsed, ok := parseSkillSemanticGateDecision(raw); ok {
		if parsed.Field == "" {
			parsed.Field = detectSkillQuestionField(skillName, text, session)
		}
		return parsed
	}
	return fallback
}

func parseSkillSemanticGateDecision(raw string) (skillSemanticGateDecision, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var out skillSemanticGateDecision
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start || json.Unmarshal([]byte(raw[start:end+1]), &out) != nil {
			return skillSemanticGateDecision{}, false
		}
	}
	out.Decision = strings.TrimSpace(strings.ToLower(out.Decision))
	out.Field = strings.TrimSpace(out.Field)
	out.Reason = strings.TrimSpace(out.Reason)
	switch out.Decision {
	case "execute", "explain", "planner":
		return out, true
	default:
		return skillSemanticGateDecision{}, false
	}
}

func fallbackSkillSemanticGate(skillName, action string, session skillSession, text string) skillSemanticGateDecision {
	if looksLikeExplanationQuestion(text) {
		return skillSemanticGateDecision{
			Decision: "explain",
			Field:    "",
		}
	}
	if strings.TrimSpace(session.Name) == skillName && strings.TrimSpace(session.Action) != "" {
		return skillSemanticGateDecision{Decision: "execute"}
	}
	if isSimpleEntityMutationAction(action) && !hasExplicitSkillCueForSemanticGate(skillName, text) {
		return skillSemanticGateDecision{Decision: "planner", Reason: "requires_llm_intent_resolution"}
	}
	if missing := semanticReadinessMissingSlots(skillName, action, session, text); len(missing) > 0 {
		return skillSemanticGateDecision{Decision: "planner", Reason: "semantic_readiness_missing_core_fields"}
	}
	return skillSemanticGateDecision{Decision: "execute"}
}

func hasExplicitSkillCueForSemanticGate(skillName, text string) bool {
	switch strings.TrimSpace(skillName) {
	case "trader_management":
		return hasExplicitManagementDomainCue(text, "trader") || hasExplicitCreateIntentForDomain(text, "trader")
	case "exchange_management":
		return hasExplicitManagementDomainCue(text, "exchange")
	case "model_management":
		return hasExplicitManagementDomainCue(text, "model")
	case "strategy_management":
		return hasExplicitManagementDomainCue(text, "strategy")
	default:
		return false
	}
}

func semanticReadinessMissingSlots(skillName, action string, session skillSession, text string) []string {
	if strings.TrimSpace(action) == "" {
		return nil
	}
	if strings.TrimSpace(session.Name) == skillName && strings.TrimSpace(session.Action) != "" {
		return nil
	}
	values := map[string]string{}
	missing := missingRequiredActionSlots(skillName, action, values)
	if action != "create" {
		return nil
	}
	if skillName == "strategy_management" {
		return nil
	}
	if skillName == "model_management" {
		coreMissing := missingCoreReadinessSlots([]string{"provider"}, values)
		if len(coreMissing) > 0 {
			return coreMissing
		}
		return nil
	}
	if skillName == "exchange_management" {
		coreMissing := missingCoreReadinessSlots([]string{"exchange_type", "account_name", "api_key", "secret_key"}, values)
		if len(coreMissing) > 0 {
			return coreMissing
		}
		return nil
	}
	if len(missing) == 0 || len(missing) == len(missingRequiredActionSlots(skillName, action, map[string]string{})) {
		if hasExplicitCreateIntentForDomain(text, "trader") || containsAny(strings.ToLower(text), []string{"创建", "新建", "create", "new"}) {
			return nil
		}
	}
	if len(missing) >= 2 {
		return missing
	}
	return nil
}

func missingCoreReadinessSlots(keys []string, values map[string]string) []string {
	missing := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(values[key]) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func (a *Agent) skillSemanticReadinessSummary(lang string, session skillSession, skillName, action, text string) string {
	missing := semanticReadinessMissingSlots(skillName, action, session, text)
	if len(missing) == 0 {
		if lang == "zh" {
			return "当前语义已足够进入该 skill。"
		}
		return "Semantic readiness is sufficient for this skill."
	}
	display := make([]string, 0, len(missing))
	for _, slot := range missing {
		display = append(display, slotDisplayName(slot, lang))
	}
	if lang == "zh" {
		return "当前语义还缺核心字段：" + strings.Join(display, "、") + "。如果直接执行只会变成程序式缺字段提示，应优先走 planner/ask_user。"
	}
	return "Core semantic fields are still missing: " + strings.Join(display, ", ") + ". Prefer planner/ask_user before direct execution."
}

func detectSkillQuestionField(skillName, text string, session skillSession) string {
	return ""
}

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
		add(slotDisplayName("name", lang))
		add(slotDisplayName("exchange", lang))
		add(slotDisplayName("model", lang))
		add(slotDisplayName("strategy", lang))
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
