package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

type llmSkillRouteDecision struct {
	Intent           string         `json:"intent,omitempty"`
	TargetSnapshotID string         `json:"target_snapshot_id,omitempty"`
	TargetSkill      string         `json:"target_skill,omitempty"`
	ExtractedFields  map[string]any `json:"extracted_fields,omitempty"`
	NeedPlannerHelp  bool           `json:"need_planner_help,omitempty"`
	Route            string         `json:"route"`
	Track            string         `json:"track,omitempty"`
	Skill            string         `json:"skill,omitempty"`
	Action           string         `json:"action,omitempty"`
	Filter           string         `json:"filter,omitempty"`
	InlineSubIntent  string         `json:"inline_sub_intent,omitempty"`
	Tasks            []WorkflowTask `json:"tasks,omitempty"`
	ContextSwitch    bool           `json:"context_switch,omitempty"`
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

	decision, ok, err := a.routeTurnWithLLM(ctx, userID, lang, text)
	if err != nil || !ok {
		return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
	}

	switch decision.Intent {
	case "continue", "continue_active":
		if _, hasProposal := a.getPendingProposalSession(userID); hasProposal && !a.hasAnyActiveContext(userID) {
			return a.handlePendingProposalResponse(ctx, storeUserID, userID, lang, text, onEvent)
		}
		if _, has := a.getActiveSkillSession(userID); has {
			return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
		}
		if a.hasAnyActiveContext(userID) {
			return a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent)
		}
		return "", false, nil
	case "cancel":
		a.clearPendingProposalSession(userID)
		if a.hasAnyActiveContext(userID) {
			a.clearSkillSession(userID)
			a.clearWorkflowSession(userID)
			a.clearExecutionState(userID)
			return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
		}
		return "", false, nil
	case "resume_snapshot":
		a.clearPendingProposalSession(userID)
		if a.tryRestoreSuspendedTaskAfterSwitch(userID, text, decision.TargetSnapshotID) {
			if _, has := a.getActiveSkillSession(userID); has {
				return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
			}
			return a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent)
		}
		return "", false, nil
	case "instant_reply":
		if a.hasAnyActiveContext(userID) {
			return a.replyToActiveFlowInstantReply(ctx, userID, lang, text, onEvent), true, nil
		}
		if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, onEvent); ok {
			return answer, true, nil
		}
		answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
		return answer, true, err
	}

	if a.hasAnyActiveContext(userID) {
		a.clearPendingProposalSession(userID)
		return a.handoffFromActiveFlow(ctx, storeUserID, userID, lang, text, decision.TargetSnapshotID, onEvent)
	}

	switch decision.Route {
	case "workflow":
		a.clearPendingProposalSession(userID)
		answer, handled, execErr := a.executeWorkflowDecomposition(ctx, storeUserID, userID, lang, text, workflowDecomposition{Tasks: decision.Tasks}, onEvent)
		return answer, handled, execErr
	case "skill":
		a.clearPendingProposalSession(userID)
		return a.executeRoutedAtomicSkill(ctx, storeUserID, userID, lang, text, decision, onEvent)
	case "planner":
		a.clearPendingProposalSession(userID)
		answer, execErr := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
		return answer, true, execErr
	default:
		if decision.NeedPlannerHelp || decision.Track == "planning_track" {
			a.clearPendingProposalSession(userID)
			answer, execErr := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
			return answer, true, execErr
		}
	}

	return "", false, nil
}

