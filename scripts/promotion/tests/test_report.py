"""Deterministic offline calculations and real SQLite I/O boundaries."""

from contextlib import closing, redirect_stderr, redirect_stdout
from datetime import datetime, timedelta
import io
import json
import math
import os
from pathlib import Path
import sqlite3
import tempfile
import threading
import unittest
from unittest.mock import patch

from scripts.promotion import report as promotion


START = datetime(2026, 10, 1, tzinfo=promotion.UTC)
NOW = START + timedelta(days=14, hours=12)
SCHEMA = """
CREATE TABLE trader_positions (
    id INTEGER PRIMARY KEY, trader_id TEXT NOT NULL, symbol TEXT NOT NULL,
    status TEXT DEFAULT 'OPEN', entry_time INTEGER NOT NULL, exit_time INTEGER,
    realized_pnl REAL DEFAULT 0, fee REAL DEFAULT 0,
    initial_stop_loss REAL DEFAULT 0, entry_price REAL NOT NULL,
    quantity REAL NOT NULL
);
CREATE TABLE trader_equity_snapshots (
    id INTEGER PRIMARY KEY, trader_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL, total_equity REAL NOT NULL DEFAULT 0
);
CREATE INDEX idx_equity_trader_time ON trader_equity_snapshots(trader_id, timestamp);
CREATE INDEX idx_positions_trader ON trader_positions(trader_id);
"""


def pair(**overrides):
    return {"id": "ensemble", "title": "因子 ensemble", "feature": "enable_factor_ensemble",
            "candidate_trader_id": "candidate", "control_trader_id": "control",
            "live_trader_id": "", "started_at": promotion.iso(START),
            "min_days": 7, "min_trades": 20, "notes": "", **overrides}


def trade(position_id=1, **overrides):
    return {"id": position_id, "symbol": "BTCUSDT", "exit_time": int((START + timedelta(days=1)).timestamp() * 1000),
            "realized_pnl": 3.0, "fee": 1.0, "entry_price": 100.0,
            "initial_stop_loss": 99.0, "quantity": 2.0, **overrides}


def snapshots(values, start=START):
    return [{"id": i + 1, "timestamp": promotion.iso(start + timedelta(days=i, hours=12)),
             "total_equity": equity} for i, equity in enumerate(values)]


