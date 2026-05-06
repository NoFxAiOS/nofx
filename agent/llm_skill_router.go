package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

type unifiedTurnDecision struct {
	TopicIntent      string         `json:"topic_intent,omitempty"`
	BusinessAction   string         `json:"business_action,omitempty"`
	TargetSkill      string         `json:"target_skill,omitempty"`
	Tasks            []WorkflowTask `json:"tasks,omitempty"`
	TargetSnapshotID string         `json:"target_snapshot_id,omitempty"`
	ContextMode      string         `json:"context_mode,omitempty"`
	ExtractedData    map[string]any `json:"extracted_data,omitempty"`
	ReplyToUser      string         `json:"reply_to_user,omitempty"`
	Confidence       float64        `json:"confidence,omitempty"`
}

func (a *Agent) tryLLMIntentRoute(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	if a.aiClient == nil {
		return "", false, nil
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", false, nil
	}

	if decision, ok, err := a.routeTurnUnifiedWithLLM(ctx, userID, lang, text); err == nil && ok {
		if answer, handled, execErr := a.executeUnifiedTurnDecision(ctx, storeUserID, userID, lang, text, decision, onEvent); handled || execErr != nil {
			return answer, handled, execErr
		}
	}
	return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
}

func parseUnifiedTurnDecision(raw string) (unifiedTurnDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision unifiedTurnDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeUnifiedTurnDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeUnifiedTurnDecision(decision), nil
		}
	}
	return unifiedTurnDecision{}, fmt.Errorf("invalid unified turn decision json")
}

func normalizeUnifiedTurnDecision(decision unifiedTurnDecision) unifiedTurnDecision {
	decision.TopicIntent = strings.TrimSpace(strings.ToLower(decision.TopicIntent))
	decision.BusinessAction = strings.TrimSpace(strings.ToLower(decision.BusinessAction))
	decision.TargetSkill = strings.TrimSpace(decision.TargetSkill)
	decision.TargetSnapshotID = strings.TrimSpace(decision.TargetSnapshotID)
	decision.ContextMode = strings.TrimSpace(strings.ToLower(decision.ContextMode))
	decision.ReplyToUser = strings.TrimSpace(decision.ReplyToUser)
	decision.Tasks = normalizeWorkflowDecomposition(workflowDecomposition{Tasks: decision.Tasks}).Tasks
	if decision.ExtractedData == nil {
		decision.ExtractedData = map[string]any{}
	}
	if decision.Confidence < 0 {
		decision.Confidence = 0
	}
	if decision.Confidence > 1 {
		decision.Confidence = 1
	}
	switch decision.TopicIntent {
	case "continue", "continue_active":
		decision.TopicIntent = "continue_active"
	case "start_new", "resume_snapshot", "cancel", "instant_reply":
	default:
		decision.TopicIntent = ""
	}
	switch decision.BusinessAction {
	case "direct_answer", "new_skill", "skill_tasks", "continue_skill", "planned_agent", "none":
	default:
		decision.BusinessAction = ""
	}
	switch decision.ContextMode {
	case "use_current", "fresh_context", "resume_snapshot":
	default:
		decision.ContextMode = "use_current"
	}
	return decision
}

func (d unifiedTurnDecision) reliable() bool {
	if d.TopicIntent == "" || d.BusinessAction == "" {
		return false
	}
	if d.Confidence > 0 && d.Confidence < 0.45 {
		return false
	}
	switch d.BusinessAction {
	case "direct_answer":
		return strings.TrimSpace(d.ReplyToUser) != ""
	case "new_skill":
		if len(d.Tasks) > 0 {
			return true
		}
		skill, _ := parseTargetSkill(d.TargetSkill)
		return skill != ""
	case "skill_tasks":
		return len(d.Tasks) > 0
	case "continue_skill":
		return d.TopicIntent == "continue_active"
	case "planned_agent", "none":
		return true
	default:
		return false
	}
}

func (a *Agent) routeTurnUnifiedWithLLM(ctx context.Context, userID int64, lang, text string) (unifiedTurnDecision, bool, error) {
	systemPrompt, userPrompt := a.buildUnifiedTurnRouterPrompt(userID, lang, text)
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
		return unifiedTurnDecision{}, false, err
	}
	decision, err := parseUnifiedTurnDecision(raw)
	if err != nil {
		return unifiedTurnDecision{}, false, err
	}
	if !decision.reliable() {
		return decision, false, nil
	}
	return decision, true, nil
}

