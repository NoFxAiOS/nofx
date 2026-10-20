package binance_stocks

import "testing"

// The adapter has no account-wide position endpoint: GetPositions must fail
// instead of reporting a successful empty (flat) account.
func TestGetPositionsFailsInsteadOfReportingFlatAccount(t *testing.T) {
	tr := &StocksTrader{}
	pos, err := tr.GetPositions()
	if err == nil {
		t.Fatalf("GetPositions returned success with %v; want an error", pos)
	}
}
