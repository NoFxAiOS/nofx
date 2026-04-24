package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
	SessionID        string        `json:"session_id"`
	UserID           int64         `json:"user_id"`
	Goal             string        `json:"goal"`
	Status           string        `json:"status"`
	PlanID           string        `json:"plan_id"`
	Steps            []PlanStep    `json:"steps,omitempty"`
	CurrentStepID    string        `json:"current_step_id,omitempty"`
	CurrentReferences *CurrentReferences `json:"current_references,omitempty"`
	DynamicSnapshots []Observation `json:"dynamic_snapshots,omitempty"`
	ExecutionLog     []Observation `json:"execution_log,omitempty"`
	SummaryNotes     []Observation `json:"summary_notes,omitempty"`
	Waiting          *WaitingState `json:"waiting,omitempty"`
	Observations     []Observation `json:"observations,omitempty"`
	FinalAnswer      string        `json:"final_answer,omitempty"`
	LastError        string        `json:"last_error,omitempty"`
	UpdatedAt        string        `json:"updated_at"`
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
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type CurrentReferences struct {
	Strategy *EntityReference `json:"strategy,omitempty"`
	Trader   *EntityReference `json:"trader,omitempty"`
	Model    *EntityReference `json:"model,omitempty"`
	Exchange *EntityReference `json:"exchange,omitempty"`
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
	if ref.ID == "" && ref.Name == "" {
		return nil
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
		"dynamic_snapshots": state.DynamicSnapshots,
		"execution_log":     state.ExecutionLog,
		"summary_notes":     state.SummaryNotes,
	}
}
