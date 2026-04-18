package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (a *Agent) executeTraderManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	switch session.Action {
	case "query":
		return formatReadFastPathResponse(lang, "list_traders", a.toolListTraders(storeUserID))
	case "start", "stop", "delete":
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		var resp string
		switch session.Action {
		case "start":
			resp = a.toolStartTrader(storeUserID, session.TargetRef.ID)
		case "stop":
			resp = a.toolStopTrader(storeUserID, session.TargetRef.ID)
		case "delete":
			resp = a.toolDeleteTrader(storeUserID, session.TargetRef.ID)
		}
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "执行失败：" + errMsg
			}
			return "Action failed: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已完成交易员操作：%s。", session.Action)
		}
		return fmt.Sprintf("Completed trader action: %s.", session.Action)
	case "update":
		newName := extractTraderName(text)
		if newName == "" {
			newName = extractPostKeywordName(text, []string{"改成", "改为", "rename to"})
		}
		if newName == "" {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新交易员这条 skill 先支持改名。请直接告诉我新的名字。"
			}
			return "This trader update skill currently supports renaming first. Tell me the new name."
		}
		args := manageTraderArgs{Action: "update", TraderID: session.TargetRef.ID, Name: newName}
		resp := a.toolUpdateTrader(storeUserID, args)
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "更新交易员失败：" + errMsg
			}
			return "Failed to update trader: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已将交易员改名为“%s”。", newName)
		}
		return fmt.Sprintf("Renamed trader to %q.", newName)
	default:
		return ""
	}
}

func (a *Agent) executeExchangeManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	switch session.Action {
	case "delete":
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		args, _ := json.Marshal(map[string]any{"action": "delete", "exchange_id": session.TargetRef.ID})
		resp := a.toolManageExchangeConfig(storeUserID, string(args))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "删除交易所配置失败：" + errMsg
			}
			return "Failed to delete exchange config: " + errMsg
		}
		if lang == "zh" {
			return "已删除交易所配置。"
		}
		return "Deleted exchange config."
	case "update":
		accountName := extractTraderName(text)
		if accountName == "" {
			accountName = extractPostKeywordName(text, []string{"改成", "改为", "账户名改成", "rename to"})
		}
		payload := map[string]any{"action": "update", "exchange_id": session.TargetRef.ID}
		if accountName != "" {
			payload["account_name"] = accountName
		}
		if containsAny(strings.ToLower(text), []string{"启用", "enable"}) {
			payload["enabled"] = true
		}
		if containsAny(strings.ToLower(text), []string{"禁用", "disable"}) {
			payload["enabled"] = false
		}
		if len(payload) == 2 {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新交易所 skill 先支持改账户名和启用/禁用。请告诉我你要改什么。"
			}
			return "This exchange update skill currently supports renaming and enable/disable. Tell me what to change."
		}
		raw, _ := json.Marshal(payload)
		resp := a.toolManageExchangeConfig(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "更新交易所配置失败：" + errMsg
			}
			return "Failed to update exchange config: " + errMsg
		}
		if lang == "zh" {
			return "已更新交易所配置。"
		}
		return "Updated exchange config."
	default:
		return ""
	}
}

func (a *Agent) executeModelManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	switch session.Action {
	case "delete":
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		raw, _ := json.Marshal(map[string]any{"action": "delete", "model_id": session.TargetRef.ID})
		resp := a.toolManageModelConfig(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "删除模型配置失败：" + errMsg
			}
			return "Failed to delete model config: " + errMsg
		}
		if lang == "zh" {
			return "已删除模型配置。"
		}
		return "Deleted model config."
	case "update":
		payload := map[string]any{"action": "update", "model_id": session.TargetRef.ID}
		if url := extractURL(text); url != "" {
			payload["custom_api_url"] = url
		}
		if containsAny(strings.ToLower(text), []string{"启用", "enable"}) {
			payload["enabled"] = true
		}
		if containsAny(strings.ToLower(text), []string{"禁用", "disable"}) {
			payload["enabled"] = false
		}
		if modelName := extractPostKeywordName(text, []string{"model name", "模型名", "模型名称", "改成"}); modelName != "" {
			payload["custom_model_name"] = modelName
		}
		if len(payload) == 2 {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新模型 skill 先支持改 URL、模型名和启用状态。请告诉我你要改什么。"
			}
			return "This model update skill currently supports URL, model name, and enabled state."
		}
		raw, _ := json.Marshal(payload)
		resp := a.toolManageModelConfig(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "更新模型配置失败：" + errMsg
			}
			return "Failed to update model config: " + errMsg
		}
		if lang == "zh" {
			return "已更新模型配置。"
		}
		return "Updated model config."
	default:
		return ""
	}
}

