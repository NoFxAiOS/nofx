package agent

import (
	"strings"
	"testing"
)

func TestClassifyWorkflowTaskTreatsTraderEditAsBindingsOrRename(t *testing.T) {
	task, ok := classifyWorkflowTask("帮我把交易员小爱换策略")
	if !ok {
		t.Fatal("expected trader binding edit to classify")
	}
	if task.Skill != "trader_management" || task.Action != "update_bindings" {
		t.Fatalf("unexpected task: %+v", task)
	}

	task, ok = classifyWorkflowTask("帮我把交易员小爱改名")
	if !ok {
		t.Fatal("expected trader rename to classify")
	}
	if task.Skill != "trader_management" || task.Action != "update_name" {
		t.Fatalf("unexpected rename task: %+v", task)
	}
}

func TestTraderDomainPrimerExplainsInternalConfigBoundary(t *testing.T) {
	primer := buildSkillDomainPrimer("zh", "trader_management")
	for _, want := range []string{
		"交易员是装配层",
		"默认只处理绑定关系",
		"应切到对应 management skill",
	} {
		if !strings.Contains(primer, want) {
			t.Fatalf("expected primer to contain %q, got: %s", want, primer)
		}
	}
}
