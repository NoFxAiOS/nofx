"""Compare paper traders without changing the database or trader configuration.

Run from the repository root: python -m scripts.promotion.report
All calculations use the standard library; calculation functions do no I/O.
"""

from __future__ import annotations

import argparse
from collections import defaultdict
from contextlib import closing
from datetime import datetime, timedelta, timezone
import json
import math
import os
from pathlib import Path
import sqlite3
import sys
import tempfile


UTC = timezone.utc
REPO_ROOT = Path(__file__).resolve().parents[2]
CAVEAT = (
    "一周样本统计功效有限：ready 只表示未发现比对照差，不是已证明更优；"
    "上线前仍需人工确认并保持小仓位/可回滚"
)
PNL_BASIS = "net_pnl_usdt = realized_pnl - fee（realized_pnl 为未扣手续费盈亏）"


def as_utc(value: str | datetime) -> datetime:
    """Parse GORM/ISO timestamps; legacy naive database times mean UTC."""
    parsed = value if isinstance(value, datetime) else datetime.fromisoformat(value.replace("Z", "+00:00"))
    return parsed.replace(tzinfo=UTC) if parsed.tzinfo is None else parsed.astimezone(UTC)


def iso(value: datetime) -> str:
    return as_utc(value).isoformat().replace("+00:00", "Z")


def midnight(value: datetime) -> datetime:
    return as_utc(value).replace(hour=0, minute=0, second=0, microsecond=0)


def finite(value) -> float | None:
    try:
        result = float(value)
    except (ValueError, TypeError, OverflowError):
        return None
    return result if math.isfinite(result) else None


def finite_sum(values) -> float | None:
    values = list(values)
    if any(value is None for value in values):
        return None
    try:
        return finite(math.fsum(values))
    except (ValueError, OverflowError):
        return None


def mean(values) -> float | None:
    values = list(values)
    total = finite_sum(values)
    return finite(total / len(values)) if values and total is not None else None


