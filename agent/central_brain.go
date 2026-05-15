package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
	"nofx/store"
)

// brainDecision is the routing contract between the first-pass LLM and the executor.
type brainDecision struct {
	ThoughtProcess string         `json:"thought_process"`
	ActionType     string         `json:"action_type"`            // CONTINUE_TASK | NEW_TASK | EXPLAIN_KNOWLEDGE | CANCEL_TASK
	TargetSkill    string         `json:"target_skill,omitempty"` // "skill_name:action" for NEW_TASK
	ExtractedData  map[string]any `json:"extracted_data,omitempty"`
	ReplyToUser    string         `json:"reply_to_user"`
}

// activeSessionStepDecision is the per-turn control loop inside one active skill task.
type activeSessionStepDecision struct {
	Route         string         `json:"route"` // ask_user | execute_skill | finish_task | cancel_task
	Reply         string         `json:"reply,omitempty"`
	ExtractedData map[string]any `json:"extracted_data,omitempty"`
}

// tryMinimalBrain is the single entry point replacing tryUnifiedSemanticGateway.
// Intelligence layer: one routing LLM call → active-session loop → legacy skill execution.
func (a *Agent) tryMinimalBrain(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	if a.aiClient == nil {
		return "", false, nil
	}

	activeSession, hasActive := a.getActiveSkillSession(userID)
	recentHistory := a.buildRecentConversationContext(userID, text)
	currentRefs := buildCurrentReferenceSummary(lang, a.semanticCurrentReferences(userID))
	previousAssistantReply := a.currentPendingHintText(userID)

	systemPrompt := buildBrainSystemPrompt(lang)
	userPrompt := buildBrainUserPrompt(lang, text, previousAssistantReply, recentHistory, currentRefs, activeSession, hasActive)

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
		return "", false, nil
	}

	decision, ok := parseBrainDecision(raw)
	if !ok {
		return "", false, nil
	}

	return a.executeBrainDecision(ctx, storeUserID, userID, lang, text, decision, activeSession, hasActive, onEvent)
}

func buildBrainSystemPrompt(lang string) string {
	return prependNOFXiAdvisorPreamble(`You are the central brain of NOFXi. Read the intelligence report and output ONE JSON decision. No markdown, no extra text.

Available action_type values:
- "CONTINUE_TASK": user is continuing the current active task
- "NEW_TASK": user is starting a new task
- "EXPLAIN_KNOWLEDGE": user is asking a knowledge question only
- "CANCEL_TASK": user wants to stop the current task

Available skills (for NEW_TASK target_skill):
trader_management, exchange_management, model_management, strategy_management,
trader_diagnosis, exchange_diagnosis, model_diagnosis, strategy_diagnosis

Available actions:
create, update, update_name, update_bindings, configure_strategy, configure_exchange, configure_model,
update_status, update_endpoint, update_config, update_prompt, delete, start, stop, activate, duplicate,
query_list, query_detail, query_running

Rules:
- Prefer CONTINUE_TASK when there is an active task and the user is still talking about it.
- If the current user message is only a greeting, thanks, acknowledgement, or lightweight social chat like "你好", "hi", "hello", "thanks", "谢谢", "收到", do NOT continue the task.
- For those lightweight social messages, choose EXPLAIN_KNOWLEDGE and reply naturally, or let the task stay suspended.
- Use NEW_TASK only when there is no active task, or the user clearly switches goals/domains.
- Use EXPLAIN_KNOWLEDGE for concept/range/help questions; do not change state. When answering, use ONLY the options/values listed in the active session's missing_required_fields. Never invent field values or provider names.
- For diagnosis, create, update, delete, start, stop, activation, duplication, or historical-performance analysis tasks, never reply only with a future promise such as "I'll do it now", "please wait", "diagnosis is running", or "I'll tell you later". If the next step is execution, choose the corresponding skill/planned execution. If execution is impossible, say exactly what information or data is missing.
- Use CANCEL_TASK for "cancel", "stop", "forget it", "never mind", "算了", "取消".
- Domain guard: if the user says "模型", "AI 模型", or "model" and asks to create or configure one, you must route to model_management, not exchange_management.
- Domain guard: for model_management, the field "provider" means the AI model vendor such as OpenAI, DeepSeek, Claude, Gemini, Qwen, Kimi, Grok, Minimax, claw402, blockrun-base, or blockrun-sol. It never means an exchange like Binance, OKX, Bybit, CFD, forex, or metals.
- extracted_data should include any concrete facts from the user's message.
- When an active session exposes allowed_field_spec_json, extracted_data must use only those canonical keys. Never output aliases, translated labels, or raw user wording as keys.
- If the user clearly means a bulk destructive operation like "删除所有策略" or "全部删除策略", put the intent signal into extracted_data too. Example: {"bulk_scope":"all"}.
- For strategy changes, do not use the generic "strategy_management:update" action. Use "strategy_management:update_name" for renaming, "strategy_management:update_prompt" for prompt changes, or "strategy_management:update_config" for parameter/config changes. For strategy_management:update_config, extracted_data may include a StrategyConfig-shaped "config_patch".
- Current references are context only. Do not turn a current reference into target_ref_id/target_ref_name unless the user explicitly names that object or clearly refers to "this/current/that previous one". If a mutating task has no clear target, ask instead of executing.
- reply_to_user should be concise and in the user's language.
- For NEW_TASK, target_skill format must be "skill_name:action", for example "strategy_management:create".

Output shape (JSON only):
{"thought_process":"...","action_type":"...","target_skill":"...","extracted_data":{},"reply_to_user":"..."}`)
}

func buildBrainUserPrompt(lang, text, previousAssistantReply, recentHistory, currentRefs string, activeSession ActiveSkillSession, hasActive bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Language: %s\nUser message: %s\n\n", lang, text))
	sb.WriteString("=== PREVIOUS ASSISTANT REPLY ===\n")
	sb.WriteString(defaultIfEmpty(strings.TrimSpace(previousAssistantReply), "none"))
	sb.WriteString("\n\n")
	sb.WriteString("=== MANAGEMENT DOMAIN PRIMER ===\n")
	if hasActive {
		sb.WriteString(defaultIfEmpty(buildSkillDomainPrimerForSession(lang, activeToLegacySkillSession(activeSession)), "none"))
	} else {
		sb.WriteString(defaultIfEmpty(buildManagementDomainPrimer(lang), "none"))
	}
	sb.WriteString("\n\n")

	sb.WriteString("=== ACTIVE SESSION ===\n")
	if hasActive {
		sb.WriteString(fmt.Sprintf("skill: %s\naction: %s\n", activeSession.SkillName, activeSession.ActionName))
		if strings.TrimSpace(activeSession.Goal) != "" {
			sb.WriteString(fmt.Sprintf("goal: %s\n", activeSession.Goal))
		}
		if activeSession.PendingHint != nil && strings.TrimSpace(activeSession.PendingHint.Prompt) != "" {
			sb.WriteString(fmt.Sprintf("pending_hint: %s\n", strings.TrimSpace(activeSession.PendingHint.Prompt)))
		}
		if len(activeSession.CollectedFields) > 0 {
			fieldsJSON, _ := json.Marshal(activeSession.CollectedFields)
			sb.WriteString(fmt.Sprintf("collected_fields: %s\n", fieldsJSON))
		}
		if missing := fieldConstraintSummary(activeSession); missing != "" {
			sb.WriteString("missing_required_fields:\n")
			sb.WriteString(missing)
			sb.WriteString("\n")
		}
		fieldSpecs := allowedFieldSpecsForSkillSession(activeToLegacySkillSession(activeSession), lang)
		if len(fieldSpecs) > 0 {
			fieldSpecsJSON, _ := json.Marshal(fieldSpecs)
			sb.WriteString(fmt.Sprintf("allowed_field_spec_json: %s\n", fieldSpecsJSON))
		}
	} else {
		sb.WriteString("none\n")
	}

	sb.WriteString("\n=== CURRENT REFERENCES ===\n")
	sb.WriteString(currentRefs)

	sb.WriteString("\n\n=== RECENT CONVERSATION ===\n")
	sb.WriteString(recentHistory)

	return sb.String()
}

