import json
import os
import tempfile
import unittest

from scripts.backtest.entry import build_rows

MIN = 60_000
HOUR = 3_600_000
BASE = 1_700_000_000_000 // MIN * MIN


def sig(sym, side, t0, sl0=95.0, tp0=105.0):
    return dict(symbol=sym, side=side, kind="limit", outcome="unfilled", t0=str(t0), t0_ms=t0, sl0=sl0, tp0=tp0, px0=99.0 if side == "long" else 101.0)


class EntryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        os.makedirs(os.path.join(self.tmp.name, "kl1m"))

    def put(self, s):
        # 价格恒定 100,从 t0 起 1500 根 1m:SL/TP 永远碰不到,括号一直持有到 horizon 末
        bars = [[s["t0_ms"] + i * MIN, 100.0, 100.0, 100.0, 100.0] for i in range(1500)]
        with open(os.path.join(self.tmp.name, "kl1m", f"{s['symbol']}_{s['t0_ms']}.json"), "w") as f:
            json.dump(bars, f)

    def rows(self, sigs, **kw):
        for s in sigs:
            self.put(s)
        res, _ = build_rows(sigs, self.tmp.name, [8], **kw)
        return res[8]

    def test_market_rows_blocked_while_position_open(self):
        a = sig("X", "long", BASE)
        b = sig("X", "long", BASE + 10 * MIN)             # a 的括号要到 8h 末才出场 → 被挡
        c = sig("X", "short", BASE + 10 * MIN, sl0=105.0, tp0=95.0)            # 不同方向
        d = sig("Y", "long", BASE + 10 * MIN)             # 不同币
        e = sig("X", "long", BASE + 8 * HOUR + 5 * MIN)   # a 已出场 → 通过
        rows = {(r["sym"], r["side"], r["t0"]): r for r in self.rows([a, b, c, d, e])}
        self.assertEqual(rows[("X", "long", str(b["t0_ms"]))]["mkt_how"], "blocked")
        self.assertIsNone(rows[("X", "long", str(b["t0_ms"]))]["mkt_r"])
        for s in (a, c, d, e):
            self.assertIsNotNone(rows[(s["symbol"], s["side"], s["t0"])]["mkt_r"])

    def test_allow_overlap_disables_blocking(self):
        a = sig("X", "long", BASE)
        b = sig("X", "long", BASE + 10 * MIN)
        rows = self.rows([a, b], allow_overlap=True)
        self.assertTrue(all(r["mkt_r"] is not None for r in rows))
        self.assertTrue(all(r["lim_how"] == "unfilled" for r in rows))


if __name__ == "__main__":
    unittest.main()
