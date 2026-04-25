package agent

import (
	"fmt"
	"strings"
)

func (a *Agent) buildCurrentTurnContext(userID int64, lang, currentUserText string) string {
	var parts []string
	previousAssistantReply := strings.TrimSpace(a.currentPendingHintText(userID))
	if previousAssistantReply != "" {
		parts = append(parts, "Previous assistant reply:\n"+previousAssistantReply)
	}
	recentConversation := strings.TrimSpace(a.buildRecentConversationContext(userID, currentUserText))
	if recentConversation != "" {
		parts = append(parts, "Recent conversation:\n"+recentConversation)
	}
	currentRefs := strings.TrimSpace(buildCurrentReferenceSummary(lang, a.semanticCurrentReferences(userID)))
	if currentRefs != "" {
		parts = append(parts, "Current references:\n"+currentRefs)
	}
	return strings.Join(parts, "\n\n")
}

func (a *Agent) buildActiveTaskStateContext(userID int64, lang string) string {
	activeSkill := a.getSkillSession(userID)
	activeTask, hasActiveTask := a.getActiveSkillSession(userID)
	activeWorkflow := a.getWorkflowSession(userID)
	activeExec := normalizeExecutionState(a.getExecutionState(userID))
	pendingProposal, hasPendingProposal := a.getPendingProposalSession(userID)

	lines := []string{}
	if hasActiveTask || strings.TrimSpace(activeSkill.Name) != "" || hasActiveWorkflowSession(activeWorkflow) || hasActiveExecutionState(activeExec) || hasPendingProposal {
		summary := strings.TrimSpace(buildTopLevelActiveFlowSummary(lang, activeSkill, activeTask, hasActiveTask, activeWorkflow, activeExec, pendingProposal, hasPendingProposal))
		if summary != "" {
			lines = append(lines, summary)
		}
	}

	taskState := normalizeTaskState(a.getTaskState(userID))
	if taskState.CurrentGoal != "" {
		lines = append(lines, "Durable goal: "+taskState.CurrentGoal)
	}
	if taskState.ActiveFlow != "" {
		lines = append(lines, "Durable active flow: "+taskState.ActiveFlow)
	}
	if len(taskState.OpenLoops) > 0 {
		limit := len(taskState.OpenLoops)
		if limit > 3 {
			limit = 3
		}
		for _, loop := range taskState.OpenLoops[:limit] {
			lines = append(lines, "Open loop: "+loop)
		}
	}

	if hasActiveExecutionState(activeExec) {
		lines = append(lines, fmt.Sprintf("Execution status: %s", activeExec.Status))
		if strings.TrimSpace(activeExec.Goal) != "" {
			lines = append(lines, "Execution goal: "+strings.TrimSpace(activeExec.Goal))
		}
		if activeExec.Waiting != nil && strings.TrimSpace(activeExec.Waiting.Question) != "" {
			lines = append(lines, "Waiting question: "+strings.TrimSpace(activeExec.Waiting.Question))
		}
		if strings.TrimSpace(activeExec.CurrentStepID) != "" {
			lines = append(lines, "Current step id: "+strings.TrimSpace(activeExec.CurrentStepID))
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}
