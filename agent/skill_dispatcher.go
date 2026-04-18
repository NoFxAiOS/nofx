package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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
}

var (
	quotedNamePattern    = regexp.MustCompile(`[“"]([^“”"]{1,40})[”"]`)
	traderNamedPattern   = regexp.MustCompile(`(?:叫|名为|名字是)\s*([A-Za-z0-9_\-\p{Han}]{2,40})`)
)

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
			key = strings.TrimSpace(key)
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
		session.Slots.Name = strings.TrimSpace(session.Slots.Name)
		session.Slots.ExchangeID = strings.TrimSpace(session.Slots.ExchangeID)
		session.Slots.ExchangeName = strings.TrimSpace(session.Slots.ExchangeName)
		session.Slots.ModelID = strings.TrimSpace(session.Slots.ModelID)
		session.Slots.ModelName = strings.TrimSpace(session.Slots.ModelName)
		session.Slots.StrategyID = strings.TrimSpace(session.Slots.StrategyID)
		session.Slots.StrategyName = strings.TrimSpace(session.Slots.StrategyName)
		if session.Slots.Name == "" &&
			session.Slots.ExchangeID == "" &&
			session.Slots.ModelID == "" &&
			session.Slots.StrategyID == "" &&
			session.Slots.AutoStart == nil {
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
	return lower == "取消" || lower == "/cancel" || lower == "cancel"
}

func detectCreateTraderSkill(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	hasCreate := containsAny(lower, []string{"创建", "新建", "建一个", "create", "new"})
	hasTrader := containsAny(lower, []string{"交易员", "trader", "agent"})
	return hasCreate && hasTrader
}

func detectModelDiagnosisSkill(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if containsAny(lower, []string{"custom_api_url", "invalid custom_api_url", "ai assistant unavailable", "模型配置失败", "模型不可用", "ai unavailable"}) {
		return true
	}
	return containsAny(lower, []string{"模型", "model", "api key", "base url", "custom_api_url"}) &&
		containsAny(lower, []string{"报错", "错误", "失败", "不可用", "不生效", "invalid", "error", "failed"})
}

func detectExchangeDiagnosisSkill(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"invalid signature", "timestamp", "ip not allowed", "permission denied",
		"签名错误", "签名失败", "时间戳", "白名单", "权限不足", "交易所 api 报错", "交易所连接不上",
	})
}

func detectStartIntent(text string) bool {
	lower := strings.ToLower(text)
	return containsAny(lower, []string{"启动", "跑起来", "run", "start", "立即运行", "并启动"})
}

func extractTraderName(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if matches := quotedNamePattern.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	if matches := traderNamedPattern.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func pickMentionedOption(text string, options []traderSkillOption) *traderSkillOption {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return nil
	}
	var matched *traderSkillOption
	for _, option := range options {
		id := strings.ToLower(strings.TrimSpace(option.ID))
		name := strings.ToLower(strings.TrimSpace(option.Name))
		if id == "" && name == "" {
			continue
		}
		if (id != "" && strings.Contains(lower, id)) || (name != "" && strings.Contains(lower, name)) {
			if matched != nil {
				return nil
			}
			copy := option
			matched = &copy
		}
	}
	return matched
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
			name = strings.TrimSpace(model.CustomModelName)
		}
		if name == "" {
			name = strings.TrimSpace(model.Provider)
		}
		out = append(out, traderSkillOption{ID: model.ID, Name: name, Enabled: model.Enabled})
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

