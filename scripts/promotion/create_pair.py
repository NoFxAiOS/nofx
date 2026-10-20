"""Clone a live trader into two stopped paper traders (default: read-only dry-run).

Creation: python -m scripts.promotion.create_pair --source-trader ID ... [--apply]
After manually starting both traders: python -m scripts.promotion.create_pair start PAIR_ID
Only the standard library is required. Planning and JSON patching do no I/O.
"""

from __future__ import annotations

import argparse
from contextlib import closing, contextmanager
from dataclasses import dataclass
from datetime import datetime, timezone
import fcntl
import json
import math
import os
from pathlib import Path
import shlex
import sqlite3
import sys
import tempfile
import uuid


REPO_ROOT = Path(__file__).resolve().parents[2]
MIGRATION_HINT = "请先用包含 paper 支持的新版后端启动一次完成迁移"
START_NOTE = "首次启动后用 report.py --set-started 回填"


def main_repo_root() -> Path:
    """Resolve a linked worktree's main checkout without running git."""
    marker = REPO_ROOT / ".git"
    if marker.is_file():
        gitdir = (REPO_ROOT / marker.read_text().strip().removeprefix("gitdir: ")).resolve()
        common = (gitdir / (gitdir / "commondir").read_text().strip()).resolve()
        return common.parent
    return REPO_ROOT


def encode(value) -> str:
    return json.dumps(value, ensure_ascii=False, allow_nan=False)


def _unique_object(items):
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError(f"JSON 含重复字段：{key}")
        result[key] = value
    return result


def _invalid_constant(value):
    raise ValueError(f"JSON 不允许 {value}")


def decode(text: str):
    return json.loads(text, object_pairs_hook=_unique_object, parse_constant=_invalid_constant)


def set_feature(config: str, feature_path: str, value) -> str:
    """Replace only the target JSON span; retain every other source byte.

    Missing object members are inserted. Arrays/scalars on intermediate nodes,
    duplicate keys and nonstandard JSON constants are rejected, never overwritten.
    """
    parts = feature_path.split(".")
    if any(not part or part != part.strip() for part in parts):
        raise ValueError("feature-path 必须是非空点路径，例如 indicators.enable_factor_ensemble")
    if not isinstance(decode(config), dict):
        raise ValueError("源策略 config 必须是 JSON 对象")
    replacement = encode(value)
    decoder = json.JSONDecoder()

    def skip(text, index):
        while index < len(text) and text[index] in " \t\r\n":
            index += 1
        return index

    def patch(text, remaining):
        index = skip(text, 0) + 1  # opening object brace
        insert_at, count = index, 0
        while True:
            index = skip(text, index)
            if text[index] == "}":
                nested = value
                for key in reversed(remaining[1:]):
                    nested = {key: nested}
                member = encode(remaining[0]) + ": " + encode(nested)
                return text[:insert_at] + (", " if count else "") + member + text[insert_at:]
            key, end = decoder.raw_decode(text, index)
            start = skip(text, skip(text, end) + 1)  # colon, then value
            node, end = decoder.raw_decode(text, start)
            if key == remaining[0]:
                if len(remaining) == 1:
                    changed = replacement
                else:
                    if not isinstance(node, dict):
                        raise ValueError(f"feature-path 中间节点 {key} 必须是对象")
                    changed = patch(text[start:end], remaining[1:])
                return text[:start] + changed + text[end:]
            count += 1
            insert_at = end
            index = skip(text, end)
            if text[index] == ",":
                index += 1

    return patch(config, parts)