func (a *Agent) buildUnifiedTurnRouterPrompt(userID int64, lang, text string) (string, string) {
	activeSkill := a.getSkillSession(userID)
	activeTask, hasActiveTask := a.getActiveSkillSession(userID)
	activeWorkflow := a.getWorkflowSession(userID)
	activeExec := a.getExecutionState(userID)
	pendingProposal, hasPendingProposal := a.getPendingProposalSession(userID)
	previousAssistantReply := a.currentPendingHintText(userID)
	snapshots := a.SnapshotManager(userID).List()
	snapshotJSON, _ := json.Marshal(snapshots)
	currentRefs := buildCurrentReferenceSummary(lang, a.semanticCurrentReferences(userID))
	recentConversation := a.buildRecentConversationContext(userID, text)
	if strings.TrimSpace(recentConversation) == "" {
		recentConversation = "(empty)"
	}
	activeFlowSummary := buildTopLevelActiveFlowSummary(lang, activeSkill, activeTask, hasActiveTask, activeWorkflow, activeExec, pendingProposal, hasPendingProposal)
	if strings.TrimSpace(activeFlowSummary) == "" {
		activeFlowSummary = "none"
	}

	activeTaskDetails := "none"
	if hasActiveTask {
		activeTaskDetails = buildBrainUserPrompt(lang, text, previousAssistantReply, recentConversation, currentRefs, activeTask, true)
	}

	systemPrompt := prependNOFXiAdvisorPreamble(`You are the unified turn router for NOFXi.
Return JSON only. No markdown.

You must make ONE combined decision for this user turn:
1. Topic/context decision: continue active context, start fresh/new context, resume snapshot, cancel, or direct conversational reply.
2. Business routing decision: answer directly, start/continue a management skill, or hand off to the planner.
3. Context policy: whether downstream modules may use current references, must use fresh context, or must resume a snapshot.

topic_intent values:
- "continue_active": user is answering or continuing the active flow
- "start_new": user starts or switches to a new task/topic
- "resume_snapshot": user wants to resume one suspended snapshot
- "cancel": user cancels the current active flow
- "instant_reply": user only greets, thanks, chats, or asks a direct explanation

business_action values:
- "direct_answer": reply_to_user is the final answer; do not change state
- "skill_tasks": start one or more management/diagnosis skill tasks; tasks is required
- "new_skill": legacy single-skill route; target_skill is required if tasks is empty
- "continue_skill": continue the active skill session
- "planned_agent": hand off to the execution planner/tools
- "none": only valid with cancel when no more action is needed

tasks format for skill_tasks:
- id: "task_1", "task_2", ...
- skill: one available skill name
- action: one available action
- request: the self-contained user-readable subtask
- depends_on: array of task ids, empty when independent

target_skill format for legacy new_skill:
skill_name:action, for example "trader_management:create".
Available skills:
trader_management, exchange_management, model_management, strategy_management,
trader_diagnosis, exchange_diagnosis, model_diagnosis, strategy_diagnosis

Available actions:
create, update, update_name, update_bindings, configure_strategy, configure_exchange, configure_model,
update_status, update_endpoint, update_config, update_prompt, delete, start, stop, activate, duplicate,
query_list, query_detail, query_running

context_mode values:
- "use_current": downstream modules may use current references and recent context
- "fresh_context": the user is switching topic; do not use old current references to fill business fields
- "resume_snapshot": restore target_snapshot_id first

Rules:
- This router decides what context downstream LLMs will see. Be conservative with stale references.
- Treat topic_intent as the primary decision. If the user is naturally responding to the active flow, choose topic_intent="continue_active", business_action="continue_skill", context_mode="use_current"; do not hand off a continuing active flow to planned_agent.
- When an active flow has a previous assistant question, proposal, or confirmation request, reason about what the user's message refers to in that context before deciding it is a new task.
- If the user clearly switches domain/entity, set topic_intent="start_new" and context_mode="fresh_context".
- If the user says "不是交易员，是策略" or similar corrections, use fresh_context.
- If the user answers the previous assistant question, choose continue_active.
- If the user only says "你好", "hi", "谢谢", "收到", choose instant_reply + direct_answer unless it clearly answers a pending task.
- If the user asks a read-only management query, prefer planned_agent unless the answer is already fully available in the provided context.
- Use skill_tasks for clear management tasks such as creating/updating/deleting/configuring trader/model/exchange/strategy.
- If the user request contains multiple management operations, include multiple tasks and depends_on where a later task needs an earlier result.
- If the request contains exactly one management operation, include exactly one task.
- Use planned_agent for multi-step, tool-heavy, market/account, diagnosis, or ambiguous tasks.
- For model_management, "provider" means AI vendor, never an exchange.
- Current references are context only. Do not copy them into extracted_data unless the user explicitly says this/current/that previous one.
- extracted_data must contain only concrete facts from the current user message.
- reply_to_user must be concise and in the user's language.
- confidence should reflect how safe it is to execute this decision without the old router fallback.

Return JSON with this exact shape:
{"topic_intent":"continue_active|start_new|resume_snapshot|cancel|instant_reply","business_action":"direct_answer|skill_tasks|new_skill|continue_skill|planned_agent|none","target_skill":"","tasks":[{"id":"task_1","skill":"","action":"","request":"","depends_on":[]}],"target_snapshot_id":"","context_mode":"use_current|fresh_context|resume_snapshot","extracted_data":{},"reply_to_user":"","confidence":0.0}`)

	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nPrevious assistant reply:\n%s\n\nCurrent reference summary:\n%s\n\nActive flow summary:\n%s\n\nSuspended snapshots JSON:\n%s\n\nRecent conversation:\n%s\n\nManagement domain primer:\n%s\n\nActive task details:\n%s\n",
		lang,
		text,
		defaultIfEmpty(previousAssistantReply, "(empty)"),
		currentRefs,
		activeFlowSummary,
		defaultIfEmpty(string(snapshotJSON), "[]"),
		recentConversation,
		defaultIfEmpty(buildManagementDomainPrimer(lang), "(empty)"),
		activeTaskDetails,
	)

	return systemPrompt, userPrompt
}

