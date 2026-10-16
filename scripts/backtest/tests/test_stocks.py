import copy
import io
import json
import math
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest.mock import patch

import pandas as pd

from scripts.backtest.cli import build_parser, main
from scripts.backtest.stocks import Config, bt, run_strategy, run_study
from scripts.backtest.stocks_data import (History, Study, SPY, bullish_key, death, entry_rule,
    exit_rule, golden, indicators, load_study, rank_at)
from scripts.backtest.stocks_report import comparison_table, cross_check, render_report, return_stats


def bars(n, price=lambda i: 100):
    return [dict(open_time=str(t), open=(c := price(i)), high=c+1, low=c-1, close=c, volume=100)
            for i, t in enumerate(pd.date_range('2020-01-01', periods=n))]


def replace(b, i, o, h, l, c):
    b[i].update(open=o, high=h, low=l, close=c)


def rising(n):
    return bars(n, lambda i: 100+0.2*i)


def fixture():
    b = bars(360, lambda i: 80 if i >= 300 else 120 if i >= 270 else 100)
    replace(b, 271, 137, 138, 119, 120)
    return {'AAABUSDT': b}, rising(360)


def study_of(stocks, spy, tf='1d', daily=None, spy_daily=None):
    execution = {s: History(b, tf) for s, b in stocks.items()}
    execution[SPY] = History(spy, tf)
    d = {s: History(b) for s, b in (daily or stocks).items()}
    d[SPY] = History(spy_daily or spy)
    return Study(execution, d, tf)


class StockSignalTest(unittest.TestCase):
    def test_constructed_cross_and_wilder_atr(self):
        b = bars(130, lambda i: 80 if i >= 100 else 120 if i >= 80 else 100)
        v = indicators(b)
        self.assertEqual((v[79].fast, v[79].slow, v[13].atr), (100, 100, 2))
        self.assertAlmostEqual(v[80].atr, (2*13+21)/14)
        self.assertTrue(golden(v, 80))
        self.assertFalse(golden(v, 79) or golden(v, 81))
        self.assertEqual(len([i for i in range(130) if death(v, i)]), 1)
        self.assertTrue(math.isnan(v[12].atr) and math.isnan(v[18].fast) and math.isnan(v[58].slow))

    def test_each_entry_rule_and_engulfing(self):
        e1 = bars(81, lambda i: 110 if i == 80 else 100)
        e2 = rising(90)
        replace(e2, 89, e2[89]['close']-0.5, e2[89]['close']+1, indicators(e2)[89].fast*1.001, e2[89]['close'])
        e3 = rising(90)
        replace(e3, 88, 115, 115.5, 113.5, 114)
        replace(e3, 89, 118, 119, 117, 117.8)
        e4 = bars(82)
        replace(e4, 80, 76, 77, 74, 75)
        replace(e4, 81, 78, 81, 77, 80)
        for rule, b in [('E1', e1), ('E2', e2), ('E3', e3), ('E4', e4)]:
            with self.subTest(rule=rule):
                self.assertEqual(entry_rule(b, indicators(b), len(b)-1), rule)
        replace(e4, 80, 76, 90, 74, 75)
        replace(e4, 81, 74.5, 81, 74, 80)
        self.assertTrue(bullish_key(e4, 81))
        self.assertEqual(entry_rule(e4, indicators(e4), 81), 'E4')
        # Golden cross precedes a simultaneous pullback (E2).
        replace(e1, 80, 100, 111, 99, 110)
        self.assertEqual(entry_rule(e1, indicators(e1), 80), 'E1')

    def test_each_exit_rule_and_target_priority(self):
        x1 = bars(81, lambda i: 90 if i == 80 else 100)
        x2 = bars(81)
        replace(x2, 70, 120, 121, 119, 120)
        replace(x2, 80, 96, 97, 94, 95)
        x3 = bars(90, lambda i: 200-0.5*i)
        x4 = rising(90)
        for rule, b, price, atr in [('X1', x1, 100, 10), ('X2', x2, 100, 10),
                                     ('X3', x3, 100, 10), ('X4', x4, 130, 2)]:
            with self.subTest(rule=rule):
                self.assertEqual(exit_rule(b, indicators(b), len(b)-1, price, atr), rule)
        target = bars(81)
        self.assertEqual(exit_rule(target, indicators(target), 80, 75, 5, True), 'TARGET')
        self.assertEqual(exit_rule(target, indicators(target), 80, 75, 5, False), '')
        self.assertEqual(exit_rule(x1, indicators(x1), 80, 100, 1, True), 'X1')
        # X4 precedes TARGET, and the stop comparison is strict.
        self.assertEqual(exit_rule(target, indicators(target), 80, 110, 2, True), 'X4')
        self.assertEqual(exit_rule(target, indicators(target), 80, 104, 2), '')

    def test_future_mutation_does_not_change_signals_or_daily_rank(self):
        b = rising(340)
        replace(b, 280, 100, 101, 99, 100)
        changed = copy.deepcopy(b)
        for i in range(300, len(changed)):
            replace(changed, i, 999, 1000, 998, 999)
        h1, h2 = History(b), History(changed)
        self.assertEqual(h1.ind[:300], h2.ind[:300])
        self.assertEqual(h1.entries[:300], h2.entries[:300])
        self.assertEqual(h1.exits[:300], h2.exits[:300])
        key = h1.keys[299]
        self.assertEqual(rank_at('A', h1, key, '1d'), rank_at('A', h2, key, '1d'))
        # Today's completed DAILY candle must be unavailable to hourly signals.
        replace(changed, 299, 999, 1000, 998, 999)
        r = rank_at('A', h1, key+pd.Timedelta(hours=15), '1h')
        self.assertEqual(r, rank_at('A', History(changed), key+pd.Timedelta(hours=15), '1h'))
        self.assertAlmostEqual(r[3], b[277]['close']/b[46]['close']-1)


