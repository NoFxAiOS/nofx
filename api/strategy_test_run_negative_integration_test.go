package api

import (
	"encoding/json"
	"strings"
	"testing"

	"nofx/kernel"
)

func TestInvalidProtectionResponsesExposeParseErrorInEnvelope(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantError string
	}{
		{
			name: "close action with protection plan",
			raw:  `[{"symbol":"ETHUSDT","action":"close_long","confidence":82,"reasoning":"test","protection_plan":{"mode":"full","take_profit_pct":8,"stop_loss_pct":3}}]`,
			wantError: "protection_plan is only allowed for open actions",
		},
		{
			name: "full mode with ladder rules",
			raw:  `[{"symbol":"BTCUSDT","action":"open_long","leverage":3,"position_size_usd":100,"reasoning":"test","protection_plan":{"mode":"full","take_profit_pct":8,"stop_loss_pct":3,"ladder_rules":[{"take_profit_pct":3}]}}]`,
			wantError: "full protection_plan must not include ladder_rules",
		},
		{
			name: "ladder mode without rules",
			raw:  `[{"symbol":"SOLUSDT","action":"open_short","leverage":2,"position_size_usd":100,"reasoning":"test","protection_plan":{"mode":"ladder"}}]`,
			wantError: "ladder protection_plan requires ladder_rules",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decisions, parseErr := kernel.ParseAIDecisions(tc.raw)
			if parseErr != nil {
				t.Fatalf("expected parser to succeed before validator failure, got %v", parseErr)
			}
			validationErr := kernel.ValidateAIDecisions(decisions)
			if validationErr == nil {
				t.Fatal("expected validator to fail")
			}
			if !strings.Contains(validationErr.Error(), tc.wantError) {
				t.Fatalf("expected validation error to contain %q, got %v", tc.wantError, validationErr)
			}

			payload := map[string]any{
				"ai_response":      tc.raw,
				"parsed_decisions": decisions,
				"parse_error":      validationErr.Error(),
			}
			blob, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal envelope failed: %v", err)
			}
			var decoded struct {
				ParseError string `json:"parse_error"`
			}
			if err := json.Unmarshal(blob, &decoded); err != nil {
				t.Fatalf("unmarshal envelope failed: %v", err)
			}
			if !strings.Contains(decoded.ParseError, tc.wantError) {
				t.Fatalf("expected envelope parse_error to contain %q, got %q", tc.wantError, decoded.ParseError)
			}
		})
	}
}