class CalculationTests(unittest.TestCase):
    def test_net_pnl_win_rate_r_exclusions_and_extremes(self):
        rows = [trade(1), trade(2, realized_pnl=0.5),
                trade(3, initial_stop_loss=0), trade(4, initial_stop_loss=None),
                trade(5, initial_stop_loss=100), trade(6, initial_stop_loss=100 + 1e-10),
                trade(7, quantity=0), trade(8, realized_pnl=69),
                trade(9, realized_pnl=-31), trade(10, realized_pnl=31)]
        stats, samples = promotion.trade_statistics(rows)
        self.assertEqual(stats["n"], 10)
        self.assertAlmostEqual(stats["net_pnl_usdt"], sum(row["realized_pnl"] - row["fee"] for row in rows))
        self.assertEqual(stats["fees_usdt"], 10)
        self.assertEqual(stats["win_rate"], 0.8)  # A gross win below its fee is a net loss.
        self.assertEqual(stats["r_n"], 5)
        self.assertEqual(stats["r_excluded"], 5)
        self.assertEqual(stats["r_excluded_reasons"], {
            "missing_or_invalid_price_stop": 2, "near_zero_stop_distance": 2,
            "missing_or_invalid_quantity": 1})
        self.assertEqual([r for _, r in samples], [1, -0.25, 34, -16, 15])
        self.assertAlmostEqual(stats["avg_r"], 6.75)  # Extreme R stays in avgR.
        self.assertEqual(stats["r_extreme_count"], 2)
        self.assertEqual([row["position_id"] for row in stats["r_extremes"]], [8, 9])
        self.assertIn("realized_pnl - fee", stats["pnl_basis"])
        self.assertTrue(all(row["exit_time"].endswith("Z") for row in stats["r_extremes"]))

    def test_nonfinite_trade_data_is_not_a_zero_profit_or_infinite_r(self):
        stats, samples = promotion.trade_statistics([
            trade(1, realized_pnl=float("inf")), trade(2, fee=float("nan")),
            trade(3, initial_stop_loss=float("inf")), trade(4, quantity=float("nan"))])
        self.assertIsNone(stats["net_pnl_usdt"])
        self.assertIsNone(stats["win_rate"])
        self.assertEqual(stats["pnl_invalid"], 2)
        self.assertEqual(stats["r_excluded"], 4)
        self.assertEqual(samples, [])

    def test_daily_close_forward_fill_and_max_drawdown(self):
        rows = [
            {"id": 1, "timestamp": "2026-10-01T09:00:00Z", "total_equity": 100},
            {"id": 2, "timestamp": "2026-10-01T23:00:00Z", "total_equity": 120},
            {"id": 3, "timestamp": "2026-10-03 10:00:00+00:00", "total_equity": 90},
            {"id": 4, "timestamp": "2026-10-04T06:00:00+08:00", "total_equity": 80},
            {"id": 5, "timestamp": "2026-10-04T12:00:00Z", "total_equity": 110},
            {"id": 6, "timestamp": "2026-09-30T12:00:00Z", "total_equity": 500},
            {"id": 7, "timestamp": "2026-10-16T12:00:00Z", "total_equity": 900},
        ]
        stats = promotion.equity_statistics(list(reversed(rows)), START, NOW)
        curve = stats["daily_equity"]
        self.assertEqual([row["equity"] for row in curve], [120, 120, 80, 110])
        self.assertEqual([row["return"] for row in curve[:2]], [None, 0])
        self.assertAlmostEqual(curve[2]["return"], -1 / 3)
        self.assertEqual(stats["forward_filled_days"], 1)
        self.assertTrue(curve[1]["forward_filled"])
        self.assertAlmostEqual(stats["max_drawdown"], 1 / 3)
        self.assertAlmostEqual(stats["period_return"], 0.1)  # First actual snapshot was 100.
        self.assertEqual(stats["snapshot_count"], 5)
        self.assertEqual(curve[-1]["date"], "2026-10-04T00:00:00Z")  # No extension to NOW.
        self.assertEqual(curve[2]["snapshot_at"], "2026-10-03T22:00:00Z")

    def test_equity_empty_invalid_zero_baseline_and_tie_order(self):
        stats = promotion.equity_statistics([], START, NOW)
        self.assertEqual(stats["snapshot_count"], 0)
        for key in ("period_return", "max_drawdown", "first_equity_usdt", "last_snapshot_at"):
            self.assertIsNone(stats[key])
        stats = promotion.equity_statistics([
            {"id": 1, "timestamp": "bad", "total_equity": 1},
            *snapshots([float("inf"), 0, 100]),
            {"id": 99, "timestamp": snapshots([0, 0, 0])[2]["timestamp"], "total_equity": 90},
        ], START, NOW)
        self.assertEqual(stats["snapshot_invalid"], 2)
        self.assertIsNone(stats["period_return"])
        self.assertEqual(stats["last_equity_usdt"], 90)
        self.assertIsNone(stats["daily_equity"][1]["return"])

    def test_daily_paired_difference_cluster_se_and_normal_ci(self):
        # Candidate returns increase from 1% to 10%; control is flat.
        equities = [100.0]
        for i in range(1, 11):
            equities.append(equities[-1] * (1 + i / 100))
        cand = promotion.equity_statistics(snapshots(equities), START, NOW)
        ctrl = promotion.equity_statistics(snapshots([100] * 11), START, NOW)
        result = promotion.paired_daily_comparison(cand, ctrl, NOW)
        # Hand calculation: sample variance of .01,...,.10 is 0.0009166667.
        expected_se = math.sqrt(0.0009166666666666666 / 10)
        self.assertEqual(result["n_days"], 10)
        self.assertAlmostEqual(result["mean_diff"], 0.055)
        self.assertAlmostEqual(result["cluster_se"], expected_se)
        self.assertAlmostEqual(result["ci95"][0], 0.055 - 1.96 * expected_se)
        self.assertAlmostEqual(result["ci95"][1], 0.055 + 1.96 * expected_se)
        self.assertAlmostEqual(result["cumulative_return_diff"], equities[-1] / 100 - 1)
        self.assertAlmostEqual(result["aligned_cumulative_return_diff"], equities[-1] / 100 - 1)

    def test_daily_small_sample_alignment_and_unfinished_day(self):
        cand = promotion.equity_statistics(snapshots([100, 110, 121, 133.1]), START, NOW)
        ctrl = promotion.equity_statistics(snapshots([100, 100, 100], START + timedelta(days=1)), START, NOW)
        result = promotion.paired_daily_comparison(cand, ctrl, START + timedelta(days=3, hours=23))
        self.assertEqual(result["n_days"], 1)  # Oct 3 only; Oct 4 is incomplete, Oct 2 has no ctrl return.
        self.assertAlmostEqual(result["mean_diff"], 0.1)
        self.assertIsNone(result["cluster_se"])
        self.assertIsNone(result["ci95"])
        empty = promotion.paired_daily_comparison(cand, promotion.equity_statistics([], START, NOW), NOW)
        self.assertEqual(empty["n_days"], 0)
        self.assertIsNone(empty["mean_diff"])
        self.assertIsNone(empty["cumulative_return_diff"])

    def test_weekly_trade_weighted_difference_joint_cluster_variance(self):
        monday = datetime(2026, 10, 5, tzinfo=promotion.UTC)
        # Unequal weekly trade counts; the final week has no control trades.
        candidate = [(monday, 1), (monday, 3), (monday + timedelta(days=7), 4),
                     (monday + timedelta(days=14), 8)]
        control = [(monday, 0), (monday + timedelta(days=7), 2),
                   (monday + timedelta(days=7), 1)]
        result = promotion.weekly_r_comparison(candidate, control, min_trades=3)
        # avgC=4, avgB=1; week influences [-2/3, -1/3, 1].
        expected = math.sqrt(1.5 * (4 / 9 + 1 / 9 + 1))
        self.assertEqual(result["n_weeks"], 3)
        self.assertEqual(result["mean_diff"], 3)
        self.assertAlmostEqual(result["cluster_se"], expected)
        self.assertAlmostEqual(result["ci95"][0], 3 - 1.96 * expected)
        self.assertEqual(result["weeks"][2]["control_n"], 0)
        self.assertEqual(result["weeks"][0]["week_start"], "2026-10-05T00:00:00Z")

    def test_weekly_small_sample_no_ci_and_r_sample_minimum(self):
        sample = [(START, 1), (START + timedelta(days=7), 2)]
        small = promotion.weekly_r_comparison(sample, [(day, 0) for day, _ in sample], 2)
        self.assertEqual(small["mean_diff"], 1.5)
        self.assertEqual(small["n_weeks"], 2)
        self.assertIsNone(small["cluster_se"])
        self.assertIsNone(small["ci95"])
        insufficient = promotion.weekly_r_comparison(sample, sample, 20)
        self.assertIsNone(insufficient["mean_diff"])
        self.assertIn("有效 R", insufficient["note"])

    def test_verdict_boundaries_all_unmet_reasons_and_missing_metrics(self):
        cand = {"n": 20, "net_pnl_usdt": 1, "period_return": 0.1}
        ctrl = {"n": 20, "period_return": 0.1}
        for days, c, b, diff, expected in (
            (7, cand, ctrl, 0, "ready"),
            (6.999, cand, ctrl, 0, "insufficient"),
            (7, {**cand, "n": 19}, ctrl, 0, "insufficient"),
            (7, cand, {**ctrl, "n": 19}, 0, "insufficient"),
            (7, {**cand, "net_pnl_usdt": 0}, ctrl, 0, "hold"),
            (7, {**cand, "net_pnl_usdt": -1}, ctrl, 0, "hold"),
            (7, cand, {**ctrl, "period_return": 0.2}, 0, "hold"),
            (7, cand, ctrl, -0.001, "hold"),
            (7, cand, ctrl, None, "hold"),
            (7, {**cand, "period_return": None}, ctrl, 0, "hold"),
            (7, {**cand, "net_pnl_usdt": float("nan")}, ctrl, 0, "hold"),
        ):
            with self.subTest(expected=expected, days=days, c=c, b=b, diff=diff):
                verdict, reasons = promotion.decide_verdict(days, 7, 20, c, b, {"mean_diff": diff})
                self.assertEqual(verdict, expected)
                self.assertEqual(bool(reasons), expected != "ready")
        verdict, reasons = promotion.decide_verdict(6, 7, 20,
            {"n": 0, "net_pnl_usdt": -1, "period_return": None},
            {"n": 0, "period_return": None}, {"mean_diff": None})
        self.assertEqual(verdict, "insufficient")
        self.assertEqual(len(reasons), 6)


