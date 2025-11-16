package decision

import (
	"strings"
	"testing"
)

// TestBuildSystemPrompt_ContainsAllValidActions 测试 prompt 是否包含所有有效的 action
func TestBuildSystemPrompt_ContainsAllValidActions(t *testing.T) {
	// 这是系统中定义的所有有效 action（来自 validateDecision）
	validActions := []string{
		"open_long",
		"open_short",
		"close_long",
		"close_short",
		"update_stop_loss",
		"update_take_profit",
		"partial_close",
		"hold",
		"wait",
	}

	// 构建 prompt
	prompt := buildSystemPrompt(1000.0, 10, 5, "default")

	// 验证每个有效 action 都在 prompt 中出现
	for _, action := range validActions {
		if !strings.Contains(prompt, action) {
			t.Errorf("Prompt 缺少有效的 action: %s", action)
		}
	}
}

// TestBuildSystemPrompt_ActionListCompleteness 测试 action 列表的完整性
func TestBuildSystemPrompt_ActionListCompleteness(t *testing.T) {
	prompt := buildSystemPrompt(1000.0, 10, 5, "default")

	// 检查是否包含关键的缺失 action
	missingActions := []string{
		"update_stop_loss",
		"update_take_profit",
		"partial_close",
	}

	for _, action := range missingActions {
		if !strings.Contains(prompt, action) {
			t.Errorf("Prompt 缺少关键 action: %s（这会导致 AI 返回无效决策）", action)
		}
	}
}