func parseBrainDecision(raw string) (brainDecision, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var d brainDecision
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start {
			return brainDecision{}, false
		}
		if err := json.Unmarshal([]byte(raw[start:end+1]), &d); err != nil {
			return brainDecision{}, false
		}
	}
	d.ActionType = strings.ToUpper(strings.TrimSpace(d.ActionType))
	d.TargetSkill = strings.TrimSpace(d.TargetSkill)
	d.ReplyToUser = strings.TrimSpace(d.ReplyToUser)
	switch d.ActionType {
	case "CONTINUE_TASK", "NEW_TASK", "EXPLAIN_KNOWLEDGE", "CANCEL_TASK":
		return d, true
	default:
		return brainDecision{}, false
	}
}

func parseActiveSessionStepDecision(raw string) (activeSessionStepDecision, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var d activeSessionStepDecision
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start {
			return activeSessionStepDecision{}, false
		}
		if err := json.Unmarshal([]byte(raw[start:end+1]), &d); err != nil {
			return activeSessionStepDecision{}, false
		}
	}
	d.Route = strings.TrimSpace(strings.ToLower(d.Route))
	d.Reply = strings.TrimSpace(d.Reply)
	switch d.Route {
	case "ask_user", "execute_skill", "finish_task", "cancel_task":
		return d, true
	default:
		return activeSessionStepDecision{}, false
	}
}

func (a *Agent) executeBrainDecision(ctx context.Context, storeUserID string, userID int64, lang, text string, d brainDecision, activeSession ActiveSkillSession, hasActive bool, onEvent func(event, data string)) (string, bool, error) {
	switch d.ActionType {
	case "CANCEL_TASK":
		a.clearActiveSkillSession(userID)
		a.clearAnyActiveContext(userID)
		reply := d.ReplyToUser
		if reply == "" {
			if lang == "zh" {
				reply = "已取消当前流程。"
			} else {
				reply = "Cancelled the current flow."
			}
		}
		emitBrainReply(onEvent, reply)
		a.recordSkillInteraction(userID, text, reply)
		return reply, true, nil

	case "EXPLAIN_KNOWLEDGE":
		reply := d.ReplyToUser
		if reply == "" {
			return "", false, nil
		}
		if guarded, blocked := guardUnsupportedAsyncPromise(lang, reply); blocked {
			reply = guarded
		}
		emitBrainReply(onEvent, reply)
		a.recordSkillInteraction(userID, text, reply)
		return reply, true, nil

	case "NEW_TASK":
		skill, action := parseTargetSkill(d.TargetSkill)
		if skill == "" {
			answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
			return answer, true, err
		}
		session := newActiveSkillSession(userID, skill, action)
		session.Goal = strings.TrimSpace(text)
		d.ExtractedData = filterExtractedDataForActiveSession(session, d.ExtractedData, lang)
		markStrategyCreateConfigProgressThisTurn(&session, d.ExtractedData)
		mergeExtractedData(&session, d.ExtractedData)
		return a.driveActiveSession(ctx, storeUserID, userID, lang, text, session, onEvent)

	case "CONTINUE_TASK":
		if !hasActive {
			return "", false, nil
		}
		d.ExtractedData = filterExtractedDataForActiveSession(activeSession, d.ExtractedData, lang)
		markStrategyCreateConfigProgressThisTurn(&activeSession, d.ExtractedData)
		mergeExtractedData(&activeSession, d.ExtractedData)
		return a.driveActiveSession(ctx, storeUserID, userID, lang, text, activeSession, onEvent)

	default:
		return "", false, nil
	}
}

