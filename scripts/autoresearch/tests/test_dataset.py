import json
from pathlib import Path
import tempfile
import unittest
import numpy as np
from scripts.autoresearch import build_dataset as bd
from scripts.autoresearch.tests.helpers import bars, synthetic_dataset


class DatasetTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.ctx = bd.sf.bd.Context.from_bars(bars())

    def test_new_features_causal_freshness_windows_fng(self):
        rows = [dict(timestamp=i * bd.HOUR, sumOpenInterest=100 + i,
                     sumOpenInterestValue=1000 + 10 * i, longShortRatio=1 + i / 100,
                     buySellRatio=1 + i / 100, close=i / 10000) for i in range(300)]
        series = {k: bd.Series(rows) for k in ('oi', 'glsr', 'toplsp', 'topls_acct', 'taker', 'premium')}
        fng = bd.Series([dict(timestamp=i * bd.DAY, value=i) for i in range(20)])
        t = 10 * bd.DAY
        full = bd.alt_features(series, fng, t, {'ret_4h': .1, 'ret_24h': -.2})
        cut = bd.alt_features({k: v.prefix(t) for k, v in series.items()}, fng.prefix(t), t,
                              {'ret_4h': .1, 'ret_24h': -.2})
        np.testing.assert_allclose(list(full.values()), list(cut.values()), equal_nan=True)
        self.assertEqual(full['fng'], 10)
        self.assertEqual(full['fng_change_7d'], 7)
        self.assertEqual(full['oi_price_sign_4h'], 1)
        self.assertEqual(full['oi_price_sign_24h'], -1)
        self.assertAlmostEqual(full['oi_qty_chg_4h'], 340 / 336 - 1)
        self.assertAlmostEqual(full['taker_ratio_mean_4h'], np.mean([1 + i / 100 for i in range(237, 241)]))
        self.assertTrue(np.isnan(series['oi'].at(304 * bd.HOUR, 'sumOpenInterest')))
        # A missing interior hourly bucket cannot be silently forward-filled.
        broken = bd.Series([r for r in rows if r['timestamp'] != 239 * bd.HOUR])
        self.assertEqual(len(broken.window(t, 4, 'buySellRatio')), 0)
        self.assertEqual(fng.at(10 * bd.DAY - 1, 'value', 36 * bd.HOUR), 9)

    def test_full_prefix_check_missing_coin_and_xs(self):
        syms = ['BTCUSDT', 'A', 'MISSING']
        ctxs = {'BTCUSDT': self.ctx, 'A': self.ctx}
        aux = {s: ({}, np.empty((0, 2))) for s in syms}
        alts = {s: {k: bd.Series() for k in ('oi', 'glsr', 'toplsp', 'topls_acct', 'taker', 'premium')} for s in syms}
        t = 36 * bd.DAY
        self.assertTrue(bd.causal_check(ctxs, syms, aux, alts, bd.Series(), t)['passed'])
        features = bd.features_at(ctxs, syms, aux, alts, bd.Series(), t)
        self.assertTrue(np.isnan(features.loc['MISSING', 'ret_24h']))
        self.assertTrue(features['ret_24h_xs'].dropna().between(0, 1).all())
        # Hostile unclosed candles cannot change any feature or rank.
        raw = bars()
        bad = [r if r[0] < t else [r[0], 900, 2000, 1, 1500] for r in raw]
        hostile = bd.sf.bd.Context.from_bars(bad)
        altered = bd.features_at({'BTCUSDT': hostile, 'A': hostile}, syms, aux, alts, bd.Series(), t)
        np.testing.assert_allclose(features, altered, equal_nan=True)

    def test_split_purge_no_holdout_in_visible_and_coverage(self):
        with tempfile.TemporaryDirectory() as tmp:
            frame, meta = synthetic_dataset(tmp)
            with np.load(Path(tmp) / 'work/visible.npz', allow_pickle=False) as v, np.load(Path(tmp) / 'private/holdout.npz', allow_pickle=False) as h:
                self.assertEqual(set(v['split']), {'train', 'val'})
                self.assertEqual(set(h['split']), {'holdout'})
                self.assertFalse(set(v['t_ms']) & set(h['t_ms']))
                train, val = v['t_ms'][v['split'] == 'train'], v['t_ms'][v['split'] == 'val']
                self.assertLess(train.max() + 76 * bd.HOUR, val.min())
                self.assertLess(val.max() + 76 * bd.HOUR, h['t_ms'].min())
                np.testing.assert_allclose(v['y'][:, 1], v['y'][:, 0] - .0016)
                np.testing.assert_allclose(v['y'][:, 3], -v['y'][:, 0])
            self.assertFalse((Path(tmp) / 'work/holdout.npz').exists())
            self.assertEqual(meta['feature_status']['missing'], 'immature')
            self.assertEqual(meta['coverage']['missing'], 0)
            # Holdout values cannot change visible data or visible metadata coverage.
            frame.loc[frame.t_ms >= bd.split_times(frame.t_ms)['holdout'][0], 'missing'] = 999
            old = meta['data_sha256']['visible']
            new = bd.save_dataset(frame, tmp, meta['features'])
            self.assertEqual(new['coverage'], meta['coverage'])
            self.assertEqual(old, new['data_sha256']['visible'])

    def test_forward_return_exact_entry_and_maturity(self):
        t = 36 * bd.DAY
        a = self.ctx.a5
        i = np.searchsorted(a['t_open'], t)
        j = np.searchsorted(a['t_close'], t + 72 * bd.HOUR)
        self.assertAlmostEqual(bd.sf.forward_return(self.ctx, t, 72), a['c'][j] / a['o'][i] - 1)
        self.assertTrue(np.isnan(bd.sf.forward_return(self.ctx, 49 * bd.DAY, 72)))


