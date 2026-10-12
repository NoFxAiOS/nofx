"""出场规则模拟器(R 空间)。多空镜像:把价格换算成「有利方向 R」后统一处理。"""
import csv
import json
from datetime import datetime

from .klines import floor_min, load_bars

TAKER, MAKER, SLIP = 0.0005, 0.0002, 0.0003
HOUR_MS = 3_600_000


def build_trade(sig, bars, kind):
    """kind: mkt=在 t0 市价入场;lim=限价成交的信号按实际成交价入场。无法模拟返回 None。"""
    side = sig["side"]
    t0 = floor_min(sig["t0_ms"])
    if kind == "mkt":
        e = bars[0][4] * (1 + SLIP if side == "long" else 1 - SLIP)
        sl, tp = sig["sl0"], sig["tp0"]
        start, fee_in = t0 + 60_000, TAKER
        mode, atr = sig.get("exit_mode") or "trend", sig.get("atr1h")
    else:
        if sig["outcome"] != "filled":
            return None
        e, sl, tp = sig["fill_px"], sig["sl"], sig["tp"]
        ft = int(datetime.fromisoformat(sig["fill_t"]).timestamp() * 1000)
        start, fee_in = floor_min(ft) + 60_000, MAKER
        mode = sig.get("exit_mode_last") or sig.get("exit_mode") or "trend"
        atr = sig.get("atr1h_last") if sig.get("atr1h_last") is not None else sig.get("atr1h")
    if sl is None or tp is None or e is None:
        return None
    risk = abs(e - sl)
    if risk <= 0 or not ((sl < e < tp) if side == "long" else (tp < e < sl)):
        return None
    sgn = 1 if side == "long" else -1
    rb = []
    for t, o, h, l, c in bars:
        if t < start:
            continue
        fav, adv = (h, l) if side == "long" else (l, h)
        rb.append((t, sgn * (fav - e) / risk, sgn * (adv - e) / risk, sgn * (c - e) / risk))
    if not rb:
        return None
    return dict(sym=sig["symbol"], side=side, t0=sig["t0"], t0_ms=sig["t0_ms"], start_ms=rb[0][0], bars=rb,
                tp_r=abs(tp - e) / risk, atr_r=(atr or 0) / 100 * e / risk, mode=mode,
                fee_r=(fee_in + TAKER) / (risk / e), risk_pct=risk / e * 100)


def load_trades(signals, cache, kind, min_bars=10, include_market=True):
    trades, skipped = [], {"nodata": 0, "short": 0, "invalid": 0}
    for sig in signals:
        if kind == "lim" and sig["outcome"] != "filled":
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
        tr = build_trade(sig, bars, kind)
        if tr is None:
            skipped["invalid"] += 1
            continue
        trades.append(tr)
    return trades, skipped


def simulate(tr, p):
    """返回 (净 R, 出场类型, peak, 出场时间 ms)。出场类型: sl|be|trail|tp|pd|time|open。
    出场时间=触发出场的 K 线时间;horizon 到点仍持仓(open)则取 horizon 末(无 horizon 时取最后一根 K 线)。"""
    mode = p.get("force_mode") or tr["mode"]
    sl, sl_src = -1.0, "sl"
    pos, real, peak = 1.0, 0.0, 0.0
    trimmed = lock_trimmed = False
    tp_on = p["use_tp"]
    tp = min(tr["tp_r"], p["tp_cap"]) if p["tp_cap"] else tr["tp_r"]
    t_start = tr["bars"][0][0]
    how = "open"
    exit_t = None
    last_c = tr["bars"][0][3]
    be_off = p["be_off"]
    for t, hi, lo, c in tr["bars"]:
        if p["horizon_h"] and t - t_start >= p["horizon_h"] * HOUR_MS:
            break
        exit_t = t  # 若本根 K 线触发出场,出场时间即此刻
        # (1) 止损(同一根 K 线内 SL 与 TP 都触及时算止损)
        if lo <= sl:
            real += pos * sl
            pos = 0.0
            how = sl_src
            break
        # (2) 止盈
        if tp_on and hi >= tp:
            f = p["tp_frac_trend"] if (mode == "trend" and p["trail"]) else 1.0
            real += pos * f * tp
            pos *= 1 - f
            tp_on = False
            if pos <= 1e-9:
                pos = 0.0
                how = "tp"
                break
        # (3) 峰值
        peak = max(peak, hi)
        # (4) 保本档
        if p["be_arm"] > 0 and peak >= p["be_arm"] and sl < be_off:
            sl, sl_src = be_off, "be"
        # (5) 盈利锁:止损抬到 be_off,可选减仓 50%(一次)
        if p.get("lock_r", 0) > 0 and peak >= p["lock_r"]:
            if sl < be_off:
                sl, sl_src = be_off, "be"
            if p.get("lock_trim") and not lock_trimmed:
                real += pos * 0.5 * p["lock_r"]
                pos *= 0.5
                lock_trimmed = True
        # (6) R 减仓档
        if p["trim_r"] > 0 and not trimmed and peak >= p["trim_r"]:
            real += pos * p["trim_frac"] * p["trim_r"]
            pos *= 1 - p["trim_frac"]
            trimmed = True
        # (7) 追踪止损(仅趋势模式)
        if p["trail"] and mode == "trend" and tr["atr_r"] > 0 and peak >= p["trail_arm"]:
            cand = hi - p["trail_atr"] * tr["atr_r"]
            if cand > sl:
                sl, sl_src = cand, "trail"
        # (8) 峰值回撤
        if p["pd_arm"] > 0 and peak >= p["pd_arm"]:
            lvl = peak * (1 - p["pd_give"])
            if lo <= lvl and lvl > sl:
                real += pos * lvl
                pos = 0.0
                how = "pd"
                break
        # (9) quick 模式时间止损
        if mode == "quick" and p["quick_stop_h"] > 0 and t - t_start >= p["quick_stop_h"] * HOUR_MS and c <= 0:
            real += pos * c
            pos = 0.0
            how = "time"
            break
        last_c = c
    if pos > 0 and how == "open":
        real += pos * last_c  # 按 horizon 内最后一根 K 线收盘价盯市,而非数据末尾
        exit_t = t_start + p["horizon_h"] * HOUR_MS if p["horizon_h"] else tr["bars"][-1][0]
    return real - tr["fee_r"], how, peak, exit_t


