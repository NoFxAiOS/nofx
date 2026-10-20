package trader

import (
	"strings"
	"fmt"

	"nofx/kernel"
	"nofx/logger"
	"nofx/store"
	notify "nofx/telegram/notify"
)

// preTradeRuleCheck filters open decisions through the trader's rule system
// (extracted from past trade reviews). Hard rules with action=block reject the
// decision; warn rules log a warning. Soft lessons are logged for awareness.
// Returns the filtered decision list safe to execute.
//
// F5 (2026-10-01 review): covers BOTH market and resting-limit opens — the
// old open_long/open_short filter let open_*_limit bypass every review rule.
// Rule-load failures are FAIL-CLOSED for new risk: opens are dropped (with an
// alert) instead of silently skipping the check; closes/adjustments always
// pass.
func (at *AutoTrader) preTradeRuleCheck(decisions []kernel.Decision, equity float64) []kernel.Decision {
	return at.preTradeRuleCheckWithContext(decisions, equity, nil)
}

// preTradeRuleCheckWithContext is preTradeRuleCheck with the cycle context, so
// price-derived rule facts (stop_loss_pct / take_profit_pct / risk_reward) use
// the PROGRAM-determined final entry price (live price for market opens, limit
// anchor for resting limits) instead of the model's optional `price`
// (2026-10-20 area4 #5: a market decision that omitted price made
// "stop_loss_pct > N → block" silently evaluate against 0). When such a rule is
// enabled and no entry price can be determined, the open is refused (fail
// closed for new risk).
func (at *AutoTrader) preTradeRuleCheckWithContext(decisions []kernel.Decision, equity float64, ctx *kernel.Context) []kernel.Decision {
	if len(decisions) == 0 {
		return decisions
	}
	if at.store == nil {
		// No rule store wired → the review feedback loop cannot run → fail
		// closed for new risk (production traders always carry a store).
		return at.ruleCheckFailClosed(decisions, "rule store unavailable")
	}

	rules, err := at.store.Rule().GetEnabledRules(at.id)
	if err != nil {
		return at.ruleCheckFailClosed(decisions, fmt.Sprintf("rule load failed: %v", err))
	}
	if len(rules) == 0 {
		return decisions
	}

	needsPrice := rulesNeedEntryPrice(rules)
	filtered := make([]kernel.Decision, 0, len(decisions))
	for _, d := range decisions {
		if !kernel.IsOpenDecision(d.Action) {
			filtered = append(filtered, d)
			continue
		}

		entryPx := at.entryReferencePrice(&d, ctx)
		if entryPx <= 0 && needsPrice {
			logger.Warnf("🚫 [%s] RULE FACTS UNAVAILABLE — blocked %s %s: no program-determined entry price for price-based review rules", at.name, d.Action, d.Symbol)
			at.setFilterReason(d, "rule facts unavailable (no entry price)")
			continue
		}
		violations, warnings := kernel.CheckDecisionAgainstRules(rules, d, equity, entryPx)

		blocked := false
		blockReasons := []string{}
		for _, v := range violations {
			blocked = true
			blockReasons = append(blockReasons, v.RuleName)
			logger.Warnf("🚫 [%s] RULE BLOCKED %s %s: %s (%s) — %s",
				at.name, d.Action, d.Symbol, v.RuleName, v.Detail, v.Message)
			at.logRuleCheck(v, d, true)
			notify.Notify("ALERT", at.name, fmt.Sprintf("<b>🚫 规则拦截 %s %s</b>\n%s（%s）\n本次开仓已阻止",
				d.Action, notify.Escape(d.Symbol), notify.Escape(v.RuleName), notify.Escape(v.Detail)))
		}
		for _, w := range warnings {
			logger.Warnf("⚠️ [%s] RULE WARN %s %s: %s (%s) — %s",
				at.name, d.Action, d.Symbol, w.RuleName, w.Detail, w.Message)
			at.logRuleCheck(w, d, false)
		}

		// Soft lessons relevant to this decision (awareness log only)
		for _, s := range kernel.MatchSoftRules(rules, d) {
			logger.Infof("📚 [%s] LESSON %s %s: %s", at.name, d.Action, d.Symbol, s.Message)
		}

		if blocked {
			at.setFilterReason(d, "trading rule: "+strings.Join(blockReasons, "+"))
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered
}

// ruleCheckFailClosed drops every NEW-RISK decision when the rule system
// itself is unavailable (F5, 2026-10-01 review: a fail-open skip silently
// voided the review → rule → execution feedback loop). Closes/adjustments
// always pass — the account must be able to de-risk.
func (at *AutoTrader) ruleCheckFailClosed(decisions []kernel.Decision, cause string) []kernel.Decision {
	filtered := make([]kernel.Decision, 0, len(decisions))
	dropped := 0
	for _, d := range decisions {
		if kernel.IsOpenDecision(d.Action) {
			dropped++
			logger.Warnf("🚫 [%s] RULE SYSTEM UNAVAILABLE — blocked %s %s (%s)", at.name, d.Action, d.Symbol, cause)
			at.setFilterReason(d, "rule system unavailable ("+cause+")")
			continue
		}
		filtered = append(filtered, d)
	}
	if dropped > 0 {
		notify.Notify("ALERT", at.name, fmt.Sprintf(
			"<b>🚨 复盘规则系统不可用 — 停止新增风险</b>\n<i>%s</i>\n本轮 <code>%d</code> 个开仓决策被保守拦截(平仓/调整不受影响),请检查规则存储",
			notify.Escape(cause), dropped))
	}
	return filtered
}

// logRuleCheck persists a fired rule and increments its hit counter
func (at *AutoTrader) logRuleCheck(v kernel.RuleViolation, d kernel.Decision, blocked bool) {
	if at.store == nil {
		return
	}
	log := &store.RuleCheckLogDB{
		TraderID: at.id,
		RuleID:   v.RuleID,
		RuleName: v.RuleName,
		Action:   d.Action,
		Symbol:   d.Symbol,
		Violated: true,
		Blocked:  blocked,
		Message:  fmt.Sprintf("%s | %s", v.Detail, v.Message),
	}
	if err := at.store.Rule().LogCheck(log); err != nil {
		logger.Warnf("⚠️ [%s] Failed to log rule check: %v", at.name, err)
	}
	if err := at.store.Rule().IncrementHitCount(v.RuleID, blocked); err != nil {
		logger.Warnf("⚠️ [%s] Failed to update rule hit count: %v", at.name, err)
	}
}

// rulesNeedEntryPrice reports whether any enabled hard rule reads a fact that
// is derived from the entry price.
func rulesNeedEntryPrice(rules []*store.TradingRuleDB) bool {
	for _, r := range rules {
		if r.RuleType != "hard" {
			continue
		}
		cond, err := kernel.ParseRuleCondition(r.ConditionJSON)
		if err != nil {
			continue // invalid rules already fail on their own
		}
		switch cond.Field {
		case "stop_loss_pct", "take_profit_pct", "risk_reward":
			return true
		}
	}
	return false
}
