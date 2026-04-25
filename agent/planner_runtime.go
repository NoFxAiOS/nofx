package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"nofx/mcp"
)

const (
	plannerMaxSteps      = 8
	plannerMaxIterations = 12
	observationMaxLength = 1000
)

var (
	plannerCreateTimeout = 36 * time.Second
	plannerReplanTimeout = 24 * time.Second
	plannerReasonTimeout = 30 * time.Second
	plannerFinalTimeout  = 36 * time.Second
	directReplyTimeout   = 8 * time.Second
)

type replannerDecision struct {
	Action      string     `json:"action"`
	Goal        string     `json:"goal,omitempty"`
	Steps       []PlanStep `json:"steps,omitempty"`
	Instruction string     `json:"instruction,omitempty"`
	Question    string     `json:"question,omitempty"`
}

type readFastPathRequest struct {
	Kind     string
	ArgsJSON string
}

type directReplyDecision struct {
	Action string `json:"action"`
	Answer string `json:"answer,omitempty"`
}

func latestAskedQuestion(state ExecutionState) string {
	if state.Waiting != nil && strings.TrimSpace(state.Waiting.Question) != "" {
		return strings.TrimSpace(state.Waiting.Question)
	}
	for i := len(state.Steps) - 1; i >= 0; i-- {
		step := state.Steps[i]
		if step.Type == planStepTypeAskUser {
			if q := strings.TrimSpace(step.Instruction); q != "" {
				return q
			}
			if q := strings.TrimSpace(step.OutputSummary); q != "" {
				return q
			}
		}
	}
	if state.Status == executionStatusWaitingUser {
		return strings.TrimSpace(state.FinalAnswer)
	}
	return ""
}

func buildWaitingState(state ExecutionState, step PlanStep, question string) *WaitingState {
	waiting := &WaitingState{
		Question:           strings.TrimSpace(question),
		Intent:             inferWaitingIntent(state.Goal, step, question),
		PendingFields:      inferPendingFields(step, question),
		ConfirmationTarget: inferConfirmationTarget(state.Goal, step, question),
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	return normalizeWaitingState(waiting)
}

func inferWaitingIntent(goal string, step PlanStep, question string) string {
	lowerGoal := strings.ToLower(strings.TrimSpace(goal))
	lowerQuestion := strings.ToLower(strings.TrimSpace(question))
	switch {
	case step.RequiresConfirmation || strings.Contains(lowerQuestion, "需要我") || strings.Contains(lowerQuestion, "confirm") || strings.Contains(lowerQuestion, "确认"):
		return "confirm_action"
	case strings.Contains(lowerGoal, "交易员") || strings.Contains(lowerGoal, "trader"):
		return "complete_trader_setup"
	case strings.Contains(lowerGoal, "交易所") || strings.Contains(lowerGoal, "exchange"):
		return "complete_exchange_config"
	case strings.Contains(lowerGoal, "模型") || strings.Contains(lowerGoal, "model"):
		return "complete_model_config"
	default:
		return "provide_missing_information"
	}
}

func inferPendingFields(step PlanStep, question string) []string {
	source := strings.ToLower(strings.TrimSpace(question))
	if source == "" {
		sourceBytes, _ := json.Marshal(step.ToolArgs)
		source = strings.ToLower(string(sourceBytes))
	}
	candidates := []struct {
		key      string
		patterns []string
	}{
		{key: "ai_model_id", patterns: []string{"ai_model_id", "model id", "模型id", "模型 id"}},
		{key: "exchange_id", patterns: []string{"exchange_id", "exchange id", "交易所id", "交易所 id"}},
		{key: "strategy_id", patterns: []string{"strategy_id", "strategy id", "策略id", "策略 id"}},
		{key: "name", patterns: []string{"trader name", "name", "名字", "名称"}},
		{key: "api_key", patterns: []string{"api key", "apikey", "api_key"}},
		{key: "secret_key", patterns: []string{"secret key", "secret_key", "密钥"}},
		{key: "passphrase", patterns: []string{"passphrase", "密码短语"}},
	}
	fields := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		for _, pattern := range candidate.patterns {
			if strings.Contains(source, pattern) {
				fields = append(fields, candidate.key)
				break
			}
		}
	}
	return cleanStringList(fields)
}

func inferConfirmationTarget(goal string, step PlanStep, question string) string {
	if step.RequiresConfirmation {
		if step.ToolName != "" {
			return step.ToolName
		}
	}
	lowerGoal := strings.ToLower(strings.TrimSpace(goal))
	lowerQuestion := strings.ToLower(strings.TrimSpace(question))
	switch {
	case strings.Contains(lowerGoal, "交易员") || strings.Contains(lowerQuestion, "交易员") || strings.Contains(lowerGoal, "trader"):
		return "trader"
	case strings.Contains(lowerGoal, "交易所") || strings.Contains(lowerQuestion, "交易所") || strings.Contains(lowerGoal, "exchange"):
		return "exchange_config"
	case strings.Contains(lowerGoal, "模型") || strings.Contains(lowerQuestion, "模型") || strings.Contains(lowerGoal, "model"):
		return "model_config"
	default:
		return ""
	}
}

func isConfigOrTraderIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	keywords := []string{
		"交易员", "trader", "exchange", "交易所", "模型", "model", "api key", "apikey",
		"绑定", "配置", "setup", "configure", "deepseek", "openai", "claude", "gemini",
		"okx", "binance", "bybit", "gate", "kucoin", "hyperliquid", "aster", "lighter",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func isStrategyIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	keywords := []string{
		"策略", "strategy", "template", "模板", "激进", "趋势跟踪", "网格策略",
		"量化策略", "策略模板", "strategy studio",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func isRealtimeAccountIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	keywords := []string{
		"余额", "balance", "equity", "净值", "available", "available balance",
		"持仓", "position", "positions", "仓位", "unrealized pnl", "浮盈", "浮亏",
		"交易历史", "trade history", "history", "closed trades", "recent trades",
		"订单", "order", "orders", "成交", "pnl", "profit", "loss",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func snapshotKindsForIntent(userText string) []string {
	kinds := make([]string, 0, 6)
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" || isRealtimeAccountIntent(lower) {
		return nil
	}

	configKeywords := []string{
		"交易员", "trader", "traders",
		"交易所", "exchange", "exchanges",
		"模型", "model", "models", "llm", "ai model",
		"策略", "strategy", "strategies",
		"配置", "config", "setup", "create", "创建", "修改", "更新", "删除", "delete",
	}
	if containsAnyKeyword(lower, configKeywords) {
		kinds = append(kinds,
			"current_model_configs",
			"current_exchange_configs",
			"current_traders",
		)
		if strings.Contains(lower, "策略") || strings.Contains(lower, "strategy") {
			kinds = append(kinds, "current_strategies")
		}
	}
	return uniqueStrings(kinds)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func withPlannerStageTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= timeout {
			return context.WithCancel(ctx)
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func isPlannerTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func plannerTimeoutMessage(lang string) string {
	if lang == "zh" {
		return "⏱️ 当前请求处理超时，请重试一次。若持续出现，请把问题拆小一点。"
	}
	return "⏱️ This request timed out. Please try again, or break it into a smaller request."
}

func shouldResetExecutionStateForNewAttempt(text string, state ExecutionState) bool {
	if state.SessionID == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	retrySignals := []string{
		"再试", "重试", "重新", "继续", "继续创建", "我已经配置好了", "已经配置好了", "我配好了",
		"我已经弄好了", "已经弄好了", "好了", "retry", "try again", "continue", "resume",
		"i configured it", "i've configured it", "i already configured", "configured already",
	}
	for _, signal := range retrySignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	if isConfigOrTraderIntent(lower) && (state.Status == executionStatusFailed || state.Status == executionStatusCompleted) {
		return true
	}
	if isConfigOrTraderIntent(lower) && state.Status == executionStatusWaitingUser {
		return true
	}
	return false
}

func ensureCurrentReferences(state *ExecutionState) {
	if state.CurrentReferences == nil {
		state.CurrentReferences = &CurrentReferences{}
	}
}

func preferReference(current **EntityReference, id, name, source string) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	source = strings.TrimSpace(source)
	if id == "" && name == "" {
		return
	}
	if *current == nil {
		*current = &EntityReference{}
	}
	if id != "" {
		(*current).ID = id
	}
	if name != "" {
		(*current).Name = name
	}
	if source != "" {
		(*current).Source = source
	}
	(*current).UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func appendReferenceHistory(state *ExecutionState, kind, id, name, source string) {
	if state == nil {
		return
	}
	kind = strings.TrimSpace(kind)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	source = strings.TrimSpace(source)
	if kind == "" || (id == "" && name == "") {
		return
	}
	state.ReferenceHistory = append(state.ReferenceHistory, ReferenceRecord{
		Kind:      kind,
		ID:        id,
		Name:      name,
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	state.ReferenceHistory = normalizeReferenceHistory(state.ReferenceHistory)
}

func matchEntityReference(text string, candidates []EntityReference) *EntityReference {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return nil
	}
	var matched *EntityReference
	for _, candidate := range candidates {
		id := strings.ToLower(strings.TrimSpace(candidate.ID))
		name := strings.ToLower(strings.TrimSpace(candidate.Name))
		if id == "" && name == "" {
			continue
		}
		if (id != "" && strings.Contains(lower, id)) || (name != "" && strings.Contains(lower, name)) {
			if matched != nil {
				return nil
			}
			copy := candidate
			matched = &copy
		}
	}
	return matched
}

func (a *Agent) refreshCurrentReferencesForUserText(storeUserID, text string, state *ExecutionState) {
	if a.store == nil || strings.TrimSpace(text) == "" {
		return
	}
	ensureCurrentReferences(state)

	if strategies, err := a.store.Strategy().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(strategies))
		for _, strategy := range strategies {
			candidates = append(candidates, EntityReference{ID: strategy.ID, Name: strategy.Name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Strategy, ref.ID, ref.Name, "user_mention")
			appendReferenceHistory(state, "strategy", ref.ID, ref.Name, "user_mention")
		}
	}
	if traders, err := a.store.Trader().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(traders))
		for _, trader := range traders {
			candidates = append(candidates, EntityReference{ID: trader.ID, Name: trader.Name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Trader, ref.ID, ref.Name, "user_mention")
			appendReferenceHistory(state, "trader", ref.ID, ref.Name, "user_mention")
		}
	}
	if models, err := a.store.AIModel().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(models))
		for _, model := range models {
			name := model.Name
			if name == "" {
				name = model.CustomModelName
			}
			if name == "" {
				name = model.Provider
			}
			candidates = append(candidates, EntityReference{ID: model.ID, Name: name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Model, ref.ID, ref.Name, "user_mention")
			appendReferenceHistory(state, "model", ref.ID, ref.Name, "user_mention")
		}
	}
	if exchanges, err := a.store.Exchange().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(exchanges))
		for _, exchange := range exchanges {
			name := exchange.AccountName
			if name == "" {
				name = exchange.ExchangeType
			}
			candidates = append(candidates, EntityReference{ID: exchange.ID, Name: name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Exchange, ref.ID, ref.Name, "user_mention")
			appendReferenceHistory(state, "exchange", ref.ID, ref.Name, "user_mention")
		}
	}
}

func updateCurrentReferencesFromToolResult(state *ExecutionState, toolName, raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}
	ensureCurrentReferences(state)
	before, _ := json.Marshal(state.CurrentReferences)

	switch toolName {
	case "manage_strategy":
		if item, ok := payload["strategy"].(map[string]any); ok {
			id, name := asString(item["id"]), asString(item["name"])
			preferReference(&state.CurrentReferences.Strategy, id, name, "tool_output")
			appendReferenceHistory(state, "strategy", id, name, "tool_output")
		}
	case "manage_trader":
		if item, ok := payload["trader"].(map[string]any); ok {
			id, name := asString(item["id"]), asString(item["name"])
			preferReference(&state.CurrentReferences.Trader, id, name, "tool_output")
			appendReferenceHistory(state, "trader", id, name, "tool_output")
			preferReference(&state.CurrentReferences.Model, asString(item["ai_model_id"]), "", "tool_output")
			preferReference(&state.CurrentReferences.Exchange, asString(item["exchange_id"]), "", "tool_output")
			preferReference(&state.CurrentReferences.Strategy, asString(item["strategy_id"]), "", "tool_output")
		}
	case "manage_model_config":
		if item, ok := payload["model"].(map[string]any); ok {
			name := asString(item["name"])
			if name == "" {
				name = asString(item["provider"])
			}
			id := asString(item["id"])
			preferReference(&state.CurrentReferences.Model, id, name, "tool_output")
			appendReferenceHistory(state, "model", id, name, "tool_output")
		}
	case "manage_exchange_config":
		if item, ok := payload["exchange"].(map[string]any); ok {
			name := asString(item["account_name"])
			if name == "" {
				name = asString(item["exchange_type"])
			}
			id := asString(item["id"])
			preferReference(&state.CurrentReferences.Exchange, id, name, "tool_output")
			appendReferenceHistory(state, "exchange", id, name, "tool_output")
		}
	case "get_strategies":
		if items, ok := payload["strategies"].([]any); ok {
			var matched map[string]any
			if len(items) == 1 {
				matched, _ = items[0].(map[string]any)
			} else {
				goal := strings.ToLower(strings.TrimSpace(state.Goal))
				for _, it := range items {
					item, ok := it.(map[string]any)
					if !ok {
						continue
					}
					name := strings.ToLower(strings.TrimSpace(asString(item["name"])))
					if name != "" && goal != "" && strings.Contains(goal, name) {
						matched = item
						break
					}
				}
			}
			if matched != nil {
				id, name := asString(matched["id"]), asString(matched["name"])
				preferReference(&state.CurrentReferences.Strategy, id, name, "tool_output")
				appendReferenceHistory(state, "strategy", id, name, "tool_output")
			}
		}
	}
	state.CurrentReferences = normalizeCurrentReferences(state.CurrentReferences)
	after, _ := json.Marshal(state.CurrentReferences)
	return string(before) != string(after)
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func detectReadFastPath(text string) *readFastPathRequest {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return nil
	}

	switch lower {
	case "/traders":
		return &readFastPathRequest{Kind: "list_traders"}
	case "/strategies":
		return &readFastPathRequest{Kind: "get_strategies"}
	case "/models":
		return &readFastPathRequest{Kind: "get_model_configs"}
	case "/exchanges":
		return &readFastPathRequest{Kind: "get_exchange_configs"}
	case "/balance":
		return &readFastPathRequest{Kind: "get_balance"}
	case "/positions":
		return &readFastPathRequest{Kind: "get_positions"}
	case "/history", "/trades":
		return &readFastPathRequest{Kind: "get_trade_history", ArgsJSON: `{"limit":10}`}
	default:
		switch {
		case containsAnyKeyword(lower, []string{"列出", "查看", "看看", "查询", "list", "show"}) && containsAnyKeyword(lower, []string{"策略", "strategy"}):
			return &readFastPathRequest{Kind: "get_strategies"}
		case containsAnyKeyword(lower, []string{"列出", "查看", "看看", "查询", "list", "show"}) && containsAnyKeyword(lower, []string{"交易员", "trader"}):
			return &readFastPathRequest{Kind: "list_traders"}
		case containsAnyKeyword(lower, []string{"列出", "查看", "看看", "查询", "list", "show"}) && containsAnyKeyword(lower, []string{"模型", "model"}):
			return &readFastPathRequest{Kind: "get_model_configs"}
		case containsAnyKeyword(lower, []string{"列出", "查看", "看看", "查询", "list", "show"}) && containsAnyKeyword(lower, []string{"交易所", "exchange"}):
			return &readFastPathRequest{Kind: "get_exchange_configs"}
		default:
			return nil
		}
	}
}

func isEphemeralReadFastPathKind(kind string) bool {
	switch kind {
	case "get_balance", "get_positions", "get_trade_history":
		return true
	default:
		return false
	}
}

func (a *Agent) executeReadFastPath(storeUserID string, _ int64, req *readFastPathRequest) string {
	switch req.Kind {
	case "get_balance":
		return a.toolGetBalance()
	case "get_positions":
		return a.toolGetPositions()
	case "get_trade_history":
		return a.toolGetTradeHistory(req.ArgsJSON)
	case "get_strategies":
		return a.toolGetStrategies(storeUserID)
	case "list_traders":
		return a.toolListTraders(storeUserID)
	case "get_model_configs":
		return a.toolGetModelConfigs(storeUserID)
	case "get_exchange_configs":
		return a.toolGetExchangeConfigs(storeUserID)
	default:
		return `{"error":"unsupported fast path"}`
	}
}

