"""ICT 研究回测: 把 ict.find_setups 的 Setup 转成 Backtrader 交易(run_trade),并行运行,统一汇总。

撮合/账务/手续费全部由 backtrader_adapter.run_trade(Backtrader)完成,这里只做数据适配与统计。
"""
import math
import multiprocessing as mp
import random

from .backtrader_adapter import run_trade
from .params import FALLBACK

M5 = 300_000
DATA = {}  # sym -> ict.load_arrays 结果(fork 后子进程继承)


def plain_rules(horizon_h=72, **kw):
    """只有初始 SL 与 TP,无保本/减仓/追踪/峰值回撤。"""
    return dict(FALLBACK, be_arm=0, lock_r=0, lock_trim=False, trim_r=0, trail=False, pd_arm=0,
                tp_cap=0, quick_stop_h=0, horizon_h=horizon_h, **kw)


def build_tr(job, horizon_h=72):
    """job: sym, side, idx(决策K线), kind(mkt|lim), ref, sl, tp, atr_pct, valid_bars。"""
    a = DATA[job["sym"]]
    i = job["idx"]
    n = int(horizon_h * 12) + 30
    j = min(len(a["o"]), i + n)
    if j - i < 3:
        return None
    t_close = int(a["t_close"][i])
    bars = [(int(a["t_close"][k]), float(a["o"][k]), float(a["h"][k]), float(a["l"][k]),
             float(a["c"][k]), int(a["t_open"][k])) for k in range(i, j)]
    ref, sl, tp = job["ref"], job["sl"], job["tp"]
    if any(v is None or not math.isfinite(v) or v <= 0 for v in (ref, sl, tp)):
        return None
    ok = (sl < ref < tp) if job["side"] == "long" else (tp < ref < sl)
    if not ok:
        return None
    return dict(sym=job["sym"], side=job["side"], t0=t_close, t0_ms=t_close, start_ms=t_close,
                bars=bars, reference=ref, sl=sl, tp=tp, kind=job["kind"],
                limit_valid_min=job.get("valid_bars", 24) * 5, mode="trend",
                atr_pct=job.get("atr_pct") or 0.0, risk_pct=abs(ref - sl) / ref * 100)


def _work(args):
    job, rules, horizon_h, comm, slip = args
    tr = build_tr(job, horizon_h)
    if tr is None:
        return dict(r=None, how="invalid", exit_ms=None)
    res = run_trade(tr, rules, comm, slip)
    return dict(r=res["r"], how=res["how"], exit_ms=res["exit_ms"], entry_ms=res["entry_ms"],
                peak=res["peak"], risk_pct=tr["risk_pct"])


def run_jobs(jobs, rules, horizon_h=72, comm=0.0005, slip=0.0003, procs=9):
    args = [(j, rules, horizon_h, comm, slip) for j in jobs]
    with mp.get_context("fork").Pool(procs, maxtasksperchild=200) as pool:
        return pool.map(_work, args, chunksize=4)


def overlap_filter(jobs, results):
    """同 (币,方向) 同时只持有一个仓位;按决策时间顺序,仓位未平不再接受新信号。"""
    busy, kept = {}, []
    order = sorted(range(len(jobs)), key=lambda k: jobs[k]["t_ms"])
    blocked = 0
    for k in order:
        r = results[k]
        if r["r"] is None:
            continue
        key = (jobs[k]["sym"], jobs[k]["side"])
        if jobs[k]["t_ms"] < busy.get(key, 0):
            blocked += 1
            continue
        busy[key] = r["exit_ms"] or jobs[k]["t_ms"]
        kept.append(k)
    return kept, blocked


def stats(rs):
    n = len(rs)
    if n == 0:
        return dict(n=0, sum=0.0, avg=0.0, win=0.0, pf=0.0, dd=0.0, se=0.0, payoff=0.0)
    w = [r for r in rs if r > 0]
    l = [r for r in rs if r <= 0]
    avg = sum(rs) / n
    sd = (sum((r - avg) ** 2 for r in rs) / max(1, n - 1)) ** 0.5
    eq = pk = dd = 0.0
    for r in rs:
        eq += r
        pk = max(pk, eq)
        dd = max(dd, pk - eq)
    pf = sum(w) / -sum(l) if l and sum(l) < 0 else float("inf")
    payoff = (sum(w) / len(w)) / (-sum(l) / len(l)) if w and l and sum(l) < 0 else 0.0
    return dict(n=n, sum=sum(rs), avg=avg, win=len(w) / n, pf=pf, dd=dd, se=sd / n ** 0.5, payoff=payoff)


def fmt(name, s, width=34, extra=""):
    return (f"{name:{width}s} n={s['n']:5d} avgR={s['avg']:+.3f}±{s['se']:.3f} sumR={s['sum']:+8.1f} "
            f"win={s['win']:.0%} payoff={s['payoff']:.2f} PF={s['pf']:.2f} maxDD_R={s['dd']:6.1f} {extra}")


def summarize(jobs, results, allow_overlap=False):
    if allow_overlap:
        kept = [k for k, r in enumerate(results) if r["r"] is not None]
        blocked = 0
    else:
        kept, blocked = overlap_filter(jobs, results)
    kept.sort(key=lambda k: jobs[k]["t_ms"])
    rs = [results[k]["r"] for k in kept]
    return stats(rs), kept, blocked
