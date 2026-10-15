package stockengine

import (
	"encoding/json"
	"fmt"
	"nofx/mcp"
	"strings"
	"time"
)

func validAction(action string) bool {
	switch action {
	case ActionOpenLong, ActionAddLong, ActionReduceLong, ActionCloseLong, ActionAdjustStop, ActionHold, ActionWait:
		return true
	}
	return false
}

// normalizeDecisions upper-cases symbols, lower-cases action/entry_type, trims the
// free-text thesis fields and drops decisions with an unknown action.
func normalizeDecisions(found []Decision) []Decision {
	decisions := make([]Decision, 0, len(found))
	for _, d := range found {
		d.Symbol = strings.ToUpper(strings.TrimSpace(d.Symbol))
		d.Action = strings.ToLower(strings.TrimSpace(d.Action))
		d.EntryType = strings.ToLower(strings.TrimSpace(d.EntryType))
		d.Thesis = strings.TrimSpace(d.Thesis)
		d.Invalidation = strings.TrimSpace(d.Invalidation)
		if validAction(d.Action) {
			decisions = append(decisions, d)
		}
	}
	return decisions
}

// ParseDecisions selects the last array that decodes as []Decision. Scanning
// with json.Decoder handles brackets in strings, fences, and trailing prose.
func ParseDecisions(response string) ([]Decision, error) {
	var found []Decision
	ok := false
	for i := 0; i < len(response); i++ {
		if response[i] != '[' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(response[i:]))
		var candidate []Decision
		if err := decoder.Decode(&candidate); err != nil {
			continue
		}
		found, ok = candidate, true
		// Do not treat nested arrays inside this successfully decoded array as decisions.
		i += int(decoder.InputOffset()) - 1
	}
	if !ok {
		return nil, fmt.Errorf("no JSON decision array parses in response")
	}
	return normalizeDecisions(found), nil
}

// responseObject is the strict output contract: reasoning, market_context and
// decisions. Decisions is a pointer so an object without the key is not mistaken
// for a response.
type responseObject struct {
	Reasoning     string         `json:"reasoning"`
	MarketContext *MarketContext `json:"market_context"`
	Decisions     *[]Decision    `json:"decisions"`
}

// ParseResponse extracts the model's answer. It prefers the LAST top-level JSON
// object that carries a "decisions" array (reasoning and market_context come from
// the same object) and falls back to the legacy bare decision array, for which
// reasoning and market context are empty. Fences and prose around the JSON, and
// brackets/braces inside strings, are handled by scanning with json.Decoder.
func ParseResponse(response string) (reasoning string, mc *MarketContext, decisions []Decision, err error) {
	var found *responseObject
	for i := 0; i < len(response); i++ {
		if response[i] != '{' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(response[i:]))
		var candidate responseObject
		if decodeErr := decoder.Decode(&candidate); decodeErr != nil || candidate.Decisions == nil {
			continue
		}
		c := candidate
		found = &c
		// Objects nested inside this response are not responses themselves.
		i += int(decoder.InputOffset()) - 1
	}
	if found == nil {
		decisions, err = ParseDecisions(response)
		if err != nil {
			return "", nil, nil, fmt.Errorf("no JSON decision object or array parses in response")
		}
		return "", nil, decisions, nil
	}
	if mc = found.MarketContext; mc != nil {
		mc.OverallSentiment = strings.ToLower(strings.TrimSpace(mc.OverallSentiment))
	}
	return strings.TrimSpace(found.Reasoning), mc, normalizeDecisions(*found.Decisions), nil
}

// repairable collects the indexes of rejected open/add verdicts the model can
// fix: every code on the verdict must be in RepairableCodes.
func repairable(verdicts []Verdict) []int {
	var idx []int
	for i, v := range verdicts {
		d := v.Decision
		if v.Accepted || len(v.Codes) == 0 || (d.Action != ActionOpenLong && d.Action != ActionAddLong) {
			continue
		}
		all := true
		for _, c := range v.Codes {
			if !RepairableCodes[c] {
				all = false
				break
			}
		}
		if all {
			idx = append(idx, i)
		}
	}
	return idx
}

func isZH(lang string) bool { return strings.HasPrefix(strings.ToLower(lang), "zh") }

