"""信号 → 特征 + 标签(ml-dataset)。每个信号一行,不做「同币同向单仓」过滤。"""
import csv
import json
from collections import Counter
from datetime import timedelta

from . import exitsim, mlfeatures
from .signals import parse_utc

LOOKUP_MIN = 10  # 决策记录与信号 t0 的最大时间差(分钟)


def plain_params(base):
    """plain = 无保本/盈利锁/减仓/追踪/峰值回撤/止盈上限。"""
    return dict(base, be_arm=0, lock_r=0, lock_trim=False, trim_r=0, trail=False, pd_arm=0, tp_cap=0)


def find_record(conn, trader_id, sig):
    """±LOOKUP_MIN 分钟内、同 action+币的决策记录,取离 t0 最近的。返回 (structured signal dict, AI 决策 dict) 或 None。"""
    t = parse_utc(sig["t0"])
    lo = (t - timedelta(minutes=LOOKUP_MIN)).strftime("%Y-%m-%d %H:%M:%S")
    hi = (t + timedelta(minutes=LOOKUP_MIN)).strftime("%Y-%m-%d %H:%M:%S")
    act = f"open_{sig['side']}" + ("_limit" if sig["kind"] == "limit" else "")
    rec = conn.execute(
        "select input_prompt, decision_json from decision_records where trader_id=? and timestamp between ? and ? "
        "and decisions like ? order by abs(julianday(substr(timestamp,1,19))-julianday(?)) limit 1",
        (trader_id, lo, hi, f'%"{act}","symbol":"{sig["symbol"]}"%', t.strftime("%Y-%m-%d %H:%M:%S"))).fetchone()
    if not rec:
        return None
    s = mlfeatures.extract_structured(rec[0], sig["symbol"])
    if s is None:
        return None
    try:
        arr = json.loads(rec[1] or "[]") or []
    except ValueError:
        arr = []
    ai = next((d for d in arr if isinstance(d, dict) and d.get("symbol") == sig["symbol"]
               and str(d.get("action", "")).startswith("open")), {})
    return s, ai


def build_rows(conn, trader_id, sigs, cache, base, horizon=72):
    """返回 (rows, missing)。标签:r_base(基线出场参数)、r_plain(plain)、mfe(plain 持仓期内最大有利 R),均为 t0 市价入场。"""
    base = dict(base, horizon_h=horizon)
    plain = plain_params(base)
    trades, _ = exitsim.load_trades(sigs, cache, "mkt")
    tmap = {(t["sym"], t["t0"]): t for t in trades}
    rows, missing = [], 0
    for sg in sigs:
        tr = tmap.get((sg["symbol"], sg["t0"]))
        found = find_record(conn, trader_id, sg) if tr else None
        if not tr or not found:
            missing += 1
            continue
        s, ai = found
        rb = exitsim.simulate(tr, base)[0]
        rp, _, peak, _ = exitsim.simulate(tr, plain)
        rows.append(dict(id=sg["id"], sym=sg["symbol"], side=sg["side"], t0=sg["t0"], t0_ms=sg["t0_ms"],
                         kind=sg["kind"], outcome=sg["outcome"], r_base=rb, r_plain=rp, mfe=peak,
                         feat=mlfeatures.features(s, sg["side"], ai)))
    return rows, missing


def coverage(rows):
    n = len(rows)
    cnt = Counter(k for r in rows for k, v in r["feat"].items() if v is not None)
    keys = sorted({k for r in rows for k in r["feat"]})
    return {k: (cnt[k] / n if n else 0.0) for k in keys}


def write_csv(rows, path):
    keys = sorted({k for r in rows for k in r["feat"]})
    head = ["id", "sym", "side", "t0", "kind", "outcome", "r_base", "r_plain", "mfe"]
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(head + keys)
        for r in rows:
            w.writerow([r[h] for h in head] + ["" if r["feat"].get(k) is None else r["feat"][k] for k in keys])
