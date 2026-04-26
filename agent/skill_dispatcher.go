package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type skillSession struct {
	Name      string                  `json:"name,omitempty"`
	Action    string                  `json:"action,omitempty"`
	Phase     string                  `json:"phase,omitempty"`
	TargetRef *EntityReference        `json:"target_ref,omitempty"`
	Fields    map[string]string       `json:"fields,omitempty"`
	Slots     *createTraderSkillSlots `json:"slots,omitempty"`
	UpdatedAt string                  `json:"updated_at,omitempty"`
}

type createTraderSkillSlots struct {
	Name         string `json:"name,omitempty"`
	ExchangeID   string `json:"exchange_id,omitempty"`
	ExchangeName string `json:"exchange_name,omitempty"`
	ModelID      string `json:"model_id,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	StrategyID   string `json:"strategy_id,omitempty"`
	StrategyName string `json:"strategy_name,omitempty"`
	AutoStart    *bool  `json:"auto_start,omitempty"`
}

type traderSkillOption struct {
	ID      string
	Name    string
	Enabled bool
	Hint    string
}

func skillSessionConfigKey(userID int64) string {
	return fmt.Sprintf("agent_skill_session_%d", userID)
}

func normalizeSkillSession(session skillSession) skillSession {
	session.Name = strings.TrimSpace(session.Name)
	session.Action = strings.TrimSpace(session.Action)
	session.Phase = strings.TrimSpace(session.Phase)
	session.TargetRef = normalizeEntityReference(session.TargetRef)
	if len(session.Fields) > 0 {
		normalized := make(map[string]string, len(session.Fields))
		for key, value := range session.Fields {
			key = normalizeFieldKey(&session, key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			normalized[key] = value
		}
		if len(normalized) > 0 {
			session.Fields = normalized
		} else {
			session.Fields = nil
		}
	}
	if session.Slots != nil {
		ensureSkillFields(&session)
		session.Slots.Name = strings.TrimSpace(session.Slots.Name)
		session.Slots.ExchangeID = strings.TrimSpace(session.Slots.ExchangeID)
		session.Slots.ExchangeName = strings.TrimSpace(session.Slots.ExchangeName)
		session.Slots.ModelID = strings.TrimSpace(session.Slots.ModelID)
		session.Slots.ModelName = strings.TrimSpace(session.Slots.ModelName)
		session.Slots.StrategyID = strings.TrimSpace(session.Slots.StrategyID)
		session.Slots.StrategyName = strings.TrimSpace(session.Slots.StrategyName)
		if session.Slots.Name != "" {
			session.Fields["name"] = session.Slots.Name
		}
		if session.Slots.ExchangeID != "" {
			session.Fields["exchange_id"] = session.Slots.ExchangeID
		}
		if session.Slots.ExchangeName != "" {
			session.Fields["exchange_name"] = session.Slots.ExchangeName
		}
		if session.Slots.ModelID != "" {
			session.Fields["model_id"] = session.Slots.ModelID
		}
		if session.Slots.ModelName != "" {
			session.Fields["model_name"] = session.Slots.ModelName
		}
		if session.Slots.StrategyID != "" {
			session.Fields["strategy_id"] = session.Slots.StrategyID
		}
		if session.Slots.StrategyName != "" {
			session.Fields["strategy_name"] = session.Slots.StrategyName
		}
		if session.Slots.AutoStart != nil {
			if *session.Slots.AutoStart {
				session.Fields["auto_start"] = "true"
			} else {
				session.Fields["auto_start"] = "false"
			}
		}
		syncTraderCreateSlotMirror(&session)
		if fieldValue(session, "name") == "" &&
			fieldValue(session, "exchange_id") == "" &&
			fieldValue(session, "model_id") == "" &&
			fieldValue(session, "strategy_id") == "" &&
			fieldValue(session, "exchange_name") == "" &&
			fieldValue(session, "model_name") == "" &&
			fieldValue(session, "strategy_name") == "" &&
			fieldValue(session, "auto_start") == "" {
			session.Slots = nil
		}
	}
	if session.Name == "" {
		return skillSession{}
	}
	if session.UpdatedAt == "" {
		session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return session
}

func (a *Agent) getSkillSession(userID int64) skillSession {
	if a.store == nil {
		return skillSession{}
	}
	raw, err := a.store.GetSystemConfig(skillSessionConfigKey(userID))
	if err != nil || strings.TrimSpace(raw) == "" {
		return skillSession{}
	}
	var session skillSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return skillSession{}
	}
	return normalizeSkillSession(session)
}

func (a *Agent) saveSkillSession(userID int64, session skillSession) {
	if a.store == nil {
		return
	}
	session = normalizeSkillSession(session)
	if session.Name == "" {
		_ = a.store.SetSystemConfig(skillSessionConfigKey(userID), "")
		return
	}
	data, err := json.Marshal(session)
	if err != nil {
		return
	}
	_ = a.store.SetSystemConfig(skillSessionConfigKey(userID), string(data))
}

func (a *Agent) clearSkillSession(userID int64) {
	if a.store == nil {
		return
	}
	_ = a.store.SetSystemConfig(skillSessionConfigKey(userID), "")
}

func isYesReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, candidate := range []string{"是", "好", "好的", "确认", "确认启动", "确认创建", "要", "启动", "开始", "yes", "y", "ok", "confirm", "go ahead"} {
		if lower == candidate {
			return true
		}
	}
	return false
}

func isNoReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, candidate := range []string{"不", "不用", "先不用", "取消", "不要", "no", "n", "cancel", "stop"} {
		if lower == candidate {
			return true
		}
	}
	return false
}

func isCancelSkillReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch lower {
	case "取消", "/cancel", "cancel", "不改", "先不改", "算了", "先不用", "不用了", "不弄了", "不搞了", "换话题", "换话题了", "聊别的", "先聊别的":
		return true
	default:
		return false
	}
}

func normalizeTraderDraftName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, prefix := range []string{"名称：", "名称:", "名字：", "名字:", "name:", "name："} {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			value = strings.TrimSpace(value[len(prefix):])
			break
		}
	}
	for _, sep := range []string{"交易所：", "交易所:", "模型：", "模型:", "策略：", "策略:", "exchange:", "model:", "strategy:"} {
		if idx := strings.Index(strings.ToLower(value), strings.ToLower(sep)); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	for _, sep := range []string{"，", ",", "。", "；", ";", "\n"} {
		if idx := strings.Index(value, sep); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	return strings.Trim(value, "“”\"'：: ")
}

func choosePreferredOption(options []traderSkillOption) *traderSkillOption {
	if len(options) == 1 {
		copy := options[0]
		return &copy
	}
	enabled := make([]traderSkillOption, 0, len(options))
	for _, option := range options {
		if option.Enabled {
			enabled = append(enabled, option)
		}
	}
	if len(enabled) == 1 {
		copy := enabled[0]
		return &copy
	}
	return nil
}

func formatOptionList(prefix string, options []traderSkillOption) string {
	parts := make([]string, 0, len(options))
	for _, option := range options {
		label := option.Name
		if label == "" {
			label = option.ID
		}
		if option.Enabled {
			label += "（已启用）"
		} else {
			label += "（已禁用）"
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return ""
	}
	return prefix + strings.Join(parts, "、")
}

func parseSkillError(raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		if msg, _ := payload["error"].(string); strings.TrimSpace(msg) != "" {
			return strings.TrimSpace(msg)
		}
	}
	return strings.TrimSpace(raw)
}

func (a *Agent) loadEnabledModelOptions(storeUserID string) []traderSkillOption {
	if a.store == nil {
		return nil
	}
	models, err := a.store.AIModel().List(storeUserID)
	if err != nil {
		return nil
	}
	out := make([]traderSkillOption, 0, len(models))
	for _, model := range models {
		name := strings.TrimSpace(model.Name)
		if name == "" {
			name = strings.TrimSpace(model.ID)
		}
		hint := strings.Join(cleanStringList([]string{
			strings.TrimSpace(model.CustomModelName),
			strings.TrimSpace(model.Provider),
		}), " / ")
		out = append(out, traderSkillOption{ID: model.ID, Name: name, Hint: hint, Enabled: model.Enabled})
	}
	return out
}

func (a *Agent) loadExchangeOptions(storeUserID string) []traderSkillOption {
	if a.store == nil {
		return nil
	}
	exchanges, err := a.store.Exchange().List(storeUserID)
	if err != nil {
		return nil
	}
	out := make([]traderSkillOption, 0, len(exchanges))
	for _, exchange := range exchanges {
		name := strings.TrimSpace(exchange.AccountName)
		if name == "" {
			name = strings.TrimSpace(exchange.ExchangeType)
		}
		out = append(out, traderSkillOption{ID: exchange.ID, Name: name, Enabled: exchange.Enabled})
	}
	return out
}

func (a *Agent) loadStrategyOptions(storeUserID string) []traderSkillOption {
	if a.store == nil {
		return nil
	}
	strategies, err := a.store.Strategy().List(storeUserID)
	if err != nil {
		return nil
	}
	out := make([]traderSkillOption, 0, len(strategies))
	for _, strategy := range strategies {
		out = append(out, traderSkillOption{ID: strategy.ID, Name: strategy.Name, Enabled: true})
	}
	return out
}

func (a *Agent) buildTraderCreateConversationResources(storeUserID string, session skillSession) map[string]any {
	missing := missingFieldKeysForSkillSession(session)
	needExchange := false
	needModel := false
	needStrategy := false
	for _, field := range missing {
		switch strings.TrimSpace(field) {
		case "exchange_name", "exchange_id", "exchange":
			needExchange = true
		case "model_name", "model_id", "ai_model_id", "model":
			needModel = true
		case "strategy_name", "strategy_id", "strategy":
			needStrategy = true
		}
	}
	resources := map[string]any{}
	if needExchange {
		resources["exchanges"] = a.loadExchangeOptions(storeUserID)
	}
	if needModel {
		resources["models"] = a.loadEnabledModelOptions(storeUserID)
	}
	if needStrategy {
		resources["strategies"] = a.loadStrategyOptions(storeUserID)
	}
	return resources
}

func (a *Agent) tryHardSkill(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool) {
	if ctx != nil && ctx.Err() != nil {
		return "", false
	}
	emptySession := skillSession{}
	if hasExplicitCreateIntentForDomain(text, "trader") {
		answer, handled := a.handleCreateTraderSkill(storeUserID, userID, lang, text, emptySession)
		if handled {
			a.recordSkillInteraction(userID, text, answer)
			if onEvent != nil {
				onEvent(StreamEventTool, "hard_skill:trader_management:create")
				emitStreamText(onEvent, answer)
			}
			return answer, true
		}
	}
	if hasExplicitDiagnosisIntentForDomain(text, "model") {
		answer := a.handleModelDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:model_diagnosis")
			emitStreamText(onEvent, answer)
		}
		return answer, true
	}
	if hasExplicitDiagnosisIntentForDomain(text, "exchange") {
		answer := a.handleExchangeDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:exchange_diagnosis")
			emitStreamText(onEvent, answer)
		}
		return answer, true
	}
	if hasExplicitDiagnosisIntentForDomain(text, "trader") {
		answer := a.handleTraderDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:trader_diagnosis")
			emitStreamText(onEvent, answer)
		}
		return answer, true
	}
	if hasExplicitDiagnosisIntentForDomain(text, "strategy") {
		answer := a.handleStrategyDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:strategy_diagnosis")
			emitStreamText(onEvent, answer)
		}
		return answer, true
	}
	return "", false
}

func (a *Agent) recordSkillInteraction(userID int64, userText, answer string) {
	if a.history == nil {
		a.history = newChatHistory(chatHistoryMaxTurns)
	}
	a.history.Add(userID, "user", userText)
	a.history.Add(userID, "assistant", answer)
}

func (a *Agent) rerouteRejectedSkillFlow(ctx context.Context, storeUserID string, userID int64, lang, text string) (string, bool) {
	a.clearSkillSession(userID)
	if a == nil || a.aiClient == nil {
		return "", false
	}
	if answer, handled, err := a.tryLLMIntentRoute(ctx, storeUserID, userID, lang, text, nil); err == nil && handled {
		return answer, true
	}
	if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, nil); ok {
		return answer, true
	}
	if answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, nil); err == nil && strings.TrimSpace(answer) != "" {
		return answer, true
	}
	return "", false
}

func ensureSkillFields(session *skillSession) {
	if session.Fields == nil {
		session.Fields = make(map[string]string)
	}
}

func (a *Agent) handleCreateTraderSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	if session.Name == "" {
		session = skillSession{
			Name:   "trader_management",
			Action: "create",
			Phase:  "collecting",
			Fields: map[string]string{},
		}
	}
	if session.Fields == nil {
		session.Fields = map[string]string{}
	}
	syncTraderCreateSlotMirror(&session)

	if session.Phase == "await_start_confirmation" {
		switch {
		case isYesReply(text):
			return a.executeCreateTraderSkill(storeUserID, userID, lang, session, true), true
		case isNoReply(text):
			return a.executeCreateTraderSkill(storeUserID, userID, lang, session, false), true
		}
	}
	if session.Phase == "await_create_confirmation" {
		switch {
		case isYesReply(text):
			return a.executeCreateTraderSkill(storeUserID, userID, lang, session, false), true
		case isNoReply(text), isCancelSkillReply(text):
			session.Phase = "collecting"
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "好的，那我先不创建。你也可以继续改名称、交易所、模型或策略。", true
			}
			return "Okay, I won't create it yet. You can keep adjusting the name, exchange, model, or strategy.", true
		}
	}

	a.hydrateCreateTraderSlotReferences(storeUserID, &session)
	if fieldValue(session, "exchange_id") != "" && fieldValue(session, "model_id") != "" && fieldValue(session, "strategy_id") != "" {
		if err := a.validateTraderDraft(storeUserID, fieldValue(session, "model_id"), fieldValue(session, "exchange_id"), fieldValue(session, "strategy_id")); err != nil {
			session.Phase = "collecting"
			a.saveSkillSession(userID, session)
			return formatValidationFeedback(lang, "trader", err), true
		}
	}
	if missing := missingFieldKeysForSkillSession(session); len(missing) > 0 {
		session.Phase = "collecting"
		a.saveSkillSession(userID, session)
		return a.buildTraderCreateMissingPrompt(storeUserID, lang, session, a.buildTraderCreateConversationResources(storeUserID, session)), true
	}

	if stillMissing := missingFieldKeysForSkillSession(session); len(stillMissing) > 0 {
		session.Phase = "collecting"
		a.saveSkillSession(userID, session)
		return a.buildTraderCreateMissingPrompt(storeUserID, lang, session, a.buildTraderCreateConversationResources(storeUserID, session)), true
	}

	if fieldValue(session, "auto_start") == "true" {
		session.Phase = "await_start_confirmation"
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return fmt.Sprintf("准备创建交易员并立即启动。\n交易所：%s\n模型：%s\n策略：%s\n\n回复确认继续，回复先不用则只创建不启动。",
				traderCreateExchangeNameOrID(session), traderCreateModelNameOrID(session), traderCreateStrategyNameOrID(session)), true
		}
		return fmt.Sprintf("Ready to create trader and start it immediately.\nExchange: %s\nModel: %s\nStrategy: %s\n\nReply confirm to continue, or no to create without starting.",
			traderCreateExchangeNameOrID(session), traderCreateModelNameOrID(session), traderCreateStrategyNameOrID(session)), true
	}

	session.Phase = "await_create_confirmation"
	a.saveSkillSession(userID, session)
	return formatTraderCreateDraftSummary(lang, session), true
}

func (s *createTraderSkillSlots) ExchangeNameOrID() string {
	if strings.TrimSpace(s.ExchangeName) != "" {
		return s.ExchangeName
	}
	return s.ExchangeID
}

func (s *createTraderSkillSlots) ModelNameOrID() string {
	if strings.TrimSpace(s.ModelName) != "" {
		return s.ModelName
	}
	return s.ModelID
}

func (s *createTraderSkillSlots) StrategyNameOrID() string {
	if strings.TrimSpace(s.StrategyName) != "" {
		return s.StrategyName
	}
	return s.StrategyID
}

func traderCreateExchangeNameOrID(session skillSession) string {
	if value := fieldValue(session, "exchange_name"); value != "" {
		return value
	}
	return fieldValue(session, "exchange_id")
}

func traderCreateModelNameOrID(session skillSession) string {
	if value := fieldValue(session, "model_name"); value != "" {
		return value
	}
	return fieldValue(session, "model_id")
}

func traderCreateStrategyNameOrID(session skillSession) string {
	if value := fieldValue(session, "strategy_name"); value != "" {
		return value
	}
	return fieldValue(session, "strategy_id")
}

func renderSkillMissingLabels(lang string, missing []string) []string {
	out := make([]string, 0, len(missing))
	for _, field := range missing {
		out = append(out, slotDisplayName(field, lang))
	}
	return out
}

func (a *Agent) buildTraderCreateMissingPrompt(storeUserID, lang string, session skillSession, availableResources map[string]any) string {
	missing := missingFieldKeysForSkillSession(session)
	missingLabels := strings.Join(renderSkillMissingLabels(lang, missing), "、")
	prereqs := make([]string, 0, 3)
	optionLines := make([]string, 0, 3)
	if exchanges, _ := availableResources["exchanges"].([]traderSkillOption); len(exchanges) == 0 && containsString(missing, "exchange_name") {
		if lang == "zh" {
			prereqs = append(prereqs, "当前还没有可用交易所配置")
		} else {
			prereqs = append(prereqs, "there is no exchange config yet")
		}
	} else if containsString(missing, "exchange_name") {
		if list := formatOptionList("现有交易所：", exchanges); lang == "zh" && list != "" {
			optionLines = append(optionLines, list)
		} else if list := formatOptionList("Available exchanges:", exchanges); lang != "zh" && list != "" {
			optionLines = append(optionLines, list)
		}
	}
	if models, _ := availableResources["models"].([]traderSkillOption); len(models) == 0 && containsString(missing, "model_name") {
		if lang == "zh" {
			prereqs = append(prereqs, "当前还没有可用模型配置")
		} else {
			prereqs = append(prereqs, "there is no model config yet")
		}
	} else if containsString(missing, "model_name") {
		if list := formatOptionList("现有模型：", models); lang == "zh" && list != "" {
			optionLines = append(optionLines, list)
		} else if list := formatOptionList("Available models:", models); lang != "zh" && list != "" {
			optionLines = append(optionLines, list)
		}
	}
	if strategies, _ := availableResources["strategies"].([]traderSkillOption); len(strategies) == 0 && containsString(missing, "strategy_name") {
		if lang == "zh" {
			prereqs = append(prereqs, "当前还没有可用策略")
		} else {
			prereqs = append(prereqs, "there is no strategy yet")
		}
	} else if containsString(missing, "strategy_name") {
		if list := formatOptionList("现有策略：", strategies); lang == "zh" && list != "" {
			optionLines = append(optionLines, list)
		} else if list := formatOptionList("Available strategies:", strategies); lang != "zh" && list != "" {
			optionLines = append(optionLines, list)
		}
	}
	if lang == "zh" {
		reply := "新建交易员还缺这些槽位：" + missingLabels + "。"
		if len(prereqs) > 0 {
			reply += "\n" + strings.Join(prereqs, "；") + "。"
		}
		if len(optionLines) > 0 {
			reply += "\n" + strings.Join(optionLines, "\n")
		}
		return reply
	}
	reply := "Creating the trader still needs these slots: " + strings.Join(renderSkillMissingLabels(lang, missing), ", ") + "."
	if len(prereqs) > 0 {
		reply += "\n" + strings.Join(prereqs, "; ") + "."
	}
	if len(optionLines) > 0 {
		reply += "\n" + strings.Join(optionLines, "\n")
	}
	return reply
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func shouldPreserveTraderCreateSessionOnError(errMsg string) bool {
	lower := strings.ToLower(strings.TrimSpace(errMsg))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "exchange is disabled") ||
		strings.Contains(lower, "exchange_id is required") ||
		strings.Contains(lower, "model_id is required") ||
		strings.Contains(lower, "strategy_id is required")
}

func (a *Agent) executeCreateTraderSkill(storeUserID string, userID int64, lang string, session skillSession, startAfterCreate bool) string {
	a.hydrateCreateTraderSlotReferences(storeUserID, &session)
	normalizedArgs, _ := normalizeTraderArgsToManualLimits(lang, buildTraderUpdateArgsFromSession(session))
	args := manageTraderArgs{
		Action:              "create",
		Name:                fieldValue(session, "name"),
		AIModelID:           fieldValue(session, "model_id"),
		ExchangeID:          fieldValue(session, "exchange_id"),
		StrategyID:          fieldValue(session, "strategy_id"),
		ScanIntervalMinutes: normalizedArgs.ScanIntervalMinutes,
		IsCrossMargin:       normalizedArgs.IsCrossMargin,
		ShowInCompetition:   normalizedArgs.ShowInCompetition,
	}
	createRaw := a.toolCreateTrader(storeUserID, args)
	if errMsg := parseSkillError(createRaw); errMsg != "" && strings.Contains(createRaw, `"error"`) {
		if shouldPreserveTraderCreateSessionOnError(errMsg) {
			session.Phase = "collecting"
			a.saveSkillSession(userID, session)
		} else {
			a.clearSkillSession(userID)
		}
		if strings.Contains(strings.ToLower(errMsg), "exchange is disabled") {
			exchanges := a.loadExchangeOptions(storeUserID)
			if lang == "zh" {
				reply := fmt.Sprintf("创建交易员失败：你选的交易所“%s”当前已禁用，请换一个已启用的交易所。", traderCreateExchangeNameOrID(session))
				if list := formatOptionList("可用交易所：", exchanges); list != "" {
					reply += "\n" + list
				}
				return reply
			}
			reply := fmt.Sprintf("That trader could not be created because the exchange %q is turned off. Please choose one that is enabled.", traderCreateExchangeNameOrID(session))
			if list := formatOptionList("Available exchanges:", exchanges); list != "" {
				reply += "\n" + list
			}
			return reply
		}
		if lang == "zh" {
			return "创建交易员失败：" + errMsg
		}
		return "That create request did not go through: " + errMsg
	}
	var created struct {
		Trader safeTraderToolConfig `json:"trader"`
	}
	if err := json.Unmarshal([]byte(createRaw), &created); err != nil || created.Trader.ID == "" {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "交易员创建后返回结果异常，请稍后到列表里确认。"
		}
		return "The trader was created but the response could not be verified. Please check the trader list."
	}

	if !startAfterCreate {
		setSkillDAGStep(&session, "execute_create_only")
		a.clearSkillSession(userID)
		if lang == "zh" {
			return fmt.Sprintf("已创建交易员“%s”。\n交易所：%s\n模型：%s\n策略：%s\n当前状态：未启动。",
				created.Trader.Name, traderCreateExchangeNameOrID(session), traderCreateModelNameOrID(session), traderCreateStrategyNameOrID(session))
		}
		return fmt.Sprintf("Created trader %q.\nExchange: %s\nModel: %s\nStrategy: %s\nCurrent status: not started.",
			created.Trader.Name, traderCreateExchangeNameOrID(session), traderCreateModelNameOrID(session), traderCreateStrategyNameOrID(session))
	}

	setSkillDAGStep(&session, "execute_create_and_start")
	startRaw := a.toolStartTrader(storeUserID, created.Trader.ID)
	if errMsg := parseSkillError(startRaw); errMsg != "" && strings.Contains(startRaw, `"error"`) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return fmt.Sprintf("交易员“%s”已创建，但启动失败：%s", created.Trader.Name, errMsg)
		}
		return fmt.Sprintf("Trader %q was created, but starting it failed: %s", created.Trader.Name, errMsg)
	}

	a.clearSkillSession(userID)
	if lang == "zh" {
		return fmt.Sprintf("已创建并启动交易员“%s”。\n交易所：%s\n模型：%s\n策略：%s",
			created.Trader.Name, traderCreateExchangeNameOrID(session), traderCreateModelNameOrID(session), traderCreateStrategyNameOrID(session))
	}
	return fmt.Sprintf("Created and started trader %q.\nExchange: %s\nModel: %s\nStrategy: %s",
		created.Trader.Name, traderCreateExchangeNameOrID(session), traderCreateModelNameOrID(session), traderCreateStrategyNameOrID(session))
}

func (a *Agent) handleModelDiagnosisSkill(storeUserID, lang, text string) string {
	raw := a.toolGetModelConfigs(storeUserID)
	errMsg := parseSkillError(raw)
	if errMsg != "" && strings.Contains(raw, `"error"`) {
		if lang == "zh" {
			return "现象：模型配置读取失败。\n更可能原因：当前存储不可用或配置列表读取失败。\n下一步：请稍后重试，或先检查后端日志。"
		}
		return "Symptom: failed to read model configs.\nLikely cause: the store is unavailable or loading configs failed.\nNext step: retry later or check backend logs."
	}

	var payload struct {
		ModelConfigs []safeModelToolConfig `json:"model_configs"`
	}
	_ = json.Unmarshal([]byte(raw), &payload)

	if len(payload.ModelConfigs) == 0 {
		if lang == "zh" {
			return "现象：当前没有任何模型配置。\n更可能原因：还没创建模型绑定。\n先检查什么：先确认你要使用哪个 provider。\n下一步：先新增并启用一个模型配置，再继续排查。"
		}
		return "Symptom: there are no model configs yet.\nLikely cause: no model binding has been created.\nNext step: create and enable a model config first."
	}

	enabledCount := 0
	var incomplete []string
	for _, model := range payload.ModelConfigs {
		if model.Enabled {
			enabledCount++
		}
		if model.Enabled && (!model.HasAPIKey || strings.TrimSpace(model.CustomAPIURL) == "") {
			incomplete = append(incomplete, model.Name)
		}
	}

	lines := make([]string, 0, 6)
	if lang == "zh" {
		lines = append(lines, "现象：这是模型配置/调用失败类问题。")
		switch {
		case enabledCount == 0:
			lines = append(lines, "更可能原因：当前没有已启用模型。")
		case len(incomplete) > 0:
			lines = append(lines, "更可能原因：已启用模型里至少有一项缺少 API Key 或 custom_api_url，例如："+strings.Join(incomplete, "、")+"。")
		case containsAny(strings.ToLower(text), []string{"custom_api_url", "url", "https"}):
			lines = append(lines, "更可能原因：custom_api_url 不是合法 HTTPS 地址，后端会直接拒绝保存。")
		default:
			lines = append(lines, "更可能原因：模型已保存，但 custom_model_name、API Key 或 provider 运行配置不匹配。")
		}
		lines = append(lines, "先检查什么：")
		lines = append(lines, fmt.Sprintf("1. 当前共 %d 个模型配置，已启用 %d 个。", len(payload.ModelConfigs), enabledCount))
		lines = append(lines, "2. 检查目标模型是否同时具备 enabled、API Key、custom_api_url。")
		lines = append(lines, "3. 如果是 OpenAI / Claude / DeepSeek 等 provider，确认 model name 填的是该 provider 实际可用的模型名。")
		if excerpt := backendLogDiagnosisExcerpt(lang, text, "model"); excerpt != "" {
			lines = append(lines, excerpt)
		}
		lines = append(lines, "下一步：如果你愿意，我下一步可以继续帮你逐项检查你当前配置里的具体模型。")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "Symptom: this looks like a model configuration or model runtime issue.")
	switch {
	case enabledCount == 0:
		lines = append(lines, "Likely cause: there is no enabled model.")
	case len(incomplete) > 0:
		lines = append(lines, "Likely cause: at least one enabled model is missing an API key or custom_api_url, for example: "+strings.Join(incomplete, ", ")+".")
	default:
		lines = append(lines, "Likely cause: the model was saved, but the API key, custom_api_url, or custom_model_name does not match the provider runtime config.")
	}
	lines = append(lines, fmt.Sprintf("Check first: %d model configs exist, %d are enabled.", len(payload.ModelConfigs), enabledCount))
	if excerpt := backendLogDiagnosisExcerpt(lang, text, "model"); excerpt != "" {
		lines = append(lines, excerpt)
	}
	lines = append(lines, "Next step: verify the target model has enabled=true, a non-empty API key, a valid HTTPS custom_api_url, and a correct model name.")
	return strings.Join(lines, "\n")
}

func (a *Agent) handleExchangeDiagnosisSkill(storeUserID, lang, text string) string {
	exchanges := a.loadExchangeOptions(storeUserID)
	lower := strings.ToLower(text)
	lines := make([]string, 0, 8)
	if lang == "zh" {
		lines = append(lines, "现象：这是交易所 API 连接或签名类问题。")
		switch {
		case containsAny(lower, []string{"invalid signature", "签名"}):
			lines = append(lines, "更可能原因：API Secret / passphrase 不匹配，或者系统时间不同步。")
		case containsAny(lower, []string{"timestamp", "时间戳"}):
			lines = append(lines, "更可能原因：服务器时间偏差过大。")
		case containsAny(lower, []string{"ip not allowed", "白名单"}):
			lines = append(lines, "更可能原因：API 白名单没有包含当前服务器 IP。")
		case containsAny(lower, []string{"permission denied", "权限"}):
			lines = append(lines, "更可能原因：交易或合约权限没有打开。")
		default:
			lines = append(lines, "更可能原因：密钥配置、时间同步、白名单或权限设置存在问题。")
		}
		lines = append(lines, "先检查什么：")
		lines = append(lines, "1. 先同步系统时间，尤其是出现 invalid signature / timestamp 时。")
		lines = append(lines, "2. 确认 API Key 和 Secret 没有填反、没有过期。")
		if containsAny(lower, []string{"okx", "欧易"}) || containsAny(strings.ToLower(formatOptionList("", exchanges)), []string{"okx"}) {
			lines = append(lines, "3. 如果是 OKX，再确认 passphrase 没漏填。")
		}
		lines = append(lines, "4. 检查 API 白名单是否包含当前服务器 IP。")
		lines = append(lines, "5. 检查是否已经开启交易/合约权限。")
		if excerpt := backendLogDiagnosisExcerpt(lang, text, "exchange"); excerpt != "" {
			lines = append(lines, excerpt)
		}
		lines = append(lines, "下一步：如果你把具体报错原文贴给我，我可以按报错类型继续缩小范围。")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "Symptom: this looks like an exchange API connectivity or signature issue.")
	lines = append(lines, "Check first: system time sync, API key/secret correctness, IP whitelist, trading permissions, and passphrase for OKX.")
	if len(exchanges) > 0 {
		lines = append(lines, "Current exchange bindings exist, so the next step is to match the exact error text to the most likely cause.")
	}
	if excerpt := backendLogDiagnosisExcerpt(lang, text, "exchange"); excerpt != "" {
		lines = append(lines, excerpt)
	}
	return strings.Join(lines, "\n")
}

func backendLogDiagnosisExcerpt(lang, text, fallbackFilter string) string {
	filter := strings.TrimSpace(text)
	if strings.TrimSpace(filter) == "" {
		filter = fallbackFilter
	}
	_, entries, err := readBackendLogEntries(8, filter, true)
	if err != nil || len(entries) == 0 {
		if filter != fallbackFilter {
			_, entries, err = readBackendLogEntries(8, fallbackFilter, true)
		}
	}
	if err != nil || len(entries) == 0 {
		return ""
	}
	if lang == "zh" {
		return "最近命中的后端错误日志：\n- " + strings.Join(entries, "\n- ")
	}
	return "Recent matching backend error logs:\n- " + strings.Join(entries, "\n- ")
}

type targetResolution struct {
	Ref          *EntityReference
	Ambiguous    []traderSkillOption
	WasMentioned bool
}

func enabledTraderSkillOptions(options []traderSkillOption) []traderSkillOption {
	out := make([]traderSkillOption, 0, len(options))
	for _, o := range options {
		if o.Enabled {
			out = append(out, o)
		}
	}
	return out
}

func resolveSemanticExistingTraderDependency(currentRef *EntityReference, options []traderSkillOption) targetResolution {
	if currentRef != nil && strings.TrimSpace(currentRef.ID) != "" {
		for _, opt := range options {
			if opt.ID == currentRef.ID {
				return targetResolution{Ref: &EntityReference{ID: opt.ID, Name: opt.Name}}
			}
		}
	}
	enabled := enabledTraderSkillOptions(options)
	if len(enabled) == 1 {
		return targetResolution{Ref: &EntityReference{ID: enabled[0].ID, Name: enabled[0].Name}}
	}
	if len(enabled) > 1 {
		return targetResolution{Ambiguous: enabled}
	}
	return targetResolution{}
}

func (a *Agent) hydrateCreateTraderSlotReferences(storeUserID string, session *skillSession) {
	if session == nil {
		return
	}
	if fieldValue(*session, "exchange_id") == "" && fieldValue(*session, "exchange_name") != "" {
		options := a.loadExchangeOptions(storeUserID)
		if opt := findOptionByIDOrName(options, fieldValue(*session, "exchange_name")); opt != nil {
			setField(session, "exchange_id", opt.ID)
		} else if opt := findUniqueContainingOption(options, fieldValue(*session, "exchange_name")); opt != nil {
			setField(session, "exchange_id", opt.ID)
		}
	}
	if fieldValue(*session, "exchange_id") != "" {
		options := a.loadExchangeOptions(storeUserID)
		if opt := findOptionByIDOrName(options, fieldValue(*session, "exchange_id")); opt != nil {
			setField(session, "exchange_id", opt.ID)
			if fieldValue(*session, "exchange_name") == "" {
				setField(session, "exchange_name", opt.Name)
			}
		}
	}
	if fieldValue(*session, "model_id") == "" && fieldValue(*session, "model_name") != "" {
		options := a.loadEnabledModelOptions(storeUserID)
		if opt := findOptionByIDOrName(options, fieldValue(*session, "model_name")); opt != nil {
			setField(session, "model_id", opt.ID)
		} else if opt := findUniqueContainingOption(options, fieldValue(*session, "model_name")); opt != nil {
			setField(session, "model_id", opt.ID)
		}
	}
	if fieldValue(*session, "model_id") != "" {
		options := a.loadEnabledModelOptions(storeUserID)
		if opt := findOptionByIDOrName(options, fieldValue(*session, "model_id")); opt != nil {
			setField(session, "model_id", opt.ID)
			if fieldValue(*session, "model_name") == "" {
				setField(session, "model_name", opt.Name)
			}
		}
	}
	if fieldValue(*session, "strategy_id") == "" && fieldValue(*session, "strategy_name") != "" {
		options := a.loadStrategyOptions(storeUserID)
		if opt := findOptionByIDOrName(options, fieldValue(*session, "strategy_name")); opt != nil {
			setField(session, "strategy_id", opt.ID)
		} else if opt := findUniqueContainingOption(options, fieldValue(*session, "strategy_name")); opt != nil {
			setField(session, "strategy_id", opt.ID)
		}
	}
	if fieldValue(*session, "strategy_id") != "" {
		options := a.loadStrategyOptions(storeUserID)
		if opt := findOptionByIDOrName(options, fieldValue(*session, "strategy_id")); opt != nil {
			setField(session, "strategy_id", opt.ID)
			if fieldValue(*session, "strategy_name") == "" {
				setField(session, "strategy_name", opt.Name)
			}
		}
	}
}

func (a *Agent) maybeResumeParentTaskAfterSuccessfulSkill(storeUserID string, userID int64, lang, skill, action, answer string) string {
	sm := a.SnapshotManager(userID)
	parent, ok := sm.Peek()
	if !ok || !parent.ResumeOnSuccess {
		return answer
	}
	triggered := false
	for _, t := range parent.ResumeTriggers {
		if t == skill {
			triggered = true
			break
		}
	}
	if !triggered {
		return answer
	}
	sm.Load() // pop
	// restore parent history
	if a.history != nil && len(parent.LocalHistory) > 0 {
		a.history.Replace(userID, parent.LocalHistory)
	}
	// inject child result as system message
	if a.history != nil && strings.TrimSpace(answer) != "" {
		inject := fmt.Sprintf("[子任务 %s/%s 已完成，结果：%s]", skill, action, answer)
		a.history.Add(userID, "system", inject)
	}
	// restore parent skill session
	if parent.SkillSession != nil {
		restored := *parent.SkillSession
		a.hydrateCreateTraderSlotReferences(storeUserID, &restored)
		a.saveSkillSession(userID, restored)
		resumeNotice := ""
		if lang == "zh" {
			resumeNotice = "我已经切回刚才的主任务。"
		} else {
			resumeNotice = "I switched back to the earlier main task."
		}
		if restored.Name == "trader_management" && restored.Action == "create" {
			followup := a.buildTraderCreateMissingPrompt(storeUserID, lang, restored, a.buildTraderCreateConversationResources(storeUserID, restored))
			if strings.TrimSpace(followup) != "" {
				if strings.TrimSpace(answer) == "" {
					return resumeNotice + "\n" + followup
				}
				return strings.TrimSpace(answer) + "\n" + resumeNotice + "\n" + followup
			}
		}
		if strings.TrimSpace(answer) == "" {
			return resumeNotice
		}
		return strings.TrimSpace(answer) + "\n" + resumeNotice
	}
	return answer
}

func resolveTargetSelection(text string, options []traderSkillOption, existing *EntityReference) targetResolution {
	if existing != nil && strings.TrimSpace(existing.ID) != "" {
		for _, opt := range options {
			if opt.ID == existing.ID {
				return targetResolution{Ref: &EntityReference{ID: opt.ID, Name: defaultIfEmpty(opt.Name, existing.Name), Source: existing.Source}}
			}
		}
	}
	if existing != nil && strings.TrimSpace(existing.Name) != "" {
		if opt := findOptionByIDOrName(options, existing.Name); opt != nil {
			return targetResolution{Ref: &EntityReference{ID: opt.ID, Name: opt.Name, Source: existing.Source}}
		}
		if opt := findUniqueContainingOption(options, existing.Name); opt != nil {
			return targetResolution{Ref: &EntityReference{ID: opt.ID, Name: opt.Name, Source: existing.Source}}
		}
	}
	if opt := findOptionByIDOrName(options, text); opt != nil {
		return targetResolution{Ref: &EntityReference{ID: opt.ID, Name: opt.Name, Source: "user_mention"}}
	}
	if opt := findUniqueContainingOption(options, text); opt != nil {
		return targetResolution{Ref: &EntityReference{ID: opt.ID, Name: opt.Name, Source: "user_mention"}}
	}
	if len(options) > 1 {
		return targetResolution{Ambiguous: options}
	}
	return targetResolution{}
}

func findOptionByIDOrName(options []traderSkillOption, query string) *traderSkillOption {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	for i, opt := range options {
		if opt.ID == query || strings.EqualFold(opt.Name, query) {
			return &options[i]
		}
	}
	return nil
}

func findUniqueContainingOption(options []traderSkillOption, query string) *traderSkillOption {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	matches := make([]traderSkillOption, 0, 1)
	for _, opt := range options {
		if strings.Contains(strings.ToLower(opt.Name), query) || strings.Contains(query, strings.ToLower(opt.Name)) {
			matches = append(matches, opt)
		}
	}
	if len(matches) != 1 {
		return nil
	}
	return &matches[0]
}

func formatAmbiguousTargetPrompt(lang string, options []traderSkillOption) string {
	if duplicateName, ok := sharedAmbiguousOptionName(options); ok {
		if lang == "zh" {
			return fmt.Sprintf("你提到的是“%s”，但当前有 %d 个同名对象。请告诉我你要操作哪一个。\n%s", duplicateName, len(options), formatDisambiguationOptionList("可选对象：", options))
		}
		return fmt.Sprintf("You mentioned %q, but there are %d objects with the same name. Please tell me which one to operate on.\n%s", duplicateName, len(options), formatDisambiguationOptionList("Available targets:", options))
	}
	if lang == "zh" {
		return "找到多个匹配对象，请告诉我你要操作哪一个。\n" + formatDisambiguationOptionList("可选对象：", options)
	}
	return "Multiple matches found. Please tell me which one to operate on.\n" + formatDisambiguationOptionList("Available targets:", options)
}

func sharedAmbiguousOptionName(options []traderSkillOption) (string, bool) {
	if len(options) < 2 {
		return "", false
	}
	base := strings.TrimSpace(options[0].Name)
	if base == "" {
		return "", false
	}
	for _, option := range options[1:] {
		if !strings.EqualFold(strings.TrimSpace(option.Name), base) {
			return "", false
		}
	}
	return base, true
}

func formatDisambiguationOptionList(prefix string, options []traderSkillOption) string {
	parts := make([]string, 0, len(options))
	for _, option := range options {
		label := strings.TrimSpace(option.Name)
		if label == "" {
			label = option.ID
		}
		if hint := strings.TrimSpace(option.Hint); hint != "" {
			label += "（" + hint + "）"
		}
		if suffix := shortOptionIDSuffix(option.ID); suffix != "" {
			label += fmt.Sprintf("（ID后缀 %s）", suffix)
		}
		if option.Enabled {
			label += "（已启用）"
		} else {
			label += "（已禁用）"
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return ""
	}
	return prefix + strings.Join(parts, "、")
}

func shortOptionIDSuffix(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	runes := []rune(id)
	if len(runes) <= 4 {
		return id
	}
	return string(runes[len(runes)-4:])
}
