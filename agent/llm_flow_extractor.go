package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type llmFlowExtractionTask struct {
	Skill  string            `json:"skill,omitempty"`
	Action string            `json:"action,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

type llmFlowExtractionResult struct {
	Intent           string                  `json:"intent,omitempty"`
	TargetSnapshotID string                  `json:"target_snapshot_id,omitempty"`
	InlineSubIntent  string                  `json:"inline_sub_intent,omitempty"`
	Fields           map[string]string       `json:"fields,omitempty"`
	Tasks            []llmFlowExtractionTask `json:"tasks,omitempty"`
	Reason           string                  `json:"reason,omitempty"`
}

type llmFlowFieldSpec struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

func buildActiveFlowExtractionPrompt(lang, flowLabel, flowContext string, text string, recentConversationCtx string, currentRefs any, suspendedSnapshots any, extraSections []string) (string, string) {
	systemPrompt := `You extract structured continuation input for an active NOFXi flow.
Return JSON only. No markdown.

You must decide one of:
- "continue": the user is continuing the current flow and may have supplied fields
- "switch": the user is switching away to another task
- "cancel": the user is cancelling the current flow
- "instant_reply": the user is only chatting / greeting and no task fields should be written

Rules:
- Prefer "continue" only when the message clearly contributes to the current flow.
- Set target_snapshot_id only when the user is clearly referring to one suspended snapshot from Suspended snapshots JSON.
- For greetings, thanks, and casual chat, use "instant_reply".
- Consider Current references JSON and Suspended snapshots JSON when resolving vague references like "那个", "刚才那个", or "前面那个".
- Treat this as semantic slot filling, not keyword copying.
- Users will often speak in natural language, shorthand, colloquial labels, translated labels, or mild misspellings instead of exact schema keys.
- Your job is to decide which allowed canonical field each value belongs to based on the active flow, field descriptions, current missing fields, and conversation context.
- Never require the user to say the exact internal field key.
- In task.fields, always emit the canonical field keys from Allowed field spec JSON, never aliases, paraphrases, or user wording.
- If the user clearly supplied a value for one allowed field, normalize it to that canonical key before returning JSON.`

	sections := []string{
		fmt.Sprintf("Language: %s", lang),
		fmt.Sprintf("Active flow label: %s", flowLabel),
		flowContext,
		fmt.Sprintf("Current references JSON: %s", mustMarshalJSON(currentRefs)),
		fmt.Sprintf("Suspended snapshots JSON: %s", mustMarshalJSON(suspendedSnapshots)),
	}
	sections = append(sections, extraSections...)
	sections = append(sections, fmt.Sprintf("User message: %s", text), fmt.Sprintf("Recent conversation:\n%s", recentConversationCtx))
	return systemPrompt, strings.Join(sections, "\n")
}

func parseLLMFlowExtractionResult(raw string) llmFlowExtractionResult {
	out, ok := parseRawFlowExtractionEnvelope(raw)
	if !ok {
		return llmFlowExtractionResult{}
	}
	switch out.Intent {
	case "continue", "switch", "cancel", "instant_reply":
		return out
	default:
		return llmFlowExtractionResult{}
	}
}

func parseRawFlowExtractionEnvelope(raw string) (llmFlowExtractionResult, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var out llmFlowExtractionResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start || json.Unmarshal([]byte(raw[start:end+1]), &out) != nil {
			return llmFlowExtractionResult{}, false
		}
	}

	out.Intent = strings.TrimSpace(strings.ToLower(out.Intent))
	out.TargetSnapshotID = strings.TrimSpace(out.TargetSnapshotID)
	out.Reason = strings.TrimSpace(out.Reason)
	if len(out.Fields) > 0 {
		clean := make(map[string]string, len(out.Fields))
		for key, value := range out.Fields {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			clean[key] = value
		}
		out.Fields = clean
	}
	cleanTasks := make([]llmFlowExtractionTask, 0, len(out.Tasks))
	for _, task := range out.Tasks {
		task.Skill = strings.TrimSpace(task.Skill)
		task.Action = strings.TrimSpace(task.Action)
		if len(task.Fields) > 0 {
			clean := make(map[string]string, len(task.Fields))
			for key, value := range task.Fields {
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key == "" || value == "" {
					continue
				}
				clean[key] = value
			}
			task.Fields = clean
		}
		cleanTasks = append(cleanTasks, task)
	}
	out.Tasks = cleanTasks
	return out, out.Intent != ""
}

func filterLLMFlowExtractionFields(result llmFlowExtractionResult, specs []llmFlowFieldSpec) llmFlowExtractionResult {
	if len(specs) == 0 {
		result.Fields = nil
		for i := range result.Tasks {
			result.Tasks[i].Fields = nil
		}
		return result
	}
	allowed := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		key := strings.TrimSpace(spec.Key)
		if key != "" {
			allowed[key] = struct{}{}
		}
	}
	filter := func(fields map[string]string) map[string]string {
		if len(fields) == 0 {
			return fields
		}
		clean := make(map[string]string, len(fields))
		for key, value := range fields {
			if _, ok := allowed[key]; !ok {
				continue
			}
			clean[key] = value
		}
		if len(clean) == 0 {
			return nil
		}
		return clean
	}
	result.Fields = filter(result.Fields)
	for i := range result.Tasks {
		result.Tasks[i].Fields = filter(result.Tasks[i].Fields)
	}
	return result
}

func formatConversationMissingFields(lang string, missingFields []string) string {
	if len(missingFields) == 0 {
		if lang == "zh" {
			return "当前没有缺失槽位。"
		}
		return "There are currently no missing slots."
	}
	display := make([]string, 0, len(missingFields))
	for _, field := range missingFields {
		display = append(display, slotDisplayName(field, lang))
	}
	if lang == "zh" {
		return "当前仍缺这些槽位：" + strings.Join(display, "、")
	}
	return "Current missing slots: " + strings.Join(display, ", ")
}

func skillSessionExtractionContext(session skillSession, lang string) (string, []llmFlowFieldSpec, map[string]string, []string) {
	currentStep, _ := currentSkillDAGStep(session)
	fieldSpecs := allowedFieldSpecsForSkillSession(session, lang)
	currentValues := currentFieldValuesForSkillSession(session)
	missing := missingFieldKeysForSkillSession(session)
	summary := fmt.Sprintf("Active flow type: skill_session\nSkill: %s\nAction: %s\nCurrent DAG step: %s", session.Name, session.Action, currentStep.ID)
	return summary, fieldSpecs, currentValues, missing
}

func allowedFieldSpecsForSkillSession(session skillSession, lang string) []llmFlowFieldSpec {
	add := func(out *[]llmFlowFieldSpec, key, description string, required bool) {
		*out = append(*out, llmFlowFieldSpec{Key: key, Description: description, Required: required})
	}
	out := make([]llmFlowFieldSpec, 0, 24)
	if actionRequiresSlot(session.Name, session.Action, "target_ref") {
		add(&out, "target_ref_id", slotDisplayName("target_ref", lang)+" ID", true)
		add(&out, "target_ref_name", slotDisplayName("target_ref", lang), true)
	}
	if supportsBulkTargetSelection(session.Name, session.Action) {
		add(&out, "bulk_scope", "bulk deletion scope, use all only when the user clearly requested all targets", false)
	}
	switch session.Name {
	case "model_management":
		required := map[string]bool{"provider": true}
		if strings.HasPrefix(session.Action, "update") {
			add(&out, "update_field", displayCatalogFieldName("update_field", lang), false)
		}
		add(&out, "provider", slotDisplayName("provider", lang), required["provider"])
		add(&out, "name", displayCatalogFieldName("name", lang), required["name"])
		add(&out, "custom_model_name", displayCatalogFieldName("custom_model_name", lang), required["custom_model_name"])
		add(&out, "api_key", displayCatalogFieldName("api_key", lang), required["api_key"])
		add(&out, "custom_api_url", displayCatalogFieldName("custom_api_url", lang), false)
		add(&out, "enabled", displayCatalogFieldName("enabled", lang), false)
	case "exchange_management":
		required := map[string]bool{"exchange_type": true, "account_name": true}
		if strings.HasPrefix(session.Action, "update") {
			add(&out, "update_field", displayCatalogFieldName("update_field", lang), false)
		}
		add(&out, "exchange_type", slotDisplayName("exchange_type", lang), required["exchange_type"])
		add(&out, "account_name", displayCatalogFieldName("account_name", lang), required["account_name"])
		add(&out, "api_key", displayCatalogFieldName("api_key", lang), false)
		add(&out, "secret_key", displayCatalogFieldName("secret_key", lang), false)
		add(&out, "passphrase", displayCatalogFieldName("passphrase", lang), false)
		add(&out, "testnet", displayCatalogFieldName("testnet", lang), false)
		add(&out, "enabled", displayCatalogFieldName("enabled", lang), false)
		add(&out, "hyperliquid_wallet_addr", displayCatalogFieldName("hyperliquid_wallet_addr", lang), false)
		add(&out, "aster_user", displayCatalogFieldName("aster_user", lang), false)
		add(&out, "aster_signer", displayCatalogFieldName("aster_signer", lang), false)
		add(&out, "aster_private_key", displayCatalogFieldName("aster_private_key", lang), false)
		add(&out, "lighter_wallet_addr", displayCatalogFieldName("lighter_wallet_addr", lang), false)
		add(&out, "lighter_api_key_private_key", displayCatalogFieldName("lighter_api_key_private_key", lang), false)
		add(&out, "lighter_api_key_index", displayCatalogFieldName("lighter_api_key_index", lang), false)
	case "trader_management":
		if strings.HasPrefix(session.Action, "update") {
			add(&out, "update_field", displayCatalogFieldName("update_field", lang), false)
		}
		add(&out, "name", slotDisplayName("name", lang), true)
		add(&out, "exchange_id", slotDisplayName("exchange", lang)+" ID", false)
		add(&out, "exchange_name", slotDisplayName("exchange", lang), true)
		add(&out, "model_id", slotDisplayName("model", lang)+" ID", false)
		add(&out, "model_name", slotDisplayName("model", lang), true)
		add(&out, "strategy_id", slotDisplayName("strategy", lang)+" ID", false)
		add(&out, "strategy_name", slotDisplayName("strategy", lang), true)
		add(&out, "auto_start", "auto_start", false)
		add(&out, "scan_interval_minutes", displayCatalogFieldName("scan_interval_minutes", lang), false)
		add(&out, "is_cross_margin", displayCatalogFieldName("is_cross_margin", lang), false)
		add(&out, "show_in_competition", displayCatalogFieldName("show_in_competition", lang), false)
	case "strategy_management":
		if session.Action == "create" || session.Action == "update_config" {
			add(&out, "config_patch", "Partial StrategyConfig JSON patch inferred from the user's strategy intent. Use this for strategy requirements such as target coins, trend style, short/long bias, indicators, risk, timeframes, and prompt sections.", false)
		}
		if session.Action == "update_prompt" {
			add(&out, "prompt", "Full strategy prompt text to write into the strategy custom prompt.", false)
			add(&out, "custom_prompt", strategyConfigFieldDisplayName("custom_prompt", lang), false)
		}
		if session.Action == "update_config" {
			add(&out, "config_field", strategyConfigFieldDisplayName("config_field", lang), false)
			add(&out, "config_value", strategyConfigFieldDisplayName("config_value", lang), false)
		}
		add(&out, "name", slotDisplayName("name", lang), true)
		for _, key := range manualStrategyEditableFieldKeys() {
			add(&out, key, strategyConfigFieldDisplayName(key, lang), false)
		}
	}
	return out
}

func currentFieldValuesForSkillSession(session skillSession) map[string]string {
	values := map[string]string{}
	for key, value := range session.Fields {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values[key] = trimmed
		}
	}
	if session.TargetRef != nil {
		if session.TargetRef.ID != "" {
			values["target_ref_id"] = session.TargetRef.ID
		}
		if session.TargetRef.Name != "" {
			values["target_ref_name"] = session.TargetRef.Name
		}
	}
	for _, key := range []string{"name", "exchange_id", "exchange_name", "model_id", "model_name", "strategy_id", "strategy_name", "auto_start"} {
		if value := fieldValue(session, key); value != "" {
			values[key] = value
		}
	}
	return values
}

func missingFieldKeysForSkillSession(session skillSession) []string {
	missing := make([]string, 0, 8)
	switch session.Name {
	case "model_management":
		if session.Action != "create" && session.Action != "query_list" && session.Action != "query" && session.Action != "query_detail" && session.TargetRef == nil {
			missing = append(missing, "target_ref")
		}
		if strings.HasPrefix(session.Action, "update") {
			if session.Action == "update_status" {
				if fieldValue(session, "enabled") == "" {
					missing = append(missing, "enabled")
				}
			} else if session.Action == "update_endpoint" {
				if fieldValue(session, "custom_api_url") == "" {
					missing = append(missing, "custom_api_url")
				}
			} else {
				if fieldValue(session, "update_field") == "" {
					missing = append(missing, "update_field")
				}
			}
		} else {
			for _, key := range []string{"provider"} {
				if fieldValue(session, key) == "" {
					missing = append(missing, key)
				}
			}
			if fieldValue(session, "api_key") == "" {
				missing = append(missing, "api_key")
			}
		}
	case "exchange_management":
		if session.Action != "create" && session.Action != "query_list" && session.Action != "query" && session.Action != "query_detail" && session.TargetRef == nil {
			missing = append(missing, "target_ref")
		}
		if strings.HasPrefix(session.Action, "update") {
			if session.Action == "update_status" {
				if fieldValue(session, "enabled") == "" {
					missing = append(missing, "enabled")
				}
			} else {
				if fieldValue(session, "update_field") == "" {
					missing = append(missing, "update_field")
				}
			}
		} else {
			for _, key := range []string{"exchange_type", "account_name", "api_key", "secret_key"} {
				if fieldValue(session, key) == "" {
					missing = append(missing, key)
				}
			}
		}
	case "trader_management":
		if strings.HasPrefix(session.Action, "update") || strings.HasPrefix(session.Action, "configure_") {
			if session.TargetRef == nil {
				missing = append(missing, "target_ref")
			}
			if session.Action == "update_bindings" || session.Action == "configure_strategy" || session.Action == "configure_exchange" || session.Action == "configure_model" {
				switch session.Action {
				case "configure_strategy":
					if fieldValue(session, "strategy_id") == "" {
						missing = append(missing, "strategy_name")
					}
					break
				case "configure_exchange":
					if fieldValue(session, "exchange_id") == "" {
						missing = append(missing, "exchange_name")
					}
					break
				case "configure_model":
					if fieldValue(session, "model_id") == "" {
						missing = append(missing, "model_name")
					}
					break
				}
				if len(missing) > 0 {
					break
				}
				if fieldValue(session, "model_id") == "" && fieldValue(session, "exchange_id") == "" && fieldValue(session, "strategy_id") == "" &&
					fieldValue(session, "model_name") == "" && fieldValue(session, "exchange_name") == "" && fieldValue(session, "strategy_name") == "" {
					missing = append(missing, "update_field")
				}
			} else {
				if fieldValue(session, "update_field") == "" {
					missing = append(missing, "update_field")
				}
			}
		} else {
			if fieldValue(session, "name") == "" {
				missing = append(missing, "name")
			}
			if fieldValue(session, "exchange_id") == "" {
				missing = append(missing, "exchange_name")
			}
			if fieldValue(session, "model_id") == "" {
				missing = append(missing, "model_name")
			}
			if fieldValue(session, "strategy_id") == "" {
				missing = append(missing, "strategy_name")
			}
		}
	case "strategy_management":
		if session.Action != "create" && session.Action != "query_list" && session.Action != "query" && session.Action != "query_detail" && session.TargetRef == nil {
			missing = append(missing, "target_ref")
		}
		switch session.Action {
		case "update_name":
			if fieldValue(session, "name") == "" {
				missing = append(missing, "name")
			}
		case "update_prompt":
			if fieldValue(session, "prompt") == "" && fieldValue(session, "custom_prompt") == "" {
				missing = append(missing, "prompt")
			}
		case "update_config":
			if fieldValue(session, "config_patch") != "" {
				break
			}
			if fieldValue(session, "config_field") == "" {
				missing = append(missing, "config_field")
			} else if fieldValue(session, "config_value") == "" {
				missing = append(missing, "config_value")
			}
		case "create":
			if fieldValue(session, "name") == "" {
				missing = append(missing, "name")
			}
		default:
			missing = append(missing, "update_field")
		}
	}
	sort.Strings(missing)
	return missing
}

func providerExplicitlyMentionedInText(provider, text string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	lower := strings.ToLower(strings.TrimSpace(text))
	if provider == "" || lower == "" {
		return false
	}
	spec, _ := modelProviderSpecByID(provider)
	candidates := []string{provider, strings.ToLower(strings.TrimSpace(spec.DisplayName))}
	switch provider {
	case "blockrun-base":
		candidates = append(candidates, "blockrun", "blockrun base", "base wallet")
	case "blockrun-sol":
		candidates = append(candidates, "blockrun", "blockrun sol", "solana wallet")
	case "claw402":
		candidates = append(candidates, "claw 402")
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && strings.Contains(lower, candidate) {
			return true
		}
	}
	return false
}

func sanitizeLLMExtractionForSkillSession(text string, session skillSession, result llmFlowExtractionResult) llmFlowExtractionResult {
	if session.Name != "model_management" || len(result.Tasks) == 0 {
		return result
	}
	task := result.Tasks[0]
	if task.Fields == nil {
		return result
	}
	if provider := strings.TrimSpace(task.Fields["provider"]); provider != "" && !providerExplicitlyMentionedInText(provider, text) {
		delete(task.Fields, "provider")
		result.Tasks[0] = task
	}
	return result
}

func (a *Agent) applyLLMExtractionToSkillSession(storeUserID string, session *skillSession, result llmFlowExtractionResult, lang string, text string) {
	if session == nil {
		return
	}
	result = sanitizeLLMExtractionForSkillSession(text, *session, result)
	if sub := strings.TrimSpace(result.InlineSubIntent); sub == "create_sub_resource" || sub == "edit_sub_resource" {
		setField(session, "inline_sub_intent", sub)
	}
	if len(result.Tasks) == 0 {
		return
	}
	task := result.Tasks[0]
	if task.Skill != "" && task.Skill != session.Name {
		return
	}
	if task.Action != "" && session.Action != "" && task.Action != session.Action {
		return
	}
	for key, value := range task.Fields {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch key {
		case "target_ref_id":
			if session.TargetRef == nil {
				session.TargetRef = &EntityReference{}
			}
			session.TargetRef.ID = value
			if session.TargetRef.Source == "" {
				session.TargetRef.Source = "llm_extraction"
			}
			continue
		case "target_ref_name":
			if session.TargetRef == nil {
				session.TargetRef = &EntityReference{}
			}
			session.TargetRef.Name = value
			if session.TargetRef.Source == "" {
				session.TargetRef.Source = "llm_extraction"
			}
			continue
		}
		switch session.Name {
		case "model_management":
			if key == "provider" || key == "name" || key == "custom_model_name" || key == "api_key" || key == "custom_api_url" || key == "enabled" || key == "update_field" {
				setField(session, key, value)
			}
		case "exchange_management":
			switch key {
			case "exchange_type", "account_name", "api_key", "secret_key", "passphrase", "testnet", "enabled", "update_field":
				setField(session, key, value)
			}
		case "trader_management":
			switch key {
			case "update_field":
				setField(session, key, value)
			case "name", "exchange_id", "exchange_name", "model_id", "ai_model_id", "model_name", "strategy_id", "strategy_name", "auto_start":
				setField(session, key, value)
			case "scan_interval_minutes", "is_cross_margin", "show_in_competition":
				setField(session, key, value)
			}
		case "strategy_management":
			if key == "name" {
				setField(session, "name", value)
				continue
			}
			if key == "config_field" || key == "config_value" {
				setField(session, key, value)
				continue
			}
			if session.Action == "update_config" {
				setField(session, "config_field", key)
				setField(session, "config_value", value)
				continue
			}
			cfg := unmarshalStrategyCreateDraft(fieldValue(*session, strategyCreateDraftConfigField), lang)
			if err := applyStrategyConfigPatch(&cfg, key, value); err == nil {
				setField(session, strategyCreateDraftConfigField, marshalStrategyCreateDraft(cfg))
			}
		}
	}
}
