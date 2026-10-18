import json
import math
from pathlib import Path
import tempfile
import time
import unittest
from unittest.mock import patch
import numpy as np
from scripts.autoresearch import evaluate as ev, judge
from scripts.autoresearch.tests.helpers import synthetic_dataset, candidate


class EvaluatorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.root = Path(cls.temp.name)
        _, cls.meta = synthetic_dataset(cls.root)
        cls.work = cls.root / 'work'
        cls.good = candidate(cls.work / 'good.py')
        cls.noise = candidate(cls.work / 'noise.py', 'noise')

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def test_signal_high_fitness_noise_low_and_val_redacted(self):
        ledger = self.work / 'test_trials.jsonl'
        good = ev.evaluate(self.good, 'long', 24, 'train', self.work, ledger)
        self.assertTrue(good['valid'], good)
        self.assertGreater(good['fitness'], 10)
        self.assertGreater(good['q5_net'], good['q1_net'])
        self.assertGreater(good['win_uplift'], .2)
        self.assertGreater(good['monotonicity'], .9)
        noise = ev.evaluate(self.noise, 'long', 24, 'train', self.work, ledger)
        self.assertTrue(noise['valid'], noise)
        self.assertLess(noise['fitness'], 2)
        val = ev.evaluate(self.good, 'long', 24, 'val', self.work, ledger)
        self.assertEqual(val, {'valid': True, 'pass': True, 'val_ic_t_bucket': '>2'})
        nv = ev.evaluate(self.noise, 'long', 24, 'val', self.work, ledger)
        self.assertFalse(nv['pass'])
        self.assertEqual(set(nv), {'valid', 'pass', 'val_ic_t_bucket'})
        rows = ev.ledger_rows(ledger)
        self.assertEqual(len(rows), 2)
        self.assertTrue(all(set(r) == {'ts', 'side', 'horizon', 'split', 'candidate_sha256', 'fitness', 'ic_t', 'qs', 'n_features'} for r in rows))

    def test_ast_rejections_no_trial_and_pointwise_allowlist(self):
        bads = ['import os\nFEATURES=["noise"]\ndef score(X): return X["noise"]',
                'FEATURES=["noise"]\ndef score(X): return open("/tmp/x")',
                'FEATURES=["noise"]\ndef score(X): return X["noise"].__class__',
                'FEATURES=["noise"]\ndef score(X): return getattr(X,"noise")',
                'FEATURES=["noise"]\ndef score(X): return [v for v in X["noise"]]',
                'FEATURES=["noise"]\ndef score(X):\n for x in X: pass\n return X["noise"]',
                'FEATURES=["noise"]\ndef score(X): return X["long_net_24"]',
                'import numpy as np\nFEATURES=["noise"]\ndef score(X): return np.load("x")',
                'FEATURES=["noise"]\ndef score(X): return X["noise"][0]',
                'FEATURES=["noise"]\ndef score(X): return eval("1")',
                'FEATURES=["noise"]\ndef score(X): return __import__("os")',
                'import numpy as np\nFEATURES=["noise"]\ndef score(X): return np.mean(X["noise"])']
        path = self.work / 'bad.py'
        ledger = self.work / 'bad_ledger.jsonl'
        for source in bads:
            path.write_text(source)
            result = ev.evaluate(path, 'long', 24, 'train', self.work, ledger)
            self.assertFalse(result['valid'], source)
        self.assertFalse(ledger.exists())
        source = 'import numpy as np\nFEATURES=["noise"]\ndef score(X: dict) -> "np.ndarray":\n z=np.nan_to_num(X["noise"], nan=0.0)\n return np.clip(z,-2,2)\n'
        scores, features = ev.score_candidate(source, self.meta['features'], np.ones((10, 6)) * np.arange(10)[:, None])
        self.assertEqual(features, ['noise'])
        self.assertEqual(scores.shape, (10,))
        with self.assertRaises(ValueError):
            ev.score_candidate('FEATURES=["noise"]\ndef score(X): return X["noise"]*0+1', self.meta['features'], np.ones((10, 6)))

    def test_threshold_trials_direction_and_budget(self):
        vals = [ev.t_required(k) for k in (1, 2, 10, 100, 10000)]
        self.assertEqual(vals[0], 2)
        self.assertTrue(all(b > a for a, b in zip(vals, vals[1:])))
        for k in (2, 10, 10000):
            self.assertAlmostEqual(.5 * math.erfc(ev.t_required(k) / math.sqrt(2)), .025 / k)
        ledger = self.work / 'budget.jsonl'
        for i in range(8):
            ev.append_ledger(ledger, {'side': 'short', 'horizon': 72, 'split': 'train'})
        self.assertEqual(ev.valid_trials(ledger, 'long', 24), 0)
        with patch.dict('os.environ', {'AR_ROUND_LEDGER_START': '0'}):
            r = ev.evaluate(self.good, 'short', 72, 'train', self.work, ledger)
        self.assertFalse(r['valid'])
        self.assertIn('budget', r['reason'])
        self.assertEqual(ev.valid_trials(ledger, 'short', 72), 8)

    def test_worker_timeout(self):
        def sleepy(*args):
            time.sleep(10)
        with patch.object(ev, '_score_worker', sleepy), self.assertRaisesRegex(ValueError, 'timeout'):
            ev.score_candidate(self.good.read_text(), self.meta['features'], np.random.default_rng(1).normal(size=(20, 6)), timeout=.05)

    def test_judge_guard_and_audit(self):
        private = self.root / 'private'
        with self.assertRaisesRegex(ValueError, 'spends-the-holdout'):
            judge.judge(self.good, 'long', 24, private)
        self.assertFalse((private / 'judge_log.jsonl').exists())
        r = judge.judge(self.good, 'long', 24, private, acknowledged=True)
        self.assertTrue(r['valid'])
        self.assertGreater(r['fitness'], 10)
        row = ev.ledger_rows(private / 'judge_log.jsonl')[-1]
        self.assertEqual(row['candidate_sha256'], ev.sha256(self.good))
        self.assertTrue(row['acknowledged'])
        bad = self.work / 'judge_bad.py'
        bad.write_text('import os')
        with self.assertRaises(ValueError):
            judge.judge(bad, 'long', 24, private, acknowledged=True)
        self.assertEqual(len(ev.ledger_rows(private / 'judge_log.jsonl')), 2)

    def test_dataset_checksum_rejects_changes(self):
        with tempfile.TemporaryDirectory() as tmp:
            synthetic_dataset(tmp)
            work = Path(tmp) / 'work'
            c = candidate(work / 'c.py')
            with (work / 'visible.npz').open('ab') as stream:
                stream.write(b'changed')
            r = ev.evaluate(c, 'long', 24, 'train', work)
            self.assertFalse(r['valid'])
            self.assertIn('sha256', r['reason'])


if __name__ == '__main__':
    unittest.main()
