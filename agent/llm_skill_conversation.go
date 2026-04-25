package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

type skillConversationResult struct {
	Ready                           bool              `json:"ready"`
	Question                        string            `json:"question,omitempty"`
	Extracted                       map[string]string `json:"extracted,omitempty"`
	DraftGeneratedFields            map[string]string `json:"draft_generated_fields,omitempty"`
	RequiresConfirmationBeforeApply bool              `json:"requires_confirmation_before_apply,omitempty"`
	UserRejectedFlow                bool              `json:"user_rejected_flow,omitempty"`
	Cancel                          bool              `json:"cancel,omitempty"`
	NeedsClarification              bool              `json:"needs_clarification,omitempty"`
}

// llmSkillConversationDriver replaces rule-based field collection.
// It gives the LLM the skill schema, current collected fields, available resources,
// and the current waiting fields — then lets LLM decide what to ask or whether to proceed.
func (a *Agent) llmSkillConversationDriver(
	ctx context.Context,
	storeUserID string,
	userID int64,
	lang, text string,
	session skillSession,
	availableResources map[string]any,
) skillConversationResult {
	if a == nil || a.aiClient == nil {
		return skillConversationResult{}
	}

	currentFields := currentFieldValuesForSkillSession(session)
	missingFields := missingFieldKeysForSkillSession(session)
	recentCtx := a.buildRecentConversationContext(userID, text)
	skillJSON := loadSkillJSON(session.Name)
	skillContext := buildCurrentSkillExecutionContext(lang, session)
	relevantResources := filterConversationResourcesForSession(session, missingFields, availableResources)
	missingSummary := formatConversationMissingFields(lang, missingFields)
	domainPrimer := buildSkillDomainPrimer(lang, session.Name)

	resourcesJSON, _ := json.Marshal(relevantResources)
	currentFieldsJSON, _ := json.Marshal(currentFields)

	waitingHint := ""
	if len(missingFields) > 0 {
		waitingHint = fmt.Sprintf("\nCurrently waiting for: [%s]. The user's message may be answering one of these fields directly — recognize it even without a keyword prefix.", strings.Join(missingFields, ", "))
	}

	systemPrompt := fmt.Sprintf(`You are the conversation driver for NOFXi skill: %s / %s.
Your job: first understand what the user means in this exact turn, then decide how to continue the current skill action.
You are not a keyword matcher. Infer whether the user is filling a slot, choosing an existing resource, asking to create/enable a dependency, clarifying an earlier answer, or cancelling.

Active skill/action contract:
%s

Skill schema JSON (field constraints and action definitions):
%s

Skill domain primer:
%s

Only the currently relevant resource groups are disclosed below. Use them only when they help resolve the current missing slots. Do not assume omitted resource groups are unavailable globally.
Available resources (each resource includes an ID and display name; return the ID when you can resolve it):
%s

Current collected fields:
%s
%s
Rules:
- Highest-priority safety rule: before extracting any field, first judge whether the user is rejecting, correcting, or denying the current task itself.
- If the current flow is wrong, the user is saying things like "不是交易员，是策略", "弄错了", "不是这个", "I mean the strategy, not the trader", or the core entity has clearly crossed into another domain, do NOT extract any field.
- In those rejection/correction/cross-domain cases, immediately return {"user_rejected_flow":true,"ready":false,"question":"","extracted":{}}.
- Any user-facing question or reply must be simple, clear, and beginner-friendly.
- Treat the user like a trading beginner, not a developer.
- Prefer short sentences and plain language.
- Do not expose internal field names, JSON keys, tool names, or backend terminology to the user unless the user explicitly asks.
- If the user is cancelling, return {"cancel":true}
- If the user answer is ambiguous, return {"ready":false,"needs_clarification":true,"question":"<clarifying question in %s>","extracted":{...any newly extracted fields...}}
- If disclosed resources include an ambiguity/conflict list for the current target, do not repeat a robotic stock phrase. Use the disclosed distinguishing details to ask a natural clarifying question.
- If the user clearly delegates content generation to you (for example: "交给你", "你帮我写", "你自己设计", "you decide", "draft it for me", "所有字段都由你来定", "你帮我配置好"), do not mechanically ask for the same text again.
- In those delegation cases, when the missing slot is a text-like field such as custom_prompt, role_definition, trading_frequency, entry_standards, decision_process, description, or name, you should draft a strong candidate yourself, put that draft into draft_generated_fields, keep ready false if confirmation is still needed, set requires_confirmation_before_apply=true, and use question to show the draft and ask for confirmation.
- When the user delegates ALL fields (e.g. "所有字段都由你来定", "你帮我全部配好", "all fields up to you"), also infer reasonable values for structured fields (such as static_coins, primary_timeframe, selected_timeframes, btceth_max_leverage, altcoin_max_leverage, min_confidence, source_type, etc.) based on the strategy name and stated goal. Put all inferred structured values into draft_generated_fields as well. Present a concise summary of ALL drafted fields in the question and ask for one confirmation before applying.
- If all required fields are collected and there is no ambiguity, return {"ready":true,"extracted":{...all newly resolved fields for this turn...}}
- Otherwise, return {"ready":false,"question":"<natural language next question in %s>","extracted":{...any newly extracted fields...}}
- Extract fields from the user message even without keyword prefixes
- When asking for a field that has available options, list them concisely in the question
- Never ask for fields that are already collected
- For entity refs (exchange, model, strategy): if the user clearly means one option from available resources, use its ID and put it in extracted as exchange_id/ai_model_id/strategy_id
- For target object selection: if the user clearly means one option from available targets, return target_ref_id and target_ref_name
- If the user says to use an existing/current/already-configured resource and there is exactly one usable option in the disclosed resource group, resolve it automatically to that ID
- If multiple disclosed options fit and the user did not disambiguate, ask a clarifying question instead of guessing
- "ready" must stay false if any DAG-required slot is still missing or ambiguous. Current missing field summary: %s
- Distinguish between user-supplied values (put in extracted) and AI-drafted proposal values (put in draft_generated_fields). Do not pretend AI-generated drafts were literal user input.

Return JSON only. No markdown.`,
		session.Name, session.Action,
		defaultIfEmpty(skillContext, "No active contract available."),
		skillJSON,
		defaultIfEmpty(domainPrimer, "No extra domain primer."),
		defaultIfEmpty(string(resourcesJSON), "{}"),
		string(currentFieldsJSON),
		waitingHint,
		lang,
		lang,
		missingSummary,
	)

	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nRecent conversation:\n%s", lang, text, recentCtx)

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
		return skillConversationResult{}
	}

	return parseSkillConversationResult(raw)
}