func (a *Agent) driveActiveSession(ctx context.Context, storeUserID string, userID int64, lang, text string, session ActiveSkillSession, onEvent func(event, data string)) (string, bool, error) {
	session = appendActiveSessionLocalHistory(session, "user", text)
	clearActiveSessionPendingHint(&session)

	stepDecision, ok := a.planActiveSessionStep(ctx, storeUserID, userID, lang, text, session)
	if !ok {
		stepDecision = activeSessionStepDecision{}
	}
	configProgressThisTurn := consumeStrategyCreateConfigProgressThisTurn(&session)
	if strategyCreateDecisionHasConfigProgress(session, stepDecision.ExtractedData) {
		configProgressThisTurn = true
	}
	mergeExtractedData(&session, stepDecision.ExtractedData)
	maybeForceStrategyCreateExecutionOnConfirmation(lang, text, &session, &stepDecision)

	if stepDecision.Route == "" {
		if len(missingRequiredFields(session)) > 0 {
			stepDecision.Route = "ask_user"
		} else {
			stepDecision.Route = "execute_skill"
		}
	}
	switch stepDecision.Route {
	case "cancel_task":
		a.clearActiveSkillSession(userID)
		reply := defaultIfEmpty(stepDecision.Reply, "已取消当前流程。")
		if lang != "zh" && strings.TrimSpace(stepDecision.Reply) == "" {
			reply = "Cancelled the current flow."
		}
		emitBrainReply(onEvent, reply)
		a.recordSkillInteraction(userID, text, reply)
		return reply, true, nil

	case "finish_task":
		reply := strings.TrimSpace(stepDecision.Reply)
		if guarded, blocked := guardUnexecutedActiveTaskCompletion(lang, session, reply); blocked {
			session = appendActiveSessionLocalHistory(session, "assistant", guarded)
			setActiveSessionPendingHint(&session, guarded)
			a.saveActiveSkillSession(session)
			emitBrainReply(onEvent, guarded)
			a.recordSkillInteraction(userID, text, guarded)
			return guarded, true, nil
		}
		if guarded, blocked := guardUnsupportedAsyncPromise(lang, reply); blocked {
			session = appendActiveSessionLocalHistory(session, "assistant", guarded)
			setActiveSessionPendingHint(&session, guarded)
			a.saveActiveSkillSession(session)
			emitBrainReply(onEvent, guarded)
			a.recordSkillInteraction(userID, text, guarded)
			return guarded, true, nil
		}
		a.clearActiveSkillSession(userID)
		if reply == "" {
			return "", false, nil
		}
		emitBrainReply(onEvent, reply)
		a.recordSkillInteraction(userID, text, reply)
		return reply, true, nil

	case "ask_user":
		reply := ""
		if guarded, blocked := guardStrategyCreateBeforeFinalConfirmation(lang, session); blocked {
			session.CollectedFields["awaiting_final_confirmation"] = true
			reply = guarded
		}
		if reply == "" && configProgressThisTurn {
			if deterministic, ok := strategyCreateTemplateMissingReply(lang, text, session); ok {
				reply = deterministic
			}
		}
		if reply == "" {
			reply = strings.TrimSpace(stepDecision.Reply)
			if reply == "" {
				reply = a.askForMissingFields(lang, session)
			}
		}
		if guarded, blocked := guardStrategyCreateAINonTemplateQuestion(lang, session, reply); blocked {
			reply = guarded
		}
		if guarded, blocked := guardUnsupportedAsyncPromise(lang, reply); blocked {
			reply = guarded
		}
		if len(missingRequiredFields(session)) == 0 && actionNeedsConfirmation(session.SkillName, session.ActionName) {
			session.LegacyPhase = "await_confirmation"
			session.CollectedFields["phase"] = "await_confirmation"
		}
		session = appendActiveSessionLocalHistory(session, "assistant", reply)
		setActiveSessionPendingHint(&session, reply)
		a.saveActiveSkillSession(session)
		emitBrainReply(onEvent, reply)
		a.recordSkillInteraction(userID, text, reply)
		return reply, true, nil

	case "execute_skill":
		var repairReply string
		var canExecute bool
		session, repairReply, canExecute = a.ensureStrategyCreateExecutableState(ctx, lang, text, session)
		if !canExecute {
			if strategyCreateLooseConfirmationReply(text) {
				repairReply = a.askForMissingFields(lang, session)
			} else {
				repairReply = defaultIfEmpty(repairReply, a.askForMissingFields(lang, session))
			}
			session = appendActiveSessionLocalHistory(session, "assistant", repairReply)
			setActiveSessionPendingHint(&session, repairReply)
			a.saveActiveSkillSession(session)
			emitBrainReply(onEvent, repairReply)
			a.recordSkillInteraction(userID, text, repairReply)
			return repairReply, true, nil
		}
		if !strategyCreateLooseConfirmationReply(text) {
			if guarded, blocked := guardStrategyCreateBeforeFinalConfirmation(lang, session); blocked {
				session.CollectedFields["awaiting_final_confirmation"] = true
				session = appendActiveSessionLocalHistory(session, "assistant", guarded)
				setActiveSessionPendingHint(&session, guarded)
				a.saveActiveSkillSession(session)
				emitBrainReply(onEvent, guarded)
				a.recordSkillInteraction(userID, text, guarded)
				return guarded, true, nil
			}
		}
		outcome, nextSession, pending, ok := a.executeActiveSkillSession(storeUserID, userID, lang, text, session)
		if !ok {
			return "", false, nil
		}
		if pending {
			reply := strings.TrimSpace(outcome.UserMessage)
			if reply == "" {
				reply = a.askForMissingFields(lang, nextSession)
			}
			nextSession = appendActiveSessionLocalHistory(nextSession, "assistant", reply)
			setActiveSessionPendingHint(&nextSession, reply)
			a.saveActiveSkillSession(nextSession)
			emitBrainReply(onEvent, reply)
			a.recordSkillInteraction(userID, text, reply)
			return reply, true, nil
		}

		if shouldTrustDeterministicSkillReply(outcome) {
			answer := strings.TrimSpace(outcome.UserMessage)
			if answer == "" {
				return "", false, nil
			}
			a.clearActiveSkillSession(userID)
			emitBrainReply(onEvent, answer)
			a.recordSkillInteraction(userID, text, answer)
			return answer, true, nil
		}

		review, err := a.reviewTaskCompletion(ctx, userID, lang, text, outcome)
		if err != nil {
			review = taskReviewDecision{Route: "complete", Answer: outcome.UserMessage}
		}
		answer := strings.TrimSpace(review.Answer)
		if answer == "" {
			answer = strings.TrimSpace(outcome.UserMessage)
		}
		if review.Route == "replan" && answer == "" {
			answer = outcome.UserMessage
		}
		if answer == "" {
			return "", false, nil
		}
		a.clearActiveSkillSession(userID)
		emitBrainReply(onEvent, answer)
		a.recordSkillInteraction(userID, text, answer)
		return answer, true, nil

	default:
		return "", false, nil
	}
}

func strategyCreateLooseConfirmationReply(text string) bool {
	if strategyCreateConfirmationReply(text) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "确认创建") ||
		strings.Contains(lower, "按这个创建") ||
		strings.Contains(lower, "confirm create")
}

func (a *Agent) ensureStrategyCreateExecutableState(ctx context.Context, lang, text string, session ActiveSkillSession) (ActiveSkillSession, string, bool) {
	if session.SkillName != "strategy_management" || session.ActionName != "create" {
		return session, "", true
	}
	if strategyCreateSessionReady(lang, session) {
		return session, "", true
	}
	if a.aiClient == nil {
		return session, "", true
	}

	legacy := activeToLegacySkillSession(session)
	collectedJSON, _ := json.Marshal(session.CollectedFields)
	fieldSpecsJSON, _ := json.Marshal(allowedFieldSpecsForSkillSession(legacy, lang))
	history := formatActiveSessionLocalHistory(session.LocalHistory)
	if history == "" {
		history = "(empty)"
	}
	systemPrompt := prependNOFXiAdvisorPreamble(`You repair structured state for one active NOFXi strategy creation task.
Return JSON only.

Rules:
- Think from the current user message, previous assistant proposal, and active history.
- If the previous assistant already asked the user to confirm a concrete creation proposal in chat and the current user confirms it, set extracted_data.awaiting_final_confirmation=true too.
- For each user message, decide how it relates to the currently selected strategy product template.
- If the message provides explicit values, corrections, preferences, constraints, or asks you to recommend/design, translate only the determinable template fields into extracted_data.config_patch as a StrategyConfig-shaped JSON patch.
- If the message is only a question, explanation request, greeting, or unrelated text, answer it without inventing config_patch.
- Do not silently fill missing fields when the user has not authorized it. But if the user explicitly says things like "你帮我定 / 你推荐 / 按稳健高频设计 / 其他你定", that is authorization for the Agent to design the remaining fields. In that case you must produce a recommended config_patch based on the current strategy template and field limits, and explain which values came from the user versus which values are Agent recommendations.
- The product editor template is the source of truth. Use only fields from the selected product template.
- If the user switches strategy type, set extracted_data.strategy_type to the new type and discard fields from the previous type. Keep only shared fields such as name/description/publish settings.
- In NOFXi product schema, AI500/OI Top/OI Low/static coin-source requests are ai_trading, not grid_trading.
- Strategy creation is chat-executable. Do not tell the user to click a web/app button, open a page, or manually create it elsewhere.
- Do not claim the strategy was created and do not promise future execution ("马上创建", "正在创建", "稍后通知"). This step only repairs state or asks for missing information.
- When the current user message is a confirmation, prefer route="ready" whenever the structured template can be repaired. If it cannot be repaired, route="ask_user" with only the missing fields; never reply that you are about to create it.
- If the template is still incomplete after applying determinable config_patch, ask one natural follow-up question or explain the missing fields.

Return shape:
{"route":"ready|ask_user","reply":"","extracted_data":{}}`)
	userPrompt := fmt.Sprintf("Language: %s\nCurrent user message: %s\n\nCurrent collected fields JSON:\n%s\n\nAllowed field spec JSON:\n%s\n\nActive task history:\n%s", lang, text, string(collectedJSON), string(fieldSpecsJSON), history)

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
		return session, "", false
	}
	decision, ok := parseStrategyCreateStateRepairDecision(raw)
	if !ok {
		return session, "", false
	}
	decision.ExtractedData = filterExtractedDataForActiveSession(session, decision.ExtractedData, lang)
	mergeExtractedData(&session, decision.ExtractedData)
	if decision.Route == "ask_user" {
		return session, strings.TrimSpace(decision.Reply), false
	}
	if strategyCreateSessionReady(lang, session) {
		return session, strings.TrimSpace(decision.Reply), true
	}
	return session, strings.TrimSpace(decision.Reply), false
}

