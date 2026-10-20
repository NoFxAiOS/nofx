"""All writes target temporary databases, never the live database."""

from contextlib import closing, redirect_stderr, redirect_stdout
from dataclasses import replace
from datetime import datetime, timedelta, timezone
import io
import json
import os
from pathlib import Path
import sqlite3
import tempfile
import unittest
from unittest.mock import patch
import uuid

from scripts.promotion import create_pair as pair_cli
from scripts.promotion import report


# Copied from sqlite3 -readonly /Users/zhangyun/workspace/nofx/data/data.db
# .schema traders / .schema strategies / .schema exchanges, 2026-10-20.
# The live schema has not migrated paper_config yet; tests add that column.
SCHEMA = """
CREATE TABLE `traders` (`id` text,`user_id` text NOT NULL DEFAULT "default",`name` text NOT NULL,`ai_model_id` text NOT NULL,`exchange_id` text NOT NULL,`strategy_id` text DEFAULT "",`initial_balance` real NOT NULL,`scan_interval_minutes` integer DEFAULT 3,`is_running` numeric DEFAULT false,`is_cross_margin` numeric DEFAULT true,`show_in_competition` numeric DEFAULT true,`created_at` datetime,`updated_at` datetime,`btc_eth_leverage` integer DEFAULT 5,`altcoin_leverage` integer DEFAULT 5,`trading_symbols` text DEFAULT "",`use_coin_pool` numeric DEFAULT false,`use_oi_top` numeric DEFAULT false,`custom_prompt` text DEFAULT "",`override_base_prompt` numeric DEFAULT false,`system_prompt_template` text DEFAULT "default", `run_version` integer NOT NULL DEFAULT 0,PRIMARY KEY (`id`));
CREATE INDEX `idx_traders_user_id` ON `traders`(`user_id`);
CREATE TABLE `strategies` (`id` text,`user_id` text NOT NULL DEFAULT "",`name` text NOT NULL,`description` text DEFAULT "",`is_active` numeric DEFAULT false,`is_default` numeric DEFAULT false,`is_public` numeric DEFAULT false,`config_visible` numeric DEFAULT true,`config` text NOT NULL DEFAULT "{}",`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`));
CREATE INDEX `idx_strategies_is_public` ON `strategies`(`is_public`);
CREATE INDEX `idx_strategies_is_active` ON `strategies`(`is_active`);
CREATE INDEX `idx_strategies_user_id` ON `strategies`(`user_id`);
CREATE TABLE `exchanges` (`id` text,`exchange_type` text NOT NULL DEFAULT "",`account_name` text NOT NULL DEFAULT "",`user_id` text NOT NULL DEFAULT "default",`name` text NOT NULL,`type` text NOT NULL,`enabled` numeric DEFAULT false,`api_key` text DEFAULT "",`secret_key` text DEFAULT "",`passphrase` text DEFAULT "",`testnet` numeric DEFAULT false,`hyperliquid_wallet_addr` text DEFAULT "",`hyperliquid_unified_account` numeric DEFAULT true,`aster_user` text DEFAULT "",`aster_signer` text DEFAULT "",`aster_private_key` text DEFAULT "",`lighter_wallet_addr` text DEFAULT "",`lighter_private_key` text DEFAULT "",`lighter_api_key_private_key` text DEFAULT "",`lighter_api_key_index` integer DEFAULT 0,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`));
CREATE INDEX `idx_exchanges_user_id` ON `exchanges`(`user_id`);
"""
CONFIG = (' {\n  "indicators" : { "enable_factor_ensemble" : false, "k" : 1e-3 },\n'
          ' "risk": {"x":1.00}, "中文":"保留\\u4e2d", "array": [1, {"a":2}]\n} \n')
SOURCE = "abcd1234_user-model_12940599-650a-4e4b-a5fc-986208fb912e"
OPTIONS = pair_cli.PairOptions(SOURCE, "indicators.enable_factor_ensemble", True, "ensemble-v1", "因子验证")


