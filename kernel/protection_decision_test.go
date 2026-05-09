package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestExtractDecisionsPreservesProtectionPlan(t *testing.T) {
	response := `[
	  {
	    "symbol": "BTCUSDT",
	    "action": "open_long",
	    "leverage": 3,
	    "position_size_usd": 100,
	    "confidence": 85,
	    "risk_usd": 5,
	    "reasoning": "test",
	    "protection_plan": {
	      "mode": "ladder",
	      "ladder_rules": [
	        {
	          "take_profit_pct": 3,
	          "take_profit_close_ratio_pct": 40,
	          "stop_loss_pct": 1.5,
	          "stop_loss_close_ratio_pct": 25
	        }
	      ]
	    }
	  }
	]`

	decisions, _, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].ProtectionPlan == nil {
		t.Fatal("expected protection_plan to be preserved")
	}
	if decisions[0].ProtectionPlan.Mode != "ladder" || len(decisions[0].ProtectionPlan.LadderRules) != 1 {
		t.Fatalf("unexpected protection_plan: %+v", decisions[0].ProtectionPlan)
	}
}

func TestValidateDecisionFormatAcceptsCurrentActionSet(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
	}}

	if err := ValidateDecisionFormat(decisions); err != nil {
		t.Fatalf("expected current action set to validate, got %v", err)
	}
}

func TestExtractDecisionsAllowsCommaSeparatedPricesInsideReasonString(t *testing.T) {
	response := `<decision>[{"symbol":"BTCUSDT","action":"hold","confidence":61,"reasoning":"已有BTCUSDT空单,15分钟反弹仍主要受76754.19,76887.05斐波那契区域压制,尚未确认站上77019.91及77209.06关键结构阻力。"}]</decision>`
	decisions, _, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || !strings.Contains(decisions[0].Reasoning, "76754.19,76887.05") {
		t.Fatalf("expected reason string preserved, got %#v", decisions)
	}
}

func TestExtractDecisionsPreservesChineseCommaSeparatedPricesInsideReasonString(t *testing.T) {
	response := `<decision>[{"symbol":"ETHUSDT","action":"hold","confidence":28,"reasoning":"ETHUSDT 当前多单处于不利状态，15m 已跌破 2275.58、2268.11 斐波那契区域。"}]</decision>`
	decisions, _, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || !strings.Contains(decisions[0].Reasoning, "2275.58、2268.11") {
		t.Fatalf("expected Chinese punctuation inside reason string preserved, got %#v", decisions)
	}
}

func TestExtractDecisionsPreservesRangeSymbolInsideReasonString(t *testing.T) {
	response := `<decision>[{"symbol":"ZECUSDT","action":"wait","confidence":45,"reasoning":"ZEC今日经历从~350到393的大幅拉升，进场~383.4但RR不足。"}]</decision>`
	decisions, _, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions should allow ~ inside reason string, got %v", err)
	}
	if len(decisions) != 1 || !strings.Contains(decisions[0].Reasoning, "从~350到393") {
		t.Fatalf("expected reason string with ~ preserved, got %#v", decisions)
	}
}

func TestValidateJSONFormatRejectsRangeSymbolOutsideStrings(t *testing.T) {
	bad := `[{"symbol":"BTCUSDT","action":"hold","confidence":61,"entry":97~98}]`
	if err := validateJSONFormat(bad); err == nil {
		t.Fatal("expected range symbol outside string fields to be rejected")
	}
}

func TestValidateJSONFormatRejectsThousandsSeparatorsInNumericFields(t *testing.T) {
	bad := `[{"symbol":"BTCUSDT","action":"hold","confidence":61,"entry":97,687.05}]`
	if err := validateJSONFormat(bad); err == nil {
		t.Fatal("expected thousand separator in numeric field to be rejected")
	}
}

func TestSystemPromptForbidsThousandsSeparatorsInNumericFields(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&cfg)
	prompt := engine.BuildSystemPrompt(1000, "balanced")
	for _, want := range []string{"STRICT JSON NUMBER RULE", "Never use thousands separators", "Wrong: `97,687.05`"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q", want)
		}
	}
}
