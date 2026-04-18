package agent

import "testing"

func TestSkillRegistryLoadsDefinitions(t *testing.T) {
	names := listSkillNames()
	if len(names) < 4 {
		t.Fatalf("expected skill registry to load definitions, got %v", names)
	}

	for _, name := range []string{
		"trader_management",
		"exchange_management",
		"model_management",
		"strategy_management",
		"exchange_diagnosis",
		"model_diagnosis",
	} {
		if _, ok := getSkillDefinition(name); !ok {
			t.Fatalf("missing skill definition %q", name)
		}
	}
}

func TestTraderManagementDefinitionHasCreateAction(t *testing.T) {
	def, ok := getSkillDefinition("trader_management")
	if !ok {
		t.Fatalf("missing trader_management definition")
	}
	action, ok := def.Actions["create"]
	if !ok {
		t.Fatalf("missing create action in trader_management")
	}
	if len(action.RequiredSlots) == 0 {
		t.Fatalf("expected required slots for trader_management create action")
	}
}

func TestActionNeedsConfirmationUsesSkillDefinition(t *testing.T) {
	if !actionNeedsConfirmation("exchange_management", "delete") {
		t.Fatalf("expected exchange_management delete to require confirmation")
	}
	if actionNeedsConfirmation("exchange_management", "query") {
		t.Fatalf("did not expect exchange_management query to require confirmation")
	}
}

func TestActionRequiresSlotUsesSkillDefinition(t *testing.T) {
	if !actionRequiresSlot("model_management", "create", "provider") {
		t.Fatalf("expected model_management create to require provider")
	}
	if actionRequiresSlot("model_management", "create", "target_ref") {
		t.Fatalf("did not expect model_management create to require target_ref")
	}
}