func (a *Agent) tryHardSkill(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool) {
	if ctx != nil && ctx.Err() != nil {
		return "", false
	}
	session := a.getSkillSession(userID)
	if (session.Name == "trader_management" && session.Action == "create") || detectCreateTraderSkill(text) {
		answer, handled := a.handleCreateTraderSkill(storeUserID, userID, lang, text, session)
		if handled {
			a.recordSkillInteraction(userID, text, answer)
			if onEvent != nil {
				onEvent(StreamEventTool, "hard_skill:trader_management:create")
				onEvent(StreamEventDelta, answer)
			}
		}
		return answer, handled
	}
	if (session.Name == "trader_management" && session.Action != "create") || detectTraderManagementIntent(text) {
		answer, handled := a.handleTraderManagementSkill(storeUserID, userID, lang, text, session)
		if handled {
			a.recordSkillInteraction(userID, text, answer)
			if onEvent != nil {
				onEvent(StreamEventTool, "hard_skill:trader_management")
				onEvent(StreamEventDelta, answer)
			}
		}
		return answer, handled
	}
	if session.Name == "exchange_management" || detectExchangeManagementIntent(text) {
		answer, handled := a.handleExchangeManagementSkill(storeUserID, userID, lang, text, session)
		if handled {
			a.recordSkillInteraction(userID, text, answer)
			if onEvent != nil {
				onEvent(StreamEventTool, "hard_skill:exchange_management")
				onEvent(StreamEventDelta, answer)
			}
		}
		return answer, handled
	}
	if session.Name == "model_management" || detectModelManagementIntent(text) {
		answer, handled := a.handleModelManagementSkill(storeUserID, userID, lang, text, session)
		if handled {
			a.recordSkillInteraction(userID, text, answer)
			if onEvent != nil {
				onEvent(StreamEventTool, "hard_skill:model_management")
				onEvent(StreamEventDelta, answer)
			}
		}
		return answer, handled
	}
	if session.Name == "strategy_management" || detectStrategyManagementIntent(text) {
		answer, handled := a.handleStrategyManagementSkill(storeUserID, userID, lang, text, session)
		if handled {
			a.recordSkillInteraction(userID, text, answer)
			if onEvent != nil {
				onEvent(StreamEventTool, "hard_skill:strategy_management")
				onEvent(StreamEventDelta, answer)
			}
		}
		return answer, handled
	}
	if detectModelDiagnosisSkill(text) {
		answer := a.handleModelDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:model_diagnosis")
			onEvent(StreamEventDelta, answer)
		}
		return answer, true
	}
	if detectExchangeDiagnosisSkill(text) {
		answer := a.handleExchangeDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:exchange_diagnosis")
			onEvent(StreamEventDelta, answer)
		}
		return answer, true
	}
	if detectTraderDiagnosisSkill(text) {
		answer := a.handleTraderDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:trader_diagnosis")
			onEvent(StreamEventDelta, answer)
		}
		return answer, true
	}
	if detectStrategyDiagnosisSkill(text) {
		answer := a.handleStrategyDiagnosisSkill(storeUserID, lang, text)
		a.recordSkillInteraction(userID, text, answer)
		if onEvent != nil {
			onEvent(StreamEventTool, "hard_skill:strategy_diagnosis")
			onEvent(StreamEventDelta, answer)
		}
		return answer, true
	}
	return "", false
}

func (a *Agent) recordSkillInteraction(userID int64, userText, answer string) {
	if a.history == nil {
		a.history = newChatHistory(100)
	}
	a.history.Add(userID, "user", userText)
	a.history.Add(userID, "assistant", answer)
}

func ensureSkillFields(session *skillSession) {
	if session.Fields == nil {
		session.Fields = make(map[string]string)
	}
}

