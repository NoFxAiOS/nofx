import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import numpy as np
import pandas as pd

from scripts.factor_mining.config import MARKETS
from scripts.factor_mining.panel import HOLDOUT_SEALED, forward_target, load_panels, make_splits
from scripts.factor_mining.tests.synth import planted, write_market


class PanelTests(unittest.TestCase):
    def test_exact_open_alignment_and_benchmarks(self):
        for market in ('crypto', 'us'):
            panels = planted(rows=100, columns=20, market=market)
            prices = np.arange(1., 101.)[:, None] + np.arange(20)[None, :]
            panels.fields['open'][:] = prices
            for h in (1, 3, 7):
                excess, raw = forward_target(panels, h)
                expected = prices[1+h] / prices[1] - 1
                np.testing.assert_allclose(raw.iloc[0], expected)
                if market == 'crypto':
                    np.testing.assert_allclose(excess.iloc[0], expected - expected.mean())
                else:
                    b = panels.benchmark_open
                    np.testing.assert_allclose(excess.iloc[0], expected - (b.iloc[1+h]/b.iloc[1]-1))
                self.assertTrue(raw.iloc[-h-1:].isna().all().all())
                self.assertTrue(excess.loc[make_splits(raw.index, h)['holdout']].isna().all().all())

    def test_purge_and_ranges(self):
        index = pd.date_range('2020', periods=100)
        for h in (1, 7, 20):
            split = make_splits(index, h)
            self.assertEqual(split['train'].sum(), 60-h-2)
            self.assertEqual(split['val'].sum(), max(0, 20-h-2))
            self.assertEqual(split['holdout'].sum(), 20)
            a = np.flatnonzero(split['train'])[-1]
            self.assertEqual(60-a-1, h+2)
            self.assertEqual(split['ranges']['train'], ('2020-01-01', index[a].strftime('%Y-%m-%d')))
        self.assertTrue(HOLDOUT_SEALED)

    def test_rolling_windows_lengths_purge_ranges_and_daily_shift(self):
        windows = dict(train=300, val=100, holdout=60)
        for market in ('crypto', 'us'):
            with self.subTest(market=market), tempfile.TemporaryDirectory() as root, \
                 patch.dict(MARKETS[market], windows=windows):
                write_market(root, market, rows=650, columns=20)
                first = load_panels(market, root)
                self.assertEqual(first.windows, windows)
                self.assertFalse((Path(root) / market / 'split.json').exists())
                for h in MARKETS[market]['horizons']:
                    index = first.mask.index
                    split = make_splits(index, h, first.windows)
                    self.assertTrue(split['rolling'])
                    starts = dict(train=190, val=490, holdout=590)
                    stops = dict(train=490-h-2, val=590-h-2, holdout=650)
                    for name in starts:
                        expected = np.arange(starts[name], stops[name])
                        np.testing.assert_array_equal(np.flatnonzero(split[name]), expected)
                        self.assertEqual(split['ranges'][name],
                                         tuple(index[i].strftime('%Y-%m-%d')
                                               for i in (starts[name], stops[name]-1)))
                    self.assertFalse((split['train'] | split['val'] | split['holdout']).iloc[:190].any())
                    excess, raw = forward_target(first, h)
                    sealed = ~(split['train'] | split['val'])
                    for target in (excess, raw):
                        self.assertTrue(target.loc[sealed].isna().all().all())
                    # Calendar bars, including business-day gaps, move every endpoint.
                    freq = 'B' if market == 'us' else 'D'
                    extended = pd.date_range(index[0], periods=651, freq=freq)
                    after = make_splits(extended, h, windows)
                    for name in starts:
                        self.assertEqual(after[name].sum(), split[name].sum())
                        self.assertEqual(after['ranges'][name],
                                         tuple(extended[i+1].strftime('%Y-%m-%d')
                                               for i in (starts[name], stops[name]-1)))

    def test_rolling_fallback_and_legacy_split_file_ignored(self):
        windows = dict(train=300, val=100, holdout=60)
        for h in (1, 7, 20):
            threshold = sum(windows.values()) + 2 * (h + 2)
            index = pd.date_range('2020', periods=threshold)
            self.assertTrue(make_splits(index, h, windows)['rolling'])
            fallback = make_splits(index[:-1], h, windows)
            proportional = make_splits(index[:-1], h)
            self.assertFalse(fallback['rolling'])
            self.assertEqual(fallback['ranges'], proportional['ranges'])
            for name in ('train', 'val', 'holdout'):
                pd.testing.assert_series_equal(fallback[name], proportional[name])
        with tempfile.TemporaryDirectory() as root:
            write_market(root, rows=100, columns=20)
            path = Path(root) / 'crypto/split.json'
            path.write_text('ignored legacy split data')
            panels = load_panels('crypto', root)
            split = make_splits(panels.mask.index, 3, panels.windows)
            self.assertFalse(split['rolling'])
            self.assertEqual(path.read_text(), 'ignored legacy split data')
            excess, raw = forward_target(panels, 3)
            for target in (excess, raw):
                self.assertTrue(target.loc[split['holdout']].isna().all().all())

    def test_calendar_bad_rows_missing_values_and_spy(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root, 'us', rows=100, columns=2)
            path = Path(root) / 'us/daily/S000.json'
            rows = json.loads(path.read_text())
            rows.pop(10)
            rows.extend([['bad'], [None, 1, 2, 0, 1, 2, 1],
                         ['2020-01-04', 1, 2, 0, 1, 2, 1],
                         ['2021-01-01', -1, 2, -2, -1, 2, -1]])
            path.write_text(json.dumps(rows))
            panel = load_panels('us', root)
            self.assertEqual(len(panel.mask), 100)
            self.assertNotIn('SPY', panel.mask.columns)
            self.assertTrue(np.isnan(panel.fields['close'].iloc[10, 0]))
            self.assertFalse(panel.mask.iloc[10, 0])
            (Path(root) / 'us/daily/SPY.json').unlink()
            with self.assertRaisesRegex(ValueError, 'SPY'):
                load_panels('us', root)

    def test_mask_causal_history_and_liquidity_ranking(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root, rows=120, columns=65)
            full = load_panels('crypto', root)
            self.assertFalse(full.mask.iloc[:59].to_numpy().any())
            self.assertTrue((full.mask.iloc[59:].sum(axis=1) == 60).all())
            for path in (Path(root) / 'crypto/daily').glob('*.json'):
                path.write_text(json.dumps(json.loads(path.read_text())[:85]))
            short = load_panels('crypto', root)
            pd.testing.assert_frame_equal(short.mask, full.mask.iloc[:85])

    def test_crypto_union_no_fill(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root, rows=100, columns=2)
            path = Path(root) / 'crypto/daily/S000.json'
            rows = json.loads(path.read_text())
            rows.pop(70)
            path.write_text(json.dumps(rows))
            panels = load_panels('crypto', root)
            self.assertEqual(len(panels.mask), 100)
            self.assertTrue(np.isnan(panels.fields['close'].iloc[70, 0]))
            self.assertFalse(panels.mask.iloc[70, 0])