func (a *Agent) executeRoutedAtomicSkill(ctx context.Context, storeUserID string, userID int64, lang, text string, decision llmSkillRouteDecision, onEvent func(event, data string)) (string, bool, error) {
	outcome, ok := a.executeLLMSkillRoute(storeUserID, userID, lang, text, decision)
	if !ok {
		return "", false, nil
	}

	if isReadOnlyAtomicSkillAction(outcome.Skill, outcome.Action) {
		answer := strings.TrimSpace(outcome.UserMessage)
		if answer == "" {
			return "", false, nil
		}
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			label := "llm_intent_plan"
			if decision.Skill != "" {
				label += ":" + decision.Skill
			}
			if decision.Action != "" {
				label += ":" + decision.Action
			}
			onEvent(StreamEventTool, label)
			emitStreamText(onEvent, answer)
		}
		return answer, true, nil
	}

	review, err := a.reviewTaskCompletion(ctx, userID, lang, text, outcome)
	if err != nil {
		if outcome.Status == skillOutcomeRecoverableError || outcome.Status == skillOutcomeFatalError || outcome.Status == skillOutcomeNotHandled {
			return "", false, nil
		}
		review = taskReviewDecision{Route: "complete", Answer: outcome.UserMessage}
	}
	if review.Route == "replan" {
		answer, planErr := a.runPlannedAgent(ctx, storeUserID, userID, lang, fmt.Sprintf("Original user request:\n%s\n\nPrevious skill outcome JSON:\n%s", text, mustMarshalJSON(outcome)), onEvent)
		return answer, true, planErr
	}

	answer := strings.TrimSpace(review.Answer)
	if answer == "" {
		answer = strings.TrimSpace(outcome.UserMessage)
	}
	if answer == "" {
		return "", false, nil
	}

	a.recordSkillInteraction(userID, text, answer)
	if onEvent != nil {
		label := "llm_intent_plan"
		if decision.Skill != "" {
			label += ":" + decision.Skill
		}
		if decision.Action != "" {
			label += ":" + decision.Action
		}
		onEvent(StreamEventTool, label)
		emitStreamText(onEvent, answer)
	}
	return answer, true, nil
}

func isReadOnlyAtomicSkillAction(skill, action string) bool {
	action = strings.TrimSpace(strings.ToLower(action))
	switch action {
	case "query", "query_list", "query_detail", "query_running", "query_strategy_binding", "query_exchange_binding", "query_model_binding":
		return true
	}
	return false
}

func parseLLMSkillRouteDecision(raw string) (llmSkillRouteDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision llmSkillRouteDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeLLMSkillRouteDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeLLMSkillRouteDecision(decision), nil
		}
	}
	return llmSkillRouteDecision{}, fmt.Errorf("invalid llm skill route json")
}

func normalizeLLMSkillRouteDecision(decision llmSkillRouteDecision) llmSkillRouteDecision {
	decision.Intent = strings.TrimSpace(strings.ToLower(decision.Intent))
	decision.TargetSnapshotID = strings.TrimSpace(decision.TargetSnapshotID)
	decision.TargetSkill = strings.TrimSpace(strings.ToLower(decision.TargetSkill))
	decision.Route = strings.TrimSpace(strings.ToLower(decision.Route))
	decision.Track = strings.TrimSpace(strings.ToLower(decision.Track))
	decision.Skill = strings.TrimSpace(strings.ToLower(decision.Skill))
	decision.Filter = strings.TrimSpace(strings.ToLower(decision.Filter))
	decision.Tasks = normalizeWorkflowDecomposition(workflowDecomposition{Tasks: decision.Tasks}).Tasks
	if decision.Confidence < 0 {
		decision.Confidence = 0
	}
	if decision.Confidence > 1 {
		decision.Confidence = 1
	}
	if decision.Route == "" {
		switch {
		case len(decision.Tasks) > 1:
			decision.Route = "workflow"
		case decision.TargetSkill != "":
			decision.Route = "skill"
		case decision.Skill != "" || decision.Action != "":
			decision.Route = "skill"
		case decision.Track == "planning_track":
			decision.Route = "planner"
		}
	}
	if decision.Track == "" {
		switch decision.Route {
		case "skill", "workflow":
			decision.Track = "fast_track"
		case "planner":
			decision.Track = "planning_track"
		}
	}
	if decision.Intent == "" {
		switch {
		case decision.Route == "instant_reply":
			decision.Intent = "instant_reply"
		case decision.TargetSnapshotID != "" && decision.Route == "" && decision.Skill == "" && decision.Action == "" && len(decision.Tasks) == 0:
			decision.Intent = "resume_snapshot"
		case decision.Route != "" || decision.Track != "" || decision.Skill != "" || decision.Action != "" || decision.TargetSkill != "" || len(decision.Tasks) > 0:
			decision.Intent = "start_new"
		}
	}
	if decision.Skill == "" && decision.Action == "" && decision.TargetSkill != "" {
		decision.Skill, decision.Action = parseTargetSkill(decision.TargetSkill)
	}
	if decision.Route == "" && decision.NeedPlannerHelp {
		decision.Route = "planner"
	}
	if decision.Route == "workflow" {
		decision.Skill = ""
		decision.Action = ""
		decision.Filter = ""
		return decision
	}
	if decision.Route != "skill" {
		decision.Action = ""
		decision.Skill = ""
		decision.Filter = ""
		decision.Tasks = nil
		return decision
	}
	decision.Tasks = nil
	if decision.Action == "query" && decision.Filter == "running_only" && decision.Skill == "trader_management" {
		decision.Action = "query_running"
	} else {
		decision.Action = normalizeAtomicSkillAction(decision.Skill, decision.Action)
	}
	return decision
}