func (a *Agent) executeUnifiedTurnDecision(ctx context.Context, storeUserID string, userID int64, lang, text string, decision unifiedTurnDecision, onEvent func(event, data string)) (string, bool, error) {
	switch decision.TopicIntent {
	case "cancel":
		a.clearPendingProposalSession(userID)
		if a.hasAnyActiveContext(userID) {
			a.clearActiveSkillSession(userID)
			a.clearAnyActiveContext(userID)
			return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
		}
		if decision.BusinessAction == "direct_answer" && decision.ReplyToUser != "" {
			emitBrainReply(onEvent, decision.ReplyToUser)
			a.recordSkillInteraction(userID, text, decision.ReplyToUser)
			return decision.ReplyToUser, true, nil
		}
		return "", false, nil
	case "resume_snapshot":
		a.clearPendingProposalSession(userID)
		if a.tryRestoreSuspendedTaskAfterSwitch(userID, text, decision.TargetSnapshotID) {
			if decision.BusinessAction == "planned_agent" {
				answer, err := a.runPlannedAgentWithContextMode(ctx, storeUserID, userID, lang, text, "use_current", onEvent)
				return answer, true, err
			}
			return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
		}
		return "", false, nil
	}

	if decision.TopicIntent == "continue_active" {
		if _, hasProposal := a.getPendingProposalSession(userID); hasProposal && !a.hasAnyActiveContext(userID) {
			return a.handlePendingProposalResponse(ctx, storeUserID, userID, lang, text, onEvent)
		}
		if activeSession, hasActive := a.getActiveSkillSession(userID); hasActive {
			decision.ExtractedData = filterExtractedDataForActiveSession(activeSession, decision.ExtractedData, lang)
			mergeExtractedData(&activeSession, decision.ExtractedData)
			return a.driveActiveSession(ctx, storeUserID, userID, lang, text, activeSession, onEvent)
		}
		if a.hasAnyActiveContext(userID) {
			return a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent)
		}
	}

	switch decision.BusinessAction {
	case "direct_answer":
		if decision.ReplyToUser == "" {
			return "", false, nil
		}
		if decision.TopicIntent == "instant_reply" && a.hasAnyActiveContext(userID) {
			return a.replyToActiveFlowInstantReply(ctx, userID, lang, text, onEvent), true, nil
		}
		if guarded, blocked := guardUnsupportedAsyncPromise(lang, decision.ReplyToUser); blocked {
			decision.ReplyToUser = guarded
		}
		emitBrainReply(onEvent, decision.ReplyToUser)
		a.recordSkillInteraction(userID, text, decision.ReplyToUser)
		a.runPostResponseMaintenanceAsync(userID)
		return decision.ReplyToUser, true, nil
	case "new_skill":
		if len(decision.Tasks) > 0 {
			return a.executeUnifiedSkillTasks(ctx, storeUserID, userID, lang, text, decision, onEvent)
		}
		skill, action := parseTargetSkill(decision.TargetSkill)
		if skill == "" {
			return "", false, nil
		}
		if a.hasAnyActiveContext(userID) && decision.ContextMode == "fresh_context" {
			if !a.suspendActiveContexts(userID, lang) {
				a.clearSkillSession(userID)
				a.clearWorkflowSession(userID)
				a.clearExecutionState(userID)
			}
			a.clearActiveSkillSession(userID)
		}
		session := newActiveSkillSession(userID, skill, action)
		session.Goal = strings.TrimSpace(text)
		decision.ExtractedData = filterExtractedDataForActiveSession(session, decision.ExtractedData, lang)
		mergeExtractedData(&session, decision.ExtractedData)
		return a.driveActiveSession(ctx, storeUserID, userID, lang, text, session, onEvent)
	case "skill_tasks":
		return a.executeUnifiedSkillTasks(ctx, storeUserID, userID, lang, text, decision, onEvent)
	case "continue_skill":
		activeSession, hasActive := a.getActiveSkillSession(userID)
		if !hasActive {
			return "", false, nil
		}
		decision.ExtractedData = filterExtractedDataForActiveSession(activeSession, decision.ExtractedData, lang)
		mergeExtractedData(&activeSession, decision.ExtractedData)
		return a.driveActiveSession(ctx, storeUserID, userID, lang, text, activeSession, onEvent)
	case "planned_agent":
		contextMode := decision.ContextMode
		if contextMode == "resume_snapshot" {
			contextMode = "use_current"
		}
		answer, err := a.runPlannedAgentWithContextMode(ctx, storeUserID, userID, lang, text, contextMode, onEvent)
		return answer, true, err
	case "none":
		return "", false, nil
	default:
		return "", false, nil
	}
}

