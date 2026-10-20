package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"nofx/config"
	"nofx/store"
)

func paperExchangeRequest(srv *Server, user, method, path string, body interface{}) *httptest.ResponseRecorder {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", user) })
	r.POST("/exchanges", srv.handleCreateExchange)
	r.PUT("/exchanges", srv.handleUpdateExchangeConfigs)
	r.GET("/exchanges", srv.handleGetExchangeConfigs)
	r.GET("/supported", srv.handleGetSupportedExchanges)
	r.POST("/traders", srv.handleCreateTrader)
	r.PUT("/traders/:id", srv.handleUpdateTrader)
	data, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

func TestPaperExchangeAPI(t *testing.T) {
	srv, user, _ := syncBalanceFixture(t, true)
	old := config.Get().TransportEncryption
	config.Get().TransportEncryption = false
	t.Cleanup(func() { config.Get().TransportEncryption = old })
	cfg := store.DefaultPaperConfig()
	rec := paperExchangeRequest(srv, user, http.MethodPost, "/exchanges", CreateExchangeRequest{ExchangeType: "paper", AccountName: "candidate", Enabled: true, PaperConfig: &cfg, APIKey: "ignored", SecretKey: "ignored"})
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	ex, err := srv.store.Exchange().GetByID(user, result.ID)
	if err != nil || ex.APIKey != "" || ex.SecretKey != "" || ex.ExchangeType != "paper" {
		t.Fatalf("account: %+v %v", ex, err)
	}
	if message, _, _ := validateExchangeForTraderCreation(ex); message != "" {
		t.Fatal(message)
	}
	if fields := missingExchangeFields(ex); len(fields) > 0 {
		t.Fatal(fields)
	}
	state := probeExchangeAccountState(ex, user, srv.store)
	if state.Status != "ok" || state.TotalEquity != 1000 || state.AvailableBalance != 1000 {
		t.Fatalf("unstarted account: %+v", state)
	}
	cfg.InitialBalance = 2500
	update := UpdateExchangeConfigRequest{Exchanges: map[string]ExchangeConfigPayload{ex.ID: {Enabled: true, AccountName: "renamed", PaperConfig: &cfg}}}
	rec = paperExchangeRequest(srv, user, http.MethodPut, "/exchanges", update)
	if rec.Code != 200 {
		t.Fatalf("update: %s", rec.Body.String())
	}
	rec = paperExchangeRequest(srv, user, http.MethodGet, "/exchanges", nil)
	var accounts []SafeExchangeConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &accounts); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, account := range accounts {
		if account.ID == ex.ID {
			found = true
			if account.PaperConfig == nil || account.PaperConfig.InitialBalance != 2500 || account.AccountName != "renamed" {
				t.Fatalf("safe config: %+v", account)
			}
		}
	}
	if !found {
		t.Fatal("paper omitted from safe list")
	}
	rec = paperExchangeRequest(srv, "other", http.MethodPut, "/exchanges", update)
	if rec.Code != 404 {
		t.Fatalf("cross-user update: %d", rec.Code)
	}
	rec = paperExchangeRequest(srv, user, http.MethodGet, "/supported", nil)
	if !strings.Contains(rec.Body.String(), `"exchange_type":"paper"`) {
		t.Fatal("paper template missing")
	}
	// The legacy plaintext fixture key is never sent: construction does not call the model.
	if err := srv.store.GormDB().Exec("UPDATE ai_models SET api_key = ? WHERE id = ? AND user_id = ?", "test-model-key", "model-sync", user).Error; err != nil {
		t.Fatal(err)
	}
	// Real trader creation traverses the manager without any exchange credential/balance probe.
	if err := srv.store.Strategy().Create(&store.Strategy{ID: "paper-strategy", UserID: user, Name: "test", Config: `{}`}); err != nil {
		t.Fatal(err)
	}
	rec = paperExchangeRequest(srv, user, http.MethodPost, "/traders", CreateTraderRequest{Name: "candidate", AIModelID: "model-sync", ExchangeID: ex.ID, StrategyID: "paper-strategy", InitialBalance: 99999})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create trader: %d %s", rec.Code, rec.Body.String())
	}
	traders, _ := srv.store.Trader().ListByExchangeID(user, ex.ID)
	if len(traders) != 1 || traders[0].InitialBalance != 2500 {
		t.Fatalf("traders: %+v", traders)
	}
	at, err := srv.traderManager.GetTrader(traders[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(at.Stop)
	rec = paperExchangeRequest(srv, user, http.MethodPost, "/traders", CreateTraderRequest{Name: "control", AIModelID: "model-sync", ExchangeID: ex.ID, StrategyID: "paper-strategy"})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "只能绑定一个交易员") {
		t.Fatalf("shared paper account: %d %s", rec.Code, rec.Body.String())
	}
	rec = paperExchangeRequest(srv, user, http.MethodPut, "/traders/trader-sync", UpdateTraderRequest{ExchangeID: ex.ID})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "只能绑定一个交易员") {
		t.Fatalf("shared paper update: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPaperExchangeAPIValidation(t *testing.T) {
	srv, user, _ := syncBalanceFixture(t, true)
	old := config.Get().TransportEncryption
	config.Get().TransportEncryption = false
	t.Cleanup(func() { config.Get().TransportEncryption = old })
	for _, mutate := range []func(*store.PaperConfig){
		func(c *store.PaperConfig) { c.InitialBalance = 0 }, func(c *store.PaperConfig) { c.InitialBalance = -1 },
		func(c *store.PaperConfig) { c.TakerFeeBps = -0.1 }, func(c *store.PaperConfig) { c.MakerFeeBps = 100.1 },
		func(c *store.PaperConfig) { c.SlippageBps = 101 }, func(c *store.PaperConfig) { c.DefaultLeverage = 0 }, func(c *store.PaperConfig) { c.DefaultLeverage = 126 },
	} {
		cfg := store.DefaultPaperConfig()
		mutate(&cfg)
		rec := paperExchangeRequest(srv, user, http.MethodPost, "/exchanges", CreateExchangeRequest{ExchangeType: "paper", PaperConfig: &cfg})
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "模拟盘") {
			t.Fatalf("invalid config: %+v %d %s", cfg, rec.Code, rec.Body.String())
		}
	}
	rec := paperExchangeRequest(srv, user, http.MethodPost, "/exchanges", CreateExchangeRequest{ExchangeType: "paper", Enabled: true})
	if rec.Code != 200 {
		t.Fatalf("defaults: %s", rec.Body.String())
	}
	var result struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &result)
	cfg := store.DefaultPaperConfig()
	cfg.TakerFeeBps = 0
	cfg.MakerFeeBps = 100
	cfg.DefaultLeverage = 125
	rec = paperExchangeRequest(srv, user, http.MethodPut, "/exchanges", UpdateExchangeConfigRequest{Exchanges: map[string]ExchangeConfigPayload{result.ID: {Enabled: true, PaperConfig: &cfg}}})
	if rec.Code != 200 {
		t.Fatalf("inclusive bounds: %s", rec.Body.String())
	}
	cfg.InitialBalance = -1
	rec = paperExchangeRequest(srv, user, http.MethodPut, "/exchanges", UpdateExchangeConfigRequest{Exchanges: map[string]ExchangeConfigPayload{result.ID: {Enabled: true, PaperConfig: &cfg}}})
	if rec.Code != 400 {
		t.Fatalf("update validation: %s", rec.Body.String())
	}
	ex, _ := srv.store.Exchange().GetByID(user, result.ID)
	got, _ := ex.ParsePaperConfig()
	if got.InitialBalance != 1000 {
		t.Fatal("invalid update changed storage")
	}
}

