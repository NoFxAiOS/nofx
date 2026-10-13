"""命令行入口: python3 -m scripts.backtest <signals|fetch|entry|exit> ..."""
import argparse
import json
import os
import sys
from collections import Counter
from datetime import datetime, timezone

from . import entry, exitsim, fetch, mldataset, params, signals
from .klines import default_cache_dir

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def _add_common(p, with_db=False):
    p.add_argument("--cache", default=default_cache_dir(REPO_ROOT), help="K 线缓存目录(含 kl1m/ kl5m/)")
    p.add_argument("--signals", dest="signals_file", default=None, help="信号 JSON(默认 <cache>/signals.json)")
    if with_db:
        p.add_argument("--db", default=None, help="sqlite 路径(默认取信号文件 meta,再默认 <repo>/data/data.db)")
        p.add_argument("--trader", default=None)
        p.add_argument("--trader-id", default=None)


def _sig_path(a):
    return a.signals_file or os.path.join(a.cache, "signals.json")


def cmd_signals(a):
    db = a.db or os.path.join(REPO_ROOT, "data", "data.db")
    conn = signals.connect_ro(db)
    tr = signals.resolve_trader(conn, a.trader, a.trader_id)
    since = signals.parse_utc(a.since) if a.since else None
    until = signals.parse_utc(a.until) if a.until else None
    sigs, n_raw = signals.extract(conn, tr["id"], since, until, a.dedupe_min, a.max_fill_dev_pct)
    out = a.out or _sig_path(a)
    os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
    meta = dict(db=os.path.abspath(db), trader=tr["name"], trader_id=tr["id"], strategy_id=tr["strategy_id"],
                since=a.since, until=a.until, dedupe_min=a.dedupe_min, max_fill_dev_pct=a.max_fill_dev_pct, raw_decisions=n_raw,
                generated=datetime.now(timezone.utc).isoformat())
    with open(out, "w") as f:
        json.dump(dict(meta=meta, signals=sigs), f, ensure_ascii=False)
    print(f"trader {tr['name']} ({tr['id']})  raw decisions {n_raw} -> signals {len(sigs)}  written {out}")
    print("by outcome:", dict(Counter(s["outcome"] for s in sigs)))
    print("by kind/side:", dict(Counter(f"{s['kind']}/{s['side']}" for s in sigs)))
    if sigs:
        print("placements per limit signal avg: %.2f" % (
            sum(s["n"] for s in sigs if s["kind"] == "limit") / max(1, sum(1 for s in sigs if s["kind"] == "limit"))))


def cmd_fetch(a):
    _, sigs = signals.load_signals_file(_sig_path(a))
    r = fetch.fetch_all(sigs, a.cache, sleep=a.sleep, dry_run=a.dry_run)
    label = "需补抓" if a.dry_run else "已抓取"
    print(f"signals(去重后 symbol+分钟) {r['signals']}  {label}: 1m={r['fetched_1m']} 5m={r['fetched_5m']}  "
          f"requests={r['requests']}  failed={r['failed']}")
    for f in r["failed_list"]:
        print("  FAILED", f)


def cmd_entry(a):
    _, sigs = signals.load_signals_file(_sig_path(a))
    horizons = [int(h) for h in a.horizons.split(",")]
    res, skipped = entry.build_rows(sigs, a.cache, horizons, allow_overlap=a.allow_overlap)
    entry.report(res, skipped)


def _baseline(a, meta):
    if not a.from_strategy:
        p = dict(params.FALLBACK)
        p["horizon_h"] = a.horizon
        return p
    db = a.db or meta.get("db") or os.path.join(REPO_ROOT, "data", "data.db")
    conn = signals.connect_ro(db)
    if a.trader or a.trader_id:
        tr = signals.resolve_trader(conn, a.trader, a.trader_id)
    else:
        tr = signals.resolve_trader(conn, trader_id=meta.get("trader_id"))
    rc = signals.load_risk_control(conn, tr["strategy_id"])
    p, warns = params.derive_baseline(rc, a.horizon)
    print(f"baseline from strategy of trader {tr['name']} ({tr['strategy_id']})")
    for w in warns:
        print("WARNING:", w)
    return p


def cmd_exit(a):
    meta, sigs = signals.load_signals_file(_sig_path(a))
    base = _baseline(a, meta)
    base["horizon_h"] = a.horizon
    print("baseline params:", params.describe(base))
    trades, skipped = exitsim.load_trades(sigs, a.cache, a.set, include_market=not a.exclude_market)
    if not trades:
        print("no tradable signals", skipped)
        return 1
    print(f"set={a.set} horizon={a.horizon}h trades={len(trades)} skipped={skipped} "
          f"avg stop {sum(t['risk_pct'] for t in trades) / len(trades):.2f}%")
    variants = [["baseline", {}]]
    if a.variants:
        with open(a.variants) as f:
            variants += json.load(f)
    exitsim.run_variants(trades, base, variants, csv_path=a.csv, allow_overlap=a.allow_overlap)
    return 0


def cmd_ml_dataset(a):
    meta, sigs = signals.load_signals_file(_sig_path(a))
    base = _baseline(a, meta)
    base["horizon_h"] = a.horizon
    db = a.db or meta.get("db") or os.path.join(REPO_ROOT, "data", "data.db")
    conn = signals.connect_ro(db)
    if a.trader or a.trader_id:
        tid = signals.resolve_trader(conn, a.trader, a.trader_id)["id"]
    else:
        tid = meta["trader_id"]
    rows, missing = mldataset.build_rows(conn, tid, sigs, a.cache, base, a.horizon)
    out = a.out or os.path.join(a.cache, "ml_rows.json")
    with open(out, "w") as f:
        json.dump(rows, f)
    if a.csv:
        mldataset.write_csv(rows, a.csv)
    print(f"rows {len(rows)}  missing {missing}  written {out}")
    cov = mldataset.coverage(rows)
    low = {k: round(v, 2) for k, v in cov.items() if v < a.min_coverage}
    print(f"features {len(cov)}  (coverage >= {a.min_coverage:.0%}: {len(cov) - len(low)})")
    print(f"coverage below {a.min_coverage:.0%}: {low}")
    return 0


