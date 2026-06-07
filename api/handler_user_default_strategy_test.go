package api

import (
	"testing"

	"github.com/google/uuid"
	"nofx/store"
)

func TestCreateDefaultStrategiesUsesReadyToRunUSStockPresets(t *testing.T) {
	st, err := store.New(t.TempDir() + "/nofx.db")
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	s := &Server{store: st}
	userID := "user-us-stock-presets"
	if err := s.createDefaultStrategies(userID, "zh"); err != nil {
		t.Fatalf("createDefaultStrategies failed: %v", err)
	}

	strategies, err := st.Strategy().List(userID)
	if err != nil {
		t.Fatalf("List strategies failed: %v", err)
	}
	if len(strategies) != 3 {
		t.Fatalf("expected 3 default strategies, got %d", len(strategies))
	}

	byName := map[string]*store.Strategy{}
	activeCount := 0
	for _, strategy := range strategies {
		byName[strategy.Name] = strategy
		if strategy.IsActive {
			activeCount++
		}
		if strategy.Name == "均衡策略" || strategy.Name == "稳健策略" || strategy.Name == "积极策略" {
			t.Fatalf("legacy crypto-style default strategy still present: %s", strategy.Name)
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active strategy, got %d", activeCount)
	}

	trend := byName["美股趋势策略"]
	if trend == nil || !trend.IsActive {
		t.Fatalf("美股趋势策略 should exist and be active")
	}
	trendCfg, err := trend.ParseConfig()
	if err != nil {
		t.Fatalf("trend ParseConfig failed: %v", err)
	}
	if trendCfg.CoinSource.SourceType != "hyper_rank" || trendCfg.CoinSource.HyperRankCategory != "stock" || trendCfg.CoinSource.HyperRankDirection != "volume" {
		t.Fatalf("trend strategy should use Hyperliquid stock volume ranking, got %+v", trendCfg.CoinSource)
	}
	if trendCfg.CoinSource.UseAI500 || trendCfg.RiskControl.MaxPositions > 2 || trendCfg.RiskControl.MaxMarginUsage > 0.45 {
		t.Fatalf("trend strategy should be low-risk Hyperliquid native, got coin=%+v risk=%+v", trendCfg.CoinSource, trendCfg.RiskControl)
	}

	megaCap := byName["美股大盘稳健策略"]
	if megaCap == nil {
		t.Fatalf("美股大盘稳健策略 should exist")
	}
	megaCfg, err := megaCap.ParseConfig()
	if err != nil {
		t.Fatalf("megaCap ParseConfig failed: %v", err)
	}
	if megaCfg.CoinSource.SourceType != "static" {
		t.Fatalf("mega-cap strategy should use static stock symbols, got %+v", megaCfg.CoinSource)
	}
	wantSymbols := []string{"AAPL-USDC", "MSFT-USDC", "GOOGL-USDC", "AMZN-USDC", "META-USDC"}
	if len(megaCfg.CoinSource.StaticCoins) != len(wantSymbols) {
		t.Fatalf("unexpected static stock list: %+v", megaCfg.CoinSource.StaticCoins)
	}
	for i, want := range wantSymbols {
		if megaCfg.CoinSource.StaticCoins[i] != want {
			t.Fatalf("static stock %d: want %s got %s", i, want, megaCfg.CoinSource.StaticCoins[i])
		}
	}
}

func TestCreateDefaultStrategiesMigratesLegacyPresetsWithoutOverridingActiveCustom(t *testing.T) {
	st, err := store.New(t.TempDir() + "/nofx.db")
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	userID := "user-existing-custom"
	legacyCfg := store.GetDefaultStrategyConfig("zh")
	legacy := &store.Strategy{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        "均衡策略",
		Description: "legacy",
		IsActive:    false,
	}
	if err := legacy.SetConfig(&legacyCfg); err != nil {
		t.Fatalf("legacy SetConfig failed: %v", err)
	}
	if err := st.Strategy().Create(legacy); err != nil {
		t.Fatalf("create legacy failed: %v", err)
	}

	custom := &store.Strategy{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        "aa",
		Description: "user custom active strategy",
		IsActive:    true,
	}
	if err := custom.SetConfig(&legacyCfg); err != nil {
		t.Fatalf("custom SetConfig failed: %v", err)
	}
	if err := st.Strategy().Create(custom); err != nil {
		t.Fatalf("create custom failed: %v", err)
	}

	s := &Server{store: st}
	if err := s.createDefaultStrategies(userID, "zh"); err != nil {
		t.Fatalf("createDefaultStrategies failed: %v", err)
	}
	if err := s.createDefaultStrategies(userID, "zh"); err != nil {
		t.Fatalf("second createDefaultStrategies should be idempotent: %v", err)
	}

	strategies, err := st.Strategy().List(userID)
	if err != nil {
		t.Fatalf("List strategies failed: %v", err)
	}
	byName := map[string]int{}
	activeNames := []string{}
	for _, strategy := range strategies {
		byName[strategy.Name]++
		if strategy.IsActive {
			activeNames = append(activeNames, strategy.Name)
		}
	}
	if byName["均衡策略"] != 0 {
		t.Fatalf("legacy preset should be removed, got names=%+v", byName)
	}
	for _, name := range []string{"美股趋势策略", "美股大盘稳健策略", "美股突破策略"} {
		if byName[name] != 1 {
			t.Fatalf("expected exactly one %s, got names=%+v", name, byName)
		}
	}
	if len(activeNames) != 1 || activeNames[0] != "aa" {
		t.Fatalf("existing active custom strategy should stay the only active one, got %+v", activeNames)
	}
}
