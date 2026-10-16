"""命令行入口: python3 -m scripts.backtest <signals|fetch|entry|exit> ..."""
import argparse
import json
import math
import os
import sys
from collections import Counter
from datetime import datetime, timezone

from . import entry, exitsim, fetch, mldataset, params, signals
from .backtrader_adapter import BacktraderUnavailable, _backtrader
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


def _add_execution(p, with_limit=False):
    p.add_argument("--engine", choices=["backtrader"], default="backtrader", help="使用 Backtrader 原生 broker")
    p.add_argument("--commission", type=float, default=exitsim.TAKER, help="每次成交手续费率，0.0005=0.05%%")
    p.add_argument("--slippage", type=float, default=exitsim.SLIP, help="市价/止损滑点率，0.0003=0.03%%")
    if with_limit:
        p.add_argument("--limit-valid-min", type=float, default=45, help="从信号时间起算的限价有效期(分钟)")


def _execution(a):
    bt = _backtrader()
    if not all(math.isfinite(v) and v >= 0 for v in (a.commission, a.slippage)):
        raise ValueError("手续费和滑点必须是非负有限数")
    if hasattr(a, "limit_valid_min") and (not math.isfinite(a.limit_valid_min) or a.limit_valid_min <= 0):
        raise ValueError("限价有效期必须大于 0")
    print(f"engine=backtrader version={bt.__version__} commission={a.commission} slippage={a.slippage}")
    print("收盘提交订单，下一根起撮合；独立信号 R 研究，非组合资金回测；open 仅盯市、不收未成交的平仓费。")


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
    _execution(a)
    _, sigs = signals.load_signals_file(_sig_path(a))
    horizons = [int(h) for h in a.horizons.split(",")]
    if not horizons or any(h <= 0 for h in horizons):
        raise ValueError("horizons 必须是正整数小时列表")
    res, skipped = entry.build_rows(sigs, a.cache, horizons, allow_overlap=a.allow_overlap,
                                    commission=a.commission, slippage=a.slippage, limit_valid_min=a.limit_valid_min)
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
    _execution(a)
    meta, sigs = signals.load_signals_file(_sig_path(a))
    base = _baseline(a, meta)
    base["horizon_h"] = a.horizon
    print("baseline params:", params.describe(base))
    trades, skipped = exitsim.load_trades(sigs, a.cache, a.set, include_market=not a.exclude_market,
                                         limit_valid_min=a.limit_valid_min)
    if not trades:
        print("no tradable signals", skipped)
        return 1
    print(f"set={a.set} horizon={a.horizon}h trades={len(trades)} skipped={skipped} "
          f"avg stop {sum(t['risk_pct'] for t in trades) / len(trades):.2f}%")
    variants = [["baseline", {}]]
    if a.variants:
        with open(a.variants) as f:
            variants += json.load(f)
    exitsim.run_variants(trades, base, variants, csv_path=a.csv, allow_overlap=a.allow_overlap,
                         commission=a.commission, slippage=a.slippage)
    return 0


def cmd_ml_dataset(a):
    _execution(a)
    meta, sigs = signals.load_signals_file(_sig_path(a))
    base = _baseline(a, meta)
    base["horizon_h"] = a.horizon
    db = a.db or meta.get("db") or os.path.join(REPO_ROOT, "data", "data.db")
    conn = signals.connect_ro(db)
    if a.trader or a.trader_id:
        tid = signals.resolve_trader(conn, a.trader, a.trader_id)["id"]
    else:
        tid = meta["trader_id"]
    rows, missing = mldataset.build_rows(conn, tid, sigs, a.cache, base, a.horizon, a.commission, a.slippage)
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
    if any(row.get("engine") != "backtrader" for row in rows):
        raise ValueError("ml_rows 含旧模拟器或未知来源的标签，请先用 Backtrader 重新运行 ml-dataset")
    labels = mleval.LABELS if a.label == "both" else (a.label,)
    names = a.models.split(",") if a.models else None
    results, line = mleval.run(rows, labels, min_coverage=a.min_coverage, folds=a.folds, min_train=a.min_train,
                               perms=a.perms, seed=a.seed, models=names, alpha=a.alpha)
    return 0


