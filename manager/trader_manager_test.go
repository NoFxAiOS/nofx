package manager

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"nofx/store"
	"nofx/trader"
)

// newIdleTrader returns a zero-value AutoTrader. It is safe to store in the
// manager map for map-semantics tests: GetStatus works on a zero value and
// Stop returns early because the trader is not running. It must NOT be used
// for anything that touches an exchange (Run, GetAccountInfo, ...).
func newIdleTrader() *trader.AutoTrader {
	return &trader.AutoTrader{}
}

// insertTrader places a trader directly into the manager's internal map,
// bypassing store loading (same-package access).
func insertTrader(tm *TraderManager, id string, t *trader.AutoTrader) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.traders[id] = t
}

func TestNewTraderManager(t *testing.T) {
	tm := NewTraderManager()

	if tm == nil {
		t.Fatal("NewTraderManager() returned nil")
	}
	if tm.traders == nil {
		t.Error("traders map should be initialized, got nil")
	}
	if len(tm.traders) != 0 {
		t.Errorf("traders map should be empty, got %d entries", len(tm.traders))
	}
	if tm.loadErrors == nil {
		t.Error("loadErrors map should be initialized, got nil")
	}
	if len(tm.loadErrors) != 0 {
		t.Errorf("loadErrors map should be empty, got %d entries", len(tm.loadErrors))
	}
	if tm.competitionCache == nil {
		t.Fatal("competitionCache should be initialized, got nil")
	}
	if tm.competitionCache.data == nil {
		t.Error("competitionCache.data should be initialized, got nil")
	}
	if !tm.competitionCache.timestamp.IsZero() {
		t.Errorf("competitionCache.timestamp should be zero, got %v", tm.competitionCache.timestamp)
	}
}

func TestGetTrader(t *testing.T) {
	tm := NewTraderManager()

	t.Run("missing ID returns error", func(t *testing.T) {
		got, err := tm.GetTrader("does-not-exist")
		if err == nil {
			t.Fatal("GetTrader on missing ID expected error, got nil")
		}
		if got != nil {
			t.Errorf("GetTrader on missing ID should return nil trader, got %v", got)
		}
		if !strings.Contains(err.Error(), "does-not-exist") {
			t.Errorf("error %q should mention the trader ID", err.Error())
		}
	})

	t.Run("existing ID returns same instance", func(t *testing.T) {
		at := newIdleTrader()
		insertTrader(tm, "trader-1", at)

		got, err := tm.GetTrader("trader-1")
		if err != nil {
			t.Fatalf("GetTrader unexpected error: %v", err)
		}
		if got != at {
			t.Errorf("GetTrader returned %p, want the stored instance %p", got, at)
		}
	})
}

func TestGetLoadError(t *testing.T) {
	tm := NewTraderManager()

	t.Run("unknown trader returns nil", func(t *testing.T) {
		if err := tm.GetLoadError("unknown"); err != nil {
			t.Errorf("GetLoadError for unknown trader = %v, want nil", err)
		}
	})

	t.Run("stored error is returned", func(t *testing.T) {
		wantErr := errors.New("failed to create trader: boom")
		tm.mu.Lock()
		tm.loadErrors["trader-x"] = wantErr
		tm.mu.Unlock()

		if got := tm.GetLoadError("trader-x"); !errors.Is(got, wantErr) {
			t.Errorf("GetLoadError = %v, want %v", got, wantErr)
		}
	})
}

func TestGetAllTradersReturnsCopy(t *testing.T) {
	tm := NewTraderManager()
	at1 := newIdleTrader()
	at2 := newIdleTrader()
	insertTrader(tm, "t1", at1)
	insertTrader(tm, "t2", at2)

	all := tm.GetAllTraders()

	if len(all) != 2 {
		t.Fatalf("GetAllTraders returned %d entries, want 2", len(all))
	}
	if all["t1"] != at1 || all["t2"] != at2 {
		t.Error("GetAllTraders should return the same trader instances")
	}

	// Mutating the returned map must not affect internal state.
	delete(all, "t1")
	all["t3"] = newIdleTrader()

	if _, err := tm.GetTrader("t1"); err != nil {
		t.Errorf("deleting from returned map leaked into internal state: %v", err)
	}
	if _, err := tm.GetTrader("t3"); err == nil {
		t.Error("adding to returned map leaked into internal state")
	}
	if got := len(tm.GetTraderIDs()); got != 2 {
		t.Errorf("internal trader count = %d after mutating returned map, want 2", got)
	}
}

