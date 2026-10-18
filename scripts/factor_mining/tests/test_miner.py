import json
from pathlib import Path
import random
import subprocess
import tempfile
import unittest
from unittest.mock import patch, Mock

from scripts.factor_mining import miner, store
from scripts.factor_mining.expr import canonical, expr_hash, validate_expr
from scripts.factor_mining.miner import Miner, family, llm_propose, mutate
from scripts.factor_mining.tests.synth import SIGNAL_EXPR, SIGNAL_SEED, write_market


class MinerTests(unittest.TestCase):
    def test_mutations_legal_unique_and_reproducible(self):
        pool = [SIGNAL_EXPR, '$volume', 'Mean($close,10)']
        for expression in ('Mean($close,10)', 'Std($ret,20)', 'Corr($close,$volume,30)',
                           'Skew($ret,5)', 'Max($open,5)', SIGNAL_EXPR):
            a = mutate(expression, random.Random(81), pool)
            self.assertEqual(a, mutate(expression, random.Random(81), pool))
            self.assertGreater(len(a), 3)
            self.assertEqual(len(a), len(set(a)))
            self.assertNotIn(canonical(expression), a)
            for expr in a:
                validate_expr(expr)
        variants = mutate('Mean($close,10)', random.Random(81), pool)
        self.assertTrue(any('EMA' in e for e in variants))
        self.assertTrue(any('$vwap' in e for e in variants))
        self.assertTrue(any('Mean($close,7)' in e for e in variants))
        self.assertTrue(any('Mean($close,14)' in e for e in variants))

    def test_family_priority(self):
        self.assertEqual(family('Corr($close,$volume,5)'), 'price_volume')
        self.assertEqual(family('Std($volume,5)'), 'volume')
        self.assertEqual(family('Kurt($ret,5)'), 'volatility')
        self.assertEqual(family('Delta($close,5)'), 'momentum')
        self.assertEqual(family('$open'), 'other')

    def test_llm_last_array_invalid_counts_and_errors(self):
        with tempfile.TemporaryDirectory() as root:
            def fake(work, prompt, out):
                self.assertEqual(work, Path(root).resolve())
                text = prompt.read_text()
                self.assertIn('train_t', text)
                self.assertNotIn('val', text)
                out.write_text('示例 ["$high"]\n结果\n```json\n' +
                               json.dumps(['$close', '__import__("os")', '$volume', 12]) + '\n```')
            exprs, stats = llm_propose('crypto', '[{"expr":"$close","train_t":3}]',
                                      '[]', 20, fake, root)
            self.assertEqual(exprs, ['$close', '$volume'])
            self.assertEqual((stats['llm_proposed'], stats['llm_valid'], stats['llm_invalid']), (4, 2, 2))
            self.assertIsNone(stats['llm_error'])
            self.assertEqual({p.name for p in Path(root).iterdir()}, {'prompt.md', 'out.md'})
            def bad(*args):
                raise RuntimeError('fake model failed')
            exprs, stats = llm_propose('us', '[]', '[]', 20, bad, root)
            self.assertEqual(exprs, [])
            self.assertIn('fake model failed', stats['llm_error'])
            self.assertEqual(stats['llm_invalid'], 0)

    def test_model_deadline_kills_process_group(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            process = Mock(pid=12345)
            process.communicate.side_effect = subprocess.TimeoutExpired('fake', 900)
            with patch.object(miner.subprocess, 'Popen', return_value=process) as popen, \
                 patch.object(miner.os, 'killpg') as kill:
                with self.assertRaises(subprocess.TimeoutExpired):
                    miner.model_call(root, root/'prompt.md', root/'out.md')
                self.assertTrue(popen.call_args.kwargs['start_new_session'])
                self.assertEqual(process.communicate.call_args.kwargs['timeout'], 900)
                kill.assert_called_once()

    def test_redundancy_dedup_and_history_reclassification(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root)
            search = Miner(root)
            seeds = [SIGNAL_SEED, dict(SIGNAL_SEED, expr=f'2*({SIGNAL_EXPR})')]
            with patch.object(miner.zoo, 'seed_factors', return_value=seeds):
                output = search.run_market('crypto', n_mutations=0, n_llm=0, budget_s=120,
                                            seed=1, use_llm=False)
                first = store.read_ledger('crypto', root)
                h1 = [f for f in output['factors'] if f['horizon'] == 1]
                self.assertEqual({f['status'] for f in h1}, {'accepted', 'redundant'})
                redundant = next(f for f in h1 if f['status'] == 'redundant')
                self.assertAlmostEqual(redundant['corr_max'], 1.)
                self.assertIsNotNone(redundant['corr_with'])
                search.run_market('crypto', n_mutations=0, n_llm=0, budget_s=120, seed=1, use_llm=False)
                self.assertEqual(first, store.read_ledger('crypto', root))
            # Historical experiment just cleared its old threshold. More honest
            # ledger trials must demote it without evaluating that expression again.
            previous = store.read_json(Path(root)/'crypto/zoo.json')
            accepted = next(f for f in previous['factors'] if f['horizon'] == 1 and f['status'] == 'accepted')
            accepted['train']['t'] = previous['horizons']['1']['t_req'] + .001
            store.write_zoo('crypto', previous, root)
            extra = [dict(first[0], id=f'f_history{i:05}', status='rejected', train_t=0.) for i in range(100)]
            store.append_ledger('crypto', extra, root)
            with patch.object(miner.zoo, 'seed_factors', return_value=seeds):
                output = search.run_market('crypto', n_mutations=0, n_llm=0, budget_s=120,
                                            seed=1, use_llm=False)
            updated = next(f for f in output['factors'] if f['horizon'] == 1 and f['id'] == accepted['id'])
            self.assertEqual(updated['status'], 'rejected')
            self.assertEqual(updated['status_history'][-1]['from'], 'accepted')
            self.assertEqual(updated['status_history'][-1]['to'], 'rejected')
            self.assertEqual(output['horizons']['1']['trials'], 102)
            for h in output['horizons'].values():
                self.assertEqual(h['trials'], sum(h[s] for s in ('accepted', 'candidate', 'rejected', 'redundant')))

    def test_llm_train_only_view_invalid_and_valid_trial_accounting(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root)
            search = Miner(root)
            with patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
                search.run_market('crypto', n_mutations=0, n_llm=0, budget_s=120, seed=2, use_llm=False)
                path = Path(root)/'crypto/zoo.json'
                old = store.read_json(path)
                for f in old['factors']:
                    f['val']['ic'] = 987654.321
                store.write_json(path, old)
                before = len(store.read_ledger('crypto', root))
                def fake(work, prompt, out):
                    text = prompt.read_text()
                    self.assertNotIn('987654', text)
                    self.assertNotIn('val', text)
                    self.assertEqual({p.name for p in work.iterdir()}, {'prompt.md'})
                    out.write_text(json.dumps(['$volume', 'bad()', '0', SIGNAL_EXPR]))
                search.run_market('crypto', n_mutations=0, n_llm=20, budget_s=120, seed=2,
                                  use_llm=True, runner=fake)
            run = store.read_jsonl(Path(root)/'runs.jsonl')[-1]
            self.assertEqual((run['llm_proposed'], run['llm_valid'], run['llm_invalid']), (4, 3, 1))
            self.assertEqual(len(store.read_ledger('crypto', root)) - before, 3)
            self.assertEqual(run['trials_total'], before+3)
            self.assertTrue(run['invalid'] >= 3)

    def test_budget_finishes_normally(self):
        with tempfile.TemporaryDirectory() as root:
            write_market(root, rows=150, columns=15)
            with patch.object(miner.zoo, 'seed_factors', return_value=[SIGNAL_SEED]):
                out = Miner(root).run_market('crypto', n_mutations=0, n_llm=0, budget_s=0,
                                             seed=1, use_llm=False)
            self.assertFalse(out['factors'])
            self.assertEqual(store.read_ledger('crypto', root), [])
            self.assertTrue(store.read_jsonl(Path(root)/'runs.jsonl')[-1]['budget_exhausted'])
            self.assertFalse(store.read_json(Path(root)/'status.json')['running'])
