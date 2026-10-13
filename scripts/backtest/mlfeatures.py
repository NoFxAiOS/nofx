"""Structured Signal → 特征字典(纯 stdlib)。符号约定:凡带 `_with` 或方向相关的特征,正值 = 与交易方向一致
(做多取原值,做空取反);`is_long`、`hour_utc` 等不带方向。缺失字段一律为 None。"""
import json
import math
import re
from datetime import datetime

TIMEFRAMES = ("15m", "1h", "4h", "1d")


def num(x):
    try:
        return float(x)
    except (TypeError, ValueError):
        return None


def dig(d, *path):
    for k in path:
        if not isinstance(d, dict):
            return None
        d = d.get(k)
    return d


def extract_structured(prompt, symbol):
    """从 input_prompt 取 `=== SYMBOL Structured Signal` 后面的 JSON;取不到返回 None。"""
    if not prompt:
        return None
    m = re.search(r"=== %s Structured Signal.*?\n(\{.*?\})\n" % re.escape(symbol), prompt, re.S)
    if not m:
        return None
    try:
        s = json.loads(m.group(1))
    except ValueError:
        return None
    return s if isinstance(s, dict) else None


def _first(v):
    return num(v[0]) if isinstance(v, list) and v else None


def features(s, side, ai):
    """s: Structured Signal dict;side: long|short;ai: 该币的 AI 决策 dict(confidence / exit_mode)。"""
    L = side == "long"
    sg = 1 if L else -1
    tf = s.get("timeframes") or {}
    price = num(s.get("price"))
    out = {}
    for k in TIMEFRAMES:
        t = tf.get(k) or {}
        out[f"rsi_{k}"] = num(t.get("rsi"))
        out[f"atrpct_{k}"] = num(t.get("atr_pct"))
        out[f"atrpctl_{k}"] = num(t.get("atr_percentile"))
        ef, es = num(t.get("ema_fast")), num(t.get("ema_slow"))
        out[f"ema_align_{k}"] = (sg if ef > es else -sg) if ef and es else None  # +1 = 与交易方向一致
        out[f"dist_emaf_{k}"] = sg * (price / ef - 1) * 100 if ef and price else None  # + = 价格在 EMA 的交易方向一侧
        tr = t.get("trend")
        if tr is None:
            out[f"trend_with_{k}"] = None
        elif tr == "range":
            out[f"trend_with_{k}"] = 0
        else:
            out[f"trend_with_{k}"] = 1 if (tr in ("up", "pullback")) == L else -1
        mt = t.get("macd_trend")
        out[f"macd_rising_{k}"] = None if mt is None else sg * (1 if mt == "rising" else -1)
        out[f"volratio_{k}"] = num(t.get("volume_ratio"))
        rw = num(t.get("trend_window_return_pct"))
        out[f"retwin_{k}"] = sg * rw if rw is not None else None
        lc = t.get("last_closed_candle")
        out[f"lastcandle_with_{k}"] = None if lc is None else (1 if (lc == "bullish") == L else -1)
        if L:
            wall = _first(t.get("resistance_dist_pct"))
        else:
            sup = t.get("support_dist_pct")
            wall = _first([abs(num(x)) for x in sup if num(x) is not None]) if isinstance(sup, list) else None
        out[f"wall_dist_{k}"] = wall
        sh, sl = num(t.get("structure_high_dist_pct")), num(t.get("structure_low_dist_pct"))
        out[f"room_struct_{k}"] = sh if L else (abs(sl) if sl is not None else None)

    mr = s.get("market_regime") or {}
    inp = mr.get("inputs") or {}
    reg = mr.get("regime")
    if not mr:
        out["regime_trend_with"] = out["regime_chop"] = None
    else:
        out["regime_trend_with"] = (1 if reg == ("TREND_UP" if L else "TREND_DOWN")
                                    else (-1 if reg == ("TREND_DOWN" if L else "TREND_UP") else 0))
        out["regime_chop"] = 1 if str(reg or "").startswith("CHOP") else 0
    out["regime_conf"] = num(mr.get("confidence"))
    out["adx_1h"] = num(inp.get("adx_1h"))
    out["adx_4h"] = num(inp.get("adx_4h"))
    out["bbw_pctl_1h"] = num(inp.get("bb_width_percentile_1h"))

    g = dig(s, "hard_entry_gate", side) or {}
    rr = g.get("rr_scan") or {}
    out["stop_pct"] = num(g.get("stop_plan_pct"))
    out["best_rr"] = num(rr.get("best_rr"))
    out["first_obstacle_rr"] = num(rr.get("first_obstacle_rr"))
    out["first_target_rr"] = num(rr.get("first_target_rr"))

    d = s.get("derivatives") or {}
    fa = num(d.get("funding_annualized_pct"))
    out["funding_ann_with"] = sg * fa if fa is not None else None  # + = 拥挤方向与我们同侧
    out["ls_acct"] = num(d.get("long_short_account_ratio"))
    out["top_ratio"] = num(d.get("top_trader_position_ratio"))
    out["oi_1h"] = num(d.get("oi_change_1h_pct"))
    out["oi_vs_avg"] = num(d.get("oi_vs_avg_pct"))
    c24, c60 = num(d.get("price_change_24h_live_pct")), num(d.get("price_change_60m_live_pct"))
    out["chg24_with"] = sg * c24 if c24 is not None else None
    out["chg60_with"] = sg * c60 if c60 is not None else None
    tk = num(d.get("taker_buy_sell_ratio"))
    out["taker_ratio_with"] = tk if L else (1 / tk if tk else None)

    b = s.get("bias") or {}
    out["scanner_with"] = None if not b else (1 if b.get("scanner") == side else (-1 if b.get("scanner") in ("long", "short") else 0))
    out["structure_with"] = None if not b else (1 if b.get("structure") == side else (-1 if b.get("structure") in ("long", "short") else 0))
    sc = s.get("signal_conflict") or {}
    out["conflict"] = 1 if sc.get("directional_conflict") else 0
    ds = num(sc.get("directional_score"))
    out["dir_score_with"] = sg * ds if ds is not None else None
    out["breakout_extended"] = 1 if (s.get("breakout") or {}).get("status") == "extended" else 0
    out["ret4h_pump"] = num(dig(s, "pump_guard", "return_4h_pct"))
    out["ema20_4h_dist"] = num(dig(s, "long_pullback", "ema20_4h_dist_pct"))
    q = num(dig(s, "liquidity", "quote_volume_24h_usd"))
    out["qvol24_log"] = math.log10(q) if q and q > 0 else None
    out["btc_beta"] = num(dig(s, "btc_correlation", "beta"))
    out["is_long"] = 1 if L else 0
    out["ai_conf"] = num((ai or {}).get("confidence"))
    out["exit_trend"] = 1 if ((ai or {}).get("exit_mode") or "trend") == "trend" else 0
    ts = None
    if s.get("timestamp_utc"):
        try:
            ts = datetime.fromisoformat(str(s["timestamp_utc"]).replace("Z", "+00:00"))
        except ValueError:
            ts = None
    out["hour_utc"] = ts.hour if ts else None
    return out
