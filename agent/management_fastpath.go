package agent

import (
	"context"
	"regexp"
	"strings"
)

// DetectManagementIntent is exported for diagnostic tooling — calls
// detectManagementIntent.
func DetectManagementIntent(text string) *ManagementIntent {
	mi := detectManagementIntent(text)
	if mi == nil {
		return nil
	}
	return &ManagementIntent{Skill: mi.Skill, Action: mi.Action, ExtractedData: mi.ExtractedData}
}

// ManagementIntent mirrors managementIntent for external callers.
type ManagementIntent struct {
	Skill         string
	Action        string
	ExtractedData map[string]any
}

// managementIntent represents a deterministic match against an explicit
// management/diagnosis command (verb + entity), e.g. "创建一个新交易员".
type managementIntent struct {
	Skill         string
	Action        string
	ExtractedData map[string]any
}

var (
	mfpVerbCreate    = regexp.MustCompile(`(?i)^\s*(创建|新建|添加|新增|建一个|建个|create|add|new)\s*`)
	mfpVerbList      = regexp.MustCompile(`(?i)^\s*(查看|列出|显示|看一下|看下|查询|查一下|看看|查|有哪些|list|show|view)\s*`)
	mfpVerbDelete    = regexp.MustCompile(`(?i)^\s*(删除|删掉|去掉|移除|delete|remove)\s*`)
	mfpVerbStop      = regexp.MustCompile(`(?i)^\s*(停掉|停止|关闭|关掉|stop|disable)\s*`)
	mfpVerbStart     = regexp.MustCompile(`(?i)^\s*(启动|开启|开启|start|enable|activate)\s*`)
	mfpVerbSwitch    = regexp.MustCompile(`(?i)^\s*(切换|换一个|换|换成|改成|switch\s+to|switch|change|use|用)\s*`)
	mfpVerbConfigure = regexp.MustCompile(`(?i)^\s*(配置|设置|设定|configure)\s*`)
	mfpDiagnoseCue   = regexp.MustCompile(`(?i)(怎么没|没下单|没成交|没运行|没启动|连不上|连接失败|失败|不工作|为啥|为何|为什么|why\s+(?:isn'?t|is not|cannot|can't))`)
	mfpBulkAll       = regexp.MustCompile(`(?i)(所有|全部|全|all|every)`)
)

type entityMatch struct {
	canonical string // skill domain
	provider  string // optional extracted provider/sub-type
}

// detectEntity returns the canonical entity (trader/exchange/model/strategy)
// referenced in the message, plus an optional provider hint extracted from
// the same text.
func detectEntity(text string) entityMatch {
	lower := strings.ToLower(text)

	// Strategy first (more specific than "trader" since 策略 is unambiguous).
	if strings.Contains(text, "策略") || strings.Contains(lower, "strategy") || strings.Contains(lower, "grid") || strings.Contains(text, "网格") {
		em := entityMatch{canonical: "strategy"}
		if strings.Contains(lower, "grid") || strings.Contains(text, "网格") {
			em.provider = "grid_trading"
		} else if strings.Contains(lower, "ai") || strings.Contains(text, "ai 策略") {
			em.provider = "ai_trading"
		}
		return em
	}

	// Exchange before trader (binance/okx etc. are exchange-specific keywords).
	if strings.Contains(text, "交易所") || strings.Contains(lower, "exchange") {
		return entityMatch{canonical: "exchange", provider: extractExchangeProvider(lower)}
	}
	if p := extractExchangeProvider(lower); p != "" {
		// "binance 怎么连不上" — entity inferred from provider keyword.
		return entityMatch{canonical: "exchange", provider: p}
	}

	// Model — must check before trader because 模型 ≠ 交易员.
	if strings.Contains(text, "模型") || strings.Contains(lower, "model") || strings.Contains(lower, "deepseek") || strings.Contains(lower, "gpt") || strings.Contains(lower, "claude") || strings.Contains(lower, "qwen") || strings.Contains(lower, "kimi") || strings.Contains(lower, "grok") || strings.Contains(lower, "gemini") || strings.Contains(lower, "minimax") || strings.Contains(lower, "claw402") || strings.Contains(lower, "glm") {
		em := entityMatch{canonical: "model"}
		em.provider = extractModelProvider(lower)
		return em
	}

	if strings.Contains(text, "交易员") || strings.Contains(lower, "trader") {
		return entityMatch{canonical: "trader"}
	}

	return entityMatch{}
}

func extractExchangeProvider(lower string) string {
	for _, p := range []string{"binance", "bybit", "okx", "bitget", "kucoin", "gate", "hyperliquid", "aster", "lighter", "indodax", "alpaca"} {
		if strings.Contains(lower, p) {
			return p
		}
	}
	return ""
}