type strategyCreateStateRepairDecision struct {
	Route         string         `json:"route"`
	Reply         string         `json:"reply,omitempty"`
	ExtractedData map[string]any `json:"extracted_data,omitempty"`
}

func parseStrategyCreateStateRepairDecision(raw string) (strategyCreateStateRepairDecision, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var d strategyCreateStateRepairDecision
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start {
			return strategyCreateStateRepairDecision{}, false
		}
		if err := json.Unmarshal([]byte(raw[start:end+1]), &d); err != nil {
			return strategyCreateStateRepairDecision{}, false
		}
	}
	d.Route = strings.ToLower(strings.TrimSpace(d.Route))
	d.Reply = strings.TrimSpace(d.Reply)
	switch d.Route {
	case "ready", "ask_user":
		return d, true
	default:
		return strategyCreateStateRepairDecision{}, false
	}
}

func strategyCreateSessionReady(lang string, session ActiveSkillSession) bool {
	legacy := activeToLegacySkillSession(session)
	cfg, _, _, err := strategyCreateConfigFromSession(legacy, lang)
	if err != nil {
		return false
	}
	ready, _ := strategyCreateConfigReady(legacy, cfg, "")
	return ready
}

func strategyCreateDecisionHasConfigProgress(session ActiveSkillSession, data map[string]any) bool {
	if session.SkillName != "strategy_management" || session.ActionName != "create" || len(data) == 0 {
		return false
	}
	patch, ok := data[strategyCreateConfigPatchField]
	if !ok {
		return false
	}
	sanitized := sanitizeStrategyCreateConfigPatchForType(patch, defaultIfEmpty(strategyTypeFromExtractedData(data), strategyTypeFromCollectedFields(session.CollectedFields)))
	return len(sanitized) > 0
}

const strategyCreateConfigProgressThisTurnField = "__strategy_create_config_progress_this_turn"

func markStrategyCreateConfigProgressThisTurn(session *ActiveSkillSession, data map[string]any) {
	if session == nil || !strategyCreateDecisionHasConfigProgress(*session, data) {
		return
	}
	if session.CollectedFields == nil {
		session.CollectedFields = map[string]any{}
	}
	session.CollectedFields[strategyCreateConfigProgressThisTurnField] = true
}

func consumeStrategyCreateConfigProgressThisTurn(session *ActiveSkillSession) bool {
	if session == nil || session.CollectedFields == nil {
		return false
	}
	progress := activeFieldBool(session.CollectedFields[strategyCreateConfigProgressThisTurnField])
	delete(session.CollectedFields, strategyCreateConfigProgressThisTurnField)
	return progress
}

func maybeForceStrategyCreateExecutionOnConfirmation(lang, text string, session *ActiveSkillSession, decision *activeSessionStepDecision) bool {
	if session == nil || decision == nil {
		return false
	}
	if session.SkillName != "strategy_management" || session.ActionName != "create" {
		return false
	}
	if !strategyCreateLooseConfirmationReply(text) {
		return false
	}
	if !strategyCreateSessionReady(lang, *session) {
		return false
	}
	if session.CollectedFields == nil {
		session.CollectedFields = map[string]any{}
	}
	session.CollectedFields["awaiting_final_confirmation"] = true
	decision.Route = "execute_skill"
	decision.Reply = ""
	return true
}

func (a *Agent) activeStrategyCreateSession(userID int64) (ActiveSkillSession, bool) {
	if session, ok := a.getActiveSkillSession(userID); ok && session.SkillName == "strategy_management" && session.ActionName == "create" {
		return session, true
	}
	if legacy := a.getSkillSession(userID); legacy.Name == "strategy_management" && legacy.Action == "create" {
		return activeSessionFromLegacy(ActiveSkillSession{
			UserID:     userID,
			SkillName:  "strategy_management",
			ActionName: "create",
		}, legacy), true
	}
	return ActiveSkillSession{}, false
}

func guardStrategyCreateBeforeFinalConfirmation(lang string, session ActiveSkillSession) (string, bool) {
	if session.SkillName != "strategy_management" || session.ActionName != "create" {
		return "", false
	}
	if activeFieldBool(session.CollectedFields["awaiting_final_confirmation"]) && strategyCreateHasPriorConfirmationPrompt(session) {
		return "", false
	}
	legacy := activeToLegacySkillSession(session)
	cfg, _, _, err := strategyCreateConfigFromSession(legacy, lang)
	if err != nil {
		return "", false
	}
	if ready, _ := strategyCreateConfigReady(legacy, cfg, ""); !ready {
		return "", false
	}
	return formatStrategyCreateFinalConfirmation(lang, legacy, cfg), true
}

func strategyCreateTemplateMissingReply(lang, text string, session ActiveSkillSession) (string, bool) {
	if session.SkillName != "strategy_management" || session.ActionName != "create" {
		return "", false
	}
	legacy := activeToLegacySkillSession(session)
	cfg, _, _, err := strategyCreateConfigFromSession(legacy, lang)
	if err != nil {
		return "", false
	}
	ready, missingKind := strategyCreateConfigReady(legacy, cfg, "")
	if ready || strings.TrimSpace(missingKind) == "" {
		return "", false
	}
	if reply := formatStrategyCreateFieldOptionsReply(lang, text, missingKind); reply != "" {
		return reply, true
	}
	return formatStrategyCreateConfigNeeded(lang, missingKind), true
}

func strategyCreateHasPriorConfirmationPrompt(session ActiveSkillSession) bool {
	for i := len(session.LocalHistory) - 1; i >= 0; i-- {
		msg := session.LocalHistory[i]
		if msg.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		lower := strings.ToLower(content)
		return strings.Contains(content, "确认创建") ||
			strings.Contains(content, "确认后我再创建") ||
			strings.Contains(content, "配置整理好了") ||
			strings.Contains(content, "请确认是否按以上设置创建") ||
			strings.Contains(content, "如果没问题，我就执行创建") ||
			strings.Contains(content, "是否按以上设置创建") ||
			strings.Contains(lower, "confirm") ||
			strings.Contains(lower, "create it")
	}
	return false
}

