package agent

import "strings"

type SkillDAG struct {
	SkillName string
	Action    string
	Steps     []SkillDAGStep
}

type SkillDAGStep struct {
	ID             string
	Kind           string
	RequiredFields []string
	OptionalFields []string
	Next           []string
	Terminal       bool
}

var skillDAGRegistry = buildSkillDAGRegistry()

func buildSkillDAGRegistry() map[string]SkillDAG {
	dags := []SkillDAG{
		{
			SkillName: "trader_management",
			Action:    "create",
			Steps: []SkillDAGStep{
				{ID: "resolve_name", Kind: "collect_slot", RequiredFields: []string{"name"}, Next: []string{"resolve_exchange"}},
				{ID: "resolve_exchange", Kind: "collect_slot", RequiredFields: []string{"exchange_id"}, OptionalFields: []string{"exchange_name"}, Next: []string{"resolve_model"}},
				{ID: "resolve_model", Kind: "collect_slot", RequiredFields: []string{"model_id"}, OptionalFields: []string{"model_name"}, Next: []string{"resolve_strategy"}},
				{ID: "resolve_strategy", Kind: "collect_slot", RequiredFields: []string{"strategy_id"}, OptionalFields: []string{"strategy_name"}, Next: []string{"maybe_confirm_start"}},
				{ID: "maybe_confirm_start", Kind: "branch", OptionalFields: []string{"auto_start"}, Next: []string{"await_start_confirmation", "execute_create_only"}},
				{ID: "await_start_confirmation", Kind: "confirm", RequiredFields: []string{"auto_start"}, Next: []string{"execute_create_and_start", "execute_create_only"}},
				{ID: "execute_create_only", Kind: "execute", RequiredFields: []string{"name", "exchange_id", "model_id", "strategy_id"}, Terminal: true},
				{ID: "execute_create_and_start", Kind: "execute", RequiredFields: []string{"name", "exchange_id", "model_id", "strategy_id"}, OptionalFields: []string{"auto_start"}, Terminal: true},
			},
		},
		{
			SkillName: "trader_management",
			Action:    "update_name",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_name"}},
				{ID: "collect_name", Kind: "collect_slot", RequiredFields: []string{"name"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "name"}, Terminal: true},
			},
		},
		{
			SkillName: "trader_management",
			Action:    "update_bindings",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_bindings"}},
				{ID: "collect_bindings", Kind: "collect_slot", RequiredFields: []string{"binding_update"}, OptionalFields: []string{"ai_model_id", "exchange_id", "strategy_id"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "binding_update"}, OptionalFields: []string{"ai_model_id", "exchange_id", "strategy_id"}, Terminal: true},
			},
		},
		{
			SkillName: "trader_management",
			Action:    "start",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"await_confirmation"}},
				{ID: "await_confirmation", Kind: "confirm", RequiredFields: []string{"target_ref"}, Next: []string{"execute_start"}},
				{ID: "execute_start", Kind: "execute", RequiredFields: []string{"target_ref"}, Terminal: true},
			},
		},
		{
			SkillName: "trader_management",
			Action:    "stop",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"await_confirmation"}},
				{ID: "await_confirmation", Kind: "confirm", RequiredFields: []string{"target_ref"}, Next: []string{"execute_stop"}},
				{ID: "execute_stop", Kind: "execute", RequiredFields: []string{"target_ref"}, Terminal: true},
			},
		},
		{
			SkillName: "trader_management",
			Action:    "delete",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"await_confirmation"}},
				{ID: "await_confirmation", Kind: "confirm", RequiredFields: []string{"target_ref"}, Next: []string{"execute_delete"}},
				{ID: "execute_delete", Kind: "execute", RequiredFields: []string{"target_ref"}, Terminal: true},
			},
		},
		{
			SkillName: "strategy_management",
			Action:    "create",
			Steps: []SkillDAGStep{
				{ID: "resolve_name", Kind: "collect_slot", RequiredFields: []string{"name"}, OptionalFields: []string{"lang", "description", "config"}, Next: []string{"execute_create"}},
				{ID: "execute_create", Kind: "execute", RequiredFields: []string{"name"}, OptionalFields: []string{"lang", "description", "config"}, Terminal: true},
			},
		},
		{
			SkillName: "strategy_management",
			Action:    "update_name",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_name"}},
				{ID: "collect_name", Kind: "collect_slot", RequiredFields: []string{"name"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "name"}, Terminal: true},
			},
		},
		{
			SkillName: "strategy_management",
			Action:    "update_prompt",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_prompt"}},
				{ID: "collect_prompt", Kind: "collect_slot", RequiredFields: []string{"prompt"}, Next: []string{"load_config"}},
				{ID: "load_config", Kind: "load_state", RequiredFields: []string{"target_ref"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "prompt"}, Terminal: true},
			},
		},
		{
			SkillName: "strategy_management",
			Action:    "update_config",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"resolve_config_field"}},
				{ID: "resolve_config_field", Kind: "collect_slot", RequiredFields: []string{"config_field"}, Next: []string{"resolve_config_value"}},
				{ID: "resolve_config_value", Kind: "collect_slot", RequiredFields: []string{"config_value"}, Next: []string{"load_config"}},
				{ID: "load_config", Kind: "load_state", RequiredFields: []string{"target_ref"}, Next: []string{"apply_field_update"}},
				{ID: "apply_field_update", Kind: "transform", RequiredFields: []string{"config_field", "config_value"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "config_field", "config_value"}, Terminal: true},
			},
		},
		{
			SkillName: "strategy_management",
			Action:    "duplicate",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_name"}},
				{ID: "collect_name", Kind: "collect_slot", RequiredFields: []string{"name"}, Next: []string{"execute_duplicate"}},
				{ID: "execute_duplicate", Kind: "execute", RequiredFields: []string{"target_ref", "name"}, Terminal: true},
			},
		},
		{
			SkillName: "strategy_management",
			Action:    "activate",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"execute_activate"}},
				{ID: "execute_activate", Kind: "execute", RequiredFields: []string{"target_ref"}, Terminal: true},
			},
		},
		{
			SkillName: "strategy_management",
			Action:    "delete",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"await_confirmation"}},
				{ID: "await_confirmation", Kind: "confirm", RequiredFields: []string{"target_ref"}, Next: []string{"execute_delete"}},
				{ID: "execute_delete", Kind: "execute", RequiredFields: []string{"target_ref"}, Terminal: true},
			},
		},
		{
			SkillName: "model_management",
			Action:    "create",
			Steps: []SkillDAGStep{
				{ID: "resolve_provider", Kind: "collect_slot", RequiredFields: []string{"provider"}, Next: []string{"collect_optional_fields"}},
				{ID: "collect_optional_fields", Kind: "collect_slot", OptionalFields: []string{"name", "custom_api_url", "custom_model_name"}, Next: []string{"execute_create"}},
				{ID: "execute_create", Kind: "execute", RequiredFields: []string{"provider"}, OptionalFields: []string{"name", "custom_api_url", "custom_model_name"}, Terminal: true},
			},
		},
		{
			SkillName: "model_management",
			Action:    "update_status",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_enabled"}},
				{ID: "collect_enabled", Kind: "collect_slot", RequiredFields: []string{"enabled"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "enabled"}, Terminal: true},
			},
		},
		{
			SkillName: "model_management",
			Action:    "update_endpoint",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_custom_api_url"}},
				{ID: "collect_custom_api_url", Kind: "collect_slot", RequiredFields: []string{"custom_api_url"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "custom_api_url"}, Terminal: true},
			},
		},
		{
			SkillName: "model_management",
			Action:    "update_name",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_custom_model_name"}},
				{ID: "collect_custom_model_name", Kind: "collect_slot", RequiredFields: []string{"custom_model_name"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "custom_model_name"}, Terminal: true},
			},
		},
		{
			SkillName: "model_management",
			Action:    "delete",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"await_confirmation"}},
				{ID: "await_confirmation", Kind: "confirm", RequiredFields: []string{"target_ref"}, Next: []string{"execute_delete"}},
				{ID: "execute_delete", Kind: "execute", RequiredFields: []string{"target_ref"}, Terminal: true},
			},
		},
		{
			SkillName: "exchange_management",
			Action:    "create",
			Steps: []SkillDAGStep{
				{ID: "resolve_exchange_type", Kind: "collect_slot", RequiredFields: []string{"exchange_type"}, Next: []string{"collect_account_name"}},
				{ID: "collect_account_name", Kind: "collect_slot", OptionalFields: []string{"account_name"}, Next: []string{"execute_create"}},
				{ID: "execute_create", Kind: "execute", RequiredFields: []string{"exchange_type"}, OptionalFields: []string{"account_name"}, Terminal: true},
			},
		},
		{
			SkillName: "exchange_management",
			Action:    "update_name",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_account_name"}},
				{ID: "collect_account_name", Kind: "collect_slot", RequiredFields: []string{"account_name"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "account_name"}, Terminal: true},
			},
		},
		{
			SkillName: "exchange_management",
			Action:    "update_status",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"collect_enabled"}},
				{ID: "collect_enabled", Kind: "collect_slot", RequiredFields: []string{"enabled"}, Next: []string{"execute_update"}},
				{ID: "execute_update", Kind: "execute", RequiredFields: []string{"target_ref", "enabled"}, Terminal: true},
			},
		},
		{
			SkillName: "exchange_management",
			Action:    "delete",
			Steps: []SkillDAGStep{
				{ID: "resolve_target", Kind: "resolve_target", RequiredFields: []string{"target_ref"}, Next: []string{"await_confirmation"}},
				{ID: "await_confirmation", Kind: "confirm", RequiredFields: []string{"target_ref"}, Next: []string{"execute_delete"}},
				{ID: "execute_delete", Kind: "execute", RequiredFields: []string{"target_ref"}, Terminal: true},
			},
		},
	}

	registry := make(map[string]SkillDAG, len(dags))
	for _, dag := range dags {
		dag = normalizeSkillDAG(dag)
		if dag.SkillName == "" || dag.Action == "" {
			continue
		}
		registry[skillDAGKey(dag.SkillName, dag.Action)] = dag
	}
	return registry
}

func normalizeSkillDAG(dag SkillDAG) SkillDAG {
	dag.SkillName = strings.TrimSpace(dag.SkillName)
	dag.Action = strings.TrimSpace(dag.Action)
	steps := make([]SkillDAGStep, 0, len(dag.Steps))
	for _, step := range dag.Steps {
		step.ID = strings.TrimSpace(step.ID)
		step.Kind = strings.TrimSpace(step.Kind)
		step.RequiredFields = cleanStringList(step.RequiredFields)
		step.OptionalFields = cleanStringList(step.OptionalFields)
		step.Next = cleanStringList(step.Next)
		if step.ID == "" {
			continue
		}
		steps = append(steps, step)
	}
	dag.Steps = steps
	return dag
}

func skillDAGKey(skillName, action string) string {
	return strings.TrimSpace(skillName) + ":" + strings.TrimSpace(action)
}

func getSkillDAG(skillName, action string) (SkillDAG, bool) {
	dag, ok := skillDAGRegistry[skillDAGKey(skillName, action)]
	return dag, ok
}

func listSkillDAGs() []SkillDAG {
	out := make([]SkillDAG, 0, len(skillDAGRegistry))
	for _, dag := range skillDAGRegistry {
		out = append(out, dag)
	}
	return out
}