func (a *Agent) handleCreateTraderSkill(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	if isCancelSkillReply(text) {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "已取消当前创建交易员流程。", true
		}
		return "Cancelled the current trader creation flow.", true
	}

		if session.Name == "" {
			session = skillSession{
				Name:   "trader_management",
				Action: "create",
				Phase:  "collecting",
				Slots:  &createTraderSkillSlots{},
			}
		if detectStartIntent(text) {
			autoStart := true
			session.Slots.AutoStart = &autoStart
		}
	}
	if session.Slots == nil {
		session.Slots = &createTraderSkillSlots{}
	}

	if session.Phase == "await_start_confirmation" {
		switch {
		case isYesReply(text):
			answer := a.executeCreateTraderSkill(storeUserID, userID, lang, session, true)
			return answer, true
		case isNoReply(text):
			answer := a.executeCreateTraderSkill(storeUserID, userID, lang, session, false)
			return answer, true
		default:
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "当前流程在等待你确认是否立即启动交易员。回复“确认”继续启动，回复“先不用”则只创建不启动。", true
			}
			return "This flow is waiting for your confirmation to start the trader. Reply 'confirm' to start it now, or 'no' to create without starting.", true
		}
	}

	slots := session.Slots
	if slots.Name == "" {
		slots.Name = extractTraderName(text)
	}

	models := a.loadEnabledModelOptions(storeUserID)
	exchanges := a.loadExchangeOptions(storeUserID)
	strategies := a.loadStrategyOptions(storeUserID)

	if slots.ModelID == "" {
		if match := pickMentionedOption(text, models); match != nil {
			slots.ModelID = match.ID
			slots.ModelName = match.Name
		} else if choice := choosePreferredOption(models); choice != nil {
			slots.ModelID = choice.ID
			slots.ModelName = choice.Name
		}
	}
	if slots.ExchangeID == "" {
		if match := pickMentionedOption(text, exchanges); match != nil {
			slots.ExchangeID = match.ID
			slots.ExchangeName = match.Name
		} else if choice := choosePreferredOption(exchanges); choice != nil {
			slots.ExchangeID = choice.ID
			slots.ExchangeName = choice.Name
		}
	}
	if slots.StrategyID == "" {
		if match := pickMentionedOption(text, strategies); match != nil {
			slots.StrategyID = match.ID
			slots.StrategyName = match.Name
		} else if choice := choosePreferredOption(strategies); choice != nil {
			slots.StrategyID = choice.ID
			slots.StrategyName = choice.Name
		}
	}

	if slots.AutoStart == nil && detectStartIntent(text) {
		autoStart := true
		slots.AutoStart = &autoStart
	}

	if len(strategies) == 0 {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "当前还没有可用策略，暂时不能创建交易员。请先创建一个策略，再回来继续。", true
		}
		return "There is no strategy available yet, so I can't create a trader. Please create a strategy first.", true
	}
	if len(models) == 0 {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "当前还没有模型配置，暂时不能创建交易员。请先配置并启用一个模型。", true
		}
		return "There is no model config yet, so I can't create a trader. Please configure and enable a model first.", true
	}
	if len(exchanges) == 0 {
		a.clearSkillSession(userID)
		if lang == "zh" {
			return "当前还没有交易所配置，暂时不能创建交易员。请先配置并启用一个交易所账户。", true
		}
		return "There is no exchange config yet, so I can't create a trader. Please configure and enable an exchange first.", true
	}

	missing := make([]string, 0, 3)
	extraLines := make([]string, 0, 3)
	if actionRequiresSlot("trader_management", "create", "name") && slots.Name == "" {
		missing = append(missing, slotDisplayName("name", lang))
	}
	if actionRequiresSlot("trader_management", "create", "exchange") && slots.ExchangeID == "" {
		missing = append(missing, slotDisplayName("exchange", lang))
		extraLines = append(extraLines, formatOptionList("可用交易所：", exchanges))
	}
	if actionRequiresSlot("trader_management", "create", "model") && slots.ModelID == "" {
		missing = append(missing, slotDisplayName("model", lang))
		extraLines = append(extraLines, formatOptionList("可用模型：", models))
	}
	if actionRequiresSlot("trader_management", "create", "strategy") && slots.StrategyID == "" {
		missing = append(missing, slotDisplayName("strategy", lang))
		extraLines = append(extraLines, formatOptionList("可用策略：", strategies))
	}

	if len(missing) > 0 {
		session.Phase = "collecting"
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			reply := "要继续创建交易员，还缺这些信息：" + strings.Join(missing, "、") + "。"
			if len(extraLines) > 0 {
				reply += "\n" + strings.Join(cleanStringList(extraLines), "\n")
			}
			reply += "\n你可以直接一次性告诉我，例如：名称、用哪个交易所、哪个模型、哪个策略。"
			return reply, true
		}
		reply := "To continue creating the trader, I still need: " + strings.Join(missing, ", ") + "."
		if len(extraLines) > 0 {
			reply += "\n" + strings.Join(cleanStringList(extraLines), "\n")
		}
		reply += "\nYou can reply with all missing fields in one message."
		return reply, true
	}

	if slots.AutoStart != nil && *slots.AutoStart {
		session.Phase = "await_start_confirmation"
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return fmt.Sprintf("我已经准备好创建交易员“%s”，并在创建后立即启动它。\n使用的交易所：%s\n使用的模型：%s\n使用的策略：%s\n\n这是高风险动作。回复“确认”继续，回复“先不用”则只创建不启动。",
				slots.Name, slots.ExchangeNameOrID(), slots.ModelNameOrID(), slots.StrategyNameOrID()), true
		}
		return fmt.Sprintf("I'm ready to create trader %q and start it immediately.\nExchange: %s\nModel: %s\nStrategy: %s\n\nThis is a high-risk action. Reply 'confirm' to continue, or 'no' to create it without starting.",
			slots.Name, slots.ExchangeNameOrID(), slots.ModelNameOrID(), slots.StrategyNameOrID()), true
	}

	answer := a.executeCreateTraderSkill(storeUserID, userID, lang, session, false)
	return answer, true
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

