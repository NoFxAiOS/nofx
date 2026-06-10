package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"nofx/mcp"
)

const (
	// agenticMaxToolRounds bounds the number of LLM round-trips in one user
	// turn. Each round may execute several tool calls, so this comfortably
	// covers chained operations (create → configure → start) while still
	// terminating runaway loops.
	agenticMaxToolRounds = 12

	// agenticHistoryMessages is the number of recent history messages replayed
	// to the LLM as real conversation turns.
	agenticHistoryMessages = 12
)

// agentV2Enabled reports whether the native function-calling loop is the
// primary brain. Enabled by default; set NOFX_AGENT_V2=0/false/off/disabled
// to fall back to the legacy routing stack.
func agentV2Enabled() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("NOFX_AGENT_V2"))) {
	case "0", "false", "off", "disabled":
		return false
	}
	return true
}

// shouldUseAgenticTurn reports whether this turn should go through the native
// function-calling loop. In-flight legacy flows (skill sessions, workflows,
// execution states, pending proposals) stay on the legacy stack so they finish
// with the state machine that started them.
func (a *Agent) shouldUseAgenticTurn(userID int64) bool {
	if a.aiClient == nil || !agentV2Enabled() {
		return false
	}
	if a.hasAnyActiveContext(userID) {
		return false
	}
	if _, ok := a.getPendingProposalSession(userID); ok {
		return false
	}
	// Suspended tasks are parked on the snapshot stack with all active
	// contexts cleared; only the legacy router knows how to resume them.
	if len(a.SnapshotManager(userID).Stack()) > 0 {
		return false
	}
	return true
}