class SQLiteReportTests(unittest.TestCase):
    def setUp(self):
        # Keep all test artifacts inside the task's permitted directory.
        self.temporary = tempfile.TemporaryDirectory(prefix=".test-", dir=Path(__file__).parent)
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.db = self.directory / "paper #1?.db"  # URI metacharacters must be escaped.
        with sqlite3.connect(self.db) as connection:
            connection.executescript(SCHEMA)

    def populate(self, trader_id, n=20, pnl=3, daily_gain=1):
        with sqlite3.connect(self.db) as connection:
            for i in range(n):
                end = START + timedelta(days=1, minutes=i)
                connection.execute(
                    "INSERT INTO trader_positions (trader_id,symbol,status,entry_time,exit_time,"
                    "realized_pnl,fee,initial_stop_loss,entry_price,quantity) VALUES (?,?,?,?,?,?,?,?,?,?)",
                    (trader_id, "BTCUSDT", "CLOSED", int(START.timestamp() * 1000),
                     int(end.timestamp() * 1000), pnl, 1, 99, 100, 2))
            for row in snapshots([100 + daily_gain * i for i in range(14)]):
                connection.execute("INSERT INTO trader_equity_snapshots (trader_id,timestamp,total_equity) VALUES (?,?,?)",
                                   (trader_id, row["timestamp"], row["total_equity"]))

    def test_end_to_end_three_verdicts_and_optional_live(self):
        self.populate("candidate", daily_gain=2)
        self.populate("control", daily_gain=1)
        self.populate("losing", pnl=0, daily_gain=-1)
        self.populate("few", n=3)
        self.populate("live", n=1, pnl=11, daily_gain=3)
        result = promotion.build_report([
            pair(id="ready", live_trader_id="live"),
            pair(id="hold", candidate_trader_id="losing"),
            pair(id="insufficient", candidate_trader_id="few"),
        ], self.db, NOW)
        self.assertEqual([item["verdict"] for item in result["pairs"]], ["ready", "hold", "insufficient"])
        ready, hold, insufficient = result["pairs"]
        self.assertEqual(ready["candidate"]["net_pnl_usdt"], 40)
        self.assertEqual(ready["live"]["net_pnl_usdt"], 10)
        self.assertEqual(ready["live"]["n"], 1)
        self.assertEqual(ready["paired_daily"]["n_days"], 13)
        self.assertEqual(ready["elapsed_days"], 14.5)
        self.assertEqual(ready["reasons"], [])
        self.assertGreaterEqual(len(hold["reasons"]), 3)
        self.assertTrue(any("3 笔" in reason for reason in insufficient["reasons"]))
        self.assertNotIn("live", hold)
        self.assertTrue(all(item["caveats"] == promotion.CAVEAT for item in result["pairs"]))

    def test_exit_time_window_closed_status_open_count_and_no_history(self):
        with sqlite3.connect(self.db) as connection:
            for status, entry, exit_ms in (
                ("CLOSED", START - timedelta(days=2), int(START.timestamp() * 1000)),
                ("CLOSED", START, int(START.timestamp() * 1000) - 1),
                ("CLOSED", START, int(NOW.timestamp() * 1000) + 1),
                ("OPEN", START - timedelta(days=2), None),
                ("OPEN", NOW + timedelta(days=1), None),
            ):
                connection.execute(
                    "INSERT INTO trader_positions (trader_id,symbol,status,entry_time,exit_time,"
                    "realized_pnl,fee,initial_stop_loss,entry_price,quantity) VALUES (?,?,?,?,?,?,?,?,?,?)",
                    ("candidate", "X", status, int(entry.timestamp() * 1000), exit_ms, 3, 1, 99, 100, 2))
        item = promotion.build_report([pair()], self.db, NOW)["pairs"][0]
        self.assertEqual(item["candidate"]["n"], 1)
        self.assertEqual(item["candidate"]["net_pnl_usdt"], 2)
        self.assertEqual(item["candidate"]["open_positions"], 1)
        self.assertEqual(item["control"]["n"], 0)
        self.assertEqual(item["control"]["net_pnl_usdt"], 0)
        self.assertIsNone(item["control"]["avg_r"])
        self.assertIsNone(item["control"]["max_drawdown"])
        self.assertIsNone(item["control"]["period_return"])
        self.assertIsNone(item["paired_daily"]["mean_diff"])
        self.assertEqual(item["verdict"], "insufficient")

    def test_snapshot_bounds_different_timestamp_encodings_are_exact_utc(self):
        start = START + timedelta(hours=12)
        with sqlite3.connect(self.db) as connection:
            for timestamp, equity in (
                ("2026-10-01 11:59:59.999999+00:00", 1000),
                ("2026-10-01T12:00:00Z", 100),
                ("2026-10-02 00:00:00+00:00", 110),
                ("2026-10-15T12:00:00.000001Z", 2000),
            ):
                connection.execute("INSERT INTO trader_equity_snapshots (trader_id,timestamp,total_equity) VALUES ('candidate',?,?)",
                                   (timestamp, equity))
        item = promotion.build_report([pair(started_at=promotion.iso(start))], self.db, NOW)["pairs"][0]
        self.assertEqual(item["candidate"]["snapshot_count"], 2)
        self.assertEqual(item["candidate"]["first_equity_usdt"], 100)
        self.assertEqual(item["candidate"]["last_equity_usdt"], 110)

    def test_mode_ro_rejects_writes_missing_db_not_created_and_db_unchanged(self):
        self.populate("candidate")
        before = self.db.read_bytes()
        with closing(promotion.open_readonly(self.db)) as connection:
            self.assertEqual(connection.execute("PRAGMA query_only").fetchone()[0], 1)
            connection.execute("PRAGMA query_only = OFF")  # mode=ro itself must prohibit writes.
            with self.assertRaises(sqlite3.OperationalError):
                connection.execute("CREATE TABLE forbidden (id INTEGER)")
        promotion.build_report([pair()], self.db, NOW)
        self.assertEqual(self.db.read_bytes(), before)
        missing = self.directory / "absent.db"
        with self.assertRaises(sqlite3.OperationalError):
            promotion.open_readonly(missing)
        self.assertFalse(missing.exists())

    def test_readonly_database_in_readonly_directory(self):
        self.populate("candidate")
        self.db.chmod(0o444)
        self.directory.chmod(0o555)
        try:
            result = promotion.build_report([pair()], self.db, NOW)
            self.assertEqual(result["pairs"][0]["candidate"]["n"], 20)
        finally:
            self.directory.chmod(0o755)
            self.db.chmod(0o644)

    def test_concurrent_wal_writer_uncommitted_changes_invisible(self):
        self.populate("candidate")
        self.populate("control")
        with sqlite3.connect(self.db) as writer:
            writer.execute("PRAGMA journal_mode=WAL")
            writer.execute("BEGIN IMMEDIATE")
            writer.execute("UPDATE trader_positions SET realized_pnl=999 WHERE trader_id='candidate'")
            result = promotion.build_report([pair()], self.db, NOW)
            self.assertEqual(result["pairs"][0]["candidate"]["net_pnl_usdt"], 40)
            writer.rollback()

    def test_one_consistent_read_transaction_even_if_writer_commits_mid_report(self):
        self.populate("candidate")
        self.populate("control")
        with sqlite3.connect(self.db) as connection:
            connection.execute("PRAGMA journal_mode=WAL")
        original = promotion.read_trader
        committed, errors = threading.Event(), []

        def update_control():
            try:
                with sqlite3.connect(self.db) as writer:
                    writer.execute("UPDATE trader_positions SET realized_pnl=999 WHERE trader_id='control'")
            except Exception as error:
                errors.append(error)
            finally:
                committed.set()

        def read_and_commit(connection, trader_id, start, as_of):
            result = original(connection, trader_id, start, as_of)
            if trader_id == "candidate":
                worker = threading.Thread(target=update_control)
                worker.start()
                self.assertTrue(committed.wait(5))
                worker.join(5)
            return result

        with patch.object(promotion, "read_trader", side_effect=read_and_commit):
            result = promotion.build_report([pair()], self.db, NOW)
        self.assertEqual(errors, [])
        self.assertEqual(result["pairs"][0]["control"]["net_pnl_usdt"], 40)
        fresh = promotion.build_report([pair()], self.db, NOW)
        self.assertEqual(fresh["pairs"][0]["control"]["net_pnl_usdt"], 20 * 998)

    def test_missing_and_empty_configuration_cli_zero_exit_without_database(self):
        output = self.directory / "promotion"
        for configuration in (None, {"pairs": []}):
            with self.subTest(configuration=configuration):
                output.mkdir(exist_ok=True)
                if configuration is not None:
                    (output / "pairs.json").write_text(json.dumps(configuration), encoding="utf-8")
                with patch.dict(os.environ, {"PROMOTION_DIR": str(output), "NOFX_DB": str(output / "missing.db")}):
                    stdout = io.StringIO()
                    with redirect_stdout(stdout):
                        self.assertEqual(promotion.main([]), 0)
                result = json.loads((output / "report.json").read_text())
                self.assertEqual(result["pairs"], [])
                self.assertIn("pairs=0", stdout.getvalue())
                self.assertFalse((output / "missing.db").exists())

    def test_config_validation_and_failed_cli_preserves_existing_report(self):
        for configuration in ([], {}, {"pairs": {}}, {"pairs": ["bad"]}):
            (self.directory / "pairs.json").write_text(json.dumps(configuration))
            with self.assertRaises(ValueError):
                promotion.load_pairs(self.directory)
        for overrides in ({"min_days": 6}, {"min_trades": float("nan")},
                          {"min_trades": 1.5}, {"candidate_trader_id": "control"},
                          {"started_at": "bad"}, {"live_trader_id": 123}):
            with self.subTest(overrides=overrides), self.assertRaises(ValueError):
                promotion.normalize_pair(pair(**overrides))
        with self.assertRaises(ValueError):
            promotion.build_report([pair(), pair()], self.db, NOW)
        (self.directory / "report.json").write_text("previous report")
        with patch.dict(os.environ, {"PROMOTION_DIR": str(self.directory), "NOFX_DB": str(self.db)}):
            with redirect_stderr(io.StringIO()) as errors:
                self.assertEqual(promotion.main([]), 1)
        self.assertIn("晋升报告失败", errors.getvalue())
        self.assertEqual((self.directory / "report.json").read_text(), "previous report")

    def test_nonfinite_json_and_atomic_replace_cleanup(self):
        path = self.directory / "report.json"
        path.write_text("old")
        report = {"nested": [float("nan"), {"inf": float("inf"), "negative": -float("inf")}], "ok": 1}
        original_replace = os.replace

        def verify_replace(source, destination):
            self.assertEqual(path.read_text(), "old")
            self.assertEqual(Path(source).parent, path.parent)
            self.assertEqual(json.loads(Path(source).read_text())["nested"], [None, {"inf": None, "negative": None}])
            original_replace(source, destination)

        with patch.object(promotion.os, "replace", side_effect=verify_replace) as replace:
            promotion.write_report(path, report)
        replace.assert_called_once()
        self.assertNotIn("NaN", path.read_text())
        self.assertNotIn("Infinity", path.read_text())
        self.assertEqual(list(self.directory.glob(".report-*.tmp")), [])
        with patch.object(promotion.os, "replace", side_effect=OSError("simulated replacement failure")):
            with self.assertRaises(OSError):
                promotion.write_report(path, {"new": True})
        self.assertEqual(json.loads(path.read_text())["ok"], 1)
        self.assertEqual(list(self.directory.glob(".report-*.tmp")), [])

    def test_nonfinite_database_values_serialized_as_null_and_extremes_warned(self):
        self.populate("candidate", n=1, pnl=float("inf"))
        self.populate("control", n=1, pnl=69)
        with sqlite3.connect(self.db) as connection:
            connection.execute("UPDATE trader_equity_snapshots SET total_equity=? WHERE trader_id='candidate'", (float("inf"),))
        result = promotion.build_report([pair(notes=float("nan"))], self.db, NOW)
        path = self.directory / "report.json"
        promotion.write_report(path, result)
        item = json.loads(path.read_text())["pairs"][0]
        self.assertIsNone(item["notes"])
        self.assertIsNone(item["candidate"]["net_pnl_usdt"])
        self.assertIsNone(item["candidate"]["period_return"])
        self.assertEqual(item["control"]["r_extreme_count"], 1)
        self.assertTrue(any("|R|>15" in message for message in item["warnings"]))
        self.assertNotIn("NaN", path.read_text())
        self.assertNotIn("Infinity", path.read_text())


if __name__ == "__main__":
    unittest.main()
