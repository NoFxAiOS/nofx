package decision

import (
	"strings"
	"testing"
)

// TestBuildSystemPrompt_ContainsAllValidActions tests whether prompt contains all valid actions
func TestBuildSystemPrompt_ContainsAllValidActions(t *testing.T) {
	// These are all valid actions defined in the system (from validateDecision)
	validActions := []string{
		"open_long",
		"open_short",
		"close_long",
		"close_short",
		"hold",
		"wait",
	}

	// Build prompt
	prompt := buildSystemPrompt(1000.0, 10, 5, "default", "")

	// Verify each valid action appears in prompt
	for _, action := range validActions {
		if !strings.Contains(prompt, action) {
			t.Errorf("Prompt missing valid action: %s", action)
		}
	}
}