// runAgenticTurn drives one user turn through a native function-calling loop:
// the LLM sees the full toolset plus recent conversation, decides which tools
// to call, receives every tool result (including errors) as observations, and
// writes the final user-facing reply itself.
//
// Returns handled=false when nothing user-visible happened and the caller
// should fall back to the legacy routing stack (e.g. the very first LLM call
// failed). Once any tool has executed, the turn is always handled so side
// effects are never silently repeated by a fallback path.
func (a *Agent) runAgenticTurn(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	if a.aiClient == nil {
		return "", false, nil
	}

	messages := []mcp.Message{mcp.NewSystemMessage(a.buildSystemPromptForStoreUser(lang, storeUserID))}
	if prefs := a.buildPersistentPreferencesContext(userID); prefs != "" {
		messages = append(messages, mcp.NewSystemMessage(prefs))
	}
	if taskCtx := buildTaskStateContext(a.getTaskState(userID)); taskCtx != "" {
		messages = append(messages, mcp.NewSystemMessage(taskCtx))
	}
	messages = append(messages, a.recentHistoryMessages(userID, text)...)
	messages = append(messages, mcp.NewUserMessage(text))

	tools := agentTools()
	var executedTools []string

	for round := 0; round < agenticMaxToolRounds; round++ {
		resp, err := a.aiClient.CallWithRequestFull(&mcp.Request{
			Messages:   messages,
			Tools:      tools,
			ToolChoice: "auto",
			Ctx:        ctx,
		})
		if err != nil {
			a.logger.Warn("agentic turn LLM call failed", "error", err, "user_id", userID, "round", round)
			if len(executedTools) == 0 {
				// Nothing happened yet — safe to let the legacy stack retry.
				return "", false, nil
			}
			reply := agenticInterruptedReply(lang, executedTools)
			return a.finishAgenticTurn(userID, lang, text, reply, onEvent), true, nil
		}

		if len(resp.ToolCalls) == 0 {
			reply := strings.TrimSpace(resp.Content)
			if reply == "" {
				if len(executedTools) == 0 {
					return "", false, nil
				}
				reply = agenticInterruptedReply(lang, executedTools)
			}
			return a.finishAgenticTurn(userID, lang, text, reply, onEvent), true, nil
		}

		assistantMsg := mcp.Message{Role: "assistant", ToolCalls: resp.ToolCalls}
		if resp.Content != "" {
			assistantMsg.Content = resp.Content
		}
		if resp.ReasoningContent != "" {
			assistantMsg.ReasoningContent = resp.ReasoningContent
		}
		messages = append(messages, assistantMsg)

		for _, tc := range resp.ToolCalls {
			if onEvent != nil {
				onEvent(StreamEventTool, tc.Function.Name)
			}
			executedTools = append(executedTools, tc.Function.Name)
			result := a.handleToolCall(ctx, storeUserID, userID, lang, tc)
			messages = append(messages, mcp.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// Round budget exhausted: ask the LLM to wrap up with what it has, without
	// offering further tools.
	messages = append(messages, mcp.NewSystemMessage(agenticWrapUpInstruction(lang)))
	final, err := a.aiClient.CallWithRequest(&mcp.Request{Messages: messages, Ctx: ctx})
	if err != nil || strings.TrimSpace(final) == "" {
		if err != nil {
			a.logger.Warn("agentic wrap-up call failed", "error", err, "user_id", userID)
		}
		final = agenticInterruptedReply(lang, executedTools)
	}
	return a.finishAgenticTurn(userID, lang, text, final, onEvent), true, nil
}

// finishAgenticTurn applies final-reply guards, records the turn in history,
// and streams the reply.
func (a *Agent) finishAgenticTurn(userID int64, lang, text, reply string, onEvent func(event, data string)) string {
	if guarded, blocked := guardUnsupportedAsyncPromise(lang, reply); blocked {
		reply = guarded
	}
	if a.history != nil {
		a.history.Add(userID, "user", text)
		a.history.Add(userID, "assistant", reply)
	}
	emitStreamText(onEvent, reply)
	return reply
}

// recentHistoryMessages replays recent conversation turns as real chat
// messages so the LLM has multi-turn context, dropping a trailing duplicate of
// the current user text if the caller already recorded it.
func (a *Agent) recentHistoryMessages(userID int64, currentText string) []mcp.Message {
	if a.history == nil {
		return nil
	}
	msgs := a.history.Get(userID)
	if n := len(msgs); n > 0 && msgs[n-1].Role == "user" &&
		strings.TrimSpace(msgs[n-1].Content) == strings.TrimSpace(currentText) {
		msgs = msgs[:n-1]
	}
	if len(msgs) > agenticHistoryMessages {
		msgs = msgs[len(msgs)-agenticHistoryMessages:]
	}
	out := make([]mcp.Message, 0, len(msgs))
	for _, m := range msgs {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		switch m.Role {
		case "user":
			out = append(out, mcp.NewUserMessage(content))
		case "assistant":
			out = append(out, mcp.Message{Role: "assistant", Content: content})
		}
	}
	return out
}

// agenticInterruptedReply tells the user exactly which tools already ran when
// a turn cannot produce an LLM-written reply, so work is never silently lost.
func agenticInterruptedReply(lang string, executedTools []string) string {
	tools := strings.Join(executedTools, ", ")
	if lang == "zh" {
		if tools == "" {
			return "刚才处理你的请求时 AI 服务中断了，已执行的操作没有丢失。请再说一次你想做什么，我接着处理。"
		}
		return fmt.Sprintf("处理过程中 AI 服务中断了。已执行的操作：%s。这些结果已生效，你可以让我继续下一步或查询当前状态。", tools)
	}
	if tools == "" {
		return "The AI service was interrupted while handling your request. Nothing was lost — please tell me again what you'd like to do."
	}
	return fmt.Sprintf("The AI service was interrupted mid-task. Tools already executed: %s. Those results took effect — ask me to continue or check the current state.", tools)
}

// agenticWrapUpInstruction is appended when the tool-round budget is spent.
func agenticWrapUpInstruction(lang string) string {
	if lang == "zh" {
		return "工具调用轮次已达上限。请基于以上已获得的全部结果，直接给用户一个完整的中文总结回复：说明已完成什么、未完成什么、建议的下一步。不要再请求调用工具。"
	}
	return "Tool-call round limit reached. Using everything gathered above, write the final reply for the user now: what was completed, what was not, and the suggested next step. Do not request more tool calls."
}
