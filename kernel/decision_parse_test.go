package kernel

import (
	"strings"
	"testing"
)

func TestEmptyArrayRegex(t *testing.T) {
	cases := []struct {
		name  string
		fence string
		raw   string
	}{
		{"empty fenced", "fence", "```json [] ```"},
		{"empty fenced multiline", "fence", "```json\n[]\n```"},
		{"one item fenced", "fence", "```json [{\"symbol\":\"BTCUSDT\",\"action\":\"wait\"}] ```"},
		{"two items fenced", "fence", "```json\n[{\"symbol\":\"A\",\"action\":\"wait\"},{\"symbol\":\"B\",\"action\":\"wait\"}]\n```"},
		{"empty raw", "raw", "decide: []"},
		{"one item raw", "raw", "decide: [{\"symbol\":\"BTCUSDT\",\"action\":\"wait\"}]"},
	}
	for _, tc := range cases {
		s := removeInvisibleRunes(tc.raw)
		if tc.fence == "fence" {
			m := reJSONFence.FindStringSubmatch(s)
			if m == nil {
				t.Errorf("%s: fence regex no match", tc.name)
				continue
			}
			if err := validateJSONFormat(m[1]); err != nil {
				t.Errorf("%s: validate failed: %v", tc.name, err)
			}
		} else {
			got := reJSONArray.FindString(s)
			if got == "" {
				t.Errorf("%s: array regex no match", tc.name)
				continue
			}
			if err := validateJSONFormat(got); err != nil {
				t.Errorf("%s: validate failed: %v", tc.name, err)
			}
		}
	}
}

// Reproduces the user's exact scenario: reasoning-model output ends with an
// empty decision array and must NOT enter safe wait.
func TestExtractDecisionsAcceptsEmptyArray(t *testing.T) {
	resp := "Let me analyze this carefully... BTR overextended, XLM flat. Output: empty decision array.\n```json [] ```"

	decisions, err := extractDecisions(resp)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions, got %d", len(decisions))
	}
	// No safe-wait ALL decision may be synthesized
	for _, d := range decisions {
		if d.Symbol == "ALL" {
			t.Fatalf("safe-wait ALL decision synthesized for valid empty array")
		}
	}
}

// Full decision parse path with reasoning text around the empty array.
func TestParseFullDecisionResponseWithEmptyArray(t *testing.T) {
	resp := "analysis preamble\n```json [] ```"
	fd, err := parseFullDecisionResponse(resp, 1000, 10, 5, 0.5, 0.5, 12, nil, nil, nil)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(fd.Decisions) != 0 {
		t.Fatalf("expected empty decisions, got %d", len(fd.Decisions))
	}
	if !strings.Contains(fd.CoTTrace, "analysis preamble") {
		t.Fatalf("CoT trace lost: %q", fd.CoTTrace)
	}
}

// Reproduces the user's exact custom-model failure: valid decision with a
// "?" placeholder for an unknown risk_usd value.
func TestExtractDecisionsPlaceholderValue(t *testing.T) {
	resp := "```json [{\"symbol\":\"INJUSDT\",\"action\":\"open_long\",\"leverage\":5,\"position_size_usd\":255,\"stop_loss\":5.20,\"take_profit\":5.62,\"confidence\":80,\"risk_usd\":?}] ```"

	decisions, err := extractDecisions(resp)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	d := decisions[0]
	if d.Symbol != "INJUSDT" || d.Action != "open_long" || d.Confidence != 80 {
		t.Fatalf("decision fields lost: %+v", d)
	}
	if d.StopLoss != 5.20 || d.TakeProfit != 5.62 {
		t.Fatalf("numeric fields lost: %+v", d)
	}
}