func filterConversationResourcesForSession(session skillSession, missingFields []string, availableResources map[string]any) map[string]any {
	if len(availableResources) == 0 {
		return nil
	}

	need := map[string]bool{}
	for _, field := range missingFields {
		switch strings.TrimSpace(field) {
		case "target_ref":
			need["targets"] = true
		case "exchange", "exchange_id", "exchange_name":
			need["exchanges"] = true
		case "model", "model_id", "model_name", "ai_model_id":
			need["models"] = true
		case "strategy", "strategy_id", "strategy_name":
			need["strategies"] = true
		}
	}

	if len(need) == 0 {
		switch session.Action {
		case "configure_exchange":
			need["exchanges"] = true
		case "configure_model":
			need["models"] = true
		case "configure_strategy":
			need["strategies"] = true
		}
	}

	if len(need) == 0 {
		return nil
	}

	filtered := make(map[string]any, len(need))
	for key := range need {
		if value, ok := availableResources[key]; ok {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
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

func parseSkillConversationResult(raw string) skillConversationResult {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var out skillConversationResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			json.Unmarshal([]byte(raw[start:end+1]), &out)
		}
	}
	if !out.Cancel && !out.UserRejectedFlow && !out.Ready && out.Question == "" && len(out.Extracted) == 0 && len(out.DraftGeneratedFields) == 0 {
		var flow llmFlowExtractionResult
		if err := json.Unmarshal([]byte(raw), &flow); err == nil {
			if strings.TrimSpace(flow.Intent) == "continue" {
				if len(flow.Fields) > 0 {
					out.Extracted = flow.Fields
				} else if len(flow.Tasks) > 0 {
					out.Extracted = flow.Tasks[0].Fields
				}
				if len(out.Extracted) > 0 {
					out.Ready = true
				}
			}
		}
	}
	out.Question = strings.TrimSpace(out.Question)
	return out
}

// loadSkillJSON returns the raw skill JSON bytes for the given skill name.
func loadSkillJSON(skillName string) string {
	data, err := embeddedSkillDefinitions.ReadFile("skills/" + skillName + ".json")
	if err != nil {
		return "{}"
	}
	return string(data)
}
