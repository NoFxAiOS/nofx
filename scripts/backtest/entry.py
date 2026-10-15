"""入场对比适配：市价/限价都交给 Backtrader 撮合。"""
from .exitsim import SLIP, TAKER, build_trade
from .backtrader_adapter import run_trade
from .klines import load_bars
from .params import FALLBACK


def build_rows(signals, cache, horizons, min_bars=10, allow_overlap=False,
               commission=TAKER, slippage=SLIP, limit_valid_min=45):
    res = {h: [] for h in horizons}
    skipped = {"nodata": 0, "short": 0}
    busy = {h: {"mkt": {}, "lim": {}} for h in horizons}
    for s in sorted(signals, key=lambda x: x["t0_ms"]):
        got = load_bars(cache, s["symbol"], s["t0_ms"])
        if got is None:
            skipped["nodata"] += 1
            continue
        bars, n1m = got
        if n1m < min_bars:
            skipped["short"] += 1
            continue
        for h in horizons:
            p = dict(FALLBACK, be_arm=0, lock_r=0, lock_trim=False, trim_r=0,
                     trail=False, pd_arm=0, tp_cap=0, quick_stop_h=0, horizon_h=h)
            row = dict(sym=s["symbol"], side=s["side"], kind=s["kind"], outcome=s["outcome"],
                       t0=s["t0"], mkt_r=None, mkt_how="invalid")
            key = (s["symbol"], s["side"])
            for kind in ("mkt", "lim"):
                if kind == "lim" and s["kind"] != "limit":
                    continue
                row[kind + "_r"] = None
                row[kind + "_how"] = "invalid"
                tr = build_trade(s, bars, kind, n1m, limit_valid_min)
                if tr is None:
                    continue
                if not allow_overlap and tr["start_ms"] < busy[h][kind].get(key, 0):
                    row[kind + "_how"] = "blocked"
                    continue
                result = run_trade(tr, p, commission, slippage)
                row[kind + "_r"], row[kind + "_how"] = result["r"], result["how"]
                if result["r"] is not None:
                    row[kind + "_risk"] = result["risk"] / result["entry_price"]
                    busy[h][kind][key] = result["exit_ms"]
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
        out(f"\n===== horizon {h}h (Backtrader SL/TP, actual fill fees) =====")
        out("A  limit simulated by Backtrader      : " + stats([r.get("lim_r") for r in lim_origin]))
        out("B  market next open, all limit signals  : " + stats([r["mkt_r"] for r in lim_origin]))
        out("B1   market, signals limit DID fill     : " + stats([r["mkt_r"] for r in lim_origin if r["outcome"] == "filled"]))
        out("B2   market, signals limit did NOT fill : " + stats([r["mkt_r"] for r in lim_origin if r["outcome"] != "filled"]))
        out("M  native market opens (AI chose market): " + stats([r["mkt_r"] for r in rows if r["kind"] == "market"]))
        for side in ("long", "short"):
            out(f"  [{side}] limit simulated: " + stats([r.get("lim_r") for r in lim_origin if r["side"] == side])
                + " | market (limit signals): " + stats([r["mkt_r"] for r in lim_origin if r["side"] == side]))
        inv = sum(1 for r in rows if r["mkt_how"] == "invalid")
        blk = sum(1 for r in rows if r["mkt_how"] == "blocked")
        out(f"  market-invalid (price already beyond SL/TP at signal): {inv}")
        out(f"  market-blocked (同币同向已有持仓,跳过): {blk}")
        unfilled = sum(r.get("lim_how") == "unfilled" for r in lim_origin)
        out(f"  limit-unfilled: {unfilled}")
        risk_m = [r["mkt_risk"] for r in rows if r.get("mkt_risk")]
        risk_l = [r["lim_risk"] for r in rows if r.get("lim_risk")]
        if risk_m:
            out(f"  avg stop distance: market {100 * sum(risk_m) / len(risk_m):.2f}%  limit "
                f"{100 * sum(risk_l) / max(1, len(risk_l)):.2f}%")
