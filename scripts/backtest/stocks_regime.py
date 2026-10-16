"""Point-in-time regime features and position scales. No execution or ledger."""
import json
import math
from pathlib import Path

import pandas as pd

from .stocks_data import History, SPY

SESSIONS_PER_YEAR = 252
VOL_WINDOW = 21
RANK_WINDOW = 252
MIN_RANK_VALUES = 126
PCT_MIDPOINT = 0.50
VIX_LOW = 0.90
VIX_HIGH = 1.10
HARD_RV_THRESHOLD = 0.80
HARD_VIX_THRESHOLD = 1.00
HARD_BREADTH_THRESHOLD = 0.20
SCALERS = ('NONE', 'RV_PCT', 'RV_RATIO', 'BREADTH', 'VIX_TS', 'BASKET_VOL',
           'COMBO', 'HARD_RV', 'HARD_VIX', 'HARD_BREADTH')
MACRO_SCALERS = {'VIX_TS', 'COMBO', 'HARD_VIX'}


def load_macro(folder):
    out = {}
    for symbol in ('VIX', 'VIX3M'):
        path = Path(folder)/(symbol+'.json')
        try:
            doc = json.loads(path.read_text())['series']
            if doc['symbol'] != symbol or doc['timeframe'] != '1d':
                raise ValueError('symbol/timeframe mismatch')
            out[symbol] = History(doc['bars'])
            if not out[symbol].keys:
                raise ValueError('no valid bars')
        except (OSError, ValueError, KeyError) as exc:
            raise ValueError(f'macro cache {path}: {exc}') from exc
    return out


def percentile(series):
    """Inclusive rolling average rank / n; NaNs never count as observations."""
    return series.rolling(RANK_WINDOW, min_periods=MIN_RANK_VALUES).rank(pct=True)


def rv_pct_scale(pct, floor):
    return 1.0 if not math.isfinite(pct) or pct <= PCT_MIDPOINT else (
        1-(1-floor)*(pct-PCT_MIDPOINT)/(1-PCT_MIDPOINT))


def breadth_scale(pct, floor):
    return 1.0 if not math.isfinite(pct) or pct >= PCT_MIDPOINT else (
        floor+(1-floor)*pct/PCT_MIDPOINT)


def ratio_scale(reference, current, floor):
    # Zero realized volatility warrants full size; unavailable inputs are neutral.
    if not math.isfinite(reference) or not math.isfinite(current) or current <= 0:
        return 1.0
    return min(1.0, max(floor, reference/current))


def vix_scale(value, floor):
    if not math.isfinite(value):
        return 1.0
    return min(1.0, max(floor, 1-(1-floor)*(value-VIX_LOW)/(VIX_HIGH-VIX_LOW)))


def build_features(study, maxpos=10, macro=None):
    if study.tf != '1d':
        raise ValueError('regime overlays require --tf 1d; --tf 1h is unsupported')
    daily = study.daily[SPY].frame.index
    close = study.daily[SPY].frame['close'].reindex(daily)
    log_returns = close.map(math.log).diff()
    f = pd.DataFrame(index=daily)
    f['rv21'] = log_returns.rolling(VOL_WINDOW).std(ddof=1)*math.sqrt(SESSIONS_PER_YEAR)
    f['rv_pct'] = percentile(f['rv21'])
    # Unlike the percentile rules, RV_RATIO has no 126-observation warmup.
    f['rv_median'] = f['rv21'].rolling(RANK_WINDOW, min_periods=1).median()
    breadth, basket_vol = {}, {}
    stock_returns = pd.DataFrame({s: h.frame['close'].reindex(daily).pct_change(fill_method=None)
                                 for s, h in study.daily.items() if s != SPY}, index=daily)
    for t in study.calendar:
        ranks = study.ranks[t]
        valid = [r for r in ranks if r[1]]
        breadth[t] = sum(r[1] and r[2] for r in valid)/len(valid) if valid else math.nan
        names = [r[0] for r in ranks if r[1] and r[2]][:maxpos]
        # Constant CURRENT basket over the trailing 21 SPY session returns.
        # A name needs all 21 returns (22 closes); no stale-price forward fill.
        window = stock_returns.loc[:t, names].tail(VOL_WINDOW).dropna(axis=1)
        basket_vol[t] = (window.mean(axis=1).std(ddof=1)*math.sqrt(SESSIONS_PER_YEAR)
                         if len(window) == VOL_WINDOW and len(window.columns) else math.nan)
    f['breadth'] = pd.Series(breadth, dtype=float)
    f['breadth_pct'] = percentile(f['breadth'])
    f['basket_vol'] = pd.Series(basket_vol, dtype=float)
    f['vix_ratio'] = math.nan
    if macro is not None:
        # reindex(method=ffill) searches the ORIGINAL macro dates, including dates
        # absent from SPY; it cannot pick a later macro observation.
        vix = macro['VIX'].frame['close'].reindex(daily, method='ffill')
        vix3m = macro['VIX3M'].frame['close'].reindex(daily, method='ffill')
        f['vix_ratio'] = vix/vix3m
    return f


def compute_scales(study, scaler='NONE', floor=0.25, target_vol=0.40,
                   maxpos=10, macro=None, features=None):
    if study.tf != '1d':
        raise ValueError('regime overlays require --tf 1d; --tf 1h is unsupported')
    if scaler not in SCALERS:
        raise ValueError('unknown scaler: '+scaler)
    if not math.isfinite(floor) or not 0 <= floor <= 1:
        raise ValueError('floor must be finite in [0,1]')
    if not math.isfinite(target_vol) or target_vol <= 0:
        raise ValueError('target-vol must be positive and finite')
    if scaler == 'NONE':
        return dict.fromkeys(study.calendar, 1.0)
    if scaler in MACRO_SCALERS and macro is None and features is None:
        raise ValueError(scaler+' requires local VIX and VIX3M macro cache')
    features = features if features is not None else build_features(study, maxpos, macro)
    out = {}
    for t, r in features.reindex(study.calendar).iterrows():
        rv = rv_pct_scale(r.rv_pct, floor)
        breadth = breadth_scale(r.breadth_pct, floor)
        vix = vix_scale(r.vix_ratio, floor)
        value = dict(RV_PCT=rv, RV_RATIO=ratio_scale(r.rv_median, r.rv21, floor),
                     BREADTH=breadth, VIX_TS=vix,
                     BASKET_VOL=ratio_scale(target_vol, r.basket_vol, floor),
                     COMBO=(rv+breadth+vix)/3,
                     HARD_RV=float(not r.rv_pct > HARD_RV_THRESHOLD),
                     HARD_VIX=float(not r.vix_ratio > HARD_VIX_THRESHOLD),
                     HARD_BREADTH=float(not r.breadth_pct < HARD_BREADTH_THRESHOLD))[scaler]
        out[t] = float(min(1.0, max(0.0, value)))
    return out


def sweep_parameters():
    """Fixed one-factor-at-a-time grid; overlapping default cases run once."""
    grid = [dict(scaler=s, floor=f, band=.20, target_vol=.40)
            for s in ('RV_PCT', 'COMBO') for f in (0.0, .25, .50)]
    grid += [dict(scaler='BASKET_VOL', floor=.25, band=.20, target_vol=v) for v in (.30, .40, .50)]
    grid += [dict(scaler='COMBO', floor=.25, band=b, target_vol=.40) for b in (.10, .35)]
    return grid