def cmd_stocks(a):
    # Lazy imports keep signals/fetch/help usable without portfolio dependencies.
    _backtrader()
    try:
        from .stocks import Config, run_study
        from .stocks_data import load_study
        from .stocks_report import cross_check, render_report
    except ImportError as exc:
        raise ValueError('stocks needs pandas; install scripts/backtest/requirements.txt') from exc
    if a.out and a.json_out and os.path.abspath(a.out) == os.path.abspath(a.json_out):
        raise ValueError('out and json must be different paths')
    cfg = Config(a.tf, a.maxpos, a.rebalance, a.commission, a.cash)
    cfg.validate()
    names = tuple(n.strip() for n in a.strategies.split(',') if n.strip())
    study = load_study(a.cache, a.tf, a.exclude)
    result = run_study(study, cfg, names, progress=lambda line: print(line, flush=True))
    if a.compare:
        path = a.compare
        if not os.path.exists(path):
            path = os.path.join(a.cache, path)
        with open(path) as f:
            result['cross_check'] = cross_check(result, json.load(f))
        result['compare_path'] = path
    report = render_report(result)
    for path, content in ((a.out, report), (a.json_out, json.dumps(result, indent=2, ensure_ascii=False, allow_nan=False)+'\n')):
        if path:
            os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
            with open(path, 'w') as f:
                f.write(content)
            print('written '+path)
    print(f"engine=backtrader version={result['engine_version']} tf={a.tf} stocks={study.coverage['universe_size']} "
          f"bars={len(study.calendar)} commission={a.commission} slippage=0")
    if a.out:
        if a.compare:
            from .stocks_report import comparison_table
            print(comparison_table(result['cross_check']))
    else:
        print(report)
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
    _add_execution(p, with_limit=True)
    p.add_argument("--horizons", default="8,24", help="小时,逗号分隔")
    p.add_argument("--allow-overlap", action="store_true", help="市价/限价行不做「同币同向一次只持一仓」过滤")
    p.set_defaults(fn=cmd_entry)

    p = sub.add_parser("exit", help="Backtrader 出场策略回测")
    _add_common(p, with_db=True)
    _add_execution(p, with_limit=True)
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
    _add_execution(p)
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

    p = sub.add_parser('stocks', help='离线美股组合回测（Backtrader）')
    p.add_argument('--cache', default=os.path.join(REPO_ROOT, 'data', 'backtest_cache', 'stocks'))
    p.add_argument('--tf', choices=['1d', '1h'], default='1d')
    p.add_argument('--maxpos', type=int, default=10)
    p.add_argument('--rebalance', type=int, default=10)
    p.add_argument('--commission', type=float, default=0.001)
    p.add_argument('--cash', type=float, default=100000)
    p.add_argument('--exclude', default='TQQQBUSDT,SQQQBUSDT,SOXLBUSDT,SOXSBUSDT,UVXYBUSDT,MUUBUSDT,KORUBUSDT',
                   help='逗号分隔，替换默认排除项；SPY/QQQ 始终仅作基准')
    p.add_argument('--strategies', default='MOM_TOP,MA_CROSS,MA2060_RULES,MOM_X_MA,SPY_BH,EW_BH')
    p.add_argument('--compare', help='Go 结果 JSON 路径（或 cache 下文件名）')
    p.add_argument('--out', help='Markdown 输出，默认 stdout')
    p.add_argument('--json', dest='json_out', help='JSON 输出，含原生分析器、权益、订单与交易')
    p.set_defaults(fn=cmd_stocks)
    return ap


def main(argv=None):
    a = build_parser().parse_args(argv)
    try:
        return a.fn(a) or 0
    except (BacktraderUnavailable, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