@dataclass(frozen=True)
class PairOptions:
    source_trader: str
    feature_path: str
    feature_value: object
    pair_id: str
    title: str
    initial_balance: float = 1000
    taker_bps: float = 5
    maker_bps: float = 2
    slippage_bps: float = 3
    leverage: int = 5
    min_days: float = 7
    min_trades: int = 20

    def validate(self):
        for field in ("source_trader", "feature_path", "pair_id", "title"):
            value = getattr(self, field)
            if not isinstance(value, str) or not value.strip():
                raise ValueError(f"{field} 不可为空")
        if not math.isfinite(self.initial_balance) or self.initial_balance <= 0:
            raise ValueError("initial-balance 必须是大于 0 的有限数")
        for field in ("taker_bps", "maker_bps", "slippage_bps"):
            value = getattr(self, field)
            if not math.isfinite(value) or not 0 <= value <= 100:
                raise ValueError(f"{field} 必须在 0 到 100 bps 之间")
        if type(self.leverage) is not int or not 1 <= self.leverage <= 125:
            raise ValueError("leverage 必须是 1 到 125 的整数")
        if not math.isfinite(self.min_days) or self.min_days < 7:
            raise ValueError("min-days 必须至少为 7 天")
        if type(self.min_trades) is not int or self.min_trades < 1:
            raise ValueError("min-trades 必须是正整数")
        encode(self.feature_value)

    def paper_config(self) -> dict:
        return {"initial_balance": self.initial_balance, "taker_fee_bps": self.taker_bps,
                "maker_fee_bps": self.maker_bps, "slippage_bps": self.slippage_bps,
                "default_leverage": self.leverage}


def validate_pairs(document: dict) -> None:
    if not isinstance(document, dict) or not isinstance(document.get("pairs"), list):
        raise ValueError("pairs.json 必须是包含 pairs 数组的对象")
    seen = set()
    for pair in document["pairs"]:
        if not isinstance(pair, dict) or not isinstance(pair.get("id"), str) or not pair["id"].strip():
            raise ValueError("pairs 中每一项必须是包含有效 id 的对象")
        if pair["id"] in seen:
            raise ValueError(f"pair-id 重复：{pair['id']}")
        seen.add(pair["id"])


def build_plan(options: PairOptions, source: dict, strategy: dict, exchange: dict,
               document: dict, identifiers: dict, timestamp: str) -> dict:
    """Pure plan builder. IDs/time/snapshots are explicit inputs for testing."""
    options.validate()
    validate_pairs(document)
    if any(pair["id"] == options.pair_id for pair in document["pairs"]):
        raise ValueError(f"pair-id 重复：{options.pair_id}")
    if source["id"] != options.source_trader or source["exchange_id"] != exchange["id"]:
        raise ValueError("源交易员或交易所不匹配")
    if source["strategy_id"] != strategy["id"]:
        raise ValueError("源策略不匹配")
    if exchange["exchange_type"] == "paper" or exchange["type"] == "paper":
        raise ValueError("source trader 的交易所不能是 paper")
    candidate_config = set_feature(strategy["config"], options.feature_path, options.feature_value)
    rows = {"exchanges": [], "strategies": [], "traders": []}
    all_ids = [ids[key] for ids in identifiers.values()
               for key in ("exchange_id", "strategy_id", "trader_id")]
    if len(all_ids) != 6 or len(set(all_ids)) != 6:
        raise ValueError("候选/对照的六个新 id 必须互异")
    for role, suffix, label in (("candidate", "cand", "候选"), ("control", "ctrl", "对照")):
        ids = identifiers[role]
        name = f"{options.title}-{label}"
        rows["exchanges"].append({
            "id": ids["exchange_id"], "user_id": source["user_id"],
            "exchange_type": "paper", "name": "Paper Futures (模拟盘)", "type": "paper",
            "account_name": f"{options.pair_id}-{suffix}", "enabled": 1,
            "paper_config": encode(options.paper_config()), "created_at": timestamp, "updated_at": timestamp,
        })
        rows["strategies"].append({
            **strategy, "id": ids["strategy_id"], "name": name,
            "is_active": 0, "is_default": 0, "is_public": 0,
            "config": candidate_config if role == "candidate" else strategy["config"],
        })
        rows["traders"].append({
            **source, "id": ids["trader_id"], "name": name,
            "exchange_id": ids["exchange_id"], "strategy_id": ids["strategy_id"],
            "initial_balance": options.initial_balance, "is_running": 0,
            "show_in_competition": 0, "run_version": 0,
        })
    pair = {"id": options.pair_id, "title": options.title, "feature": options.feature_path,
            "feature_value": options.feature_value,
            "candidate_trader_id": identifiers["candidate"]["trader_id"],
            "control_trader_id": identifiers["control"]["trader_id"],
            "live_trader_id": source["id"], "started_at": None, "note": START_NOTE,
            "min_days": options.min_days, "min_trades": options.min_trades}
    return {"objects": rows, "ids": identifiers, "pair": pair,
            "pairs_document": {**document, "pairs": [*document["pairs"], pair]}}