func (a *Agent) executeUnifiedSkillTasks(ctx context.Context, storeUserID string, userID int64, lang, text string, decision unifiedTurnDecision, onEvent func(event, data string)) (string, bool, error) {
	tasks := normalizeWorkflowDecomposition(workflowDecomposition{Tasks: decision.Tasks}).Tasks
	if len(tasks) == 0 {
		return "", false, nil
	}
	if a.hasAnyActiveContext(userID) && decision.ContextMode == "fresh_context" {
		if !a.suspendActiveContexts(userID, lang) {
			a.clearSkillSession(userID)
			a.clearWorkflowSession(userID)
			a.clearExecutionState(userID)
		}
		a.clearActiveSkillSession(userID)
	}
	if len(tasks) == 1 {
		task := tasks[0]
		session := newActiveSkillSession(userID, task.Skill, task.Action)
		session.Goal = defaultIfEmpty(strings.TrimSpace(task.Request), strings.TrimSpace(text))
		decision.ExtractedData = filterExtractedDataForActiveSession(session, decision.ExtractedData, lang)
		mergeExtractedData(&session, decision.ExtractedData)
		return a.driveActiveSession(ctx, storeUserID, userID, lang, defaultIfEmpty(task.Request, text), session, onEvent)
	}
	session := normalizeWorkflowSession(WorkflowSession{
		UserID:          userID,
		OriginalRequest: strings.TrimSpace(text),
		Tasks:           tasks,
	})
	if len(session.Tasks) == 0 {
		return "", false, nil
	}
	a.saveWorkflowSession(userID, session)
	return a.maybeAdvanceWorkflow(ctx, storeUserID, userID, lang, session, onEvent)
}

