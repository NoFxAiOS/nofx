"""Validate the full deterministic seed library on synthetic daily panels."""

import unittest

import numpy as np
import pandas as pd

from scripts.factor_mining.expr import canonical, evaluate_expr, expr_hash, validate_expr
from scripts.factor_mining.tests.test_expr import make_panels
from scripts.factor_mining.zoo import seed_factors


class SeedFactorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.factors = seed_factors()

    def test_count_schema_families_and_determinism(self):
        self.assertGreaterEqual(len(self.factors), 120)
        self.assertLessEqual(len(self.factors), 260)
        families = {"candle", "momentum", "reversal", "volatility", "volume",
                    "price_volume", "drawdown", "other"}
        self.assertEqual({factor["family"] for factor in self.factors}, families)
        self.assertEqual(self.factors, seed_factors())
        names = []
        for factor in self.factors:
            with self.subTest(name=factor["name"]):
                self.assertEqual(set(factor), {"expr", "family", "name", "id"})
                self.assertTrue(all(isinstance(value, str) and value for value in factor.values()))
                self.assertIn(factor["family"], families)
                self.assertEqual(factor["id"], expr_hash(factor["expr"]))
                names.append(factor["name"])
        self.assertEqual(len(names), len(set(names)))
        changed = seed_factors()
        changed[0]["expr"] = "0"
        self.assertEqual(self.factors, seed_factors())

    def test_all_validate_and_canonical_expressions_are_unique(self):
        keys = []
        ids = []
        for factor in self.factors:
            with self.subTest(name=factor["name"]):
                meta = validate_expr(factor["expr"])
                self.assertLessEqual(meta["nodes"], 40)
                self.assertLessEqual(meta["depth"], 8)
                self.assertLessEqual(meta["max_window"], 120)
                keys.append(canonical(factor["expr"]))
                ids.append(factor["id"])
        self.assertEqual(len(keys), len(set(keys)))
        self.assertEqual(len(ids), len(set(ids)))

    def test_requested_templates_and_all_five_windows(self):
        names = {factor["name"] for factor in self.factors}
        self.assertTrue({"KMID", "KLEN", "KUP", "KLOW", "KSFT", "KMID2",
                         "KUP2", "KLOW2", "KSFT2", "TURNOVER_SHOCK"} <= names)
        templates = ("ROC", "MOM", "MA", "STD", "BETA", "RSQR", "RESI", "MAX",
                     "MIN", "QTLU", "QTLD", "RANK", "RSV", "IMAX", "IMIN", "IMXD",
                     "CORR", "CORD", "CNTP", "CNTN", "CNTD", "SUMP", "SUMN", "SUMD",
                     "VMA", "VSTD", "WVMA", "VSUMP", "VSUMN", "VSUMD", "DRAWDOWN",
                     "AMPLITUDE", "GAP", "RV", "DOWNSIDE", "SKEW", "KURT")
        for template in templates:
            for n in (5, 10, 20, 30, 60):
                self.assertIn(f"{template}{n}", names)

    def test_all_evaluate_with_at_least_ninety_percent_usable(self):
        panels = make_panels(180, 8)
        panels["close"].iloc[0, 0] = np.nan
        panels["volume"].iloc[15, 1] = 0
        usable = 0
        for factor in self.factors:
            with self.subTest(name=factor["name"]):
                result = evaluate_expr(factor["expr"], panels)
                self.assertIsInstance(result, pd.DataFrame)
                self.assertEqual(result.shape, panels["close"].shape)
                pd.testing.assert_index_equal(result.index, panels["close"].index)
                pd.testing.assert_index_equal(result.columns, panels["close"].columns)
                self.assertFalse(np.isinf(result.to_numpy()).any())
                usable += bool(result.notna().any().any())
        self.assertGreaterEqual(usable / len(self.factors), 0.9)


if __name__ == "__main__":
    unittest.main()
