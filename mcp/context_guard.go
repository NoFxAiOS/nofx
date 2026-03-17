package mcp

import (
	"fmt"
	"unicode/utf8"
)

// estimateMessageTokens estimates the token count for a list of chat messages.
// Uses ~3 chars per token heuristic (conservative for mixed CJK/English text).
// Each message has ~10 tokens overhead for role/formatting.
func estimateMessageTokens(messages []map[string]string) int {
	total := 0
	for _, msg := range messages {
		content := msg["content"]
		charCount := utf8.RuneCountInString(content)
		total += charCount/3 + 10 // ~3 chars per token + overhead
	}
	return total
}

// estimateMessageTokensAny is like estimateMessageTokens but for map[string]any messages
// (used by BuildRequestBodyFromRequest which needs tool_calls support).
func estimateMessageTokensAny(messages []map[string]any) int {
	total := 0
	for _, msg := range messages {
		content := fmt.Sprintf("%v", msg["content"])
		charCount := utf8.RuneCountInString(content)
		total += charCount/3 + 10
	}
	return total
}

// truncateMessages removes oldest non-system messages until estimated tokens
// fit within the context limit. Returns the truncated messages and the number
// of messages removed.
//
// Rules:
//   - Never removes system messages (role="system")
//   - Removes from the oldest non-system message first
//   - Keeps the most recent messages
//   - Returns original messages unchanged if no truncation needed
func truncateMessages(messages []map[string]string, maxContext, maxTokens int) ([]map[string]string, int) {
	if maxContext <= 0 {
		return messages, 0
	}

	budget := maxContext - maxTokens
	if budget <= 0 {
		budget = maxContext / 2 // safety: at least half for input
	}

	estimated := estimateMessageTokens(messages)
	if estimated <= budget {
		return messages, 0
	}

	// Separate system messages (keep all) from non-system (truncatable)
	var systemMsgs []map[string]string
	var otherMsgs []map[string]string
	for _, msg := range messages {
		if msg["role"] == "system" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			otherMsgs = append(otherMsgs, msg)
		}
	}

	// Calculate system message tokens (non-removable)
	systemTokens := estimateMessageTokens(systemMsgs)
	remainingBudget := budget - systemTokens
	if remainingBudget <= 0 {
		return messages, 0
	}

	// Remove oldest non-system messages until we fit
	removed := 0
	for len(otherMsgs) > 1 {
		currentTokens := estimateMessageTokens(otherMsgs)
		if currentTokens <= remainingBudget {
			break
		}
		otherMsgs = otherMsgs[1:]
		removed++
	}

	if removed == 0 {
		return messages, 0
	}

	result := make([]map[string]string, 0, len(systemMsgs)+len(otherMsgs))
	result = append(result, systemMsgs...)
	result = append(result, otherMsgs...)
	return result, removed
}

// truncateMessagesAny is like truncateMessages but for map[string]any messages.
func truncateMessagesAny(messages []map[string]any, maxContext, maxTokens int) ([]map[string]any, int) {
	if maxContext <= 0 {
		return messages, 0
	}

	budget := maxContext - maxTokens
	if budget <= 0 {
		budget = maxContext / 2
	}

	estimated := estimateMessageTokensAny(messages)
	if estimated <= budget {
		return messages, 0
	}

	var systemMsgs []map[string]any
	var otherMsgs []map[string]any
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		if role == "system" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			otherMsgs = append(otherMsgs, msg)
		}
	}

	systemTokens := estimateMessageTokensAny(systemMsgs)
	remainingBudget := budget - systemTokens
	if remainingBudget <= 0 {
		return messages, 0
	}

	removed := 0
	for len(otherMsgs) > 1 {
		currentTokens := estimateMessageTokensAny(otherMsgs)
		if currentTokens <= remainingBudget {
			break
		}
		otherMsgs = otherMsgs[1:]
		removed++
	}

	if removed == 0 {
		return messages, 0
	}

	result := make([]map[string]any, 0, len(systemMsgs)+len(otherMsgs))
	result = append(result, systemMsgs...)
	result = append(result, otherMsgs...)
	return result, removed
}
