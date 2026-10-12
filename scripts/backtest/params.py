"""从线上策略 risk_control 推导回测基线参数(语义均对照 Go 代码核对过)。"""

# trader/auto_trader_vol.go 里的代码常量
TRAIL_ARM_R = 1.5
TRAIL_ATR_MULT = 2.0

# 参数含义(R 空间,以初始止损距离为 1R):
#   be_arm       保本档触发 peak(R),<=0 关闭
#   be_off       保本/锁盈把止损抬到的 R 位置
#   lock_r       盈利锁触发 R,<=0 关闭;触发后止损抬到 be_off
#   lock_trim    盈利锁触发时是否同时减仓 50%(一次)
#   trim_r       R 减仓档触发 R,<=0 关闭;trim_frac 为减仓比例
#   tp_cap       止盈上限 R,<=0 不截断
#   tp_frac_trend 趋势模式且开追踪止损时,止盈位平仓比例(其余情况 1.0)
#   trail/trail_arm/trail_atr  追踪止损(仅趋势模式)
#   pd_arm/pd_give 峰值回撤(R 模式),pd_arm<=0 关闭
#   quick_stop_h quick 模式时间止损小时数,<=0 关闭
#   force_mode   强制所有信号使用该 exit_mode;use_tp 是否挂止盈;horizon_h 持仓上限小时
FALLBACK = dict(
    be_arm=0.75, be_off=0.1, lock_r=0.0, lock_trim=False, trim_r=1.2, trim_frac=1 / 3,
    tp_cap=2.0, tp_frac_trend=0.5, trail=True, trail_arm=TRAIL_ARM_R, trail_atr=TRAIL_ATR_MULT,
    pd_arm=2.0, pd_give=0.5, quick_stop_h=4, force_mode=None, use_tp=True, horizon_h=72,
)


def _f(rc, key, default=0.0):
    v = rc.get(key)
    return default if v is None else float(v)


def derive_baseline(rc, horizon_h=72):
    """rc: strategies.config 里的 risk_control dict。返回 (params, warnings)。"""
    warns = []
    p = dict(FALLBACK)
    p["horizon_h"] = horizon_h

    be = _f(rc, "breakeven_arm_r")
    p["be_arm"] = be if be > 0 else 0.0

    lock = _f(rc, "profit_lock_at_r")
    p["lock_r"] = 0.0 if lock < 0 else (1.0 if lock == 0 else lock)

    off = _f(rc, "profit_lock_be_offset_r")
    p["be_off"] = 0.0 if off < 0 else (0.2 if off == 0 else off)

    # 锁盈减仓 50%:仅当 tp_trim_yields_to_lock 为 null/true
    yl = rc.get("tp_trim_yields_to_lock")
    p["lock_trim"] = bool(p["lock_r"] > 0 and (yl is None or yl))

    trim = _f(rc, "tp_trim_at_r")
    if trim > 0:
        p["trim_r"], p["trim_frac"] = trim, 1 / 3
    else:
        p["trim_r"], p["trim_frac"] = 0.0, 1 / 3
        if trim == 0 and _f(rc, "tp_trim_profit_pct") >= 0:
            warns.append("tp_trim_at_r=0:线上回退到 ROE 减仓档(tp_trim_profit_pct),回测未建模,按关闭处理")

    cap = _f(rc, "tp_max_r")
    p["tp_cap"] = cap if cap >= 1 else 0.0

    p["trail"] = bool(rc.get("trailing_stop_enabled"))
    p["trail_arm"], p["trail_atr"] = TRAIL_ARM_R, TRAIL_ATR_MULT

    tcf = _f(rc, "tp_close_fraction")
    frac = 0.5 if tcf == 0 else (1.0 if tcf < 0 else tcf)
    p["tp_frac_trend"] = frac if p["trail"] else 1.0

    pd_arm = _f(rc, "peak_drawdown_arm_r")
    if pd_arm > 0:
        give = _f(rc, "peak_drawdown_giveback_r")
        p["pd_arm"], p["pd_give"] = pd_arm, (give if give > 0 else 0.5)
    else:
        p["pd_arm"], p["pd_give"] = 0.0, 0.5
        warns.append("peak_drawdown_arm_r<=0:线上使用 ROE 旧版峰值回撤,回测未建模,按关闭处理")

    ts = _f(rc, "time_stop_hours")
    p["quick_stop_h"] = 4 if ts == 0 else (0 if ts < 0 else ts)
    return p, warns


def describe(p):
    keys = ["be_arm", "be_off", "lock_r", "lock_trim", "trim_r", "trim_frac", "tp_cap", "tp_frac_trend",
            "trail", "trail_arm", "trail_atr", "pd_arm", "pd_give", "quick_stop_h", "horizon_h"]
    return ", ".join(f"{k}={p[k]:.4g}" if isinstance(p[k], float) else f"{k}={p[k]}" for k in keys)