ML_EVAL_HINT = ("ml-eval 需要 numpy 和 scikit-learn,当前环境缺少。可以这样运行:\n"
                "  uv run --no-project --with numpy --with scikit-learn python -m scripts.backtest ml-eval ...")


def cmd_ml_eval(a):
    try:
        from . import mleval
    except ImportError:
        print(ML_EVAL_HINT, file=sys.stderr)
        return 2
    path = a.rows or os.path.join(a.cache, "ml_rows.json")
    with open(path) as f:
        rows = json.load(f)
    labels = mleval.LABELS if a.label == "both" else (a.label,)
    names = a.models.split(",") if a.models else None
    results, line = mleval.run(rows, labels, min_coverage=a.min_coverage, folds=a.folds, min_train=a.min_train,
                               perms=a.perms, seed=a.seed, models=names, alpha=a.alpha)
    return 0


def build_parser():
    ap = argparse.ArgumentParser(prog="python3 -m scripts.backtest", description=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("signals", help="从 decision_records 提取入场信号")
    _add_common(p, with_db=True)
    p.add_argument("--since", help="UTC ISO 日期/时间,如 2026-09-12")
    p.add_argument("--until", help="UTC ISO 日期/时间,如 2026-10-12T16:00(不含)")
    p.add_argument("--dedupe-min", type=int, default=60)
    p.add_argument("--max-fill-dev-pct", type=float, default=0.5,
                   help="成交价需在该信号任一挂单价的 N%% 以内才算成交(排除手动单);0=不检查")
    p.add_argument("--out", help="输出 JSON(默认 <cache>/signals.json)")
    p.set_defaults(fn=cmd_signals)

    p = sub.add_parser("fetch", help="缓存币安合约 1m/5m K 线")
    _add_common(p)
    p.add_argument("--sleep", type=float, default=1.0, help="请求间隔秒数(>=1)")
    p.add_argument("--dry-run", action="store_true", help="只统计需要补抓的数量")
    p.set_defaults(fn=cmd_fetch)

    p = sub.add_parser("entry", help="限价 vs 市价入场对比")
    _add_common(p)
    p.add_argument("--horizons", default="8,24", help="小时,逗号分隔")
    p.add_argument("--allow-overlap", action="store_true", help="市价行不做「同币同向一次只持一仓」过滤")
    p.set_defaults(fn=cmd_entry)

    p = sub.add_parser("exit", help="出场规则模拟")
    _add_common(p, with_db=True)
    p.add_argument("--set", choices=["mkt", "lim"], default="mkt")
    p.add_argument("--horizon", type=float, default=72, help="持仓上限(小时)")
    p.add_argument("--variants", help="变体 JSON: [[name, {overrides}], ...]")
    p.add_argument("--from-strategy", action=argparse.BooleanOptionalAction, default=True,
                   help="基线参数取自线上策略 risk_control(默认开)")
    p.add_argument("--exclude-market", action="store_true", help="--set mkt 时排除 AI 原生市价开仓信号")
    p.add_argument("--allow-overlap", action="store_true",
                   help="关闭「同币同向一次只持一仓」约束(默认开:持仓期间的同币同向新信号被跳过)")
    p.add_argument("--csv", help="汇总结果写 CSV")
    p.set_defaults(fn=cmd_exit)

    p = sub.add_parser("ml-dataset", help="信号 → 特征 + 标签(r_base / r_plain / mfe)")
    _add_common(p, with_db=True)
    p.add_argument("--horizon", type=float, default=72)
    p.add_argument("--from-strategy", action=argparse.BooleanOptionalAction, default=True)
    p.add_argument("--out", help="输出 JSON(默认 <cache>/ml_rows.json)")
    p.add_argument("--csv", help="同时写 CSV")
    p.add_argument("--min-coverage", type=float, default=0.8, help="低于该覆盖率的特征会被列出")
    p.set_defaults(fn=cmd_ml_dataset)

    p = sub.add_parser("ml-eval", help="信号过滤器可行性检验(需要 numpy + scikit-learn)")
    p.add_argument("--cache", default=default_cache_dir(REPO_ROOT))
    p.add_argument("--rows", help="ml-dataset 的输出(默认 <cache>/ml_rows.json)")
    p.add_argument("--label", choices=["r_base", "r_plain", "both"], default="both")
    p.add_argument("--min-coverage", type=float, default=0.8)
    p.add_argument("--folds", type=int, default=5)
    p.add_argument("--min-train", type=float, default=0.4)
    p.add_argument("--perms", type=int, default=400, help="置换次数;Bonferroni 后要能通过需 >= 1/阈值")
    p.add_argument("--seed", type=int, default=7)
    p.add_argument("--alpha", type=float, default=0.05)
    p.add_argument("--models", help="逗号分隔,默认 logit_L2,logit_L1,ridge,hgb_depth2")
    p.set_defaults(fn=cmd_ml_eval)
    return ap


def main(argv=None):
    a = build_parser().parse_args(argv)
    return a.fn(a) or 0


if __name__ == "__main__":
    sys.exit(main())
