package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

type llmSkillRouteDecision struct {
	Route  string `json:"route"`
	Skill  string `json:"skill,omitempty"`
	Action string `json:"action,omitempty"`
	Filter string `json:"filter,omitempty"`
}

func (a *Agent) tryLLMSkillRoute(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool) {
	if a.aiClient == nil {
		return "", false
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}

	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	taskStateCtx := buildTaskStateContext(a.getTaskState(userID))
	executionState := normalizeExecutionState(a.getExecutionState(userID))
	executionJSON, _ := json.Marshal(executionState)
	systemPrompt := `You are the lightweight skill router for NOFXi.
Decide whether the user's message should go to a structured skill or continue to the planner.
Return JSON only. Do not return markdown.

Use route "skill" only when the user intent is clear enough to send directly to one structured skill.
Use route "planner" for ambiguous, multi-step, open-ended, analytical, or diagnostic requests.

Available skills:
- trader_management
- exchange_management
- model_management
- strategy_management
- trader_diagnosis
- exchange_diagnosis
- model_diagnosis
- strategy_diagnosis

For management skills, choose one action from:
- query
- create
- update
- delete
- start
- stop
- activate
- duplicate

Set filter only when it is clearly implied by the user. Use values like:
- running_only
- stopped_only
- enabled_only
- disabled_only
- active_only
- default_only

Rules:
- Prefer route "planner" when uncertain.
- Prefer route "planner" for market analysis, broad advice, multi-step troubleshooting, or requests that need synthesis.
- Prefer route "skill" for straightforward management requests like listing, creating, starting, stopping, enabling, disabling, renaming, or deleting known entities.
- Questions like "当前有运行中的trader吗" and "有没有 trader 在跑" are trader_management with action "query" and filter "running_only".
- Do not use route "skill" for casual chat.
- Consider Recent conversation, Task state, and Execution state JSON before deciding.

Return JSON with this exact shape:
{"route":"skill|planner","skill":"","action":"","filter":""}`
	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nRecent conversation:\n%s\n\nTask state:\n%s\n\nExecution state JSON:\n%s", lang, text, recentConversationCtx, taskStateCtx, string(executionJSON))

	stageCtx, cancel := withPlannerStageTimeout(ctx, directReplyTimeout)
	defer cancel()

	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	if err != nil {
		return "", false
	}

	decision, err := parseLLMSkillRouteDecision(raw)
	if err != nil || decision.Route != "skill" {
		return "", false
	}

	answer, ok := a.executeLLMSkillRoute(storeUserID, userID, lang, text, decision)
	if !ok {
		return "", false
	}

	a.recordSkillInteraction(userID, text, answer)
	if onEvent != nil {
		label := "llm_skill_route"
		if decision.Skill != "" {
			label += ":" + decision.Skill
		}
		if decision.Action != "" {
			label += ":" + decision.Action
		}
		onEvent(StreamEventTool, label)
		onEvent(StreamEventDelta, answer)
	}
	return answer, true
}

func parseLLMSkillRouteDecision(raw string) (llmSkillRouteDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision llmSkillRouteDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeLLMSkillRouteDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeLLMSkillRouteDecision(decision), nil
		}
	}
	return llmSkillRouteDecision{}, fmt.Errorf("invalid llm skill route json")
}

func normalizeLLMSkillRouteDecision(decision llmSkillRouteDecision) llmSkillRouteDecision {
	decision.Route = strings.TrimSpace(strings.ToLower(decision.Route))
	decision.Skill = strings.TrimSpace(strings.ToLower(decision.Skill))
	decision.Action = strings.TrimSpace(strings.ToLower(decision.Action))
	decision.Filter = strings.TrimSpace(strings.ToLower(decision.Filter))
	return decision
}

func (a *Agent) executeLLMSkillRoute(storeUserID string, userID int64, lang, text string, decision llmSkillRouteDecision) (string, bool) {
	session := skillSession{Name: decision.Skill, Action: decision.Action}

	switch decision.Skill {
	case "trader_management":
		if decision.Action == "create" {
			return a.handleCreateTraderSkill(storeUserID, userID, lang, text, session)
		}
		answer, handled := a.handleTraderManagementSkill(storeUserID, userID, lang, text, session)
		if handled && decision.Action == "query" {
			return applyTraderQueryFilter(lang, answer, a.toolListTraders(storeUserID), decision.Filter), true
		}
		return answer, handled
	case "exchange_management":
		return a.handleExchangeManagementSkill(storeUserID, userID, lang, text, session)
	case "model_management":
		return a.handleModelManagementSkill(storeUserID, userID, lang, text, session)
	case "strategy_management":
		return a.handleStrategyManagementSkill(storeUserID, userID, lang, text, session)
	case "model_diagnosis":
		return a.handleModelDiagnosisSkill(storeUserID, lang, text), true
	case "exchange_diagnosis":
		return a.handleExchangeDiagnosisSkill(storeUserID, lang, text), true
	case "trader_diagnosis":
		return a.handleTraderDiagnosisSkill(storeUserID, lang, text), true
	case "strategy_diagnosis":
		return a.handleStrategyDiagnosisSkill(storeUserID, lang, text), true
	default:
		return "", false
	}
}

func applyTraderQueryFilter(lang, fallback, raw, filter string) string {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return fallback
	}

	var payload struct {
		Traders []struct {
			Name      string `json:"name"`
			IsRunning bool   `json:"is_running"`
		} `json:"traders"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return fallback
	}

	switch filter {
	case "running_only":
		names := make([]string, 0, len(payload.Traders))
		for _, trader := range payload.Traders {
			if trader.IsRunning {
				names = append(names, strings.TrimSpace(trader.Name))
			}
		}
		if lang == "zh" {
			if len(names) == 0 {
				return "当前没有运行中的交易员。"
			}
			return fmt.Sprintf("当前有 %d 个运行中的交易员：%s。", len(names), strings.Join(names, "、"))
		}
		if len(names) == 0 {
			return "There are no running traders right now."
		}
		return fmt.Sprintf("There are %d running traders right now: %s.", len(names), strings.Join(names, ", "))
	case "stopped_only":
		names := make([]string, 0, len(payload.Traders))
		for _, trader := range payload.Traders {
			if !trader.IsRunning {
				names = append(names, strings.TrimSpace(trader.Name))
			}
		}
		if lang == "zh" {
			if len(names) == 0 {
				return "当前没有已停止的交易员。"
			}
			return fmt.Sprintf("当前有 %d 个未运行的交易员：%s。", len(names), strings.Join(names, "、"))
		}
		if len(names) == 0 {
			return "There are no stopped traders right now."
		}
		return fmt.Sprintf("There are %d stopped traders right now: %s.", len(names), strings.Join(names, ", "))
	default:
		return fallback
	}
}
