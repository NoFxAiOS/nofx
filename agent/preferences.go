package agent

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// PersistentPreference is a durable user instruction shown in the UI and
// injected into the agent context for future conversations.
type PersistentPreference struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at,omitempty"`
}

func NewPersistentPreference(text string) (PersistentPreference, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return PersistentPreference{}, fmt.Errorf("text required")
	}

	now := time.Now().UTC()
	return PersistentPreference{
		ID:        now.Format("20060102150405.000000000"),
		Text:      text,
		CreatedAt: now.Format(time.RFC3339),
	}, nil
}

// SessionUserIDFromKey maps a stable user key (for example a UUID string from
// auth) to the int64 session id expected by the current agent implementation.
func SessionUserIDFromKey(userKey string) int64 {
	if strings.TrimSpace(userKey) == "" {
		return 1
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(userKey))
	sum := h.Sum64() & 0x7fffffffffffffff
	if sum == 0 {
		return 1
	}
	return int64(sum)
}

func PreferencesConfigKey(userID int64) string {
	return fmt.Sprintf("agent_preferences_%d", userID)
}

func (a *Agent) getPersistentPreferences(userID int64) []PersistentPreference {
	if a.store == nil {
		return nil
	}

	raw, err := a.store.GetSystemConfig(PreferencesConfigKey(userID))
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}

	var prefs []PersistentPreference
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		a.logger.Warn("failed to parse persistent preferences", "error", err, "user_id", userID)
		return nil
	}
	return prefs
}

func (a *Agent) savePersistentPreferences(userID int64, prefs []PersistentPreference) error {
	if a.store == nil {
		return fmt.Errorf("store unavailable")
	}
	data, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	return a.store.SetSystemConfig(PreferencesConfigKey(userID), string(data))
}

func (a *Agent) addPersistentPreference(userID int64, text string) ([]PersistentPreference, PersistentPreference, error) {
	created, err := NewPersistentPreference(text)
	if err != nil {
		return nil, PersistentPreference{}, err
	}
	prefs := a.getPersistentPreferences(userID)
	prefs = append([]PersistentPreference{created}, prefs...)
	if len(prefs) > 20 {
		prefs = prefs[:20]
	}
	if err := a.savePersistentPreferences(userID, prefs); err != nil {
		return nil, PersistentPreference{}, err
	}
	return prefs, created, nil
}

func (a *Agent) updatePersistentPreference(userID int64, match, replacement string) ([]PersistentPreference, *PersistentPreference, error) {
	match = strings.TrimSpace(match)
	replacement = strings.TrimSpace(replacement)
	if match == "" || replacement == "" {
		return nil, nil, fmt.Errorf("match and replacement are required")
	}

	prefs := a.getPersistentPreferences(userID)
	for i := range prefs {
		if prefs[i].ID == match || strings.Contains(strings.ToLower(prefs[i].Text), strings.ToLower(match)) {
			prefs[i].Text = replacement
			if err := a.savePersistentPreferences(userID, prefs); err != nil {
				return nil, nil, err
			}
			return prefs, &prefs[i], nil
		}
	}
	return prefs, nil, fmt.Errorf("preference not found")
}

func (a *Agent) deletePersistentPreference(userID int64, match string) ([]PersistentPreference, *PersistentPreference, error) {
	match = strings.TrimSpace(match)
	if match == "" {
		return nil, nil, fmt.Errorf("match required")
	}

	prefs := a.getPersistentPreferences(userID)
	filtered := make([]PersistentPreference, 0, len(prefs))
	var removed *PersistentPreference
	for i := range prefs {
		p := prefs[i]
		if removed == nil && (p.ID == match || strings.Contains(strings.ToLower(p.Text), strings.ToLower(match))) {
			cp := p
			removed = &cp
			continue
		}
		filtered = append(filtered, p)
	}
	if removed == nil {
		return prefs, nil, fmt.Errorf("preference not found")
	}
	if err := a.savePersistentPreferences(userID, filtered); err != nil {
		return nil, nil, err
	}
	return filtered, removed, nil
}

func (a *Agent) buildPersistentPreferencesContext(userID int64) string {
	prefs := a.getPersistentPreferences(userID)
	if len(prefs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Persistent User Preferences - follow unless the user explicitly overrides them]\n")
	for _, pref := range prefs {
		if strings.TrimSpace(pref.Text) == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(pref.Text)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}
