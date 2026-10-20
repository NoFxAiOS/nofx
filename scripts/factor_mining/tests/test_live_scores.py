import contextlib
import copy
import datetime as dt
import io
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import numpy as np
import pandas as pd

from scripts.factor_mining import ensemble, live_scores, miner, store
from scripts.factor_mining.panel import Panels
from scripts.factor_mining.tests.synth import SIGNAL_SEED, write_market


NOW = dt.datetime(2026, 10, 20, 1, tzinfo=dt.timezone.utc)


def panels_for_scores(names=15):
    index = pd.date_range('2026-10-17', periods=5)
    close = pd.DataFrame(np.tile(np.arange(1., names + 1), (len(index), 1)),
                         index=index, columns=[f'S{i:03}' for i in range(names)])
    fields = {name: close.copy() for name in ('open', 'high', 'low', 'close', 'vwap')}
    fields['volume'] = close ** 2
    return Panels(fields, close.notna())


def factor(key='f_close', h=1, t=5., **extra):
    return dict(id=key, horizon=h, expr='$close', direction=1,
                status='accepted', train={'t': t}, **extra)


class LiveScoresTests(unittest.TestCase):
    def test_as_of_excludes_today_and_future_using_utc(self):
        panels = panels_for_scores()
        for now, expected in (
                (NOW, '2026-10-19'),
                (dt.datetime(2026, 10, 20, 1, tzinfo=dt.timezone(dt.timedelta(hours=8))), '2026-10-18')):
            with self.subTest(now=now), tempfile.TemporaryDirectory() as root:
                output = live_scores.export_live_scores('crypto', [factor()], panels, root=root, now=now)
                self.assertEqual(output['as_of'], expected)
                self.assertEqual(output['version'], 1)
                self.assertEqual(output['market'], 'crypto')
                self.assertEqual(output['universe_size'], 15)
                generated = dt.datetime.fromisoformat(output['generated_at'])
                self.assertEqual(generated.utcoffset(), dt.timedelta())
                self.assertEqual(generated, now)
                self.assertEqual(set(output['horizons']), {'1'})

    def test_direction_equal_weight_percentiles_and_inputs_unchanged(self):
        panels = panels_for_scores()
        panels.mask.iloc[:, -1] = False
        factors = [factor(forward={'verdict': 'failed'}),
                   dict(factor('f_volume'), expr='$volume', direction=-1)]
        cache = {'f_close': panels.fields['close'], 'f_volume': panels.fields['volume']}
        before_factors, before_cache = copy.deepcopy(factors), copy.deepcopy(cache)
        before_mask = panels.mask.copy()
        with tempfile.TemporaryDirectory() as root, \
             patch.object(ensemble, 'evaluate_expr', side_effect=AssertionError('cache must be reused')):
            output = live_scores.export_live_scores('crypto', factors, panels,
                                                     root=root, now=NOW, values_cache=cache)
        horizon = output['horizons']['1']
        self.assertEqual(horizon['factor_count'], 2)
        self.assertEqual(horizon['forward'], {'failed': 1, 'none': 1})
        self.assertEqual(output['universe_size'], 14)
        close = panels.fields['close'].loc['2026-10-19'].iloc[:-1]
        volume = panels.fields['volume'].loc['2026-10-19'].iloc[:-1]
        expected = ((close - close.mean()) / close.std() -
                    (volume - volume.mean()) / volume.std()) / 2
        for symbol, z in expected.items():
            self.assertEqual(horizon['scores'][symbol],
                             {'z': round(float(z), 4), 'pct': round(float(expected.rank(pct=True)[symbol]), 4)})
        self.assertNotIn('S014', horizon['scores'])
        self.assertEqual(factors, before_factors)
        self.assertEqual(cache.keys(), before_cache.keys())
        for key in cache:
            pd.testing.assert_frame_equal(cache[key], before_cache[key])
        pd.testing.assert_frame_equal(panels.mask, before_mask)

    def test_nonfinite_and_masked_scores_excluded_before_ranking(self):
        panels = panels_for_scores()
        panels.mask.loc['2026-10-19', 'S014'] = False
        score = panels.fields['close'].copy()
        score.loc['2026-10-19', ['S000', 'S001', 'S002']] = [np.nan, np.inf, -np.inf]
        score.loc['2026-10-19', 'S004'] = score.loc['2026-10-19', 'S003']
        with tempfile.TemporaryDirectory() as root, \
             patch.object(live_scores, 'ensemble_scores', return_value=score):
            output = live_scores.export_live_scores('crypto', [factor()], panels, root=root, now=NOW)
            text = (Path(root) / 'crypto/live_scores.json').read_text()
        scores = output['horizons']['1']['scores']
        self.assertEqual(len(scores), 11)
        self.assertEqual(scores['S003']['pct'], round(1.5 / 11, 4))
        self.assertEqual(scores['S004']['pct'], scores['S003']['pct'])
        self.assertEqual(scores['S013']['pct'], 1.)
        self.assertTrue(all(np.isfinite(list(value.values())).all() for value in scores.values()))
        self.assertNotIn('NaN', text)
        self.assertNotIn('Infinity', text)
        self.assertEqual(json.loads(text), output)

    def test_thin_horizon_omitted_without_falling_back(self):
        panels = panels_for_scores()
        thin = panels.fields['close'].copy()
        thin.loc['2026-10-19', thin.columns[9:]] = np.nan
        with tempfile.TemporaryDirectory() as root, \
             patch.object(live_scores, 'ensemble_scores', side_effect=[thin, panels.fields['close']]):
            output = live_scores.export_live_scores('crypto', [factor(), factor('f_three', h=3)],
                                                     panels, root=root, now=NOW)
        self.assertEqual(output['as_of'], '2026-10-19')
        self.assertEqual(set(output['horizons']), {'3'})

    def test_less_than_ten_names_deletes_stale_file(self):
        panels = panels_for_scores()
        panels.mask.loc['2026-10-19', panels.mask.columns[9:]] = False
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'crypto/live_scores.json'
            store.write_json(path, {'stale': True})
            self.assertIsNone(live_scores.export_live_scores('crypto', [factor()], panels, root=root, now=NOW))
            self.assertFalse(path.exists())

    def test_exactly_ten_names_are_exported(self):
        with tempfile.TemporaryDirectory() as root:
            output = live_scores.export_live_scores('crypto', [factor()], panels_for_scores(10), root=root, now=NOW)
        self.assertEqual(len(output['horizons']['1']['scores']), 10)

    def test_no_accepted_factors_deletes_stale_file(self):
        for factors in ([], [dict(factor(), status='candidate')], [dict(factor(), status='redundant')]):
            with self.subTest(factors=factors), tempfile.TemporaryDirectory() as root:
                path = Path(root) / 'crypto/live_scores.json'
                store.write_json(path, {'stale': True})
                self.assertIsNone(live_scores.export_live_scores('crypto', factors, panels_for_scores(),
                                                               root=root, now=NOW))
                self.assertFalse(path.exists())
                # Repeating cleanup when there is no previous file is harmless.
                self.assertIsNone(live_scores.export_live_scores('crypto', factors, panels_for_scores(),
                                                               root=root, now=NOW))

    def test_no_closed_dates_deletes_stale_file(self):
        panels = panels_for_scores()
        panels.mask = panels.mask.loc['2026-10-20':]
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'crypto/live_scores.json'
            store.write_json(path, {'stale': True})
            self.assertIsNone(live_scores.export_live_scores('crypto', [factor()], panels, root=root, now=NOW))
            self.assertFalse(path.exists())

    def test_selection_matches_build_ensemble_and_forward_counts_selected_only(self):
        panels = panels_for_scores()
        factors = [factor(f'f_{i:02}', t=float(i // 2), forward={'verdict': 'inconclusive'})
                   for i in reversed(range(24))]
        factors.extend([dict(factor('f_candidate', t=999), status='candidate'),
                        dict(factor('f_redundant', t=999), status='redundant'), factor('f_other', h=3)])
        cfg = {'horizons': (1, 3, 7)}
        dates = panels.mask.index
        splits = {h: {'val': pd.Series(True, index=dates), 'ranges': {'val': ('2026-10-17', '2026-10-21')}}
                  for h in cfg['horizons']}
        targets = {h: (None, None) for h in cfg['horizons']}
        with patch.object(ensemble, 'evaluate_factor', return_value={'train': {}, 'val': {}}), \
             patch.object(ensemble, 'long_short', return_value=(pd.Series(dtype=float), None)):
            built = ensemble.build_ensemble('crypto', factors, panels, targets, splits, cfg)
        expected_ids = [f'f_{i:02}' for pair in range(11, 1, -1) for i in (2 * pair, 2 * pair + 1)]
        self.assertEqual(built['horizons']['1']['factor_ids'], expected_ids)
        with tempfile.TemporaryDirectory() as root, \
             patch.object(live_scores, 'ensemble_scores', wraps=ensemble.ensemble_scores) as calls:
            output = live_scores.export_live_scores('crypto', factors, panels, root=root, now=NOW)
        for call in calls.call_args_list:
            selected = call.args[0]
            h = str(selected[0]['horizon'])
            self.assertEqual([f['id'] for f in selected], built['horizons'][h]['factor_ids'])
        self.assertEqual(output['horizons']['1']['factor_count'], 20)
        self.assertEqual(output['horizons']['1']['forward'], {'inconclusive': 20})
        self.assertEqual(output['horizons']['3']['forward'], {'none': 1})
        self.assertNotIn('7', output['horizons'])

    def test_atomic_write_failure_preserves_previous_snapshot(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'crypto/live_scores.json'
            store.write_json(path, {'previous': True})
            before = path.read_bytes()
            with patch.object(store.os, 'replace', side_effect=OSError('disk error')):
                with self.assertRaisesRegex(OSError, 'disk error'):
                    live_scores.export_live_scores('crypto', [factor()], panels_for_scores(), root=root, now=NOW)
            self.assertEqual(path.read_bytes(), before)
            self.assertEqual(list(path.parent.glob('.live_scores.json.*')), [])

    def test_miner_exports_both_markets_without_changing_existing_artifacts_or_reloading(self):
        export = live_scores.export_live_scores
        with tempfile.TemporaryDirectory() as tmp:
            for market in ('crypto', 'us'):
                source = Path(tmp) / f'{market}_source'
                write_market(source, market)
                snapshots = []
                for mode in ('disabled', 'success', 'failure'):
                    root = Path(tmp) / f'{market}_{mode}'
                    shutil.copytree(source, root)
                    def attempt(*args, **kwargs):
                        self.assertTrue((root / market / 'ensemble.json').exists())
                        if mode == 'failure':
                            raise OSError('export unavailable')
                        if mode == 'success':
                            return export(*args, **kwargs)
                    stderr = io.StringIO()
                    with self.subTest(market=market, mode=mode), \
                         patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]), \
                         patch.object(miner.time, 'monotonic', return_value=0.), \
                         patch.object(store, 'timestamp', return_value=NOW.isoformat()), \
                         patch.object(miner, 'load_panels', wraps=miner.load_panels) as loads, \
                         patch.object(miner, 'build_ensemble', wraps=ensemble.build_ensemble) as builds, \
                         patch.object(ensemble, 'evaluate_expr', side_effect=AssertionError('cache was discarded')), \
                         patch.object(live_scores, 'export_live_scores', side_effect=attempt) as exports, \
                         contextlib.redirect_stderr(stderr):
                        miner.Miner(root).run_market(market, n_mutations=0, n_llm=0, budget_s=120,
                                                     seed=1, use_llm=False, now=NOW)
                    loads.assert_called_once()
                    exports.assert_called_once()
                    self.assertIs(exports.call_args.args[1], builds.call_args.args[1])
                    self.assertIs(exports.call_args.args[2], builds.call_args.args[2])
                    self.assertIs(exports.call_args.kwargs['values_cache'], builds.call_args.kwargs['values_cache'])
                    files = [root / market / name for name in ('zoo.json', 'ensemble.json', 'ledger.jsonl')]
                    files.extend([root / 'runs.jsonl', root / 'status.json'])
                    snapshots.append([path.read_bytes() for path in files])
                    if mode == 'success':
                        output = store.read_json(root / market / 'live_scores.json')
                        self.assertTrue(output['horizons'])
                        built = store.read_json(root / market / 'ensemble.json')
                        for h, value in output['horizons'].items():
                            self.assertEqual(value['factor_count'], len(built['horizons'][h]['factor_ids']))
                    elif mode == 'failure':
                        self.assertIn('live_scores export failed', stderr.getvalue())
                        self.assertIn('OSError: export unavailable', stderr.getvalue())
                        self.assertIsNone(store.read_jsonl(root / 'runs.jsonl')[0]['error'])
                self.assertEqual(snapshots[0], snapshots[1])
                self.assertEqual(snapshots[0], snapshots[2])