def summarize(trades, p, allow_overlap=False):
    """逐笔模拟。默认「同币同向一次只持一仓」:按入场时间顺序,入场早于该 (币,方向) 上一笔已接受交易的
    出场时间的信号被跳过(blocked),与线上「已有同币同向仓位则拒绝开仓」一致;allow_overlap 关闭该约束。"""
    rs, hows, blocked, busy = [], {}, 0, {}
    for tr in sorted(trades, key=lambda x: (x.get("start_ms", x["bars"][0][0]), x["t0_ms"])):
        key = (tr["sym"], tr["side"])
        if not allow_overlap and tr["bars"][0][0] < busy.get(key, 0):
            blocked += 1
            continue
        r, how, _, exit_t = simulate(tr, p)
        busy[key] = exit_t
        rs.append((tr["t0_ms"], r))
        hows[how] = hows.get(how, 0) + 1
    rs.sort()
    v = [r for _, r in rs]
    if not v:
        return dict(n=0, sum=0.0, avg=0.0, win=0.0, dd=0.0, h1=0.0, h2=0.0, drop5=0.0, open=0, hows={}, blocked=blocked)
    eq = pk = dd = 0.0
    for r in v:
        eq += r
        pk = max(pk, eq)
        dd = max(dd, pk - eq)
    h = len(v) // 2
    s = sorted(v)
    top = s[:-5] if len(s) > 5 else s
    return dict(n=len(v), sum=sum(v), avg=sum(v) / len(v), win=sum(r > 0 for r in v) / len(v), dd=dd,
                h1=sum(v[:h]), h2=sum(v[h:]), drop5=sum(top) / len(top), open=hows.get("open", 0), hows=hows,
                blocked=blocked)


def format_row(name, s, width=34):
    hows = " ".join(f"{k}={v}" for k, v in sorted(s["hows"].items()))
    return (f"{name:{width}s} n={s['n']} sumR={s['sum']:+7.1f} avgR={s['avg']:+.3f} win={s['win']:.0%} "
            f"maxDD={s['dd']:5.1f} H1={s['h1']:+6.1f} H2={s['h2']:+6.1f} drop5avg={s['drop5']:+.3f} "
            f"blocked={s['blocked']} open={s['open']} [{hows}]")


def run_variants(trades, base, variants, csv_path=None, out=print, allow_overlap=False):
    rows = []
    width = max([len(n) for n, _ in variants] + [8])
    for name, over in variants:
        p = dict(base)
        p.update(over)
        s = summarize(trades, p, allow_overlap)
        out(format_row(name, s, width))
        rows.append((name, s))
    if csv_path:
        with open(csv_path, "w", newline="") as f:
            w = csv.writer(f)
            w.writerow(["name", "n", "sumR", "avgR", "win", "maxDD", "H1", "H2", "drop5avg", "blocked", "open", "exit_types"])
            for name, s in rows:
                w.writerow([name, s["n"], f"{s['sum']:.3f}", f"{s['avg']:.4f}", f"{s['win']:.4f}", f"{s['dd']:.3f}",
                            f"{s['h1']:.3f}", f"{s['h2']:.3f}", f"{s['drop5']:.4f}", s["blocked"], s["open"], json.dumps(s["hows"])])
    return rows
