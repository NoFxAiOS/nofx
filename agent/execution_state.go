package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	executionStatusPlanning    = "planning"
	executionStatusRunning     = "running"
	executionStatusWaitingUser = "waiting_user"
	executionStatusCompleted   = "completed"
	executionStatusFailed      = "failed"
)

const (
	planStepTypeTool    = "tool"
	planStepTypeReason  = "reason"
	planStepTypeAskUser = "ask_user"
	planStepTypeRespond = "respond"
)

const (
	planStepStatusPending   = "pending"
	planStepStatusRunning   = "running"
	planStepStatusCompleted = "completed"
	planStepStatusFailed    = "failed"
)

type ExecutionState struct {
	SessionID         string             `json:"session_id"`
	UserID            int64              `json:"user_id"`
	Goal              string             `json:"goal"`
	Status            string             `json:"status"`
	PlanID            string             `json:"plan_id"`
	Steps             []PlanStep         `json:"steps,omitempty"`
	CurrentStepID     string             `json:"current_step_id,omitempty"`
	CurrentReferences *CurrentReferences `json:"current_references,omitempty"`
	ReferenceHistory  []ReferenceRecord  `json:"reference_history,omitempty"`
	DynamicSnapshots  []Observation      `json:"dynamic_snapshots,omitempty"`
	ExecutionLog      []Observation      `json:"execution_log,omitempty"`
	SummaryNotes      []Observation      `json:"summary_notes,omitempty"`
	Waiting           *WaitingState      `json:"waiting,omitempty"`
	Observations      []Observation      `json:"observations,omitempty"`
	FinalAnswer       string             `json:"final_answer,omitempty"`
	LastError         string             `json:"last_error,omitempty"`
	UpdatedAt         string             `json:"updated_at"`
}

type SuspendedTask struct {
	SnapshotID      string           `json:"snapshot_id,omitempty"`
	IntentID        string           `json:"intent_id,omitempty"`
	ParentIntentID  string           `json:"parent_intent_id,omitempty"`
	Kind            string           `json:"kind,omitempty"`
	ResumeHint      string           `json:"resume_hint,omitempty"`
	ResumeOnSuccess bool             `json:"resume_on_success,omitempty"`
	ResumeTriggers  []string         `json:"resume_triggers,omitempty"`
	SkillSession    *skillSession    `json:"skill_session,omitempty"`
	WorkflowSession *WorkflowSession `json:"workflow_session,omitempty"`
	ExecutionState  *ExecutionState  `json:"execution_state,omitempty"`
	LocalHistory    []chatMessage    `json:"local_history,omitempty"`
	SuspendedAt     string           `json:"suspended_at,omitempty"`
}

type PlanStep struct {
	ID                   string         `json:"id"`
	Type                 string         `json:"type"`
	Title                string         `json:"title,omitempty"`
	Status               string         `json:"status,omitempty"`
	ToolName             string         `json:"tool_name,omitempty"`
	ToolArgs             map[string]any `json:"tool_args,omitempty"`
	Instruction          string         `json:"instruction,omitempty"`
	RequiresConfirmation bool           `json:"requires_confirmation,omitempty"`
	OutputSummary        string         `json:"output_summary,omitempty"`
	Error                string         `json:"error,omitempty"`
}

type Observation struct {
	StepID    string `json:"step_id,omitempty"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	RawJSON   string `json:"raw_json,omitempty"`
	CreatedAt string `json:"created_at"`
}

type WaitingState struct {
	Question           string   `json:"question,omitempty"`
	Intent             string   `json:"intent,omitempty"`
	PendingFields      []string `json:"pending_fields,omitempty"`
	ConfirmationTarget string   `json:"confirmation_target,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty"`
}

type EntityReference struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Source    string `json:"source,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ReferenceRecord struct {
	Kind      string `json:"kind,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Source    string `json:"source,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type CurrentReferences struct {
	Strategy *EntityReference `json:"strategy,omitempty"`
	Trader   *EntityReference `json:"trader,omitempty"`
	Model    *EntityReference `json:"model,omitempty"`
	Exchange *EntityReference `json:"exchange,omitempty"`
}

