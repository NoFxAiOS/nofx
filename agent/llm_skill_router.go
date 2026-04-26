package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

type llmSkillRouteDecision struct {
	Intent           string  `json:"intent,omitempty"`
	TargetSnapshotID string  `json:"target_snapshot_id,omitempty"`
	ContextSwitch    bool    `json:"context_switch,omitempty"`
	Confidence       float64 `json:"confidence,omitempty"`
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
		return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
	case "cancel":
		a.clearPendingProposalSession(userID)
		if a.hasAnyActiveContext(userID) {
			a.clearActiveSkillSession(userID)
			a.clearAnyActiveContext(userID)
			return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
		}
		return "", false, nil
	case "resume_snapshot":
		a.clearPendingProposalSession(userID)
		if a.tryRestoreSuspendedTaskAfterSwitch(userID, text, decision.TargetSnapshotID) {
			return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
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
		if a.suspendAndTryRestoreSuspendedTask(userID, lang, text, decision.TargetSnapshotID) {
			return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
		}
	}

	return a.tryMinimalBrain(ctx, storeUserID, userID, lang, text, onEvent)
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
	if decision.Confidence < 0 {
		decision.Confidence = 0
	}
	if decision.Confidence > 1 {
		decision.Confidence = 1
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
	recentConversation := a.buildRecentConversationContext(userID, text)
	if strings.TrimSpace(recentConversation) == "" {
		recentConversation = "(empty)"
	}

	activeFlowSummary := buildTopLevelActiveFlowSummary(lang, activeSkill, activeTask, hasActiveTask, activeWorkflow, activeExec, pendingProposal, hasPendingProposal)
	if strings.TrimSpace(activeFlowSummary) == "" {
		activeFlowSummary = "none"
	}

	systemPrompt := prependNOFXiAdvisorPreamble(`You are the lightweight topic router for NOFXi.
Return JSON only.

Your only job is to decide whether the current user turn continues the current topic/state, starts a new topic, resumes a suspended topic, cancels the current topic, or is a direct conversational reply.
Do not perform business intent recognition. Do not choose skills, actions, tasks, or fields. The central brain will do that after you return.

Valid intents:
- "continue_active": the user is still working on the current active flow
- "start_new": the user is starting or switching to a new task
- "resume_snapshot": the user wants to resume one suspended snapshot
- "cancel": the user wants to cancel the current active flow
- "instant_reply": the user is greeting, chatting, thanking, or asking for a direct explanation without changing task state

Rules:
- Read the previous assistant reply carefully. The user's short answer may be replying to that exact proposal or question.
- If Active flow summary includes a pending hint or waiting question, short replies like "1", "2", "A", "B", "确认", "需要", or "好的" usually mean the user is continuing that flow unless they clearly switch tasks.
- If the user is clearly answering the previous question, prefer "continue_active".
- If the user clearly corrects the entity/domain, you must output "start_new", not "continue_active".
- If the user explicitly refers to a suspended task like "刚才那个", "恢复刚才那个", choose "resume_snapshot" and fill target_snapshot_id.
- If the user is only greeting, thanking, social chatting, or asking a concept question without changing task state, choose "instant_reply".
- Set context_switch=true when the user is opening a new topic/task and prior current references or suspended snapshots should not be used to fill business fields. Set context_switch=false when the user intentionally relies on previous context.
- Do not hallucinate snapshot ids; only use those disclosed in Suspended snapshots JSON.

Return JSON with this exact shape:
{"intent":"continue_active|start_new|resume_snapshot|cancel|instant_reply","target_snapshot_id":"","context_switch":false,"confidence":0.0}`)

	if strings.TrimSpace(activeSkill.Name) != "" || hasActiveTask || hasPendingProposal {
		systemPrompt = prependNOFXiAdvisorPreamble(`You are the one-pass topic gateway for NOFXi.
Return JSON only.

Your only job is topic-state routing: continuing the active flow, switching to a new topic, resuming a suspended snapshot, cancelling, or giving a direct conversational reply.
Do not perform business intent recognition. Do not choose skills, actions, tasks, or fields. The central brain will do that after you return.

Rules:
- Read the previous assistant reply carefully. The user's short answer may be replying to that exact proposal or question.
- If Active flow summary includes a pending hint or waiting question, short replies like "1", "2", "A", "B", "确认", "需要", or "好的" usually mean the user is continuing that flow unless they clearly switch tasks.
- Prefer "continue_active" when the user is plausibly answering the current active flow.
- If the user asks a read-only management query while an active flow is open, output intent "start_new"; the central brain will choose the query tool.
- If the user starts a multi-step, multi-domain, batch, or condition-based management request while an active flow is open, output intent "start_new"; the central brain will decompose it.
- If the user clearly corrects the entity/domain, you must output "start_new", not "continue_active".
- Examples of forced switch: "不是交易员，是策略", "不是这个", "换个任务", "I mean the strategy, not the trader".
- If the user refers to a suspended task and one snapshot clearly matches, use "resume_snapshot".
- If the user cancels the current task, use "cancel".
- If the user only greets, thanks, chats, or asks for explanation without changing state, use "instant_reply".
- Short greetings or acknowledgements like "你好", "hi", "hello", "谢谢", "收到", "好的" should default to "instant_reply" unless they clearly contain task data.
- Do not hallucinate snapshot ids; only use those disclosed in Suspended snapshots JSON.

Return JSON with this exact shape:
{"intent":"continue_active|start_new|resume_snapshot|cancel|instant_reply","target_snapshot_id":"","context_switch":false,"confidence":0.0}`)
	}

	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nPrevious assistant reply:\n%s\n\nCurrent reference summary:\n%s\n\nActive flow summary:\n%s\n\nSuspended snapshots JSON:\n%s\n\nRecent conversation:\n%s\n",
		lang,
		text,
		defaultIfEmpty(previousAssistantReply, "(empty)"),
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
