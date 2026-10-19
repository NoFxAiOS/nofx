"""Causal account-state examples; no matching/accounting implementation."""
import collections
import sqlite3
import tempfile
from pathlib import Path
import unittest

from scripts.backtest import breaker_state_study as bs


def trade(i, entry, exit_, r, symbol='BTC'):
    return dict(id=i, entry_ms=entry, exit_ms=exit_, r=r, symbol=symbol, side='short')


class BreakerStateTests(unittest.TestCase):
    def test_only_already_closed_outcomes_and_exact_exit_are_visible(self):
        h = bs.HOUR
        rows = [trade(0, h, 10*h, -100), trade(1, 2*h, 3*h, -1),
                trade(2, 4*h, 5*h, -2), trade(3, 5*h, 7*h, 9)]
        states = bs.compute_states(rows)
        self.assertEqual(states[2], dict(day_r=-1, consec_6h=1, consec_24h=1))
        self.assertEqual(states[3], dict(day_r=-3, consec_6h=2, consec_24h=2))
        # Changing an open trade's future outcome cannot change earlier states.
        altered = [dict(t, r=100000) if t['id'] == 0 else t for t in rows]
        self.assertEqual(states, bs.compute_states(altered))
        self.assertEqual(states[:3], bs.compute_states(rows[:3]))

    def test_window_left_open_right_closed_for_both_horizons(self):
        for hours in bs.WINDOWS:
            with self.subTest(hours=hours):
                now = 3 * bs.DAY
                start = now - hours * bs.HOUR
                rows = [trade(0, start-100, start-1, -1),
                        trade(1, start-100, start, -1),
                        trade(2, start-100, start+1, -1),
                        trade(3, start-100, now, -1),
                        trade(4, start-100, now+1, -1),
                        trade(5, now, now+100, 1)]
                state = bs.compute_states(rows)[-1]
                self.assertEqual(state[f'consec_{hours}h'], 2)

    def test_utc_midnight_includes_midnight_and_overnight_close(self):
        midnight = 10 * bs.DAY
        rows = [trade(0, midnight-10000, midnight-1, -5),
                trade(1, midnight-10000, midnight, -2),
                trade(2, midnight, midnight+100, 9),
                trade(3, midnight+1, midnight+100, 9)]
        states = bs.compute_states(rows)
        self.assertEqual(states[2]['day_r'], -2)
        self.assertEqual(states[3]['day_r'], -2)
        # There are no closes on the following UTC day.
        rows.append(trade(4, midnight+bs.DAY, midnight+bs.DAY+100, 1))
        self.assertEqual(bs.compute_states(rows)[-1]['day_r'], 0)

    def test_hand_calculated_cross_coin_streak_win_zero_and_tied_entries(self):
        h = bs.HOUR
        rows = [trade(0, h, 2*h, -1, 'BTC'), trade(1, h, 3*h, -2, 'ETH'),
                trade(2, 3*h, 4*h, 1, 'SOL'), trade(3, 4*h, 5*h, -1),
                trade(4, 5*h, 6*h, 0), trade(5, 6*h, 7*h, -1),
                trade(6, 7*h, 9*h, 1), trade(7, 7*h, 8*h, -1)]
        states = bs.compute_states(rows)
        self.assertEqual([s['consec_24h'] for s in states], [0, 0, 2, 0, 1, 0, 1, 1])
        self.assertEqual(states[6], states[7])
        # Input order must not become the close order.
        self.assertEqual(list(reversed(states)), bs.compute_states(list(reversed(rows))))

    def test_exact_day_buckets_and_inclusive_thresholds(self):
        self.assertEqual([bs.day_bucket(x) for x in (0, -1, -2, -3, -4, -6)],
                         ['> -1', '(-2,-1]', '(-3,-2]', '(-4,-3]', '<= -4', '<= -4'])
        states = [dict(day_r=-x, consec_6h=x, consec_24h=x) for x in range(7)]
        groups = {g['name']: g['members'] for g in bs.state_groups(states)}
        self.assertEqual(len(groups), 25)
        self.assertEqual(groups['day/<= -3'], {3, 4, 5, 6})
        self.assertEqual(groups['24h/k=4+'], {4, 5, 6})
        self.assertEqual(groups['6h/k>=2'], {2, 3, 4, 5, 6})

    def test_daily_sampling_is_fixed_capped_and_ignores_outcomes(self):
        rows = [trade(i, 10*bs.DAY + (i//20)*bs.DAY + (i%20)*100,
                      15*bs.DAY, i-10) for i in range(40)]
        sampled = bs.sample_daily(rows, 7)
        self.assertEqual(sampled, bs.sample_daily(list(reversed(rows)), 7))
        self.assertEqual(len(sampled), 16)
        counts = collections.Counter(t['entry_ms']//bs.DAY for t in sampled)
        self.assertEqual(set(counts.values()), {8})
        hostile = [dict(t, r=-1000*t['r']) for t in rows]
        self.assertEqual([t['id'] for t in sampled], [t['id'] for t in bs.sample_daily(hostile, 7)])
        self.assertNotEqual(sampled, bs.sample_daily(rows, 8))

    def test_close_order_drawdown_and_halves_preserve_prior_state(self):
        day = bs.DAY
        rows = [trade(0, day, 3*day, -2), trade(1, day+100, 2*day, 4),
                trade(2, 3*day, 4*day, 1)]
        self.assertEqual(bs.outcome_stats(rows, range(3))['dd'], 2)
        stream = bs.analyse_stream(rows, day+100)
        self.assertEqual(stream['observations'][2]['state']['consec_24h'], 1)
        row = bs.get_row(stream, '24h/k=1', 'second')
        self.assertEqual(row['stats']['n'], 1)
        self.assertEqual(row['after']['n'], 0)

    def test_repeated_seeds_do_not_reduce_shared_week_standard_error(self):
        d = dict(diff=-.2, cluster_se=.1, week_influence={'10': .05, '11': -.05})
        d['cluster_se'] = (.01)**.5
        agg = bs.ensemble_difference([d] * 20)
        self.assertAlmostEqual(agg['diff'], -.2)
        self.assertAlmostEqual(agg['cluster_se'], d['cluster_se'])
        self.assertIsNone(bs.ensemble_difference([d, None])['cluster_se'])

    def test_live_r_quantity_priority_and_missing_stop_exclusion(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'live.db'
            with sqlite3.connect(path) as con:
                con.execute('CREATE TABLE trader_positions (id INTEGER,trader_id TEXT,symbol TEXT,'
                            'side TEXT,entry_time INTEGER,exit_time INTEGER,entry_quantity REAL,'
                            'quantity REAL,entry_price REAL,initial_stop_loss REAL,realized_pnl REAL,'
                            'fee REAL,ai_managed INTEGER,source TEXT,close_reason TEXT,status TEXT)')
                for i, qty, stop, managed in ((0, 4, 90, 1), (1, 0, 90, 1),
                                               (2, 4, 0, 1), (3, 4, 0, 0)):
                    con.execute('INSERT INTO trader_positions VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)',
                                (i, 'test_1787982996', 'BTC', 'long', 1000, 2000,
                                 qty, 2, 100, stop, -20, 5, managed, 'sync', 'sl', 'CLOSED'))
            trades, meta = bs.load_live(path)
        self.assertEqual([t['r'] for t in trades], [-.5, -1])
        self.assertEqual(meta['quantity_fallback'], 1)
        self.assertEqual(meta['excluded'], {'no_initial_stop': 1, 'manual_unmanaged': 1})


if __name__ == '__main__':
    unittest.main()
