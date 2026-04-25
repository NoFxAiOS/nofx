package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ActiveSkillSession is the minimal session for the central brain architecture.
// It replaces the old skillSession + ExecutionState combo for management skill flows.
type ActiveSkillSession struct {
	SessionID       string            `json:"session_id"`
	UserID          int64             `json:"user_id"`
	SkillName       string            `json:"skill_name"`
	ActionName      string            `json:"action_name"`
	LegacyPhase     string            `json:"legacy_phase,omitempty"`
	Goal            string            `json:"goal,omitempty"`
	PendingHint     *PendingHint      `json:"pending_hint,omitempty"`
	CollectedFields map[string]any    `json:"collected_fields,omitempty"`
	LocalHistory    []chatMessage     `json:"local_history,omitempty"`
	UpdatedAt       string            `json:"updated_at"`
}

type PendingHint struct {
	Prompt   string `json:"prompt,omitempty"`
	HintType string `json:"hint_type,omitempty"`
}

type PendingProposalSession struct {
	UserID         int64  `json:"user_id"`
	SourceUserText string `json:"source_user_text,omitempty"`
	ProposalText   string `json:"proposal_text,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

func activeSkillSessionKey(userID int64) string {
	return fmt.Sprintf("agent_active_skill_session_%d", userID)
}

func pendingProposalSessionKey(userID int64) string {
	return fmt.Sprintf("agent_pending_proposal_session_%d", userID)
}

func (a *Agent) getActiveSkillSession(userID int64) (ActiveSkillSession, bool) {
	if a.store == nil {
		return ActiveSkillSession{}, false
	}
	raw, err := a.store.GetSystemConfig(activeSkillSessionKey(userID))
	if err != nil || strings.TrimSpace(raw) == "" {
		return ActiveSkillSession{}, false
	}
	var s ActiveSkillSession
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return ActiveSkillSession{}, false
	}
	if s.SessionID == "" || s.SkillName == "" {
		return ActiveSkillSession{}, false
	}
	s.PendingHint = normalizePendingHint(s.PendingHint)
	return s, true
}

func (a *Agent) saveActiveSkillSession(s ActiveSkillSession) {
	if a.store == nil {
		return
	}
	s.PendingHint = normalizePendingHint(s.PendingHint)
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, _ := json.Marshal(s)
	_ = a.store.SetSystemConfig(activeSkillSessionKey(s.UserID), string(data))
}

func (a *Agent) clearActiveSkillSession(userID int64) {
	if a.store == nil {
		return
	}
	_ = a.store.SetSystemConfig(activeSkillSessionKey(userID), "")
}

func (a *Agent) getPendingProposalSession(userID int64) (PendingProposalSession, bool) {
	if a.store == nil {
		return PendingProposalSession{}, false
	}
	raw, err := a.store.GetSystemConfig(pendingProposalSessionKey(userID))
	if err != nil || strings.TrimSpace(raw) == "" {
		return PendingProposalSession{}, false
	}
	var s PendingProposalSession
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return PendingProposalSession{}, false
	}
	if s.UserID == 0 || strings.TrimSpace(s.ProposalText) == "" {
		return PendingProposalSession{}, false
	}
	return s, true
}

func (a *Agent) savePendingProposalSession(s PendingProposalSession) {
	if a.store == nil {
		return
	}
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, _ := json.Marshal(s)
	_ = a.store.SetSystemConfig(pendingProposalSessionKey(s.UserID), string(data))
}

func (a *Agent) clearPendingProposalSession(userID int64) {
	if a.store == nil {
		return
	}
	_ = a.store.SetSystemConfig(pendingProposalSessionKey(userID), "")
}

func newActiveSkillSession(userID int64, skill, action string) ActiveSkillSession {
	return ActiveSkillSession{
		SessionID:       fmt.Sprintf("as_%d", time.Now().UnixNano()),
		UserID:          userID,
		SkillName:       skill,
		ActionName:      action,
		LegacyPhase:     "collecting",
		Goal:            "",
		PendingHint:     nil,
		CollectedFields: map[string]any{},
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
}

func normalizePendingHint(hint *PendingHint) *PendingHint {
	if hint == nil {
		return nil
	}
	prompt := strings.TrimSpace(hint.Prompt)
	if prompt == "" {
		return nil
	}
	out := &PendingHint{
		Prompt:   prompt,
		HintType: strings.TrimSpace(hint.HintType),
	}
	return out
}

func pendingHintFromAssistantReply(reply string) *PendingHint {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}
	hintType := ""
	switch {
	case strings.Contains(reply, "请选择") || strings.Contains(strings.ToLower(reply), "choose"):
		hintType = "choice"
	case strings.Contains(reply, "确认") || strings.Contains(strings.ToLower(reply), "confirm"):
		hintType = "confirmation"
	case strings.HasSuffix(reply, "?") || strings.HasSuffix(reply, "？"):
		hintType = "question"
	}
	if hintType == "" {
		return nil
	}
	return &PendingHint{Prompt: reply, HintType: hintType}
}

func setActiveSessionPendingHint(session *ActiveSkillSession, reply string) {
	if session == nil {
		return
	}
	session.PendingHint = pendingHintFromAssistantReply(reply)
}

func clearActiveSessionPendingHint(session *ActiveSkillSession) {
	if session == nil {
		return
	}
	session.PendingHint = nil
}

func (a *Agent) currentPendingHintText(userID int64) string {
	if active, ok := a.getActiveSkillSession(userID); ok && active.PendingHint != nil && strings.TrimSpace(active.PendingHint.Prompt) != "" {
		return strings.TrimSpace(active.PendingHint.Prompt)
	}
	if state := a.getExecutionState(userID); state.Waiting != nil && strings.TrimSpace(state.Waiting.Question) != "" {
		return strings.TrimSpace(state.Waiting.Question)
	}
	if proposal, ok := a.getPendingProposalSession(userID); ok && strings.TrimSpace(proposal.ProposalText) != "" {
		return strings.TrimSpace(proposal.ProposalText)
	}
	return strings.TrimSpace(a.getLastAssistantReply(userID))
}

func activeSessionHasField(s ActiveSkillSession, slot string) bool {
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return false
	}
	if len(s.CollectedFields) == 0 {
		return false
	}
	switch slot {
	case "target_ref":
		if value, ok := s.CollectedFields["bulk_scope"]; ok && strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "all") {
			return true
		}
		for _, key := range []string{"target_ref", "target_ref_id", "target_ref_name"} {
			if value, ok := s.CollectedFields[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
				return true
			}
		}
		return false
	default:
		value, ok := s.CollectedFields[slot]
		return ok && strings.TrimSpace(fmt.Sprint(value)) != ""
	}
}

// missingRequiredFields returns required slots not yet collected, reading from skill registry.
func missingRequiredFields(s ActiveSkillSession) []string {
	def, ok := getSkillDefinition(s.SkillName)
	if !ok {
		return nil
	}
	actionDef, ok := def.Actions[s.ActionName]
	if !ok {
		return nil
	}
	var missing []string
	for _, slot := range actionDef.RequiredSlots {
		if !activeSessionHasField(s, slot) {
			missing = append(missing, slot)
		}
	}
	return missing
}

// fieldConstraintSummary returns a compact description of missing fields for the LLM prompt.
func fieldConstraintSummary(s ActiveSkillSession) string {
	def, ok := getSkillDefinition(s.SkillName)
	if !ok {
		return ""
	}
	missing := missingRequiredFields(s)
	if len(missing) == 0 {
		return ""
	}
	lines := make([]string, 0, len(missing))
	for _, key := range missing {
		constraint, ok := def.FieldConstraints[key]
		if !ok {
			lines = append(lines, fmt.Sprintf("- %s (required)", key))
			continue
		}
		desc := constraint.Description
		if len(constraint.Values) > 0 {
			desc += fmt.Sprintf(" [options: %s]", strings.Join(constraint.Values, ", "))
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", key, desc))
	}
	return strings.Join(lines, "\n")
}
