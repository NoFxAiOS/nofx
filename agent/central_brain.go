package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

// brainDecision is the routing contract between the first-pass LLM and the executor.
type brainDecision struct {
	ThoughtProcess string         `json:"thought_process"`
	ActionType     string         `json:"action_type"` // CONTINUE_TASK | NEW_TASK | EXPLAIN_KNOWLEDGE | CANCEL_TASK
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
- Use CANCEL_TASK for "cancel", "stop", "forget it", "never mind", "算了", "取消".
- Domain guard: if the user says "模型", "AI 模型", or "model" and asks to create or configure one, you must route to model_management, not exchange_management.
- Domain guard: for model_management, the field "provider" means the AI model vendor such as OpenAI, DeepSeek, Claude, Gemini, Qwen, Kimi, Grok, Minimax, claw402, blockrun-base, or blockrun-sol. It never means an exchange like Binance, OKX, Bybit, CFD, forex, or metals.
- extracted_data should include any concrete facts from the user's message.
- If the user clearly means a bulk destructive operation like "删除所有策略" or "全部删除策略", put the intent signal into extracted_data too. Example: {"bulk_scope":"all"}.
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
		sb.WriteString(defaultIfEmpty(buildSkillDomainPrimer(lang, activeSession.SkillName), "none"))
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
		mergeExtractedData(&session, d.ExtractedData)
		return a.driveActiveSession(ctx, storeUserID, userID, lang, text, session, onEvent)

	case "CONTINUE_TASK":
		if !hasActive {
			return "", false, nil
		}
		mergeExtractedData(&activeSession, d.ExtractedData)
		return a.driveActiveSession(ctx, storeUserID, userID, lang, text, activeSession, onEvent)

	default:
		return "", false, nil
	}
}

func (a *Agent) driveActiveSession(ctx context.Context, storeUserID string, userID int64, lang, text string, session ActiveSkillSession, onEvent func(event, data string)) (string, bool, error) {
	if answer, ok := a.answerSkillSessionExplanation(storeUserID, lang, activeToLegacySkillSession(session), text); ok {
		session = appendActiveSessionLocalHistory(session, "user", text)
		session = appendActiveSessionLocalHistory(session, "assistant", answer)
		setActiveSessionPendingHint(&session, answer)
		a.saveActiveSkillSession(session)
		emitBrainReply(onEvent, answer)
		a.recordSkillInteraction(userID, text, answer)
		return answer, true, nil
	}

	session = appendActiveSessionLocalHistory(session, "user", text)
	clearActiveSessionPendingHint(&session)

	stepDecision, ok := a.planActiveSessionStep(ctx, storeUserID, userID, lang, text, session)
	if !ok {
		stepDecision = activeSessionStepDecision{}
	}
	mergeExtractedData(&session, stepDecision.ExtractedData)

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
		a.clearActiveSkillSession(userID)
		reply := strings.TrimSpace(stepDecision.Reply)
		if reply == "" {
			return "", false, nil
		}
		emitBrainReply(onEvent, reply)
		a.recordSkillInteraction(userID, text, reply)
		return reply, true, nil

	case "ask_user":
		reply := strings.TrimSpace(stepDecision.Reply)
		if reply == "" {
			reply = a.askForMissingFields(lang, session)
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

func (a *Agent) planActiveSessionStep(ctx context.Context, storeUserID string, userID int64, lang, text string, session ActiveSkillSession) (activeSessionStepDecision, bool) {
	if a.aiClient == nil {
		return activeSessionStepDecision{}, false
	}

	legacy := activeToLegacySkillSession(session)
	resources := a.buildActiveSessionResources(storeUserID, legacy)
	resourcesJSON, _ := json.Marshal(resources)
	collectedJSON, _ := json.Marshal(session.CollectedFields)
	missingSummary := formatConversationMissingFields(lang, missingRequiredFieldsForBrain(session))
	localHistory := formatActiveSessionLocalHistory(session.LocalHistory)
	if localHistory == "" {
		localHistory = "(empty)"
	}
	previousAssistantReply := a.currentPendingHintText(userID)

	domainPrimer := buildSkillDomainPrimer(lang, session.SkillName)

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

Domain knowledge:
%s

Rules:
- Your job is to decide the next move, not to explain internal schema names.
- Read the previous assistant reply carefully. The user's short answer may be replying to that exact proposal, confirmation request, or question.
- Use contextual memory from the active task history and current references.
- Prefer "execute_skill" when the user has already given enough information to act.
- Prefer "ask_user" only when something truly necessary is still missing.
- If the current message is only a greeting, thanks, acknowledgement, or small talk and does not add task information, do NOT continue task execution. Choose "ask_user" only if you need to gently restate what is pending; otherwise choose "finish_task" with a short social reply.
- Ask naturally. Do not say raw slot names like target_ref unless the user explicitly asks for internal details.
- If the user clearly means a bulk destructive operation like "删除所有策略", "全部删除策略", "all strategies", set extracted_data to {"bulk_scope":"all"} and choose "execute_skill". Do not ask for target_ref.
- If the user refers to a specific object from disclosed targets, set target_ref_id and target_ref_name when you can resolve it.
- If there are multiple targets and the user did not disambiguate, ask a natural question with the available names.
- If the current user message answers a missing field directly, extract it and continue.
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
		defaultIfEmpty(domainPrimer, "(none)"),
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
	return parseActiveSessionStepDecision(raw)
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
		str := strings.TrimSpace(fmt.Sprint(v))
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
	return legacy
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
		return a.buildSimpleEntityConversationResources(storeUserID, session, a.loadStrategyOptions(storeUserID))
	default:
		return nil
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
	for k, v := range data {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		s.CollectedFields[k] = v
	}
}

func emitBrainReply(onEvent func(event, data string), reply string) {
	if onEvent == nil || reply == "" {
		return
	}
	onEvent(StreamEventTool, "central_brain")
	emitStreamText(onEvent, reply)
}
