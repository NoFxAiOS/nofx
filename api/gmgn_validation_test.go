package api

import (
	"nofx/store"
	"testing"
)

func TestValidateTraderExchangeSelectionRequiresGMGNChainAndWallet(t *testing.T) {
	exchange := &store.Exchange{
		ExchangeType: "gmgn",
		Name:         "GMGN",
		AccountName:  "Primary",
		Enabled:      true,
	}

	if msg, _, _ := validateTraderExchangeSelection(exchange, "", "0xabc"); msg == "" {
		t.Fatal("expected missing chain validation error")
	}
	if msg, _, _ := validateTraderExchangeSelection(exchange, "sol", ""); msg == "" {
		t.Fatal("expected missing wallet validation error")
	}
	if msg, _, _ := validateTraderExchangeSelection(exchange, "base", "0xabc"); msg != "" {
		t.Fatalf("unexpected validation error: %s", msg)
	}
}
