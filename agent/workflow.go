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
	switch skill {
	case "trader_management", "strategy_management", "model_management", "exchange_management":
		switch action {
		case "create", "query_list", "query_detail", "query_running", "activate":
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

func (a *Agent) handleWorkflowSession(ctx context.Context, storeUserID string, userID int64, lang, text string, session WorkflowSession, onEvent func(event, data string)) (string, bool, error) {
	if isExplicitFlowAbort(text) {
		a.clearSkillSession(userID)
		a.clearWorkflowSession(userID)
		if lang == "zh" {
			return "已取消当前任务流。", true, nil
		}
		return "Cancelled the current workflow.", true, nil
	}

	if activeSkill := a.getSkillSession(userID); strings.TrimSpace(activeSkill.Name) != "" {
		answer, handled := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent)
		if !handled {
			return "", false, nil
		}
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

	return a.maybeAdvanceWorkflow(ctx, storeUserID, userID, lang, session, onEvent)
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
			onEvent(StreamEventDelta, summary)
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

	answer, handled := a.tryHardSkill(ctx, storeUserID, userID, lang, task.Request, onEvent)
	if !handled {
		session.Tasks[index].Status = workflowTaskFailed
		session.Tasks[index].Error = "task_not_handled"
		a.saveWorkflowSession(userID, session)
		return "", false, nil
	}

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
Do not mention internal DAG, scheduler, or JSON.`
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
	return count > 0
}

func (a *Agent) decomposeWorkflowIntentWithLLM(ctx context.Context, userID int64, lang, text string) (workflowDecomposition, error) {
	stageCtx, cancel := withPlannerStageTimeout(ctx, directReplyTimeout)
	defer cancel()
	systemPrompt := `You decompose one NOFXi user request into a small task graph.
Return JSON only. No markdown.
Only use these skills: trader_management, strategy_management, model_management, exchange_management.
Only use one atomic action per task.
Each task must include:
- id
- skill
- action
- request
- depends_on (array, may be empty)
If the request is effectively a single task, return one task only.`
	userPrompt := fmt.Sprintf("Language: %s\nUser request: %s", lang, text)
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
	for i, segment := range segments {
		task, ok := classifyWorkflowTask(segment)
		if !ok {
			continue
		}
		task.ID = fmt.Sprintf("task_%d", i+1)
		task.Status = workflowTaskPending
		if len(tasks) > 0 {
			task.DependsOn = []string{tasks[len(tasks)-1].ID}
		}
		tasks = append(tasks, task)
	}
	return workflowDecomposition{Tasks: tasks}
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
	switch {
	case detectCreateTraderSkill(segment):
		return WorkflowTask{Skill: "trader_management", Action: "create", Request: segment}, true
	case detectTraderManagementIntent(segment):
		action := normalizeAtomicSkillAction("trader_management", detectManagementAction(segment, "trader"))
		if supportedWorkflowSkill("trader_management", action) {
			return WorkflowTask{Skill: "trader_management", Action: action, Request: segment}, true
		}
	case detectExchangeManagementIntent(segment):
		action := normalizeAtomicSkillAction("exchange_management", detectManagementAction(segment, "exchange"))
		if supportedWorkflowSkill("exchange_management", action) {
			return WorkflowTask{Skill: "exchange_management", Action: action, Request: segment}, true
		}
	case detectModelManagementIntent(segment):
		action := normalizeAtomicSkillAction("model_management", detectManagementAction(segment, "model"))
		if supportedWorkflowSkill("model_management", action) {
			return WorkflowTask{Skill: "model_management", Action: action, Request: segment}, true
		}
	case detectStrategyManagementIntent(segment):
		action := normalizeAtomicSkillAction("strategy_management", detectManagementAction(segment, "strategy"))
		if action == "" && wantsStrategyDetails(segment) {
			action = "query_detail"
		}
		if supportedWorkflowSkill("strategy_management", action) {
			return WorkflowTask{Skill: "strategy_management", Action: action, Request: segment}, true
		}
	}
	return WorkflowTask{}, false
}
