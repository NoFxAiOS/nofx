"""从 decision_records 提取某个 trader 的 AI 开仓信号,并用 trader_positions 判定限价单是否成交。"""
import json
import re
import sqlite3
from datetime import datetime, timedelta, timezone

OPEN_ACTIONS = {"open_long", "open_short", "open_long_limit", "open_short_limit"}
FILL_WINDOW_MIN = 45  # 最后一次限价决策之后多久内出现的持仓算这张单成交


def connect_ro(path):
    """只读打开 sqlite(线上 bot 正在写这个库,绝不能读写打开)。"""
    return sqlite3.connect(f"file:{path}?mode=ro", uri=True)


def parse_utc(s):
    """UTC ISO 日期/时间(无时区视为 UTC)→ aware datetime。"""
    d = datetime.fromisoformat(s.strip().replace("Z", "+00:00"))
    return d if d.tzinfo else d.replace(tzinfo=timezone.utc)


def _ms(dt):
    return int(dt.timestamp() * 1000)


def resolve_trader(conn, name=None, trader_id=None):
    if trader_id:
        row = conn.execute("select id, name, strategy_id from traders where id=?", (trader_id,)).fetchone()
    elif name:
        row = conn.execute("select id, name, strategy_id from traders where name=?", (name,)).fetchone()
    else:
        raise ValueError("需要 --trader 或 --trader-id")
    if not row:
        raise ValueError(f"找不到 trader: {name or trader_id}")
    return {"id": row[0], "name": row[1], "strategy_id": row[2]}


def load_risk_control(conn, strategy_id):
    row = conn.execute("select config from strategies where id=?", (strategy_id,)).fetchone()
    if not row or not row[0]:
        raise ValueError(f"找不到策略配置: {strategy_id}")
    return (json.loads(row[0]).get("risk_control")) or {}


def _loads(s, default):
    if not s:
        return default
    try:
        v = json.loads(s)
    except (TypeError, ValueError):
        return default
    return v


def _atr_1h(prompt, symbol):
    if not prompt:
        return None
    m = re.search(r"=== %s Structured Signal.*?\n(\{.*?\})\n" % re.escape(symbol), prompt, re.S)
    if not m:
        return None
    try:
        sig = json.loads(m.group(1))
        v = ((sig.get("timeframes") or {}).get("1h") or {}).get("atr_pct")
        return float(v) if v is not None else None
    except (ValueError, TypeError, AttributeError):
        return None


def _meta(decision_json, prompt, symbol, action):
    """同一条记录里该币的 exit_mode(缺省 trend)与 1h ATR%。"""
    mode = None
    arr = _loads(decision_json, [])
    cands = [d for d in arr if isinstance(d, dict) and d.get("symbol") == symbol
             and str(d.get("action", "")).startswith("open")] if isinstance(arr, list) else []
    cands.sort(key=lambda d: d.get("action") != action)  # 同 action 的优先
    if cands:
        mode = cands[0].get("exit_mode")
    return mode or "trend", _atr_1h(prompt, symbol)


def _side(action):
    return "long" if "long" in action else "short"


def _rec_time(ts):
    try:
        return parse_utc(ts)
    except (ValueError, AttributeError):
        return None


