import unittest
from datetime import datetime, timezone

from scripts.backtest.exitsim import SLIP, TAKER, build_trade, simulate, summarize

MIN = 60_000
HOUR = 3_600_000


def P(**kw):
    """全关的 plain 参数,按需打开某一项。"""
    p = dict(be_arm=0, be_off=0.1, lock_r=0, lock_trim=False, trim_r=0, trim_frac=1 / 3, tp_cap=0,
             tp_frac_trend=0.5, trail=False, trail_arm=1.5, trail_atr=2.0, pd_arm=0, pd_give=0.5,
             quick_stop_h=0, force_mode=None, use_tp=True, horizon_h=72)
    p.update(kw)
    return p


def T(bars, tp_r=3.0, atr_r=0.0, mode="trend", fee_r=0.0):
    """bars: [(t_ms, hi, lo, close)](R 空间)。"""
    return dict(bars=bars, tp_r=tp_r, atr_r=atr_r, mode=mode, fee_r=fee_r, sym="X", side="long",
                t0="", t0_ms=0, risk_pct=1.0)


class ExitSimTest(unittest.TestCase):
    def test_same_bar_sl_and_tp_counts_as_sl(self):
        r, how, _, _ = simulate(T([(0, 3.5, -1.2, 0.0)]), P())
        self.assertEqual((r, how), (-1.0, "sl"))

    def test_range_mode_full_tp(self):
        r, how, _, _ = simulate(T([(0, 2.5, 0.0, 2.0)], tp_r=2.0, mode="range", atr_r=0.5), P(trail=True))
        self.assertAlmostEqual(r, 2.0)
        self.assertEqual(how, "tp")

    def test_trend_half_tp_then_trail_exit(self):
        bars = [(0, 2.2, -0.1, 2.0), (MIN, 2.0, 1.0, 1.1)]
        r, how, _, _ = simulate(T(bars, tp_r=2.0, atr_r=0.5), P(trail=True))
        # 50% 在 2R 止盈 = 1.0;追踪线 = 2.2 - 2*0.5 = 1.2,剩余 50% 在 1.2 出 = 0.6
        self.assertAlmostEqual(r, 1.6)
        self.assertEqual(how, "trail")

    def test_trend_tp_full_when_trailing_off(self):
        r, how, _, _ = simulate(T([(0, 2.5, 0.0, 2.0)], tp_r=2.0), P(trail=False))
        self.assertAlmostEqual(r, 2.0)
        self.assertEqual(how, "tp")

    def test_breakeven_tier_then_stopout_yields_be_off(self):
        bars = [(0, 0.8, -0.2, 0.5), (MIN, 0.3, -0.5, -0.4)]
        r, how, _, _ = simulate(T(bars), P(be_arm=0.75, be_off=0.1))
        self.assertAlmostEqual(r, 0.1)
        self.assertEqual(how, "be")

    def test_be_not_armed_below_threshold(self):
        bars = [(0, 0.7, -0.2, 0.5), (MIN, 0.3, -1.5, -1.4)]
        r, how, _, _ = simulate(T(bars), P(be_arm=0.75))
        self.assertAlmostEqual(r, -1.0)
        self.assertEqual(how, "sl")

    def test_lock_without_trim_only_moves_stop(self):
        bars = [(0, 1.6, 0.0, 1.5), (MIN, 0.2, -0.1, 0.0)]
        r, how, _, _ = simulate(T(bars), P(lock_r=1.5, lock_trim=False, be_off=0.1))
        self.assertAlmostEqual(r, 0.1)
        self.assertEqual(how, "be")

    def test_lock_trim_closes_half_once(self):
        bars = [(0, 1.6, 0.0, 1.5), (MIN, 1.7, 0.5, 1.0), (2 * MIN, 0.2, -0.1, 0.0)]
        r, how, _, _ = simulate(T(bars), P(lock_r=1.5, lock_trim=True, be_off=0.1))
        # 50% 在 1.5R = 0.75;剩余 50% 止损在 0.1 = 0.05
        self.assertAlmostEqual(r, 0.8)
        self.assertEqual(how, "be")

    def test_trim_tier(self):
        bars = [(0, 1.3, 0.2, 1.0), (MIN, 0.5, -1.5, -1.2)]
        r, how, _, _ = simulate(T(bars), P(trim_r=1.2, trim_frac=1 / 3))
        # 1/3 在 1.2R = 0.4;剩余 2/3 在 -1R
        self.assertAlmostEqual(r, 0.4 - 2 / 3)
        self.assertEqual(how, "sl")

    def test_peak_drawdown_exit_level(self):
        bars = [(0, 2.4, 1.0, 1.5)]
        r, how, peak, _ = simulate(T(bars, tp_r=5.0), P(pd_arm=2.0, pd_give=0.5))
        self.assertAlmostEqual(r, 1.2)  # peak 2.4 * (1-0.5)
        self.assertEqual(how, "pd")
        self.assertAlmostEqual(peak, 2.4)

    def test_peak_drawdown_not_armed_below_arm(self):
        bars = [(0, 1.9, 0.5, 1.0)]
        _, how, _, _ = simulate(T(bars, tp_r=5.0), P(pd_arm=2.0))
        self.assertEqual(how, "open")

    def test_quick_time_stop(self):
        bars = [(0, 0.2, -0.2, 0.1), (4 * HOUR, 0.1, -0.4, -0.3)]
        r, how, _, _ = simulate(T(bars, mode="quick"), P(quick_stop_h=4))
        self.assertAlmostEqual(r, -0.3)
        self.assertEqual(how, "time")

    def test_quick_time_stop_skipped_when_in_profit_or_other_modes(self):
        bars = [(0, 0.2, -0.2, 0.1), (4 * HOUR, 0.6, -0.4, 0.3)]
        _, how, _, _ = simulate(T(bars, mode="quick"), P(quick_stop_h=4))
        self.assertEqual(how, "open")
        bars = [(0, 0.2, -0.2, 0.1), (4 * HOUR, 0.1, -0.4, -0.3)]
        _, how, _, _ = simulate(T(bars, mode="trend"), P(quick_stop_h=4))
        self.assertEqual(how, "open")

    def test_fees_subtracted_in_r(self):
        r, _, _, _ = simulate(T([(0, 2.5, 0.0, 2.0)], tp_r=2.0, fee_r=0.1), P())
        self.assertAlmostEqual(r, 1.9)

    def test_tp_cap_applied(self):
        r, how, _, _ = simulate(T([(0, 2.1, 0.0, 2.0)], tp_r=3.0), P(tp_cap=2.0))
        self.assertAlmostEqual(r, 2.0)
        self.assertEqual(how, "tp")

    def test_horizon_marks_last_bar_inside_horizon(self):
        # horizon 2h:t=0、1h 在内;2h 起在外。盯市必须用 1h 那根的收盘,不是数据末尾
        bars = [(0, 0.3, -0.1, 0.2), (HOUR, 0.4, 0.0, 0.5), (2 * HOUR, 0.5, 0.1, 0.9), (3 * HOUR, 0.5, -0.9, -0.8)]
        r, how, _, _ = simulate(T(bars), P(horizon_h=2))
        self.assertEqual(how, "open")
        self.assertAlmostEqual(r, 0.5)

    def test_short_mirrors_long(self):
        e = 100.0
        fill_ms = int(datetime(2026, 9, 1, 0, 0, tzinfo=timezone.utc).timestamp() * 1000)
        start = fill_ms + MIN
        long_bars = [[start, e, 100.8, 99.5, 100.2], [start + MIN, 100.2, 103.5, 100.0, 103.0],
                     [start + 2 * MIN, 103.0, 103.1, 98.0, 98.5]]
        short_bars = [[b[0], 2 * e - b[1], 2 * e - b[3], 2 * e - b[2], 2 * e - b[4]] for b in long_bars]
        fill_t = datetime.fromtimestamp(fill_ms / 1000, timezone.utc).isoformat()
        base = dict(symbol="X", t0_ms=fill_ms, t0=fill_t, outcome="filled", fill_t=fill_t, fill_px=e,
                    exit_mode_last="trend", atr1h_last=1.0, kind="limit")
        lsig = dict(base, side="long", sl=98.0, tp=104.0)
        ssig = dict(base, side="short", sl=102.0, tp=96.0)
        lt = build_trade(lsig, long_bars, "lim")
        st = build_trade(ssig, short_bars, "lim")
        self.assertEqual(len(lt["bars"]), len(st["bars"]))
        for a, b in zip(lt["bars"], st["bars"]):
            for x, y in zip(a, b):
                self.assertAlmostEqual(x, y)
        p = P(be_arm=0.75, trail=True, pd_arm=2.0)
        self.assertEqual(simulate(lt, p), simulate(st, p))

    def test_market_entry_slippage_and_fee(self):
        t0 = 1_700_000_000_000 // MIN * MIN
        bars = [[t0, 100, 100, 100, 100.0], [t0 + MIN, 100, 101, 99, 100]]
        sig = dict(symbol="X", side="long", t0_ms=t0, t0="x", outcome="unfilled", sl0=98.0, tp0=104.0)
        tr = build_trade(sig, bars, "mkt")
        e = 100.0 * (1 + SLIP)
        self.assertAlmostEqual(tr["risk_pct"], (e - 98.0) / e * 100)
        self.assertAlmostEqual(tr["fee_r"], 2 * TAKER / ((e - 98.0) / e))
        # 价格已越过止盈 → 无法模拟
        self.assertIsNone(build_trade(dict(sig, tp0=100.0), bars, "mkt"))

    def test_simulate_returns_exit_time(self):
        bars = [(0, 0.2, -0.2, 0.0), (10 * MIN, 0.3, -1.5, -1.2)]
        self.assertEqual(simulate(T(bars), P())[3], 10 * MIN)
        # horizon 末仍持仓:出场时间=入场起点+horizon
        self.assertEqual(simulate(T([(0, 0.2, -0.2, 0.0), (MIN, 0.2, -0.2, 0.0)]), P(horizon_h=1))[3], HOUR)


