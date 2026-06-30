package kernel

import (
	"testing"

	"nofx/provider/vergex"
)

func TestDirectionalCandidates(t *testing.T) {
	e := &StrategyEngine{
		vergexRankingCache: map[string]*vergex.SignalRankItem{
			"xyz:NVDA": {Symbol: "xyz:NVDA", Bias: "bullish", Rank: 2},
			"xyz:AAPL": {Symbol: "xyz:AAPL", Bias: "bullish", Rank: 1},
			"BTC":      {Symbol: "BTC", Bias: "bearish", Rank: 3},
			"ETH":      {Symbol: "ETH", Bias: "bearish", Rank: 1},
			"SOL":      {Symbol: "SOL", Bias: "neutral", Rank: 1},
		},
	}

	bull, bear := e.DirectionalCandidates()

	if len(bull) != 2 || bull[0] != "xyz:AAPL" || bull[1] != "xyz:NVDA" {
		t.Fatalf("bullish should be rank-ordered [xyz:AAPL xyz:NVDA], got %v", bull)
	}
	if len(bear) != 2 || bear[0] != "ETH" || bear[1] != "BTC" {
		t.Fatalf("bearish should be rank-ordered [ETH BTC], got %v", bear)
	}
}

func TestDirectionalCandidatesEmpty(t *testing.T) {
	e := &StrategyEngine{}
	bull, bear := e.DirectionalCandidates()
	if len(bull) != 0 || len(bear) != 0 {
		t.Fatalf("empty cache should yield no candidates, got %v %v", bull, bear)
	}
}