def raw_decisions(conn, trader_id, since, until):
    """按时间顺序返回成功的开仓决策(含同记录的 exit_mode / atr)。since/until 为 aware datetime 或 None。"""
    q = "select id, timestamp, decisions, decision_json, input_prompt from decision_records where trader_id=?"
    args = [trader_id]
    # 记录时间戳晚于决策元素时间戳若干秒;SQL 上放宽 1 小时,精确过滤在元素时间上做
    if since:
        q += " and timestamp >= ?"
        args.append((since - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"))
    if until:
        q += " and timestamp <= ?"
        args.append((until + timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"))
    q += " and decisions like '%\"open_%' order by timestamp, id"
    out = []
    for rid, rts, decisions, dj, prompt in conn.execute(q, args):
        arr = _loads(decisions, [])
        if not isinstance(arr, list):
            continue
        for d in arr:
            if not isinstance(d, dict) or d.get("action") not in OPEN_ACTIONS or not d.get("success"):
                continue
            t = _rec_time(d.get("timestamp")) or _rec_time(rts)
            if t is None or (since and t < since) or (until and t >= until):
                continue
            mode, atr = _meta(dj, prompt, d.get("symbol"), d["action"])
            out.append(dict(rid=rid, t=t, symbol=d.get("symbol"), action=d["action"], side=_side(d["action"]),
                            limit="limit" in d["action"], price=d.get("price"), sl=d.get("stop_loss"),
                            tp=d.get("take_profit"), exit_mode=mode, atr=atr))
    out.sort(key=lambda x: (x["t"], x["rid"]))
    return out


def load_positions(conn, trader_id):
    rows = conn.execute(
        "select id, symbol, side, entry_price, entry_time, ai_managed from trader_positions "
        "where trader_id=? and entry_time is not null", (trader_id,)).fetchall()
    return [dict(id=r[0], symbol=r[1], side=(r[2] or "").lower(), entry_price=r[3], entry_time=r[4],
                 ai_managed=1 if r[5] else 0) for r in rows]


def _has_fill(positions, symbol, side, lo_ms, hi_ms):
    return any(p["symbol"] == symbol and p["side"] == side and lo_ms < p["entry_time"] <= hi_ms for p in positions)


def build_signals(decisions, positions, dedupe_min=60, max_fill_dev_pct=0.5):
    """把原始决策归并成信号。

    同币同方向的连续限价决策,相邻间隔 < dedupe_min 分钟且期间没有成交 → 同一个信号
    (t0/sl0/tp0/exit_mode/atr 取第一条,最后一条的 sl/tp 记为 *_last)。市价开仓各自独立。
    """
    chains = []
    open_chain = {}  # symbol -> chain
    for d in decisions:
        sym = d["symbol"]
        ch = open_chain.get(sym)
        if ch is not None:
            gap_ok = (d["t"] - ch["last_t"]) < timedelta(minutes=dedupe_min)
            filled = _has_fill(positions, sym, ch["side"], _ms(ch["last_t"]), _ms(d["t"]))
            if not (d["limit"] and ch["kind"] == "limit" and d["side"] == ch["side"] and gap_ok and not filled):
                open_chain.pop(sym)
                ch = None
        if ch is None:
            ch = dict(symbol=sym, side=d["side"], kind="limit" if d["limit"] else "market", t0=d["t"],
                      px0=d["price"], sl0=d["sl"], tp0=d["tp"], exit_mode=d["exit_mode"], atr1h=d["atr"], n=0, prices=[])
            chains.append(ch)
            if d["limit"]:
                open_chain[sym] = ch
        ch["n"] += 1
        if d["price"]:
            ch["prices"].append(d["price"])
        ch.update(last_t=d["t"], px_last=d["price"], sl_last=d["sl"], tp_last=d["tp"],
                  exit_mode_last=d["exit_mode"], atr1h_last=d["atr"] if d["atr"] is not None else ch["atr1h"])
    return _finalize(chains, positions, max_fill_dev_pct)


def _near_limit(fill_px, prices, max_dev_pct):
    """成交价是否贴近这个信号挂过的某个限价(用来排除同币同向的手动单)。max_dev_pct<=0 关闭检查。"""
    if max_dev_pct <= 0 or not prices or not fill_px:
        return True
    return any(abs(fill_px - p) / p * 100 <= max_dev_pct for p in prices)


def _finalize(chains, positions, max_fill_dev_pct=0.5):
    used = set()
    out = []
    for i, ch in enumerate(chains):
        sig = dict(
            id=i, symbol=ch["symbol"], side=ch["side"], kind=ch["kind"],
            t0=ch["t0"].isoformat(), t0_ms=_ms(ch["t0"]),
            px0=ch["px0"], sl0=ch["sl0"], tp0=ch["tp0"], exit_mode=ch["exit_mode"], atr1h=ch["atr1h"],
            sl_last=ch["sl_last"], tp_last=ch["tp_last"], exit_mode_last=ch["exit_mode_last"],
            atr1h_last=ch["atr1h_last"], n=ch["n"], last_t=ch["last_t"].isoformat(),
        )
        if ch["kind"] == "market":
            sig.update(outcome="market", fill_px=None, fill_t=None, sl=ch["sl0"], tp=ch["tp0"])
        else:
            lo, hi = _ms(ch["t0"]), _ms(ch["last_t"] + timedelta(minutes=FILL_WINDOW_MIN))
            cands = [p for p in positions if p["symbol"] == ch["symbol"] and p["side"] == ch["side"]
                     and lo <= p["entry_time"] <= hi and p["id"] not in used
                     and _near_limit(p["entry_price"], ch["prices"], max_fill_dev_pct)]
            # ai_managed=1 优先,同级取离 t0 最近的
            cands.sort(key=lambda p: (-p["ai_managed"], abs(p["entry_time"] - lo)))
            if cands:
                p = cands[0]
                used.add(p["id"])
                sig.update(outcome="filled", fill_px=p["entry_price"], position_id=p["id"],
                           fill_t=datetime.fromtimestamp(p["entry_time"] / 1000, timezone.utc).isoformat(),
                           sl=ch["sl_last"], tp=ch["tp_last"])
            else:
                sig.update(outcome="unfilled", fill_px=None, fill_t=None, sl=ch["sl_last"], tp=ch["tp_last"])
        out.append(sig)
    return out


def extract(conn, trader_id, since=None, until=None, dedupe_min=60, max_fill_dev_pct=0.5):
    decisions = raw_decisions(conn, trader_id, since, until)
    positions = load_positions(conn, trader_id)
    return build_signals(decisions, positions, dedupe_min, max_fill_dev_pct), len(decisions)


def load_signals_file(path):
    with open(path) as f:
        d = json.load(f)
    if isinstance(d, list):
        return {}, d
    return d.get("meta", {}), d["signals"]
