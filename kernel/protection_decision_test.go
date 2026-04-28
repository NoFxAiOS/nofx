package kernel

import "testing"

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

func TestExtractDecisionsRepairsNumericThousandsSeparators(t *testing.T) {
	response := `<reasoning>test</reasoning><decision>[{"symbol":"BTCUSDT","action":"hold","confidence":45,"reason":"price near 76,500 resistance","entry_protection_rationale":{"risk_reward":{"entry":9,76887.05,"invalidation":98000,"first_target":93000}}}]</decision>`
	decisions, _, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decisions))
	}
	if decisions[0].EntryProtection == nil || decisions[0].EntryProtection.RiskReward.Entry != 976887.05 {
		t.Fatalf("expected malformed numeric separator repaired, got %#v", decisions[0].EntryProtection)
	}
}
