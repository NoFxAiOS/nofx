"""Offline formula, causality, native execution and overlay report regressions."""
import copy
import io
import json
import math
import statistics
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import pandas as pd

from scripts.backtest.cli import main
from scripts.backtest.stocks import Config, add_regime_runs, run_strategy, run_study
from scripts.backtest.stocks_data import History, SPY
from scripts.backtest.stocks_regime import (SCALERS, breadth_scale, build_features, compute_scales,
    load_macro, percentile, ratio_scale, rv_pct_scale, sweep_parameters, vix_scale)
from scripts.backtest.stocks_report import drawdown_episodes, regime_analysis, render_report
from scripts.backtest.tests.test_stocks import bars, replace, rising, study_of


class RegimeFormulaTest(unittest.TestCase):
    def test_continuous_ramps_clips_and_percentile_convention(self):
        for p, expected in ((0, 1), (.5, 1), (.75, .625), (1, .25)):
            self.assertAlmostEqual(rv_pct_scale(p, .25), expected)
        for p, expected in ((0, .25), (.25, .625), (.5, 1), (1, 1)):
            self.assertAlmostEqual(breadth_scale(p, .25), expected)
        for reference, current, expected in ((.2, .4, .5), (.2, 2, .25), (.4, .2, 1), (0, 0, 1)):
            self.assertEqual(ratio_scale(reference, current, .25), expected)
        for r, expected in ((.8, 1), (.9, 1), (1, .625), (1.1, .25), (1.2, .25)):
            self.assertAlmostEqual(vix_scale(r, .25), expected)
        self.assertEqual(rv_pct_scale(math.nan, .25), 1)
        self.assertEqual(breadth_scale(math.nan, .25), 1)
        self.assertEqual(vix_scale(math.nan, .25), 1)
        self.assertEqual(ratio_scale(.4, math.nan, .25), 1)
        x = pd.Series(range(300), dtype=float)
        p = percentile(x)
        self.assertTrue(math.isnan(p.iloc[124]))
        self.assertEqual(p.iloc[125], 1)
        self.assertEqual(p.iloc[-1], 1)
        # Ties use average rank; the current observation is included.
        self.assertAlmostEqual(percentile(pd.Series([1.0]*126)).iloc[-1], 63.5/126)
        self.assertEqual(percentile(pd.Series(list(range(126, 252))+list(range(126)))).iloc[-1], .5)

    def test_scale_dispatch_hard_thresholds_and_combo(self):
        dates = pd.date_range('2020-01-01', periods=4)
        study = SimpleNamespace(tf='1d', calendar=dates)
        f = pd.DataFrame(dict(rv_pct=[.8, .81, math.nan, 1], rv21=[.4]*4,
            rv_median=[.2]*4, breadth_pct=[.2, .19, math.nan, .25],
            vix_ratio=[1, 1.01, math.nan, 1.1], basket_vol=[.8]*4), index=dates)
        for scaler, expected in (('HARD_RV', [1, 0, 1, 0]), ('HARD_VIX', [1, 0, 1, 0]),
                                  ('HARD_BREADTH', [1, 0, 1, 1]), ('RV_RATIO', [.5]*4),
                                  ('BASKET_VOL', [.5]*4), ('NONE', [1]*4)):
            self.assertEqual(list(compute_scales(study, scaler, features=f).values()), expected)
        combo = compute_scales(study, 'COMBO', features=f)
        parts = [compute_scales(study, s, features=f) for s in ('RV_PCT', 'BREADTH', 'VIX_TS')]
        for t in dates:
            self.assertAlmostEqual(combo[t], sum(p[t] for p in parts)/3)

    def test_log_rv_median_breadth_denominator_and_warmup(self):
        b = bars(420, lambda i: 100*math.exp(.001*i+.01*math.sin(i)))
        study = study_of({'A': rising(420), 'B': rising(420), 'C': rising(420)}, b)
        # Include a valid below-SMA200 name and an invalid-rank name.
        for t in study.calendar:
            study.ranks[t] = [('A', True, True, .2), ('B', True, False, .1), ('C', False, True, 0)]
        f = build_features(study)
        t = study.calendar[-1]
        log_rs = [math.log(b[i]['close']/b[i-1]['close']) for i in range(399, 420)]
        self.assertAlmostEqual(f.loc[t, 'rv21'], statistics.stdev(log_rs)*math.sqrt(252))
        self.assertAlmostEqual(f.loc[t, 'rv_median'], f.rv21.loc[:t].tail(252).median())
        self.assertTrue((f.breadth.loc[study.calendar] == .5).all())
        self.assertTrue(math.isnan(f.loc[study.calendar[124], 'breadth_pct']))
        self.assertTrue(math.isfinite(f.loc[study.calendar[125], 'breadth_pct']))
        self.assertEqual(compute_scales(study, 'BREADTH', features=f)[study.calendar[124]], 1)

    def test_vix_independent_ffill_no_backfill_and_cache_loader(self):
        study = study_of({'A': rising(280)}, rising(280))
        t = study.calendar[0]
        a = [dict(bars(1)[0], open_time=str(t+pd.Timedelta(days=i)),
                  open=v, high=v+1, low=v-1, close=v) for i, v in ((-1, 18), (2, 22))]
        b = [dict(bars(1)[0], open_time=str(t+pd.Timedelta(days=i)),
                  open=20, high=21, low=19, close=20) for i in (-1, 1)]
        macro = {'VIX': History(a), 'VIX3M': History(b)}
        f = build_features(study, macro=macro)
        self.assertAlmostEqual(f.loc[t, 'vix_ratio'], .9)
        self.assertAlmostEqual(f.loc[t+pd.Timedelta(days=1), 'vix_ratio'], .9)
        self.assertAlmostEqual(f.loc[t+pd.Timedelta(days=2), 'vix_ratio'], 1.1)
        self.assertTrue(math.isnan(f.loc[t-pd.Timedelta(days=2), 'vix_ratio']))
        scales = compute_scales(study, 'VIX_TS', features=f)
        self.assertEqual(scales[t], 1)
        self.assertAlmostEqual(scales[t+pd.Timedelta(days=2)], .25)
        with tempfile.TemporaryDirectory() as folder:
            for symbol, raw in (('VIX', a), ('VIX3M', b)):
                (Path(folder)/(symbol+'.json')).write_text(json.dumps(dict(series=dict(
                    symbol=symbol, timeframe='1d', bars=raw))))
            with patch('socket.socket', side_effect=AssertionError('network forbidden')):
                loaded = load_macro(folder)
            self.assertEqual(loaded['VIX'].keys, macro['VIX'].keys)
            with self.assertRaisesRegex(ValueError, 'macro cache'):
                load_macro(Path(folder)/'missing')

    def test_current_constant_basket_simple_returns_skips_incomplete_names(self):
        a = bars(290, lambda i: 100*math.exp(.001*i+.01*math.sin(i)))
        study = study_of({'A': a, 'B': rising(290)}, rising(290))
        for t in study.calendar:
            study.ranks[t] = [('B', True, True, 2), ('A', True, True, 1)]
        f = build_features(study, maxpos=1)
        t = study.calendar[-1]
        rs = study.daily['B'].frame.close.pct_change(fill_method=None).loc[:t].tail(21)
        self.assertAlmostEqual(f.loc[t, 'basket_vol'], rs.std()*math.sqrt(252))
        # Current selection, not a historical rotating basket; incomplete B is skipped.
        study = study_of({'A': a, 'B': rising(290)[:280]+rising(290)[281:]}, rising(290))
        for t0 in study.calendar:
            study.ranks[t0] = [('B', True, True, 2), ('A', True, True, 1)]
        f = build_features(study, maxpos=2)
        rs = study.daily['A'].frame.close.pct_change(fill_method=None).loc[:t].tail(21)
        self.assertAlmostEqual(f.loc[t, 'basket_vol'], rs.std()*math.sqrt(252))

    def test_every_scaler_is_point_in_time_under_future_mutation(self):
        raw = {'A': bars(620, lambda i: 100*math.exp(.002*i+.03*math.sin(i))),
               'B': bars(620, lambda i: 100*math.exp(.001*i+.04*math.cos(i)))}
        spy = bars(620, lambda i: 100*math.exp(.0005*i+.01*math.sin(i)))
        vix, vix3m = bars(620, lambda i: 20+2*math.sin(i)), bars(620, lambda i: 22+math.cos(i))
        study = study_of(raw, spy)
        cutoff = study.calendar[200]
        inputs = [raw['A'], raw['B'], spy, vix, vix3m]
        mutated = copy.deepcopy(inputs)
        for b in mutated:
            for row in b:
                if pd.Timestamp(row['open_time']) > cutoff:
                    row.update(open=999, high=1000, low=998, close=999)
        other = study_of({'A': mutated[0], 'B': mutated[1]}, mutated[2])
        f1 = build_features(study, 2, {'VIX': History(vix), 'VIX3M': History(vix3m)})
        f2 = build_features(other, 2, {'VIX': History(mutated[3]), 'VIX3M': History(mutated[4])})
        for scaler in SCALERS:
            with self.subTest(scaler=scaler):
                s1 = compute_scales(study, scaler, features=f1)
                s2 = compute_scales(other, scaler, features=f2)
                self.assertEqual({t: s for t, s in s1.items() if t <= cutoff},
                                 {t: s for t, s in s2.items() if t <= cutoff})