def open_database(path: Path, *, writable=False) -> sqlite3.Connection:
    mode = "rw" if writable else "ro"  # never create a missing source database
    connection = sqlite3.connect(path.resolve().as_uri() + f"?mode={mode}", uri=True, timeout=30)
    connection.row_factory = sqlite3.Row
    connection.execute("PRAGMA foreign_keys = ON")
    if not writable:
        connection.execute("PRAGMA query_only = ON")
    return connection


def quote(identifier: str) -> str:
    return '"' + identifier.replace('"', '""') + '"'


def read_source(connection, source_id):
    required = {
        "exchanges": {"id", "user_id", "exchange_type", "type", "name", "account_name", "enabled",
                      "paper_config", "created_at", "updated_at"},
        "strategies": {"id", "name", "config", "is_active", "is_default", "is_public"},
        "traders": {"id", "name", "user_id", "ai_model_id", "exchange_id", "strategy_id",
                    "initial_balance", "is_running", "show_in_competition", "run_version"},
    }
    for table, fields in required.items():
        columns = {row["name"] for row in connection.execute(f"PRAGMA table_info({quote(table)})")}
        if table == "exchanges" and "paper_config" not in columns:
            raise ValueError(MIGRATION_HINT)
        if missing := fields - columns:
            raise ValueError(f"{table} 缺少字段 {', '.join(sorted(missing))}；{MIGRATION_HINT}")

    def one(table, row_id, columns="*"):
        row = connection.execute(f"SELECT {columns} FROM {quote(table)} WHERE id = ?", (row_id,)).fetchone()
        if row is None:
            raise ValueError(f"{table} 中不存在 id={row_id}")
        return dict(row)

    source = one("traders", source_id)
    exchange = one("exchanges", source["exchange_id"], "id, exchange_type, type")
    strategy = one("strategies", source["strategy_id"])
    return source, strategy, exchange


def load_document(path: Path) -> tuple[dict, bytes | None]:
    try:
        previous = path.read_bytes()
    except FileNotFoundError:
        return {"pairs": []}, None
    document = decode(previous.decode("utf-8"))
    validate_pairs(document)
    return document, previous


def atomic_write(path: Path, payload: bytes) -> None:
    """Stage/fsync beside the destination, atomically replace, clean up failures."""
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=path.parent, prefix=".pairs-", suffix=".tmp", delete=False) as handle:
            temporary = Path(handle.name)
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def document_bytes(document: dict) -> bytes:
    return (json.dumps(document, ensure_ascii=False, indent=2, allow_nan=False) + "\n").encode("utf-8")


@contextmanager
def pairs_lock(directory: Path):
    """Serialize this script's apply/start operations, including different DBs."""
    directory.mkdir(parents=True, exist_ok=True)
    with (directory / ".pairs.lock").open("a") as handle:
        fcntl.flock(handle, fcntl.LOCK_EX)
        try:
            yield
        finally:
            fcntl.flock(handle, fcntl.LOCK_UN)


def backup_database(db_path: Path, directory: Path) -> Path:
    """Online SQLite backup includes committed WAL data; never copy just the .db."""
    directory.mkdir(parents=True, exist_ok=True)
    target = None
    try:
        stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
        with tempfile.NamedTemporaryFile(dir=directory, prefix=f"{db_path.stem}-{stamp}-",
                                         suffix=".sqlite3", delete=False) as handle:
            target = Path(handle.name)
        with closing(open_database(db_path)) as source, closing(sqlite3.connect(target)) as destination:
            source.backup(destination)
        return target
    except BaseException:
        if target is not None:
            target.unlink(missing_ok=True)
        raise


