"""Range/chasing features stay causal; paired inference preserves covariance."""
import math
import unittest

from scripts.backtest import breakdown_study as bd, breakdown_range_study as rs


def bars(days=5):
    rows = []
    for i in range(days * 288):
        o = 100 + math.sin(i / 27)
        rows.append([i * bd.M5, o, o + 1, o - 1, o + .1])
    return rows


class BreakdownRangeStudyTests(unittest.TestCase):
    def test_features_numeric_and_future_truncation(self):
        raw, t = bars(), 4*bd.DAY
        ctx = bd.Context.from_bars(raw)
        job = dict(sym='TEST', t_ms=t, ref=100., atr_pct=2.)
        f = rs.decision_features(ctx, ctx, job)
        bk = bd.closed_window(ctx.a4, t, 18, 4*bd.HOUR)
        k1 = bd.closed_window(ctx.a1, t, 24, bd.HOUR)
        self.assertAlmostEqual(f['btc_range72'], (max(bk['h'])-min(bk['l']))/bk['c'][-1]*100)
        self.assertAlmostEqual(f['chase_low_atr'], (100-min(k1['l']))/2)
        clipped = bd.Context.from_bars([r for r in raw if r[0]+bd.M5 <= t])
        self.assertEqual(f, rs.decision_features(clipped, clipped, job))
        hostile = bd.Context.from_bars([r if r[0] < t else [r[0], 100, 100000, .00001, 100] for r in raw])
        self.assertEqual(f, rs.decision_features(hostile, hostile, job))
        rs.causal_check(ctx, ctx, job)
        self.assertEqual(int(bk['t_close'][-1]), t)
        self.assertTrue(all(int(x) <= t for x in bk['t_close']))

    def test_buckets_at_exact_boundaries_and_ties(self):
        self.assertEqual([rs.range_bucket(v, (3, 6)) for v in (2.999, 3, 5.999, 6)], ['低', '中', '中', '高'])
        self.assertEqual(rs.range_bucket(3, (3, 3)), '高')
        self.assertEqual([rs.chase_bucket(v) for v in (-1, .4999, .5, 1.4999, 1.5)],
                         ['<0.5', '<0.5', '0.5–1.5', '0.5–1.5', '>=1.5'])
        jobs = [dict(regime='chop')] * 2
        fs = [dict(btc_range72=v, btc_range72_bucket='低', chase_low_atr=.49) for v in (2.999, 3)]
        groups = {n: ix for n, ix, _ in rs.group_definitions(jobs, fs)}
        self.assertEqual(groups['range<3%'], {0})
        self.assertEqual(groups['range>=3%'], {1})
        self.assertEqual(groups['range<3%∧chase<0.5'], {0})

    def test_paired_week_cluster_hand_calculation(self):
        # Two complete week clusters: differences [1,3] and [-1,1].
        # mean=1; cluster influences +.5 and -.5 -> sqrt(2*.5)=1.
        jobs = [dict(t_ms=t) for t in (0, bd.DAY, 7*bd.DAY, 8*bd.DAY)]
        mkt = [dict(r=0., how='tp')] * 4
        pb = [dict(r=v, how='tp' if v is not None else 'unfilled') for v in (1., 3., -1., 1.)]
        d = rs.paired_difference(jobs, pb, mkt, list(range(4)))
        self.assertAlmostEqual(d['diff'], 1.)
        self.assertAlmostEqual(d['cluster_se'], 1.)
        self.assertEqual(d['weeks'], 2)
        # Missing fills are zero returns, not dropped; market covariance retained.
        pb = [dict(r=None, how='unfilled'), dict(r=4., how='tp')]
        market = [dict(r=1., how='tp'), dict(r=2., how='tp')]
        d = rs.paired_difference([jobs[0], jobs[2]], pb, market, [0, 1])
        self.assertAlmostEqual(d['diff'], .5)
        self.assertAlmostEqual(d['cluster_se'], 1.5)
        self.assertEqual(rs.paired_difference(jobs, mkt, mkt, list(range(4)))['cluster_se'], 0.)
        with self.assertRaises(ValueError):
            rs.paired_difference(jobs[:1], [dict(r=None, how='invalid')], mkt[:1], [0])

    def test_pullback_geometry_and_fill_window(self):
        job = dict(sym='TEST', side='short', kind='mkt', idx=7, t_ms=10*bd.HOUR,
                   ref=100., sl=104., tp=92., atr_pct=2., key=['TEST', 'short', 10*bd.HOUR])
        for name, price, valid in (('PB05', 101., 24), ('PB10', 102., 48)):
            pb = rs.pullback_job(job, name)
            self.assertEqual(pb['ref'], price)
            self.assertEqual(pb['sl'], price+4)
            self.assertEqual(pb['tp'], price-8)
            self.assertEqual(pb['idx'], job['idx'])
            self.assertEqual(pb['valid_bars'], valid)
            r = dict(r=1., how='tp', entry_ms=job['t_ms'], exit_ms=job['t_ms']+2*bd.M5)
            rs.assert_limit_result(pb, r)
            with self.assertRaises(AssertionError):
                rs.assert_limit_result(pb, dict(r, entry_ms=job['t_ms']-bd.M5))
            with self.assertRaises(AssertionError):
                rs.assert_limit_result(pb, dict(r, entry_ms=job['t_ms']+valid*bd.M5))
        rs.assert_limit_result(pb, dict(r=None, how='unfilled', entry_ms=None))

    def test_a_filters_full_strategy_before_feature_slices(self):
        jobs = [dict(sym='TEST', side='short', t_ms=t, new=60, regime='chop') for t in (0, 5, 10)]
        results = [dict(r=1., exit_ms=t) for t in (20, 6, 11)]
        features = [dict(btc_range72=v, btc_range72_bucket='低', chase_low_atr=0.) for v in (4, 2, 2)]
        a = rs.analyse_a(jobs, results, features)
        row = next(r for r in a['rows'] if r['sample']=='NEW>=50' and not r['allow_overlap'] and r['group']=='range<3%')
        self.assertEqual(row['stats']['n'], 0)  # bucket cannot reopen while the first position is busy


if __name__ == '__main__':
    unittest.main()
