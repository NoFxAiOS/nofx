package trader

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/store"
	"nofx/trader/paper"
)

type paperTestPrices struct{ mark float64 }

func (p *paperTestPrices) Price(string) (float64, error)                     { return p.mark, nil }
func (*paperTestPrices) Candles1m(string, time.Time) ([]paper.Candle, error) { return nil, nil }
func (*paperTestPrices) QuantityRules(string) (paper.QuantityRule, error) {
	return paper.QuantityRule{StepSize: "0.001", MinQty: 0.001, MinNotional: 1}, nil
}

func TestIsBinanceFuturesLike(t *testing.T) {
	for _, exchange := range []string{"binance", "paper", "bybit", "binance_stocks", "", "BINANCE"} {
		want := exchange == "binance" || exchange == "paper"
		if isBinanceFuturesLike(exchange) != want {
			t.Fatal(exchange)
		}
		at := &AutoTrader{exchange: exchange, config: AutoTraderConfig{IsCrossMargin: true}}
		if at.entryUsesCrossMargin() == want {
			t.Fatalf("wrong margin mode: %s", exchange)
		}
	}
}

func TestPaperAutoTraderSmoke(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "paper.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := AutoTraderConfig{ID: t.Name(), Name: "Paper smoke", Exchange: "paper", ExchangeID: t.Name(), InitialBalance: 99999,
		PaperConfig:      &paper.Config{ExchangeID: "ignored-client-id", InitialBalance: 2000, TakerFeeBps: 8, MakerFeeBps: 3, SlippageBps: 4, DefaultLeverage: 7},
		PaperPriceSource: &paperTestPrices{mark: 100}, StrategyConfig: &store.StrategyConfig{}}
	at, err := NewAutoTrader(cfg, st, "u")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(at.Stop)
	p, ok := at.trader.(*paper.PaperFuturesTrader)
	if !ok {
		t.Fatalf("executor %T", at.trader)
	}
	if at.InitialBalance() != 2000 || at.binanceRealtimeCapable() {
		t.Fatal("baseline or stream capability incorrect")
	}
	if err := p.SyncOrdersFromBinance(at.id, at.exchangeID, "paper", st); err != nil {
		t.Fatal(err)
	}
	if _, err := p.OpenLong("BTCUSDT", 1, 7); err != nil {
		t.Fatal(err)
	}
	balance, err := p.GetBalance()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(balance["totalWalletBalance"].(float64)-(2000-100.04*8/10000)) > 1e-8 {
		t.Fatalf("fees: %v", balance)
	}
	if _, err := p.CloseLong("BTCUSDT", 1); err != nil {
		t.Fatal(err)
	}
	positions, err := p.GetPositions()
	if err != nil || len(positions) != 0 {
		t.Fatalf("positions %v %v", positions, err)
	}
	var fills int64
	if err := st.GormDB().Model(&store.TraderFill{}).Where("exchange_id = ?", at.exchangeID).Count(&fills).Error; err != nil || fills != 2 {
		t.Fatalf("fills=%d err=%v", fills, err)
	}
	// recordAndConfirmOrder must not add a second accounting row for an already committed fill.
	at.recordAndConfirmOrder(map[string]interface{}{"orderId": "1"}, "BTCUSDT", "open_long", 1, 100, 7, 0)
	var orders int64
	st.GormDB().Model(&store.TraderOrder{}).Where("exchange_id = ?", at.exchangeID).Count(&orders)
	if orders != 2 {
		t.Fatalf("duplicate orders: %d", orders)
	}
	cfg.PaperConfig.InitialBalance = 5000
	restored, err := NewAutoTrader(cfg, st, "u")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restored.Stop)
	if restored.InitialBalance() != 2000 {
		t.Fatalf("restart baseline %f", restored.InitialBalance())
	}
	cfg.ID = "another-trader"
	if _, err := NewAutoTrader(cfg, st, "u"); err == nil {
		t.Fatal("another trader took over the account")
	}
	cfg.ExchangeID = "independent-control"
	control, err := NewAutoTrader(cfg, st, "u")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(control.Stop)
	info, err := control.GetAccountInfo()
	if err != nil || info["total_equity"] != float64(5000) {
		t.Fatalf("independent control: %v %v", info, err)
	}
}

func TestPaperSymbolRiskParity(t *testing.T) {
	for _, exchange := range []string{"binance", "paper"} {
		at := &AutoTrader{exchange: exchange, config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{RiskControl: store.RiskControlConfig{RiskPerTradePct: 1}}}}
		d := &kernel.Decision{Symbol: "BTCUSDT", Action: "open_long", StopLoss: 90, PositionSizeUSD: 200}
		ctx := &kernel.Context{}
		if blocked, _, _ := at.accountRiskExposureBlocks(d, 100, ctx); !blocked {
			t.Fatalf("%s accepted zero equity", exchange)
		}
		ctx.Account.TotalEquity = 1000
		at.cycleSymbolRiskReservedUSD = map[string]float64{"BTCUSDT": 1}
		if blocked, _, _ := at.accountRiskExposureBlocks(d, 100, ctx); !blocked {
			t.Fatalf("%s accepted symbol risk > 1%%", exchange)
		}
	}
}

func TestPaperMarketDataVenue(t *testing.T) {
	for _, exchange := range []string{"paper", "binance", "bybit", "okx", "hyperliquid"} {
		at := &AutoTrader{exchange: exchange}
		want := exchange
		if exchange == "paper" {
			want = "binance"
		}
		if got := at.marketDataExchange(); got != want {
			t.Fatalf("%s market venue = %s, want %s", exchange, got, want)
		}
		if at.GetExchange() != exchange {
			t.Fatal("execution identity changed")
		}
	}
}
