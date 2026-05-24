package store

import "testing"

func TestDefaultHyperliquidStrategyDoesNotEnableNofxOSData(t *testing.T) {
	cfg := GetDefaultStrategyConfig("zh")
	assertHyperliquidStockRankDefault(t, cfg)
	ind := cfg.Indicators
	if ind.NofxOSAPIKey != "" {
		t.Fatalf("default should not include a NofxOS API key for Hyperliquid strategies")
	}
	if ind.EnableQuantData || ind.EnableQuantOI || ind.EnableQuantNetflow || ind.EnableOIRanking || ind.EnableNetFlowRanking || ind.EnablePriceRanking {
		t.Fatalf("default Hyperliquid strategy must not enable NofxOS datasets: %+v", ind)
	}
	if !ind.EnableRawKlines {
		t.Fatalf("raw Hyperliquid klines must stay enabled")
	}
}

func TestHyperliquidRankDefaultSurvivesClampAndNormalize(t *testing.T) {
	cfg := GetDefaultStrategyConfig("zh")
	cfg.CoinSource.UseAI500 = true
	cfg.ClampLimits()
	assertHyperliquidStockRankDefault(t, cfg)
	if cfg.CoinSource.UseAI500 {
		t.Fatalf("Hyperliquid rank strategy must clear stale AI500 flag: %+v", cfg.CoinSource)
	}
}

func TestEmptyCoinSourceInfersHyperliquidRankNotAI500(t *testing.T) {
	cfg := GetDefaultStrategyConfig("zh")
	cfg.CoinSource = CoinSourceConfig{}
	cfg.NormalizeProductSchema()
	assertHyperliquidStockRankDefault(t, cfg)
}

func assertHyperliquidStockRankDefault(t *testing.T, cfg StrategyConfig) {
	t.Helper()
	if cfg.CoinSource.SourceType != "hyper_rank" || cfg.CoinSource.HyperRankCategory != "stock" || cfg.CoinSource.HyperRankDirection != "gainers" || cfg.CoinSource.HyperRankLimit != 5 {
		t.Fatalf("coin source = %+v, want Hyperliquid dynamic stock gainers top 5", cfg.CoinSource)
	}
}