func activeFieldBool(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func guardUnexecutedActiveTaskCompletion(lang string, session ActiveSkillSession, reply string) (string, bool) {
	if !isMutatingActiveTask(session) || !looksLikeCompletionClaim(reply) {
		return "", false
	}
	if lang == "zh" {
		if session.SkillName == "strategy_management" {
			return "还没有真正创建到策略列表里。刚才只是整理/确认配置方案；需要继续的话，我会先用结构化配置调用策略创建工具，再基于真实结果回复。", true
		}
		return "还没有真正执行完成。刚才只是继续当前配置流程；需要实际执行时，我会调用对应工具后再基于真实结果回复。", true
	}
	return "It has not actually been executed yet. The previous step only prepared or confirmed the draft; I need to run the structured tool before claiming completion.", true
}

func guardStrategyCreateAINonTemplateQuestion(lang string, session ActiveSkillSession, reply string) (string, bool) {
	if session.SkillName != "strategy_management" || session.ActionName != "create" {
		return "", false
	}
	if strategyTypeFromCollectedFields(session.CollectedFields) != "ai_trading" {
		return "", false
	}
	lower := strings.ToLower(strings.TrimSpace(reply))
	if lower == "" {
		return "", false
	}
	if !containsAny(lower, []string{
		"投入多少", "投入资金", "总投入", "固定投入", "每笔交易", "每笔固定", "100u", "500u", "1000u",
		"止损", "日亏损", "最大回撤",
		"investment amount", "capital", "fixed amount", "per-trade", "stop loss", "daily loss", "max drawdown",
	}) {
		return "", false
	}
	legacy := activeToLegacySkillSession(session)
	cfg, _, _, err := strategyCreateConfigFromSession(legacy, lang)
	if err != nil {
		return "", false
	}
	_, missingKind := strategyCreateConfigReady(legacy, cfg, "")
	if strings.TrimSpace(missingKind) == "" {
		return "", false
	}
	if lang == "zh" {
		return "这些不是 AI 策略创建模板里的字段。我会继续按 AI 策略模板填写；当前还需要围绕选币来源、周期、杠杆、置信度、盈亏比、交易频率和开仓标准来确定配置。你也可以直接说“全部你定，按稳健/高频/激进”。", true
	}
	return "Those are not fields in the AI strategy creation template. I will continue using the AI strategy template: coin source, timeframes, leverage, confidence, risk/reward, trading frequency, and entry standards.", true
}

func guardUnsupportedAsyncPromise(lang, reply string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(reply))
	if lower == "" {
		return "", false
	}
	promiseSignals := []string{
		"请稍等", "稍等片刻", "再稍等", "马上", "稍后", "立刻告诉", "数据一出来", "一两分钟",
		"还在进行", "正在进行", "正在为", "正在帮", "一直在帮", "诊断中", "分析中",
		"please wait", "give me a moment", "still running", "i'll let you know", "i will let you know",
	}
	taskSignals := []string{
		"诊断", "分析", "历史交易", "历史表现", "亏损原因", "创建", "修改", "删除", "启动", "停止",
		"diagnos", "analyz", "history", "performance", "loss", "create", "update", "delete", "start", "stop",
	}
	if !containsAny(lower, promiseSignals) || !containsAny(lower, taskSignals) {
		return "", false
	}
	if lang == "zh" {
		return "我需要纠正一下：我没有后台异步任务在运行，也不会自动推送后续结果。诊断/创建/修改/启动这类任务必须在当前回复里实际执行并给出真实结果；如果还不能执行，我应该直接说明缺少哪个对象、时间范围或数据。", true
	}
	return "I need to correct that: there is no background task running, and I will not automatically push a later result. Diagnosis/create/update/start tasks must actually execute and return a real result in the current response; if execution is not possible, I should state which target, range, or data is missing.", true
}

func isMutatingActiveTask(session ActiveSkillSession) bool {
	if strings.TrimSpace(session.SkillName) == "" {
		return false
	}
	switch strings.TrimSpace(session.ActionName) {
	case "create", "update", "update_name", "update_bindings", "configure_strategy", "configure_exchange", "configure_model", "update_status", "update_endpoint", "update_config", "update_prompt", "delete", "start", "stop", "activate", "duplicate":
		return true
	default:
		return false
	}
}

func looksLikeCompletionClaim(reply string) bool {
	lower := strings.ToLower(strings.TrimSpace(reply))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"已创建", "创建好了", "创建好", "已经创建", "已更新", "更新好了", "已修改", "已删除", "已启动", "已停止", "已激活", "已复制", "已经完成", "已完成",
		"created", "has been created", "updated", "deleted", "started", "stopped", "activated", "duplicated", "completed",
	})
}

func (a *Agent) planActiveSessionStep(ctx context.Context, storeUserID string, userID int64, lang, text string, session ActiveSkillSession) (activeSessionStepDecision, bool) {
	if a.aiClient == nil {
		return activeSessionStepDecision{}, false
	}

	legacy := activeToLegacySkillSession(session)
	resources := a.buildActiveSessionResources(storeUserID, legacy)
	resourcesJSON, _ := json.Marshal(resources)
	collectedJSON, _ := json.Marshal(session.CollectedFields)
	missingSummary := formatConversationMissingFields(lang, missingRequiredFieldsForBrain(session))
	fieldSpecs := allowedFieldSpecsForSkillSession(legacy, lang)
	fieldSpecsJSON, _ := json.Marshal(fieldSpecs)
	localHistory := formatActiveSessionLocalHistory(session.LocalHistory)
	if localHistory == "" {
		localHistory = "(empty)"
	}
	previousAssistantReply := a.currentPendingHintText(userID)

	domainPrimer := buildSkillDomainPrimerForSession(lang, legacy)
	specificRules := activeSessionSpecificRules(legacy)

	systemPrompt := prependNOFXiAdvisorPreamble(fmt.Sprintf(`You are the active-task orchestration loop for NOFXi.
You decide the NEXT step for exactly one active task. Return JSON only.

Active task:
- skill: %s
- action: %s
- goal: %s

Current collected fields:
%s

Current missing field summary:
%s

Relevant disclosed resources:
%s

Allowed field spec JSON:
%s

Domain knowledge:
%s

Rules:
- Your job is to decide the next move, not to explain internal schema names.
- Read the previous assistant reply carefully. The user's short answer may be replying to that exact proposal, confirmation request, or question.
- Use contextual memory from the active task history and current references.
- Prefer "execute_skill" when the user has already given enough information to act.
- Prefer "ask_user" only when something truly necessary is still missing.
%s
- For any mutating task, a reply that only promises future execution ("now I will create/update/start it", "result soon") is not a valid finish_task or ask_user outcome. If execution is the next step, choose execute_skill.
- For diagnosis, create, update, delete, start, stop, query/history, and performance-analysis tasks, never answer with only "马上处理 / 请稍等 / 诊断中 / I'll tell you later". NOFXi has no background chat job that will later push an answer. Choose execute_skill/planned_agent when enough information exists; otherwise ask for the missing target/range/data.
- Never choose finish_task for an unfinished mutating active task by claiming it was created/updated/deleted/started/stopped. Only a real skill/tool execution outcome can support that claim.
- If the user says they do not understand the current form, choices, or required information, choose "ask_user" and explain the current pending question in plain language before asking the next easiest question. Cover the relevant concepts from the previous assistant reply; do not collapse the answer to only the first missing field.
- For beginner/confusion replies, give a safe recommended path when the domain supports one, but do not execute or create anything unless the user confirms after the explanation.
- If the current message is only a greeting, thanks, acknowledgement, or small talk and does not add task information, do NOT continue task execution. Choose "ask_user" only if you need to gently restate what is pending; otherwise choose "finish_task" with a short social reply.
- Ask naturally. Do not say raw slot names like target_ref unless the user explicitly asks for internal details.
- If the user clearly means a bulk destructive operation like "删除所有策略", "全部删除策略", "all strategies", set extracted_data to {"bulk_scope":"all"} and choose "execute_skill". Do not ask for target_ref.
- If the user refers to a specific object from disclosed targets, set target_ref_id and target_ref_name when you can resolve it.
- Current references are context for reasoning only. Do not copy a current reference into target_ref_id/target_ref_name unless the user explicitly refers to that object by name/id or clearly says "this/current/that previous one". If the target is not clear, ask instead of executing.
- For trader bindings, exchange/model/strategy must resolve to an ID from Relevant disclosed resources before execution. Never invent a resource name or use a generic venue type like Binance/OKX as the bound exchange unless it appears as an actual disclosed resource.
- If there are multiple targets and the user did not disambiguate, ask a natural question with the available names.
- If the current user message answers a missing field directly, extract it and continue.
- extracted_data must use only canonical keys from Allowed field spec JSON. Never output aliases, translated labels, or raw user wording as keys.
- If a user-provided value does not fit one of those canonical keys, omit it; never create another key.
- If this task is already done and the best next step is just to tell the user the result, choose "finish_task".
- If the user aborts the task, choose "cancel_task".

Return JSON with this exact shape:
{"route":"ask_user|execute_skill|finish_task|cancel_task","reply":"","extracted_data":{}}`,
		session.SkillName,
		session.ActionName,
		defaultIfEmpty(session.Goal, "(not set)"),
		defaultIfEmpty(string(collectedJSON), "{}"),
		missingSummary,
		defaultIfEmpty(string(resourcesJSON), "{}"),
		defaultIfEmpty(string(fieldSpecsJSON), "[]"),
		defaultIfEmpty(domainPrimer, "(none)"),
		specificRules,
	))
	userPrompt := fmt.Sprintf("Language: %s\nCurrent user message: %s\n\nPrevious assistant reply:\n%s\n\nActive task local history:\n%s\n", lang, text, defaultIfEmpty(previousAssistantReply, "(empty)"), localHistory)

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
		return activeSessionStepDecision{}, false
	}
	decision, ok := parseActiveSessionStepDecision(raw)
	if !ok {
		return activeSessionStepDecision{}, false
	}
	decision.ExtractedData = filterExtractedDataForActiveSession(session, decision.ExtractedData, lang)
	return decision, true
}