def new_identifiers(source: dict) -> dict:
    result = {}
    for role in ("candidate", "control"):
        exchange_id = str(uuid.uuid4())
        result[role] = {"exchange_id": exchange_id, "strategy_id": str(uuid.uuid4()),
                        # Same style as api/handler_trader.go: exchange prefix + model + UUID.
                        "trader_id": f"{exchange_id[:8]}_{source['ai_model_id']}_{uuid.uuid4()}"}
    return result


def insert_statement(table: str, row: dict) -> tuple[str, tuple]:
    columns = ", ".join(quote(column) for column in row)
    placeholders = ", ".join("?" for _ in row)
    return f"INSERT INTO {quote(table)} ({columns}) VALUES ({placeholders})", tuple(row.values())


def create_pair(db_path: Path, promotion_dir: Path, options: PairOptions, *,
                apply=False, backup_dir: Path | None = None) -> tuple[dict, Path | None]:
    """Read-only planning by default; apply backs up before the six-row transaction.

    Publish pairs.json before COMMIT so file failures roll back all inserts. If
    COMMIT fails, restore the exact old file. SQLite and JSON cannot share a
    crash-atomic transaction; an abrupt process/machine failure needs inspection.
    """
    options.validate()
    path = promotion_dir / "pairs.json"

    def plan(connection):
        document, previous = load_document(path)
        source, strategy, exchange = read_source(connection, options.source_trader)
        timestamp = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
        return build_plan(options, source, strategy, exchange, document,
                          new_identifiers(source), timestamp), previous

    if not apply:
        with closing(open_database(db_path)) as connection:
            connection.execute("BEGIN")
            result, _ = plan(connection)
        return result, None

    with pairs_lock(promotion_dir), closing(open_database(db_path, writable=True)) as connection:
        published = False
        previous = None
        connection.execute("BEGIN IMMEDIATE")
        try:
            result, previous = plan(connection)
            # A separate RO connection can back up while this connection holds
            # the writer reservation. No other writer can change the snapshot.
            backup = backup_database(db_path, backup_dir if backup_dir is not None else promotion_dir / "backups")
            for table, rows in result["objects"].items():
                for row in rows:
                    sql, parameters = insert_statement(table, row)
                    connection.execute(sql, parameters)
            atomic_write(path, document_bytes(result["pairs_document"]))
            published = True
            connection.commit()
        except BaseException:
            try:
                if published:
                    if previous is None:
                        path.unlink(missing_ok=True)
                    else:
                        atomic_write(path, previous)
            finally:
                connection.rollback()
            raise
    return result, backup


def start_pair(promotion_dir: Path, pair_id: str, *, now: datetime | None = None) -> dict:
    """Record the first manual start, without opening a database or starting traders."""
    with pairs_lock(promotion_dir):
        path = promotion_dir / "pairs.json"
        document, _ = load_document(path)
        pair = next((pair for pair in document["pairs"] if pair["id"] == pair_id), None)
        if pair is None:
            raise ValueError(f"pair-id 不存在：{pair_id}")
        if pair.get("started_at") is not None:
            raise ValueError(f"{pair_id} 已有 started_at，拒绝重置实验起点")
        timestamp = now if now is not None else datetime.now(timezone.utc)
        if timestamp.tzinfo is None or timestamp.utcoffset() is None:
            raise ValueError("started_at 必须包含时区")
        pair["started_at"] = timestamp.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")
        pair["note"] = "已用 create_pair.py start 回填首次启动时间（UTC）"
        atomic_write(path, document_bytes(document))
        return pair