class FeaturePatchTests(unittest.TestCase):
    def test_preserves_every_byte_outside_target_and_does_not_mutate_value(self):
        changed = pair_cli.set_feature(CONFIG, OPTIONS.feature_path, True)
        self.assertEqual(changed, CONFIG.replace("false", "true", 1))
        self.assertEqual(pair_cli.set_feature(CONFIG, OPTIONS.feature_path, False), CONFIG)
        value = {"weights": [1, 2], "enabled": True}
        changed = pair_cli.set_feature(CONFIG, "indicators.new", value)
        self.assertEqual(value, {"weights": [1, 2], "enabled": True})
        parsed = json.loads(changed)
        self.assertEqual(parsed["indicators"].pop("new"), value)
        self.assertEqual(parsed, json.loads(CONFIG))

    def test_missing_nodes_empty_objects_escaped_keys_and_complex_leaf(self):
        for source in ('{}', ' \n{ \t}\n', '{"risk": 1.00, "other": [true, null]}'):
            with self.subTest(source=source):
                changed = pair_cli.set_feature(source, "indicators.nested.flag", True)
                expected = json.loads(source)
                expected["indicators"] = {"nested": {"flag": True}}
                self.assertEqual(json.loads(changed), expected)
        escaped = '{"indica\\u0074ors":{"flag": [0, {"x": "}"}]},"tail":1e2}'
        self.assertEqual(pair_cli.set_feature(escaped, "indicators.flag", None),
                         escaped.replace('[0, {"x": "}"}]', 'null'))

    def test_invalid_path_json_and_non_object_intermediate_are_rejected(self):
        for source, path in ((CONFIG, ""), (CONFIG, "a..b"), (CONFIG, ".a"),
                             (CONFIG, "a."), ('{"indicators":null}', OPTIONS.feature_path),
                             ('{"indicators":[]}', OPTIONS.feature_path),
                             ('{"indicators":false}', OPTIONS.feature_path),
                             ('[]', 'a'), ('{"a":1,"a":2}', 'a'), ('{"a":NaN}', 'a')):
            with self.subTest(source=source, path=path), self.assertRaises(ValueError):
                pair_cli.set_feature(source, path, True)

    def test_paper_limits_and_report_thresholds(self):
        for changes in ({"initial_balance": 0}, {"initial_balance": float("inf")},
                        {"taker_bps": -1}, {"maker_bps": 101}, {"slippage_bps": float("nan")},
                        {"leverage": 126}, {"leverage": 1.5}, {"min_days": 6.9},
                        {"min_days": float("nan")}, {"min_trades": 0}, {"min_trades": 1.5},
                        {"pair_id": " "}, {"feature_value": float("inf")}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                replace(OPTIONS, **changes).validate()


class CreatePairTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix=".test-pair-", dir=Path(__file__).parent)
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.db = self.directory / "paper #1?.db"
        self.promotion = self.directory / "promotion"
        self.backups = self.directory / "backups"
        with sqlite3.connect(self.db) as connection:
            connection.executescript(SCHEMA)
            connection.execute("ALTER TABLE exchanges ADD COLUMN paper_config TEXT NOT NULL DEFAULT ''")
            connection.execute("INSERT INTO exchanges(id,user_id,exchange_type,name,type,api_key,secret_key) "
                               "VALUES ('live-exchange','user','binance','Binance','cex','secret-api','secret-key')")
            connection.execute("INSERT INTO strategies(id,user_id,name,description,is_active,is_default,"
                               "is_public,config_visible,config,created_at,updated_at) "
                               "VALUES ('source-strategy','user','源策略','说明',1,1,1,0,?,'2026-01-01','2026-02-01')", (CONFIG,))
            connection.execute("INSERT INTO traders(id,user_id,name,ai_model_id,exchange_id,strategy_id,"
                               "initial_balance,scan_interval_minutes,is_running,is_cross_margin,show_in_competition,"
                               "btc_eth_leverage,altcoin_leverage,trading_symbols,use_coin_pool,use_oi_top,custom_prompt,"
                               "override_base_prompt,system_prompt_template,run_version,created_at,updated_at) "
                               "VALUES (?,'user','源交易员','user-model','live-exchange','source-strategy',"
                               "4200,11,1,0,1,7,4,'BTCUSDT,ETHUSDT',1,1,'自定义提示词',1,'aggressive',33,"
                               "'2026-01-02','2026-02-02')", (SOURCE,))

    def rows(self, table, database=None):
        with closing(pair_cli.open_database(database or self.db)) as connection:
            return [dict(row) for row in connection.execute(f'SELECT * FROM "{table}" ORDER BY id')]

    def counts(self):
        return [len(self.rows(table)) for table in ("exchanges", "strategies", "traders")]

    def apply(self, options=OPTIONS):
        return pair_cli.create_pair(self.db, self.promotion, options, apply=True, backup_dir=self.backups)

    def existing_document(self):
        document = {"schema_version": 9, "custom": {"keep": [1, "中文"]},
                    "pairs": [{"id": "old-pair", "note": "保留", "started_at": None}]}
        self.promotion.mkdir()
        (self.promotion / "pairs.json").write_text(json.dumps(document, ensure_ascii=False), encoding="utf-8")
        return document

    def test_dry_run_cli_is_readonly_no_directories_or_changes_and_prints_plan(self):
        before = self.db.read_bytes()
        with patch.dict(os.environ, {"PROMOTION_DIR": str(self.promotion), "NOFX_DB": str(self.db)}):
            with redirect_stdout(io.StringIO()) as output:
                self.assertEqual(pair_cli.main([
                    "--source-trader", SOURCE, "--feature-path", OPTIONS.feature_path, "--feature-value", "true",
                    "--pair-id", OPTIONS.pair_id, "--title", OPTIONS.title, "--backup-dir", str(self.backups)]), 0)
        self.assertEqual(before, self.db.read_bytes())
        self.assertEqual(self.counts(), [1, 1, 1])
        self.assertFalse(self.promotion.exists())
        self.assertFalse(self.backups.exists())
        text = output.getvalue()
        self.assertIn("DRY-RUN", text)
        self.assertEqual(text.count('INSERT INTO "'), 6)
        self.assertIn('"started_at": null', text)
        self.assertIn("start ensemble-v1", text)
        self.assertNotIn("secret-api", text)
        self.assertNotIn("secret-key", text)

    def test_readonly_mode_cannot_write_and_missing_db_is_never_created(self):
        with closing(pair_cli.open_database(self.db)) as connection:
            self.assertEqual(connection.execute("PRAGMA query_only").fetchone()[0], 1)
            connection.execute("PRAGMA query_only = OFF")
            with self.assertRaises(sqlite3.OperationalError):
                connection.execute("DELETE FROM traders")
        missing = self.directory / "absent.db"
        for apply in (False, True):
            with self.assertRaises(sqlite3.OperationalError):
                pair_cli.create_pair(missing, self.promotion, OPTIONS, apply=apply)
        self.assertFalse(missing.exists())

    def test_apply_two_independent_accounts_exact_clone_and_only_feature_diff(self):
        original = {table: self.rows(table)[0] for table in ("traders", "strategies", "exchanges")}
        options = replace(OPTIONS, initial_balance=2500, taker_bps=7, maker_bps=1, slippage_bps=4, leverage=8)
        plan, backup = self.apply(options)
        self.assertEqual(self.counts(), [3, 3, 3])
        self.assertTrue(backup.is_file())
        self.assertEqual([len(self.rows(table, backup)) for table in original], [1, 1, 1])
        self.assertEqual(self.rows("traders", backup), [original["traders"]])
        with closing(pair_cli.open_database(self.db)) as connection:
            configs = []
            for role, suffix, label in (("candidate", "cand", "候选"), ("control", "ctrl", "对照")):
                ids = plan["ids"][role]
                self.assertEqual(uuid.UUID(ids["exchange_id"]).version, 4)
                self.assertEqual(uuid.UUID(ids["strategy_id"]).version, 4)
                self.assertTrue(ids["trader_id"].startswith(ids["exchange_id"][:8] + "_user-model_"))
                trader = dict(connection.execute("SELECT * FROM traders WHERE id=?", (ids["trader_id"],)).fetchone())
                strategy = dict(connection.execute("SELECT * FROM strategies WHERE id=?", (trader["strategy_id"],)).fetchone())
                exchange = dict(connection.execute("SELECT * FROM exchanges WHERE id=?", (trader["exchange_id"],)).fetchone())
                for key, value in original["traders"].items():
                    if key not in {"id", "name", "exchange_id", "strategy_id", "initial_balance", "is_running",
                                   "show_in_competition", "run_version"}:
                        self.assertEqual(trader[key], value, key)
                for key, value in original["strategies"].items():
                    if key not in {"id", "name", "is_active", "is_default", "is_public", "config"}:
                        self.assertEqual(strategy[key], value, key)
                self.assertEqual(trader["name"], f"因子验证-{label}")
                self.assertEqual(strategy["name"], trader["name"])
                self.assertEqual([trader[key] for key in ("initial_balance", "is_running", "show_in_competition", "run_version")],
                                 [2500, 0, 0, 0])
                self.assertEqual([strategy[key] for key in ("is_active", "is_default", "is_public")], [0, 0, 0])
                self.assertEqual([exchange[key] for key in ("user_id", "exchange_type", "name", "type", "enabled")],
                                 ["user", "paper", "Paper Futures (模拟盘)", "paper", 1])
                self.assertEqual(exchange["account_name"], "ensemble-v1-" + suffix)
                self.assertEqual(exchange["api_key"], "")
                self.assertEqual(exchange["secret_key"], "")
                self.assertEqual(json.loads(exchange["paper_config"]), options.paper_config())
                configs.append(strategy["config"])
            for table, row in original.items():
                self.assertEqual(dict(connection.execute(f'SELECT * FROM "{table}" WHERE id=?', (row["id"],)).fetchone()), row)
        self.assertEqual(configs, [CONFIG.replace("false", "true", 1), CONFIG])
        self.assertNotEqual(plan["ids"]["candidate"]["exchange_id"], plan["ids"]["control"]["exchange_id"])
        document = json.loads((self.promotion / "pairs.json").read_text())
        pair = document["pairs"][0]
        self.assertIsNone(pair["started_at"])
        self.assertEqual(pair["live_trader_id"], SOURCE)
        self.assertEqual(pair["note"], pair_cli.START_NOTE)
        self.assertEqual((pair["min_days"], pair["min_trades"]), (7, 20))

    def test_existing_metadata_preserved_and_duplicate_rejected_without_changes(self):
        document = self.existing_document()
        self.apply()
        path = self.promotion / "pairs.json"
        updated = json.loads(path.read_text())
        self.assertEqual(updated["custom"], document["custom"])
        self.assertEqual(updated["schema_version"], 9)
        self.assertEqual(updated["pairs"][0], document["pairs"][0])
        before = self.db.read_bytes(), path.read_bytes(), list(self.backups.iterdir())
        for apply in (False, True):
            with self.subTest(apply=apply), self.assertRaisesRegex(ValueError, "pair-id 重复"):
                pair_cli.create_pair(self.db, self.promotion, OPTIONS, apply=apply, backup_dir=self.backups)
        self.assertEqual(before, (self.db.read_bytes(), path.read_bytes(), list(self.backups.iterdir())))

    def test_missing_migration_source_or_strategy_and_paper_source_rejected(self):
        cases = [
            ("ALTER TABLE exchanges DROP COLUMN paper_config", pair_cli.MIGRATION_HINT),
            ("DELETE FROM traders", "traders 中不存在"),
            ("DELETE FROM strategies", "strategies 中不存在"),
            ("DELETE FROM exchanges", "exchanges 中不存在"),
            ("UPDATE exchanges SET exchange_type='paper'", "不能是 paper"),
            ("UPDATE exchanges SET type='paper'", "不能是 paper"),
            ("UPDATE strategies SET config='{\"indicators\":false}'", "中间节点"),
        ]
        for sql, message in cases:
            with self.subTest(sql=sql):
                with sqlite3.connect(self.db) as connection:
                    connection.execute("BEGIN")
                    connection.execute(sql)
                    # Inspect a committed isolated copy without altering the fixture.
                    snapshot = self.directory / "invalid.db"
                    # serialize includes this connection's uncommitted modifications.
                    snapshot.write_bytes(connection.serialize())
                    connection.rollback()
                for apply in (False, True):
                    with self.assertRaisesRegex(ValueError, message):
                        pair_cli.create_pair(snapshot, self.promotion, OPTIONS, apply=apply, backup_dir=self.backups)
                self.assertEqual(self.counts(), [1, 1, 1])
                self.assertFalse(self.backups.exists())
                self.assertFalse((self.promotion / "pairs.json").exists())

    def test_fifth_insert_failure_rolls_back_all_six_rows_and_preserves_pairs(self):
        self.existing_document()
        path = self.promotion / "pairs.json"
        before = path.read_bytes()
        with sqlite3.connect(self.db) as connection:
            connection.execute("CREATE TRIGGER fail_fifth BEFORE INSERT ON traders "
                               "BEGIN SELECT RAISE(ABORT, 'fifth insert failed'); END")
        with self.assertRaisesRegex(sqlite3.IntegrityError, "fifth insert failed"):
            self.apply()
        self.assertEqual(self.counts(), [1, 1, 1])
        self.assertEqual(path.read_bytes(), before)
        self.assertEqual(len(list(self.backups.glob("*.sqlite3"))), 1)

    def test_atomic_file_replace_failure_rolls_back_database_and_cleans_staging(self):
        self.existing_document()
        path = self.promotion / "pairs.json"
        before = path.read_bytes()
        with patch.object(pair_cli.os, "replace", side_effect=OSError("file publish failed")):
            with self.assertRaisesRegex(OSError, "file publish failed"):
                self.apply()
        self.assertEqual(self.counts(), [1, 1, 1])
        self.assertEqual(path.read_bytes(), before)
        self.assertEqual(list(self.promotion.glob(".pairs-*.tmp")), [])

    def test_commit_failure_restores_exact_old_file_or_removes_new_file(self):
        original_open = pair_cli.open_database

        class BadCommit(sqlite3.Connection):
            def commit(self):
                raise sqlite3.OperationalError("commit failed")

        def open_with_bad_commit(path, *, writable=False):
            if not writable:
                return original_open(path)
            connection = sqlite3.connect(path, factory=BadCommit)
            connection.row_factory = sqlite3.Row
            return connection

        for existing in (False, True):
            if existing:
                path = self.promotion / "pairs.json"
                path.write_bytes(b'{ "pairs" : [], "unknown": 1 }\n')
            path = self.promotion / "pairs.json"
            before = path.read_bytes() if path.exists() else None
            with patch.object(pair_cli, "open_database", side_effect=open_with_bad_commit):
                with self.assertRaisesRegex(sqlite3.OperationalError, "commit failed"):
                    self.apply()
            self.assertEqual(self.counts(), [1, 1, 1])
            self.assertEqual(path.read_bytes() if path.exists() else None, before)

    def test_backup_failure_aborts_before_inserts_or_pairs_publish(self):
        self.backups.write_text("not a directory")
        before = self.db.read_bytes()
        with self.assertRaises(OSError):
            self.apply()
        self.assertEqual(self.counts(), [1, 1, 1])
        self.assertEqual(self.db.read_bytes(), before)
        self.assertFalse((self.promotion / "pairs.json").exists())

    def test_online_backup_and_clone_include_committed_wal_data(self):
        with sqlite3.connect(self.db) as writer:
            writer.execute("PRAGMA journal_mode=WAL")
            writer.execute("PRAGMA wal_autocheckpoint=0")
            writer.execute("UPDATE traders SET custom_prompt='committed WAL'")
            writer.commit()
            self.assertTrue(Path(str(self.db) + "-wal").exists())
            _, backup = self.apply()
        self.assertEqual(self.rows("traders", backup)[0]["custom_prompt"], "committed WAL")
        self.assertEqual({row["custom_prompt"] for row in self.rows("traders")}, {"committed WAL"})

    def test_clone_unknown_columns_and_missing_feature_nodes(self):
        with sqlite3.connect(self.db) as connection:
            connection.execute("ALTER TABLE traders ADD COLUMN future_setting TEXT DEFAULT 'trader-value'")
            connection.execute("ALTER TABLE strategies ADD COLUMN future_setting TEXT DEFAULT 'strategy-value'")
            connection.execute("UPDATE strategies SET config=' {\"risk\":1.00} '")
        self.apply()
        self.assertEqual({row["future_setting"] for row in self.rows("traders")}, {"trader-value"})
        strategies = self.rows("strategies")
        self.assertEqual({row["future_setting"] for row in strategies}, {"strategy-value"})
        self.assertIn(' {"risk":1.00} ', [row["config"] for row in strategies])
        self.assertIn({"risk": 1.0, "indicators": {"enable_factor_ensemble": True}},
                      [json.loads(row["config"]) for row in strategies])

    def test_start_sets_utc_once_and_is_consumable_by_report_without_database_access(self):
        self.existing_document()
        plan, _ = self.apply()
        before_db = self.db.read_bytes()
        with patch.object(pair_cli, "open_database", side_effect=AssertionError("start opened DB")):
            timestamp = datetime(2026, 10, 20, 16, 30, tzinfo=timezone(timedelta(hours=8)))
            pair = pair_cli.start_pair(self.promotion, OPTIONS.pair_id, now=timestamp)
        self.assertEqual(pair["started_at"], "2026-10-20T08:30:00Z")
        self.assertEqual(report.normalize_pair(pair)["candidate_trader_id"], plan["ids"]["candidate"]["trader_id"])
        self.assertEqual(report.load_pairs(self.promotion)[0]["id"], "old-pair")
        path = self.promotion / "pairs.json"
        before = path.read_bytes()
        for pair_id, message in ((OPTIONS.pair_id, "拒绝重置"), ("missing", "不存在")):
            with self.assertRaisesRegex(ValueError, message):
                pair_cli.start_pair(self.promotion, pair_id)
        self.assertEqual(path.read_bytes(), before)
        self.assertEqual(self.db.read_bytes(), before_db)

    def test_cli_apply_start_and_db_flag_overrides_env(self):
        with patch.dict(os.environ, {"PROMOTION_DIR": str(self.promotion), "NOFX_DB": str(self.directory / "missing.db")}):
            with redirect_stdout(io.StringIO()) as output:
                code = pair_cli.main(["--db", str(self.db), "--source-trader", SOURCE, "--feature-path", OPTIONS.feature_path,
                                      "--feature-value", "true", "--pair-id", OPTIONS.pair_id, "--title", OPTIONS.title,
                                      "--backup-dir", str(self.backups), "--apply"])
            self.assertEqual(code, 0)
            self.assertIn("APPLIED", output.getvalue())
            with redirect_stdout(io.StringIO()) as output:
                self.assertEqual(pair_cli.main(["start", OPTIONS.pair_id]), 0)
            self.assertIn("started_at=", output.getvalue())
            self.assertIsNotNone(json.loads((self.promotion / "pairs.json").read_text())["pairs"][0]["started_at"])
            with redirect_stderr(io.StringIO()) as errors:
                self.assertEqual(pair_cli.main(["start", OPTIONS.pair_id]), 1)
            self.assertIn("拒绝重置", errors.getvalue())

    def test_malformed_pairs_never_overwritten(self):
        self.promotion.mkdir()
        path = self.promotion / "pairs.json"
        for text in ('[]', '{}', '{"pairs":{}}', '{"pairs":["bad"]}',
                     '{"pairs":[{"id":"x"},{"id":"x"}]}', '{invalid'):
            path.write_text(text)
            with self.subTest(text=text), self.assertRaises(ValueError):
                self.apply()
            self.assertEqual(path.read_text(), text)
            self.assertEqual(self.counts(), [1, 1, 1])
        self.assertFalse(self.backups.exists())


if __name__ == "__main__":
    unittest.main()