class StockExecutionTest(unittest.TestCase):
    def test_native_next_open_and_hand_computed_commission(self):
        class Toy(bt.Strategy):
            def __init__(self):
                self.fills = []

            def next(self):
                if len(self) == 1:
                    self.buy(size=5)
                if len(self) == 3:
                    self.close()

            def notify_order(self, order):
                if order.status == order.Completed:
                    self.fills.append((order.executed.dt, order.executed.price, order.executed.comm))

        b = bars(5)
        replace(b, 1, 110, 112, 109, 111)
        replace(b, 3, 140, 142, 139, 141)
        cerebro = bt.Cerebro(stdstats=False)
        cerebro.broker.setcash(1000)
        cerebro.broker.setcommission(commission=.001)
        cerebro.adddata(bt.feeds.PandasData(dataname=History(b).frame))
        cerebro.addstrategy(Toy)
        cerebro.addanalyzer(bt.analyzers.TradeAnalyzer, _name='trades')
        s = cerebro.run()[0]
        self.assertEqual([x[1] for x in s.fills], [110, 140])
        self.assertEqual(bt.num2date(s.fills[0][0]).date(), pd.Timestamp(b[1]['open_time']).date())
        self.assertAlmostEqual(s.fills[0][2], .55)
        self.assertAlmostEqual(s.fills[1][2], .70)
        self.assertAlmostEqual(s.broker.getvalue(), 1000+5*(140-110)-.55-.70)
        self.assertEqual(s.analyzers.trades.get_analysis()['total']['closed'], 1)

    def test_slot_limit_momentum_priority_and_gap_cash_cap(self):
        stocks, spy = fixture()
        b = copy.deepcopy(stocks['AAABUSDT'])
        for i in range(50):
            replace(b, i, 50, 51, 49, 50)
        stocks['BBBBUSDT'] = b
        stocks['000BUSDT'] = copy.deepcopy(b[200:])  # invalid daily rank, still has a cross
        study = study_of(stocks, spy)
        for slots in (1, 2):
            s = run_strategy(study, Config(max_positions=slots), 'MA_CROSS')
            buys = [f for f in s['fills'] if f['size'] > 0]
            self.assertEqual([f['symbol'] for f in buys], ['BBBBUSDT', 'AAABUSDT'][:slots])
            self.assertEqual(buys[0]['price'], 137)
            self.assertEqual(buys[0]['time'], str(study.histories[SPY].keys[271]))
            self.assertTrue(all(p['positions'] <= slots and p['cash'] >= -1e-8 and p['invested_weight'] <= 1+1e-10
                                for p in s['equity_curve']))
            self.assertEqual(s['metrics']['closed_trades'], slots)
            self.assertAlmostEqual(s['metrics']['total_commission'], sum(f['commission'] for f in s['fills']))
            self.assertGreater(len(s['cash_size_adjustments']), 0)

    def test_late_feed_does_not_delay_prenext_or_trade_start(self):
        stocks, spy = fixture()
        stocks['LATEBUSDT'] = rising(360)[330:]
        s = run_strategy(study_of(stocks, spy), Config(), 'MA_CROSS')
        self.assertEqual(s['equity_curve'][0]['time'], str(pd.Timestamp(spy[252]['open_time'])))
        self.assertEqual(s['fills'][0]['time'], str(pd.Timestamp(spy[271]['open_time'])))

    def test_cash_roundoff_does_not_create_dust_positions(self):
        stocks, spy = fixture()
        replace(stocks['AAABUSDT'], 271, 240, 241, 119, 120)
        stocks['B'] = bars(360, lambda i: 120 if i >= 274 else 100)
        s = run_strategy(study_of(stocks, spy), Config(max_positions=2), 'MA_CROSS')
        buys = [f for f in s['fills'] if f['size'] > 0]
        self.assertEqual([f['symbol'] for f in buys], ['AAABUSDT'])
        self.assertTrue(all(f['size']*f['price'] > 1e-7 for f in buys))

    def test_missing_next_open_cancels_without_rank_backfill(self):
        stocks, spy = fixture()
        b = copy.deepcopy(stocks['AAABUSDT'])
        for i in range(50):
            replace(b, i, 50, 51, 49, 50)
        stocks['BBBBUSDT'] = b[:271]+b[272:]
        s = run_strategy(study_of(stocks, spy), Config(max_positions=1), 'MA_CROSS')
        self.assertEqual(s['decisions'][0]['orders'][0]['symbol'], 'BBBBUSDT')
        self.assertFalse(s['fills'])
        self.assertEqual(s['missing_open_orders'], 1)
        self.assertIn('Canceled', [o['status'] for o in s['order_events']])

    def test_e4_target_and_x4_use_actual_entry_and_signal_atr(self):
        b = bars(280)
        replace(b, 270, 76, 77, 74, 75)
        replace(b, 271, 78, 81, 77, 80)
        replace(b, 272, 82, 101, 81, 100)
        replace(b, 273, 99, 101, 98, 100)
        study = study_of({'A': b}, rising(280))
        s = run_strategy(study, Config(max_positions=1), 'MA2060_RULES')
        tr = s['closed_trades'][0]
        self.assertEqual((tr['entry_type'], tr['exit_type']), ('E4', 'TARGET'))
        self.assertEqual([f['price'] for f in s['fills'][:2]], [82, 99])
        self.assertEqual(tr['entry_atr'], study.histories['A'].ind[271].atr)
        self.assertEqual(s['entry_breakdown']['E4']['closed_trades'], 1)
        self.assertEqual(s['exit_order_counts']['TARGET'], 1)
        stocks, spy = fixture()
        study = study_of(stocks, spy)
        s = run_strategy(study, Config(max_positions=1), 'MA2060_RULES')
        tr = s['closed_trades'][0]
        self.assertEqual(tr['entry_price'], 137)
        self.assertEqual(tr['exit_type'], 'X4')
        self.assertEqual(tr['entry_atr'], study.histories['AAABUSDT'].ind[270].atr)

    def test_sells_fund_buys_and_missing_exit_cannot_overfill_slots(self):
        stocks, spy = fixture()
        a = stocks['AAABUSDT']
        v = indicators(a)
        d = next(i for i in range(271, len(a)) if death(v, i))
        stocks['B'] = bars(360, lambda i: 120 if i >= d else 100)
        s = run_strategy(study_of(stocks, spy), Config(max_positions=1), 'MA_CROSS')
        fills = [f for f in s['fills'] if f['time'] == str(pd.Timestamp(spy[d+1]['open_time']))]
        self.assertEqual([f['symbol'] for f in fills], ['AAABUSDT', 'B'])
        self.assertLess(fills[0]['size'], 0)
        self.assertGreater(fills[1]['size'], 0)
        stocks['AAABUSDT'] = a[:d+1]+a[d+2:]
        s = run_strategy(study_of(stocks, spy), Config(max_positions=1), 'MA_CROSS')
        self.assertFalse(any(f['symbol'] == 'B' for f in s['fills']))
        self.assertEqual(s['missing_open_orders'], 1)
        self.assertEqual(s['cash_or_slot_zero_fill_orders'], 1)
        self.assertTrue(all(p['positions'] <= 1 for p in s['equity_curve']))

    def test_momentum_timing_veto_without_backfill_or_between_rebalance_entry(self):
        spy = rising(300)
        a_daily = rising(300)
        for i in range(50):
            replace(a_daily, i, 50, 51, 49, 50)
        # Best daily momentum A has bearish execution MAs; B must not replace it.
        study = study_of({'A': bars(300, lambda i: 300-.2*i), 'B': rising(300)}, spy,
                         daily={'A': a_daily, 'B': rising(300)})
        top = run_strategy(study, Config(max_positions=1), 'MOM_TOP')
        timing = run_strategy(study, Config(max_positions=1), 'MOM_X_MA')
        self.assertEqual(top['decisions'][0]['targets'], [dict(symbol='A', weight=1)])
        self.assertEqual(timing['decisions'][0]['targets'], [])
        self.assertFalse(timing['fills'])
        stocks, spy = fixture()
        timing = run_strategy(study_of(stocks, spy), Config(max_positions=1, rebalance_trading_days=20), 'MOM_X_MA')
        self.assertTrue(any(not d['rebalance'] and any(o.get('exit') == 'X1' for o in d['orders']) for d in timing['decisions']))
        self.assertTrue(all(not any(o.get('entry') for o in d['orders']) for d in timing['decisions']))

    def test_ew_fixed_initial_universe_and_63_bar_schedule(self):
        stocks, spy = fixture()
        stocks['B'] = copy.deepcopy(stocks['AAABUSDT'])
        stocks['IPO'] = copy.deepcopy(stocks['AAABUSDT'][260:])
        s = run_strategy(study_of(stocks, spy), Config(max_positions=1), 'EW_BH')
        self.assertEqual(s['equity_curve'][1]['positions'], 2)
        self.assertTrue(all({t['symbol'] for t in d['targets']} == {'AAABUSDT', 'B'} for d in s['decisions']))
        self.assertEqual(pd.Timestamp(s['decisions'][1]['signal_time'])-pd.Timestamp(s['decisions'][0]['signal_time']), pd.Timedelta(days=63))

    def test_hourly_session_end_rebalance_and_daily_return_analyzer(self):
        d = rising(340)
        hourly = []
        for day in range(260, 320):
            for hour in range(14, 17):
                b = dict(d[day], open_time=str(pd.Timestamp(d[day]['open_time'])+pd.Timedelta(hours=hour)))
                hourly.append(b)
        study = study_of({'A': hourly}, hourly, '1h', daily={'A': d}, spy_daily=d)
        r = run_study(study, Config(timeframe='1h', max_positions=1, rebalance_trading_days=2), ['MOM_TOP', 'EW_BH'])
        s, ew = r['strategies']
        dates = [pd.Timestamp(x['signal_time']) for x in s['decisions']]
        self.assertTrue(all(t.hour == 16 for t in dates))
        self.assertTrue(all(b-a == pd.Timedelta(days=2) for a, b in zip(dates, dates[1:])))
        self.assertEqual(len(ew['decisions']), 2)
        self.assertEqual(pd.Timestamp(ew['decisions'][1]['signal_time'])-pd.Timestamp(ew['decisions'][0]['signal_time']), pd.Timedelta(days=21))
        self.assertLess(len(s['analyzers']['time_return']), len(s['equity_curve']))
        self.assertEqual(len(s['equity_curve']), len(study.calendar))


