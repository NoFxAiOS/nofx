package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nofx/mcp"
)

const (
	skillOutcomeSuccess          = "success"
	skillOutcomeNeedMoreInfo     = "need_more_info"
	skillOutcomeRecoverableError = "recoverable_error"
	skillOutcomeFatalError       = "fatal_error"
	skillOutcomeNotHandled       = "not_handled"
)

type skillOutcome struct {
	Skill        string         `json:"skill"`
	Action       string         `json:"action"`
	Status       string         `json:"status"`
	GoalAchieved bool           `json:"goal_achieved"`
	UserMessage  string         `json:"user_message,omitempty"`
	ErrorCode    string         `json:"error_code,omitempty"`
	Error        string         `json:"error,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
}

type taskReviewDecision struct {
	Route  string `json:"route"`
	Answer string `json:"answer,omitempty"`
}

func normalizeAtomicSkillAction(skill, action string) string {
	action = strings.TrimSpace(strings.ToLower(action))
	switch skill {
	case "trader_management":
		switch action {
		case "query", "query_list":
			return "query_list"
		case "query_running":
			return "query_running"
		case "query_detail":
			return "query_detail"
		case "update":
			return "update_name"
		case "update_name", "update_bindings":
			return action
		}
	case "exchange_management":
		switch action {
		case "query", "query_list":
			return "query_list"
		case "query_detail":
			return "query_detail"
		case "update":
			return "update_name"
		case "update_name", "update_status":
			return action
		}
	case "model_management":
		switch action {
		case "query", "query_list":
			return "query_list"
		case "query_detail":
			return "query_detail"
		case "update":
			return "update_name"
		case "update_name", "update_endpoint", "update_status":
			return action
		}
	case "strategy_management":
		switch action {
		case "query", "query_list":
			return "query_list"
		case "query_detail":
			return "query_detail"
		case "update":
			return "update_name"
		case "update_name", "update_config", "update_prompt":
			return action
		}
	}
	return action
}

func inferSkillOutcome(skill, action, answer string, activeSession skillSession, data map[string]any) skillOutcome {
	outcome := skillOutcome{
		Skill:       skill,
		Action:      action,
		Status:      skillOutcomeSuccess,
		UserMessage: strings.TrimSpace(answer),
		Data:        data,
	}
	if activeSession.Name != "" {
		outcome.Status = skillOutcomeNeedMoreInfo
		outcome.GoalAchieved = false
		return outcome
	}

	lower := strings.ToLower(strings.TrimSpace(answer))
	switch {
	case lower == "":
		outcome.Status = skillOutcomeNotHandled
	case strings.Contains(lower, "失败") || strings.Contains(lower, "failed") || strings.Contains(lower, "error"):
		outcome.Status = skillOutcomeRecoverableError
		outcome.Error = strings.TrimSpace(answer)
	default:
		outcome.GoalAchieved = true
	}
	return outcome
}

func parseTaskReviewDecision(raw string) (taskReviewDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision taskReviewDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		decision.Route = strings.TrimSpace(strings.ToLower(decision.Route))
		decision.Answer = strings.TrimSpace(decision.Answer)
		return decision, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			decision.Route = strings.TrimSpace(strings.ToLower(decision.Route))
			decision.Answer = strings.TrimSpace(decision.Answer)
			return decision, nil
		}
	}
	return taskReviewDecision{}, fmt.Errorf("invalid task review json")
}

func (a *Agent) reviewTaskCompletion(ctx context.Context, userID int64, lang, text string, outcome skillOutcome) (taskReviewDecision, error) {
	if a.aiClient == nil {
		if outcome.Status == skillOutcomeRecoverableError || outcome.Status == skillOutcomeFatalError || outcome.Status == skillOutcomeNotHandled {
			return taskReviewDecision{Route: "replan"}, nil
		}
		return taskReviewDecision{Route: "complete", Answer: outcome.UserMessage}, nil
	}

	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	outcomeJSON, _ := json.Marshal(outcome)
	systemPrompt := `You are the task-level Plan-Execute-Review supervisor for NOFXi.
You are reviewing the JSON result returned by one structured skill execution.
Return JSON only. Do not return markdown.

Rules:
- Decide whether the OVERALL user task is finished, not whether the skill itself ran successfully.
- Use route "complete" only when the user's task is now complete or the best next message is a final user-facing reply.
- Use route "replan" when the user's task is not complete yet and the planner should continue from the new skill outcome.
- Prefer route "replan" for recoverable errors, unmet goals, missing prerequisites, or cases where another skill/tool sequence may help.
- If you choose "complete", produce the final user-facing answer in the user's language.

Return JSON with this exact shape:
{"route":"complete|replan","answer":""}`
	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nRecent conversation:\n%s\n\nSkill outcome JSON:\n%s", lang, text, recentConversationCtx, string(outcomeJSON))

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
		return taskReviewDecision{}, err
	}
	return parseTaskReviewDecision(raw)
}
