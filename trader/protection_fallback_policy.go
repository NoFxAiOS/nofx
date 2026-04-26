package trader

import "strings"

type ProtectionFallbackEvent struct {
	Symbol string
	Reason string
	Source string
}

type ProtectionFallbackPolicyResult struct {
	Allowed       bool
	BugSignal     bool
	ReviewLevel   string
	ReviewMessage string
}

func evaluateProtectionFallbackPolicy(events []ProtectionFallbackEvent, current ProtectionFallbackEvent, normalWindowLimit int) ProtectionFallbackPolicyResult {
	result := ProtectionFallbackPolicyResult{Allowed: true, ReviewLevel: "warning"}
	reason := strings.TrimSpace(current.Reason)
	if reason == "" {
		result.Allowed = false
		result.ReviewLevel = "error"
		result.ReviewMessage = "fallback rejected: missing fallback reason"
		return result
	}

	count := 1
	for _, event := range events {
		if current.Symbol != "" && event.Symbol != "" && !strings.EqualFold(event.Symbol, current.Symbol) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(event.Reason), reason) {
			count++
		}
	}
	if normalWindowLimit <= 0 {
		normalWindowLimit = 1
	}
	if count > normalWindowLimit {
		result.BugSignal = true
		result.ReviewLevel = "error"
		result.ReviewMessage = "fallback repeated above normal limit: fix prompt/schema/data plumbing"
		return result
	}
	result.ReviewMessage = "fallback allowed as explicit degraded exception"
	return result
}
