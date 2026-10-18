"""Offline pre-registration, isolation, lifecycle and full-run checks."""
import copy
import datetime as dt
import json
from pathlib import Path
import tempfile
import time
import unittest
from unittest.mock import patch

import numpy as np
import pandas as pd

from scripts.factor_mining import ensemble, evaluate, forward, miner, panel, store
from scripts.factor_mining.config import MARKETS
from scripts.factor_mining.expr import evaluate_expr
from scripts.factor_mining.tests.synth import SIGNAL_EXPR, SIGNAL_SEED, planted, write_market


class ForwardTests(unittest.TestCase):
    def test_registration_only_accepted_once_and_direction_frozen(self):
        entry = dict(status='candidate', direction=-1, horizon=20)
        ranges = {'holdout': ('2026-04-20', '2026-10-18')}
        for status in ('rejected', 'candidate', 'redundant'):
            entry['status'] = status
            self.assertIsNone(forward.register(entry, ranges, '2026-10-19'))
            self.assertNotIn('forward', entry)
        entry['status'] = 'accepted'
        record = forward.register(entry, ranges, '2026-10-19')
        self.assertEqual((record['accepted_at'], record['start'], record['direction']),
                         ('2026-10-19', '2026-04-20', -1))
        self.assertEqual((record['n_days'], record['min_days'], record['verdict']), (0, 80, 'pending'))
        for status in ('accepted', 'rejected', 'candidate', 'accepted'):
            entry.update(status=status, direction=1)
            self.assertIs(forward.register(entry, {'holdout': ('2026-05-20', None)}, '2026-11-19'), record)
            self.assertEqual(record['direction'], -1)
            self.assertEqual(record['start'], '2026-04-20')
            self.assertEqual(record['accepted_at'], '2026-10-19')

    def test_verdict_rules_and_boundaries(self):
        cases = [(39, 40, .1, 5, .1, 'pending'),
                 (40, 40, .1, 1.65, .1, 'confirmed'),
                 (40, 40, 0., 5, .1, 'failed'),
                 (40, 40, -.1, -5, -.1, 'failed'),
                 (40, 40, .1, 1.64, .1, 'inconclusive'),
                 (40, 40, .1, 2, 0., 'inconclusive'),
                 (40, 40, None, None, None, 'inconclusive')]
        for *args, expected in cases:
            with self.subTest(args=args):
                self.assertEqual(forward.verdict(*args), expected)

    def test_labels_exact_alignment_benchmarks_missing_and_sealed_tail(self):
        for market in ('crypto', 'us'):
            panels = planted(rows=100, columns=20, market=market)
            index = panels.mask.index
            start = index[80].strftime('%Y-%m-%d')
            for h in (1, 3, 7):
                excess, raw = panel.forward_labels(panels, h, start)
                expected = panels.fields['open'].shift(-1-h) / panels.fields['open'].shift(-1) - 1
                expected.loc[index < start] = np.nan
                pd.testing.assert_frame_equal(raw, expected)
                if market == 'crypto':
                    baseline = expected.where(panels.mask).mean(axis=1)
                else:
                    b = panels.benchmark_open
                    baseline = b.shift(-1-h) / b.shift(-1) - 1
                pd.testing.assert_frame_equal(excess, expected.sub(baseline, axis=0))
                self.assertEqual(raw.notna().any(axis=1).sum(), 20-h-1)
                sealed = panel.forward_target(panels, h)[0]
                self.assertTrue(sealed.loc[index >= start].isna().all().all())
            panels.fields['open'].iloc[90, 0] = np.nan
            raw = panel.forward_labels(panels, 3, start)[1]
            self.assertTrue(np.isnan(raw.iloc[86, 0]))
            self.assertTrue(np.isnan(raw.iloc[89, 0]))
            excess, raw = panel.forward_labels(panels, 3, '2099-01-01')
            self.assertTrue(excess.isna().all().all() and raw.isna().all().all())

    def test_metrics_match_frozen_evaluator_and_curve_downsampling(self):
        panels = planted(rows=650)
        values = evaluate_expr(SIGNAL_EXPR, panels.fields)
        start = values.index[250].strftime('%Y-%m-%d')
        for h in (1, 3, 20):
            for direction in (1, -1):
                entry = dict(forward=dict(start=start, direction=direction, accepted_at='2026-10-19'))
                excess, raw = panel.forward_labels(panels, h, start)
                result = forward.evaluate_forward(entry, values, panels, h, MARKETS['crypto'])
                segment = pd.Series(values.index >= start, index=values.index)
                ic = evaluate.cs_rank_ic(values*direction, excess, panels.mask, 15)
                expected = evaluate.segment_metrics(ic, values*direction, raw, panels.mask,
                                                    segment, h, MARKETS['crypto'])
                self.assertEqual(result['n_days'], expected.pop('n'))
                for key, value in expected.items():
                    self.assertAlmostEqual(result[key], value)
                self.assertEqual(result['min_days'], max(40, 4*h))
                self.assertEqual(result['last_date'], values.index[-h-2].strftime('%Y-%m-%d'))
                self.assertEqual(len(result['curve']), 120)
                sample = ic.dropna()
                self.assertEqual(result['curve'][0][0], start)
                self.assertEqual(result['curve'][-1][0], result['last_date'])
                self.assertAlmostEqual(result['curve'][-1][1], sample.sum())
                self.assertEqual(result['curve'], sorted(result['curve']))
                self.assertEqual(entry['forward'], dict(start=start, direction=direction, accepted_at='2026-10-19'))

    def test_only_targets_on_or_after_registered_start_affect_metrics(self):
        panels = planted(rows=250)
        values = evaluate_expr(SIGNAL_EXPR, panels.fields)
        start = values.index[120].strftime('%Y-%m-%d')
        entry = dict(forward=dict(start=start, direction=1, accepted_at='2026-10-19'))
        labels = panel.forward_labels(panels, 1, start)
        baseline = forward.evaluate_forward(entry, values, panels, 1, MARKETS['crypto'], labels=labels)
        poisoned = tuple(x.copy() for x in labels)
        for x in poisoned:
            x.loc[x.index < start] = 99999.
        self.assertEqual(baseline, forward.evaluate_forward(entry, values, panels, 1,
                                                           MARKETS['crypto'], labels=poisoned))
        for x in poisoned:
            x.loc[x.index >= start] *= -1
        altered = forward.evaluate_forward(entry, values, panels, 1, MARKETS['crypto'], labels=poisoned)
        self.assertNotEqual(baseline['ic'], altered['ic'])
        self.assertAlmostEqual(baseline['ic'], -altered['ic'])
        self.assertNotEqual(baseline['ls_ret'], altered['ls_ret'])

    def test_no_labels_and_constant_values(self):
        panels = planted(rows=80)
        values = evaluate_expr(SIGNAL_EXPR, panels.fields)
        entry = dict(forward=dict(start='2099-01-01', direction=1, accepted_at='2026-10-19'))
        result = forward.evaluate_forward(entry, values, panels, 20, MARKETS['crypto'])
        self.assertEqual((result['n_days'], result['min_days'], result['verdict']), (0, 80, 'pending'))
        self.assertEqual(result['curve'], [])
        self.assertIsNone(result['last_date'])
        entry['forward']['start'] = values.index[0].strftime('%Y-%m-%d')
        for constant in (values*0, 1+values*1e-16):
            self.assertEqual(forward.evaluate_forward(entry, constant, panels, 1,
                                                     MARKETS['crypto'])['n_days'], 0)

    def test_summary_counts_only_accepted(self):
        factors = [dict(horizon=1, status='accepted', forward={'verdict': v}) for v in forward.VERDICTS]
        factors += [dict(horizon=1, status='rejected', forward={'verdict': 'confirmed'}),
                    dict(horizon=3, status='candidate')]
        self.assertEqual(forward.summarize(factors),
                         {'1': dict.fromkeys(forward.VERDICTS, 1), '3': dict.fromkeys(forward.VERDICTS, 0)})


