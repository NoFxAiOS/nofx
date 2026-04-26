package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nofx/mcp"
)

const (
	workflowTaskPending   = "pending"
	workflowTaskRunning   = "running"
	workflowTaskCompleted = "completed"
	workflowTaskFailed    = "failed"
)

type WorkflowTask struct {
	ID        string   `json:"id,omitempty"`
	Skill     string   `json:"skill,omitempty"`
	Action    string   `json:"action,omitempty"`
	Request   string   `json:"request,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
	Status    string   `json:"status,omitempty"`
	Error     string   `json:"error,omitempty"`
}

type WorkflowSession struct {
	UserID          int64          `json:"user_id"`
	OriginalRequest string         `json:"original_request,omitempty"`
	Tasks           []WorkflowTask `json:"tasks,omitempty"`
	UpdatedAt       string         `json:"updated_at,omitempty"`
}

type workflowDecomposition struct {
	Tasks []WorkflowTask `json:"tasks"`
}

func workflowSessionConfigKey(userID int64) string {
	return fmt.Sprintf("agent_workflow_session_%d", userID)
}

func normalizeWorkflowSession(session WorkflowSession) WorkflowSession {
	session.OriginalRequest = strings.TrimSpace(session.OriginalRequest)
	normalized := make([]WorkflowTask, 0, len(session.Tasks))
	for i, task := range session.Tasks {
		task.ID = strings.TrimSpace(task.ID)
		if task.ID == "" {
			task.ID = fmt.Sprintf("task_%d", i+1)
		}
		task.Skill = strings.TrimSpace(task.Skill)
		task.Action = normalizeAtomicSkillAction(task.Skill, task.Action)
		task.Request = strings.TrimSpace(task.Request)
		task.DependsOn = cleanStringList(task.DependsOn)
		task.Status = strings.TrimSpace(task.Status)
		if task.Status == "" {
			task.Status = workflowTaskPending
		}
		task.Error = strings.TrimSpace(task.Error)
		if task.Skill == "" || task.Action == "" || task.Request == "" {
			continue
		}
		normalized = append(normalized, task)
	}
	session.Tasks = normalized
	if len(session.Tasks) == 0 {
		return WorkflowSession{}
	}
	if session.UpdatedAt == "" {
		session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return session
}

func (a *Agent) getWorkflowSession(userID int64) WorkflowSession {
	if a.store == nil {
		return WorkflowSession{}
	}
	raw, err := a.store.GetSystemConfig(workflowSessionConfigKey(userID))
	if err != nil || strings.TrimSpace(raw) == "" {
		return WorkflowSession{}
	}
	var session WorkflowSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return WorkflowSession{}
	}
	return normalizeWorkflowSession(session)
}

func (a *Agent) saveWorkflowSession(userID int64, session WorkflowSession) {
	if a.store == nil {
		return
	}
	session = normalizeWorkflowSession(session)
	if len(session.Tasks) == 0 {
		_ = a.store.SetSystemConfig(workflowSessionConfigKey(userID), "")
		return
	}
	session.UserID = userID
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(session)
	if err != nil {
		return
	}
	_ = a.store.SetSystemConfig(workflowSessionConfigKey(userID), string(data))
}

func (a *Agent) clearWorkflowSession(userID int64) {
	if a.store == nil {
		return
	}
	_ = a.store.SetSystemConfig(workflowSessionConfigKey(userID), "")
}

func hasActiveWorkflowSession(session WorkflowSession) bool {
	if len(session.Tasks) == 0 {
		return false
	}
	for _, task := range session.Tasks {
		if task.Status == workflowTaskPending || task.Status == workflowTaskRunning {
			return true
		}
	}
	return false
}

func nextRunnableWorkflowTask(session WorkflowSession) (WorkflowTask, int, bool) {
	for i, task := range session.Tasks {
		if task.Status != workflowTaskPending && task.Status != workflowTaskRunning {
			continue
		}
		depsReady := true
		for _, dep := range task.DependsOn {
			ok := false
			for _, candidate := range session.Tasks {
				if candidate.ID == dep && candidate.Status == workflowTaskCompleted {
					ok = true
					break
				}
			}
			if !ok {
				depsReady = false
				break
			}
		}
		if depsReady {
			return task, i, true
		}
	}
	return WorkflowTask{}, -1, false
}

func supportedWorkflowSkill(skill, action string) bool {
	skill = strings.TrimSpace(skill)
	action = normalizeAtomicSkillAction(skill, action)
	if skill == "" || action == "" {
		return false
	}
	if _, ok := getSkillDAG(skill, action); ok {
		return true
	}
	if def, ok := getSkillDefinition(skill); ok {
		if _, ok := def.Actions[action]; ok {
			return true
		}
	}
	switch skill {
	case "trader_management", "strategy_management", "model_management", "exchange_management":
		if action == "query_running" {
			return true
		}
	}
	return false
}

func (a *Agent) tryWorkflowIntent(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	if session := a.getWorkflowSession(userID); hasActiveWorkflowSession(session) {
		return a.handleWorkflowSession(ctx, storeUserID, userID, lang, text, session, onEvent)
	}

	decomposition, err := a.decomposeWorkflowIntent(ctx, userID, lang, text)
	if err != nil || len(decomposition.Tasks) <= 1 {
		return "", false, err
	}
	session := WorkflowSession{
		UserID:          userID,
		OriginalRequest: text,
		Tasks:           decomposition.Tasks,
	}
	a.saveWorkflowSession(userID, session)
	return a.handleWorkflowSession(ctx, storeUserID, userID, lang, text, session, onEvent)
}

func (a *Agent) executeWorkflowDecomposition(ctx context.Context, storeUserID string, userID int64, lang, text string, decomposition workflowDecomposition, onEvent func(event, data string)) (string, bool, error) {
	if len(decomposition.Tasks) <= 1 {
		return "", false, nil
	}
	session := WorkflowSession{
		UserID:          userID,
		OriginalRequest: text,
		Tasks:           decomposition.Tasks,
	}
	a.saveWorkflowSession(userID, session)
	return a.handleWorkflowSession(ctx, storeUserID, userID, lang, text, session, onEvent)
}

func (a *Agent) handleWorkflowSession(ctx context.Context, storeUserID string, userID int64, lang, text string, session WorkflowSession, onEvent func(event, data string)) (string, bool, error) {
	if isExplicitFlowAbort(text) {
		a.clearSkillSession(userID)
		a.clearWorkflowSession(userID)
		return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
	}

	if activeSkill := a.getSkillSession(userID); strings.TrimSpace(activeSkill.Name) != "" {
		decision, extraction := a.resolveSkillSessionTurn(ctx, userID, lang, text, activeSkill)
		switch decision.Intent {
		case "cancel":
			a.clearSkillSession(userID)
			a.clearWorkflowSession(userID)
			return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
		case "instant_reply":
			return a.replyToActiveFlowInstantReply(ctx, userID, lang, text, onEvent), true, nil
		case "resume_snapshot", "start_new":
			if shouldSuspendInterruptedTask(text) || decision.Intent == "resume_snapshot" {
				answer, handled, err := a.handoffFromActiveFlow(ctx, storeUserID, userID, lang, text, decision.TargetSnapshotID, onEvent)
				return answer, handled, err
			}
			a.clearSkillSession(userID)
			a.clearWorkflowSession(userID)
			return "", false, nil
		case "continue_active":
			if extraction.Intent == "continue" {
				a.applyLLMExtractionToSkillSession(storeUserID, &activeSkill, extraction, lang, text)
				a.saveSkillSession(userID, activeSkill)
			}
		}
		answer, handled := a.executeAtomicSkillTask(storeUserID, userID, lang, text, activeSkill.Name, activeSkill.Action, onEvent)
		if !handled {
			return "", false, nil
		}
		a.recordSkillInteraction(userID, text, answer)
		session = a.getWorkflowSession(userID)
		if hasActiveWorkflowSession(session) && strings.TrimSpace(a.getSkillSession(userID).Name) == "" {
			session = markCurrentWorkflowTask(session, workflowTaskCompleted, "")
			a.saveWorkflowSession(userID, session)
			if final, done, err := a.maybeAdvanceWorkflow(ctx, storeUserID, userID, lang, session, onEvent); done || err != nil {
				if final != "" && answer != "" {
					return answer + "\n\n" + final, true, err
				}
				if answer != "" {
					return answer, true, err
				}
				return final, true, err
			}
		}
		return answer, true, nil
	}

	if decision := a.classifyWorkflowSessionInput(ctx, userID, lang, session, text); decision.Intent != "" && decision.Intent != "continue_active" {
		switch decision.Intent {
		case "cancel":
			a.clearWorkflowSession(userID)
			return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
		case "instant_reply":
			return a.replyToActiveFlowInstantReply(ctx, userID, lang, text, onEvent), true, nil
		case "resume_snapshot", "start_new":
			if shouldSuspendInterruptedTask(text) || decision.Intent == "resume_snapshot" {
				answer, handled, err := a.handoffFromActiveFlow(ctx, storeUserID, userID, lang, text, decision.TargetSnapshotID, onEvent)
				return answer, handled, err
			}
			a.clearWorkflowSession(userID)
			return "", false, nil
		}
	}

	return a.maybeAdvanceWorkflow(ctx, storeUserID, userID, lang, session, onEvent)
}

func (a *Agent) classifyWorkflowSessionInput(ctx context.Context, userID int64, lang string, session WorkflowSession, text string) unifiedFlowDecision {
	text = strings.TrimSpace(text)
	if text == "" {
		return unifiedFlowDecision{Intent: "continue_active"}
	}
	if isExplicitFlowAbort(text) {
		return unifiedFlowDecision{Intent: "cancel"}
	}
	if isInstantDirectReplyText(text) {
		return unifiedFlowDecision{Intent: "instant_reply"}
	}
	if a == nil || a.aiClient == nil {
		if looksLikeNewTopLevelIntent(text) && !strings.EqualFold(text, strings.TrimSpace(session.OriginalRequest)) {
			return unifiedFlowDecision{Intent: "start_new"}
		}
		return unifiedFlowDecision{Intent: "continue_active"}
	}
	currentTask, _, _ := nextRunnableWorkflowTask(session)
	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	flowContext := fmt.Sprintf(
		"Workflow original request: %s\nCurrent runnable task: %s / %s / %s\nWorkflow tasks JSON: %s",
		session.OriginalRequest,
		currentTask.Skill,
		currentTask.Action,
		currentTask.Request,
		mustMarshalJSON(session.Tasks),
	)
	state := a.getExecutionState(userID)
	systemPrompt, userPrompt := buildActiveFlowClassifierPrompt(
		lang,
		"workflow_session",
		flowContext,
		text,
		recentConversationCtx,
		state.CurrentReferences,
		a.SnapshotManager(userID).List(),
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
		return unifiedFlowDecision{}
	}
	return unifiedFlowDecisionFromIntent(parseActiveFlowIntentDecision(raw), "")
}

func (a *Agent) maybeAdvanceWorkflow(ctx context.Context, storeUserID string, userID int64, lang string, session WorkflowSession, onEvent func(event, data string)) (string, bool, error) {
	task, index, ok := nextRunnableWorkflowTask(session)
	if !ok {
		summary := a.generateWorkflowSummary(ctx, userID, lang, session)
		a.clearWorkflowSession(userID)
		if summary == "" {
			if lang == "zh" {
				summary = "已完成当前任务流。"
			} else {
				summary = "Completed the current workflow."
			}
		}
		if onEvent != nil {
			onEvent(StreamEventPlan, summary)
			emitStreamText(onEvent, summary)
		}
		return summary, true, nil
	}

	session.Tasks[index].Status = workflowTaskRunning
	a.saveWorkflowSession(userID, session)
	taskSession := skillSession{Name: task.Skill, Action: task.Action, Phase: "collecting"}
	a.saveSkillSession(userID, taskSession)

	if onEvent != nil {
		onEvent(StreamEventPlan, a.formatWorkflowStatus(lang, session))
		onEvent(StreamEventTool, "workflow:"+task.Skill+":"+task.Action)
	}

	answer, handled := a.executeAtomicSkillTask(storeUserID, userID, lang, task.Request, task.Skill, task.Action, onEvent)
	if !handled {
		session.Tasks[index].Status = workflowTaskFailed
		session.Tasks[index].Error = "task_not_handled"
		a.saveWorkflowSession(userID, session)
		return "", false, nil
	}
	a.recordSkillInteraction(userID, task.Request, answer)

	if strings.TrimSpace(a.getSkillSession(userID).Name) == "" {
		session = a.getWorkflowSession(userID)
		session = markCurrentWorkflowTask(session, workflowTaskCompleted, "")
		a.saveWorkflowSession(userID, session)
		if more, ok, err := a.maybeAdvanceWorkflow(ctx, storeUserID, userID, lang, session, onEvent); ok || err != nil {
			if answer != "" && more != "" {
				return answer + "\n\n" + more, true, err
			}
			if answer != "" {
				return answer, true, err
			}
			return more, true, err
		}
	}
	return answer, true, nil
}

func markCurrentWorkflowTask(session WorkflowSession, status, errMsg string) WorkflowSession {
	for i := range session.Tasks {
		if session.Tasks[i].Status == workflowTaskRunning {
			session.Tasks[i].Status = status
			session.Tasks[i].Error = strings.TrimSpace(errMsg)
			return session
		}
	}
	return session
}

func (a *Agent) formatWorkflowStatus(lang string, session WorkflowSession) string {
	parts := make([]string, 0, len(session.Tasks))
	for _, task := range session.Tasks {
		label := task.Request
		if label == "" {
			label = task.Skill + ":" + task.Action
		}
		switch task.Status {
		case workflowTaskCompleted:
			label = "✓ " + label
		case workflowTaskRunning:
			label = "→ " + label
		default:
			label = "· " + label
		}
		parts = append(parts, label)
	}
	if lang == "zh" {
		return "任务流：" + strings.Join(parts, " | ")
	}
	return "Workflow: " + strings.Join(parts, " | ")
}

func (a *Agent) generateWorkflowSummary(ctx context.Context, userID int64, lang string, session WorkflowSession) string {
	completed := make([]string, 0, len(session.Tasks))
	for _, task := range session.Tasks {
		if task.Status == workflowTaskCompleted {
			completed = append(completed, task.Request)
		}
	}
	if len(completed) == 0 {
		return ""
	}
	if a.aiClient == nil {
		if lang == "zh" {
			return "已完成这些任务：" + strings.Join(completed, "；")
		}
		return "Completed these tasks: " + strings.Join(completed, "; ")
	}
	stageCtx, cancel := withPlannerStageTimeout(ctx, directReplyTimeout)
	defer cancel()
	systemPrompt := `You are summarizing a finished workflow for NOFXi.
Return one short user-facing summary in the user's language.
Do not mention internal DAG, scheduler, or JSON.
` + cleanUserFacingReplyInstruction
	userPrompt := fmt.Sprintf("Language: %s\nOriginal request: %s\nCompleted tasks:\n- %s", lang, session.OriginalRequest, strings.Join(completed, "\n- "))
	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	if err != nil {
		if lang == "zh" {
			return "已完成这些任务：" + strings.Join(completed, "；")
		}
		return "Completed these tasks: " + strings.Join(completed, "; ")
	}
	return strings.TrimSpace(raw)
}

func (a *Agent) decomposeWorkflowIntent(ctx context.Context, userID int64, lang, text string) (workflowDecomposition, error) {
	if !looksLikeMultiTaskIntent(text) {
		return workflowDecomposition{}, nil
	}
	if a.aiClient != nil {
		if dec, err := a.decomposeWorkflowIntentWithLLM(ctx, userID, lang, text); err == nil && len(dec.Tasks) > 1 {
			return dec, nil
		}
	}
	return a.decomposeWorkflowIntentFallback(text), nil
}

func looksLikeMultiTaskIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	connectors := []string{"，", ",", "然后", "再", "并且", "并", "同时", "and", "then"}
	count := 0
	for _, c := range connectors {
		if strings.Contains(lower, c) {
			count++
		}
	}
	if count > 0 {
		return true
	}
	if looksLikeCompoundStrategyIntent(text) || looksLikeCompoundTraderIntent(text) ||
		looksLikeCompoundModelIntent(text) || looksLikeCompoundExchangeIntent(text) {
		return true
	}
	return false
}

func looksLikeCompoundStrategyIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !hasExplicitManagementDomainCue(text, "strategy") {
		return false
	}
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "加一个", "create", "new"})
	hasConfigUpdate := containsAny(lower, []string{"修改", "更新", "参数", "配置", "prompt", "提示词", "改成", "改为"})
	hasLifecycle := containsAny(lower, []string{"激活", "activate", "复制", "duplicate", "删除", "删了", "删掉", "delete"})
	hasMetaUpdate := containsAny(lower, []string{"发布", "公开", "可见", "描述", "改成", "改为"})
	return (hasCreate && (hasConfigUpdate || hasLifecycle || hasMetaUpdate)) ||
		(hasConfigUpdate && hasLifecycle)
}

func looksLikeCompoundTraderIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !(hasExplicitManagementDomainCue(text, "trader") || hasExplicitCreateIntentForDomain(text, "trader")) {
		return false
	}
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"})
	hasBindingsOrConfig := containsAny(lower, []string{"修改", "更新", "换模型", "换交易所", "换策略", "切换模型", "切换交易所", "切换策略", "扫描间隔", "全仓", "逐仓", "竞技场"})
	hasLifecycle := containsAny(lower, []string{"启动", "开始", "start", "停止", "stop"})
	return (hasCreate && (hasBindingsOrConfig || hasLifecycle)) ||
		(hasBindingsOrConfig && hasLifecycle)
}

func looksLikeCompoundModelIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !hasExplicitManagementDomainCue(text, "model") {
		return false
	}
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"})
	hasConfig := containsAny(lower, []string{"修改", "更新", "改", "接口地址", "模型名", "启用", "禁用", "api key"})
	hasLifecycle := containsAny(lower, []string{"启用", "禁用", "enable", "disable", "删除", "删了", "删掉", "delete"})
	return (hasCreate && (hasConfig || hasLifecycle)) || (hasConfig && hasLifecycle)
}

func looksLikeCompoundExchangeIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !hasExplicitManagementDomainCue(text, "exchange") {
		return false
	}
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"})
	hasConfig := containsAny(lower, []string{"修改", "更新", "改", "账户名", "api key", "secret", "passphrase", "钱包", "启用", "禁用"})
	hasLifecycle := containsAny(lower, []string{"启用", "禁用", "enable", "disable", "删除", "删了", "删掉", "delete"})
	return (hasCreate && (hasConfig || hasLifecycle)) || (hasConfig && hasLifecycle)
}

func (a *Agent) decomposeWorkflowIntentWithLLM(ctx context.Context, userID int64, lang, text string) (workflowDecomposition, error) {
	stageCtx, cancel := withPlannerStageTimeout(ctx, directReplyTimeout)
	defer cancel()
	systemPrompt := `You decompose one NOFXi user request into a small task graph for execution.
Return JSON only. No markdown.
Only use these skills: trader_management, strategy_management, model_management, exchange_management.
Only use one atomic action per task.
You are the action decomposition layer. Split complex requests into atomic management steps and decide dependencies.
Each task must include:
- id
- skill
- action
- request
- depends_on (array, may be empty)
Rules:
- Prefer atomic actions such as create, update_bindings, configure_strategy, configure_exchange, configure_model, update_status, update_endpoint, update_config, update_prompt, activate, duplicate, start, stop, delete, query_list, query_detail.
- If one request contains create plus follow-up edits in the same skill, split them into multiple tasks.
- If later tasks need an entity created earlier, make the dependency explicit in depends_on.
- Keep each request user-readable and self-contained enough for a single skill handler to execute.
- Do not merge two actions into one task.
- If the request is effectively a single task, return one task only.`
	userPrompt := fmt.Sprintf("Language: %s\nUser request: %s", lang, text)
	if skillContext := buildManagementSkillRoutingContext(lang); skillContext != "" {
		userPrompt += "\n\n" + skillContext
	}
	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	if err != nil {
		return workflowDecomposition{}, err
	}
	return parseWorkflowDecomposition(raw)
}

func parseWorkflowDecomposition(raw string) (workflowDecomposition, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var out workflowDecomposition
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		out = normalizeWorkflowDecomposition(out)
		return out, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err == nil {
			out = normalizeWorkflowDecomposition(out)
			return out, nil
		}
	}
	return workflowDecomposition{}, fmt.Errorf("invalid workflow json")
}

func normalizeWorkflowDecomposition(out workflowDecomposition) workflowDecomposition {
	normalized := make([]WorkflowTask, 0, len(out.Tasks))
	for i, task := range out.Tasks {
		task.ID = strings.TrimSpace(task.ID)
		if task.ID == "" {
			task.ID = fmt.Sprintf("task_%d", i+1)
		}
		task.Skill = strings.TrimSpace(task.Skill)
		task.Action = normalizeAtomicSkillAction(task.Skill, task.Action)
		task.Request = strings.TrimSpace(task.Request)
		task.DependsOn = cleanStringList(task.DependsOn)
		if !supportedWorkflowSkill(task.Skill, task.Action) || task.Request == "" {
			continue
		}
		task.Status = workflowTaskPending
		normalized = append(normalized, task)
	}
	out.Tasks = normalized
	return out
}

func (a *Agent) decomposeWorkflowIntentFallback(text string) workflowDecomposition {
	segments := splitWorkflowSegments(text)
	tasks := make([]WorkflowTask, 0, len(segments))
	nextID := 1
	for _, segment := range segments {
		prevSkill := ""
		if len(tasks) > 0 {
			prevSkill = tasks[len(tasks)-1].Skill
		}
		compound := classifyCompoundWorkflowTasksWithContext(segment, prevSkill)
		if len(compound) == 0 {
			task, ok := classifyWorkflowTaskWithContext(segment, prevSkill)
			if !ok {
				continue
			}
			compound = []WorkflowTask{task}
		}
		for i := range compound {
			compound[i].ID = fmt.Sprintf("task_%d", nextID)
			compound[i].Status = workflowTaskPending
			if len(tasks) > 0 && len(compound[i].DependsOn) == 0 {
				compound[i].DependsOn = []string{tasks[len(tasks)-1].ID}
			}
			if i > 0 {
				compound[i].DependsOn = []string{compound[i-1].ID}
			}
			tasks = append(tasks, compound[i])
			nextID++
		}
	}
	return workflowDecomposition{Tasks: tasks}
}

func classifyCompoundWorkflowTasksWithContext(text, previousSkill string) []WorkflowTask {
	if tasks := classifyCompoundWorkflowTasks(text); len(tasks) > 1 {
		return tasks
	}
	switch strings.TrimSpace(previousSkill) {
	case "strategy_management":
		return classifyContextualStrategyWorkflowTasks(text)
	case "trader_management":
		return classifyContextualTraderWorkflowTasks(text)
	}
	return nil
}

func classifyCompoundWorkflowTasks(text string) []WorkflowTask {
	segment := strings.TrimSpace(text)
	if segment == "" {
		return nil
	}

	if tasks := classifyCompoundStrategyWorkflowTasks(segment); len(tasks) > 1 {
		return tasks
	}
	if tasks := classifyCompoundTraderWorkflowTasks(segment); len(tasks) > 1 {
		return tasks
	}
	if tasks := classifyCompoundModelWorkflowTasks(segment); len(tasks) > 1 {
		return tasks
	}
	if tasks := classifyCompoundExchangeWorkflowTasks(segment); len(tasks) > 1 {
		return tasks
	}
	return nil
}

func classifyContextualStrategyWorkflowTasks(text string) []WorkflowTask {
	lower := strings.ToLower(strings.TrimSpace(text))
	hasConfig := containsAny(lower, []string{"修改", "更新", "参数", "配置", "prompt", "提示词", "改成", "改为"})
	hasActivate := containsAny(lower, []string{"激活", "activate"})
	hasDuplicate := containsAny(lower, []string{"复制", "duplicate"})
	if !hasConfig && !hasActivate && !hasDuplicate {
		return nil
	}
	var tasks []WorkflowTask
	if hasConfig {
		action := "update_config"
		if containsAny(lower, []string{"prompt", "提示词"}) {
			action = "update_prompt"
		}
		tasks = append(tasks, WorkflowTask{Skill: "strategy_management", Action: action, Request: text})
	}
	if hasActivate {
		tasks = append(tasks, WorkflowTask{Skill: "strategy_management", Action: "activate", Request: text})
	}
	if hasDuplicate {
		tasks = append(tasks, WorkflowTask{Skill: "strategy_management", Action: "duplicate", Request: text})
	}
	if len(tasks) == 0 {
		return nil
	}
	return tasks
}

func classifyContextualTraderWorkflowTasks(text string) []WorkflowTask {
	lower := strings.ToLower(strings.TrimSpace(text))
	hasUpdate := containsAny(lower, []string{"修改", "更新", "换模型", "换交易所", "换策略", "切换模型", "切换交易所", "切换策略", "扫描间隔", "全仓", "逐仓", "竞技场"})
	hasStart := containsAny(lower, []string{"启动", "开始", "run", "start"})
	hasStop := containsAny(lower, []string{"停止", "停掉", "stop", "pause"})
	if !hasUpdate && !hasStart && !hasStop {
		return nil
	}
	var tasks []WorkflowTask
	if hasUpdate {
		tasks = append(tasks, WorkflowTask{Skill: "trader_management", Action: "update_bindings", Request: text})
	}
	if hasStart {
		tasks = append(tasks, WorkflowTask{Skill: "trader_management", Action: "start", Request: text})
	}
	if hasStop {
		tasks = append(tasks, WorkflowTask{Skill: "trader_management", Action: "stop", Request: text})
	}
	if len(tasks) == 0 {
		return nil
	}
	return tasks
}

func classifyWorkflowTaskWithContext(text, previousSkill string) (WorkflowTask, bool) {
	if task, ok := classifyWorkflowTask(text); ok {
		return task, true
	}
	switch strings.TrimSpace(previousSkill) {
	case "strategy_management":
		if tasks := classifyContextualStrategyWorkflowTasks(text); len(tasks) > 0 {
			return tasks[0], true
		}
	case "trader_management":
		if tasks := classifyContextualTraderWorkflowTasks(text); len(tasks) > 0 {
			return tasks[0], true
		}
	}
	return WorkflowTask{}, false
}

func classifyCompoundStrategyWorkflowTasks(text string) []WorkflowTask {
	if !hasExplicitManagementDomainCue(text, "strategy") {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "加一个", "create", "new"})
	hasConfig := containsAny(lower, []string{"修改", "更新", "参数", "配置", "prompt", "提示词", "改成", "改为"})
	hasActivate := containsAny(lower, []string{"激活", "activate"})
	hasDuplicate := containsAny(lower, []string{"复制", "duplicate"})

	if !hasCreate && !hasConfig && !hasActivate && !hasDuplicate {
		return nil
	}

	var tasks []WorkflowTask
	if hasCreate {
		tasks = append(tasks, WorkflowTask{Skill: "strategy_management", Action: "create", Request: text})
	}
	if hasConfig {
		action := "update_config"
		if containsAny(lower, []string{"prompt", "提示词"}) {
			action = "update_prompt"
		}
		tasks = append(tasks, WorkflowTask{Skill: "strategy_management", Action: action, Request: text})
	}
	if hasActivate {
		tasks = append(tasks, WorkflowTask{Skill: "strategy_management", Action: "activate", Request: text})
	}
	if hasDuplicate {
		tasks = append(tasks, WorkflowTask{Skill: "strategy_management", Action: "duplicate", Request: text})
	}
	if len(tasks) <= 1 {
		return nil
	}
	return tasks
}

func classifyCompoundTraderWorkflowTasks(text string) []WorkflowTask {
	if !(hasExplicitManagementDomainCue(text, "trader") || hasExplicitCreateIntentForDomain(text, "trader")) {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"})
	hasUpdate := containsAny(lower, []string{"修改", "更新", "换模型", "换交易所", "换策略", "切换模型", "切换交易所", "切换策略", "扫描间隔", "全仓", "逐仓", "竞技场"})
	hasStart := containsAny(lower, []string{"启动", "开始", "run", "start"})
	hasStop := containsAny(lower, []string{"停止", "停掉", "stop", "pause"})

	var tasks []WorkflowTask
	if hasCreate {
		tasks = append(tasks, WorkflowTask{Skill: "trader_management", Action: "create", Request: text})
	}
	if hasUpdate {
		tasks = append(tasks, WorkflowTask{Skill: "trader_management", Action: "update_bindings", Request: text})
	}
	if hasStart {
		tasks = append(tasks, WorkflowTask{Skill: "trader_management", Action: "start", Request: text})
	}
	if hasStop {
		tasks = append(tasks, WorkflowTask{Skill: "trader_management", Action: "stop", Request: text})
	}
	if len(tasks) <= 1 {
		return nil
	}
	return tasks
}

func classifyCompoundModelWorkflowTasks(text string) []WorkflowTask {
	if !hasExplicitManagementDomainCue(text, "model") {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"})
	hasConfig := containsAny(lower, []string{"修改", "更新", "改", "接口地址", "模型名", "api key"})
	hasStatus := containsAny(lower, []string{"启用", "禁用", "enable", "disable"})

	var tasks []WorkflowTask
	if hasCreate {
		tasks = append(tasks, WorkflowTask{Skill: "model_management", Action: "create", Request: text})
	}
	if hasConfig {
		action := "update_endpoint"
		tasks = append(tasks, WorkflowTask{Skill: "model_management", Action: action, Request: text})
	}
	if hasStatus {
		tasks = append(tasks, WorkflowTask{Skill: "model_management", Action: "update_status", Request: text})
	}
	if len(tasks) <= 1 {
		return nil
	}
	return tasks
}

func classifyCompoundExchangeWorkflowTasks(text string) []WorkflowTask {
	if !hasExplicitManagementDomainCue(text, "exchange") {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	hasCreate := containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"})
	hasConfig := containsAny(lower, []string{"修改", "更新", "改", "账户名", "api key", "secret", "passphrase", "钱包"})
	hasStatus := containsAny(lower, []string{"启用", "禁用", "enable", "disable"})

	var tasks []WorkflowTask
	if hasCreate {
		tasks = append(tasks, WorkflowTask{Skill: "exchange_management", Action: "create", Request: text})
	}
	if hasConfig {
		tasks = append(tasks, WorkflowTask{Skill: "exchange_management", Action: "update_name", Request: text})
	}
	if hasStatus {
		tasks = append(tasks, WorkflowTask{Skill: "exchange_management", Action: "update_status", Request: text})
	}
	if len(tasks) <= 1 {
		return nil
	}
	return tasks
}

func splitWorkflowSegments(text string) []string {
	parts := []string{strings.TrimSpace(text)}
	separators := []string{"，", ",", "然后", "再", "并且", "同时", " and then ", " then ", " and "}
	for _, sep := range separators {
		next := make([]string, 0, len(parts))
		for _, part := range parts {
			split := strings.Split(part, sep)
			for _, candidate := range split {
				candidate = strings.TrimSpace(candidate)
				if candidate != "" {
					next = append(next, candidate)
				}
			}
		}
		parts = next
	}
	return parts
}

func classifyWorkflowTask(text string) (WorkflowTask, bool) {
	segment := strings.TrimSpace(text)
	if segment == "" {
		return WorkflowTask{}, false
	}
	lower := strings.ToLower(segment)
	switch {
	case hasExplicitCreateIntentForDomain(segment, "trader"):
		return WorkflowTask{Skill: "trader_management", Action: "create", Request: segment}, true
	case hasExplicitManagementDomainCue(segment, "trader"):
		action := ""
		switch {
		case containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"}):
			action = "create"
		case containsAny(lower, []string{"启动", "开始", "run", "start"}):
			action = "start"
		case containsAny(lower, []string{"停止", "停掉", "stop", "pause"}):
			action = "stop"
		case containsAny(lower, []string{"删除", "删了", "删掉", "delete"}):
			action = "delete"
		case containsAny(lower, []string{"换模型", "换交易所", "换策略", "切换模型", "切换交易所", "切换策略", "扫描间隔", "全仓", "逐仓", "竞技场"}):
			action = "update_bindings"
		case containsAny(lower, []string{"修改", "更新", "改"}):
			action = "update_bindings"
		case containsAny(lower, []string{"详情", "配置", "参数", "what", "detail"}):
			action = "query_detail"
		case containsAny(lower, []string{"列表", "全部", "哪些", "list"}):
			action = "query_list"
		}
		if supportedWorkflowSkill("trader_management", action) {
			return WorkflowTask{Skill: "trader_management", Action: action, Request: segment}, true
		}
	case hasExplicitManagementDomainCue(segment, "exchange"):
		action := ""
		switch {
		case containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"}):
			action = "create"
		case containsAny(lower, []string{"启用", "enable", "禁用", "disable"}):
			action = "update_status"
		case containsAny(lower, []string{"删除", "删了", "删掉", "delete"}):
			action = "delete"
		case containsAny(lower, []string{"修改", "更新", "改", "账户名", "api key", "secret", "passphrase", "钱包"}):
			action = "update"
		case containsAny(lower, []string{"详情", "配置", "参数", "what", "detail"}):
			action = "query_detail"
		case containsAny(lower, []string{"列表", "全部", "哪些", "list"}):
			action = "query_list"
		}
		if supportedWorkflowSkill("exchange_management", action) {
			return WorkflowTask{Skill: "exchange_management", Action: action, Request: segment}, true
		}
	case hasExplicitManagementDomainCue(segment, "model"):
		action := ""
		switch {
		case containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"}):
			action = "create"
		case containsAny(lower, []string{"启用", "enable", "禁用", "disable"}):
			action = "update_status"
		case containsAny(lower, []string{"删除", "删了", "删掉", "delete"}):
			action = "delete"
		case containsAny(lower, []string{"接口地址", "endpoint", "url"}):
			action = "update_endpoint"
		case containsAny(lower, []string{"修改", "更新", "改", "模型名", "api key"}):
			action = "update"
		case containsAny(lower, []string{"详情", "配置", "参数", "what", "detail"}):
			action = "query_detail"
		case containsAny(lower, []string{"列表", "全部", "哪些", "list"}):
			action = "query_list"
		}
		if supportedWorkflowSkill("model_management", action) {
			return WorkflowTask{Skill: "model_management", Action: action, Request: segment}, true
		}
	case hasExplicitManagementDomainCue(segment, "strategy"):
		action := ""
		switch {
		case containsAny(lower, []string{"创建", "新建", "创一个", "创个", "create", "new"}):
			action = "create"
		case containsAny(lower, []string{"激活", "activate"}):
			action = "activate"
		case containsAny(lower, []string{"复制", "duplicate"}):
			action = "duplicate"
		case containsAny(lower, []string{"删除", "删了", "删掉", "delete"}):
			action = "delete"
		case containsAny(lower, []string{"prompt", "提示词"}):
			action = "update_prompt"
		case containsAny(lower, []string{"修改", "更新", "改", "参数", "配置"}):
			action = "update_config"
		case containsAny(lower, []string{"详情", "配置", "参数", "what", "detail"}) || hasExplicitStrategyDetailIntent(segment):
			action = "query_detail"
		case containsAny(lower, []string{"列表", "全部", "哪些", "list"}):
			action = "query_list"
		}
		if action == "" && hasExplicitStrategyDetailIntent(segment) {
			action = "query_detail"
		}
		if supportedWorkflowSkill("strategy_management", action) {
			return WorkflowTask{Skill: "strategy_management", Action: action, Request: segment}, true
		}
	}
	return WorkflowTask{}, false
}