type SnapshotSummary struct {
	SnapshotID  string `json:"snapshot_id,omitempty"`
	IntentID    string `json:"intent_id,omitempty"`
	ParentIntentID string `json:"parent_intent_id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	ResumeHint  string `json:"resume_hint,omitempty"`
	SuspendedAt string `json:"suspended_at,omitempty"`
}

type SnapshotManager struct {
	agent  *Agent
	userID int64
}

type executionPlan struct {
	Goal  string     `json:"goal"`
	Steps []PlanStep `json:"steps"`
}

const (
	executionLogMaxEntries = 8
	summaryNotesMaxEntries = 4
)

func ExecutionStateConfigKey(userID int64) string {
	return fmt.Sprintf("agent_execution_state_%d", userID)
}

func taskStackConfigKey(userID int64) string {
	return fmt.Sprintf("agent_task_stack_%d", userID)
}

func (a *Agent) SnapshotManager(userID int64) SnapshotManager {
	return SnapshotManager{agent: a, userID: userID}
}

func (m SnapshotManager) Save(task SuspendedTask) {
	if m.agent == nil {
		return
	}
	m.agent.pushTaskStack(m.userID, task)
}

func (m SnapshotManager) Load() (SuspendedTask, bool) {
	if m.agent == nil {
		return SuspendedTask{}, false
	}
	return m.agent.popTaskStack(m.userID)
}

func (m SnapshotManager) Peek() (SuspendedTask, bool) {
	if m.agent == nil {
		return SuspendedTask{}, false
	}
	return m.agent.peekTaskStack(m.userID)
}

func (m SnapshotManager) List() []SnapshotSummary {
	if m.agent == nil {
		return nil
	}
	stack := m.agent.getTaskStack(m.userID)
	out := make([]SnapshotSummary, 0, len(stack))
	for _, item := range stack {
		out = append(out, SnapshotSummary{
			SnapshotID:  strings.TrimSpace(item.SnapshotID),
			IntentID: strings.TrimSpace(item.IntentID),
			ParentIntentID: strings.TrimSpace(item.ParentIntentID),
			Kind:        strings.TrimSpace(item.Kind),
			ResumeHint:  strings.TrimSpace(item.ResumeHint),
			SuspendedAt: strings.TrimSpace(item.SuspendedAt),
		})
	}
	return out
}

func (m SnapshotManager) Stack() []SuspendedTask {
	if m.agent == nil {
		return nil
	}
	return m.agent.getTaskStack(m.userID)
}

func (m SnapshotManager) RemoveAt(index int) (SuspendedTask, bool) {
	if m.agent == nil {
		return SuspendedTask{}, false
	}
	stack := m.agent.getTaskStack(m.userID)
	if index < 0 || index >= len(stack) {
		return SuspendedTask{}, false
	}
	task := stack[index]
	stack = append(stack[:index], stack[index+1:]...)
	m.agent.saveTaskStack(m.userID, stack)
	return task, true
}

func (m SnapshotManager) Clear() {
	if m.agent == nil {
		return
	}
	m.agent.clearTaskStack(m.userID)
}

func (a *Agent) getExecutionState(userID int64) ExecutionState {
	if a.store == nil {
		return ExecutionState{}
	}
	raw, err := a.store.GetSystemConfig(ExecutionStateConfigKey(userID))
	if err != nil {
		a.logger.Warn("failed to load execution state", "error", err, "user_id", userID)
		return ExecutionState{}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ExecutionState{}
	}

	var state ExecutionState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		a.logger.Warn("failed to parse execution state", "error", err, "user_id", userID)
		return ExecutionState{}
	}
	return normalizeExecutionState(state)
}

func (a *Agent) saveExecutionState(state ExecutionState) error {
	if a.store == nil {
		return fmt.Errorf("store unavailable")
	}
	state = normalizeExecutionState(state)
	if state.SessionID == "" {
		return a.store.SetSystemConfig(ExecutionStateConfigKey(state.UserID), "")
	}
	if state.UserID != 0 && (state.CurrentReferences != nil || len(state.ReferenceHistory) > 0) {
		a.saveReferenceMemory(state.UserID, state.CurrentReferences, state.ReferenceHistory)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return a.store.SetSystemConfig(ExecutionStateConfigKey(state.UserID), string(data))
}

func (a *Agent) clearExecutionState(userID int64) {
	if a.store == nil {
		return
	}
	if err := a.store.SetSystemConfig(ExecutionStateConfigKey(userID), ""); err != nil {
		a.logger.Warn("failed to clear execution state", "error", err, "user_id", userID)
	}
}

func (a *Agent) getTaskStack(userID int64) []SuspendedTask {
	if a.store == nil {
		return nil
	}
	raw, err := a.store.GetSystemConfig(taskStackConfigKey(userID))
	if err != nil {
		a.logger.Warn("failed to load task stack", "error", err, "user_id", userID)
		return nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var stack []SuspendedTask
	if err := json.Unmarshal([]byte(raw), &stack); err != nil {
		a.logger.Warn("failed to parse task stack", "error", err, "user_id", userID)
		return nil
	}
	return normalizeTaskStack(stack)
}

func (a *Agent) saveTaskStack(userID int64, stack []SuspendedTask) {
	if a.store == nil {
		return
	}
	stack = normalizeTaskStack(stack)
	if len(stack) == 0 {
		_ = a.store.SetSystemConfig(taskStackConfigKey(userID), "")
		return
	}
	data, err := json.Marshal(stack)
	if err != nil {
		return
	}
	_ = a.store.SetSystemConfig(taskStackConfigKey(userID), string(data))
}

func (a *Agent) peekTaskStack(userID int64) (SuspendedTask, bool) {
	stack := a.getTaskStack(userID)
	if len(stack) == 0 {
		return SuspendedTask{}, false
	}
	return stack[len(stack)-1], true
}

func (a *Agent) pushTaskStack(userID int64, task SuspendedTask) {
	task = normalizeSuspendedTask(task)
	if task.Kind == "" {
		return
	}
	stack := a.getTaskStack(userID)
	stack = append(stack, task)
	stack = normalizeTaskStack(stack)
	a.saveTaskStack(userID, stack)
}

func (a *Agent) popTaskStack(userID int64) (SuspendedTask, bool) {
	stack := a.getTaskStack(userID)
	if len(stack) == 0 {
		return SuspendedTask{}, false
	}
	task := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	a.saveTaskStack(userID, stack)
	return task, true
}

func (a *Agent) clearTaskStack(userID int64) {
	if a.store == nil {
		return
	}
	_ = a.store.SetSystemConfig(taskStackConfigKey(userID), "")
}

func newExecutionState(userID int64, goal string) ExecutionState {
	now := time.Now().UTC().Format(time.RFC3339)
	return normalizeExecutionState(ExecutionState{
		SessionID: fmt.Sprintf("sess_%d", time.Now().UTC().UnixNano()),
		UserID:    userID,
		Goal:      strings.TrimSpace(goal),
		Status:    executionStatusPlanning,
		PlanID:    fmt.Sprintf("plan_%d", time.Now().UTC().UnixNano()),
		UpdatedAt: now,
	})
}

func normalizeExecutionState(state ExecutionState) ExecutionState {
	state.Goal = strings.TrimSpace(state.Goal)
	state.Status = strings.TrimSpace(state.Status)
	state.CurrentStepID = strings.TrimSpace(state.CurrentStepID)
	state.FinalAnswer = strings.TrimSpace(state.FinalAnswer)
	state.LastError = strings.TrimSpace(state.LastError)
	state.CurrentReferences = normalizeCurrentReferences(state.CurrentReferences)
	state.ReferenceHistory = normalizeReferenceHistory(state.ReferenceHistory)
	state.Waiting = normalizeWaitingState(state.Waiting)
	if state.Status == "" && state.SessionID != "" {
		state.Status = executionStatusPlanning
	}
	for i := range state.Steps {
		state.Steps[i].ID = strings.TrimSpace(state.Steps[i].ID)
		if state.Steps[i].ID == "" {
			state.Steps[i].ID = fmt.Sprintf("step_%d", i+1)
		}
		state.Steps[i].Type = strings.TrimSpace(state.Steps[i].Type)
		state.Steps[i].Title = strings.TrimSpace(state.Steps[i].Title)
		state.Steps[i].ToolName = strings.TrimSpace(state.Steps[i].ToolName)
		state.Steps[i].Instruction = strings.TrimSpace(state.Steps[i].Instruction)
		state.Steps[i].OutputSummary = strings.TrimSpace(state.Steps[i].OutputSummary)
		state.Steps[i].Error = strings.TrimSpace(state.Steps[i].Error)
		if state.Steps[i].Status == "" {
			state.Steps[i].Status = planStepStatusPending
		}
	}
	if len(state.Observations) > 0 {
		state.ExecutionLog = append(state.ExecutionLog, state.Observations...)
		state.Observations = nil
	}
	state.DynamicSnapshots = normalizeObservationList(state.DynamicSnapshots)
	state.ExecutionLog = normalizeObservationList(state.ExecutionLog)
	state.SummaryNotes = normalizeObservationList(state.SummaryNotes)
	state = compactExecutionLog(state)
	if state.UpdatedAt == "" && state.SessionID != "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return state
}

func normalizeSuspendedTask(task SuspendedTask) SuspendedTask {
	task.SnapshotID = strings.TrimSpace(task.SnapshotID)
	task.IntentID = strings.TrimSpace(task.IntentID)
	task.ParentIntentID = strings.TrimSpace(task.ParentIntentID)
	task.Kind = strings.TrimSpace(task.Kind)
	task.ResumeHint = strings.TrimSpace(task.ResumeHint)
	task.ResumeTriggers = cleanStringList(task.ResumeTriggers)
	task.SuspendedAt = strings.TrimSpace(task.SuspendedAt)
	if task.SkillSession != nil {
		session := normalizeSkillSession(*task.SkillSession)
		if session.Name == "" {
			task.SkillSession = nil
		} else {
			task.SkillSession = &session
		}
	}
	if task.WorkflowSession != nil {
		session := normalizeWorkflowSession(*task.WorkflowSession)
		if len(session.Tasks) == 0 {
			task.WorkflowSession = nil
		} else {
			task.WorkflowSession = &session
		}
	}
	if task.ExecutionState != nil {
		state := normalizeExecutionState(*task.ExecutionState)
		if strings.TrimSpace(state.SessionID) == "" {
			task.ExecutionState = nil
		} else {
			task.ExecutionState = &state
		}
	}
	if task.Kind == "" {
		switch {
		case task.SkillSession != nil:
			task.Kind = "skill_session"
		case task.WorkflowSession != nil:
			task.Kind = "workflow_session"
		case task.ExecutionState != nil:
			task.Kind = "execution_state"
		}
	}
	if task.Kind == "" {
		return SuspendedTask{}
	}
	if task.SnapshotID == "" {
		task.SnapshotID = "snap_" + uuid.NewString()
	}
	if task.IntentID == "" {
		task.IntentID = "intent_" + uuid.NewString()
	}
	if task.SuspendedAt == "" {
		task.SuspendedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return task
}

func normalizeTaskStack(stack []SuspendedTask) []SuspendedTask {
	if len(stack) == 0 {
		return nil
	}
	now := time.Now().UTC()
	out := make([]SuspendedTask, 0, len(stack))
	for _, item := range stack {
		item = normalizeSuspendedTask(item)
		if item.Kind == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, item.SuspendedAt); err == nil && now.Sub(t) > 24*time.Hour {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	if len(out) > 5 {
		out = out[len(out)-5:]
	}
	return out
}

func normalizeWaitingState(waiting *WaitingState) *WaitingState {
	if waiting == nil {
		return nil
	}
	waiting.Question = strings.TrimSpace(waiting.Question)
	waiting.Intent = strings.TrimSpace(waiting.Intent)
	waiting.PendingFields = cleanStringList(waiting.PendingFields)
	waiting.ConfirmationTarget = strings.TrimSpace(waiting.ConfirmationTarget)
	if waiting.CreatedAt == "" && (waiting.Question != "" || waiting.Intent != "" || len(waiting.PendingFields) > 0 || waiting.ConfirmationTarget != "") {
		waiting.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if waiting.Question == "" && waiting.Intent == "" && len(waiting.PendingFields) == 0 && waiting.ConfirmationTarget == "" {
		return nil
	}
	return waiting
}

func normalizeEntityReference(ref *EntityReference) *EntityReference {
	if ref == nil {
		return nil
	}
	ref.ID = strings.TrimSpace(ref.ID)
	ref.Name = strings.TrimSpace(ref.Name)
	ref.Source = strings.TrimSpace(ref.Source)
	ref.UpdatedAt = strings.TrimSpace(ref.UpdatedAt)
	if ref.ID == "" && ref.Name == "" {
		return nil
	}
	if ref.UpdatedAt == "" {
		ref.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return ref
}

func normalizeCurrentReferences(refs *CurrentReferences) *CurrentReferences {
	if refs == nil {
		return nil
	}
	refs.Strategy = normalizeEntityReference(refs.Strategy)
	refs.Trader = normalizeEntityReference(refs.Trader)
	refs.Model = normalizeEntityReference(refs.Model)
	refs.Exchange = normalizeEntityReference(refs.Exchange)
	if refs.Strategy == nil && refs.Trader == nil && refs.Model == nil && refs.Exchange == nil {
		return nil
	}
	return refs
}

func normalizeReferenceHistory(history []ReferenceRecord) []ReferenceRecord {
	if len(history) == 0 {
		return nil
	}
	out := make([]ReferenceRecord, 0, len(history))
	for _, item := range history {
		item.Kind = strings.TrimSpace(item.Kind)
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		item.Source = strings.TrimSpace(item.Source)
		item.CreatedAt = strings.TrimSpace(item.CreatedAt)
		if item.Kind == "" || (item.ID == "" && item.Name == "") {
			continue
		}
		if item.CreatedAt == "" {
			item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	if len(out) > 12 {
		out = out[len(out)-12:]
	}
	return out
}

func normalizeObservationList(values []Observation) []Observation {
	if len(values) == 0 {
		return nil
	}
	out := make([]Observation, 0, len(values))
	for _, value := range values {
		value.StepID = strings.TrimSpace(value.StepID)
		value.Kind = strings.TrimSpace(value.Kind)
		value.Summary = strings.TrimSpace(value.Summary)
		value.RawJSON = strings.TrimSpace(value.RawJSON)
		if value.Kind == "" && value.Summary == "" && value.RawJSON == "" {
			continue
		}
		if value.CreatedAt == "" {
			value.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactExecutionLog(state ExecutionState) ExecutionState {
	if len(state.ExecutionLog) <= executionLogMaxEntries {
		if len(state.SummaryNotes) > summaryNotesMaxEntries {
			state.SummaryNotes = state.SummaryNotes[len(state.SummaryNotes)-summaryNotesMaxEntries:]
		}
		return state
	}

	overflow := state.ExecutionLog[:len(state.ExecutionLog)-executionLogMaxEntries]
	state.ExecutionLog = state.ExecutionLog[len(state.ExecutionLog)-executionLogMaxEntries:]
	summary := summarizeExecutionOverflow(overflow)
	if summary != nil {
		state.SummaryNotes = append(state.SummaryNotes, *summary)
		if len(state.SummaryNotes) > summaryNotesMaxEntries {
			state.SummaryNotes = state.SummaryNotes[len(state.SummaryNotes)-summaryNotesMaxEntries:]
		}
	}
	return state
}

func summarizeExecutionOverflow(values []Observation) *Observation {
	if len(values) == 0 {
		return nil
	}
	summaries := make([]string, 0, len(values))
	for _, value := range values {
		label := value.Kind
		if label == "" {
			label = "observation"
		}
		if value.Summary != "" {
			summaries = append(summaries, fmt.Sprintf("%s: %s", label, value.Summary))
		} else if value.RawJSON != "" {
			summaries = append(summaries, fmt.Sprintf("%s: %s", label, value.RawJSON))
		}
	}
	if len(summaries) == 0 {
		return nil
	}
	text := strings.Join(summaries, " | ")
	if len(text) > 500 {
		text = text[:500] + "..."
	}
	return &Observation{
		Kind:      "execution_summary",
		Summary:   text,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func appendDynamicSnapshot(state *ExecutionState, obs Observation) {
	state.DynamicSnapshots = append(state.DynamicSnapshots, obs)
	state.DynamicSnapshots = normalizeObservationList(state.DynamicSnapshots)
}

func appendExecutionLog(state *ExecutionState, obs Observation) {
	state.ExecutionLog = append(state.ExecutionLog, obs)
	*state = normalizeExecutionState(*state)
}

func buildObservationContext(state ExecutionState) map[string]any {
	state = normalizeExecutionState(state)
	return map[string]any{
		"current_references": state.CurrentReferences,
		"dynamic_snapshots":  state.DynamicSnapshots,
		"execution_log":      state.ExecutionLog,
		"summary_notes":      state.SummaryNotes,
	}
}
