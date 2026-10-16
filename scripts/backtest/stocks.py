"""Native Backtrader multi-asset stock portfolios; no alternate simulator."""
import math
from dataclasses import dataclass

from .backtrader_adapter import _backtrader
from .stocks_data import NAMES, SPY, death, golden

bt = _backtrader()
# Same negligible-dollar cutoff as the specification's initialEquity*1e-12.
MIN_NOTIONAL = 1e-7


@dataclass(frozen=True)
class Config:
    timeframe: str = '1d'
    max_positions: int = 10
    rebalance_trading_days: int = 10
    commission: float = 0.001
    cash: float = 100000

    def validate(self):
        if self.timeframe not in ('1d', '1h') or self.max_positions <= 0 or self.rebalance_trading_days <= 0:
            raise ValueError('tf must be 1d/1h; maxpos and rebalance must be positive')
        if not math.isfinite(self.commission) or not 0 <= self.commission < 1 or not math.isfinite(self.cash) or self.cash <= 0:
            raise ValueError('commission must be finite in [0,1); cash must be positive and finite')


class CashSlotSizer:
    """BackBroker filler hook: only cap size, broker still matches/accounts.

    Signal-close share sizes cannot anticipate gaps. Cap a buy using actual
    available broker cash at execution, after earlier sell orders, and native
    commission. No price override, synthetic fills, or portfolio ledger.
    A capped Market remainder is canceled on that same strategy callback.
    """
    def __init__(self, broker):
        self.broker = broker
        self.adjustments = []

    def __call__(self, order, price, ago):
        requested = abs(order.executed.remsize)
        if order.issell():
            return min(requested, max(0, self.broker.getposition(order.data).size))
        occupied = sum(p.size > 0 for p in self.broker.positions.values())
        if self.broker.getposition(order.data).size <= 0 and occupied >= order.info.limit:
            size = 0.0
        else:
            ci = self.broker.getcommissioninfo(order.data)
            unit = price + ci.getcommission(1, price)
            size = min(requested, max(0, self.broker.get_cash()) / unit * (1-1e-14))
            if size*price <= MIN_NOTIONAL:
                size = 0.0
        if size < requested*(1-1e-10):
            self.adjustments.append(dict(symbol=order.data._name, signal_time=order.info.signal_time,
                fill_time=str(bt.num2date(order.data.datetime[ago])), requested_size=requested,
                filled_size=size, price=price))
        return size


class PortfolioAudit(bt.Analyzer):
    """Observe native broker values; never alter cash, positions, or P&L."""
    def start(self):
        self.curve = []
        self.last = None

    def next(self):
        s = self.strategy
        key = s.current_key()
        if key not in s.p.study.calendar_index or key == self.last:
            return
        self.last = key
        value, cash = s.broker.getvalue(), s.broker.get_cash()
        self.curve.append(dict(time=str(key), equity=value, cash=cash,
            invested_weight=(value-cash)/value if value else 0,
            positions=sum(p.size > 0 for p in s.broker.positions.values())))

    def get_analysis(self):
        return self.curve