func formatReadFastPathResponse(lang, kind, raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return summarizeObservation(raw)
	}
	if errMsg, _ := payload["error"].(string); strings.TrimSpace(errMsg) != "" {
		return summarizeObservation(raw)
	}

	switch kind {
	case "get_strategies":
		items, _ := payload["strategies"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有策略。"
			}
			return "There are no strategies yet."
		}
		lines := []string{"Current strategies:"}
		if lang == "zh" {
			lines[0] = "当前策略："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["name"])
			if name == "" {
				name = asString(entry["id"])
			}
			meta := make([]string, 0, 2)
			if active, _ := entry["is_active"].(bool); active {
				meta = append(meta, "active")
			}
			if isDefault, _ := entry["is_default"].(bool); isDefault {
				meta = append(meta, "default")
			}
			if len(meta) > 0 {
				lines = append(lines, fmt.Sprintf("- %s (%s)", name, strings.Join(meta, ", ")))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
		}
		return strings.Join(lines, "\n")
	case "list_traders":
		items, _ := payload["traders"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有交易员。"
			}
			return "There are no traders yet."
		}
		lines := []string{"Current traders:"}
		if lang == "zh" {
			lines[0] = "当前交易员："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["name"])
			line := fmt.Sprintf("- %s", name)
			meta := cleanStringList([]string{asString(entry["exchange_type"]), asString(entry["ai_model_id"])})
			if len(meta) > 0 {
				line += fmt.Sprintf(" (%s)", strings.Join(meta, ", "))
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	case "get_model_configs":
		items, _ := payload["model_configs"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有模型配置。"
			}
			return "There are no model configs yet."
		}
		lines := []string{"Current model configs:"}
		if lang == "zh" {
			lines[0] = "当前模型配置："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["name"])
			if name == "" {
				name = asString(entry["provider"])
			}
			meta := make([]string, 0, 2)
			if enabled, _ := entry["enabled"].(bool); enabled {
				meta = append(meta, "enabled")
			}
			if model := asString(entry["custom_model_name"]); model != "" {
				meta = append(meta, model)
			}
			if len(meta) > 0 {
				lines = append(lines, fmt.Sprintf("- %s (%s)", name, strings.Join(meta, ", ")))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
		}
		return strings.Join(lines, "\n")
	case "get_exchange_configs":
		items, _ := payload["exchange_configs"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有交易所配置。"
			}
			return "There are no exchange configs yet."
		}
		lines := []string{"Current exchange configs:"}
		if lang == "zh" {
			lines[0] = "当前交易所配置："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["account_name"])
			if name == "" {
				name = asString(entry["exchange_type"])
			}
			meta := cleanStringList([]string{asString(entry["exchange_type"])})
			if enabled, _ := entry["enabled"].(bool); enabled {
				meta = append(meta, "enabled")
			}
			if len(meta) > 0 {
				lines = append(lines, fmt.Sprintf("- %s (%s)", name, strings.Join(meta, ", ")))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
		}
		return strings.Join(lines, "\n")
	case "get_balance":
		items, _ := payload["balances"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前没有可用的余额数据。"
			}
			return "No balance data is available right now."
		}
		lines := []string{"Current balance overview:"}
		if lang == "zh" {
			lines[0] = "当前余额概览："
		}
		var totalEquity float64
		var totalAvailable float64
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			equity := toFloat(entry["total_equity"])
			available := toFloat(entry["available"])
			totalEquity += equity
			totalAvailable += available
			lines = append(lines, fmt.Sprintf("- %s (%s): equity %.4f, available %.4f",
				asString(entry["name"]), asString(entry["exchange"]),
				equity, available))
		}
		if len(items) > 1 {
			if lang == "zh" {
				lines = append(lines, fmt.Sprintf("汇总：equity %.4f, available %.4f", totalEquity, totalAvailable))
			} else {
				lines = append(lines, fmt.Sprintf("Total: equity %.4f, available %.4f", totalEquity, totalAvailable))
			}
		}
		return strings.Join(lines, "\n")
	case "get_positions":
		items, _ := payload["positions"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前没有持仓。"
			}
			return "There are no open positions right now."
		}
		lines := []string{"Current positions:"}
		if lang == "zh" {
			lines[0] = "当前持仓："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s %s size %.4f, entry %.4f, pnl %.4f",
				asString(entry["symbol"]), asString(entry["side"]),
				toFloat(entry["size"]), toFloat(entry["entry_price"]), toFloat(entry["unrealized_pnl"])))
		}
		return strings.Join(lines, "\n")
	case "get_trade_history":
		items, _ := payload["trades"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前没有已平仓交易历史。"
			}
			return "There is no closed trade history yet."
		}
		summary, _ := payload["summary"].(map[string]any)
		head := fmt.Sprintf("Recent trades: %.0f total, win rate %s, total PnL %.4f",
			toFloat(summary["total_trades"]), asString(summary["win_rate"]), toFloat(summary["total_pnl"]))
		if lang == "zh" {
			head = fmt.Sprintf("最近交易：共 %.0f 笔，胜率 %s，总 PnL %.4f",
				toFloat(summary["total_trades"]), asString(summary["win_rate"]), toFloat(summary["total_pnl"]))
		}
		lines := []string{head}
		for idx, item := range items {
			if idx >= 5 {
				break
			}
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s %s pnl %.4f (%s -> %s)",
				asString(entry["symbol"]), asString(entry["side"]), toFloat(entry["pnl"]),
				asString(entry["entry_time"]), asString(entry["exit_time"])))
		}
		return strings.Join(lines, "\n")
	default:
		return summarizeObservation(raw)
	}
}

func (a *Agent) thinkAndAct(ctx context.Context, storeUserID string, userID int64, lang, text string) (string, error) {
	lock := a.flowLock(userID)
	lock.Lock()
	defer lock.Unlock()
	if a.aiClient != nil {
		if answer, ok, err := a.tryLLMIntentRoute(ctx, storeUserID, userID, lang, text, nil); ok || err != nil {
			return a.maybeAppendResumePrompt(userID, lang, text, answer), err
		}
	} else if a.hasAnyActiveContext(userID) {
		if answer, ok, err := a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, nil); ok || err != nil {
			return a.maybeAppendResumePrompt(userID, lang, text, answer), err
		}
	}
	if a.aiClient == nil {
		if !a.hasAnyActiveContext(userID) {
			if answer, ok, err := a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, nil); ok || err != nil {
				return a.maybeAppendResumePrompt(userID, lang, text, answer), err
			}
		}
		if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, nil); ok {
			return a.maybeAppendResumePrompt(userID, lang, text, answer), nil
		}
		if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, nil); ok {
			return a.maybeAppendResumePrompt(userID, lang, text, answer), nil
		}
		return a.noAIFallback(lang, text)
	}
	answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, nil)
	return a.maybeAppendResumePrompt(userID, lang, text, answer), err
}

func (a *Agent) thinkAndActStream(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, error) {
	lock := a.flowLock(userID)
	lock.Lock()
	defer lock.Unlock()
	if a.aiClient != nil {
		if answer, ok, err := a.tryLLMIntentRoute(ctx, storeUserID, userID, lang, text, onEvent); ok || err != nil {
			answer = a.maybeAppendResumePrompt(userID, lang, text, answer)
			return answer, err
		}
	} else if a.hasAnyActiveContext(userID) {
		if answer, ok, err := a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent); ok || err != nil {
			answer = a.maybeAppendResumePrompt(userID, lang, text, answer)
			return answer, err
		}
	}
	if a.aiClient == nil {
		if !a.hasAnyActiveContext(userID) {
			if answer, ok, err := a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent); ok || err != nil {
				answer = a.maybeAppendResumePrompt(userID, lang, text, answer)
				return answer, err
			}
		}
		if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, onEvent); ok {
			answer = a.maybeAppendResumePrompt(userID, lang, text, answer)
			return answer, nil
		}
		if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
			return a.maybeAppendResumePrompt(userID, lang, text, answer), nil
		}
		return a.noAIFallback(lang, text)
	}
	answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
	return a.maybeAppendResumePrompt(userID, lang, text, answer), err
}

func isInstantDirectReplyText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch lower {
	case "hi", "hello", "hey", "你好", "嗨", "在吗", "你好吗", "最近怎么样", "最近还好吗", "谢谢", "多谢", "谢了", "ok", "好的", "收到", "thanks", "thank you", "okay", "got it", "how are you":
		return true
	default:
		return false
	}
}

func (a *Agent) hasActiveSkillSession(userID int64) bool {
	session := a.getSkillSession(userID)
	return strings.TrimSpace(session.Name) != ""
}

func (a *Agent) hasAnyActiveContext(userID int64) bool {
	if a.hasActiveSkillSession(userID) {
		return true
	}
	if hasActiveWorkflowSession(a.getWorkflowSession(userID)) {
		return true
	}
	return hasActiveExecutionState(a.getExecutionState(userID))
}

func hasActiveExecutionState(state ExecutionState) bool {
	if strings.TrimSpace(state.SessionID) == "" {
		return false
	}
	switch strings.TrimSpace(state.Status) {
	case executionStatusPlanning, executionStatusRunning, executionStatusWaitingUser:
		return true
	default:
		return false
	}
}

func (a *Agent) tryStatePriorityPath(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	if answer, ok := a.tryResumeSuspendedTask(userID, lang, text); ok {
		return answer, true, nil
	}
	if !a.hasActiveSkillSession(userID) && !hasActiveWorkflowSession(a.getWorkflowSession(userID)) && !hasActiveExecutionState(a.getExecutionState(userID)) {
		if a.tryRestoreSuspendedTaskFromIdle(ctx, userID, lang, text) {
			return a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent)
		}
	}
	if workflow := a.getWorkflowSession(userID); hasActiveWorkflowSession(workflow) {
		answer, handled, err := a.handleWorkflowSession(ctx, storeUserID, userID, lang, text, workflow, onEvent)
		if handled || err != nil {
			return answer, true, err
		}
	}
	if session := a.getSkillSession(userID); strings.TrimSpace(session.Name) != "" {
		if answer, ok := a.answerSkillSessionExplanation(storeUserID, lang, session, text); ok {
			return answer, true, nil
		}
		decision, extraction := a.resolveSkillSessionTurn(ctx, userID, lang, text, session)
		switch decision.Intent {
		case "cancel":
			a.clearSkillSession(userID)
			a.clearWorkflowSession(userID)
			return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
		case "instant_reply":
			return a.replyToActiveFlowInstantReply(ctx, userID, lang, text, onEvent), true, nil
		case "resume_snapshot", "start_new":
			answer, handled, err := a.handoffFromActiveFlow(ctx, storeUserID, userID, lang, text, decision.TargetSnapshotID, onEvent)
			return answer, handled, err
		default:
			if extraction.Intent == "continue" {
				a.applyLLMExtractionToSkillSession(storeUserID, &session, extraction, lang, text)
				a.saveSkillSession(userID, session)
			}
			if answer, ok := a.dispatchBridgedSkillSession(storeUserID, userID, lang, text, session); ok {
				if onEvent != nil && strings.TrimSpace(answer) != "" {
					switch session.Name {
					case "trader_management":
						onEvent(StreamEventTool, "hard_skill:trader_management")
					case "model_management":
						onEvent(StreamEventTool, "hard_skill:model_management")
					case "exchange_management":
						onEvent(StreamEventTool, "hard_skill:exchange_management")
					case "strategy_management":
						onEvent(StreamEventTool, "hard_skill:strategy_management")
					}
					emitStreamText(onEvent, answer)
				}
				return answer, true, nil
			}
		}
	}

	state := a.getExecutionState(userID)
	if hasActiveExecutionState(state) {
		decision, extraction := a.resolveExecutionStateTurn(ctx, userID, lang, state, text)
		switch decision.Intent {
		case "cancel":
			a.clearExecutionState(userID)
			return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
		case "instant_reply":
			return a.replyToActiveFlowInstantReply(ctx, userID, lang, text, onEvent), true, nil
		case "resume_snapshot", "start_new":
			answer, handled, err := a.handoffFromActiveFlow(ctx, storeUserID, userID, lang, text, decision.TargetSnapshotID, onEvent)
			return answer, handled, err
		default:
			if decision.Intent == "continue_active" {
				if session, ok := a.bridgeExecutionStateToSkillSession(storeUserID, userID, text, state, extraction); ok {
					answer, handled := a.dispatchBridgedSkillSession(storeUserID, userID, lang, text, session)
					return answer, handled, nil
				}
			}
			if extraction.Intent == "continue" {
				a.applyExecutionStateExtraction(&state, extraction)
				if err := a.saveExecutionState(state); err != nil {
					return "", true, err
				}
			}
			answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
			return answer, true, err
		}
	}

	return "", false, nil
}

func isTraderCreateWaitingState(state ExecutionState) bool {
	lowerGoal := strings.ToLower(strings.TrimSpace(state.Goal))
	if strings.Contains(lowerGoal, "创建交易员") || strings.Contains(lowerGoal, "新建交易员") || strings.Contains(lowerGoal, "create trader") {
		return true
	}
	if state.Waiting == nil {
		return false
	}
	lowerIntent := strings.ToLower(strings.TrimSpace(state.Waiting.Intent))
	lowerTarget := strings.ToLower(strings.TrimSpace(state.Waiting.ConfirmationTarget))
	return lowerIntent == "complete_trader_setup" || (lowerIntent == "confirm_action" && lowerTarget == "trader")
}

