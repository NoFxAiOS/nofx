package trader

import (
	"testing"

	"nofx/kernel"
)

func TestRunCycleReviewContextMarksParseFallback(t *testing.T) {
	record := struct {
		ExecutionLog []string
		ReviewContext map[string]interface{}
	}{
		ExecutionLog: []string{},
		ReviewContext: map[string]interface{}{},
	}
	aiDecision := &kernel.FullDecision{ParseFallback: true, ParseFallbackReason: "missing_json_decision_array"}

	if aiDecision.ParseFallback {
		fallbackMsg := "AI decision parser used safe fallback"
		if aiDecision.ParseFallbackReason != "" {
			fallbackMsg = "AI decision parser used safe fallback: " + aiDecision.ParseFallbackReason
		}
		record.ExecutionLog = append(record.ExecutionLog, fallbackMsg)
		if record.ReviewContext == nil {
			record.ReviewContext = map[string]interface{}{}
		}
		record.ReviewContext["parse_fallback"] = true
		record.ReviewContext["parse_fallback_reason"] = aiDecision.ParseFallbackReason
	}

	if len(record.ExecutionLog) != 1 {
		t.Fatalf("expected one execution log entry, got %+v", record.ExecutionLog)
	}
	if got, _ := record.ReviewContext["parse_fallback"].(bool); !got {
		t.Fatalf("expected parse_fallback=true, got %+v", record.ReviewContext)
	}
	if got, _ := record.ReviewContext["parse_fallback_reason"].(string); got != "missing_json_decision_array" {
		t.Fatalf("unexpected parse_fallback_reason: %+v", record.ReviewContext)
	}
}
