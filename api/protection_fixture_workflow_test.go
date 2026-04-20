package api

import (
	"encoding/json"
	"os"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestProtectionFixtureWorkflowAcceptance(t *testing.T) {
	blob, err := os.ReadFile("../docs/fixtures/protection-test-run-fixture.json")
	if err != nil {
		t.Fatalf("read fixture failed: %v", err)
	}
	var fixture struct {
		Config store.StrategyConfig `json:"config"`
	}
	if err := json.Unmarshal(blob, &fixture); err != nil {
		t.Fatalf("unmarshal fixture failed: %v", err)
	}

	engine := kernel.NewStrategyEngine(&fixture.Config)
	systemPrompt := engine.BuildSystemPrompt(1000, "balanced")
	if systemPrompt == "" {
		t.Fatal("system prompt should not be empty")
	}
	for _, needle := range []string{"protection_plan", "mode=full", "mode=ladder", "mode=drawdown", "open_long", "close_long"} {
		if !containsString(systemPrompt, needle) {
			t.Fatalf("system prompt should contain %q", needle)
		}
	}

	responses := []struct {
		name      string
		raw       string
		wantMode  string
		wantCount int
	}{
		{
			name:      "fixture full open",
			raw:       `[{"symbol":"BTCUSDT","action":"open_long","leverage":3,"position_size_usd":500,"confidence":78,"reasoning":"test","protection_plan":{"mode":"full","take_profit_pct":8,"stop_loss_pct":3}}]`,
			wantMode:  "full",
			wantCount: 1,
		},
		{
			name:      "fixture ladder open",
			raw:       `[{"symbol":"SOLUSDT","action":"open_short","leverage":2,"position_size_usd":300,"confidence":80,"reasoning":"test","protection_plan":{"mode":"ladder","ladder_rules":[{"take_profit_pct":3,"take_profit_close_ratio_pct":40,"stop_loss_pct":1.5,"stop_loss_close_ratio_pct":25}]}}]`,
			wantMode:  "ladder",
			wantCount: 1,
		},
		{
			name:      "fixture drawdown open",
			raw:       `[{"symbol":"XRPUSDT","action":"open_long","leverage":2,"position_size_usd":400,"confidence":74,"reasoning":"15m primary timeframe with 5m continuation, support/resistance, fibonacci, volatility","protection_plan":{"mode":"drawdown","drawdown_rules":[{"timeframe":"5m","min_profit_pct":2.5,"max_drawdown_pct":35,"close_ratio_pct":20,"poll_interval_seconds":30,"reason_anchor":"micro structure"},{"timeframe":"15m","min_profit_pct":4.5,"max_drawdown_pct":30,"close_ratio_pct":50,"poll_interval_seconds":60,"reason_anchor":"primary structure"}]}}]`,
			wantMode:  "drawdown",
			wantCount: 1,
		},
		{
			name:      "fixture close without protection",
			raw:       `[{"symbol":"ETHUSDT","action":"close_long","confidence":82,"reasoning":"test"}]`,
			wantMode:  "",
			wantCount: 1,
		},
	}

	for _, tc := range responses {
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
					t.Fatalf("expected no protection_plan, got %+v", decisions[0].ProtectionPlan)
				}
			} else if decisions[0].ProtectionPlan == nil || decisions[0].ProtectionPlan.Mode != tc.wantMode {
				t.Fatalf("expected protection_plan mode %q, got %+v", tc.wantMode, decisions[0].ProtectionPlan)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s == substr || containsAtAny(s, substr)))
}

func containsAtAny(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
