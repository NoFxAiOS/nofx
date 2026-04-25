package api

import "testing"

func TestValidateTraderLeverageRangeMatchesManualLimits(t *testing.T) {
	if msg, code := validateTraderLeverageRange(20, 20); msg != "" || code != "" {
		t.Fatalf("expected 20/20 leverage to be accepted, got msg=%q code=%q", msg, code)
	}

	if msg, code := validateTraderLeverageRange(21, 20); msg == "" || code != "trader.create.invalid_btc_eth_leverage" {
		t.Fatalf("expected BTC/ETH leverage > 20 to be rejected, got msg=%q code=%q", msg, code)
	}

	if msg, code := validateTraderLeverageRange(20, 21); msg == "" || code != "trader.create.invalid_altcoin_leverage" {
		t.Fatalf("expected altcoin leverage > 20 to be rejected, got msg=%q code=%q", msg, code)
	}
}
