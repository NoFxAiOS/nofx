package agent

import (
	"strings"
)

func inferStandaloneStrategyName(text string) string {
	value := strings.TrimSpace(text)
	if value == "" || len([]rune(value)) > 50 {
		return ""
	}
	if strategyCreateConfirmationReply(value) || strategyCreateDefaultConfigReply(value) || isCancelSkillReply(value) {
		return ""
	}
	if parseStrategyTypeValue(value) != "" {
		return ""
	}
	if containsAny(strings.ToLower(value), []string{"创建", "新建", "create", "grid_trading", "ai_trading"}) {
		return ""
	}
	return value
}

func activeHistoryMessageAsksStrategyName(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAny(lower, []string{"策略名", "名称", "名字", "叫什么", "name"})
}