func activeSessionSpecificRules(session skillSession) string {
	if session.Name != "strategy_management" {
		return ""
	}
	switch session.Action {
	case "create", "update_config":
		return strings.Join([]string{
			"- For strategy_management:create/update_config, the selected product editor template is the only schema. Write values only through extracted_data.config_patch, using the current type branch only: ai_trading => strategy_type + ai_config + publish_config; grid_trading => strategy_type + grid_config + publish_config.",
			"- For strategy_management:create/update_config, config_patch values must be product schema raw values, not user-facing labels. Examples: source_type=\"ai500\" not \"AI500\"; strategy_type=\"ai_trading\" not \"AI 策略\"; selected_timeframes=[\"1m\",\"5m\",\"15m\"] not a JSON string.",
			"- For strategy_management:create, AI500/OI Top/OI Low/static coin-source requests imply strategy_type=\"ai_trading\". Do not leave strategy type ambiguous in that case.",
			"- For strategy_management:create/update_config, judge the user's natural-language intent. Explicit values, corrections, constraints, preferences, or requests to recommend/design must become config_patch for every determinable current-template field; pure questions/greetings/acknowledgements must not invent config_patch.",
			"- For strategy_management:create, the Relevant disclosed resources include product_default_template and current_missing_template_fields. Treat product_default_template as the product editor's default template and field shape.",
			"- For strategy_management:create, do not ask for or present fields listed in product_default_template.non_fields. They are not part of the selected product editor template.",
			"- For strategy_management:create, when the user states a strategy style/preference or authorizes the Agent to recommend/design remaining settings, use product_default_template as the base, adjust it to the user's stated preference, and output config_patch that fills every determinable missing template field. Do not ask the user to restate fields that can be responsibly selected from the default template.",
			"- For grid_trading create, if the user authorizes the Agent to choose/recommend remaining settings, set grid_config.use_atr_bounds=true for the price range unless the user explicitly gives manual upper_price/lower_price. Never invent current market prices or say a symbol is currently near a price without a fresh market-data tool result.",
			"- For strategy_management:create, any user-facing strategy plan must be generated from the post-merge structured config built from config_patch and the current strategy type. Do not display fields that would be filtered out or belong to the other strategy type.",
			"- For strategy_management:create, once complete, ask for one chat confirmation with awaiting_final_confirmation=true; after confirmation execute synchronously with empty reply and only report success after the tool returns.",
		}, "\n")
	default:
		return ""
	}
}

func (a *Agent) executeActiveSkillSession(storeUserID string, userID int64, lang, text string, session ActiveSkillSession) (skillOutcome, ActiveSkillSession, bool, bool) {
	legacy := activeToLegacySkillSession(session)
	a.saveSkillSession(userID, legacy)
	answer, handled := a.dispatchBridgedSkillSession(storeUserID, userID, lang, text, legacy)
	if !handled {
		a.clearSkillSession(userID)
		return skillOutcome{}, ActiveSkillSession{}, false, false
	}

	updatedLegacy := a.getSkillSession(userID)
	a.clearSkillSession(userID)
	outcome := inferSkillOutcome(session.SkillName, session.ActionName, answer, updatedLegacy, skillDataForAction(storeUserID, session.SkillName, session.ActionName, a))
	if updatedLegacy.Name != "" {
		nextSession := activeSessionFromLegacy(session, updatedLegacy)
		return outcome, nextSession, true, true
	}
	return outcome, ActiveSkillSession{}, false, true
}

func shouldTrustDeterministicSkillReply(outcome skillOutcome) bool {
	if outcome.Status != skillOutcomeSuccess || !outcome.GoalAchieved {
		return false
	}
	switch outcome.Skill {
	case "strategy_management", "trader_management", "model_management", "exchange_management":
		switch outcome.Action {
		case "create", "update", "update_name", "update_bindings", "configure_strategy", "configure_exchange", "configure_model", "update_status", "update_endpoint", "update_config", "update_prompt", "delete", "start", "stop", "activate", "duplicate":
			return true
		}
	}
	return false
}

func (a *Agent) askForMissingFields(lang string, session ActiveSkillSession) string {
	missing := missingRequiredFieldsForBrain(session)
	if len(missing) == 0 {
		if lang == "zh" {
			return "还需要一点信息，我再继续。"
		}
		return "I need a bit more information before I continue."
	}

	if session.SkillName == "model_management" && session.ActionName == "create" {
		for _, field := range missing {
			if field == "provider" {
				return modelProviderChoicePrompt(lang)
			}
		}
	}

	def, ok := getSkillDefinition(session.SkillName)
	if !ok {
		if lang == "zh" {
			return "还需要更多信息，请继续。"
		}
		return "I need a bit more information to continue."
	}

	labels := make([]string, 0, len(missing))
	for _, field := range missing {
		label := slotDisplayName(field, lang)
		if constraint, ok := def.FieldConstraints[field]; ok {
			desc := strings.TrimSpace(constraint.Description)
			if len(constraint.Values) > 0 {
				desc = strings.Join(constraint.Values, " / ")
			}
			if desc != "" {
				label = fmt.Sprintf("%s（%s）", label, desc)
			}
		}
		labels = append(labels, label)
	}

	if lang == "zh" {
		return "还差一点信息，我才能继续：" + strings.Join(labels, "、") + "。"
	}
	return "I still need a bit more information before I can continue: " + strings.Join(labels, ", ") + "."
}