func hasSkillBridgeSignal(a *Agent, storeUserID, skillName, action, text string, extraction executionFlowExtractionResult) bool {
	if len(extraction.Fields) > 0 {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if isYesReply(text) || isNoReply(text) {
		return true
	}
	switch skillName {
	case "trader_management":
		if containsAny(lower, []string{"名称", "名字", "name", "交易所", "exchange", "模型", "model", "策略", "strategy"}) {
			return true
		}
	case "model_management":
		if containsAny(lower, []string{"provider", "模型名", "模型名称", "api key", "api_key", "apikey", "url", "endpoint", "名称", "名字", "name"}) {
			return true
		}
	case "exchange_management":
		if containsAny(lower, []string{"交易所", "exchange", "账户名", "account", "api key", "secret", "passphrase", "testnet", "名称", "名字", "name"}) {
			return true
		}
	case "strategy_management":
		if containsAny(lower, []string{"策略", "strategy", "名称", "名字", "name", "prompt", "提示词", "配置", "参数"}) {
			return true
		}
	}
	if action == "create" && containsAny(lower, []string{"名称", "名字", "name"}) {
		return true
	}
	if a == nil {
		return false
	}
	return hasStrictOptionMention(text, a.loadEnabledModelOptions(storeUserID)) ||
		hasStrictOptionMention(text, a.loadExchangeOptions(storeUserID)) ||
		hasStrictOptionMention(text, a.loadStrategyOptions(storeUserID))
}

func inferExecutionStateSkillBridge(state ExecutionState, text string) (string, string) {
	lowerGoal := strings.ToLower(strings.TrimSpace(state.Goal))
	waitingIntent := ""
	waitingTarget := ""
	if state.Waiting != nil {
		waitingIntent = strings.ToLower(strings.TrimSpace(state.Waiting.Intent))
		waitingTarget = strings.ToLower(strings.TrimSpace(state.Waiting.ConfirmationTarget))
	}
	switch waitingIntent {
	case "complete_trader_setup":
		return "trader_management", "create"
	case "complete_model_config":
		return "model_management", "create"
	case "complete_exchange_config":
		return "exchange_management", "create"
	}
	switch waitingTarget {
	case "trader":
		if containsAny(lowerGoal, []string{"创建", "新建", "create", "setup", "配置"}) || hasExplicitCreateIntentForDomain(state.Goal, "trader") {
			return "trader_management", "create"
		}
		return "trader_management", "create"
	case "model", "model_config":
		return "model_management", "create"
	case "exchange", "exchange_config":
		return "exchange_management", "create"
	case "strategy", "manage_strategy":
		return "strategy_management", "create"
	}
	switch {
	case hasExplicitCreateIntentForDomain(state.Goal, "trader"):
		return "trader_management", "create"
	}
	return "", ""
}

func traderCreateFieldsFromExecutionExtraction(result executionFlowExtractionResult) map[string]string {
	if len(result.Fields) == 0 {
		return nil
	}
	fields := make(map[string]string, len(result.Fields))
	for key, value := range result.Fields {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			fields["name"] = value
		case "model", "model_id", "ai_model_id":
			fields["model_id"] = value
		case "model_name":
			fields["model_name"] = value
		case "exchange", "exchange_id":
			fields["exchange_id"] = value
		case "exchange_name":
			fields["exchange_name"] = value
		case "strategy", "strategy_id":
			fields["strategy_id"] = value
		case "strategy_name":
			fields["strategy_name"] = value
		case "auto_start", "scan_interval_minutes", "is_cross_margin", "show_in_competition":
			fields[key] = value
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func executionStateCurrentReference(state ExecutionState, skillName string) *EntityReference {
	if state.CurrentReferences == nil {
		return nil
	}
	switch skillName {
	case "trader_management":
		return state.CurrentReferences.Trader
	case "model_management":
		return state.CurrentReferences.Model
	case "exchange_management":
		return state.CurrentReferences.Exchange
	case "strategy_management":
		return state.CurrentReferences.Strategy
	default:
		return nil
	}
}

func (a *Agent) bridgeExecutionStateToSkillSession(storeUserID string, userID int64, text string, state ExecutionState, extraction executionFlowExtractionResult) (skillSession, bool) {
	skillName, action := inferExecutionStateSkillBridge(state, text)
	if a == nil || skillName == "" || action == "" || !hasSkillBridgeSignal(a, storeUserID, skillName, action, text, extraction) {
		return skillSession{}, false
	}

	session := a.getSkillSession(userID)
	if session.Name != "" && (session.Name != skillName || session.Action != action) {
		return skillSession{}, false
	}
	if session.Name == "" {
		session = skillSession{
			Name:   skillName,
			Action: action,
			Phase:  "collecting",
		}
	}
	if len(extraction.Fields) > 0 {
		fields := extraction.Fields
		if skillName == "trader_management" {
			fields = traderCreateFieldsFromExecutionExtraction(extraction)
		}
		if len(fields) > 0 {
			a.applyLLMExtractionToSkillSession(storeUserID, &session, llmFlowExtractionResult{
				Tasks: []llmFlowExtractionTask{{
					Skill:  skillName,
					Action: action,
					Fields: fields,
				}},
			}, "zh", text)
		}
	}

	switch skillName {
	case "trader_management":
		if action != "create" && session.TargetRef == nil {
			session.TargetRef = normalizeEntityReference(executionStateCurrentReference(state, "trader_management"))
		}
		a.hydrateCreateTraderSlotReferences(storeUserID, &session)
	case "model_management":
		if session.TargetRef == nil && action != "create" {
			session.TargetRef = normalizeEntityReference(executionStateCurrentReference(state, "model_management"))
		}
	case "exchange_management":
		if session.TargetRef == nil && action != "create" {
			session.TargetRef = normalizeEntityReference(executionStateCurrentReference(state, "exchange_management"))
		}
	case "strategy_management":
		if session.TargetRef == nil && action != "create" {
			session.TargetRef = normalizeEntityReference(executionStateCurrentReference(state, "strategy_management"))
		}
	}
	a.saveSkillSession(userID, session)
	a.clearExecutionState(userID)
	return session, true
}

func (a *Agent) dispatchBridgedSkillSession(storeUserID string, userID int64, lang, text string, session skillSession) (string, bool) {
	switch session.Name {
	case "trader_management":
		if session.Action == "create" {
			return a.handleCreateTraderSkill(storeUserID, userID, lang, text, session)
		}
		return a.handleTraderManagementSkill(storeUserID, userID, lang, text, session)
	case "model_management":
		if session.Action == "create" {
			return a.handleModelCreateSkill(storeUserID, userID, lang, text, session), true
		}
		return a.handleModelManagementSkill(storeUserID, userID, lang, text, session)
	case "exchange_management":
		if session.Action == "create" {
			return a.handleExchangeCreateSkill(storeUserID, userID, lang, text, session), true
		}
		return a.handleExchangeManagementSkill(storeUserID, userID, lang, text, session)
	case "strategy_management":
		if session.Action == "create" {
			return a.handleStrategyCreateSkill(storeUserID, userID, lang, text, session), true
		}
		return a.handleStrategyManagementSkill(storeUserID, userID, lang, text, session)
	default:
		return "", false
	}
}

func (a *Agent) resolveSkillSessionTurn(ctx context.Context, userID int64, lang, text string, session skillSession) (unifiedFlowDecision, llmFlowExtractionResult) {
	text = strings.TrimSpace(text)
	if text == "" {
		return unifiedFlowDecision{Intent: "continue_active"}, llmFlowExtractionResult{}
	}
	if isInstantDirectReplyText(text) {
		return unifiedFlowDecision{Intent: "instant_reply"}, llmFlowExtractionResult{Intent: "instant_reply"}
	}
	if a.aiClient != nil {
		result := a.extractSkillSessionFieldsWithLLM(ctx, userID, lang, text, session)
		if decision := unifiedFlowDecisionFromIntent(result.Intent, result.TargetSnapshotID); decision.Intent != "" {
			return decision, result
		}
	}
	return a.classifySkillSessionDecision(ctx, userID, lang, session, text), llmFlowExtractionResult{}
}

func (a *Agent) resolveExecutionStateTurn(ctx context.Context, userID int64, lang string, state ExecutionState, text string) (unifiedFlowDecision, executionFlowExtractionResult) {
	text = strings.TrimSpace(text)
	if text == "" {
		return unifiedFlowDecision{Intent: "continue_active"}, executionFlowExtractionResult{}
	}
	if isInstantDirectReplyText(text) {
		return unifiedFlowDecision{Intent: "instant_reply"}, executionFlowExtractionResult{Intent: "instant_reply"}
	}
	if a.aiClient != nil {
		result := a.extractExecutionStateContinuationWithLLM(ctx, userID, lang, state, text)
		if decision := unifiedFlowDecisionFromIntent(result.Intent, result.TargetSnapshotID); decision.Intent != "" {
			return decision, result
		}
	}
	return a.classifyExecutionStateDecision(ctx, userID, lang, state, text), executionFlowExtractionResult{}
}

func unifiedFlowDecisionFromIntent(intent, targetSnapshotID string) unifiedFlowDecision {
	intent = strings.TrimSpace(strings.ToLower(intent))
	targetSnapshotID = strings.TrimSpace(targetSnapshotID)
	switch intent {
	case "continue", "continue_active":
		return unifiedFlowDecision{Intent: "continue_active"}
	case "cancel":
		return unifiedFlowDecision{Intent: "cancel"}
	case "instant_reply":
		return unifiedFlowDecision{Intent: "instant_reply"}
	case "switch", "interrupt", "start_new", "resume_snapshot":
		if targetSnapshotID != "" {
			return unifiedFlowDecision{Intent: "resume_snapshot", TargetSnapshotID: targetSnapshotID}
		}
		return unifiedFlowDecision{Intent: "start_new"}
	default:
		return unifiedFlowDecision{}
	}
}

func (a *Agent) replyToActiveFlowInstantReply(ctx context.Context, userID int64, lang, text string, onEvent func(event, data string)) string {
	a.suspendActiveContexts(userID, lang)
	if a.aiClient != nil {
		if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, onEvent); ok {
			return a.maybeAppendResumePrompt(userID, lang, text, answer)
		}
	}
	if lang == "zh" {
		return a.maybeAppendResumePrompt(userID, lang, text, "在，有什么我帮你看的？")
	}
	return a.maybeAppendResumePrompt(userID, lang, text, "I'm here. What would you like me to help you with?")
}

func (a *Agent) handoffFromActiveFlow(ctx context.Context, storeUserID string, userID int64, lang, text, targetSnapshotID string, onEvent func(event, data string)) (string, bool, error) {
	if a.suspendAndTryRestoreSuspendedTask(userID, lang, text, targetSnapshotID) {
		return a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent)
	}
	if answer, ok, err := a.tryLLMIntentRoute(ctx, storeUserID, userID, lang, text, onEvent); ok || err != nil {
		return a.maybeAppendResumePrompt(userID, lang, text, answer), true, err
	}
	if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, onEvent); ok {
		return a.maybeAppendResumePrompt(userID, lang, text, answer), true, nil
	}
	if a.aiClient == nil {
		if a.tryRestoreSuspendedTaskAfterSwitch(userID, text, "") {
			if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
				return answer, true, nil
			}
		}
		if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
			return a.maybeAppendResumePrompt(userID, lang, text, answer), true, nil
		}
		answer, err := a.noAIFallback(lang, text)
		return a.maybeAppendResumePrompt(userID, lang, text, answer), true, err
	}
	answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
	return a.maybeAppendResumePrompt(userID, lang, text, answer), true, err
}

func (a *Agent) extractExecutionStateContinuationWithLLM(ctx context.Context, userID int64, lang string, state ExecutionState, text string) executionFlowExtractionResult {
	if a == nil || a.aiClient == nil || strings.TrimSpace(text) == "" {
		return executionFlowExtractionResult{}
	}
	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	flowContext := fmt.Sprintf(
		"Active flow type: execution_state\nGoal: %s\nStatus: %s",
		state.Goal,
		state.Status,
	)
	waitingSummary := ""
	if state.Waiting != nil {
		waitingSummary = fmt.Sprintf("Waiting summary: question=%s pending_fields=%s", strings.TrimSpace(state.Waiting.Question), strings.Join(state.Waiting.PendingFields, ", "))
	}
	systemPrompt, userPrompt := buildActiveFlowExtractionPrompt(
		lang,
		"execution_state",
		flowContext,
		text,
		recentConversationCtx,
		state.CurrentReferences,
		a.SnapshotManager(userID).List(),
		[]string{
			fmt.Sprintf("Waiting JSON: %s", mustMarshalJSON(state.Waiting)),
			waitingSummary,
		},
	)
	systemPrompt += `
- This is the structured continuation input for an active NOFXi execution flow.
- Prefer "continue" only when the message clearly contributes to the current waiting question or active execution goal.
- Use "switch" for read-only queries, unrelated requests, explanation requests, or clear topic changes.
- For "continue", extract only explicit field values that answer the waiting question or pending fields.
- Do not invent fields. If no field can be safely extracted, you may still return "continue" when the message is a meaningful free-form answer.

Return JSON with this exact shape:
{"intent":"continue|switch|cancel|instant_reply","target_snapshot_id":"","fields":{},"reason":""}`
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
		return executionFlowExtractionResult{}
	}
	envelope, ok := parseRawFlowExtractionEnvelope(raw)
	if !ok {
		return executionFlowExtractionResult{}
	}
	out := executionFlowExtractionResult{
		Intent:           envelope.Intent,
		TargetSnapshotID: envelope.TargetSnapshotID,
		Reason:           envelope.Reason,
	}
	if len(envelope.Fields) > 0 {
		out.Fields = envelope.Fields
	} else if len(envelope.Tasks) > 0 {
		out.Fields = envelope.Tasks[0].Fields
	}
	switch out.Intent {
	case "continue", "switch", "cancel", "instant_reply", "interrupt":
		return out
	default:
		return executionFlowExtractionResult{}
	}
}

func parseSuspendedTaskSelectionResult(raw string) (suspendedTaskSelectionResult, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var out suspendedTaskSelectionResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start || json.Unmarshal([]byte(raw[start:end+1]), &out) != nil {
			return suspendedTaskSelectionResult{}, false
		}
	}
	out.TargetSnapshotID = strings.TrimSpace(out.TargetSnapshotID)
	if out.TargetSnapshotID == "" {
		return suspendedTaskSelectionResult{}, false
	}
	return out, true
}

func (a *Agent) applyExecutionStateExtraction(state *ExecutionState, result executionFlowExtractionResult) {
	if state == nil || result.Intent != "continue" {
		return
	}
	if len(result.Fields) == 0 && strings.TrimSpace(result.Reason) == "" {
		return
	}
	fieldBits := make([]string, 0, len(result.Fields))
	for key, value := range result.Fields {
		fieldBits = append(fieldBits, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(fieldBits)
	summary := "User continued the active execution flow."
	if len(fieldBits) > 0 {
		summary = "User supplied continuation fields: " + strings.Join(fieldBits, ", ")
	}
	appendExecutionLog(state, Observation{
		Kind:      "waiting_user_input",
		Summary:   summary,
		RawJSON:   mustMarshalJSON(result),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if state.Waiting != nil && len(state.Waiting.PendingFields) > 0 && len(result.Fields) > 0 {
		remaining := make([]string, 0, len(state.Waiting.PendingFields))
		for _, field := range state.Waiting.PendingFields {
			if _, ok := result.Fields[field]; ok {
				continue
			}
			remaining = append(remaining, field)
		}
		state.Waiting.PendingFields = cleanStringList(remaining)
	}
}

func (a *Agent) classifySkillSessionDecision(ctx context.Context, userID int64, lang string, session skillSession, text string) unifiedFlowDecision {
	return unifiedFlowDecisionFromIntent(a.classifySkillSessionInput(ctx, userID, lang, session, text), "")
}

func (a *Agent) classifySkillSessionInput(ctx context.Context, userID int64, lang string, session skillSession, text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return "continue"
	}
	if isYesReply(text) || isNoReply(text) {
		return "continue"
	}
	if isExplicitFlowAbort(text) {
		return "cancel"
	}
	if shouldContinueSkillSessionByExpectedSlot(session, text) {
		return "continue"
	}
	if strings.TrimSpace(session.Name) == "trader_management" && strings.TrimSpace(session.Action) == "create" {
		if detectReadFastPath(text) == nil {
			switch detectMentionedSkillDomain(text) {
			case "exchange_management", "model_management", "strategy_management":
				return "continue"
			}
		}
	}
	if a != nil && a.aiClient != nil {
		if decision := a.classifySkillSessionIntentWithLLM(ctx, userID, lang, session, text); decision != "" {
			return decision
		}
		return "continue"
	}
	if strings.TrimSpace(session.Name) != "" && strings.TrimSpace(session.Action) != "" &&
		!looksLikeNewTopLevelIntent(text) {
		return "continue"
	}
	if shouldInterruptSkillSessionBySnapshot(session, text) || shouldInterruptSkillSessionByExplicitDomainMention(session, text) || isNewSkillRootIntent(session, text) || isSkillFlowDeflection(session, text) {
		return "interrupt"
	}
	if belongsToSkillDomain(session.Name, text) || !looksLikeNewTopLevelIntent(text) {
		return "continue"
	}
	return "interrupt"
}

type activeFlowIntentDecision struct {
	Decision string `json:"decision"`
}

type unifiedFlowDecision struct {
	Intent           string
	TargetSnapshotID string
}

type executionFlowExtractionResult struct {
	Intent           string            `json:"intent,omitempty"`
	TargetSnapshotID string            `json:"target_snapshot_id,omitempty"`
	Fields           map[string]string `json:"fields,omitempty"`
	Reason           string            `json:"reason,omitempty"`
}

type suspendedTaskSelectionResult struct {
	TargetSnapshotID string `json:"target_snapshot_id,omitempty"`
}

func buildActiveFlowClassifierPrompt(lang, flowLabel, flowContext, text, recentConversationCtx string, currentRefs any, suspendedSnapshots any) (string, string) {
	systemPrompt := `You classify one user message while an active NOFXi flow is in progress.
Return JSON only. No markdown.

Possible decisions:
- "continue": the user is still continuing the current active flow
- "cancel": the user wants to stop the current active flow
- "interrupt": the user wants to leave the current active flow for another task, query, explanation, or topic
- "instant_reply": the user is only greeting, chatting, or thanking

Be conservative:
- Prefer "continue" only when the message still contributes to the current active flow.
- Use "cancel" for explicit abandonment.
- Use "instant_reply" for greetings, thanks, and simple social chat.
- Use "interrupt" for unrelated requests, explanation requests, read-only queries, or clear topic shifts.
- Consider Current references JSON and Suspended snapshots JSON when resolving vague phrases like "那个", "刚才那个", or "前面那个".

Return JSON with this exact shape:
{"decision":"continue|cancel|interrupt|instant_reply"}`
	return systemPrompt, fmt.Sprintf(
		"Language: %s\nActive flow label: %s\n%s\nCurrent references JSON: %s\nSuspended snapshots JSON: %s\nUser message: %s\n\nRecent conversation:\n%s",
		lang,
		flowLabel,
		flowContext,
		mustMarshalJSON(currentRefs),
		mustMarshalJSON(suspendedSnapshots),
		text,
		recentConversationCtx,
	)
}

func parseActiveFlowIntentDecision(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var decision activeFlowIntentDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start || json.Unmarshal([]byte(raw[start:end+1]), &decision) != nil {
			return ""
		}
	}
	switch strings.TrimSpace(decision.Decision) {
	case "continue", "cancel", "interrupt", "instant_reply":
		return decision.Decision
	default:
		return ""
	}
}

func shouldUseLLMSkillSessionClassifier(session skillSession, text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if isExplicitFlowAbort(text) || isYesReply(text) || isNoReply(text) {
		return false
	}
	if shouldContinueSkillSessionByExpectedSlot(session, text) {
		return false
	}
	return true
}

func detectRootSkillIntent(text string) string {
	return ""
}

func shouldInterruptSkillSessionBySnapshot(session skillSession, text string) bool {
	currentSkill := strings.TrimSpace(session.Name)
	if currentSkill == "" {
		return false
	}
	rootSkill := detectRootSkillIntent(text)
	if rootSkill == "" {
		return false
	}
	if rootSkill != currentSkill && looksLikeNewTopLevelIntent(text) {
		return true
	}
	return false
}

func detectMentionedSkillDomain(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(lower, []string{"交易员", "trader", "agent"}):
		return "trader_management"
	case containsAny(lower, []string{"策略", "strategy"}):
		return "strategy_management"
	case containsAny(lower, []string{"模型", "model"}):
		return "model_management"
	case containsAny(lower, []string{"交易所", "exchange"}):
		return "exchange_management"
	default:
		return ""
	}
}

func shouldInterruptSkillSessionByExplicitDomainMention(session skillSession, text string) bool {
	currentSkill := strings.TrimSpace(session.Name)
	if currentSkill == "" {
		return false
	}
	if currentSkill == "trader_management" {
		if currentStep, ok := currentSkillDAGStep(session); ok {
			switch currentStep.ID {
			case "resolve_exchange", "resolve_model", "resolve_strategy", "collect_bindings":
				return false
			}
		}
	}
	mentioned := detectMentionedSkillDomain(text)
	if mentioned == "" || mentioned == currentSkill {
		return false
	}
	return looksLikeNewTopLevelIntent(text)
}

func looksLikeExplanationQuestion(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"怎么", "什么意思", "为什么", "格式", "要求", "示例", "例子", "怎么填", "怎么写", "是啥", "是什么", "有哪些", "有什么", "可选", "选项",
		"how", "what", "why", "format", "requirement", "example", "examples", "what is", "how should", "which options", "available",
	})
}

func explanationFieldForSession(session skillSession, text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch strings.TrimSpace(session.Name) {
	case "model_management":
		if containsAny(lower, []string{"provider", "提供商", "模型类型", "模型有哪些类型"}) {
			return "provider"
		}
		if containsAny(lower, []string{"api key", "apikey", "api_key", "密钥"}) {
			return "api_key"
		}
		if containsAny(lower, []string{"model name", "模型名称", "模型名"}) {
			return "custom_model_name"
		}
		if containsAny(lower, []string{"url", "endpoint", "地址", "接口"}) {
			return "custom_api_url"
		}
	case "exchange_management":
		if containsAny(lower, []string{"交易所类型", "交易所有哪些类型", "支持哪些交易所", "哪些交易所", "exchange type", "exchange types", "which exchanges", "supported exchanges"}) {
			return "exchange_type"
		}
		if containsAny(lower, []string{"api key", "apikey", "api_key"}) {
			return "api_key"
		}
		if containsAny(lower, []string{"secret", "secret key", "secret_key", "密钥"}) {
			return "secret_key"
		}
		if containsAny(lower, []string{"passphrase", "密码短语"}) {
			return "passphrase"
		}
	case "trader_management":
		if containsAny(lower, []string{"扫描间隔", "scan interval", "scan frequency"}) {
			return "scan_interval_minutes"
		}
	case "strategy_management":
		if containsAny(lower, []string{"杠杆", "leverage"}) {
			return "leverage"
		}
	}
	if currentStep, ok := currentSkillDAGStep(session); ok {
		switch currentStep.ID {
		case "resolve_provider":
			return "provider"
		case "resolve_exchange_type":
			return "exchange_type"
		case "resolve_name", "collect_name":
			return "name"
		case "collect_custom_api_url":
			return "custom_api_url"
		case "collect_custom_model_name":
			return "custom_model_name"
		case "collect_enabled":
			return "enabled"
		case "collect_field_value":
			return fieldValue(session, "update_field")
		}
	}
	return ""
}

