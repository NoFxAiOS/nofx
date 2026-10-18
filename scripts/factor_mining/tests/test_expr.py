"""Offline numerical, syntax, causality and performance checks for the DSL."""

import ast
import hashlib
import re
import time
import unittest

import numpy as np
import pandas as pd

from scripts.factor_mining.expr import (
    FIELDS, OPERATORS, ExprError, canonical, dsl_help, evaluate_expr, expr_hash,
    validate_expr,
)


def make_panels(rows=100, columns=4, seed=20261018):
    """Deterministic synthetic OHLCV, with changing trends and trading volume."""
    rng = np.random.default_rng(seed)
    close = 100 * np.exp(np.cumsum(rng.normal(0.001, 0.025, (rows, columns)), axis=0))
    previous = np.vstack((close[:1], close[:-1])) if rows else close
    open_ = previous * np.exp(rng.normal(0, 0.01, (rows, columns)))
    high = np.maximum(open_, close) * (1 + rng.uniform(0, 0.03, (rows, columns)))
    low = np.minimum(open_, close) * (1 - rng.uniform(0, 0.03, (rows, columns)))
    volume = rng.lognormal(9, 0.6, (rows, columns))
    index = pd.date_range("2020-01-01", periods=rows, name="date")
    labels = pd.Index([f"asset_{i}" for i in range(columns)], name="asset")
    return {name: pd.DataFrame(values, index=index, columns=labels)
            for name, values in zip(FIELDS, (open_, high, low, close, volume,
                                             (open_ + high + low + close) / 4))}


