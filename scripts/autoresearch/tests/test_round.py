import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from scripts.autoresearch import init_work, run_round, evaluate
from scripts.autoresearch.tests.helpers import synthetic_dataset, candidate
from scripts.autoresearch.common import sha256


class RoundTests(unittest.TestCase):
    def test_init_evaluate_and_dry_run_rotation_cli(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic_dataset(root)
            # Copy to separate work root: the copied evaluate.py must work on its own.
            fake = root / 'other_repo/scripts/backtest'
            fake.mkdir(parents=True)
            (fake / '__init__.py').write_text('')
            (fake / 'short_features_study.py').write_text('raise AssertionError("wrong repository")')
            work = init_work.initialize(root / 'work', root / 'other_repo/data/autoresearch/work')
            frozen = work / 'evaluate.py'
            self.assertEqual(frozen.stat().st_mode & 0o222, 0)
            self.assertFalse((work / 'holdout.npz').exists())
            result = subprocess.run([sys.executable, str(frozen), '--candidate', str(work / 'candidate_long_24.py'),
                                     '--side', 'long', '--horizon', '24', '--split', 'train', '--work-dir', str(work)],
                                    capture_output=True, text=True, check=True)
            self.assertTrue(json.loads(result.stdout)['valid'])
            before = (work / 'ledger.jsonl').read_bytes()
            init_work.initialize(root / 'work', work)
            self.assertEqual(before, (work / 'ledger.jsonl').read_bytes())
            # A rebuild must never replace a dataset after init freezes it.
            checksum = sha256(work / 'visible.npz')
            with self.assertRaisesRegex(ValueError, 'frozen'):
                synthetic_dataset(work.parent)
            self.assertEqual(checksum, sha256(work / 'visible.npz'))
            script = Path(run_round.__file__).with_name('run_round.sh')
            env = dict(os.environ, AR_DRY_RUN='1', AR_PYTHON=sys.executable)
            original = (work / 'candidate_short_24.py').read_bytes()
            for pair in [('short', 24), ('long', 24)]:
                p = subprocess.run([str(script), '--work-dir', str(work)], env=env, text=True,
                                   capture_output=True, check=True)
                row = json.loads(p.stdout)
                self.assertEqual((row['side'], row['horizon']), pair)
                self.assertIsNone(row['error'])
                self.assertEqual(row['trials'], 2)
            self.assertEqual(original, (work / 'candidate_short_24.py').read_bytes())
            self.assertTrue((work / 'round_log.md').exists())
            self.assertTrue((work / 'round.log').exists())

    def test_rollback_model_failure_and_frozen_tamper(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic_dataset(root)
            work = init_work.initialize(root / 'work')
            original = (work / 'candidate_short_24.py').read_bytes()
            frozen = (work / 'evaluate.py').read_bytes()
            def bad_model(*args):
                (work / 'candidate_short_24.py').write_text('broken')
                (work / 'evaluate.py').chmod(0o644)
                (work / 'evaluate.py').write_text('tampered')
                raise RuntimeError('mock timeout')
            with patch.object(run_round, 'model_call', bad_model):
                row = run_round.run(work)
            self.assertIn('mock timeout', row['error'])
            self.assertEqual(original, (work / 'candidate_short_24.py').read_bytes())
            self.assertEqual(frozen, (work / 'evaluate.py').read_bytes())
            self.assertEqual((work / 'evaluate.py').stat().st_mode & 0o222, 0)
            self.assertEqual(evaluate.valid_trials(work / 'ledger.jsonl', 'short', 24), 1)
            run_round.verify(work)

    def test_promoted_bytes_match_evaluated_proposal_even_when_restored(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic_dataset(root)
            work = init_work.initialize(root / 'work')
            path = work / 'candidate_short_24.py'
            old = path.read_bytes()
            proposal = 'import numpy as np\nFEATURES=["noise"]\ndef score(X): return X["noise"]\n'
            def model(*args):
                path.write_text(proposal)
            calls = [dict(valid=True, fitness=10), dict(valid=True, fitness=9, candidate_sha256='test'),
                     {'valid': True, 'pass': True, 'val_ic_t_bucket': '>2'}]
            with patch.object(run_round, 'model_call', model), patch.object(run_round, 'evaluate', side_effect=calls):
                row = run_round.run(work)
            self.assertIsNone(row['error'])
            self.assertEqual(path.read_bytes(), old)
            promoted = list((work / 'promoted').glob('*.py'))
            self.assertEqual(len(promoted), 1)
            self.assertEqual(promoted[0].read_text(), proposal)


if __name__ == '__main__':
    unittest.main()
