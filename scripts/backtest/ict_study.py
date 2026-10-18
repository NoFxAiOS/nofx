"""ICT vs 现行规则的 Backtrader 对比研究。

  python -m scripts.backtest.ict_study b  --cache <cache> --out <dir>   # 独立 ICT 入场(多币种5m) + 时间错位对照
  python -m scripts.backtest.ict_study a  --cache <cache> --out <dir>   # 叠加到 AI 开仓信号上的 ICT 过滤/止损止盈
"""
import argparse
import json
import os
import random
import sys
import datetime as dt

from . import ict, ict_bt, ict_fetch
from .backtrader_adapter import run_trade
from .ict_bt import DATA, plain_rules, run_jobs, summarize, fmt
from .params import derive_baseline
from .signals import connect_ro, load_risk_control, resolve_trader

UNIVERSE = ("BTC ETH SOL XRP DOGE BNB NEAR TIA HYPE WLD SUI FIL INJ STRK ENA UNI ZEC ADA LINK AVAX LTC "
            "ARB OP AAVE APT DOT BCH TRX TON ATOM").split()
TRIGGERS = ("sweep", "cisd", "mss", "fvg", "full")
PATHS = ("any", "erl_swept", "smt", "p1", "p2", "p3")


def nofx_rules(db):
    conn = connect_ro(db)
    tr = resolve_trader(conn, name="mac-nofx")
    p, _ = derive_baseline(load_risk_control(conn, tr["strategy_id"]), 72)
    p["force_mode"] = "trend"
    return p


def tp_for(s, key):
    risk = abs(s.ref - s.sl)
    sign = 1 if s.side == "long" else -1
    if key == "2r":
        return s.ref + sign * 2 * risk
    path = {"erl": s.tp_erl, "p1": s.tp_p1, "p2": s.tp_p2, "p3": s.tp_p3}[key]
    if path is None or sign * (path - s.ref) < 1.2 * risk:  # 目标不足 1.2R 视为无交易空间
        return None
    return path


def make_job(sym, s, tpkey, floor_atr=0.0):
    sl = s.sl
    if floor_atr > 0 and s.atr1h_pct:  # 止损距离至少 floor_atr × 1h ATR(与现行 sl_min_atr_mult 同口径)
        sign = 1 if s.side == "long" else -1
        sl = min(sl, s.ref - floor_atr * s.atr1h_pct / 100 * s.ref) if sign > 0 else max(sl, s.ref + floor_atr * s.atr1h_pct / 100 * s.ref)
        s = type(s)(**dict(s.__dict__, sl=sl))
    tp = tp_for(s, tpkey)
    if tp is None:
        return None
    return dict(sym=sym, side=s.side, idx=s.idx, kind=s.kind, ref=s.ref, sl=s.sl, tp=tp,
                atr_pct=s.atr1h_pct, valid_bars=24, t_ms=s.t_close_ms, trig=s.trigger,
                tpkey=tpkey, key=(sym, s.side, s.trigger, s.idx, tpkey))


def control_of(job, a, rng, lo=72, hi=576):
    """同币同向、同百分比止损/止盈几何、随机错位 6~48h 的对照入场。"""
    n = len(a["c"])
    for _ in range(20):
        k = rng.choice([-1, 1]) * rng.randint(lo, hi)
        i = job["idx"] + k
        if 300 < i < n - 900:
            break
    else:
        return None
    sign = 1 if job["side"] == "long" else -1
    sl_p = abs(job["ref"] - job["sl"]) / job["ref"]
    tp_p = abs(job["tp"] - job["ref"]) / job["ref"]
    c = float(a["c"][i])
    ref = c
    kind = job["kind"]
    if kind == "lim":  # 保持"回撤到价位才成交"的选择偏差一致
        ref = c * (1 - sign * job["pull"])
    return dict(job, idx=i, ref=ref, sl=ref * (1 - sign * sl_p), tp=ref * (1 + sign * tp_p),
                t_ms=int(a["t_close"][i]), control=True, key=job["key"] + ("ctl",))