class ExpressionTests(unittest.TestCase):
    def setUp(self):
        self.panels = make_panels(18, 4)
        values = np.array([2, 1, 1, 4, 3, 3, 6, 2, 8, 7, 9, 10, 11, 12, 13, 11, 15, 16.])
        x = np.column_stack((values, -values, values ** 1.1, np.full(18, 3.)))
        x[0, 1] = np.nan
        x[10, 2] = np.nan
        self.x = pd.DataFrame(x, index=self.panels["close"].index,
                              columns=self.panels["close"].columns)
        self.panels["close"] = self.x

    def check(self, expr, expected, *, atol=1e-10, rtol=1e-10):
        if not isinstance(expected, pd.DataFrame):
            expected = pd.DataFrame(np.broadcast_to(expected, self.x.shape),
                                    index=self.x.index, columns=self.x.columns)
        expected = expected.astype(float).replace([np.inf, -np.inf], np.nan)
        actual = evaluate_expr(expr, self.panels)
        pd.testing.assert_frame_equal(actual, expected, check_exact=False,
                                      atol=atol, rtol=rtol)
        self.assertTrue(all(dtype == np.float64 for dtype in actual.dtypes))
        return actual

    def manual_windows(self, function, n=5):
        out = np.full(self.x.shape, np.nan)
        data = self.x.to_numpy()
        for row in range(n - 1, len(data)):
            for col in range(data.shape[1]):
                window = data[row - n + 1:row + 1, col]
                if np.isfinite(window).all():
                    out[row, col] = function(window)
        return out

    def test_fields_constants_and_derived_ret(self):
        for field in FIELDS[:-1]:
            with self.subTest(field=field):
                self.check("$" + field, self.panels[field])
        self.check("2.5", 2.5)
        self.check("-2", -2.)
        self.panels["ret"] = self.x * 0 + 123
        self.check("$ret", self.x / self.x.shift(1) - 1)
        self.assertEqual(validate_expr("$ret")["max_window"], 1)

    def test_arithmetic_and_scalar_broadcasting(self):
        cases = {
            "$close+2": self.x + 2,
            "2-$close": 2 - self.x,
            "$close*$open": self.x * self.panels["open"],
            "$close/$open": self.x / self.panels["open"],
            "2/$close": 2 / self.x,
            "-$close": -self.x,
            "($close+2)*3-1": (self.x + 2) * 3 - 1,
            "Mean(2,5)": pd.DataFrame(2., index=self.x.index,
                                      columns=self.x.columns).rolling(5).mean(),
        }
        for expr, expected in cases.items():
            with self.subTest(expr=expr):
                self.check(expr, expected)

    def test_division_zero_overflow_and_input_infinities(self):
        for expr in ("$close/0", "0/0", "1/0", "$close/($close-$close)",
                     "Power(2,10000)", "1e308*1e308"):
            with self.subTest(expr=expr):
                self.check(expr, np.nan)
        with np.errstate(over="ignore"):
            self.check("Power($close,10000)", self.x ** 10000)
        panels = {name: frame.copy() for name, frame in self.panels.items()}
        panels["close"].iloc[1, 0] = np.inf
        panels["close"].iloc[2, 0] = -np.inf
        result = evaluate_expr("$close", panels)
        self.assertTrue(result.iloc[1:3, 0].isna().all())
        self.assertFalse(np.isinf(result.to_numpy()).any())

    def test_comparisons_and_if(self):
        lag = self.x.shift(1)
        for op, expected in [(">", self.x > lag), ("<", self.x < lag),
                             (">=", self.x >= lag), ("<=", self.x <= lag)]:
            with self.subTest(op=op):
                self.check(f"$close{op}Ref($close,1)",
                           expected.astype(float).mask(self.x.isna() | lag.isna()))
        # At a missing comparison, take the false branch even when x is missing.
        self.check("If($close>Ref($close,1),$close,-2)",
                   np.where(self.x > lag, self.x, -2.))
        self.check("If(2>1,4,-2)", 4.)
        self.check("If(2<1,$close,3)", 3.)

    def test_elementwise_operators(self):
        cases = {
            "Abs($close)": self.x.abs(),
            "Sign($close-3)": np.sign(self.x - 3),
            "Log($close-2)": np.log((self.x - 2).where(self.x > 2)),
            "Sqrt($close-2)": np.sqrt((self.x - 2).where(self.x >= 2)),
            "Power($close,-2)": self.x ** -2,
            "Power($close,0.5)": np.sqrt(self.x.where(self.x >= 0)),
            "Clip($close,-2,5)": self.x.clip(-2, 5),
            "Greater($close,2)": np.maximum(self.x, 2),
            "Less($close,2)": np.minimum(self.x, 2),
            "Clip(-5,-1,1)": -1.,
            "Log(0)": np.nan,
            "Sqrt(-1)": np.nan,
        }
        for expr, expected in cases.items():
            with self.subTest(expr=expr):
                self.check(expr, expected)

    def test_ref_and_delta(self):
        for n in (1, 3, 120):
            with self.subTest(n=n):
                self.check(f"Ref($close,{n})", self.x.shift(n))
                self.check(f"Delta($close,{n})", self.x - self.x.shift(n))

    def test_pandas_rolling_operators_and_warmup(self):
        rolling = self.x.rolling(5, min_periods=5)
        methods = {"Mean": "mean", "Sum": "sum", "Std": "std", "Var": "var",
                   "Max": "max", "Min": "min", "Median": "median",
                   "Skew": "skew", "Kurt": "kurt"}
        for name, method in methods.items():
            with self.subTest(operator=name):
                result = self.check(f"{name}($close,5)", getattr(rolling, method)())
                self.assertTrue(result.iloc[:4].isna().all().all())
                self.assertTrue(result.iloc[10:15, 2].isna().all())
        self.check("Rank($close,5)", rolling.rank(method="average", pct=True))
        for q in (0, 0.2, 0.8, 1):
            with self.subTest(q=q):
                self.check(f"Quantile($close,5,{q})", rolling.quantile(q))
        for name in methods:
            self.assertTrue(evaluate_expr(f"{name}($close,120)", self.panels).isna().all().all())

    def test_ema(self):
        self.check("EMA($close,5)", self.x.ewm(span=5, adjust=False, min_periods=5).mean())

    def test_wma_mad_and_extreme_distances(self):
        n = 5
        weights = np.arange(1, n + 1)
        cases = {
            "WMA": lambda w: np.dot(w, weights) / weights.sum(),
            "Mad": lambda w: np.abs(w - w.mean()).mean(),
            "IdxMax": lambda w: np.argmax(w[::-1]) / n,
            "IdxMin": lambda w: np.argmin(w[::-1]) / n,
        }
        for name, fn in cases.items():
            with self.subTest(operator=name):
                self.check(f"{name}($close,{n})", self.manual_windows(fn, n))
        # In a constant window, today's occurrence is the most recent extreme.
        self.assertEqual(evaluate_expr("IdxMax($close,5)", self.panels).iloc[-1, 3], 0)
        self.assertEqual(evaluate_expr("IdxMin($close,5)", self.panels).iloc[-1, 3], 0)

    def test_corr_cov(self):
        y = self.panels["volume"]
        self.check("Corr($close,$volume,5)", self.x.rolling(5).corr(y))
        self.check("Cov($close,$volume,5)", self.x.rolling(5).cov(y))
        self.check("Cov($close,$close,5)", self.x.rolling(5).var())
        result = evaluate_expr("Corr($close,$close,5)", self.panels)
        self.assertTrue(result.iloc[:, 3].isna().all())

    def test_slope_rsquare_resi_against_polyfit(self):
        for n in (2, 5, 7):
            times = np.arange(n, dtype=float)

            def fit(window, name):
                slope, intercept = np.polyfit(times, window, 1)
                residuals = window - (slope * times + intercept)
                if name == "Slope":
                    return slope
                if name == "Resi":
                    return residuals[-1]
                if np.all(window == window[0]):
                    return np.nan
                return 1 - np.dot(residuals, residuals) / np.sum((window - window.mean()) ** 2)

            for name in ("Slope", "Rsquare", "Resi"):
                with self.subTest(n=n, operator=name):
                    self.check(f"{name}($close,{n})",
                               self.manual_windows(lambda w: fit(w, name), n))

    def test_constant_variance_regression_and_small_samples(self):
        for name in ("Std", "Var", "Slope", "Resi", "Mad"):
            with self.subTest(operator=name):
                result = evaluate_expr(f"{name}(0.3,5)", self.panels)
                self.assertTrue(result.iloc[:4].isna().all().all())
                np.testing.assert_allclose(result.iloc[4:], 0, atol=1e-15)
        self.check("Rsquare(0.3,5)", np.nan)
        self.check("Skew($close,2)", np.nan)
        self.check("Kurt($close,3)", np.nan)

    def test_cross_sectional_operators(self):
        self.check("CSRank($close)", self.x.rank(axis=1, method="average", pct=True))
        centered = self.x.sub(self.x.mean(axis=1), axis=0)
        self.check("CSDemean($close)", centered)
        self.check("CSZ($close)", centered.div(self.x.std(axis=1).replace(0, np.nan), axis=0))
        self.check("CSZ(3)", np.nan)
        self.check("CSDemean(3)", 0.)
        self.check("CSRank(3)", 2.5 / 4)
        panels = {name: frame.iloc[:, :1] for name, frame in self.panels.items()}
        self.assertTrue(evaluate_expr("CSZ($close)", panels).isna().all().all())

    def test_every_operator_is_causal_under_truncation(self):
        panels = make_panels(90, 5)
        panels["close"].iloc[3, 0] = np.nan
        expressions = {}
        for name, spec in OPERATORS.items():
            if name in {"Corr", "Cov"}:
                expr = f"{name}($close,$volume,5)"
            elif name == "Quantile":
                expr = "Quantile($close,5,0.8)"
            elif spec["window_arg"]:
                expr = f"{name}($close,5)"
            elif name == "Power":
                expr = "Power($close,-2)"
            elif name == "Clip":
                expr = "Clip($close,90,110)"
            elif name in {"Greater", "Less"}:
                expr = f"{name}($close,$open)"
            elif name == "If":
                expr = "If($close>$open,$close,$volume)"
            else:
                expr = f"{name}($close)"
            expressions[name] = expr
        self.assertEqual(set(expressions), set(OPERATORS))
        for op in ("+", "-", "*", "/", ">", "<", ">=", "<="):
            expressions[op] = f"$close{op}$open"
        expressions.update(ret="$ret", neg="-$close",
                           nested="Slope(Mean($close,5),5)")
        for name, expr in expressions.items():
            full = evaluate_expr(expr, panels)
            for t in (2, 30, 69):
                with self.subTest(operator=name, t=t):
                    prefix = {field: frame.iloc[:t + 1] for field, frame in panels.items()}
                    short = evaluate_expr(expr, prefix)
                    pd.testing.assert_series_equal(full.iloc[t], short.iloc[t],
                                                   check_exact=True)

    def test_metadata_and_budget_boundaries(self):
        self.assertEqual(validate_expr("Mean($close,5)"),
                         {"nodes": 7, "depth": 4, "max_window": 5, "fields": ["close"]})
        self.assertEqual(validate_expr("Corr($close,$volume,120)")["max_window"], 120)
        self.assertEqual(validate_expr("$open+$close")["fields"], ["close", "open"])
        for expr in ("0", "$close", "Mean($close,120)", "-($close+$open)"):
            meta = validate_expr(expr)
            tree = ast.parse(re.sub(r"\$([a-z]+)", r"F_\1", expr), mode="eval")
            self.assertEqual(meta["nodes"], sum(1 for _ in ast.walk(tree)))

    def test_reject_unsafe_syntax_and_nonliteral_parameters(self):
        expressions = [
            "__import__('os')", "$close.mean()", "$close[0]", "lambda x:x",
            "[x for x in $close]", "(x for x in $close)", "{x:x for x in $close}",
            "(x:=1)", "1 if $close>0 else 0", "$close and $open",
            "open(1)", "Foo($close)", "Mean.__call__($close,5)",
            "Mean($close,n=5)", "Mean(*$close,5)", "Mean(**$close)",
            "Mean($close,2+3)", "Ref($close,1+1)", "Mean($close,121)",
            "Mean($close,1)", "Ref($close,0)", "Ref($close,-1)",
            "Mean($close,5.0)", "Mean($close,True)", "Mean($close,$volume)",
            "Mean($close,+5)", "Delta($close,0)", "Corr($close,$volume,121)",
            "Quantile($close,5,0.1+0.2)", "Quantile($close,5,-0.1)",
            "Quantile($close,5,1.1)", "Power($close,1+1)", "Power($close,$open)",
            "Clip($close,-1,1+1)", "Clip($close,$open,2)", "Clip($close,2,1)",
            "If($close,1,0)", "If(1,2,3)", "Mean($close)", "Abs($close,2)",
            "$foo", "$Close", "close", "F_close", "np.nan", "'close'", "True",
            "None", "1j", "1e999", "[1,2]", "(1,2)", "{'a':1}",
            "$close**2", "$close//2", "$close%2", "$close==1", "$close!=1",
            "0<$close<1", "+$close", "not $close", "", " ", None, 12,
        ]
        for expr in expressions:
            with self.subTest(expr=expr):
                for operation in (validate_expr, canonical, expr_hash):
                    with self.assertRaises(ExprError):
                        operation(expr)
                with self.assertRaises(ExprError):
                    evaluate_expr(expr, self.panels)

    def test_reject_node_and_depth_overflows(self):
        leaves = ["$close"] * 16
        while len(leaves) > 1:
            leaves = [f"({a}+{b})" for a, b in zip(leaves[::2], leaves[1::2])]
        for expr in (leaves[0], "Abs(" * 8 + "$close" + ")" * 8):
            with self.subTest(expr=expr), self.assertRaises(ExprError):
                validate_expr(expr)

    def test_canonical_and_hash(self):
        for left, right in [("$close+$open", " $open + $close "),
                            ("$close*$open", "$open * $close"),
                            ("Mean($close,5)+$open", "$open+Mean( $close , 5 )"),
                            ("Clip($close,1.0,2e0)", "Clip($close,1,2)"),
                            ("Power($close,-2.0)", "Power($close,-2)")]:
            with self.subTest(expr=left):
                self.assertEqual(canonical(left), canonical(right))
                self.assertEqual(expr_hash(left), expr_hash(right))
                self.assertEqual(canonical(canonical(left)), canonical(left))
        self.assertNotEqual(canonical("$open-$close"), canonical("$close-$open"))
        self.assertNotEqual(canonical("$open/$close"), canonical("$close/$open"))
        expr = "$open + $close"
        expected = "f_" + hashlib.sha1(b"($close+$open)").hexdigest()[:8]
        self.assertEqual(expr_hash(expr), expected)
        self.assertRegex(expr_hash(expr), r"^f_[0-9a-f]{8}$")
        self.assertEqual(expr_hash(expr), expr_hash(expr))
        self.check(canonical("($close+2)*3-1"), (self.x + 2) * 3 - 1)

    def test_operator_metadata_and_chinese_help(self):
        help_text = dsl_help()
        for field in FIELDS:
            self.assertIn("$" + field, help_text)
        for name, spec in OPERATORS.items():
            self.assertEqual(set(spec), {"arity", "doc", "window_arg"})
            self.assertIs(type(spec["arity"]), int)
            self.assertIs(type(spec["window_arg"]), bool)
            self.assertTrue(spec["doc"])
            self.assertIn(name, help_text)
        self.assertIn("禁止", help_text)
        for line in help_text.splitlines():
            if line.startswith("例子："):
                validate_expr(line.removeprefix("例子："))

    def test_panel_validation_and_inputs_are_not_mutated(self):
        snapshot = {name: frame.copy(deep=True) for name, frame in self.panels.items()}
        evaluate_expr("WMA($ret,5)+Mean($close,5)", self.panels)
        for field in self.panels:
            pd.testing.assert_frame_equal(self.panels[field], snapshot[field])
        bad = [None, {}, {"close": self.x}]
        for name in ("open", "close"):
            mismatch = self.panels.copy()
            mismatch[name] = self.x.iloc[:-1]
            bad.append(mismatch)
        mismatch = self.panels.copy()
        mismatch["high"] = self.x.rename(columns={self.x.columns[0]: "wrong"})
        bad.append(mismatch)
        bad.append({name: frame.iloc[::-1] for name, frame in self.panels.items()})
        bad.append({name: frame.reset_index(drop=True) for name, frame in self.panels.items()})
        nonnumeric = self.panels.copy()
        nonnumeric["volume"] = self.x.astype(str)
        nonnumeric["volume"].iloc[0, 0] = "invalid"
        bad.append(nonnumeric)
        for panels in bad:
            with self.subTest(panels_type=type(panels).__name__), self.assertRaises(ExprError):
                evaluate_expr("$close", panels)

    def test_slope_performance_smoke(self):
        panels = make_panels(1500, 100)
        start = time.perf_counter()
        result = evaluate_expr("Slope($close,60)", panels)
        elapsed = time.perf_counter() - start
        self.assertLess(elapsed, 3., f"Slope(1500x100,60) took {elapsed:.3f}s")
        self.assertEqual(result.shape, (1500, 100))
        self.assertTrue(result.iloc[:59].isna().all().all())
        self.assertTrue(np.isfinite(result.iloc[59:].to_numpy()).all())


if __name__ == "__main__":
    unittest.main()
