package hyperliquid

import "testing"

func TestUnifiedAccountDoesNotDoubleCountXYZAccountValue(t *testing.T) {
	breakdown := calculateHyperliquidBalanceBreakdown(
		true,
		26.33, // Spot USDC collateral
		0,
		0,
		0,
		0,
		25.96, // xyz account value is a view of the same shared collateral
		-0.32,
		25.96,
	)

	if breakdown.TotalEquity < 25.99 || breakdown.TotalEquity > 26.02 {
		t.Fatalf("expected total equity to be spot + unrealized pnl, got %.4f", breakdown.TotalEquity)
	}
	if breakdown.TotalEquity > 40 {
		t.Fatalf("unified collateral was double-counted: %.4f", breakdown.TotalEquity)
	}
	if breakdown.AvailableBalance > 0.1 {
		t.Fatalf("expected almost no free collateral with full-size margin, got %.4f", breakdown.AvailableBalance)
	}
}

func TestSeparateAccountsStillAddIndependentBalances(t *testing.T) {
	breakdown := calculateHyperliquidBalanceBreakdown(
		false,
		30,
		10,
		1,
		2,
		8,
		5,
		-0.5,
		1,
	)

	if breakdown.TotalEquity != 45 {
		t.Fatalf("expected independent accounts to add to 45, got %.4f", breakdown.TotalEquity)
	}
	if breakdown.TotalWalletBalance != 44.5 {
		t.Fatalf("expected wallet balance 44.5, got %.4f", breakdown.TotalWalletBalance)
	}
}