func (a *Agent) routeTurnWithLLM(ctx context.Context, userID int64, lang, text string) (llmSkillRouteDecision, bool, error) {
	systemPrompt, userPrompt := a.buildTopLevelRouterPrompt(userID, lang, text)
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
		return llmSkillRouteDecision{}, false, err
	}
	decision, err := parseLLMSkillRouteDecision(raw)
	if err != nil {
		return llmSkillRouteDecision{}, false, err
	}
	return decision, true, nil
}

func (a *Agent) buildTopLevelRouterPrompt(userID int64, lang, text string) (string, string) {
	activeSkill := a.getSkillSession(userID)
	activeTask, hasActiveTask := a.getActiveSkillSession(userID)
	activeWorkflow := a.getWorkflowSession(userID)
	activeExec := a.getExecutionState(userID)
	pendingProposal, hasPendingProposal := a.getPendingProposalSession(userID)
	previousAssistantReply := a.currentPendingHintText(userID)
	snapshots := a.SnapshotManager(userID).List()
	snapshotJSON, _ := json.Marshal(snapshots)

	currentRefs := buildCurrentReferenceSummary(lang, a.semanticCurrentReferences(userID))
	managementSummary := buildManagementSkillRoutingContextWithSession(lang, &activeSkill)
	recentConversation := a.buildRecentConversationContext(userID, text)
	if strings.TrimSpace(recentConversation) == "" {
		recentConversation = "(empty)"
	}

	activeFlowSummary := buildTopLevelActiveFlowSummary(lang, activeSkill, activeTask, hasActiveTask, activeWorkflow, activeExec, pendingProposal, hasPendingProposal)
	if strings.TrimSpace(activeFlowSummary) == "" {
		activeFlowSummary = "none"
	}

	systemPrompt := prependNOFXiAdvisorPreamble(`You are the lightweight intent planner for NOFXi.
Return JSON only.

You are deciding what the current user turn should do at the top level.
You must classify every message into exactly one of these intents before any execution layer takes over.

Valid intents:
- "continue_active": the user is still working on the current active flow
- "start_new": the user is starting or switching to a new task
- "resume_snapshot": the user wants to resume one suspended snapshot
- "cancel": the user wants to cancel the current active flow
- "instant_reply": the user is greeting, chatting, thanking, or asking for a direct explanation without changing task state

Valid routes when intent=start_new:
- "skill"
- "workflow"
- "planner"

Rules:
- Read the previous assistant reply carefully. The user's short answer may be replying to that exact proposal or question.
- If Active flow summary includes a pending hint or waiting question, short replies like "1", "2", "A", "B", "确认", "需要", or "好的" usually mean the user is continuing that flow unless they clearly switch tasks.
- If the user is clearly answering the previous question, prefer "continue_active".
- If the user clearly corrects the entity/domain, you must output "start_new", not "continue_active".
- If the user explicitly refers to a suspended task like "刚才那个", "恢复刚才那个", choose "resume_snapshot" and fill target_snapshot_id.
- If the user is only greeting, thanking, social chatting, or asking a concept question without changing task state, choose "instant_reply".
- If the request is broad, ambiguous, or creative, you may choose route "planner".
- If a single management or diagnosis skill can handle it directly, prefer route "skill".
- If multiple dependent steps are needed, prefer route "workflow".
- Do not hallucinate snapshot ids; only use those disclosed in Suspended snapshots JSON.

Return JSON with this exact shape:
{"intent":"continue_active|start_new|resume_snapshot|cancel|instant_reply","target_snapshot_id":"","route":"skill|workflow|planner","track":"fast_track|planning_track","skill":"","action":"","target_skill":"","filter":"","tasks":[],"context_switch":false,"need_planner_help":false,"confidence":0.0}`)

	if strings.TrimSpace(activeSkill.Name) != "" || hasActiveTask || hasPendingProposal {
		systemPrompt = prependNOFXiAdvisorPreamble(`You are the one-pass semantic gateway for NOFXi.
Return JSON only.

You are deciding whether the user is continuing the current active flow, switching to a new task, resuming a suspended snapshot, cancelling, or simply asking for a direct reply.

Rules:
- Read the previous assistant reply carefully. The user's short answer may be replying to that exact proposal or question.
- If Active flow summary includes a pending hint or waiting question, short replies like "1", "2", "A", "B", "确认", "需要", or "好的" usually mean the user is continuing that flow unless they clearly switch tasks.
- Prefer "continue_active" when the user is plausibly answering the current active flow.
- If the user clearly corrects the entity/domain, you must output "start_new", not "continue_active".
- Examples of forced switch: "不是交易员，是策略", "不是这个", "换个任务", "I mean the strategy, not the trader".
- If the user refers to a suspended task and one snapshot clearly matches, use "resume_snapshot".
- If the user cancels the current task, use "cancel".
- If the user only greets, thanks, chats, or asks for explanation without changing state, use "instant_reply".
- Short greetings or acknowledgements like "你好", "hi", "hello", "谢谢", "收到", "好的" should default to "instant_reply" unless they clearly contain task data.
- You may set target_skill when intent=start_new and the next task is clear.

Return JSON with this exact shape:
{"intent":"continue_active|start_new|resume_snapshot|cancel|instant_reply","target_snapshot_id":"","target_skill":"","extracted_fields":{},"need_planner_help":false,"reason":"","confidence":0.0}`)
	}

	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nPrevious assistant reply:\n%s\n\nManagement skill summary:\n%s\n\nManagement domain primer:\n%s\n\nCurrent reference summary:\n%s\n\nActive flow summary:\n%s\n\nSuspended snapshots JSON:\n%s\n\nRecent conversation:\n%s\n",
		lang,
		text,
		defaultIfEmpty(previousAssistantReply, "(empty)"),
		defaultIfEmpty(managementSummary, "(empty)"),
		defaultIfEmpty(buildManagementDomainPrimer(lang), "(empty)"),
		currentRefs,
		activeFlowSummary,
		defaultIfEmpty(string(snapshotJSON), "[]"),
		recentConversation,
	)

	return systemPrompt, userPrompt
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