func activeToLegacySkillSession(s ActiveSkillSession) skillSession {
	legacy := skillSession{
		Name:   s.SkillName,
		Action: s.ActionName,
		Phase:  defaultIfEmpty(strings.TrimSpace(s.LegacyPhase), "executing"),
		Fields: make(map[string]string),
	}
	for k, v := range s.CollectedFields {
		str := activeFieldString(v)
		if str == "" || str == "<nil>" {
			continue
		}
		switch k {
		case "phase":
			legacy.Phase = str
		case "target_ref_id":
			ensureTargetRef(&legacy)
			legacy.TargetRef.ID = str
		case "target_ref_name":
			ensureTargetRef(&legacy)
			legacy.TargetRef.Name = str
		case "target_ref":
			ensureTargetRef(&legacy)
			if legacy.TargetRef.ID == "" {
				legacy.TargetRef.ID = str
			}
			if legacy.TargetRef.Name == "" {
				legacy.TargetRef.Name = str
			}
		default:
			legacy.Fields[k] = str
		}
	}
	if s.SkillName == "strategy_management" && s.ActionName == "create" && legacy.Fields["name"] == "" {
		for i := len(s.LocalHistory) - 1; i > 0; i-- {
			msg := s.LocalHistory[i]
			if msg.Role != "user" || !activeHistoryMessageAsksStrategyName(s.LocalHistory[i-1].Content) {
				continue
			}
			if inferred := inferStandaloneStrategyName(msg.Content); inferred != "" {
				legacy.Fields["name"] = inferred
				break
			}
		}
	}
	return legacy
}

func activeFieldString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case map[string]any, []any, map[string]string, []string:
		raw, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(raw))
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func activeSessionFromLegacy(base ActiveSkillSession, legacy skillSession) ActiveSkillSession {
	next := base
	next.LegacyPhase = strings.TrimSpace(legacy.Phase)
	if next.CollectedFields == nil {
		next.CollectedFields = map[string]any{}
	}
	for key, value := range legacy.Fields {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		next.CollectedFields[key] = value
	}
	if legacy.TargetRef != nil {
		if value := strings.TrimSpace(legacy.TargetRef.ID); value != "" {
			next.CollectedFields["target_ref_id"] = value
		}
		if value := strings.TrimSpace(legacy.TargetRef.Name); value != "" {
			next.CollectedFields["target_ref_name"] = value
		}
	}
	return next
}

func ensureTargetRef(s *skillSession) {
	if s.TargetRef == nil {
		s.TargetRef = &EntityReference{}
	}
}

func (a *Agent) buildActiveSessionResources(storeUserID string, session skillSession) map[string]any {
	switch session.Name {
	case "trader_management":
		if session.Action == "create" {
			return a.buildTraderCreateConversationResources(storeUserID, session)
		}
		return a.buildSimpleEntityConversationResources(storeUserID, session, a.loadTraderOptions(storeUserID))
	case "exchange_management":
		return a.buildSimpleEntityConversationResources(storeUserID, session, a.loadExchangeOptions(storeUserID))
	case "model_management":
		return a.buildSimpleEntityConversationResources(storeUserID, session, a.loadEnabledModelOptions(storeUserID))
	case "strategy_management":
		resources := a.buildSimpleEntityConversationResources(storeUserID, session, a.loadStrategyOptions(storeUserID))
		if strategyType := explicitStrategyCreateType(session); strategyType != "" {
			lang := defaultIfEmpty(a.config.Language, "zh")
			resources["current_strategy_type"] = strategyType
			resources["current_editable_fields"] = manualStrategyEditableFieldKeysForType(strategyType)
			if session.Action == "create" || session.Action == "update_config" {
				resources["product_default_template"] = strategyProductDefaultTemplateResource(lang, strategyType)
				if cfg, _, _, err := strategyCreateConfigFromSession(session, lang); err == nil {
					resources["current_missing_template_fields"] = strategyCreateMissingTemplateFields(session, cfg)
				}
			}
		} else if strategyType, ok := a.strategyTypeForTarget(storeUserID, session.TargetRef); ok {
			lang := defaultIfEmpty(a.config.Language, "zh")
			resources["target_strategy_type"] = strategyType
			resources["target_editable_fields"] = manualStrategyEditableFieldKeysForType(strategyType)
			resources["product_default_template"] = strategyProductDefaultTemplateResource(lang, strategyType)
		}
		return resources
	default:
		return nil
	}
}

func strategyProductDefaultTemplateResource(lang, strategyType string) map[string]any {
	cfg := store.GetDefaultStrategyConfig(defaultIfEmpty(lang, "zh"))
	cfg.StrategyType = strings.TrimSpace(strategyType)
	cfg.ClampLimits()
	publish := map[string]any{
		"is_public":      false,
		"config_visible": true,
	}
	switch cfg.StrategyType {
	case "grid_trading":
		grid := cfg.GridConfig
		if grid == nil {
			defaultGrid := store.DefaultGridStrategyConfig()
			grid = &defaultGrid
		}
		return map[string]any{
			"strategy_type":   "grid_trading",
			"grid_config":     grid,
			"publish_config":  publish,
			"required_fields": strategyCreateMissingGridFields(skillSession{}),
		}
	default:
		return map[string]any{
			"strategy_type": "ai_trading",
			"ai_config": map[string]any{
				"coin_source": map[string]any{
					"source_type":    cfg.CoinSource.SourceType,
					"static_coins":   cfg.CoinSource.StaticCoins,
					"excluded_coins": cfg.CoinSource.ExcludedCoins,
					"ai500_limit":    cfg.CoinSource.AI500Limit,
					"oi_top_limit":   cfg.CoinSource.OITopLimit,
					"oi_low_limit":   cfg.CoinSource.OILowLimit,
				},
				"indicators": map[string]any{
					"klines": map[string]any{
						"primary_timeframe":   cfg.Indicators.Klines.PrimaryTimeframe,
						"primary_count":       cfg.Indicators.Klines.PrimaryCount,
						"selected_timeframes": cfg.Indicators.Klines.SelectedTimeframes,
					},
					"enable_ema":          cfg.Indicators.EnableEMA,
					"enable_macd":         cfg.Indicators.EnableMACD,
					"enable_rsi":          cfg.Indicators.EnableRSI,
					"enable_atr":          cfg.Indicators.EnableATR,
					"enable_boll":         cfg.Indicators.EnableBOLL,
					"enable_volume":       cfg.Indicators.EnableVolume,
					"enable_oi":           cfg.Indicators.EnableOI,
					"enable_funding_rate": cfg.Indicators.EnableFundingRate,
				},
				"risk_control": map[string]any{
					"btc_eth_max_leverage":  cfg.RiskControl.BTCETHMaxLeverage,
					"altcoin_max_leverage":  cfg.RiskControl.AltcoinMaxLeverage,
					"min_confidence":        cfg.RiskControl.MinConfidence,
					"min_risk_reward_ratio": cfg.RiskControl.MinRiskRewardRatio,
				},
				"prompt_sections": map[string]any{
					"trading_frequency": cfg.PromptSections.TradingFrequency,
					"entry_standards":   cfg.PromptSections.EntryStandards,
				},
				"custom_prompt": cfg.CustomPrompt,
			},
			"publish_config": publish,
			"non_fields": []string{
				"investment_amount",
				"fixed_position_size",
				"stop_loss_pct",
				"daily_loss_limit_pct",
				"max_drawdown_pct",
			},
			"required_fields": []string{
				"source_type",
				"primary_timeframe",
				"selected_timeframes",
				"btceth_max_leverage",
				"altcoin_max_leverage",
				"min_confidence",
				"min_risk_reward_ratio",
				"trading_frequency",
				"entry_standards",
			},
		}
	}
}