func (a *Agent) answerSkillSessionExplanation(storeUserID, lang string, session skillSession, text string) (string, bool) {
	if !looksLikeExplanationQuestion(text) {
		return "", false
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if containsAny(lower, []string{"选项", "有哪些", "有什么", "可选", "available options", "which options"}) {
		if session.Name == "model_management" {
			provider := strings.TrimSpace(fieldValue(session, "provider"))
			if provider == "" {
				return modelProviderChoicePrompt(lang), true
			}
			if lang == "zh" {
				return fmt.Sprintf("当前 provider 是 %s。这里要填的是这个 provider 实际支持的模型名称，例如 OpenAI 常见是 `gpt-5`、`gpt-5-mini`，DeepSeek 常见是 `deepseek-chat`。你也可以直接告诉我“用默认推荐模型”。", provider), true
			}
			return fmt.Sprintf("The current provider is %s. This field expects a real runtime model ID for that provider, for example `gpt-5` or `gpt-5-mini` for OpenAI, or `deepseek-chat` for DeepSeek. You can also tell me to use the default recommended model.", provider), true
		}
		if summary := a.skillVisibleOptionSummary(storeUserID, lang, session.Name, session.Action); summary != "" {
			return summary, true
		}
	}
	if detectSkillQuestionField(session.Name, text, skillSession{Name: session.Name}) == "" &&
		containsAny(lower, []string{"字段", "界面", "可配", "能配", "what fields", "which fields", "available fields"}) {
		if summary := a.skillVisibleFieldSummary(storeUserID, lang, session.Name, session.Action); summary != "" {
			return summary, true
		}
	}
	field := explanationFieldForSession(session, text)
	if lang == "zh" {
		switch field {
		case "api_key":
			if session.Name == "model_management" {
				provider := strings.TrimSpace(fieldValue(session, "provider"))
				if provider != "" {
					if spec, ok := modelProviderSpecByID(provider); ok && spec.UsesWalletCredential {
						return modelProviderCredentialGuidance(lang, provider), true
					}
				}
				return "API Key 一般是你在模型提供商后台生成的密钥。以 OpenAI 为例，通常以 `sk-` 开头，后面跟一长串字母数字。不要带引号外的说明文字，也不要把别的字段一起贴进来。你直接把完整 API Key 发我就行，我会继续当前模型草稿。", true
			}
			return "API Key 是交易所/服务商发给你的访问密钥，一般是一长串字母数字。你直接把完整值发我就行，不用附带说明文字。", true
		case "secret_key":
			return "Secret 是和 API Key 配套的密钥，通常也是一长串字符串。直接把完整 Secret 发我就行，不要和 API Key 填反。", true
		case "passphrase":
			return "Passphrase 是部分交易所额外要求的密码短语。像 OKX 这类账户除了 API Key 和 Secret 之外，还需要这个字段。直接把完整 Passphrase 发我就行。", true
		case "custom_model_name":
			return "模型名称填提供商实际可调用的模型 ID 就行，比如 OpenAI 常见是 `gpt-5`、`gpt-5-mini` 这类。你直接告诉我要用哪个模型名，我继续当前草稿。", true
		case "custom_api_url":
			return "接口地址填这个 provider 对应的 Base URL。OpenAI 常见是 `https://api.openai.com/v1`。如果你用官方地址，也可以直接告诉我“用默认地址”。", true
		case "provider":
			if options := a.modelSkillOptionSummary(lang); options != "" {
				return options + "。你直接说其中一个就行。", true
			}
			return "这里要填模型提供商，比如 OpenAI、DeepSeek、Claude、Gemini、Qwen、Kimi、Grok、Minimax。你直接说其中一个就行。", true
		case "exchange_type":
			if options := a.exchangeSkillOptionSummary(lang); options != "" {
				return options + "。你直接说要接哪个交易所就行。", true
			}
			return "这里要填交易所类型，比如 OKX、Binance、Bybit、Gate、KuCoin、Hyperliquid。你直接说要接哪个交易所就行。", true
		case "name":
			return "这里要填你想给这个对象起的名字，方便后面识别和管理。你直接说“叫 XXX”就行。", true
		case "enabled":
			return "启用状态就是创建后是否立即生效。你可以说“启用/开启”或“禁用/先不开启”。", true
		case "scan_interval_minutes":
			return "扫描间隔是交易员多久扫描一次市场，单位是分钟。你直接给我一个数字就行，比如 `5`、`15`。", true
		case "leverage":
			return "杠杆就是策略允许使用的倍数。你可以直接给我数字，但系统会按安全上限做约束。", true
		case "source_type":
			return "选币来源就是策略最初从哪里拿候选币。手动面板里常见可选项有：`static`（你自己指定静态币池）、`ai500`、`oi_top`、`oi_low`。如果你要自己指定币，就继续告诉我“静态币用 BTC、ETH”；如果你想排除某些币，也可以直接补“排除币不要 SOL、DOGE”。", true
		case "max_positions":
			return "最大持仓就是同一时间最多允许开几个仓位。你直接给我整数就行，比如 `3`、`5`、`10`。", true
		case "min_confidence":
			return "最小置信度是策略允许开仓的最低信心门槛，通常填 `0-100` 的整数。数值越高，开单会更谨慎。", true
		case "min_risk_reward_ratio":
			return "最小盈亏比是每笔交易至少要满足的风险收益比，比如 `1.5`、`2.0`。数值越高，策略会更挑机会。", true
		case "primary_timeframe":
			return "主周期是策略主要参考的 K 线周期，比如 `1m`、`5m`、`15m`、`1h`。如果你偏高频，一般会更短；偏稳一些就会更长。", true
		case "selected_timeframes":
			return "多周期时间框架就是除了主周期之外，还一起参考哪些周期。常见填法像 `1m,5m,15m` 或 `5m,15m,1h`，直接按逗号列给我就行。", true
		}
		if summary := a.skillVisibleFieldSummary(storeUserID, lang, session.Name, session.Action); summary != "" {
			return summary, true
		}
		return "", false
	}
	switch field {
	case "api_key":
		return "The API key is the secret token issued by the provider. For OpenAI it usually starts with `sk-` followed by a long string. Just send the full API key and I'll keep the current draft going.", true
	case "secret_key":
		return "The secret key is the credential paired with your API key. Send the full secret value directly, and make sure it isn't swapped with the API key.", true
	case "passphrase":
		return "The passphrase is an extra credential required by some exchanges such as OKX. Send the full passphrase value and I'll continue the current draft.", true
	case "custom_model_name":
		return "The model name should be the real runtime model ID exposed by the provider, such as `gpt-5` or `gpt-5-mini`. Tell me which one you want and I'll continue the draft.", true
	case "custom_api_url":
		return "The API URL should be the provider's base endpoint. For OpenAI, a common value is `https://api.openai.com/v1`. You can also tell me to use the default endpoint.", true
	case "provider":
		if options := a.modelSkillOptionSummary(lang); options != "" {
			return options + ". You can reply with any one of them.", true
		}
		return "The provider should be one of the supported vendors like OpenAI, DeepSeek, Claude, Gemini, Qwen, Kimi, Grok, or Minimax.", true
	case "exchange_type":
		if options := a.exchangeSkillOptionSummary(lang); options != "" {
			return options + ". You can reply with the one you want to connect.", true
		}
		return "The exchange type should be the venue you want to connect, such as OKX, Binance, Bybit, Gate, KuCoin, or Hyperliquid.", true
	case "name":
		return "This field is just the display name for the object. You can answer with something like 'call it X'.", true
	case "enabled":
		return "Enabled means whether the config should be active right away. You can answer with enable/disable.", true
	case "scan_interval_minutes":
		return "The scan interval is the number of minutes between trader scans. Just send a number such as `5` or `15`.", true
	case "leverage":
		return "Leverage is the multiplier the strategy is allowed to use. You can send a number, and the system will still enforce safety limits.", true
	}
	if summary := a.skillVisibleFieldSummary(storeUserID, lang, session.Name, session.Action); summary != "" {
		return summary, true
	}
	return "", false
}

func shouldContinueSkillSessionByExpectedSlot(session skillSession, text string) bool {
	return false
}

func (a *Agent) classifySkillSessionIntentWithLLM(ctx context.Context, userID int64, lang string, session skillSession, text string) string {
	if a == nil || a.aiClient == nil {
		return ""
	}
	if !shouldUseLLMSkillSessionClassifier(session, text) {
		return ""
	}
	currentStep, _ := currentSkillDAGStep(session)
	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	state := a.getExecutionState(userID)
	flowContext := fmt.Sprintf(
		"Active skill: %s\nAction: %s\nCurrent DAG step: %s\nExpected required fields: %s\nSkill session fields JSON: %s",
		session.Name,
		session.Action,
		currentStep.ID,
		strings.Join(currentStep.RequiredFields, ", "),
		mustMarshalJSON(session.Fields),
	)
	if skillContext := buildCurrentSkillExecutionContext(lang, session); skillContext != "" {
		flowContext += "\n" + skillContext
	}
	systemPrompt, userPrompt := buildActiveFlowClassifierPrompt(
		lang,
		"skill_session",
		flowContext,
		text,
		recentConversationCtx,
		state.CurrentReferences,
		a.SnapshotManager(userID).List(),
	)
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
		return ""
	}
	return parseActiveFlowIntentDecision(raw)
}

func (a *Agent) classifyExecutionStateIntentWithLLM(ctx context.Context, userID int64, lang string, state ExecutionState, text string) string {
	if a == nil || a.aiClient == nil {
		return ""
	}
	if strings.TrimSpace(text) == "" || isExplicitFlowAbort(text) || isYesReply(text) || isNoReply(text) || shouldResetExecutionStateForNewAttempt(text, state) {
		return ""
	}
	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	flowContext := fmt.Sprintf(
		"Goal: %s\nStatus: %s\nWaiting JSON: %s",
		state.Goal,
		state.Status,
		mustMarshalJSON(state.Waiting),
	)
	systemPrompt, userPrompt := buildActiveFlowClassifierPrompt(
		lang,
		"execution_state",
		flowContext,
		text,
		recentConversationCtx,
		state.CurrentReferences,
		a.SnapshotManager(userID).List(),
	)
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
		return ""
	}
	return parseActiveFlowIntentDecision(raw)
}

func isSkillFlowDeflection(session skillSession, text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if containsAny(lower, []string{
		"看下报错", "看看报错", "帮我看下报错", "帮我看看报错", "报错怎么回事", "错误怎么回事",
		"换话题", "聊别的", "不是这个", "先说别的", "不聊这个",
	}) {
		return true
	}
	switch strings.TrimSpace(session.Name) {
	case "exchange_management":
		return hasExplicitDiagnosisIntentForDomain(text, "model") || hasExplicitDiagnosisIntentForDomain(text, "trader") || hasExplicitDiagnosisIntentForDomain(text, "strategy")
	case "model_management":
		return hasExplicitDiagnosisIntentForDomain(text, "exchange") || hasExplicitDiagnosisIntentForDomain(text, "trader") || hasExplicitDiagnosisIntentForDomain(text, "strategy")
	case "strategy_management":
		return hasExplicitDiagnosisIntentForDomain(text, "exchange") || hasExplicitDiagnosisIntentForDomain(text, "trader") || hasExplicitDiagnosisIntentForDomain(text, "model")
	case "trader_management":
		return hasExplicitDiagnosisIntentForDomain(text, "exchange") || hasExplicitDiagnosisIntentForDomain(text, "model") || hasExplicitDiagnosisIntentForDomain(text, "strategy")
	default:
		return false
	}
}

func isNewSkillRootIntent(session skillSession, text string) bool {
	currentSkill := strings.TrimSpace(session.Name)
	currentAction := strings.TrimSpace(session.Action)
	if currentSkill == "" {
		return false
	}
	if currentSkill != "trader_management" && hasExplicitManagementDomainCue(text, "trader") && containsAny(strings.ToLower(strings.TrimSpace(text)), []string{"创建", "新建", "create", "new"}) {
		return true
	}
	if currentSkill != "strategy_management" && hasExplicitManagementDomainCue(text, "strategy") && containsAny(strings.ToLower(strings.TrimSpace(text)), []string{"创建", "新建", "create", "new"}) {
		return true
	}
	if currentSkill != "model_management" && hasExplicitManagementDomainCue(text, "model") && containsAny(strings.ToLower(strings.TrimSpace(text)), []string{"创建", "新建", "create", "new"}) {
		return true
	}
	if currentSkill != "exchange_management" && hasExplicitManagementDomainCue(text, "exchange") && containsAny(strings.ToLower(strings.TrimSpace(text)), []string{"创建", "新建", "create", "new"}) {
		return true
	}
	switch currentSkill {
	case "trader_management":
		return hasExplicitCreateIntentForDomain(text, "trader") && currentAction != "create"
	case "strategy_management":
		return hasExplicitManagementDomainCue(text, "strategy") && containsAny(strings.ToLower(strings.TrimSpace(text)), []string{"创建", "新建", "create", "new"}) && currentAction != "create"
	case "model_management":
		return hasExplicitManagementDomainCue(text, "model") && containsAny(strings.ToLower(strings.TrimSpace(text)), []string{"创建", "新建", "create", "new"}) && currentAction != "create"
	case "exchange_management":
		return hasExplicitManagementDomainCue(text, "exchange") && containsAny(strings.ToLower(strings.TrimSpace(text)), []string{"创建", "新建", "create", "new"}) && currentAction != "create"
	}
	return false
}

func shouldSuspendInterruptedTask(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if isConfigOrTraderIntent(text) || detectRootSkillIntent(text) != "" {
		return false
	}
	if hasExplicitManagementDomainCue(text, "trader") || hasExplicitManagementDomainCue(text, "model") ||
		hasExplicitManagementDomainCue(text, "exchange") || hasExplicitManagementDomainCue(text, "strategy") {
		return false
	}
	if req := detectReadFastPath(text); req != nil {
		return isEphemeralReadFastPathKind(req.Kind)
	}
	return containsAny(lower, []string{
		"btc", "eth", "sol", "价格", "行情", "balance", "position", "positions", "portfolio",
		"market", "price", "仓位", "持仓", "余额", "账户", "trade history", "历史成交",
	})
}

func (a *Agent) classifyExecutionStateDecision(ctx context.Context, userID int64, lang string, state ExecutionState, text string) unifiedFlowDecision {
	return unifiedFlowDecisionFromIntent(a.classifyExecutionStateInput(ctx, userID, lang, state, text), "")
}

func (a *Agent) classifyExecutionStateInput(ctx context.Context, userID int64, lang string, state ExecutionState, text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return "continue"
	}
	if isExplicitFlowAbort(text) {
		return "cancel"
	}
	if isYesReply(text) || isNoReply(text) || shouldResetExecutionStateForNewAttempt(text, state) {
		return "continue"
	}
	if a != nil && a.aiClient != nil {
		if decision := a.classifyExecutionStateIntentWithLLM(ctx, userID, lang, state, text); decision != "" {
			return decision
		}
		return "continue"
	}
	if state.Waiting != nil && !looksLikeNewTopLevelIntent(text) {
		return "continue"
	}
	if looksLikeNewTopLevelIntent(text) {
		return "interrupt"
	}
	return "continue"
}

func isResumeFlowReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch lower {
	case "继续", "继续吧", "继续刚才的", "恢复", "恢复刚才的", "resume", "continue", "继续创建", "继续配置":
		return true
	default:
		return false
	}
}

func isCancelParentFlowReply(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch lower {
	case "一并取消", "也取消", "都取消", "全部取消", "取消父任务", "cancel all", "cancel parent", "drop all":
		return true
	default:
		return false
	}
}

func suspendedTaskResumePrompt(lang string, task SuspendedTask) string {
	hint := strings.TrimSpace(task.ResumeHint)
	if hint == "" {
		if lang == "zh" {
			hint = "刚才未完成的任务还在，要继续吗？"
		} else {
			hint = "Your previous unfinished task is still here. Do you want to continue?"
		}
	}
	return hint
}

func (a *Agent) maybeOfferParentTaskAfterCancel(userID int64, lang string) string {
	task, ok := a.SnapshotManager(userID).Peek()
	if !ok {
		if lang == "zh" {
			return "已取消当前流程。"
		}
		return "Cancelled the current flow."
	}
	if lang == "zh" {
		return "已取消当前流程。\n" + suspendedTaskResumePrompt(lang, task) + "\n如果父任务也不要了，回复“一并取消”。"
	}
	return "Cancelled the current flow.\n" + suspendedTaskResumePrompt(lang, task) + "\nReply 'cancel all' if you want to cancel the parent task too."
}

func suspendedTaskDomain(task SuspendedTask) string {
	task = normalizeSuspendedTask(task)
	if task.SkillSession != nil {
		return strings.TrimSpace(task.SkillSession.Name)
	}
	if task.WorkflowSession != nil {
		for _, item := range task.WorkflowSession.Tasks {
			if strings.TrimSpace(item.Skill) != "" {
				return strings.TrimSpace(item.Skill)
			}
		}
	}
	return ""
}

