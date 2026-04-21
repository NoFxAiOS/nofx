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
		return "provider"
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

func formatStillWaitingConfirmationMessage(lang string) string {
	if lang == "zh" {
		return "当前流程仍在等待你确认。回复“确认”继续，或“取消”终止。"
	}
	return "This flow is still waiting for your confirmation."
}

func beginConfirmationIfNeeded(userID int64, lang string, session *skillSession, targetLabel string) (string, bool) {
	if session == nil || !actionNeedsConfirmation(session.Name, session.Action) {
		return "", false
	}
	if session.Phase != "await_confirmation" {
		session.Phase = "await_confirmation"
		return formatAwaitConfirmationMessage(lang, session.Action, targetLabel), true
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
