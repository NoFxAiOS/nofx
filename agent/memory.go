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
	recentConversationRounds       = 3
	recentConversationMessages     = recentConversationRounds * 2
	taskStateSummaryTokenLimit     = 1200
	shortTermCompressThreshold     = 900
	incrementalTaskStateMessages   = 6
	incrementalTaskStateTokenLimit = 500
)

type DecisionMemory struct {
	Action     string `json:"action,omitempty"`
	Reason     string `json:"reason,omitempty"`
	StillValid bool   `json:"still_valid,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
}

type TaskState struct {
	CurrentGoal string `json:"current_goal,omitempty"`
	ActiveFlow  string `json:"active_flow,omitempty"`
	// OpenLoops stores only high-level unresolved issues that still matter across turns.
	// Step-level pending work belongs in ExecutionState, not here.
	OpenLoops      []string        `json:"open_loops,omitempty"`
	ImportantFacts []string        `json:"important_facts,omitempty"`
	LastDecision   *DecisionMemory `json:"last_decision,omitempty"`
	UpdatedAt      string          `json:"updated_at,omitempty"`
}

func TaskStateConfigKey(userID int64) string {
	return fmt.Sprintf("agent_task_state_%d", userID)
}

func (a *Agent) getTaskState(userID int64) TaskState {
	if a.store == nil {
		return TaskState{}
	}
	raw, err := a.store.GetSystemConfig(TaskStateConfigKey(userID))
	if err != nil {
		a.logger.Warn("failed to load task state", "error", err, "user_id", userID)
		return TaskState{}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return TaskState{}
	}

	var state TaskState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		a.logger.Warn("failed to parse task state", "error", err, "user_id", userID)
		return TaskState{}
	}
	return normalizeTaskState(state)
}

func (a *Agent) saveTaskState(userID int64, state TaskState) error {
	if a.store == nil {
		return fmt.Errorf("store unavailable")
	}
	state = normalizeTaskState(state)
	if isZeroTaskState(state) {
		return a.store.SetSystemConfig(TaskStateConfigKey(userID), "")
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return a.store.SetSystemConfig(TaskStateConfigKey(userID), string(data))
}

func (a *Agent) clearTaskState(userID int64) {
	if a.store == nil {
		return
	}
	if err := a.store.SetSystemConfig(TaskStateConfigKey(userID), ""); err != nil {
		a.logger.Warn("failed to clear task state", "error", err, "user_id", userID)
	}
}

func normalizeTaskState(state TaskState) TaskState {
	state.CurrentGoal = strings.TrimSpace(state.CurrentGoal)
	state.ActiveFlow = strings.TrimSpace(state.ActiveFlow)
	state.OpenLoops = filterTaskStateOpenLoops(cleanStringList(state.OpenLoops))
	state.ImportantFacts = cleanStringList(state.ImportantFacts)
	if state.LastDecision != nil {
		state.LastDecision.Action = strings.TrimSpace(state.LastDecision.Action)
		state.LastDecision.Reason = strings.TrimSpace(state.LastDecision.Reason)
		state.LastDecision.Timestamp = strings.TrimSpace(state.LastDecision.Timestamp)
		if state.LastDecision.Timestamp == "" && (state.LastDecision.Action != "" || state.LastDecision.Reason != "") {
			state.LastDecision.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}
		if state.LastDecision.Action == "" && state.LastDecision.Reason == "" {
			state.LastDecision = nil
		}
	}
	if state.UpdatedAt == "" && !isZeroTaskState(state) {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return state
}

func isZeroTaskState(state TaskState) bool {
	return state.CurrentGoal == "" &&
		state.ActiveFlow == "" &&
		len(state.OpenLoops) == 0 &&
		len(state.ImportantFacts) == 0 &&
		state.LastDecision == nil
}

func cleanStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterTaskStateOpenLoops(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	rejectedPrefixes := []string{
		"wait for ",
		"waiting for ",
		"ask for ",
		"call ",
		"run ",
		"execute ",
		"invoke ",
		"use tool",
		"step ",
	}
	rejectedContains := []string{
		"current step",
		"tool call",
		"api key",
		"api secret",
		"secret key",
		"passphrase",
		"model id",
		"exchange id",
	}

	filtered := make([]string, 0, len(values))
	for _, value := range values {
		lower := strings.ToLower(strings.TrimSpace(value))
		if lower == "" {
			continue
		}
		if matchesAnyPrefix(lower, rejectedPrefixes) || matchesAnyContains(lower, rejectedContains) {
			continue
		}
		filtered = append(filtered, value)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func matchesAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func matchesAnyContains(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func buildTaskStateContext(state TaskState) string {
	state = normalizeTaskState(state)
	if isZeroTaskState(state) {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Structured Task State - durable, non-derivable context]\n")
	if state.CurrentGoal != "" {
		sb.WriteString("- Current goal: ")
		sb.WriteString(state.CurrentGoal)
		sb.WriteString("\n")
	}
	if state.ActiveFlow != "" {
		sb.WriteString("- Active flow: ")
		sb.WriteString(state.ActiveFlow)
		sb.WriteString("\n")
	}
	for _, loop := range state.OpenLoops {
		sb.WriteString("- High-level open loop: ")
		sb.WriteString(loop)
		sb.WriteString("\n")
	}
	for _, fact := range state.ImportantFacts {
		sb.WriteString("- Important fact: ")
		sb.WriteString(fact)
		sb.WriteString("\n")
	}
	if state.LastDecision != nil {
		sb.WriteString("- Last decision: ")
		sb.WriteString(state.LastDecision.Action)
		if state.LastDecision.Reason != "" {
			sb.WriteString(" | reason: ")
			sb.WriteString(state.LastDecision.Reason)
		}
		if state.LastDecision.StillValid {
			sb.WriteString(" | still valid")
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func estimateChatMessagesTokens(msgs []chatMessage) int {
	total := 0
	for _, msg := range msgs {
		total += len([]rune(msg.Content))/3 + 10
	}
	return total
}

func formatChatMessagesForSummary(msgs []chatMessage) string {
	var sb strings.Builder
	for _, msg := range msgs {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func (a *Agent) maybeCompressHistory(ctx context.Context, userID int64) {
	if a.aiClient == nil || a.history == nil {
		return
	}

	msgs := a.history.Get(userID)
	if len(msgs) <= recentConversationMessages {
		return
	}
	if estimateChatMessagesTokens(msgs) <= shortTermCompressThreshold {
		return
	}

	splitAt := len(msgs) - recentConversationMessages
	if splitAt <= 0 {
		return
	}

	oldPart := msgs[:splitAt]
	recentPart := msgs[splitAt:]
	existingState := a.getTaskState(userID)
	updatedState, err := a.summarizeConversationToTaskState(ctx, userID, existingState, oldPart)
	if err != nil {
		a.logger.Warn("failed to compress chat history", "error", err, "user_id", userID)
		return
	}
	if err := a.saveTaskState(userID, updatedState); err != nil {
		a.log().Warn("failed to persist task state", "error", err, "user_id", userID)
		return
	}
	a.history.Replace(userID, recentPart)
}

func (a *Agent) maybeUpdateTaskStateIncrementally(ctx context.Context, userID int64) {
	if a.aiClient == nil || a.history == nil {
		return
	}

	msgs := a.history.Get(userID)
	if len(msgs) < 2 {
		return
	}

	window := msgs
	if len(window) > incrementalTaskStateMessages {
		window = window[len(window)-incrementalTaskStateMessages:]
	}

	existingState := a.getTaskState(userID)
	updatedState, err := a.summarizeRecentConversationToTaskState(ctx, userID, existingState, window)
	if err != nil {
		a.log().Warn("failed to incrementally update task state", "error", err, "user_id", userID)
		return
	}
	if err := a.saveTaskState(userID, updatedState); err != nil {
		a.log().Warn("failed to persist incremental task state", "error", err, "user_id", userID)
	}
}

func (a *Agent) summarizeConversationToTaskState(ctx context.Context, userID int64, existing TaskState, oldPart []chatMessage) (TaskState, error) {
	transcript := formatChatMessagesForSummary(oldPart)
	if transcript == "" {
		return normalizeTaskState(existing), nil
	}

	existingJSON, err := json.Marshal(normalizeTaskState(existing))
	if err != nil {
		return TaskState{}, err
	}

	systemPrompt := `You maintain structured task state for a trading assistant.
Update the task state using the existing state plus archived dialogue.
Return JSON only. Do not return markdown.

Rules:
- Keep only durable, non-derivable context useful for future turns.
- Do not store market prices, balances, positions, or anything tools can fetch again.
- Do not store chit-chat or repeated wording.
- current_goal: the user's active objective, if any.
- active_flow: a named flow such as onboarding, trading_confirmation, market_analysis, or empty.
- open_loops: only high-level unresolved issues that still matter across turns.
- Do not put execution-step pending work into open_loops.
- Bad open_loops examples: "wait for API secret", "call get_exchange_configs", "run step 2", "ask user for exchange_id".
- Good open_loops examples: "finish trader setup after external configuration is ready", "user still wants to complete onboarding".
- important_facts: non-derivable facts worth remembering briefly.
- last_decision: keep only one current relevant decision; omit if none.
- Replace stale items instead of appending blindly.
- If a field is no longer relevant, return it empty or omit it.
- Never invent facts.`

	userPrompt := fmt.Sprintf("Existing task state JSON:\n%s\n\nArchived dialogue to compress:\n%s\n\nReturn the new task state JSON with this exact shape:\n{\"current_goal\":\"\",\"active_flow\":\"\",\"open_loops\":[],\"important_facts\":[],\"last_decision\":{\"action\":\"\",\"reason\":\"\",\"still_valid\":false,\"timestamp\":\"\"},\"updated_at\":\"\"}", string(existingJSON), transcript)

	req := &mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx:       ctx,
		MaxTokens: intPtr(taskStateSummaryTokenLimit),
	}

	resp, err := a.aiClient.CallWithRequest(req)
	if err != nil {
		return TaskState{}, err
	}

	state, err := parseTaskStateJSON(resp)
	if err != nil {
		return TaskState{}, err
	}
	state = normalizeTaskState(state)
	a.log().Info("compressed chat history into task state", "user_id", userID, "archived_messages", len(oldPart))
	return state, nil
}

func (a *Agent) summarizeRecentConversationToTaskState(ctx context.Context, userID int64, existing TaskState, recentPart []chatMessage) (TaskState, error) {
	transcript := formatChatMessagesForSummary(recentPart)
	if transcript == "" {
		return normalizeTaskState(existing), nil
	}

	existingJSON, err := json.Marshal(normalizeTaskState(existing))
	if err != nil {
		return TaskState{}, err
	}

	systemPrompt := `You maintain structured task state for a trading assistant.
Update the task state incrementally using the existing state plus the latest conversation window.
Return JSON only. Do not return markdown.

Rules:
- Capture newly confirmed facts from the latest few turns immediately.
- Preserve important existing facts that still matter; replace stale items when contradicted.
- Keep only durable, non-derivable context useful for the next turns.
- current_goal: the user's active objective right now.
- active_flow: a named flow such as onboarding, trading_confirmation, market_analysis, strategy_debugging, or empty.
- open_loops: only high-level unresolved issues that still matter across turns.
- important_facts: include recently confirmed concrete facts, such as the current trader under discussion, the reported runtime error, the user's claimed config value, or the environment where the issue occurs.
- Do not store execution-step pending work or tool instructions.
- Do not store market prices, balances, or anything tools can fetch again.
- Keep last_decision only if there is a current relevant decision; omit it otherwise.
- Never invent facts.`

	userPrompt := fmt.Sprintf("Existing task state JSON:\n%s\n\nLatest conversation window:\n%s\n\nReturn the updated task state JSON with this exact shape:\n{\"current_goal\":\"\",\"active_flow\":\"\",\"open_loops\":[],\"important_facts\":[],\"last_decision\":{\"action\":\"\",\"reason\":\"\",\"still_valid\":false,\"timestamp\":\"\"},\"updated_at\":\"\"}", string(existingJSON), transcript)

	req := &mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx:       ctx,
		MaxTokens: intPtr(incrementalTaskStateTokenLimit),
	}

	resp, err := a.aiClient.CallWithRequest(req)
	if err != nil {
		return TaskState{}, err
	}

	state, err := parseTaskStateJSON(resp)
	if err != nil {
		return TaskState{}, err
	}
	state = normalizeTaskState(state)
	a.log().Info("incrementally refreshed task state", "user_id", userID, "window_messages", len(recentPart))
	return state, nil
}

func parseTaskStateJSON(raw string) (TaskState, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var state TaskState
	if err := json.Unmarshal([]byte(raw), &state); err == nil {
		return state, nil
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &state); err == nil {
			return state, nil
		}
	}
	return TaskState{}, fmt.Errorf("invalid task state json")
}

func intPtr(v int) *int {
	return &v
}