func TestGetTraderIDs(t *testing.T) {
	tm := NewTraderManager()

	t.Run("empty manager returns empty non-nil slice", func(t *testing.T) {
		ids := tm.GetTraderIDs()
		if ids == nil {
			t.Fatal("GetTraderIDs should return an empty slice, got nil")
		}
		if len(ids) != 0 {
			t.Errorf("GetTraderIDs = %v, want empty", ids)
		}
	})

	t.Run("returns all IDs", func(t *testing.T) {
		want := []string{"a", "b", "c"}
		for _, id := range want {
			insertTrader(tm, id, newIdleTrader())
		}

		got := tm.GetTraderIDs()
		sort.Strings(got)
		if len(got) != len(want) {
			t.Fatalf("GetTraderIDs returned %d IDs, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("GetTraderIDs[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestRemoveTrader(t *testing.T) {
	t.Run("removes existing non-running trader", func(t *testing.T) {
		tm := NewTraderManager()
		insertTrader(tm, "t1", newIdleTrader())

		tm.RemoveTrader("t1")

		if _, err := tm.GetTrader("t1"); err == nil {
			t.Error("trader t1 should be removed")
		}
		if got := len(tm.GetTraderIDs()); got != 0 {
			t.Errorf("trader count after removal = %d, want 0", got)
		}
	})

	t.Run("missing ID is a no-op", func(t *testing.T) {
		tm := NewTraderManager()
		insertTrader(tm, "t1", newIdleTrader())

		tm.RemoveTrader("missing") // must not panic

		if _, err := tm.GetTrader("t1"); err != nil {
			t.Errorf("unrelated trader was removed: %v", err)
		}
	})
}

func TestStartAllEmpty(t *testing.T) {
	tm := NewTraderManager()
	tm.StartAll() // must not panic with no traders
}

func TestStopAllWithIdleTraders(t *testing.T) {
	tm := NewTraderManager()
	tm.StopAll() // empty: must not panic

	insertTrader(tm, "t1", newIdleTrader())
	insertTrader(tm, "t2", newIdleTrader())
	tm.StopAll() // not-running traders: Stop is an early-return no-op
}

func TestTraderLogTag(t *testing.T) {
	tests := []struct {
		name       string
		traderID   string
		traderName string
		want       string
	}{
		{
			name:       "with name",
			traderID:   "abc-123",
			traderName: "MyBot",
			want:       "[trader_id=abc-123 trader_name=MyBot]",
		},
		{
			name:     "without name",
			traderID: "abc-123",
			want:     "[trader_id=abc-123]",
		},
		{
			name: "both empty",
			want: "[trader_id=]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := traderLogTag(tt.traderID, tt.traderName); got != tt.want {
				t.Errorf("traderLogTag(%q, %q) = %q, want %q", tt.traderID, tt.traderName, got, tt.want)
			}
		})
	}
}

func TestEnsureHyperliquidNativeStrategy(t *testing.T) {
	t.Run("nil config does not panic", func(t *testing.T) {
		ensureHyperliquidNativeStrategy("bot", "hyperliquid", nil)
	})

	t.Run("non-hyperliquid exchange is untouched", func(t *testing.T) {
		cfg := &store.StrategyConfig{
			CoinSource: store.CoinSourceConfig{
				SourceType: "ai500",
				UseAI500:   true,
			},
		}
		ensureHyperliquidNativeStrategy("bot", "binance", cfg)

		if cfg.CoinSource.SourceType != "ai500" || !cfg.CoinSource.UseAI500 {
			t.Errorf("non-hyperliquid config was modified: %+v", cfg.CoinSource)
		}
	})

	t.Run("native sources are kept as-is", func(t *testing.T) {
		nativeSources := []string{"hyper_rank", "static", "hyper_all", "hyper_main", " Hyper_Rank "}
		for _, src := range nativeSources {
			cfg := &store.StrategyConfig{
				CoinSource: store.CoinSourceConfig{SourceType: src},
			}
			ensureHyperliquidNativeStrategy("bot", "hyperliquid", cfg)

			if cfg.CoinSource.SourceType != src {
				t.Errorf("native source %q was rewritten to %q", src, cfg.CoinSource.SourceType)
			}
		}
	})

	t.Run("legacy source on hyperliquid is forced to hyper_rank with defaults", func(t *testing.T) {
		cfg := &store.StrategyConfig{
			CoinSource: store.CoinSourceConfig{
				SourceType:   "ai500",
				UseAI500:     true,
				UseOITop:     true,
				UseOILow:     true,
				UseHyperAll:  true,
				UseHyperMain: true,
			},
		}
		ensureHyperliquidNativeStrategy("bot", "hyperliquid", cfg)

		cs := cfg.CoinSource
		if cs.SourceType != "hyper_rank" {
			t.Errorf("SourceType = %q, want hyper_rank", cs.SourceType)
		}
		if cs.UseAI500 || cs.UseOITop || cs.UseOILow || cs.UseHyperAll || cs.UseHyperMain {
			t.Errorf("legacy source flags should all be cleared: %+v", cs)
		}
		if cs.HyperRankCategory != "stock" {
			t.Errorf("HyperRankCategory = %q, want stock", cs.HyperRankCategory)
		}
		if cs.HyperRankDirection != "gainers" {
			t.Errorf("HyperRankDirection = %q, want gainers", cs.HyperRankDirection)
		}
		if cs.HyperRankLimit != 5 {
			t.Errorf("HyperRankLimit = %d, want 5", cs.HyperRankLimit)
		}
	})

	t.Run("existing hyper_rank settings are preserved when forcing", func(t *testing.T) {
		cfg := &store.StrategyConfig{
			CoinSource: store.CoinSourceConfig{
				SourceType:         "oi_top",
				HyperRankCategory:  "crypto",
				HyperRankDirection: "losers",
				HyperRankLimit:     8,
			},
		}
		ensureHyperliquidNativeStrategy("bot", "hyperliquid", cfg)

		cs := cfg.CoinSource
		if cs.SourceType != "hyper_rank" {
			t.Errorf("SourceType = %q, want hyper_rank", cs.SourceType)
		}
		if cs.HyperRankCategory != "crypto" {
			t.Errorf("HyperRankCategory = %q, want crypto (preserved)", cs.HyperRankCategory)
		}
		if cs.HyperRankDirection != "losers" {
			t.Errorf("HyperRankDirection = %q, want losers (preserved)", cs.HyperRankDirection)
		}
		if cs.HyperRankLimit != 8 {
			t.Errorf("HyperRankLimit = %d, want 8 (preserved)", cs.HyperRankLimit)
		}
	})

	t.Run("exchange type is matched case-insensitively with whitespace", func(t *testing.T) {
		cfg := &store.StrategyConfig{
			CoinSource: store.CoinSourceConfig{SourceType: "ai500"},
		}
		ensureHyperliquidNativeStrategy("bot", "  HyperLiquid  ", cfg)

		if cfg.CoinSource.SourceType != "hyper_rank" {
			t.Errorf("SourceType = %q, want hyper_rank for case-insensitive exchange match", cfg.CoinSource.SourceType)
		}
	})
}

func TestGetCompetitionDataEmptyAndCache(t *testing.T) {
	tm := NewTraderManager()

	first, err := tm.GetCompetitionData()
	if err != nil {
		t.Fatalf("GetCompetitionData unexpected error: %v", err)
	}
	if got := first["count"]; got != 0 {
		t.Errorf("count = %v, want 0", got)
	}
	if got := first["total_count"]; got != 0 {
		t.Errorf("total_count = %v, want 0", got)
	}

	tm.competitionCache.mu.RLock()
	cachedTimestamp := tm.competitionCache.timestamp
	tm.competitionCache.mu.RUnlock()
	if cachedTimestamp.IsZero() {
		t.Error("competition cache timestamp should be set after first call")
	}

	// Second call within 30s must be served from the cache.
	second, err := tm.GetCompetitionData()
	if err != nil {
		t.Fatalf("GetCompetitionData (cached) unexpected error: %v", err)
	}
	if got := second["count"]; got != 0 {
		t.Errorf("cached count = %v, want 0", got)
	}

	tm.competitionCache.mu.RLock()
	timestampAfterSecond := tm.competitionCache.timestamp
	tm.competitionCache.mu.RUnlock()
	if !timestampAfterSecond.Equal(cachedTimestamp) {
		t.Error("cached call should not refresh the cache timestamp")
	}
}

func TestGetTopTradersDataEmpty(t *testing.T) {
	tm := NewTraderManager()

	result, err := tm.GetTopTradersData()
	if err != nil {
		t.Fatalf("GetTopTradersData unexpected error: %v", err)
	}
	if got := result["count"]; got != 0 {
		t.Errorf("count = %v, want 0", got)
	}
	traders, ok := result["traders"].([]map[string]interface{})
	if !ok {
		t.Fatalf("traders has type %T, want []map[string]interface{}", result["traders"])
	}
	if len(traders) != 0 {
		t.Errorf("traders length = %d, want 0", len(traders))
	}
}

func TestGetComparisonDataEmpty(t *testing.T) {
	tm := NewTraderManager()

	result, err := tm.GetComparisonData()
	if err != nil {
		t.Fatalf("GetComparisonData unexpected error: %v", err)
	}
	if got := result["count"]; got != 0 {
		t.Errorf("count = %v, want 0", got)
	}
}

// TestConcurrentAccess exercises the RWMutex by hammering the read paths
// while traders are removed concurrently. Run with -race.
func TestConcurrentAccess(t *testing.T) {
	tm := NewTraderManager()

	const traderCount = 16
	ids := make([]string, 0, traderCount)
	for i := 0; i < traderCount; i++ {
		id := fmt.Sprintf("trader-%d", i)
		ids = append(ids, id)
		insertTrader(tm, id, newIdleTrader())
	}

	const (
		goroutinesPerKind = 8
		iterations        = 200
	)

	var wg sync.WaitGroup

	// Readers: GetTrader / GetLoadError
	for g := 0; g < goroutinesPerKind; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id := ids[(seed+i)%traderCount]
				_, _ = tm.GetTrader(id)
				_ = tm.GetLoadError(id)
			}
		}(g)
	}

	// Readers: GetAllTraders / GetTraderIDs
	for g := 0; g < goroutinesPerKind; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = tm.GetAllTraders()
				_ = tm.GetTraderIDs()
			}
		}()
	}

	// Writers: RemoveTrader (including repeated removal of the same ID)
	for g := 0; g < goroutinesPerKind; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				tm.RemoveTrader(ids[(seed+i)%traderCount])
			}
		}(g)
	}

	wg.Wait()

	if got := len(tm.GetTraderIDs()); got != 0 {
		t.Errorf("all traders should be removed after concurrent removal, %d left", got)
	}
}