def load_universe(cache, syms, start_ms=None, end_ms=None, equity=False):
    ctxs = {}
    ref_eth = ict_fetch.load(cache, "SPYUSDT" if equity else "ETHUSDT")
    ref_btc = ict_fetch.load(cache, "QQQUSDT" if equity else "BTCUSDT")
    for s in syms:
        bars = ict_fetch.load(cache, s)
        if len(bars) < 3000:
            print("skip(no data)", s)
            continue
        ref = ref_eth if s in ("BTCUSDT", "QQQUSDT") else ref_btc
        ctxs[s] = ict.Ctx(bars, ref)
        DATA[s] = ctxs[s].a5
    return ctxs


def cmd_b(a):
    cache = a.cache
    if a.universe:
        syms = json.load(open(a.universe_json))[a.universe]
        syms = [x for x in syms if len(ict_fetch.load(cache, x)) > 3000]
    else:
        syms = [u + "USDT" for u in UNIVERSE if len(ict_fetch.load(cache, u + "USDT")) > 3000]
    print("universe", len(syms), syms)
    ctxs = load_universe(cache, syms, equity=a.universe == "equity")
    setups = []
    for sym, ctx in ctxs.items():
        for s in ict.find_setups(ctx, {}):
            s.sym = sym
            setups.append(s)
    print("setups", len(setups))
    import collections
    print(collections.Counter((s.trigger, s.side) for s in setups))
    # 去掉最早 2 天预热
    exits = {"floor_2r": (plain_rules(), "2r", 1.0, 0.0005, 0.0003),
             "gross_2r": (plain_rules(), "2r", 0.0, 0.0, 0.0),
             "floor_gross_2r": (plain_rules(), "2r", 1.0, 0.0, 0.0),
             "floor_erl": (plain_rules(), "erl", 1.0, 0.0005, 0.0003),
             "floor_nofx_2r": (nofx_rules(a.db), "2r", 1.0, 0.0005, 0.0003)}
    if a.only:
        exits = {k: v for k, v in exits.items() if k in a.only.split(",")}
    rng = random.Random(7)
    out = {}
    for ename, (rules, tpkey, floor, comm, slip) in exits.items():
        jobs = []
        for s in setups:
            keys = [tpkey] if tpkey != "erl" else ["erl", "p1", "p2", "p3"]
            for k in keys:
                j = make_job(s.sym, s, k, floor)
                if j is None:
                    continue
                if tpkey == "erl" and k in ("p1", "p2", "p3") and not s.flags.get(k):
                    continue
                close = float(DATA[s.sym]["c"][s.idx])
                j["pull"] = abs(close - s.ref) / close if s.kind == "lim" else 0.0
                j["flags"] = dict(s.flags, ny=s.ny_hour)
                jobs.append(j)
        ctl = [c for c in (control_of(j, DATA[j["sym"]], rng) for j in jobs) if c]
        print(ename, "jobs", len(jobs), "controls", len(ctl), flush=True)
        res = run_jobs(jobs, rules, comm=comm, slip=slip, procs=a.procs)
        cres = run_jobs(ctl, rules, comm=comm, slip=slip, procs=a.procs)
        out[ename] = dict(jobs=jobs, res=res, ctl=ctl, cres=cres)
        with open(os.path.join(a.out, f"b_{ename}.json"), "w") as f:
            json.dump(dict(jobs=jobs, res=res, ctl=ctl, cres=cres), f, default=str)
    report_b(out, a)


def report_b(out, a):
    lines = []
    emit = lambda s: (print(s), lines.append(s))
    for ename, d in out.items():
        emit(f"\n===== 出场 {ename} (Backtrader, 费 0.05%/边 + 滑点 0.03%, 同(币,方向)单仓) =====")
        for trig in TRIGGERS:
            for path in PATHS:
                if "erl" not in ename and path in ("p1", "p2", "p3") and False:
                    continue
                sel = lambda J, key: [i for i, j in enumerate(J)
                                      if j["trig"] == trig and (path == "any" or j["flags"].get(path))
                                      and ("erl" not in ename or path == "any" and j["tpkey"] == "erl"
                                           or path != "any" and j["tpkey"] == path)]
                ji = sel(d["jobs"], 0)
                if not ji:
                    continue
                sub = [d["jobs"][i] for i in ji]
                sres = [d["res"][i] for i in ji]
                s, kept, blk = summarize(sub, sres, a.allow_overlap)
                ckeys = {d["jobs"][i]["key"] for i in ji}
                ci = [i for i, c in enumerate(d["ctl"]) if c["key"][:-1] in ckeys]
                csub = [d["ctl"][i] for i in ci]
                cs, _, cblk = summarize(csub, [d["cres"][i] for i in ci], a.allow_overlap)
                if s["n"] < 30:
                    continue
                emit(fmt(f"{trig}/{path}", s, 22, f"blk={blk}") + f" | 对照 avgR={cs['avg']:+.3f}±{cs['se']:.3f} n={cs['n']} 差={s['avg'] - cs['avg']:+.3f}±{(s['se'] ** 2 + cs['se'] ** 2) ** 0.5:.3f}")
    open(os.path.join(a.out, "b_report.txt"), "w").write("\n".join(lines))


