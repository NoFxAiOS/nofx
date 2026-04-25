package agent

import "strings"

func (a *Agent) executeAtomicSkillTask(storeUserID string, userID int64, lang, text, skill, action string, onEvent func(event, data string)) (string, bool) {
	return a.executeAtomicSkillTaskWithSession(storeUserID, userID, lang, text, skillSession{Name: strings.TrimSpace(skill), Action: normalizeAtomicSkillAction(strings.TrimSpace(skill), action), Phase: "collecting"}, onEvent)
}

func (a *Agent) executeAtomicSkillTaskWithSession(storeUserID string, userID int64, lang, text string, session skillSession, onEvent func(event, data string)) (string, bool) {
	skill := strings.TrimSpace(session.Name)
	action := normalizeAtomicSkillAction(skill, session.Action)
	session.Name = skill
	session.Action = action
	if strings.TrimSpace(session.Phase) == "" {
		session.Phase = "collecting"
	}
	skill = strings.TrimSpace(skill)
	action = normalizeAtomicSkillAction(skill, action)

	var (
		answer  string
		handled bool
	)

	switch skill {
	case "trader_management":
		if action == "create" {
			answer, handled = a.handleCreateTraderSkill(storeUserID, userID, lang, text, session)
		} else {
			answer, handled = a.handleTraderManagementSkill(storeUserID, userID, lang, text, session)
			if handled && action == "query_running" {
				answer = applyTraderQueryFilter(lang, answer, a.toolListTraders(storeUserID), "running_only")
			}
		}
	case "exchange_management":
		answer, handled = a.handleExchangeManagementSkill(storeUserID, userID, lang, text, session)
	case "model_management":
		answer, handled = a.handleModelManagementSkill(storeUserID, userID, lang, text, session)
	case "strategy_management":
		answer, handled = a.handleStrategyManagementSkill(storeUserID, userID, lang, text, session)
	case "model_diagnosis":
		answer, handled = a.handleModelDiagnosisSkill(storeUserID, lang, text), true
	case "exchange_diagnosis":
		answer, handled = a.handleExchangeDiagnosisSkill(storeUserID, lang, text), true
	case "trader_diagnosis":
		answer, handled = a.handleTraderDiagnosisSkill(storeUserID, lang, text), true
	case "strategy_diagnosis":
		answer, handled = a.handleStrategyDiagnosisSkill(storeUserID, lang, text), true
	default:
		return "", false
	}

	if handled && onEvent != nil {
		label := "atomic_skill:" + skill
		if action != "" {
			label += ":" + action
		}
		onEvent(StreamEventTool, label)
		emitStreamText(onEvent, answer)
	}
	return answer, handled
}

func (a *Agent) executeAtomicSkillTaskOutcome(storeUserID string, userID int64, lang, text, skill, action string, onEvent func(event, data string)) (skillOutcome, bool) {
	return a.executeAtomicSkillTaskOutcomeWithSession(storeUserID, userID, lang, text, skillSession{Name: strings.TrimSpace(skill), Action: normalizeAtomicSkillAction(strings.TrimSpace(skill), action), Phase: "collecting"}, onEvent)
}

func (a *Agent) executeAtomicSkillTaskOutcomeWithSession(storeUserID string, userID int64, lang, text string, session skillSession, onEvent func(event, data string)) (skillOutcome, bool) {
	answer, handled := a.executeAtomicSkillTaskWithSession(storeUserID, userID, lang, text, session, onEvent)
	if !handled {
		return skillOutcome{}, false
	}
	skill := strings.TrimSpace(session.Name)
	action := normalizeAtomicSkillAction(skill, session.Action)
	switch skill {
	case "model_diagnosis", "exchange_diagnosis", "trader_diagnosis", "strategy_diagnosis":
		return skillOutcome{
			Skill:        skill,
			Action:       defaultIfEmpty(action, "diagnose"),
			Status:       skillOutcomeSuccess,
			GoalAchieved: true,
			UserMessage:  answer,
		}, true
	default:
		return inferSkillOutcome(skill, action, answer, a.getSkillSession(userID), skillDataForAction(storeUserID, skill, action, a)), true
	}
}
