import json
from pathlib import Path
import tempfile
import unittest

import numpy as np
import pandas as pd

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

    def test_fixed_split_bounds_do_not_drift_with_new_data(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root, 'crypto', rows=100, columns=20)
            first = load_panels('crypto', root)
            saved = json.loads((Path(root) / 'crypto/split.json').read_text())
            self.assertEqual(saved['train_end'], '2020-02-29')   # position 59 of 100 daily bars
            self.assertEqual(saved['val_end'], '2020-03-20')     # position 79
            before = make_splits(first.mask.index, 3, first.split_bounds)
            write_market(root, 'crypto', rows=150, columns=20)   # same seed => same prefix? bounds must not move anyway
            second = load_panels('crypto', root)
            self.assertEqual(second.split_bounds, first.split_bounds)
            after = make_splits(second.mask.index, 3, second.split_bounds)
            for name in ('train', 'val'):
                self.assertEqual(after['ranges'][name], before['ranges'][name])
                self.assertEqual(int(after[name].sum()), int(before[name].sum()))
            # Everything after val_end, including all new bars, is sealed holdout.
            self.assertEqual(int(after['holdout'].sum()), 150 - 80)
            self.assertTrue(after['holdout'].iloc[-1] and not after['val'].iloc[80:].any())
            excess, raw = forward_target(second, 3)
            self.assertTrue(raw.loc[after['holdout']].isna().all().all())

    def test_corrupt_split_file_is_not_silently_moved_to_a_later_date(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root, 'crypto', rows=100, columns=20)
            (Path(root) / 'crypto').mkdir(exist_ok=True)
            (Path(root) / 'crypto/split.json').write_text('{"train_end": "2020-03-01", "val_end": "2020-02-01"}')
            panels = load_panels('crypto', root)   # invalid ordering => regenerate proportionally
            self.assertEqual(panels.split_bounds[0].strftime('%Y-%m-%d'), '2020-02-29')

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
