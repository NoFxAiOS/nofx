"""Backtrader 数据/策略适配。撮合、持仓、费用和盈亏均使用原生 broker。"""
from datetime import datetime, timedelta, timezone
import math


class BacktraderUnavailable(RuntimeError):
    pass


def _backtrader():
    try:
        import backtrader as bt
    except ImportError as exc:
        raise BacktraderUnavailable(
            "回测需要 Backtrader，不会回退到自写模拟器。请先安装并使用该环境运行：\n"
            "  uv venv .venv\n"
            "  uv pip install --python .venv/bin/python -r scripts/backtest/requirements.txt\n"
            "  .venv/bin/python -m scripts.backtest <command> ..."
        ) from exc
    return bt


def run_trade(tr, rules, commission=0.0005, slippage=0.0003):
    """独立信号研究：发出真实 Backtrader 订单，返回 broker 的净盈亏/R。

    不使用 cheat-on-close/open；本根收盘产生的指令最早于下一根执行。
    限价单在信号后固定有效期内撮合，不使用未来实盘成交标签决定是否开仓。
    """
    bt = _backtrader()

    class CachedOHLC(bt.feed.DataBase):
        lines = ("open_ms",)

        def start(self):
            super().start()
            self.rows = iter(self.p.dataname)

        def _load(self):
            row = next(self.rows, None)
            if row is None:
                return False
            close_ms, o, h, l, c, open_ms = row
            self.lines.datetime[0] = bt.date2num(datetime.fromtimestamp(close_ms / 1000, timezone.utc))
            self.lines.open[0], self.lines.high[0], self.lines.low[0], self.lines.close[0] = o, h, l, c
            self.lines.volume[0] = self.lines.openinterest[0] = 0
            self.lines.open_ms[0] = open_ms
            return True

    class SignalStrategy(bt.Strategy):
        def __init__(self):
            self.entry_order = None
            self.exits = []
            self.done = set()
            self.entry_price = self.risk = self.entry_ms = None
            self.peak = 0.0
            self.stop_r, self.stop_source = -1.0, "sl"
            self.how = "unfilled"
            self.exit_ms = None
            self.closing = False
            self.refresh = False
            self.commission_paid = 0.0
            self.mode = rules.get("force_mode") or tr["mode"]
            self.sign = 1 if tr["side"] == "long" else -1

        def _price(self, r):
            return self.entry_price + self.sign * r * self.risk

        def _cancel_exits(self):
            for order in self.exits:
                if order.alive():
                    self.cancel(order)
            self.exits = []

        def _place_exits(self):
            self._cancel_exits()
            size = abs(self.position.size)
            submit = self.sell if self.sign > 0 else self.buy
            # 原生 OCO：任何减仓成交后取消其余旧订单，下次回调用剩余数量重挂。
            stop = submit(size=size, exectype=bt.Order.Stop, price=self._price(self.stop_r))
            stop.addinfo(reason=self.stop_source)
            self.exits.append(stop)
            tp_r = abs(tr["tp"] - self.entry_price) / self.risk
            if rules["tp_cap"] > 0:
                tp_r = min(tp_r, rules["tp_cap"])
            levels = []
            if rules["use_tp"]:
                fraction = rules["tp_frac_trend"] if self.mode == "trend" and rules["trail"] else 1.0
                levels.append(("tp", tp_r, fraction))
            if rules["lock_r"] > 0 and rules["lock_trim"]:
                levels.append(("lock", rules["lock_r"], 0.5))
            if rules["trim_r"] > 0:
                levels.append(("trim", rules["trim_r"], rules["trim_frac"]))
            # 同根触及多档时先执行近档；其余档在后续 K 线上重新撮合。
            for reason, level, fraction in sorted(levels, key=lambda x: x[1]):
                if reason in self.done or fraction <= 0:
                    continue
                order = submit(size=size * fraction, exectype=bt.Order.Limit,
                               price=self._price(level), oco=stop)
                order.addinfo(reason=reason)
                self.exits.append(order)
            self.refresh = False

        def notify_order(self, order):
            if order.status == bt.Order.Completed:
                self.commission_paid += order.executed.comm
                if order.ref == self.entry_order.ref:
                    self.entry_price = order.executed.price
                    self.entry_ms = int(self.data.open_ms[0])
                    self.risk = abs(self.entry_price - tr["sl"])
                    self.how = "open"
                    # 跳空越过原 SL/TP 时提交市价平仓，让 broker 执行，不伪造填价。
                    if self.risk <= 0 or not (self.sign * (self.entry_price - tr["sl"]) > 0
                                             and self.sign * (tr["tp"] - self.entry_price) > 0):
                        self.closing = True
                        close = self.close()
                        close.addinfo(reason="invalid_fill")
                    else:
                        self.refresh = True
                else:
                    reason = order.info.get("reason", "time")
                    self.done.add(reason)
                    self.exit_ms = int(round(bt.num2date(order.executed.dt, tz=timezone.utc,
                                                         naive=False).timestamp() * 1000))
                    if not self.position:
                        self.how = reason
                        self._cancel_exits()
                    else:
                        self.refresh = True
            elif order.status in (bt.Order.Expired, bt.Order.Margin, bt.Order.Rejected, bt.Order.Canceled):
                if order.ref == self.entry_order.ref:
                    self.how = "unfilled" if order.status == bt.Order.Expired else "rejected"
                elif order.status in (bt.Order.Margin, bt.Order.Rejected):
                    raise RuntimeError(f"Backtrader 拒绝出场订单: {order.getstatusname()}")

        def next(self):
            now_ms = int(round(self.data.datetime.datetime(tz=timezone.utc, naive=False).timestamp() * 1000))
            if self.entry_order is None:
                if now_ms < tr["start_ms"]:
                    return
                submit = self.buy if self.sign > 0 else self.sell
                if tr["kind"] == "lim":
                    expiry = datetime.fromtimestamp(tr["t0_ms"] / 1000, timezone.utc) + timedelta(
                        minutes=tr["limit_valid_min"])
                    if now_ms >= int(expiry.timestamp() * 1000):
                        self.env.runstop()
                        return
                    self.entry_order = submit(size=1.0, exectype=bt.Order.Limit,
                                              price=tr["reference"], valid=expiry)
                else:
                    self.entry_order = submit(size=1.0, exectype=bt.Order.Market)
                return
            if not self.position:
                if not self.entry_order.alive():
                    self.env.runstop()
                return
            if self.closing:
                return
            if self.entry_ms is None:
                return
            favourable = self.data.high[0] if self.sign > 0 else self.data.low[0]
            self.peak = max(self.peak, self.sign * (favourable - self.entry_price) / self.risk)
            close_r = self.sign * (self.data.close[0] - self.entry_price) / self.risk
            elapsed_h = (now_ms - self.entry_ms) / 3_600_000
            timed_out = rules["horizon_h"] > 0 and elapsed_h >= rules["horizon_h"]
            quick_stop = (self.mode == "quick" and rules["quick_stop_h"] > 0
                          and elapsed_h >= rules["quick_stop_h"] and close_r <= 0)
            if timed_out or quick_stop:
                self._cancel_exits()
                close = self.close()
                close.addinfo(reason="time")
                self.closing = True
                return
            candidates = [(self.stop_r, self.stop_source)]
            if ((rules["be_arm"] > 0 and self.peak >= rules["be_arm"])
                    or (rules["lock_r"] > 0 and self.peak >= rules["lock_r"])):
                candidates.append((rules["be_off"], "be"))
            atr_r = tr["atr_pct"] / 100 * self.entry_price / self.risk
            if rules["trail"] and self.mode == "trend" and atr_r > 0 and self.peak >= rules["trail_arm"]:
                candidates.append((self.peak - rules["trail_atr"] * atr_r, "trail"))
            if rules["pd_arm"] > 0 and self.peak >= rules["pd_arm"]:
                candidates.append((self.peak * (1 - rules["pd_give"]), "pd"))
            stop_r, source = max(candidates, key=lambda x: x[0])
            if stop_r > self.stop_r:
                self.stop_r, self.stop_source, self.refresh = stop_r, source, True
            if self.refresh:
                self._place_exits()

    if not all(math.isfinite(v) and v >= 0 for v in (commission, slippage)):
        raise ValueError("手续费和滑点必须是非负有限数")
    if not math.isfinite(rules["horizon_h"]) or rules["horizon_h"] < 0:
        raise ValueError("持仓上限必须是非负有限数")
    for key in ("trim_frac", "tp_frac_trend"):
        if not 0 <= rules[key] <= 1:
            raise ValueError(f"{key} 必须在 [0, 1] 内")
    cerebro = bt.Cerebro(stdstats=False)
    cerebro.adddata(CachedOHLC(dataname=tr["bars"], timeframe=bt.TimeFrame.Minutes))
    cerebro.addstrategy(SignalStrategy)
    # 独立信号账户给予充足资金；只用信号当时已知价格，不读取未来高点设资金。
    initial_cash = tr["reference"] * 100
    cerebro.broker.setcash(initial_cash)
    cerebro.broker.setcommission(commission=commission, stocklike=True, percabs=True)
    cerebro.broker.set_slippage_perc(slippage, slip_open=True, slip_limit=False)
    cerebro.addanalyzer(bt.analyzers.TradeAnalyzer, _name="trades")
    cerebro.addanalyzer(bt.analyzers.DrawDown, _name="drawdown")
    strategy = cerebro.run(preload=False, runonce=False)[0]
    trade_analysis = strategy.analyzers.trades.get_analysis()
    # 已平仓用原生 TradeAnalyzer 的 PnL，避免小价格币种减去大现金余额损失精度。
    net_pnl = (trade_analysis["pnl"]["net"]["total"] if trade_analysis["total"].get("closed", 0)
               else cerebro.broker.getvalue() - initial_cash)
    risk = strategy.risk
    result = dict(engine="backtrader", engine_version=bt.__version__, how=strategy.how,
                  r=net_pnl / risk if risk and strategy.entry_price is not None else None,
                  peak=strategy.peak, entry_price=strategy.entry_price, entry_ms=strategy.entry_ms,
                  exit_ms=strategy.exit_ms, commission=strategy.commission_paid,
                  net_pnl=net_pnl, risk=risk, open=bool(strategy.position.size),
                  coverage_end_ms=tr["bars"][-1][0],
                  trade_analysis=trade_analysis,
                  drawdown=strategy.analyzers.drawdown.get_analysis())
    if result["open"]:
        result["exit_ms"] = max(tr["bars"][-1][0], int(strategy.entry_ms + rules["horizon_h"] * 3_600_000))
    return result
