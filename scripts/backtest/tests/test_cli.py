import csv
import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stderr
from unittest.mock import patch

from scripts.backtest.cli import main
from scripts.backtest.exitsim import run_variants
from scripts.backtest.tests.test_exitsim import P, T


class BacktraderCliTest(unittest.TestCase):
    def test_missing_framework_exits_without_fallback(self):
        err = io.StringIO()
        with patch.dict('sys.modules', {'backtrader': None}), redirect_stderr(err):
            self.assertEqual(main(['exit']), 2)
        self.assertIn('不会回退到自写模拟器', err.getvalue())
        self.assertIn('requirements.txt', err.getvalue())

    def test_csv_records_engine_and_actual_variant_parameters(self):
        tr = T([(100, 100, 100, 100), (100, 101, 99, 100), (100, 111, 99, 110)])
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, 'result.csv')
            run_variants([tr], P(), [['baseline', {}], ['capped', {'tp_cap': 1}]],
                         csv_path=path, commission=0, slippage=0, out=lambda _: None)
            with open(path) as f:
                rows = list(csv.DictReader(f))
        self.assertEqual([r['engine'] for r in rows], ['backtrader', 'backtrader'])
        self.assertTrue(all(r['engine_version'] == '1.9.78.123' for r in rows))
        self.assertEqual(json.loads(rows[1]['params'])['tp_cap'], 1)
        self.assertEqual(float(rows[0]['sumR']), 2)
        self.assertEqual(float(rows[1]['sumR']), 1)

    def test_invalid_execution_settings_are_rejected(self):
        with redirect_stderr(io.StringIO()):
            self.assertEqual(main(['exit', '--commission', 'nan']), 2)
            self.assertEqual(main(['exit', '--limit-valid-min', '0']), 2)


if __name__ == '__main__':
    unittest.main()