class FullBuildSmokeTests(unittest.TestCase):
    def test_mock_cache_full_build_no_fetch_all_members_and_sample(self):
        from unittest.mock import patch
        from scripts.autoresearch import init_work
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            cache, alt, out = root / 'cache', root / 'alt', root / 'out'
            # One cached coin and one missing coin: every decision retains both.
            bd.atomic_json(cache / 'ict5m/BTCUSDT.json', bars(days=50))
            bd.atomic_json(alt / 'universe_src.json', {'crypto': ['BTCUSDT', 'MISSING', 'STXUSDT']})
            with patch('urllib.request.OpenerDirector.open', side_effect=AssertionError('network forbidden')):
                meta = bd.build(cache, cache, alt, out)
                work = init_work.initialize(out / 'work')
            self.assertEqual(len(meta['causal_checks']), 3)
            self.assertEqual(meta['missing_price_files'], ['MISSING'])
            with np.load(work / 'visible.npz', allow_pickle=False) as v:
                self.assertEqual(set(v['sym']), {'BTCUSDT', 'MISSING'})
                self.assertEqual(v['X'].shape[1], 2 * len(bd.FEATURES))
                missing = v['sym'] == 'MISSING'
                self.assertTrue(np.isnan(v['X'][missing, meta['features'].index('ret_24h')]).all())
                self.assertTrue(np.isnan(v['y'][missing]).all())
                self.assertEqual(len(v['t_ms']), 2 * len(np.unique(v['t_ms'])))
            # --sample is a deterministic debug universe subset, not a row filter.
            sampled = bd.build(cache, cache, alt, root / 'sampled', sample=1)
            with np.load(root / 'sampled/work/visible.npz', allow_pickle=False) as v:
                self.assertEqual(set(v['sym']), {'BTCUSDT'})
                self.assertEqual(len(v['t_ms']), len(np.unique(v['t_ms'])))


if __name__ == '__main__':
    unittest.main()
