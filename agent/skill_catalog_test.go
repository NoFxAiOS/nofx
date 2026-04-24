package agent

import (
	"log/slog"
	"strings"
	"testing"
)

func TestSkillCatalogPromptZHIncludesDiagnosisSkills(t *testing.T) {
	got := skillCatalogPrompt("zh")
	for _, want := range []string{
		"多轮与 Skill-First 工作模式",
		"skill_model_config_diagnosis",
		"skill_exchange_api_diagnosis",
		"skill_trader_start_diagnosis",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("skillCatalogPrompt(zh) missing %q\n%s", want, got)
		}
	}
}

func TestBuildSystemPromptIncludesSkillCatalog(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), slog.Default())
	got := a.buildSystemPrompt("zh")
	for _, want := range []string{
		"多轮与 Skill-First 工作模式",
		"skill_exchange_api_setup",
		"skill_order_execution_diagnosis",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildSystemPrompt(zh) missing %q", want)
		}
	}
}