func (a *Agent) buildSuspendedTask(userID int64, lang string) SuspendedTask {
	task := SuspendedTask{}
	if session := a.getSkillSession(userID); strings.TrimSpace(session.Name) != "" {
		sessionCopy := normalizeSkillSession(session)
		task.Kind = "skill_session"
		task.SkillSession = &sessionCopy
		task.ResumeHint = buildSkillResumeHint(lang, sessionCopy)
		if sessionCopy.Name == "trader_management" && sessionCopy.Action == "create" {
			task.ResumeOnSuccess = true
			task.ResumeTriggers = []string{"exchange_management", "model_management", "strategy_management"}
		}
	}
	if workflow := a.getWorkflowSession(userID); hasActiveWorkflowSession(workflow) {
		workflowCopy := normalizeWorkflowSession(workflow)
		task.Kind = "workflow_session"
		task.WorkflowSession = &workflowCopy
		if task.ResumeHint == "" {
			task.ResumeHint = buildWorkflowResumeHint(lang, workflowCopy)
		}
	}
	if state := a.getExecutionState(userID); hasActiveExecutionState(state) {
		stateCopy := normalizeExecutionState(state)
		if task.Kind == "" {
			task.Kind = "execution_state"
		}
		task.ExecutionState = &stateCopy
		if task.ResumeHint == "" {
			task.ResumeHint = buildExecutionResumeHint(lang, stateCopy)
		}
	}
	if a.history != nil {
		if msgs := a.history.Get(userID); len(msgs) > 0 {
			if len(msgs) > chatHistoryMaxTurns {
				msgs = msgs[len(msgs)-chatHistoryMaxTurns:]
			}
			task.LocalHistory = msgs
		}
	}
	return normalizeSuspendedTask(task)
}

func buildSkillResumeHint(lang string, session skillSession) string {
	target := ""
	if session.TargetRef != nil {
		target = defaultIfEmpty(session.TargetRef.Name, session.TargetRef.ID)
	}
	if lang == "zh" {
		switch session.Name {
		case "strategy_management":
			if target != "" {
				return fmt.Sprintf("刚才关于策略“%s”的流程还没完成，要继续吗？", target)
			}
			return "刚才的策略配置流程还没完成，要继续吗？"
		case "model_management":
			if target != "" {
				return fmt.Sprintf("刚才关于模型“%s”的流程还没完成，要继续吗？", target)
			}
			return "刚才的模型配置流程还没完成，要继续吗？"
		case "exchange_management":
			if target != "" {
				return fmt.Sprintf("刚才关于交易所“%s”的流程还没完成，要继续吗？", target)
			}
			return "刚才的交易所配置流程还没完成，要继续吗？"
		case "trader_management":
			if target != "" {
				return fmt.Sprintf("刚才关于交易员“%s”的流程还没完成，要继续吗？", target)
			}
			return "刚才的交易员配置流程还没完成，要继续吗？"
		}
	}
	if target != "" {
		return fmt.Sprintf("The flow for %s is still unfinished. Do you want to continue?", target)
	}
	return "The previous configuration flow is still unfinished. Do you want to continue?"
}

func buildWorkflowResumeHint(lang string, session WorkflowSession) string {
	req := strings.TrimSpace(session.OriginalRequest)
	if req == "" {
		if lang == "zh" {
			return "刚才的多步任务还没完成，要继续吗？"
		}
		return "The previous workflow is still unfinished. Do you want to continue?"
	}
	if lang == "zh" {
		return fmt.Sprintf("刚才关于“%s”的多步任务还没完成，要继续吗？", req)
	}
	return fmt.Sprintf("The workflow for %q is still unfinished. Do you want to continue?", req)
}

func buildExecutionResumeHint(lang string, state ExecutionState) string {
	if state.Waiting != nil && strings.TrimSpace(state.Waiting.Question) != "" {
		if lang == "zh" {
			return fmt.Sprintf("刚才我们停在这个问题：%s 回复“继续”我就接着来。", state.Waiting.Question)
		}
		return fmt.Sprintf("We paused at this question: %s Reply 'continue' and I'll resume.", state.Waiting.Question)
	}
	goal := strings.TrimSpace(state.Goal)
	if goal == "" {
		if lang == "zh" {
			return "刚才未完成的任务还在，要继续吗？"
		}
		return "The previous unfinished task is still here. Do you want to continue?"
	}
	if lang == "zh" {
		return fmt.Sprintf("刚才关于“%s”的任务还没完成，要继续吗？", goal)
	}
	return fmt.Sprintf("The task for %q is still unfinished. Do you want to continue?", goal)
}

func (a *Agent) suspendActiveContexts(userID int64, lang string) bool {
	task := a.buildSuspendedTask(userID, lang)
	if task.Kind == "" {
		return false
	}
	a.SnapshotManager(userID).Save(task)
	a.clearSkillSession(userID)
	a.clearWorkflowSession(userID)
	a.clearExecutionState(userID)
	return true
}

func (a *Agent) restoreSuspendedTask(userID int64, task SuspendedTask) bool {
	task = normalizeSuspendedTask(task)
	if task.Kind == "" {
		return false
	}
	a.clearSkillSession(userID)
	a.clearWorkflowSession(userID)
	a.clearExecutionState(userID)
	if a.history != nil && len(task.LocalHistory) > 0 {
		a.history.Replace(userID, task.LocalHistory)
	}
	if task.ExecutionState != nil {
		_ = a.saveExecutionState(*task.ExecutionState)
	}
	if task.WorkflowSession != nil {
		a.saveWorkflowSession(userID, *task.WorkflowSession)
	}
	if task.SkillSession != nil {
		a.saveSkillSession(userID, *task.SkillSession)
	}
	return true
}

func (a *Agent) restoreSuspendedTaskByID(userID int64, snapshotID string) bool {
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return false
	}
	manager := a.SnapshotManager(userID)
	stack := manager.Stack()
	if len(stack) == 0 {
		return false
	}
	match := -1
	for i := len(stack) - 1; i >= 0; i-- {
		if strings.TrimSpace(stack[i].SnapshotID) == snapshotID {
			match = i
			break
		}
	}
	if match < 0 {
		return false
	}
	task, ok := manager.RemoveAt(match)
	if !ok {
		return false
	}
	return a.restoreSuspendedTask(userID, task)
}

func (a *Agent) tryRestoreSuspendedTaskAfterSwitch(userID int64, text, targetSnapshotID string) bool {
	if a.restoreSuspendedTaskByID(userID, targetSnapshotID) {
		return true
	}
	return a.restoreMatchingSuspendedTask(userID, text)
}

func (a *Agent) suspendAndTryRestoreSuspendedTask(userID int64, lang, text, targetSnapshotID string) bool {
	a.suspendActiveContexts(userID, lang)
	return a.tryRestoreSuspendedTaskAfterSwitch(userID, text, targetSnapshotID)
}

func (a *Agent) tryResumeSuspendedTask(userID int64, lang, text string) (string, bool) {
	if isCancelParentFlowReply(text) && !a.hasActiveSkillSession(userID) && !hasActiveWorkflowSession(a.getWorkflowSession(userID)) && !hasActiveExecutionState(a.getExecutionState(userID)) {
		a.SnapshotManager(userID).Clear()
		if lang == "zh" {
			return "已把之前挂起的父任务也一并取消。", true
		}
		return "Cancelled the previously suspended parent tasks as well.", true
	}
	if !isResumeFlowReply(text) {
		return "", false
	}
	if a.hasActiveSkillSession(userID) || hasActiveWorkflowSession(a.getWorkflowSession(userID)) || hasActiveExecutionState(a.getExecutionState(userID)) {
		return "", false
	}
	task, ok := a.SnapshotManager(userID).Load()
	if !ok {
		return "", false
	}
	if !a.restoreSuspendedTask(userID, task) {
		return "", false
	}
	return suspendedTaskResumePrompt(lang, task), true
}

func (a *Agent) tryRestoreSuspendedTaskWithLLM(ctx context.Context, userID int64, lang, text string) bool {
	if a == nil || a.aiClient == nil || strings.TrimSpace(text) == "" {
		return false
	}
	snapshots := a.SnapshotManager(userID).List()
	if len(snapshots) == 0 {
		return false
	}
	snapshotsJSON, _ := json.Marshal(snapshots)
	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	systemPrompt := `You select whether a user message refers to one suspended NOFXi snapshot that should be restored now.
Return JSON only. No markdown.

Rules:
- Choose target_snapshot_id only when the user clearly refers to exactly one suspended snapshot.
- Prefer empty target_snapshot_id when uncertain.
- Use the snapshot resume hint, kind, and recent conversation to resolve references like "刚才那个", "the model one", or "继续那个策略".
- Never invent snapshot ids.

Return JSON with this exact shape:
{"target_snapshot_id":""}`
	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\nSuspended snapshots JSON: %s\n\nRecent conversation:\n%s", lang, text, string(snapshotsJSON), recentConversationCtx)

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
		return false
	}
	selection, ok := parseSuspendedTaskSelectionResult(raw)
	if !ok {
		return false
	}
	return a.restoreSuspendedTaskByID(userID, selection.TargetSnapshotID)
}

func (a *Agent) tryRestoreSuspendedTaskFromIdle(ctx context.Context, userID int64, lang, text string) bool {
	if a.tryRestoreAwaitingConfirmationSnapshot(userID, text) {
		return true
	}
	if a.tryRestoreSuspendedTaskWithLLM(ctx, userID, lang, text) {
		return true
	}
	return a.restoreMatchingSuspendedTask(userID, text)
}

func (a *Agent) tryRestoreAwaitingConfirmationSnapshot(userID int64, text string) bool {
	if !isYesReply(text) && !isNoReply(text) && !createConfirmationReply(text) {
		return false
	}
	stack := a.SnapshotManager(userID).Stack()
	if len(stack) != 1 {
		return false
	}
	task := stack[0]
	if task.Kind != "skill_session" || task.SkillSession == nil {
		return false
	}
	phase := strings.TrimSpace(task.SkillSession.Phase)
	switch phase {
	case "await_confirmation", "await_create_confirmation", "await_start_confirmation":
		return a.restoreSuspendedTask(userID, task)
	default:
		return false
	}
}

func (a *Agent) restoreMatchingSuspendedTask(userID int64, text string) bool {
	wanted := detectRootSkillIntent(text)
	if wanted == "" {
		wanted = detectMentionedSkillDomain(text)
	}
	if wanted == "" {
		return false
	}
	manager := a.SnapshotManager(userID)
	fullStack := manager.Stack()
	if len(fullStack) == 0 {
		return false
	}
	match := -1
	for i := len(fullStack) - 1; i >= 0; i-- {
		if suspendedTaskDomain(fullStack[i]) == wanted {
			match = i
			break
		}
	}
	if match < 0 {
		return false
	}
	task, ok := manager.RemoveAt(match)
	if !ok {
		return false
	}
	return a.restoreSuspendedTask(userID, task)
}

func (a *Agent) maybeAppendResumePrompt(userID int64, lang, text, answer string) string {
	a.trackPendingProposalSession(userID, lang, text, answer)
	if strings.TrimSpace(answer) == "" || !shouldSuspendInterruptedTask(text) {
		return answer
	}
	if a.hasActiveSkillSession(userID) || hasActiveWorkflowSession(a.getWorkflowSession(userID)) || hasActiveExecutionState(a.getExecutionState(userID)) {
		return answer
	}
	task, ok := a.SnapshotManager(userID).Peek()
	if !ok {
		return answer
	}
	prompt := suspendedTaskResumePrompt(lang, task)
	if prompt == "" || strings.Contains(answer, prompt) {
		return answer
	}
	return strings.TrimSpace(answer) + "\n\n" + prompt
}

func (a *Agent) trackPendingProposalSession(userID int64, lang, sourceUserText, answer string) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return
	}
	if looksLikePendingProposalReply(answer) {
		if a.hasActiveSkillSession(userID) || hasActiveWorkflowSession(a.getWorkflowSession(userID)) || hasActiveExecutionState(a.getExecutionState(userID)) {
			a.suspendActiveContexts(userID, lang)
		}
		a.clearActiveSkillSession(userID)
		a.savePendingProposalSession(PendingProposalSession{
			UserID:         userID,
			SourceUserText: strings.TrimSpace(sourceUserText),
			ProposalText:   answer,
		})
		return
	}
	a.clearPendingProposalSession(userID)
}

func looksLikePendingProposalReply(answer string) bool {
	lower := strings.ToLower(strings.TrimSpace(answer))
	if lower == "" {
		return false
	}
	return containsAny(lower, []string{
		"需要我按这个方案操作吗",
		"按这个方案操作吗",
		"你想选哪个",
		"请选择",
		"两个选择",
		"直接使用已有的",
		"which option do you want",
		"do you want me to follow this plan",
		"should i proceed with this plan",
	})
}

func isExplicitFlowAbort(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if isCancelSkillReply(text) {
		return true
	}
	return containsAny(lower, []string{
		"算了", "先不", "不配了", "别弄了", "不搞了", "先停", "换个话题", "换话题", "聊点别的", "聊别的",
		"stop this", "drop it", "never mind", "forget it", "skip this",
	})
}

func belongsToSkillDomain(skillName, text string) bool {
	switch strings.TrimSpace(skillName) {
	case "trader_management":
		return hasExplicitCreateIntentForDomain(text, "trader") || detectTraderManagementIntent(text) || hasExplicitDiagnosisIntentForDomain(text, "trader")
	case "strategy_management":
		return detectStrategyManagementIntent(text) || hasExplicitDiagnosisIntentForDomain(text, "strategy")
	case "model_management":
		return detectModelManagementIntent(text) ||
			hasExplicitDiagnosisIntentForDomain(text, "model")
	case "exchange_management":
		return detectExchangeManagementIntent(text) ||
			hasExplicitDiagnosisIntentForDomain(text, "exchange")
	default:
		return false
	}
}

func looksLikeNewTopLevelIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if strings.HasPrefix(lower, "/") {
		return true
	}
	if hasExplicitCreateIntentForDomain(text, "trader") ||
		detectTraderManagementIntent(text) ||
		detectExchangeManagementIntent(text) ||
		detectModelManagementIntent(text) ||
		detectStrategyManagementIntent(text) ||
		hasExplicitDiagnosisIntentForDomain(text, "trader") ||
		hasExplicitDiagnosisIntentForDomain(text, "exchange") ||
		hasExplicitDiagnosisIntentForDomain(text, "model") ||
		hasExplicitDiagnosisIntentForDomain(text, "strategy") {
		return true
	}
	if detectReadFastPath(text) != nil {
		return true
	}
	return containsAny(lower, []string{
		"btc", "eth", "sol", "市场", "行情", "余额", "仓位", "持仓", "订单", "账户",
		"price", "market", "balance", "position", "portfolio", "account",
	})
}

func (a *Agent) tryDirectAnswer(ctx context.Context, userID int64, lang, text string, onEvent func(event, data string)) (string, bool) {
	if a.aiClient == nil {
		return "", false
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}

	currentTurnCtx := a.buildCurrentTurnContext(userID, lang, text)
	activeTaskCtx := a.buildActiveTaskStateContext(userID, lang)
	systemPrompt := `You are the first-pass router for NOFXi.
Decide whether the assistant can answer the user's message directly without using skills, tools, or planning.
Return JSON only. Do not return markdown.

Use "direct_answer" only when a concise, self-contained answer is sufficient.
Examples that often fit direct_answer:
- greetings, thanks, small talk
- concept explanations
- open-ended advice that does not require current system state
- trading education or opinion questions that can be answered from general reasoning

Use "defer" when the message likely needs:
- a management or diagnosis skill
- tool reads
- multi-step planning
- continuation of an active execution flow that needs stateful follow-up
- interpretation of current product state, observations, counts, duplicates, balances, configuration-page findings, or anything that sounds like "I see / I noticed / there are still ..."

Rules:
- If you choose direct_answer, write for a trading beginner, not a developer.
- Keep the answer simple, clear, and easy to act on.
- Lead with the conclusion first, then one or two concrete next steps when helpful.
- Avoid internal jargon, architecture talk, tool names, or implementation detail unless the user explicitly asks.
- Use Current turn context as the primary memory for this turn.
- Use Active task state only as a compact summary of any unfinished operational flow.
- Default to direct_answer for greetings, thanks, identity questions, and other lightweight conversational turns unless there is a clearly unfinished operational flow that the user is continuing.
- If the user is clearly continuing an unfinished operational flow, choose defer.
- If the user mentions concrete operational entities or observations such as traders, strategies, models, exchanges, balances, counts, duplicate items, config pages, or numeric account facts, choose defer.
- If you choose direct_answer, provide the final user-facing answer in the same language as the user.
- Prefer defer when uncertain.

Return JSON with this exact shape:
{"action":"direct_answer|defer","answer":""}`
	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nCurrent turn context:\n%s\n\nActive task state:\n%s", lang, text, defaultIfEmpty(currentTurnCtx, "(empty)"), defaultIfEmpty(activeTaskCtx, "(empty)"))

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

	decision, err := parseDirectReplyDecision(raw)
	if err != nil {
		return "", false
	}
	if decision.Action != "direct_answer" {
		return "", false
	}

	answer := strings.TrimSpace(decision.Answer)
	if answer == "" {
		return "", false
	}

	if a.history == nil {
		a.history = newChatHistory(chatHistoryMaxTurns)
	}
	a.history.Add(userID, "user", text)
	a.history.Add(userID, "assistant", answer)
	a.maybeUpdateTaskStateIncrementally(ctx, userID)
	a.maybeCompressHistory(ctx, userID)
	if onEvent != nil {
		emitStreamText(onEvent, answer)
	}
	return answer, true
}

