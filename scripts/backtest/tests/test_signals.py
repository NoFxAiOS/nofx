import json
import sqlite3
import unittest
from datetime import datetime, timezone

from scripts.backtest import signals


def ts(h, m, s=0):
    return datetime(2026, 10, 1, h, m, s, tzinfo=timezone.utc)


def ms(dt):
    return int(dt.timestamp() * 1000)


def dec(action, sym, t, sl, tp, price=10.0, success=True):
    return {"action": action, "symbol": sym, "price": price, "stop_loss": sl, "take_profit": tp,
            "success": success, "timestamp": t.strftime("%Y-%m-%dT%H:%M:%S.000000Z")}


def prompt(sym, atr):
    sig = {"symbol": sym, "timeframes": {"1h": {"atr_pct": atr}}}
    return f"preamble\n=== {sym} Structured Signal (x) ===\n{json.dumps(sig)}\nfooter"


class SignalsTest(unittest.TestCase):
    def setUp(self):
        db = sqlite3.connect(":memory:")
        db.executescript("""
        create table traders(id text, name text, strategy_id text);
        create table strategies(id text, config text);
        create table decision_records(id integer primary key, trader_id text, timestamp text, decisions text,
                                      decision_json text, input_prompt text);
        create table trader_positions(id integer primary key, trader_id text, symbol text, side text,
                                      entry_price real, entry_time integer, ai_managed integer);
        """)
        db.execute("insert into traders values ('T1','bot','S1')")
        db.execute("insert into strategies values ('S1', ?)", (json.dumps({"risk_control": {"tp_max_r": 2}}),))
        self.db = db
        self.rid = 0

    def rec(self, t, decisions, mode="trend", atr=2.5, trader="T1", with_prompt=True):
        self.rid += 1
        dj = [{"symbol": d["symbol"], "action": d["action"], "exit_mode": mode} for d in decisions]
        if mode is None:
            dj = [{"symbol": d["symbol"], "action": d["action"]} for d in decisions]
        pr = prompt(decisions[0]["symbol"], atr) if with_prompt else "no signal block"
        self.db.execute("insert into decision_records values (?,?,?,?,?,?)",
                        (self.rid, trader, t.strftime("%Y-%m-%d %H:%M:%S.123456+00:00"), json.dumps(decisions),
                         json.dumps(dj), pr))

    def pos(self, sym, side, t, price, ai):
        self.db.execute("insert into trader_positions(trader_id,symbol,side,entry_price,entry_time,ai_managed) "
                        "values ('T1',?,?,?,?,?)", (sym, side, price, ms(t), ai))

    def run_extract(self, **kw):
        sigs, n = signals.extract(self.db, "T1", **kw)
        return sigs, n

    def test_resolve_trader_and_risk_control(self):
        self.assertEqual(signals.resolve_trader(self.db, name="bot")["id"], "T1")
        self.assertEqual(signals.resolve_trader(self.db, trader_id="T1")["strategy_id"], "S1")
        self.assertEqual(signals.load_risk_control(self.db, "S1"), {"tp_max_r": 2})
        with self.assertRaises(ValueError):
            signals.resolve_trader(self.db, name="nope")

    def test_dedupe_chain_fill_and_market(self):
        L = "open_long_limit"
        # 链 1:10:00 / 10:20 同币同向,间隔 < 60 分钟 → 合并成一个信号;之后无成交 → unfilled
        self.rec(ts(10, 0), [dec(L, "AAAUSDT", ts(10, 0), 9.0, 12.0)], mode="range", atr=3.1)
        self.rec(ts(10, 20), [dec(L, "AAAUSDT", ts(10, 20), 9.1, 12.1)], mode="trend", atr=2.0)
        # 链 2:间隔 70 分钟 → 新信号;11:50 成交(ai 与手动两个候选,ai 优先)
        self.rec(ts(11, 30), [dec(L, "AAAUSDT", ts(11, 30), 9.2, 12.2)])
        self.rec(ts(11, 40), [dec(L, "AAAUSDT", ts(11, 40), 9.3, 12.3)])
        self.pos("AAAUSDT", "LONG", ts(11, 50), 10.0, 1)
        self.pos("AAAUSDT", "LONG", ts(11, 45), 10.02, 0)
        # 链 3:12:00 紧接着成交之后(间隔仅 20 分钟,但期间有成交)→ 独立信号
        self.rec(ts(12, 0), [dec(L, "AAAUSDT", ts(12, 0), 9.4, 12.4)])
        # 市价开仓独立成信号;失败的决策忽略
        self.rec(ts(10, 5), [dec("open_short", "BBBUSDT", ts(10, 5), 11.0, 8.0, price=0),
                             dec(L, "CCCUSDT", ts(10, 5), 9.0, 12.0, success=False)])
        # 反向限价打断同币链
        self.rec(ts(10, 0, 30), [dec(L, "DDDUSDT", ts(10, 0, 30), 9.0, 12.0)])
        self.rec(ts(10, 10), [dec("open_short_limit", "DDDUSDT", ts(10, 10), 11.0, 8.0)])
        # 别的 trader 不算
        self.rec(ts(10, 0), [dec(L, "ZZZUSDT", ts(10, 0), 9.0, 12.0)], trader="T2")
        sigs, n = self.run_extract()
        by = {}
        for s in sigs:
            by.setdefault(s["symbol"], []).append(s)
        self.assertNotIn("ZZZUSDT", by)
        self.assertNotIn("CCCUSDT", by)

        a = sorted(by["AAAUSDT"], key=lambda s: s["t0_ms"])
        self.assertEqual(len(a), 3)
        s1, s2, s3 = a
        # 第一个链:取第一条的 t0/sl0/tp0/exit_mode/atr,最后一条的 sl/tp
        self.assertEqual((s1["n"], s1["outcome"]), (2, "unfilled"))
        self.assertEqual((s1["sl0"], s1["tp0"], s1["exit_mode"], s1["atr1h"]), (9.0, 12.0, "range", 3.1))
        self.assertEqual((s1["sl_last"], s1["tp_last"], s1["sl"], s1["tp"]), (9.1, 12.1, 9.1, 12.1))
        self.assertEqual((s1["exit_mode_last"], s1["atr1h_last"]), ("trend", 2.0))
        self.assertEqual(s1["t0_ms"], ms(ts(10, 0)))
        # 第二个链:成交,匹配 ai_managed=1 的持仓
        self.assertEqual((s2["n"], s2["outcome"], s2["fill_px"]), (2, "filled", 10.0))
        self.assertEqual((s2["sl"], s2["tp"]), (9.3, 12.3))
        self.assertEqual(s2["fill_t"], ts(11, 50).isoformat())
        # 第三个链:成交后新信号;已被占用的持仓不能重复使用
        self.assertEqual((s3["n"], s3["outcome"]), (1, "unfilled"))

        self.assertEqual([(s["kind"], s["outcome"], s["side"]) for s in by["BBBUSDT"]], [("market", "market", "short")])
        self.assertEqual(len(by["DDDUSDT"]), 2)
        self.assertEqual(n, 8)  # 8 条成功的原始开仓决策(失败的和别的 trader 不算)

    def test_dedupe_min_parameter(self):
        L = "open_long_limit"
        self.rec(ts(10, 0), [dec(L, "AAAUSDT", ts(10, 0), 9.0, 12.0)])
        self.rec(ts(10, 20), [dec(L, "AAAUSDT", ts(10, 20), 9.0, 12.0)])
        self.assertEqual(len(self.run_extract(dedupe_min=60)[0]), 1)
        self.assertEqual(len(self.run_extract(dedupe_min=10)[0]), 2)

    def test_fill_price_guard_excludes_manual_trade(self):
        L = "open_long_limit"
        self.rec(ts(10, 0), [dec(L, "AAAUSDT", ts(10, 0), 9.0, 12.0, price=10.0)])
        self.pos("AAAUSDT", "LONG", ts(10, 5), 11.0, 0)  # 离挂单价 10%,是手动单
        self.assertEqual(self.run_extract()[0][0]["outcome"], "unfilled")
        self.assertEqual(self.run_extract(max_fill_dev_pct=0)[0][0]["outcome"], "filled")

    def test_fill_must_be_inside_window(self):
        L = "open_long_limit"
        self.rec(ts(10, 0), [dec(L, "AAAUSDT", ts(10, 0), 9.0, 12.0)])
        self.pos("AAAUSDT", "LONG", ts(10, 50), 10.0, 1)  # 最后决策 + 45 分钟之后
        self.assertEqual(self.run_extract()[0][0]["outcome"], "unfilled")

    def test_since_until_and_missing_meta(self):
        L = "open_long_limit"
        self.rec(ts(9, 0), [dec(L, "AAAUSDT", ts(9, 0), 9.0, 12.0)], mode=None, with_prompt=False)
        self.rec(ts(10, 0), [dec(L, "BBBUSDT", ts(10, 0), 9.0, 12.0)], mode=None, with_prompt=False)
        self.rec(ts(11, 0), [dec(L, "CCCUSDT", ts(11, 0), 9.0, 12.0)])
        sigs, _ = self.run_extract(since=signals.parse_utc("2026-10-01T09:30"), until=signals.parse_utc("2026-10-01T11:00"))
        self.assertEqual([s["symbol"] for s in sigs], ["BBBUSDT"])
        # 缺 exit_mode → trend;缺结构化信号 → atr None
        self.assertEqual((sigs[0]["exit_mode"], sigs[0]["atr1h"]), ("trend", None))

    def test_parse_utc(self):
        self.assertEqual(signals.parse_utc("2026-09-12"), datetime(2026, 9, 12, tzinfo=timezone.utc))
        self.assertEqual(signals.parse_utc("2026-10-12T16:00"), datetime(2026, 10, 12, 16, tzinfo=timezone.utc))


if __name__ == "__main__":
    unittest.main()
