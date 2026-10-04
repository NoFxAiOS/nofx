package kernel

import (
	"strings"
	"testing"

	"nofx/provider/vergex"
)

func TestPositionPromptIncludesCanonicalVergexSignalWithoutCandles(t *testing.T) {
	core := &vergex.MarketAnalysis{Symbol: "SOL", DirectionCurrent: []byte(`{"direction":"bullish"}`)}
	hip3 := &vergex.MarketAnalysis{Symbol: "xyz:SOL", DirectionCurrent: []byte(`{"direction":"bearish"}`)}
	ctx := &Context{VergexDataMap: map[string]*vergex.MarketAnalysis{"SOL": core, "xyz:SOL": hip3}}
	if vergexDataForSymbol(ctx, "SOLUSDT") != core || vergexDataForSymbol(ctx, "xyz:SOL") != hip3 {
		t.Fatal("canonical lookup crossed market namespaces")
	}
	e := &StrategyEngine{}
	prompt := e.formatPositionInfo(1, PositionInfo{Symbol: "SOLUSDT", Side: "long"}, ctx)
	if !strings.Contains(prompt, "Vergex Claw402 Signals") || !strings.Contains(prompt, "bullish") || strings.Contains(prompt, "bearish") {
		t.Fatalf("wrong position signal: %s", prompt)
	}
}

func TestVergexDetailQueryCandidatesUseHIP3MarketAndMainnetChain(t *testing.T) {
	candidates := vergexDetailQueryCandidates(vergex.Query{
		MarketType: vergex.DefaultMarketType,
		Symbol:     "xyz:INTC",
		Chain:      vergex.DefaultChain,
		Category:   "stock",
	})

	if len(candidates) == 0 {
		t.Fatal("expected detail query candidates")
	}
	if candidates[0].MarketType != "hip3_perp" || candidates[0].Chain != "mainnet" {
		t.Fatalf("first candidate = %+v, want hip3_perp/mainnet", candidates[0])
	}

	if !hasVergexDetailCandidate(candidates, "hip3_perp", "") {
		t.Fatalf("expected hip3_perp/default-chain fallback in %+v", candidates)
	}
	if hasVergexDetailCandidate(candidates, "stock", "mainnet") {
		t.Fatalf("did not expect stock marketType fallback for Vergex detail endpoint: %+v", candidates)
	}
}

func TestVergexDetailSymbolForLookupKeepsCoreCryptoBaseSymbols(t *testing.T) {
	cases := []struct {
		name       string
		marketType string
		symbol     string
		want       string
	}{
		{
			name:       "core crypto from all board",
			marketType: "all",
			symbol:     "AAVE",
			want:       "AAVE",
		},
		{
			name:       "core crypto with usdt suffix",
			marketType: "all",
			symbol:     "HYPEUSDT",
			want:       "HYPE",
		},
		{
			name:       "xyz stock keeps xyz prefix",
			marketType: "all",
			symbol:     "xyz:INTC",
			want:       "xyz:INTC",
		},
		{
			name:       "hip3 symbol gains xyz prefix",
			marketType: vergex.DefaultMarketType,
			symbol:     "SNDK",
			want:       "xyz:SNDK",
		},
		{
			name:       "core market strips suffix",
			marketType: "core_perp",
			symbol:     "LITUSDT",
			want:       "LIT",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := vergexDetailSymbolForLookup(tc.marketType, tc.symbol); got != tc.want {
				t.Fatalf("vergexDetailSymbolForLookup(%q, %q) = %q, want %q", tc.marketType, tc.symbol, got, tc.want)
			}
		})
	}
}

func TestVergexDetailQueryCandidatesPreferMarketTypeBySymbolWhenSourceIsAll(t *testing.T) {
	cryptoCandidates := vergexDetailQueryCandidates(vergex.Query{
		MarketType: "all",
		Symbol:     "AAVE",
		Chain:      "mainnet",
	})
	if len(cryptoCandidates) == 0 || cryptoCandidates[0].MarketType != "core_perp" {
		t.Fatalf("crypto candidates should prefer core_perp first: %+v", cryptoCandidates)
	}

	xyzCandidates := vergexDetailQueryCandidates(vergex.Query{
		MarketType: "all",
		Symbol:     "xyz:SNDK",
		Chain:      "mainnet",
	})
	if len(xyzCandidates) == 0 || xyzCandidates[0].MarketType != vergex.DefaultMarketType {
		t.Fatalf("xyz candidates should prefer hip3_perp first: %+v", xyzCandidates)
	}
}

func hasVergexDetailCandidate(candidates []vergex.Query, marketType, chain string) bool {
	for _, candidate := range candidates {
		if candidate.MarketType == marketType && candidate.Chain == chain {
			return true
		}
	}
	return false
}