func (a *Agent) executeStrategyManagementAction(storeUserID string, userID int64, lang, text string, session skillSession) string {
	switch session.Action {
	case "activate":
		raw, _ := json.Marshal(map[string]any{"action": "activate", "strategy_id": session.TargetRef.ID})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "激活策略失败：" + errMsg
			}
			return "Failed to activate strategy: " + errMsg
		}
		if lang == "zh" {
			return "已激活策略。"
		}
		return "Activated strategy."
	case "duplicate":
		newName := extractTraderName(text)
		if newName == "" {
			newName = extractPostKeywordName(text, []string{"叫", "名为", "改成", "rename to"})
		}
		if newName == "" {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "复制策略时，我还需要一个新名称。"
			}
			return "I still need a new name for the duplicated strategy."
		}
		raw, _ := json.Marshal(map[string]any{"action": "duplicate", "strategy_id": session.TargetRef.ID, "name": newName})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "复制策略失败：" + errMsg
			}
			return "Failed to duplicate strategy: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已复制策略，新名称为“%s”。", newName)
		}
		return fmt.Sprintf("Duplicated strategy as %q.", newName)
	case "delete":
		if msg, waiting := beginConfirmationIfNeeded(userID, lang, &session, defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		if msg, waiting := awaitingConfirmationButNotApproved(lang, session, text); waiting {
			a.saveSkillSession(userID, session)
			return msg
		}
		raw, _ := json.Marshal(map[string]any{"action": "delete", "strategy_id": session.TargetRef.ID})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "删除策略失败：" + errMsg
			}
			return "Failed to delete strategy: " + errMsg
		}
		if lang == "zh" {
			return "已删除策略。"
		}
		return "Deleted strategy."
	case "update":
		newName := extractTraderName(text)
		if newName == "" {
			newName = extractPostKeywordName(text, []string{"改成", "改为", "rename to"})
		}
		if newName == "" {
			a.saveSkillSession(userID, session)
			if lang == "zh" {
				return "目前更新策略 skill 先支持改名。请告诉我新的策略名称。"
			}
			return "This strategy update skill currently supports renaming first."
		}
		raw, _ := json.Marshal(map[string]any{"action": "update", "strategy_id": session.TargetRef.ID, "name": newName})
		resp := a.toolManageStrategy(storeUserID, string(raw))
		a.clearSkillSession(userID)
		if errMsg := parseSkillError(resp); strings.Contains(resp, `"error"`) {
			if lang == "zh" {
				return "更新策略失败：" + errMsg
			}
			return "Failed to update strategy: " + errMsg
		}
		if lang == "zh" {
			return fmt.Sprintf("已将策略改名为“%s”。", newName)
		}
		return fmt.Sprintf("Renamed strategy to %q.", newName)
	default:
		return ""
	}
}

func (a *Agent) handleTraderDiagnosisSkill(storeUserID, lang, text string) string {
	raw := a.toolListTraders(storeUserID)
	list := formatReadFastPathResponse(lang, "list_traders", raw)
	if lang == "zh" {
		return "现象：这是交易员运行诊断问题。\n优先排查：\n1. 交易员是否已创建并处于运行状态。\n2. 绑定的模型、交易所、策略是否齐全。\n3. 是“没有启动”、还是“启动了但 AI 没有下单”、还是“下单失败”。\n当前交易员概览：\n" + list
	}
	return "This looks like a trader diagnosis issue.\nCheck whether the trader exists, is running, and has model/exchange/strategy bindings.\nCurrent trader overview:\n" + list
}

func (a *Agent) handleStrategyDiagnosisSkill(storeUserID, lang, text string) string {
	raw := a.toolGetStrategies(storeUserID)
	list := formatReadFastPathResponse(lang, "get_strategies", raw)
	if lang == "zh" {
		return "现象：这是策略或提示词生效问题。\n优先排查：\n1. 你改的是策略模板，还是 trader 上的 custom prompt。\n2. 策略是否真的保存成功。\n3. 运行结果不符合预期，是配置问题还是市场条件问题。\n当前策略概览：\n" + list
	}
	return "This looks like a strategy or prompt diagnosis issue.\nCheck whether you changed the strategy template or a trader-specific prompt override.\nCurrent strategy overview:\n" + list
}