def OT(sym, side, start, bars_rel):
    """bars_rel: [(相对 start 的分钟, hi, lo, close)]。"""
    bars = [(start + m * MIN, hi, lo, c) for m, hi, lo, c in bars_rel]
    return dict(sym=sym, side=side, t0=str(start), t0_ms=start, start_ms=start, bars=bars, tp_r=3.0, atr_r=0.0,
                mode="trend", fee_r=0.0, risk_pct=1.0)


# 入场后第 0/10/20 分钟:先冲到 0.8R,之后回落;plain 在第 20 分钟止损,开了保本档则第 10 分钟就被保本出场
LONG_HOLD = [(0, 0.8, -0.2, 0.5), (10, 0.3, -0.5, -0.4), (20, 0.3, -1.5, -1.4)]


class OverlapTest(unittest.TestCase):
    def test_second_signal_same_symbol_side_is_blocked(self):
        a = OT("X", "long", 0, LONG_HOLD)
        b = OT("X", "long", 5 * MIN, [(0, 0.1, -0.1, 0.0)])
        s = summarize([b, a], P())  # 乱序传入也按入场时间处理
        self.assertEqual((s["n"], s["blocked"]), (1, 1))
        s = summarize([a, b], P(), allow_overlap=True)
        self.assertEqual((s["n"], s["blocked"]), (2, 0))

    def test_other_side_or_symbol_not_blocked(self):
        a = OT("X", "long", 0, LONG_HOLD)
        short = OT("X", "short", 5 * MIN, [(0, 0.1, -0.1, 0.0)])
        other = OT("Y", "long", 5 * MIN, [(0, 0.1, -0.1, 0.0)])
        s = summarize([a, short, other], P())
        self.assertEqual((s["n"], s["blocked"]), (3, 0))

    def test_signal_after_exit_is_accepted(self):
        a = OT("X", "long", 0, LONG_HOLD)  # 第 20 分钟止损
        b = OT("X", "long", 20 * MIN, [(0, 0.1, -1.5, -1.4)])  # 入场时刻 == a 的出场时刻,不早于出场;自己立刻止损
        c = OT("X", "long", 30 * MIN, [(0, 0.1, -0.1, 0.0)])
        s = summarize([a, b, c], P())
        self.assertEqual((s["n"], s["blocked"]), (3, 0))

    def test_longer_hold_blocks_more(self):
        a = OT("X", "long", 0, LONG_HOLD)
        b = OT("X", "long", 15 * MIN, [(0, 0.1, -0.1, 0.0)])
        plain = summarize([a, b], P())  # a 持有到第 20 分钟 → b 被挡
        be = summarize([a, b], P(be_arm=0.75, be_off=0.1))  # a 第 10 分钟被保本出场 → b 通过
        self.assertEqual((plain["n"], plain["blocked"]), (1, 1))
        self.assertEqual((be["n"], be["blocked"]), (2, 0))

    def test_open_trade_blocks_until_horizon_end(self):
        a = OT("X", "long", 0, [(0, 0.1, -0.1, 0.0), (1, 0.1, -0.1, 0.0)])  # 一直不出场
        b = OT("X", "long", 30 * MIN, [(0, 0.1, -0.1, 0.0)])
        c = OT("X", "long", 61 * MIN, [(0, 0.1, -0.1, 0.0)])
        s = summarize([a, b, c], P(horizon_h=1))
        self.assertEqual((s["n"], s["blocked"]), (2, 1))


if __name__ == "__main__":
    unittest.main()
