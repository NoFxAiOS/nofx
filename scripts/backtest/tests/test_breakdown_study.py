"""Causal boundaries, Go predicates, and native Backtrader entry contract."""
import math
import random
import unittest

import numpy as np

from scripts.backtest import breakdown_study as bd, ict_bt


def bars(days=22):
    rows = []
    for i in range(days * 288):
        o = 200 - i * 0.008 + 0.5 * math.sin(i / 15)
        c = o - 0.003
        rows.append([i * bd.M5, o, o + 0.2, c - 0.2, c])
    return rows


class BreakdownStudyTests(unittest.TestCase):
    def test_go_indicator_seeds_and_rounding(self):
        self.assertEqual(bd.ema([10, 20, 30], 3), [10, 15, 22.5])
        for actual, expected in zip(bd.atr([11, 14, 16, 17], [9, 10, 12, 13], [10, 12, 14, 15], 3),
                                    [2, 3, 10 / 3, 32 / 9]):
            self.assertAlmostEqual(actual, expected)
        self.assertEqual(bd.rsi_series([1] * 20)[-1], 100)
        self.assertEqual(bd.rsi_series(list(range(20, 0, -1)))[-1], 0)
        self.assertEqual(bd.round2(-1.125), -1.13)
        self.assertEqual(bd.round2(1.125), 1.13)

    def test_continuation_exact_bounds(self):
        c = np.full(60, 100.0)
        h, lo = np.full(60, 99.0), np.full(60, 95.0)
        c[-1], h[-1], lo[-1] = 94, 94, 93
        c[-14], c[-13] = 99, 99
        k1 = dict(c=c, h=h, l=lo)
        k4 = dict(c=np.ones(84), h=np.r_[np.full(78, 120), np.full(6, 110)])
        self.assertEqual(bd.continuation(k1, k4), (55, True))
        # n-13 is included; n-14 and the current candle are excluded.
        h[-14] = 101
        self.assertEqual(bd.continuation(k1, k4), (55, True))
        h[-13] = 101
        self.assertEqual(bd.continuation(k1, k4), (100, True))
        c[-1] = 96
        self.assertEqual(bd.continuation(k1, k4), (88, True))
        h[-13] = 99
        self.assertEqual(bd.continuation(k1, k4), (43, False))

    def test_score_formula_and_btc_relative_weakness(self):
        ctx = bd.Context.from_bars(bars())
        t = 17 * bd.DAY
        k1 = bd.closed_window(ctx.a1, t, 60, bd.HOUR)
        k4 = bd.closed_window(ctx.a4, t, 84, 4 * bd.HOUR)
        s = bd.score_windows(k1, k4, k4)
        self.assertTrue(s['gate'])
        self.assertEqual(s['rel_weak'], 50)
        self.assertEqual(s['btc_outperform'], 0)
        self.assertEqual(s['new'], bd.round2(.35*s['continuation'] + .25*bd.sigmoid(-s['down_ext'], -3, 1.5)
                                           + .15*s['rejection'] + 5 + 7.5))
        self.assertEqual(s['old'], bd.round2(.30*s['structure'] + .25*bd.sigmoid(-s['down_ext'], -1.5, 4)
                                           + .15*s['rejection'] + .15*s['divergence'] + 7.5))
        self.assertEqual(bd.btc_regime(k4), 'btc_bear')

    def test_resample_prefix_and_hostile_future_are_causal(self):
        rows = bars()
        ctx = bd.Context.from_bars(rows)
        t = 17 * bd.DAY
        bd.causal_check(ctx, ctx, t)
        before = bd.snapshot(ctx, ctx, t)
        hostile = [r if r[0] < t else [r[0], 900, 2000, 1, 1500] for r in rows]
        altered = bd.Context.from_bars(hostile)
        self.assertEqual(before, bd.snapshot(altered, altered, t))
        # A 4h bar ending at t+4h, even if present in full history, is excluded.
        w = bd.closed_window(altered.a4, t, 84, 4 * bd.HOUR)
        self.assertEqual(int(w['t_close'][-1]), t)
        self.assertTrue(np.all(w['t_close'] <= t))

    def test_old_structure_and_divergence_predicates(self):
        c1 = np.full(60, 100.0)
        c1[-5], c1[-4:] = 110, 90
        k1 = dict(c=c1, o=c1.copy(), h=c1+1, l=c1-1)
        c4 = np.r_[np.linspace(80, 120, 61), np.linspace(119, 90, 23)]
        k4 = dict(c=c4, o=c4.copy(), h=c4+1, l=c4-1)
        k4['h'][60], k4['h'][80] = 150, 151
        s = bd.score_windows(k1, k4, k4)
        self.assertEqual(s['structure'], 80)  # fresh MA break, no failed breakout
        self.assertEqual(s['divergence'], 100)  # strictly higher high, RSI lower by >2
        k1['h'][-2] = 200
        self.assertEqual(bd.score_windows(k1, k4, k4)['structure'], 100)
        k4['h'][80] = 149.5
        self.assertEqual(bd.score_windows(k1, k4, k4)['divergence'], 40)
        k4['h'][80] = 148
        self.assertEqual(bd.score_windows(k1, k4, k4)['divergence'], 0)
        # A failed breakout that has since been reclaimed must not stay sticky.
        k1['c'][-1], k1['o'][-1], k1['h'][-1], k1['l'][-1] = 150, 150, 151, 149
        self.assertEqual(bd.score_windows(k1, k4, k4)['structure'], 0)

    def test_overlap_filters_each_strategy_before_slicing(self):
        jobs = [dict(sym='TEST', side='short', t_ms=t) for t in (0, 5, 10)]
        results = [dict(r=1, exit_ms=t) for t in (20, 6, 11)]
        self.assertEqual(bd.kept_indices(jobs, results, False), ([0], 2))
        self.assertEqual(bd.kept_indices(jobs, results, False, [1, 2]), ([1, 2], 0))
        self.assertEqual(bd.kept_indices(jobs, results, True), ([0, 1, 2], 0))

    def test_market_entry_and_shifted_control_geometry(self):
        ctx = bd.Context.from_bars(bars())
        t = 17 * bd.DAY
        s = bd.snapshot(ctx, ctx, t)
        job = bd.make_job('TESTUSDT', ctx, t, s)
        self.assertEqual(job['ref'], ctx.a5['o'][job['idx'] + 1])
        control = bd.control_of(job, ctx, random.Random(7))
        shift = abs(control['t_ms'] - t)
        self.assertEqual(shift % bd.DAY, 0)
        self.assertTrue(2 * bd.DAY <= shift <= 14 * bd.DAY)
        self.assertAlmostEqual(control['sl'] / control['ref'], job['sl'] / job['ref'])
        self.assertAlmostEqual(control['tp'] / control['ref'], job['tp'] / job['ref'])
        # Exercise the required ict_bt.run_jobs -> adapter.run_trade path.
        previous = dict(ict_bt.DATA)
        try:
            ict_bt.DATA['TESTUSDT'] = ctx.a5
            result = ict_bt.run_jobs([job], ict_bt.plain_rules(), comm=0, slip=0, procs=1)[0]
        finally:
            ict_bt.DATA.clear()
            ict_bt.DATA.update(previous)
        self.assertEqual(result['entry_ms'], t)
        self.assertEqual(result['how'], 'tp')
        self.assertAlmostEqual(result['r'], 2)

    def test_shared_group_difference_and_tied_spearman(self):
        j = [dict(t_ms=i * bd.DAY) for i in range(20)]
        r = [dict(r=float(i % 3 - 1)) for i in range(20)]
        diff = bd.mean_difference(j, r, list(range(20)), j, r, list(range(20)))
        self.assertEqual(diff['diff'], 0)
        self.assertAlmostEqual(diff['cluster_se'], 0)
        self.assertEqual(bd.spearman([1, 1, 3], [2, 2, 5]), 1)
        self.assertIsNone(bd.spearman([1, 1], [1, 2]))


if __name__ == '__main__':
    unittest.main()
