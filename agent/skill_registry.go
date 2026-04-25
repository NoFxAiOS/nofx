package agent

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed skills/*.json
var embeddedSkillDefinitions embed.FS

type SkillDefinition struct {
	Name                      string                           `json:"name"`
	Kind                      string                           `json:"kind"`
	Domain                    string                           `json:"domain"`
	Description               string                           `json:"description"`
	Intents                   []string                         `json:"intents,omitempty"`
	Actions                   map[string]SkillActionDefinition `json:"actions,omitempty"`
	ToolMapping               map[string]string                `json:"tool_mapping,omitempty"`
	FieldConstraints          map[string]SkillFieldConstraint  `json:"field_constraints,omitempty"`
	ValidationRules           []string                         `json:"validation_rules,omitempty"`
	PerExchangeRequiredFields map[string][]string              `json:"per_exchange_required_fields,omitempty"`
}

type SkillFieldConstraint struct {
	Type        string            `json:"type,omitempty"`
	Required    bool              `json:"required,omitempty"`
	Values      []string          `json:"values,omitempty"`
	Aliases     map[string]string `json:"aliases,omitempty"`
	Description string            `json:"description,omitempty"`
	RequiredFor []string          `json:"required_for,omitempty"`
	Default     any               `json:"default,omitempty"`
	Min         *float64          `json:"min,omitempty"`
	Max         *float64          `json:"max,omitempty"`
	MaxLength   int               `json:"max_length,omitempty"`
	MustBeHTTPS bool              `json:"must_be_https,omitempty"`
	Pattern     string            `json:"pattern,omitempty"`
}

type SkillActionDefinition struct {
	Description       string   `json:"description,omitempty"`
	RequiredSlots     []string `json:"required_slots,omitempty"`
	OptionalSlots     []string `json:"optional_slots,omitempty"`
	NeedsConfirmation bool     `json:"needs_confirmation,omitempty"`
	Goal              string   `json:"goal,omitempty"`
	DynamicRules      []string `json:"dynamic_rules,omitempty"`
	SuccessOutput     string   `json:"success_output,omitempty"`
	FailureOutput     string   `json:"failure_output,omitempty"`
}

var skillRegistry = mustLoadSkillRegistry()
var skillContextCache sync.Map

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
			action.Goal = strings.TrimSpace(action.Goal)
			action.DynamicRules = cleanStringList(action.DynamicRules)
			action.SuccessOutput = strings.TrimSpace(action.SuccessOutput)
			action.FailureOutput = strings.TrimSpace(action.FailureOutput)
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

	if len(def.FieldConstraints) > 0 {
		normalized := make(map[string]SkillFieldConstraint, len(def.FieldConstraints))
		for key, constraint := range def.FieldConstraints {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			constraint.Type = strings.TrimSpace(constraint.Type)
			constraint.Values = cleanStringList(constraint.Values)
			constraint.RequiredFor = cleanStringList(constraint.RequiredFor)
			constraint.Description = strings.TrimSpace(constraint.Description)
			if len(constraint.Aliases) > 0 {
				aliases := make(map[string]string, len(constraint.Aliases))
				for alias, value := range constraint.Aliases {
					alias = strings.TrimSpace(alias)
					value = strings.TrimSpace(value)
					if alias == "" || value == "" {
						continue
					}
					aliases[alias] = value
				}
				constraint.Aliases = aliases
			}
			normalized[key] = constraint
		}
		def.FieldConstraints = normalized
	}
	def.ValidationRules = cleanStringList(def.ValidationRules)
	if len(def.PerExchangeRequiredFields) > 0 {
		normalized := make(map[string][]string, len(def.PerExchangeRequiredFields))
		for key, fields := range def.PerExchangeRequiredFields {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			normalized[key] = cleanStringList(fields)
		}
		def.PerExchangeRequiredFields = normalized
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

func buildSkillRoutingSummary(lang string, skillNames []string) string {
	lines := make([]string, 0, len(skillNames))
	for _, name := range skillNames {
		def, ok := getSkillDefinition(name)
		if !ok {
			continue
		}
		parts := []string{strings.TrimSpace(def.Description)}
		switch name {
		case "trader_management":
			if lang == "zh" {
				parts = append(parts, "这个 skill 负责交易员本体和绑定关系；交易员编辑默认只换绑定，不改策略、模型、交易所的内部配置。")
			} else {
				parts = append(parts, "This skill owns the trader itself and its bindings; trader edits should switch bindings, not mutate the internals of the strategy, model, or exchange.")
			}
		case "strategy_management":
			if lang == "zh" {
				parts = append(parts, "策略模板不能直接启动；只有绑定了该策略的交易员可以启动。")
			} else {
				parts = append(parts, "Strategy templates do not run directly; only traders bound to a strategy can run.")
			}
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", name, strings.Join(cleanStringList(parts), " ")))
	}
	return strings.Join(lines, "\n")
}

func buildSkillDefinitionSummary(lang string, skillNames []string) string {
	lines := make([]string, 0, len(skillNames))
	for _, name := range skillNames {
		def, ok := getSkillDefinition(name)
		if !ok {
			continue
		}
		parts := []string{strings.TrimSpace(def.Description)}
		if action, ok := def.Actions["create"]; ok && len(action.RequiredSlots) > 0 {
			if lang == "zh" {
				parts = append(parts, "创建必填: "+formatRequiredSlotList(lang, action.RequiredSlots))
			} else {
				parts = append(parts, "create requires: "+formatRequiredSlotList(lang, action.RequiredSlots))
			}
		}
		switch name {
		case "trader_management":
			if lang == "zh" {
				parts = append(parts, "这个 skill 负责交易员本体和绑定关系；交易员编辑默认只换绑定，不改策略、模型、交易所的内部配置。")
			} else {
				parts = append(parts, "This skill owns the trader itself and its bindings; trader edits should switch bindings, not mutate the internals of the strategy, model, or exchange.")
			}
		case "strategy_management":
			if lang == "zh" {
				parts = append(parts, "策略模板不能直接启动；只有绑定了该策略的交易员可以启动。")
			} else {
				parts = append(parts, "Strategy templates do not run directly; only traders bound to a strategy can run.")
			}
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", name, strings.Join(cleanStringList(parts), " ")))
	}
	return strings.Join(lines, "\n")
}

func defaultManagementSkillNames() []string {
	return []string{
		"trader_management",
		"exchange_management",
		"model_management",
		"strategy_management",
	}
}

func buildSkillDependencySummary(lang string, session skillSession) string {
	if strings.TrimSpace(session.Name) == "" {
		return ""
	}
	switch session.Name {
	case "trader_management":
		if session.Action == "create" {
			if lang == "zh" {
				return "trader_management:create 必须收齐 4 个核心槽位：交易员名称、交易所、模型、策略。后 3 个依赖项都允许两种补法：直接选用户已有可用资源，或在当前主流程里立即新建/启用后再回流继续创建交易员。若用户是在启用、修复或新建这些依赖资源，这仍然是在继续创建交易员主流程，不是新开平级任务。"
			}
			return "trader_management:create requires 4 core slots: trader name, exchange, model, and strategy. The last 3 dependencies can be satisfied in two ways: choose an existing usable resource, or create/enable one inline and then resume trader creation. If the user is enabling, fixing, or creating one of those dependencies, that is still continuation of the trader creation flow, not a new peer task."
		}
		if lang == "zh" {
			return "当当前对象是交易员时，换绑模型、交易所、策略都属于 trader_management 的继续操作；但如果用户要改这些对象的内部配置，应切到对应 management skill。"
		}
		return "When the current object is a trader, rebinding its model, exchange, or strategy remains inside trader_management; but if the user wants to change the internals of those resources, switch to the corresponding management skill."
	default:
		return ""
	}
}

func buildSkillActionContractSummary(lang string, session skillSession) string {
	if strings.TrimSpace(session.Name) == "" || strings.TrimSpace(session.Action) == "" {
		return ""
	}

	def, ok := getSkillDefinition(session.Name)
	if !ok {
		return ""
	}
	action, ok := def.Actions[session.Action]
	if !ok {
		return ""
	}

	required := defaultIfEmpty(formatRequiredSlotList(lang, action.RequiredSlots), "无")
	goal := strings.TrimSpace(action.Goal)
	if goal == "" {
		goal = strings.TrimSpace(action.Description)
	}

	lines := []string{
		fmt.Sprintf("### Active Skill Contract: %s:%s", session.Name, session.Action),
	}
	if lang == "zh" {
		lines = append(lines, "- 目标："+defaultIfEmpty(goal, "按该动作的业务规则完成当前请求。"))
		lines = append(lines, "- 必填输入："+required)
		if len(action.DynamicRules) > 0 {
			lines = append(lines, "- 动态逻辑规则：")
			for i, rule := range action.DynamicRules {
				lines = append(lines, fmt.Sprintf("  %d. %s", i+1, rule))
			}
		}
		if action.SuccessOutput != "" || action.FailureOutput != "" {
			lines = append(lines, "- 预期输出："+strings.TrimSpace(strings.Join(cleanStringList([]string{
				ifThenElse(action.SuccessOutput != "", "成功："+action.SuccessOutput, ""),
				ifThenElse(action.FailureOutput != "", "失败："+action.FailureOutput, ""),
			}), "；")))
		}
	} else {
		lines = append(lines, "- Goal: "+defaultIfEmpty(goal, "Complete the current request under this action's business rules."))
		lines = append(lines, "- Required input: "+required)
		if len(action.DynamicRules) > 0 {
			lines = append(lines, "- Dynamic rules:")
			for i, rule := range action.DynamicRules {
				lines = append(lines, fmt.Sprintf("  %d. %s", i+1, rule))
			}
		}
		if action.SuccessOutput != "" || action.FailureOutput != "" {
			lines = append(lines, "- Expected output: "+strings.TrimSpace(strings.Join(cleanStringList([]string{
				ifThenElse(action.SuccessOutput != "", "success: "+action.SuccessOutput, ""),
				ifThenElse(action.FailureOutput != "", "failure: "+action.FailureOutput, ""),
			}), "; ")))
		}
	}
	return strings.Join(lines, "\n")
}

func ifThenElse[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func buildSkillForbiddenSummary(lang string, skillNames []string) string {
	lines := make([]string, 0, len(skillNames))
	for _, name := range skillNames {
		switch name {
		case "trader_management":
			if lang == "zh" {
				lines = append(lines, "- trader_management 不能直接设计赚钱/不亏钱方案；那类目标应交给 planner。")
				lines = append(lines, "- trader_management 不能让用户手动设置、充值或修改交易员余额；交易员初始余额应由系统自动读取绑定交易所净值。")
			} else {
				lines = append(lines, "- trader_management must not invent a profit-seeking plan; those requests belong to the planner.")
				lines = append(lines, "- trader_management must not let the user set, top up, or manually edit trader balance; trader initial balance should be auto-read from the bound exchange equity.")
			}
		case "exchange_management":
			if lang == "zh" {
				lines = append(lines, "- exchange_management 只负责保存和修改交易所配置，不负责行情查询、交易执行或诊断 API 报错。")
			} else {
				lines = append(lines, "- exchange_management only saves and updates exchange configs; it does not do market reads, trading, or API diagnosis.")
			}
		case "model_management":
			if lang == "zh" {
				lines = append(lines, "- model_management 只负责保存和修改模型配置，不负责测试连接、诊断上游错误或生成策略方案。")
			} else {
				lines = append(lines, "- model_management only saves and updates model configs; it does not test connectivity, diagnose upstream failures, or design strategies.")
			}
		case "strategy_management":
			if lang == "zh" {
				lines = append(lines, "- strategy_management 只负责模板管理；策略模板不能直接启动运行，运行态属于 trader。")
			} else {
				lines = append(lines, "- strategy_management only manages templates; strategy templates do not run directly and runtime belongs to traders.")
			}
		}
	}
	return strings.Join(lines, "\n")
}

func buildManagementSkillContext(lang string, session *skillSession) string {
	key := fmt.Sprintf("full|%s|", lang)
	if session != nil {
		key = fmt.Sprintf("full|%s|%s|%s", lang, strings.TrimSpace(session.Name), strings.TrimSpace(session.Action))
	}
	return cachedSkillContext(key, func() string {
		parts := make([]string, 0, 3)
		if summary := buildSkillDefinitionSummary(lang, defaultManagementSkillNames()); summary != "" {
			parts = append(parts, "Management skill summary:\n"+summary)
		}
		if forbidden := buildSkillForbiddenSummary(lang, defaultManagementSkillNames()); forbidden != "" {
			parts = append(parts, "Management skill negative constraints:\n"+forbidden)
		}
		if session != nil {
			if dependency := buildSkillDependencySummary(lang, *session); dependency != "" {
				parts = append(parts, "Active skill dependency summary:\n"+dependency)
			}
			if contract := buildSkillActionContractSummary(lang, *session); contract != "" {
				parts = append(parts, contract)
			}
		}
		return strings.Join(parts, "\n\n")
	})
}

func buildManagementSkillRoutingContext(lang string) string {
	return buildManagementSkillRoutingContextWithSession(lang, nil)
}

func buildSkillActionRoutingSummary(lang string, session skillSession) string {
	if strings.TrimSpace(session.Name) == "" || strings.TrimSpace(session.Action) == "" {
		return ""
	}
	def, ok := getSkillDefinition(session.Name)
	if !ok {
		return ""
	}
	action, ok := def.Actions[session.Action]
	if !ok {
		return ""
	}

	lines := []string{
		fmt.Sprintf("### Active skill routing hints: %s:%s", session.Name, session.Action),
	}
	if goal := strings.TrimSpace(action.Goal); goal != "" {
		if lang == "zh" {
			lines = append(lines, "- 当前动作目标："+goal)
		} else {
			lines = append(lines, "- Current action goal: "+goal)
		}
	}
	if dependency := buildSkillDependencySummary(lang, session); dependency != "" {
		if lang == "zh" {
			lines = append(lines, "- 当前 flow 依赖提示："+dependency)
		} else {
			lines = append(lines, "- Flow dependency hint: "+dependency)
		}
	}
	if len(action.DynamicRules) > 0 {
		if lang == "zh" {
			lines = append(lines, "- 当前动作动态规则：")
		} else {
			lines = append(lines, "- Current action dynamic rules:")
		}
		for i, rule := range action.DynamicRules {
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, rule))
		}
	}
	return strings.Join(lines, "\n")
}

func buildManagementSkillRoutingContextWithSession(lang string, session *skillSession) string {
	key := fmt.Sprintf("routing|%s|", lang)
	if session != nil {
		key = fmt.Sprintf("routing|%s|%s|%s", lang, strings.TrimSpace(session.Name), strings.TrimSpace(session.Action))
	}
	return cachedSkillContext(key, func() string {
		parts := make([]string, 0, 1)
		if summary := buildSkillRoutingSummary(lang, defaultManagementSkillNames()); summary != "" {
			parts = append(parts, "Management skill summary:\n"+summary)
		}
		if session != nil {
			if summary := buildSkillActionRoutingSummary(lang, *session); summary != "" {
				parts = append(parts, summary)
			}
		}
		return strings.Join(parts, "\n\n")
	})
}

func buildCurrentSkillExecutionContext(lang string, session skillSession) string {
	key := fmt.Sprintf("current|%s|%s|%s", lang, strings.TrimSpace(session.Name), strings.TrimSpace(session.Action))
	return cachedSkillContext(key, func() string {
		parts := make([]string, 0, 3)
		if dependency := buildSkillDependencySummary(lang, session); dependency != "" {
			parts = append(parts, "Active skill dependency summary:\n"+dependency)
		}
		if contract := buildSkillActionContractSummary(lang, session); contract != "" {
			parts = append(parts, contract)
		}
		if knowledge := buildSkillFieldKnowledgeSummary(lang, session); knowledge != "" {
			parts = append(parts, knowledge)
		}
		return strings.Join(parts, "\n\n")
	})
}

func buildSkillFieldKnowledgeSummary(lang string, session skillSession) string {
	def, ok := getSkillDefinition(session.Name)
	if !ok {
		return ""
	}
	action, hasAction := def.Actions[session.Action]
	relevant := orderedSkillFieldKeys(def, action, hasAction)
	lines := make([]string, 0, len(relevant)+6)
	title := "### Active Field Knowledge"
	if lang == "zh" {
		title = "### 当前字段知识"
	}
	lines = append(lines, title)
	for _, field := range relevant {
		constraint, ok := def.FieldConstraints[field]
		if !ok {
			continue
		}
		lines = append(lines, formatFieldKnowledgeLine(lang, field, constraint))
	}
	if len(def.PerExchangeRequiredFields) > 0 {
		if lang == "zh" {
			lines = append(lines, "- 按交易所类型的必填字段：")
		} else {
			lines = append(lines, "- Required fields by exchange type:")
		}
		keys := make([]string, 0, len(def.PerExchangeRequiredFields))
		for key := range def.PerExchangeRequiredFields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fields := make([]string, 0, len(def.PerExchangeRequiredFields[key]))
			for _, field := range def.PerExchangeRequiredFields[key] {
				fields = append(fields, fieldKnowledgeDisplayName(field, lang))
			}
			lines = append(lines, fmt.Sprintf("  - %s: %s", key, strings.Join(fields, "、")))
		}
	}
	if len(def.ValidationRules) > 0 {
		if lang == "zh" {
			lines = append(lines, "- 关键校验规则：")
		} else {
			lines = append(lines, "- Key validation rules:")
		}
		for i, rule := range def.ValidationRules {
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, rule))
		}
	}
	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func orderedSkillFieldKeys(def SkillDefinition, action SkillActionDefinition, hasAction bool) []string {
	keys := make([]string, 0, len(def.FieldConstraints))
	seen := map[string]struct{}{}
	add := func(field string) {
		field = strings.TrimSpace(field)
		if field == "" {
			return
		}
		if _, ok := def.FieldConstraints[field]; !ok {
			return
		}
		if _, ok := seen[field]; ok {
			return
		}
		seen[field] = struct{}{}
		keys = append(keys, field)
	}
	if hasAction {
		for _, field := range action.RequiredSlots {
			add(field)
		}
		for _, field := range action.OptionalSlots {
			add(field)
		}
	}
	if len(keys) == 0 {
		for field := range def.FieldConstraints {
			add(field)
		}
	}
	return keys
}

func formatFieldKnowledgeLine(lang, field string, constraint SkillFieldConstraint) string {
	parts := make([]string, 0, 8)
	if constraint.Description != "" {
		parts = append(parts, constraint.Description)
	}
	if constraint.Type != "" {
		if lang == "zh" {
			parts = append(parts, "类型="+constraint.Type)
		} else {
			parts = append(parts, "type="+constraint.Type)
		}
	}
	if constraint.Required {
		if lang == "zh" {
			parts = append(parts, "当前全局必填")
		} else {
			parts = append(parts, "globally required")
		}
	}
	if len(constraint.Values) > 0 {
		label := "可选值="
		if lang != "zh" {
			label = "values="
		}
		parts = append(parts, label+strings.Join(constraint.Values, "/"))
	}
	if len(constraint.RequiredFor) > 0 {
		label := "仅这些类型必填="
		if lang != "zh" {
			label = "required_for="
		}
		parts = append(parts, label+strings.Join(constraint.RequiredFor, "/"))
	}
	if len(constraint.Aliases) > 0 {
		aliasPairs := make([]string, 0, len(constraint.Aliases))
		keys := make([]string, 0, len(constraint.Aliases))
		for alias := range constraint.Aliases {
			keys = append(keys, alias)
		}
		sort.Strings(keys)
		for _, alias := range keys {
			aliasPairs = append(aliasPairs, alias+"->"+constraint.Aliases[alias])
		}
		label := "别名="
		if lang != "zh" {
			label = "aliases="
		}
		parts = append(parts, label+strings.Join(aliasPairs, ", "))
	}
	if constraint.MustBeHTTPS {
		if lang == "zh" {
			parts = append(parts, "必须是 HTTPS")
		} else {
			parts = append(parts, "must be HTTPS")
		}
	}
	if constraint.Min != nil || constraint.Max != nil {
		rangeText := ""
		switch {
		case constraint.Min != nil && constraint.Max != nil:
			rangeText = fmt.Sprintf("%.0f~%.0f", *constraint.Min, *constraint.Max)
		case constraint.Min != nil:
			rangeText = fmt.Sprintf(">=%.0f", *constraint.Min)
		case constraint.Max != nil:
			rangeText = fmt.Sprintf("<=%.0f", *constraint.Max)
		}
		if rangeText != "" {
			label := "范围="
			if lang != "zh" {
				label = "range="
			}
			parts = append(parts, label+rangeText)
		}
	}
	return fmt.Sprintf("- %s: %s", fieldKnowledgeDisplayName(field, lang), strings.Join(cleanStringList(parts), "；"))
}

func fieldKnowledgeDisplayName(field, lang string) string {
	if lang == "zh" {
		switch field {
		case "exchange_type":
			return "交易所类型"
		case "account_name":
			return "账户名"
		case "provider":
			return "模型提供商"
		case "custom_model_name":
			return "模型名称"
		case "custom_api_url":
			return "接口地址"
		}
	}
	return displayCatalogFieldName(field, lang)
}

func formatRequiredSlotList(lang string, slots []string) string {
	display := make([]string, 0, len(slots))
	for _, slot := range cleanStringList(slots) {
		display = append(display, slotDisplayName(slot, lang))
	}
	return strings.Join(display, "、")
}

func missingRequiredActionSlots(skillName, action string, values map[string]string) []string {
	runtime, ok := getSkillActionRuntime(skillName, action)
	if !ok {
		return nil
	}
	missing := make([]string, 0, len(runtime.Action.RequiredSlots))
	for _, slot := range runtime.Action.RequiredSlots {
		if strings.TrimSpace(values[slot]) == "" {
			missing = append(missing, slot)
		}
	}
	return missing
}
func cachedSkillContext(key string, build func() string) string {
	if cached, ok := skillContextCache.Load(key); ok {
		if s, ok := cached.(string); ok {
			return s
		}
	}
	value := build()
	skillContextCache.Store(key, value)
	return value
}