func (a *Agent) executeCreateTraderSkill(storeUserID string, userID int64, lang string, session skillSession, startAfterCreate bool) string {
	args := manageTraderArgs{
		Action:      "create",
		Name:        session.Slots.Name,
		AIModelID:   session.Slots.ModelID,
		ExchangeID:  session.Slots.ExchangeID,
		StrategyID:  session.Slots.StrategyID,
	}
	createRaw := a.toolCreateTrader(storeUserID, args)
	if errMsg := parseSkillError(createRaw); errMsg != "" && strings.Contains(createRaw, `"error"`) {
		session.Phase = "collecting"
		a.saveSkillSession(userID, session)
		if lang == "zh" {
			return "创建交易员失败：" + errMsg
		}
		return "Failed to create trader: " + errMsg
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
		a.clearSkillSession(userID)
		if lang == "zh" {
			return fmt.Sprintf("已创建交易员“%s”。\n交易所：%s\n模型：%s\n策略：%s\n当前状态：未启动。",
				created.Trader.Name, session.Slots.ExchangeNameOrID(), session.Slots.ModelNameOrID(), session.Slots.StrategyNameOrID())
		}
		return fmt.Sprintf("Created trader %q.\nExchange: %s\nModel: %s\nStrategy: %s\nCurrent status: not started.",
			created.Trader.Name, session.Slots.ExchangeNameOrID(), session.Slots.ModelNameOrID(), session.Slots.StrategyNameOrID())
	}

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
			created.Trader.Name, session.Slots.ExchangeNameOrID(), session.Slots.ModelNameOrID(), session.Slots.StrategyNameOrID())
	}
	return fmt.Sprintf("Created and started trader %q.\nExchange: %s\nModel: %s\nStrategy: %s",
		created.Trader.Name, session.Slots.ExchangeNameOrID(), session.Slots.ModelNameOrID(), session.Slots.StrategyNameOrID())
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
		lines = append(lines, "下一步：如果你把具体报错原文贴给我，我可以按报错类型继续缩小范围。")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "Symptom: this looks like an exchange API connectivity or signature issue.")
	lines = append(lines, "Check first: system time sync, API key/secret correctness, IP whitelist, trading permissions, and passphrase for OKX.")
	if len(exchanges) > 0 {
		lines = append(lines, "Current exchange bindings exist, so the next step is to match the exact error text to the most likely cause.")
	}
	return strings.Join(lines, "\n")
}