func (a *Agent) executeLLMSkillRoute(storeUserID string, userID int64, lang, text string, decision llmSkillRouteDecision) (skillOutcome, bool) {
	session := skillSession{Name: decision.Skill, Action: decision.Action, Phase: "collecting"}
	applyExtractedFieldsToSkillSession(&session, decision.ExtractedFields, "llm_router")
	return a.executeAtomicSkillTaskOutcomeWithSession(storeUserID, userID, lang, text, session, nil)
}

func applyExtractedFieldsToSkillSession(session *skillSession, values map[string]any, source string) {
	if session == nil || len(values) == 0 {
		return
	}
	ensureSkillFields(session)
	for key, raw := range values {
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" {
			continue
		}
		switch key {
		case "target_ref_id":
			if session.TargetRef == nil {
				session.TargetRef = &EntityReference{}
			}
			session.TargetRef.ID = value
			if source != "" {
				session.TargetRef.Source = source
			}
		case "target_ref_name":
			if session.TargetRef == nil {
				session.TargetRef = &EntityReference{}
			}
			session.TargetRef.Name = value
			if source != "" {
				session.TargetRef.Source = source
			}
		default:
			setField(session, key, value)
		}
	}
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
	return a.hasActiveSkillSession(userID) || hasActiveWorkflowSession(a.getWorkflowSession(userID)) || hasActiveExecutionState(a.getExecutionState(userID))
}

func (a *Agent) clearAnyActiveContext(userID int64) bool {
	cleared := false
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
