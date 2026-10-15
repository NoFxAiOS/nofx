"""验证策略适配确实使用 Backtrader 的订单执行和账务结果。"""
import unittest

from scripts.backtest.backtrader_adapter import run_trade
from scripts.backtest.exitsim import build_trade, simulate, summarize
from scripts.backtest.params import FALLBACK

MIN = 60_000
BASE = 1_700_000_040_000


def P(**kw):
    return dict(FALLBACK, **dict(dict(be_arm=0, lock_r=0, lock_trim=False, trim_r=0,
                                    trail=False, pd_arm=0, tp_cap=0, quick_stop_h=0), **kw))


def T(rows, side="long", kind="mkt", **signal):
    sig = dict(symbol="X", side=side, kind="limit", outcome="unfilled", t0="test", t0_ms=BASE,
               px0=99., sl0=95. if side == "long" else 105., tp0=110. if side == "long" else 90.)
    sig.update(signal)
    bars = [[BASE + i * MIN, *row] for i, row in enumerate(rows)]
    return build_trade(sig, bars, kind)


class BacktraderExecutionTest(unittest.TestCase):
    def run_trade(self, rows, rules=None, **signal):
        return run_trade(T(rows, **signal), rules or P(), commission=0, slippage=0)

    def test_market_fills_next_open_and_native_analyzer_matches_pnl(self):
        r = self.run_trade([(100, 100, 100, 100), (102, 104, 101, 103), (103, 111, 102, 110)])
        self.assertEqual((r["entry_price"], r["entry_ms"]), (102, BASE + MIN))
        self.assertEqual((r["how"], r["net_pnl"]), ("tp", 8))
        self.assertAlmostEqual(r["r"], 8 / 7)
        self.assertAlmostEqual(r["trade_analysis"]["pnl"]["net"]["total"], r["net_pnl"])
        self.assertEqual(r["engine"], "backtrader")
        self.assertEqual(r["exit_ms"], BASE + 3 * MIN)

    def test_gap_stop_fills_at_open_not_requested_stop(self):
        r = self.run_trade([(100, 100, 100, 100), (100, 101, 99, 100), (90, 92, 88, 89)])
        self.assertEqual((r["how"], r["r"]), ("sl", -2))

    def test_existing_stop_precedes_tp_on_same_bar(self):
        r = self.run_trade([(100, 100, 100, 100), (100, 101, 99, 100), (100, 112, 94, 100)])
        self.assertEqual((r["how"], r["r"]), ("sl", -1))
        self.assertFalse(r["open"])

    def test_native_commission_on_each_partial_fill_and_no_reversal(self):
        tr = T([(100, 100, 100, 100), (100, 101, 99, 100), (100, 111, 99, 110),
                (110, 111, 109, 110), (94, 95, 93, 94)])
        r = run_trade(tr, P(trail=True, trail_arm=999), commission=0.0005, slippage=0)
        expected_fees = (100 + 110 * 0.5 + 94 * 0.5) * 0.0005
        self.assertAlmostEqual(r["commission"], expected_fees)
        self.assertAlmostEqual(r["net_pnl"], 5 - 3 - expected_fees)
        self.assertFalse(r["open"])
        self.assertEqual(r["trade_analysis"]["total"]["closed"], 1)

    def test_breakeven_stop_is_effective_after_signal_bar_closes(self):
        r = self.run_trade([(100, 100, 100, 100), (100, 104, 99, 103), (101, 102, 99, 100)],
                           P(be_arm=0.75, be_off=0.1))
        self.assertEqual(r["how"], "be")
        self.assertAlmostEqual(r["r"], 0.1)

    def test_peak_drawdown_does_not_fill_retroactively_in_same_bar(self):
        r = self.run_trade([(100, 100, 100, 100), (100, 112, 99, 105), (104, 105, 103, 104)],
                           P(pd_arm=2, pd_give=0.5, use_tp=False))
        self.assertEqual(r["how"], "pd")
        self.assertAlmostEqual(r["r"], 0.8)  # 下一根开盘104，而非上一根虚构成交106

    def test_native_slippage_changes_market_fill(self):
        tr = T([(100, 100, 100, 100), (100, 102, 99, 100), (100, 101, 99, 100)])
        r = run_trade(tr, P(), commission=0, slippage=0.01)
        self.assertEqual(r["entry_price"], 101)

    def test_low_price_asset_keeps_r_precision(self):
        tr = T([(100, 100, 100, 100), (100, 101, 99, 100), (100, 111, 99, 110)])
        scale = 1e-9
        tr["bars"] = [(t, o * scale, h * scale, l * scale, c * scale, ot)
                      for t, o, h, l, c, ot in tr["bars"]]
        for key in ("reference", "sl", "tp"):
            tr[key] *= scale
        r = run_trade(tr, P(), commission=0, slippage=0)
        self.assertAlmostEqual(r["r"], 2, places=10)

    def test_open_position_only_pays_executed_entry_fee(self):
        tr = T([(100, 100, 100, 100), (100, 102, 99, 101)])
        r = run_trade(tr, P(), commission=0.0005, slippage=0)
        self.assertEqual(r["how"], "open")
        self.assertTrue(r["open"])
        self.assertAlmostEqual(r["net_pnl"], 1 - 0.05)
        self.assertAlmostEqual(r["commission"], 0.05)

    def test_horizon_submits_market_close_for_next_bar(self):
        r = self.run_trade([(100, 100, 100, 100), (100, 101, 99, 100), (102, 103, 101, 102)],
                           P(horizon_h=1 / 60))
        self.assertEqual((r["how"], r["net_pnl"]), ("time", 2))

    def test_limit_not_filled_has_no_fees_or_pnl(self):
        tr = T([(100, 100, 100, 100)] * 5, kind="lim", px0=98., sl0=95.)
        tr["limit_valid_min"] = 2
        r = run_trade(tr, P())
        self.assertEqual((r["r"], r["how"], r["commission"], r["net_pnl"]), (None, "unfilled", 0, 0))

    def test_historical_fill_label_does_not_control_simulated_limit_fill(self):
        rows = [(100, 100, 100, 100), (100, 101, 98, 100), (100, 111, 99, 110)]
        a = self.run_trade(rows, kind="lim", outcome="unfilled", fill_px=None)
        b = self.run_trade(rows, kind="lim", outcome="filled", fill_px=1000, sl_last=1, tp_last=9999)
        self.assertEqual((a["entry_price"], a["r"]), (b["entry_price"], b["r"]))
        self.assertEqual(a["entry_price"], 99)

    def test_short_order_direction(self):
        rows = [(100, 100, 100, 100), (100, 101, 99, 100), (100, 102, 89, 90)]
        r = self.run_trade(rows, side="short")
        self.assertEqual((r["how"], r["r"]), ("tp", 2))

    def test_overlapping_signals_and_unfilled_counts(self):
        rows = [(100, 100, 100, 100)] * 5
        a = T(rows)
        b = dict(a, start_ms=a["start_ms"] + MIN, t0_ms=a["t0_ms"] + MIN)
        s = summarize([b, a], P(horizon_h=1), commission=0, slippage=0)
        self.assertEqual((s["n"], s["blocked"]), (1, 1))
        s = summarize([a, b], P(), allow_overlap=True, commission=0, slippage=0)
        self.assertEqual((s["n"], s["blocked"]), (2, 0))
        tr = T(rows, kind="lim", px0=98)
        self.assertEqual(summarize([tr], P())["unfilled"], 1)

    def test_invalid_data_and_missing_signal_minute_are_skipped(self):
        sig = dict(symbol="X", side="long", t0_ms=BASE, t0="x", sl0=95, tp0=110)
        self.assertIsNone(build_trade(sig, [[BASE + MIN, 100, 101, 99, 100]], "mkt"))
        self.assertIsNone(build_trade(sig, [[BASE, 100, 99, 101, 100], [BASE + MIN, 100, 101, 99, 100]], "mkt"))

    def test_mixed_timeframes_use_close_times_and_reject_overlap(self):
        tr = T([(100, 100, 100, 100)] * 3)
        sig = dict(symbol="X", side="long", t0_ms=BASE, t0="x", sl0=95, tp0=110)
        bars = [[BASE, 100, 100, 100, 100], [BASE + MIN, 100, 100, 100, 100]]
        converted = build_trade(sig, bars, "mkt", n1m=1)
        self.assertEqual(converted["bars"][1][0], BASE + 6 * MIN)
        bars[1][0] = BASE
        self.assertIsNone(build_trade(sig, bars, "mkt", n1m=1))


if __name__ == "__main__":
    unittest.main()