func TestPaperAccountStateUsesLedger(t *testing.T) {
	srv, user, _ := syncBalanceFixture(t, true)
	id, err := srv.store.Exchange().CreatePaper(user, "ledger", true, store.DefaultPaperConfig())
	if err != nil {
		t.Fatal(err)
	}
	// Use the real ledger schema: simultaneous long/short margin is gross, PnL is signed.
	_, err = srv.store.PaperLedger().Ensure(&store.PaperLedger{ExchangeID: id, InitialBalance: 1000, WalletBalance: 980, Version: 1, State: `{"Marks":{"BTCUSDT":110},"Positions":{"long":{"Symbol":"BTCUSDT","Side":"LONG","Quantity":2,"EntryPrice":100,"Leverage":5},"short":{"Symbol":"BTCUSDT","Side":"SHORT","Quantity":1,"EntryPrice":120,"Leverage":10}}}`})
	if err != nil {
		t.Fatal(err)
	}
	ex, _ := srv.store.Exchange().GetByID(user, id)
	state := probeExchangeAccountState(ex, user, srv.store)
	if state.Status != "ok" || state.TotalEquity != 1010 || state.UnrealizedPnL != 30 || state.AvailableBalance != 925 {
		t.Fatalf("snapshot: %+v", state)
	}
	states, err := srv.getExchangeAccountStates(user)
	if err != nil || states[id].TotalEquity != 1010 {
		t.Fatalf("states: %v %v", states, err)
	}
	ex.Enabled = false
	if state := probeExchangeAccountState(ex, user, srv.store); state.Status != "disabled" {
		t.Fatal(state)
	}
}