func parseDirectReplyDecision(raw string) (directReplyDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision directReplyDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeDirectReplyDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeDirectReplyDecision(decision), nil
		}
	}
	return directReplyDecision{}, fmt.Errorf("invalid direct reply decision json")
}

func normalizeDirectReplyDecision(decision directReplyDecision) directReplyDecision {
	decision.Action = strings.TrimSpace(strings.ToLower(decision.Action))
	decision.Answer = strings.TrimSpace(decision.Answer)
	return decision
}

func looksLikeInternalAgentJSON(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}
	if _, ok := payload["intent"]; ok {
		if _, hasTasks := payload["tasks"]; hasTasks {
			return true
		}
		if _, hasFields := payload["fields"]; hasFields {
			return true
		}
		if _, hasReason := payload["reason"]; hasReason {
			return true
		}
	}
	return false
}

func firstFlowExtractionFields(result llmFlowExtractionResult) map[string]string {
	if len(result.Fields) > 0 {
		return result.Fields
	}
	if len(result.Tasks) > 0 && len(result.Tasks[0].Fields) > 0 {
		return result.Tasks[0].Fields
	}
	return nil
}

func (a *Agent) tryRecoverFromInternalAgentJSON(ctx context.Context, storeUserID string, userID int64, lang, text, raw string, onEvent func(event, data string)) (string, bool, error) {
	result := parseLLMFlowExtractionResult(raw)
	if result.Intent == "" {
		return "", false, nil
	}
	switch result.Intent {
	case "instant_reply":
		return a.replyToActiveFlowInstantReply(ctx, userID, lang, text, onEvent), true, nil
	case "cancel":
		if a.hasActiveSkillSession(userID) {
			a.clearSkillSession(userID)
		}
		if hasActiveExecutionState(a.getExecutionState(userID)) {
			a.clearExecutionState(userID)
		}
		return a.maybeOfferParentTaskAfterCancel(userID, lang), true, nil
	case "continue":
		if session := a.getSkillSession(userID); strings.TrimSpace(session.Name) != "" {
			a.applyLLMExtractionToSkillSession(storeUserID, &session, result, lang, text)
			a.saveSkillSession(userID, session)
			if answer, ok := a.dispatchBridgedSkillSession(storeUserID, userID, lang, text, session); ok {
				return answer, true, nil
			}
		}
		if len(result.Tasks) > 0 {
			task := result.Tasks[0]
			if strings.TrimSpace(task.Skill) != "" {
				recovered := skillSession{
					Name:   strings.TrimSpace(task.Skill),
					Action: strings.TrimSpace(task.Action),
					Phase:  "collecting",
					Fields: map[string]string{},
				}
				if suspended, ok := a.SnapshotManager(userID).Peek(); ok && suspended.SkillSession != nil {
					suspendedSkill := strings.TrimSpace(suspended.SkillSession.Name)
					suspendedAction := strings.TrimSpace(suspended.SkillSession.Action)
					if suspendedSkill == recovered.Name && (recovered.Action == "" || suspendedAction == recovered.Action) {
						recovered = *suspended.SkillSession
					}
				}
				for key, value := range task.Fields {
					setField(&recovered, key, value)
				}
				recovered = normalizeSkillSession(recovered)
				if recovered.Name == "trader_management" && recovered.Action == "create" {
					a.hydrateCreateTraderSlotReferences(storeUserID, &recovered)
				}
				if recovered.Name == "trader_management" && recovered.Action == "create" && len(missingFieldKeysForSkillSession(recovered)) == 0 {
					if fieldValue(recovered, "auto_start") == "true" {
						recovered.Phase = "await_start_confirmation"
						a.saveSkillSession(userID, recovered)
						if lang == "zh" {
							return fmt.Sprintf("准备创建交易员并立即启动。\n交易所：%s\n模型：%s\n策略：%s\n\n回复确认继续，回复先不用则只创建不启动。",
								traderCreateExchangeNameOrID(recovered), traderCreateModelNameOrID(recovered), traderCreateStrategyNameOrID(recovered)), true, nil
						}
						return fmt.Sprintf("Ready to create trader and start it immediately.\nExchange: %s\nModel: %s\nStrategy: %s\n\nReply confirm to continue, or no to create without starting.",
							traderCreateExchangeNameOrID(recovered), traderCreateModelNameOrID(recovered), traderCreateStrategyNameOrID(recovered)), true, nil
					}
					recovered.Phase = "await_create_confirmation"
					a.saveSkillSession(userID, recovered)
					return formatTraderCreateDraftSummary(lang, recovered), true, nil
				}
				a.saveSkillSession(userID, recovered)
				if answer, ok := a.dispatchBridgedSkillSession(storeUserID, userID, lang, text, recovered); ok {
					return answer, true, nil
				}
			}
		}
		if state := a.getExecutionState(userID); hasActiveExecutionState(state) {
			extraction := executionFlowExtractionResult{
				Intent:           "continue",
				TargetSnapshotID: result.TargetSnapshotID,
				Fields:           firstFlowExtractionFields(result),
				Reason:           result.Reason,
			}
			if session, ok := a.bridgeExecutionStateToSkillSession(storeUserID, userID, text, state, extraction); ok {
				answer, handled := a.dispatchBridgedSkillSession(storeUserID, userID, lang, text, session)
				return answer, handled, nil
			}
		}
	}
	return "", false, nil
}

func (a *Agent) runPlannedAgent(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, error) {
	a.history.Add(userID, "user", text)
	if onEvent != nil {
		onEvent(StreamEventPlanning, a.planningStatusText(lang))
	}

	requestStartedAt := time.Now()
	state, err := a.prepareExecutionState(ctx, storeUserID, userID, lang, text)
	if err != nil {
		a.logPlannerTiming("", userID, "prepare_execution_state", requestStartedAt, err)
		if isPlannerTimeoutError(err) {
			msg := plannerTimeoutMessage(lang)
			if onEvent != nil {
				onEvent(StreamEventError, msg)
				emitStreamText(onEvent, msg)
			}
			return msg, nil
		}
		a.logger.Warn("planner failed, falling back to legacy loop", "error", err, "user_id", userID)
		return a.thinkAndActLegacyWithStore(ctx, storeUserID, userID, lang, text, onEvent)
	}
	a.logPlannerTiming(state.SessionID, userID, "prepare_execution_state", requestStartedAt, nil)

	executionStartedAt := time.Now()
	answer, err := a.executePlan(ctx, storeUserID, userID, lang, &state, onEvent)
	a.logPlannerTiming(state.SessionID, userID, "execute_plan", executionStartedAt, err)
	if err != nil {
		if isPlannerTimeoutError(err) {
			msg := plannerTimeoutMessage(lang)
			if onEvent != nil {
				onEvent(StreamEventError, msg)
				emitStreamText(onEvent, msg)
			}
			return msg, nil
		}
		if answer, ok := a.tryExecutionSummaryFallbackOnAIError(lang, &state, err, onEvent); ok {
			return answer, nil
		}
		a.logger.Warn("plan execution failed, falling back to legacy loop", "error", err, "user_id", userID)
		return a.thinkAndActLegacyWithStore(ctx, storeUserID, userID, lang, text, onEvent)
	}

	a.history.Add(userID, "assistant", answer)
	a.maybeUpdateTaskStateIncrementally(ctx, userID)
	a.maybeCompressHistory(ctx, userID)
	a.logPlannerTiming(state.SessionID, userID, "run_planned_agent_total", requestStartedAt, nil)
	return answer, nil
}

func (a *Agent) prepareExecutionState(ctx context.Context, storeUserID string, userID int64, lang, text string) (ExecutionState, error) {
	existing := a.getExecutionState(userID)
	if shouldResetExecutionStateForNewAttempt(text, existing) {
		a.clearExecutionState(userID)
		existing = ExecutionState{}
	}
	if existing.Status == executionStatusWaitingUser && existing.SessionID != "" {
		a.refreshCurrentReferencesForUserText(storeUserID, text, &existing)
		askedQuestion := latestAskedQuestion(existing)
		replySummary := strings.TrimSpace(text)
		if askedQuestion != "" {
			replySummary = fmt.Sprintf("Answer to previous question [%s]: %s", askedQuestion, replySummary)
		}
		appendExecutionLog(&existing, Observation{
			Kind:      "user_reply",
			Summary:   replySummary,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
		existing.Status = executionStatusPlanning
		existing.Waiting = nil
		existing.FinalAnswer = ""
		existing.LastError = ""
		existing = a.refreshStateForDynamicRequests(storeUserID, text, existing)
		existing.Steps = completedSteps(existing.Steps)
		existing.CurrentStepID = ""
		existing.Status = executionStatusRunning
		existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := a.saveExecutionState(existing); err != nil {
			return ExecutionState{}, err
		}
		return existing, nil
	}

	state := newExecutionState(userID, text)
	if mem := a.getReferenceMemory(userID); mem.CurrentReferences != nil {
		state.CurrentReferences = mem.CurrentReferences
		state.ReferenceHistory = mem.ReferenceHistory
	}
	a.refreshCurrentReferencesForUserText(storeUserID, text, &state)
	state = a.refreshStateForDynamicRequests(storeUserID, text, state)
	state.Status = executionStatusRunning
	if err := a.saveExecutionState(state); err != nil {
		return ExecutionState{}, err
	}
	return state, nil
}

type nextStepDecision struct {
	Goal  string     `json:"goal"`
	Steps []PlanStep `json:"steps,omitempty"`
	Step  PlanStep   `json:"step"`
}

func (a *Agent) decideNextStep(ctx context.Context, userID int64, lang string, state ExecutionState) (nextStepDecision, error) {
	toolDefs, _ := json.Marshal(agentTools())
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	recentlyFetchedJSON, _ := json.Marshal(buildRecentlyFetchedData(state, time.Now().UTC()))
	currentTurnCtx := a.buildCurrentTurnContext(userID, lang, state.Goal)
	activeTaskCtx := a.buildActiveTaskStateContext(userID, lang)

	systemPrompt := `You are the step selector for NOFXi.
Return JSON only. Do not return markdown.

You are operating in ReAct mode: Thought -> Action -> Observation.
Choose the immediate next action batch. Do not generate a long multi-step execution plan.

Allowed step types:
- tool
- reason
- ask_user
- respond

Rules:
- Use Current turn context and Active task state as the main conversational memory.
- Use Observations JSON as the source of truth for what tools already revealed in this execution.
- Use Recently fetched data JSON as the deduplication source of truth for fresh tool results.
- Prefer the freshest evidence in this order: observations, current turn context, active task state.
- If fresh external or system data is needed, choose a tool step.
- If the user is blocked on a missing parameter, choose ask_user.
- If there is enough information to answer now, choose respond.
- Use reason only when a short intermediate synthesis is necessary before the next action.
- Prefer tool or respond over reason whenever possible.
- Never emit the same reason step twice in a row.
- After a reason step, the next batch should usually be tool, ask_user, or respond. Do not stay in analysis loops.
- Never invent tools.
- If the task needs multiple independent tool reads, emit ALL of them together in one response.
- Parallelism rule: when multiple tool reads are mutually independent, do not split them across turns. Return them together in steps.
- Never mix ask_user/respond with additional steps in the same batch.
- Only emit multiple steps when every emitted step is a tool step.
- Avoid repeated tool calls. If a matching tool call already exists in Recently fetched data and age_seconds <= 60, do not call it again unless the user explicitly asks to refresh.
- For tool steps, set tool_name exactly to one available tool and provide tool_args as a JSON object.
- For ask_user or respond steps, put the user-facing question/response instruction in instruction.
- If the latest observation already answers the goal, prefer respond over another tool call.
- Never place a trade unless the user intent is explicit.

Return JSON with this exact shape:
{"goal":"","steps":[{"id":"step_1","type":"tool|reason|ask_user|respond","title":"","tool_name":"","tool_args":{},"instruction":"","requires_confirmation":false}]}`

	userPrompt := fmt.Sprintf("Language: %s\nGoal: %s\n\nCurrent turn context:\n%s\n\nActive task state:\n%s\n\nAvailable tools JSON:\n%s\n\nPersistent preferences:\n%s\n\nObservations JSON:\n%s\n\nRecently fetched data JSON:\n%s", lang, state.Goal, defaultIfEmpty(currentTurnCtx, "(empty)"), defaultIfEmpty(activeTaskCtx, "(empty)"), string(toolDefs), a.buildPersistentPreferencesContext(userID), string(obsJSON), string(recentlyFetchedJSON))

	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerCreateTimeout)
	defer cancel()

	startedAt := time.Now()
	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "decide_next_step_llm", startedAt, err)
	if err != nil {
		return nextStepDecision{}, err
	}
	return parseNextStepDecisionJSON(raw)
}

func parseNextStepDecisionJSON(raw string) (nextStepDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision nextStepDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeNextStepDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeNextStepDecision(decision), nil
		}
	}
	return nextStepDecision{}, fmt.Errorf("invalid next step decision json")
}

func normalizeNextStepDecision(decision nextStepDecision) nextStepDecision {
	decision.Goal = strings.TrimSpace(decision.Goal)
	steps := decision.Steps
	if len(steps) == 0 && decision.Step.Type != "" {
		steps = []PlanStep{decision.Step}
	}
	if len(steps) > 0 {
		steps = normalizeExecutionState(ExecutionState{Steps: steps}).Steps
	}
	decision.Steps = steps
	if len(steps) > 0 {
		decision.Step = steps[0]
	}
	return decision
}