class RegimeExecutionTest(unittest.TestCase):
    def setUp(self):
        self.study = study_of({'A': rising(320), 'B': rising(320)}, rising(320))
        self.cfg = Config(max_positions=1, commission=0, rebalance_trading_days=40)

    def test_none_exact_fills_equity_and_all_original_metrics(self):
        plain = run_strategy(self.study, self.cfg, 'MOM_TOP')
        overlay = run_strategy(self.study, self.cfg, 'MOM_TOP',
                               compute_scales(self.study), overlay=True)
        for field in ('fills', 'equity_curve', 'decisions', 'closed_trades', 'order_events', 'analyzers'):
            self.assertEqual(plain[field], overlay[field])
        self.assertEqual(plain['metrics'], {k: overlay['metrics'][k] for k in plain['metrics']})

    def test_band_half_equity_next_open_and_no_basket_rotation(self):
        t = self.study.calendar[5]
        # Prices remain flat from initial signal onward, for exact half holdings.
        a = bars(320, lambda i: 150 if i >= 252 else 100+.1*i)
        study = study_of({'A': a, 'B': a}, rising(320))
        for key in study.calendar:
            study.ranks[key] = ([('A', True, True, 2), ('B', True, True, 1)] if key < t else
                                [('B', True, True, 2), ('A', True, True, 1)])
        scales = {k: 1 if k < t else .5 for k in study.calendar}
        r = run_strategy(study, self.cfg, 'MOM_TOP', scales)
        fills = [f for f in r['fills'] if f['exit'] == 'SCALE']
        self.assertEqual(len(fills), 1)
        self.assertEqual(fills[0]['symbol'], 'A')
        self.assertEqual(fills[0]['time'], str(t+pd.Timedelta(days=1)))
        self.assertAlmostEqual(-fills[0]['size'], r['fills'][0]['size']/2)
        decision = next(d for d in r['decisions'] if d['signal_time'] == str(t))
        self.assertFalse(decision['rebalance'])
        self.assertEqual(decision['targets'], [dict(symbol='A', weight=.5)])

    def test_sub_band_moves_accumulate_and_ordinary_rebalance_resets(self):
        t = self.study.calendar[5]
        scales = {k: 1 if k < t else .9 for k in self.study.calendar}
        r = run_strategy(self.study, self.cfg, 'MOM_TOP', scales)
        self.assertFalse(any(f['entry'] == 'SCALE' or f['exit'] == 'SCALE' for f in r['fills']))
        # Changes compare to last APPLIED scale, not the previous day's scale.
        scales = {k: 1 if i < 5 else .9 if i < 6 else .7 for i, k in enumerate(self.study.calendar)}
        r = run_strategy(self.study, self.cfg, 'MOM_TOP', scales)
        self.assertEqual([f['signal_time'] for f in r['fills'] if f['exit'] == 'SCALE'],
                         [str(self.study.calendar[6])])
        # First rebalance uses .9 even though its change is below the band.
        scales = {k: 1 if i < 40 else .9 for i, k in enumerate(self.study.calendar)}
        r = run_strategy(self.study, self.cfg, 'MOM_TOP', scales)
        d = next(d for d in r['decisions'] if d['signal_time'] == str(self.study.calendar[40]))
        self.assertEqual(d['targets'][0]['weight'], .9)
        self.assertFalse(any(f['exit'] == 'SCALE' for f in r['fills']))

    def test_zero_transitions_bypass_band_liquidate_and_rebuy_current_ranks(self):
        from dataclasses import replace as config_replace
        scales = {k: .1 if i < 5 else 0 if i < 8 else .1 for i, k in enumerate(self.study.calendar)}
        for t in self.study.calendar[8:]:
            self.study.ranks[t] = [('B', True, True, 2), ('A', True, True, 1)]
        r = run_strategy(self.study, config_replace(self.cfg, band=.35), 'MOM_TOP', scales)
        scale = [f for f in r['fills'] if f['entry'] == 'SCALE' or f['exit'] == 'SCALE']
        self.assertEqual([(f['symbol'], f['entry'], f['exit']) for f in scale],
                         [('A', '', 'SCALE'), ('B', 'SCALE', '')])
        self.assertEqual(scale[0]['time'], str(self.study.calendar[6]))
        self.assertEqual(scale[1]['time'], str(self.study.calendar[9]))
        self.assertEqual(r['equity_curve'][6]['positions'], 0)
        self.assertEqual(r['equity_curve'][9]['positions'], 1)
        self.assertEqual(r['metrics']['scale_trades'], 2)

    def test_scale_commission_counts_native_fills(self):
        from dataclasses import replace as config_replace
        scales = {k: 1 if i < 5 else .5 for i, k in enumerate(self.study.calendar)}
        r = run_strategy(self.study, config_replace(self.cfg, commission=.001), 'MOM_TOP', scales)
        self.assertGreater(r['metrics']['scale_commission'], 0)
        self.assertAlmostEqual(r['metrics']['scale_commission'],
            sum(f['commission'] for f in r['fills'] if f['exit'] == 'SCALE' or f['entry'] == 'SCALE'))
        self.assertAlmostEqual(r['metrics']['total_commission'], sum(f['commission'] for f in r['fills']))