// Reproduces the live failure mode (09-08/09-09, 25 safe-waits): a
// max-token-truncated response whose decision array was cut mid-object.
// The completed objects before the cut must be salvaged instead of
// discarding everything to safe-wait.
func TestExtractDecisionsSalvagesTruncatedArray(t *testing.T) {
	// Mirrors the real 2026-09-08 21:55 record: reasoning, then a decision
	// array that dies mid-object on the last coin.
	resp := "<reasoning>\nCL 持仓检查: exit_rule_triggered=true, 平仓。\n候选币逐个评估...\n</reasoning>\n<decision>\n[\n  {\"symbol\": \"CLUSDT\", \"action\": \"close_long\", \"decision_stage\": \"EXIT\", \"reasoning\": \"多头结构转弱\"},\n  {\"symbol\": \"BULLAUSDT\", \"action\": \"wait\", \"decision_stage\": \"NO_SETUP\", \"no_trade_reason\": [\"15m偏空但1h/4h方向分歧\", \"做空限价被抑制\", \"无突破或反转确认\""
	// note: deliberately NO closing "}" "]" anywhere after the first object

	// review 2026-10-20 area4 #3: salvage is limited to the final <decision>
	// payload; drafts inside <reasoning> can never be picked up.
	decisions, err := extractDecisions(resp)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Symbol != "CLUSDT" || decisions[0].Action != "close_long" {
		t.Fatalf("expected 1 salvaged decision, got %+v", decisions)
	}
}

// Salvage still applies to a truncated bare array (no <decision> tag).
func TestExtractDecisionsSalvagesTruncatedBareArray(t *testing.T) {
	resp := "分析...\n[\n  {\"symbol\": \"CLUSDT\", \"action\": \"close_long\", \"reasoning\": \"多头结构转弱\"},\n  {\"symbol\": \"BULLAUSDT\", \"action\": \"wait\", \"no_trade_reason\": [\"15m偏空"
	decisions, err := extractDecisions(resp)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Symbol != "CLUSDT" || decisions[0].Action != "close_long" {
		t.Fatalf("expected 1 salvaged decision, got %+v", decisions)
	}
}

// The salvage path must NOT fire when the response contains no complete
// decision object (or the array is malformed beyond repair) — it falls
// through to the safe-wait fallback.
func TestExtractDecisionsTruncatedWithNothingComplete(t *testing.T) {
	resp := "分析中...\n[{\"symbol\": \"DOTUSDT\", \"action\": \"wait\", \"no_trade_reason\": [\"15m为range且5m转弱,execution_filter无方向\",\"1h/4h结构仍"

	decisions, err := extractDecisions(resp)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Symbol != "ALL" || decisions[0].Action != "wait" {
		t.Fatalf("expected safe-wait ALL fallback, got: %+v", decisions)
	}
}

// Escaped quotes inside string values must not confuse the brace-depth scan.
func TestSalvageHandlesEscapedQuotes(t *testing.T) {
	resp := "[{\"symbol\": \"AUSDT\", \"action\": \"wait\", \"reasoning\": \"价格 \\\"贴近\\\" 前高\"},\n  {\"symbol\": \"BUSDT\", \"actio"

	decisions, err := extractDecisions(resp)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Symbol != "AUSDT" {
		t.Fatalf("expected 1 salvaged decision with escaped quotes, got: %+v", decisions)
	}
}

// Common placeholder variants must all be tolerated.
func TestExtractDecisionsPlaceholderVariants(t *testing.T) {
	variants := []string{
		`[{"symbol":"BTCUSDT","action":"wait","risk_usd":?}]`,
		`[{"symbol":"BTCUSDT","action":"wait","risk_usd":??}]`,
		`[{"symbol":"BTCUSDT","action":"wait","risk_usd":？}]`,
		`[{"symbol":"BTCUSDT","action":"wait","risk_usd":"N/A"}]`,
		`[{"symbol":"BTCUSDT","action":"wait","confidence":75,}]`,
	}
	for i, v := range variants {
		decisions, err := extractDecisions(v)
		if err != nil {
			t.Errorf("variant %d (%s): %v", i, v, err)
			continue
		}
		if len(decisions) != 1 {
			t.Errorf("variant %d: expected 1 decision, got %d", i, len(decisions))
		}
	}
}
