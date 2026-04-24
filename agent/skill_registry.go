package agent

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed skills/*.json
var embeddedSkillDefinitions embed.FS

type SkillDefinition struct {
	Name        string                           `json:"name"`
	Kind        string                           `json:"kind"`
	Domain      string                           `json:"domain"`
	Description string                           `json:"description"`
	Intents     []string                         `json:"intents,omitempty"`
	Actions     map[string]SkillActionDefinition `json:"actions,omitempty"`
	ToolMapping map[string]string                `json:"tool_mapping,omitempty"`
}

type SkillActionDefinition struct {
	Description       string   `json:"description,omitempty"`
	RequiredSlots     []string `json:"required_slots,omitempty"`
	OptionalSlots     []string `json:"optional_slots,omitempty"`
	NeedsConfirmation bool     `json:"needs_confirmation,omitempty"`
}

var skillRegistry = mustLoadSkillRegistry()

func mustLoadSkillRegistry() map[string]SkillDefinition {
	registry, err := loadSkillRegistry()
	if err != nil {
		panic(err)
	}
	return registry
}

func loadSkillRegistry() (map[string]SkillDefinition, error) {
	entries, err := embeddedSkillDefinitions.ReadDir("skills")
	if err != nil {
		return nil, err
	}

	registry := make(map[string]SkillDefinition, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := embeddedSkillDefinitions.ReadFile("skills/" + entry.Name())
		if err != nil {
			return nil, err
		}
		var def SkillDefinition
		if err := json.Unmarshal(raw, &def); err != nil {
			return nil, fmt.Errorf("parse skill definition %s: %w", entry.Name(), err)
		}
		def = normalizeSkillDefinition(def)
		if def.Name == "" {
			return nil, fmt.Errorf("skill definition %s has empty name", entry.Name())
		}
		registry[def.Name] = def
	}
	return registry, nil
}

func normalizeSkillDefinition(def SkillDefinition) SkillDefinition {
	def.Name = strings.TrimSpace(def.Name)
	def.Kind = strings.TrimSpace(def.Kind)
	def.Domain = strings.TrimSpace(def.Domain)
	def.Description = strings.TrimSpace(def.Description)
	def.Intents = cleanStringList(def.Intents)

	if len(def.Actions) > 0 {
		normalized := make(map[string]SkillActionDefinition, len(def.Actions))
		for key, action := range def.Actions {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			action.Description = strings.TrimSpace(action.Description)
			action.RequiredSlots = cleanStringList(action.RequiredSlots)
			action.OptionalSlots = cleanStringList(action.OptionalSlots)
			normalized[key] = action
		}
		def.Actions = normalized
	}

	if len(def.ToolMapping) > 0 {
		normalized := make(map[string]string, len(def.ToolMapping))
		for key, value := range def.ToolMapping {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			normalized[key] = value
		}
		def.ToolMapping = normalized
	}

	return def
}

func getSkillDefinition(name string) (SkillDefinition, bool) {
	def, ok := skillRegistry[strings.TrimSpace(name)]
	return def, ok
}

func listSkillNames() []string {
	names := make([]string, 0, len(skillRegistry))
	for name := range skillRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
