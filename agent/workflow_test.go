package agent

import "testing"

func TestSplitWorkflowSegments(t *testing.T) {
	got := splitWorkflowSegments("把策略删了，再把交易所改名")
	if len(got) != 2 {
		t.Fatalf("expected 2 segments, got %d: %#v", len(got), got)
	}
}

func TestClassifyWorkflowTask(t *testing.T) {
	task, ok := classifyWorkflowTask("把策略删了")
	if !ok {
		t.Fatal("expected task")
	}
	if task.Skill != "strategy_management" || task.Action != "delete" {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func TestFallbackWorkflowDecompositionBuildsTwoTasks(t *testing.T) {
	a := &Agent{}
	out := a.decomposeWorkflowIntentFallback("把策略删了，再把交易所改名")
	if len(out.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(out.Tasks))
	}
	if out.Tasks[0].Skill != "strategy_management" {
		t.Fatalf("unexpected first task: %+v", out.Tasks[0])
	}
	if out.Tasks[1].Skill != "exchange_management" {
		t.Fatalf("unexpected second task: %+v", out.Tasks[1])
	}
	if len(out.Tasks[1].DependsOn) != 1 || out.Tasks[1].DependsOn[0] != out.Tasks[0].ID {
		t.Fatalf("expected dependency on first task, got %+v", out.Tasks[1].DependsOn)
	}
}