class RegimeReportCliTest(unittest.TestCase):
    def test_offline_cli_overlay_sweep_json_report_and_plain_unchanged(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root/'macro').mkdir()
            for s in ('ABUSDT', SPY, 'VIX', 'VIX3M'):
                p = root/'macro' if s in ('VIX', 'VIX3M') else root
                (p/(s+'.json')).write_text(json.dumps(dict(series=dict(symbol=s, timeframe='1d', bars=rising(300)))))
            with patch('socket.socket', side_effect=AssertionError('network forbidden')):
                with redirect_stdout(io.StringIO()):
                    self.assertEqual(main(['stocks', '--cache', folder, '--strategies', 'SPY_BH,EW_BH',
                        '--scalers', ','.join(SCALERS), '--sweep', '--out', str(root/'r.md'),
                        '--json', str(root/'r.json')]), 0)
            d = json.loads((root/'r.json').read_text())
            report = (root/'r.md').read_text()
            self.assertEqual(len(d['strategies']), 12)
            self.assertEqual(len(d['sensitivity']), 11)
            self.assertIn('仓位缩放（regime overlay）', report)
            self.assertIn('no refit', report)
            self.assertIn('16:15 ET', report)
            self.assertIn('Sensitivity', report)
            for r in d['strategies'][2:]+d['sensitivity']:
                self.assertEqual(len(r['scale_series']), d['coverage']['evaluation_bars'])
            cfg = Config()
            study = study_of({'A': rising(300)}, rising(300))
            plain = run_study(study, cfg, ['MOM_TOP'])
            self.assertNotIn('仓位缩放（regime overlay）', render_report(plain))
            add_regime_runs(plain, study, cfg, ['RV_PCT'])
            self.assertEqual([s['name'] for s in plain['strategies']],
                             ['MOM_TOP', 'MOM_TOP+NONE', 'MOM_TOP+RV_PCT'])

    def test_sweep_grid_and_invalid_options(self):
        params = sweep_parameters()
        self.assertEqual({p['floor'] for p in params if p['scaler'] == 'RV_PCT'}, {0, .25, .5})
        self.assertEqual({p['target_vol'] for p in params if p['scaler'] == 'BASKET_VOL'}, {.3, .4, .5})
        self.assertEqual({p['band'] for p in params if p['scaler'] == 'COMBO'}, {.1, .2, .35})
        with redirect_stderr(io.StringIO()):
            for flags in (['--tf', '1h', '--scalers', 'NONE'], ['--scalers', 'BAD'],
                          ['--scalers', 'NONE,NONE'], ['--scalers', ''], ['--sweep'],
                          ['--floor', 'nan'], ['--floor', '1.1'], ['--band', '0'],
                          ['--band', '1.1'], ['--target-vol', '-1']):
                self.assertEqual(main(['stocks']+flags), 2)
        with self.assertRaisesRegex(ValueError, 'MOM_TOP only'):
            study = study_of({'A': rising(300)}, rising(300))
            run_strategy(study, Config(scaler='RV_PCT'), 'SPY_BH')

    def test_half_split_and_nonoverlapping_episode_windows(self):
        values = [100, 110, 90, 100, 111, 80, 112, 100, 113, 70]
        dates = pd.date_range('2020-01-01', periods=len(values))
        curve = [dict(time=str(t), equity=v, invested_weight=1) for t, v in zip(dates, values)]
        episodes = drawdown_episodes(curve)
        self.assertEqual([(e['peak'][:10], e['trough'][:10]) for e in episodes],
            [('2020-01-09', '2020-01-10'), ('2020-01-05', '2020-01-06'), ('2020-01-02', '2020-01-03')])
        r = dict(strategies=[dict(name='MOM_TOP+NONE', equity_curve=curve, overlay_config=dict(scaler='NONE'))])
        analysis = regime_analysis(r)
        halves = analysis['split']['MOM_TOP+NONE']
        self.assertAlmostEqual((1+halves['first_half']['total_return'])*
                               (1+halves['second_half']['total_return']), .7)
        self.assertEqual((analysis['first_half_sessions'], analysis['second_half_sessions']), (5, 5))
        self.assertAlmostEqual(analysis['drawdown_windows'][0]['returns']['MOM_TOP+NONE'], 70/113-1)


if __name__ == '__main__':
    unittest.main()
