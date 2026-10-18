import contextlib
import datetime as dt
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

import numpy as np

from scripts.factor_mining import ensemble, judge, miner, run_mining, store
from scripts.factor_mining.config import MARKETS
from scripts.factor_mining.expr import expr_hash
from scripts.factor_mining.panel import forward_target, load_panels, make_splits
from scripts.factor_mining.tests.synth import SIGNAL_EXPR, SIGNAL_SEED, write_market


class PipelineTests(unittest.TestCase):
    def cli(self, root, market='crypto', mutations=0, more=()):
        args = ['--root', str(root), '--market', market, '--mutations', str(mutations),
                '--no-fetch', '--no-llm', '--seed', '42', *more]
        with patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]), \
             patch.object(run_mining.data_fetch, 'update', side_effect=AssertionError('network forbidden')):
            return run_mining.main(args)

    def check_schema(self, root, market):
        root = Path(root)
        zoo = store.read_json(root/market/'zoo.json')
        self.assertTrue({'market', 'generated_at', 'split', 'horizons', 'factors'} <= zoo.keys())
        self.assertEqual(zoo['market'], market)
        self.assertTrue(zoo['split']['holdout_sealed'])
        self.assertEqual(set(zoo['split']), {'rolling', 'windows', 'train', 'val', 'holdout_sealed'})
        self.assertEqual(zoo['split']['windows'], MARKETS[market]['windows'])
        self.assertIsInstance(zoo['split']['rolling'], bool)
        for segment in ('train', 'val'):
            self.assertEqual(len(zoo['split'][segment]), 2)
        self.assertNotIn('holdout', zoo['split'])
        for h in MARKETS[market]['horizons']:
            value = zoo['horizons'][str(h)]
            self.assertEqual(set(value), {'trials', 't_req', 'accepted', 'candidate', 'rejected', 'redundant', 'forward'})
            self.assertEqual(set(value['forward']), {'pending', 'confirmed', 'inconclusive', 'failed'})
            self.assertEqual(value['trials'], sum(value[s] for s in ('accepted', 'candidate', 'rejected', 'redundant')))
        factor_keys = {'id', 'expr', 'source', 'family', 'horizon', 'direction', 'status', 'first_seen',
                       'run_id', 'train', 'val', 'corr_max', 'corr_with', 'ic_monthly'}
        metrics = {'ic', 'icir', 't', 'ls_ret', 'ls_sharpe', 'turnover', 'n'}
        for f in zoo['factors']:
            self.assertTrue(factor_keys <= f.keys())
            self.assertEqual(f['id'], expr_hash(f['expr']))
            self.assertIn(f['direction'], (-1, 1))
            self.assertTrue(metrics <= f['train'].keys())
            self.assertTrue(metrics <= f['val'].keys())
            self.assertEqual(len(f['train']['ic_halves']), 2)
            self.assertTrue(all(len(month) == 2 for month in f['ic_monthly']))
            if f['status'] == 'accepted':
                self.assertIn('forward', f)
            if 'forward' in f:
                self.assertEqual(set(f['forward']), {'accepted_at', 'start', 'direction', 'verdict',
                    'n_days', 'min_days', 'last_date', 'ic', 'icir', 't', 'ls_ret', 'ls_sharpe',
                    'turnover', 'curve'})
                self.assertLessEqual(len(f['forward']['curve']), 120)
                self.assertEqual(f['forward']['min_days'], max(40, 4*f['horizon']))
        combined = store.read_json(root/market/'ensemble.json')
        for h, value in combined['horizons'].items():
            if value is None:
                continue
            self.assertEqual(set(value), {'factor_ids', 'train', 'val', 'curve'})
            self.assertTrue(value['factor_ids'])
            self.assertTrue({'ic', 'icir', 'ls_sharpe'} <= value['train'].keys())
            self.assertTrue({'ic', 'icir', 'ls_sharpe'} <= value['val'].keys())
            self.assertEqual(value['curve']['val'][0][1], 1.)
        status = store.read_json(root/'status.json')
        self.assertTrue({'updated_at', 'running', 'current', 'schedule', 'markets'} <= status.keys())
        self.assertFalse(status['running'])
        self.assertIsNone(status['current'])
        self.assertEqual(status['schedule'], '每天 02:30')
        for m in status['markets'].values():
            self.assertTrue({'symbols', 'start', 'end', 'bars', 'universe', 'data_updated_at'} <= m.keys())
        self.assertEqual(status['evaluator_sha256'], miner.frozen_hashes()['evaluator_sha256'])
        ledger = store.read_ledger(market, root)
        for r in ledger:
            self.assertEqual(set(r), {'ts', 'run_id', 'id', 'expr', 'source', 'horizon',
                                      'train_t', 'train_ic', 'status'})
        for r in store.read_jsonl(root/'runs.jsonl'):
            self.assertTrue({'run_id', 'market', 'started_at', 'finished_at', 'seconds', 'seed_evaluated',
                'mutation_evaluated', 'llm_proposed', 'llm_valid', 'new_accepted', 'new_candidate',
                'trials_total', 'error', 'evaluator_sha256', 'reevaluated', 'forward_updated'} <= r.keys())
            self.assertEqual(r['evaluator_sha256'], status['evaluator_sha256'])
        for p in [root/'status.json', root/'runs.jsonl', root/market/'zoo.json',
                  root/market/'ensemble.json', root/market/'ledger.jsonl']:
            self.assertTrue(p.exists())
            self.assertNotIn('NaN', p.read_text())
            self.assertNotIn('Infinity', p.read_text())
        return zoo

    def test_end_to_end_both_markets_repeat_and_new_mutations(self):
        with tempfile.TemporaryDirectory() as root:
            for market in ('crypto', 'us'):
                write_market(root, market)
            self.assertEqual(self.cli(root, 'all'), 0)
            before = {}
            for market in ('crypto', 'us'):
                zoo = self.check_schema(root, market)
                h = min(MARKETS[market]['horizons'])
                planted = next(f for f in zoo['factors'] if f['horizon'] == h)
                self.assertEqual(planted['status'], 'accepted')
                before[market] = (store.read_ledger(market, root), zoo['horizons'][str(h)]['t_req'])
            self.assertEqual(self.cli(root, 'all'), 0)
            for market in ('crypto', 'us'):
                self.assertEqual(before[market][0], store.read_ledger(market, root))
            self.assertEqual(self.cli(root, 'crypto', mutations=5), 0)
            after = self.check_schema(root, 'crypto')
            self.assertGreater(len(store.read_ledger('crypto', root)), len(before['crypto'][0]))
            self.assertGreater(after['horizons']['1']['t_req'], before['crypto'][1])
            self.assertEqual(len({(r['id'], r['horizon']) for r in store.read_ledger('crypto', root)}),
                             len(store.read_ledger('crypto', root)))

    def test_rolling_pipeline_refreshes_metrics_without_new_trials(self):
        windows = dict(train=300, val=100, holdout=60)
        with tempfile.TemporaryDirectory() as root, patch.dict(MARKETS['crypto'], windows=windows):
            write_market(root, rows=680)
            full = {}
            for path in (Path(root)/'crypto/daily').glob('*.json'):
                full[path] = json.loads(path.read_text())
                path.write_text(json.dumps(full[path][:650]))
            self.assertEqual(self.cli(root), 0)
            before = self.check_schema(root, 'crypto')
            ledger = (Path(root)/'crypto/ledger.jsonl').read_bytes()
            for path, rows in full.items():
                path.write_text(json.dumps(rows))
            self.assertEqual(self.cli(root), 0)
            after = self.check_schema(root, 'crypto')
            self.assertEqual((Path(root)/'crypto/ledger.jsonl').read_bytes(), ledger)
            self.assertTrue(before['split']['rolling'] and after['split']['rolling'])
            self.assertNotEqual(before['split']['train'], after['split']['train'])
            old = {(f['id'], f['horizon']): f for f in before['factors']}
            self.assertTrue(any(f['train'] != old[f['id'], f['horizon']]['train']
                                for f in after['factors']))
            runs = store.read_jsonl(Path(root)/'runs.jsonl')
            self.assertEqual(runs[0]['reevaluated'], 0)
            self.assertEqual(runs[1]['reevaluated'], len(before['factors']))
            self.assertGreater(runs[1]['reevaluated'], 0)
            self.assertEqual(runs[1]['trials_total'], runs[0]['trials_total'])
            self.assertEqual(runs[1]['seed_evaluated'], 0)
            registered = [f for f in after['factors'] if 'forward' in f]
            self.assertTrue(registered)
            for f in registered:
                previous = old[f['id'], f['horizon']]['forward']
                for key in ('accepted_at', 'start', 'direction'):
                    self.assertEqual(f['forward'][key], previous[key])
                self.assertEqual(f['forward']['n_days'], previous['n_days'] + 30)
            self.assertEqual(runs[1]['forward_updated'], len(registered))

    def test_target_poisoning_changes_only_forward_display(self):
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as other, \
             patch.dict(MARKETS['crypto'], windows=dict(train=300, val=100, holdout=60)):
            for directory in (root, other):
                write_market(directory)
            now = dt.datetime(2026, 10, 18, tzinfo=dt.timezone.utc)
            def run(directory):
                return miner.Miner(directory).run_market('crypto', n_mutations=4, n_llm=0,
                    budget_s=120, seed=42, use_llm=False, now=now)
            with patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
                run(root)
                run(other)
                baseline = run(root)  # Exercise poisoning during retained-factor refresh too.
                calls = []
                def poisoned(panels, h):
                    targets = forward_target(panels, h)
                    sealed = make_splits(panels.mask.index, h, panels.windows)['holdout']
                    for i, target in enumerate(targets):
                        target.loc[sealed] = (np.nan if i else np.random.default_rng(h).normal(
                            size=target.loc[sealed].shape))
                    calls.append(h)
                    return targets
                # Change future cached bars too. All visible labels end before
                # this boundary because of purge, and factor operators are causal.
                for path in (Path(other)/'crypto/daily').glob('*.json'):
                    rows = json.loads(path.read_text())
                    for row in rows[-MARKETS['crypto']['windows']['holdout']:]:
                        row[1:5] = [v*3 for v in row[1:5]]
                        row[6] *= 3
                    path.write_text(json.dumps(rows))
                with patch.object(miner, 'forward_target', side_effect=poisoned):
                    actual = run(other)
            self.assertEqual(calls, [1, 3, 7])
            self.assertNotEqual([f.get('forward') for f in baseline['factors']],
                                [f.get('forward') for f in actual['factors']])
            def selection_bytes(output):
                # Strip only the two contractually allowed display differences;
                # compare every remaining serialized byte, including row order.
                value = json.loads(json.dumps(output))
                for f in value['factors']:
                    f.pop('forward', None)
                for stats in value['horizons'].values():
                    stats.pop('forward')
                return json.dumps(value, separators=(',', ':')).encode()
            self.assertEqual(selection_bytes(baseline), selection_bytes(actual))
            for name in ('ledger.jsonl', 'ensemble.json'):
                self.assertEqual((Path(root)/'crypto'/name).read_bytes(),
                                 (Path(other)/'crypto'/name).read_bytes())

    def test_lock_other_process_immediate_rejection(self):
        with tempfile.TemporaryDirectory() as root:
            with store.FileLock(root):
                start = time.monotonic()
                result = subprocess.run([sys.executable, '-m', 'scripts.factor_mining.run_mining',
                    '--root', root, '--market', 'crypto', '--no-fetch', '--no-llm', '--mutations', '0'],
                    env=dict(os.environ, PYTHONDONTWRITEBYTECODE='1'), capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 2)
                self.assertIn('already running', result.stderr)
                self.assertLess(time.monotonic()-start, 3.)
            self.assertFalse((Path(root)/'status.json').exists())

    def test_exception_resets_status_and_writes_one_error_run(self):
        with tempfile.TemporaryDirectory() as root:
            with patch.object(miner, 'load_panels', side_effect=RuntimeError('synthetic failure')), \
                 contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(self.cli(root), 1)
            status = store.read_json(Path(root)/'status.json')
            self.assertFalse(status['running'])
            self.assertIsNone(status['current'])
            runs = store.read_jsonl(Path(root)/'runs.jsonl')
            self.assertEqual(len(runs), 1)
            self.assertIn('synthetic failure', runs[0]['error'])

    def test_fetch_failure_uses_cache_and_records_error(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root)
            args = ['--root', root, '--market', 'crypto', '--mutations', '0', '--no-llm']
            with patch.object(run_mining.data_fetch, 'update', side_effect=RuntimeError('offline fetch')), \
                 patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]), \
                 contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(run_mining.main(args), 0)
            self.assertIn('offline fetch', store.read_jsonl(Path(root)/'runs.jsonl')[0]['error'])
            self.assertTrue((Path(root)/'crypto/zoo.json').exists())

    def test_judge_explicit_confirmation_and_receipt(self):
        with tempfile.TemporaryDirectory() as root, \
             patch.dict(MARKETS['crypto'], windows=dict(train=300, val=100, holdout=60)):
            args = ['--root', root, '--market', 'crypto', '--horizon', '1']
            with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
                judge.main(args)
            self.assertFalse((Path(root)/'holdout_spent.jsonl').exists())
            write_market(root)
            self.assertEqual(self.cli(root), 0)
            with contextlib.redirect_stdout(io.StringIO()) as output:
                self.assertEqual(judge.main([*args, '--i-understand-this-spends-the-holdout']), 0)
            result = json.loads(output.getvalue())
            self.assertGreater(result['result']['ic'], .5)
            self.assertEqual(len(result['factor_ids_hash']), 64)
            panels = load_panels('crypto', root)
            expected = make_splits(panels.mask.index, 1, panels.windows)['ranges']['holdout']
            self.assertEqual(result['holdout_range'], list(expected))
            self.assertEqual(store.read_jsonl(Path(root)/'holdout_spent.jsonl'), [result])

    def test_atomic_finite_store_and_merge(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root)/'finite.json'
            store.write_json(path, dict(x=np.nan, y=np.inf, z=[np.float64(3)]))
            self.assertEqual(store.read_json(path), dict(x=None, y=None, z=[3.]))
            store.write_status({'markets': {'crypto': {'symbols': 5}}}, root)
            store.write_status({'markets': {'us': {'symbols': 10}}}, root)
            self.assertEqual(set(store.read_json(Path(root)/'status.json')['markets']), {'crypto', 'us'})
            original = path.read_bytes()
            with patch.object(store.os, 'replace', side_effect=OSError('simulated disk error')):
                with self.assertRaises(OSError):
                    store.write_json(path, {'replaced': True})
            self.assertEqual(path.read_bytes(), original)
            self.assertFalse(list(Path(root).glob('.finite.json.*')))

    def test_ensemble_fixed_positive_direction_and_empty_horizons(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root)
            panels = load_panels('crypto', root)
            cfg = MARKETS['crypto']
            targets = {h: forward_target(panels, h) for h in cfg['horizons']}
            splits = {h: make_splits(panels.mask.index, h, panels.windows) for h in cfg['horizons']}
            self.assertTrue(all(v is None for v in ensemble.build_ensemble('crypto', [], panels,
                targets, splits, cfg)['horizons'].values()))
            factor = dict(id=expr_hash(SIGNAL_EXPR), expr=SIGNAL_EXPR, horizon=1,
                          direction=-1, status='accepted', train={'t': 10.})
            result = ensemble.build_ensemble('crypto', [factor], panels, targets, splits, cfg)
            # A deliberately wrong, precommitted direction stays wrong.
            self.assertLess(result['horizons']['1']['train']['ic'], 0)
            self.assertLess(result['horizons']['1']['val']['ic'], 0)