func buildTopLevelActiveFlowSummary(lang string, skill skillSession, activeTask ActiveSkillSession, hasActiveTask bool, workflow WorkflowSession, state ExecutionState, pendingProposal PendingProposalSession, hasPendingProposal bool) string {
	lines := make([]string, 0, 8)
	if hasActiveTask {
		lines = append(lines, fmt.Sprintf("Active task session: %s / %s / phase=%s", activeTask.SkillName, activeTask.ActionName, defaultIfEmpty(activeTask.LegacyPhase, "collecting")))
		if strings.TrimSpace(activeTask.Goal) != "" {
			lines = append(lines, "Active task goal: "+strings.TrimSpace(activeTask.Goal))
		}
		if activeTask.PendingHint != nil && strings.TrimSpace(activeTask.PendingHint.Prompt) != "" {
			lines = append(lines, "Active task pending hint: "+strings.TrimSpace(activeTask.PendingHint.Prompt))
		}
		if len(activeTask.CollectedFields) > 0 {
			fieldsJSON, _ := json.Marshal(activeTask.CollectedFields)
			lines = append(lines, "Active task collected_fields: "+string(fieldsJSON))
		}
	}
	if strings.TrimSpace(skill.Name) != "" {
		lines = append(lines, fmt.Sprintf("Active skill session: %s / %s / phase=%s", skill.Name, skill.Action, defaultIfEmpty(skill.Phase, "collecting")))
		if routing := buildSkillActionRoutingSummary(lang, skill); routing != "" {
			lines = append(lines, routing)
		}
	}
	if hasActiveWorkflowSession(workflow) {
		lines = append(lines, fmt.Sprintf("Active workflow: original_request=%s pending_tasks=%d", workflow.OriginalRequest, countPendingWorkflowTasks(workflow)))
	}
	if hasActiveExecutionState(state) {
		lines = append(lines, fmt.Sprintf("Active execution state: status=%s goal=%s", state.Status, state.Goal))
		if state.Waiting != nil && strings.TrimSpace(state.Waiting.Question) != "" {
			lines = append(lines, "Waiting question: "+strings.TrimSpace(state.Waiting.Question))
		}
	}
	if hasPendingProposal {
		lines = append(lines, "Pending assistant proposal awaiting user response.")
		if strings.TrimSpace(pendingProposal.SourceUserText) != "" {
			lines = append(lines, "Proposal source request: "+strings.TrimSpace(pendingProposal.SourceUserText))
		}
		lines = append(lines, "Proposal text: "+strings.TrimSpace(pendingProposal.ProposalText))
	}
	return strings.Join(lines, "\n")
}

func (a *Agent) handlePendingProposalResponse(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	proposal, ok := a.getPendingProposalSession(userID)
	if !ok {
		return "", false, nil
	}
	answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, fmt.Sprintf("The user is replying to the assistant's previous proposal.\n\nOriginal user request:\n%s\n\nPrevious assistant proposal:\n%s\n\nCurrent user reply:\n%s", proposal.SourceUserText, proposal.ProposalText, text), onEvent)
	if err == nil && strings.TrimSpace(answer) != "" {
		a.clearPendingProposalSession(userID)
	}
	return answer, true, err
}

func countPendingWorkflowTasks(session WorkflowSession) int {
	count := 0
	for _, task := range session.Tasks {
		switch task.Status {
		case workflowTaskPending, workflowTaskRunning:
			count++
		}
	}
	return count
}

func buildCurrentReferenceSummary(lang string, refs *CurrentReferences) string {
	if refs == nil {
		if lang == "zh" {
			return "- 当前没有明确锁定的操作对象。"
		}
		return "- No current entity references are locked yet."
	}

	lines := make([]string, 0, 4)
	appendLine := func(kind string, ref *EntityReference) {
		if ref == nil {
			return
		}
		name := strings.TrimSpace(defaultIfEmpty(ref.Name, ref.ID))
		if name == "" {
			return
		}
		source := formatReferenceSourceLabel(lang, ref.Source)
		if lang == "zh" {
			line := fmt.Sprintf("- 当前%s: %s", referenceKindDisplayName(lang, kind), name)
			if source != "" {
				line += fmt.Sprintf("（来源: %s）", source)
			}
			if strings.TrimSpace(ref.ID) != "" && strings.TrimSpace(ref.ID) != name {
				line += fmt.Sprintf(" [id=%s]", ref.ID)
			}
			lines = append(lines, line)
			return
		}

		line := fmt.Sprintf("- Current %s: %s", referenceKindDisplayName(lang, kind), name)
		if source != "" {
			line += fmt.Sprintf(" (source: %s)", source)
		}
		if strings.TrimSpace(ref.ID) != "" && strings.TrimSpace(ref.ID) != name {
			line += fmt.Sprintf(" [id=%s]", ref.ID)
		}
		lines = append(lines, line)
	}

	appendLine("strategy", refs.Strategy)
	appendLine("trader", refs.Trader)
	appendLine("model", refs.Model)
	appendLine("exchange", refs.Exchange)

	if len(lines) == 0 {
		if lang == "zh" {
			return "- 当前没有明确锁定的操作对象。"
		}
		return "- No current entity references are locked yet."
	}
	return strings.Join(lines, "\n")
}