def ctx_levels(ctx, k, sign, entry, atr5):
    """决策 K 线 k 处因果可得的最近结构止损位与 ERL 目标。sign=+1 多头。"""
    lows, highs = [], []
    for sw in ctx.swings5:
        if sw.avail <= k and k - sw.idx <= 288:
            (lows if sw.kind == "low" else highs).append(sw.price)
    t = int(ctx.a5["t_close"][k])
    for sw in ctx.swings1h:
        if sw.avail < len(ctx.t_close1h) and int(ctx.t_close1h[sw.avail]) <= t and t - int(ctx.t_close1h[sw.idx]) <= 48 * 3600_000:
            (lows if sw.kind == "low" else highs).append(sw.price)
    below = [x for x in lows if x < entry]
    above = [x for x in highs if x > entry]
    if sign > 0:
        return (max(below) - 0.1 * atr5 if below else None), (min(above) if above else None)
    below_h = [x for x in highs if x > entry]
    above_l = [x for x in lows if x < entry]
    return (min(below_h) + 0.1 * atr5 if below_h else None), (max(above_l) if above_l else None)


def _awork(a):
    tr, rules = a
    r = run_trade(tr, rules, 0.0005, 0.0003)
    return dict(r=r["r"], how=r["how"], exit_ms=r["exit_ms"])


