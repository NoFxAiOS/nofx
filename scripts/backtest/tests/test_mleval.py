import contextlib
import io
import random
import unittest

try:
    import numpy  # noqa: F401
    import sklearn  # noqa: F401
    from scripts.backtest import mleval
except ImportError:
    mleval = None

MODELS = ["logit_L2", "ridge"]


def make_rows(n, signal, seed=1, n_noise=10):
    """feat f0 在 signal=True 时强预测两个标签;其余特征纯噪声。"""
    rnd = random.Random(seed)
    rows = []
    for i in range(n):
        feat = {f"f{j}": rnd.gauss(0, 1) for j in range(n_noise + 1)}
        base = 2.0 * feat["f0"] if signal else 0.0
        rows.append(dict(t0_ms=i * 60_000, feat=feat, r_base=base + rnd.gauss(0, 1), r_plain=base + rnd.gauss(0, 1)))
    return rows


def run(rows, **kw):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        results, line = mleval.run(rows, models=MODELS, seed=3, **kw)
    return results, line, buf.getvalue()


@unittest.skipUnless(mleval is not None, "需要 numpy + scikit-learn")
class MlEvalTest(unittest.TestCase):
    def test_strong_feature_passes(self):
        results, line, text = run(make_rows(300, True), perms=400)
        self.assertTrue(line.startswith("PASS: candidate filter"), line)
        for lab in mleval.LABELS:
            r = results[lab]
            self.assertTrue(all("f0" in c for c in r["nested"]["chosen"]), r["nested"]["chosen"])
            self.assertLess(r["nested"]["p"], 0.005)
            self.assertLess(min(r["models"]["ridge"]["p70"], r["models"]["ridge"]["p50"]), 0.01)
        self.assertIn("检验数=10", text)  # 2 个 label × (2 模型 × 2 阈值 + 1 嵌套)

    def test_pure_noise_has_no_deployable_filter(self):
        _, line, _ = run(make_rows(300, False), perms=200)
        self.assertEqual(line, "NO DEPLOYABLE FILTER")

    def test_single_label_cannot_pass(self):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            _, line = mleval.run(make_rows(300, True), ("r_base",), models=MODELS, seed=3, perms=400)
        self.assertTrue(line.startswith("NO DEPLOYABLE FILTER"), line)

    def test_too_few_perms_warns_and_cannot_pass(self):
        _, line, text = run(make_rows(300, True), perms=50)
        self.assertIn("--perms >=", text)
        self.assertEqual(line, "NO DEPLOYABLE FILTER")

    def test_coverage_filter_drops_sparse_features(self):
        rows = make_rows(120, True)
        for r in rows[:60]:
            r["feat"]["f1"] = None  # 覆盖率 50%
        d = mleval.Data(rows, "r_base", 0.8)
        self.assertNotIn("f1", d.keys)
        self.assertIn("f0", d.keys)


if __name__ == "__main__":
    unittest.main()
