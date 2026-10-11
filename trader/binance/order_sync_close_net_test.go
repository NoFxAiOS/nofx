package binance

import (
	"math"
	"strings"
	"testing"
	"time"

	"nofx/store"
	"nofx/trader/types"
)

func assertCloseResult(t *testing.T, got, want closeFillResult) {
	t.Helper()
	if math.Abs(got.Net-want.Net) > 1e-9 || math.Abs(got.Gross-want.Gross) > 1e-9 ||
		math.Abs(got.Fee-want.Fee) > 1e-9 || got.Partial != want.Partial || got.EntryFeeKnown != want.EntryFeeKnown {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// A closed row already holds the whole position (every leg's PnL, every
// commission including the entry one), so the notice shows row PnL minus row fee.
func TestNetCloseFillClosedPosition(t *testing.T) {
	// XRPUSDT short 71.8 @1.3919 -> 1.3907: gross 0.08616; commissions
	// 0.01999 (open, maker) + 0.04993 (close) = 0.06992 on the row.
	trade := types.TradeRecord{Quantity: 71.8, RealizedPnL: 0.08616, Fee: 0.04993}
	pos := &store.TraderPosition{Status: "CLOSED", EntryQuantity: 71.8, RealizedPnL: 0.08616, Fee: 0.06992}
	assertCloseResult(t, netCloseFill(trade, pos, 0), closeFillResult{
		Net: 0.01624, Gross: 0.08616, Fee: 0.06992, EntryFeeKnown: true,
	})
}

// A partial close leaves the row OPEN: its Fee is the entry commission plus
// every close leg so far (this one included). The entry commission is prorated
// by the closed share of the entry quantity.
func TestNetCloseFillPartialCloseWithPriorLegs(t *testing.T) {
	// LONG 100 @1.40 (entry fee 0.02); closed 30 @1.45 (fee 0.009) and 30 @1.44 (fee 0.0086).
	pos := &store.TraderPosition{Status: "OPEN", EntryQuantity: 100, Quantity: 40, RealizedPnL: 2.7, Fee: 0.0376}
	trade := types.TradeRecord{Quantity: 30, RealizedPnL: 1.2, Fee: 0.0086}
	// entry share = (0.0376 - (0.009 + 0.0086)) * 30 / 100 = 0.006; fee = 0.0086 + 0.006
	assertCloseResult(t, netCloseFill(trade, pos, 0.009+0.0086), closeFillResult{
		Net: 1.1854, Gross: 1.2, Fee: 0.0146, Partial: true, EntryFeeKnown: true,
	})
}

func TestNetCloseFillFirstPartialClose(t *testing.T) {
	// Only this leg among the close fills: 0.0286 on the row = 0.02 entry + 0.0086 close.
	pos := &store.TraderPosition{Status: "OPEN", EntryQuantity: 100, Quantity: 70, RealizedPnL: 1.2, Fee: 0.0286}
	trade := types.TradeRecord{Quantity: 30, RealizedPnL: 1.2, Fee: 0.0086}
	assertCloseResult(t, netCloseFill(trade, pos, 0.0086), closeFillResult{
		Net: 1.1854, Gross: 1.2, Fee: 0.0146, Partial: true, EntryFeeKnown: true,
	})
}

// No position row: the fill's own commission, entry commission flagged as missing.
func TestNetCloseFillMissingPositionFallsBack(t *testing.T) {
	trade := types.TradeRecord{Quantity: 30, RealizedPnL: 1.2, Fee: 0.0086}
	assertCloseResult(t, netCloseFill(trade, nil, 0), closeFillResult{Net: 1.1914, Gross: 1.2, Fee: 0.0086})
}

// Inconsistent rows (the close legs already exceed the row's fee, or the entry
// quantity is missing) must not produce a made-up entry share.
func TestNetCloseFillInconsistentOpenRowFallsBack(t *testing.T) {
	trade := types.TradeRecord{Quantity: 30, RealizedPnL: 1.2, Fee: 0.0086}
	cases := []struct {
		name string
		pos  *store.TraderPosition
	}{
		{"close legs exceed row fee", &store.TraderPosition{Status: "OPEN", EntryQuantity: 100, Quantity: 40, Fee: 0.01}},
		{"no entry quantity", &store.TraderPosition{Status: "OPEN", EntryQuantity: 0, Quantity: 40, Fee: 0.0286}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCloseResult(t, netCloseFill(trade, c.pos, 0.02), closeFillResult{Net: 1.1914, Gross: 1.2, Fee: 0.0086})
		})
	}
}

func TestFormatCloseResultSingleSign(t *testing.T) {
	profit := formatCloseResult(closeFillResult{Net: 0.01624, Gross: 0.08616, Fee: 0.06992, EntryFeeKnown: true})
	if want := "已实现盈亏（扣手续费）\n✅ 盈利 <b>+0.0162 USDT</b>\n<i>毛利 +0.0862 · 手续费 -0.0699 · 未含资金费</i>"; profit != want {
		t.Fatalf("profit block:\n%s\nwant:\n%s", profit, want)
	}
	loss := formatCloseResult(closeFillResult{Net: -1.7, Gross: -1.5, Fee: 0.2, EntryFeeKnown: true})
	if want := "已实现盈亏（扣手续费）\n❌ 亏损 <b>-1.70 USDT</b>\n<i>毛利 -1.50 · 手续费 -0.20 · 未含资金费</i>"; loss != want {
		t.Fatalf("loss block:\n%s\nwant:\n%s", loss, want)
	}
	for _, out := range []string{profit, loss} {
		for _, bad := range []string{"++", "+-", "--", "+ -"} {
			if strings.Contains(out, bad) {
				t.Fatalf("double sign %q in:\n%s", bad, out)
			}
		}
	}
}

func TestFormatCloseResultPartialAndFallbackNotes(t *testing.T) {
	partial := formatCloseResult(closeFillResult{Net: 1.1854, Gross: 1.2, Fee: 0.0146, Partial: true, EntryFeeKnown: true})
	if want := "已实现盈亏（扣手续费）\n✅ 盈利 <b>+1.19 USDT</b>\n<i>部分平仓 · 毛利 +1.20 · 手续费 -0.0146 · 未含资金费</i>"; partial != want {
		t.Fatalf("partial block:\n%s\nwant:\n%s", partial, want)
	}
	fallback := formatCloseResult(closeFillResult{Net: 1.1914, Gross: 1.2, Fee: 0.0086})
	if want := "已实现盈亏（扣手续费）\n✅ 盈利 <b>+1.19 USDT</b>\n<i>毛利 +1.20 · 手续费 -0.0086 · 开仓手续费未计入 · 未含资金费</i>"; fallback != want {
		t.Fatalf("fallback block:\n%s\nwant:\n%s", fallback, want)
	}
}

// Small figures keep 4 decimals so they never print as 0.00; -0 prints as +0.
func TestSignedAmountPrecisionAndSign(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0.01624, "+0.0162"},
		{-0.06992, "-0.0699"},
		{0.08616, "+0.0862"},
		{0.1, "+0.10"},
		{12.5, "+12.50"},
		{-1.7, "-1.70"},
		{0, "+0.0000"},
		{math.Copysign(0, -1), "+0.0000"},
	}
	for _, c := range cases {
		if got := signedAmount(c.in); got != c.want {
			t.Errorf("signedAmount(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Real sync path: the XRPUSDT short from the bug report is booked by the
// position builder, then the notice looks the closed row up and nets it.
func TestCloseNoticeNetsEntryCommissionAfterSync(t *testing.T) {
	st := batch2SyncStore(t)
	ms := time.Now().Add(-time.Minute).UnixMilli()
	trades := []map[string]interface{}{
		batch2SyncTrade(1, "XRPUSDT", "SELL", "SHORT", 71.8, 1.3919, 0, 0.01999, ms),
		batch2SyncTrade(2, "XRPUSDT", "BUY", "SHORT", 71.8, 1.3907, 0.08616, 0.04993, ms+1),
	}
	tr := batch1FuturesTrader(batch2SyncTransport(trades))
	if err := tr.SyncOrdersFromBinance("net-trader", t.Name(), "binance", st); err != nil {
		t.Fatal(err)
	}
	closeFill := types.TradeRecord{
		Symbol: "XRPUSDT", Side: "BUY", PositionSide: "SHORT", Quantity: 71.8, Price: 1.3907,
		RealizedPnL: 0.08616, Fee: 0.04993, Time: time.UnixMilli(ms + 1).UTC(),
	}
	got := closeResultBlock(st, "net-trader", "XRPUSDT", "close_short", closeFill)
	if want := "已实现盈亏（扣手续费）\n✅ 盈利 <b>+0.0162 USDT</b>\n<i>毛利 +0.0862 · 手续费 -0.0699 · 未含资金费</i>"; got != want {
		t.Fatalf("closed-position notice:\n%s\nwant:\n%s", got, want)
	}
}

// Partial close (LONG) through the real sync: the row stays OPEN, so the last
// leg prorates the entry commission and the notice says so.
func TestCloseNoticePartialCloseAfterSync(t *testing.T) {
	st := batch2SyncStore(t)
	ms := time.Now().Add(-time.Minute).UnixMilli()
	trades := []map[string]interface{}{
		batch2SyncTrade(1, "SOLUSDT", "BUY", "LONG", 100, 1.40, 0, 0.02, ms),
		batch2SyncTrade(2, "SOLUSDT", "SELL", "LONG", 30, 1.45, 1.5, 0.009, ms+1),
		batch2SyncTrade(3, "SOLUSDT", "SELL", "LONG", 30, 1.44, 1.2, 0.0086, ms+2),
	}
	tr := batch1FuturesTrader(batch2SyncTransport(trades))
	if err := tr.SyncOrdersFromBinance("net-trader", t.Name(), "binance", st); err != nil {
		t.Fatal(err)
	}
	last := types.TradeRecord{
		Symbol: "SOLUSDT", Side: "SELL", PositionSide: "LONG", Quantity: 30, Price: 1.44,
		RealizedPnL: 1.2, Fee: 0.0086, Time: time.UnixMilli(ms + 2).UTC(),
	}
	got := closeResultBlock(st, "net-trader", "SOLUSDT", "close_long", last)
	if want := "已实现盈亏（扣手续费）\n✅ 盈利 <b>+1.19 USDT</b>\n<i>部分平仓 · 毛利 +1.20 · 手续费 -0.0146 · 未含资金费</i>"; got != want {
		t.Fatalf("partial-close notice:\n%s\nwant:\n%s", got, want)
	}
}

// No row for the symbol: the notice says the entry commission is not counted.
func TestCloseNoticeFallsBackWithoutPosition(t *testing.T) {
	st := batch2SyncStore(t)
	trade := types.TradeRecord{
		Symbol: "NOPEUSDT", Side: "BUY", PositionSide: "SHORT", Quantity: 71.8, Price: 1.3907,
		RealizedPnL: 0.08616, Fee: 0.04993, Time: time.Now().UTC(),
	}
	got := closeResultBlock(st, "net-trader", "NOPEUSDT", "close_short", trade)
	if want := "已实现盈亏（扣手续费）\n✅ 盈利 <b>+0.0362 USDT</b>\n<i>毛利 +0.0862 · 手续费 -0.0499 · 开仓手续费未计入 · 未含资金费</i>"; got != want {
		t.Fatalf("fallback notice:\n%s\nwant:\n%s", got, want)
	}
}