func (a *Agent) refreshStateForDynamicRequests(storeUserID, userText string, state ExecutionState) ExecutionState {
	kinds := snapshotKindsForIntent(userText)
	if len(kinds) == 0 {
		return state
	}
	kindsToRefresh := make(map[string]struct{}, len(kinds))
	for _, kind := range kinds {
		kindsToRefresh[kind] = struct{}{}
	}

	fresh := make([]Observation, 0, len(state.DynamicSnapshots)+3)
	for _, obs := range state.DynamicSnapshots {
		if _, ok := kindsToRefresh[obs.Kind]; ok {
			continue
		}
		fresh = append(fresh, obs)
	}

	appendSnapshot := func(kind, raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		fresh = append(fresh, Observation{
			Kind:      kind,
			Summary:   summarizeObservation(raw),
			RawJSON:   raw,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	for _, kind := range kinds {
		switch kind {
		case "current_model_configs":
			appendSnapshot(kind, a.toolGetModelConfigs(storeUserID))
		case "current_exchange_configs":
			appendSnapshot(kind, a.toolGetExchangeConfigs(storeUserID))
		case "current_traders":
			appendSnapshot(kind, a.toolListTraders(storeUserID))
		case "current_strategies":
			appendSnapshot(kind, a.toolGetStrategies(storeUserID))
		case "current_balances":
			appendSnapshot(kind, a.toolGetBalance())
		case "current_positions":
			appendSnapshot(kind, a.toolGetPositions())
		case "recent_trade_history":
			appendSnapshot(kind, a.toolGetTradeHistory(`{"limit":10}`))
		}
	}
	state.DynamicSnapshots = fresh
	return state
}

func (a *Agent) buildRecentConversationContext(userID int64, currentUserText string) string {
	if a.history == nil {
		return ""
	}

	msgs := a.history.Get(userID)
	if len(msgs) == 0 {
		return ""
	}

	currentUserText = strings.TrimSpace(currentUserText)
	if currentUserText != "" {
		last := msgs[len(msgs)-1]
		if last.Role == "user" && strings.TrimSpace(last.Content) == currentUserText {
			msgs = msgs[:len(msgs)-1]
		}
	}

	if len(msgs) == 0 {
		return ""
	}
	if len(msgs) > recentConversationMessages {
		msgs = msgs[len(msgs)-recentConversationMessages:]
	}

	transcript := formatChatMessagesForSummary(msgs)
	if transcript == "" {
		return ""
	}
	return transcript
}

func (a *Agent) createExecutionPlan(ctx context.Context, userID int64, lang, userText string, state ExecutionState) (executionPlan, error) {
	toolDefs, _ := json.Marshal(agentTools())
	currentTurnCtx := a.buildCurrentTurnContext(userID, lang, userText)
	activeTaskCtx := a.buildActiveTaskStateContext(userID, lang)
	currentReferenceSummary := buildCurrentReferenceSummary(lang, a.semanticCurrentReferences(userID))
	skillContext := buildManagementSkillRoutingContext(lang)
	if isConfigOrTraderIntent(userText) {
		// Configuration and trader setup requests are especially sensitive to stale
		// durable summaries. Prefer the current turn context plus fresh tool checks.
		activeTaskCtx = ""
	}

	systemPrompt := prependNOFXiAdvisorPreamble(`You are the planning module for NOFXi.
Return JSON only. Do not return markdown.

Create a minimal safe execution plan using these step types only:
- tool
- reason
- ask_user
- respond

Rules:
- Use a compact memory layout when planning: Current reference summary, Current turn context, and Active task state.
- Memory priority order:
  1. Current reference summary = the currently locked entity/object memory for follow-up turns.
  2. Current turn context = the best source for what was just said, especially the last assistant reply and latest turns.
  3. Active task state = compact unfinished-task memory only.
- If these memory layers conflict, prefer current reference summary first for the target entity, then current turn context, then active task state.
- Do not ask the user to repeat a fact that is already explicit in current reference summary, current turn context, or active task state unless the inputs are contradictory.
- Use tool steps whenever fresh external data is required.
- Use ask_user if required parameters are missing.
- For config or create flows, prefer multi-slot ask_user prompts: ask for the main missing fields together instead of one field per turn whenever practical.
- When safe defaults are common and the user has not expressed a preference, offer those defaults in the same ask_user turn instead of forcing a separate follow-up for every slot.
- Never place a trade unless the user intent is explicit.
- For exchange binding or exchange credential requests, prefer get_exchange_configs/manage_exchange_config.
- For AI model binding or model credential requests, prefer get_model_configs/manage_model_config.
- For strategy template creation or editing requests, prefer get_strategies/manage_strategy.
- For trader creation or trader lifecycle requests, prefer manage_trader.
- A strategy template is independent and does not require exchange/model bindings unless the user explicitly asks to run or deploy it through a trader.
- Do NOT expand the goal beyond what the user explicitly requested. When the user's request is fulfilled, respond and stop. Do not proactively suggest or ask about the next logical step (e.g. do not ask "should I bind this to a trader?" after a strategy update unless the user asked for that).
- If these tools exist, never answer that the system lacks exchange/model/trader management capability.
- When configuration, strategy, or trader creation is requested, gather missing required fields via ask_user, then call the appropriate tool.
- Before concluding that exchange/model/trader/strategy setup is impossible or missing, first inspect current state with the relevant tools.
- For high-volatility state such as balances, positions, recent trade history, or current config availability, prefer fresh tool reads over old observations.
- Keep the plan short and practical.
- End with either ask_user or respond.
- At most 8 steps.
- For tool steps, set tool_name exactly to one of the available tool names and provide tool_args as JSON object.
- For reason steps, put the reasoning task in instruction.
- For ask_user steps, put the exact follow-up question in instruction.
- For respond steps, put either a short instruction or leave instruction empty.
- If resuming after a waiting_user state, incorporate the new user reply and return a fresh full plan.
- Never invent tools.`)

	resumeContext := ""
	if state.SessionID != "" {
		if askedQuestion := latestAskedQuestion(state); askedQuestion != "" {
			resumeContext = fmt.Sprintf("\n\nResume context:\n- The assistant was waiting for the user's answer to this exact question: %s\n- Interpret the new user message as the answer to that question unless the message clearly starts a new topic.", askedQuestion)
			if state.Waiting != nil {
				waitingJSON, _ := json.Marshal(state.Waiting)
				resumeContext += fmt.Sprintf("\n- Structured waiting state JSON: %s", string(waitingJSON))
			}
		}
	}

	userPrompt := fmt.Sprintf("Language: %s\nUser request: %s%s\n\n%s\n\nCurrent reference summary:\n%s\n\nCurrent turn context:\n%s\n\nActive task state:\n%s\n\nAvailable tools JSON:\n%s\n\nPersistent preferences:\n%s\n\nReturn JSON with this exact shape:\n{\"goal\":\"\",\"steps\":[{\"id\":\"step_1\",\"type\":\"tool|reason|ask_user|respond\",\"title\":\"\",\"tool_name\":\"\",\"tool_args\":{},\"instruction\":\"\",\"requires_confirmation\":false}]}", lang, userText, resumeContext, skillContext, currentReferenceSummary, defaultIfEmpty(currentTurnCtx, "(empty)"), defaultIfEmpty(activeTaskCtx, "(empty)"), string(toolDefs), a.buildPersistentPreferencesContext(userID))

	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerCreateTimeout)
	defer cancel()

	startedAt := time.Now()
	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "create_execution_plan_llm", startedAt, err)
	if err != nil {
		return executionPlan{}, err
	}

	plan, err := parseExecutionPlanJSON(resp)
	if err != nil {
		return executionPlan{}, err
	}
	if len(plan.Steps) == 0 {
		return executionPlan{}, fmt.Errorf("empty execution plan")
	}
	if len(plan.Steps) > plannerMaxSteps {
		plan.Steps = plan.Steps[:plannerMaxSteps]
	}
	for i := range plan.Steps {
		if plan.Steps[i].ID == "" {
			plan.Steps[i].ID = fmt.Sprintf("step_%d", i+1)
		}
		if plan.Steps[i].Status == "" {
			plan.Steps[i].Status = planStepStatusPending
		}
		if plan.Steps[i].Title == "" {
			plan.Steps[i].Title = strings.ReplaceAll(plan.Steps[i].ID, "_", " ")
		}
	}
	if strings.TrimSpace(plan.Goal) == "" {
		plan.Goal = strings.TrimSpace(userText)
	}
	return plan, nil
}

func parseExecutionPlanJSON(raw string) (executionPlan, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var plan executionPlan
	if err := json.Unmarshal([]byte(raw), &plan); err == nil {
		return plan, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &plan); err == nil {
			return plan, nil
		}
	}
	return executionPlan{}, fmt.Errorf("invalid execution plan json")
}

func (a *Agent) executePlan(ctx context.Context, storeUserID string, userID int64, lang string, state *ExecutionState, onEvent func(event, data string)) (string, error) {
	if onEvent != nil && len(state.Steps) > 0 {
		onEvent(StreamEventPlan, formatPlanStatus(*state, lang))
	}

	for i := 0; i < plannerMaxIterations; i++ {
		stepIndex := nextPendingStepIndex(state.Steps)
		if stepIndex < 0 {
			decisionStartedAt := time.Now()
			decision, err := a.decideNextStep(ctx, userID, lang, *state)
			a.logPlannerTiming(state.SessionID, userID, "decide_next_step", decisionStartedAt, err)
			if err != nil {
				return "", err
			}
			steps := filterFreshDuplicateToolSteps(decision.Steps, *state, time.Now().UTC())
			if len(steps) == 0 {
				return "", fmt.Errorf("all next steps are duplicate fresh tool calls")
			}
			if hasRepeatedReasonLoop(*state, steps) {
				return "", fmt.Errorf("repeated reasoning loop detected")
			}
			if decision.Goal != "" {
				state.Goal = decision.Goal
			}
			base := len(completedSteps(state.Steps))
			for idx := range steps {
				if steps[idx].Type == "" {
					return "", fmt.Errorf("next step decision missing step type")
				}
				if steps[idx].ID == "" {
					steps[idx].ID = fmt.Sprintf("step_%d", base+idx+1)
				}
				if steps[idx].Title == "" {
					steps[idx].Title = strings.ReplaceAll(steps[idx].ID, "_", " ")
				}
				if steps[idx].Status == "" {
					steps[idx].Status = planStepStatusPending
				}
			}
			state.Steps = append(completedSteps(state.Steps), steps...)
			state.Status = executionStatusRunning
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := a.saveExecutionState(*state); err != nil {
				return "", err
			}
			if onEvent != nil {
				onEvent(StreamEventPlan, formatPlanStatus(*state, lang))
			}
			continue
		}

		step := &state.Steps[stepIndex]
		step.Status = planStepStatusRunning
		state.Status = executionStatusRunning
		state.CurrentStepID = step.ID
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if onEvent != nil {
			onEvent(StreamEventStepStart, formatStepStatus(*step, stepIndex, len(state.Steps), lang))
		}
		if err := a.saveExecutionState(*state); err != nil {
			return "", err
		}

		switch step.Type {
		case planStepTypeTool:
			if onEvent != nil {
				onEvent(StreamEventTool, step.ToolName)
			}
			stepStartedAt := time.Now()
			result := a.executePlanTool(ctx, storeUserID, userID, lang, *step)
			a.logPlannerTiming(state.SessionID, userID, "tool:"+step.ToolName, stepStartedAt, nil)
			summary := summarizeObservation(result)
			referencesChanged := false
			step.Status = planStepStatusCompleted
			step.OutputSummary = summary
			appendExecutionLog(state, Observation{
				StepID:    step.ID,
				Kind:      "tool_result",
				Summary:   summary,
				RawJSON:   result,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
			referencesChanged = updateCurrentReferencesFromToolResult(state, step.ToolName, result)
			_ = referencesChanged
		case planStepTypeReason:
			reasonStartedAt := time.Now()
			reasoning, err := a.executeReasonStep(ctx, userID, lang, state.Goal, *state, *step)
			a.logPlannerTiming(state.SessionID, userID, "reason_step", reasonStartedAt, err)
			if err != nil {
				step.Status = planStepStatusFailed
				step.Error = err.Error()
				state.Status = executionStatusFailed
				state.LastError = err.Error()
				_ = a.saveExecutionState(*state)
				return "", err
			}
			step.Status = planStepStatusCompleted
			step.OutputSummary = reasoning
			appendExecutionLog(state, Observation{
				StepID:    step.ID,
				Kind:      "reasoning",
				Summary:   reasoning,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
		case planStepTypeAskUser:
			question := strings.TrimSpace(step.Instruction)
			if question == "" {
				if lang == "zh" {
					question = "我还缺少一些信息，麻烦你补充一下。"
				} else {
					question = "I need a bit more information before I continue."
				}
			}
			step.Status = planStepStatusCompleted
			step.OutputSummary = question
			state.Status = executionStatusWaitingUser
			state.Waiting = buildWaitingState(*state, *step, question)
			state.FinalAnswer = question
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := a.saveExecutionState(*state); err != nil {
				return "", err
			}
			if onEvent != nil {
				onEvent(StreamEventStepComplete, formatStepCompleteStatus(*step, lang))
				emitStreamText(onEvent, question)
			}
			return question, nil
		case planStepTypeRespond:
			respondStartedAt := time.Now()
			finalText, err := a.generateFinalPlanResponse(ctx, userID, lang, *state, step.Instruction)
			a.logPlannerTiming(state.SessionID, userID, "respond_step", respondStartedAt, err)
			if err != nil {
				return "", err
			}
			step.Status = planStepStatusCompleted
			step.OutputSummary = finalText
			state.Status = executionStatusCompleted
			state.Waiting = nil
			state.FinalAnswer = finalText
			state.CurrentStepID = ""
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := a.saveExecutionState(*state); err != nil {
				return "", err
			}
			if onEvent != nil {
				onEvent(StreamEventStepComplete, formatStepCompleteStatus(*step, lang))
				emitStreamText(onEvent, finalText)
			}
			return finalText, nil
		default:
			return "", fmt.Errorf("unsupported step type: %s", step.Type)
		}

		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := a.saveExecutionState(*state); err != nil {
			return "", err
		}
		if onEvent != nil {
			onEvent(StreamEventStepComplete, formatStepCompleteStatus(*step, lang))
		}
	}

	return "", fmt.Errorf("plan execution exceeded iteration limit")
}

type fetchedToolRecord struct {
	ToolName     string `json:"tool_name"`
	ToolArgsJSON string `json:"tool_args_json"`
	FetchedAt    string `json:"fetched_at"`
	AgeSeconds   int64  `json:"age_seconds"`
}

func buildRecentlyFetchedData(state ExecutionState, now time.Time) []fetchedToolRecord {
	state = normalizeExecutionState(state)
	stepByID := make(map[string]PlanStep, len(state.Steps))
	for _, step := range state.Steps {
		stepByID[step.ID] = step
	}
	latest := map[string]fetchedToolRecord{}
	for _, obs := range state.ExecutionLog {
		if obs.Kind != "tool_result" {
			continue
		}
		step, ok := stepByID[obs.StepID]
		if !ok || step.ToolName == "" {
			continue
		}
		sig := toolCallSignature(step.ToolName, step.ToolArgs)
		createdAt := parseRFC3339(obs.CreatedAt)
		record := fetchedToolRecord{
			ToolName:     step.ToolName,
			ToolArgsJSON: toolArgsJSONString(step.ToolArgs),
			FetchedAt:    obs.CreatedAt,
			AgeSeconds:   int64(now.Sub(createdAt).Seconds()),
		}
		prev, exists := latest[sig]
		if !exists || prev.FetchedAt < record.FetchedAt {
			latest[sig] = record
		}
	}
	out := make([]fetchedToolRecord, 0, len(latest))
	for _, record := range latest {
		if record.AgeSeconds < 0 {
			record.AgeSeconds = 0
		}
		out = append(out, record)
	}
	return out
}

func filterFreshDuplicateToolSteps(steps []PlanStep, state ExecutionState, now time.Time) []PlanStep {
	if len(steps) == 0 {
		return nil
	}
	fresh := make(map[string]struct{})
	for _, item := range buildRecentlyFetchedData(state, now) {
		if item.AgeSeconds <= 60 {
			fresh[item.ToolName+"|"+item.ToolArgsJSON] = struct{}{}
		}
	}
	out := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		if step.Type != planStepTypeTool {
			out = append(out, step)
			continue
		}
		sig := toolCallSignature(step.ToolName, step.ToolArgs)
		if _, ok := fresh[sig]; ok {
			continue
		}
		fresh[sig] = struct{}{}
		out = append(out, step)
	}
	return out
}

func hasRepeatedReasonLoop(state ExecutionState, steps []PlanStep) bool {
	if len(steps) == 0 {
		return false
	}
	last := lastCompletedStep(state.Steps)
	if last == nil || last.Type != planStepTypeReason {
		return false
	}
	for _, step := range steps {
		if step.Type != planStepTypeReason {
			return false
		}
		if stepSemanticKey(*last) != stepSemanticKey(step) {
			return false
		}
	}
	return true
}

func lastCompletedStep(steps []PlanStep) *PlanStep {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Status == planStepStatusCompleted {
			return &steps[i]
		}
	}
	return nil
}

func stepSemanticKey(step PlanStep) string {
	return strings.ToLower(strings.TrimSpace(
		step.Type + "|" + step.ToolName + "|" + step.Title + "|" + step.Instruction,
	))
}

func toolCallSignature(toolName string, args map[string]any) string {
	return strings.TrimSpace(toolName) + "|" + toolArgsJSONString(args)
}

func toolArgsJSONString(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func parseRFC3339(value string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return t
}

func (a *Agent) replanAfterStep(ctx context.Context, userID int64, lang string, state ExecutionState, completedStep PlanStep) (replannerDecision, error) {
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	stepsJSON, _ := json.Marshal(state.Steps)
	systemPrompt := prependNOFXiAdvisorPreamble(`You are the replanning module for NOFXi.
Return JSON only.

Decide what to do after a plan step completed.
Allowed actions:
- continue
- replace_remaining
- ask_user
- finish

Rules:
- Use continue when the current remaining steps still make sense.
- Use replace_remaining when the observations materially change the remaining plan.
- Use ask_user when execution is blocked on missing user input.
- Use finish when there is enough information to answer and remaining steps are unnecessary.
- If action=replace_remaining, return a fresh list of remaining steps only.
- Keep plans short and safe.
- Never invent tools.`)

	userPrompt := fmt.Sprintf("Language: %s\nGoal: %s\nCompleted step: %s (%s)\nCompleted summary: %s\n\nCurrent steps JSON:\n%s\n\nObservations JSON:\n%s\n\nPersistent preferences:\n%s\n\nTask state:\n%s\n\nReturn JSON with this exact shape:\n{\"action\":\"continue|replace_remaining|ask_user|finish\",\"goal\":\"\",\"instruction\":\"\",\"question\":\"\",\"steps\":[{\"id\":\"step_x\",\"type\":\"tool|reason|ask_user|respond\",\"title\":\"\",\"tool_name\":\"\",\"tool_args\":{},\"instruction\":\"\",\"requires_confirmation\":false}]}", lang, state.Goal, completedStep.ID, completedStep.Type, completedStep.OutputSummary, string(stepsJSON), string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))

	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerReplanTimeout)
	defer cancel()

	startedAt := time.Now()
	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx:       stageCtx,
		MaxTokens: intPtr(500),
	})
	a.logPlannerTiming(state.SessionID, userID, "replan_after_step_llm", startedAt, err)
	if err != nil {
		return replannerDecision{}, err
	}
	return parseReplannerDecisionJSON(raw)
}

func parseReplannerDecisionJSON(raw string) (replannerDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision replannerDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeReplannerDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeReplannerDecision(decision), nil
		}
	}
	return replannerDecision{}, fmt.Errorf("invalid replanner decision json")
}

func normalizeReplannerDecision(decision replannerDecision) replannerDecision {
	decision.Action = strings.TrimSpace(decision.Action)
	decision.Goal = strings.TrimSpace(decision.Goal)
	decision.Instruction = strings.TrimSpace(decision.Instruction)
	decision.Question = strings.TrimSpace(decision.Question)
	for i := range decision.Steps {
		if decision.Steps[i].ID == "" {
			decision.Steps[i].ID = fmt.Sprintf("step_%d", i+1)
		}
		if decision.Steps[i].Status == "" {
			decision.Steps[i].Status = planStepStatusPending
		}
		decision.Steps[i].Type = strings.TrimSpace(decision.Steps[i].Type)
		decision.Steps[i].Title = strings.TrimSpace(decision.Steps[i].Title)
		decision.Steps[i].ToolName = strings.TrimSpace(decision.Steps[i].ToolName)
		decision.Steps[i].Instruction = strings.TrimSpace(decision.Steps[i].Instruction)
	}
	return decision
}

func applyReplannerDecision(state *ExecutionState, decision replannerDecision) bool {
	switch decision.Action {
	case "", "continue":
		return false
	case "finish":
		state.Steps = append(completedSteps(state.Steps), PlanStep{
			ID:          fmt.Sprintf("step_finish_%d", time.Now().UTC().UnixNano()),
			Type:        planStepTypeRespond,
			Title:       "final response",
			Status:      planStepStatusPending,
			Instruction: decision.Instruction,
		})
		state.CurrentStepID = ""
		if decision.Goal != "" {
			state.Goal = decision.Goal
		}
		state.Waiting = nil
		return true
	case "ask_user":
		question := decision.Question
		if question == "" {
			question = decision.Instruction
		}
		state.Steps = append(completedSteps(state.Steps), PlanStep{
			ID:          fmt.Sprintf("step_ask_%d", time.Now().UTC().UnixNano()),
			Type:        planStepTypeAskUser,
			Title:       "need user input",
			Status:      planStepStatusPending,
			Instruction: question,
		})
		state.CurrentStepID = ""
		if decision.Goal != "" {
			state.Goal = decision.Goal
		}
		state.Waiting = buildWaitingState(*state, state.Steps[len(state.Steps)-1], question)
		return true
	case "replace_remaining":
		if len(decision.Steps) == 0 {
			return false
		}
		state.Steps = append(completedSteps(state.Steps), decision.Steps...)
		state.CurrentStepID = ""
		if decision.Goal != "" {
			state.Goal = decision.Goal
		}
		state.Waiting = nil
		return true
	default:
		return false
	}
}

