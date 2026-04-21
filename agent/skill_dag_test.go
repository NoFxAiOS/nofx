package agent

import "testing"

func TestGetSkillDAGForStructuredActions(t *testing.T) {
	tests := []struct {
		skill  string
		action string
	}{
		{skill: "trader_management", action: "create"},
		{skill: "trader_management", action: "update_bindings"},
		{skill: "strategy_management", action: "update_config"},
		{skill: "strategy_management", action: "update_prompt"},
		{skill: "model_management", action: "update_status"},
		{skill: "exchange_management", action: "update_name"},
	}

	for _, tt := range tests {
		dag, ok := getSkillDAG(tt.skill, tt.action)
		if !ok {
			t.Fatalf("expected DAG for %s/%s", tt.skill, tt.action)
		}
		if dag.SkillName != tt.skill || dag.Action != tt.action {
			t.Fatalf("unexpected dag identity: %+v", dag)
		}
		if len(dag.Steps) == 0 {
			t.Fatalf("expected DAG steps for %s/%s", tt.skill, tt.action)
		}
	}
}

func TestStructuredDAGsHaveTerminalStep(t *testing.T) {
	for _, dag := range listSkillDAGs() {
		hasTerminal := false
		for _, step := range dag.Steps {
			if step.Terminal {
				hasTerminal = true
				break
			}
		}
		if !hasTerminal {
			t.Fatalf("expected terminal step for %s/%s", dag.SkillName, dag.Action)
		}
	}
}

func TestStrategyUpdateConfigDAGMatchesCurrentAtomicFlow(t *testing.T) {
	dag, ok := getSkillDAG("strategy_management", "update_config")
	if !ok {
		t.Fatal("missing strategy update_config dag")
	}
	if len(dag.Steps) != 6 {
		t.Fatalf("expected 6 steps, got %d", len(dag.Steps))
	}
	if dag.Steps[0].ID != "resolve_target" {
		t.Fatalf("expected first step resolve_target, got %s", dag.Steps[0].ID)
	}
	if dag.Steps[1].ID != "resolve_config_field" {
		t.Fatalf("expected second step resolve_config_field, got %s", dag.Steps[1].ID)
	}
	if dag.Steps[2].ID != "resolve_config_value" {
		t.Fatalf("expected third step resolve_config_value, got %s", dag.Steps[2].ID)
	}
	if dag.Steps[5].ID != "execute_update" || !dag.Steps[5].Terminal {
		t.Fatalf("expected final terminal execute step, got %+v", dag.Steps[5])
	}
}