// buildRepairPrompt appends the single repair request to the original user prompt.
func buildRepairPrompt(userPrompt, rawResponse string, verdicts []Verdict, idx []int, lang string) string {
	var b strings.Builder
	b.WriteString(userPrompt)
	zh := isZH(lang)
	if zh {
		b.WriteString("\n\n## 修复请求（仅此一次）\n你之前的回复：\n")
	} else {
		b.WriteString("\n\n## REPAIR REQUEST (one attempt)\nYour previous response:\n")
	}
	b.WriteString(rawResponse)
	if zh {
		b.WriteString("\n程序拒绝了以下入场决策：\n")
	} else {
		b.WriteString("\nThe program rejected these entry decisions:\n")
	}
	for _, i := range idx {
		v := verdicts[i]
		fmt.Fprintf(&b, "- %s %s: %s", v.Decision.Symbol, v.Decision.Action, strings.Join(v.Codes, ", "))
		if v.Note != "" {
			fmt.Fprintf(&b, " (%s)", v.Note)
		}
		b.WriteString("\n")
	}
	if zh {
		b.WriteString("请返回相同的 JSON 对象格式，decisions 中只包含这些标的，并修正为满足程序的所有规则——如果不再想入场，则对该标的输出 action \"wait\"。不要包含其他标的。")
	} else {
		b.WriteString("Return the SAME JSON object format with decisions ONLY for these symbols, corrected to satisfy every program rule — or action \"wait\" for any you no longer want. Do not include other symbols.")
	}
	return b.String()
}

// repairEntries runs the one repair attempt on r (already holding the first-round
// verdicts). It never fails the cycle: on any error the first-round verdicts are
// kept and RepairRawResponse carries the error text.
func repairEntries(ctx *Context, client mcp.AIClient, lang string, r *Result) {
	idx := repairable(r.Verdicts)
	if len(idx) == 0 {
		return
	}
	r.RepairPrompt = buildRepairPrompt(r.UserPrompt, r.RawResponse, r.Verdicts, idx, lang)
	targets := map[string]bool{}
	for _, i := range idx {
		sym := r.Verdicts[i].Decision.Symbol
		targets[sym] = true
		r.Repaired = append(r.Repaired, sym)
	}
	start := time.Now()
	response, err := client.CallWithMessages(r.SystemPrompt, r.RepairPrompt)
	r.RepairDurationMs = time.Since(start).Milliseconds()
	if err != nil {
		r.RepairRawResponse = "repair call failed: " + err.Error()
		return
	}
	r.RepairRawResponse = response
	_, _, fixes, err := ParseResponse(response)
	if err != nil {
		r.RepairRawResponse = response + "\n[repair parse failed: " + err.Error() + "]"
		return
	}
	replacement := map[string]Decision{}
	for _, d := range fixes {
		if _, dup := replacement[d.Symbol]; targets[d.Symbol] && !dup {
			replacement[d.Symbol] = d
		}
	}
	merged := make([]Decision, len(r.Decisions))
	copy(merged, r.Decisions)
	for _, i := range idx {
		if d, ok := replacement[merged[i].Symbol]; ok {
			merged[i] = d
		}
	}
	r.Decisions = merged
	r.Verdicts = Validate(ctx, merged)
}

// Decide builds prompts, invokes the injected client, parses, and validates.
// When open/add decisions are rejected for a fixable reason it makes one repair
// call and re-validates. Only the client calls use wall-clock timing; all
// trading time comes from ctx.Now.
func Decide(ctx *Context, client mcp.AIClient, lang string) (*Result, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil stock context")
	}
	r := &Result{SystemPrompt: BuildSystemPrompt(ctx.Config, contextPreset(ctx), lang), UserPrompt: BuildUserPrompt(ctx, lang)}
	if client == nil {
		return r, fmt.Errorf("nil AI client")
	}
	start := time.Now()
	response, err := client.CallWithMessages(r.SystemPrompt, r.UserPrompt)
	r.DurationMs = time.Since(start).Milliseconds()
	r.RawResponse = response
	if err != nil {
		return r, fmt.Errorf("AI call failed: %w", err)
	}
	r.Reasoning, r.MarketContext, r.Decisions, err = ParseResponse(response)
	if err != nil {
		return r, err
	}
	r.Verdicts = Validate(ctx, r.Decisions)
	repairEntries(ctx, client, lang, r)
	return r, nil
}
