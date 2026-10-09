package trader

import "testing"

func TestExtractInitialBalancePrefersExplicitTotalEquity(t *testing.T) {
	account := map[string]interface{}{
		"totalEquity":           307.571392,
		"totalWalletBalance":    305.729922,
		"totalUnrealizedProfit": 1.84147,
	}

	got := extractInitialBalance(account)
	if got != 307.571392 {
		t.Fatalf("expected mark-to-market total equity, got %.6f", got)
	}
}

func TestExtractInitialBalanceSupportsLegacyExchangeFields(t *testing.T) {
	account := map[string]interface{}{
		"totalWalletBalance": 100.5,
	}

	got := extractInitialBalance(account)
	if got != 100.5 {
		t.Fatalf("expected legacy wallet balance fallback, got %.6f", got)
	}
}
