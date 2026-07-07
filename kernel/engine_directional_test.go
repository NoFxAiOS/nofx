package kernel

import (
	"testing"

	"nofx/provider/vergex"
)

func TestDirectionalCandidates(t *testing.T) {
	e := &StrategyEngine{
		vergexRankingCache: map[string]*vergex.SignalRankItem{
			"xyz:NVDA": {Symbol: "xyz:NVDA", Bias: "bullish", Rank: 2, Score: 1.07},
			"xyz:AAPL": {Symbol: "xyz:AAPL", Bias: "bullish", Rank: 1, Score: 1.76},
			"BTC":      {Symbol: "BTC", Bias: "bearish", Rank: 3, Score: -0.9},
			"ETH":      {Symbol: "ETH", Bias: "bearish", Rank: 1, Score: -0.05},
			"SOL":      {Symbol: "SOL", Bias: "neutral", Rank: 1, Score: 0.4},
		},
	}

	bull, bear := e.DirectionalCandidates()

	if len(bull) != 2 || bull[0].Symbol != "xyz:AAPL" || bull[1].Symbol != "xyz:NVDA" {
		t.Fatalf("bullish should be rank-ordered [xyz:AAPL xyz:NVDA], got %v", bull)
	}
	if bull[0].Score != 1.76 || bull[1].Score != 1.07 {
		t.Fatalf("bullish candidates should carry their board scores, got %v", bull)
	}
	if len(bear) != 2 || bear[0].Symbol != "ETH" || bear[1].Symbol != "BTC" {
		t.Fatalf("bearish should be rank-ordered [ETH BTC], got %v", bear)
	}
	if bear[0].Score != -0.05 || bear[1].Score != -0.9 {
		t.Fatalf("bearish candidates should carry their board scores, got %v", bear)
	}
}

func TestDirectionalCandidatesEmpty(t *testing.T) {
	e := &StrategyEngine{}
	bull, bear := e.DirectionalCandidates()
	if len(bull) != 0 || len(bear) != 0 {
		t.Fatalf("empty cache should yield no candidates, got %v %v", bull, bear)
	}
}