def cmd_a(a):
    import multiprocessing as mp
    import numpy as np
    from .backtrader_adapter import run_trade as _rt  # noqa: F401
    from .exitsim import build_trade
    from .klines import load_bars
    sigs = json.load(open(os.path.join(a.cache, "signals.json")))["signals"]
    base = nofx_rules(a.db)
    plain = plain_rules()
    ctxs = {}
    ref = {u: ict_fetch.load(a.cache, u) for u in ("BTCUSDT", "ETHUSDT")}
    rows = []
    for sg in sorted(sigs, key=lambda x: x["t0_ms"]):
        sym = sg["symbol"]
        if sym not in ctxs:
            bars5 = ict_fetch.load(a.cache, sym)
            ctxs[sym] = (ict.Ctx(bars5, ref["ETHUSDT" if sym == "BTCUSDT" else "BTCUSDT"]) if len(bars5) > 1500 else None)
            ctxs[sym] = (ctxs[sym], ict.find_setups(ctxs[sym], {}) if ctxs[sym] else [])
        ctx, setups = ctxs[sym]
        got = load_bars(a.cache, sym, sg["t0_ms"])
        if ctx is None or got is None:
            continue
        for st in setups:
            st.sym = sym
        bars, n1m = got
        tr0 = build_trade(sg, bars, "mkt", n1m)
        if tr0 is None:
            continue
        sign = 1 if sg["side"] == "long" else -1
        t0 = sg["t0_ms"]
        k = int(np.searchsorted(ctx.a5["t_close"], t0, side="right")) - 1
        if k < 400:
            continue
        entry = tr0["reference"]
        atr5 = float(ctx.atr5[k])
        near = [s for s in setups if s.side == sg["side"] and t0 - 4 * 3600_000 <= s.t_close_ms <= t0]
        trig = {s.trigger for s in near}
        # 1h 24 根区间的溢价/折价位置
        n1 = int(np.searchsorted(ctx.t_close1h, t0, side="right"))
        hh, ll = ctx.a1h["h"][max(0, n1 - 24):n1], ctx.a1h["l"][max(0, n1 - 24):n1]
        pos = (entry - ll.min()) / (hh.max() - ll.min()) if len(hh) > 12 and hh.max() > ll.min() else 0.5
        sl_s, erl = ctx_levels(ctx, k, sign, entry, atr5)
        risk0 = abs(entry - tr0["sl"])
        variants = {"A0 现行(sl0/tp0+阶梯)": (tr0, base)}
        variants["A3 sl0/tp0 plain"] = (tr0, plain)
        trs = dict(tr0, sl=sl_s) if sl_s and sign * (entry - sl_s) > 0.3 * risk0 and sign * (entry - sl_s) < 3 * risk0 else None
        if trs:
            variants["A1 结构SL+tp0(阶梯)"] = (trs, base)
            variants["A6 结构SL+tp0 plain"] = (trs, plain)
        tre = None
        if erl and sign * (erl - entry) >= 1.2 * risk0:
            tre = dict(tr0, tp=erl)
            variants["A4 sl0+ERL plain"] = (tre, plain)
        if trs and erl and sign * (erl - entry) >= 1.2 * abs(entry - sl_s):
            variants["A5 结构SL+ERL plain"] = (dict(tr0, sl=sl_s, tp=erl), plain)
        rows.append(dict(sg=sg, trig=trig, pos=pos, variants=variants,
                         pd_ok=(pos < 0.5) if sign > 0 else (pos > 0.5), t_ms=t0, entry_ms=t0 + 60000))
    print("signals usable", len(rows))
    names = sorted({n for r in rows for n in r["variants"]})
    allres = {}
    for n in names:
        idxs = [i for i, r in enumerate(rows) if n in r["variants"]]
        with mp.get_context("fork").Pool(a.procs) as pool:
            res = pool.map(_awork, [rows[i]["variants"][n] for i in idxs], chunksize=4)
        allres[n] = dict(zip(idxs, res))
    lines = []
    emit = lambda s: (print(s), lines.append(s))
    groups = {
        "全部": lambda r: True,
        "近4h有sweep": lambda r: "sweep" in r["trig"],
        "近4h有MSS": lambda r: "mss" in r["trig"],
        "近4h有CISD": lambda r: "cisd" in r["trig"],
        "近4h有FVG回补": lambda r: "fvg" in r["trig"],
        "近4h任一ICT触发": lambda r: bool(r["trig"]),
        "无ICT触发": lambda r: not r["trig"],
        "折价做多/溢价做空(pd_ok)": lambda r: r["pd_ok"],
        "逆位置(pd_bad)": lambda r: not r["pd_ok"],
        "ICT触发&pd_ok": lambda r: bool(r["trig"]) and r["pd_ok"],
    }
    for n in names:
        emit(f"\n===== {n} =====")
        res = allres[n]
        # 先对全体做同(币,方向)单仓过滤,再切组,保证各组口径一致
        order = sorted(res, key=lambda i: rows[i]["t_ms"])
        busy, kept, blocked = {}, [], 0
        for i in order:
            if res[i]["r"] is None:
                continue
            key = (rows[i]["sg"]["symbol"], rows[i]["sg"]["side"])
            if rows[i]["t_ms"] < busy.get(key, 0):
                blocked += 1
                continue
            busy[key] = res[i]["exit_ms"] or rows[i]["t_ms"]
            kept.append(i)
        for gname, fn in groups.items():
            rs = [res[i]["r"] for i in kept if fn(rows[i])]
            emit(fmt(gname, ict_bt.stats(rs), 26))
        emit(f"(blocked={blocked})")
    open(os.path.join(a.out, "a_report.txt"), "w").write("\n".join(lines))
    json.dump([dict(sym=r["sg"]["symbol"], side=r["sg"]["side"], t=r["t_ms"], trig=sorted(r["trig"]), pos=r["pos"]) for r in rows],
              open(os.path.join(a.out, "a_rows.json"), "w"))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("mode", choices=["a", "b"])
    ap.add_argument("--cache", default="/Users/zhangyun/workspace/nofx/data/backtest_cache")
    ap.add_argument("--db", default="/Users/zhangyun/workspace/nofx/data/data.db")
    ap.add_argument("--out", required=True)
    ap.add_argument("--procs", type=int, default=9)
    ap.add_argument("--only", default="")
    ap.add_argument("--universe", default="")
    ap.add_argument("--universe-json", default="")
    ap.add_argument("--allow-overlap", action="store_true")
    a = ap.parse_args()
    os.makedirs(a.out, exist_ok=True)
    {"b": cmd_b, "a": cmd_a}[a.mode](a)


if __name__ == "__main__":
    main()