class StockPortfolio(bt.Strategy):
    params = (('study', None), ('config', None), ('name', 'MOM_TOP'))

    def __init__(self):
        self.feeds = {d._name: d for d in self.datas}
        self.spy = self.feeds[SPY]
        self.pending, self.metadata, self.trade_tags, self.last_exit = {}, {}, {}, {}
        self.fills, self.closed, self.decisions, self.order_events = [], [], [], []
        self.entry_counts, self.exit_counts = {}, {}
        self.commission, self.missing, self.last_key = 0.0, 0, None
        self.zero_fill = 0
        self.notified_sizes, self.notified_comm = {}, {}
        self.limit = (1 if self.p.name == 'SPY_BH' else len(self.p.study.ew)
                      if self.p.name == 'EW_BH' else self.p.config.max_positions)

    def current_key(self):
        # SPY is data0. Ignore union-clock callbacks on other feeds' dates.
        import pandas as pd
        if not len(self.spy) or self.spy.datetime[0] != self.datetime[0]:
            return None
        return pd.Timestamp(self.spy.datetime.datetime(0))

    def prenext(self):
        self.next()

    def next(self):
        key = self.current_key()
        study, cfg, name = self.p.study, self.p.config, self.p.name
        if key not in study.calendar_index or key == self.last_key:
            return
        self.last_key = key
        current = {s for s, d in self.feeds.items()
                   if len(d) and d.datetime[0] == self.datetime[0]}
        # A missing intended next-SPY open expires; no delayed fill/backfill.
        for ref, order in list(self.pending.items()):
            if order.alive():
                if not order.executed.size:
                    if order.data._name not in current:
                        self.missing += 1
                    else:
                        self.zero_fill += 1
                self.cancel(order)
            self.pending.pop(ref, None)
        positions = {s for s, d in self.feeds.items() if self.getposition(d).size > 0}
        j = study.calendar_index[key]
        rebalance = study.last_of_day[key] and study.days[key] % cfg.rebalance_trading_days == 0
        if name == 'EW_BH':
            rebalance = j % 63 == 0
        targets, exits, entries = None, {}, []
        if name in ('MOM_TOP', 'MOM_X_MA') and rebalance:
            targets = {}
            basket = [r for r in study.ranks[key] if r[1] and r[2]][:cfg.max_positions]
            for s, *_ in basket:
                h = study.histories[s]
                a = h.ind[h.index[key]]
                if name != 'MOM_X_MA' or a.fast > a.slow:
                    targets[s] = 1/cfg.max_positions
        elif name == 'SPY_BH' and j == 0:
            targets = {SPY: 1.0}
        elif name == 'EW_BH' and rebalance:
            targets = {s: 1/len(study.ew) for s in study.ew}
        for s in sorted(positions & current):
            h = study.histories[s]
            i = h.index[key]
            reason = ''
            if name in ('MA_CROSS', 'MOM_X_MA') and death(h.ind, i):
                reason = 'X1'
            elif name == 'MA2060_RULES':
                m, a, close = self.metadata[s], h.ind[i], h.bars[i]['close']
                reason = h.exits[i]
                if not reason and close < m['price']-2*m['atr']:
                    reason = 'X4'
                if not reason and m['entry'] == 'E4' and close >= a.fast:
                    reason = 'TARGET'
            if reason:
                exits[s] = reason
                if targets is not None:
                    targets.pop(s, None)
        if name in ('MA_CROSS', 'MA2060_RULES'):
            free = cfg.max_positions-len(positions)+len(exits)
            for s, *_ in study.ranks[key]:
                if free <= 0:
                    break
                if s in positions or s not in current:
                    continue
                h = study.histories[s]
                i = h.index[key]
                entry = ('E1' if golden(h.ind, i) else '') if name == 'MA_CROSS' else h.entries[i]
                if entry:
                    entries.append((s, entry, h.ind[i].atr))
                    free -= 1
        if targets is None and not exits and not entries:
            return
        self.decisions.append(dict(signal_time=str(key), rebalance=targets is not None,
            targets=[dict(symbol=s, weight=w) for s, w in (targets or {}).items()],
            orders=[dict(symbol=s, exit=x) for s, x in exits.items()]+
                   [dict(symbol=s, entry=e, entry_atr=a) for s, e, a in entries]))
        equity = self.broker.getvalue()
        # Size at the observable close; BackBroker fills fixed shares next open.
        # Estimate proceeds solely for allocation. Actual buying power is capped
        # again by CashSlotSizer using broker.get_cash() at execution.
        available = self.broker.get_cash()
        buys = []
        if targets is not None:
            for s in sorted(positions | set(targets)):
                if s not in current:
                    continue
                d = self.feeds[s]
                delta = equity*targets.get(s, 0)/d.close[0]-self.getposition(d).size
                if delta < -1e-10:
                    self.submit(s, -delta, key, exit=exits.get(s, 'REBALANCE'))
                    available += -delta*d.close[0]*(1-cfg.commission)
            for s in targets:  # momentum order, not alphabetical order
                if s in current:
                    d = self.feeds[s]
                    delta = equity*targets[s]/d.close[0]-self.getposition(d).size
                    if delta > 1e-10:
                        buys.append((s, delta, 'REBALANCE', 0.0))
        else:
            for s, reason in sorted(exits.items()):
                d = self.feeds[s]
                size = self.getposition(d).size
                self.submit(s, size, key, exit=reason)
                available += size*d.close[0]*(1-cfg.commission)
            buys = [(s, equity/cfg.max_positions/self.feeds[s].close[0], e, a) for s, e, a in entries]
        for s, wanted, e, a in buys:
            unit = self.feeds[s].close[0]*(1+cfg.commission)
            size = min(wanted, max(0, available)/unit)
            if size*self.feeds[s].close[0] > MIN_NOTIONAL:
                self.submit(s, size, key, entry=e, atr=a)
                available -= size*unit

    def submit(self, symbol, size, key, entry='', exit='', atr=0.0):
        info = dict(entry=entry, exit=exit, atr=atr, limit=self.limit, signal_time=str(key))
        fn = self.buy if entry else self.sell
        order = fn(data=self.feeds[symbol], size=size, exectype=bt.Order.Market, **info)
        self.pending[order.ref] = order
        return order

    def notify_order(self, order):
        if order.status in (order.Submitted, order.Accepted):
            return
        s, ref = order.data._name, order.ref
        self.order_events.append(dict(symbol=s, signal_time=order.info.signal_time,
                                      status=order.getstatusname()))
        delta = order.executed.size-self.notified_sizes.get(ref, 0)
        comm = order.executed.comm-self.notified_comm.get(ref, 0)
        self.notified_sizes[ref], self.notified_comm[ref] = order.executed.size, order.executed.comm
        self.commission += comm
        if delta:
            self.fills.append(dict(symbol=s, signal_time=order.info.signal_time,
                time=str(bt.num2date(order.executed.dt).replace(tzinfo=None)), size=delta,
                price=order.executed.price, commission=comm, entry=order.info.entry,
                exit=order.info.exit, entry_atr=order.info.atr))
            if order.isbuy() and s not in self.metadata:
                self.metadata[s] = dict(entry=order.info.entry, atr=order.info.atr,
                    price=order.executed.price, basis=delta*order.executed.price+comm)
                self.getposition(order.data).info = dict(self.metadata[s])
                self.entry_counts[order.info.entry] = self.entry_counts.get(order.info.entry, 0)+1
            elif order.issell():
                self.last_exit[s] = order.info.exit
                self.exit_counts[order.info.exit] = self.exit_counts.get(order.info.exit, 0)+1
        if order.status in (order.Completed, order.Canceled, order.Margin, order.Rejected, order.Expired):
            self.pending.pop(ref, None)

    def notify_trade(self, trade):
        s = trade.data._name
        if trade.justopened:
            self.trade_tags[trade.ref] = dict(self.metadata[s])
        if trade.isclosed:
            m = self.trade_tags.pop(trade.ref)
            self.closed.append(dict(symbol=s, entry_type=m['entry'], exit_type=self.last_exit.get(s, ''),
                entry_time=str(bt.num2date(trade.dtopen).replace(tzinfo=None)),
                exit_time=str(bt.num2date(trade.dtclose).replace(tzinfo=None)),
                entry_price=trade.price, entry_atr=m['atr'], net_pnl=trade.pnlcomm,
                costs=trade.commission,
                net_return=trade.pnlcomm/m['basis'] if m['entry'] != 'REBALANCE' else None,
                bars_held=trade.barlen,
                holding_calendar_days=trade.dtclose-trade.dtopen))
            self.metadata.pop(s, None)
            self.getposition(trade.data).info = {}