func extractModelProvider(lower string) string {
	for _, candidate := range []string{
		"deepseek-v4-pro", "deepseek-v4-flash", "deepseek-reasoner", "deepseek",
		"gpt-5.4", "gpt-5.3", "gpt-5-mini", "gpt-5", "gpt",
		"claude-opus", "claude",
		"gemini-3.1-pro", "gemini",
		"qwen-flash", "qwen-max", "qwen-plus", "qwen-turbo", "qwen",
		"kimi-k2.5", "kimi",
		"grok-4.1", "grok",
		"glm-5-turbo", "glm-5", "glm",
		"minimax",
		"claw402",
	} {
		if strings.Contains(lower, candidate) {
			return candidate
		}
	}
	return ""
}

// detectManagementIntent inspects an unambiguous user message and returns the
// (skill, action) it should map to. Returns nil when the text is too vague
// or cleary not a management command — in that case the LLM router runs.
//
// Goal: bypass the LLM router for explicit "verb + entity" commands so they
// reliably trigger the right skill instead of being answered conversationally
// by central_brain ("好的我来帮你创建一个新交易员，先告诉我名字").
func detectManagementIntent(text string) *managementIntent {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	// Diagnosis takes priority when the user is reporting a problem.
	if mfpDiagnoseCue.MatchString(trimmed) {
		entity := detectEntity(trimmed)
		if entity.canonical == "" {
			return nil
		}
		skill := entity.canonical + "_diagnosis"
		intent := &managementIntent{Skill: skill, Action: "query_detail", ExtractedData: map[string]any{}}
		if entity.provider != "" {
			intent.ExtractedData["provider"] = entity.provider
		}
		return intent
	}

	// Verb classification — first verb that matches wins.
	verbs := []struct {
		re     *regexp.Regexp
		action string
	}{
		{mfpVerbList, "query_list"},
		{mfpVerbCreate, "create"},
		{mfpVerbDelete, "delete"},
		{mfpVerbStop, "stop"},
		{mfpVerbStart, "start"},
		{mfpVerbConfigure, "create"}, // "配置一个 X" treated as create
		{mfpVerbSwitch, "update"},
	}
	var matchedAction string
	for _, v := range verbs {
		if v.re.MatchString(trimmed) {
			matchedAction = v.action
			break
		}
	}
	if matchedAction == "" {
		return nil
	}

	entity := detectEntity(trimmed)
	if entity.canonical == "" {
		return nil
	}

	// Map (entity, verb) → (skill, final action). Action vocabulary is
	// constrained by what each skill actually supports — see agent/skills/*.json.
	skill := entity.canonical + "_management"
	action := matchedAction

	switch entity.canonical {
	case "model":
		// model_management has update_endpoint for switching models, plus
		// update for general changes. "用/换" maps to update; create for
		// configuring new credentials.
		if matchedAction == "update" {
			action = "update"
		}
	case "strategy":
		if matchedAction == "stop" {
			// strategy doesn't have stop — closest is delete or update_status
			action = "update_config"
		}
		if matchedAction == "start" {
			action = "activate"
		}
	case "exchange":
		// exchange_management has create/update/update_name/update_status/delete/query_list
		if matchedAction == "stop" || matchedAction == "start" {
			action = "update_status"
		}
	case "trader":
		// trader_management has start/stop/create/delete/update etc directly.
	}

	intent := &managementIntent{Skill: skill, Action: action, ExtractedData: map[string]any{}}
	if entity.provider != "" {
		switch entity.canonical {
		case "exchange":
			intent.ExtractedData["provider"] = entity.provider
		case "model":
			intent.ExtractedData["custom_model_name"] = entity.provider
		case "strategy":
			intent.ExtractedData["strategy_type"] = entity.provider
		}
	}
	if mfpBulkAll.MatchString(trimmed) {
		intent.ExtractedData["bulk_scope"] = "all"
	}
	return intent
}

// handleManagementIntent runs a deterministically detected management/
// diagnosis command directly through the skill driver, skipping the LLM
// router. Returns ("", false) when no match — caller falls through to the
// LLM router.
func (a *Agent) handleManagementIntent(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	intent := detectManagementIntent(text)
	if intent == nil {
		return "", false, nil
	}
	// Don't fast-path when the user is already mid-flow — let the router
	// decide whether the new message belongs to the active session.
	if a.hasAnyActiveContext(userID) {
		return "", false, nil
	}
	session := newActiveSkillSession(userID, intent.Skill, intent.Action)
	session.Goal = strings.TrimSpace(text)
	if intent.ExtractedData != nil {
		intent.ExtractedData = filterExtractedDataForActiveSession(session, intent.ExtractedData, lang)
		mergeExtractedData(&session, intent.ExtractedData)
	}
	answer, handled, err := a.driveActiveSession(ctx, storeUserID, userID, lang, text, session, onEvent)
	if err != nil || handled {
		return answer, true, err
	}
	return "", false, nil
}
