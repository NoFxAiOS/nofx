import json
import sqlite3
import unittest

from scripts.backtest.mldataset import coverage, find_record, plain_params

T0 = "2026-10-01T10:00:00+00:00"


def prompt(sym, price):
    return f"=== {sym} Structured Signal (x) ===\n{json.dumps({'symbol': sym, 'price': price})}\n"


class FindRecordTest(unittest.TestCase):
    def setUp(self):
        db = sqlite3.connect(":memory:")
        db.execute("create table decision_records(id integer primary key, trader_id text, timestamp text, "
                   "input_prompt text, decision_json text, decisions text)")
        self.db = db

    def add(self, ts, sym, action, price, trader="T1", conf=70, prompt_text=None):
        dec = [{"action": action, "symbol": sym, "success": True}]
        dj = [{"symbol": sym, "action": action, "confidence": conf, "exit_mode": "range"}]
        self.db.execute("insert into decision_records(trader_id,timestamp,input_prompt,decision_json,decisions) "
                        "values (?,?,?,?,?)",
                        (trader, ts, prompt_text if prompt_text is not None else prompt(sym, price),
                         json.dumps(dj), json.dumps(dec, separators=(",", ":"))))  # 线上是紧凑 JSON

    sig = dict(symbol="AAAUSDT", side="long", kind="limit", t0=T0)

    def test_picks_nearest_matching_record(self):
        self.add("2026-10-01 10:04:00.1+00:00", "AAAUSDT", "open_long_limit", 2.0)   # 偏 4 分钟
        self.add("2026-10-01 10:00:20.1+00:00", "AAAUSDT", "open_long_limit", 1.0)   # 最近
        self.add("2026-10-01 10:00:10.1+00:00", "BBBUSDT", "open_long_limit", 3.0)   # 币不同
        self.add("2026-10-01 10:00:05.1+00:00", "AAAUSDT", "open_short_limit", 4.0)  # 动作不同
        self.add("2026-10-01 10:00:05.1+00:00", "AAAUSDT", "open_long_limit", 5.0, trader="T2")  # trader 不同
        s, ai = find_record(self.db, "T1", self.sig)
        self.assertEqual(s["price"], 1.0)
        self.assertEqual((ai["confidence"], ai["exit_mode"]), (70, "range"))

    def test_market_signal_uses_market_action(self):
        self.add("2026-10-01 10:00:20+00:00", "AAAUSDT", "open_long_limit", 1.0)
        self.add("2026-10-01 10:01:00+00:00", "AAAUSDT", "open_long", 9.0)
        s, _ = find_record(self.db, "T1", dict(self.sig, kind="market"))
        self.assertEqual(s["price"], 9.0)

    def test_outside_window_or_no_structured_signal(self):
        self.add("2026-10-01 10:11:00+00:00", "AAAUSDT", "open_long_limit", 1.0)  # 超过 ±10 分钟
        self.assertIsNone(find_record(self.db, "T1", self.sig))
        self.add("2026-10-01 10:00:20+00:00", "AAAUSDT", "open_long_limit", 1.0, prompt_text="no block")
        self.assertIsNone(find_record(self.db, "T1", self.sig))


class MiscTest(unittest.TestCase):
    def test_plain_params_turns_everything_off(self):
        base = dict(be_arm=0.75, lock_r=1.5, lock_trim=True, trim_r=1.2, trail=True, pd_arm=2, tp_cap=2, horizon_h=72)
        p = plain_params(base)
        self.assertEqual((p["be_arm"], p["lock_r"], p["lock_trim"], p["trim_r"], p["trail"], p["pd_arm"], p["tp_cap"]),
                         (0, 0, False, 0, False, 0, 0))
        self.assertEqual(p["horizon_h"], 72)
        self.assertEqual(base["be_arm"], 0.75)  # 不改原 dict

    def test_coverage(self):
        rows = [{"feat": {"a": 1, "b": None}}, {"feat": {"a": 2, "b": 3}}, {"feat": {"a": None, "b": None}}]
        cov = coverage(rows)
        self.assertAlmostEqual(cov["a"], 2 / 3)
        self.assertAlmostEqual(cov["b"], 1 / 3)


if __name__ == "__main__":
    unittest.main()
