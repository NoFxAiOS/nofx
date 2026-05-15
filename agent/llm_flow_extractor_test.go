package agent

import (
	"strings"
	"testing"
)

func TestBuildActiveFlowExtractionPromptRequiresCanonicalFieldOutput(t *testing.T) {
	systemPrompt, _ := buildActiveFlowExtractionPrompt(
		"zh",
		"skill_session",
		"Active flow type: skill_session\nSkill: exchange_management\nAction: create",
		"secret是abc123456",
		"",
		nil,
		nil,
		nil,
	)

	for _, want := range []string{
		"Treat this as semantic slot filling, not keyword copying.",
		"always emit the canonical field keys from Allowed field spec JSON",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected system prompt to contain %q, got:\n%s", want, systemPrompt)
		}
	}
}
