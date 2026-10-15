"""回测数据和报告适配；订单撮合与盈亏委托 Backtrader。"""
import csv
import json
import math

from .backtrader_adapter import _backtrader, run_trade
from .klines import floor_min, load_bars

TAKER, SLIP = 0.0005, 0.0003


def build_trade(sig, bars, kind, n1m=None, limit_valid_min=45):
    """缓存 OHLC → Backtrader feed；限价使用信号当时的价格和 SL/TP。"""
    if kind not in ("mkt", "lim") or sig["side"] not in ("long", "short"):
        return None
    if kind == "lim" and sig.get("kind") != "limit":
        return None
    t0 = floor_min(sig["t0_ms"])
    signal_bar = next((b for b in bars if b[0] == t0), None)
    if signal_bar is None:
        return None
    reference = signal_bar[4] if kind == "mkt" else sig.get("px0")
    sl, tp = sig.get("sl0"), sig.get("tp0")
    if any(v is None or not math.isfinite(v) or v <= 0 for v in (reference, sl, tp)):
        return None
    if not ((sl < reference < tp) if sig["side"] == "long" else (tp < reference < sl)):
        return None
    feed = []
    for i, (t, o, h, l, c) in enumerate(bars):
        if not all(math.isfinite(v) and v > 0 for v in (o, h, l, c)) or not l <= min(o, c) <= max(o, c) <= h:
            return None
        duration = 60_000 if n1m is None or i < n1m else 300_000
        if t < t0:
            continue
        feed.append((t + duration, o, h, l, c, t))
    if len(feed) < 2 or any(a[0] >= b[0] or a[0] > b[5] for a, b in zip(feed, feed[1:])):
        return None
    return dict(sym=sig["symbol"], side=sig["side"], t0=sig["t0"], t0_ms=sig["t0_ms"],
                start_ms=t0 + 60_000, bars=feed, reference=reference, sl=sl, tp=tp, kind=kind,
                limit_valid_min=limit_valid_min, mode=sig.get("exit_mode") or "trend",
                atr_pct=sig.get("atr1h") or 0, risk_pct=abs(reference - sl) / reference * 100)


def load_trades(signals, cache, kind, min_bars=10, include_market=True, limit_valid_min=45):
    trades, skipped = [], {"nodata": 0, "short": 0, "invalid": 0}
    for sig in signals:
        if kind == "lim" and sig["kind"] != "limit":
            continue
        if sig["kind"] == "market" and not include_market:
            continue
        got = load_bars(cache, sig["symbol"], sig["t0_ms"])
        if got is None:
            skipped["nodata"] += 1
            continue
        bars, n1m = got
        if n1m < min_bars:
            skipped["short"] += 1
            continue
        tr = build_trade(sig, bars, kind, n1m, limit_valid_min)
        if tr is None:
            skipped["invalid"] += 1
            continue
        trades.append(tr)
    return trades, skipped


def simulate(tr, p, commission=TAKER, slippage=SLIP):
    """兼容研究标签接口。实际运行 bt.Cerebro，无内置撮合后备路径。"""
    result = run_trade(tr, p, commission, slippage)
    return result["r"], result["how"], result["peak"], result["exit_ms"]


def summarize(trades, p, allow_overlap=False, commission=TAKER, slippage=SLIP):
    """汇总独立信号的 R，不是组合账户权益。持仓时间取 Backtrader 结果。"""
    rs, hows, blocked, unfilled, busy = [], {}, 0, 0, {}
    for tr in sorted(trades, key=lambda x: (x["start_ms"], x["t0_ms"])):
        key = (tr["sym"], tr["side"])
        if not allow_overlap and tr["start_ms"] < busy.get(key, 0):
            blocked += 1
            continue
        r, how, _, exit_t = simulate(tr, p, commission, slippage)
        if r is None:
            unfilled += 1
            continue
        busy[key] = exit_t
        rs.append((tr["t0_ms"], r))
        hows[how] = hows.get(how, 0) + 1
    rs.sort()
    v = [r for _, r in rs]
    eq = pk = dd = 0.0
    for r in v:
        eq += r
        pk = max(pk, eq)
        dd = max(dd, pk - eq)
    h = len(v) // 2
    ordered = sorted(v)
    top = ordered[:-5] if len(v) > 5 else ordered
    return dict(n=len(v), sum=sum(v), avg=sum(v) / len(v) if v else 0.0,
                win=sum(r > 0 for r in v) / len(v) if v else 0.0, dd=dd,
                h1=sum(v[:h]), h2=sum(v[h:]), drop5=sum(top) / len(top) if top else 0.0,
                open=hows.get("open", 0), hows=hows, blocked=blocked, unfilled=unfilled)


def format_row(name, s, width=34):
    hows = " ".join(f"{k}={v}" for k, v in sorted(s["hows"].items()))
    return (f"{name:{width}s} n={s['n']} sumR={s['sum']:+7.1f} avgR={s['avg']:+.3f} win={s['win']:.0%} "
            f"maxDD_R={s['dd']:5.1f} H1={s['h1']:+6.1f} H2={s['h2']:+6.1f} drop5avg={s['drop5']:+.3f} "
            f"blocked={s['blocked']} unfilled={s['unfilled']} open={s['open']} [{hows}]")


def run_variants(trades, base, variants, csv_path=None, out=print, allow_overlap=False, commission=TAKER, slippage=SLIP):
    rows = []
    configs = {}
    width = max([len(n) for n, _ in variants] + [8])
    for name, over in variants:
        p = dict(base)
        unknown = set(over) - set(base)
        if unknown:
            raise ValueError(f"未知变体参数: {', '.join(sorted(unknown))}")
        p.update(over)
        configs[name] = p
        s = summarize(trades, p, allow_overlap, commission, slippage)
        out(format_row(name, s, width))
        rows.append((name, s))
    if csv_path:
        with open(csv_path, "w", newline="") as f:
            w = csv.writer(f)
            w.writerow(["name", "n", "sumR", "avgR", "win", "maxDD_R", "H1", "H2", "drop5avg", "blocked", "unfilled", "open", "exit_types",
                        "engine", "engine_version", "commission", "slippage", "params"])
            for name, s in rows:
                w.writerow([name, s["n"], f"{s['sum']:.3f}", f"{s['avg']:.4f}", f"{s['win']:.4f}", f"{s['dd']:.3f}",
                            f"{s['h1']:.3f}", f"{s['h2']:.3f}", f"{s['drop5']:.4f}", s["blocked"], s["unfilled"], s["open"], json.dumps(s["hows"]),
                            "backtrader", _backtrader().__version__, commission, slippage, json.dumps(configs[name])])
    return rows