def json_safe(value):
    """Recursively replace non-finite floats, including configuration metadata."""
    if isinstance(value, float):
        return finite(value)
    if isinstance(value, dict):
        return {key: json_safe(item) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [json_safe(item) for item in value]
    return value


def trade_statistics(trades: list[dict]) -> tuple[dict, list[tuple[datetime, float]]]:
    """Closed rows, already restricted by exit_time; R uses quantity, not entry_quantity."""
    nets, fees, gross, r_samples, extremes = [], [], [], [], []
    excluded = defaultdict(int)
    for row in trades:
        pnl, fee = finite(row.get("realized_pnl")), finite(row.get("fee"))
        net = finite(pnl - fee) if pnl is not None and fee is not None else None
        gross.append(pnl)
        fees.append(fee)
        nets.append(net)
        entry = finite(row.get("entry_price"))
        stop = finite(row.get("initial_stop_loss"))
        quantity = finite(row.get("quantity"))
        if entry is None or entry <= 0 or stop is None or stop <= 0:
            excluded["missing_or_invalid_price_stop"] += 1
            continue
        distance = abs(entry - stop)
        if distance <= max(1e-12, abs(entry) * 1e-9):
            excluded["near_zero_stop_distance"] += 1
            continue
        if quantity is None or quantity <= 0:
            excluded["missing_or_invalid_quantity"] += 1
            continue
        risk = finite(distance * quantity)
        r = finite(net / risk) if net is not None and risk is not None and risk > 0 else None
        if r is None:
            excluded["nonfinite_pnl_or_risk"] += 1
            continue
        exited = datetime.fromtimestamp(row["exit_time"] / 1000, UTC)
        r_samples.append((exited, r))
        if abs(r) > 15:
            extremes.append({"position_id": row["id"], "symbol": row.get("symbol"),
                             "exit_time": iso(exited), "r": r, "net_pnl_usdt": net,
                             "initial_risk_usdt": risk})
    n = len(trades)
    invalid = sum(value is None for value in nets)
    return {
        "n": n, "net_pnl_usdt": finite_sum(nets), "gross_pnl_usdt": finite_sum(gross),
        "fees_usdt": finite_sum(fees), "pnl_basis": PNL_BASIS,
        "pnl_invalid": invalid,
        "win_rate": sum(value > 0 for value in nets if value is not None) / n if n and not invalid else None,
        "avg_r": mean(value for _, value in r_samples), "r_n": len(r_samples),
        "r_excluded": n - len(r_samples), "r_excluded_reasons": dict(excluded),
        "r_extreme_count": len(extremes), "r_extremes": extremes,
    }, r_samples


def equity_statistics(snapshots: list[dict], started_at: datetime, as_of: datetime) -> dict:
    """Last snapshot per UTC day; forward-fill only between observed endpoints.

    No backfill, and no extension after the last snapshot. Period return uses
    the first actual snapshot, while MaxDD uses the daily close equity curve.
    """
    valid, invalid = [], 0
    for row in snapshots:
        try:
            timestamp = as_utc(row["timestamp"])
        except (ValueError, TypeError, AttributeError):
            invalid += 1
            continue
        if not started_at <= timestamp <= as_of:
            continue
        equity = finite(row["total_equity"])
        if equity is None:
            invalid += 1
        else:
            valid.append((timestamp, row.get("id", 0), equity))
    valid.sort(key=lambda row: (row[0], row[1]))
    closes = {midnight(timestamp): (timestamp, equity) for timestamp, _, equity in valid}
    curve, previous, peak, drawdowns = [], None, None, []
    filled = 0
    if closes:
        day, end = min(closes), max(closes)
        last = None
        while day <= end:
            observed = day in closes
            last = closes.get(day, last)
            timestamp, equity = last
            daily_return = finite(equity / previous - 1) if previous is not None and previous > 0 else None
            peak = equity if peak is None else max(peak, equity)
            if peak > 0:
                drawdowns.append(finite((peak - equity) / peak))
            curve.append({"date": iso(day), "equity": equity, "return": daily_return,
                          "snapshot_at": iso(timestamp), "forward_filled": not observed})
            filled += not observed
            previous = equity
            day += timedelta(days=1)
    first, last = (valid[0], valid[-1]) if valid else (None, None)
    return {
        "snapshot_count": len(valid), "snapshot_invalid": invalid,
        "first_snapshot_at": iso(first[0]) if first else None,
        "last_snapshot_at": iso(last[0]) if last else None,
        "first_equity_usdt": first[2] if first else None,
        "last_equity_usdt": last[2] if last else None,
        "period_return": finite(last[2] / first[2] - 1) if first and first[2] > 0 else None,
        "max_drawdown": max((d for d in drawdowns if d is not None), default=None),
        "forward_filled_days": filled, "daily_equity": curve,
    }


def cluster_se(influences: list[float]) -> float | None:
    """Finite-cluster corrected SE: sqrt(G/(G-1) * sum(influence_g**2))."""
    if len(influences) < 2:
        return None
    # hypot avoids overflow from directly squaring large, finite influences.
    return finite(math.hypot(*influences) * math.sqrt(len(influences) / (len(influences) - 1)))


def ci95(estimate: float | None, se: float | None):
    return [finite(estimate - 1.96 * se), finite(estimate + 1.96 * se)] if estimate is not None and se is not None else None


def paired_daily_comparison(candidate: dict, control: dict, as_of: datetime) -> dict:
    """One difference per completed UTC day: days, rather than snapshots, are clusters."""
    def returns(stats):
        return {row["date"]: row["return"] for row in stats["daily_equity"]
                if row["return"] is not None and as_utc(row["date"]) < midnight(as_of)}

    cand, ctrl = returns(candidate), returns(control)
    sequence = [{"date": day, "candidate_return": cand[day], "control_return": ctrl[day],
                 "diff": finite(cand[day] - ctrl[day])} for day in sorted(cand.keys() & ctrl.keys())]
    sequence = [row for row in sequence if row["diff"] is not None]
    diffs = [row["diff"] for row in sequence]
    estimate, n = mean(diffs), len(diffs)
    se = cluster_se([(value - estimate) / n for value in diffs]) if n >= 10 and estimate is not None else None
    c_return, b_return = candidate["period_return"], control["period_return"]
    aligned_c = finite(math.prod(1 + row["candidate_return"] for row in sequence) - 1) if n else None
    aligned_b = finite(math.prod(1 + row["control_return"] for row in sequence) - 1) if n else None
    return {"n_days": n, "mean_diff": estimate, "cluster_se": se, "ci95": ci95(estimate, se),
            "cumulative_return_diff": finite(c_return - b_return) if c_return is not None and b_return is not None else None,
            "aligned_candidate_return": aligned_c, "aligned_control_return": aligned_b,
            "aligned_cumulative_return_diff": finite(aligned_c - aligned_b) if aligned_c is not None and aligned_b is not None else None,
            "ci_note": None if n >= 10 else "完整配对日不足 10 日，仅报告均值，不报告标准误或 CI",
            "daily_diffs": sequence}


def weekly_r_comparison(candidate: list[tuple[datetime, float]],
                        control: list[tuple[datetime, float]], min_trades: int) -> dict:
    """Trade-weighted avgR difference; joint cluster by UTC Monday exit week.

    For each week, influence = sum_c(R-avg_c)/N_c - sum_b(R-avg_b)/N_b.
    Weeks with trades on only one side still contribute to the joint variance.
    """
    result = {"candidate_r_n": len(candidate), "control_r_n": len(control),
              "n_weeks": 0, "mean_diff": None, "cluster_se": None, "ci95": None,
              "weeks": [], "note": None}
    if len(candidate) < min_trades or len(control) < min_trades:
        result["note"] = f"两侧有效 R 成交均需至少 {min_trades} 笔，当前不足"
        return result
    avg_c, avg_b = mean(r for _, r in candidate), mean(r for _, r in control)
    if avg_c is None or avg_b is None:
        result["note"] = "有效 R 均值不可计算"
        return result
    buckets = defaultdict(lambda: [[], []])
    for side, samples in enumerate((candidate, control)):
        for timestamp, r in samples:
            day = midnight(timestamp)
            buckets[day - timedelta(days=day.weekday())][side].append(r)
    influences = []
    for week, (cand, ctrl) in sorted(buckets.items()):
        influence = finite_sum([(r - avg_c) / len(candidate) for r in cand] +
                               [-(r - avg_b) / len(control) for r in ctrl])
        influences.append(influence)
        result["weeks"].append({"week_start": iso(week), "candidate_n": len(cand),
                                "control_n": len(ctrl), "candidate_avg_r": mean(cand),
                                "control_avg_r": mean(ctrl), "influence": influence})
    n = len(buckets)
    estimate = finite(avg_c - avg_b)
    se = cluster_se(influences) if n >= 3 and all(i is not None for i in influences) else None
    result.update(n_weeks=n, mean_diff=estimate, cluster_se=se, ci95=ci95(estimate, se),
                  note=None if n >= 3 else "退出周不足 3 周，仅报告 avgR 差，不报告标准误或 CI")
    return result


def decide_verdict(elapsed_days: float, min_days: float, min_trades: int,
                   candidate: dict, control: dict, paired: dict) -> tuple[str, list[str]]:
    insufficient, performance = [], []
    if elapsed_days < min_days:
        insufficient.append(f"已运行 {elapsed_days:.2f} 天，少于要求的 {min_days:g} 天")
    for title, stats in (("候选", candidate), ("对照", control)):
        if stats["n"] < min_trades:
            insufficient.append(f"{title}已平仓 {stats['n']} 笔，少于要求的 {min_trades} 笔")
    pnl = finite(candidate["net_pnl_usdt"])
    if pnl is None or pnl <= 0:
        performance.append("候选净盈亏不可计算或不大于 0 USDT")
    c_return, b_return = finite(candidate["period_return"]), finite(control["period_return"])
    if c_return is None or b_return is None:
        performance.append("候选或对照缺少有效权益快照，期间收益率无法比较")
    elif c_return < b_return:
        performance.append("候选期间收益率低于对照")
    diff = finite(paired["mean_diff"])
    if diff is None or diff < 0:
        performance.append("完整日配对收益差均值不可计算或小于 0")
    verdict = "insufficient" if insufficient else "hold" if performance else "ready"
    return verdict, insufficient + performance


def normalize_pair(pair: dict) -> dict:
    pair = dict(pair)
    for key in ("id", "candidate_trader_id", "control_trader_id", "started_at"):
        if not isinstance(pair.get(key), str) or not pair[key].strip():
            raise ValueError(f"pair 缺少有效字段：{key}")
    if pair["candidate_trader_id"] == pair["control_trader_id"]:
        raise ValueError(f"{pair['id']}：候选与对照必须是不同交易员")
    if pair.get("live_trader_id") is not None and not isinstance(pair["live_trader_id"], str):
        raise ValueError(f"{pair['id']}：live_trader_id 必须是字符串或 null")
    pair["started_at"] = iso(as_utc(pair["started_at"]))
    days = finite(pair.get("min_days", 7))
    trades = finite(pair.get("min_trades", 20))
    if days is None or days < 7:
        raise ValueError(f"{pair['id']}：min_days 必须至少为 7 天")
    if trades is None or not trades.is_integer() or trades < 1:
        raise ValueError(f"{pair['id']}：min_trades 必须是正整数")
    pair.update(min_days=days, min_trades=int(trades))
    return pair


def open_readonly(db_path: Path | str) -> sqlite3.Connection:
    """Do not use immutable=1: a live database may have committed data in WAL."""
    connection = sqlite3.connect(Path(db_path).resolve().as_uri() + "?mode=ro", uri=True, timeout=30)
    connection.row_factory = sqlite3.Row
    connection.execute("PRAGMA query_only = ON")
    return connection


def read_trader(connection: sqlite3.Connection, trader_id: str,
                started_at: datetime, as_of: datetime) -> tuple[dict, list]:
    # Millisecond bounds are inclusive, without rounding rows into the window.
    start_ms = math.ceil(started_at.timestamp() * 1000)
    end_ms = math.floor(as_of.timestamp() * 1000)
    rows = [dict(row) for row in connection.execute(
        "SELECT id, symbol, exit_time, realized_pnl, fee, initial_stop_loss, entry_price, quantity "
        "FROM trader_positions WHERE trader_id = ? AND status = 'CLOSED' "
        "AND exit_time >= ? AND exit_time <= ? ORDER BY exit_time, id",
        (trader_id, start_ms, end_ms))]
    # Use the trader/time index, padding whole dates for GORM's space/T and
    # offset encodings. Exact UTC bounds are applied after parsing, above.
    lower = (started_at - timedelta(days=1)).date().isoformat()
    upper = (as_of + timedelta(days=2)).date().isoformat()
    snapshots = [dict(row) for row in connection.execute(
        "SELECT id, timestamp, total_equity FROM trader_equity_snapshots "
        "WHERE trader_id = ? AND timestamp >= ? AND timestamp < ? ORDER BY timestamp, id",
        (trader_id, lower, upper))]
    open_positions = connection.execute(
        "SELECT COUNT(*) FROM trader_positions WHERE trader_id = ? AND status = 'OPEN' AND entry_time <= ?",
        (trader_id, end_ms)).fetchone()[0]
    stats, r_samples = trade_statistics(rows)
    stats.update(equity_statistics(snapshots, started_at, as_of))
    stats.update(trader_id=trader_id, open_positions=open_positions)
    return stats, r_samples


def build_report(pairs: list[dict], db_path: Path | str, as_of: datetime) -> dict:
    """Read every configured pair in one consistent, bounded read transaction."""
    as_of = as_utc(as_of)
    pairs = [normalize_pair(pair) for pair in pairs]
    if len({pair["id"] for pair in pairs}) != len(pairs):
        raise ValueError("pair id 不可重复")
    report = {"schema_version": 1, "generated_at": iso(as_of), "pairs": [],
              "caveats": CAVEAT, "methodology": {
                  "pnl": PNL_BASIS,
                  "r": "净盈亏 / (abs(entry_price - initial_stop_loss) * quantity)；|R|>15 保留并提示",
                  "rates": "胜率、收益率、MaxDD 均为小数比例，非百分数",
                  "equity": "UTC 日末最后快照，观测区间内缺日向前填充；MaxDD 基于日末权益；期间收益率以首个实际快照为基准",
                  "daily_comparison": "完整 UTC 日收益取交集，不含当天；日为聚类，SE=sqrt(G/(G-1)*sum(influence^2))；G<10 不给 SE/CI",
                  "weekly_r": "两侧有效 R 笔数均达 min_trades 后，按退出时间的 UTC 周一聚类；交易加权 avgR 差；联合周<3 不给 SE/CI",
                  "ci": "近似正态 95% CI = 均值差 +/- 1.96*聚类 SE",
                  "open_positions": "当前 status=OPEN 的持仓数，包含 started_at 前入场的持仓",
                  "scope": "仅统计，不校验模拟盘标识或两侧配置一致性，不自动修改任何交易员配置",
              }}
    if not pairs:
        return report  # Empty configuration must work even without a database.
    with closing(open_readonly(db_path)) as connection:
        connection.execute("BEGIN")
        cache = {}
        for pair in pairs:
            start = as_utc(pair["started_at"])

            def trader(trader_id):
                key = (trader_id, start)
                if key not in cache:
                    cache[key] = read_trader(connection, trader_id, start, as_of)
                return cache[key]

            candidate, cand_r = trader(pair["candidate_trader_id"])
            control, ctrl_r = trader(pair["control_trader_id"])
            paired = paired_daily_comparison(candidate, control, as_of)
            weekly = weekly_r_comparison(cand_r, ctrl_r, pair["min_trades"])
            elapsed = max(0.0, (as_of - start).total_seconds() / 86400)
            verdict, reasons = decide_verdict(elapsed, pair["min_days"], pair["min_trades"], candidate, control, paired)
            warnings = []
            item = {**pair, "elapsed_days": elapsed, "candidate": candidate, "control": control,
                    "paired_daily": paired, "weekly_avg_r": weekly, "verdict": verdict,
                    "reasons": reasons, "caveats": CAVEAT, "warnings": warnings}
            if pair.get("live_trader_id"):
                item["live"], _ = trader(pair["live_trader_id"])
            for role in ("candidate", "control", "live"):
                if role not in item:
                    continue
                stats = item[role]
                for field, label in (("r_extreme_count", "笔 |R|>15 极端值，已保留在 avgR 中，请核查原始成交"),
                                     ("r_excluded", "笔未计入 R 统计"),
                                     ("pnl_invalid", "笔净盈亏无效，汇总净盈亏及胜率置 null"),
                                     ("snapshot_invalid", "条无效权益快照被排除"),
                                     ("forward_filled_days", "个权益缺失日向前填充，可能低估波动")):
                    if stats[field]:
                        warnings.append(f"{role}：{stats[field]} {label}")
                if not stats["snapshot_count"]:
                    warnings.append(f"{role}：期间无有效权益快照")
            report["pairs"].append(item)
    return json_safe(report)


def load_pairs(promotion_dir: Path) -> list[dict]:
    path = promotion_dir / "pairs.json"
    if not path.exists():
        return []
    config = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(config, dict) or not isinstance(config.get("pairs"), list):
        raise ValueError("pairs.json 必须是包含 pairs 数组的对象")
    if any(not isinstance(pair, dict) for pair in config["pairs"]):
        raise ValueError("pairs 中每一项必须是对象")
    return config["pairs"]


def write_report(path: Path, report: dict) -> None:
    """Write next to destination, fsync, then atomically replace; clean up on failure."""
    payload = json.dumps(json_safe(report), ensure_ascii=False, indent=2, allow_nan=False) + "\n"
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=path.parent,
                                         prefix=".report-", suffix=".tmp", delete=False) as handle:
            temporary = Path(handle.name)
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def summary(report: dict) -> str:
    def number(value, percent=False):
        return "null" if value is None else f"{value * 100:.2f}%" if percent else f"{value:.6f}"

    lines = [f"模拟盘晋升报告 {report['generated_at']} | pairs={len(report['pairs'])}"]
    for item in report["pairs"]:
        lines.append(f"[{item['id']}] {item['verdict']} | 已运行 {item['elapsed_days']:.2f} 天")
        for role, label in (("candidate", "候选"), ("control", "对照"), ("live", "正式参照")):
            if role in item:
                stats = item[role]
                lines.append(f"  {label}: n={stats['n']}, 净盈亏={number(stats['net_pnl_usdt'])} USDT, "
                             f"胜率={number(stats['win_rate'], True)}, avgR={number(stats['avg_r'])}, "
                             f"R排除={stats['r_excluded']}, 极端R={stats['r_extreme_count']}, "
                             f"MaxDD={number(stats['max_drawdown'], True)}, "
                             f"收益率={number(stats['period_return'], True)}, 开仓数={stats['open_positions']}")
        paired = item["paired_daily"]
        lines.append(f"  配对日={paired['n_days']}, 日收益差均值={number(paired['mean_diff'], True)}, "
                     f"SE={number(paired['cluster_se'], True)}, CI95={paired['ci95']}, "
                     f"累计收益差={number(paired['cumulative_return_diff'], True)}")
        weekly = item["weekly_avg_r"]
        lines.append(f"  avgR差={number(weekly['mean_diff'])}, 聚类周={weekly['n_weeks']}, "
                     f"SE={number(weekly['cluster_se'])}, CI95={weekly['ci95']}")
        for reason in item["reasons"] + item["warnings"]:
            lines.append(f"  - {reason}")
    lines.append(report["caveats"])
    return "\n".join(lines)


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args(argv)
    promotion_dir = Path(os.environ.get("PROMOTION_DIR", REPO_ROOT / "data/promotion"))
    db_path = Path(os.environ.get("NOFX_DB", REPO_ROOT / "data/data.db"))
    try:
        report = build_report(load_pairs(promotion_dir), db_path, datetime.now(UTC))
        write_report(promotion_dir / "report.json", report)
    except (OSError, ValueError, sqlite3.Error) as error:
        print(f"晋升报告失败：{error}", file=sys.stderr)
        return 1
    print(summary(report))
    print(f"报告已写入 {promotion_dir / 'report.json'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
