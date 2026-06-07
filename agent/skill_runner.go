package agent

import (
	"fmt"
	"strings"
)

type skillActionRuntime struct {
	Skill  SkillDefinition
	Name   string
	Action SkillActionDefinition
}

func getSkillActionRuntime(skillName, action string) (skillActionRuntime, bool) {
	def, ok := getSkillDefinition(skillName)
	if !ok {
		return skillActionRuntime{}, false
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return skillActionRuntime{Skill: def}, true
	}
	actionDef, ok := def.Actions[action]
	if !ok {
		return skillActionRuntime{}, false
	}
	return skillActionRuntime{
		Skill:  def,
		Name:   action,
		Action: actionDef,
	}, true
}

func actionNeedsConfirmation(skillName, action string) bool {
	runtime, ok := getSkillActionRuntime(skillName, action)
	if !ok {
		return false
	}
	return runtime.Action.NeedsConfirmation
}

func actionRequiresSlot(skillName, action, slot string) bool {
	runtime, ok := getSkillActionRuntime(skillName, action)
	if !ok {
		return false
	}
	slot = strings.TrimSpace(slot)
	for _, candidate := range runtime.Action.RequiredSlots {
		if candidate == slot {
			return true
		}
	}
	return false
}

func slotDisplayName(slot, lang string) string {
	slot = strings.TrimSpace(slot)
	if lang != "zh" {
		switch slot {
		case "target_ref":
			return "target"
		case "name":
			return "name"
		case "exchange":
			return "exchange"
		case "model":
			return "model"
		case "strategy":
			return "strategy"
		case "exchange_type":
			return "exchange type"
		case "provider":
			return "provider"
		default:
			return slot
		}
	}
	switch slot {
	case "target_ref":
		return "目标对象"
	case "name":
		return "名称"
	case "exchange":
		return "交易所"
	case "model":
		return "模型"
	case "strategy":
		return "策略"
	case "exchange_type":
		return "交易所类型"
	case "provider":
		return "模型提供商"
	default:
		return slot
	}
}

func formatAwaitConfirmationMessage(lang, action, targetLabel string) string {
	actionLabel := action
	if lang == "zh" {
		switch action {
		case "start":
			actionLabel = "启动"
		case "stop":
			actionLabel = "停止"
		case "delete":
			actionLabel = "删除"
		case "activate":
			actionLabel = "激活"
		default:
			actionLabel = action
		}
		return fmt.Sprintf("即将%s“%s”。这是需要确认的操作，请回复“确认”继续，回复“取消”终止。", actionLabel, targetLabel)
	}
	return fmt.Sprintf("You are about to %s %q. Please reply 'confirm' to continue or 'cancel' to stop.", actionLabel, targetLabel)
}

func formatTargetConfirmationLabel(lang string, session *skillSession, targetLabel string) string {
	targetLabel = strings.TrimSpace(targetLabel)
	if session == nil || session.TargetRef == nil || targetLabel == "" {
		return targetLabel
	}
	source := strings.TrimSpace(session.TargetRef.Source)
	if source == "" {
		return targetLabel
	}
	if lang == "zh" {
		sourceLabel := "系统上下文"
		switch source {
		case "user_mention":
			sourceLabel = "你刚才点名的对象"
		case "tool_output":
			sourceLabel = "刚刚工具返回的对象"
		case "inferred_from_context":
			sourceLabel = "上下文推断对象"
		}
		return fmt.Sprintf("%s（当前识别来源：%s）", targetLabel, sourceLabel)
	}
	sourceLabel := "context"
	switch source {
	case "user_mention":
		sourceLabel = "your explicit mention"
	case "tool_output":
		sourceLabel = "recent tool output"
	case "inferred_from_context":
		sourceLabel = "context inference"
	}
	return fmt.Sprintf("%s (current reference source: %s)", targetLabel, sourceLabel)
}

func formatStillWaitingConfirmationMessage(lang string) string {
	if lang == "zh" {
		return "当前流程仍在等待你确认。回复“确认”继续，或“取消”终止。"
	}
	return "This flow is still waiting for your confirmation."
}

func referenceKindForSkill(skillName string) string {
	switch strings.TrimSpace(skillName) {
	case "strategy_management":
		return "strategy"
	case "trader_management":
		return "trader"
	case "model_management":
		return "model"
	case "exchange_management":
		return "exchange"
	default:
		return ""
	}
}

func referenceKindDisplayName(lang, kind string) string {
	if lang == "zh" {
		switch kind {
		case "strategy":
			return "策略"
		case "trader":
			return "交易员"
		case "model":
			return "模型"
		case "exchange":
			return "交易所"
		}
		return "对象"
	}
	return kind
}

func (a *Agent) formatConfirmationTargetLabel(userID int64, lang string, session *skillSession, targetLabel string) string {
	label := formatTargetConfirmationLabel(lang, session, targetLabel)
	if session == nil || session.TargetRef == nil {
		return label
	}
	kind := referenceKindForSkill(session.Name)
	if kind == "" {
		return label
	}
	state := a.getExecutionState(userID)
	recentNames := map[string]struct{}{}
	for _, item := range state.ReferenceHistory {
		if item.Kind != kind {
			continue
		}
		name := strings.TrimSpace(defaultIfEmpty(item.Name, item.ID))
		if name == "" {
			continue
		}
		recentNames[name] = struct{}{}
	}
	targetName := strings.TrimSpace(defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID))
	_, inferred := recentNames[targetName]
	if targetName == "" {
		return label
	}
	if len(recentNames) <= 1 && strings.TrimSpace(session.TargetRef.Source) != "inferred_from_context" && inferred {
		return label
	}
	if lang == "zh" {
		return fmt.Sprintf("%s。系统当前理解你要操作的%s是“%s”。", label, referenceKindDisplayName(lang, kind), targetName)
	}
	return fmt.Sprintf("%s. The current %s I'm about to operate on is %q.", label, referenceKindDisplayName(lang, kind), targetName)
}

func (a *Agent) beginConfirmationIfNeeded(userID int64, lang string, session *skillSession, targetLabel string) (string, bool) {
	if session == nil || !actionNeedsConfirmation(session.Name, session.Action) {
		return "", false
	}
	if session.Phase != "await_confirmation" {
		session.Phase = "await_confirmation"
		return formatAwaitConfirmationMessage(lang, session.Action, a.formatConfirmationTargetLabel(userID, lang, session, targetLabel)), true
	}
	return "", false
}

func awaitingConfirmationButNotApproved(lang string, session skillSession, text string) (string, bool) {
	if !actionNeedsConfirmation(session.Name, session.Action) || session.Phase != "await_confirmation" {
		return "", false
	}
	if isYesReply(text) {
		return "", false
	}
	return formatStillWaitingConfirmationMessage(lang), true
}