class StockCacheReportCliTest(unittest.TestCase):
    def test_offline_cache_cleaning_exclusions_and_cli_reports(self):
        stocks, spy = fixture()
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            for s, b in dict(stocks, SPYBUSDT=spy, QQQBUSDT=spy, TQQQBUSDT=spy).items():
                (root/(s+'.json')).write_text(json.dumps(dict(requested_bars=1300,
                    series=dict(symbol=s, timeframe='1d', source='yahoo', bars=b))))
            # Invalid duplicate must not replace the latest valid duplicate.
            b = copy.deepcopy(stocks['AAABUSDT'])
            duplicate = dict(b[0], close=101)
            duplicate['high'] = 102
            h = History(b+[duplicate, dict(duplicate, low=-1)])
            self.assertEqual(h.bars[0]['close'], 101)
            self.assertEqual(h.removed, 2)
            with patch('socket.socket', side_effect=AssertionError('network forbidden')):
                study = load_study(root)
                self.assertEqual(set(study.histories), {SPY, 'AAABUSDT'})
                output = io.StringIO()
                with redirect_stdout(output):
                    self.assertEqual(main(['stocks', '--cache', folder, '--strategies', 'MA_CROSS,MA2060_RULES,SPY_BH',
                        '--out', str(root/'report.md'), '--json', str(root/'result.json')]), 0)
            report = (root/'report.md').read_text()
            result = json.loads((root/'result.json').read_text())
            self.assertEqual(result['engine'], 'backtrader')
            self.assertIn('MA2060 rule breakdown', report)
            self.assertIn('Survivorship', report)
            self.assertIn('LLM', report)
            self.assertIn('no slippage', report)
            self.assertEqual(result['strategies'][-1]['metrics']['closed_trades'], 0)
            self.assertEqual(result['strategies'][-1]['open_positions_at_end'], 1)
            self.assertTrue(result['strategies'][-1]['metrics']['last_12_months_partial'])
            self.assertAlmostEqual(result['strategies'][-1]['metrics']['total_return'],
                result['strategies'][-1]['metrics']['calendar_year_returns']['2020'])

    def test_parser_validation_missing_framework_and_no_nan_json(self):
        a = build_parser().parse_args(['stocks'])
        self.assertEqual((a.tf, a.maxpos, a.rebalance, a.commission, a.cash), ('1d', 10, 10, .001, 100000))
        with redirect_stderr(io.StringIO()):
            for flags in (['--maxpos', '0'], ['--cash', 'nan'], ['--commission', '-1'], ['--rebalance', '0']):
                self.assertEqual(main(['stocks']+flags), 2)
            with patch.dict('sys.modules', {'backtrader': None}):
                self.assertEqual(main(['stocks']), 2)
        stocks, spy = fixture()
        result = run_study(study_of(stocks, spy), Config(), ['MA_CROSS'])
        with self.assertRaises(ValueError):
            run_study(study_of(stocks, spy), Config(), ['UNKNOWN'])
        json.dumps(result, allow_nan=False)

    def test_cross_check_and_known_daily_statistics(self):
        self.assertIsNone(return_stats([0, 0])['sharpe'])
        self.assertAlmostEqual(return_stats([.1, -.1])['annualized_vol'], math.sqrt(.02)*math.sqrt(252))
        stocks, spy = fixture()
        result = run_study(study_of(stocks, spy), Config(), ['MA_CROSS'])
        m = result['strategies'][0]['metrics']
        g = dict(name='MA_CROSS', metrics=dict(cagr=m['cagr'], sharpe=m['sharpe'], max_drawdown=m['max_drawdown'],
                 closed_lots=m['closed_trades']+3, total_costs=m['total_commission']),
                 equity_curve=result['strategies'][0]['equity_curve'])
        cov = copy.deepcopy(result['coverage'])
        cov['symbols'].pop(SPY)
        go = dict(config=dict(timeframe='1d', max_positions=10, rebalance_trading_days=10, cost_bps_per_side=10),
                  coverage=cov, strategies=[g])
        result['cross_check'] = cross_check(result, go)
        self.assertFalse(result['cross_check']['configuration_mismatches'])
        self.assertEqual(result['cross_check']['rows'][0]['difference']['closed_trades'], -3)
        self.assertIn('Go lots', comparison_table(result['cross_check']))
        self.assertIn('Cross-check vs Go engine', render_report(result))


if __name__ == '__main__':
    unittest.main()
