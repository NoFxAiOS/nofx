package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ReferenceMemory struct {
	CurrentReferences *CurrentReferences `json:"current_references,omitempty"`
	ReferenceHistory  []ReferenceRecord  `json:"reference_history,omitempty"`
	UpdatedAt         string             `json:"updated_at,omitempty"`
}

func referenceMemoryConfigKey(userID int64) string {
	return fmt.Sprintf("agent_reference_memory_%d", userID)
}

func (a *Agent) getReferenceMemory(userID int64) ReferenceMemory {
	if a == nil || a.store == nil {
		return ReferenceMemory{}
	}
	raw, err := a.store.GetSystemConfig(referenceMemoryConfigKey(userID))
	if err != nil {
		return ReferenceMemory{}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ReferenceMemory{}
	}
	var memory ReferenceMemory
	if err := json.Unmarshal([]byte(raw), &memory); err != nil {
		return ReferenceMemory{}
	}
	memory.CurrentReferences = normalizeCurrentReferences(memory.CurrentReferences)
	memory.ReferenceHistory = normalizeReferenceHistory(memory.ReferenceHistory)
	return memory
}

func (a *Agent) saveReferenceMemory(userID int64, refs *CurrentReferences, history []ReferenceRecord) {
	if a == nil || a.store == nil {
		return
	}
	memory := ReferenceMemory{
		CurrentReferences: normalizeCurrentReferences(refs),
		ReferenceHistory:  normalizeReferenceHistory(history),
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	if memory.CurrentReferences == nil && len(memory.ReferenceHistory) == 0 {
		_ = a.store.SetSystemConfig(referenceMemoryConfigKey(userID), "")
		return
	}
	data, err := json.Marshal(memory)
	if err != nil {
		return
	}
	_ = a.store.SetSystemConfig(referenceMemoryConfigKey(userID), string(data))
}

func (a *Agent) clearReferenceMemory(userID int64) {
	if a == nil || a.store == nil {
		return
	}
	_ = a.store.SetSystemConfig(referenceMemoryConfigKey(userID), "")
}

func (a *Agent) semanticCurrentReferences(userID int64) *CurrentReferences {
	state := a.getExecutionState(userID)
	if refs := normalizeCurrentReferences(state.CurrentReferences); refs != nil {
		return refs
	}
	return a.getReferenceMemory(userID).CurrentReferences
}

func (a *Agent) semanticReferenceHistory(userID int64) []ReferenceRecord {
	state := a.getExecutionState(userID)
	if history := normalizeReferenceHistory(state.ReferenceHistory); len(history) > 0 {
		return history
	}
	return a.getReferenceMemory(userID).ReferenceHistory
}

func (a *Agent) rememberReferencesFromToolResult(userID int64, toolName, raw string) {
	if a == nil {
		return
	}
	memory := a.getReferenceMemory(userID)
	state := ExecutionState{
		UserID:            userID,
		CurrentReferences: memory.CurrentReferences,
		ReferenceHistory:  memory.ReferenceHistory,
	}
	if !updateCurrentReferencesFromToolResult(&state, toolName, raw) {
		return
	}
	a.saveReferenceMemory(userID, state.CurrentReferences, state.ReferenceHistory)
	execState := a.getExecutionState(userID)
	execState.CurrentReferences = state.CurrentReferences
	a.saveExecutionState(execState)
}
