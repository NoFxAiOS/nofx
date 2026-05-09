package api

import (
	"encoding/json"
	"testing"

	"nofx/kernel"
)

func TestMockAIResponsesProduceStructuredDecisionEnvelopes(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantMode  string
		wantCount int
	}{
		{
			name: "full protection open",
			raw: `[
			  {
			    "symbol": "BTCUSDT",
			    "action": "open_long",
			    "leverage": 3,
			    "position_size_usd": 500,
			    "confidence": 78,
			    "reasoning": "test",
			    "protection_plan": {"mode":"full","take_profit_pct":8,"stop_loss_pct":3}
			  }
			]`,
			wantMode:  "full",
			wantCount: 1,
		},
		{
			name: "ladder protection open",
			raw: `[
			  {
			    "symbol": "SOLUSDT",
			    "action": "open_short",
			    "leverage": 2,
			    "position_size_usd": 300,
			    "confidence": 80,
			    "reasoning": "test",
			    "protection_plan": {
			      "mode":"ladder",
			      "ladder_rules":[{"take_profit_pct":3,"take_profit_close_ratio_pct":40,"stop_loss_pct":1.5,"stop_loss_close_ratio_pct":25}]
			    }
			  }
			]`,
			wantMode:  "ladder",
			wantCount: 1,
		},
		{
			name: "close action without protection",
			raw: `[
			  {
			    "symbol": "ETHUSDT",
			    "action": "close_long",
			    "confidence": 82,
			    "reasoning": "test"
			  }
			]`,
			wantMode:  "",
			wantCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decisions, err := kernel.ParseAIDecisions(tc.raw)
			if err != nil {
				t.Fatalf("ParseAIDecisions failed: %v", err)
			}
			if err := kernel.ValidateAIDecisions(decisions); err != nil {
				t.Fatalf("ValidateAIDecisions failed: %v", err)
			}
			if len(decisions) != tc.wantCount {
				t.Fatalf("expected %d decisions, got %d", tc.wantCount, len(decisions))
			}
			if tc.wantMode == "" {
				if decisions[0].ProtectionPlan != nil {
					t.Fatalf("expected no protection_plan for close action, got %+v", decisions[0].ProtectionPlan)
				}
			} else {
				if decisions[0].ProtectionPlan == nil || decisions[0].ProtectionPlan.Mode != tc.wantMode {
					t.Fatalf("expected protection_plan mode %q, got %+v", tc.wantMode, decisions[0].ProtectionPlan)
				}
			}

			payload := map[string]any{
				"ai_response":      tc.raw,
				"parsed_decisions": decisions,
				"parse_error":      "",
			}
			blob, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal envelope failed: %v", err)
			}
			var decoded map[string]json.RawMessage
			if err := json.Unmarshal(blob, &decoded); err != nil {
				t.Fatalf("unmarshal envelope failed: %v", err)
			}
			if _, ok := decoded["parsed_decisions"]; !ok {
				t.Fatal("expected parsed_decisions key")
			}
			if _, ok := decoded["parse_error"]; !ok {
				t.Fatal("expected parse_error key")
			}
		})
	}
}
