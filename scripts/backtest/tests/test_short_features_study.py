"""Synthetic causal checks, numerical statistics, and label-maturity boundaries."""
import tempfile
import unittest

import numpy as np
import pandas as pd

from scripts.backtest import short_features_study as sf


def synthetic_bars(days=40, offset=0):
    n = days*288
    i = np.arange(n)
    o = 200+offset-.001*i+np.sin(i/37)*.2
    c = o-.0005
    return np.column_stack([i*sf.M5,o,o+.3,c-.3,c]).tolist()


def volume_arrays(hours=40):
    i = np.arange(hours)
    return dict(t_open=i*sf.HOUR,t_close=(i+1)*sf.HOUR,o=np.full(hours,100.),
                c=np.full(hours,99.),h=np.full(hours,101.),l=np.full(hours,98.),
                v=np.full(hours,10.),qv=np.full(hours,1000.),tb=np.r_[np.full(21,6.),np.full(hours-21,3.)])


class ShortFeatureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.ctx = sf.bd.Context.from_bars(synthetic_bars())
        cls.t = 36*sf.DAY

    def test_breadth_from_all_members(self):
        a = dict(ret_24h=-.1,price_ema20=-1,ema_gap=-2)
        b = dict(ret_24h=.2,price_ema20=1,ema_gap=-1)
        c = dict(ret_24h=-.02,price_ema20=3,ema_gap=2)
        m = sf.market_features(dict(a=a,b=b,c=c),self.ctx,self.t)
        self.assertAlmostEqual(m['breadth_negative'],2/3)
        self.assertAlmostEqual(m['breadth_below_ema20'],1/3)
        self.assertAlmostEqual(m['breadth_ema_bear'],2/3)
        self.assertEqual(m['market_median'],-.02)

    def test_ema_gap_and_new_low_bounds(self):
        f = sf.price_features(self.ctx,self.ctx,self.t,{},np.empty((0,2)))
        k = sf.bd.closed_window(self.ctx.a4,self.t,84,4*sf.HOUR)
        expected = (sf.bd.ema(k['c'],20)[-1]-sf.bd.ema(k['c'],50)[-1])/sf.bd.atr(k['h'],k['l'],k['c'])[-1]
        self.assertAlmostEqual(f['ema_gap'],expected)
        k1 = sf.bd.closed_window(self.ctx.a1,self.t,60,sf.HOUR)
        self.assertEqual(f['new_low'],float(k1['l'][-1] < min(k1['l'][-25:-1])))
        # An exact tie with a previous low does not count as a fresh low.
        x = dict(c=np.full(60,100.),h=np.full(60,101.),l=np.full(60,90.))
        y = dict(h=np.full(84,101.))
        self.assertEqual(sf.continuation_parts(x,y)[1],0)
        x['c'][-1],x['l'][-1] = 89,88
        self.assertEqual(sf.continuation_parts(x,y)[1],1)

    def test_new_listing_kept_with_missing_trend_features(self):
        # At 3d there is enough 1h ATR history but not 84x4h trend history.
        f = sf.price_features(self.ctx,self.ctx,3*sf.DAY,{},np.empty((0,2)))
        self.assertIsNotNone(f)
        self.assertTrue(np.isfinite(f['ret_24h']))
        self.assertGreater(f['atr'],0)
        self.assertTrue(np.isnan(f['ema_gap']))
        self.assertTrue(np.isnan(f['gate']))

    def test_taker_share_and_exact_close_boundary(self):
        a = volume_arrays()
        f = sf.volume_features(a,24*sf.HOUR)
        self.assertAlmostEqual(f['taker_buy'],.3)
        self.assertAlmostEqual(f['sell_pressure'],.7)
        self.assertAlmostEqual(f['taker_change'],-.3)
        self.assertAlmostEqual(f['quote_volume_ratio'],1)
        a['tb'][24:] = 10**8  # hostile future must not enter the calculation
        self.assertEqual(f,sf.volume_features(a,24*sf.HOUR))
        self.assertTrue(np.isnan(sf.volume_features(a,24*sf.HOUR-1)['taker_buy']))

    def test_funding_no_future_and_stale_is_missing(self):
        f = np.array([[0,.001],[8*sf.HOUR,.002],[16*sf.HOUR,.004],[24*sf.HOUR,.006],[32*sf.HOUR,-99.]])
        r = sf.funding_features(f,24*sf.HOUR)
        self.assertEqual(r['funding_rate'],.006)
        self.assertAlmostEqual(r['funding_change3'],.005)
        self.assertTrue(np.isnan(sf.funding_features(f,50*sf.HOUR)['funding_rate']))

    def test_full_universe_causal_prefix_rebuild(self):
        syms = ['BTCUSDT','TEST']
        ctxs = dict.fromkeys(syms,self.ctx)
        aux = dict.fromkeys(syms,(volume_arrays(1000),np.array([[35*sf.DAY,.0001],[37*sf.DAY,-.9]])))
        self.assertEqual(sf.causal_check(ctxs,syms,aux,self.t),2)
        rows = synthetic_bars()
        hostile = [r if r[0]<self.t else [r[0],900,2000,1,1500] for r in rows]
        alt = sf.bd.Context.from_bars(hostile)
        old = sf.price_features(self.ctx,self.ctx,self.t,{},np.empty((0,2)))
        new = sf.price_features(alt,alt,self.t,{},np.empty((0,2)))
        for k in old:
            self.assertTrue(np.isclose(old[k],new[k],equal_nan=True),k)

    def test_spearman_ties_missing_and_constant(self):
        self.assertEqual(sf.spearman([1,1,3,np.nan],[2,2,5,9]),1)
        self.assertEqual(sf.spearman([1,2,3],[3,2,1]),-1)
        self.assertTrue(np.isnan(sf.spearman([1,1,1],[1,2,3])))
        d = pd.DataFrame(dict(t_ms=[0]*6+[sf.HOUR]*6,regime=['chop']*12,
                              x=list(range(6))*2,y=list(range(6))+list(range(5,-1,-1))))
        self.assertTrue(np.allclose(sf.ic_series(d,'x','y')['ic'],[1,-1]))

    def test_ridge_numerics_and_strong_regularization(self):
        rng = np.random.default_rng(7)
        x = rng.normal(size=(600,3))
        y = 2+x@np.array([.4,-.2,0.])
        weak = sf.fit_ridge(x,y,alpha=1e-8)
        self.assertTrue(np.allclose(weak['coef'],[.4,-.2,0],atol=1e-6))
        self.assertTrue(np.allclose(sf.predict_ridge(weak,x),y,atol=1e-6))
        strong = sf.fit_ridge(x,y,alpha=10)
        self.assertLess(np.linalg.norm(strong['coef']),np.linalg.norm(weak['coef'])/5)

    def test_walk_forward_has_no_label_leakage(self):
        times = np.arange(1000)*4*sf.HOUR
        for folds in (1,4):
            seen = []
            splits = list(sf.walk_forward(times,folds))
            self.assertEqual(len(splits),folds)
            for train,test in splits:
                self.assertLess(train[-1]+sf.PURGE,test[0])
                self.assertFalse(set(train)&set(test))
                self.assertFalse(set(seen)&set(test))
                seen.extend(test)

    def test_test_labels_cannot_change_model_or_prediction(self):
        rng = np.random.default_rng(33)
        n,coins = 600,12
        d = pd.DataFrame({'t_ms':np.repeat(np.arange(n)*4*sf.HOUR,coins),
                          'sym':list(range(coins))*n,'all':([True]*6+[False]*6)*n,
                          'regime':np.repeat(np.array(['btc_bear','chop','btc_bull'])[np.arange(n)%3],coins),
                          'signal':rng.normal(size=n*coins)})
        d['excess_24h'] = .01*d['signal']+rng.normal(0,.002,len(d))
        d['new'],d['old'] = d['signal'],d['signal']
        cache = {('signal','excess_24h'):sf.ic_series(d[d['all']],'signal','excess_24h')}
        tr,te = next(sf.walk_forward(d['t_ms'],1))
        p,fit = sf.fit_scores(d,['signal'],cache,tr,te)
        self.assertEqual(fit['stable'],[('signal',1.0)])
        self.assertTrue(p['equal'].notna().all())
        d.loc[d['t_ms'].isin(te),'excess_24h'] = 999
        hostile_cache = {('signal','excess_24h'):sf.ic_series(d[d['all']],'signal','excess_24h')}
        pp,fit2 = sf.fit_scores(d,['signal'],hostile_cache,tr,te)
        self.assertTrue(np.allclose(p['ridge'],pp['ridge']))
        self.assertTrue(np.allclose(p['equal'],pp['equal'],equal_nan=True))
        self.assertEqual(fit['stable'],fit2['stable'])
        self.assertEqual(fit['ridge_coef'],fit2['ridge_coef'])
        self.assertLess(fit['train_last']+sf.PURGE,fit['test_first'])

    def test_shared_week_difference_and_bootstrap(self):
        d = pd.DataFrame(dict(t_ms=np.arange(120)*sf.DAY,net_r=np.sin(np.arange(120))))
        delta = sf.cluster_difference(d,d,'net_r')
        self.assertEqual(delta['mean'],0)
        self.assertAlmostEqual(delta['se'],0)
        s = sf.cluster_estimate(d['t_ms'],d['net_r'])
        self.assertGreater(s['weeks'],10)
        self.assertGreater(s['se'],0)
        d['x'],d['y'] = np.arange(120),np.arange(120)
        p = sf.pooled_ic(d,'x','y',reps=9)
        self.assertAlmostEqual(p['mean'],1)
        self.assertAlmostEqual(p['se'],0)

    def test_week_clustering_accounts_for_shared_market_shocks(self):
        week_shocks = np.tile([-1.,1.],4)
        times = np.repeat(np.arange(8)*7*sf.DAY,100)
        values = np.repeat(week_shocks,100)
        s = sf.cluster_estimate(times,values)
        iid = np.std(values,ddof=1)/np.sqrt(len(values))
        self.assertGreater(s['se'],8*iid)

    def test_fetch_page_resume_and_schema_without_network(self):
        class Client:
            def __init__(self):
                self.calls = 0
            def get(self, endpoint, params):
                self.calls += 1
                t = params['startTime']
                return [[t,'1','2','.5','1.5','10',t+sf.HOUR-1,'15',2,'4','6',0]]
        with tempfile.TemporaryDirectory() as tmp:
            c = Client()
            r = sf.fetch_symbol(c,tmp,'TEST',0,2*sf.HOUR,'ict1h')
            self.assertTrue(r['complete'])
            self.assertEqual(r['n'],2)
            sf.fetch_symbol(c,tmp,'TEST',0,2*sf.HOUR,'ict1h')
            self.assertEqual(c.calls,2)
            a,f = sf.load_aux(tmp,'TEST')
            self.assertEqual(a['tb'][0],4)
            self.assertEqual(a['t_close'][0],sf.HOUR)
            self.assertEqual(f.shape,(0,2))


if __name__ == '__main__':
    unittest.main()
