import time
import unittest

import numpy as np
import pandas as pd

from scripts.factor_mining.config import MARKETS
from scripts.factor_mining.evaluate import (classify, cross_corr, cs_rank_ic,
    evaluate_factor, long_short, nw_t, segment_metrics, t_required)
from scripts.factor_mining.expr import evaluate_expr
from scripts.factor_mining.panel import forward_target, make_splits
from scripts.factor_mining.tests.synth import SIGNAL_EXPR, planted


class EvaluateTests(unittest.TestCase):
    def test_signals_noise_and_direction_multiple_seeds(self):
        for seed in (3, 17, 42, 100):
            panels = planted(rows=750, seed=seed)
            targets = forward_target(panels, 1)
            splits = make_splits(panels.mask.index, 1)
            signal = evaluate_expr(SIGNAL_EXPR, panels.fields)
            for sign in (1, -1):
                result = evaluate_factor(signal*sign, *targets, panels.mask, splits, 1, MARKETS['crypto'])
                self.assertEqual(result['direction'], sign)
                self.assertEqual(classify(result['train'], result['val'], 200, t_required(200)), 'accepted')
            noise = pd.DataFrame(np.random.default_rng(seed+900).normal(size=signal.shape),
                                 index=signal.index, columns=signal.columns)
            result = evaluate_factor(noise, *targets, panels.mask, splits, 1, MARKETS['crypto'])
            self.assertEqual(classify(result['train'], result['val'], 200, t_required(200)), 'rejected')

    def test_threshold_and_classification_boundaries(self):
        self.assertEqual(t_required(1), 2.)
        values = [t_required(k) for k in range(1, 1000)]
        self.assertTrue(all(a <= b for a, b in zip(values, values[1:])))
        train = dict(ic=.02, t=5., ls_ret=.05, ic_halves=[.01, .03])
        val = dict(ic=.01, t=1.65, ls_ret=.01)
        self.assertEqual(classify(train, val, 10, 3.), 'accepted')
        self.assertEqual(classify(train, dict(val, t=1.64), 10, 3.), 'candidate')
        self.assertEqual(classify(train, dict(val, ls_ret=0), 10, 3.), 'candidate')
        self.assertEqual(classify(dict(train, ls_ret=-.01), val, 10, 3.), 'candidate')
        self.assertEqual(classify(dict(train, ic_halves=[-.01, .03]), val, 10, 3.), 'rejected')
        self.assertEqual(classify(dict(train, ic_halves=[0., .03]), val, 10, 3.), 'rejected')
        self.assertEqual(classify(train, val, 10, 6.), 'rejected')

    def test_hac_noise_distribution_and_autocorrelation(self):
        rng = np.random.default_rng(481)
        estimates = np.array([nw_t(rng.normal(size=600), 5) for _ in range(300)])
        self.assertLess(abs(estimates.mean()), .18)
        self.assertGreater(estimates.std(), .8)
        self.assertLess(estimates.std(), 1.2)
        x = np.zeros(2000)
        for t in range(1, len(x)):
            x[t] = .9*x[t-1] + rng.normal()
        x += .5
        naive = x.mean()/x.std(ddof=1)*np.sqrt(len(x))
        self.assertLess(abs(nw_t(x, 20)), abs(naive))
        self.assertEqual(nw_t([1.]*19, 3), 0.)
        self.assertTrue(np.isfinite(nw_t([1.]*30, 3)))

    def test_tied_rank_pairwise_mask_and_min_names(self):
        index = pd.date_range('2020', periods=2)
        values = pd.DataFrame([[1, 1, 3, 4, 99], [1, 2, 3, 4, 5]], index=index)
        target = pd.DataFrame([[2, 2, 4, 3, -999], [1, 2, np.nan, 4, 5]], index=index)
        mask = values.notna()
        mask.iloc[:, -1] = False
        ic = cs_rank_ic(values, target, mask, 4)
        expected = np.corrcoef([1.5, 1.5, 3, 4], [1.5, 1.5, 4, 3])[0, 1]
        self.assertAlmostEqual(ic.iloc[0], expected)
        self.assertTrue(np.isnan(ic.iloc[1]))
        self.assertAlmostEqual(cross_corr(values, values, index), 1.)

    def test_turnover_costs_and_segment_schedule_hand_calculation(self):
        index = pd.date_range('2020', periods=5)
        v = pd.DataFrame(np.tile(np.arange(15), (5, 1)), index=index, dtype=float)
        raw = v*.001
        v.iloc[2, :3], v.iloc[2, -3:] = [12, 13, 14], [0, 1, 2]
        segment = pd.Series(True, index=index)
        cfg = dict(MARKETS['crypto'], one_way_cost=.001)
        net, turnover = long_short(v, raw, v.notna(), segment, 2, cfg)
        np.testing.assert_allclose(net, [.012-.004, -.012-.004, .012-.004])
        np.testing.assert_allclose(turnover, [1, 1, 1])
        v.iloc[2] = v.iloc[0]
        net, turnover = long_short(v, raw, v.notna(), segment, 2, cfg)
        np.testing.assert_allclose(net, [.008, .012, .012])
        np.testing.assert_allclose(turnover, [1, 0, 0])
        # A warmup miss doesn't move the h-day schedule to day 1.
        v.iloc[0] = np.nan
        net, _ = long_short(v, raw, raw.notna(), segment, 2, cfg)
        self.assertEqual(list(net.index), [index[2], index[4]])
        metric = segment_metrics(pd.Series(.1, index=index), v, raw, raw.notna(), segment, 2, cfg)
        self.assertAlmostEqual(metric['ls_ret'], .01 * 365/2)

    def test_validity_and_future_values_causality(self):
        panels = planted(rows=350)
        index = panels.mask.index
        splits = make_splits(index, 1)
        targets = forward_target(panels, 1)
        values = evaluate_expr(SIGNAL_EXPR, panels.fields)
        for invalid in (values*0, values*float('nan'), 1+values*1e-16):
            self.assertIsNone(evaluate_factor(invalid, *targets, panels.mask, splits, 1, MARKETS['crypto']))
        sparse = values.copy()
        sparse.iloc[:300] = np.nan
        self.assertIsNone(evaluate_factor(sparse, *targets, panels.mask, splits, 1, MARKETS['crypto']))
        full = cs_rank_ic(values, targets[0], panels.mask, 15)
        altered = values.copy()
        altered.iloc[151:] = np.random.default_rng(22).normal(size=altered.iloc[151:].shape)
        modified = cs_rank_ic(altered, targets[0], panels.mask, 15)
        pd.testing.assert_series_equal(full.iloc[:151], modified.iloc[:151])
        prefix = cs_rank_ic(values.iloc[:151], targets[0].iloc[:151], panels.mask.iloc[:151], 15)
        pd.testing.assert_series_equal(full.iloc[:151], prefix)

    def test_vectorized_performance(self):
        panels = planted(rows=2500, columns=200)
        values = evaluate_expr(SIGNAL_EXPR, panels.fields)
        targets = forward_target(panels, 1)
        splits = make_splits(values.index, 1)
        begin = time.perf_counter()
        result = evaluate_factor(values, *targets, panels.mask, splits, 1, MARKETS['crypto'])
        elapsed = time.perf_counter()-begin
        self.assertIsNotNone(result)
        self.assertLess(elapsed, .6, f'2500x200 single horizon took {elapsed:.3f}s')
