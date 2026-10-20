package store

import (
	"math"
	"testing"
)

func TestPaperExchangeMigrationAndCRUD(t *testing.T) {
	st := newTestStore(t)
	// Simulate a database from before PaperConfig existed, preserving a live account.
	id, err := st.Exchange().Create("u", "binance", "live", true, "", "", "", false, "", false, "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.gdb.Migrator().DropColumn(&Exchange{}, "PaperConfig"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := st.Exchange().initTables(); err != nil {
			t.Fatal(err)
		}
	}
	live, err := st.Exchange().GetByID("u", id)
	if err != nil || live.PaperConfig != "" || live.AccountName != "live" {
		t.Fatalf("migration: %+v %v", live, err)
	}
	cfg := DefaultPaperConfig()
	first, err := st.Exchange().CreatePaper("u", "candidate", true, cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.Exchange().CreatePaper("u", "control", true, cfg)
	if err != nil || first == second {
		t.Fatalf("independent accounts: %s %s %v", first, second, err)
	}
	ex, err := st.Exchange().GetByID("u", first)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ex.ParsePaperConfig()
	if err != nil || got != cfg || ex.Type != "paper" || ex.Name != "Paper Futures (模拟盘)" || ex.APIKey != "" {
		t.Fatalf("read: %+v %+v %v", ex, got, err)
	}
	cfg.MakerFeeBps, cfg.TakerFeeBps, cfg.SlippageBps, cfg.DefaultLeverage = 0, 100, 0, 125
	if err := st.Exchange().UpdatePaperConfig("other", first, "bad", false, cfg); err == nil {
		t.Fatal("cross-user update allowed")
	}
	if err := st.Exchange().UpdatePaperConfig("u", id, "bad", false, cfg); err == nil {
		t.Fatal("live exchange converted")
	}
	if err := st.Exchange().UpdatePaperConfig("u", first, "renamed", false, cfg); err != nil {
		t.Fatal(err)
	}
	ex, _ = st.Exchange().GetByID("u", first)
	got, err = ex.ParsePaperConfig()
	if err != nil || got != cfg || ex.Enabled || ex.AccountName != "renamed" {
		t.Fatalf("update: %+v %+v %v", ex, got, err)
	}
	accounts, err := st.Exchange().List("u")
	if err != nil || len(accounts) != 3 {
		t.Fatalf("list: %v %v", accounts, err)
	}
	if err := st.Exchange().Delete("other", first); err == nil {
		t.Fatal("cross-user deletion allowed")
	}
	if err := st.Exchange().Delete("u", first); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Exchange().GetByID("u", first); err == nil {
		t.Fatal("deleted account remains")
	}
}

func TestPaperConfigValidation(t *testing.T) {
	for _, mutate := range []func(*PaperConfig){
		func(c *PaperConfig) { c.InitialBalance = 0 }, func(c *PaperConfig) { c.InitialBalance = math.NaN() },
		func(c *PaperConfig) { c.TakerFeeBps = -1 }, func(c *PaperConfig) { c.MakerFeeBps = 101 },
		func(c *PaperConfig) { c.SlippageBps = math.Inf(1) }, func(c *PaperConfig) { c.DefaultLeverage = 0 }, func(c *PaperConfig) { c.DefaultLeverage = 126 },
	} {
		cfg := DefaultPaperConfig()
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("accepted invalid config: %+v", cfg)
		}
	}
	if _, err := (&Exchange{PaperConfig: "{"}).ParsePaperConfig(); err == nil {
		t.Fatal("accepted malformed JSON")
	}
}