func shouldAttemptReplan(state ExecutionState, step PlanStep, referencesChanged bool) bool {
	if step.Type != planStepTypeTool {
		return false
	}
	if toolResultIndicatesError(step.OutputSummary) || toolResultSignalsDependencyGap(step.OutputSummary) {
		return true
	}
	if referencesChanged {
		return true
	}
	if !hasPendingWorkAfterStep(state.Steps) {
		return false
	}
	switch step.ToolName {
	case "manage_trader", "manage_strategy", "manage_model_config", "manage_exchange_config", "execute_trade":
		return toolActionMayChangePlan(step.ToolArgs)
	default:
		return false
	}
}

func hasPendingWorkAfterStep(steps []PlanStep) bool {
	for _, step := range steps {
		if step.Status == planStepStatusPending {
			return true
		}
	}
	return false
}

func toolActionMayChangePlan(args map[string]any) bool {
	action, _ := args["action"].(string)
	switch strings.TrimSpace(action) {
	case "create", "update", "delete", "start", "stop", "activate", "duplicate":
		return true
	default:
		return false
	}
}

func toolResultIndicatesError(summary string) bool {
	lower := strings.ToLower(strings.TrimSpace(summary))
	return strings.Contains(lower, `"error"`) || strings.Contains(lower, `"status":"error"`) || strings.Contains(lower, "failed to ")
}

func toolResultSignalsDependencyGap(summary string) bool {
	lower := strings.ToLower(strings.TrimSpace(summary))
	patterns := []string{
		"is required", "invalid ai_model_id", "invalid exchange_id", "invalid strategy_id",
		"ai model is disabled", "exchange is disabled", "not found", "missing",
	}
	return containsAnyKeyword(lower, patterns)
}

func completedSteps(steps []PlanStep) []PlanStep {
	out := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		if step.Status == planStepStatusCompleted {
			out = append(out, step)
		}
	}
	return out
}

func (a *Agent) planningStatusText(lang string) string {
	if lang == "zh" {
		return "🧭 正在规划执行步骤..."
	}
	return "🧭 Planning the next execution steps..."
}

func formatPlanStatus(state ExecutionState, lang string) string {
	parts := make([]string, 0, len(state.Steps))
	for i, step := range state.Steps {
		label := step.Title
		if label == "" {
			label = step.Type
		}
		parts = append(parts, fmt.Sprintf("%d.%s", i+1, label))
	}
	if lang == "zh" {
		return fmt.Sprintf("🗺️ 计划: %s", strings.Join(parts, " -> "))
	}
	return fmt.Sprintf("🗺️ Plan: %s", strings.Join(parts, " -> "))
}

func formatStepStatus(step PlanStep, idx, total int, lang string) string {
	label := step.Title
	if label == "" {
		label = step.Type
	}
	if lang == "zh" {
		return fmt.Sprintf("▶️ 步骤 %d/%d: %s", idx+1, total, label)
	}
	return fmt.Sprintf("▶️ Step %d/%d: %s", idx+1, total, label)
}

func formatStepCompleteStatus(step PlanStep, lang string) string {
	label := step.Title
	if label == "" {
		label = step.Type
	}
	if lang == "zh" {
		return fmt.Sprintf("✅ 已完成: %s", label)
	}
	return fmt.Sprintf("✅ Completed: %s", label)
}

func formatReplanStatus(decision replannerDecision, lang string) string {
	switch decision.Action {
	case "replace_remaining":
		if lang == "zh" {
			return "🔄 已根据新结果更新后续步骤"
		}
		return "🔄 Updated the remaining steps based on new results"
	case "ask_user":
		if lang == "zh" {
			return "📝 当前流程需要用户补充信息"
		}
		return "📝 This flow needs more user input"
	case "finish":
		if lang == "zh" {
			return "🏁 已提前收敛到最终回复"
		}
		return "🏁 Converged early to the final response"
	default:
		if lang == "zh" {
			return "🔄 已重新评估计划"
		}
		return "🔄 Re-evaluated the plan"
	}
}

func (a *Agent) executePlanTool(ctx context.Context, storeUserID string, userID int64, lang string, step PlanStep) string {
	argsJSON := "{}"
	if len(step.ToolArgs) > 0 {
		if data, err := json.Marshal(step.ToolArgs); err == nil {
			argsJSON = string(data)
		}
	}
	return a.handleToolCall(ctx, storeUserID, userID, lang, mcp.ToolCall{
		ID:   step.ID,
		Type: "function",
		Function: mcp.ToolCallFunction{
			Name:      step.ToolName,
			Arguments: argsJSON,
		},
	})
}

func (a *Agent) executeReasonStep(ctx context.Context, userID int64, lang, goal string, state ExecutionState, step PlanStep) (string, error) {
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerReasonTimeout)
	defer cancel()

	startedAt := time.Now()
	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage("You are the reasoning module for NOFXi. Return one short paragraph only. No markdown, no bullet list."),
			mcp.NewUserMessage(fmt.Sprintf("Language: %s\nGoal: %s\nReasoning task: %s\nObservations JSON: %s\nPersistent preferences: %s\nTask state: %s", lang, goal, step.Instruction, string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "reason_step_llm", startedAt, err)
	if err != nil {
		return "", err
	}
	return summarizeObservation(resp), nil
}

func (a *Agent) generateFinalPlanResponse(ctx context.Context, userID int64, lang string, state ExecutionState, instruction string) (string, error) {
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	systemPrompt := a.buildSystemPrompt(lang)
	if instruction == "" {
		instruction = "Provide the best possible final response to the user based on the finished execution."
	}
	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerFinalTimeout)
	defer cancel()
	startedAt := time.Now()
	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewSystemMessage("You are responding after a completed execution plan. Use the observations as the source of truth. Be concise and actionable."),
			mcp.NewSystemMessage(cleanUserFacingReplyInstruction),
			mcp.NewUserMessage(fmt.Sprintf("Goal: %s\nResponse instruction: %s\nObservations JSON: %s\nPersistent preferences: %s\nTask state: %s", state.Goal, instruction, string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "generate_final_response_llm", startedAt, err)
	return resp, err
}

func (a *Agent) logPlannerTiming(sessionID string, userID int64, stage string, startedAt time.Time, err error) {
	if stage == "" || startedAt.IsZero() {
		return
	}
	attrs := []any{
		"session_id", sessionID,
		"user_id", userID,
		"stage", stage,
		"elapsed_ms", time.Since(startedAt).Milliseconds(),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	a.log().Info("planner timing", attrs...)
}

func nextPendingStepIndex(steps []PlanStep) int {
	for i := range steps {
		if steps[i].Status == "" || steps[i].Status == planStepStatusPending {
			return i
		}
	}
	return -1
}

func summarizeObservation(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= observationMaxLength {
		return value
	}
	return strings.TrimSpace(value[:observationMaxLength]) + "..."
}

func isAIServiceFailureError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "api returned error") ||
		strings.Contains(lower, "rate_limit_error") ||
		strings.Contains(lower, "upstream_empty_output") ||
		strings.Contains(lower, "insufficient balance") ||
		strings.Contains(lower, "context deadline exceeded")
}

func planStepFallbackLabel(step PlanStep) string {
	for _, candidate := range []string{
		strings.TrimSpace(step.Title),
		strings.TrimSpace(step.Instruction),
		strings.TrimSpace(step.ToolName),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return strings.TrimSpace(step.ID)
}

func formatCompletedPlanFallback(lang string, steps []PlanStep) string {
	labels := make([]string, 0, len(steps))
	for _, step := range steps {
		if label := planStepFallbackLabel(step); label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return ""
	}
	if lang == "zh" {
		lines := []string{"当前 AI 在最后生成总结时失败了，但我已经完成这些步骤："}
		for _, label := range labels {
			lines = append(lines, "- "+label)
		}
		lines = append(lines, "你现在可以先去页面确认结果；如果需要，我稍后可以继续补一版确认说明。")
		return strings.Join(lines, "\n")
	}
	lines := []string{"The AI failed while generating the final summary, but I already completed these steps:"}
	for _, label := range labels {
		lines = append(lines, "- "+label)
	}
	lines = append(lines, "You can verify the result in the UI now, and I can provide a follow-up confirmation summary later.")
	return strings.Join(lines, "\n")
}

func (a *Agent) tryExecutionSummaryFallbackOnAIError(lang string, state *ExecutionState, err error, onEvent func(event, data string)) (string, bool) {
	if a == nil || state == nil || !isAIServiceFailureError(err) {
		return "", false
	}
	completed := make([]PlanStep, 0, len(state.Steps))
	for _, step := range state.Steps {
		if step.Status == planStepStatusCompleted && step.Type == planStepTypeTool {
			completed = append(completed, step)
		}
	}
	if len(completed) == 0 {
		return "", false
	}
	answer := formatCompletedPlanFallback(lang, completed)
	if answer == "" {
		return "", false
	}
	currentStepID := state.CurrentStepID
	state.Status = executionStatusCompleted
	state.Waiting = nil
	state.FinalAnswer = answer
	state.LastError = strings.TrimSpace(err.Error())
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	for i := range state.Steps {
		if state.Steps[i].ID == currentStepID || (state.Steps[i].Status == planStepStatusRunning && state.Steps[i].Type == planStepTypeRespond) {
			state.Steps[i].Status = planStepStatusCompleted
			state.Steps[i].OutputSummary = answer
			state.Steps[i].Error = ""
		}
	}
	state.CurrentStepID = ""
	appendExecutionLog(state, Observation{
		Kind:      "respond_fallback",
		Summary:   summarizeObservation(answer),
		RawJSON:   err.Error(),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	_ = a.saveExecutionState(*state)
	if onEvent != nil {
		emitStreamText(onEvent, answer)
	}
	return answer, true
}

func (a *Agent) tryDeterministicFallbackAfterAIServiceFailure(ctx context.Context, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	storeUserID := storeUserIDFromContext(ctx)
	if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
		return a.maybeAppendResumePrompt(userID, lang, text, answer), true, nil
	}
	if state := a.getExecutionState(userID); hasActiveExecutionState(state) || len(state.Steps) > 0 {
		completed := make([]PlanStep, 0, len(state.Steps))
		for _, step := range state.Steps {
			if step.Status == planStepStatusCompleted && step.Type == planStepTypeTool {
				completed = append(completed, step)
			}
		}
		if answer := formatCompletedPlanFallback(lang, completed); answer != "" {
			return a.maybeAppendResumePrompt(userID, lang, text, answer), true, nil
		}
	}
	return "", false, nil
}

func (a *Agent) thinkAndActLegacy(ctx context.Context, userID int64, lang, text string, onEvent func(event, data string)) (string, error) {
	return a.thinkAndActLegacyWithStore(ctx, storeUserIDFromContext(ctx), userID, lang, text, onEvent)
}

func (a *Agent) thinkAndActLegacyWithStore(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, error) {
	systemPrompt := a.buildSystemPrompt(lang)
	enrichment := a.gatherContext(text)
	preferencesCtx := a.buildPersistentPreferencesContext(userID)

	userPrompt := text
	if preferencesCtx != "" {
		userPrompt = preferencesCtx + "\n\n---\n" + userPrompt
	}
	if enrichment != "" {
		userPrompt = text + "\n\n---\n[NOFXi System Context - real-time data for reference]\n" + enrichment
		if preferencesCtx != "" {
			userPrompt = preferencesCtx + "\n\n---\n" + userPrompt
		}
	}

	messages := []mcp.Message{mcp.NewSystemMessage(systemPrompt)}
	taskStateCtx := buildTaskStateContext(a.getTaskState(userID))
	if isConfigOrTraderIntent(text) {
		taskStateCtx = ""
	}
	if taskStateCtx != "" {
		messages = append(messages, mcp.NewSystemMessage(taskStateCtx))
	}
	history := a.history.Get(userID)
	if len(history) > 0 {
		history = history[:len(history)-1]
	}
	for _, msg := range history {
		messages = append(messages, mcp.NewMessage(msg.Role, msg.Content))
	}
	messages = append(messages, mcp.NewUserMessage(userPrompt))

	tools := agentTools()

	const maxToolRounds = 5
	for round := 0; round < maxToolRounds; round++ {
		req := &mcp.Request{
			Messages:   messages,
			Tools:      tools,
			ToolChoice: "auto",
			Ctx:        ctx,
		}

		resp, err := a.aiClient.CallWithRequestFull(req)
		if err != nil {
			if round == 0 {
				plainResp, plainErr := a.aiClient.CallWithRequest(&mcp.Request{Messages: messages, Ctx: ctx})
				if plainErr != nil {
					a.logger.Warn("legacy AI plain fallback failed", "error", plainErr, "user_id", userID)
					if answer, ok, fallbackErr := a.tryDeterministicFallbackAfterAIServiceFailure(ctx, userID, lang, text, onEvent); ok || fallbackErr != nil {
						return answer, fallbackErr
					}
					return a.aiServiceFailure(lang, plainErr)
				}
				if looksLikeInternalAgentJSON(plainResp) {
					a.logger.Warn("legacy AI plain fallback returned internal orchestration json; attempting active-flow recovery", "user_id", userID)
					if answer, ok, err := a.tryRecoverFromInternalAgentJSON(ctx, storeUserID, userID, lang, text, plainResp, onEvent); ok || err != nil {
						return answer, err
					}
					if answer, ok, fallbackErr := a.tryDeterministicFallbackAfterAIServiceFailure(ctx, userID, lang, text, onEvent); ok || fallbackErr != nil {
						return answer, fallbackErr
					}
					if lang == "zh" {
						return "我理解到你还在继续刚才的操作，但这次内部回复格式不对。你再说一次刚才想做的那一步，我继续接着帮你。", nil
					}
					return "I can tell you're continuing the previous task, but the internal response format was invalid. Please repeat that step and I'll keep going.", nil
				}
				if onEvent != nil {
					emitStreamText(onEvent, plainResp)
				}
				return plainResp, nil
			}
			a.logger.Warn("legacy AI tool round failed", "error", err, "user_id", userID, "round", round)
			if answer, ok, fallbackErr := a.tryDeterministicFallbackAfterAIServiceFailure(ctx, userID, lang, text, onEvent); ok || fallbackErr != nil {
				return answer, fallbackErr
			}
			return a.aiServiceFailure(lang, err)
		}

		if len(resp.ToolCalls) == 0 {
			if looksLikeInternalAgentJSON(resp.Content) {
				a.logger.Warn("legacy AI returned internal orchestration json; attempting active-flow recovery", "user_id", userID)
				if answer, ok, err := a.tryRecoverFromInternalAgentJSON(ctx, storeUserID, userID, lang, text, resp.Content, onEvent); ok || err != nil {
					return answer, err
				}
				if answer, ok, fallbackErr := a.tryDeterministicFallbackAfterAIServiceFailure(ctx, userID, lang, text, onEvent); ok || fallbackErr != nil {
					return answer, fallbackErr
				}
				if lang == "zh" {
					return "我理解到你还在继续刚才的操作，但这次内部回复格式不对。你再说一次刚才想做的那一步，我继续接着帮你。", nil
				}
				return "I can tell you're continuing the previous task, but the internal response format was invalid. Please repeat that step and I'll keep going.", nil
			}
			if onEvent != nil {
				emitStreamText(onEvent, resp.Content)
			}
			return resp.Content, nil
		}

		assistantMsg := mcp.Message{Role: "assistant", ToolCalls: resp.ToolCalls}
		if resp.Content != "" {
			assistantMsg.Content = resp.Content
		}
		messages = append(messages, assistantMsg)

		for _, tc := range resp.ToolCalls {
			if onEvent != nil {
				onEvent(StreamEventTool, tc.Function.Name)
			}
			result := a.handleToolCall(ctx, storeUserID, userID, lang, tc)
			messages = append(messages, mcp.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	finalResp, err := a.aiClient.CallWithRequest(&mcp.Request{Messages: messages, Ctx: ctx})
	if err != nil {
		a.logger.Warn("legacy AI final response failed", "error", err, "user_id", userID)
		if answer, ok, fallbackErr := a.tryDeterministicFallbackAfterAIServiceFailure(ctx, userID, lang, text, onEvent); ok || fallbackErr != nil {
			return answer, fallbackErr
		}
		return a.aiServiceFailure(lang, err)
	}
	if looksLikeInternalAgentJSON(finalResp) {
		a.logger.Warn("legacy AI final response returned internal orchestration json; attempting active-flow recovery", "user_id", userID)
		if answer, ok, err := a.tryRecoverFromInternalAgentJSON(ctx, storeUserID, userID, lang, text, finalResp, onEvent); ok || err != nil {
			return answer, err
		}
		if answer, ok, fallbackErr := a.tryDeterministicFallbackAfterAIServiceFailure(ctx, userID, lang, text, onEvent); ok || fallbackErr != nil {
			return answer, fallbackErr
		}
		if lang == "zh" {
			return "我理解到你还在继续刚才的操作，但这次内部回复格式不对。你再说一次刚才想做的那一步，我继续接着帮你。", nil
		}
		return "I can tell you're continuing the previous task, but the internal response format was invalid. Please repeat that step and I'll keep going.", nil
	}
	if onEvent != nil {
		emitStreamText(onEvent, finalResp)
	}
	return finalResp, nil
}
