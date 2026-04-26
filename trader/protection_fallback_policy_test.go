package trader

import "testing"

func TestEvaluateProtectionFallbackPolicyAllowsExplicitRareFallback(t *testing.T) {
	result := evaluateProtectionFallbackPolicy(nil, ProtectionFallbackEvent{Symbol: "BTCUSDT", Reason: "market_data_unavailable", Source: "fallback_exception"}, 1)
	if !result.Allowed || result.BugSignal || result.ReviewLevel != "warning" {
		t.Fatalf("expected rare explicit fallback allowed as warning, got %+v", result)
	}
}

func TestEvaluateProtectionFallbackPolicyRejectsMissingReason(t *testing.T) {
	result := evaluateProtectionFallbackPolicy(nil, ProtectionFallbackEvent{Symbol: "BTCUSDT"}, 1)
	if result.Allowed || !stringsEqual(result.ReviewLevel, "error") {
		t.Fatalf("expected missing-reason fallback rejected as error, got %+v", result)
	}
}

func TestEvaluateProtectionFallbackPolicyFlagsRepeatedFallbackAsBugSignal(t *testing.T) {
	history := []ProtectionFallbackEvent{{Symbol: "BTCUSDT", Reason: "missing_ai_fields", Source: "fallback_exception"}}
	result := evaluateProtectionFallbackPolicy(history, ProtectionFallbackEvent{Symbol: "BTCUSDT", Reason: "missing_ai_fields", Source: "fallback_exception"}, 1)
	if !result.Allowed || !result.BugSignal || result.ReviewLevel != "error" {
		t.Fatalf("expected repeated fallback bug signal, got %+v", result)
	}
}

func stringsEqual(a, b string) bool { return a == b }
