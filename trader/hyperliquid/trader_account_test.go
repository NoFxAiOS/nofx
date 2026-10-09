package hyperliquid

import (
	"math"
	"testing"
)

func requireClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.9f, want %.9f", got, want)
	}
}

func TestUnifiedAccountUsesMarkToMarketSpotTotalAsEquity(t *testing.T) {
	breakdown := calculateHyperliquidBalanceBreakdown(
		true,
		26.33, // Unified Spot total already includes unrealized PnL.
		25.96, // Unified Spot hold is authoritative reserved collateral.
		0,
		0,
		0,
		0,
		25.96, // xyz account value is a view of the same shared collateral.
		-0.32,
		25.96,
	)

	requireClose(t, breakdown.TotalEquity, 26.33)
	requireClose(t, breakdown.TotalWalletBalance, 26.65)
	requireClose(t, breakdown.AvailableBalance, 0.37)
	requireClose(t, breakdown.TotalUnrealizedProfit, -0.32)
}

func TestUnifiedAccountDoesNotAddPositiveUnrealizedPnlTwice(t *testing.T) {
	breakdown := calculateHyperliquidBalanceBreakdown(
		true,
		307.571392,
		164.898229,
		0,
		0,
		0,
		0,
		164.898229,
		1.84147,
		150, // Approximate position-derived margin must not override Spot hold.
	)

	requireClose(t, breakdown.TotalEquity, 307.571392)
	requireClose(t, breakdown.TotalWalletBalance, 305.729922)
	requireClose(t, breakdown.AvailableBalance, 142.673163)
	requireClose(t, breakdown.TotalMarginUsed, 150)
}

func TestUnifiedAccountNeverFallsBackToSeparateAccountAggregation(t *testing.T) {
	breakdown := calculateHyperliquidBalanceBreakdown(
		true,
		0,
		0,
		100,
		5,
		10,
		90,
		80,
		-2,
		8,
	)

	requireClose(t, breakdown.TotalEquity, 0)
	requireClose(t, breakdown.AvailableBalance, 0)
}

func TestSeparateAccountsStillAddIndependentBalances(t *testing.T) {
	breakdown := calculateHyperliquidBalanceBreakdown(
		false,
		30,
		0,
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