func missingRequiredFieldsForBrain(session ActiveSkillSession) []string {
	missing := missingRequiredFields(session)
	if len(missing) == 0 {
		return nil
	}
	out := make([]string, 0, len(missing))
	for _, field := range missing {
		if field == "target_ref" {
			if activeSessionHasField(session, "target_ref") {
				continue
			}
		}
		out = append(out, field)
	}
	return out
}

func formatActiveSessionLocalHistory(history []chatMessage) string {
	if len(history) == 0 {
		return ""
	}
	start := 0
	if len(history) > 8 {
		start = len(history) - 8
	}
	lines := make([]string, 0, len(history)-start)
	for _, msg := range history[start:] {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "unknown"
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}
	return strings.Join(lines, "\n")
}

func appendActiveSessionLocalHistory(session ActiveSkillSession, role, content string) ActiveSkillSession {
	content = strings.TrimSpace(content)
	if content == "" {
		return session
	}
	session.LocalHistory = append(session.LocalHistory, chatMessage{
		Role:    strings.TrimSpace(role),
		Content: content,
	})
	if len(session.LocalHistory) > 12 {
		session.LocalHistory = append([]chatMessage(nil), session.LocalHistory[len(session.LocalHistory)-12:]...)
	}
	return session
}

func parseTargetSkill(target string) (skill, action string) {
	parts := strings.SplitN(target, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func mergeExtractedData(s *ActiveSkillSession, data map[string]any) {
	if s.CollectedFields == nil {
		s.CollectedFields = map[string]any{}
	}
	if s.SkillName == "strategy_management" && s.ActionName == "create" {
		if incomingType := strategyTypeFromExtractedData(data); incomingType != "" {
			currentType := strategyTypeFromCollectedFields(s.CollectedFields)
			if currentType != "" && currentType != incomingType {
				resetActiveStrategyCreateFieldsForType(s, incomingType)
			}
		}
	}
	for k, v := range data {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		s.CollectedFields[k] = v
	}
}

func filterExtractedDataForActiveSession(session ActiveSkillSession, data map[string]any, lang string) map[string]any {
	if len(data) == 0 {
		return data
	}
	specs := allowedFieldSpecsForSkillSession(activeToLegacySkillSession(session), lang)
	if len(specs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		key := strings.TrimSpace(spec.Key)
		if key != "" {
			allowed[key] = struct{}{}
		}
	}
	out := make(map[string]any, len(data))
	for key, value := range data {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := allowed[key]; !ok {
			continue
		}
		out[key] = value
	}
	if session.SkillName == "strategy_management" && session.ActionName == "create" {
		out = filterStrategyCreateExtractedDataByTemplate(session, out)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func strategyTypeFromExtractedData(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	if value, ok := data["strategy_type"]; ok {
		if strategyType := parseStrategyTypeValue(fmt.Sprint(value)); strategyType != "" {
			return strategyType
		}
	}
	if patch, ok := data[strategyCreateConfigPatchField]; ok {
		if strategyType := strategyTypeFromConfigPatchAny(patch); strategyType != "" {
			return strategyType
		}
	}
	return ""
}

func strategyTypeFromCollectedFields(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	if value, ok := fields["strategy_type"]; ok {
		if strategyType := parseStrategyTypeValue(fmt.Sprint(value)); strategyType != "" {
			return strategyType
		}
	}
	if patch, ok := fields[strategyCreateConfigPatchField]; ok {
		if strategyType := strategyTypeFromConfigPatchAny(patch); strategyType != "" {
			return strategyType
		}
	}
	return ""
}

func strategyTypeFromConfigPatchAny(value any) string {
	patch := mapFromAny(value)
	if len(patch) == 0 {
		return ""
	}
	if strategyType := parseStrategyTypeValue(fmt.Sprint(patch["strategy_type"])); strategyType != "" {
		return strategyType
	}
	if _, ok := patch["grid_config"]; ok {
		return "grid_trading"
	}
	if _, ok := patch["ai_config"]; ok {
		return "ai_trading"
	}
	return ""
}

func resetActiveStrategyCreateFieldsForType(s *ActiveSkillSession, strategyType string) {
	if s.CollectedFields == nil {
		s.CollectedFields = map[string]any{}
	}
	keep := map[string]any{}
	for _, key := range []string{"name", "description", "is_public", "config_visible", "lang"} {
		if value, ok := s.CollectedFields[key]; ok {
			keep[key] = value
		}
	}
	keep["strategy_type"] = strategyType
	s.CollectedFields = keep
}

func filterStrategyCreateExtractedDataByTemplate(session ActiveSkillSession, data map[string]any) map[string]any {
	if len(data) == 0 {
		return data
	}
	strategyType := strategyTypeFromExtractedData(data)
	if strategyType == "" {
		strategyType = strategyTypeFromCollectedFields(session.CollectedFields)
	}
	if strategyType == "" {
		return data
	}
	allowed := map[string]struct{}{}
	for _, key := range manualStrategyEditableFieldKeysForType(strategyType) {
		allowed[key] = struct{}{}
	}
	out := make(map[string]any, len(data))
	for key, value := range data {
		if key == strategyCreateConfigPatchField {
			if patch := sanitizeStrategyCreateConfigPatchForType(value, strategyType); len(patch) > 0 {
				out[key] = patch
			}
			continue
		}
		if key == "awaiting_final_confirmation" {
			out[key] = value
			continue
		}
		if _, ok := allowed[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeStrategyCreateConfigPatchForType(value any, strategyType string) map[string]any {
	patch := mapFromAny(value)
	if len(patch) == 0 {
		return nil
	}
	out := map[string]any{
		"strategy_type": strategyType,
	}
	if publish := mapFromAny(patch["publish_config"]); len(publish) > 0 {
		out["publish_config"] = publish
	}
	switch strategyType {
	case "grid_trading":
		if grid := mapFromAny(patch["grid_config"]); len(grid) > 0 {
			out["grid_config"] = grid
		}
	case "ai_trading":
		ai := mapFromAny(patch["ai_config"])
		if ai == nil {
			ai = map[string]any{}
		}
		for _, key := range []string{"coin_source", "indicators", "risk_control", "prompt_sections", "custom_prompt"} {
			if value, ok := patch[key]; ok {
				ai[key] = value
			}
		}
		if len(ai) > 0 {
			out["ai_config"] = ai
		}
	}
	if len(out) == 1 {
		return nil
	}
	return out
}

func mapFromAny(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case string:
		var out map[string]any
		if err := json.Unmarshal([]byte(typed), &out); err == nil {
			return out
		}
	}
	return nil
}

func emitBrainReply(onEvent func(event, data string), reply string) {
	if onEvent == nil || reply == "" {
		return
	}
	onEvent(StreamEventTool, "central_brain")
	emitStreamText(onEvent, reply)
}