def print_plan(plan: dict, db_path: Path, promotion_dir: Path, *, apply: bool, backup: Path | None):
    print(f"{'APPLIED' if apply else 'DRY-RUN（数据库以 mode=ro 打开，未写库/文件）'} | db={db_path}")
    if backup is not None:
        print(f"备份：{backup}")
    if not apply:
        print("-- 将执行的 SQL（? 为绑定参数，params 按顺序列出）")
        print("BEGIN IMMEDIATE;")
        for table, rows in plan["objects"].items():
            for row in rows:
                sql, parameters = insert_statement(table, row)
                print(sql + ";")
                print("-- params: " + encode(parameters))
        print(f"-- 原子更新 {promotion_dir / 'pairs.json'}，失败则 ROLLBACK")
        print("COMMIT;")
        print("-- 将写入的对象：")
        print(json.dumps(plan["objects"], ensure_ascii=False, indent=2, allow_nan=False))
        print("-- pairs.json 合并结果：")
        print(document_bytes(plan["pairs_document"]).decode("utf-8"), end="")
    print(f"[{plan['pair']['id']}] 最终清单" if apply else f"[{plan['pair']['id']}] 预览清单（apply 将生成新 id）")
    for role, label in (("candidate", "候选"), ("control", "对照")):
        ids = plan["ids"][role]
        print(f"{label}: exchange={ids['exchange_id']} strategy={ids['strategy_id']} trader={ids['trader_id']}")
    command = (f"PROMOTION_DIR={shlex.quote(str(promotion_dir.resolve()))} {shlex.quote(sys.executable)} "
               f"{shlex.quote(str(Path(__file__).resolve()))} start {shlex.quote(plan['pair']['id'])}")
    if not apply:
        print("下一步：核对预览后，在原建对命令追加 --apply。")
    print("建对后在界面里手动启动这两个交易员，然后在首次启动当天执行（不会自动启动）：")
    print(command)


def main(argv=None) -> int:
    argv = list(sys.argv[1:] if argv is None else argv)
    promotion_dir = Path(os.environ.get("PROMOTION_DIR", REPO_ROOT / "data/promotion"))
    if argv and argv[0] == "start":
        parser = argparse.ArgumentParser(description="首次手动启动当天回填 started_at（只写 pairs.json）")
        parser.add_argument("pair_id")
        args = parser.parse_args(argv[1:])
        try:
            pair = start_pair(promotion_dir, args.pair_id)
        except (OSError, ValueError) as error:
            print(f"记录启动时间失败：{error}", file=sys.stderr)
            return 1
        print(f"[{pair['id']}] started_at={pair['started_at']}；未启动交易员，未访问数据库")
        return 0

    parser = argparse.ArgumentParser(description=__doc__, epilog="子命令：start <pair-id> 回填首次启动 UTC 时间")
    parser.add_argument("--db", type=Path, default=None, help="SQLite 路径；默认 NOFX_DB 或主仓库 data/data.db")
    for name in ("source-trader", "feature-path", "pair-id", "title"):
        parser.add_argument(f"--{name}", required=True)
    parser.add_argument("--feature-value", required=True, type=decode, help="JSON 值，例如 true、0.5、\"text\"")
    parser.add_argument("--initial-balance", type=float, default=1000)
    parser.add_argument("--taker-bps", type=float, default=5)
    parser.add_argument("--maker-bps", type=float, default=2)
    parser.add_argument("--slippage-bps", type=float, default=3)
    parser.add_argument("--leverage", type=int, default=5)
    parser.add_argument("--min-days", type=float, default=7)
    parser.add_argument("--min-trades", type=int, default=20)
    parser.add_argument("--apply", action="store_true", help="先备份，再写入；省略时只读 dry-run")
    parser.add_argument("--backup-dir", type=Path, help="默认 PROMOTION_DIR/backups；备份失败则中止")
    args = parser.parse_args(argv)
    try:
        db_path = args.db if args.db is not None else Path(os.environ.get("NOFX_DB") or main_repo_root() / "data/data.db")
        options = PairOptions(**{key: getattr(args, key) for key in PairOptions.__dataclass_fields__})
        plan, backup = create_pair(db_path, promotion_dir, options, apply=args.apply, backup_dir=args.backup_dir)
    except (OSError, ValueError, sqlite3.Error) as error:
        print(f"创建模拟盘对失败：{error}", file=sys.stderr)
        return 1
    print_plan(plan, db_path, promotion_dir, apply=args.apply, backup=backup)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