class ForwardPipelineTests(unittest.TestCase):
    def run_market(self, root, **kwargs):
        return miner.Miner(root).run_market('crypto', n_mutations=0, n_llm=0, budget_s=120,
            seed=42, use_llm=False, now=dt.datetime(2026, 10, 19, tzinfo=dt.timezone.utc), **kwargs)

    def test_training_direction_flips_demotion_repromotion_and_one_value_calculation(self):
        with tempfile.TemporaryDirectory() as root, \
             patch.dict(MARKETS['crypto'], windows=dict(train=300, val=100, holdout=60)), \
             patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
            write_market(root)
            first = self.run_market(root)
            original = next(f['forward'] for f in first['factors'] if f['horizon'] == 1)
            self.assertEqual(original['start'], '2021-08-13')
            def reverse_training(panels, h):
                excess, raw = panel.forward_target(panels, h)
                # The new training/validation signal relationship reverses.
                return -excess, raw
            with patch.dict(MARKETS['crypto'], windows=dict(train=300, val=100, holdout=50)), \
                 patch.object(miner, 'forward_target', side_effect=reverse_training), \
                 patch.object(miner, 'evaluate_expr', wraps=evaluate_expr) as values, \
                 patch.object(panel, 'forward_labels', wraps=panel.forward_labels) as labels, \
                 patch.object(ensemble, 'evaluate_expr', side_effect=AssertionError('uncached ensemble')):
                second = self.run_market(root)
            self.assertEqual(values.call_count, 1)
            self.assertEqual(labels.call_count, len({f['horizon'] for f in second['factors'] if 'forward' in f}))
            changed = next(f for f in second['factors'] if f['horizon'] == 1)
            self.assertEqual(changed['direction'], -1)
            self.assertEqual(changed['status'], 'candidate')
            self.assertEqual(changed['forward'], original)
            third = self.run_market(root)
            restored = next(f for f in third['factors'] if f['horizon'] == 1)
            self.assertEqual(restored['status'], 'accepted')
            self.assertEqual(restored['forward'], original)
            self.assertEqual(store.read_jsonl(Path(root)/'runs.jsonl')[-1]['forward_updated'],
                             sum('forward' in f for f in third['factors']))

    def test_budget_during_forward_discards_batch_and_preserves_old_registration(self):
        with tempfile.TemporaryDirectory() as root, \
             patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
            write_market(root)
            before = self.run_market(root)
            clock = [0.]
            evaluator = forward.evaluate_forward
            def slow(*args, **kwargs):
                result = evaluator(*args, **kwargs)
                result['ic'] = 99999.
                clock[0] = 121.
                return result
            with patch.object(miner.time, 'monotonic', side_effect=lambda: clock[0]), \
                 patch.object(forward, 'evaluate_forward', side_effect=slow):
                after = self.run_market(root)
            self.assertEqual([f.get('forward') for f in before['factors']],
                             [f.get('forward') for f in after['factors']])
            run = store.read_jsonl(Path(root)/'runs.jsonl')[-1]
            self.assertTrue(run['budget_exhausted'])
            self.assertEqual(run['forward_updated'], 0)

    def test_first_acceptance_is_registered_even_when_forward_budget_expires(self):
        with tempfile.TemporaryDirectory() as root, \
             patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
            write_market(root)
            clock = [0.]
            classify = miner.Miner._classify_and_correlate
            def slow_classify(*args):
                classify(*args)
                clock[0] = 121.
            with patch.object(miner.time, 'monotonic', side_effect=lambda: clock[0]), \
                 patch.object(miner.Miner, '_classify_and_correlate', side_effect=slow_classify), \
                 patch.object(forward, 'evaluate_forward', side_effect=AssertionError('budget exhausted')):
                result = self.run_market(root)
            registered = [f for f in result['factors'] if f['status'] == 'accepted']
            self.assertTrue(registered)
            for f in registered:
                self.assertEqual(f['forward']['accepted_at'], '2026-10-19')
                self.assertEqual(f['forward']['verdict'], 'pending')
                self.assertIsNone(f['forward']['ic'])
            self.assertEqual(store.read_jsonl(Path(root)/'runs.jsonl')[-1]['forward_updated'], 0)

    def test_rebuilt_zoo_entry_keeps_registration_and_reuses_values(self):
        with tempfile.TemporaryDirectory() as root, \
             patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
            write_market(root)
            first = self.run_market(root)
            original = next(f['forward'] for f in first['factors'] if f['horizon'] == 1)
            # Simulate an older zoo row without a corresponding ledger trial,
            # then an invalid refresh followed by candidate reconstruction.
            ledger = store.read_ledger('crypto', root)
            (Path(root)/'crypto/ledger.jsonl').unlink()
            store.append_ledger('crypto', [r for r in ledger if r['horizon'] != 1], root)
            evaluator, refreshed = evaluate.evaluate_factor, set()
            def once_invalid(*args):
                h = args[-2]
                if h == 1 and h not in refreshed:
                    refreshed.add(h)
                    return None
                return evaluator(*args)
            with patch.object(evaluate, 'evaluate_factor', side_effect=once_invalid), \
                 patch.object(miner, 'evaluate_expr', wraps=evaluate_expr) as values:
                result = self.run_market(root)
            rebuilt = next(f for f in result['factors'] if f['horizon'] == 1)
            self.assertEqual(rebuilt['forward'], original)
            self.assertEqual(values.call_count, 1)

    def test_forward_cannot_affect_classification_mutations_llm_ensemble_or_ledger(self):
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as other, \
             patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
            for directory in (root, other):
                write_market(directory)
                self.run_market(directory)
            path = Path(other)/'crypto/zoo.json'
            zoo = store.read_json(path)
            for f in zoo['factors']:
                if 'forward' in f:
                    f['forward'].update(ic=-987654.321, t=-987654.321, verdict='failed', direction=-1)
            store.write_json(path, zoo)
            prompts, pools = [], []
            def proposal(work, prompt, out):
                prompts.append(prompt.read_bytes())
                self.assertNotIn(b'forward', prompts[-1])
                self.assertNotIn(b'987654', prompts[-1])
                out.write_text('["$volume"]')
            mutation = miner.mutate
            def mutate(expr, rng, pool):
                self.assertTrue(all('forward' not in f for f in pool))
                pools.append(copy.deepcopy(pool))
                return mutation(expr, rng, pool)
            with patch.object(miner, 'mutate', side_effect=mutate):
                for directory in (root, other):
                    miner.Miner(directory).run_market('crypto', n_mutations=1, n_llm=1,
                        budget_s=120, seed=42, use_llm=True, runner=proposal,
                        now=dt.datetime(2026, 10, 20, tzinfo=dt.timezone.utc))
            self.assertEqual(prompts[0], prompts[1])
            self.assertEqual(pools[:len(pools)//2], pools[len(pools)//2:])
            for name in ('ledger.jsonl', 'ensemble.json'):
                self.assertEqual((Path(root)/'crypto'/name).read_bytes(), (Path(other)/'crypto'/name).read_bytes())
            def selection(directory):
                value = store.read_json(Path(directory)/'crypto/zoo.json')
                for f in value['factors']:
                    f.pop('forward', None)
                for h in value['horizons'].values():
                    h.pop('forward')
                return json.dumps(value, separators=(',', ':'))
            self.assertEqual(selection(root), selection(other))

    def test_full_seed_run_at_real_panel_scale(self):
        panels = planted(rows=2500, columns=200)
        panels.windows = MARKETS['crypto']['windows']
        with tempfile.TemporaryDirectory() as root, \
             patch.object(miner, 'load_panels', return_value=panels), \
             patch.object(miner, 'evaluate_expr', wraps=evaluate_expr) as values, \
             patch.object(panel, 'forward_labels', wraps=panel.forward_labels) as labels, \
             patch.object(ensemble, 'evaluate_expr', side_effect=AssertionError('uncached ensemble')):
            begin = time.perf_counter()
            result = miner.Miner(root).run_market('crypto', n_mutations=0, n_llm=0,
                                                 budget_s=180, seed=42, use_llm=False)
            elapsed = time.perf_counter() - begin
            self.assertLess(elapsed, 180., f'full seed run (2500x200) took {elapsed:.2f}s')
            self.assertEqual(values.call_count, len(miner.zoo.seed_factors()))
            run = store.read_jsonl(Path(root)/'runs.jsonl')[-1]
            self.assertFalse(run['budget_exhausted'])
            self.assertGreater(run['forward_updated'], 0)
            self.assertLessEqual(labels.call_count, 3)
            self.assertTrue(result['factors'])