func formatReferenceSourceLabel(lang, source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if lang == "zh" {
		switch source {
		case "user_mention":
			return "用户提及"
		case "tool_output":
			return "工具结果"
		case "inferred_from_context":
			return "上下文推断"
		default:
			return source
		}
	}
	switch source {
	case "user_mention":
		return "user mention"
	case "tool_output":
		return "tool output"
	case "inferred_from_context":
		return "context inference"
	default:
		return source
	}
}

func hasAnyActiveContext(a *Agent, userID int64) bool {
	if a == nil {
		return false
	}
	if _, ok := a.getActiveSkillSession(userID); ok {
		return true
	}
	return a.hasActiveSkillSession(userID) || hasActiveWorkflowSession(a.getWorkflowSession(userID)) || hasActiveExecutionState(a.getExecutionState(userID))
}

func (a *Agent) clearAnyActiveContext(userID int64) bool {
	cleared := false
	if _, ok := a.getActiveSkillSession(userID); ok {
		a.clearActiveSkillSession(userID)
		cleared = true
	}
	if a.hasActiveSkillSession(userID) {
		a.clearSkillSession(userID)
		cleared = true
	}
	if hasActiveWorkflowSession(a.getWorkflowSession(userID)) {
		a.clearWorkflowSession(userID)
		cleared = true
	}
	if hasActiveExecutionState(a.getExecutionState(userID)) {
		a.clearExecutionState(userID)
		cleared = true
	}
	if cleared {
		a.SnapshotManager(userID).Clear()
	}
	return cleared
}

func skillDataForAction(storeUserID, skill, action string, a *Agent) map[string]any {
	var raw string
	switch skill {
	case "trader_management":
		if strings.HasPrefix(action, "query") {
			raw = a.toolListTraders(storeUserID)
		}
	case "exchange_management":
		if strings.HasPrefix(action, "query") {
			raw = a.toolGetExchangeConfigs(storeUserID)
		}
	case "model_management":
		if strings.HasPrefix(action, "query") {
			raw = a.toolGetModelConfigs(storeUserID)
		}
	case "strategy_management":
		if strings.HasPrefix(action, "query") {
			raw = a.toolGetStrategies(storeUserID)
		}
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil
	}
	return data
}

func mustMarshalJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func applyTraderQueryFilter(lang, fallback, raw, filter string) string {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return fallback
	}

	var payload struct {
		Traders []struct {
			Name      string `json:"name"`
			IsRunning bool   `json:"is_running"`
		} `json:"traders"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return fallback
	}

	switch filter {
	case "running_only":
		names := make([]string, 0, len(payload.Traders))
		for _, trader := range payload.Traders {
			if trader.IsRunning {
				names = append(names, strings.TrimSpace(trader.Name))
			}
		}
		if lang == "zh" {
			if len(names) == 0 {
				return "当前没有运行中的交易员。"
			}
			return fmt.Sprintf("当前有 %d 个运行中的交易员：%s。", len(names), strings.Join(names, "、"))
		}
		if len(names) == 0 {
			return "There are no running traders right now."
		}
		return fmt.Sprintf("There are %d running traders right now: %s.", len(names), strings.Join(names, ", "))
	case "stopped_only":
		names := make([]string, 0, len(payload.Traders))
		for _, trader := range payload.Traders {
			if !trader.IsRunning {
				names = append(names, strings.TrimSpace(trader.Name))
			}
		}
		if lang == "zh" {
			if len(names) == 0 {
				return "当前没有已停止的交易员。"
			}
			return fmt.Sprintf("当前有 %d 个未运行的交易员：%s。", len(names), strings.Join(names, "、"))
		}
		if len(names) == 0 {
			return "There are no stopped traders right now."
		}
		return fmt.Sprintf("There are %d stopped traders right now: %s.", len(names), strings.Join(names, ", "))
	default:
		return fallback
	}
}
