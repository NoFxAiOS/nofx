package agent

import "testing"

func TestSanitizeLLMExtractionForSkillSessionDoesNotInventModelProvider(t *testing.T) {
	session := skillSession{Name: "model_management", Action: "create"}
	result := llmFlowExtractionResult{
		Intent: "continue",
		Tasks: []llmFlowExtractionTask{{
			Skill:  "model_management",
			Action: "create",
			Fields: map[string]string{
				"provider": "claw402",
			},
		}},
	}

	sanitized := sanitizeLLMExtractionForSkillSession("新建一个模型", session, result)
	if got := sanitized.Tasks[0].Fields["provider"]; got != "" {
		t.Fatalf("expected provider guess to be stripped, got %q", got)
	}
}

func TestSanitizeLLMExtractionForSkillSessionKeepsExplicitModelProvider(t *testing.T) {
	session := skillSession{Name: "model_management", Action: "create"}
	result := llmFlowExtractionResult{
		Intent: "continue",
		Tasks: []llmFlowExtractionTask{{
			Skill:  "model_management",
			Action: "create",
			Fields: map[string]string{
				"provider": "claw402",
			},
		}},
	}

	sanitized := sanitizeLLMExtractionForSkillSession("新建一个 claw402 模型", session, result)
	if got := sanitized.Tasks[0].Fields["provider"]; got != "claw402" {
		t.Fatalf("expected explicit provider to remain, got %q", got)
	}
}

