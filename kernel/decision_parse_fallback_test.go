package kernel

import "testing"

func TestParseFullDecisionResponseMarksMissingJSONFallback(t *testing.T) {
	decision, err := parseFullDecisionResponse("analysis only; no structured decision", 1000, 5, 3, 0.2, 0.1)
	if err != nil {
		t.Fatalf("expected missing JSON fallback to remain backward-compatible, got error: %v", err)
	}
	if decision == nil || !decision.ParseFallback || decision.ParseFallbackReason != "missing_json_decision_array" {
		t.Fatalf("expected parse fallback metadata, got %+v", decision)
	}
	if len(decision.Decisions) != 1 || decision.Decisions[0].Action != "wait" || decision.Decisions[0].Symbol != "ALL" {
		t.Fatalf("expected synthetic safe wait fallback, got %+v", decision.Decisions)
	}
}

func TestParseFullDecisionResponseDoesNotMarkLegalEmptyArrayFallback(t *testing.T) {
	decision, err := parseFullDecisionResponse("[]", 1000, 5, 3, 0.2, 0.1)
	if err != nil {
		t.Fatalf("expected legal empty array to parse, got error: %v", err)
	}
	if decision == nil {
		t.Fatal("expected decision")
	}
	if decision.ParseFallback || decision.ParseFallbackReason != "" {
		t.Fatalf("legal [] no-trade must not be marked as parser fallback, got %+v", decision)
	}
	if len(decision.Decisions) != 0 {
		t.Fatalf("expected empty decisions for legal [], got %+v", decision.Decisions)
	}
}
