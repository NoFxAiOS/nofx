package mcp

import (
	"strings"
	"testing"
)

func TestEstimateMessageTokens(t *testing.T) {
	msgs := []map[string]string{
		{"role": "system", "content": "You are a helpful assistant."},
		{"role": "user", "content": "Hello, how are you?"},
	}
	tokens := estimateMessageTokens(msgs)
	if tokens <= 0 {
		t.Errorf("expected positive token count, got %d", tokens)
	}
	// "You are a helpful assistant." = 28 chars / 3 + 10 = ~19
	// "Hello, how are you?" = 19 chars / 3 + 10 = ~16
	// Total ~35
	if tokens < 20 || tokens > 60 {
		t.Errorf("expected ~35 tokens, got %d", tokens)
	}
}

func TestTruncateMessages_NoTruncationNeeded(t *testing.T) {
	msgs := []map[string]string{
		{"role": "system", "content": "Be helpful."},
		{"role": "user", "content": "Hi"},
	}
	result, removed := truncateMessages(msgs, 131072, 2000)
	if removed != 0 {
		t.Errorf("expected no truncation, got %d removed", removed)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestTruncateMessages_NoLimit(t *testing.T) {
	msgs := []map[string]string{
		{"role": "user", "content": strings.Repeat("x", 1000000)},
	}
	result, removed := truncateMessages(msgs, 0, 2000)
	if removed != 0 {
		t.Errorf("expected no truncation when maxContext=0, got %d removed", removed)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 message, got %d", len(result))
	}
}

func TestTruncateMessages_TruncatesOldest(t *testing.T) {
	// Create messages that definitely exceed a small context limit
	msgs := []map[string]string{
		{"role": "system", "content": "System prompt"},
		{"role": "user", "content": strings.Repeat("old message ", 500)},     // ~2000 chars
		{"role": "assistant", "content": strings.Repeat("old reply ", 500)},   // ~2000 chars
		{"role": "user", "content": strings.Repeat("newer msg ", 500)},        // ~2000 chars
		{"role": "assistant", "content": strings.Repeat("newer reply ", 500)}, // ~2000 chars
		{"role": "user", "content": "latest question"},
	}

	// Set a small context limit that forces truncation
	result, removed := truncateMessages(msgs, 2000, 500)
	if removed == 0 {
		t.Fatal("expected some messages to be truncated")
	}

	// System message should always be preserved
	if result[0]["role"] != "system" {
		t.Error("system message should be first")
	}

	// Last message should be the latest user message
	last := result[len(result)-1]
	if last["content"] != "latest question" {
		t.Errorf("last message should be 'latest question', got '%s'", last["content"])
	}

	// Should have fewer messages than original
	if len(result) >= len(msgs) {
		t.Errorf("expected fewer messages after truncation, got %d (original %d)", len(result), len(msgs))
	}
}

func TestTruncateMessages_PreservesSystemMessages(t *testing.T) {
	msgs := []map[string]string{
		{"role": "system", "content": "System 1"},
		{"role": "system", "content": "System 2"},
		{"role": "user", "content": strings.Repeat("long msg ", 1000)},
		{"role": "user", "content": "short"},
	}

	result, _ := truncateMessages(msgs, 500, 100)

	// Count system messages - should all be preserved
	systemCount := 0
	for _, msg := range result {
		if msg["role"] == "system" {
			systemCount++
		}
	}
	if systemCount != 2 {
		t.Errorf("expected 2 system messages preserved, got %d", systemCount)
	}
}

func TestTruncateMessages_KeepsAtLeastOneNonSystem(t *testing.T) {
	msgs := []map[string]string{
		{"role": "system", "content": "System"},
		{"role": "user", "content": strings.Repeat("very long ", 10000)},
	}

	result, _ := truncateMessages(msgs, 100, 50)

	nonSystem := 0
	for _, msg := range result {
		if msg["role"] != "system" {
			nonSystem++
		}
	}
	if nonSystem < 1 {
		t.Error("should keep at least 1 non-system message")
	}
}
