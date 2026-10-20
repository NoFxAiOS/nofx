package manager

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"nofx/store"
)

func TestPaperExchangeManagerMapping(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "paper.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := store.PaperConfig{InitialBalance: 3210, TakerFeeBps: 9, MakerFeeBps: 4, SlippageBps: 6, DefaultLeverage: 11}
	exchangeID, err := st.Exchange().CreatePaper("u", "candidate", true, cfg)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(&store.StrategyConfig{})
	if err := st.Strategy().Create(&store.Strategy{ID: "s", UserID: "u", Name: "test", Config: string(data)}); err != nil {
		t.Fatal(err)
	}
	ex, _ := st.Exchange().GetByID("u", exchangeID)
	row := &store.Trader{ID: t.Name(), UserID: "u", Name: "paper", StrategyID: "s", InitialBalance: 99999}
	tm := NewTraderManager()
	if err := tm.addTraderFromStore(row, &store.AIModel{Provider: "deepseek"}, ex, st); err != nil {
		t.Fatal(err)
	}
	at, err := tm.GetTrader(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(at.Stop)
	if at.InitialBalance() != 3210 {
		t.Fatalf("baseline=%f", at.InitialBalance())
	}
	ledger, err := st.PaperLedger().Get(exchangeID)
	if err != nil || ledger.WalletBalance != 3210 || ledger.ExchangeID != ex.ID {
		t.Fatalf("ledger=%+v err=%v", ledger, err)
	}
	// Parsing errors must stop construction rather than silently use live/default settings.
	ex.PaperConfig = `{"initial_balance":-1}`
	row.ID = "invalid"
	if err := tm.addTraderFromStore(row, &store.AIModel{Provider: "deepseek"}, ex, st); err == nil {
		t.Fatal("accepted invalid configuration")
	}
}