def run_strategy(study, cfg, name):
    from .stocks_report import compute_metrics, rule_breakdown
    cfg.validate()
    if name not in NAMES or cfg.timeframe != study.tf:
        raise ValueError('unknown strategy or study/config timeframe mismatch')
    cerebro = bt.Cerebro(stdstats=False, cheat_on_open=False)
    cerebro.broker.setcash(cfg.cash)
    cerebro.broker.setcommission(commission=cfg.commission)
    cerebro.broker.set_slippage_perc(0.0)
    cerebro.broker.set_coc(False)
    cerebro.broker.set_coo(False)
    # Skip pseudo-fill cash rejection: execution-size cap handles gaps and
    # allows same-bar sells to fund buys, with native cash checks still active.
    cerebro.broker.set_checksubmit(False)
    sizing = CashSlotSizer(cerebro.broker)
    cerebro.broker.set_filler(sizing)
    spy_calendar = study.histories[SPY].frame.index
    for s in [SPY]+sorted(set(study.histories)-{SPY}):
        h = study.histories[s]
        frame = h.frame.loc[h.frame.index.isin(spy_calendar)]
        if frame.empty:
            continue
        cerebro.adddata(bt.feeds.PandasData(dataname=frame, openinterest=None,
            timeframe=bt.TimeFrame.Days if study.tf == '1d' else bt.TimeFrame.Minutes,
            compression=1 if study.tf == '1d' else 60), name=s)
    cerebro.addstrategy(StockPortfolio, study=study, config=cfg, name=name)
    cerebro.addanalyzer(bt.analyzers.TimeReturn, _name='returns', timeframe=bt.TimeFrame.Days)
    cerebro.addanalyzer(bt.analyzers.DrawDown, _name='drawdown')
    cerebro.addanalyzer(bt.analyzers.TradeAnalyzer, _name='trades')
    cerebro.addanalyzer(PortfolioAudit, _name='portfolio')
    strategy = cerebro.run(runonce=False)[0]
    analyzers = dict(time_return={str(k): v for k, v in strategy.analyzers.returns.get_analysis().items()
                                 if k.date() >= study.calendar[0].date()},
                     drawdown=dict(strategy.analyzers.drawdown.get_analysis()),
                     trade_analyzer=strategy.analyzers.trades.get_analysis())
    curve = strategy.analyzers.portfolio.get_analysis()
    result = dict(name=name, metrics=compute_metrics(curve, analyzers, strategy.commission),
        equity_curve=curve, decisions=strategy.decisions, closed_trades=strategy.closed,
        fills=strategy.fills, order_events=strategy.order_events, analyzers=analyzers,
        exit_order_counts=strategy.exit_counts, entry_order_counts=strategy.entry_counts,
        missing_open_orders=strategy.missing, cash_or_slot_zero_fill_orders=strategy.zero_fill,
        cash_size_adjustments=sizing.adjustments,
        open_positions_at_end=sum(p.size > 0 for p in strategy.broker.positions.values()),
        pending_orders_at_end=sum(o.alive() for o in strategy.pending.values()))
    if name == 'MA2060_RULES':
        result['entry_breakdown'] = rule_breakdown(strategy.closed, 'entry_type', ('E1', 'E2', 'E3', 'E4'))
        result['exit_breakdown'] = rule_breakdown(strategy.closed, 'exit_type', ('X1', 'X2', 'X3', 'X4', 'TARGET'))
    return result


def run_study(study, cfg, names=NAMES, progress=None):
    from dataclasses import asdict
    cfg.validate()
    if not names or any(n not in NAMES for n in names) or len(set(names)) != len(names):
        raise ValueError('strategies must be a nonempty, unique comma-separated list of known names')
    results = []
    for name in names:
        if progress:
            progress(f'running {name} ({cfg.timeframe})')
        results.append(run_strategy(study, cfg, name))
    return dict(engine='backtrader', engine_version=bt.__version__, config=asdict(cfg),
                coverage=study.coverage, strategies=results)
