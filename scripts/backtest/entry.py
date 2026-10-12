"""入场方式对比:限价实际成交 vs 信号时刻市价入场(固定 SL/TP 括号,同根 K 线 SL+TP 算止损)。"""
from datetime import datetime

from .exitsim import MAKER, SLIP, TAKER
from .klines import floor_min, load_bars


def bracket(bars, start_ms, sl, tp, side, hours):
    """从 start_ms 起走 SL/TP 括号,返回 (出场价, sl|tp|time, 出场时间ms);time 出场的时间取 horizon 末;
    horizon 内无 K 线返回 (None, 'time', end)。"""
    end = start_ms + hours * 3_600_000
    last = None
    for t, o, h, l, c in bars:
        if t < start_ms:
            continue
        if t >= end:
            break
        last = c
        if side == "long":
            if l <= sl:
                return sl, "sl", t
            if h >= tp:
                return tp, "tp", t
        else:
            if h >= sl:
                return sl, "sl", t
            if l <= tp:
                return tp, "tp", t
    return last, "time", end


def r_mult(entry, exit_, sl, side, fee_in):
    risk = abs(entry - sl) / entry
    pnl = (exit_ - entry) / entry * (1 if side == "long" else -1)
    return (pnl - fee_in - TAKER) / risk, risk


def build_rows(signals, cache, horizons, min_bars=10, allow_overlap=False):
    res = {h: [] for h in horizons}
    skipped = {"nodata": 0, "short": 0}
    # 市价行默认「同币同向一次只持一仓」:每个 horizon 各自维护 (币,方向) 的 busy-until(=括号出场时间)
    busy = {h: {} for h in horizons}
    for s in sorted(signals, key=lambda x: x["t0_ms"]):
        got = load_bars(cache, s["symbol"], s["t0_ms"])
        if got is None:
            skipped["nodata"] += 1
            continue
        bars, n1m = got
        if n1m < min_bars:
            skipped["short"] += 1
            continue
        t0 = floor_min(s["t0_ms"])
        side = s["side"]
        mkt = bars[0][4] * (1 + SLIP if side == "long" else 1 - SLIP)
        sl, tp = s["sl0"], s["tp0"]
        ok = sl is not None and tp is not None and ((sl < mkt < tp) if side == "long" else (tp < mkt < sl))
        for h in horizons:
            row = dict(sym=s["symbol"], side=side, kind=s["kind"], outcome=s["outcome"], t0=s["t0"],
                       mkt_r=None, mkt_how="invalid")
            if ok and not allow_overlap and t0 + 60_000 < busy[h].get((s["symbol"], side), 0):
                row["mkt_how"] = "blocked"
            elif ok:
                ex, how, exit_t = bracket(bars, t0 + 60_000, sl, tp, side, h)
                if ex is None:
                    continue
                busy[h][(s["symbol"], side)] = exit_t
                row["mkt_r"], row["mkt_risk"] = r_mult(mkt, ex, sl, side, TAKER)
                row["mkt_how"] = how
            if s["outcome"] == "filled":
                ft = int(datetime.fromisoformat(s["fill_t"]).timestamp() * 1000)
                ex, how, _ = bracket(bars, floor_min(ft) + 60_000, s["sl"], s["tp"], side, h)
                if ex is not None:
                    row["lim_r"], row["lim_risk"] = r_mult(s["fill_px"], ex, s["sl"], side, MAKER)
                    row["lim_how"] = how
            res[h].append(row)
    return res, skipped


def stats(rs):
    rs = [r for r in rs if r is not None]
    if not rs:
        return "n=  0"
    w = sum(1 for r in rs if r > 0)
    return f"n={len(rs):3d} sumR={sum(rs):+7.1f} avgR={sum(rs) / len(rs):+.3f} win={w / len(rs):.0%}"


def report(res, skipped, out=print):
    out(f"skipped {skipped}")
    for h, rows in res.items():
        lim_origin = [r for r in rows if r["kind"] == "limit"]
        out(f"\n===== horizon {h}h (SL/TP bracket, fees incl.) =====")
        out("A  limit as executed (filled only)      : " + stats([r.get("lim_r") for r in lim_origin if r["outcome"] == "filled"]))
        out("B  market at signal, all limit signals  : " + stats([r["mkt_r"] for r in lim_origin]))
        out("B1   market, signals limit DID fill     : " + stats([r["mkt_r"] for r in lim_origin if r["outcome"] == "filled"]))
        out("B2   market, signals limit did NOT fill : " + stats([r["mkt_r"] for r in lim_origin if r["outcome"] != "filled"]))
        out("M  native market opens (AI chose market): " + stats([r["mkt_r"] for r in rows if r["kind"] == "market"]))
        for side in ("long", "short"):
            out(f"  [{side}] limit filled: " + stats([r.get("lim_r") for r in lim_origin if r["outcome"] == "filled" and r["side"] == side])
                + " | market (limit signals): " + stats([r["mkt_r"] for r in lim_origin if r["side"] == side]))
        inv = sum(1 for r in rows if r["mkt_how"] == "invalid")
        blk = sum(1 for r in rows if r["mkt_how"] == "blocked")
        out(f"  market-invalid (price already beyond SL/TP at signal): {inv}")
        out(f"  market-blocked (同币同向已有持仓,跳过): {blk}")
        risk_m = [r["mkt_risk"] for r in rows if r.get("mkt_risk")]
        risk_l = [r["lim_risk"] for r in rows if r.get("lim_risk")]
        if risk_m:
            out(f"  avg stop distance: market {100 * sum(risk_m) / len(risk_m):.2f}%  limit "
                f"{100 * sum(risk_l) / max(1, len(risk_l)):.2f}%")
